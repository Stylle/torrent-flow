# Contributor instructions

Read README.md, SECURITY.md and docs/ARCHITECTURE.md before changes. Preserve downloader authentication, per-torrent identity, cumulative accounting, credential encryption and durable recovery ownership contracts.

Never operate on a real downloader in tests. Use isolated httptest/Playwright fixtures. Do not commit actual credentials, passkeys, data volumes, SQLite files, encryption keys, private screenshots or logs.

Control is opt-in and requires a healthy recovery worker. Default deployment is one container with an embedded worker; independent recovery is an optional deployment mode. Never claim recovery continues while the whole single container is stopped. Persist intended limits before outbound writes, serialize recovery/control writes, verify readback before success, and never discard a failed recovery entry. Changes to user limits outside this controller suspend ownership persistently. A manual change must not be silently undone on restart.

Frontend follows the selected warm-white/teal design, compact device layouts, system/dark/light theme and Chinese/English preferences. Browser verification requires screenshots at desktop/tablet/mobile and interaction checks; a build is not visual QA. Keep all test fixtures and operational helpers outside the public repository unless they are self-contained synthetic unit/integration tests.

Run frontend production build, Go tests/vet, Linux race tests and isolated fault injection before publishing. Retain pinned dependency locks, cross-architecture Docker builds and versioned tags. Do not push to another existing project's Docker repository.
