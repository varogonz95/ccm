BIN := ccm
PKG := ./cmd/ccm
PLATFORMS := windows/amd64 windows/arm64 linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
# Version without the leading "v". Only v* tags count: on v0.1.0 -> 0.1.0, after
# it -> 0.1.0-3-gabc1234; before any v* tag -> dev+<commit>; no git -> dev.
# A ".dirty"/"-dirty" suffix marks uncommitted changes. An empty VERSION= means dev.
VERSION ?= $(or $(patsubst v%,%,$(shell git describe --tags --match 'v[0-9]*' --dirty 2>/dev/null)),$(addprefix dev+,$(shell git describe --always --abbrev=12 --dirty=.dirty --exclude='*' 2>/dev/null)),dev)
override VERSION := $(or $(strip $(VERSION)),dev)
LDFLAGS := -X ccm/internal/api.Version=$(VERSION)

.PHONY: build test vet dist docker-dist clean

build:
	go build -ldflags="$(LDFLAGS)" -o $(BIN) $(PKG)

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
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags="-s -w $(LDFLAGS)" \
			-o dist/$(BIN)-$$os-$$arch$$ext $(PKG) || exit 1; \
	done

docker-dist:
	docker buildx build --build-arg VERSION=$(VERSION) --target dist --output type=local,dest=dist .

clean:
	rm -rf dist $(BIN) $(BIN).exe
