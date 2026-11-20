package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (e *Engine) restoreClient(ctx context.Context, c Config, id string) error {
	var first error
	for _, key := range e.leaseKeys() {
		if strings.HasPrefix(key, id+":") {
			if err := e.releaseKey(ctx, c, key); err != nil && first == nil {
				first = err
			}
		}
	}
	return first
}
func (sv *Server) clientEnabled(w http.ResponseWriter, r *http.Request) {
	var v struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if decode(w, r, &v) != nil {
		failHTTP(w, 400, "invalid downloader switch")
		return
	}
	sv.e.mu.Lock()
	defer sv.e.mu.Unlock()
	if !sv.stillAuthorized(r) {
		failHTTP(w, 401, "sign in required")
		return
	}
	c, err := sv.s.config()
	if err != nil {
		failHTTP(w, 500, "configuration unavailable")
		return
	}
	i := -1
	for j, cl := range c.Clients {
		if cl.ID == v.ID {
			i = j
		}
	}
	if i < 0 {
		failHTTP(w, 404, "downloader not found")
		return
	}
	if v.Enabled && (c.Clients[i].RestorePending || c.Clients[i].ClearPending || c.ClearPending) {
		failHTTP(w, 409, "请等待限速恢复完成 / Recovery pending")
		return
	}
	if v.Enabled && clientPolicyOn(c.Clients[i]) && time.Now().Unix()-heartbeat(sv.s, "guardian") > 20 {
		failHTTP(w, 409, "恢复守护未连接 / Guardian offline")
		return
	}
	c.Clients[i].Enabled = v.Enabled
	c.Clients[i].RestorePending = !v.Enabled
	c.Revision++
	if sv.s.saveConfig(c) != nil {
		failHTTP(w, 500, "cannot save downloader switch")
		return
	}
	if !v.Enabled {
		if sv.e.restoreClient(r.Context(), c, v.ID) == nil {
			c.Clients[i].RestorePending = false
			c.Revision++
			if sv.s.saveConfig(c) != nil {
				failHTTP(w, 500, "cannot finish recovery")
				return
			}
		}
	}
	sv.s.db.Exec("INSERT INTO events(at,kind,k,message) VALUES(?,?,?,?)", time.Now().Unix(), "downloader-enabled", v.ID, fmt.Sprintf("下载器启用 / Downloader enabled: %t", v.Enabled))
	jsonOut(w, redacted(c))
}
