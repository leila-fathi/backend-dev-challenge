import { create } from "zustand";
import type { AuthPayload } from "../api";

const AUTH_STORAGE_KEY = "challenge_auth_state";
const LEGACY_TOKEN_STORAGE_KEY = "challenge_token";

export type AuthState = {
  token: string;
  tokenExpiresAt: number | null;
  userId: string;
  groupId: string;
  email: string;
  displayName: string;
  setAuth: (payload: AuthPayload) => void;
  clearAuth: () => void;
};

type StoredAuth = {
  token: string;
  tokenExpiresAt: number | null;
  userId: string;
  groupId: string;
  email: string;
  displayName: string;
};

const emptyAuth: StoredAuth = {
  token: "",
  tokenExpiresAt: null,
  userId: "",
  groupId: "",
  email: "",
  displayName: "",
};

function readStoredAuth(): StoredAuth {
  if (typeof window === "undefined") {
    return emptyAuth;
  }

  const raw = window.localStorage.getItem(AUTH_STORAGE_KEY);
  if (raw) {
    try {
      const parsed = JSON.parse(raw) as Partial<StoredAuth>;
      if (typeof parsed.token === "string" && parsed.token.trim() !== "") {
        return {
          token: parsed.token,
          tokenExpiresAt:
            typeof parsed.tokenExpiresAt === "number" ? parsed.tokenExpiresAt : null,
          userId: parsed.userId ?? "",
          groupId: parsed.groupId ?? "",
          email: parsed.email ?? "",
          displayName: parsed.displayName ?? "",
        };
      }
    } catch {
      window.localStorage.removeItem(AUTH_STORAGE_KEY);
    }
  }

  const legacyToken = window.localStorage.getItem(LEGACY_TOKEN_STORAGE_KEY);
  if (legacyToken && legacyToken.trim() !== "") {
    return {
      ...emptyAuth,
      token: legacyToken,
    };
  }

  return emptyAuth;
}

function persistAuth(value: StoredAuth): void {
  if (typeof window === "undefined") {
    return;
  }

  if (value.token) {
    window.localStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify(value));
    window.localStorage.setItem(LEGACY_TOKEN_STORAGE_KEY, value.token);
    return;
  }

  window.localStorage.removeItem(AUTH_STORAGE_KEY);
  window.localStorage.removeItem(LEGACY_TOKEN_STORAGE_KEY);
}

const initialAuth = readStoredAuth();

export const useAuthStore = create<AuthState>((set) => ({
  ...initialAuth,
  setAuth: (payload) => {
    const nextState: StoredAuth = {
      token: payload.token,
      tokenExpiresAt: payload.tokenExpiresAt,
      userId: payload.userId,
      groupId: payload.groupId,
      email: payload.email,
      displayName: payload.displayName,
    };
    persistAuth(nextState);
    set(nextState);
  },
  clearAuth: () => {
    persistAuth(emptyAuth);
    set(emptyAuth);
  },
}));
