#!/usr/bin/env python3
"""Isolated UX QA seed for the Work Area epic (Phase 3).

Creates Space "ITA Internal Timeline" and project "Lalang Platform — UX QA" with five workstream folders and
about 30 tasks, using only the public API. Every title is prefixed with the QA marker so the data is easy to spot.

Usage (local database, never dev-lens unless explicitly approved):
  QA_BASE=http://localhost:8080 QA_ORIGIN=http://localhost:5173 QA_EMAIL=... QA_PASSWORD=... python3 ux_qa_seed.py

Writes the created ids to ux_qa_ledger.json next to this file. Re-running creates a second project; delete the old one first.
"""
import json, os, sys, http.cookiejar, urllib.request
from datetime import date

BASE = os.environ.get("QA_BASE", "http://localhost:8080")
ORIGIN = os.environ.get("QA_ORIGIN", "http://localhost:5173")
EMAIL = os.environ["QA_EMAIL"]
PASSWORD = os.environ["QA_PASSWORD"]
MARK = "[UXQA]"
HERE = os.path.dirname(os.path.abspath(__file__))

jar = http.cookiejar.CookieJar()
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))


def call(method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE + path, data, {"Content-Type": "application/json", "Origin": ORIGIN}, method=method)
    try:
        with opener.open(req) as resp:
            raw = resp.read()
            return resp.status, (json.loads(raw) if raw else None)
    except urllib.error.HTTPError as err:
        raw = err.read()
        return err.code, (json.loads(raw) if raw else None)


status, _ = call("POST", "/api/v1/auth/login", {"email": EMAIL, "password": PASSWORD})
assert status == 200, f"login failed: {status}"

workflows = call("GET", "/api/v1/workflows")[1]
workflows = workflows.get("items", workflows)
default = next(w for w in workflows if w.get("is_default"))
status_id = {s["name"]: s["id"] for s in default["statuses"]}

log = []


def expect(result, what):
    code, body = result
    if code not in (200, 201, 204):
        log.append(f"FAIL {what}: {code} {str(body)[:200]}")
        return None
    return body


space = expect(call("POST", "/api/v1/spaces", {"name": f"{MARK} ITA Internal Timeline", "description": "UX QA seed"}), "space")
space_id = space["id"]

project = expect(call("POST", "/api/v1/items", {
    "title": f"{MARK} Lalang Platform — UX QA", "type": "project",
    "locations": [{"space_id": space_id}], "start_date": "2026-11-02", "due_date": "2026-12-18",
}), "project")
project_id = project["id"]

WORKSTREAMS = ["Process Safety", "HAZOP", "HAZID", "MAH", "SECE"]
folder = {}
for name in WORKSTREAMS:
    f = expect(call("POST", "/api/v1/items", {
        "title": f"{MARK} {name}", "type": "folder",
        "locations": [{"space_id": space_id, "parent_item_id": project_id}],
    }), f"folder {name}")
    folder[name] = f["id"] if f else None

# (workstream, title, status, start, due, extras). Dates cover single-day, multi-week, cross-month, overdue,
# unscheduled and milestone cases. Weekend starts are avoided: the work schedule rejects them.
T = "New"
P = "In Progress"
D = "Completed"
ITEMS = [
    ("Process Safety", "Equipment Datasheet", D, "2026-11-02", "2026-11-06", {}),
    ("Process Safety", "Hazardous Area Classification", P, "2026-11-09", "2026-11-20", {}),
    ("Process Safety", "Cause and Effect Diagram", T, "2026-11-23", "2026-12-04", {}),
    ("Process Safety", "Report Submission IFR", T, "2026-11-30", "2026-12-01", {"milestone": True}),
    ("Process Safety", "Report Submission IFA", T, None, None, {}),
    ("HAZOP", "Preparation", D, "2026-11-03", "2026-11-13", {}),
    ("HAZOP", "Workshop", P, "2026-11-16", "2026-11-27", {}),
    ("HAZOP", "Report", T, "2026-11-30", "2026-12-11", {}),
    ("HAZOP", "Report Submission IFR", T, "2026-12-14", "2026-12-14", {"milestone": True}),
    ("HAZOP", "Final Review", T, "2026-12-15", "2026-12-18", {}),
    ("HAZID", "Scenario list", D, "2026-11-04", "2026-11-05", {}),
    ("HAZID", "Risk workshop", P, "2026-11-10", "2026-11-12", {}),
    ("HAZID", "Action register", P, "2026-11-11", "2026-11-18", {}),
    ("HAZID", "Closure report", T, "2026-11-25", "2026-12-08", {}),
    ("HAZID", "Late review (overdue)", P, "2026-11-02", "2026-11-09", {}),
    ("MAH", "Inventory of hazardous material", D, "2026-11-02", "2026-11-17", {}),
    ("MAH", "Consequence modelling", P, "2026-11-18", "2026-12-02", {}),
    ("MAH", "Emergency response plan", T, "2026-12-03", "2026-12-16", {}),
    ("MAH", "Stakeholder sign-off", T, None, None, {"milestone": True}),
    ("MAH", "Dependency gate", T, "2026-11-30", "2026-12-03", {}),
    ("SECE", "Scope definition", D, "2026-11-02", "2026-11-03", {}),
    ("SECE", "Element list", P, "2026-11-04", "2026-11-24", {}),
    ("SECE", "Verification plan", T, "2026-11-25", "2026-12-10", {}),
    ("SECE", "Unscheduled backlog item", T, None, None, {}),
    ("SECE", "Overlap with element list", P, "2026-11-20", "2026-11-27", {}),
    ("Process Safety", "Overlap with datasheet", P, "2026-11-04", "2026-11-10", {}),
    ("HAZOP", "Single-day walkthrough", T, "2026-11-17", "2026-11-17", {}),
    ("HAZID", "Week-spanning survey", P, "2026-11-23", "2026-11-27", {}),
    ("MAH", "Cross-month review", T, "2026-11-27", "2026-12-04", {}),
    ("SECE", "Completed short task", D, "2026-11-16", "2026-11-16", {}),
]

task_ids = {}
for workstream, title, st, start, due, extra in ITEMS:
    body = {"title": f"{MARK} {title}", "type": "task",
            "locations": [{"space_id": space_id, "parent_item_id": folder[workstream]}],
            "status_id": status_id[st]}
    if start:
        body["start_date"] = start
        body["due_date"] = due
    if extra.get("milestone"):
        body["is_milestone"] = True
    created = expect(call("POST", "/api/v1/items", body), f"task {title}")
    if created:
        task_ids[title] = created["id"]

# A subitem under a task, to exercise parent-child hierarchy and the subitem creation rule.
sub = expect(call("POST", "/api/v1/items", {
    "title": f"{MARK} Datasheet checklist", "type": "task",
    "locations": [{"space_id": space_id, "parent_item_id": task_ids.get("Equipment Datasheet")}],
}), "subitem")

ledger = {"marker": MARK, "space_id": space_id, "project_id": project_id, "folders": folder,
          "tasks": task_ids, "subitem_id": sub and sub["id"], "errors": log,
          "counts": {"tasks": len(task_ids), "folders": len(folder)}}
with open(os.path.join(HERE, "ux_qa_ledger.json"), "w") as fh:
    json.dump(ledger, fh, indent=1, ensure_ascii=False)
print(json.dumps(ledger["counts"]), "errors:", len(log))
for line in log:
    print(line, file=sys.stderr)
