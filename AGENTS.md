# Agent Instructions for the DIN Caddy Plugins Repository

This document provides essential information for AI agents working within this codebase.

## Project Overview

This repository contains a collection of Go-based plugins for the Caddy web server. These plugins provide DIN-specific functionality, including custom middleware, authentication handlers, and dynamic upstream management. The project is built as a Caddy module and leverages `xcaddy` for custom builds.

## Essential Commands

The `Makefile` is the primary entry point for all development tasks.

*   **Setup:** `make dev` - Installs development dependencies and Git hooks. This should be the first command you run.
*   **Run Development Server:** `make run` - Starts Caddy using `Caddyfile.private`. This is the main command for local development.
*   **Build:** `make build` - Compiles a Caddy binary with all the DIN plugins included, placing it in the `build/` directory.
*   **Testing:**
    *   `make test`: Run all tests.
    *   `make test-verbose`: Run tests with verbose output.
    *   `make test-coverage`: Generate an HTML test coverage report.
    *   `make test-race`: Run tests with the race detector enabled.
    *   `make quick-test`: Run a faster subset of tests.
*   **Code Quality:**
    *   `make lint`: Run the `golangci-lint` linter.
    *   `make format`: Format Go code using `gofmt` and `goimports`.
    *   `make vet`: Run `go vet`.
    *   `make check`: Run a comprehensive suite of checks (format, vet, lint, test, security). This is used in the pre-commit hook.
*   **Mocks:** `make generate-mocks` - Regenerates mock interfaces using `mockgen`.

## Code Organization

The repository is structured as a standard Go project with a few Caddy-specific conventions.

*   `module.go`: The main entry point for the Caddy plugins. It handles the registration of all modules and directives.
*   `modules/network/`: Contains the network/RPC proxy Caddy modules (`din`, `din_auth`, `din_select`, `din_upstreams`).
*   `modules/ai/`: Contains the AI proxy Caddy module (`din_ai`) for routing AI API requests.
*   `lib/`: Shared libraries and packages used by the modules. This includes authentication logic (`auth/`), network utilities (`network/`), health check framework (`health/`), shared provider interface (`provider/`), metrics interface (`metrics/`), and Prometheus metrics (`prometheus/`).
*   `scripts/`: Contains helper scripts, including one for managing secrets by generating Caddyfiles from templates.
*   `Caddyfile*`: Various Caddy configuration files for different environments. `Caddyfile.private` is used for local development and is not checked into source control.
*   `Makefile`: Defines all the common development and build tasks.

## Architecture Principles & Patterns

*   **Plugin-Based Architecture:** All functionality is implemented as Caddy modules, which are registered in `module.go`.
*   **Handler Abstraction:** Maintain a handler-based abstraction for network-specific logic. Avoid hardcoded network-specific string checks in core modules.
*   **Dependency Injection:** Use dependency injection and interfaces for testability. Mocks are generated for these interfaces.
*   **Configuration:** The plugins are configured through the Caddyfile using custom directives like `din` and `din_auth`. The parsing logic for these directives is located in the respective module files.
*   **Extensibility:** When adding new network types or other major features, follow the established patterns in the existing code.

## Code Quality, Style, & Conventions

This project follows strict professional and quality guidelines.

### Professional Standards
*   NEVER use emojis in any code, comments, commit messages, or documentation.
*   Maintain professional, business-appropriate language throughout the codebase.
*   Use clear, descriptive comments without decorative symbols.

### Code Quality Guidelines
*   Follow Go best practices and idioms.
*   Use clear, descriptive variable and function names.
*   Prefer explicit error handling over silent failures.

### Documentation Standards
*   Use plain text markers like "NOTE:", "TODO:", "DEPRECATED:" instead of emojis.
*   Write clear, professional documentation.
*   Use standard markdown formatting without decorative symbols.

## Testing Approach

*   Tests are located in `_test.go` files alongside the code they are testing.
*   Write comprehensive tests for new functionality.
*   The project uses the standard Go testing library.
*   `make test` is sufficient to run all tests.
*   Mocks are used to test components in isolation. You can regenerate them with `make generate-mocks` if you modify an interface.

## Gotchas & Important Notes

*   **Not a Standalone App:** This project provides plugins for Caddy. It is not a standalone executable and must be built into a Caddy binary using `xcaddy`.
*   **Secrets Management:** The `secrets` targets in the `Makefile` (e.g., `make secrets-init`) use `scripts/generate-caddyfile.go` to manage API keys and other secrets. For local development, copy `.env.example` to `.env.local` and populate it with your credentials.
*   **Dependency Installation:** The `Makefile` notes that `golangci-lint` and other tools should ideally be installed using `asdf` according to the `.tool-versions` file, not via `go install`.
