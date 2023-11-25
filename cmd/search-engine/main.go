package main

import (
	"log"

	"github.com/DavidUlloa6310/WebCrawler/internal/crawl"
	"github.com/gocql/gocql"
)

func main() {

	cluster := gocql.NewCluster("127.0.0.1")
    cluster.Keyspace = "tfidf_keyspace"
    session, err := cluster.CreateSession()
    if err != nil {
        log.Fatal(err)
    }
    defer session.Close()

	crawl.IndexDocument(session, "https://en.wikipedia.org/wiki/United_States")
	crawl.IndexDocument(session, "https://en.wikipedia.org/wiki/Russia")
	crawl.IndexDocument(session, "https://en.wikipedia.org/wiki/Pineapple")
	crawl.IndexDocument(session, "https://en.wikipedia.org/wiki/Basketball")
	crawl.IndexDocument(session, "https://en.wikipedia.org/wiki/Botany")
	crawl.IndexDocument(session, "https://en.wikipedia.org/wiki/Quantum_mechanics")
}
