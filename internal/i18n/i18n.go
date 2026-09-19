package i18n

import (
	"fmt"
	"strings"

	"puresend/internal/safetext"
)

// Lang represents a supported language.
type Lang string

const (
	TR Lang = "tr"
	EN Lang = "en"
)

// DetectOS detects the operating system language and returns TR or EN.
func DetectOS() Lang {
	loc := strings.ToLower(osLocale())
	if strings.Contains(loc, "tr") {
		return TR
	}
	return EN
}

// Normalize parses a user-provided language string.
func Normalize(s string) Lang {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "tr", "tr_tr", "tr-tr", "turkish", "turkce", "türkçe":
		return TR
	case "en", "en_us", "en-us", "en_gb", "english", "ingilizce":
		return EN
	default:
		return DetectOS()
	}
}

// Toggle returns the alternate language (TR -> EN, EN -> TR).
func Toggle(l Lang) Lang {
	if l == TR {
		return EN
	}
	return TR
}

// Messages contains all localizable strings across the TUI.
type Messages struct {
	// Welcome
	WelcomeHeadline  string
	WelcomeHelp1     string
	WelcomeHelp2     string
	WelcomeQuestion  string
	WelcomeSend      string
	WelcomeRecv      string
	WelcomeChangeDir string
	WelcomeSavingTo  string
	WelcomeFooter    string

	// Connecting
	ConnectingTitle  string
	ConnectingStatus string
	ConnectingHelp1  string
	ConnectingHelp2  string
	ConnectingHelp3  string
	ConnectingFooter string

	// PickFiles
	PickTitle          string
	PickHelp1          string
	PickHelp2          string
	PickHelp3          string
	PickSelected       func(n int) string
	PickFolderFiles    func(count int, size string) string
	PickTotal          func(size string) string
	PickStartBtn       string
	PickEmpty          string
	PickFooterSelected string
	PickFooterEmpty    string

	// OutDir
	OutDirTitle     string
	OutDirHelp1     string
	OutDirHelp2     string
	OutDirHelp3     string
	OutDirCurrent   string
	OutDirSelectBtn func(dir string) string
	OutDirFooter    string

	// RoomCode
	RoomTitle     string
	RoomBody      string
	RoomHelp1     string
	RoomHelp2     string
	RoomHelp3     string
	RoomSingleUse string
	RoomSentBtn   string
	RoomFooter    string

	// Waiting
	WaitingTitle     string
	WaitingStatus    string
	WaitingCodeLabel string
	WaitingHelp1     string
	WaitingHelp2     string
	WaitingHelp3     string
	WaitingFooter    string

	// Hosting notes
	HostReady     string
	HostPreparing func(cur, total int) string
	HostLost      string
	HostLostHelp1 string
	HostLostHelp2 string

	// EnterCode
	EnterTitle       string
	EnterHelp1       string
	EnterHelp2       string
	EnterPlaceholder string
	EnterSavingTo    string
	EnterFooter      string

	// Finding
	FindingTitle     string
	FindingHelp1     string
	FindingHelp2     string
	FindingFooter    string
	StatusLookingUp  string
	StatusConnecting string
	StatusDirect     string
	StatusDefault    string

	// Confirm
	ConfirmTitle     string
	ConfirmMoreFiles func(n int) string
	ConfirmTotal     func(files int, size string) string
	ConfirmDest      string
	ConfirmQuestion  string
	ConfirmYes       string
	ConfirmNo        string
	ConfirmFooter    string
	RelayWarning     func(limit string) string

	// Transfer
	TransferSendTitle  string
	TransferRecvTitle  string
	TransferDirectOk   string
	TransferDirectHelp string
	TransferRelayWarn  string
	TransferRelayHelp1 string
	TransferRelayHelp2 string
	TransferPrepFiles  func(cur, total int) string
	TransferRemotePrep func(cur, total int) string
	TransferPreparing  string
	TransferFileIndex  func(cur, total int) string
	TransferEta        func(eta string) string
	TransferFooter     string

	// Done
	DoneSendTitle string
	DoneSendBody  string
	DoneSendHelp  string
	DoneRecvTitle string
	DoneRecvBody  string
	DoneMoreFiles func(n int) string
	DoneVerified  string
	DoneFooter    string

	// Error
	ErrorTitle   string
	ErrorWhatCan string
	ErrorFooter  string
}

var trMessages = &Messages{
	WelcomeHeadline:  "Dosyalarını arkadaşına doğrudan gönderirsin.",
	WelcomeHelp1:     "Dosyaların hiçbir siteye yüklenmez — senin bilgisayarından",
	WelcomeHelp2:     "çıkar, arkadaşının bilgisayarına iner.",
	WelcomeQuestion:  "Ne yapmak istiyorsun?",
	WelcomeSend:      "📤  Dosya göndereceğim",
	WelcomeRecv:      "📥  Bana dosya gönderilecek",
	WelcomeChangeDir: "📁  İndirme klasörünü değiştir",
	WelcomeSavingTo:  "İnenler şuraya kaydediliyor:",
	WelcomeFooter:    "[↑/↓] Gezin  ·  [Enter] Onayla  ·  [L] Dil: English  ·  [q] Çıkış",

	ConnectingTitle:  "Bağlanılıyor",
	ConnectingStatus: "Buluşma noktasına bağlanılıyor...",
	ConnectingHelp1:  "Bu, iki bilgisayarın birbirini bulmasını sağlayan küçük bir",
	ConnectingHelp2:  "adres defteri. Dosyaların oraya gitmiyor, sadece",
	ConnectingHelp3:  "\"buradayım\" diyorsun.",
	ConnectingFooter: "[Ctrl+C] İptal",

	PickTitle: "📤  Ne göndereceksin?",
	PickHelp1: "Klasörlerin içine girmek için [Enter]'a bas. Göndermek",
	PickHelp2: "istediğin dosyanın üzerinde [Enter]'a basınca listeye eklenir.",
	PickHelp3: "İçinde olduğun klasörün tamamını eklemek için [f]'ye bas.",
	PickSelected: func(n int) string {
		return fmt.Sprintf("Seçtiklerin (%d):", n)
	},
	PickFolderFiles: func(count int, size string) string {
		return fmt.Sprintf("%d dosya, %s", count, size)
	},
	PickTotal: func(size string) string {
		return "  Toplam: " + size
	},
	PickStartBtn:       "[s]  ·  Göndermeye başla",
	PickEmpty:          "Henüz bir şey seçmedin.",
	PickFooterSelected: "[↑/↓] Gezin  ·  [Enter] Aç/Seç  ·  [f] Klasör Ekle  ·  [x] Çıkar  ·  [s] Başlat  ·  [Ctrl+C] Çıkış",
	PickFooterEmpty:    "[↑/↓] Gezin  ·  [Enter] Aç/Seç  ·  [f] Klasör Ekle  ·  [Ctrl+C] Çıkış",

	OutDirTitle:   "📁  İnen dosyalar nereye kaydedilsin?",
	OutDirHelp1:   "• [Enter] : Seçili klasörün içine gir",
	OutDirHelp2:   "• [Backspace] veya [←] : Bir üst klasöre çık",
	OutDirHelp3:   "• [s] : Aşağıda gösterilen klasörü hedef seç",
	OutDirCurrent: "Şu an burası: ",
	OutDirSelectBtn: func(dir string) string {
		return "[s]  ·  Burayı seç (" + dir + ")"
	},
	OutDirFooter: "[↑/↓] Gezin  ·  [Enter] Gir  ·  [Backspace/←] Üst Klasör  ·  [s] Seç  ·  [Esc] Vazgeç",

	RoomTitle:     "🔑  Oda kodun hazır!",
	RoomBody:      "Şimdi arkadaşına bu kodu ilet.",
	RoomHelp1:     "WhatsApp'tan yaz, SMS at ya da telefonda söyle — fark etmez.",
	RoomHelp2:     "Arkadaşın programı açacak, \"Bana dosya gönderilecek\"i",
	RoomHelp3:     "seçecek ve bu kodu yazacak.",
	RoomSingleUse: "Kod tek kullanımlık: dosyalar gittiği anda geçersiz olur.",
	RoomSentBtn:   "✓  Arkadaşıma ilettim",
	RoomFooter:    "[Enter] Devam Et  ·  [L] Dil: English  ·  [Ctrl+C] Çıkış",

	WaitingTitle:     "Bekleniyor",
	WaitingStatus:    "Arkadaşının kodu girmesi bekleniyor...",
	WaitingCodeLabel: "Kod: ",
	WaitingHelp1:     "Bu pencereyi kapatma. Arkadaşın kodu girdiği anda",
	WaitingHelp2:     "gönderme kendiliğinden başlayacak.",
	WaitingHelp3:     "Kod en fazla 1 saat geçerli.",
	WaitingFooter:    "[L] Dil: English  ·  [Ctrl+C] İptal Et",

	HostReady: "✓ Dosyalar gönderilmeye hazır",
	HostPreparing: func(cur, total int) string {
		return fmt.Sprintf("Dosyalar hazırlanıyor (%d/%d)...", cur, total)
	},
	HostLost:      "! Buluşma noktasıyla bağlantı koptu, yeniden bağlanılıyor...",
	HostLostHelp1: "  Kodun geçerliliğini koruyor. Arkadaşın bu arada denerse",
	HostLostHelp2: "  birkaç saniye sonra tekrar denesin.",

	EnterTitle:       "📥  Arkadaşının verdiği kodu yaz",
	EnterHelp1:       "Arkadaşın sana 3 parçalı bir kod verdi.",
	EnterHelp2:       "Şuna benziyor: kiraz-liman-42",
	EnterPlaceholder: "kiraz-liman-42",
	EnterSavingTo:    "İnenler şuraya kaydedilecek:",
	EnterFooter:      "[Enter] Onayla & Bağlan  ·  [Ctrl+O] Klasör Değiştir  ·  [Ctrl+C] Çıkış",

	FindingTitle:     "Aranıyor",
	FindingHelp1:     "İki bilgisayar arasında doğrudan bir yol açılmaya",
	FindingHelp2:     "çalışılıyor. Bu birkaç saniye sürebilir.",
	FindingFooter:    "[Ctrl+C] İptal Et",
	StatusLookingUp:  "Kod kontrol ediliyor...",
	StatusConnecting: "Arkadaşının bilgisayarına bağlanılıyor...",
	StatusDirect:     "Doğrudan yol açılıyor...",
	StatusDefault:    "Arkadaşın aranıyor...",

	ConfirmTitle: "📥  Sana dosya gönderilmek isteniyor",
	ConfirmMoreFiles: func(n int) string {
		return fmt.Sprintf("  ... ve %d dosya daha", n)
	},
	ConfirmTotal: func(files int, size string) string {
		return fmt.Sprintf("  Toplam: %d dosya, %s", files, size)
	},
	ConfirmDest:     "Kaydedilecek yer:",
	ConfirmQuestion: "Bu dosyaları almak istiyor musun?",
	ConfirmYes:      "Evet, indir",
	ConfirmNo:       "Hayır, iptal",
	ConfirmFooter:   "[←/→] Seçim  ·  [Enter] Onayla  ·  [y] Evet, İndir  ·  [n] İptal  ·  [Ctrl+C] Çıkış",
	RelayWarning: func(limit string) string {
		return "! Bu kadarı yedek yoldan geçemez.\n" +
			"  Doğrudan yol açılamadı ve yedek yolun " + limit + " sınırı var;\n" +
			"  transfer büyük ihtimalle yarıda kesilecek. Yarım kalırsa kaldığı\n" +
			"  yerden devam eder — ama önce ikinizin de başka bir ağ denemesi\n" +
			"  (mesela wifi yerine mobil veri) daha hızlı sonuç verir."
	},

	TransferSendTitle:  "📤  Gönderiliyor",
	TransferRecvTitle:  "📥  İndiriliyor",
	TransferDirectOk:   "✓ Doğrudan bağlantı kuruldu",
	TransferDirectHelp: "  Dosyalar iki bilgisayar arasında akıyor, kimse aradan geçmiyor.",
	TransferRelayWarn:  "! Yedek yol kullanılıyor",
	TransferRelayHelp1: "  Doğrudan yol açılamadı (bazı internet bağlantıları buna izin",
	TransferRelayHelp2: "  vermiyor). Transfer yine de şifreli, sadece biraz daha yavaş.",
	TransferPrepFiles: func(cur, total int) string {
		return fmt.Sprintf("Dosyalar kontrol ediliyor (%d/%d)...", cur, total)
	},
	TransferRemotePrep: func(cur, total int) string {
		return fmt.Sprintf("Arkadaşının bilgisayarı dosyaları hazırlıyor (%d/%d)...", cur, total)
	},
	TransferPreparing: "Hazırlanıyor...",
	TransferFileIndex: func(cur, total int) string {
		return fmt.Sprintf("Dosya %d / %d", cur, total)
	},
	TransferEta: func(eta string) string {
		return "kalan süre " + eta
	},
	TransferFooter: "[Ctrl+C] Aktarımı Durdur",

	DoneSendTitle: "✓  Gönderildi!",
	DoneSendBody:  "Dosyaların arkadaşına ulaştı ve eksiksiz indiği doğrulandı.",
	DoneSendHelp:  "Kod artık geçersiz — aynı kodla kimse bir daha indiremez.",
	DoneRecvTitle: "✓  İndi!",
	DoneRecvBody:  "Dosyalar şuraya kaydedildi:",
	DoneMoreFiles: func(n int) string {
		return fmt.Sprintf("  ... ve %d dosya daha", n)
	},
	DoneVerified: "Her dosyanın eksiksiz indiği doğrulandı.",
	DoneFooter:   "[Enter] Ana Menü  ·  [L] Dil: English  ·  [q] Çıkış",

	ErrorTitle:   "✗  Bir sorun çıktı",
	ErrorWhatCan: "Ne yapabilirsin:",
	ErrorFooter:  "[Enter] Ana Menü  ·  [L] Dil: English  ·  [q] Çıkış",
}

var enMessages = &Messages{
	WelcomeHeadline:  "Send files directly to your friend peer-to-peer.",
	WelcomeHelp1:     "Your files are never uploaded to any server — they leave your computer",
	WelcomeHelp2:     "and land directly onto your friend's computer.",
	WelcomeQuestion:  "What would you like to do?",
	WelcomeSend:      "📤  I want to send files",
	WelcomeRecv:      "📥  I want to receive files",
	WelcomeChangeDir: "📁  Change download folder",
	WelcomeSavingTo:  "Downloads are saved to:",
	WelcomeFooter:    "[↑/↓] Navigate  ·  [Enter] Confirm  ·  [L] Language: Türkçe  ·  [q] Quit",

	ConnectingTitle:  "Connecting",
	ConnectingStatus: "Connecting to rendezvous server...",
	ConnectingHelp1:  "This is a lightweight signaling registry connecting peers.",
	ConnectingHelp2:  "Your files do not go through it; you only register your",
	ConnectingHelp3:  "address to find each other.",
	ConnectingFooter: "[Ctrl+C] Cancel",

	PickTitle: "📤  What do you want to send?",
	PickHelp1: "Press [Enter] to open folders. Highlight a file and press [Enter]",
	PickHelp2: "to add it to the transfer list.",
	PickHelp3: "Press [f] to add the entire current folder.",
	PickSelected: func(n int) string {
		return fmt.Sprintf("Selected (%d):", n)
	},
	PickFolderFiles: func(count int, size string) string {
		return fmt.Sprintf("%d files, %s", count, size)
	},
	PickTotal: func(size string) string {
		return "  Total: " + size
	},
	PickStartBtn:       "[s]  ·  Start sending",
	PickEmpty:          "Nothing selected yet.",
	PickFooterSelected: "[↑/↓] Navigate  ·  [Enter] Open/Pick  ·  [f] Add Folder  ·  [x] Remove  ·  [s] Start  ·  [Ctrl+C] Quit",
	PickFooterEmpty:    "[↑/↓] Navigate  ·  [Enter] Open/Pick  ·  [f] Add Folder  ·  [Ctrl+C] Quit",

	OutDirTitle:   "📁  Where should incoming files be saved?",
	OutDirHelp1:   "• [Enter] : Open selected folder",
	OutDirHelp2:   "• [Backspace] or [←] : Go to parent directory",
	OutDirHelp3:   "• [s] : Select the current directory shown below",
	OutDirCurrent: "Current directory: ",
	OutDirSelectBtn: func(dir string) string {
		return "[s]  ·  Select here (" + dir + ")"
	},
	OutDirFooter: "[↑/↓] Navigate  ·  [Enter] Open Folder  ·  [Backspace/←] Parent Dir  ·  [s] Select Dir  ·  [Esc] Cancel",

	RoomTitle:     "🔑  Your room code is ready!",
	RoomBody:      "Now share this room code with your friend.",
	RoomHelp1:     "Send via WhatsApp, SMS, or read aloud over phone — anything works.",
	RoomHelp2:     "Your friend will launch PureSend, choose \"I want to receive\",",
	RoomHelp3:     "and enter this room code.",
	RoomSingleUse: "Single-use: code expires as soon as transfer finishes.",
	RoomSentBtn:   "✓  I shared the code",
	RoomFooter:    "[Enter] Continue  ·  [L] Language: Türkçe  ·  [Ctrl+C] Quit",

	WaitingTitle:     "Waiting",
	WaitingStatus:    "Waiting for your friend to enter the code...",
	WaitingCodeLabel: "Code: ",
	WaitingHelp1:     "Keep this window open. As soon as your friend enters the code,",
	WaitingHelp2:     "the transfer will begin automatically.",
	WaitingHelp3:     "Code is valid for up to 1 hour.",
	WaitingFooter:    "[L] Language: Türkçe  ·  [Ctrl+C] Cancel",

	HostReady: "✓ Files are ready to send",
	HostPreparing: func(cur, total int) string {
		return fmt.Sprintf("Preparing files (%d/%d)...", cur, total)
	},
	HostLost:      "! Connection to rendezvous dropped, reconnecting...",
	HostLostHelp1: "  Your code remains valid. If your friend tries now,",
	HostLostHelp2: "  have them retry in a few seconds.",

	EnterTitle:       "📥  Enter the code from your friend",
	EnterHelp1:       "Your friend generated a 3-word room code for you.",
	EnterHelp2:       "It looks like: cherry-harbor-42",
	EnterPlaceholder: "cherry-harbor-42",
	EnterSavingTo:    "Incoming files will be saved to:",
	EnterFooter:      "[Enter] Confirm & Connect  ·  [Ctrl+O] Change Folder  ·  [Ctrl+C] Quit",

	FindingTitle:     "Connecting",
	FindingHelp1:     "Attempting direct peer-to-peer connection between both computers.",
	FindingHelp2:     "This usually takes a few seconds.",
	FindingFooter:    "[Ctrl+C] Cancel",
	StatusLookingUp:  "Looking up room code...",
	StatusConnecting: "Connecting to your friend's computer...",
	StatusDirect:     "Establishing direct P2P route...",
	StatusDefault:    "Looking for friend...",

	ConfirmTitle: "📥  Incoming file transfer request",
	ConfirmMoreFiles: func(n int) string {
		return fmt.Sprintf("  ... and %d more files", n)
	},
	ConfirmTotal: func(files int, size string) string {
		return fmt.Sprintf("  Total: %d files, %s", files, size)
	},
	ConfirmDest:     "Save destination:",
	ConfirmQuestion: "Do you want to accept these files?",
	ConfirmYes:      "Yes, download",
	ConfirmNo:       "No, cancel",
	ConfirmFooter:   "[←/→] Select  ·  [Enter] Confirm  ·  [y] Yes, Download  ·  [n] Cancel  ·  [Ctrl+C] Quit",
	RelayWarning: func(limit string) string {
		return "! Transfer size exceeds relay fallback limit.\n" +
			"  Direct P2P could not be opened and relay route has a " + limit + " limit;\n" +
			"  the transfer will likely be interrupted. If interrupted, it can resume,\n" +
			"  but trying another network (e.g. mobile hotspot) will achieve direct P2P."
	},

	TransferSendTitle:  "📤  Sending",
	TransferRecvTitle:  "📥  Downloading",
	TransferDirectOk:   "✓ Direct P2P connection established",
	TransferDirectHelp: "  Files stream directly between computers without intermediate servers.",
	TransferRelayWarn:  "! Using encrypted relay route",
	TransferRelayHelp1: "  Direct connection could not be opened (NAT/firewall restriction).",
	TransferRelayHelp2: "  Transfer is still end-to-end encrypted, just slightly slower.",
	TransferPrepFiles: func(cur, total int) string {
		return fmt.Sprintf("Checking files (%d/%d)...", cur, total)
	},
	TransferRemotePrep: func(cur, total int) string {
		return fmt.Sprintf("Friend's computer is preparing files (%d/%d)...", cur, total)
	},
	TransferPreparing: "Preparing...",
	TransferFileIndex: func(cur, total int) string {
		return fmt.Sprintf("File %d / %d", cur, total)
	},
	TransferEta: func(eta string) string {
		return "remaining " + eta
	},
	TransferFooter: "[Ctrl+C] Stop Transfer",

	DoneSendTitle: "✓  Sent!",
	DoneSendBody:  "Your files reached your friend and are verified intact.",
	DoneSendHelp:  "Code is now expired — no one can download again with this code.",
	DoneRecvTitle: "✓  Downloaded!",
	DoneRecvBody:  "Files saved to:",
	DoneMoreFiles: func(n int) string {
		return fmt.Sprintf("  ... and %d more files", n)
	},
	DoneVerified: "Every file has been verified intact with SHA-256.",
	DoneFooter:   "[Enter] Main Menu  ·  [L] Language: Türkçe  ·  [q] Quit",

	ErrorTitle:   "✗  Something went wrong",
	ErrorWhatCan: "What you can do:",
	ErrorFooter:  "[Enter] Main Menu  ·  [L] Language: Türkçe  ·  [q] Quit",
}

// Get returns the localized messages for the given language.
func Get(lang Lang) *Messages {
	if lang == EN {
		return enMessages
	}
	return trMessages
}

// Explain turns an error into a localized headline and actionable hints.
func Explain(err error, lang Lang) (string, []string) {
	if err == nil {
		if lang == EN {
			return "An unknown error occurred.", nil
		}
		return "Bilinmeyen bir hata oldu.", nil
	}

	s := strings.ToLower(err.Error())
	has := func(parts ...string) bool {
		for _, p := range parts {
			if strings.Contains(s, p) {
				return true
			}
		}
		return false
	}

	if lang == EN {
		retryTogether := []string{
			"Both peers should keep the app open and retry with the same code — transfer resumes where it left off.",
			"If direct route failed, relay has a size limit; large files might hit this limit.",
			"If possible, switch to another network (e.g. mobile hotspot instead of restricted Wi-Fi).",
		}

		switch {
		case has("room code does not match"):
			return "Room code does not match.", []string{
				"Make sure you typed the code character for character.",
				"Ask your friend to read the code again — every letter matters.",
				"Code is single-use: if already used, ask for a new code.",
			}
		case has("already sending"):
			return "Another transfer is currently in progress with this code.", []string{
				"Ask your friend what appears on their screen — files might be going to someone else.",
				"If they didn't share the code with anyone else, have them start fresh with a new code.",
			}
		case has("not a room code", "malformed room code"):
			return "This is not a valid room code.", []string{
				"A room code consists of two words and a number, e.g. cherry-harbor-42.",
			}
		case has("not used in room codes"):
			return "Unrecognized word in room code.", []string{
				"Ask your friend for the code spelling again.",
			}
		case has("room code expired"):
			return "Room code expired.", []string{
				"Start a new transfer to obtain a fresh code.",
			}
		case has("reconnecting"):
			return "Your friend's client is reconnecting to the meeting point.", []string{
				"Wait a few seconds and try with the same code again.",
			}
		case has("sender could not prepare"):
			return "Your friend's computer could not read the files.", []string{
				"Ask your friend to check that the files exist and start a new transfer.",
			}
		case has("invalid file list", "message exceeds"):
			return "The other side sent an invalid file list.", []string{
				"Transfer halted for safety; nothing was written to disk.",
			}
		case has("cannot store"):
			return "An incoming file name cannot be used on this computer.", []string{
				"Ask your friend to remove special characters like ? * < > | \" from file names.",
				"Names like CON, NUL, AUX are also reserved on Windows.",
			}
		case has("unsafe file name", "reserved file name"):
			return "The other side sent an unacceptable file name.", []string{
				"Transfer halted for safety; nothing was written to disk.",
				"Ask your friend to rename the file and try again.",
			}
		case has("stopped responding"):
			return "The other side stopped responding.", retryTogether
		case has("connection lost", "stream reset", "connection closed", "acknowledgement",
			"no answer from receiver", "could not read manifest", "security handshake",
			"no confirmation from the other side"):
			return "Connection dropped during transfer.", retryTogether
		case has("too many failed lookups"):
			return "Too many invalid code attempts.", []string{
				"Ask your friend for the code again and type carefully.",
				"Wait a minute and try again from the main menu.",
			}
		case has("too many open rooms"):
			return "Too many active send transfers.", []string{
				"Close one of the open transfer windows and try again.",
			}
		case has("server is full", "server is busy"):
			return "The meeting point is currently busy.", []string{
				"Please try again in a few minutes.",
			}
		case has("already in use"):
			return "This code is currently in use by someone else.", []string{
				"Try again — a new code will be generated.",
			}
		case has("not found", "room"):
			return "No room found with this code.", []string{
				"Make sure the code is typed correctly (three parts with dashes).",
				"Your friend's app must still be open and waiting.",
				"Codes are single-use and valid for 1 hour; request a new code if needed.",
			}
		case has("meeting point", "all rendezvous servers unreachable", "could not connect to any rendezvous"):
			return "Could not reach the meeting point.", []string{
				"Check your internet connection.",
				"If your firewall blocks WebSocket/P2P traffic, try another network.",
			}
		case has("could not connect to the other computer"):
			return "Could not connect to your friend's computer.", []string{
				"Check if your friend's app is still open and waiting.",
				"Try again — connection often succeeds on the second attempt.",
			}
		case has("declined"):
			return "Transfer declined by receiver.", nil
		case has("checksum"):
			return "File download was incomplete or corrupted.", []string{
				"Restart the transfer; corrupted file was not saved to disk.",
			}
		case has("too many files"):
			return "Too many files to send in one batch.", []string{
				"Split the folder into smaller batches and send separately.",
			}
		case has("no files", "not an ordinary file"):
			return "Nothing selected to send.", []string{
				"Select at least one file or non-empty folder from the list.",
			}
		case has("could not read", "could not open file"):
			return "Could not read one of the selected files.", []string{
				"Check that the file exists and you have permission to read it.",
			}
		case has("no space left", "could not write to disk"):
			return "Could not write to disk.", []string{
				"Check if there is enough free disk space at destination.",
				"You can select another download folder from the main menu.",
			}
		case has("relay limit exceeded", "circuit relay"):
			return "Transfer exceeded relay size limit.", retryTogether
		default:
			return safetext.Clean(err.Error(), 300), []string{"Press Enter to try again."}
		}
	}

	// Turkish fallback
	retryTogether := []string{
		"İkiniz de programı açık tutup aynı kodla tekrar deneyin — indirme kaldığı yerden devam eder.",
		"Doğrudan yol açılamadıysa yedek yolun bir boyut sınırı var; büyük dosyalarda bu sınıra takılmış olabilirsiniz.",
		"Mümkünse ikiniz de başka bir ağa geçin (wifi yerine mobil veri gibi).",
	}

	switch {
	case has("room code does not match"):
		return "Kod eşleşmedi.", []string{
			"Kodu harfi harfine doğru yazdığından emin ol.",
			"Arkadaşın sana kodu yeniden okusun — bir harf bile fark eder.",
			"Kod tek kullanımlık: daha önce kullanıldıysa yenisini istemen gerekir.",
		}
	case has("already sending"):
		return "Bu kodla şu an başka bir transfer sürüyor.", []string{
			"Arkadaşına ekranında ne yazdığını sor — dosyalar başka birine gidiyor olabilir.",
			"Kodu senden başka kimseye vermediyse, yeni bir kodla baştan başlasın.",
		}
	case has("not a room code", "malformed room code"):
		return "Bu bir oda kodu değil.", []string{
			"Kod iki kelime ve bir sayıdan oluşur, örneğin kiraz-liman-42.",
		}
	case has("not used in room codes"):
		return "Kodda tanınmayan bir kelime var.", []string{
			"Kodu arkadaşından harf harf yeniden iste.",
		}
	case has("room code expired"):
		return "Kodun süresi doldu.", []string{
			"Yeni bir gönderim başlat; yeni bir kod alırsın.",
		}
	case has("reconnecting"):
		return "Arkadaşının programı buluşma noktasına yeniden bağlanıyor.", []string{
			"Birkaç saniye bekleyip aynı kodla tekrar dene.",
		}
	case has("sender could not prepare"):
		return "Arkadaşının bilgisayarı dosyaları okuyamadı.", []string{
			"Arkadaşın dosyaların yerinde durduğunu kontrol edip baştan göndersin.",
		}
	case has("invalid file list", "message exceeds"):
		return "Karşı taraf geçersiz bir dosya listesi gönderdi.", []string{
			"Güvenlik için transfer durduruldu, diske hiçbir şey yazılmadı.",
		}
	case has("cannot store"):
		return "Gelen dosyalardan birinin adı bu bilgisayarda kullanılamıyor.", []string{
			"Arkadaşın dosyanın adından ? * < > | \" gibi işaretleri çıkarıp tekrar göndersin.",
			"CON, NUL, AUX gibi adlar da Windows'ta kullanılamaz.",
		}
	case has("unsafe file name", "reserved file name"):
		return "Karşı taraf kabul edilemez bir dosya adı gönderdi.", []string{
			"Güvenlik için transfer durduruldu, diske hiçbir şey yazılmadı.",
			"Arkadaşın dosyanın adını değiştirip tekrar denesin.",
		}
	case has("stopped responding"):
		return "Karşı taraf yanıt vermeyi bıraktı.", retryTogether
	case has("connection lost", "stream reset", "connection closed", "acknowledgement",
		"no answer from receiver", "could not read manifest", "security handshake",
		"no confirmation from the other side"):
		return "Bağlantı transfer sırasında koptu.", retryTogether
	case has("too many failed lookups"):
		return "Çok fazla hatalı kod denendi.", []string{
			"Kodu arkadaşından yeniden iste ve dikkatle yaz.",
			"Bir dakika bekleyip ana menüden tekrar dene.",
		}
	case has("too many open rooms"):
		return "Aynı anda çok fazla gönderim başlattın.", []string{
			"Açık kalan pencerelerden birini kapatıp tekrar dene.",
		}
	case has("server is full", "server is busy"):
		return "Buluşma noktası şu an çok yoğun.", []string{
			"Birkaç dakika sonra tekrar dene.",
		}
	case has("already in use"):
		return "Bu kod şu an başkası tarafından kullanılıyor.", []string{
			"Tekrar dene — yeni bir kod üretilecek.",
		}
	case has("not found", "room"):
		return "Bu kodla açılmış bir oda bulunamadı.", []string{
			"Kodu doğru yazdığından emin ol (üç parça, aralarında tire).",
			"Arkadaşının programı hâlâ açık ve bekliyor olmalı.",
			"Kod tek kullanımlık ve en fazla 1 saat geçerli; kullanıldıysa yeni kod istesin.",
		}
	case has("meeting point", "all rendezvous servers unreachable", "could not connect to any rendezvous"):
		return "Buluşma noktasına ulaşılamadı.", []string{
			"İnternet bağlantını kontrol et.",
			"Güvenlik duvarın engelliyor olabilir, başka bir ağdan dene.",
		}
	case has("could not connect to the other computer"):
		return "Arkadaşının bilgisayarına bağlanılamadı.", []string{
			"Arkadaşının programı açık ve bekliyor durumda mı, sor.",
			"İkiniz de tekrar deneyin — çoğu zaman ikinci denemede olur.",
		}
	case has("declined"):
		return "Karşı taraf transferi kabul etmedi.", nil
	case has("checksum"):
		return "Dosya eksik veya bozuk indi.", []string{
			"Transferi tekrar başlatın; bozuk dosya diske kaydedilmedi.",
		}
	case has("too many files"):
		return "Tek seferde gönderilemeyecek kadar çok dosya var.", []string{
			"Klasörü birkaç parçaya bölüp ayrı ayrı gönderin.",
		}
	case has("no files", "not an ordinary file"):
		return "Gönderilecek bir şey seçilmedi.", []string{
			"Listeden en az bir dosya ya da içi dolu bir klasör seç.",
		}
	case has("could not read", "could not open file"):
		return "Seçtiğin dosyalardan biri okunamadı.", []string{
			"Dosya yerinde duruyor mu ve açma iznin var mı, kontrol et.",
		}
	case has("no space left", "could not write to disk"):
		return "Diske yazılamadı.", []string{
			"Kaydedilecek yerde yeterli boş alan var mı, bak.",
			"Ana menüden başka bir indirme klasörü seçebilirsin.",
		}
	case has("relay limit exceeded", "circuit relay"):
		return "Dosya boyutu yedek yol sınırını aştı.", retryTogether
	default:
		return safetext.Clean(err.Error(), 300), []string{"Tekrar denemek için Enter'a bas."}
	}
}
