import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { StatePage } from "./StatePage";
import { adminApi } from "../api/adminApi";

vi.mock("../api/adminApi", () => ({
  adminApi: {
    getPersistence: vi.fn(),
    getState: vi.fn(),
    getStateDetail: vi.fn(),
    getSessionSummary: vi.fn(),
    getConversationMessages: vi.fn(),
  },
}));
const record = (id, extra = {}) => ({
  id,
  userId: 42,
  sessionId: id,
  updatedAt: "2026-10-15T00:00:00Z",
  payload: {},
  ...extra,
});
const page = (records = [], nextCursor = "") => ({ records, nextCursor });
const deferred = () => {
  let resolve;
  const promise = new Promise((done) => {
    resolve = done;
  });
  return { promise, resolve };
};

beforeEach(() => {
  vi.resetAllMocks();
  adminApi.getPersistence.mockResolvedValue({
    ready: true,
    backend: "sqlite",
    schemaVersion: 4,
    capabilities: ["state_query", "conversation_query"],
  });
  adminApi.getState.mockResolvedValue(page());
  adminApi.getSessionSummary.mockImplementation(async (id) => record(id));
  adminApi.getConversationMessages.mockResolvedValue({ messages: [] });
});

describe("state queries", () => {
  it("checks capability, shows file-backend guidance and sends no state requests", async () => {
    const pending = deferred();
    adminApi.getPersistence.mockReturnValue(pending.promise);
    render(<StatePage />);
    expect(screen.getByRole("status")).toHaveTextContent("正在检查");
    await act(async () =>
      pending.resolve({ ready: true, backend: "file", capabilities: [] }),
    );
    expect(
      await screen.findByText("当前后端不支持状态查询"),
    ).toBeInTheDocument();
    expect(adminApi.getState).not.toHaveBeenCalled();
    expect(adminApi.getConversationMessages).not.toHaveBeenCalled();
  });

  it("supports empty results, errors and explicit retry", async () => {
    adminApi.getState.mockRejectedValueOnce(new Error("offline"));
    render(<StatePage />);
    expect(await screen.findByRole("alert")).toHaveTextContent("offline");
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    expect(
      await screen.findByText("没有符合条件的会话记录。"),
    ).toBeInTheDocument();
  });

  it("filters, paginates and resets pagination when query changes", async () => {
    adminApi.getState.mockImplementation(async (_kind, filters) =>
      filters.cursor
        ? page([record("second")])
        : page([record("first")], "next-token"),
    );
    render(<StatePage />);
    await screen.findByRole("button", { name: "查看 first" });
    fireEvent.change(screen.getByLabelText("用户 ID"), {
      target: { value: "9007199254740993" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询", exact: true }));
    await waitFor(() =>
      expect(adminApi.getState).toHaveBeenLastCalledWith(
        "sessions",
        expect.objectContaining({ user_id: "9007199254740993", cursor: "" }),
        expect.anything(),
      ),
    );
    fireEvent.click(await screen.findByRole("button", { name: "状态下一页" }));
    expect(
      await screen.findByRole("button", { name: "查看 second" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "查看 first" }),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "状态上一页" }));
    await screen.findByRole("button", { name: "查看 first" });
    fireEvent.click(screen.getByRole("button", { name: /投递 Delivery/ }));
    await waitFor(() =>
      expect(adminApi.getState).toHaveBeenLastCalledWith(
        "deliveries",
        expect.objectContaining({ user_id: "", cursor: "" }),
        expect.anything(),
      ),
    );
  });

  it("aborts superseded requests and ignores late responses even if transport ignores abort", async () => {
    const pending = deferred();
    let previousSignal;
    adminApi.getState.mockImplementation((kind, _filters, { signal }) => {
      if (kind === "sessions") {
        previousSignal = signal;
        return pending.promise;
      }
      return Promise.resolve(page([record("intent-now", { sessionId: "s" })]));
    });
    const view = render(<StatePage />);
    await waitFor(() => expect(adminApi.getState).toHaveBeenCalled());
    expect(screen.getByText("正在查询会话…")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /意图 Intent/ }));
    await screen.findByRole("button", { name: "查看 intent-now" });
    expect(previousSignal.aborted).toBe(true);
    await act(async () => pending.resolve(page([record("stale-session")])));
    expect(
      screen.queryByRole("button", { name: "查看 stale-session" }),
    ).not.toBeInTheDocument();
    const lastSignal = adminApi.getState.mock.calls.at(-1)[2].signal;
    view.unmount();
    expect(lastSignal.aborted).toBe(true);
  });

  it("discards stale pagination and restarts from the first page on 409", async () => {
    adminApi.getState.mockImplementation(async (_kind, filters) => {
      if (filters.cursor)
        throw Object.assign(new Error("changed"), { status: 409 });
      return page([record("first")], "stale-token");
    });
    render(<StatePage />);
    fireEvent.click(await screen.findByRole("button", { name: "状态下一页" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("数据已变化");
    expect(
      screen.queryByRole("button", { name: "查看 first" }),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "重新查询" }));
    await screen.findByRole("button", { name: "查看 first" });
    expect(adminApi.getState.mock.calls.at(-1)[1].cursor).toBe("");
  });

  it("loads session metadata separately, renders safe chat and follows associated state", async () => {
    adminApi.getState.mockResolvedValue(page([record("s")]));
    adminApi.getConversationMessages.mockResolvedValue({
      messages: [
        {
          sequence: 0,
          role: "user",
          recordedAt: null,
          payload: {
            role: "user",
            content: [
              { type: "text", text: "<script>bad()</script>" },
              {
                type: "image_url",
                image_url: { url: "https://example.invalid/private.png" },
              },
            ],
          },
        },
        {
          sequence: 1,
          role: "assistant",
          recordedAt: null,
          payload: { content: "delivered reply" },
        },
      ],
      nextCursor: "chat-next",
    });
    const view = render(<StatePage />);
    fireEvent.click(await screen.findByRole("button", { name: "查看 s" }));
    await screen.findByText("delivered reply");
    expect(adminApi.getStateDetail).not.toHaveBeenCalled();
    expect(adminApi.getSessionSummary).toHaveBeenCalledWith(
      "s",
      expect.anything(),
    );
    expect(screen.getByText("图片附件 · 未自动加载")).toBeInTheDocument();
    expect(view.container.querySelector("img, script, iframe")).toBeNull();
    expect(screen.getByText("<script>bad()</script>")).toBeInTheDocument();
    expect(screen.getAllByText("时间未知").length).toBeGreaterThan(0);
    expect(
      [...view.container.querySelectorAll("details")].every(
        (element) => !element.open,
      ),
    ).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: "聊天下一页" }));
    await waitFor(() =>
      expect(adminApi.getConversationMessages).toHaveBeenLastCalledWith(
        "s",
        { limit: 50, cursor: "chat-next" },
        expect.anything(),
      ),
    );
    fireEvent.click(screen.getByRole("button", { name: "本会话的投递" }));
    await waitFor(() =>
      expect(adminApi.getState).toHaveBeenLastCalledWith(
        "deliveries",
        expect.objectContaining({ session_id: "s", cursor: "" }),
        expect.anything(),
      ),
    );
  });

  it("follows intent to open loop and reports missing related records", async () => {
    adminApi.getState.mockResolvedValue(
      page([record("i", { sessionId: "s" })]),
    );
    adminApi.getStateDetail.mockImplementation(async (kind) => {
      if (kind === "intents")
        return record("i", {
          payload: { open_loop_id: "loop" },
          sessionId: "s",
        });
      throw Object.assign(new Error("missing"), { status: 404 });
    });
    render(<StatePage />);
    fireEvent.click(await screen.findByRole("button", { name: /意图 Intent/ }));
    fireEvent.click(await screen.findByRole("button", { name: "查看 i" }));
    fireEvent.click(await screen.findByRole("button", { name: "待办：loop" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("记录不存在");
  });

  it("shows expired authentication without offering endless retry", async () => {
    adminApi.getPersistence.mockRejectedValue(
      Object.assign(new Error("unauthorized"), { status: 401 }),
    );
    render(<StatePage />);
    expect(await screen.findByRole("alert")).toHaveTextContent("登录已失效");
    expect(
      screen.queryByRole("button", { name: "重试" }),
    ).not.toBeInTheDocument();
  });

  it("cancels an in-flight chat request when switching sessions", async () => {
    adminApi.getState.mockResolvedValue(page([record("one"), record("two")]));
    const pending = deferred();
    let signal;
    adminApi.getConversationMessages.mockImplementation(
      (id, _filters, options) => {
        if (id === "one") {
          signal = options.signal;
          return pending.promise;
        }
        return Promise.resolve({ messages: [] });
      },
    );
    render(<StatePage />);
    fireEvent.click(await screen.findByRole("button", { name: "查看 one" }));
    await waitFor(() => expect(signal).toBeDefined());
    fireEvent.click(screen.getByRole("button", { name: "查看 two" }));
    await screen.findByText("此会话没有已保存的聊天消息。");
    expect(signal.aborted).toBe(true);
    await act(async () =>
      pending.resolve({
        messages: [
          {
            sequence: 0,
            role: "assistant",
            payload: { content: "stale private chat" },
          },
        ],
      }),
    );
    expect(
      within(screen.getByRole("region", { name: "聊天历史" })).queryByText(
        "stale private chat",
      ),
    ).not.toBeInTheDocument();
  });
});
