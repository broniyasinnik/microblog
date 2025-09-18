package main

import (
	"log"
	"micro-blog/httpapi"
	"micro-blog/microblog"
)

func main() {
	manager := microblog.NewInMemoryManager()
	srv := httpapi.NewServer(manager)
	err := srv.ListenAndServe()
	log.Fatal(err)
}
