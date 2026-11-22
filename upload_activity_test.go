package main

import (
	"context"
	"testing"
	"time"
)

func TestSustainedUploadControlsAllocation(t *testing.T) {
	store := testStore(t)
	client, mock := mockClient(t)
	client.Pool = "pool"
	config := defaultConfig()
	config.Clients = []Client{client}
	config.Pools = []Pool{{ID: "pool", Name: "Fixture", Budget: 100000, Enabled: true}}
	config.PollSeconds = 15
	config.UploadConfirmSeconds = 15
	if err := store.saveConfig(config); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("INSERT OR REPLACE INTO settings VALUES('guardian',?)", time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	engine := newEngine(store)
	key := client.ID + ":abc"
	check := func(uploaded int64, connections int, want bool) {
		t.Helper()
		mock.mu.Lock()
		mock.uploaded, mock.connections = uploaded, connections
		mock.mu.Unlock()
		// Advance the previous observation's clock without waiting 15 seconds.
		if prior, ok := engine.uploadActivity[key]; ok {
			prior.sampledAt = prior.sampledAt.Add(-15 * time.Second)
			if !prior.firstGrowth.IsZero() {
				prior.firstGrowth = prior.firstGrowth.Add(-15 * time.Second)
			}
			if !prior.lastGrowth.IsZero() {
				prior.lastGrowth = prior.lastGrowth.Add(-15 * time.Second)
			}
			engine.uploadActivity[key] = prior
		}
		engine.tick(context.Background())
		if len(engine.torrents) != 1 {
			t.Fatalf("expected one sampled torrent, got %d", len(engine.torrents))
		}
		torrent := engine.torrents[0]
		mock.mu.Lock()
		limited := mock.limit > 0
		mock.mu.Unlock()
		if limited != want || torrent.Managed != want || showInAllocation(torrent, config) != want {
			t.Fatalf("upload %d, connections %d: limit applied=%t, torrent=%+v, want active=%t", uploaded, connections, limited, torrent, want)
		}
	}

	check(100, 2, false) // Connected or reporting speed with no measured progress.
	check(100, 2, false)
	check(200, 0, false) // One isolated burst does not qualify.
	check(200, 0, false)
	check(250, 0, false)
	check(300, 0, true)  // Consecutive byte growth despite every displayed speed being zero.
	check(300, 2, false) // Stalled beyond idle delay with a peer still attached.
	check(310, 0, false)
	check(20, 0, false) // Counter reset discards the preceding observation.
	check(25, 0, false)
	check(30, 0, true)
}
