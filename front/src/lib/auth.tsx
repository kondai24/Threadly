import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { getApiMe, postApiAuthLogout } from "../orval/threadyAPI";
import type { InternalInterfaceControllersAuthResponse } from "../orval/threadyAPI.schemas";
import { AuthContext, type AuthContextValue } from "./auth-context";

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthContextValue["user"]>(null);
  const [isLoading, setIsLoading] = useState(true);

  const clearSession = useCallback(() => {
    setUser(null);
  }, []);

  const signOut = useCallback(() => {
    void postApiAuthLogout().catch(() => undefined);
    clearSession();
    setIsLoading(false);
  }, [clearSession]);

  useEffect(() => {
    let isCurrent = true;

    const loadUser = async () => {
      try {
        const me = await getApiMe();

        if (!isCurrent) return;

        setUser(me);
      } catch {
        if (!isCurrent) return;

        clearSession();
      } finally {
        if (isCurrent) {
          setIsLoading(false);
        }
      }
    };

    void loadUser();

    return () => {
      isCurrent = false;
    };
  }, [clearSession]);

  const setSession = useCallback(
    (response: InternalInterfaceControllersAuthResponse) => {
      setUser(response.user ?? null);
      setIsLoading(false);
    },
    [],
  );

  const value = useMemo(
    () => ({
      isLoading,
      isAuthenticated: Boolean(user),
      user,
      setSession,
      signOut,
    }),
    [isLoading, setSession, signOut, user],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
