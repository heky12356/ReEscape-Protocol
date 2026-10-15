import { useCallback, useState } from "react";
import { adminApi } from "../api/adminApi";
import { Panel } from "../components/common/Panel";
import { useStateQuery } from "../hooks/useStateQuery";

const KINDS = [
  ["sessions", "会话", "Session"],
  ["intents", "意图", "Intent"],
  ["open_loops", "待办", "OpenLoop"],
  ["deliveries", "投递", "Delivery"],
];
const initialFilters = { user_id: "", session_id: "", status: "", limit: "25" };

export function StatePage() {
  const loadInfo = useCallback(
    (signal) => adminApi.getPersistence({ signal }),
    [],
  );
  const info = useStateQuery("persistence", loadInfo);
  if (info.loading) return <Notice>正在检查持久化查询能力…</Notice>;
  if (info.error) return <QueryError error={info.error} onRetry={info.retry} />;
  if (!info.data?.ready)
    return (
      <Notice>
        持久化尚未就绪。
        <button className="btn-ghost" onClick={info.retry}>
          重新检查
        </button>
      </Notice>
    );
  if (
    info.data.backend !== "sqlite" ||
    !info.data.capabilities?.includes("state_query")
  ) {
    return (
      <Panel
        eyebrow="PERSISTENCE"
        title="当前后端不支持状态查询"
        subtitle={`实际运行后端：${info.data.backend}`}
      >
        <p>
          文件后端仍正常保存数据。会话、聊天和状态查询需要
          SQLite；请先按运维文档迁移数据，修改后端配置并重启服务。
        </p>
        <button className="btn-ghost" onClick={info.retry}>
          重新检查
        </button>
      </Panel>
    );
  }
  return <StateBrowser info={info.data} />;
}

function StateBrowser({ info }) {
  const [kind, setKind] = useState("sessions");
  const [draft, setDraft] = useState(initialFilters);
  const [filters, setFilters] = useState(initialFilters);
  const [cursors, setCursors] = useState([""]);
  const [selected, setSelected] = useState("");
  const [refresh, setRefresh] = useState(0);
  const cursor = cursors[cursors.length - 1];
  const queryKey = JSON.stringify([kind, filters, cursor, refresh]);
  const load = useCallback(
    (signal) => adminApi.getState(kind, { ...filters, cursor }, { signal }),
    [kind, filters, cursor],
  );
  const page = useStateQuery(queryKey, load);
  const restart = () => {
    setCursors([""]);
    setSelected("");
    setRefresh((value) => value + 1);
  };
  const navigate = (target, id = "", sessionID = "") => {
    const next = { ...initialFilters, session_id: sessionID };
    setKind(target);
    setDraft(next);
    setFilters(next);
    setCursors([""]);
    setSelected(id);
  };
  const label = KINDS.find(([key]) => key === kind)[1];
  return (
    <div className="state-page">
      <div className="state-toolbar">
        <nav className="state-tabs" aria-label="状态类型">
          {KINDS.map(([key, name, code]) => (
            <button
              key={key}
              className={`state-tab ${kind === key ? "active" : ""}`}
              aria-pressed={kind === key}
              onClick={() => navigate(key)}
            >
              {name}
              <span className="mono">{code}</span>
            </button>
          ))}
        </nav>
        <span className="state-backend mono">
          SQLite · schema {info.schemaVersion} · 只读
        </span>
      </div>
      <Panel
        title={`${label}查询`}
        subtitle="按更新时间降序浏览已持久化状态。尚未保存的内存状态可能暂未显示。"
      >
        <form
          className="state-filters"
          onSubmit={(event) => {
            event.preventDefault();
            setFilters({
              ...draft,
              user_id: draft.user_id.trim(),
              session_id: draft.session_id.trim(),
              status: kind === "sessions" ? "" : draft.status.trim(),
            });
            restart();
          }}
        >
          <label>
            用户 ID
            <input
              inputMode="numeric"
              pattern="[0-9]*"
              value={draft.user_id}
              onChange={(event) =>
                setDraft({ ...draft, user_id: event.target.value })
              }
              placeholder="全部用户"
            />
          </label>
          <label>
            会话 ID
            <input
              value={draft.session_id}
              onChange={(event) =>
                setDraft({ ...draft, session_id: event.target.value })
              }
              placeholder="全部会话"
            />
          </label>
          {kind !== "sessions" ? (
            <label>
              状态
              <input
                value={draft.status}
                onChange={(event) =>
                  setDraft({ ...draft, status: event.target.value })
                }
                placeholder={
                  kind === "deliveries"
                    ? "如 delivered / failed"
                    : kind === "open_loops"
                      ? "如 open / resolved"
                      : "如 pending"
                }
              />
            </label>
          ) : null}
          <label>
            每页
            <select
              value={draft.limit}
              onChange={(event) =>
                setDraft({ ...draft, limit: event.target.value })
              }
            >
              {[25, 50, 100, 200].map((limit) => (
                <option key={limit}>{limit}</option>
              ))}
            </select>
          </label>
          <button className="btn-primary" type="submit">
            查询
          </button>
          <button className="btn-ghost" type="button" onClick={restart}>
            刷新
          </button>
        </form>
        {page.loading ? <Notice>正在查询{label}…</Notice> : null}
        {page.error ? (
          <QueryError
            error={page.error}
            onRetry={page.error.status === 409 ? restart : page.retry}
          />
        ) : null}
        {page.data ? (
          <>
            <div className="state-table-scroll">
              <table className="state-table">
                <caption className="sr-only">{label}查询结果</caption>
                <thead>
                  <tr>
                    <th scope="col">记录 ID</th>
                    <th scope="col">用户 / 会话</th>
                    <th scope="col">状态</th>
                    <th scope="col">更新时间</th>
                  </tr>
                </thead>
                <tbody>
                  {page.data.records.map((record) => (
                    <tr
                      key={record.id}
                      className={selected === record.id ? "selected" : ""}
                    >
                      <td>
                        <button
                          className="state-link mono"
                          aria-label={`查看 ${record.id}`}
                          onClick={() => setSelected(record.id)}
                        >
                          {record.id}
                        </button>
                      </td>
                      <td>
                        <span className="mono">{record.userId || "—"}</span>
                        {record.sessionId && kind !== "sessions" ? (
                          <button
                            className="state-link state-session mono"
                            onClick={() =>
                              navigate(
                                "sessions",
                                record.sessionId,
                                record.sessionId,
                              )
                            }
                          >
                            {record.sessionId}
                          </button>
                        ) : null}
                      </td>
                      <td>
                        <span className="state-badge">
                          {record.status || "—"}
                        </span>
                      </td>
                      <td>
                        <Time value={record.updatedAt} />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            {page.data.records.length === 0 ? (
              <Notice>没有符合条件的{label}记录。</Notice>
            ) : null}
            <Pager
              label="状态"
              cursors={cursors}
              setCursors={setCursors}
              next={page.data.nextCursor}
              onChange={() => setSelected("")}
            />
            <p className="state-footnote">
              投影同步时间：
              <Time value={page.data.refreshedAt} />
            </p>
          </>
        ) : null}
      </Panel>
      {selected ? (
        <StateDetail
          key={`${kind}:${selected}`}
          kind={kind}
          id={selected}
          navigate={navigate}
          hasChat={info.capabilities.includes("conversation_query")}
          onClose={() => setSelected("")}
        />
      ) : (
        <Notice>选择记录查看详情；在会话详情中分页浏览聊天。</Notice>
      )}
    </div>
  );
}

function StateDetail({ kind, id, navigate, hasChat, onClose }) {
  const load = useCallback(
    (signal) =>
      kind === "sessions"
        ? adminApi.getSessionSummary(id, { signal })
        : adminApi.getStateDetail(kind, id, { signal }),
    [kind, id],
  );
  const detail = useStateQuery(`${kind}:${id}`, load);
  const record = detail.data;
  const payload = record?.payload || {};
  const relatedSession = record?.sessionId || payload.session_id;
  const relatedIntent = record?.intentId || payload.intent_id;
  const relatedLoop = payload.open_loop_id;
  const relatedDelivery = payload.delivery_id;
  const relatedTurn =
    record?.turnId || payload.turn_id || payload.source_turn_id;
  return (
    <Panel
      eyebrow="DETAIL"
      title={kind === "sessions" ? "会话与聊天" : "记录详情"}
      subtitle={id}
      actions={
        <button className="btn-ghost" onClick={onClose}>
          关闭详情
        </button>
      }
    >
      {detail.loading ? <Notice>正在加载详情…</Notice> : null}
      {detail.error ? (
        <QueryError error={detail.error} onRetry={detail.retry} />
      ) : null}
      {record ? (
        <>
          <dl className="state-metadata">
            <div>
              <dt>用户 ID</dt>
              <dd className="mono">{record.userId || "—"}</dd>
            </div>
            <div>
              <dt>状态</dt>
              <dd>{record.status || (payload.current_state ?? "—")}</dd>
            </div>
            <div>
              <dt>更新时间</dt>
              <dd>
                <Time value={record.updatedAt} />
              </dd>
            </div>
          </dl>
          <div className="state-relations" aria-label="关联记录">
            {relatedSession && kind !== "sessions" ? (
              <button
                className="state-link"
                onClick={() =>
                  navigate("sessions", relatedSession, relatedSession)
                }
              >
                会话：{relatedSession}
              </button>
            ) : null}
            {relatedIntent && kind !== "intents" ? (
              <button
                className="state-link"
                onClick={() => navigate("intents", relatedIntent)}
              >
                意图：{relatedIntent}
              </button>
            ) : null}
            {relatedLoop && kind !== "open_loops" ? (
              <button
                className="state-link"
                onClick={() => navigate("open_loops", relatedLoop)}
              >
                待办：{relatedLoop}
              </button>
            ) : null}
            {relatedDelivery && kind !== "deliveries" ? (
              <button
                className="state-link"
                onClick={() => navigate("deliveries", relatedDelivery)}
              >
                投递：{relatedDelivery}
              </button>
            ) : null}
            {relatedTurn ? (
              <span className="mono">Turn：{relatedTurn}</span>
            ) : null}
            {kind === "sessions"
              ? ["intents", "open_loops", "deliveries"].map((target) => (
                  <button
                    key={target}
                    className="state-link"
                    onClick={() => navigate(target, "", id)}
                  >
                    本会话的{KINDS.find(([key]) => key === target)[1]}
                  </button>
                ))
              : null}
          </div>
          <RawPayload
            value={payload}
            label={
              kind === "sessions"
                ? "会话元信息（不含聊天正文）"
                : "原始记录（可能含敏感正文）"
            }
          />
          {kind === "sessions" ? (
            hasChat ? (
              <Conversation id={id} />
            ) : (
              <Notice>当前服务未提供聊天分页能力，请升级服务。</Notice>
            )
          ) : null}
        </>
      ) : null}
    </Panel>
  );
}

function Conversation({ id }) {
  const [cursors, setCursors] = useState([""]);
  const [revision, setRevision] = useState(0);
  const cursor = cursors[cursors.length - 1];
  const load = useCallback(
    (signal) =>
      adminApi.getConversationMessages(id, { limit: 50, cursor }, { signal }),
    [id, cursor],
  );
  const page = useStateQuery(JSON.stringify([id, cursor, revision]), load);
  const restart = () => {
    setCursors([""]);
    setRevision((value) => value + 1);
  };
  return (
    <section className="conversation" aria-label="聊天历史">
      <div className="conversation-heading">
        <div>
          <h4>聊天历史</h4>
          <p>
            按保存顺序从早到晚，每页 50
            条。只显示用户与已投递助手消息；旧消息可能没有时间。
          </p>
        </div>
        <button className="btn-ghost" onClick={restart}>
          刷新聊天
        </button>
      </div>
      {page.loading ? <Notice>正在加载聊天…</Notice> : null}
      {page.error ? (
        <QueryError
          error={page.error}
          onRetry={page.error.status === 409 ? restart : page.retry}
        />
      ) : null}
      {page.data ? (
        <>
          {page.data.messages.length === 0 ? (
            <Notice>此会话没有已保存的聊天消息。</Notice>
          ) : (
            <ol className="conversation-list">
              {page.data.messages.map((message) => (
                <li
                  key={message.sequence}
                  className={`conversation-message ${message.role}`}
                >
                  <div className="conversation-meta">
                    <strong>
                      {message.role === "user" ? "用户" : "助手 · 已投递"}
                    </strong>
                    <span className="mono">#{message.sequence + 1}</span>
                    <Time value={message.recordedAt} />
                  </div>
                  <MessageContent content={message.payload?.content} />
                  <RawPayload
                    value={message.payload}
                    label="原始消息（可能含敏感内容）"
                  />
                </li>
              ))}
            </ol>
          )}
          <Pager
            label="聊天"
            cursors={cursors}
            setCursors={setCursors}
            next={page.data.nextCursor}
          />
        </>
      ) : null}
    </section>
  );
}

function MessageContent({ content }) {
  if (typeof content === "string")
    return <p className="conversation-text">{content || "（空正文）"}</p>;
  if (Array.isArray(content))
    return (
      <div>
        {content.map((part, index) =>
          part?.type === "text" && typeof part.text === "string" ? (
            <p className="conversation-text" key={index}>
              {part.text}
            </p>
          ) : (
            <p className="conversation-attachment" key={index}>
              {part?.type === "image_url"
                ? "图片附件 · 未自动加载"
                : "结构化附件 · 展开原始消息查看"}
            </p>
          ),
        )}
      </div>
    );
  return (
    <p className="conversation-attachment">结构化消息 · 展开原始消息查看</p>
  );
}
function Pager({ label, cursors, setCursors, next, onChange = () => {} }) {
  return (
    <nav className="state-pager" aria-label={`${label}分页`}>
      <span>第 {cursors.length} 页</span>
      <button
        className="btn-ghost"
        disabled={cursors.length === 1}
        onClick={() => {
          setCursors((values) => values.slice(0, -1));
          onChange();
        }}
      >
        {label}上一页
      </button>
      <button
        className="btn-ghost"
        disabled={!next}
        onClick={() => {
          setCursors((values) => [...values, next]);
          onChange();
        }}
      >
        {label}下一页
      </button>
    </nav>
  );
}
function Time({ value }) {
  if (!value || value.startsWith("0001-")) return <span>时间未知</span>;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? (
    <span>时间未知</span>
  ) : (
    <time dateTime={value} title={value}>
      {date.toLocaleString()}
    </time>
  );
}
function RawPayload({ value, label }) {
  return (
    <details className="state-raw">
      <summary>{label}</summary>
      <pre className="mono">{JSON.stringify(value, null, 2)}</pre>
    </details>
  );
}
function Notice({ children }) {
  return (
    <div className="state-notice" role="status">
      {children}
    </div>
  );
}
function QueryError({ error, onRetry }) {
  const text =
    error.status === 409
      ? "数据已变化，请从第一页重新查询。"
      : error.status === 401
        ? "登录已失效，请重新登录。"
        : error.status === 404
          ? "记录不存在，可能已被清理。"
          : `查询失败：${error.message}`;
  return (
    <div className="banner error" role="alert">
      <span>{text}</span>
      {error.status !== 401 ? (
        <button className="btn-ghost" onClick={onRetry}>
          {error.status === 409 ? "重新查询" : "重试"}
        </button>
      ) : null}
    </div>
  );
}
