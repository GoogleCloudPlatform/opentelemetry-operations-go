TOOLS_MOD_DIR := ./tools

# All source code and documents. Used in spell check.
ALL_DOCS := $(shell find . -name '*.md' -type f | sort)
# All directories with go.mod files related to exporters. Used for building, testing and linting.
ALL_GO_MOD_DIRS := $(filter-out $(TOOLS_MOD_DIR), $(shell find . -type f -name 'go.mod' -exec dirname {} \; | sort))
ALL_COVERAGE_MOD_DIRS := $(shell find . -type f -name 'go.mod' -exec dirname {} \; | egrep -v '^./example|^$(TOOLS_MOD_DIR)' | sort)

# Mac OS Catalina 10.5.x doesn't support 386. Hence skip 386 test
SKIP_386_TEST = false
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
	SW_VERS := $(shell sw_vers -productVersion)
	ifeq ($(shell echo $(SW_VERS) | egrep '^(10.1[5-9]|1[1-9]|[2-9])'), $(SW_VERS))
		SKIP_386_TEST = true
	endif
endif

GOTEST_MIN = go test -v -timeout 70s
GOTEST = $(GOTEST_MIN) -race
GOTEST_WITH_COVERAGE = $(GOTEST) -coverprofile=coverage.txt -covermode=atomic

.DEFAULT_GOAL := precommit

.PHONY: precommit

TOOLS_DIR := $(abspath ./.tools)

$(TOOLS_DIR)/golangci-lint: $(TOOLS_MOD_DIR)/go.mod $(TOOLS_MOD_DIR)/go.sum $(TOOLS_MOD_DIR)/tools.go
	cd $(TOOLS_MOD_DIR) && \
	go build -o $(TOOLS_DIR)/golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint

$(TOOLS_DIR)/misspell: $(TOOLS_MOD_DIR)/go.mod $(TOOLS_MOD_DIR)/go.sum $(TOOLS_MOD_DIR)/tools.go
	cd $(TOOLS_MOD_DIR) && \
	go build -o $(TOOLS_DIR)/misspell github.com/client9/misspell/cmd/misspell

$(TOOLS_DIR)/stringer: $(TOOLS_MOD_DIR)/go.mod $(TOOLS_MOD_DIR)/go.sum $(TOOLS_MOD_DIR)/tools.go
	cd $(TOOLS_MOD_DIR) && \
	go build -o $(TOOLS_DIR)/stringer golang.org/x/tools/cmd/stringer

$(TOOLS_DIR)/gojq: $(TOOLS_MOD_DIR)/go.mod $(TOOLS_MOD_DIR)/go.sum $(TOOLS_MOD_DIR)/tools.go
	cd $(TOOLS_MOD_DIR) && \
	go build -o $(TOOLS_DIR)/gojq github.com/itchyny/gojq/cmd/gojq

$(TOOLS_DIR)/protoc-gen-go: $(TOOLS_MOD_DIR)/go.mod $(TOOLS_MOD_DIR)/go.sum $(TOOLS_MOD_DIR)/tools.go
	cd $(TOOLS_MOD_DIR) && \
	go build -o $(TOOLS_DIR)/protoc-gen-go google.golang.org/protobuf/cmd/protoc-gen-go

PROTOBUF_VERSION = 3.19.0
PROTOBUF_OS = linux
ifeq ($(UNAME_S),Darwin)
	PROTOBUF_OS = osx
endif

$(TOOLS_DIR)/protoc: $(TOOLS_DIR)/protoc-gen-go
	tmpdir=$$(mktemp -d) && \
	cd $$tmpdir && \
	curl -L https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOBUF_VERSION)/protoc-$(PROTOBUF_VERSION)-$(PROTOBUF_OS)-x86_64.zip \
		-o protoc.zip && \
	unzip protoc.zip bin/protoc && \
	cp bin/protoc $(TOOLS_DIR)/ ; \
	rm -rf $$tmpdir

precommit: generate build lint test

.PHONY: test-with-coverage
test-with-coverage:
	set -e; for dir in $(ALL_COVERAGE_MOD_DIRS); do \
	  echo "go test ./... + coverage in $${dir}"; \
	  (cd "$${dir}" && \
	    $(GOTEST_WITH_COVERAGE) ./... && \
	    go tool cover -html=coverage.txt -o coverage.html); \
	done

.PHONY: ci
ci: precommit check-clean-work-tree test-with-coverage test-386

.PHONY: check-clean-work-tree
check-clean-work-tree:
	@if ! git diff --quiet; then \
	  echo; \
	  echo 'Working tree is not clean, did you forget to run "make precommit"?'; \
	  echo; \
	  git status; \
	  exit 1; \
	fi

.PHONY: build
build:
	# TODO: Fix this on windows.
	set -e; for dir in $(ALL_GO_MOD_DIRS); do \
	  echo "compiling all packages in $${dir}"; \
	  (cd "$${dir}" && \
	    go build ./... && \
	    go test -run xxxxxMatchNothingxxxxx ./... >/dev/null); \
	done

.PHONY: test
test:
	set -e; for dir in $(ALL_GO_MOD_DIRS); do \
	  echo "go test ./... + race in $${dir}"; \
	  (cd "$${dir}" && \
	    $(GOTEST) ./...); \
	done

.PHONY: integrationtest
integrationtest:
	set -e; for dir in $(ALL_GO_MOD_DIRS); do \
	  echo "go test ./... + race in $${dir}"; \
	  (cd "$${dir}" && \
	    $(GOTEST) -tags=integrationtest -run=TestIntegration ./...); \
	done

.PHONY: test-386
test-386:
	if [ $(SKIP_386_TEST) = true ] ; then \
	  echo "skipping the test for GOARCH 386 as it is not supported on the current OS"; \
	else \
	  set -e; for dir in $(ALL_GO_MOD_DIRS); do \
	  echo "go test ./... GOARCH 386 in $${dir}"; \
	    (cd "$${dir}" && \
	      GOARCH=386 $(GOTEST_MIN) ./...); \
	  done; \
	fi

.PHONY: lint
lint: $(TOOLS_DIR)/golangci-lint $(TOOLS_DIR)/misspell
	set -e; for dir in $(ALL_GO_MOD_DIRS); do \
	  echo "golangci-lint in $${dir}"; \
	  (cd "$${dir}" && \
	    $(TOOLS_DIR)/golangci-lint --config $(CURDIR)/golangci.yml run --fix && \
	    $(TOOLS_DIR)/golangci-lint --config $(CURDIR)/golangci.yml run); \
	done
	$(TOOLS_DIR)/misspell -w $(ALL_DOCS)
	set -e; for dir in $(ALL_GO_MOD_DIRS) $(TOOLS_MOD_DIR); do \
	  echo "go mod tidy in $${dir}"; \
	  (cd "$${dir}" && \
	    go mod tidy); \
	done

generate: $(TOOLS_DIR)/stringer $(TOOLS_DIR)/protoc
	$(MAKE) for-all PATH="$(TOOLS_DIR):$${PATH}" CMD="go generate ./..."

.PHONY: for-all
for-all:
	@$${CMD}
	@set -e; for dir in $(ALL_GO_MOD_DIRS); do \
	  (cd "$${dir}" && \
	  	echo "running $${CMD} in $${dir}" && \
	 	$${CMD} ); \
	done

.PHONY: gotidy
gotidy:
	$(MAKE) for-all CMD="go mod tidy"

.PHONY: update-dep
update-dep:
	$(MAKE) for-all CMD="$(PWD)/internal/buildscripts/update-dep"

STABLE_OTEL_VERSION=v1.3.0
UNSTABLE_OTEL_VERSION=v0.26.0

.PHONY: update-otel
update-otel:
	$(MAKE) update-dep MODULE=go.opentelemetry.io/otel VERSION=$(STABLE_OTEL_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/otel/metric VERSION=$(UNSTABLE_OTEL_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/otel/sdk VERSION=$(STABLE_OTEL_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/otel/sdk/export/metric VERSION=$(UNSTABLE_OTEL_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/otel/sdk/metric VERSION=$(UNSTABLE_OTEL_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/otel/trace VERSION=$(STABLE_OTEL_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp VERSION=$(UNSTABLE_OTEL_VERSION)
	$(MAKE) build
	$(MAKE) gotidy
