import { afterEach, expect, it, vi } from "vitest";
import { adminApi } from "./adminApi";

afterEach(() => vi.unstubAllGlobals());

it("preserves exact filters, escapes IDs, passes abort and Cookie credentials", async () => {
  const fetch = vi
    .fn()
    .mockImplementation(
      async () =>
        new Response(JSON.stringify({ messages: [] }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
    );
  vi.stubGlobal("fetch", fetch);
  const controller = new AbortController();
  await adminApi.getState(
    "sessions",
    {
      user_id: "9007199254740993",
      session_id: "private:42",
      status: "",
      limit: 25,
    },
    { signal: controller.signal },
  );
  expect(fetch).toHaveBeenLastCalledWith(
    "/api/admin/state/sessions?user_id=9007199254740993&session_id=private%3A42&limit=25",
    expect.objectContaining({
      credentials: "include",
      signal: controller.signal,
      cache: "no-store",
    }),
  );
  await adminApi.getConversationMessages("group:42/one", { limit: 50 });
  expect(fetch.mock.calls.at(-1)[0]).toBe(
    "/api/admin/sessions/group%3A42%2Fone/messages?limit=50",
  );
});

it("notifies the existing login flow on 401 and preserves conflict codes", async () => {
  const listener = vi.fn();
  window.addEventListener("admin:unauthorized", listener);
  try {
    const fetch = vi
      .fn()
      .mockResolvedValueOnce(
        new Response('{"error":"expired"}', {
          status: 401,
          headers: { "Content-Type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response('{"error":"changed","code":"read_model_changed"}', {
          status: 409,
          headers: { "Content-Type": "application/json" },
        }),
      );
    vi.stubGlobal("fetch", fetch);
    await expect(adminApi.getPersistence()).rejects.toMatchObject({
      status: 401,
    });
    expect(listener).toHaveBeenCalledTimes(1);
    await expect(adminApi.getState("sessions")).rejects.toMatchObject({
      status: 409,
      code: "read_model_changed",
    });
    expect(listener).toHaveBeenCalledTimes(1);
  } finally {
    window.removeEventListener("admin:unauthorized", listener);
  }
});
