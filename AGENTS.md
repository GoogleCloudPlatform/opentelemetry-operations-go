# Guidance for AI Agents working in this Repo

This repository is a multi-module Go project containing OpenTelemetry exporters for Google Cloud. To work effectively here, please follow these guidelines.

## Core Expectations

- **Application Safety**: Exporters run in the context of host applications. Exporters should not panic, block indefinitely, or leak memory.
- **Minimal Changes**: Prefer minimal, surgical changes over broad refactors.
- **Backward Compatibility**: Keep public APIs backward compatible unless the task explicitly requires a breaking change.
- **Boundary Inspection**: Carefully inspect input validation, resource limits, cancellation, shutdown, and error propagation.

## Repository Structure

- This is a **multi-module repository**. There are over 20 `go.mod` files scattered across directories like `exporter`, `detectors`, `example`, and `tools`.
- Key directories include:
    - `exporter/`: Contains the core exporter implementations (Trace, Metric, etc.).
    - `detectors/`: Contains resource detectors for GCP environments.
    - `internal/`: Internal packages shared across modules.
    - `tools/`: Contains tools used for building and linting.

## Working with Modules

1.  **Stay in Context**: When running Go commands (`go test`, `go build`, `go mod tidy`), ensure your current working directory is the one containing the relevant `go.mod` file. Running these from the root may not work as expected or may operate on the root module only.
2.  **Go Workspaces**: You can use `make go-work` to initialize a Go workspace (`go.work`) locally. This makes it easier to work across multiple modules without adding temporary `replace` directives. **Do not commit `go.work` or `go.work.sum`**; they are ignored by git.

## Default Workflow

1.  **Read Context**: Read the relevant package, its tests, and any available documentation before making changes.
2.  **Test-Driven**: When fixing a bug or adding a feature, prefer adding a failing test first to demonstrate the issue or required behavior.
3.  **Surgical Edits**: Implement the smallest change that fulfills the requirement.
4.  **Verification**: Always run `make precommit` from the root (or relevant commands in the module directory) before considering the work complete.

## Building and Testing

- **Makefile**: The root `Makefile` contains targets that iterate over all modules.
- **Validation**: Run `make precommit` to validate your changes. This target typically runs generation, builds all packages, runs linters, and runs tests with the race detector. It is a good proxy for what CI will run.
- **Tests**: Place unit tests in the same directory as the code being tested, following standard Go conventions (e.g., `foo_test.go` for `foo.go`).

## Pull Requests

When creating or proposing a pull request, please structure the description with the following sections:
-   **Summary**: A clear explanation of what changed and why. Reference any related issues.
-   **Testing**: A description of how the changes were tested (e.g., added unit tests, ran integration tests).
