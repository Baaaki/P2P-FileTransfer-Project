import { useEffect, useState, type RefObject } from 'react'
import { clamp } from '../tui/text'

export type StoryState = {
  /** Which step the viewport is parked on. */
  index: number
  /** 0→1 progress inside that step — this is what animates the screens. */
  sub: number
  /** 0→1 across the whole section, for the progress rail. */
  progress: number
  /** Spinner frame, advanced every 120ms just like tick() in the TUI. */
  frame: number
}

/**
 * Turns vertical scroll into playback position.
 *
 * The raw scroll offset is chased by an exponential follower rather than
 * used directly: a wheel notch moves the page in one jump, and stepping
 * the animation straight off that reads as a stutter. Easing toward the
 * target makes the same gesture look like a video being scrubbed.
 */
export function useScrollStory(
  ref: RefObject<HTMLElement | null>,
  stepCount: number,
): StoryState {
  const [state, setState] = useState<StoryState>({
    index: 0,
    sub: 0,
    progress: 0,
    frame: 0,
  })

  useEffect(() => {
    const el = ref.current
    if (!el) return

    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    let raf = 0
    let current = 0
    let last = performance.now()
    let spinAcc = 0
    let frame = 0
    let prev = ''

    const targetNow = () => {
      const span = el.offsetHeight - window.innerHeight
      if (span <= 0) return 0
      return clamp(-el.getBoundingClientRect().top / span)
    }

    const loop = (now: number) => {
      const dt = Math.min(64, now - last)
      last = now

      const target = targetNow()
      // Frame-rate independent lerp: same feel at 60Hz and 144Hz.
      current = reduced
        ? target
        : current + (target - current) * (1 - Math.pow(0.86, dt / 16.67))

      spinAcc += dt
      while (spinAcc >= 120) {
        spinAcc -= 120
        frame++
      }

      const f = current * stepCount
      const index = clamp(Math.floor(f), 0, stepCount - 1)
      const sub = clamp(f - index)
      // Quantise so we only re-render when something visibly changed.
      const q = Math.round(sub * 240)
      const key = `${index}:${q}:${frame}`
      if (key !== prev) {
        prev = key
        setState({ index, sub: q / 240, progress: current, frame })
      }

      raf = requestAnimationFrame(loop)
    }

    const start = () => {
      if (raf) return
      last = performance.now()
      raf = requestAnimationFrame(loop)
    }
    const stop = () => {
      cancelAnimationFrame(raf)
      raf = 0
    }

    // Only burn frames while the story is anywhere near the viewport.
    const io = new IntersectionObserver(
      ([entry]) => (entry.isIntersecting ? start() : stop()),
      { rootMargin: '200px 0px' },
    )
    io.observe(el)

    // Jump straight to the right position on a mid-page reload.
    current = targetNow()

    return () => {
      io.disconnect()
      stop()
    }
  }, [ref, stepCount])

  return state
}
