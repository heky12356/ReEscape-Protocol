import { MetricCard } from "../components/common/MetricCard";
import { Panel } from "../components/common/Panel";

export function OverviewPage({ panel }) {
  const latestLogFile = panel.logFiles?.[0]?.name || "-";
  const logCount = panel.logFiles?.length || 0;
  const configChecks = Object.entries(panel.ready.checks || {});
  const healthChecks = Object.entries(panel.health.checks || {});

  return (
    <div className="stack">
      <div className="metric-grid">
        <MetricCard
          label="Model route"
          value={panel.config.aiModel || "-"}
          hint={panel.config.aiBaseUrl || "AI base URL not set"}
          tone="mint"
        />
        <MetricCard
          label="Character file"
          value={panel.config.character || "-"}
          hint={panel.config.aiProfile || "default"}
          tone="copper"
        />
        <MetricCard label="Log archive" value={String(logCount)} hint={latestLogFile} tone="sky" />
        <MetricCard
          label="Retry window"
          value={`${panel.config.aiTimeout || 0}s`}
          hint={`${panel.config.aiRetryCount || 0} retries / ${panel.config.aiRateLimit || 0} rpm`}
        />
      </div>

      <div className="split-layout">
        <Panel
          eyebrow="Control map"
          title="当前运行画像"
          subtitle="这块不是纯配置抄写，而是把值重新组织成操作视角。"
        >
          <div className="kv-grid">
            <KV label="Environment file" value={panel.config.environmentConfig || "-"} mono />
            <KV label="AI Base URL" value={panel.config.aiBaseUrl || "-"} />
            <KV
              label="AI Key"
              value={panel.config.aiKeySet ? panel.config.aiKeyMasked : "未设置"}
            />
            <KV
              label="Sampling"
              value={`Top P ${panel.config.aiTopP} / Temp ${panel.config.aiTemperature}`}
            />
            <KV
              label="Time context"
              value={
                panel.config.enableTimeContext
                  ? `${panel.config.timeContextTimezone || "default"} / ${panel.config.timeContextFormat || "default"}`
                  : "disabled"
              }
            />
            <KV label="Max tokens" value={String(panel.config.aiMaxTokens || 0)} />
            <KV label="Profile file" value={panel.config.aiConfigFile || "-"} mono />
          </div>
        </Panel>

        <Panel
          eyebrow="Probe report"
          title="健康与就绪检查"
          subtitle="用两套探针区分“服务活着”与“服务适合接流量”。"
        >
          <div className="probe-report">
            <ProbeBlock title="Health / process" status={panel.health.status} checks={healthChecks} />
            <ProbeBlock title="Ready / dependencies" status={panel.ready.status} checks={configChecks} />
          </div>
        </Panel>
      </div>

      <Panel
        eyebrow="Recent artifacts"
        title="最近日志与资产"
        subtitle="保持操作者对文件层的直觉，不必先切去日志页。"
      >
        <div className="artifact-grid">
          <div className="artifact-card">
            <div className="artifact-title">Latest log files</div>
            <ul className="artifact-list mono">
              {panel.logFiles.slice(0, 5).map((item) => (
                <li key={item.name}>
                  <span>{item.name}</span>
                  <span>{formatDate(item.modTime)}</span>
                </li>
              ))}
              {panel.logFiles.length === 0 ? <li>暂无日志文件</li> : null}
            </ul>
          </div>

          <div className="artifact-card emphasis">
            <div className="artifact-title">Operator note</div>
            <p className="artifact-note">
              这个面板的单一任务不是“展示所有配置”，而是帮你更快判断现在该改模型、改人格，还是直接去监听日志流。
            </p>
          </div>
        </div>
      </Panel>
    </div>
  );
}

function KV({ label, value, mono = false }) {
  return (
    <div className="kv-item">
      <div className="kv-label">{label}</div>
      <div className={`kv-value ${mono ? "mono" : ""}`}>{value}</div>
    </div>
  );
}

function ProbeBlock({ title, status, checks }) {
  return (
    <div className="probe-block">
      <div className="probe-block-head">
        <div className="probe-block-title">{title}</div>
        <div className={`inline-status ${status === "ok" ? "good" : status === "unknown" ? "idle" : "warn"}`}>
          {status || "unknown"}
        </div>
      </div>

      <div className="probe-checks">
        {checks.map(([key, value]) => (
          <div key={key} className="probe-check">
            <span className="mono">{key}</span>
            <strong>{value || "-"}</strong>
          </div>
        ))}
        {checks.length === 0 ? <div className="probe-check empty">no checks reported</div> : null}
      </div>
    </div>
  );
}

function formatDate(value) {
  if (!value) {
    return "-";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return String(value);
  }

  return date.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  });
}
