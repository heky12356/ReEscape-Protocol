import { useState } from "react";
import { Panel } from "../components/common/Panel";
import { MAX_VISIBLE_EVENTS, useEventStream } from "../hooks/useEventStream";

const FILTERS = [
  ["type", "事件类型"],
  ["request_id", "请求 ID"],
  ["turn_id", "Turn ID"],
  ["session_id", "会话 ID"],
  ["intent_id", "意图 ID"],
  ["delivery_id", "投递 ID"],
  ["tool", "工具名称"],
];
const EMPTY_FILTERS = Object.fromEntries(FILTERS.map(([key]) => [key, ""]));
const STATUS = {
  loading: "加载近期历史",
  connecting: "连接中",
  catching_up: "补发中",
  live: "实时连接",
  reconnecting: "等待重连",
  paused: "已暂停",
  error: "连接异常",
  unauthorized: "登录失效",
};

export function EventsPage() {
  const [draft, setDraft] = useState(EMPTY_FILTERS);
  const [filters, setFilters] = useState(EMPTY_FILTERS);
  const [selected, setSelected] = useState("");
  const stream = useEventStream(filters);
  const detail = stream.events.find((event) => event.cursor === selected);
  const follow = (key, value) => {
    const next = { ...EMPTY_FILTERS, [key]: value };
    setDraft(next);
    setFilters(next);
    setSelected("");
  };
  const apply = (event) => {
    event.preventDefault();
    setFilters(
      Object.fromEntries(
        Object.entries(draft).map(([key, value]) => [key, value.trim()]),
      ),
    );
    setSelected("");
  };
  return (
    <div className="events-page">
      <Panel
        eyebrow="EVENT DIAGNOSTICS"
        title="事件诊断"
        subtitle="先加载近期历史，再从检查点接收实时事件。所有筛选为精确匹配，多项条件同时满足。"
        actions={
          <>
            <button className="btn-ghost" onClick={stream.togglePaused}>
              {stream.paused ? "继续接收" : "暂停接收"}
            </button>
            <button
              className="btn-ghost"
              onClick={() => {
                stream.reload();
                setSelected("");
              }}
            >
              重新加载近期历史
            </button>
          </>
        }
      >
        <div className="events-status" role="status">
          <span
            className={`status-dot ${stream.status === "live" ? "good" : "warn"}`}
          />
          <strong>{STATUS[stream.status]}</strong>
          {stream.status === "reconnecting" ? (
            <span>{stream.retryIn / 1000} 秒后重试</span>
          ) : null}
          <span className="mono">cursor {stream.cursor || "0"}</span>
          <span>当前 {stream.events.length} 条</span>
        </div>
        {stream.warning ? (
          <div className="banner events-warning" role="alert">
            {stream.warning}
          </div>
        ) : null}
        {stream.error ? (
          <div className="banner error" role="alert">
            <span>{stream.error}</span>
            {stream.status === "error" ? (
              <button className="btn-ghost" onClick={stream.retry}>
                重试连接
              </button>
            ) : null}
          </div>
        ) : null}
        <form className="events-filters" onSubmit={apply}>
          {FILTERS.map(([key, label]) => (
            <label key={key}>
              {label}
              <input
                value={draft[key]}
                onChange={(event) =>
                  setDraft({ ...draft, [key]: event.target.value })
                }
                placeholder={key === "tool" ? "兼容历史 tool 字段" : "全部"}
              />
            </label>
          ))}
          <div className="action-row">
            <button className="btn-primary" type="submit">
              应用筛选
            </button>
            <button
              className="btn-ghost"
              type="button"
              onClick={() => {
                setDraft(EMPTY_FILTERS);
                setFilters(EMPTY_FILTERS);
                setSelected("");
              }}
            >
              清除筛选
            </button>
          </div>
        </form>
        <p className="state-footnote">
          首次仅扫描最近 200 条事件；筛选可能使显示数量更少。界面保留最近{" "}
          {MAX_VISIBLE_EVENTS} 条匹配事件
          {stream.evicted ? `，本视图已移出 ${stream.evicted} 条` : ""}
          。暂停会断开连接，继续时尝试补发；file 后端仅保留最近 1000 条事实。
        </p>
        <div className="state-table-scroll">
          <table className="state-table events-table">
            <caption className="sr-only">事件诊断结果</caption>
            <thead>
              <tr>
                <th scope="col">Cursor</th>
                <th scope="col">事件</th>
                <th scope="col">关联</th>
                <th scope="col">发生时间</th>
              </tr>
            </thead>
            <tbody>
              {[...stream.events].reverse().map((event) => (
                <tr
                  key={event.cursor}
                  className={selected === event.cursor ? "selected" : ""}
                >
                  <td className="mono">{event.cursor}</td>
                  <td>
                    <button
                      className="state-link"
                      aria-label={`查看事件 ${event.cursor}`}
                      onClick={() => setSelected(event.cursor)}
                    >
                      {event.type}
                    </button>
                    <span className="events-id mono">{event.id}</span>
                  </td>
                  <td>
                    <span className="events-id mono">
                      {event.session_id ||
                        event.turn_id ||
                        event.request_id ||
                        "—"}
                    </span>
                    {event.tool_name || event.tool ? (
                      <span className="events-id">
                        {event.tool_name || event.tool}
                      </span>
                    ) : null}
                  </td>
                  <td>
                    <EventTime value={event.created_at} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {stream.events.length === 0 ? (
          <p className="state-notice">
            {stream.status === "loading"
              ? "正在加载事件…"
              : "当前视图没有匹配事件。"}
          </p>
        ) : null}
      </Panel>
      {detail ? (
        <Panel
          eyebrow="CORRELATION"
          title="事件详情与关联追踪"
          subtitle={`${detail.type} · ${detail.id}`}
          actions={
            <button className="btn-ghost" onClick={() => setSelected("")}>
              关闭详情
            </button>
          }
        >
          <p className="state-footnote">
            点击关联值，只查询该关联的事件。历史正文不会触发发送或工具执行。
          </p>
          <dl className="events-correlations">
            {FILTERS.map(([key, label]) => {
              const value =
                key === "tool" ? detail.tool_name || detail.tool : detail[key];
              return (
                <div key={key}>
                  <dt>{label}</dt>
                  <dd>
                    {value ? (
                      <button
                        className="state-link mono"
                        onClick={() => follow(key, value)}
                        aria-label={`追踪${label} ${value}`}
                      >
                        {value}
                      </button>
                    ) : (
                      "—"
                    )}
                  </dd>
                </div>
              );
            })}
          </dl>
          <details className="state-raw">
            <summary>原始事件（可能含敏感正文与 URL）</summary>
            <pre className="mono">{JSON.stringify(detail, null, 2)}</pre>
          </details>
        </Panel>
      ) : (
        <p className="state-notice">
          选择事件查看关联字段与原始内容。原始内容默认折叠，远端附件不会自动加载。
        </p>
      )}
    </div>
  );
}

function EventTime({ value }) {
  const date = new Date(value);
  return !value || value.startsWith("0001-") || Number.isNaN(date.getTime()) ? (
    <span>时间未知</span>
  ) : (
    <time dateTime={value} title={value}>
      {date.toLocaleString()}
    </time>
  );
}
