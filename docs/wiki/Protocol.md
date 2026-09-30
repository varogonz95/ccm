# Protocol

The agent speaks JSON over HTTP plus one WebSocket endpoint. The Go types live in `internal/api/types.go`. Protocol version: `0.1.0`.

## Auth

Every endpoint except `/v1/health` requires:

```
Authorization: Bearer <token>
```

Failures return `401` with `{"error":"unauthorized"}`. All errors use `{"error":"<message>"}`.

## Endpoints

| Method | Path | |
|---|---|---|
| `GET` | `/v1/health` | `{"host","os","version"}`; no auth |
| `GET` | `/v1/sessions` | Array of sessions, oldest first |
| `POST` | `/v1/sessions` | Create a session |
| `DELETE` | `/v1/sessions/{id}` | Stop and remove a session |
| `GET` | `/v1/sessions/{id}/attach` | WebSocket upgrade |

`{id}` may be a unique prefix. An ambiguous prefix or unknown ID is an error.

### Session

```json
{
  "id": "815856a6",
  "name": "api",
  "dir": "/home/alvaro/src/api",
  "args": ["--resume"],
  "pid": 41233,
  "created": "2026-09-30T10:12:00Z",
  "status": "running",
  "viewers": 1
}
```

`status` is `running` or `exited`. `exit_code` is present only once exited; `args` only when non-empty.

### Create request

```json
{ "name": "api", "dir": "~/src/api", "args": ["--resume"], "cols": 120, "rows": 40 }
```

All fields are optional. `dir` defaults to the agent user's home and `~` is expanded on the agent. Size defaults to 120×40. There is no field for the executable; that is fixed by the agent's `--claude` flag.

## Attach WebSocket

- **Binary frames** carry raw terminal bytes in both directions.
- **Text frames** carry JSON control messages:

| `type` | Direction | Fields |
|---|---|---|
| `resize` | hub → agent | `cols`, `rows` |
| `exit` | agent → hub | `code` |

On connect the agent sends the scrollback as binary, then live output. When the process ends it sends `exit` and closes normally. The agent pings every 20 s and drops a client silent for 60 s.
