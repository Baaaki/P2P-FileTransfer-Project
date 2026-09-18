# LandingPage

FileTransferilla'nın tanıtım sayfası. React + Vite + Tailwind CSS v4.

Sayfanın merkezinde, programın kendi arayüzünün **tarayıcıda yeniden
çizilmiş** hâli var: aşağı kaydırdıkça uygulama kendi kendine baştan sona
işliyor — açılış ekranından dosya seçimine, oda kodundan transferin
bitişine kadar. Ekran görüntüsü ya da video değil; `internal/tui` içindeki
metinlerin ve renklerin birebir karşılığı.

> Bu klasör ana projeye dokunmaz. Go tarafında hiçbir değişiklik yok.

## Çalıştır

```bash
cd LandingPage
npm install
npm run dev        # http://localhost:5173
```

```bash
npm run build      # dist/ üretir
npm run preview    # üretilen dist'i servis eder
```

Alt yolda yayınlanacaksa (ör. GitHub Pages proje sayfası):

```bash
BASE_URL=/P2P-FileTransfer-Project/ npm run build
```

## Nasıl kurulu

```
src/
  tui/            programın arayüzünün tarayıcı karşılığı
    text.ts       terminal hücre genişliği, formatBytes, easing
    primitives.tsx  lipgloss stillerinin birebir karşılığı (styles.go)
    screens.tsx   10 ekran — metinler tui.go'daki dizelerle aynı
  story/steps.tsx kaydırma senaryosu: hangi adımda hangi ekran, hangi anlatım
  lib/
    useScrollStory.ts  kaydırma konumunu oynatma kafasına çeviren hook
    release.ts    GitHub Releases'ten güncel sürümü ve dosyaları çeker
    platform.ts   ziyaretçinin işletim sistemini tahmin eder
  components/     sayfa bölümleri
```

### Kaydırma nasıl "video gibi" ilerliyor

`useScrollStory`, bölümün kaydırma konumunu 0→1 aralığına çevirir ve bunu
adım sayısıyla çarpar: tam sayı kısmı hangi ekranın çizileceğini, ondalık
kısmı o ekranın içindeki ilerlemeyi (yazılan kod, dolan çubuk, değişen
durum satırı) belirler. Ham kaydırma değeri doğrudan kullanılmaz; üstel bir
takipçiyle yumuşatılır — tekerlek tıkırtısı sayfayı bir anda kaydırdığı
için, animasyonu doğrudan ona bağlamak kesik kesik görünür.

`prefers-reduced-motion` açıksa yumuşatma devre dışı kalır ve süslemeli
animasyonlar durur.

### İndirme bağlantıları

Arşiv adları sürüm numarası taşıdığı için (`.goreleaser.yaml`) sabit
bağlantı verilemez. Sayfa açılırken GitHub Releases API'sinden güncel sürüm
çekilir ve her işletim sistemi için doğru dosya bağlanır. Henüz yayımlanmış
sürüm yoksa veya istek başarısız olursa bağlantılar Releases sayfasına gider
ve dosya adları örnek biçimiyle gösterilir.

Depo adresi tek yerde tanımlı: `src/lib/release.ts` içindeki `REPO`.

## Tema ve yazı tipleri

Tema arcade: neon camgöbeği / macenta / sarı, sert kenarlıklar, gölgesi
kaymış 8-bit kutular, CRT tarama çizgileri. Terminal penceresi bir kabin
ekranı gibi çerçevelenir.

Renkler iki aileye ayrılır:

- **`tui-*`** — ekranın kendisi. `internal/tui/styles.go` içindeki anlamsal
  rolleri (accent, ok, warn, err, muted, faint, default) birebir korur,
  yalnızca tonu değişir; bir terminal renk şemasının yaptığı şey.
- **`arc-*`** — kabin. Sayfa çerçevesinde kullanılır, program penceresinin
  içine hiç girmez.

Yazı tiplerinde bir tuzak var: **Press Start 2P** 8 piksellik hücreye
sığmadığı için `İ`, `Ö`, `Ü` harflerini x-yüksekliğinde çiziyor —
"ÖZELLİKLER" ekranda "öZELLiKLER" gibi görünüyor. Bu yüzden `pixel` sınıfı
yalnızca yapısı gereği ASCII olan dizelerde kullanılır (`STAGE`, `1P`,
`PROG`, logo). Türkçe etiketlerin tamamı `hud` sınıfını kullanır: Inter 800,
harf aralığı açılmış, büyük harf.

İkinci tuzak: sayfa `lang="tr"` olduğu için `text-transform: uppercase`
Türkçe kurallarını uygular ve "GitHub" → "GİTHUB", "glibc" → "GLİBC" olur.
Marka ve teknik terimler ya `lang="en"` ile işaretli ya da büyük harfe hiç
çevrilmiyor.

## Dış bağımlılık

Sayfa üçüncü taraf bir sunucuya istek atmaz; yazı tipleri (Inter, Press
Start 2P) paketle birlikte gelir. Terminal metni bilerek ziyaretçinin kendi
monospace yazı tipini kullanır — kutu çizgisi karakterlerinin tam karşılığı
olan yazı tipi odur. Tek dış istek, güncel sürümü öğrenmek için GitHub
API'sine gidendir.

## `public/server.txt`

Sayfayla birlikte yayınlanan küçük bir metin dosyası:
`https://p2p-filetransfer.madebybaki.com/server.txt`. Sayfanın kendisi onu
kullanmaz; **yayınlanan istemciler** kullanır. İçine gömülü sunucu adresi
cevap vermeyen bir istemci buradaki adresleri dener — sunucu taşınır ya da
kimliği değişirse, bu dosyayı güncellemek daha önce indirilmiş bütün
kopyaları yaşatır.

Her satıra bir multiaddr, `#` ile başlayanlar yorum. Sunucuyu deploy ettikten
sonra açılışta bastığı istemci adresini buraya ekle. CI, dosyanın build
çıktısına girdiğini kontrol eder (`.github/workflows/landing.yml`).
