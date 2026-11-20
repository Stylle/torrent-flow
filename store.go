package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Store struct {
	db   *sql.DB
	aead cipher.AEAD
}

func openStore(dir string) (*Store, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	path := filepath.Join(dir, "master.key")
	key, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		if st, e2 := os.Stat(filepath.Join(dir, "flow.sqlite")); e2 == nil && st.Size() > 0 {
			// Another first-start process may have published the key since our first read.
			key, e = os.ReadFile(path)
			if e != nil {
				return nil, fmt.Errorf("master.key missing; restore the complete data volume")
			}
		} else {
			key = make([]byte, 32)
			if _, e = rand.Read(key); e != nil {
				return nil, e
			}
			f, e2 := os.CreateTemp(dir, ".master-*.tmp")
			if e2 != nil {
				return nil, e2
			}
			tmp := f.Name()
			defer os.Remove(tmp)
			if e2 = f.Chmod(0600); e2 != nil {
				f.Close()
				return nil, e2
			}
			if _, e2 = f.Write(key); e2 != nil {
				f.Close()
				return nil, e2
			}
			if e2 = f.Sync(); e2 != nil {
				f.Close()
				return nil, e2
			}
			if e2 = f.Close(); e2 != nil {
				return nil, e2
			}
			// Link publication is atomic and never replaces another process's key.
			if e2 = os.Link(tmp, path); e2 != nil && !os.IsExist(e2) {
				return nil, fmt.Errorf("cannot atomically publish master.key: %w", e2)
			}
			if directory, e2 := os.Open(dir); e2 == nil {
				directory.Sync()
				directory.Close()
			}
			key, e = os.ReadFile(path)
		}
	}
	if e != nil || len(key) != 32 {
		return nil, fmt.Errorf("cannot read valid master.key")
	}
	block, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	a, e := cipher.NewGCM(block)
	if e != nil {
		return nil, e
	}
	db, e := sql.Open("sqlite", filepath.Join(dir, "flow.sqlite"))
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	s := &Store{db, a}
	if _, e = db.Exec("PRAGMA busy_timeout=10000"); e != nil {
		db.Close()
		return nil, e
	}
	for retry := 0; retry < 50; retry++ {
		_, e = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL;
CREATE TABLE IF NOT EXISTS settings(k TEXT PRIMARY KEY,v BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS baseline(k TEXT PRIMARY KEY,up INTEGER,down INTEGER,at INTEGER,added INTEGER,present INTEGER DEFAULT 1);
CREATE TABLE IF NOT EXISTS traffic(id INTEGER PRIMARY KEY,k TEXT,site TEXT,at INTEGER,up INTEGER,down INTEGER,uncertain INTEGER);
CREATE INDEX IF NOT EXISTS traffic_at ON traffic(at);
CREATE INDEX IF NOT EXISTS traffic_rollup ON traffic(k,site,at);
CREATE TABLE IF NOT EXISTS speeds(at INTEGER PRIMARY KEY,up INTEGER,down INTEGER);
CREATE TABLE IF NOT EXISTS site_speeds(at INTEGER,site TEXT,up INTEGER,down INTEGER,PRIMARY KEY(at,site));
CREATE TABLE IF NOT EXISTS leases(k TEXT PRIMARY KEY,client TEXT,hash TEXT,original INTEGER,last INTEGER,planned INTEGER,state TEXT,at INTEGER);
CREATE TABLE IF NOT EXISTS control_blocks(k TEXT PRIMARY KEY,at INTEGER);
CREATE TABLE IF NOT EXISTS events(id INTEGER PRIMARY KEY,at INTEGER,kind TEXT,k TEXT,message TEXT);
CREATE TABLE IF NOT EXISTS sessions(token TEXT PRIMARY KEY,csrf TEXT,expires INTEGER);
CREATE TABLE IF NOT EXISTS notification_jobs(id TEXT PRIMARY KEY,kind TEXT,title TEXT,content TEXT,at INTEGER,next INTEGER,attempts INTEGER,state TEXT,status INTEGER,error TEXT);
`)
		if e == nil || !strings.Contains(e.Error(), "locked") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if e != nil {
		db.Close()
		return nil, e
	}
	var b []byte
	e = db.QueryRow("SELECT v FROM settings WHERE k='config'").Scan(&b)
	if e == sql.ErrNoRows {
		c := defaultConfig()
		initial, _ := json.Marshal(c)
		_, e = db.Exec("INSERT OR IGNORE INTO settings VALUES('config',?)", s.seal(initial))
	} else if e == nil {
		_, e = s.config()
	}
	return s, e
}
func (s *Store) seal(b []byte) []byte {
	nonce := make([]byte, s.aead.NonceSize())
	rand.Read(nonce)
	return append(nonce, s.aead.Seal(nil, nonce, b, nil)...)
}
func (s *Store) unseal(b []byte) ([]byte, error) {
	n := s.aead.NonceSize()
	if len(b) < n {
		return nil, fmt.Errorf("invalid encrypted settings")
	}
	return s.aead.Open(nil, b[:n], b[n:], nil)
}
func (s *Store) config() (Config, error) {
	var b []byte
	c := defaultConfig()
	e := s.db.QueryRow("SELECT v FROM settings WHERE k='config'").Scan(&b)
	if e != nil {
		return c, e
	}
	b, e = s.unseal(b)
	if e == nil {
		e = json.Unmarshal(b, &c)
	}
	if e == nil && c.PolicyVersion < 1 {
		if len(c.Pools) == 0 {
			c.Pools = []Pool{{ID: "default-outlet", Name: "默认出口", Budget: 6250000, Enabled: true}}
		}
		for i := range c.Clients {
			c.Clients[i].Enabled = true
			if c.Clients[i].Pool == "" {
				c.Clients[i].Pool = c.Pools[0].ID
			}
		}
		for i := range c.Sites {
			if c.Sites[i].Weight > 10 {
				c.Sites[i].Weight = 10
			}
			if c.Sites[i].Minimum == 0 {
				c.Sites[i].Minimum = 50000
			}
		}
		for i := range c.Pools {
			c.Pools[i].Enabled = true
		}
		c.PolicyVersion = 1
	}
	for i, p := range c.Pools {
		if p.ID == "default-outlet" && p.Name == "默认出口 · 50 Mbps" {
			c.Pools[i].Name = "默认出口"
		}
	}
	return c, e
}
func (s *Store) saveConfig(c Config) error {
	b, e := json.Marshal(c)
	if e != nil {
		return e
	}
	_, e = s.db.Exec("INSERT OR REPLACE INTO settings VALUES('config',?)", s.seal(b))
	return e
}
func randomID() string {
	b := make([]byte, 24)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
func event(tx *sql.Tx, kind, key, msg string) {
	tx.Exec("INSERT INTO events(at,kind,k,message) VALUES(?,?,?,?)", time.Now().Unix(), kind, key, msg)
}
func (s *Store) ingest(client string, items []Torrent, now time.Time, poll int) error {
	tx, e := s.db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	rows, e := tx.Query("SELECT k,up,down,at,added,present FROM baseline WHERE k>=? AND k<?", client+":", client+";")
	if e != nil {
		return e
	}
	type prev struct {
		up, down, at, added int64
		present             bool
	}
	old := map[string]prev{}
	for rows.Next() {
		var k string
		var v prev
		rows.Scan(&k, &v.up, &v.down, &v.at, &v.added, &v.present)
		old[k] = v
	}
	rows.Close()
	tx.Exec("UPDATE baseline SET present=0 WHERE k>=? AND k<?", client+":", client+";")
	for _, t := range items {
		if p, ok := old[t.Key]; ok && p.present && (p.added == t.Added || p.added == 0 || t.Added == 0) {
			if t.Uploaded >= p.up && t.Downloaded >= p.down {
				u, d := t.Uploaded-p.up, t.Downloaded-p.down
				if u > 0 || d > 0 {
					uncertain := now.Unix()-p.at > int64(poll*3) || time.Unix(p.at, 0).In(now.Location()).Format("2006-01-02") != now.Format("2006-01-02")
					hour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
					result, updateErr := tx.Exec("UPDATE traffic SET up=up+?,down=down+? WHERE id=(SELECT id FROM traffic WHERE k=? AND site=? AND at>=? AND at<? AND uncertain=? ORDER BY id LIMIT 1)", u, d, t.Key, t.Site, hour.Unix(), hour.Add(time.Hour).Unix(), uncertain)
					if updateErr != nil {
						return updateErr
					}
					changed, _ := result.RowsAffected()
					if changed == 0 {
						_, e = tx.Exec("INSERT INTO traffic(k,site,at,up,down,uncertain) VALUES(?,?,?,?,?,?)", t.Key, t.Site, now.Unix(), u, d, uncertain)
					}
					if e != nil {
						return e
					}
				}
			} else {
				event(tx, "counter-reset", t.Key, "Counters reset; new baseline established")
			}
		}
		_, e = tx.Exec("INSERT OR REPLACE INTO baseline(k,up,down,at,added,present) VALUES(?,?,?,?,?,1)", t.Key, t.Uploaded, t.Downloaded, now.Unix(), t.Added)
		if e != nil {
			return e
		}
	}
	var maintenance int64
	tx.QueryRow("SELECT CAST(v AS INTEGER) FROM settings WHERE k='maintenance'").Scan(&maintenance)
	if now.Unix()-maintenance >= 3600 || now.Unix() < maintenance {
		tx.Exec("DELETE FROM traffic WHERE at<?", now.AddDate(-1, 0, 0).Unix())
		tx.Exec("DELETE FROM speeds WHERE at<?", now.Add(-24*time.Hour).Unix())
		tx.Exec("DELETE FROM site_speeds WHERE at<?", now.Add(-24*time.Hour).Unix())
		tx.Exec("DELETE FROM sessions WHERE expires<?", now.Unix())
		tx.Exec("DELETE FROM events WHERE at<?", now.AddDate(-1, 0, 0).Unix())
		tx.Exec("INSERT OR REPLACE INTO settings VALUES('maintenance',?)", now.Unix())
	}
	return tx.Commit()
}
func (s *Store) report(start, end time.Time) ([]Bucket, error) {
	rows, e := s.db.Query("SELECT at,site,up,down,uncertain FROM traffic WHERE at>=? AND at<? ORDER BY at", start.Unix(), end.Unix())
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	loc := start.Location()
	m := map[string]*Bucket{}
	var keys []string
	duration := end.Sub(start)
	for rows.Next() {
		var at, u, d int64
		var site string
		var uncertain bool
		rows.Scan(&at, &site, &u, &d, &uncertain)
		t := time.Unix(at, 0).In(loc)
		var bucket time.Time
		if duration <= 25*time.Hour {
			bucket = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, loc)
		} else if duration > 100*24*time.Hour {
			bucket = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
		} else {
			bucket = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		}
		k := fmt.Sprintf("%d/%s", bucket.Unix(), site)
		if m[k] == nil {
			m[k] = &Bucket{Time: bucket.Unix(), Site: site}
			keys = append(keys, k)
		}
		m[k].Up += u
		m[k].Down += d
		m[k].Uncertain = m[k].Uncertain || uncertain
	}
	out := []Bucket{}
	for _, k := range keys {
		out = append(out, *m[k])
	}
	return out, rows.Err()
}
func period(now time.Time, p string) (time.Time, time.Time) {
	y, m, d := now.Date()
	s := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	switch p {
	case "week":
		s = s.AddDate(0, 0, -(int(s.Weekday())+6)%7)
		return s, s.AddDate(0, 0, 7)
	case "month":
		s = time.Date(y, m, 1, 0, 0, 0, 0, now.Location())
		return s, s.AddDate(0, 1, 0)
	case "year":
		s = time.Date(y, 1, 1, 0, 0, 0, 0, now.Location())
		return s, s.AddDate(1, 0, 0)
	}
	return s, s.AddDate(0, 0, 1)
}
