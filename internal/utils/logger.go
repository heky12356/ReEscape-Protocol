package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogLevel 日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

const (
	LogFormatText = "text"
	LogFormatJSON = "json"
)

type Field = zap.Field

var defaultLogger *Logger

// Logger 自定义日志器
type Logger struct {
	mu sync.RWMutex

	level       LogLevel
	enableFile  bool
	enableColor bool
	logDir      string
	format      string

	atomicLevel zap.AtomicLevel
	logger      *zap.Logger
	sugar       *zap.SugaredLogger
	file        *os.File
}

func init() {
	defaultLogger = NewLogger(INFO, false, true, "", LogFormatText)
}

// NewLogger 创建新的日志器
func NewLogger(level LogLevel, enableFile bool, enableColor bool, logDir, format string) *Logger {
	logger := &Logger{}
	if err := logger.Configure(level, enableFile, enableColor, logDir, format); err != nil {
		fmt.Fprintf(os.Stderr, "初始化 zap 日志失败: %v\n", err)
	}
	return logger
}

func (l *Logger) Configure(level LogLevel, enableFile bool, enableColor bool, logDir, format string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}

	normalizedFormat := normalizeLogFormat(format)
	atomicLevel := zap.NewAtomicLevelAt(toZapLevel(level))

	consoleEncoder := buildEncoder(normalizedFormat, enableColor)
	consoleCore := zapcore.NewCore(
		consoleEncoder,
		zapcore.AddSync(os.Stdout),
		atomicLevel,
	)

	cores := []zapcore.Core{consoleCore}

	if enableFile {
		file, err := openLogFile(logDir)
		if err != nil {
			return err
		}
		l.file = file
		fileEncoder := buildEncoder(normalizedFormat, false)
		cores = append(cores, zapcore.NewCore(
			fileEncoder,
			zapcore.AddSync(file),
			atomicLevel,
		))
	}

	zapLogger := zap.New(
		zapcore.NewTee(cores...),
	)

	l.level = level
	l.enableFile = enableFile
	l.enableColor = enableColor
	l.logDir = logDir
	l.format = normalizedFormat
	l.atomicLevel = atomicLevel
	l.logger = zapLogger
	l.sugar = zapLogger.Sugar()

	return nil
}

func buildEncoder(format string, enableColor bool) zapcore.Encoder {
	config := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeTime:     zapcore.TimeEncoderOfLayout(time.RFC3339),
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	if format == LogFormatJSON {
		config.EncodeLevel = zapcore.LowercaseLevelEncoder
		return zapcore.NewJSONEncoder(config)
	}

	if enableColor {
		config.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config.EncodeLevel = zapcore.CapitalLevelEncoder
	}
	return zapcore.NewConsoleEncoder(config)
}

func openLogFile(logDir string) (*os.File, error) {
	resolvedDir := strings.TrimSpace(logDir)
	if resolvedDir == "" {
		resolvedDir = "./logs"
	}
	if err := os.MkdirAll(resolvedDir, 0o755); err != nil {
		return nil, err
	}

	filename := fmt.Sprintf("bot_%s.log", time.Now().Format("2006-01-02"))
	fullPath := filepath.Join(resolvedDir, filename)
	return os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
}

func normalizeLogFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case LogFormatJSON:
		return LogFormatJSON
	default:
		return LogFormatText
	}
}

func toZapLevel(level LogLevel) zapcore.Level {
	switch level {
	case DEBUG:
		return zap.DebugLevel
	case WARN:
		return zap.WarnLevel
	case ERROR:
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}

func (l *Logger) cloneLogger() (*zap.Logger, *zap.SugaredLogger) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.logger, l.sugar
}

func (l *Logger) logf(level LogLevel, format string, args ...interface{}) {
	_, sugar := l.cloneLogger()
	if sugar == nil {
		return
	}

	switch level {
	case DEBUG:
		sugar.Debugf(format, args...)
	case WARN:
		sugar.Warnf(format, args...)
	case ERROR:
		sugar.Errorf(format, args...)
	default:
		sugar.Infof(format, args...)
	}
}

func (l *Logger) logw(level LogLevel, msg string, fields ...Field) {
	base, _ := l.cloneLogger()
	if base == nil {
		return
	}

	switch level {
	case DEBUG:
		base.Debug(msg, fields...)
	case WARN:
		base.Warn(msg, fields...)
	case ERROR:
		base.Error(msg, fields...)
	default:
		base.Info(msg, fields...)
	}
}

// Debug 调试日志
func (l *Logger) Debug(format string, args ...interface{}) {
	l.logf(DEBUG, format, args...)
}

// Info 信息日志
func (l *Logger) Info(format string, args ...interface{}) {
	l.logf(INFO, format, args...)
}

// Warn 警告日志
func (l *Logger) Warn(format string, args ...interface{}) {
	l.logf(WARN, format, args...)
}

// Error 错误日志
func (l *Logger) Error(format string, args ...interface{}) {
	l.logf(ERROR, format, args...)
}

func (l *Logger) Debugw(msg string, fields ...Field) {
	l.logw(DEBUG, msg, fields...)
}

func (l *Logger) Infow(msg string, fields ...Field) {
	l.logw(INFO, msg, fields...)
}

func (l *Logger) Warnw(msg string, fields ...Field) {
	l.logw(WARN, msg, fields...)
}

func (l *Logger) Errorw(msg string, fields ...Field) {
	l.logw(ERROR, msg, fields...)
}

// 全局日志函数
func Debug(format string, args ...interface{}) {
	defaultLogger.Debug(format, args...)
}

func Info(format string, args ...interface{}) {
	defaultLogger.Info(format, args...)
}

func Warn(format string, args ...interface{}) {
	defaultLogger.Warn(format, args...)
}

func Error(format string, args ...interface{}) {
	defaultLogger.Error(format, args...)
}

func Debugw(msg string, fields ...Field) {
	defaultLogger.Debugw(msg, fields...)
}

func Infow(msg string, fields ...Field) {
	defaultLogger.Infow(msg, fields...)
}

func Warnw(msg string, fields ...Field) {
	defaultLogger.Warnw(msg, fields...)
}

func Errorw(msg string, fields ...Field) {
	defaultLogger.Errorw(msg, fields...)
}

// SetLevel 设置日志级别
func SetLevel(level LogLevel) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()

	defaultLogger.level = level
	defaultLogger.atomicLevel.SetLevel(toZapLevel(level))
}

func ParseLogLevel(raw string) LogLevel {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DEBUG":
		return DEBUG
	case "WARN", "WARNING":
		return WARN
	case "ERROR":
		return ERROR
	default:
		return INFO
	}
}

func ConfigureDefaultLogger(level LogLevel, enableFile bool, enableColor bool, logDir, format string) error {
	return defaultLogger.Configure(level, enableFile, enableColor, logDir, format)
}

// LogWithContext 带上下文的日志
func LogWithContext(context string, level LogLevel, format string, args ...interface{}) {
	fields := []Field{String("context", context)}
	message := fmt.Sprintf(format, args...)
	defaultLogger.logw(level, message, fields...)
}

func String(key, value string) Field {
	return zap.String(key, value)
}

func Int(key string, value int) Field {
	return zap.Int(key, value)
}

func Int64(key string, value int64) Field {
	return zap.Int64(key, value)
}

func Bool(key string, value bool) Field {
	return zap.Bool(key, value)
}

func Duration(key string, value time.Duration) Field {
	return zap.Duration(key, value)
}

func Any(key string, value interface{}) Field {
	return zap.Any(key, value)
}

func Err(err error) Field {
	return zap.Error(err)
}
