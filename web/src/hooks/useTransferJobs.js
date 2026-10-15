import { useCallback, useEffect, useRef, useState } from "react";
import { adminApi } from "../api/adminApi";

export const TRANSFER_IDS_KEY = "reescape.transfer.job-ids.v1";
const MAX_KNOWN_IDS = 50;
const validID = (id) =>
  typeof id === "string" && /^[A-Za-z0-9_-]{1,160}$/.test(id);

export function useTransferJobs() {
  const ids = useRef(null);
  const [storageWarning, setStorageWarning] = useState("");
  const [state, setState] = useState({
    jobs: [],
    overview: null,
    loading: true,
    error: "",
  });
  const [revision, setRevision] = useState(0);
  const [submitting, setSubmitting] = useState("");
  const [actionError, setActionError] = useState("");
  const operation = useRef(null);
  const mounted = useRef(false);
  const refresh = useCallback(() => setRevision((value) => value + 1), []);
  const remember = useCallback((newIDs) => {
    ids.current = [...new Set([...newIDs, ...(ids.current || [])])]
      .filter(validID)
      .slice(0, MAX_KNOWN_IDS);
    try {
      localStorage.setItem(TRANSFER_IDS_KEY, JSON.stringify(ids.current));
    } catch {
      setStorageWarning(
        "浏览器存储不可用，刷新后可能无法保留旧任务 ID；当前任务仍可查询。",
      );
    }
  }, []);

  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      operation.current?.abort();
      operation.current = null;
    };
  }, []);

  useEffect(() => {
    let active = true;
    let timer;
    let failures = 0;
    const controller = new AbortController();
    if (ids.current === null) {
      try {
        const stored = JSON.parse(
          localStorage.getItem(TRANSFER_IDS_KEY) || "[]",
        );
        ids.current = Array.isArray(stored)
          ? stored.filter(validID).slice(0, MAX_KNOWN_IDS)
          : [];
      } catch {
        ids.current = [];
        setStorageWarning("无法读取旧任务 ID；当前任务仍可查询。");
      }
    }
    async function poll() {
      try {
        const overview = await adminApi.getTransfers({
          signal: controller.signal,
        });
        if (!active) return;
        if (!Array.isArray(overview.jobs) || !overview.instanceId)
          throw Object.assign(new Error("任务列表协议不完整，请升级服务。"), {
            protocol: true,
          });
        remember(overview.jobs.map((job) => job.id));
        const current = new Set(overview.jobs.map((job) => job.id));
        const missing = ids.current
          .filter((id) => !current.has(id))
          .map((id) => ({
            id,
            kind: "unknown",
            status: "unavailable",
            stage: "unavailable",
            error:
              "任务已失效或不存在，可能因服务重启或任务记录回收。保留产物不会自动恢复或执行。",
          }));
        setState({
          overview,
          jobs: [...overview.jobs, ...missing],
          loading: false,
          error: "",
        });
        failures = 0;
        timer = setTimeout(poll, overview.activeJobId ? 1000 : 5000);
      } catch (error) {
        if (!active || error.name === "AbortError") return;
        setState((previous) => ({
          ...previous,
          loading: false,
          error:
            error.status === 401
              ? "登录已失效，请重新登录。"
              : `任务查询失败：${error.message}`,
        }));
        if (!error.protocol && ![401, 403, 501].includes(error.status))
          timer = setTimeout(
            poll,
            Math.min(2000 * 2 ** Math.min(failures++, 4), 30000),
          );
      }
    }
    void poll();
    return () => {
      active = false;
      controller.abort();
      clearTimeout(timer);
    };
  }, [revision, remember]);

  const run = useCallback(
    async (kind, file) => {
      if (operation.current) return;
      const controller = new AbortController();
      operation.current = controller;
      setSubmitting(kind);
      setActionError("");
      try {
        const options = { signal: controller.signal };
        const job =
          kind === "export"
            ? await adminApi.startTransferExport(options)
            : kind === "sqlite_backup"
              ? await adminApi.startSQLiteBackup(options)
              : kind === "download"
                ? await adminApi.downloadTransfer(file, options)
                : await adminApi.startTransferImport(file, options);
        if (!mounted.current || controller.signal.aborted) return;
        remember([job.id]);
        refresh();
        return job;
      } catch (error) {
        if (
          !mounted.current ||
          controller.signal.aborted ||
          error.name === "AbortError"
        )
          return;
        setActionError(
          error.status === 409
            ? "已有传输任务正在执行，请等待完成后再操作。"
            : error.status === 401
              ? "登录已失效，请重新登录。"
              : `${error.message}。若请求结果未确认，请刷新任务列表核对，避免重复提交。`,
        );
        if (error.status !== 401) refresh();
      } finally {
        if (operation.current === controller) {
          operation.current = null;
          if (mounted.current) setSubmitting("");
        }
      }
    },
    [remember, refresh],
  );
  const track = (id) => {
    if (!validID(id.trim())) {
      setActionError(
        "请输入有效的任务 ID（字母、数字、下划线或短横线，最多 160 字符）。",
      );
      return false;
    }
    remember([id.trim()]);
    refresh();
    return true;
  };
  return {
    ...state,
    submitting,
    actionError,
    storageWarning,
    refresh,
    run,
    track,
  };
}
