GOCMD=go

all:
	$(GOCMD) build -o bin/main ./cmd/WebCrawler/main.go

clean:
	rm -f bin/main
