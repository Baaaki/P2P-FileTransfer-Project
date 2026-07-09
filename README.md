# PureSend 📦

[libp2p](https://libp2p.io) üzerine kurulu, uçtan uca P2P dosya transfer
uygulaması. İki bilgisayar küçük bir buluşma (rendezvous) sunucusu üzerinden
birbirini bulur; ardından dosyalar **sunucuya hiç uğramadan, doğrudan iki
bilgisayar arasında** aktarılır — farklı ağlarda, ikisi de ev modemi (NAT)
arkasında olsa bile.

🇬🇧 English documentation: [README.en.md](README.en.md)

```
Gönderen (İstanbul)                Sunucu (VPS)                 Alıcı (İzmir)
       │                                │                             │
       │ 1. "cherry-harbor-42"          │                             │
       │    odasını adreslerimle        │                             │
       │    kaydet ────────────────────►│◄─── 2. "cherry-harbor-42"   │
       │                                │        odasında kim var?    │
       │                                │                             │
       │◄══════ 3. Doğrudan P2P bağlantı (hole punching) ════════════►│
       │                4. Dosyalar doğrudan akar                     │
```

## Nasıl çalışıyor?

1. **Buluşma** — Gönderen, rastgele bir oda kodunu (ör. `cherry-harbor-42`)
   kendi ağ adresleriyle birlikte sunucuya kaydeder. Alıcı aynı kodu girince
   bu adresleri sunucudan alır.
2. **NAT delme** — İki taraf da NAT arkasındaysa ilk bağlantı sunucunun
   **Circuit Relay v2** köprüsü üzerinden kurulur; libp2p'nin **DCUtR hole
   punching** mekanizması bunu doğrudan bağlantıya yükseltir.
3. **Transfer** — Dosyalar doğrudan bağlantı üzerinden, dosya başına
   **SHA-256 doğrulamasıyla** akar. Hole punching başarısız olursa
   (ör. simetrik NAT) transfer yedek olarak relay üzerinden yine tamamlanır.

## Teknolojiler

- **Go 1.25+** — tek harici bağımlılık: **go-libp2p v0.48**
- Kullanılan libp2p özellikleri: TCP + QUIC transport, Circuit Relay v2,
  DCUtR hole punching, AutoNAT v2, UPnP port yönlendirme, Noise/TLS
  şifreleme (her zaman açık)
- İki küçük özel protokol: `/puresend/rendezvous/1.0.0`
  (oda kayıt/sorgulama) ve `/puresend/transfer/1.0.0`
  (manifest + dosya baytları + onay)

```
cmd/server      rendezvous + relay sunucusu (VPS'te çalışır)
cmd/client      interaktif terminal uygulaması (gönder / al)
internal/       iki protokolün implementasyonu
```

## Kurulum ve çalıştırma

```bash
git clone <repo> && cd PureSend
go build ./...
```

### 1. Sunucuyu başlat — portu açık herhangi bir makine (ör. ucuz bir VPS)

```bash
go run ./cmd/server -port 4001
```

Güvenlik duvarında 4001 portunu (**TCP ve UDP**) açın. Sunucu, istemcilerin
kullanacağı adresleri ekrana basar — genel (public) IP'yi içeren satırı
kopyalayın:

```
/ip4/<VPS-IP>/tcp/4001/p2p/<PeerID>
```

Kimlik anahtarı `server.key` dosyasına kaydedilir; sunucu yeniden başlasa
da adres değişmez.

**Ya da Docker ile** (konteyner iç ağ IP'lerini yazdırır — adresi, genel IP
ve loglardaki Peer ID ile kendiniz oluşturun):

```bash
docker build -t puresend-server .
docker run -d -p 4001:4001 -p 4001:4001/udp -v ft-data:/data \
  --restart unless-stopped puresend-server
```

> *İstemciyi* Docker'da çalıştırmayın — konteynerin ek NAT katmanı hole
> punching'i bozar. İstemci tek bir statik ikili; doğrudan makinede çalıştırın.

### 2. Gönderen tarafta

```bash
go run ./cmd/client -server /ip4/<VPS-IP>/tcp/4001/p2p/<PeerID>
```

**1) Send files**'ı seçin, dosya yollarını satır satır girin (bitirmek için
boş bırakın). Ekrana basılan oda kodunu alıcıya iletin — kod 1 saat geçerlidir.

### 3. Alıcı tarafta

Diğer bilgisayarda aynı komutu çalıştırın, **2) Receive files**'ı seçip oda
kodunu girin. Dosyalar varsayılan olarak `received/` dizinine iner ve her
biri SHA-256 özetiyle doğrulanır:

```
Sender found: 12D3KooWHxxef3pj...
✓ Direct P2P connection established — files will bypass the server.
  photo1.jpg  [████████████████████████] 100%  2.1 MB / 2.1 MB
✓ 3 file(s) received and verified
```

> **Yerelde denemek:** üç programı da tek makinede üç ayrı terminalde,
> sunucunun yazdırdığı `127.0.0.1`'li adresi kullanarak çalıştırabilirsiniz.

## Sınırlamalar

- Oda kodunu bilen herkes dosyaları alabilir (tek transfer, 1 saatlik
  tasarım; PAKE tabanlı parola doğrulama güzel bir ek olurdu).
- Kesilen transfer baştan başlar — henüz devam etme (resume) yok.
