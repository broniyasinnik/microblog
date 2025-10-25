package main

import (
	"log"
	"micro-blog/httpapi"
	"micro-blog/microblog"
	"micro-blog/microblog/inmemoryimpl"
	"micro-blog/microblog/mongoimpl"
	"micro-blog/microblog/redisimpl"
	"os"

	"github.com/redis/go-redis/v9"
)

func main() {
	mode := os.Getenv("STORAGE_MODE")
	var manager microblog.Manager
	if mode == "inmemory" {
		manager = inmemoryimpl.NewInMemoryManager()
	} else if mode == "mongo" {
		dbName := os.Getenv("MONGO_DB_NAME")
		url := os.Getenv("MONGO_URL")
		manager = mongoimpl.NewMongoManager(url, dbName)
	} else if mode == "cached" {
		dbName := os.Getenv("MONGO_DB_NAME")
		mongoURL := os.Getenv("MONGO_URL")
		redisURL := os.Getenv("REDIS_URL")
		mongoManager := mongoimpl.NewMongoManager(mongoURL, dbName)
		redisClient := redis.NewClient(&redis.Options{Addr: redisURL})
		manager = redisimpl.NewRedisManager(redisClient, mongoManager)
	}

	srv := httpapi.NewServer(manager)
	err := srv.ListenAndServe()
	log.Fatal(err)
}
