import assert from "node:assert/strict";
import test from "node:test";
import { etag, ownRank, sortSpaces, type Group, type Space } from "./spaces.ts";

const space = (id: string, kind: Space["kind"]): Space => ({ id, name: id, kind, version: 3 });

test("personal space sorts first and etag is quoted", () => {
  assert.deepEqual(sortSpaces([space("a", "shared"), space("p", "personal"), space("b", "shared")]).map((s) => s.id), ["p", "a", "b"]);
  assert.equal(etag({ version: 7 }), '"7"');
});

test("ownRank counts direct and group rows, -1 when none", () => {
  const groups: Group[] = [{ id: "g1", name: "Team", member_ids: ["me"] }, { id: "g2", name: "Other", member_ids: [] }];
  const rows = [
    { principal_type: "group" as const, principal_id: "g1", role: "editor" as const },
    { principal_type: "group" as const, principal_id: "g2", role: "full" as const },
  ];
  assert.equal(ownRank(rows, "me", groups), 2);
  assert.equal(ownRank([...rows, { principal_type: "user", principal_id: "me", role: "full" }], "me", groups), 3);
  assert.equal(ownRank([], "me", groups), -1);
});
