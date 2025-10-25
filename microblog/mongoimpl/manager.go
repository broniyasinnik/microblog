package mongoimpl

import (
	"context"
	"errors"
	"fmt"
	"micro-blog/microblog"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const collName = "posts"

type MongoManager struct {
	posts  *mongo.Collection
	client *mongo.Client
}

func ensureIndexes(ctx context.Context, collection *mongo.Collection) {
	indexModels := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "author_id", Value: 1}, {Key: "_id", Value: -1}},
		},
	}
	opts := options.CreateIndexes().SetMaxTime(10 * time.Second)

	_, err := collection.Indexes().CreateMany(ctx, indexModels, opts)
	if err != nil {
		panic(fmt.Errorf("failed to ensure indexes %w", err))
	}
}

func NewMongoManager(mongoURL string, dbName string) *MongoManager {
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURL))
	if err != nil {
		panic(err)
	}

	collection := client.Database(dbName).Collection(collName)
	ensureIndexes(ctx, collection)

	return &MongoManager{
		posts:  collection,
		client: client,
	}
}

func (m *MongoManager) IsReady(ctx context.Context) bool {
	// Ping the database
	if err := m.client.Ping(ctx, nil); err != nil {
		return false
	}
	return true
}

func (m *MongoManager) AddPost(ctx context.Context, userId string, text string) (microblog.UserPost, error) {
	usrPost := microblog.UserPost{AuthorId: userId, Text: text, CreatedAt: time.Now()}
	res, err := m.posts.InsertOne(ctx, usrPost)
	if err != nil {
		return microblog.UserPost{}, fmt.Errorf("something went wrong - %w", microblog.ErrStorage)
	}
	id := res.InsertedID.(primitive.ObjectID)
	err = m.posts.FindOne(ctx, bson.M{"_id": id}).Decode(&usrPost)
	usrPost.PostId = id.Hex()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return microblog.UserPost{}, microblog.ErrNotFound
	}
	return usrPost, nil
}

func (m *MongoManager) GetPost(ctx context.Context, postID string) (microblog.UserPost, error) {
	var resPost microblog.UserPost
	objID, _ := primitive.ObjectIDFromHex(postID)
	err := m.posts.FindOne(
		ctx, bson.M{"_id": objID},
	).Decode(&resPost)
	if err != nil {
		return resPost, err
	}
	resPost.PostId = objID.Hex()
	return resPost, nil
}

func (m *MongoManager) GetPostsInPage(ctx context.Context, userId string, token string, size uint8) ([]microblog.UserPost, string, error) {

	var posts []microblog.UserPost
	var filter bson.M
	if token != "" {
		objID, _ := primitive.ObjectIDFromHex(token)
		filter = bson.M{"author_id": userId, "_id": bson.M{"$lt": objID}}
	} else {
		filter = bson.M{"author_id": userId}
	}
	cursor, err := m.posts.Find(
		ctx,
		filter,
		options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}),
		options.Find().SetLimit(int64(size)),
	)
	if err != nil {
		return posts, "", err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var resPost microblog.UserPost
		err = cursor.Decode(&resPost)
		if err != nil {
			return posts, "", err
		}

		var doc bson.M
		err = cursor.Decode(&doc)
		if err != nil {
			return posts, "", err
		}
		id := doc["_id"].(primitive.ObjectID)
		resPost.PostId = id.Hex()
		posts = append(posts, resPost)
	}

	return posts, posts[len(posts)-1].PostId, nil
}

func (m *MongoManager) ModifyPost(ctx context.Context, postID string, text string) (microblog.UserPost, error) {
	var updated microblog.UserPost
	objID, _ := primitive.ObjectIDFromHex(postID)

	update := bson.M{
		"$set": bson.M{
			"text":             text,
			"last_modified_at": time.Now(),
		},
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	err := m.posts.FindOneAndUpdate(ctx, bson.M{"_id": objID}, update, opts).Decode(&updated)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return microblog.UserPost{}, microblog.ErrNotFound
		}
		return microblog.UserPost{}, fmt.Errorf("something went wrong - %w", microblog.ErrStorage)
	}
	updated.PostId = objID.Hex()
	return updated, nil
}
