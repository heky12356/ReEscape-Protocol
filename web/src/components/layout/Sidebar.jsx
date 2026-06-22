export function Sidebar({ items, active, onChange, panel }) {
  return (
    <aside className="sidebar">
      <div className="brand">
        <div className="brand-mark">
          <span className="brand-mark-core" />
        </div>
        <div>
          <div className="brand-title">ReEscape</div>
          <div className="brand-subtitle">Operator deck for tuning voice, state, and telemetry.</div>
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
            >
              <span className="menu-icon">{item.icon}</span>
              <span className="menu-copy">
                <span className="menu-label">{item.label}</span>
                <span className="menu-note">{item.note}</span>
              </span>
            </button>
          );
        })}
      </nav>

      <div className="sidebar-foot">
        <div className="sidebar-eyebrow">System pulse</div>
        <div className="sidebar-probes">
          <ProbeCard label="Health" value={panel.health.status} />
          <ProbeCard label="Ready" value={panel.ready.status} />
        </div>
        <div className="sidebar-hint mono">{panel.config.environmentConfig || ".env"}</div>
      </div>
    </aside>
  );
}

function ProbeCard({ label, value }) {
  const tone = value === "ok" ? "good" : value === "unknown" ? "idle" : "warn";

  return (
    <div className={`probe-card ${tone}`}>
      <span>{label}</span>
      <strong>{value || "unknown"}</strong>
    </div>
  );
}
