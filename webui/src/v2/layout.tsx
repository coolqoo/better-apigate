import { useState, type ReactNode } from "react";
import { Link, NavLink, Outlet, useLocation } from "react-router-dom";
import {
  Activity,
  ArrowUpRight,
  BookOpen,
  ChevronRight,
  CreditCard,
  KeyRound,
  LayoutDashboard,
  LogOut,
  Menu,
  Moon,
  Palette,
  Settings,
  Shield,
  SlidersHorizontal,
  Sun,
  Users,
  Wallet,
  Zap,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { Separator } from "@/components/ui/separator";
import { api } from "./api";
import { toast } from "sonner";
import { useSession } from "./context";
import { version } from "../../package.json";
const customer = [
  { path: "/portal", label: "Overview", icon: LayoutDashboard },
  { path: "/portal/keys", label: "API Keys", icon: KeyRound },
  { path: "/portal/usage", label: "Usage", icon: Activity },
  { path: "/portal/plans", label: "Plans", icon: Zap },
  { path: "/portal/wallet", label: "Wallet & Payments", icon: Wallet },
  { path: "/docs", label: "API Docs", icon: BookOpen },
  { path: "/portal/account", label: "Account", icon: Settings },
];
const admin = [
  { path: "/admin", label: "Overview", icon: LayoutDashboard },
  { path: "/admin/customers", label: "Customers", icon: Users },
  {
    path: "/admin/configuration",
    label: "API Configuration",
    icon: SlidersHorizontal,
  },
  { path: "/admin/plans", label: "Plans & Pricing", icon: Zap },
  { path: "/admin/payments", label: "Payments", icon: CreditCard },
  {
    path: "/admin/payment-providers",
    label: "Payment Providers",
    icon: Wallet,
  },
  { path: "/admin/branding", label: "Branding", icon: Palette },
  { path: "/admin/settings", label: "Settings", icon: Settings },
];
export function ThemeToggle() {
  const [dark, setDark] = useState(() =>
    document.documentElement.classList.contains("dark"),
  );
  return (
    <Button
      variant="ghost"
      size="icon"
      aria-label={dark ? "Switch to light theme" : "Switch to dark theme"}
      onClick={() => {
        document.documentElement.classList.toggle("dark", !dark);
        localStorage.setItem("apigate_theme", dark ? "light" : "dark");
        setDark(!dark);
      }}
    >
      {dark ? <Sun className="size-4" /> : <Moon className="size-4" />}
    </Button>
  );
}
export function Logo() {
  const { status } = useSession();
  const [failedLogo, setFailedLogo] = useState("");
  const name = status?.app_name || "better-apigate";
  const logo = /^https?:\/\//i.test(status?.logo_url || "")
    ? status?.logo_url
    : "";
  return (
    <Link
      to="/portal"
      className="flex min-w-0 items-center gap-2.5 font-semibold tracking-tight"
    >
      {logo && failedLogo !== logo ? (
        <img
          src={logo}
          alt=""
          className="size-8 shrink-0 rounded-lg object-contain"
          onError={() => setFailedLogo(logo)}
        />
      ) : (
        <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary text-primary-foreground">
          <Zap className="size-4" fill="currentColor" />
        </span>
      )}
      <span className="min-w-0">
        <span className="block truncate" title={name}>
          {name}
        </span>
        <span className="block text-[10px] font-medium tracking-normal text-muted-foreground">
          v{version}
        </span>
      </span>
    </Link>
  );
}
export function BrandFooter({ className = "" }: { className?: string }) {
  const { status } = useSession();
  const support = /^https?:\/\//i.test(status?.support_url || "")
    ? status?.support_url
    : "";
  return (
    <footer
      className={`flex flex-wrap items-center justify-between gap-3 py-7 text-xs text-muted-foreground ${className}`}
    >
      <span className="break-words">
        {status?.footer_text ||
          `${status?.app_name || "better-apigate"} · Your API, under control.`}
      </span>
      <div className="flex flex-wrap items-center gap-4">
        {support && (
          <a
            href={support}
            target="_blank"
            rel="noreferrer"
            className="hover:text-foreground"
          >
            Support <ArrowUpRight className="inline size-3" />
          </a>
        )}
        {status?.support_email && (
          <a
            href={`mailto:${encodeURIComponent(status.support_email)}`}
            className="break-all hover:text-foreground"
          >
            {status.support_email}
          </a>
        )}
        <Link to="/docs" className="hover:text-foreground">
          Documentation <ArrowUpRight className="inline size-3" />
        </Link>
      </div>
    </footer>
  );
}
function Navigation({
  adminArea,
  onNavigate,
}: {
  adminArea: boolean;
  onNavigate?: () => void;
}) {
  const { session } = useSession();
  const [signingOut, setSigningOut] = useState(false);
  return (
    <div className="flex h-full flex-col">
      <div className="px-6 py-7">
        <Logo />
      </div>
      <div className="mx-4 mb-6 rounded-lg border bg-background p-3">
        <p className="text-sm font-medium">
          {adminArea ? "Administration" : "Personal account"}
        </p>
        <p className="mt-1 truncate text-xs text-muted-foreground">
          {session?.email}
        </p>
      </div>
      <p className="px-6 text-[10px] font-semibold uppercase tracking-[.12em] text-muted-foreground">
        {adminArea ? "Manage your gateway" : "Your workspace"}
      </p>
      <nav
        className="mt-3 flex flex-col gap-1 px-3"
        aria-label={adminArea ? "Administration" : "Customer portal"}
      >
        {(adminArea ? admin : customer).map(({ path, label, icon: Icon }) => (
          <NavLink
            key={path}
            to={path}
            end={path === "/admin" || path === "/portal"}
            onClick={onNavigate}
            className={({ isActive }) =>
              `flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm transition-colors ${isActive ? "bg-accent font-medium text-accent-foreground" : "text-muted-foreground hover:bg-muted hover:text-foreground"}`
            }
          >
            <Icon className="size-[18px]" />
            {label}
          </NavLink>
        ))}
      </nav>
      <div className="mt-auto px-4 pb-5 pt-8">
        {session?.role === "admin" && (
          <Button
            variant="outline"
            className="mb-4 w-full justify-between"
            asChild
          >
            <Link to={adminArea ? "/portal" : "/admin"} onClick={onNavigate}>
              {adminArea ? "Customer portal" : "Administration"}
              <ArrowUpRight className="size-4" />
            </Link>
          </Button>
        )}
        <Separator />
        <div className="mt-4 flex items-center gap-3 px-1">
          <div className="flex size-9 shrink-0 items-center justify-center rounded-full bg-accent text-sm font-semibold text-accent-foreground">
            {session?.name?.slice(0, 2).toUpperCase() || "AG"}
          </div>
          <div className="min-w-0 flex-1">
            <p className="truncate text-xs font-medium">
              {session?.name || "Developer"}
            </p>
            <p className="mt-0.5 text-[11px] text-muted-foreground">
              {adminArea ? "Administrator" : "Individual account"}
            </p>
          </div>
          <Button
            aria-label="Sign out"
            disabled={signingOut}
            variant="ghost"
            size="icon"
            onClick={async () => {
              setSigningOut(true);
              try {
                await api("/auth/logout", "POST");
                window.location.assign("/login");
              } catch (error) {
                toast.error(
                  error instanceof Error
                    ? error.message
                    : "Sign out could not complete. Please retry.",
                );
                setSigningOut(false);
              }
            }}
          >
            <LogOut className="size-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}
export function Layout({ children }: { children?: ReactNode }) {
  const location = useLocation();
  const adminArea = location.pathname.startsWith("/admin");
  const [open, setOpen] = useState(false);
  const entry = [...admin, ...customer].find(
    (n) => n.path === location.pathname,
  );
  return (
    <div className="min-h-screen">
      <a
        href="#main-content"
        className="sr-only z-50 rounded bg-primary p-3 text-primary-foreground focus:not-sr-only focus:fixed"
      >
        Skip to content
      </a>
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-64 border-r bg-card lg:block">
        <Navigation adminArea={adminArea} />
      </aside>
      <div className="lg:pl-64">
        <header className="flex h-[72px] items-center justify-between border-b bg-card/80 px-5 md:px-10">
          <div className="flex items-center gap-3">
            <Sheet open={open} onOpenChange={setOpen}>
              <SheetTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="lg:hidden"
                  aria-label="Open navigation"
                >
                  <Menu className="size-5" />
                </Button>
              </SheetTrigger>
              <SheetContent side="left" className="w-72 p-0">
                <SheetTitle className="sr-only">Navigation</SheetTitle>
                <Navigation
                  adminArea={adminArea}
                  onNavigate={() => setOpen(false)}
                />
              </SheetContent>
            </Sheet>
            <span className="hidden text-sm text-muted-foreground sm:block">
              {adminArea ? "Administration" : "Workspace"}
            </span>
            <ChevronRight className="hidden size-3.5 text-muted-foreground sm:block" />
            <span className="text-sm font-medium">
              {entry?.label || "API Docs"}
            </span>
          </div>
          <div className="flex items-center gap-3">
            <span className="hidden items-center gap-1.5 text-xs text-muted-foreground sm:flex">
              <Shield className="size-3.5" />
              Prepaid billing
            </span>
            <ThemeToggle />
          </div>
        </header>
        <main
          id="main-content"
          className="mx-auto max-w-[1440px] px-5 py-8 md:px-10 md:py-10"
        >
          {children ?? <Outlet />}
        </main>
        <BrandFooter className="mx-auto max-w-[1440px] px-5 md:px-10" />
      </div>
    </div>
  );
}
