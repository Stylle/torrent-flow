# Security

This application is designed for one trusted deployment administrator. No public registration or multi-tenant untrusted settings are supported. Downloader credentials, SQLite files, the master encryption key and initial administrator password are private data and must not be committed or published.

Only authenticated administrators may configure request destinations. LAN targets are intentionally supported. Redirects are disabled except bounded Transmission RPC same-origin redirects or same-host HTTPS upgrades; credentials must never follow a redirect to another host or arbitrary port. Rate-limit operations must only affect torrent entries opted in individually or through an explicit downloader-wide setting (individual exclusions take precedence), with persisted intent and readback confirmation. A changed external limit must not be silently overwritten. Recovery must retain pending entries when a downloader cannot be reached.

Report suspected vulnerabilities through a private GitHub security advisory. Do not include real credentials or tracker passkeys in public issues.
