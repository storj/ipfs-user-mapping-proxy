# IPFS User-Mapping Proxy

This is a reverse proxy that runs in front of the IPFS node's HTTP API and intercepts the requests to the following endpoints:
- /api/v0/add
- /api/v0/dag/import
- /api/v0/pin/ls
- /api/v0/pin/rm

The proxy would detect the authenticated user name and will map it to the IPFS hash of the uploaded file. The mapping is stored in a local database. Respectively, listing and removing of pinned files is scoped to the authenticated user.

## Usage

```
ipfs-proxy run --address <[host]:port> --target http://<host>:<port> --database-url <database_url>
```
Flags:
```
--address string        address to listen for incoming requests
--database-url string   database url to store user to content mappings
--target string         target url of the IPFS HTTP API to redirect the incoming requests
```

## Database Schema

```sql
CREATE TABLE IF NOT EXISTS content (
	id SERIAL PRIMARY KEY,
	username TEXT NOT NULL,                    # The authenticated user name.
	created TIMESTAMP NOT NULL DEFAULT NOW(),  # The time when the content was uploaded.
	hash TEXT UNIQUE NOT NULL,                 # The IPFS hash of the uploaded content.
	name TEXT NOT NULL,                        # The name associated with the uploaded content, usually file name.
	size BIGINT NOT NULL                       # The size of the uploaded content.
)
```
## Run With Docker

```
docker run --rm -d \
    --network host \
    -e PROXY_PORT=7070 \
    -e PROXY_TARGET=http://localhost:5001 \
    -e PROXY_DATABASE_URL=<database_url> \
    -e PROXY_LOG_FILE=/app/log/output.log \
    -e PROXY_LOG_LEVEL=info \
    -e PROXY_DEBUG_ADDR=<[host]:port> \
    storjlabs/ipfs-user-mapping-proxy:<tag>
```

Docker images are published to https://hub.docker.com/r/storjlabs/ipfs-user-mapping-proxy.

`PROXY_PORT` must be set to the port number the proxy will listen on for incoming requests.

`PROXY_TARGET` must be set to the HTTP API URL of the IPFS node. The proxy will redirect all incoming requests to this address.

`PROXY_DATABASE_URL` must be set to a Postgres or CockroachDB database URL.

If `PROXY_LOG_FILE` is not set, the logs are printed to the standard error.

`PROXY_LOG_LEVEL` sets the log level. The default level is INFO. 

`PROXY_DEBUG_ADDR` can be set to a specific `[host]:port` address to listen on for the debug endpoints. If not set, the debug endpoints will listen on a random port on the localhost.
