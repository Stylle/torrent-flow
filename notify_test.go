package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWebhookTemplateEscapesAndDoesNotRedirect(t *testing.T) {
	calls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("X-Fixture") != "fixture" {
			t.Error("custom headers missing")
		}
		var body map[string]string
		if json.NewDecoder(r.Body).Decode(&body) != nil || body["content"] != "line1\n\"quoted\"" {
			t.Error("template escaping failed")
		}
		w.WriteHeader(204)
	}))
	defer target.Close()
	n := NotifyConfig{URL: target.URL, Method: "POST", Headers: `{"X-Fixture":"fixture"}`, BodyType: "json", BodyTemplate: `{"title":"{{title}}","content":"{{content}}"}`}
	if _, err := sendWebhook(context.Background(), n, "Title", "line1\n\"quoted\"", "test", "fixture-id"); err != nil {
		t.Fatal(err)
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer redirect.Close()
	n.URL = redirect.URL
	if _, err := sendWebhook(context.Background(), n, "Title", "content", "test", ""); err == nil || calls != 1 {
		t.Fatal("webhook followed redirect")
	}
}
func TestDraftWebhookDoesNotSaveOrEnable(t *testing.T) {
	s := testStore(t)
	sv := &Server{s: s, e: newEngine(s)}
	calls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(204) }))
	defer target.Close()
	body, _ := json.Marshal(NotifyConfig{URL: target.URL, Enabled: true})
	rec := httptest.NewRecorder()
	sv.testWebhook(rec, httptest.NewRequest("POST", "/api/test-webhook", bytes.NewReader(body)))
	c, _ := s.config()
	if rec.Code != 200 || calls != 1 || c.Notify.Enabled || c.Notify.URL != "" {
		t.Fatal("draft test saved settings or failed")
	}
}
func TestReportsPersistentRetryAndDedup(t *testing.T) {
	s := testStore(t)
	sv := &Server{s: s, e: newEngine(s)}
	calls := 0
	fail := true
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if fail {
			w.WriteHeader(503)
		} else {
			w.WriteHeader(204)
		}
	}))
	defer target.Close()
	c := defaultConfig()
	c.Timezone = "UTC"
	c.Notify = NotifyConfig{Enabled: true, URL: target.URL, Daily: true, ReportTime: "08:00"}
	s.saveConfig(c)
	now := time.Date(2026, 6, 1, 8, 1, 0, 0, time.UTC)
	sv.notificationTick(context.Background(), now)
	var attempts int
	var state string
	s.db.QueryRow("SELECT attempts,state FROM notification_jobs").Scan(&attempts, &state)
	if attempts != 1 || state != "pending" {
		t.Fatal("failed delivery was not persisted")
	}
	sv.notificationTick(context.Background(), now.Add(30*time.Second))
	if calls != 1 {
		t.Fatal("retry ignored backoff")
	}
	fail = false
	restarted := &Server{s: s, e: newEngine(s)}
	restarted.notificationTick(context.Background(), now.Add(61*time.Second))
	restarted.notificationTick(context.Background(), now.Add(3*time.Minute))
	s.db.QueryRow("SELECT attempts,state FROM notification_jobs").Scan(&attempts, &state)
	if calls != 2 || attempts != 2 || state != "sent" {
		t.Fatal("restart retry or dedup failed")
	}
	c.Notify.Weekly = true
	c.Notify.Monthly = true
	s.saveConfig(c)
	restarted.notificationTick(context.Background(), now.Add(4*time.Minute))
	if calls != 4 {
		t.Fatal("weekly/monthly boundaries not delivered")
	}
	c.Notify.Headers = `{"Authorization":"fixture-only"}`
	if redacted(c).Notify.Headers != "" || !redacted(c).Notify.HeadersStored {
		t.Fatal("headers leaked in settings")
	}
	if !strings.Contains(c.Notify.Headers, "fixture") {
		t.Fatal("redaction mutated original")
	}
}
