# Sidebar UX Test Report (Phase 3.2)

Date: 16 Nov 2026. Local environment. Code state: working tree on `a972e78` plus the uncommitted sidebar changes.

## Automated

| Check | Result |
|---|---|
| Lint, token lint, typecheck | PASS |
| Unit tests | PASS, 85/85 |
| Production build | PASS |
| Mock Playwright (full) | First run: 505 passed, 1 failed (`dashboards.spec.ts`, Tools now collapsed by default). Test updated to expand Tools. Full rerun: 506 passed, 0 failed. |
| New sidebar tests (`sidebar-p32.spec.ts`) | PASS, 3/3: SB-T09 Tools collapse, SB-T07 expanded-toggle colour, SB-T14 thin scrollbar |
| Existing sidebar and layout tests (`uip4-sidebar`, `work-area-layout`, `spaces`) | PASS, 37/37 in the run before the Dashboards test update |

## Visual (browser captures, not committed)

| Viewport | Theme | Before | After |
|---|---|---|---|
| 1280×720, 1440×900, 1920×1080 | light and dark | Captured | Captured |

Captures show the expanded-toggle light box removed and Tools collapsed under the active Space. Density, compact mode, and the collapsed sidebar were NOT captured.

## Scenario status (SB-T01 to SB-T27)

| ID | Status | Notes |
|---|---|---|
| SB-T01 | PASS | Existing `uip4-sidebar` Expand/Collapse Space test |
| SB-T02 | NOT RUN | Nested project expand |
| SB-T03 | NOT RUN | Nested folder expand |
| SB-T04 | NOT RUN | Between Spaces |
| SB-T05 | NOT RUN | Between projects |
| SB-T06 | NOT RUN | Active route highlight not re-checked |
| SB-T07 | PASS | Expanded toggle keeps sidebar colour |
| SB-T08 | NOT RUN | Deep link expansion |
| SB-T09 | PASS | Tools collapse |
| SB-T10 | NOT RUN | Customize global navigation |
| SB-T11 | NOT RUN | Menu visibility after refresh |
| SB-T12 | NOT RUN | Sidebar collapse |
| SB-T13 | N/A | No resize control in source |
| SB-T14 | PASS (scrollbar style only) | Independence of scroll not separately tested |
| SB-T15 | NOT RUN | Long names with tooltip |
| SB-T16 | NOT RUN | Contextual actions by permission |
| SB-T17 | NOT RUN | Private Space visibility |
| SB-T18 | PARTIAL | Light and dark captured; no assertions |
| SB-T19 | NOT RUN | Density |
| SB-T20 | NOT RUN | Large hierarchy |
| SB-T21 | NOT RUN | Keyboard navigation (existing arrow-key test passes) |
| SB-T22 | NOT RUN | Loading, empty, error |
| SB-T23 | NOT RUN | Preference persistence |
| SB-T24 | NOT RUN | Sign-out and sign-in |
| SB-T25 | PASS | Work Area layout and project tests in the mock suite |
| SB-T26 | NOT RUN | Create Project and Folder from sidebar |
| SB-T27 | NOT RUN | Short viewport height |

## Bug fix: duplicate active highlight (user report)

- Symptom: on a Space page, both the Space name row and "Space overview" showed the selected background.
- Cause: the Space name link had `activeProps={activeLink}`, so it highlighted whenever its route was active, which is also true for the overview.
- Fix: the Space name link no longer receives the active class. Only "Space overview" is highlighted.
- Test: `sidebar-p32.spec.ts` "only Space overview is highlighted" (PASS).
- Known limitation: TanStack Router sets `aria-current="page"` on any active link, so the Space name link still announces `aria-current` in the accessibility tree. Removing it would require changing the element from a link to a button, which would break existing link-based tests. Not changed.
