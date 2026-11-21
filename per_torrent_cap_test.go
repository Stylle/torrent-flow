package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEightTorrentsEachGetTheir100KBCeiling(t *testing.T) {
	s := testStore(t)
	var mu sync.Mutex
	limits := map[string]int64{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/info"):
			rows := []map[string]any{}
			for i := 0; i < 8; i++ {
				hash := fmt.Sprintf("hash%d", i)
				if _, ok := limits[hash]; !ok {
					limits[hash] = 0
				}
				rows = append(rows, map[string]any{"hash": hash, "name": "Fixture", "tracker": "https://a.example/announce", "uploaded": 100, "downloaded": 100, "upspeed": 1000, "num_leechs": 1, "up_limit": limits[hash], "added_on": 1})
			}
			json.NewEncoder(w).Encode(rows)
		case strings.HasSuffix(r.URL.Path, "/uploadLimit"):
			json.NewEncoder(w).Encode(limits)
		case strings.HasSuffix(r.URL.Path, "/setUploadLimit"):
			r.ParseForm()
			var value int64
			fmt.Sscan(r.Form.Get("limit"), &value)
			for _, hash := range strings.Split(r.Form.Get("hashes"), "|") {
				limits[hash] = value
			}
			fmt.Fprint(w, "{}")
		default:
			fmt.Fprint(w, "[]")
		}
	}))
	defer server.Close()
	c := defaultConfig()
	c.Clients = []Client{{ID: "fixture", Name: "Fixture", Kind: "qb", URL: server.URL, Enabled: true, Pool: "pool"}}
	c.Pools = []Pool{{ID: "pool", Name: "Fixture", Budget: 1000000, Enabled: true}}
	c.Sites = []Site{{ID: "site", Name: "Fixture", Domains: []string{"a.example"}, Weight: 7, Minimum: 50000, Maximum: 100000}}
	if err := s.saveConfig(c); err != nil {
		t.Fatal(err)
	}
	s.db.Exec("INSERT OR REPLACE INTO settings VALUES('guardian',?)", time.Now().Unix())
	engine := newEngine(s)
	for _, scenario := range []struct {
		budget, want int64
		warning      bool
	}{{1000000, 100000, false}, {600000, 75000, false}, {300000, 50000, true}, {1000000, 100000, false}} {
		c.Pools[0].Budget = scenario.budget
		if err := s.saveConfig(c); err != nil {
			t.Fatal(err)
		}
		engine.tick(context.Background())
		mu.Lock()
		if len(limits) != 8 {
			mu.Unlock()
			t.Fatal("not all torrents allocated")
		}
		for _, limit := range limits {
			if limit != scenario.want {
				mu.Unlock()
				t.Fatalf("budget%d: per-torrent limit%d, want%d", scenario.budget, limit, scenario.want)
			}
		}
		mu.Unlock()
		if (len(engine.warnings) > 0) != scenario.warning {
			t.Fatal("outlet warning does not match actual minimum feasibility")
		}
	}
}

func TestContinuousAlertSentOnceAndResolvedRetriesCancelled(t *testing.T) {
	s := testStore(t)
	calls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(200) }))
	defer target.Close()
	c := defaultConfig()
	c.Notify = NotifyConfig{Enabled: true, Alerts: true, URL: target.URL, Method: "POST", BodyType: "json", BodyTemplate: `{"title":"{{title}}","content":"{{content}}"}`}
	if err := s.saveConfig(c); err != nil {
		t.Fatal(err)
	}
	e := newEngine(s)
	e.warnings = []string{"出口 Home：最低额度合计 1.25 MB/s，超过预算 1 MB/s"}
	sv := &Server{s: s, e: e}
	now := time.Now()
	sv.notificationTick(context.Background(), now)
	restarted := &Server{s: s, e: e}
	restarted.notificationTick(context.Background(), now.Add(16*time.Minute))
	if calls != 1 {
		t.Fatal("continuous alert resent after cooldown or restart")
	}
	s.db.Exec("INSERT INTO notification_jobs VALUES('alert:legacy','alert','Old','Old',?,?,0,'pending',0,'')", now.Unix(), now.Unix())
	e.warnings = nil
	restarted.notificationTick(context.Background(), now.Add(17*time.Minute))
	if calls != 1 {
		t.Fatal("resolved legacy retry sent")
	}
	e.warnings = []string{"出口 Home：最低额度合计 1.30 MB/s，超过预算 1 MB/s"}
	restarted.notificationTick(context.Background(), now.Add(18*time.Minute))
	if calls != 2 {
		t.Fatal("new incident did not notify")
	}
}

func TestAlertIncidentPersistenceResolutionAndRecurrence(t *testing.T) {
	s := testStore(t)
	now := time.Now()
	msg := "出口 Home1：最低额度合计 1.25 MB/s，超过预算 1 MB/s；最低额度保持不变"
	if err := s.syncAlertEpisodes([]string{msg}, now); err != nil {
		t.Fatal(err)
	}
	// Include a numeric fluctuation and a new scheduler after restart: no new job.
	if err := s.syncAlertEpisodes([]string{strings.Replace(msg, "1.25", "1.30", 1)}, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM notification_jobs").Scan(&count)
	if count != 1 {
		t.Fatal("continuous incident resent")
	}
	s.db.Exec("INSERT INTO notification_jobs VALUES('alert:legacy:1','alert','Old','Old',1,1,1,'pending',0,'')")
	if err := s.syncAlertEpisodes(nil, now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	s.db.QueryRow("SELECT COUNT(*) FROM notification_jobs WHERE state='pending'").Scan(&count)
	if count != 0 {
		t.Fatal("resolved or legacy alert still retrying")
	}
	if err := s.syncAlertEpisodes([]string{msg}, now.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	s.db.QueryRow("SELECT COUNT(*) FROM notification_jobs WHERE state='pending'").Scan(&count)
	if count != 1 {
		t.Fatal("recurring incident not queued")
	}
	if alertIdentity("下载器连接异常：NAS1") == alertIdentity("下载器连接异常：NAS2") {
		t.Fatal("distinct downloader identities merged")
	}
}
