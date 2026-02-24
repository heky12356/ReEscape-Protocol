package admin

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"project-yume/internal/aifunction"
	"project-yume/internal/character"
	"project-yume/internal/config"
	"project-yume/internal/utils"

	"github.com/gin-gonic/gin"
)

const (
	defaultHTTPPort = "8088"
	defaultLogLines = 200
	maxLogLines     = 2000
)

type configResponse struct {
	AIBaseURL         string   `json:"aiBaseUrl"`
	AIModel           string   `json:"aiModel"`
	AIKeyMasked       string   `json:"aiKeyMasked"`
	AIKeySet          bool     `json:"aiKeySet"`
	AIProfile         string   `json:"aiProfile"`
	AIProfiles        []string `json:"aiProfiles"`
	AIConfigFile      string   `json:"aiConfigFile"`
	AITemperature     float32  `json:"aiTemperature"`
	AIMaxTokens       int      `json:"aiMaxTokens"`
	AITimeout         int      `json:"aiTimeout"`
	AIRetryCount      int      `json:"aiRetryCount"`
	AIRateLimit       int      `json:"aiRateLimit"`
	AITopP            float32  `json:"aiTopP"`
	AIPromptRaw       string   `json:"aiPromptRaw"`
	Character         string   `json:"character"`
	CharacterOptions  []string `json:"characterOptions"`
	EffectivePrompt   string   `json:"effectivePrompt"`
	EnvironmentConfig string   `json:"environmentConfig"`
}

type aiProfileResponse struct {
	Name          string  `json:"name"`
	AIBaseURL     string  `json:"aiBaseUrl"`
	AIModel       string  `json:"aiModel"`
	AIKeyMasked   string  `json:"aiKeyMasked"`
	AIKeySet      bool    `json:"aiKeySet"`
	AITemperature float32 `json:"aiTemperature"`
	AIMaxTokens   int     `json:"aiMaxTokens"`
	AITimeout     int     `json:"aiTimeout"`
	AIRetryCount  int     `json:"aiRetryCount"`
	AIRateLimit   int     `json:"aiRateLimit"`
	AITopP        float32 `json:"aiTopP"`
}

type updateConfigRequest struct {
	AIBaseURL     string  `json:"aiBaseUrl"`
	AIModel       string  `json:"aiModel"`
	AIProfile     string  `json:"aiProfile"`
	AITemperature float32 `json:"aiTemperature"`
	AIMaxTokens   int     `json:"aiMaxTokens"`
	AITimeout     int     `json:"aiTimeout"`
	AIRetryCount  int     `json:"aiRetryCount"`
	AIRateLimit   int     `json:"aiRateLimit"`
	AITopP        float32 `json:"aiTopP"`
	AIPromptRaw   string  `json:"aiPromptRaw"`
	Character     string  `json:"character"`
	AIKey         string  `json:"aiKey"`
}

type logFileInfo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

type logContentResponse struct {
	File    string `json:"file"`
	Lines   int    `json:"lines"`
	Content string `json:"content"`
}

type characterConfigResponse struct {
	File   string                    `json:"file"`
	Config character.CharacterConfig `json:"config"`
}

type saveCharacterRequest struct {
	Config character.CharacterConfig `json:"config"`
}

type createCharacterRequest struct {
	Name   string                    `json:"name"`
	Config character.CharacterConfig `json:"config"`
}

type server struct {
	webDistDir string
}

func Start(ctx context.Context) {
	s := &server{
		webDistDir: filepath.Clean("./web/dist"),
	}

	httpPort := config.GetConfig().HttpPort
	if strings.TrimSpace(httpPort) == "" {
		httpPort = defaultHTTPPort
	}

	engine := s.routes()
	httpServer := &http.Server{
		Addr:    ":" + httpPort,
		Handler: engine,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	utils.Info("admin web server listening on :%s", httpPort)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		utils.Error("admin web server error: %v", err)
	}
}

func (s *server) routes() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(corsMiddleware())

	adminGroup := engine.Group("/api/admin")
	{
		adminGroup.GET("/config", s.handleGetConfig)
		adminGroup.PUT("/config", s.handlePutConfig)
		adminGroup.GET("/ai-profiles/:name", s.handleGetAIProfile)
		adminGroup.GET("/logs/files", s.handleLogFiles)
		adminGroup.GET("/logs/content", s.handleLogContent)
		adminGroup.GET("/logs/stream", s.handleLogStream)
		adminGroup.GET("/characters", s.handleCharacters)
		adminGroup.GET("/characters/:name", s.handleGetCharacterConfig)
		adminGroup.PUT("/characters/:name", s.handleUpdateCharacterConfig)
		adminGroup.POST("/characters", s.handleCreateCharacterConfig)
	}

	engine.NoRoute(func(c *gin.Context) {
		s.handleWeb(c.Writer, c.Request)
	})

	return engine
}

func (s *server) handleGetConfig(c *gin.Context) {
	resp, err := s.buildConfigResponse()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (s *server) handlePutConfig(c *gin.Context) {
	var req updateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}
	if err := validateUpdateRequest(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	profileName := strings.TrimSpace(req.AIProfile)
	aiConfigFile := config.GetAIConfigFilePath()

	aiProfiles, err := config.EnsureAIProfileSet(aiConfigFile, profileName, config.AIProfile{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("load ai profiles failed: %v", err)})
		return
	}

	existingProfile, exists := aiProfiles.Profiles[profileName]
	aiKey := strings.TrimSpace(req.AIKey)
	if aiKey == "" {
		if exists && strings.TrimSpace(existingProfile.AIKey) != "" {
			aiKey = strings.TrimSpace(existingProfile.AIKey)
		} else if currentActive, ok := aiProfiles.Profiles[aiProfiles.Active]; ok {
			aiKey = strings.TrimSpace(currentActive.AIKey)
		}
	}

	targetProfile := config.AIProfile{
		AIBaseURL:     req.AIBaseURL,
		AIModel:       req.AIModel,
		AIKey:         aiKey,
		AITemperature: req.AITemperature,
		AIMaxTokens:   req.AIMaxTokens,
		AITimeout:     req.AITimeout,
		AIRetryCount:  req.AIRetryCount,
		AIRateLimit:   req.AIRateLimit,
		AITopP:        req.AITopP,
	}
	if err := config.ValidateAIProfile(targetProfile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	savedProfileName, err := config.UpsertAIProfile(&aiProfiles, profileName, targetProfile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := config.SetActiveAIProfileName(&aiProfiles, savedProfileName); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := config.SaveAIProfileSet(aiConfigFile, aiProfiles); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("save ai profiles failed: %v", err)})
		return
	}

	updates := map[string]string{
		"AI_PROFILE":     savedProfileName,
		"AI_CONFIG_FILE": aiConfigFile,
		"AI_PROMPT":      req.AIPromptRaw,
		"CHARACTER":      req.Character,
	}

	envFile := resolveEnvFilePath(config.GetEnvFilePath())
	if err := upsertEnvFile(envFile, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("update env failed: %v", err)})
		return
	}

	for key, value := range updates {
		_ = os.Setenv(key, value)
	}

	if err := config.ReloadRuntimeConfig(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("reload config failed: %v", err)})
		return
	}
	aifunction.ReloadClient()
	aifunction.ResetRateLimiter()

	resp, err := s.buildConfigResponse()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (s *server) handleGetAIProfile(c *gin.Context) {
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ai profile name is required"})
		return
	}

	cfg := config.GetConfig()
	aiProfiles, err := config.EnsureAIProfileSet(
		config.GetAIConfigFilePath(),
		cfg.AiProfile,
		config.AIProfile{
			AIBaseURL:     cfg.AiBaseUrl,
			AIModel:       cfg.AiModel,
			AIKey:         cfg.AiKEY,
			AITemperature: cfg.AiTemperature,
			AIMaxTokens:   cfg.AiMaxTokens,
			AITimeout:     cfg.AiTimeout,
			AIRetryCount:  cfg.AiRetryCount,
			AIRateLimit:   cfg.AiRateLimit,
			AITopP:        cfg.AiTopP,
		},
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("load ai profiles failed: %v", err)})
		return
	}

	profile, ok := aiProfiles.Profiles[name]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("ai profile not found: %s", name)})
		return
	}

	c.JSON(http.StatusOK, aiProfileResponse{
		Name:          name,
		AIBaseURL:     profile.AIBaseURL,
		AIModel:       profile.AIModel,
		AIKeyMasked:   maskSecret(profile.AIKey),
		AIKeySet:      strings.TrimSpace(profile.AIKey) != "",
		AITemperature: profile.AITemperature,
		AIMaxTokens:   profile.AIMaxTokens,
		AITimeout:     profile.AITimeout,
		AIRetryCount:  profile.AIRetryCount,
		AIRateLimit:   profile.AIRateLimit,
		AITopP:        profile.AITopP,
	})
}

func (s *server) handleLogFiles(c *gin.Context) {
	setNoCacheHeaders(c)

	logFiles, err := listLogFiles(currentLogDir())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, logFiles)
}

func (s *server) handleLogContent(c *gin.Context) {
	setNoCacheHeaders(c)

	filename := strings.TrimSpace(c.Query("file"))
	if filename == "" {
		files, err := listLogFiles(currentLogDir())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if len(files) == 0 {
			c.JSON(http.StatusOK, logContentResponse{})
			return
		}
		filename = files[0].Name
	}

	lines := defaultLogLines
	if raw := c.Query("lines"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil && parsed > 0 {
			if parsed > maxLogLines {
				parsed = maxLogLines
			}
			lines = parsed
		}
	}

	content, err := tailLogFile(currentLogDir(), filename, lines)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, logContentResponse{
		File:    filename,
		Lines:   lines,
		Content: content,
	})
}

func (s *server) handleLogStream(c *gin.Context) {
	setNoCacheHeaders(c)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	lines := defaultLogLines
	if raw := c.Query("lines"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil && parsed > 0 {
			if parsed > maxLogLines {
				parsed = maxLogLines
			}
			lines = parsed
		}
	}

	autoSelectLatest := strings.TrimSpace(c.Query("file")) == ""
	filename := strings.TrimSpace(c.Query("file"))
	logDir := currentLogDir()

	if filename == "" {
		latest, err := latestLogFileName(logDir)
		if err != nil {
			_ = writeSSE(c.Writer, flusher, "error", map[string]string{"error": err.Error()})
			return
		}
		filename = latest
	}

	offset := int64(0)
	if filename != "" {
		initial, lastOffset, err := tailLogFileWithOffset(logDir, filename, lines)
		if err != nil {
			_ = writeSSE(c.Writer, flusher, "error", map[string]string{"error": err.Error()})
			return
		}
		offset = lastOffset
		_ = writeSSE(c.Writer, flusher, "init", map[string]string{
			"file":    filename,
			"content": initial,
		})
	} else {
		_ = writeSSE(c.Writer, flusher, "init", map[string]string{
			"file":    "",
			"content": "",
		})
	}

	ticker := time.NewTicker(900 * time.Millisecond)
	heartbeat := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	defer heartbeat.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			if autoSelectLatest {
				latest, err := latestLogFileName(logDir)
				if err == nil && latest != "" && latest != filename {
					filename = latest
					initial, lastOffset, tailErr := tailLogFileWithOffset(logDir, filename, lines)
					if tailErr != nil {
						_ = writeSSE(c.Writer, flusher, "error", map[string]string{"error": tailErr.Error()})
						continue
					}
					offset = lastOffset
					_ = writeSSE(c.Writer, flusher, "reset", map[string]string{
						"file":    filename,
						"content": initial,
					})
					continue
				}
			}

			if filename == "" {
				continue
			}

			appendContent, nextOffset, rotated, err := readLogAppend(logDir, filename, offset)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				_ = writeSSE(c.Writer, flusher, "error", map[string]string{"error": err.Error()})
				continue
			}

			if rotated {
				initial, lastOffset, tailErr := tailLogFileWithOffset(logDir, filename, lines)
				if tailErr != nil {
					_ = writeSSE(c.Writer, flusher, "error", map[string]string{"error": tailErr.Error()})
					continue
				}
				offset = lastOffset
				_ = writeSSE(c.Writer, flusher, "reset", map[string]string{
					"file":    filename,
					"content": initial,
				})
				continue
			}

			offset = nextOffset
			if appendContent == "" {
				continue
			}

			_ = writeSSE(c.Writer, flusher, "append", map[string]string{
				"file":    filename,
				"content": appendContent,
			})
		case <-heartbeat.C:
			_ = writeSSE(c.Writer, flusher, "ping", map[string]int64{"ts": time.Now().Unix()})
		}
	}
}

func setNoCacheHeaders(c *gin.Context) {
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
}

func latestLogFileName(logDir string) (string, error) {
	files, err := listLogFiles(logDir)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", nil
	}
	return files[0].Name, nil
}

func (s *server) handleCharacters(c *gin.Context) {
	names, err := listCharacterNames()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, names)
}

func (s *server) handleGetCharacterConfig(c *gin.Context) {
	name, err := normalizeCharacterName(c.Param("name"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg, err := readCharacterConfigFile(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, characterConfigResponse{
		File:   name,
		Config: cfg,
	})
}

func (s *server) handleUpdateCharacterConfig(c *gin.Context) {
	name, err := normalizeCharacterName(c.Param("name"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var req saveCharacterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}
	if err := validateCharacterConfig(&req.Config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := writeCharacterConfigFile(name, req.Config, true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := config.ReloadRuntimeConfig(); err != nil {
		utils.Warn("reload runtime config after character update failed: %v", err)
	}

	c.JSON(http.StatusOK, characterConfigResponse{
		File:   name,
		Config: req.Config,
	})
}

func (s *server) handleCreateCharacterConfig(c *gin.Context) {
	var req createCharacterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	name, err := normalizeCharacterName(req.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateCharacterConfig(&req.Config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if strings.TrimSpace(req.Config.Name) == "" {
		req.Config.Name = name
	}

	if err := writeCharacterConfigFile(name, req.Config, false); err != nil {
		if errors.Is(err, os.ErrExist) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := config.ReloadRuntimeConfig(); err != nil {
		utils.Warn("reload runtime config after character create failed: %v", err)
	}

	c.JSON(http.StatusCreated, characterConfigResponse{
		File:   name,
		Config: req.Config,
	})
}

func (s *server) handleWeb(w http.ResponseWriter, r *http.Request) {
	if _, err := os.Stat(s.webDistDir); err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("web ui not built. run `cd web && npm install && npm run build`"))
		return
	}

	if r.URL.Path == "/" {
		http.ServeFile(w, r, filepath.Join(s.webDistDir, "index.html"))
		return
	}

	cleanPath := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	target := filepath.Join(s.webDistDir, cleanPath)
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		http.ServeFile(w, r, target)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.webDistDir, "index.html"))
}

func (s *server) buildConfigResponse() (configResponse, error) {
	cfg := config.GetConfig()
	characterOptions, err := listCharacterNames()
	if err != nil {
		return configResponse{}, err
	}

	aiProfiles, err := config.EnsureAIProfileSet(
		config.GetAIConfigFilePath(),
		cfg.AiProfile,
		config.AIProfile{
			AIBaseURL:     cfg.AiBaseUrl,
			AIModel:       cfg.AiModel,
			AIKey:         cfg.AiKEY,
			AITemperature: cfg.AiTemperature,
			AIMaxTokens:   cfg.AiMaxTokens,
			AITimeout:     cfg.AiTimeout,
			AIRetryCount:  cfg.AiRetryCount,
			AIRateLimit:   cfg.AiRateLimit,
			AITopP:        cfg.AiTopP,
		},
	)
	if err != nil {
		return configResponse{}, err
	}
	aiProfileNames := config.AIProfileNames(aiProfiles)

	envFile := resolveEnvFilePath(config.GetEnvFilePath())
	envMap, _ := readEnvMap(envFile)
	character := firstNonEmpty(cfg.Character, readEnvValue(envMap, "CHARACTER", "Character"))
	aiPromptRaw := firstNonEmpty(os.Getenv("AI_PROMPT"), readEnvValue(envMap, "AI_PROMPT", "AiPrompt"))

	return configResponse{
		AIBaseURL:         cfg.AiBaseUrl,
		AIModel:           cfg.AiModel,
		AIKeyMasked:       maskSecret(cfg.AiKEY),
		AIKeySet:          strings.TrimSpace(cfg.AiKEY) != "",
		AIProfile:         cfg.AiProfile,
		AIProfiles:        aiProfileNames,
		AIConfigFile:      cfg.AiConfigFile,
		AITemperature:     cfg.AiTemperature,
		AIMaxTokens:       cfg.AiMaxTokens,
		AITimeout:         cfg.AiTimeout,
		AIRetryCount:      cfg.AiRetryCount,
		AIRateLimit:       cfg.AiRateLimit,
		AITopP:            cfg.AiTopP,
		AIPromptRaw:       aiPromptRaw,
		Character:         character,
		CharacterOptions:  characterOptions,
		EffectivePrompt:   cfg.AiPrompt,
		EnvironmentConfig: envFile,
	}, nil
}

func readEnvMap(path string) (map[string]string, error) {
	result := make(map[string]string)

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}

		if idx := strings.Index(value, " #"); idx >= 0 {
			value = strings.TrimSpace(value[:idx])
		}
		if len(value) >= 2 {
			if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
				(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
				value = value[1 : len(value)-1]
			}
		}

		result[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func readEnvValue(envMap map[string]string, keys ...string) string {
	for _, key := range keys {
		for existingKey, value := range envMap {
			if strings.EqualFold(strings.TrimSpace(existingKey), strings.TrimSpace(key)) {
				trimmed := strings.TrimSpace(value)
				if trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func resolveEnvFilePath(path string) string {
	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) {
		return cleaned
	}

	candidates := []string{
		cleaned,
		filepath.Join(".", cleaned),
		filepath.Join("..", cleaned),
		filepath.Join("..", "..", cleaned),
		filepath.Join("..", "..", "..", cleaned),
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates,
			filepath.Join(exeDir, cleaned),
			filepath.Join(exeDir, "..", cleaned),
			filepath.Join(exeDir, "..", "..", cleaned),
		)
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Clean(candidate)
		}
	}

	return cleaned
}

func validateUpdateRequest(req updateConfigRequest) error {
	if strings.TrimSpace(req.AIProfile) == "" {
		return fmt.Errorf("aiProfile is required")
	}
	if strings.TrimSpace(req.AIBaseURL) == "" {
		return fmt.Errorf("aiBaseUrl is required")
	}
	if strings.TrimSpace(req.AIModel) == "" {
		return fmt.Errorf("aiModel is required")
	}
	if strings.TrimSpace(req.Character) == "" {
		return fmt.Errorf("character is required")
	}
	if req.AIMaxTokens <= 0 {
		return fmt.Errorf("aiMaxTokens must be > 0")
	}
	if req.AITimeout <= 0 {
		return fmt.Errorf("aiTimeout must be > 0")
	}
	if req.AIRetryCount < 0 {
		return fmt.Errorf("aiRetryCount must be >= 0")
	}
	if req.AIRateLimit <= 0 {
		return fmt.Errorf("aiRateLimit must be > 0")
	}
	if req.AITemperature < 0 || req.AITemperature > 2 {
		return fmt.Errorf("aiTemperature must be in [0,2]")
	}
	if req.AITopP < 0 || req.AITopP > 1 {
		return fmt.Errorf("aiTopP must be in [0,1]")
	}
	return nil
}

func currentLogDir() string {
	logDir := config.GetConfig().LogDir
	if strings.TrimSpace(logDir) == "" {
		return "./logs"
	}
	return logDir
}

func listCharacterNames() ([]string, error) {
	if err := os.MkdirAll(characterConfigDir(), 0o755); err != nil {
		return nil, err
	}
	files, err := os.ReadDir(characterConfigDir())
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if strings.HasSuffix(name, ".json") {
			names = append(names, strings.TrimSuffix(name, ".json"))
		}
	}
	sort.Strings(names)
	return names, nil
}

func characterConfigDir() string {
	return filepath.Clean("./config/character")
}

func normalizeCharacterName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	name = strings.TrimSuffix(name, ".json")
	name = strings.TrimSpace(name)

	if name == "" {
		return "", fmt.Errorf("character name is required")
	}
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid character name")
	}
	if strings.ContainsAny(name, `/\:*?"<>|`) {
		return "", fmt.Errorf("invalid character name")
	}
	return name, nil
}

func characterFilePath(name string) (string, error) {
	normalized, err := normalizeCharacterName(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(characterConfigDir(), normalized+".json"), nil
}

func readCharacterConfigFile(name string) (character.CharacterConfig, error) {
	path, err := characterFilePath(name)
	if err != nil {
		return character.CharacterConfig{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return character.CharacterConfig{}, fmt.Errorf("read character file failed: %w", err)
	}

	var cfg character.CharacterConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return character.CharacterConfig{}, fmt.Errorf("parse character file failed: %w", err)
	}
	normalizeCharacterConfig(&cfg)
	return cfg, nil
}

func writeCharacterConfigFile(name string, cfg character.CharacterConfig, overwrite bool) error {
	if err := os.MkdirAll(characterConfigDir(), 0o755); err != nil {
		return err
	}
	path, err := characterFilePath(name)
	if err != nil {
		return err
	}
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%w: character file already exists: %s", os.ErrExist, name)
		}
	}

	normalizeCharacterConfig(&cfg)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal character config failed: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write character file failed: %w", err)
	}
	return nil
}

func normalizeCharacterConfig(cfg *character.CharacterConfig) {
	cfg.Name = strings.TrimSpace(cfg.Name)
	cfg.Description = strings.TrimSpace(cfg.Description)
	if cfg.Personality == nil {
		cfg.Personality = map[string]string{}
	}
	if cfg.Responses == nil {
		cfg.Responses = map[string]interface{}{}
	}
	if cfg.Behavior == nil {
		cfg.Behavior = map[string]interface{}{}
	}
	if cfg.Quotes == nil {
		cfg.Quotes = []string{}
	}
}

func validateCharacterConfig(cfg *character.CharacterConfig) error {
	normalizeCharacterConfig(cfg)
	if cfg.Name == "" {
		return fmt.Errorf("character config.name is required")
	}
	return nil
}

func listLogFiles(logDir string) ([]logFileInfo, error) {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil, err
	}

	files := make([]logFileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, logFileInfo{
			Name:    entry.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})
	return files, nil
}

func tailLogFile(logDir, filename string, lines int) (string, error) {
	if filename == "" {
		return "", fmt.Errorf("file is required")
	}

	cleanFilename := filepath.Base(filename)
	fullPath := filepath.Join(logDir, cleanFilename)

	file, err := os.Open(fullPath)
	if err != nil {
		return "", fmt.Errorf("open log file failed: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("read log file failed: %w", err)
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	rows := strings.Split(content, "\n")
	if len(rows) == 0 {
		return "", nil
	}

	if rows[len(rows)-1] == "" {
		rows = rows[:len(rows)-1]
	}

	start := 0
	if len(rows) > lines {
		start = len(rows) - lines
	}

	return strings.Join(rows[start:], "\n"), nil
}

func tailLogFileWithOffset(logDir, filename string, lines int) (string, int64, error) {
	content, err := tailLogFile(logDir, filename, lines)
	if err != nil {
		return "", 0, err
	}

	fullPath := filepath.Join(logDir, filepath.Base(filename))
	info, err := os.Stat(fullPath)
	if err != nil {
		return "", 0, err
	}
	return content, info.Size(), nil
}

func readLogAppend(logDir, filename string, offset int64) (string, int64, bool, error) {
	fullPath := filepath.Join(logDir, filepath.Base(filename))
	info, err := os.Stat(fullPath)
	if err != nil {
		return "", offset, false, err
	}

	size := info.Size()
	if size < offset {
		return "", size, true, nil
	}
	if size == offset {
		return "", offset, false, nil
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return "", offset, false, err
	}
	defer file.Close()

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return "", offset, false, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return "", offset, false, err
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	return content, size, false, nil
}

func writeSSE(w io.Writer, flusher http.Flusher, event string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func upsertEnvFile(path string, updates map[string]string) error {
	var lines []string
	if data, err := os.ReadFile(path); err == nil {
		content := strings.ReplaceAll(string(data), "\r\n", "\n")
		lines = strings.Split(content, "\n")
	} else if !os.IsNotExist(err) {
		return err
	}

	used := make(map[string]bool, len(updates))

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, _, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value, exists := updates[key]
		if !exists {
			continue
		}
		lines[i] = key + "=" + formatEnvValue(value)
		used[key] = true
	}

	remainKeys := make([]string, 0, len(updates))
	for key := range updates {
		if used[key] {
			continue
		}
		remainKeys = append(remainKeys, key)
	}
	sort.Strings(remainKeys)
	for _, key := range remainKeys {
		lines = append(lines, key+"="+formatEnvValue(updates[key]))
	}

	output := strings.Join(lines, "\n")
	if !strings.HasSuffix(output, "\n") {
		output += "\n"
	}
	return os.WriteFile(path, []byte(output), 0o644)
}

func formatEnvValue(value string) string {
	if value == "" {
		return `""`
	}
	needQuote := strings.ContainsAny(value, " \t#\"'") || strings.Contains(value, "\n") || strings.Contains(value, "\r")
	if !needQuote {
		return value
	}
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	escaped := strconv.Quote(value)
	return escaped
}

func maskSecret(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 8 {
		return "****"
	}
	return trimmed[:4] + strings.Repeat("*", len(trimmed)-8) + trimmed[len(trimmed)-4:]
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, PUT, OPTIONS")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
