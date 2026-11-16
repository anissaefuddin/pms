// Keadaan sesi yang dihapus saat sign-out (WA-D06). Preferensi tersimpan (tema, sidebar, view, kolom, expansi) tidak disentuh.

export const lastSpaceKey = (userId: string) => `pms:last-space:${userId}`;

/** Hapus Space terakhir milik pengguna ini; pengguna lain di perangkat yang sama tidak terpengaruh. */
export function clearSessionState(storage: Pick<Storage, "removeItem"> | null | undefined, userId: string) {
  try {
    storage?.removeItem(lastSpaceKey(userId));
  } catch {
    // penyimpanan diblokir: tidak ada yang bisa dihapus
  }
}
