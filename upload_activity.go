package main

import "time"

// Confirmation uses changes in a downloader's cumulative upload counter.
// A positive delta proves traffic occurred somewhere in the interval; the
// configured duration is measured between consecutive observations of growth.
type uploadActivity struct {
	source      *Adapter
	sampledAt   time.Time
	firstGrowth time.Time
	lastGrowth  time.Time
	uploaded    int64
	added       int64
	qualified   bool
}

func observeUpload(previous uploadActivity, source *Adapter, torrent Torrent, at time.Time, c Config) uploadActivity {
	current := uploadActivity{source: source, sampledAt: at, uploaded: torrent.Uploaded, added: torrent.Added}
	gap := at.Sub(previous.sampledAt)
	if previous.source != source || gap <= 0 || gap > 3*time.Duration(c.PollSeconds)*time.Second ||
		(previous.added != 0 && torrent.Added != 0 && previous.added != torrent.Added) ||
		torrent.Uploaded < previous.uploaded {
		return current
	}
	idle := time.Duration(c.IdleSeconds) * time.Second
	current.lastGrowth = previous.lastGrowth
	if torrent.Uploaded == previous.uploaded {
		current.qualified = previous.qualified && at.Sub(previous.lastGrowth) < idle
		return current
	}
	current.lastGrowth = at
	current.firstGrowth = previous.firstGrowth
	if current.firstGrowth.IsZero() {
		current.firstGrowth = at
	}
	current.qualified = previous.qualified || at.Sub(current.firstGrowth) >= time.Duration(c.UploadConfirmSeconds)*time.Second
	return current
}

func showInAllocation(t Torrent, c Config) bool {
	if c.UploadConfirmSeconds > 0 {
		return t.UploadQualified || t.State == "release pending"
	}
	return t.Connections > 0 || t.Up > 0
}
