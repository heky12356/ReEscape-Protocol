import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { adminApi, openEventSocket } from "../api/adminApi";
import { MAX_VISIBLE_EVENTS, useEventStream } from "./useEventStream";

vi.mock("../api/adminApi", () => ({
  adminApi: { getEventPage: vi.fn() },
  openEventSocket: vi.fn(),
}));
let sockets;
class FakeSocket {
  close = vi.fn();
  open() {
    this.onopen?.();
  }
  emit(frame) {
    this.onmessage?.({ data: JSON.stringify(frame) });
  }
  disconnect(code = 1006) {
    this.onclose?.({ code });
  }
}
const event = (cursor) => ({
  id: `event-${cursor}`,
  type: "probe",
  cursor: Number(cursor),
});
const baseline = (cursor = "10", streamId = "process-one") => ({
  events: [event(cursor)],
  eventCursors: [cursor],
  cursor,
  streamId,
  hasMore: false,
});
const frame = (cursor, extra = {}) => ({
  type: "event",
  cursor,
  event: event(cursor),
  streamId: "process-one",
  ...extra,
});
const flush = async () => {
  await act(async () => {
    await Promise.resolve();
  });
};
const advance = async (milliseconds) => {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(milliseconds);
  });
};

beforeEach(() => {
  vi.resetAllMocks();
  vi.useFakeTimers();
  sockets = [];
  adminApi.getEventPage.mockImplementation(async (filters) =>
    filters.after
      ? {
          events: [],
          eventCursors: [],
          cursor: filters.after,
          streamId: "process-one",
          hasMore: false,
        }
      : baseline(),
  );
  openEventSocket.mockImplementation(() => {
    const socket = new FakeSocket();
    sockets.push(socket);
    return socket;
  });
});
afterEach(() => vi.useRealTimers());

it("hands off from the HTTP baseline, deduplicates events and preserves exact cursors", async () => {
  const { result } = renderHook(() => useEventStream({ session_id: "s" }));
  await flush();
  expect(openEventSocket).toHaveBeenCalledWith({
    session_id: "s",
    after: "10",
    stream_id: "process-one",
    limit: 200,
  });
  act(() => {
    sockets[0].open();
    sockets[0].emit(frame("10"));
    sockets[0].emit(frame("11"));
    sockets[0].emit(frame("11"));
    sockets[0].emit(frame("9007199254740993"));
    sockets[0].emit({
      type: "checkpoint",
      cursor: "9007199254740993",
      streamId: "process-one",
      hasMore: false,
    });
  });
  expect(result.current.events.map((entry) => entry.cursor)).toEqual([
    "10",
    "11",
    "9007199254740993",
  ]);
  expect(result.current.cursor).toBe("9007199254740993");
  expect(result.current.status).toBe("live");
});

it("resumes multiple disconnects and filtered checkpoints without gaps", async () => {
  const { result } = renderHook(() => useEventStream({ request_id: "r" }));
  await flush();
  act(() =>
    sockets[0].emit({
      type: "checkpoint",
      cursor: "100",
      streamId: "process-one",
      hasMore: false,
    }),
  );
  act(() => sockets[0].disconnect());
  expect(result.current.status).toBe("reconnecting");
  await advance(1000);
  expect(adminApi.getEventPage.mock.calls.at(-1)[0]).toMatchObject({
    after: "100",
    request_id: "r",
  });
  act(() => {
    sockets[1].emit(frame("101"));
    sockets[1].emit({
      type: "checkpoint",
      cursor: "101",
      streamId: "process-one",
      hasMore: false,
    });
    sockets[1].disconnect();
  });
  await advance(1000);
  expect(openEventSocket.mock.calls.at(-1)[0].after).toBe("101");
  expect(result.current.events.map((entry) => entry.cursor)).toEqual([
    "10",
    "101",
  ]);
});

it("pauses by closing the socket, then resumes from the last applied cursor", async () => {
  const { result } = renderHook(() => useEventStream({}));
  await flush();
  act(() => {
    sockets[0].emit(frame("12"));
    result.current.togglePaused();
  });
  expect(result.current.status).toBe("paused");
  expect(sockets[0].close).toHaveBeenCalledOnce();
  expect(vi.getTimerCount()).toBe(0);
  await advance(60000);
  expect(sockets).toHaveLength(1);
  act(() => result.current.togglePaused());
  await flush();
  expect(openEventSocket.mock.calls.at(-1)[0].after).toBe("12");
  expect(result.current.events).toHaveLength(2);
});

it("rebuilds on gap/reset frames and ignores callbacks from the replaced socket", async () => {
  const { result } = renderHook(() => useEventStream({}));
  await flush();
  const oldMessage = sockets[0].onmessage;
  adminApi.getEventPage.mockResolvedValueOnce(baseline("2000", "process-two"));
  act(() => sockets[0].emit({ type: "gap", reason: "retention_gap" }));
  await flush();
  expect(result.current.warning).toContain("无法补回");
  expect(result.current.events.map((entry) => entry.cursor)).toEqual(["2000"]);
  expect(openEventSocket.mock.calls.at(-1)[0]).toMatchObject({
    after: "2000",
    stream_id: "process-two",
  });
  act(() => oldMessage({ data: JSON.stringify(frame("9999")) }));
  expect(result.current.cursor).toBe("2000");
});

it("rebuilds when an HTTP replay detects a restarted process", async () => {
  const { result } = renderHook(() => useEventStream({}));
  await flush();
  adminApi.getEventPage
    .mockRejectedValueOnce(
      Object.assign(new Error("stream_changed"), {
        status: 409,
        code: "stream_changed",
      }),
    )
    .mockResolvedValueOnce(baseline("2", "restarted"));
  act(() => sockets[0].disconnect());
  await advance(1000);
  expect(result.current.warning).toContain("重置");
  expect(result.current.events.map((entry) => entry.cursor)).toEqual(["2"]);
});

it("keeps a bounded visible buffer through a high-volume batch", async () => {
  const { result } = renderHook(() => useEventStream({}));
  await flush();
  act(() => {
    for (let cursor = 11; cursor < 800; cursor++)
      sockets[0].emit(frame(String(cursor)));
  });
  expect(result.current.events).toHaveLength(MAX_VISIBLE_EVENTS);
  expect(result.current.events[0].cursor).toBe("300");
  expect(result.current.evicted).toBe(290);
});

it("stops retries and notifies login on websocket authentication expiry", async () => {
  const expired = vi.fn();
  window.addEventListener("admin:unauthorized", expired);
  try {
    const { result } = renderHook(() => useEventStream({}));
    await flush();
    act(() => sockets[0].disconnect(4001));
    expect(result.current.status).toBe("unauthorized");
    expect(expired).toHaveBeenCalledOnce();
    await advance(60000);
    expect(adminApi.getEventPage).toHaveBeenCalledTimes(1);
    expect(vi.getTimerCount()).toBe(0);
  } finally {
    window.removeEventListener("admin:unauthorized", expired);
  }
});

it("stops on HTTP 401 and unsupported replay without an endless retry loop", async () => {
  adminApi.getEventPage.mockRejectedValue(
    Object.assign(new Error("unavailable"), { status: 501 }),
  );
  const first = renderHook(() => useEventStream({}));
  await flush();
  expect(first.result.current.status).toBe("error");
  expect(sockets).toHaveLength(0);
  expect(vi.getTimerCount()).toBe(0);
  first.unmount();
  adminApi.getEventPage.mockRejectedValue(
    Object.assign(new Error("expired"), { status: 401 }),
  );
  const second = renderHook(() => useEventStream({}));
  await flush();
  expect(second.result.current.status).toBe("unauthorized");
  expect(vi.getTimerCount()).toBe(0);
});

it("caps retry backoff, detects a stalled connection, and clears all timers on unmount", async () => {
  adminApi.getEventPage.mockRejectedValue(new Error("offline"));
  const first = renderHook(() => useEventStream({}));
  await flush();
  for (const delay of [1000, 2000, 4000, 8000, 16000, 30000])
    await advance(delay);
  expect(first.result.current.retryIn).toBe(30000);
  expect(vi.getTimerCount()).toBe(1);
  first.unmount();
  expect(vi.getTimerCount()).toBe(0);
  adminApi.getEventPage.mockResolvedValue(baseline());
  const second = renderHook(() => useEventStream({}));
  await flush();
  await advance(10000);
  expect(second.result.current.status).toBe("reconnecting");
  const options = adminApi.getEventPage.mock.calls.at(-1)[1];
  second.unmount();
  expect(options.signal.aborted).toBe(true);
  expect(vi.getTimerCount()).toBe(0);
});

it("aborts old baseline requests and isolates filters even when the response arrives late", async () => {
  let resolve;
  const pending = new Promise((done) => {
    resolve = done;
  });
  adminApi.getEventPage.mockReturnValueOnce(pending);
  const { result, rerender } = renderHook(
    ({ filters }) => useEventStream(filters),
    { initialProps: { filters: { request_id: "old" } } },
  );
  const signal = adminApi.getEventPage.mock.calls[0][1].signal;
  rerender({ filters: { request_id: "new" } });
  await flush();
  expect(signal.aborted).toBe(true);
  await act(async () => resolve(baseline("999")));
  expect(result.current.cursor).toBe("10");
  expect(sockets).toHaveLength(1);
  expect(openEventSocket.mock.calls.at(-1)[0].request_id).toBe("new");
});
