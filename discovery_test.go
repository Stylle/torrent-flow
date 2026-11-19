package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDiscoveryGroupsHostsWithoutSecretsAndPreservesNames(t *testing.T) {
	c := defaultConfig()
	c.Sites = []Site{{ID: "manual", Name: "Existing", Domains: []string{"manual.example"}, Weight: 3}, {ID: "regex", Name: "Custom", Regex: `special\.example/custom/`, Weight: 2}}
	torrents := []Torrent{{Trackers: []string{"https://a.example/private-key/announce", "https://a.example/announce?passkey=another-private-key", "udp://b.example:80/announce", "https://sub.manual.example/key", "https://special.example/custom/key", "DHT"}}}
	next, n := discoverSites(c, torrents)
	if n != 2 || len(next.Sites) != 4 || len(c.Sites) != 2 {
		t.Fatal("discovery duplicated aliases, replaced manual rules, or mutated input")
	}
	encoded, _ := json.Marshal(next)
	if strings.Contains(string(encoded), "private-key") {
		t.Fatal("discovery persisted tracker path or passkey")
	}
	id, amb := siteFor(Torrent{Trackers: []string{"https://a.example/secret"}}, next)
	if id == "" || amb {
		t.Fatal("single discovered site not classified")
	}
	for i := range next.Sites {
		if next.Sites[i].ID == id {
			next.Sites[i].Name = "Renamed site"
		}
	}
	again, n := discoverSites(next, torrents)
	if n != 0 || len(again.Sites) != 4 {
		t.Fatal("discovery not idempotent")
	}
	for _, s := range again.Sites {
		if s.ID == id && s.Name != "Renamed site" {
			t.Fatal("rename lost")
		}
	}
	multi := Torrent{Key: "fixture:abc", Trackers: []string{"udp://b.example/secret", "https://a.example/secret"}}
	primary, _ := siteFor(Torrent{Trackers: multi.Trackers[:1]}, next)
	if got, amb := siteFor(multi, next); got != primary || amb {
		t.Fatal("automatic multi-tracker ownership did not use first tracker")
	}
	next.Overrides[multi.Key] = Override{Site: id}
	if got, _ := siteFor(multi, next); got != id {
		t.Fatal("manual torrent owner did not override discovery")
	}
}

func TestAutomaticControlRequiresOptInAndRespectsExclusions(t *testing.T) {
	s := testStore(t)
	cl, m := mockClient(t)
	cl.Pool = "pool"
	c := defaultConfig()
	c.Clients = []Client{cl}
	c.Pools = []Pool{{ID: "pool", Name: "Example", Budget: 100000, Enabled: true}}
	s.saveConfig(c)
	s.db.Exec("INSERT OR REPLACE INTO settings VALUES('guardian',?)", time.Now().Unix())
	e := newEngine(s)
	e.tick(context.Background())
	c, _ = s.config()
	if len(c.Sites) == 0 || len(e.torrents) != 1 || e.torrents[0].Site == "" || m.limit != 0 {
		t.Fatal("discovery failed or enabled control without consent")
	}
	revision := c.Revision
	c.Clients[0].AutoManage = true
	s.saveConfig(c)
	e.tick(context.Background())
	if m.limit == 0 || !e.torrents[0].Managed {
		t.Fatal("downloader automatic opt-in did not allocate")
	}
	current, _ := s.config()
	if current.Revision != revision {
		t.Fatal("discovery rewrites configuration every tick")
	}
	c.Overrides[e.torrents[0].Key] = Override{Manage: false}
	s.saveConfig(c)
	e.tick(context.Background())
	if m.limit != 0 || e.torrents[0].Managed {
		t.Fatal("explicit per-torrent exclusion did not restore limit")
	}
}
