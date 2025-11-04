        # Transfer Service (combined Transfer + Temperature) - Single Gateway

        This repo runs Transfer and Temperature capabilities in one binary.

Run (Windows):
1. Ensure use lates Go version   go1.25.3
2. Ensure PostgreSQL is running and matches .env ( create database coldstorage before)
3. go mod tidy
4. go install github.com/swaggo/swag/cmd/swag@latest
5. swag init -g cmd/transfer-service/main.go -o docs
6. go run cmd/transfer-service/main.go




# 🧊 Cold Storage WMS — Perencanaan Arsitektur (MVP)

## 1. Konteks & Batasan Layanan

Sistem **Cold Storage WMS (Warehouse Management System)** ini dirancang sebagai _Minimum Viable Product (MVP)_ untuk mengelola aktivitas logistik di fasilitas cold storage, termasuk pengelolaan transfer barang dan pemantauan temperatur.

Tujuan utamanya adalah membangun arsitektur layanan modular yang siap dikembangkan lebih lanjut menuju arsitektur mikroservis penuh.

**Komponen Utama:**
- **Transfer Service** → menangani transaksi perpindahan barang antar lokasi penyimpanan.
- **Temperature Service** → mencatat dan memonitor temperatur ruang penyimpanan untuk menjaga rantai dingin.
- **HTTP Gateway** → satu proses utama yang melayani dua service di satu endpoint.

**Batasan MVP:**
- Belum ada autentikasi user.
- Belum ada load balancing / scaling.
- Penggunaan PostgreSQL lokal.
- Fokus pada arsitektur modular & observabilitas.

---

## 2. Arsitektur Sistem

transfer-service/
├─ cmd/
│ └─ transfer-service/main.go # Entry point aplikasi
├─ internal/
│ ├─ handler/http.go # HTTP routing dengan chi
│ ├─ service/
│ │ ├─ transfer.go # Logika bisnis transfer barang
│ │ └─ temperature.go # Logika bisnis monitoring temperatur
│ └─ repo/postgres_repo.go # Akses data Postgres (auto migration)
├─ outbox_events/ # (Opsional) Event store
├─ logs/
│ └─ app.log # File log dari Zerolog
├─ docs/ # Hasil generate dari Swagger (OpenAPI)
├─ go.mod
├─ go.sum
├─ .env
└─ README.md


**Desain Internal:**
- **Chi Router** → menangani HTTP routing ringan dan cepat.
- **Zerolog** → logging efisien dan terstruktur.
- **Prometheus Metrics** → endpoint `/metrics` untuk observabilitas.
- **Auto Migration** → otomatis membuat tabel jika belum ada.
- **Swagger (OpenAPI)** → dokumentasi API otomatis.

---

## 3. Dependency Utama

| Package | Fungsi |
|----------|--------|
| `github.com/go-chi/chi/v5` | HTTP Router ringan |
| `github.com/rs/zerolog` | Logging terstruktur |
| `github.com/joho/godotenv` | Loader `.env` |
| `github.com/lib/pq` | PostgreSQL driver |
| `github.com/prometheus/client_golang` | Metrics Prometheus |
| `github.com/swaggo/http-swagger` | Swagger UI untuk dokumentasi |
| `github.com/google/uuid` | UUID generator untuk record |

---

## 4. Endpoint Utama

### **Transfer Service**
| Method | Endpoint | Deskripsi |
|--------|-----------|-----------|
| `POST` | `/transfer` | Membuat transfer baru |
| `GET`  | `/transfer` | Mengambil daftar transfer |

### **Temperature Service**
| Method | Endpoint | Deskripsi |
|--------|-----------|-----------|
| `POST` | `/temperature` | Menyimpan pembacaan temperatur |
| `GET`  | `/temperature` | Mendapatkan histori temperatur |

### **Monitoring**
| Endpoint | Deskripsi |
|-----------|------------|
| `/metrics` | Prometheus metrics |
| `/swagger/index.html` | Swagger UI dokumentasi API |

---

## 5. Observabilitas & Logging

**Zerolog** menulis log ke dua tempat:
- Konsol (stdout)
- File log di `logs/app.log`

**Prometheus** tersedia otomatis di port utama aplikasi.
Gunakan:

