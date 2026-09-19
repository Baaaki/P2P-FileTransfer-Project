# PureSend 📦

> **Uçtan uca şifreli, sunucusuz doğrudan P2P dosya ve klasör transfer aracı.**  
> İki cihaz arasında aracı sunucuya dosya yüklemeden, ev modemleri (NAT) arkasında olsalar dahi güvenle dosya aktarın.

[![CI Status](https://github.com/Baaaki/PureSend/actions/workflows/ci.yml/badge.svg)](https://github.com/Baaaki/PureSend/actions)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Baaaki/PureSend)](https://go.dev/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/Baaaki/PureSend)](https://github.com/Baaaki/PureSend/releases/latest)

🇬🇧 **[English Documentation](README.en.md)** | 🌐 **[Web Sitesi & Canlı Demo](https://puresend.madebybaki.com)** | 📋 **[Yol Haritası](docs/ROADMAP.md)** | 🛡️ **[Güvenlik Politikası](docs/SECURITY.md)**

---

## ✨ Öne Çıkan Özellikler

- 🚀 **Doğrudan P2P Aktarım:** Dosyalarınız hiçbir bulut veya aracı sunucuya kaydedilmez; doğrudan iki bilgisayar arasında akar.
- 🔑 **Sıfır Ayar, 3 Kelimelik Kod:** Port yönlendirme (port forwarding), IP yapılandırması veya hesap açma yok. Ekranda çıkan kodu arkadaşınıza iletin, yeter.
- 🛡️ **Kriptografik Güvenlik (PAKE/SPAKE2):** Oda kodu aynı zamanda parola işlevi görür. Buluşma sunucusu dosyalarınızı veya parolanızı göremez, araya giremez (Zero-Trust).
- ⚡ **NAT Delme & Akıllı Yedek (Hole Punching):** DCUtR ve UPnP ile ev ağları arasında doğrudan tünel açılır; delik açılamazsa sınırlı relay devreye girer.
- 📂 **Klasör ve Çoklu Dosya:** Klasör hiyerarşisi bozulmadan tek seferde binlerce dosya aktarımı.
- 🔄 **Kaldığı Yerden Devam (Resume):** Bağlantı kopsa bile aktarılan dosyalar tekrar indirilmez; yarım kalan dosya `.part` üzerinden devam eder.
- 💻 **TUI & Headless CLI:** Hem kullanımı keyifli Terminal Arayüzü (Bubble Tea) hem de betikler ve CI için CLI bayrakları (`-send`, `-receive`).
- 🐧 **Masaüstü Entegrasyonu:** Debian/Ubuntu için `.deb` paketi, sistem başlatıcı (`.desktop`), SVG simgesi ve Linux'ta çift tıklamayla otomatik terminal açma.

---

## 🚀 Hızlı Başlangıç

> [!TIP]
> **🟢 Canlı ve Kullanıma Hazır:**  
> PureSend şu an **canlı olarak çalışmaktadır**! Herhangi bir sunucu kurmanıza, port yönlendirmenize veya ayar yapmanıza gerek yoktur. Ortak ve resmi buluşturucu sunucumuz (`rendezvous.madebybaki.com`) 7/24 devrededir. Doğrudan aşağıdaki linklerden işletim sisteminize uygun olanı indirip hemen kullanmaya başlayabilirsiniz.

### İndir ve Çalıştır

Tüm derlenmiş ikililer doğrudan deponun [`bin/`](bin/) klasöründe hazırdır. Aşağıdaki linklere tıklayarak doğrudan indirebilirsiniz:

| Platform | İndirme Bağlantısı (`bin/`) | Kurulum & Çalıştırma |
|---|---|---|
| **Ubuntu / Debian / Mint** | [📥 `puresend_0.2.0_amd64.deb`](https://github.com/Baaaki/PureSend/raw/main/bin/puresend_0.2.0_amd64.deb) | `sudo apt install ./puresend_0.2.0_amd64.deb` *(Menüye eklenir)* |
| **Windows** | [📥 `puresend.exe`](https://github.com/Baaaki/PureSend/raw/main/bin/puresend.exe) | İndir ve çift tıkla |
| **Linux (Taşınabilir)** | [📥 `puresend`](https://github.com/Baaaki/PureSend/raw/main/bin/puresend) | `chmod +x puresend && ./puresend` *(Çift tıkla da çalışır)* |
| **macOS (Apple Silicon)** | [📥 `puresend_mac_arm64`](https://github.com/Baaaki/PureSend/raw/main/bin/puresend_mac_arm64) | `chmod +x puresend_mac_arm64 && ./puresend_mac_arm64` |
| **macOS (Intel)** | [📥 `puresend_mac_amd64`](https://github.com/Baaaki/PureSend/raw/main/bin/puresend_mac_amd64) | `chmod +x puresend_mac_amd64 && ./puresend_mac_amd64` |

> 📦 Alternatif olarak arşiv paketlerine ve kaynak kodlara [**GitHub Releases**](https://github.com/Baaaki/PureSend/releases/latest) sayfasından da ulaşabilirsiniz.

#### İlk Açılış Uyarısı Hakkında
Uygulama ikilileri açık kaynak olarak derlendiği ve ücretli imzalama sertifikası taşımadığı için işletim sisteminiz ilk açılışta izin isteyebilir:
- **macOS:** Dosyaya **Sağ tıkla → Aç** deyin (veya terminalde `xattr -d com.apple.quarantine puresend`).
- **Windows:** *"Windows kişisel bilgisayarınızı korudu"* ekranında **Ek bilgi → Yine de çalıştır**'a tıklayın.
- **Linux:** Gerekirse `chmod +x puresend` ile çalıştırma yetkisi verin.

---

## 💡 Nasıl Kullanılır?

### 1. Terminal Arayüzü (TUI) ile

Uygulamayı açmanız yeterlidir:
```bash
puresend
```
* **Gönderen:** *"Dosya göndereceğim"* seçin → Dosyaları/klasörleri seçin (`Enter` ile seç, `f` ile tüm klasör, `Backspace` ile üst klasör) → `s` ile başlatın → Ekranda beliren 3 kelimelik kodu (`kiraz-liman-42`) arkadaşınıza iletin.
* **Alan:** *"Bana dosya gönderilecek"* seçin → 3 kelimelik kodu yazın → Onaylayın. Dosyalar `İndirilenler/PureSend` dizinine iner (indirme klasörünü arayüzden veya `-out` ile değiştirebilirsiniz).

### 2. Başsız (Headless) CLI Modu

Grafik arayüzü olmayan sunucular veya betik otomasyonları için:

```bash
# Dosya veya klasör gönder (kodu stdout'a yazar ve alıcıyı bekler)
puresend -send tatil/

# Kodu alıp doğrudan belirtilen klasöre indir
puresend -receive kiraz-liman-42 -out /mnt/depo -yes

# Sürüm bilgisi
puresend -version
```

---

## 🔍 Nasıl Çalışıyor?

```
Gönderen (İstanbul)              Buluşma Sunucusu (Rendezvous)             Alıcı (İzmir)
      │                                       │                                  │
      │ 1. "kiraz-liman-42" odasını aç        │                                  │
      │──────────────────────────────────────►│◄─────────────────────────────────│ 2. "kiraz-liman-42" kimde?
      │                                       │                                  │
      │◄═══════════ 3. SPAKE2 El Sıkışması & NAT Delme (Hole Punching) ═════════►│
      │                                                                          │
      │════════════ 4. Dosyalar DOĞRUDAN P2P Olarak Akar (SHA-256) ══════════════►│
```

1. **Buluşma:** Gönderen rastgele bir oda kodunu sunucuya kaydeder; alıcı aynı kodu arar.
2. **Kimlik Doğrulama (PAKE):** İki taraf, oda kodundan **SPAKE2** ile ortak bir anahtar türetir ve Peer ID'lerini bağlar. Kod sunucuya asla iletilmez; sunucu yalan söylese bile transfer başlamaz.
3. **NAT Delme:** İlk temas sunucunun **Circuit Relay v2** köprüsünden geçer; **DCUtR** bunu doğrudan eşler arası P2P bağlantıya yükseltir.
4. **Güvenli Aktarım:** Dosyalar blok blok, dosya başına **SHA-256 sağlama doğrulamasıyla** doğrudan akar. Transfer koparsa kaldığı yerden devam eder.

---

## 🛠️ Kendi Sunucunu Barındırma (Self-Hosting)

Resmi istemciler gömülü topluluk sunucusuna bağlanır. Ancak kendi buluşma sunucunuzu çalıştırmak isterseniz:

```bash
PUBLIC_HOST=rendezvous.example.com docker compose up -d
```

> Ayrıntılı Cloudflare Tunnel entegrasyonu, Prometheus metrikleri, rate limiting ve operasyon adımları için:  
> 👉 **[Sunucu Kurulum ve Dağıtım Rehberi (docs/DEPLOYMENT.md)](docs/DEPLOYMENT.md)**

---

## 💻 Geliştirme

```bash
make            # Tüm make hedeflerini listeler
make build      # İstemci ve sunucuyu bin/ altına derler
make test       # Yarış durumu (race detector) açık testleri koşar
make test-relay # Yalıtılmış ağ isim uzaylarında relay fallback testini koşar
make lint       # golangci-lint analizini çalıştırır
make vuln       # govulncheck zafiyet taramasını çalıştırır
make deb        # Debian/Ubuntu için .deb paketi üretir
```

Yerel test için 3 ayrı terminalde:
```bash
# 1. Sunucu
go run ./cmd/server -ws-port 8080

# 2. Gönderen ve 3. Alıcı (Peer ID'yi sunucu çıktısından alın)
go run ./cmd/client -server /ip4/127.0.0.1/tcp/8080/ws/p2p/<PeerID>
```

---

## 📂 Proje Mimarisi

| Klasör / Bileşen | Açıklama |
|---|---|
| **`cmd/client/`** | Masaüstü istemcisi (TUI, headless CLI, otomatik terminal başlatma) |
| **`cmd/server/`** | Rendezvous ve Circuit Relay v2 sunucusu |
| **`internal/p2p/`** | libp2p host yönetimi, çoklu sunucu ve dinamik `server.txt` desteği |
| **`internal/transfer/`** | Dosya aktarımı, SPAKE2 kimlik doğrulama ve resume motoru |
| **`internal/tui/`** | Bubble Tea & Lipgloss tabanlı terminal arayüzü |
| **`internal/safetext/`** | Terminal kaçış dizileri ve bidi/RTLO güvenlik filtresi |
| **`LandingPage/`** | React 19 + Vite + Tailwind v4 tanıtım sayfası ve TUI simülatörü |
| **`packaging/`** | `.desktop` başlatıcı, SVG simge ve `.deb` paket tanımları |
| **`deploy/`** | Cloudflare Tunnel yapılandırma şablonları |

---

## 📚 Dokümantasyon

- 🛡️ [Güvenlik Modeli ve Politikası](docs/SECURITY.md)
- 🚀 [Sunucu Kurulum ve Dağıtım Rehberi](docs/DEPLOYMENT.md)
- 📋 [Ürünleşme Yol Haritası](docs/ROADMAP.md)
- 📝 [Değişiklik Günlüğü (Changelog)](docs/CHANGELOG.md)
- 🤝 [Katkıda Bulunma Rehberi](docs/CONTRIBUTING.md)

---

## 📄 Lisans

Bu proje [GNU General Public License v3.0](LICENSE) altında lisanslanmıştır.
