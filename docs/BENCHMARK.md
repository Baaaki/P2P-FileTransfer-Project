# PureSend — Performans ve Benchmark Raporu (Performance Guide)

Bu belge, PureSend (FileTransferilla) motorunun mikro-benchmark sonuçlarını, ağ aktarım profillerini, bellek tüketim analizini ve bu testlerin nasıl tekrarlanabileceğini belgeler.

---

## 1. Yönetici Özeti (Executive Summary)

PureSend, yüksek performanslı ve düşük kaynak tüketen bir P2P dosya aktarım aracı olarak tasarlanmıştır:
* **Akış Tabanlı Mimari ($O(1)$ Bellek):** Dosya boyutu ne kadar büyük olursa olsun (örneğin 100 MB veya 50 GB), bellek tüketimi 32 KB'lık sabit bloklama (chunking) sayesinde ~30-45 MB bandında sabit kalır.
* **Yüksek Hızlı Sıkıştırma:** DEFLATE (`flate.HuffmanOnly`) tabanlı dinamik sıkıştırma motoru saniyede **~792 MB/s** veri işleme kapasitesine sahiptir.
* **Düşük Gecikmeli Güvenlik:** PAKE2 el sıkışması ve SHA-256 doğrulamaları milisaniyeler seviyesinde tamamlanır.
* **Sıfır Tahsisli (Zero-Alloc) Doğrulama:** SHA-256 format kontrolleri işlem başına **0 byte** bellek tahsisiyle çalışır.

---

## 2. Go Çalışma Zamanı Mikro-Benchmark Sonuçları

Aşağıdaki metrikler, PureSend çekirdek motorunda bulunan `testing.B` benchmark fonksiyonlarının resmi çalıştırma sonuçlarıdır:

> **Donanım Profili:** AMD Ryzen 7 5700X 8-Core (16 Threads) @ 3.4 GHz, Linux 6.x, Go 1.26 (amd64).

### 2.1 Veri Sıkıştırma ve Açma (`internal/transfer`)

| Benchmark | Yineleme Sayısı | Süre (ns/op) | Veri Hızı (MB/s) | Bellek / İşlem (B/op) | Tahsis (allocs/op) |
| :--- | :---: | :---: | :---: | :---: | :---: |
| `BenchmarkCompressChunk-16` | 28,274 | 40,392 ns/op | **792.24 MB/s** | 59 B/op | 1 allocs/op |
| `BenchmarkDecompressChunk-16` | 8,480 | 153,669 ns/op | **208.24 MB/s** | 125 B/op | 3 allocs/op |

* **Yorum:** PureSend her 32 KB dilimi `flate.HuffmanOnly` ile sıkıştırmayı dener ve sonuç küçülmediyse dilimi olduğu gibi gönderir. Rastgele veya zaten sıkıştırılmış (ZIP, MP4, JPEG) dosyalarda böylece hat üzerinde hiçbir şey büyümez; metin/kod/log dosyalarında ise 790+ MB/s hızında anlık sıkıştırma uygulanır.

### 2.2 Güvenlik ve Doğrulama (`internal/transfer` & `internal/safetext`)

| Benchmark | Yineleme Sayısı | Süre (ns/op) | Bellek / İşlem (B/op) | Tahsis (allocs/op) |
| :--- | :---: | :---: | :---: | :---: |
| `BenchmarkValidDigest-16` | 47,440,627 | **25.57 ns/op** | **0 B/op** | **0 allocs/op** |
| `BenchmarkSafeJoin-16` | 1,889,763 | **800.40 ns/op** | 240 B/op | 5 allocs/op |
| `BenchmarkClean_CleanText-16` | 15,280,320 | **78.40 ns/op** | **0 B/op** | **0 allocs/op** |
| `BenchmarkClean_UnsafeText-16` | 3,110,400 | **385.10 ns/op** | 64 B/op | 2 allocs/op |

* **Yorum:** Dosya transferi sırasında gelen hash doğrulaması ve güvenli metin temizleme işlemleri neredeyse sıfır gecikme (25-80 nanosaniye) ile çalışır ve GC (Garbage Collection) üzerinde yük oluşturmaz.

---

## 3. Ağ Profilleri ve Uçtan Uca Aktarım Senaryoları

PureSend bağlantıyı 3 farklı ağ topolojisi üzerinden kurabilir. Her senaryonun performans karakteristiği aşağıda özetlenmiştir:

```
[LAN Direct Transfer]   ──► Hat Doygunluğu (~112 MB/s on Gigabit, ~280 MB/s on 2.5G)
[WAN DCUtR Hole Punch]  ──► İki Uç Noktanın İnternet Hız Sınırı (Sıfır Sunucu Maliyeti)
[Relay Fallback]        ──► Güvenli Köprü (Kota ve Hız Sınırlı Yedek Hat)
```

### 3.1 Senaryo A: Yerel Ağ (Direct LAN Transfer)
* **Bağlantı Türü:** Aynı Wi-Fi veya Ethernet ağı üzerinden doğrudan TCP/QUIC.
* **El Sıkışma Süresi:** < 100 ms.
* **Aktarım Hızı:** Ağ arabirimi limitinde (Gigabit ağlarda **~112 - 118 MB/s**, 2.5G ağlarda **~280 MB/s**).
* **Sunucu Yükü:** Sıfır (Yalnızca oda kodu el sıkışması için ~1 KB sinyal trafiği).

### 3.2 Senaryo B: Farklı Ağlar - Doğrudan P2P (DCUtR NAT Hole Punching)
* **Bağlantı Türü:** NAT arkasındaki iki cihaz arasında delik açılarak kurulan doğrudan bağlantı.
* **El Sıkışma Süresi:** ~1.2 saniye - 2.8 saniye (STUN tespiti + DCUtR senkronizasyonu).
* **Aktarım Hızı:** Göndericinin yükleme (upload) veya alıcının indirme (download) hızının en düşüğü.
* **Sunucu Yükü:** Sıfır (Delik açıldıktan sonra sunucu bağlantıdan tamamen çıkar).

### 3.3 Senaryo C: Kısıtlı Ağlar - Röle Yedeği (Circuit Relay v2)
* **Bağlantı Türü:** Simetrik kurumsal güvenlik duvarı veya CGNAT nedeniyle delik açılamadığında sunucu üzerinden köprüleme.
* **Kullanım Amacı:** Kesintisiz aktarım güvencesi.
* **Hız Profili:** Sunucu bant genişliğine ve hız sınırlamalarına bağlıdır (Önerilen: 2-5 MB/s sınırlandırması).

---

## 4. Bellek ve Kaynak Tüketim Analizi

```
Bellek (RAM)
  ▲
  │     Geleneksel Araçlar (Tüm dosyayı belleğe alanlar)
  │    /
  │   /  ◄── Bellek dosya boyutuyla doğru orantılı artar (O(N))
  │  /
  │ ──────────────────────────────────────  PureSend (O(1) Streaming)
  │                                         Sabit ~35-45 MB RAM
  └──────────────────────────────────────────────────────────► Dosya Boyutu
       100 MB       1 GB       10 GB       50 GB
```

* **Chunk Boyutu:** Sabit `32 KB` (`chunkSize = 32 * 1024`).
* **Akış Prensibi:** Dosyalar diske veya ağa blok blok aktarılır. Dosyanın tamamı asla bellekte tutulmaz.
* **Kaynak Yöneticisi:** Sunucu tarafında `libp2p/p2p/host/resource-manager` aktif olup, bağlantı başına maksimum kaynak sınırları (FD ve RAM) tanımlıdır.

---

## 5. Benchmark'ları Yeniden Çalıştırma Rehberi

Bu testleri kendi makinenizde çalıştırmak ve doğrulamak için aşağıdaki komutları kullanabilirsiniz:

```bash
# 1. Çekirdek transfer ve sıkıştırma benchmark'larını çalıştırın
go test -bench=. -benchmem ./internal/transfer

# 2. Güvenlik ve metin sanitizasyon benchmark'larını çalıştırın
go test -bench=. -benchmem ./internal/safetext

# 3. Yalnızca belirli bir testi detaylı çalıştırma (örneğin sıkıştırma)
go test -bench=BenchmarkCompressChunk -benchtime=5s -benchmem ./internal/transfer
```

