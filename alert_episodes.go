package main

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

var alertRates = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?\s*(?:KB/s|MB/s|GB/s|B/s)`)
var alertCounts = regexp.MustCompile(`(有 |活跃 )[0-9]+( 个种子)`)

// Normalize changing measurements, but retain digits in downloader/outlet names.
func alertIdentity(message string) string {
	message = alertRates.ReplaceAllString(message, "#rate")
	message = alertCounts.ReplaceAllString(message, "${1}#${2}")
	hash := sha256.Sum256([]byte(message))
	return fmt.Sprintf("%x", hash[:12])
}

// One persisted job per continuous incident. Resolution expires queued retries;
// recurrence gets a fresh identity, while restart cannot resend an acknowledged job.
func (s *Store) syncAlertEpisodes(messages []string, now time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	previous := map[string]string{}
	var raw []byte
	err = tx.QueryRow("SELECT v FROM settings WHERE k='alert_episodes_v1'").Scan(&raw)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	stored := raw
	if len(raw) > 0 {
		if err = json.Unmarshal(raw, &previous); err != nil {
			return err
		}
	}
	active := map[string]string{}
	for _, message := range messages {
		key := alertIdentity(message)
		if _, ok := active[key]; ok {
			continue
		}
		id := previous[key]
		if id == "" {
			id = "alert:" + key + ":" + randomID()
			if _, err = tx.Exec("INSERT INTO notification_jobs VALUES(?,?,?,?,?,?,0,'pending',0,'')", id, "alert", "流量台 · 告警", message, now.Unix(), now.Unix()); err != nil {
				return err
			}
		}
		active[key] = id
	}
	rows, err := tx.Query("SELECT id FROM notification_jobs WHERE kind='alert' AND state='pending'")
	if err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, id := range active {
		keep[id] = true
	}
	var expired []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		if !keep[id] {
			expired = append(expired, id)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range expired {
		if _, err = tx.Exec("UPDATE notification_jobs SET state='expired' WHERE id=? AND state='pending'", id); err != nil {
			return err
		}
	}
	raw, err = json.Marshal(active)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, stored) {
		if _, err = tx.Exec("INSERT OR REPLACE INTO settings VALUES('alert_episodes_v1',?)", raw); err != nil {
			return err
		}
	}
	return tx.Commit()
}
