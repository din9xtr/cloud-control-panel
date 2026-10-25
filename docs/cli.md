# Command line client

The same `cloud` binary runs the server and a remote client. The client talks to the
HTTP API, so it can run on another machine and does not need the server's database,
storage directory or environment configuration.

## Build and connect

[Shared connection setup](clients.md) is the canonical guide for starting the
server, signing in and sharing a session between CLI and MCP.

```bash
go build -o cloud ./cmd/cloud
./cloud cli help
export CLOUD_SERVER=http://localhost:8080
./cloud cli login --username owner
Password:
```

The password prompt hides input in a terminal. A first installation can be initialized
with `./cloud cli setup --username owner` instead. Both operations save a session,
just like signing in through the web. Use HTTPS for a server outside your local machine.

Global flags go **before** the command; operation flags go after it:

```bash
./cloud cli --server https://cloud.example.com --session ./owner-session.json ls --path garden
./cloud cli --json storageUsage
./cloud cli download --help
./cloud cli help createLink
```

| Global flag | Default | Meaning |
| --- | --- | --- |
| `--server` | `CLOUD_SERVER` or `http://localhost:8080` | Server origin, without `/api/v1` |
| `--session` | `CLOUD_SESSION_FILE` or a per-server file in the user configuration directory | Session credentials |
| `--timeout` | `5m` | Timeout for each HTTP request, including each upload chunk |
| `--json` | false | Compact JSON instead of indented JSON |
| `--transfer-dir` | unset | Optional root for local upload/download paths |

On macOS the default session directory is `~/Library/Application Support/cloud`;
on Linux it follows Go's `os.UserConfigDir` (`$XDG_CONFIG_HOME` or `~/.config`).
On Windows the directory is under `%AppData%`. The filename includes a hash of the
server origin. Session files are bound to one server and contain credentials:
do not commit or share them. On Linux/macOS, files are private (`0600`); files with
group or other permissions are rejected. Restore them with `chmod 600 FILE`.
On Windows, sessions are encrypted with DPAPI for the current Windows account;
they cannot be moved to a different account or used as plaintext JSON.
The CLI never prints the session token. Sessions expire and can be revoked from any
client. Use `logout` to invalidate the current session on the server.

## A terminal workflow

```console
$ ./cloud cli mkdir --path garden
{
  "status": 201,
  "data": {
    "name": "garden",
    "path": "garden",
    "kind": "dir",
    "size": 0,
    "modifiedAt": "2026-10-25T05:00:00Z"
  }
}
$ ./cloud cli upload --input ./seed-catalog-2026.txt --folder garden
uploadId: <upload-id>
22/22 bytes
{
  "status": 204,
  "data": { "bytes": 22, "uploadId": "<upload-id>" }
}
$ ./cloud cli ls --path garden
{
  "status": 200,
  "data": {
    "path": "garden",
    "entries": [
      {
        "name": "seed-catalog-2026.txt",
        "path": "garden/seed-catalog-2026.txt",
        "kind": "file",
        "size": 22,
        "modifiedAt": "2026-10-25T05:00:00Z",
        "contentType": "text/plain; charset=utf-8"
      }
    ]
  }
}
$ ./cloud cli download --path garden/seed-catalog-2026.txt --output ./catalog-copy.txt
{
  "status": 200,
  "data": { "bytes": 22, "output": "./catalog-copy.txt" }
}
```

These are illustrative responses; timestamps, IDs, file sizes and detected media
types depend on the files. Progress and prompts go to stderr. Successful command
results go to stdout as JSON with `status`, optional `headers`, and optional `data`.
A command exits with `0` on success and `1` on argument, network or API errors.
Errors go to stderr and include the HTTP status and API error code when available.
The client does not automatically retry mutations after ambiguous network failures.
JSON API responses are limited to 8 MiB; file and archive downloads stream without
that limit.

## Files, links and trash

```bash
./cloud cli search --q catalog --path garden
./cloud cli rename --path garden/seed-catalog-2026.txt --name catalog.txt
./cloud cli mkdir --path archive
./cloud cli mv --paths '["garden/catalog.txt"]' --destination archive
./cloud cli createLink --path archive/catalog.txt --ttlSeconds 3600
./cloud cli listLinks
./cloud cli revokeLink --id LINK_ID
./cloud cli downloadArchive --path '["garden","archive"]' --output ./garden.zip
./cloud cli rm --paths '["archive/catalog.txt"]'
./cloud cli listTrash
./cloud cli restoreTrashItem --id TRASH_ID
```

Paths are relative to the remote user's storage root. Array arguments are JSON
arrays, booleans are `true`/`false`, and numbers are JSON numbers. A flag takes a
value even when its API type is boolean, for example `--inline true`.

`rm` moves entries to trash. `deleteTrashItem --id ID` and `emptyTrash` delete
permanently. Those explicit commands execute immediately, with the same rights
and retention rules as the web interface.

Downloads stream to a new local file and refuse to overwrite an existing one.
An incomplete destination is removed if the transfer fails normally. A force-killed
process may leave a partial file; remove it or choose a new output name before retrying.
Use `--output -` to stream file or ZIP bytes to stdout; the JSON result then goes
to stderr:

```bash
./cloud cli download --path archive/catalog.txt --output - > ./catalog-copy.txt
```

## Resumable uploads

`upload` / `uploadFile` streams a regular local file in chunks of up to 4 MiB. It
prints the upload ID before sending chunks. After an interruption, pass that ID:

```bash
./cloud cli upload --input ./seed-catalog-2026.txt --folder garden
./cloud cli upload --input ./seed-catalog-2026.txt --uploadId UPLOAD_ID
./cloud cli tusStatus --id UPLOAD_ID
./cloud cli tusTerminate --id UPLOAD_ID
```

Resume checks the server's offset and total length, then seeks to that offset in
the local file. The source must be **the same unchanged file**; equal length alone
does not prove equal content. Do not reuse an upload ID for another file. The
original upload determines its remote folder and name. Completed or expired uploads
no longer have an active upload ID. Zero-byte files are published during creation.

## Account, settings and scripts

```bash
./cloud cli whoami
./cloud cli listSessions
./cloud cli revokeSession --id SESSION_ID
./cloud cli storageUsage
./cloud cli getSettings
./cloud cli updateSettings --quotaBytes 10737418240 --trashRetentionDays 14 --defaultLinkTtlSeconds 3600
./cloud cli logout
```

Settings updates send the complete settings object. For credentials or other
sensitive arguments, read a JSON object from stdin or a private file instead of
putting passwords in shell history and process arguments:

```bash
./cloud cli changeCredentials --data-file ./private-credentials.json
./cloud cli login --data-file - < ./private-login.json
./cloud cli --json ls --path garden | jq '.data.entries[] | {path, size}'
```

`--data-file` supplies the flat operation arguments; explicit flags override its
fields. The file is limited to 2 MiB. Integer literals retain int64 precision.

## Complete command reference

Every `operationId` in `api/openapi.yaml` is a CLI command. `help COMMAND` prints
its JSON Schema, and `COMMAND --help` describes its flags. The contract is embedded
in the binary, so rebuild the client after updating it.

See the [generated command and tool reference](commands.md) for every canonical
name, CLI alias and required argument. Its source is `api/openapi.yaml`; the same
catalog drives CLI help and MCP tool discovery.

Low-level tus calls use the contract's header names as flags (`--Upload-Length`,
`--Upload-Metadata`, `--Upload-Offset`). `Tus-Resumable: 1.0.0` is supplied
automatically. `tusWrite --data BASE64` accepts up to 1 MiB decoded per call.
Prefer `uploadFile` for ordinary file transfers.

## Folder links and file requests

Create a browsing link or an upload-only request for an existing folder:

```bash
./cloud cli createLink --path garden --mode download --ttlSeconds 86400
./cloud cli createLink --path garden --mode upload --ttlSeconds 86400
```

Folder URLs use `/share/TOKEN`. Public commands accept the `TOKEN` portion and do
not require login. All paths are relative to the shared folder:

```bash
./cloud cli publicFolder --token TOKEN
./cloud cli publicFolder --token TOKEN --path greenhouse
./cloud cli publicFolderContent --token TOKEN --path greenhouse/seeds.txt --output ./seeds.txt
./cloud cli publicFolderArchive --token TOKEN --output ./garden.zip
./cloud cli publicFolderArchive --token TOKEN --path '["greenhouse","catalog.txt"]' --output ./selection.zip
./cloud cli uploadFile --token UPLOAD_TOKEN --input ./seed-catalog-2026.txt
./cloud cli uploadFile --token UPLOAD_TOKEN --input ./seed-catalog-2026.txt --uploadId UPLOAD_ID
```

Upload-only links do not list or download existing files. Their destination is
fixed: omit `--folder`. Each incoming file is limited to 5 GiB and uses the owner's
storage quota. Revoked or expired links reject subsequent requests, including
upload chunks. Existing names receive a suffix instead of being overwritten.
Low-level commands are `publicTusOptions`, `publicTusCreate`, `publicTusStatus`,
`publicTusWrite`, and `publicTusTerminate`; they also require `--token`.

## Upload a directory

```bash
./cloud cli uploadFolder --input ./garden --folder documents
./cloud cli uploadFolder --input ./garden --folder documents --name spring-garden
```

The remote parent must exist. The command creates the top-level folder and nested
folders, including empty ones, then streams files through tus. Existing directories
are merged, duplicate filenames receive suffixes, and symlinks are skipped.
`uploadFolder` requires a signed-in session; file requests accept individual files.
Failures leave completed files and directories in place. Repeating the entire
command can create duplicates. An interrupted file reports an upload ID which can
be resumed separately with `uploadFile`, the same unchanged input, and `--uploadId`.
