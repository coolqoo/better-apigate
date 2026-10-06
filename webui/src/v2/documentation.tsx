import { useState } from "react";
import { ArrowUpRight, Search } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { APIEndpoint } from "./contracts.generated";
import { useData } from "./customer";
import {
  ActionLink,
  CopyCode,
  Empty,
  Failure,
  Field,
  Heading,
  Loading,
} from "./shared";

function pretty(value: string) {
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return value;
  }
}
function shell(value: string) {
  return "'" + value.replaceAll("'", "'\"'\"'") + "'";
}
function Endpoint({ route: r }: { route: APIEndpoint }) {
  const methods = r.methods.length
    ? r.methods
    : ["GET", "POST", "PUT", "PATCH", "DELETE"];
  const [method, setMethod] = useState(methods[0]);
  const [path, setPath] = useState(
    r.match_type === "regex"
      ? "/your-endpoint"
      : r.path_pattern
          .replace(/\*$/, "YOUR_PATH")
          .replace(
            /\{([^}]+)\}/g,
            (_, name: string) => `YOUR_${name.toUpperCase()}`,
          ),
  );
  const [body, setBody] = useState(pretty(r.example_request));
  const url = location.origin + (path.startsWith("/") ? path : "/" + path);
  const hasBody = method !== "GET" && method !== "HEAD" && body.trim() !== "";
  const headers: Record<string, string> = {
    ...(r.auth_required ? { "X-API-Key": "YOUR_API_KEY" } : {}),
    ...(hasBody ? { "Content-Type": "application/json" } : {}),
  };
  const curl = `curl ${r.protocol === "sse" || r.protocol === "http_stream" ? "-N " : ""}-X ${method} ${shell(url)}${Object.entries(
    headers,
  )
    .map(([k, v]) => ` \\\n  -H ${shell(`${k}: ${v}`)}`)
    .join("")}${hasBody ? ` \\\n  --data ${shell(body)}` : ""}`;
  const js = `const response = await fetch(${JSON.stringify(url)}, {\n  method: ${JSON.stringify(method)},\n  headers: ${JSON.stringify(headers, null, 2).replaceAll("\n", "\n  ")}${hasBody ? `,\n  body: ${JSON.stringify(body)}` : ""}\n});\n${r.protocol === "sse" || r.protocol === "http_stream" ? "const reader = response.body.getReader();\nconst decoder = new TextDecoder();\nwhile (true) {\n  const { done, value } = await reader.read();\n  if (done) break;\n  console.log(decoder.decode(value, { stream: true }));\n}" : "console.log(await response.text());"}`;
  const python = `import requests\n\nresponse = requests.request(\n    ${JSON.stringify(method)},\n    ${JSON.stringify(url)},\n    headers=${JSON.stringify(headers, null, 4).replaceAll("\n", "\n    ")}${hasBody ? `,\n    data=${JSON.stringify(body)}` : ""}${r.protocol === "sse" || r.protocol === "http_stream" ? ",\n    stream=True" : ""}\n)\n${r.protocol === "sse" || r.protocol === "http_stream" ? "for line in response.iter_lines():\n    if line:\n        print(line.decode())" : "print(response.status_code)\nprint(response.text)"}`;
  return (
    <Card id={`endpoint-${r.id}`} className="scroll-mt-6 shadow-none">
      <CardHeader className="border-b">
        <div className="flex flex-wrap items-center gap-2">
          {methods.map((m) => (
            <Badge key={m} variant="outline" className="font-mono">
              {m}
            </Badge>
          ))}
          <Badge variant="secondary" className="ml-auto">
            {r.auth_required
              ? `${r.unit_cost} prepaid units / request`
              : "Free public route"}
          </Badge>
        </div>
        <CardTitle className="mt-3 text-xl">{r.name}</CardTitle>
        <code className="break-all text-sm text-muted-foreground">
          {r.path_pattern}
        </code>
        {r.description && (
          <CardDescription className="whitespace-pre-wrap leading-6">
            {r.description}
          </CardDescription>
        )}
        <div className="mt-2 flex flex-wrap gap-3 text-xs text-muted-foreground">
          <span>
            {r.auth_required ? "API key required" : "No API key required"}
          </span>
          <span>
            {r.protocol === "sse"
              ? "Server-sent events"
              : r.protocol === "http_stream"
                ? "Streaming HTTP"
                : "Buffered HTTP"}
          </span>
          <span>{r.match_type} matching</span>
        </div>
      </CardHeader>
      <CardContent className="space-y-6 pt-6">
        <div className="grid gap-5 lg:grid-cols-2">
          <div className="space-y-4">
            <div className="grid grid-cols-[110px_1fr] gap-3">
              <Field label="Method" id={`${r.id}-method`}>
                <select
                  id={`${r.id}-method`}
                  className="h-10 rounded-md border bg-background px-3 text-sm"
                  value={method}
                  onChange={(e) => setMethod(e.target.value)}
                >
                  {methods.map((m) => (
                    <option key={m}>{m}</option>
                  ))}
                </select>
              </Field>
              <Field label="Example request path" id={`${r.id}-path`}>
                <Input
                  id={`${r.id}-path`}
                  value={path}
                  onChange={(e) => setPath(e.target.value)}
                  className="font-mono text-xs"
                />
              </Field>
            </div>
            {r.example_request && (
              <div>
                <p className="mb-2 text-sm font-medium">Request body</p>
                <textarea
                  aria-label={`${r.name} example request body`}
                  className="min-h-40 w-full rounded-lg border bg-background p-3 font-mono text-xs leading-6"
                  value={body}
                  onChange={(e) => setBody(e.target.value)}
                />
              </div>
            )}
          </div>
          <div>
            {r.example_response ? (
              <>
                <p className="mb-2 text-sm font-medium">Example response</p>
                <CopyCode value={pretty(r.example_response)} />
              </>
            ) : (
              <div className="rounded-xl border border-dashed p-5 text-sm leading-6 text-muted-foreground">
                The upstream determines the response format. No response example
                has been published for this endpoint.
              </div>
            )}
          </div>
        </div>
        <Tabs defaultValue="curl">
          <TabsList>
            <TabsTrigger value="curl">cURL</TabsTrigger>
            <TabsTrigger value="javascript">JavaScript</TabsTrigger>
            <TabsTrigger value="python">Python</TabsTrigger>
          </TabsList>
          <TabsContent value="curl">
            <CopyCode value={curl} />
          </TabsContent>
          <TabsContent value="javascript">
            <CopyCode value={js} />
          </TabsContent>
          <TabsContent value="python">
            <CopyCode value={python} />
          </TabsContent>
        </Tabs>
        <p className="text-xs leading-5 text-muted-foreground">
          Replace YOUR_API_KEY and any path placeholders with your own values.
          {r.metering_unit &&
            ` Response usage is reported in ${r.metering_unit === "bytes" ? "KB" : r.metering_unit}.`}
        </p>
      </CardContent>
    </Card>
  );
}
export function Documentation() {
  const routes = useData<APIEndpoint[]>("/documentation");
  const [search, setSearch] = useState("");
  const filtered =
    routes.data?.filter((r) =>
      `${r.name} ${r.path_pattern} ${r.description}`
        .toLowerCase()
        .includes(search.toLowerCase()),
    ) || [];
  return (
    <>
      <Heading
        eyebrow="Developer resources"
        title="API documentation"
        description="Explore your endpoints, review examples and copy a request in your preferred language."
        action={
          <Button variant="outline" asChild>
            <a href="/api/v1/openapi.json" download>
              Download OpenAPI
              <ArrowUpRight className="size-4" />
            </a>
          </Button>
        }
      />
      <div className="mb-8 grid gap-5 lg:grid-cols-2">
        <Card className="shadow-none">
          <CardHeader>
            <CardTitle>Authentication</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4 text-sm leading-6 text-muted-foreground">
            <p>
              Send your API key in the{" "}
              <code className="text-foreground">X-API-Key</code> header. Public
              endpoints are marked as free and do not require a key.
            </p>
            <CopyCode value="X-API-Key: YOUR_API_KEY" />
            <ActionLink to="/portal/keys">Manage API keys</ActionLink>
          </CardContent>
        </Card>
        <Card className="shadow-none">
          <CardHeader>
            <CardTitle>Prepaid usage</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm leading-6 text-muted-foreground">
            <p>
              Each authenticated request reserves its fixed route cost. Included
              plan units are used first; wallet funds cover the remainder.
            </p>
            <p>
              Upstream 2xx–4xx responses are charged. Verified upstream 5xx and
              transport failures release the reservation.
            </p>
            <p>
              Response metering reports tokens, data points or response size
              separately from the prepaid charge.
            </p>
            <ActionLink to="/portal/wallet">Wallet & payments</ActionLink>
          </CardContent>
        </Card>
      </div>
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-xl font-semibold">
          Endpoints{" "}
          <span className="ml-2 text-sm font-normal text-muted-foreground">
            {routes.data?.length || 0}
          </span>
        </h2>
        <div className="relative w-full sm:w-72">
          <Search className="absolute left-3 top-3 size-4 text-muted-foreground" />
          <Input
            aria-label="Search endpoints"
            placeholder="Search endpoints…"
            className="pl-9"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
      </div>
      {routes.isPending ? (
        <Loading />
      ) : routes.error ? (
        <Failure error={routes.error} retry={() => void routes.refetch()} />
      ) : !filtered.length ? (
        <Empty
          title={
            search ? "No matching endpoints" : "No endpoints published yet"
          }
          description={
            search
              ? "Try another name or request path."
              : "Your administrator can publish routes and examples from API Configuration."
          }
        />
      ) : (
        <div className="space-y-6">
          {filtered.map((r) => (
            <Endpoint key={r.id} route={r} />
          ))}
        </div>
      )}
      <Card className="mt-8 shadow-none">
        <CardHeader>
          <CardTitle>HTTP errors</CardTitle>
          <CardDescription>
            Use the status code to decide what to do next.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {[
              ["401", "Missing or revoked API key", "Use an active key."],
              [
                "402",
                "Insufficient prepaid funding",
                "Add wallet funds or buy a plan.",
              ],
              [
                "403",
                "Access denied",
                "Check permissions or contact your administrator.",
              ],
              [
                "429",
                "Rate limit reached",
                "Wait until the account limit resets.",
              ],
              [
                "502",
                "Upstream unavailable",
                "Retry after the upstream recovers.",
              ],
              [
                "503",
                "Enforcement unavailable",
                "Retry after gateway dependencies recover.",
              ],
            ].map(([status, title, detail]) => (
              <div key={status} className="rounded-xl border p-4">
                <Badge variant="outline" className="font-mono">
                  {status}
                </Badge>
                <p className="mt-3 text-sm font-medium">{title}</p>
                <p className="mt-1 text-xs leading-5 text-muted-foreground">
                  {detail}
                </p>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </>
  );
}
