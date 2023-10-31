CREATE KEYSPACE IF NOT EXISTS indexing WITH REPLICATION = {'class': 'SimpleStrategy', 'replication_factor': 1};

USE indexing;

CREATE TABLE indexing.Documents (
    doc_id UUID PRIMARY KEY,
    title TEXT,
    content TEXT,
    url TEXT,
    last_updated TEXT,
    links LIST<TEXT>,
    total_tokens INT
);

CREATE INDEX ON Documents (url);

CREATE TABLE indexing.Terms (
    term TEXT PRIMARY KEY,
    doc_frequency INT,
    inv_doc_frequency DOUBLE,
);

CREATE TABLE indexing.TermFrequencies (
    term TEXT,
    doc_id UUID,
    term_occurrences INT,
    tf_idf DOUBLE,
    doc_frequency INT,
    total_tokens INT,
    PRIMARY KEY (term, doc_id)
) WITH CLUSTERING ORDER BY (doc_id ASC);