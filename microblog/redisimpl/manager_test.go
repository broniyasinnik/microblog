package redisimpl

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"micro-blog/microblog"
	"micro-blog/microblog/inmemoryimpl"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
)

var ctx = context.Background()

func TestRedisManager(t *testing.T) {
	suite.Run(t, new(RedisManagerSuite))
}

type RedisManagerSuite struct {
	suite.Suite

	mini        *miniredis.Miniredis
	redisClient *redis.Client
	persistent  *inmemoryimpl.InMemoryManager
	cached      microblog.Manager
}

func (s *RedisManagerSuite) SetupSuite() {
	mr, err := miniredis.Run()
	s.Require().NoError(err)
	s.mini = mr
	// create redis client
	s.redisClient = redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func (s *RedisManagerSuite) TearDownSuite() {
	if s.redisClient != nil {
		_ = s.redisClient.Close()
	}
	if s.mini != nil {
		s.mini.Close()
	}
}

func (s *RedisManagerSuite) SetupTest() {
	// fresh persistent store per test
	s.persistent = inmemoryimpl.NewInMemoryManager()
	// flush redis between tests
	if s.mini != nil {
		s.mini.FlushAll()
	}
	// wrap with RedisManager
	s.cached = NewRedisManager(s.redisClient, s.persistent)
}

func (s *RedisManagerSuite) TestAddPost_WritesThroughAndCaches() {
	user := "user123"
	text := "Hello from cache"
	created, err := s.cached.AddPost(ctx, user, text)
	s.Require().NoError(err)
	s.Require().Equal(user, created.AuthorId)
	s.Require().Equal(text, created.Text)
	s.Require().NotEmpty(created.PostId)

	// ensure cached in redis
	key := cacheKeyForPost(created.PostId)
	bytes, err := s.redisClient.Get(ctx, key).Bytes()
	s.Require().NoError(err)
	var cached microblog.UserPost
	s.Require().NoError(json.Unmarshal(bytes, &cached))
	s.Require().Equal(created, cached)
}

func (s *RedisManagerSuite) TestGetPost_ReadThrough_PopulatesCache() {
	// create a post directly in persistent storage to simulate cache miss
	user := "alice"
	text := "Original"
	p, err := s.persistent.AddPost(ctx, user, text)
	s.Require().NoError(err)

	// ensure not present in cache initially
	key := cacheKeyForPost(p.PostId)
	_, err = s.redisClient.Get(ctx, key).Bytes()
	s.Require().Error(err)

	// now read via cached manager -> should fetch from persistent and populate cache
	got, err := s.cached.GetPost(ctx, p.PostId)
	s.Require().NoError(err)
	s.Require().Equal(p, got)

	bytes, err := s.redisClient.Get(ctx, key).Bytes()
	s.Require().NoError(err)
	var cached microblog.UserPost
	s.Require().NoError(json.Unmarshal(bytes, &cached))
	s.Require().Equal(p, cached)
}

func (s *RedisManagerSuite) TestModifyPost_RefreshesCache() {
	user := "bob"
	p, err := s.cached.AddPost(ctx, user, "v1")
	s.Require().NoError(err)

	// modify
	updated, err := s.cached.ModifyPost(ctx, p.PostId, "v2")
	s.Require().NoError(err)
	s.Require().Equal("v2", updated.Text)
	s.Require().False(updated.LastModifiedAt.IsZero())

	// check cache holds updated value
	key := cacheKeyForPost(p.PostId)
	bytes, err := s.redisClient.Get(ctx, key).Bytes()
	s.Require().NoError(err)
	var cached microblog.UserPost
	s.Require().NoError(json.Unmarshal(bytes, &cached))
	s.Require().Equal("v2", cached.Text)
	// LastModifiedAt should be set (allow small time skew)
	s.Require().WithinDuration(updated.LastModifiedAt, cached.LastModifiedAt, time.Second)
}

func (s *RedisManagerSuite) addNPosts(user string, n int) []microblog.UserPost {
	var posts []microblog.UserPost
	for i := 1; i <= n; i++ {
		msg := fmt.Sprintf("This is post number %d", i)
		p, err := s.cached.AddPost(ctx, user, msg)
		s.Require().NoError(err)
		posts = append(posts, p)
	}
	return posts
}

func (s *RedisManagerSuite) TestGetPostsInPage_PassThrough() {
	user := "carol"
	s.addNPosts(user, 10)

	token := ""
	posts, next, err := s.cached.GetPostsInPage(ctx, user, token, 5)
	s.Require().NoError(err)
	s.Require().Len(posts, 5)
	s.Require().Equal("This is post number 10", posts[0].Text)
	s.Require().Equal("This is post number 6", posts[4].Text)
	s.Require().NotEmpty(next)

	posts, next, err = s.cached.GetPostsInPage(ctx, user, next, 5)
	s.Require().NoError(err)
	s.Require().Len(posts, 5)
	s.Require().Equal("This is post number 5", posts[0].Text)
	s.Require().Equal("This is post number 1", posts[4].Text)
}

func (s *RedisManagerSuite) TestIsReady_TrueWhenHealthy() {
	ready := s.cached.IsReady(ctx)
	s.Require().True(ready)
}
