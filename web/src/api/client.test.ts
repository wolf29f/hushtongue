import { afterEach, describe, expect, test, vi } from "vitest";
import { ApiError, request, toApiError, UNAVAILABLE } from "./client.ts";

describe("toApiError", () => {
  test("reads the error of the API", async () => {
    const response = Response.json(
      { error: "name_taken", message: "Ce nom est déjà pris." },
      { status: 409 },
    );

    expect(await toApiError(response)).toEqual(
      new ApiError(409, "name_taken", "Ce nom est déjà pris."),
    );
  });

  test("makes an error for a page that isn't the API's", async () => {
    const response = new Response("<html>Bad Gateway</html>", { status: 502 });

    const error = await toApiError(response);

    expect(error.status).toBe(502);
    expect(error.code).toBe(UNAVAILABLE);
  });

  test("makes an error for JSON that isn't the API's", async () => {
    const error = await toApiError(Response.json({ detail: "nope" }, { status: 500 }));

    expect(error.code).toBe(UNAVAILABLE);
  });
});

describe("request", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  test("sends the body as JSON and returns the response", async () => {
    const fetch = vi.fn<typeof globalThis.fetch>().mockResolvedValue(Response.json({ id: 1 }));
    vi.stubGlobal("fetch", fetch);

    expect(await request("POST", "/players", { name: "Ana" })).toEqual({ id: 1 });
    expect(fetch).toHaveBeenCalledWith("/api/players", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: '{"name":"Ana"}',
    });
  });

  test("returns nothing for a 204", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null, { status: 204 })));

    expect(await request("POST", "/logout")).toBeUndefined();
  });

  test("throws the error of the API", async () => {
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValue(
          Response.json({ error: "forbidden", message: "Réservé au MJ." }, { status: 403 }),
        ),
    );

    await expect(request("GET", "/players")).rejects.toEqual(
      new ApiError(403, "forbidden", "Réservé au MJ."),
    );
  });

  test("throws an error when the server can't be reached", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));

    await expect(request("GET", "/settings")).rejects.toMatchObject({
      status: 0,
      code: UNAVAILABLE,
    });
  });
});
