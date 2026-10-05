import type * as Contract from "./contracts.generated";
export type Money = Contract.Money;
export type Session = Contract.Session & { id: string };
export type Term = Contract.Term;
export type Wallet = Contract.Wallet & { id: string };
export type Plan = Contract.Plan;
export type Order = Contract.Order;
export type Ledger = Contract.LedgerEntry;
export type APIKey = Contract.APIKey;
export type UsageDay = Contract.UsageDay;
export type UsageSummary = Contract.UsageSummary;
export type Provider = Contract.ProviderInfo;
export type Status = Contract.Installation;
export type Customer = Contract.Customer;
export type Reservation = Contract.Reservation;
export type AuditEntry = Contract.AuditEntry;
export class APIError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}
let csrf = "";
export function setCSRF(value: string) {
  csrf = value;
}
type Resource<T> = { id: string; attributes: T };
async function requestDocument(
  path: string,
  method = "GET",
  body?: unknown,
  operation?: string,
) {
  const response = await fetch(`/api/v1${path}`, {
    method,
    credentials: "same-origin",
    headers: {
      Accept: "application/vnd.api+json",
      "Content-Type": "application/vnd.api+json",
      ...(csrf ? { "X-CSRF-Token": csrf } : {}),
      ...(operation ? { "Idempotency-Key": operation } : {}),
    },
    ...(body === undefined
      ? {}
      : { body: JSON.stringify({ data: { attributes: body } }) }),
  });
  if (response.status === 204) return undefined;
  const doc = await response.json().catch(() => ({}));
  if (!response.ok)
    throw new APIError(
      response.status,
      doc.errors?.[0]?.detail ||
        doc.errors?.[0]?.title ||
        "This request could not be completed. Please try again.",
    );
  return doc;
}
export async function api<T>(
  path: string,
  method = "GET",
  body?: unknown,
  operation?: string,
): Promise<T> {
  const doc = await requestDocument(path, method, body, operation);
  if (doc === undefined) return undefined as T;
  const unwrap = (r: Resource<T>) => ({ id: r.id, ...r.attributes });
  return (
    Array.isArray(doc.data)
      ? doc.data.map(unwrap)
      : doc.data?.attributes
        ? unwrap(doc.data)
        : doc.meta || doc
  ) as T;
}
export type HistoryPage<T> = { data: T[]; meta: Contract.Pagination };
export async function apiPage<T>(
  path: string,
  page: number,
): Promise<HistoryPage<T>> {
  const params = new URLSearchParams({
    "page[number]": String(page),
    "page[size]": "50",
  });
  const doc = await requestDocument(
    `${path}${path.includes("?") ? "&" : "?"}${params}`,
  );
  if (!Array.isArray(doc?.data) || !doc.meta)
    throw new APIError(503, "History could not be loaded. Please retry.");
  return {
    data: doc.data.map((r: Resource<T>) => ({ id: r.id, ...r.attributes })),
    meta: doc.meta,
  };
}
export function operation() {
  return Array.from(crypto.getRandomValues(new Uint8Array(16)), (value) =>
    value.toString(16).padStart(2, "0"),
  ).join("");
}
// Integer arithmetic retains all six stored decimal places until display.
export function micros(value: Money) {
  const negative = value.startsWith("-");
  const [whole, fraction = ""] = value.replace(/^-/, "").split(".");
  const amount =
    BigInt(whole || "0") * 1_000_000n +
    BigInt(fraction.padEnd(6, "0").slice(0, 6));
  return negative ? -amount : amount;
}
export function dollars(value: Money, precision = 2) {
  const amount = micros(value);
  const negative = amount < 0;
  const absolute = negative ? -amount : amount;
  const whole = (absolute / 1_000_000n).toLocaleString("en-US");
  const fraction = (absolute % 1_000_000n)
    .toString()
    .padStart(6, "0")
    .slice(0, precision);
  return `${negative ? "-" : ""}$${whole}${precision ? "." + fraction : ""}`;
}
export function date(value?: string | null) {
  return value
    ? new Date(value).toLocaleDateString("en-US", {
        month: "short",
        day: "numeric",
        year: "numeric",
      })
    : "—";
}
