import { Application, Container, Sprite, Text, TextStyle, Assets } from 'pixi.js'
import { HandZone } from '../components/hand-zone'
import { TableZone } from '../components/table-zone'
import type { LocalState } from '../state/client-state'
import type { ServerMessage, PublicPlayer } from '../protocol'
import type { WsClient } from '../ws-client'

const AVATAR_SIZE = 40
const AVATAR_MARGIN = 8

interface PlayerSlot {
  container: Container
  avatar: Sprite
  nameText: Text
  scoreText: Text
}

export class GameScene extends Container {
  private handZone: HandZone
  private tableZone: TableZone
  private statusText: Text
  private scoreText: Text
  private ws: WsClient
  private app: Application
  private playerSlots: PlayerSlot[] = []

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

    // Score text — top right (fallback / team mode)
    this.scoreText = new Text({
      text: '',
      style: new TextStyle({ fontFamily: 'sans-serif', fontSize: 14, fill: 0xdddddd }),
    })
    this.scoreText.anchor.set(1, 0)
    this.scoreText.position.set(W - 16, 16)
    this.addChild(this.scoreText)
  }

  /** Call whenever the renderer size changes to reposition all elements. */
  resize(W: number, H: number) {
    this.tableZone.position.set(W / 2, H / 2 - 60)
    this.handZone.position.set(W / 2, H - 80)
    this.statusText.position.set(W / 2, 16)
    this.scoreText.position.set(W - 16, 16)
    // Player slots stay pinned top-left — no update needed
  }

  private async buildPlayerSlots(players: PublicPlayer[]) {
    // Remove old slots
    for (const slot of this.playerSlots) this.removeChild(slot.container)
    this.playerSlots = []

    const slotHeight = AVATAR_SIZE + AVATAR_MARGIN
    const startY = 60

    for (let i = 0; i < players.length; i++) {
      const p = players[i]
      const container = new Container()
      container.position.set(8, startY + i * slotHeight)

      // Avatar sprite
      const url = `/avatars/${p.avatarIndex || 1}.png`
      let texture
      try {
        texture = await Assets.load(url)
      } catch {
        texture = null
      }
      const avatar = texture ? new Sprite(texture) : new Sprite()
      avatar.width = AVATAR_SIZE
      avatar.height = AVATAR_SIZE
      avatar.roundPixels = true
      container.addChild(avatar)

      // Name text
      const nameText = new Text({
        text: p.name,
        style: new TextStyle({ fontFamily: 'sans-serif', fontSize: 13, fill: 0xffffff }),
      })
      nameText.position.set(AVATAR_SIZE + 6, 2)
      container.addChild(nameText)

      // Score text
      const scoreText = new Text({
        text: '0',
        style: new TextStyle({ fontFamily: 'sans-serif', fontSize: 12, fill: 0xdddddd }),
      })
      scoreText.position.set(AVATAR_SIZE + 6, 18)
      container.addChild(scoreText)

      this.addChild(container)
      this.playerSlots.push({ container, avatar, nameText, scoreText })
    }
  }

  private updateSlotScores(state: LocalState) {
    const gs = state.gameState
    if (!gs) return
    for (let i = 0; i < this.playerSlots.length && i < gs.players.length; i++) {
      const p = gs.players[i]
      const t = state.playerTablas[p.id] ?? 0
      // Line 1: total score
      // Line 2: captured cards + scoring points this round
      const capInfo = p.capturedCount > 0
        ? `${p.capturedCount} cards  ${p.scoringPoints}pt`
        : 'no captures yet'
      const tablaInfo = t > 0 ? `  T:${t}` : ''
      this.playerSlots[i].scoreText.text = `${p.totalScore} pts  |  ${capInfo}${tablaInfo}`
    }
  }

  /** Sync the scene to the current local state. Called after every server message. */
  sync(state: LocalState, prevMsg: ServerMessage | null) {
    const gs = state.gameState
    if (!gs) return

    const currentPlayer = gs.players[gs.currentPlayerIndex]
    const isMyTurn = currentPlayer?.id === state.myPlayerId

    // Build player slots once on game start
    if (prevMsg?.type === 'GAME_STARTED') {
      this.buildPlayerSlots(gs.players)
      // In team mode, keep scoreText for team summary; otherwise hide it
      this.scoreText.text = ''
    }

    // Status line
    const dealsLabel = gs.dealsRemaining > 0
      ? `  [${gs.dealsRemaining} deal${gs.dealsRemaining !== 1 ? 's' : ''} left]`
      : '  [last deal]'
    if (gs.phase === 'game_over') {
      this.statusText.text = state.winner ? `Game Over — Winner: ${state.winner.name}` : 'Game Over'
    } else if (isMyTurn) {
      this.statusText.text = `Your turn — play a card${dealsLabel}`
    } else if (currentPlayer && state.disconnectedPlayers.has(currentPlayer.id)) {
      this.statusText.text = `Waiting for ${currentPlayer.name} to reconnect… (auto-skip in ~30s)${dealsLabel}`
    } else {
      this.statusText.text = `${currentPlayer?.name ?? '?'}'s turn${dealsLabel}`
    }

    // Team mode: show team scores in top-right text; individual mode: use player slots
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
    }

    // Update per-player score labels in the left sidebar
    this.updateSlotScores(state)

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
