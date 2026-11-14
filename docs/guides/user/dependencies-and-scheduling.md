# Dependencies and scheduling

## Purpose
Link items so that finishing/starting one drives another, and keep dates on working days.

## Dependencies (Gantt)
1. Open a project's **Gantt** view; select a task.
2. In **Predecessors of {name}** use **Add predecessor**: choose the **Predecessor** and the **Type**: **FS** (finish-to-start), **SS**, **FF**, **SF**; confirm.
3. Change a link's type via **Link type from {name}**; delete with **Remove link from {name}** (**Remove**). “No predecessors.” shows when empty.
4. Moving a predecessor shifts dependent tasks; scheduling is calculated by the server (working days only).

## Working days, duration, effort
- Dates must be on working days (“Choose dates from Monday to Friday” by default). Administrators set working days, hours per day and holidays in Settings → Work schedule.
- **Effort and allocation:** **Effort (hours)**; per assignee **Allocation** as **Percent** or **Hours per day**; **Save effort and allocation**. “Total allocation exceeds 100%…” warns of over-allocation. **Recalculate duration** recomputes workdays from effort/allocation. Duration shows as “Duration: N workdays”.
- Milestones: **Convert to milestone** in item actions.

## Permissions
Changing dates/links needs edit rights (“You cannot change dates of {name}” otherwise).

## Limitations
Lag/lead and critical path are not shown in the UI strings. Calendar/Table column **Predecessors** is sort/display only.
