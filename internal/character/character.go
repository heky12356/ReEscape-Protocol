package character

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

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
	NormalizeConfig(&config)
	if err := ValidateConfig(&config); err != nil {
		return fmt.Errorf("invalid character config: %w", err)
	}

	// 生成prompt
	cm.prompt = BuildPrompt(config)

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
	NormalizeConfig(cm.config)
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
