import { Application } from 'pixi.js'
import { WsClient, buildWsUrlDefault } from './ws-client'
import { createLocalState, applyServerMessage } from './state/client-state'
import { GameScene } from './scenes/game'
import type { ServerMessage } from './protocol'
import { DEBUG } from './debug'
import { playCardSound, playCaptureSound, playTablaSound } from './sounds'

if (DEBUG) console.log('[tablic] script start')

const app = new Application()
if (DEBUG) console.log('[tablic] Application created')

const initWatchdog = setTimeout(() => {
  if (DEBUG) console.warn('[tablic] app.init still pending after 3s — likely WebGPU hang')
}, 3000)

try {
  if (DEBUG) console.log('[tablic] calling app.init...')
  await app.init({
    preference: 'webgl',
    resizeTo: window,
    background: 0x1a472a,
    antialias: true,
  })
  if (DEBUG) console.log('[tablic] app.init complete, renderer:', app.renderer?.type)
} catch (e) {
  if (DEBUG) console.error('[tablic] PixiJS init failed:', e)
} finally {
  clearTimeout(initWatchdog)
}
if (DEBUG) console.log('[tablic] appending canvas')
document.body.appendChild(app.canvas)

// --- State ---
if (DEBUG) console.log('[tablic] setting up state and UI')
let state = createLocalState()
let gameScene: GameScene | null = null

// --- Lobby UI ---
const lobby = document.getElementById('lobby')!
const lobbyStatus = document.getElementById('lobby-status')!
const playerNameInput = document.getElementById('player-name') as HTMLInputElement
const roomIdInput = document.getElementById('room-id') as HTMLInputElement
const maxPlayersSelect = document.getElementById('max-players') as HTMLSelectElement
const btnCreate = document.getElementById('btn-create')!
const btnJoin = document.getElementById('btn-join')!
const btnVsAI = document.getElementById('btn-vs-ai')!

// --- Avatar picker ---
let selectedAvatar = 1
document.querySelectorAll<HTMLElement>('.avatar-opt').forEach((el) => {
  el.addEventListener('click', () => {
    document.querySelectorAll('.avatar-opt').forEach((e) => e.classList.remove('selected'))
    el.classList.add('selected')
    selectedAvatar = parseInt(el.dataset.avatar ?? '1')
  })
})

// --- Score overlay ---
const scoreOverlay = document.getElementById('score-overlay')!
const scoreTitle = document.getElementById('score-title')!
const scoreRows = document.getElementById('score-rows')!
const scoreContinue = document.getElementById('score-continue')!

scoreContinue.addEventListener('click', () => {
  scoreOverlay.classList.add('hidden')
})

// --- History overlay ---
const historyOverlay = document.getElementById('history-overlay')!
const historyList = document.getElementById('history-list')!

document.getElementById('btn-history')!.addEventListener('click', () => {
  loadHistory()
  historyOverlay.classList.remove('hidden')
})
document.getElementById('btn-history-close')!.addEventListener('click', () => {
  historyOverlay.classList.add('hidden')
})

async function loadHistory() {
  type RoundScore = { playerName: string; cardPoints: number; spilPoints: number; tablaPoints: number; total: number }
  type Round = { number: number; scores: RoundScore[] }
  type Player = { name: string; seatIndex: number; finalScore: number }
  type Game = { id: string; startedAt: string; winnerName: string; players: Player[]; rounds: Round[] }

  historyList.innerHTML = '<div style="color:#888;text-align:center;padding:2rem 0">Loading…</div>'

  let games: Game[] = []
  try {
    const res = await fetch('/api/history')
    games = await res.json()
  } catch {
    historyList.innerHTML = '<div style="color:#c00;text-align:center;padding:2rem 0">Failed to load history.</div>'
    return
  }

  if (games.length === 0) {
    historyList.innerHTML = '<div style="color:#888;text-align:center;padding:2rem 0">No completed games yet.</div>'
    return
  }

  historyList.innerHTML = ''

  for (const g of games) {
    const isTeam = g.players.length === 4
    const date = new Date(g.startedAt).toLocaleString()

    let headerLabel: string
    if (isTeam) {
      const teamA = g.players.filter((p) => p.seatIndex % 2 === 0).map((p) => p.name).join(' & ')
      const teamB = g.players.filter((p) => p.seatIndex % 2 === 1).map((p) => p.name).join(' & ')
      headerLabel = `${teamA}  vs  ${teamB}`
    } else {
      headerLabel = g.players.map((p) => p.name).join(' vs ')
    }

    const details = document.createElement('details')
    details.className = 'hist-game'

    const summary = document.createElement('summary')
    summary.innerHTML =
      `<span><strong>${headerLabel}</strong></span>` +
      `<span class="hist-meta">🏆 ${g.winnerName} &nbsp;·&nbsp; ${date}</span>`
    details.appendChild(summary)

    // Round breakdown table
    const roundsDiv = document.createElement('div')
    roundsDiv.className = 'hist-rounds'
    const table = document.createElement('table')

    const thead = document.createElement('thead')
    thead.innerHTML = `<tr><th>Round</th>${g.players.map((p) => `<th>${p.name}</th>`).join('')}</tr>`
    table.appendChild(thead)

    const tbody = document.createElement('tbody')
    for (const round of g.rounds) {
      const scoreByName: Record<string, number> = {}
      for (const s of round.scores) scoreByName[s.playerName] = s.total
      const tr = document.createElement('tr')
      tr.innerHTML =
        `<td>R${round.number}</td>` +
        g.players.map((p) => `<td>${scoreByName[p.name] ?? 0}</td>`).join('')
      tbody.appendChild(tr)
    }

    // Final score row (bold, last row CSS applies)
    const finalTr = document.createElement('tr')
    finalTr.innerHTML =
      `<td>Final</td>` +
      g.players.map((p) => `<td>${p.finalScore}</td>`).join('')
    tbody.appendChild(finalTr)

    table.appendChild(tbody)
    roundsDiv.appendChild(table)
    details.appendChild(roundsDiv)
    historyList.appendChild(details)
  }
}

// --- Game-over overlay ---
const gameoverOverlay = document.getElementById('gameover-overlay')!
const gameoverTitle = document.getElementById('gameover-title')!
const gameoverWinner = document.getElementById('gameover-winner')!
const gameoverRows = document.getElementById('gameover-rows')!
const gameoverBtn = document.getElementById('gameover-btn')!

gameoverBtn.addEventListener('click', () => {
  location.reload()
})

// --- Top game bar (leave + room code) ---
const gameTopBar = document.getElementById('game-top-bar')!
const btnLeave = document.getElementById('btn-leave')!
const roomCodeBar = document.getElementById('room-code-bar')!
const roomCodeText = document.getElementById('room-code-text')!
const btnCopyRoom = document.getElementById('btn-copy-room')!
let currentRoomId = ''

btnLeave.addEventListener('click', () => {
  ws.clearReconnectInfo()
  gameTopBar.classList.remove('visible')
  location.reload()
})

btnCopyRoom.addEventListener('click', async () => {
  if (!currentRoomId) return
  try {
    await navigator.clipboard.writeText(currentRoomId)
    btnCopyRoom.textContent = 'Copied!'
    btnCopyRoom.classList.add('copied')
    setTimeout(() => {
      btnCopyRoom.textContent = 'Copy'
      btnCopyRoom.classList.remove('copied')
    }, 1500)
  } catch {
    // fallback: select text for manual copy
    const el = document.createElement('input')
    el.value = currentRoomId
    document.body.appendChild(el)
    el.select()
    document.execCommand('copy')
    document.body.removeChild(el)
  }
})

// --- Room list ---
const roomListSection = document.getElementById('room-list-section')!
const roomListEl = document.getElementById('room-list')!
const roomListEmpty = document.getElementById('room-list-empty')!

type RoomInfo = { id: string; maxPlayers: number; players: number }

async function loadRooms() {
  try {
    const res = await fetch('/api/rooms')
    const rooms: RoomInfo[] = await res.json()
    renderRooms(rooms)
  } catch {
    roomListEmpty.textContent = 'Could not load rooms.'
  }
}

function renderRooms(rooms: RoomInfo[]) {
  // Remove old items (keep the empty placeholder)
  Array.from(roomListEl.querySelectorAll('.room-item')).forEach((el) => el.remove())
  if (rooms.length === 0) {
    roomListEmpty.textContent = 'No open rooms — create one!'
    roomListEmpty.style.display = ''
    roomListSection.classList.add('visible')
    return
  }
  roomListEmpty.style.display = 'none'
  roomListSection.classList.add('visible')
  for (const room of rooms) {
    const item = document.createElement('div')
    item.className = 'room-item'
    item.innerHTML =
      `<span><strong>${room.id}</strong> <span class="room-meta">${room.players}/${room.maxPlayers} players</span></span>` +
      `<button data-room-id="${room.id}">Join</button>`
    item.querySelector('button')!.addEventListener('click', () => {
      roomIdInput.value = room.id
      btnJoin.click()
    })
    roomListEl.appendChild(item)
  }
}

document.getElementById('btn-refresh-rooms')!.addEventListener('click', loadRooms)
loadRooms()

// --- Toast ---
const toast = document.getElementById('toast')!
let toastTimer: ReturnType<typeof setTimeout> | null = null

function showToast(msg: string, durationMs = 3000) {
  toast.textContent = msg
  toast.classList.add('show')
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => {
    toast.classList.remove('show')
    toastTimer = null
  }, durationMs)
}

// --- WebSocket ---
const ws = new WsClient((msg: ServerMessage) => {
  const prev = msg
  state = applyServerMessage(state, msg)
  handleServerMessage(msg)
  if (gameScene) {
    gameScene.sync(state, prev)
  }
})

function handleServerMessage(msg: ServerMessage) {
  switch (msg.type) {
    case 'ROOM_JOINED':
      currentRoomId = msg.roomId
      roomCodeText.textContent = msg.roomId
      lobbyStatus.textContent = `Joined room ${msg.roomId} (seat ${msg.seatIndex + 1}) — waiting for opponent…`
      ws.setReconnectInfo(msg.roomId, msg.reconnectToken, playerNameInput.value.trim())
      break

    case 'PLAYER_JOINED':
      lobbyStatus.textContent = `${msg.player.name} joined — waiting for more players…`
      break

    case 'GAME_STARTED':
      hideLobby()
      ensureGameScene()
      scoreOverlay.classList.add('hidden')
      gameTopBar.classList.add('visible')
      roomCodeBar.classList.remove('hidden')
      document.title = 'Tablić'
      break

    case 'TURN_START': {
      scoreOverlay.classList.add('hidden')
      const gs = state.gameState
      if (gs) {
        const curPlayer = gs.players[msg.playerIndex]
        if (curPlayer?.id === state.myPlayerId) {
          document.title = '🃏 Your turn — Tablić'
        } else {
          document.title = 'Tablić'
        }
      }
      break
    }

    case 'HAND_DEALT':
      scoreOverlay.classList.add('hidden')
      break

    case 'CARD_DISCARDED':
      playCardSound()
      break

    case 'CAPTURE_MADE':
      if (msg.wasTabla) playTablaSound()
      else playCaptureSound()
      break

    case 'ROUND_END':
      showRoundScores(msg.scores, 'Round Over')
      break

    case 'GAME_OVER':
      ws.clearReconnectInfo()
      scoreOverlay.classList.add('hidden')
      document.title = 'Tablić'
      showGameOver(msg.winner.name, msg.players)
      break

    case 'TURN_AUTO_SKIPPED':
      showToast(`Auto-played for ${msg.playerName} (disconnected)`)
      break

    case 'ERROR':
      setLobbyBusy(false)
      if (msg.code === 'SESSION_EXPIRED') {
        ws.clearReconnectInfo()
        lobby.classList.remove('hidden')
        gameTopBar.classList.remove('visible')
        lobbyStatus.textContent = msg.message
      } else {
        lobbyStatus.textContent = `Error: ${msg.message}`
      }
      break
  }
}

function ensureGameScene() {
  if (!gameScene) {
    gameScene = new GameScene(app, ws)
    app.stage.addChild(gameScene)
  }
}

// Reposition scene elements whenever the canvas is resized.
app.renderer.on('resize', (width: number, height: number) => {
  if (gameScene) gameScene.resize(width, height)
})

function hideLobby() {
  lobby.classList.add('hidden')
}

function showRoundScores(scores: import('./protocol').RoundScore[], title: string) {
  scoreTitle.textContent = title
  scoreRows.innerHTML = ''
  for (const s of scores) {
    const player = state.gameState?.players.find((p) => p.id === s.playerId)
    const name = player?.name ?? s.playerId
    const tr = document.createElement('tr')
    tr.innerHTML = `<td>${name}</td><td>${s.cardPoints}</td><td>${s.spilPoints}</td><td>${s.tablaPoints}</td><td><strong>${s.total}</strong></td>`
    scoreRows.appendChild(tr)
  }
  scoreOverlay.classList.remove('hidden')
}

function showGameOver(winnerName: string, players: import('./protocol').PublicPlayer[]) {
  gameoverTitle.textContent = 'Game Over'
  gameoverRows.innerHTML = ''

  if (players.length === 4) {
    // Team mode: show two team rows
    const winnerTeam = players.find((p) => p.name === winnerName)?.seatIndex ?? 0
    const winningTeamId = winnerTeam % 2
    const teamLabels = ['A', 'B']
    for (let teamId = 0; teamId < 2; teamId++) {
      const members = players.filter((p) => p.seatIndex % 2 === teamId)
      const score = members[0]?.totalScore ?? 0
      const tablas = members.reduce((s, p) => s + p.tablas, 0)
      const names = members.map((p) => p.name).join(' & ')
      const medal = teamId === winningTeamId ? ' 🏆' : ''
      const tr = document.createElement('tr')
      tr.innerHTML = `<td>Team ${teamLabels[teamId]}: ${names}${medal}</td><td><strong>${score}</strong></td><td>${tablas}</td>`
      gameoverRows.appendChild(tr)
    }
    const winTeamName = `Team ${teamLabels[winningTeamId]}`
    gameoverWinner.textContent = `${winTeamName} wins!`
  } else {
    gameoverWinner.textContent = `Winner: ${winnerName}`
    const sorted = [...players].sort((a, b) => b.totalScore - a.totalScore)
    for (const p of sorted) {
      const tr = document.createElement('tr')
      const medal = p.name === winnerName ? ' 🏆' : ''
      tr.innerHTML = `<td>${p.name}${medal}</td><td><strong>${p.totalScore}</strong></td><td>${p.tablas}</td>`
      gameoverRows.appendChild(tr)
    }
  }
  gameoverOverlay.classList.remove('hidden')
}

if (DEBUG) console.log('[tablic] attaching button listeners')

// --- Lobby buttons ---
function setLobbyBusy(busy: boolean) {
  ;(btnCreate as HTMLButtonElement).disabled = busy
  ;(btnJoin as HTMLButtonElement).disabled = busy
  ;(btnVsAI as HTMLButtonElement).disabled = busy
}

btnCreate.addEventListener('click', () => {
  if (DEBUG) console.log('[tablic] create room clicked')
  const name = playerNameInput.value.trim()
  if (!name) { lobbyStatus.textContent = 'Enter your name first'; return }
  const maxPlayers = parseInt(maxPlayersSelect.value) as 2 | 4
  setLobbyBusy(true)
  lobbyStatus.textContent = 'Connecting…'
  const url = buildWsUrlDefault()
  if (DEBUG) console.log('[tablic] connecting to', url)
  ws.connect(url, () => {
    if (DEBUG) console.log('[tablic] ws connected, sending CREATE_ROOM')
    ws.send({ type: 'CREATE_ROOM', playerName: name, maxPlayers, avatarIndex: selectedAvatar })
  })
})

btnJoin.addEventListener('click', () => {
  const name = playerNameInput.value.trim()
  const roomId = roomIdInput.value.trim()
  if (!name) { lobbyStatus.textContent = 'Enter your name first'; return }
  if (!roomId) { lobbyStatus.textContent = 'Enter a room ID to join'; return }
  setLobbyBusy(true)
  lobbyStatus.textContent = 'Connecting…'
  ws.connect(buildWsUrlDefault(), () => {
    ws.send({ type: 'JOIN_ROOM', roomId, playerName: name, avatarIndex: selectedAvatar })
  })
})

btnVsAI.addEventListener('click', () => {
  const name = playerNameInput.value.trim()
  if (!name) { lobbyStatus.textContent = 'Enter your name first'; return }
  setLobbyBusy(true)
  lobbyStatus.textContent = 'Connecting…'
  ws.connect(buildWsUrlDefault(), () => {
    ws.send({ type: 'CREATE_ROOM', playerName: name, maxPlayers: 2, avatarIndex: selectedAvatar, vsAI: true })
  })
})
