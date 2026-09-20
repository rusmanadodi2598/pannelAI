# 004-ENDPOINT-READINESS-FOLLOW-UP.md: Lanjutan Audit Kesiapan Endpoint app-serv (Pasca-P4)

Dokumen kerja hasil pemeriksaan ulang kesiapan endpoint pada 2026-09-20, setelah P4
dibangun dan SPEC-API-001 berstatus Closed. Bukan kontrak; kontrak tetap
`docs/SPEC-API/001-SPEC-API.md`. Ini adalah daftar item kerja yang bisa dikerjakan
berikutnya, melanjutkan pola `003-ENDPOINT-READINESS.md` (temuan bernomor F, owner
memilih nomor yang dikerjakan).

| | |
|---|---|
| **Status** | draft siap dikerjakan; belum ada nomor yang disetujui owner |
| **Dibuat** | 2026-09-20, dari `app-serv/internal/router/*.go`, `internal/service/*.go`, dan eksekusi gerbang build/test |
| **Kaitan** | SPEC-API-001 §7.1 sampai §7.18, §9 (non-fungsional), AGENTS.md §1.1, §2.1; lanjutan `003-ENDPOINT-READINESS.md` |
| **Lingkup** | hanya `app-serv/`. `app-ui/` di luar lingkup dokumen ini: file token-saver yang belum ter-track sedang dikerjakan AI agent lain, jangan disentuh |

## 1. Metode

Empat langkah, semua bisa diulang:

1. Diff mekanis verb+path: katalog spec §7.1 sampai §7.18 diekstrak dari tabel (93 baris,
   baris `GET / PUT` dipecah dua, baris rujukan `audio/*` di §7.15 dan duplikat
   `POST /embeddings` dibuang) dibandingkan dengan 93 registrasi di `router.go`,
   `router_dataplane.go`, `router_media.go`. `comm` dua arah kosong: 93 = 93, nol drift.
2. Gerbang dieksekusi langsung: `go build`, `go vet`, `go test -race ./...` (13 package
   hijau), `gofmt -l` bersih, `staticcheck ./...` bersih (staticcheck dipasang dulu di
   mesin ini karena belum ada; gerbang `go-lint.sh` memang menganggap ketiadaannya
   kegagalan, sesuai desain).
3. Kepatuhan AGENTS.md: header §1.2 diperiksa ke setiap file `.go` app-serv (tidak ada
   yang bolong); jumlah baris §1.1 dihitung ke semua file non-generated.
4. Pemetaan cakupan test per area: grep nama service, nama method, dan path di semua
   `*_test.go` (service, handler, router, domain, repository, cmd).

## 2. Bukti kesiapan yang sudah lolos

- Paritas route 93 = 93, termasuk tiga route P4 (`/skills`, `/openapi.json`, `/changelog`).
- Kontrak mesin-baca terkunci: `TestOpenAPICoversEveryRegisteredRoute`
  (`router_p4_routes_test.go:112`) membandingkan route terdaftar dengan dokumen
  `openapi.json` yang dilayani, dua arah.
- Area yang sudah CLOSED dengan perjalanan test lengkap (dari 003, dikonfirmasi masih ada):
  gateway-keys §7.3, combos + vision §7.7/§7.8, models §7.6
  (`TestManagementRoutesRequireSession`), token-saver §7.9
  (`TestTokenSaverRoutes_SessionGated`), media §7.10, proxies §7.11, P4 §7.16-§7.18,
  OAuth §7.4 (callback/status/refresh), data plane §7.15 (auth chat, count_tokens,
  embeddings di level service).
- Working tree `app-serv` bersih; tidak ada perubahan belum ter-commit.

## 3. F1 (HIGH): Empat route baca Usage §7.12 tanpa test di semua lapisan

**Fakta.** `UsageService` (`internal/service/usage.go`: `Summary`, `Timeseries`,
`Records`, `Detail`) tidak direferensikan satu pun file test. Tidak ada test handler
untuk `handler/usage.go`, tidak ada test router yang menyentuh `/api/v1/usage`.
Yang sudah teruji hanya seam perekaman (`stubUsageRecorder` di `chat_record_test.go`,
`media_record_test.go`) dan level repository (`usage_integration_test.go`,
`usage_read_integration_test.go`).

**Risiko.** Logika filter (`from`/`to`), group-by, paginasi, dan join detail
(`RecordDetail` di `usage.go:105`) berjalan di produksi tanpa satu pun asersi.
Melanggar AGENTS §2.1: logika `service` baru wajib ditemani test.

**Kerja yang diusulkan.** Test table-driven di level service untuk keempat method
(filter wajib valid, group_by di luar enum ditolak, paginasi `page`/`per_page` dengan
batas 100, `Detail` menggabungkan log saat capture aktif dan tetap menjawab saat tidak),
plus satu test mux happy-path per route dengan fixture yang sudah ada.

**Kriteria selesai.** Setiap method `UsageService` punya kasus happy dan kasus
validasi; keempat route `GET /usage/*` menjawab 200 lewat mux nyata dengan session;
`go test -race ./internal/service/ ./internal/router/` hijau.

## 4. F2 (HIGH): `GET/PATCH /api/v1/settings` nyaris tanpa test

**Fakta.** `SettingsService.Update` (`internal/service/settings.go:90`, termasuk
`touchedGroups` dan `decodeOr`) tidak dipanggil test mana pun. Tidak ada test handler
untuk `handler/settings.go`, tidak ada test router yang menyentuh path settings.
Domain `SettingsPatch` sudah tervalidasi di `domain/settings_test.go` dan
`settings_caveman_test.go`, jadi yang kosong adalah jalur merge service dan route-nya.

**Risiko.** PATCH adalah satu-satunya pintu perubahan konfigurasi gateway; aturan
"partial update, validated per key" (§7.14) belum terbukti di kabel. Gerbang session
route ini juga belum teruji di mux (pembungkus `gateway()` ada di `router.go:197-198`,
diperiksa manual).

**Kerja yang diusulkan.** Test service: Update mengubah hanya group yang disentuh,
group lain utuh; nilai tersimpan rusak jatuh ke default (`decodeOr`); round-trip
GET = PATCH. Test mux: 401 tanpa session untuk kedua verb, 200 dengan session
(benign control), PATCH payload tidak valid = 400 `VALIDATION_ERROR`.

**Kriteria selesai.** `TestSettingsRoutes_SessionGated` dan test round-trip lewat mux
nyata; rahasia tidak pernah muncul di jawaban GET (§7.14: "secrets never returned").

## 5. F3 (MEDIUM): §7.4 Provider dan provider-nodes: lapisan service tak teruji

**Fakta.** `ProviderService.List/Detail/Models` (`internal/service/provider.go`:
routability, status summary, filter `?category=`/`?routability=`) dan `NodeService`
(`internal/service/provider_node.go`: `Create`, `Update`, `Delete`, `List`) nol
referensi test. Yang sudah teruji: aturan domain node (`domain/provider_node_test.go`),
probe node (`TestProbeNode_*` di `cmd/app-serv/provider_probe_test.go`), dan runtime
index custom node (`provider_index_test.go`).

**Risiko.** Semantik `CONFLICT` §7.4 justru hidup di service dan belum terbukti:
prefix node yang menabrak id atau alias registry ditolak (Create dan Update), delete
node ditolak selama masih ada endpoint yang merujuknya. Itu dua aturan kontrak tanpa
satu pun asersi.

**Kerja yang diusulkan.** Test service `NodeService`: create dengan prefix tabrakan =
`CONFLICT`, update yang memindah prefix ke tabrakan lain = `CONFLICT`, delete saat
direferensikan = `CONFLICT` (mock `EndpointCounter`), delete bebas = sukses. Test
service `ProviderService`: filter kategori dan routability, `Detail` untuk id tak
dikenal = `NOT_FOUND`, endpoint_count akurat. Test mux 401 untuk `GET /providers*`
dan keenam route `provider-nodes`.

**Kriteria selesai.** Kedua aturan `CONFLICT` §7.4 punya test bernama; sapu 401
mencakup sembilan route §7.4 (tiga providers, empat oauth, enam node dikurangi
callback yang memang publik).

## 6. F4 (MEDIUM): Gerbang session level mux belum merata

**Fakta.** Test 401 tanpa session sudah ada untuk: gateway-keys
(`TestVersionedRoutesRequireSession`), models §7.6, combos + vision, token-saver,
media, proxies, P4. Belum ada untuk: §7.4 (providers, provider-nodes), §7.5
(endpoints, keys, bulk), §7.12 (usage, quotas), §7.13 (logs), §7.14 (settings).

**Risiko.** Semua route manajemen memang dibungkus `gateway()` saat registrasi
(`router.go:101-206`, diperiksa manual per baris), jadi ini bukan lubang sekarang.
Tapi proteksinya per-baris-registrasi: satu route baru yang lupa dibungkus tidak akan
ketahuan oleh test mana pun, dan pola per-area (yang dipakai 003 untuk menutup area)
hanya menutup area yang sudah disentuh.

**Kerja yang diusulkan.** Satu test sapu yang menelusuri daftar pattern terdaftar
(`routeRecorder` sudah menyimpannya) dan mengasertif 401 untuk setiap route
manajemen tanpa session, dengan daftar pengecualian eksplisit dan beralasan:
`/health`, `/version`, `/auth/login`, `/auth/status`, oauth callback, dan seluruh
data plane. Pengecualian baru wajib menulis alasannya di test.

**Kriteria selesai.** Sapu 401 berlaku untuk 93 route dikurangi pengecualian
ternyata; test gagal jika route manajemen baru terdaftar tanpa `gateway()`; tidak
perlu test per-area lagi untuk gerbang session (yang per-area tetap berguna untuk
verb dan happy path).

## 7. F5 (LOW): Hutang line limit §1.1, praposisi

**Fakta.** Empat file source di atas 250: `internal/provider/plugin.go` (331),
`internal/service/oauth_flow_refresh.go` (279), `internal/schema/messages.go` (260),
`internal/service/quota_flush.go` (255). Empat belas file test di atas 250, terbesar
`dataplane/translate_openai_claude_test.go` (506). Semua ter-commit 2026-09-18/19;
working tree bersih; jadi bukan regresi dari perubahan terbaru.

**Catatan struktur.** Gerbang `scrypts/gates/go-lint.sh` tidak memuat cek jumlah
baris, jadi aturan "CI gate fails if any modified source file exceeds 250 lines"
(AGENTS §1.1) saat ini hanya hidup di review.

**Kerja yang diusulkan (dua bagian, bisa terpisah).**
1. Pecah empat file source ke atas batas sesuai concerns masing-masing (plugin.go:
   pisahkan transport/auth dari registry lookup; oauth_flow_refresh.go: pisahkan
   worker loop dari logika refresh; messages.go: pisahkan decode tools dari decode
   content; quota_flush.go: pisahkan flush counter dari penulisan window).
2. Tambahkan langkah wc -l ke `go-lint.sh` (gagal di atas 250 untuk file source yang
   berubah di staging, peringatan di 220) supaya aturan punya penegak mekanis, bukan
   hanya review.

**Kriteria selesai.** Tidak ada file source non-generated di atas 250; gerbang lint
menolak file baru di atas batas; file test boleh menyusul pelan-pelan (bukan bloker).

## 8. F6 (INFO): Catatan lingkungan dan lingkup

- `golangci-lint` belum terpasang lokal; gerbang melaporkan ketiadaannya sebagai
  `gate_skip`, bukan membungkamnya. Pasang bila ingin suite §1.4 penuh.
- `staticcheck` dipasang dan dijalankan pada audit ini: bersih.
- File untracked di git status ada di `app-ui/` (skema dan komponen token-saver).
  Di luar lingkup dokumen ini dan sedang dikerjakan agent lain; jangan disentuh,
  jangan di-commit bersama pekerjaan app-serv.

## 9. Urutan pengerjaan yang disarankan

1. F2 (paling kecil: satu service, dua verb, fixture sudah ada).
2. F1 (pola sama dengan F2, empat route).
3. F4 (satu test sapu, menutup gerbang session untuk semua area sekaligus,
   termasuk yang F2/F1 sentuh).
4. F3 (paling banyak kasus: dua service + sembilan route sapu 401).
5. F5 bagian 2 (gerbang wc -l) kapan saja; F5 bagian 1 (pecah file) sebagai PR
   tersendiri tanpa perubahan perilaku.

Setiap PR yang mengerjakan nomor di atas wajib menyatakan di deskripsi apakah
`SYSTEM_MAP.md` dan spec ikut berubah (agar "N/A, perubahan test saja" cukup bila
memang tidak ada perubahan struktur). Bila pengerjaan menemukan deviasi kontrak,
spec diedit di PR yang sama, aturan spec-first §0 SPEC-API-001.
