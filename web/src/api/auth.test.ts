import { afterEach, expect, test, vi } from "vitest";
import { getMe } from "./auth.ts";
import { UNAVAILABLE } from "./client.ts";

afterEach(() => {
  vi.unstubAllGlobals();
});

test("getMe returns the user", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ name: "mj", role: "gm" })));

  expect(await getMe()).toEqual({ name: "mj", role: "gm" });
});

test("getMe returns null without a session", async () => {
  vi.stubGlobal(
    "fetch",
    vi
      .fn()
      .mockResolvedValue(
        Response.json({ error: "unauthenticated", message: "Connexion requise." }, { status: 401 }),
      ),
  );

  expect(await getMe()).toBeNull();
});

test("getMe throws when the server can't be reached", async () => {
  vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));

  await expect(getMe()).rejects.toMatchObject({ code: UNAVAILABLE });
});
