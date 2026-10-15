import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { EventsPage } from "./EventsPage";
import { useEventStream } from "../hooks/useEventStream";

vi.mock("../hooks/useEventStream", () => ({
  MAX_VISIBLE_EVENTS: 500,
  useEventStream: vi.fn(),
}));
let stream;
beforeEach(() => {
  vi.resetAllMocks();
  stream = {
    status: "live",
    cursor: "42",
    paused: false,
    events: [
      {
        cursor: "42",
        id: "e-42",
        type: "tool_finished",
        request_id: "r",
        session_id: "s",
        tool: "legacy-search",
        message: "<img src=x onerror=bad()>",
        created_at: "2026-10-15T00:00:00Z",
      },
    ],
    togglePaused: vi.fn(),
    reload: vi.fn(),
    retry: vi.fn(),
  };
  useEventStream.mockImplementation(() => stream);
});

it("shows live state, applies exact filters and follows legacy tool correlations", () => {
  render(<EventsPage />);
  expect(screen.getByRole("status")).toHaveTextContent("实时连接");
  fireEvent.change(screen.getByLabelText("请求 ID"), {
    target: { value: " r-2 " },
  });
  fireEvent.click(screen.getByRole("button", { name: "应用筛选" }));
  expect(useEventStream.mock.calls.at(-1)[0].request_id).toBe("r-2");
  fireEvent.click(screen.getByRole("button", { name: "查看事件 42" }));
  fireEvent.click(
    screen.getByRole("button", { name: "追踪工具名称 legacy-search" }),
  );
  expect(useEventStream.mock.calls.at(-1)[0]).toMatchObject({
    tool: "legacy-search",
    request_id: "",
  });
  fireEvent.click(screen.getByRole("button", { name: "暂停接收" }));
  expect(stream.togglePaused).toHaveBeenCalledOnce();
});

it("keeps raw payload collapsed and displays gap/retry and buffer limits", () => {
  stream.status = "reconnecting";
  stream.retryIn = 30000;
  stream.warning = "历史窗口过期，无法补回";
  stream.evicted = 250;
  const view = render(<EventsPage />);
  expect(screen.getByRole("alert")).toHaveTextContent("无法补回");
  expect(screen.getByRole("status")).toHaveTextContent("30 秒后重试");
  expect(screen.getByText(/已移出 250 条/)).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "查看事件 42" }));
  expect(view.container.querySelector("details").open).toBe(false);
  expect(view.container.querySelector("img, iframe, script")).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "重新加载近期历史" }));
  expect(stream.reload).toHaveBeenCalledOnce();
});
