# Search-Engine (Golang)
This project implements a simple search engine in Go using TF-IDF and Vector similarity search with Postgres. The initial implementation included using Cassandra as the main database, but was switched to Postgres for flexibility. Learned:

* Golang project structure and langauge details
* Cassandra database constraints / requirements
* Information Retrieval from scratch, including web crawling, HTML parsing, TF-IDF formulation.

# What's next?
Next is to...
* use a local model to create document embeddings to further improve the accuracy of the engine
* use a graph database like Neo4j to implement ranking algorithms to supplement vector similarity and TF-IDF  
