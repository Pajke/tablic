import { Container, Text, TextStyle } from 'pixi.js'
import { CardSprite, CARD_WIDTH, CARD_HEIGHT } from './card-sprite'
import type { Card, CaptureOption } from '../protocol'
import { gsap } from 'gsap'

const COLS = 6
const COL_GAP = CARD_WIDTH + 12
const ROW_GAP = CARD_HEIGHT + 10

export class TableZone extends Container {
  private sprites: Map<string, CardSprite> = new Map()
  private captureOptions: CaptureOption[] | null = null
  private onCaptureChosen: ((optionIndex: number) => void) | null = null
  private selectedGroupIdx: number | null = null
  private optionLabel: Text | null = null

  /** Fully replace the table cards (called after GAME_STARTED or reconnect). */
  setCards(cards: Card[]) {
    this.clearAll()
    for (const card of cards) {
      const sprite = new CardSprite(card, true)
      this.sprites.set(card.id, sprite)
      this.addChild(sprite)
    }
    this.layout()
  }

  /** Add a single card to the table (CARD_DISCARDED). Animates the card in from above. */
  addCard(card: Card) {
    const sprite = new CardSprite(card, true)

    // Pre-compute target position BEFORE inserting into sprites map
    const idx = this.sprites.size
    const col = idx % COLS
    const row = Math.floor(idx / COLS)
    const targetX = col * COL_GAP - (Math.min(idx + 1, COLS) * COL_GAP) / 2
    const targetY = row * ROW_GAP - CARD_HEIGHT / 2

    // Start above target, invisible
    sprite.x = targetX
    sprite.y = targetY - 50
    sprite.alpha = 0

    this.sprites.set(card.id, sprite)
    this.addChild(sprite)

    // layout() animates all sprites to their positions;
    // for this new sprite: (targetX, targetY-50) → (targetX, targetY)
    this.layout()
    gsap.to(sprite, { alpha: 1, duration: 0.3, ease: 'power2.out' })
  }

  /** Remove cards from the table (CAPTURE_MADE). */
  removeCards(cardIds: string[]) {
    for (const id of cardIds) {
      const sprite = this.sprites.get(id)
      if (!sprite) continue
      this.sprites.delete(id)
      gsap.to(sprite, {
        alpha: 0, y: sprite.y + 30, duration: 0.25,
        onComplete: () => { this.removeChild(sprite); sprite.destroy() },
      })
    }
    this.layout()
  }

  /** Show capture options as highlighted card groups. */
  showCaptureOptions(options: CaptureOption[], onChosen: (idx: number) => void) {
    this.captureOptions = options
    this.onCaptureChosen = onChosen
    this.selectedGroupIdx = null
    this.clearHighlights()

    if (this.optionLabel) { this.removeChild(this.optionLabel); this.optionLabel = null }

    const label = new Text({
      text: `Choose capture (${options.length} options):`,
      style: new TextStyle({ fontFamily: 'sans-serif', fontSize: 14, fill: 0xffffff }),
    })
    label.position.set(0, -30)
    this.addChild(label)
    this.optionLabel = label

    // Make table cards clickable — clicking selects the option that contains that card
    for (const [id, sprite] of this.sprites) {
      sprite.eventMode = 'static'
      sprite.cursor = 'pointer'
      sprite.on('pointertap', () => this.handleCardClickForCapture(id))
    }
  }

  private handleCardClickForCapture(clickedCardId: string) {
    if (!this.captureOptions) return
    // Find which option(s) contain this card
    const matchingOptions: number[] = []
    for (let i = 0; i < this.captureOptions.length; i++) {
      const opt = this.captureOptions[i]
      const allCards = opt.groups.flat()
      if (allCards.some((c) => c.id === clickedCardId)) {
        matchingOptions.push(i)
      }
    }
    if (matchingOptions.length === 0) return
    // If multiple options contain this card, cycle through them
    const currentIdx = matchingOptions.indexOf(this.selectedGroupIdx ?? -1)
    const nextIdx = matchingOptions[(currentIdx + 1) % matchingOptions.length]
    this.selectOption(nextIdx)
  }

  private selectOption(optionIndex: number) {
    this.selectedGroupIdx = optionIndex
    this.clearHighlights()

    const opt = this.captureOptions![optionIndex]
    const captureIds = new Set(opt.groups.flat().map((c) => c.id))
    for (const [id, sprite] of this.sprites) {
      sprite.setHighlight(captureIds.has(id))
    }

    // Auto-confirm after brief delay (or could add a "Confirm" button)
    // For now: single-tap selects AND confirms
    this.onCaptureChosen?.(optionIndex)
    this.hideCaptureOptions()
  }

  hideCaptureOptions() {
    this.captureOptions = null
    this.onCaptureChosen = null
    this.clearHighlights()
    if (this.optionLabel) { this.removeChild(this.optionLabel); this.optionLabel = null }
    for (const sprite of this.sprites.values()) {
      sprite.eventMode = 'none'
      sprite.off('pointertap')
    }
  }

  private clearHighlights() {
    for (const sprite of this.sprites.values()) {
      sprite.setHighlight(false)
    }
  }

  private layout() {
    let i = 0
    for (const sprite of this.sprites.values()) {
      const col = i % COLS
      const row = Math.floor(i / COLS)
      const targetX = col * COL_GAP - (Math.min(this.sprites.size, COLS) * COL_GAP) / 2
      const targetY = row * ROW_GAP - CARD_HEIGHT / 2
      gsap.to(sprite, { x: targetX, y: targetY, duration: 0.25, ease: 'power2.out' })
      i++
    }
  }

  private clearAll() {
    for (const sprite of this.sprites.values()) {
      this.removeChild(sprite)
      sprite.destroy()
    }
    this.sprites.clear()
  }
}
