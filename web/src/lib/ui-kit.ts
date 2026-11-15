// Logika murni untuk komponen bersama di components/ui/ (tanpa import agar bisa dites dengan node --test).

export const CAT_TOKENS = ["cat-1", "cat-2", "cat-3", "cat-4", "cat-5", "cat-6"] as const;
export type CatToken = (typeof CAT_TOKENS)[number];

export function isCatToken(color?: string): color is CatToken {
  return (CAT_TOKENS as readonly string[]).includes(color ?? "");
}

/** Inisial nama: dua huruf pertama bila satu kata, selain itu huruf pertama dari dua kata pertama. */
export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return parts.slice(0, 2).map((p) => p[0].toUpperCase()).join("");
}

/** Indeks berikutnya untuk tombol panah/Home/End; null bila tombol bukan navigasi. Membungkus di ujung. */
export function nextIndex(current: number, count: number, key: string): number | null {
  if (count <= 0) return null;
  switch (key) {
    case "ArrowRight":
    case "ArrowDown":
      return (current + 1) % count;
    case "ArrowLeft":
    case "ArrowUp":
      return (current - 1 + count) % count;
    case "Home":
      return 0;
    case "End":
      return count - 1;
    default:
      return null;
  }
}

/** Aksi utama hanya ada bila pengguna berhak membuat; selain itu tidak dirender. */
export function primaryActionFor<T>(canCreate: boolean, action: T | undefined): T | undefined {
  return canCreate ? action : undefined;
}
