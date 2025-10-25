package redisimpl

import (
	"context"
	"encoding/json"
	"micro-blog/microblog"
	"time"

	"github.com/redis/go-redis/v9"
)

const postCacheTTL = 10 * time.Minute

func cacheKeyForPost(id string) string { return "post:" + id }

func NewRedisManager(
	client *redis.Client,
	persistentManager microblog.Manager,
) *RedisManager {

	return &RedisManager{
		client:            client,
		persistentManager: persistentManager,
	}
}

type RedisManager struct {
	client            *redis.Client
	persistentManager microblog.Manager
}

// AddPost writes through to persistent storage and updates the cache.
func (r RedisManager) AddPost(ctx context.Context, userId string, post string) (microblog.UserPost, error) {
	created, err := r.persistentManager.AddPost(ctx, userId, post)
	if err != nil {
		return created, err
	}
	if raw, mErr := json.Marshal(created); mErr == nil {
		_ = r.client.Set(ctx, cacheKeyForPost(created.PostId), raw, postCacheTTL).Err()
	}

	return created, nil
}

// GetPost uses read-through cache backed by persistent storage.
func (r RedisManager) GetPost(ctx context.Context, postID string) (microblog.UserPost, error) {
	var cached microblog.UserPost

	if bytes, err := r.client.Get(ctx, cacheKeyForPost(postID)).Bytes(); err == nil {
		if uErr := json.Unmarshal(bytes, &cached); uErr == nil {
			return cached, nil
		}
	}

	post, err := r.persistentManager.GetPost(ctx, postID)
	if err != nil {
		return post, err
	}
	if r.client != nil {
		if raw, mErr := json.Marshal(post); mErr == nil {
			_ = r.client.Set(ctx, cacheKeyForPost(postID), raw, postCacheTTL).Err()
		}
	}
	return post, nil
}

// GetPostsInPage delegates pagination to the persistent manager.
func (r RedisManager) GetPostsInPage(ctx context.Context, userId string, token string, size uint8) ([]microblog.UserPost, string, error) {
	return r.persistentManager.GetPostsInPage(ctx, userId, token, size)
}

// IsReady checks both Redis and the persistent manager health.
func (r RedisManager) IsReady(ctx context.Context) bool {
	if r.client == nil {
		return false
	}
	if err := r.client.Ping(ctx).Err(); err != nil {
		return false
	}
	return r.persistentManager.IsReady(ctx)
}

// ModifyPost writes through the update and refreshes the cache entry.
func (r RedisManager) ModifyPost(ctx context.Context, postID string, post string) (microblog.UserPost, error) {
	updated, err := r.persistentManager.ModifyPost(ctx, postID, post)
	if err != nil {
		return updated, err
	}
	if r.client != nil {
		if raw, mErr := json.Marshal(updated); mErr == nil {
			_ = r.client.Set(ctx, cacheKeyForPost(postID), raw, postCacheTTL).Err()
		}
	}
	return updated, nil
}
