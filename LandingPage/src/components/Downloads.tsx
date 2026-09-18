import { useMemo } from 'react'
import { detectPlatform } from '../lib/platform'
import {
  RELEASES_URL,
  assetFor,
  exampleName,
  useLatestRelease,
  type PlatformId,
} from '../lib/release'
import { formatBytes } from '../tui/text'

const Windows = () => (
  <svg viewBox="0 0 24 24" className="size-6" fill="currentColor" aria-hidden>
    <path d="M0 3.45 9.75 2.1v9.4H0Zm10.95-1.5L24 0v11.4H10.95ZM0 12.6h9.75V22L0 20.65ZM10.95 12.6H24V24l-13.05-1.8Z" />
  </svg>
)

const Apple = () => (
  <svg viewBox="0 0 24 24" className="size-6" fill="currentColor" aria-hidden>
    <path d="M16.36 12.72c-.02-2.36 1.93-3.5 2.02-3.55-1.1-1.61-2.81-1.83-3.42-1.85-1.46-.15-2.84.86-3.58.86s-1.88-.84-3.09-.82c-1.59.02-3.06.92-3.88 2.34-1.65 2.87-.42 7.12 1.19 9.45.79 1.14 1.72 2.42 2.95 2.37 1.19-.05 1.64-.77 3.07-.77s1.84.77 3.09.74c1.28-.02 2.08-1.16 2.86-2.31.9-1.32 1.27-2.6 1.29-2.67-.03-.01-2.48-.95-2.5-3.79ZM14.02 5.4c.65-.79 1.09-1.89.97-2.99-.94.04-2.07.63-2.75 1.41-.6.7-1.13 1.82-.99 2.9 1.05.08 2.12-.53 2.77-1.32Z" />
  </svg>
)

const Linux = () => (
  <svg viewBox="0 0 24 24" className="size-6" aria-hidden>
    <g fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinejoin="round">
      <path d="M8.6 9.2C8.6 5.3 9.5 2.4 12 2.4s3.4 2.9 3.4 6.8c0 1.6 1 2.7 1.9 4.3.9 1.6 1.6 3 2.1 4 .4.8.5 1.6-.2 2-.8.5-1.7-.1-2.3-.9" />
      <path d="M7.1 18.6c-.6.8-1.5 1.4-2.3.9-.7-.4-.6-1.2-.2-2 .5-1 1.2-2.4 2.1-4 .9-1.6 1.9-2.7 1.9-4.3" />
      <path d="M7.1 18.6c.6-1.2 1.2-2.4 1.2-3.4 0-1.4 1.4-2.6 3.7-2.6s3.7 1.2 3.7 2.6c0 1 .6 2.2 1.2 3.4-.9 1.4-2.7 2.4-4.9 2.4s-4-1-4.9-2.4Z" />
      <path d="M10.1 6.3v.6M13.9 6.3v.6" strokeLinecap="round" />
      <path d="M10.4 9.4c0-.5.7-.9 1.6-.9s1.6.4 1.6.9c0 .4-1.1 1.2-1.6 1.2s-1.6-.8-1.6-1.2Z" />
    </g>
  </svg>
)

type Card = {
  id: PlatformId
  name: string
  note: string
  Icon: () => React.ReactElement
  run: string
}

const CARDS: Card[] = [
  {
    id: 'windows',
    name: 'Windows',
    note: '10 / 11 · 64 bit',
    Icon: Windows,
    run: 'Zip’ten çıkar, dosyaya çift tıkla.',
  },
  {
    id: 'macos-arm64',
    name: 'macOS',
    note: 'Apple Silicon · M1–M4',
    Icon: Apple,
    run: 'Arşivden çıkar, çift tıkla — Terminal’de açılır.',
  },
  {
    id: 'macos-x86_64',
    name: 'macOS',
    note: 'Intel · 2020 öncesi',
    Icon: Apple,
    run: 'Arşivden çıkar, çift tıkla — Terminal’de açılır.',
  },
  {
    id: 'linux',
    name: 'Linux',
    note: 'x86_64 · glibc gerekmez',
    Icon: Linux,
    run: 'chmod +x filetransferilla && ./filetransferilla',
  },
]

export function Downloads() {
  const detected = useMemo(detectPlatform, [])
  const { release, loading } = useLatestRelease()

  // Put the visitor's own platform first; on macOS keep Apple Silicon
  // ahead of Intel rather than reshuffling the whole row.
  const cards = useMemo(() => {
    if (!detected) return CARDS
    return [...CARDS].sort(
      (a, b) => Number(b.id === detected) - Number(a.id === detected),
    )
  }, [detected])

  return (
    <section id="indir" className="relative mx-auto max-w-7xl scroll-mt-20 px-4 py-24 sm:px-8">
      <div className="text-center">
        <p className="pixel text-[9px] text-arc-yellow">&gt;&gt; SELECT YOUR PLATFORM</p>
        <h2 className="mx-auto mt-5 max-w-2xl text-3xl font-extrabold tracking-tight text-white text-balance sm:text-5xl">
          İşletim sistemini seç, tek dosyayı indir.
        </h2>
        <p className="mx-auto mt-5 max-w-xl text-[15px] leading-relaxed text-tui-muted">
          Arşivin içinden <code className="font-mono text-tui-fg">filetransferilla</code>{' '}
          adında tek bir dosya çıkar. Başka hiçbir şeye ihtiyacın yok.
        </p>

        <p className="mt-6 font-mono text-xs text-tui-faint">
          {loading
            ? 'Sürüm bilgisi alınıyor…'
            : release
              ? `En güncel sürüm: ${release.tag}${
                  release.publishedAt
                    ? ` · ${new Date(release.publishedAt).toLocaleDateString('tr-TR')}`
                    : ''
                }`
              : 'İlk sürüm henüz yayımlanmadı — bağlantılar Releases sayfasına gider.'}
        </p>
      </div>

      <div className="mt-14 grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
        {cards.map((c) => {
          const asset = assetFor(release, c.id)
          const href = asset?.url ?? RELEASES_URL
          const mine = detected === c.id
          return (
            <a
              key={c.id}
              href={href}
              className={`group relative flex flex-col border-2 bg-ink-900 p-6 transition-all duration-200 hover:-translate-y-1 hover:shadow-[7px_9px_0_0_rgba(0,0,0,0.55)] ${
                mine
                  ? 'border-arc-yellow bg-arc-yellow/[0.06] shadow-[5px_5px_0_0_rgba(255,210,63,0.28)]'
                  : 'border-arc-cyan/25 shadow-[5px_5px_0_0_rgba(0,0,0,0.5)] hover:border-arc-cyan/60'
              }`}
            >
              {mine && (
                <span className="hud absolute -top-3 left-5 bg-arc-yellow px-2.5 py-1 text-[10px] text-ink-950">
                  1P · Senin sistemin
                </span>
              )}
              <div
                className={`transition-colors ${mine ? 'text-arc-yellow' : 'text-tui-fg group-hover:text-arc-cyan'}`}
              >
                <c.Icon />
              </div>
              <h3 className="mt-4 text-lg font-bold text-white">{c.name}</h3>
              <p className="mt-2 font-mono text-[10px] tracking-wide text-tui-faint">{c.note}</p>

              <p className="mt-5 flex-1 text-[13px] leading-relaxed text-tui-muted">{c.run}</p>

              <div className="mt-6 flex items-center justify-between border-t-2 border-arc-cyan/20 pt-4">
                <span className="hud text-[11px] text-arc-cyan">
                  {asset ? '▶ İndir' : '▶ Releases'}
                </span>
                <span className="pixel text-[8px] text-tui-faint">
                  {asset ? formatBytes(asset.size) : '—'}
                </span>
              </div>
              <p className="mt-2 truncate font-mono text-[10px] text-tui-faint/80">
                {asset?.name ?? exampleName(c.id)}
              </p>
            </a>
          )
        })}
      </div>

      <div className="mt-10 grid items-start gap-4 lg:grid-cols-2">
        <details className="group border-2 border-arc-yellow/35 bg-ink-900 p-6 shadow-[5px_5px_0_0_rgba(0,0,0,0.5)]">
          <summary className="cursor-pointer list-none text-sm font-bold text-white marker:content-none">
            <span className="text-arc-yellow">⚠</span> İlk açılışta bir uyarı çıkarsa
            <span className="float-right text-tui-faint transition group-open:rotate-180">⌄</span>
          </summary>
          <div className="mt-4 space-y-4 text-[13px] leading-relaxed text-tui-muted">
            <p>
              Program <strong className="text-tui-fg">imzalı olmadığı için</strong> işletim
              sistemi ilk seferde soru sorabilir. Bu bir hata değil — imza sertifikaları
              ücretli olduğu için alınmadı. Bir kez izin verdikten sonra bir daha sorulmaz.
            </p>
            <p>
              <strong className="text-tui-fg">macOS</strong> — “geliştirici doğrulanamadı”
              diyorsa dosyaya <strong className="text-tui-fg">sağ tıkla → Aç</strong>, çıkan
              pencerede tekrar <em>Aç</em>’a bas. Sadece çift tıklamak bu durumda yetmez.
            </p>
            <p>
              <strong className="text-tui-fg">Windows</strong> — “Windows kişisel
              bilgisayarınızı korudu” ekranında{' '}
              <strong className="text-tui-fg">Ek bilgi → Yine de çalıştır</strong>.
            </p>
            <p>
              <strong className="text-tui-fg">Linux</strong> — çalıştırma izni gerekebilir:{' '}
              <code className="bg-ink-950 px-1.5 py-0.5 font-mono text-arc-cyan">
                chmod +x filetransferilla
              </code>
            </p>
          </div>
        </details>

        <div className="border-2 border-arc-green/35 bg-ink-900 p-6 shadow-[5px_5px_0_0_rgba(0,0,0,0.5)]">
          <h3 className="text-sm font-bold text-white">
            <span className="text-arc-green">✓</span> İndirdiğini doğrulamak istersen
          </h3>
          <p className="mt-4 text-[13px] leading-relaxed text-tui-muted">
            Her sürümün yanında bir <code className="font-mono text-tui-fg">checksums.txt</code>{' '}
            dosyası var. İndirdiğin arşivin bozulmadığını tek komutla doğrulayabilirsin:
          </p>
          <pre className="mt-4 overflow-x-auto border-2 border-arc-cyan/20 bg-ink-950 px-4 py-3 font-mono text-[12px] text-tui-fg">
            sha256sum -c checksums.txt --ignore-missing
          </pre>
          <a
            href={release?.checksums ?? RELEASES_URL}
            className="pixel mt-4 inline-block text-[8px] text-arc-cyan hover:text-arc-yellow"
          >
            checksums.txt →
          </a>
        </div>
      </div>
    </section>
  )
}
