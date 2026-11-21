# LENS — Mapping Konsolidasi Create (Batch 2)

Status: **mapping sebelum edit.** Belum ada perubahan komponen. Keputusan dari persetujuan Batch 2 berlaku.

## Enam entry point saat ini → target

| # | Entry point (sekarang) | Label / selector sekarang | Lokasi kode | Target | Perubahan |
|---|---|---|---|---|---|
| 1 | Global `+ Create` (topbar) | "Create" | `components/topbar.tsx`, `create-menu-dialog.tsx` | Hanya pada halaman **tanpa** container aktif | Sembunyikan pada Space, Folder, Project, dan Task detail |
| 2 | Header Space "New item" | `items.new` = "New item" | `routes/_app/spaces.$spaceId.tsx` | Primary **"+ New"** dengan dropdown child yang diizinkan | Label dan isi dropdown mengikuti `allowed-children`, Custom Item Types, dan capability `create` |
| 3 | Sidebar Space `+` | `nav.space.createIn` ("Create in {name}") | `components/sidebar.tsx` | Shortcut kontekstual, tetap | Tidak berubah |
| 4 | Header Project "Add task" | `views.addTask` = "Add task" | `views-*`, halaman project | Primary **"+ New Task"** | Label diganti "New task" |
| 5 | Footer Table "+ Add item" | `views.add` = "Add item" | `components/view-workspace.tsx` | Quick inline entry, sekunder | Tidak berubah (label tetap) |
| 6 | Empty state "New item" / "New project" | `spaces.empty.newProject`, `items.new` | halaman kosong Space dan view | Tindakan kontekstual tanpa primary yang bersaing | Jadi tombol sekunder (outline) atau tautan, bukan primary kedua |

Tambahan, tidak masuk daftar enam:
- **Task detail "Add subitem"** (`items.addChild`) → **"Add subtask"** sesuai keputusan 4.
- **View tabs "+"** tetap Add View, bukan Create Item.
- **Sidebar SPACES "+ New Space"** tetap untuk membuat Space.

## File E2E yang bergantung pada label (14)

| File | Selector yang terdampak |
|---|---|
| `create-menu.spec.ts`, `create-menu-live.spec.ts`, `dr03-create-closes.spec.ts` | Global "Create", dialog create |
| `spaces.spec.ts`, `space-keyboard-focus.spec.ts` | "New item" di header Space |
| `views.spec.ts`, `items.spec.ts` | "Add task", "Add item" |
| `creation-context.spec.ts` | Konteks parent otomatis |
| `blueprints.spec.ts`, `blueprint-editor.spec.ts` | "New item" di blueprint |
| `pilot-live.spec.ts`, `m8-live.spec.ts`, `uip2-b.spec.ts`, `dev-lens-findings.spec.ts` | Kombinasi label di atas |

## Selector yang akan dipakai
- Role dan nama aksesibel (`getByRole("button", { name })`) untuk aksi yang berubah.
- Test ID stabil (`data-testid="space-create-primary"`, `project-create-primary`) hanya bila nama tombol ambigu, misalnya saat "+ New" dipakai di beberapa konteks.

## Aturan yang harus dijaga
- Parent ID otomatis mengikuti konteks. Global Create wajib memilih Space/Project bila belum ada.
- Child yang ditawarkan hanya yang lolos `allowed-children`, Custom Item Types, dan capability `create`.
- Tidak ada perubahan RBAC atau endpoint backend.

## Uji yang harus lulus sesudah edit
- Unit: `eslint`, `tsc`, `npm test`.
- E2E: 14 file di atas, plus `space-actions.spec.ts` dan `space-settings.spec.ts`.
- Peran: editor, limited, viewer, external, dan admin tanpa implicit Space grant.
- Konteks: Workspace, Space, Folder, Project, Task, Personal Space, dan Custom Item Types.
