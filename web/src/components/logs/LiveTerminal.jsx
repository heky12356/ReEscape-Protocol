import { useEffect, useRef } from "react";

export function LiveTerminal({
  content,
  connected,
  autoScroll,
  onToggleAutoScroll,
  onClear
}) {
  const bodyRef = useRef(null);

  useEffect(() => {
    if (!autoScroll || !bodyRef.current) {
      return;
    }
    bodyRef.current.scrollTop = bodyRef.current.scrollHeight;
  }, [content, autoScroll]);

  return (
    <section className="terminal-shell">
      <header className="terminal-toolbar">
        <div className="terminal-dot-group" aria-hidden="true">
          <span className="terminal-dot amber" />
          <span className="terminal-dot sky" />
          <span className="terminal-dot foam" />
        </div>
        <div className="terminal-title">Signal tape / live stream</div>
        <div className="terminal-actions">
          <span className={`terminal-status ${connected ? "on" : "off"}`}>
            {connected ? "streaming" : "paused"}
          </span>
          <button type="button" className="btn-ghost" onClick={onToggleAutoScroll}>
            {autoScroll ? "锁定视图" : "跟随输出"}
          </button>
          <button type="button" className="btn-ghost" onClick={onClear}>
            清空
          </button>
        </div>
      </header>

      <pre ref={bodyRef} className="terminal-body">
        {content || "# waiting for fresh signal..."}
      </pre>
    </section>
  );
}
