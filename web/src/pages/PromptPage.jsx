import { useEffect, useState } from "react";
import { Panel } from "../components/common/Panel";
import { InputField, SelectField, TextAreaField } from "../components/common/FormField";

const BOOLEAN_OPTIONS = ["true", "false"];
const IDENTITY_MODE_OPTIONS = ["product_identity", "legacy"];

export function PromptPage({ panel }) {
  const cfg = panel.config;
  const [personalityText, setPersonalityText] = useState("{}");
  const [responsesText, setResponsesText] = useState("{}");
  const [behaviorText, setBehaviorText] = useState("{}");
  const [quotesText, setQuotesText] = useState("");
  const [vocabularyText, setVocabularyText] = useState("");
  const [avoidText, setAvoidText] = useState("");
  const [doNotRevealText, setDoNotRevealText] = useState("");
  const [safetyBoundariesText, setSafetyBoundariesText] = useState("");
  const [relationshipRulesText, setRelationshipRulesText] = useState("");
  const [examplesText, setExamplesText] = useState("[]");
  const [newFileName, setNewFileName] = useState("");

  useEffect(() => {
    setPersonalityText(toPrettyJSON(panel.characterConfig.personality));
    setResponsesText(toPrettyJSON(panel.characterConfig.responses));
    setBehaviorText(toPrettyJSON(panel.characterConfig.behavior));
    setQuotesText((panel.characterConfig.quotes || []).join("\n"));
    setVocabularyText((panel.characterConfig.voice?.vocabulary || []).join("\n"));
    setAvoidText((panel.characterConfig.voice?.avoid || []).join("\n"));
    setDoNotRevealText((panel.characterConfig.boundaries?.doNotReveal || []).join("\n"));
    setSafetyBoundariesText((panel.characterConfig.boundaries?.safetyBoundaries || []).join("\n"));
    setRelationshipRulesText((panel.characterConfig.boundaries?.relationshipRules || []).join("\n"));
    setExamplesText(toPrettyJSON(panel.characterConfig.examples || []));
  }, [panel.characterConfig]);

  const saveCharacter = async () => {
    try {
      panel.setError("");
      const next = buildCharacterConfig(panel.characterConfig, {
        personalityText,
        responsesText,
        behaviorText,
        quotesText,
        vocabularyText,
        avoidText,
        doNotRevealText,
        safetyBoundariesText,
        relationshipRulesText,
        examplesText
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
        quotesText,
        vocabularyText,
        avoidText,
        doNotRevealText,
        safetyBoundariesText,
        relationshipRulesText,
        examplesText
      });
      panel.setCharacterConfig(next);
      await panel.createCharacterConfig(newFileName, next);
      setNewFileName("");
    } catch (err) {
      panel.setError(err instanceof Error ? err.message : String(err));
    }
  };

  const preview = cfg.promptPreview || {};

  return (
    <div className="stack">
      <Panel
        eyebrow="Voice source"
        title="生效人格与系统补充 Prompt"
        subtitle="角色身份、系统补充和 ReAct 工具规则分层预览，最终回复仍保持角色口吻。"
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
        <div className="split-layout">
          <div className="stack compact">
            <SelectField
              label="人格文件 (CHARACTER)"
              value={cfg.character}
              options={cfg.characterOptions}
              onChange={(v) => updateConfigField(panel, "character", v)}
            />
            <SelectField
              label="Identity mode"
              value={cfg.characterIdentityMode}
              options={IDENTITY_MODE_OPTIONS}
              hint="默认使用产品内身份，不把角色描述为临时扮演。"
              onChange={(v) => updateConfigField(panel, "characterIdentityMode", v)}
            />
            <SelectField
              label="Allow identity explanation"
              value={String(cfg.allowCharacterIdentityExplanation)}
              options={BOOLEAN_OPTIONS}
              hint="关闭时不主动解释模型、提示词或工具机制。"
              onChange={(v) => updateConfigField(panel, "allowCharacterIdentityExplanation", v === "true")}
            />
            <TextAreaField
              label="AI_PROMPT"
              value={cfg.aiPromptRaw}
              rows={7}
              hint="补充系统说明，适合放临时策略，不建议放角色身份。"
              onChange={(v) => updateConfigField(panel, "aiPromptRaw", v)}
            />
          </div>

          <div className="insight-card alternate">
            <div className="insight-kicker">Prompt layers</div>
            <div className="insight-title">角色身份只约束最终可见回复，工具调用保持客观结构化。</div>
            <p className="insight-copy">
              Admin 现在会返回 base、user、character、agent、effective 五段预览，方便定位每层来源。
            </p>
            <div className="insight-meta mono">{panel.characterFile || cfg.character || "-"}</div>
          </div>
        </div>
      </Panel>

      <Panel
        eyebrow="Character card"
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
            label="新文件名"
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

        <div className="editor-grid">
          <InputField
            label="Role name"
            value={panel.characterConfig.identity?.roleName}
            onChange={(v) => updateNestedCharacterField(panel, "identity", "roleName", v)}
          />
          <InputField
            label="Self reference"
            value={panel.characterConfig.identity?.selfReference}
            onChange={(v) => updateNestedCharacterField(panel, "identity", "selfReference", v)}
          />
          <TextAreaField
            label="Product identity"
            value={panel.characterConfig.identity?.productIdentity}
            rows={5}
            hint="例如：你在 ReEscape Protocol 中以江梦的身份与用户对话。"
            onChange={(v) => updateNestedCharacterField(panel, "identity", "productIdentity", v)}
          />
          <TextAreaField
            label="Identity policy"
            value={panel.characterConfig.identity?.identityPolicy}
            rows={5}
            hint="用于身份边界，不要写“假扮”。"
            onChange={(v) => updateNestedCharacterField(panel, "identity", "identityPolicy", v)}
          />
        </div>

        <div className="editor-grid">
          <InputField
            label="Tone"
            value={panel.characterConfig.voice?.tone}
            onChange={(v) => updateNestedCharacterField(panel, "voice", "tone", v)}
          />
          <InputField
            label="Style"
            value={panel.characterConfig.voice?.style}
            onChange={(v) => updateNestedCharacterField(panel, "voice", "style", v)}
          />
          <InputField
            label="Pacing"
            value={panel.characterConfig.voice?.pacing}
            onChange={(v) => updateNestedCharacterField(panel, "voice", "pacing", v)}
          />
          <TextAreaField
            label="Vocabulary (one per line)"
            value={vocabularyText}
            rows={6}
            onChange={setVocabularyText}
          />
          <TextAreaField
            label="Avoid words (one per line)"
            value={avoidText}
            rows={6}
            onChange={setAvoidText}
          />
          <TextAreaField
            label="Quotes (one line per quote)"
            value={quotesText}
            rows={6}
            onChange={setQuotesText}
          />
        </div>

        <div className="editor-grid">
          <InputField
            label="Default stage"
            value={panel.characterConfig.relationship?.defaultStage}
            onChange={(v) => updateNestedCharacterField(panel, "relationship", "defaultStage", v)}
          />
          <InputField
            label="Addressing"
            value={panel.characterConfig.relationship?.addressing}
            onChange={(v) => updateNestedCharacterField(panel, "relationship", "addressing", v)}
          />
          <TextAreaField
            label="Intimacy rule"
            value={panel.characterConfig.relationship?.intimacyRule}
            rows={5}
            onChange={(v) => updateNestedCharacterField(panel, "relationship", "intimacyRule", v)}
          />
          <TextAreaField
            label="Do not reveal (one per line)"
            value={doNotRevealText}
            rows={5}
            onChange={setDoNotRevealText}
          />
          <TextAreaField
            label="Safety boundaries (one per line)"
            value={safetyBoundariesText}
            rows={5}
            onChange={setSafetyBoundariesText}
          />
          <TextAreaField
            label="Relationship rules (one per line)"
            value={relationshipRulesText}
            rows={5}
            onChange={setRelationshipRulesText}
          />
        </div>

        <div className="form-row">
          <TextAreaField
            label="Examples (JSON array)"
            value={examplesText}
            rows={9}
            hint='格式：[{"situation":"安慰","user":"我好累","reply":"先停一下嘛$你已经撑很久了"}]'
            onChange={setExamplesText}
          />
        </div>

        <div className="editor-grid">
          <TextAreaField
            label="Legacy personality (JSON object)"
            value={personalityText}
            rows={10}
            onChange={setPersonalityText}
          />
          <TextAreaField
            label="Legacy responses (JSON object)"
            value={responsesText}
            rows={10}
            onChange={setResponsesText}
          />
          <TextAreaField
            label="Legacy behavior (JSON object)"
            value={behaviorText}
            rows={10}
            onChange={setBehaviorText}
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

      <Panel eyebrow="Prompt preview" title="分层 Prompt 预览" subtitle="后端实际返回的 prompt section。">
        <div className="editor-grid">
          <PromptPreview title="Base prompt" content={preview.basePrompt} />
          <PromptPreview title="User prompt" content={preview.userPrompt} />
          <PromptPreview title="Character prompt" content={preview.characterPrompt} />
          <PromptPreview title="Agent prompt" content={preview.agentPrompt} />
        </div>
        <div className="form-row">
          <PromptPreview title="Effective prompt" content={preview.effectivePrompt || cfg.effectivePrompt} />
        </div>
      </Panel>

      <Panel
        eyebrow="Image shelf"
        title="图片素材索引"
        subtitle="这里先做只读可视化，素材图片和 index.json 仍然在仓库目录里维护。"
      >
        <div className="artifact-grid">
          <div className="artifact-card emphasis">
            <div className="artifact-title">Asset index</div>
            <p className="artifact-note">
              {cfg.imageAssetIndexFile || "-"}
              <br />
              目录：{cfg.imageAssetDir || "-"}
            </p>
          </div>

          <div className="artifact-card">
            <div className="artifact-title">Registered assets</div>
            <ul className="artifact-list mono">
              {panel.imageAssets.map((asset) => (
                <li key={asset.id}>
                  <span>{asset.id}</span>
                  <span>{asset.enabled ? "enabled" : "disabled"}</span>
                </li>
              ))}
              {panel.imageAssets.length === 0 ? <li>暂无素材索引</li> : null}
            </ul>
          </div>
        </div>
      </Panel>
    </div>
  );
}

function PromptPreview({ title, content }) {
  return (
    <div className="stack compact">
      <div className="field-label">{title}</div>
      <pre className="prompt-preview">{content || "暂无内容"}</pre>
    </div>
  );
}

function updateConfigField(panel, key, value) {
  panel.setConfig((prev) => ({ ...prev, [key]: value }));
}

function updateCharacterField(panel, key, value) {
  panel.setCharacterConfig((prev) => ({ ...prev, [key]: value }));
}

function updateNestedCharacterField(panel, section, key, value) {
  panel.setCharacterConfig((prev) => ({
    ...prev,
    [section]: {
      ...(prev[section] || {}),
      [key]: value
    }
  }));
}

function toPrettyJSON(value) {
  try {
    return JSON.stringify(value || (Array.isArray(value) ? [] : {}), null, 2);
  } catch {
    return Array.isArray(value) ? "[]" : "{}";
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

function parseExamples(text) {
  let parsed;
  try {
    parsed = JSON.parse(text || "[]");
  } catch (err) {
    throw new Error(`Examples 不是合法 JSON: ${err instanceof Error ? err.message : String(err)}`);
  }
  if (!Array.isArray(parsed)) {
    throw new Error("Examples 必须是 JSON 数组");
  }
  return parsed.map((item) => ({
    situation: String(item?.situation || "").trim(),
    user: String(item?.user || "").trim(),
    reply: String(item?.reply || "").trim()
  })).filter((item) => item.situation || item.user || item.reply);
}

function linesToArray(text) {
  return String(text || "")
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean);
}

function buildCharacterConfig(base, editor) {
  const personalityRaw = parseJSONObject(editor.personalityText, "Personality");
  const personality = Object.fromEntries(
    Object.entries(personalityRaw).map(([key, value]) => [String(key), String(value)])
  );
  const responses = parseJSONObject(editor.responsesText, "Responses");
  const behavior = parseJSONObject(editor.behaviorText, "Behavior");

  return {
    ...base,
    identity: {
      ...(base.identity || {}),
      roleName: String(base.identity?.roleName || base.name || "").trim(),
      productIdentity: String(base.identity?.productIdentity || "").trim(),
      selfReference: String(base.identity?.selfReference || "").trim(),
      identityPolicy: String(base.identity?.identityPolicy || "").trim()
    },
    voice: {
      ...(base.voice || {}),
      tone: String(base.voice?.tone || "").trim(),
      style: String(base.voice?.style || "").trim(),
      pacing: String(base.voice?.pacing || "").trim(),
      vocabulary: linesToArray(editor.vocabularyText),
      avoid: linesToArray(editor.avoidText)
    },
    boundaries: {
      ...(base.boundaries || {}),
      doNotReveal: linesToArray(editor.doNotRevealText),
      safetyBoundaries: linesToArray(editor.safetyBoundariesText),
      relationshipRules: linesToArray(editor.relationshipRulesText)
    },
    examples: parseExamples(editor.examplesText),
    personality,
    responses,
    behavior,
    quotes: linesToArray(editor.quotesText)
  };
}
