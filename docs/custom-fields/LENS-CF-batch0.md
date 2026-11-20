# LENS Custom Fields: Batch 0 (contract and decisions)

Status: working document. Written against the working tree on branch `feat/m12c-ui-clean` after commit `d2e8876` (Workspace custom fields, migration 00328).

## 0. Correction to the earlier audit

The earlier audit said "custom_fields.space_id is NOT NULL, so no Workspace scope". That was out of date. Commit `d2e8876` and migration `00328_workspace_custom_fields.sql` already make `space_id` nullable. A Workspace field has `space_id NULL`. Admins manage them through `/api/v1/custom-fields`, and a Workspace item type can attach only Workspace fields. Item values for Workspace fields are settable in any Space the item sits in. The Settings dialog for Workspace fields is opened from the item types page.

## 1. Traceability matrix

Classes: EXISTING, PARTIAL, MISSING, DEFECT, DECISION REQUIRED. "Now" means the state after this batch.

| ID | Requirement | Class | Evidence (file) | Now |
|---|---|---|---|---|
| R-01 | Workspace-level and Space-level field definitions (CF-D01) | EXISTING | `00328`, `items/custom_fields.go` (`fieldSpace`), `httpserver/custom_fields.go` | Done before this batch |
| R-02 | Existing fields stay Space-level, IDs and values kept | EXISTING | `00328` only drops NOT NULL; no data moved | Verified by reading the migration |
| R-03 | Workspace fields reused through assignments, not copies | PARTIAL | Reuse exists only through `item_type_fields` (`type_fields.go`). No Space, Project, or Folder assignment | Open (Batch 4) |
| R-04 | Space admins manage Space fields; admin manages Workspace fields | EXISTING | `fieldSpace` requires the Full role (`RoleCaps(Full)`) | Open question on "authorised Space admin" (DR-06) |
| R-05 | Prevent enumeration of restricted-Space fields | PARTIAL | Space list is scoped by `SpaceAccessFor`; Workspace list is admin-only; `read.go` shows Workspace values only where the item sits | Field-level part open (Batch 2) |
| R-06 | Field-level view and edit permissions (CF-D03) | MISSING | No table or check for field access in `read.go`, `updateCustomFieldValues`, `list_fields.go`, `analytics.go`, `projectexcel/export.go` | Open (Batch 2) |
| R-07 | Separate: manage definition, assign, read value, write value, view-visibility | PARTIAL | Manage and write follow item and Space caps. View visibility exists per view. No separate field-level capability | Open (Batch 2) |
| R-08 | Restricted fields fail closed in every read and write path | MISSING | See R-06 | Open (Batch 2) |
| R-09 | Archive preserved (CF-D04) | EXISTING | `ArchiveCustomField` sets `archived_at` | Kept |
| R-10 | Restore archived fields | EXISTING (added) | `items/custom_field_lifecycle.go` `RestoreCustomField`; route `POST .../restore` | Done in this batch |
| R-11 | Dependency impact preview before archive | PARTIAL (added) | `CustomFieldImpactOf`; route `GET .../impact`; confirmation UI in `custom-fields-dialog.tsx` | Counts use JSON text match (see §4) |
| R-12 | Impact covers items, item types, workflows and guards, forms, automation, blueprints, formulas, views, dashboards, relations | PARTIAL (added) | `custom_field_lifecycle.go` queries | Covered for all listed; formulas only within the Space |
| R-13 | Permanent deletion | OUT OF SCOPE (CF-D05) | none | Not built |
| R-14 | 12 field types preserved (CF-D06) | EXISTING | `custom_field_types.go`; dialog `fieldTypes` | Kept |
| R-15 | Formula and Relation reused (CF-D07) | EXISTING | `formula.go`, `item_relations` | Not reclassified as missing |
| R-16 | Type cannot change after creation (CF-D10) | EXISTING | `UpdateCustomField` does not accept `type` | Kept. Confirm with a test in Batch 6 |
| R-17 | Search, filter, sort, group, visibility | PARTIAL | `view-toolbar.tsx` (one field filter, no people), `view-columns.ts` (no multi-select or people sort), `item-table.tsx` (group only by status or type) | Open (Batch 5) |
| R-18 | Settings > Work configuration > Custom fields page, not the Personal Space | DEFECT | `settings-sidebar.tsx`: `inPersonalSpace("/spaces/$spaceId", ...)` sends admins to the Personal Space | Open (Batch 3) |
| R-19 | Registry table with scope tabs, archived view, search, sort, columns | MISSING | Only a dialog without search (`custom-fields-dialog.tsx`) | Open (Batch 3) |
| R-20 | Right-drawer editor with the five sections | MISSING | Editor is an inline form in a modal | Open (Batch 3) |
| R-21 | Assignment at Space, Project, Folder, Item Type with entry points | MISSING except Item Type (PARTIAL) | Item type checklist in `settings/item-types.tsx` | Open (Batch 4) |
| R-22 | Assignment removal keeps stored values | PARTIAL | `DeleteTypeField` removes only the link; values stay in `items.fields` | Verify with a test (Batch 4) |
| R-23 | Type default values via `item_type_fields` | EXISTING | `SetTypeField`, `typeFieldDefaults` | Kept |
| R-24 | Duplicate field definitions | MISSING | none | P2 (Batch 7) |
| R-25 | Usage statistics | MISSING | none | P2 (Batch 7) |
| R-26 | Bulk assignment and bulk archive | MISSING | none | P2 (Batch 7) |
| R-27 | Dashboards and analytics respect field permissions | MISSING | `dashboards_public.go:143` and `analytics.go:724` read field type and values without field access | Open (Batch 2). Verify before release |
| R-28 | Excel export respects field permissions | MISSING | `projectexcel/export.go:130` exports every field value of each item | Open (Batch 2). Verify before release |
| R-29 | Request forms and blueprints respect field permissions | MISSING | `form_submit.go` maps by field id; no access check of the field | Open (Batch 2) |
| R-30 | Status guards on archived fields | UNKNOWN | `guards.go` requires a field by id; behaviour after archive not tested | Test in Batch 6 (NOT RUN) |
| R-31 | Automation with archived fields | UNKNOWN | `automation` references `fields.<uuid>`; archive behaviour not tested | Test in Batch 6 (NOT RUN) |
| R-32 | Table, Board, Item Detail stay synchronised | EXISTING | All three read `item.fields` | Kept |
| R-33 | Multi-location items and inheritance | DECISION REQUIRED | `item_locations` supports several Spaces | DR-07 |

## 2. Items to verify before release (possible defects)

These are facts from reading code, not yet proven by a test. Each one is a candidate for Batch 2.

1. **Excel export** (`projectexcel/export.go:130`) writes all custom field values of an item without any field-access check. With field permissions this becomes a leak.
2. **Public dashboards** (`httpserver/dashboards_public.go:143`) read a field's type by id. Check whether any public aggregate can reveal a restricted value.
3. **Analytics** (`analytics/analytics.go:146`, `:724`) aggregate by field value without field access.
4. **Settings entry point** (`settings-sidebar.tsx`) sends administrators to their Personal Space for custom fields, which the spec forbids (R-18).

## 3. Decisions (DECISION REQUIRED), with recommendations

Each recommendation is the default the implementation will follow unless you say otherwise.

| ID | Question | Recommendation |
|---|---|---|
| DR-01 | Default for a field with no restriction | Inherit item access. Restricted only when a row in the new access table exists |
| DR-02 | Admin access to a restricted value | Admin manages the definition and sees the field list. Admin reads or writes a value only if the admin holds the item's access and the field's grant, as any member does. Documented in the matrix |
| DR-03 | Automation principal | Runs as the rule owner's effective access at trigger time, fails closed if the owner lacks the field grant |
| DR-04 | Public links and API keys | Restricted fields never appear in public payloads. API keys follow the key owner's grants |
| DR-05 | Formula over a restricted input | The formula value is hidden from a viewer who cannot read every input. The formula itself stays visible |
| DR-06 | "Authorised Space admin" | Space Full role, as now. Unchanged |
| DR-07 | Multi-location items | A field applies to an item if any location's Space enables it. Values are stored once per item |
| DR-08 | Workspace field availability in a Space | Opt-in per Space through an assignment. Until assigned, the field's values are hidden in that Space |
| DR-09 | Archive with dependencies | Not blocked, except a formula that reads the field (current rule). Impact is shown first. Guards, forms and automations keep the archived id; their behaviour is tested in Batch 6 |
| DR-10 | Restore name clash | Refuse with 422; the user renames first (implemented) |
| DR-11 | Usage statistics and bulk actions | P2, after Batches 2 to 6 |
| DR-12 | Access table shape | New table `custom_field_grants(field_id, subject_kind, subject_id, level)` with `level` in `view`, `edit`, `none`. Reuse groups and user ids from existing tables. No job-title rules |

DR-08 and DR-07 change behaviour for existing data. They need a reviewed migration plan before implementation (see §5).

## 4. Known limits of this batch

- **Impact by text match.** `CustomFieldImpactOf` counts JSON columns that contain the field UUID. It cannot miss a stored reference, but it may count an unrelated occurrence. It only over-warns. Upgrade to typed reference tables when the JSON shapes stop changing.
- **Formulas.** The impact lists formulas in the same Space only. A Workspace field cannot be read by a formula, so this is correct for today's rules.
- **Views.** A view with no scope (`scope_space_id NULL`) is counted only for a Workspace field.

## 5. Migration and rollback notes

- This batch adds no migration. It uses only existing columns.
- Rollback: revert the commit. No data changes. Restored fields are only the archived_at value.
- Batch 2 (field grants) needs a new table. Its migration is additive, so rollback is `DROP TABLE` with no effect on values.
- DR-07 and DR-08 change assignment semantics. They need a backfill that keeps every current item visible to its current Space. Plan it before Batch 4.

## 6. Test evidence (this batch)

| Test | Result |
|---|---|
| `TestCustomFieldArchiveRestoreImpact` (Go, real Postgres, new) | PASS |
| `TestWorkspaceCustomFields` (Go, existing) | PASS |
| `go test ./server/internal/items/...` and related httpserver tests (`Custom|Formula|ItemType|Workspace|Guard|Relation|Field|Form`) | PASS |
| `tests/custom-fields.spec.ts` (Playwright mock, 5 tests, updated for the confirmation step and restore) | PASS |
| `tsc --noEmit`, `eslint` on changed files, `lint:tokens` | PASS |
| Field permission tests (Batch 2) | NOT RUN (not built) |
| Registry, drawer, assignment, board/table E2E (Batches 3 to 5) | NOT RUN (not built) |
| Status guard and automation after archive (R-30, R-31) | NOT RUN |
| Live browser check on losaka-app | NOT RUN |

## 7. Status after Batches 2 and 3

- **Batch 2 (field grants):** backend done. Table `custom_field_grants` (00331). Resolver in `policy/field_access.go`. Enforced on item read, item write, list sort and filter, analytics, Excel export, and public dashboards. Grants API with audit. Test: `TestCustomFieldAccessGrants` PASS. Not covered yet: automation conditions (DR-03), guard error text that names a restricted field, relations, search.
- **Batch 3 (registry):** Settings > Custom fields page (`/settings/custom-fields`) with Workspace and Space scopes, search, status filter, sort, and a right-side drawer. The drawer has General, Type settings, Ownership and applicability, Permissions, and Preview. Unsaved changes ask before closing. Mock tests: 2 PASS. Full Playwright mock suite: 542 PASS, 43 skipped.
- **Shortcomings:** no description, author, or timestamps (the table has none); no Access or applicability column in the table; no column configuration (P2); no concurrency token for field edits (the API has no version); the drawer uses one width; dark-mode axe not run; no live browser check on losaka-app.

## 8. Status after Batch 4 (assignments)

- **Decisions applied:** DR-07 (an item sees a field if any of its Spaces applies it) and DR-08 (a Workspace field applies in a Space only when assigned). Project and folder assignments narrow a field to their subtree. This is DR-13, recorded here: once a field has any project or folder assignment in a Space, it applies only inside those subtrees. A field with none applies to the whole Space.
- **Migration 00332:** table `custom_field_assignments`, function `custom_field_applies(item, field, space)`, backfill of every existing Workspace field to every existing Space, so current values stay visible. **Rollout effect:** a Space created after this migration does not see Workspace field values until a Space manager assigns the field.
- **Enforced:** item read, value write (create, patch, blueprint, form submit), list sort and filter, analytics field values, Excel export. Removing an assignment hides values and keeps them stored.
- **API:** `GET|POST /api/v1/spaces/{id}/custom-fields/{fieldId}/assignments`, `DELETE .../assignments/{kind}/{scopeId}`, `GET /api/v1/spaces/{id}/field-choices` (Full role only).
- **UI:** the registry drawer lists the Spaces a Workspace field applies in, with a checkbox per Space.
- **Tests:** `TestCustomFieldAssignments` PASS. Existing tests updated for DR-08 (assignment before use) and for Space isolation in analytics. Full Go suite PASS. Full Playwright mock suite 543 PASS, 43 skipped.
- **Not built:** project and folder assignment UI (menus, item picker), Custom Item Type builder entry, Table and Board field menus, defaults gating in `typeFieldDefaults`, automation conditions, search, relations, and the guard message. Project and folder assignments exist in the API only.

## 9. Gaps closed after Batch 4, and Batch 5 entry points

- **Type defaults:** written only for fields that apply at Space level, have no project or folder assignment, and have no grants (fail safe, since the item's container is unknown at create).
- **Guards:** a refusal names a required field only to people who can read it.
- **Automation:** conditions read only values the rule owner may read where the field applies. Other fields count as absent (condition not met). Actions write through the normal patch path.
- **Relations:** an item's relation lists a field only when the caller can read it and it applies to the item.
- **Search:** field values are not indexed, so nothing to gate.
- **Project and folder menu:** "Custom fields…" in item detail (Full role in the item's first Space) sets which fields apply to that container.
- **Batch 5 entry points (Table and Board):** the Space's view field list (`GET /api/v1/spaces/{id}/fields`) includes its own fields and assigned Workspace fields, without fields the viewer cannot read. The Columns (table) and Card fields (board) menu has, for Full members, "Add to Space" / "Remove from Space" per Workspace field and "Create field…" (drawer).
- **Tests:** Go: guard disclosure, assignment list. Playwright mock: project menu, registry drawer, Workspace assignment in drawer, table Columns menu. Full Go suite PASS; full Playwright mock suite PASS (see report).
- **Still open:** the board menu entry is the same component as the table menu but has no dedicated test; relations and automation have no dedicated test; live check on losaka-app not done.
