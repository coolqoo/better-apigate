import { Link, Navigate } from "react-router-dom";
import { ArrowRight, BookOpen, KeyRound, Wallet } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useSession } from "./context";
import { BrandFooter, Logo, ThemeToggle } from "./layout";
import { Failure, Loading } from "./shared";
import { PayWith } from "./payment-methods";

export function Home() {
  const { session, status, loading, error, refresh } = useSession();
  if (loading)
    return (
      <div className="p-10">
        <Loading />
      </div>
    );
  if (error)
    return (
      <div className="p-10">
        <Failure error={error} retry={() => void refresh()} />
      </div>
    );
  if (status?.setup_required) return <Navigate to="/setup" replace />;
  const workspace = session?.role === "admin" ? "/admin" : "/portal";
  return (
    <div className="flex min-h-screen flex-col">
      <header className="border-b bg-card">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-5 py-5 md:px-10">
          <Logo to="/" />
          <nav
            aria-label="Homepage"
            className="flex items-center gap-2 sm:gap-4"
          >
            <Link
              to="/docs"
              className="hidden text-sm text-muted-foreground hover:text-foreground sm:block"
            >
              API Docs
            </Link>
            <ThemeToggle />
            <Button size="sm" variant="outline" asChild>
              <Link to={session ? workspace : "/login"}>
                {session ? "Workspace" : "Sign in"}
              </Link>
            </Button>
          </nav>
        </div>
      </header>
      <main className="mx-auto w-full max-w-6xl flex-1 px-5 md:px-10">
        <section className="py-16 text-center md:py-24">
          <p className="mb-5 text-xs font-semibold uppercase tracking-[.16em] text-primary">
            {status?.app_name || "better-apigate"}
          </p>
          <h1 className="mx-auto max-w-2xl text-4xl font-semibold leading-tight tracking-tight md:text-6xl">
            Your API access.
            <br />
            <span className="text-primary">Simple, prepaid.</span>
          </h1>
          <p className="mx-auto mt-6 max-w-xl text-base leading-7 text-muted-foreground">
            Explore the API, manage your keys and keep track of every request.
            Fund your wallet when you’re ready.
          </p>
          <div className="mt-8 flex flex-wrap justify-center gap-3">
            <Button size="lg" asChild>
              <Link to={session ? workspace : "/signup"}>
                {session ? "Open your workspace" : "Get started"}
                <ArrowRight className="size-4" />
              </Link>
            </Button>
            <Button size="lg" variant="outline" asChild>
              <Link to="/docs">Explore API docs</Link>
            </Button>
          </div>
          <div className="mx-auto mt-14 grid max-w-3xl gap-5 text-left sm:grid-cols-3">
            {[
              {
                icon: BookOpen,
                title: "Explore the API",
                description:
                  "Endpoint documentation and ready-to-use request examples.",
              },
              {
                icon: KeyRound,
                title: "Manage your access",
                description: "Create API keys and follow your request usage.",
              },
              {
                icon: Wallet,
                title: "Stay in control",
                description:
                  "Prepaid plans and a wallet with clear usage costs.",
              },
            ].map(({ icon: Icon, title, description }) => (
              <div key={title} className="rounded-2xl border bg-card p-5">
                <Icon className="mb-4 size-5 text-primary" />
                <h2 className="text-sm font-semibold">{title}</h2>
                <p className="mt-2 text-xs leading-6 text-muted-foreground">
                  {description}
                </p>
              </div>
            ))}
          </div>
        </section>
        <PayWith selected={status?.payment_methods || []} />
      </main>
      <BrandFooter className="mx-auto w-full max-w-6xl px-5 md:px-10" />
    </div>
  );
}
