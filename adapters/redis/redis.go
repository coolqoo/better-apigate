// Package redis implements shared request controls. It never owns money.
package redis

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/coolqoo/better-apigate/domain/key"
	"github.com/coolqoo/better-apigate/domain/ratelimit"
	"github.com/coolqoo/better-apigate/ports"
	redis "github.com/redis/go-redis/v9"
)

type Client struct{ *redis.Client }

func Open(url string) (*Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	opts.DialTimeout = 3 * time.Second
	opts.ReadTimeout = 2 * time.Second
	opts.WriteTimeout = 2 * time.Second
	c := &Client{redis.NewClient(opts)}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = c.Ping(ctx).Err(); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

var rateScript = redis.NewScript(`
local limit=tonumber(ARGV[1]); local burst=tonumber(ARGV[2]); local ttl=tonumber(ARGV[3])
local count=tonumber(redis.call('GET',KEYS[1]) or '0')
if count>=limit+burst then return {0,0} end
count=redis.call('INCR',KEYS[1])
if count==1 then redis.call('PEXPIRE',KEYS[1],ttl) end
return {1,math.max(0,limit-count)}
`)

func (c *Client) Allow(ctx context.Context, userID string, cfg ratelimit.Config, now time.Time) (ratelimit.CheckResult, error) {
	if cfg.Limit <= 0 || cfg.Window <= 0 || cfg.BurstTokens < 0 {
		return ratelimit.CheckResult{}, fmt.Errorf("invalid rate limit configuration")
	}
	// Redis time keeps window boundaries consistent across gateway clocks.
	serverNow, err := c.Time(ctx).Result()
	if err != nil {
		return ratelimit.CheckResult{}, err
	}
	reset := serverNow.Truncate(cfg.Window).Add(cfg.Window)
	k := fmt.Sprintf("apigate:v2:rate:%s:%d", userID, reset.UnixMilli())
	vals, err := rateScript.Run(ctx, c.Client, []string{k}, cfg.Limit, cfg.BurstTokens, max(reset.Sub(serverNow).Milliseconds(), 1)).Int64Slice()
	if err != nil {
		return ratelimit.CheckResult{}, err
	}
	if len(vals) != 2 {
		return ratelimit.CheckResult{}, fmt.Errorf("invalid limiter response")
	}
	return ratelimit.CheckResult{Allowed: vals[0] == 1, Remaining: int(vals[1]), ResetAt: reset, Reason: ratelimit.ReasonLimitExceeded}, nil
}

type DigestStore interface {
	GetByDigest(context.Context, []byte) (key.Key, error)
}
type KeyStore struct {
	ports.KeyStore
	source DigestStore
	client *Client
}

func CachedKeys(store ports.KeyStore, source DigestStore, c *Client) *KeyStore {
	return &KeyStore{KeyStore: store, source: source, client: c}
}
func (s *KeyStore) GetByDigest(ctx context.Context, digest []byte) (key.Key, error) {
	name := "apigate:v2:key:" + hex.EncodeToString(digest)
	b, err := s.client.Get(ctx, name).Bytes()
	if err == nil {
		var k key.Key
		if json.Unmarshal(b, &k) == nil {
			return k, nil
		}
	}
	if err != nil && err != redis.Nil {
		return key.Key{}, err
	}
	k, err := s.source.GetByDigest(ctx, digest)
	if err != nil {
		return k, err
	}
	data, err := json.Marshal(k)
	if err != nil {
		return k, err
	}
	if err = s.client.Set(ctx, name, data, 30*time.Second).Err(); err != nil {
		return key.Key{}, err
	}
	return k, nil
}
