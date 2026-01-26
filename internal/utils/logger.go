package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

// LogLevel 日志级别
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var levelNames = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
}

var levelColors = map[LogLevel]string{
	DEBUG: "\033[36m", // 青色
	INFO:  "\033[32m", // 绿色
	WARN:  "\033[33m", // 黄色
	ERROR: "\033[31m", // 红色
}

const colorReset = "\033[0m"

// Logger 自定义日志器
type Logger struct {
	level       LogLevel
	fileWriter  io.Writer
	enableFile  bool
	enableColor bool
}

var defaultLogger *Logger

func init() {
	defaultLogger = NewLogger(INFO, true, true)
}

// NewLogger 创建新的日志器
func NewLogger(level LogLevel, enableFile bool, enableColor bool) *Logger {
	logger := &Logger{
		level:       level,
		enableFile:  enableFile,
		enableColor: enableColor,
	}

	if enableFile {
		if err := logger.initFileWriter(); err != nil {
			log.Printf("初始化文件日志失败: %v", err)
		}
	}

	return logger
}

// initFileWriter 初始化文件写入器
func (l *Logger) initFileWriter() error {
	logDir := "./logs"
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return err
	}

	filename := fmt.Sprintf("bot_%s.log", time.Now().Format("2006-01-02"))
	filepath := filepath.Join(logDir, filename)

	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		return err
	}

	l.fileWriter = file
	return nil
}

// log 内部日志方法
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelName := levelNames[level]
	message := fmt.Sprintf(format, args...)

	// 控制台输出（带颜色）
	if l.enableColor {
		color := levelColors[level]
		fmt.Printf("%s[%s] %s%s %s\n", color, timestamp, levelName, colorReset, message)
	} else {
		fmt.Printf("[%s] %s %s\n", timestamp, levelName, message)
	}

	// 文件输出（无颜色）
	if l.enableFile && l.fileWriter != nil {
		fileMessage := fmt.Sprintf("[%s] %s %s\n", timestamp, levelName, message)
		_, _ = l.fileWriter.Write([]byte(fileMessage))
	}
}

// Debug 调试日志
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info 信息日志
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn 警告日志
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error 错误日志
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
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

// SetLevel 设置日志级别
func SetLevel(level LogLevel) {
	defaultLogger.level = level
}

// LogWithContext 带上下文的日志
func LogWithContext(context string, level LogLevel, format string, args ...interface{}) {
	message := fmt.Sprintf("[%s] %s", context, fmt.Sprintf(format, args...))
	defaultLogger.log(level, "%s", message)
}
