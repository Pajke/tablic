package ws

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"tablic/server/internal/protocol"
	"tablic/server/internal/room"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Handler handles WebSocket connections for the game.
type Handler struct {
	manager *room.Manager
}

func NewHandler(manager *room.Manager) *Handler {
	return &Handler{manager: manager}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[ws] new connection from %s", r.RemoteAddr)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws] upgrade error from %s: %v", r.RemoteAddr, err)
		return
	}

	rm, playerID, err := h.handshake(conn)
	if err != nil {
		log.Printf("[ws] handshake failed from %s: %v", r.RemoteAddr, err)
		conn.Close()
		return
	}

	defer func() {
		log.Printf("[ws] disconnected: player=%s room=%s addr=%s", playerID, rm.ID(), r.RemoteAddr)
		rm.Disconnect(playerID)
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		msgType, msg, parseErr := protocol.ParseClientMessage(data)
		if parseErr != nil {
			log.Printf("[ws] parse error from %s: %v", r.RemoteAddr, parseErr)
			rm.SendErrorTo(playerID, "PARSE_ERROR", parseErr.Error())
			continue
		}
		rm.HandleMessage(playerID, msgType, msg)
	}
}

func (h *Handler) handshake(conn *websocket.Conn) (*room.Room, string, error) {
	_, data, err := conn.ReadMessage()
	if err != nil {
		return nil, "", err
	}
	msgType, msg, err := protocol.ParseClientMessage(data)
	if err != nil {
		writeError(conn, "PARSE_ERROR", err.Error())
		return nil, "", err
	}

	var rm *room.Room
	var playerID string

	switch msgType {
	case "CREATE_ROOM":
		m := msg.(protocol.CreateRoomMsg)
		if m.MaxPlayers != 2 && m.MaxPlayers != 4 {
			writeError(conn, "INVALID_MAX_PLAYERS", "maxPlayers must be 2 or 4")
			return nil, "", errBadHandshake
		}
		rm = h.manager.Create(m.MaxPlayers)
		pid, token, seat, joinErr := rm.Join(m.PlayerName, m.AvatarIndex)
		if joinErr != nil {
			writeError(conn, "JOIN_ERROR", joinErr.Error())
			return nil, "", joinErr
		}
		playerID = pid
		log.Printf("[ws] CREATE_ROOM: player=%q seat=%d room=%s maxPlayers=%d token=%s",
			m.PlayerName, seat, rm.ID(), m.MaxPlayers, token[:8])
		writeMsg(conn, protocol.MustMarshal(protocol.RoomJoinedMsg{
			Type: "ROOM_JOINED", RoomID: rm.ID(),
			PlayerID: playerID, ReconnectToken: token, SeatIndex: seat,
		}))

	case "JOIN_ROOM":
		m := msg.(protocol.JoinRoomMsg)
		r, getErr := h.manager.Get(m.RoomID)
		if getErr != nil {
			// If a reconnect token is known to the DB but the room is gone (server restarted),
			// send a clear SESSION_EXPIRED error so the client can reset to lobby.
			if m.ReconnectToken != "" {
				if st := h.manager.Storage(); st != nil && st.GetReconnectToken(m.ReconnectToken) != nil {
					log.Printf("[ws] RECONNECT: room %s gone (server restarted?), token valid for player=%q", m.RoomID, m.PlayerName)
					writeError(conn, "SESSION_EXPIRED", "Game session ended — please start a new game")
					return nil, "", getErr
				}
			}
			log.Printf("[ws] JOIN_ROOM: room %s not found for player=%q", m.RoomID, m.PlayerName)
			writeError(conn, "ROOM_NOT_FOUND", getErr.Error())
			return nil, "", getErr
		}
		rm = r

		if m.ReconnectToken != "" {
			pid, seat, rcErr := rm.Reconnect(m.ReconnectToken)
			if rcErr != nil {
				log.Printf("[ws] RECONNECT failed for player=%q room=%s: %v", m.PlayerName, m.RoomID, rcErr)
				writeError(conn, "RECONNECT_FAILED", rcErr.Error())
				return nil, "", rcErr
			}
			playerID = pid
			log.Printf("[ws] RECONNECT: player=%q seat=%d room=%s", m.PlayerName, seat, rm.ID())
			if err := rm.AttachConn(playerID, conn); err != nil {
				return nil, "", err
			}
			writeMsg(conn, protocol.MustMarshal(protocol.RoomJoinedMsg{
				Type: "ROOM_JOINED", RoomID: rm.ID(),
				PlayerID: playerID, ReconnectToken: m.ReconnectToken, SeatIndex: seat,
			}))
			writeMsg(conn, protocol.MustMarshal(protocol.GameStartedMsg{
				Type: "GAME_STARTED", State: rm.ClientState(),
			}))
			if hand := rm.PlayerHand(playerID); hand != nil {
				writeMsg(conn, protocol.MustMarshal(protocol.HandDealtMsg{
					Type: "HAND_DEALT", Cards: hand,
				}))
			}
			return rm, playerID, nil
		}

		pid, token, seat, joinErr := rm.Join(m.PlayerName, m.AvatarIndex)
		if joinErr != nil {
			log.Printf("[ws] JOIN_ROOM: player=%q failed to join room=%s: %v", m.PlayerName, m.RoomID, joinErr)
			writeError(conn, "JOIN_ERROR", joinErr.Error())
			return nil, "", joinErr
		}
		playerID = pid
		log.Printf("[ws] JOIN_ROOM: player=%q seat=%d room=%s token=%s",
			m.PlayerName, seat, rm.ID(), token[:8])
		writeMsg(conn, protocol.MustMarshal(protocol.RoomJoinedMsg{
			Type: "ROOM_JOINED", RoomID: rm.ID(),
			PlayerID: playerID, ReconnectToken: token, SeatIndex: seat,
		}))

	default:
		writeError(conn, "MUST_JOIN_FIRST", "first message must be CREATE_ROOM or JOIN_ROOM")
		return nil, "", errBadHandshake
	}

	if err := rm.AttachConn(playerID, conn); err != nil {
		return nil, "", err
	}

	if rm.IsFull() {
		log.Printf("[ws] room %s is full — starting game", rm.ID())
		if err := rm.StartGame(); err != nil {
			log.Printf("[ws] start game error room=%s: %v", rm.ID(), err)
			writeError(conn, "START_GAME_ERROR", err.Error())
			return nil, "", err
		}
		rm.BroadcastGameStart()
	}

	return rm, playerID, nil
}

func writeMsg(conn *websocket.Conn, data []byte) {
	conn.WriteMessage(websocket.TextMessage, data)
}

func writeError(conn *websocket.Conn, code, msg string) {
	writeMsg(conn, protocol.MustMarshal(protocol.ErrorMsg{Type: "ERROR", Code: code, Message: msg}))
}

var errBadHandshake = &handshakeError{}

type handshakeError struct{}

func (e *handshakeError) Error() string { return "handshake failed" }
