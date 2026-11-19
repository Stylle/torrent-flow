# Changelog

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
