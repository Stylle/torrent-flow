package main

import (
	"context"
	"fmt"
	"log"
	"time"
)

const diagnosticLimit = 100

// The controller owns episode state. The pending queue is also read by HTTP
// handlers, so it is protected independently of the controller lock.
func (e *Engine) diagnostic(kind, client, message string) {
	entry := Event{Time: time.Now().Unix(), Kind: "diag-" + kind, Key: client, Message: message}
	log.Printf("Diagnostic %s downloader=%q: %s", kind, client, message)
	e.diagnosticMu.Lock()
	defer e.diagnosticMu.Unlock()
	if len(e.pendingDiagnostics) == diagnosticLimit {
		copy(e.pendingDiagnostics, e.pendingDiagnostics[1:])
		e.pendingDiagnostics = e.pendingDiagnostics[:diagnosticLimit-1]
	}
	e.pendingDiagnostics = append(e.pendingDiagnostics, entry)
}

func (e *Engine) diagnosticCondition(kind, client, reason string) {
	key := [2]string{kind, client}
	prior := e.diagnosticConditions[key]
	if reason == prior {
		return
	}
	if reason == "" {
		delete(e.diagnosticConditions, key)
		e.diagnostic(kind, client, "Recovered")
		return
	}
	e.diagnosticConditions[key] = reason
	e.diagnostic(kind, client, reason)
}

func (e *Engine) flushDiagnostics(ctx context.Context) {
	e.diagnosticMu.Lock()
	defer e.diagnosticMu.Unlock()
	if len(e.pendingDiagnostics) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	tx, err := e.s.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	for _, entry := range e.pendingDiagnostics {
		if _, err := tx.ExecContext(ctx, "INSERT INTO events(at,kind,k,message) VALUES(?,?,?,?)", entry.Time, entry.Kind, entry.Key, entry.Message); err != nil {
			return
		}
	}
	if err := tx.Commit(); err == nil {
		e.pendingDiagnostics = e.pendingDiagnostics[:0]
	}
}

func (e *Engine) diagnostics(ctx context.Context) ([]Event, error) {
	e.diagnosticMu.Lock()
	defer e.diagnosticMu.Unlock()
	entries := make([]Event, 0, diagnosticLimit)
	for i := len(e.pendingDiagnostics) - 1; i >= 0; i-- {
		entries = append(entries, e.pendingDiagnostics[i])
	}
	rows, err := e.s.db.QueryContext(ctx, "SELECT at,kind,k,message FROM events WHERE kind GLOB 'diag-*' ORDER BY id DESC LIMIT ?", diagnosticLimit-len(entries))
	if err != nil {
		return entries, err
	}
	defer rows.Close()
	for rows.Next() {
		var entry Event
		if err := rows.Scan(&entry.Time, &entry.Kind, &entry.Key, &entry.Message); err != nil {
			return entries, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func samplingFailureMessage(failures int, err error) string {
	return fmt.Sprintf("Torrent list sampling failed (%d/%d consecutive cycles): %s", failures, downloaderAlertFailures, err)
}
