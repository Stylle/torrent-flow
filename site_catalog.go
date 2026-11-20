package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed site_catalog.json
var bundledCatalog []byte

type CatalogEntry struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Domains []string `json:"domains"`
}
type CatalogState struct {
	Version string         `json:"version"`
	Checked int64          `json:"checked"`
	Updated int64          `json:"updated"`
	Auto    *bool          `json:"auto"`
	Error   string         `json:"error"`
	Entries []CatalogEntry `json:"entries"`
}
type SiteCatalog struct {
	mu         sync.Mutex
	updateMu   sync.Mutex
	s          *Store
	state      CatalogState
	indexCache map[string]CatalogEntry
}

func newSiteCatalog(s *Store) *SiteCatalog {
	var state CatalogState
	json.Unmarshal(bundledCatalog, &state)
	var raw []byte
	if s.db.QueryRow("SELECT v FROM settings WHERE k='site-catalog'").Scan(&raw) == nil {
		var cached CatalogState
		if json.Unmarshal(raw, &cached) == nil && len(cached.Entries) > 0 {
			state = cached
		}
	}
	return &SiteCatalog{s: s, state: state}
}
func (c *SiteCatalog) snapshot() CatalogState { c.mu.Lock(); defer c.mu.Unlock(); return c.state }
func (c *SiteCatalog) status() map[string]any {
	s := c.snapshot()
	domains := 0
	for _, e := range s.Entries {
		domains += len(e.Domains)
	}
	return map[string]any{"version": s.Version, "checked": s.Checked, "updated": s.Updated, "auto": s.Auto == nil || *s.Auto, "error": s.Error, "sites": len(s.Entries), "domains": domains, "source": "pt-plugins/PT-depiler"}
}
func (c *SiteCatalog) persist(s CatalogState) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if _, err = c.s.db.Exec("INSERT OR REPLACE INTO settings VALUES('site-catalog',?)", raw); err == nil {
		c.state = s
		c.indexCache = nil
	}
	return err
}
func (c *SiteCatalog) setAuto(on bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.state
	s.Auto = &on
	return c.persist(s)
}
func (c *SiteCatalog) index() map[string]CatalogEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.indexCache != nil {
		return c.indexCache
	}
	out := map[string]CatalogEntry{}
	conflicts := map[string]bool{}
	for _, e := range c.state.Entries {
		for _, d := range e.Domains {
			if old, ok := out[d]; ok && old.ID != e.ID {
				conflicts[d] = true
			}
			out[d] = e
		}
	}
	for d := range conflicts {
		out[d] = CatalogEntry{}
	}
	c.indexCache = out
	return out
}
func catalogLookup(index map[string]CatalogEntry, h string) (CatalogEntry, bool) {
	for strings.Contains(h, ".") {
		if e, ok := index[h]; ok {
			return e, e.ID != ""
		}
		at := strings.IndexByte(h, '.')
		h = h[at+1:]
	}
	return CatalogEntry{}, false
}
func (c *SiteCatalog) run(ctx context.Context) {
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			s := c.snapshot()
			if (s.Auto == nil || *s.Auto) && time.Now().Unix()-s.Checked >= 86400 {
				c.update(ctx)
			}
			timer.Reset(time.Hour)
		}
	}
}
func catalogHTTP(ctx context.Context, client *http.Client, target string, limit int64) ([]byte, error) {
	r, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return nil, err
	}
	r.Header.Set("User-Agent", "Torrent-Flow-site-catalog")
	res, err := client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("站点库连接失败 / Catalog connection failed")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("站点库 HTTP %d / Catalog HTTP %d", res.StatusCode, res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil || int64(len(raw)) > limit {
		return nil, fmt.Errorf("站点库响应无效或过大 / Invalid catalog response")
	}
	return raw, nil
}
func fetchCatalog(ctx context.Context, client *http.Client, oldVersion string) (CatalogState, error) {
	raw, err := catalogHTTP(ctx, client, "https://api.github.com/repos/pt-plugins/PT-depiler/commits/master", 1<<20)
	if err != nil {
		return CatalogState{}, err
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	if json.Unmarshal(raw, &commit) != nil || !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(commit.SHA) {
		return CatalogState{}, fmt.Errorf("站点库版本无效 / Invalid catalog version")
	}
	if commit.SHA == oldVersion {
		return CatalogState{Version: commit.SHA}, nil
	}
	raw, err = catalogHTTP(ctx, client, "https://codeload.github.com/pt-plugins/PT-depiler/tar.gz/"+commit.SHA, 32<<20)
	if err != nil {
		return CatalogState{}, err
	}
	entries, err := parseCatalogArchive(raw)
	if err != nil {
		return CatalogState{}, err
	}
	return CatalogState{Version: commit.SHA, Entries: entries}, nil
}
func (c *SiteCatalog) update(ctx context.Context) error {
	return c.updateWithClient(ctx, &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(r *http.Request, v []*http.Request) error { return http.ErrUseLastResponse }})
}
func (c *SiteCatalog) updateWithClient(ctx context.Context, client *http.Client) error {
	if !c.updateMu.TryLock() {
		return fmt.Errorf("站点库正在更新 / Catalog update in progress")
	}
	defer c.updateMu.Unlock()
	old := c.snapshot()
	next, err := fetchCatalog(ctx, client, old.Version)
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.state
	s.Checked = time.Now().Unix()
	s.Error = ""
	if err != nil {
		s.Error = err.Error()
	} else if len(next.Entries) > 0 {
		s.Version = next.Version
		s.Entries = next.Entries
		s.Updated = s.Checked
	}
	if saveErr := c.persist(s); saveErr != nil {
		return saveErr
	}
	return err
}

var metadataBlock = regexp.MustCompile(`(?ms)^export const siteMetadata[^=\n]*=\s*\{(.*?)^\};`)
var jsString = regexp.MustCompile(`"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'`)

func literalString(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return "", false
	}
	if s[0] == '"' {
		v, e := strconv.Unquote(s)
		return v, e == nil
	}
	if s[0] == '\'' && s[len(s)-1] == '\'' && !strings.Contains(s[1:len(s)-1], "\\") {
		return s[1 : len(s)-1], true
	}
	return "", false
}
func metadataString(block, key string) string {
	r := regexp.MustCompile(`(?m)^  ` + key + `:\s*("(?:\\.|[^"\\])*"|'[^'\\]*')\s*,?\s*$`)
	m := r.FindStringSubmatch(block)
	if len(m) < 2 {
		return ""
	}
	v, _ := literalString(m[1])
	return v
}
func catalogDomains(block, key string) []string {
	r := regexp.MustCompile(`(?ms)^  ` + key + `:\s*\[([^\]]*)\]`)
	m := r.FindStringSubmatch(block)
	if len(m) < 2 {
		return nil
	}
	body := regexp.MustCompile(`"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|//[^\n]*|/\*[^*]*\*+(?:[^/*][^*]*\*+)*/`).ReplaceAllStringFunc(m[1], func(token string) string {
		if strings.HasPrefix(token, "//") || strings.HasPrefix(token, "/*") {
			return " "
		}
		return token
	})
	parts := jsString.FindAllString(body, -1)
	rest := jsString.ReplaceAllString(body, "")
	rest = regexp.MustCompile(`(?m)//[^\n]*`).ReplaceAllString(rest, "")
	rest = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(rest, "")
	if strings.Trim(rest, " \r\n\t,") != "" {
		return nil
	}
	var out []string
	for _, p := range parts {
		v, ok := literalString(p)
		if !ok {
			continue
		}
		if strings.HasPrefix(v, "uggc:") || strings.HasPrefix(v, "uggcf:") {
			v = strings.Map(func(r rune) rune {
				switch {
				case r >= 'a' && r <= 'z':
					return 'a' + (r-'a'+13)%26
				case r >= 'A' && r <= 'Z':
					return 'A' + (r-'A'+13)%26
				}
				return r
			}, v)
		}
		u, err := url.Parse(v)
		if err != nil || u.User != nil || u.RawQuery != "" || (u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "udp") {
			continue
		}
		h := host(v)
		if trackerHostname.MatchString(h) && strings.Contains(h, ".") {
			out = append(out, h)
		}
	}
	return out
}
func parseCatalogDefinition(src string) (CatalogEntry, bool) {
	m := metadataBlock.FindStringSubmatch(src)
	if len(m) < 2 {
		return CatalogEntry{}, false
	}
	b := m[1]
	if regexp.MustCompile(`(?m)^  isDead:\s*true`).MatchString(b) {
		return CatalogEntry{}, false
	}
	e := CatalogEntry{ID: metadataString(b, "id"), Name: metadataString(b, "name")}
	if !regexp.MustCompile(`^[a-z0-9]{1,80}$`).MatchString(e.ID) || e.Name == "" || len(e.Name) > 240 {
		return e, false
	}
	seen := map[string]bool{}
	for _, key := range []string{"trackerUrls", "urls", "legacyUrls"} {
		for _, d := range catalogDomains(b, key) {
			if !seen[d] {
				e.Domains = append(e.Domains, d)
				seen[d] = true
			}
		}
	}
	return e, len(e.Domains) > 0
}
func parseCatalogArchive(raw []byte) ([]CatalogEntry, error) {
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("无效站点库压缩包 / Invalid catalog archive")
	}
	defer gz.Close()
	expanded := &io.LimitedReader{R: gz, N: 96 << 20}
	tr := tar.NewReader(expanded)
	var entries []CatalogEntry
	count := 0
	seen := map[string]bool{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			if expanded.N == 0 {
				return nil, fmt.Errorf("catalog expansion limit exceeded")
			}
			break
		}
		if err != nil {
			return nil, fmt.Errorf("站点库不完整 / Incomplete catalog archive")
		}
		if h.Typeflag != tar.TypeReg || !strings.Contains(h.Name, "/src/packages/site/definitions/") || !strings.HasSuffix(h.Name, ".ts") {
			continue
		}
		count++
		if count > 1200 || h.Size > 2<<20 {
			return nil, fmt.Errorf("站点库超出限制 / Catalog size exceeded")
		}
		src, err := io.ReadAll(io.LimitReader(tr, 2<<20))
		if err != nil {
			return nil, err
		}
		if e, ok := parseCatalogDefinition(string(src)); ok && !seen[e.ID] {
			seen[e.ID] = true
			entries = append(entries, e)
		}
	}
	if len(entries) < 100 {
		return nil, fmt.Errorf("站点库有效条目过少，保留原库 / Too few valid catalog entries")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return entries, nil
}
func (sv *Server) catalogSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		jsonOut(w, sv.e.catalog.status())
		return
	}
	var v struct {
		Auto   *bool `json:"auto"`
		Update bool  `json:"update"`
	}
	if decode(w, r, &v) != nil {
		failHTTP(w, 400, "invalid catalog action")
		return
	}
	if v.Auto != nil {
		if err := sv.e.catalog.setAuto(*v.Auto); err != nil {
			failHTTP(w, 500, "cannot save catalog settings")
			return
		}
	}
	if v.Update {
		if err := sv.e.catalog.update(r.Context()); err != nil {
			failHTTP(w, 502, err.Error())
			return
		}
	}
	jsonOut(w, sv.e.catalog.status())
}

func (c *SiteCatalog) reload() {
	c.updateMu.Lock()
	defer c.updateMu.Unlock()
	next := newSiteCatalog(c.s)
	c.mu.Lock()
	c.state = next.state
	c.indexCache = nil
	c.mu.Unlock()
}
