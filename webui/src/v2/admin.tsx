import { useState, type FormEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  Activity,
  CreditCard,
  Plus,
  RefreshCw,
  Users,
  Wallet as WalletIcon,
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
import { Textarea } from "@/components/ui/textarea";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
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
  operation,
  type Customer,
  type AuditEntry,
  type Order,
  type Plan,
  type Reservation,
} from "./api";
import { useData } from "./customer";
import { useHistory, HistoryNavigation } from "./history";
import {
  ActionLink,
  Confirm,
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
interface AdminStats {
  customers: number;
  requests: number;
  errors: number;
  pending_reservations: number;
  wallet_liability: string;
  top_up_volume: string;
}
export function AdminOverview() {
  const q = useData<AdminStats>("/admin/overview");
  const orders = useData<Order[]>("/admin/orders");
  if (q.isPending) return <Loading />;
  if (q.error)
    return <Failure error={q.error} retry={() => void q.refetch()} />;
  const a = q.data!;
  return (
    <>
      <Heading
        eyebrow="Administration"
        title="Gateway overview"
        description="The health of your API business. Funding, traffic, and the actions that need your attention."
        action={
          <Button
            variant="outline"
            onClick={() => {
              void q.refetch();
              void orders.refetch();
            }}
          >
            <RefreshCw className="size-4" />
            Refresh
          </Button>
        }
      />
      <div className="mb-8 grid gap-5 sm:grid-cols-2 xl:grid-cols-4">
        <Metric
          label="Customers"
          value={a.customers.toLocaleString()}
          detail="Individual customer accounts"
          icon={<Users className="size-4" />}
        />
        <Metric
          label="Requests today"
          value={a.requests.toLocaleString()}
          detail={`${a.errors.toLocaleString()} error responses · last 24 hours`}
          icon={<Activity className="size-4" />}
        />
        <Metric
          label="Wallet liability"
          value={dollars(a.wallet_liability)}
          detail="Customer balances held in prepaid wallets"
          icon={<WalletIcon className="size-4" />}
        />
        <Metric
          label="Top-up volume"
          value={dollars(a.top_up_volume)}
          detail="Verified credits · last 30 days"
          icon={<CreditCard className="size-4" />}
        />
      </div>
      <div className="grid gap-6 xl:grid-cols-[1.6fr_1fr]">
        <Card className="shadow-none">
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle>Recent payments</CardTitle>
            <ActionLink to="/admin/payments">All payments</ActionLink>
          </CardHeader>
          <CardContent>
            {orders.error ? (
              <Failure error={orders.error} />
            ) : orders.data?.length ? (
              <OrderTable orders={orders.data.slice(0, 6)} />
            ) : (
              <Empty
                title="Ready for your first payment"
                description="Enable a payment provider in Settings so customers can fund their wallets."
                action={
                  <ActionLink to="/admin/settings">
                    Configure providers
                  </ActionLink>
                }
              />
            )}
          </CardContent>
        </Card>
        <div className="space-y-6">
          <Card className="shadow-none">
            <CardHeader>
              <CardTitle>Reconciliation</CardTitle>
              <CardDescription>
                Unresolved request holds need verified outcomes.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <p className="text-4xl font-semibold tabular-nums">
                {a.pending_reservations}
              </p>
              <p className="mb-5 mt-2 text-sm text-muted-foreground">
                Pending reservations, including requests currently in progress.
              </p>
              <ActionLink to="/admin/payments">
                Review older reservations
              </ActionLink>
            </CardContent>
          </Card>
          <Card className="shadow-none">
            <CardHeader>
              <CardTitle>Make the gateway yours</CardTitle>
            </CardHeader>
            <CardContent className="space-y-5">
              {[
                { title: "Connect an upstream", to: "/admin/configuration" },
                { title: "Set your pricing", to: "/admin/plans" },
                { title: "Enable payment providers", to: "/admin/settings" },
              ].map((a) => (
                <div
                  key={a.to}
                  className="flex items-center justify-between text-sm"
                >
                  <span>{a.title}</span>
                  <ActionLink to={a.to}>Configure</ActionLink>
                </div>
              ))}
            </CardContent>
          </Card>
        </div>
      </div>
    </>
  );
}
export function Customers() {
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const q = useHistory<Customer>(
    `/admin/customers${query ? "?q=" + encodeURIComponent(query) : ""}`,
  );
  const plans = useData<Plan[]>("/admin/plans");
  const qc = useQueryClient();
  const [selected, setSelected] = useState<Customer>();
  const [action, setAction] = useState("credit");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [op, setOp] = useState(operation);
  const submit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!selected) return;
    setBusy(true);
    setError(null);
    const f = new FormData(e.currentTarget);
    try {
      const path = `/admin/customers/${selected.id}`;
      if (action === "credit" || action === "debit")
        await api(
          path + "/adjustments",
          "POST",
          {
            amount: f.get("amount"),
            direction: action,
            reason: f.get("reason"),
          },
          op,
        );
      else if (action === "unfreeze")
        await api(path + "/unfreeze", "POST", { reason: f.get("reason") });
      else
        await api(path, "PATCH", { status: action, reason: f.get("reason") });
      setSelected(undefined);
      await qc.invalidateQueries();
      toast.success("Customer updated");
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };
  return (
    <>
      <Heading
        title="Customers"
        description="Manage individual accounts and reconcile balances with an auditable record."
      />
      <Card className="shadow-none">
        <CardHeader>
          <CardTitle>Customer accounts</CardTitle>
          <CardDescription>
            Suspension immediately prevents new paid requests.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form
            className="mb-5 flex flex-wrap gap-2"
            onSubmit={(event) => {
              event.preventDefault();
              q.setPage(1);
              setQuery(search.trim());
            }}
          >
            <Label htmlFor="customer-search" className="sr-only">
              Search customer name or email
            </Label>
            <Input
              id="customer-search"
              type="search"
              maxLength={200}
              className="max-w-sm"
              placeholder="Search name or email"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
            <Button type="submit" variant="outline">
              Search
            </Button>
            {query && (
              <Button
                type="button"
                variant="ghost"
                onClick={() => {
                  setSearch("");
                  setQuery("");
                }}
              >
                Clear
              </Button>
            )}
          </form>
          {q.isPending ? (
            <Loading />
          ) : q.error ? (
            <Failure error={q.error} />
          ) : !q.data?.length ? (
            <Empty
              title="No customers yet"
              description="Share your signup page to invite customers to the gateway."
            />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Customer</TableHead>
                  <TableHead>Plan</TableHead>
                  <TableHead>Balance</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {q.data.map((c) => (
                  <TableRow key={c.id}>
                    <TableCell>
                      <p className="font-medium">{c.name}</p>
                      <p className="mt-1 text-xs text-muted-foreground">
                        {c.email}
                      </p>
                    </TableCell>
                    <TableCell>
                      {plans.data?.find((plan) => plan.id === c.plan_id)
                        ?.name || "Current plan"}
                    </TableCell>
                    <TableCell>
                      {dollars(c.balance)}
                      {c.frozen && (
                        <p className="mt-1 text-xs text-destructive">
                          Frozen · {dollars(c.shortfall)} shortfall
                        </p>
                      )}
                    </TableCell>
                    <TableCell>
                      <StatusBadge state={c.status} />
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => {
                          setSelected(c);
                          setAction("credit");
                          setError(null);
                          setOp(operation());
                        }}
                      >
                        Manage
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
          <HistoryNavigation history={q} />
        </CardContent>
      </Card>
      <Dialog open={!!selected} onOpenChange={() => setSelected(undefined)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Manage {selected?.name}</DialogTitle>
            <DialogDescription>
              Financial and access changes require a reason and are recorded for
              audit.
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={submit} className="space-y-5">
            {error !== null && <Failure error={error} />}
            <Field label="Action" id="customer-action">
              <select
                id="customer-action"
                className="h-10 w-full rounded-md border bg-background px-3 text-sm"
                value={action}
                onChange={(e) => {
                  setAction(e.target.value);
                  setOp(operation());
                }}
              >
                <option value="credit">Credit wallet / cover shortfall</option>
                <option value="debit">Debit available wallet funds</option>
                {selected?.role === "user" && (
                  <>
                    <option value="suspended">Suspend paid access</option>
                    <option value="active">Restore account status</option>
                  </>
                )}
                <option value="unfreeze">
                  Unfreeze after shortfall is resolved
                </option>
              </select>
            </Field>
            {["credit", "debit"].includes(action) && (
              <TextField
                label="Amount (USD)"
                name="amount"
                required
                hint="Decimal USD amount, up to six decimal places."
              />
            )}
            <Field label="Audit reason" id="reason">
              <Textarea
                name="reason"
                id="reason"
                required
                placeholder="Explain the verified evidence and why this action is needed."
              />
            </Field>
            <p className="text-xs leading-5 text-muted-foreground">
              Debits only use available funds. Reversals and request holds
              remain recorded. Access can only be unfrozen after the shortfall
              is covered.
            </p>
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setSelected(undefined)}
              >
                Cancel
              </Button>
              <Submit busy={busy}>Confirm action</Submit>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}
function OrderTable({
  orders,
  onReconcile,
  onReverse,
}: {
  orders: Order[];
  onReconcile?: (o: Order) => void;
  onReverse?: (o: Order) => void;
}) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Order</TableHead>
          <TableHead>Provider</TableHead>
          <TableHead>Wallet credit</TableHead>
          <TableHead>Status</TableHead>
          {onReconcile && <TableHead className="text-right">Actions</TableHead>}
        </TableRow>
      </TableHeader>
      <TableBody>
        {orders.map((o) => (
          <TableRow key={o.id}>
            <TableCell>
              <p className="font-mono text-xs">{o.id.slice(0, 12)}</p>
              <p className="mt-1 text-xs text-muted-foreground">
                {date(o.created_at)}
              </p>
            </TableCell>
            <TableCell className="capitalize">{o.provider}</TableCell>
            <TableCell>
              {dollars(o.amount)}
              {o.crypto_amount && (
                <p className="mt-1 text-xs text-muted-foreground">
                  {o.crypto_amount} {o.crypto_token}
                </p>
              )}
            </TableCell>
            <TableCell>
              <StatusBadge state={o.state} />
            </TableCell>
            {onReconcile && (
              <TableCell className="text-right">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => onReconcile(o)}
                >
                  Reconcile
                </Button>
                {onReverse &&
                  (o.state === "paid" || o.state === "reversed") && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => onReverse(o)}
                    >
                      Record reversal
                    </Button>
                  )}
              </TableCell>
            )}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
export function Payments() {
  const orders = useHistory<Order>("/admin/orders");
  const pending = useHistory<Reservation>("/admin/reservations");
  const audit = useHistory<AuditEntry>("/admin/audit");
  const qc = useQueryClient();
  const [reversing, setReversing] = useState<Order>();
  const [reversalOperation, setReversalOperation] = useState("");
  const [selected, setSelected] = useState<Reservation>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const submit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!selected) return;
    setBusy(true);
    setError(null);
    const f = new FormData(e.currentTarget);
    try {
      await api("/admin/reservations/" + selected.id + "/resolve", "POST", {
        charge: f.get("outcome") === "charge",
        reason: f.get("reason"),
      });
      setSelected(undefined);
      await qc.invalidateQueries();
      toast.success("Reservation resolved");
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };
  return (
    <>
      <Heading
        title="Payments & reconciliation"
        description="Verified payments, unresolved requests, and the evidence behind every financial outcome."
        action={
          <Button variant="outline" onClick={() => void qc.invalidateQueries()}>
            <RefreshCw className="size-4" />
            Refresh
          </Button>
        }
      />
      <Tabs defaultValue="orders">
        <TabsList className="mb-6 grid h-auto w-full grid-cols-1 gap-1 sm:w-fit sm:grid-cols-3">
          <TabsTrigger value="orders">Payment orders</TabsTrigger>
          <TabsTrigger value="reservations">Unresolved requests</TabsTrigger>
          <TabsTrigger value="audit">Audit trail</TabsTrigger>
        </TabsList>
        <TabsContent value="orders">
          <Card className="shadow-none">
            <CardHeader>
              <CardTitle>Payment orders</CardTitle>
              <CardDescription>
                Reconciliation checks the provider’s authenticated API. Browser
                redirects cannot credit funds.
              </CardDescription>
            </CardHeader>
            <CardContent>
              {orders.isPending ? (
                <Loading />
              ) : orders.error ? (
                <Failure error={orders.error} />
              ) : orders.data?.length ? (
                <OrderTable
                  orders={orders.data}
                  onReverse={(o) => {
                    setError(null);
                    setReversalOperation(crypto.randomUUID());
                    setReversing(o);
                  }}
                  onReconcile={async (o) => {
                    try {
                      await api("/admin/orders/" + o.id + "/reconcile", "POST");
                      await qc.invalidateQueries();
                      toast.success("Provider status checked");
                    } catch (e) {
                      toast.error(
                        e instanceof Error
                          ? e.message
                          : "Reconciliation unavailable",
                      );
                    }
                  }}
                />
              ) : (
                <Empty
                  title="No orders yet"
                  description="Customer checkouts will appear here as pending, paid, expired, or reversed."
                />
              )}
              <HistoryNavigation history={orders} />
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="reservations">
          <Card className="shadow-none">
            <CardHeader>
              <CardTitle>Older pending reservations</CardTitle>
              <CardDescription>
                Holds older than five minutes are preserved until an outcome is
                established. Cache expiry never restores funds.
              </CardDescription>
            </CardHeader>
            <CardContent>
              {pending.isPending ? (
                <Loading />
              ) : pending.error ? (
                <Failure error={pending.error} />
              ) : pending.data?.length ? (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Reservation</TableHead>
                      <TableHead>Account</TableHead>
                      <TableHead>Units</TableHead>
                      <TableHead>Money held</TableHead>
                      <TableHead>Created</TableHead>
                      <TableHead />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {pending.data.map((r) => (
                      <TableRow key={r.id}>
                        <TableCell className="font-mono text-xs">
                          {r.id.slice(0, 12)}
                        </TableCell>
                        <TableCell className="font-mono text-xs">
                          {r.user_id.slice(0, 12)}
                        </TableCell>
                        <TableCell>{r.units}</TableCell>
                        <TableCell>{dollars(r.amount, 6)}</TableCell>
                        <TableCell>{date(r.created_at)}</TableCell>
                        <TableCell>
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => {
                              setSelected(r);
                              setError(null);
                            }}
                          >
                            Resolve
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              ) : (
                <Empty
                  title="Nothing needs reconciliation"
                  description="Older unresolved requests will appear here if a gateway crashes or an outcome remains unknown."
                />
              )}
              <HistoryNavigation history={pending} />
            </CardContent>
          </Card>
        </TabsContent>
        <TabsContent value="audit">
          <Card className="shadow-none">
            <CardHeader>
              <CardTitle>Administrator audit trail</CardTitle>
              <CardDescription>
                Review who changed access, pricing or funds, the affected
                record, and the evidence recorded at the time.
              </CardDescription>
            </CardHeader>
            <CardContent>
              {audit.isPending ? (
                <Loading />
              ) : audit.error ? (
                <Failure
                  error={audit.error}
                  retry={() => void audit.refetch()}
                />
              ) : audit.data?.length ? (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>When</TableHead>
                      <TableHead>Administrator</TableHead>
                      <TableHead>Action</TableHead>
                      <TableHead>Record</TableHead>
                      <TableHead>Reason & evidence</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {audit.data.map((entry) => (
                      <TableRow key={entry.id}>
                        <TableCell className="whitespace-nowrap">
                          {new Date(entry.created_at).toLocaleString("en-US")}
                        </TableCell>
                        <TableCell>
                          {entry.actor_email || entry.actor_id}
                        </TableCell>
                        <TableCell className="whitespace-nowrap">
                          {entry.action
                            .replaceAll(".", " · ")
                            .replaceAll("_", " ")}
                        </TableCell>
                        <TableCell
                          className="font-mono text-xs"
                          title={entry.target_id}
                        >
                          {entry.target_id}
                        </TableCell>
                        <TableCell className="min-w-64 max-w-xl whitespace-normal break-words">
                          {entry.reason}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              ) : (
                <Empty
                  title="No administrator actions yet"
                  description="Audited adjustments and reconciliation decisions will appear here."
                />
              )}
              <HistoryNavigation history={audit} />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
      <Dialog open={!!reversing} onOpenChange={() => setReversing(undefined)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Record a verified reversal</DialogTitle>
            <DialogDescription>
              Verify the external refund or chargeback first. This removes
              wallet credit and may freeze paid access if the funds were spent.
              It does not send funds through the provider.
            </DialogDescription>
          </DialogHeader>
          <form
            className="space-y-5"
            onSubmit={async (e) => {
              e.preventDefault();
              if (!reversing) return;
              const f = new FormData(e.currentTarget);
              setBusy(true);
              setError(null);
              try {
                await api(
                  "/admin/orders/" + reversing.id + "/reversals",
                  "POST",
                  { amount: f.get("amount"), reason: f.get("reason") },
                  reversalOperation,
                );
                setReversing(undefined);
                await qc.invalidateQueries();
                toast.success("Reversal recorded");
              } catch (err) {
                setError(err);
              } finally {
                setBusy(false);
              }
            }}
          >
            {error !== null && <Failure error={error} />}
            <Field label="USD credit to reverse" id="reversal-amount">
              <Input
                id="reversal-amount"
                name="amount"
                inputMode="decimal"
                required
                placeholder="10.00"
              />
            </Field>
            <Field
              label="Provider evidence and audit reason"
              id="reversal-reason"
            >
              <Textarea
                id="reversal-reason"
                name="reason"
                required
                minLength={10}
                placeholder="Provider refund ID, receipt and verified amount."
              />
            </Field>
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setReversing(undefined)}
              >
                Cancel
              </Button>
              <Submit busy={busy}>Confirm reversal</Submit>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
      <Dialog open={!!selected} onOpenChange={() => setSelected(undefined)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Resolve a request reservation</DialogTitle>
            <DialogDescription>
              This is a final accounting action. Confirm the upstream outcome
              before continuing.
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={submit} className="space-y-5">
            {error !== null && <Failure error={error} />}
            <Field label="Verified outcome" id="outcome">
              <select
                id="outcome"
                name="outcome"
                className="h-10 w-full rounded-md border bg-background px-3 text-sm"
              >
                <option value="charge">
                  Charge — upstream returned 2xx–4xx
                </option>
                <option value="release">
                  Release — verified upstream failure
                </option>
              </select>
            </Field>
            <Field label="Evidence and audit reason" id="resolve-reason">
              <Textarea
                id="resolve-reason"
                name="reason"
                required
                placeholder="Link to upstream logs or describe verified evidence."
              />
            </Field>
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setSelected(undefined)}
              >
                Cancel
              </Button>
              <Submit busy={busy}>Confirm resolution</Submit>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}
export function AdminPlans() {
  const q = useData<Plan[]>("/admin/plans");
  const qc = useQueryClient();
  const [selected, setSelected] = useState<Partial<Plan>>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const submit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    const f = new FormData(e.currentTarget);
    const body = {
      name: f.get("name"),
      description: f.get("description"),
      price: f.get("price"),
      unit_price: f.get("unit_price"),
      included_units: Number(f.get("included_units")),
      rate_limit_per_minute: Number(f.get("rate_limit_per_minute")),
      term_days: 30,
      is_base: f.get("is_base") === "on",
      enabled: f.get("enabled") === "on",
    };
    try {
      await api(
        "/admin/plans" + (selected?.id ? "/" + selected.id : ""),
        selected?.id ? "PATCH" : "POST",
        body,
      );
      setSelected(undefined);
      await qc.invalidateQueries();
      toast.success("Pricing saved");
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };
  return (
    <>
      <Heading
        title="Plans & Pricing"
        description="Set clear prepaid prices. Purchased terms keep a snapshot of their agreed pricing."
        action={
          <Button
            onClick={() => {
              setSelected({
                price: "25.000000",
                unit_price: "0.001000",
                included_units: 10000,
                rate_limit_per_minute: 600,
                enabled: true,
                is_base: false,
              });
              setError(null);
            }}
          >
            <Plus className="size-4" />
            Create plan
          </Button>
        }
      />
      <Card className="shadow-none">
        <CardHeader>
          <CardTitle>Plan catalog</CardTitle>
          <CardDescription>
            All terms last 30 days. Plan changes apply at renewal.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {q.isPending ? (
            <Loading />
          ) : q.error ? (
            <Failure error={q.error} />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Plan</TableHead>
                  <TableHead>Term price</TableHead>
                  <TableHead>Included units</TableHead>
                  <TableHead>Unit price</TableHead>
                  <TableHead>Rate limit</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead />
                </TableRow>
              </TableHeader>
              <TableBody>
                {q.data?.map((p) => (
                  <TableRow key={p.id}>
                    <TableCell>
                      <p className="font-medium">{p.name}</p>
                      <p className="mt-1 text-xs text-muted-foreground">
                        {p.is_base
                          ? "Base pay-as-you-go"
                          : "30-day prepaid term"}
                      </p>
                    </TableCell>
                    <TableCell>{dollars(p.price)}</TableCell>
                    <TableCell>{p.included_units.toLocaleString()}</TableCell>
                    <TableCell>{dollars(p.unit_price, 6)}</TableCell>
                    <TableCell>
                      {p.rate_limit_per_minute.toLocaleString()} / min
                    </TableCell>
                    <TableCell>
                      <StatusBadge state={p.enabled ? "active" : "disabled"} />
                    </TableCell>
                    <TableCell>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => {
                          setSelected(p);
                          setError(null);
                        }}
                      >
                        Edit
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
      <Dialog open={!!selected} onOpenChange={() => setSelected(undefined)}>
        <DialogContent className="max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {selected?.id ? "Edit pricing" : "Create a plan"}
            </DialogTitle>
            <DialogDescription>
              Enter prices in USD. Existing purchased terms retain their
              original prices.
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={submit} className="space-y-5">
            {error !== null && <Failure error={error} />}
            <TextField
              name="name"
              label="Plan name"
              required
              defaultValue={selected?.name}
            />
            <TextField
              name="description"
              label="Description"
              defaultValue={selected?.description}
            />
            <div className="grid grid-cols-2 gap-4">
              <TextField
                name="price"
                label="30-day price (USD)"
                required
                defaultValue={selected?.price}
              />
              <TextField
                name="unit_price"
                label="Additional unit price (USD)"
                required
                defaultValue={selected?.unit_price}
              />
              <TextField
                name="included_units"
                label="Included units"
                type="number"
                required
                defaultValue={String(selected?.included_units || 0)}
              />
              <TextField
                name="rate_limit_per_minute"
                label="Requests per minute"
                type="number"
                required
                defaultValue={String(selected?.rate_limit_per_minute || 600)}
              />
            </div>
            <div className="flex items-center gap-3">
              <input
                id="is_base"
                name="is_base"
                type="checkbox"
                defaultChecked={selected?.is_base}
              />
              <Label htmlFor="is_base">
                Base pay-as-you-go plan (price and quota must be zero)
              </Label>
            </div>
            <div className="flex items-center gap-3">
              <input
                id="enabled"
                name="enabled"
                type="checkbox"
                defaultChecked={selected?.enabled}
              />
              <Label htmlFor="enabled">Available for purchase</Label>
            </div>
            <DialogFooter>
              <Submit busy={busy}>Save pricing</Submit>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}
type ConfigField = {
  name: string;
  type: string;
  default?: unknown;
  required: boolean;
  internal?: boolean;
  implicit?: boolean;
  computed?: boolean;
  description?: string;
  values?: string[];
  ref?: string;
};
type ConfigSchema = { fields: ConfigField[]; module: string };
type ConfigRow = {
  id: string;
  name?: string;
  path_pattern?: string;
  base_url?: string;
  enabled?: boolean;
  unit_cost?: number;
  [key: string]: unknown;
};
const titles: Record<string, string> = {
  route: "Routes",
  upstream: "Upstreams",
  entitlement: "Features",
  plan_entitlement: "Plan features",
  webhook: "Webhooks",
};
const paths: Record<string, string> = {
  route: "routes",
  upstream: "upstreams",
  entitlement: "entitlements",
  plan_entitlement: "plan-entitlements",
  webhook: "webhooks",
};
const primaryFields: Record<string, string[]> = {
  upstream: [
    "name",
    "base_url",
    "description",
    "auth_type",
    "auth_header",
    "auth_reference",
    "enabled",
  ],
  route: [
    "name",
    "description",
    "path_pattern",
    "match_type",
    "upstream_id",
    "unit_cost",
    "auth_required",
    "enabled",
  ],
  entitlement: ["name", "display_name", "description", "enabled"],
  plan_entitlement: ["plan_id", "entitlement_id", "value", "enabled"],
  webhook: ["name", "user_id", "url", "events", "secret", "enabled"],
};
const fieldLabels: Record<string, string> = {
  base_url: "Service URL",
  path_pattern: "Request path",
  match_type: "Path matching",
  upstream_id: "Upstream service",
  unit_cost: "Units per request",
  auth_required: "Require an API key",
  auth_type: "Upstream authentication",
  auth_header: "Authentication header",
  auth_reference: "Credential environment variable",
  timeout_ms: "Request timeout (ms)",
  display_name: "Display name",
  plan_id: "Plan",
  entitlement_id: "Feature",
  user_id: "Customer ID",
  header_name: "Upstream header",
  protocol: "Response protocol",
  host_pattern: "Hostname pattern",
  max_idle_conns: "Idle connection limit",
};
function label(name: string) {
  return (
    fieldLabels[name] ||
    name.replaceAll("_", " ").replace(/\b\w/g, (v) => v.toUpperCase())
  );
}
function ConfigurationSection({ module }: { module: string }) {
  const base = "/admin/config/api/" + paths[module];
  const q = useData<ConfigRow[]>(base + "/");
  const schema = useData<ConfigSchema>("/admin/config/_schema/" + module);
  const upstreams = useData<ConfigRow[]>("/admin/config/api/upstreams/");
  const qc = useQueryClient();
  const [selected, setSelected] = useState<Partial<ConfigRow>>();
  const [remove, setRemove] = useState<ConfigRow>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const fields =
    schema.data?.fields
      .filter((f) => !f.implicit && !f.internal && !f.computed)
      .sort((a, b) => {
        const order = primaryFields[module] || [];
        const index = (f: ConfigField) =>
          order.includes(f.name) ? order.indexOf(f.name) : order.length;
        return index(a) - index(b) || a.name.localeCompare(b.name);
      }) || [];
  const submit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    const f = new FormData(e.currentTarget);
    const body: Record<string, unknown> = {};
    try {
      for (const field of fields) {
        const v = f.get(field.name);
        if (field.type === "bool") {
          body[field.name] = v === "on";
        } else if (v !== null && v !== "") {
          body[field.name] = ["int", "float"].includes(field.type)
            ? Number(v)
            : ["json", "strings"].includes(field.type)
              ? JSON.parse(String(v))
              : v;
        }
      }
      await api(
        base + (selected?.id ? "/" + selected.id : "/"),
        selected?.id ? "PATCH" : "POST",
        body,
      );
      setSelected(undefined);
      await qc.invalidateQueries();
      toast.success("Configuration saved");
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };
  const renderField = (field: ConfigField) => {
    const value = selected?.[field.name] ?? field.default;
    const id = "config-" + field.name;
    if (field.type === "bool")
      return (
        <div
          key={field.name}
          className="flex items-start gap-3 rounded-lg border p-4"
        >
          <input
            className="mt-1"
            id={id}
            name={field.name}
            type="checkbox"
            defaultChecked={Boolean(value)}
          />
          <div>
            <Label htmlFor={id}>{label(field.name)}</Label>
            <p className="mt-1 text-xs leading-5 text-muted-foreground">
              {field.name === "auth_required"
                ? "Uncheck only for explicitly free public routes. These requests require no prepaid funding."
                : field.description}
            </p>
          </div>
        </div>
      );
    return (
      <Field
        key={field.name}
        label={label(field.name)}
        id={id}
        hint={
          field.name === "auth_reference"
            ? "Use a reference such as ${UPSTREAM_API_TOKEN}. Set its value on every gateway instance."
            : field.description
        }
      >
        {field.type === "enum" ? (
          <select
            id={id}
            name={field.name}
            className="h-10 w-full rounded-md border bg-background px-3 text-sm"
            defaultValue={String(value || "")}
          >
            {field.values?.map((v) => (
              <option key={v} value={v}>
                {v || "None"}
              </option>
            ))}
          </select>
        ) : field.type === "ref" && field.ref === "upstream" ? (
          <select
            id={id}
            name={field.name}
            className="h-10 w-full rounded-md border bg-background px-3 text-sm"
            defaultValue={String(value || "")}
            required={field.required}
          >
            <option value="">Select an upstream</option>
            {upstreams.data?.map((u) => (
              <option value={u.id} key={u.id}>
                {u.name}
              </option>
            ))}
          </select>
        ) : ["json", "strings"].includes(field.type) ? (
          <Textarea
            id={id}
            name={field.name}
            className="min-h-24 font-mono text-xs"
            defaultValue={
              value === undefined || value === null
                ? ""
                : typeof value === "string"
                  ? value
                  : JSON.stringify(value, null, 2)
            }
            placeholder={
              field.name === "methods"
                ? '["GET", "POST"]'
                : "JSON configuration"
            }
          />
        ) : (
          <Input
            id={id}
            name={field.name}
            required={field.required}
            type={
              ["int", "float"].includes(field.type)
                ? "number"
                : field.name === "secret"
                  ? "password"
                  : "text"
            }
            min={field.name === "unit_cost" ? 1 : undefined}
            defaultValue={
              value === undefined || value === null ? "" : String(value)
            }
          />
        )}
      </Field>
    );
  };
  return (
    <>
      <Card className="shadow-none">
        <CardHeader className="flex flex-row items-start justify-between gap-4">
          <div>
            <CardTitle>{titles[module]}</CardTitle>
            <CardDescription className="mt-2">
              {module === "route"
                ? "Match endpoints, choose an upstream, and assign a fixed unit cost."
                : module === "upstream"
                  ? "Connect the backend services your gateway forwards requests to."
                  : "Control plan features and upstream entitlement headers."}
            </CardDescription>
          </div>
          <Button
            size="sm"
            onClick={() => {
              setSelected({});
              setError(null);
            }}
          >
            <Plus className="size-4" />
            Add{" "}
            {module === "plan_entitlement"
              ? "assignment"
              : module === "entitlement"
                ? "feature"
                : module}
          </Button>
        </CardHeader>
        <CardContent>
          {q.isPending ? (
            <Loading />
          ) : q.error ? (
            <Failure error={q.error} />
          ) : !q.data?.length ? (
            <Empty
              title={`No ${titles[module].toLowerCase()} configured`}
              description="Add your first configuration to get started."
            />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Configuration</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {q.data.map((row) => (
                  <TableRow key={row.id}>
                    <TableCell className="font-medium">
                      {row.name || String(row.plan_id || row.id).slice(0, 18)}
                    </TableCell>
                    <TableCell className="max-w-sm truncate font-mono text-xs text-muted-foreground">
                      {row.path_pattern
                        ? `${row.path_pattern} · ${row.unit_cost || 1} units`
                        : row.base_url ||
                          String(row.value || row.entitlement_id || "—")}
                    </TableCell>
                    <TableCell>
                      <StatusBadge
                        state={row.enabled ? "active" : "disabled"}
                      />
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          setSelected(row);
                          setError(null);
                        }}
                      >
                        Edit
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setRemove(row)}
                      >
                        Delete
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
      <Dialog open={!!selected} onOpenChange={() => setSelected(undefined)}>
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {selected?.id ? "Edit" : "Add"}{" "}
              {module === "plan_entitlement"
                ? "plan feature"
                : module === "entitlement"
                  ? "feature"
                  : module}
            </DialogTitle>
            <DialogDescription>
              Changes apply to new requests. Keep costs predictable and
              credentials in your deployment environment.
            </DialogDescription>
          </DialogHeader>
          <form onSubmit={submit} className="space-y-5">
            {error !== null && <Failure error={error} />}{" "}
            {schema.isPending ? (
              <Loading />
            ) : schema.error ? (
              <Failure error={schema.error} />
            ) : (
              <>
                {fields
                  .filter((field) =>
                    primaryFields[module]?.includes(field.name),
                  )
                  .map(renderField)}
                {fields.some(
                  (field) => !primaryFields[module]?.includes(field.name),
                ) && (
                  <details className="rounded-lg border p-4">
                    <summary className="cursor-pointer text-sm font-medium">
                      Advanced configuration
                    </summary>
                    <div className="mt-5 space-y-5">
                      {fields
                        .filter(
                          (field) =>
                            !primaryFields[module]?.includes(field.name),
                        )
                        .map(renderField)}
                    </div>
                  </details>
                )}
              </>
            )}
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setSelected(undefined)}
              >
                Cancel
              </Button>
              <Submit busy={busy}>Save configuration</Submit>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
      <Confirm
        open={!!remove}
        onOpenChange={() => setRemove(undefined)}
        title={`Delete this ${module.replace("_", " ")}?`}
        description="This change takes effect for new requests. Dependent records may need to be removed or reassigned first."
        label="Delete"
        destructive
        busy={busy}
        onConfirm={async () => {
          if (!remove) return;
          setBusy(true);
          try {
            await api(base + "/" + remove.id, "DELETE");
            setRemove(undefined);
            await qc.invalidateQueries();
            toast.success("Configuration deleted");
          } catch (e) {
            toast.error(e instanceof Error ? e.message : "Unable to delete");
          } finally {
            setBusy(false);
          }
        }}
      />
    </>
  );
}
export function Configuration() {
  return (
    <>
      <Heading
        title="API Configuration"
        description="Connect your services, shape requests, and make every endpoint’s cost predictable."
      />
      <Tabs defaultValue="upstream">
        <TabsList className="mb-6 flex h-auto flex-wrap justify-start">
          {Object.entries(titles).map(([key, title]) => (
            <TabsTrigger key={key} value={key}>
              {title}
            </TabsTrigger>
          ))}
        </TabsList>
        {Object.keys(titles).map((module) => (
          <TabsContent key={module} value={module}>
            <ConfigurationSection module={module} />
          </TabsContent>
        ))}
      </Tabs>
    </>
  );
}
const settingsGroups = [
  {
    id: "billing",
    title: "Billing preferences",
    description: "Amounts available in the customer top-up flow.",
    fields: [
      [
        "billing.top_up_amounts",
        "Top-up amounts (USD, comma separated)",
        "text",
      ],
      ["portal.app_name", "Application name", "text"],
      [
        "auth.require_verification",
        "Require email verification before funding",
        "bool",
      ],
    ],
  },
  {
    id: "stripe",
    title: "Stripe",
    description:
      "One-time Checkout sessions. Your webhook endpoint is /api/v1/payment-webhooks/stripe.",
    fields: [
      ["payment.stripe.enabled", "Enable Stripe", "bool"],
      ["payment.stripe.secret_key", "Secret API key", "password"],
      ["payment.stripe.webhook_secret", "Webhook signing secret", "password"],
    ],
  },
  {
    id: "paddle",
    title: "Paddle",
    description: "Non-recurring transactions with a one-time top-up product.",
    fields: [
      ["payment.paddle.enabled", "Enable Paddle", "bool"],
      ["payment.paddle.sandbox", "Use sandbox", "bool"],
      ["payment.paddle.api_key", "API key", "password"],
      ["payment.paddle.client_token", "Publishable client-side token", "text"],
      ["payment.paddle.topup_product_id", "One-time product ID", "text"],
      ["payment.paddle.webhook_secret", "Webhook signing secret", "password"],
    ],
  },
  {
    id: "lemonsqueezy",
    title: "Lemon Squeezy",
    description: "Use a one-time product variant with custom pricing.",
    fields: [
      ["payment.lemonsqueezy.enabled", "Enable Lemon Squeezy", "bool"],
      ["payment.lemonsqueezy.api_key", "API key", "password"],
      ["payment.lemonsqueezy.store_id", "Store ID", "text"],
      ["payment.lemonsqueezy.topup_variant_id", "One-time variant ID", "text"],
      [
        "payment.lemonsqueezy.webhook_secret",
        "Webhook signing secret",
        "password",
      ],
    ],
  },
  {
    id: "epusdt",
    title: "EPUSDT · USDT",
    description:
      "Current GMPay HMAC-SHA256 API. The USDT quote is separate from the USD wallet credit.",
    fields: [
      ["payment.epusdt.enabled", "Enable EPUSDT", "bool"],
      ["payment.epusdt.base_url", "Merchant server URL", "url"],
      ["payment.epusdt.pid", "Merchant PID", "text"],
      ["payment.epusdt.secret_key", "Signing secret", "password"],
      ["payment.epusdt.network", "Network (for example tron)", "text"],
    ],
  },
  {
    id: "email",
    title: "Email delivery",
    description:
      "Verification and password reset messages use your configured provider.",
    fields: [
      ["email.provider", "Provider (smtp, none)", "text"],
      ["email.from_address", "From email address", "email"],
      ["email.from_name", "From name", "text"],
      ["email.smtp.host", "SMTP host", "text"],
      ["email.smtp.port", "SMTP port", "text"],
      ["email.smtp.use_tls", "Require STARTTLS (typically port 587)", "bool"],
      ["email.smtp.username", "SMTP username", "text"],
      ["email.smtp.password", "SMTP password", "password"],
    ],
  },
];
export function SettingsPage() {
  const q = useData<{ values: Record<string, string> }>("/admin/settings");
  const qc = useQueryClient();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const submit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    const f = new FormData(e.currentTarget);
    const values: Record<string, string> = {};
    for (const group of settingsGroups)
      for (const [key, , type] of group.fields) {
        values[key] =
          type === "bool"
            ? f.get(key) === "on"
              ? "true"
              : "false"
            : String(f.get(key) || "");
      }
    try {
      await api("/admin/settings", "PATCH", { values });
      await qc.invalidateQueries();
      toast.success("Settings saved");
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };
  if (q.isPending) return <Loading />;
  if (q.error)
    return <Failure error={q.error} retry={() => void q.refetch()} />;
  return (
    <>
      <Heading
        title="Settings"
        description="Configure payments, billing preferences, and customer communication."
      />
      {error !== null && (
        <div className="mb-6">
          <Failure error={error} />
        </div>
      )}
      <form onSubmit={submit} className="space-y-6">
        {settingsGroups.map((group) => (
          <Card key={group.id} className="shadow-none">
            <CardHeader>
              <CardTitle>{group.title}</CardTitle>
              <CardDescription>{group.description}</CardDescription>
            </CardHeader>
            <CardContent className="grid gap-5 sm:grid-cols-2">
              {group.fields.map(([key, text, type]) =>
                type === "bool" ? (
                  <div
                    key={key}
                    className="flex items-center gap-3 sm:col-span-2"
                  >
                    <input
                      id={key}
                      name={key}
                      type="checkbox"
                      defaultChecked={
                        q.data?.values[key] === "true" ||
                        (key === "email.smtp.use_tls" &&
                          q.data?.values[key] === undefined)
                      }
                    />
                    <Label htmlFor={key}>{text}</Label>
                  </div>
                ) : (
                  <Field key={key} label={text} id={key}>
                    <Input
                      id={key}
                      name={key}
                      type={type}
                      defaultValue={
                        q.data?.values[key] ||
                        (key === "billing.top_up_amounts" ? "10,25,50,100" : "")
                      }
                      autoComplete="off"
                    />
                  </Field>
                ),
              )}
            </CardContent>
          </Card>
        ))}
        <div className="sticky bottom-4 flex items-center justify-between gap-4 rounded-xl border bg-card/95 p-4 shadow-sm backdrop-blur">
          <p className="text-xs text-muted-foreground">
            Saved secrets stay masked. Leave the mask to keep their value.
          </p>
          <Submit busy={busy}>Save settings</Submit>
        </div>
      </form>
    </>
  );
}
