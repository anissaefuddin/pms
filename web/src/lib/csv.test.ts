import assert from "node:assert/strict";
import test from "node:test";
import { csvCell, toCsv } from "./csv.ts";

test("plain cells are written as-is and null becomes empty", () => {
  assert.equal(csvCell("ada"), "ada");
  assert.equal(csvCell(12), "12");
  assert.equal(csvCell(null), "");
  assert.equal(csvCell(undefined), "");
});

test("cells with commas, quotes and newlines are quoted and quotes doubled", () => {
  assert.equal(csvCell("a,b"), '"a,b"');
  assert.equal(csvCell('say "hi"'), '"say ""hi"""');
  assert.equal(csvCell("line1\nline2"), '"line1\nline2"');
  assert.equal(csvCell("x\r\ny"), '"x\r\ny"');
});

test("formula-like cells are prefixed with an apostrophe", () => {
  assert.equal(csvCell('=HYPERLINK("x")'), `"'=HYPERLINK(""x"")"`);
  assert.equal(csvCell("+1"), "'+1");
  assert.equal(csvCell("-2"), "'-2");
  assert.equal(csvCell("@sum"), "'@sum");
});

test("toCsv joins rows with CRLF and cells with commas", () => {
  assert.equal(toCsv([["id", "name"], [1, "Ada, L."]]), 'id,name\r\n1,"Ada, L."');
  assert.equal(toCsv([]), "");
});
