# 005-QUOTA-TRACKER-CLOSURE.md: Penutupan Vertical Quota Tracker app-serv

Dokumen kerja hasil audit vertical Quota Tracker (SPEC-API-001 §7.12) pada 2026-09-20,
melanjutkan pola `003`/`004` (temuan bernomor F, owner memilih nomor yang dikerjakan).
Bukan kontrak; kontrak tetap `docs/SPEC-API/001-SPEC-API.md`.

| | |
|---|---|
| **Status** | F1 **CLOSED 2026-09-20** (Jalur A: implementasi). F2 **CLOSED 2026-09-20** (endpoint existence gate on write). F3 **CLOSED 2026-09-20** (split flush). F4 **CLOSED 2026-09-20** (per-endpoint batch identity). F5 **CLOSED 2026-09-20** (single-source validation). F6 **CLOSED 2026-09-20** (table-driven handler tests + sweep 401). F7 **CLOSED 2026-09-20** (single endpoint_id guard). F8 **CLOSED 2026-09-20** (em dash sweep). F9 **CLOSED 2026-09-20** (split touched over-limit tests). |
| **Dibuat** | 2026-09-20, dari pembacaan `app-serv/internal/**` (domain, repository, service, handler, schema, router), migrasi `000007`, dan eksekusi gerbang build/test |
| **Kaitan** | SPEC-API-001 §6 (schema), §7.12 (endpoint), §8 (kode error), §9 (non-fungsional), §10 (fase); AGENTS.md §1.1–§1.9, §2.1–§2.4; `docs/RULLES/TDD.md`, `docs/RULLES/OWASP.md`; lanjutan `004-ENDPOINT-READINESS-FOLLOW-UP.md` |
| **Lingkup** | hanya `app-serv/`. `app-ui/` di luar lingkup: file token-saver yang belum ter-track sedang dikerjakan agent lain, jangan disentuh |

## 1. Metode

Lima langkah, semua bisa diulang:

1. Penelusuran vertical per lapisan: `domain (quota.go, quota_cap.go)` → `repository (usage.go,
   postgres/quota.go, redis/quota_counter.go)` → `service (quota.go, quota_flush.go)` →
   `handler (quota.go)` → `schema (usage_record.go)` → `router.go:183-191`.
2. Pemetaan pemanggil produksi: `grep` setiap method kuota di seluruh `internal/` dan `cmd/`,
   lalu pisahkan hasil test dari hasil non-test.
3. Pemeriksaan header §1.2 dan jumlah baris §1.1 ke setiap file `.go` app-serv.
4. Eksekusi gerbang: `go build`, `go vet`, `go test -race ./internal/...`.
5. Pemetaan cakupan test: grep nama test kuota, dipetakan ke aturan TDD.md §2.5 (table-driven,
   3–5 variasi) dan AGENTS.md §2.1 (happy + validasi + auth per route).

## 2. Bukti yang sudah lolos

- `go build ./...`, `go vet ./...` bersih. `go test -race ./internal/domain/... ./internal/handler/... ./internal/service/... ./internal/repository/...` semua `ok`.
- Tujuh file header patuh AGENTS.md §1.2: `@file` cocok path relative, `@layer` benar, semua tag
  wajib ada. Tidak ada satu pun yang bolong.
- Aturan domain kaya dan terkapsul (AGENTS.md §2.2): `QuotaWindow` dan `QuotaCap` berfield
  unexported, mutasi hanya lewat `Add`/`Report`, konstruktor memvalidasi. Test domain
  table-driven sesuai TDD.md §2.5 (`TestQuotaWindow_Add`, `TestQuotaWindow_Exhausted`,
  `TestNewQuotaWindow`, `TestQuotaCap_Exhausted`, `TestNewQuotaCap`).
- Worker flush sudah memenuhi AGENTS.md §1.6 sepenuhnya: pulih panic di dua batas goroutine,
  kondisi terminasi eksplisit lewat context, kebijakan retry dan dead-letter ditulis di doc
  komentar, `atomic.Bool` sebagai guard flush tunggal, dan query set-based (`unnest`) alih-alih
  N+1 (§1.7).
- Ketiga route §7.12 dibungkus `gateway()` (`router.go:189-191`), jadi session-gated, dengan
  `RequireSession` dan deny-by-default saat auth tidak tersedia (`router.go:93-99`).
- Migrasi `000007` punya CHECK constraint untuk `window` dan `source`, index lengkap di
  `usage_records`, dan tidak ada query tak terbatas.

## 3. F1 (HIGH): Vertical kuota lengkap untuk baca, tidak untuk tulis dan tegakkan

**Fakta.** Tiga hal, satu akar:

1. `QuotaCounterStore.Add()` (`redis/quota_counter.go:63`) **nol pemanggil produksi**. Hanya
   `Pending` dan `Clear` yang dipanggil, keduanya oleh `QuotaFlusher`. Tidak ada satu pun kode
   yang menaikkan counter setelah request dilayani.
2. `QuotaCap.Exhausted()` (`domain/quota_cap.go:88`) dan `QuotaWindow.Exhausted()`
   (`domain/quota.go:193`) **nol pemanggil produksi**.
3. `UsageRecordRepository.MonthlyUsage()` (`repository/usage.go:54`) **nol pemanggil produksi**.

Akibatnya, key Redis `pannelai:quota:*` tidak pernah dibuat, sehingga `Pending` selalu
mengembalikan slice kosong dan `quota_windows` tidak pernah terisi. `GET /api/v1/quotas` di
produksi selalu menjawab `{"data":[]}`. Router juga tidak pernah melewati endpoint yang capnya
habis.

**Deviasi kontrak.** SPEC-API-001 §7.12 baris 452 menjanjikan "router stops picking exhausted
endpoints"; baris 454 menjanjikan "Quota worker re-checks windows per provider cadence"; §6 baris
137 menjanjikan "counters cached in Redis, flushed to PG"; SYSTEM_MAP §6 mencantumkan baris
"Quota re-check" sebagai worker P2. Keempat janji ini tidak punya pelaksana di kode.

**Risiko.** Ini bukan test yang kurang, tapi fitur yang tidak tersambung. Layar Usage panel
berfungsi (usage ditulis per request lewat `dataPlaneRecorder` dan `chat_record`), sedangkan
layar Quota di sebelahnya membaca tabel yang tidak pernah diisi. Breaking change berikutnya
pada kontrak kuota akan lolos tanpa ada yang menyadari sisi tulis belum ada.

**Dua jalur perbaikan, keduanya sah, owner memilih satu:**

- **Jalur A, implementasikan.** Sambungkan titik tulis di choke point yang sudah ada
  (`service/chat_record.go:83`, `service/dataplane_record.go:85`, `service/media_perform.go:73`),
  tambahkan pemeriksaan cap di pemilihan endpoint (`dataplane/selection.go`), lalu tutup dengan
  worker re-check §7.12. Ini pekerjaan lintas lapisan dan menyentuh permukaan P1/P2 yang sudah
  Closed, jadi butuh spec line sendiri sebelum kode (aturan spec-first §0).
- **Jalur B, turunkan kontrak.** Edit SPEC-API-001 §6, §7.12, dan SYSTEM_MAP §6 supaya menyatakan
  penegakan kuota dan re-check sebagai deferred, dengan alasan dan register yang sama seperti
  item deferred lain (pola G21 di `001-P2-GAPS.md`). Layar Quota tetap ada sebagai permukaan
  baca atas `quota_caps` yang sudah berfungsi.

**Kriteria selesai.** Tidak ada lagi janji di spec/SYSTEM_MAP tanpa pelaksana. Kalau Jalur A:
satu test bernama membuktikan endpoint ber-cap habis dilewati router, dan satu test membuktikan
`Add` dipanggil setelah request dilayani. Kalau Jalur B: §6/§7.12/SYSTEM_MAP §6 menyebut deferred
secara eksplisit beserta alasannya.

**Catatan urutan.** F3 (endpoint existence) sebaiknya dikerjakan bersama F1 Jalur A, karena
keduanya menyentuh validasi endpoint yang sama.

## 4. F2 (HIGH): `SetCap` tidak memverifikasi endpoint, dan komentarnya mengklaim sebaliknya

**Fakta.** Doc komentar `service/quota.go:61-63` menyatakan: "the endpoint is required to exist
first: a cap on an endpoint that is not configured is a typo the caller should see rather than a
row nothing reads." Tidak ada pengecekan seperti itu di `SetCap` (`service/quota.go:64-76`), di
`postgres.QuotaRepository.SetCap` (`postgres/quota.go:164`), maupun di handler. `quota_caps`
juga tidak punya foreign key ke `upstream_endpoints` (migrasi `000007`, diverifikasi: nol
`REFERENCES`), jadi database pun tidak menolak.

**Risiko.** `PUT /api/v1/quotas/ep_tidak_ada` menjawab 200 dan menulis baris yang tidak akan
pernah dibaca router. Komentar yang mengklaim invariant tidak ada lebih berbahaya daripada tidak
ada komentar: reviewer berikutnya akan mempercayainya. Melanggar AGENTS.md §1.3 (dokumen jujur).

**Kerja yang diusulkan.** Pilih satu: (a) tambahkan pengecekan eksistensi endpoint di service
lewat seam yang sudah ada, dengan `NOT_FOUND` bila tidak ada; atau (b) tambahkan FK
`quota_caps.endpoint_id → upstream_endpoints(id) ON DELETE CASCADE` di migrasi baru, dan
terjemahkan pelanggaran FK ke `NOT_FOUND`. Opsi (a) memberi pesan yang lebih baik; opsi (b)
menutup jalur tulis lain sekaligus. Salah satu, bukan keduanya.

**Kriteria selesai.** Test bernama: cap untuk endpoint tak dikenal = `NOT_FOUND`; cap untuk
endpoint sah = 200. Komentar dan perilaku sepakat.

## 5. F3 (HIGH): `quota_flush.go` 255 baris, lewat hard cap 250

**Fakta.** `internal/service/quota_flush.go` = 255 baris. AGENTS.md §1.1: "CI gate fails if any
modified source file exceeds 250 lines", dengan pengecualian hanya untuk generated code. Ini
sudah tercatat sebagai bagian F5 di `004-ENDPOINT-READINESS-FOLLOW-UP.md`; nomor ini tidak
menduplikasi, hanya mengangkatnya sebagai bagian penutupan vertical ini karena file-nya milik
kuota.

**Kerja yang diusulkan.** Pecah sesuai concerns yang sudah terlihat: blok doc kebijakan
(retry/dead-letter) dan tipe `QuotaFlushPolicy` + `DefaultQuotaFlushPolicy` ke satu file; worker
lifecycle (`Run`, `FlushOnce`, `flushOnce`, `StartQuotaFlush`) tetap; `drain` + `batchIdentity`
ke file ketiga. Tanpa perubahan perilaku.

**Kriteria selesai.** Ketiga file di bawah 220 (ambang peringatan), `go test -race` hijau tanpa
perubahan asersi.

## 6. F4 (MEDIUM): `batchIdentity` di-key ke window kind, komentarnya bilang endpoint id

**Fakta.** `quota_flush.go:232-238` mengumpulkan `window.Window()` (jenis window, mis. `daily`)
ke dalam slice bernama `seen`, dan komentarnya menyebut nilainya "endpoint ids". Komentar di
`QuotaFlushPolicy` (baris 68) dan di jalur dead-letter (baris 209-211) juga menyebut "endpoint
ids". Akibatnya dua batch gagal berbeda dengan bentuk yang sama dihitung sebagai batch yang sama,
sehingga counter `MaxAttempts` bisa habis untuk batch yang bukan penyebabnya.

**Kerja yang diusulkan.** Ubah `batchIdentity` untuk memakai `window.EndpointID()` (dan kind
sebagai pelengkap), atau perbaiki komentar bila key saat ini memang disengaja. Test: dua batch
dengan endpoint berbeda tapi kind sama tidak saling mewarisi hitungan percobaan.

**Kriteria selesai.** Komentar dan perilaku sepakat; ada test yang mengunci perilaku
per-endpoint.

## 7. F5 (MEDIUM): Validasi `QuotaCapRequest` tidak lengkap dan duplikatif

**Fakta.** `schema/usage_record.go:102-105`: `MonthlyTokens` punya `validate:"omitempty,min=0"`,
`MonthlyCostUSD` tidak punya tag struct sama sekali. `ValidateQuotaCap` (baris 110-124)
memeriksa parseable dan non-negatif. Tapi:

- Tidak ada batas atas. `monthly_tokens: 9223372036854775807` atau cost 400 digit diterima dan
  disimpan sebagai `numeric(20,8)` (yang akan gagal di driver, bukan di validasi).
- `ValidateQuotaCap` tidak menegakkan aturan "cost nol tanpa token cap ditolak" yang ada di
  `domain.NewQuotaCap` (baris 46-51). Dua validator dengan aturan berbeda, jadi error yang
  dilihat klien bergantung mana yang jalan lebih dulu. Melanggar AGENTS.md §1.4 (satu batas
  tervalidasi).

**Kerja yang diusulkan.** Satu sumber aturan: pindahkan seluruh pemeriksaan ke `ValidateQuotaCap`
lengkap dengan batas atas, dan biarkan `NewQuotaCap` sebagai penjaga terakhir yang konsisten
dengannya. Tambahkan batas atas eksplisit yang masuk akal untuk kolom target.

**Kriteria selesai.** Test table-driven minimal 5 kasus (valid, nol, negatif, batas atas, di atas
batas), dan kedua validator tidak bisa lagi berbeda pendapat.

## 8. F6 (MEDIUM): Test handler `quota` bukan table-driven, tanpa kasus auth

**Fakta.** Empat test handler (`handler/quota_test.go`): `PutThenGetReadsTheCapBack`,
`GetWithoutACapAnswersNull`, `ListStaysWindowsOnly`, `PutCapRejectsANegativeCost` — masing-masing
satu skenario. TDD.md §2.5 mewajibkan setiap test memakai tabel 3–5 variasi (nilai tipikal,
batas, nol/kosong, negatif, ekstrem). AGENTS.md §2.1 mewajibkan setiap route terproteksi punya
test happy + validasi + auth; test auth belum ada untuk ketiga route §7.12 (dicatat juga sebagai
bagian F4 di `004`).

**Kerja yang diusulkan.** Ubah keempat test ke bentuk tabel dan tambahkan kasus auth untuk
ketiga route. Test domain kuota sudah patuh, tidak perlu disentuh.

**Kriteria selesai.** Semua test kuota di lapisan handler dan service berbentuk tabel ≥3 kasus;
ketiga route punya asersi 401 tanpa session.

## 9. F7 (LOW): Guard `endpoint_id == ""` tiga kali untuk satu nilai

**Fakta.** `handler/quota.go:49-52` dan `:78-81` masing-masing memeriksa `endpointID == ""`,
lalu `service/quota.go:65-67` memeriksa ketiga kalinya. Go 1.22 ServeMux hanya menjalankan handler
untuk pattern yang cocok, jadi `{endpoint_id}` tidak akan pernah kosong lewat route nyata; cabang
ini mati. AGENTS.md §1.1 (mixed concerns) dan §1.4 (validasi di satu tempat).

**Kerja yang diusulkan.** Sisakan satu pemeriksaan di batas yang benar (service sebagai penjaga
domain), hapus duplikat di handler. Test handler yang memanggil langsung tanpa path value tetap
hijau.

**Kriteria selesai.** Satu pemeriksaan `endpoint_id`, bukan tiga.

## 10. F8 (LOW): Em dash di SPEC-API-001 (antislop R-02)

**Fakta.** Karakter em dash (`—`) tersebar di `docs/SPEC-API/001-SPEC-API.md`, termasuk baris
§7.12 yang diaudit (baris 452, 455) dan seluruh blok changelog. Carve-out R-02 hanya mencakup
struktur dokumentasi skill antislop itu sendiri, bukan file spec proyek.

**Kerja yang diusulkan.** Ganti em dash dengan koma, titik dua, titik, atau tanda kurung. Sapuan
teks murni, tanpa perubahan kontrak.

**Kriteria selesai.** Nol em dash di `docs/SPEC-API/001-SPEC-API.md`.

## 11. F9 (LOW): File test di atas batas §1.1

**Fakta.** `internal/service/quota_flusher_test.go` 300 baris, `internal/domain/quota_test.go`
247 baris. AGENTS.md §1.1 berlaku untuk semua `.go` di bawah `internal/**`; pengecualian hanya
generated code. Tercatat juga sebagai F5 `004`.

**Kerja yang diusulkan.** Pecah dua file test ini bila disentuh oleh F4/F5/F6; jangan jadikan
pekerjaan tersendiri. Bukan bloker.

**Kriteria selesai.** Kedua file di bawah 250 saat berikutnya disentuh.

## 12. Urutan pengerjaan yang disarankan

1. **F1** dulu, karena menentukan apakah F2/F3 dikerjakan sebagai bagian jalur A atau jalur B.
2. **F3** (endpoint existence) bersama F1 Jalur A.
3. **F5** (satu validator) dan **F4** (`batchIdentity`), keduanya perubahan kecil dan terlokalisasi.
4. **F2** (pecah `quota_flush.go`), tanpa perubahan perilaku.
5. **F6** (table-driven + auth) menutup cakupan test vertical ini.
6. **F7**, **F8**, **F9** kapan saja, sapuan kecil.

Setiap PR yang mengerjakan nomor di atas wajib menyatakan di deskripsi apakah `SYSTEM_MAP.md`
dan spec ikut berubah (AGENTS.md §1.9). Bila pengerjaan menemukan deviasi kontrak, spec diedit di
PR yang sama (aturan spec-first §0 SPEC-API-001). Bila F1 diambil sebagai Jalur B, edit spec dan
`SYSTEM_MAP.md §6` wajib di PR yang sama dengan perubahan kodenya.
