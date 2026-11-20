package main

import (
	"bytes"
	"github.com/Stylle/torrent-flow/internal/portablebackup"
	"golang.org/x/crypto/bcrypt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPortableBackupRestoreAcrossKeys(t *testing.T) {
	source := testStore(t)
	dest := testStore(t)
	t.Setenv("ADMIN_PASSWORD", "SourceFixture!Password123")
	sourceDir, _ := source.directory()
	if err := initAdmin(source, sourceDir); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ADMIN_PASSWORD", "TargetFixture!Password123")
	destDir, _ := dest.directory()
	if err := initAdmin(dest, destDir); err != nil {
		t.Fatal(err)
	}
	c := defaultConfig()
	c.Clients = []Client{{ID: "fixture", Name: "Fixture", Kind: "qb", URL: "http://downloader.example:8080", Password: "fixture-only", Enabled: true}}
	c.Sites = []Site{{ID: "site", Name: "Retained", Domains: []string{"a.example"}, Weight: 10, Minimum: 50000}}
	c.Notify.Headers = `{"X-Fixture":"fixture-only"}`
	source.saveConfig(c)
	if err := newSiteCatalog(source).setAuto(false); err != nil {
		t.Fatal(err)
	}
	source.db.Exec("INSERT INTO site_speeds VALUES(1,'site',123,4)")
	source.db.Exec("INSERT INTO traffic(k,site,at,up,down,uncertain)VALUES('fixture:abc','site',1,12345,99,0)")
	archive := filepath.Join(t.TempDir(), "backup.zip")
	if err := portablebackup.Export(sourceDir, source.db, flowBackupSpec, archive); err != nil {
		t.Fatal(err)
	}
	originalKey, _ := os.ReadFile(filepath.Join(destDir, "master.key"))
	content, _ := os.ReadFile(archive)
	sv := &Server{s: dest, e: newEngine(dest)}
	r := httptest.NewRequest("POST", "/api/backup", bytes.NewReader(content))
	r.Header.Set("X-Confirm-Restore", "replace")
	rec := httptest.NewRecorder()
	sv.backup(rec, r)
	if rec.Code != 200 {
		t.Fatalf("restore HTTP %d: %s", rec.Code, rec.Body.String())
	}
	if state := sv.e.catalog.snapshot(); state.Auto == nil || *state.Auto {
		t.Fatal("catalog preferences not restored into live cache")
	}
	loaded, err := dest.config()
	if err != nil || len(loaded.Clients) != 1 || loaded.Clients[0].Password != "fixture-only" || loaded.Clients[0].StrategyEnabled == nil || *loaded.Clients[0].StrategyEnabled {
		t.Fatal("credentials or paused state not restored", err)
	}
	currentKey, _ := os.ReadFile(filepath.Join(destDir, "master.key"))
	if !bytes.Equal(originalKey, currentKey) {
		t.Fatal("restore replaced live guardian key")
	}
	var up int64
	dest.db.QueryRow("SELECT SUM(up) FROM traffic").Scan(&up)
	if up != 12345 {
		t.Fatal("statistics lost")
	}
	var hash []byte
	dest.db.QueryRow("SELECT v FROM settings WHERE k='password'").Scan(&hash)
	if bcrypt.CompareHashAndPassword(hash, []byte("SourceFixture!Password123")) != nil {
		t.Fatal("backup login not restored")
	}
	var samples int
	dest.db.QueryRow("SELECT COUNT(*) FROM site_speeds").Scan(&samples)
	if samples != 1 {
		t.Fatal("per-site history lost in backup")
	}
	previous, _ := filepath.Glob(filepath.Join(destDir, "previous-*.zip"))
	if len(previous) != 1 {
		t.Fatal("pre-restore backup not retained")
	}
	source.db.Exec("DROP TABLE site_speeds")
	legacy := filepath.Join(t.TempDir(), "legacy.zip")
	if err := portablebackup.Export(sourceDir, source.db, flowBackupSpec, legacy); err != nil {
		t.Fatal(err)
	}
	legacyBytes, _ := os.ReadFile(legacy)
	legacyRequest := httptest.NewRequest("POST", "/api/backup", bytes.NewReader(legacyBytes))
	legacyRequest.Header.Set("X-Confirm-Restore", "replace")
	legacyResponse := httptest.NewRecorder()
	sv.backup(legacyResponse, legacyRequest)
	if legacyResponse.Code != 200 {
		t.Fatal("pre-site-chart backup was rejected")
	}
	broken := httptest.NewRequest("POST", "/api/backup", bytes.NewReader([]byte("not a zip")))
	broken.Header.Set("X-Confirm-Restore", "replace")
	bad := httptest.NewRecorder()
	sv.backup(bad, broken)
	if bad.Code != 400 {
		t.Fatal("corrupt archive accepted")
	}
	dest.db.QueryRow("SELECT SUM(up) FROM traffic").Scan(&up)
	if up != 12345 {
		t.Fatal("corrupt archive changed data")
	}
}

func TestBackupRejectsPendingDownloaderClearance(t *testing.T) {
	s := testStore(t)
	off := false
	c := defaultConfig()
	c.Clients = []Client{{ID: "fixture", Name: "Offline", Kind: "qb", URL: "http://offline.example", StrategyEnabled: &off, ClearPending: true}}
	s.saveConfig(c)
	sv := &Server{s: s, e: newEngine(s)}
	r := httptest.NewRequest("POST", "/api/backup", bytes.NewReader([]byte("ignored")))
	r.Header.Set("X-Confirm-Restore", "replace")
	rec := httptest.NewRecorder()
	sv.backup(rec, r)
	if rec.Code != 409 {
		t.Fatal("pending clearance was allowed to be discarded")
	}
}
