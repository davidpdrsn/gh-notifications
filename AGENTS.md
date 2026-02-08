# AGENTS

This file documents working conventions for humans and coding agents in this repository.

## Project Overview

- Language: Go (`go 1.23`)
- CLI binary: `gh-pr`
- Main command surface:
  - `gh-pr timeline <owner>/<repo>#<number>`
  - `gh-pr notifications`
  - `gh-pr timeline --schema`
  - `gh-pr notifications --schema`

## Tooling and Installation

- Install development tools only via Nix (`nix develop`).
- Do not install project tooling with `go install`, Homebrew, or other package managers for this repo.

## Behavioral Contract

- Normal command output is JSON events written to stdout, one event per line (streaming).
- Warnings and errors are written to stderr, not using JSON.
- Timeline commands always emit a synthetic first event (`pr.opened` for PR timelines, `issue.opened` for issue timelines).
- All events have stable ids, including thread ids.

## Architecture Map

- CLI orchestration: `internal/cli/timeline.go`
- Error model/exit codes: `internal/cli/errors.go`
- PR ref parsing: `internal/cli/parse.go`
- GitHub API client: `internal/github/client.go`
- Auth token resolution: `internal/github/auth.go`
  - Resolution order: `GITHUB_TOKEN` -> `GH_TOKEN` -> `gh auth token`
- Event mapping and normalization: `internal/timeline/mapper.go`
- Notification mapping and normalization: `internal/notifications/mapper.go`
- Event sorting: `internal/timeline/sort.go`
- Embedded OpenAPI schema: `internal/schema/timeline.openapi.yaml`
- Public OpenAPI schema source: `openapi/timeline.openapi.yaml`
- Notifications OpenAPI schema source: `openapi/notifications.openapi.yaml`

## Code Generation Rules (Hard Requirement)

- Never manually edit generated code.
- Always update source schema/config and regenerate.
- Generated file in this repo:
  - `internal/timelineapi/types.gen.go`
  - `internal/notificationsapi/types.gen.go`
- Codegen command:

```bash
oapi-codegen -config openapi/timeline.codegen.yaml openapi/timeline.openapi.yaml
oapi-codegen -config openapi/notifications.codegen.yaml openapi/notifications.openapi.yaml
```

- Keep `openapi/timeline.codegen.yaml` with:
  - `output-options.skip-prune: true`
- Reason: this spec is component-focused (no `paths`), and pruning can drop required models.

## Schema Update Workflow

When changing timeline payload fields:

1. Update `openapi/timeline.openapi.yaml`.
2. Mirror changes in `internal/schema/timeline.openapi.yaml`.
3. Regenerate models with `oapi-codegen`.
4. Update mapper/call sites for generated naming and pointer semantics.
5. Run tests: `go test ./...`.

## Testing Expectations

- Run `go test ./...` after functional or schema/model changes.
- Keep mapper tests focused on deterministic IDs and stable grouping behavior.

## Notes

- `oapi-codegen` may warn about OpenAPI 3.1 support; this is currently expected in this repo.
