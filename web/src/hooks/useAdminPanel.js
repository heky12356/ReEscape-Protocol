import { useCallback, useEffect, useMemo, useState } from "react";
import { adminApi } from "../api/adminApi";

const defaultConfig = {
  aiBaseUrl: "",
  aiModel: "",
  aiProfile: "default",
  aiProfiles: [],
  aiConfigFile: "",
  aiTemperature: 1,
  aiMaxTokens: 2000,
  aiTimeout: 30,
  aiRetryCount: 3,
  aiRateLimit: 20,
  aiTopP: 0.9,
  aiPromptRaw: "",
  enableTimeContext: true,
  timeContextTimezone: "Asia/Shanghai",
  timeContextFormat: "2006-01-02 15:04:05",
  character: "",
  aiKey: "",
  aiKeyMasked: "",
  aiKeySet: false,
  characterOptions: [],
  effectivePrompt: "",
  environmentConfig: ".env"
};

const defaultCharacterConfig = {
  name: "",
  description: "",
  personality: {},
  responses: {},
  behavior: {},
  quotes: []
};

const defaultProbe = {
  status: "unknown",
  checks: {},
  time: "",
  uptimeSec: 0
};

export function useAdminPanel() {
  const [config, setConfig] = useState(defaultConfig);
  const [saving, setSaving] = useState(false);
  const [loadingConfig, setLoadingConfig] = useState(false);
  const [loadingAIProfile, setLoadingAIProfile] = useState(false);
  const [status, setStatus] = useState("");
  const [error, setError] = useState("");

  const [health, setHealth] = useState(defaultProbe);
  const [ready, setReady] = useState(defaultProbe);

  const [logFiles, setLogFiles] = useState([]);
  const [selectedLogFile, setSelectedLogFile] = useState("");
  const [logLines, setLogLines] = useState(200);
  const [logContent, setLogContent] = useState("");
  const [loadingLogs, setLoadingLogs] = useState(false);

  const [characterFile, setCharacterFile] = useState("");
  const [characterConfig, setCharacterConfig] = useState(defaultCharacterConfig);
  const [loadingCharacter, setLoadingCharacter] = useState(false);
  const [savingCharacter, setSavingCharacter] = useState(false);
  const [creatingCharacter, setCreatingCharacter] = useState(false);

  const resetMsg = useCallback(() => {
    setError("");
    setStatus("");
  }, []);

  const loadConfig = useCallback(async () => {
    setLoadingConfig(true);
    setError("");
    try {
      const data = await adminApi.getConfig();
      setConfig((prev) => ({ ...prev, ...data, aiKey: "" }));
    } catch (err) {
      setError(getErrMsg(err));
    } finally {
      setLoadingConfig(false);
    }
  }, []);

  const loadSystemStatus = useCallback(async () => {
    const [healthResult, readyResult] = await Promise.allSettled([
      adminApi.getHealth(),
      adminApi.getReady()
    ]);

    if (healthResult.status === "fulfilled") {
      setHealth((prev) => ({ ...prev, ...healthResult.value }));
    } else {
      setHealth((prev) => ({
        ...prev,
        status: "down",
        checks: { process: getErrMsg(healthResult.reason) }
      }));
    }

    if (readyResult.status === "fulfilled") {
      setReady((prev) => ({ ...prev, ...readyResult.value }));
    } else {
      setReady((prev) => ({
        ...prev,
        status: "down",
        checks: { process: getErrMsg(readyResult.reason) }
      }));
    }
  }, []);

  const saveConfig = useCallback(
    async (override = null) => {
      const nextConfig = override ? { ...config, ...override } : config;
      const payload = buildConfigPayload(nextConfig);

      setSaving(true);
      resetMsg();
      try {
        const data = await adminApi.updateConfig(payload);
        setConfig((prev) => ({ ...prev, ...nextConfig, ...data, aiKey: "" }));
        setStatus("配置已保存并热重载");
        void loadSystemStatus();
      } catch (err) {
        setError(getErrMsg(err));
      } finally {
        setSaving(false);
      }
    },
    [config, loadSystemStatus, resetMsg]
  );

  const loadAIProfile = useCallback(async (name) => {
    const target = String(name || "").trim();
    if (!target) {
      return;
    }

    setLoadingAIProfile(true);
    setError("");
    try {
      const data = await adminApi.getAIProfile(target);
      setConfig((prev) => ({
        ...prev,
        aiProfile: data.name || target,
        aiBaseUrl: data.aiBaseUrl || "",
        aiModel: data.aiModel || "",
        aiTemperature: data.aiTemperature ?? prev.aiTemperature,
        aiMaxTokens: data.aiMaxTokens ?? prev.aiMaxTokens,
        aiTimeout: data.aiTimeout ?? prev.aiTimeout,
        aiRetryCount: data.aiRetryCount ?? prev.aiRetryCount,
        aiRateLimit: data.aiRateLimit ?? prev.aiRateLimit,
        aiTopP: data.aiTopP ?? prev.aiTopP,
        aiKeyMasked: data.aiKeyMasked || "",
        aiKeySet: Boolean(data.aiKeySet),
        aiKey: ""
      }));
    } catch (err) {
      setError(getErrMsg(err));
    } finally {
      setLoadingAIProfile(false);
    }
  }, []);

  const selectAIProfile = useCallback(
    async (name) => {
      const target = String(name || "").trim();
      if (!target) {
        return;
      }
      setConfig((prev) => ({ ...prev, aiProfile: target, aiKey: "" }));
      await loadAIProfile(target);
    },
    [loadAIProfile]
  );

  const loadLogContent = useCallback(
    async (file, lines = logLines) => {
      if (!file) {
        setLogContent("");
        return;
      }
      setLoadingLogs(true);
      setError("");
      try {
        const data = await adminApi.getLogContent(file, lines);
        setLogContent(data.content || "");
        setSelectedLogFile(data.file || file);
      } catch (err) {
        setError(getErrMsg(err));
      } finally {
        setLoadingLogs(false);
      }
    },
    [logLines]
  );

  const loadLogFiles = useCallback(async () => {
    setLoadingLogs(true);
    setError("");
    try {
      const files = await adminApi.getLogFiles();
      setLogFiles(files || []);
      if (files?.length) {
        const target = selectedLogFile || files[0].name;
        setSelectedLogFile(target);
        await loadLogContent(target, logLines);
      } else {
        setSelectedLogFile("");
        setLogContent("");
      }
    } catch (err) {
      setError(getErrMsg(err));
    } finally {
      setLoadingLogs(false);
    }
  }, [loadLogContent, logLines, selectedLogFile]);

  const loadCharacterConfig = useCallback(async (name) => {
    const target = String(name || "").trim();
    if (!target) {
      setCharacterFile("");
      setCharacterConfig(defaultCharacterConfig);
      return;
    }

    setLoadingCharacter(true);
    setError("");
    try {
      const data = await adminApi.getCharacterConfig(target);
      setCharacterFile(data.file || target);
      setCharacterConfig(normalizeCharacterConfig(data.config));
    } catch (err) {
      setError(getErrMsg(err));
    } finally {
      setLoadingCharacter(false);
    }
  }, []);

  const refreshCharacterOptions = useCallback(async () => {
    try {
      const files = await adminApi.getCharacters();
      setConfig((prev) => ({ ...prev, characterOptions: files || [] }));
    } catch (err) {
      setError(getErrMsg(err));
    }
  }, []);

  const saveCharacterConfig = useCallback(
    async (name = config.character, nextConfig = characterConfig) => {
      const target = String(name || "").trim();
      if (!target) {
        setError("character file name is required");
        return;
      }

      setSavingCharacter(true);
      resetMsg();
      try {
        const data = await adminApi.updateCharacterConfig(target, nextConfig);
        setCharacterFile(data.file || target);
        setCharacterConfig(normalizeCharacterConfig(data.config));
        setStatus("人格文件已保存");
        await refreshCharacterOptions();
      } catch (err) {
        setError(getErrMsg(err));
      } finally {
        setSavingCharacter(false);
      }
    },
    [characterConfig, config.character, refreshCharacterOptions, resetMsg]
  );

  const createCharacterConfig = useCallback(
    async (name, nextConfig = characterConfig) => {
      const target = String(name || "").trim();
      if (!target) {
        setError("new character file name is required");
        return;
      }

      setCreatingCharacter(true);
      resetMsg();
      try {
        const data = await adminApi.createCharacterConfig(target, nextConfig);
        const createdFile = data.file || target;
        setCharacterFile(createdFile);
        setCharacterConfig(normalizeCharacterConfig(data.config));
        setConfig((prev) => ({ ...prev, character: createdFile }));
        setStatus("新人格文件已创建");
        await refreshCharacterOptions();
      } catch (err) {
        setError(getErrMsg(err));
      } finally {
        setCreatingCharacter(false);
      }
    },
    [characterConfig, refreshCharacterOptions, resetMsg]
  );

  useEffect(() => {
    void loadConfig();
    void loadLogFiles();
    void loadSystemStatus();
  }, [loadConfig, loadLogFiles, loadSystemStatus]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      void loadSystemStatus();
    }, 30000);

    return () => window.clearInterval(timer);
  }, [loadSystemStatus]);

  useEffect(() => {
    if (!config.character) {
      return;
    }
    void loadCharacterConfig(config.character);
  }, [config.character, loadCharacterConfig]);

  const digest = useMemo(
    () => ({
      profileCount: config.aiProfiles.length,
      characterCount: config.characterOptions.length,
      logCount: logFiles.length,
      healthState: health.status,
      readyState: ready.status
    }),
    [config.aiProfiles.length, config.characterOptions.length, health.status, logFiles.length, ready.status]
  );

  return {
    config,
    setConfig,
    saving,
    loadingConfig,
    loadingAIProfile,
    status,
    setStatus,
    error,
    setError,
    saveConfig,
    selectAIProfile,
    loadAIProfile,
    loadConfig,
    health,
    ready,
    loadSystemStatus,
    digest,
    logFiles,
    selectedLogFile,
    setSelectedLogFile,
    logLines,
    setLogLines,
    logContent,
    loadingLogs,
    loadLogFiles,
    loadLogContent,
    characterFile,
    characterConfig,
    setCharacterConfig,
    loadingCharacter,
    savingCharacter,
    creatingCharacter,
    loadCharacterConfig,
    saveCharacterConfig,
    createCharacterConfig
  };
}

function normalizeCharacterConfig(raw) {
  const config = raw || {};
  return {
    name: config.name || "",
    description: config.description || "",
    personality: config.personality && typeof config.personality === "object" ? config.personality : {},
    responses: config.responses && typeof config.responses === "object" ? config.responses : {},
    behavior: config.behavior && typeof config.behavior === "object" ? config.behavior : {},
    quotes: Array.isArray(config.quotes) ? config.quotes : []
  };
}

function buildConfigPayload(config) {
  return {
    aiBaseUrl: String(config.aiBaseUrl || "").trim(),
    aiModel: String(config.aiModel || "").trim(),
    aiProfile: String(config.aiProfile || "").trim(),
    aiTemperature: Number(config.aiTemperature),
    aiMaxTokens: Number(config.aiMaxTokens),
    aiTimeout: Number(config.aiTimeout),
    aiRetryCount: Number(config.aiRetryCount),
    aiRateLimit: Number(config.aiRateLimit),
    aiTopP: Number(config.aiTopP),
    aiPromptRaw: config.aiPromptRaw,
    enableTimeContext: Boolean(config.enableTimeContext),
    timeContextTimezone: String(config.timeContextTimezone || "").trim(),
    timeContextFormat: String(config.timeContextFormat || "").trim(),
    character: config.character,
    aiKey: config.aiKey
  };
}

function getErrMsg(err) {
  if (err instanceof Error) {
    return err.message;
  }
  return String(err);
}
