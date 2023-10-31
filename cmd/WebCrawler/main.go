package main

import (
	"log"

	"github.com/DavidUlloa6310/WebCrawler/internal/processdoc"
	"github.com/gocql/gocql"
)

func main() {

	cluster := gocql.NewCluster("127.0.0.1")
    cluster.Keyspace = "indexing"
    session, err := cluster.CreateSession()
    if err != nil {
        log.Fatal(err)
    }
    defer session.Close()

	processdoc.IndexDocument(session, "https://apnews.com/")
}
