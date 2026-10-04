import { type ReactNode, useState } from "react";
import { Link } from "react-router-dom";
import {
  AlertCircle,
  ArrowUpRight,
  Check,
  Copy,
  Inbox,
  Loader2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";
export function Heading({
  eyebrow,
  title,
  description,
  action,
}: {
  eyebrow?: string;
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <div className="mb-8 flex flex-wrap items-end justify-between gap-4">
      <div>
        {eyebrow && (
          <p className="mb-2 text-xs font-semibold uppercase tracking-[.14em] text-primary">
            {eyebrow}
          </p>
        )}
        <h1 className="text-3xl font-semibold tracking-tight md:text-4xl">
          {title}
        </h1>
        <p className="mt-3 max-w-2xl text-sm leading-6 text-muted-foreground">
          {description}
        </p>
      </div>
      {action}
    </div>
  );
}
export function Loading() {
  return (
    <div aria-label="Loading" className="space-y-6">
      <Skeleton className="h-10 w-64" />
      <div className="grid gap-5 md:grid-cols-3">
        {[1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-40 rounded-xl" />
        ))}
      </div>
      <Skeleton className="h-80 rounded-xl" />
    </div>
  );
}
export function Empty({
  title,
  description,
  action,
}: {
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex min-h-56 flex-col items-center justify-center gap-3 px-6 py-12 text-center">
      <div className="rounded-full bg-muted p-3">
        <Inbox className="size-5 text-muted-foreground" />
      </div>
      <h3 className="font-medium">{title}</h3>
      <p className="max-w-sm text-sm leading-6 text-muted-foreground">
        {description}
      </p>
      {action}
    </div>
  );
}
export function Failure({
  error,
  retry,
}: {
  error: unknown;
  retry?: () => void;
}) {
  return (
    <div
      role="alert"
      className="rounded-xl border border-destructive/30 bg-destructive/5 p-5"
    >
      <div className="flex gap-3">
        <AlertCircle className="mt-0.5 size-5 shrink-0 text-destructive" />
        <div>
          <p className="font-medium">Something needs attention</p>
          <p className="mt-1 text-sm text-muted-foreground">
            {error instanceof Error
              ? error.message
              : "Please retry this request."}
          </p>
          {retry && (
            <Button className="mt-4" variant="outline" onClick={retry}>
              Try again
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
export function Metric({
  label,
  value,
  detail,
  icon,
}: {
  label: string;
  value: ReactNode;
  detail: string;
  icon: ReactNode;
}) {
  return (
    <Card className="gap-0 border shadow-none">
      <CardContent className="p-6">
        <div className="mb-5 flex items-center justify-between">
          <span className="text-sm text-muted-foreground">{label}</span>
          <span className="text-muted-foreground">{icon}</span>
        </div>
        <p className="text-3xl font-semibold tabular-nums tracking-tight">
          {value}
        </p>
        <p className="mt-2 text-xs text-muted-foreground">{detail}</p>
      </CardContent>
    </Card>
  );
}
export function StatusBadge({ state }: { state: string }) {
  const positive = ["paid", "charged", "active"].includes(state);
  return (
    <Badge
      variant="outline"
      className={
        positive
          ? "border-emerald-500/20 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
          : ["pending", "expired"].includes(state)
            ? "border-amber-500/20 bg-amber-500/10 text-amber-700 dark:text-amber-300"
            : "text-muted-foreground"
      }
    >
      {state.charAt(0).toUpperCase() + state.slice(1)}
    </Badge>
  );
}
export function Field({
  label,
  children,
  hint,
  id,
}: {
  label: string;
  children: ReactNode;
  hint?: string;
  id?: string;
}) {
  return (
    <div className="space-y-2">
      <Label htmlFor={id}>{label}</Label>
      {children}
      {hint && (
        <p className="text-xs leading-5 text-muted-foreground">{hint}</p>
      )}
    </div>
  );
}
export function TextField({
  label,
  name,
  defaultValue,
  required = false,
  type = "text",
  hint,
}: {
  label: string;
  name: string;
  defaultValue?: string;
  required?: boolean;
  type?: string;
  hint?: string;
}) {
  return (
    <Field label={label} id={name} hint={hint}>
      <Input
        id={name}
        name={name}
        type={type}
        defaultValue={defaultValue}
        required={required}
      />
    </Field>
  );
}
export function CopyCode({
  value,
  label = "Copy",
}: {
  value: string;
  label?: string;
}) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="group relative rounded-lg border bg-muted/50 p-4 pr-14">
      <pre className="overflow-x-auto whitespace-pre-wrap break-all font-mono text-xs leading-6">
        {value}
      </pre>
      <Button
        className="absolute right-2 top-2"
        size="icon"
        variant="ghost"
        aria-label={label}
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(value);
            setCopied(true);
            toast.success("Copied to clipboard");
            setTimeout(() => setCopied(false), 2000);
          } catch {
            toast.error("Copy is unavailable. Select the text to copy it.");
          }
        }}
      >
        {copied ? <Check className="size-4" /> : <Copy className="size-4" />}
      </Button>
    </div>
  );
}
export function ActionLink({
  to,
  children,
}: {
  to: string;
  children: ReactNode;
}) {
  return (
    <Link
      to={to}
      className="inline-flex items-center gap-1.5 text-sm font-medium text-primary hover:underline"
    >
      {children}
      <ArrowUpRight className="size-4" />
    </Link>
  );
}
export function Submit({
  busy,
  children,
}: {
  busy?: boolean;
  children: ReactNode;
}) {
  return (
    <Button type="submit" disabled={busy}>
      {busy && <Loader2 className="size-4 animate-spin" />}
      {children}
    </Button>
  );
}
export function Confirm({
  open,
  onOpenChange,
  title,
  description,
  children,
  onConfirm,
  busy,
  label = "Confirm",
  destructive = false,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  title: string;
  description: string;
  children?: ReactNode;
  onConfirm: () => void;
  busy?: boolean;
  label?: string;
  destructive?: boolean;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        {children}
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            variant={destructive ? "destructive" : "default"}
            disabled={busy}
            onClick={onConfirm}
          >
            {busy && <Loader2 className="size-4 animate-spin" />}
            {label}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
