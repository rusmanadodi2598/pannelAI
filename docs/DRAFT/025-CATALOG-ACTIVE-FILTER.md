# 025-CATALOG-ACTIVE-FILTER.md: Filter `?active=true` pada katalog model

Dokumen kerja untuk permintaan owner (2026-09-24): picker Combo di panel harus hanya menawarkan
model dari **provider yang saat itu aktif** — seperti REFERENCE (`ModelSelectModal.js` memfilter
`activeProviders` di waktu baca) — dan penyaringan itu harus **pindah ke server** melalui parameter
query baru `GET /api/v1/models/catalog?active=true`. Bukan kontrak; kontrak tetap
`docs/SPEC-API/001-SPEC-API.md` (wire) dan `docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel). Pola
mengikuti draft 017–024: temuan bernomor F, bukti yang bisa diulang, rencana DURING, dan keputusan
owner di depan implementasi.

| | |
|---|---|
| **Status** | **CLOSED 2026-09-24**: F1–F5 selesai, diverifikasi hidup, seluruh gate lulus |
| **Mechanism** | DURING & AFTER (antislop) |
| **Scope** | `app-serv/.` saja. `app-ui/.` TIDAK disentuh pass ini (owner: panel dikerjakan terpisah) |
| **Permintaan owner** | "Ini belum ada filtering seperti REFERENCE: panel tetap dikerjakan sekarang, dan saya file permintaan ke app-serv (DRAFT baru) untuk filter `?active=true` di `/models/catalog` supaya penyaringan pindah ke server" |
| **Reference** | `decolua/9router`, checkout `/home/rusmanadodi/apps/9router`, `21583c03` = tag `v0.5.85` (2026-09-22) |
| **Kaitan** | SPEC-API §7.6, §7.7, §8; SPEC-UI §6.4; AGENTS.md §1.1–§1.9; draft 024 §5 (picker aktif dicatat sebagai pekerjaan panel berikutnya); draft 017 §3.1 (pemetaan registry) |
| **Tanggal** | 2026-09-24 (DURING) |

## 1. Ringkasan

Draft 024 menutup "Add Combo belum bekerja semestinya" untuk **kebenaran** nama model (tiga ejaan,
servability, combo bersarang). Yang tersisa adalah **penawaran**: picker Combo `app-ui`
(`CombosTab.svelte:69 loadSuggestions`) masih memanggil `listModelCatalog({})` **tanpa filter** dan
menawarkan seluruh 587 baris dari 67 provider — termasuk provider yang tidak punya endpoint aktif,
yang modelnya pasti gagal `NO_PROVIDER_AVAILABLE` saat dicoba. REFERENCE tidak pernah begini:
pickernya menawarkan hanya provider aktif (baris `filteredActiveProviders`), dinyalakan dari
`connections` (`GET /api/providers`), **di sisi klien**.

Owner memutuskan arsitektur berbeda untuk port ini: penyaringan pindah ke **server**. Picker panel
nanti tinggal memanggil `GET /models/catalog?active=true` (pekerjaan `app-ui`, pass terpisah), dan
server yang menjawab baris-baris mana yang benar-benar berasal dari provider aktif.

## 2. Bukti yang bisa diulang (diukur hidup, 2026-09-24)

Gateway yang berjalan (build 6e017fb), sesi panel, PostgreSQL+Redis lokal:

```
GET /api/v1/models/catalog                     -> 200, 587 baris, 67 provider berbeda
GET /api/v1/models/catalog?active=true         -> 200, 587 baris (parameter diabaikan senyap)
GET /api/v1/models/catalog?active=yes          -> 200, 587 baris (idem — typo tak ditolak)
GET /api/v1/models/catalog?active=1            -> 200, 587 baris (idem)
GET /api/v1/providers?per_page=100             -> provider dengan >=1 endpoint aktif: 1 dari 86
```

Router menolak model dari provider tanpa endpoint aktif (probe hidup, key sementara dihapus lagi):

```
POST /chat/completions  model=opencode/muse-spark-1.2-contributor-free  (no_auth, 0 endpoint)
  -> 503 NO_PROVIDER_AVAILABLE "no upstream endpoint is configured for provider opencode"
POST /chat/completions  model=mmf/gpt-5  (alias mimo-free, 0 endpoint)
  -> 503 NO_PROVIDER_AVAILABLE "no upstream endpoint is configured for provider mimo-free"
```

Jadi hari ini picker menawarkan 586 dari 587 baris yang **dijamin gagal** di permintaan pertama.
Itulah "belum ada filtering seperti REFERENCE" dalam angka.

**Aturan pemilihan endpoint router** (`internal/dataplane/selection.go:200-215 candidates`):
`EndpointFilter{ProviderID: resolution.Provider.ID, Status: active}` — kandidat hanya endpoint
**aktif** di bawah **id kanonik** provider (bukan alias/prefix; `resolution.Provider.ID` hasil
`index.Provider(name)`). Endpoint rate-limited tetap kandidat tapi diskip `Available()`; endpoint
`error`/`disabled` bukan kandidat.

## 3. Findings

### 3.1 F1 (HIGH) — `?active=true` tidak ada; katalog tak bisa diminta "hanya provider aktif"

**Status: OPEN (dikerjakan pass ini).**

Handler `Catalog` (`internal/handler/model.go:41-55`) membaca tiga parameter (`provider_id`,
`capability`, `q`) dan mengabaikan parameter lain senyap. `CatalogFilter` (service) juga hanya tiga
field. Tidak ada jalur untuk panel meminta baris dari provider aktif saja.

### 3.2 F2 (HIGH) — presisi "aktif" harus didefinisikan dari fakta router, bukan tebakan

**Status: OPEN (dikerjakan pass ini).**

"Aktif" punya tiga kandidat makna, dan pilihan menentukan jawaban:

- **(a) Provider punya ≥1 endpoint ber-status `active`** — kandidat katalog pemilihan router;
  rate-limited masih aktif (di-skip runtime, bukan bukan-aktif); `error`/`disabled` tidak.
- **(b) Provider `no_auth`** (reference: `NO_AUTH_PROVIDER_IDS` selalu tampil di pickernya).
- **(c) Kombinasi (a) OR (b)**.

Bukti pemutus: `opencode` dan `mimo-free` adalah `no_auth` **dan** tidak punya endpoint, dan keduanya
menjawab `NO_PROVIDER_AVAILABLE` di probe hidup — jadi `no_auth` **tanpa endpoint** tetap tidak bisa
dilayani router port ini. Di reference `no_auth` tampil karena connection virtualnya **memang**
menjawab (tanpa perlu endpoint). Maka definisi yang konsisten dengan router port ini adalah **(a)**
murni: baris dari provider dengan ≥1 endpoint aktif. `no_auth`-dengan-endpoint-aktif otomatis
tercakup oleh (a) itu sendiri. Keputusan ini dicatat sebagai keputusan owner di §6.

### 3.3 F3 (MEDIUM) — `provider_id` endpoint tersimpan sebagai **id kanonik**, baris katalog juga id kanonik; tapi ada ejaan lain di jalur tulis

Endpoint baris (`provider_id` kolom) ditulis dari `resolution.Provider.ID` — selalu id kanonik.
Baris katalog `lookups()` juga di-key `provider.ID` (draft 024 F2). Match keduanya cukup `==`
langsung. **Tetapi** `Custom()` dan `matchesCatalogFilter` menerima ejaan alias/prefix; penyaringan
`active` tidak boleh mengubah aturan itu — ia hanya menambah predikat, bukan mengganti cara
provider_id dicocokkan.

### 3.4 F4 (MEDIUM) — N+1: roll-up status per provider sudah ada dan satu-query

`EndpointStatusCountsByProvider(ctx, ids)` (postgres `endpoint_summary.go`) menjawab semua provider
dalam **satu** `SELECT ... GROUP BY provider_id`. Katalog aktif tinggal memanggilnya sekali dengan
daftar id provider di katalog (atau lebih murah: hanya id yang match filter). Bukan temuan cacat —
preseden yang dipakai; dicatat supaya penerapan tidak mengarang query kedua.

### 3.5 F5 (LOW) — `active=yes` harus `VALIDATION_ERROR`, bukan diabaikan senyap

Preseden house: `routability` di handler provider menolak nilai di luar closed set dengan
`ValidateStruct` `oneof`; `per_page>100` ditolak (changelog 2026-09-22). Parameter boolean yang tidak
dikenal harus ditolak dengan 400 supaya panel salah eja tidak silently melihat katalog penuh
(masalah tepat yang sama dengan status filter lama yang "SUCCESS" dibaca "kosong").

## 4. Rencana implementasi (DURING)

1. **Domain/service**: `CatalogFilter` bertambah `Active *bool`. Predikat `activeProviders` =
   `counts[providerID].Active >= 1` (satu query `EndpointStatusCountsByProvider`). Deps baru
   **opsional** (`EndpointCounterByProvider`, sudah ada tipenya di `provider.go`); tanpa counter →
   penyaring aktif menolak `active=true` dengan `INTERNAL_ERROR` naming the seam (preseden: prober
   nil = "connectivity testing is unavailable"). Kombinasi dengan `provider_id`: hitung hanya untuk
   provider yang match (dua ejaan dua arah seperti F3).
2. **Handler**: parse `active` dengan closed set (`true`/`false`, keduanya valid; selain itu 400
   `VALIDATION_ERROR`). `active=false` = **tidak menyaring** (default, kompatibel mundur) — bukan
   "hanya provider non-aktif", karena tak ada kasus pakai yang terukur dan membalik makna parameter
   adalah teknik anti-hardcoding terburuk (memperluas tanpa bukti kebutuhan). Dicatat di §6.
3. **Kontrak**: `active` parameter boolean di CONTRACT YAML `/models/catalog` + regen
   `tools/openapi-gen` + SPEC-API §7.6 baris tabel + changelog. **Batas koordinasi**: worktree YAML
   milik aktor lain (drift `default_model` 11→14 belum ter-generate ke JSON) — saya hanya menambah
   hunk sendiri di bagian `/models/catalog` dan regen; JSON akan ikut membawa hunks aktor lain yang
   sudah konsisten di worktree (YAML 14 == JSON 14). Hal ini aman karena kedua file konsisten di
   worktree; gate `-check` tetap harus lulus.
4. **TDD (RED dulu)**: service table-test (filter aktif: provider tanpa endpoint hilang; provider
   dengan endpoint aktif bertahan; rate-limited masih aktif; tanpa counter → error; kombinasi
   `provider_id` ejaan alias; `Active=nil` tidak menyaring) + handler test (400 untuk `yes`/`1`/kosong
   string, 200 untuk `true`/`false`).
5. **Gates**: `go build`, `go vet`, `gofmt -l .`, `staticcheck`, `golangci-lint`,
   `go test -race -count=1 ./...`, `go-headers.sh`, `contract-openapi.sh`, `contract-drift.sh`,
   `secrets.sh`. File baru < 220 baris (header AGENTS.md §1.2).
6. **Verifikasi hidup**: login → `GET /models/catalog?active=true` → harus menjawab hanya baris dari
   provider dengan endpoint aktif (hari ini: 1 provider); `?active=yes` → 400; `?active=false` →
   katalog penuh; kombinasi `provider_id=<id-node>` + `active=true`.

## 5. Yang sengaja TIDAK dikerjakan pass ini

- **`app-ui`** — owner menegaskan panel dikerjakan terpisah; permintaan ini hanya app-serv. Picker
  Combo (CombosTab `loadSuggestions` → `listModelCatalog({active:true})` + label) dicatat sebagai
  pekerjaan panel berikutnya.
- **Menyaring `Custom()`** (`GET /models/custom?active=`) — belum ada pemanggil; YAGNI. Dicatat di
  §6 kalau owner minta nanti.
- **Virtual connection `no_auth`** seperti reference (endpoint sintetis untuk provider no_auth) —
  perubahan arsitektur router besar, di luar permintaan.

## 6. Keputusan owner (ruling) pass ini

| # | Pertanyaan | Jawaban | Konsekuensi |
|---|---|---|---|
| 1 | Definisi "aktif" | **(a) ≥1 endpoint ber-status `active`** di bawah provider itu | `no_auth` tanpa endpoint (opencode, mimo-free hari ini) **tidak** dianggap aktif — terukur gagal `NO_PROVIDER_AVAILABLE`; konsisten router port ini, tidak meniru reference yang connection virtualnya memang menjawab |
| 2 | `active=false` | **Tidak menyaring** (≡ default) | Kompatibel mundur; bukan komplemen "hanya non-aktif" — tak ada kasus pakai terukur |
| 3 | Nilai liar (`yes`, `1`, `TRUE`) | **400 `VALIDATION_ERROR`** | Preseden `routability`/`per_page`; salah eja tidak boleh silently jadi katalog penuh. **String kosong** mengikuti house rule semua parameter query (DecodePage): kosong = absen, tidak menyaring |

## 7. Status final (CLOSED 2026-09-24)

Semua temuan dikerjakan dan diverifikasi hidup terhadap build baru (port verifikasi terpisah :9099;
gateway :9090 milik aktor lain tidak disentuh):

| | Bukti hidup |
|---|---|
| F1 | `GET /models/catalog?active=true` menjawab **1 baris dari 1 provider** (dari 587 baris / 67 provider): hanya `openai-compatible-…BKG9…` yang memegang endpoint aktif. Tanpa parameter tetap 587 (kompatibel mundur) |
| F2 | Predikat diambil dari query kandidat router sendiri: `ActiveProviders` (`SELECT DISTINCT provider_id ... status='active'`), satu statement. Kombinasi `?provider_id=th-1&active=true` (prefix node) dan bentuk id kanonik keduanya menjawab baris yang sama |
| F3 | Filter `provider_id` dua-ejaan dua-arah tetap utuh; predikat aktif tidak mengubah pencocokan nama |
| F4 | Satu panggilan seam per baca (test `CatalogActiveCountsOnce`: 1 panggilan, 3 id dalam satu daftar); baca tanpa `active` **tidak menyentuh** seam (test `CatalogWithoutActiveNeverCounts`) |
| F5 | `?active=yes` → 400, `?active=1` → 400, `?active=TRUE` → 400; `?active=` (kosong) = absen (587 baris), mengikuti house rule `DecodePage`. Kontrak YAML + JSON served membawa `enum: [true, false]`, dipin oleh test kontrak baru (`BooleanQueryParametersAreClosed`) |

**Presisi predikat (temuan ulasan aktor panel, diterima penuh).** Komentar pertama saya mengklaim
`Active >= 1` dari roll-up adalah "himpunan router" — itu salah dua arah: `EndpointStatusCounts.Add`
melipat endpoint rate-limited **keluar** dari `Active`, sementara router tetap menganggapnya kandidat;
dan `MarkUnhealthy` (status→error) **tidak membersihkan** jendela backoff, jadi timestamp basi akan
menghitung endpoint mati sebagai aktif lewat bucket `RateLimited`. Roll-up tidak dapat menyatakan
himpunan kandidat. Perbaikannya bukan aproksimasi lebih baik, melainkan **bertanya langsung**: seam
baru `ActiveProviderSet.ActiveProviders` dengan predikat yang identik dengan kandidat router
(`status='active'`), dipin oleh test integrasi yang menata keempat keadaan (aktif; aktif+backoff;
disabled; error dengan timestamp basi) berlawanan satu per satu.

Sambungan wiring: `cmd/app-serv/catalog_wiring.go` (dipecah dari `management_wiring.go` di ambang §1.1)
mengikat `Active: endpointRepo`. Deployment tanpa seam menjawab penolakan bernama (`INTERNAL_ERROR`
"requires the endpoint candidate reader"), dan baca polos tetap berfungsi tanpa seam itu (dipin di test).

Gates: `go build`, `go vet` (kedua tag), `gofmt`, `staticcheck` (kedua tag), `golangci-lint` 0 issues,
`go test -race ./...` (semua paket, default tag), suite integrasi `-tags=integration -p 1` terhadap
`pannelai_test` (lulus), `go-headers.sh` (830 berkas), `contract-openapi.sh`, `contract-drift.sh`,
`secrets.sh`. Semua berkas yang disentuh < 224 baris.

## 8. Insiden dan remediasi (2026-09-24, dicatat terbuka)

**Apa yang terjadi.** Saat menjalankan test integrasi baru, saya mengarahkan
`PANNELAI_TEST_POSTGRES_DSN` ke **database dev** `pannelai` (menyalin `POSTGRES_DSN` dari `.env`,
bukan menulis DSN test). Harness paket itu menjalankan `TRUNCATE upstream_endpoints CASCADE`, dan 3
endpoint + 3 key operator (node `th-1`, TH HARBOR 1) terhapus. Data terhapus **dua kali**: pertama oleh
run saya (07:07), kedua (08:01) oleh run yang sama terhadap file yang belum dipagari — setelah pemulihan
pertama, guard yang saya pasang hanya di satu keluarga test, sementara keluarga kedua
(`cmd/app-serv/playground_live_stack_test.go`, milik aktor lain) membuka DSN yang sama tanpa guard.

**Pemulihan (terverifikasi).** Tuple INSERT asli masih ada di WAL segmen `000000010000000000000023`;
nilai lengkap (id, label, provider, auth, priority, status, ciphertext `value_encrypted`, `key_hint`,
`created_at`) diekstrak dengan `pg_waldump`/`strings` dan di-INSERT ulang dengan id yang sama. Buktinya
bukan hanya jumlah baris: panggilan chat nyata melalui endpoint yang dipulihkan berhasil (HTTP 200,
jawaban `"OK"`/`"RESTORED"` dari `deepseek-v4.1-flash` via tokenharbor) — ciphertext terdekripsi dengan
`ENCRYPTION_KEY` yang sama, jadi key yang dipulihkan adalah key yang asli, bukan baris kosong.

**Remediasi (pencegahan).** Guard nama-database sekarang menolak DSN yang tidak menyatakan dirinya
database test (nama mengandung `test`) **sebelum satu statement pun berjalan**, di **setiap** titik yang
membuka pool dari env: `requireTestDSN` di harness `internal/repository/postgres` (empat opener), dan
`liveTestDSN` di `cmd/app-serv/playground_live_stack_test.go`. Diverifikasi: DSN dev ditolak dengan pesan
penolakan yang menyebut alasan (RED), DSN `pannelai_test` diterima (GREEN), dan data dev tetap utuh
setelah seluruh suite integrasi berjalan. Test yang menata keadaan predikat (`status='active'` vs
disabled/error/backoff) adalah bagian dari remediasi: predikat yang tepat adalah yang membuat "provider
aktif" berarti "router punya kandidat", bukan tebakan dari roll-up.

**Pelajaran yang dicatat.** Variabel env bernama `*_TEST_*` tidak pernah cukup sebagai pagar; yang
menghapus data adalah statement pertama yang berjalan, bukan niat variabelnya. Pagar yang hanya ada di
harness yang saya tulis sendiri tidak melindungi dari harness yang ditulis orang lain terhadap variabel
yang sama — guard harus ada di titik pembukaan koneksi, dan satu-satunya cara memastikannya adalah
menolak nama database yang tidak menyatakan dirinya test.
