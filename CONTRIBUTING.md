# Contributing to Stentor

Small tool, small process.

## Adding a platform

1. Create `internal/platforms/<name>.go` implementing the `Platform` interface (see `platform.go`).
2. Register it in `All()` in `platform.go`, in the right tier.
3. Add a test with a mocked HTTP server (`httptest.NewServer`) -- see `mastodon_test.go` / `discord_test.go` for the pattern. Never require a real account to run `go test`.
4. Update the platform table in `README.md`.

## Rules

- No database, no daemon, no background process of any kind. If a change needs one, it doesn't belong in this tool.
- Every HTTP call goes through `internal/platforms/httpx.go`'s helpers -- don't add a second way to make requests.
- No real credentials, tokens, usernames, or hostnames in any commit, ever -- examples use obvious placeholders (`your-token-here`, `example.social`).
- Keep the binary dependency-free where reasonably possible (standard library only, currently). If a platform genuinely needs an external library, say why in the PR description.

## Reporting a platform API change

Platform APIs drift. If a platform's endpoint, auth flow, or payload shape has changed, open an issue with a link to that platform's current docs -- PRs fixing it are very welcome.
