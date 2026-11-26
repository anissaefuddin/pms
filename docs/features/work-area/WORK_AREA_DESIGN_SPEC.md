# Work Area Design Spec

Scope: foundation only (FND-05). This file says what the work area should look like and which tokens already exist. It does not replace the approved mockup. Where a value is not measured yet it is marked NOT CHECKED.

## 1. Principles kept from the current product

- LENS branding: navy sidebar (`sidebar-navy`, `--nav*` tokens), blue primary (`--brand-primary` `#2563EB`), light and dark themes, density preference (`data-density`).
- Work surface is neutral (`--background` `#F9FAFB` in light). Cards use `--card`.
- No new global header. The shell stays: sidebar on the left and the topbar that already exists.

## 2. Token inventory (from `web/src/index.css`)

| Role | Token(s) | Light value |
|---|---|---|
| Background | `--background` | `#F9FAFB` |
| Surface | `--card`, `--popover` | `#FFFFFF` |
| Border | `--border`, `--border-subtle`, `--input` | `#E5E7EB`, `#F3F4F6`, `#E5E7EB` |
| Text primary | `--foreground` | `#111827` |
| Text secondary | `--muted-foreground` | `#6B7280` |
| Hover and selected | `--muted`, `--accent` | `#F3F4F6` |
| Focus ring | `--ring` | `--brand-primary` |
| Primary | `--primary`, `--brand-primary` | `#2563EB` |
| Success, warning, danger, info | `--success`, `--warning`, `--destructive`, `--info` (+ `-subtle`) | `#16A34A`, `#D97706`, `#DC2626`, `#2563EB` |
| Categories for charts and tags | `--cat-1` … `--cat-6` (+ `-subtle`) | violet, teal, rose, orange, cyan, fuchsia |
| Sidebar | `--nav`, `--nav-text`, `--nav-muted`, `--nav-hover`, `--nav-active` | navy set (`sidebar-navy`) |
| Radius | `--radius-md` 6px (control), `--radius-xl` 8px (card) | |

Workflow status colors are not tokens. They come from each workflow (`color` on each status) and are mapped by `lib/status-colors.ts`. Keep that rule.

Overdue and blocked states: not separate tokens yet. Use `--destructive` for overdue and a text label. Decision recorded in `WORK_AREA_IMPLEMENTATION_PLAN.md`.

## 3. Type and spacing

| Use | Recommended | Status |
|---|---|---|
| Font | Inter | MEASURED: `Inter Variable`, Inter, system-ui |
| Body | 13–14px | MEASURED: 13px, line-height 19.5px |
| Metadata | 11–12px | MEASURED: 13px for the header meta line (above the spec; not changed) |
| Workspace title | 20–22px (`.workspace-title` class exists) | MEASURED: 20px, weight 600 |
| Section title | 14–16px | NOT CHECKED (no section title on the Space page to measure) |
| Icon | 16–18px | NOT CHECKED |
| Spacing grid | multiples of 4px (Tailwind default) | in use |
| Compact table row | 32px | NOT CHECKED (table needs items in the mock) |
| Comfortable table row | 40px | NOT CHECKED |
| Toolbar height | 36–44px | NOT CHECKED |

## 4. Layout rules (FND-04)

- Shell: sidebar `h-dvh` and sticky. The document must not scroll sideways (test `work-area-layout.spec.ts`).
- Workspace content takes the remaining viewport width and height.
- Views that are wide (Board, Gantt, Table with many columns) scroll inside their own container. The page does not.
- Task detail is a right-side Dialog with `overflow-y-auto` and does not move the page.
- Page padding: one value for all work pages (gap WA-10). Proposed: `p-4 sm:p-6`. Decision: adopt in `WorkspaceLayout`.

## 5. Dark theme and density

- Dark theme overrides are in `index.css` under `:root[data-theme="dark"]` and the media query. Not every view was checked in dark (gap WA-14).
- Density is `data-density="compact" | "comfortable"` on `<html>`. Row heights above are for the two values.

## 6. What this spec does not decide

- Exact typography sizes (needs measurement against the mockup).
- Overdue and blocked colors as tokens (needs a decision, see DECISION list in the final report).
