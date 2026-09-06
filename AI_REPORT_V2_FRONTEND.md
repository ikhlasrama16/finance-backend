# Integrasi frontend: AI Report v2

Backend menyediakan laporan berbasis rentang tanggal bebas dan pembuatan insight AI asinkron. Endpoint lama `POST /api/v1/reports/ai` tidak perlu dipakai untuk UI baru.

Base path: `/api/v2`

## 1. Ambil statistik dulu

```http
GET /api/v2/reports/statistics?start_date=2026-09-05&end_date=2026-09-05&comparison=previous_equivalent
```

Parameter:

- `start_date`, `end_date`: wajib, format `YYYY-MM-DD`, boleh tanggal lampau dan tidak boleh masa depan.
- `comparison`: opsional; default `previous_equivalent`.
- Nilai comparison: `none`, `previous_equivalent`, `previous_calendar_week`, `previous_calendar_month`, `custom`.
- Untuk `custom`, kirim `comparison_start_date` dan `comparison_end_date`.

Contoh comparison custom:

```text
/api/v2/reports/statistics?start_date=2026-09-01&end_date=2026-09-15&comparison=custom&comparison_start_date=2026-08-01&comparison_end_date=2026-08-15
```

Response sukses (disederhanakan):

```json
{
  "success": true,
  "data": {
    "range": { "start_date": "2026-09-05", "end_date": "2026-09-05" },
    "comparison_mode": "previous_equivalent",
    "comparison_range": { "start_date": "2026-09-04", "end_date": "2026-09-04" },
    "summary": { "income": 0, "expense": 25000, "net_cashflow": -25000 },
    "expense_by_category": [],
    "top_merchants": [],
    "comparison": {
      "previous_period_expense": 10000,
      "expense_change_amount": 15000,
      "expense_change_percentage": 150
    },
    "snapshot_hash": "..."
  }
}
```

Render angka/statistik segera setelah endpoint ini berhasil. Simpan seluruh konfigurasi range, comparison, dan `snapshot_hash` di state halaman.

## 2. Buat atau ambil job insight AI

```http
POST /api/v2/reports/ai
Content-Type: application/json
```

```json
{
  "start_date": "2026-09-05",
  "end_date": "2026-09-05",
  "comparison": {
    "mode": "previous_equivalent"
  },
  "snapshot_hash": "<snapshot_hash dari endpoint statistik>"
}
```

Untuk comparison custom, bentuk body-nya:

```json
{
  "start_date": "2026-09-01",
  "end_date": "2026-09-15",
  "comparison": {
    "mode": "custom",
    "start_date": "2026-08-01",
    "end_date": "2026-08-15"
  },
  "snapshot_hash": "<snapshot_hash>"
}
```

Response `202 Accepted` berarti job baru/masih diproses. Response `200 OK` berarti hasil identik sudah tersedia di cache. Keduanya mengembalikan object job dengan `id`, `status`, dan bila complete `content`.

## 3. Poll status job

```http
GET /api/v2/reports/ai/{id}
```

State UI:

- `queued`: tampilkan “Menyiapkan analisis…”.
- `running`: tampilkan “Menganalisis transaksi…”.
- `complete`: render Markdown dari `data.content` dengan renderer Markdown yang aman/sanitized.
- `failed`: tampilkan pesan ringan dan tombol **Coba lagi**; ulangi POST dengan range dan `snapshot_hash` yang terbaru.

Poll setiap 1–2 detik, lalu hentikan saat `complete` atau `failed`. Batalkan polling saat pengguna pindah halaman atau mengganti range.

## Data berubah pada range yang sama

Contoh: pengguna membuka report hari ini pagi, lalu ada transaksi sore. Saat halaman direfresh atau range dipilih lagi, selalu panggil endpoint statistik dahulu. Jika `snapshot_hash` berbeda, jangan tampilkan insight lama sebagai hasil terbaru; buat job baru memakai hash baru. Backend akan membalas `409 Conflict` bila frontend mencoba membuat AI report memakai hash yang sudah tidak sesuai dengan data terkini—ambil statistik ulang lalu coba lagi.

## Preset yang disarankan

Preset hanya membantu UI mengisi date picker, bukan dikirim sebagai `period`:

- Hari ini: tanggal Jakarta hari ini sampai hari ini.
- Kemarin: kemarin sampai kemarin.
- Minggu ini: Senin sampai hari ini.
- Bulan ini: tanggal 1 sampai hari ini.
- Custom: dua date picker.

Untuk `previous_calendar_week` gunakan range Senin–Minggu penuh. Untuk `previous_calendar_month` gunakan tanggal 1 sampai hari terakhir bulan penuh; backend akan menolak bentuk range lain untuk kedua mode tersebut. Pada range parsial gunakan `previous_equivalent` atau `custom`.
