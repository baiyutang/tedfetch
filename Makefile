.PHONY: build test lint clean

build:
	go build -o tedfetch main.go

test:
	go test ./... -v

lint:
	golangci-lint run --timeout=3m ./...

clean:
	rm -f tedfetch
	rm -rf test_downloads/* 