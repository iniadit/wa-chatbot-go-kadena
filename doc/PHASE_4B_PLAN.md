Commit 1 — Perbaiki LastActivity + testing.
Commit 2 — Tambahkan LastTouched pada ConversationOwner.
Commit 3 — Implementasikan TTL conversationOwners.
Commit 4 — Integrasikan cleanup worker.
Commit 5 — Tambahkan konfigurasi TTL di config.go.
Commit 6 — Jalankan smoke test, race detector (go test -race jika memungkinkan), dan verifikasi graceful shutdown.