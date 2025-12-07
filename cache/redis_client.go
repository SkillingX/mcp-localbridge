package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/SkillingX/mcp-localbridge/config"
)

// RedisClient wraps a Redis client with convenience methods
type RedisClient struct {
	client *redis.Client
	name   string
	config config.RedisInstanceConfig
}

// NewRedisClient creates a new Redis client
func NewRedisClient(cfg config.RedisInstanceConfig) (*RedisClient, error) {
	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Address(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  time.Duration(cfg.DialTimeout) * time.Second,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis %s: %w", cfg.Name, err)
	}

	return &RedisClient{
		client: client,
		name:   cfg.Name,
		config: cfg,
	}, nil
}

// Get retrieves a value by key
func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	result, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil // Key does not exist
	}
	return result, err
}

// Set sets a key-value pair with optional expiration
func (r *RedisClient) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

// Del deletes one or more keys
func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Exists checks if keys exist
func (r *RedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	return r.client.Exists(ctx, keys...).Result()
}

// Scan iterates keys matching a pattern
func (r *RedisClient) Scan(ctx context.Context, cursor uint64, match string, count int64) ([]string, uint64, error) {
	keys, newCursor, err := r.client.Scan(ctx, cursor, match, count).Result()
	return keys, newCursor, err
}

// Keys returns all keys matching a pattern (use with caution on large datasets)
func (r *RedisClient) Keys(ctx context.Context, pattern string) ([]string, error) {
	return r.client.Keys(ctx, pattern).Result()
}

// TTL returns the remaining time to live of a key
func (r *RedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

// Expire sets a timeout on a key
func (r *RedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return r.client.Expire(ctx, key, expiration).Err()
}

// Incr increments the integer value of a key
func (r *RedisClient) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

// Decr decrements the integer value of a key
func (r *RedisClient) Decr(ctx context.Context, key string) (int64, error) {
	return r.client.Decr(ctx, key).Result()
}

// HGet gets a field value from a hash
func (r *RedisClient) HGet(ctx context.Context, key, field string) (string, error) {
	result, err := r.client.HGet(ctx, key, field).Result()
	if err == redis.Nil {
		return "", nil
	}
	return result, err
}

// HSet sets a field value in a hash
func (r *RedisClient) HSet(ctx context.Context, key string, values ...any) error {
	return r.client.HSet(ctx, key, values...).Err()
}

// HGetAll gets all fields and values from a hash
func (r *RedisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return r.client.HGetAll(ctx, key).Result()
}

// HDel deletes fields from a hash
func (r *RedisClient) HDel(ctx context.Context, key string, fields ...string) error {
	return r.client.HDel(ctx, key, fields...).Err()
}

// LPush prepends values to a list
func (r *RedisClient) LPush(ctx context.Context, key string, values ...any) error {
	return r.client.LPush(ctx, key, values...).Err()
}

// RPush appends values to a list
func (r *RedisClient) RPush(ctx context.Context, key string, values ...any) error {
	return r.client.RPush(ctx, key, values...).Err()
}

// LRange gets a range of elements from a list
func (r *RedisClient) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.LRange(ctx, key, start, stop).Result()
}

// SAdd adds members to a set
func (r *RedisClient) SAdd(ctx context.Context, key string, members ...any) error {
	return r.client.SAdd(ctx, key, members...).Err()
}

// SMembers gets all members of a set
func (r *RedisClient) SMembers(ctx context.Context, key string) ([]string, error) {
	return r.client.SMembers(ctx, key).Result()
}

// SIsMember checks if a value is a member of a set
func (r *RedisClient) SIsMember(ctx context.Context, key string, member any) (bool, error) {
	return r.client.SIsMember(ctx, key, member).Result()
}

// ZAdd adds members to a sorted set
func (r *RedisClient) ZAdd(ctx context.Context, key string, members ...redis.Z) error {
	return r.client.ZAdd(ctx, key, members...).Err()
}

// ZRange gets a range of members from a sorted set by index
func (r *RedisClient) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.ZRange(ctx, key, start, stop).Result()
}

// ZRangeWithScores gets a range of members with scores from a sorted set
func (r *RedisClient) ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]redis.Z, error) {
	return r.client.ZRangeWithScores(ctx, key, start, stop).Result()
}

// Ping tests the connection to Redis
func (r *RedisClient) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// GetName returns the client name
func (r *RedisClient) GetName() string {
	return r.name
}

// GetClient returns the underlying Redis client for advanced operations
func (r *RedisClient) GetClient() *redis.Client {
	return r.client
}
