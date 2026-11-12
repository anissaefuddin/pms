import assert from "node:assert/strict";
import test from "node:test";
import { aggregateCustomField, customFieldMarker, formatCustomNumber, parseCustomNumber, validCustomNumber, type NumericField } from "./custom-fields.ts";

test("round-trips localized currency and rejects malformed grouping", () => {
  const usd: NumericField = { type: "currency", config: { currency: "USD" } };
  const idr: NumericField = { type: "currency", config: { currency: "IDR" } };
  assert.equal(formatCustomNumber(usd, 1234.5, "en-US"), "$1,234.50");
  assert.equal(formatCustomNumber(idr, 1234.5, "id-ID").replace(/\s/g, " "), "Rp 1.234,50");
  assert.equal(parseCustomNumber(usd, "-$1,234.50", "en-US"), -1234.5);
  assert.equal(parseCustomNumber(idr, "-Rp 1.234,50", "id-ID"), -1234.5);
  assert.equal(parseCustomNumber(idr, ",50", "id-ID"), 0.5);
  assert.equal(parseCustomNumber(usd, "1,2,3", "en-US"), undefined);
  assert.equal(parseCustomNumber(idr, "1,234.50", "id-ID"), undefined);
  assert.equal(parseCustomNumber(usd, "1$2.00", "en-US"), undefined);
  assert.equal(parseCustomNumber({ type: "number" }, "1,000,000,000.00000001", "en-US"), undefined);
  assert.equal(validCustomNumber(usd, 12.345), false);
  assert.equal(validCustomNumber({ type: "currency", config: { currency: "IDR", decimals: 0 } }, 12.5), false);
});

test("handles percent and duration limits", () => {
  const percent: NumericField = { type: "percent" };
  assert.equal(formatCustomNumber(percent, 25.25, "id-ID"), "25,25%");
  assert.equal(parseCustomNumber(percent, "25,25%", "id-ID"), 25.25);
  assert.equal(validCustomNumber(percent, 100), true);
  assert.equal(validCustomNumber(percent, -1), false);
  assert.equal(validCustomNumber(percent, 100.01), false);
  assert.equal(validCustomNumber({ type: "percent", config: { allow_over_100: true } }, 100.01), true);
  assert.equal(validCustomNumber({ type: "number" }, 100.123456), true);
  assert.equal(validCustomNumber({ type: "number" }, 100.1234567), false);
  assert.equal(validCustomNumber({ type: "duration" }, 1.25), false);
  assert.equal(validCustomNumber({ type: "duration" }, 0), true);
  assert.equal(validCustomNumber({ type: "duration" }, 2_147_483_648), false);
});

test("applies the first matching marker including inclusive comparisons", () => {
  const markers = [
    { operator: "lt", value: 0, color: "#ff0000" },
    { operator: "eq", value: 0, color: "#000000" },
    { operator: "lte", value: 10, color: "#ffff00" },
    { operator: "gte", value: 10, color: "#00ff00" },
  ] as const;
  const field: NumericField = { type: "number", config: { markers: [...markers] } };
  assert.equal(customFieldMarker(field, -1), markers[0]);
  assert.equal(customFieldMarker(field, 0), markers[1]);
  assert.equal(customFieldMarker(field, 10), markers[2]);
  assert.equal(customFieldMarker(field, 11), markers[3]);
  assert.equal(customFieldMarker({ type: "number", config: { markers: [{ ...markers[3], operator: "gt" }] } }, 10), undefined);
});

test("aggregates finite numeric values including zero", () => {
  const values = [null, undefined, false, "2", 0, 1.25, -0.25, 2, Infinity, NaN];
  const expected = { sum: 3, average: 0.75, min: -0.25, max: 2, count: 4 } as const;
  for (const aggregation of ["sum", "average", "min", "max", "count"] as const) {
    const field: NumericField = { type: "currency", config: { currency: "USD", aggregation } };
    assert.equal(aggregateCustomField(field, values), expected[aggregation]);
    assert.equal(aggregateCustomField(field, []), aggregation === "count" ? 0 : undefined);
  }
  assert.equal(aggregateCustomField({ type: "number" }, values), undefined);
});
