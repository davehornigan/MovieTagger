.PHONY: build tests

build:
	go build ./cmd/movietagger

tests:
	go test ./...
