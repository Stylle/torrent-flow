package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var errTorrentGone = errors.New("torrent no longer exists")

type Adapter struct {
	c            Client
	http         *http.Client
	session      string
	trackerCache map[string]trackerEntry
	progress     func()
	endpoint     string
}
type trackerEntry struct {
	urls []string
	at   time.Time
}

func adapter(c Client) *Adapter {
	c = normalizeClient(c)
	jar, _ := cookiejar.New(nil)
	return &Adapter{c: c, endpoint: c.URL, http: &http.Client{Timeout: 5 * time.Second, Jar: jar, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}}
}
func normalizeClient(c Client) Client {
	c.URL = strings.TrimSpace(c.URL)
	if c.Kind != "tr" {
		return c
	}
	u, e := url.Parse(c.URL)
	if e != nil {
		return c
	}
	p := strings.TrimRight(u.Path, "/")
	lower := strings.ToLower(p)
	isWeb := strings.HasSuffix(lower, "/web") || strings.Contains(lower, "/web/")
	switch {
	case p == "":
		p = "/transmission/rpc"
	case strings.HasSuffix(lower, "/transmission"):
		p += "/rpc"
	case strings.HasSuffix(lower, "/web"):
		p = p[:len(p)-4] + "/rpc"
	case strings.Contains(lower, "/web/"):
		p = p[:strings.Index(lower, "/web/")] + "/rpc"
	}
	u.Path = p
	u.RawPath = ""
	u.Fragment = ""
	if isWeb {
		u.RawQuery = ""
	}
	c.URL = u.String()
	return c
}
func effectivePort(u *url.URL) string {
	if u.Port() != "" {
		return u.Port()
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}
func safeRPCDestination(base, target *url.URL) bool {
	if target.User != nil || !strings.EqualFold(base.Hostname(), target.Hostname()) {
		return false
	}
	if target.Scheme == base.Scheme {
		return effectivePort(base) == effectivePort(target)
	}
	return base.Scheme == "http" && target.Scheme == "https" && (effectivePort(base) == effectivePort(target) || (effectivePort(base) == "80" && effectivePort(target) == "443"))
}
func (a *Adapter) request(ctx context.Context, path string, body []byte, ctype string) ([]byte, int, error) {
	method := "GET"
	if body != nil {
		method = "POST"
	}
	u := strings.TrimRight(a.c.URL, "/") + path
	if a.c.Kind == "tr" && a.endpoint != "" {
		u = a.endpoint
	}
	base, _ := url.Parse(a.c.URL)
	for hop := 0; hop < 4; hop++ {
		req, e := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(body))
		if e != nil {
			return nil, 0, fmt.Errorf("invalid downloader request")
		}
		if ctype != "" {
			req.Header.Set("Content-Type", ctype)
		}
		if a.c.Kind == "tr" {
			req.SetBasicAuth(a.c.User, a.c.Password)
			if a.session != "" {
				req.Header.Set("X-Transmission-Session-Id", a.session)
			}
		} else {
			req.Header.Set("Referer", strings.TrimRight(a.c.URL, "/")+"/")
		}
		r, e := a.http.Do(req)
		if e != nil {
			return nil, 0, fmt.Errorf("downloader unreachable or request timed out")
		}
		defer r.Body.Close()
		if a.c.Kind == "tr" && (r.StatusCode == 301 || r.StatusCode == 302 || r.StatusCode == 307 || r.StatusCode == 308) {
			current, _ := url.Parse(u)
			location, e := r.Location()
			io.Copy(io.Discard, io.LimitReader(r.Body, 4096))
			r.Body.Close()
			if e != nil {
				return nil, r.StatusCode, fmt.Errorf("Transmission redirect has no valid destination")
			}
			target := current.ResolveReference(location)
			if !safeRPCDestination(base, target) || !safeRPCDestination(current, target) {
				return nil, r.StatusCode, fmt.Errorf("Transmission redirects to another host/port or an insecure address; enter the final RPC address explicitly")
			}
			u = target.String()
			continue
		}
		if a.c.Kind == "tr" {
			a.endpoint = u
		}
		if a.c.Kind == "tr" && r.StatusCode == 409 {
			a.session = r.Header.Get("X-Transmission-Session-Id")
		}
		b, e := io.ReadAll(io.LimitReader(r.Body, 32<<20))
		return b, r.StatusCode, e
	}
	return nil, 0, fmt.Errorf("Transmission RPC redirect loop; check RPC endpoint")
}
func (a *Adapter) qb(ctx context.Context, path string, form url.Values) ([]byte, error) {
	var body []byte
	if form != nil {
		body = []byte(form.Encode())
	}
	b, status, e := a.request(ctx, "/api/v2/"+path, body, "application/x-www-form-urlencoded")
	if e != nil {
		return nil, e
	}
	if status == 403 {
		v := url.Values{"username": {a.c.User}, "password": {a.c.Password}}
		loginBody, code, loginErr := a.request(ctx, "/api/v2/auth/login", []byte(v.Encode()), "application/x-www-form-urlencoded")
		if loginErr != nil || code != 200 || string(loginBody) != "Ok." {
			return nil, fmt.Errorf("qBittorrent authentication failed")
		}
		b, status, e = a.request(ctx, "/api/v2/"+path, body, "application/x-www-form-urlencoded")
		if e != nil {
			return nil, e
		}
	}
	if status != 200 {
		return nil, fmt.Errorf("qBittorrent API returned HTTP %d", status)
	}
	return b, nil
}
func (a *Adapter) rpc(ctx context.Context, method string, args any, out any) error {
	body, _ := json.Marshal(map[string]any{"method": method, "arguments": args})
	for i := 0; i < 2; i++ {
		b, status, e := a.request(ctx, "", body, "application/json")
		if e != nil {
			return e
		}
		if status == 409 {
			if a.session == "" {
				return fmt.Errorf("Transmission session header missing; check reverse proxy X-Transmission-Session-Id forwarding")
			}
			continue
		}
		if status != 200 {
			switch status {
			case 401:
				return fmt.Errorf("Transmission RPC HTTP 401: check RPC username/password")
			case 403, 421:
				return fmt.Errorf("Transmission RPC HTTP %d: check RPC IP/host whitelist and proxy access policy", status)
			case 404:
				return fmt.Errorf("Transmission RPC HTTP 404: check RPC path (usually /transmission/rpc)")
			}
			return fmt.Errorf("Transmission RPC returned HTTP %d", status)
		}
		var result struct {
			Result    string          `json:"result"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if e = json.Unmarshal(b, &result); e != nil {
			return fmt.Errorf("Transmission response is not RPC JSON; use the RPC address rather than the Web login page")
		}
		if result.Result != "success" {
			return fmt.Errorf("Transmission request rejected")
		}
		if out != nil {
			return json.Unmarshal(result.Arguments, out)
		}
		return nil
	}
	return fmt.Errorf("Transmission session negotiation failed")
}
func (a *Adapter) list(ctx context.Context) ([]Torrent, error) {
	out := []Torrent{}
	if a.c.Kind == "qb" {
		b, e := a.qb(ctx, "torrents/info", nil)
		if e != nil {
			return nil, e
		}
		var rows []struct {
			Hash        string `json:"hash"`
			Name        string `json:"name"`
			Uploaded    int64  `json:"uploaded"`
			Downloaded  int64  `json:"downloaded"`
			Up          int64  `json:"upspeed"`
			Down        int64  `json:"dlspeed"`
			Limit       int64  `json:"up_limit"`
			Connections int    `json:"num_leechs"`
			Tracker     string `json:"tracker"`
			Added       int64  `json:"added_on"`
		}
		if e = json.Unmarshal(b, &rows); e != nil {
			return nil, fmt.Errorf("invalid qBittorrent torrent list")
		}
		trackerRequests := 0
		for _, r := range rows {
			t := Torrent{Client: a.c.ID, Hash: strings.ToLower(r.Hash), Name: r.Name, Uploaded: r.Uploaded, Downloaded: r.Downloaded, Up: r.Up, Down: r.Down, Limit: r.Limit, Connections: r.Connections, Added: r.Added}
			if t.Limit < 0 {
				t.Limit = 0
			}
			t.Key = a.c.ID + ":" + t.Hash
			if a.progress != nil {
				a.progress()
			}
			if a.trackerCache == nil {
				a.trackerCache = map[string]trackerEntry{}
			}
			cached, ok := a.trackerCache[t.Hash]
			primaryAvailable := host(r.Tracker) != ""
			primaryMatches := !primaryAvailable
			for _, raw := range cached.urls {
				if raw == r.Tracker {
					primaryMatches = true
				}
			}
			if ok && primaryMatches && time.Since(cached.at) < 30*time.Minute {
				if primaryAvailable {
					t.Trackers = append([]string{r.Tracker}, cached.urls...)
				} else {
					t.Trackers = cached.urls
				}
				out = append(out, t)
				continue
			}
			if trackerRequests >= 32 {
				if ok && primaryMatches {
					t.Trackers = append(t.Trackers, cached.urls...)
				}
				if host(r.Tracker) != "" {
					t.Trackers = append([]string{r.Tracker}, t.Trackers...)
				}
				out = append(out, t)
				continue
			}
			trackerRequests++
			tb, e := a.qb(ctx, "torrents/trackers?hash="+url.QueryEscape(t.Hash), nil)
			if e != nil {
				if host(r.Tracker) != "" {
					t.Trackers = []string{r.Tracker}
				} else if ok {
					t.Trackers = cached.urls
				}
				out = append(out, t)
				continue
			}
			if host(r.Tracker) != "" {
				t.Trackers = append(t.Trackers, r.Tracker)
			}
			var trackers []struct {
				URL string `json:"url"`
			}
			if json.Unmarshal(tb, &trackers) != nil {
				return nil, fmt.Errorf("invalid qBittorrent tracker list")
			}
			for _, tr := range trackers {
				if host(tr.URL) != "" {
					t.Trackers = append(t.Trackers, tr.URL)
				}
			}
			a.trackerCache[t.Hash] = trackerEntry{t.Trackers, time.Now()}
			out = append(out, t)
		}
	} else {
		var r struct {
			Torrents []struct {
				Hash        string `json:"hashString"`
				Name        string `json:"name"`
				Uploaded    int64  `json:"uploadedEver"`
				Downloaded  int64  `json:"downloadedEver"`
				Up          int64  `json:"rateUpload"`
				Down        int64  `json:"rateDownload"`
				Limit       int64  `json:"uploadLimit"`
				Limited     bool   `json:"uploadLimited"`
				Connections int    `json:"peersGettingFromUs"`
				Added       int64  `json:"addedDate"`
				Trackers    []struct {
					Announce string `json:"announce"`
				} `json:"trackers"`
				Peers []struct {
					Interested bool `json:"peerIsInterested"`
				} `json:"peers"`
			} `json:"torrents"`
		}
		fields := []string{"hashString", "name", "uploadedEver", "downloadedEver", "rateUpload", "rateDownload", "uploadLimit", "uploadLimited", "peersGettingFromUs", "addedDate", "trackers", "peers"}
		if e := a.rpc(ctx, "torrent-get", map[string]any{"fields": fields}, &r); e != nil {
			return nil, e
		}
		for _, v := range r.Torrents {
			t := Torrent{Client: a.c.ID, Hash: strings.ToLower(v.Hash), Name: v.Name, Uploaded: v.Uploaded, Downloaded: v.Downloaded, Up: v.Up, Down: v.Down, Connections: v.Connections, Added: v.Added}
			interested := 0
			for _, p := range v.Peers {
				if p.Interested {
					interested++
				}
			}
			if interested > t.Connections {
				t.Connections = interested
			}
			if v.Limited {
				t.Limit = v.Limit * 1000
			}
			for _, tr := range v.Trackers {
				t.Trackers = append(t.Trackers, tr.Announce)
			}
			t.Key = a.c.ID + ":" + t.Hash
			out = append(out, t)
		}
	}
	for i := range out {
		seen := map[string]bool{}
		for _, tr := range out[i].Trackers {
			h := host(tr)
			if h != "" && !seen[h] {
				out[i].Hosts = append(out[i].Hosts, h)
				seen[h] = true
			}
		}
	}
	return out, nil
}
func (a *Adapter) setLimit(ctx context.Context, hash string, limit int64) error {
	if limit < 0 {
		return fmt.Errorf("negative limit")
	}
	if a.c.Kind == "qb" {
		_, e := a.qb(ctx, "torrents/setUploadLimit", url.Values{"hashes": {hash}, "limit": {strconv.FormatInt(limit, 10)}})
		return e
	}
	return a.rpc(ctx, "torrent-set", map[string]any{"ids": []string{hash}, "uploadLimited": limit > 0, "uploadLimit": (limit + 999) / 1000}, nil)
}
func (a *Adapter) readLimit(ctx context.Context, hash string) (int64, error) {
	if a.c.Kind == "qb" {
		b, e := a.qb(ctx, "torrents/uploadLimit", url.Values{"hashes": {hash}})
		if e != nil {
			return 0, e
		}
		var m map[string]int64
		if e = json.Unmarshal(b, &m); e != nil {
			return 0, e
		}
		v, ok := m[hash]
		if !ok {
			return 0, errTorrentGone
		}
		if v < 0 {
			v = 0
		}
		return v, nil
	}
	var r struct {
		Torrents []struct {
			Limit   int64 `json:"uploadLimit"`
			Enabled bool  `json:"uploadLimited"`
		} `json:"torrents"`
	}
	e := a.rpc(ctx, "torrent-get", map[string]any{"ids": []string{hash}, "fields": []string{"uploadLimit", "uploadLimited"}}, &r)
	if e != nil {
		return 0, e
	}
	if len(r.Torrents) != 1 {
		return 0, errTorrentGone
	}
	if !r.Torrents[0].Enabled {
		return 0, nil
	}
	return r.Torrents[0].Limit * 1000, nil
}
func normalizedLimit(c Client, v int64) int64 {
	if c.Kind == "tr" && v > 0 {
		return (v + 999) / 1000 * 1000
	}
	return v
}
