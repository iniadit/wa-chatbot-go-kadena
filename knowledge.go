package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// =============================================================================
//  HOTEL KNOWLEDGE BASE — RAG Lokal
//  File: hotel_knowledge.db
//  Gemini cari di sini dulu sebelum fallback ke website cache
// =============================================================================

// dbKB adalah koneksi ke hotel_knowledge.db (terpisah dari guest_profiles.db)
var dbKB *sql.DB

// daftarTabel berisi semua tabel kategori yang tersedia
var daftarTabel = []string{
	"kb_umum",
    "kb_qna",
    "kb_kamar",
    "kb_fasilitas",
	"contact_us",
}

// kategoriKeywords memetakan tabel → kata kunci pemicu pencarian
var kategoriKeywords = map[string][]string{
	"kb_umum": {
		"alamat", "lokasi", "dimana", "di mana", "address", "telepon", 
		"nomor", "kontak", "email", "maps", "google maps", "koordinat", 
		"arah", "rute", "jalan", "ke sini", "tentang", "about", "sejarah",
	},
	"kb_kamar": {
		"kamar", "room", "glamping", "superior", "deluxe", "suite", 
		"bed", "kasur", "twin", "king", "queen", "hollywood", "balkon", 
		"balcony", "view", "pemandangan", "laut", "strait", "sunda", 
		"harga kamar", "rate", "kapasitas", "dewasa", "anak", "extra bed",
	},
	"kb_fasilitas": {
		"fasilitas", "facilities", "playground", "anak", "pantai", 
		"beachfront", "hiburan", "entertainment", "band", "dj", 
		"live music", "parkir", "parking", "keamanan", "security", 
		"taman", "garden", "towel", "handuk",
	},
	"contact_us": {
		"hubungi kami", "telepon", "whatsapp", "email", "lokasi", 
    	"alamat", "instagram", "bantuan", "support", "call center", 
    	"reservasi",
	},
}

// =============================================================================
//  INIT & SCHEMA
// =============================================================================

func initKnowledgeBase() error {
	db, err := sql.Open("sqlite", "file:hotel_knowledge.db?_pragma=foreign_keys(1)")
	if err != nil {
		return fmt.Errorf("gagal buka hotel_knowledge.db: %w", err)
	}
	dbKB = db

	for _, tabel := range daftarTabel {
		query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			judul       TEXT    NOT NULL,
			konten      TEXT    NOT NULL,
			kata_kunci  TEXT    NOT NULL DEFAULT '',
			created_at  TEXT    NOT NULL,
			updated_at  TEXT    NOT NULL
		);`, tabel)
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("gagal init tabel %s: %w", tabel, err)
		}
	}

	// Hitung total entri yang sudah ada
	total := hitungTotalEntri()
	fmt.Printf("[KB] hotel_knowledge.db siap — %d entri tersimpan.\n", total)
	return nil
}

func hitungTotalEntri() int {
	var total int
	for _, tabel := range daftarTabel {
		var n int
		_ = dbKB.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s`, tabel)).Scan(&n)
		total += n
	}
	return total
}

// =============================================================================
//  PENCARIAN UTAMA
// =============================================================================

// cariDiKnowledgeBase adalah entry point pencarian KB.
// Selalu cari di kb_umum, lalu tambah tabel kategori relevan.
// Mengembalikan string kosong jika tidak ada hasil.
func cariDiKnowledgeBase(pesan string) string {
	if dbKB == nil || isShuttingDown.Load() {
		return ""
	}

	pesanLower := strings.ToLower(pesan)

	// Selalu sertakan kb_umum dan kb_qna
	tabelDicari := map[string]bool{"kb_umum": true, "kb_qna": true}

	for tabel, keywords := range kategoriKeywords {
		for _, kw := range keywords {
			if strings.Contains(pesanLower, kw) {
				tabelDicari[tabel] = true
				break
			}
		}
	}

	var gabungan strings.Builder

	if hasil := cariDiTabel("kb_umum", pesanLower); hasil != "" {
		gabungan.WriteString(hasil)
	}
	if hasil := cariDiTabel("kb_qna", pesanLower); hasil != "" {
		gabungan.WriteString(hasil)
		fmt.Printf("[KB] Hit di tabel: kb_qna\n")
	}

	for tabel := range tabelDicari {
		if tabel == "kb_umum" || tabel == "kb_qna" {
			continue
		}
		if hasil := cariDiTabel(tabel, pesanLower); hasil != "" {
			gabungan.WriteString(hasil)
			fmt.Printf("[KB] Hit di tabel: %s\n", tabel)
		}
	}

	return strings.TrimSpace(gabungan.String())
}

// cariDiTabel melakukan pencarian LIKE di satu tabel.
// Mengembalikan max 3 hasil teratas yang relevan.
// Relevance scoring: judul match (3 poin), konten match (2 poin), kata_kunci match (1 poin)
func cariDiTabel(tabel, pesanLower string) string {
	if isShuttingDown.Load() {
		return ""
	}
	kata := strings.Fields(pesanLower)

	// Kumpulkan kata dengan panjang minimal 3 karakter
	var kataFilter []string
	for _, k := range kata {
		if len(k) >= 3 {
			kataFilter = append(kataFilter, k)
		}
	}
	if len(kataFilter) == 0 {
		return ""
	}

	// Batasi maksimal 5 kata agar query tidak terlalu panjang
	if len(kataFilter) > 5 {
		kataFilter = kataFilter[:5]
	}

// Build kondisi OR dan relevance scoring
	var kondisi []string
	var scoreParts []string
	var args []interface{}
	for _, k := range kataFilter {
		kondisi = append(kondisi,
			`(LOWER(judul) LIKE ? OR LOWER(konten) LIKE ? OR LOWER(kata_kunci) LIKE ?)`)
		pola := "%" + k + "%"
		args = append(args, pola, pola, pola) 
		scoreParts = append(scoreParts,
			`(CASE WHEN LOWER(judul) LIKE ? THEN 3 ELSE 0 END) + (CASE WHEN LOWER(konten) LIKE ? THEN 2 ELSE 0 END) + (CASE WHEN LOWER(kata_kunci) LIKE ? THEN 1 ELSE 0 END)`)
		args = append(args, pola, pola, pola)
	}

	scoreExpr := strings.Join(scoreParts, " + ")
	query := fmt.Sprintf(
		`SELECT judul, konten FROM %s WHERE %s ORDER BY (%s) DESC, id DESC LIMIT 3`,
		tabel, strings.Join(kondisi, " OR "), scoreExpr)

	rows, err := dbKB.Query(query, args...)
	if err != nil {
		fmt.Printf("[KB] Gagal query %s: %v\n", tabel, err)
		return ""
	}
	defer rows.Close()

	var hasil strings.Builder
	for rows.Next() {
		var judul, konten string
		if err := rows.Scan(&judul, &konten); err == nil {
			hasil.WriteString(fmt.Sprintf("**%s**\n%s\n\n", judul, konten))
		}
	}

	return hasil.String()
}

// =============================================================================
//  CRUD HELPERS — untuk mengisi data KB
// =============================================================================

// TambahKnowledge menambahkan satu entri ke tabel KB.
// Contoh: TambahKnowledge("kb_kamar", "Junior Suite", "Luas 45m², king bed...", "junior suite,kamar,suite")
func TambahKnowledge(tabel, judul, konten, kataKunci string) error {
	if dbKB == nil {
		return fmt.Errorf("knowledge base belum diinisialisasi")
	}
	if !tabelValid(tabel) {
		return fmt.Errorf("tabel '%s' tidak dikenal, pilih: %s", tabel, strings.Join(daftarTabel, ", "))
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := dbKB.Exec(
		fmt.Sprintf(`INSERT INTO %s (judul, konten, kata_kunci, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`, tabel),
		judul, konten, kataKunci, now, now)
	if err != nil {
		return fmt.Errorf("gagal insert ke %s: %w", tabel, err)
	}
	fmt.Printf("[KB] Entri baru ditambahkan → %s | %s\n", tabel, judul)
	return nil
}

// UpdateKnowledge memperbarui entri berdasarkan ID.
func UpdateKnowledge(tabel string, id int, judul, konten, kataKunci string) error {
	if dbKB == nil {
		return fmt.Errorf("knowledge base belum diinisialisasi")
	}
	if !tabelValid(tabel) {
		return fmt.Errorf("tabel '%s' tidak dikenal", tabel)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := dbKB.Exec(
		fmt.Sprintf(`UPDATE %s SET judul=?, konten=?, kata_kunci=?, updated_at=? WHERE id=?`, tabel),
		judul, konten, kataKunci, now, id)
	if err != nil {
		return fmt.Errorf("gagal update %s id=%d: %w", tabel, id, err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("entri id=%d tidak ditemukan di tabel %s", id, tabel)
	}
	fmt.Printf("[KB] Entri diperbarui → %s | id=%d\n", tabel, id)
	return nil
}

// HapusKnowledge menghapus entri berdasarkan ID.
func HapusKnowledge(tabel string, id int) error {
	if dbKB == nil {
		return fmt.Errorf("knowledge base belum diinisialisasi")
	}
	if !tabelValid(tabel) {
		return fmt.Errorf("tabel '%s' tidak dikenal", tabel)
	}
	_, err := dbKB.Exec(fmt.Sprintf(`DELETE FROM %s WHERE id=?`, tabel), id)
	if err != nil {
		return fmt.Errorf("gagal hapus dari %s id=%d: %w", tabel, id, err)
	}
	fmt.Printf("[KB] Entri dihapus → %s | id=%d\n", tabel, id)
	return nil
}

// ListKnowledge menampilkan semua entri dari satu tabel (untuk debugging/review).
func ListKnowledge(tabel string) ([]map[string]interface{}, error) {
	if dbKB == nil {
		return nil, fmt.Errorf("knowledge base belum diinisialisasi")
	}
	if !tabelValid(tabel) {
		return nil, fmt.Errorf("tabel '%s' tidak dikenal", tabel)
	}
	rows, err := dbKB.Query(
		fmt.Sprintf(`SELECT id, judul, kata_kunci, updated_at FROM %s ORDER BY id`, tabel))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hasil []map[string]interface{}
	for rows.Next() {
		var id int
		var judul, kataKunci, updatedAt string
		if err := rows.Scan(&id, &judul, &kataKunci, &updatedAt); err == nil {
			hasil = append(hasil, map[string]interface{}{
				"id":         id,
				"judul":      judul,
				"kata_kunci": kataKunci,
				"updated_at": updatedAt,
			})
		}
	}
	return hasil, rows.Err()
}

// tabelValid memvalidasi nama tabel untuk mencegah SQL injection
func tabelValid(tabel string) bool {
	for _, t := range daftarTabel {
		if t == tabel {
			return true
		}
	}
	return false
}