package main

import "fmt"

// =============================================================================
//  SEED KNOWLEDGE BASE — Isi semua informasi hotel di sini
//
//  Cara pakai:
//  1. Isi semua data hotel di setiap fungsi di bawah
//  2. Di main.go, tambahkan pemanggilan seedKnowledgeBase()
//     tepat setelah initKnowledgeBase():
//
//     if err := initKnowledgeBase(); err != nil {
//         fmt.Printf("[WARN] Gagal init knowledge base: %v\n", err)
//     }
//     seedKnowledgeBase()  // ← tambahkan baris ini
//
//  Catatan:
//  - Data hanya diisi SEKALI saat DB masih kosong (cek otomatis)
//  - Untuk update data: hapus file hotel_knowledge.db lalu restart bot
//  - Setiap TambahKnowledge() punya 4 parameter:
//      1. tabel      → nama kategori (jangan diubah)
//      2. judul      → nama/topik entri (singkat & jelas)
//      3. konten     → isi informasi lengkap yang akan dibaca bot
//      4. kata_kunci → kata-kata pemicu pencarian, pisah koma
// =============================================================================

func seedKnowledgeBase() {
	// Cek apakah KB sudah terisi — hindari duplikat saat restart
	if hitungTotalEntri() > 0 {
		fmt.Println("[SEED] Knowledge base sudah terisi, skip seeding.")
		return
	}

	fmt.Println("[SEED] Mengisi knowledge base hotel...")

    seedUmum()
	seedKamar()
	seedFasilitas()
    seedContactUs()


	

	fmt.Printf("[SEED] Selesai! Total entri: %d\n", hitungTotalEntri())
}

// =============================================================================
//  kb_umum — Informasi umum hotel
//  Cocok untuk: alamat, jam operasional, kebijakan, kontak, WiFi, parkir, dll.
// =============================================================================

func seedUmum() {
    data := []struct{ judul, konten, kataKunci string }{
         {
            judul:    "Profil Hotel",
            konten:   `Aryan by Kadena adalah resort tepi pantai yang terletak di Blue Ring East Biluhu Beach, Provinsi Gorontalo. Resort ini dirancang sebagai tempat berkumpul ideal untuk liburan panjang, perjalanan singkat, menyelam, snorkeling, olahraga air, bersantap lokal, dan acara sosial. Tamu dapat bersantai di restoran dengan pemandangan laut serta menikmati layanan khas resort.`,
            kataKunci: "hotel,resort,profil,tentang,Aryan,Kadena,Gorontalo,tepipantai",
        },
        {
            judul:    "Kamar Tamu",
            konten:   `Resort ini menawarkan total 15 kamar, terdiri dari 1 Family Room (52 m²) dan 14 Deluxe Room (32 m²). Semua kamar memiliki balkon pribadi, banyak di antaranya dengan pemandangan laut. Kamar dilengkapi tempat tidur gaya Hollywood yang dapat menampung hingga 3 orang dewasa, shower air panas dan dingin, serta perlengkapan tamu (guest amenities).`,
            kataKunci: "kamar,tamu,deluxe,family,balkon,pemandangan,laut,shore,amenities",
        },
        {
            judul:    "Fasilitas Resort",
            konten:   `Fasilitas utama meliputi Dive Center untuk aktivitas menyelam, Watersports untuk berbagai olahraga air, restoran dengan pemandangan laut, Wi-Fi gratis, shower air panas dan dingin, tempat parkir gratis, dan guest amenities.`,
            kataKunci: "fasilitas,divecenter,watersports,restoran,wifi,parking,shower,amenities",
        },
        {
            judul:    "Lokasi & Kontak",
            konten:   `Aryan by Kadena beralamat di Blue Ring East Biluhu Beach, Provinsi Gorontalo. Untuk informasi lebih lanjut, hubungi +62 852 1111 5115 atau email info@aryanbykadena.com.`,
            kataKunci: "lokasi,kontak,alamat,Gorontalo,telepon,email",
        },
        {
            judul: "Google Maps",
            konten: `- https://maps.app.goo.gl/YU7KZrznS4zUfarG8.`,
            kataKunci: "alamat,koordinat,lokasi,dimana,di mana,address,google maps",
        },

		// Tambahkan entri kb_umum lainnya di sini...
	}

	for _, d := range data {
		if err := TambahKnowledge("kb_umum", d.judul, d.konten, d.kataKunci); err != nil {
			fmt.Printf("[SEED][ERROR] kb_umum '%s': %v\n", d.judul, err)
		}
	}
}

// =============================================================================
//  kb_kamar — Tipe kamar & detail fasilitas per kamar
//  Cocok untuk: nama tipe, luas, bed type, view, fasilitas kamar, harga mulai
// =============================================================================

func seedKamar() {
    data := []struct{ judul, konten, kataKunci string }{
       {
            judul: "Deluxe Room",
            konten: `Deskripsi Lengkap:
Kamar deluxe seluas 32 m² dengan lokasi tepi pantai di Biluhu, Gorontalo. Kamar elegan ini menawarkan semua kenyamanan untuk pelancong dengan suasana yang nyaman, balkon pribadi, dan pemandangan laut.

Room Information:
- Kapasitas: Hingga 3 Orang Dewasa
- Ukuran Kamar: 32 m²
- Waktu Check-in: Mulai pukul 14.00 WIB (2:00 PM)
- Waktu Check-out: Sebelum pukul 12.00 WIB (12:00 PM)
- Minimum Stay: 1 hari

Fasilitas Kamar & Hotel:
- Tempat tidur Hollywood Style (Queen Bed)
- Balkon pribadi dengan pemandangan laut
- Kamar mandi dengan shower dan air panas
- AC (Air Conditioning)
- Akses internet WiFi Gratis (Free WiFi)
- Televisi layar datar (Flat-screen TV)
- Perlengkapan mandi (sabun, sampo, handuk)
- Area tepi pantai (seaside location)
- Parkir tersedia`,
            kataKunci: "deluxe,kamar,room,seaside,pantai,hollywood,bed,balkon,pemandangan,laut,AC,wifi",
        },
        {
            judul: "Family Room",
            konten: `Deskripsi Lengkap:
Kamar keluarga seluas 52 m² dengan lokasi tepi pantai di Biluhu, Gorontalo. Dirancang untuk keluarga atau kelompok kecil yang membutuhkan ruang lebih luas dengan kenyamanan maksimal.

Room Information:
- Kapasitas: Hingga 4-5 Orang Dewasa
- Ukuran Kamar: 52 m²
- Waktu Check-in: Mulai pukul 14.00 WIB (2:00 PM)
- Waktu Check-out: Sebelum pukul 12.00 WIB (12:00 PM)
- Minimum Stay: 1 hari

Fasilitas Kamar & Hotel:
- Tempat tidur Hollywood Style (Queen Bed + Extra Bed)
- Balkon pribadi dengan pemandangan laut
- Kamar mandi dengan shower dan air panas
- AC (Air Conditioning)
- Akses internet WiFi Gratis (Free WiFi)
- Televisi layar datar (Flat-screen TV)
- Perlengkapan mandi (sabun, sampo, handuk)
- Area tepi pantai (seaside location)
- Parkir tersedia`,
            kataKunci: "family,kamar,room,keluarga,seaside,pantai,hollywood,bed,balkon,pemandangan,laut,AC,wifi,luas",
        },
		// Tambahkan tipe kamar lainnya di sini...
	}

	for _, d := range data {
		if err := TambahKnowledge("kb_kamar", d.judul, d.konten, d.kataKunci); err != nil {
			fmt.Printf("[SEED][ERROR] kb_kamar '%s': %v\n", d.judul, err)
		}
	}
}



// =============================================================================
//  kb_fasilitas — Fasilitas umum hotel
//  Cocok untuk: restoran, kolam renang, gym, spa, shuttle, laundry, dll.
// =============================================================================

func seedFasilitas() {
    data := []struct{ judul, konten, kataKunci string }{
          {
            judul: "Lokasi & Kontak",
            konten: `Aryan by Kadena beralamat di Jalan Hj. Rusli Habibie, Biluhu Tim., Kec. Batudaa Pantai, Kabupaten Gorontalo, Gorontalo, 96221, Indonesia. Resort ini terletak tepat di tepi Blue Ring East Biluhu Beach, Provinsi Gorontalo. Untuk informasi lebih lanjut, hubungi +62 852-1111-5115 atau email info@aryanbykadena.com. Check-in mulai pukul 14:00 WIB dan check-out sebelum pukul 12:00 WIB.`,
            kataKunci: "lokasi,kontak,alamat,Gorontalo,telepon,email,check-in,check-out,Biluhu,Batudaa Pantai",
        },

		// Tambahkan entri kb_umum lainnya di sini...
	}

	for _, d := range data {
		if err := TambahKnowledge("kb_umum", d.judul, d.konten, d.kataKunci); err != nil {
			fmt.Printf("[SEED][ERROR] kb_umum '%s': %v\n", d.judul, err)
		}
	}
}


