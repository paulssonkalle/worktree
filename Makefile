.PHONY: build install test clean

BINARY := worktree

build:
	go build -buildvcs=false -o $(BINARY) .

install:
	go install -buildvcs=false .

test:
	go test ./...

clean:
	rm -f $(BINARY)
