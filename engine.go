package main

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

type Engine struct {
	catalog   *SiteCatalog
	s         *Store
	mu        sync.Mutex
	stateMu   sync.RWMutex
	torrents  []Torrent
	statuses  []ClientStatus
	idle      map[string]time.Time
	suspended map[string]bool
	adapters  map[string]*Adapter
	started   bool
	lastSpeed int64
	warnings  []string
}

func newEngine(s *Store) *Engine {
	return &Engine{s: s, catalog: newSiteCatalog(s), idle: map[string]time.Time{}, suspended: map[string]bool{}, adapters: map[string]*Adapter{}, torrents: []Torrent{}, statuses: []ClientStatus{}}
}
func (e *Engine) get(c Client) *Adapter {
	c = normalizeClient(c)
	a := e.adapters[c.ID]
	if a == nil || a.c.ID != c.ID || a.c.Kind != c.Kind || a.c.URL != c.URL || a.c.User != c.User || a.c.Password != c.Password {
		a = adapter(c)
		e.adapters[c.ID] = a
	}
	return a
}
func heartbeat(s *Store, key string) int64 {
	var v int64
	s.db.QueryRow("SELECT CAST(v AS INTEGER) FROM settings WHERE k=?", key).Scan(&v)
	return v
}
func (e *Engine) tick(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()
	c, err := e.s.config()
	if err != nil {
		return
	}
	changed := false
	for i, cl := range c.Clients {
		if cl.RestorePending && e.restoreClient(ctx, c, cl.ID) == nil {
			c.Clients[i].RestorePending = false
			changed = true
		}
		if cl.ClearPending {
			one := c
			one.Clients = []Client{cl}
			if e.clearAll(ctx, one) == nil {
				c.Clients[i].ClearPending = false
				changed = true
			}
		}
	}
	if changed {
		c.Revision++
		if e.s.saveConfig(c) != nil {
			return
		}
	}
	if c.ClearPending {
		if e.clearAll(ctx, c) == nil {
			c.ClearPending = false
			c.Revision++
			if e.s.saveConfig(c) != nil {
				return
			}
		}
	}
	locNow, _ := time.LoadLocation(c.Timezone)
	now := time.Now().In(locNow)
	if !e.started {
		// Unreachable recovery entries must not prevent monitoring other downloaders.
		e.restore(ctx, c)
		e.started = true
	}
	blocked, _ := e.s.db.Query("SELECT k FROM control_blocks")
	if blocked != nil {
		for blocked.Next() {
			var k string
			blocked.Scan(&k)
			e.suspended[k] = true
		}
		blocked.Close()
	}
	e.s.db.Exec("INSERT OR REPLACE INTO settings VALUES('controller',?)", now.Unix())
	all := []Torrent{}
	statuses := []ClientStatus{}
	warnings := []string{}
	defer func() {
		e.stateMu.Lock()
		e.torrents = all
		e.statuses = statuses
		e.warnings = warnings
		e.stateMu.Unlock()
	}()
	var up, down int64
	for _, cl := range c.Clients {
		if !cl.Enabled {
			continue
		}
		status := ClientStatus{ID: cl.ID, Name: cl.Name, Kind: cl.Kind, AsOf: now.Unix()}
		a := e.get(cl)
		a.progress = func() { e.s.db.Exec("INSERT OR REPLACE INTO settings VALUES('controller',?)", time.Now().Unix()) }
		items, err := a.list(ctx)
		if err != nil {
			status.Error = err.Error()
			statuses = append(statuses, status)
			continue
		}
		status.Online = true
		if next, added := discoverSitesWithCatalog(c, items, e.catalog.index()); added > 0 {
			next.Revision++
			if next.validate() == nil && e.s.saveConfig(next) == nil {
				c = next
			} else {
				warnings = append(warnings, "Tracker discovery could not be saved; existing rules retained")
			}
		}
		for i := range items {
			items[i].Site, items[i].Ambiguous = siteFor(items[i], c)
			if items[i].Site == "" {
				for _, raw := range items[i].Trackers {
					if items[i].Site != "" {
						break
					}
					for _, site := range c.Sites {
						one := c
						one.Sites = []Site{site}
						one.Overrides = map[string]Override{}
						if id, _ := siteFor(Torrent{Trackers: []string{raw}}, one); id != "" {
							items[i].Site = id
							break
						}
					}
				}
				if items[i].Site == "" {
					items[i].Site = "tracker-unavailable"
				}
			}
			items[i].State = "monitoring"
			if e.suspended[items[i].Key] {
				items[i].State = "external limit change"
			}
			up += items[i].Up
			down += items[i].Down
			status.Up += items[i].Up
			status.Down += items[i].Down
		}
		if err = e.s.ingest(cl.ID, items, now, c.PollSeconds); err != nil {
			status.Error = "Statistics could not be committed"
			status.Online = false
		}
		all = append(all, items...)
		statuses = append(statuses, status)
	}
	online := false
	for _, status := range statuses {
		online = online || status.Online
	}
	if online && now.Unix()-e.lastSpeed >= 15 {
		tx, err := e.s.db.Begin()
		if err == nil {
			_, err = tx.Exec("INSERT OR REPLACE INTO speeds VALUES(?,?,?)", now.Unix(), up, down)
			totals := map[string][2]int64{}
			for _, torrent := range all {
				v := totals[torrent.Site]
				v[0] += torrent.Up
				v[1] += torrent.Down
				totals[torrent.Site] = v
			}
			for site, v := range totals {
				if err != nil {
					break
				}
				_, err = tx.Exec("INSERT OR REPLACE INTO site_speeds VALUES(?,?,?,?)", now.Unix(), site, v[0], v[1])
			}
			if err == nil {
				err = tx.Commit()
			} else {
				tx.Rollback()
			}
			if err == nil {
				e.lastSpeed = now.Unix()
			}
		}
	}
	guard := heartbeat(e.s, "guardian")
	if now.Unix()-guard > 20 {
		e.restore(ctx, c)
		return
	}
	targets := map[string]int64{}
	sites := map[string]Site{}
	for _, s := range c.Sites {
		sites[s.ID] = s
	}
	loc, _ := time.LoadLocation(c.Timezone)
	dayStart, dayEnd := period(now.In(loc), "day")
	monthStart, monthEnd := period(now.In(loc), "month")
	day, _ := e.s.report(dayStart, dayEnd)
	month, _ := e.s.report(monthStart, monthEnd)
	du, mu := map[string]int64{}, map[string]int64{}
	for _, b := range day {
		du[b.Site] += b.Up
	}
	for _, b := range month {
		mu[b.Site] += b.Up
	}
	for _, pool := range c.Pools {
		if !pool.Enabled || !policyOn(c) {
			continue
		}
		groups := map[string][]Torrent{}
		for _, t := range all {
			cl := Client{}
			for _, v := range c.Clients {
				if v.ID == t.Client {
					cl = v
					break
				}
			}
			if cl.Pool != pool.ID || !clientPolicyOn(cl) || t.Site == "" || e.suspended[t.Key] {
				continue
			}
			if t.Connections > 0 || t.Up > 0 {
				e.idle[t.Key] = now
			} else {
				since, ok := e.idle[t.Key]
				if !ok {
					since = now.Add(-time.Duration(c.IdleSeconds) * time.Second)
					e.idle[t.Key] = since
				}
				if now.Sub(since) >= time.Duration(c.IdleSeconds)*time.Second {
					continue
				}
			}
			groups[t.Site] = append(groups[t.Site], t)
		}
		var siteItems []Allocation
		floors := map[string]int64{}
		var totalFloor int64
		for id, ts := range groups {
			s := sites[id]
			perSeed := s.Minimum
			if perSeed < 1000 {
				perSeed = 1000
			}
			floor := int64(len(ts)) * ((perSeed + 999) / 1000 * 1000)
			// Maximum is a per-torrent ceiling, unlike the daily/monthly site's
			// post-target aggregate allowance. Share only the combined capacity.
			maximum := int64(len(ts)) * s.Maximum
			if s.DayTarget > 0 && du[id] >= s.DayTarget || s.MonthTarget > 0 && mu[id] >= s.MonthTarget {
				if maximum == 0 || s.AfterTarget < maximum {
					maximum = s.AfterTarget
				}
			}
			if maximum > 0 && maximum < floor {
				warnings = append(warnings, fmt.Sprintf("站点 %s：活跃 %d 个种子，最低额度合计 %s 高于有效上限 %s；保留最低额度（非出口超额） / Site minimum conflicts with cap, not outlet budget", s.Name, len(ts), formatBytes(floor, true), formatBytes(maximum, true)))
				maximum = floor
			}
			floors[id] = floor
			totalFloor += floor
			siteItems = append(siteItems, Allocation{id, s.Weight, 0, maximum})
		}
		effectiveBudget, _ := pool.effectiveBudget(now)
		siteBudgets, exceeded := allocateSiteShares(effectiveBudget, siteItems, floors)
		if exceeded {
			warnings = append(warnings, fmt.Sprintf("出口 %s：最低额度合计 %s，超过预算 %s；最低额度保持不变 / Minimum allowances exceed outlet budget", pool.Name, formatBytes(totalFloor, true), formatBytes(effectiveBudget, true)))
		}
		for site, ts := range groups {
			var items []Allocation
			for _, t := range ts {
				w := int64(1)
				if w < 1 {
					w = 1
				}
				items = append(items, Allocation{t.Key, w, 0, 0})
			}
			for k, v := range allocate(siteBudgets[site], items) {
				targets[k] = v / 1000 * 1000
			}
		}
	}
	owned := map[string]bool{}
	confirmed := map[string]int64{}
	leaseRows, leaseErr := e.s.db.Query("SELECT k,last,state FROM leases")
	if leaseErr != nil {
		warnings = append(warnings, "限速归属不可读取，本轮不写入 / Ownership journal unavailable")
		return
	}
	for leaseRows.Next() {
		var key, state string
		var last int64
		if err := leaseRows.Scan(&key, &last, &state); err != nil {
			leaseRows.Close()
			warnings = append(warnings, "限速归属记录损坏 / Invalid ownership journal")
			return
		}
		owned[key] = true
		if state == "confirmed" {
			confirmed[key] = last
		}
	}
	leaseReadErr := leaseRows.Err()
	leaseRows.Close()
	if leaseReadErr != nil {
		warnings = append(warnings, "限速归属读取失败 / Ownership journal read failed")
		return
	}
	for i := range all {
		t := &all[i]
		if v, ok := targets[t.Key]; ok && v >= 1000 {
			var cl Client
			for _, v := range c.Clients {
				if v.ID == t.Client {
					cl = v
				}
			}
			if last, ok := confirmed[t.Key]; ok && last == normalizedLimit(cl, v) && t.Limit == last {
				t.Managed = true
				t.State = "allocated"
				continue
			}
			err := e.apply(ctx, cl, *t, v)
			if err != nil {
				t.State = err.Error()
			} else {
				t.Managed = true
				t.State = "allocated"
				t.Limit = normalizedLimit(cl, v)
			}
		} else {
			if owned[t.Key] {
				if err := e.releaseKey(ctx, c, t.Key); err != nil {
					t.State = "release pending"
				} else if t.Connections == 0 && t.Up == 0 {
					t.State = "released"
				}
			} else if t.Connections == 0 && t.Up == 0 {
				t.State = "released"
			}
		}
	} // Also restore missing/deleted/disabled torrents from the persistent ownership list.
	keys := e.leaseKeys()
	for _, k := range keys {
		if _, ok := targets[k]; !ok {
			e.releaseKey(ctx, c, k)
		}
	}
	e.s.db.Exec("INSERT OR REPLACE INTO settings VALUES('controller',?)", time.Now().Unix())
}
func (e *Engine) leaseKeys() []string {
	rows, err := e.s.db.Query("SELECT k FROM leases")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		rows.Scan(&k)
		keys = append(keys, k)
	}
	return keys
}
func (e *Engine) apply(ctx context.Context, cl Client, t Torrent, target int64) error {
	target = normalizedLimit(cl, target)
	tx, err := e.s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT OR REPLACE INTO settings VALUES('controller',?)", time.Now().Unix()); err != nil {
		return err
	}
	var guard int64
	tx.QueryRow("SELECT CAST(v AS INTEGER) FROM settings WHERE k='guardian'").Scan(&guard)
	if time.Now().Unix()-guard > 20 {
		return fmt.Errorf("guardian offline")
	}
	var last, original, planned int64
	var state string
	err = tx.QueryRow("SELECT last,original,planned,state FROM leases WHERE k=?", t.Key).Scan(&last, &original, &planned, &state)
	exists := err == nil
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if exists && state == "confirmed" && last == target && t.Limit == target {
		return tx.Commit()
	}
	a := e.get(cl)
	actual, err := a.readLimit(ctx, t.Hash)
	if err != nil {
		return err
	}
	if exists && actual != last && actual != planned {
		e.suspended[t.Key] = true
		tx.Exec("INSERT OR REPLACE INTO control_blocks VALUES(?,?)", t.Key, time.Now().Unix())
		tx.Exec("DELETE FROM leases WHERE k=?", t.Key)
		event(tx, "external-change", t.Key, "User changed the limit; ownership released")
		tx.Commit()
		return fmt.Errorf("external limit change")
	}
	if exists && actual == target && state == "confirmed" {
		return tx.Commit()
	}
	nextState := "prepared"
	if !exists {
		original = actual
		nextState = "prepared-new"
	}
	_, err = tx.Exec("INSERT OR REPLACE INTO leases VALUES(?,?,?,?,?,?,?,?)", t.Key, cl.ID, t.Hash, original, actual, target, nextState, time.Now().Unix())
	if err != nil {
		return err
	}
	// Intent is durable before sending any command. Guardian can recover either old or planned limit.
	if err = tx.Commit(); err != nil {
		return err
	}
	tx, err = e.s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec("UPDATE leases SET at=? WHERE k=?", time.Now().Unix(), t.Key)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return fmt.Errorf("recovery superseded this command")
	}
	var stamp int64
	tx.QueryRow("SELECT CAST(v AS INTEGER) FROM settings WHERE k='controller'").Scan(&stamp)
	if time.Now().Unix()-stamp > 20 {
		return fmt.Errorf("expired control lease")
	}
	if err = a.setLimit(ctx, t.Hash, target); err != nil {
		event(tx, "write-pending", t.Key, "Limit command pending reconciliation")
		tx.Commit()
		return err
	}
	actual, err = a.readLimit(ctx, t.Hash)
	if err != nil || actual != target {
		event(tx, "readback-failed", t.Key, "Limit command could not be confirmed")
		tx.Commit()
		return fmt.Errorf("limit not confirmed")
	}
	tx.Exec("UPDATE leases SET last=?,state='confirmed',at=? WHERE k=?", target, time.Now().Unix(), t.Key)
	event(tx, "allocated", t.Key, "上传限速已确认 / Upload limit confirmed: "+formatBytes(target, true))
	return tx.Commit()
}
func (e *Engine) releaseKey(ctx context.Context, c Config, key string) error {
	tx, err := e.s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("UPDATE leases SET state=state WHERE k=?", key); err != nil {
		return err
	}
	var clID, hash, state string
	var last, original, planned int64
	err = tx.QueryRow("SELECT client,hash,last,original,planned,state FROM leases WHERE k=?", key).Scan(&clID, &hash, &last, &original, &planned, &state)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	var cl Client
	for _, v := range c.Clients {
		if v.ID == clID {
			cl = v
			break
		}
	}
	if cl.ID == "" {
		return fmt.Errorf("downloader configuration required for recovery")
	}
	a := e.get(cl)
	actual, err := a.readLimit(ctx, hash)
	if err == errTorrentGone {
		tx.Exec("DELETE FROM leases WHERE k=?", key)
		event(tx, "removed", key, "Torrent removed from downloader; recovery entry retired")
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	if state == "prepared-new" && actual == original && actual != planned {
		tx.Exec("DELETE FROM leases WHERE k=?", key)
		event(tx, "not-applied", key, "Prepared limit was not applied; original setting retained")
		return tx.Commit()
	}
	if actual != last && actual != planned && actual != 0 {
		e.suspended[key] = true
		tx.Exec("INSERT OR REPLACE INTO control_blocks VALUES(?,?)", key, time.Now().Unix())
		tx.Exec("DELETE FROM leases WHERE k=?", key)
		event(tx, "external-change", key, "External limit retained; control suspended")
		return tx.Commit()
	}
	if actual != 0 {
		if err = a.setLimit(ctx, hash, 0); err != nil {
			return err
		}
		actual, err = a.readLimit(ctx, hash)
		if err != nil || actual != 0 {
			return fmt.Errorf("unlimited restore not confirmed")
		}
	}
	tx.Exec("DELETE FROM leases WHERE k=?", key)
	event(tx, "released", key, "System-managed torrent limit restored to unlimited")
	return tx.Commit()
}
func (e *Engine) restore(ctx context.Context, c Config) error {
	var first error
	for _, k := range e.leaseKeys() {
		if err := e.releaseKey(ctx, c, k); err != nil && first == nil {
			first = err
		}
	}
	return first
}
func (e *Engine) run(ctx context.Context) {
	for {
		e.tick(ctx)
		c, err := e.s.config()
		wait := 5
		if err == nil {
			wait = c.PollSeconds
		}
		select {
		case <-ctx.Done():
			e.mu.Lock()
			c, _ := e.s.config()
			restoreCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			e.restore(restoreCtx, c)
			cancel()
			e.mu.Unlock()
			return
		case <-time.After(time.Duration(wait) * time.Second):
		}
	}
}
func (e *Engine) guardian(ctx context.Context) {
	for {
		e.mu.Lock()
		e.s.db.Exec("INSERT OR REPLACE INTO settings VALUES('guardian',?)", time.Now().Unix())
		last := heartbeat(e.s, "controller")
		if time.Now().Unix()-last > 30 {
			c, err := e.s.config()
			if err == nil {
				e.restore(ctx, c)
			}
		}
		e.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}
