# bad-genderize

A tiny Go API that guesses a name's gender using an LLM and caches the answer
in Redis so no name is ever sent to the model twice.

```
GET /api/v1/gender?name=Hello%20World
→ {"gender": "male", "confidence": 0.31}   // 31% confidence
```

## How it works

1. The name is normalized (trimmed + lowercased) into a stable cache key.
2. If Redis has the key, the cached result is returned immediately.
3. Otherwise the name is sent to an LLM through your OpenAI-compatible proxy
   (`POST {LLM_PROXY_URL}/chat/completions`). The model is asked to return a
   strict JSON object with a `gender` and a `confidence` in `[0,1]`.
4. The result is stored in Redis and returned to the caller.

The service is resilient: if Redis is down it still answers (uncached), and
anything that isn't `male`/`female` from the model is rejected with a 502.

## Running

```sh
# local (needs Go 1.27+ and a reachable Redis)
go build -o gender-api .
./gender-api
```

```sh
# or via Docker Compose (starts Redis + the API)
docker compose up --build
```

The LLM proxy is documented at `http://localhost:8080/llms.txt`. It speaks the
OpenAI Chat Completions protocol with base URL `http://localhost:8080/v1` and
**requires an API key** sent as a Bearer token (`Authorization: Bearer sk-gw-…`).
Set it via `LLM_API_KEY`.

## Configuration

Everything is read from the environment (see `.env.example`):

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_ADDR` | `:8081` | Address the API listens on |
| `REDIS_ADDR` | `localhost:6379` | Redis host:port |
| `REDIS_USERNAME` | *(empty)* | Redis ACL username |
| `REDIS_PASSWORD` | *(empty)* | Redis password |
| `REDIS_DB` | `0` | Redis database number |
| `REDIS_TLS` | `false` | Enable TLS to Redis |
| `CACHE_TTL` | `0` | Cache entry lifetime (`0` = forever) |
| `LLM_PROXY_URL` | `http://localhost:8080/v1` | OpenAI-compatible base URL (the part before `/chat/completions`) |
| `LLM_MODEL` | `gpt-4o-mini` | Model id sent to the proxy |
| `LLM_API_KEY` | *(empty)* | Bearer API key for the proxy (required by the gateway) |
| `LLM_TIMEOUT` | `30s` | Per-call timeout to the LLM |

> **The LLM proxy URL is fully configurable** (`LLM_PROXY_URL`) so you can point
> it at any host — not just localhost.

## API

- `GET /api/v1/gender?name=<name>` → `200 {"gender":"male|female","confidence":0.0..1.0}`
- Missing/empty `name` → `400 {"error":"..."}`
- LLM failure → `502 {"error":"..."}`

## Tests

```sh
go test ./...
```
