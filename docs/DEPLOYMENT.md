# PureSend — Sunucu Kurulum ve Dağıtım Rehberi (Deployment Guide)

Bu kılavuz, PureSend buluşma (rendezvous) ve yedek aktarım (relay) sunucusunun Docker Compose veya ters vekil (reverse proxy) arkasında güvenli, kesintisiz ve standart bir şekilde çalıştırılması için gerekli adımları içerir.

---

## 1. Mimarî ve Ön Koşullar

Sunucu (`cmd/server`) istemcilerin dosya içeriklerine asla dokunmaz ve oda kodlarının yalnızca numarasını (`kiraz-liman-42` için `42`) görür; gizli kelimeler ona hiç gönderilmez:
* **8080/TCP:** libp2p WebSocket sinyalleşme ve DCUtR delik açma portu.
* **8081/TCP:** Sağlık kontrolü (`/health`) ve Prometheus metrik (`/metrics`) portu (yalnızca yerel erişim).

```
İstemci ──wss://p2p.alanadiniz.com:443──► [Ters Vekil / Tünel] ──ws://localhost:8080──► PureSend Sunucu
                                            (TLS Sonlandırma)                                (Docker)
```

---

## 2. Hızlı Başlangıç: Docker Compose (Önerilen)

En kolay ve güvenli yöntem, repo kökündeki `docker-compose.yml` dosyasını kullanmaktır.

### Adım 1: Alan Adınızı Belirleyin ve Başlatın

```bash
# Alan adınızı çevre değişkeni olarak tanımlayıp sunucuyu başlatın
PUBLIC_HOST=p2p.alanadiniz.com docker compose up -d
```

*(Dilerseniz repo köküne bir `.env` dosyası oluşturup `PUBLIC_HOST=p2p.alanadiniz.com` yazabilirsiniz.)*

### Adım 2: Sunucu Peer ID'sini Alın

Sunucu ilk başladığında kalıcı bir kimlik anahtarı üretir. İstemcilerin sunucuya bağlanabilmesi için bu Peer ID gereklidir:

```bash
docker compose logs rendezvous | grep "Peer ID"
```

Çıktı örneği:
```text
  Peer ID: 12D3KooWKKqpYTw3D8arNmcNG7ZK1mPfSH2cQ7ohZqHBmYN6eEAn

Client address:
  /dns4/p2p.alanadiniz.com/tcp/443/tls/ws/p2p/12D3KooWKKqpYTw3D8arNmcNG7ZK1mPfSH2cQ7ohZqHBmYN6eEAn
```

> ⚠️ **ÖNEMLİ:** `rendezvous-key` Docker volume'ü sunucu kimliğini saklar. Bu volume silinirse Peer ID değişir ve eski istemciler sunucuya bağlanamaz.

---

## 3. Ters Vekil (Reverse Proxy) & Tünel Seçenekleri

Sunucu yerel ağda düz `ws://` dinlediği için TLS sonlandırması ters vekil tarafından yapılmalıdır. İhtiyacınıza uygun olanı seçin:

### Seçenek A: Cloudflare Tunnel (Statik IP Gerektirmez)

Eğer sunucunuzun sabit bir genel IP'si veya açık portu yoksa Cloudflare Tunnel en pratik çözümdür.

`cloudflared` ingress yapılandırmanıza (`/etc/cloudflared/config.yml`) ekleyin:

```yaml
tunnel: <tunnel-uuid-veya-adi>
credentials-file: /root/.cloudflared/<tunnel-uuid>.json

ingress:
  - hostname: p2p.alanadiniz.com
    service: http://localhost:8080
    originRequest:
      connectTimeout: 30s
  - service: http_status:404
```

Servisi yeniden başlatın:
```bash
sudo systemctl restart cloudflared
```

---

### Seçenek B: Caddy (Otomatik Let's Encrypt TLS)

Sabit IP'li bir VPS kullanıyorsanız Caddy otomatik SSL sertifikası üretir ve WebSocket trafiğini yönlendirir.

`/etc/caddy/Caddyfile`:
```caddy
p2p.alanadiniz.com {
    reverse_proxy localhost:8080
}
```

---

### Seçenek C: Nginx

Mevcut bir Nginx altyapınız varsa `/etc/nginx/sites-available/puresend.conf`:

```nginx
server {
    server_name p2p.alanadiniz.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
    }

    listen 443 ssl; # SSL sertifika direktiflerinizi ekleyin
}
```

---

### Tüm Seçenekler İçin: Kenar Katmanında IP Başına Hız Sınırı

Tünelin ya da ters vekilin arkasında bütün istemciler sunucuya aynı adresten (tünelin kendisinden) gelir. Bu yüzden `-trusted-proxies` ağlarından gelen bağlantılar libp2p'nin adres başına sınırlarından muaftır; muaf olmasalardı, dünyadaki bütün kullanıcılar tek bir adresin 8 bağlantılık payını paylaşırdı. Sunucunun kendi sınırları (eş başına bir oda, eş başına dakikada 5 sorgu, sunucu genelinde dakikada 200 boş sorgu) her yeni bağlantıyı biraz pahalılaştırır; ama istemci kimliği bedava olduğundan tek bir adresten gelen bir seli **durduramaz**. Oda numaraları herkese açık olduğu ve gönderici 3 yanlış koddan sonra odasını kapattığı için, sınırsız bağlantı açabilen biri numaraları tarayıp odaları kapatabilir. Adres başına sınırın yeri kenar katmanıdır.

Her istemci oturumu tek bir WebSocket bağlantısıdır, yani saydığımız şey `/` yoluna gelen yeni bağlantı istekleridir.

#### Cloudflare (Tunnel ile)

Cloudflare Dashboard → alan adı → **Security** → **WAF** → **Rate limiting rules** → **Create rule**:

| Ayar | Değer |
| :--- | :--- |
| Kural adı | `PureSend bağlantı sınırı` |
| İfade | *URI Path* **equals** `/` (ifade düzenleyicide: `(http.request.uri.path eq "/")`) |
| Sayılan özellik | IP |
| Sınır | **10 saniyede 5 istek** |
| Eylem | **Block**, süre **10 saniye** |

Ücretsiz planda tek kural hakkı vardır; ifadede yalnızca yol (Path) alanı kullanılabilir, sayma ve engelleme süresi 10 saniyedir. Alan adı (Host) seçilemediği için kural, bu bölgede (zone) Cloudflare üzerinden geçen **bütün** alt alan adlarının `/` isteklerine uygulanır; aynı bölgede bir web sitesi de varsa, onun ana sayfasına gelen istekler de sayılır. 10 saniyede 5 istek bir insanın gezinmesine dokunmaz. Pro ve üstü planlarda ifadeye `http.host eq "rendezvous.alanadiniz.com"` ekleyip süreleri uzatabilirsiniz (örn. dakikada 20 istek, 10 dakika engel).

#### Nginx

```nginx
# http bloğunda: IP başına dakikada 20 yeni bağlantı
limit_req_zone $binary_remote_addr zone=puresend:10m rate=20r/m;

server {
    server_name p2p.alanadiniz.com;

    location / {
        limit_req zone=puresend burst=5 nodelay;
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
    }

    listen 443 ssl;
}
```

Caddy'nin standart sürümünde hız sınırı yoktur; gerekiyorsa [caddy-ratelimit](https://github.com/mholt/caddy-ratelimit) eklentisiyle derlenmiş bir Caddy ya da yukarıdaki Nginx yapılandırması kullanılmalıdır.

`puresend_lookups_throttled_total` ve `libp2p_rcmgr_blocked_resources` metriklerindeki ani artışlar, kenar sınırının yetersiz kaldığını gösterir.

---

## 4. Dağıtımı Doğrulama ve Sağlık Kontrolü

### 1. Yerel Sağlık Kontrolü
Sunucu üzerinde JSON sağlık çıktısını test edin:
```bash
curl http://localhost:8081/health
```
**Beklenen Yanıt:**
```json
{"status":"ok","version":"v0.2.0","peer_id":"12D3KooW...","active_rooms":0}
```

### 2. Dışarıdan WebSocket El Sıkışması Testi
Sunucu dışındaki bir ağdan WebSocket bağlantısını test edin:
```bash
curl -sI https://p2p.alanadiniz.com \
     -H "Connection: Upgrade" -H "Upgrade: websocket"
```
**Beklenen Yanıt:** `HTTP/1.1 101 Switching Protocols`.

---

## 5. Güvenlik Sertleştirmesi ve İzleme

### 5.1 Docker Güvenliği
Sağlanan `docker-compose.yml` şu sertleştirmelerle birlikte gelir:
* **Salt-okunur Dosya Sistemi (`read_only: true`):** Konteyner kök dizinine zararlı dosya yazılamaz.
* **Yetki İzolasyonu (`cap_drop: ALL`, `no-new-privileges: true`):** Root yetki yükseltmeleri engellenir.
* **Port İzolasyonu:** `8080` ve `8081` yalnızca `127.0.0.1` dinler; dış dünyaya doğrudan açılmaz.

### 5.2 Prometheus Metrikleri
Sunucu `http://localhost:8081/metrics` üzerinden metrik yayınlar. Öne çıkan metrikler:

| Metrik | Anlamı |
|---|---|
| `puresend_active_rooms` | Anlık aktif transfer odası sayısı |
| `puresend_rooms_expired_total` | Zaman aşımına uğrayıp kapatılan odalar |
| `puresend_rooms_evicted_total` | Tablo dolduğunda yer açmak için erken düşürülen, sahibi ayrılmış odalar |
| `puresend_lookups_throttled_total` | Hız sınırına (rate limit) takılan oda sorguları |
| `libp2p_relaysvc_data_transferred_bytes_total` | Röle üzerinden akan veri miktarı (bayt) |
| `libp2p_rcmgr_blocked_resources` | Kaynak yöneticisinin reddettiği aşırı istekler |

### 5.3 Sunucu Kimlik Anahtarını Yedekleme (Kurtarma)
Sunucu anahtarını parola yöneticinizde saklamak için:
```bash
docker compose exec rendezvous base64 -w0 /data/server.key
```
Yeni veya farklı bir sunucuda aynı kimliği kullanmak için compose ortamında `FT_IDENTITY_KEY` değişkenine bu çıktıyı atamanız yeterlidir:
```yaml
environment:
  - FT_IDENTITY_KEY=CAESQ...
```

---

## 6. Sürüm Yayınlama ve İmzalama

İstemciler `v*` etiketi itildiğinde `.github/workflows/release.yml` ile derlenip GitHub Releases'e yüklenir. İki kural geçerlidir:

### 6.1 Yayınlanmış bir sürüm değiştirilmez

İş akışı, etiketi için zaten bir sürüm bulunan bir yayını reddeder. Yayınlanmış dosyaların özetleri kullanıcılar, `puresend -update`, kurulum betikleri ve AUR `PKGBUILD` tarafından denetlenmiştir; aynı etiket altında yeniden yayın, bu denetimlerin hepsini geçersiz kılar. Bir düzeltme yeni bir sürüm numarasıdır (`v1.0.1`). Deponun **Settings → General → Releases** bölümünde GitHub'ın *release immutability* ayarı varsa açın; etiket ve dosyalar GitHub tarafında da kilitlenir.

### 6.2 `checksums.txt` minisign ile imzalanır

`puresend -update` ve kurulum betikleri indirdikleri arşivi her zaman `checksums.txt` ile karşılaştırır. Bu, bozuk ya da değiştirilmiş bir indirmeyi yakalar; ama sürüm sayfasını değiştirebilen biri listeyi de değiştirebilir. İmza, listeyi başka bir yerde saklanan bir anahtara bağlar.

Bir kez yapılacak kurulum (anahtar parolasız üretilir, çünkü iş akışı parola giremez):

```bash
minisign -G -W -p puresend.pub -s puresend.key
```

1. **Settings → Secrets and variables → Actions → Secrets:** `MINISIGN_SECRET_KEY` = `puresend.key` dosyasının tamamı.
2. **Settings → Secrets and variables → Actions → Variables:** `FT_UPDATE_KEY` = `puresend.pub` dosyasının **ikinci satırı** (`RW...` ile başlar).
3. Aynı `RW...` satırını `install.sh` içindeki `PUBKEY=""` ve `install.ps1` içindeki `$pubKey = ""` değerlerine yazın. İki betik de imzayı yalnızca makinede `minisign` kuruluysa denetler; SHA-256 denetimi her durumda yapılır.
4. `puresend.key` dosyasını parola yöneticinizde ya da çevrimdışı bir yerde saklayın ve depoya koymayın (`.gitignore` `*.key` dosyalarını zaten dışarıda tutar).

Bundan sonra her yayın `checksums.txt.minisig` dosyasını da içerir ve istemcilere `FT_UPDATE_KEY` gömülür. İş akışı, iki değerden yalnızca biri ayarlıysa ya da gizli anahtar açık anahtarla eşleşmiyorsa hiçbir şey derlemeden durur; eşleşmeyen bir anahtarla çıkan istemciler bir daha kendiliğinden güncellenemezdi.

> ⚠️ Anahtarı gömülü istemciler, imzasız ya da başka anahtarla imzalanmış bir sürüme güncellenmeyi reddeder. Gizli anahtarı kaybetmek, bu istemcilerin `-update` ile güncellenemeyeceği anlamına gelir; kullanıcıların kurulum betiğiyle yeniden kurması gerekir.

Bir sürümü elle doğrulamak için:

```bash
minisign -Vm checksums.txt -P RW...
sha256sum --ignore-missing -c checksums.txt
```

### 6.3 Sunucu adresi ve `server.txt`

Her istemciye buluşma noktasının adresi, Peer ID'siyle birlikte gömülür: `FT_SERVER` depo değişkeni ayarlıysa o, değilse `release.yml` içindeki varsayılan. Adres yanlışsa bu, ancak insanlar dosyaları indirdikten sonra fark edilir. Bu yüzden iş akışı, gömeceği adresin `FT_SERVER_LIST` (varsayılan `https://puresend.madebybaki.com/server.txt`) içinde listelendiğini denetler ve listede yoksa hiçbir şey derlemeden durur. İkisi uyuşmuyorsa biri eskimiştir; çoğu zaman sunucu anahtarı değiştikten sonra güncellenmemiş varsayılan. Liste o an indirilemezse iş akışı yalnızca uyarı verip devam eder.

Sunucu anahtarı değiştiğinde (bkz. 5.3) sıra şudur: önce `server.txt`'e yeni adres eklenir; eski sürümler gömülü adrese ulaşamayınca oraya bakar. Sonra `FT_SERVER` değişkeni ya da iş akışındaki varsayılan güncellenir, en son yeni sürüm etiketlenir.
