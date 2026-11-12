import { useId, useState, type ComponentProps } from "react";
import { Eye, EyeOff } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

type Props = ComponentProps<typeof Input> & { label: string; hint?: string };

/** Label + input; password fields get a show/hide toggle. */
export function TextField({ label, hint, type, ...props }: Props) {
  const { t } = useTranslation();
  const id = useId();
  const [shown, setShown] = useState(false);
  const isPassword = type === "password";
  const input = <Input id={id} type={isPassword && shown ? "text" : type} aria-describedby={hint ? `${id}-hint` : undefined} {...props} />;
  return (
    <div className="space-y-1.5">
      <label htmlFor={id} className="block text-sm font-medium">{label}</label>
      {isPassword ? (
        <div className="relative">
          {input}
          <Button type="button" variant="ghost" size="icon-xs" className="absolute top-1 right-1" aria-pressed={shown} aria-label={shown ? t("auth.fields.hidePassword") : t("auth.fields.showPassword")} onClick={() => setShown(!shown)}>
            {shown ? <EyeOff /> : <Eye />}
          </Button>
        </div>
      ) : input}
      {hint && <p id={`${id}-hint`} className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  );
}
