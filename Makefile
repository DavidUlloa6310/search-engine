GOCMD=go

all:
	$(GOCMD) build -o bin/main ./cmd/search-engine/main.go

clean:
	rm -f bin/main
