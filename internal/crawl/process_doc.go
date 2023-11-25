package crawl

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"regexp"
	"strings"

	"github.com/gocql/gocql"
	"golang.org/x/net/html"
)

type DocumentInfo struct {
	doc_id gocql.UUID
	title string
	url string
	content *[]byte
	links *[]string
	wordCount *map[string]int
	totalTokens int
	lastModified string
}

const MaxBatchSize = 10

func IndexDocument(session *gocql.Session, url string) {
	// var documentInfo DocumentInfo;
	var documentID gocql.UUID; 
	var lastModified string;
	docExistsErr := session.Query("SELECT doc_id, last_updated FROM documents WHERE url = ? LIMIT 1", url).Scan(&documentID, &lastModified);

	if docExistsErr != nil && !errors.Is(docExistsErr, gocql.ErrNotFound) {
		log.Fatalf("Error retrieving when document was last updated: %s\n", docExistsErr.Error());
	}

	if  hasUpdated, newDate, checkUpdateError := checkPageUpdate(url, lastModified); checkUpdateError == nil {
		if !hasUpdated {
			return;
		} else if newDate != "" {
			lastModified = newDate
		} 
	}

	content := collectDocumentContent(url)
	var currDoc DocumentInfo
	currDoc.url = url
	currDoc.content = content
	currDoc.lastModified = lastModified

	if processErr := processDocumentContent(&currDoc); processErr != nil {
		log.Fatalf("Error processing document content: %s\n", processErr.Error())
	}

	if updateDocErr := updateDocumentTable(session, currDoc); updateDocErr != nil {
		log.Fatalf("Error updating documents table: %s\n", updateDocErr.Error())
	}
	
	if updateTermFreqErr := updateTermFrequenciesTable(session, currDoc); updateTermFreqErr != nil {
		log.Fatalf("Error updating term frequencies: %s\n", updateTermFreqErr.Error())
	}

	if updateDocFreqErr := updateDocumentFrequencyTable(session, currDoc); updateDocFreqErr != nil {
		log.Fatalf("Error updating document frequency: %s\n", updateDocFreqErr.Error())
	}

	if updateTFIDFErr := updateTFIDFTable(session, currDoc); updateTFIDFErr != nil {
		log.Fatalf("Error updating tf-idf table: %s\n", updateTFIDFErr.Error())
	}

}

func updateDocumentTable(session *gocql.Session, documentInfo DocumentInfo) error {
	updateDocsQuery := session.Query(`
	INSERT INTO documents 
	(doc_id, title, url, last_updated, content, links, total_tokens) 
	VALUES (?, ?, ?, ?, ?, ?, ?)`, 
	documentInfo.doc_id, documentInfo.title, documentInfo.url, documentInfo.lastModified, documentInfo.content, documentInfo.links, documentInfo.totalTokens);

	if updateDocsErr := updateDocsQuery.Exec(); updateDocsErr != nil {
		return updateDocsErr
	}

	return nil
}


func updateTermFrequenciesTable(session *gocql.Session, currDoc DocumentInfo) error {

	if currDoc.wordCount == nil {
		return errors.New("wordCount was not passed into updateTermFrequenciesTable")
	}

	batch := session.NewBatch(gocql.LoggedBatch)
	batchSize := 0
	for term, count := range *currDoc.wordCount {
		batch.Query(`
		INSERT INTO term_frequency
		(term, frequency, doc_id)
		VALUES (?, ?, ?)
		`, term, count, currDoc.doc_id)
		batchSize += 1

		if batchSize >= MaxBatchSize {
			if err := session.ExecuteBatch(batch); err != nil {
				return err
			}
			batch = session.NewBatch(gocql.LoggedBatch)
			batchSize = 0
		}
	}

	if batchSize > 0 {
		if err := session.ExecuteBatch(batch); err != nil {
			return err
		}
	}

	return nil;
}

func updateDocumentFrequencyTable(session *gocql.Session, currDoc DocumentInfo) error {
	batch := session.NewBatch(gocql.LoggedBatch)
	batchSize := 0
	for term := range *currDoc.wordCount {

		batch.Query(`
		INSERT INTO document_frequency
		 (term, doc_count)
		VALUES (?, ?)
		`, term, 1)

		if batchSize >= MaxBatchSize {
			if err := session.ExecuteBatch(batch); err != nil {
				return err
			}
			batch = session.NewBatch(gocql.LoggedBatch)
			batchSize = 0
		}
	}

	if batchSize > 0 {
		if err := session.ExecuteBatch(batch); err != nil {
			return err
		}
	}

	return nil
}

func updateTFIDFTable(session *gocql.Session, currDoc DocumentInfo) error {

    totalDocs, docCountErr := getDocumentCount(session)
    if docCountErr != nil {
        return docCountErr
    }

    for term, count := range *currDoc.wordCount {
        var docFrequency int
        docFreqErr := session.Query(`
        SELECT doc_count 
        FROM document_frequency 
        WHERE term = ?
        `, term).Scan(&docFrequency)

        if errors.Is(docFreqErr, gocql.ErrNotFound) {
            docFrequency = 1
        } else if docFreqErr != nil {
            return docFreqErr
        }

        idf := calcIDF(float64(totalDocs), float64(docFrequency))
        tf := float64(count) / float64(currDoc.totalTokens)

        if err := session.Query(`
        DELETE FROM tfidf_scores
        WHERE term = ? AND doc_id = ? IF EXISTS
        `, term, currDoc.doc_id).Exec(); err != nil {
            return err
        }

        if err := session.Query(`
        INSERT INTO tfidf_scores (term, doc_id, tfidf_score) 
        VALUES (?, ?, ?)	
        `, term, currDoc.doc_id, tf*idf).Exec(); err != nil {
            return err
        }
    }

    return nil
}

// ================
// HELPER FUNCTIONS
// ================

func calcIDF(totalDocs float64 , docFrequency float64) float64 {
	return 1 + math.Log2(totalDocs / docFrequency)
}

func collectDocumentContent(url string) *[]byte {
	res, err := http.Get(url)
	if err != nil {
		log.Fatalf("Could not retrieve document %s, error: %v", url, err);
	}
	defer res.Body.Close();
	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatalf("Could not process GET body for url %s, error: %v", url, err);
	}
	return &body;
}

func cleanText(input *string) *string {
	// Remove all punctuation
	re := regexp.MustCompile(`\p{P}`)
	output := re.ReplaceAllString(*input, "")

	// Replace all whitespace sequences with a single space
	re = regexp.MustCompile(`\s+`)
	output = re.ReplaceAllString(output, " ")

	// Trim leading and trailing spaces
	output = strings.TrimSpace(output)

	return &output 
}

func getDocumentCount(session *gocql.Session) (int, error) {
	var totalDocs int
	getTotalDocsErr :=session.Query(`
	SELECT 
	 COUNT(*)
	FROM documents
	LIMIT 1`).Scan(&totalDocs);

	if getTotalDocsErr != nil {
		return -1, getTotalDocsErr	
	} 

	return totalDocs, nil;
}

func checkPageUpdate(url string, lastModified string) (bool, string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false, "", err
	}

	if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, resp.Header.Get("Last-Modified"), nil
	case http.StatusNotModified:
		return false, lastModified, nil
	default:
		return false, "", fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}
}

func processDocumentContent(currDoc *DocumentInfo) error {
	if currDoc.content == nil {
		return errors.New("DocumentInfo content is nil, cannot parse information")
	}

	tkn := html.NewTokenizer(bytes.NewReader(*currDoc.content))

    var links []string
	
	tokenCount := make(map[string]int);
	totalTokens := 0

    for {
        tt := tkn.Next()

        switch {
		case tt == html.ErrorToken:
			currDoc.totalTokens = totalTokens;
			currDoc.wordCount = &tokenCount;
			currDoc.links = &links;
			return nil

		case tt == html.StartTagToken:
			t := tkn.Token()
			for _, a := range t.Attr {
				if a.Key == "href" {
					links = append(links, a.Val);
				}
			}

			if t.Data == "title" {
				tokenType := tkn.Next()
				if tokenType == html.TextToken {
					// Get the text token
					textToken := tkn.Token()
					// Clean the text and assign it to the title field
					cleanedTitle := cleanText(&textToken.Data)
					currDoc.title = *cleanedTitle
				}
			}

		case tt == html.TextToken:
			t := tkn.Token()
			cleanData := cleanText(&t.Data);

			if *cleanData == "" {
				continue;
			}

			words := strings.Split(*cleanData, " ")
			for _, word := range words {
				word = strings.ReplaceAll(word, " ", "")
				if word != "" {
					tokenCount[strings.ToLower(word)]++	
					totalTokens += 1
				}
			}
		}
	}
}