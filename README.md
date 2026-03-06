# Tablić

A multiplayer web implementation of Tablić — a Serbian fishing card game.

Play in your browser, no installation needed. Supports **2-player** and **4-player team** (2v2) modes.

---

## How to play

Players take turns playing one card from their hand. If your card matches the value of a card on the table — or the **sum** of multiple cards — you capture them. Clear the whole table and score a **tabla** (bonus point). First to **101 points** wins.

Full rules: [rules.md](rules.md)

---

## Features

- Real-time multiplayer via WebSockets
- 2-player and 4-player 2v2 team mode
- Reconnect with token — rejoin after a dropped connection
- 30-second auto-skip for disconnected players
- Game history with per-round breakdown
- Single binary deployment (client embedded in server)

---

## Tech stack

| Layer | Technology |
|-------|-----------|
| Backend | Go — `gorilla/websocket`, `modernc.org/sqlite` |
| Frontend | TypeScript, PixiJS v8, GSAP |
| Build | Vite (client), `go build` (server) |

---

## Run locally

**Requirements:** Go 1.21+, Node 20+

```bash
# Terminal 1 — backend
cd server
go run main.go          # listens on :3579

# Terminal 2 — frontend
cd client
npm install
npm run dev             # listens on :3000, proxies /ws and /api to :3579
```

Open http://localhost:3000

---

## Deploy

See [DEPLOY.md](DEPLOY.md) for a full Hetzner self-hosting guide (Docker + Caddy + HTTPS).

Quick build for production:

```bash
chmod +x build.sh
./build.sh              # produces server/tablic — single binary, no runtime deps
```

---

## Tests

```bash
cd server && go test ./...
```

83 tests covering game logic, protocol serialization, and room management.

---

## License

[MIT](LICENSE)
