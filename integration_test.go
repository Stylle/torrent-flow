package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestConcurrentFirstStartup(t *testing.T) {
	for i := 0; i < 10; i++ {
		dir := t.TempDir()
		var wg sync.WaitGroup
		errors := make(chan error, 2)
		for j := 0; j < 2; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				s, e := openStore(dir)
				if e == nil {
					s.db.Close()
				}
				errors <- e
			}()
		}
		wg.Wait()
		close(errors)
		for e := range errors {
			if e != nil {
				t.Fatal("concurrent startup failed", e)
			}
		}
	}
}

func TestEngineIdleAndExternalSuspension(t *testing.T) {
	s := testStore(t)
	cl, m := mockClient(t)
	cl.Pool = "pool"
	c := defaultConfig()
	c.Clients = []Client{cl}
	c.Pools = []Pool{{ID: "pool", Name: "Fixture", Budget: 20000, Enabled: true}}
	c.Sites = []Site{{ID: "site", Name: "Fixture", Domains: []string{"a.example", "b.example"}, Weight: 1}}
	c.Overrides["fixture:abc"] = Override{Site: "site", Manage: true, Weight: 1}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	s.saveConfig(c)
	s.db.Exec("INSERT OR REPLACE INTO settings VALUES('guardian',?)", time.Now().Unix())
	e := newEngine(s)
	e.tick(context.Background())
	if m.limit != 50000 || len(e.torrents) != 1 || !e.torrents[0].Managed {
		t.Fatal("active torrent not allocated")
	}
	m.connections = 0
	e.idle["fixture:abc"] = time.Now().Add(-time.Minute)
	e.tick(context.Background())
	if m.limit != 0 || len(e.leaseKeys()) != 0 {
		t.Fatal("idle release did not restore unlimited")
	}
	m.connections = 2
	e.tick(context.Background())
	m.limit = 4321
	e.tick(context.Background())
	if m.limit != 4321 {
		t.Fatal("external change overwritten")
	}
	e2 := newEngine(s)
	e2.tick(context.Background())
	if m.limit != 4321 {
		t.Fatal("restart erased external-change protection")
	}
}
func TestGuardianSeparateConnectionRecovers(t *testing.T) {
	dir := t.TempDir()
	s, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	cl, m := mockClient(t)
	c := defaultConfig()
	c.Clients = []Client{cl}
	s.saveConfig(c)
	s.db.Exec("INSERT OR REPLACE INTO settings VALUES('guardian',?)", time.Now().Unix())
	e := newEngine(s)
	if err = e.apply(context.Background(), cl, Torrent{Key: "fixture:abc", Hash: "abc"}, 8000); err != nil {
		t.Fatal(err)
	}
	s.db.Exec("INSERT OR REPLACE INTO settings VALUES('controller',?)", time.Now().Add(-time.Minute).Unix())
	guardStore, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer guardStore.db.Close()
	g := newEngine(guardStore)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { g.guardian(ctx); close(done) }()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		limit := m.limit
		m.mu.Unlock()
		if limit == 0 && len(e.leaseKeys()) == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
	if m.limit != 0 || len(e.leaseKeys()) != 0 {
		t.Fatal("independent guardian failed recovery")
	}
}
func TestHTTPAuthCSRFAndSettings(t *testing.T) {
	s := testStore(t)
	t.Setenv("ADMIN_PASSWORD", "Fixture!Password123")
	if err := initAdmin(s, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	sv := &Server{s: s, e: newEngine(s), attempts: map[string][]time.Time{}}
	server := httptest.NewServer(sv.handler())
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	get := func(path string) int {
		r, e := client.Get(server.URL + path)
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		return r.StatusCode
	}
	if get("/api/snapshot") != 401 {
		t.Fatal("unauthenticated data exposed")
	}
	post := func(path string, body any, csrf, origin string) (int, map[string]any) {
		b, _ := json.Marshal(body)
		r, _ := http.NewRequest("POST", server.URL+path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		if csrf != "" {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		response, e := client.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		v := map[string]any{}
		json.NewDecoder(response.Body).Decode(&v)
		return response.StatusCode, v
	}
	code, v := post("/api/login", map[string]string{"user": "admin", "password": "Fixture!Password123"}, "", server.URL)
	if code != 200 {
		t.Fatal("login failed")
	}
	csrf, _ := v["csrf"].(string)
	if get("/api/snapshot") != 200 {
		t.Fatal("authenticated read failed")
	}
	c := defaultConfig()
	if code, _ = post("/api/config", c, "", server.URL); code != 403 {
		t.Fatal("missing CSRF accepted")
	}
	if code, _ = post("/api/config", c, csrf, "http://malicious.example"); code != 403 {
		t.Fatal("cross-origin accepted")
	}
	if code, _ = post("/api/config", c, csrf, server.URL); code != 200 {
		t.Fatal("valid configuration rejected")
	}
	if code, _ = post("/api/config", c, csrf, server.URL); code != 409 {
		t.Fatal("stale revision overwrote settings")
	}
	if code, _ = post("/api/password", map[string]string{"current": "Fixture!Password123", "password": "NewFixture!Password123"}, csrf, server.URL); code != 200 {
		t.Fatal("password update failed")
	}
	if get("/api/snapshot") != 401 {
		t.Fatal("old session not revoked")
	}
}

func TestFixedMinimumTakesPrecedenceWithWarning(t *testing.T) {
	s := testStore(t)
	cl, m := mockClient(t)
	cl.Pool = "pool"
	c := defaultConfig()
	c.Clients = []Client{cl}
	c.Pools = []Pool{{ID: "pool", Name: "Fixture", Budget: 20000, Enabled: true}}
	c.Sites = []Site{{ID: "site", Name: "Fixture", Domains: []string{"a.example"}, Weight: 1, Maximum: 500}}
	c.Overrides["fixture:abc"] = Override{Site: "site", Manage: true, Weight: 1}
	s.saveConfig(c)
	s.db.Exec("INSERT OR REPLACE INTO settings VALUES('guardian',?)", time.Now().Unix())
	e := newEngine(s)
	e.tick(context.Background())
	if m.limit != 50000 || len(e.warnings) == 0 {
		t.Fatal("fixed minimum not preserved or warning missing")
	}
}

func TestRecoveryConnectionRepair(t *testing.T) {
	s := testStore(t)
	cl, m := mockClient(t)
	m.limit = 8000
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "offline", 503) }))
	defer bad.Close()
	old := defaultConfig()
	broken := cl
	broken.URL = bad.URL
	old.Clients = []Client{broken}
	s.saveConfig(old)
	s.db.Exec("INSERT INTO leases VALUES(?,?,?,?,?,?,?,?)", "fixture:abc", cl.ID, "abc", 0, 8000, 8000, "confirmed", time.Now().Unix())
	next := old
	next.Clients = []Client{cl}
	body, _ := json.Marshal(next)
	req := httptest.NewRequest("POST", "/api/config", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	sv := &Server{s: s, e: newEngine(s)}
	sv.save(rec, req)
	if rec.Code != 200 || m.limit != 0 {
		t.Fatalf("connection repair failed: HTTP %d", rec.Code)
	}
	loaded, err := s.config()
	if err != nil || loaded.Clients[0].URL != cl.URL || loaded.Revision != 2 {
		t.Fatal("repaired connection not persisted")
	}
}

func TestUnreachableRecoveryDoesNotStopOtherMonitoring(t *testing.T) {
	s := testStore(t)
	good, _ := mockClient(t)
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "offline", 503) }))
	defer bad.Close()
	c := defaultConfig()
	c.Clients = []Client{good, {ID: "offline", Name: "Offline", Kind: "qb", URL: bad.URL, Enabled: true}}
	s.saveConfig(c)
	s.db.Exec("INSERT INTO leases VALUES(?,?,?,?,?,?,?,?)", "offline:abc", "offline", "abc", 0, 8000, 8000, "confirmed", time.Now().Unix())
	e := newEngine(s)
	e.tick(context.Background())
	if len(e.torrents) != 1 || e.torrents[0].Client != good.ID {
		t.Fatal("unreachable recovery blocked unrelated sampling")
	}
	if len(e.leaseKeys()) != 1 {
		t.Fatal("offline ownership lost")
	}
}
