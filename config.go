package main

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	GeminiAPIKey         string
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

	HumanResponseTimeout time.Duration
	SesiChatExpiry       time.Duration
	AIRetryMaxAttempts   int
	AIRetryInitialDelay  time.Duration
	AIRetryMaxDelay      time.Duration

	// FASE 4B.5: Owner TTL Config
	OwnerTTLBot          time.Duration
	OwnerTTLHuman        time.Duration
	OwnerTTLWaitingHuman time.Duration
	OwnerCleanupInterval time.Duration
}

var AppConfig *Config

func getEnvAsInt(key string, fallback int) int {
	val := getEnv(key, "")
	if val == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return parsed
}

func LoadConfig() {
	loadDotEnv(".env")

	AppConfig = &Config{
		GeminiAPIKey:         getEnv("GEMINI_API_KEY", ""),
		LiteLLMBaseURL:       getEnv("LITE_LLM_BASE_URL", "https://lite.koboillm.com/v1"),
		LiteLLMModelRingan:   getEnv("LITE_LLM_MODEL_RINGAN", "gemini-2.5-flash-lite"),
		LiteLLMModelKompleks: getEnv("LITE_LLM_MODEL_KOMPLEKS", "gemini-2.5-flash"),
		LiteLLMModelPro:      getEnv("LITE_LLM_MODEL_PRO", "gemini-2.5-pro"),

		RoomURLMap: getEnv("ROOM_URL_MAP", `PETA URL TIPE KAMAR (gunakan URL yang sesuai dengan tipe kamar yang ditanyakan):
- Glamping Superior    → https://kadenaglampingdiveresort.com/room/glamping-superior/
- Glamping Deluxe      → https://kadenaglampingdiveresort.com/room/single-room/
- Glamping Suite       → https://kadenaglampingdiveresort.com/room/junior-suite/`),

		InfoHotelFallback: getEnv("INFO_HOTEL_FALLBACK", `Kadena Glamping dive Resort
- Lokasi: Jl. Raya Anyer-Sirih, Kamasan, Kec. Cinangka, Kabupaten Serang, Banten 42167
- Tipe Kamar: Glamping Superior, Glamping Deluxe, Glamping Suite 
- Meeting & Event: Atur acara Anda bersama kami, 2 ruang pertemuan dalam ruangan kami dapat menampung hingga 200 orang dengan fasilitas pertemuan standar. Kami juga dapat menangani acara sosial luar ruangan Anda di taman atau di "K Resto & Lounge" terbuka kami dengan pemandangan laut yang menakjubkan
- Fasilitas: Resor tepi pantai dengan 32 Glamping, Kolam Renang Infinity, Pusat Selam, Olahraga Air, Restoran & Bar, Taman Bermain Anak, Ruang Serbaguna, DJ dan Hiburan Langsung (Band) setiap Jumat & Sabtu pukul 16:00-22:00
- Check-in: 14.00 WIB | Check-out: 12.00 WIB`),

		BookingURLBase: getEnv("BOOKING_URL_BASE", "https://www.tiket.com/id-id/hotel/indonesia/kadena-glamping-dive-resort-410001635521096877"),

		KontakReservasi: getEnv("KONTAK_RESERVASI", "6281211493810"),
		KontakSales:     getEnv("KONTAK_SALES", "6281211493810"),
		KontakFO:        getEnv("KONTAK_FO", "6281211493810"),
		KontakResto:     getEnv("KONTAK_RESTO", "6281211493810"),

		HumanResponseTimeout: getEnvAsDuration("HUMAN_RESPONSE_TIMEOUT", 3*time.Minute),
		SesiChatExpiry:       getEnvAsDuration("SESI_CHAT_EXPIRY", 24*time.Hour),

		// FASE 4A: Inisialisasi Retry AI Config
		AIRetryMaxAttempts:  getEnvAsInt("AI_RETRY_MAX_ATTEMPTS", 4),
		AIRetryInitialDelay: getEnvAsDuration("AI_RETRY_INITIAL_DELAY", 2*time.Second),
		AIRetryMaxDelay:     getEnvAsDuration("AI_RETRY_MAX_DELAY", 8*time.Second),

		// FASE 4B.5: Inisialisasi Owner TTL Config
		OwnerTTLBot:          getEnvAsDuration("OWNER_TTL_BOT", 24*time.Hour),
		OwnerTTLHuman:        getEnvAsDuration("OWNER_TTL_HUMAN", 8*time.Hour),
		OwnerTTLWaitingHuman: getEnvAsDuration("OWNER_TTL_WAITING_HUMAN", 30*time.Minute),
		OwnerCleanupInterval: getEnvAsDuration("OWNER_CLEANUP_INTERVAL", 24*time.Hour),
	}

	if AppConfig.GeminiAPIKey == "" {
		log.Println("[FATAL] GEMINI_API_KEY belum di-set di .env atau environment variable.")
		os.Exit(1)
	}
}

// loadDotEnv adalah parser manual .env ringan tanpa perlu mendownload library godotenv
func loadDotEnv(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			os.Setenv(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}

func getEnvAsDuration(key string, fallback time.Duration) time.Duration {
	valStr := getEnv(key, "")
	if valStr == "" {
		return fallback
	}
	d, err := time.ParseDuration(valStr)
	if err != nil {
		log.Printf("[WARN] Format durasi tidak valid untuk %s: %v. Menggunakan default: %v\n", key, err, fallback)
		return fallback
	}
	return d
}