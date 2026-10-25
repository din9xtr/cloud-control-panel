# Command and tool reference

Generated from [api/openapi.yaml](../api/openapi.yaml). Do not edit this table by hand.
Regenerate with `go run ./cmd/cloud-docs`; verify with `go run ./cmd/cloud-docs --check`.

Canonical names are both CLI commands and MCP tools. Aliases are CLI-only.
HTTP operations use `/api/v1` as their prefix. Local transfer helpers call the HTTP API.
All arguments are named CLI flags or flat MCP arguments.

| Command / MCP tool | CLI aliases | Required arguments | Optional arguments | Operation |
| --- | --- | --- | --- | --- |
| `changeCredentials` | — | `currentPassword` | `newPassword`, `username` | `PUT /account/credentials` |
| `createFolder` | mkdir | `path` | — | `POST /files/folders` |
| `createLink` | — | `path` | `mode`, `ttlSeconds` | `POST /links` |
| `deleteTrashItem` | — | `id` | — | `DELETE /trash/{id}` |
| `downloadArchive` | — | `output`, `path` | — | `GET /files/archive` |
| `downloadFile` | download | `output`, `path` | `inline` | `GET /files/content` |
| `emptyTrash` | — | — | — | `DELETE /trash` |
| `getSettings` | — | — | — | `GET /settings` |
| `listFolder` | ls | — | `path` | `GET /files` |
| `listLinks` | — | — | — | `GET /links` |
| `listSessions` | — | — | — | `GET /account/sessions` |
| `listTrash` | — | — | — | `GET /trash` |
| `login` | — | `password`, `username` | — | `POST /auth/login` |
| `logout` | — | — | — | `POST /auth/logout` |
| `me` | whoami | — | — | `GET /auth/me` |
| `moveEntries` | mv | `destination`, `paths` | — | `POST /files/move` |
| `publicFolder` | — | `token` | `path` | `GET /public/links/{token}` |
| `publicFolderArchive` | — | `output`, `token` | `path` | `GET /public/links/{token}/archive` |
| `publicFolderContent` | — | `output`, `token` | `path` | `GET /public/links/{token}/content` |
| `publicTusCreate` | — | `Upload-Length`, `Upload-Metadata`, `token` | — | `POST /public/links/{token}/uploads` |
| `publicTusOptions` | — | `token` | — | `OPTIONS /public/links/{token}/uploads` |
| `publicTusStatus` | — | `id`, `token` | — | `HEAD /public/links/{token}/uploads/{id}` |
| `publicTusTerminate` | — | `id`, `token` | — | `DELETE /public/links/{token}/uploads/{id}` |
| `publicTusWrite` | — | `Upload-Offset`, `data`, `id`, `token` | — | `PATCH /public/links/{token}/uploads/{id}` |
| `renameEntry` | rename | `name`, `path` | — | `POST /files/rename` |
| `restoreTrashItem` | — | `id` | — | `POST /trash/{id}/restore` |
| `revokeLink` | — | `id` | — | `DELETE /links/{id}` |
| `revokeSession` | — | `id` | — | `DELETE /account/sessions/{id}` |
| `searchFiles` | search | `q` | `path` | `GET /files/search` |
| `setup` | — | `password`, `username` | — | `POST /setup` |
| `setupStatus` | — | — | — | `GET /setup` |
| `storageUsage` | — | — | — | `GET /storage` |
| `trashEntries` | rm | `paths` | — | `POST /files/trash` |
| `tusCreate` | — | `Upload-Length`, `Upload-Metadata` | — | `POST /uploads` |
| `tusOptions` | — | — | — | `OPTIONS /uploads` |
| `tusStatus` | — | `id` | — | `HEAD /uploads/{id}` |
| `tusTerminate` | — | `id` | — | `DELETE /uploads/{id}` |
| `tusWrite` | — | `Upload-Offset`, `data`, `id` | — | `PATCH /uploads/{id}` |
| `updateSettings` | — | `defaultLinkTtlSeconds`, `quotaBytes`, `trashRetentionDays` | — | `PUT /settings` |
| `uploadFile` | upload | `input` | `conflict`, `folder`, `name`, `token`, `uploadId` | Local transfer helper |
| `uploadFolder` | — | `input` | `folder`, `name` | Local transfer helper |

Use `cloud cli help COMMAND` for its complete JSON Schema, `cloud cli COMMAND --help` for flags,
or MCP tool discovery for the same schema. See [connection setup](clients.md), [CLI examples](cli.md)
and [MCP configuration](mcp.md).
