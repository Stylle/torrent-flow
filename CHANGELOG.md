# Changelog

## 0.1.8 · 2026-11-20

- Add bundled/daily-updated PT-depiler site recognition with manual-rule priority and retained history.
- Reorganize navigation into five primary entries and contextual sections.
- Add independent downloader enable/disable with durable owned-limit recovery.
- Unify decimal units across inputs, charts, notifications and historical log display without changing stored byte values.
- Add a bounded, paginated downloading list with qBittorrent/Transmission progress on Overview.


## 0.1.6

- Place Webhook and backup above settings, unify Chinese menu labels, and add an About page explaining allocation, history and recovery.
- Replace the two language choices with one language button.
- Record per-site upload/download rate history for24h; offer5/15/30m and1/3/6/12/24h windows with bounded aggregation and caching.
- Draw separate site lines and site-colored stacked traffic bars with byte totals and percentage tooltips; colors agree across charts and site tables.
- Compact torrent rows into separate name/rate/limit/state/downloader/site/connection/Tracker columns; drag or keyboard-resize columns, keeping widths only on the device.
- Add active upload/allowance summaries and remove duplicate per-torrent ownership from the outlet page.
- Include per-site history in backups while still accepting0.1.5 archives without that table.


## 0.1.5

- Independently start/stop automatic allocation for each downloader while keeping monitoring active; stopping clears torrent-level limits and persists failed clearance.
- Share outlet budgets across downloaders, provide a default 50 Mbps outlet, use site weights 1–10 (default 10), and preserve fixed per-torrent minimums even above budget with warnings.
- Apply proportional total site shares before floor adjustments and split equally inside a site.
- Show downloader/Tracker identity and active pending/failed torrents; simplify overview and display assigned outlets directly.
- Add API pagination/search, bounded tracker metadata queries, unchanged-limit fast paths and serialized browser polling.
- Move explicit day/week/month/year selection into statistics; preserve one-year retention semantics.
- Add portable full backups using the Movie project archive validator, atomic logical restores, credential re-encryption, rollback archives and session revocation.
- Add Gallery-style Webhook draft tests/templates and persisted daily/weekly/monthly reports, cooldown alerts, bounded retries and delivery history.


## 0.1.4

- Discover Tracker hostnames automatically, preserve renamed sites and existing manual rules, and keep passkeys out of site settings and UI.
- Show only actively controlled torrents with upload connections in the overview; display Tracker hostnames and retain full configuration lists.
- Add explicit downloader-wide automatic active-torrent control with individual exclusions and unchanged recovery ownership.
- Provide recommended defaults and explain weights, minimums, caps, targets, sampling and idle release in settings.
- Preselect a sole bandwidth pool, associate unassigned downloaders when creating the first pool, and keep the editable example budget disabled until confirmed.

## 0.1.3

- Keep form dialogs open when their backdrop is clicked; preserve explicit close/cancel and keyboard dismissal.
- Fix touch navigation scrim inheriting button hover/active styles; lock background scrolling and clear drawer state when resizing to desktop.
- Normalize common Transmission base/Web URLs to RPC, preserve POST/session negotiation through bounded same-origin redirects, and reject cross-origin credential forwarding.
- Return to independent guardian deployment by default, prioritizing recovery while the main container is stopped, following the user's revised requirement.
- Add a concrete 1 MiB/s combined qBittorrent/Transmission budget guide with site weights, floors and unit explanations.

## 0.1.2

- Fix creation of downloaders, sites and bandwidth pools on ordinary LAN HTTP origins by using a cryptographically secure UUID fallback.
- Center the work area within the space beside the sidebar on 4K displays, with responsive width and chart sizing.
- Refresh light/dark green-gray surfaces and solid icons using Uptime Kuma as the user's visual reference.

## 0.1.1

- Default single-container deployment with embedded recovery; external guardian becomes optional.
- Explain whole-container outage recovery boundaries in the UI and deployment guide.
- Compact new hourly torrent traffic deltas without changing totals or deleting historical data.
- Sample speed history at most every 15 seconds and throttle expiry work to hourly.


## 0.1.0

- Multi-instance qBittorrent / Transmission monitoring with cumulative counter deltas.
- Site aliases and RE2 tracker rules; explicit per-torrent ownership for multi-site matches.
- Day/week/month/year reports, one-year traffic retention and separate live speed history.
- Responsive light/dark/system dashboard, Chinese/English device preferences.
- Opt-in torrent upload allocation with weighted site budgets, idle release and target allowances.
- Independent recovery guardian, durable command intent and readback, external-change handling.
- Docker deployment with an encrypted local SQLite configuration and administrator authentication.
