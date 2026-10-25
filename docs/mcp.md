# MCP server

`cloud mcp` exposes the application's HTTP operations as MCP tools over stdio.
It uses the [official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk).
The MCP client starts this process; the Cloud HTTP server must already be running.
No extra listening port or database access is needed.

## Connect a client

Start with [shared connection setup](clients.md) for the server address, CLI login
and session configuration. This page describes the MCP transport and permissions.

Build the binary, sign in through the CLI, and create a dedicated local transfer
directory if the agent should upload or download files:

```bash
go build -o cloud ./cmd/cloud
./cloud cli --server http://localhost:8080 login --username owner
mkdir -p ./agent-files
./cloud mcp --help
```

For clients that accept an `mcpServers` configuration, the equivalent stdio entry
is below. Replace all example paths with absolute paths on the machine running
the MCP process:

```json
{
  "mcpServers": {
    "cloud": {
      "command": "/absolute/path/cloud",
      "args": [
        "mcp",
        "--server", "http://localhost:8080",
        "--transfer-dir", "/absolute/path/agent-files"
      ]
    }
  }
}
```

The client and MCP process must run as the same OS user to share the default CLI
session. For separate credentials or multiple accounts, pass the same explicit
`--session /absolute/path/session.json` to both login and MCP. `CLOUD_SERVER` and
`CLOUD_SESSION_FILE` can also supply those settings. Use HTTPS for remote servers.

The MCP server reads its session when it starts. After a session expires, is
revoked, or is replaced by an external CLI login, sign in again and restart the
MCP connection. `login`, `setup` and `logout` tools can also change this process's
session, but signing in through the CLI keeps the password out of model context.

## Available tools

Tools have the same names and flat arguments as the CLI's canonical commands.
All HTTP `operationId` values come from the embedded OpenAPI contract, with
`uploadFile` and `uploadFolder` added for local file transfers. CLI aliases are not extra MCP tools.

See the [generated command and tool reference](commands.md) for every canonical
name, CLI alias and required argument. Its source is `api/openapi.yaml`; the same
catalog drives CLI help and MCP tool discovery.

The MCP client's tool discovery returns input schemas with required fields,
types, array constraints and limits taken from OpenAPI. Unknown fields and invalid
arguments are rejected before sending an HTTP request. The HTTP server still
owns authorization, validation, quotas, throttling and business rules.

## Example agent workflow

An instruction such as “Create a garden folder, upload the catalog from the
transfer directory, and create a link valid for one hour” maps to these tool calls:

```json
{"name":"createFolder","arguments":{"path":"garden"}}
{"name":"uploadFile","arguments":{"input":"seed-catalog-2026.txt","folder":"garden"}}
{"name":"createLink","arguments":{"path":"garden/seed-catalog-2026.txt","ttlSeconds":3600}}
```

To inspect and download the result:

```json
{"name":"listFolder","arguments":{"path":"garden"}}
{"name":"downloadFile","arguments":{"path":"garden/seed-catalog-2026.txt","output":"catalog-copy.txt"}}
```

A successful download has the following tool result shape (22 is an example size):

```json
{
  "content": [
    {"type":"text","text":"{\"status\":200,\"data\":{\"bytes\":22,\"output\":\"catalog-copy.txt\"}}"}
  ],
  "structuredContent": {
    "status": 200,
    "data": {"bytes":22,"output":"catalog-copy.txt"}
  }
}
```

API and transfer failures return `isError: true` with a text explanation, so the
agent can correct its request. Authentication errors include HTTP `401`; limits
may return `429` and a retry delay. The server does not automatically retry writes.
JSON API responses are limited to 8 MiB. File and archive downloads stream directly
to disk without that response-size limit.

## Local file access and permissions

Without `--transfer-dir`, local upload and download tools fail with a clear error.
Other management tools continue to work. When set, the directory must already
exist. Transfer paths are relative to it and opened through Go's `os.Root`;
absolute paths, `..` escapes and symlinks escaping that root are rejected.

Downloads stage bytes in a temporary file with mode `0600`, sync it, then publish
the final name without overwriting existing files. Normal failures remove the
temporary file. They do not put
binary contents into model context. `uploadFile` reads a regular file within this
directory and streams it in chunks of up to 4 MiB. An interrupted upload reports
its ID; call it again with `uploadId` and the **same unchanged source** to resume.
Size and offset are checked; source content is not hashed. Completed upload receipts remain available until `Upload-Expires`, including
after a process restart or lost final reply. A force-killed download can leave a
`.cloud-download-*.part` temporary file; the final destination remains absent.

The low-level `tusWrite` tool accepts at most 1 MiB of decoded base64 data per
call; prefer `uploadFile` for large files. To download elsewhere, move the finished
file outside the transfer directory yourself.

The default tool set can modify and permanently delete data with the signed-in
account's permissions. For browsing and inspection only, add `--read-only`:

```bash
./cloud mcp --server http://localhost:8080 --read-only
```

That mode exposes only GET, HEAD and OPTIONS operations. It removes tools for
login, uploads, mutations and permanent deletion from this MCP instance; it does
not change the underlying account or session permissions. Download tools remain
available and still require a transfer directory. Configure approval of destructive
tool calls in your MCP client according to how you use the agent.

Session tokens and `Set-Cookie` headers are excluded from tool results. Link
creation intentionally returns a share URL, which grants access to its recipient.
The session file itself is not accessible through a tool unless you deliberately
place it inside the transfer directory; keep credentials outside that directory.

## Transport and development

The server implements the [MCP stdio transport](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports):
newline-delimited JSON-RPC messages on stdin/stdout, with lifecycle negotiation,
tool discovery and calls handled by the SDK. Stdout contains only protocol
messages. Startup failures are written to stderr. The process exits when the
client disconnects or it receives a termination signal.

The adapters are `internal/command`, `internal/mcpserver` and `internal/remote`.
They call the existing HTTP API rather than duplicating domain services or
bypassing authentication. `internal/httpapi/clients_test.go` exercises CLI and
MCP against a real temporary application, including file transfers and the
local filesystem boundary. Client tests cover ambiguous upload failures, session
isolation, cancellation, integer precision and failed download cleanup.

## Shared folders, file requests and directory uploads

The generated tools include `publicFolder`, `publicFolderContent`,
`publicFolderArchive` and the five `publicTus*` operations. They use the token from
`/share/TOKEN` and need no signed-in session. Binary downloads still require a
transfer directory and write a new local output file.

```json
{"name":"createLink","arguments":{"path":"garden","mode":"download","ttlSeconds":86400}}
{"name":"createLink","arguments":{"path":"inbox","mode":"upload","ttlSeconds":86400}}
{"name":"publicFolder","arguments":{"token":"TOKEN","path":"greenhouse"}}
{"name":"publicFolderArchive","arguments":{"token":"TOKEN","output":"garden.zip"}}
{"name":"uploadFile","arguments":{"input":"seed-catalog-2026.txt","token":"UPLOAD_TOKEN"}}
{"name":"uploadFolder","arguments":{"input":"garden","folder":"documents","name":"spring-garden"}}
```

Replace token placeholders with actual 22-character tokens returned by `createLink`.
Public paths are relative to the shared folder. Upload-only links never return
its contents, accept individual files up to 5 GiB, and cannot change destination.
To resume an interrupted request upload, call `uploadFile` with the same `token`,
`uploadId` and unchanged source file. Link expiry and revocation are checked again
on each chunk request.

`uploadFolder` requires a signed-in session and a source directory inside
`--transfer-dir`. It preserves nesting and empty directories, merges existing
remote directories, skips symlinks and never overwrites existing files. Partial
results remain after failure. Repeating a folder upload reuses same-name files
only when every byte matches; different content returns a conflict with no
suffixed copy. Comparison retransmits the local file and requires temporary disk
space, but no additional logical quota. Individual authenticated `uploadFile`
calls can opt in with `"conflict":"skip-identical"`; public upload links cannot. It is excluded in read-only mode. Neither upload helper reads the server's
database or storage directly.
