package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/coolqoo/better-apigate/domain/settings"
	"github.com/coolqoo/better-apigate/ports"
	redis "github.com/redis/go-redis/v9"
)

// SettingsStore caches configuration only. PostgreSQL owns every mutation.
// A generation key prevents an old concurrent reader from repopulating the
// current generation after a configuration change.
type SettingsStore struct {
	ports.SettingsStore
	client    *Client
	namespace string
}

func CachedSettings(source ports.SettingsStore, client *Client, deployment string) *SettingsStore {
	digest := sha256.Sum256([]byte(deployment))
	return &SettingsStore{source, client, "apigate:v2:settings:" + hex.EncodeToString(digest[:])}
}
func (s *SettingsStore) GetAll(ctx context.Context) (settings.Settings, error) {
	generation, err := s.client.Get(ctx, s.namespace+":generation").Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}
	key := s.namespace + ":" + generation
	raw, err := s.client.Get(ctx, key).Bytes()
	if err == nil {
		var values settings.Settings
		if json.Unmarshal(raw, &values) == nil {
			return values, nil
		}
	}
	if err != nil && err != redis.Nil {
		return nil, err
	}
	values, err := s.SettingsStore.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	raw, err = json.Marshal(values)
	if err != nil {
		return nil, err
	}
	if err = s.client.Set(ctx, key, raw, 30*time.Second).Err(); err != nil {
		return nil, err
	}
	return values, nil
}
func (s *SettingsStore) Set(ctx context.Context, key, value string, encrypted bool) error {
	if err := s.SettingsStore.Set(ctx, key, value, encrypted); err != nil {
		return err
	}
	return s.client.Incr(ctx, s.namespace+":generation").Err()
}
func (s *SettingsStore) SetBatch(ctx context.Context, values settings.Settings) error {
	if err := s.SettingsStore.SetBatch(ctx, values); err != nil {
		return err
	}
	return s.client.Incr(ctx, s.namespace+":generation").Err()
}
func (s *SettingsStore) Delete(ctx context.Context, key string) error {
	if err := s.SettingsStore.Delete(ctx, key); err != nil {
		return err
	}
	return s.client.Incr(ctx, s.namespace+":generation").Err()
}
