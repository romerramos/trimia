BINARY := dist/trimia
INPUT ?=
OUTPUT ?=

.PHONY: build run test clean

build:
	mkdir -p dist
	cd core && go build -o ../$(BINARY) ./cmd/trimia

run: build
	$(BINARY) $(INPUT) $(if $(OUTPUT),--output $(OUTPUT),)

test:
	cd core && go test ./...

clean:
	rm -rf dist
