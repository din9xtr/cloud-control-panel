# Connect to Cloud

Cloud is a control panel for files on your own server. The browser, command line
and MCP agent use the same HTTP API, accounts, storage quota and access rules.
Use this page for shared setup; see [CLI examples](cli.md), [MCP configuration](mcp.md)
and the [generated command reference](commands.md) for client-specific details.

## Start the server

From a checkout, with Docker and Compose v2 installed:

```bash
mkdir -p data
docker compose up -d --build
```

Open `http://localhost:8080` and create your first account. The server must keep
running while you use any client. For remote access, configure HTTPS using the
[reverse proxy guide](configuration.md). The server URL below is the origin only,
such as `https://cloud.example.com`, without `/api/v1`.

Files and metadata are stored in `./data`. Container UID/GID, proxy trust and
listen settings are covered in [configuration](configuration.md). User preferences
and quotas are managed in Settings and apply to all clients.

## Use the web interface

Sign in at the server address. On Files, use **Upload → Upload files / Upload folder**
on desktop, or the **+** button on mobile. Dropping a folder preserves its nesting
and empty directories; browser directory pickers expose directories containing files.
Existing directories are merged and duplicate filenames receive a suffix.

To share or receive files, use **⋯ beside a folder → Temporary link**:

- **View and download:** visitors browse the folder and download files, selected
  items or a ZIP archive.
- **Upload only:** visitors send files without signing in or seeing the folder's
  contents. The per-file limit is shown on the public page; it is currently 5 GiB.

Choose an expiry and select **Create link → Copy**. Revoke links from Links or the
folder's sharing dialog. Expiry and revocation block subsequent requests, including
upload chunks. Upload requests use the owner's storage quota and do not overwrite
existing files. They accept individual files; authenticated clients can upload folders.

Large browser uploads use tus chunks of up to 64 MiB and can resume after a lost
connection. Keep the tab open while uploading. A mobile browser can suspend a tab
in the background; reopen it and resume, selecting the same unchanged source if needed.

## Install the CLI and sign in

Build the same binary on the computer that will run CLI or MCP, with Go 1.27:

```bash
go build -o cloud ./cmd/cloud
export CLOUD_SERVER=https://cloud.example.com
./cloud cli login --username owner
./cloud cli ls
```

Use your account name instead of `owner`. The password prompt avoids shell history.
For a new server, `./cloud cli setup --username owner` can create the first account
instead of the browser setup form. `./cloud cli help` lists commands and
`./cloud cli help createLink` prints the exact argument schema.

Global options go before the command. `--server` overrides `CLOUD_SERVER`;
`--session` overrides `CLOUD_SESSION_FILE`. By default, a private per-server session
file is stored in the OS user configuration directory. See [CLI session details](cli.md).

The remote client needs HTTP access to the server. It does not need the server's
database, files, Docker socket or SSH credentials.

## Connect an agent over MCP

After CLI login, create a directory for agent transfers on the same computer:

```bash
mkdir -p agent-files
./cloud mcp --server https://cloud.example.com --transfer-dir /absolute/path/agent-files
```

`cloud mcp` is a stdio server: an MCP client normally starts it, sends JSON-RPC on
stdin and reads responses from stdout. Running it by itself waits for that client;
it does not open a web page or another network port.

For clients supporting `mcpServers`, use:

```json
{
  "mcpServers": {
    "cloud": {
      "command": "/absolute/path/cloud",
      "args": [
        "mcp",
        "--server", "https://cloud.example.com",
        "--transfer-dir", "/absolute/path/agent-files"
      ]
    }
  }
}
```

Replace the paths and origin with your actual values. Run CLI login and MCP as the
same OS user to share the default session, or pass the same explicit `--session`
to both. After an external login refresh, restart the MCP connection so it reloads
the session. Details and permission controls are in the [MCP guide](mcp.md).

Without a transfer directory, local uploads and downloads are unavailable; other
tools still work. Local paths are confined to that directory. Add `--read-only`
for an agent that should only inspect and download. Canonical MCP tool names and
schemas match CLI commands; short CLI aliases are not additional tools.

## Where definitions live

[`api/openapi.yaml`](../api/openapi.yaml) is the source for HTTP operations,
frontend API types, CLI command/tool names, aliases and argument schemas. CLI and
MCP load it from the binary; the web generates its types with `pnpm gen`.
The [command reference](commands.md) is generated from that same catalog and checked
by tests. Domain services on the server own business rules, limits and permissions;
clients display server values and call HTTP rather than reimplementing those rules.
