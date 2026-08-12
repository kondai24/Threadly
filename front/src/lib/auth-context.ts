import { createContext, useContext } from "react";
import type {
  InternalInterfaceControllersAuthResponse,
  InternalInterfaceControllersUserResponse,
} from "../orval/threadyAPI.schemas";

export type AuthContextValue = {
  isLoading: boolean;
  isAuthenticated: boolean;
  user: InternalInterfaceControllersUserResponse | null;
  setSession: (response: InternalInterfaceControllersAuthResponse) => void;
  signOut: () => void;
};

export const AuthContext = createContext<AuthContextValue | undefined>(
  undefined,
);

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) {
    throw new Error("useAuth must be used inside AuthProvider");
  }
  return value;
}
