package main

import (
	"context"
	"fmt"
	"time"
)

// Explicit strategy stop clears per-torrent limits on all configured downloaders.
// Journal each operation before recovery performs the write; retries survive restart.
func (e *Engine) clearAll(ctx context.Context, c Config) error {
	var failed bool
	for _, cl := range c.Clients {
		a := e.get(cl)
		items, err := a.list(ctx)
		if err != nil {
			failed = true
			continue
		}
		for _, t := range items {
			var owned int
			e.s.db.QueryRow("SELECT COUNT(*) FROM leases WHERE k=?", t.Key).Scan(&owned)
			if t.Limit == 0 && owned == 0 {
				continue
			}
			_, err = e.s.db.Exec("INSERT OR REPLACE INTO leases VALUES(?,?,?,?,?,?,?,?)", t.Key, cl.ID, t.Hash, 0, t.Limit, 0, "prepared", time.Now().Unix())
			if err != nil {
				failed = true
				continue
			}
			if e.releaseKey(ctx, c, t.Key) != nil {
				failed = true
			} else if actual, verifyErr := a.readLimit(ctx, t.Hash); verifyErr != nil || actual != 0 {
				failed = true
			}
		}
	}
	if failed {
		return fmt.Errorf("部分下载器清空限速待重试 / Some downloader limits remain pending")
	}
	return nil
}

// Preserve weighted total shares unless fixed per-torrent floors need redistribution.
func allocateSiteShares(budget int64, items []Allocation, floors map[string]int64) (map[string]int64, bool) {
	out := map[string]int64{}
	var total int64
	for id, v := range floors {
		out[id] = v
		total += v
	}
	if total > budget {
		return out, true
	}
	remaining := budget
	pending := append([]Allocation{}, items...)
	for len(pending) > 0 {
		shares := allocate(remaining, pending)
		next := []Allocation{}
		fixed := false
		for _, item := range pending {
			if shares[item.ID] < floors[item.ID] {
				out[item.ID] = floors[item.ID]
				remaining -= floors[item.ID]
				fixed = true
			} else {
				next = append(next, item)
			}
		}
		if !fixed {
			for id, v := range shares {
				out[id] = v
			}
			break
		}
		pending = next
	}
	return out, false
}
