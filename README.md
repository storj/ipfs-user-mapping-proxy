# IPFS User-Mapping Proxy

This is a reverse proxy that runs in front of the IPFS node's HTTP API and intercepts the requests to the `/api/v0/add` endpoint.

The proxy would detect the authenticated user name and will map it to the IPFS hash of the uploaded file. The mapping is stored in a local Postgres database.

## Usage

```
ipfs-proxy run --address <host:port> --target http://<host>:<port> --database-url <database_url>
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
	username TEXT NOT NULL,     # The authenticated user name.
	hash TEXT UNIQUE NOT NULL,  # The IPFS hash of the uploaded content.
	name TEXT NOT NULL,         # The name associated with the uploaded content, usually file name.
	size BIGINT NOT NULL        # The size of the uploaded content.
)
```
## Run With Docker

```
docker run --rm -d \
    --network host \
    -e PROXY_PORT=7070 \
    -e PROXY_TARGET=http://localhost:5001 \
    -e PROXY_DATABASE_URL=<database_url> \
    kaloyanraev/ipfs-user-mapping-proxy
```

`PROXY_PORT` must be set to the port number the proxy will listen for incoming requests.

`PROXY_TARGET` must be set to the HTTP API URL of the IPFS node. The proxy will redirect all incoming requests to this address.
