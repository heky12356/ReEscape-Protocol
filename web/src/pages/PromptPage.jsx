import { useEffect, useState } from "react";
import { Panel } from "../components/common/Panel";
import { InputField, SelectField, TextAreaField } from "../components/common/FormField";

export function PromptPage({ panel }) {
  const cfg = panel.config;
  const [personalityText, setPersonalityText] = useState("{}");
  const [responsesText, setResponsesText] = useState("{}");
  const [behaviorText, setBehaviorText] = useState("{}");
  const [quotesText, setQuotesText] = useState("");
  const [newFileName, setNewFileName] = useState("");

  useEffect(() => {
    setPersonalityText(toPrettyJSON(panel.characterConfig.personality));
    setResponsesText(toPrettyJSON(panel.characterConfig.responses));
    setBehaviorText(toPrettyJSON(panel.characterConfig.behavior));
    setQuotesText((panel.characterConfig.quotes || []).join("\n"));
  }, [panel.characterConfig]);

  const saveCharacter = async () => {
    try {
      panel.setError("");
      const next = buildCharacterConfig(panel.characterConfig, {
        personalityText,
        responsesText,
        behaviorText,
        quotesText
      });
      panel.setCharacterConfig(next);
      await panel.saveCharacterConfig(cfg.character, next);
    } catch (err) {
      panel.setError(err instanceof Error ? err.message : String(err));
    }
  };

  const createCharacter = async () => {
    try {
      panel.setError("");
      const next = buildCharacterConfig(panel.characterConfig, {
        personalityText,
        responsesText,
        behaviorText,
        quotesText
      });
      panel.setCharacterConfig(next);
      await panel.createCharacterConfig(newFileName, next);
      setNewFileName("");
    } catch (err) {
      panel.setError(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div className="stack">
      <Panel
        title="人格与 Prompt"
        subtitle="切换生效人格，编辑 AI_PROMPT，并应用到运行配置"
        actions={
          <button
            type="button"
            className="btn-primary"
            onClick={panel.saveConfig}
            disabled={panel.saving || panel.loadingConfig}
          >
            {panel.saving ? "保存中..." : "保存并应用"}
          </button>
        }
      >
        <div className="form-row">
          <SelectField
            label="人格文件 (CHARACTER)"
            value={cfg.character}
            options={cfg.characterOptions}
            onChange={(v) => updateConfigField(panel, "character", v)}
          />
        </div>
        <TextAreaField
          label="AI_PROMPT (补充提示词)"
          value={cfg.aiPromptRaw}
          rows={8}
          onChange={(v) => updateConfigField(panel, "aiPromptRaw", v)}
        />
      </Panel>

      <Panel
        title="人格文件编辑器"
        subtitle={`当前文件: ${panel.characterFile || cfg.character || "-"}`}
        actions={
          <div className="action-row">
            <button
              type="button"
              className="btn-ghost"
              onClick={saveCharacter}
              disabled={panel.loadingCharacter || panel.savingCharacter || !cfg.character}
            >
              {panel.savingCharacter ? "保存中..." : "保存人格文件"}
            </button>
          </div>
        }
      >
        <div className="form-grid">
          <InputField
            label="显示名称 (name)"
            value={panel.characterConfig.name}
            onChange={(v) => updateCharacterField(panel, "name", v)}
          />
          <InputField
            label="新文件名 (用于新建)"
            value={newFileName}
            placeholder="example: assistant_v2"
            onChange={setNewFileName}
          />
        </div>

        <div className="form-row">
          <TextAreaField
            label="描述 (description)"
            value={panel.characterConfig.description}
            rows={4}
            onChange={(v) => updateCharacterField(panel, "description", v)}
          />
        </div>

        <div className="form-row">
          <TextAreaField
            label="Personality (JSON object)"
            value={personalityText}
            rows={8}
            onChange={setPersonalityText}
          />
        </div>

        <div className="form-row">
          <TextAreaField
            label="Responses (JSON object)"
            value={responsesText}
            rows={10}
            onChange={setResponsesText}
          />
        </div>

        <div className="form-row">
          <TextAreaField
            label="Behavior (JSON object)"
            value={behaviorText}
            rows={10}
            onChange={setBehaviorText}
          />
        </div>

        <div className="form-row">
          <TextAreaField
            label="Quotes (one line per quote)"
            value={quotesText}
            rows={6}
            onChange={setQuotesText}
          />
        </div>

        <div className="form-row prompt-editor-actions">
          <button
            type="button"
            className="btn-primary"
            onClick={createCharacter}
            disabled={panel.creatingCharacter || !newFileName.trim()}
          >
            {panel.creatingCharacter ? "创建中..." : "新建人格文件"}
          </button>
        </div>
      </Panel>

      <Panel title="最终生效 Prompt" subtitle="系统基础 prompt + AI_PROMPT + 人格 prompt">
        <pre className="prompt-preview">{cfg.effectivePrompt || "暂无内容"}</pre>
      </Panel>
    </div>
  );
}

function updateConfigField(panel, key, value) {
  panel.setConfig((prev) => ({ ...prev, [key]: value }));
}

function updateCharacterField(panel, key, value) {
  panel.setCharacterConfig((prev) => ({ ...prev, [key]: value }));
}

function toPrettyJSON(value) {
  try {
    return JSON.stringify(value || {}, null, 2);
  } catch {
    return "{}";
  }
}

function parseJSONObject(text, label) {
  let parsed;
  try {
    parsed = JSON.parse(text || "{}");
  } catch (err) {
    throw new Error(`${label} 不是合法 JSON: ${err instanceof Error ? err.message : String(err)}`);
  }
  if (!parsed || Array.isArray(parsed) || typeof parsed !== "object") {
    throw new Error(`${label} 必须是 JSON 对象`);
  }
  return parsed;
}

function buildCharacterConfig(base, editor) {
  const personalityRaw = parseJSONObject(editor.personalityText, "Personality");
  const personality = Object.fromEntries(
    Object.entries(personalityRaw).map(([key, value]) => [String(key), String(value)])
  );
  const responses = parseJSONObject(editor.responsesText, "Responses");
  const behavior = parseJSONObject(editor.behaviorText, "Behavior");
  const quotes = String(editor.quotesText || "")
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean);

  return {
    ...base,
    personality,
    responses,
    behavior,
    quotes
  };
}
