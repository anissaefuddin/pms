# Sidebar UX Audit (Phase 3.2.0)

Scope: `web/src/components/sidebar.tsx`, `web/src/lib/sidebar-prefs.ts`, and the sidebar rules in `web/src/index.css`. Validated against source and browser captures (1280×720, 1440×900, 1920×1080; light and dark; Space expanded, project expanded). Baseline captures are local, not committed.

## Components found

| Concept | Implementation | Note |
|---|---|---|
| Space list | `SpacesNav`, `SpaceNode`, `ItemTree`, `Branch` | Reuses existing components; no rewrite |
| Disclosure toggle | `Button variant="ghost" size="icon-sm"` in `SpaceNode` and `Branch` | Two copies of the same class string (now one constant) |
| Global navigation | `Nav` rows in `SidebarBody` | Existing row component |
| Tools section | Inline in `SpaceNode` | Previously always open under each expanded Space |
| Preferences | `lib/sidebar-prefs.ts` (open Spaces, pins, hidden items) | Unchanged |

## Findings and validation

| ID | Finding | Validated? | Classification | Evidence |
|---|---|---|---|---|
| F-1 | Expanded disclosure toggles show a light box in dark sidebar | Yes | DEFECT (visual) | Baseline dark 1440: expanded Space and project chevrons show a white rounded background. Cause: ghost variant `aria-expanded:bg-muted aria-expanded:text-foreground`, which is light in dark theme. |
| F-2 | Tools repeats under every expanded Space and uses vertical space | Yes | UX IMPROVEMENT | Baseline: Tools block (Dashboards, Reports, Create analytics board) always open under the active Space. |
| F-3 | Scrollbar uses browser default (white on dark) | Partly | UX IMPROVEMENT | The Spaces nav has `overflow-y-auto`, so it scrolls. Headless capture does not show scrollbar chrome, so the visual result is not verified. Fix applied as a thin-scrollbar rule. |
| F-4 | Project and folder names truncate with ellipsis | Yes | OK | Baseline shows "[UXQA] Lalang Platfo…". A tooltip with the full name was NOT verified. |
| F-5 | Global navigation uses many rows | Partly | UX IMPROVEMENT | Nine rows (Search, Inbox, Request forms, Stream, Dashboards, More) at 1440×900. Row height was not measured against the 32–36 px target. |
| F-6 | Hierarchy lines are strong | Yes | UX IMPROVEMENT | `border-border` on nested containers; visible in the dark baseline. Not changed in this pass. |
| F-7 | Sidebar resize, nested depth 5–8, large hierarchy (SB-10, SB-11) | Not checked | NOT RUN | No resize control found in this pass; not investigated further. |
| F-8 | Customize menu and pin behaviour (SB-13) | Not checked | NOT RUN | Existing tests cover pins; behaviour not re-audited. |

## Decision points

- **DEC-SB-01 (Tools collapsed by default): APPROVED by Product Owner.** Tools starts collapsed for every Space, including the active one, and opens automatically on `/dashboards` or `/reports`. Recorded in `DECISIONS.md` section 8.
