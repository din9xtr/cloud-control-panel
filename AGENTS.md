# AGENTS.md

Guidance for coding agents working in this repository.

## Project

Self-hosted file storage: Go backend with an embedded React SPA, SQLite for metadata,
files on local disk, resumable uploads over tus 1.0. One binary, one container.

```text
cmd/cloud            entry point: serve, passwd, healthcheck, cli, mcp
internal/command     remote CLI commands and client configuration
internal/remote      OpenAPI operation catalog, HTTP client and file transfers
internal/mcpserver   MCP tools over the same authenticated HTTP client
internal/app         composition root, janitor
internal/config      environment configuration
internal/auth        users, sessions, bcrypt, login throttling, first-run setup
internal/files       storage on os.Root, trash, usage, archives
internal/upload      tus upload sessions
internal/share       temporary links
internal/settings    application settings
internal/sqlite      SQLite adapters and embedded migrations
internal/httpapi     handlers, error mapping, security middleware, rate limits
internal/web         embedded build of web/
api/openapi.yaml     HTTP contract, the single source of truth for the API
web/                 React 19, Vite, Tailwind 4, TanStack Query and Router
```

Dependencies point inward: `httpapi` and `sqlite` depend on domain packages, never the reverse.
Wiring happens only in `internal/app`.

## Commands

```bash
go test -race ./...
golangci-lint run ./...
cd web && pnpm install && pnpm test && pnpm lint && pnpm typecheck
cd web && pnpm build                          # writes internal/web/dist for embedding
CLOUD_DATA_DIR=data go run ./cmd/cloud        # API and embedded UI on :8080
cd web && pnpm dev                            # Vite on :5173, proxies the API
docker compose up -d --build                  # full stack
```

Every change must leave all of the above green.

## Conventions

- Go follows the Uber Go Style Guide and `.golangci.yml`. TypeScript is strict with ESLint.
- No code comments. Names and small functions carry the meaning.
- Work test-first where practical: table-driven Go tests, Testing Library and msw on the web side.
- Errors are wrapped with context, matched with `errors.Is`/`errors.As`, and mapped to HTTP
  exactly once in `internal/httpapi/respond.go`. Client messages never contain internal paths.
- `context.Context` is the first parameter of anything that does I/O.
- Interfaces live with their consumer and stay small.
- The frontend never hand-writes API types: change `api/openapi.yaml`, then `pnpm gen`.
  `internal/httpapi/api_test.go` validates every response against the contract.
- Every new or changed API endpoint must be supported in the web interface, CLI and MCP
  server in the same change. Update the OpenAPI contract first, verify the generated CLI
  command and MCP tool (including argument schemas and binary transfers), implement the web
  workflow, and update tests plus `docs/cli.md` and `docs/mcp.md`.
  Define local helper schemas in `x-client-operations` and aliases in `x-cli-aliases`
  in the same contract. Run `go run ./cmd/cloud-docs` to regenerate `docs/commands.md`;
  keep shared connection instructions in `docs/clients.md`. An endpoint is not done
  until all three clients support its behavior. Keep business rules and authorization in
  the server; CLI and MCP must use HTTP rather than accessing the database or storage directly.
- Database changes are new files in `internal/sqlite/migrations`, never edits of applied ones.
  Existing installations must keep working, including legacy tables.
- The environment only configures how the server runs. Anything a user can change in Settings
  lives in the database, with its default seeded by a migration.

## Security rules

- File access goes through `files.Volume` and `os.Root`. Never build filesystem paths from
  user input with `filepath.Join` or open files outside the volume.
- New names pass `files.ValidateNewName`; existing names only need `ValidateName`.
- Every endpoint requires a session unless it is deliberately public; public endpoints get a
  dedicated rate limit in `internal/httpapi/protect.go`.
- Keep the middleware chain intact: recover, access log, security headers, shield
  (load shedding, per-IP rate and concurrency limits, read deadlines), cross-origin protection.
- Expensive work (archives, search, password hashing) runs behind bounded slots.
- Never log tokens, passwords, file names or paths; access logs record route patterns only,
  and errors pass through `redact` in `internal/httpapi` before they are logged.
- Downloads are attachments; inline display is limited to images, audio, video, PDF and plain
  text, always with a sandboxing Content Security Policy.
- Keep the CSP strict: no inline scripts or styles, no third-party origins.

## Web design

Read [docs/design.md](docs/design.md) before changing anything visual in `web/`: tokens,
typography, layout, components and motion timings are defined there. Use the existing
tokens and components instead of new values, and check both desktop and phone widths.

## Fixtures and screenshots

Use invented, neutral names in tests, fixtures, demo data and screenshots
(for example `garden/greenhouse/seed-catalog-2026.pdf`). Never use real user data.
