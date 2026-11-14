# Automation

## Purpose
Rules that run by themselves: **WHEN** an event happens, **IF** conditions hold, **THEN** actions. Rules run with the rights of their owner.

## Steps
1. In a Space choose **Automation** → **New rule**; enter **Rule name**.
2. **When** — Trigger: *An item changes status* (**Moved to status**), *Someone is assigned to an item*, *An item is created* (**Item type**), *A due date is near* (**Days before the due date**), *A form is submitted* (**Form**).
3. **If** — **Add a condition** (all must hold; none = always run). Fields: Importance, Status group, Item type, Assignee, Due date, Start date. Comparisons: is, is not, includes, does not include, is empty/not empty, is set/not set, is before/after (dates `YYYY-MM-DD` or `today`).
4. **Then** — **Add an action**: **Set a field** (importance, dates, effort, duration, status), **Assign people**, **Post a comment**, **Send a notification**, **Create an item**.
5. **Try it**: search an item to see whether the rule would run, without changing anything.
6. **Save draft** or **Publish**. Only a published version runs. List shows **Published** / **Draft only** and an **Enabled** On/Off switch.
7. **Log** shows runs: Done, Not allowed, Conditions not met, Stopped (loop), Stopped (limit), Failed. **Delete** removes the rule and its log permanently.

## Permissions
“You do not have permission to manage automation in this Space” if you lack manage rights.

## Limitations
Loop and quota protection stop runaway rules (see statuses above).
