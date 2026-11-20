import { Check, Minus } from "lucide-react";
import { useTranslation } from "react-i18next";

export type MatrixColumn = { id: string; label: string };
export type MatrixRow = { id: string; label: string; help?: string };

/**
 * Capability matrix (rows: capabilities, columns: user types or roles). Each cell is text plus an icon,
 * so the state never depends on colour alone.
 */
export function PermissionMatrix({ caption, rows, columns, allowed }: {
  caption: string; rows: MatrixRow[]; columns: MatrixColumn[]; allowed: (column: string, row: string) => boolean;
}) {
  const { t } = useTranslation();
  return <div className="overflow-x-auto rounded-lg border">
    <table className="w-full min-w-max border-collapse text-left text-sm">
      <caption className="sr-only">{caption}</caption>
      <thead className="bg-muted/50">
        <tr>
          <th scope="col" className="sticky left-0 bg-muted/50 px-3 py-2 font-medium">{t("settings.matrix.capability")}</th>
          {columns.map((column) => <th key={column.id} scope="col" className="px-3 py-2 text-center font-medium">{column.label}</th>)}
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => <tr key={row.id} className="border-t">
          <th scope="row" className="sticky left-0 bg-card px-3 py-2 text-left font-normal">
            <span className="block">{row.label}</span>
            {row.help && <span className="block text-xs text-muted-foreground">{row.help}</span>}
          </th>
          {columns.map((column) => {
            const yes = allowed(column.id, row.id);
            return <td key={column.id} className="px-3 py-2 text-center">
              <span className="inline-flex items-center gap-1 text-xs">
                {yes ? <Check aria-hidden="true" className="size-4 text-success" /> : <Minus aria-hidden="true" className="size-4 text-muted-foreground" />}
                <span>{yes ? t("settings.matrix.allowed") : t("settings.matrix.notAllowed")}</span>
              </span>
            </td>;
          })}
        </tr>)}
      </tbody>
    </table>
  </div>;
}
