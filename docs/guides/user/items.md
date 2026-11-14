# Items: types, status, dates, assignees, description

## Purpose
Everything you work on is an **item**: a **Task**, **Project** or **Folder** (or a custom type such as Bug, created by an administrator in Settings → Item types). Click an item to open **Item details**.

## Create an item
1. Use the create control (**New item**; inside a view, **Add item**; on an item, **Add subitem**).
2. In **Create item**, set **Type** (Project, Folder, Task, or custom types under *More*), **Name** (required), **Space**, **Parent** (or *Space root*).
3. Projects also offer **Workflow** (*Default*), **Owner**, **Start date**, **Finish date**, **Default view** (Table or Board) and **Sharing**. A **Start from blueprint** option is described in [Forms and blueprints](forms-and-blueprints.md).
4. Choose **Create item**.

Which types are offered can be limited per Space: see [“+” menu](create-menu.md).

## Edit an item
- **Name:** inline edit in a Table row or the detail panel.
- **Status:** the **Status** control follows the item's workflow. Board: drag a card to another column, or use **Move to status**. If the workflow blocks the move you see “*{title}* was not moved” (**Dismiss**).
- **Dates:** **Start date** and **Finish date**. Finish must be on or after start. Dates must fall on working days (default Monday–Friday; set by the workspace work schedule). See [Dependencies and scheduling](dependencies-and-scheduling.md).
- **Assignees:** **Assignees** → **Search active users**; several people allowed; **Remove {name}** removes one.
- **Description:** rich text in **Description**.
- **Subitems:** **Subitems** section, **Add subitem**; each shows a count (e.g. “3 subitems”). **Back to parent** returns up.
- **Locations:** **Locations** → **Add location** adds the item to more Spaces/Projects; the last location cannot be removed.
- **Progress (projects):** **Progress method** — *Completed task count*, *Weighted by duration*, *Average task progress*; or **Manual progress (%)** then **Save progress** (leave blank to clear). Risk shows Green/Yellow/Red.
- **Item actions** menu: **Copy link**, **Convert to milestone**, **Save as blueprint**, **Delete item** (moves it to the Recycle bin; **Undo** is offered). A star icon (**Star this item**) bookmarks it.
- **Activity:** the **Activity** section lists changes; **Load more**.

## Concurrent edits
If someone else changed the item you see “Someone else changed this item. Review the latest version.” Choose **Reapply my changes** or **Discard my changes**.

## Permissions
Editing needs *Edit*; changing status needs *Change status*; deleting needs *Delete*. Read-only users see fields but no editors. See roles in [Spaces and folders](spaces-and-folders.md#sharing-and-roles).

## Limitations
- Importance (High/Normal/Low) appears on comments/forms and in reports; where it is edited on an item panel was not confirmed from the code.
- Baseline columns are marked “not available yet”.
