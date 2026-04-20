##
# Project Name
#
# @file
# @version 0.1

# Go Makefile

# Variables
PRJ=monujo
BINDIR=build

PREFIX?=/usr/local/bin

# Replace it with "sudo", "doas" or somethat, that allows root privileges on your
# system.
# SUDO=sudo
SUDO?=

# Version information
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0-dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
FLAGS := -buildvcs=false -ldflags "-X main.version=$(VERSION) -X main.gitCommit=$(COMMIT)"

.PHONY: all
all: build

.PHONY: build
build:
	$(foreach dir,$(wildcard cmd/*), echo "$(dir) building..."; go build $(FLAGS) -o $(BINDIR)/ ./$(dir);)

.PHONY: docker-build
docker-build: # just set token into GITHUB_TOKEN environment variable
	docker build -f Dockerfile -t local:$(PRJ) --secret id=github_token,env=GITHUB_TOcKEN .

.PHONY: test
test:
	go tool ginkgo ./...

.PHONY: test-race
test-race:
	go tool ginkgo -r --race --randomize-all --randomize-suites ./...

.PHONY: test-coverage
test-coverage:
	go tool ginkgo -r --cover ./...

.PHONY: test-integration
test-integration:
	MONUJO_TRON_E2E=1 go tool ginkgo --tags=integration -r ./internal/tron

.PHONY: run
run: build
	./$(BINDIR)/$(APP)

.PHONY: run-log
run-log: tidy build
	SLOG_LEVEL=debug ./$(BINDIR)/$(APP)

.PHONY: run-race
run-race: tidy
	go run -race $(LDFLAGS) ./cmd/$(APP)

.PHONY: lint
lint:
	go tool golangci-lint run ./...

.PHONY: lint-fix
lint-fix:
	go tool golangci-lint run -v --fix ./...

.PHONY: sec
sec:
	go tool gosec ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: sloc
sloc:
	cloc * >sloc.stats

.PHONY: clean
clean:
	go clean
	rm -rf $(BINDIR)

# end
