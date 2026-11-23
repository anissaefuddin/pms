// Tata letak kalender per minggu (C2): setiap item muncul sekali per minggu sebagai bar, bukan di setiap sel hari.
// Fungsi murni: bisa dites tanpa React. Tanggal dipakai sebagai hari (lihat calendar-days.ts), tanpa zona waktu.
// Helper kecil di sini (bukan impor) agar bisa dites dengan node --test tanpa resolusi alias.
const addDays = (date: Date, days: number) => new Date(date.getFullYear(), date.getMonth(), date.getDate() + days);

export const MAX_LANES = 3;

export type Bar = { id: string; startCol: number; endCol: number; lane: number; continuesBefore: boolean; continuesAfter: boolean };
export type WeekLayout = { bars: Bar[]; hidden: Omit<Bar, "lane">[]; overflowByCol: number[] };

export type CalendarEntry = { id: string; itemId: string; start: Date; end: Date };

/** Month view defaults to meaningful boundaries instead of repeating a months-long item every week. */
export function calendarEntries(items: { id: string; start: Date; end: Date }[], boundariesOnly: boolean): CalendarEntry[] {
  if (!boundariesOnly) return items.map((item) => ({ ...item, itemId: item.id }));
  return items.flatMap((item) => item.start.getTime() === item.end.getTime()
    ? [{ ...item, itemId: item.id }]
    : [
        { id: `${item.id}:start`, itemId: item.id, start: item.start, end: item.start },
        { id: `${item.id}:end`, itemId: item.id, start: item.end, end: item.end },
      ]);
}

/**
 * Tata letak satu minggu (7 kolom) dari `weekStart`. Item diberi lane pertama yang bebas; yang melewati MAX_LANES
 * tidak digambar tetapi dihitung per kolom (`overflowByCol`) dan disimpan di `hidden`, supaya tetap bisa dibuka lewat "+N".
 * Urutan deterministik: mulai lebih dulu, lalu lebih panjang, lalu id.
 */
export function layoutDays(items: { id: string; start: Date; end: Date }[], days: Date[], maxLanes = MAX_LANES): WeekLayout {
  const weekStart = days[0], weekEnd = days[days.length - 1];
  const segments = items
    .filter((item) => item.end >= weekStart && item.start <= weekEnd && item.end >= item.start)
    .map((item) => {
      const startCol = days.findIndex((day) => day >= item.start);
      let endCol = -1;
      for (let index = days.length - 1; index >= 0; index--) if (days[index] <= item.end) { endCol = index; break; }
      return { id: item.id, startCol, endCol,
        continuesBefore: item.start < weekStart, continuesAfter: item.end > weekEnd };
    })
    .filter((item) => item.startCol >= 0 && item.endCol >= item.startCol)
    .sort((a, b) => a.startCol - b.startCol || (b.endCol - b.startCol) - (a.endCol - a.startCol) || a.id.localeCompare(b.id));

  const laneEnds: number[][] = []; // per lane: columns already taken
  const bars: Bar[] = [];
  const hidden: WeekLayout["hidden"] = [];
  const overflowByCol = Array.from({ length: days.length }, () => 0);
  for (const segment of segments) {
    let lane = laneEnds.findIndex((taken) => taken.every((col) => col < segment.startCol || col > segment.endCol));
    if (lane === -1) lane = laneEnds.length;
    if (lane >= maxLanes) {
      hidden.push(segment);
      for (let col = segment.startCol; col <= segment.endCol; col++) overflowByCol[col]++;
      continue;
    }
    if (!laneEnds[lane]) laneEnds[lane] = [];
    for (let col = segment.startCol; col <= segment.endCol; col++) laneEnds[lane].push(col);
    bars.push({ ...segment, lane });
  }
  return { bars, hidden, overflowByCol };
}

export function layoutWeek(items: { id: string; start: Date; end: Date }[], weekStart: Date, maxLanes = MAX_LANES): WeekLayout {
  return layoutDays(items, Array.from({ length: 7 }, (_, index) => addDays(weekStart, index)), maxLanes);
}
