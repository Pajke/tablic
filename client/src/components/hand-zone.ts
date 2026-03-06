import { Container } from 'pixi.js'
import { CardSprite, CARD_WIDTH, CARD_HEIGHT } from './card-sprite'
import type { Card } from '../protocol'
import { gsap } from 'gsap'

const FAN_SPREAD = 80   // horizontal spacing between cards
const FAN_ANGLE = 0.06  // radians of rotation per card from center

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
    this.sprites = cards.map((card) => {
      const sprite = new CardSprite(card, true)
      sprite.on('pointertap', () => {
        if (!this._enabled) return
        if (this.selectedCard?.id === card.id) {
          // Second click on the already-selected card → play it
          this.onCardClick?.(card)
        } else {
          // First click → select (highlight), deselect previous
          this.selectedSprite?.setHighlight(false)
          sprite.setHighlight(true)
          this.selectedCard = card
          this.selectedSprite = sprite
        }
      })
      this.addChild(sprite)
      return sprite
    })
    this.layout()
  }

  setInteractive(on: boolean) {
    this._enabled = on
    if (!on) {
      // Clear selection when disabling
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

      gsap.to(s, { x: targetX, y: targetY, rotation: angle, duration: 0.3, ease: 'power2.out' })
    }
  }

  removeCard(cardId: string) {
    // Clear selection if the removed card was selected
    if (this.selectedCard?.id === cardId) {
      this.selectedCard = null
      this.selectedSprite = null
    }
    const idx = this.sprites.findIndex((s) => s.card.id === cardId)
    if (idx === -1) return
    const [sprite] = this.sprites.splice(idx, 1)
    gsap.to(sprite, {
      y: sprite.y - 40,
      alpha: 0,
      duration: 0.25,
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
