import { useState } from "react";
import { Panel } from "../components/common/Panel";
import { InputField, SelectField } from "../components/common/FormField";

const BOOLEAN_OPTIONS = ["true", "false"];

export function AIConfigPage({ panel }) {
  const cfg = panel.config;
  const [newProfileName, setNewProfileName] = useState("");

  const createProfile = async () => {
    const target = newProfileName.trim();
    if (!target) {
      panel.setError("new ai profile name is required");
      return;
    }
    await panel.saveConfig({ aiProfile: target });
    setNewProfileName("");
  };

  return (
    <div className="stack">
      <Panel
        eyebrow="Routing deck"
        title="模型路由与身份切换"
        subtitle="把配置档、角色文件和 API 入口放在同一张调音台上处理。"
        actions={
          <button
            type="button"
            className="btn-primary"
            onClick={() => panel.saveConfig()}
            disabled={panel.saving || panel.loadingConfig}
          >
            {panel.saving ? "保存中..." : "保存配置"}
          </button>
        }
      >
        <div className="split-layout">
          <div className="stack compact">
            <SelectField
              label="Active profile"
              value={cfg.aiProfile}
              options={cfg.aiProfiles}
              hint="切换当前生效的模型配置档。"
              onChange={(v) => void panel.selectAIProfile(v)}
            />
            <InputField
              label="Create profile"
              value={newProfileName}
              placeholder="example: deepseek_backup"
              hint="先输入名字，再从当前表单内容复制出一个新档。"
              onChange={setNewProfileName}
            />
            <div className="action-row">
              <button
                type="button"
                className="btn-ghost"
                onClick={createProfile}
                disabled={panel.saving || panel.loadingAIProfile || !newProfileName.trim()}
              >
                另存为新档
              </button>
            </div>
          </div>

          <div className="insight-card">
            <div className="insight-kicker">Signal note</div>
            <div className="insight-title">当前这层决定的是模型怎么回答，不是人格怎么说话。</div>
            <p className="insight-copy">
              Base URL、模型名、采样参数和限流放在一起，是因为它们共同决定延迟、稳定性和语气边界。
            </p>
            <div className="insight-meta mono">{cfg.aiConfigFile || "-"}</div>
          </div>
        </div>
      </Panel>

      <Panel eyebrow="Transport" title="入口与采样" subtitle="先决定接到哪路模型，再决定回答的自由度。">
        <div className="form-grid">
          <InputField
            label="AI Base URL"
            value={cfg.aiBaseUrl}
            onChange={(v) => updateField(panel, "aiBaseUrl", v)}
          />
          <InputField
            label="AI Model"
            value={cfg.aiModel}
            onChange={(v) => updateField(panel, "aiModel", v)}
          />
          <InputField
            label="Temperature"
            type="number"
            step="0.1"
            hint="越高越放，越低越稳。"
            value={cfg.aiTemperature}
            onChange={(v) => updateField(panel, "aiTemperature", v)}
          />
          <InputField
            label="Top P"
            type="number"
            step="0.1"
            hint="和 temperature 一起决定采样边界。"
            value={cfg.aiTopP}
            onChange={(v) => updateField(panel, "aiTopP", v)}
          />
        </div>
      </Panel>

      <Panel
        eyebrow="Temporal anchor"
        title="真实时间锚点"
        subtitle="这层决定模型如何理解今天、明天、昨晚和现在，不改语气，只校正时间感。"
      >
        <div className="split-layout">
          <div className="form-grid">
            <SelectField
              label="Enable time context"
              value={String(cfg.enableTimeContext)}
              options={BOOLEAN_OPTIONS}
              hint="关闭后，模型将不再收到动态时间上下文。"
              onChange={(v) => updateField(panel, "enableTimeContext", v === "true")}
            />
            <InputField
              label="Timezone"
              value={cfg.timeContextTimezone}
              placeholder="Asia/Shanghai"
              hint="IANA 时区名，例如 Asia/Shanghai 或 America/Los_Angeles。"
              onChange={(v) => updateField(panel, "timeContextTimezone", v)}
            />
            <InputField
              label="Time format"
              value={cfg.timeContextFormat}
              placeholder="2006-01-02 15:04:05"
              hint="Go 时间格式，决定注入给模型的显示样式。"
              onChange={(v) => updateField(panel, "timeContextFormat", v)}
            />
          </div>

          <div className="insight-card">
            <div className="insight-kicker">Clock note</div>
            <div className="insight-title">模型现在会拿到一份动态时间标签，而不是自己猜今天是几号。</div>
            <p className="insight-copy">
              推荐保持开启，并用业务真实时区对齐。这样“今天”“明天”“周末”“昨晚”这些相对时间会稳定很多。
            </p>
            <div className="insight-meta mono">
              {cfg.enableTimeContext ? `${cfg.timeContextTimezone || "default"} / ${cfg.timeContextFormat || "default"}` : "time context disabled"}
            </div>
          </div>
        </div>
      </Panel>

      <Panel eyebrow="Guardrails" title="吞吐与重试" subtitle="让模型失败时更可控，而不是更吵。">
        <div className="form-grid">
          <InputField
            label="Max Tokens"
            type="number"
            value={cfg.aiMaxTokens}
            onChange={(v) => updateField(panel, "aiMaxTokens", v)}
          />
          <InputField
            label="Timeout (s)"
            type="number"
            value={cfg.aiTimeout}
            onChange={(v) => updateField(panel, "aiTimeout", v)}
          />
          <InputField
            label="Retry Count"
            type="number"
            value={cfg.aiRetryCount}
            onChange={(v) => updateField(panel, "aiRetryCount", v)}
          />
          <InputField
            label="Rate Limit (/min)"
            type="number"
            value={cfg.aiRateLimit}
            onChange={(v) => updateField(panel, "aiRateLimit", v)}
          />
        </div>
      </Panel>

      <Panel eyebrow="Secrets" title="访问密钥与人格映射" subtitle="保留关键入口，但把高风险字段放在单独一层。">
        <div className="form-grid">
          <InputField
            label="AI Key"
            placeholder={cfg.aiKeySet ? cfg.aiKeyMasked : "not set"}
            hint="留空则保留当前 profile 已有密钥。"
            value={cfg.aiKey}
            onChange={(v) => updateField(panel, "aiKey", v)}
          />
          <SelectField
            label="Character (CHARACTER)"
            value={cfg.character}
            options={cfg.characterOptions}
            hint="人格文件由服务端在保存配置时同步应用。"
            onChange={(v) => updateField(panel, "character", v)}
          />
        </div>
      </Panel>
    </div>
  );
}

function updateField(panel, key, value) {
  panel.setConfig((prev) => ({ ...prev, [key]: value }));
}
