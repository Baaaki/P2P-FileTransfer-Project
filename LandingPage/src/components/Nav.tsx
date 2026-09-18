import { useEffect, useState } from 'react'
import { REPO_URL } from '../lib/release'

const LINKS = [
  { href: '#nasil-calisir', label: 'Nasıl çalışır' },
  { href: '#ozellikler', label: 'Özellikler' },
  { href: '#indir', label: 'İndir' },
]

export function Nav() {
  const [solid, setSolid] = useState(false)

  useEffect(() => {
    const onScroll = () => setSolid(window.scrollY > 24)
    onScroll()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  return (
    <nav
      className={`fixed inset-x-0 top-0 z-50 transition-colors duration-300 ${
        solid ? 'border-b-2 border-arc-cyan/25 bg-ink-950/90 backdrop-blur-md' : 'border-b-2 border-transparent'
      }`}
    >
      <div className="mx-auto flex max-w-7xl items-center justify-between px-4 py-3 sm:px-8">
        <a href="#" className="flex items-center gap-2.5">
          <span
            className="text-base"
            style={{ animation: 'ft-marquee 3.2s ease-in-out infinite' }}
          >
            📦
          </span>
          <span className="pixel glow-cyan text-[10px] text-arc-cyan sm:text-xs">
            FILETRANSFERILLA
          </span>
        </a>

        <div className="flex items-center gap-1 sm:gap-2">
          {LINKS.map((l) => (
            <a
              key={l.href}
              href={l.href}
              className="hud hidden px-3 py-2 text-[10px] text-tui-muted transition-colors hover:text-arc-yellow lg:block"
            >
              {l.label}
            </a>
          ))}
          <a
            href={REPO_URL}
            target="_blank"
            rel="noreferrer"
            lang="en"
            className="hud px-3 py-2 text-[10px] text-tui-muted transition-colors hover:text-arc-yellow"
          >
            GitHub
          </a>
          <a
            href="#indir"
            className="hud ml-1 border-2 border-arc-magenta bg-arc-magenta px-3.5 py-2 text-[10px] text-ink-950 shadow-[3px_3px_0_0_#000] transition-all hover:translate-x-[2px] hover:translate-y-[2px] hover:shadow-[1px_1px_0_0_#000]"
          >
            İndir
          </a>
        </div>
      </div>
    </nav>
  )
}
