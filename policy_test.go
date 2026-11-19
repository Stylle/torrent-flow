package main

import (
	"context"
	"testing"
	"time"
)

func TestPolicyWeightedSharesAndFixedFloors(t *testing.T) {
	cases := []struct {
		name     string
		budget   int64
		items    []Allocation
		floors   map[string]int64
		want     map[string]int64
		exceeded bool
	}{
		{"one", 1000000, []Allocation{{"a", 10, 0, 0}}, map[string]int64{"a": 50000}, map[string]int64{"a": 1000000}, false},
		{"one-two-two", 1000000, []Allocation{{"a", 1, 0, 0}, {"b", 2, 0, 0}, {"c", 2, 0, 0}}, map[string]int64{"a": 50000, "b": 50000, "c": 100000}, map[string]int64{"a": 200000, "b": 400000, "c": 400000}, false},
		{"nineteen-and-one", 1000000, []Allocation{{"a", 10, 0, 0}, {"b", 10, 0, 0}}, map[string]int64{"a": 50000, "b": 950000}, map[string]int64{"a": 50000, "b": 950000}, false},
		{"over-budget", 1000000, []Allocation{{"a", 10, 0, 0}, {"b", 10, 0, 0}}, map[string]int64{"a": 50000, "b": 1200000}, map[string]int64{"a": 50000, "b": 1200000}, true},
		{"cap-redistribution", 1000000, []Allocation{{"a", 10, 0, 100000}, {"b", 10, 0, 0}}, map[string]int64{"a": 50000, "b": 50000}, map[string]int64{"a": 100000, "b": 900000}, false},
	}
	for _, v := range cases {
		t.Run(v.name, func(t *testing.T) {
			got, over := allocateSiteShares(v.budget, v.items, v.floors)
			if over != v.exceeded {
				t.Fatal("warning mismatch")
			}
			for id, n := range v.want {
				if got[id] != n {
					t.Fatalf("%s got %d want %d", id, got[id], n)
				}
			}
		})
	}
}
func TestExplicitStopClearsUnownedLimit(t *testing.T) {
	s := testStore(t)
	cl, m := mockClient(t)
	m.limit = 123000
	c := defaultConfig()
	c.Clients = []Client{cl}
	s.saveConfig(c)
	e := newEngine(s)
	if err := e.clearAll(context.Background(), c); err != nil || m.limit != 0 {
		t.Fatal("unowned limit not cleared", err)
	}
	m.limit = 456000
	m.reject = true
	off := false
	c.PolicyEnabled = &off
	c.ClearPending = true
	s.saveConfig(c)
	e.tick(context.Background())
	pending, _ := s.config()
	if !pending.ClearPending {
		t.Fatal("failed clear task discarded")
	}
	m.reject = false
	s.db.Exec("INSERT OR REPLACE INTO settings VALUES('guardian',?)", time.Now().Unix())
	e.tick(context.Background())
	done, _ := s.config()
	if done.ClearPending || m.limit != 0 {
		t.Fatal("pending clear failed after reconnect")
	}
}

func TestDownloaderSwitchIsIndependentAndMonitoringContinues(t *testing.T) {
	s := testStore(t)
	a, ma := mockClient(t)
	b, mb := mockClient(t)
	b.ID = "second"
	a.Pool = "home"
	b.Pool = "home"
	c := defaultConfig()
	c.Clients = []Client{a, b}
	c.Pools = []Pool{{ID: "home", Name: "Shared", Budget: 1000000, Enabled: true}}
	s.saveConfig(c)
	s.db.Exec("INSERT OR REPLACE INTO settings VALUES('guardian',?)", time.Now().Unix())
	e := newEngine(s)
	e.tick(context.Background())
	if ma.limit != 500000 || mb.limit != 500000 {
		t.Fatal("downloaders did not share outlet budget")
	}
	c, _ = s.config()
	off := false
	c.Clients[0].StrategyEnabled = &off
	s.saveConfig(c)
	ma.uploaded += 123
	e.tick(context.Background())
	if ma.limit != 0 || mb.limit != 1000000 {
		t.Fatal("stopping one strategy changed the wrong downloader")
	}
	var up int64
	s.db.QueryRow("SELECT COALESCE(SUM(up),0) FROM traffic WHERE k='fixture:abc'").Scan(&up)
	if up != 123 {
		t.Fatal("stopped strategy disabled statistics")
	}
}

func TestActualUploadIsActiveEvenIfPeerCountMissing(t *testing.T) {
	s := testStore(t)
	cl, m := mockClient(t)
	m.connections = 0
	m.forceTraffic = true
	c := defaultConfig()
	c.Clients = []Client{cl}
	s.saveConfig(c)
	s.db.Exec("INSERT OR REPLACE INTO settings VALUES('guardian',?)", time.Now().Unix())
	e := newEngine(s)
	e.tick(context.Background())
	if len(e.torrents) != 1 || !e.torrents[0].Managed || m.limit <= 0 {
		t.Fatal("active upload hidden when peer count is zero")
	}
}
