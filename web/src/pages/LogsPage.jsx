import {
  startTransition,
  useDeferredValue,
  useEffect,
  useMemo,
  useState
} from "react";
import { getAdminAPIKeyForStream } from "../api/adminApi";
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
  const recentEvents = (panel.events || []).slice(-6).reverse();

  useEffect(() => {
    if (!streamEnabled) {
      setConnected(false);
      setStreamState("paused");
      return undefined;
    }

    const controller = new AbortController();
    setStreamState("connecting...");

    const handleEvent = (type, data) => {
      const payload = parseSSEData({ data });
      if (type === "init" || type === "reset") {
        startTransition(() => {
          setTerminalContent(compactContent(payload.content || ""));
        });
        if (payload.file && payload.file !== selectedLogFile) {
          setSelectedLogFile(payload.file);
        }
      } else if (type === "append" && payload.content) {
        startTransition(() => {
          setTerminalContent((prev) => compactContent(prev + payload.content));
        });
      } else if (type === "error") {
        setStreamState(payload.error ? `error: ${payload.error}` : "stream error");
        setConnected(false);
      }
    };

    async function consumeStream() {
      while (!controller.signal.aborted) {
        try {
        const headers = {};
        const apiKey = getAdminAPIKeyForStream();
        if (apiKey) {
          headers.Authorization = `Bearer ${apiKey}`;
        }
        const response = await fetch(streamUrl, { headers, signal: controller.signal });
        if (!response.ok || !response.body) {
          throw new Error(`stream request failed (${response.status})`);
        }
        setConnected(true);
        setStreamState("streaming");
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";
        while (!controller.signal.aborted) {
          const { value, done } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          const events = buffer.split("\n\n");
          buffer = events.pop() || "";
          for (const event of events) {
            let type = "message";
            let data = "";
            for (const line of event.split("\n")) {
              if (line.startsWith("event:")) type = line.slice(6).trim();
              if (line.startsWith("data:")) data += line.slice(5).trim();
            }
            if (data) handleEvent(type, data);
          }
        }
        } catch (error) {
          if (controller.signal.aborted) break;
          setConnected(false);
          setStreamState(error?.message || "reconnecting...");
        }
        if (!controller.signal.aborted) {
          await new Promise((resolve) => setTimeout(resolve, 1000));
          setStreamState("reconnecting...");
        }
      }
    }
    void consumeStream();

    return () => {
      controller.abort();
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
            <div className="artifact-card">
              <div className="artifact-title">ReAct events</div>
              <ul className="artifact-list">
                {recentEvents.map((event) => (
                  <li key={event.id || `${event.type}-${event.created_at}`}>
                    <span>{formatEvent(event)}</span>
                    <span>{formatEventTime(event.created_at)}</span>
                  </li>
                ))}
                {recentEvents.length === 0 ? <li>暂无事件</li> : null}
              </ul>
            </div>
          </aside>
        </div>
      </Panel>
    </div>
  );
}

function formatEvent(event) {
  const type = event.type || "-";
  if (event.tool) {
    return `${type} · ${event.tool}`;
  }
  return type;
}

function formatEventTime(value) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return String(value);
  }
  return date.toLocaleTimeString("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit"
  });
}
