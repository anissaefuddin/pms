# Work Area Component Contract

Names follow the repository convention (kebab-case files in `web/src/components`, PascalCase exports). Each row says whether the component exists, and what this phase changed.

| Concept | Implementation | File | Status |
|---|---|---|---|
| WorkspaceLayout | Not extracted. Layout is `AppShell` in `routes/_app.tsx` and page-level `main` elements | `routes/_app.tsx`, `routes/_app/spaces.$spaceId.tsx`, `routes/_app/tasks.$itemId.tsx` | OPEN (FND-04). Planned `WorkspaceLayout` takes `header`, `summary?`, `nav`, `toolbar`, `children` |
| WorkspaceHeader | Not shared. `ProjectHeader` for project; inline header on Space | `components/project-header.tsx`, `spaces.$spaceId.tsx` | OPEN (FND-02). Planned props: `item` or `space`, `canCreate`, `canShare`, `onInfo`, `onShare`, `menu` |
| ProjectSummary | `ProjectHeader` KPI strip and details | `components/project-header.tsx` | CHANGED: compact strip with `Summary` toggle (`aria-expanded`, `aria-controls`) |
| ViewNavigation | `ViewTabs` | `components/view-tabs.tsx` | EXISTS, not changed |
| ViewToolbar | `ViewToolbar`, `ColumnPicker` | `components/view-toolbar.tsx`, `components/view-columns.tsx` | EXISTS, not changed |
| StatusIndicator | `StatusChip` (two copies) | `components/status-chip.tsx`, `components/ui/status-chip.tsx` | DUPLICATE: consolidate to `ui/` (OPEN, WA-07) |
| ItemTypeIcon | `TypeIcon`, `TypeBadge` | `components/type-icon.tsx`, `components/type-badge.tsx` | EXISTS |
| AssigneeAvatar / AvatarStack | `AvatarStack` (two copies) | `components/ui/avatar-stack.tsx`, `components/avatar-stack.tsx` | DUPLICATE (OPEN, WA-07) |
| PeoplePicker | `ItemUserPicker` | `components/item-user-picker.tsx` | EXISTS |
| DateRangePicker | `DatePicker` in `ui/date-picker.tsx`, `ItemDates` | `components/ui/date-picker.tsx`, `components/item-dates.tsx` | EXISTS |
| FilterPopover | Filters in `ViewToolbar` | `components/view-toolbar.tsx` | EXISTS as part of toolbar, not a shared popover |
| ContextMenu | `DropdownMenu` | `components/ui/dropdown-menu.tsx` | EXISTS |
| InlineItemCreator | Inline row creation inside `ItemTable` | `components/item-table.tsx` | EXISTS; contract not audited (FND-08 OPEN) |
| EmptyState | `EmptyState` | `components/ui/empty-state.tsx` | EXISTS. Heading level changed to `h2` in an earlier change |
| LoadingState | Inline `role="status"` text | per page | OPEN: no shared component |
| ErrorState | Inline `role="alert"` text and retry button | per page | OPEN: no shared component |
| ConfirmationDialog | `ConfirmDialog` | `components/confirm-dialog.tsx` | EXISTS |
| DetailPanelShell | Dialog with right-side classes in `_app.tsx`; `ItemDetail` | `routes/_app.tsx`, `components/item-detail.tsx` | EXISTS, not extracted |
| ToastNotification | `sonner` via `Toaster` | `components/ui/toaster.tsx` | EXISTS |
| KPI card | `KpiCard` | `components/ui/kpi-card.tsx` | EXISTS, not changed |

## Props contracts that already exist

- `ViewWorkspace({ spaceId, parent?, me?, canCreate, canEdit, onOpen, spaceEmpty? })`: `parent` selects the item children endpoint. Without it the Space root is used.
- `ProjectHeader({ item, children, canCreate, infoOpen, onToggleInfo, onShare?, onAddTask })`.
- `ItemDetail({ id, me, onOpen, onDeleted, onFullDetails? })`.
- `CreateItemDialog({ open, onOpenChange, me, location?, initialKind?, taskOnly?, onCreated })`, where `location` is `{ space_id, parent_item_id? }`.

## Rules for new shared components

1. Do not add a second component for a concept that already has one. Extend the existing one or move it to `components/ui` when it is generic.
2. Accept permissions as props computed by the caller from `my_capabilities`. Do not compute server rules in the component.
3. Colors come from tokens. Workflow status colors come from the workflow data.
4. Loading and error states use `role="status"` and `role="alert"`, and offer a retry button for failed requests.
