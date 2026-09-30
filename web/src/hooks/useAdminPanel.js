import { useCallback, useEffect, useMemo, useState } from "react";
import { adminApi } from "../api/adminApi";

const defaultConfig = {
  targetId: 0,
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
  allowCharacterIdentityExplanation: false,
  enableTimeContext: true,
  timeContextTimezone: "Asia/Shanghai",
  timeContextFormat: "2006-01-02 15:04:05",
  enableVisionInput: false,
  visionImageDetail: "auto",
  enableImageOCRFallback: false,
  enableImageAssetReply: true,
  imageAssetDir: "./assets/images",
  imageAssetIndexFile: "./assets/images/index.json",
  character: "",
  aiKey: "",
  aiKeyMasked: "",
  aiKeySet: false,
  characterOptions: [],
  effectivePrompt: "",
  promptPreview: {
    basePrompt: "",
    userPrompt: "",
    characterPrompt: "",
    agentPrompt: "",
    effectivePrompt: ""
  },
  enableReactAgent: false,
  reactMaxSteps: 4,
  reactToolTimeoutMs: 3000,
  reactAllowWriteTools: false,
  reactTraceMode: "basic",
  reactTotalTimeoutMs: 30000,
  enableWebTools: false,
  webSearchProvider: "searxng",
  webSearchEndpoint: "",
  webSearchApiKey: "",
  webSearchApiKeyMasked: "",
  webSearchApiKeySet: false,
  webSearchMaxResults: 5,
  webToolTimeoutMs: 8000,
  webFetchMaxBytes: 1048576,
  webFetchMaxChars: 6000,
  webFetchUserAgent: "ReEscapeProtocolBot/1.0",
  environmentConfig: ".env"
};

const defaultCharacterConfig = {
  name: "",
  description: "",
  identity: {
    roleName: "",
    productIdentity: "",
    selfReference: "",
    identityPolicy: ""
  },
  background: {
    age: "",
    occupation: "",
    traits: [],
    interests: [],
    habits: [],
    skills: []
  },
  voice: {
    tone: "",
    style: "",
    pacing: "",
    vocabulary: [],
    avoid: []
  },
  boundaries: {
    doNotReveal: [],
    safetyBoundaries: [],
    relationshipRules: []
  },
  relationship: {
    defaultStage: "",
    addressing: "",
    intimacyRule: ""
  },
  examples: []
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
  const [imageAssets, setImageAssets] = useState([]);
  const [toolDefinitions, setToolDefinitions] = useState([]);
  const [events, setEvents] = useState([]);
  const [affectionState, setAffectionState] = useState(null);
  const [selectedLogFile, setSelectedLogFile] = useState("");
  const [logLines, setLogLines] = useState(200);
  const [logContent, setLogContent] = useState("");
  const [loadingLogs, setLoadingLogs] = useState(false);
  const [loadingDiagnostics, setLoadingDiagnostics] = useState(false);

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
      setConfig((prev) => ({
        ...prev,
        ...data,
        promptPreview: { ...defaultConfig.promptPreview, ...(data.promptPreview || {}) },
        aiKey: "",
        webSearchApiKey: ""
      }));
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
        setConfig((prev) => ({ ...prev, ...nextConfig, ...data, aiKey: "", webSearchApiKey: "" }));
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

  const loadImageAssets = useCallback(async () => {
    setError("");
    try {
      const data = await adminApi.getImageAssets();
      setImageAssets(data.assets || []);
    } catch (err) {
      setError(getErrMsg(err));
    }
  }, []);

  const loadDiagnostics = useCallback(
    async (targetId = config.targetId) => {
      const userID = Number(targetId || 0);
      setLoadingDiagnostics(true);
      try {
        const [toolsResult, eventsResult, affectionResult] = await Promise.allSettled([
          adminApi.getTools(),
          adminApi.getEvents(80),
          userID > 0 ? adminApi.getAffection(userID) : Promise.resolve({ state: null })
        ]);

        if (toolsResult.status === "fulfilled") {
          setToolDefinitions(toolsResult.value.tools || []);
        }
        if (eventsResult.status === "fulfilled") {
          setEvents(eventsResult.value.events || []);
        }
        if (affectionResult.status === "fulfilled") {
          setAffectionState(affectionResult.value.state || null);
        }

        const failed = [toolsResult, eventsResult, affectionResult].find(
          (result) => result.status === "rejected"
        );
        if (failed) {
          setError(getErrMsg(failed.reason));
        }
      } finally {
        setLoadingDiagnostics(false);
      }
    },
    [config.targetId]
  );

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
    void loadImageAssets();
    void loadDiagnostics();
    void loadSystemStatus();
  }, [loadConfig, loadDiagnostics, loadImageAssets, loadLogFiles, loadSystemStatus]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      void loadSystemStatus();
      void loadDiagnostics();
    }, 30000);

    return () => window.clearInterval(timer);
  }, [loadDiagnostics, loadSystemStatus]);

  useEffect(() => {
    if (!config.targetId) {
      return;
    }
    void loadDiagnostics(config.targetId);
  }, [config.targetId, loadDiagnostics]);

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
      imageAssetCount: imageAssets.length,
      toolCount: toolDefinitions.length,
      eventCount: events.length,
      affectionScore: affectionState?.score ?? null,
      affectionStage: affectionState?.stage || "",
      logCount: logFiles.length,
      healthState: health.status,
      readyState: ready.status
    }),
    [
      affectionState?.score,
      affectionState?.stage,
      config.aiProfiles.length,
      config.characterOptions.length,
      events.length,
      health.status,
      imageAssets.length,
      logFiles.length,
      ready.status,
      toolDefinitions.length
    ]
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
    imageAssets,
    loadImageAssets,
    toolDefinitions,
    events,
    affectionState,
    loadingDiagnostics,
    loadDiagnostics,
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
    identity: {
      roleName: config.identity?.roleName || config.name || "",
      productIdentity: config.identity?.productIdentity || "",
      selfReference: config.identity?.selfReference || "",
      identityPolicy: config.identity?.identityPolicy || ""
    },
    voice: {
      tone: config.voice?.tone || "",
      style: config.voice?.style || "",
      pacing: config.voice?.pacing || "",
      vocabulary: Array.isArray(config.voice?.vocabulary) ? config.voice.vocabulary : [],
      avoid: Array.isArray(config.voice?.avoid) ? config.voice.avoid : []
    },
    boundaries: {
      doNotReveal: Array.isArray(config.boundaries?.doNotReveal) ? config.boundaries.doNotReveal : [],
      safetyBoundaries: Array.isArray(config.boundaries?.safetyBoundaries) ? config.boundaries.safetyBoundaries : [],
      relationshipRules: Array.isArray(config.boundaries?.relationshipRules) ? config.boundaries.relationshipRules : []
    },
    relationship: {
      defaultStage: config.relationship?.defaultStage || "",
      addressing: config.relationship?.addressing || "",
      intimacyRule: config.relationship?.intimacyRule || ""
    },
    background: {
      age: config.background?.age || "",
      occupation: config.background?.occupation || "",
      traits: Array.isArray(config.background?.traits) ? config.background.traits : [],
      interests: Array.isArray(config.background?.interests) ? config.background.interests : [],
      habits: Array.isArray(config.background?.habits) ? config.background.habits : [],
      skills: Array.isArray(config.background?.skills) ? config.background.skills : []
    },
    examples: Array.isArray(config.examples) ? config.examples : []
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
    allowCharacterIdentityExplanation: Boolean(config.allowCharacterIdentityExplanation),
    enableTimeContext: Boolean(config.enableTimeContext),
    timeContextTimezone: String(config.timeContextTimezone || "").trim(),
    timeContextFormat: String(config.timeContextFormat || "").trim(),
    enableVisionInput: Boolean(config.enableVisionInput),
    visionImageDetail: String(config.visionImageDetail || "").trim(),
    enableImageOCRFallback: Boolean(config.enableImageOCRFallback),
    enableImageAssetReply: Boolean(config.enableImageAssetReply),
    imageAssetDir: String(config.imageAssetDir || "").trim(),
    imageAssetIndexFile: String(config.imageAssetIndexFile || "").trim(),
    character: config.character,
    aiKey: config.aiKey,
    enableReactAgent: Boolean(config.enableReactAgent),
    reactMaxSteps: Number(config.reactMaxSteps),
    reactToolTimeoutMs: Number(config.reactToolTimeoutMs),
    reactAllowWriteTools: Boolean(config.reactAllowWriteTools),
    reactTraceMode: String(config.reactTraceMode || "basic").trim(),
    reactTotalTimeoutMs: Number(config.reactTotalTimeoutMs),
    enableWebTools: Boolean(config.enableWebTools),
    webSearchProvider: String(config.webSearchProvider || "searxng").trim(),
    webSearchEndpoint: String(config.webSearchEndpoint || "").trim(),
    webSearchApiKey: String(config.webSearchApiKey || "").trim(),
    webSearchMaxResults: Number(config.webSearchMaxResults),
    webToolTimeoutMs: Number(config.webToolTimeoutMs),
    webFetchMaxBytes: Number(config.webFetchMaxBytes),
    webFetchMaxChars: Number(config.webFetchMaxChars),
    webFetchUserAgent: String(config.webFetchUserAgent || "").trim()
  };
}

function getErrMsg(err) {
  if (err instanceof Error) {
    return err.message;
  }
  return String(err);
}
