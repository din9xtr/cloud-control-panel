# Security

- Every file operation goes through `os.Root`, so paths and symlinks cannot escape the user folder.
- Sessions are random 256-bit tokens; only their SHA-256 hashes are stored.
  Cookies are `HttpOnly`, `SameSite=Strict` and `Secure` over HTTPS.
- Cross-origin writes are rejected; responses carry a strict Content Security Policy.
- Sign-in is throttled per address and per account, with constant-time handling of unknown users.
  Lockouts cannot be flushed out by flooding the limiter with new keys, and changing the
  password with a wrong current password counts against the same account limit.
- Every client gets a request rate and concurrency budget; sign-in, first-run setup and public
  links have stricter budgets. IPv6 clients are grouped by `/64`, so rotating addresses inside
  one network does not reset the budget. When the server is saturated it sheds load with `503`
  instead of queueing.
- Slow request bodies are cut off, uploads must keep making progress, and password hashing,
  search and ZIP archives run in bounded slots so they cannot exhaust the CPU.
- File names with bidirectional control characters are rejected to prevent extension spoofing.
  A rename never replaces another file, including one whose name differs only in letter case.
- Downloads are served as attachments; only images, audio, video, PDF and plain text
  open inline, inside a sandbox.
- Logs record route patterns, status codes and client addresses. File names and paths are
  removed from error messages before they are logged.
- The container runs as a non-root user on a read-only filesystem with all capabilities dropped.
- Until the first account exists the app accepts only the setup form; publish it to the
  network after the account is created.
