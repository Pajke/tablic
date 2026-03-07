import { Container, Graphics, Text, TextStyle } from 'pixi.js'
import type { Card } from '../protocol'

export const CARD_WIDTH = 82
export const CARD_HEIGHT = 116
export const CARD_RADIUS = 9

const SUIT_SYMBOLS: Record<string, string> = {
  clubs: '♣',
  diamonds: '♦',
  hearts: '♥',
  spades: '♠',
}

const SUIT_COLORS: Record<string, number> = {
  clubs: 0x111111,
  diamonds: 0xcc2200,
  hearts: 0xcc2200,
  spades: 0x111111,
}

export class CardSprite extends Container {
  readonly card: Card
  private bg: Graphics
  private highlighted = false

  constructor(card: Card, faceUp = true) {
    super()
    this.card = card

    this.bg = new Graphics()
    this.addChild(this.bg)

    if (faceUp) {
      this.drawFace()
    } else {
      this.drawBack()
    }

    this.eventMode = 'static'
    this.cursor = 'pointer'
  }

  private drawFace() {
    const color = SUIT_COLORS[this.card.suit] ?? 0x111111
    const symbol = SUIT_SYMBOLS[this.card.suit] ?? '?'

    this.bg.clear()
    this.bg.roundRect(0, 0, CARD_WIDTH, CARD_HEIGHT, CARD_RADIUS)
    this.bg.fill({ color: this.highlighted ? 0xfffbd4 : 0xffffff })
    this.bg.stroke({ color: this.highlighted ? 0xffd700 : 0xcccccc, width: this.highlighted ? 2.5 : 1 })

    const rankStyle = new TextStyle({
      fontFamily: 'serif',
      fontSize: 19,
      fill: color,
      fontWeight: 'bold',
    })
    const rankText = new Text({ text: this.card.rank, style: rankStyle })
    rankText.position.set(6, 4)
    this.addChild(rankText)

    const suitStyle = new TextStyle({
      fontFamily: 'serif',
      fontSize: 28,
      fill: color,
    })
    const suitText = new Text({ text: symbol, style: suitStyle })
    suitText.anchor.set(0.5, 0.5)
    suitText.position.set(CARD_WIDTH / 2, CARD_HEIGHT / 2)
    this.addChild(suitText)

    // Bottom-right rank (rotated)
    const rankBR = new Text({ text: this.card.rank, style: rankStyle })
    rankBR.anchor.set(1, 1)
    rankBR.rotation = Math.PI
    rankBR.position.set(CARD_WIDTH - 6, CARD_HEIGHT - 4)
    this.addChild(rankBR)
  }

  private drawBack() {
    this.bg.clear()
    this.bg.roundRect(0, 0, CARD_WIDTH, CARD_HEIGHT, CARD_RADIUS)
    this.bg.fill({ color: 0x1a6b3c })
    this.bg.stroke({ color: 0x0d3d1f, width: 1 })

    // Simple crosshatch pattern
    const inner = new Graphics()
    inner.rect(8, 8, CARD_WIDTH - 16, CARD_HEIGHT - 16)
    inner.stroke({ color: 0x2d8f55, width: 1 })
    this.addChild(inner)
  }

  setHighlight(on: boolean) {
    if (this.highlighted === on) return
    this.highlighted = on
    // Redraw face with new highlight state
    this.removeChildren()
    this.bg = new Graphics()
    this.addChild(this.bg)
    this.drawFace()
  }
}
