# PHASE_5_ARCHITECTURE.md

Project : Hotel Chatbot (Golang)

Version : 2.0

Status : ACTIVE

---

# Tujuan

Dokumen ini menjelaskan arsitektur project setelah Phase 5 selesai.

Phase 5 bukan merupakan redesign project.

Phase 5 hanya melakukan modularisasi source code agar project lebih mudah dirawat, lebih mudah dikembangkan, dan lebih mudah di-debug.

Business Logic TIDAK BOLEH berubah.

---

# Filosofi Project

Project ini menggunakan prinsip utama:

> **One File = One Business Domain**

Artinya:

Setiap file memiliki satu tanggung jawab utama (Single Business Responsibility), tetapi boleh berisi banyak fungsi selama masih berada dalam domain business yang sama.

Contoh:

knowledge.go

boleh berisi:

- SQLite Query
- Ranking
- Keyword Mapping
- Website Cache
- RAG Retrieval

karena semuanya masih termasuk domain Knowledge Base.

---

# Arsitektur yang TIDAK Digunakan

Project ini sengaja TIDAK menggunakan:

- Clean Architecture
- Hexagonal Architecture
- Onion Architecture
- Repository Pattern
- Service Layer
- Controller Layer
- Dependency Injection Framework
- Interface berlebihan
- Package yang tidak diperlukan

Semua source code tetap menggunakan:

package main

hingga benar-benar diperlukan perubahan arsitektur yang lebih besar.

---

# Prioritas Project

Prioritas project adalah:

1.

Stabilitas

↓

2.

Maintainability

↓

3.

Scalability

↓

4.

Refactoring

Jika terjadi konflik antara kerapihan dan stabilitas,

maka stabilitas harus selalu diprioritaskan.

---

# Struktur Project

```
WA-CHATBOT-GO/

doc/
│
├── PROJECT_CONTEXT.md
├── FUNCTION_MAP.md
├── PHASE_4A_COMPLETED.md
├── PHASE_4B_PLAN.md
├── PHASE_4B_CHANGELOG.md
├── PHASE_4C_VALIDATION.md
├── PHASE_5_PLAN.md
└── PHASE_5_ARCHITECTURE.md

.env
.gitignore
go.mod
go.sum
run.bat

main.go

config.go

knowledge.go

learning.go

seed_knowledge.go

message_handler.go

whatsapp.go

ai.go

prompt.go

ownership.go

memory.go

scheduler.go

retry.go

validation.go

database.go

logger.go

utils.go

hotel_knowledge.db

guest_profiles.db

whatsapp_session.db
```

---

# High Level Architecture

```
User

↓

WhatsApp

↓

message_handler.go

↓

Template Matching

↓

Ownership

↓

Knowledge Base

↓

AI

↓

Validation

↓

WhatsApp Send

↓

User
```

Seluruh alur chatbot tetap mengikuti pola di atas.

---

# Dependency Flow

Dependency harus mengalir satu arah.

```
main.go

↓

message_handler.go

↓

knowledge.go

↓

ai.go

↓

validation.go

↓

whatsapp.go
```

Tidak boleh terjadi:

```
knowledge.go

↓

ai.go

↓

knowledge.go
```

atau dependency melingkar lainnya.

---

# File Responsibility

## main.go

Domain:

Application Bootstrap

Berisi:

- main()
- Startup
- Init Config
- Init Database
- Init WhatsApp
- Register Handler
- Graceful Shutdown

Tidak boleh lagi berisi business logic.

---

## config.go

Domain:

Configuration

Berisi:

- Environment
- API Key
- Model AI
- Hotel Contact
- URL
- Timeout
- Feature Flag

---

## knowledge.go

Domain:

Knowledge Base (RAG)

Berisi:

- SQLite Query
- Ranking
- Retrieval
- Website Cache
- FAQ
- Keyword Mapping

Tidak boleh melakukan HTTP Request ke AI.

---

## learning.go

Domain:

Learning

Berisi:

- Unknown Question
- Learning Queue
- Training Data
- Review

---

## seed_knowledge.go

Domain:

Knowledge Seeder

Berisi:

- Initial Seed
- Update Seed
- Seeder Helper

---

## message_handler.go

Domain:

Message Routing

Berisi:

- Incoming Message
- Routing
- Template Matching
- Ownership Check
- AI Call
- Send Response

Merupakan pusat alur chatbot.

---

## whatsapp.go

Domain:

WhatsApp Communication

Berisi:

- Send Message
- Receive Message
- Typing
- Read Receipt

---

## ai.go

Domain:

Artificial Intelligence

Berisi:

- callKoboLLM()
- callGemini()
- AI Routing
- Payload
- Response Parsing
- Multi Model

---

## prompt.go

Domain:

Prompt Engineering

Berisi:

- System Prompt
- User Prompt
- RAG Prompt
- Prompt Builder

---

## retry.go

Domain:

Retry

Berisi:

- Retry AI
- Exponential Backoff
- Retry Helper

---

## validation.go

Domain:

Validation

Berisi:

- AI Validation
- Prompt Validation
- Response Validation
- Input Validation

---

## ownership.go

Domain:

Ownership State Machine

Berisi:

- Owner State
- Human Handover
- Owner Cleanup
- Owner Timeout

---

## memory.go

Domain:

Conversation Memory

Berisi:

- Chat History
- Session Memory
- TTL
- Last Activity

---

## scheduler.go

Domain:

Background Worker

Berisi:

- Cleanup Scheduler
- Periodic Task
- Background Worker

---

## database.go

Domain:

Database

Berisi:

- Open Database
- Close Database
- SQLite Helper

Tidak boleh berisi business logic.

---

## logger.go

Domain:

Logging

Berisi:

- Info
- Warning
- Error
- Debug
- Audit

---

## utils.go

Domain:

General Utility

Berisi:

- Helper
- Formatter
- Time Helper
- String Helper

---

# Database Ownership

hotel_knowledge.db

↓

knowledge.go

seed_knowledge.go

---

guest_profiles.db

↓

learning.go

message_handler.go

---

whatsapp_session.db

↓

whatsapp.go

---

# Layer Interaction

Project menggunakan Domain Interaction.

Bukan Layered Architecture.

Contoh:

message_handler.go

boleh memanggil

knowledge.go

boleh memanggil

ai.go

boleh memanggil

validation.go

boleh memanggil

whatsapp.go

Namun,

knowledge.go

TIDAK boleh memanggil

message_handler.go

---

# Modularization Rules

Rule 1

Satu file boleh memiliki banyak fungsi.

Tidak perlu dipecah hanya karena jumlah baris.

---

Rule 2

Pisahkan berdasarkan domain business.

Bukan berdasarkan jenis function.

---

Rule 3

Jangan membuat package baru.

Gunakan package main.

---

Rule 4

Jangan membuat interface jika hanya memiliki satu implementasi.

---

Rule 5

Jangan membuat helper baru jika helper lama masih relevan.

---

Rule 6

Jangan mengubah business logic saat memindahkan fungsi.

---

Rule 7

Target utama adalah maintainability.

Bukan mengejar jumlah file yang sedikit atau banyak.

---

# Refactoring Principles

Selama Phase 5,

yang diperbolehkan:

✅ Memindahkan fungsi.

✅ Memindahkan struct.

✅ Memindahkan const.

✅ Memindahkan helper.

✅ Mengelompokkan source code.

Yang tidak diperbolehkan:

❌ Mengubah algoritma.

❌ Mengubah flow chatbot.

❌ Mengubah Retry.

❌ Mengubah Memory.

❌ Mengubah Ownership.

❌ Mengubah RAG.

❌ Mengubah AI Routing.

❌ Mengubah Prompt.

---

# Build Requirement

Setelah setiap sub-phase selesai:

- gofmt
- go build

Jika build gagal,

sub-phase dianggap BELUM selesai.

---

# Regression Requirement

Minimal seluruh skenario berikut harus tetap PASS.

- Sapaan
- FAQ Hotel
- Breakfast
- Alamat
- Booking
- Transfer
- Human Handover
- Unknown Question
- Restart Bot

Behavior chatbot harus IDENTIK dengan sebelum modularisasi.

---

# Definition of Done

Phase 5 dianggap selesai apabila:

✅ main.go telah diperkecil secara signifikan.

✅ Business logic dipisahkan sesuai domain.

✅ Tidak ada perubahan perilaku chatbot.

✅ Build PASS.

✅ Regression PASS.

✅ Manual Test PASS.

✅ Seluruh fitur Phase 1–4 tetap berjalan tanpa perubahan.

---

# Catatan

Phase 5 adalah fase modularisasi, bukan fase penambahan fitur.

Keberhasilan Phase 5 diukur dari:

- Kemudahan maintenance.
- Kemudahan debugging.
- Konsistensi struktur project.
- Tidak adanya regresi pada fitur yang sudah stabil.

Apabila terdapat keraguan dalam proses refactor, selalu pilih solusi yang paling aman dan paling sedikit mengubah source code.
