import type { ComponentType } from 'react'
import type { Ctx } from '../tui/screens'
import {
  Confirm,
  Connecting,
  Done,
  EnterCode,
  Finding,
  PickFiles,
  RoomCode,
  Transfer,
  Waiting,
  Welcome,
} from '../tui/screens'

export type Side = 'send' | 'recv'

export type Step = {
  id: string
  side: Side
  /** Short label for the rail. */
  tag: string
  title: string
  body: string
  Screen: ComponentType<Ctx>
}

/**
 * The scroll story: one pass through the program, told from the sender's
 * machine and then — halfway down — from the receiver's.
 */
export const STEPS: Step[] = [
  {
    id: 'welcome',
    side: 'send',
    tag: 'Aç',
    title: 'Aç, tek soruyla karşılaş',
    body: 'Kurulum yok, hesap yok, ayar yok. Program açılır açılmaz sorduğu tek şey: gönderecek misin, alacak mısın?',
    Screen: Welcome,
  },
  {
    id: 'connecting',
    side: 'send',
    tag: 'Bağlan',
    title: 'Buluşma noktasına bağlan',
    body: 'Küçük bir adres defterine "buradayım" dersin. Dosyalar oraya gitmez — sunucu iki bilgisayarı tanıştırmaktan başka bir şey yapmaz.',
    Screen: Connecting,
  },
  {
    id: 'pick',
    side: 'send',
    tag: 'Seç',
    title: 'Dosyaları seç',
    body: 'Dosya gezgini programın içinde. Yön tuşlarıyla gez, Enter ile listeye ekle. Boyut sınırı yok; 40 GB da gönderirsin.',
    Screen: PickFiles,
  },
  {
    id: 'room',
    side: 'send',
    tag: 'Kod',
    title: 'Üç kelimelik kodu al',
    body: 'IP adresi yok, link yok, QR yok. Telefonda söyleyebileceğin üç kelime — arkadaşına ilettiğin tek şey bu.',
    Screen: RoomCode,
  },
  {
    id: 'waiting',
    side: 'send',
    tag: 'Bekle',
    title: 'Ve bekle',
    body: 'Kodu WhatsApp\'tan yolla, pencereyi açık bırak. Arkadaşın kodu girdiği anda gönderme kendiliğinden başlar. Kod bir saat geçerli.',
    Screen: Waiting,
  },
  {
    id: 'enter',
    side: 'recv',
    tag: 'Gir',
    title: 'Karşı tarafta: kodu yaz',
    body: 'Arkadaşın aynı programı açar, "Bana dosya gönderilecek" der ve üç kelimeyi yazar. Onun bilmesi gereken de bu kadar.',
    Screen: EnterCode,
  },
  {
    id: 'finding',
    side: 'recv',
    tag: 'Bul',
    title: 'İki bilgisayar birbirini bulur',
    body: 'İki modem arkasındaki iki bilgisayar arasında doğrudan bir yol açılır — NAT delme. Sunucu bu noktadan sonra devre dışı.',
    Screen: Finding,
  },
  {
    id: 'confirm',
    side: 'recv',
    tag: 'Onayla',
    title: 'Ne geldiğini gör, sonra onayla',
    body: 'Hiçbir şey sorulmadan inmez. Dosya adlarını, boyutları ve nereye kaydedileceğini görür, ondan sonra "Evet" dersin.',
    Screen: Confirm,
  },
  {
    id: 'transfer',
    side: 'recv',
    tag: 'Aktar',
    title: 'Dosyalar doğrudan akar',
    body: 'Bağlantı uçtan uca şifreli ve doğrudan: veri iki bilgisayar dışında hiçbir yere uğramaz. Doğrudan yol açılamazsa şifreli yedek yola düşer.',
    Screen: Transfer,
  },
  {
    id: 'done',
    side: 'recv',
    tag: 'Bitti',
    title: 'İndi — ve eksiksiz indiği doğrulandı',
    body: 'Her dosya SHA-256 ile tek tek doğrulanır. Bozuk inen dosya sessizce kabul edilmez.',
    Screen: Done,
  },
]

export const HOSTNAME: Record<Side, string> = {
  send: 'ayse@macbook — gönderen',
  recv: 'mehmet@thinkpad — alan',
}
