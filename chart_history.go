package main

import (
	"time"
)

type speedHistoryEntry struct {
	epoch uint64
	at    time.Time
	rows  []map[string]any
}

// Bound expensive long-range aggregation and reuse immutable samples across dashboard polls.
func (sv *Server) siteSpeedHistory(window int) []map[string]any {
	sv.speedMu.Lock()
	defer sv.speedMu.Unlock()
	epoch := sv.speedEpoch.Load()
	ttl := 15 * time.Second
	if window > 3600 {
		ttl = time.Minute
	}
	if cached, ok := sv.speedCache[window]; ok && cached.epoch == epoch && time.Since(cached.at) < ttl {
		return cached.rows
	}
	values := []map[string]any{}
	step := max(15, window/480)
	rows, err := sv.s.db.Query("SELECT (at / ?) * ?,site,AVG(up),AVG(down) FROM site_speeds WHERE at>? GROUP BY at / ?,site ORDER BY at / ?", step, step, time.Now().Add(-time.Duration(window)*time.Second).Unix(), step, step)
	if err != nil {
		return values
	}
	defer rows.Close()
	for rows.Next() {
		var at int64
		var site string
		var up, down float64
		if rows.Scan(&at, &site, &up, &down) == nil {
			values = append(values, map[string]any{"time": at, "site": site, "up": int64(up), "down": int64(down)})
		}
	}
	if len(values) > 0 && rows.Err() == nil {
		if sv.speedCache == nil {
			sv.speedCache = map[int]speedHistoryEntry{}
		}
		sv.speedCache[window] = speedHistoryEntry{epoch: epoch, at: time.Now(), rows: values}
	}
	return values
}
