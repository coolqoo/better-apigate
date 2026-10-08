import {
  BrowserRouter,
  Link,
  Navigate,
  Outlet,
  Route,
  Routes,
} from "react-router-dom";
import { Toaster } from "sonner";
import { SessionProvider, useSession } from "./v2/context";
import { BrandFooter, Logo, ThemeToggle, Layout } from "./v2/layout";
import { AuthPage } from "./v2/auth";
import { Home } from "./v2/home";
import { AppErrorBoundary } from "./v2/error-boundary";
import { Failure, Loading } from "./v2/shared";
import { lazy, Suspense } from "react";
const Overview = lazy(() =>
  import("./v2/customer").then((m) => ({ default: m.Overview })),
);
const Keys = lazy(() =>
  import("./v2/customer").then((m) => ({ default: m.Keys })),
);
const Usage = lazy(() =>
  import("./v2/customer").then((m) => ({ default: m.Usage })),
);
const Plans = lazy(() =>
  import("./v2/customer").then((m) => ({ default: m.Plans })),
);
const WalletPage = lazy(() =>
  import("./v2/customer").then((m) => ({ default: m.WalletPage })),
);
const Account = lazy(() =>
  import("./v2/customer").then((m) => ({ default: m.Account })),
);
const Docs = lazy(() =>
  import("./v2/customer").then((m) => ({ default: m.Docs })),
);
const AdminOverview = lazy(() =>
  import("./v2/admin").then((m) => ({ default: m.AdminOverview })),
);
const Customers = lazy(() =>
  import("./v2/admin").then((m) => ({ default: m.Customers })),
);
const Configuration = lazy(() =>
  import("./v2/admin").then((m) => ({ default: m.Configuration })),
);
const AdminPlans = lazy(() =>
  import("./v2/admin").then((m) => ({ default: m.AdminPlans })),
);
const Payments = lazy(() =>
  import("./v2/admin").then((m) => ({ default: m.Payments })),
);
const SettingsPage = lazy(() =>
  import("./v2/admin").then((m) => ({ default: m.SettingsPage })),
);

function PublicDocs() {
  const { session } = useSession();
  if (session)
    return (
      <Layout>
        <Docs />
      </Layout>
    );
  return (
    <div className="min-h-screen">
      <header className="flex items-center justify-between border-b bg-card px-6 py-5 md:px-12">
        <Logo />
        <div className="flex items-center gap-5">
          <Link
            to={session ? "/portal" : "/login"}
            className="text-sm font-medium text-primary"
          >
            {session ? "Your workspace" : "Sign in"}
          </Link>
          <ThemeToggle />
        </div>
      </header>
      <main className="mx-auto max-w-7xl px-6 py-10 md:px-12">
        <Docs />
      </main>
      <BrandFooter className="mx-auto max-w-7xl px-6 md:px-12" />
    </div>
  );
}
function Guard({ admin = false }: { admin?: boolean }) {
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
  if (!session) return <Navigate to="/login" replace />;
  if (admin && session.role !== "admin")
    return <Navigate to="/portal" replace />;
  return <Outlet />;
}
export function App() {
  return (
    <BrowserRouter>
      <AppErrorBoundary>
        <SessionProvider>
          <Suspense
            fallback={
              <div className="p-10">
                <Loading />
              </div>
            }
          >
            <Routes>
              <Route path="/" element={<Home />} />
              {[
                "login",
                "signup",
                "setup",
                "forgot-password",
                "reset-password",
                "verify",
              ].map((path) => (
                <Route key={path} path={"/" + path} element={<AuthPage />} />
              ))}
              <Route path="/docs" element={<PublicDocs />} />
              <Route element={<Guard />}>
                <Route element={<Layout />}>
                  <Route path="/portal" element={<Overview />} />
                  <Route path="/portal/keys" element={<Keys />} />
                  <Route path="/portal/usage" element={<Usage />} />
                  <Route path="/portal/plans" element={<Plans />} />
                  <Route path="/portal/wallet" element={<WalletPage />} />
                  <Route path="/portal/account" element={<Account />} />

                  <Route element={<Guard admin />}>
                    <Route path="/admin" element={<AdminOverview />} />
                    <Route path="/admin/customers" element={<Customers />} />
                    <Route
                      path="/admin/configuration"
                      element={<Configuration />}
                    />
                    <Route path="/admin/plans" element={<AdminPlans />} />
                    <Route path="/admin/payments" element={<Payments />} />
                    <Route
                      path="/admin/payment-providers"
                      element={
                        <SettingsPage key="payments" section="payments" />
                      }
                    />
                    <Route
                      path="/admin/branding"
                      element={
                        <SettingsPage key="branding" section="branding" />
                      }
                    />
                    <Route
                      path="/admin/settings"
                      element={<SettingsPage key="general" section="general" />}
                    />
                  </Route>
                </Route>
              </Route>
              <Route path="*" element={<Navigate to="/portal" replace />} />
            </Routes>
          </Suspense>
          <Toaster richColors position="bottom-right" />
        </SessionProvider>
      </AppErrorBoundary>
    </BrowserRouter>
  );
}
