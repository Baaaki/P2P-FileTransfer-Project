# PureSend 📦

[libp2p](https://libp2p.io) üzerine kurulu, uçtan uca P2P dosya transfer
uygulaması. İki bilgisayar küçük bir buluşma (rendezvous) sunucusu üzerinden
birbirini bulur; ardından dosyalar **sunucuya hiç uğramadan, doğrudan iki
bilgisayar arasında** aktarılır — farklı ağlarda, ikisi de ev modemi (NAT)
arkasında olsa bile.

Kullanıcı hiçbir teknik şey görmez: programı açar, dosyayı seçer, ekranda
çıkan **3 kelimelik kodu** arkadaşına iletir. Hepsi bu.

Kod aynı zamanda **paroladır**: iki uç, kodu bildiklerini birbirine
kanıtlamadan tek bir bayt bile akmaz. Buluşma sunucusu bu yüzden güvenilir
bir taraf değildir — yalan söylerse transfer başlamaz.

🇬🇧 English documentation: [README.en.md](README.en.md)  
🌐 Web sitesi & Canlı simülasyon: [puresend.madebybaki.com](https://puresend.madebybaki.com)  
📋 Ürünleşme planı: [ROADMAP.md](docs/ROADMAP.md)

```
Gönderen (İstanbul)          Buluşma sunucusu           Alıcı (İzmir)
       │                            │                          │
       │ 1. "kiraz-liman-42"        │                          │
       │    odasını aç ────────────►│◄─── 2. "kiraz-liman-42"  │
       │                            │        odasında kim var? │
       │                            │                          │
       │◄═══ 3. Doğrudan P2P bağlantı (NAT delme) ════════════►│
       │              4. Dosyalar doğrudan akar                │
```

## İndir ve çalıştır

Kurulum yok, ayar yok, hesap açmak yok.
[**Releases**](https://github.com/Baaaki/PureSend/releases/latest)
sayfasından işletim sistemine uygun dosyayı indir, arşivden çıkar, çalıştır.

| İşletim sistemin | İndireceğin dosya |
|---|---|
| Ubuntu / Debian / Mint (`.deb`) | `puresend_<sürüm>_amd64.deb` (ARM için `arm64.deb`) |
| Linux (Taşınabilir ikili) | `puresend_<sürüm>_linux_x86_64.tar.gz` (ARM: `arm64.tar.gz`) |
| Windows | `puresend_<sürüm>_windows_x86_64.zip` (ARM: `arm64.zip`) |
| macOS (M1 / M2 / M3 / M4) | `puresend_<sürüm>_macOS_arm64.tar.gz` |
| macOS (2020 öncesi, Intel) | `puresend_<sürüm>_macOS_x86_64.tar.gz` |

- **Ubuntu / Debian / Mint (`.deb`):** En pratik yöntem. İndirip çift tıklayarak ya da terminalden:
  ```bash
  sudo apt install ./puresend_<sürüm>_amd64.deb
  ```
  Program sisteminize kurulur, uygulama menüsüne logosuyla birlikte eklenir ve doğrudan menüden tıklanarak ya da terminalden `puresend` yazılarak açılabilir.
- **Windows:** `.zip` içindeki dosyaya çift tıkla, program açılır.
- **macOS:** `.tar.gz` içindeki dosyaya çift tıkla — Terminal penceresinde açılır.
- **Linux (Taşınabilir):** Arşivden çıkan dosyaya dosya yöneticisinden çift tıkladığında sistemindeki terminal (GNOME Terminal, Konsole, XFCE Terminal vb.) otomatik olarak açılır. Alternatif olarak terminalden çalıştırmak için:
  `chmod +x puresend && ./puresend`

### İlk açılışta bir uyarı çıkarsa

Program **imzalı olmadığı için** işletim sistemi ilk seferde soru sorabilir.
Bu bir hata değil ve programda bir sorun olduğu anlamına gelmiyor — imza
sertifikaları ücretli olduğu için alınmadı. Bir kez izin verdikten sonra
bir daha sorulmaz.

**macOS** — *"geliştirici doğrulanamadığı için açılamadı"* diyorsa dosyaya
**sağ tıkla → Aç**, çıkan pencerede tekrar **Aç**'a bas. (Sadece çift
tıklamak bu durumda işe yaramaz; sağ tık şart.) Alternatif olarak
Terminal'de:

```bash
xattr -d com.apple.quarantine puresend
```

**Windows** — *"Windows kişisel bilgisayarınızı korudu"* mavi ekranı
çıkarsa: **Ek bilgi** → **Yine de çalıştır**.

**Linux** — çalıştırma izni vermen gerekebilir:

```bash
chmod +x puresend
```

İndirdiğin dosyanın bozulmadığını doğrulamak istersen, her sürümün
yanındaki `checksums.txt` dosyasını indirdiğin arşivle aynı klasöre koy:

```bash
# Linux
sha256sum -c checksums.txt --ignore-missing

# macOS — sha256sum yok, aynı işi shasum yapar
grep macOS checksums.txt | shasum -a 256 -c

# Windows (PowerShell) — çıkan değeri checksums.txt'deki satırla karşılaştır
Get-FileHash .\puresend_*_windows_x86_64.zip -Algorithm SHA256
```

---

## Hangi klasör nerede çalışır?

Bu ayrımı karıştırmamak önemli:

| Klasör | Nerede çalışır | Nasıl dağıtılır |
|---|---|---|
| **`cmd/server/`** | 🖥️ **Ubuntu sunucunda**, 7/24 açık | Docker + Cloudflare Tunnel |
| **`cmd/client/`** | 💻 **Kullanıcının masaüstünde** | GitHub Releases (tek dosya veya `.deb`) |
| `internal/rendezvous/` | ikisinde de | ortak protokol (`/puresend/rendezvous/1.1.0`) |
| `internal/transfer/` | sadece istemci | dosya aktarımı, PAKE doğrulaması, resume |
| `internal/p2p/`, `internal/tui/` | sadece istemci | ağ katmanı + Bubble Tea terminal arayüzü |
| `internal/safetext/` | sadece istemci | ANSI kaçış dizileri ve bidi enjeksiyon koruması |
| `internal/headless/` | sadece istemci | terminal arayüzsüz CLI modu (`-send`, `-receive`) |
| **`LandingPage/`** | 🌐 **Web'de (Vercel/CDN)** | React + Vite + Tailwind v4 (tanıtım & TUI simülasyonu) |
| `packaging/` | Linux masaüstü | `.desktop` başlatıcı, SVG logo ve `.deb` paket dosyaları |
| `deploy/` | 🖥️ sunucu | cloudflared ayarları (compose kökte) |
| `scripts/` | derleme / paketleme | yerel `.deb` üretim betiği (`build-deb.sh`) |

**Kısaca:** sunucuda `cmd/server` çalışır ve dosyalara asla dokunmaz;
dosyalar kullanıcının çalıştırdığı `cmd/client`'tan çıkar.

---

## Kullanıcı ne yapıyor? (teknik olmayan biri için)

**Gönderen:**

1. Programı açar → *"Dosya göndereceğim"*
2. Dosya gezgininden dosyaları seçer → `Enter`
   (bulunduğu **klasörün tamamını** göndermek için `f`, son seçimi silmek
   için `x`, üst klasöre çıkmak için `Backspace`)
3. `s` ile başlatır, ekranda kod çıkar: **`kiraz-liman-42`**
4. Kodu arkadaşına WhatsApp'tan yazar → *"✓ Arkadaşıma ilettim"*
5. Arkadaşı kodu girince gönderme kendiliğinden başlar
6. Transfer bitince **kod otomatik geçersiz olur** — kullanıcının bir tuşa
   basmasını beklemez

**Alan:**

1. Programı açar → *"Bana dosya gönderilecek"*
2. Kodu yazar: `kiraz-liman-42` — büyük harf, Türkçe harf ya da tire yerine
   boşluk fark etmez (`KİRAZ liman 42` de olur). Kodun biçimi yanlışsa ya da
   kodlarda geçmeyen bir kelime varsa, bu daha sunucuya sorulmadan söylenir.
3. Gelen dosya listesini görür → *"Evet, indir"*
4. Dosyalar `İndirilenler/PureSend` klasörüne iner —
   başka bir yere inmesini isterse ana menüden **"İndirme klasörünü
   değiştir"** (klasör gezgininde `Backspace` veya `←` ile üst klasöre çıkılabilir),
   ya da `-out /mnt/disk`

Transfer sırasında **hız ve kalan süre** görünür. Bağlantı koparsa aynı
kodla tekrar denendiğinde **kaldığı yerden devam eder** — önceki denemede
bitmiş dosyalar yeniden indirilmez.

Kod ekranı açıkken gönderenin programı dosyaları arka planda okur (büyük
bir klasörde bu zaman alır); arkadaş kodu girdiğinde liste çoğu zaman hazır
olur. Buluşma noktasıyla bağlantı koparsa (tünel yeniden başladı, sunucu
yeniden deploy edildi) gönderen bunu ekranda görür, program kendiliğinden
yeniden bağlanır ve **aynı kod** çalışmaya devam eder.

Hiçbir aşamada IP adresi, port ya da ayar dosyası yok.

---

## Nasıl çalışıyor?

1. **Buluşma** — Gönderen rastgele bir oda kodunu kendi ağ adresleriyle
   birlikte sunucuya kaydeder. Alıcı aynı kodu girince adresleri alır.
2. **NAT delme** — İlk bağlantı sunucunun **Circuit Relay v2** köprüsü
   üzerinden kurulur; **DCUtR hole punching** bunu doğrudan bağlantıya
   yükseltir. Relay yalnızca aktif odası olan eşlere hizmet verir (ACL).
   Delme umutsuzsa DCUtR'nin kendi "pes ettim" sinyali dinlenir, sabit bir
   süre boyunca beklenmez.
3. **Kod doğrulaması (PAKE)** — İki uç, oda kodundan **SPAKE2 benzeri bir
   parola-doğrulamalı anahtar değişimi** ile ortak bir anahtar türetir ve
   birbirine HMAC ile kanıtlar. Kod tele hiç çıkmaz; yanlış kod dosya
   listesi görülmeden başarısız olur. Değişime **iki tarafın Peer ID'si de
   bağlanır**, böylece araya giren biri iki ucun mesajlarını birbirine
   taşıyamaz.
4. **Transfer** — Alıcı gelen listeyi görüp **onaylar**; dosyalar doğrudan
   bağlantı üzerinden, dosya başına **SHA-256 doğrulamasıyla** akar.
   Delme başarısız olursa transfer relay üzerinden yedeklenir.
5. **Kapanış** — Transfer bitince gönderen protokol handler'ını kaldırır ve
   odayı sunucudan düşürür: kod tek kullanımlıktır.

### Sunucu neden güvenilir taraf değil?

Alıcıya "gönderen şu Peer ID" diyen sunucudur. Kendi eşlerinden birini
göstermeye kalkarsa, o eş oda kodunu bilmediği için 3. adımdaki
doğrulamayı geçemez ve alıcı hiçbir dosya görmez. Sunucunun gördüğü tek
şey **kimin ne zaman buluştuğudur**.

### Kod tahmin edilebilir mi?

Kodlar 256 kelimelik Türkçe bir listeden iki kelime ve 10–99 arası bir
sayıdır: yaklaşık **5,9 milyon** olasılık (~22,5 bit). Kod aynı zamanda
sunucunun odayı aradığı anahtar olduğu için, tahminlere karşı asıl savunma
sunucunun ne kadar hızlı cevap verdiğidir:

- Bir kimlik dakikada 5 yanlış denemeden sonra hiçbir cevap alamaz.
- Sunucu genelinde yanlış denemeler dakikada 200'ü geçerse, daha önce bir
  kez yanılmış her kimlik susturulur. Kodu **ilk seferde doğru yazan**
  kişi ise hiçbir zaman reddedilmez — saldırgan limiti doldurarak gerçek
  kullanıcıları dışarıda bırakamaz.
- Kimlik bedava olduğu için (her açılışta yenisi), saldırganı gerçekten
  yavaşlatan şey bağlantı başına maliyettir. Sunucu tünel arkasında
  gerçek IP'yi göremez; **IP başına sınırı Cloudflare koyar** — aşağıdaki
  2. adıma bak.

### Cloudflare Tunnel ile neden WebSocket?

`cloudflared` internete **yalnızca HTTP/WebSocket** açar. Ham TCP portu
(`tcp://`) için karşı tarafın da `cloudflared access` çalıştırması gerekir;
UDP/QUIC ise hiç açılamaz. Bu yüzden sunucu libp2p'yi **WebSocket
transport** üzerinden konuşur — normal bir HTTP ingress'ten sorunsuz geçer.

```
İstemci ──wss://...:443──► Cloudflare ──ws://localhost:8080──► sunucu
                            (TLS burada biter)
```

Tünel yalnızca **buluşma** ve **delme koordinasyonu** için kullanılır
(birkaç KB). Delik açıldıktan sonra dosyalar Cloudflare'i hiç görmez.

---

## Sunucu kurulumu (Ubuntu + Docker Compose + Cloudflare Tunnel)

### 1. Sunucuyu başlat

```bash
PUBLIC_HOST=rendezvous.madebybaki.com \
  docker compose up -d
```

Loglardan **Peer ID**'yi al — birazdan lazım:

```bash
docker compose logs rendezvous
```

```
  Peer ID: 12D3KooWKKqpYTw3D8arNmcNG7ZK1mPfSH2cQ7ohZqHBmYN6eEAn

Client address (bake this into the client build):
  /dns4/rendezvous.madebybaki.com/tcp/443/tls/ws/p2p/12D3KooW...
```

> ⚠️ `rendezvous-key` volume'ünü **silme**. Peer ID değişirse daha önce
> dağıttığın bütün istemciler çalışmaz hale gelir.

Sağlık kontrolü ve metrikler:

```bash
curl localhost:8081/health
# {"status":"ok","version":"v0.2.0","peer_id":"12D3KooW...","active_rooms":0}

curl -s localhost:8081/metrics | grep -E '^(puresend|libp2p_relaysvc)_'
```

`8081` portu **asla internete açılmamalı**. Sunucu ikilisi bu uç noktayı
varsayılan olarak yalnızca `127.0.0.1:8081`'de dinler (`-health-addr`);
Docker imajı konteyner içinde tüm arayüzleri açar ve compose dosyası onu
host'ta yine `127.0.0.1`'e bağlar.

Prometheus tarafında en çok işine yarayacak sorgular:

```promql
# Kaç transfer doğrudan yol açamayıp relay'e düşüyor? Delik açma
# koordinasyonu relay'den birkaç KB geçirir; bunun MB'larla ölçülen
# kısmı yedek yoldan giden transferlerdir.
rate(libp2p_relaysvc_data_transferred_bytes_total[1h])
rate(libp2p_relaysvc_connections_total{type="opened"}[1h])

puresend_active_rooms
rate(puresend_rooms_expired_total[1h])     # yarıda bırakılan gönderimler
rate(puresend_rooms_abandoned_total[1h])   # sahibi kopup dönmeyen odalar
rate(puresend_lookups_throttled_total[5m]) # tahmin denemeleri
libp2p_rcmgr_blocked_resources                     # bağlantı sınırlarına takılanlar
```

`-relay-data` sınırını doğru ayarlamak için lazım olan sayı, ilk
sorgunun gösterdiği relay trafiğidir.

**Tünel arkasındaki bağlantı sınırları.** libp2p varsayılan olarak bir IP
adresine aynı anda 8 bağlantı, 8 relay rezervasyonu ve dakikada birkaç yeni
bağlantı tanır. Tünel arkasında bütün kullanıcılar aynı adresten
(cloudflared'in ya da Docker köprüsünün adresinden) geldiği için bu,
dünyadaki herkesin bu 8'i paylaşması demekti. Sunucu, `-trusted-proxies`
ağlarından (varsayılan: loopback ve özel ağlar) gelen bağlantılara adres
başına sınır uygulamaz; relay rezervasyon sınırları da oda tablosuna
(1000) eşitlenmiştir. İnternete doğrudan açık adresler libp2p'nin
varsayılanlarıyla sınırlı kalır.

### 2. Cloudflare Tunnel

**Makinede zaten bir tünel varsa** (başka bir proje için), config'i üzerine
yazma — tek bir `cloudflared` bütün hostname'lere aynı dosyadan hizmet
eder, üzerine yazarsan diğer projenin kuralı silinir ve o proje düşer.
Mevcut `ingress:` listesine kural **ekle**:

```bash
sudo cat /etc/cloudflared/config.yml          # önce ne olduğuna bak
cloudflared tunnel list

cloudflared tunnel route dns <mevcut-tünel> rendezvous.madebybaki.com

# config.yml'deki ingress listesine, catch-all 404'ten ÖNCE:
#   - hostname: rendezvous.madebybaki.com
#     service: http://localhost:8080
#     originRequest:
#       connectTimeout: 30s

cloudflared tunnel ingress validate
sudo systemctl restart cloudflared
```

Kurallar yukarıdan aşağı eşleşir ve `http_status:404` her şeyi yakalar —
yeni kural ondan sonra kalırsa hiç çalışmaz.

**IP başına hız sınırı (önerilir).** Sunucu tünelin arkasında gerçek IP'yi
göremez; göremediği şeyi de sınırlayamaz. Cloudflare görebilir, ve ücretsiz
plan bile bir rate limiting kuralına izin verir. *Security → Security rules
→ Create rule → Rate limiting rules*:

| Alan | Değer |
|---|---|
| When incoming requests match | `http.host eq "rendezvous.madebybaki.com"` |
| With the same characteristics | IP |
| When rate exceeds | 20 istek / 10 saniye |
| Then take action | Block, 10 saniye |

Her istemci bağlantısı tek bir WebSocket isteğidir; normal bir kullanıcı
bu sınıra yaklaşmaz. Kod tahmin etmeye çalışan biri ise IP başına saniyede
iki bağlantıya — dolayısıyla en fazla iki tahmine — iner.

**Makinede hiç tünel yoksa** dosya olduğu gibi kullanılabilir:

```bash
cloudflared tunnel login
cloudflared tunnel create puresend
cloudflared tunnel route dns puresend rendezvous.madebybaki.com
sudo cp deploy/cloudflared-config.yml /etc/cloudflared/config.yml
sudo cloudflared service install
```

Ayrıntılar ve gerekçeler: [deploy/cloudflared-config.yml](deploy/cloudflared-config.yml)

### 3. Dışarıdan eriştiğini doğrula

Sunucunun **dışındaki** bir ağdan (telefon hotspot'u iyi bir test):

```bash
curl -sI https://rendezvous.madebybaki.com \
     -H "Connection: Upgrade" -H "Upgrade: websocket"
```

### Konteyner Güvenliği ve Kimlik Yedeği

Docker Compose dosyası (`docker-compose.yml`) sunucuyu en sıkı güvenlik sınırlarıyla çalıştıracak şekilde yapılandırılmıştır:
- **Salt-okunur dosya sistemi (`read_only: true`):** Konteyner yalnızca `/data` (anahtar) ve `/tmp` alanına yazabilir.
- **Tüm yetkiler düşürülmüştür (`cap_drop: ALL`, `no-new-privileges: true`):** Konteyner içinde yetki yükseltilemez.
- **Kaynak sınırları (`mem_limit: 512m`, `pids_limit: 256`):** Olası bellek ve süreç tükenmesi saldırılarına karşı host'u korur.
- **Port İzolasyonu:** `8080` (WebSocket) ve `8081` (Health/Metrics) yalnızca `127.0.0.1` üzerinden dinlenir; internete yalnızca tünel üzerinden kontrollü açılır.

> 💡 **Kimlik Anahtarını Yedekleme:**  
> Sunucunun Peer ID'si `/data/server.key` dosyasında kalıcı `rendezvous-key` volume'ünde saklanır. Sunucuyu farklı bir makineye taşımak veya yedeklemek istersen:
> ```bash
> docker compose exec rendezvous base64 -w0 /data/server.key
> ```
> Bu çıktıyı güvenli bir yerde (örneğin parola yöneticisinde) saklayabilirsin. İhtiyaç halinde başka bir sunucuda `FT_IDENTITY_KEY` ortam değişkeniyle vererek aynı Peer ID'yi diske ihtiyaç duymadan ayağa kaldırabilirsin.

---

## İstemciyi yayınlama

Sunucu adresi **derleme sırasında** gömülür; kullanıcı hiçbir ayar yapmaz.
Adresin sonundaki Peer ID sunucunun kimlik anahtarından gelir: anahtar
kaybolursa bugüne kadar indirilmiş **her istemci** ölür. İki sigorta var:

1. **Anahtarın yedeği.** `base64 -w0 /data/server.key` çıktısını sunucunun
   dışında (parola yöneticisi gibi) sakla. `FT_IDENTITY_KEY` ile aynı
   kimliği her yerde yeniden kurabilirsin.
2. **Sunucu listesi.** Yayınlanan istemciler, gömülü adreslerin hiçbiri
   cevap vermezse landing page'in altındaki
   [`server.txt`](LandingPage/public/server.txt)'yi okur
   (`https://puresend.madebybaki.com/server.txt`) ve oradaki
   adresleri dener. Sunucu taşınırsa ya da kimliği değişirse bu dosyaya
   yeni adresi yazman, eski sürümleri kurtarır. **Sunucuyu deploy ettikten
   sonra adresini bu dosyaya ekle.** Liste güvenilir bir kaynak sayılmaz:
   onu değiştiren biri istemcileri kendi sunucusuna yönlendirebilir, ama
   oda kodu doğrulaması yüzünden transferleri okuyamaz.

GitHub'da → *Settings → Secrets and variables → Actions → Variables* →
`FT_SERVER` değişkenini oluştur:

```
/dns4/rendezvous.madebybaki.com/tcp/443/tls/ws/p2p/12D3KooW...
```

Liste adresini değiştirmek istersen `FT_SERVER_LIST` değişkenini de
tanımlayabilirsin (tanımlanmazsa yukarıdaki landing page adresi gömülür).

Sonra tag at:

```bash
git tag v0.1.0 && git push --tags
```

Release workflow'u yayınlamadan önce `go vet` ve bütün testleri koşar,
SBOM için `syft`'i kendisi kurar.

[GoReleaser](.goreleaser.yaml) 6 ikili üretir (Linux / macOS / Windows ×
x86_64 / arm64) ve Releases sayfasına koyar.

Elle derlemek istersen:

```bash
go build -ldflags "-X main.defaultServer=/dns4/.../p2p/12D3KooW..." ./cmd/client
```

---

## Geliştirme

```bash
make            # hedeflerin listesi
make test       # tüm testler, race detector açık
make lint       # golangci-lint (CI ile aynı sürüm)
make vuln       # govulncheck — kodun gerçekten eriştiği açıklar
make cover      # kapsama raporu
make test-relay # delik açılamayan iki ağ arasında relay yedeği (aşağıda)
make deb        # Debian/Ubuntu için .deb paketi üretir
```

Tek makinede denemek için üç terminal:

```bash
# 1) sunucu
go run ./cmd/server -ws-port 8080

# 2) gönderen  ve  3) alan  (Peer ID'yi sunucunun çıktısından al)
go run ./cmd/client -server /ip4/127.0.0.1/tcp/8080/ws/p2p/<PeerID>
```

### Başsız (headless) mod

Arayüz istemeyen her şey için — scriptler, terminali olmayan sunucular ve
CI. Oda kodu **stdout'a tek satır** olarak basılır, geri kalan her şey
stderr'e gider:

```bash
puresend -send tatil/            # kodu basar, alıcıyı bekler
puresend -receive kiraz-liman-42 -out /mnt/disk -yes
puresend -version
```

### Relay yedeğini test etmek

Diğer bütün testler loopback üzerinde koşar, yani doğrudan yol **her zaman**
açılır — relay yedeği hiç denenmez. `make test-relay` ayrıcalıksız kullanıcı
ve ağ isim uzaylarıyla üç ağ kurar ve köprüde port yalıtımını açar: iki
istemci sunucuyu görür ama birbirini **hiç** göremez, dolayısıyla delik
açmak matematiksel olarak imkânsızdır ve transfer relay üzerinden gitmek
zorunda kalır.

```bash
make test-relay
#   A -> server (10.0.0.1)       REACHABLE
#   B -> server (10.0.0.1)       REACHABLE
#   A -> B      (10.0.0.3)       BLOCKED
#   B -> A      (10.0.0.2)       BLOCKED
#   ...
#   relay fallback works end to end
```

Root gerekmez; CI'da her commit'te koşar. İkililer isim uzaylarına
girmeden önce derlenir (içeride ağ yok), ve test relay'in kendi
metriklerinin transferi gördüğünü de doğrular.

`make lint` ve `make vuln` araçları `go run` ile, makinedeki Go ile
derleyerek çalıştırır: eski bir Go ile derlenmiş `golangci-lint` ya da
`govulncheck` yeni standart kütüphanede anlamsız hatalar verir.

## Teknolojiler ve Mekanizmalar

- **Go 1.26+**, **go-libp2p v0.49** — TCP + QUIC + WebSocket transport,
  Circuit Relay v2 (ACL ve kota korumalı), DCUtR hole punching (NAT delme),
  AutoNAT v2, UPnP port açma, Noise/TLS şifreleme ve Resource Manager
- **Bubble Tea + Lipgloss + Bubbles** — Terminal kullanıcı arayüzü (TUI), dinamik hız ve kalan süre hesaplama
- **schollz/pake/v3** — Oda kodundan anahtar türetme (P-256 üzerinde SPAKE2), HMAC kanıtı ve Peer ID bağlama (MITM ve sahte sunucu koruması)
- **Prometheus client_golang v1.24.1** — Sunucu `/metrics` (relay trafiği, aktif odalar, limitler) ve `/health` JSON uç noktası
- **Landing Page** — React 19, Vite, Tailwind CSS v4, TypeScript, Oxlint; tarayıcıda çalışan canlı TUI simülatörü ve Release API entegrasyonu
- **Paketleme & Sistem Entegrasyonu** — GoReleaser v2, Syft (SBOM), Debian/Ubuntu `.deb` üretimi (`dpkg-deb` / `nfpm`), FreeDesktop Desktop Entry (`.desktop`) ve SVG uygulama ikonu
- **Güvenlik & Dezenfeksiyon (`internal/safetext`)** — Terminal enjeksiyonlarına karşı ANSI kaçış kodları ve Unicode bidi (RTLO) temizliği, Windows aygıt adı ve dizin dışına taşma (Path Traversal) engelleme
- **Kalıcı Aktarım (Resume)** — SHA-256 doğrulamalı parça (`.part`) dosyaları ile kesilen transferleri kaldığı yerden sürdürme
- **İki özel protokol**: `/puresend/rendezvous/1.1.0` ve
  `/puresend/transfer/2.0.0`

## Sınırlamalar

- **Relay yedeğinde bağlantı başına 256 MB sınırı var** (`-relay-data`).
  Delik açılamayan bir çiftte daha büyük bir transfer yarıda kesilir —
  onay ekranında bu baştan söylenir ve kesilirse kaldığı yerden devam eder.
- **Boş klasörler taşınmaz**: protokol dosya taşır, klasör yapısı dosyaların
  göreli yollarından yeniden kurulur.
- **Sembolik bağlar izlenmez.** Klasör dışına çıkan bir bağ, kullanıcının
  göndermeyi kabul ettiği şeyi sessizce genişletirdi.
- **Tek seferde en fazla 5000 dosya.**
- **Windows'ta saklanamayan adlar reddedilir**: `? * < > | "` içeren ya da
  `CON`, `NUL`, `AUX` gibi aygıt adı olan bir dosya Windows'a gönderilemez;
  alıcı bunu transferden önce söyler. Kontrol karakteri içeren adlar her
  sistemde reddedilir.
- **Sunucu meta veriyi görür**: kimin kiminle ne zaman buluştuğunu bilir.
  Dosyaları ve — PAKE sayesinde — oda kodunu göremez.
- **İkili dosyalar imzalı değil**; işletim sistemi ilk açılışta uyarabilir.
