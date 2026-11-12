import { useState, type FormEvent } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { ApiError, api, type CurrentUser } from "@/lib/api";
import { AuthCard } from "@/components/auth/auth-card";
import { FormError } from "@/components/auth/form-error";
import { TextField } from "@/components/auth/text-field";
import { Button } from "@/components/ui/button";

export const Route = createFileRoute("/invitations/$token")({ component: Invitation });

type InvitationData = { email: string; expires_at: string; invited_by_name?: string };

function Invitation() {
  const { t } = useTranslation();
  const { token } = Route.useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const invite = useQuery({ queryKey: ["invitation", token], queryFn: () => api<InvitationData>(`/api/v1/auth/invitations/${token}`) });
  const [error, setError] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const text = (key: string) => String(data.get(key) ?? "").trim();
    const password = String(data.get("password") ?? "");
    if (!text("firstName") || !text("lastName")) return setError(t("auth.invitation.namesRequired"));
    if (password.length < 8) return setError(t("auth.password.tooShort"));
    if (password !== data.get("repeat")) return setError(t("auth.reset.passwordMismatch"));
    if (data.get("terms") !== "on") return setError(t("auth.invitation.termsRequired"));
    setError("");
    try {
      const me = await api<CurrentUser>("/api/v1/auth/invitations/accept", {
        method: "POST",
        body: JSON.stringify({
          token,
          first_name: text("firstName"),
          last_name: text("lastName"),
          password,
          department: text("department") || undefined,
          job_title: text("jobTitle") || undefined,
          accept_terms: true,
        }),
      });
      toast.success(t("auth.invitation.success"));
      queryClient.setQueryData(["me"], me);
      navigate({ to: "/" });
    } catch (e) {
      setError(e instanceof ApiError && e.status === 410 ? t("auth.invitation.expired") : (e as Error).message);
    }
  }

  return (
    <AuthCard title={t("auth.invitation.title")}>
      {invite.isLoading ? (
        <p role="status">{t("common.feedback.loading")}</p>
      ) : invite.error ? (
        <p className="text-sm text-destructive" role="alert">{invite.error instanceof ApiError && invite.error.status === 410 ? t("auth.invitation.expired") : invite.error.message}</p>
      ) : (
        <form className="space-y-3" noValidate onSubmit={submit}>
          <p className="text-sm text-muted-foreground">{t("auth.invitation.recipient", { email: invite.data?.email })}</p>
          <TextField label={t("auth.fields.firstName")} name="firstName" autoComplete="given-name" required autoFocus />
          <TextField label={t("auth.fields.lastName")} name="lastName" autoComplete="family-name" required />
          <TextField label={t("auth.fields.password")} hint={t("auth.fields.passwordRule")} name="password" type="password" autoComplete="new-password" required />
          <TextField label={t("auth.fields.repeatPassword")} name="repeat" type="password" autoComplete="new-password" required />
          <TextField label={t("auth.fields.department")} name="department" autoComplete="organization" />
          <TextField label={t("auth.fields.jobTitle")} name="jobTitle" autoComplete="organization-title" />
          <label className="flex gap-2 text-sm"><input name="terms" type="checkbox" required />{t("auth.invitation.terms")}</label>
          <FormError message={error} />
          <Button type="submit" className="w-full">{t("auth.invitation.submit")}</Button>
        </form>
      )}
    </AuthCard>
  );
}
