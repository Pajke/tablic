// Protocol types — mirrors server/internal/protocol/protocol.go
// Keep in sync manually.

export type Suit = 'clubs' | 'diamonds' | 'hearts' | 'spades'
export type Rank = '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9' | '10' | 'J' | 'Q' | 'K' | 'A'

export interface Card {
  id: string
  rank: Rank
  suit: Suit
}

export interface CaptureOption {
  groups: Card[][]
}

export interface PublicPlayer {
  id: string
  name: string
  seatIndex: number
  totalScore: number
  tablas: number
}

export type GamePhase = 'waiting' | 'playing' | 'round_end' | 'game_over'

export interface ClientGameState {
  roomId: string
  phase: GamePhase
  players: PublicPlayer[]
  tableCards: Card[]
  currentPlayerIndex: number
  lastCapturerIndex: number | null
  dealNumber: number
  roundNumber: number
  teamMode: boolean
}

export interface RoundScore {
  playerId: string
  cardPoints: number
  spilPoints: number
  tablaPoints: number
  total: number
}

// --- Client → Server ---

export interface CreateRoomMsg {
  type: 'CREATE_ROOM'
  playerName: string
  maxPlayers: 2 | 4
}

export interface JoinRoomMsg {
  type: 'JOIN_ROOM'
  roomId: string
  playerName: string
  reconnectToken?: string
}

export interface PlayCardMsg {
  type: 'PLAY_CARD'
  cardId: string
}

export interface ChooseCaptureMsg {
  type: 'CHOOSE_CAPTURE'
  optionIndex: number
}

export interface PingMsg {
  type: 'PING'
}

export type ClientMessage = CreateRoomMsg | JoinRoomMsg | PlayCardMsg | ChooseCaptureMsg | PingMsg

// --- Server → Client ---

export interface RoomJoinedMsg {
  type: 'ROOM_JOINED'
  roomId: string
  playerId: string
  reconnectToken: string
  seatIndex: number
}

export interface PlayerJoinedMsg {
  type: 'PLAYER_JOINED'
  player: PublicPlayer
}

export interface GameStartedMsg {
  type: 'GAME_STARTED'
  state: ClientGameState
}

export interface HandDealtMsg {
  type: 'HAND_DEALT'
  cards: Card[]
}

export interface TurnStartMsg {
  type: 'TURN_START'
  playerIndex: number
}

export interface CardPlayedMsg {
  type: 'CARD_PLAYED'
  playerId: string
  card: Card
}

export interface CaptureOptionsMsg {
  type: 'CAPTURE_OPTIONS'
  options: CaptureOption[]
}

export interface CaptureMadeMsg {
  type: 'CAPTURE_MADE'
  playerId: string
  capturedCards: Card[]
  wasTabla: boolean
}

export interface CardDiscardedMsg {
  type: 'CARD_DISCARDED'
  card: Card
}

export interface RoundEndMsg {
  type: 'ROUND_END'
  scores: RoundScore[]
}

export interface GameOverMsg {
  type: 'GAME_OVER'
  winner: PublicPlayer
  players: PublicPlayer[]
}

export interface PlayerDisconnectedMsg {
  type: 'PLAYER_DISCONNECTED'
  playerId: string
}

export interface ErrorMsg {
  type: 'ERROR'
  code: string
  message: string
}

export interface PongMsg {
  type: 'PONG'
}

export type ServerMessage =
  | RoomJoinedMsg
  | PlayerJoinedMsg
  | GameStartedMsg
  | HandDealtMsg
  | TurnStartMsg
  | CardPlayedMsg
  | CaptureOptionsMsg
  | CaptureMadeMsg
  | CardDiscardedMsg
  | RoundEndMsg
  | GameOverMsg
  | PlayerDisconnectedMsg
  | ErrorMsg
  | PongMsg
