# Contributor instructions

Read README.md, SECURITY.md and docs/ARCHITECTURE.md before changes. Preserve downloader authentication, per-torrent identity, cumulative accounting, credential encryption and durable recovery ownership contracts.

Never operate on a real downloader in tests. Use isolated httptest/Playwright fixtures. Do not commit actual credentials, passkeys, data volumes, SQLite files, encryption keys, private screenshots or logs.

Control is opt-in and requires a healthy recovery worker. Per the user's latest decision, default Docker deployment uses a main container plus an independent recovery guardian so limits can be released while the main container is stopped. Embedded recovery remains available for standalone development but must never be described as surviving the whole process/container being stopped. Persist intended limits before outbound writes, serialize recovery/control writes, verify readback before success, and never discard a failed recovery entry. Changes to user limits outside this controller suspend ownership persistently. A manual change must not be silently undone on restart.

Frontend now follows the user's Uptime Kuma reference for green/gray theme tokens and Font Awesome solid icons, keeping compact device layouts, system/dark/light theme and Chinese/English preferences. Do not copy the Kuma mascot or claim affiliation. Main content must remain centered in the usable area beside the sidebar on 4K displays. Creation must work over ordinary NAS HTTP, not just localhost/HTTPS. Browser verification requires screenshots at desktop/tablet/mobile and interaction checks; a build is not visual QA. Keep all test fixtures and operational helpers outside the public repository unless they are self-contained synthetic unit/integration tests.

Run frontend production build, Go tests/vet, Linux race tests and isolated fault injection before publishing. Retain pinned dependency locks, cross-architecture Docker builds and versioned tags. Do not push to another existing project's Docker repository.
