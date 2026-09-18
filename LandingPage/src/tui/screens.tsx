import {
  Body,
  ButtonRow,
  Caret,
  Choice,
  Foot,
  Gap,
  Help,
  Ok,
  ProgressBar,
  Title,
  TuiBox,
} from './primitives'
import { DEMO_FILES, DEMO_TOTAL, OUT_DIR, ROOM_CODE } from './demo'
import { SPINNER, cells, formatBytes, pickerSize, range, repeat } from './text'

/** What the story hands every screen: a spinner frame and 0→1 step progress. */
export type Ctx = { frame: number; sub: number }

const Spin = ({ frame }: { frame: number }) => (
  <span className="text-tui-accent">{SPINNER[frame % SPINNER.length]}</span>
)

// ---------------------------------------------------------------------------
// 1 · Welcome
// ---------------------------------------------------------------------------

export function Welcome() {
  return (
    <>
      <Title>📦  FileTransferilla</Title>
      <Gap />
      <Body>Dosyalarını arkadaşına doğrudan gönderirsin.</Body>
      <Help>Dosyaların hiçbir siteye yüklenmez — senin bilgisayarından</Help>
      <Help>çıkar, arkadaşının bilgisayarına iner.</Help>
      <Gap />
      <Body>Ne yapmak istiyorsun?</Body>
      <Gap />
      <Choice label="📤  Dosya göndereceğim" selected />
      <Choice label="📥  Bana dosya gönderilecek" selected={false} />
      <Gap />
      <Foot>↑ ↓ ile seç · Enter ile onayla · q ile çık</Foot>
    </>
  )
}

// ---------------------------------------------------------------------------
// 2 · Connecting
// ---------------------------------------------------------------------------

export function Connecting({ frame }: Ctx) {
  return (
    <>
      <Title>Bağlanılıyor</Title>
      <Gap />
      <div className="text-tui-fg">
        <Spin frame={frame} /> Buluşma noktasına bağlanılıyor...
      </div>
      <Gap />
      <Help>Bu, iki bilgisayarın birbirini bulmasını sağlayan küçük bir</Help>
      <Help>adres defteri. Dosyaların oraya gitmiyor, sadece</Help>
      <Help>"buradayım" diyorsun.</Help>
      <Gap />
      <Foot>Ctrl+C ile çık</Foot>
    </>
  )
}

// ---------------------------------------------------------------------------
// 3 · File picker
// ---------------------------------------------------------------------------

const BROWSER = [
  { name: 'Belgeler', size: 4096, dir: true },
  { name: 'İndirilenler', size: 4096, dir: true },
  { name: 'Masaüstü', size: 4096, dir: true },
  { name: 'tatil-fotograflari.zip', size: DEMO_FILES[0].size, dir: false },
  { name: 'dugun-videosu.mp4', size: DEMO_FILES[1].size, dir: false },
  { name: 'notlar.txt', size: 12_204, dir: false },
]

export function PickFiles({ sub }: Ctx) {
  // The cursor walks down to the two files, picking each one up on the way.
  const picked = sub < 0.34 ? 0 : sub < 0.66 ? 1 : 2
  const cursor = Math.min(3 + picked, BROWSER.length - 1)
  const chosen = DEMO_FILES.slice(0, picked)
  const total = chosen.reduce((a, f) => a + f.size, 0)

  return (
    <>
      <Title>📤  Hangi dosyaları göndereceksin?</Title>
      <Gap />
      <Help>Klasörlerin içine girmek için Enter'a bas. Göndermek</Help>
      <Help>istediğin dosyanın üzerinde Enter'a basınca listeye eklenir.</Help>
      <Gap />
      {BROWSER.map((f, i) => {
        const size = pickerSize(f.size).padStart(7)
        const on = i === cursor
        return (
          <div key={f.name}>
            <span className="text-tui-accent">{on ? '>' : ' '}</span>
            <span className="text-tui-faint">{size}</span>
            <span
              className={
                on
                  ? 'font-bold text-tui-accent'
                  : f.dir
                    ? 'text-tui-dir'
                    : 'text-tui-fg'
              }
            >
              {' ' + f.name}
            </span>
          </div>
        )
      })}
      <Gap />
      {picked > 0 ? (
        <>
          <Ok>{`Seçtiklerin (${picked}):`}</Ok>
          {chosen.map((f) => (
            <div key={f.name}>
              <span className="text-tui-fg">{`  • ${f.name}`}</span>
              <span className="text-tui-muted">{` (${formatBytes(f.size)})`}</span>
            </div>
          ))}
          <div className="text-tui-muted">{`  Toplam: ${formatBytes(total)}`}</div>
          <Gap />
          <ButtonRow buttons={['s  ·  Göndermeye başla']} selected={0} />
          <Gap />
          <Foot>↑ ↓ gez · Enter seç · Backspace son seçimi sil · Ctrl+C çık</Foot>
        </>
      ) : (
        <>
          <Help>Henüz dosya seçmedin.</Help>
          <Gap />
          <Foot>↑ ↓ gez · Enter seç · Ctrl+C çık</Foot>
        </>
      )}
    </>
  )
}

// ---------------------------------------------------------------------------
// 4 · Room code
// ---------------------------------------------------------------------------

export function RoomCode({ sub }: Ctx) {
  // The code lands one word at a time, the way it does when the room is claimed.
  const words = ROOM_CODE.split('-')
  const shown = sub < 0.12 ? 1 : sub < 0.24 ? 2 : 3
  const code =
    words.slice(0, shown).join('-') + repeat(' ', cells(ROOM_CODE) - cells(words.slice(0, shown).join('-')))

  return (
    <>
      <Title>🔑  Oda kodun hazır!</Title>
      <Gap />
      <TuiBox lines={[code]} />
      <Gap />
      <Body>Şimdi arkadaşına bu kodu ilet.</Body>
      <Help>WhatsApp'tan yaz, SMS at ya da telefonda söyle — fark etmez.</Help>
      <Gap />
      <Help>Arkadaşın programı açacak, "Bana dosya gönderilecek"i</Help>
      <Help>seçecek ve bu kodu yazacak.</Help>
      <Gap />
      <Help>Kod tek kullanımlık: dosyalar gittiği anda geçersiz olur.</Help>
      <Gap />
      {/* The files are read in the background while the code is read out. */}
      <Ok>✓ Dosyalar gönderilmeye hazır</Ok>
      <Gap />
      <ButtonRow buttons={['✓  Arkadaşıma ilettim']} selected={0} />
      <Gap />
      <Foot>Enter ile devam et · Ctrl+C ile çık</Foot>
    </>
  )
}

// ---------------------------------------------------------------------------
// 5 · Waiting
// ---------------------------------------------------------------------------

export function Waiting({ frame }: Ctx) {
  return (
    <>
      <Title>Bekleniyor</Title>
      <Gap />
      <div className="text-tui-fg">
        <Spin frame={frame} /> Arkadaşının kodu girmesi bekleniyor...
      </div>
      <Gap />
      <Body>Kod:</Body>
      <TuiBox lines={[ROOM_CODE]} />
      <Gap />
      <Ok>✓ Dosyalar gönderilmeye hazır</Ok>
      <Gap />
      <Help>Bu pencereyi kapatma. Arkadaşın kodu girdiği anda</Help>
      <Help>gönderme kendiliğinden başlayacak.</Help>
      <Gap />
      <Help>Kod en fazla 1 saat geçerli.</Help>
      <Gap />
      <Foot>Ctrl+C ile vazgeç</Foot>
    </>
  )
}

// ---------------------------------------------------------------------------
// 6 · Enter code (receiver)
// ---------------------------------------------------------------------------

export function EnterCode({ sub }: Ctx) {
  const typed = ROOM_CODE.slice(0, Math.floor(range(sub, 0.05, 0.8) * ROOM_CODE.length))
  return (
    <>
      <Title>📥  Arkadaşının verdiği kodu yaz</Title>
      <Gap />
      <Help>Arkadaşın sana 3 parçalı bir kod verdi.</Help>
      <Help>Şuna benziyor: kiraz-liman-42</Help>
      <Gap />
      <div>
        <span className="text-tui-accent">{'  ➜  '}</span>
        <span className="text-tui-fg">{typed}</span>
        <Caret />
        {typed.length === 0 && <span className="text-tui-faint">{'kiraz-liman-42'.slice(1)}</span>}
      </div>
      <Gap />
      <Foot>Enter ile devam et · Ctrl+C ile çık</Foot>
    </>
  )
}

// ---------------------------------------------------------------------------
// 7 · Finding
// ---------------------------------------------------------------------------

export function Finding({ frame, sub }: Ctx) {
  const status =
    sub < 0.33
      ? 'Kod kontrol ediliyor...'
      : sub < 0.66
        ? 'Arkadaşının bilgisayarına bağlanılıyor...'
        : 'Doğrudan yol açılıyor...'
  return (
    <>
      <Title>Aranıyor</Title>
      <Gap />
      <div className="text-tui-fg">
        <Spin frame={frame} /> {status}
      </div>
      <Gap />
      <Help>İki bilgisayar arasında doğrudan bir yol açılmaya</Help>
      <Help>çalışılıyor. Bu birkaç saniye sürebilir.</Help>
      <Gap />
      <Foot>Ctrl+C ile vazgeç</Foot>
    </>
  )
}

// ---------------------------------------------------------------------------
// 8 · Confirm
// ---------------------------------------------------------------------------

export function Confirm() {
  return (
    <>
      <Title>📥  Sana dosya gönderilmek isteniyor</Title>
      <Gap />
      {DEMO_FILES.map((f) => (
        <div key={f.name}>
          <span className="text-tui-fg">{`  • ${f.name}`}</span>
          <span className="text-tui-muted">{` (${formatBytes(f.size)})`}</span>
        </div>
      ))}
      <Gap />
      <div className="text-tui-muted">{`  Toplam: ${DEMO_FILES.length} dosya, ${formatBytes(DEMO_TOTAL)}`}</div>
      <Gap />
      <Help>Kaydedilecek yer:</Help>
      <div className="text-tui-fg">{`  ${OUT_DIR}`}</div>
      <Gap />
      <Body>Bu dosyaları almak istiyor musun?</Body>
      <Gap />
      <ButtonRow buttons={['Evet, indir', 'Hayır, iptal']} selected={0} />
      <Gap />
      <Foot>← → ile seç · Enter ile onayla · y / n kısayolları</Foot>
    </>
  )
}

// ---------------------------------------------------------------------------
// 9 · Transfer
// ---------------------------------------------------------------------------

export function Transfer({ sub }: Ctx) {
  const moved = Math.min(DEMO_TOTAL, sub * DEMO_TOTAL * 1.05)
  const idx = moved < DEMO_FILES[0].size ? 0 : 1
  const file = DEMO_FILES[idx]
  const done = idx === 0 ? moved : Math.min(file.size, moved - DEMO_FILES[0].size)
  const pct = Math.round((done / file.size) * 100)

  return (
    <>
      <Title>📥  İndiriliyor</Title>
      <Gap />
      <Ok>✓ Doğrudan bağlantı kuruldu</Ok>
      <Help>{'  Dosyalar iki bilgisayar arasında akıyor, kimse aradan geçmiyor.'}</Help>
      <Gap />
      <div className="text-tui-fg">{`  ${file.name}`}</div>
      <div>
        {'  '}
        <ProgressBar done={done} total={file.size} />
        <span className="text-tui-fg">{`  ${String(pct).padStart(3)}%  `}</span>
        <span className="text-tui-muted">
          {`${formatBytes(Math.round(done))} / ${formatBytes(file.size)}`}
        </span>
      </div>
      <Gap />
      <div className="text-tui-muted">{`  Dosya ${idx + 1} / ${DEMO_FILES.length}`}</div>
      <Gap />
      <Foot>Ctrl+C ile vazgeç</Foot>
    </>
  )
}

// ---------------------------------------------------------------------------
// 10 · Done
// ---------------------------------------------------------------------------

export function Done() {
  return (
    <>
      <Ok>✓  İndi!</Ok>
      <Gap />
      <Body>Dosyalar şuraya kaydedildi:</Body>
      <Gap />
      {DEMO_FILES.map((f) => (
        <div key={f.name} className="text-tui-fg">
          {`  • ${OUT_DIR}/${f.name}`}
        </div>
      ))}
      <Gap />
      <Help>Her dosyanın eksiksiz indiği doğrulandı.</Help>
      <Gap />
      <Foot>Enter ile ana menüye dön · q ile çık</Foot>
    </>
  )
}
