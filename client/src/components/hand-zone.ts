import { Container } from 'pixi.js'
import { CardSprite, CARD_WIDTH, CARD_HEIGHT } from './card-sprite'
import type { Card } from '../protocol'
import { gsap } from 'gsap'

const FAN_SPREAD = 88   // horizontal spacing between cards
const FAN_ANGLE = 0.05  // radians of rotation per card from center

export class HandZone extends Container {
  private sprites: CardSprite[] = []
  private onCardClick: ((card: Card) => void) | null = null
  private _enabled = false
  private selectedCard: Card | null = null
  private selectedSprite: CardSprite | null = null

  setCards(cards: Card[], enabled: boolean, onCardClick: (card: Card) => void) {
    this._enabled = enabled
    this.onCardClick = onCardClick
    this.selectedCard = null
    this.selectedSprite = null
    this.removeAll()

    const n = cards.length
    const totalWidth = (n - 1) * FAN_SPREAD
    const startX = -totalWidth / 2

    this.sprites = cards.map((card, i) => {
      const sprite = new CardSprite(card, true)

      // Start above the hand zone (deal-from-deck effect)
      sprite.alpha = 0
      sprite.x = 0
      sprite.y = -360

      sprite.on('pointertap', () => {
        if (!this._enabled) return
        if (this.selectedCard?.id === card.id) {
          this.onCardClick?.(card)
        } else {
          this.selectedSprite?.setHighlight(false)
          sprite.setHighlight(true)
          this.selectedCard = card
          this.selectedSprite = sprite
        }
      })
      this.addChild(sprite)

      // Animate to fan position with stagger (deal animation)
      const targetX = startX + i * FAN_SPREAD - CARD_WIDTH / 2
      const targetY = -CARD_HEIGHT / 2
      const angle = (i - (n - 1) / 2) * FAN_ANGLE
      const delay = i * 0.09

      gsap.to(sprite, { alpha: 1, duration: 0.2, delay, ease: 'power1.out' })
      gsap.to(sprite, { x: targetX, y: targetY, rotation: angle, duration: 0.4, delay, ease: 'power2.out' })

      return sprite
    })
  }

  setInteractive(on: boolean) {
    this._enabled = on
    if (!on) {
      this.selectedSprite?.setHighlight(false)
      this.selectedCard = null
      this.selectedSprite = null
    }
    for (const s of this.sprites) {
      s.cursor = on ? 'pointer' : 'default'
    }
  }

  private layout() {
    const n = this.sprites.length
    const totalWidth = (n - 1) * FAN_SPREAD
    const startX = -totalWidth / 2

    for (let i = 0; i < n; i++) {
      const s = this.sprites[i]
      const targetX = startX + i * FAN_SPREAD - CARD_WIDTH / 2
      const targetY = -CARD_HEIGHT / 2
      const angle = (i - (n - 1) / 2) * FAN_ANGLE
      gsap.to(s, { x: targetX, y: targetY, rotation: angle, duration: 0.25, ease: 'power2.out' })
    }
  }

  removeCard(cardId: string) {
    if (this.selectedCard?.id === cardId) {
      this.selectedCard = null
      this.selectedSprite = null
    }
    const idx = this.sprites.findIndex((s) => s.card.id === cardId)
    if (idx === -1) return
    const [sprite] = this.sprites.splice(idx, 1)

    // Fly toward the table zone (upward + slight scale-down)
    gsap.to(sprite, {
      y: sprite.y - 220,
      x: sprite.x * 0.6,
      scaleX: 0.7,
      scaleY: 0.7,
      alpha: 0,
      duration: 0.35,
      ease: 'power2.in',
      onComplete: () => {
        this.removeChild(sprite)
        sprite.destroy()
      },
    })
    this.layout()
  }

  private removeAll() {
    for (const s of this.sprites) {
      this.removeChild(s)
      s.destroy()
    }
    this.sprites = []
  }
}
