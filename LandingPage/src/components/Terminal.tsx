import type { ReactNode } from 'react'
import { LINE_H } from '../tui/text'

/**
 * The program window, mounted like a screen in a cabinet: a bezel, a lit
 * marquee strip along the top, and scanlines over the glass. The screen
 * itself is left alone — the chrome never bleeds into the character grid.
 */
export function Terminal({
  title,
  rows = 25,
  children,
  className = '',
  glow = true,
}: {
  title: string
  /** Fixed height in text rows, so the window never resizes mid-story. */
  rows?: number
  children: ReactNode
  className?: string
  glow?: boolean
}) {
  return (
    <div className={`tui-auto relative ${className}`}>
      {glow && (
        <div
          aria-hidden
          className="pointer-events-none absolute -inset-8 -z-10 opacity-80 blur-3xl"
          style={{
            background:
              'radial-gradient(55% 50% at 50% 45%, rgba(34,230,255,0.22), transparent 70%)',
          }}
        />
      )}

      <div className="border-2 border-arc-cyan/35 bg-ink-850 p-1.5 shadow-[7px_7px_0_0_rgba(0,0,0,0.6)] sm:p-2">
        {/* marquee */}
        <div className="flex items-center gap-2 border-b-2 border-arc-cyan/25 bg-ink-900 px-3 py-2">
          <span className="size-2.5 bg-arc-red" />
          <span className="size-2.5 bg-arc-yellow" />
          <span className="size-2.5 bg-arc-green" />
          <span className="ml-2 truncate font-mono text-[11px] tracking-wide text-arc-cyan/70">
            {title}
          </span>
        </div>

        {/* screen */}
        <div className="relative bg-ink-950">
          <div
            className="tui-grid overflow-x-auto px-3 py-4 text-tui-fg sm:px-5"
            style={{
              fontSize: 'var(--tui-size, 13px)',
              lineHeight: LINE_H,
              height: `calc(${rows} * ${LINE_H}em + 2rem)`,
            }}
          >
            {children}
          </div>
          <div
            aria-hidden
            className="scanlines pointer-events-none absolute inset-0 opacity-25 mix-blend-overlay"
          />
          {/* corner vignette, the way a curved tube darkens at the edges */}
          <div
            aria-hidden
            className="pointer-events-none absolute inset-0"
            style={{
              background:
                'radial-gradient(120% 120% at 50% 50%, transparent 55%, rgba(0,0,0,0.55) 100%)',
            }}
          />
        </div>
      </div>
    </div>
  )
}
