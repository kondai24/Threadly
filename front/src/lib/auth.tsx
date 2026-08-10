import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { getApiMe } from "../orval/threadyAPI";
import type { InternalInterfaceControllersAuthResponse } from "../orval/threadyAPI.schemas";
import { AuthContext, type AuthContextValue } from "./auth-context";

const TOKEN_KEY = "threadly.access-token";

function readToken() {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(TOKEN_KEY);
}

function removeToken() {
  if (typeof window !== "undefined") {
    window.localStorage.removeItem(TOKEN_KEY);
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(readToken);
  const [user, setUser] = useState<AuthContextValue["user"]>(null);
  const [isLoading, setIsLoading] = useState(Boolean(readToken()));

  const signOut = useCallback(() => {
    removeToken();
    setToken(null);
    setUser(null);
    setIsLoading(false);
  }, []);

  useEffect(() => {
    let isCurrent = true;

    if (!token) {
      return () => {
        isCurrent = false;
      };
    }

    getApiMe()
      .then((me) => {
        if (!isCurrent) return;
        setUser(me);
      })
      .catch(() => {
        if (!isCurrent) return;
        signOut();
      })
      .finally(() => {
        if (isCurrent) setIsLoading(false);
      });

    return () => {
      isCurrent = false;
    };
  }, [signOut, token]);

  const setSession = useCallback(
    (response: InternalInterfaceControllersAuthResponse) => {
      if (!response.token) return;
      window.localStorage.setItem(TOKEN_KEY, response.token);
      setToken(response.token);
      setUser(response.user ?? null);
      setIsLoading(false);
    },
    [],
  );

  const value = useMemo(
    () => ({
      isLoading,
      isAuthenticated: Boolean(token && user),
      token,
      user,
      setSession,
      signOut,
    }),
    [isLoading, setSession, signOut, token, user],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
