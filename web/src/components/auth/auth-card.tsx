import type { ReactNode } from "react";
import { WorkspaceBrandMark } from "@/lib/workspace-branding";

export function AuthCard({ title, children }: { title: string; children: ReactNode }) {
  return (
    <main className="grid min-h-screen place-items-center bg-muted p-6">
      <section className="w-full max-w-sm rounded-xl border bg-card p-7 shadow-md">
        <div className="mb-6">
          <WorkspaceBrandMark />
          <h1 className="mt-2 text-2xl font-semibold">{title}</h1>
        </div>
        {children}
      </section>
    </main>
  );
}
