package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode"

	"github.com/agnivade/levenshtein"
	_ "github.com/glebarez/go-sqlite"
	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

// =============================================================================
//  KONFIGURASI
// =============================================================================

var (
	LiteLLMBaseURL       string
	LiteLLMModelRingan   string
	LiteLLMModelKompleks string
	LiteLLMModelPro      string

	RoomURLMap        string
	InfoHotelFallback string

	BookingURLBase    string
	BookingPropertyID string
	BookingRoomIDs    string
	BookingGsID       string

	KontakReservasi string
	KontakSales     string
	KontakFO        string
	KontakResto     string

	AIRetryMaxAttempts  int
	AIRetryInitialDelay time.Duration
	AIRetryMaxDelay     time.Duration
)

// =============================================================================
//  TIPE DATA
// =============================================================================

// TurnChat menyimpan satu turn percakapan (user atau model).
type TurnChat struct {
	Role string // "user" atau "model"
	Teks string
}

// SesiChat menyimpan riwayat percakapan per nomor WA.
type SesiChat struct {
	Riwayat      []TurnChat
	Mu           sync.Mutex
	LastActivity time.Time
}


// ProfilTamu menyimpan data personalisasi tamu.
type ProfilTamu struct {
	NomorWA         string
	Nama            string
	KunjunganKe     int
	KamarFavorit    string
	TerakhirCheckin time.Time
	StatusVIP       bool
}

// Template sorting helper
type templateDenganPanjang struct {
	teks  string
	nKata int
}

// [GANTI #3] Response struct — OpenAI-compatible format (digunakan KoboLLM)
type KoboLLMResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// =============================================================================
//  VARIABEL GLOBAL
// =============================================================================

var clientWA *whatsmeow.Client
var startupTime time.Time

// =============================================================================
//  GRACEFUL SHUTDOWN — STATE
// =============================================================================
//
// appCtx/appCancel  : context utama aplikasi. Dibatalkan saat proses shutdown
//                      dimulai, sehingga goroutine background (mis. session
//                      cleanup) bisa berhenti dengan rapi.
// activeWorkers     : WaitGroup yang melacak SEMUA goroutine "kerja" yang
//                      menyentuh resource bersama (DB, HTTP client), termasuk
//                      goroutine per-pesan (handleMessage) dan goroutine
//                      background lainnya. main() menunggu WaitGroup ini
//                      sebelum menutup koneksi database.
// isShuttingDown    : flag atomic. Jika true, eventHandler tidak lagi
//                      menerima/memproses pesan WhatsApp baru.
// cleanupTicker     : referensi ticker session cleanup agar bisa di-Stop()
//                      secara eksplisit saat shutdown.
// ShutdownTimeout   : batas waktu maksimum menunggu goroutine aktif selesai
//                      sebelum shutdown dipaksa lanjut (mencegah hang selamanya).
// =============================================================================

const ShutdownTimeout = 30 * time.Second

var (
	appCtx    context.Context
	appCancel context.CancelFunc

	activeWorkers sync.WaitGroup

	isShuttingDown atomic.Bool

	cleanupTicker *time.Ticker
)

// =============================================================================
//  OWNERSHIP STATE MACHINE
// =============================================================================

const (
	StateBot          = "BOT"
	StateHuman        = "HUMAN"
	StateWaitingHuman = "WAITING_HUMAN"
)

// TTLOwner* dan OwnerCleanupInterval dikonfigurasi dari Config Manager (FASE 4B.5).
var (
	TTLOwnerBot          time.Duration
	TTLOwnerWaitingHuman time.Duration
	TTLOwnerHuman        time.Duration
	OwnerCleanupInterval time.Duration
)

var HumanResponseTimeout time.Duration

// ConversationOwner menyimpan state kepemilikan percakapan per nomor WA.
type ConversationOwner struct {
	State               string
	LastHumanReply      time.Time
	LastCustomerMessage time.Time
	LastTouched    		time.Time
	WaitingSince        time.Time
}

var (
	conversationOwners = make(map[string]*ConversationOwner)
	ownerMutex         sync.RWMutex
)

// --- Conversation Memory ---
var (
	sesiChatMap      = make(map[string]*SesiChat)
	sesiChatMutex    sync.RWMutex
	sesiChatExpiry   time.Duration // Cleanup sesi yang tidak aktif (Dimuat dari config)
)

// --- Per-user processing guard (cegah double-processing spam) ---
var (
	processingMap   = make(map[string]bool)
	processingMutex sync.Mutex
)

// --- Guest Database ---
var dbTamu *sql.DB

// --- Guest Profile Mutex (prevent race condition on profile access) ---
var profilTamuMutex sync.RWMutex

// --- Pre-sorted template list ---
var templateTersortir []templateDenganPanjang

// --- Reusable HTTP client ---
var httpClientGemini *http.Client // untuk panggilan KoboLLM API


func startHealthCheckServer() {
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("Bot is running"))
    })
    go http.ListenAndServe(":"+port, nil)
}

// =============================================================================
//  INIT — Pre-sort template, init HTTP clients
// =============================================================================

func init() {
	// Memuat konfigurasi dari Config Manager
	LoadConfig()

	// Mapping Config ke Variabel Global
	LiteLLMBaseURL = AppConfig.LiteLLMBaseURL
	LiteLLMModelRingan = AppConfig.LiteLLMModelRingan
	LiteLLMModelKompleks = AppConfig.LiteLLMModelKompleks
	LiteLLMModelPro = AppConfig.LiteLLMModelPro

	RoomURLMap = AppConfig.RoomURLMap
	InfoHotelFallback = AppConfig.InfoHotelFallback

	BookingURLBase = AppConfig.BookingURLBase
	BookingPropertyID = AppConfig.BookingPropertyID
	BookingRoomIDs = AppConfig.BookingRoomIDs
	BookingGsID = AppConfig.BookingGsID

	KontakReservasi = AppConfig.KontakReservasi
	KontakSales = AppConfig.KontakSales
	KontakFO = AppConfig.KontakFO
	KontakResto = AppConfig.KontakResto

	HumanResponseTimeout = AppConfig.HumanResponseTimeout
	sesiChatExpiry = AppConfig.SesiChatExpiry

	// Mapping Retry
	AIRetryMaxAttempts = AppConfig.AIRetryMaxAttempts
	AIRetryInitialDelay = AppConfig.AIRetryInitialDelay
	AIRetryMaxDelay = AppConfig.AIRetryMaxDelay

	// Mapping Owner TTL (FASE 4B.5)
	TTLOwnerBot = AppConfig.OwnerTTLBot
	TTLOwnerHuman = AppConfig.OwnerTTLHuman
	TTLOwnerWaitingHuman = AppConfig.OwnerTTLWaitingHuman
	OwnerCleanupInterval = AppConfig.OwnerCleanupInterval

	daftarTemplate := []string{
		// --- Bahasa Indonesia ---
		"mau reservasi kamar",
		"reservasi kamar",
		"reservasi room",
		"booking room",
		"saya mau sewa kamar min",
		"saya mau sewa kamar",
		"ada kamar apa saja",
		"ada tipe kamar apa saja",
		"mau booking kamar",
		"cara booking kamar",
		"pesan kamar",
		"mau reservasi",
		"mau booking",
		"mau pesan kamar",
		"mau sewa kamar",
		"reservasi dong",
		"booking dong",
		"mau book",
		"pengen reservasi",
		"pengen booking",
		"ingin reservasi",
		"ingin booking",
		"selamat pagi",
		"selamat siang",
		"selamat sore",
		"selamat malam",
		"pagi",
		"siang",
		"sore",
		"malam",
		"halo admin",
		"halo",
		"hai",
		"ping",
		"pak",
		"kak",
		"mas",
		"mba",
		"min",
		"bu",
		"p",
		// --- Salam Keagamaan (diolah AI, bukan static) ---
		"assalamualaikum",
		"assalamu alaikum",
		"assalamu'alaikum",
		"waalaikumsalam",
		"wa alaikumsalam",
		"wa'alaikumsalam",
		"om swastiastu",
		"shalom",
		"namo buddhaya",
		"rahayu",
		"salam sejahtera",
		"selamat sejahtera",
		// --- English ---
		"book a room",
		"room booking",
		"room reservation",
		"i want to book",
		"how to book",
		"check availability",
		"available rooms",
		"i want to reserve",
		"make a reservation",
		"good morning",
		"good afternoon",
		"good evening",
		"good night",
		"hello",
		"hi there",
		"hi",
	}
	for _, t := range daftarTemplate {
		templateTersortir = append(templateTersortir,
			templateDenganPanjang{teks: t, nKata: len(strings.Fields(t))})
	}
	sort.Slice(templateTersortir, func(i, j int) bool {
		return templateTersortir[i].nKata > templateTersortir[j].nKata
	})

	httpClientGemini = &http.Client{Timeout: 45 * time.Second}
}

// =============================================================================
//  HELPER: API Key
// =============================================================================

func geminiAPIKey() string {
	return AppConfig.GeminiAPIKey
}

// generateBookingURL membuat URL booking Swiftbook dengan tanggal check-in hari ini
// dan check-out besok secara otomatis.
func generateBookingURL() string {
    now := time.Now()
    checkIn := now.Format("2006-01-02")
    checkOut := now.AddDate(0, 0, 1).Format("2006-01-02")
    
    // Ambil base URL dari environment variable atau gunakan default fallback
    baseURL := getEnv("BOOKING_URL_BASE", "https://www.tiket.com/id-id/hotel/indonesia/kadena-glamping-dive-resort-410001635521096877")

    // Masukkan baseURL ke dalam format string
    return fmt.Sprintf(
        "%s?propertyId=%s&checkIn=%s&checkOut=%s&clientWidth=1351&JDRN=Y&RoomID=%s&noofrooms=1&adult0=2&gsId=%s",
        baseURL, BookingPropertyID, checkIn, checkOut, BookingRoomIDs, BookingGsID,
    )
}

// pesanHubungiDepartemen mengembalikan pesan arahan ke departemen yang tepat.
func pesanHubungiDepartemen(departemen, namaPanggil string) string {
	switch departemen {
	case "reservasi":
		return fmt.Sprintf(
			"Untuk informasi reservasi lebih lanjut atau bantuan langsung, Bapak/Ibu %s dapat menghubungi tim Reservasi kami:\n\n"+
				"📞 WhatsApp Reservasi: https://wa.me/%s\n\n"+
				"Tim kami siap membantu Bapak/Ibu! 🙏",
			namaPanggil, KontakReservasi)
	case "sales":
		return fmt.Sprintf(
			"Untuk kebutuhan event, kerja sama, atau penawaran khusus, Bapak/Ibu %s dapat menghubungi tim Sales kami:\n\n"+
				"📞 WhatsApp Sales: https://wa.me/%s\n\n"+
				"Tim Sales kami siap memberikan penawaran terbaik! 🙏",
			namaPanggil, KontakSales)
	case "fo":
		return fmt.Sprintf(
			"Untuk pertanyaan umum atau bantuan check-in/check-out, Bapak/Ibu %s dapat menghubungi Layanan Tamu (Guest Services) kami:\n\n"+
				"📞 WhatsApp Guest Services: https://wa.me/%s\n\n"+
				"Tim Layanan Tamu kami siap membantu 24 jam! 🙏",
			namaPanggil, KontakFO)
	case "resto":
    return fmt.Sprintf(
        "Untuk reservasi meja, menu, atau informasi restoran, Bapak/Ibu %s dapat menghubungi tim Restoran kami:\n\n"+
				"📞 WhatsApp Restoran: https://wa.me/%s\n\n"+
				"Tim restoran kami siap menyambut Bapak/Ibu! 🍽️🙏",
			namaPanggil, KontakResto)
	default:
		return fmt.Sprintf(
			"Izinkan kami menghubungkan Bapak/Ibu %s dengan tim yang tepat. Silakan pilih:\n\n"+
				"📋 Reservasi Kamar: https://wa.me/%s\n"+
				"💼 Sales & Event: https://wa.me/%s\n"+
				"🏨 Guest Services: https://wa.me/%s\n"+
				"🍽️ Restoran: https://wa.me/%s",
			namaPanggil, KontakReservasi, KontakSales, KontakFO, KontakResto)
	}
}


// sapaanFormal mengembalikan sapaan formal dengan awalan Bapak/Ibu.
// Jika nama kosong → "Bapak/Ibu". Jika ada nama → "Bapak/Ibu <nama>".
func sapaanFormal(nama string) string {
    if strings.TrimSpace(nama) == "" {
        return "Bapak/Ibu"
    }
    return "Bapak/Ibu " + nama
}
// =============================================================================
//  GUEST DATABASE — Profil Tamu untuk Personalisasi
// =============================================================================

// cleanupSesiChatMembers membersihkan sesi chat yang sudah expired (tidak aktif > 24 jam)
func cleanupSesiChatMembers() {
	sesiChatMutex.Lock()
	defer sesiChatMutex.Unlock()

	now := time.Now()
	deleted := 0
	for nomor, sesi := range sesiChatMap {
		// Amankan pembacaan data sesi secara individual (thread-safe)
		sesi.Mu.Lock()
		lastAct := sesi.LastActivity
		riwayatLen := len(sesi.Riwayat)
		sesi.Mu.Unlock()

		// Hapus sesi jika riwayat kosong ATAU melebihi batas waktu expiry
		if riwayatLen == 0 {
			delete(sesiChatMap, nomor)
			deleted++
		} else if now.Sub(lastAct) > sesiChatExpiry {
			delete(sesiChatMap, nomor)
			deleted++
		}
	}

	if deleted > 0 {
		fmt.Printf("[CLEANUP] %d sesi chat dihapus (idle/kosong).\n", deleted)
	}
}

// cleanupConversationOwners menerapkan logika TTL pada map conversationOwners.
// Dipanggil oleh cleanup worker setiap tick, setelah cleanupSesiChatMembers().
func cleanupConversationOwners() {
	applyConversationOwnerTTL()
}

// startSessionCleanup menjalankan cleanup routine setiap 24 jam.
// Routine ini berhenti secara otomatis saat appCtx dibatalkan (graceful shutdown).
func startSessionCleanup() {
	cleanupTicker = time.NewTicker(OwnerCleanupInterval)

	activeWorkers.Add(1)
	go func() {
		defer activeWorkers.Done()
		for {
			select {
			case <-cleanupTicker.C:
				cleanupSesiChatMembers()
				cleanupConversationOwners()
			case <-appCtx.Done():
				fmt.Println("[CLEANUP] Session cleanup routine dihentikan (shutdown).")
				return
			}
		}
	}()
	fmt.Printf("[INIT] Session cleanup routine aktif (setiap %s).\n", OwnerCleanupInterval)
}

func initDatabaseTamu() error {
	db, err := sql.Open("sqlite", "file:guest_profiles.db?_pragma=foreign_keys(1)")
	if err != nil {
		return fmt.Errorf("gagal buka database tamu: %w", err)
	}
	dbTamu = db

	query := `CREATE TABLE IF NOT EXISTS tamu (
		nomor_wa TEXT PRIMARY KEY,
		nama TEXT NOT NULL DEFAULT '',
		kunjungan_ke INTEGER NOT NULL DEFAULT 0,
		kamar_favorit TEXT,
		terakhir_checkin TEXT,
		status_vip INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);`
	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("gagal init tabel tamu: %w", err)
	}

	queryHandover := `CREATE TABLE IF NOT EXISTS handover_sessions (
		nomor_wa TEXT PRIMARY KEY,
		aktif INTEGER NOT NULL DEFAULT 0,
		mulai_at TEXT NOT NULL,
		selesai_at TEXT
	);`
	if _, err := db.Exec(queryHandover); err != nil {
		return fmt.Errorf("gagal init tabel handover_sessions: %w", err)
	}

	queryChatHistory := `CREATE TABLE IF NOT EXISTS chat_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nomor_wa TEXT NOT NULL,
		role TEXT NOT NULL,
		teks TEXT NOT NULL,
		created_at TEXT NOT NULL
	);`
	if _, err := db.Exec(queryChatHistory); err != nil {
		return fmt.Errorf("gagal init tabel chat_history: %w", err)
	}

	queryChatHistoryIdx := `CREATE INDEX IF NOT EXISTS idx_chat_nomor ON chat_history(nomor_wa, id);`
	if _, err := db.Exec(queryChatHistoryIdx); err != nil {
		return fmt.Errorf("gagal init index chat_history: %w", err)
	}

	// Migrasi: pastikan role lama "cs" diganti ke "guest_services"
	if _, err := db.Exec(`UPDATE chat_history SET role = 'guest_services' WHERE role = 'cs'`); err != nil {
		fmt.Printf("[WARN] Migrasi role chat_history gagal (bisa diabaikan jika DB baru): %v\n", err)
	}

	return nil
}
func getOrCreateProfilTamu(nomorWA string) (*ProfilTamu, error) {
	if isShuttingDown.Load() {
		return nil, fmt.Errorf("shutdown in progress")
	}
	row := dbTamu.QueryRow(
		`SELECT nomor_wa, nama, kunjungan_ke, kamar_favorit, terakhir_checkin, status_vip 
		 FROM tamu WHERE nomor_wa = ?`, nomorWA)

	var p ProfilTamu
	var kamarFavoritNull sql.NullString
	var checkinNull sql.NullString
	var vipInt int
	err := row.Scan(&p.NomorWA, &p.Nama, &p.KunjunganKe, &kamarFavoritNull, &checkinNull, &vipInt)
	if err == sql.ErrNoRows {
		now := time.Now().UTC().Format(time.RFC3339)
		_, err := dbTamu.Exec(
			`INSERT INTO tamu (nomor_wa, nama, kunjungan_ke, status_vip, created_at, updated_at) 
			 VALUES (?, '', 0, 0, ?, ?)`,
			nomorWA, now, now)
		if err != nil {
			return nil, fmt.Errorf("gagal insert tamu baru: %w", err)
		}
		return &ProfilTamu{NomorWA: nomorWA, KunjunganKe: 0}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gagal query tamu: %w", err)
	}
	p.StatusVIP = vipInt == 1
	p.KamarFavorit = kamarFavoritNull.String // "" jika NULL
	if checkinNull.Valid && checkinNull.String != "" {
		p.TerakhirCheckin, _ = time.Parse(time.RFC3339, checkinNull.String)
	}
	return &p, nil
}

func updateNamaTamu(nomorWA, nama string) error {
	if isShuttingDown.Load() {
		return fmt.Errorf("shutdown in progress")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := dbTamu.Exec(
		`UPDATE tamu SET nama = ?, updated_at = ? WHERE nomor_wa = ?`,
		nama, now, nomorWA)
	return err
}

func updateKamarFavorit(nomorWA, kamar string) error {
	if isShuttingDown.Load() {
		return fmt.Errorf("shutdown in progress")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := dbTamu.Exec(
		`UPDATE tamu SET kamar_favorit = ?, updated_at = ? WHERE nomor_wa = ?`,
		kamar, now, nomorWA)
	return err
}

func incrementKunjungan(nomorWA string) error {
	if isShuttingDown.Load() {
		return fmt.Errorf("shutdown in progress")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := dbTamu.Exec(
		`UPDATE tamu SET kunjungan_ke = kunjungan_ke + 1, terakhir_checkin = ?, updated_at = ? 
		 WHERE nomor_wa = ?`,
		now, now, nomorWA)
	return err
}

func updateStatusVIP(nomorWA string, vip bool) error {
	if isShuttingDown.Load() {
		return fmt.Errorf("shutdown in progress")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	vipInt := 0
	if vip {
		vipInt = 1
	}
	_, err := dbTamu.Exec(
		`UPDATE tamu SET status_vip = ?, updated_at = ? WHERE nomor_wa = ?`,
		vipInt, now, nomorWA)
	return err
}

// =============================================================================
//  DB HELPERS — Handover Persistence
// =============================================================================

func simpanHandoverDB(nomorWA string) error {
	if isShuttingDown.Load() {
		return fmt.Errorf("shutdown in progress")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := dbTamu.Exec(`
		INSERT INTO handover_sessions (nomor_wa, aktif, mulai_at)
		VALUES (?, 1, ?)
		ON CONFLICT(nomor_wa) DO UPDATE SET aktif=1, mulai_at=excluded.mulai_at, selesai_at=NULL`,
		nomorWA, now)
	return err
}

func resetHandoverDB(nomorWA string) error {
	if isShuttingDown.Load() {
		return fmt.Errorf("shutdown in progress")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := dbTamu.Exec(
		`UPDATE handover_sessions SET aktif=0, selesai_at=? WHERE nomor_wa=?`,
		now, nomorWA)
	return err
}

func loadHandoverDariDB() error {
	rows, err := dbTamu.Query(`SELECT nomor_wa FROM handover_sessions WHERE aktif=1`)
	if err != nil {
		return err
	}
	defer rows.Close()

	ownerMutex.Lock()
	defer ownerMutex.Unlock()
	count := 0
	for rows.Next() {
		var nomor string
		if err := rows.Scan(&nomor); err == nil {
			conversationOwners[nomor] = &ConversationOwner{
				State:          StateHuman,
				LastHumanReply: time.Now(),
				LastTouched:    time.Now(), // <-- MUTASI 2: Tambahkan LastTouched saat dimuat dari database
			}
			count++
		}
	}
	if count > 0 {
		fmt.Printf("[OWNER] %d sesi HUMAN dimuat dari DB.\n", count)
	}
	return rows.Err()
}

// =============================================================================
//  DB HELPERS — Chat History Persistence
// =============================================================================


func simpanRiwayatChatDB(nomorWA, role, teks string) error {
	if isShuttingDown.Load() {
		return fmt.Errorf("shutdown in progress")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := dbTamu.Exec(
		`INSERT INTO chat_history (nomor_wa, role, teks, created_at) VALUES (?, ?, ?, ?)`,
		nomorWA, role, teks, now)
	return err
}

func loadRiwayatChatDariDB(nomorWA string) ([]TurnChat, error) {
	rows, err := dbTamu.Query(`
		SELECT role, teks FROM (
			SELECT role, teks, id FROM chat_history
			WHERE nomor_wa = ?
			ORDER BY id DESC LIMIT 10
		) ORDER BY id ASC`, nomorWA)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hasil []TurnChat
	for rows.Next() {
		var t TurnChat
		if err := rows.Scan(&t.Role, &t.Teks); err == nil {
			hasil = append(hasil, t)
		}
	}
	return hasil, rows.Err()
}

func hapusRiwayatChatDB(nomorWA string) error {
	_, err := dbTamu.Exec(`DELETE FROM chat_history WHERE nomor_wa = ?`, nomorWA)
	return err
}

// =============================================================================
//  HUMAN HANDOVER
// =============================================================================

// =============================================================================
//  OWNERSHIP STATE MACHINE — Helper Functions
// =============================================================================

// getOrCreateOwnerLocked mengambil atau membuat ConversationOwner.
// WAJIB dipanggil dengan ownerMutex sudah di-Lock().
func getOrCreateOwnerLocked(nomorWA string) *ConversationOwner {
	owner, ada := conversationOwners[nomorWA]
	if !ada {
		owner = &ConversationOwner{
			State: StateBot,
		}
		conversationOwners[nomorWA] = owner
	}
	owner.LastTouched = time.Now() // <-- MUTASI 1: Diperbarui saat objek dibuat/diakses di memori
	return owner
}

// getOwnerState mengembalikan state kepemilikan percakapan saat ini.
// Aman dipanggil dari goroutine mana pun (menggunakan RLock).
func getOwnerState(nomorWA string) string {
	ownerMutex.RLock()
	defer ownerMutex.RUnlock()
	owner, ada := conversationOwners[nomorWA]
	if !ada {
		return StateBot
	}
	return owner.State
}

// handleGuestServiceMessage dipanggil ketika pesan IsFromMe (bukan command)
// dikirim ke chat customer — artinya Guest Services sedang aktif membalas.
// State di-set ke HUMAN dan timestamp diperbarui.
	func handleGuestServiceMessage(nomorWA string) {
	ownerMutex.Lock()
	defer ownerMutex.Unlock()
	owner := getOrCreateOwnerLocked(nomorWA)
	prevState := owner.State
	owner.State = StateHuman
	owner.LastHumanReply = time.Now()
	owner.LastTouched = time.Now() // <-- MUTASI 4: Diperbarui saat CS/Guest Services merespons chat
	fmt.Printf("[OWNER] Guest Services membalas → %s -> HUMAN (nomor: %s)\n", prevState, nomorWA)


	// TITIK 1: Update LastActivity saat Guest Services membalas
	sesiChatMutex.Lock()
	sesi, ada := sesiChatMap[nomorWA]
	sesiChatMutex.Unlock()
	if ada {
		sesi.Mu.Lock()
		sesi.LastActivity = time.Now()
		sesi.Mu.Unlock()
	}
}


// handleCustomerTurn dipanggil ketika customer mengirim pesan.
// Menjalankan state machine dan mengembalikan:
//   - true  → bot harus memproses pesan (Gemini aktif)
//   - false → bot diam, tunggu Guest Services
func handleCustomerTurn(nomorWA string) bool {
	ownerMutex.Lock()
	defer ownerMutex.Unlock()

	owner := getOrCreateOwnerLocked(nomorWA)
	owner.LastCustomerMessage = time.Now()
	owner.LastTouched = time.Now()

	switch owner.State {
	case StateBot:
		// Bot aktif — proses dengan Gemini
		return true

	case StateHuman:
		// GS sedang menangani — customer chat lagi → masuk WAITING_HUMAN
		owner.State = StateWaitingHuman
		owner.WaitingSince = time.Now()
		fmt.Printf("[OWNER] HUMAN -> WAITING_HUMAN (nomor: %s)\n", nomorWA)
		return false

	case StateWaitingHuman:
		// Sedang menunggu GS — cek timeout
		elapsed := time.Since(owner.WaitingSince)
		if elapsed >= HumanResponseTimeout {
			fmt.Printf("[OWNER] WAITING_HUMAN -> BOT (timeout %.0fs, nomor: %s)\n",
				elapsed.Seconds(), nomorWA)
			owner.State = StateBot
			// Reset di DB
			activeWorkers.Add(1)
			go func() {
				defer activeWorkers.Done()
				if err := resetHandoverDB(nomorWA); err != nil {
					fmt.Printf("[WARN] Gagal reset owner di DB: %v\n", err)
				}
			}()
			return true // Timeout — Gemini ambil alih
		}
		// Masih dalam timeout — bot tetap diam
		fmt.Printf("[OWNER] WAITING_HUMAN masih aktif (%.0fs / %.0fs, nomor: %s)\n",
			elapsed.Seconds(), HumanResponseTimeout.Seconds(), nomorWA)
		return false
	}

	return true
}

// aktifkanHandoverOwner dipanggil ketika AI mendeteksi [BUTUH_CS].
// Mengirim pesan handover ke customer dan set state ke HUMAN.
func aktifkanHandoverOwner(nomorWA string, chatJID types.JID) {
	ownerMutex.Lock()
	owner := getOrCreateOwnerLocked(nomorWA)
	prevState := owner.State
	owner.State = StateHuman
	owner.LastHumanReply = time.Now()
	owner.LastTouched = time.Now()
	ownerMutex.Unlock()

	fmt.Printf("[OWNER] %s -> HUMAN via [BUTUH_CS] (nomor: %s)\n", prevState, nomorWA)

	if err := simpanHandoverDB(nomorWA); err != nil {
		fmt.Printf("[WARN] Gagal simpan handover ke DB: %v\n", err)
	}

	sendReply(chatJID,
		"Terima kasih atas pesan Anda. Agar kami dapat memberikan informasi dan pelayanan yang lebih tepat, "+
			"saat ini pesan Anda sedang kami hubungkan langsung dengan Layanan Tamu (Guest Services) kami. "+
			"Mohon kesediaannya untuk menunggu sejenak. 🙏\n\n"+
			"Sementara itu, Anda juga dapat menghubungi kami langsung melalui:\n"+
			"📋 Reservasi: https://wa.me/"+KontakReservasi+"\n"+
			"💼 Sales & Event: https://wa.me/"+KontakSales+"\n"+
			"🏨 Guest Services: https://wa.me/"+KontakFO+"\n"+
			"🍽️ Restoran: https://wa.me/"+KontakResto)
}

// forceOwnerBot memaksa state ke BOT (untuk command !selesai).
// Juga membersihkan sesi chat dan riwayat.
func forceOwnerBot(nomorWA string, chatJID types.JID) {
	ownerMutex.Lock()
	owner := getOrCreateOwnerLocked(nomorWA)
	prevState := owner.State
	owner.State = StateBot
	ownerMutex.Unlock()
	owner.LastTouched = time.Now()

	fmt.Printf("[MANUAL] %s -> BOT (nomor: %s)\n", prevState, nomorWA)

	if err := resetHandoverDB(nomorWA); err != nil {
		fmt.Printf("[WARN] Gagal reset owner di DB: %v\n", err)
	}

	// Bersihkan sesi memory agar tamu mulai percakapan baru
	sesiChatMutex.Lock()
	delete(sesiChatMap, nomorWA)
	sesiChatMutex.Unlock()

	if err := hapusRiwayatChatDB(nomorWA); err != nil {
		fmt.Printf("[WARN] Gagal hapus riwayat chat dari DB: %v\n", err)
	}

	sendReply(chatJID,
		"Terima kasih telah menghubungi Kadena Glamping Dive Resort. "+
			"Apabila masih ada yang ingin ditanyakan, kami siap membantu kapan saja. 😊✨")
}

// forceOwnerHuman memaksa state ke HUMAN (untuk command !handover).
func forceOwnerHuman(nomorWA string) {
	ownerMutex.Lock()
	owner := getOrCreateOwnerLocked(nomorWA)
	prevState := owner.State
	owner.State = StateHuman
	owner.LastHumanReply = time.Now()
	owner.LastTouched = time.Now()
	ownerMutex.Unlock()

	fmt.Printf("[MANUAL] %s -> HUMAN (nomor: %s)\n", prevState, nomorWA)

	if err := simpanHandoverDB(nomorWA); err != nil {
		fmt.Printf("[WARN] Gagal simpan handover ke DB: %v\n", err)
	}
}

// applyConversationOwnerTTL memeriksa semua entry di conversationOwners dan
// menerapkan logika TTL berdasarkan State dan LastTouched:
//
//   - BOT          : hapus entry jika idle > TTLOwnerBot (tanpa update DB)
//   - WAITING_HUMAN: reset State ke BOT + panggil resetHandoverDB jika idle > TTLOwnerWaitingHuman
//   - HUMAN        : reset State ke BOT + panggil resetHandoverDB jika idle > TTLOwnerHuman
//     (disertai log AUTO-EXPIRE)
//
// Fungsi ini akan dipanggil oleh cleanup worker pada FASE 4B.4.
func applyConversationOwnerTTL() {
	now := time.Now()

	// Kumpulkan nomor WA yang perlu di-reset di DB, agar tidak memanggil
	// I/O database di dalam critical section ownerMutex.
	var toResetDB []string

	ownerMutex.Lock()
	for nomorWA, owner := range conversationOwners {
		age := now.Sub(owner.LastTouched)

		switch owner.State {
		case StateBot:
			if age > TTLOwnerBot {
				delete(conversationOwners, nomorWA)
				fmt.Printf("[TTL] BOT entry dihapus (idle %.0fh, nomor: %s)\n",
					age.Hours(), nomorWA)
			}

		case StateWaitingHuman:
			if age > TTLOwnerWaitingHuman {
				owner.State = StateBot
				toResetDB = append(toResetDB, nomorWA)
				fmt.Printf("[TTL] WAITING_HUMAN -> BOT (expired %.0fm, nomor: %s)\n",
					age.Minutes(), nomorWA)
			}

		case StateHuman:
			if age > TTLOwnerHuman {
				owner.State = StateBot
				toResetDB = append(toResetDB, nomorWA)
				fmt.Printf("[TTL] AUTO-EXPIRE: HUMAN -> BOT (idle %.1fh, nomor: %s)\n",
					age.Hours(), nomorWA)
			}
		}
	}
	ownerMutex.Unlock()

	// Reset handover di DB setelah mutex dilepas — hindari blocking I/O di bawah lock.
	for _, nomor := range toResetDB {
		if err := resetHandoverDB(nomor); err != nil {
			fmt.Printf("[WARN] TTL: Gagal reset handover DB untuk %s: %v\n", nomor, err)
		}
	}
}

// deteksiKamarDariTeks mengekstrak tipe kamar yang disebut dalam teks percakapan.
func deteksiKamarDariTeks(teks string) string {
	teksLower := strings.ToLower(teks)

	// Urutan dari paling spesifik ke paling umum agar tidak salah match
	kamarList := []struct{ nama, kata string }{
		{"Glamping Superior", "glamping superior"},
        {"Glamping Deluxe", "glamping deluxe"},
        {"Glamping Suite", "glamping suite"},
	}

	for _, k := range kamarList {
		if strings.Contains(teksLower, k.kata) {
			return k.nama
		}
	}
	return ""
}

// tentukanModel memilih tier AI berdasarkan kompleksitas pertanyaan.
//
// Target distribusi: ~50% Lite / ~45% Flash / ~5% Pro
//
// TIER 3 — Pro  (~5%):  perbandingan aktif, rekomendasi multi-opsi, brief/proposal
// TIER 2 — Flash (~45%): pertanyaan spesifik hotel: harga, paket, kamar, event
// TIER 1 — Lite  (~50%): sapaan, pertanyaan umum, info dasar, singkat & faktual
func tentukanModel(pesan string) string {
	pesanLower := strings.ToLower(strings.TrimSpace(pesan))
	panjang := len([]rune(pesan))

	// ── TIER 3 — Pro ─────────────────────────────────────────────────────────────────────────
	// Hanya untuk pertanyaan yang benar-benar butuh reasoning multi-langkah:
	// perbandingan eksplisit, rekomendasi aktif antar opsi, atau brief event panjang.
	kataPro := []string{
		// Perbandingan eksplisit
		"bandingkan", "compare", " vs ", "versus",
		"perbedaan antara", "difference between", "apa bedanya",
		// Rekomendasi multi-opsi (frasa, bukan kata tunggal)
		"rekomendasikan ruang", "ruang mana yang", "ruangan mana yang",
		"which room is", "which venue", "most suitable for",
		"paling cocok untuk acara", "paling sesuai untuk",
		// Dokumen / perencanaan event lengkap
		"itinerary", "rundown", "susunan acara", "proposal acara",
		"buat jadwal", "rancangan acara",
	}
	for _, kata := range kataPro {
		if strings.Contains(pesanLower, kata) {
			return LiteLLMModelPro
		}
	}
	// Brief/RFP panjang — kemungkinan permintaan penawaran event atau keluhan panjang
	if panjang > 200 {
		return LiteLLMModelPro
	}

	// ── TIER 2 — Flash ───────────────────────────────────────────────────────────────────
	// Pertanyaan spesifik hotel yang membutuhkan data KB atau reasoning ringan.
	kataFlash := []string{
		// Harga & transaksi
		"harga", "tarif", "biaya", "promo", "diskon", "paket",
		"booking", "reservasi", "pesan kamar", "available", "tersedia",
		// Kamar
		"kamar", "suite", "room", "tipe kamar", "jenis kamar",
		// Event & venue
		"meeting", "ballroom", "convention", "seminar", "konferensi",
		"wedding", "pernikahan", "event", "gathering",
		"ulang tahun", "birthday", "wisuda", "graduation",
		// Fasilitas spesifik
		"fasilitas", "kolam renang", "gym", "fitness", "spa",
		"restoran", "restaurant", "menu", "sarapan", "breakfast",
		// Kapasitas
		"kapasitas", "berapa orang", "berapa pax", "muat berapa",
	}
	for _, kata := range kataFlash {
		if strings.Contains(pesanLower, kata) {
			return LiteLLMModelKompleks
		}
	}
	// Pesan menengah (>80 karakter) tanpa keyword Flash → masih mungkin perlu detail
	if panjang > 80 {
		return LiteLLMModelKompleks
	}

	// ── TIER 1 — Flash-Lite ──────────────────────────────────────────────────────────────
	// Sapaan, pertanyaan umum singkat, info dasar (wifi, parkir, jam check-in),
	// dan semua pesan yang tidak memicu keyword di atas.
	return LiteLLMModelRingan
}


// doAICallWithRetry menangani request ke endpoint AI dengan Exponential Backoff.
// Hanya melakukan retry pada 429, 503, Timeout, dan Transient Network Error.
func doAICallWithRetry(ctx context.Context, urlAPI string, requestBody []byte, modelName string) ([]byte, error) {
	var lastErr error
	delay := AIRetryInitialDelay

	for attempt := 1; attempt <= AIRetryMaxAttempts; attempt++ {
		// Timeout request dibatasi 15 detik per attempt agar tidak hang permanen
		reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)

		req, err := http.NewRequestWithContext(reqCtx, "POST", urlAPI, bytes.NewBuffer(requestBody))
		if err != nil {
			cancel()
			return nil, fmt.Errorf("gagal buat request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+geminiAPIKey())

		resp, err := httpClientGemini.Do(req)

		isRetryable := false
		reason := ""

		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				isRetryable = true
				reason = "net/http timeout"
			} else if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
				isRetryable = true
				reason = "context deadline exceeded"
			} else if strings.Contains(strings.ToLower(err.Error()), "connection reset") ||
				strings.Contains(strings.ToLower(err.Error()), "eof") ||
				strings.Contains(strings.ToLower(err.Error()), "no such host") {
				isRetryable = true
				reason = "transient network error"
			} else if errors.As(err, &netErr) && netErr.Temporary() {
				isRetryable = true
				reason = "net.Error temporary"
			}
		} else {
			if resp.StatusCode == 429 || resp.StatusCode == 503 || resp.StatusCode == 502 || resp.StatusCode == 504 {
				isRetryable = true
				reason = fmt.Sprintf("HTTP %d", resp.StatusCode)
			} else if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				isRetryable = false // Permanent client error (400, 401, 403, 404)
			}
		}

		// Jika request sukses ATAU ada error permanent yang tidak bisa di-retry
		if !isRetryable {
			if err == nil {
				defer resp.Body.Close()
				body, readErr := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // limit 5MB
				cancel()
				if readErr != nil {
					return nil, fmt.Errorf("gagal baca body: %w", readErr)
				}
				if resp.StatusCode >= 300 {
					return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
				}

				if attempt > 1 {
					fmt.Printf("[AI RETRY SUCCESS]\nattempt=%d\nmodel=%s\n", attempt, modelName)
				}
				return body, nil
			}
			cancel()
			return nil, err
		}

		// Jika error retryable tapi ini attempt terakhir
		if attempt == AIRetryMaxAttempts {
			cancel()
			if err != nil {
				lastErr = err
			} else {
				lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
				resp.Body.Close()
			}
			break
		}

		// Print LOG AI RETRY sesuai instruksi
		fmt.Printf("[AI RETRY]\nattempt=%d\nmodel=%s\nreason=%s\n", attempt, modelName, reason)

		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		cancel()

		// Jeda sebelum retry berikutnya (Exponential Backoff)
		select {
		case <-ctx.Done(): // Jika bot sedang shutdown
			return nil, ctx.Err()
		case <-time.After(delay):
		}

		// Set delay eksponensial (2s -> 4s -> 8s)
		delay *= 2
		if delay > AIRetryMaxDelay {
			delay = AIRetryMaxDelay
		}
	}

	return nil, fmt.Errorf("max retry reached (%d attempts), last error: %w", AIRetryMaxAttempts, lastErr)
}


// =============================================================================
//  VALIDASI KUALITAS RESPONS AI — FIX BUG #2
//  Validasi sebelumnya (len(teks) < 4) terlalu lemah: satu emoji UTF-8 saja
//  sudah 4 byte sehingga lolos. Validasi di bawah ringan (tanpa NLP/model
//  tambahan) tapi menolak: respons kosong, hanya emoji/simbol, terlalu
//  sedikit karakter bermakna, atau balasan generik yang tidak menjawab apa-apa.
// =============================================================================

// minKarakterBermaknaRespons = jumlah minimal huruf/angka (bukan emoji, spasi,
// atau tanda baca) yang harus ada agar respons dianggap valid untuk dikirim.
const minKarakterBermaknaRespons = 8

// balasanGenerikSaja = balasan basa-basi yang tidak benar-benar menjawab apa
// yang ditanyakan tamu (mis. "Baik kak."). Dicek setelah teks dibersihkan
// (lowercase, tanpa tanda baca) dan harus cocok PERSIS (bukan substring) agar
// tidak menolak jawaban normal yang kebetulan memuat kata "baik"/"siap".
var balasanGenerikSaja = map[string]bool{
	"baik": true, "baik kak": true, "baik pak": true, "baik bu": true,
	"oke": true, "ok": true, "siap": true, "siap kak": true,
	"noted": true, "terima kasih": true, "sama sama": true,
	"baik terima kasih": true, "iya": true, "iya kak": true, "baik ya": true,
}

// hitungKarakterBermakna menghitung rune berupa huruf/angka saja — emoji,
// simbol, spasi, dan tanda baca tidak dihitung.
func hitungKarakterBermakna(s string) int {
	total := 0
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			total++
		}
	}
	return total
}

// bersihkanUntukPerbandingan → lowercase, hanya huruf/angka/spasi, dipakai
// untuk mencocokkan ke daftar balasanGenerikSaja.
func bersihkanUntukPerbandingan(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == ' ' {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// validasiResponsAI mengembalikan (false, alasan) jika respons AI tidak layak
// dikirim ke tamu: kosong, hanya emoji/simbol, terlalu pendek, atau balasan
// generik yang tidak menjawab pertanyaan.
func validasiResponsAI(teks string) (bool, string) {
	trimmed := strings.TrimSpace(teks)
	if trimmed == "" {
		return false, "respons kosong"
	}

	// Marker handover bawaan — biarkan lolos, ditangani oleh logika [BUTUH_CS]
	// yang sudah ada di bawah (Human Handover tidak diubah).
	if trimmed == "[BUTUH_CS]" {
		return true, ""
	}

	if balasanGenerikSaja[bersihkanUntukPerbandingan(trimmed)] {
		return false, fmt.Sprintf("balasan generik, tidak menjawab pertanyaan: '%s'", trimmed)
	}

	jumlahBermakna := hitungKarakterBermakna(trimmed)
	if jumlahBermakna == 0 {
		return false, "respons hanya berisi emoji/simbol"
	}
	if jumlahBermakna < minKarakterBermaknaRespons {
		return false, fmt.Sprintf("respons terlalu pendek (%d karakter bermakna): '%s'", jumlahBermakna, trimmed)
	}

	return true, ""
}

// =============================================================================
//  KOBOILLM AI dengan CONVERSATION MEMORY
// =============================================================================
func tanyaGeminiAI(sesi *SesiChat, pesanUser, nomorWA, modelName, extraContext string) (string, bool) {
	// Data cadangan — selalu pakai InfoHotelFallback (website fetch dihapus)
	dataCadangan := InfoHotelFallback
	labelCadangan := "data manual fallback"

	// Ambil profil tamu untuk personalization (tanpa sesiChatMutex — akses DB, bukan sesiChatMap)
	profil, errProfil := getOrCreateProfilTamu(nomorWA)
	if errProfil != nil || profil == nil {
		profil = &ProfilTamu{NomorWA: nomorWA}
	}

	namaTamu := sapaanFormal(profil.Nama) // "Bapak/Ibu" atau "Bapak/Ibu <nama>"

	personalizationHint := ""
	if profil.KunjunganKe > 1 {
		personalizationHint = fmt.Sprintf(
			"Tamu ini adalah tamu yang sudah menginap %d kali sebelumnya. "+
				"dengan hangat dan profesional. Nama tamu: %s.", profil.KunjunganKe, namaTamu)
		if profil.KamarFavorit != "" {
			personalizationHint += fmt.Sprintf(" Kamar favorit sebelumnya: %s.", profil.KamarFavorit)
		}
	}
	if profil.StatusVIP {
		personalizationHint += " Tamu ini adalah tamu VIP Kadena Glamping Dive Resort — berikan sapaan dan layanan yang lebih istimewa."
	}

	// Cari di knowledge base lokal dulu (prioritas tertinggi, hemat token)
	dataKB := cariDiKnowledgeBase(pesanUser)
	sumberKB := ""
	if dataKB != "" {
		sumberKB = "✅ Knowledge Base Lokal"
		fmt.Printf("[KB] Data ditemukan untuk: %.50s\n", pesanUser)
	} else {
		fmt.Printf("[KB] Tidak ada di KB lokal, fallback ke website cache\n")
	}

	// Build system prompt
wibZone := time.FixedZone("WIB", 7*60*60)
jamWIB := time.Now().In(wibZone)
var sapaanWaktu string
switch h := jamWIB.Hour(); {
case h >= 5 && h < 11:
    sapaanWaktu = "Selamat Pagi"
case h >= 11 && h < 15:
    sapaanWaktu = "Selamat Siang"
case h >= 15 && h < 18:
    sapaanWaktu = "Selamat Sore"
default:
    sapaanWaktu = "Selamat Malam"
}
infoWaktu := fmt.Sprintf("🕐 WAKTU SEKARANG: %s WIB. Sapaan waktu yang WAJIB digunakan saat ini adalah \"%s\". JANGAN gunakan sapaan waktu lain.",
    jamWIB.Format("15:04"), sapaanWaktu)
	systemPrompt := fmt.Sprintf(`Kamu adalah resepsionis virtual  Kadena Glamping Dive Resort.
	%s
Tugasmu membalas pesan pelanggan dengan ramah, sopan, dan profesional sesuai standar hotel bintang 4-5. 
Panggil tamu dengan "%s". Gunakan bahasa yang formal namun tetap hangat.

🌐 BAHASA: Deteksi bahasa pelanggan secara otomatis.
- Jika pelanggan menulis dalam Bahasa Indonesia → balas dalam Bahasa Indonesia.
- Jika pelanggan menulis dalam English → reply in English.
- Jika pelanggan menulis dalam bahasa daerah indonesia → reply menggunakan bahasa daerah yang 
sedang digunakan oleh pelanggan.
- Gunakan bahasa yang sama dengan pelanggan di setiap balasan.

%s

%s

SUMBER DATA (gunakan urutan prioritas ini):

PRIORITAS 1 — KNOWLEDGE BASE LOKAL %s

STATUS:
✅ DATA RESMI HOTEL
✅ SUMBER FAKTA UTAMA
✅ PRIORITAS TERTINGGI

PENTING:
Informasi di bawah berasal dari database resmi hotel.

Jika informasi di bawah sudah cukup menjawab pertanyaan pelanggan, MAKA:

- WAJIB gunakan informasi ini sebagai jawaban utama.
- WAJIB mempertahankan seluruh fakta penting (lihat pengecualian strategi ringkas di ATURAN MENJAWAB poin 1 & 3 untuk pertanyaan panjang).
- JANGAN mengubah fakta.
- JANGAN menambahkan fakta yang tidak ada.
- JANGAN mencari sumber lain.
- Tugasmu mengubah bahasa agar lebih natural, ramah, sopan, dan profesional, sambil tetap patuh strategi ringkas di ATURAN MENJAWAB poin 3 bila berlaku.

===== DATA RESMI HOTEL =====

%s

===== AKHIR DATA RESMI =====

PRIORITAS 2 — URL HALAMAN HOTEL (jika KB lokal tidak cukup):
- Halaman utama   → https://kadenaglampingdiveresort.com/home/
- Semua kamar     → https://kadenaglampingdiveresort.com/room/
- Fasilitas hotel → https://kadenaglampingdiveresort.com/home/
- Paket           → https://kadenaglampingdiveresort.com/home/
- Meeting & Event → https://kadenaglampingdiveresort.com/service/conference-room/

Promotion Deals dibagi menjadi 4 paket
- Opening Rates → https://kadenaglampingdiveresort.com/curabitur-a-lectus/
- Early Bird Promotion → https://kadenaglampingdiveresort.com/sed-ornare-porta/
- F&B Promotion → https://kadenaglampingdiveresort.com/donec-faucibus/
- CHEF RECOMENDED → https://kadenaglampingdiveresort.com/suspendisse-potenti/

Fasilitas hotel atau Glamping ada beberapa pilihan seperti:
- Dive Center & Water Sport → https://kadenaglampingdiveresort.com/service/spa-beauty-health/
- Meeting & Event → https://kadenaglampingdiveresort.com/service/conference-room/
- Dining → https://kadenaglampingdiveresort.com/service/restaurant/
- Swimming Pool → https://kadenaglampingdiveresort.com/service/swimming-pool/


PANDUAN GOOGLE SEARCH (gunakan jika KB lokal & URL di atas tidak cukup):
Gunakan query: "Kadena Glamping Dive Resort [topik yang ditanyakan]"

PRIORITAS 3 — DATA CADANGAN (%s):
%s

BOOKING KAMAR — LINK RESMI:
Jika pelanggan bertanya tentang booking/reservasi kamar atau cara memesan kamar, selalu sertakan link ini:
🔗 %s

KONTAK DEPARTEMEN (gunakan saat perlu mengarahkan pelanggan ke tim yang tepat):
- Reservasi        → WhatsApp: https://wa.me/%s
- Sales & Event    → WhatsApp: https://wa.me/%s
- Guest Services   → WhatsApp: https://wa.me/%s
- Restoran         → WhatsApp: https://wa.me/%s

ATURAN MENJAWAB:
1. Jika Knowledge Base Lokal tersedia dan sudah dapat menjawab pertanyaan pelanggan:
   - Gunakan Knowledge Base sebagai SUMBER FAKTA UTAMA, JANGAN mengubah atau menambah fakta baru, JANGAN mencari sumber lain.
   - Untuk pertanyaan yang jawabannya PANJANG (perbandingan, rincian harga, menu, syarat, jam operasional, dll), WAJIB ikuti STRATEGI RINGKAS di poin 3 — JANGAN langsung memuat seluruh detail KB di balasan pertama meskipun datanya lengkap tersedia.
   - Untuk pertanyaan yang jawabannya PENDEK/sederhana (1 fakta, 1 angka, ya/tidak), boleh langsung jawab lengkap tanpa perlu strategi ringkas.
   - Kelengkapan fakta KB (semua detail dipertahankan) hanya WAJIB dipenuhi penuh pada balasan LANJUTAN, setelah pelanggan minta detail lebih (lihat poin 3).
2. Jika KB lokal kosong, baru cek URL hotel atau Google Search.
3. STRATEGI MENJAWAB INFORMASI PANJANG (WAJIB, PRIORITAS DI ATAS KELENGKAPAN DATA KB):
   Untuk pertanyaan yang jawabannya sangat panjang dari Knowledge Base (seperti perbandingan fasilitas, rincian harga, menu, syarat, jam operasional, dll), balasan PERTAMA WAJIB dibatasi sekitar 100-150 kata saja, dengan struktur:
   - Sebutkan HANYA 2-3 poin pembeda/poin utama (bukan seluruh detail yang ada di KB).
   - JANGAN coba memuat semua data KB sekaligus di balasan pertama, meskipun datanya lengkap tersedia di Knowledge Base.
   - WAJIB akhiri pesan dengan pertanyaan proaktif. Contoh: "Apakah Bapak/Ibu ingin saya jabarkan rincian fasilitas lengkapnya?" atau "Apakah informasi ini sudah cukup, atau ada detail lain yang ingin Bapak/Ibu ketahui?"
   - TUNGGU RESPON: Jika pelanggan membalas "Ya", "Lanjut", atau eksplisit minta detail, BARULAH pada balasan LANJUTAN ini berikan sisa informasi secara rinci dan lengkap sesuai data di Knowledge Base (di sinilah aturan "pertahankan seluruh fakta" di poin 1 berlaku penuh).
4. Gunakan emoji secukupnya agar terasa ramah.
5. ANALISIS topik pertanyaan pelanggan dan PROAKTIF berikan info relevan:
   - Tentang booking/reservasi unit glamping → sertakan link booking berikut → https://www.tiket.com/id-id/hotel/indonesia/kadena-glamping-dive-resort-410001635521096877.
   - Tentang Promotion Deals → coba jawab, jika tidak bisa berikan rekomendasi ke empat Promotion Deals beserta linknya.
   - Tentang fasilitas hotel atau galmping → jawab sesui fasilitas yang ada dan bila cutomer meminta detail coba jawab jika tidak bisa berikan link fasilitas sesuai fasilitas yang ditannyakan customer.
6. Jika kamu bisa menjawab sebagian tapi butuh eskalasi, JAWAB DULU lalu sertakan kontak yang relevan.
7. JANGAN spekulatif atau memberikan info yang tidak ada dalam sumber data.
8. Ingat percakapan sebelumnya dan lanjutkan secara natural.
9. Jika tamu menyebutkan nama mereka, catat dan gunakan di balasan berikutnya.
10. [BUTUH_CS] HANYA untuk situasi darurat atau kompleks yang tidak bisa ditangani sama sekali.`,
		infoWaktu,
		namaTamu, RoomURLMap, personalizationHint,
		sumberKB, dataKB,
		labelCadangan, dataCadangan,
		generateBookingURL(),
		KontakReservasi, KontakSales, KontakFO, KontakResto)

	// Inject extra context dari template hint jika ada (misal: salam keagamaan)
	if extraContext != "" {
		systemPrompt += "\n\nINSTRUKSI KHUSUS UNTUK PESAN INI:\n" + extraContext
	}

	// =========================================================================
	// Build messages — format OpenAI-compatible (KoboLLM)
	// System prompt masuk sebagai role "system", history tetap sama
	// =========================================================================

	// Simpan pesan ini ke riwayat
	sesi.Mu.Lock()
	// System message pertama
	messages := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
	}
	// Persist pesan user ke DB (session memory tahan restart)
	if err := simpanRiwayatChatDB(nomorWA, "user", pesanUser); err != nil {
		fmt.Printf("[WARN] Gagal simpan riwayat user ke DB: %v\n", err)
	}

	// Append riwayat percakapan — "model" → "assistant" sesuai format OpenAI
	for _, turn := range sesi.Riwayat {
		role := "user"
		if turn.Role == "model" {
			role = "assistant"
		}
		messages = append(messages, map[string]interface{}{
			"role":    role,
			"content": turn.Teks,
		})
	}

	// FIX BUG #1 (CRITICAL): pesan user TERBARU wajib ikut terkirim ke AI.
	// Sebelumnya pesanUser hanya dipersist ke DB (baris di atas) tapi tidak
	// pernah dimasukkan ke payload `messages` — akibatnya AI menerima system
	// prompt + KB tapi tidak pernah tahu pertanyaan user. Urutan akhir payload
	// sekarang: system → history (assistant/user lama, jika ada) → user (baru).
	messages = append(messages, map[string]interface{}{
		"role":    "user",
		"content": pesanUser,
	})
	sesi.Mu.Unlock()

	// Parameter per tier — temperature rendah = lebih faktual & patuh KB
	temperature := 0.4
	maxTokens   := 1000
	topP        := 0.8
	if modelName == LiteLLMModelPro {
		temperature = 0.3 // Pro: paling ketat, reasoning mendalam
		maxTokens   = 1800
		topP        = 0.75
	} else if modelName == LiteLLMModelRingan {
		temperature = 0.6 // Lite: sedikit lebih natural untuk sapaan
		maxTokens   = 400
		topP        = 0.9
	}

	requestBody, err := json.Marshal(map[string]interface{}{
		"model":       modelName,
		"messages":    messages,
		"temperature": temperature,
		"max_tokens":  maxTokens,
		"top_p":       topP,
	})
	if err != nil {
		fmt.Printf("[ERROR AI] Gagal susun payload: %v\n", err)
		return "", true
	}

	// Endpoint LiteLLM proxy KoboLLM
	urlAPI := LiteLLMBaseURL + "/chat/completions"

	// =========================================================
	// FASE 4A: Panggil HTTP Request menggunakan Helper Retry
	// =========================================================
	bodyBytes, err := doAICallWithRetry(appCtx, urlAPI, requestBody, modelName)
	if err != nil {
		fmt.Printf("[ERROR AI] Gagal AI call setelah retry: %v\n", err)
		return "", true // Error trigger Human Handover (Guest Services)
	}

	// Parse response — format OpenAI-compatible
	var data KoboLLMResponse
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		fmt.Printf("[ERROR AI] Gagal parse JSON KoboLLM: %v\n", err)
		return "", true
	}

	if len(data.Choices) == 0 || data.Choices[0].Message.Content == "" {
		fmt.Println("[ERROR AI] KoboLLM mengembalikan respons kosong (empty choices)")
		return "", true
	}

	teks := strings.TrimSpace(data.Choices[0].Message.Content)

	if valid, alasan := validasiResponsAI(teks); !valid {
		fmt.Printf("[ERROR AI] Respons KoboLLM ditolak (%s)\n", alasan)
		return "", true
	}

	// HANYA [BUTUH_CS] yang trigger handover ke Layanan Tamu
	if strings.Contains(teks, "[BUTUH_CS]") || teks == "[BUTUH_CS]" {
		fmt.Printf("[AI] KoboLLM tidak bisa menjawab pertanyaan: '%s'\n", pesanUser)
		return "", true
	}

	// Deteksi sinyal departemen dari AI dan ganti dengan pesan kontak
	pesanLower := strings.ToLower(pesanUser)
	namaPanggilDept := profil.Nama // raw name — pesanHubungiDepartemen sudah tambah "Bapak/Ibu"

	penandaAmbigu := []string{
		"saya tidak yakin", "kurang yakin", "saya tidak tahu", "tidak pasti",
		"tidak memiliki informasi", "tidak ada informasi", "mungkin anda bisa",
		"sebaiknya hubungi", "coba hubungi",
	}
	teksLower := strings.ToLower(teks)

	isAmbigu := false
	for _, penanda := range penandaAmbigu {
		if strings.Contains(teksLower, penanda) {
			fmt.Printf("[AI] Jawaban KoboLLM terdeteksi ambigu (kata: '%s') → routing ke departemen\n", penanda)
			isAmbigu = true
			break
		}
	}

	if isAmbigu {
		// Routing cerdas berdasarkan topik pertanyaan user
		topikResto    := []string{"makan", "restoran", "restaurant", "menu", "sarapan", "breakfast", "lunch", "dinner", "makan malam", "makan siang", "food", "makanan", "minuman", "beverage"}
		topikSales    := []string{"event", "meeting", "ballroom", "wedding", "pernikahan", "seminar", "gathering", "konvensi", "convention", "social event", "corporate", "kerjasama"}
		topikFO       := []string{"check-in", "checkout", "check out", "luggage", "bagasi", "kunci", "key", "parkir", "parking", "lobi", "lobby", "fasilitas"}
		topikReservasi := []string{"reservasi", "booking", "pesan kamar", "book", "reservation", "kamar", "room", "suite", "menginap", "stay", "harga kamar", "tarif", "promotion deals"}

		dept := "umum"
		for _, k := range topikResto {
			if strings.Contains(pesanLower, k) { dept = "resto"; break }
		}
		if dept == "umum" {
			for _, k := range topikSales {
				if strings.Contains(pesanLower, k) { dept = "sales"; break }
			}
		}
		if dept == "umum" {
			for _, k := range topikFO {
				if strings.Contains(pesanLower, k) { dept = "fo"; break }
			}
		}
		if dept == "umum" {
			for _, k := range topikReservasi {
				if strings.Contains(pesanLower, k) { dept = "reservasi"; break }
			}
		}

		pesanDept := pesanHubungiDepartemen(dept, namaPanggilDept)

		
	// Simpan pesan ini ke riwayat (user lalu model, urut sesuai percakapan asli)
		sesi.Mu.Lock()
		sesi.Riwayat = append(sesi.Riwayat, TurnChat{Role: "user", Teks: pesanUser})
		sesi.Riwayat = append(sesi.Riwayat, TurnChat{Role: "model", Teks: pesanDept})
		if len(sesi.Riwayat) > 10 {
			sesi.Riwayat = sesi.Riwayat[len(sesi.Riwayat)-10:]
		}
		sesi.LastActivity = time.Now() // TITIK 3: Update saat AI mengarahkan tamu ke departemen
		sesi.Mu.Unlock()
		if err := simpanRiwayatChatDB(nomorWA, "model", pesanDept); err != nil {
			fmt.Printf("[WARN] Gagal simpan riwayat dept ke DB: %v\n", err)
		}

		fmt.Printf("[ROUTING] Pertanyaan '%s' diarahkan ke departemen: %s\n", pesanUser, dept)
		return pesanDept, false
	}

	// Simpan giliran percakapan ke riwayat (user lalu model, urut sesuai percakapan asli)
	sesi.Mu.Lock()
	sesi.Riwayat = append(sesi.Riwayat, TurnChat{Role: "user", Teks: pesanUser})
	sesi.Riwayat = append(sesi.Riwayat, TurnChat{Role: "model", Teks: teks})
	if len(sesi.Riwayat) > 10 {
		sesi.Riwayat = sesi.Riwayat[len(sesi.Riwayat)-10:]
	}
	sesi.LastActivity = time.Now() // TITIK 4: Update saat AI merespons dengan jawaban normal
	sesi.Mu.Unlock()

	// Persist jawaban model ke DB
	if err := simpanRiwayatChatDB(nomorWA, "model", teks); err != nil {
		fmt.Printf("[WARN] Gagal simpan riwayat model ke DB: %v\n", err)
	}

	fmt.Printf("[AI] KoboLLM menjawab (%s)\n", labelCadangan)
	return teks, false
}

// =============================================================================
//  FUZZY MATCHING — dengan pre-sorted template
// =============================================================================

func mengandungKataUtuh(input, template string) bool {
	inputKata := strings.Fields(input)
	templateKata := strings.Fields(template)

	if len(templateKata) == 0 || len(inputKata) == 0 {
		return false
	}

	if len(templateKata) == 1 {
		for _, kata := range inputKata {
			if strings.EqualFold(kata, templateKata[0]) {
				return true
			}
		}
		return false
	}

	for i := 0; i <= len(inputKata)-len(templateKata); i++ {
		cocok := true
		for j, tw := range templateKata {
			if !strings.EqualFold(inputKata[i+j], tw) {
				cocok = false
				break
			}
		}
		if cocok {
			return true
		}
	}
	return false
}

func cariTemplateMirip(inputUser string) string {
	// LANGKAH 1: Exact match
	for _, td := range templateTersortir {
		if inputUser == td.teks {
			return td.teks
		}
	}

	// LANGKAH 2: Word-boundary match menggunakan pre-sorted list
	for _, td := range templateTersortir {
		if mengandungKataUtuh(inputUser, td.teks) {
			if td.nKata >= 2 {
				return td.teks
			}
			if len(strings.Fields(inputUser)) <= 2 {
				return td.teks
			}
		}
	}

	// LANGKAH 3: Fuzzy match (Levenshtein) — hanya untuk input pendek
	if len(inputUser) > 10 {
		return ""
	}

	batasToleransi := 4
	if len(inputUser) <= 4 {
		batasToleransi = 1
	} else if len(inputUser) <= 8 {
		batasToleransi = 2
	}

	targetTerdekat := ""
	jarakTerkecil := 999

	for _, td := range templateTersortir {
		if td.nKata == 1 && len(td.teks) <= 2 {
			continue
		}
		jarak := levenshtein.ComputeDistance(inputUser, td.teks)
		if jarak < jarakTerkecil {
			jarakTerkecil = jarak
			targetTerdekat = td.teks
		}
	}

	if jarakTerkecil <= batasToleransi {
		return targetTerdekat
	}
	return ""
}

// =============================================================================
//  JAWABAN TEMPLATE — dengan Guest Personalization
// =============================================================================

func jawabanTemplate(perintah string, msg *events.Message, profil *ProfilTamu) string {
    // Nama mentah (dari profil DB atau WhatsApp push name)
    rawNama := profil.Nama
    if rawNama == "" {
        rawNama = msg.Info.PushName
    }
    // Sapaan formal Indonesia: "Bapak/Ibu" atau "Bapak/Ibu <nama>"
    namaPanggil := sapaanFormal(rawNama)

    // Timezone WIB (UTC+7) — pakai FixedZone agar tidak perlu tzdata
    wib := time.FixedZone("WIB", 7*60*60)

    switch perintah {
    // --- Salam Keagamaan — hint untuk AI, bukan static ---
    case "assalamualaikum", "assalamu alaikum", "assalamu'alaikum":
        return "__HINT__:Tamu mengucapkan salam Islam 'Assalamu'alaikum'. " +
            "WAJIB awali balasan dengan 'Wa'alaikumsalam Warahmatullahi Wabarakatuh 🌙', " +
            "lalu sambut tamu dengan hangat sebagai resepsionis Kadena Glamping Dive Resort dan tanya ada yang bisa dibantu."

    case "waalaikumsalam", "wa alaikumsalam", "wa'alaikumsalam":
        return "__HINT__:Tamu membalas salam Islam. " +
            "Sambut dengan hangat dan lanjutkan percakapan sebagai resepsionis Kadena Glamping Dive Resort."

    case "om swastiastu":
        return "__HINT__:Tamu mengucapkan salam Hindu 'Om Swastiastu'. " +
            "WAJIB awali balasan dengan 'Om Swastiastu 🙏', " +
            "lalu sambut tamu dengan hangat sebagai resepsionis Kadena Glamping Dive Resort dan tanya ada yang bisa dibantu."

    case "shalom":
        return "__HINT__:Tamu mengucapkan salam Kristiani/Yahudi 'Shalom'. " +
            "WAJIB awali balasan dengan 'Shalom 🙏', " +
            "lalu sambut tamu dengan hangat sebagai resepsionis Kadena Glamping Dive Resort dan tanya ada yang bisa dibantu."

    case "namo buddhaya":
        return "__HINT__:Tamu mengucapkan salam Buddhis 'Namo Buddhaya'. " +
            "WAJIB awali balasan dengan 'Namo Buddhaya 🙏', " +
            "lalu sambut tamu dengan hangat sebagai resepsionis Kadena Glamping Dive Resort dan tanya ada yang bisa dibantu."

    case "rahayu":
        return "__HINT__:Tamu mengucapkan salam 'Rahayu'. " +
            "WAJIB awali balasan dengan 'Rahayu 🙏', " +
            "lalu sambut tamu dengan hangat sebagai resepsionis Kadena Glamping Dive Resort dan tanya ada yang bisa dibantu."

    case "salam sejahtera", "selamat sejahtera":
        return "__HINT__:Tamu mengucapkan salam lintas agama 'Salam Sejahtera'. " +
            "WAJIB awali balasan dengan 'Salam Sejahtera 🙏', " +
            "lalu sambut tamu dengan hangat sebagai resepsionis Kadena Glamping Dive Resort dan tanya ada yang bisa dibantu."

    case "ping":
        return "pong"

    case "halo":
        return "Halo " + namaPanggil + "! Ada yang bisa saya bantu? 😊"

    case "mau reservasi kamar", "reservasi kamar", "reservasi room", "booking room",
        "mau booking kamar", "cara booking kamar", "pesan kamar",
        "mau reservasi", "mau booking", "mau pesan kamar", "mau sewa kamar",
        "reservasi dong", "booking dong", "mau book",
        "pengen reservasi", "pengen booking", "ingin reservasi", "ingin booking":
        return "Dengan senang hati " + namaPanggil + "! 😊\n" +
            "Untuk reservasi kamar dengan aman, cepat, dan harga terbaik, " +
            "Bapak/Ibu bisa langsung klik link booking engine resmi di bawah ya:\n\n" +
            "🔗 " + generateBookingURL() + "\n\n" +
            "Di sana Bapak/Ibu bisa pilih tanggal, tipe kamar, dan selesaikan pembayaran secara instan. " +
            "Jika ada kendala, langsung kabari kami di sini ya " + namaPanggil + "! ✨\n\n" +
            "Atau hubungi tim Reservasi kami langsung:\n" +
            "📞 https://wa.me/" + KontakReservasi

    case "book a room", "room booking", "room reservation",
        "i want to book", "how to book", "check availability",
        "available rooms", "i want to reserve", "make a reservation":
        return "Sure " + rawNama + "! 😊\n" +
            "To book a room securely and get the best rate, click the official booking link below:\n\n" +
            "🔗 " + generateBookingURL() + "\n\n" +
            "You can select your dates, room type, and complete payment instantly. " +
            "Let me know if you need any help, " + rawNama + "! ✨\n\n" +
            "Or contact our Reservation team directly:\n" +
            "📞 https://wa.me/" + KontakReservasi

    case "saya mau sewa kamar", "saya mau sewa kamar min", "ada kamar apa saja", "ada tipe kamar apa saja":
        return "Boleh " + namaPanggil + " 😊\nBerikut tipe kamar yang tersedia di Kadena Glamping Dive Resort:\n\n" +
            "• Glamping Superior\n" +
            "• Glamping Deluxe\n" +
            "• Glamping Suite\n" +
            "Mau info harga atau fasilitas untuk salah satu tipe kamar di atas, " + namaPanggil + "?\n" +
            "Atau langsung booking di sini: 🔗 " + generateBookingURL()

    case "halo admin", "hai", "hi", "p", "min", "mas", "mba", "bu", "pak", "kak":
        return "Selamat datang di Kadena Glamping Dive Resort, " + namaPanggil + ". Ada yang bisa kami bantu? 😊"

    case "hello", "hi there":
        // English — gunakan rawNama tanpa prefix Indonesia
        rawDisplay := rawNama
        if rawDisplay == "" {
            rawDisplay = "there"
        }
        return "Welcome to Kadena Glamping Dive Resort, " + rawDisplay + "! How can I help you today? 😊"

    	case "good morning", "good afternoon", "good evening", "good night":
	
  
       jam := time.Now().In(wib).Hour()
	   fmt.Println("Jam sekarang menurut sistem bot:", jam)
        var sapaan string
        switch {
        case jam < 11:
            sapaan = "Good Morning 🌅"
        case jam < 15:
            sapaan = "Good Afternoon ☀️"
        case jam < 18:
            sapaan = "Good Evening 🌇"
        default:
            sapaan = "Good Night 🌙"
        }
        rawDisplay := rawNama
        if rawDisplay == "" {
            rawDisplay = "there"
        }
        return sapaan + "! How can I help you, " + rawDisplay + "? 😊"

    case "pagi", "siang", "sore", "malam",
        "selamat pagi", "selamat siang", "selamat sore", "selamat malam":
        // FIX: gunakan WIB agar jam tidak salah
   
       jam := time.Now().In(wib).Hour()
	   fmt.Println("Jam sekarang menurut sistem bot:", jam)
        var sapaan string
        switch {
        case jam < 11:
            sapaan = "Selamat Pagi 🌅"
        case jam < 15:
            sapaan = "Selamat Siang ☀️"
        case jam < 18:
            sapaan = "Selamat Sore 🌇"
        default:
            sapaan = "Selamat Malam 🌙"
        }
		

        // Personalisasi untuk tamu repeat/VIP
        if profil.KunjunganKe > 1 {
            return sapaan + "! Senang melihat " + namaPanggil + " kembali di Kadena Dive Resort! 😊\n" +
                "Seperti kunjungan sebelumnya, ada yang bisa kami bantu mengenai reservasi kamar, " +
                "ruang meeting, atau event? ✨"
        }
        return sapaan + "! Ada yang bisa kami bantu mengenai reservasi kamar, ruang meeting, " +
            "event, dan lain sebagainya 😊"
    }

    return ""
}
// =============================================================================
//  EVENT HANDLER
// =============================================================================

func eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		// Graceful shutdown: jangan terima/proses pesan baru jika bot sedang mematikan diri.
		if isShuttingDown.Load() {
			fmt.Println("[SHUTDOWN] Pesan baru diabaikan — bot sedang dalam proses shutdown.")
			return
		}
		activeWorkers.Add(1)
		go func() {
			defer activeWorkers.Done()
			handleMessage(v) // non-blocking: setiap pesan jalan di goroutine sendiri
		}()
	}
}

// =============================================================================
//  HANDLE PESAN MASUK — Inti Logika (Memory + Guest Context + Prioritas)
// =============================================================================

func handleMessage(msg *events.Message) {
	// Filter dasar
	if msg.Info.IsFromMe {
		// Intercept command admin dari perangkat terhubung
		var cmdText string
		if conv := msg.Message.GetConversation(); conv != "" {
			cmdText = conv
		} else if ext := msg.Message.GetExtendedTextMessage(); ext != nil {
			cmdText = ext.GetText()
		}
		cmdText = strings.TrimSpace(cmdText)

		// --- Command: !selesai <nomor_wa> → paksa state ke BOT ---
		if strings.HasPrefix(cmdText, "!selesai") {
			parts := strings.Fields(cmdText)
			if len(parts) >= 2 {
				targetNomor := parts[1]

				// Pelajari dari sesi Guest Services SEBELUM riwayat dihapus.
				prosesPembelajaranCS(targetNomor)

				forceOwnerBot(targetNomor, msg.Info.Chat)
				fmt.Printf("[ADMIN] !selesai — %s direset ke BOT oleh Guest Services Officer.\n", targetNomor)
			} else {
				sendReply(msg.Info.Chat, "[BOT] Format: !selesai <nomor_wa> — contoh: !selesai 6281234567890")
			}
			return
		}

		// --- Command: !handover <nomor_wa> → paksa state ke HUMAN ---
		if strings.HasPrefix(cmdText, "!handover") {
			parts := strings.Fields(cmdText)
			if len(parts) >= 2 {
				targetNomor := parts[1]
				forceOwnerHuman(targetNomor)
				fmt.Printf("[ADMIN] !handover — %s dipaksa ke HUMAN oleh operator.\n", targetNomor)
				sendReply(msg.Info.Chat, fmt.Sprintf("[BOT] Handover aktif untuk %s. Bot diam, Guest Services aktif.", targetNomor))
			} else {
				sendReply(msg.Info.Chat, "[BOT] Format: !handover <nomor_wa> — contoh: !handover 6281234567890")
			}
			return
		}

		// Bukan command — ini adalah balasan Guest Services ke customer.
		// Semua pesan IsFromMe non-command yang dikirim ke chat customer
		// dianggap sebagai balasan Layanan Tamu → set state HUMAN.
		nomorTarget := msg.Info.Chat.User
		if cmdText != "" && nomorTarget != "" {
			// Simpan ke riwayat DB untuk pembelajaran
			if err := simpanRiwayatChatDB(nomorTarget, "guest_services", cmdText); err != nil {
				fmt.Printf("[WARN] Gagal simpan balasan Layanan Tamu ke DB: %v\n", err)
			} else {
				fmt.Printf("[GUEST_SERVICES] Balasan ke %s tersimpan: %.50s\n", nomorTarget, cmdText)
			}
			// Update ownership → HUMAN (GS sedang aktif membalas)
			handleGuestServiceMessage(nomorTarget)
		}
		return
	}
	if msg.Info.IsGroup || msg.Info.Chat.String() == "status@broadcast" {
		return
	}

	// Abaikan pesan sebelum startup
	if msg.Info.Timestamp.Before(startupTime) {
		fmt.Printf("[SKIP] Pesan lama diabaikan — ts: %v (startup: %v)\n",
			msg.Info.Timestamp.Format("15:04:05"),
			startupTime.Format("15:04:05"))
		return
	}

	nomorWA := msg.Info.Sender.User

	// Per-user lock: cegah spam / double-response untuk 1 user
	processingMutex.Lock()
	if processingMap[nomorWA] {
		processingMutex.Unlock()
		fmt.Printf("[SKIP] %s masih diproses, pesan baru diabaikan.\n", nomorWA)
		return
	}
	processingMap[nomorWA] = true
	processingMutex.Unlock()
	defer func() {
		processingMutex.Lock()
		delete(processingMap, nomorWA)
		processingMutex.Unlock()
	}()

	// =========================================================================
	//  OWNERSHIP STATE MACHINE — cek state sebelum proses
	// =========================================================================
	ownerState := getOwnerState(nomorWA)
	fmt.Printf("[OWNER] State %s = %s\n", nomorWA, ownerState)

	shouldCallAI := handleCustomerTurn(nomorWA)
	if !shouldCallAI {
		// Bot diam — GS sedang/masih menangani percakapan ini
		fmt.Printf("[OWNER] Bot diam untuk %s (state: %s)\n", nomorWA, getOwnerState(nomorWA))
		return
	}

	// Ambil teks pesan
	var messageText string
	if conv := msg.Message.GetConversation(); conv != "" {
		messageText = conv
	} else if ext := msg.Message.GetExtendedTextMessage(); ext != nil {
		messageText = ext.GetText()
	}
	if messageText == "" {
		return
	}

	fmt.Printf("[MASUK] Dari: %-25s | Isi: %s\n", msg.Info.Sender.String(), messageText)

// Ambil atau buat sesi chat (Conversation Memory — persist tahan restart)
	sesiChatMutex.Lock()
	sesi, ada := sesiChatMap[nomorWA]
	if !ada {
		// Coba load riwayat dari database agar percakapan tidak hilang saat restart
		riwayatDB, errDB := loadRiwayatChatDariDB(nomorWA)
		if errDB != nil {
			fmt.Printf("[WARN] Gagal load riwayat dari DB untuk %s: %v\n", nomorWA, errDB)
			riwayatDB = []TurnChat{}
		}
		sesi = &SesiChat{Riwayat: riwayatDB, LastActivity: time.Now()} // Inisiasi nilai awal
		sesiChatMap[nomorWA] = sesi
		if len(riwayatDB) > 0 {
			fmt.Printf("[MEMORY] Loaded %d pesan dari DB untuk %s\n", len(riwayatDB), nomorWA)
		}
	}
	sesiChatMutex.Unlock()

	// TITIK 2: Update LastActivity setiap kali pesan baru masuk dari tamu
	sesi.Mu.Lock()
	sesi.LastActivity = time.Now()
	sesi.Mu.Unlock()

	// Ambil profil tamu (Guest Context) — dilindungi mutex untuk mencegah race condition
	profilTamuMutex.Lock()
	profil, err := getOrCreateProfilTamu(nomorWA)
	if err != nil {
		fmt.Printf("[WARN] Gagal ambil profil tamu %s: %v\n", nomorWA, err)
		profil = &ProfilTamu{NomorWA: nomorWA}
	}

	// Jika nama belum tersimpan tapi PushName tersedia, simpan
	if profil.Nama == "" && msg.Info.PushName != "" {
		profil.Nama = msg.Info.PushName
		if err := updateNamaTamu(nomorWA, msg.Info.PushName); err != nil {
			fmt.Printf("[WARN] Gagal simpan nama tamu %s: %v\n", nomorWA, err)
		}
	}
	profilTamuMutex.Unlock()

	if profil.KunjunganKe == 0 {
    sapaanPerdana := "Hai! Perkenalkan saya Dena, asisten virtual dari Kadena Glamping Dive Resort. Ada yang bisa Dena bantu untuk rencana liburan Anda?"
    
    // Naikkan status kunjungan agar interaksi berikutnya langsung masuk ke NLP/LLM
    _ = incrementKunjungan(nomorWA)
    
    // Kirim balasan langsung ke WhatsApp
    _ = clientWA.SendChatPresence(context.Background(), msg.Info.Chat, types.ChatPresencePaused, types.ChatPresenceMediaText)
    sendReply(msg.Info.Chat, sapaanPerdana)
    return // Hentikan eksekusi di sini agar tidak lanjut ke pemrosesan AI
}

	// Tampilkan indikator "sedang mengetik"
	_ = clientWA.SendChatPresence(context.Background(), msg.Info.Chat,
		types.ChatPresenceComposing, types.ChatPresenceMediaText)

	// =========================================================================
	//  PRIORITAS 1 — Template switch-case (cepat, deterministik)
	// =========================================================================
	normalizedText := strings.TrimSpace(strings.ToLower(messageText))
	perintahMaksud := cariTemplateMirip(normalizedText)

	var replyText string
	var salamHint string // hint dari template salam keagamaan

	if perintahMaksud != "" {
		replyText = jawabanTemplate(perintahMaksud, msg, profil)
		if strings.HasPrefix(replyText, "__HINT__:") {
			// Template salam — bukan static, kirim ke AI untuk diolah
			salamHint = strings.TrimPrefix(replyText, "__HINT__:")
			replyText = ""
			fmt.Printf("[SALAM] Detected: '%s' → diolah AI\n", perintahMaksud)
		} else if replyText != "" {
			fmt.Printf("[TEMPLATE] Match: '%s'\n", perintahMaksud)
		}
	}

	// =========================================================================
	//  PRIORITAS 2-4 — KoboLLM AI dengan Conversation Memory
	// =========================================================================
	if replyText == "" {
		if salamHint != "" {
			fmt.Println("[AI] Mengolah salam keagamaan dengan Gemini...")
		} else {
			fmt.Println("[AI] Tidak ada template cocok — meneruskan ke KoboLLM AI...")
		}

		// Delay natural
		time.Sleep(2500 * time.Millisecond)

		modelDipakai := LiteLLMModelRingan // salam pakai Lite (hemat)
		if salamHint == "" {
			modelDipakai = tentukanModel(messageText)
		}
		tierLabel := "Lite"
		if modelDipakai == LiteLLMModelKompleks {
			tierLabel = "Flash"
		} else if modelDipakai == LiteLLMModelPro {
			tierLabel = "Pro ⭐"
		}
		fmt.Printf("[AI] Model dipilih: %s (%s)\n", modelDipakai, tierLabel)
		jawaban, butuhHandover := tanyaGeminiAI(sesi, messageText, nomorWA, modelDipakai, salamHint)

		if butuhHandover {
			_ = clientWA.SendChatPresence(context.Background(), msg.Info.Chat,
				types.ChatPresencePaused, types.ChatPresenceMediaText)
			aktifkanHandoverOwner(nomorWA, msg.Info.Chat)
			return
		}

		replyText = jawaban
	} else {
		// Untuk template, delay lebih pendek
		time.Sleep(800 * time.Millisecond)
	}

	// Hentikan indikator mengetik
	_ = clientWA.SendChatPresence(context.Background(), msg.Info.Chat,
		types.ChatPresencePaused, types.ChatPresenceMediaText)

	// Kirim balasan
	if replyText != "" {
		sendReply(msg.Info.Chat, replyText)
	}

	// Deteksi & simpan kamar favorit dari pesan user SAJA (bukan dari jawaban AI)
	if replyText != "" {
		kamarTerdeteksi := deteksiKamarDariTeks(messageText)
		if kamarTerdeteksi != "" && kamarTerdeteksi != profil.KamarFavorit {
			if err := updateKamarFavorit(nomorWA, kamarTerdeteksi); err != nil {
				fmt.Printf("[WARN] Gagal update kamar favorit %s: %v\n", nomorWA, err)
			} else {
				fmt.Printf("[PROFIL] Kamar favorit %s diperbarui → %s\n", nomorWA, kamarTerdeteksi)
			}
		}
	}

	// Update metrik kunjungan (hanya untuk percakapan yang berhasil)
	kunjunganSekarang := profil.KunjunganKe
	if kunjunganSekarang == 0 {
		// Tamu baru — increment setelah percakapan pertama yang sukses
		if err := incrementKunjungan(nomorWA); err != nil {
			fmt.Printf("[WARN] Gagal increment kunjungan %s: %v\n", nomorWA, err)
		} else {
			kunjunganSekarang = 1 // Tamu baru sekarang jadi kunjungan ke-1
		}
	}

	// Auto-set VIP jika kunjungan sudah >= 5 — real-time check
	if !profil.StatusVIP && kunjunganSekarang >= 5 {
		if err := updateStatusVIP(nomorWA, true); err != nil {
			fmt.Printf("[WARN] Gagal set VIP %s: %v\n", nomorWA, err)
		} else {
			fmt.Printf("[VIP] Tamu %s otomatis dinaikkan ke status VIP (kunjungan ke-%d).\n",
				nomorWA, kunjunganSekarang)
		}
	}
}

// =============================================================================
//  KIRIM PESAN BALASAN
// =============================================================================

func sendReply(to types.JID, text string) {
	message := &waProto.Message{
		Conversation: proto.String(text),
	}
	_, err := clientWA.SendMessage(context.Background(), to, message)
	if err != nil {
		fmt.Printf("[ERROR] Gagal kirim ke %s: %v\n", to.String(), err)
	} else {
		fmt.Printf("[KIRIM] Ke: %-25s | Balasan: %.60s...\n", to.String(), text)
	}
}

// =============================================================================
//  MAIN
// =============================================================================

func main() {
	startHealthCheckServer()
	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║     WhatsApp Bot — KADENA AI BOT         ║")
	fmt.Println("║     v2.0 (Memory + Guest Context)        ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()

	// LANGKAH -1: Siapkan context aplikasi untuk graceful shutdown.
	// appCancel akan dipanggil saat sinyal SIGINT/SIGTERM diterima,
	// menandai semua goroutine background (mis. session cleanup) untuk berhenti.
	appCtx, appCancel = context.WithCancel(context.Background())
	defer appCancel()

	// LANGKAH 0: Inisialisasi database tamu
	fmt.Println("[INIT] Menyiapkan database profil tamu...")
	if err := initDatabaseTamu(); err != nil {
		fmt.Printf("[FATAL] Gagal init database tamu: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[INIT] Database tamu siap.")
	fmt.Println()

	// LANGKAH 0a: Inisialisasi knowledge base hotel
	fmt.Println("[INIT] Menyiapkan knowledge base hotel...")
	if err := initKnowledgeBase(); err != nil {
		fmt.Printf("[WARN] Gagal init knowledge base: %v — bot tetap jalan tanpa KB lokal.\n", err)
	}
	seedKnowledgeBase()
	fmt.Println()

	// LANGKAH 0b: Start session cleanup routine (anti memory leak)
	startSessionCleanup()
	fmt.Println()

	// LANGKAH 0c: Muat status handover aktif dari DB (tahan restart)
	fmt.Println("[INIT] Memuat status ownership dari database...")
	if err := loadHandoverDariDB(); err != nil {
		fmt.Printf("[WARN] Gagal muat ownership dari DB: %v\n", err)
	} else {
		ownerMutex.RLock()
		jumlahHuman := 0
		for _, o := range conversationOwners {
			if o.State == StateHuman {
				jumlahHuman++
			}
		}
		ownerMutex.RUnlock()
		if jumlahHuman > 0 {
			fmt.Printf("[INIT] %d sesi HUMAN aktif dimuat dari DB.\n", jumlahHuman)
		} else {
			fmt.Println("[INIT] Tidak ada sesi HUMAN aktif.")
		}
	}
	fmt.Println()

	// Catat waktu startup
	startupTime = time.Now()
	fmt.Printf("[INFO] Startup time: %s\n", startupTime.Format("2006-01-02 15:04:05 WIB"))
	fmt.Println()

	// LANGKAH 2: Setup database & klien WhatsApp
	dbLog := waLog.Noop
	clientLog := waLog.Noop

	container, err := sqlstore.New(context.Background(), "sqlite",
		"file:whatsapp_session.db?_pragma=foreign_keys(1)", dbLog)
	if err != nil {
		fmt.Printf("[FATAL] Gagal membuka database: %v\n", err)
		os.Exit(1)
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		fmt.Printf("[FATAL] Gagal mengambil data device: %v\n", err)
		os.Exit(1)
	}

	clientWA = whatsmeow.NewClient(deviceStore, clientLog)
	clientWA.AddEventHandler(eventHandler)

	// LANGKAH 3: Login atau gunakan sesi tersimpan
	if clientWA.Store.ID == nil {
		fmt.Println("[INFO] Belum ada sesi. Menyiapkan QR Code...")
		qrChan, _ := clientWA.GetQRChannel(context.Background())

		err = clientWA.Connect()
		if err != nil {
			fmt.Printf("[FATAL] Gagal koneksi awal: %v\n", err)
			os.Exit(1)
		}

		for evt := range qrChan {
			switch evt.Event {
			case "code":
				fmt.Println("┌─────────────────────────────────────┐")
				fmt.Println("│         SCAN QR CODE INI            │")
				fmt.Println("└─────────────────────────────────────┘")
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				fmt.Println()
			case "success":
				fmt.Println("[✓] Login sukses! Sesi disimpan.")
			}
		}
	} else {
		fmt.Println("[INFO] Menghubungkan otomatis dengan sesi tersimpan...")
		err := clientWA.Connect()
		if err != nil {
			fmt.Printf("[FATAL] Gagal terhubung: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[✓] Terhubung ke WhatsApp! Bot aktif.")
	}

	fmt.Println()
	fmt.Println("[INFO] Bot siap. Fitur aktif:")
	fmt.Println("       • Knowledge Base Lokal (hotel_knowledge.db, 7 kategori)")
	fmt.Println("       • Conversation Memory (5-turn context, persisted ke DB)")
	fmt.Println("       • Guest Profile Database (Kamar Favorit + VIP auto-detect)")
	fmt.Println("       • Concurrent Message Handling (goroutine per user)")
	fmt.Println("       • Pre-sorted Template Matching")
	fmt.Println("       • Ownership State Machine (BOT / HUMAN / WAITING_HUMAN)")
	fmt.Println("       • Auto-Timeout WAITING_HUMAN → BOT (3 menit)")
	fmt.Println("       • Commands: !handover <nomor> | !selesai <nomor>")
	fmt.Println("       • Auto-Learning KB dari sesi Layanan Tamu (kb_qna)")
	fmt.Println("       • Dual Model: kompleks=flash, ringan=flash-lite, super kompleks=pro")
	fmt.Println()
	fmt.Println("[INFO] Tekan Ctrl+C (SIGINT) atau kirim SIGTERM untuk berhenti secara graceful.")

	// =========================================================================
	//  GRACEFUL SHUTDOWN
	// =========================================================================
	//
	// Alur:
	//  1. Tunggu sinyal SIGINT (Ctrl+C) atau SIGTERM (systemd/PM2/docker stop).
	//  2. Set isShuttingDown=true → eventHandler berhenti menerima pesan baru.
	//  3. Buat shutdownCtx dengan timeout sebagai batas waktu total shutdown.
	//  4. Disconnect dari WhatsApp → tidak ada event baru yang masuk lagi.
	//  5. Batalkan appCtx → goroutine background (session cleanup) berhenti.
	//  6. Tunggu activeWorkers (semua goroutine yang sedang memproses pesan
	//     atau menulis ke DB) selesai, maksimal selama shutdownCtx berjalan.
	//  7. Tutup semua koneksi SQLite (hotel_knowledge.db, guest_profiles.db,
	//     whatsapp_session.db) dengan aman.
	//  8. Bersihkan resource HTTP client.
	// =========================================================================
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	sig := <-stop
	signal.Stop(stop) // sinyal kedua (mis. Ctrl+C lagi) tidak akan ditangani ulang di sini

	fmt.Println()
	fmt.Printf("[SHUTDOWN] Sinyal diterima (%v). Memulai graceful shutdown...\n", sig)

	// LANGKAH 1/6 — Berhenti menerima pesan baru.
	// eventHandler langsung mengabaikan event *events.Message setelah flag ini true.
	isShuttingDown.Store(true)
	fmt.Println("[SHUTDOWN] (1/6) Penerimaan pesan baru dihentikan.")

	// LANGKAH 2/6 — Buat context shutdown dengan timeout sebagai batas waktu
	// total proses shutdown, agar aplikasi tidak menggantung selamanya.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer shutdownCancel()
	fmt.Printf("[SHUTDOWN] (2/6) Batas waktu shutdown: %s.\n", ShutdownTimeout)

	// LANGKAH 3/6 — Putuskan koneksi WhatsApp.
	// Setelah ini whatsmeow tidak akan memanggil eventHandler lagi.
	fmt.Println("[SHUTDOWN] (3/6) Memutus koneksi WhatsApp...")
	clientWA.Disconnect()
	fmt.Println("[SHUTDOWN]       Koneksi WhatsApp diputus.")

	// LANGKAH 4/6 — Hentikan goroutine background (session cleanup, dll)
	// dengan membatalkan appCtx, dan stop ticker secara eksplisit.
	fmt.Println("[SHUTDOWN] (4/6) Menghentikan goroutine background...")
	appCancel()
	if cleanupTicker != nil {
		cleanupTicker.Stop()
	}

	// LANGKAH 5/6 — Tunggu seluruh goroutine aktif selesai:
	//  - handleMessage yang sedang berjalan (termasuk panggilan ke KoboLLM/Gemini)
	//  - goroutine persist handover (simpanHandoverDB/resetHandoverDB)
	//  - goroutine session cleanup
	// Dibatasi oleh shutdownCtx agar tidak menggantung selamanya.
	fmt.Println("[SHUTDOWN] (5/6) Menunggu goroutine aktif selesai...")
	workersDone := make(chan struct{})
	go func() {
		activeWorkers.Wait()
		close(workersDone)
	}()

	select {
	case <-workersDone:
		fmt.Println("[SHUTDOWN]       Semua goroutine aktif telah selesai.")
	case <-shutdownCtx.Done():
		fmt.Println("[SHUTDOWN]       Timeout tercapai — beberapa goroutine mungkin belum selesai, " +
			"lanjut menutup resource (DB ditutup setelah ini, query yang masih berjalan akan gagal).")
	}

	// LANGKAH 6/6 — Tutup semua koneksi SQLite dengan aman, lalu bersihkan
	// resource lain (HTTP client). Dilakukan SETELAH goroutine aktif selesai
	// (atau timeout) agar tidak ada query yang berjalan di koneksi yang sudah ditutup.
	fmt.Println("[SHUTDOWN] (6/6) Menutup koneksi database & resource...")

	if dbKB != nil {
		if err := dbKB.Close(); err != nil {
			fmt.Printf("[SHUTDOWN]       hotel_knowledge.db: gagal ditutup — %v\n", err)
		} else {
			fmt.Println("[SHUTDOWN]       hotel_knowledge.db ditutup.")
		}
	}

	if dbTamu != nil {
		if err := dbTamu.Close(); err != nil {
			fmt.Printf("[SHUTDOWN]       guest_profiles.db: gagal ditutup — %v\n", err)
		} else {
			fmt.Println("[SHUTDOWN]       guest_profiles.db ditutup.")
		}
	}

	if err := container.Close(); err != nil {
		fmt.Printf("[SHUTDOWN]       whatsapp_session.db: gagal ditutup — %v\n", err)
	} else {
		fmt.Println("[SHUTDOWN]       whatsapp_session.db ditutup.")
	}

	httpClientGemini.CloseIdleConnections()
	fmt.Println("[SHUTDOWN]       Koneksi HTTP idle (KoboLLM) dibersihkan.")

	fmt.Println()
	fmt.Println("[SHUTDOWN] Selesai. Bot berhenti dengan aman. 👋")
}