# Changelog

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
