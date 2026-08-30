import { createContext, useContext } from "react";
import type {
  ThreadlyInternalInterfaceDtoAuthResponse,
  ThreadlyInternalInterfaceDtoUserResponse,
} from "../orval/threadyAPI.schemas";

export type AuthContextValue = {
  isLoading: boolean;
  isAuthenticated: boolean;
  user: ThreadlyInternalInterfaceDtoUserResponse | null;
  setSession: (response: ThreadlyInternalInterfaceDtoAuthResponse) => void;
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
