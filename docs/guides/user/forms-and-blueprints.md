# Forms and Blueprints

## Purpose
**Forms** let people send requests that become items. **Blueprints** save an item structure to reuse.

## Build a form
1. In a Space choose **Forms** → **New form** (or **Edit**/**Delete** on a row; **Manage all forms** lists every form you can edit).
2. Set **Form name** and **Description**.
3. **Add question**; kinds: **Single line text**, **Paragraph**, **Single choice**, **Multiple choice**, **Date**, **Number**, **Attachment** (max 1–5 files), **Assignee**, **Importance**, **Section header**. Each has **Label**, **Help text**, **Required**; choice kinds have **Options**; number has **Minimum/Maximum**. **Add page** splits into pages.
4. **Visibility:** show a question only when an earlier one is answered a certain way (is / is not / contains / is answered / is not answered).
5. **Response settings:** **Create as** Task or Project, **Destination Space**, **Parent project or folder**, **Initial status**, **Assignees**, **Item title comes from**, and **Request approval for each new item** (with **Approvers**).
6. **Item type and fields:** routes (“when answer is X, create type Y with blueprint Z”) and **Fill custom fields from answers**.
7. **Confirmation email** (for public submitters): **Sender name**, **Logo address (https)**, **Greeting**, **Closing**, **Message**; placeholders `{name}`, `{title}`, `{form}`.
8. Check the **Preview**, then **Save form**.

## Share a form
**Share / Change access**: **Only form managers** or **Everyone in the workspace** (appears under **Request forms** with a link). **Public link**: **Create public link**, **Replace link** (old link stops), **Turn off link**. Anyone with it can submit without signing in; questions naming people are left out.

## Submit a request
Open **Request forms** (sidebar), choose **Start**, answer, **Next**/**Back**, **Submit**. Track under **My requests**. Public submitters enter **Your name** and **Your email** and receive a confirmation email.

## Blueprints
- **Save:** item actions → **Save as blueprint**, enter **Blueprint name**, **Save blueprint**. It copies the item and the visible items inside it (status, assignees, dates, custom fields, links between them). Comments, files, approvals and sharing are not copied.
- **Use:** when creating an item choose **Start from blueprint**. Dates are counted from the **Start date** you set; without one, new items have no dates.

## Permissions
Managing forms needs edit rights in the Space; only Space owners change who can fill a form. Keeping blueprints needs edit access to a shared Space.

## Limitations
Public forms are rate limited (“Too many requests”). Customising blueprint menu icons/names (PRD SPC-14) is not confirmed in code; see [“+” menu](create-menu.md) for what exists.
