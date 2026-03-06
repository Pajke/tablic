# Tablić — Server Dev Log

Chronological notes on design decisions and bugs encountered during development.

---

## Phase 1 — Core Game Engine

**Goal**: Pure Go library for Tablić rules with no I/O.

### Capture logic (`capture.go`)

Tablić capture rules are deceptively complex:

- Numeric cards (2–10) can be captured by **exact rank** or **sums** of table cards.
- Ace is dual-value: worth 1 or 11 — whichever produces more captures.
- Face cards (J, Q, K) can only be captured by **exact rank match** (no sums).
  - However J (12), Q (13), K (14) can still be captured via numeric sums
    (e.g. 9+3=12 captures J).
- Multiple non-overlapping capture groups can be taken in one move.

Implementation: `FindSubsets` generates all subsets of table cards that sum to
the target value. `FindCaptureCombinations` then finds all non-overlapping
combinations of subsets. `ComputeCaptures` wraps both and handles the Ace
dual-value logic.

### Scoring (`scoring.go`)

Scoring cards: Ace (1pt each), 10♦ (+1 extra = 2pt total), 2♣ (1pt), all
face cards (1pt each), all other 10s (1pt each). Total card points across a
full deck: 22.

Špil (most cards): 3 points to the player with more captured cards; 0 if tied.

Tabla: +1 point each time a capture clears the table entirely.

Grand total possible per round: 22 (cards) + 3 (špil) + tablas.

### State machine (`state.go`)

`GameState` is the single source of truth. Key methods:

- `DealNextHand()` — first deal places 4 cards on the table; subsequent deals do not.
- `ApplyCapture()` — removes played card from hand, captured cards from table,
  updates `LastCapturerIndex`, detects tabla.
- `ApplyLastHandRule()` — at round end, remaining table cards go to the last
  player who made a capture. No-op if nobody captured yet.
- `CheckWinCondition()` — returns index of winner (highest scorer ≥ 101);
  −1 if no winner yet. Handles simultaneous 101+ via tiebreak.

---

## Phase 2 — WebSocket Server

**Goal**: Real-time multiplayer using a single room abstraction.

### Room lifecycle

`Manager` holds a map of active rooms. Rooms are created via `CREATE_ROOM`
and joined via `JOIN_ROOM`. When the room fills (`PlayerCount == maxPlayers`)
the WebSocket handler calls `StartGame()` then `BroadcastGameStart()`.

### Per-player write goroutine

Early prototype wrote directly to `websocket.Conn` inside the room's mutex.
This caused deadlocks when the write blocked. Solution: each player gets a
buffered `writeCh chan []byte` (size 32) and a dedicated `writeLoop` goroutine.
The mutex is never held across a write call.

### Non-deterministic seat order — bug

**Problem**: `for id := range r.conns` (a `map`) in `StartGame()` iterates in
random order. In 4-player mode this made team assignments random across restarts.

**Fix**: Added `seatOrder []string` to `Room`. `Join()` appends each player ID
in arrival order. `StartGame()` iterates `seatOrder` instead of the map.

### Two-phase capture flow

When `handlePlayCard` computes 2+ capture options it stores them in
`pendingCapture` and sends `CAPTURE_OPTIONS` to the player. `PLAY_CARD`
messages are rejected while a capture choice is pending.

### Dangling pointer after `ApplyDiscard` — bug

**Problem**: `playedCard = &cur.Hand[i]` stores a pointer into the slice.
`ApplyDiscard` then shifts `cur.Hand` via `append(hand[:i], hand[i+1:]...)`,
making the pointer stale. Broadcasting `CARD_DISCARDED` with `*playedCard`
would send the wrong card.

**Fix**: Copy the card value before calling `ApplyDiscard`:
```go
discardedCard := *playedCard
r.state.ApplyDiscard(...)
r.broadcast(CardDiscardedMsg{Card: discardedCard})
```

Same pattern applied in `autoSkipTurn`:
```go
card := cur.Hand[0]  // value copy
r.state.ApplyDiscard(playerID, card.ID)
```

---

## Phase 3 — Persistence (SQLite)

**Goal**: Record game history without blocking game play.

Used `modernc.org/sqlite` — a pure-Go SQLite driver with no cgo dependency,
which simplifies Docker builds and cross-compilation.

`storage.Storage` is nilable: callers guard every call with `if r.storage != nil`.
This lets the server start cleanly if the DB file is unwritable.

Schema: three tables — `games`, `game_players`, `game_rounds`. All writes use
`OR REPLACE` / `UPDATE` to be idempotent on retry.

---

## Phase 4 — 4-Player Team Mode (2v2)

**Goal**: Support 2-versus-2 with shared team scoring.

### Team scoring

Seats 0, 2 = Team A; seats 1, 3 = Team B. `ComputeTeamRoundScores` aggregates
captured cards and tabla counts per team. Both teammates receive the same
`RoundScore.Total` so that `ApplyRoundScores` gives them equal `TotalScore`.
`CheckWinCondition` then works unchanged — the two teammates are identical
in score so the one at the lower index is returned as "winner" (display only).

---

## Phase 5 — Reconnect & Auto-Skip

**Goal**: Games shouldn't stall if a player disconnects.

### Design

- `reconnectToken` issued to each player on `ROOM_JOINED`.
- `Reconnect(token)` looks up the player and returns their ID + seat.
- `AttachConn` replaces the `*websocket.Conn` and resets the `done` channel.

### Turn timer

When the active player is offline at `TURN_START`, a `time.AfterFunc(30s)`
goroutine fires and calls `autoSkipTurn`. The goroutine acquires `r.mu` before
touching state — safe because `AfterFunc` runs in its own goroutine, not
inside the existing mutex hold.

`autoSkipTurn` discards the player's first hand card (no table scan; the
penalty for being disconnected is forfeiting capture opportunities).

If the player reconnects before the timer fires, `AttachConn` calls
`cancelTurnTimer()` inside the mutex so there is no race.

### Client-side disconnection indicator

`PLAYER_DISCONNECTED` messages are tracked in `disconnectedPlayers: Set<string>`.
The status bar shows "Waiting for X to reconnect… (auto-skip in ~30s)" while
the current player is in that set. The set is cleared on each `TURN_START`.

---

## Phase 6 — Hosting (Fly.io)

**Goal**: Single-binary deploy, friends can play over the internet.

### Approach

- `//go:embed all:static` bundles the built Vite client into the Go binary.
- `build.sh` runs `npm ci && npm run build` then copies `client/dist/` into
  `server/static/` before `go build`.
- `spaHandler` serves static files and falls back to `index.html` for unknown
  paths (required for client-side routing).
- `PORT` and `DB_PATH` env vars allow Fly.io to inject the correct socket and
  mount point without rebuilding.
- A persistent Fly volume (`tablic_data`) is mounted at `/data` so SQLite
  survives redeploys.

### Multi-stage Dockerfile

```
node:20-alpine  → build client  → /client/dist
golang:1.25     → embed dist, go build  → /tablic binary
alpine:latest   → copy binary, expose 8080
```

Total image size: ~20 MB.

---

## Test Coverage Summary

| Package | Tests | Coverage |
|---------|-------|----------|
| `game` | 57 | ~93% |
| `protocol` | 9 | ~94% |
| `room` | 11 | ~40% |
| `storage` | — | 0% (requires real FS) |
| `ws` | — | 0% (requires real WS) |

Room coverage is limited by WebSocket dependency — `writeLoop` and the full
`advanceTurnOrDeal` paths are exercised end-to-end but not in unit tests.
Storage and WS handlers are best covered by integration/e2e tests.
