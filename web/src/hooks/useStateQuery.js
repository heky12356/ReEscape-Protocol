import { useEffect, useState } from "react";

// Cancellation saves work; the active guard also protects against transports
// that complete after abort. Hide previous data immediately on a key change.
export function useStateQuery(key, request, enabled = true) {
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState({
    key: null,
    data: null,
    error: null,
    loading: false,
  });
  useEffect(() => {
    if (!enabled) return;
    const controller = new AbortController();
    let active = true;
    setState({ key, data: null, error: null, loading: true });
    Promise.resolve()
      .then(() => {
        if (!active) return;
        return request(controller.signal);
      })
      .then((data) => {
        if (active) setState({ key, data, error: null, loading: false });
      })
      .catch((error) => {
        if (active && error.name !== "AbortError")
          setState({ key, data: null, error, loading: false });
      });
    return () => {
      active = false;
      controller.abort();
    };
  }, [key, request, enabled, attempt]);
  const visible =
    enabled && state.key === key
      ? state
      : { data: null, error: null, loading: enabled };
  return { ...visible, retry: () => setAttempt((value) => value + 1) };
}
