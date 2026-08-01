# FileTransferilla — Ürünleşme Yol Haritası

Bu dosya, projeyi "çalışan bir prototip"ten "sıradan bir insanın indirip
kullanabileceği bir ürün"e dönüştürmek için yapılacakların tamamıdır.
Hedef altyapı: **Ubuntu Server + Cloudflare Tunnel + OpenShip**.

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
/dns4/p2p-filetransfer.madebybaki.com/tcp/443/tls/ws/p2p/<PeerID>
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
| `deploy/` *(yeni)* | 🖥️ Ubuntu Server | cloudflared config, compose dosyası |

**Tek cümlede:** Sunucuda `cmd/server` çalışır ve dosyalara asla dokunmaz;
kullanıcı `cmd/client`'ı indirir ve dosyalar onun bilgisayarından çıkar.

---

## 2. Yapılacaklar

### Faz A — Sunucuyu tünele uygun hale getir

- [ ] `cmd/server/main.go`: `-ws-port` ile `/ip4/0.0.0.0/tcp/<port>/ws`
      dinleyicisi ekle (Cloudflare Tunnel hedefi).
- [ ] Ham TCP/QUIC dinleyicilerini `-port` ile **opsiyonel** yap (0 =
      kapalı). Port yönlendirme yapabilenler için dursun.
- [ ] `-announce` bayrağı: sunucunun public multiaddr'ını (`/dns4/.../
      tcp/443/tls/ws`) `AddrsFactory` ile ilan et. Relay circuit adresleri
      buna dayanacağı için şart.
- [ ] `relay.WithInfiniteLimits()` → `relay.WithResources(...)`,
      `-relay-data` ve `-relay-duration` bayraklarıyla.
- [ ] `libp2p.EnableAutoNATv2()` ekle — istemciler ulaşılabilirliklerini
      ölçebilsin (şu an sunucu bu servisi vermiyor).
- [ ] `-health-port` üzerinde küçük bir HTTP `/health` endpoint'i —
      OpenShip health check için.
- [ ] Peer ID'yi açılışta net biçimde logla (deploy sonrası lazım olacak).

### Faz B — İstemciyi TUI'den sürülebilir hale getir

- [ ] `internal/p2p/` paketi: `Node` tipi — host kurulumu, sunucuya
      bağlanma, oda kaydı, oda sorgulama, bağlantı türü izleme.
      **Hiçbir `fmt.Println` içermeyecek** — TUI kanal üzerinden olay alacak.
- [ ] `cmd/client/main.go` sadece bayrakları okuyup TUI'yi başlatsın.
- [ ] `defaultServer` değişkeni + `-ldflags -X` ile gömme.

### Faz C — Adım adım yönlendiren TUI

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

- [ ] Her ekranda alt bilgi çubuğu: hangi tuşlar çalışıyor.
- [ ] Hata ekranları da düz Türkçe: "Kod bulunamadı. Ya yanlış yazıldı ya
      da süresi doldu (kodlar 1 saat geçerli)."
- [ ] Ctrl+C her yerde temiz çıkış.

### Faz D — Dağıtım

- [ ] `.goreleaser.yaml`: linux/darwin/windows × amd64/arm64 = 6 ikili,
      `CGO_ENABLED=0`, ldflags ile sunucu adresi gömülü, checksum.
- [ ] `.github/workflows/release.yml`: `v*` tag'inde tetiklenir.
- [ ] README'de "İndir ve çift tıkla" bölümü + macOS Gatekeeper notu.

### Faz E — Deploy

- [ ] `deploy/cloudflared-config.yml` örneği.
- [ ] `deploy/docker-compose.yml` (OpenShip'e verilecek).
- [ ] `Dockerfile` güncelle: ws portu + health portu expose.
- [ ] DNS: `p2p-filetransfer.madebybaki.com` → tünel CNAME.

---

## 3. Deploy sırası (bu sırayla yapılacak)

1. Sunucuyu OpenShip ile Ubuntu'ya kur, `/health` yeşil olsun.
2. `cloudflared` ingress'i `p2p-filetransfer.madebybaki.com` →
   `http://localhost:8080` olarak ayarla.
3. Loglardan **Peer ID**'yi al.
4. `wss://p2p-filetransfer.madebybaki.com` dışarıdan erişilebiliyor mu
   test et (telefon hotspot'undan).
5. Peer ID'yi GoReleaser ldflags'ine gömüp `v0.1.0` tag'i at.
6. İki farklı ağdaki iki bilgisayarda indirip gerçek transfer dene.

---

## 4. Kapsam dışı (v1 sonrası)

- Klasör gönderme (şu an sadece dosya)
- Yarım kalan transferi devam ettirme (resume)
- PAKE — kodu şifreleme anahtarına çevirip sunucuyu güvenilmez tarafa
  indirgemek
- QR kod
