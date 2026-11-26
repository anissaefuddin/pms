# Work Area Interaction Standards (FND-07)

These are the rules every work-area page follows. Each rule says whether the current code already does it. "Partial" means some screens do it and others do not. Rules are only marked as implemented when a test or the shared component proves it.

| Rule | Standard | Status | Evidence |
|---|---|---|---|
| Single click | Opens a row, card, or bar in the detail panel, or follows a link. Selects a view tab. | Implemented for rows (`openItem`) and view tabs | `spaces.$spaceId.tsx`, `view-tabs.tsx` |
| Double click | Only where the existing view defines it (Board title edit is not defined). Not added in this phase. | Not applicable yet | - |
| Inline editing | Edits in place on hover or focus, saves on blur or Enter, cancels with Escape. | Partial: table cells edit inline; cancel on Escape not audited | `item-table.tsx` |
| Opening details | Tasks open in the right-side panel (`?task=`). Projects and folders open as pages. | Implemented | `spaces.$spaceId.tsx` `openItem`, `tasks.$itemId.tsx` `openChild` |
| Context menu | `DropdownMenu` for item and view actions. Keyboard reachable through the menu trigger. | Partial: exists per screen, not one shared item menu | `ui/dropdown-menu.tsx` |
| Keyboard navigation | Dialog focus trap and Escape close (base-ui Dialog). Table arrow keys not audited. | Partial | `ui/dialog.tsx` |
| Escape cancels | Closes dialogs and popovers. Inline edits: not audited. | Partial | base-ui behaviour |
| Enter confirms inline creation | Create dialog submits with the "Create item" button. Enter key submit in the title field not verified. | Not verified | - |
| Loading feedback | `LoadingState` (`role="status"`) for region loads. | Implemented for Space items; other pages still inline text | `ui/loading-state.tsx` |
| Validation | Field errors shown next to the field (`aria-invalid`, `FieldErrors`). Server 422 fields mapped back. | Implemented in create dialog | `create-item-dialog.tsx` lines around 158–160 |
| Optimistic updates | Only where the rollback is tested. Board drag has rollback (existing test). | Partial | `views.spec.ts` "Board rolls back conflict" |
| Failed mutation recovery | Toast for failed action, `ErrorState` with retry for failed load. Retry does not repeat successful work. | Implemented for loads (`ErrorState`), partial for mutations | `ui/error-state.tsx`, `views.spec.ts` |
| Unsaved changes | Not implemented: closing a dialog with changes discards them without warning. | Gap | - (recorded as FEATURE GAP) |
| Permission-restricted actions | Hidden when `my_capabilities` lacks the capability; server still enforces. | Implemented | `spaces.$spaceId.tsx`, `tasks.$itemId.tsx` |

## Rules for new code

1. Use `LoadingState` and `ErrorState` for region loads, not inline text.
2. A failed mutation shows a toast and leaves the previous value on screen. A retry button is shown only for failed loads.
3. Do not add optimistic updates without a rollback test.
4. Disabled controls explain why in text, not only by colour.
5. Keyboard: every action reachable by a menu or button must also work from the keyboard.
