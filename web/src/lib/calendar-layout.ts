// Tata letak kalender per minggu (C2): setiap item muncul sekali per minggu sebagai bar, bukan di setiap sel hari.
// Fungsi murni: bisa dites tanpa React. Tanggal dipakai sebagai hari (lihat calendar-days.ts), tanpa zona waktu.
// Helper kecil di sini (bukan impor) agar bisa dites dengan node --test tanpa resolusi alias.
const addDays = (date: Date, days: number) => new Date(date.getFullYear(), date.getMonth(), date.getDate() + days);
const daysBetween = (from: Date, to: Date) => Math.round((to.getTime() - from.getTime()) / 86_400_000);

export const MAX_LANES = 3;

export type Bar = { id: string; startCol: number; endCol: number; lane: number; continuesBefore: boolean; continuesAfter: boolean };
export type WeekLayout = { bars: Bar[]; hidden: { id: string; startCol: number; endCol: number }[]; overflowByCol: number[] };

/**
 * Tata letak satu minggu (7 kolom) dari `weekStart`. Item diberi lane pertama yang bebas; yang melewati MAX_LANES
 * tidak digambar tetapi dihitung per kolom (`overflowByCol`) dan disimpan di `hidden`, supaya tetap bisa dibuka lewat "+N".
 * Urutan deterministik: mulai lebih dulu, lalu lebih panjang, lalu id.
 */
export function layoutWeek(items: { id: string; start: Date; end: Date }[], weekStart: Date): WeekLayout {
  const weekEnd = addDays(weekStart, 6);
  const segments = items
    .filter((item) => item.end >= weekStart && item.start <= weekEnd && item.end >= item.start)
    .map((item) => {
      const from = item.start < weekStart ? weekStart : item.start;
      const to = item.end > weekEnd ? weekEnd : item.end;
      return { id: item.id, startCol: daysBetween(weekStart, from), endCol: daysBetween(weekStart, to),
        continuesBefore: item.start < weekStart, continuesAfter: item.end > weekEnd };
    })
    .sort((a, b) => a.startCol - b.startCol || (b.endCol - b.startCol) - (a.endCol - a.startCol) || a.id.localeCompare(b.id));

  const laneEnds: number[][] = []; // per lane: columns already taken
  const bars: Bar[] = [];
  const hidden: WeekLayout["hidden"] = [];
  const overflowByCol = Array.from({ length: 7 }, () => 0);
  for (const segment of segments) {
    let lane = laneEnds.findIndex((taken) => taken.every((col) => col < segment.startCol || col > segment.endCol));
    if (lane === -1) lane = laneEnds.length;
    if (lane >= MAX_LANES) {
      hidden.push({ id: segment.id, startCol: segment.startCol, endCol: segment.endCol });
      for (let col = segment.startCol; col <= segment.endCol; col++) overflowByCol[col]++;
      continue;
    }
    if (!laneEnds[lane]) laneEnds[lane] = [];
    for (let col = segment.startCol; col <= segment.endCol; col++) laneEnds[lane].push(col);
    bars.push({ ...segment, lane });
  }
  return { bars, hidden, overflowByCol };
}
