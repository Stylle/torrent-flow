package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, e := openStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.db.Close() })
	return s
}
func TestAllocationInvariants(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	for trial := 0; trial < 500; trial++ {
		var items []Allocation
		var minSum int64
		for i := 0; i < 1+r.Intn(40); i++ {
			min := int64(r.Intn(1000))
			minSum += min
			items = append(items, Allocation{fmt.Sprint(i), int64(1 + r.Intn(20)), min, min + int64(r.Intn(10000))})
		}
		budget := minSum + int64(r.Intn(100000))
		v := allocate(budget, items)
		var sum int64
		for _, i := range items {
			if v[i.ID] < i.Minimum || v[i.ID] > i.Maximum {
				t.Fatal("allocation outside allowed bounds")
			}
			sum += v[i.ID]
		}
		if sum > budget {
			t.Fatal("budget exceeded")
		}
	}
	if len(allocate(5, []Allocation{{"one", 1, 10, 0}})) != 0 {
		t.Fatal("infeasible floor accepted")
	}
	v := allocate(100, []Allocation{{"a", 3, 0, 10}, {"b", 1, 0, 0}})
	if v["a"] != 10 || v["b"] != 90 {
		t.Fatal(v)
	}
}
func TestDomainAndRegex(t *testing.T) {
	c := defaultConfig()
	c.Sites = []Site{{ID: "s", Domains: []string{"a.example", "b.example"}}}
	v := Torrent{Trackers: []string{"https://a.example/different/secret", "https://b.example/announce?passkey=fixture"}}
	if id, amb := siteFor(v, c); id != "s" || amb {
		t.Fatal("same site aliases not deduplicated")
	}
	v.Trackers = []string{"https://not-a.example/x"}
	if id, _ := siteFor(v, c); id != "" {
		t.Fatal("suffix collision")
	}
	c.Sites = append(c.Sites, Site{ID: "other", Regex: `a\.example/`})
	v.Trackers = []string{"https://a.example/x"}
	if _, amb := siteFor(v, c); !amb {
		t.Fatal("multiple owners not marked")
	}
	c.Overrides[v.Key] = Override{Site: "s"}
	if id, amb := siteFor(v, c); id != "s" || amb {
		t.Fatal("override ignored")
	}
}

func TestOldHourlyRowsPreserved(t *testing.T) {
	s := testStore(t)
	now := time.Date(2026, 11, 19, 1, 10, 0, 0, time.UTC)
	v := Torrent{Key: "fixture:abc", Site: "site", Uploaded: 100}
	s.ingest("fixture", []Torrent{v}, now, 5)
	s.db.Exec("INSERT INTO traffic(k,site,at,up,down,uncertain) VALUES(?,?,?,?,?,0)", v.Key, v.Site, now.Unix()-10, 15, 0)
	s.db.Exec("INSERT INTO traffic(k,site,at,up,down,uncertain) VALUES(?,?,?,?,?,0)", v.Key, v.Site, now.Unix()-5, 20, 0)
	v.Uploaded = 130
	if e := s.ingest("fixture", []Torrent{v}, now.Add(5*time.Second), 5); e != nil {
		t.Fatal(e)
	}
	var n, total int64
	s.db.QueryRow("SELECT COUNT(*),SUM(up) FROM traffic").Scan(&n, &total)
	if n != 2 || total != 65 {
		t.Fatal("existing historical rows were removed or totals changed")
	}
}

func TestHourlyRollupBoundsStorage(t *testing.T) {
	s := testStore(t)
	now := time.Date(2026, 11, 19, 1, 0, 0, 0, time.UTC)
	items := make([]Torrent, 1000)
	for i := range items {
		items[i] = Torrent{Key: fmt.Sprintf("fixture:%040d", i), Site: "site"}
	}
	start := time.Now()
	if e := s.ingest("fixture", items, now, 5); e != nil {
		t.Fatal(e)
	}
	for step := 1; step <= 24; step++ {
		for i := range items {
			items[i].Uploaded += 1024
		}
		if e := s.ingest("fixture", items, now.Add(time.Duration(step)*5*time.Second), 5); e != nil {
			t.Fatal(e)
		}
	}
	var rows, total int64
	s.db.QueryRow("SELECT COUNT(*),SUM(up) FROM traffic").Scan(&rows, &total)
	if rows != 1000 || total != 24*1000*1024 {
		t.Fatal("rollup changed accounting or did not bound rows")
	}
	t.Logf("1000 torrents, 24 delta polls: %d rows, %d bytes accounted, elapsed=%s", rows, total, time.Since(start))
}

func TestClientCounterNamespaceIsolation(t *testing.T) {
	s := testStore(t)
	now := time.Now()
	a := Torrent{Key: "first_one:abc", Uploaded: 100}
	b := Torrent{Key: "firstXone:abc", Uploaded: 100}
	if e := s.ingest("first_one", []Torrent{a}, now, 5); e != nil {
		t.Fatal(e)
	}
	if e := s.ingest("firstXone", []Torrent{b}, now, 5); e != nil {
		t.Fatal(e)
	}
	s.ingest("first_one", nil, now.Add(time.Second), 5)
	b.Uploaded = 130
	s.ingest("firstXone", []Torrent{b}, now.Add(5*time.Second), 5)
	out, e := s.report(now.Add(-time.Second), now.Add(time.Minute))
	if e != nil || len(out) != 1 || out[0].Up != 30 {
		t.Fatal("downloader namespaces interfered with cumulative baselines")
	}
}
func TestLedgerLifecycle(t *testing.T) {
	s := testStore(t)
	at := time.Date(2026, 11, 19, 1, 0, 0, 0, time.UTC)
	v := Torrent{Key: "client:abc", Site: "site", Uploaded: 100, Downloaded: 50, Added: 1}
	ingest := func(items []Torrent, now time.Time) {
		t.Helper()
		if e := s.ingest("client", items, now, 5); e != nil {
			t.Fatal(e)
		}
	}
	ingest([]Torrent{v}, at)
	v.Uploaded = 130
	v.Downloaded = 65
	ingest([]Torrent{v}, at.Add(5*time.Second))
	ingest([]Torrent{v}, at.Add(10*time.Second))
	b, e := s.report(at, at.Add(time.Hour))
	if e != nil || len(b) != 1 || b[0].Up != 30 || b[0].Down != 15 {
		t.Fatal("duplicate or baseline counted", b, e)
	}
	v.Uploaded = 140
	ingest([]Torrent{v}, at.Add(5*time.Minute))
	b, _ = s.report(at, at.Add(time.Hour))
	if b[0].Up != 40 || !b[0].Uncertain {
		t.Fatal("gap not flagged")
	}
	v.Uploaded = 1
	v.Downloaded = 1
	ingest([]Torrent{v}, at.Add(6*time.Minute))
	ingest(nil, at.Add(7*time.Minute))
	v.Uploaded = 1000
	ingest([]Torrent{v}, at.Add(8*time.Minute))
	b, _ = s.report(at, at.Add(time.Hour))
	if b[0].Up != 40 {
		t.Fatal("reset or re-add miscounted")
	}
	ingest(nil, at.AddDate(1, 0, 1))
	b, _ = s.report(at, at.Add(time.Hour))
	if len(b) != 0 {
		t.Fatal("expired traffic retained")
	}
}
func TestCalendarAndEncryptedConfig(t *testing.T) {
	s := testStore(t)
	c := defaultConfig()
	c.Clients = []Client{{ID: "fixture", Name: "fixture", Kind: "qb", URL: "http://127.0.0.1:1", Password: "fixture-private-value"}}
	if e := s.saveConfig(c); e != nil {
		t.Fatal(e)
	}
	var raw []byte
	s.db.QueryRow("SELECT v FROM settings WHERE k='config'").Scan(&raw)
	if strings.Contains(string(raw), c.Clients[0].Password) {
		t.Fatal("plaintext credential stored")
	}
	loaded, e := s.config()
	if e != nil || loaded.Clients[0].Password != c.Clients[0].Password {
		t.Fatal("encrypted round trip failed")
	}
	loc, _ := time.LoadLocation("America/New_York")
	day := time.Date(2026, 3, 8, 12, 0, 0, 0, loc)
	a, b := period(day, "day")
	if b.Sub(a) != 23*time.Hour {
		t.Fatal("DST day duration incorrect")
	}
	a, b = period(time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC), "week")
	if a.Weekday() != time.Monday || b.Sub(a) != 7*24*time.Hour {
		t.Fatal("week boundaries incorrect")
	}
}

type qbMock struct {
	mu          sync.Mutex
	limit       int64
	reject      bool
	connections int
	uploaded    int64
	writes      int
}

func (m *qbMock) serve(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.reject {
		http.Error(w, "unavailable", 503)
		return
	}
	r.ParseForm()
	switch r.URL.Path {
	case "/api/v2/torrents/uploadLimit":
		json.NewEncoder(w).Encode(map[string]int64{"abc": m.limit})
	case "/api/v2/torrents/setUploadLimit":
		fmt.Sscan(r.Form.Get("limit"), &m.limit)
		m.writes++
	case "/api/v2/torrents/info":
		json.NewEncoder(w).Encode([]map[string]any{{"hash": "abc", "name": "Fixture torrent", "uploaded": m.uploaded, "downloaded": 20, "upspeed": 3000, "num_leechs": m.connections, "up_limit": m.limit, "added_on": 1}})
	case "/api/v2/torrents/trackers":
		json.NewEncoder(w).Encode([]map[string]any{{"url": "https://a.example/fixture"}, {"url": "https://b.example/fixture"}})
	default:
		http.NotFound(w, r)
	}
}
func mockClient(t *testing.T) (Client, *qbMock) {
	t.Helper()
	m := &qbMock{connections: 2, uploaded: 100}
	server := httptest.NewServer(http.HandlerFunc(m.serve))
	t.Cleanup(server.Close)
	return Client{ID: "fixture", Name: "Fixture", Kind: "qb", URL: server.URL, Enabled: true}, m
}
func TestControlAndRecovery(t *testing.T) {
	s := testStore(t)
	cl, m := mockClient(t)
	c := defaultConfig()
	c.Clients = []Client{cl}
	e := newEngine(s)
	s.db.Exec("INSERT OR REPLACE INTO settings VALUES('guardian',?)", time.Now().Unix())
	torrent := Torrent{Key: "fixture:abc", Client: cl.ID, Hash: "abc"}
	ctx := context.Background()
	if err := e.apply(ctx, cl, torrent, 9000); err != nil {
		t.Fatal(err)
	}
	if m.limit != 9000 {
		t.Fatal("limit not sent")
	}
	if err := e.restore(ctx, c); err != nil || m.limit != 0 {
		t.Fatal("release failed", err)
	}
	if err := e.apply(ctx, cl, torrent, 7000); err != nil {
		t.Fatal(err)
	}
	m.limit = 4321
	if err := e.restore(ctx, c); err != nil || m.limit != 4321 {
		t.Fatal("external setting overwritten", err)
	}
	if !e.suspended[torrent.Key] {
		t.Fatal("external control not suspended")
	}
}
func TestCrashWindows(t *testing.T) {
	for _, v := range []struct {
		name, state                           string
		original, last, planned, actual, want int64
	}{{"first-never-sent", "prepared-new", 4000, 4000, 9000, 4000, 4000}, {"first-sent-no-ack", "prepared-new", 4000, 4000, 9000, 9000, 0}, {"update-never-sent", "prepared", 0, 7000, 9000, 7000, 0}, {"update-sent-no-ack", "prepared", 0, 7000, 9000, 9000, 0}, {"external-after-crash", "prepared", 0, 7000, 9000, 1234, 1234}} {
		t.Run(v.name, func(t *testing.T) {
			s := testStore(t)
			cl, m := mockClient(t)
			m.limit = v.actual
			c := defaultConfig()
			c.Clients = []Client{cl}
			s.db.Exec("INSERT INTO leases VALUES(?,?,?,?,?,?,?,?)", "fixture:abc", cl.ID, "abc", v.original, v.last, v.planned, v.state, time.Now().Unix())
			e := newEngine(s)
			if err := e.restore(context.Background(), c); err != nil {
				t.Fatal(err)
			}
			if m.limit != v.want {
				t.Fatal("unexpected crash recovery outcome")
			}
		})
	}
}
func TestOfflineRetainsRecoveryQueue(t *testing.T) {
	s := testStore(t)
	cl, m := mockClient(t)
	c := defaultConfig()
	c.Clients = []Client{cl}
	e := newEngine(s)
	s.db.Exec("INSERT OR REPLACE INTO settings VALUES('guardian',?)", time.Now().Unix())
	torrent := Torrent{Key: "fixture:abc", Client: cl.ID, Hash: "abc"}
	if err := e.apply(context.Background(), cl, torrent, 9000); err != nil {
		t.Fatal(err)
	}
	m.reject = true
	if err := e.restore(context.Background(), c); err == nil {
		t.Fatal("offline recovery falsely reported success")
	}
	if len(e.leaseKeys()) != 1 {
		t.Fatal("recovery record lost")
	}
	m.reject = false
	if err := e.restore(context.Background(), c); err != nil || m.limit != 0 {
		t.Fatal("reconnect recovery failed", err)
	}
}
func TestTransmissionNegotiationAndUnits(t *testing.T) {
	limited := true
	limit := int64(7)
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.Header.Get("X-Transmission-Session-Id") != "fixture-session" {
			w.Header().Set("X-Transmission-Session-Id", "fixture-session")
			w.WriteHeader(409)
			return
		}
		var v struct {
			Method string                     `json:"method"`
			Args   map[string]json.RawMessage `json:"arguments"`
		}
		json.NewDecoder(r.Body).Decode(&v)
		args := map[string]any{}
		if v.Method == "torrent-set" {
			json.Unmarshal(v.Args["uploadLimited"], &limited)
			json.Unmarshal(v.Args["uploadLimit"], &limit)
		} else {
			args["torrents"] = []map[string]any{{"uploadLimited": limited, "uploadLimit": limit}}
		}
		json.NewEncoder(w).Encode(map[string]any{"result": "success", "arguments": args})
	}))
	defer server.Close()
	a := adapter(Client{Kind: "tr", URL: server.URL})
	ctx := context.Background()
	got, err := a.readLimit(ctx, "abc")
	if err != nil || got != 7000 || attempts != 2 {
		t.Fatal("session/units failed")
	}
	if err = a.setLimit(ctx, "abc", 0); err != nil || limited {
		t.Fatal("unlimited not disabled")
	}
	if err = a.setLimit(ctx, "abc", 9500); err != nil || limit != 10 || !limited {
		t.Fatal("upload KB conversion failed")
	}
}
func TestQbDraftAuthentication(t *testing.T) {
	logged := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			r.ParseForm()
			if r.Form.Get("password") != "fixture" {
				http.Error(w, "no", 401)
				return
			}
			logged = true
			fmt.Fprint(w, "Ok.")
			return
		}
		if !logged {
			w.WriteHeader(403)
			return
		}
		fmt.Fprint(w, "[]")
	}))
	defer server.Close()
	a := adapter(Client{Kind: "qb", URL: server.URL, Password: "fixture"})
	if _, e := a.list(context.Background()); e != nil || !logged {
		t.Fatal("draft login failed", e)
	}
	if _, e := url.Parse(server.URL); e != nil {
		t.Fatal(e)
	}
}
