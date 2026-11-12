export function FormError({ message }: { message: string }) {
  return message ? <p className="text-sm text-destructive" role="alert">{message}</p> : null;
}
