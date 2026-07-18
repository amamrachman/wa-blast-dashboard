# WA Blast Engine & Dashboard 🟢

Aplikasi WhatsApp Blast sederhana berbasis *monorepo* yang efisien, responsif, dan aman dari *banned* massal. Sistem ini menggunakan **Go Fiber** & **Whatsmeow** di bagian backend untuk interaksi WhatsApp Web berkinerja tinggi berbasis SQLite (WAL mode), serta **Vite + React + TypeScript + Tailwind CSS** di bagian frontend sebagai *dashboard* operasional.

## 🚀 Fitur Utama
- **Multi-Device Pairing:** Login asinkronus menggunakan pemindaian QR Code langsung dari terminal.
- **Session Persistence:** Sesi tersimpan aman menggunakan database lokal SQLite (Pure Go, tidak perlu CGO/GCC di Windows).
- **Concurreny Safe (WAL Mode):** Database dikonfigurasi menggunakan mode *Write-Ahead Logging* untuk mencegah *database deadlock* saat menerima muatan massal.
- **Bulk Messaging with Protection:** Melakukan *looping* pengiriman massal di latar belakang (*background goroutine*) dengan jeda aman otomatis (5 detik/pesan) untuk meminimalisasi risiko blokir.
- **Teks & Media Support:** Mampu mengirim pesan teks biasa atau lampiran foto dengan *caption* (otomatis dikonversi ke format Base64 di sisi client).

## 🛠️ Struktur Proyek (Monorepo)
```text
.
├── backend/                  # REST API Go Fiber & Engine WhatsApp
│   ├── cmd/api/main.go       # Entrypoint & Router
│   └── pkg/whatsapp/         # Core WhatsApp Wrapper Logic
└── frontend/                 # Dashboard Tampilan UI (Vite + React)
    ├── src/App.tsx           # Form Input Blast
    └── tailwind.config.js    # Konfigurasi Tailwind CSS