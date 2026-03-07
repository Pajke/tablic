// Minimal sound effects via Web Audio API — no external files required.

let ctx: AudioContext | null = null

function getCtx(): AudioContext {
  if (!ctx) ctx = new AudioContext()
  // Resume if suspended (browsers suspend until user gesture)
  if (ctx.state === 'suspended') ctx.resume()
  return ctx
}

function playTone(
  freq: number,
  type: OscillatorType,
  gainPeak: number,
  attackMs: number,
  decayMs: number,
  offset = 0,
) {
  const c = getCtx()
  const osc = c.createOscillator()
  const gain = c.createGain()
  osc.connect(gain)
  gain.connect(c.destination)

  osc.type = type
  osc.frequency.value = freq

  const now = c.currentTime + offset
  gain.gain.setValueAtTime(0, now)
  gain.gain.linearRampToValueAtTime(gainPeak, now + attackMs / 1000)
  gain.gain.exponentialRampToValueAtTime(0.0001, now + (attackMs + decayMs) / 1000)

  osc.start(now)
  osc.stop(now + (attackMs + decayMs) / 1000 + 0.01)
}

/** Short click when a card is played or captured. */
export function playCardSound() {
  try {
    playTone(480, 'sine', 0.18, 5, 80)
    playTone(380, 'sine', 0.08, 5, 60, 0.04)
  } catch { /* ignore if AudioContext unavailable */ }
}

/** Richer chord for capturing multiple cards. */
export function playCaptureSound() {
  try {
    playTone(520, 'sine', 0.2, 8, 120)
    playTone(660, 'sine', 0.12, 8, 100, 0.03)
    playTone(780, 'sine', 0.08, 8, 90, 0.07)
  } catch { /* ignore */ }
}

/** Ascending fanfare for tabla (table cleared). */
export function playTablaSound() {
  try {
    playTone(523, 'triangle', 0.25, 10, 180)        // C5
    playTone(659, 'triangle', 0.2, 10, 160, 0.12)   // E5
    playTone(784, 'triangle', 0.18, 10, 200, 0.22)  // G5
    playTone(1047, 'triangle', 0.15, 10, 300, 0.34) // C6
  } catch { /* ignore */ }
}
