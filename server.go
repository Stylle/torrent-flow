package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
)

//go:embed frontend/dist/client/*
var assets embed.FS

type Server struct {
	e        *Engine
	s        *Store
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func digestToken(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
func strong(p string) bool {
	var u, l, n, s bool
	for _, r := range p {
		u = u || unicode.IsUpper(r)
		l = l || unicode.IsLower(r)
		n = n || unicode.IsDigit(r)
		s = s || (!unicode.IsLetter(r) && !unicode.IsDigit(r))
	}
	return len(p) >= 12 && len(p) <= 72 && u && l && n && s
}
func initAdmin(s *Store, dir string) error {
	var value []byte
	err := s.db.QueryRow("SELECT v FROM settings WHERE k='password'").Scan(&value)
	if err == nil {
		return nil
	}
	p := os.Getenv("ADMIN_PASSWORD")
	generated := false
	if p == "" {
		p = "Tf!7" + randomID()
		generated = true
	}
	if !strong(p) {
		return fmt.Errorf("ADMIN_PASSWORD must be 12-72 bytes with upper/lower case, number and symbol")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if _, err = s.db.Exec("INSERT INTO settings VALUES('password',?)", hash); err != nil {
		return err
	}
	if generated {
		if err = os.WriteFile(filepath.Join(dir, "initial-password.txt"), []byte(p+"\n"), 0600); err != nil {
			return err
		}
		log.Print("Initial admin password written to /data/initial-password.txt; read locally and remove after changing it")
	}
	return nil
}
func jsonOut(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}
func failHTTP(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return fmt.Errorf("invalid request")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("invalid trailing data")
	}
	return nil
}
func (sv *Server) csrfOrigin(r *http.Request) bool {
	if origin := r.Header.Get("Origin"); origin != "" {
		u, e := url.Parse(origin)
		return e == nil && u.Host == r.Host
	}
	return r.Header.Get("Sec-Fetch-Site") != "cross-site"
}
func (sv *Server) session(r *http.Request) (string, string) {
	c, e := r.Cookie("flow_session")
	if e != nil {
		return "", ""
	}
	var csrf string
	e = sv.s.db.QueryRow("SELECT csrf FROM sessions WHERE token=? AND expires>?", digestToken(c.Value), time.Now().Unix()).Scan(&csrf)
	if e != nil {
		return "", ""
	}
	return c.Value, csrf
}
func (sv *Server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if sv.s.db.Ping() != nil {
			http.Error(w, "database unavailable", 503)
			return
		}
		jsonOut(w, map[string]any{"ok": true, "version": Version})
	})
	mux.HandleFunc("POST /api/login", sv.login)
	mux.HandleFunc("GET /api/session", func(w http.ResponseWriter, r *http.Request) {
		_, csrf := sv.session(r)
		if csrf == "" {
			failHTTP(w, 401, "sign in required")
			return
		}
		jsonOut(w, map[string]string{"csrf": csrf})
	})
	api := http.NewServeMux()
	api.HandleFunc("POST /api/logout", func(w http.ResponseWriter, r *http.Request) {
		token, _ := sv.session(r)
		sv.s.db.Exec("DELETE FROM sessions WHERE token=?", digestToken(token))
		http.SetCookie(w, &http.Cookie{Name: "flow_session", Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
		jsonOut(w, map[string]bool{"ok": true})
	})
	api.HandleFunc("GET /api/snapshot", sv.snapshot)
	api.HandleFunc("POST /api/config", sv.save)
	api.HandleFunc("POST /api/test-client", sv.testClient)
	api.HandleFunc("POST /api/release", sv.release)
	api.HandleFunc("POST /api/password", sv.password)
	api.HandleFunc("POST /api/match", sv.match)
	mux.Handle("/api/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, csrf := sv.session(r)
		if csrf == "" {
			failHTTP(w, 401, "sign in required")
			return
		}
		if r.Method != "GET" && (!sv.csrfOrigin(r) || subtle.ConstantTimeCompare([]byte(csrf), []byte(r.Header.Get("X-CSRF-Token"))) != 1) {
			failHTTP(w, 403, "invalid request origin or CSRF token")
			return
		}
		api.ServeHTTP(w, r)
	}))
	sub, _ := fs.Sub(assets, "frontend/dist/client")
	mux.Handle("/", http.FileServer(http.FS(sub)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; font-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		mux.ServeHTTP(w, r)
	})
}
func (sv *Server) login(w http.ResponseWriter, r *http.Request) {
	if !sv.csrfOrigin(r) {
		failHTTP(w, 403, "invalid origin")
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	sv.mu.Lock()
	now := time.Now()
	valid := []time.Time{}
	for _, t := range sv.attempts[ip] {
		if now.Sub(t) < 10*time.Minute {
			valid = append(valid, t)
		}
	}
	if len(valid) >= 10 {
		sv.mu.Unlock()
		failHTTP(w, 429, "try again later")
		return
	}
	sv.attempts[ip] = append(valid, now)
	if len(sv.attempts) > 10000 {
		for k, v := range sv.attempts {
			if len(v) == 0 || now.Sub(v[len(v)-1]) > 10*time.Minute {
				delete(sv.attempts, k)
			}
		}
	}
	sv.mu.Unlock()
	var v struct {
		User     string `json:"user"`
		Password string `json:"password"`
	}
	if decode(w, r, &v) != nil {
		failHTTP(w, 400, "invalid request")
		return
	}
	var hash []byte
	sv.s.db.QueryRow("SELECT v FROM settings WHERE k='password'").Scan(&hash)
	if bcrypt.CompareHashAndPassword(hash, []byte(v.Password)) != nil || v.User != "admin" {
		failHTTP(w, 401, "invalid credentials")
		return
	}
	token, csrf := randomID(), randomID()
	if _, e := sv.s.db.Exec("INSERT INTO sessions VALUES(?,?,?)", digestToken(token), csrf, time.Now().Add(7*24*time.Hour).Unix()); e != nil {
		failHTTP(w, 500, "cannot create session")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "flow_session", Value: token, Path: "/", HttpOnly: true, Secure: r.TLS != nil || os.Getenv("COOKIE_SECURE") == "true", SameSite: http.SameSiteStrictMode, MaxAge: 7 * 86400})
	jsonOut(w, map[string]string{"csrf": csrf})
}
func redacted(c Config) Config {
	for i := range c.Clients {
		c.Clients[i].Password = ""
	}
	return c
}
func (sv *Server) snapshot(w http.ResponseWriter, r *http.Request) {
	sv.e.stateMu.RLock()
	statuses := append([]ClientStatus{}, sv.e.statuses...)
	torrents := append([]Torrent{}, sv.e.torrents...)
	warnings := append([]string{}, sv.e.warnings...)
	sv.e.stateMu.RUnlock()
	c, err := sv.s.config()
	if err != nil {
		failHTTP(w, 500, "cannot read configuration")
		return
	}
	loc, _ := time.LoadLocation(c.Timezone)
	now := time.Now().In(loc)
	if d := r.URL.Query().Get("date"); d != "" {
		v, e := time.ParseInLocation("2006-01-02", d, loc)
		if e != nil {
			failHTTP(w, 400, "invalid date")
			return
		}
		if v.Before(now.AddDate(-1, 0, -1)) || v.After(now.AddDate(0, 0, 1)) {
			failHTTP(w, 400, "date outside retention window")
			return
		}
		now = v
	}
	start, end := period(now, r.URL.Query().Get("period"))
	buckets, err := sv.s.report(start, end)
	if err != nil {
		failHTTP(w, 500, "statistics unavailable")
		return
	}
	speeds := []map[string]int64{}
	rows, err := sv.s.db.Query("SELECT at,up,down FROM speeds WHERE at>? ORDER BY at", time.Now().Add(-30*time.Minute).Unix())
	if err == nil {
		for rows.Next() {
			var at, u, d int64
			rows.Scan(&at, &u, &d)
			speeds = append(speeds, map[string]int64{"time": at, "up": u, "down": d})
		}
		rows.Close()
	}
	leases := []map[string]any{}
	rows, err = sv.s.db.Query("SELECT k,last,state,at FROM leases")
	if err == nil {
		for rows.Next() {
			var key, state string
			var last, at int64
			rows.Scan(&key, &last, &state, &at)
			leases = append(leases, map[string]any{"key": key, "limit": last, "state": state, "time": at})
		}
		rows.Close()
	}
	events := []Event{}
	rows, err = sv.s.db.Query("SELECT at,kind,k,message FROM events ORDER BY id DESC LIMIT 100")
	if err == nil {
		for rows.Next() {
			var e Event
			rows.Scan(&e.Time, &e.Kind, &e.Key, &e.Message)
			events = append(events, e)
		}
		rows.Close()
	}
	jsonOut(w, map[string]any{"version": Version, "config": redacted(c), "clients": statuses, "torrents": torrents, "warnings": warnings, "buckets": buckets, "speeds": speeds, "leases": leases, "events": events, "guardian": time.Now().Unix()-heartbeat(sv.s, "guardian") < 20, "start": start.Unix(), "end": end.Unix()})
}
func (sv *Server) save(w http.ResponseWriter, r *http.Request) {
	var c Config
	if decode(w, r, &c) != nil {
		failHTTP(w, 400, "invalid configuration")
		return
	}
	if c.Clients == nil {
		c.Clients = []Client{}
	}
	if c.Sites == nil {
		c.Sites = []Site{}
	}
	if c.Pools == nil {
		c.Pools = []Pool{}
	}
	if c.Overrides == nil {
		c.Overrides = map[string]Override{}
	}
	sv.e.mu.Lock()
	defer sv.e.mu.Unlock()
	old, err := sv.s.config()
	if err != nil {
		failHTTP(w, 500, "configuration unavailable")
		return
	}
	if c.Revision != old.Revision {
		failHTTP(w, 409, "settings changed; reload before saving")
		return
	}
	for i := range c.Clients {
		for _, o := range old.Clients {
			if c.Clients[i].ID == o.ID && c.Clients[i].Password == "" {
				c.Clients[i].Password = o.Password
			}
		}
	}
	if err = c.validate(); err != nil {
		failHTTP(w, 400, err.Error())
		return
	}
	var control bool
	for _, p := range c.Pools {
		control = control || p.Enabled
	}
	if control && time.Now().Unix()-heartbeat(sv.s, "guardian") > 20 {
		failHTTP(w, 409, "independent guardian must be online before enabling control")
		return
	} // Release using OLD credentials before changing endpoints, ownership or budgets.
	if err = sv.e.restore(r.Context(), old); err != nil {
		// A changed password/address must be repairable without discarding the ownership journal.
		// Verify the administrator-selected replacement can read every outstanding owned hash first.
		recovery := old
		recovery.Clients = append([]Client{}, old.Clients...)
		repaired := false
		for i, previous := range recovery.Clients {
			for _, replacement := range c.Clients {
				if replacement.ID != previous.ID || replacement == previous || replacement.Kind != previous.Kind {
					continue
				}
				rows, qerr := sv.s.db.Query("SELECT hash,last,planned,original,state FROM leases WHERE client=?", previous.ID)
				if qerr != nil {
					failHTTP(w, 500, "recovery records unavailable")
					return
				}
				type pending struct {
					hash, state             string
					last, planned, original int64
				}
				pendingList := []pending{}
				for rows.Next() {
					var v pending
					rows.Scan(&v.hash, &v.last, &v.planned, &v.original, &v.state)
					pendingList = append(pendingList, v)
				}
				rows.Close()
				if len(pendingList) == 0 {
					continue
				}
				a := adapter(replacement)
				for _, v := range pendingList {
					actual, readErr := a.readLimit(r.Context(), v.hash)
					if readErr != nil || (actual != v.last && actual != v.planned && actual != 0 && !(v.state == "prepared-new" && actual == v.original)) {
						failHTTP(w, 409, "replacement downloader cannot confirm pending owned torrents; recovery records retained")
						return
					}
				}
				recovery.Clients[i] = replacement
				repaired = true
			}
		}
		if !repaired {
			failHTTP(w, 409, "restore managed limits before changing settings; downloader may be offline")
			return
		}
		for i := range recovery.Pools {
			recovery.Pools[i].Enabled = false
		}
		recovery.Revision = old.Revision + 1
		if sv.s.saveConfig(recovery) != nil {
			failHTTP(w, 500, "cannot persist recovery connection")
			return
		}
		if sv.e.restore(r.Context(), recovery) != nil {
			failHTTP(w, 502, "recovery connection saved and control disabled; restoration remains pending, refresh settings")
			return
		}
		c.Revision = recovery.Revision
	}
	c.Revision++
	if err = sv.s.saveConfig(c); err != nil {
		failHTTP(w, 500, "cannot save settings")
		return
	}
	for key, newValue := range c.Overrides {
		oldValue, exists := old.Overrides[key]
		if !exists || oldValue != newValue {
			sv.s.db.Exec("DELETE FROM control_blocks WHERE k=?", key)
			delete(sv.e.suspended, key)
		}
	}
	jsonOut(w, redacted(c))
}
func (sv *Server) testClient(w http.ResponseWriter, r *http.Request) {
	var cl Client
	if decode(w, r, &cl) != nil {
		failHTTP(w, 400, "invalid downloader")
		return
	}
	c, _ := sv.s.config()
	for _, old := range c.Clients {
		if cl.ID == old.ID && cl.Password == "" {
			cl.Password = old.Password
		}
	}
	test := defaultConfig()
	cl.Pool = ""
	test.Clients = []Client{cl}
	if err := test.validate(); err != nil {
		failHTTP(w, 400, err.Error())
		return
	}
	items, err := adapter(cl).list(r.Context())
	if err != nil {
		failHTTP(w, 502, err.Error())
		return
	}
	jsonOut(w, map[string]any{"ok": true, "torrents": len(items)})
}
func (sv *Server) release(w http.ResponseWriter, r *http.Request) {
	sv.e.mu.Lock()
	defer sv.e.mu.Unlock()
	c, err := sv.s.config()
	if err != nil {
		failHTTP(w, 500, "configuration unavailable")
		return
	}
	for i := range c.Pools {
		c.Pools[i].Enabled = false
	}
	c.Revision++
	if err = sv.s.saveConfig(c); err != nil {
		failHTTP(w, 500, "cannot disable controller")
		return
	}
	if err = sv.e.restore(r.Context(), c); err != nil {
		failHTTP(w, 502, "some limits could not be restored; recovery list retained")
		return
	}
	jsonOut(w, map[string]bool{"ok": true})
}
func (sv *Server) password(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Current  string `json:"current"`
		Password string `json:"password"`
	}
	if decode(w, r, &v) != nil || !strong(v.Password) {
		failHTTP(w, 400, "use 12-72 bytes with upper/lowercase, number and symbol")
		return
	}
	var hash []byte
	sv.s.db.QueryRow("SELECT v FROM settings WHERE k='password'").Scan(&hash)
	if bcrypt.CompareHashAndPassword(hash, []byte(v.Current)) != nil {
		failHTTP(w, 403, "current password incorrect")
		return
	}
	hash, _ = bcrypt.GenerateFromPassword([]byte(v.Password), bcrypt.DefaultCost)
	tx, err := sv.s.db.Begin()
	if err != nil {
		failHTTP(w, 500, "cannot update password")
		return
	}
	defer tx.Rollback()
	tx.Exec("UPDATE settings SET v=? WHERE k='password'", hash)
	tx.Exec("DELETE FROM sessions")
	if tx.Commit() != nil {
		failHTTP(w, 500, "cannot update password")
		return
	}
	jsonOut(w, map[string]bool{"ok": true})
}
func (sv *Server) match(w http.ResponseWriter, r *http.Request) {
	var v struct {
		URL string `json:"url"`
	}
	if decode(w, r, &v) != nil {
		failHTTP(w, 400, "invalid URL")
		return
	}
	c, _ := sv.s.config()
	id, ambiguous := siteFor(Torrent{Trackers: []string{v.URL}}, c)
	jsonOut(w, map[string]any{"host": host(v.URL), "site": id, "ambiguous": ambiguous})
}
