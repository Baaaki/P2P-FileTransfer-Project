const QA = [
  {
    q: 'Dosyalarım gerçekten hiçbir sunucuya gitmiyor mu?',
    a: 'Gitmiyor. Sunucunun gördüğü tek şey oda kodu ve iki tarafın ağ adresleri — yani birkaç kilobayt. Dosyalar açılan doğrudan bağlantı üzerinden akar; sunucu kodu (cmd/server) transfer kodunu hiç çalıştırmaz.',
  },
  {
    q: 'İkimiz de ev internetindeyiz, farklı şehirlerdeyiz. Çalışır mı?',
    a: 'Evet, normal durum bu. İlk temas sunucunun köprüsü üzerinden kurulur, ardından NAT delme (DCUtR) devreye girip bağlantıyı doğrudan hale getirir. Delme başarısız olursa transfer şifreli yedek yoldan tamamlanır — biraz yavaşlar ama durmaz.',
  },
  {
    q: 'Kod ne kadar geçerli? Başkası tahmin edebilir mi?',
    a: 'Kod bir saat sonra düşer. Kod rastgele üretilir ve yalnızca bir transfer için geçerlidir; ayrıca dosyalar sen "Evet, indir" demeden inmeye başlamaz.',
  },
  {
    q: 'Karşı tarafın da aynı programı çalıştırması gerekiyor mu?',
    a: 'Evet — iki taraf da aynı tek dosyayı çalıştırır. Biri "göndereceğim", diğeri "bana gönderilecek" der. Kurulum ya da hesap ikisinde de yok.',
  },
  {
    q: 'Neden bir tarayıcı uygulaması değil de terminal programı?',
    a: 'Tarayıcı, iki bilgisayar arasında doğrudan yol açma konusunda kısıtlı ve büyük dosyalarda diske yazma tarafında sorunlu. Tek dosyalık bir program hem sınırsız boyutu hem de gerçek P2P bağlantıyı sorunsuz kaldırıyor.',
  },
  {
    q: 'Ücretli mi? Reklam, hesap, takip var mı?',
    a: 'Hayır. MIT lisanslı, açık kaynak. Hesap yok, telemetri yok, ücretli plan yok. Kodun tamamı GitHub’da okunabilir.',
  },
]

export function Faq() {
  return (
    <section className="relative mx-auto max-w-4xl px-4 py-24 sm:px-8">
      <p className="hud text-[11px] text-arc-green">★ Sık sorulanlar</p>
      <h2 className="mt-5 text-3xl font-extrabold tracking-tight text-white text-balance sm:text-4xl">
        Akla gelen ilk sorular
      </h2>

      <div className="mt-12 space-y-3">
        {QA.map((item) => (
          <details
            key={item.q}
            className="group border-2 border-arc-cyan/20 bg-ink-900 p-5 transition-colors open:border-arc-cyan/45"
          >
            <summary className="flex cursor-pointer list-none items-start justify-between gap-6 text-[15px] font-bold text-white marker:content-none">
              {item.q}
              <span className="mt-0.5 shrink-0 text-arc-yellow transition-transform group-open:rotate-45">
                ＋
              </span>
            </summary>
            <p className="mt-3 max-w-3xl text-[14px] leading-relaxed text-tui-muted">
              {item.a}
            </p>
          </details>
        ))}
      </div>
    </section>
  )
}
