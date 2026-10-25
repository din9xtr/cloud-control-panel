# Transfer reliability

The upload service owns resumable progress; the file service owns publication and
same-name conflicts. HTTP, the web queue, CLI and MCP use the same server policy.

## Upload state

1. Create a private staging file and reserve quota for a new file.
2. Write each PATCH at the persisted offset. Discard unacknowledged trailing bytes
   before retrying, but reject a staging file shorter than the acknowledged prefix.
3. Sync staging bytes before persisting an offset. Reject an oversized or failed
   final body without advancing to the declared length.
4. Persist a ready state after the complete body has been validated and synced.
   A ready staging file is immutable; subsequent requests only retry publication.
5. Save the chosen destination in a synced publication receipt, then hard-link the
   staging file to that name without overwriting. A retry recognizes the same inode
   and does not choose another suffix.
6. Save the completed result in SQLite before removing staging and its publication
   receipt. Keep the completed upload row until its expiry, excluding it from
   active-upload limits and quota reservations. HEAD can confirm a lost final reply.

On upgrade, legacy nonempty sessions with a full offset but no validated ready
state restart at offset zero. This prevents an old failed final body from appearing
complete; partial legacy sessions keep their offsets.

HEAD retries pending publication after authentication and upload-link scope checks.
Deleting or expiring a completed receipt never removes the published file.
Expiry cleanup rechecks the current row while holding the upload lock.

New-upload admission is serialized so simultaneous requests cannot spend the same
quota. Publication checks the current quota again, including other reservations.
Already published files and unfinished reservations can briefly count twice after
a database failure; this conservatively refuses new work until recovery.

## Repeating a folder upload

Folder upload uses `conflict=skip-identical` tus metadata. The server compares every
byte at the exact destination. Identical content reuses the existing file. Different
content returns HTTP 409 and leaves the existing file unchanged. It never chooses a
suffix in this mode. Ordinary individual uploads keep their default suffix policy.

Comparisons stream through bounded buffers, with at most two comparisons active,
and release the file-mutation lock while reading. Before accepting a match, the
server checks the destination's identity, size and modification time again.

An existing same-size candidate does not reserve additional logical quota. Such a
session can only reuse an existing file: if it disappears before comparison, the
session fails instead of creating a new file without a reservation. Temporary disk
space and retransmission are still required. Empty files follow the same policy.
Upload-only public links cannot request content comparison.

Completed files and folders remain after interruption. Repeating the folder command
or selecting the directory again checks them and sends missing files. Unfinished
sessions keep their reservations until resumed, explicitly terminated or expired;
they are not a persistent directory-upload manifest. CLI errors report the session
ID for resuming an unchanged individual source.

## Downloads and operating limits

CLI and MCP download to a random `.cloud-download-*.part` file beside the destination,
sync and close it, then hard-link it to the final name without overwriting. A failed
transfer removes the temporary file; a killed process can leave it for manual
cleanup. Streaming to stdout and browser-managed downloads retain their existing
behavior.

The storage filesystem must support hard links, and staging and published files
must reside on the same filesystem. Local download destinations also require hard
links. Unsupported filesystems fail safely rather than overwrite a destination.

The supported deployment remains one server process per database and volume.
The checks cover process restarts, interrupted I/O, failed writes and database
operations, not sudden power loss or arbitrary external changes to stored files.
SQLite and directory-entry durability still depend on the configured filesystem
and operating system. Content corruption already present in an acknowledged
prefix cannot be detected solely from its length. CLI resume requires an unchanged
source file; it does not authenticate a source by content hash.

## Regression checks

- Upload tests inject interrupted and oversized bodies, a shorter staging prefix,
  failed staging access and a failure between file publication and SQLite completion.
- Recreating the upload service and reopening SQLite verify resumable ready state
  and retained completion receipts.
- Concurrent admission/publication and a quota reduction during transfer verify
  quota enforcement. Expiry tests cover a refreshed session in a stale purge batch.
- Folder tests cover identical files at full quota, equal-size different content,
  empty files, deletion during retry and concurrent same-name uploads.
- HTTP contract tests and real CLI/MCP folder uploads verify the same policy.
- Download tests verify truncated-body cleanup, existing-file preservation and the
  absence of the final name until a transfer completes.

Run `go test -race ./...`, `golangci-lint run ./...`, and the frontend test, lint,
typecheck and build commands from [development](development.md).
