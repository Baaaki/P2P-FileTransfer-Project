# FileTransferilla — Sunucu Kurulum ve Dağıtım Rehberi (Deployment Guide)

Bu belge, FileTransferilla buluşma (rendezvous) ve yedek aktarım (relay) sunucusunun **Ubuntu + Docker Compose + Cloudflare Tunnel** altyapısında 7/24 kesintisiz ve güvenli şekilde çalıştırılması için gereken tüm adımları içerir.

---

## 1. Mimarî Genel Bakış

Sunucu (`cmd/server`) dosya içeriklerine asla dokunmaz ve bunları diske kaydetmez. İki temel görevi vardır:
1. **Buluşma (Rendezvous):** Gönderici ve alıcının oda kodları üzerinden ağ adreslerini takas etmesini sağlar.
2. **Circuit Relay v2:** NAT arkasındaki iki cihaz arasında doğrudan P2P bağlantı (DCUtR hole punching) kurulana kadar ilk el sıkışmayı köprüler. Doğrudan yol açılamazsa sınırlı bir yedek aktarım sağlar.

```
İstemci ──wss://...:443──► Cloudflare Edge ──ws://localhost:8080──► FileTransferilla Sunucu
                              (TLS burada biter)                        (Docker)
```

`cloudflared` internete yalnızca HTTP/WebSocket trafiğini açar. Bu nedenle sunucu libp2p WebSocket transport üzerinden haberleşir. Buluşma ve delik açma koordinasyonu birkaç kilobayttır; P2P delik açıldıktan sonra dosya trafiği Cloudflare'e hiç uğramaz.

---

## 2. Docker Compose ile Başlatma

Repo kökündeki `docker-compose.yml` dosyası üretim ortamı için hazır ve sertleştirilmiştir.

```bash
# Alan adınızı belirterek sunucuyu arka planda başlatın
PUBLIC_HOST=rendezvous.madebybaki.com docker compose up -d
```

### Peer ID'yi Alma
Sunucu ilk açıldığında kalıcı bir kimlik anahtarı üretir. Bu Peer ID istemcilerin sunucuya bağlanabilmesi için gereklidir:

```bash
docker compose logs rendezvous | grep "Peer ID"
```

Çıktı örneği:
```
  Peer ID: 12D3KooWKKqpYTw3D8arNmcNG7ZK1mPfSH2cQ7ohZqHBmYN6eEAn

Client address:
  /dns4/rendezvous.madebybaki.com/tcp/443/tls/ws/p2p/12D3KooWKKqpYTw3D8arNmcNG7ZK1mPfSH2cQ7ohZqHBmYN6eEAn
```

> ⚠️ **ÖNEMLİ:** `rendezvous-key` Docker volume'ünü **kesinlikle silmeyin**. Peer ID değişirse daha önce dağıtılan istemciler bu sunucuyu bulamaz.

---

## 3. Sağlık Kontrolü ve Prometheus Metrikleri

Sunucu yerel arayüzde (`127.0.0.1:8081`) sağlık ve Prometheus metrik uç noktalarını dinler:

```bash
# Sağlık kontrolü (JSON)
curl http://localhost:8081/health
# {"status":"ok","version":"v0.2.0","peer_id":"12D3KooW...","active_rooms":0}

# Prometheus metrikleri
curl -s http://localhost:8081/metrics | grep -E '^(filetransferilla|libp2p_relaysvc)_'
```

### Önemli Prometheus Sorguları

| Amaç | PromQL Sorgusu |
|---|---|
| Relay üzerinden akan veri hızı (yedek aktarım) | `rate(libp2p_relaysvc_data_transferred_bytes_total[1h])` |
| Açılan relay bağlantıları | `rate(libp2p_relaysvc_connections_total{type="opened"}[1h])` |
| Anlık aktif oda sayısı | `filetransferilla_active_rooms` |
| Zaman aşımına uğrayan odalar | `rate(filetransferilla_rooms_expired_total[1h])` |
| Sahibi kopup dönmeyen odalar | `rate(filetransferilla_rooms_abandoned_total[1h])` |
| Engellenen / kısıtlanan sorgular | `rate(filetransferilla_lookups_throttled_total[5m])` |
| Kaynak yöneticisi sınırlarına takılanlar | `libp2p_rcmgr_blocked_resources` |

---

## 4. Cloudflare Tunnel Yapılandırması

### Mevcut bir tünel varsa
Aynı makinede çalışan mevcut bir `cloudflared` varsa config dosyasını ezmeyin. DNS rotasını ekleyip ingress listesini güncelleyin:

```bash
cloudflared tunnel route dns <mevcut-tünel-adı> rendezvous.madebybaki.com
```

`/etc/cloudflared/config.yml` dosyasındaki `ingress:` bölümüne, `http_status:404` kuralından **önce** ekleyin:
```yaml
  - hostname: rendezvous.madebybaki.com
    service: http://localhost:8080
    originRequest:
      connectTimeout: 30s
```

Yapılandırmayı doğrulayıp servisi yeniden başlatın:
```bash
cloudflared tunnel ingress validate
sudo systemctl restart cloudflared
```

### Sıfırdan tünel kuruluyorsa
```bash
cloudflared tunnel login
cloudflared tunnel create filetransferilla
cloudflared tunnel route dns filetransferilla rendezvous.madebybaki.com
sudo cp deploy/cloudflared-config.yml /etc/cloudflared/config.yml
sudo cloudflared service install
```

### IP Başına Hız Sınırı (Rate Limiting — Şiddetle Önerilir)
Sunucu tünelin arkasında gerçek istemci IP adresini doğrudan göremez (bütün bağlantılar yerel tünelden gelir). Kod tahmin saldırılarına (brute-force) karşı IP başına sınırı Cloudflare Dashboard üzerinden koyabilirsiniz:

* **Yol:** *Cloudflare Dashboard → Security → Security rules → Create rule → Rate limiting rules*
* **Koşul:** `Hostname equals rendezvous.madebybaki.com`
* **Karakteristik:** IP
* **Eşik:** 20 istek / 10 saniye
* **Eylem:** Block (10 saniye)

---

## 5. Dışarıdan Erişimi Doğrulama

Sunucunun bulunduğu yerel ağın dışındaki bir bağlantıdan (örneğin mobil veri / telefon interneti) WebSocket el sıkışmasını test edin:

```bash
curl -sI https://rendezvous.madebybaki.com \
     -H "Connection: Upgrade" -H "Upgrade: websocket"
```
Beklenen yanıt: `HTTP/1.1 101 Switching Protocols`.

---

## 6. Konteyner Güvenliği ve Kimlik Yedeği

`docker-compose.yml` şu güvenlik sertleştirmelerini içerir:
- **Salt-okunur kök dosya sistemi (`read_only: true`):** Konteyner içine zararlı dosya yazılamaz.
- **Düşürülmüş yetkiler (`cap_drop: ALL`, `no-new-privileges: true`):** Root yetki yükseltmeleri engellenir.
- **Kaynak sınırları (`mem_limit: 512m`, `pids_limit: 256`):** Olası bellek ve süreç tükenmesi saldırılarına karşı host korunur.
- **Yerel Port İzolasyonu:** `8080` ve `8081` yalnızca `127.0.0.1` arayüzüne bağlanır.

### Kimlik Anahtarını Yedekleme (Disksiz Kurtarma)
Sunucu kimlik anahtarını sunucu dışına yedeklemek için:
```bash
docker compose exec rendezvous base64 -w0 /data/server.key
```
Elde edilen base64 dizgisini parola yöneticinizde saklayabilirsiniz. Farklı bir sunucuda bu kimliği çalıştırmak için compose ortamına `FT_IDENTITY_KEY` olarak tanımlamanız yeterlidir:
```yaml
environment:
  - FT_IDENTITY_KEY=CAESQ...
```

