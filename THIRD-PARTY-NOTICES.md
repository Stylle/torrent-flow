# Third-party notices

Torrent Flow is MIT licensed. Runtime and build dependencies retain their upstream licenses. License text copies for direct runtime dependencies and Go modules are included under `licenses/`; exact module/package versions are pinned in `go.sum` and `frontend/package-lock.json`.

- React / React DOM — MIT, Meta Platforms, Inc. and affiliates.
- Chart.js — MIT, Chart.js contributors.
- Font Awesome Free solid icons and React/core packages — upstream Fonticons notices apply (icons CC BY 4.0, fonts SIL OFL 1.1, code MIT as specified by their distributions). The project uses the SVG icon and React packages; license texts are included.
- Go x/crypto and indirect x/sys — BSD-style Go project license.
- modernc SQLite and related Go support modules — their respective BSD-style/public-domain upstream licenses, copied where provided.
- Vite and its React build plugin are build-time dependencies, retaining upstream notices in their distributions.

The repository used for feature research is credited in README.md; no source files or interface screenshots from that project are redistributed as this implementation's own code or assets.

Portable archive validation/snapshot code under internal/portablebackup is reused from [Stylle/today-random-movie-web](https://github.com/Stylle/today-random-movie-web) (MIT). Webhook JSON template escaping is adapted from [Stylle/wusaqi-gallery-go](https://github.com/Stylle/wusaqi-gallery-go) (MIT). Their author and MIT notices remain covered by LICENSE; the original projects retain their own notices.
