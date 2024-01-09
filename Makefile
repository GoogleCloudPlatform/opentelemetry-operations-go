TOOLS_MOD_DIR := ./tools

# All source code and documents. Used in spell check.
ALL_DOCS := $(shell find . -name '*.md' -type f | sort)
# All directories with go.mod files related to exporters. Used for building, testing and linting.
ALL_GO_MOD_DIRS := $(filter-out $(TOOLS_MOD_DIR), $(shell find . -type f -name 'go.mod' -exec dirname {} \; | sort))
ALL_GO_FILES_DIRS := $(filter-out $(TOOLS_MOD_DIR), $(shell find . -type f -name '*.go' -exec dirname {} \; | sort | uniq))
ALL_COVERAGE_MOD_DIRS := $(shell find . -type f -name 'go.mod' -exec dirname {} \; | grep -E -v '^./example|^$(TOOLS_MOD_DIR)' | sort)

GOTEST = go test -v -timeout 70s
GOTEST_SHORT = $(GOTEST) -short
GOTEST_RACE = $(GOTEST) -race

.DEFAULT_GOAL := precommit

.PHONY: precommit

TOOLS = $(CURDIR)/.tools

$(TOOLS):
	@mkdir -p $@
$(TOOLS)/%: | $(TOOLS)
	cd $(TOOLS_MOD_DIR) && \
	go build -o $@ $(PACKAGE)

GOLANGCI_LINT = $(TOOLS)/golangci-lint
$(TOOLS)/golangci-lint: PACKAGE=github.com/golangci/golangci-lint/cmd/golangci-lint

GOVULNCHECK = $(TOOLS)/govulncheck
$(TOOLS)/govulncheck: PACKAGE=golang.org/x/vuln/cmd/govulncheck

MISSPELL = $(TOOLS)/misspell
$(TOOLS)/misspell: PACKAGE=github.com/client9/misspell/cmd/misspell

STRINGER = $(TOOLS)/stringer
$(TOOLS)/stringer: PACKAGE=golang.org/x/tools/cmd/stringer

GOJQ = $(TOOLS)/gojq
$(TOOLS)/gojq: PACKAGE=github.com/itchyny/gojq/cmd/gojq

PROTOC_GEN_GO = $(TOOLS)/protoc-gen-go
$(TOOLS)/protoc-gen-go: PACKAGE=google.golang.org/protobuf/cmd/protoc-gen-go

FIELDALIGNMENT = $(TOOLS)/fieldalignment
$(TOOLS)/fieldalignment: PACKAGE=golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment

GOCOVMERGE = $(TOOLS)/gocovmerge
$(TOOLS)/gocovmerge: PACKAGE=github.com/wadey/gocovmerge

.PHONY: tools
tools: $(GOLANGCI_LINT) $(MISSPELL) $(GOCOVMERGE) $(STRINGER) $(GOJQ) $(FIELDALIGNMENT) $(PROTOC_GEN_GO)

PROTOBUF_VERSION = 3.19.0
PROTOBUF_OS = linux
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
	PROTOBUF_OS = osx
endif

PROTOC = $(TOOLS)/protoc
$(TOOLS)/protoc: $(PROTOC_GEN_GO)
	tmpdir=$$(mktemp -d) && \
	cd $$tmpdir && \
	curl -L https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOBUF_VERSION)/protoc-$(PROTOBUF_VERSION)-$(PROTOBUF_OS)-x86_64.zip \
		-o protoc.zip && \
	unzip protoc.zip bin/protoc && \
	cp bin/protoc $(TOOLS)/ ; \
	rm -rf $$tmpdir

precommit: generate build lint test-race fixtures

COVERAGE_MODE    = atomic
COVERAGE_PROFILE = coverage.out
.PHONY: test-coverage
test-coverage: | $(GOCOVMERGE)
	@set -e; \
	printf "" > coverage.txt; \
	for dir in $(ALL_COVERAGE_MOD_DIRS); do \
	  echo "go test -coverpkg=go.opentelemetry.io/otel/... -covermode=$(COVERAGE_MODE) -coverprofile="$(COVERAGE_PROFILE)" $${dir}/..."; \
	  (cd "$${dir}" && \
	    go list ./... \
	    | grep -v third_party \
	    | grep -v 'semconv/v.*' \
	    | xargs go test -coverpkg=./... -covermode=$(COVERAGE_MODE) -coverprofile="$(COVERAGE_PROFILE)" && \
	  go tool cover -html=coverage.out -o coverage.html); \
	done; \
	$(GOCOVMERGE) $$(find . -name coverage.out) > coverage.txt

.PHONY: ci
ci: precommit check-clean-work-tree test-coverage test-race

.PHONY: check-clean-work-tree
check-clean-work-tree:
	@if ! git diff --quiet; then \
	  echo; \
	  echo 'Working tree is not clean, did you forget to run "make precommit"?'; \
	  echo; \
	  git status; \
	  git diff; \
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

.PHONY: test-race
test-race:
	set -e; for dir in $(ALL_GO_MOD_DIRS); do \
	  echo "go test ./... + race in $${dir}"; \
	  (cd "$${dir}" && \
	    $(GOTEST_RACE) ./...); \
	done

.PHONY: integrationtest
integrationtest:
	set -e; for dir in $(ALL_GO_MOD_DIRS); do \
	  echo "go test ./... + race in $${dir}"; \
	  (cd "$${dir}" && \
	    $(GOTEST_RACE) -tags=integrationtest -run=TestIntegration ./...); \
	done

.PHONY: test-short
test-short:
	set -e; for dir in $(ALL_GO_MOD_DIRS); do \
	echo "go test ./... in $${dir}"; \
	(cd "$${dir}" && $(GOTEST_SHORT) ./...); \
	done

.PHONY: test
test:
	set -e; for dir in $(ALL_GO_MOD_DIRS); do \
	echo "go test ./... in $${dir}"; \
	(cd "$${dir}" && $(GOTEST) ./...); \
	done

.PHONY: lint
lint: $(GOLANGCI_LINT) $(MISSPELL) govulncheck
	set -e; for dir in $(ALL_GO_MOD_DIRS); do \
	  echo "golangci-lint in $${dir}"; \
	  (cd "$${dir}" && \
	    $(GOLANGCI_LINT) --config $(CURDIR)/golangci.yml run --fix && \
	    $(GOLANGCI_LINT) --config $(CURDIR)/golangci.yml run); \
	done
	$(MISSPELL) -w $(ALL_DOCS)
	set -e; for dir in $(ALL_GO_MOD_DIRS) $(TOOLS_MOD_DIR); do \
	  echo "go mod tidy -compat=1.20 in $${dir}"; \
	  (cd "$${dir}" && \
	    go mod tidy -compat=1.20); \
	done

generate: $(STRINGER) $(PROTOC)
	$(MAKE) for-all-mod PATH="$(TOOLS):$${PATH}" CMD="go generate ./..."

.PHONY: fieldalignment
fieldalignment: $(FIELDALIGNMENT)
	$(MAKE) for-all-package CMD="$(FIELDALIGNMENT) -fix ."


.PHONY: for-all-mod
for-all-mod:
	@$${CMD}
	@set -e; for dir in $(ALL_GO_MOD_DIRS); do \
	  (cd "$${dir}" && \
	  	echo "running $${CMD} in $${dir}" && \
	 	$${CMD} ); \
	done

.PHONY: for-all-package
for-all-package:
	@$${CMD}
	@set -e; for dir in $(ALL_GO_FILES_DIRS); do \
	  (cd "$${dir}" && \
	  	echo "running $${CMD} in $${dir}" && \
	 	$${CMD} || true); \
	done

.PHONY: govulncheck
govulncheck: $(ALL_GO_MOD_DIRS:%=govulncheck/%)
govulncheck/%: DIR=$*
govulncheck/%: | $(GOVULNCHECK)
	@echo "govulncheck ./... in $(DIR)" \
		&& cd $(DIR) \
		&& $(GOVULNCHECK) ./...

.PHONY: gotidy
gotidy:
	$(MAKE) for-all-mod CMD="go mod tidy -compat=1.20"

.PHONY: update-dep
update-dep:
	$(MAKE) for-all-mod CMD="$(PWD)/internal/buildscripts/update-dep"

STABLE_OTEL_VERSION=v1.21.0
UNSTABLE_OTEL_VERSION=v0.44.0
STABLE_CONTRIB_OTEL_VERSION=v1.21.1
UNSTABLE_CONTRIB_OTEL_VERSION=v0.46.1
STABLE_COLLECTOR_VERSION=v1.0.0
UNSTABLE_COLLECTOR_VERSION=v0.91.0
UNSTABLE_COLLECTOR_CONTRIB_VERSION=v0.91.0

.PHONY: update-otel
update-otel:
	$(MAKE) update-dep MODULE=go.opentelemetry.io/otel VERSION=$(STABLE_OTEL_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/otel/metric VERSION=$(STABLE_OTEL_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/otel/metric/instrument VERSION=$(UNSTABLE_OTEL_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/otel/sdk VERSION=$(STABLE_OTEL_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/otel/sdk/metric VERSION=$(STABLE_OTEL_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/otel/trace VERSION=$(STABLE_OTEL_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp VERSION=$(UNSTABLE_OTEL_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/otel/exporters/prometheus VERSION=$(UNSTABLE_OTEL_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/contrib/detectors/gcp VERSION=$(STABLE_CONTRIB_OTEL_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp VERSION=$(UNSTABLE_CONTRIB_OTEL_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/collector VERSION=$(UNSTABLE_COLLECTOR_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/collector/semconv VERSION=$(UNSTABLE_COLLECTOR_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/collector/otelcol VERSION=$(UNSTABLE_COLLECTOR_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/collector/extension/auth VERSION=$(UNSTABLE_COLLECTOR_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/collector/featuregate VERSION=$(STABLE_COLLECTOR_VERSION)
	$(MAKE) update-dep MODULE=go.opentelemetry.io/collector/pdata VERSION=$(STABLE_COLLECTOR_VERSION)
	$(MAKE) update-dep MODULE=github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus VERSION=$(UNSTABLE_COLLECTOR_CONTRIB_VERSION)
	$(MAKE) gotidy
	$(MAKE) build

.PHONY: prepare-release
prepare-release:
	echo "make sure tools/release.go is updated to your desired stable and unstable versions"
	go run tools/release.go prepare
	$(MAKE) fixtures

.PHONY: release
release: prepare-release check-clean-work-tree
	go run tools/release.go tag

.PHONY: fixtures
fixtures:
	cd ./exporter/collector/integrationtest && \
	go run cmd/recordfixtures/main.go

.PHONY: go-work
go-work:
	go work init
	go work use -r .
	go work sync
