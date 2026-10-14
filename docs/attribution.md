# Attribution and licenses

Yarrow is distributed under the MIT License (see [LICENSE](../LICENSE)).

## Based on Yarr

Yarrow is a fork of **Yarr** (yet another rss reader) by Nazar Kanaev — <https://github.com/nkanaev/yarr> — MIT License, © 2020 Nazar Kanaev. The full upstream history is preserved in this repository; the original README is kept at [yarr-upstream-readme.md](yarr-upstream-readme.md) and Yarr's documentation site sources remain in `doc/`.

## Included through Yarr

As listed in `etc/thirdparty.txt`:

- **readability** and **sanitizer** — adapted from Miniflux (<https://github.com/miniflux/v2>), Apache License 2.0; allow-lists from DOMPurify (<https://github.com/cure53/DOMPurify>).
- **fixconsole** — <https://github.com/apenwarr/fixconsole>, Apache License 2.0.
- Go dependencies in `vendor/` keep their own license files (go-sqlite3: MIT; golang.org/x: BSD-3-Clause; lib/pq: MIT; fyne systray: Apache 2.0; godbus: BSD-2-Clause; expr: MIT).

## Frontend

- **Vue** (<https://vuejs.org>) — MIT, bundled into `bundle.js` at build time.
- **Feather icons** (<https://feathericons.com>) — MIT; the SVGs in `src/frontend/svg/` (several added by Yarrow follow the same Feather drawings).

## Design inspiration

The reading experience is inspired by **feeeed** (<https://feeeed.nateparrott.com/>). Yarrow contains none of feeeed's code, branding, artwork or assets; its name, icon and visual design are original.

## Yarrow branding

The Yarrow icon (`etc/branding/*.svg`, PNG renders in `src/assets/static/icons/`) is original artwork made for this project and is covered by the project's MIT license.
