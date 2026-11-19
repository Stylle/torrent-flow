package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPerSiteSpeedsSamplingAndRange(t *testing.T) {
	s := testStore(t)
	cl, _ := mockClient(t)
	c := defaultConfig()
	c.Clients = []Client{cl}
	s.saveConfig(c)
	e := newEngine(s)
	e.tick(context.Background())
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM site_speeds").Scan(&count)
	if count != 1 {
		t.Fatal("per-site sample missing")
	}
	old := time.Now().Add(-25 * time.Hour).Unix()
	s.db.Exec("INSERT INTO site_speeds VALUES(?,?,?,?)", old, "old", 123, 0)
	sv := &Server{s: s, e: e}
	rec := httptest.NewRecorder()
	sv.snapshot(rec, httptest.NewRequest("GET", "/api/snapshot?window=86400&view=active", nil))
	var v struct {
		Speeds []struct {
			Site string `json:"site"`
		} `json:"siteSpeeds"`
		Window  int              `json:"speedWindow"`
		Summary map[string]int64 `json:"activeSummary"`
	}
	if json.Unmarshal(rec.Body.Bytes(), &v) != nil || rec.Code != 200 || v.Window != 86400 || len(v.Speeds) != 1 || v.Speeds[0].Site == "old" || v.Summary["up"] != 3000 {
		t.Fatal("24h range or active summary incorrect")
	}
}

func TestHistoryCacheInvalidatesAfterRestoreEpoch(t *testing.T) {
	s := testStore(t)
	now := time.Now().Unix()
	s.db.Exec("INSERT INTO site_speeds VALUES(?,?,?,?)", now, "a", 100, 0)
	sv := &Server{s: s, e: newEngine(s)}
	if len(sv.siteSpeedHistory(86400)) != 1 {
		t.Fatal("history missing")
	}
	s.db.Exec("INSERT INTO site_speeds VALUES(?,?,?,?)", now, "b", 200, 0)
	if len(sv.siteSpeedHistory(86400)) != 1 {
		t.Fatal("history cache not reused")
	}
	sv.speedEpoch.Add(1)
	if len(sv.siteSpeedHistory(86400)) != 2 {
		t.Fatal("restore epoch did not invalidate history")
	}
}
