const FEATURES = [
  {
    icon: '🔒',
    tone: 'cyan',
    title: 'Dosyaların bir yere yüklenmiyor',
    body: 'Buluşma sunucusu iki bilgisayarı tanıştırır, o kadar. Dosyanın tek bir baytı bile üzerinden geçmez — silinecek bir kopya hiç oluşmaz.',
  },
  {
    icon: '∞',
    tone: 'magenta',
    title: 'Boyut sınırı yok',
    body: '10 MB da olur, 40 GB da. Ücretli plan, günlük kota, "dosyanız çok büyük" ekranı yok. Sınır sadece internet hızın.',
  },
  {
    icon: '🕒',
    tone: 'yellow',
    title: 'Link süresi dolmaz, çünkü link yok',
    body: 'Paylaşılan şey üç kelimelik bir oda kodu. Bir saat sonra kod düşer, dosya hiçbir yerde asılı kalmaz.',
  },
  {
    icon: '🛡️',
    tone: 'green',
    title: 'Uçtan uca şifreli, üstelik doğrulanmış',
    body: 'Bağlantı libp2p ile şifrelenir; her dosya iniş sonrası SHA-256 ile tek tek doğrulanır. Bozuk inen dosya sessizce kabul edilmez.',
  },
  {
    icon: '🌍',
    tone: 'cyan',
    title: 'İki ayrı şehir, iki ev modemi — sorun değil',
    body: 'İki taraf da NAT arkasındayken bile doğrudan yol açılır (DCUtR hole punching). Açılamazsa transfer şifreli yedek yoldan tamamlanır.',
  },
  {
    icon: '📦',
    tone: 'magenta',
    title: 'Tek dosya, kurulum yok',
    body: 'İndirdiğin arşivden tek bir çalıştırılabilir dosya çıkar. Kurulum sihirbazı, bağımlılık, yönetici izni, arka planda çalışan servis yok.',
  },
] as const

const RING: Record<string, string> = {
  cyan: 'border-arc-cyan/50 text-arc-cyan',
  magenta: 'border-arc-magenta/50 text-arc-magenta',
  yellow: 'border-arc-yellow/50 text-arc-yellow',
  green: 'border-arc-green/50 text-arc-green',
}

export function Features() {
  return (
    <section id="ozellikler" className="relative mx-auto max-w-7xl scroll-mt-20 px-4 py-24 sm:px-8">
      <p className="hud text-[11px] text-arc-magenta glow-magenta">★ Neden böyle</p>
      <h2 className="mt-5 max-w-2xl text-3xl font-extrabold tracking-tight text-white text-balance sm:text-5xl">
        Dosya göndermenin normalde neyi rahatsız ediyorsa, o burada yok.
      </h2>

      <div className="mt-14 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
        {FEATURES.map((f) => (
          <div
            key={f.title}
            className="group border-2 border-arc-cyan/20 bg-ink-900 p-6 shadow-[5px_5px_0_0_rgba(0,0,0,0.5)] transition-all duration-200 hover:-translate-y-1 hover:border-arc-cyan/50 hover:shadow-[7px_9px_0_0_rgba(0,0,0,0.5)]"
          >
            <div
              className={`flex size-11 items-center justify-center border-2 bg-ink-850 text-lg ${RING[f.tone]}`}
            >
              {f.icon}
            </div>
            <h3 className="mt-5 text-[17px] font-bold text-white">{f.title}</h3>
            <p className="mt-2.5 text-sm leading-relaxed text-tui-muted">{f.body}</p>
          </div>
        ))}
      </div>
    </section>
  )
}
