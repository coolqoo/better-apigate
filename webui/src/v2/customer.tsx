import { useState, type FormEvent } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Activity,
  ArrowRight,
  ArrowUpRight,
  Check,
  CheckCircle2,
  Clock,
  CreditCard,
  KeyRound,
  Plus,
  RefreshCw,
  Shield,
  Wallet as WalletIcon,
  Zap,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
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
import { Switch } from "@/components/ui/switch";
import { Progress } from "@/components/ui/progress";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  api,
  date,
  dollars,
  micros,
  operation,
  type APIKey,
  type Ledger,
  type Order,
  type Plan,
  type Provider,
  type UsageDay,
  type UsageSummary,
  type Wallet,
} from "./api";
import { useSession } from "./context";
import { useHistory, HistoryNavigation } from "./history";
import {
  ActionLink,
  Confirm,
  CopyCode,
  Empty,
  Failure,
  Field,
  Heading,
  Loading,
  Metric,
  StatusBadge,
  Submit,
  TextField,
} from "./shared";
export function useData<T>(path: string) {
  return useQuery({ queryKey: [path], queryFn: () => api<T>(path) });
}
export function UsageChart({ days }: { days: UsageDay[] }) {
  if (days.length === 0)
    return (
      <Empty
        title="Your usage story starts here"
        description="Once you make your first API call, request volume and errors will appear here."
        action={<ActionLink to="/docs">Make your first request</ActionLink>}
      />
    );
  const maximum = Math.max(...days.map((d) => d.requests), 1);
  return (
    <div className="px-6 pb-6">
      <div className="mb-6 flex gap-5 text-xs text-muted-foreground">
        <span className="flex items-center gap-2">
          <span className="size-2 rounded-full bg-primary" />
          Requests
        </span>
        <span className="flex items-center gap-2">
          <span className="size-2 rounded-full bg-destructive/70" />
          Errors
        </span>
      </div>
      <div
        role="img"
        aria-label={`Daily requests over ${days.length} days`}
        className="flex h-48 items-end gap-2 border-b border-dashed pb-0"
      >
        {days.map((d) => (
          <div
            key={d.id}
            className="group flex h-full min-w-0 flex-1 flex-col justify-end"
            title={`${d.id}: ${d.requests} requests, ${d.errors} errors`}
          >
            <div
              className="relative min-h-1 rounded-t-md bg-primary/75 transition-colors hover:bg-primary"
              style={{ height: `${(d.requests / maximum) * 100}%` }}
            >
              {d.errors > 0 && (
                <div
                  className="absolute bottom-0 w-full rounded-t bg-destructive/70"
                  style={{
                    height: `${(d.errors / Math.max(d.requests, 1)) * 100}%`,
                  }}
                />
              )}
            </div>
          </div>
        ))}
      </div>
      <div className="mt-3 flex justify-between text-[10px] text-muted-foreground">
        <span>{date(days[0].id)}</span>
        <span>{date(days.at(-1)?.id)}</span>
      </div>
    </div>
  );
}
export function LedgerTable({ entries }: { entries: Ledger[] }) {
  return entries.length ? (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Activity</TableHead>
          <TableHead>Date</TableHead>
          <TableHead className="text-right">Amount</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {entries.map((e) => (
          <TableRow key={e.id}>
            <TableCell>
              <p className="text-sm font-medium">{e.description}</p>
              <p className="mt-1 text-xs text-muted-foreground">
                {e.kind.replaceAll("_", " ")}
              </p>
            </TableCell>
            <TableCell className="whitespace-nowrap text-xs text-muted-foreground">
              {date(e.created_at)}
            </TableCell>
            <TableCell
              className={`text-right font-mono text-sm ${micros(e.amount) > 0 ? "text-emerald-600 dark:text-emerald-400" : ""}`}
            >
              {micros(e.amount) > 0 ? "+" : ""}
              {dollars(e.amount, 6)}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  ) : (
    <Empty
      title="No transactions yet"
      description="Top-ups, plan purchases, and request spending will appear in your ledger."
    />
  );
}
export function Overview() {
  const { session } = useSession();
  const wallet = useData<Wallet>("/wallet");
  const usage = useData<UsageDay[]>("/usage");
  const summary = useData<UsageSummary>("/usage/summary");
  const ledger = useData<Ledger[]>("/ledger");
  const keys = useData<APIKey[]>("/keys");
  if (wallet.isPending || usage.isPending || summary.isPending)
    return <Loading />;
  if (wallet.error || usage.error || summary.error)
    return (
      <Failure
        error={wallet.error || usage.error || summary.error}
        retry={() => {
          void wallet.refetch();
          void usage.refetch();
          void summary.refetch();
        }}
      />
    );
  const a = wallet.data!;
  const days = usage.data || [];
  const requests = summary.data!.requests;
  const remaining = a.term
    ? Math.max(
        0,
        a.term.included_units - a.term.used_units - a.term.reserved_units,
      )
    : 0;
  const funded = micros(a.available) > 0 || remaining > 0;
  return (
    <>
      <Heading
        eyebrow="Your workspace"
        title={`Hello, ${session?.name.split(" ")[0] || "there"}.`}
        description="Everything you need to keep your API running, in one place."
        action={
          <Button asChild>
            <Link to="/portal/wallet">
              <Plus className="size-4" />
              Add funds
            </Link>
          </Button>
        }
      />
      {a.frozen && (
        <div className="mb-6">
          <Failure
            error={
              new Error(
                `Paid access is frozen. A payment reversal left ${dollars(a.shortfall)} to reconcile. Contact the gateway administrator.`,
              )
            }
          />
        </div>
      )}
      <div className="mb-8 grid gap-5 sm:grid-cols-2 xl:grid-cols-3">
        <Metric
          label="Available balance"
          value={dollars(a.available)}
          detail={`${dollars(a.reserved, 6)} reserved for active requests`}
          icon={<WalletIcon className="size-4" />}
        />
        <Metric
          label="Included units left"
          value={a.term ? remaining.toLocaleString() : "Pay as you go"}
          detail={
            a.term
              ? `of ${a.term.included_units.toLocaleString()} this term`
              : "Usage draws directly from your wallet"
          }
          icon={<Zap className="size-4" />}
        />
        <Metric
          label="Requests this month"
          value={requests.toLocaleString()}
          detail="Last 30 days · all API keys"
          icon={<Activity className="size-4" />}
        />
        <Metric
          label={a.term && !a.auto_renew ? "Term ends" : "Next renewal"}
          value={
            a.term
              ? date(a.term.ends_at).replace(/, \d{4}/, "")
              : "No subscription"
          }
          detail={
            a.term
              ? !a.auto_renew
                ? "Pay as you go at expiry · no renewal charge"
                : a.renewal_price
                  ? `${dollars(a.renewal_price)} · auto-renew on`
                  : "Plan unavailable · pay as you go at expiry"
              : "Choose a plan to include usage units"
          }
          icon={<RefreshCw className="size-4" />}
        />
        <Metric
          label="Spent in last 30 days"
          value={dollars(summary.data!.spending)}
          detail={`${dollars(summary.data!.usage_spending)} requests · ${dollars(summary.data!.plan_spending)} plans`}
          icon={<CreditCard className="size-4" />}
        />
        <Metric
          label="API errors"
          value={summary.data!.errors.toLocaleString()}
          detail={`Last 30 days · ${requests ? ((summary.data!.errors / requests) * 100).toFixed(1) : "0.0"}% of requests`}
          icon={<Activity className="size-4" />}
        />
      </div>
      {!funded && requests > 0 && !a.frozen && (
        <Card className="mb-8 border-amber-500/25 bg-amber-500/5 shadow-none">
          <CardContent className="flex flex-wrap items-center justify-between gap-4 p-6">
            <div>
              <p className="font-semibold">Add funds to resume requests</p>
              <p className="mt-1 text-sm text-muted-foreground">
                Your included units and available balance are exhausted. New
                paid requests are paused until you fund your wallet.
              </p>
            </div>
            <Button asChild>
              <Link to="/portal/wallet">
                Top up wallet
                <ArrowRight className="size-4" />
              </Link>
            </Button>
          </CardContent>
        </Card>
      )}
      {(!keys.data?.some((k) => !k.revoked_at) || requests === 0) && (
        <Card className="mb-8 gap-0 border-primary/15 bg-primary/[.025] shadow-none">
          <CardContent className="p-6">
            <div className="mb-5 flex items-center justify-between">
              <div>
                <p className="text-base font-semibold">
                  Your next great idea starts with one request
                </p>
                <p className="mt-1 text-sm text-muted-foreground">
                  A few quick steps and you’re ready to build.
                </p>
              </div>
              <Zap className="hidden size-6 text-primary sm:block" />
            </div>
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              {[
                {
                  done: funded,
                  title: "1. Fund your wallet",
                  detail: "Start small. Your balance stays yours.",
                  to: "/portal/wallet",
                },
                {
                  done: !!a.term || requests > 0,
                  title: "2. Choose your plan",
                  detail: "Included units or flexible pay as you go.",
                  to: "/portal/plans",
                },
                {
                  done: !!keys.data?.some((k) => !k.revoked_at),
                  title: "3. Create an API key",
                  detail: "Secure access for your application.",
                  to: "/portal/keys",
                },
                {
                  done: requests > 0,
                  title: "4. Make your first call",
                  detail: "Copy a request from the API docs.",
                  to: "/docs",
                },
              ].map((step) => (
                <Link
                  key={step.to}
                  to={step.to}
                  className="flex items-center gap-3 rounded-lg border bg-card p-4 transition-colors hover:border-primary/40"
                >
                  <span
                    className={`flex size-7 shrink-0 items-center justify-center rounded-full ${step.done ? "bg-emerald-500/10 text-emerald-600" : "bg-primary/10 text-primary"}`}
                  >
                    {step.done ? (
                      <Check className="size-4" />
                    ) : (
                      <ArrowRight className="size-4" />
                    )}
                  </span>
                  <div>
                    <p className="text-xs font-semibold">{step.title}</p>
                    <p className="mt-1 text-[11px] text-muted-foreground">
                      {step.detail}
                    </p>
                  </div>
                </Link>
              ))}
            </div>
          </CardContent>
        </Card>
      )}
      <div className="grid gap-6 xl:grid-cols-[1.6fr_1fr]">
        <Card className="shadow-none">
          <CardHeader className="flex flex-row items-start justify-between">
            <div>
              <CardTitle>Request activity</CardTitle>
              <CardDescription className="mt-2">
                A clear view of your last 30 days.
              </CardDescription>
            </div>
            <Badge variant="outline">Last 30 days</Badge>
          </CardHeader>
          <UsageChart days={days} />
        </Card>
        <Card className="shadow-none">
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>Current plan</CardTitle>
              <Badge variant="secondary">
                {a.term ? "30-day term" : "Flexible"}
              </Badge>
            </div>
          </CardHeader>
          <CardContent>
            <div className="mb-6 flex items-center gap-3">
              <div className="rounded-lg bg-accent p-3">
                <Zap className="size-5 text-primary" />
              </div>
              <div>
                <p className="text-xl font-semibold">
                  {a.term?.plan_name || "Pay as you go"}
                </p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {a.term
                    ? `${dollars(a.term.price)} per 30-day term`
                    : "Only pay for the units you use"}
                </p>
              </div>
            </div>
            {a.term && (
              <>
                <div className="mb-2 flex justify-between text-xs">
                  <span>Included usage</span>
                  <span className="tabular-nums">
                    {a.term.used_units.toLocaleString()} /{" "}
                    {a.term.included_units.toLocaleString()}
                  </span>
                </div>
                <Progress
                  value={
                    a.term.included_units
                      ? (a.term.used_units / a.term.included_units) * 100
                      : 0
                  }
                  className="h-1.5"
                />
              </>
            )}
            <p className="mb-6 mt-5 text-sm leading-6 text-muted-foreground">
              {a.term
                ? "Additional usage draws from available wallet funds. Included units reset at renewal and do not roll over."
                : "Your wallet funds each request before it is forwarded. Choose a plan when you’re ready for included units."}
            </p>
            <ActionLink to="/portal/plans">Explore plans</ActionLink>
          </CardContent>
        </Card>
      </div>
      <Card className="mt-6 shadow-none">
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>Recent wallet activity</CardTitle>
          <ActionLink to="/portal/wallet">View ledger</ActionLink>
        </CardHeader>
        <CardContent>
          {ledger.error ? (
            <Failure error={ledger.error} />
          ) : (
            <LedgerTable entries={(ledger.data || []).slice(0, 5)} />
          )}
        </CardContent>
      </Card>
    </>
  );
}
export function Keys() {
  const keys = useData<APIKey[]>("/keys");
  const qc = useQueryClient();
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [created, setCreated] = useState<APIKey>();
  const [revoke, setRevoke] = useState<APIKey>();
  const create = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    const f = new FormData(e.currentTarget);
    try {
      const k = await api<APIKey>("/keys", "POST", {
        name: f.get("name"),
        scopes: String(f.get("scopes") || "")
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean),
      });
      setCreated(k);
      await qc.invalidateQueries({ queryKey: ["/keys"] });
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };
  return (
    <>
      <Heading
        title="API Keys"
        description="Secure credentials for your applications. All keys share your account’s funding and rate limits."
        action={
          <Button
            onClick={() => {
              setCreated(undefined);
              setError(null);
              setOpen(true);
            }}
          >
            <Plus className="size-4" />
            Create key
          </Button>
        }
      />
      <Card className="shadow-none">
        <CardHeader>
          <CardTitle>Your API keys</CardTitle>
          <CardDescription>
            Keys are shown once at creation. Store them securely.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {keys.isPending ? (
            <Loading />
          ) : keys.error ? (
            <Failure error={keys.error} />
          ) : !keys.data?.length ? (
            <Empty
              title="Your first key is one click away"
              description="Create a key to connect your application to the gateway."
              action={
                <Button variant="outline" onClick={() => setOpen(true)}>
                  <KeyRound className="size-4" />
                  Create your first key
                </Button>
              }
            />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Key</TableHead>
                  <TableHead>Last used</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {keys.data.map((k) => (
                  <TableRow key={k.id}>
                    <TableCell className="font-medium">{k.name}</TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">
                      {k.prefix}••••••
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {date(k.last_used)}
                    </TableCell>
                    <TableCell>
                      <StatusBadge
                        state={
                          k.revoked_at
                            ? "revoked"
                            : k.expires_at &&
                                new Date(k.expires_at) < new Date()
                              ? "expired"
                              : "active"
                        }
                      />
                    </TableCell>
                    <TableCell className="text-right">
                      {!k.revoked_at && (
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => setRevoke(k)}
                        >
                          Revoke
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
      <div className="mt-6 flex items-start gap-3 rounded-xl border p-5">
        <Shield className="mt-0.5 size-4 shrink-0 text-primary" />
        <p className="text-sm leading-6 text-muted-foreground">
          Keep keys on your server. Use endpoint scopes for limited access, and
          revoke any key that may have been exposed.
        </p>
      </div>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {created ? "Your key is ready" : "Create an API key"}
            </DialogTitle>
            <DialogDescription>
              {created
                ? "Copy this key now. You won’t be able to view it again."
                : "Give your key a name so you can recognize it later."}
            </DialogDescription>
          </DialogHeader>
          {created ? (
            <>
              <CopyCode value={created.raw_key || ""} label="Copy API key" />
              <Button onClick={() => setOpen(false)}>I’ve saved my key</Button>
            </>
          ) : (
            <form onSubmit={create} className="space-y-5">
              {error !== null && <Failure error={error} />}
              <TextField label="Key name" name="name" required />
              <TextField
                label="Endpoint scopes (optional)"
                name="scopes"
                hint="Comma-separated paths, such as /v1/chat/* . Leave empty for all endpoints."
              />
              <DialogFooter>
                <Submit busy={busy}>Create key</Submit>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>
      <Confirm
        open={!!revoke}
        onOpenChange={() => setRevoke(undefined)}
        title="Revoke this key?"
        description={`Applications using “${revoke?.name}” will immediately lose access. This cannot be undone.`}
        destructive
        label="Revoke key"
        busy={busy}
        onConfirm={async () => {
          if (!revoke) return;
          setBusy(true);
          try {
            await api("/keys/" + revoke.id, "DELETE");
            setRevoke(undefined);
            await qc.invalidateQueries({ queryKey: ["/keys"] });
            toast.success("Key revoked");
          } catch (e) {
            toast.error(
              e instanceof Error ? e.message : "Unable to revoke key",
            );
          } finally {
            setBusy(false);
          }
        }}
      />
    </>
  );
}
export function Usage() {
  const q = useData<UsageDay[]>("/usage");
  if (q.isPending) return <Loading />;
  if (q.error)
    return <Failure error={q.error} retry={() => void q.refetch()} />;
  const days = q.data || [];
  const requests = days.reduce((s, d) => s + d.requests, 0),
    errors = days.reduce((s, d) => s + d.errors, 0);
  return (
    <>
      <Heading
        title="Usage"
        description="Understand your API activity across all keys. Usage appears shortly after each settled request."
      />
      <div className="mb-6 grid gap-5 md:grid-cols-3">
        <Metric
          label="Total requests"
          value={requests.toLocaleString()}
          detail="Last 30 days"
          icon={<Activity className="size-4" />}
        />
        <Metric
          label="Usage units"
          value={days.reduce((s, d) => s + d.units, 0).toLocaleString()}
          detail="Fixed route costs, recorded per request"
          icon={<Zap className="size-4" />}
        />
        <Metric
          label="Error responses"
          value={errors.toLocaleString()}
          detail={`${requests ? ((errors / requests) * 100).toFixed(1) : "0"}% of requests`}
          icon={<Shield className="size-4" />}
        />
      </div>
      <Card className="shadow-none">
        <CardHeader>
          <CardTitle>Request volume</CardTitle>
          <CardDescription>Daily requests and error responses.</CardDescription>
        </CardHeader>
        <UsageChart days={days} />
      </Card>
      {days.length > 0 && (
        <Card className="mt-6 shadow-none">
          <CardHeader>
            <CardTitle>Daily breakdown</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Date (UTC)</TableHead>
                  <TableHead>Requests</TableHead>
                  <TableHead>Units</TableHead>
                  <TableHead>Errors</TableHead>
                  <TableHead>Average latency</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {[...days].reverse().map((d) => (
                  <TableRow key={d.id}>
                    <TableCell>{date(d.id)}</TableCell>
                    <TableCell>{d.requests.toLocaleString()}</TableCell>
                    <TableCell>{d.units.toLocaleString()}</TableCell>
                    <TableCell>{d.errors}</TableCell>
                    <TableCell>{d.latency_ms.toFixed(0)} ms</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}
    </>
  );
}
export function Plans() {
  const plans = useData<Plan[]>("/plans");
  const wallet = useData<Wallet>("/wallet");
  const qc = useQueryClient();
  const [selected, setSelected] = useState<Plan>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  if (plans.isPending || wallet.isPending) return <Loading />;
  if (plans.error || wallet.error)
    return <Failure error={plans.error || wallet.error} />;
  const a = wallet.data!;
  return (
    <>
      <Heading
        title="Plans that fit your pace"
        description="Included units for predictable usage. A prepaid wallet for everything beyond it."
      />
      <div className="mb-8 flex flex-wrap items-center gap-3 rounded-lg border bg-card p-4 text-xs text-muted-foreground">
        <Clock className="size-4" />
        <span>30-day terms</span>
        <span>·</span>
        <span>No rollover or proration</span>
        <span>·</span>
        <span>Changes apply at the next renewal</span>
      </div>
      <p className="mb-6 text-sm text-muted-foreground">
        Catalog prices apply to new terms. Your active term keeps its purchased
        price and unit rate until expiry.
      </p>
      {a.term && a.next_plan_id && (
        <Card className="mb-6 border-primary/25 shadow-none">
          <CardContent className="flex flex-wrap items-center justify-between gap-4 p-6">
            <div>
              <p className="font-medium">Scheduled plan change</p>
              <p className="mt-1 text-sm text-muted-foreground">
                {plans.data?.find((p) => p.id === a.next_plan_id)?.name ||
                  "Your selected plan"}{" "}
                at renewal on {date(a.term.ends_at)}. No charge is taken today.
                {!a.auto_renew &&
                  " Enable automatic renewal below for this change to take effect."}
              </p>
            </div>
            <Button
              variant="outline"
              disabled={busy}
              onClick={async () => {
                setBusy(true);
                try {
                  await api("/plan/renewal", "PATCH", {
                    auto_renew: a.auto_renew,
                    next_plan_id: "",
                  });
                  await qc.invalidateQueries({ queryKey: ["/wallet"] });
                  toast.success("Scheduled change canceled");
                } catch (e) {
                  toast.error(
                    e instanceof Error
                      ? e.message
                      : "Unable to cancel the change",
                  );
                } finally {
                  setBusy(false);
                }
              }}
            >
              Cancel scheduled change
            </Button>
          </CardContent>
        </Card>
      )}
      <div className="grid gap-6 md:grid-cols-2 xl:grid-cols-3">
        {plans.data?.map((p) => {
          const active = a.term?.plan_id === p.id || (!a.term && p.is_base);
          return (
            <Card
              key={p.id}
              className={`relative gap-0 shadow-none ${active ? "border-primary/40" : ""}`}
            >
              <CardHeader className="pb-6">
                <div className="flex items-center justify-between">
                  <CardTitle>{p.name}</CardTitle>
                  {active && <Badge>Current plan</Badge>}
                </div>
                <CardDescription className="mt-3 min-h-10 leading-6">
                  {p.description}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="mb-6">
                  <span className="text-4xl font-semibold tracking-tight">
                    {dollars(p.price)}
                  </span>
                  <span className="ml-2 text-xs text-muted-foreground">
                    {p.is_base ? "subscription fee" : "/ 30 days"}
                  </span>
                </div>
                <div className="mb-7 space-y-4 text-sm">
                  {[
                    p.is_base
                      ? "Wallet-funded usage"
                      : `${p.included_units.toLocaleString()} included units`,
                    `${dollars(p.unit_price, 6)} per additional unit`,
                    `${p.rate_limit_per_minute.toLocaleString()} requests / minute`,
                    "All API keys share your account limits",
                  ].map((item) => (
                    <div key={item} className="flex items-start gap-2.5">
                      <Check className="mt-0.5 size-4 shrink-0 text-primary" />
                      {item}
                    </div>
                  ))}
                </div>
                <Button
                  className="w-full"
                  variant={active ? "outline" : "default"}
                  disabled={active || busy || (p.is_base && !a.auto_renew)}
                  onClick={() => {
                    setSelected(p);
                    setError(null);
                  }}
                >
                  {active
                    ? "Your current plan"
                    : p.is_base
                      ? a.auto_renew
                        ? "Return to pay as you go"
                        : "Pay as you go at expiry"
                      : a.term
                        ? "Schedule plan change"
                        : "Choose plan"}
                </Button>
              </CardContent>
            </Card>
          );
        })}
      </div>
      {a.term && (
        <Card className="mt-8 shadow-none">
          <CardContent className="flex flex-wrap items-center justify-between gap-4 p-6">
            <div className="max-w-xl">
              <h3 className="font-medium">Automatic renewal</h3>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">
                {a.auto_renew
                  ? a.renewal_price
                    ? `At expiry on ${date(a.term.ends_at)}, we’ll use ${dollars(a.renewal_price)} in available wallet funds to renew your plan. If the full price isn’t available, you’ll switch to pay as you go.`
                    : "The renewal plan is currently unavailable. You’ll return to pay as you go at expiry."
                  : `Automatic renewal is off. Your current term ends on ${date(a.term.ends_at)}, then you’ll return to pay as you go.`}{" "}
                Top-ups won’t restart a failed subscription automatically.
              </p>
            </div>
            <Switch
              aria-label="Automatic renewal"
              checked={a.auto_renew}
              disabled={busy}
              onCheckedChange={async (v) => {
                setBusy(true);
                try {
                  await api("/plan/renewal", "PATCH", {
                    auto_renew: v,
                    next_plan_id: a.next_plan_id || "",
                  });
                  await qc.invalidateQueries({ queryKey: ["/wallet"] });
                  toast.success(
                    v
                      ? "Automatic renewal enabled"
                      : "Automatic renewal disabled",
                  );
                } catch (e) {
                  toast.error(
                    e instanceof Error ? e.message : "Unable to update renewal",
                  );
                } finally {
                  setBusy(false);
                }
              }}
            />
          </CardContent>
        </Card>
      )}
      <Confirm
        open={!!selected}
        onOpenChange={() => setSelected(undefined)}
        title={
          selected?.is_base
            ? "Return to pay as you go?"
            : a.term
              ? "Schedule a plan change?"
              : "Purchase this plan?"
        }
        description={
          selected?.is_base
            ? "Automatic renewal will be switched off. Your current term stays in place until expiry, then usage will draw from your wallet at the base plan’s prices. No charge is taken today."
            : a.term
              ? `“${selected?.name}” will start at your next renewal. Your current term stays in place. No charge is taken today.`
              : `${dollars(selected?.price || "0")} will be debited from your available wallet balance for a 30-day term. Automatic renewal starts enabled and can be switched off in Plans.`
        }
        busy={busy}
        label={
          selected?.is_base
            ? "Switch off renewal"
            : a.term
              ? "Schedule change"
              : "Purchase plan"
        }
        onConfirm={async () => {
          if (!selected) return;
          setBusy(true);
          setError(null);
          try {
            if (selected.is_base) {
              await api("/plan/renewal", "PATCH", {
                auto_renew: false,
                next_plan_id: "",
              });
            } else {
              await api(
                "/plan/purchase",
                "POST",
                { plan_id: selected.id },
                operation(),
              );
            }
            setSelected(undefined);
            await qc.invalidateQueries({ queryKey: ["/wallet"] });
            toast.success(
              selected.is_base
                ? "Renewal switched off"
                : a.term
                  ? "Plan change scheduled"
                  : "Plan activated",
            );
          } catch (e) {
            setError(e);
          } finally {
            setBusy(false);
          }
        }}
      >
        {error !== null && <Failure error={error} />}
        <div className="rounded-lg bg-muted p-4 text-sm">
          Available wallet balance{" "}
          <strong className="float-right">{dollars(a.available)}</strong>
        </div>
      </Confirm>
    </>
  );
}
export function WalletPage() {
  const q = useData<Wallet>("/wallet");
  const ledger = useHistory<Ledger>("/ledger");
  const orders = useHistory<Order>("/orders");
  const providers = useData<Provider[]>("/providers");
  const { status } = useSession();
  const qc = useQueryClient();
  const [params] = useSearchParams();
  const orderID = params.get("order");
  const order = useQuery({
    queryKey: ["order", orderID],
    queryFn: () => api<Order>("/orders/" + orderID),
    enabled: !!orderID,
    refetchInterval: (query) =>
      query.state.data?.state === "pending" ? 5000 : false,
  });
  const [open, setOpen] = useState(false);
  const [amount, setAmount] = useState("");
  const [provider, setProvider] = useState("");
  const [step, setStep] = useState(1);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [op, setOp] = useState(operation);
  const begin = () => {
    setAmount(status?.top_up_amounts[0] || "10.000000");
    setProvider(providers.data?.[0]?.id || "");
    setStep(1);
    setError(null);
    setOp(operation());
    setOpen(true);
  };
  const checkout = async () => {
    setBusy(true);
    setError(null);
    try {
      const o = await api<Order>("/top-ups", "POST", { amount, provider }, op);
      await qc.invalidateQueries({ queryKey: ["/orders"] });
      if (o.checkout_url) {
        if (o.provider === "paddle" && status?.paddle_client_token) {
          await launchPaddle(
            o,
            status.paddle_client_token,
            status.paddle_sandbox,
          );
          setOpen(false);
        } else window.location.assign(o.checkout_url);
      } else {
        window.location.assign("/portal/wallet?order=" + o.id);
      }
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };
  if (q.isPending) return <Loading />;
  if (q.error)
    return <Failure error={q.error} retry={() => void q.refetch()} />;
  const a = q.data!;
  return (
    <>
      <Heading
        title="Wallet & Payments"
        description="Add funds on your terms. Every payment and every unit of spending, accounted for."
        action={
          <Button onClick={begin}>
            <Plus className="size-4" />
            Add funds
          </Button>
        }
      />
      {orderID &&
        (order.isPending ? (
          <div className="mb-6">
            <Loading />
          </div>
        ) : order.error ? (
          <div className="mb-6">
            <Failure error={order.error} />
          </div>
        ) : (
          order.data && (
            <Card className="mb-6 border-primary/30 shadow-none">
              <CardContent className="flex flex-wrap items-center justify-between gap-4 p-6">
                <div>
                  <div className="flex items-center gap-3">
                    <p className="font-semibold">
                      {order.data.state === "paid"
                        ? "Payment verified"
                        : order.data.state === "expired"
                          ? "Checkout expired"
                          : order.data.state === "reversed"
                            ? "Payment reversed"
                            : "Awaiting verified payment"}
                    </p>
                    <StatusBadge state={order.data.state} />
                  </div>
                  <p className="mt-2 text-sm text-muted-foreground">
                    Wallet credit: {dollars(order.data.amount)} USD
                    {order.data.crypto_amount
                      ? ` · Provider quote: ${order.data.crypto_amount} ${order.data.crypto_token}`
                      : ""}
                  </p>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {order.data.state === "paid"
                      ? "Your wallet has been credited. Refresh your balance to continue."
                      : order.data.state === "pending"
                        ? "This page checks for a signed provider confirmation. Returning from checkout does not credit funds."
                        : order.data.state === "expired"
                          ? "No unverified funds were credited. Start a new checkout when ready."
                          : "Review the ledger or contact the gateway administrator."}
                  </p>
                </div>
                {order.data.state === "paid" ? (
                  <Button
                    variant="outline"
                    onClick={() => void qc.invalidateQueries()}
                  >
                    Refresh balance
                  </Button>
                ) : order.data.state === "pending" &&
                  order.data.checkout_url ? (
                  <Button asChild>
                    <a href={order.data.checkout_url}>
                      Continue checkout
                      <ArrowUpRight className="size-4" />
                    </a>
                  </Button>
                ) : null}
              </CardContent>
            </Card>
          )
        ))}
      <div className="mb-8 grid gap-6 md:grid-cols-[1.5fr_1fr]">
        <Card className="gap-0 border-primary/20 bg-gradient-to-br from-primary/[.08] via-card to-card shadow-none">
          <CardContent className="p-7">
            <div className="flex justify-between">
              <p className="text-sm text-muted-foreground">
                Available to spend
              </p>
              <WalletIcon className="size-5 text-primary" />
            </div>
            <p className="mt-5 text-5xl font-semibold tracking-tight tabular-nums">
              {dollars(a.available)}
            </p>
            <div className="mt-6 flex flex-wrap gap-x-8 gap-y-3 text-xs text-muted-foreground">
              <span>
                Wallet total{" "}
                <strong className="ml-2 font-medium text-foreground">
                  {dollars(a.balance, 6)}
                </strong>
              </span>
              <span>
                Reserved{" "}
                <strong className="ml-2 font-medium text-foreground">
                  {dollars(a.reserved, 6)}
                </strong>
              </span>
            </div>
            <p className="mt-6 text-xs leading-5 text-muted-foreground">
              All balances are in USD. Reserved funds cover requests in progress
              and are unavailable for new spending.
            </p>
          </CardContent>
        </Card>
        <Card className="shadow-none">
          <CardHeader>
            <CardTitle>Flexible ways to pay</CardTitle>
            <CardDescription>
              All providers fund the same wallet.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {providers.data?.length ? (
                providers.data.map((p) => (
                  <div key={p.id} className="flex items-center gap-3 text-sm">
                    <div className="rounded-md bg-muted p-2">
                      {p.crypto ? (
                        <Zap className="size-4" />
                      ) : (
                        <CreditCard className="size-4" />
                      )}
                    </div>
                    {p.name}
                    <CheckCircle2 className="ml-auto size-4 text-emerald-600" />
                  </div>
                ))
              ) : (
                <p className="text-sm leading-6 text-muted-foreground">
                  Payment providers aren’t configured yet. Contact your gateway
                  administrator to enable top-ups.
                </p>
              )}
            </div>
          </CardContent>
        </Card>
      </div>
      {a.frozen && (
        <div className="mb-6">
          <Failure
            error={
              new Error(
                `Paid access is frozen pending reconciliation. Outstanding shortfall: ${dollars(a.shortfall)}.`,
              )
            }
          />
        </div>
      )}
      <Card className="mb-6 shadow-none">
        <CardHeader>
          <CardTitle>Wallet ledger</CardTitle>
          <CardDescription>
            Every balance change is recorded in an immutable ledger.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {ledger.error ? (
            <Failure error={ledger.error} />
          ) : (
            <LedgerTable entries={ledger.data || []} />
          )}
          <HistoryNavigation history={ledger} />
        </CardContent>
      </Card>
      <Card className="shadow-none">
        <CardHeader>
          <CardTitle>Payment history</CardTitle>
          <CardDescription>
            Hosted checkouts and their verified status.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {orders.error ? (
            <Failure error={orders.error} />
          ) : orders.data?.length ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Order</TableHead>
                  <TableHead>Provider</TableHead>
                  <TableHead>Wallet credit</TableHead>
                  <TableHead>Payment amount</TableHead>
                  <TableHead>Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {orders.data.map((o) => (
                  <TableRow key={o.id}>
                    <TableCell>
                      <Link
                        to={"/portal/wallet?order=" + o.id}
                        className="font-mono text-xs text-primary hover:underline"
                      >
                        {o.id.slice(0, 12)}
                      </Link>
                      <p className="mt-1 text-xs text-muted-foreground">
                        {date(o.created_at)}
                      </p>
                    </TableCell>
                    <TableCell className="capitalize">{o.provider}</TableCell>
                    <TableCell>{dollars(o.amount)} USD</TableCell>
                    <TableCell>
                      {o.crypto_amount
                        ? `${o.crypto_amount} ${o.crypto_token}`
                        : `${dollars(o.amount)} USD`}
                    </TableCell>
                    <TableCell>
                      <StatusBadge state={o.state} />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ) : (
            <Empty
              title="No payments yet"
              description="Choose an amount and payment provider to fund your wallet."
            />
          )}
          <HistoryNavigation history={orders} />
        </CardContent>
      </Card>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Add funds to your wallet</DialogTitle>
            <DialogDescription>
              {step === 1
                ? "Choose how much USD wallet credit to add."
                : "Choose an enabled provider. Payment is completed on its hosted checkout."}
            </DialogDescription>
          </DialogHeader>
          <div className="mb-1 flex items-center gap-3 text-xs">
            <span className="flex size-6 items-center justify-center rounded-full bg-primary text-primary-foreground">
              {step === 1 ? "1" : <Check className="size-3" />}
            </span>
            <span>Amount</span>
            <div className="h-px flex-1 bg-border" />
            <span
              className={`flex size-6 items-center justify-center rounded-full ${step === 2 ? "bg-primary text-primary-foreground" : "bg-muted"}`}
            >
              2
            </span>
            <span>Payment</span>
          </div>
          {error !== null && <Failure error={error} />}{" "}
          {step === 1 ? (
            <div className="grid grid-cols-2 gap-3">
              {status?.top_up_amounts.map((v) => (
                <Button
                  key={v}
                  variant="outline"
                  className={`h-20 text-xl ${amount === v ? "border-primary bg-accent text-accent-foreground" : ""}`}
                  onClick={() => setAmount(v)}
                >
                  {dollars(v)}
                </Button>
              ))}
            </div>
          ) : (
            <div className="space-y-3">
              <div className="mb-5 flex justify-between rounded-lg bg-muted p-4 text-sm">
                <span>USD wallet credit</span>
                <strong>{dollars(amount)} USD</strong>
              </div>
              {providers.data?.map((p) => (
                <button
                  key={p.id}
                  type="button"
                  className={`flex w-full items-center gap-3 rounded-lg border p-4 text-left text-sm ${provider === p.id ? "border-primary bg-accent" : ""}`}
                  onClick={() => setProvider(p.id)}
                >
                  {p.crypto ? (
                    <Zap className="size-4" />
                  ) : (
                    <CreditCard className="size-4" />
                  )}
                  <span className="flex-1">
                    {p.name}
                    {p.crypto && (
                      <span className="mt-1 block text-xs text-muted-foreground">
                        The provider quotes USDT separately at checkout.
                      </span>
                    )}
                  </span>
                  {provider === p.id && (
                    <Check className="size-4 text-primary" />
                  )}
                </button>
              ))}
              {!providers.data?.length && (
                <Failure
                  error={
                    new Error(
                      "No payment providers are enabled. Contact the administrator.",
                    )
                  }
                />
              )}
            </div>
          )}
          <p className="text-xs leading-5 text-muted-foreground">
            Only a verified successful payment credits your wallet. Your browser
            redirect is not payment confirmation.
          </p>
          <DialogFooter>
            {step === 2 && (
              <Button
                variant="outline"
                disabled={busy}
                onClick={() => setStep(1)}
              >
                Back
              </Button>
            )}
            <Button
              disabled={busy || !amount || (step === 2 && !provider)}
              onClick={() => (step === 1 ? setStep(2) : void checkout())}
            >
              {busy
                ? "Creating checkout…"
                : step === 1
                  ? "Continue"
                  : "Continue to checkout"}
              <ArrowRight className="size-4" />
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
declare global {
  interface Window {
    Paddle?: {
      Environment: { set: (v: string) => void };
      Initialize: (v: unknown) => void;
      Checkout: { open: (v: unknown) => void };
    };
  }
}
async function launchPaddle(o: Order, token: string, sandbox: boolean) {
  if (!window.Paddle) {
    await new Promise<void>((resolve, reject) => {
      const script = document.createElement("script");
      script.src = "https://cdn.paddle.com/paddle/v2/paddle.js";
      script.onload = () => resolve();
      script.onerror = () =>
        reject(
          new Error(
            "Paddle checkout could not load. Open the order and retry.",
          ),
        );
      document.head.appendChild(script);
    });
  }
  if (!window.Paddle) throw new Error("Checkout unavailable");
  if (sandbox) window.Paddle.Environment.set("sandbox");
  window.Paddle.Initialize({ token });
  window.Paddle.Checkout.open({
    transactionId: o.provider_id,
    settings: { successUrl: location.origin + "/portal/wallet?order=" + o.id },
  });
}
export function Account() {
  const { session, refresh } = useSession();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const submit = async (e: FormEvent<HTMLFormElement>, path: string) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    const form = e.currentTarget;
    try {
      await api(
        path,
        path === "/account" ? "PATCH" : "POST",
        Object.fromEntries(new FormData(form)),
      );
      await refresh();
      toast.success("Account updated");
      if (path !== "/account") form.reset();
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };
  return (
    <>
      <Heading
        title="Account"
        description="Your personal details, verification, and account security."
      />
      {error !== null && (
        <div className="mb-6">
          <Failure error={error} />
        </div>
      )}
      <div className="grid gap-6 xl:grid-cols-2">
        <Card className="shadow-none">
          <CardHeader>
            <CardTitle>Personal information</CardTitle>
            <CardDescription>
              Your account has one wallet and one set of shared limits.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form
              onSubmit={(e) => void submit(e, "/account")}
              className="space-y-5"
            >
              <TextField
                label="Full name"
                name="name"
                defaultValue={session?.name}
                required
              />
              <Field label="Email address" id="account-email">
                <Input
                  id="account-email"
                  value={session?.email || ""}
                  readOnly
                />
                <div className="flex items-center justify-between">
                  <span className="text-xs text-muted-foreground">
                    {session?.email_verified
                      ? "Email verified"
                      : "Email verification pending"}
                  </span>
                  {!session?.email_verified && (
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      onClick={async () => {
                        try {
                          await api("/auth/resend-verification", "POST");
                          toast.success("Verification email sent");
                        } catch (e) {
                          toast.error(
                            e instanceof Error
                              ? e.message
                              : "Could not send email",
                          );
                        }
                      }}
                    >
                      Resend verification
                    </Button>
                  )}
                </div>
              </Field>
              <Submit busy={busy}>Save details</Submit>
            </form>
          </CardContent>
        </Card>
        <Card className="shadow-none">
          <CardHeader>
            <CardTitle>Change password</CardTitle>
            <CardDescription>
              Changing your password ends other active sessions.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form
              onSubmit={(e) => void submit(e, "/account/password")}
              className="space-y-5"
            >
              <TextField
                label="Current password"
                name="current_password"
                type="password"
                required
              />
              <Field
                label="New password"
                id="new-password"
                hint="Use 12–72 characters."
              >
                <Input
                  name="password"
                  id="new-password"
                  type="password"
                  required
                  minLength={12}
                  maxLength={72}
                  autoComplete="new-password"
                />
              </Field>
              <Submit busy={busy}>Update password</Submit>
            </form>
          </CardContent>
        </Card>
      </div>
    </>
  );
}
type DocRoute = {
  id: string;
  name: string;
  description: string;
  path_pattern: string;
  methods: string[];
  unit_cost: number;
  auth_required: boolean;
  example_request: string;
};
export function Docs() {
  const routes = useData<DocRoute[]>("/documentation");
  return (
    <>
      <Heading
        eyebrow="Developer resources"
        title="Build with the API"
        description="From your first request to production. Clear pricing and familiar HTTP conventions."
        action={
          <Button variant="outline" asChild>
            <a href="/api/v1/openapi.json" download>
              Download OpenAPI
              <ArrowUpRight className="size-4" />
            </a>
          </Button>
        }
      />
      <div className="grid gap-6 xl:grid-cols-[1.5fr_1fr]">
        <Card className="shadow-none">
          <CardHeader>
            <CardTitle>Make your first request</CardTitle>
            <CardDescription>
              Fund your wallet, create a key, and send it in the X-API-Key
              header.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-5">
            <CopyCode
              value={`curl ${location.origin}${routes.data?.[0]?.path_pattern || "/your-endpoint"} \\\n  -H 'X-API-Key: YOUR_API_KEY'`}
            />
            <p className="text-sm leading-6 text-muted-foreground">
              Keep your API key on your server. All keys share your account rate
              limits and prepaid balance.
            </p>
            <ActionLink to="/portal/keys">Manage API keys</ActionLink>
          </CardContent>
        </Card>
        <Card className="shadow-none">
          <CardHeader>
            <CardTitle>How billing works</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4 text-sm leading-6 text-muted-foreground">
            <p>
              Each route has a fixed unit cost. Included plan units are reserved
              first, then available wallet funds cover any remainder.
            </p>
            <p>
              Upstream responses from 2xx through 4xx are charged. Verified
              upstream 5xx and transport failures release their reservation.
            </p>
            <p>
              A customer disconnect alone does not prove an upstream failure.
              Unresolved requests remain reserved until reconciled.
            </p>
          </CardContent>
        </Card>
      </div>
      <Card className="mt-6 shadow-none">
        <CardHeader>
          <CardTitle>Available endpoints</CardTitle>
          <CardDescription>
            Fixed costs are known before your request is forwarded.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {routes.isPending ? (
            <Loading />
          ) : routes.error ? (
            <Failure error={routes.error} />
          ) : !routes.data?.length ? (
            <Empty
              title="Endpoints are being configured"
              description="Your gateway administrator can add API routes. Check back when the configuration is ready."
            />
          ) : (
            <div className="divide-y">
              {routes.data.map((r) => (
                <div key={r.id} className="py-5">
                  <div className="mb-2 flex flex-wrap items-center gap-3">
                    <Badge variant="outline">
                      {r.methods?.join(", ") || "ALL"}
                    </Badge>
                    <code className="text-sm font-medium">
                      {r.path_pattern}
                    </code>
                    <Badge variant="secondary" className="ml-auto">
                      {r.auth_required
                        ? `${r.unit_cost} units`
                        : "Free public route"}
                    </Badge>
                  </div>
                  <p className="text-sm font-medium">{r.name}</p>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {r.description}
                  </p>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
      <Card className="mt-6 shadow-none">
        <CardHeader>
          <CardTitle>Recovering from errors</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>HTTP status</TableHead>
                <TableHead>What it means</TableHead>
                <TableHead>Next step</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {[
                [
                  "401",
                  "Credential missing, expired, or revoked",
                  "Use an active API key.",
                ],
                [
                  "402",
                  "Not enough available units or funds",
                  "Add funds to your wallet.",
                ],
                [
                  "403",
                  "Access denied or account frozen",
                  "Check scope or contact the administrator.",
                ],
                [
                  "429",
                  "Account rate limit reached",
                  "Wait until the limit resets.",
                ],
                [
                  "503",
                  "Enforcement temporarily unavailable",
                  "Retry after dependencies recover.",
                ],
              ].map(([code, meaning, next]) => (
                <TableRow key={code}>
                  <TableCell className="font-mono">{code}</TableCell>
                  <TableCell>{meaning}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {next}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </>
  );
}
