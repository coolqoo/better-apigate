import { useState, type FormEvent } from "react";
import {
  Link,
  Navigate,
  useLocation,
  useNavigate,
  useSearchParams,
} from "react-router-dom";
import { ArrowRight, Check, ShieldCheck, Zap } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { api } from "./api";
import { useSession } from "./context";
import { Failure, Field, Loading, Submit } from "./shared";
import { BrandFooter, Logo, ThemeToggle } from "./layout";
export function AuthPage() {
  const location = useLocation();
  return <AuthForm key={location.pathname + location.search} />;
}

function AuthForm() {
  const { status, session, loading, error: loadError, refresh } = useSession();
  const location = useLocation();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [complete, setComplete] = useState(false);
  const mode = location.pathname.slice(1) || "login";
  const title: Record<string, string> = {
    login: "Welcome back",
    signup: "Build something great",
    setup: "Set up your gateway",
    "forgot-password": "Reset your password",
    "reset-password": "Choose a new password",
    verify: "Verify your email",
  };
  const descriptions: Record<string, string> = {
    login: "Sign in to your account to manage API access and billing.",
    signup:
      "Create your account. Add funds when you’re ready to make your first request.",
    setup: "Create the administrator account for this fresh deployment.",
    "forgot-password": "We’ll send you a link to get back into your account.",
    "reset-password": "Use a unique password with at least 12 characters.",
    verify: "Confirm your email to finish setting up your account.",
  };
  if (loading)
    return (
      <div className="p-12">
        <Loading />
      </div>
    );
  if (loadError)
    return (
      <div className="p-12">
        <Failure error={loadError} retry={() => void refresh()} />
      </div>
    );
  if (status?.setup_required && mode !== "setup")
    return <Navigate to="/setup" replace />;
  if (mode === "setup" && !status?.setup_required)
    return <Navigate to="/" replace />;
  if (session && ["login", "signup", "setup"].includes(mode))
    return (
      <Navigate to={session.role === "admin" ? "/admin" : "/portal"} replace />
    );
  const submit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    const data = Object.fromEntries(new FormData(e.currentTarget));
    try {
      const body =
        mode === "verify"
          ? { token: params.get("token") }
          : mode === "reset-password"
            ? { token: params.get("token"), password: data.password }
            : data;
      await api(
        `/auth/${mode === "forgot-password" ? "forgot-password" : mode === "reset-password" ? "reset-password" : mode === "verify" ? "verify" : mode}`,
        "POST",
        body,
      );
      if (["verify", "forgot-password", "reset-password"].includes(mode)) {
        setComplete(true);
      } else {
        await refresh();
        navigate(mode === "setup" ? "/admin" : "/portal");
      }
    } catch (e) {
      setError(e);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="relative min-h-screen">
      <div className="flex items-center justify-between px-6 py-6 md:px-12">
        <Logo />
        <ThemeToggle />
      </div>
      <div className="mx-auto grid max-w-6xl gap-12 px-6 py-10 md:grid-cols-2 md:gap-24 md:py-20">
        <div className="hidden self-center md:block">
          <div className="mb-8 inline-flex items-center gap-2 rounded-full border border-primary/15 bg-primary/5 px-3 py-1.5 text-xs font-medium text-primary">
            <Zap className="size-3.5" />A better way to power your API
          </div>
          <h1 className="max-w-md text-5xl font-semibold leading-[1.15] tracking-tight">
            One gateway.
            <br />
            <span className="text-primary">Every possibility.</span>
          </h1>
          <p className="mt-6 max-w-sm text-base leading-7 text-muted-foreground">
            A clear view of your usage, straightforward pricing, and API access
            that grows with you.
          </p>
          <div className="mt-10 space-y-5">
            {[
              "Know your costs before every request",
              "One wallet, flexible payment options",
              "Secure API keys and shared account limits",
            ].map((t) => (
              <div key={t} className="flex items-center gap-3 text-sm">
                <div className="rounded-full bg-primary/10 p-1">
                  <Check className="size-3 text-primary" />
                </div>
                {t}
              </div>
            ))}
          </div>
        </div>
        <Card className="self-start border shadow-sm">
          <CardContent className="p-7 sm:p-9">
            <h2 className="text-2xl font-semibold">{title[mode]}</h2>
            <p className="mb-7 mt-3 text-sm leading-6 text-muted-foreground">
              {descriptions[mode]}
            </p>
            {complete ? (
              <div className="space-y-5">
                <div className="rounded-xl bg-emerald-500/10 p-5 text-sm text-emerald-700 dark:text-emerald-300">
                  {mode === "forgot-password"
                    ? "If an account exists, a reset link is on its way. Check your inbox."
                    : mode === "verify"
                      ? "Your email is verified. You’re ready to continue."
                      : "Your password has been reset. Sign in with your new password."}
                </div>
                <Button asChild className="w-full">
                  <Link to="/login">
                    Continue to sign in
                    <ArrowRight className="size-4" />
                  </Link>
                </Button>
              </div>
            ) : (
              <form onSubmit={submit} className="space-y-5">
                {error !== null && <Failure error={error} />}{" "}
                {["setup", "signup"].includes(mode) && (
                  <Field label="Full name" id="name">
                    <Input
                      name="name"
                      id="name"
                      autoComplete="name"
                      required
                      maxLength={120}
                    />
                  </Field>
                )}
                {["setup", "signup", "login", "forgot-password"].includes(
                  mode,
                ) && (
                  <Field label="Email address" id="email">
                    <Input
                      name="email"
                      id="email"
                      type="email"
                      autoComplete="email"
                      placeholder="you@example.com"
                      required
                    />
                  </Field>
                )}
                {["setup", "signup", "login", "reset-password"].includes(
                  mode,
                ) && (
                  <Field
                    label="Password"
                    id="password"
                    hint={
                      mode === "login"
                        ? undefined
                        : "At least 12 characters. Use a password you don’t use elsewhere."
                    }
                  >
                    <Input
                      name="password"
                      id="password"
                      type="password"
                      autoComplete={
                        mode === "login" ? "current-password" : "new-password"
                      }
                      minLength={mode === "login" ? undefined : 12}
                      maxLength={72}
                      required
                    />
                  </Field>
                )}
                {mode === "setup" && (
                  <Field
                    label="Deployment setup token"
                    id="setup_token"
                    hint="Find APIGATE_SETUP_TOKEN in your deployment configuration."
                  >
                    <Input
                      name="setup_token"
                      id="setup_token"
                      type="password"
                      required
                      autoComplete="off"
                    />
                  </Field>
                )}
                {mode === "login" && (
                  <div className="text-right">
                    <Link
                      className="text-xs text-primary hover:underline"
                      to="/forgot-password"
                    >
                      Forgot password?
                    </Link>
                  </div>
                )}
                <div className="grid">
                  <Submit busy={busy}>
                    {mode === "login"
                      ? "Sign in"
                      : mode === "signup"
                        ? "Create account"
                        : mode === "setup"
                          ? "Create administrator"
                          : mode === "verify"
                            ? "Verify email"
                            : "Continue"}
                    <ArrowRight className="size-4" />
                  </Submit>
                </div>
              </form>
            )}
            {["login", "signup"].includes(mode) && (
              <p className="mt-6 text-center text-sm text-muted-foreground">
                {mode === "login" ? "New here?" : "Already have an account?"}{" "}
                <Link
                  className="font-medium text-primary hover:underline"
                  to={mode === "login" ? "/signup" : "/login"}
                >
                  {mode === "login" ? "Create an account" : "Sign in"}
                </Link>
              </p>
            )}
            <p className="mt-8 flex items-center justify-center gap-2 text-[11px] text-muted-foreground">
              <ShieldCheck className="size-3.5" />
              Your account is protected with a secure session.
            </p>
          </CardContent>
        </Card>
      </div>
      <BrandFooter className="mx-auto max-w-6xl px-6" />
    </div>
  );
}
