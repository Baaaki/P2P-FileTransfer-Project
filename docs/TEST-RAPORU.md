# PureSend — Test Raporu ve Çalışma Mantığı

*7 Ağustos 2026 · commit `86a28ea` · Go 1.26.0, linux/amd64*

> **Not (8 Ağustos 2026):** Bu rapordaki bulguların ve §6'daki önerilerin
> tamamı uygulandı — kritik §5.1 dahil. Ne yapıldığı
> [CHANGELOG.md](CHANGELOG.md)'de; rapor olduğu gibi bırakıldı, çünkü
> değişikliklerin gerekçesi burada yazıyor. Aşağıdaki gözlemler artık
> **düzeltilmiş olan** davranışı anlatıyor.

---

## 1. Özet: çalışıyor mu?

**Evet, çalışıyor.** İki ayrı istemci ortak bir buluşma sunucusu üzerinden
birbirini buldu ve dosyayı **sunucuya uğratmadan** birbirine aktardı. Hem
doğrudan P2P yolu hem de delik açılamadığında devreye giren relay yedeği
uçtan uca doğrulandı; her transferde alınan dosyanın SHA-256 özeti
gönderilenle bire bir aynı çıktı.

| | Sonuç |
|---|---|
| `go build ./...` / `go vet ./...` | temiz |
| `go test ./...` | **36 test, 7 paket, hepsi geçti** |
| Doğrudan P2P transferi (64 MB) | ✅ doğrulandı |
| Relay yedeği ile transfer (64 MB) | ✅ doğrulandı |
| Docker + OpenShip tarzı deploy | ✅ doğrulandı |
| Kritik bulgu | ⚠️ **1 adet** — oda kodu tek kullanımlık değil (bkz. [§5.1](#51-kritik--oda-kodu-tek-kullanımlık-değil)) |

Bulunan sorunların hiçbiri "program çalışmıyor" cinsinden değil; hepsi
davranış/kullanıcı deneyimi/sertleştirme başlıkları altında.

---

## 2. Testi nasıl kurguladım

Senin gerçek senaryon **ev kasan (ev interneti) ↔ mobil veriyle bağlanan
laptop**, ortada da Cloudflare Tunnel arkasındaki bir relay sunucusu. Tek
makinede buna en çok benzeyen kurulumu iki ayrı topolojiyle kurdum:

### Topoloji A — "aynı ağdalar" (doğrudan P2P mümkün)

```
  ftclient (gönderen)  ─┐                     ┌─  ftclient (alan)
                        ├── 127.0.0.1 ────────┤
                        └── ftserver /ws ─────┘
```

Her istemciye **kendi `$HOME`'u** verildi, böylece dosya gezgini ve indirme
klasörü iki ayrı bilgisayardaki gibi birbirinden bağımsız oldu.

### Topoloji B — "ayrı ağdalar, delik açılamıyor" (relay yedeği zorunlu)

Bu asıl ilginç olan. `unshare -Urnm` ile ayrıcalıksız bir kullanıcı+ağ
namespace'i açıp içinde üç ağ kurdum. Köprüde **port isolation** açık:

```
  netns A (10.0.0.2)  ─┐                            ┌─  netns B (10.0.0.3)
   gönderen            ├──  br0 · 10.0.0.1  ────────┤       alan
                       │    ftserver + relay        │
                       └──── A ✗ B (izole) ─────────┘
```

Doğrulaması:

```
A -> server(10.0.0.1): REACHABLE
B -> server(10.0.0.1): REACHABLE
A -> B    (10.0.0.3): BLOCKED
B -> A    (10.0.0.2): BLOCKED
```

İki istemci birbirini **hiçbir şekilde** göremiyor, ikisi de sunucuya
ulaşabiliyor. Bu, "CGNAT arkasındaki mobil veri + simetrik NAT'lı ev
modemi" durumunun en sert hâli: delik açma matematiksel olarak imkânsız,
dolayısıyla relay yedeği devreye girmek **zorunda**.

### İstemciler nasıl sürüldü

`cmd/client` sadece bir TUI; başsız (headless) modu yok. Bu yüzden her iki
istemciyi de birer **pty** üzerinden gerçek tuş vuruşlarıyla sürdüm
(`pexpect`): ok tuşları, Enter, `s`, `y`. Ekrandan çıkan ANSI temizlenip
oda kodu ekrandan okundu ve karşı istemciye yazıldı — yani tam olarak
insanın yaptığı şey.

> İki tuzağa düştüm, not düşeyim ki sen tekrar düşme:
> **(1)** pty'yi sürekli okumazsan ~64 KB tampon dolar ve TUI kendi
> `write`'ında bloke olur — program transferin ortasında durur gibi görünür.
> **(2)** Bubble Tea açılışta terminale imleç konumu sorar (`ESC[6n`);
> cevap gelmezse **5 saniye** bekler. Gerçek terminalde bu 0,3 saniye.
> İlk ölçümlerimdeki "5 sn açılış" tamamen bu yüzdendi, üründe böyle bir
> gecikme yok.

---

## 3. Adım adım: tam olarak ne oluyor?

Aşağıdaki her adım gerçekten çalıştırılıp doğrulandı; süreler yukarıdaki
Topoloji A ölçümleri.

### 3.1. Sunucu açılışı — `cmd/server/main.go`

1. **Kimlik çözülür** (`loadOrCreateKey`, sırayla):
   `FT_IDENTITY_KEY` ortam değişkeni → `-key` dosyası → yeni Ed25519
   anahtar üretip dosyaya yaz. Ekrana `Identity from:` satırıyla hangisinin
   kullanıldığı basılır. **Bu anahtar sunucunun Peer ID'sini belirler ve
   Peer ID dağıttığın istemcilerin içine gömülüdür** — değişirse herkes
   kırılır.
2. **libp2p host'u kurulur:**
   - `ListenAddrStrings("/ip4/0.0.0.0/tcp/8080/ws")` — düz WebSocket.
     TLS **yok**, çünkü TLS'i Cloudflare sonlandırıyor.
   - `ForceReachabilityPublic()` — tünel arkasında AutoNAT probe'u
     başarısız olur (tünel dışa-doğru), o yüzden "ben public'im" denir.
     Bu olmadan relay servisi hiç açılmaz.
   - `EnableRelayService(WithACL(registry), WithResources(...))` —
     **Circuit Relay v2**. ACL olarak oda kayıt defteri verilir.
   - `AddrsFactory(...)` — `-announce` verilmişse host'un kendi
     adresleri (0.0.0.0, konteynerin 172.17.x.x'i) **tamamen atılır** ve
     yerine sadece ilan edilen adres konur.
3. `SetStreamHandler("/puresend/rendezvous/1.0.0", registry.Handler)`
4. `:8081/health` açılır → `{"status":"ok","peer_id":"...","active_rooms":N}`

### 3.2. İstemci açılışı — `internal/p2p/node.go:91`

1. `libp2p.New(EnableHolePunching(), EnableAutoNATv2(), NATPortMap())`
   - `EnableHolePunching` = **DCUtR**. Relay üzerindeki ilk bağlantıyı
     doğrudan bağlantıya yükseltmeye çalışan mekanizma.
   - `NATPortMap` = modem UPnP/NAT-PMP konuşuyorsa port açtırmayı dener.
   - Dikkat: `libp2p.Identity` verilmiyor → **istemci her açılışta yeni
     bir kimlik üretir**. ([§5.6](#56-lookup-hız-limiti-pratikte-aşılabilir)'da buna döneceğim.)
2. `h.Connect(server)` — 30 sn timeout.
   **Ölçüm: 0,45 sn.**

### 3.3. Gönderen oda açıyor — `Node.Host()`

1. **İlan edilecek adres listesi hazırlanır**, sırası önemli:
   - önce sunucunun her adresi için bir **relay circuit adresi**:
     `<sunucu-adresi>/p2p/<sunucuPeerID>/p2p-circuit`
   - sonra host'un kendi adresleri (LAN IP'si, varsa UPnP ile açılmış port)
   - `rendezvous.MaxAddrs = 64` ile kırpılır
2. `NewRoomCode()` → `kiraz-liman-42` biçimi.
   138 × 138 × 90 ≈ **1,7 milyon** kombinasyon.
3. `rendezvous.Register` — tek satır JSON, `roundTrip`, 30 sn deadline.
4. `relayclient.Reserve(...)` — relay'de yer ayırtır.
   **Sıra kritik:** relay'in ACL'i "bu eşin aktif odası var mı?" diye
   sorar (`Registry.AllowReserve`), o yüzden **önce kayıt, sonra
   rezervasyon** yapılıyor. Ters olsa ACL reddederdi.
5. `SetStreamHandler("/puresend/transfer/1.1.0", ...)` kurulur —
   gönderen artık pasif; alan bağlanınca bu handler tetiklenecek.

   **Ölçüm: kod ekranda, 0,75 sn.**

### 3.4. Alan kodu giriyor — `Node.fetch()`

1. `rendezvous.Lookup(room)` → sunucu `{"type":"found","peer_id":...,"addrs":[...]}`
   döner. Bulunamazsa o eşin **başarısız deneme sayacı** artar
   (dakikada 5 hata → geçici blok).
2. `h.Connect(sender)` — 60 sn timeout. libp2p listedeki adresleri
   dener; ayrı ağlardaysanız çalışan tek adres circuit adresidir, yani
   ilk bağlantı **relay üzerinden** kurulur.
3. `waitForDirect(20 sn)` — arka planda DCUtR delik açmaya çalışırken
   300 ms'de bir "artık `Limited` olmayan bir bağlantı var mı?" diye
   bakılır. `Limited == true` = relay üzerinden demek.
4. `ConnectedEvent{Direct: ...}` → ekranda ya
   *"✓ Doğrudan bağlantı kuruldu"* ya da *"! Yedek yol kullanılıyor"*.
5. `NewStream(transfer.ProtocolID)` — bağlantı relay üzerindeyse
   `network.WithAllowLimitedConn` ile açılır (libp2p sınırlı bağlantıda
   stream açmaya açık izin ister).

### 3.5. Transfer — `internal/transfer/transfer.go`

```
GÖNDEREN                                    ALAN
   │                                          │
   │  1. manifest (JSON, tek satır)          │
   │     her dosya: ad + boyut + SHA-256     │
   │  ───────────────────────────────────►   │
   │                                          │  2. kullanıcıya onay ekranı
   │                                          │     ("Evet, indir" / "Hayır")
   │  ◄───────────────────────────────────   │
   │     3. ack{ok:true}                     │
   │                                          │
   │  4. ham byte'lar, 32 KB'lık parçalar    │
   │     dosya sırası manifest sırası        │
   │  ───────────────────────────────────►   │  5. .part dosyasına yaz,
   │                                          │     akarken SHA-256 hesapla
   │                                          │  6. özet tutuyorsa rename
   │  ◄───────────────────────────────────   │
   │     7. ack{ok:true}                     │
   │  8. ancak şimdi "Gönderildi!" der       │
```

Kodda dikkat çeken, doğru yapılmış detaylar:

- **Manifest `json.Decoder` ile okunmuyor** (`transfer.go:122`). Decoder
  stream'den fazladan byte tamponlar ve arkadan gelen dosya byte'larının
  başını yutardı. Onun yerine sınırlı bir satır okuyucu var
  (`readLimitedLine`, 1 MB tavan).
- **Önce `.part`, sonra rename** — özet doğrulanmadan dosya nihai adını
  almıyor. Yarıda kesilen transfer "tamam gibi görünen bozuk dosya"
  bırakmıyor. Testte doğruladım: relay limiti aşılıp transfer koptuğunda
  klasörde **hiçbir şey** kalmadı.
- **`filepath.Base(info.Name)`** — karşı taraf `../../.bashrc` yollasa
  bile hedef klasörden çıkamıyor (path traversal koruması).
- **`availablePath`** — var olan dosyanın üzerine yazmıyor, `foto (1).jpg`
  diye açıyor.
- **Gönderen alanın son ack'ini bekliyor**, alan da stream kapansın diye
  bekliyor (`transfer.go:165`). Yani "Gönderildi!" gerçekten "karşıya
  eksiksiz indi" demek.

---

## 4. Test sonuçları

### 4.1. Uçtan uca transferler

| # | Senaryo | Ortam | Yol | Sonuç |
|---|---|---|---|---|
| 1 | 8 MB tek dosya | 127.0.0.1 | doğrudan | ✅ SHA-256 aynı |
| 2 | 64 MB tek dosya | 127.0.0.1 | doğrudan | ✅ SHA-256 aynı |
| 3 | 64 MB tek dosya | izole netns | **relay** | ✅ SHA-256 aynı |
| 4 | 3 dosya (0,3 + 1,2 + 3,5 MB) | 127.0.0.1 | doğrudan | ✅ üçünün de özeti aynı |
| 5 | 32 MB, sunucu Docker'da | konteyner | doğrudan | ✅ SHA-256 aynı |
| 6 | 16 MB, sunucu ham TCP/QUIC modunda (`-port`) | 127.0.0.1 | doğrudan | ✅ SHA-256 aynı |

### 4.2. Ölçümler

| Adım | Doğrudan | Relay |
|---|---|---|
| İstemci → buluşma sunucusu | 0,45 sn | 0,45 sn |
| Gönderende oda kodu hazır | 0,75 sn | 0,75 sn |
| Alan koddan → onay ekranı | 1,4 sn | **21,6 sn** |
| 64 MB transferin tamamı | 1,65 sn | 22,0 sn |

Relay sütunundaki ~20 saniyenin tamamı `directWait` sabiti
([`node.go:27`](../internal/p2p/node.go#L27)) — delik açılmayacağı belli olana
kadar beklenen süre. Transferin kendisi yine ~1 saniye.

### 4.3. Hata ve kenar durumlar

| Senaryo | Beklenen | Gerçekleşen |
|---|---|---|
| Olmayan kod girildi (`zebra-zebra-99`) | anlaşılır hata | ✅ *"Bu kodla açılmış bir oda bulunamadı."* + 3 öneri |
| Alan "Hayır, iptal" dedi | diske hiçbir şey yazılmaz | ✅ klasör boş, alan ana menüye döndü, **hata ekranı yok** |
| Alan reddetti → gönderen | oturum ölmesin | ✅ *"Bir deneme yarıda kaldı. Arkadaşın aynı kodla tekrar deneyebilir."* |
| Relay veri limiti aşıldı (8 MB dosya, 4 MB limit) | temiz başarısızlık | ✅ transfer koptu, `.part` kalmadı, bozuk dosya yok — ⚠️ ama hata metni İngilizce ve teknik ([§5.3](#53-teknik-hata-metni-kullanıcıya-sızıyor)) |
| Gönderen relay hatası sonrası | beklemeye dönsün | ✅ döndü, aynı kod hâlâ geçerli |
| Aynı kodla **ikinci** alıcı | reddedilsin | ❌ **dosyaları tekrar indirdi** ([§5.1](#51-kritik--oda-kodu-tek-kullanımlık-değil)) |

### 4.4. Deploy zinciri

| Adım | Sonuç |
|---|---|
| `docker compose build` | ✅ (çok aşamalı, alpine, statik binary) |
| `docker compose up -d` | ✅ Peer ID + gömülecek istemci adresi loglandı |
| `/health` host portundan | ✅ `{"status":"ok",...}` |
| Compose healthcheck | ✅ `Up (healthy)` |
| README'deki `base64 -w0 /data/server.key` | ✅ busybox base64 `-w`'yi destekliyor, komut doğru |
| Volume yok + `FT_IDENTITY_KEY` ile yeniden başlat | ✅ **Peer ID birebir korundu** (`Identity from: FT_IDENTITY_KEY`) |
| `FT_ANNOUNCE` ile ilan | ✅ istemci adresi doğru basıldı |
| Konteynerdeki sunucu üzerinden gerçek transfer | ✅ 32 MB, özet aynı |

Yani README'nin OpenShip bölümü (`command:` yok sayılıyor, volume
kayboluyor → ortam değişkenleriyle çöz) **doğru ve çalışıyor**.

---

## 5. Bulunan sorunlar

Önem sırasına göre.

### 5.1. KRİTİK — Oda kodu tek kullanımlık değil

**Ne oluyor:** Transfer bitiyor, gönderende *"✓ Gönderildi!"* ekranı
duruyor. Kullanıcı Enter'a basana kadar (çay almaya gitti, ekranı kapattı,
her neyse) **aynı kodu bilen ikinci biri dosyaları tekrar indirebiliyor.**
Gönderen bunu hiçbir şekilde görmüyor — ekranda hâlâ "Gönderildi!" yazıyor.

Doğrudan test ettim:

```
[22:33:23] birinci alıcı bitirdi; gönderen 'Gönderildi!' ekranında
[22:33:25] ikinci alıcı AYNI kodu giriyor...
[22:33:31] ikinci alıcı gördü: 'Sana dosya gönderilmek isteniyor'
[22:33:31] onayladıktan sonra: 'İndi!'
===== SONUÇ =====
ikinci alıcı aynı kodla dosyaları aldı: True
gönderen ekranı hâlâ 'Gönderildi!' diyor: True
```

**Neden:** İki şey üst üste biniyor.

1. Oturumu kapatan `reset()` ([`tui.go:460`](../internal/tui/tui.go#L460))
   yalnızca kullanıcı Enter/`q`'ya basınca çağrılıyor. O ana kadar node
   ayakta ve `transfer.ProtocolID` handler'ı kayıtlı duruyor.
   İşin ironisi, `reset()`'in kendi yorumu bunu engellediğini söylüyor:
   *"that stops the finished session from serving its files to whoever
   still has the old code"* — niyet doğru, ama tetikleyici kullanıcının
   tuşuna bağlı.
2. Başarılı `DoneEvent` sonrası `waitEvent` **yeniden kurulmuyor**
   ([`tui.go:309-311`](../internal/tui/tui.go#L309-L311)). Bu yüzden ikinci
   transferin olayları hiç işlenmiyor ve ekrana yansımıyor — sessizce oluyor.

**Öneri:** `Send` başarıyla bittiğinde `p2p` katmanında
`host.RemoveStreamHandler(transfer.ProtocolID)` çağır ve odayı sunucudan
düşür. Böylece kullanıcının tuşuna bağlı kalmaz.

### 5.2. Oda kaydı hiç silinmiyor

`active_rooms` bütün testlerde 1'de kaldı — transfer bittikten, hatta iki
istemci de kapandıktan sonra bile. `rendezvous` protokolünde
`register`/`lookup` var, **`unregister` yok**
([`rendezvous.go:195`](../internal/rendezvous/rendezvous.go#L195)); kayıtlar
yalnızca 1 saatlik TTL dolunca düşüyor.

Sonuçları: kod havuzu boşuna dolu kalıyor, `maxRooms` (1000) gereksiz yere
tükenebilir, ve §5.1'deki pencere TTL boyunca açık kalır.

### 5.3. Teknik hata metni kullanıcıya sızıyor

Relay limiti aşılıp bağlantı koptuğunda ekranda birebir bu çıktı:

```
✗  Bir sorun çıktı

connection lost while receiving tatil-fotograflari.bin: stream reset:
stream reset: connection closed: unexp…

Ne yapabilirsin:
  • Tekrar denemek için Enter'a bas.
```

Bu, projenin kendi altın kuralına aykırı —
[`tui.go:5`](../internal/tui/tui.go#L5): *"Words like multiaddr, peer, NAT or
relay never reach the screen."* `explain()`
([`tui.go:740`](../internal/tui/tui.go#L740)) `connection lost` / `stream
reset` durumunu yakalamıyor, `default` dalına düşüyor.

**Öneri:** `explain()`'e bir dal daha:
> *"Bağlantı transfer sırasında koptu."* → "İkiniz de programı açık tutup
> tekrar deneyin", "Dosya çok büyükse ve doğrudan yol açılamadıysa yedek
> yolun bir boyut sınırı var".

### 5.4. Delik açılamazsa büyük dosya sessizce duvara toslar

Relay yedeğinde bağlantı başına **256 MB** sınırı var (`-relay-data`).
Yani doğrudan yol açılamayan bir çiftte 300 MB'lık bir video **her zaman**
yarıda kesilir. Testte 4 MB limitle bunu birebir üretebildim.

Şu an kullanıcı bunu ancak transfer kopunca öğreniyor. Bilgi zaten elde
var: manifest toplam boyutu biliniyor ve bağlantının `Limited` olup
olmadığı biliniyor. **Öneri:** onay ekranında, bağlantı relay üzerindeyse
ve toplam boyut limiti aşıyorsa baştan uyar.

### 5.5. Sabit 20 saniyelik bekleme

`directWait = 20 * time.Second` sabit. Delik açılamayacak bir çiftte
kullanıcı 20 saniye boyunca *"Doğrudan yol açılıyor..."* ekranına bakıyor;
en azından "birkaç saniye sürebilir" yazısı var ama 20 saniye uzun.
Tersten, kötü bir mobil bağlantıda 20 saniye bazen az kalabilir.
AutoNAT sonucu zaten elde — erken pes etmek için kullanılabilir.

### 5.6. Lookup hız limiti pratikte aşılabilir

Kaba kuvvete karşı koruma "eş başına dakikada 5 hatalı deneme"
([`rendezvous.go:224`](../internal/rendezvous/rendezvous.go#L224)). Ama sayaç
**peer ID başına** tutuluyor ve istemci `p2p.New` içinde `libp2p.Identity`
vermiyor — yani **her açılışta yepyeni bir kimlik** üretiyor. Saldırgan
süreci yeniden başlatarak (ya da döngüde kimlik üreterek) sayacı sıfırlar.

1,7 milyon kombinasyon ve 1 saatlik pencereyle hâlâ zahmetli, ama
tasarlanan koruma pratikte yok sayılabilir. Gerçek limit bağlantı/IP
düzeyinde olmalı (veya §6'daki PAKE bunu tümden gereksiz kılar).

### 5.7. Eş başına oda sınırı yok

`maxRooms = 1000` yalnızca **toplam**. Tek bir eş art arda 1000 farklı kod
kaydedip sunucuyu doldurabilir; sonrasında herkes
*"server is full, try again later"* alır. Eş başına 1–2 oda sınırı ucuz bir
düzeltme.

### 5.8. İstemcide sürüm bilgisi yok

`puresend -version` yok; GoReleaser da yalnızca `main.defaultServer`
gömüyor. Kullanıcı "hangi sürümü çalıştırıyorsun?" sorusuna cevap veremez —
destek ve hata ayıklama için ilk sorulacak şey bu.

### 5.9. İndirme klasörü seçilemiyor

`~/İndirilenler/PureSend` sabit. Harici diske indirmek isteyen
kullanıcı için hiçbir yol yok (bayrak da yok).

### 5.10. Küçük notlar

- **CPR yanıtı vermeyen terminallerde 5 sn boş ekran.** Gerçek
  terminallerde sorun yok (0,3 sn); `script`, bazı CI koşucuları ve bazı
  uzak kabuklar `ESC[6n`'e cevap vermez ve kullanıcı 5 saniye boş ekrana
  bakar. Bubble Tea'nin davranışı, ama bilerek olsun.
- **Health portu doluysa sunucu yine de ayağa kalkıyor**, sadece
  `health server stopped: ... address already in use` loglanıyor. Testte
  birebir yaşadım. Graceful ama OpenShip healthcheck'i sessizce hep
  başarısız olur — bunu ölümcül yapmak veya en azından başlangıç
  banner'ında belirtmek daha iyi.
- **Başsız istemci modu yok.** `internal/p2p` paketinin yorumu
  *"the same logic works under a TUI, a CLI or a test"* diyor ve gerçekten
  öyle tasarlanmış, ama karşılığı olan CLI yok. Bu yüzden bu testleri
  pty'den tuş göndererek yapmak zorunda kaldım. Basit bir
  `-send <dosya>` / `-receive <kod>` modu, CI'da uçtan uca test
  koşturmayı da mümkün kılar.

### 5.11. README'de zaten yazan, doğruladığım sınırlar

Bunlar bulgu değil, bilinen sınırlar — teyit ettim: resume yok (kesilen
transfer baştan başlar), klasör gönderilemiyor, PAKE yok (sunucu güvenilir
taraf), relay yedeği bağlantı başına 256 MB.

---

## 6. Ne eklense güzel olur?

Etki/emek dengesine göre sıraladım.

**Önce bunlar (küçük iş, büyük etki)**

1. **Odayı transfer bitince kapat** — §5.1 + §5.2'yi birlikte çözer.
   Protokole `unregister` ekle, `Send` başarılı bitince handler'ı kaldır.
2. **`explain()`'e bağlantı-koptu dalı** — §5.3. Yarım saatlik iş.
3. **İstemciye `-version`** — GoReleaser zaten `.Version` veriyor,
   tek bir `ldflags` satırı.
4. **Relay limiti uyarısı** — onay ekranında "bu dosya yedek yoldan
   geçemeyecek kadar büyük" uyarısı.
5. **Eş başına oda sınırı** — §5.7, birkaç satır.

**Sonra bunlar (kullanıcının ilk isteyeceği şeyler)**

6. **Transfer hızı ve kalan süre.** Şu an sadece yüzde ve byte var.
   `ProgressEvent` içinde zaten yeterli bilgi var, hesap tamamen TUI
   tarafında yapılabilir.
7. **Resume.** `.part` dosyaları zaten var; manifeste offset eklenip
   `Range` benzeri bir devam mekanizması kurulabilir. Büyük dosyalarda
   mobil bağlantı için neredeyse zorunlu.
8. **Klasör gönderme.** Manifest'e göreli yol alanı eklenip
   `filepath.Base` yerine güvenli bir `filepath.Clean` + kaçış kontrolü.
9. **İndirme klasörünü seçebilme** (TUI'de bir ekran veya `-out`).

**Mimari / güvenlik**

10. **PAKE (SPAKE2) ile koddan anahtar türetme.** Şu an sunucu güvenilir
    taraf: alıcıya "gönderen bu Peer ID" diyen o. `magic-wormhole`
    modelinde oda kodu bir parola gibi kullanılıp iki uç arasında ortak
    anahtar türetilir; sunucu yalan söylerse transfer başlamaz. Bu, §5.6'yı
    da tümden gereksiz kılar (yanlış kod = anahtar tutmaz, kaba kuvvet
    denemesi tek atışlık olur). **Projenin en değerli tek eklemesi bu.**
11. **Birden fazla buluşma sunucusu.** Şu an tek adres gömülü — sunucu
    düşerse dağıtılmış bütün istemciler ölür. Virgülle ayrılmış birkaç
    adres + sırayla deneme, tek satırlık bir SPOF sigortası.
12. **Sunucuda Prometheus metrikleri** — aktif oda, relay üzerinden akan
    byte, delik açma başarı oranı. Özellikle "kaç kullanıcı relay'e
    düşüyor" sorusunun cevabı, 256 MB limitini ayarlaman için lazım olacak.

**Test / DX**

13. **Başsız istemci modu** (§5.10) → CI'da gerçek uçtan uca test.
14. **CI'ya netns tabanlı relay testi.** Bu raporda kullandığım topoloji
    ayrıcalık gerektirmiyor (`unshare -Urnm`), GitHub Actions'ta çalışır.
    "Delik açılamadığında relay yedeği hâlâ çalışıyor mu" sorusunu her
    commit'te cevaplar.

---

## 7. Relay sunucusunu deploy etme

Aşağıdaki adımların hepsi bu makinede (DNS/tünel kısmı hariç) birebir
çalıştırıldı.

### 7.1. Ön koşullar

Ubuntu kasada Docker + `docker compose`, ve `cloudflared`. Alan adın
Cloudflare'de olmalı.

### 7.2. Sunucuyu ayağa kaldır

```bash
git clone <repo> && cd PureSend

PUBLIC_HOST=puresend.madebybaki.com docker compose up -d
docker compose logs rendezvous
```

Çıktıdan **iki şeyi** al:

```
  Peer ID: 12D3KooWBHbknSWd9Fu2rZKcR1GnZ8nWzV5rZzDYZvnQVyAfTe2x
  Identity from: /data/server.key (newly generated)

Client address (bake this into the client build):
  /dns4/puresend.madebybaki.com/tcp/443/tls/ws/p2p/12D3KooWBHbk...
```

Alttaki satır istemcilere gömeceğin adres. Sağlık kontrolü:

```bash
curl -s localhost:8081/health
# {"status":"ok","peer_id":"12D3KooW...","active_rooms":0}
docker compose ps        # "Up (healthy)" görmelisin
```

### 7.3. 🔑 Anahtarı hemen yedekle

**Bu adımı atlama.** `server.key` giderse Peer ID değişir ve o ana kadar
dağıttığın bütün istemciler çöpe gider.

```bash
docker exec $(docker compose ps -q rendezvous) base64 -w0 /data/server.key
```

Çıkan tek satırı bir parola yöneticisine koy. (Test ettim: alpine'in
busybox `base64`'ü `-w0`'ı destekliyor, komut olduğu gibi çalışıyor.)

### 7.4. Cloudflare Tunnel

**Makinede zaten bir tünel varsa** — config'i ASLA üzerine yazma, tek
`cloudflared` bütün hostname'lere aynı dosyadan hizmet eder:

```bash
sudo cat /etc/cloudflared/config.yml     # önce bak
cloudflared tunnel list
cloudflared tunnel route dns <mevcut-tünel> puresend.madebybaki.com
```

Sonra `ingress:` listesine, **catch-all `http_status:404`'ten ÖNCE**:

```yaml
  - hostname: puresend.madebybaki.com
    service: http://localhost:8080
    originRequest:
      connectTimeout: 30s
```

```bash
cloudflared tunnel ingress validate
sudo systemctl restart cloudflared
```

**Hiç tünel yoksa:**

```bash
cloudflared tunnel login
cloudflared tunnel create puresend
cloudflared tunnel route dns puresend puresend.madebybaki.com
sudo cp deploy/cloudflared-config.yml /etc/cloudflared/config.yml
sudo cloudflared service install
```

DNS kaydı **CNAME → `<tunnel-id>.cfargotunnel.com`** ve **proxied (turuncu
bulut)** olmalı. A kaydı veya gri bulut tünele hiç ulaşmaz.

### 7.5. Dışarıdan doğrula

Sunucunun **dışındaki** bir ağdan (telefon hotspot'u ideal):

```bash
curl -sI https://puresend.madebybaki.com \
     -H "Connection: Upgrade" -H "Upgrade: websocket"
```

### 7.6. İstemcileri yayınla

GitHub → *Settings → Secrets and variables → Actions → Variables* →
`FT_SERVER`:

```
/dns4/puresend.madebybaki.com/tcp/443/tls/ws/p2p/12D3KooW...
```

```bash
git tag v0.1.0 && git push --tags
```

GoReleaser 6 ikili üretip Releases'a koyar. Elle:

```bash
go build -ldflags "-X main.defaultServer=/dns4/.../p2p/12D3KooW..." ./cmd/client
```

### 7.7. OpenShip kullanıyorsan

OpenShip compose'u **kısmen** uyguluyor: `command:` yok sayılıyor ve
volume'ler anonim açıldığı için her deploy'da `server.key` kayboluyor
(→ Peer ID değişir → herkes kırılır). Çözüm ortam değişkenleri; bunları
OpenShip **uyguluyor**. Bu yolu birebir test ettim ve **Peer ID korundu**:

| Değişken | Değer |
|---|---|
| `FT_IDENTITY_KEY` | §7.3'teki base64 çıktısı (**gizli tut**) |
| `FT_ANNOUNCE` | `/dns4/puresend.madebybaki.com/tcp/443/tls/ws` |

Doğrulaması: açılış logunda `Identity from: FT_IDENTITY_KEY` satırını
görmelisin. Gördüysen kimlik artık diskten tamamen bağımsız.

`cloudflared` hedefi de `8080` değil, OpenShip'in sabitlediği port:

```bash
docker ps --format '{{.Names}}\t{{.Ports}}' | grep filetransfer
# ... 127.0.0.1:20001->8080/tcp   →  ingress: http://localhost:20001
```

> ⚠️ Bu port yeniden deploy'da değişebilir ve değişirse tünel **sessizce**
> kırılır. "Bir gün çalışmıyor" olursa ilk buraya bak.

`8081` host'a çıkmadığı için sağlık kontrolü konteyner içinden:

```bash
docker exec <konteyner> wget -qO- http://127.0.0.1:8081/health
```

### 7.8. Alternatif: tünel yok, port yönlendirme var

Modeminde port açabiliyorsan tünele hiç gerek yok — ham TCP+QUIC modu var
ve bunu da test ettim (16 MB transfer, sorunsuz):

```bash
go run ./cmd/server -port 4001 -ws-port 0 \
  -announce /ip4/<genel-IP>/tcp/4001
```

QUIC de açıldığı için delik açma şansı WebSocket'e göre **daha yüksektir**.
Sabit IP'n yoksa `/dns4/<dinamik-dns-adın>/tcp/4001` kullan.

---

## 8. Senin kendi testin için

Ev kasan + mobil veriyle bağlanan laptop denemende bakman gerekenler:

1. **En kritik satır** — transfer ekranında hangisi çıkıyor?
   - *"✓ Doğrudan bağlantı kuruldu"* → delik açıldı, dosyalar Cloudflare'i
     hiç görmüyor. İstediğin bu.
   - *"! Yedek yol kullanılıyor"* → delik açılamadı, her byte senin
     sunucundan ve Cloudflare'den geçiyor. Mobil operatörler genelde
     **CGNAT** arkasındadır ve bu oldukça olası.

2. **İki yönde de dene.** Delik açma asimetriktir: ev→mobil çalışıp
   mobil→ev çalışmayabilir. Gönderen/alan rollerini değiştirip tekrarla.

3. **Büyük dosyayı unutma.** Yedek yola düşerseniz 256 MB'lık sınır var
   (§5.4) — 500 MB'lık bir dosyayla dene, kopmasını bekliyorum. Kopmazsa
   delik açılmış demektir, o da iyi haber.

4. **Aynı LAN'da da dene** (ikisi de evde, aynı wifi). Bu durumda
   `Node.Host` kendi LAN adreslerini de ilan ettiği için bağlantı anında
   ve doğrudan kurulmalı.

5. **Sunucuda izle:**
   ```bash
   watch -n1 'docker exec <konteyner> wget -qO- http://127.0.0.1:8081/health'
   docker compose logs -f rendezvous
   ```
   `active_rooms` transferden sonra da **1'de kalacak** — bu §5.2, hata
   değil, bilinen davranış.

6. **Kodun tek kullanımlık olmadığını unutma** (§5.1). Test sırasında
   gönderende "Gönderildi!" ekranını bırakıp aynı kodu üçüncü bir cihazda
   dene — dosyalar tekrar inecek. Bunu doğrulaman düzeltmenin değerini
   netleştirir.

---

## Ek: testleri yeniden koşturmak

Bu raporun yazıldığı oturumda kullanılan betikler (`e2e.py`, `edge.py`,
`replay.py`, `netns-setup.sh`) o oturumun geçici klasöründeydi ve repoya
hiç alınmadı. Kapsadıkları senaryoların hepsi artık repodaki testlerde:

| Rapordaki betik | Repodaki karşılığı |
|---|---|
| `e2e.py local` — sunucu + 2 istemci, SHA-256 doğrulaması | `internal/integration/headless_test.go` (gerçek ikili, headless mod) |
| `e2e.py relay` + `netns-setup.sh` — delik açılamayan iki ağ | `test/relay/netns-relay-test.sh` (`make test-relay`) |
| `edge.py` — çok dosya, yanlış kod, reddetme | `internal/integration/features_test.go`, `singleuse_test.go`, `internal/transfer/*_test.go` |
| `replay.py` — §5.1, aynı kodla ikinci alıcı | `internal/integration/singleuse_test.go` (`TestRoomIsSingleUse`) |

```bash
make test         # hepsi, race detector açık
make test-relay   # relay yedeği, izole ağlar arasında
```
