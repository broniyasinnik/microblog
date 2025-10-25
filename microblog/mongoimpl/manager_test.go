package mongoimpl

import (
	"context"
	"fmt"
	"micro-blog/microblog"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ctx = context.Background()

func TestManager(t *testing.T) {
	suite.Run(t, &ManagerSuite{mongoAddr: "mongodb://localhost:27017", mongodbName: "test"})
}

type ManagerSuite struct {
	suite.Suite

	mongoAddr   string
	mongodbName string
	mongoClient *mongo.Client
	mongoDB     *mongo.Database
	manager     microblog.Manager
}

func (s *ManagerSuite) SetupSuite() {
	s.manager = NewMongoManager(s.mongoAddr, s.mongodbName)

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(s.mongoAddr))
	s.Require().NoError(err)
	db := mongoClient.Database(s.mongodbName)
	s.mongoClient = mongoClient
	s.mongoDB = db
}

func (s *ManagerSuite) SetupTest() {
	s.Require().NoError(s.mongoClient.Database(s.mongodbName).Drop(ctx))
}

//func (s *ManagerSuite) TearDownTest() {
//	s.Require().NoError(s.mongoClient.Database(s.mongodbName).Drop(ctx))
//}

func (s *ManagerSuite) TestAddPost() {
	usr := "broniy"
	msg := "This is my first post"
	p, err := s.manager.AddPost(ctx, usr, msg)
	s.Require().Equal(usr, p.AuthorId)
	s.Require().Equal(msg, p.Text)
	s.Require().NoError(err)
}

func (s *ManagerSuite) addNPosts(usrName string, n int) {
	for i := 1; i <= 10; i++ {
		msg := fmt.Sprintf("This is post number %d", i)
		_, err := s.manager.AddPost(ctx, usrName, msg)
		s.Require().NoError(err)
	}
}

func (s *ManagerSuite) TestGetPost() {
	usr := "broniy"
	s.addNPosts(usr, 10)

	post, err := s.manager.GetPost(ctx, usr)
	s.Require().NoError(err)
	s.Require().Equal(usr, post.AuthorId)
	s.Require().Equal("This is post number 10", post.Text)
}

func (s *ManagerSuite) TestGetPostsInPage() {
	usr := "bobi"
	s.addNPosts(usr, 10)

	token := ""
	var posts []microblog.UserPost
	var err error

	posts, token, err = s.manager.GetPostsInPage(ctx, usr, token, 5)
	s.Require().NoError(err)
	s.Require().Len(posts, 5)
	s.Require().Equal(usr, posts[0].AuthorId)
	s.Require().Equal("This is post number 10", posts[0].Text)
	s.Require().Equal("This is post number 6", posts[4].Text)

	posts, token, err = s.manager.GetPostsInPage(ctx, usr, token, 5)
	s.Require().Len(posts, 5)
	s.Require().Equal(usr, posts[0].AuthorId)
	s.Require().Equal("This is post number 5", posts[0].Text)
	s.Require().Equal(usr, posts[4].AuthorId)
	s.Require().Equal("This is post number 1", posts[4].Text)
}

func (s *ManagerSuite) TestModifyPost() {
	usr := "alice"
	msg := "Original message"
	p, err := s.manager.AddPost(ctx, usr, msg)
	s.Require().NoError(err)
	s.Require().NotEmpty(p.PostId)
	// Modify the post text
	newText := "Updated message"
	updated, err := s.manager.ModifyPost(ctx, p.PostId, newText)
	s.Require().NoError(err)
	s.Require().Equal(p.PostId, updated.PostId)
	s.Require().Equal(usr, updated.AuthorId)
	s.Require().Equal(newText, updated.Text)
	s.Require().False(updated.LastModifiedAt.IsZero())
	s.Require().True(updated.LastModifiedAt.After(p.CreatedAt))

	// Ensure it persisted in storage by fetching it again
	fetched, err := s.manager.GetPost(ctx, p.PostId)
	s.Require().NoError(err)
	s.Require().Equal(newText, fetched.Text)
	s.Require().False(fetched.LastModifiedAt.IsZero())
}
