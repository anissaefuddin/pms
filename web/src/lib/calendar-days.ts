/** Dates are calendar days (YYYY-MM-DD) held as local midnights, so no time zone can shift a day. */
export const toDay = (value?: string | null) => value ? new Date(+value.slice(0, 4), +value.slice(5, 7) - 1, +value.slice(8, 10)) : undefined;
export const fromDay = (date: Date) => `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
export const addDays = (date: Date, days: number) => new Date(date.getFullYear(), date.getMonth(), date.getDate() + days);
export const daysBetween = (from: Date, to: Date) => Math.round((to.getTime() - from.getTime()) / 86_400_000);
