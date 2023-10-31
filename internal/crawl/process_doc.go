package crawl

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

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
	alreadyStored bool 
}

func IndexDocument(session *gocql.Session, url string) {
	var documentInfo DocumentInfo;
	documentInfo.url = url;
	docExistsErr := session.Query("SELECT doc_id, last_updated FROM indexing.Documents WHERE url = ? LIMIT 1", url).Scan(&documentInfo.doc_id, &documentInfo.lastModified);

	if docExistsErr == nil {
		// No error finding document; It already exists
		documentInfo.alreadyStored = true;
	} else if errors.Is(docExistsErr, gocql.ErrNotFound) {
		// Error is that document was not found
		documentInfo.alreadyStored = false
		documentInfo.lastModified = "";
		documentInfo.doc_id = gocql.UUIDFromTime(time.Now())
	} else {
		// Panic if any other type of error
		log.Fatal(docExistsErr);
	}

	if documentInfo.alreadyStored {
		hasUpdated, newDate, checkUpdateError := checkPageUpdate(url, documentInfo.lastModified)

		if checkUpdateError == nil && !hasUpdated {
			return;
		} else if checkUpdateError == nil && newDate != "" {
			documentInfo.lastModified = newDate
		} 
	}

	documentInfo.content = collectDocumentContent(url)
	processDocumentContent(&documentInfo)
	updateDocumentTable(session, &documentInfo)

	// totalDocs, totalDocsErr := getDocumentCount(session)
	// if totalDocsErr != nil {
	// 	log.Fatal(totalDocsErr)
	// }

	// updateTermsTable(session, documentInfo.wordCount, documentInfo.totalTokens, totalDocs)
	// updateTermFrequenciesTable(session, documentInfo.doc_id, documentInfo.wordCount, documentInfo.totalTokens, totalDocs)
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
	FROM indexing.Documents
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

func processDocumentContent(documentInfo *DocumentInfo) error {
	content := *documentInfo.content;
	if content == nil {
		return errors.New("DocumentInfo content is nil, cannot parse information")
	}

	tkn := html.NewTokenizer(bytes.NewReader(content))

    var links []string
	
	tokenCount := make(map[string]int);
	totalTokens := 0

    for {
        tt := tkn.Next()

        switch {
		case tt == html.ErrorToken:
			documentInfo.totalTokens = totalTokens;
			documentInfo.wordCount = &tokenCount;
			documentInfo.links = &links;
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
					documentInfo.title = *cleanedTitle
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

func updateDocumentTable(session *gocql.Session, documentInfo *DocumentInfo) {
	var updateDocsQuery *gocql.Query; 
	if (documentInfo.alreadyStored) {
		updateDocsQuery = session.Query(`
		UPDATE indexing.Documents SET
		 title = ?, 
		 last_updated = ?,
		 content = ?,
		 links = ?, 
		 total_tokens = ? 
		WHERE doc_id = ?`,
		documentInfo.title, documentInfo.lastModified, string(*documentInfo.content), documentInfo.links, documentInfo.totalTokens, documentInfo.doc_id);
	} else {
		updateDocsQuery = session.Query(`
		INSERT INTO indexing.Documents 
		 (doc_id, title, url, last_updated, content, links, total_tokens) 
		VALUES (?, ?, ?, ?, ?, ?, ?)`, 
		documentInfo.doc_id, documentInfo.title, documentInfo.url, documentInfo.lastModified, documentInfo.content, documentInfo.links, documentInfo.totalTokens);
	}

	if updateDocsErr := updateDocsQuery.Exec(); updateDocsErr != nil {
		log.Fatal(updateDocsErr);
	}
}

func updateTermFrequenciesTable(session *gocql.Session, doc_id gocql.UUID, wordCount *map[string]int, totalTokens int, totalDocs int) {

	for term, count := range *wordCount{

		updateDocFreqErr := session.Query(
			`
			UPDATE indexing.TermFrequencies SET
			 doc_frequency = doc_frequency + 1,
			WHERE term = ? 
			`, term).Exec()

		if updateDocFreqErr != nil && !errors.Is(updateDocFreqErr, gocql.ErrNotFound) {
			log.Fatal(updateDocFreqErr);
		}

		termFrequency := count / totalTokens
		updateTFErr := session.Query(
			`
			UPDATE indexing.TermFrequencies SET
			 term_occurrences = ?,
			 total_tokens = ?,
			 tf_idf = ? * (1 + LOG(? / doc_frequency))
			WHERE doc_id = ? AND term = ? 
			`, count, totalTokens, termFrequency, totalDocs, doc_id, term).Exec()
		
		if errors.Is(updateTFErr, gocql.ErrNotFound) {
			insertTFErr := session.Query(`
			INSERT INTO indexing.TermFrequencies
			 (term, doc_id, term_occurences, tf_idf, doc_frequency, total_docs, total_tokens)
			VALUES (?, ?, ?, ?, 1) 
			`, doc_id, count, termFrequency, 1, totalDocs, totalTokens)

			if insertTFErr != nil {
				log.Fatal(insertTFErr)
			}
		} 
	}
}

func updateTermsTable(session *gocql.Session, wordCount *map[string]int, totalTokens int, totalDocs int) {

	for term := range *wordCount {

		updateTermsErr := session.Query(
		`UPDATE indexing.Terms SET 
		 doc_frequency = doc_frequency + 1, 
		 inv_doc_frequency = 1 + LOG(? / (doc_frequency + 1)) 
		WHERE term = ? 
		LIMIT 1`, totalDocs, term).Exec()

		if errors.Is(updateTermsErr, gocql.ErrNotFound) {
			insertTermErr := session.Query(`
			INSERT INTO 
			 indexing.Terms (term, doc_frequency, inv_doc_frequency, total_docs) 
			VALUES (?, 1, 1, ?)`, term, totalDocs).Exec()
			if insertTermErr != nil {
				continue;
			}
		}
	}
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