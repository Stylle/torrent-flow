package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Stylle/torrent-flow/internal/portablebackup"
	"golang.org/x/crypto/bcrypt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Portable archive validation is shared with Stylle/today-random-movie-web (MIT).
var flowBackupSpec = portablebackup.Spec{App: "torrent-flow", Database: "flow.sqlite", Key: "master.key", Tables: []string{"settings", "baseline", "traffic", "speeds", "leases", "control_blocks", "events", "sessions", "notification_jobs"}}

func (s *Store) directory() (string, error) {
	var seq int
	var name, file string
	err := s.db.QueryRow("PRAGMA database_list").Scan(&seq, &name, &file)
	return filepath.Dir(file), err
}
func readBackupConfig(stage string) (Config, error) {
	key, err := os.ReadFile(filepath.Join(stage, "master.key"))
	if err != nil {
		return Config{}, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return Config{}, err
	}
	a, err := cipher.NewGCM(block)
	if err != nil {
		return Config{}, err
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(stage, "flow.sqlite"))+"?mode=ro")
	if err != nil {
		return Config{}, err
	}
	defer db.Close()
	imported := &Store{db: db, aead: a}
	c, err := imported.config()
	if err != nil {
		return c, fmt.Errorf("backup key does not match settings")
	}
	var hash []byte
	if err = db.QueryRow("SELECT v FROM settings WHERE k='password'").Scan(&hash); err != nil {
		return c, err
	}
	if _, err = bcrypt.Cost(hash); err != nil {
		return c, fmt.Errorf("backup administrator is invalid")
	}
	return c, c.validate()
}
func (sv *Server) backup(w http.ResponseWriter, r *http.Request) {
	sv.notifyMu.Lock()
	defer sv.notifyMu.Unlock()
	sv.e.mu.Lock()
	defer sv.e.mu.Unlock()
	if !sv.stillAuthorized(r) {
		failHTTP(w, 401, "sign in required")
		return
	}
	dir, err := sv.s.directory()
	if err != nil {
		failHTTP(w, 500, "backup directory unavailable")
		return
	}
	if r.Method == "GET" {
		path := filepath.Join(dir, ".backup-"+randomID()+".zip")
		defer os.Remove(path)
		if portablebackup.Export(dir, sv.s.db, flowBackupSpec, path) != nil {
			failHTTP(w, 500, "创建完整备份失败 / Backup failed")
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="torrent-flow-`+time.Now().Format("20060102-150405")+`.zip"`)
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, path)
		return
	}
	if r.Header.Get("X-Confirm-Restore") != "replace" {
		failHTTP(w, 400, "请确认恢复覆盖数据 / Restore confirmation required")
		return
	}
	c, err := sv.s.config()
	if err != nil {
		failHTTP(w, 500, "configuration unavailable")
		return
	}
	var leases int
	sv.s.db.QueryRow("SELECT COUNT(*) FROM leases").Scan(&leases)
	running := false
	pending := c.ClearPending
	for _, cl := range c.Clients {
		pending = pending || cl.ClearPending || cl.RestorePending
		running = running || (cl.Enabled && policyOn(c) && clientPolicyOn(cl))
	}
	if running || leases > 0 || pending {
		failHTTP(w, 409, "请先停止所有下载器自动策略并等待限速清空 / Stop all downloader strategies and wait for clearance")
		return
	}
	stage, err := os.MkdirTemp(dir, ".import-")
	if err != nil {
		failHTTP(w, 500, "staging unavailable")
		return
	}
	defer os.RemoveAll(stage)
	upload := filepath.Join(stage, "upload.zip")
	f, err := os.OpenFile(upload, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		failHTTP(w, 500, "staging unavailable")
		return
	}
	_, err = io.Copy(f, http.MaxBytesReader(w, r.Body, portablebackup.Limit))
	ce := f.Close()
	if err != nil || ce != nil {
		failHTTP(w, 400, "备份上传不完整或过大 / Upload incomplete or too large")
		return
	}
	var imported Config
	if err = portablebackup.Stage(stage, upload, flowBackupSpec, func(path string) error { var e error; imported, e = readBackupConfig(path); return e }); err != nil {
		failHTTP(w, 400, "备份无效或不属于流量台，当前数据未修改 / Invalid Torrent Flow backup; current data unchanged")
		return
	}
	var plan struct{ Stage string }
	b, err := os.ReadFile(filepath.Join(stage, "restore-ready.json"))
	if err != nil || json.Unmarshal(b, &plan) != nil {
		failHTTP(w, 400, "invalid restore plan")
		return
	}
	source := filepath.Join(stage, plan.Stage, "flow.sqlite")
	previous := filepath.Join(dir, "previous-"+time.Now().Format("20060102-150405")+"-"+randomID()+".zip")
	if portablebackup.Export(dir, sv.s.db, flowBackupSpec, previous) != nil {
		failHTTP(w, 500, "无法保存恢复前备份 / Cannot preserve pre-restore backup")
		return
	}
	off := false
	imported.PolicyEnabled = &off
	imported.ClearPending = false
	for i := range imported.Clients {
		imported.Clients[i].StrategyEnabled = &off
		imported.Clients[i].ClearPending = false
		imported.Clients[i].RestorePending = false
	}
	imported.Revision++
	conn, err := sv.s.db.Conn(r.Context())
	if err != nil {
		failHTTP(w, 500, "database unavailable")
		return
	}
	defer sv.e.catalog.reload()
	defer conn.Close()
	if _, err = conn.ExecContext(r.Context(), "ATTACH DATABASE ? AS imported", source); err != nil {
		failHTTP(w, 400, "backup database unavailable")
		return
	}
	defer conn.ExecContext(context.Background(), "DETACH DATABASE imported")
	tx, err := conn.BeginTx(r.Context(), nil)
	if err != nil {
		failHTTP(w, 500, "restore transaction unavailable")
		return
	}
	defer tx.Rollback()
	for _, table := range []string{"baseline", "traffic", "speeds", "site_speeds", "lifetime", "control_blocks", "events", "notification_jobs"} {
		if _, err = tx.Exec("DELETE FROM " + table); err != nil {
			break
		}
		if table == "site_speeds" || table == "lifetime" {
			var present int
			if tx.QueryRow("SELECT COUNT(*) FROM imported.sqlite_master WHERE type='table' AND name=?", table).Scan(&present) != nil {
				err = fmt.Errorf("cannot inspect chart history")
				break
			}
			if present == 0 {
				if table == "lifetime" {
					_, err = tx.Exec("INSERT INTO lifetime SELECT 1,COALESCE(SUM(up),0),COALESCE(SUM(down),0),COALESCE(MAX(uncertain),0) FROM traffic")
					if err != nil {
						break
					}
				}
				continue
			}
		}
		if _, err = tx.Exec("INSERT INTO " + table + " SELECT * FROM imported." + table); err != nil {
			break
		}
	}
	if err == nil {
		var count, up, down int64
		if err = tx.QueryRow("SELECT COUNT(*),COALESCE(MIN(up),-1),COALESCE(MIN(down),-1) FROM lifetime WHERE id=1").Scan(&count, &up, &down); err == nil && (count != 1 || up < 0 || down < 0) {
			err = fmt.Errorf("invalid cumulative totals")
		}
	}
	if err == nil {
		_, err = tx.Exec("DELETE FROM settings WHERE k NOT IN ('controller','guardian')")
	}
	if err == nil {
		_, err = tx.Exec("INSERT INTO settings SELECT * FROM imported.settings WHERE k NOT IN ('config','controller','guardian')")
	}
	encoded, _ := json.Marshal(imported)
	if err == nil {
		_, err = tx.Exec("INSERT OR REPLACE INTO settings VALUES('config',?)", sv.s.seal(encoded))
	}
	if err == nil {
		_, err = tx.Exec("DELETE FROM sessions")
	}
	if err == nil {
		_, err = tx.Exec("DELETE FROM leases")
	}
	if err != nil {
		failHTTP(w, 400, "备份结构不兼容，当前数据未修改 / Incompatible backup; current data unchanged")
		return
	}
	if tx.Commit() != nil {
		failHTTP(w, 500, "restore failed")
		return
	}
	sv.speedEpoch.Add(1)
	sv.e.started = false
	sv.e.adapters = map[string]*Adapter{}
	sv.e.idle = map[string]time.Time{}
	sv.e.suspended = map[string]bool{}
	sv.e.stateMu.Lock()
	sv.e.torrents = []Torrent{}
	sv.e.statuses = []ClientStatus{}
	sv.e.stateMu.Unlock()
	jsonOut(w, map[string]any{"ok": true, "relogin": true, "message": "恢复完成，请使用备份账号重新登录。自动策略已暂停，请确认下载器地址后启动。 / Restored; sign in with the backup account. Strategies are paused until restarted."})
}
