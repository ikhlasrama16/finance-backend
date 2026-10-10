# Audit: fitur AI report

Tanggal audit: 6 September 2026
Ruang lingkup: `internal/report`, endpoint `POST /api/v1/reports/ai`, konfigurasi server, dan skema cache. Tidak ada kode aplikasi atau migrasi yang diubah oleh audit ini.

## Ringkasan eksekutif

Fitur saat ini menghasilkan statistik dan narasi AI dalam **satu HTTP request sinkron**. Ini adalah penyebab utama pengalaman frontend yang lama dan rentan timeout: request harus menunggu dua pembacaan transaksi, agregasi di aplikasi, lookup cache, lalu panggilan OpenRouter yang diberi timeout 45 detik. Server sendiri berhenti menulis response setelah 60 detik.

Fitur juga memaksakan empat tipe periode (`daily`, `weekly`, `monthly`, `custom`). `custom` memang memungkinkan rentang tanggal bebas, tetapi kontrak perbandingannya selalu “rentang dengan panjang sama tepat sebelum rentang saat ini”. Pengguna tidak dapat menyatakan dengan eksplisit periode pembanding yang diinginkan.

Prioritas refactor: pisahkan statistik dari pembuatan narasi, jadikan laporan sebagai job persisten, dan ubah kontrak menjadi rentang tanggal + konfigurasi pembanding. Dengan demikian frontend mendapat respons cepat (`cached`, `queued`, atau `running`), sedangkan AI diproses di background dan dapat dipoll.

## Kebutuhan produk yang dikonfirmasi

- Pengguna dapat memilih rentang tanggal apa pun, termasuk kemarin, tanggal lampau tertentu, atau beberapa hari/bulan; tidak dibatasi “hari ini”, “minggu ini”, dan “bulan ini”.
- Laporan untuk hari berjalan bersifat **live terhadap data**. Contoh: laporan dibuka pagi setelah membeli mi ayam, lalu dibuka kembali di akhir hari setelah ada transaksi lain. Statistik dan insight akhir hari harus mencerminkan transaksi tambahan itu.
- Hasil lama tetap berguna sebagai snapshot historis, tetapi UI harus jelas bahwa itu dibuat dari versi data tertentu dan menawarkan “perbarui analisis” ketika data transaksi telah berubah.
- Pengguna tetap mendapat angka/statistik segera; narasi AI tidak boleh membuat halaman report terasa macet.

Implikasinya: “daily” bukan identitas cache. Identitas laporan harus mencakup rentang eksplisit dan hash snapshot statistik. Rentang yang sama dengan data berbeda berarti analisis versi baru.

## Perilaku yang ada saat ini

1. Frontend memanggil `POST /api/v1/reports/ai` dengan `period` serta opsional `start_date` dan `end_date`.
2. Backend membentuk periode kalender Asia/Jakarta dan periode pembanding implisit.
3. Backend menjalankan `LoadTransactions` dua kali, satu untuk periode saat ini dan satu untuk pembanding.
4. Semua baris transaksi pada kedua periode dikirim ke Go, kemudian kategori dan merchant diagregasi dalam memori.
5. Backend menghitung hash statistik dan mencari hasil narasi di tabel `ai_reports`.
6. Saat cache miss, backend memanggil OpenRouter dalam request yang sama, menyimpan hasilnya bila sukses, lalu baru membalas ke frontend.

Statistik tetap dikembalikan bila AI gagal, tetapi pengguna tetap menunggu hingga panggilan AI gagal atau timeout sebelum menerima statistik itu.

## Temuan

| Prioritas | Temuan | Dampak |
| --- | --- | --- |
| P0 | Panggilan LLM sinkron berada pada critical path HTTP. | Loading UI bisa sampai 45–60 detik dan request dapat timeout. |
| P0 | Cache diperiksa **setelah** dua query dan seluruh agregasi. | Bahkan cache hit tidak cepat; aplikasi masih membaca dan memproses data lengkap. |
| P1 | Query report mengambil setiap transaksi lalu agregasi dilakukan di Go. | Latensi, memori, dan ukuran transfer bertumbuh linear terhadap jumlah transaksi/rentang tanggal. |
| P1 | Periode dan pembanding merupakan aturan implisit, bukan pilihan API yang eksplisit. | UI sulit menyediakan analisis fleksibel dan label pembanding dapat ambigu. |
| P1 | Tidak ada job state, retry, atau penyimpanan error untuk generasi AI. | Tidak ada cara aman untuk polling, melanjutkan setelah restart, atau membedakan outage sementara dari laporan yang belum dibuat. |
| P2 | Model default `openrouter/free` tidak dipin ke model/version tertentu. | Latensi dan gaya/hasil narasi dapat berubah tanpa perubahan aplikasi. |
| P2 | Tidak ada metrik/tracing khusus report. | Sulit membuktikan sumber timeout (DB, OpenRouter, antrean, atau client/proxy). |

### P0 — timeout adalah konsekuensi desain saat ini

`OpenRouterClient` menggunakan timeout HTTP 45 detik. `http.Server` memiliki `WriteTimeout` 60 detik. Karena generator dipanggil langsung dari handler melalui `r.Context()`, waktu menunggu OpenRouter menjadi bagian dari durasi request pengguna. Jika browser, reverse proxy, atau server memutus request lebih cepat, context ikut dibatalkan; hasil yang sedang dibuat juga tidak dapat dipakai untuk cache.

Error generator sengaja disembunyikan dan response statistik diberi `ai.status: "unavailable"`. Ini baik untuk graceful degradation, tetapi tidak mengatasi waktu tunggu sebelum fallback tersebut dikembalikan.

### P0 — cache belum menghilangkan pekerjaan mahal

Kunci cache memakai tipe/rentang periode, `summary_hash`, dan model. Hash baru diketahui setelah backend memuat dua set transaksi dan menghitung semua statistik. Akibatnya cache hanya menghemat LLM call, bukan pembacaan dan agregasi data. Dua request cache miss yang bersamaan juga sama-sama dapat memanggil LLM; `ON CONFLICT DO NOTHING` hanya mencegah penyimpanan duplikat, bukan pekerjaan duplikat.

### P1 — pola query dapat menjadi mahal seiring data bertambah

Query sudah menggunakan batas waktu `occurred_at` dan skema memiliki indeks `idx_transactions_occurred_at`; itu fondasi yang benar. Namun query memilih setiap baris transaksi beserta merchant/kategori, lalu `Calculate` mengagregasikannya di Go. Untuk laporan, PostgreSQL lebih tepat menghitung total, kategori, dan merchant melalui agregasi SQL. Query saat ini juga dijalankan serial dua kali.

Perlu lakukan `EXPLAIN (ANALYZE, BUFFERS)` pada database produksi dengan rentang yang representatif sebelum memilih detail indeks tambahan. Audit kode saja tidak cukup untuk mengklaim indeks yang paling tepat.

### P1 — semantik periode membatasi produk

Saat ini:

- `daily`: hari ini dibanding kemarin.
- `weekly`: Senin sampai hari ini dibanding Senin–hari yang sama minggu lalu.
- `monthly`: tanggal 1 sampai hari ini dibanding month-to-date bulan lalu.
- `custom`: dibanding window sebelumnya dengan jumlah hari yang sama.

Aturan tersebut masuk akal sebagai preset, tetapi bukan format laporan generik. Tidak ada pilihan seperti 1–31 Agustus dibanding 1–31 Juli, dibanding periode tanggal khusus, tanpa pembanding, atau dibanding periode kalender penuh. Response juga hanya memuat total belanja periode sebelumnya—bukan metrik perbandingan kategori, income, net cashflow, maupun rentang pembanding yang eksplisit.

### P1 — tabel cache bukan antrean pekerjaan

`ai_reports` hanya mengizinkan status `complete`. Tidak ada payload statistik/snapshot, owner job, waktu mulai/selesai, percobaan, `next_attempt_at`, error aman untuk pengguna, atau mekanisme claim pekerjaan atomik. Karena itu tabel ini tidak bisa menjamin deduplikasi job, retry, dan pemulihan saat proses API restart.

## Target desain yang direkomendasikan

### Kontrak laporan

Jadikan rentang saat ini sebagai input utama dan preset hanya sebagai helper frontend. Contoh konseptual:

```json
{
  "range": { "start_date": "2026-08-01", "end_date": "2026-08-31" },
  "comparison": {
    "mode": "previous_equivalent",
    "range": null
  },
  "include_ai": true
}
```

Mode pembanding yang disarankan:

- `none` — hanya analisis periode utama.
- `previous_equivalent` — window langsung sebelumnya, panjang hari sama.
- `previous_calendar_month` — untuk rentang yang merepresentasikan bulan penuh.
- `previous_calendar_week` — untuk minggu Senin–Minggu.
- `custom` — wajib mengirim `comparison.range.start_date` dan `end_date`.

Pertahankan `daily`, `weekly`, dan `monthly` sebagai preset di frontend atau endpoint adaptor sementara; backend harus menyimpan rentang hasil resolusinya, bukan bergantung pada label preset. Semua response harus mengembalikan rentang utama dan pembanding secara eksplisit.

### Pisahkan statistik dan AI

Pisahkan dua kebutuhan ini:

1. **Statistik deterministik**: endpoint cepat untuk total, kategori, merchant, dan perbandingan. Tidak pernah menunggu LLM.
2. **Narasi AI**: job asinkron berdasarkan snapshot statistik yang sudah dibakukan.

Alur yang disarankan:

```text
Frontend -> POST laporan AI -> 200 cached / 202 queued + report_id
                                  |
                                  v
                         PostgreSQL job queue (durable)
                                  |
                                  v
                         worker -> OpenRouter -> report complete/failed
                                  |
Frontend <- GET report_id (poll) -+
```

Untuk request cache hit, response dapat langsung berisi narasi. Untuk laporan belum ada, frontend langsung menampilkan statistik dan skeleton/status “sedang menyiapkan analisis”; polling endpoint status hingga `complete` atau `failed`. Jangan gunakan request HTTP pengguna sebagai worker.

### Perilaku laporan live dan historis

Backend perlu menerima rentang mana pun sepanjang `start_date <= end_date` dan tidak membatasi `end_date` pada hari ini. Untuk menghindari masa depan yang tidak bermakna, validasi `end_date <= tanggal Jakarta saat request` kecuali produk nantinya memang mendukung proyeksi.

Saat halaman report dibuka, backend menghitung snapshot statistik terbaru untuk rentang tersebut dan mengirim `snapshot_hash`/`data_version` bersamanya. Narasi cache hanya valid bila hash tersebut sama.

Contoh alur hari ini:

1. Pagi, pengguna membeli mi ayam dan membuka `2026-09-06`.
2. Statistik berisi transaksi itu; job narasi A dibuat untuk hash A.
3. Sore terdapat transaksi baru. Saat halaman dibuka/direfresh, statistik berubah menjadi hash B.
4. UI tidak menampilkan narasi A sebagai “analisis terbaru”; UI membuat atau memakai job untuk hash B dan menandai A sebagai snapshot sebelumnya bila ingin ditampilkan.

Untuk kemarin dan rentang lampau, mekanismenya sama. Biasanya hash tidak berubah sehingga cache langsung dipakai. Bila ada transaksi backdate, edit kategori, atau koreksi nominal, hash berubah dan UI dapat menyatakan bahwa data berubah sejak analisis terakhir, lalu membuat analisis baru. Jangan menimpa isi snapshot lama; simpan version history secukupnya agar angka dan narasi yang pernah dilihat tetap dapat diaudit.

Endpoint statistik idealnya menggunakan `GET`, misalnya:

```text
GET /api/v2/reports/statistics?start_date=2026-09-05&end_date=2026-09-05&comparison=previous_equivalent
```

Response memuat rentang utama/pembanding yang sudah teresolusi, statistik, dan `snapshot_hash`. Pembuatan narasi menjadi request terpisah yang membawa konfigurasi terkanonisasi plus `snapshot_hash`; backend menolak atau menghitung ulang bila hash yang dikirim pengguna sudah usang. Ini mencegah narasi dibuat dari angka pagi tetapi ditempelkan sebagai hasil akhir hari.

### Queue persisten di PostgreSQL

Mulai dengan worker in-process yang memakai PostgreSQL; Redis belum diperlukan. Tambahkan tabel terpisah, misalnya `ai_report_jobs`, dengan:

- `id` UUID/ULID publik, `status` (`queued`, `running`, `complete`, `failed`);
- konfigurasi terkanonisasi: rentang utama, rentang pembanding, model, dan versi prompt;
- `snapshot` JSONB serta `input_hash`, agar isi AI selalu sesuai data yang dianalisis;
- `attempts`, `max_attempts`, `next_attempt_at`, `started_at`, `completed_at`;
- error terstruktur yang aman ditampilkan/log, bukan API key atau payload mentah;
- unique key untuk laporan identik yang masih aktif/selesai, sehingga beberapa klik UI hanya membuat satu job.

Worker mengklaim satu job dengan transaksi dan `FOR UPDATE SKIP LOCKED`, lalu memperbarui state secara atomik. Terapkan timeout per-job yang lebih pendek dan terukur (mis. 20–30 detik), exponential backoff untuk error transient/429, dan batas retry. Pada startup, job `running` yang lease-nya kedaluwarsa harus dikembalikan ke antrean atau ditandai gagal secara deterministik.

### Agregasi dan cache snapshot

Pindahkan agregasi ke PostgreSQL: total per tipe, kategori pengeluaran, dan merchant teratas. Batasi merchant di SQL dan kirim hanya data agregat ke Go. Bila tetap perlu beberapa query, jalankan query current dan comparison paralel dengan batas context internal; jangan memuat seluruh transaksi hanya untuk membuat report.

Hash harus berasal dari payload AI terkanonisasi: versi prompt, rentang utama, rentang pembanding, model yang dipin, serta statistik/snapshot. Jangan memasukkan field runtime seperti status, timestamp pembuatan, atau hasil AI ke hash. Snapshot disimpan bersama job agar transaksi yang berubah setelah job diantrekan tidak membuat narasi dan angka yang ditampilkan saling berbeda.

Untuk invalidasi, pilih salah satu secara eksplisit:

- **snapshot immutable (direkomendasikan awal)**: perubahan transaksi menghasilkan report baru saat user meminta ulang; report lama tetap merepresentasikan snapshot-nya.
- **versioned invalidation**: simpan revision/data hash per rentang dan buat report baru jika revision berubah. Ini lebih kompleks, tetapi memungkinkan label “data terbaru”.

## Rencana implementasi bertahap

1. Tambahkan pengukuran lebih dahulu: durasi query current/comparison, jumlah rows, waktu agregasi, cache lookup/hit, waktu antre, waktu LLM, status/error provider, dan total request. Tambahkan request/job ID ke log.
2. Buat endpoint statistik berbasis rentang eksplisit dan SQL aggregation, dengan test batas tanggal Asia/Jakarta, empty period, reconciliation, transfer, serta perbandingan custom.
3. Tambahkan migrasi baru untuk job dan snapshot (jangan mengubah `008_create_ai_reports.sql` yang mungkin sudah diterapkan). Implementasikan endpoint create/status dan worker PostgreSQL satu concurrency terlebih dahulu.
4. Ubah frontend: render statistik segera, submit job AI, polling dengan backoff, tampilkan cached report bila ada, serta tombol retry yang terkontrol saat gagal.
5. Setelah metrik stabil, pin model khusus report, atur concurrency/rate limit, dan hapus atau jadikan adaptor endpoint sinkron lama dengan masa deprecation yang jelas.

## Kriteria selesai dan verifikasi

- P95 endpoint statistik dan create-job tidak bergantung pada latensi provider AI.
- Create job mengembalikan maksimal satu job aktif untuk input yang sama meski dipanggil paralel.
- Frontend dapat membedakan `cached`, `queued`, `running`, `complete`, dan `failed` tanpa menebak dari `null` content.
- Narasi `complete` dapat ditelusuri ke snapshot dan rentang pembanding yang sama dengan angka UI.
- Retry hanya dilakukan untuk error yang transient; error validasi/payload tidak di-retry.
- Tes mencakup deduplikasi, lease recovery setelah restart, timeout provider, 429/backoff, pembatalan request browser, dan perubahan transaksi saat job berjalan.
- Load test memakai volume transaksi nyata dan hasil `EXPLAIN (ANALYZE, BUFFERS)` sebelum menambah/mengubah indeks.

## Referensi kode yang diaudit

- Orkestrasi sinkron dan cache: `internal/report/service.go`.
- Batas periode/pembanding: `internal/report/period.go`.
- Agregasi di memori: `internal/report/aggregate.go`.
- Query seluruh transaksi: `internal/report/repository.go`.
- Timeout OpenRouter: `internal/report/openrouter.go`.
- Batas HTTP server dan route: `internal/server/server.go`.
- Tabel cache saat ini: `migrations/008_create_ai_reports.sql`.
