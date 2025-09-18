package microblog

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"sync"
	"time"
)

type PostIdsList []string
type UserPost struct {
	PostId    string
	Text      string
	AuthorId  string
	CreatedAt time.Time
}

type Manager interface {
	AddPost(userId string, post string) (UserPost, error)
	GetPost(userId string) (UserPost, error)
	GetPostsInPage(userId string, token string, size uint8) ([]UserPost, string, error)
}

type InMemoryManager struct {
	mu        sync.RWMutex
	userPosts map[string]PostIdsList
	allPosts  map[string]UserPost
}

func NewInMemoryManager() *InMemoryManager {
	return &InMemoryManager{
		userPosts: make(map[string]PostIdsList),
		allPosts:  make(map[string]UserPost),
	}
}

func createPostID(createTime int) string {
	randomPart := rand.Intn(1_000_000)
	raw := strconv.FormatInt(int64(createTime), 10) + ":" + strconv.Itoa(randomPart)
	postID := base64.RawURLEncoding.EncodeToString([]byte(raw))
	return postID
}

func (manager *InMemoryManager) AddPost(userId string, text string) (UserPost, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	postsList, _ := manager.userPosts[userId]
	createTime := time.Now().UTC()
	postId := createPostID(createTime.Second())
	post := UserPost{postId, text, userId, createTime}
	manager.userPosts[userId] = append(postsList, postId)
	manager.allPosts[post.PostId] = post
	return post, nil
}

func (manager *InMemoryManager) GetPost(postId string) (UserPost, error) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	post, ok := manager.allPosts[postId]
	if !ok {
		return UserPost{}, errors.New("post not found")
	}
	return post, nil
}

func (manager *InMemoryManager) GetPostsInPage(userId string, token string, size uint8) ([]UserPost, string, error) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	start := 0
	if token != "" {
		decoded, err := base64.StdEncoding.DecodeString(token)
		if err == nil {
			fmt.Sscanf(string(decoded), "%d", &start)
		} else {
			return nil, "", err
		}
	}
	posts, ok := manager.userPosts[userId]
	if !ok {
		return nil, "", errors.New("user not found")
	}
	if start >= len(posts) {
		return nil, "", errors.New("no posts available for this page")
	}
	sort.Slice(posts, func(i, j int) bool {
		return manager.allPosts[posts[i]].CreatedAt.After(manager.allPosts[posts[j]].CreatedAt)
	})

	end := start + int(size)
	if end > len(posts) {
		end = len(posts)
	}
	var nextToken string
	var result []UserPost
	for _, post := range posts[start:end] {
		result = append(result, manager.allPosts[post])
	}
	if end < len(posts) {
		nextToken = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", end)))
	}
	return result, nextToken, nil

}
