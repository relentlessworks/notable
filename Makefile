.PHONY: build test vet clean run

BINARY=notable
CMD_DIR=cmd/notable

build:
	CGO_ENABLED=0 go build -o $(BINARY) ./$(CMD_DIR)

test:
	go test ./...

vet:
	go vet ./...

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY) notable.json

docker-build:
	docker build -t notable .
