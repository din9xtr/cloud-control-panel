# Configuration

The environment only describes how the server runs. Everything that can be changed in
**Settings** (account name and password, quota, trash retention, default link lifetime)
lives in the database. A fresh installation starts with no quota, 30 days of trash
retention and links that last 24 hours.

Put overrides in `.env` next to `docker-compose.yml` (see [`.env.example`](../.env.example))
and run `docker compose up -d` again.

| Variable                 | Default          | Purpose                                                          |
| :----------------------- | :--------------- | :--------------------------------------------------------------- |
| `CLOUD_PORT`             | `8080`           | Published port                                                   |
| `CLOUD_BIND`             | `127.0.0.1`      | Published address; `0.0.0.0` makes it reachable from the network |
| `CLOUD_UID`, `CLOUD_GID` | `1000`           | User and group the container runs as; they must own `./data`     |
| `CLOUD_TRUSTED_PROXIES`  | empty            | Reverse proxy addresses or CIDRs allowed to set `X-Forwarded-*`  |
| `CLOUD_SECURE_COOKIES`   | `auto`           | `auto` follows HTTPS, or force `true` / `false`                  |
| `CLOUD_LOG_LEVEL`        | `info`           | `debug`, `info`, `warn` or `error`                               |
| `CLOUD_DATA_DIR`         | `/data`          | Data root inside the container                                   |
| `CLOUD_DB_PATH`          | `/data/cloud.db` | SQLite database path                                             |
| `CLOUD_STORAGE_DIR`      | `/data/storage`  | File storage root                                                |

## Data on disk

`docker-compose.yml` mounts `./data` into the container. It holds the database and every
uploaded file, so it survives `docker compose down -v`, image rebuilds and Docker
reinstalls. To keep it elsewhere, add a `docker-compose.override.yml` with another
`volumes` entry for `/data`.

## Internet access

Put any TLS reverse proxy in front and list its address in `CLOUD_TRUSTED_PROXIES`.
Create the account before the port is reachable from the network: until the first
account exists, whoever opens the page first can claim it.
