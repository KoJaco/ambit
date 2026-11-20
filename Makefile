# Dev workflow (stage 03) is unchanged and does not need this Makefile: run
# `ambit start` (or `make dev-build && bin/ambit start`) and `npm run dev` in
# frontend/ (Vite proxies to the API). `make build` embeds the UI for distribution.
VERSION ?= dev
PREFIX  ?= $(HOME)/.local
WEBUI   := internal/webui/dist

.PHONY: frontend webui build dev-build install test clean release

frontend:
	cd frontend && npm ci && npm run build

webui: frontend
	@mkdir -p $(WEBUI)
	find $(WEBUI) -mindepth 1 -not -name .gitkeep -delete
	cp -r frontend/build/client/. $(WEBUI)/
	@test -f $(WEBUI)/index.html || { echo "error: $(WEBUI)/index.html is missing; the frontend build did not produce frontend/build/client/index.html" >&2; exit 1; }

build: webui
	go build -ldflags "-s -w -X main.version=$(VERSION)" -trimpath -o bin/ambit ./cmd/ambit

# API-only binary without the UI bundle, for quick backend work.
dev-build:
	go build -o bin/ambit ./cmd/ambit

install: build
	install -d $(PREFIX)/bin
	install -m755 bin/ambit $(PREFIX)/bin/ambit
	@echo "installed ambit to $(PREFIX)/bin/ambit"

test:
	go test ./... -race

clean:
	rm -rf bin
	find $(WEBUI) -mindepth 1 -not -name .gitkeep -delete

# Build release artifacts and pin checksums (does not publish). Usage: make release VERSION=0.1.0
release:
	@test "$(VERSION)" != dev || { echo "usage: make release VERSION=x.y.z" >&2; exit 2; }
	scripts/release.sh $(VERSION)
