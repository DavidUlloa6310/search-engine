package crawl

import (
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestCleanText(t *testing.T) {
    // Define test cases
    testCases := []struct {
        name     string
        input    string
        expected string
    }{
        {"Remove punctuation", "Hello, world!", "Hello world"},
        {"Replace whitespace", "Hello     world", "Hello world"},
        {"Trim spaces", " Hello world ", "Hello world"},
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Convert string to pointer
            input := tc.input
            // Call function
            result := cleanText(&input)
            // Check result
            if *result != tc.expected {
                t.Errorf("cleanText(%q) = %q; want %q", tc.input, *result, tc.expected)
            }
        })
    }
}

func TestCheckPageUpdate_OK_NewDate(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	url := "http://example.com"
	lastModified := "Mon, 02 Jan 2006 15:04:05 MST"
	newLastModified := "Tue, 03 Jan 2006 15:04:05 MST"

	httpmock.RegisterResponder("HEAD", url,
		func(req *http.Request) (*http.Response, error) {
			resp := httpmock.NewStringResponse(http.StatusOK, "")
			resp.Header.Set("Last-Modified", newLastModified)
			return resp, nil
		})

	hasUpdated, newDate, err := checkPageUpdate(url, lastModified)
	assert.Nil(t, err)
	assert.True(t, hasUpdated)
	assert.Equal(t, newLastModified, newDate)
}

func TestCheckPageUpdate_NotModified(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	url := "http://example.com"
	lastModified := "Mon, 02 Jan 2006 15:04:05 MST"

	httpmock.RegisterResponder("HEAD", url,
		httpmock.NewStringResponder(http.StatusNotModified, ""))

	hasUpdated, newDate, err := checkPageUpdate(url, lastModified)
	assert.Nil(t, err)
	assert.False(t, hasUpdated)
	assert.Equal(t, lastModified, newDate)
}

func TestCheckPageUpdate_UnexpectedStatus(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	url := "http://example.com"
	lastModified := "Mon, 02 Jan 2006 15:04:05 MST"

	httpmock.RegisterResponder("HEAD", url,
		httpmock.NewStringResponder(http.StatusInternalServerError, ""))

	hasUpdated, newDate, err := checkPageUpdate(url, lastModified)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unexpected HTTP status")
	assert.False(t, hasUpdated)
	assert.Equal(t, "", newDate)
}

func TestCheckPageUpdate_NetworkError(t *testing.T) {
	url := "http://10.255.255.1" // non-routable IP address to simulate network error
	lastModified := "Mon, 02 Jan 2006 15:04:05 MST"

	hasUpdated, newDate, err := checkPageUpdate(url, lastModified)
	assert.NotNil(t, err)
	assert.False(t, hasUpdated)
	assert.Equal(t, "", newDate)
}

