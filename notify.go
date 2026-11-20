package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

type NotifyConfig struct {
	Enabled       bool   `json:"enabled"`
	URL           string `json:"url"`
	Method        string `json:"method"`
	Headers       string `json:"headers"`
	HeadersStored bool   `json:"headersStored,omitempty"`
	BodyType      string `json:"bodyType"`
	BodyTemplate  string `json:"bodyTemplate"`
	Daily         bool   `json:"daily"`
	Weekly        bool   `json:"weekly"`
	Monthly       bool   `json:"monthly"`
	Alerts        bool   `json:"alerts"`
	ReportTime    string `json:"reportTime"`
}

func (n NotifyConfig) validate() error {
	if n.URL != "" {
		u, e := url.Parse(n.URL)
		if e != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
			return fmt.Errorf("Webhook 地址必须为 HTTP/HTTPS / Invalid webhook URL")
		}
	}
	if n.Enabled && n.URL == "" {
		return fmt.Errorf("请填写 Webhook 地址 / Webhook URL required")
	}
	if n.Method != "" && n.Method != "POST" && n.Method != "PUT" && n.Method != "PATCH" {
		return fmt.Errorf("Webhook method must be POST, PUT or PATCH")
	}
	if n.BodyType != "" && n.BodyType != "json" && n.BodyType != "text" {
		return fmt.Errorf("invalid webhook body type")
	}
	if len(n.Headers) > 32768 || len(n.BodyTemplate) > 131072 {
		return fmt.Errorf("Webhook template too large")
	}
	if n.ReportTime != "" {
		if _, err := time.Parse("15:04", n.ReportTime); err != nil {
			return fmt.Errorf("报告时间应为 HH:MM / Report time must be HH:MM")
		}
	}
	if n.Headers != "" {
		var h map[string]string
		if json.Unmarshal([]byte(n.Headers), &h) != nil {
			return fmt.Errorf("自定义请求头不是合法 JSON / Invalid JSON headers")
		}
		for k, v := range h {
			if strings.ContainsAny(k+v, "\r\n") || strings.ContainsAny(k, " :\t") {
				return fmt.Errorf("invalid webhook header")
			}
			switch strings.ToLower(k) {
			case "host", "content-length", "connection", "transfer-encoding":
				return fmt.Errorf("unsupported webhook transport header")
			}
		}
	}
	if n.BodyTemplate != "" && n.BodyType != "text" {
		if _, err := renderNotifyJSON(n.BodyTemplate, map[string]any{"title": "Test", "content": "line 1\nline 2", "event": "test", "text": "Test", "time": "2026-01-01", "fields": "{}"}); err != nil {
			return err
		}
	}
	return nil
}

// JSON escaping and supported {{fields}} behavior reused from Wusaqi Gallery.
var templateVar = regexp.MustCompile(`\{\{\w+\}\}`)

// Substitute JSON templates without turning quotes or multiline notification
// text into invalid JSON. {{fields}} also works as an unquoted JSON object.
func renderNotifyJSON(tpl string, ctx map[string]any) ([]byte, error) {
	var out strings.Builder
	quoted, escaped := false, false
	for i := 0; i < len(tpl); {
		if strings.HasPrefix(tpl[i:], "{{") {
			if match := templateVar.FindStringIndex(tpl[i:]); match != nil && match[0] == 0 {
				key := tpl[i+2 : i+match[1]-2]
				value, found := ctx[key]
				if quoted {
					b, _ := json.Marshal(fmt.Sprint(value))
					out.Write(b[1 : len(b)-1])
				} else if !found {
					out.WriteString("null")
				} else if key == "fields" && json.Valid([]byte(fmt.Sprint(value))) {
					out.WriteString(fmt.Sprint(value))
				} else {
					b, e := json.Marshal(value)
					if e != nil {
						return nil, e
					}
					out.Write(b)
				}
				i += match[1]
				continue
			}
		}
		c := tpl[i]
		out.WriteByte(c)
		if escaped {
			escaped = false
		} else if quoted && c == '\\' {
			escaped = true
		} else if c == '"' {
			quoted = !quoted
		}
		i++
	}
	result := []byte(out.String())
	if !json.Valid(result) {
		return nil, fmt.Errorf("请求体模板不是合法 JSON，请检查模板或选择 Text 格式")
	}
	return result, nil
}

func sendWebhook(ctx context.Context, n NotifyConfig, title, content, event, id string) (int, error) {
	if err := n.validate(); err != nil {
		return 0, err
	}
	if n.URL == "" {
		return 0, fmt.Errorf("Webhook URL required")
	}
	values := map[string]any{"title": title, "content": content, "text": content, "event": event, "time": time.Now().Format(time.RFC3339), "fields": "{}"}
	payload, _ := json.Marshal(map[string]string{"title": title, "content": content})
	contentType := "application/json"
	if n.BodyType == "text" {
		payload = []byte(content)
		contentType = "text/plain; charset=utf-8"
	}
	if n.BodyTemplate != "" {
		if n.BodyType == "text" {
			body := n.BodyTemplate
			for k, v := range values {
				body = strings.ReplaceAll(body, "{{"+k+"}}", fmt.Sprint(v))
			}
			payload = []byte(templateVar.ReplaceAllString(body, ""))
		} else {
			var err error
			payload, err = renderNotifyJSON(n.BodyTemplate, values)
			if err != nil {
				return 0, err
			}
		}
	}
	method := n.Method
	if method == "" {
		method = "POST"
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, n.URL, bytes.NewReader(payload))
	if err != nil {
		return 0, fmt.Errorf("invalid webhook request")
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "Torrent-Flow/"+Version)
	var h map[string]string
	json.Unmarshal([]byte(n.Headers), &h)
	for k, v := range h {
		req.Header.Set(k, v)
	}
	if id != "" {
		req.Header.Set("Idempotency-Key", id)
	}
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("Webhook 请求失败或超时 / Webhook request failed or timed out")
	}
	defer response.Body.Close()
	io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return response.StatusCode, fmt.Errorf("Webhook HTTP %d", response.StatusCode)
	}
	return response.StatusCode, nil
}
func (sv *Server) testWebhook(w http.ResponseWriter, r *http.Request) {
	var n NotifyConfig
	if decode(w, r, &n) != nil {
		failHTTP(w, 400, "invalid webhook configuration")
		return
	}
	c, err := sv.s.config()
	if err != nil {
		failHTTP(w, 500, "configuration unavailable")
		return
	}
	if n.Headers == "" {
		n.Headers = c.Notify.Headers
	}
	if err = n.validate(); err != nil {
		failHTTP(w, 400, err.Error())
		return
	}
	status, err := sendWebhook(r.Context(), n, "流量台测试通知 / Torrent Flow test", "这是当前表单的测试通知，不会保存或启用设置。\nDraft test only; settings were not saved or enabled.", "test", "")
	if err != nil {
		failHTTP(w, 502, err.Error())
		return
	}
	jsonOut(w, map[string]any{"ok": true, "status": status})
}
func (s *Store) queueNotification(id, kind, title, content string, now time.Time) {
	s.db.Exec("INSERT OR IGNORE INTO notification_jobs VALUES(?,?,?,?,?,?,0,'pending',0,'')", id, kind, title, content, now.Unix(), now.Unix())
}
func (s *Store) reportMessage(c Config, kind string, start, end time.Time) (string, error) {
	buckets, err := s.report(start, end)
	if err != nil {
		return "", err
	}
	var up, down int64
	totals := map[string]int64{}
	for _, b := range buckets {
		up += b.Up
		down += b.Down
		totals[b.Site] += b.Up
	}
	names := map[string]string{}
	for _, site := range c.Sites {
		names[site.ID] = site.Name
	}
	lines := []string{"流量台 · " + kind, "统计区间：" + start.Format("2006-01-02") + " — " + end.Add(-time.Second).Format("2006-01-02"), fmt.Sprintf("上传：%s\n下载：%s", formatBytes(up, false), formatBytes(down, false)), "站点上传："}
	ids := []string{}
	for id := range totals {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		name := names[id]
		if name == "" {
			name = "历史归属 / Historical attribution"
		}
		lines = append(lines, fmt.Sprintf("%s：%s", name, formatBytes(totals[id], false)))
	}
	lines = append(lines, "注：离线补记可能无法精确还原发生时间；报告覆盖实际保留的数据。")
	return strings.Join(lines, "\n"), nil
}
func (sv *Server) notificationTick(ctx context.Context, now time.Time) {
	sv.notifyMu.Lock()
	defer sv.notifyMu.Unlock()
	c, err := sv.s.config()
	if err != nil {
		return
	}
	n := c.Notify
	if !n.Enabled || n.URL == "" {
		return
	}
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return
	}
	now = now.In(loc)
	clock := n.ReportTime
	if clock == "" {
		clock = "08:00"
	}
	parsed, _ := time.Parse("15:04", clock)
	for _, v := range []struct {
		kind, title string
		enabled     bool
	}{{"day", "日报", n.Daily}, {"week", "周报", n.Weekly}, {"month", "月报", n.Monthly}} {
		if !v.enabled {
			continue
		}
		boundary, _ := period(now, v.kind)
		due := time.Date(boundary.Year(), boundary.Month(), boundary.Day(), parsed.Hour(), parsed.Minute(), 0, 0, loc)
		if now.Before(due) || now.Sub(due) > 24*time.Hour {
			continue
		}
		previous := boundary.Add(-time.Second)
		start, end := period(previous, v.kind)
		var exists int
		if sv.s.db.QueryRow("SELECT 1 FROM notification_jobs WHERE id=?", v.kind+":"+boundary.Format("20060102")).Scan(&exists) == nil {
			continue
		}
		body, e := sv.s.reportMessage(c, v.title, start, end)
		if e == nil {
			sv.s.queueNotification(v.kind+":"+boundary.Format("20060102"), v.kind, "流量台 · "+v.title, body, now)
		}
	}
	if n.Alerts {
		sv.e.stateMu.RLock()
		messages := append([]string{}, sv.e.warnings...)
		for _, cl := range sv.e.statuses {
			if !cl.Online {
				messages = append(messages, "下载器连接异常："+cl.Name)
			}
		}
		failures := map[string]int{}
		for _, t := range sv.e.torrents {
			if t.State != "" && t.State != "monitoring" && t.State != "allocated" && t.State != "released" {
				failures[t.Client]++
			}
		}
		sv.e.stateMu.RUnlock()
		for client, count := range failures {
			for _, cl := range c.Clients {
				if cl.ID == client {
					messages = append(messages, fmt.Sprintf("下载器 %s 有 %d 个种子限速操作异常，请查看运行记录", cl.Name, count))
				}
			}
		}
		running := false
		for _, cl := range c.Clients {
			running = running || (cl.Enabled && clientPolicyOn(cl) && policyOn(c))
		}
		if running && now.Unix()-heartbeat(sv.s, "guardian") > 20 {
			messages = append(messages, "恢复守护未连接，自动调速已暂停 / Recovery guardian offline; allocation suspended")
		}
		for _, cl := range c.Clients {
			if cl.ClearPending || cl.RestorePending {
				messages = append(messages, "下载器限速清空待重试："+cl.Name)
			}
		}
		if c.ClearPending {
			messages = append(messages, "系统限速清空尚未完成，正在重试")
		}
		for _, msg := range messages {
			hash := sha256.Sum256([]byte(regexp.MustCompile(`[0-9]+`).ReplaceAllString(msg, "#")))
			sv.s.queueNotification(fmt.Sprintf("alert:%x:%d", hash[:12], now.Unix()/900), "alert", "流量台 · 告警", msg, now)
		}
	}
	rows, err := sv.s.db.Query("SELECT id,kind,title,content,at,attempts FROM notification_jobs WHERE state='pending' AND next<=? ORDER BY at LIMIT 4", now.Unix())
	if err != nil {
		return
	}
	type job struct {
		id, kind, title, content string
		at                       int64
		attempts                 int
	}
	jobs := []job{}
	for rows.Next() {
		var j job
		rows.Scan(&j.id, &j.kind, &j.title, &j.content, &j.at, &j.attempts)
		jobs = append(jobs, j)
	}
	rows.Close()
	for _, j := range jobs {
		latest, e := sv.s.config()
		if e != nil {
			return
		}
		n = latest.Notify
		if !n.Enabled {
			return
		}
		enabled := map[string]bool{"day": n.Daily, "week": n.Weekly, "month": n.Monthly, "alert": n.Alerts}[j.kind]
		if !enabled || now.Unix()-j.at > 86400 {
			sv.s.db.Exec("UPDATE notification_jobs SET state='expired' WHERE id=?", j.id)
			continue
		}
		status, e := sendWebhook(ctx, n, j.title, j.content, j.kind, j.id)
		attempts := j.attempts + 1
		state := "sent"
		message := ""
		next := now.Unix()
		if e != nil {
			message = e.Error()
			state = "pending"
			delays := []int64{60, 300, 900}
			if attempts >= 4 {
				state = "failed"
			} else {
				next += delays[attempts-1]
			}
		}
		sv.s.db.Exec("UPDATE notification_jobs SET state=?,attempts=?,next=?,status=?,error=? WHERE id=?", state, attempts, next, status, message, j.id)
		sv.s.db.Exec("INSERT INTO events(at,kind,k,message)VALUES(?,?,?,?)", now.Unix(), "webhook", j.kind, fmt.Sprintf("Webhook %s · HTTP %d · attempt %d", state, status, attempts))
	}
	sv.s.db.Exec("DELETE FROM notification_jobs WHERE at<?", now.AddDate(0, 0, -366).Unix())
}
func (sv *Server) notifications(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			sv.notificationTick(ctx, now)
		}
	}
}
func (sv *Server) notificationJobs(w http.ResponseWriter, r *http.Request) {
	rows, err := sv.s.db.Query("SELECT kind,at,attempts,state,status,error FROM notification_jobs ORDER BY at DESC LIMIT 100")
	if err != nil {
		failHTTP(w, 500, "notification history unavailable")
		return
	}
	defer rows.Close()
	jobs := []map[string]any{}
	for rows.Next() {
		var kind, state, message string
		var at int64
		var attempts, status int
		rows.Scan(&kind, &at, &attempts, &state, &status, &message)
		jobs = append(jobs, map[string]any{"kind": kind, "time": at, "attempts": attempts, "state": state, "status": status, "error": message})
	}
	jsonOut(w, jobs)
}
