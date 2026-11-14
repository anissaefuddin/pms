# Spaces, projects and folders

## Purpose
A **Space** is the top-level container for work. Inside a Space you organise work as **Projects** and **Folders**, which can nest. Every user also has a **Personal** Space.

## Steps
- **Create a Space:** in the sidebar under *Spaces*, choose **New Space**. Enter a **Name** (required) and optionally a **Description**. Under **Start from** pick **Blank Space** or a template (the dialog lists what it includes, e.g. projects, tasks, extra views, a form and custom fields). Choose **Create Space**.
- **Rename or delete:** open the Space's **Space actions** menu, then **Rename** or **Delete**. Delete moves the Space and everything in it to the Recycle bin (see [Recycle bin](search-and-navigation.md#recycle-bin-starred-shared-with-me)).
- **Browse:** the sidebar tree shows Projects and Folders; use the expand/collapse arrows. **Browse inside** on an item opens its children.
- **Create a Project or Folder:** see [Items](items.md#create-an-item).
- **Share:** choose **Share** on the Space (or an item). Search people or groups, pick a role (Full, Editor, Limited, Read only, or a custom role), then **Save**. See [Sharing and roles](#sharing-and-roles).
- **Lock a Space:** **Lock space** / **Unlock space** (badge: *Locked space*). A locked Space's items can only be shared with people who already have the Space, and only an administrator can change access or move items out.
- **Bookmarks:** the Space's **Bookmarks** page keeps external links. Choose **Add a bookmark**, fill **URL**, **Title**, **Description**, then **Add bookmark**. Only `http` and `https` addresses are accepted.
- **Space-level pages:** **Custom fields** ([Custom fields](custom-fields.md)), **Forms** ([Forms](forms-and-blueprints.md)), **Automation** ([Automation](automation.md)), **“+” menu** ([“+” menu](create-menu.md)).

## Sharing and roles
| Role | What it allows (as shown in the app) |
|---|---|
| Full | Everything an Editor can, plus delete and share |
| Editor | Edit and create, plus everything Limited can |
| Limited | Comment, change status, attach files |
| Read only | View only |

Access set on a parent is shown as **Inherited**; change it where it was set. If your change lowers or removes your own access, the dialog warns you and asks **Save anyway** or **Go back**. If someone else changed sharing meanwhile, you get a **Reload** prompt. Typing an email with no account offers **Invite as external user**.
Administrators can define **custom roles** (View, Comment, Change status, Attach files, Edit, Create, Delete, Share); a role never gives more than the user's type allows.

## Permissions
Creating, renaming or deleting needs the matching capability on the Space (you only see Spaces where you have **create** when picking a location).

## Limitations
- Excel export/import of a Space's hierarchy exists (**Export to Excel**, **Import from Excel**) but its entry point could not be confirmed from the code; ask your administrator.
- Space templates are provided by the server; this guide does not list them.
