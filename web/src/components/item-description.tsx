import { useEffect, useState } from "react";
import { EditorContent, useEditor, type JSONContent } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import type { Item, ItemUpdate } from "@/lib/items";

export function ItemDescription({ description, editable, busy, save }: {
  description: Item["description"];
  editable: boolean;
  busy: boolean;
  save: (update: ItemUpdate) => Promise<boolean>;
}) {
  const { t } = useTranslation();
  const [dirty, setDirty] = useState(false);
  const editor = useEditor({
    extensions: [StarterKit.configure({ link: { openOnClick: false } })],
    content: description ?? "",
    editable,
    editorProps: { attributes: { class: "min-h-24 rounded border p-2 focus-visible:outline-ring", "aria-label": t("items.description") } },
    onUpdate: () => setDirty(true),
  });
  useEffect(() => { editor?.setEditable(editable && !busy, false); }, [editor, editable, busy]);
  async function flush() {
    if (!editor || !editable || !dirty || busy) return;
    const description = editor.getJSON() as JSONContent & NonNullable<Item["description"]>;
    if (await save({ description })) setDirty(false);
  }
  return <section className="space-y-2" onBlur={(event) => {
    if (!event.currentTarget.contains(event.relatedTarget)) void flush();
  }}>
    <h3 className="font-medium">{t("items.description")}</h3>
    <EditorContent editor={editor} />
    {editable && dirty && <Button type="button" size="sm" variant="outline" disabled={busy} onClick={() => void flush()}>{t("items.save")}</Button>}
  </section>;
}
