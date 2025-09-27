package microblog

import (
	"context"
	"time"
)

type UserPost struct {
	PostId    string    `bson:"-"`
	Text      string    `bson:"text"`
	AuthorId  string    `bson:"author_id"`
	CreatedAt time.Time `bson:"created_at"`
}

type Manager interface {
	AddPost(ctx context.Context, userId string, post string) (UserPost, error)
	GetPost(ctx context.Context, postID string) (UserPost, error)
	GetPostsInPage(ctx context.Context, userId string, token string, size uint8) ([]UserPost, string, error)
	IsReady(ctx context.Context) bool
}
