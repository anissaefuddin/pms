# Sidebar UX Refinement Plan (Phase 3.2)

Status at this pass: **PARTIAL**. Foundation audit done. Two visual defects fixed, Tools made collapsible, scrollbar styled. Multi-agent execution was NOT used: work was done sequentially by one operator.

## Ownership and execution

| Area | Owner | Files |
|---|---|---|
| Visual foundation (SB-01, SB-02, SB-03, SB-06, SB-07, SB-08) | Operator (sequential) | `components/sidebar.tsx`, `index.css` |
| Navigation architecture (SB-04, SB-05, SB-09, SB-10, SB-11) | Operator (sequential) | `components/sidebar.tsx` |
| Interaction and discoverability (SB-12 to SB-15) | Not executed | — |
| QA and visual regression (Agent D) | Operator, mock and screenshot checks only | `tests/sidebar-p32.spec.ts`, `tests/dashboards.spec.ts` |

## Items

| ID | Item | Implementation | Verification |
|---|---|---|---|
| SB-01 | Remove light box on expanded toggles | DONE (`treeToggle` constant with `aria-expanded:bg-transparent`) | PASS (`sidebar-p32.spec.ts`, computed background is transparent) |
| SB-02 | Thin scrollbar | PARTIAL (`.sidebar-scroll` with `scrollbar-width: thin`) | PASS (computed `scrollbarWidth` is `thin`); visual scrollbar not verified in headless |
| SB-03 | Navigation density | NOT DONE | NOT RUN |
| SB-04 | Independent scroll areas | NOT CHANGED (existing `overflow-y-auto` on Spaces list) | Covered by existing layout test |
| SB-05 | Collapsible Tools | DONE, with DEC-SB-01 | PASS (`sidebar-p32.spec.ts`, `dashboards.spec.ts` updated to expand Tools) |
| SB-06 | Unify navigation components | NOT DONE | NOT RUN |
| SB-07 | Theme and density | Light and dark captured; density NOT RUN | PARTIAL |
| SB-08 | Visual hierarchy | NOT DONE | NOT RUN |
| SB-09 | Deep hierarchy | NOT CHANGED | NOT RUN at depth 5–8 |
| SB-10 | Resizable sidebar | NOT FOUND in source; not implemented | NOT RUN |
| SB-11 | Large hierarchy performance | NOT RUN | NOT RUN |
| SB-12 | Contextual actions | NOT CHANGED | NOT RUN |
| SB-13 | Menu customization | NOT CHANGED | NOT RUN |
| SB-14 | Navigation feedback and tooltips | NOT CHANGED | NOT RUN |
| SB-15 | Accessibility | Tools toggle has `aria-expanded`; toggle keyboard behaviour unchanged | Existing axe test PASS |
