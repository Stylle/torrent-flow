package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLifetimeSurvivesRetentionRestartAndCounterReset(t *testing.T) {
	dir := t.TempDir()
	s, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2024, 1, 1, 23, 59, 55, 0, time.UTC)
	v := Torrent{Key: "fixture:abc", Site: "site", Uploaded: 100, Downloaded: 200}
	ingest := func(at time.Time) {
		t.Helper()
		if err := s.ingest("fixture", []Torrent{v}, at, 5); err != nil {
			t.Fatal(err)
		}
	}
	ingest(now)
	v.Uploaded += 80
	v.Downloaded += 40
	ingest(now.Add(10 * time.Second))
	// One year later the detailed rows expire, but the aggregate must survive.
	v.Uploaded += 20
	v.Downloaded += 10
	ingest(now.AddDate(2, 0, 0))
	v.Uploaded = 0
	v.Downloaded = 0
	ingest(now.AddDate(2, 0, 0).Add(5 * time.Second))
	s.db.Close()
	s, err = openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	var up, down int64
	if err = s.db.QueryRow("SELECT up,down FROM lifetime WHERE id=1").Scan(&up, &down); err != nil || up != 100 || down != 50 {
		t.Fatalf("aggregate %d/%d: %v", up, down, err)
	}
	sv := &Server{s: s, e: newEngine(s)}
	rec := httptest.NewRecorder()
	sv.snapshot(rec, httptest.NewRequest("GET", "/api/snapshot?view=active&period=day", nil))
	var result struct{ Lifetime struct{ Up, Down int64 } }
	if err = json.Unmarshal(rec.Body.Bytes(), &result); err != nil || rec.Code != 200 || result.Lifetime.Up != 100 || result.Lifetime.Down != 50 {
		t.Fatalf("snapshot aggregate unavailable: HTTP %d", rec.Code)
	}
}

func TestLifetimeLegacyMigrationIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	s, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	s.db.Exec("DROP TABLE lifetime")
	s.db.Exec("INSERT INTO traffic(k,site,at,up,down,uncertain) VALUES('fixture:a','site',1,123,456,0)")
	s.db.Close()
	for i := 0; i < 2; i++ {
		s, err = openStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		var up, down int64
		err = s.db.QueryRow("SELECT up,down FROM lifetime WHERE id=1").Scan(&up, &down)
		s.db.Close()
		if err != nil || up != 123 || down != 456 {
			t.Fatal("legacy aggregate lost or counted twice")
		}
	}
}
