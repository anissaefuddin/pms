import { useState, type FormEvent } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { ApiError, api } from "@/lib/api";
import type { CustomField } from "@/lib/custom-fields";
import { itemKeys } from "@/lib/items";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogTitle } from "@/components/ui/dialog";
import { FormError } from "@/components/auth/form-error";
import { DestinationTree } from "@/components/excel-destination-tree";

const templateKinds = ["basic", "hierarchy", "advanced"] as const;

type Preview = {
  columns: string[];
  rows: number;
  errors: { row: number; column: string; message: string }[];
  digest: string;
  sample?: string[][];
};

const itemColumns = ["title", "type", "id", "parent_id", "start_date", "due_date", "duration_days", "is_milestone"] as const;

export function SpaceExcelImport({ spaceId, open, onOpenChange }: {
  spaceId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  return <Dialog open={open} onOpenChange={(next) => { if (!busy) onOpenChange(next); }}>
    <DialogContent showCloseButton={!busy} className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl">
      <DialogTitle>{t("excel.importTitle")}</DialogTitle>
      <DialogDescription>{t("excel.importDescription")}</DialogDescription>
      {open && <ImportForm spaceId={spaceId} busy={busy} setBusy={setBusy} onClose={() => onOpenChange(false)} />}
    </DialogContent>
  </Dialog>;
}

function ImportForm({ spaceId, busy, setBusy, onClose }: {
  spaceId: string;
  busy: boolean;
  setBusy: (busy: boolean) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const { data: fieldPage } = useQuery({
    queryKey: ["custom-fields", spaceId],
    queryFn: ({ signal }) => api<{ items: CustomField[] }>(`/api/v1/spaces/${spaceId}/custom-fields`, { signal }),
  });
  const [file, setFile] = useState<File>();
  const [columns, setColumns] = useState<string[]>([]);
  const [mapping, setMapping] = useState<Record<string, string>>({});
  const [validated, setValidated] = useState<{ preview: Preview; mapped?: string }>();
  const [destination, setDestination] = useState("");
  const [step, setStep] = useState(0);
  const [error, setError] = useState("");
  const preview = validated?.preview;
  const fieldChoices = (fieldPage?.items ?? []).filter((field) => !field.archived_at);
  const targets = [
    ...itemColumns.map((id) => ({ id, label: t(`excel.column.${id}`) })),
    ...fieldChoices.map((field) => ({ id: `field:${field.id}`, label: field.name })),
  ];
  function targetFor(column: string) {
    const normalized = column.trim().toLowerCase();
    return mapping[column] ?? (targets.some((target) => target.id === normalized) ? normalized : undefined);
  }

  async function submit(commit: boolean) {
    if (!file || busy || (commit && (!preview?.digest || preview.errors.length))) return;
    setError("");
    setBusy(true);
    const data = new FormData();
    data.set("file", file);
    const mapped = commit ? validated?.mapped : Object.keys(mapping).length
      ? JSON.stringify(Object.fromEntries(columns.map((column) => [column, targetFor(column)]))) : undefined;
    if (mapped) data.set("mapping", mapped);
    if (destination) data.set("destination", destination);
    if (commit && preview) {
      data.set("commit", "true");
      data.set("digest", preview.digest);
    }
    try {
      const path = `/api/v1/spaces/${spaceId}/excel-imports`;
      if (commit) {
        const result = await api<{ item_ids: string[] }>(path, { method: "POST", body: data });
        void qc.invalidateQueries({ queryKey: itemKeys.all });
        void qc.invalidateQueries({ queryKey: ["search"] });
        toast.success(t("excel.imported", { count: result.item_ids.length }));
        onClose();
      } else {
        const result = await api<Preview>(path, { method: "POST", body: data });
        setColumns(result.columns);
        setValidated({ preview: { ...result, errors: result.errors ?? [] }, mapped });
      }
    } catch (err) {
      if (commit && err instanceof ApiError && err.fields.length && validated) {
        setValidated({ ...validated, preview: { ...validated.preview, errors: err.fields.map(({ field, message }) => {
          const [, row, column = ""] = /^rows\.(\d+)(?:\.(.*))?$/.exec(field) ?? [];
          return { row: Number(row ?? 1), column, message };
        }) } });
      } else setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  const steps = ["upload", "map", "validate", "import"] as const;
  const names = steps.map((name) => t(`excel.wizard.${name}`));
  // Next runs the server preview: step 1 reads the columns, step 2 checks the mapping, and the import needs a clean preview.
  const canNext = !busy && (step === 0 ? !!file : step === 1 ? !!columns.length : step === 2 ? !!preview?.digest && !preview.errors.length : true);
  function next() {
    if (step === 0 || step === 1) void submit(false).then(() => setStep((current) => current + 1));
    else setStep((current) => current + 1);
  }

  return <form className="space-y-4" onSubmit={(event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (canNext && step < 3) next();
  }}>
    <div className="space-y-2">
      <p className="text-sm font-medium" aria-live="polite">{t("excel.wizard.stepOf", { step: step + 1, total: steps.length, name: names[step] })}</p>
      <ol className="grid grid-cols-4 gap-1 text-xs">
        {names.map((name, index) => <li key={name} aria-current={index === step ? "step" : undefined}
          className={`border-t-4 pt-1 ${index <= step ? "border-primary" : "border-muted"}`}>{name}</li>)}
      </ol>
    </div>
    {step === 0 && <div className="space-y-4">
      <div className="flex flex-wrap gap-2 text-sm">
        <span>{t("excel.templates")}</span>
        {templateKinds.map((kind) => <a key={kind} className="underline" download
          href={`/api/v1/spaces/${spaceId}/excel-templates/${kind}`}>{t(`excel.template.${kind}`)}</a>)}
      </div>
      <label className="block space-y-1">
        <span>{t("excel.workbook")}</span>
        <Input type="file" accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" disabled={busy}
          onChange={(event) => {
            setFile(event.target.files?.[0]);
            setStep(0);
            setColumns([]);
            setMapping({});
            setValidated(undefined);
            setError("");
          }} />
      </label>
      <DestinationTree spaceId={spaceId} value={destination} disabled={busy}
        onChange={(id) => { setDestination(id); setValidated(undefined); }} />
    </div>}
    {step === 1 && !!columns.length && <fieldset className="grid gap-2" disabled={busy}>
      <legend className="mb-2 font-medium">{t("excel.mapping")}</legend>
      {columns.map((column) => <label key={column} className="grid items-center gap-2 sm:grid-cols-2">
        <span className="break-all">{column}</span>
        <select aria-label={t("excel.mapColumn", { column })} className="min-w-0 rounded border bg-background px-2 py-1"
          value={targetFor(column) ?? "unmapped"} onChange={(event) => {
            setMapping({ ...mapping, [column]: event.target.value });
            setValidated(undefined);
          }}>
          <option value="unmapped" disabled>{t("excel.mapColumn", { column })}</option>
          <option value="">{t("excel.ignoreColumn")}</option>
          {targets.map(({ id, label }) => <option key={id} value={id}>{label}</option>)}
        </select>
      </label>)}
    </fieldset>}
    {step === 2 && preview && <div className="space-y-2">
      <p role="status">{t("excel.previewRows", { count: preview.rows })}</p>
      <p>{t("excel.validSummary", { valid: Math.max(0, preview.rows - preview.errors.length), errors: preview.errors.length })}</p>
      {!!preview.sample?.length && <div className="overflow-auto rounded border">
        <table aria-label={t("excel.sample")} className="w-full text-left">
          <thead><tr>{columns.map((column) => <th scope="col" className="p-2" key={column}>{column}</th>)}</tr></thead>
          <tbody>{preview.sample.map((row, index) => <tr key={index} className="border-t">
            {columns.map((column, cell) => <td className="max-w-44 break-words p-2" key={column}>{row[cell]}</td>)}
          </tr>)}</tbody>
        </table>
      </div>}
      {!!preview.errors.length && <ul aria-label={t("excel.validationErrors")} className="max-h-40 space-y-1 overflow-auto rounded border p-2">
        {preview.errors.map(({ row, column, message }, index) => <li key={index}>
          <strong>{t(column ? "excel.rowColumn" : "excel.row", { row, column })} </strong>{message}
        </li>)}
      </ul>}
    </div>}
    {step === 3 && preview && <dl className="grid grid-cols-2 gap-2 rounded border p-3 text-sm">
      <dt>{t("excel.review.rows")}</dt><dd>{preview.rows}</dd>
      <dt>{t("excel.review.mode")}</dt><dd>{t("excel.review.createOnly")}</dd>
      <dt>{t("excel.review.destination")}</dt><dd>{destination ? t("excel.review.chosen") : t("excel.spaceRoot")}</dd>
    </dl>}
    <FormError message={error} />
    <DialogFooter>
      <Button variant="outline" onClick={onClose} disabled={busy}>{t("excel.close")}</Button>
      {step > 0 && <Button type="button" variant="outline" disabled={busy} onClick={() => setStep((current) => current - 1)}>{t("excel.wizard.back")}</Button>}
      {step < 3 && <Button type="submit" disabled={!canNext}>{t("excel.wizard.next")}</Button>}
      {step === 3 && <Button type="button" disabled={busy || !preview?.digest || !!preview.errors.length} onClick={() => void submit(true)}>{t("excel.commitImport")}</Button>}
    </DialogFooter>
  </form>;
}
