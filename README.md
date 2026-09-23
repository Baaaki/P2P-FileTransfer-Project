# PureSend 📦 · [English](README.en.md)

> **Go (Golang) ile geliştirilmiş, uçtan uca şifreli ve doğrudan eşler arası (P2P) dosya transfer sistemi.**  
> Bulut depolamaya ve üyeliğe gerek yok. Ev modemi (NAT) arkasındaki iki cihaz mümkün olduğunda doğrudan bağlanır; delik açılamayan ağlarda aktarım yine şifreli olarak bir röle üzerinden sürer.

[![CI Pipeline](https://github.com/Baaaki/PureSend/actions/workflows/ci.yml/badge.svg)](https://github.com/Baaaki/PureSend/actions)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Baaaki/PureSend)](https://go.dev/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/Baaaki/PureSend)](https://github.com/Baaaki/PureSend/releases/latest)

---

## 🎯 Öne Çıkan Özellikler

* 🔒 **Sunucuya Güvenmek Gerekmez:** Kodun gizli kelimeleri sunucuya hiç gitmez; **PAKE** el sıkışması sayesinde sunucu veriyi göremez ve kelimeleri tahmin etmeden araya giremez.
* ⚡ **Akıllı NAT Delme (P2P):** **libp2p (DCUtR)** ile port açmadan doğrudan cihazdan cihaza aktarım (gerekirse Relay v2 yedeği).
* 🔄 **Kesintisiz Devam (Resume):** Kopan transferler kaldığı bayttan devam eder; her dosya bitince SHA-256 ile doğrulanır.
* 💻 **TUI & CLI Desteği:** Etkileşimli çift dilli terminal arayüzü (`Bubble Tea`) veya otomasyon için bayraklar (`-send`, `-receive`).

---

## 🛠️ Teknoloji Yığını (Tech Stack)

| Alan | Teknolojiler |
| :--- | :--- |
| **Programlama Dili** | Go (Golang 1.27) — `CGO_ENABLED=0` (tamamen bağımsız statik ikili dosyalar) |
| **Ağ & Eşler Arası (P2P)** | libp2p (v0.50), WebSockets, TLS, DCUtR (Hole Punching), Circuit Relay v2, STUN (pion/stun v3.1.7), UPnP |
| **Güvenlik & Kriptografi** | PAKE (`schollz/pake` v3, SPAKE2 tarzı, P-256), Noise / TLS 1.3, dosya başına SHA-256, minisign imzalı sürümler, Govulncheck |
| **Kullanıcı Arayüzü** | Charmbracelet Bubble Tea (v2.0 - Elm Mimarisi), Lip Gloss (v2.0) |
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
       │◄══════════ 3. NAT Delme & PAKE ile Karşılıklı Doğrulama ═══════════►│
       │                                                                     │
       │══════════ 4. Dosyalar Şifreli Bağlantıdan Akar (SHA-256) ═══════════►│
```

1. **Buluşma (Discovery):** Gönderici sunucudan bir oda numarası alır (`42`) ve önüne kendi seçtiği iki gizli kelimeyi koyar: `kiraz-liman-42`. Sunucu yalnızca numarayı bilir.
2. **Doğrudan Bağlantı:** İlk bağlantı sunucunun rölesi üzerinden kurulur; **DCUtR** bunu doğrudan bir bağlantıyla değiştirmeye çalışır. Simetrik NAT gibi delik açılamayan durumlarda aktarım röle üzerinden (uçtan uca şifreli, boyut ve süre sınırlı) devam eder.
3. **Kimlik Doğrulama:** Eşler kodun tamamını ortak parola olarak kullanıp bir **PAKE** el sıkışmasıyla birbirlerini doğrular; iki tarafın peer ID'si de anahtara bağlanır, bu yüzden kelimeleri bilmeyen sunucu araya giremez. Yanlış kodla 3 denemeden sonra oda kapanır.
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

## ⚡ Performans

Aşağıdaki sayılar ölçüldü; yöntem ve tek tek bütün çalıştırmalar [docs/BENCHMARK.md](docs/BENCHMARK.md) içinde. Donanım: AMD Ryzen 7 5700X, NVMe disk, Linux 6.8, Go 1.27.

| Ölçüm | Sonuç | Nasıl |
| :--- | :---: | :--- |
| **Uçtan uca aktarım hızı** | **360–390 MB/s** | Gerçek sunucu + gönderici + alıcı süreçleri, loopback üzerinde; 2–8 GiB rastgele veri, şifreli libp2p bağlantısı, iki uçta SHA-256, diske yazma (`make bench-e2e`) |
| **Tepe bellek (RSS)** | **35–41 MB** | Aynı ölçümde; 256 MiB ile 8 GiB arasında dosya boyutuyla değişmiyor |
| **El sıkışma (CPU)** | **~0,6 ms** | İki tarafın PAKE ve onay adımları birlikte (`BenchmarkHandshake`); gerçek bir bağlantıda ağ gidiş-dönüşleri baskındır |
| **Sıkıştırma** | **~800 MB/s** | Metin benzeri 32 KB dilim, DEFLATE `HuffmanOnly`; küçülmeyen dilim olduğu gibi gönderilir |

**Ne anlama geliyor:** Loopback'in kendi hat hızı yoktur, bu yüzden bu ölçüm yazılımın koyduğu tavanı gösterir. 390 MB/s, gigabit Ethernet'in (~118 MB/s) yaklaşık üç katıdır, yani bu donanımda yerel ağdaki darboğaz PureSend değil ağın kendisidir. Daha yavaş bir işlemci ya da diskte tavan düşer. İnternet üzerinden hız iki ucun bağlantısıyla, röle yedeğinde ise sunucunun koyduğu sınırlarla belirlenir. Farklı gerçek ağ çiftlerinde delik açma başarı oranı henüz ölçülmedi.

---

## 📂 Proje Dizin Mimarisi

```
├── cmd/
│   ├── client/          # Terminal istemcisi (TUI + Headless CLI giriş noktası)
│   └── server/          # Rendezvous & Circuit Relay v2 buluşma sunucusu
├── internal/
│   ├── p2p/             # libp2p host yönetimi, çoklu sunucu ve dinamik liste senkronizasyonu
│   ├── rendezvous/      # Oda numarası (nameplate) protokolü ve kod kelime listesi
│   ├── headless/        # Betikler için terminal arayüzsüz gönderme / alma
│   ├── transfer/        # PAKE el sıkışması, dilim akışı ve kaldığı yerden devam
│   ├── tui/             # Bubble Tea bileşenleri, modeller, formatlayıcılar ve tuş haritaları
│   ├── i18n/            # İşletim sistemi yerel dil algılama ve çift dil (TR/EN) sözlüğü
│   ├── update/          # GitHub API üzerinden çalışan in-place ikili dosya güncelleme motoru
│   └── safetext/        # Terminal escape dizisi ve RTLO karakter güvenlik filtresi
├── packaging/           # Arch Linux PKGBUILD, .desktop başlatıcı ve uygulama simgeleri
├── scripts/             # .deb paketleme ve uçtan uca benchmark betikleri
└── test/relay/          # İzole network namespace'lerde röle yedeği testi
```

---

## 🧪 Testler ve Kod Kalitesi

CI her commit'te şunları çalıştırır: `gofmt` denetimi, `go vet`, race detector ile tüm testler, golangci-lint, govulncheck, sunucu Docker imajının sağlık kontrolü ve iki eşin birbirini hiç göremediği izole network namespace'lerde röle yedeği testi. Test kapsamı, entegrasyon testlerinin çalıştırdığı derlenmiş client binary'si dahil ~%77'dir.

```bash
make test        # go vet + race detector ile tüm testler
make cover       # birleşik kapsama raporu (binary dahil)
make lint        # golangci-lint, CI ile aynı sürüm
make vuln        # erişilebilir bilinen açıklar (govulncheck)
make test-relay  # röle yedeği, izole ağlarda (root gerekmez)
make bench-e2e   # uçtan uca hız ve bellek ölçümü
```

---

## 📄 Lisans & İletişim

Bu proje [GNU General Public License v3.0](LICENSE) ile lisanslanmıştır.

* **Web:** [https://puresend.madebybaki.com](https://puresend.madebybaki.com)
* **İletişim:** [contact@madebybaki.com](mailto:contact@madebybaki.com)
* **Güvenlik Politikası:** [SECURITY.md](.github/SECURITY.md)
* **Katkı Yönergeleri:** [CONTRIBUTING.md](.github/CONTRIBUTING.md)
