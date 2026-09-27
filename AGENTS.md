# Project Guidelines

## Scope

- Keep platform API integration in `core` or the upstream `music-lib` boundary.
- Keep Web routing and page state in `internal/web`.
- Keep browser behavior in `internal/web/templates/static/js/app.js` and reuse the existing server-rendered templates.
- Preserve the existing CLI, desktop, and mobile entry points unless a task explicitly changes them.

## Implementation

- Follow existing Go and JavaScript style; avoid unrelated refactors.
- Never log or return stored platform cookies.
- Platform API failures must remain distinguishable from empty results.
- Add focused tests for changed recommendation and route behavior.

## Verification

- Run `go test ./core ./internal/web ./cmd/music-dl` for backend and Web changes.
- Run broader tests only when shared behavior changes.
- Update `ROADMAP.md` after implemented work is verified.
