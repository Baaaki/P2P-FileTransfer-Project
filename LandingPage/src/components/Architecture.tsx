/**
 * The same picture the README draws in ASCII, redrawn so the important
 * part is impossible to miss: the thin dashed lines are the only thing
 * that touches the server.
 */
export function Architecture() {
  return (
    <section className="relative mx-auto max-w-7xl px-4 py-24 sm:px-8">
      <div className="grid gap-12 lg:grid-cols-[22rem_minmax(0,1fr)] lg:items-center">
        <div>
          <p className="hud text-[11px] text-arc-cyan glow-cyan">★ Perde arkası</p>
          <h2 className="mt-5 text-3xl font-extrabold tracking-tight text-white text-balance sm:text-4xl">
            Sunucu dosyayı görmez, sadece tanıştırır.
          </h2>
          <p className="mt-5 text-[15px] leading-relaxed text-tui-muted">
            Gönderen, rastgele bir oda kodunu kendi ağ adresleriyle sunucuya bırakır.
            Alıcı aynı kodu sorunca adresleri alır. Sonrası iki bilgisayarın kendi
            arasında: NAT delinir, doğrudan bir yol açılır ve dosyalar oradan akar.
          </p>

          <dl className="mt-8 space-y-5">
            <div className="border-l-4 border-arc-yellow pl-4">
              <dt className="hud text-[10px] text-arc-yellow">Sunucudan geçen</dt>
              <dd className="mt-2 text-sm text-tui-muted">
                Birkaç KB — oda kaydı ve delme koordinasyonu.
              </dd>
            </div>
            <div className="border-l-4 border-arc-green pl-4">
              <dt className="hud text-[10px] text-arc-green">Dosyalardan geçen</dt>
              <dd className="mt-2 text-sm text-tui-muted">Hiçbiri.</dd>
            </div>
          </dl>
        </div>

        <div className="border-2 border-arc-cyan/25 bg-ink-900 p-4 shadow-[6px_6px_0_0_rgba(0,0,0,0.5)] sm:p-8">
          <svg
            viewBox="0 0 880 340"
            className="w-full"
            role="img"
            aria-label="Gönderen ve alıcı, buluşma sunucusu üzerinden tanışıp doğrudan bağlanır"
          >
            <defs>
              <linearGradient id="p2p" x1="0" x2="1">
                <stop offset="0%" stopColor="#22e6ff" />
                <stop offset="50%" stopColor="#ffd23f" />
                <stop offset="100%" stopColor="#ff2e88" />
              </linearGradient>
            </defs>

            {/* rendezvous hops */}
            <path
              d="M170 205 C 170 110, 300 80, 400 80"
              fill="none"
              stroke="#46617d"
              strokeWidth="2"
              strokeDasharray="6 6"
              style={{ animation: 'ft-dash 1.6s linear infinite' }}
            />
            <path
              d="M710 205 C 710 110, 580 80, 480 80"
              fill="none"
              stroke="#46617d"
              strokeWidth="2"
              strokeDasharray="6 6"
              style={{ animation: 'ft-dash 1.6s linear infinite' }}
            />

            {/* the direct route */}
            <path
              d="M250 250 L630 250"
              fill="none"
              stroke="url(#p2p)"
              strokeWidth="5"
              strokeDasharray="16 10"
              style={{ animation: 'ft-dash 1.1s linear infinite' }}
            />

            {/* rendezvous server */}
            <g>
              <rect x="360" y="46" width="160" height="68" fill="#0b1022" stroke="#46617d" strokeWidth="2" />
              <text x="440" y="76" textAnchor="middle" fill="#d9f5ff" fontSize="15">
                Buluşma sunucusu
              </text>
              <text x="440" y="97" textAnchor="middle" fill="#7f9bb8" fontSize="12">
                sadece adres defteri
              </text>
            </g>

            {/* peers */}
            <g>
              <rect x="60" y="205" width="200" height="90" fill="#111935" stroke="#22e6ff" strokeWidth="2" />
              <text x="160" y="243" textAnchor="middle" fill="#22e6ff" fontSize="16">
                Gönderen
              </text>
              <text x="160" y="266" textAnchor="middle" fill="#7f9bb8" fontSize="12">
                İstanbul
              </text>
            </g>
            <g>
              <rect x="620" y="205" width="200" height="90" fill="#111935" stroke="#ff2e88" strokeWidth="2" />
              <text x="720" y="243" textAnchor="middle" fill="#ff2e88" fontSize="16">
                Alıcı
              </text>
              <text x="720" y="266" textAnchor="middle" fill="#7f9bb8" fontSize="12">
                İzmir
              </text>
            </g>

            {/* labels */}
            <text x="185" y="150" fill="#7f9bb8" fontSize="12.5">
              1 · odayı aç
            </text>
            <text x="600" y="150" fill="#7f9bb8" fontSize="12.5">
              2 · kodu sor
            </text>
            <text x="440" y="233" textAnchor="middle" fill="#ffd23f" fontSize="13.5">
              3 · doğrudan bağlantı
            </text>
            <text x="440" y="288" textAnchor="middle" fill="#7f9bb8" fontSize="12.5">
              dosyalar yalnızca buradan akar
            </text>
          </svg>
        </div>
      </div>
    </section>
  )
}
