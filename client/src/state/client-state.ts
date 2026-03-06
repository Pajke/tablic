import type { Card, ClientGameState, PublicPlayer, RoundScore, ServerMessage } from '../protocol'

export interface LocalState {
  gameState: ClientGameState | null
  myPlayerId: string | null
  myHand: Card[]
  captureOptions: import('../protocol').CaptureOption[] | null
  pendingRoundScores: RoundScore[] | null
  winner: PublicPlayer | null
  playerTablas: Record<string, number>
  disconnectedPlayers: Set<string>
}

export function createLocalState(): LocalState {
  return {
    gameState: null,
    myPlayerId: null,
    myHand: [],
    captureOptions: null,
    pendingRoundScores: null,
    winner: null,
    playerTablas: {},
    disconnectedPlayers: new Set(),
  }
}

/** Pure reducer: returns a new state after applying a server message. */
export function applyServerMessage(state: LocalState, msg: ServerMessage): LocalState {
  switch (msg.type) {
    case 'ROOM_JOINED':
      return { ...state, myPlayerId: msg.playerId }

    case 'GAME_STARTED':
      return { ...state, gameState: msg.state, captureOptions: null, pendingRoundScores: null, winner: null, playerTablas: {}, disconnectedPlayers: new Set() }

    case 'HAND_DEALT':
      return { ...state, myHand: msg.cards }

    case 'TURN_START':
      if (!state.gameState) return state
      return {
        ...state,
        gameState: { ...state.gameState, currentPlayerIndex: msg.playerIndex },
        captureOptions: null,
        disconnectedPlayers: new Set(), // reset on each new turn
      }

    case 'CARD_PLAYED': {
      // Remove the played card from my hand if it's mine
      if (msg.playerId !== state.myPlayerId) return state
      return {
        ...state,
        myHand: state.myHand.filter((c) => c.id !== msg.card.id),
      }
    }

    case 'CARD_DISCARDED': {
      if (!state.gameState) return state
      return {
        ...state,
        gameState: {
          ...state.gameState,
          tableCards: [...state.gameState.tableCards, msg.card],
        },
      }
    }

    case 'CAPTURE_OPTIONS':
      return { ...state, captureOptions: msg.options }

    case 'CAPTURE_MADE': {
      if (!state.gameState) return state
      const capturedIds = new Set(msg.capturedCards.map((c) => c.id))
      const tableCards = state.gameState.tableCards.filter((c) => !capturedIds.has(c.id))
      const playerTablas = msg.wasTabla
        ? { ...state.playerTablas, [msg.playerId]: (state.playerTablas[msg.playerId] ?? 0) + 1 }
        : state.playerTablas
      return {
        ...state,
        gameState: { ...state.gameState, tableCards },
        captureOptions: null,
        playerTablas,
      }
    }

    case 'ROUND_END':
      return { ...state, pendingRoundScores: msg.scores }

    case 'GAME_OVER':
      return {
        ...state,
        winner: msg.winner,
        gameState: state.gameState
          ? { ...state.gameState, phase: 'game_over', players: msg.players }
          : null,
      }

    case 'PLAYER_DISCONNECTED': {
      const disconnectedPlayers = new Set(state.disconnectedPlayers)
      disconnectedPlayers.add(msg.playerId)
      return { ...state, disconnectedPlayers }
    }

    case 'ERROR':
      console.error(`[server error] ${msg.code}: ${msg.message}`)
      return state

    default:
      return state
  }
}

/** Returns true if it's currently this client's turn. */
export function isMyTurn(state: LocalState): boolean {
  if (!state.gameState || !state.myPlayerId) return false
  const currentPlayer = state.gameState.players[state.gameState.currentPlayerIndex]
  return currentPlayer?.id === state.myPlayerId
}
