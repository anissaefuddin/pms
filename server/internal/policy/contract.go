package policy

// The A1 authorization contract (D-17). Each decision has its own name so that administration, content access, reporting
// and approval never silently stand in for one another.

// CanAdministerWorkspace reports Workspace administration: users, workspace settings, workflows, rates. It grants no
// content access to any Space.
func CanAdministerWorkspace(actor Actor) bool { return actor.Type == Admin }

// CanViewItem reports whether an item is readable given the caps the actor holds on it (from ItemCaps / ItemsCaps).
// The caps are the only source: a Workspace admin's type never makes an item readable.
func CanViewItem(caps Caps) bool { return caps.Has(View) }

// CanViewOperationalReport reports whether the actor may request the operational aggregate report. The report returns
// counts only, never item rows; the endpoint must audit each use. Raw data still needs CanViewItem.
func CanViewOperationalReport(actor Actor) bool { return actor.Type == Admin }

// CanApprove reports whether the actor may decide an approval. Only an assignment or an explicit approval grant
// on the approval counts; no user type or Space role lets someone approve by default.
func CanApprove(assigned bool) bool { return assigned }
