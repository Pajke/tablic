# Tablić — Server

Go WebSocket server for the card game Tablić (a Serbian fishing card game).

## Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.25 |
| WebSocket | `github.com/gorilla/websocket` |
| Persistence | SQLite (`modernc.org/sqlite` — pure Go, no cgo) |
| Static files | `embed.FS` (serves built Vite client) |

## Project Layout

```
server/
├── main.go                  # Entry point: HTTP mux, embed, env vars
├── static/                  # Built client (git: .gitkeep; populated by build.sh)
└── internal/
    ├── game/                # Pure game logic (no I/O)
    │   ├── types.go         # Card, Player, GameState, CaptureOption, RoundScore
    │   ├── deck.go          # NewDeck(), Shuffle()
    │   ├── capture.go       # ComputeCaptures() — all capture option computation
    │   ├── scoring.go       # ComputeRoundScores(), ComputeTeamRoundScores()
    │   └── state.go         # GameState methods: deal, capture, discard, win check
    ├── protocol/            # JSON wire types for client ↔ server messages
    │   └── protocol.go      # ParseClientMessage(), MustMarshal(), all msg structs
    ├── room/                # Room and connection management
    │   ├── room.go          # Room: join, reconnect, turn flow, auto-skip timer
    │   └── manager.go       # Manager: create/get/remove rooms
    ├── storage/             # SQLite persistence (nil-safe — game runs without it)
    │   └── storage.go       # Open(), RecordGameStart/Round/End
    └── ws/                  # WebSocket upgrade and message loop
        └── handler.go       # Handshake (CREATE_ROOM, JOIN_ROOM, RECONNECT), read loop
```

## Running Locally

```bash
# from server/
go run main.go
# Listens on :3579 (PORT env var overrides)
# SQLite DB created at ./tablic.db (DB_PATH env var overrides)
```

The client dev server (Vite on :3000) proxies `/ws` to `:3579`. For a full
local build that bundles the client into the binary:

```bash
# from repo root
./build.sh
./server/tablic   # single binary, serves everything on :3579
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3579` | HTTP listen port |
| `DB_PATH` | `tablic.db` | Path to SQLite database file |

## Testing

```bash
go test ./...
# or with coverage:
go test ./... -cover
```

Current coverage:
- `game`: ~93% — all capture logic, scoring, state mutations
- `protocol`: ~94% — all message types, parse errors
- `room`: ~40% — join/reconnect lifecycle, turn message routing

Run the race detector during development:

```bash
go test -race ./...
```

## WebSocket Protocol

### Handshake (first message from client)

| Message | Direction | Description |
|---------|-----------|-------------|
| `CREATE_ROOM` | C→S | Create a new room (`maxPlayers`: 2 or 4) |
| `JOIN_ROOM` | C→S | Join an existing room by `roomId` |

After a successful join the server sends `ROOM_JOINED` with a `reconnectToken`.
When all seats fill, `GAME_STARTED` + `HAND_DEALT` + `TURN_START` are broadcast.

### In-game (client → server)

| Message | Fields | Description |
|---------|--------|-------------|
| `PLAY_CARD` | `cardId` | Play a card from hand |
| `CHOOSE_CAPTURE` | `optionIndex` | Select a capture combination |
| `PING` | — | Keep-alive (server replies `PONG`) |

### In-game (server → client)

| Message | Description |
|---------|-------------|
| `TURN_START` | `playerIndex` of current player |
| `CARD_PLAYED` | Card lifted off the table |
| `CARD_DISCARDED` | Card placed on table (no capture) |
| `CAPTURE_OPTIONS` | Player must choose among multiple capture sets |
| `CAPTURE_MADE` | Capture resolved; `wasTabla` if table cleared |
| `ROUND_END` | Round scores |
| `GAME_OVER` | Winner + final standings |
| `PLAYER_DISCONNECTED` | A player's connection dropped |
| `ERROR` | `code` + `message` |

## Key Design Decisions

### Two-phase capture flow
When a player plays a card:
- 0 capture options → auto-discard
- 1 capture option → auto-capture (no round-trip needed)
- 2+ options → send `CAPTURE_OPTIONS`, wait for `CHOOSE_CAPTURE`

### 4-player team mode (2v2)
Seats 0, 2 = Team A; seats 1, 3 = Team B. Team members share captured piles
for scoring. Both teammates receive identical `RoundScore.Total`.

### Reconnect & auto-skip
Each player receives a `reconnectToken` on join. If the current player
disconnects, a 30-second `time.AfterFunc` fires and auto-discards their first
hand card (`autoSkipTurn`). Reconnecting before the timer cancels it.

### Deterministic seat order
Players are stored in a `seatOrder []string` slice (append order) to avoid
non-deterministic `map` iteration when assigning `SeatIndex`.

### Per-player write goroutine
Each connection has a buffered `writeCh chan []byte` and a dedicated
`writeLoop` goroutine. The main mutex is never held across I/O.

### SQLite is optional
`storage.Storage` is nil-guarded throughout. The server runs fine without a
database (useful for local dev without a writable filesystem).
