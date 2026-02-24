export function Sidebar({ items, active, onChange }) {
  return (
    <aside className="sidebar">
      <div className="brand">
        {/* <div className="brand-logo">R</div> */}
        <div>
          <div className="brand-title">ReEscape</div>
          <div className="brand-subtitle">Admin Panel</div>
        </div>
      </div>

      <nav className="menu">
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
              <span>{item.label}</span>
            </button>
          );
        })}
      </nav>
    </aside>
  );
}
