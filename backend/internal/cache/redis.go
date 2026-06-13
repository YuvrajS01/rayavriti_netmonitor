package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
}

type RedisConfig struct {
	URL          string
	PoolSize     int
	MinIdleConns int
}

func NewRedis(cfg RedisConfig) (*Redis, error) {
	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}
	if cfg.PoolSize > 0 {
		opts.PoolSize = cfg.PoolSize
	}
	if cfg.MinIdleConns > 0 {
		opts.MinIdleConns = cfg.MinIdleConns
	}

	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &Redis{client: client}, nil
}

func (r *Redis) Close() error {
	return r.client.Close()
}

func (r *Redis) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *Redis) Client() *redis.Client {
	return r.client
}

// Get fetches a key and unmarshals JSON into dst. Returns false if key missing.
func (r *Redis) Get(ctx context.Context, key string, dst any) (bool, error) {
	data, err := r.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return false, fmt.Errorf("unmarshal cache value: %w", err)
	}
	return true, nil
}

// Set marshals val as JSON and stores with TTL.
func (r *Redis) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	data, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("marshal cache value: %w", err)
	}
	return r.client.Set(ctx, key, data, ttl).Err()
}

// Del deletes one or more keys.
func (r *Redis) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Exists checks if a key exists.
func (r *Redis) Exists(ctx context.Context, key string) (bool, error) {
	n, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
