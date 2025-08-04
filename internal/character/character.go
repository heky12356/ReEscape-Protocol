package character

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CharacterConfig 角色配置结构
type CharacterConfig struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Personality map[string]string      `json:"personality"`
	Responses   map[string]interface{} `json:"responses"`
	Behavior    map[string]interface{} `json:"behavior"`
	Quotes      []string               `json:"quotes"` // 例子
}

// CharacterManager 角色管理器
type CharacterManager struct {
	config     *CharacterConfig
	configPath string
	prompt     string
}

// NewCharacterManager 创建新的角色管理器
func NewCharacterManager(configDir, characterName string) (*CharacterManager, error) {
	configPath := filepath.Join(configDir, characterName+".json")

	manager := &CharacterManager{
		configPath: configPath,
	}

	if err := manager.loadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load character config: %w", err)
	}

	return manager, nil
}

// loadConfig 加载配置文件
func (cm *CharacterManager) loadConfig() error {
	// 检查文件是否存在
	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", cm.configPath)
	}

	// 读取文件内容
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// 解析JSON
	var config CharacterConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse JSON config: %w", err)
	}

	// 生成prompt
	cm.prompt = cm.generatePrompt(&config)

	cm.config = &config
	return nil
}

// GetConfig 获取配置
func (cm *CharacterManager) GetConfig() *CharacterConfig {
	return cm.config
}

// ReloadConfig 重新加载配置
func (cm *CharacterManager) ReloadConfig() error {
	return cm.loadConfig()
}

// SaveConfig 保存配置
func (cm *CharacterManager) SaveConfig() error {
	data, err := json.MarshalIndent(cm.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(cm.configPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetPrompt 获取对应prompt
func (cm *CharacterManager) GetPrompt() string {
	return cm.prompt
}

// generatePrompt 基于配置生成prompt
func (cm *CharacterManager) generatePrompt(config *CharacterConfig) string {
	var prompt string

	// 基础角色信息
	prompt += fmt.Sprintf("你是%s，%s\n\n", config.Name, config.Description)

	// 性格特征
	if len(config.Personality) > 0 {
		prompt += "【性格特征】\n"
		for key, value := range config.Personality {
			prompt += fmt.Sprintf("- %s: %s\n", key, value)
		}
		prompt += "\n"
	}

	// 行为特征
	if len(config.Behavior) > 0 {
		prompt += "【行为特征】\n"
		for key, value := range config.Behavior {
			switch v := value.(type) {
			case string:
				prompt += fmt.Sprintf("- %s: %s\n", key, v)
			case map[string]interface{}:
				prompt += fmt.Sprintf("- %s:\n", key)
				for subKey, subValue := range v {
					switch sv := subValue.(type) {
					case string:
						prompt += fmt.Sprintf("  * %s: %s\n", subKey, sv)
					case []interface{}:
						prompt += fmt.Sprintf("  * %s: ", subKey)
						for i, item := range sv {
							if i > 0 {
								prompt += ", "
							}
							prompt += fmt.Sprintf("%v", item)
						}
						prompt += "\n"
					}
				}
			}
		}
		prompt += "\n"
	}

	// 回复风格
	if len(config.Responses) > 0 {
		prompt += "【回复风格】\n"
		for key, value := range config.Responses {
			if responses, ok := value.([]interface{}); ok {
				prompt += fmt.Sprintf("- %s时的回复示例:\n", key)
				for _, response := range responses {
					prompt += fmt.Sprintf("  * \"%v\"\n", response)
				}
			}
		}
		prompt += "\n"
	}

	// 经典语录
	if len(config.Quotes) > 0 {
		prompt += "【经典语录】\n"
		for _, quote := range config.Quotes {
			prompt += fmt.Sprintf("- \"%s\"\n", quote)
		}
		prompt += "\n"
	}

	// 角色扮演指令
	prompt += "【角色扮演要求】\n"
	prompt += "请严格按照以上角色设定进行对话，保持角色的一致性和真实感。\n"
	prompt += "回复时要体现出角色的性格特点、说话风格和行为习惯。"

	return prompt
}
