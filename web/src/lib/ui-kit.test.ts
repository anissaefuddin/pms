import assert from "node:assert/strict";
import { test } from "node:test";
import { initials, isCatToken, nextIndex, primaryActionFor } from "./ui-kit.ts";

test("initials: satu kata dua huruf, banyak kata maks dua inisial", () => {
  assert.equal(initials("anissa"), "AN");
  assert.equal(initials("Anissa Efuddin"), "AE");
  assert.equal(initials("  budi   santoso  wijaya "), "BS");
  assert.equal(initials(""), "");
});

test("nextIndex: panah membungkus, Home dan End ke ujung, tombol lain diabaikan", () => {
  assert.equal(nextIndex(0, 3, "ArrowRight"), 1);
  assert.equal(nextIndex(2, 3, "ArrowRight"), 0);
  assert.equal(nextIndex(0, 3, "ArrowLeft"), 2);
  assert.equal(nextIndex(1, 3, "ArrowDown"), 2);
  assert.equal(nextIndex(1, 3, "End"), 2);
  assert.equal(nextIndex(1, 3, "Home"), 0);
  assert.equal(nextIndex(1, 3, "Enter"), null);
  assert.equal(nextIndex(0, 0, "ArrowRight"), null);
});

test("primaryActionFor: canCreate=false menyembunyikan aksi utama", () => {
  const action = { label: "Buat", onClick: () => {} };
  assert.equal(primaryActionFor(false, action), undefined);
  assert.equal(primaryActionFor(true, action), action);
  assert.equal(primaryActionFor(true, undefined), undefined);
});

test("isCatToken hanya menerima cat-1 sampai cat-6", () => {
  assert.equal(isCatToken("cat-3"), true);
  assert.equal(isCatToken("cat-7"), false);
  assert.equal(isCatToken("#7C3AED"), false);
  assert.equal(isCatToken(undefined), false);
});
