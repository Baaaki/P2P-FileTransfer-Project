# PureSend 📦

[libp2p](https://libp2p.io) üzerine kurulu, uçtan uca P2P dosya transfer
uygulaması. İki bilgisayar küçük bir buluşma (rendezvous) sunucusu üzerinden
birbirini bulur; ardından dosyalar **sunucuya hiç uğramadan, doğrudan iki
bilgisayar arasında** aktarılır — farklı ağlarda, ikisi de ev modemi (NAT)
arkasında olsa bile.

Kullanıcı hiçbir teknik şey görmez: programı açar, dosyayı seçer, ekranda
çıkan **3 kelimelik kodu** arkadaşına iletir. Hepsi bu.

🇬🇧 English documentation: [README.en.md](README.en.md)
📋 Ürünleşme planı: [ROADMAP.md](ROADMAP.md)

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
| Windows | `puresend_<sürüm>_windows_x86_64.zip` |
| macOS (M1 / M2 / M3 / M4) | `puresend_<sürüm>_macOS_arm64.tar.gz` |
| macOS (2020 öncesi, Intel) | `puresend_<sürüm>_macOS_x86_64.tar.gz` |
| Linux | `puresend_<sürüm>_linux_x86_64.tar.gz` |

Arşivin içinden `puresend` adında **tek bir dosya** çıkar.
Başka hiçbir şeye ihtiyacın yok.

- **Windows:** dosyaya çift tıkla, program açılır.
- **macOS:** çift tıkla — Terminal penceresinde açılır.
- **Linux:** masaüstü ortamları terminal programlarını çift tıklamayla
  açmayabilir; en garantisi terminalden çalıştırmak:
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
yanındaki `checksums.txt` dosyasını kullanabilirsin:

```bash
sha256sum -c checksums.txt --ignore-missing
```

---

## Hangi klasör nerede çalışır?

Bu ayrımı karıştırmamak önemli:

| Klasör | Nerede çalışır | Nasıl dağıtılır |
|---|---|---|
| **`cmd/server/`** | 🖥️ **Ubuntu sunucunda**, 7/24 açık | Docker + Cloudflare Tunnel |
| **`cmd/client/`** | 💻 **Kullanıcının masaüstünde** | GitHub Releases'ten indirilen tek dosya |
| `internal/rendezvous/` | ikisinde de | ortak protokol |
| `internal/transfer/` | sadece istemci | sunucu bu kodu hiç çalıştırmaz |
| `internal/p2p/`, `internal/tui/` | sadece istemci | ağ katmanı + arayüz |
| `deploy/` | 🖥️ sunucu | cloudflared ayarları (compose kökte) |

**Kısaca:** sunucuda `cmd/server` çalışır ve dosyalara asla dokunmaz;
dosyalar kullanıcının çalıştırdığı `cmd/client`'tan çıkar.

---

## Kullanıcı ne yapıyor? (teknik olmayan biri için)

**Gönderen:**

1. Programı açar → *"Dosya göndereceğim"*
2. Dosya gezgininden dosyaları seçer → `s`
3. Ekranda kod çıkar: **`kiraz-liman-42`**
4. Kodu arkadaşına WhatsApp'tan yazar → *"✓ Arkadaşıma ilettim"*
5. Arkadaşı kodu girince gönderme kendiliğinden başlar

**Alan:**

1. Programı açar → *"Bana dosya gönderilecek"*
2. Kodu yazar: `kiraz-liman-42`
3. Gelen dosya listesini görür → *"Evet, indir"*
4. Dosyalar `İndirilenler/PureSend` klasörüne iner

Hiçbir aşamada IP adresi, port ya da ayar dosyası yok.

---

## Nasıl çalışıyor?

1. **Buluşma** — Gönderen rastgele bir oda kodunu kendi ağ adresleriyle
   birlikte sunucuya kaydeder. Alıcı aynı kodu girince adresleri alır.
2. **NAT delme** — İlk bağlantı sunucunun **Circuit Relay v2** köprüsü
   üzerinden kurulur; **DCUtR hole punching** bunu doğrudan bağlantıya
   yükseltir. Relay yalnızca aktif odası olan eşlere hizmet verir (ACL).
3. **Transfer** — Alıcı gelen listeyi görüp **onaylar**; dosyalar doğrudan
   bağlantı üzerinden, dosya başına **SHA-256 doğrulamasıyla** akar.
   Delme başarısız olursa transfer relay üzerinden yedeklenir.

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

## Sunucu kurulumu (Ubuntu + Cloudflare Tunnel + OpenShip)

### 1. Sunucuyu başlat

```bash
PUBLIC_HOST=puresend.madebybaki.com \
  docker compose up -d
```

Loglardan **Peer ID**'yi al — birazdan lazım:

```bash
docker compose logs rendezvous
```

```
  Peer ID: 12D3KooWKKqpYTw3D8arNmcNG7ZK1mPfSH2cQ7ohZqHBmYN6eEAn

Client address (bake this into the client build):
  /dns4/puresend.madebybaki.com/tcp/443/tls/ws/p2p/12D3KooW...
```

> ⚠️ `rendezvous-key` volume'ünü **silme**. Peer ID değişirse daha önce
> dağıttığın bütün istemciler çalışmaz hale gelir.

Sağlık kontrolü:

```bash
curl localhost:8081/health
# {"status":"ok","peer_id":"12D3KooW...","active_rooms":0}
```

### 2. Cloudflare Tunnel

**Makinede zaten bir tünel varsa** (başka bir proje için), config'i üzerine
yazma — tek bir `cloudflared` bütün hostname'lere aynı dosyadan hizmet
eder, üzerine yazarsan diğer projenin kuralı silinir ve o proje düşer.
Mevcut `ingress:` listesine kural **ekle**:

```bash
sudo cat /etc/cloudflared/config.yml          # önce ne olduğuna bak
cloudflared tunnel list

cloudflared tunnel route dns <mevcut-tünel> puresend.madebybaki.com

# config.yml'deki ingress listesine, catch-all 404'ten ÖNCE:
#   - hostname: puresend.madebybaki.com
#     service: http://localhost:8080
#     originRequest:
#       connectTimeout: 30s

cloudflared tunnel ingress validate
sudo systemctl restart cloudflared
```

Kurallar yukarıdan aşağı eşleşir ve `http_status:404` her şeyi yakalar —
yeni kural ondan sonra kalırsa hiç çalışmaz.

**Makinede hiç tünel yoksa** dosya olduğu gibi kullanılabilir:

```bash
cloudflared tunnel login
cloudflared tunnel create puresend
cloudflared tunnel route dns puresend puresend.madebybaki.com
sudo cp deploy/cloudflared-config.yml /etc/cloudflared/config.yml
sudo cloudflared service install
```

Ayrıntılar ve gerekçeler: [deploy/cloudflared-config.yml](deploy/cloudflared-config.yml)

### 3. Dışarıdan eriştiğini doğrula

Sunucunun **dışındaki** bir ağdan (telefon hotspot'u iyi bir test):

```bash
curl -sI https://puresend.madebybaki.com \
     -H "Connection: Upgrade" -H "Upgrade: websocket"
```

### OpenShip ile

OpenShip compose dosyasını **kısmen** uyguluyor. Gerçekte gözlenen
davranış (v0.4.8):

| Compose bloğu | Ne oluyor |
|---|---|
| `build`, `image` | ✅ uygulanıyor — context repo köküne sabitleniyor |
| `command:` | ❌ **yok sayılıyor** — Dockerfile'ın `CMD`'si çalışıyor |
| `volumes:` | ❌ **yok sayılıyor** — Dockerfile'daki `VOLUME` için anonim volume açılıyor |
| `ports:` | ⚠️ yeniden eşleniyor — `127.0.0.1:<sabitlenmiş>` (örn. `20001`) |
| `environment:` | ✅ uygulanıyor |

Bunun iki sonucu var ve ikisi de sessizce vurur:

**1. `command:` düştüğü için `-announce` kaybolur.** Sunucu kendini
konteynerin iç adresleriyle (`172.17.x.x`) tanıtır.

**2. Anonim volume yeniden deploy'da kaybolur** — `server.key` gider,
**Peer ID değişir** ve dağıttığın bütün istemciler ölür.

İkisinin de çözümü ortam değişkeni, çünkü OpenShip onları uyguluyor:

```bash
# Mevcut anahtarı konteynerden al (deploy'dan ÖNCE!)
docker exec <konteyner> base64 -w0 /data/server.key
```

OpenShip → servis → *Ortam değişkenleri*:

| Değişken | Değer |
|---|---|
| `FT_IDENTITY_KEY` | yukarıdaki base64 çıktısı (**gizli tut**) |
| `FT_ANNOUNCE` | `/dns4/puresend.madebybaki.com/tcp/443/tls/ws` |

Böylece kimlik diskten tamamen bağımsız olur. Açılışta
`Identity from: FT_IDENTITY_KEY` satırını görürsen doğru çalışıyordur.

**cloudflared hedefi**, `8080` değil OpenShip'in sabitlediği port:

```bash
docker ps --format '{{.Names}}\t{{.Ports}}' | grep filetransfer
# ... 127.0.0.1:20001->8080/tcp   →  ingress: http://localhost:20001
```

> ⚠️ Bu port yeniden deploy'da değişebilir. Değişirse tünel sessizce
> kırılır — "bir gün çalışmıyor" olursa ilk buraya bak.

`8081` host'a çıkmaz, sağlık kontrolü konteyner içinden:

```bash
docker exec <konteyner> wget -qO- http://127.0.0.1:8081/health
```

---

## İstemciyi yayınlama

Sunucu adresi **derleme sırasında** gömülür; kullanıcı hiçbir ayar yapmaz.

GitHub'da → *Settings → Secrets and variables → Actions → Variables* →
`FT_SERVER` değişkenini oluştur:

```
/dns4/puresend.madebybaki.com/tcp/443/tls/ws/p2p/12D3KooW...
```

Sonra tag at:

```bash
git tag v0.1.0 && git push --tags
```

[GoReleaser](.goreleaser.yaml) 6 ikili üretir (Linux / macOS / Windows ×
x86_64 / arm64) ve Releases sayfasına koyar.

Elle derlemek istersen:

```bash
go build -ldflags "-X main.defaultServer=/dns4/.../p2p/12D3KooW..." ./cmd/client
```

---

## Geliştirme

```bash
go build ./...
go test ./...    # birim + libp2p üzerinden gerçek uçtan uca testler
```

Tek makinede denemek için üç terminal:

```bash
# 1) sunucu
go run ./cmd/server -ws-port 8080 -health-port 8081

# 2) gönderen  ve  3) alan  (Peer ID'yi sunucunun çıktısından al)
go run ./cmd/client -server /ip4/127.0.0.1/tcp/8080/ws/p2p/<PeerID>
```

## Teknolojiler

- **Go 1.25+**, **go-libp2p v0.48** — TCP + QUIC + WebSocket transport,
  Circuit Relay v2, DCUtR hole punching, AutoNAT v2, UPnP, Noise/TLS
- **Bubble Tea + Lipgloss** — terminal arayüzü
- İki özel protokol: `/puresend/rendezvous/1.0.0` ve
  `/puresend/transfer/1.1.0`

## Sınırlamalar

- Oda kodunu ilk giren alıcı dosyaları alabilir. ~1,7 milyon kombinasyon,
  eş başına deneme sınırı ve oda sahipliği koruması var; yine de PAKE
  tabanlı doğrulama güzel bir ek olurdu. Şu haliyle **sunucu güvenilir
  taraftır** (alıcıya gönderenin kimliğini o söyler).
- Kesilen transfer baştan başlar — resume yok. Yarım dosya `.part` olarak
  temizlenir, bitmiş gibi görünen bozuk dosya kalmaz.
- Klasör gönderilemiyor, sadece dosya.
- Delme başarısız olursa relay bağlantı başına 256 MB ile sınırlı
  (`-relay-data` ile değiştirilebilir).
