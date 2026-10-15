import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { adminApi } from "../api/adminApi";
import { TRANSFER_IDS_KEY, useTransferJobs } from "./useTransferJobs";

vi.mock("../api/adminApi", () => ({
  adminApi: {
    getTransfers: vi.fn(),
    startTransferExport: vi.fn(),
    startSQLiteBackup: vi.fn(),
    startTransferImport: vi.fn(),
    downloadTransfer: vi.fn(),
  },
}));
const job = (extra = {}) => ({
  id: "export-one",
  kind: "export",
  status: "completed",
  stage: "completed",
  downloadAvailable: true,
  ...extra,
});
const overview = (jobs = [], extra = {}) => ({
  jobs,
  instanceId: "process-one",
  backend: "file",
  maxJobs: 100,
  ...extra,
});
const flush = async () => {
  await act(async () => {
    await Promise.resolve();
  });
};
const advance = async (ms) => {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
};
beforeEach(() => {
  vi.resetAllMocks();
  vi.useFakeTimers();
  localStorage.clear();
  adminApi.getTransfers.mockResolvedValue(overview());
});
afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

it("polls active tasks to completion and retains only bounded job IDs", async () => {
  adminApi.getTransfers
    .mockResolvedValueOnce(
      overview([job({ status: "running", stage: "archiving" })], {
        activeJobId: "export-one",
      }),
    )
    .mockResolvedValue(overview([job()]));
  const { result } = renderHook(() => useTransferJobs());
  await flush();
  expect(result.current.jobs[0].status).toBe("running");
  await advance(1000);
  expect(result.current.jobs[0].status).toBe("completed");
  const stored = JSON.parse(localStorage.getItem(TRANSFER_IDS_KEY));
  expect(stored).toEqual(["export-one"]);
  adminApi.getTransfers.mockResolvedValue(
    overview(
      Array.from({ length: 100 }, (_, index) =>
        job({ id: `export-${index}`, report: { files: ["private.json"] } }),
      ),
    ),
  );
  act(() => result.current.refresh());
  await flush();
  const serialized = localStorage.getItem(TRANSFER_IDS_KEY);
  expect(JSON.parse(serialized)).toHaveLength(50);
  expect(serialized).not.toContain("private.json");
});

it("restores known IDs after refresh and marks lost server jobs unavailable", async () => {
  localStorage.setItem(
    TRANSFER_IDS_KEY,
    JSON.stringify(["import_validation-old", "invalid/id"]),
  );
  const { result } = renderHook(() => useTransferJobs());
  await flush();
  expect(result.current.jobs).toHaveLength(1);
  expect(result.current.jobs[0]).toMatchObject({
    id: "import_validation-old",
    status: "unavailable",
  });
  expect(result.current.jobs[0].error).toContain("服务重启");
  act(() => result.current.track("sqlite_backup-another"));
  await flush();
  expect(
    result.current.jobs.find((entry) => entry.id === "sqlite_backup-another")
      .status,
  ).toBe("unavailable");
});

it("creates each task once, reports conflicts and never automatically retries a POST", async () => {
  adminApi.startTransferExport.mockRejectedValue(
    Object.assign(new Error("busy"), { status: 409 }),
  );
  const { result } = renderHook(() => useTransferJobs());
  await flush();
  await act(async () => result.current.run("export"));
  expect(result.current.actionError).toContain("已有传输任务");
  await advance(5000);
  expect(adminApi.startTransferExport).toHaveBeenCalledOnce();
  adminApi.startSQLiteBackup.mockResolvedValue(
    job({ id: "sqlite_backup-new", kind: "sqlite_backup" }),
  );
  await act(async () => result.current.run("sqlite_backup"));
  expect(JSON.parse(localStorage.getItem(TRANSFER_IDS_KEY))).toContain(
    "sqlite_backup-new",
  );
});

it("serializes submissions and cancels a pending upload on unmount without resubmitting", async () => {
  let resolve;
  adminApi.startTransferImport.mockReturnValue(
    new Promise((done) => {
      resolve = done;
    }),
  );
  const hook = renderHook(() => useTransferJobs());
  await flush();
  let operation;
  act(() => {
    operation = hook.result.current.run(
      "import_validation",
      new File(["zip"], "import.zip"),
    );
  });
  await act(async () => hook.result.current.run("export"));
  expect(adminApi.startTransferExport).not.toHaveBeenCalled();
  const signal = adminApi.startTransferImport.mock.calls[0][1].signal;
  hook.unmount();
  expect(signal.aborted).toBe(true);
  await advance(0);
  expect(vi.getTimerCount()).toBe(0);
  await act(async () => {
    resolve(job({ id: "import_validation-late" }));
    await operation;
  });
  expect(localStorage.getItem(TRANSFER_IDS_KEY)).not.toContain(
    "import_validation-late",
  );
});

it("isolates stale list responses after refresh and clears polling on unmount", async () => {
  let resolve;
  adminApi.getTransfers
    .mockReturnValueOnce(
      new Promise((done) => {
        resolve = done;
      }),
    )
    .mockResolvedValue(overview([job({ id: "export-current" })]));
  const hook = renderHook(() => useTransferJobs());
  const signal = adminApi.getTransfers.mock.calls[0][0].signal;
  act(() => hook.result.current.refresh());
  await flush();
  expect(signal.aborted).toBe(true);
  await act(async () => resolve(overview([job({ id: "export-stale" })])));
  expect(hook.result.current.jobs.map((entry) => entry.id)).toEqual([
    "export-current",
  ]);
  hook.unmount();
  await advance(0);
  expect(vi.getTimerCount()).toBe(0);
});

it("handles blocked browser storage and stops polling after 401", async () => {
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
    throw new Error("blocked");
  });
  const hook = renderHook(() => useTransferJobs());
  await flush();
  expect(hook.result.current.overview.backend).toBe("file");
  expect(hook.result.current.storageWarning).toContain("存储不可用");
  adminApi.getTransfers.mockRejectedValue(
    Object.assign(new Error("expired"), { status: 401 }),
  );
  act(() => hook.result.current.refresh());
  await flush();
  expect(hook.result.current.error).toContain("登录已失效");
  await advance(0);
  expect(vi.getTimerCount()).toBe(0);
});

it("uses bounded backoff for list failures and reports unknown POST outcomes", async () => {
  adminApi.getTransfers.mockRejectedValue(new Error("offline"));
  const hook = renderHook(() => useTransferJobs());
  await flush();
  expect(hook.result.current.error).toContain("offline");
  for (const delay of [2000, 4000, 8000, 16000, 30000]) await advance(delay);
  expect(vi.getTimerCount()).toBe(1);
  adminApi.startTransferExport.mockRejectedValue(new Error("connection lost"));
  await act(async () => hook.result.current.run("export"));
  expect(hook.result.current.actionError).toContain("避免重复提交");
  expect(adminApi.startTransferExport).toHaveBeenCalledOnce();
});
