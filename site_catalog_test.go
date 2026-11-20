package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCatalogLiteralParsing(t *testing.T) {
	src := `export const siteMetadata: ISiteMetadata = {
  id: "fixture",
  name: "Fixture Site",
  urls: ["https://site.example/"],
  trackerUrls: ["uggcf://genpxre.rknzcyr/", // "https://comment.example/"
    "https://tracker2.example/announce", "https://unsafe.example/?passkey=secret"],
};`
	e, ok := parseCatalogDefinition(src)
	if !ok || e.ID != "fixture" || e.Name != "Fixture Site" || strings.Join(e.Domains, ",") != "tracker.example,tracker2.example,site.example" {
		t.Fatalf("literal metadata extraction mismatch: %#v", e)
	}
	src = strings.Replace(src, `urls: ["https://site.example/"]`, `urls: [executeCode()]`, 1)
	e, ok = parseCatalogDefinition(src)
	if !ok || len(e.Domains) != 2 {
		t.Fatal("dynamic expressions were executed or guessed")
	}
	src = strings.Replace(src, `  id:`, "  isDead: true,\n  id:", 1)
	if _, ok = parseCatalogDefinition(src); ok {
		t.Fatal("dead site included")
	}
}
func TestCatalogDiscoveryPreservesManualAndHistoryIdentity(t *testing.T) {
	entry := CatalogEntry{ID: "fixture", Name: "Official name", Domains: []string{"a.example", "b.example"}}
	index := map[string]CatalogEntry{"a.example": entry, "b.example": entry}
	torrents := []Torrent{{Trackers: []string{"https://a.example/secret", "https://b.example/another"}}}
	c, n := discoverSitesWithCatalog(defaultConfig(), torrents, index)
	if n != 2 || len(c.Sites) != 1 || len(c.Sites[0].Domains) != 2 || c.Sites[0].Name != entry.Name {
		t.Fatal("new aliases not grouped")
	}
	oldID := c.Sites[0].ID
	c.Sites[0].Name = "Custom name"
	c.Sites[0].Weight = 2
	c, _ = discoverSitesWithCatalog(c, torrents, index)
	if c.Sites[0].ID != oldID || c.Sites[0].Name != "Custom name" || c.Sites[0].Weight != 2 {
		t.Fatal("existing identity/policy overwritten")
	}
	manual := Site{ID: "manual", Name: "User group", Domains: []string{"a.example"}, Weight: 1}
	c = defaultConfig()
	c.Sites = []Site{manual}
	c, _ = discoverSitesWithCatalog(c, torrents, index)
	id, amb := siteFor(torrents[0], c)
	if amb || id != "manual" || c.Sites[0].Name != "User group" {
		t.Fatal("manual rule lost priority")
	}
	c = defaultConfig()
	c.Sites = []Site{{ID: "historical-a", Name: "a.example", Domains: []string{"a.example"}, Automatic: true, Weight: 3}, {ID: "historical-b", Name: "Custom", Domains: []string{"b.example"}, Automatic: true, Weight: 7}}
	c, _ = discoverSitesWithCatalog(c, torrents, index)
	if len(c.Sites) != 2 || c.Sites[0].ID != "historical-a" || c.Sites[1].Weight != 7 {
		t.Fatal("old groups merged or policy rewritten")
	}
}
func TestCatalogConflictsAndSubdomains(t *testing.T) {
	s := testStore(t)
	c := newSiteCatalog(s)
	c.state.Entries = []CatalogEntry{{ID: "a", Name: "A", Domains: []string{"a.example", "shared.example"}}, {ID: "b", Name: "B", Domains: []string{"shared.example"}}}
	if _, ok := catalogLookup(c.index(), "shared.example"); ok {
		t.Fatal("conflicting domain guessed")
	}
	if e, ok := catalogLookup(c.index(), "tracker.a.example"); !ok || e.ID != "a" {
		t.Fatal("subdomain match failed")
	}
	if _, ok := catalogLookup(c.index(), "nota.example"); ok {
		t.Fatal("domain boundary ignored")
	}
	if err := c.setAuto(false); err != nil {
		t.Fatal(err)
	}
	next := newSiteCatalog(s)
	if next.snapshot().Auto == nil || *next.snapshot().Auto {
		t.Fatal("catalog preference not persisted")
	}
}
func TestDownloaderDisablePreservesManualLimitsAndRetriesOwned(t *testing.T) {
	s := testStore(t)
	cl, m := mockClient(t)
	cl.Pool = "pool"
	c := defaultConfig()
	c.Clients = []Client{cl}
	c.Pools = []Pool{{ID: "pool", Name: "Fixture", Budget: 1000000, Enabled: true}}
	c.Sites = []Site{{ID: "site", Name: "Fixture", Domains: []string{"a.example", "b.example"}, Weight: 1}}
	s.saveConfig(c)
	s.db.Exec("INSERT OR REPLACE INTO settings VALUES('guardian',?)", time.Now().Unix())
	e := newEngine(s)
	e.tick(context.Background())
	if m.limit == 0 {
		t.Fatal("fixture not controlled")
	}
	sv := &Server{e: e, s: s}
	req := httptest.NewRequest("POST", "/api/client-enabled", strings.NewReader(`{"id":"fixture","enabled":false}`))
	w := httptest.NewRecorder()
	sv.clientEnabled(w, req)
	if w.Code != 200 || m.limit != 0 {
		t.Fatal("owned limit not restored on disable")
	}
	e.tick(context.Background())
	if len(e.torrents) != 0 {
		t.Fatal("disabled downloader still collected")
	}
	saved, _ := s.config()
	if saved.Clients[0].Enabled {
		t.Fatal("disabled switch lost")
	}
	m.limit = 123456
	w = httptest.NewRecorder()
	sv.clientEnabled(w, httptest.NewRequest("POST", "/api/client-enabled", strings.NewReader(`{"id":"fixture","enabled":false}`)))
	if m.limit != 123456 {
		t.Fatal("disable erased unowned manual limit")
	}
	// Retain offline owned work and prevent premature re-enable.
	s.db.Exec("INSERT OR REPLACE INTO leases(k,client,hash,original,last,planned,state,at) VALUES('fixture:abc','fixture','abc',0,123456,123456,'confirmed',?)", time.Now().Unix())
	saved, _ = s.config()
	saved.Clients[0].URL = "http://127.0.0.1:1"
	s.saveConfig(saved)
	w = httptest.NewRecorder()
	sv.clientEnabled(w, httptest.NewRequest("POST", "/api/client-enabled", strings.NewReader(`{"id":"fixture","enabled":false}`)))
	saved, _ = s.config()
	if !saved.Clients[0].RestorePending || saved.Clients[0].Enabled {
		t.Fatal("offline recovery intent lost")
	}
	w = httptest.NewRecorder()
	sv.clientEnabled(w, httptest.NewRequest("POST", "/api/client-enabled", strings.NewReader(`{"id":"fixture","enabled":true}`)))
	if w.Code != 409 {
		t.Fatal("enabled with pending restoration")
	}
	saved.Clients[0].URL = cl.URL
	s.saveConfig(saved)
	e.tick(context.Background())
	saved, _ = s.config()
	if saved.Clients[0].RestorePending || m.limit != 0 {
		t.Fatal("pending recovery not retried")
	}
}
func TestBundledCatalogValid(t *testing.T) {
	var s CatalogState
	if json.Unmarshal(bundledCatalog, &s) != nil || len(s.Entries) < 100 || len(s.Version) != 40 {
		t.Fatal("missing bundled public catalog")
	}
	for _, e := range s.Entries {
		for _, h := range e.Domains {
			if strings.ContainsAny(h, "/:?@") {
				t.Fatal("catalog contains full URLs or secrets")
			}
		}
	}
}

type catalogTransport func(*http.Request) (*http.Response, error)

func (f catalogTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCatalogFailedUpdateKeepsLastGoodData(t *testing.T) {
	s := testStore(t)
	c := newSiteCatalog(s)
	before := c.snapshot()
	client := &http.Client{Transport: catalogTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("unavailable")), Header: make(http.Header)}, nil
	})}
	if c.updateWithClient(context.Background(), client) == nil {
		t.Fatal("failure was ignored")
	}
	after := c.snapshot()
	if after.Version != before.Version || len(after.Entries) != len(before.Entries) || after.Error == "" || after.Checked == 0 {
		t.Fatal("failure discarded cache or not visible")
	}
	restored := newSiteCatalog(s)
	if restored.snapshot().Error == "" {
		t.Fatal("update error not persisted")
	}
}
func TestCatalogSameVersionSkipsArchiveDownload(t *testing.T) {
	s := testStore(t)
	c := newSiteCatalog(s)
	before := c.snapshot()
	calls := 0
	client := &http.Client{Transport: catalogTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "api.github.com" {
			t.Fatal("unchanged version downloaded archive")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"sha":"` + before.Version + `"}`)), Header: make(http.Header)}, nil
	})}
	if err := c.updateWithClient(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || c.snapshot().Version != before.Version || len(c.snapshot().Entries) != len(before.Entries) {
		t.Fatal("unchanged catalog replaced")
	}
}

func TestSnapshotDownloaderStatistics(t *testing.T) {
	s := testStore(t)
	now := time.Now().Unix()
	s.db.Exec("INSERT INTO traffic(k,site,at,up,down,uncertain) VALUES('one:abc','a',?,100,50,0),('one:def','b',?,200,70,1),('two:abc','a',?,400,90,0)", now, now, now)
	sv := &Server{s: s, e: newEngine(s)}
	w := httptest.NewRecorder()
	sv.snapshot(w, httptest.NewRequest("GET", "/api/snapshot?period=day&group=downloaders&view=none", nil))
	if w.Code != 200 {
		t.Fatal("snapshot failed")
	}
	var payload struct {
		DownloaderStats []struct {
			ID        string `json:"id"`
			Up        int64  `json:"up"`
			Down      int64  `json:"down"`
			Uncertain bool   `json:"uncertain"`
		} `json:"downloaderStats"`
	}
	if json.Unmarshal(w.Body.Bytes(), &payload) != nil || len(payload.DownloaderStats) != 2 {
		t.Fatal("missing downloader totals")
	}
	for _, v := range payload.DownloaderStats {
		if v.ID == "one" && (v.Up != 300 || v.Down != 120 || !v.Uncertain) {
			t.Fatal("downloader totals inaccurate")
		}
		if v.ID == "two" && (v.Up != 400 || v.Uncertain) {
			t.Fatal("downloader identities mixed")
		}
	}
}
