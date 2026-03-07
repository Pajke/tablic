package storage

import (
	"database/sql"
	"log"
	"time"

	_ "modernc.org/sqlite"

	"tablic/server/internal/game"
)

const schema = `
CREATE TABLE IF NOT EXISTS reconnect_tokens (
  token       TEXT PRIMARY KEY,
  player_id   TEXT NOT NULL,
  player_name TEXT NOT NULL,
  room_id     TEXT NOT NULL,
  seat_index  INTEGER NOT NULL,
  expires_at  DATETIME NOT NULL
);
CREATE TABLE IF NOT EXISTS games (
  id          TEXT PRIMARY KEY,
  room_id     TEXT NOT NULL,
  started_at  DATETIME NOT NULL,
  finished_at DATETIME,
  winner_name TEXT
);
CREATE TABLE IF NOT EXISTS game_players (
  game_id     TEXT NOT NULL,
  player_id   TEXT NOT NULL,
  player_name TEXT NOT NULL,
  seat_index  INTEGER NOT NULL,
  final_score INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (game_id, player_id)
);
CREATE TABLE IF NOT EXISTS game_rounds (
  game_id      TEXT NOT NULL,
  round_number INTEGER NOT NULL,
  player_id    TEXT NOT NULL,
  card_points  INTEGER NOT NULL,
  spil_points  INTEGER NOT NULL,
  tabla_points INTEGER NOT NULL,
  round_total  INTEGER NOT NULL,
  PRIMARY KEY (game_id, round_number, player_id)
);
`

// Storage wraps a SQLite database for recording game history.
type Storage struct {
	db *sql.DB
}

// Open opens (or creates) a SQLite database at path and initialises the schema.
func Open(path string) (*Storage, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	s := &Storage{db: db}
	s.cleanExpiredTokens()
	return s, nil
}

// TokenRecord holds a persisted reconnect token.
type TokenRecord struct {
	PlayerID   string
	PlayerName string
	RoomID     string
	SeatIndex  int
}

// WriteReconnectToken persists a reconnect token with a 24-hour TTL.
func (s *Storage) WriteReconnectToken(token, playerID, playerName, roomID string, seatIndex int) {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO reconnect_tokens (token, player_id, player_name, room_id, seat_index, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		token, playerID, playerName, roomID, seatIndex, time.Now().UTC().Add(24*time.Hour),
	)
	if err != nil {
		log.Printf("[storage] WriteReconnectToken: %v", err)
	}
}

// GetReconnectToken looks up a token. Returns nil if not found or expired.
func (s *Storage) GetReconnectToken(token string) *TokenRecord {
	row := s.db.QueryRow(
		`SELECT player_id, player_name, room_id, seat_index
		 FROM reconnect_tokens WHERE token = ? AND expires_at > ?`,
		token, time.Now().UTC(),
	)
	var t TokenRecord
	if err := row.Scan(&t.PlayerID, &t.PlayerName, &t.RoomID, &t.SeatIndex); err != nil {
		return nil
	}
	return &t
}

// DeleteRoomTokens removes all tokens for a given room (call on game over).
func (s *Storage) DeleteRoomTokens(roomID string) {
	if _, err := s.db.Exec(`DELETE FROM reconnect_tokens WHERE room_id = ?`, roomID); err != nil {
		log.Printf("[storage] DeleteRoomTokens: %v", err)
	}
}

func (s *Storage) cleanExpiredTokens() {
	if _, err := s.db.Exec(`DELETE FROM reconnect_tokens WHERE expires_at <= ?`, time.Now().UTC()); err != nil {
		log.Printf("[storage] cleanExpiredTokens: %v", err)
	}
}

// RecordGameStart inserts a new game row and its players.
func (s *Storage) RecordGameStart(gameID, roomID string, players []game.Player) {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO games (id, room_id, started_at) VALUES (?, ?, ?)`,
		gameID, roomID, time.Now().UTC(),
	)
	if err != nil {
		log.Printf("[storage] RecordGameStart: %v", err)
		return
	}
	for _, p := range players {
		_, err := s.db.Exec(
			`INSERT OR IGNORE INTO game_players (game_id, player_id, player_name, seat_index) VALUES (?, ?, ?, ?)`,
			gameID, p.ID, p.Name, p.SeatIndex,
		)
		if err != nil {
			log.Printf("[storage] RecordGameStart player: %v", err)
		}
	}
}

// RecordRound inserts per-player round scores.
func (s *Storage) RecordRound(gameID string, roundNum int, scores []game.RoundScore) {
	for _, sc := range scores {
		_, err := s.db.Exec(
			`INSERT OR REPLACE INTO game_rounds (game_id, round_number, player_id, card_points, spil_points, tabla_points, round_total)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			gameID, roundNum, sc.PlayerID, sc.CardPoints, sc.SpilPoints, sc.TablaPoints, sc.Total,
		)
		if err != nil {
			log.Printf("[storage] RecordRound: %v", err)
		}
	}
}

// --- History query types ---

// GameRecord is a completed game with players and per-round scores.
type GameRecord struct {
	ID         string         `json:"id"`
	StartedAt  string         `json:"startedAt"`
	FinishedAt string         `json:"finishedAt"`
	WinnerName string         `json:"winnerName"`
	Players    []PlayerRecord `json:"players"`
	Rounds     []RoundRecord  `json:"rounds"`
}

// PlayerRecord holds a player's final result in one game.
type PlayerRecord struct {
	Name       string `json:"name"`
	SeatIndex  int    `json:"seatIndex"`
	FinalScore int    `json:"finalScore"`
}

// RoundRecord holds all players' scores for one round.
type RoundRecord struct {
	Number int                `json:"number"`
	Scores []RoundScoreRecord `json:"scores"`
}

// RoundScoreRecord is one player's breakdown for one round.
type RoundScoreRecord struct {
	PlayerName  string `json:"playerName"`
	CardPoints  int    `json:"cardPoints"`
	SpilPoints  int    `json:"spilPoints"`
	TablaPoints int    `json:"tablaPoints"`
	Total       int    `json:"total"`
}

// QueryHistory returns the most recent `limit` completed games, each with
// their players and per-round scores.
func (s *Storage) QueryHistory(limit int) ([]GameRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, started_at, COALESCE(finished_at,''), COALESCE(winner_name,'')
		FROM games
		WHERE finished_at IS NOT NULL
		ORDER BY started_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []GameRecord
	for rows.Next() {
		var g GameRecord
		if err := rows.Scan(&g.ID, &g.StartedAt, &g.FinishedAt, &g.WinnerName); err != nil {
			return nil, err
		}

		// Players for this game
		prows, err := s.db.Query(
			`SELECT player_name, seat_index, final_score
			 FROM game_players WHERE game_id = ? ORDER BY seat_index`, g.ID)
		if err != nil {
			return nil, err
		}
		for prows.Next() {
			var p PlayerRecord
			prows.Scan(&p.Name, &p.SeatIndex, &p.FinalScore) //nolint:errcheck
			g.Players = append(g.Players, p)
		}
		prows.Close()

		// Rounds — joined with game_players to resolve player names
		rrows, err := s.db.Query(`
			SELECT gr.round_number, gp.player_name,
			       gr.card_points, gr.spil_points, gr.tabla_points, gr.round_total
			FROM game_rounds gr
			JOIN game_players gp
			  ON gp.game_id = gr.game_id AND gp.player_id = gr.player_id
			WHERE gr.game_id = ?
			ORDER BY gr.round_number, gp.seat_index`, g.ID)
		if err != nil {
			return nil, err
		}
		roundMap := map[int]*RoundRecord{}
		var roundOrder []int
		for rrows.Next() {
			var roundNum int
			var sc RoundScoreRecord
			rrows.Scan(&roundNum, &sc.PlayerName, &sc.CardPoints, &sc.SpilPoints, &sc.TablaPoints, &sc.Total) //nolint:errcheck
			if _, ok := roundMap[roundNum]; !ok {
				roundMap[roundNum] = &RoundRecord{Number: roundNum}
				roundOrder = append(roundOrder, roundNum)
			}
			roundMap[roundNum].Scores = append(roundMap[roundNum].Scores, sc)
		}
		rrows.Close()
		for _, n := range roundOrder {
			g.Rounds = append(g.Rounds, *roundMap[n])
		}

		games = append(games, g)
	}
	if games == nil {
		games = []GameRecord{}
	}
	return games, nil
}

// RecordGameEnd sets finished_at, winner_name, and final scores for each player.
func (s *Storage) RecordGameEnd(gameID, winnerName string, finalPlayers []game.PublicPlayer) {
	_, err := s.db.Exec(
		`UPDATE games SET finished_at = ?, winner_name = ? WHERE id = ?`,
		time.Now().UTC(), winnerName, gameID,
	)
	if err != nil {
		log.Printf("[storage] RecordGameEnd: %v", err)
	}
	for _, p := range finalPlayers {
		_, err := s.db.Exec(
			`UPDATE game_players SET final_score = ? WHERE game_id = ? AND player_id = ?`,
			p.TotalScore, gameID, p.ID,
		)
		if err != nil {
			log.Printf("[storage] RecordGameEnd player: %v", err)
		}
	}
}
