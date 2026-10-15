import { useEffect, useRef, useState } from "react";
import { adminApi, openEventSocket } from "../api/adminApi";

export const MAX_VISIBLE_EVENTS = 500;
const validCursor = (value) =>
  typeof value === "string" && /^(0|[1-9][0-9]*)$/.test(value);
const gapText = (reason) =>
  reason === "retention_gap"
    ? "事件历史已超出保留窗口，部分事件无法补回；旧视图已清空，恢复后仅显示近期历史。"
    : "服务或事件游标已重置；旧视图已清空，恢复后仅显示近期历史。";

export function useEventStream(filters) {
  const key = JSON.stringify(filters);
  const sessionRef = useRef(null);
  const [paused, setPaused] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState({
    key: null,
    events: [],
    status: "loading",
  });

  useEffect(() => {
    if (!sessionRef.current || sessionRef.current.key !== key) {
      sessionRef.current = {
        key,
        cursor: "0",
        streamId: "",
        initialized: false,
        events: [],
        warning: "",
        evicted: 0,
      };
    }
    const session = sessionRef.current;
    let active = true;
    let socket = null;
    let retryTimer = null;
    let watchdog = null;
    let failures = 0;
    let resets = 0;
    let socketGeneration = 0;
    const controller = new AbortController();
    const publish = (status, extra = {}) => {
      if (active)
        setState({
          key,
          events: session.events,
          cursor: session.cursor,
          streamId: session.streamId,
          warning: session.warning,
          evicted: session.evicted,
          status,
          error: "",
          ...extra,
        });
    };
    const closeSocket = () => {
      socketGeneration++;
      clearTimeout(watchdog);
      if (!socket) return;
      socket.onopen = socket.onmessage = socket.onclose = socket.onerror = null;
      socket.close();
      socket = null;
    };
    const append = (events) => {
      const merged = [...session.events, ...events];
      session.evicted += Math.max(0, merged.length - MAX_VISIBLE_EVENTS);
      session.events = merged.slice(-MAX_VISIBLE_EVENTS);
    };
    const applyPage = (page) => {
      if (
        !validCursor(page.cursor) ||
        !page.streamId ||
        !Array.isArray(page.events) ||
        page.events.length !== page.eventCursors?.length
      )
        throw new Error("服务未提供完整事件游标协议，请升级后重试。");
      const events = [];
      let last = session.initialized ? session.cursor : "0";
      for (let index = 0; index < page.events.length; index++) {
        const cursor = page.eventCursors[index];
        if (!validCursor(cursor)) throw new Error("事件游标无效。");
        if (BigInt(cursor) > BigInt(last)) {
          events.push({ ...page.events[index], cursor });
          last = cursor;
        }
      }
      if (BigInt(page.cursor) < BigInt(last))
        throw new Error("事件检查点早于已接收事件。");
      append(events);
      session.cursor = page.cursor;
      session.streamId = page.streamId;
      session.initialized = true;
    };
    const expired = () => {
      clearTimeout(retryTimer);
      closeSocket();
      publish("unauthorized", { error: "登录已失效，请重新登录。" });
      window.dispatchEvent(new Event("admin:unauthorized"));
    };
    const fail = (error) => {
      if (!active || error.name === "AbortError") return;
      if (error.status === 401) {
        expired();
        return;
      }
      if (
        error.status === 409 &&
        ["retention_gap", "cursor_reset", "stream_changed"].includes(error.code)
      ) {
        reset(error.code);
        return;
      }
      if ([400, 403, 501].includes(error.status) || error.protocol) {
        publish("error", { error: error.message });
        return;
      }
      schedule(error.message);
    };
    const schedule = (error = "连接已断开") => {
      if (!active) return;
      closeSocket();
      clearTimeout(retryTimer);
      const retryIn = Math.min(1000 * 2 ** Math.min(failures++, 5), 30000);
      publish("reconnecting", { error, retryIn });
      retryTimer = setTimeout(connect, retryIn);
    };
    const reset = (reason) => {
      closeSocket();
      clearTimeout(retryTimer);
      session.events = [];
      session.cursor = "0";
      session.streamId = "";
      session.initialized = false;
      session.evicted = 0;
      session.warning = gapText(reason);
      publish("loading");
      if (++resets > 1) schedule("事件历史持续变化，稍后重新建立基线");
      else void connect();
    };
    const armWatchdog = (timeout = 15000) => {
      clearTimeout(watchdog);
      watchdog = setTimeout(() => schedule("事件连接超时"), timeout);
    };
    async function connect() {
      if (!active || paused) return;
      publish(session.initialized ? "connecting" : "loading");
      try {
        // Revalidate auth and consume one replay batch before every handshake.
        // The socket continues from this exact checkpoint, including empty scans.
        const page = await adminApi.getEventPage(
          {
            ...filters,
            limit: 200,
            ...(session.initialized
              ? { after: session.cursor, stream_id: session.streamId }
              : {}),
          },
          { signal: controller.signal, notifyUnauthorized: false },
        );
        if (!active) return;
        if (session.initialized && page.streamId !== session.streamId) {
          reset("stream_changed");
          return;
        }
        try {
          applyPage(page);
        } catch (error) {
          error.protocol = true;
          throw error;
        }
        publish("connecting");
        socket = openEventSocket({
          ...filters,
          after: session.cursor,
          stream_id: session.streamId,
          limit: 200,
        });
        const generation = ++socketGeneration;
        const isCurrent = () => active && generation === socketGeneration;
        armWatchdog(10000);
        socket.onopen = () => {
          if (isCurrent()) {
            publish("catching_up");
            armWatchdog();
          }
        };
        socket.onmessage = ({ data }) => {
          if (!isCurrent()) return;
          armWatchdog();
          try {
            const frame = JSON.parse(data);
            if (["gap", "reset"].includes(frame.type)) {
              reset(frame.reason);
              return;
            }
            if (frame.streamId !== session.streamId) {
              reset("stream_changed");
              return;
            }
            if (frame.type === "event") {
              if (!validCursor(frame.cursor)) throw new Error("事件游标无效。");
              if (BigInt(frame.cursor) <= BigInt(session.cursor)) return;
              append([{ ...frame.event, cursor: frame.cursor }]);
              session.cursor = frame.cursor;
              publish("catching_up");
            } else if (frame.type === "checkpoint") {
              if (
                !validCursor(frame.cursor) ||
                BigInt(frame.cursor) < BigInt(session.cursor)
              )
                throw new Error("事件检查点无效。");
              session.cursor = frame.cursor;
              if (!frame.hasMore) {
                failures = 0;
                resets = 0;
              }
              publish(frame.hasMore ? "catching_up" : "live");
            }
          } catch (error) {
            closeSocket();
            publish("error", { error: `事件协议错误：${error.message}` });
          }
        };
        socket.onclose = (event) => {
          if (!isCurrent()) return;
          if (event.code === 4001) expired();
          else schedule();
        };
        socket.onerror = () => {
          if (isCurrent()) schedule("事件连接失败");
        };
      } catch (error) {
        fail(error);
      }
    }
    if (paused) publish("paused");
    else void connect();
    return () => {
      active = false;
      controller.abort();
      clearTimeout(retryTimer);
      closeSocket();
    };
  }, [key, paused, attempt]);

  const visible =
    state.key === key ? state : { events: [], status: "loading", cursor: "0" };
  return {
    ...visible,
    paused,
    togglePaused: () => setPaused((value) => !value),
    retry: () => setAttempt((value) => value + 1),
    reload: () => {
      sessionRef.current = null;
      setAttempt((value) => value + 1);
    },
  };
}
