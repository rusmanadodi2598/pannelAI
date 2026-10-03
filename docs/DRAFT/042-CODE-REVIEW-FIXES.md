# 042-CODE-REVIEW-FIXES.md: 22 temuan review source BE, papan progres perbaikan

Register kerja untuk hasil review `/code-review` 2026-10-03 atas `app-serv` (5 sub-agent paralel:
security, dead code, correctness/concurrency, kepatuhan AGENTS.md, contract/config drift — setiap
klaim utama diverifikasi ulang di main thread, termasuk probe Go empiris untuk temuan R01). Dokumen
ini bukan kontrak; kontrak tetap `docs/SPEC-API/001-SPEC-API.md` dan
`docs/CONTRACT/001-CONTRACT-API-V1.yaml`.

|                      |                                                                                                                                                           |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**           | **DONE 2026-10-03** — 22/22. 20 ter-commit, R06 + R07 dirampungkan di working-tree pass ini. Bukti commit→temuan di §4 |
| **Mechanism**        | DURING (test lebih dulu untuk perubahan perilaku, lalu implementasi), AFTER sebagai register ini                                                          |
| **Scope**            | `app-serv` (dataplane, domain, repository, service, provider, handler, router, cmd, config), `docs/CONTRACT`, `docs/RULLES`, `README.md`, `SYSTEM_MAP.md` |
| **Permintaan owner** | "Fix semua hasil temuannya" (goal sesi 2026-10-03), "kita DRAFT kan dulu untuk progres pengerjaannya. Baru kita patch semua"                              |
| **Kaitan**           | 041 (MODEL_NOT_FOUND 404 — R04 menyentuh jalur yang sama), 027 F1 (builder Gemini), 033 (proxy connect), 036 (Qoder egress), `scrypts/gates/*`            |
| **Tanggal**          | 2026-10-03                                                                                                                                                |

## 1. Cara review, dan baseline yang terukur

- `go build ./...` PASS, `go vet ./...` PASS pada `9f0d4cb` (HEAD saat review).
- `scrypts/gates/go-headers.sh` PASS 1159 file; `contract-openapi.sh` PASS (YAML→JSON sinkron);
  `contract-drift.sh` hanya membandingkan SPEC §8 vs enum panel — **bukan** vs Go/YAML, itu akar R10/R11.
- Yang terbukti bersih dan tidak dikerjakan di sini: `internal/netguard` (fail-closed, `Dialer.Control`
  anti DNS-rebinding), SQL seluruhnya parameterized tanpa `Sprintf`, kripto sesi (HMAC + `hmac.Equal`,
  bcrypt, `crypto/rand`), tidak ada `t.Skip`, semua 5 goroutine punya recover, pool limit eksplisit.
- R01 dibuktikan empiris dengan probe `http.Server{WriteTimeout: 2s}` + dua frame berjarak 4 detik:
  client menerima `unexpected EOF`; deadline tulis Go bersifat absolut per request
  (`net/http/server.go` `readRequest` → `SetWriteDeadline(t0+d)`).

## 2. Papan temuan

Severity mengikuti laporan review. Status: OPEN → IN PROGRESS → DONE (dengan bukti di §4).

| ID  | Sev        | Lokasi                                                                                                                           | Temuan                                                                                                                     | Rencana perbaikan                                                                                             | Status |
| --- | ---------- | -------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- | ------ |
| R01 | critical   | `cmd/app-serv/main.go:132-135`, `internal/handler/datplane_errors.go:116-140`                                                    | `WriteTimeout` 120 s memotong SSE stream > 120 s; `sseSink` tidak pernah `SetWriteDeadline`, padahal `IdleTimeout` 300 s   | Bersihkan deadline tulis saat frame pertama; test unit sink + test server nyata                               | DONE   |
| R02 | warning    | `internal/dataplane/transport_call.go:129`, `proxy_route.go:113`                                                                 | Panggilan lewat proxy kehilangan deadline per-attempt; `request.Clone(ctx)` menimpa `attemptCtx`; idle guard tak menutup   | Teruskan `attemptCtx` ke `dialer.Do`; test stub dialer yang merekam deadline request                          | DONE   |
| R03 | warning    | `internal/domain/upstream_endpoint_keys.go:73-109`                                                                               | `UpdateKey` (jalur PATCH live) tak punya guard "minimal satu key aktif" yang dimiliki `RemoveKey`/`SetKeyStatus` (mati)    | Guard di `UpdateKey` via helper bersama; test PATCH menolak disable key terakhir                              | DONE   |
| R04 | warning    | `internal/dataplane/engine_relay.go:97-99`, `engine.go:219-221`, `errors.go:171-172`                                             | Kegagalan `RecordSuccess` setelah jawaban sukses menggagalkan request; domain `NOT_FOUND` → `MODEL_NOT_FOUND`/404          | Telan-log kegagalan persist `RecordSuccess` (simetris `recordFailure`); petakan `NOT_FOUND` ke kode non-model | DONE   |
| R05 | warning    | `internal/dataplane/transport.go:126-140`, `proxy_route.go:146-149`                                                              | Tanpa `CheckRedirect`: 307/308 meneruskan `x-api-key`/`x-goog-api-key` + body ke host redirect                             | `CheckRedirect: ErrUseLastResponse` di client bersama + clone; test upstream redirect menolak leak            | DONE   |
| R06 | warning    | `cmd/app-serv/provider_index.go:101-117`, `internal/dataplane/catalog.go:67,89-90`, `repository/postgres/provider_node.go:71-72` | Overlay registry dibangun ulang per lookup (1 query penuh + rebuild index) dengan `context.Background()`; N+1 di `/models` | Cache overlay + invalidasi; peta dari `All()` untuk custom pair; teruskan ctx; batasi query                   | DONE   |
| R07 | warning    | `cmd/app-serv/provider_wiring.go:76`, `internal/provider/qoder.go:59-61`                                                         | Connector Qoder dibangun dengan client nil → PAT exchange/catalog/identity tanpa netguard                                  | Bangun connector setelah egress, atau gagalkan boot saat client nil                                           | DONE   |
| R08 | warning    | `internal/service/quotafetch/dispatcher.go:26`                                                                                   | `http.Client` global tanpa guard dipakai semua quota fetch, tanpa `CheckRedirect`                                          | Inject client ter-guard lewat seam fetch; set `CheckRedirect`                                                 | DONE   |
| R09 | warning    | `internal/handler/auth_cookie.go:39-40`, `internal/config/config.go:72,83,85,96,97`, `.env.example`                              | `APP_ENV` + 4 var lain terparsing+terkonsumsi tapi tidak ada di template; cookie `Secure` default mati di development      | Tambahkan kelima var ke `.env.example` (+ README bila perlu)                                                  | DONE   |
| R10 | warning    | `internal/router/router_dataplane.go:47-49` vs `docs/CONTRACT/001-CONTRACT-API-V1.yaml` + `openapi.json`                         | `POST /api/v1/systemone` dilayani tapi 0 kemunculan di kontrak & artifact; fixture test coverage tak memuat SystemOne      | Tambah path+skema ke YAML, regenerate artifact, lengkapi fixture test                                         | DONE   |
| R11 | warning    | `internal/dataplane/errors.go:39,107-110` vs YAML                                                                                | `UPSTREAM_REJECTED` diemit Go + ada di SPEC §8/enum panel, tapi 0 kemunculan di YAML/openapi                               | Tambah `UPSTREAM_REJECTED: 400` ke `codes.data_plane` + respons terkait, regenerate                           | DONE   |
| R12 | warning    | `internal/service/session.go:52-74`                                                                                              | `ChangePassword` tidak mencabut sesi lain                                                                                  | Cabut semua digest sesi akun saat password berubah (bulk revoke); test                                        | DONE   |
| R13 | warning    | `AGENTS.md:272`, `docs/RULLES/`                                                                                                  | `docs/RULLES/SSRF.md` diwajibkan NOTE tapi tidak ada                                                                       | Tulis `docs/RULLES/SSRF.md` dari kontrak netguard                                                             | DONE   |
| R14 | warning    | `internal/dataplane/translate_openai_gemini.go:146`, `translate_gemini_parts.go`, `target.go:41-45`                              | Cluster translasi Gemini nol pemanggil + nol test di balik klaim "exercised directly by its own tests" (palsu)             | Test langsung builder (klaim jadi benar) + koreksi komentar bahwa builder belum dirutekan (027 F1)            | DONE   |
| R15 | warning    | `internal/dataplane/translate_claude_to_openai.go:115`                                                                           | `NormalizeClaudeTx` nol pemanggil; header billing Claude diteruskan ke upstream                                            | Panggil di jalur translasi Claude→OpenAI; test header terstrip                                                | DONE   |
| R16 | suggestion | `internal/schema/messages.go` (260), `internal/dataplane/translate_stream_openai.go` (254)                                       | Dua file non-test melewati §1.1 dan lolos gate (hanya file changed yang diperiksa)                                         | Pecah keduanya di bawah 250                                                                                   | DONE   |
| R17 | suggestion | `internal/domain/upstream_endpoint_parity.go:128,145`, `internal/handler/endpoint_response.go:66-97`                             | `RecordUpstreamSuccess/Error` nol pemanggil; `LastError` schema tak pernah terisi live                                     | Panggil recorder dari jalur sukses/gagal data-plane + petakan field; test                                     | DONE   |
| R18 | suggestion | `internal/repository/redis/quota_counter.go:102-115`                                                                             | `Pending` memotong key dulu lalu memfilter → counter kotor bisa kelaparan                                                  | Lanjutkan SCAN sampai terkumpul limit window pending (dengan batas scan); test                                | DONE   |
| R19 | suggestion | `internal/service/oauth_grant.go:100`                                                                                            | Layer service membawa `*http.Request` (§1.5)                                                                               | Ganti dengan tipe request netral (method/url/header/body) di seam grant                                       | DONE   |
| R20 | suggestion | `internal/router/limiter.go:50-56`                                                                                               | Rate limit hanya `RemoteAddr`; XFF diabaikan → satu bucket di belakang reverse proxy                                       | Baca XFF hanya dari trusted proxy (config `TRUSTED_PROXY_CIDRS`, default kosong = perilaku kini)              | DONE   |
| R21 | suggestion | `internal/dataplane/engine_fusion.go:127-153`                                                                                    | Fan-out satu goroutine per anggota combo tanpa semaphore                                                                   | Batasi fan-out dengan semaphore (4-8) sambil menjaga urutan hasil                                             | DONE   |
| R22 | suggestion | `app-serv/README.md:100-112`, `SYSTEM_MAP.md:46,498`, `docs/CONTRACT/001-CONTRACT-API-V1.md`                                     | Drift dokumentasi: struktur 7 dari 13 paket, rentang migrasi, jumlah worker, 6 rute manual contract                        | Rebuild blok struktur dari `git ls-files`, sinkronkan rentang migrasi/worker/daftar rute                      | DONE   |

## 3. Urutan pengerjaan (batch commit)

1. **Batch A — dataplane correctness**: R01, R02, R05, R04, R21, R16, R14, R15.
2. **Batch B — domain & store**: R03, R18, R17, R12.
3. **Batch C — egress & auth**: R07, R08, R20, R19.
4. **Batch D — kontrak & dokumen**: R09, R10, R11, R13, R22.
5. **Batch E — registry cache**: R06.

## 4. Bukti per temuan

Dipetakan commit→temuan (baseline review `9f0d4cb`), lalu pass working-tree ini merampungkan R06
dan menutup tiga temuan yang ter-commit tapi masih merah saat suite `-race` penuh dijalankan.

**Batch A — dataplane correctness**
- R01 `1d7bce2` — deadline tulis SSE diperbarui per frame; `handler/chat_stream_write_budget_test.go`.
- R02 + R05 `49a823a` — `attemptCtx` diteruskan lewat proxy walk, `CheckRedirect: ErrUseLastResponse`; redirect test + stub dialer.
- R04 `43e5d7c` — kegagalan persist `RecordSuccess` ditelan-log, `NOT_FOUND` tak lagi dibaca 404-model.
- R21 `e3b7936` — fan-out dibatasi semaphore 4; `engine_fusion_bound_test.go`. **Pass ini:** race `-race` ditutup (lihat catatan working-tree).
- R14/R15/R16 `82c76af` — header billing Claude di_strip, builder Gemini diuji langsung, dua file dipecah < 250.

**Batch B — domain & store**
- R03 `bcd02ea` — `UpdateKey` menolak disable key terakhir (`refusesDeactivation`).
- R18 `33ac55a` — SCAN quota `Pending` jalan sampai limit window terisi.
- R17 `ec97f7a` — recorder upstream outcome dipanggil jalur data-plane, field parity dipetakan ke wire.
- R12 `e6ae084` — `ChangePassword` me-revoke semua sesi (`RevokeAll`) + test.

**Batch C — egress & auth**
- R07 + R08 `0b2496c` — connector Qoder + quota fetch naik client egress ter-guard; `provider_wiring_guard_test`, `quotafetch/egress_client_test`. **Pass ini:** R07 ditutup di konstruktor — `NewQoder` menolak client nil (fail-closed) sesuai SSRF §2.1; subtest "nil egress client" di `qoder_test.go`.
- R19 `437dd3d` — grant call netral (method/url/header/body), service lepas dari `*http.Request`.
- R20 `e36b08d` — bucket rate-limit dari client nyata di belakang `TRUSTED_PROXY_CIDRS`; `limiter_test`.

**Batch D — kontrak & dokumen**
- R09 `bb6bb36` — enam var terparsing kini ada di `.env.example` (+ cookie `Secure` dari `IsProduction`).
- R10 + R11 `3c298af` — path `/systemone` dan kode `UPSTREAM_REJECTED` masuk YAML + `openapi.json` + fixture. **Pass ini:** R10 menutup celuh audit sesi (lihat catatan).
- R13 `a77e369` — `docs/RULLES/SSRF.md` ditulis dari kontrak netguard.
- R22 `68a63cc` — blok struktur, jumlah worker, rentang migrasi, daftar rute disinkron. **Pass ini:** README rentang migrasi dibetulkan (lihat catatan).

**Batch E — registry cache**
- R06 (working tree, belum di-commit) — overlay di-cache + TTL + `InvalidateNodeOverlay`; `catalog.go` peta custom-pair dari snapshot `All()` (nol lookup per pair); `NodeService` Create/Update/Delete meng-invalidasi. Test: `provider_index_cache_test.go`, `catalog_pair_map_test.go`, `provider_node_invalidate_test.go`.

**Yang pass ini perbaiki dari temuan ter-commit tapi suite-nya merah**
- R21: `domain.UpstreamEndpoint.Clone()` + `List` mengembalikan clone + `RecordUpstreamOutcome` menulis balik baris → race fan-out hilang, rotasi key tetap maju.
- R10: `POST /api/v1/systemone` didaftarkan ke `excludedFromSessionSweep` (rute data-plane gateway-key) → audit sesi hijau.
- R22: `README.md` rentang migrasi 000001-000011 → 000013.

Gate pada pass ini: `go build ./...` PASS, `go vet ./...` PASS, `go test -race ./...` PASS (18 paket, 0 race), `go-headers.sh` PASS 1182 file.

## 5. Yang tidak dikerjakan di sini

- Temuan 041 §6 yang masih terbuka (body upstream bocor ke `message` client, `PROVIDER_NOT_ROUTABLE`
  tak ada di §8) — menunggu keputusan owner, bukan bagian 22 temuan review ini.
- `-race` flaky `provider_model_probe_budget_test.go:33` (sudah ada sebelum pass ini).
