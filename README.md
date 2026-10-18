# Stentor

One post, everywhere -- a free, single-binary CLI for crossposting to many social platforms at once. No database, no daemon, no Docker required: it runs, posts, exits.

## Why this exists

The usual self-hosted answer to "post to several platforms at once" is [Postiz](https://github.com/gitroomhq/postiz-app) -- a genuinely capable tool, but its self-hosted stack bundles its own Postgres, its own Redis, a *second* separate Postgres for its Temporal workflow engine, and the Temporal server itself. On a small VPS (2 vCPU / 4GB, already running other things), that stack doesn't fit: pulling and extracting its images alone was enough to saturate swap and grind to a halt before a single container even started.

Stentor is the other end of that spectrum: one static Go binary, ~9MB, measured peak memory under 7MB for a real run. It does less than Postiz -- no scheduling UI, no analytics, no daemon -- on purpose. It posts, and exits.

## Install

Download a release binary (see Releases) or build from source:

```sh
git clone https://github.com/<you>/stentor.git
cd stentor
go build -o stentor ./cmd/stentor
```

A Dockerfile is included for convenience, but using it is never required -- the binary alone is the whole tool.

## Usage

```sh
stentor platforms                 # every supported platform + its friction tier
stentor auth <platform>           # interactive credential setup, one platform at a time
stentor status                    # which platforms you're already authenticated with
stentor post --platform mastodon,bluesky --text "hello" [--image path] [--link url]
stentor post --platform all --text "hello"   # every platform you've authenticated
```

Credentials live in `~/.stentor/credentials.json`, mode `0600`. Nothing else is written anywhere, and nothing runs in the background.

## Platform support and who gates it

Every platform below is fully implemented in this tool. The "friction" column is about **you getting credentials approved by that platform** -- it is never about whether Stentor supports it.

| Platform | Friction | Notes |
|---|---|---|
| Mastodon | None | Paste a token you generate yourself |
| Bluesky | None | An app password, not your real one |
| Telegram | None | Your own bot via @BotFather |
| Discord | None | A webhook on a channel you admin |
| dev.to | None | An API key from your own account |
| Hashnode | None | A personal access token |
| LinkedIn | Self-serve | Your own app, linked to a Page you admin |
| Reddit | **Currently blocked platform-side** | Reddit closed self-service app creation in late 2025; new apps need a manual "Responsible Builder Policy" review with no stated timeline. This tool's code works -- getting credentials is Reddit's process, not this tool's |
| Twitter/X | Self-serve, uncertain pricing | As of 2026, free write access is unclear -- check the X developer portal before relying on it |
| Facebook | Platform review | Meta App Review for `pages_manage_posts` |
| Instagram | Platform review | Same Meta app as Facebook, plus a linked IG Business account. **Images must be a public URL, not a local file** -- the Graph API fetches them server-side |
| Threads | Platform review | Its own Meta app product, separate review |
| TikTok | Platform review | No plain-text post type exists on TikTok at all -- posts always need `--image` (sent as a single-photo post with your text as caption) |
| YouTube | Platform review, and still won't work | The YouTube Data API has **no endpoint for text/Community posts**, only video uploads -- `stentor post --platform youtube` will always fail. Included for completeness and because `stentor auth youtube` is still useful to have verified working |

## Architecture

One interface, one file per platform, in `internal/platforms/`:

```go
type Platform interface {
    Name() string
    Tier() Tier
    FrictionNote() string
    IsAuthenticated(cfg *config.Store) bool
    Authenticate(ctx context.Context, cfg *config.Store, in *bufio.Reader) error
    Post(ctx context.Context, cfg *config.Store, c content.Content) (content.Result, error)
}
```

Adding a platform means adding one file that implements this interface and registering it in `internal/platforms/platform.go`'s `All()` -- nothing else in the codebase needs to change.

## What this is not

- Not a scheduler -- there's no "post later," only "post now."
- Not a dashboard -- there's no UI, analytics, or history beyond your shell's own scrollback.
- Not a replacement for Postiz if you actually want scheduling, analytics, and team features and have the VPS headroom to run it. Stentor exists for the case where you don't.

## License

[MIT](LICENSE)
