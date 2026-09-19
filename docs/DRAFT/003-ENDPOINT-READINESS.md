# 003-ENDPOINT-READINESS.md: Audit Kesiapan Endpoint app-serv

Dokumen kerja hasil audit endpoint pada 2026-09-20. Bukan kontrak; kontrak tetap
`docs/SPEC-API/001-SPEC-API.md`. Temuan yang diterima owner sudah diedit ke spec
di PR yang sama dengan dokumen ini (lihat §6).

| | |
|---|---|
| **Status** | audit selesai; F2 dan F3 diterima owner (2026-09-20) dan diedit ke SPEC-API-001 |
| **Dibuat** | 2026-09-20, dari `app-serv/internal/router/*.go` (main) dibandingkan dengan SPEC-API-001 §7.1 sampai §7.15 |
| **Kaitan** | SPEC-API-001 §2.2 (eksklusi), §7.6 (drift), §7.16 sampai §7.18 (section baru), §10 (fase P4) |

## 1. Metode

Tiga langkah:

1. Ekstraksi route terdaftar dari tiga file registrasi (`router.go`, `router_dataplane.go`,
   `router_media.go`), termasuk verifikasi bahwa tidak ada registrasi route di luar package
   `internal/router` (grep `HandleFunc|mux.Handle` di seluruh `cmd/` dan `internal/`: kosong).
2. Ekstraksi tabel route SPEC-API-001 §7.1 sampai §7.15 sebagai kontrak pembanding.
3. Pemetaan 14 area yang owner daftarkan ke section spec dan ke route yang ada.

Catatan referensi: checkout 9Router ternyata tersedia di `/home/rusmanadodi/apps/9router`
(spec mencatat `/home/rusmanadodi/ai-gateway`, path lama). Ini dipakai untuk membangun
desain F3 dari perilaku reference yang sesungguhnya, dan membuka parity spot-check
SPEC-API-002 §9 yang selama ini tertunda (lihat F5).

## 2. F1: Versioning, PASS

Semua route dibangun dari satu konstanta `APIVersion = "/api/v1"`
(`internal/router/router.go:28`), mengikuti §11.1 (satu prefix versi untuk management
dan data plane). Tidak ada path `/api/...` hardcoded di file registrasi, dan tidak ada
route yang mendaftar di luar package router. Versi `/api/v2` belum ada, sesuai §4:
breaking change baru memunculkannya.

## 3. Pemetaan 14 area owner

| Area owner | Section spec | Route di router | Status |
|---|---|---|---|
| Endpoint & Key | §7.3 + §7.5 | gateway-keys (5), endpoints CRUD + test (7), keys CRUD + bulk (5), endpoints/bulk, oauth/bulk | lengkap |
| Provider | §7.4 | providers (3), provider-nodes CRUD + test (6), OAuth start/callback/status/refresh (4) | lengkap |
| Combo & Vision Adapter | §7.7 + §7.8 | combos CRUD + test (6), vision-adapter GET/PUT | lengkap |
| Usage | §7.12 | summary, timeseries, records, records/{request_id} | lengkap |
| Quota Tracker | §7.12 | quotas, quotas/{endpoint_id} GET/PUT | lengkap |
| Token Saver | §7.9 | token-saver GET/PUT; engine native sudah Closed lewat 002 | lengkap |
| Skill | tidak ada (dulu §2.2) | tidak ada | F3, lihat §5 |
| Media Provider | §7.10 | media-providers (3) + speech, transcriptions, voices, images, videos, search | lengkap |
| Playground Chat | tidak ada (dulu §2.2) | tidak ada | F3, lihat §5 |
| Proxy Pools | §7.11 | proxies CRUD + test (6) | lengkap |
| API Docs | tidak ada (dulu §2.2) | tidak ada | F3, lihat §5 |
| Changelog | tidak ada (dulu §2.2) | tidak ada | F3, lihat §5 |
| Console Log | §7.13 | logs/console GET/DELETE, logs/requests (3) | lengkap |
| Setting | §7.14 | settings GET/PATCH | lengkap |

Data plane §7.15 lengkap juga: `chat/completions`, `messages`, `responses`, `models`,
`messages/count_tokens`, `embeddings` (`router_dataplane.go:33-47`). Semua fase P0
sampai P3 sudah ada route-nya di produksi.

## 4. F2: Drift `GET /models/custom`

Router mendaftarkan `GET /api/v1/models/custom` (`router.go:147`, daftar custom model,
teruji di `router_models_test.go:31`), tetapi tabel §7.6 hanya mencantumkan `POST` dan
`DELETE` untuk path yang sama. Implementasi di depan spec, melanggar aturan spec-first
("setiap deviasi wajib mengedit spec di PR yang sama").

**Keputusan owner (2026-09-20): route dipertahankan**, karena panel memang membutuhkan
daftar custom model yang sudah dibuat. Spec diedit: satu baris `GET /api/v1/models/custom`
masuk tabel §7.6, fase P2.

## 5. F3: Empat area yang dulu dieksklusikan, sekarang dimau

§2.2 mengecualikan "translator playground page, skills, basic-chat page" secara eksplisit,
dan menutup dengan "anything not listed in §2.1 is assumed not wanted; it may be added
later via a new spec only". Owner memutuskan (2026-09-20): empat area ini adalah fitur
yang harus ada dan akan diimplementasikan. Grounding dari reference sebelum menulis spec:

| Area | Bukti di reference | Keputusan desain |
|---|---|---|
| Skill | `src/shared/constants/skills.js`: skill = dokumen `SKILL.md` per kapabilitas di repo GitHub produk, URL raw-nya user tempel ke AI agent; halaman dashboard hanya merender katalog statis | Port bentuk yang sama: `GET /api/v1/skills` (S) melayani katalog embedded (id, name, description, endpoint, entry, raw_url, blob_url), satu entri per kapabilitas yang gateway ini benar-benar layani. Tidak ada DB, tidak ada CRUD, isi `SKILL.md` tetap konten repo owner |
| Playground Chat | `src/app/(dashboard)/dashboard/basic-chat/page.js`, terpisah dari halaman translator | Panel pannelAI adalah app terpisah, jadi playground memanggil route data plane §7.15 dengan gateway key dari §7.3, persis seperti CLI tool. Tanpa endpoint baru; halamannya milik SPEC-UI |
| API Docs | reference menyimpan docs di gitbook eksternal | Gateway self-hosted tidak boleh mengasumsikan internet, jadi kontrak dilayani sendiri: `GET /api/v1/openapi.json` (S), embedded saat build, memuat setiap route terdaftar |
| Changelog | `src/shared/components/ChangelogModal.js:21` fetch `GITHUB_CONFIG.changelogUrl` dari browser | pannelAI melayani riwayatnya sendiri: `GET /api/v1/changelog` (S), `{data: [{version, date, title, notes}]}`, embedded saat build, read-only, tanpa tabel DB |

Spec menerima section baru §7.16 (Skills), §7.17 (API Docs), §7.18 (Changelog), catatan
playground di §7.15, dan fase P4 di §10. Translator playground tetap dieksklusikan.

## 6. F4 dan F5: catatan penutup

- **F4 (bukan masalah):** `POST /embeddings` tercantum di §7.10 dan §7.15, tetapi
  diregistrasi tepat satu kali (`router_dataplane.go:41`). Tidak ada konflik mux.
- **F5 (tugas terpisah):** checkout reference tersedia di
  `/home/rusmanadodi/apps/9router`, jadi parity spot-check yang §9 SPEC-API-002 catat
  sebagai "tertunda sampai checkout tersedia" sekarang bisa dilakukan. Tidak dikerjakan
  di audit ini; spec 002 tidak diedit untuk itu di PR ini.

## 7. Jumlah route

Spec §7.1 sampai §7.15 mendefinisikan 89 route; router mendaftarkan 90. Satu-satunya
selisih adalah F2. Setelah edit spec ini, kedua sisi 90.
