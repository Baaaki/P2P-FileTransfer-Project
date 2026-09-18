/**
 * Terminal text helpers.
 *
 * The screens on this page are not screenshots — they are re-rendered in
 * the browser from the same strings the Go program prints. To keep the
 * boxes square we have to measure text the way a terminal does: in cells,
 * where an emoji occupies two.
 */

/**
 * Height of one text row, as a multiple of the font size. Shared with the
 * CSS so box borders can be laid out on the same grid as the glyphs.
 */
export const LINE_H = 1.45

/**
 * Display width of a string in terminal cells.
 *
 * Only true emoji take two cells. Dingbats like ✓ ➜ ▸ live in the same
 * neighbourhood of Unicode but render single-width in every monospace
 * font we hit, so measuring them as wide would push box borders one
 * column past their contents.
 */
const VS16 = '\uFE0F'
const ZWJ = '\u200D'

export function cells(s: string): number {
  const chars = [...s]
  let n = 0
  for (let i = 0; i < chars.length; i++) {
    const ch = chars[i]
    if (ch === VS16 || ch === ZWJ) continue // presentation joiners take no space
    const cp = ch.codePointAt(0)!
    n += cp >= 0x1f000 || chars[i + 1] === VS16 ? 2 : 1
  }
  return n
}

export function pad(s: string, width: number): string {
  return s + ' '.repeat(Math.max(0, width - cells(s)))
}

export function repeat(s: string, n: number): string {
  return n <= 0 ? '' : s.repeat(n)
}

/** formatBytes mirrors the Go helper of the same name. */
export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let v = n / 1024
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(1)} ${units[i]}`
}

/** humanize.Bytes-ish, the compact form the file picker prints. */
export function pickerSize(n: number): string {
  if (n < 1000) return `${n}B`
  const units = ['kB', 'MB', 'GB']
  let v = n / 1000
  let i = 0
  while (v >= 1000 && i < units.length - 1) {
    v /= 1000
    i++
  }
  return `${v < 10 ? v.toFixed(1) : Math.round(v)}${units[i]}`
}

export const SPINNER = ['⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏']

export const clamp = (v: number, lo = 0, hi = 1) => Math.min(hi, Math.max(lo, v))

/** Maps a value in [a,b] onto [0,1]; anything outside is clipped. */
export const range = (v: number, a: number, b: number) => clamp((v - a) / (b - a))

/** Smoothstep, for movement that starts and stops gently. */
export const ease = (t: number) => {
  const x = clamp(t)
  return x * x * (3 - 2 * x)
}
