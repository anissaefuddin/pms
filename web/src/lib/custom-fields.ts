import type { components } from "@/api/schema";

export type CustomField = components["schemas"]["CustomField"];
export type CustomFieldValue = components["schemas"]["CustomFieldValues"][string];
export type NumericField = Pick<CustomField, "type" | "config">;

function maxDecimals(field: NumericField) {
  return field.type === "currency" ? field.config?.decimals ?? 2 : field.type === "duration" ? 0 : 6;
}

function numberFormatter(field: NumericField, locale: string) {
  if (field.type === "currency") {
    const decimals = maxDecimals(field);
    return new Intl.NumberFormat(locale, {
      style: "currency", currency: field.config?.currency ?? "USD",
      minimumFractionDigits: decimals, maximumFractionDigits: decimals,
    });
  }
  return new Intl.NumberFormat(locale, {
    style: field.type === "percent" ? "percent" : "decimal",
    maximumFractionDigits: 6,
  });
}

export function formatCustomNumber(field: NumericField, value: number, locale: string) {
  return numberFormatter(field, locale).format(field.type === "percent" ? value / 100 : value);
}

export function parseCustomNumber(field: NumericField, input: string, locale: string): number | undefined {
  const parts = numberFormatter(field, locale).formatToParts(12345.67);
  const { group, decimal } = Object.fromEntries(new Intl.NumberFormat(locale).formatToParts(12345.67)
    .map(({ type, value }) => [type, value]));
  let value = input.trim();
  const sign = value.startsWith("-") || value.startsWith("+") ? value[0] : "";
  if (sign) value = value.slice(1).trim();
  const unitIndex = parts.findIndex(({ type }) => type === "currency" || type === "percentSign");
  if (unitIndex === 0 && value.startsWith(parts[0].value)) {
    value = value.slice(parts[0].value.length).trim();
  } else if (unitIndex > 0 && value.endsWith(parts[unitIndex].value)) {
    value = value.slice(0, -parts[unitIndex].value.length).trim();
  }

  const [integer, fraction, extra] = value.split(decimal);
  if (extra !== undefined || (fraction !== undefined && !/^\d+$/.test(fraction))) return;
  if (fraction && fraction.replace(/0+$/, "").length > maxDecimals(field)) return;
  const groups = integer.split(group);
  if (groups.length > 1 && (!/^\d{1,3}$/.test(groups[0]) || !groups.slice(1).every((digits) => /^\d{3}$/.test(digits)))) return;
  const digits = groups.join("");
  if (!/^\d*$/.test(digits) || (!digits && !fraction)) return;
  const parsed = Number(`${sign}${digits}${fraction === undefined ? "" : `.${fraction}`}`);
  return Number.isFinite(parsed) ? parsed : undefined;
}

export function validCustomNumber(field: NumericField, value: number) {
  if (!Number.isFinite(value)) return false;
  if (field.type === "duration") return Number.isInteger(value) && value >= 0 && value <= 2_147_483_647;
  const decimals = maxDecimals(field);
  if (Math.abs(value) > 1_000_000_000 || Number(value.toFixed(decimals)) !== value) return false;
  return field.type !== "percent" || value >= 0 && (field.config?.allow_over_100 || value <= 100);
}

export function customFieldMarker(field: NumericField, value: number) {
  return field.config?.markers?.find((marker) => {
    switch (marker.operator) {
      case "lt": return value < marker.value;
      case "lte": return value <= marker.value;
      case "eq": return value === marker.value;
      case "gte": return value >= marker.value;
      case "gt": return value > marker.value;
    }
  });
}

export function aggregateCustomField(field: NumericField, values: readonly (CustomFieldValue | undefined)[]) {
  const numbers = values.filter((value): value is number => typeof value === "number" && Number.isFinite(value));
  const operation = field.config?.aggregation;
  if (operation === "count") return numbers.length;
  if (!numbers.length) return;
  switch (operation) {
    case "sum": return numbers.reduce((total, value) => total + value, 0);
    case "average": return numbers.reduce((total, value) => total + value, 0) / numbers.length;
    case "min": return numbers.reduce((lowest, value) => Math.min(lowest, value));
    case "max": return numbers.reduce((highest, value) => Math.max(highest, value));
  }
}
