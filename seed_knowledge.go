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
    seedPromotionDeals()


	

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
            konten:   `Kadena Glamping Dive Resort adalah resort glamping tepi pantai di Anyer yang menawarkan tempat berkumpul ideal untuk liburan panjang, perjalanan singkat, menyelam, snorkeling, olahraga air, bersantap lokal, dan acara sosial. Setelah hari panjang menjelajahi atraksi lokal, tamu dapat bersantai di kolam outdoor yang dirancang indah atau tetap terhubung dengan Wi-Fi berkecepatan tinggi.`,
            kataKunci: "hotel,resort,profil,tentang,sejarah,glamping,Anyer",
        },
        {
            judul:    "Kamar Tamu",
            konten:   `Resort ini menawarkan 17 Glamping Superior, 14 Glamping Deluxe, dan 1 Glamping Suite dengan tempat tidur nyaman. Semua kamar memiliki balkon pribadi – banyak di antaranya dengan pemandangan Selat Sunda. Kamar elegan ini menyediakan semua kenyamanan untuk semua pelancong, dengan tempat tidur gaya Hollywood yang dapat menampung hingga 3 orang dewasa, serta menyediakan perlengkapan mandi ramah lingkungan.`,
            kataKunci: "kamar,tamu,glamping,superior,deluxe,suite,balkon,pemandangan",
        },
        {
            judul:    "Fasilitas",
            konten:   `Kadena Glamping Dive Resort memiliki 32 glamping, kolam infinity, dive center, olahraga air, resto & bar, playground anak, ruang fungsi, dan hiburan DJ langsung serta band setiap Jumat & Sabtu pukul 16:00-22:00. Resort beachfront ini menawarkan berbagai fasilitas lengkap untuk kenyamanan tamu selama menginap.`,
            kataKunci: "fasilitas,kolam,renang,diver,center,olahraga,air,resto,bar,playground,ruang,fungsi,hiburan,DJ,band",
        },
        {
            judul:    "Dive Center & Olahraga Air",
            konten:   `Resort ini memiliki dive center dan fasilitas olahraga air lengkap untuk tamu yang ingin mengeksplorasi keindahan bawah laut Anyer. Tersedia berbagai aktivitas air yang dapat dinikmati selama menginap di resort.`,
            kataKunci: "dive,center,olahraga,air,menyelam,snorkeling,aktivitas,air,bawah,laut",
        },
        {
            judul:    "Meeting & Event",
            konten:   `Atur acara Anda bersama kami, dengan 2 ruang meeting indoor yang dapat menampung hingga 200 orang. Tersedia fasilitas lengkap untuk berbagai jenis acara, mulai dari pertemuan bisnis hingga acara sosial.`,
            kataKunci: "meeting,event,ruang,rapat,indoor,acara,pertemuan,bisnis,sosial,200,orang",
        },
        {
            judul:    "Dining",
            konten:   `"K Resto & Bar" adalah venue bersantap sepanjang hari yang kontemporer, tempat sempurna untuk memanjakan selera makan atau menikmati koktail dan teh sore dengan pemandangan laut di The Lounge.`,
            kataKunci: "dining,bersantap,resto,bar,K,Resto,venue,makan,koktail,teh,sore,pemandangan,laut,Lounge",
        },
        {
            judul:    "Kolam Renang",
            konten:   `Dengan konsep kolam infinity, tamu dapat menikmati aktivitas selama menginap dan juga menikmati pemandangan matahari terbenam. Kolam renang ini menawarkan pengalaman bersantai yang unik dengan pemandangan laut.`,
            kataKunci: "kolam,renang,infinity,pool,matahari,terbenam,sunset,pemandangan,laut,bersantai",
        },
        {
            judul: "Google Maps",
            konten: `- https://maps.app.goo.gl/hsr13pL5uTJKCqVdA.`,
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
            judul: "Glamping Superior",
            konten: `Deskripsi Lengkap:
Kamar glamping seluas 22 m² dengan lokasi tepi pantai. Kamar elegan ini menawarkan semua kenyamanan untuk semua pelancong dengan suasana yang nyaman dan dekat dengan alam.

Room Information:
- Kapasitas: 2 Orang Dewasa
- Ukuran Kamar: 22 m² (22 Ft)
- Waktu Check-in: Mulai pukul 14.00 WIB (2:00 PM)
- Waktu Check-out: Sebelum pukul 12.00 WIB (12:00 PM)
- Minimum Stay: 1 hari

Fasilitas Kamar & Hotel:
- Tempat tidur Hollywood Style
- Kamar mandi dengan perlengkapan mandi ramah lingkungan (eco-friendly toiletries)
- Akses internet WiFi Gratis (Free Wifi)
- Televisi layar datar (Flat-screen TV)
-Area tepi pantai (seaside location)
- Parkir tersedia`,
            kataKunci: "glamping,superior,kamar,room,seaside,pantai,hollywood,bed",
        },
        {
            judul: "Glamping Deluxe",
            konten: `Deskripsi Lengkap:
Kamar glamping seluas 30 m² dengan pemandangan pantai sebagian. Kamar elegan ini menawarkan semua kenyamanan untuk semua pelancong dengan tempat tidur gaya Hollywood yang dapat menampung hingga 3 orang dewasa.

Room Information:
- Kapasitas: 1-3 Orang Dewasa
- Ukuran Kamar: 30 m² (30 Ft)
- Waktu Check-in: Mulai pukul 14.00 WIB (2:00 PM)
- Waktu Check-out: Sebelum pukul 12.00 WIB (12:00 PM)
- Minimum Stay: 1 hari

Fasilitas Kamar & Hotel:
- Tempat tidur Hollywood Style (Twin bed)
- Kamar mandi dengan perlengkapan mandi ramah lingkungan (eco-friendly toiletries)
- Televisi layar datar (Flat-screen TV)
- Pembuat kopi/teh (Coffee/Tea Maker)
- Akses internet WiFi Gratis (Free Wifi)
- Pemandangan pantai sebagian (some beach views)
- Parkir tersedia`,
            kataKunci: "glamping,deluxe,kamar,room,beach,view,pantai,hollywood,twin,bed",
        },
        {
            judul: "Glamping Suite",
            konten: `Deskripsi Lengkap:
Kamar glamping suite seluas 35 m² dengan pemandangan pantai. Ruang ekstra yang dirancang khusus untuk menampung hingga 4 orang dengan nyaman. Tempat tidur gaya Hollywood di setiap glamping dengan perlengkapan mandi ramah lingkungan.

Room Information:
- Kapasitas: 2-4 Orang Dewasa
- Ukuran Kamar: 35 m² (36 Ft)
- Waktu Check-in: Mulai pukul 14.00 WIB (2:00 PM)
- Waktu Check-out: Sebelum pukul 12.00 WIB (12:00 PM)
- Minimum Stay: 1 hari

Fasilitas Kamar & Hotel:
- Tempat tidur Hollywood Style (Twin bed)
- Kamar mandi dengan perlengkapan mandi ramah lingkungan (eco-friendly toiletries)
- Televisi layar datar (Flat-screen TV)
- Akses internet WiFi Gratis (Free Wifi)
- Pemandangan pantai (beach views)
- Parkir tersedia
- Ruang ekstra untuk kenyamanan maksimal`,
            kataKunci: "glamping,suite,kamar,room,beach,view,pantai,hollywood,twin,bed,ekstra,space",
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
            judul: "Dive Center & Water Sport",
            konten: `Deskripsi Lengkap:
Pusat penyelaman dan olahraga air lengkap untuk tamu yang ingin mengeksplorasi keindahan bawah laut Anyer. Tersedia berbagai aktivitas air yang dapat dinikmati selama menginap di resort.

Detail Fasilitas:
- Dive Center dengan peralatan lengkap
- Water Sport activities tersedia
- Akses langsung ke area pantai
- Instruksi dan panduan diving tersedia

Cocok untuk:
Menyelam (diving), snorkeling, aktivitas air, eksplorasi bawah laut, recreational diving, dan olahraga air.`,
            kataKunci: "dive center,water sport,menyelam,snorkeling,aktivitas air,bawah laut,diving,resort,beach,Anyer",
        },
        {
            judul: "Swimming Pool",
            konten: `Deskripsi Lengkap:
Kolam renang dengan konsep infinity pool yang dapat menikmati aktivitas Anda selama menginap dan juga dapat menikmati matahari terbenam saat Anda bersantai di kolam renang. Tersedia untuk kolam dewasa dan anak.

Detail Fasilitas:
- Kolam Infinity dengan pemandangan
- Kolam Dewasa tersedia
- Kolam Anak tersedia
- Area bersantai di tepi kolam
- Pemandangan sunset

Cocok untuk:
Relaksasi, aktivitas keluarga, recreational swimming, menikmati sunset, dan bersantai poolside.`,
            kataKunci: "swimming pool,kolam renang,infinity pool,pool,sunset,matahari terbenam,kolam dewasa,kolam anak,family,leisure",
        },
        {
            judul: "Dining",
            konten: `Deskripsi Lengkap:
"K Resto & Bar" adalah venue bersantap sepanjang hari yang kontemporer, tempat sempurna untuk memanjakan selera makan Anda atau menikmati koktail atau teh sore dengan pemandangan laut di The Lounge.

Detail Fasilitas:
- Restoran all-day dining
- Bar dengan berbagai pilihan minuman
- The Lounge dengan pemandangan laut
- Menu koktail dan afternoon tea
- Pemandangan ocean view

Cocok untuk:
Bersantap keluarga, romantic dining, business lunch, afternoon tea, cocktails, dan menikmati pemandangan laut.`,
            kataKunci: "dining,restaurant,resto,bar,K resto,all-day dining,lounge,ocean view,pemandangan laut,cocktails,afternoon tea",
        },
        {
            judul: "Meeting & Event",
            konten: `Deskripsi Lengkap:
Atur acara Anda bersama kami, 2 ruang meeting indoor kami dapat menampung hingga 200 orang dengan pengaturan fasilitas meeting standar. Kami juga dapat menangani acara sosial outdoor Anda di taman atau di "K Resto & Lounge" terbuka kami dengan pemandangan laut yang luar biasa.

Detail Fasilitas:
- 2 Ruang Meeting Indoor
- Kapasitas hingga 200 orang
- Fasilitas meeting standar tersedia
- Area outdoor untuk acara sosial
- Taman (garden) untuk event
- "K Resto & Lounge" open air dengan sea view

Cocok untuk:
Business meetings, conference, social events, outdoor gatherings, wedding receptions, corporate events, dan acara dengan pemandangan laut.`,
            kataKunci: "meeting,event,conference room,ruang rapat,indoor meeting,outdoor events,social events,garden,sea view,200 persons,business",
        },

		// Tambahkan fasilitas lainnya di sini...
	}

	for _, d := range data {
		if err := TambahKnowledge("kb_fasilitas", d.judul, d.konten, d.kataKunci); err != nil {
			fmt.Printf("[SEED][ERROR] kb_fasilitas '%s': %v\n", d.judul, err)
		}
	}
}

// =========================

// =========================
   func seedPromotionDeals() {
    data := []struct{ judul, konten, kataKunci string }{
        {
            judul:    "Opening Rates",
            konten:   `Deskripsi Lengkap:
Tarif menginap di Kadena Glamping Dive Resort dengan berbagai pilihan tipe glamping. Harga sudah termasuk sarapan untuk 2 orang.

Detail Tarif:
- Glamping Superior (2 orang dewasa):
  * Minggu - Kamis: Rp 1.500.000,- / malam
  * Jumat - Sabtu: Rp 1.700.000,- / malam
- Glamping Deluxe (3 orang dewasa):
  * Minggu - Kamis: Rp 2.000.000,- / malam
  * Jumat - Sabtu: Rp 2.500.000,- / malam
- Glamping Suite (3-4 orang dewasa):
  * Minggu - Kamis: Rp 3.000.000,- / malam
  * Jumat - Sabtu: Rp 3.200.000,- / malam

Informasi Tambahan:
- Check-in: 14.00 WIB
- Check-out: 12.00 WIB
- Harga dapat berubah sesuai ketentuan
- Reservasi disarankan melalui Instagram atau website resmi`,
            kataKunci: "opening rates,tarif,harga,menginap,glamping superior,glamping deluxe,glamping suite,per malam,termasuk sarapan",
        },
        {
            judul:    "F&B Promotion",
            konten:   `Deskripsi Lengkap:
Promosi makanan dan minuman di K Resto & Bar dengan berbagai pilihan menu spesial.

Detail Promosi:
- Menu all-day dining tersedia
- Special cocktails di The Lounge
- Afternoon tea dengan pemandangan laut
- Menu seafood segar
- Local dan Western cuisine

Lokasi:
- K Resto & Bar: restoran utama
- The Lounge: area bersantai dengan ocean view

Waktu Operasional:
- Restoran: 06:00 - 22:00 WIB
- Lounge: 10:00 - 23:00 WIB`,
            kataKunci: "f&b promotion,promosi makanan,minuman,resto,bar,cocktails,afternoon tea,seafood,menu spesial,ocean view",
        },
        {
            judul:    "Chef Recommended",
            konten:   `Deskripsi Lengkap:
Rekomendasi menu spesial dari chef Kadena Glamping Dive Resort dengan bahan-bahan segar dan cita rasa autentik.

Menu Rekomendasi:
- Seafood platter dengan bahan segar lokal
- Western cuisine dengan sentuhan modern
- Local Indonesian dishes autentik
- Special cocktails signature
- Fresh caught fish of the day

Keunggulan:
- Bahan-bahan segar berkualitas
- Resep autentik dan modern
- Presentasi menarik
- Porsi generous

Tersedia di:
- K Resto & Bar
- The Lounge (untuk cocktails & afternoon tea)`,
            kataKunci: "chef recommended,menu spesial,seafood,western,local cuisine,cocktails,fresh catch,signature dishes,resto,bar",
        },
        {
            judul:    "Early Bird Promotion",
            konten:   `Deskripsi Lengkap:
Promosi early bird untuk reservasi lebih awal dengan harga spesial dan benefit tambahan.

Benefit Early Bird:
- Discount khusus untuk booking early
- Free upgrade (tersedia untuk periode tertentu)
- Welcome drink gratis
- Priority check-in
- Special amenity di kamar

Syarat & Ketentuan:
- Booking minimal 7 hari sebelum kedatangan
- Berlaku untuk minimum stay 2 malam
- Subjek ketersediaan kamar
- Tidak dapat digabung dengan promosi lain
- Blackout dates berlaku pada high season

Cara Booking:
- Via website resmi
- Instagram: @kadenaglampingdiveresort
- WhatsApp: 0818-0707-8680
- Email: info@kadenaglampingdiveresort.com`,
            kataKunci: "early bird,promotion,diskon,booking awal,reservasi,free upgrade,welcome drink,priority check-in,special offer",
        },

		// Tambahkan entri kb_umum lainnya di sini...
	}

	for _, d := range data {
		if err := TambahKnowledge("kb_umum", d.judul, d.konten, d.kataKunci); err != nil {
			fmt.Printf("[SEED][ERROR] kb_umum '%s': %v\n", d.judul, err)
		}
	}
}


