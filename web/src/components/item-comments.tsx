import { useId, useRef, useState } from "react";
import { EditorContent, useEditor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import type { components } from "@/api/schema";
import { api } from "@/lib/api";
import type { Item } from "@/lib/items";
import { mention, MentionPicker } from "@/components/rich-text-mention";
import { Button } from "@/components/ui/button";
import { FormError } from "@/components/auth/form-error";
import { AvatarStack } from "@/components/ui/avatar-stack";
import { formatDateTime, useProfile } from "@/lib/profile";

type Comment = components["schemas"]["Comment"];
const extensions = [StarterKit.configure({ link: { openOnClick: false } }), mention];

export function ItemComments({ item, userId }: { item: Item; userId: string }) {
  const { t } = useTranslation();
  const profile = useProfile();
  const heading = useId();
  const qc = useQueryClient();
  const [cursor, setCursor] = useState<string>();
  const [editing, setEditing] = useState<Comment>();
  const [error, setError] = useState("");
  const path = `/api/v1/items/${item.id}/comments`;
  const key = ["item-comments", item.id];
  const page = useQuery({ queryKey: [...key, cursor], queryFn: () =>
    api<components["schemas"]["CommentPage"]>(path + (cursor ? `?cursor=${encodeURIComponent(cursor)}` : "")) });
  const canComment = item.my_capabilities.includes("comment");
  const full = item.my_capabilities.includes("share") && item.my_capabilities.includes("delete");
  async function refresh() { await qc.invalidateQueries({ queryKey: key }); }
  return <section className="space-y-3 border-t pt-4" aria-labelledby={heading}>
    <h3 id={heading} className="font-semibold">{t("items.comments")}</h3>
    {page.isError && <FormError message={t("items.loadError")} />}
    <FormError message={error} />
    {page.data?.items.map((comment) => {
      const editable = full || (canComment && comment.author.id === userId);
      return <article key={comment.id} className="space-y-2 rounded-lg bg-muted/40 p-3">
        <p className="flex items-center gap-2"><AvatarStack people={[comment.author]} />
          <span className="font-medium">{comment.author.name}</span>
          <time className="text-xs text-muted-foreground" dateTime={comment.created_at}>{formatDateTime(comment.created_at, profile)}</time></p>
        <CommentEditor key={editing?.id === comment.id ? "editing" : comment.version}
          body={editing?.id === comment.id ? editing.body : comment.body}
          editable={editing?.id === comment.id} save={async (body) => {
            if (editing?.id !== comment.id) return;
            await api(`/api/v1/comments/${comment.id}`, { method: "PATCH", headers: { "If-Match": `"${editing.version}"` }, body: JSON.stringify({ body }) });
            await refresh(); setEditing(undefined);
          }} />
        {editable && editing?.id !== comment.id && <div className="flex gap-2">
          <Button size="sm" variant="ghost" onClick={() => setEditing(comment)}>{t("items.editComment")}</Button>
          <Button size="sm" variant="ghost" onClick={async () => {
            try { await api(`/api/v1/comments/${comment.id}`, { method: "DELETE" }); await refresh(); }
            catch (err) { setError((err as Error).message); }
          }}>{t("items.deleteComment")}</Button>
        </div>}
        {editing?.id === comment.id && <Button variant="ghost" size="sm" onClick={() => setEditing(undefined)}>{t("items.cancel")}</Button>}
      </article>;
    })}
    {page.data?.next_cursor && <Button variant="outline" onClick={() => setCursor(page.data?.next_cursor ?? undefined)}>{t("items.olderComments")}</Button>}
    {cursor && <Button variant="ghost" onClick={() => setCursor(undefined)}>{t("items.latestComments")}</Button>}
    {canComment && <div className="sticky bottom-0 -mx-1 rounded-xl border bg-background p-2 shadow-sm"><CommentEditor editable save={async (body) => {
      await api(path, { method: "POST", body: JSON.stringify({ body }) });
      setCursor(undefined); await refresh();
    }} /></div>}
  </section>;
}

function CommentEditor({ body, editable, save }: { body?: Comment["body"]; editable: boolean; save: (body: unknown) => Promise<void> }) {
  const { t } = useTranslation();
  const shortcut = useProfile()?.comment_shortcut ?? "mod_enter";
  const saving = useRef(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const editor = useEditor({ extensions, content: body ?? "", editable,
    editorProps: { attributes: { class: editable ? "min-h-16 rounded border p-2" : "", "aria-label": t("items.comment") } } });
  async function submit() {
    if (!editor || editor.isDestroyed || saving.current) return;
    saving.current = true;
    setBusy(true); setError(""); editor.setEditable(false, false);
    try { await save(editor.getJSON()); if (!body && !editor.isDestroyed) editor.commands.clearContent(); }
    catch (err) { setError((err as Error).message); }
    finally {
      if (!editor.isDestroyed) editor.setEditable(true, false);
      setBusy(false);
      saving.current = false;
    }
  }
  return <div className="space-y-2">
    <div onKeyDownCapture={(event) => {
      if (!editable || event.key !== "Enter" || event.nativeEvent.isComposing || event.shiftKey || event.altKey) return;
      const modifier = event.metaKey || event.ctrlKey;
      if (shortcut === "mod_enter" && !modifier) return;
      event.preventDefault();
      event.stopPropagation();
      void submit();
    }}><EditorContent editor={editor} /></div>
    {editable && <>
      <MentionPicker editor={editor} busy={busy} />
      <FormError message={error} />
      <Button disabled={busy || !editor} onClick={() => void submit()}>{t(body ? "items.save" : "items.postComment")}</Button>
    </>}
  </div>;
}
