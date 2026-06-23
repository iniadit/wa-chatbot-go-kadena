package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// hasilEkstraksiQnA adalah struktur output dari LLM saat menganalisis sesi CS
type hasilEkstraksiQnA struct {
	Berguna   bool   `json:"berguna"`
	Judul     string `json:"judul"`
	Konten    string `json:"konten"`
	KataKunci string `json:"kata_kunci"`
}

// prosesPembelajaranCS dipanggil SEBELUM forceOwnerBot (!selesai), agar chat_history
// masih tersedia untuk dianalisis. Role "guest_services" digunakan sejak migrasi DB.
func prosesPembelajaranCS(nomorWA string) {
	riwayat, err := loadRiwayatChatDariDB(nomorWA)
	if err != nil {
		fmt.Printf("[LEARN] Gagal load riwayat %s: %v\n", nomorWA, err)
		return
	}
	if len(riwayat) == 0 {
		return
	}

	var transkrip strings.Builder
	for _, t := range riwayat {
		switch t.Role {
		case "user":
			transkrip.WriteString("Customer: " + t.Teks + "\n")
		case "guest_services": // role resmi sejak migrasi DB (menggantikan "cs" lama)
			transkrip.WriteString("CS Admin: " + t.Teks + "\n")
		case "model":
			transkrip.WriteString("Bot: " + t.Teks + "\n")
		}
	}
	if transkrip.Len() == 0 {
		return
	}

	hasil, err := ekstrakQnADariTranskrip(transkrip.String())
	if err != nil {
		fmt.Printf("[LEARN] Gagal ekstrak Q&A untuk %s: %v\n", nomorWA, err)
		return
	}

	if !hasil.Berguna || strings.TrimSpace(hasil.Judul) == "" || strings.TrimSpace(hasil.Konten) == "" {
		fmt.Printf("[LEARN] Sesi %s tidak menghasilkan Q&A berguna — di-skip.\n", nomorWA)
		return
	}

	if err := TambahKnowledge("kb_qna", hasil.Judul, hasil.Konten, hasil.KataKunci); err != nil {
		fmt.Printf("[LEARN] Gagal simpan KB baru dari sesi %s: %v\n", nomorWA, err)
		return
	}
	fmt.Printf("[LEARN] ✅ KB baru ditambahkan dari sesi %s → \"%s\"\n", nomorWA, hasil.Judul)
}

// ekstrakQnADariTranskrip mengirim transkrip ke Flash-Lite dan minta hasil dalam JSON.
func ekstrakQnADariTranskrip(transkrip string) (*hasilEkstraksiQnA, error) {
	systemPrompt := `Kamu menganalisis transkrip percakapan antara Customer, Bot AI, dan CS Admin Le Polonia Hotel.

TUGAS: Tentukan apakah dari percakapan ini ada SATU pasangan Tanya-Jawab yang layak disimpan sebagai knowledge base hotel untuk menjawab pertanyaan serupa di masa depan.

ANGGAP TIDAK BERGUNA jika:
- CS hanya basa-basi ("oke", "siap", "baik", "ditunggu ya", "terima kasih")
- Tidak ada informasi faktual baru tentang hotel (harga, fasilitas, kebijakan, dll)
- CS hanya mengarahkan ke departemen lain tanpa memberi info konkret

Jika BERGUNA, ekstrak:
- judul: ringkas & jelas, misal "Kebijakan Late Checkout"
- konten: rangkuman informasi dari CS dalam 2-5 kalimat, ditulis netral & informatif, siap dipakai bot AI menjawab pertanyaan serupa
- kata_kunci: 3-6 kata kunci dipisah koma untuk pencarian

Balas HANYA dengan JSON murni (tanpa markdown/backtick), format persis:
{"berguna": true/false, "judul": "...", "konten": "...", "kata_kunci": "..."}`

	requestBody, err := json.Marshal(map[string]interface{}{
		"model": LiteLLMModelRingan,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": transkrip},
		},
		"temperature": 0.2,
		"max_tokens":  400,
	})
	if err != nil {
		return nil, fmt.Errorf("gagal susun payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(appCtx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", LiteLLMBaseURL+"/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("gagal buat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+geminiAPIKey())

	resp, err := httpClientGemini.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal koneksi: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("gagal baca body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var data KoboLLMResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("gagal parse response: %w", err)
	}
	if len(data.Choices) == 0 || data.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("respons kosong")
	}

	teks := strings.TrimSpace(data.Choices[0].Message.Content)
	teks = strings.TrimPrefix(teks, "```json")
	teks = strings.TrimPrefix(teks, "```")
	teks = strings.TrimSuffix(teks, "```")
	teks = strings.TrimSpace(teks)

	var hasil hasilEkstraksiQnA
	if err := json.Unmarshal([]byte(teks), &hasil); err != nil {
		return nil, fmt.Errorf("gagal parse JSON hasil ekstraksi: %w (raw: %.200s)", err, teks)
	}
	return &hasil, nil
}