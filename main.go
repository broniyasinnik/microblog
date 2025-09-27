package main

import (
	"log"
	"micro-blog/httpapi"
	"micro-blog/microblog"
	"micro-blog/microblog/inmemoryimpl"
	"micro-blog/microblog/mongoimpl"
	"os"
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
	}
	srv := httpapi.NewServer(manager)
	err := srv.ListenAndServe()
	log.Fatal(err)
}
