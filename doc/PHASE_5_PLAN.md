# PHASE_5_PLAN.md

Project : Hotel Chatbot (Golang)

Version : 2.0

Status : PLANNING

---

# Latar Belakang

Project Hotel Chatbot telah menyelesaikan seluruh fase stabilisasi utama.

Phase yang telah selesai:

✅ Phase 4A — Retry AI

✅ Phase 4B — Memory TTL Cleanup

✅ Phase 4C — Validation & Production Checklist

Seluruh fitur utama telah berjalan stabil.

Phase 5 bertujuan meningkatkan maintainability project tanpa mengubah perilaku chatbot.

---

# Tujuan Phase 5

Target utama:

- Memperkecil ukuran main.go
- Memisahkan source code berdasarkan Business Domain
- Mempermudah maintenance
- Mempermudah debugging
- Menyiapkan fondasi untuk Phase berikutnya

Phase 5 BUKAN redesign project.

Phase 5 TIDAK menambahkan fitur baru.

---

# Prinsip Phase 5

Seluruh perubahan HARUS mengikuti prinsip berikut.

1.

Business Logic Tidak Berubah.

Input yang sama harus menghasilkan output yang sama.

---

2.

Refactor Bersifat Modular.

Memindahkan source code.

Bukan menulis ulang source code.

---

3.

Stabilitas Lebih Penting daripada Kerapihan.

Jika terdapat dua solusi,

pilih solusi yang paling sedikit mengubah source code.

---

4.

One File = One Business Domain.

Ikuti FUNCTION_MAP.md.

---

5.

Seluruh source code tetap menggunakan:

package main

---

# Yang Tidak Boleh Dilakukan

Selama Phase 5 DILARANG:

- Mengubah flow chatbot.
- Mengubah Prompt.
- Mengubah AI Routing.
- Mengubah Retry Logic.
- Mengubah Ownership Logic.
- Mengubah Memory Logic.
- Mengubah Knowledge Base.
- Mengubah SQLite.
- Mengubah Config.
- Menambah fitur baru.
- Mengubah algoritma.

---

# Roadmap Phase 5

Phase 5 dibagi menjadi empat sub-phase.

Setiap sub-phase WAJIB selesai sepenuhnya sebelum melanjutkan ke sub-phase berikutnya.

---

# Phase 5A

Nama

Message Layer Modularization

Status

⬜ Planned

Tujuan

Mengurangi ukuran main.go dengan memindahkan seluruh proses routing pesan.

Target File

message_handler.go

whatsapp.go

Yang Dipindahkan

message_handler.go

- Incoming Message
- Routing
- Template Matching
- Send Response Flow
- Event Processing

whatsapp.go

- Send Message
- Receive Message
- Typing
- Read Receipt

Yang Tidak Disentuh

- AI
- Retry
- Memory
- Ownership
- Knowledge Base

Target

main.go hanya berisi:

- Startup
- Init
- Register Event
- Graceful Shutdown

Estimasi Risiko

🟢 Rendah

---

Validation

Build

□ gofmt

□ go build

Regression

□ Login WhatsApp

□ Kirim Pesan

□ Breakfast

□ Booking

□ Human Handover

□ Restart Bot

---

Definition of Done

- main.go lebih kecil
- Build PASS
- Manual Test PASS

---

# Phase 5B

Nama

AI Layer Modularization

Status

⬜ Planned

Tujuan

Memisahkan seluruh AI Layer.

Target File

ai.go

prompt.go

retry.go

validation.go

Yang Dipindahkan

ai.go

- callKoboLLM()
- callGemini()
- AI Routing
- Payload Builder
- Response Parsing

prompt.go

- Build Prompt
- System Prompt
- User Prompt
- RAG Prompt

retry.go

- Retry AI
- Backoff
- Retry Helper

validation.go

- AI Validation
- Prompt Validation
- Response Validation

Yang Tidak Disentuh

- Model Selection
- Retry Behaviour
- Payload Format

Estimasi Risiko

🟡 Sedang

---

Validation

□ Build PASS

□ AI Response PASS

□ RAG PASS

□ Retry PASS

□ Long Response PASS

□ Unknown Question PASS

---

Definition of Done

Seluruh AI Layer berada pada file terpisah.

---

# Phase 5C

Nama

Ownership & Memory Modularization

Status

⬜ Planned

Tujuan

Memisahkan Ownership State Machine dan Session Memory.

Target File

ownership.go

memory.go

scheduler.go

Yang Dipindahkan

ownership.go

- cekOwner()
- setOwner()
- Human Handover
- Owner Cleanup
- Owner Timeout

memory.go

- Conversation History
- TTL
- Last Activity
- Memory Cleanup

scheduler.go

- Cleanup Worker
- Background Task
- Scheduler

Yang Tidak Disentuh

- Ownership Behaviour
- Memory TTL
- Cleanup Interval

Estimasi Risiko

🟠 Tinggi

---

Validation

□ Build PASS

□ Restart PASS

□ Long Running PASS

□ Race Detector PASS

□ Memory Leak PASS

□ Human Handover PASS

---

Definition of Done

Ownership dan Memory berhasil dipisahkan tanpa mengubah perilaku chatbot.

---

# Phase 5D

Nama

Infrastructure Modularization

Status

⬜ Planned

Tujuan

Merapikan helper dan infrastruktur project.

Target File

database.go

logger.go

utils.go

Yang Dipindahkan

database.go

- Database Init
- SQLite Helper

logger.go

- Logging

utils.go

- Helper
- Formatter
- Utility

Yang Tidak Disentuh

- Business Logic

Estimasi Risiko

🟢 Rendah

---

Validation

□ Build PASS

□ Database PASS

□ Logging PASS

□ Helper PASS

---

Definition of Done

Seluruh helper berada pada file yang sesuai.

---

# Phase Gate

Tidak boleh melanjutkan ke phase berikutnya apabila:

- Build gagal
- Regression gagal
- Manual Test gagal
- Ditemukan bug baru
- Claude belum menyelesaikan seluruh target phase

Setiap phase harus dinyatakan PASS terlebih dahulu.

---

# Rollback Strategy

Jika setelah modularisasi ditemukan bug:

1.

Hentikan pengembangan phase berikutnya.

↓

2.

Perbaiki bug pada phase aktif.

↓

3.

Lakukan Regression Test.

↓

4.

Baru lanjut ke phase berikutnya.

Tidak boleh membawa bug ke phase selanjutnya.

---

# Regression Checklist

Minimal seluruh test berikut harus dilakukan setelah setiap sub-phase.

□ Halo

□ Breakfast

□ Alamat

□ Booking

□ Transfer

□ Unknown Question

□ Human Handover

□ Restart Bot

□ AI Retry

□ Long Conversation

□ Session Memory

---

# Build Checklist

□ gofmt

□ go build

□ Tidak ada compile error

□ Tidak ada unused import

□ Tidak ada duplicate function

□ Tidak ada dead code

□ Tidak ada circular dependency

---

# Risk Assessment

Phase 5A

🟢 Low Risk

Hanya memindahkan routing pesan.

---

Phase 5B

🟡 Medium Risk

Berhubungan dengan AI.

---

Phase 5C

🟠 High Risk

Berhubungan dengan Ownership dan Memory.

---

Phase 5D

🟢 Low Risk

Berhubungan dengan helper.

---

# Final Target

Setelah seluruh Phase 5 selesai,

project memiliki struktur modular,

namun behavior chatbot tetap IDENTIK.

Target akhir:

- main.go menjadi entry point.
- Business Logic dipisahkan berdasarkan domain.
- Seluruh regression test PASS.
- Seluruh fitur Phase 1–4 tetap berjalan.
- Project siap memasuki Phase 6.

---

# Phase 6 Preview

Setelah Phase 5 selesai,

pengembangan akan difokuskan pada:

- Admin API
- Monitoring
- Dashboard
- Configuration Panel
- Multi Hotel Support

Phase 6 hanya boleh dimulai apabila seluruh sub-phase Phase 5 telah selesai dan dinyatakan PASS.

---

# Catatan

Phase 5 bukanlah perlombaan untuk menghasilkan jumlah file terbanyak.

Keberhasilan Phase 5 diukur dari:

- Stabilitas project
- Kemudahan maintenance
- Kemudahan debugging
- Konsistensi struktur
- Tidak adanya bug baru

Apabila terdapat pilihan antara refactor besar atau refactor bertahap,

selalu pilih refactor bertahap.

Project yang stabil jauh lebih berharga daripada project yang terlihat rapi tetapi menghasilkan regresi.
