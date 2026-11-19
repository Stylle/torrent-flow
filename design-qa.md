# Design QA

final result: passed

Selected target: the first warm-white/teal design direction, subsequently revised in the chat to add theme/language controls and active-torrent allocation/recovery. This is a responsive web application, not a screenshot replica or mobile device-frame demo.

Evidence: local isolated browser captures at 1440×1024, 834×1194 and 390×844; light/dark and English variants; signed-in empty and synthetic connected states. Operational captures and comparison sheets live outside the repository in `runtime/qa-torrent-flow/`; only explicitly marked mock-data screenshots are copied to `docs/screenshots/`.

The generated target is a presentation board with a desktop and mobile surface. Desktop target was cropped to its application area and compared next to the 1440px browser application; exact raster/pixel equality is not claimed across differently sized frames. Both source and rendered evidence were opened and compared.

## Iteration 1

- P2: period/date controls displaced below the header, making the desktop dashboard unnecessarily tall. Moved overview controls into the desktop header and retained the mobile filter row.
- P2: metric numerals inherited upload/download colors, unlike the target's neutral primary values. Changed metric values to the primary text token while preserving colored status icons.
- P2: base surface was cooler than the selected warm-white direction. Adjusted light-theme surface tokens and condensed desktop card spacing.

## Iteration 2

Recaptured light desktop/tablet/phone, dark phone and English dark desktop, then opened a new side-by-side comparison with the selected target. No remaining P0/P1/P2 layout issues observed. Compact lists, chart legends, sidebar drawer, theme/language controls and moderately rounded surfaces follow the selected direction. Additional real-product states (login, empty data, pending recovery, sample-data banner) are intentional and are not part of the generated example's data.

## Interaction checks

- All five sections reachable on desktop and through the tablet/phone drawer.
- No document-level horizontal overflow at tested sizes; large detail tables may scroll within their own containers.
- Theme and English preference survive reload.
- Isolated real-backend login, draft connection test, configuration revisions, site matching and opt-in allocation exercised.
- Idle release and forced-controller-stop recovery tested against a synthetic downloader; no real downloads were accessed.
- No browser runtime errors in these tests. Modal supports focus containment and Escape dismissal.

P3 follow-up: split the chart bundle if substantially slower devices show startup delay; current production JavaScript is approximately 505 kB before compression. Generated charts and icon assets are implemented as Chart.js data plots and the Phosphor library rather than rasterized UI.

## 0.1.1 follow-up

Default deployment is now one container with embedded recovery; independent recovery is optional. The policy screen explains the whole-container downtime boundary in Chinese and English. Repeated desktop/tablet/phone route, overflow, theme/language and isolated integration checks passed. The subprocess fixture explicitly confirmed that recovery does not run while the entire single process is stopped, then confirmed restart reconciliation releases the limit. Historical-row preservation and a 1000-torrent/24-delta-poll storage test preserve byte totals while limiting new rows to one per torrent/site/hour/uncertainty class.
