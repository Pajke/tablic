package protocol

import (
	"encoding/json"
	"testing"
)

func TestParseClientMessage_CreateRoom(t *testing.T) {
	data := []byte(`{"type":"CREATE_ROOM","playerName":"Alice","maxPlayers":4}`)
	msgType, msg, err := ParseClientMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msgType != "CREATE_ROOM" {
		t.Errorf("want CREATE_ROOM, got %s", msgType)
	}
	m, ok := msg.(CreateRoomMsg)
	if !ok {
		t.Fatalf("expected CreateRoomMsg, got %T", msg)
	}
	if m.PlayerName != "Alice" || m.MaxPlayers != 4 {
		t.Errorf("unexpected fields: name=%s maxPlayers=%d", m.PlayerName, m.MaxPlayers)
	}
}

func TestParseClientMessage_JoinRoom(t *testing.T) {
	data := []byte(`{"type":"JOIN_ROOM","roomId":"abc123","playerName":"Bob"}`)
	msgType, msg, err := ParseClientMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msgType != "JOIN_ROOM" {
		t.Errorf("want JOIN_ROOM, got %s", msgType)
	}
	m, ok := msg.(JoinRoomMsg)
	if !ok {
		t.Fatalf("expected JoinRoomMsg, got %T", msg)
	}
	if m.RoomID != "abc123" || m.PlayerName != "Bob" {
		t.Errorf("unexpected fields: %+v", m)
	}
}

func TestParseClientMessage_JoinRoom_WithReconnectToken(t *testing.T) {
	data := []byte(`{"type":"JOIN_ROOM","roomId":"r1","playerName":"Carol","reconnectToken":"tok999"}`)
	_, msg, err := ParseClientMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := msg.(JoinRoomMsg)
	if !ok {
		t.Fatalf("expected JoinRoomMsg, got %T", msg)
	}
	if m.ReconnectToken != "tok999" {
		t.Errorf("want reconnect token tok999, got %q", m.ReconnectToken)
	}
}

func TestParseClientMessage_PlayCard(t *testing.T) {
	data := []byte(`{"type":"PLAY_CARD","cardId":"5-hearts"}`)
	msgType, msg, err := ParseClientMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msgType != "PLAY_CARD" {
		t.Errorf("want PLAY_CARD, got %s", msgType)
	}
	m, ok := msg.(PlayCardMsg)
	if !ok {
		t.Fatalf("expected PlayCardMsg, got %T", msg)
	}
	if m.CardID != "5-hearts" {
		t.Errorf("want cardId=5-hearts, got %s", m.CardID)
	}
}

func TestParseClientMessage_ChooseCapture(t *testing.T) {
	data := []byte(`{"type":"CHOOSE_CAPTURE","optionIndex":2}`)
	msgType, msg, err := ParseClientMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msgType != "CHOOSE_CAPTURE" {
		t.Errorf("want CHOOSE_CAPTURE, got %s", msgType)
	}
	m, ok := msg.(ChooseCaptureMsg)
	if !ok {
		t.Fatalf("expected ChooseCaptureMsg, got %T", msg)
	}
	if m.OptionIndex != 2 {
		t.Errorf("want OptionIndex=2, got %d", m.OptionIndex)
	}
}

func TestParseClientMessage_Ping(t *testing.T) {
	data := []byte(`{"type":"PING"}`)
	msgType, msg, err := ParseClientMessage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msgType != "PING" {
		t.Errorf("want PING, got %s", msgType)
	}
	if _, ok := msg.(PingMsg); !ok {
		t.Fatalf("expected PingMsg, got %T", msg)
	}
}

func TestParseClientMessage_UnknownType_ReturnsError(t *testing.T) {
	data := []byte(`{"type":"INVALID_TYPE"}`)
	_, _, err := ParseClientMessage(data)
	if err == nil {
		t.Error("expected error for unknown message type")
	}
}

func TestParseClientMessage_MalformedJSON_ReturnsError(t *testing.T) {
	data := []byte(`{not valid json}`)
	_, _, err := ParseClientMessage(data)
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestMustMarshal_ProducesRoundTrippableJSON(t *testing.T) {
	msg := PongMsg{Type: "PONG"}
	data := MustMarshal(msg)
	if len(data) == 0 {
		t.Fatal("expected non-empty JSON output")
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if m["type"] != "PONG" {
		t.Errorf("want type=PONG, got %s", m["type"])
	}
}
