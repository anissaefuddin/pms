import { ArrowDown, ArrowUp, Plus, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { ELEMENT_KINDS, MAX_ELEMENTS, MAX_PAGES, allKeys, isChoice, moveAt, newElement, uniqueKey, type FormElement, type FormPage } from "@/lib/forms";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";

type Props = {
  pages: FormPage[];
  /** Keys already saved on the server: they never change, whatever the label says. */
  lockedKeys: Set<string>;
  errors: Record<string, string>;
  /** `rename` is set when a not-yet-saved element's key was re-derived from its label. */
  onChange: (pages: FormPage[], rename?: [from: string, to: string]) => void;
};

const area = "block w-full rounded-lg border border-input bg-transparent px-2.5 py-1.5 text-sm dark:bg-input/30";

export function FormQuestionsEditor({ pages, lockedKeys, errors, onChange }: Props) {
  const { t } = useTranslation();
  const total = pages.reduce((n, p) => n + p.elements.length, 0);
  const patchPage = (pi: number, page: FormPage) => onChange(pages.map((p, i) => i === pi ? page : p));

  function patchElement(pi: number, ei: number, patch: Partial<FormElement>) {
    const old = pages[pi].elements[ei];
    let element = { ...old, ...patch };
    let rename: [string, string] | undefined;
    if (patch.label !== undefined && !lockedKeys.has(old.key)) { // brand-new element: key follows the label until the first save
      const others = allKeys(pages).filter((k) => k !== old.key);
      const key = uniqueKey(patch.label || old.kind, others);
      if (key !== old.key) { element = { ...element, key }; rename = [old.key, key]; }
    }
    onChange(pages.map((p, i) => i === pi ? { ...p, elements: p.elements.map((e, j) => j === ei ? element : e) } : p), rename);
  }
  const addElement = (pi: number, kind: FormElement["kind"]) =>
    patchPage(pi, { ...pages[pi], elements: [...pages[pi].elements, newElement(kind, t(`forms.kind.${kind}`), allKeys(pages))] });

  return <section aria-labelledby="fq-title" className="space-y-4">
    <h2 id="fq-title" className="text-lg font-semibold">{t("forms.questions.title")}</h2>
    {pages.map((page, pi) => {
      const n = pi + 1;
      return <fieldset key={pi} className="space-y-3 rounded-xl border p-4">
        <legend className="px-1 font-medium">{t("forms.questions.page", { n })}</legend>
        <div className="flex flex-wrap items-end gap-2">
          <label className="min-w-48 flex-1 space-y-1 text-sm"><span className="block">{t("forms.questions.pageTitle")}</span>
            <Input value={page.title} maxLength={120} aria-invalid={!!errors[`p${pi}-title`]} onChange={(e) => patchPage(pi, { ...page, title: e.target.value })} /></label>
          <Button type="button" size="icon" variant="outline" aria-label={t("forms.questions.movePageUp", { n })} disabled={pi === 0} onClick={() => onChange(moveAt(pages, pi, -1))}><ArrowUp /></Button>
          <Button type="button" size="icon" variant="outline" aria-label={t("forms.questions.movePageDown", { n })} disabled={pi === pages.length - 1} onClick={() => onChange(moveAt(pages, pi, 1))}><ArrowDown /></Button>
          <Button type="button" size="icon" variant="outline" aria-label={t("forms.questions.removePage", { n })} disabled={pages.length === 1} onClick={() => onChange(pages.filter((_, i) => i !== pi))}><Trash2 /></Button>
        </div>
        <FormError message={errors[`p${pi}-title`] ?? ""} />
        {page.elements.length === 0 && <p className="text-sm text-muted-foreground">{t("forms.questions.noQuestions")}</p>}
        <ol className="space-y-3">
          {page.elements.map((element, ei) => <li key={ei}>
            <ElementCard element={element} pi={pi} ei={ei} count={page.elements.length} errors={errors}
              onPatch={(patch) => patchElement(pi, ei, patch)}
              onMove={(delta) => patchPage(pi, { ...page, elements: moveAt(page.elements, ei, delta) })}
              onRemove={() => patchPage(pi, { ...page, elements: page.elements.filter((_, j) => j !== ei) })} />
          </li>)}
        </ol>
        <DropdownMenu>
          <DropdownMenuTrigger render={<Button type="button" variant="outline" disabled={total >= MAX_ELEMENTS} aria-label={t("forms.questions.addToPage", { n })} />}><Plus />{t("forms.questions.add")}</DropdownMenuTrigger>
          <DropdownMenuContent align="start">
            {ELEMENT_KINDS.map((kind) => <DropdownMenuItem key={kind} onClick={() => addElement(pi, kind)}>{t(`forms.kind.${kind}`)}</DropdownMenuItem>)}
          </DropdownMenuContent>
        </DropdownMenu>
      </fieldset>;
    })}
    <Button type="button" variant="outline" disabled={pages.length >= MAX_PAGES} onClick={() => onChange([...pages, { title: "", elements: [] }])}><Plus />{t("forms.questions.addPage")}</Button>
  </section>;
}

function ElementCard({ element, pi, ei, count, errors, onPatch, onMove, onRemove }: {
  element: FormElement; pi: number; ei: number; count: number; errors: Record<string, string>;
  onPatch: (patch: Partial<FormElement>) => void; onMove: (delta: -1 | 1) => void; onRemove: () => void;
}) {
  const { t } = useTranslation();
  const err = (prop: string) => errors[`e${pi}-${ei}-${prop}`] ?? "";
  const name = element.label || t("forms.questions.untitled");
  const section = element.kind === "section";
  const options = element.options ?? [];
  const setOptions = (next: string[]) => onPatch({ options: next });
  const num = (value: string) => value === "" ? undefined : Number(value);

  return <div className="space-y-3 rounded-lg border bg-card p-3">
    <div className="flex flex-wrap items-center gap-2">
      <span className="rounded border px-2 py-0.5 text-xs font-medium">{t(`forms.kind.${element.kind}`)}</span>
      <span className="text-xs text-muted-foreground">{t("forms.questions.key", { key: element.key })}</span>
      <span className="flex-1" />
      <Button type="button" size="icon-sm" variant="outline" aria-label={t("forms.questions.moveUp", { label: name })} disabled={ei === 0} onClick={() => onMove(-1)}><ArrowUp /></Button>
      <Button type="button" size="icon-sm" variant="outline" aria-label={t("forms.questions.moveDown", { label: name })} disabled={ei === count - 1} onClick={() => onMove(1)}><ArrowDown /></Button>
      <Button type="button" size="icon-sm" variant="outline" aria-label={t("forms.questions.remove", { label: name })} onClick={onRemove}><Trash2 /></Button>
    </div>
    <label className="block space-y-1 text-sm"><span>{t(section ? "forms.questions.sectionLabel" : "forms.questions.label")}</span>
      <Input value={element.label} maxLength={200} aria-invalid={!!err("label")} onChange={(e) => onPatch({ label: e.target.value })} /></label>
    <FormError message={err("label")} />
    <label className="block space-y-1 text-sm"><span>{t(section ? "forms.questions.sectionHelp" : "forms.questions.help")}</span>
      <textarea className={area} rows={2} maxLength={500} value={element.help ?? ""} aria-invalid={!!err("help")} onChange={(e) => onPatch({ help: e.target.value })} /></label>
    <FormError message={err("help")} />
    {!section && <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={!!element.required} onChange={(e) => onPatch({ required: e.target.checked })} />{t("forms.questions.required")}</label>}
    {isChoice(element.kind) && <fieldset className="space-y-2">
      <legend className="text-sm font-medium">{t("forms.questions.options")}</legend>
      <ol className="space-y-2">
        {options.map((option, oi) => <li key={oi} className="flex items-center gap-2">
          <Input aria-label={t("forms.questions.option", { n: oi + 1 })} value={option} maxLength={200} aria-invalid={!!err("options")}
            onChange={(e) => setOptions(options.map((o, j) => j === oi ? e.target.value : o))} />
          <Button type="button" size="icon-sm" variant="outline" aria-label={t("forms.questions.moveOptionUp", { n: oi + 1 })} disabled={oi === 0} onClick={() => setOptions(moveAt(options, oi, -1))}><ArrowUp /></Button>
          <Button type="button" size="icon-sm" variant="outline" aria-label={t("forms.questions.moveOptionDown", { n: oi + 1 })} disabled={oi === options.length - 1} onClick={() => setOptions(moveAt(options, oi, 1))}><ArrowDown /></Button>
          <Button type="button" size="icon-sm" variant="outline" aria-label={t("forms.questions.removeOption", { n: oi + 1 })} onClick={() => setOptions(options.filter((_, j) => j !== oi))}><Trash2 /></Button>
        </li>)}
      </ol>
      <FormError message={err("options")} />
      <Button type="button" size="sm" variant="outline" disabled={options.length >= 50} onClick={() => setOptions([...options, ""])}><Plus />{t("forms.questions.addOption")}</Button>
    </fieldset>}
    {element.kind === "number" && <div className="grid gap-3 sm:grid-cols-2">
      <label className="block space-y-1 text-sm"><span>{t("forms.questions.min")}</span>
        <Input type="number" value={element.min ?? ""} aria-invalid={!!err("min")} onChange={(e) => onPatch({ min: num(e.target.value) })} /></label>
      <label className="block space-y-1 text-sm"><span>{t("forms.questions.max")}</span>
        <Input type="number" value={element.max ?? ""} aria-invalid={!!err("max")} onChange={(e) => onPatch({ max: num(e.target.value) })} /></label>
      <FormError message={err("min")} /><FormError message={err("max")} />
    </div>}
    {element.kind === "attachment" && <div><label className="block space-y-1 text-sm"><span>{t("forms.questions.maxFiles")}</span>
      <Input type="number" min={1} max={5} value={element.max_files ?? ""} aria-invalid={!!err("max_files")} onChange={(e) => onPatch({ max_files: num(e.target.value) })} /></label>
      <FormError message={err("max_files")} /></div>}
    {element.kind === "assignee" && <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={!!element.multiple} onChange={(e) => onPatch({ multiple: e.target.checked })} />{t("forms.questions.multiple")}</label>}
  </div>;
}
