import {
  startTransition,
  useDeferredValue,
  useEffect,
  useMemo,
  useState
} from "react";
import { Panel } from "../components/common/Panel";
import { InputField, SelectField } from "../components/common/FormField";
import { LiveTerminal } from "../components/logs/LiveTerminal";

const MAX_TERMINAL_CHARS = 300000;

function compactContent(text) {
  if (text.length <= MAX_TERMINAL_CHARS) {
    return text;
  }
  return text.slice(text.length - MAX_TERMINAL_CHARS);
}

function parseSSEData(event) {
  try {
    return JSON.parse(event.data);
  } catch {
    return {};
  }
}

export function LogsPage({ panel }) {
  const [streamEnabled, setStreamEnabled] = useState(true);
  const [connected, setConnected] = useState(false);
  const [autoScroll, setAutoScroll] = useState(true);
  const [terminalContent, setTerminalContent] = useState("");
  const [streamState, setStreamState] = useState("connecting...");
  const deferredTerminalContent = useDeferredValue(terminalContent);

  const streamUrl = useMemo(() => {
    const file = encodeURIComponent(panel.selectedLogFile || "");
    const lines = Number(panel.logLines) > 0 ? Number(panel.logLines) : 200;
    return `/api/admin/logs/stream?file=${file}&lines=${lines}`;
  }, [panel.selectedLogFile, panel.logLines]);

  const selectedLogFile = panel.selectedLogFile;
  const setSelectedLogFile = panel.setSelectedLogFile;

  useEffect(() => {
    if (!streamEnabled) {
      setConnected(false);
      setStreamState("paused");
      return undefined;
    }

    const source = new EventSource(streamUrl);
    setStreamState("connecting...");

    source.onopen = () => {
      setConnected(true);
      setStreamState("streaming");
    };

    source.onerror = () => {
      setConnected(false);
      setStreamState("reconnecting...");
    };

    source.addEventListener("init", (event) => {
      const payload = parseSSEData(event);
      startTransition(() => {
        setTerminalContent(compactContent(payload.content || ""));
      });
      if (payload.file && payload.file !== selectedLogFile) {
        setSelectedLogFile(payload.file);
      }
    });

    source.addEventListener("append", (event) => {
      const payload = parseSSEData(event);
      const delta = payload.content || "";
      if (!delta) {
        return;
      }
      startTransition(() => {
        setTerminalContent((prev) => compactContent(prev + delta));
      });
    });

    source.addEventListener("reset", (event) => {
      const payload = parseSSEData(event);
      startTransition(() => {
        setTerminalContent(compactContent(payload.content || ""));
      });
      if (payload.file && payload.file !== selectedLogFile) {
        setSelectedLogFile(payload.file);
      }
    });

    source.addEventListener("error", (event) => {
      const payload = parseSSEData(event);
      const message = payload.error ? `error: ${payload.error}` : "stream error";
      setStreamState(message);
      setConnected(false);
    });

    return () => {
      source.close();
      setConnected(false);
    };
  }, [selectedLogFile, setSelectedLogFile, streamEnabled, streamUrl]);

  return (
    <div className="stack">
      <Panel
        eyebrow="Listening room"
        title="日志流与文件切换"
        subtitle={`实时监听 logs 目录中的输出 (${streamState})`}
        actions={
          <div className="action-row">
            <button
              type="button"
              className="btn-ghost"
              onClick={() => setStreamEnabled((prev) => !prev)}
            >
              {streamEnabled ? "暂停流" : "恢复流"}
            </button>
            <button
              type="button"
              className="btn-ghost"
              onClick={() => void panel.loadLogFiles()}
              disabled={panel.loadingLogs}
            >
              刷新文件
            </button>
          </div>
        }
      >
        <div className="logs-layout">
          <div className="logs-control-stack">
            <div className="logs-toolbar">
              <SelectField
                label="Log file"
                value={panel.selectedLogFile}
                options={panel.logFiles.map((item) => item.name)}
                onChange={(value) => panel.setSelectedLogFile(value)}
              />
              <InputField
                label="Initial lines"
                type="number"
                value={panel.logLines}
                onChange={(value) => panel.setLogLines(Number(value))}
              />
            </div>

            <div className="logs-meta">
              Files: {panel.logFiles.length}
              {panel.selectedLogFile ? ` | Current: ${panel.selectedLogFile}` : ""}
            </div>

            <LiveTerminal
              content={deferredTerminalContent}
              connected={connected && streamEnabled}
              autoScroll={autoScroll}
              onToggleAutoScroll={() => setAutoScroll((prev) => !prev)}
              onClear={() => setTerminalContent("")}
            />
          </div>

          <aside className="log-aside">
            <div className="artifact-card">
              <div className="artifact-title">监听建议</div>
              <p className="artifact-note">
                当你在调 temperature、切人格或改 prompt 时，最先看的应该是这条信号带，而不是配置表本身。
              </p>
            </div>
            <div className="artifact-card">
              <div className="artifact-title">当前状态</div>
              <ul className="artifact-list">
                <li>
                  <span>stream</span>
                  <span>{streamEnabled ? "enabled" : "paused"}</span>
                </li>
                <li>
                  <span>connection</span>
                  <span>{connected ? "online" : "offline"}</span>
                </li>
                <li>
                  <span>follow output</span>
                  <span>{autoScroll ? "yes" : "no"}</span>
                </li>
              </ul>
            </div>
          </aside>
        </div>
      </Panel>
    </div>
  );
}
