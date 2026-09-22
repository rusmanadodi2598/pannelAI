# 010-USAGE-ENDPOINT-READINESS.md: DRAFT PLAN Kesiapan Endpoint Usage (app-serv) dan Node Animation SSE (app-ui)

Dokumen ini adalah **DRAFT PLAN**, bukan kontrak. Kontrak tetap `docs/SPEC-API/001-SPEC-API.md`,
`docs/CONTRACT/001-CONTRACT-API-V1.yaml`, dan `docs/SPEC-UI/001-SPEC-UI.md`. Audit ini melanjutkan
pola `003` sampai `009`: temuan bernomor F, owner memilih nomor, mode AFTER untuk memeriksa yang
sudah ada tanpa mengubah source produksi, mode DURING untuk setiap perbaikan yang dipilih.

Status pengerjaan DURING: **F5 CLOSED 2026-09-22**, lalu **F2 + F9 CLOSED 2026-09-22**, lalu
**F1 + F6 + F7 CLOSED 2026-09-22**, lalu **F3 + F8 CLOSED 2026-09-22** (bukti lengkap di §5
masing-masing temuan). Nomor lain menunggu pemilihan owner.

Lingkup berbeda dari draft sebelumnya: audit ini **menjangkau dua aplikasi sekaligus** atas permintaan
owner. Bagian pertama (`§3` sampai `§9`) mengaudit readiness empat endpoint Usage di `app-serv/.`.
Bagian kedua (`§10`) adalah cross audit yang owner minta eksplisit: **Endpoint Usage (app-serv)
dibandingkan dengan Node Animation SSE (app-ui)**, dengan repo referensi
`https://github.com/decolua/9router` sebagai sumber parity. Setiap temuan menyebut aplikasi mana
yang harus mengerjakannya (`BE` = app-serv, `FE` = app-ui), dan `§11` merangkum gap per aplikasi.

| | |
|---|---|
| **Status** | DRAFT. Semua F1 sampai F14 menunggu pemilihan owner. Tidak ada source produksi yang diubah oleh audit ini |
| **Mechanism** | DURING & AFTER |
| **Tanggal audit** | 2026-09-22 |
| **Scope** | `app-serv/.` (Usage vertical, §7.12) dan `app-ui/.` (layar `/usage`, jalur SSE) |
| **Endpoint utama** | `GET /api/v1/usage/summary`, `GET /api/v1/usage/timeseries`, `GET /api/v1/usage/records`, `GET /api/v1/usage/records/{request_id}` |
| **Layar terkait** | `/usage` (Overview tab, Records tab) |
| **Reference fork** | `https://github.com/decolua/9router` (9Router, `open-sse/`, `src/app/api/usage/*`) |
| **Sumber aturan** | `AGENTS.md`, `SYSTEM_MAP.md`, `docs/RULLES/TDD.md`, `docs/RULLES/OWASP.md`, `docs/SPEC-API/001-SPEC-API.md` §7.12, `docs/SPEC-UI/001-SPEC-UI.md` §6.5/§8.6, `docs/CONTRACT/001-CONTRACT-API-V1.yaml`, `DESIGN.md` |

## 1. DURING & AFTER record

User menetapkan mekanisme `DURING & AFTER`.

- **AFTER (audit ini):** seluruh temuan di bawah dikumpulkan tanpa mengubah source produksi. Probe
  mekanis dijalankan lewat test file sementara yang dihapus kembali; working tree `app-serv` hanya
  membawa perubahan actor lain (opencode/forced-chat), tidak satu pun dari audit ini.
- **DURING (perbaikan berikutnya):** setiap nomor yang dipilih owner dikerjakan mulai dari failing
  table-driven test, mempertahankan header `AGENTS.md` §1.2, validasi typed
  (`go-playground/validator/v10` / Zod v4), pemisahan layer, line limit `<250` dengan warning `220`,
  dan gate penuh sebelum status finding ditutup.

Antislop UI tidak diterapkan pada source Go karena app-serv tidak merender interface; aturan
kejujuran (R-36/R-38, C-5), error terstruktur, dan verify-before-deliver (R-35) tetap diterapkan
pada klaim readiness. Untuk sisi app-ui, aturan antislop R-27 (tiga state), R-26 (kontrol hidup),
R-19 (motion sesuai dial MOTION 1 di `DESIGN.md`), dan R-36 (tidak ada klaim "live" tanpa stream)
adalah bagian dari cross audit `§10`.

## 2. Ringkasan keputusan kesiapan

**Empat endpoint Usage sudah ada, session-gated, dan jalur produksinya terhubung**: route terdaftar
(`internal/router/router.go:185-188`), handler men-decode filter ter-typed
(`internal/handler/usage.go`), service memvalidasi dan mendelegasikan (`internal/service/usage.go`),
repository menjalankan agregasi set-based dengan `date_bin` dan window function
(`internal/repository/postgres/usage_{read,scan}.go`), kontrak YAML dan OpenAPI generated membawa
keempat path, dan panel sudah membangun dua tab `/usage` di atasnya dengan Zod v4.6.5.

Tetapi belum dapat dinyatakan **READY** untuk seluruh mandatory, karena:

1. tidak ada satu pun HTTP handler test untuk keempat route Usage (happy, validation, auth) padahal
   AGENTS.md §2.1 mewajibkannya per route;
2. filter `status` query menerima nilai di luar closed set domain (`banana`, `SUCCESS`) dan
   query mengembalikan nol baris sebagai `200`, bukan `400`;
3. `sseSink.Flush()` **tidak pernah sampai ke klien** karena middleware chain produksi membungkus
   `http.ResponseWriter` dengan recorder yang tidak meneruskan `http.Flusher`, sehingga SSE
   app-serv tidak benar-benar streaming. Ini juga akar defect yang selama ini tercatat di
   `app-ui/README.md` ("the whole answer is written at once");
4. `latency_ms` yang dikontrak dan dikirim adalah **jumlah** latensi per-request, bukan mean,
   sementara SPEC-UI menampilkan p50/p95 dan menyebut `latency_ms` sengaja tidak ditampilkan. Domain
   tak pernah diberi kesempatan menjelaskan dirinya di wire;
5. tidak ada dukungan node live / SSE / polling pada layar Usage, sementara referensi 9router punya
   `/api/usage/stream` SSE plus animasi node provider. SPEC-UI §9.5 justru mengunci kebalikannya
   ("polling instead of a live stream" karena "management API v1 exposes no stream"), jadi parity
   ini adalah keputusan owner, bukan gap yang diam-diam ditutup.

**Keputusan audit:** empat route Usage = `NOT READY (evidence)`. Dukungan node live / SSE untuk
Usage = `TIDAK ADA`, dan status itu saat ini **sesuai spec** (`§9.5`), sehingga pertanyaannya
adalah "apakah owner ingin mengubah spec ke arah parity 9router", bukan "apakah implementasinya
melenceng".

**Status per 2026-09-22:** butir 1, 2, 3, dan 4 di atas sudah ditutup dengan bukti (F1, F2+F9, F5,
F3 berturut-turut), dan F8 (scope `q`, butir 4b) menyusul di batch yang sama; F6 dan F7 juga sudah
tertutup. Butir 5 (node live / SSE) tetap keputusan owner (D5), dan F4 (domain event) tetap
menunggu keputusan D2. Lihat §5 untuk bukti tiap penutupan dan §11 untuk sisa keputusan.

## 3. Jalur endpoint yang diperiksa

### 3.1 Rute dan handler

- `app-serv/internal/router/router.go:185-188` mendaftarkan keempat route Usage di bawah
  `gateway(...)` (session gate). `router_session_sweep_test.go` menyapu seluruh tabel route dan
  memaksa 401 untuk pemanggil anonim, termasuk keempat route ini secara implisit (mereka tidak
  masuk daftar eksklusi).
- `app-serv/internal/handler/usage.go` (171 baris): `Summary`, `Timeseries`, `Records`, `Detail`.
  Decode filter terjadi di schema (`DecodeUsageFilter`), decode pagination di `DecodePage`, join
  captured log di service. Handler tidak berisi SQL atau Redis (AGENTS.md §1.5 terpenuhi).
- `Detail` membaca `r.PathValue("request_id")` dan menolak kosong dengan `VALIDATION_ERROR`.
- Keempat route hanya menerima `GET`; wrong verb dijawab mux 405 yang dibungkus `envelope()`
  menjadi `METHOD_NOT_ALLOWED` JSON.

### 3.2 Schema, domain, dan validasi

- `app-serv/internal/schema/usage.go` (198 baris): `UsageFilterQuery` + `DecodeUsageFilter`
  memvalidasi RFC3339 `from`/`to`, rentang tidak terbalik, closed set `group_by`
  (provider|model|endpoint|gateway_key) dan `granularity` (hour|day), serta batas 200 karakter pada
  enam field teks. Probe mekanis (test sementara, dihapus kembali) membuktikan: `q` 250 karakter
  ditolak, `from=banana` ditolak, rentang terbalik ditolak. **`status` tidak divalidasi**
  (`banana` dan `SUCCESS` diterima; lihat F2).
- `app-serv/internal/schema/usage_record.go` (191 baris): DTO record, list, detail, quota. Nilai
  `cost_usd` string desimal (SPEC-API §4), `limit` pointer, `capture_enabled` selalu hadir.
- `app-serv/internal/domain/usage_types.go`: `UsageGroupBy`/`UsageGranularity` closed set dengan
  `ParseUsageGroupBy`/`ParseUsageGranularity`; `NewUsageFilter` menerapkan default window 24 jam
  sehingga read selalu bounded (AGENTS.md §1.7); `UsageFilter.Validate` menolak rentang terbalik.
- `app-serv/internal/domain/usage_totals.go`: `ErrorRate()` menghitung `big.Rat` dan mengembalikan
  `"0.0000"` untuk requests 0, sehingga wire tidak pernah membagi nol.
- `app-serv/internal/repository/postgres/usage.go`: `usageFilterClause` memakai parameter kosong
  sebagai switch (`$3 = '' OR provider_id = $3`), tidak ada string dari caller yang sampai ke SQL
  (OWASP A05 terpenuhi pada read path). `percentile_disc` menghitung p50/p95 di database, cost
  dijumlah sebagai `numeric` lalu di-cast text.
- Catatan: `DecodeUsageFilter` memvalidasi manual, bukan struct tag `go-playground/validator/v10`.
  Ini bukan pelanggaran §1.4 (query param tidak ber-body; decoder tetap typed dan tervalidasi),
  tapi dicatat karena mandatory menyebut validator dan owner bisa meminta konsistensi.

### 3.3 Service dan repository

- `app-serv/internal/service/usage.go` (154 baris): validasi filter + `group_by`/`granularity`
  ulang di service (defense in depth terhadap caller non-HTTP), pagination bound
  `1 <= per_page <= 100` (`maxUsagePageSize`), `Detail` join log dengan semantik "log hilang bukan
  error; usage hilang = 404" dan melaporkan `CaptureEnabled`/`CaptureBody` dari settings.
- `app-serv/internal/repository/postgres/usage_read.go` (165 baris): `Timeseries` pakai `date_bin`
  dengan origin tetap `2000-01-01` (bucket stabil lintas DST), bucket kosong tidak diisi nol
  (gap di data = gap di chart), `List` pakai `count(*) OVER()` + fallback `count()` saat halaman
  kosong, `MonthlyUsage` untuk budget cap.
- Integration test Postgres nyata ada: `usage_integration_test.go`
  (`SummaryArithmetic`, `GroupByDimensions`) dan `usage_read_integration_test.go`
  (`Timeseries`, `ListAndDetail`, `MonthlyUsage`), tag `integration`.
- `UsageService.Record` memvalidasi lewat `domain.NewUsageRecord` (token negatif / cost tak
  parseable ditolak) sehingga agregat tidak bisa dirusak reporter yang salah.

### 3.4 Kontrak dan OpenAPI

- `docs/CONTRACT/001-CONTRACT-API-V1.yaml` memuat keempat path dengan `sessionCookie` security,
  parameter `from/to/group_by/granularity/status/provider_id/endpoint_id/model/gateway_key_id/q`
  (plus `page/per_page` di records). `from/to` ditulis `type: string` tanpa `format: date-time`
  dan tanpa enum pada `status/group_by/granularity` (lihat F8).
- `app-serv/internal/handler/openapi.json` (generated, jangan diedit tangan) membawa
  `UsageSummaryResponse`, `UsageTimeseriesResponse`, `UsageRecordList`,
  `UsageRecordDetailResponse`, `UsageTotals`, `UsageGroup`, `UsageBucket`,
  `UsageRecordResponse`. Gate `contract-openapi.sh` menjaga artifact tetap segar;
  `TestOpenAPICoversEveryRegisteredRoute` mem-pin route ke dokumen dua arah.

### 3.5 Panel (app-ui) di atas endpoint Usage

- `app-ui/src/lib/api/usage.ts`: empat pemanggil GET dengan Zod schema (`schemaUsageSummary`,
  dst.) plus tiga pemanggilan quota. Tidak ada pemanggilan dari komponen; semua lewat satu modul
  HTTP (SPEC-UI §10.1).
- `app-ui/src/lib/schemas/usage.ts` + `usage-view.ts` + `usage-search.ts`: periode ter-URL,
  `periodRange` menyelesaikan kedua ujung dari satu `now`, `granularityFor` menurunkan bucket dari
  periode, `usageQuery` membuang filter kosong (karena `group_by=` kosong adalah 400, bukan "tanpa
  breakdown"), `errorRatePercent` mengonversi fraksi ke persen di titik tampil.
- Layar: `routes/usage/+page.svelte` (36 baris) + `UsageOverviewTab` (211) + `UsageRecordsTab`
  (180) + tabel/chart/tiles/drawer/filter. Tiga state (loading/error/empty) ada di kedua tab
  (R-27); drawer detail punya tiga state capture (off / on-tapi-terotasi / on-dengan-log).
- Test: `tests/schemas/usage.test.ts` (355 baris), `usage-view.test.ts`,
  `usage-overview.test.ts` (296), `usage-records.test.ts` (295). Audit ini menjalankan keenam file
  usage + client + navigation: **211 test PASS** (dari jumlah itu 143 di empat file usage).
  `bun run check` (svelte-check) juga **0 errors 0 warnings**.
- Semua file `app-ui/src` terkait usage di bawah 220 baris warning.

## 4. Bukti verification yang dijalankan audit ini

Perintah dijalankan dari `app-serv/` dan `app-ui/` sesuai kolom. Probe mekanis (test sementara)
dijalankan lalu **dihapus**; `git status` pada akhir audit bersih dari jejak probe.

| Pemeriksaan | Hasil | Catatan |
|---|---|---|
| `go build ./...` (app-serv) | PASS | Go 1.26.5 |
| `go vet ./internal/handler/... ./internal/service/... ./internal/router/...` | PASS | |
| `go test -count=1 ./internal/handler/` | PASS | 4.3s |
| `go test -count=1 ./internal/schema/... ./internal/service/...` | PASS | |
| `go test -count=1 ./internal/router/ ./internal/repository/...` | PASS | |
| Probe `DecodeUsageFilter` (status `banana`/`SUCCESS`/``) | **LOLOS VALIDASI** | temuan F2 |
| Probe `q` 250 char / `from=banana` / rentang terbalik | ditolak `VALIDATION_ERROR` | sesuai desain |
| Probe `http.Flusher` melalui `chain()`/`envelope()`/`responseRecorder` | **false di ketiganya** | temuan F5 |
| Probe SSE timing: handler bare vs `chain()` | bare: frame pertama tiba 1.29 ms (`1.291408ms`); `chain()`: **tidak ada yang tiba dalam 2.5 dtk** | temuan F5 |
| Probe `UsageHandler` di seluruh `*_test.go` | **0 file** | temuan F1 |
| `bun run check` (app-ui) | PASS | 0 errors 0 warnings |
| `vitest run` usage + client + navigation (6 file) | PASS | 211/211 |
| `vitest run` suite penuh (`bun run test`) | **timeout 300s dan 600s pada sesi audit ini** | transform vitest lambat (45s untuk 6 file); bukan kegagalan assertion. Bukti per-file dipakai; suite penuh dijalankan ulang di batch DURING |
| `git status` app-serv setelah probe | bersih dari probe | hanya perubahan actor opencode/forced-chat yang sudah ada sebelum audit |

## 5. Temuan yang memerlukan keputusan owner (app-serv)

### F1 HIGH: Keempat route Usage tidak punya HTTP handler test sama sekali

**Status: CLOSED 2026-09-22 (dikerjakan setelah F2/F9 supaya ekspektasi test menulis aturan yang benar; bukti di bawah).**

**Fakta.** `grep -rln "UsageHandler" --include="*_test.go"` mengembalikan nol file. Coverage yang
ada: service doubles (`usage_readiness_test.go`, 135 baris), schema quota-cap
(`usage_record_test.go`), integration Postgres (tag `integration`), dan sweep session 401 di router.
Yang tidak ada: satu pun test yang memanggil `h.Summary/h.Timeseries/h.Records/h.Detail` lewat
`httptest` dengan request nyata. Bandingkan: chat punya sembilan file test HTTP setelah draft 009 F4,
quota punya `quota_test.go` + `quota_read_test.go`.

**Risiko dan aturan.**

- AGENTS.md §2.1: "Every new HTTP route has: one happy-path test, one validation-failure test, one
  auth-failure test if protected". Keempat route Usage protected dan tidak satu pun punya.
- TDD.md §2.5: table-driven 3-5 variasi per fungsi.
- Tanpa ini, regression pada mapper response (`UsageTotalsResponseFrom`, `UsageRecordDetailFrom`)
  hanya tertangkap oleh panel, bukan oleh gate app-serv.

**Rencana DURING (BE).**

1. `internal/handler/usage_test.go`: table per route; happy (200 + shape), filter valid penuh,
   `group_by` invalid, `granularity` invalid, `from` malformed, `q` over-limit, `status` invalid
   (tergantung F2), halaman `per_page=0`/`per_page=999`, request_id kosong, repo error ->
   `INTERNAL_ERROR`.
2. `internal/router/router_usage_routes_test.go`: wrong verb 405 + envelope `METHOD_NOT_ALLOWED`,
   401 tanpa session (memakai fixture router), `X-Request-Id` ter-echo.
3. Double repo yang sudah ada di service test dipakai ulang lewat seam `UsageService`; jangan
   `&service.UsageService{}` zero-value pada happy path.

**Kriteria selesai.** Setiap route punya happy + validation + auth test hijau di
`go test -race ./internal/handler/ ./internal/router/`; tabel minimal 5 variasi; tanpa `t.Skip`.

#### Bukti penutupan F1 (2026-09-22)

**Implementasi.** Tidak ada satu baris pun handler produksi yang berubah; yang hilang adalah
coverage-nya, jadi yang ditambah adalah test (plus satu perubahan perilaku F6 di decoder bersama,
lihat F6). Test memakai seam yang sudah ada (`UsageService` dibangun dari repo in-memory), bukan
zero-value service.

| File | Test | Isi |
|---|---|---|
| `internal/handler/usage_test.go` | `TestUsageHandler_Summary` | 8 variasi: happy dengan group breakdown (`error_rate:"1.0000"` dipaksa oleh seed row gagal, jadi mapper benar-benar diuji), happy tanpa group_by, `from` malformed, rentang terbalik, `group_by` invalid, `status` invalid (regresi F2), `q` 201 char, repo error -> `INTERNAL_ERROR` |
| sama | `TestUsageHandler_SummaryForwardsFilters` | filter valid penuh diteruskan ke repo (window, provider, endpoint, model, gateway_key, q, status, group_by) |
| sama | `TestUsageHandler_Timeseries` | 5 variasi: default hour, day, `granularity=week` ditolak, `to` malformed, repo error |
| `internal/handler/usage_records_test.go` | `TestUsageHandler_Records` | 7 variasi: default (meta `per_page:25 total:1`), `page=3&per_page=100` di-echo, `status=banana` 400, `page=0` 400, `per_page=101` 400 (F6), `per_page=abc` 400, repo error 500 |
| sama | `TestUsageHandler_RecordsForwardsPage` | `page=2&per_page=50` sampai ke `PageQuery` repo, bukan hanya di-echo |
| sama | `TestUsageHandler_Detail` | 5 variasi: capture off (tanpa blok log), capture on + log tersimpan (bodies ada), capture on + log hilang (`capture_enabled:true`, tanpa log; "ter-log" vs "tak pernah di-log" terlihat beda), request id tak dikenal -> 404, request id kosong -> 400 |
| sama | `TestUsageHandler_DetailStorageFailure` | repo error pada detail -> `INTERNAL_ERROR` |
| `internal/handler/usage_stub_test.go` | doubles | `stubUsageRepo` (dengan seam error per read + pencatatan filter/page yang diteruskan), `stubUsageLogs`, `missingUsageLogs`, fixture capture on/off |
| `internal/handler/usage_route_helpers_test.go` | runner | `usageCase` + `runUsageCases` + `assertUsageResponse` (satu jalur assertion untuk semua tabel; file di-split demi §1.1) |
| `internal/router/router_usage_routes_test.go` | `TestUsageRoutes_RequireSession` | 401 `UNAUTHORIZED` untuk keempat route tanpa session (per-route, bukan hanya sweep) |
| sama | `TestUsageRoutes_ThroughMux` | happy keempat route lewat mux nyata + session, lalu POST tiap route -> 405 `METHOD_NOT_ALLOWED` envelope |
| sama | `TestUsageRoutes_EchoRequestID` | `X-Request-Id` caller ter-echo pada respons 200 |

**Bukti test menangkap regresi (mutation check, dijalankan lalu dipulihkan).**

- `schema.UsageTotalsResponseFrom` dimutasi `ErrorRate: t.ErrorRate()` -> konstanta `"0.0000"`:
  `TestUsageHandler_Summary` **FAIL** (`error_rate":"0.0000"` pada seed 1/1). Dipulihkan, hijau.
  (Mutasi pertama dengan seed row sukses lolos karena 0/1 memang "0.0000"; seed lalu diganti ke
  row gagal supaya nilai turunan mapper benar-benar diuji, itulah sebabnya tabel memuat
  `error_rate":"1.0000"`.)
- `schema.UsageRecordDetailFrom` dimutasi `captureEnabled && log != nil` -> `log != nil`:
  `TestUsageHandler_Detail` **FAIL** (`log block present = true, want false` pada kasus capture off).
  Dipulihkan, hijau.
- `router.go` dimutasi: satu route kehilangan `gateway()` (`HandleFunc` langsung):
  `TestUsageRoutes_RequireSession` **FAIL** (`GET /usage/summary without a session = 200, want 401`).
  Dipulihkan, hijau. (Sweep seluruh tabel menangkap ini juga, tapi test per-route kini
  menyebutkan route yang mana.)

**Gate.** `go build ./...`, `go vet ./...`, `gofmt -l .` bersih; `go test -race -count=1 ./...`
PASS penuh, diukur dua kali pada tree yang sama: `internal/handler` 78 dtk lalu 81 dtk,
`internal/router` 283 dtk lalu 302 dtk (durasi suite ini bervariasi antar run, bukan klaim angka
tunggal); `staticcheck` 0 issue; `golangci-lint` 0 issue;
`go-headers.sh` PASS (689 file, diukur pada tree setelah split `usage_route_helpers_test.go`;
pengukuran pertama batch ini membaca 688 sebelum split terjadi); semua file baru <220 baris
(terbesar 218 `usage_records_test.go`).

### F2 HIGH: Filter `status` menerima nilai di luar closed set, dan query diam-diam mencocokkan nol baris

**Status: CLOSED 2026-09-22 (dikerjakan bersama F9; bukti di bawah).**

**Fakta.** Probe mekanis: `?status=banana` dan `?status=SUCCESS` **diterima** `DecodeUsageFilter`
dan diteruskan ke `UsageFilter.Status`. Karena `status` adalah kolom closed set (`success`|`error`),
predicate `($7 = '' OR status = $7)` menjadi `status = 'banana'` yang tidak pernah cocok, sehingga
query mengembalikan **nol baris**. Caller menerima `200` dengan tabel kosong dan membacanya sebagai
"tidak ada request gagal", padahal yang salah adalah nilai filternya. Domain sudah punya closed set
`UsageStatus` (`success`|`error`, `IsValid()` ada di `domain/usage.go:37`) tapi boundary query
tidak memakainya, tidak seperti `group_by`/`granularity` yang sudah divalidasi ketat.

**Risiko dan aturan.**

- AGENTS.md §1.4: every external input validated; closed set domain tidak boleh bocor.
- OWASP A05/A06: input yang "diterima lalu diabaikan" adalah kegagalan desain input handling;
  caller menerima 200 dan membaca tabel kosong sebagai "tidak ada request gagal", padahal
  filternya yang tidak dikenal.
- SPEC-API §7.12 tidak menulis `status` sebagai enum di kontrak YAML juga (lihat F8), jadi dua
  lapis (schema + kontrak) sama-sama longgar.

**Rencana DURING (BE).**

1. Failing test table dulu: `success`, `error`, kosong (valid), lalu `banana`, `SUCCESS`, `SUCCESS `
   (trailing space harus tetap trim lalu ditolak), `null` literal.
2. Validasi di `DecodeUsageFilter`: trim lalu, bila tidak kosong, `domain.UsageStatus(v).IsValid()`
   atau `ParseUsageStatus` baru; error `status must be one of success, error` (bahasa Inggris).
3. Ulang di service (`UsageFilter.Validate` atau `Summary/Timeseries/Records` guard) agar caller
   non-HTTP juga tertangkap.
4. Sinkron kontrak (F8) pada saat yang sama.

**Kriteria selesai.** Nilai di luar set ditolak `400 VALIDATION_ERROR` dengan pesan English; nilai
valid tetap memfilter; test table 5+ variasi termasuk benign control.

#### Bukti penutupan F2 (2026-09-22)

**Implementasi** (bersama F9, satu perubahan tipe + satu boundary check).

- `internal/schema/usage.go`: `DecodeUsageFilter` memanggil `domain.ParseUsageStatus` pada nilai
  `status` yang sudah di-trim, hanya bila non-empty (guard yang sama dengan `group_by`/
  `granularity`); `UsageFilterQuery.Status` menjadi `domain.UsageStatus`. `DecodeLogFilter`
  mewarisinya lewat decoder bersama, jadi route logs memegang set yang sama tanpa mendeklarasi
  ulang.
- `internal/domain/usage_types.go`: `ParseUsageStatus` baru (mirip `ParseUsageGroupBy`), dan
  `UsageFilter.Validate` menolak status di luar set.
- `internal/domain/log.go`: `LogFilter.Validate` baru dengan aturan yang sama.
- `internal/service/log.go`: `Requests` memanggil `filter.Validate()` (defense in depth; usage
  service sudah melakukannya).
- Kontrak: `status` jadi `enum: [success, error]` pada `summary`/`timeseries`/`records`/
  `logs/requests` di `docs/CONTRACT/001-CONTRACT-API-V1.yaml`; `openapi.json` di-regenerasi lewat
  `tools/openapi-gen` (tidak diedit tangan); `contract-openapi.sh` PASS.

**Test** (semuanya table-driven, benign control termasuk, TDD §2.5):

| File | Test | Kasus |
|---|---|---|
| `internal/schema/usage_filter_status_test.go` | `TestDecodeUsageFilter_StatusClosedSet` | 12 (3 benign: empty/success/error; 9 ditolak: `banana`, `SUCCESS`, `Success`, trimmed uppercase, `success,error`, SQL fragment, `null`, `,`; trailing-space di-trim lalu diterima sebagai `success`) |
| sama | `TestDecodeUsageFilter_StatusFlowsToDomainFilter` | 3 benign (nilai lolos sampai domain filter) |
| sama | `TestDecodeLogFilter_InheritsStatusClosedSet` | logs route mewarisi set (tolak `banana`, terima `error`) |
| `internal/domain/usage_types_status_test.go` | `TestNewUsageFilter_StatusClosedSet` + `TestNewLogFilter_StatusClosedSet` | 9 kasus dibagikan dua filter (closed set by construction + `Validate`) |
| sama | `TestUsageFilter_ValidateStillRejectsInvertedRange` | regresi: aturan range lama tetap |
| `internal/repository/postgres/usage_status_filter_integration_test.go` (tag `integration`) | `TestUsageRepository_StatusFilterMatchesRows` | 5 baris seed (2 sukses, 3 gagal): unfiltered=5, `success`=2, `error`=3, `error_count` summary cocok |

**RED → GREEN.** Test schema + domain ditulis dulu dan **gagal** terhadap kode pra-perbaikan
(`banana`/`SUCCESS`/`Success`/`success,error`/`null`/SQL fragment diterima; domain tidak compile
karena field masih `string`), lalu hijau setelah implementasi. Panel sudah memvalidasi field ini
sebagai Zod enum `['success','error']` (`primitives.ts` `REQUEST_STATUSES`), jadi tidak ada
perubahan FE yang dibutuhkan.

**Bukti SQL.** Test integrasi dijalankan terhadap PostgreSQL nyata pada database throwaway
`pannelai_f2f9_evidence` (dibuat untuk ini, dihapus setelahnya; database dev tidak disentuh),
termasuk seluruh suite integrasi `internal/repository/postgres` (semua PASS, `-race`) untuk
membuktikan perubahan tipe `string` → `UsageStatus` tidak merusak encode pgx.

**Gate.** `go build ./...`, `go vet ./...`, `go test -count=1 ./...` (lalu `-race` penuh di bawah)
pASS; `gofmt -l .` bersih; `staticcheck` 0 issue; `golangci-lint` 0 issue; `go-headers.sh` PASS;
`contract-openapi.sh` PASS.

**Dokumen.** SPEC-API changelog 2026-09-22 (§7.12/§7.13) mencatat enum + value object ini.

### F3 MEDIUM: `latency_ms` wire adalah sum, bukan mean, dan domain tidak pernah menjelaskannya

**Status: CLOSED 2026-09-22 (keputusan owner D1 = c: dokumentasikan sebagai sum; bukti di bawah).**

**Fakta.** `usageTotalsProjection` menjumlahkan `latency_ms` per baris (`sum(latency_ms) AS
latency_total`), dan `UsageTotals.LatencyMS` diisi dari situ. Karena p50/p95 dihitung di baris yang
sama, `latency_ms` pada summary/group/bucket adalah **total milidetik semua request**, angka yang
naik seiring jumlah request dan tidak punya pembacaan operasional tunggal. Panel sudah menyadari dan
sengaja tidak menampilkannya (`UsageTotalsTiles` komentar: "not among them"), dan `UsageRecordTable`
menampilkan `latency_ms` per-row (di mana maknanya benar). Tapi wire tetap mengirim angka yang
menyesatkan pada setiap aggregate, dan kontrak tidak mendokumentasikan semantiknya.

**Risiko dan aturan.**

- R-17/C-5 (evidence over claims): angka di wire yang mudah dibaca sebagai "latency" padahal sum.
- AGENTS.md §1.3 (honest docs): kontrak menyebut field tanpa semantik.

**Rencana DURING (BE).** Pilih salah satu, owner memutuskan:
(a) ganti semantic jadi mean (`avg(latency_ms)`), (b) hapus dari aggregate response dan biarkan
hanya p50/p95 + per-record, atau (c) dokumentasikan eksplisit `latency_ms = total` di YAML +
SPEC-API §7.12 dan panel (sudah sinkron dengan tidak-menampilkannya). Opsi (c) termurah dan jujur;
opsi (b) breaking change kecil yang perlu dicek konsumen. Apa pun pilihannya, tulis di kontrak.

**Kriteria selesai.** Semantik `latency_ms` dinyatakan di SPEC-API §7.12 dan YAML; bila diubah,
panel dan test disinkronkan di batch yang sama.

#### Bukti penutupan F3 (2026-09-22, keputusan owner D1 = c: dokumentasikan sebagai sum)

**Implementasi.** Tidak ada aritmetika yang berubah: `sum(latency_ms)` tetap jumlah, sesuai
keputusan D1 (c). Yang berubah adalah kejujuran kontrak dan satu test yang mengunci sum supaya
perubahan ke mean di kemudian hari tidak bisa lolos diam-diam.

- `docs/CONTRACT/001-CONTRACT-API-V1.yaml`: `UsageTotals.latency_ms` menyatakan dirinya sebagai
  sum yang tumbuh seiring jumlah request, `latency_p50_ms`/`latency_p95_ms` menyatakan gambar
  per-request, dan `UsageRecordResponse.latency_ms` menyatakan "one call", bukan bagian dari
  window. `openapi.json` di-regenerasi lewat `tools/openapi-gen`, tidak diedit tangan.
- `docs/SPEC-API/001-SPEC-API.md` §7.12: satu paragraf "Aggregate latency is a sum" yang menyebut
  keputusan, angka mana yang per-request, dan alasan field itu tidak diubah.
- Panel tidak berubah: `UsageTotalsTiles.svelte` memang sudah menolak menampilkan `latency_ms`
  aggregate dengan alasan yang sekarang tertulis di kontrak, jadi komentarnya tetap benar dan
  tidak perlu disinkronkan ulang.

**Test.** `internal/handler/openapi_usage_semantics_test.go`
`TestOpenAPIContract_AggregateLatencyIsStatedAsASum` (3 subtest) membaca dokumen yang **disajikan**,
 bukan YAML-nya. Terhadap `openapi.json` pra-perbaikan ketiganya **FAIL** (tiga properti tanpa
`description`), hijau setelah regenerasi. `internal/repository/postgres/usage_latency_semantics_integration_test.go`
`TestUsageRepository_AggregateLatencyIsASumNotAMean` (4 subtest) menyeed tiga baris 100/200/300 ms:
sum = 600, mean = 200, jadi keduanya angka yang berbeda dan assertion benar-benar memilih satu.
Test aritmetika yang sudah ada menyeed tiga baris sama-sama 100 ms, sehingga sum dan mean dua-duanya
membaca 300 dan tidak bisa membedakannya; itulah kenapa test ini terpisah.

**Mutation check (dijalankan lalu dipulihkan).** `sum(latency_ms)` dimutasi menjadi
`avg(latency_ms)::bigint`: `TestUsageRepository_AggregateLatencyIsASumNotAMean` **FAIL**
(`LatencyMS = 200, want 600 (the sum); a mean would read 200`, plus bucket yang sama), lalu hijau
setelah dipulihkan. Deskripsi `UsageTotals.latency_ms` dihapus dari `openapi.json` yang disajikan:
`TestOpenAPIContract_AggregateLatencyIsStatedAsASum` **FAIL** (`carries no description`), lalu
hijau setelah `tools/openapi-gen` dijalankan ulang.

**Gate.** `go build ./...`, `go vet ./...` (default dan `-tags=integration`), `gofmt -l .` bersih;
`go test -race -count=1 ./...` PASS penuh; suite integrasi `-race` PASS terhadap database throwaway;
`staticcheck` (default dan tagged) 0 issue; `golangci-lint` 0 issue; `go-headers.sh` PASS;
`contract-openapi.sh` PASS.

### F4 MEDIUM: `UsageEvent` / `usage.recorded` domain event tidak punya publisher maupun subscriber

**Status: OPEN.**

**Fakta.** `internal/domain/usage_event.go` mendefinisikan `UsageEventName = "usage.recorded"`,
`UsageEvent`, dan `UsageRecord.NewUsageEvent()`. `grep` seluruh non-test Go: **nol caller**. Comment
service `Record` menyebut "returning the stored record so a caller can publish its event", tapi
tidak ada yang mem-publish (tidak ada Redis Pub/Sub di app-serv; satu-satunya transport event calon
adalah Redis yang sudah ada sebagai dependency). AGENTS.md §2.3 (EDD) mewajibkan event pada mutasi
aggregate penting; saat ini hanya komentar.

**Risiko dan aturan.**

- AGENTS.md §2.3: stateful cross-domain harus via event async; today `quota flush` membaca Redis
  counter yang ditulis langsung, bukan event.
- Kode mati yang mengklaim arsitektur (R-36 pada dokumentasi kode).

**Rencana DURING (BE).** Pilihan owner: (a) publish ke Redis Pub/Sub channel `usage.recorded` dari
satu choke point `UsageService.Record` (goroutine with panic recovery + bounded buffer + drop
policy, AGENTS.md §1.6) dan buat subscriber internal (mis. invalidasi ring/console) sebagai bukti
hidup; atau (b) hapus `usage_event.go` + test-nya dan amandemen AGENTS.md §2.3 note bahwa EDD
diterapkan per-domain pada saat ada konsumen nyata. Jangan biarkan keduanya setengah.

**Kriteria selesai.** Event punya minimal satu publisher dan satu subscriber nyata di produksi,
atau file dihapus dengan alasan tertulis. `-race` clean.

### F5 HIGH: `sseSink.Flush()` tidak pernah mencapai klien; middleware chain menyembunyikan `http.Flusher`

**Status: CLOSED 2026-09-22 (scope app-serv, tanpa perubahan app-ui).** Ini root cause defect yang
tercatat di `app-ui/README.md` dan SPEC-UI §16 sebagai "request to app-serv". Audit ini
memperketatnya menjadi temuan bernomor.

**Fakta.** Probe mekanis terhadap wrapper produksi:

- `http.Flusher` melalui `chain(...)`: **false**;
- melalui `envelope(...)`: **false**;
- melalui `responseRecorder`: **false**.

Probe timing dengan `httptest.Server` nyata: handler SSE bare mengirim frame pertama ke klien dalam
**1.29 ms** (terukur `1.291408ms`); handler yang sama dibungkus `chain()` mengirim **tidak ada apa
pun** selama handler masih terbuka (probe menunggu 2.5 s). Sebab: `chain = requestID(logging(recoverer(envelope(mux))))`,
dan `logging` membungkus writer dengan `responseRecorder` yang hanya meng-embed
`http.ResponseWriter` + `WriteHeader`/`Write`/`SetErrorCode`. Go mempromosikan method interface
embed, tapi **bukan interface tambahan** pada concrete writer di belakangnya, jadi assertion
`w.(http.Flusher)` gagal, `newSSESink` menyimpan `flusher == nil`, `Flush()` menjadi no-op, dan
seluruh jawaban stream tertahan di buffer TCP/kernel sampai handler selesai. Ini persis yang
dilaporkan panel: "the whole answer is written at once ... the stub's own frame-by-frame writes
arrive as one 1074-byte read".

Dampaknya pada scope audit ini: **seluruh klaim "SSE" app-serv saat ini adalah SSE yang tidak
streaming per-frame**. Playground panel sudah punya relay sendiri yang terbukti streaming frame by
frame (isolated pass), jadi layar itu selamat; tapi setiap route SSE app-serv (chat, messages,
responses, dan calon route SSE Usage bila F10 dipilih) mewarisi defect ini. Test handler
(`chat_stream_lifecycle_test.go`) tidak menangkapnya karena `httptest.ResponseRecorder`
mengimplementasikan `Flush()` sendiri sehingga sink "berfungsi" di test.

**Risiko dan aturan.**

- SPEC-API §2 "Streaming" (baris 101): passthrough SSE dengan `X-Accel-Buffering: no`; header
  tanpa flush per frame tidak memenuhi janji itu.
- `docs/SPEC-UI/001-SPEC-UI.md` §6.15 "answer streams: the text grows frame by frame".
- OWASP A10 / AGENTS.md §1.3: perilaku yang diklaim berbeda dari perilaku yang dikirim.
- R-26/C-2 pada boundary API: output harus benar-benar dapat dipakai caller.

**Rencana DURING (BE).**

1. Failing test dulu di level router (bukan handler): satu `httptest.Server` + `chain()`, handler
   SSE dummy, assert frame pertama terbaca klien **sebelum** handler melepas `release` channel.
   (Probe audit ini sudah bentuk kasarnya; jadikan test permanen.)
2. `responseRecorder` dan `statusRecorder` menambahkan `Flush()` yang mem-forward:
   `if f, ok := rec.ResponseWriter.(http.Flusher); ok { f.Flush() }`. Kedua recorder di kedua file
   (`middleware.go`, `envelope.go`) agar rantai penuh meneruskan.
3. Pertimbangkan `Unwrap() http.ResponseWriter` (konvensi Go 1.20+) di kedua recorder agar wrapper
   masa depan tidak mengulang kelas bug ini.
4. Jalankan ulang live evidence style draft 009 §10.9 dan perbarui `app-ui/README.md` (tabel
   defect) serta SPEC-UI §16 (kalimat "the gateway writes the whole answer at once, so its
   `sseSink.Flush()` never reaches the client") menjadi resolved, dengan tanggal.

**Kriteria selesai.** Frame pertama terukur sampai klien sebelum handler selesai pada test
permanen; `sseSink.Flush()` berfungsi di bawah rantai produksi; dokumen panel dan spec diperbarui.

#### Bukti penutupan (2026-09-22)

**Implementasi.**

- `internal/router/middleware.go`: `responseRecorder.Flush()` meneruskan ke writer dalam, dan
  `responseRecorder.Unwrap()` mengekspos writer itu agar `http.ResponseController` dapat menembus
  wrapper.
- `internal/router/envelope.go`: `statusRecorder.Flush()` (melewati respons yang sedang
  `suppressed`, karena flush akan meng-commit status lebih dulu) dan `statusRecorder.Unwrap()`.
- `internal/handler/datplane_errors.go`: `newSSESink` menyimpan `http.ResponseController` dan
  `Flush()` memanggil `controller.Flush()`; jalur `Unwrap()` membuat wrapper yang hanya mengekspos
  `Unwrap()` tetap dapat di-flush.

**Test permanen** (`internal/router/router_stream_flush_test.go`, 219 baris, 3 test table-driven):

| Test | Hasil |
|---|---|
| `TestStreamFlush_EveryLayerForwardsFlusher` (bare control, `logging`, `envelope`, `chain`) | PASS |
| `TestStreamFlush_ControllerReachesThroughUnwrap` (bare control, `chain`) | PASS |
| `TestStreamFlush_FirstFrameArrivesBeforeHandlerReturns` (socket nyata, bare control, `chain`) | PASS |

Ketiganya **FAIL** terhadap kode pra-F5 (bare control PASS, lapisan produksi FAIL: `http.Flusher`
tidak terlihat, `ResponseController.Flush()` = `feature not supported`, status line timeout 3 dtk),
yang menjadikannya regresi, bukan test yang mengikuti implementasi.

**Bukti live** (`cmd/app-serv/playground_live_flush_test.go`, tag `integration`) terhadap database
throwaway `pannelai_flush_evidence` + Redis terpisah pada port 6399, upstream SSE yang menahan frame
kedua 750 ms:

| Konfigurasi | Frame pertama tiba | Hasil |
|---|---|---|
| Pra-F5 (perbaikan di-stash) | **770.738869 ms** | FAIL, tertahan sampai handler selesai |
| Pasca-F5 | **16.744005 ms** | PASS, jauh sebelum frame kedua upstream |

Dua pengukuran dijalankan pada stack throwaway yang sama, sehingga selisih 754 ms adalah efek
perbaikan dan bukan efek lingkungan. Regresi `TestPlaygroundLive` yang sudah ada juga PASS
(`status=200`, `usage_rows=1 log_rows=1`, `tokens_in=7 tokens_out=3`, `stream usage_rows=1`, empat
refusal path sesuai kontrak).

**Gate.** `go build ./...`, `go vet ./...`, `gofmt -l .` bersih; `go test -race -count=1 ./...` PASS
(termasuk `internal/router` 164 s); `go vet -tags=integration ./...` PASS; `staticcheck ./...` 0
issue; `golangci-lint run ./...` 0 issue; `go-headers.sh` PASS (679 file). Stack throwaway
dihapus dan database dev dikembalikan ke keadaan semula.

**Dokumen.** `app-serv/README.md`, `SYSTEM_MAP.md` §3.2, `docs/SPEC-UI/001-SPEC-UI.md` §16, dan
`app-ui/README.md` (tabel defect) menyebut penutupan ini dengan tanggal; dua defect lain di tabel
yang sama tetap terbuka dan tetap menjadi permintaan ke `app-serv`.

### F6 MEDIUM: `per_page` diklaim 1..100 oleh UI, diklem oleh handler, tapi kontrak tidak menyatakan batas

**Status: CLOSED 2026-09-22 (keputusan owner D3 = Tolak; bukti di bawah).**

**Fakta.** `DecodePage` menolak `per_page < 1` dan **men-klem** `per_page > 100` ke 100 secara
diam-diam (`if perPage > MaxPerPage { perPage = MaxPerPage }`). Panel (`primitives.ts` `perPage`)
memvalidasi 1..100 di Zod dan `usage-search.ts` bahkan menampilkan notice bila URL membawa
`per_page` selain 25. Kontrak YAML menyatakan `per_page` hanya `type: string` tanpa `minimum`,
`maximum`, atau `default`. Tiga lapis punya tiga perilaku: UI menolak, BE meng-klem, kontrak
diam.

**Risiko dan aturan.** AGENTS.md §2.4 CDD (kontrak = sumber kebenaran), §1.4. Klem diam juga
bertabrakan dengan pola panel "visible correction" (§7.1.1): koreksi server tidak terlihat
caller sama sekali.

**Rencana DURING (BE).** Putuskan: tolak `per_page > 100` dengan `400` (konsisten dengan
`per_page < 1` yang sudah ditolak, dan dengan `page < 1`), atau dokumentasikan klem di YAML
(`maximum: 100`, `default: 25`) dan SPEC-API §4. Rekomendasi audit: **tolak**, karena perilaku
refuse-lebih-jelas daripada-meng-klem sudah menjadi pola handler ini untuk nilai di bawah 1.

**Kriteria selesai.** Kontrak menyatakan `minimum/maximum/default`; implementasi dan kontrak
sepakat; test table untuk 0, 1, 100, 101, `abc`, kosong.

#### Bukti penutupan F6 (2026-09-22)

**Implementasi.** `internal/schema/dto.go` `DecodePage`: cabang klem `perPage > MaxPerPage ->
perPage = MaxPerPage` diganti `VALIDATION_ERROR "per_page must be at most 100"`, konsisten dengan
penolakan `page < 1`/`per_page < 1` yang sudah ada. Karena decoder ini dibagi seluruh list
management (gateway-keys, providers, endpoints, combos, logs, usage), satu titik menutup semua
route. Panel tidak terdampak: setiap pemanggil panel memakai 25 (atau `per_page: 100` pada quota,
yang tetap legal).

**Kontrak.** Keempat deklarasi `page`/`per_page` di YAML (`providers`, `endpoints`,
`usage/records`, `logs/records`) kini `type: integer, minimum: 1, default: 1` (page) dan
`minimum: 1, maximum: 100, default: 25` (per_page); dua deklarasi yang sebelumnya `type: string`
juga dikoreksi ke `integer`, karena decoder memang mem-parse integer. `openapi.json`
di-regenerasi lewat `tools/openapi-gen` (tidak diedit tangan); `contract-openapi.sh` PASS.

**Test (RED -> GREEN).** `internal/schema/dto_page_test.go`
`TestDecodePage_BoundsIsRefusalNotClamp`: 12 kasus table-driven (4 benign control: kosong
default 25, `page=1&per_page=1`, `per_page=100`, mid-range; 8 penolakan: `page=0`, `page=-2`,
`page=abc`, `per_page=0`, `per_page=101`, `per_page=99999`, `per_page=-5`, `per_page=x`).
Terhadap kode pra-perbaikan, tepat dua kasus over-cap **FAIL** (`(1, 100, nil), want a validation
error`) dan sisanya pass, yang membuktikan test menargetkan klem diam itu sendiri.
`TestGatewayKey_List_Pagination` lama yang memaku klem (`?per_page=99999` -> 200/100) dipindah
kasusnya ke tabel `TestGatewayKey_List_InvalidPage` sebagai dua penolakan baru, karena perilaku
lamanya kini ilegal. `TestUsageHandler_Records` (F1) memaku `per_page=101 -> 400` di route.
Kontrak dipaku `TestOpenAPIContract_PaginationParamsAreBounded` (lihat F7).

**Gate.** `go test -race -count=1 ./...` PASS penuh; `staticcheck`/`golangci-lint` 0 issue;
`contract-openapi.sh` + `contract-drift.sh` PASS.

### F7 LOW: OpenAPI `from`/`to` tanpa `format: date-time`, `status`/`group_by`/`granularity` tanpa enum

**Status: CLOSED 2026-09-22 (sisa poin `from`/`to` + `group_by`/`granularity` dikerjakan bersama F6; bukti di bawah; `status` sudah tertutup bersama F2/F9).**

**Fakta.** YAML path Usage mendeklarasikan `from`/`to` sebagai `type: string` polos (kontrak ini
memuat 38 deklarasi `format: date-time` di tempat lain, jadi pola itu sudah ada), dan
`status/group_by/granularity` tanpa `enum` meski domain dan schema menolak nilai liar.
Generator `tools/openapi-gen` meng-copy YAML, jadi perbaikan harus mulai dari YAML lalu regenerasi
JSON + gate `contract-openapi.sh`.

**Risiko dan aturan.** AGENTS.md §2.4 CDD; R-38/C-5 untuk kontrak machine-readable yang bisa
diaudit konsumen eksternal.

**Rencana DURING (BE).** Satu edit YAML per parameter (`format: date-time` untuk `from`/`to`;
`enum: [success, error]`, `enum: [provider, model, endpoint, gateway_key]`,
`enum: [hour, day]`), `go run ./tools/openapi-gen`, gate, dan parity test sudah ada akan
mem-verifikasi.

**Kriteria selesai.** `openapi.json` generated membawa format/enum; `contract-openapi.sh` PASS;
tidak ada edit tangan pada JSON.

#### Bukti penutupan F7 (2026-09-22)

**Kontrak.** Satu pass edit YAML pada path Usage: `from`/`to` di `summary`, `timeseries`,
`records`, dan `logs/requests` kini `type: string, format: date-time` (8 deklarasi); `group_by`
kini `enum: [provider, model, endpoint, gateway_key]` dan `granularity` `enum: [hour, day]`
pada `summary` dan `timeseries` (4 deklarasi). `openapi.json` di-regenerasi lewat
`tools/openapi-gen` (tidak diedit tangan); `contract-openapi.sh` PASS.

**Test (RED -> GREEN).** `internal/handler/openapi_usage_params_test.go`, tiga test
table-driven yang membaca dokumen tersaji (bukan YAML, jadi yang diperiksa adalah apa yang
konsumen terima):

| Test | Kasus | Pra-perbaikan |
|---|---|---|
| `TestOpenAPIContract_UsageWindowParamsAreDateTimes` | 4 path x 2 param (`from`/`to`) + aturan `required: false` | 8 FAIL (`format = ""`) |
| `TestOpenAPIContract_UsageEnumParams` | 8 kombinasi path/param (`group_by`, `granularity`, `status`) | 4 FAIL (`group_by`/`granularity` enum kosong; `status` sudah enum dari F2) |
| `TestOpenAPIContract_PaginationParamsAreBounded` | seluruh deklarasi `page`/`per_page` di dokumen (4 path) | 14+ FAIL (minimum/default kosong, `page type: "string"` pada 2 path) |

Total **26 assertion failure dalam 12 subtest** pra-edit YAML (diukur ulang terhadap
`openapi.json` pra-perbaikan dari `HEAD~1`, bukan dikutip dari output pertama yang ter-truncate),
hijau setelah regenerasi. `TestOpenAPICoversEveryRegisteredRoute`
(parity dua arah route) tetap PASS, membuktikan perubahan schema tidak mengubah kumpulan path.

### F8 MEDIUM: Kontrak vs implementasi pada `q` scope: filter bebas yang hanya menyentuh `model`

**Status: CLOSED 2026-09-22 (keputusan owner D4 = perluas; bukti di bawah).**

**Fakta.** `usageFilterClause` baris terakhir: `AND ($8 = '' OR model ILIKE '%' || $8 || '%')`.
Komentar repository menyatakan "§7.13 dan panel keduanya menscope ke nama model". Tapi placeholder
panel (`UsageRecordsFilters.svelte`, dan `LogsFilters.svelte` juga) menulis **"Request id, error
code"** sedangkan SPEC-UI §6.5 menulis "free-text `q`" tanpa scope, dan SPEC-API §7.12 tidak
mendefinisikan scope `q` sama sekali (hanya §7.13 untuk logs yang menyebutnya implisit lewat
komentar repo). Operator yang mengetik request id di Search akan mendapat hasil kosong tanpa tahu
kenapa.

**Risiko dan aturan.** R-26 pada API boundary (kontrol yang tidak melakukan apa yang labelnya
janjikan), AGENTS.md §1.3 (honest docs). Ini juga persis pola "klaim tanpa bukti" yang antislop
R-36 larang pada UI.

**Rencana DURING.** Owner memilih scope: (a) `q` tetap model-only, maka SPEC-API §7.12 +
SPEC-UI §6.5 + placeholder panel diperbaiki menyebut "model name" (BE: kontrak; FE: placeholder
`Search model`, satu string); (b) `q` diperluas jadi `request_id OR error_code OR model`
(`ILIKE` pada kolom text, tetap parameterized, masih bounded 200 char) karena itu yang placeholder
panel janjikan. Rekomendasi audit: **(b)**, karena dua placeholder panel sudah menjanjikannya dan
request id adalah pencarian paling wajar di layar records; tapi itu perubahan SQL + test.

**Kriteria selesai.** Scope `q` tertulis di SPEC-API §7.12 dan YAML; implementasi dan placeholder
panel sepakat; test table `q=usg_` (id), `q=MODEL_NOT_FOUND` (error code), `q=gpt` (model).

#### Bukti penutupan F8 (2026-09-22, keputusan owner D4 = perluas)

**Implementasi (BE).** Predikat bebas diperluas di dua `FILTER` yang sudah ada, tanpa menambah
parameter dan tanpa membangun SQL dari teks caller:

- `internal/repository/postgres/usage.go` `usageFilterClause`: `$8` kini
  `request_id ILIKE ... OR id ILIKE ... OR error_code ILIKE ... OR model ILIKE ...`.
- `internal/repository/postgres/log.go` `logFilterClause`: `$7` kini
  `request_id ILIKE ... OR error ILIKE ... OR model ILIKE ...` (baris log membawa teks error,
  baris usage membawa kode error, jadi kedua scope memang berbeda dan disebut terpisah).
- Nilai tetap satu bind parameter (`'%' || $n || '%'`), tetap dibatasi 200 karakter di
  `DecodeUsageFilter`, jadi OWASP A05 tidak berubah: tidak ada teks caller yang menjadi SQL.
- `internal/repository/postgres/log.go` (225 baris) masuk band warning §1.1 saat predikatnya
  bertambah, jadi scanner-nya dipindah ke `log_scan.go` (83 baris) mengikuti pola `usage_scan.go`
  yang sudah ada di package yang sama; `log.go` kini 166 baris.

**Temuan tambahan saat implementasi (coalesce adalah kode mati).** Draf awal menambahkan
`coalesce(error_code, '')` untuk menjaga NULL. Mutation check membuktikan coalesce itu **tidak
load-bearing**: di SQL tiga-nilai, `false OR NULL` mengecualikan baris persis seperti `false`, dan
`TRUE OR NULL` adalah `TRUE`, sementara `request_id`, `id`, dan `model` semuanya `NOT NULL`
(dibuktikan langsung di PostgreSQL: `select (true or null), (false or null)`). Karena kode tanpa
pembuktian itu yang antislop R-17/R-36 larang, coalesce dihapus dan komentar diganti dengan fakta
yang benar, bukan klaim yang tidak bisa gagal.

**Kontrak.** `q` kini menyatakan scope-nya di empat path (`summary`, `timeseries`, `records`,
`logs/requests`); `openapi.json` di-regenerasi lewat `tools/openapi-gen`; `docs/SPEC-API/001-SPEC-API.md`
§7.12 menambah paragraf "Free-text `q` scope" dengan keputusan D4 dan alasan placeholder panel.

**Panel (FE).** Tidak ada perubahan kode: placeholder "Request id, error code" di
`UsageRecordsFilters.svelte` dan `LogsFilters.svelte` sekarang **benar** karena BE melayani keduanya.
Itu sekaligus menutup F11 sebagai konsekuensi, bukan sebagai pekerjaan FE terpisah.

**Test (RED -> GREEN, semuanya table-driven, TDD §2.5).**

| File | Test | Kasus |
|---|---|---|
| `internal/repository/postgres/usage_query_scope_integration_test.go` | `TestUsageRepository_QueryScopeMatchesIdentityAndError` | 13 kasus: 4 benign control (kosong, `req_`, `usg_`, substring bersama), id, record id, error code (dengan varian huruf kecil), model (dengan varian huruf besar), baris ber-`error_code` NULL yang cocok lewat kolom lain, nilai tak cocok -> 0 baris, dan fragmen SQL yang tetap literal |
| sama | `TestUsageRepository_QueryScopeNarrowsTheAggregates` | scope yang sama menyempitkan `Summary` dan `Timeseries`, bukan hanya `List` |
| `internal/repository/postgres/log_query_scope_integration_test.go` | `TestLogRepository_QueryScopeMatchesIdentityAndError` | 11 kasus dengan set yang sama untuk request_id/error/model |
| `internal/handler/openapi_usage_semantics_test.go` | `TestOpenAPIContract_FreeTextScopeIsStated` | 4 path, membaca dokumen tersaji |

Terhadap kode pra-perbaikan: 13 kasus SQL **FAIL** (`rows = 0, want N`) dan 4 assertion kontrak
**FAIL** (`q description = ""`), hijau setelah implementasi. Mutation checks: predikat dikembalikan
ke model-only -> FAIL; arm `error_code` dihapus sendirian -> FAIL; arm `id` dihapus -> FAIL; ini
membuktikan tiap arm dipaku, bukan hanya salah satu.

**Gate.** `go build ./...`, `go vet ./...` (default dan tagged), `gofmt -l .` bersih;
`go test -race -count=1 ./...` PASS penuh; suite integrasi `-race` PASS terhadap database throwaway
`pannelai_f3f8_evidence` (dibuat untuk bukti ini, dihapus setelahnya; database dev tidak disentuh);
`staticcheck` 0 issue (default dan tagged); `golangci-lint` 0 issue; `go-headers.sh` PASS
(694 file); `contract-openapi.sh` dan `contract-drift.sh` PASS.

### F9 LOW: `UsageStatus` tidak dipakai sebagai tipe pada `UsageFilter.Status` (string mentah lolos domain)

**Status: CLOSED 2026-09-22 (dikerjakan bersama F2).**

**Fakta.** `UsageFilter.Status` bertipe `string`, bukan `UsageStatus`, dan `UsageFilterInput`
sama. Domain punya `UsageStatus` + `IsValid()` tapi field filter memakai `string`, sehingga F2
bisa terjadi: tidak ada tipe yang menolak nilai liar pada jalur non-HTTP sekalipun.

**Risiko dan aturan.** AGENTS.md §2.2 (value object over primitive), §1.4.

**Rencana DURING (BE).** Ganti field ke `UsageStatus` + validasi di constructor
`NewUsageFilter` (nilai non-empty harus `IsValid`). Satu perubahan tipe menutup kelas, bukan satu
nilai. Sinkron dengan F2.

**Kriteria selesai.** Compiler menolak nilai liar masuk domain; test table idem F2.

#### Bukti penutupan F9 (2026-09-22)

`UsageFilter.Status`, `UsageFilterInput.Status`, dan `UsageFilterQuery.Status` (schema) kini
bertipe `domain.UsageStatus`; `LogFilter.Status` dan `LogFilterInput.Status` kini bertipe
`domain.RequestLogStatus`. Compiler kini menolak `filter.Status = "banana"` pada jalur non-HTTP
(nilai mentah harus melalui `UsageStatus("banana")` eksplisit, yang `Validate` tolak, atau
`ParseUsageStatus`, yang juga menolak). `repository/postgres` `filterArgs`/`logFilterArgs`
menjalankan nilai typed itu sebagai argumen query yang sama (`$n = '' OR status = $n` tetap
parameterized; encode pgx tidak berubah, dibuktikan suite integrasi penuh). Test table idem F2
(`internal/domain/usage_types_status_test.go`, dua filter membagikan satu tabel kasus) plus
regresi rentang terbalik tetap hijau.

## 6. Non-findings dan batasan yang dipertahankan (app-serv)

Hal berikut sudah sesuai atau sengaja bukan gap:

1. **Session gate:** keempat route Usage ter-sweep oleh `TestEveryManagementRouteRejectsAnonymousCallers`;
   tidak perlu exclusion karena memang management routes.
2. **SQL injection:** semua filter parameterized dengan pola `($n = '' OR col = $n)`; tidak ada
   string caller yang masuk query. `date_bin` interval datang dari closed set, bukan input.
3. **Unbounded query:** `NewUsageFilter` selalu mengisi default window 24 jam bila caller tanpa
   range; `Timeseries`/`List`/`Summary` selalu membawa `ts >= $1 AND ts <= $2`. `List` paginated
   dengan LIMIT/OFFSET. `usage_records` ter-partisi per bulan dengan index `ts`, `(provider_id, ts)`,
   `(endpoint_id, ts)`.
4. **N+1:** keempat read adalah satu statement set-based; `Detail` melakukan 2 query (usage + log)
   untuk 1 request id, bukan loop.
5. **Error envelope:** semua error lewat `schema.WriteError` -> management envelope English;
   `writeDataPlaneError` tidak dipakai di route ini.
6. **Cost sebagai string desimal:** `sum(cost_usd)::numeric(20,8)::text`, tidak pernah float.
7. **`ErrorRate` nol-request:** mengembalikan `"0.0000"`, bukan division by zero.
8. **Line limit:** semua file Go terkait Usage di bawah 250 (terbesar `domain/usage.go` 228, sudah
   di warning band 220 tapi bukan pelanggaran hard cap; dicatat, bukan finding).
9. **Sweep test rot:** `excludedFromSessionSweep` gagal bila ada entry basi, menjaga tabel audit.
10. **Zod v4.6.5 / TS 6.0.3 / Svelte 5.57 / Bun 1.3 / Vitest 5.0.1 di app-ui** sesuai mandatory stack.

## 7. Urutan pengerjaan yang disarankan (app-serv)

Jika owner memilih semua:

1. **F5** (Flusher) lebih dulu: CLOSED 2026-09-22.
2. **F2 + F9** (status closed set): CLOSED 2026-09-22.
3. **F1** (handler test): CLOSED 2026-09-22 (dikerjakan setelah F2/F9, sesuai urutan ini).
4. **F7 + F6** (kontrak): CLOSED 2026-09-22 (YAML di-edit satu pass, regenerasi, gate).
5. **F3, F8** (semantik `latency_ms`, scope `q`): CLOSED 2026-09-22 (D1 = c dan D4 = perluas;
   sum dikunci test dan ditulis di kontrak, `q` diperluas di usage dan logs).
6. **F4** (domain event): keputusan publish-vs-hapus (D2 = publish tercatat), jangan setengah.

Setiap task: analysis dulu (TDD §2.3 / OWASP §2.3 bila menyentuh security behavior), failing test
table-driven, header AGENTS.md, cek line count, compliance self-check.

## 8. Gate dan evidence setelah perbaikan

```bash
cd app-serv
go test -race -count=1 ./...
go vet ./...
gofmt -l .
go run ./tools/openapi-gen -check
cd ..
bash scrypts/gates/go-headers.sh
bash scrypts/gates/go-lint.sh
bash scrypts/gates/contract-drift.sh
bash scrypts/gates/contract-openapi.sh
cd app-ui && bun run check && bun run test
```

Untuk live evidence Usage: boot app-serv + Postgres + Redis, tulis dua usage row lewat
data-plane call, lalu baca keempat route dengan session cookie dan bandingkan angkanya; catat
request id, status, dan hitungan baris saja (tanpa DSN/credential/body).

## 9. Ringkasan temuan

### 9.1 app-serv (BE)

| # | Temuan | Prioritas | Aplikasi |
|---|---|---|---|
| F1 | Empat route Usage tanpa HTTP handler test | HIGH | BE |
| F2 | Filter `status` menerima nilai liar; query diam-diam cocok nol baris | HIGH | BE |
| F3 | `latency_ms` aggregate = sum, semantik tak terdokumentasi | MEDIUM | BE (keputusan) |
| F4 | `usage.recorded` event tanpa publisher/subscriber | MEDIUM | BE (keputusan) || F5 | `http.Flusher` hilang di middleware chain; SSE tidak streaming | HIGH | BE |
| F6 | `per_page` klem diam vs tolak vs kontrak diam | MEDIUM | BE (keputusan) |
| F7 | YAML tanpa `format: date-time`/enum pada param Usage | LOW | BE |
| F8 | Scope `q` (model-only) vs janji placeholder panel | MEDIUM | BE+FE |
| F9 | `UsageFilter.Status` string, bukan `UsageStatus` | LOW | BE |

F5 **CLOSED 2026-09-22** (bukti di §5 F5). F2 + F9 **CLOSED 2026-09-22** (bukti di §5 F2 dan §5
F9: satu perubahan tipe + satu boundary check, test table sama, kontrak jadi enum). F1
**CLOSED 2026-09-22** (handler + route test untuk keempat route, tiga mutation check membuktikan
test menangkap regresi). F6 **CLOSED 2026-09-22** (D3 = tolak; decoder, kontrak, dan panel kini
sepakat 1..100 dengan default 25). F7 **CLOSED 2026-09-22** (sisa `from`/`to` date-time dan enum
`group_by`/`granularity`; `status` tertutup bersama F2). F3 **CLOSED 2026-09-22** (D1 = c:
semantik sum ditulis di YAML + SPEC-API §7.12, plus test yang membedakan sum dari mean). F8
**CLOSED 2026-09-22** (D4 = perluas: `q` mencakup id/request_id/error_code/model pada usage dan
request_id/error/model pada logs, kontrak menyatakan scope-nya, F11 tertutup sebagai konsekuensi).
F4 dan temuan cross audit (F10 sampai F14) tetap OPEN.

### 9.2 Cross audit Usage vs Node Animation SSE

| # | Temuan | Prioritas | Aplikasi |
|---|---|---|---|
| F10 | Tidak ada live/SSE pada Usage; parity 9router = desain dua sisi | MEDIUM (KEPUTUSAN) | BE+FE |
| F11 | Placeholder Search menjanjikan scope yang tidak dilayani | LOW | FE (terkait F8) || F12 | `per_page` URL diabaikan diam-diam tanpa di-reset | LOW | FE (terkait F6) |
| F13 | Opsi live tanpa SSE (polling) tidak dievaluasi spec-nya | MEDIUM (KEPUTUSAN) | FE+spec |
| F14 | Live evidence keempat route Usage belum direkam | LOW | BE |

## 10. Cross audit: Endpoint Usage (app-serv) vs Node Animation SSE (app-ui)

Permintaan owner: "Cross Audit tambahan: Endpoint Usage (app-serv) >> Node Animation SSE
(app-ui)". Audit ini membandingkan tiga hal: (a) dukungan **node live / SSE** pada endpoint Usage,
(b) apa yang referensi 9router punya di area yang sama, (c) apa yang harus dikerjakan di kedua
aplikasi bila parity diinginkan.

### 10.1 Apa yang ada hari ini

**app-serv (BE):** tidak ada route SSE di Usage dan tidak ada route SSE manajemen lain. SSE hanya
ada di data plane (`POST /chat/completions`, `POST /messages`, keduanya `200 JSON atau SSE` di
kontrak). `openapi.json` audit: hanya dua path yang mendeklarasikan `text/event-stream`. Management
API sepenuhnya request/response. Tidak ada Redis Pub/Sub. Tidak ada representasi request in-flight /
active di mana pun: `usage_records` ditulis **setelah** request selesai (`chat_record.go` record
pada akhir relay), jadi agregat Usage adalah histories, bukan keadaan hidup.

**app-ui (FE):** layar `/usage` sepenuhnya statis per-load: dua tab, `RefreshControl` manual
(`RefreshControl.svelte`: tombol "Refresh now", tanpa timer), tanpa polling (berbeda dari `/quota`
dan console yang punya `pollDue`/`QUOTA_POLL_MS`), tanpa `EventSource` (satu-satunya SSE reader
panel adalah `playground-reader.ts` untuk jawaban chat). Tidak ada komponen topologi/node; tidak
ada dependensi graph (`@xyflow/react` dsb. tidak ada di `package.json`).

**Referensi 9router (`decolua/9router`, Next.js monolith):** punya keduanya:

- `GET /api/usage/stream` (`src/app/api/usage/stream/route.js`): ReadableStream SSE, dua listener
  emitter (`update` full-recompute + `pending` ringan), keepalive `: ping` tiap 25 dtk, `Cache-Control:
  no-cache`, `Connection: keep-alive`. Client `UsageStats.js` membuka `new
  EventSource("/api/usage/stream")` dan **merge hanya field real-time** (`activeRequests`,
  `recentRequests`, `errorProvider`, `pending`) di atas stats REST per periode (`/api/usage/stats?
  period=`), desain hybrid REST+SSE yang jelas perpisahan tanggung jawabnya.
- `ProviderTopology.js`: ReactFlow dengan `RouterNode` pusat + `ProviderNode` per provider di
  elips; node aktif ditandai `animate-ping` dot + border/glow warna provider, **edge dianimasikan**
  (`animated: active`) saat provider punya request in-flight, edge error merah, edge "last" kuning,
  plus **FE timeout guard 60 dtk** untuk provider yang stuck aktif (BE yang macet tidak mengunci
  animasi FE selamanya).
- Sumber "active": `trackPendingRequest()` di `usageRepo.js` mengelola `global._pendingRequests`
  (byModel/byAccount) di memori proses, di-emit `statsEmitter` pada setiap start/end, ring 50 item
  untuk recent, TTL 60 dtk untuk pending yang bocor. Karena monolith satu proses, memori + emitter
  cukup; tidak ada persistence untuk active state.

### 10.2 Penilaian kesesuaian dengan spec yang berlaku

Fakta penting: **status quo app-ui/app-serv saat ini bukan pelanggaran spec, melainkan hasil
keputusan spec**. SPEC-UI §9.5 (tabel locked decision): "Polling instead of a live stream:
Management API v1 exposes no stream; labelling a poll 'Live' would be a false claim." §8.6.1:
"`/logs` console dan `/quota` memakai polling karena management API v1 tidak punya stream
(SPEC-API §7.13)." Layar Usage bahkan tidak memakai polling karena tidak ada yang bergerak sendiri
(pembandingnya: quota window berjalan tanpa penonton). Antislop R-36/R-17: tidak mengecat tombol
"Live" di atas polling adalah keputusan jujur yang harus dipertahankan sampai benar-benar ada
stream.

Jadi gap Node Animation SSE **bukan "implementasi tertinggal dari spec"**, melainkan **perbedaan
fitur terhadap referensi** yang menuntut keputusan owner untuk mengubah spec (SPEC-API §7.12 +
SPEC-UI §6.5/§9.5/§8.6) sebelum satu baris kode ditulis. Itu sebabnya temuan di bawah ditandai
sebagai keputusan, bukan defect.

### 10.3 Temuan cross audit

### F10 MEDIUM (KEPUTUSAN): Tidak ada dukungan live/SSE pada Usage di kedua aplikasi; parity 9router menuntut desain baru dua sisi

**Status: OPEN (keputusan owner).**

**Fakta.** Digunakan pada `§10.1`: tanpa route SSE management, tanpa state in-flight, tanpa
`EventSource` di `/usage`, tanpa komponen topologi. Referensi punya REST hybrid + SSE + topology
node animasi. Spesifikasi saat ini mengunci kebalikan dari referensi (§9.5). F5 juga berlaku:
**seluruh SSE app-serv hari ini tidak benar-benar streaming per-frame**, jadi membangun SSE Usage
sebelum F5 selesai akan mewarisi defect yang sama.

**Risiko dan aturan.**

- SPEC-API §7.12 tidak menjanjikan stream: menambah route SSE tanpa amandemen spec = pelanggaran
  CDD (AGENTS.md §2.4) dan membuat kontrak berbohong.
- SPEC-UI §9.5/§8.6.1 eksplisit melarang klaim live tanpa stream (R-36).
- Sebaliknya, membiarkan tanpa keputusan membuat referensi parity hanya legenda: §15 SPEC-UI
  menyebut referensi sebagai "Source Rewrite (Reference)".

**Rencana DURING bila owner memilih parity (BE lalu FE).**

BE (app-serv), urutan:

1. Selesaikan **F5** lebih dulu (Flusher) tanpa kecuali.
2. Tentukan sumber "active". Referensi pakai memori proses; app-serv multi-request per proses tapi
   punya Redis. Desain yang konsisten dengan arsitektur: penanda in-flight di Redis (mis.
   `pannelai:active:<request_id>` hash berisi `{provider_id, endpoint_id, model, started_at}` dengan
   **TTL 60 dtk** yang sama dengan guard FE referensi, ditulis pada satu choke point sebelum relay
   dan dihapus setelahnya; itu menggantikan `trackPendingRequest` + timeout memori referensi dengan
   TTL, sehingga BE mati tidak meninggalkan node aktif bohong). Atau publish `usage.recorded` +
   `usage.started` lewat Redis Pub/Sub dan biarkan handler SSE menyimpan ring di memori seperti
   referensi. Keduanya sah; Redis TTL lebih tahan multi-proses, Pub/Sub lebih dekat ke F4.
3. Route: `GET /api/v1/usage/live` (session-gated, `text/event-stream`, `X-Accel-Buffering: no`,
   keepalive `: ping` <= 25 dtk, frame `data: {"active":[...], "recent":[...],
   "error_provider":"..."}`). Setiap frame dibangun dari sumber bounded (max 20 recent seperti
   referensi), bukan dari scan tabel.
4. Mid-life kejujuran: frame pertama dikirim segera (state sekarang), lalu hanya saat berubah atau
   keepalive. Status 200 + `text/event-stream` ter-commit **setelah** frame pertama benar-benar
   ditulis (pola `sseSink` F3 draft 009, yang benar; masalahnya cuma Flush-nya).
5. Wiring: handler baru + router + wiring + OpenAPI YAML (`text/event-stream` response) +
   regenerasi + gate. Update SPEC-API §7.12 changelog dan SYSTEM_MAP.md (async topology baru,
   AGENTS.md §1.9).
6. Goroutine subscriber: panic recovery + context cancellation + wait-group; satu goroutine per
   koneksi dengan termination condition eksplisit (client disconnect / ctx done / max lifetime);
   AGENTS.md §1.6 penuh.

FE (app-ui), hanya setelah BE live:

1. Schema Zod untuk frame (`active`, `recent`, `errorProvider`), satu modul `usage-live.ts` dengan
   `EventSource` + reconnect policy + stop saat tab hidden (menggeneralisasi `pollDue` decision
   yang sudah teruji, `§8.6.1`, ke medium stream).
2. Hybrid seperti referensi: totals/chart tetap dari REST per periode; **hanya** field real-time
   (`active`, `recent`, `errorProvider`) yang datang dari SSE dan di-merge, bukan menimpa, stats
   penuh. Ini mempertahankan "panel computes nothing it cannot cite" (§6.5).
3. Komponen topologi: evaluasi dulu **tanpa** dependensi graph. DESIGN.md dial MOTION 1 ("hover,
   focus, state transition only; no loop, no scroll choreography") dan ENERGY 1; `animate-ping`
   berdenyut terus termasuk motion loop. Alternatif sesuai dial: indikator aktif statis (dot warna
   + label "routing") dengan **transisi state** saat mulai/selesai, bukan denyut abadi. Bila owner
   memang ingin denyut ala referensi, itu amandemen DESIGN.md MOTION dengan alasan tertulis
   (R-19: purpose motion = "menunjukkan request sedang berjalan"), satu baris, bukan default.
4. Baris/node hanya untuk provider yang **benar-benar punya endpoint terkonfigurasi** (dari
   `/providers`), bukan dari registry 94 provider (terhitung `grep -c "^  - id:" atas
   `internal/registry/registry.yaml`, sama dengan angka SYSTEM_MAP.md): referensi juga hanya
   menampilkan node dari koneksi aktif + provider no-auth, bukan seluruh katalog, jadi aturan ini
   parity sekaligus menghindari 94 node yang tak pernah tersentuh. Data baris dari
   `GET /api/v1/providers` + `summary?group_by=provider`, state aktif dari SSE.
5. Guard FE 60 dtk referensi **tidak perlu digandakan bila BE memakai Redis TTL** (satu pemilik
   kebenaran); bila BE memilih memori murni, guard FE wajib.
6. Test: reader (parse frame, reconnect, truncation), merge (SSE tidak menimpa REST), dan render
   (aktif/nonaktif/error state) table-driven; klik-through tercatat per R-35.

**Kriteria selesai (bila dipilih).** SPEC-API §7.12 + SPEC-UI §6.5/§9.5/§8.6 diamandemen dengan
alasan; frame pertama terukur sampai klien (test permanen, bukan klaim); panel menampilkan state
aktif yang benar dalam live pass; tidak ada klaim "Live" tanpa stream dan tidak ada stream tanpa
label; `bun run check`/`bun run test`/gate Go hijau; SYSTEM_MAP.md diperbarui.

### F11 LOW (FE): Placeholder Search Usage & Logs menjanjikan scope yang tidak dilayani

**Status: CLOSED 2026-09-22 (tertutup sebagai konsekuensi F8, keputusan owner D4 = perluas).**

**Bukti penutupan.** Keputusan D4 memperluas BE sampai memenuhi janji placeholder, jadi sisi FE
memang **tidak perlu berubah**: `UsageRecordsFilters.svelte` dan `LogsFilters.svelte` tetap menulis
`"Request id, error code"`, dan sekarang teks itu benar karena `q` benar-benar mencocokkan request id
(dan `id`, `error_code`, `model`). Ini kebalikan dari rencana DURING pilihan (a), yang akan menuntut
placeholder diganti menjadi `Search model`; karena owner memilih (b), yang berubah adalah BE dan
kontrak, bukan label panel. Tidak ada test FE baru karena tidak ada perilaku FE baru: yang diuji
adalah predikat SQL-nya (`usage_query_scope_integration_test.go`,
`log_query_scope_integration_test.go`) dan scope di kontrak tersaji
(`TestOpenAPIContract_FreeTextScopeIsStated`).

**Fakta.** `UsageRecordsFilters.svelte` dan `LogsFilters.svelte` menulis placeholder
`"Request id, error code"`; implementasi BE memfilter `q` hanya terhadap `model ILIKE`. Operator
yang mencari request id mendapat empty state "No requests match these filters" padahal barisnya ada.

**Risiko dan aturan.** R-26 (kontrol yang tidak melakukan apa labelnya), SPEC-UI §7.1.1 (koreksi
harus terlihat). Ini temuan FE-nya dari gap yang sama dengan F8 BE.

**Rencana DURING (FE, setelah keputusan F8).** Bila F8 pilihan (a): placeholder diganti
`Search model` di dua file + test string; bila (b): tidak ada perubahan FE selain test yang memaku
perilaku baru.

**Kriteria selesai.** Placeholder dan perilaku API menyebut scope yang sama, dipaku test.

### F12 LOW (FE): `usage-search.ts` mengabaikan `per_page` URL secara diam-diam tanpa me-resetnya

**Status: OPEN. Terhubung F6.**

**Fakta.** `parseUsageSearch` menampilkan notice "per_page was ignored" tapi URL tetap membawa
nilai lama; kombinasi dengan klem diam BE (F6) berarti tiga perilaku berbeda untuk satu parameter.

**Rencana DURING (FE).** Setelah F6 diputuskan: bila BE menolak, panel cukup memvalidasi
`perPage` Zod di URL dan notice mengarah ke "reset ke 25" dengan `goto` tanpa param, atau biarkan
notice dengan URL yang dibersihkan. Kecil, satu file + test.

### F13 MEDIUM (KEPUTUSAN): Dukungan "node live" tanpa SSE (opsi polling) tidak dievaluasi spec-nya

**Status: OPEN (keputusan owner).**

**Fakta.** Spektrum solusi live ada tiga: SSE penuh (F10), polling visible/pausable (pola `/quota`
sudah ada dan teruji: `pollDue`, `QUOTA_POLL_MS`, label interval, pause, stop-on-hidden), atau
tidak sama sekali (status quo, sesuai §9.5). Audit tidak menemukan evaluasi tertulis ketiganya
untuk Usage. Polling atas endpoint yang ada (`summary` + `records` halaman terakhir) bisa
menampilkan "recent requests" tanpa satu baris BE baru, tapi **tidak bisa** menampilkan state
aktif/in-flight (tidak ada sumbernya; `usage_records` hanya tertulis setelah selesai) dan tetap
melanggar §8.6.1 secara harfiah ("memakai polling karena tidak ada stream" hanya berlaku untuk
console/quota) kecuali §8.6.1 diamandemen. Topologi node dengan polling hanya bisa menampilkan
"baru saja selesai", bukan "sedang berjalan".

**Rencana DURING.** Owner memilih satu: status quo (tutup temuan dengan amandemen §6.5 menegaskan
Usage tidak live dan alasannya), polling (amandemen §8.6.1 + reuse `polling.ts`, FE only), atau SSE
(F10, dua aplikasi). Apa pun pilihannya harus tertulis di SPEC-UI, karena hari ini spec diam tentang
keinginan parity 9router padahal referensi eksplisit.

**Kriteria selesai.** Satu paragraf keputusan di SPEC-UI §6.5 atau §9.5 yang menyebut pilihan dan
alasannya; implementasi menyusul nomor terkait.

### F14 LOW (BE, catatan bukan defect): Live evidence keempat route Usage belum pernah direkam

**Status: OPEN.**

**Fakta.** Live pass panel yang ada (README) menyentuh usage rows sebagai **efek samping** pass
Playground (baseline `usage=2`), bukan sebagai pembacaan keempat route Usage dengan session nyata
dan verifikasi angka agregat vs baris. Integration test Postgres menyentuh repository, bukan HTTP.

**Rencana DURING (BE).** Satu live pass: dua request data-plane, lalu `GET summary/timeseries/
records/detail` dengan session; catat angka agregat cocok dengan penjumlahan manual dua baris;
catat error 401/400 juga. Format bukti mengikuti draft 009 §10.9.

**Kriteria selesai.** Tabel bukti live tercatat di dokumen penutupan (tanpa credential/DSN).

### 10.4 Ringkasan gap per aplikasi

**BE (app-serv) harus mengerjakan (bila owner memilih parity atau menutup temuan audit):**

- F1 handler test, F2+F9 status closed set, F5 Flusher forwarding, F7+F6+F3+F8 kontrak dan semantik
  (semuanya **CLOSED 2026-09-22**), F4 keputusan event, F14 live evidence.
- Bila parity live: state in-flight dengan Redis TTL atau Pub/Sub, route `GET /api/v1/usage/live`
  SSE, wiring, OpenAPI, amandemen SPEC-API §7.12, SYSTEM_MAP.

**FE (app-ui) harus mengerjakan:**

- F11 placeholder: **CLOSED 2026-09-22** sebagai konsekuensi F8 (D4 memperluas BE sampai memenuhi
  janji placeholder, jadi label panel tetap dan tidak ada pekerjaan FE). F12 per_page notice
  (tergantung F6, yang sudah CLOSED; sisa pekerjaan FE).
- Bila parity live: schema frame Zod, modul `usage-live.ts` (EventSource + reconnect +
  stop-on-hidden), merge field real-time tanpa menimpa stats REST, komponen topologi dengan
  keputusan MOTION eksplisit (denyut = amandemen DESIGN.md; transisi = sesuai dial), amandemen
  SPEC-UI §6.5/§8.6/§9.5, test reader/merge/render, click-through.

**Urutan yang tidak boleh dibalik:** F5 sebelum SSE apa pun; keputusan spec (F10/F13) sebelum kode;
BE live route sebelum FE EventSource (panel tidak boleh membangun reader untuk stream yang tidak
ada; itu dead control R-26).

## 11. Keputusan yang diminta owner

| # | Pertanyaan | Pilihan | Dipakai oleh |
|---|---|---|---|
| D1 | `latency_ms` aggregate: mean, hapus, atau dokumentasikan sebagai sum? | Dokumentasikan (c) | F3 |
| D2 | `usage.recorded`: publish via Redis Pub/Sub dengan subscriber nyata, atau hapus + amandemen §2.3? | Publish | F4 |
| D3 | `per_page > 100`: tolak 400 atau klem terdokumentasi? | Tolak | F6 |
| D4 | Scope `q`: tetap model-only (perbaiki placeholder FE), atau perluas ke request_id/error_code/model? | Perluas | F8, F11 |
| D5 | Node live / SSE Usage: status quo dengan amandemen spec, polling (FE only), atau SSE penuh (BE+FE, F10)? | Menunggu jawaban | F10, F13 |
| D6 | Bila SSE dipilih dan topologi node dibuat: indikator aktif sebagai transisi state (sesuai MOTION 1) atau denyut `animate-ping` (amandemen DESIGN.md)? | Transisi state | F10 FE |

D1 dan D4 sudah dipakai (F3 dan F8 **CLOSED 2026-09-22**); D2, D5, dan D6 masih menunggu jawaban.
Nomor F yang bukan keputusan (F1, F2, F5, F7, F9, F12, F14) dapat dipilih langsung tanpa D.

## 12. Catatan penutup

Audit ini tidak mengubah satu baris source produksi. Probe mekanis berupa file test sementara
dihapus; `git status app-serv` pada akhir audit hanya memuat perubahan actor lain (opencode,
forced-chat) yang sudah ada sebelum audit dimulai. Dokumen ini adalah DRAFT PLAN: nomor yang tidak
dipilih tidak dikerjakan, dan setiap nomor yang dipilih masuk mode DURING dengan gate penuh
(`§8`).
