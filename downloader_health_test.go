package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDownloaderHealthSeparatesTrackerStorageAndConnectionFailures(t *testing.T) {
	var unavailable atomic.Bool
	var uploaded atomic.Int64
	uploaded.Store(100)
	client := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/torrents/info":
			if unavailable.Load() {
				http.Error(w, "test outage", http.StatusServiceUnavailable)
				return
			}
			fmt.Fprintf(w, `[{"hash":"abc","name":"Fixture","uploaded":%d,"tracker":"https://tracker.example/announce"}]`, uploaded.Load())
		case "/api/v2/torrents/trackers":
			fmt.Fprint(w, "<html>temporary upstream error</html>")
		default:
			http.NotFound(w, r)
		}
	}))
	defer client.Close()
	s := testStore(t)
	c := defaultConfig()
	paused := false
	c.Clients = []Client{{ID: "fixture", Name: "Fixture", Kind: "qb", URL: client.URL, Enabled: true, StrategyEnabled: &paused}}
	if err := s.saveConfig(c); err != nil {
		t.Fatal(err)
	}
	e := newEngine(s)
	e.tick(context.Background())
	if len(e.statuses) != 1 || !e.statuses[0].Online || len(e.torrents) != 1 || len(e.torrents[0].Hosts) != 1 || e.torrents[0].Hosts[0] != "tracker.example" {
		t.Fatalf("supplemental tracker response invalidated main sample: %+v", e.statuses)
	}
	if _, err := s.db.Exec("CREATE TRIGGER fail_baseline BEFORE INSERT ON baseline BEGIN SELECT RAISE(ABORT, 'fixture write blocked'); END"); err != nil {
		t.Fatal(err)
	}
	uploaded.Store(150)
	e.tick(context.Background())
	if st := e.statuses[0]; !st.Online || !st.StatisticsError || st.Failures != 0 || len(e.torrents) != 1 {
		t.Fatalf("statistics write misclassified as connection failure: %+v", st)
	}
	if !strings.Contains(e.statuses[0].Error, "save baseline: database constraint") {
		t.Fatalf("missing safe reason and stage: %s", e.statuses[0].Error)
	}
	if _, err := s.db.Exec("DROP TRIGGER fail_baseline"); err != nil {
		t.Fatal(err)
	}
	unavailable.Store(true)
	for i := 1; i <= downloaderAlertFailures; i++ {
		e.tick(context.Background())
		if st := e.statuses[0]; st.Online || st.StatisticsError || st.Failures != i || st.LastSuccess == 0 {
			t.Fatalf("bad failure state after poll %d: %+v", i, st)
		}
	}
	unavailable.Store(false)
	uploaded.Store(200)
	e.tick(context.Background())
	if st := e.statuses[0]; !st.Online || st.Failures != 0 || st.Error != "" {
		t.Fatalf("successful sample did not clear outage: %+v", st)
	}
	var total int64
	if err := s.db.QueryRow("SELECT up FROM lifetime WHERE id=1").Scan(&total); err != nil || total != 100 {
		t.Fatalf("expected failed write and recovery to count 100 bytes exactly once; got %d (%v)", total, err)
	}
	entries, err := e.diagnostics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Recovered", "Recovered", "3/3 consecutive", "1/3 consecutive", "save baseline:", "invalid Tracker metadata response"}
	kinds := []string{"diag-statistics", "diag-sampling", "diag-sampling", "diag-sampling", "diag-statistics", "diag-tracker"}
	if len(entries) != len(want) {
		t.Fatalf("unexpected diagnostic events: %+v", entries)
	}
	for i, entry := range entries {
		if entry.Kind != kinds[i] || entry.Key != "fixture" || !strings.Contains(entry.Message, want[i]) {
			t.Fatalf("unexpected diagnostic %d: %+v", i, entry)
		}
	}
}

func TestDiagnosticsSurviveTemporaryWriteFailure(t *testing.T) {
	s := testStore(t)
	e := newEngine(s)
	ctx := context.Background()
	if _, err := s.db.Exec("CREATE TRIGGER fail_events BEFORE INSERT ON events BEGIN SELECT RAISE(ABORT, 'fixture write blocked'); END"); err != nil {
		t.Fatal(err)
	}
	e.diagnosticCondition("statistics", "fixture", "SQLite write failed")
	e.diagnosticCondition("statistics", "fixture", "SQLite write failed")
	e.flushDiagnostics(ctx)
	entries, err := e.diagnostics(ctx)
	if err != nil || len(entries) != 1 || entries[0].Message != "SQLite write failed" {
		t.Fatalf("diagnostic missing during failure: %+v (%v)", entries, err)
	}
	if _, err := s.db.Exec("DROP TRIGGER fail_events"); err != nil {
		t.Fatal(err)
	}
	e.diagnosticCondition("statistics", "fixture", "")
	e.flushDiagnostics(ctx)
	entries, err = newEngine(s).diagnostics(ctx)
	if err != nil || len(entries) != 2 || entries[0].Message != "Recovered" || entries[1].Message != "SQLite write failed" {
		t.Fatalf("diagnostics not persisted in order: %+v (%v)", entries, err)
	}
}

func TestPrimarySampleRetriesOnlyTransientResponses(t *testing.T) {
	var requests atomic.Int32
	var unauthorized atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requests.Add(1)
		if unauthorized.Load() {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if count == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		fmt.Fprint(w, "[]")
	}))
	defer server.Close()
	a := adapter(Client{ID: "fixture", Kind: "qb", URL: server.URL})
	if rows, err := a.sample(context.Background()); err != nil || rows == nil || requests.Load() != 2 {
		t.Fatalf("transient failure should retry once: %v, requests %d", err, requests.Load())
	}
	unauthorized.Store(true)
	if _, err := a.sample(context.Background()); err == nil || !strings.Contains(err.Error(), "HTTP 401") || requests.Load() != 3 {
		t.Fatalf("authentication failure must be reported without retry: %v, requests %d", err, requests.Load())
	}
}
