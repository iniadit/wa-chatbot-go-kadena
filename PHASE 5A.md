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