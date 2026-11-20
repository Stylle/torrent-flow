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
	return discoverSitesWithCatalog(c, torrents, nil)
}
func discoverSitesWithCatalog(c Config, torrents []Torrent, catalog map[string]CatalogEntry) (Config, int) {
	c.Sites = append([]Site{}, c.Sites...)
	changed := 0
	for i, s := range c.Sites {
		if !s.Automatic || s.CatalogID != "" || s.Regex != "" || len(s.Domains) != 1 {
			continue
		}
		h := s.Domains[0]
		entry, ok := catalogLookup(catalog, h)
		if !ok {
			continue
		}
		// Existing IDs, custom names, policies and history remain stable.
		s.CatalogID = entry.ID
		if s.Name == h {
			s.Name = entry.Name
		}
		c.Sites[i] = s
		changed++
	}
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
	added := changed
	for _, h := range hosts {
		if len(c.Sites) >= 255 {
			break
		}
		// A broader host discovered in this same batch may already cover this alias.
		id, ambiguous := siteFor(Torrent{Trackers: []string{rawByHost[h]}}, c)
		if id != "" || ambiguous {
			continue
		}
		entry, known := catalogLookup(catalog, h)
		target := -1
		if known {
			for i, s := range c.Sites {
				if s.Automatic && s.CatalogID == entry.ID {
					if target >= 0 {
						target = -2
						break
					}
					target = i
				}
			}
		}
		if target >= 0 {
			s := c.Sites[target]
			s.Domains = append(append([]string{}, s.Domains...), h)
			c.Sites[target] = s
		} else {
			s := Site{ID: randomID(), Name: h, Domains: []string{h}, Weight: 10, Minimum: 50000, AfterTarget: 52429, Automatic: true}
			if known && target != -2 {
				s.Name = entry.Name
				s.CatalogID = entry.ID
			}
			c.Sites = append(c.Sites, s)
		}
		added++
	}
	needsFallback := false
	hasFallback := false
	for _, s := range c.Sites {
		if s.ID == "tracker-unavailable" {
			hasFallback = true
		}
	}
	for _, t := range torrents {
		id, amb := siteFor(t, c)
		if id == "" && !amb {
			needsFallback = true
		}
	}
	if needsFallback && !hasFallback && len(c.Sites) < 256 {
		c.Sites = append(c.Sites, Site{ID: "tracker-unavailable", Name: "Tracker 暂不可用", Weight: 10, Minimum: 50000, AfterTarget: 50000, Automatic: true})
		added++
	}
	return c, added
}
