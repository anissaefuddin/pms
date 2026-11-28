# Views: Table, Board, Gantt, Calendar, Workload (and more)

## Purpose
A Space or Project shows its items in **Views**. Each view is a tab under **Views**. Types available (**Add view**): **Table**, **Board**, **Gantt**, **Calendar**, **Timelog**, **Files**, **Chart**. Workload is a separate page.

## Managing views
- **Add view:** choose **Add view**, enter **View name**, a type and **Visibility** (**Public** or **Personal**).
- **{name} options** menu on a tab: **Rename view**, **Duplicate as public**, **Duplicate as personal**, **Set as default**, **Move left**, **Move right**, **Delete view** (confirmed with “Delete “{name}”?”).
- **Columns / Card fields:** **Columns** → **Choose columns** → **Save columns**; saved with that view. Groups: Management, Schedule, Other, Custom fields (see [Custom fields](custom-fields.md)). Board cards use **Card fields** (e.g. Cover image, ID, Type, Locations, Author, Days in status).

## Filter, sort, group
See [Filters and saved views](filters-and-saved-views.md).

## Table
Rows with the chosen columns; expand/collapse hierarchy; click a name to edit (**Edit name**); **Add item** at the bottom; **Move to top** / **Move after {name}** reorder (sort set to **Manual order**).

## Board
One column per status. Drag a card to change its status. **Group** can be *None* or **Assignee** (swimlanes); dropping a card in another lane asks **Change assignee?** → **Change assignee**. **No status** column shows items with none.

## Gantt
- Bars per item; scale **Days / Weeks / Months** (**Time scale**).
- Drag a bar to move it; keyboard: arrows choose a task, Left/Right move one day, Shift+Left/Right change the finish date.
- Dependencies: see [Dependencies and scheduling](dependencies-and-scheduling.md).
- **Baseline:** **Save baseline** (**Baseline name**, **Create baseline**), pick a baseline to see **Baseline dates** and **Variance**.
- **Share snapshot:** a frozen read-only copy for people without an account: **Snapshot name**, **Link expires after**, **Show assignee names**, **Create link**. The link is shown once (“Copy this link now”). Snapshots list with **Revoke**. Snapshots never show comments, files or costs.
- “You can view this chart but not change dates” means you lack edit rights.

## Calendar
Month grid with **Previous month**, **Today**, **Next month**. Drag an item to another day, or press Alt+arrow to move a day/week. Items with no dates are under **Unscheduled**.

## Timelog, Files, Chart
- **Timelog:** time entries of the project, see [Time tracking](time-tracking.md).
- **Files** (Spaces, projects and folders): the documents of the item (or of every item in the Space) that you may see, with totals (direct and in subitems for projects and folders; the projects that have files for folders and Spaces). **Search files**, filter by **Project**, **Type** and **Review**, switch between **All**, **Direct** and **Subitems**, 50 rows per page. **Location** names where each file sits and opens that item. **Add file** (needs permission to attach) uploads a file or adds an external link to the folder or project you are viewing. **Open folder in Pydio** appears when the item has a valid Pydio folder link. A Space has no files of its own, so it has no **Add file**, direct/subitem split or Pydio link, and needs access to the Space itself. Only the newest 5,000 files are checked.
- **Chart:** **Items by status** bar chart.

## Workload
Open **More → Workload**. Set **From**, **To**, optional **Job role** and **Skill**; **Group by week** switches columns to weeks. Cells show minutes planned + booked over capacity; **Over capacity** is flagged. Panels: **Utilization: planned vs actual**, **Unassigned backlog**, **Assigned tasks** (with **Reassign** → choose **Replacement assignees** → **Save assignment**) and **Bookings** (**Find a project**, **Minutes per day**, **Add booking**, **Remove booking**).

## Permissions
Changing dates/status in a view needs edit/change-status rights on that item. Public views are visible to everyone who can open the container; Personal only to you.

## Limitations
Placeholder text “This view is not available yet” can appear for unsupported types. Workload data depends on administrators defining capacity (Resources settings).
