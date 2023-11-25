CREATE KEYSPACE IF NOT EXISTS tfidf_keyspace WITH REPLICATION = {'class': 'SimpleStrategy', 'replication_factor': 1};

USE tfidf_keyspace;

CREATE TABLE IF NOT EXISTS tfidf_keyspace.documents (
    doc_id UUID PRIMARY KEY,
    title TEXT,
    content TEXT,
    url TEXT,
    last_updated TEXT,
    links LIST<TEXT>,
    total_tokens INT
);

CREATE INDEX ON tfidf_keyspace.documents (url);

CREATE TABLE IF NOT EXISTS tfidf_keyspace.term_frequency (
    term TEXT,
    doc_id UUID,
    frequency INT,
    PRIMARY KEY (term, doc_id)
);

CREATE TABLE IF NOT EXISTS tfidf_keyspace.document_frequency (
    term TEXT PRIMARY KEY,
    doc_count COUNTER 
);

CREATE TABLE IF NOT EXISTS tfidf_keyspace.tfidf_scores (
    term TEXT,
    doc_id UUID,
    tfidf_score DOUBLE,
    PRIMARY KEY ((term), doc_id)
) WITH CLUSTERING ORDER BY (doc_id ASC);

-- CREATE MATERIALIZED VIEW tfidf_keyspace.tfidf_scores_by_score AS
--     SELECT * FROM tfidf_keyspace.tfidf_scores
--     WHERE term IS NOT NULL AND doc_id IS NOT NULL AND tfidf_score IS NOT NULL
--     PRIMARY KEY ((term), tfidf_score, doc_id)
--     WITH CLUSTERING ORDER BY (tfidf_score DESC, doc_id ASC);