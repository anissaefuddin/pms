# RBAC UI and action inventory

Audit date: 23 November 2026. `Current behavior` reflects `v0.10.0` plus the uncommitted low-risk UI remediation recorded in the gap report.

Legend: **OK** verified aligned; **FIXED** remediated in this audit; **NI** not implemented; **ADMIN** workspace administrator; **SA** explicit Space Admin.

| ID | Location | Component | UI element | Action | Endpoint | Required capability | Current behavior |
|---|---|---|---|---|---|---|---|
| NAV-01 | Sidebar | `sidebar.tsx` | Space list/tree | list/open visible Spaces | `GET /spaces`, `GET /spaces/{id}` | view | OK; server-filtered, personal owner-only |
| NAV-02 | Sidebar | Space row | New project `+` | create root project | `POST /items` | create | OK; hidden without capability |
| NAV-03 | Sidebar | Space menu | rename | `PATCH /spaces/{id}` | edit | OK |
| NAV-04 | Sidebar | Space menu | Members | settings route, shares API | share | OK |
| NAV-05 | Sidebar | Space menu | Space settings | direct route | view | OK; controls gate separately |
| NAV-06 | Sidebar | Space menu | archive | `POST /spaces/{id}/archive` | SA | OK |
| NAV-07 | Sidebar | Item tree | open/expand | item/children GET | view | OK |
| NAV-08 | Sidebar | Item menu | star/unstar | `PUT/DELETE /items/{id}/star` | view | OK, personal preference |
| NAV-09 | Sidebar | Customize | pin/hide shortcuts | `PATCH /me` | authenticated self | OK; preferences do not create resource access |
| NAV-10 | Sidebar | Shared with me | open list | `GET /shared-with-me` | view | OK |
| NAV-11 | Topbar | Create | destination actions | `POST /items` | create at destination | OK; destination menu comes from `/spaces/{id}/create-menu` |
| NAV-12 | Topbar | Search/palette | search/open | `GET /search` | view | OK; server scoped |
| NAV-13 | Topbar | Notifications/account | personal actions | notifications, `/me` | authenticated self | OK; independent of current Space role |
| NAV-14 | Direct URL | Router + API | open Space/item/settings | corresponding GET | view | OK; API is final guard |
| SPC-01 | Space page | Header | New item | `POST /items` | create | OK; hidden without create |
| SPC-02 | Space page | Header actions | Share | `GET/PUT /spaces/{id}/shares` | share | OK |
| SPC-03 | Space settings/General | form | rename/description/icon | `PATCH /spaces/{id}` | edit | OK; archived/personal read-only |
| SPC-04 | Space settings/Lifecycle | buttons | visibility/archive/restore | Space PATCH, archive/restore | SA | OK; Full alone insufficient |
| SPC-05 | Space settings/Members | share editor | replace members | `PUT /spaces/{id}/shares` | share | OK |
| SPC-06 | Space settings/Admins | admin editor | grant/revoke admin | `/spaces/{id}/admins` | SA | OK; eligible user types enforced backend |
| SPC-07 | Space settings/Workflows | selects | defaults | `PUT /spaces/{id}/default-workflows` | edit | OK |
| SPC-08 | Space settings/Item types | registry/editor | create/update/assign fields | item-type APIs | ADMIN or Space-scoped management rule | OK; backend-specific rules retained |
| SPC-09 | Space settings/Custom fields | registry/drawer | create/edit/archive/grants | custom-field APIs | ADMIN or Space manager + field grant | OK; separate grant checks |
| SPC-10 | Space settings/Automation | editor | create/edit/run | automation APIs | edit/manage rule scope | OK |
| SPC-11 | Space settings/Forms | manager | create/edit/publish/delete | forms APIs | edit/create/delete by handler | OK |
| SPC-12 | Space settings/Blueprints | list/editor | view/use/manage | blueprint APIs | view/use/manage_blueprint | OK |
| SPC-13 | Space menu | duplicate Space | — | — | — | NI |
| ITM-01 | Item detail | Title | edit title | `PATCH /items/{id}` | edit | OK |
| ITM-02 | Item detail | Status | change status | `PATCH /items/{id}` | change_status | OK |
| ITM-03 | Item detail | Assignee/dates/importance/effort | edit attributes | `PATCH /items/{id}` | edit | OK |
| ITM-04 | Item detail | Custom fields | edit value | `PATCH /items/{id}` | edit + field grant | OK |
| ITM-05 | Item detail | Add subitem | create child | `POST /items` | create on destination/parent | OK |
| ITM-06 | Item detail | Delete | delete item | `DELETE /items/{id}` | delete | OK |
| ITM-07 | Item detail | Share | replace item shares | `PUT /items/{id}/shares` | share | OK |
| ITM-08 | Item detail | Locations | add/move/remove | `PUT /items/{id}/locations` | edit plus source/destination checks | OK; backend cross-Space/cycle guards |
| ITM-09 | Item detail | Save as blueprint | save source structure | `POST /spaces/{id}/blueprints` | create_blueprint at destination + source view | FIXED; hidden if no eligible destination |
| ITM-10 | Item detail | Automation link | open container rules | route + automation APIs | edit | OK |
| ITM-11 | Item detail | Export project/folder | export workbook | project Excel APIs | view | OK; export is scoped to visible hierarchy |
| CMT-01 | Comments | composer | add comment/mention | `POST /items/{id}/comments` | comment | OK |
| CMT-02 | Comments | comment actions | edit/delete own comment | `PATCH/DELETE /comments/{id}` | comment + author, or full item authority | OK |
| ATT-01 | Attachments | upload/link | attach file/link | attachment POST APIs | attach | OK |
| ATT-02 | Attachments | version | upload new version | `POST /attachments/{id}/versions` | attach | OK |
| ATT-03 | Attachments | remove | delete attachment | `DELETE /attachments/{id}` | delete, or uploader+attach | OK |
| ATT-04 | Attachments | download/history/compare | read versions | attachment GET APIs | view | OK |
| ATT-05 | Proofing | annotation controls | add/resolve/delete annotation | annotation APIs | comment/author rules | OK |
| APR-01 | Approvals | create/edit/withdraw | manage approval | approval APIs | edit and handler-specific reviewer rules | OK |
| APR-02 | Approvals/Reviews | decision | approve/reject | `PUT /approvals/{id}/decision` | named reviewer + visible item | OK |
| TBL-01 | Table | inline cells | status | item PATCH | change_status | OK |
| TBL-02 | Table | inline cells | title/assignee/date/field | item PATCH | edit | OK |
| TBL-03 | Table | bulk controls | bulk status/edit | one PATCH per item | per-item capability | OK; mixed failures reported, successes not retried |
| TBL-04 | Table | drag/reorder | reorder | `POST /items/{id}/reorder` | edit | OK |
| TBL-05 | Table | add row/subitem | create | `POST /items` | create | OK |
| TBL-06 | Table | columns | save view config | `PATCH /views/{id}` | edit view | OK |
| BRD-01 | Board | card drag | status column move | item PATCH | change_status | OK |
| BRD-02 | Board | swimlane drag | assignee move | item PATCH | edit | OK |
| BRD-03 | Board | quick add/card add | create | `POST /items` | create | OK |
| CAL-01 | Calendar | event drag/keyboard move | dates | item PATCH | edit | OK |
| CAL-02 | Calendar | day `+` | create dated item | `POST /items` | create | OK |
| CAL-03 | Calendar | event menu | status/delete/open | item PATCH/DELETE | per-item change_status/delete/view | OK |
| GAN-01 | Gantt | bar drag/resize | dates | item PATCH | edit | OK; readonly library plus per-item check |
| GAN-02 | Gantt | dependency editor | add/change/remove dependency | dependency APIs | edit | OK |
| GAN-03 | Gantt | status cell | change status | item PATCH | change_status | OK |
| GAN-04 | Gantt | add row/timeline | create | `POST /items` | create | OK |
| GAN-05 | Gantt | baseline | create/view baseline | baseline APIs | edit/view | OK |
| FIL-01 | Files view | list/download | read files | item/files and attachment GET | view | OK |
| FIL-02 | Files view | upload/version/delete/proof | mutate files | attachment APIs | attach/delete/comment rules | OK |
| TIM-01 | Timelog/Timesheets | list | view permitted entries | time-entry GET | view of item/scope | OK |
| TIM-02 | Timelog/Timesheets | add/edit/delete own entry | mutate time | time-entry APIs | change_status + ownership | OK; unintuitive mapping documented |
| CHT-01 | Chart | visualization | read analytics | analytics query | view of included scope | OK |
| CHT-02 | Dashboard/chart config | save layout/widgets | dashboard APIs | edit/create/delete by operation | OK |
| VIEW-01 | View tabs | add/duplicate/reorder/hide/default/delete | saved-view APIs | edit | OK |
| VIEW-02 | View toolbar | filter/sort/search/group | list GET/query params | view | OK; no mutation |
| ADM-01 | Personal settings | profile/security/preferences/apps | self APIs | authenticated self | OK; unaffected by Space role |
| ADM-02 | Workspace Users/Groups/User types | CRUD | admin APIs | ADMIN | OK; `requireAdmin` backend |
| ADM-03 | Rates/SSO/SCIM/Roles/Audit/Ownership | administrative controls | admin APIs | ADMIN | OK; direct URL returns forbidden |
| ADM-04 | Work schedule/workflows/resources | workspace configuration | settings APIs | ADMIN or documented workspace rule | OK |
| SYS-01 | Direct API | stale/modified payload | every mutation | corresponding endpoint | backend policy | OK in sampled integration matrix; no UI trust |
| SYS-02 | Revoked access | next request/event | read/mutate | all scoped endpoints | recomputed access | OK; no role cached as authority |
| SYS-03 | Bulk mixed authority | bulk mutation | item PATCH sequence | per item | OK; no silent unauthorized processing |

This inventory intentionally records absent Space duplicate/favorite/hide features as NI rather than inventing permissions. Personal preference actions (star, sidebar order, view preferences) are not share-role mutations.
