package main

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const Version = "0.1.13"

type Client struct {
	RestorePending  bool   `json:"restorePending"`
	StrategyEnabled *bool  `json:"strategyEnabled"`
	ClearPending    bool   `json:"clearPending"`
	AutoManage      bool   `json:"autoManage,omitempty"`
	ID              string `json:"id"`
	Name            string `json:"name"`
	Kind            string `json:"kind"`
	URL             string `json:"url"`
	User            string `json:"user"`
	Password        string `json:"password,omitempty"`
	Pool            string `json:"pool"`
	Enabled         bool   `json:"enabled"`
}
type Site struct {
	CatalogID   string   `json:"catalogId,omitempty"`
	Automatic   bool     `json:"automatic,omitempty"`
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Domains     []string `json:"domains"`
	Regex       string   `json:"regex"`
	Weight      int64    `json:"weight"`
	Minimum     int64    `json:"minimum"`
	Maximum     int64    `json:"maximum"`
	DayTarget   int64    `json:"dayTarget"`
	MonthTarget int64    `json:"monthTarget"`
	AfterTarget int64    `json:"afterTarget"`
}
type Pool struct {
	Schedule OutletSchedule `json:"schedule"`
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Budget   int64          `json:"budget"`
	Enabled  bool           `json:"enabled"`
}
type Override struct {
	Site   string `json:"site"`
	Weight int64  `json:"weight"`
	Manage bool   `json:"manage"`
}
type Config struct {
	Notify        NotifyConfig        `json:"notify"`
	ClearPending  bool                `json:"clearPending"`
	PolicyEnabled *bool               `json:"policyEnabled"`
	PolicyVersion int                 `json:"policyVersion"`
	Revision      int64               `json:"revision"`
	Clients       []Client            `json:"clients"`
	Sites         []Site              `json:"sites"`
	Pools         []Pool              `json:"pools"`
	Overrides     map[string]Override `json:"overrides"`
	Timezone      string              `json:"timezone"`
	PollSeconds   int                 `json:"pollSeconds"`
	IdleSeconds   int                 `json:"idleSeconds"`

	UploadConfirmSeconds int `json:"uploadConfirmSeconds"`
}

func clientPolicyOn(cl Client) bool { return cl.StrategyEnabled == nil || *cl.StrategyEnabled }
func policyOn(c Config) bool        { return c.PolicyEnabled == nil || *c.PolicyEnabled }

func defaultConfig() Config {
	return Config{Clients: []Client{}, Sites: []Site{}, Pools: []Pool{}, Overrides: map[string]Override{}, Timezone: "Asia/Shanghai", PollSeconds: 5, IdleSeconds: 15}
}

type Torrent struct {
	Downloading bool     `json:"downloading"`
	Progress    *float64 `json:"progress,omitempty"`
	Key         string   `json:"key"`
	Client      string   `json:"client"`
	Hash        string   `json:"hash"`
	Name        string   `json:"name"`
	Trackers    []string `json:"-"`
	Hosts       []string `json:"hosts"`
	Site        string   `json:"site"`
	Ambiguous   bool     `json:"ambiguous"`
	Uploaded    int64    `json:"uploaded"`
	Downloaded  int64    `json:"downloaded"`
	Up          int64    `json:"up"`
	Down        int64    `json:"down"`
	Connections int      `json:"connections"`
	Limit       int64    `json:"limit"`
	Managed     bool     `json:"managed"`
	State       string   `json:"state"`
	Added       int64    `json:"-"`

	UploadQualified bool `json:"uploadQualified,omitempty"`
}
type ClientStatus struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Kind            string `json:"kind"`
	Online          bool   `json:"online"`
	Up              int64  `json:"up"`
	Down            int64  `json:"down"`
	Error           string `json:"error,omitempty"`
	AsOf            int64  `json:"asOf"`
	LastSuccess     int64  `json:"lastSuccess,omitempty"`
	Failures        int    `json:"failures,omitempty"`
	StatisticsError bool   `json:"statisticsError,omitempty"`
}
type Bucket struct {
	Time      int64  `json:"time"`
	Site      string `json:"site"`
	Up        int64  `json:"up"`
	Down      int64  `json:"down"`
	Uncertain bool   `json:"uncertain"`
}
type Event struct {
	Time    int64  `json:"time"`
	Kind    string `json:"kind"`
	Key     string `json:"key"`
	Message string `json:"message"`
}

func host(raw string) string {
	u, e := url.Parse(raw)
	if e != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
}
func siteFor(t Torrent, c Config) (string, bool) {
	if o, ok := c.Overrides[t.Key]; ok && o.Site != "" {
		return o.Site, false
	}
	found := map[string]bool{}
	first := ""
	for _, s := range c.Sites {
		for _, v := range t.Trackers {
			h := host(v)
			for _, d := range s.Domains {
				d = strings.ToLower(strings.TrimSpace(d))
				if h != "" && (h == d || strings.HasSuffix(h, "."+d)) {
					found[s.ID] = true
					if first == "" {
						first = s.ID
					}
				}
			}
			if s.Regex != "" {
				re, e := regexp.Compile(s.Regex)
				if e == nil && re.MatchString(v) {
					found[s.ID] = true
					if first == "" {
						first = s.ID
					}
				}
			}
		}
	}
	if len(found) == 1 {
		for k := range found {
			return k, false
		}
	}
	if len(found) > 1 {
		manual := ""
		for _, s := range c.Sites {
			if found[s.ID] && !s.Automatic {
				if manual != "" {
					return "", true
				}
				manual = s.ID
			}
		}
		if manual != "" {
			return manual, false
		}
		// Preserve downloader tracker order when all matches are automatic.
		for _, raw := range t.Trackers {
			h := host(raw)
			for _, s := range c.Sites {
				for _, d := range s.Domains {
					if found[s.ID] && (h == strings.ToLower(strings.TrimSpace(d)) || strings.HasSuffix(h, "."+strings.ToLower(strings.TrimSpace(d)))) {
						return s.ID, false
					}
				}
			}
		}
		return first, false
	}
	return "", false
}

// Per-torrent explicit choices override downloader-wide opt-in, including exclusions.
func (c Config) manages(t Torrent) bool {
	if o, ok := c.Overrides[t.Key]; ok {
		return o.Manage
	}
	for _, cl := range c.Clients {
		if cl.ID == t.Client {
			return cl.AutoManage
		}
	}
	return false
}

func (c Config) validate() error {
	if err := c.Notify.validate(); err != nil {
		return err
	}
	if len(c.Clients) > 32 || len(c.Sites) > 256 || len(c.Pools) > 32 || len(c.Overrides) > 50000 {
		return fmt.Errorf("configuration exceeds supported limits")
	}
	if _, e := time.LoadLocation(c.Timezone); e != nil {
		return fmt.Errorf("invalid timezone")
	}
	if c.PollSeconds < 5 || c.PollSeconds > 60 || c.IdleSeconds < 10 || c.IdleSeconds > 300 {
		return fmt.Errorf("invalid sampling/idle interval")
	}
	if c.UploadConfirmSeconds < 0 || c.UploadConfirmSeconds > 300 {
		return fmt.Errorf("持续上传确认时间必须为 0–300 秒 / Upload confirmation must be 0–300 seconds")
	}
	ids := map[string]bool{}
	check := func(id string) bool {
		if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`).MatchString(id) || ids[id] {
			return false
		}
		ids[id] = true
		return true
	}
	pools := map[string]bool{}
	for _, p := range c.Pools {
		if err := p.Schedule.validate(); err != nil {
			return err
		}
		if !check(p.ID) || p.Budget < 1024 || p.Budget > 1<<40 || p.Name == "" {
			return fmt.Errorf("invalid pool")
		}
		pools[p.ID] = true
	}
	for _, cl := range c.Clients {
		u, e := url.Parse(cl.URL)
		if !check(cl.ID) || cl.Name == "" || e != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (cl.Kind != "qb" && cl.Kind != "tr") {
			return fmt.Errorf("invalid downloader URL or type")
		}
		if cl.Pool != "" && !pools[cl.Pool] {
			return fmt.Errorf("unknown bandwidth pool")
		}
	}
	var floors int64
	sites := map[string]bool{}
	for _, s := range c.Sites {
		if !check(s.ID) || s.Name == "" || s.Weight < 1 || s.Weight > 10 || s.Minimum < 0 || s.Maximum < 0 || s.DayTarget < 0 || s.MonthTarget < 0 || s.AfterTarget < 0 || s.Maximum > 1<<40 || s.Minimum > 1<<40 {
			return fmt.Errorf("invalid site bandwidth")
		}
		if (s.DayTarget > 0 || s.MonthTarget > 0) && s.AfterTarget < 1024 {
			return fmt.Errorf("target requires a nonzero low-speed allowance")
		}
		if len(s.Regex) > 512 {
			return fmt.Errorf("regex too long")
		}
		if s.Regex != "" {
			if _, e := regexp.Compile(s.Regex); e != nil {
				return fmt.Errorf("invalid RE2 regex")
			}
		}
		for _, d := range s.Domains {
			if d == "" || strings.ContainsAny(d, "/:?@ #") {
				return fmt.Errorf("use hostnames only")
			}
		}
		floors += s.Minimum
		sites[s.ID] = true
	}
	_ = floors
	for _, o := range c.Overrides {
		if o.Site != "" && !sites[o.Site] {
			return fmt.Errorf("unknown site override")
		}
		if o.Weight < 0 || o.Weight > 1000 {
			return fmt.Errorf("invalid override weight")
		}
	}
	return nil
}

type Allocation struct {
	ID                       string
	Weight, Minimum, Maximum int64
}

// Weighted water filling; byte budget is never exceeded, caps return unused quota.
func allocate(budget int64, items []Allocation) map[string]int64 {
	out := map[string]int64{}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	var floor int64
	for _, i := range items {
		floor += i.Minimum
	}
	if floor > budget {
		return out
	}
	for _, i := range items {
		out[i.ID] = i.Minimum
		budget -= i.Minimum
	}
	for budget > 0 {
		var weight int64
		for _, i := range items {
			if i.Maximum <= 0 || out[i.ID] < i.Maximum {
				weight += i.Weight
			}
		}
		if weight == 0 {
			break
		}
		initial := budget
		var used int64
		for _, i := range items {
			if i.Maximum > 0 && out[i.ID] >= i.Maximum {
				continue
			}
			n := int64(math.Floor(float64(initial) * float64(i.Weight) / float64(weight)))
			if n < 1 {
				n = 1
			}
			if i.Maximum > 0 && n > i.Maximum-out[i.ID] {
				n = i.Maximum - out[i.ID]
			}
			if n > budget-used {
				n = budget - used
			}
			out[i.ID] += n
			used += n
			if used == budget {
				break
			}
		}
		if used == 0 {
			break
		}
		budget -= used
	}
	return out
}
