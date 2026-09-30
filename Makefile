BIN := ccm
PKG := ./cmd/ccm
PLATFORMS := windows/amd64 windows/arm64 linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.PHONY: build test vet dist docker-dist clean

build:
	go build -o $(BIN) $(PKG)

test:
	go test -race ./...

vet:
	go vet ./...
	GOOS=windows go vet ./...
	GOOS=darwin go vet ./...

dist:
	@mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=; [ $$os = windows ] && ext=.exe; \
		echo "building $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags="-s -w" \
			-o dist/$(BIN)-$$os-$$arch$$ext $(PKG) || exit 1; \
	done

docker-dist:
	docker buildx build --target dist --output type=local,dest=dist .

clean:
	rm -rf dist $(BIN) $(BIN).exe
