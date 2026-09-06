# Rencana implementasi autentikasi

## Keputusan scope

Tahap ini melindungi aplikasi personal dengan satu akun pemilik dan session server-side. Skema `users` dan `user_sessions` sengaja dibuat generik agar login tidak perlu dibangun ulang ketika multi-user dimulai. Data keuangan yang sudah ada tetap single-owner pada tahap ini; migrasi `user_id` penuh adalah fase terpisah.

## Tahap 1 — single-owner yang aman

1. Tambahkan tabel `users` dan `user_sessions`.
2. Sediakan CLI bootstrap untuk membuat akun pertama dengan password yang di-hash. Password tidak disimpan di migration, source code, atau image Docker.
3. Tambahkan endpoint login, logout, dan current session.
4. Gunakan cookie session `HttpOnly`, `Secure` di production, `SameSite=Lax`, dan token acak yang hanya disimpan sebagai hash di database.
5. Wajibkan session untuk semua endpoint browser `/api/*`; health check dan `POST /api/v1/notifications` tetap publik sesuai perannya. Endpoint notifikasi tetap memakai bearer ingest key khusus MacroDroid.
6. Perluas Caddy dari `/api/v1/*` ke `/api/*` agar endpoint AI Report v2 dapat diakses publik melalui proxy setelah login.
7. Tambahkan test password/session/middleware serta dokumentasi deploy dan kontrak frontend.

## Tahap 2 — frontend

Frontend menampilkan login sebelum dashboard, memanggil endpoint session saat bootstrap, dan mengarahkan ke login saat menerima `401`. Karena web dan API berada di origin yang sama, cookie terkirim tanpa menyimpan token di localStorage.

## Tahap 3 — multi-user (jangan dicampur dengan tahap 1)

1. Tambahkan `user_id` pada seluruh data milik pengguna: accounts, categories, transactions, raw notifications, rules, AI report cache, dan AI jobs.
2. Migrasikan seluruh data sekarang ke user pemilik pertama, lalu jadikan `user_id` wajib.
3. Tambahkan filter `user_id` pada setiap repository/query dan sesuaikan unique index agar scoped per user.
4. Tambahkan `ingestion_devices` dengan token panjang yang di-hash, revocable, dan terikat ke user. MacroDroid memakai token perangkat ini, bukan cookie web atau kode pendek.
5. Kode pairing pendek boleh dipakai hanya untuk menghubungkan perangkat, harus expired, one-time, dan tidak boleh menjadi credential API.

## Operasional wajib

- Rotate seluruh credential yang pernah ditulis langsung di Compose/source, lalu pindahkan ke environment secret CI/CD.
- Terapkan migration sebelum API versi baru direstart.
- Jangan membuka pendaftaran publik atau memberi akses ke pengguna lain sebelum Tahap 3 selesai.
