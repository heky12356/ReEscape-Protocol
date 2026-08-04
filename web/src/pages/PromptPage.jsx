import { useEffect, useState } from "react";
import { Panel } from "../components/common/Panel";
import { InputField, SelectField, TextAreaField } from "../components/common/FormField";

const BOOLEAN_OPTIONS = ["true", "false"];
const IDENTITY_MODE_OPTIONS = ["product_identity", "legacy"];
const BOOLEAN_OPTION_LABELS = {
  true: "允许",
  false: "不允许"
};
const IDENTITY_MODE_LABELS = {
  product_identity: "产品内身份",
  legacy: "旧版身份模式"
};

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
        eyebrow="人格来源"
        title="生效人格与系统补充提示词"
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
              label="人格文件（CHARACTER）"
              value={cfg.character}
              options={cfg.characterOptions}
              onChange={(v) => updateConfigField(panel, "character", v)}
            />
            <SelectField
              label="身份模式"
              value={cfg.characterIdentityMode}
              options={IDENTITY_MODE_OPTIONS}
              optionLabels={IDENTITY_MODE_LABELS}
              hint="默认使用产品内身份，不把角色描述为临时扮演。"
              onChange={(v) => updateConfigField(panel, "characterIdentityMode", v)}
            />
            <SelectField
              label="允许解释身份机制"
              value={String(cfg.allowCharacterIdentityExplanation)}
              options={BOOLEAN_OPTIONS}
              optionLabels={BOOLEAN_OPTION_LABELS}
              hint="关闭时不主动解释模型、提示词或工具机制。"
              onChange={(v) => updateConfigField(panel, "allowCharacterIdentityExplanation", v === "true")}
            />
            <TextAreaField
              label="系统补充提示词（AI_PROMPT）"
              value={cfg.aiPromptRaw}
              rows={7}
              hint="补充系统说明，适合放临时策略，不建议放角色身份。"
              onChange={(v) => updateConfigField(panel, "aiPromptRaw", v)}
            />
          </div>

          <div className="insight-card alternate">
            <div className="insight-kicker">提示词分层</div>
            <div className="insight-title">角色身份只约束最终可见回复，工具调用保持客观结构化。</div>
            <p className="insight-copy">
              管理后台会返回基础、用户、角色、代理和最终生效五段预览，方便定位每层来源。
            </p>
            <div className="insight-meta mono">{panel.characterFile || cfg.character || "-"}</div>
          </div>
        </div>
      </Panel>

      <Panel
        eyebrow="人格卡片"
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
            label="显示名称（name）"
            value={panel.characterConfig.name}
            onChange={(v) => updateCharacterField(panel, "name", v)}
          />
          <InputField
            label="新文件名"
            value={newFileName}
            placeholder="例如：assistant_v2"
            onChange={setNewFileName}
          />
        </div>

        <div className="form-row">
          <TextAreaField
            label="描述（description）"
            value={panel.characterConfig.description}
            rows={4}
            onChange={(v) => updateCharacterField(panel, "description", v)}
          />
        </div>

        <div className="editor-grid">
          <InputField
            label="角色名称（roleName）"
            value={panel.characterConfig.identity?.roleName}
            onChange={(v) => updateNestedCharacterField(panel, "identity", "roleName", v)}
          />
          <InputField
            label="角色自称（selfReference）"
            value={panel.characterConfig.identity?.selfReference}
            onChange={(v) => updateNestedCharacterField(panel, "identity", "selfReference", v)}
          />
          <TextAreaField
            label="产品内身份（productIdentity）"
            value={panel.characterConfig.identity?.productIdentity}
            rows={5}
            hint="例如：你在 ReEscape Protocol 中以江梦的身份与用户对话。"
            onChange={(v) => updateNestedCharacterField(panel, "identity", "productIdentity", v)}
          />
          <TextAreaField
            label="身份表达策略（identityPolicy）"
            value={panel.characterConfig.identity?.identityPolicy}
            rows={5}
            hint="用于身份边界，不要写“假扮”。"
            onChange={(v) => updateNestedCharacterField(panel, "identity", "identityPolicy", v)}
          />
        </div>

        <div className="editor-grid">
          <InputField
            label="语气（tone）"
            value={panel.characterConfig.voice?.tone}
            onChange={(v) => updateNestedCharacterField(panel, "voice", "tone", v)}
          />
          <InputField
            label="表达风格（style）"
            value={panel.characterConfig.voice?.style}
            onChange={(v) => updateNestedCharacterField(panel, "voice", "style", v)}
          />
          <InputField
            label="回复节奏（pacing）"
            value={panel.characterConfig.voice?.pacing}
            onChange={(v) => updateNestedCharacterField(panel, "voice", "pacing", v)}
          />
          <TextAreaField
            label="常用词汇（每行一个）"
            value={vocabularyText}
            rows={6}
            onChange={setVocabularyText}
          />
          <TextAreaField
            label="避免使用的词语（每行一个）"
            value={avoidText}
            rows={6}
            onChange={setAvoidText}
          />
          <TextAreaField
            label="经典语句（每行一句）"
            value={quotesText}
            rows={6}
            onChange={setQuotesText}
          />
        </div>

        <div className="editor-grid">
          <InputField
            label="默认关系阶段（defaultStage）"
            value={panel.characterConfig.relationship?.defaultStage}
            onChange={(v) => updateNestedCharacterField(panel, "relationship", "defaultStage", v)}
          />
          <InputField
            label="对用户的称呼（addressing）"
            value={panel.characterConfig.relationship?.addressing}
            onChange={(v) => updateNestedCharacterField(panel, "relationship", "addressing", v)}
          />
          <TextAreaField
            label="亲密度规则（intimacyRule）"
            value={panel.characterConfig.relationship?.intimacyRule}
            rows={5}
            onChange={(v) => updateNestedCharacterField(panel, "relationship", "intimacyRule", v)}
          />
          <TextAreaField
            label="禁止透露的内容（每行一项）"
            value={doNotRevealText}
            rows={5}
            onChange={setDoNotRevealText}
          />
          <TextAreaField
            label="安全边界（每行一项）"
            value={safetyBoundariesText}
            rows={5}
            onChange={setSafetyBoundariesText}
          />
          <TextAreaField
            label="关系规则（每行一项）"
            value={relationshipRulesText}
            rows={5}
            onChange={setRelationshipRulesText}
          />
        </div>

        <div className="form-row">
          <TextAreaField
            label="示例对话（JSON 数组）"
            value={examplesText}
            rows={9}
            hint='格式：[{"situation":"安慰","user":"我好累","reply":"先停一下嘛$你已经撑很久了"}]'
            onChange={setExamplesText}
          />
        </div>

        <div className="editor-grid">
          <TextAreaField
            label="旧版性格配置（JSON 对象）"
            value={personalityText}
            rows={10}
            onChange={setPersonalityText}
          />
          <TextAreaField
            label="旧版回复配置（JSON 对象）"
            value={responsesText}
            rows={10}
            onChange={setResponsesText}
          />
          <TextAreaField
            label="旧版行为配置（JSON 对象）"
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

      <Panel eyebrow="提示词预览" title="分层提示词预览" subtitle="后端实际返回的提示词分层内容。">
        <div className="editor-grid">
          <PromptPreview title="基础提示词" content={preview.basePrompt} />
          <PromptPreview title="用户补充提示词" content={preview.userPrompt} />
          <PromptPreview title="角色提示词" content={preview.characterPrompt} />
          <PromptPreview title="代理规则提示词" content={preview.agentPrompt} />
        </div>
        <div className="form-row">
          <PromptPreview title="最终生效提示词" content={preview.effectivePrompt || cfg.effectivePrompt} />
        </div>
      </Panel>

      <Panel
        eyebrow="图片素材"
        title="图片素材索引"
        subtitle="这里先做只读可视化，素材图片和 index.json 仍然在仓库目录里维护。"
      >
        <div className="artifact-grid">
          <div className="artifact-card emphasis">
            <div className="artifact-title">素材索引文件</div>
            <p className="artifact-note">
              {cfg.imageAssetIndexFile || "-"}
              <br />
              目录：{cfg.imageAssetDir || "-"}
            </p>
          </div>

          <div className="artifact-card">
            <div className="artifact-title">已登记素材</div>
            <ul className="artifact-list mono">
              {panel.imageAssets.map((asset) => (
                <li key={asset.id}>
                  <span>{asset.id}</span>
                  <span>{asset.enabled ? "已启用" : "已禁用"}</span>
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
    throw new Error(`示例对话不是合法 JSON: ${err instanceof Error ? err.message : String(err)}`);
  }
  if (!Array.isArray(parsed)) {
    throw new Error("示例对话必须是 JSON 数组");
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
  const personalityRaw = parseJSONObject(editor.personalityText, "旧版性格配置");
  const personality = Object.fromEntries(
    Object.entries(personalityRaw).map(([key, value]) => [String(key), String(value)])
  );
  const responses = parseJSONObject(editor.responsesText, "旧版回复配置");
  const behavior = parseJSONObject(editor.behaviorText, "旧版行为配置");

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
