import { useRef, useState, type FormEvent, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  ArrowRight,
  Check,
  Eye,
  EyeOff,
  Loader2,
  Play,
  Plus,
  Server,
  Settings2,
} from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
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
import { api } from "./api";
import type {
  ExprValidationResult,
  RouteTestResult,
  UpstreamHealth,
} from "./contracts.generated";
import { useData } from "./customer";
import {
  Confirm,
  CopyCode,
  Empty,
  Failure,
  Field,
  Loading,
  StatusBadge,
  Submit,
} from "./shared";

type Transform = {
  set_headers?: Record<string, string>;
  delete_headers?: string[];
  set_query?: Record<string, string>;
  delete_query?: string[];
  body_expr?: string;
};
type HeaderMatch = {
  Name: string;
  Value: string;
  IsRegex: boolean;
  Required: boolean;
};
type RouteConfig = {
  id?: string;
  name: string;
  description: string;
  path_pattern: string;
  match_type: string;
  methods: string[];
  headers: HeaderMatch[];
  host_pattern: string;
  host_match_type: string;
  upstream_id: string;
  path_rewrite: string;
  method_override: string;
  protocol: string;
  unit_cost: number;
  auth_required: boolean;
  priority: number;
  enabled: boolean;
  example_request: string;
  example_response: string;
  metering_mode: string;
  metering_expr: string;
  metering_unit: string;
  request_transform: Transform | null;
  response_transform: Transform | null;
};
type UpstreamConfig = {
  id?: string;
  name: string;
  description: string;
  base_url: string;
  auth_type: string;
  auth_header: string;
  auth_value: string;
  timeout_ms: number;
  max_idle_conns: number;
  idle_conn_timeout_ms: number;
  enabled: boolean;
};
const routeDefaults: RouteConfig = {
  name: "",
  description: "",
  path_pattern: "",
  match_type: "prefix",
  methods: [],
  headers: [],
  host_pattern: "",
  host_match_type: "",
  upstream_id: "",
  path_rewrite: "",
  method_override: "",
  protocol: "http",
  unit_cost: 1,
  auth_required: true,
  priority: 0,
  enabled: true,
  example_request: "",
  example_response: "",
  metering_mode: "request",
  metering_expr: "1",
  metering_unit: "requests",
  request_transform: null,
  response_transform: null,
};
const upstreamDefaults: UpstreamConfig = {
  name: "",
  description: "",
  base_url: "",
  auth_type: "none",
  auth_header: "",
  auth_value: "",
  timeout_ms: 30000,
  max_idle_conns: 100,
  idle_conn_timeout_ms: 90000,
  enabled: true,
};
const methodOptions = [
  "GET",
  "POST",
  "PUT",
  "PATCH",
  "DELETE",
  "HEAD",
  "OPTIONS",
];
const selectClass = "h-10 w-full rounded-md border bg-background px-3 text-sm";
const csv = (value: string) =>
  value
    .split(",")
    .map((v) => v.trim())
    .filter(Boolean);
function pairs(value: string) {
  const out: Record<string, string> = {};
  for (const line of value.split("\n")) {
    if (!line.trim()) continue;
    const at = line.indexOf("=");
    if (at < 1) throw new Error("Use one name=expression per line.");
    out[line.slice(0, at).trim()] = line.slice(at + 1).trim();
  }
  return out;
}
const pairText = (value?: Record<string, string>) =>
  Object.entries(value || {})
    .map(([k, v]) => `${k}=${v}`)
    .join("\n");
function Select({
  id,
  value,
  onChange,
  options,
  required,
}: {
  id: string;
  value: string;
  onChange: (value: string) => void;
  options: [string, string][];
  required?: boolean;
}) {
  return (
    <select
      id={id}
      className={selectClass}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      required={required}
    >
      {options.map(([v, title]) => (
        <option key={v} value={v}>
          {title}
        </option>
      ))}
    </select>
  );
}
function Toggle({
  id,
  title,
  description,
  checked,
  onChange,
}: {
  id: string;
  title: string;
  description: string;
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <label
      htmlFor={id}
      className="flex cursor-pointer items-start gap-3 rounded-xl border p-4"
    >
      <input
        id={id}
        type="checkbox"
        className="mt-1 size-4 accent-primary"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
      />
      <span>
        <span className="text-sm font-medium">{title}</span>
        <span className="mt-1 block text-xs leading-5 text-muted-foreground">
          {description}
        </span>
      </span>
    </label>
  );
}
function Section({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: ReactNode;
}) {
  return (
    <section className="space-y-5">
      <div>
        <h3 className="text-base font-semibold">{title}</h3>
        <p className="mt-1 text-sm leading-6 text-muted-foreground">
          {description}
        </p>
      </div>
      {children}
    </section>
  );
}
type Example = [string, string];
function Examples({
  items,
  apply,
}: {
  items: Example[];
  apply: (value: string) => void;
}) {
  return (
    <details className="rounded-xl border bg-muted/20">
      <summary className="cursor-pointer px-4 py-3 text-sm font-medium">
        Examples
      </summary>
      <div className="grid gap-2 p-3 pt-0 sm:grid-cols-2">
        {items.map(([name, value]) => (
          <button
            key={name}
            type="button"
            onClick={() => apply(value)}
            className="min-w-0 rounded-lg border bg-background p-3 text-left transition-colors hover:border-primary/50 focus-visible:outline-2 focus-visible:outline-primary"
          >
            <span className="flex items-center justify-between gap-2 text-xs font-medium">
              {name}
              <ArrowRight className="size-3 shrink-0" />
            </span>
            <code className="mt-2 block break-all whitespace-pre-wrap text-xs leading-5 text-muted-foreground">
              {value}
            </code>
          </button>
        ))}
      </div>
    </details>
  );
}
function Expression({
  id,
  title,
  value,
  onChange,
  context,
  examples = [],
  variables = [],
  hint,
}: {
  id: string;
  title: string;
  value: string;
  onChange: (v: string) => void;
  context: string;
  examples?: Example[];
  variables?: string[];
  hint?: string;
}) {
  const ref = useRef<HTMLTextAreaElement>(null);
  const [validation, setValidation] = useState<ExprValidationResult>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>();
  function change(v: string) {
    onChange(v);
    setValidation(undefined);
    setError(undefined);
  }
  return (
    <div className="space-y-3">
      <Field label={title} id={id} hint={hint}>
        <Textarea
          ref={ref}
          id={id}
          value={value}
          onChange={(e) => change(e.target.value)}
          className="min-h-20 font-mono text-xs leading-6"
          spellCheck={false}
        />
      </Field>
      <div className="flex flex-wrap items-center gap-2">
        {variables.map((v) => (
          <Button
            key={v}
            type="button"
            size="sm"
            variant="outline"
            className="h-7 px-2 font-mono text-xs"
            onClick={() => {
              const field = ref.current;
              const start = field?.selectionStart ?? value.length,
                end = field?.selectionEnd ?? value.length;
              change(value.slice(0, start) + v + value.slice(end));
              requestAnimationFrame(() => {
                field?.focus();
                field?.setSelectionRange(start + v.length, start + v.length);
              });
            }}
          >
            {v}
          </Button>
        ))}
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="ml-auto"
          disabled={busy}
          onClick={async () => {
            setBusy(true);
            setError(undefined);
            try {
              setValidation(
                await api<ExprValidationResult>(
                  "/admin/expressions/validate",
                  "POST",
                  { expression: value, context },
                ),
              );
            } catch (e) {
              setError(e);
            } finally {
              setBusy(false);
            }
          }}
        >
          {busy ? (
            <Loader2 className="size-3 animate-spin" />
          ) : (
            <Check className="size-3" />
          )}
          Validate
        </Button>
      </div>
      {validation && (
        <p
          role="status"
          className={`text-xs leading-5 ${validation.valid ? "text-emerald-700 dark:text-emerald-300" : "text-destructive"}`}
        >
          {validation.valid ? "Expression is valid" : validation.error}
        </p>
      )}
      {error != null && <Failure error={error} />}
      {examples.length > 0 && <Examples items={examples} apply={change} />}
    </div>
  );
}
const rewriteExamples: Example[] = [
  ["Change API version", '"/v2" + trimPrefix(path, "/v1")'],
  ["Add a prefix", '"/api" + path'],
  ["Remove a prefix", 'trimPrefix(path, "/proxy")'],
  ["Use a path parameter", '"/users/" + pathParams.id'],
];
const meterExamples: Example[] = [
  ["OpenAI response tokens", 'get(respBody, "usage.total_tokens") ?? 1'],
  ["SSE stream tokens", "json(sseLastData(allData)).usage.total_tokens ?? 1"],
  [
    "Anthropic stream tokens",
    "(json(sseEvents(allData)[0].data).message.usage.input_tokens ?? 0) + (json(sseEvents(allData)[count(sseEvents(allData))-2].data).usage.output_tokens ?? 0)",
  ],
  ["Response size in KB", "responseBytes / 1024"],
  ["Array item count", "len(respBody)"],
  ["Nested usage field", 'get(respBody, "data.credits_used") ?? 1'],
  ["Only successful responses", "status < 400 ? respBody.units : 0"],
  ["SSE event count", "count(sseEvents(allData))"],
];
function TransformFields({
  direction,
  value,
  onChange,
}: {
  direction: "request" | "response";
  value: Transform | null;
  onChange: (v: Transform | null) => void;
}) {
  const [headerText, setHeaderText] = useState(pairText(value?.set_headers));
  const [queryText, setQueryText] = useState(pairText(value?.set_query));
  const [error, setError] = useState<string>();
  function updateText(v: string, type: "set_headers" | "set_query") {
    if (type === "set_headers") setHeaderText(v);
    else setQueryText(v);
    try {
      onChange({ ...value, [type]: pairs(v) });
      setError(undefined);
    } catch (e) {
      setError((e as Error).message);
    }
  }
  const update = (key: keyof Transform, v: string | string[]) =>
    onChange({ ...value, [key]: v });
  return (
    <Section
      title={`${direction === "request" ? "Request" : "Response"} transform`}
      description={
        direction === "request"
          ? "Adjust headers, query parameters and the JSON body before forwarding."
          : "Adjust headers and the JSON body before returning the response to the customer. Body transforms apply to buffered HTTP responses."
      }
    >
      <Field
        label="Set headers"
        id={`${direction}-headers`}
        hint={
          'One name=expression per line. Quote literal values, for example X-Version="v2".'
        }
      >
        <Textarea
          id={`${direction}-headers`}
          className="min-h-24 font-mono text-xs"
          value={headerText}
          onChange={(e) => updateText(e.target.value, "set_headers")}
        />
      </Field>
      <Examples
        items={
          direction === "request"
            ? [
                [
                  "Authorization header",
                  'Authorization="Bearer " + env("API_KEY")',
                ],
                [
                  "Identity headers",
                  "X-User-ID=userID\nX-Key-ID=keyID\nX-Plan-ID=planID",
                ],
                ["JSON content type", 'Content-Type="application/json"'],
              ]
            : [
                ["Cache control", 'Cache-Control="no-store"'],
                ["Custom response header", 'X-Gateway="better-apigate"'],
              ]
        }
        apply={(v) => updateText(v, "set_headers")}
      />
      <Field
        label="Remove headers"
        id={`${direction}-remove-headers`}
        hint="Comma-separated header names."
      >
        <Input
          id={`${direction}-remove-headers`}
          value={value?.delete_headers?.join(", ") || ""}
          onChange={(e) => update("delete_headers", csv(e.target.value))}
          placeholder="X-Internal-Token, Server"
        />
      </Field>
      {direction === "request" && (
        <>
          <Field
            label="Set query parameters"
            id="request-query"
            hint="One name=expression per line."
          >
            <Textarea
              id="request-query"
              className="min-h-20 font-mono text-xs"
              value={queryText}
              onChange={(e) => updateText(e.target.value, "set_query")}
            />
          </Field>
          <Examples
            items={[
              ["API key in query", 'key=env("GEMINI_API_KEY")'],
              ["Force SSE", 'alt="sse"'],
              ["Fixed parameter", 'version="v2"'],
            ]}
            apply={(v) => updateText(v, "set_query")}
          />
          <Field
            label="Remove query parameters"
            id="request-remove-query"
            hint="Comma-separated parameter names."
          >
            <Input
              id="request-remove-query"
              value={value?.delete_query?.join(", ") || ""}
              onChange={(e) => update("delete_query", csv(e.target.value))}
            />
          </Field>
        </>
      )}
      {error && (
        <p
          role="alert"
          data-transform-error
          className="text-sm text-destructive"
        >
          {error}
        </p>
      )}
      <Expression
        id={`${direction}-body`}
        title="Body expression"
        context={direction}
        value={value?.body_expr || ""}
        onChange={(v) => update("body_expr", v)}
        hint="Return an object or array; the gateway serializes the result as JSON."
        variables={
          direction === "request"
            ? ["body", "headers", "query", "userID"]
            : ["respBody", "respHeaders", "status"]
        }
        examples={
          direction === "request"
            ? [
                [
                  "Wrap the request",
                  '{"wrapped": body, "metadata": {"user": userID}}',
                ],
                [
                  "Select request fields",
                  '{"model": body.model, "messages": body.messages}',
                ],
              ]
            : [
                ["Unwrap a response", "respBody.data"],
                ["Wrap the response", '{"data": respBody, "status": status}'],
              ]
        }
      />
    </Section>
  );
}
function RouteTest({ draft }: { draft?: RouteConfig }) {
  const [method, setMethod] = useState(draft?.methods?.[0] || "GET");
  const [path, setPath] = useState(
    (draft?.path_pattern || "/")
      .replace(/\*$/, "example")
      .replace(/\{[^}]+\}/g, "example"),
  );
  const [headers, setHeaders] = useState(
    draft?.host_pattern
      ? `Host=${draft.host_pattern.replace("*", "api")}`
      : "Content-Type=application/json",
  );
  const [body, setBody] = useState(draft?.example_request || "");
  const [responseBody, setResponseBody] = useState(
    draft?.example_response || '{"usage":{"total_tokens":42}}',
  );
  const [responseHeaders, setResponseHeaders] = useState(
    "Content-Type=application/json",
  );
  const [status, setStatus] = useState(200);
  const [result, setResult] = useState<RouteTestResult>();
  const [error, setError] = useState<unknown>();
  const [busy, setBusy] = useState(false);
  async function run(useDraft: boolean) {
    setBusy(true);
    setError(undefined);
    setResult(undefined);
    try {
      const previewDraft = draft
        ? Object.fromEntries(
            [
              "id",
              "name",
              "path_pattern",
              "match_type",
              "methods",
              "headers",
              "host_pattern",
              "host_match_type",
              "upstream_id",
              "path_rewrite",
              "method_override",
              "request_transform",
              "response_transform",
              "metering_expr",
              "metering_unit",
              "protocol",
              "unit_cost",
              "auth_required",
            ].map((key) => [key, draft[key as keyof RouteConfig]]),
          )
        : undefined;
      setResult(
        await api<RouteTestResult>("/admin/routes/test", "POST", {
          method,
          path,
          headers: pairs(headers),
          body,
          ...(useDraft && previewDraft ? { draft: previewDraft } : {}),
          response: {
            status,
            headers: pairs(responseHeaders),
            body: responseBody,
          },
        }),
      );
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Section
      title="Test matching & transforms"
      description="Preview locally using sample data. No upstream request is sent and no wallet funds are used."
    >
      <div className="grid gap-5 lg:grid-cols-2">
        <div className="space-y-4">
          <h4 className="text-sm font-medium">Sample request</h4>
          <div className="grid grid-cols-[110px_1fr] gap-3">
            <Field label="Method" id="test-method">
              <Select
                id="test-method"
                value={method}
                onChange={setMethod}
                options={methodOptions.map((v) => [v, v])}
              />
            </Field>
            <Field label="Path & query" id="test-path">
              <Input
                id="test-path"
                value={path}
                onChange={(e) => setPath(e.target.value)}
              />
            </Field>
          </div>
          <Field
            label="Request headers"
            id="test-headers"
            hint="One name=value per line. Include Host to test hostname matching."
          >
            <Textarea
              id="test-headers"
              className="font-mono text-xs"
              value={headers}
              onChange={(e) => setHeaders(e.target.value)}
            />
          </Field>
          <Field label="Request body" id="test-body">
            <Textarea
              id="test-body"
              className="min-h-32 font-mono text-xs"
              value={body}
              onChange={(e) => setBody(e.target.value)}
            />
          </Field>
        </div>
        <div className="space-y-4">
          <h4 className="text-sm font-medium">Sample upstream response</h4>
          <Field label="HTTP status" id="test-status">
            <Input
              id="test-status"
              type="number"
              min={100}
              max={599}
              value={status}
              onChange={(e) => setStatus(Number(e.target.value))}
            />
          </Field>
          <Field label="Response headers" id="test-response-headers">
            <Textarea
              id="test-response-headers"
              className="font-mono text-xs"
              value={responseHeaders}
              onChange={(e) => setResponseHeaders(e.target.value)}
            />
          </Field>
          <Field label="Response body / SSE data" id="test-response-body">
            <Textarea
              id="test-response-body"
              className="min-h-32 font-mono text-xs"
              value={responseBody}
              onChange={(e) => setResponseBody(e.target.value)}
            />
          </Field>
        </div>
      </div>
      <div className="flex flex-wrap gap-3">
        {draft && (
          <Button
            type="button"
            variant="outline"
            onClick={() => {
              setMethod(draft.methods[0] || "GET");
              setPath(
                draft.path_pattern
                  .replace(/\*$/, "example")
                  .replace(/\{[^}]+\}/g, "example"),
              );
              setBody(draft.example_request);
              const sample =
                draft.example_response || '{"usage":{"total_tokens":42}}';
              setResponseBody(
                draft.protocol === "sse" && !/(^|\n)(data|event):/.test(sample)
                  ? `data: ${sample.replaceAll("\n", "")}\n\ndata: [DONE]\n\n`
                  : sample,
              );
            }}
          >
            Use route examples
          </Button>
        )}
        {draft && (
          <Button type="button" disabled={busy} onClick={() => void run(true)}>
            <Play className="size-4" />
            Test current draft
          </Button>
        )}
        <Button
          type="button"
          variant={draft ? "outline" : "default"}
          disabled={busy}
          onClick={() => void run(false)}
        >
          {busy ? (
            <Loader2 className="size-4 animate-spin" />
          ) : (
            <Play className="size-4" />
          )}
          Match saved routes
        </Button>
      </div>
      {error != null && <Failure error={error} />}
      {result && (
        <div
          aria-live="polite"
          className="space-y-4 rounded-xl border bg-muted/20 p-5"
        >
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant={result.matched ? "default" : "secondary"}>
              {result.matched ? "Matched" : "No match"}
            </Badge>
            <span className="text-sm font-medium">{result.route_name}</span>
            <p className="text-xs text-muted-foreground">
              {result.match_reason}
            </p>
          </div>
          {result.error && <Failure error={new Error(result.error)} />}
          {result.matched && (
            <>
              <div className="rounded-lg border bg-background p-3">
                <p className="mb-1 text-xs text-muted-foreground">
                  Upstream · {result.upstream_name}
                </p>
                <code className="break-all text-xs">{result.upstream_url}</code>
              </div>
              <div className="grid gap-5 lg:grid-cols-2">
                <div>
                  <p className="mb-2 text-sm font-medium">
                    Transformed request
                  </p>
                  <CopyCode
                    value={`${result.transformed_method} ${result.transformed_path}${result.transformed_query ? "?" + result.transformed_query : ""}\n${Object.entries(
                      result.transformed_headers || {},
                    )
                      .map(([k, v]) => `${k}: ${v}`)
                      .join("\n")}\n\n${result.transformed_body || ""}`}
                  />
                </div>
                <div>
                  <p className="mb-2 text-sm font-medium">
                    Transformed response
                  </p>
                  <CopyCode
                    value={`HTTP ${result.response_status || status}\n${Object.entries(
                      result.response_headers || {},
                    )
                      .map(([k, v]) => `${k}: ${v}`)
                      .join("\n")}\n\n${result.response_body || ""}`}
                  />
                </div>
              </div>
              <div className="flex flex-wrap gap-3 text-xs">
                <Badge variant="outline">
                  Measured: {result.metering_sample ?? 0}{" "}
                  {result.metering_unit === "bytes"
                    ? "KB"
                    : result.metering_unit}
                </Badge>
                <Badge variant="outline">
                  Fixed prepaid cost:{" "}
                  {result.auth_required ? `${result.unit_cost} units` : "Free"}
                </Badge>
              </div>
            </>
          )}
        </div>
      )}
    </Section>
  );
}
function RouteEditor({
  initial,
  upstreams,
  close,
  saved,
  initialTab = "routing",
}: {
  initial: Partial<RouteConfig>;
  upstreams: UpstreamConfig[];
  close: () => void;
  saved: () => Promise<void>;
  initialTab?: string;
}) {
  const [draft, setDraft] = useState<RouteConfig>({
    ...(Object.fromEntries(
      Object.entries(routeDefaults).map(([key, value]) => [
        key,
        initial[key as keyof RouteConfig] ?? value,
      ]),
    ) as RouteConfig),
    id: initial.id,
  });
  const [tab, setTab] = useState(initialTab);
  const [error, setError] = useState<unknown>();
  const [busy, setBusy] = useState(false);
  function set<K extends keyof RouteConfig>(key: K, value: RouteConfig[K]) {
    setDraft((d) => ({ ...d, [key]: value }));
  }
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError(undefined);
    try {
      if (
        !draft.name.trim() ||
        !draft.path_pattern.trim() ||
        !draft.upstream_id
      ) {
        setTab("routing");
        throw new Error("Enter a route name, path pattern and upstream.");
      }
      if (e.currentTarget.querySelector("[data-transform-error]"))
        throw new Error("Fix the header or query format before saving.");
      if (!Number.isInteger(draft.unit_cost) || draft.unit_cost < 1) {
        setTab("metering");
        throw new Error("Units per request must be a positive whole number.");
      }
      const { id, ...values } = draft;
      const checks: [string, string][] = [
        [draft.path_rewrite, "rewrite"],
        [
          draft.metering_expr,
          draft.protocol === "http" ? "response" : "streaming",
        ],
      ];
      for (const direction of ["request", "response"] as const) {
        const transform = draft[`${direction}_transform`];
        if (transform)
          checks.push(
            [transform.body_expr || "", direction],
            ...Object.values(transform.set_headers || {}).map(
              (v) => [v, direction] as [string, string],
            ),
            ...Object.values(transform.set_query || {}).map(
              (v) => [v, direction] as [string, string],
            ),
          );
      }
      for (const [expression, context] of checks.filter(([v]) => v)) {
        const validation = await api<ExprValidationResult>(
          "/admin/expressions/validate",
          "POST",
          { expression, context },
        );
        if (!validation.valid)
          throw new Error(validation.error || "Invalid expression");
      }
      await api(
        `/admin/config/api/routes/${id || ""}`,
        id ? "PATCH" : "POST",
        values,
      );
      await saved();
      close();
      toast.success("Route saved");
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  }
  const tabs = [
    ["routing", "Routing"],
    ["docs", "Documentation"],
    ["metering", "Usage & pricing"],
    ["request", "Request transform"],
    ["response", "Response transform"],
    ["test", "Test"],
  ];
  return (
    <DialogContent className="flex max-h-[92dvh] flex-col gap-0 overflow-hidden p-0 sm:max-w-5xl">
      <DialogHeader className="shrink-0 border-b px-6 py-5 pr-12">
        <DialogTitle>{draft.id ? "Edit route" : "Create route"}</DialogTitle>
        <DialogDescription>
          Match requests, configure transforms and publish documentation.
        </DialogDescription>
      </DialogHeader>
      <form onSubmit={submit} noValidate className="flex min-h-0 flex-col">
        <Tabs value={tab} onValueChange={setTab} className="min-h-0 gap-0">
          <div className="shrink-0 overflow-x-auto border-b px-4">
            <TabsList variant="line" className="h-12 flex-nowrap">
              {tabs.map(([key, name]) => (
                <TabsTrigger key={key} value={key}>
                  {name}
                </TabsTrigger>
              ))}
            </TabsList>
          </div>
          <div className="min-h-0 overflow-y-auto p-5 sm:p-6">
            {error != null && (
              <div className="mb-5">
                <Failure error={error} />
              </div>
            )}
            <TabsContent
              value="routing"
              forceMount
              hidden={tab !== "routing"}
              className="space-y-6"
            >
              {!draft.id && (
                <details className="rounded-xl border bg-muted/20 p-4">
                  <summary className="cursor-pointer text-sm font-medium">
                    Start from an example
                  </summary>
                  <div className="mt-3 grid gap-2 sm:grid-cols-4">
                    {[
                      [
                        "Simple proxy",
                        {
                          path_pattern: "/api/*",
                          match_type: "prefix",
                          protocol: "http",
                          metering_expr: "1",
                        },
                      ],
                      [
                        "LLM endpoint",
                        {
                          path_pattern: "/v1/chat/completions",
                          match_type: "exact",
                          methods: ["POST"],
                          protocol: "sse",
                          metering_mode: "custom",
                          metering_unit: "tokens",
                          metering_expr:
                            "json(sseLastData(allData)).usage.total_tokens ?? 1",
                        },
                      ],
                      [
                        "SSE stream",
                        {
                          path_pattern: "/stream/*",
                          protocol: "sse",
                          metering_mode: "custom",
                          metering_unit: "data_points",
                          metering_expr: "count(sseEvents(allData))",
                        },
                      ],
                      [
                        "Version rewrite",
                        {
                          path_pattern: "/v1/*",
                          path_rewrite: rewriteExamples[0][1],
                        },
                      ],
                    ].map(([title, values]) => (
                      <Button
                        key={String(title)}
                        type="button"
                        variant="outline"
                        onClick={() =>
                          setDraft((d) => ({
                            ...d,
                            ...(values as Partial<RouteConfig>),
                          }))
                        }
                      >
                        {String(title)}
                      </Button>
                    ))}
                  </div>
                </details>
              )}
              <Section
                title="Match incoming requests"
                description="Routes with higher priority are checked first. Leave methods empty to match any HTTP method."
              >
                <div className="grid gap-5 sm:grid-cols-2">
                  <Field label="Route name" id="route-name">
                    <Input
                      id="route-name"
                      value={draft.name}
                      onChange={(e) => set("name", e.target.value)}
                      placeholder="Chat completions"
                    />
                  </Field>
                  <Field label="Path matching" id="route-match">
                    <Select
                      id="route-match"
                      value={draft.match_type}
                      onChange={(v) => set("match_type", v)}
                      options={[
                        ["exact", "Exact path"],
                        ["prefix", "Path prefix"],
                        ["regex", "Regular expression"],
                      ]}
                    />
                  </Field>
                  <Field
                    label="Path pattern"
                    id="route-path"
                    hint="Examples: /v1/chat, /api/*, /users/{id}"
                  >
                    <Input
                      id="route-path"
                      value={draft.path_pattern}
                      onChange={(e) => set("path_pattern", e.target.value)}
                      className="font-mono"
                    />
                  </Field>
                  <Field label="Priority" id="route-priority">
                    <Input
                      id="route-priority"
                      type="number"
                      value={draft.priority}
                      onChange={(e) => set("priority", Number(e.target.value))}
                    />
                  </Field>
                </div>
                <div>
                  <p className="mb-2 text-sm font-medium">HTTP methods</p>
                  <div className="flex flex-wrap gap-2">
                    {methodOptions.map((method) => (
                      <label
                        key={method}
                        className={`flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-xs ${draft.methods.includes(method) ? "border-primary/30 bg-primary/5" : ""}`}
                      >
                        <input
                          type="checkbox"
                          className="accent-primary"
                          checked={draft.methods.includes(method)}
                          onChange={(e) =>
                            set(
                              "methods",
                              e.target.checked
                                ? [...draft.methods, method]
                                : draft.methods.filter((v) => v !== method),
                            )
                          }
                        />
                        {method}
                      </label>
                    ))}
                  </div>
                </div>
                <details className="rounded-xl border p-4">
                  <summary className="cursor-pointer text-sm font-medium">
                    Host & header conditions
                  </summary>
                  <div className="mt-5 space-y-4">
                    <div className="grid gap-4 sm:grid-cols-2">
                      <Field label="Hostname pattern" id="route-host">
                        <Input
                          id="route-host"
                          value={draft.host_pattern}
                          onChange={(e) => set("host_pattern", e.target.value)}
                          placeholder="*.api.example.com"
                        />
                      </Field>
                      <Field label="Hostname matching" id="route-host-type">
                        <Select
                          id="route-host-type"
                          value={draft.host_match_type}
                          onChange={(v) => set("host_match_type", v)}
                          options={[
                            ["", "Any hostname"],
                            ["exact", "Exact"],
                            ["wildcard", "Wildcard"],
                            ["regex", "Regular expression"],
                          ]}
                        />
                      </Field>
                    </div>
                    {draft.headers.map((h, index) => (
                      <div
                        key={index}
                        className="grid items-center gap-2 rounded-lg border p-3 sm:grid-cols-[1fr_1fr_auto]"
                      >
                        <Input
                          aria-label={`Header ${index + 1} name`}
                          value={h.Name}
                          placeholder="X-API-Version"
                          onChange={(e) =>
                            set(
                              "headers",
                              draft.headers.map((v, i) =>
                                i === index
                                  ? { ...v, Name: e.target.value }
                                  : v,
                              ),
                            )
                          }
                        />
                        <Input
                          aria-label={`Header ${index + 1} value`}
                          value={h.Value}
                          placeholder="v2"
                          onChange={(e) =>
                            set(
                              "headers",
                              draft.headers.map((v, i) =>
                                i === index
                                  ? { ...v, Value: e.target.value }
                                  : v,
                              ),
                            )
                          }
                        />
                        <Button
                          type="button"
                          size="sm"
                          variant="ghost"
                          onClick={() =>
                            set(
                              "headers",
                              draft.headers.filter((_, i) => i !== index),
                            )
                          }
                        >
                          Remove
                        </Button>
                        <label className="flex gap-2 text-xs">
                          <input
                            type="checkbox"
                            checked={h.IsRegex}
                            onChange={(e) =>
                              set(
                                "headers",
                                draft.headers.map((v, i) =>
                                  i === index
                                    ? { ...v, IsRegex: e.target.checked }
                                    : v,
                                ),
                              )
                            }
                          />
                          Regex value
                        </label>
                        <label className="flex gap-2 text-xs">
                          <input
                            type="checkbox"
                            checked={h.Required}
                            onChange={(e) =>
                              set(
                                "headers",
                                draft.headers.map((v, i) =>
                                  i === index
                                    ? { ...v, Required: e.target.checked }
                                    : v,
                                ),
                              )
                            }
                          />
                          Header required
                        </label>
                      </div>
                    ))}
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      onClick={() =>
                        set("headers", [
                          ...draft.headers,
                          {
                            Name: "",
                            Value: "",
                            IsRegex: false,
                            Required: true,
                          },
                        ])
                      }
                    >
                      <Plus className="size-3" />
                      Add header condition
                    </Button>
                  </div>
                </details>
              </Section>
              <Section
                title="Forward to an upstream"
                description="Choose the backend and optionally change the path or method."
              >
                <Field
                  label="Upstream service"
                  id="route-upstream"
                  hint={
                    upstreams.length
                      ? undefined
                      : "Create an upstream in the Upstreams tab first."
                  }
                >
                  <Select
                    id="route-upstream"
                    value={draft.upstream_id}
                    onChange={(v) => set("upstream_id", v)}
                    options={[
                      ["", "Select an upstream"],
                      ...upstreams.map(
                        (u) =>
                          [u.id!, `${u.name} · ${u.base_url}`] as [
                            string,
                            string,
                          ],
                      ),
                    ]}
                  />
                </Field>
                <div className="grid gap-5 sm:grid-cols-2">
                  <Field label="Protocol" id="route-protocol">
                    <Select
                      id="route-protocol"
                      value={draft.protocol}
                      onChange={(v) => set("protocol", v)}
                      options={[
                        ["http", "HTTP · buffered response"],
                        ["http_stream", "HTTP stream · chunked response"],
                        ["sse", "SSE · server-sent events"],
                      ]}
                    />
                  </Field>
                  <Field label="Override HTTP method" id="route-method">
                    <Select
                      id="route-method"
                      value={draft.method_override}
                      onChange={(v) => set("method_override", v)}
                      options={[
                        ["", "Keep original method"],
                        ...methodOptions.map((v) => [v, v] as [string, string]),
                      ]}
                    />
                  </Field>
                </div>
                <Expression
                  id="route-rewrite"
                  title="Path rewrite expression"
                  value={draft.path_rewrite}
                  onChange={(v) => set("path_rewrite", v)}
                  context="rewrite"
                  hint="Leave empty to keep the original path."
                  examples={rewriteExamples}
                  variables={["path", "method", "pathParams"]}
                />
              </Section>
              <div className="grid gap-4 sm:grid-cols-2">
                <Toggle
                  id="route-enabled"
                  title="Enabled"
                  description="Accept matching requests through this route."
                  checked={draft.enabled}
                  onChange={(v) => set("enabled", v)}
                />
                <Toggle
                  id="route-auth"
                  title="Require an API key"
                  description="Authenticated requests use prepaid funding. Public routes are free."
                  checked={draft.auth_required}
                  onChange={(v) => set("auth_required", v)}
                />
              </div>
            </TabsContent>
            <TabsContent value="docs" forceMount hidden={tab !== "docs"}>
              <Section
                title="Customer API documentation"
                description="These details appear at /docs and in the downloadable OpenAPI specification."
              >
                <Field label="Description" id="route-description">
                  <Textarea
                    id="route-description"
                    value={draft.description}
                    onChange={(e) => set("description", e.target.value)}
                    placeholder="Explain what the endpoint does and the parameters it accepts."
                  />
                </Field>
                <div className="grid gap-5 lg:grid-cols-2">
                  <Field
                    label="Example request body"
                    id="route-example-request"
                  >
                    <Textarea
                      id="route-example-request"
                      className="min-h-64 font-mono text-xs"
                      value={draft.example_request}
                      onChange={(e) => set("example_request", e.target.value)}
                      placeholder={'{"message":"Hello"}'}
                    />
                  </Field>
                  <Field
                    label="Example response body"
                    id="route-example-response"
                  >
                    <Textarea
                      id="route-example-response"
                      className="min-h-64 font-mono text-xs"
                      value={draft.example_response}
                      onChange={(e) => set("example_response", e.target.value)}
                      placeholder={'{"result":"Hello back"}'}
                    />
                  </Field>
                </div>
              </Section>
            </TabsContent>
            <TabsContent
              value="metering"
              forceMount
              hidden={tab !== "metering"}
              className="space-y-6"
            >
              <Section
                title="Prepaid request pricing"
                description="This amount is reserved before forwarding. Included plan units are used first, followed by wallet funds."
              >
                <Field label="Units per request" id="route-cost">
                  <Input
                    id="route-cost"
                    type="number"
                    min={1}
                    step={1}
                    value={draft.unit_cost}
                    onChange={(e) => set("unit_cost", Number(e.target.value))}
                  />
                </Field>
              </Section>
              <Section
                title="Usage metering"
                description="Measure tokens, response size or custom usage for reporting. The fixed prepaid charge above stays independent."
              >
                <div className="grid gap-5 sm:grid-cols-2">
                  <Field label="Metering mode" id="route-meter-mode">
                    <Select
                      id="route-meter-mode"
                      value={draft.metering_mode}
                      onChange={(v) => {
                        set("metering_mode", v);
                        if (v !== "custom")
                          set(
                            "metering_expr",
                            v === "request"
                              ? "1"
                              : v === "bytes"
                                ? "responseBytes / 1024"
                                : draft.protocol === "sse"
                                  ? "json(sseLastData(allData)).usage.total_tokens ?? 1"
                                  : 'get(respBody, "usage.total_tokens") ?? 1',
                          );
                        if (v !== "custom")
                          set(
                            "metering_unit",
                            v === "request"
                              ? "requests"
                              : v === "bytes"
                                ? "bytes"
                                : "tokens",
                          );
                      }}
                      options={[
                        ["request", "Per request"],
                        ["response_field", "Response field"],
                        ["bytes", "Response size"],
                        ["custom", "Custom expression"],
                      ]}
                    />
                  </Field>
                  <Field label="Display unit" id="route-meter-unit">
                    <Select
                      id="route-meter-unit"
                      value={draft.metering_unit}
                      onChange={(v) => set("metering_unit", v)}
                      options={[
                        ["requests", "Requests"],
                        ["tokens", "Tokens"],
                        ["data_points", "Data points"],
                        ["bytes", "KB"],
                      ]}
                    />
                  </Field>
                </div>
                <Expression
                  id="route-meter-expr"
                  title="Metering expression"
                  value={draft.metering_expr}
                  onChange={(v) => set("metering_expr", v)}
                  context={draft.protocol === "http" ? "response" : "streaming"}
                  variables={
                    draft.protocol === "http"
                      ? ["respBody", "responseBytes", "status"]
                      : ["allData", "lastChunk", "responseBytes", "status"]
                  }
                  examples={meterExamples}
                />
              </Section>
            </TabsContent>
            <TabsContent value="request" forceMount hidden={tab !== "request"}>
              <TransformFields
                direction="request"
                value={draft.request_transform}
                onChange={(v) => set("request_transform", v)}
              />
            </TabsContent>
            <TabsContent
              value="response"
              forceMount
              hidden={tab !== "response"}
            >
              <TransformFields
                direction="response"
                value={draft.response_transform}
                onChange={(v) => set("response_transform", v)}
              />
            </TabsContent>
            <TabsContent value="test" forceMount hidden={tab !== "test"}>
              <RouteTest draft={draft} />
            </TabsContent>
          </div>
        </Tabs>
        <DialogFooter className="shrink-0 border-t bg-background px-6 py-4">
          <Button type="button" variant="outline" onClick={close}>
            Cancel
          </Button>
          <Submit busy={busy}>Save route</Submit>
        </DialogFooter>
      </form>
    </DialogContent>
  );
}
function UpstreamEditor({
  initial,
  close,
  saved,
}: {
  initial: Partial<UpstreamConfig>;
  close: () => void;
  saved: () => Promise<void>;
}) {
  const [draft, setDraft] = useState<UpstreamConfig>({
    ...(Object.fromEntries(
      Object.entries(upstreamDefaults).map(([key, value]) => [
        key,
        initial[key as keyof UpstreamConfig] ?? value,
      ]),
    ) as UpstreamConfig),
    id: initial.id,
  });
  const [show, setShow] = useState(false);
  const [error, setError] = useState<unknown>();
  const [busy, setBusy] = useState(false);
  function set<K extends keyof UpstreamConfig>(
    key: K,
    value: UpstreamConfig[K],
  ) {
    setDraft((d) => ({ ...d, [key]: value }));
  }
  return (
    <DialogContent className="max-h-[92dvh] overflow-y-auto sm:max-w-2xl">
      <DialogHeader>
        <DialogTitle>
          {draft.id ? "Edit upstream" : "Create upstream"}
        </DialogTitle>
        <DialogDescription>
          Connect a backend service and configure its authentication.
        </DialogDescription>
      </DialogHeader>
      <form
        className="space-y-6"
        onSubmit={async (e) => {
          e.preventDefault();
          setBusy(true);
          setError(undefined);
          try {
            const { id, ...values } = draft;
            await api(
              `/admin/config/api/upstreams/${id || ""}`,
              id ? "PATCH" : "POST",
              values,
            );
            await saved();
            close();
            toast.success("Upstream saved");
          } catch (e) {
            setError(e);
          } finally {
            setBusy(false);
          }
        }}
      >
        {error != null && <Failure error={error} />}
        <Section
          title="Service details"
          description="The request path is appended to this base URL."
        >
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Name" id="upstream-name">
              <Input
                id="upstream-name"
                required
                value={draft.name}
                onChange={(e) => set("name", e.target.value)}
              />
            </Field>
            <Field label="Service URL" id="upstream-url">
              <Input
                id="upstream-url"
                required
                type="url"
                placeholder="http://backend:3000"
                value={draft.base_url}
                onChange={(e) => set("base_url", e.target.value)}
              />
            </Field>
          </div>
          <Field label="Description" id="upstream-description">
            <Textarea
              id="upstream-description"
              value={draft.description}
              onChange={(e) => set("description", e.target.value)}
            />
          </Field>
        </Section>
        <Section
          title="Authentication"
          description="Each upstream has its own credential."
        >
          <Field label="Authentication type" id="upstream-auth">
            <Select
              id="upstream-auth"
              value={draft.auth_type}
              onChange={(v) => set("auth_type", v)}
              options={[
                ["none", "None"],
                ["header", "Custom header"],
                ["bearer", "Bearer token"],
                ["basic", "Basic authentication"],
              ]}
            />
          </Field>
          {draft.auth_type === "header" && (
            <Field label="Header name" id="upstream-header">
              <Input
                id="upstream-header"
                required
                value={draft.auth_header}
                onChange={(e) => set("auth_header", e.target.value)}
                placeholder="X-API-Key"
              />
            </Field>
          )}
          {draft.auth_type !== "none" && (
            <Field
              label="Authentication credential"
              id="upstream-credential"
              hint={
                draft.auth_type === "basic"
                  ? "Enter username:password. The gateway encodes it for Basic authentication."
                  : "Enter the credential directly. Leave empty to clear it."
              }
            >
              <div className="flex gap-2">
                <Input
                  id="upstream-credential"
                  type={show ? "text" : "password"}
                  value={draft.auth_value}
                  onChange={(e) => set("auth_value", e.target.value)}
                  autoComplete="off"
                />
                <Button
                  type="button"
                  size="icon"
                  variant="outline"
                  aria-label={show ? "Hide credential" : "Show credential"}
                  onClick={() => setShow(!show)}
                >
                  {show ? (
                    <EyeOff className="size-4" />
                  ) : (
                    <Eye className="size-4" />
                  )}
                </Button>
              </div>
            </Field>
          )}
          <div className="rounded-xl border bg-muted/20 p-4">
            <p className="mb-3 text-xs font-medium text-muted-foreground">
              Common configurations
            </p>
            <div className="flex flex-wrap gap-2">
              {[
                ["OpenAI", "bearer", ""],
                ["Anthropic", "header", "x-api-key"],
                ["Custom header", "header", "X-API-Key"],
                ["Google Gemini", "none", ""],
              ].map(([name, type, header]) => (
                <Button
                  key={name}
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() =>
                    setDraft((d) => ({
                      ...d,
                      auth_type: type,
                      auth_header: header,
                    }))
                  }
                >
                  {name}
                </Button>
              ))}
            </div>
            <p className="mt-3 text-xs leading-5 text-muted-foreground">
              For query-based authentication, use the route’s request transform
              to set the key parameter.
            </p>
          </div>
        </Section>
        <details className="rounded-xl border p-4">
          <summary className="cursor-pointer text-sm font-medium">
            Timeouts & connection pooling
          </summary>
          <div className="mt-5 grid gap-4 sm:grid-cols-2">
            {(
              [
                ["timeout_ms", "Request timeout (ms)"],
                ["max_idle_conns", "Maximum idle connections"],
                ["idle_conn_timeout_ms", "Idle connection timeout (ms)"],
              ] as const
            ).map(([key, title]) => (
              <Field key={key} label={title} id={`upstream-${key}`}>
                <Input
                  id={`upstream-${key}`}
                  type="number"
                  min={0}
                  value={draft[key]}
                  onChange={(e) => set(key, Number(e.target.value))}
                />
              </Field>
            ))}
          </div>
        </details>
        <Toggle
          id="upstream-enabled"
          title="Enabled"
          description="Make this upstream available for routing."
          checked={draft.enabled}
          onChange={(v) => set("enabled", v)}
        />
        <DialogFooter>
          <Button type="button" variant="outline" onClick={close}>
            Cancel
          </Button>
          <Submit busy={busy}>Save upstream</Submit>
        </DialogFooter>
      </form>
    </DialogContent>
  );
}
export function GatewayConfiguration({ kind }: { kind: "route" | "upstream" }) {
  const isRoute = kind === "route",
    path = `/admin/config/api/${isRoute ? "routes" : "upstreams"}/`;
  const q = useData<(RouteConfig & UpstreamConfig)[]>(path);
  const upstreams = useData<UpstreamConfig[]>("/admin/config/api/upstreams/");
  const qc = useQueryClient();
  const [editing, setEditing] =
    useState<Partial<RouteConfig & UpstreamConfig>>();
  const [editorTab, setEditorTab] = useState("routing");
  const [removing, setRemoving] = useState<RouteConfig & UpstreamConfig>();
  const [busy, setBusy] = useState(false);
  const [checking, setChecking] = useState<string>();
  const [health, setHealth] = useState<Record<string, UpstreamHealth>>({});
  const [testing, setTesting] = useState(false);
  const saved = async () => {
    await qc.invalidateQueries();
  };
  return (
    <>
      <Card className="shadow-none">
        <CardHeader className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <CardTitle>{isRoute ? "Routes" : "Upstreams"}</CardTitle>
            <CardDescription className="mt-2 max-w-xl leading-6">
              {isRoute
                ? "Match endpoints, transform requests and responses, measure usage and document your API."
                : "Manage the backend services your gateway forwards requests to."}
            </CardDescription>
          </div>
          <div className="flex flex-wrap gap-2">
            {isRoute && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => setTesting(true)}
              >
                <Play className="size-4" />
                Test routing
              </Button>
            )}
            <Button
              size="sm"
              onClick={() => {
                setEditorTab("routing");
                setEditing({});
              }}
            >
              <Plus className="size-4" />
              {isRoute ? "Add route" : "Add upstream"}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {q.isPending ? (
            <Loading />
          ) : q.error ? (
            <Failure error={q.error} retry={() => void q.refetch()} />
          ) : !q.data?.length ? (
            <Empty
              title={isRoute ? "No routes yet" : "Connect your first upstream"}
              description={
                isRoute
                  ? "Add a route to connect an incoming API path to a backend service."
                  : "Add a service URL and its credential, then create a route."
              }
            />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{isRoute ? "Route" : "Service"}</TableHead>
                  <TableHead>
                    {isRoute ? "Match & protocol" : "Authentication"}
                  </TableHead>
                  {isRoute && <TableHead>Pricing</TableHead>}
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {q.data.map((row) => (
                  <TableRow key={row.id}>
                    <TableCell>
                      <p className="font-medium">{row.name}</p>
                      <code className="mt-1 block max-w-72 truncate text-xs text-muted-foreground">
                        {isRoute ? row.path_pattern : row.base_url}
                      </code>
                      {!isRoute && health[row.id!] && (
                        <p
                          className={`mt-2 max-w-72 text-xs ${health[row.id!].reachable ? "text-emerald-700 dark:text-emerald-300" : "text-destructive"}`}
                        >
                          {health[row.id!].reachable
                            ? `Reached · HTTP ${health[row.id!].status_code} · ${health[row.id!].latency_ms} ms`
                            : health[row.id!].error}
                        </p>
                      )}
                    </TableCell>
                    <TableCell>
                      {isRoute ? (
                        <div className="flex flex-wrap gap-1">
                          <Badge variant="outline">
                            {row.methods?.join(", ") || "ALL"}
                          </Badge>
                          <Badge variant="secondary">{row.protocol}</Badge>
                          <span className="w-full text-xs text-muted-foreground">
                            {upstreams.data?.find(
                              (u) => u.id === row.upstream_id,
                            )?.name || "Unknown upstream"}{" "}
                            · priority {row.priority}
                          </span>
                        </div>
                      ) : (
                        <Badge variant="outline">
                          {row.auth_type === "header"
                            ? row.auth_header
                            : row.auth_type}
                        </Badge>
                      )}
                    </TableCell>
                    {isRoute && (
                      <TableCell className="text-xs">
                        {row.auth_required
                          ? `${row.unit_cost} units / request`
                          : "Free public route"}
                        <p className="mt-1 text-muted-foreground">
                          {row.metering_unit === "bytes"
                            ? "KB"
                            : row.metering_unit || "requests"}{" "}
                          metering
                        </p>
                      </TableCell>
                    )}
                    <TableCell>
                      <StatusBadge
                        state={row.enabled ? "active" : "disabled"}
                      />
                    </TableCell>
                    <TableCell>
                      <div className="flex flex-wrap justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => {
                            setEditorTab("routing");
                            setEditing(row);
                          }}
                        >
                          <Settings2 className="size-3" />
                          Edit
                        </Button>
                        {isRoute ? (
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => {
                              setEditorTab("test");
                              setEditing(row);
                            }}
                          >
                            Test
                          </Button>
                        ) : (
                          <Button
                            variant="ghost"
                            size="sm"
                            disabled={checking === row.id}
                            onClick={async () => {
                              setChecking(row.id);
                              try {
                                const result = await api<UpstreamHealth>(
                                  `/admin/upstreams/${row.id}/health`,
                                );
                                setHealth((h) => ({ ...h, [row.id!]: result }));
                              } catch (e) {
                                toast.error((e as Error).message);
                              } finally {
                                setChecking(undefined);
                              }
                            }}
                          >
                            {checking === row.id ? (
                              <Loader2 className="size-3 animate-spin" />
                            ) : (
                              <Server className="size-3" />
                            )}
                            Check
                          </Button>
                        )}
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={async () => {
                            try {
                              await api(
                                path +
                                  row.id +
                                  (row.enabled ? "/disable" : "/enable"),
                                "POST",
                              );
                              await saved();
                            } catch (e) {
                              toast.error((e as Error).message);
                            }
                          }}
                        >
                          {row.enabled ? "Disable" : "Enable"}
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="text-destructive"
                          onClick={() => setRemoving(row)}
                        >
                          Delete
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
      <Dialog
        open={editing !== undefined}
        onOpenChange={(open) => {
          if (!open) setEditing(undefined);
        }}
      >
        {editing !== undefined &&
          (isRoute ? (
            <RouteEditor
              initial={editing}
              upstreams={upstreams.data || []}
              initialTab={editorTab}
              close={() => setEditing(undefined)}
              saved={saved}
            />
          ) : (
            <UpstreamEditor
              initial={editing}
              close={() => setEditing(undefined)}
              saved={saved}
            />
          ))}
      </Dialog>
      <Dialog open={testing} onOpenChange={setTesting}>
        <DialogContent className="max-h-[92dvh] overflow-y-auto sm:max-w-4xl">
          <DialogHeader>
            <DialogTitle>Test routing</DialogTitle>
            <DialogDescription>
              Find which saved route matches a sample request.
            </DialogDescription>
          </DialogHeader>
          <RouteTest />
        </DialogContent>
      </Dialog>
      <Confirm
        open={!!removing}
        onOpenChange={() => setRemoving(undefined)}
        title={`Delete ${removing?.name || kind}?`}
        description={
          isRoute
            ? "Requests will stop matching this route. Its existing usage records remain available."
            : "Routes using this service will stop working. Reassign them before deleting the upstream."
        }
        label="Delete"
        destructive
        busy={busy}
        onConfirm={async () => {
          if (!removing) return;
          setBusy(true);
          try {
            await api(path + removing.id, "DELETE");
            await saved();
            setRemoving(undefined);
          } catch (e) {
            toast.error((e as Error).message);
          } finally {
            setBusy(false);
          }
        }}
      />
    </>
  );
}
