# Sidebar UX Closure Report (Phase 3.2)

Status: **PARTIAL. Not closed.** Acceptance criteria in the brief are not all met.

## Executive summary

Two visual defects were fixed: expanded disclosure toggles no longer show a light box in the dark sidebar, and the Tools block now collapses by default. The Spaces list uses a thin scrollbar. Targeted tests pass, and the mock suite passed with one test updated for the new Tools default. Most of the brief (SB-03, SB-06, SB-08, SB-09 to SB-14) is not done.

## Status per item (SB-01 to SB-15)

| ID | Status | Evidence |
|---|---|---|
| SB-01 | DONE | `sidebar-p32.spec.ts` |
| SB-02 | PARTIAL | Thin scrollbar set; visual not verified |
| SB-03 | OPEN | — |
| SB-04 | NOT CHANGED | Existing scroll area |
| SB-05 | DONE with DEC-SB-01 | `sidebar-p32.spec.ts` |
| SB-06 | OPEN | — |
| SB-07 | PARTIAL | Light and dark captured; density not captured |
| SB-08 | OPEN | — |
| SB-09 | NOT RUN | — |
| SB-10 | N/A (not found in source) | — |
| SB-11 | NOT RUN | — |
| SB-12 | NOT RUN | — |
| SB-13 | NOT RUN | — |
| SB-14 | NOT RUN | — |
| SB-15 | PARTIAL | Existing axe and keyboard tests |

## Before and after

Screenshots: `/tmp/pmsvis/sidebar-before/` and `/tmp/pmsvis/sidebar-after/` (local, not committed). Before: white boxes around expanded toggles in dark; Tools repeated under each expanded Space. After: toggles keep sidebar colours; Tools collapsed under the active Space.

## Five principles (heuristic, with evidence)

| Principle | Score | Basis |
|---|---|---|
| Minimalism | Not scored | Only one noise source removed (toggle box). Brief target 9/10 requires more work. |
| Compactness | Not scored | Tools collapse saves space; navigation density (SB-03) not changed. |
| Consistency | Not scored | Single toggle class; other components not unified (SB-06). |
| Scalability | Not scored | Deep hierarchy and large-data checks NOT RUN. |
| Discoverability | Not scored | Contextual actions and feedback not checked. |

No score is given because the brief requires visual evidence per principle and most items were not run.

## Decisions

- **DEC-SB-01:** Tools collapsed by default, including the active Space. APPROVED by Product Owner; recorded in `DECISIONS.md` section 8.

## Remaining issues

- SB-03, SB-06, SB-08, SB-09 to SB-14 not implemented or verified.
- Density, Stream, and dark-mode contrast of tree text not checked.
- Scrollbar appearance not verified visually.

## Regression risk

- Tools is collapsed by default. Any test or user flow that expects Dashboards or Reports visible without expanding Tools will fail. The Dashboards mock test was updated; no other test was changed.
- Visual behaviour of `aria-expanded` on non-sidebar buttons is unchanged.

## Recommendation

Not ready for closure. Complete SB-03, SB-06, SB-08, and the SB-T scenarios marked NOT RUN, then re-run the full suite and capture density screenshots.

## Files changed

- `web/src/components/sidebar.tsx`
- `web/src/index.css`
- `web/tests/sidebar-p32.spec.ts` (new)
- `web/tests/dashboards.spec.ts` (one step added: expand Tools)
- `docs/SIDEBAR_UX_AUDIT.md`, `docs/SIDEBAR_UX_REFINEMENT_PLAN.md`, `docs/SIDEBAR_UX_TEST_REPORT.md`, `docs/SIDEBAR_UX_CLOSURE_REPORT.md` (new)
- `docs/DECISIONS.md` section 8 (DEC-SB-01).
