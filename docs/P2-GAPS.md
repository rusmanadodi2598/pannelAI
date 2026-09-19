# P2-GAPS.md — Sisa Gap Fase P2 (app-serv)

Dokumen kerja untuk menutup fase **P2**, bukan kontrak. Kontrak tetap
`docs/SPEC-API/001-SPEC-API.md`; peta topologi tetap `SYSTEM_MAP.md`. Setiap
keputusan yang diambil di sini harus berakhir sebagai changelog SPEC-API (dan
SYSTEM_MAP bila topologinya berubah), bukan hanya sebagai centang di tabel.

| | |
|---|---|
| **Status** | P2: seluruh permukaan rute terpasang; §7.10 terverifikasi live, sisanya "implementasi selesai, verifikasi live outstanding" |
| **Dibuat** | 2026-09-19, dari hasil click-through live §7.10 (PostgreSQL 14 + Redis lokal, stub upstream loopback) |
| **Bukti terakhir** | commit `1f00860` + `573979b`; gate hijau; baris §8 (log mentah `/tmp/pannelai-serv.log` transient — rujukan jangka panjangnya adalah §8, bukan file itu) |

## 1. Cara pakai

- Satu gap = satu ID. ID tidak didaur ulang setelah ditutup.
- Setiap item punya: **Bukti** (keadaan hari ini), **Kenapa** (dampak), **Keputusan**
  (kalau menunggu owner), **Pendekatan**, **Definisi selesai**.
- Item yang menunggu keputusan owner tidak dikerjakan sebagian — jawab dulu,
  baru implementasi (OWASP §2.3: analisis sebelum mitigasi).
- Setiap penutupan item menambah satu baris di §9.

## 2. Register

| ID | Gap | Kelas | Keputusan? | Prioritas |
|---|---|---|---|---|
| G1 | Verifikasi live sisa permukaan P2 (OAuth, combo test, proxies, token-saver, budget caps, katalog model, chat/embeddings) | verifikasi | tidak | 1 |
| G2 | Dial upstream (chat + media + embeddings) tidak melewati `internal/netguard` | keamanan (A01) | **ya** | 2 |
| G3 | `provider_probe.go` mendial `base_url` dari user tanpa guard | keamanan (A01) | menyatu G2 | 2 |
| G4 | `settings.network.outbound_proxy_*` tidak dipakai di jalur dial | kontrak §7.11 | **ya** | 4 |
| G5 | Adapter media per-provider (format gate menolak 16 provider dengan nama) | fitur | tidak | 5 |
| G6 | Panggilan media tidak menulis usage/log; `gateway_keys.request_count` tidak pernah naik | akuntansi §7.12/§7.13 | **ya** | 3 |
| G7 | Kind `video` terdaftar tanpa provider (rute menolak) | cakupan | tidak | 6 |
| G8 | `.env` lokal drift dari `.env.example` → boot polos gagal | lingkungan | tidak | 6 |
| G9 | Kegagalan data plane hanya tercatat sebagai `status`, tanpa kode/alasan | observability §1.6 | tidak | 6 |
| G10 | Status row `SYSTEM_MAP.md` masih menarasikan P1 sebagai fase terakhir | dokumen §1.9 | tidak | 6 |

## 3. Detail per gap

### G1 — Verifikasi live sisa permukaan P2

**Bukti.** 2026-09-19 hanya §7.10 (9 rute) yang digiring terhadap stack hidup.
Belum pernah: OAuth §7.4 (`start`/`callback`/`status`/`refresh`), combo test §7.7,
proxies §7.11 (CRUD + dua rute test), token-saver §7.9 (GET/PUT), budget caps
§7.12 (`PUT /quotas/{endpoint_id}`), katalog model §7.6 (custom/aliases/disabled),
dan data plane chat + embeddings.

**Kenapa.** Aturan spec sendiri: fase tidak boleh disebut CLOSED tanpa click-through
yang tercatat. Unit test tidak membuktikan migrasi, wiring, atau bentuk wire.

**Pendekatan.** Resep yang sama dengan §7.10 (stub loopback + PostgreSQL/Redis nyata):
chat/embeddings lewat **custom provider node** §7.4 yang diarahkan ke stub; OAuth
butuh stub token endpoint + `PUBLIC_BASE_URL`; rute proxy test butuh kandidat yang
diizinkan guard (lihat G2) atau proxy asli. Catat setiap hasil di §9.

**Definisi selesai.** Setiap rute di tabel dijalankan minimal sekali dengan hasil
tercatat (status + bentuk body), plus satu jalur gagal per rute. Gap baru yang
muncul menjadi ID baru di register ini.

### G2 — Egress guard untuk dial upstream

**Bukti.** `dataplane.NewHTTPClient()` — dipakai chat, embeddings, dan media — tidak
melewati `internal/netguard`; guard itu baru dipakai kandidat proxy (§7.11). Header
`internal/domain/media_provider.go` sudah dikoreksi 2026-09-19 agar tidak lagi
mengklaim dua lapis SSRF.

**Kenapa.** Base URL upstream berasal dari operator (override §7.10, endpoint §7.5,
node §7.4). Satu akun panel yang salah konfigurasi bisa membuat gateway mendial
alamat internal; kelas A01 yang sama dengan yang sudah dijaga untuk proxy.

**Keputusan owner.** (a) guard semua dial upstream dengan `EGRESS_ALLOWED_TARGETS`
(self-hosted loopback wajib di-allowlist — itu use case resmi §7.10, jadi
allowlist harus nyaman), atau (b) nyatakan upstream yang dikonfigurasi operator
sebagai trusted dan catat itu eksplisit di SPEC-API §6.

**Pendekatan kalau (a).** Guard harus injectable di level transport: test hermetic
memakai double loopback (`httptest`) yang akan ikut ditolak kalau guard dipasang
global tanpa seam. Ini alasan guard tidak disapu diam-diam saat §7.10.

**Definisi selesai.** Keputusan tercatat di SPEC-API; implementasi + tabel test
(host publik lolos, privat/loopback ditolak, loopback yang di-allowlist lolos).

### G3 — `provider_probe.go` tanpa guard

**Bukti.** `cmd/app-serv/provider_probe.go` mendial `base_url` yang di-supply user
saat probe node kustom (§7.4/§7.5), tanpa guard. Ditemukan saat §7.11, dilaporkan,
belum diubah.

**Kenapa.** Kelas A01 yang sama dengan G2, tapi inputnya lebih langsung (satu
request panel → satu dial ke alamat pilihan penyerang).

**Pendekatan.** Menyatu dengan G2: keputusan yang sama, guard yang sama, allowlist
yang sama. Jangan diselesaikan terpisah supaya tidak ada dua semantik egress.

**Definisi selesai.** Probe memakai guard; tabel test mencakup host privat,
loopback, dan yang di-allowlist.

### G4 — Proxy assignment belum dipakai di jalur dial

**Bukti.** `settings.network.outbound_proxy_*` disimpan, divalidasi, dan dirender
panel, tetapi tidak ada pembaca di jalur request — §7.11 berbunyi "assignment rides
on the settings", dan separuh itu belum benar.

**Keputusan owner.** (a) wire egress lewat proxy terkonfigurasi (binding
per-endpoint tetap deferred, sama seperti reference), atau (b) amend SPEC-API §7.11
agar menyatakan settings belum dihormati di v1.

**Pendekatan kalau (a).** Satu transport yang memilih dialer proxy berdasarkan
settings; bisa dibuktikan dengan proxy stub loopback + allowlist G2.

**Definisi selesai.** Pilihan tercatat; kalau (a), satu panggilan chat terbukti
keluar lewat proxy (log stub).

### G5 — Adapter media per-provider

**Bukti.** 16 provider mendeklarasikan format media non-OpenAI —
`assemblyai, aws-polly, cartesia, coqui, deepgram, edge-tts, elevenlabs, gemini`
(tts+stt), `google-tts, inworld, local-device, minimax` (+`minimax-cn`),
`nvidia-tts, playht, tortoise` — dan semuanya ditolak `PROVIDER_NOT_ROUTABLE`
dengan nama formatnya (terverifikasi live untuk elevenlabs, 2026-09-19).

**Kenapa.** Ini keputusan sadar (menolak dengan nama lebih baik daripada mengirim
bentuk yang salah), tapi selama belum di-port, provider itu tidak bisa dipakai.

**Pendekatan.** Port builder per provider dari reference, satu per satu, urut
berdasarkan kebutuhan operator. Tiap adapter: bentuk payload dari registry +
konversi jawaban ke envelope §7.10.

**Definisi selesai (per adapter).** Format ditranslate, tabel test 3–5 kasus
termasuk kontrol benign, dan satu panggilan live ke stub yang meniru provider itu.

### G6 — Akuntansi panggilan media

**Bukti.** Setelah 9 panggilan media pada 2026-09-19: `usage_records` tidak
bertambah (2 baris lama), `request_logs` tidak bertambah (1 baris lama), dan
`gateway_keys.request_count` tidak pernah di-increment siapa pun — grep tidak
menemukan UPDATE untuk kolom itu, untuk chat maupun media. `MediaCall.Outcome()`
sudah ada tetapi tidak dikonsumsi pemanggil mana pun.

**Kenapa.** Layar Usage (§7.12) dan Logs (§7.13) panel kosong untuk media; kolom
request_count di daftar gateway key selalu 0.

**Keputusan owner.** (a) media menulis usage row (tokens 0; untuk search ada
`cost_per_query` di registry) dan request log; (b) `request_count` di-increment
middleware untuk setiap panggilan data plane terautentikasi; (c) combo test §7.7
tetap tanpa usage row — keputusan itu sudah final dan tidak dibuka lagi di sini.

**Pendekatan.** `Outcome()` sudah membawa provider/endpoint/model; yang kurang
hanya pemanggil di handler/service dan (untuk cost) sumber harga per kind.

**Definisi selesai.** Keputusan tercatat; kalau (a)+(b): satu panggilan media
menghasilkan satu usage row + satu request log, dan request_count key naik 1.

### G7 — Kind `video` tanpa provider

**Bukti.** Rute terdaftar dan menolak dengan nama; tidak ada provider di registry
yang mendeklarasikan `video` (reference juga tidak mendefinisikannya — paritas).

**Pendekatan.** Biarkan terdokumentasi (default), atau cari provider yang
mendeklarasikan kind + tulis adapter (masuk G5).

**Definisi selesai.** Keputusan tercatat di changelog SPEC-API §7.10.

### G8 — `.env` lokal drift dari `.env.example`

**Bukti.** `.env.example` sudah benar dan lengkap; `.env` lokal tidak:
`ENCRYPTION_KEY` 41 byte (harus tepat 32), `REDIS_PASSWORD` basi (`AUTH failed`),
`PANEL_BOOTSTRAP_PASSWORD` kosong. Boot polos gagal di validasi config.

**Pendekatan.** Aksi pemilik: sinkronkan `.env` dengan `.env.example` + secret asli
(read `requirepass` dari `/etc/redis/redis.conf` saat itu juga; jangan disimpan).
Resep override sementara ada di `reference_appserv_local_env.md`.

**Definisi selesai.** `./app-serv` boot bersih tanpa override environment.

### G9 — Logging kegagalan data plane

**Bukti.** Kegagalan media tampil di log sebagai
`request handled … "status":502` + request id, tanpa kode error atau alasan.
Klien menerima envelope yang benar; operator tidak menerima "kenapa".

**Pendekatan.** Log satu baris dengan `code` (dan `error`) di jalur
`writeDataPlaneError`/handler, memakai request id yang sudah ada.

**Definisi selesai.** Tiap kegagalan data plane menghasilkan satu baris log berisi
kode + request id; ada test yang mengunci bentuknya.

### G10 — Narasi P2 di `SYSTEM_MAP.md`

**Bukti.** Status row masih menyebut P1 sebagai fase terakhir yang CLOSED; §3.6 dan
§5 sudah memuat media dan `proxies`/`media_provider_settings`.

**Pendekatan.** Update sekali saat P2 benar-benar ditutup, bersama narasi fase —
jangan dipecah per slice agar tidak churn.

**Definisi selesai.** Status row menyebut P2 CLOSED dengan daftar buktinya.

## 4. Keputusan yang menunggu owner

| # | Pertanyaan | Pilihan | Dampak kalau ditunda |
|---|---|---|---|
| D1 | Guard egress upstream? (G2/G3) | (a) guard + allowlist, (b) trusted + catat | dial internal tetap mungkin dari konfigurasi salah |
| D2 | Proxy settings dihormati di jalur dial? (G4) | (a) wire, (b) amend spec | §7.11 separuh benar |
| D3 | Apa yang dicatat panggilan media? (G6) | (a) usage+log, (b) +request_count, (c) combo test tetap tanpa row | layar Usage/Logs panel kosong untuk media |

## 5. Urutan kerja usulan

1. **P2.1 — G1**: verifikasi live sisa permukaan; tujuannya menemukan gap baru
   sebelum menyatakan apa pun CLOSED.
2. **P2.2 — G2 + G3**: satu keputusan D1, satu implementasi guard.
3. **P2.3 — G6**: keputusan D3 lalu sambungkan `Outcome()` ke recorder.
4. **P2.4 — G4**: keputusan D2 lalu wire atau amend.
5. **P2.5 — G5**: adapter per provider, inkremental, bisa jalan paralel.
6. **P2.6 — G7, G8, G9, G10**: penutup kecil + dokumen.

## 6. Bukan gap (keputusan final — jangan dibuka lagi)

- Dua wire klien: OpenAI-compatible + Anthropic-compatible. Tidak ada bentuk ketiga.
- Format gate menolak provider yang belum di-port **dengan nama formatnya**; itu
  perilaku yang diinginkan, bukan bug (G5 yang mengubahnya, bukan keluhan).
- Shorthand `provider/model/voice` tidak di-port (id model boleh mengandung slash).
- Override §7.10 dibaca per panggilan; registry adalah default, override menang,
  tanpa fallback cloud diam-diam.
- Media tidak punya alias dan combo: rute media menyapa layanan provider langsung.
- Combo test §7.7 tidak menulis usage row (tanpa gateway key, murni diagnostik).

## 7. Definisi selesai & cara verifikasi

- **Kode:** `go test -race -count=1 ./...` hijau; tagged integration hijau;
  `go-lint.sh` dan `go-headers.sh` PASS; commit per item dengan pesan yang menyebut
  buktinya.
- **Live:** boot terhadap PostgreSQL + Redis nyata, rute digiring dengan curl,
  hasil dicatat di §9 (bukan hanya "berhasil").
- **Dokumen:** changelog SPEC-API untuk perubahan kontrak; SYSTEM_MAP untuk
  perubahan topologi; file ini diperbarui statusnya.

## 8. Log verifikasi

| Tanggal | Item | Hasil |
|---|---|---|
| 2026-09-19 | §7.10 media (G1 sebagian) | PASS — 9 rute live, override per-call, health 0→1→0, tanpa usage row; artefak uji dibersihkan |
