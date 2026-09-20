# PureSend — Mimari ve Algoritma Akışı (Architecture & Protocol Spec)

Bu belge; PureSend (FileTransferilla) eşler arası (P2P) dosya aktarım sisteminin ağ topolojisini, şifreleme mekanizmalarını ve algoritma akışını şematik olarak açıklar.

---

## 1. Sistem ve Ağ Topolojisi (System Topology)

PureSend, merkezi sunucularda veri depolamadan doğrudan cihazdan cihaza (Zero-Trust P2P) transfer sağlar:

```mermaid
flowchart TB
    subgraph Users["Kullanıcılar"]
        SenderUser["Gönderici (Kullanıcı A)"]
        ReceiverUser["Alıcı (Kullanıcı B)"]
    end

    subgraph PureSendSystem["PureSend CLI"]
        SenderApp["Gönderici Düğüm (Client A)"]
        ReceiverApp["Alıcı Düğüm (Client B)"]
    end

    subgraph Infrastructure["Sinyal Altyapısı"]
        CFEdge["Cloudflare Edge (WSS / 443)"]
        RendezvousServer["Buluşma & Röle Sunucusu (cmd/server:8080)"]
    end

    SenderUser -->|"Dosya seçer & oda kodu üretir"| SenderApp
    ReceiverUser -->|"Oda kodunu girer & onaylar"| ReceiverApp

    SenderApp <-->|"1. Sinyalleşme & Oda Kaydı (WSS)"| CFEdge
    ReceiverApp <-->|"1. Oda Çözümleme (WSS)"| CFEdge
    CFEdge <-->|"ws://localhost:8080"| RendezvousServer

    SenderApp <-.->|"2. Doğrudan P2P Tüneli (DCUtR / TCP / QUIC)"| ReceiverApp
    SenderApp <-.->|"3. Fallback: Circuit Relay v2 (Yalnızca delik açılamazsa)"| RendezvousServer
    ReceiverApp <-.->|"3. Fallback: Circuit Relay v2 (Yalnızca delik açılamazsa)"| RendezvousServer
```

---

## 2. Uçtan Uca Algoritma ve Protokol Akışı (Sequence Diagram)

PureSend protokolünün 4 ana adımı (Sinyal, NAT Delme, SPAKE2 Doğrulama, Akış):

```mermaid
sequenceDiagram
    autonumber
    participant S as Gönderici (Sender)
    participant Rnd as Buluşma Sunucusu (Rendezvous)
    participant R as Alıcı (Receiver)

    Note over S,Rnd: 1. Oda Eşleme (Discovery)
    S->>Rnd: WSS Bağlantısı & Oda Kaydı ("kiraz-liman-42", PeerID_S)
    Rnd-->>S: Kayıt Onayı (TTL: 10 dk)
    R->>Rnd: WSS Bağlantısı & Oda Sorgulama ("kiraz-liman-42")
    Rnd-->>R: Gönderici Adresleri (PeerID_S, Multiaddrs)

    Note over S,R: 2. DCUtR ile NAT Delme (Hole Punching)
    R->>Rnd: Röle Üzerinden Göndericiye Köprü Kur
    Rnd->>S: Köprü Bağlantısını İlet
    S->>R: DCUtR Port Eşleme Senkronizasyonu
    S-->>R: Doğrudan P2P Soketi Açıldı (Sunucu Devre Dışı!)

    Note over S,R: 3. Sıfır-Bilgi Kimlik Doğrulama (SPAKE2)
    R->>S: PAKE Mesaj 1 (P-256 Party A)
    S->>R: PAKE Mesaj 2 + ConfirmSender (HMAC-SHA256)
    Note over S,R: Ortak anahtar türetilir (Peer ID'ler oturuma bağlanır)
    R->>S: ConfirmReceiver (Karşılıklı Doğrulama Tamam)

    Note over S,R: 4. Manifest, Onay ve Akış Transferi
    S->>R: Offer / Manifest (Dosya listesi, boyutlar, SHA-256)
    R-->>S: Transfer Ack: Accepted (Kaldığı yer / resume offseti)
    
    loop Her 32 KB Blok İçin
        S->>R: 32 KB Chunk (Snappy Sıkıştırma + SHA-256)
        R->>R: Diske Yaz & SHA-256 Doğrula
    end

    R->>S: Final Ack (Transfer Başarılı)
```

---

## 3. Ağ Taşıyıcı ve Fallback Karar Ağacı (Traversal Flowchart)

Bağlantı koşullarına göre çalışma zamanı rota seçimi:

```mermaid
flowchart TD
    Start(["Transfer Başlatıldı"]) --> DirectLAN{"Aynı Yerel Ağda (LAN) mı?"}
    
    DirectLAN -- "Evet" --> UseLAN["Doğrudan LAN Soketi (Line-Rate Hız: 112+ MB/s)"]
    DirectLAN -- "Hayır" --> HolePunch{"DCUtR Delik Açma Başarılı mı?<br/>(Konik NAT / Port Eşleme)"}
    
    HolePunch -- "Evet (Varsayılan)" --> UseDirectWAN["Doğrudan WAN P2P Tüneli<br/>(Veri sunucuya uğramaz, hat sınırı hız)"]
    HolePunch -- "Hayır (Simetrik NAT)" --> RelayFallback["Circuit Relay v2 Köprüsü<br/>(Şifreli yedek hat - Hız/kota sınırlı)"]

    UseLAN --> StartCrypto["SPAKE2 (P-256) Kriptografik El Sıkışması"]
    UseDirectWAN --> StartCrypto
    RelayFallback --> StartCrypto

    StartCrypto --> VerifyAuth{"Parola / Kod Eşleşti mi?"}
    VerifyAuth -- "Evet" --> TransferStream["32 KB Blok Akışı + Snappy + SHA-256"]
    VerifyAuth -- "Hayır" --> DropConn["Bağlantıyı Derhal Kapat"]
```

---

## 4. Temel Algoritma Prensipleri

1. **Sıfır Güven (Zero-Trust) Buluşma:**
   * Buluşma sunucusu dosya içeriğini veya dosya adlarını göremez.
   * Sunucu, gönderici yerine araya kendi sahte düğümünü koyamaz (Impersonation koruması); çünkü istemciler SPAKE2 el sıkışmasında Peer ID'leri HMAC anahtarına bağlar.
2. **Sabit Bellekli Akış ($O(1)$ RAM):**
   * Dosyalar belleğe yüklenmez; sabit 32 KB dilimler (chunks) halinde okunup Snappy ile sıkıştırılarak doğrudan ağ soketine verilir.
   * Alıcı tarafında her dilim diske yazılırken anlık SHA-256 hash havuzuna beslenir.
3. **Kaldığı Yerden Devam (Resume):**
   * Bağlantı koptuğunda alıcı, diske kısmen yazılmış `.part` dosyasının boyutunu ve hash durumunu göndericiye aktarır; aktarım yalnızca eksik kalan bayttan devam eder.
4. **Dosya Sistemi Güvenliği (SafeJoin):**
   * Gelen manifestteki tüm yollar `SafeJoin` filtresinden geçer; mutlak yollar (`/etc/passwd`), dizin atlamalar (`../`) ve tehlikeli sistem aygıt adları (`CON`, `PRN`, `AUX`) otomatik temizlenir.
