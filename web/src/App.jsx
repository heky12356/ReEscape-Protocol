import { useMemo, useState } from "react";
import { Sidebar } from "./components/layout/Sidebar";
import { OverviewPage } from "./pages/OverviewPage";
import { AIConfigPage } from "./pages/AIConfigPage";
import { PromptPage } from "./pages/PromptPage";
import { LogsPage } from "./pages/LogsPage";
import { useAdminPanel } from "./hooks/useAdminPanel";

const MENU_ITEMS = [
  { key: "overview", label: "基础信息", icon: "☰" },
  { key: "ai", label: "AI 配置", icon: "⚙" },
  { key: "prompt", label: "人格与 Prompt", icon: "✦" },
  { key: "logs", label: "日志查看", icon: "🗒" }
];

export default function App() {
  const [active, setActive] = useState("overview");
  const panel = useAdminPanel();

  const title = useMemo(() => {
    const matched = MENU_ITEMS.find((item) => item.key === active);
    return matched?.label || "控制台";
  }, [active]);

  return (
    <div className="app-root">
      <Sidebar items={MENU_ITEMS} active={active} onChange={setActive} />

      <main className="main-area">
        <header className="top-header">
          <div className="top-title">{title}</div>
          <div className="top-subtitle">
            配置文件: <span className="mono">{panel.config.environmentConfig}</span>
          </div>
        </header>

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
