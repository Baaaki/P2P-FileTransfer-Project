# PureSend — Ürünleşme Yol Haritası

Bu dosya, projeyi "çalışan bir prototip"ten "sıradan bir insanın indirip
kullanabileceği bir ürün"e dönüştürmek için yapılacakların tamamıdır.
Hedef altyapı: **Ubuntu Server + Cloudflare Tunnel + OpenShip**.

> **Durum:** Faz A–E'nin **kod ve yapılandırma tarafı tamamlandı.**
> Geriye kalan tek iş [bölüm 3](#3-deploy-sırası-bu-sırayla-yapılacak)'teki
> deploy adımları — onlar sunucuya, Cloudflare hesabına ve DNS'e erişim
> istediği için elle yapılacak.

---

## 0. Altyapı gerçeği: Cloudflare Tunnel neyi taşır, neyi taşımaz?

Bu bölüm tüm mimariyi belirlediği için en başta.

`cloudflared` bir hostname'i internete açarken **yalnızca HTTP/HTTPS ve
WebSocket** trafiğini herkese açık şekilde proxy'ler.

| İhtiyaç | Cloudflare Tunnel ile mümkün mü? |
|---|---|
| `service: http://localhost:8080` + WebSocket upgrade | ✅ Evet, ücretsiz planda |
| Ham TCP portu (`tcp://localhost:4001`) herkese açık | ❌ Hayır — karşı taraf da `cloudflared access tcp` çalıştırmalı |
| UDP / QUIC | ❌ Hayır (yalnızca WARP-to-Tunnel ile, herkese açık değil) |
| Ham TCP, gerçekten public | ⚠️ Sadece **Cloudflare Spectrum** (Enterprise fiyatlı) |

**Sonuç:** Sunucunun bugünkü `/ip4/0.0.0.0/tcp/4001` + `/udp/4001/quic-v1`
dinleyicileri tünelin arkasından **çalışamaz**.

### Çözüm: libp2p WebSocket transport

libp2p'nin WebSocket transport'u tam olarak bu senaryo için var ve
go-libp2p'de **varsayılan olarak açık** (`defaults.go` → `Transport(ws.New)`).

```
İstemci                    Cloudflare Edge            Ubuntu Server
   │                             │                          │
   │  wss://p2p-filetransfer...  │                          │
   │  /tcp/443/tls/ws  ─────────►│  TLS burada biter        │
   │                             │                          │
   │                             │  ws:// (düz) ───────────►│ cloudflared
   │                             │                          │      │
   │                             │                          │      ▼
   │                             │                    localhost:8080
   │                             │                    (libp2p /ws listener)
```

İstemcinin kullanacağı multiaddr:

```
/dns4/rendezvous.madebybaki.com/tcp/443/tls/ws/p2p/<PeerID>
```

Sunucunun dinlediği multiaddr:

```
/ip4/0.0.0.0/tcp/8080/ws          ← TLS yok; onu Cloudflare hallediyor
```

### Peki NAT delme (hole punching) bundan etkilenir mi?

**Hayır.** Kritik nokta bu:

- Cloudflare Tunnel yalnızca **buluşma (rendezvous)** ve **DCUtR
  koordinasyonu** için kullanılır — ikisi de birkaç kilobayt.
- Delik açıldıktan sonra dosyalar **doğrudan iki bilgisayar arasında**,
  kendi TCP/QUIC dinleyicileri üzerinden akar. Cloudflare bu trafiği
  hiç görmez.

### Relay fallback: dikkat edilecek yer

Delme başarısız olursa (simetrik NAT, CGNAT) trafik relay'e düşer — yani
**Cloudflare üzerinden**. Bu iki sebeple sınırlanmalı:

1. Cloudflare self-serve ToS, proxy'nin büyük dosya transferi için
   kullanılmasını kısıtlar.
2. Trafik senin ev internetinin upload'ından gider.

Bu yüzden `relay.WithInfiniteLimits()` **kaldırılacak**, yerine
yapılandırılabilir bir veri sınırı gelecek (varsayılan: 256 MB / 10 dk).
DCUtR koordinasyonu için gereken birkaç KB bu sınırın çok altında, yani
delme yeteneği hiç etkilenmez.

> **İleride:** Relay'i ciddi kullanmak istersen 5 $/ay'lık bir VPS'e
> yalnızca relay rolüyle ikinci bir sunucu koy, rendezvous evde kalsın.

---

## 1. Klasör → nerede çalışır haritası

| Klasör | Nerede | Nasıl |
|---|---|---|
| **`cmd/server/`** | 🖥️ Ubuntu Server | OpenShip → Docker container, Cloudflare Tunnel arkasında, 7/24 açık |
| **`cmd/client/`** | 💻 Kullanıcının masaüstü | GoReleaser ile 6 platforma derlenir, GitHub Releases'ten indirilir |
| `internal/rendezvous/` | her ikisi | Ortak protokol kodu (sunucu handler + istemci çağrıları) |
| `internal/transfer/` | sadece istemci | Sunucu bu kodu hiç çalıştırmaz |
| `internal/p2p/` *(yeni)* | sadece istemci | libp2p node yönetimi, TUI'den bağımsız |
| `internal/tui/` *(yeni)* | sadece istemci | Bubble Tea arayüzü |
| `deploy/` *(yeni)* | 🖥️ Ubuntu Server | cloudflared config (compose kökte) |

**Tek cümlede:** Sunucuda `cmd/server` çalışır ve dosyalara asla dokunmaz;
kullanıcı `cmd/client`'ı indirir ve dosyalar onun bilgisayarından çıkar.

---

## 2. Yapılacaklar

### Faz A — Sunucuyu tünele uygun hale getir ✅

- [x] `cmd/server/main.go`: `-ws-port` ile `/ip4/0.0.0.0/tcp/<port>/ws`
      dinleyicisi ekle (Cloudflare Tunnel hedefi).
- [x] Ham TCP/QUIC dinleyicilerini `-port` ile **opsiyonel** yap (0 =
      kapalı). Port yönlendirme yapabilenler için dursun.
- [x] `-announce` bayrağı: sunucunun public multiaddr'ını (`/dns4/.../
      tcp/443/tls/ws`) `AddrsFactory` ile ilan et. Relay circuit adresleri
      buna dayanacağı için şart.
- [x] `relay.WithInfiniteLimits()` → `relay.WithResources(...)`,
      `-relay-data` ve `-relay-duration` bayraklarıyla.
- [x] `libp2p.EnableAutoNATv2()` ekle — istemciler ulaşılabilirliklerini
      ölçebilsin (şu an sunucu bu servisi vermiyor).
- [x] `-health-addr` üzerinde küçük bir HTTP `/health` endpoint'i —
      OpenShip health check için.
- [x] Peer ID'yi açılışta net biçimde logla (deploy sonrası lazım olacak).

Ek olarak: tünel arkasında ulaşılabilirlik ölçülemediği için
`ForceReachabilityPublic()`, relay'in kötüye kullanımını engellemek için
de oda kaydını ACL olarak kullanan `relay.WithACL(registry)` eklendi.

### Faz B — İstemciyi TUI'den sürülebilir hale getir ✅

- [x] `internal/p2p/` paketi: `Node` tipi — host kurulumu, sunucuya
      bağlanma, oda kaydı, oda sorgulama, bağlantı türü izleme.
      **Hiçbir `fmt.Println` içermeyecek** — TUI kanal üzerinden olay alacak.
- [x] `cmd/client/main.go` sadece bayrakları okuyup TUI'yi başlatsın.
- [x] `defaultServer` değişkeni + `-ldflags -X` ile gömme.

### Faz C — Adım adım yönlendiren TUI ✅

Tasarım ilkesi: **Kullanıcı hiçbir teknik terim görmeyecek.** "multiaddr",
"peer", "NAT", "relay" kelimeleri arayüzde geçmeyecek. Her ekranda tek bir
soru ve ne yapılacağının düz Türkçe açıklaması olacak.

Gönderme akışı:

```
1. Karşılama      → "Ne yapmak istiyorsun?"  [Göndereceğim / Bana gönderiliyor]
2. Bağlanılıyor   → spinner, "Buluşma noktasına bağlanılıyor..."
3. Dosya seç      → dosya gezgini, seçilenler listesi
4. ODA KODU       → büyük puntoyla kod + "Arkadaşına bu kodu ilet
                     (WhatsApp, SMS, telefon — fark etmez)"
                     [✓ Arkadaşıma ilettim]        ← kullanıcının istediği buton
5. Bekleniyor     → "Arkadaşının kodu girmesi bekleniyor..."
6. Bağlandı       → "Bağlantı kuruldu ✓ (doğrudan / yedek yol)"
7. Aktarılıyor    → ilerleme çubuğu
8. Bitti          → "✓ Gönderildi!"
```

Alma akışı:

```
1. Karşılama
2. Bağlanılıyor
3. Kod gir       → "Arkadaşın sana 3 kelimelik bir kod verdi. Buraya yaz:"
4. Aranıyor      → "Arkadaşın bulunuyor..." → "Bağlantı kuruluyor..."
5. ONAY          → "Sana şu dosyalar gönderilmek isteniyor: ... Kabul?"
6. İndiriliyor   → ilerleme çubuğu
7. Bitti         → "✓ İndi! Dosyalar şurada: /home/.../Downloads/..."
```

- [x] Her ekranda alt bilgi çubuğu: hangi tuşlar çalışıyor.
- [x] Hata ekranları da düz Türkçe: "Kod bulunamadı. Ya yanlış yazıldı ya
      da süresi doldu (kodlar 1 saat geçerli)."
- [x] Ctrl+C her yerde temiz çıkış.

Her iki akış da `internal/tui/` içinde; `tui_test.go` tüm ekranların
çizildiğini ve hata metinlerinin teknik terim sızdırmadığını doğruluyor.

### Faz D — Dağıtım ✅

- [x] `.goreleaser.yaml`: linux/darwin/windows × amd64/arm64 = 6 ikili,
      `CGO_ENABLED=0`, ldflags ile sunucu adresi gömülü, checksum.
- [x] `.github/workflows/release.yml`: `v*` tag'inde tetiklenir.
      (`FT_SERVER` ayarlanmamışsa release'i baştan durduruyor — yoksa
      ölü ikili yayınlanırdı.)
- [x] README'de "İndir ve çalıştır" bölümü + macOS Gatekeeper notu.
      Windows SmartScreen ve Linux `chmod +x` notları da eklendi.

### Faz E — Deploy ✅ *(kod tarafı)*

- [x] `deploy/cloudflared-config.yml` örneği.
- [x] `docker-compose.yml` — repo **kökünde**. OpenShip build context'ini
      repoya sabitleyip compose'un `context:` alanını yok saydığı için,
      alt dizindeki bir compose repo dışına taşan bir build yolu üretir
      ve proje oluşturma 400 ile reddedilir.
- [x] `Dockerfile` güncelle: ws portu + health portu expose.
- [ ] DNS: `rendezvous.madebybaki.com` → tünel CNAME.
      *Elle yapılacak:* `cloudflared tunnel route dns` komutu bunu
      oluşturuyor, bölüm 3'e bak.

---

## 3. Deploy sırası (bu sırayla yapılacak)

Kalan tek iş bu. Sunucuya, Cloudflare hesabına ve DNS'e erişim
gerektirdiği için elle yapılacak.

**1. Sunucuyu Ubuntu'ya kur, `/health` yeşil olsun.**

```bash
PUBLIC_HOST=rendezvous.madebybaki.com \
  docker compose up -d

curl localhost:8081/health
# {"status":"ok","peer_id":"12D3KooW...","active_rooms":0}
```

**2. Loglardan Peer ID'yi al** — 5. ve 6. adımlarda lazım, bir yere not et.

```bash
docker compose logs rendezvous | grep "Peer ID"
```

> ⚠️ `rendezvous-key` volume'ü kalıcı olmalı. Peer ID değişirse
> dağıttığın bütün istemciler çalışmaz hale gelir.

**3. `cloudflared` ingress'ini ayarla** (DNS kaydını da bu oluşturur).

Makinede **zaten bir tünel varsa** config'i üzerine yazma — tek bir
`cloudflared` bütün hostname'lere aynı dosyadan hizmet eder, üzerine
yazmak diğer projeleri düşürür. Mevcut `ingress:` listesine, catch-all
404'ten **önce** kural ekle:

```bash
sudo cat /etc/cloudflared/config.yml
cloudflared tunnel route dns <mevcut-tünel> rendezvous.madebybaki.com
#   - hostname: rendezvous.madebybaki.com
#     service: http://localhost:8080
cloudflared tunnel ingress validate
sudo systemctl restart cloudflared
```

Hiç tünel yoksa:

```bash
cloudflared tunnel login
cloudflared tunnel create puresend
cloudflared tunnel route dns puresend rendezvous.madebybaki.com
sudo cp deploy/cloudflared-config.yml /etc/cloudflared/config.yml
sudo cloudflared service install
```

Ardından Cloudflare'de hostname için **IP başına rate limiting kuralı**
kur (Security → Security rules → Rate limiting rules; IP, 20 istek / 10
saniye, Block). Sunucu tünel arkasında gerçek IP'leri göremez; kod
tahminini IP başına sınırlayan tek yer burası. Ayrıntı README'de.

**4. Dışarıdan erişimi doğrula** — sunucunun ağının *dışından*
(telefon hotspot'u iyi bir test). `101 Switching Protocols` beklenir:

```bash
curl -sI https://rendezvous.madebybaki.com \
     -H "Connection: Upgrade" -H "Upgrade: websocket"
```

**5. Adresi sunucu listesine ekle.** `LandingPage/public/server.txt`
dosyasına sunucunun açılışta bastığı istemci adresini yaz ve landing
page'i yayınla. Yayınlanan istemciler gömülü adres cevap vermezse bu
dosyaya bakar; Peer ID bir gün değişirse eski sürümleri bu kurtarır.
Anahtarın base64 yedeğini de (`base64 -w0 /data/server.key`) sunucunun
dışında bir yerde sakla.

**6. Peer ID'yi gömüp sürüm çıkar.** GitHub'da
*Settings → Secrets and variables → Actions → Variables* altına
`FT_SERVER` ekle:

```
/dns4/rendezvous.madebybaki.com/tcp/443/tls/ws/p2p/<PeerID>
```

Sonra tag at — release workflow'u 6 ikiliyi üretip yayınlar:

```bash
git tag v0.1.0 && git push --tags
```

> Değişken ayarlanmamışsa workflow bilerek durur; adressiz bir ikili
> indiren herkes için ölü doğmuş olurdu.

**7. Gerçek transfer denemesi** — iki *farklı ağdaki* iki bilgisayarda
ikiliyi indir ve bir dosya gönder. Aktarım ekranında
"✓ Doğrudan bağlantı kuruldu" yazmalı; "yedek yol" yazıyorsa delme
başarısız olmuş demektir (relay 256 MB ile sınırlı).

---

## 4. Kapsam dışı (v1 sonrası)

Aşağıdakiler **7 Ağustos 2026 test raporundan sonra yapıldı** ve artık
kapsam içinde:

- ✅ Klasör gönderme
- ✅ Yarım kalan transferi devam ettirme (resume)
- ✅ PAKE — kod artık bir parola; sunucu güvenilir taraf değil
- ✅ Başsız (headless) mod, `-version`, indirme klasörü seçimi
- ✅ Sunucuda Prometheus metrikleri, çoklu buluşma sunucusu
- ✅ Delik açılamayan iki ağ arasında relay yedeğini CI'da doğrulayan test

Hâlâ kapsam dışı:

- QR kod
- Mobil istemci
- Kod imzalama sertifikası (ücretli)
