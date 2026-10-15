import { useCallback, useState } from "react";
import { Panel } from "../components/common/Panel";
import { adminApi } from "../api/adminApi";
import { useStateQuery } from "../hooks/useStateQuery";
import { useTransferJobs } from "../hooks/useTransferJobs";

const KINDS = {
  export: "ZIP 文件导出",
  import_validation: "ZIP 上传预检查",
  sqlite_backup: "SQLite 整库备份",
  unknown: "历史任务",
};
const STATUS = {
  pending: "等待执行",
  running: "执行中",
  completed: "已完成",
  failed: "失败",
  cancelled: "已终止",
  unavailable: "已失效 / 不存在",
};
const STAGES = {
  queued: "等待执行",
  staging: "暂存上传包",
  validating: "校验内容与清单",
  archiving: "扫描并打包文件",
  backing_up: "备份并校验数据库",
  validated: "预检查通过，尚未恢复",
  completed: "产物已就绪",
  failed: "执行失败",
  cancelled: "服务停止，任务已终止",
  unavailable: "服务器不再保留此任务记录",
};
export function DataOperationsPage() {
  const transfers = useTransferJobs();
  const [file, setFile] = useState(null);
  const [fileError, setFileError] = useState("");
  const [selected, setSelected] = useState("");
  const [lookup, setLookup] = useState("");
  const overview = transfers.overview;
  const selectedJob = transfers.jobs.find((job) => job.id === selected);
  const busy = Boolean(
    transfers.submitting || overview?.activeJobId || transfers.error,
  );
  const maxBytes = overview?.maxArchiveBytes || 128 * 1024 * 1024;
  const start = async (kind) => {
    const job = await transfers.run(kind, file);
    if (job) setSelected(job.id);
  };
  const selectFile = (event) => {
    const next = event.target.files?.[0] || null;
    setFile(next);
    setFileError(
      next && (next.size === 0 || next.size > maxBytes)
        ? `请选择非空 ZIP，大小不超过 ${formatBytes(maxBytes)}。`
        : "",
    );
  };
  return (
    <div className="data-page">
      {transfers.error ? (
        <div className="banner error" role="alert">
          {transfers.error}
          <button className="btn-ghost" onClick={transfers.refresh}>
            重新查询任务
          </button>
        </div>
      ) : null}
      {transfers.actionError ? (
        <div className="banner error" role="alert">
          {transfers.actionError}
        </div>
      ) : null}
      {transfers.storageWarning ? (
        <p className="state-notice" role="status">
          {transfers.storageWarning}
        </p>
      ) : null}
      {transfers.loading ? (
        <p className="state-notice" role="status">
          正在读取数据运维能力与任务…
        </p>
      ) : null}
      {overview ? (
        <>
          <Panel
            eyebrow="DATA OPERATIONS"
            title="导出与备份"
            subtitle={`实际后端：${overview.backend} · 恢复方式：停机 CLI · 同时仅执行一个传输任务`}
          >
            <div className="data-actions">
              <div>
                <h4>ZIP 文件数据</h4>
                <p>
                  {overview.backend === "sqlite"
                    ? "仅导出允许传输的普通文件，不包含活动 SQLite 快照、事件和状态。目录中可能仍有旧 JSON，不能用 ZIP 代替数据库备份。"
                    : "导出允许传输的运行数据文件。运行中可能跨越多次刷盘，不承诺跨文件同一时间点快照。"}
                </p>
                <p>排除运行库、sidecar、密钥、锁、备份目录和恢复材料。</p>
                <button
                  className="btn-primary"
                  disabled={busy}
                  onClick={() => start("export")}
                >
                  {transfers.submitting === "export"
                    ? "正在创建…"
                    : "创建 ZIP 导出"}
                </button>
              </div>
              <div>
                <h4>SQLite 整库备份</h4>
                <p>
                  包含已提交的快照、事件和派生查询表；使用一致性备份，不包含尚未刷盘的内存修改。
                </p>
                {overview.backend === "sqlite" ? (
                  <button
                    className="btn-primary"
                    disabled={busy}
                    onClick={() => start("sqlite_backup")}
                  >
                    {transfers.submitting === "sqlite_backup"
                      ? "正在创建…"
                      : "创建 SQLite 备份"}
                  </button>
                ) : (
                  <p className="state-footnote">
                    当前为 file 后端，不提供 SQLite 整库备份。
                  </p>
                )}
              </div>
            </div>
          </Panel>
          <Panel
            eyebrow="VALIDATION ONLY"
            title="上传 ZIP 并预检查"
            subtitle="只验证并暂存，不覆盖文件、不恢复内存状态、不重发消息。"
          >
            <form
              className="data-upload"
              onSubmit={(event) => {
                event.preventDefault();
                if (file && !fileError) void start("import_validation");
              }}
            >
              <label>
                导出 ZIP 包
                <input
                  type="file"
                  accept=".zip,application/zip"
                  onChange={selectFile}
                  disabled={Boolean(transfers.submitting)}
                />
              </label>
              <button
                className="btn-primary"
                disabled={busy || !file || Boolean(fileError)}
              >
                {transfers.submitting === "import_validation"
                  ? "正在上传并暂存…"
                  : "上传并预检查"}
              </button>
            </form>
            {fileError ? (
              <p className="banner error" role="alert">
                {fileError}
              </p>
            ) : null}
            <p className="state-footnote">
              压缩包及解压内容上限 {formatBytes(maxBytes)}，最多{" "}
              {overview.maxFiles} 个文件，manifest 上限{" "}
              {formatBytes(overview.maxManifestBytes)}
              。检查规范路径、重复项、大小、SHA-256、符号链接与受保护路径。SQLite
              数据库文件请通过离线 validate-sqlite 校验。
            </p>
          </Panel>
          <Panel
            eyebrow="TRANSFER JOBS"
            title="任务记录"
            subtitle="浏览器只保留最近 50 个已知任务 ID，不保存产物、正文或密钥。服务端任务在内存中，重启后失效。"
            actions={
              <button className="btn-ghost" onClick={transfers.refresh}>
                刷新任务
              </button>
            }
          >
            <form
              className="data-lookup"
              onSubmit={(event) => {
                event.preventDefault();
                if (transfers.track(lookup)) setSelected(lookup.trim());
              }}
            >
              <label>
                任务 ID
                <input
                  value={lookup}
                  onChange={(event) => setLookup(event.target.value)}
                  placeholder="查询已知任务 ID"
                />
              </label>
              <button className="btn-ghost">查询任务</button>
            </form>
            {overview.activeJobId ? (
              <p role="status" className="state-notice">
                正在执行：<span className="mono">{overview.activeJobId}</span>
              </p>
            ) : null}
            <div className="state-table-scroll">
              <table className="state-table data-table">
                <caption className="sr-only">数据传输任务</caption>
                <thead>
                  <tr>
                    <th scope="col">任务 / 类型</th>
                    <th scope="col">状态与阶段</th>
                    <th scope="col">创建时间</th>
                    <th scope="col">操作</th>
                  </tr>
                </thead>
                <tbody>
                  {transfers.jobs.map((job) => (
                    <tr
                      key={job.id}
                      className={selected === job.id ? "selected" : ""}
                    >
                      <td>
                        <button
                          className="state-link mono"
                          aria-label={`查看任务 ${job.id}`}
                          onClick={() => setSelected(job.id)}
                        >
                          {job.id}
                        </button>
                        <span className="events-id">
                          {KINDS[job.kind] || job.kind}
                        </span>
                      </td>
                      <td>
                        {STATUS[job.status] || job.status}
                        <span className="events-id">
                          {STAGES[job.stage] || job.stage}
                        </span>
                      </td>
                      <td>
                        {job.createdAt
                          ? new Date(job.createdAt).toLocaleString()
                          : "—"}
                      </td>
                      <td>
                        {job.downloadAvailable ? (
                          <button
                            className="btn-ghost"
                            disabled={Boolean(transfers.submitting)}
                            onClick={() => transfers.run("download", job.id)}
                          >
                            下载产物
                          </button>
                        ) : (
                          "—"
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            {transfers.jobs.length === 0 ? (
              <p className="state-notice">当前没有任务。</p>
            ) : null}
            <p className="state-footnote">
              任务最多保留 {overview.maxJobs}{" "}
              条，旧记录可能被回收。产物与预检查成功的上传包保留在服务器，按部署方策略停机手动清理；失败包和未完成
              ZIP
              临时文件会删除。关闭页面只停止查询，已接受的任务继续执行。服务停止会请求取消工作，重启后不自动续跑；崩溃残留需停机核对清理。
            </p>
          </Panel>
          {selectedJob ? (
            <TransferDetail
              key={selectedJob.id}
              job={selectedJob}
              backend={overview.backend}
            />
          ) : null}
          <OfflineRestoreGuide backend={overview.backend} job={selectedJob} />
        </>
      ) : null}
    </div>
  );
}

function TransferDetail({ job, backend }) {
  const load = useCallback(
    (signal) => adminApi.getTransferJob(job.id, { signal }),
    [job.id, job.updatedAt],
  );
  const detail = useStateQuery(
    `${job.id}:${job.updatedAt}`,
    load,
    job.status !== "unavailable",
  );
  const record = detail.data || job;
  return (
    <Panel
      eyebrow="RESULT"
      title={record.kind === "import_validation" ? "预检查结果" : "任务详情"}
      subtitle={job.id}
    >
      {job.status === "unavailable" ? (
        <p className="banner error" role="alert">
          {job.error}
        </p>
      ) : (
        <>
          {detail.loading ? <p role="status">正在读取任务详情…</p> : null}
          {detail.error ? (
            <p className="banner error" role="alert">
              {detail.error.status === 404
                ? "任务已失效或不存在，请刷新任务列表。"
                : detail.error.message}
            </p>
          ) : null}
          <p>{STAGES[record.stage] || record.stage}</p>
          {["export", "sqlite_backup"].includes(record.kind) &&
          record.status === "completed" &&
          !job.downloadAvailable ? (
            <p className="state-notice">
              任务已完成，但产物目前不可下载，可能已被清理；历史结果仍保留。
            </p>
          ) : null}
          {record.error ? (
            <p className="banner error" role="alert">
              {record.error}
            </p>
          ) : null}
          {record.kind === "import_validation" &&
          record.status === "completed" ? (
            <p className="banner ok" role="status">
              预检查通过。数据尚未恢复（applied=false）。
              {backend === "sqlite"
                ? "当前 SQLite 数据应使用整库备份的离线恢复流程。"
                : "请按下方停机恢复步骤操作。"}
            </p>
          ) : null}
          {record.report?.backup ? (
            <dl className="state-metadata">
              <div>
                <dt>Schema</dt>
                <dd>{record.report.backup.schemaVersion}</dd>
              </div>
              <div>
                <dt>快照 / 事件</dt>
                <dd>
                  {record.report.backup.snapshots} /{" "}
                  {record.report.backup.events}
                </dd>
              </div>
              <div>
                <dt>备份大小</dt>
                <dd>{formatBytes(record.report.backup.size)}</dd>
              </div>
              <div>
                <dt>SHA-256</dt>
                <dd className="mono">{record.report.backup.sha256}</dd>
              </div>
            </dl>
          ) : null}
          {record.report && !record.report.backup ? (
            <p>
              清单：{record.report.fileCount} 个文件，总计{" "}
              {formatBytes(record.report.totalBytes)}。
            </p>
          ) : null}
          <details className="state-raw">
            <summary>详细结果与文件清单（可能包含敏感文件名）</summary>
            <pre className="mono">{JSON.stringify(record, null, 2)}</pre>
          </details>
        </>
      )}
    </Panel>
  );
}

function OfflineRestoreGuide({ backend, job }) {
  const readyZip =
    backend === "file" &&
    job?.kind === "import_validation" &&
    job.status === "completed";
  const archive = readyZip
    ? `<DATA_DIR>/imports/.upload-${job.id}.zip`
    : "<ZIP_PATH>";
  return (
    <Panel
      eyebrow="OFFLINE RESTORE"
      title="恢复需要停机"
      subtitle="在线恢复不可用。预检查成功只表示包有效，不代表数据已恢复。"
    >
      <p>
        {backend === "sqlite"
          ? "SQLite 恢复会替换目标数据库；原库保留 pre-restore 副本。不要用普通 ZIP 覆盖活动数据库。"
          : "文件恢复按 manifest 覆盖列出的文件，未列出的文件保留；原文件和恢复 journal 会保留，以便失败回滚。"}
      </p>
      <ol className="data-steps">
        <li>保留当前数据与所选备份，确认覆盖范围。</li>
        <li>
          停止机器人，等待最终刷盘和进程退出，关闭外部数据库编辑器。不要删除锁、WAL
          或 journal 绕过检查。
        </li>
        <li>
          在仓库根目录执行下面的 CLI。将占位路径替换为实际配置；datactl
          不自动加载 .env。
        </li>
        <li>
          成功后重新启动，登录并检查会话、聊天、记忆、任务与事件；恢复后正常调度会继续运行。
        </li>
      </ol>
      <pre className="data-command mono">
        {backend === "sqlite"
          ? 'go run ./cmd/datactl validate-sqlite --source "<BACKUP_PATH>"\ngo run ./cmd/datactl restore-sqlite --data-dir "<DATA_DIR>" --sqlite-path "<SQLITE_PATH>" --source "<BACKUP_PATH>"'
          : `go run ./cmd/datactl restore-files --data-dir "<DATA_DIR>" --sqlite-path "<SQLITE_PATH>" --backup-dir "<BACKUP_DIR>" --archive "${archive}"`}
      </pre>
      <p className="state-footnote">
        运行中执行恢复会被进程锁拒绝。页面仅提供步骤，不执行上述命令；在线恢复请求返回
        offline_restore_required。产物应放在私有目录，不提交 Git 或公开发布。
      </p>
    </Panel>
  );
}

function formatBytes(bytes = 0) {
  return bytes >= 1024 * 1024
    ? `${(bytes / (1024 * 1024)).toFixed(1)} MiB`
    : `${bytes} B`;
}
