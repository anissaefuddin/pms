// Export presets (EXP-10). They are kept in this browser's localStorage, one list per signed-in user, Space, and view, so
// a preset never shows in another person's account, another workspace, or another view. Storage can be blocked or full; the
// dialog then works without presets rather than failing.

export type ExportPreset = { name: string; scope: string; columns: string[] };

export function presetKey(userId: string, spaceId: string, view: string) {
  return `lens.excel.exportPresets:v1:${userId}:${spaceId}:${view}`;
}

export function loadPresets(key: string): ExportPreset[] {
  try {
    const parsed: unknown = JSON.parse(localStorage.getItem(key) ?? "[]");
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((entry): entry is ExportPreset => typeof entry?.name === "string" && typeof entry?.scope === "string"
      && Array.isArray(entry?.columns));
  } catch {
    return [];
  }
}

function store(key: string, presets: ExportPreset[]) {
  try {
    localStorage.setItem(key, JSON.stringify(presets));
    return true;
  } catch {
    return false;
  }
}

// upsertPreset saves a preset under its name, replacing one with the same name (update) or adding it (save).
export function upsertPreset(key: string, preset: ExportPreset): ExportPreset[] {
  const next = [...loadPresets(key).filter((entry) => entry.name !== preset.name), preset];
  store(key, next);
  return next;
}

export function removePreset(key: string, name: string): ExportPreset[] {
  const next = loadPresets(key).filter((entry) => entry.name !== name);
  store(key, next);
  return next;
}
