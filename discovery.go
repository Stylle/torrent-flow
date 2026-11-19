package main

import (
	"net"
	"regexp"
	"sort"
)

var trackerHostname = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]{0,251}[a-z0-9])?$`)

// Discover only tracker hostnames. Paths, credentials and passkeys never become rules.
// Existing manual domains/RE2 rules retain priority. A discovered site never enables control.
func discoverSites(c Config, torrents []Torrent) (Config, int) {
	c.Sites = append([]Site{}, c.Sites...)
	seen := map[string]bool{}
	rawByHost := map[string]string{}
	hosts := []string{}
	for _, torrent := range torrents {
		for _, raw := range torrent.Trackers {
			h := host(raw)
			if h == "" || seen[h] || (!trackerHostname.MatchString(h) && net.ParseIP(h) == nil) {
				continue
			}
			seen[h] = true
			id, ambiguous := siteFor(Torrent{Trackers: []string{raw}}, c)
			if id != "" || ambiguous {
				continue
			}
			hosts = append(hosts, h)
			rawByHost[h] = raw
		}
	}
	sort.Strings(hosts)
	added := 0
	for _, h := range hosts {
		if len(c.Sites) >= 256 {
			break
		}
		// A broader host discovered in this same batch may already cover this alias.
		id, ambiguous := siteFor(Torrent{Trackers: []string{rawByHost[h]}}, c)
		if id != "" || ambiguous {
			continue
		}
		c.Sites = append(c.Sites, Site{ID: randomID(), Name: h, Domains: []string{h}, Weight: 1, AfterTarget: 52429, Automatic: true})
		added++
	}
	return c, added
}
