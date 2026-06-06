BINARY := dist/trimia
INPUT ?= inputs/test.mov
OUTPUT ?=

.PHONY: build run test clean

build:
	mkdir -p dist
	go build -o $(BINARY) ./cmd/trimia

run: build
	$(BINARY) -input $(INPUT) $(if $(OUTPUT),-output $(OUTPUT),)

test:
	go test ./...

clean:
	rm -rf dist
