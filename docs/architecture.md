# Architecture

One Go binary serves the API and an embedded React interface. Metadata lives in SQLite,
files live on the local disk, and uploads use the [tus 1.0](https://tus.io) protocol.

```text
cmd/cloud            entry point: serve, passwd, healthcheck, cli, mcp
internal/command     CLI argument parsing and remote client configuration
internal/remote      OpenAPI catalog, authenticated HTTP client and transfers
internal/mcpserver   MCP tool adapter over stdio
internal/app         composition root and background cleanup
internal/config      environment configuration
internal/auth        users, sessions, bcrypt, login throttling, first-run setup
internal/files       storage on os.Root: listing, folders, trash, usage, archives
internal/upload      tus 1.0 upload sessions: creation, termination, expiration
internal/share       temporary file/folder links and upload-only requests
internal/settings    application settings
internal/sqlite      SQLite adapters and migrations
internal/httpapi     HTTP handlers, error mapping, security middleware
internal/web         embedded single page application
api/openapi.yaml     HTTP contract, OpenAPI 3.1
web/                 React interface
```

Dependencies point inward: `httpapi` and `sqlite` depend on the domain packages, never
the reverse, and everything is wired together in `internal/app`.

The HTTP contract lives in [`api/openapi.yaml`](../api/openapi.yaml). Integration tests
validate every API response against it, and the frontend types are generated from it.

The binary embeds the same contract to discover CLI commands and MCP tool schemas.
Local transfer helpers and aliases are declared in its `x-client-operations` and
`x-cli-aliases` extensions. `cmd/cloud-docs` renders their shared command reference;
a test rejects documentation that no longer matches the contract.
Both clients use the existing HTTP endpoints and cookie sessions; neither opens the
server database or storage volume. Local MCP transfers are confined to an explicitly
configured directory. See [CLI](cli.md) and [MCP](mcp.md) for workflows and configuration.
