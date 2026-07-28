package repository

import (
	"crypto/tls"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

var (
	embeddedRedisMu     sync.Mutex
	embeddedRedisServer *miniredis.Miniredis
)

// InitRedis 初始化 Redis 客户端
//
// 性能优化说明：
// 原实现使用 go-redis 默认配置，未设置连接池和超时参数：
// 1. 默认连接池大小可能不足以支撑高并发
// 2. 无超时控制可能导致慢操作阻塞
//
// 新实现支持可配置的连接池和超时参数：
// 1. PoolSize: 控制最大并发连接数（默认 128）
// 2. MinIdleConns: 保持最小空闲连接，减少冷启动延迟（默认 10）
// 3. DialTimeout/ReadTimeout/WriteTimeout: 精确控制各阶段超时
//
// DIY / embed 模式：
// 当 cfg.Redis.Embedded 或 cfg.IsDIY() 为 true 时，启动进程内 miniredis，
// 无需外部 Redis 服务。
func InitRedis(cfg *config.Config) *redis.Client {
	if shouldUseEmbeddedRedis(cfg) {
		addr, err := startEmbeddedRedis()
		if err != nil {
			// Fall through would panic later on first command; fail loudly at boot.
			log.Fatalf("failed to start embedded redis: %v", err)
		}
		log.Printf("embedded redis listening on %s (in-process, DIY mode)", addr)
		client := redis.NewClient(&redis.Options{
			Addr:         addr,
			DB:           0,
			DialTimeout:  time.Duration(cfg.Redis.DialTimeoutSeconds) * time.Second,
			ReadTimeout:  time.Duration(cfg.Redis.ReadTimeoutSeconds) * time.Second,
			WriteTimeout: time.Duration(cfg.Redis.WriteTimeoutSeconds) * time.Second,
			PoolSize:     cfg.Redis.PoolSize,
			MinIdleConns: cfg.Redis.MinIdleConns,
		})
		if cfg.Server.EnableServerTiming {
			client.AddHook(serverTimingRedisHook{})
		}
		return client
	}

	client := redis.NewClient(buildRedisOptions(cfg))
	if cfg.Server.EnableServerTiming {
		client.AddHook(serverTimingRedisHook{})
	}
	return client
}

func shouldUseEmbeddedRedis(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	if cfg.Redis.Embedded {
		return true
	}
	return cfg.IsDIY()
}

// startEmbeddedRedis starts a process-local miniredis once and returns its address.
func startEmbeddedRedis() (string, error) {
	embeddedRedisMu.Lock()
	defer embeddedRedisMu.Unlock()
	if embeddedRedisServer != nil {
		return embeddedRedisServer.Addr(), nil
	}
	mr, err := miniredis.Run()
	if err != nil {
		return "", fmt.Errorf("miniredis.Run: %w", err)
	}
	embeddedRedisServer = mr
	return mr.Addr(), nil
}

// CloseEmbeddedRedis stops the in-process Redis server if it was started.
// Safe to call multiple times or when embedded mode was not used.
func CloseEmbeddedRedis() {
	embeddedRedisMu.Lock()
	defer embeddedRedisMu.Unlock()
	if embeddedRedisServer == nil {
		return
	}
	embeddedRedisServer.Close()
	embeddedRedisServer = nil
}

// buildRedisOptions 构建 Redis 连接选项
// 从配置文件读取连接池和超时参数，支持生产环境调优
func buildRedisOptions(cfg *config.Config) *redis.Options {
	opts := &redis.Options{
		Addr:         cfg.Redis.Address(),
		Username:     cfg.Redis.Username,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		DialTimeout:  time.Duration(cfg.Redis.DialTimeoutSeconds) * time.Second,  // 建连超时
		ReadTimeout:  time.Duration(cfg.Redis.ReadTimeoutSeconds) * time.Second,  // 读取超时
		WriteTimeout: time.Duration(cfg.Redis.WriteTimeoutSeconds) * time.Second, // 写入超时
		PoolSize:     cfg.Redis.PoolSize,                                         // 连接池大小
		MinIdleConns: cfg.Redis.MinIdleConns,                                     // 最小空闲连接
	}

	if cfg.Redis.EnableTLS {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: cfg.Redis.Host,
		}
	}

	return opts
}
