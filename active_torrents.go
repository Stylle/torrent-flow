package main

import "sort"

type activeTorrent struct {
	Torrent
	Uploading      bool `json:"uploading"`
	AwaitingUpload bool `json:"awaitingUpload,omitempty"`
}

type activeTorrentSummary struct {
	Count       int   `json:"count"`
	Downloading int   `json:"downloading"`
	Uploading   int   `json:"uploading"`
	Up          int64 `json:"up"`
	Down        int64 `json:"down"`
}

func activeTorrents(torrents []Torrent, c Config, filter string) ([]activeTorrent, activeTorrentSummary) {
	rows := []activeTorrent{}
	summary := activeTorrentSummary{}
	strategies := make(map[string]bool, len(c.Clients))
	for _, client := range c.Clients {
		strategies[client.ID] = client.Enabled && clientPolicyOn(client) && policyOn(c)
	}
	for _, torrent := range torrents {
		torrent.Downloading = torrent.Downloading || torrent.Down > 0
		uploading := torrent.Up > 0 || torrent.UploadObserved || torrent.UploadQualified ||
			(c.UploadConfirmSeconds == 0 && torrent.Connections > 0)
		if !torrent.Downloading && !uploading && torrent.State != "release pending" {
			continue
		}
		summary.Count++
		summary.Up += torrent.Up
		summary.Down += torrent.Down
		if torrent.Downloading {
			summary.Downloading++
		}
		if uploading {
			summary.Uploading++
		}
		if (filter == "downloading" && !torrent.Downloading) || (filter == "uploading" && !uploading) {
			continue
		}
		rows = append(rows, activeTorrent{
			Torrent: torrent, Uploading: uploading,
			AwaitingUpload: uploading && c.UploadConfirmSeconds > 0 && !torrent.UploadQualified &&
				strategies[torrent.Client] && torrent.State == "monitoring",
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Down == rows[j].Down && rows[i].Up == rows[j].Up {
			return rows[i].Key < rows[j].Key
		}
		if filter == "uploading" || rows[i].Down == rows[j].Down {
			if rows[i].Up != rows[j].Up {
				return rows[i].Up > rows[j].Up
			}
		}
		return rows[i].Down > rows[j].Down
	})
	return rows, summary
}
