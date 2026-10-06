import { createContext, useContext, useEffect, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, APIError, setCSRF, type Session, type Status } from "./api";
const Context = createContext<{
  session?: Session;
  loading: boolean;
  status?: Status;
  error: unknown;
  refresh: () => Promise<void>;
}>({ loading: true, error: null, refresh: async () => {} });

function luminance(rgb: number[]) {
  const linear = rgb.map((v) => {
    v /= 255;
    return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
  });
  return linear[0] * 0.2126 + linear[1] * 0.7152 + linear[2] * 0.0722;
}
function contrast(a: number, b: number) {
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
}
// Keep brand-colored text readable on the light and dark surfaces.
function themeColor(hex: string, dark: boolean) {
  const original = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16));
  const surface = luminance(dark ? [38, 38, 38] : [250, 250, 250]);
  let rgb = original;
  for (let step = 0; step <= 20; step++) {
    rgb = original.map((v) =>
      Math.round(v + (((dark ? 255 : 0) - v) * step) / 20),
    );
    if (contrast(luminance(rgb), surface) >= 4.5) break;
  }
  const light = luminance(rgb);
  return {
    primary: `#${rgb.map((v) => v.toString(16).padStart(2, "0")).join("")}`,
    foreground:
      contrast(light, 1) >= contrast(light, 0) ? "#ffffff" : "#000000",
  };
}

export function SessionProvider({ children }: { children: ReactNode }) {
  const qc = useQueryClient();
  const status = useQuery({
    queryKey: ["status"],
    queryFn: () => api<Status>("/status"),
    retry: 1,
  });
  const session = useQuery({
    queryKey: ["session"],
    queryFn: async () => {
      try {
        const u = await api<Session>("/session");
        setCSRF(u.csrf_token);
        return u;
      } catch (e) {
        setCSRF("");
        if (e instanceof APIError && e.status === 401) return null;
        throw e;
      }
    },
    retry: 1,
  });
  useEffect(() => {
    document.title = `${status.data?.app_name || "better-apigate"} · Your API, under control`;
  }, [status.data?.app_name]);
  useEffect(() => {
    const root = document.documentElement;
    const color = status.data?.primary_color || "";
    if (!/^#[0-9a-f]{6}$/i.test(color)) {
      root.removeAttribute("data-brand-color");
      return;
    }
    for (const theme of ["light", "dark"] as const) {
      const colors = themeColor(color, theme === "dark");
      root.style.setProperty(`--brand-${theme}-primary`, colors.primary);
      root.style.setProperty(`--brand-${theme}-foreground`, colors.foreground);
    }
    root.setAttribute("data-brand-color", color);
  }, [status.data?.primary_color]);
  return (
    <Context.Provider
      value={{
        session: session.data || undefined,
        loading: status.isPending || session.isPending,
        status: status.data,
        error: status.error || session.error,
        refresh: async () => {
          await qc.invalidateQueries();
        },
      }}
    >
      {children}
    </Context.Provider>
  );
}
export function useSession() {
  return useContext(Context);
}
