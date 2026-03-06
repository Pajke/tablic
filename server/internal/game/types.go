package game

// Suit represents a card suit.
type Suit string

const (
	Clubs    Suit = "clubs"
	Diamonds Suit = "diamonds"
	Hearts   Suit = "hearts"
	Spades   Suit = "spades"
)

// Rank represents a card rank.
type Rank string

const (
	Rank2 Rank = "2"
	Rank3 Rank = "3"
	Rank4 Rank = "4"
	Rank5 Rank = "5"
	Rank6 Rank = "6"
	Rank7 Rank = "7"
	Rank8 Rank = "8"
	Rank9 Rank = "9"
	Rank10 Rank = "10"
	RankJ  Rank = "J"
	RankQ  Rank = "Q"
	RankK  Rank = "K"
	RankA  Rank = "A"
)

// Card is a single playing card.
type Card struct {
	ID   string `json:"id"`   // e.g. "A-spades", "10-diamonds"
	Rank Rank   `json:"rank"`
	Suit Suit   `json:"suit"`
}

// GamePhase represents the current phase of a game.
type GamePhase string

const (
	PhaseWaiting  GamePhase = "waiting"
	PhasePlaying  GamePhase = "playing"
	PhaseRoundEnd GamePhase = "round_end"
	PhaseGameOver GamePhase = "game_over"
)

// Player holds state for a single player.
type Player struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	SeatIndex  int    `json:"seatIndex"`
	Hand       []Card `json:"-"`           // never sent to other players
	Captured   []Card `json:"captured"`
	Tablas     int    `json:"tablas"`
	TotalScore int    `json:"totalScore"`
}

// GameState is the authoritative server-side game state.
type GameState struct {
	RoomID             string
	Phase              GamePhase
	Players            []Player
	Deck               []Card // never serialized to clients
	TableCards         []Card
	CurrentPlayerIndex int
	LastCapturerIndex  *int // nil = no capture yet this round
	DealNumber         int
	RoundNumber        int
}

// CaptureOption represents one valid way to capture: a list of non-overlapping
// card groups, each of which sums to the played card's value.
type CaptureOption struct {
	Groups [][]Card `json:"groups"`
}

// CaptureResult holds all valid capture options for a played card.
type CaptureResult struct {
	Options   []CaptureOption `json:"options"`
	AceUsedAs *int            `json:"aceUsedAs,omitempty"` // nil if played card was not an Ace
}

// RoundScore holds the score breakdown for one player at the end of a round.
type RoundScore struct {
	PlayerID    string `json:"playerId"`
	CardPoints  int    `json:"cardPoints"`  // points from scoring cards (10♦, 2♣, face cards, 10s, Aces)
	SpilPoints  int    `json:"spilPoints"`  // 3 pts for most cards (0 if tied or lost)
	TablaPoints int    `json:"tablaPoints"` // accumulated tabla points this round
	Total       int    `json:"total"`
}

// PublicPlayer is a sanitized player view sent to all clients (no Hand).
type PublicPlayer struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	SeatIndex  int    `json:"seatIndex"`
	TotalScore int    `json:"totalScore"`
	Tablas     int    `json:"tablas"`
}

// ToPublic converts a Player to a PublicPlayer (no hand).
func (p Player) ToPublic() PublicPlayer {
	return PublicPlayer{
		ID:         p.ID,
		Name:       p.Name,
		SeatIndex:  p.SeatIndex,
		TotalScore: p.TotalScore,
		Tablas:     p.Tablas,
	}
}

// ClientGameState is the GameState view sent to clients (no deck, no other players' hands).
type ClientGameState struct {
	RoomID             string         `json:"roomId"`
	Phase              GamePhase      `json:"phase"`
	Players            []PublicPlayer `json:"players"`
	TableCards         []Card         `json:"tableCards"`
	CurrentPlayerIndex int            `json:"currentPlayerIndex"`
	LastCapturerIndex  *int           `json:"lastCapturerIndex"`
	DealNumber         int            `json:"dealNumber"`
	RoundNumber        int            `json:"roundNumber"`
	TeamMode           bool           `json:"teamMode"` // true when 4-player 2v2
}
