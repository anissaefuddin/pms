import { useEffect, useId, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { IMPORTANCE, validateAnswers, visibleKeys, type Answers, type Files, type FormDefinition, type FormElement } from "@/lib/forms";
import { ItemUserPicker, type PersonOption } from "@/components/item-user-picker";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

type Props = {
  definition: FormDefinition;
  answers: Answers;
  onAnswersChange: (answers: Answers) => void;
  /** Preview mode: submit is rendered but disabled. */
  preview?: boolean;
  onSubmit?: (files: Files) => void;
  submitting?: boolean;
  /** Server-side per-question messages by key; each new object is shown, jumped to and focused. */
  serverErrors?: Record<string, string>;
  /** Heading level of page titles (sections are one deeper): 2 under an h1, 3 inside the builder's preview. */
  headingLevel?: 2 | 3;
};

const field = "block w-full rounded-lg border border-input bg-transparent px-2.5 py-1.5 text-sm dark:bg-input/30";

/** Renders a form definition as a requester sees it: one page at a time, Back/Next, Submit on the last page. */
export function FormRenderer({ definition, answers, onAnswersChange, preview = false, onSubmit, submitting, serverErrors, headingLevel = 3 }: Props) {
  const { t } = useTranslation();
  const [index, setIndex] = useState(0);
  const [people, setPeople] = useState<Record<string, PersonOption[]>>({});
  const [files, setFiles] = useState<Files>({});
  const [errors, setErrors] = useState<Record<string, string>>({});
  const summary = useRef<HTMLDivElement>(null);
  const [focusSummary, setFocusSummary] = useState(0);
  const pages = definition.pages;
  const at = Math.min(index, pages.length - 1);
  const page = pages[at];
  const shown = visibleKeys(definition, answers);
  const elements = pages.flatMap((p) => p.elements).filter((e) => shown.has(e.key));

  function show(next: Record<string, string>) {
    setErrors(next);
    const first = elements.find((e) => e.key in next);
    if (first) setIndex(pages.findIndex((p) => p.elements.includes(first)));
    setFocusSummary((n) => n + 1);
  }
  useEffect(() => { if (serverErrors && Object.keys(serverErrors).length) show(serverErrors); }, [serverErrors]);
  useEffect(() => { if (focusSummary) summary.current?.focus(); }, [focusSummary]);

  if (!page) return null;
  const clear = (key: string) => setErrors(({ [key]: _, ...rest }) => rest);
  const set = (key: string, value: string | string[]) => { clear(key); onAnswersChange({ ...answers, [key]: value }); };
  const setKeyFiles = (key: string, list: File[]) => { clear(key); setFiles({ ...files, [key]: list }); };

  function submit() {
    if (preview) return;
    const found = validateAnswers(definition, answers, files);
    const next = Object.fromEntries(Object.entries(found).map(([key, p]) =>
      [key, t(`forms.render.error.${p.code}`, { limit: p.limit })]));
    if (Object.keys(next).length) return show(next);
    setErrors({});
    onSubmit?.(Object.fromEntries(Object.entries(files).filter(([key]) => shown.has(key)))); // files of hidden questions are never sent
  }
  const problems = elements.filter((e) => e.key in errors);

  return <form className="space-y-5" noValidate onSubmit={(event) => { event.preventDefault(); submit(); }}>
    {problems.length > 0 && <div ref={summary} tabIndex={-1} role="alert" className="rounded-lg border border-destructive p-3 text-sm">
      <p className="font-medium">{t("forms.render.errors", { count: problems.length })}</p>
      <ul className="list-disc pl-5">{problems.map((e) => <li key={e.key}>{e.label}: {errors[e.key]}</li>)}</ul>
    </div>}
    {pages.length > 1 && <p className="text-sm text-muted-foreground" aria-live="polite">{t("forms.render.page", { current: at + 1, total: pages.length })}</p>}
    {page.title && <Heading level={headingLevel}>{page.title}</Heading>}
    {page.elements.filter((element) => shown.has(element.key)).map((element) => <Question key={element.key} element={element} value={answers[element.key]}
      onChange={(value) => set(element.key, value)} level={headingLevel + 1} people={people[element.key] ?? []} error={errors[element.key]}
      files={files[element.key] ?? []} onFiles={(list) => setKeyFiles(element.key, list)}
      onPeople={(next) => { setPeople({ ...people, [element.key]: next }); set(element.key, next.map((p) => p.id)); }} />)}
    <div className="flex items-center gap-2 border-t pt-4">
      {at > 0 && <Button type="button" variant="outline" onClick={() => setIndex(at - 1)}>{t("forms.render.back")}</Button>}
      {at < pages.length - 1
        ? <Button key="next" type="button" onClick={() => setIndex(at + 1)}>{t("forms.render.next")}</Button>
        : <Button key="submit" type="submit" disabled={preview || submitting}>{t("forms.render.submit")}</Button>}
      {preview && <span className="text-sm text-muted-foreground">{t("forms.render.previewNote")}</span>}
    </div>
  </form>;
}

function Heading({ level, children }: { level: number; children: React.ReactNode }) {
  const Tag = `h${level}` as "h2";
  return <Tag className="text-base font-semibold">{children}</Tag>;
}

function Question({ level, element, value, onChange, people, onPeople, error, files, onFiles }: {
  element: FormElement; value: string | string[] | undefined; onChange: (value: string | string[]) => void;
  level: number; people: PersonOption[]; onPeople: (people: PersonOption[]) => void; error?: string; files: File[]; onFiles: (files: File[]) => void;
}) {
  const { t } = useTranslation();
  const id = useId();
  const text = typeof value === "string" ? value : "";
  const list = Array.isArray(value) ? value : [];
  const required = element.required && element.kind !== "section";
  const label = <>{element.label}{required && <><span aria-hidden="true"> *</span><span className="sr-only"> {t("forms.render.required")}</span></>}</>;
  const help = element.help ? <p id={`${id}-help`} className="text-sm text-muted-foreground">{element.help}</p> : null;
  const errorId = `${id}-error`;
  const describedBy = [element.help && `${id}-help`, error && errorId].filter(Boolean).join(" ") || undefined;
  const invalid = error ? true : undefined;
  const message = error ? <p id={errorId} className="text-sm font-medium text-destructive"><span className="sr-only">{t("forms.render.errorPrefix")} </span>{error}</p> : null;

  if (element.kind === "section") return <div className="border-t pt-4"><Heading level={level}>{element.label}</Heading>{help}</div>;
  if (element.kind === "single_choice" || element.kind === "multiple_choice") {
    const multi = element.kind === "multiple_choice";
    return <fieldset className="space-y-1" aria-describedby={describedBy}>
      <legend className="font-medium">{label}</legend>{help}
      {(element.options ?? []).map((option) => <label key={option} className="flex items-center gap-2">
        <input type={multi ? "checkbox" : "radio"} name={id} className="size-6" checked={multi ? list.includes(option) : text === option}
          onChange={(event) => onChange(multi ? (event.target.checked ? [...list, option] : list.filter((o) => o !== option)) : option)} />
        {option}</label>)}
      {message}
    </fieldset>;
  }
  if (element.kind === "assignee") return <div className="space-y-1">
    <ItemUserPicker label={String(element.label)} selected={people} multiple={!!element.multiple} onChange={onPeople} />{help}{message}</div>;
  return <div className="space-y-1">
    <label htmlFor={id} className="block font-medium">{label}</label>{help}
    {element.kind === "single_line" && <Input aria-invalid={invalid} id={id} value={text} aria-describedby={describedBy} onChange={(e) => onChange(e.target.value)} />}
    {element.kind === "paragraph" && <textarea aria-invalid={invalid} id={id} rows={4} className={field} value={text} aria-describedby={describedBy} onChange={(e) => onChange(e.target.value)} />}
    {element.kind === "date" && <Input aria-invalid={invalid} id={id} type="date" value={text} aria-describedby={describedBy} onChange={(e) => onChange(e.target.value)} />}
    {element.kind === "number" && <Input aria-invalid={invalid} id={id} type="number" min={element.min} max={element.max} value={text} aria-describedby={describedBy} onChange={(e) => onChange(e.target.value)} />}
    {element.kind === "importance" && <select aria-invalid={invalid} id={id} className={field} value={text} aria-describedby={describedBy} onChange={(e) => onChange(e.target.value)}>
      <option value="">{t("forms.render.choose")}</option>
      {IMPORTANCE.map((level) => <option key={level} value={level}>{t(`forms.importance.${level}`)}</option>)}</select>}
    {element.kind === "attachment" && <Attachment id={id} describedBy={describedBy} max={element.max_files ?? 1} files={files} onFiles={onFiles} invalid={invalid} />}
    {message}
  </div>;
}

function Attachment({ id, describedBy, max, files, onFiles, invalid }: {
  id: string; describedBy?: string; max: number; files: File[]; onFiles: (files: File[]) => void; invalid?: boolean;
}) {
  const { t } = useTranslation();
  return <>
    <input id={id} type="file" multiple={max > 1} aria-describedby={describedBy} aria-invalid={invalid} disabled={files.length >= max} className="block text-sm"
      onChange={(e) => { onFiles([...files, ...Array.from(e.target.files ?? [])].slice(0, max)); e.target.value = ""; }} />
    <p className="text-sm text-muted-foreground">{t("forms.render.maxFiles", { count: max })}</p>
    <ul>{files.map((file, i) => <li key={`${file.name}-${i}`} className="flex items-center gap-2 text-sm">
      <span className="break-all">{file.name}</span>
      <Button type="button" variant="ghost" size="sm" aria-label={t("forms.render.removeFile", { name: file.name })}
        onClick={() => onFiles(files.filter((_, j) => j !== i))}>{t("forms.render.remove")}</Button></li>)}</ul>
  </>;
}
