# PureSend 📦 · [English](README.en.md)

> **Go (Golang) ile geliştirilmiş, uçtan uca şifreli ve doğrudan eşler arası (P2P) dosya transfer sistemi.**  
> Bulut sağlayıcılarına, üyeliklere veya üçüncü taraf sunuculara ihtiyaç duymadan; ev modemleri (NAT) ve kurumsal güvenlik duvarları arkasındaki cihazlar arasında doğrudan veri akışı sağlar.

[![CI Pipeline](https://github.com/Baaaki/PureSend/actions/workflows/ci.yml/badge.svg)](https://github.com/Baaaki/PureSend/actions)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Baaaki/PureSend)](https://go.dev/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/Baaaki/PureSend)](https://github.com/Baaaki/PureSend/releases/latest)

---

## 🎯 Öne Çıkan Özellikler

* 🔒 **Sunucuya Güvenmek Gerekmez:** Kodun gizli kelimeleri sunucuya hiç gitmez; **SPAKE2** el sıkışması sayesinde sunucu veriyi göremez ve araya giremez.
* ⚡ **Akıllı NAT Delme (P2P):** **libp2p (DCUtR)** ile port açmadan doğrudan cihazdan cihaza aktarım (gerekirse Relay v2 yedeği).
* 🔄 **Kesintisiz Devam (Resume):** Kopan transferler kaldığı bayttan devam eder; her dosya bitince SHA-256 ile doğrulanır.
* 💻 **TUI & CLI Desteği:** Etkileşimli çift dilli terminal arayüzü (`Bubble Tea`) veya otomasyon için bayraklar (`-send`, `-receive`).

---

## 🛠️ Teknoloji Yığını (Tech Stack)

| Alan | Teknolojiler |
| :--- | :--- |
| **Programlama Dili** | Go (Golang 1.27) — `CGO_ENABLED=0` (tamamen bağımsız statik ikili dosyalar) |
| **Ağ & Eşler Arası (P2P)** | libp2p (v0.49), WebSockets, TLS, DCUtR (Hole Punching), Circuit Relay v2, STUN (pion/stun v3.1.7), UPnP |
| **Güvenlik & Kriptografi** | SPAKE2 (PAKE / pake v3), Noise / TLS 1.3, dosya başına SHA-256, minisign imzalı sürümler, Govulncheck |
| **Kullanıcı Arayüzü** | Charmbracelet Bubble Tea (v1.3 - Elm Mimarisi), Lipgloss (v1.1) |
| **Dağıtım & DevOps** | GoReleaser (v2), GitHub Actions CI/CD, Debian (`.deb`), Arch Linux (`PKGBUILD`), Tek Satır Kurulumcu (`sh`/`ps1`) |

---

## ⚡ Hızlı Kurulum

İşletim sisteminize uygun tek satırlık komutu terminalde çalıştırarak anında kurabilir ve güncelleyebilirsiniz:

```bash
# Linux & macOS (Arch, Ubuntu, Fedora, Debian, macOS vb.)
curl -fsSL https://raw.githubusercontent.com/Baaaki/PureSend/main/install.sh | sh

# Windows (PowerShell)
irm https://raw.githubusercontent.com/Baaaki/PureSend/main/install.ps1 | iex
```

> **Klasik İndirme:** Kurulum yapmadan taşınabilir (portable) çalıştırmak için [GitHub Releases](https://github.com/Baaaki/PureSend/releases/latest) veya [Web Sitemizden](https://puresend.madebybaki.com/#indir) doğrudan `.exe`, `mac_arm64` veya `.deb` dosyalarını indirebilirsiniz.

---

## 🧩 Nasıl Çalışır? (Protokol Akışı)

```
Gönderici (İstemci A)           Buluşma Sunucusu (Rendezvous)         Alıcı (İstemci B)
       │                                     │                               │
       │ 1. Oda aç → sunucu "42" verir       │                               │
       │────────────────────────────────────►│◄──────────────────────────────│ 2. Yalnızca "42" numarasını sor
       │                                     │                               │
       │◄═══════════ 3. SPAKE2 Kriptografik Doğrulama & NAT Delme ══════════►│
       │                                                                     │
       │════════════ 4. Dosyalar DOĞRUDAN P2P Olarak Akar (SHA-256) ═════════►│
```

1. **Buluşma (Discovery):** Gönderici sunucudan bir oda numarası alır (`42`) ve önüne kendi seçtiği iki gizli kelimeyi koyar: `kiraz-liman-42`. Sunucu yalnızca numarayı bilir.
2. **Kimlik Doğrulama:** Eşler kodun tamamını ortak parola olarak kullanıp **SPAKE2** ile birbirlerini doğrular; kelimeleri bilmeyen sunucu araya giremez. Yanlış kodla 3 denemeden sonra oda kapanır.
3. **Doğrudan Bağlantı:** **DCUtR** koordinasyonu ile her iki tarafın NAT cihazı delinir ve eşler doğrudan birbirine bağlanır.
4. **Doğrulanmış Aktarım:** Dosyalar şifreli bağlantı üzerinden dilim dilim iletilir; her dosya bitince SHA-256 ile doğrulanır.

---

## 💻 Kullanım

### 1. Etkileşimli Terminal Arayüzü (TUI)
```bash
puresend
```
* **Gönder:** Dosya veya klasörleri seçin $\rightarrow$ Ekranda çıkan kodu (örn. `kiraz-liman-42`) alıcıya verin.
* **Al:** Kodu girin $\rightarrow$ Aktarımı onaylayın (dosyalar otomatik olarak `İndirilenler/PureSend` dizinine kaydedilir).
* **Dil:** `[L]` tuşuna basarak anında Türkçe / İngilizce arasında geçiş yapın.

### 2. Otomasyon ve Betikler İçin CLI Modu
```bash
# Belirtilen dizini arka planda gönder
puresend -send ./belgeler/

# Kodu doğrudan hedef dizine indir ve onay istemeden tamamla
puresend -receive kiraz-liman-42 -out /var/backups -yes

# En son sürüme güncelle
puresend -update
```

---

## ⚡ Performans ve Sistem Verimliliği (Performance & Efficiency)

PureSend, üçüncü taraf bulut sağlayıcılarının yapay hız ve dosya boyutu kısıtlamalarını ortadan kaldırır. Akış tabanlı mimarisi sayesinde fiziksel hat kapasitesinin tamamını kullanır:

| Metrik / Alan | Başarım & Karakteristik | Teknik Detay |
| :--- | :---: | :--- |
| **Bellek Tüketimi (RAM)** | **Sabit ~35–45 MB ($O(1)$)** | 32 KB blok akışı (chunking); dosya 100 MB da olsa 50 GB da olsa RAM şişmez. |
| **Dinamik Sıkıştırma** | **~800 MB/s** | DEFLATE (`flate.HuffmanOnly`) ile metin ve kod arşivlerinde hat hızının üzerinde aktarım; küçülmeyen veri olduğu gibi gönderilir. |
| **Sıfır Tahsisli Doğrulama** | **25 ns / 0 allocs** | Gelen SHA-256 özetlerinin biçim denetimi sıfır ek bellek tahsisiyle çalışır. |
| **LAN Aktarım Hızı** | **Hat Doygunluğu (Line-Rate)** | Gigabit ağlarda **~112 MB/s**, 2.5G ağlarda **~280 MB/s** fiziksel sınır. |
| **WAN (İnternet) Aktarımı** | **%100 Bant Genişliği** | DCUtR delik açma ile sunucusuz P2P; hız yalnızca iki ucun internet kapasitesiyle sınırlıdır. |
| **Kriptografik El Sıkışma** | **< 5 ms** | SPAKE2 (P-256) sıfır-bilgi anahtar değişimi anında tamamlanır. |

> 📊 Detaylı mikro-benchmark çıktıları, bellek profilleri ve test adımları için: **[Performans ve Benchmark Rehberi (docs/BENCHMARK.md)](docs/BENCHMARK.md)**

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

Bu proje [GNU General Public License v3.0](LICENSE) ile lisanslanmıştır.

* **Web:** [https://puresend.madebybaki.com](https://puresend.madebybaki.com)
* **İletişim:** [contact@madebybaki.com](mailto:contact@madebybaki.com)
* **Güvenlik Politikası:** [SECURITY.md](.github/SECURITY.md)
* **Katkı Yönergeleri:** [CONTRIBUTING.md](.github/CONTRIBUTING.md)
