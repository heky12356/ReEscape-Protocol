import { MetricCard } from "../components/common/MetricCard";
import { Panel } from "../components/common/Panel";

export function OverviewPage({ panel }) {
  const latestLogFile = panel.logFiles?.[0]?.name || "-";
  const logCount = panel.logFiles?.length || 0;

  return (
    <div className="stack">
      <div className="metric-grid">
        <MetricCard label="AI 模型" value={panel.config.aiModel || "-"} />
        <MetricCard label="人格文件" value={panel.config.character || "-"} />
        <MetricCard label="日志文件数" value={String(logCount)} />
        <MetricCard label="最近日志" value={latestLogFile} />
      </div>

      <Panel title="运行状态" subtitle="这里展示当前可操作的管理信息">
        <div className="kv-grid">
          <KV label="AI Base URL" value={panel.config.aiBaseUrl || "-"} />
          <KV label="环境文件" value={panel.config.environmentConfig || "-"} mono />
          <KV
            label="AI Key"
            value={panel.config.aiKeySet ? panel.config.aiKeyMasked : "未设置"}
          />
          <KV label="Timeout / Retry" value={`${panel.config.aiTimeout}s / ${panel.config.aiRetryCount}`} />
          <KV label="Rate Limit" value={`${panel.config.aiRateLimit}/min`} />
          <KV label="Top P / Temperature" value={`${panel.config.aiTopP} / ${panel.config.aiTemperature}`} />
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
