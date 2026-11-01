# Panduan Integrasi OpenClaw Bot ke Finance Backend

Dokumen ini memuat spesifikasi integrasi, referensi command CLI/tool adapter, skema endpoint, serta panduan konfigurasi `openclaw.json` untuk personal bot **OpenClaw** (berjalan via Docker di Tencent Lighthouse VPS).

---

## 1. Arsitektur Integrasi

```
+-------------------------------------------------------+
|             OpenClaw Bot (Docker Container)           |
|  - LLM Orchestrator (Personal Assistant)             |
|  - Tool Runner: executes CLI or HTTP requests         |
+---------------------------+---------------------------+
                            |
                 (1) CLI Tool Call (Subprocess)
                            v
+-------------------------------------------------------+
|            tools/finance_bot_adapter.py               |
|  - Zero pip dependencies (Standard Python 3)          |
|  - Auto name-to-ID mapping (categories, accounts)     |
|  - Date parsing ("today", "yesterday", YYYY-MM-DD)    |
|  - Compact JSON response formatter                    |
+---------------------------+---------------------------+
                            |
                 (2) HTTP REST API (JSON)
                     Auth: Bearer Token / Cookie
                            v
+-------------------------------------------------------+
|              Finance Backend API (Go)                 |
|  Container: finance-api:8080                          |
|  Database:  PostgreSQL 16 (finance-db)                |
+-------------------------------------------------------+
```

---

## 2. Mekanisme Autentikasi

Backend mendukung dua metode autentikasi:

1. **Bearer API Key (Direkomendasikan untuk Bot)**:
   - Header: `Authorization: Bearer <INGEST_API_KEY>` atau `X-API-Key: <INGEST_API_KEY>`
   - Nilai token diambil dari variabel lingkungan `INGEST_API_KEY` di file `.env`.
   - **Kelebihan:** Stateless, tidak perlu login berulang, tidak akan kedaluwarsa.
2. **Session Cookie**:
   - `POST /api/v1/auth/login` menggunakan `{ "email": "...", "password": "..." }`.
   - Menghasilkan cookie `finance_session` yang valid selama 30 hari.
   - Script adapter secara otomatis menangani login dan caching token jika mode ini digunakan.

---

## 3. Tool Adapter CLI (`tools/finance_bot_adapter.py`)

Script adapter ini terletak di [tools/finance_bot_adapter.py](file:///srv/http/finance-backend/tools/finance_bot_adapter.py) dan didampingi wrapper shell [tools/finance_bot_adapter.sh](file:///srv/http/finance-backend/tools/finance_bot_adapter.sh).

### Variabel Lingkungan (Environment Variables)

| Variable | Default | Keterangan |
|---|---|---|
| `FINANCE_API_URL` | `http://localhost:8080` | URL service backend API (gunakan `http://finance-api:8080` di internal network Docker) |
| `FINANCE_API_KEY` | (isi dari `INGEST_API_KEY`) | Bearer token rahasia untuk otentikasi bot |
| `FINANCE_EMAIL` | Opsional | Email login jika menggunakan autentikasi session |
| `FINANCE_PASSWORD` | Opsional | Password login jika menggunakan autentikasi session |

---

### Command & Contoh Pemanggilan

#### A. `add_expense` (Mencatat Pengeluaran)
Mencatat transaksi pengeluaran baru. Nama kategori dan rekening dicocokkan otomatis secara cerdas.

**CLI Call:**
```bash
python3 tools/finance_bot_adapter.py add_expense \
  --amount 35000 \
  --category "Makanan & Minuman" \
  --description "Ayam bakar madu + es teh" \
  --date "today" \
  --account "SeaBank"
```

**Output JSON (Compact untuk LLM):**
```json
{
  "success": true,
  "action": "add_expense",
  "transaction_id": 12,
  "amount": 35000,
  "category": "Makanan & Minuman",
  "account": "SeaBank",
  "description": "Ayam bakar madu + es teh",
  "occurred_at": "2026-11-01T15:30:00+07:00",
  "message": "Pengeluaran Rp 35,000 (Makanan & Minuman) berhasil dicatat ke rekening SeaBank."
}
```

---

#### B. `get_summary` (Rekapitulasi Keuangan)
Mengambil agregasi pengeluaran dan pemasukan untuk rentang waktu tertentu.

**Pilihan `--period`:**
- `today` (hari ini)
- `yesterday` (kemarin)
- `this_week` (Senin minggu berjalan s/d hari ini)
- `last_week` (Senin s/d Minggu pekan lalu)
- `this_month` (tanggal 1 bulan ini s/d hari ini)
- `last_month` (1 bulan penuh sebelumnya)
- `custom` (menggunakan `--start-date YYYY-MM-DD --end-date YYYY-MM-DD`)

**CLI Call:**
```bash
python3 tools/finance_bot_adapter.py get_summary --period this_week
```

**Output JSON (Compact untuk LLM):**
```json
{
  "success": true,
  "action": "get_summary",
  "period": "2026-10-26 s/d 2026-11-01",
  "summary": {
    "total_expense": 245000,
    "total_income": 1500000,
    "net_cashflow": 1255000,
    "expense_transaction_count": 6,
    "average_daily_expense": 35000
  },
  "expense_by_category": [
    {
      "category": "Makanan & Minuman",
      "amount": 180000,
      "percentage": 73.47
    },
    {
      "category": "Transportasi",
      "amount": 65000,
      "percentage": 26.53
    }
  ]
}
```

---

#### C. `update_expense` (Koreksi Transaksi)
Mengoreksi kesalahan input transaksi sebelumnya.
- Jika hanya mengoreksi kategori/deskripsi: Menggunakan `PATCH /api/v1/transactions/{id}`.
- Jika mengoreksi nominal (`amount`): Adapter secara otomatis menghapus catatan lama dan menerbitkan transaksi baru yang bersih sesuai prinsip *ledger integrity*.

**CLI Call:**
```bash
python3 tools/finance_bot_adapter.py update_expense \
  --id 12 \
  --amount 40000 \
  --description "Koreksi: Ayam bakar + es jeruk"
```

**Output JSON:**
```json
{
  "success": true,
  "action": "update_expense",
  "old_transaction_id": 12,
  "new_transaction_id": 13,
  "amount": 40000,
  "category": "Makanan & Minuman",
  "description": "Koreksi: Ayam bakar + es jeruk",
  "message": "Koreksi transaksi #12 berhasil: dibuat transaksi baru #13 dengan nominal Rp 40,000."
}
```

---

#### D. Helper Commands Lainnya

- **Mencatat Pemasukan (`add_income`):**
  ```bash
  python3 tools/finance_bot_adapter.py add_income --amount 500000 --description "Bonus freelance"
  ```
- **Menghapus Transaksi (`delete_expense`):**
  ```bash
  python3 tools/finance_bot_adapter.py delete_expense --id 13
  ```
- **Melihat Daftar Kategori (`list_categories`):**
  ```bash
  python3 tools/finance_bot_adapter.py list_categories
  ```
- **Melihat Daftar Akun/Rekening (`list_accounts`):**
  ```bash
  python3 tools/finance_bot_adapter.py list_accounts
  ```

---

## 4. Konfigurasi OpenClaw (`openclaw.json`)

Di Tencent Lighthouse VPS, konfigurasikan `openclaw.json` pada container OpenClaw Anda agar bot dapat mengenali tool keuangan ini secara native.

### Contoh Definisi Tools pada `openclaw.json`:

```json
{
  "tools": [
    {
      "name": "add_expense",
      "description": "Catat pengeluaran keuangan pribadi baru ke buku kas.",
      "parameters": {
        "type": "object",
        "properties": {
          "amount": {
            "type": "integer",
            "description": "Nominal uang yang dikeluarkan dalam Rupiah (misal: 35000)"
          },
          "category": {
            "type": "string",
            "description": "Kategori pengeluaran. Contoh: 'Makanan & Minuman', 'Transportasi', 'Belanja', 'Tagihan', 'Hiburan', 'Kesehatan'"
          },
          "description": {
            "type": "string",
            "description": "Catatan detail mengenai pengeluaran tersebut"
          },
          "date": {
            "type": "string",
            "description": "Tanggal pengeluaran: 'today', 'yesterday', atau format 'YYYY-MM-DD'. Default 'today'"
          },
          "account": {
            "type": "string",
            "description": "Rekening/dompet asal pembayaran (misal: 'SeaBank', 'ShopeePay', 'Bank Jago', 'Cash'). Opsional."
          }
        },
        "required": ["amount", "category", "description"]
      },
      "command": "/srv/http/finance-backend/tools/finance_bot_adapter.sh add_expense --amount {{amount}} --category \"{{category}}\" --description \"{{description}}\" --date \"{{date|default('today')}}\" {% if account %}--account \"{{account}}\"{% endif %}"
    },
    {
      "name": "get_summary",
      "description": "Ambil rekap pengeluaran dan pemasukan berdasarkan periode (harian, mingguan, bulanan).",
      "parameters": {
        "type": "object",
        "properties": {
          "period": {
            "type": "string",
            "enum": ["today", "yesterday", "this_week", "last_week", "this_month", "last_month"],
            "description": "Periode rekapitulasi waktu"
          },
          "category": {
            "type": "string",
            "description": "Opsional: nama kategori jika hanya ingin mengecek kategori spesifik"
          }
        },
        "required": ["period"]
      },
      "command": "/srv/http/finance-backend/tools/finance_bot_adapter.sh get_summary --period {{period}} {% if category %}--category \"{{category}}\"{% endif %}"
    },
    {
      "name": "update_expense",
      "description": "Koreksi catatan transaksi pengeluaran yang salah input (nominal, kategori, atau deskripsi).",
      "parameters": {
        "type": "object",
        "properties": {
          "transaction_id": {
            "type": "integer",
            "description": "Nomor ID transaksi yang hendak dikoreksi"
          },
          "amount": {
            "type": "integer",
            "description": "Nominal baru jika ada koreksi jumlah uang"
          },
          "category": {
            "type": "string",
            "description": "Kategori baru jika ada koreksi jenis pos pengeluaran"
          },
          "description": {
            "type": "string",
            "description": "Deskripsi atau keterangan baru"
          }
        },
        "required": ["transaction_id"]
      },
      "command": "/srv/http/finance-backend/tools/finance_bot_adapter.sh update_expense --id {{transaction_id}} {% if amount %}--amount {{amount}}{% endif %} {% if category %}--category \"{{category}}\"{% endif %} {% if description %}--description \"{{description}}\"{% endif %}"
    }
  ]
}
```

---

## 5. Setup Deployment di VPS (Docker Network)

Jika OpenClaw berjalan di dalam container Docker pada VPS yang sama dengan `finance-backend`:

1. **Pastikan Container Berada Dalam Satu Network:**
   ```bash
   # Sambungkan container OpenClaw ke network docker finance-backend
   docker network connect finance-backend_default <openclaw_container_name>
   ```

2. **Mount Folder Tools (atau jalankan script via host):**
   Pada konfigurasi `docker run` / `docker-compose.yml` OpenClaw:
   ```yaml
   volumes:
     - /srv/http/finance-backend/tools:/app/tools:ro
   environment:
     - FINANCE_API_URL=http://finance-api:8080
     - FINANCE_API_KEY=test123 # Samakan dengan INGEST_API_KEY di .env backend
   ```

3. **Verifikasi Jalur Komunikasi:**
   Jalankan command test dari dalam container OpenClaw:
   ```bash
   python3 /app/tools/finance_bot_adapter.py get_summary --period today
   ```

Integrasi kini selesai dan siap digunakan!
