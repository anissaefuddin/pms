import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

export function AuthCard({ title, children }: { title: string; children: ReactNode }) {
  const { t } = useTranslation();
  return (
    <main className="grid min-h-screen place-items-center bg-muted p-6">
      <section className="w-full max-w-sm rounded-xl border bg-card p-7 shadow-md">
        <div className="mb-6">
          <p className="text-xs font-semibold uppercase tracking-[.22em] text-primary">{t("common.app.name")}</p>
          <h1 className="mt-2 text-2xl font-semibold">{title}</h1>
        </div>
        {children}
      </section>
    </main>
  );
}
