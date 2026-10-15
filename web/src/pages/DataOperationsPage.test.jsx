import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { DataOperationsPage } from "./DataOperationsPage";
import { useTransferJobs } from "../hooks/useTransferJobs";
import { adminApi } from "../api/adminApi";

vi.mock("../hooks/useTransferJobs", () => ({ useTransferJobs: vi.fn() }));
vi.mock("../api/adminApi", () => ({ adminApi: { getTransferJob: vi.fn() } }));
let transfers;
const job = (extra = {}) => ({
  id: "import_validation-good",
  kind: "import_validation",
  status: "completed",
  stage: "validated",
  applied: false,
  updatedAt: "2026-10-15T00:00:00Z",
  report: {
    fileCount: 2,
    totalBytes: 200,
    files: [{ path: "memory/history.json", size: 200 }],
  },
  ...extra,
});
beforeEach(() => {
  vi.resetAllMocks();
  transfers = {
    jobs: [],
    overview: {
      backend: "file",
      restoreMode: "offline",
      maxFiles: 4096,
      maxArchiveBytes: 134217728,
      maxManifestBytes: 1048576,
      maxJobs: 100,
    },
    refresh: vi.fn(),
    run: vi.fn(),
    track: vi.fn().mockReturnValue(true),
  };
  useTransferJobs.mockImplementation(() => transfers);
  adminApi.getTransferJob.mockResolvedValue(job());
});

it("shows file scope and offline restore instructions without a live restore control", () => {
  render(<DataOperationsPage />);
  expect(screen.getByText(/运行中可能跨越多次刷盘/)).toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "创建 SQLite 备份" }),
  ).not.toBeInTheDocument();
  expect(screen.getByText("恢复需要停机")).toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: /执行恢复|立即恢复/ }),
  ).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "创建 ZIP 导出" }));
  expect(transfers.run).toHaveBeenCalledWith("export", null);
});

it("shows SQLite-specific scope and backup action", () => {
  transfers.overview.backend = "sqlite";
  render(<DataOperationsPage />);
  expect(screen.getByText(/不能用 ZIP 代替数据库备份/)).toBeInTheDocument();
  expect(screen.getByText(/restore-sqlite --data-dir/)).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "创建 SQLite 备份" }));
  expect(transfers.run).toHaveBeenCalledWith("sqlite_backup", null);
});

it("rejects empty/oversized uploads and only starts precheck for a valid selection", () => {
  render(<DataOperationsPage />);
  const input = screen.getByLabelText("导出 ZIP 包");
  fireEvent.change(input, { target: { files: [new File([], "empty.zip")] } });
  expect(screen.getByRole("alert")).toHaveTextContent("非空 ZIP");
  expect(screen.getByRole("button", { name: "上传并预检查" })).toBeDisabled();
  fireEvent.change(input, {
    target: { files: [{ size: 134217729, name: "big.zip" }] },
  });
  expect(screen.getByRole("button", { name: "上传并预检查" })).toBeDisabled();
  const file = new File(["zip"], "good.zip");
  fireEvent.change(input, { target: { files: [file] } });
  fireEvent.click(screen.getByRole("button", { name: "上传并预检查" }));
  expect(transfers.run).toHaveBeenCalledWith("import_validation", file);
});

it("distinguishes validation success from restore and keeps the file report collapsed", async () => {
  transfers.jobs = [job()];
  const view = render(<DataOperationsPage />);
  fireEvent.click(
    screen.getByRole("button", { name: "查看任务 import_validation-good" }),
  );
  await screen.findByText(/数据尚未恢复（applied=false）/);
  expect(screen.getByText(/清单：2 个文件/)).toBeInTheDocument();
  expect(view.container.querySelector("details").open).toBe(false);
  expect(
    screen.getByText(
      /--archive "<DATA_DIR>\/imports\/.upload-import_validation-good.zip"/,
    ),
  ).toBeInTheDocument();
});

it("explains missing old jobs and disables duplicate work while the server is busy", () => {
  transfers.overview.activeJobId = "export-active";
  transfers.jobs = [
    job({
      id: "import_validation-old",
      status: "unavailable",
      stage: "unavailable",
      error: "服务重启后任务已失效",
    }),
  ];
  render(<DataOperationsPage />);
  expect(screen.getByRole("button", { name: "创建 ZIP 导出" })).toBeDisabled();
  fireEvent.click(
    screen.getByRole("button", { name: "查看任务 import_validation-old" }),
  );
  expect(screen.getByRole("alert")).toHaveTextContent("服务重启");
  expect(adminApi.getTransferJob).not.toHaveBeenCalled();
  fireEvent.change(screen.getByLabelText("任务 ID"), {
    target: { value: "export-known" },
  });
  fireEvent.click(screen.getByRole("button", { name: "查询任务" }));
  expect(transfers.track).toHaveBeenCalledWith("export-known");
});

it("downloads a completed artifact and exposes safe query failures", async () => {
  transfers.jobs = [
    job({ id: "export-ready", kind: "export", downloadAvailable: true }),
  ];
  adminApi.getTransferJob.mockRejectedValue(
    Object.assign(new Error("gone"), { status: 404 }),
  );
  render(<DataOperationsPage />);
  fireEvent.click(screen.getByRole("button", { name: "下载产物" }));
  expect(transfers.run).toHaveBeenCalledWith("download", "export-ready");
  fireEvent.click(
    screen.getByRole("button", { name: "查看任务 export-ready" }),
  );
  await waitFor(() =>
    expect(screen.getByRole("alert")).toHaveTextContent("任务已失效"),
  );
});
