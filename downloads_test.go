package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDownloaderDownloadStateAndProgress(t *testing.T) {
	for _, kind := range []string{"qb", "tr"} {
		t.Run(kind, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if kind == "qb" {
					if strings.HasSuffix(r.URL.Path, "/info") {
						w.Write([]byte(`[{"hash":"one","name":"active","state":"downloading","progress":0.42},{"hash":"two","name":"waiting","state":"stalledDL","progress":0},{"hash":"three","name":"paused","state":"pausedDL","progress":0.5},{"hash":"four","name":"seeding","state":"uploading","progress":1}]`))
					} else {
						w.Write([]byte(`[]`))
					}
				} else {
					w.Write([]byte(`{"result":"success","arguments":{"torrents":[{"hashString":"one","name":"active","status":4,"percentDone":0.42},{"hashString":"two","name":"waiting","status":4,"percentDone":0},{"hashString":"three","name":"paused","status":0,"percentDone":0.5},{"hashString":"four","name":"seeding","status":6,"percentDone":1}]}}`))
				}
			}))
			defer server.Close()
			a := adapter(Client{ID: "fixture", Kind: kind, URL: server.URL})
			items, err := a.list(context.Background())
			if err != nil || len(items) != 4 {
				t.Fatal("fixture list failed", err)
			}
			if !items[0].Downloading || !items[1].Downloading || items[2].Downloading || items[3].Downloading || items[0].Progress == nil || *items[0].Progress != 0.42 || items[1].Progress == nil || *items[1].Progress != 0 {
				t.Fatal("download status/progress mismatch")
			}
		})
	}
}
func TestDownloadSnapshotSeparateBoundedPagination(t *testing.T) {
	s := testStore(t)
	e := newEngine(s)
	for i := 0; i < 130; i++ {
		e.torrents = append(e.torrents, Torrent{Key: randomID(), Name: "Download", Downloading: true, Down: 1000, Trackers: []string{"https://a.example/secret"}})
	}
	e.torrents = append(e.torrents, Torrent{Key: "seed:only", Name: "Seed only", Up: 90000, Connections: 1})
	sv := &Server{s: s, e: e}
	w := httptest.NewRecorder()
	sv.snapshot(w, httptest.NewRequest("GET", "/api/snapshot?view=active&downloadOffset=100", nil))
	var v struct {
		Downloading      []Torrent `json:"downloading"`
		DownloadingTotal int       `json:"downloadingTotal"`
		DownloadOffset   int       `json:"downloadOffset"`
		DownloadRate     int64     `json:"downloadRate"`
		Torrents         []Torrent `json:"torrents"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &v) != nil || len(v.Downloading) != 30 || v.DownloadingTotal != 130 || v.DownloadOffset != 100 || v.DownloadRate != 130000 || len(v.Torrents) != 1 {
		t.Fatal("download pagination or upload list separation failed")
	}
	if strings.Contains(w.Body.String(), "secret") {
		t.Fatal("tracker path leaked")
	}
}

func TestUnifiedActivitySnapshot(t *testing.T) {
	s := testStore(t)
	c, err := s.config()
	if err != nil {
		t.Fatal(err)
	}
	c.UploadConfirmSeconds = 15
	c.Clients = []Client{{ID: "qb", Enabled: true}}
	if err := s.saveConfig(c); err != nil {
		t.Fatal(err)
	}
	e := newEngine(s)
	e.torrents = []Torrent{
		{Key: "both", Client: "qb", Downloading: true, Down: 200, Up: 60, Managed: true, UploadQualified: true},
		{Key: "waiting", Client: "qb", Downloading: true},
		{Key: "confirm", Client: "qb", State: "monitoring", UploadObserved: true, Trackers: []string{"https://a.example/secret"}},
		{Key: "recover", Client: "qb", State: "release pending"},
		{Key: "idle", Client: "qb", Connections: 2},
	}
	for i := 0; i < 50; i++ {
		e.torrents = append(e.torrents, Torrent{Key: randomID(), Client: "qb", Downloading: true, Down: 1})
	}
	sv := &Server{s: s, e: e}
	get := func(query string) (v struct {
		Summary activeTorrentSummary `json:"activity"`
		Rows    []activeTorrent      `json:"torrents"`
		Total   int                  `json:"torrentTotal"`
		Offset  int                  `json:"torrentOffset"`
	}) {
		t.Helper()
		w := httptest.NewRecorder()
		sv.snapshot(w, httptest.NewRequest("GET", "/api/snapshot?view=overview"+query, nil))
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &v) != nil || strings.Contains(w.Body.String(), "secret") {
			t.Fatal("invalid activity snapshot")
		}
		return v
	}
	all := get("")
	want := activeTorrentSummary{Count: 54, Downloading: 52, Uploading: 2, Up: 60, Down: 250}
	if all.Summary != want || all.Total != 54 || len(all.Rows) != 50 || all.Rows[0].Key != "both" {
		t.Fatalf("wrong activity totals or ordering: %+v", all.Summary)
	}
	upload := get("&activity=uploading&offset=50")
	if upload.Summary != want || upload.Offset != 0 || len(upload.Rows) != 2 ||
		upload.Rows[0].Key != "both" || !upload.Rows[1].AwaitingUpload {
		t.Fatal("upload filter did not retain overlap or confirmation state")
	}
	download := get("&activity=downloading&offset=50")
	if download.Total != 52 || download.Offset != 50 || len(download.Rows) != 2 || download.Rows[1].Key != "waiting" {
		t.Fatal("download filter or last page lost an active download")
	}
	tail := get("&offset=50")
	if len(tail.Rows) != 4 {
		t.Fatalf("last page returned %d rows", len(tail.Rows))
	}
}
