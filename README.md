# PureSend 📦

> **Go (Golang) ile geliştirilmiş, uçtan uca şifreli (Zero-Trust) ve doğrudan eşler arası (P2P) dosya transfer sistemi.**  
> Bulut sağlayıcılarına, üyeliklere veya üçüncü taraf sunuculara ihtiyaç duymadan; ev modemleri (NAT) ve kurumsal güvenlik duvarları arkasındaki cihazlar arasında doğrudan veri akışı sağlar.

[![CI Pipeline](https://github.com/Baaaki/PureSend/actions/workflows/ci.yml/badge.svg)](https://github.com/Baaaki/PureSend/actions)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Baaaki/PureSend)](https://go.dev/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/Baaaki/PureSend)](https://github.com/Baaaki/PureSend/releases/latest)

🌐 **[Canlı Web Sitesi](https://puresend.madebybaki.com)** | 🇬🇧 **[English Documentation (README.en.md)](README.en.md)** | 🛡️ **[Güvenlik Politikası](docs/SECURITY.md)** | ✉️ **[İletişim](mailto:contact@madebybaki.com)**

---

## 🎯 Projenin Amacı ve Öne Çıkanlar

PureSend, büyük dosyaların ve dizin ağaçlarının aracı sunucularda depolanmadan, gizlilikten ödün verilmeden ve karmaşık ağ ayarları (port forwarding, sabit IP vb.) gerektirmeden iletilmesi amacıyla tasarlanmış modern bir sistem aracıdır.

* **Sıfır Güven (Zero-Trust) Kriptografi:** Parola tabanlı anahtar değişimi (**SPAKE2 / PAKE**) kullanır. Buluşma sunucusu dosyaları veya parolayı göremez, trafiği dinleyemez.
* **Akıllı NAT Delme (Hole Punching):** **libp2p**, **DCUtR** ve **UPnP** protokolleriyle modemler arasında doğrudan şifreli tünel açar; doğrudan tünelin açılamadığı katı simetrik ağlarda şifreli geçiş köprüsüne (bounded relay) güvenle yedeklenir.
* **Kaldığı Yerden Devam (Resume Engine):** Ağ kopmalarında aktarılan dosyalar baştan indirilmez; `.part` geçici dosyaları üzerinden blok seviyesinde SHA-256 sağlama doğrulamasıyla devam eder.
* **Reaktif Terminal Arayüzü (TUI):** **Bubble Tea** (Elm Mimarisi) ile geliştirilmiş, çift dilli (TR/EN, işletim sistemi dilini otomatik algılama ve çalışma anında `L` tuşuyla geçiş) modern terminal deneyimi.
* **Headless / CI/CD Desteği:** Terminali olmayan sunucular veya betikler için doğrudan komut satırı bayrakları (`-send`, `-receive`, `-yes`).

---

## 🛠️ Teknoloji Yığını (Tech Stack)

| Alan | Teknolojiler |
| :--- | :--- |
| **Programlama Dili** | Go (Golang 1.24) — `CGO_ENABLED=0` (tamamen bağımsız statik ikili dosyalar) |
| **Ağ & Eşler Arası (P2P)** | libp2p, WebSockets, TLS, DCUtR (Hole Punching), Circuit Relay v2, STUN/UPnP |
| **Güvenlik & Kriptografi** | SPAKE2 (PAKE), AES-GCM, SHA-256 blok doğrulama, RTLO/bidi terminal temizleme |
| **Kullanıcı Arayüzü** | Charmbracelet Bubble Tea (Elm Architecture), Lipgloss |
| **Dağıtım & DevOps** | Docker, Docker Compose, GitHub Actions CI/CD, Debian (`.deb`), Arch Linux (`PKGBUILD`) |

---

## ⚡ Hızlı Kurulum

İşletim sisteminize uygun tek satırlık komutu terminalde çalıştırarak anında kurabilir ve güncelleyebilirsiniz:

```bash
# Linux & macOS (Bash) — Otomatik mimari tespiti ve PATH entegrasyonu
curl -fsSL https://puresend.madebybaki.com/install.sh | sh

# Windows (PowerShell) — Tek kopya, otomatik güncelleme ve PATH entegrasyonu
irm https://puresend.madebybaki.com/install.ps1 | iex

# Arch Linux (AUR)
yay -S puresend-bin
```

> **Klasik İndirme:** Kurulum yapmadan taşınabilir (portable) çalıştırmak için [GitHub Releases](https://github.com/Baaaki/PureSend/releases/latest) veya [Web Sitemizden](https://puresend.madebybaki.com/#indir) doğrudan `.exe`, `mac_arm64` veya `.deb` dosyalarını indirebilirsiniz.

---

## 🧩 Nasıl Çalışır? (Protokol Akışı)

```
Gönderici (İstemci A)           Buluşma Sunucusu (Rendezvous)         Alıcı (İstemci B)
       │                                     │                               │
       │ 1. Odayı aç ("kiraz-liman-42")      │                               │
       │────────────────────────────────────►│◄──────────────────────────────│ 2. Odayı sor ("kiraz-liman-42")
       │                                     │                               │
       │◄═══════════ 3. SPAKE2 Kriptografik Doğrulama & NAT Delme ══════════►│
       │                                                                     │
       │════════════ 4. Dosyalar DOĞRUDAN P2P Olarak Akar (SHA-256) ═════════►│
```

1. **Buluşma (Discovery):** Gönderici 3 kelimelik geçici bir oda kodu türeterek sunucuya sinyal bırakır.
2. **Kimlik Doğrulama:** Eşler, sunucuya güvenmeden oda kodunu ortak parola kullanarak **SPAKE2** ile şifreli tünel oluşturur.
3. **Doğrudan Bağlantı:** **DCUtR** koordinasyonu ile her iki tarafın NAT cihazı delinir ve eşler doğrudan birbirine bağlanır.
4. **Doğrulanmış Aktarım:** Dosyalar blok blok şifreli olarak karşı tarafa iletilir; iniş tamamlandığında SHA-256 ile teyit edilir.

---

## 💻 Kullanım

### 1. Etkileşimli Terminal Arayüzü (TUI)
```bash
puresend
```
* **Gönder:** Dosya veya klasörleri seçin $\rightarrow$ Ekranda çıkan 3 kelimelik kodu alıcıya verin.
* **Al:** 3 kelimelik kodu girin $\rightarrow$ Aktarımı onaylayın (dosyalar otomatik olarak `İndirilenler/PureSend` dizinine kaydedilir).
* **Dil:** `[L]` tuşuna basarak anında Türkçe / İngilizce arasında geçiş yapın.

### 2. Otomasyon ve Betikler İçin CLI Modu
```bash
# Belirtilen dizini arka planda gönder
puresend -send ./belgeler/

# Kodu doğrudan hedef dizine indir ve onay istemeden tamamla
puresend -receive kiraz-liman-42 -out /var/backups -yes

# En son sürüme güncelle
puresend update
```

---

## 📊 Gerçek Saha Test Sonuçları (Real-World Benchmarks)

PureSend, doğrudan P2P tünelleme ve yerel ISP rotalama/peering avantajları sayesinde bulut sağlayıcılarının yapay hız kısıtlamalarını ortadan kaldırır. Gerçek kullanıcı senaryolarında elde edilen test verileri:

| Güzergâh | Mesafe | Dosya Boyutu | Ortalama Hız | Süre | Durum / Not |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **İstanbul ➔ Tekirdağ** | ~140 km | **1.5 GB** (Video) | **~20 MB/sn** | **~1 dk** | **Tamamlandı** (Servis sağlayıcı upload paketi 48 Mbps olmasına rağmen doğrudan P2P tünelleme ile ~160–200 Mbps efektif hıza ulaşıldı) |
| **İstanbul ➔ İstanbul** (Farklı İlçeler) | ~35 km | *1.5 GB+* | *Ölçülüyor* | *—* | ⏳ *Test aşamasında (Yakında)* |
| **İstanbul ➔ İzmir** | ~480 km | *1.5 GB+* | *Ölçülüyor* | *—* | ⏳ *Test aşamasında (Yakında)* |

> 💡 **Not:** Servis sağlayıcıların standart bulut yüklemelerine uyguladığı yapay upload sınırları, doğrudan eşler arası (P2P) soket açıldığında yerel ISP rotalama ve peering avantajı sayesinde aşılabilmekte ve hat kapasitesinin tamamı kullanılabilmektedir.

---

## 📂 Proje Dizin Mimarisi

```
├── cmd/
│   ├── client/          # Terminal istemcisi (TUI + Headless CLI giriş noktası)
│   └── server/          # Rendezvous & Circuit Relay v2 buluşma sunucusu
├── internal/
│   ├── p2p/             # libp2p host yönetimi, çoklu sunucu ve dinamik liste senkronizasyonu
│   ├── transfer/        # SPAKE2 doğrulama motoru, chunk streaming ve resume mantığı
│   ├── tui/             # Bubble Tea bileşenleri, modeller, formatlayıcılar ve tuş haritaları
│   ├── i18n/            # İşletim sistemi yerel dil algılama ve çift dil (TR/EN) sözlüğü
│   ├── update/          # GitHub API üzerinden çalışan in-place ikili dosya güncelleme motoru
│   └── safetext/        # Terminal escape dizisi ve RTLO karakter güvenlik filtresi
├── packaging/           # Arch Linux PKGBUILD, .desktop başlatıcı ve uygulama simgeleri
└── scripts/             # Otomatik .deb paketleme ve derleme otomasyonları
```

---

## 🧪 Testler ve Kod Kalitesi

Proje, kurumsal Go standartlarına uygun olarak yüksek birim test kapsamı ve yarış durumu denetimiyle geliştirilmiştir:

```bash
# Yarış durumu (race detector) ile birim testleri çalıştır
make test
# veya
go test -v -race ./...

# Statik kod analizi (linter)
golangci-lint run

# Güvenlik açığı taraması
govulncheck ./...
```

---

## 📄 Lisans & İletişim

Bu proje [GNU General Public License v3.0](LICENSE) ile sunulmaktadır.

* **E-posta:** [contact@madebybaki.com](mailto:contact@madebybaki.com)
* **Web:** [https://puresend.madebybaki.com](https://puresend.madebybaki.com)
