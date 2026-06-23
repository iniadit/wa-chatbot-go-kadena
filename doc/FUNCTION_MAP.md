# FUNCTION_MAP.md

Project: Hotel Chatbot (Golang)

Version : 2.0
Status : ACTIVE

---

# Tujuan

Dokumen ini menjadi referensi utama lokasi business logic pada project.

Setiap fitur memiliki "rumah" masing-masing sehingga developer tidak perlu mencari ke seluruh project.

Dokumen ini WAJIB diperbarui apabila:

- Menambah file baru
- Memindahkan fungsi ke file lain
- Menghapus fitur besar
- Melakukan modularisasi

---

# Filosofi Project

Project ini menggunakan prinsip:

> **One File = One Business Domain**

Artinya:

Satu file boleh memiliki puluhan bahkan ratusan fungsi,
selama seluruh fungsi tersebut masih berada pada domain business yang sama.

Contoh:

knowledge.go

boleh berisi:

- Query SQLite
- Ranking
- Keyword Mapping
- Website Cache
- RAG Retrieval

karena seluruhnya masih termasuk domain Knowledge Base.

Project ini **TIDAK** menggunakan:

- Clean Architecture
- Hexagonal Architecture
- Repository Pattern
- Layered Package
- Service Package

Seluruh source code tetap menggunakan:

package main

hingga benar-benar diperlukan perubahan arsitektur yang lebih besar.

---

# Struktur Project

```

WA-CHATBOT-GO/

doc/

main.go
config.go
knowledge.go
learning.go
seed_knowledge.go

(Phase 5)

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

# File Ownership

---

# main.go

Status:

ACTIVE

Tanggung Jawab:

- main()
- Startup Application
- Load Config
- Init Database
- Init WhatsApp
- Register Event
- Graceful Shutdown

Boleh Berisi:

- initialization
- startup
- dependency wiring

Tidak Boleh Berisi:

- AI Logic
- Query SQLite
- Knowledge Retrieval
- Prompt Builder
- Ownership Logic
- Session Memory

Target setelah Phase 5:

main.go hanya menjadi entry point aplikasi.

---

# config.go

Status:

ACTIVE

Tanggung Jawab:

- Environment Variable
- Default Config
- API Key
- AI Model
- Hotel Contact
- URL Hotel
- Timeout
- Feature Flag

Tidak Boleh Berisi:

- Business Logic
- AI Request
- Database Query

---

# knowledge.go

Status:

ACTIVE

Domain:

Knowledge Base (RAG)

Tanggung Jawab:

- cariDiKnowledgeBase()
- SQLite Query
- Keyword Routing
- kategoriKeywords
- Ranking Knowledge
- Website Cache
- RAG Retrieval
- Hotel Information
- FAQ

Tidak Boleh Berisi:

- HTTP AI Request
- WhatsApp Send
- Ownership Logic

---

# learning.go

Status:

ACTIVE

Domain:

Auto Learning

Tanggung Jawab:

- Unknown Question
- Learning Queue
- Training Data
- Save Learning
- Review Learning

Tidak Boleh Berisi:

- AI Request
- WhatsApp Logic

---

# seed_knowledge.go

Status:

ACTIVE

Domain:

Knowledge Seeder

Tanggung Jawab:

- Initial Knowledge
- Seed SQLite
- Update Default Knowledge

Tidak Boleh Berisi:

- AI
- WhatsApp

---

# message_handler.go

Status:

Phase 5A

Domain:

Message Routing

Tanggung Jawab:

- Incoming Message
- Routing Pesan
- Template Matching
- AI Routing
- Knowledge Routing
- Ownership Check
- Send Response Flow

Flow:

User

↓

Template

↓

Ownership

↓

Knowledge

↓

AI

↓

WhatsApp

↓

User

Tidak Boleh Berisi:

- HTTP AI
- SQLite Query Detail

---

# whatsapp.go

Status:

Phase 5A

Domain:

WhatsApp

Tanggung Jawab:

- Send Message
- Receive Message
- Typing Indicator
- Read Receipt
- WhatsApp Helper

Tidak Boleh Berisi:

- AI Prompt
- Knowledge Retrieval

---

# ai.go

Status:

Phase 5B

Domain:

Artificial Intelligence

Tanggung Jawab:

- callKoboLLM()
- callGemini()
- AI Routing
- Multi Model Routing
- Payload Builder
- Response Parsing
- AI Response

Tidak Boleh Berisi:

- WhatsApp API
- SQLite Query

---

# prompt.go

Status:

Phase 5B

Domain:

Prompt Engineering

Tanggung Jawab:

- Build System Prompt
- Build User Prompt
- Build RAG Prompt
- Prompt Helper

Tidak Boleh Berisi:

- HTTP Request

---

# retry.go

Status:

Phase 5B

Domain:

Retry Mechanism

Tanggung Jawab:

- Retry AI
- Exponential Backoff
- Retry Helper

Tidak Digunakan Untuk:

- Business Logic

---

# validation.go

Status:

Phase 5B

Domain:

Validation

Tanggung Jawab:

- AI Validation
- Prompt Validation
- Input Validation
- Output Validation

---

# ownership.go

Status:

Phase 5C

Domain:

Ownership State Machine

Tanggung Jawab:

- cekOwner()
- setOwner()
- Human Handover
- Owner Timeout
- Owner Cleanup

Tidak Boleh Berisi:

- AI Prompt

---

# memory.go

Status:

Phase 5C

Domain:

Session Memory

Tanggung Jawab:

- Conversation Memory
- Chat History
- Last Activity
- TTL
- Cleanup

---

# scheduler.go

Status:

Phase 5C

Domain:

Background Task

Tanggung Jawab:

- Cleanup Worker
- Scheduler
- Periodic Task

---

# database.go

Status:

Phase 5D

Domain:

Database

Tanggung Jawab:

- Open Database
- Close Database
- Migration
- SQLite Helper

Tidak Boleh Berisi:

- Business Logic

---

# logger.go

Status:

Phase 5D

Domain:

Logging

Tanggung Jawab:

- Info
- Warning
- Error
- Debug
- Audit

---

# utils.go

Status:

Phase 5D

Domain:

General Utility

Tanggung Jawab:

- String Helper
- Time Helper
- Random Helper
- Format Helper

Tidak Boleh Berisi:

Business Logic

---

# Database Ownership

hotel_knowledge.db

Domain:

Knowledge Base

Digunakan Oleh:

- knowledge.go
- seed_knowledge.go

---

guest_profiles.db

Domain:

Guest Profile

Digunakan Oleh:

- learning.go
- message_handler.go

---

whatsapp_session.db

Domain:

WhatsApp Session

Digunakan Oleh:

- whatsapp.go

---

# Dependency Flow

main.go

↓

message_handler.go

↓

knowledge.go

↓

ai.go

↓

whatsapp.go

Dependency harus mengalir satu arah.

Hindari dependency melingkar (Circular Dependency).

---

# Modularization Rules

Rule 1

Pindahkan fungsi berdasarkan domain business,
BUKAN berdasarkan jumlah baris.

---

Rule 2

Satu file boleh memiliki banyak fungsi.

Tidak perlu memecah file hanya karena ukurannya besar.

---

Rule 3

Jangan membuat file baru jika domain business masih sama.

Lebih baik:

knowledge.go

daripada:

knowledge_query.go

knowledge_cache.go

knowledge_helper.go

---

Rule 4

Jika ragu fungsi masuk ke file mana,

pilih berdasarkan:

"Fungsi ini termasuk domain business apa?"

BUKAN

"Fungsi ini dipanggil dari mana?"

---

Rule 5

Prioritaskan maintainability dibanding jumlah file.

---

Rule 6

Setiap file baru WAJIB memiliki satu domain business yang jelas.

---

# Checklist Sebelum Menambah File Baru

Sebelum membuat file baru, tanyakan:

□ Apakah domain business berbeda?

□ Apakah file lama sudah terlalu kompleks?

□ Apakah file baru benar-benar diperlukan?

Jika jawabannya TIDAK,

maka tambahkan fungsi ke file yang sudah ada.

---

# Catatan

Dokumen ini menjadi referensi utama selama Phase 5 dan fase-fase berikutnya.

Apabila terdapat perbedaan antara implementasi source code dan dokumen ini,
maka source code dianggap benar dan dokumen harus segera diperbarui agar tetap sinkron.
