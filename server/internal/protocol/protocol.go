package protocol

import (
	"encoding/json"
	"fmt"

	"tablic/server/internal/game"
)

// --- Client → Server messages ---

type CreateRoomMsg struct {
	Type       string `json:"type"`
	PlayerName string `json:"playerName"`
	MaxPlayers int    `json:"maxPlayers"` // 2 or 4
}

type JoinRoomMsg struct {
	Type           string `json:"type"`
	RoomID         string `json:"roomId"`
	PlayerName     string `json:"playerName"`
	ReconnectToken string `json:"reconnectToken,omitempty"`
}

type PlayCardMsg struct {
	Type   string `json:"type"`
	CardID string `json:"cardId"`
}

type ChooseCaptureMsg struct {
	Type        string `json:"type"`
	OptionIndex int    `json:"optionIndex"`
}

type PingMsg struct {
	Type string `json:"type"`
}

// ParseClientMessage parses an incoming WebSocket message by its "type" field.
func ParseClientMessage(data []byte) (string, any, error) {
	var base struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &base); err != nil {
		return "", nil, err
	}
	switch base.Type {
	case "CREATE_ROOM":
		var m CreateRoomMsg
		return base.Type, m, json.Unmarshal(data, &m)
	case "JOIN_ROOM":
		var m JoinRoomMsg
		return base.Type, m, json.Unmarshal(data, &m)
	case "PLAY_CARD":
		var m PlayCardMsg
		return base.Type, m, json.Unmarshal(data, &m)
	case "CHOOSE_CAPTURE":
		var m ChooseCaptureMsg
		return base.Type, m, json.Unmarshal(data, &m)
	case "PING":
		return base.Type, PingMsg{Type: "PING"}, nil
	default:
		return base.Type, nil, fmt.Errorf("unknown message type: %s", base.Type)
	}
}

// --- Server → Client messages ---

type RoomJoinedMsg struct {
	Type           string `json:"type"`
	RoomID         string `json:"roomId"`
	PlayerID       string `json:"playerId"`
	ReconnectToken string `json:"reconnectToken"`
	SeatIndex      int    `json:"seatIndex"`
}

type PlayerJoinedMsg struct {
	Type   string            `json:"type"`
	Player game.PublicPlayer `json:"player"`
}

type GameStartedMsg struct {
	Type  string               `json:"type"`
	State game.ClientGameState `json:"state"`
}

type HandDealtMsg struct {
	Type  string      `json:"type"`
	Cards []game.Card `json:"cards"`
}

type TurnStartMsg struct {
	Type        string `json:"type"`
	PlayerIndex int    `json:"playerIndex"`
}

type CardPlayedMsg struct {
	Type     string    `json:"type"`
	PlayerID string    `json:"playerId"`
	Card     game.Card `json:"card"`
}

type CaptureOptionsMsg struct {
	Type    string               `json:"type"`
	Options []game.CaptureOption `json:"options"`
}

type CaptureMadeMsg struct {
	Type          string      `json:"type"`
	PlayerID      string      `json:"playerId"`
	CapturedCards []game.Card `json:"capturedCards"`
	WasTabla      bool        `json:"wasTabla"`
}

type CardDiscardedMsg struct {
	Type string    `json:"type"`
	Card game.Card `json:"card"`
}

type RoundEndMsg struct {
	Type   string            `json:"type"`
	Scores []game.RoundScore `json:"scores"`
}

type GameOverMsg struct {
	Type    string              `json:"type"`
	Winner  game.PublicPlayer   `json:"winner"`
	Players []game.PublicPlayer `json:"players"`
}

type PlayerDisconnectedMsg struct {
	Type     string `json:"type"`
	PlayerID string `json:"playerId"`
}

type ErrorMsg struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PongMsg struct {
	Type string `json:"type"`
}

// MustMarshal marshals v to JSON; panics on error (only for server-controlled structs).
func MustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic("protocol: failed to marshal: " + err.Error())
	}
	return b
}
