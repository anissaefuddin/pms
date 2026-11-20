// Formula-looking cells get a leading apostrophe so spreadsheets do not run user-typed text (actor names, IPs) as formulas.
const FORMULA_START = /^[=+\-@\t\r]/;
const NEEDS_QUOTES = /[",\r\n]/;

export function csvCell(value: unknown): string {
  let text = value == null ? "" : String(value);
  if (FORMULA_START.test(text)) text = `'${text}`;
  return NEEDS_QUOTES.test(text) ? `"${text.replace(/"/g, '""')}"` : text;
}

export function toCsv(rows: unknown[][]): string {
  return rows.map((row) => row.map(csvCell).join(",")).join("\r\n");
}
