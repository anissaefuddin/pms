import { createFileRoute } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { AdminPageHeader, AdminStatus } from "@/components/admin/admin-ui";

export const Route = createFileRoute("/_app/settings/rates")({ component: Rates });

const RATE_TONE = { current: "success", scheduled: "warning", historical: "neutral" } as const;

/**
 * Per person or job role: the latest rate whose start date has come is Current, later ones are Scheduled,
 * and everything before the current one is Historical. Logged time keeps the rate it was logged with.
 */
function rateStatuses(items: Rate[]) {
  const today = new Date().toISOString().slice(0, 10);
  const result = new Map<string, "current" | "scheduled" | "historical">();
  const bySubject = new Map<string, Rate[]>();
  for (const rate of items) {
    const key = rate.user_id ?? rate.role_id ?? rate.id;
    bySubject.set(key, [...(bySubject.get(key) ?? []), rate]);
  }
  for (const group of bySubject.values()) {
    const sorted = [...group].sort((a, b) => b.effective_from.localeCompare(a.effective_from));
    const currentIndex = sorted.findIndex((rate) => rate.effective_from <= today);
    sorted.forEach((rate, index) => {
      if (rate.effective_from > today) result.set(rate.id, "scheduled");
      else result.set(rate.id, index === currentIndex ? "current" : "historical");
    });
  }
  return result;
}
type Choice = { id: string; name: string };
type Rate = { id: string; name: string; user_id: string | null; role_id: string | null; rate_minor: number; effective_from: string };

function Rates() {
  const { t, i18n } = useTranslation();
  const me = useProfile();
  const cache = useQueryClient();
  const admin = me?.user_type === "admin";
  const [subject, setSubject] = useState("");
  const [adding, setAdding] = useState(false);
  const rates = useQuery({ queryKey: ["rates"], enabled: admin, queryFn: () => api<{ items: Rate[] }>("/api/v1/rates") });
  const directory = useQuery({ queryKey: ["resource-directory"], enabled: admin,
    queryFn: () => api<{ roles: Choice[]; users: Choice[] }>("/api/v1/resource-directory") });
  const add = useMutation({
    mutationFn: (body: object) => api("/api/v1/rates", { method: "POST", body: JSON.stringify(body) }),
    onSuccess: () => cache.invalidateQueries({ queryKey: ["rates"] }),
  });
  const money = new Intl.NumberFormat(i18n.language, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  if (!admin) return null;
  const statuses = rateStatuses(rates.data?.items ?? []);
  return <section className="space-y-5">
    <AdminPageHeader description={t("time.rates.hint")} action={<Button onClick={() => setAdding(true)}>{t("time.rates.add")}</Button>} />
    {(add.error || rates.error) && <p role="alert">{(add.error ?? rates.error)?.message}</p>}
    {/* Adding a rate is a dialog from the header, so the list stays the main content. */}
    <Dialog open={adding} onOpenChange={setAdding}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("time.rates.add")}</DialogTitle>
          <DialogDescription>{t("time.rates.hint")}</DialogDescription>
        </DialogHeader>
        <form className="space-y-4" onSubmit={(event) => {
          event.preventDefault();
          const data = new FormData(event.currentTarget);
          const [kind, id] = subject.split(":");
          add.mutate({ [kind === "role" ? "role_id" : "user_id"]: id, rate_minor: Math.round(Number(data.get("rate")) * 100), effective_from: String(data.get("from")) },
            { onSuccess: () => { setAdding(false); setSubject(""); } });
        }}>
          <label className="block text-sm">{t("time.rates.subject")}
            <select required value={subject} onChange={(event) => setSubject(event.target.value)} className="mt-1 block w-full rounded border bg-background p-2">
              <option value="" />
              <optgroup label={t("time.rates.user")}>{directory.data?.users.map((u) => <option key={u.id} value={`user:${u.id}`}>{u.name}</option>)}</optgroup>
              <optgroup label={t("time.rates.role")}>{directory.data?.roles.map((r) => <option key={r.id} value={`role:${r.id}`}>{r.name}</option>)}</optgroup>
            </select></label>
          <label className="block text-sm">{t("time.rates.rate")}<Input name="rate" type="number" min="0" step="0.01" required className="mt-1" /></label>
          <label className="block text-sm">{t("time.rates.from")}<Input name="from" type="date" required className="mt-1" /></label>
          <DialogFooter>
            <Button type="submit" disabled={add.isPending}>{t("time.rates.add")}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
    {rates.data && !rates.data.items.length && <p className="text-muted-foreground">{t("time.rates.empty")}</p>}
    {!!rates.data?.items.length && <table className="w-full text-left text-sm">
      <caption className="sr-only">{t("time.rates.title")}</caption>
      <thead><tr><th scope="col" className="p-2">{t("time.rates.name")}</th><th scope="col" className="p-2">{t("time.rates.rate")}</th><th scope="col" className="p-2">{t("time.rates.from")}</th><th scope="col" className="p-2">{t("items.status")}</th></tr></thead>
      <tbody>{rates.data.items.map((r) => <tr key={r.id} className="border-t"><th scope="row" className="p-2">{r.name}</th>
        <td className="p-2">{money.format(r.rate_minor / 100)}</td><td className="p-2">{r.effective_from}</td>
        <td className="p-2"><AdminStatus tone={RATE_TONE[statuses.get(r.id) ?? "historical"]} label={t(`time.rates.status.${statuses.get(r.id) ?? "historical"}`)} /></td></tr>)}</tbody>
    </table>}
  </section>;
}
