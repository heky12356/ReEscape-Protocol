import { useEffect, useMemo, useState } from "react";
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
      setTerminalContent(compactContent(payload.content || ""));
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
      setTerminalContent((prev) => compactContent(prev + delta));
    });

    source.addEventListener("reset", (event) => {
      const payload = parseSSEData(event);
      setTerminalContent(compactContent(payload.content || ""));
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
    <Panel
      title="Logs"
      subtitle={`live stream from logs directory (${streamState})`}
      actions={
        <div className="action-row">
          <button
            type="button"
            className="btn-ghost"
            onClick={() => setStreamEnabled((prev) => !prev)}
          >
            {streamEnabled ? "Pause Stream" : "Resume Stream"}
          </button>
          <button
            type="button"
            className="btn-ghost"
            onClick={() => void panel.loadLogFiles()}
            disabled={panel.loadingLogs}
          >
            Refresh Files
          </button>
        </div>
      }
    >
      <div className="logs-toolbar">
        <SelectField
          label="Log File"
          value={panel.selectedLogFile}
          options={panel.logFiles.map((item) => item.name)}
          onChange={(value) => panel.setSelectedLogFile(value)}
        />
        <InputField
          label="Initial Lines"
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
        content={terminalContent}
        connected={connected && streamEnabled}
        autoScroll={autoScroll}
        onToggleAutoScroll={() => setAutoScroll((prev) => !prev)}
        onClear={() => setTerminalContent("")}
      />
    </Panel>
  );
}
