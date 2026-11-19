package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLargeQbListBoundsTrackerRequests(t *testing.T) {
	queries := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/torrents/info" {
			rows := []map[string]any{}
			for i := 0; i < 1000; i++ {
				rows = append(rows, map[string]any{"hash": fmt.Sprintf("%x", i), "name": "Fixture", "tracker": "https://a.example/fixture"})
			}
			json.NewEncoder(w).Encode(rows)
			return
		}
		queries++
		json.NewEncoder(w).Encode([]map[string]string{{"url": "https://a.example/fixture"}})
	}))
	defer server.Close()
	a := adapter(Client{ID: "fixture", Kind: "qb", URL: server.URL})
	for round := 1; round <= 2; round++ {
		rows, err := a.list(context.Background())
		if err != nil || len(rows) != 1000 {
			t.Fatal("list failed", err)
		}
		if queries != 32*round {
			t.Fatal("tracker work was unbounded or cache failed")
		}
		for _, r := range rows {
			if len(r.Hosts) == 0 {
				t.Fatal("primary tracker lost while indexing")
			}
		}
	}
}
func TestChangedPrimaryTrackerInvalidatesCache(t *testing.T) {
	primary := "https://a.example/fixture"
	queries := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/torrents/info" {
			json.NewEncoder(w).Encode([]map[string]any{{"hash": "abc", "name": "Fixture", "tracker": primary}})
			return
		}
		queries++
		json.NewEncoder(w).Encode([]map[string]string{{"url": primary}})
	}))
	defer server.Close()
	a := adapter(Client{ID: "fixture", Kind: "qb", URL: server.URL})
	a.list(context.Background())
	primary = "https://b.example/fixture"
	rows, err := a.list(context.Background())
	if err != nil || queries != 2 || len(rows) != 1 || len(rows[0].Hosts) != 1 || rows[0].Hosts[0] != "b.example" {
		t.Fatal("changed tracker used stale attribution")
	}
}
func TestAdapterCacheSurvivesStrategyAndDisplayChanges(t *testing.T) {
	e := newEngine(testStore(t))
	on := true
	cl := Client{ID: "fixture", Kind: "qb", URL: "http://fixture.example", StrategyEnabled: &on}
	a := e.get(cl)
	otherOn := true
	cl.StrategyEnabled = &otherOn
	cl.ClearPending = true
	cl.Name = "Renamed"
	cl.Pool = "other"
	if e.get(cl) != a {
		t.Fatal("non-transport config discarded cache")
	}
	cl.Password = "fixture-new"
	if e.get(cl) == a {
		t.Fatal("credentials did not invalidate adapter")
	}
}
