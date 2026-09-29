export function Sidebar({ items, active, onChange, panel }) {
  return (
    <aside className="sidebar">
      <div className="brand">
        <div className="brand-mark">
          <span className="brand-mark-core" />
        </div>
        <div>
          <div className="brand-title">ReEscape</div>
          <div className="brand-subtitle">管理控制台</div>
        </div>
      </div>

      <nav className="menu" aria-label="主导航">
        {items.map((item) => {
          const selected = item.key === active;
          return (
            <button
              key={item.key}
              type="button"
              className={`menu-item ${selected ? "active" : ""}`}
              onClick={() => onChange(item.key)}
              title={item.note}
              aria-current={selected ? "page" : undefined}
            >
              <span className="menu-icon">{item.icon}</span>
              <span className="menu-copy">
                <span className="menu-label">{item.label}</span>
              </span>
            </button>
          );
        })}
      </nav>

      <div className="sidebar-foot">
        <div className="sidebar-eyebrow">运行状态</div>
        <div className="sidebar-status-row">
          <span className={`status-dot ${panel.health.status === "ok" ? "good" : "warn"}`} />
          <span>服务</span>
          <strong>{panel.health.status || "unknown"}</strong>
        </div>
        <div className="sidebar-status-row">
          <span className={`status-dot ${panel.ready.status === "ok" ? "good" : "warn"}`} />
          <span>就绪</span>
          <strong>{panel.ready.status || "unknown"}</strong>
        </div>
        <div className="sidebar-hint mono">{panel.config.environmentConfig || ".env"}</div>
      </div>
    </aside>
  );
}
