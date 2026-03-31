##
#   Copyright 2025 TechDivision GmbH
#
#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.
##

BINARY   := valet
CMD      := ./cmd/valet
DIST     := ../dist
VERSION  := $(shell git -C .. describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -s -w -X main.Version=$(VERSION)

.PHONY: build build-all install clean test lint help

## build: Build for the current OS/arch (output: ../dist/valet)
build:
	@mkdir -p $(DIST)
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY) $(CMD)
	@echo "Built $(DIST)/$(BINARY)  ($(VERSION))"

## install: Build and copy to /usr/local/valet-sh/bin/valet
install: build
	@mkdir -p /usr/local/valet-sh/bin
	install -m 755 $(DIST)/$(BINARY) /usr/local/valet-sh/bin/$(BINARY)
	@echo "Installed /usr/local/valet-sh/bin/$(BINARY)"

## build-all: Cross-compile for all supported platforms
build-all:
	@mkdir -p $(DIST)
	GOOS=linux  GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-linux-amd64  $(CMD)
	GOOS=linux  GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-linux-arm64  $(CMD)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-darwin-amd64 $(CMD)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-darwin-arm64 $(CMD)
	@echo "All binaries written to $(DIST)/"
	@ls -lh $(DIST)/$(BINARY)-*

## test: Run all tests
test:
	go test -race ./...

## lint: Run go vet (install golangci-lint separately for full linting)
lint:
	go vet ./...

## clean: Remove build artifacts
clean:
	rm -rf $(DIST)

## help: Show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //'
