.PHONY: build test package clean

PLUGIN_NAME = tuneshine

build: plugin.wasm

plugin.wasm: *.go go.mod go.sum
	@if command -v tinygo >/dev/null 2>&1; then \
		echo "Building with TinyGo..."; \
		tinygo build -target wasip1 -buildmode=c-shared -o plugin.wasm -scheduler=none .; \
	else \
		echo "Building with standard Go..."; \
		GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm .; \
	fi

test:
	go test ./...

package: build
	zip $(PLUGIN_NAME).ndp plugin.wasm manifest.json

clean:
	rm -f plugin.wasm $(PLUGIN_NAME).ndp
