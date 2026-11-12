import { useTranslation } from "react-i18next";

/** Length plus character-class heuristic; advisory only, the server enforces the real rules. */
export function PasswordStrength({ value }: { value: string }) {
  const { t } = useTranslation();
  if (!value) return null;
  const classes = [/[a-z]/, /[A-Z]/, /\d/, /[^A-Za-z0-9]/].filter((re) => re.test(value)).length;
  const level = value.length < 8 ? "weak" : value.length >= 12 && classes >= 3 ? "strong" : "fair";
  return <p className="text-xs text-muted-foreground" role="status">{t(`auth.password.strength.${level}`)}</p>;
}
