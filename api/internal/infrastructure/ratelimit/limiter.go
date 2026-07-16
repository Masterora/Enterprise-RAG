package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"enterprise-rag/api/internal/config"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var slidingWindowScript = redis.NewScript(`
local key = KEYS[1]
local redis_time = redis.call('TIME')
local now = tonumber(redis_time[1]) * 1000 + math.floor(tonumber(redis_time[2]) / 1000)
local window = tonumber(ARGV[1])
local quota = tonumber(ARGV[2])
local member = ARGV[3]
redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
local count = redis.call('ZCARD', key)
if count >= quota then
  local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
  redis.call('PEXPIRE', key, window)
  local retry = 1
  if oldest[2] then
    retry = math.max(1, math.ceil((tonumber(oldest[2]) + window - now) / 1000))
  end
  return {0, retry}
end
redis.call('ZADD', key, now, member)
redis.call('PEXPIRE', key, window)
return {1, 0}
`)

type Decision struct {
	Allowed    bool
	RetryAfter int
}

type Limiter struct {
	client *redis.Client
	config config.RateLimitConf
}

func New(client *redis.Client, value config.RateLimitConf) *Limiter {
	return &Limiter{client: client, config: value}
}

func (l *Limiter) Allow(ctx context.Context, userID, path string) (Decision, error) {
	scope, quota, ok := l.policy(path)
	if !ok {
		return Decision{Allowed: true}, nil
	}
	window := time.Duration(l.config.PeriodSeconds) * time.Second
	key := "enterprise-rag:rate:" + scope + ":" + userID
	result, err := slidingWindowScript.Run(
		ctx,
		l.client,
		[]string{key},
		window.Milliseconds(),
		quota,
		uuid.NewString(),
	).Slice()
	if err != nil {
		return Decision{}, fmt.Errorf("run redis rate limiter: %w", err)
	}
	if len(result) != 2 {
		return Decision{}, fmt.Errorf("invalid redis rate limiter result")
	}
	allowed, err := integer(result[0])
	if err != nil {
		return Decision{}, err
	}
	retryAfter, err := integer(result[1])
	if err != nil {
		return Decision{}, err
	}
	return Decision{Allowed: allowed == 1, RetryAfter: max(int(retryAfter), 1)}, nil
}

func (l *Limiter) policy(path string) (string, int, bool) {
	switch path {
	case "/api/chat/ask", "/api/chat/stream", "/api/chat/runs/resume":
		return "chat", l.config.ChatQuota, true
	case "/api/retrieval/search":
		return "retrieval", l.config.RetrievalQuota, true
	case "/api/documents/upload":
		return "upload", l.config.UploadQuota, true
	default:
		return "", 0, false
	}
}

func integer(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse redis rate limiter result: %w", err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected redis rate limiter result type %T", value)
	}
}
