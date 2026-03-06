import type { ClientMessage, ServerMessage } from './protocol'

type MessageHandler = (msg: ServerMessage) => void

export class WsClient {
  private ws: WebSocket | null = null
  private onMessage: MessageHandler
  private reconnectToken: string | null = null
  private roomId: string | null = null
  private playerName: string | null = null
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null

  constructor(onMessage: MessageHandler) {
    this.onMessage = onMessage
  }

  connect(url: string, onConnected?: () => void): void {
    if (this.ws) {
      this.ws.close()
    }
    this.ws = new WebSocket(url)
    this.ws.addEventListener('open', () => {
      console.log('[ws] connected')
      if (this.reconnectTimer) {
        clearTimeout(this.reconnectTimer)
        this.reconnectTimer = null
      }
      onConnected?.()
    })
    this.ws.addEventListener('message', (ev) => {
      try {
        const msg = JSON.parse(ev.data) as ServerMessage
        this.onMessage(msg)
      } catch (e) {
        console.error('[ws] failed to parse message', ev.data, e)
      }
    })
    this.ws.addEventListener('close', () => {
      console.log('[ws] disconnected')
      this.scheduleReconnect()
    })
    this.ws.addEventListener('error', (e) => {
      console.error('[ws] error', e)
    })
  }

  send(msg: ClientMessage): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg))
    } else {
      console.warn('[ws] cannot send — not connected', msg)
    }
  }

  setReconnectInfo(roomId: string, reconnectToken: string, playerName: string): void {
    this.roomId = roomId
    this.reconnectToken = reconnectToken
    this.playerName = playerName
  }

  /** Clear reconnect credentials so the next disconnect won't auto-rejoin. */
  clearReconnectInfo(): void {
    this.reconnectToken = null
    this.roomId = null
    this.playerName = null
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }

  private scheduleReconnect(): void {
    if (!this.reconnectToken || !this.roomId) return
    this.reconnectTimer = setTimeout(() => {
      console.log('[ws] attempting reconnect...')
      const url = this.ws?.url ?? buildWsUrl()
      this.connect(url)
      // After connecting, send JOIN_ROOM with reconnect token
      // The open event will fire before we send; use a one-time open listener
      const tryReconnect = () => {
        this.send({
          type: 'JOIN_ROOM',
          roomId: this.roomId!,
          playerName: this.playerName ?? '',
          reconnectToken: this.reconnectToken!,
        })
        this.ws?.removeEventListener('open', tryReconnect)
      }
      this.ws?.addEventListener('open', tryReconnect)
    }, 2000)
  }
}

function buildWsUrl(): string {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  return `${proto}://${location.host}/ws`
}

export function buildWsUrlDefault(): string {
  return buildWsUrl()
}
