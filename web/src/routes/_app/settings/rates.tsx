import { createFileRoute } from "@tanstack/react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "@/lib/api";
import { useProfile } from "@/lib/profile";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export const Route = createFileRoute("/_app/settings/rates")({ component: Rates });
type Choice = { id: string; name: string };
type Rate = { id: string; name: string; user_id: string | null; role_id: string | null; rate_minor: number; effective_from: string };

function Rates() {
  const { t, i18n } = useTranslation();
  const me = useProfile();
  const cache = useQueryClient();
  const admin = me?.user_type === "admin";
  const [subject, setSubject] = useState("");
  const rates = useQuery({ queryKey: ["rates"], enabled: admin, queryFn: () => api<{ items: Rate[] }>("/api/v1/rates") });
  const directory = useQuery({ queryKey: ["resource-directory"], enabled: admin,
    queryFn: () => api<{ roles: Choice[]; users: Choice[] }>("/api/v1/resource-directory") });
  const add = useMutation({
    mutationFn: (body: object) => api("/api/v1/rates", { method: "POST", body: JSON.stringify(body) }),
    onSuccess: () => cache.invalidateQueries({ queryKey: ["rates"] }),
  });
  const money = new Intl.NumberFormat(i18n.language, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  if (!admin) return null;
  return <section className="space-y-5">
    <h2 className="text-lg font-semibold">{t("time.rates.title")}</h2>
    <p className="text-sm text-muted-foreground">{t("time.rates.hint")}</p>
    {(add.error || rates.error) && <p role="alert">{(add.error ?? rates.error)?.message}</p>}
    <form className="flex flex-wrap items-end gap-3" onSubmit={(event) => {
      event.preventDefault();
      const data = new FormData(event.currentTarget);
      const [kind, id] = subject.split(":");
      add.mutate({ [kind === "role" ? "role_id" : "user_id"]: id, rate_minor: Math.round(Number(data.get("rate")) * 100), effective_from: String(data.get("from")) });
    }}>
      <label className="block text-sm">{t("time.rates.subject")}
        <select required value={subject} onChange={(event) => setSubject(event.target.value)} className="block rounded border bg-background p-2">
          <option value="" />
          <optgroup label={t("time.rates.user")}>{directory.data?.users.map((u) => <option key={u.id} value={`user:${u.id}`}>{u.name}</option>)}</optgroup>
          <optgroup label={t("time.rates.role")}>{directory.data?.roles.map((r) => <option key={r.id} value={`role:${r.id}`}>{r.name}</option>)}</optgroup>
        </select></label>
      <label className="block text-sm">{t("time.rates.rate")}<Input name="rate" type="number" min="0" step="0.01" required /></label>
      <label className="block text-sm">{t("time.rates.from")}<Input name="from" type="date" required /></label>
      <Button type="submit" disabled={add.isPending}>{t("time.rates.add")}</Button>
    </form>
    {rates.data && !rates.data.items.length && <p className="text-muted-foreground">{t("time.rates.empty")}</p>}
    {!!rates.data?.items.length && <table className="w-full text-left text-sm">
      <caption className="sr-only">{t("time.rates.title")}</caption>
      <thead><tr><th scope="col" className="p-2">{t("time.rates.name")}</th><th scope="col" className="p-2">{t("time.rates.rate")}</th><th scope="col" className="p-2">{t("time.rates.from")}</th></tr></thead>
      <tbody>{rates.data.items.map((r) => <tr key={r.id} className="border-t"><th scope="row" className="p-2">{r.name}</th>
        <td className="p-2">{money.format(r.rate_minor / 100)}</td><td className="p-2">{r.effective_from}</td></tr>)}</tbody>
    </table>}
  </section>;
}
