# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project overview

Go-Probot is a framework for building GitHub/GitLab Apps in Go, inspired by [Probot](https://github.com/probot/probot). It receives webhook events over HTTP and dispatches them to typed handler functions. A built-in dashboard shows event metrics.

## Commands

```bash
# Run all tests with race detection
make test

# Run a single test
go test -v --race ./... -run TestGitHubAPP

# Regenerate code-gen'd files (github_events.go, github_app_handle.go, etc.)
# and rebuild the dashboard frontend
make generate
```

Code generation uses `tools/codegen/main.go`, which takes a Go template (`-t`), a YAML values file (`-v`), and an output path (`-o`). The `go:generate` directives are in `alias.go`, `github_app.go`, `github_app_handle.go`, and `gitlab_app.go`.

## Architecture

Everything lives in a single package `probot` at the repo root. The two central types are parameterized with Go generics:

- **`App[GT GitClientType]`** (`interface.go:11`) — a webhook server. Two implementations: `githubApp` and `gitlabApp`, created via `NewGitHubAPP()` / `NewGitLabAPP()`. Each app accepts `pflag` flags for configuration, builds an `http.Server`, and dispatches incoming webhooks to registered handlers.
- **`ProbotContext[GT, PT]`** (`interface.go:19`) — passed to each handler. Extends `context.Context` with `Payload()`, `Client()`, `GraphQL()`, `Logger()`, and `Must(...)` (panics on first non-nil error).

`GitClientType` (`alias.go:15`) is a union of `GitHubClient` (aliased to `*github.Client`) and `GitLabClient` (aliased to `*gitlab.Client`).

### Webhook dispatch flow

1. HTTP POST arrives at the configured path (default `/hook`).
2. The app validates the HMAC signature (GitHub) or secret token (GitLab).
3. It decodes the event type from headers and the payload body.
4. The generated `handelEvent` method (`github_app_handle.go` / `gitlab_app_handle.go`) switches on event type, calls `genericHandleFunc` (`webhook_event.go:10`), which:
   - Unmarshals the JSON payload into the typed event struct.
   - Resolves the GitHub client for the installation.
   - Looks up the registered handler key (e.g., `"issues.opened"` → falls back to `"issues"`).
   - Invokes the handler with a `ProbotContext`.
5. Panics in handlers are recovered and returned as HTTP 400 errors.

### Handler registration API

```go
app.On(probot.GitHub.IssueComment.Created).
    WithHandler(probot.GitHub.IssueComment.Handler(func(ctx probot.GitHubIssueCommentContext) {
        // ctx.Payload() is *github.IssueCommentEvent
        // ctx.Client() is *github.Client
    }))
```

- `probot.GitHub` is a `*githubEvent` struct whose fields are per-event "all-in-one" types (e.g., `githubEventIssueCommentAllInOne`).
- Each all-in-one type embeds the base event (implements `WebhookEvent`) plus per-action variants (`.Created`, `.Deleted`, etc.).
- Calling `.Handler(fn)` on the all-in-one type wraps `fn` in `EventHandlerFunc[GT, PT]`, which implements `Handler`.
- Multiple events can be registered in one `On()` call; a single handler will receive all of them.

### Code generation

The YAML files (`github_events.yaml`, `gitlab_events.yaml`) define the event types and their actions. Three kinds of files are generated from these:

| Template | Output | Purpose |
|---|---|---|
| `*_events.go.tmpl` | `*_events.go` | Event type structs, context type aliases, `.Handler()` methods |
| `*_events_test.go.tmpl` | `*_events_test.go` | Tests for event type/action strings |
| `*_app_handle.go.tmpl` | `*_app_handle.go` | `handelEvent` switch statement dispatching each event |

If you add a new event type, edit the YAML file and run `make generate`.

### Dashboard

The `web/` package embeds a simple dashboard UI (`dist/`) and registers it at `/dashboard/`. It also exposes a REST API at `/api/events` (via `go-restful`) that returns event metrics. Static assets are served from `/assets/` using `embed.FS`. The `web/dashboard/` directory contains the frontend source (built with npm).

### Testing

Tests use `ginkgo/gomega` for assertions and `h2non/gock` for HTTP mocking. The `mock` package (`mock/mock.go`) provides `Send[GT]()` which constructs a signed webhook request and POSTs it to a running app — this is the standard way to write integration-style handler tests. App tests start the server on a random port, send a mock event, and assert the handler's side effects (typically GitHub API calls mocked via gock).
