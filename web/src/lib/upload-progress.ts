/** Persentase dari progres yang bisa dihitung; 0 bila total belum diketahui. Tidak pernah melebihi 100. Murni, tanpa impor. */
export function progressPercent(loaded: number, total: number): number {
  if (!(total > 0)) return 0;
  return Math.min(100, Math.round((loaded / total) * 100));
}
