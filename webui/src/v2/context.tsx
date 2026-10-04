import { createContext, useContext, type ReactNode } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, APIError, setCSRF, type Session, type Status } from "./api";
const Context = createContext<{
  session?: Session;
  loading: boolean;
  status?: Status;
  error: unknown;
  refresh: () => Promise<void>;
}>({ loading: true, error: null, refresh: async () => {} });
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
