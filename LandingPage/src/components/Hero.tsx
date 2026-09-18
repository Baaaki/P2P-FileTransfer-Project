import { useEffect, useState } from 'react'
import { REPO_URL } from '../lib/release'
import { RoomCode } from '../tui/screens'
import { Terminal } from './Terminal'

const GitHubMark = ({ className = '' }: { className?: string }) => (
  <svg viewBox="0 0 16 16" aria-hidden className={className} fill="currentColor">
    <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82a7.4 7.4 0 0 1 2-.27c.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z" />
  </svg>
)

export function Hero() {
  // A slow spinner tick so the window is alive before the story starts.
  const [frame, setFrame] = useState(0)
  useEffect(() => {
    const id = setInterval(() => setFrame((f) => f + 1), 120)
    return () => clearInterval(id)
  }, [])

  return (
    <header className="relative overflow-hidden pt-24 pb-16 sm:pt-32 sm:pb-24">
      <div aria-hidden className="arc-floor pointer-events-none absolute inset-0 -z-20 opacity-45" />
      <div
        aria-hidden
        className="pointer-events-none absolute inset-x-0 top-0 -z-10 h-[44rem]"
        style={{
          background:
            'radial-gradient(46% 46% at 50% 8%, rgba(255,46,136,0.20), transparent 70%), radial-gradient(52% 46% at 50% 30%, rgba(34,230,255,0.16), transparent 72%)',
        }}
      />

      <div className="mx-auto max-w-6xl px-4 text-center sm:px-8">
        <a
          href={REPO_URL}
          target="_blank"
          rel="noreferrer"
          className="hud inline-flex items-center gap-2 border-2 border-arc-green/40 bg-ink-900 px-3.5 py-2.5 text-[10px] text-arc-green transition-colors hover:border-arc-green"
        >
          <span
            className="size-1.5 bg-arc-green"
            style={{ animation: 'ft-blink 1.4s steps(1) infinite' }}
          />
          Açık kaynak · <span lang="en">libp2p</span> · MIT
        </a>

        <h1 className="mx-auto mt-8 max-w-4xl text-4xl leading-[1.06] font-extrabold tracking-tight text-white text-balance sm:text-6xl lg:text-7xl">
          Dosyanı buluta değil,{' '}
          <span className="text-arc-magenta glow-magenta">doğrudan arkadaşına</span> gönder.
        </h1>

        <p className="mx-auto mt-6 max-w-2xl text-base leading-relaxed text-tui-muted text-pretty sm:text-lg">
          Kurulum yok, hesap yok, boyut sınırı yok. Üç kelimelik bir kod verirsin —
          dosya senin bilgisayarından çıkar, arkadaşının bilgisayarına iner.
          Arada duran hiçbir sunucu yok.
        </p>

        <div className="mt-10 flex flex-wrap items-center justify-center gap-3">
          <a
            href="#indir"
            className="hud border-2 border-arc-yellow bg-arc-yellow px-6 py-4 text-[12px] text-ink-950 shadow-[5px_5px_0_0_#000] transition-all hover:translate-x-[3px] hover:translate-y-[3px] hover:shadow-[2px_2px_0_0_#000]"
          >
            ▶ Ücretsiz indir
          </a>
          <a
            href="#nasil-calisir"
            className="hud border-2 border-arc-cyan/60 px-6 py-4 text-[12px] text-arc-cyan shadow-[5px_5px_0_0_rgba(0,0,0,0.5)] transition-all hover:border-arc-cyan hover:bg-arc-cyan/10 hover:translate-x-[3px] hover:translate-y-[3px] hover:shadow-[2px_2px_0_0_rgba(0,0,0,0.5)]"
          >
            Nasıl çalışır
          </a>
          <a
            href={REPO_URL}
            target="_blank"
            rel="noreferrer"
            className="hud inline-flex items-center gap-2 px-4 py-4 text-[12px] text-tui-muted transition-colors hover:text-arc-yellow"
          >
            <GitHubMark className="size-3.5" />
            Kaynak
          </a>
        </div>

        <p className="pixel mt-7 text-[8px] text-tui-faint sm:text-[9px]">
          WINDOWS &middot; MACOS &middot; LINUX
        </p>
      </div>

      <div className="relative mx-auto mt-14 max-w-3xl px-4 sm:px-8">
        <Terminal title="filetransferilla — ayse@macbook" rows={21}>
          <RoomCode frame={frame} sub={1} />
        </Terminal>
      </div>

      <div className="mt-12 flex flex-col items-center gap-3">
        <span
          className="hud text-[11px] text-arc-yellow"
          style={{ animation: 'ft-blink 1.4s steps(1) infinite' }}
        >
          ▼ Kaydır, oyun başlasın
        </span>
      </div>
    </header>
  )
}
