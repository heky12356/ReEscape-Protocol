package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// CheckDirWritable 检查目录是否可写
func CheckDirWritable(dir string) error {
	// 确保目录存在
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("无法创建目录 %s: %v", dir, err)
	}
	
	// 尝试创建临时文件
	testFile := filepath.Join(dir, ".write_test")
	file, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("目录 %s 不可写: %v", dir, err)
	}
	file.Close()
	
	// 删除临时文件
	os.Remove(testFile)
	return nil
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries int           // 最大重试次数
	Delay      time.Duration // 重试间隔
	Backoff    float64       // 退避因子
}

// DefaultRetryConfig 默认重试配置
var DefaultRetryConfig = RetryConfig{
	MaxRetries: 3,
	Delay:      time.Second,
	Backoff:    2.0,
}

// RetryFunc 可重试的函数类型
type RetryFunc func() error

// Retry 重试执行函数
func Retry(fn RetryFunc, config RetryConfig) error {
	var lastErr error
	delay := config.Delay
	
	for i := 0; i <= config.MaxRetries; i++ {
		if i > 0 {
			log.Printf("🔄 重试第 %d 次，延迟 %v", i, delay)
			time.Sleep(delay)
			delay = time.Duration(float64(delay) * config.Backoff)
		}
		
		if err := fn(); err != nil {
			lastErr = err
			log.Printf("⚠️ 执行失败: %v", err)
			continue
		}
		
		return nil // 成功执行
	}
	
	return fmt.Errorf("重试 %d 次后仍然失败: %v", config.MaxRetries, lastErr)
}

// SafeExecute 安全执行函数，捕获panic
func SafeExecute(fn func() error, context string) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ %s 发生panic: %v", context, r)
		}
	}()
	
	return fn()
}

// RateLimiter 简单的速率限制器
type RateLimiter struct {
	tokens   chan struct{}
	interval time.Duration
}

// NewRateLimiter 创建速率限制器
func NewRateLimiter(rate int, interval time.Duration) *RateLimiter {
	rl := &RateLimiter{
		tokens:   make(chan struct{}, rate),
		interval: interval,
	}
	
	// 初始化令牌
	for i := 0; i < rate; i++ {
		rl.tokens <- struct{}{}
	}
	
	// 定期补充令牌
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for range ticker.C {
			select {
			case rl.tokens <- struct{}{}:
			default:
				// 令牌桶已满
			}
		}
	}()
	
	return rl
}

// Wait 等待获取令牌
func (rl *RateLimiter) Wait() {
	<-rl.tokens
}

// TryWait 尝试获取令牌，不阻塞
func (rl *RateLimiter) TryWait() bool {
	select {
	case <-rl.tokens:
		return true
	default:
		return false
	}
}