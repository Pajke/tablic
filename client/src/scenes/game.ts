import { Application, Container, Text, TextStyle } from 'pixi.js'
import { HandZone } from '../components/hand-zone'
import { TableZone } from '../components/table-zone'
import type { LocalState } from '../state/client-state'
import type { ServerMessage } from '../protocol'
import type { WsClient } from '../ws-client'

export class GameScene extends Container {
  private handZone: HandZone
  private tableZone: TableZone
  private statusText: Text
  private scoreText: Text
  private ws: WsClient
  private app: Application

  constructor(app: Application, ws: WsClient) {
    super()
    this.app = app
    this.ws = ws

    const W = app.screen.width
    const H = app.screen.height

    // Table zone — center
    this.tableZone = new TableZone()
    this.tableZone.position.set(W / 2, H / 2 - 60)
    this.addChild(this.tableZone)

    // Hand zone — bottom
    this.handZone = new HandZone()
    this.handZone.position.set(W / 2, H - 80)
    this.addChild(this.handZone)

    // Status text — top center
    this.statusText = new Text({
      text: '',
      style: new TextStyle({ fontFamily: 'sans-serif', fontSize: 16, fill: 0xffffff }),
    })
    this.statusText.anchor.set(0.5, 0)
    this.statusText.position.set(W / 2, 16)
    this.addChild(this.statusText)

    // Score text — top right
    this.scoreText = new Text({
      text: '',
      style: new TextStyle({ fontFamily: 'sans-serif', fontSize: 14, fill: 0xdddddd }),
    })
    this.scoreText.anchor.set(1, 0)
    this.scoreText.position.set(W - 16, 16)
    this.addChild(this.scoreText)
  }

  /** Sync the scene to the current local state. Called after every server message. */
  sync(state: LocalState, prevMsg: ServerMessage | null) {
    const gs = state.gameState
    if (!gs) return

    const myPlayer = gs.players.find((p) => p.id === state.myPlayerId)
    const currentPlayer = gs.players[gs.currentPlayerIndex]
    const isMyTurn = currentPlayer?.id === state.myPlayerId

    // Status line
    if (gs.phase === 'game_over') {
      this.statusText.text = state.winner ? `Game Over — Winner: ${state.winner.name}` : 'Game Over'
    } else if (isMyTurn) {
      this.statusText.text = 'Your turn — play a card'
    } else if (currentPlayer && state.disconnectedPlayers.has(currentPlayer.id)) {
      this.statusText.text = `Waiting for ${currentPlayer.name} to reconnect… (auto-skip in ~30s)`
    } else {
      this.statusText.text = `${currentPlayer?.name ?? '?'}'s turn`
    }

    // Score line
    if (gs.teamMode) {
      const teamLabels = ['A', 'B']
      this.scoreText.text = [0, 1]
        .map((teamId) => {
          const members = gs.players.filter((p) => p.seatIndex % 2 === teamId)
          const score = members[0]?.totalScore ?? 0
          const tablas = members.reduce((s, p) => s + (state.playerTablas[p.id] ?? 0), 0)
          const names = members.map((p) => p.name).join('/')
          return tablas > 0
            ? `${teamLabels[teamId]}(${names}): ${score} T:${tablas}`
            : `${teamLabels[teamId]}(${names}): ${score}`
        })
        .join('  |  ')
    } else {
      this.scoreText.text = gs.players
        .map((p) => {
          const t = state.playerTablas[p.id] ?? 0
          return t > 0 ? `${p.name}: ${p.totalScore} (T:${t})` : `${p.name}: ${p.totalScore}`
        })
        .join('  |  ')
    }

    // Sync table cards on game start or reconnect
    if (prevMsg?.type === 'GAME_STARTED') {
      this.tableZone.setCards(gs.tableCards)
    }

    // Hand
    if (prevMsg?.type === 'HAND_DEALT') {
      this.handZone.setCards(state.myHand, isMyTurn, (card) => {
        this.ws.send({ type: 'PLAY_CARD', cardId: card.id })
      })
    }

    // Remove played card from my hand
    if (prevMsg?.type === 'CARD_PLAYED' && prevMsg.playerId === state.myPlayerId) {
      this.handZone.removeCard(prevMsg.card.id)
    }

    // Individual table updates
    if (prevMsg?.type === 'CARD_DISCARDED') {
      this.tableZone.addCard(prevMsg.card)
    }
    if (prevMsg?.type === 'CAPTURE_MADE') {
      this.tableZone.removeCards(prevMsg.capturedCards.map((c) => c.id))
    }

    // Capture options — disable hand, then auto-select the greediest option
    if (prevMsg?.type === 'CAPTURE_OPTIONS' && state.captureOptions) {
      this.handZone.setInteractive(false)
      const options = state.captureOptions
      const scores = options.map((o) => o.groups.flat().length)
      const maxScore = Math.max(...scores)
      const bestIndices = scores.reduce<number[]>((acc, s, i) => (s === maxScore ? [...acc, i] : acc), [])
      if (bestIndices.length === 1) {
        // Unique greedy winner — auto-confirm
        this.ws.send({ type: 'CHOOSE_CAPTURE', optionIndex: bestIndices[0] })
      } else {
        // Genuine tie — let player pick via table-card clicks
        this.tableZone.showCaptureOptions(options, (idx) => {
          this.ws.send({ type: 'CHOOSE_CAPTURE', optionIndex: idx })
        })
      }
    }

    // Update hand interactivity on turn changes
    if (prevMsg?.type === 'TURN_START') {
      this.handZone.setInteractive(isMyTurn)
    }
  }
}
