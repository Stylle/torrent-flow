# Security

This application is designed for one trusted deployment administrator. No public registration or multi-tenant untrusted settings are supported. Downloader credentials, SQLite files, the master encryption key and initial administrator password are private data and must not be committed or published.

Only authenticated administrators may configure request destinations. LAN targets are intentionally supported. Redirects are disabled except bounded Transmission RPC same-origin redirects or same-host HTTPS upgrades; credentials must never follow a redirect to another host or arbitrary port. Rate-limit operations must only affect configured downloaders whose strategy is enabled, with persisted intent and readback confirmation. A changed external limit must not be silently overwritten. Recovery must retain pending entries when a downloader cannot be reached.

Report suspected vulnerabilities through a private GitHub security advisory. Do not include real credentials or tracker passkeys in public issues.

An explicit downloader strategy stop clears all of that downloader's per-torrent upload limits, including manual/unowned values; it must not affect other downloaders or global upload limits. Imports require stopped strategies with cleared recovery work, validate before mutation, revoke sessions and pause imported allocation. Backup archives contain decryption material and are private. Webhook headers must never be returned or logged; redirects must not forward secrets, and report retries must not be described as exactly once.
