# pg-client

Minimal PostgreSQL execution service guarded by Karasu Auth token introspection.

## Endpoint

```http
POST /execute
Authorization: Bearer <karasu_access_token>
Content-Type: application/json
```

```json
{
  "db": "karasu_auth",
  "schema": "public",
  "query": "SELECT * FROM users LIMIT 10;"
}
```

The bearer token must introspect as active and contain:

- `aud`: `pg-client-api`
- `scope`: `pg-client:execute`

## Run

```powershell
copy .env.example .env
go run ./cmd/pg-client
```

PostgreSQL credentials are configured only through environment variables. The request can choose the database and schema, but not the host/user/password.

## Docker

```powershell
docker build -t pg-client .
docker run --rm -p 8090:8090 --env-file .env pg-client
```

## Helper Script

`run.sh` creates a signed Karasu client assertion, exchanges it for a machine access token, then calls `POST /execute`.

Required `.env` values for the helper:

```env
KARASU_TOKEN_URL=http://127.0.0.1:8080/oauth/token
PG_CLIENT_URL=http://127.0.0.1:8090/execute
CLIENT_ID=app_replace_me
CLIENT_PRIVATE_KEY=./client-private.pem
TOKEN_AUDIENCE=pg-client-api
TOKEN_SCOPE=pg-client:execute
KARASU_ASSERTION_AUD=http://127.0.0.1:8080/oauth/token
```

`KARASU_ASSERTION_AUD` must match the token endpoint audience expected by Karasu, which is based on Karasu's `AUTH_BASE_URL`.

Run:

```bash
./run.sh --db "karasu" --schema "public" --query "SELECT 1;"
```
