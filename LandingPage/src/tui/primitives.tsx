import type { ReactNode } from 'react'
import { LINE_H, cells, repeat } from './text'

/**
 * One-to-one counterparts of the lipgloss styles in internal/tui/styles.go.
 * Same colours, same weights, same padding — so the browser draws what the
 * terminal draws.
 */

export const Title = ({ children }: { children: ReactNode }) => (
  <div className="font-bold text-tui-accent">{children}</div>
)

export const Body = ({ children }: { children: ReactNode }) => (
  <div className="text-tui-fg">{children}</div>
)

export const Help = ({ children }: { children: ReactNode }) => (
  <div className="text-tui-muted">{children}</div>
)

export const Foot = ({ children }: { children: ReactNode }) => (
  <div className="text-tui-faint">{children}</div>
)

export const Ok = ({ children }: { children: ReactNode }) => (
  <div className="font-bold text-tui-ok">{children}</div>
)

export const Warn = ({ children }: { children: ReactNode }) => (
  <div className="text-tui-warn">{children}</div>
)

export const Gap = ({ n = 1 }: { n?: number }) => (
  <div aria-hidden>{repeat('\n', n - 1) || ' '}</div>
)

/** A menu row: selected rows lose their indent and gain a caret. */
export const Choice = ({ label, selected }: { label: string; selected: boolean }) =>
  selected ? (
    <div className="font-bold text-tui-accent">{`▸ ${label}`}</div>
  ) : (
    <div className="text-tui-fg">{`  ${label}`}</div>
  )

/** Grid helpers: cells → CSS length, rows → CSS length. */
const col = (n: number) => `${n}ch`
const row = (n: number) => `calc(${n} * ${LINE_H}em)`

/**
 * lipgloss.RoundedBorder.
 *
 * The border is CSS rather than ╭─╮ glyphs: at a comfortable reading
 * line-height the box characters cannot touch each other vertically and
 * the frame comes out dashed. The element still occupies exactly the
 * rows and columns the terminal would give it, so everything around it
 * stays on the grid.
 */
export function TuiBox({
  lines,
  tone = 'accent',
  padX = 4,
  padY = 1,
  marginLeft = 2,
  bold = true,
}: {
  lines: string[]
  tone?: 'accent' | 'faint'
  padX?: number
  padY?: number
  marginLeft?: number
  bold?: boolean
}) {
  const inner = Math.max(...lines.map(cells)) + padX * 2
  const rows = lines.length + padY * 2 + 2
  const edge = tone === 'accent' ? 'border-tui-accent' : 'border-tui-faint'
  const text = tone === 'accent' ? 'text-tui-accent' : 'text-tui-muted'

  return (
    <div
      className="relative"
      style={{ marginLeft: col(marginLeft), width: col(inner + 2), height: row(rows) }}
    >
      <div className={`absolute inset-0 rounded-[0.5em] border ${edge}`} />
      <div
        className={`absolute ${text} ${bold ? 'font-bold' : ''}`}
        style={{ top: row(1 + padY), left: col(1 + padX) }}
      >
        {lines.map((l, i) => (
          <div key={i}>{l}</div>
        ))}
      </div>
    </div>
  )
}

/** buttonStyle / buttonSelStyle — 3 rows tall, 3 cells of padding each side. */
export function ButtonRow({
  buttons,
  selected,
}: {
  buttons: string[]
  selected: number
}) {
  return (
    <div className="relative flex" style={{ marginLeft: col(2), height: row(3) }}>
      {buttons.map((label, i) => {
        const sel = i === selected
        return (
          <div
            key={label}
            className="relative"
            style={{ width: col(cells(label) + 8), marginRight: i < buttons.length - 1 ? col(1) : 0 }}
          >
            <div
              className={`absolute inset-0 rounded-[0.5em] border ${
                sel ? 'border-tui-accent' : 'border-tui-faint'
              }`}
            />
            <div
              className={`absolute ${sel ? 'font-bold text-tui-accent' : 'text-tui-muted'}`}
              style={{ top: row(1), left: col(4) }}
            >
              {label}
            </div>
          </div>
        )
      })}
    </div>
  )
}

/**
 * [██████░░░░░░] — 30 cells wide, exactly the footprint progressBar()
 * gives it in Go, painted rather than tiled so the fill has no seams.
 */
export function ProgressBar({
  done,
  total,
  width = 30,
}: {
  done: number
  total: number
  width?: number
}) {
  const pct = Math.min(1, Math.max(0, done / Math.max(total, 1)))
  return (
    <span className="whitespace-pre">
      <span className="text-tui-faint">[</span>
      <span
        className="relative inline-block align-middle"
        style={{ width: col(width), height: '0.78em' }}
      >
        <span className="absolute inset-0 bg-tui-faint/40" />
        <span
          className="absolute inset-y-0 left-0 bg-tui-accent"
          style={{ width: `${pct * 100}%` }}
        />
        {/* Notches on the same 1-cell pitch the block glyphs would sit on,
            which is also what gives it the arcade power-meter read. */}
        <span
          aria-hidden
          className="absolute inset-0"
          style={{
            backgroundImage:
              'repeating-linear-gradient(to right, transparent 0 calc(1ch - 1px), var(--color-ink-950) calc(1ch - 1px) 1ch)',
          }}
        />
      </span>
      <span className="text-tui-faint">]</span>
    </span>
  )
}

export const Caret = () => (
  <span
    className="text-tui-accent"
    style={{ animation: 'ft-caret 1.06s steps(1) infinite' }}
  >
    █
  </span>
)
