import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { OTHERS_KEY, contrast, foldOthers, toGrid } from "./charts.ts";

const row = (key: string, value: number) => ({ key, label: key, value });

test("foldOthers keeps the largest slices and sums the rest into Others", () => {
  const rows = Array.from({ length: 12 }, (_, i) => row(`k${i}`, i + 1));
  const folded = foldOthers(rows, 5, "Others");
  assert.equal(folded.length, 5);
  assert.deepEqual(folded.slice(0, 4).map((r) => r.value), [12, 11, 10, 9]);
  const others = folded[4];
  assert.equal(others.key, OTHERS_KEY);
  assert.equal(others.value, 1 + 2 + 3 + 4 + 5 + 6 + 7 + 8); // everything below the kept four
  assert.equal(folded.reduce((sum, r) => sum + r.value, 0), rows.reduce((sum, r) => sum + r.value, 0), "no value is lost");
});

test("foldOthers leaves small sets alone and drops zero slices", () => {
  assert.deepEqual(foldOthers([row("a", 2), row("b", 0), row("c", 5)], 8).map((r) => r.key), ["c", "a"]);
});

test("toGrid pivots rows into categories and series, filling gaps with zero", () => {
  const label = (_key: string, text: string) => text;
  const rows = [
    { group: { key: "a", label: "A" }, break: { key: "x", label: "X" }, value: 1 },
    { group: { key: "a", label: "A" }, break: { key: "y", label: "Y" }, value: 2 },
    { group: { key: "b", label: "B" }, break: { key: "y", label: "Y" }, value: 3 },
  ];
  const grid = toGrid(rows, label, "Total");
  assert.deepEqual(grid.categories.map((c) => c.label), ["A", "B"]);
  assert.deepEqual(grid.series, [{ name: "X", values: [1, 0] }, { name: "Y", values: [2, 3] }]);
  assert.deepEqual(toGrid([{ group: { key: "a", label: "A" }, break: null, value: 4 }], label, "Total").series, [{ name: "Total", values: [4] }]);
});

test("contrast follows WCAG", () => {
  assert.equal(Math.round(contrast("#000000", "#FFFFFF")), 21);
  assert.equal(contrast("#777777", "#777777"), 1);
});

// Series colors are graphical objects (WCAG 1.4.11): each needs 3:1 against the card it is drawn on, in both themes.
test("chart series colors have 3:1 contrast on the card in light and dark themes", () => {
  const css = readFileSync(new URL("../index.css", import.meta.url), "utf8");
  const token = (block: string, name: string) => new RegExp(`${name}:\\s*(#[0-9A-Fa-f]{6})`).exec(block)?.[1];
  const dark = css.slice(css.indexOf(".dark {"), css.indexOf("/* Tenant brand"));
  const light = css.slice(0, css.indexOf(".dark {"));
  for (const [theme, block] of [["light", light], ["dark", dark]] as const) {
    const card = token(block, "--card") ?? token(light, "--card")!;
    for (let n = 1; n <= 6; n++) {
      const color = token(block, `--cat-${n}`) ?? token(light, `--cat-${n}`)!;
      assert.ok(contrast(color, card) >= 3, `${theme} --cat-${n} ${color} on ${card} = ${contrast(color, card).toFixed(2)}`);
    }
  }
});
