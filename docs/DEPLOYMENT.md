# PureSend — Sunucu Kurulum ve Dağıtım Rehberi (Deployment Guide)

Bu kılavuz, PureSend buluşma (rendezvous) ve yedek aktarım (relay) sunucusunun Docker Compose veya ters vekil (reverse proxy) arkasında güvenli, kesintisiz ve standart bir şekilde çalıştırılması için gerekli adımları içerir.

---

## 1. Mimarî ve Ön Koşullar

Sunucu (`cmd/server`) istemcilerin dosya içeriklerine asla dokunmaz ve sıfır-bilgi (zero-knowledge) mantığıyla çalışır:
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
