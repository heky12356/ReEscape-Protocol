class AdminApiError extends Error {
  constructor(message, status, code, details) {
    super(message);
    this.name = "AdminApiError";
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

async function fetchJSON(url, options = {}) {
  const { notifyUnauthorized = true, ...fetchOptions } = options;
  const request = {
    ...fetchOptions,
    credentials: "include",
    headers: { ...(fetchOptions.headers || {}) }
  };
  const resp = await fetch(url, request);
  const contentType = resp.headers.get("content-type") || "";
  const isJSON = contentType.includes("application/json");
  const data = isJSON ? await resp.json() : await resp.text();

  if (!resp.ok) {
    if (resp.status === 401 && notifyUnauthorized) {
      window.dispatchEvent(new Event("admin:unauthorized"));
    }
    if (isJSON && (data?.error || data?.reason)) {
      throw new AdminApiError(data.error || data.reason, resp.status, data.code || data.reason, data);
    }
    throw new AdminApiError(typeof data === "string" ? data : "request failed", resp.status);
  }
  return data;
}

export function getAdminSession() {
  return fetchJSON("/api/auth/session", { cache: "no-store", notifyUnauthorized: false });
}

export function loginAdmin(key) {
  return fetchJSON("/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ key }),
    notifyUnauthorized: false
  });
}

export function logoutAdmin() {
  return fetchJSON("/api/auth/logout", {
    method: "POST",
    notifyUnauthorized: false
  });
}

export const adminApi = {
  getTransfers(options = {}) {
    return fetchJSON("/api/admin/transfers", { cache: "no-store", ...options });
  },
  getTransferJob(id, options = {}) {
    return fetchJSON(`/api/admin/transfers/${encodeURIComponent(id)}`, { cache: "no-store", ...options });
  },
  startTransferExport(options = {}) {
    return fetchJSON("/api/admin/transfers/export", { ...options, method: "POST" });
  },
  startSQLiteBackup(options = {}) {
    return fetchJSON("/api/admin/persistence/backup", { ...options, method: "POST" });
  },
  startTransferImport(file, options = {}) {
    const form = new FormData();
    form.append("archive", file);
    return fetchJSON("/api/admin/transfers/import", { ...options, method: "POST", body: form });
  },
  async downloadTransfer(id, options = {}) {
    const job = await this.getTransferJob(id, options);
    if (!job.downloadAvailable) throw new Error("产物已不存在或任务尚未完成，请刷新任务列表。");
    if (options.signal?.aborted) throw new DOMException("Aborted", "AbortError");
    // Let the browser stream large SQLite files instead of buffering a Blob.
    const link = document.createElement("a");
    link.href = `/api/admin/transfers/${encodeURIComponent(id)}/download`;
    link.download = `${id}.${job.kind === "sqlite_backup" ? "sqlite" : "zip"}`;
    document.body.appendChild(link); link.click(); link.remove();
    return job;
  },
  getEventPage(filters = {}, options = {}) {
    return fetchJSON(`/api/admin/events${queryString(filters)}`, { cache: "no-store", ...options });
  },
  getPersistence(options = {}) {
    return fetchJSON("/api/admin/persistence", { cache: "no-store", ...options });
  },
  getState(kind, filters = {}, options = {}) {
    return fetchJSON(`/api/admin/state/${encodeURIComponent(kind)}${queryString(filters)}`, { cache: "no-store", ...options });
  },
  getStateDetail(kind, id, options = {}) {
    return fetchJSON(`/api/admin/state/${encodeURIComponent(kind)}/${encodeURIComponent(id)}`, { cache: "no-store", ...options });
  },
  getSessionSummary(id, options = {}) {
    return fetchJSON(`/api/admin/sessions/${encodeURIComponent(id)}`, { cache: "no-store", ...options });
  },
  getConversationMessages(id, filters = {}, options = {}) {
    return fetchJSON(`/api/admin/sessions/${encodeURIComponent(id)}/messages${queryString(filters)}`, { cache: "no-store", ...options });
  },
  getHealth() {
    return fetchJSON("/healthz", { cache: "no-store" });
  },
  getReady() {
    return fetchJSON("/readyz", { cache: "no-store" });
  },
  getConfig() {
    return fetchJSON("/api/admin/config");
  },
  updateConfig(payload) {
    return fetchJSON("/api/admin/config", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload)
    });
  },
  getAIProfile(name) {
    return fetchJSON(`/api/admin/ai-profiles/${encodeURIComponent(name)}`);
  },
  getImageAssets() {
    return fetchJSON("/api/admin/image-assets");
  },
  getTools() {
    return fetchJSON("/api/admin/tools", { cache: "no-store" });
  },
  getEvents(limit = 80) {
    return fetchJSON(`/api/admin/events?limit=${encodeURIComponent(limit)}`, {
      cache: "no-store"
    });
  },
  getAffection(userID) {
    return fetchJSON(`/api/admin/affection/${encodeURIComponent(userID)}`, {
      cache: "no-store"
    });
  },
  getCharacters() {
    return fetchJSON("/api/admin/characters");
  },
  getCharacterConfig(name) {
    return fetchJSON(`/api/admin/characters/${encodeURIComponent(name)}`);
  },
  updateCharacterConfig(name, config) {
    return fetchJSON(`/api/admin/characters/${encodeURIComponent(name)}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ config })
    });
  },
  createCharacterConfig(name, config) {
    return fetchJSON("/api/admin/characters", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, config })
    });
  },
  getLogFiles() {
    return fetchJSON("/api/admin/logs/files");
  },
  getLogContent(file, lines) {
    const ts = Date.now();
    return fetchJSON(
      `/api/admin/logs/content?file=${encodeURIComponent(file)}&lines=${lines}&_ts=${ts}`,
      {
        cache: "no-store"
      }
    );
  }
};

export function openEventSocket(filters) {
  const url = new URL(`/api/admin/events/stream${queryString(filters)}`, window.location.href);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  return new WebSocket(url);
}

function queryString(filters) {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(filters)) {
    if (value !== "" && value !== undefined && value !== null) params.set(key, String(value));
  }
  const query = params.toString();
  return query ? `?${query}` : "";
}
