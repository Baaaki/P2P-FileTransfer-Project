import { REPO_URL, RELEASES_URL } from '../lib/release'

export function Footer() {
  return (
    <footer className="relative border-t-2 border-arc-cyan/20">
      <div className="mx-auto max-w-7xl px-4 py-16 sm:px-8">
        <div className="relative overflow-hidden border-2 border-arc-magenta/45 bg-ink-900 px-6 py-14 text-center shadow-[7px_7px_0_0_rgba(0,0,0,0.55)] sm:px-12">
          <div
            aria-hidden
            className="arc-floor pointer-events-none absolute inset-0 opacity-25"
          />
          <div className="relative">
            <p className="hud text-[11px] text-arc-yellow">Game over? Hayır — başla</p>
            <h2 className="mx-auto mt-5 max-w-xl text-2xl font-extrabold tracking-tight text-white text-balance sm:text-4xl">
              Bir sonraki dosyanı buluta yüklemeden gönder.
            </h2>
            <p className="mx-auto mt-4 max-w-lg text-[15px] text-tui-muted">
              İndir, çalıştır, üç kelimeyi ilet. Toplam süre: bir dakika.
            </p>
            <a
              href="#indir"
              className="hud mt-9 inline-block border-2 border-arc-yellow bg-arc-yellow px-7 py-4 text-[12px] text-ink-950 shadow-[5px_5px_0_0_#000] transition-all hover:translate-x-[3px] hover:translate-y-[3px] hover:shadow-[2px_2px_0_0_#000]"
            >
              ▶ Ücretsiz indir
            </a>
          </div>
        </div>

        <div className="mt-12 flex flex-col items-center justify-between gap-5 sm:flex-row">
          <p className="pixel text-center text-[8px] text-tui-faint sm:text-left">
            FILETRANSFERILLA &middot; MIT &middot; LIBP2P
          </p>
          <div className="hud flex gap-5 text-[10px]">
            <a href={REPO_URL} target="_blank" rel="noreferrer" lang="en" className="text-tui-muted hover:text-arc-yellow">
              GitHub
            </a>
            <a href={RELEASES_URL} target="_blank" rel="noreferrer" className="text-tui-muted hover:text-arc-yellow">
              Sürümler
            </a>
            <a
              href={`${REPO_URL}/blob/main/README.en.md`}
              target="_blank"
              rel="noreferrer"
              lang="en"
              className="text-tui-muted hover:text-arc-yellow"
            >
              English
            </a>
          </div>
        </div>
      </div>
    </footer>
  )
}
