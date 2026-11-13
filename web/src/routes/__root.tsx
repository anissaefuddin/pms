import { Link, Outlet, createRootRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { buttonVariants } from "@/components/ui/button";
import { Toaster } from "@/components/ui/toaster";

export const Route = createRootRoute({ notFoundComponent: NotFoundPage, component: () => <><Outlet /><Toaster /></> });


function NotFoundPage() {
  const { t } = useTranslation();
  return <main className="mx-auto max-w-lg space-y-4 px-4 py-12 text-center">
    <h1 className="text-xl font-semibold">{t("common.notFound.title")}</h1>
    <p className="text-muted-foreground">{t("common.notFound.description")}</p>
    <Link to="/" className={buttonVariants()}>{t("common.notFound.home")}</Link>
  </main>;
}
