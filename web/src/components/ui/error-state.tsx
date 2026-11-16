import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";

/** Failed load with a retry action. role="alert" so the failure is announced; the retry button is always offered. */
export function ErrorState({ message, retryLabel, onRetry, className }: { message: ReactNode; retryLabel: string; onRetry: () => void; className?: string }) {
  return <div className={className ?? "space-y-2"}>
    <p role="alert">{message}</p>
    <Button variant="outline" onClick={onRetry}>{retryLabel}</Button>
  </div>;
}
