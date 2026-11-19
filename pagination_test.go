package main

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
)

func TestSnapshotPaginatesLargeListsAndKeepsTotals(t *testing.T) {
	s := testStore(t)
	e := newEngine(s)
	sv := &Server{s: s, e: e}
	for i := 0; i < 20000; i++ {
		e.torrents = append(e.torrents, Torrent{Key: fmt.Sprint(i), Name: fmt.Sprintf("Torrent %05d", i), Site: "site", Up: 100, Connections: 1, Hosts: []string{"a.example"}})
	}
	rec := httptest.NewRecorder()
	sv.snapshot(rec, httptest.NewRequest("GET", "/api/snapshot?view=active&offset=50", nil))
	var v struct {
		Torrents []Torrent        `json:"torrents"`
		Total    int              `json:"torrentTotal"`
		Rates    map[string]int64 `json:"siteRates"`
	}
	if json.Unmarshal(rec.Body.Bytes(), &v) != nil || rec.Code != 200 {
		t.Fatal("snapshot failed")
	}
	if v.Total != 20000 || len(v.Torrents) != 50 || v.Torrents[0].Key != "50" || v.Rates["site"] != 2000000 {
		t.Fatal("pagination lost records or global rates")
	}
	rec = httptest.NewRecorder()
	sv.snapshot(rec, httptest.NewRequest("GET", "/api/snapshot?view=all&search=Torrent%2019999", nil))
	json.Unmarshal(rec.Body.Bytes(), &v)
	if v.Total != 1 || len(v.Torrents) != 1 {
		t.Fatal("server-side search failed")
	}
}
