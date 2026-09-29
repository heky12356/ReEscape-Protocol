import { useMemo, useState } from "react";
import { Sidebar } from "./components/layout/Sidebar";
import { OverviewPage } from "./pages/OverviewPage";
import { AIConfigPage } from "./pages/AIConfigPage";
import { PromptPage } from "./pages/PromptPage";
import { LogsPage } from "./pages/LogsPage";
import { useAdminPanel } from "./hooks/useAdminPanel";

const MENU_ITEMS = [
  {
    key: "overview",
    label: "总览",
    icon: "◌",
    note: "系统姿态",
    description: "在一张观测台上看清模型路由、人格源文件、日志脉搏和当前运行姿态。"
  },
  {
    key: "ai",
    label: "AI 配置",
    icon: "AI",
    note: "模型调音",
    description: "切换配置档、调整采样和超时策略，把模型行为调到你要的工作带宽。"
  },
  {
    key: "prompt",
    label: "人格与提示词",
    icon: "Ψ",
    note: "声音设计",
    description: "维护人格文件、补充系统提示词，并检查最终真正送给模型的完整提示词。"
  },
  {
    key: "logs",
    label: "日志流",
    icon: "≋",
    note: "监听室",
    description: "把实时输出当作一条信号带来观察，定位异常、节奏变化和配置生效情况。"
  }
];

export default function App() {
  const [active, setActive] = useState("overview");
  const panel = useAdminPanel();

  const currentSection = useMemo(
    () => MENU_ITEMS.find((item) => item.key === active) || MENU_ITEMS[0],
    [active]
  );

  return (
    <div className="app-shell">
      <Sidebar items={MENU_ITEMS} active={active} onChange={setActive} panel={panel} />

      <main className="console-shell">
        <header className="page-header">
          <div className="page-heading">
            <div className="eyebrow">ReEscape Protocol / 管理控制台</div>
            <div className="page-title-row">
              <h1 className="page-title">{currentSection.label}</h1>
              <span className="page-context">{currentSection.note}</span>
            </div>
            <p className="page-lead">{currentSection.description}</p>
          </div>

          <div className="page-status" aria-label="当前运行状态">
            <span className={`status-dot ${panel.ready.status === "ok" ? "good" : "warn"}`} />
            <strong>{panel.ready.status === "ok" ? "服务正常" : "需要检查"}</strong>
            <span className="page-status-meta mono">{panel.config.aiProfile || "default"}</span>
          </div>
        </header>

        <div className="command-strip">
          <div className="command-chip">
            <span>环境配置文件</span>
            <strong className="mono">{panel.config.environmentConfig}</strong>
          </div>
          <div className="command-chip">
            <span>模型配置档</span>
            <strong>{panel.digest.profileCount}</strong>
          </div>
          <div className="command-chip">
            <span>人格文件</span>
            <strong>{panel.digest.characterCount}</strong>
          </div>
          <div className="command-chip">
            <span>日志文件</span>
            <strong>{panel.digest.logCount}</strong>
          </div>
        </div>

        {panel.error ? <div className="banner error">{panel.error}</div> : null}
        {panel.status ? <div className="banner ok">{panel.status}</div> : null}

        <section className="page-wrap">
          {active === "overview" ? <OverviewPage panel={panel} /> : null}
          {active === "ai" ? <AIConfigPage panel={panel} /> : null}
          {active === "prompt" ? <PromptPage panel={panel} /> : null}
          {active === "logs" ? <LogsPage panel={panel} /> : null}
        </section>
      </main>
    </div>
  );
}
