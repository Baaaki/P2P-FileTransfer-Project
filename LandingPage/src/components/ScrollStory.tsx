import { useRef } from 'react'
import { useScrollStory } from '../lib/useScrollStory'
import { HOSTNAME, STEPS } from '../story/steps'
import { Terminal } from './Terminal'

/**
 * The centrepiece: a tall section whose scroll position is the playhead.
 * The window stays pinned in the middle of the screen while the program
 * runs through itself, one screen per scroll-length — and the chrome
 * around it reads like a cabinet HUD: which player, which stage, how far
 * through the run.
 */
export function ScrollStory() {
  const ref = useRef<HTMLElement>(null)
  const { index, sub, progress, frame } = useScrollStory(ref, STEPS.length)

  const step = STEPS[index]
  const { Screen } = step
  const p1 = step.side === 'send'

  return (
    <section
      id="nasil-calisir"
      ref={ref}
      className="relative scroll-mt-20"
      style={{ height: `${STEPS.length * 95 + 40}vh` }}
    >
      <div className="sticky top-0 flex h-screen items-center overflow-hidden">
        {/* The cabinet lights change side with the player. */}
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 -z-10 transition-[background] duration-700"
          style={{
            background: p1
              ? 'radial-gradient(65% 55% at 18% 32%, rgba(34,230,255,0.13), transparent 70%)'
              : 'radial-gradient(65% 55% at 82% 36%, rgba(255,46,136,0.13), transparent 70%)',
          }}
        />
        <div aria-hidden className="arc-floor pointer-events-none absolute inset-0 -z-20 opacity-25" />

        <div className="mx-auto grid w-full max-w-7xl items-center gap-8 px-4 sm:px-8 lg:grid-cols-[20rem_minmax(0,1fr)] lg:gap-14">
          {/* ---- HUD column -------------------------------------------- */}
          <div className="order-2 min-w-0 lg:order-1">
            <div className="flex flex-wrap items-center gap-2">
              <span
                className={`hud border-2 px-3 py-1.5 text-[10px] transition-colors duration-500 ${
                  p1
                    ? 'border-arc-cyan/60 bg-arc-cyan/10 text-arc-cyan'
                    : 'border-arc-magenta/60 bg-arc-magenta/10 text-arc-magenta'
                }`}
              >
                {p1 ? '1P · Gönderen' : '2P · Alan'}
              </span>
              <span className="pixel text-[8px] text-arc-yellow">
                STAGE {String(index + 1).padStart(2, '0')}/{String(STEPS.length).padStart(2, '0')}
              </span>
            </div>

            <div key={step.id} style={{ animation: 'ft-fade-up 520ms ease-out both' }}>
              <h3 className="mt-5 text-2xl font-extrabold tracking-tight text-white sm:text-3xl">
                {step.title}
              </h3>
              <p className="mt-3 max-w-md text-[15px] leading-relaxed text-tui-muted">
                {step.body}
              </p>
            </div>

            {/* Stage select — lit blocks for cleared stages. */}
            <ol className="mt-8 hidden gap-2 lg:flex lg:flex-col">
              {STEPS.map((s, i) => (
                <li key={s.id} className="flex items-center gap-3">
                  <span
                    className={`size-2.5 transition-colors duration-300 ${
                      i === index
                        ? 'bg-arc-yellow'
                        : i < index
                          ? 'bg-arc-cyan/60'
                          : 'bg-ink-700'
                    }`}
                  />
                  <span
                    className={`hud text-[10px] transition-colors duration-300 ${
                      i === index
                        ? 'text-arc-yellow'
                        : i < index
                          ? 'text-tui-muted'
                          : 'text-tui-faint/60'
                    }`}
                  >
                    {s.tag}
                  </span>
                </li>
              ))}
            </ol>
          </div>

          {/* ---- the program ------------------------------------------- */}
          <div className="order-1 min-w-0 lg:order-2">
            <Terminal title={`filetransferilla — ${HOSTNAME[step.side]}`} rows={24}>
              <Screen frame={frame} sub={sub} />
            </Terminal>

            {/* Energy meter: makes it obvious the scroll is the playhead. */}
            <div className="mt-4 flex items-center gap-3">
              <span className="pixel text-[8px] text-arc-cyan">PROG</span>
              <div className="flex flex-1 gap-[3px]">
                {Array.from({ length: 28 }, (_, i) => {
                  const lit = progress * 28 > i
                  return (
                    <span
                      key={i}
                      className={`h-2.5 flex-1 transition-colors duration-150 ${
                        lit ? (i > 22 ? 'bg-arc-magenta' : 'bg-arc-cyan') : 'bg-ink-800'
                      }`}
                    />
                  )
                })}
              </div>
              <span className="hud hidden text-[10px] text-tui-faint sm:inline">
                ↓ Kaydır
              </span>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
