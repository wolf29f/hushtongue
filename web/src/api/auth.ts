import { ApiError, request } from "./client.ts";

export type Role = "gm" | "player";

/** Me is the logged in user. */
export type Me = {
  name: string;
  role: Role;
};

/** isUnauthenticated tells whether err is the API refusing a request for lack of a session. */
export function isUnauthenticated(err: unknown): boolean {
  return err instanceof ApiError && err.code === "unauthenticated";
}

/** getMe returns the logged in user, or null without a session. */
export async function getMe(): Promise<Me | null> {
  try {
    return await request<Me>("GET", "/me");
  } catch (err) {
    if (isUnauthenticated(err)) {
      return null;
    }
    throw err;
  }
}

export function login(name: string, password: string): Promise<Me> {
  return request("POST", "/login", { name, password });
}

export function loginWithLink(token: string): Promise<Me> {
  return request("POST", "/login/link", { token });
}

export function logout(): Promise<void> {
  return request("POST", "/logout");
}
