import { useState } from "react";
import { Panel } from "../components/common/Panel";
import { InputField, SelectField } from "../components/common/FormField";

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
    <Panel
      title="AI Config"
      subtitle={`source: ${cfg.aiConfigFile || "-"} | env: ${cfg.environmentConfig || "-"}`}
      actions={
        <button
          type="button"
          className="btn-primary"
          onClick={() => panel.saveConfig()}
          disabled={panel.saving || panel.loadingConfig}
        >
          {panel.saving ? "Saving..." : "Save Config"}
        </button>
      }
    >
      <div className="form-grid">
        <SelectField
          label="Active Profile"
          value={cfg.aiProfile}
          options={cfg.aiProfiles}
          onChange={(v) => void panel.selectAIProfile(v)}
        />
        <InputField
          label="Create Profile"
          value={newProfileName}
          placeholder="example: deepseek_backup"
          onChange={setNewProfileName}
        />
      </div>

      <div className="form-row prompt-editor-actions">
        <button
          type="button"
          className="btn-ghost"
          onClick={createProfile}
          disabled={panel.saving || panel.loadingAIProfile || !newProfileName.trim()}
        >
          Save As New Profile
        </button>
      </div>

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
          value={cfg.aiTemperature}
          onChange={(v) => updateField(panel, "aiTemperature", v)}
        />
        <InputField
          label="Top P"
          type="number"
          step="0.1"
          value={cfg.aiTopP}
          onChange={(v) => updateField(panel, "aiTopP", v)}
        />
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

      <div className="form-row">
        <InputField
          label="AI Key (leave blank to keep current profile key)"
          placeholder={cfg.aiKeySet ? cfg.aiKeyMasked : "not set"}
          value={cfg.aiKey}
          onChange={(v) => updateField(panel, "aiKey", v)}
        />
      </div>

      <div className="form-row">
        <SelectField
          label="Character (CHARACTER)"
          value={cfg.character}
          options={cfg.characterOptions}
          onChange={(v) => updateField(panel, "character", v)}
        />
      </div>
    </Panel>
  );
}

function updateField(panel, key, value) {
  panel.setConfig((prev) => ({ ...prev, [key]: value }));
}
