# 024-COMBO-VISION-READINESS.md: Parity endpoint Combo & Vision `app-serv` terhadap REFERENCE

Dokumen kerja hasil pemeriksaan permukaan Combo (`§7.7`) dan Vision Adapter (`§7.8`) `app-serv/.`
terhadap reference `decolua/9router`. Bukan kontrak; kontrak tetap
`docs/SPEC-API/001-SPEC-API.md` (wire) dan `docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel). Pola
mengikuti draft 017 sampai 023: temuan bernomor F, bukti yang bisa diulang, rencana DURING, dan
keputusan owner di depan implementasi.

| | |
|---|---|
| **Status** | **CLOSED 2026-09-24** untuk scope pass ini: F1, F2, F3, F4 selesai dan diverifikasi hidup; F6, F7, F8 non-finding; F5 ditutup bersama F2. Presets dan picker aktif dicatat di §5 |
| **Mechanism** | DURING & AFTER (antislop) |
| **Scope** | `app-serv/.` saja. `app-ui/.` tidak disentuh pass ini |
| **Permintaan owner** | Endpoint Combo & Vision: cara mengambil models id dan penanganan models id di sini masih tidak 1:1 dengan REFERENCE. Filtering yang seharusnya ketika penambahan models id adalah dari provider yang saat itu memang aktif (None Global Models ID). Intinya, Add Combo belum bekerja semestinya |
| **Reference** | `decolua/9router`, checkout `/home/rusmanadodi/apps/9router`, `21583c03` = tag `v0.5.85` (2026-09-22) |
| **Kaitan** | SPEC-API §7.6, §7.7, §7.8, §7.15; SPEC-UI §6.4; AGENTS.md §1.1–§1.9, §2.2; draft 017 (F2 daftar model node), draft 019 (permukaan panel) |
| **Tanggal** | 2026-09-23 (DURING), 2026-09-24 (CLOSED) |

## 1. Ringkasan

Keluhan owner diterjemahkan menjadi pertanyaan yang bisa diukur, dan setiap jawabannya dibandingkan
dengan reference pada baris yang bisa dibuka ulang. Semua pengukuran di bawah dijalankan hidup
terhadap gateway yang berjalan (login sesi panel, gateway key sementara; semua baris probe
dihapus lagi setelah diukur — sisa `combos=0`, `aliases=0`, `vision-adapter` dinonaktifkan kembali,
gateway key probe dihapus).

**Cara reference mengambil models id (hal yang owner tanyakan pertama):**

Reference **tidak pernah** menawarkan katalog global. `ModelSelectModal.js` hanya menampilkan
provider yang **aktif** (`activeProviders` = `connections` dari `GET /api/providers`, baris
`filteredActiveProviders`; provider `noAuth` juga tampil, baris `NO_AUTH_PROVIDER_IDS`), dan model
yang ditawarkan adalah `alias/id` milik provider itu. Kombinasi `combos/page.js:162-164` memanggil
`/api/combos` + `/api/providers` + `/api/settings` sekaligus, lalu melewatkan `activeProviders` ke
`ComboFormModal` → `ModelSelectModal`. Jadi jawaban atas "filtering dari provider yang saat itu
memang aktif" ada di **waktu baca**, bukan di waktu tulis: reference tidak memvalidasi ref combo
sama sekali saat menyimpan (`combos/route.js:47-60` hanya mengecek nama), ia hanya menawarkan pilihan
dari provider aktif di picker.

`app-serv` memilih jalur lain dan sudah tertulis di SPEC-API §7.7: ref divalidasi **saat tulis**
(ValidateRefs, `internal/service/combo_order.go:60`). Keputusan itu tidak dibalik pass ini; yang
diperbaiki adalah kesalahan-kesalahan terukur di bawah, yang membuat "Add Combo" gagal persis pada
nama model yang sah.

**Empat temuan dikerjakan (F1–F4), tiga dicatat sebagai non-finding (F6–F8), satu ditutup bersama
F2 (F5).** Tiga di antaranya cacat HIGH yang menjelaskan "Add Combo belum bekerja semestinya"
secara langsung: F1 (combo bersarang gagal di runtime padahal diterima di tulis), F2 (ref bentuk
alias/prefix provider ditolak di tulis padahal router menerimanya), F3 (vision adapter menolak dua
dari tiga bentuk nama model yang panel tawarkan — akar yang sama dengan F2).

## 2. Bukti: tiga bentuk nama model, dua yang valid di runtime

Router `app-serv` (`dataplane/resolve.go:91` → `registry.Index.Provider(name)` → `byName`) menerima
**tiga bentuk** untuk segmen pertama sebuah model string: id (`openai/gpt-4o`), alias registry
(`cc/claude-sonnet-4-5` — `cc` adalah alias `claude`), dan prefix node (`oczen/big-pickle`).
Terukur hidup: ketiganya mencapai selector (`NO_PROVIDER_AVAILABLE` pada provider yang benar,
bukan `MODEL_NOT_FOUND`).

Tetapi jalur tulis combo/vision membaca **satu** sumber: `ModelCatalogService.lookups()`
(`internal/service/model_catalog.go:115-137`), yang di-key dengan `provider.ID` saja
(`model_catalog.go:147` → `NewModelRef(provider.ID, model.ID)`). Dua konsekuensi terukur (sebelum
perbaikan):

```
POST /combos  ref=cc/claude-sonnet-4-5-20250929      -> 400 "model reference does not resolve..."
POST /combos  ref=oczen/claude-fable-5               -> 400 (idem)
POST /combos  ref=th-1/anything-at-all               -> 400 (idem; prefix node th-1)
POST /combos  ref=claude/claude-sonnet-4-5-20250929  -> 201 (bentuk id diterima)
```

Sementara router menerima bentuk yang sama yang ditolak itu. Inilah inti "penanganan models id
tidak 1:1": **jalur tulis dan jalur routing tidak sepakat tentang nama mana yang valid**, dan
operator yang menyalin dari dokumentasi reference (yang memakai alias, mis. `cc/...`, `oc/...`)
ditolak di tulis padahal model itu benar-benar routable.

## 3. Findings

### 3.1 F1 (HIGH) — combo bersarang diterima di tulis, gagal di runtime

**Status: CLOSED.**

**Fakta.** SPEC-API §7.7 mengizinkan ref = "combo name (one deref level)", dan `validateRefs`
memang menerimanya. Terukur (sebelum perbaikan):

```
POST /combos {"name":"outer","models":[{"ref":"inner"}]}   -> 201  (inner adalah combo lain)
POST /chat/completions model=outer                          -> MODEL_NOT_FOUND "model inner is not a known model, alias, or combo"
```

Penyebabnya bukan validator. `dataplane/resolve_combo.go` (`resolveMember`) memanggil
`resolveReference` lebih dulu — ref tanpa `/` langsung gagal — lalu mencoba **alias** saja. Combo
bersarang tidak pernah dicoba. Bandingkan dengan pemanggilan non-member: `engine.go` memanggil
`resolver.Resolve(ctx, ref)` penuh untuk tiap member fallback/round-robin, yang **memang** melewati
jalur combo. Jadi perilakunya tidak konsisten bergantung posisi member:

```
inner sebagai member PERTAMA  -> MODEL_NOT_FOUND  (resolveCombo → resolveMember: jalur combo dilewati)
inner sebagai member KEDUA    -> diteruskan ke provider (engine loop pakai Resolve penuh)
```

Terukur keduanya. Reference mengeksekusi combo bersarang lewat `getComboModels` satu tingkat
(`chat.js:171-206` menangani "provider null → cek combo lagi"), dan `aggregateComboCapabilities`
(`capabilities.js`) bahkan me-resolve rekursif untuk kalkulasi kapabilitas. Yang benar: **jalur
runtime harus mengikuti kontrak tulis** (satu deref level, termasuk combo).

**Yang dikerjakan.**

1. `resolveMember` kini memeriksa jalur combo lebih dulu untuk ref tanpa `/`, lalu jalur
   reference, lalu alias (alias target yang menunjuk combo juga melewati jalur combo).
2. Guard kedalaman `comboDepthLimit = 16` di `resolveComboWithin`: satu tingkat deref adalah
   kontrak tulis untuk data yang jujur, batas kedalaman adalah jaminan runtime untuk data yang
   menyimpan siklus `A→B→A` (yang menulis lewat DB langsung atau baris lama). Siklus berhenti
   sebagai `MODEL_NOT_FOUND`, bukan stack overflow.
3. Identitas jawaban tetap **combo yang di-address klien**, bukan combo inner: identity respons,
   kunci rotasi, dan baris usage semuanya di-key dengan nama itu.
4. Satu pin lama di `resolve_test.go` ("a nested combo is not expanded, so the outer name is
   refused") dibalik ke perilaku kontrak: nested sekarang resolve dan membawa combo luar.

**Verifikasi hidup (sesudah):**

```
POST /combos outer (member "inner")                        -> 201
POST /chat/completions model=outer                         -> NO_PROVIDER_AVAILABLE "no upstream endpoint is configured for provider claude"
POST /combos/{outer}/test                                   -> results[0] {ref:"live-inner", provider_id:"claude", model_id:"claude-sonnet-4-5-20250929", error_code:"NO_PROVIDER_AVAILABLE"}
```

Runtime kini mencapai selector persis seperti member tidak-bersarang, dan test route melaporkan
provider/model yang ter-resolve.

**Test:** `internal/dataplane/resolve_combo_nested_test.go` (160 baris) — tabel 6 variasi (leading
member, member belakangan, via alias, rantai satu tingkat, inner kosong, siklus) + dua test
identitas dan keamanan siklus.

### 3.2 F2 (HIGH) — ref bentuk alias/prefix ditolak di tulis, diterima di routing

**Status: CLOSED.**

Sudah dibuktikan di §2. Ini menyentuh jalur tulis yang semuanya membaca `lookups()`:

| Jalur | Kode | Sebelum → sesudah |
|---|---|---|
| `POST /combos` member | `combo_order.go` (`validateRefs`) | `cc/...` 400 → **201** |
| `POST /combos` judge | `combo_order.go` | `cc/...` 400 → **201** |
| `PUT /vision-adapter` models | `vision_adapter.go` via `ModelExists` | `cc/...` 400 → **200** |
| `PUT /models/aliases` target | `model_catalog_writes.go` | `th-1/...` 400 → **200** |
| `GET /models/catalog?provider_id=` | `matchesCatalogFilter` | `provider_id=cc` 0 baris → **7 baris**; `provider_id=oczen` 0 → **82** |
| `GET /models/custom?provider_id=` | handler (diabaikan) | diabaikan → **memfilter**, dua arah |

Perhatikan baris terakhir: parameter `?provider_id=` di rute custom **terdokumentasi di SPEC-API
§7.6 dan CONTRACT YAML sejak P2 tetapi handler-nya mengabaikannya** — drift kontrak yang pass ini
tutup sekalian. Filter-nya dua arah: baris custom yang tersimpan di bawah prefix node (tulisan yang
sudah diterima sejak custom node ada) ditemukan oleh bentuk id, dan sebaliknya, karena menyimpan
nama satu arah akan menyembunyikan setengah baris dari salah satu bentuk.

**Yang dikerjakan.** Satu aturan untuk semua jalur tulis: **sebuah ref model valid bila router bisa
meresolvekannya**. Implementasinya di `internal/service/model_catalog_canonical.go`:

1. `catalogKeyFor` — kunci katalog dicoba eksak lebih dulu (ejaan operator), lalu segmen pertama
   di-resolve lewat `index.Provider()` (lookup yang sama dengan router) dan kunci dibangun ulang
   di bawah id kanonik.
2. `ModelExists`, `validateRefs` (member + judge), `ReplaceAliases` target, dan vision `Replace`
   semuanya lewat kunci kanonik itu — satu tempat, bukan lima.
3. Ref yang disimpan **tetap apa adanya** (bentuk yang diketik operator), karena router bisa
   meresolve ketiga bentuk; tidak ada rewrite diam-diam.
4. `matchesCatalogFilter.provider_id` menerima semua ejaan provider (`providerNameSet`: id, alias,
   extra aliases) sehingga `provider_id=cc` menjawab baris yang sama dengan `provider_id=claude`.
5. `Resolve` (jalur baca) melaporkan ref kanonik, dan `isDisabled` membandingkan kanonik di kedua
   sisi — pasangan yang didisable di bawah id juga menyembunyikan ejaan alias/prefix model itu.

**Keputusan implementasi yang menyimpang dari rencana awal.** Rencana awal draft menulis "baris
custom node harus tersimpan di bawah id kanonik". Itu **tidak** dikerjakan: mengubah kunci yang
tersimpan menuntut migration atas data operator yang sudah ada, sementara filter dua arah memberi
pencari yang sama tanpa menulis ulang baris apa pun. Biayanya bila salah: operator yang menyaring
lewat `?provider_id=` melihat semua barisnya (sekarang: benar), dan tidak ada jendela waktu di mana
baris lama tidak ditemukan oleh ejaan apa pun.

**Verifikasi hidup (sesudah):** baris tabel di atas; semuanya diukur ulang di gateway yang berjalan.

**Test:** `model_catalog_canonical_test.go` + `model_catalog_canonical_filters_test.go`
(342 baris, dipecah di ambang §1.1) + `model_custom_filter_test.go` di handler (91 baris).

### 3.3 F3 (HIGH) — vision adapter hanya menerima 1 dari 3 bentuk nama

**Status: CLOSED (akar yang sama dengan F2).**

Terukur (sebelum):

```
PUT /vision-adapter models=["cc/claude-sonnet-4-5-20250929"]       -> 400
PUT /vision-adapter models=["oczen/claude-fable-5"]                -> 400
PUT /vision-adapter models=["claude/claude-sonnet-4-5-20250929"]   -> 200
PUT /vision-adapter models=["openai-compatible-…/claude-fable-5"]  -> 200
```

Akar masalahnya sama dengan F2 (satu sumber `lookups`), jadi menutup F2 menutup F3 — terverifikasi
hidup: `PUT` dengan bentuk alias kini `200`. Satu catatan khusus vision yang tetap: keputusan
capability tetap pada pola model-id (`registry.VisionCapable`), yang memang keputusan reference
yang sama (`capabilities.js`) — 1:1, tidak diubah.

### 3.4 F4 (MEDIUM) — katalog menawarkan model yang tidak bisa dirouting chat

**Status: CLOSED.**

Terukur (sebelum): katalog memuat 9 model `antigravity/...` (provider `format: antigravity`,
`RoutableNeedsConnector`) dan semua model media `black-forest-labs/flux-*` (`kind: image`). Combo
terhadapnya **diterima** di tulis, dan di runtime menjawab:

```
model=antigravity/...           -> PROVIDER_NOT_ROUTABLE "speaks a wire format the gateway does not translate"
model=black-forest-labs/flux... -> NO_PROVIDER_AVAILABLE (chat selector untuk provider image)
```

Data plane sendiri sudah menolak keduanya dari `GET /api/v1/models` (`catalog.go`:
`IsChatRoutable()` dan `IsChat()`). Jadi "listed" dan "answerable" — properti yang
`internal/dataplane/catalog.go` nyatakan sebagai alasan berkas itu ada — pecah justru di jalur
tulis combo. Panel (`CombosTab.svelte:73-76`) membangun saran picker dari `listModelCatalog` penuh,
jadi picker menawarkan model yang router tolak. Reference tidak punya masalah setara:
picker-nya LLM-only untuk combo (`ModelSelectModal.js:183-190`, `filterByKind`).

**Yang dikerjakan.** Member combo, judge, dan model vision adapter harus **servable oleh chat
plane** (`ChatServable` di `ModelCatalogService`, `chatServable` di file kanonik):

1. Provider `RoutableNeedsConnector` → `VALIDATION_ERROR "provider antigravity speaks antigravity,
   which the gateway does not translate yet"` — alasan disebut, bukan generik.
2. Model `kind` non-chat (image/tts/stt/embedding/...) → `VALIDATION_ERROR "model ... is a media
   model (image), not a chat model"`.
3. Kontrol benign: model chat yang dideklarasikan, model tak-dideklarasikan di provider
   passthrough, dan baris custom di provider chat semuanya tetap diterima — aturan tidak boleh
   melebar sampai menolak yang routing terima.
4. Katalog manajemen **tidak** difilter (ia memang katalog penuh; layar provider detail
   membutuhkan baris media) — yang difilter adalah kesepakatan tulis; `kind` sudah tersaji di
   baris katalog sehingga panel bisa menyaring sendiri.

Kontrak diperbarui: SPEC-API §7.7 (POST) dan §7.8 (PUT) menyebut aturan servable + bentuk nama;
changelog 2026-09-23 dicatat. CONTRACT YAML tidak berubah bentuk (validasi pesan, bukan schema) —
gate openapi `-check` lulus tanpa regenerasi.

**Verifikasi hidup (sesudah):** dua pesan penolakan di atas terukur persis; `cc/...` dan
`oczen/...` tetap 201 (kontrol benign).

**Test:** `combo_routable_test.go` (206 baris) — tabel 4 penolakan (member/judge ×
translator/media) + kontrol benign 2 varian + penolakan vision.

### 3.5 F5 (MEDIUM) — bentuk prefix pada model node

**Status: CLOSED bersama F2.** Model node kini bisa dirujuk dengan bentuk id maupun prefix di
semua jalur tulis, dan kedua bentuk memfilter katalog dengan benar. Catatan pemetaan yang tersisa:
reference menyuguhkan model node dengan **prefix** di picker (`ModelSelectModal.js:262-266`), dan
panel `app-ui` menawarkan bentuk id dari katalog — keduanya kini valid, jadi selisih ini turun
menjadi pilihan ejaan, bukan cacat.

### 3.6 F6 (LOW) — test route combo

**Non-finding.** `POST /combos/{id}/test` menjawab `results[].error` per member dengan error code
resolver; tidak ada padanan reference yang perlu disamakan (reference tidak punya route test).
Diperbaiki secara tidak langsung oleh F1: member bersarang kini ter-resolve di probe juga
(terukur: probe atas combo luar melaporkan `provider_id`/`model_id` dari combo inner).

### 3.7 F7 (LOW) — provider `no_auth` tanpa endpoint

**Non-finding, dicatat.** Combo dengan member provider `no_auth` (mis. `opencode`) diterima di
tulis; routing menjawab `NO_PROVIDER_AVAILABLE` sampai operator membuat satu baris endpoint tanpa
key (§7.5; `selection_noauth_test.go` membuktikan endpoint no_auth bisa dipilih). Reference
menampilkan provider noAuth di picker tanpa connection karena routingnya tidak memerlukan baris.
Ini selisih desain §7.5/§7.15 yang sudah tertulis (registry-driven, bukan connection-gated), bukan
drift. Tidak dikerjakan: mengubah jalur seleksi untuk 2 dari 94 provider bukan bagian keluhan
owner.

### 3.8 F8 (LOW) — bentuk GET vision adapter

**Non-finding.** `GET /vision-adapter` mengembalikan `{enabled, round_robin, models, updated_at}`;
reference membaca konfigurasi yang sama dari `/api/settings` (`capacityAdapter`). Wire-nya beda
atas keputusan §7.8, tidak ada yang perlu disamakan.

## 4. Gerbang

Semuanya dijalankan pada working tree akhir:

| Gerbang | Hasil |
|---|---|
| `go build ./...` | bersih |
| `go vet ./...` (+ `-tags=integration`) | bersih |
| `gofmt -l .` | bersih |
| `go test -race -count=1 ./...` | **15/15 paket `ok`** (gate `go-test.sh` lulus) |
| `staticcheck ./...` (+ tagged) | bersih |
| `golangci-lint run` | **0 issues** |
| `scrypts/gates/go-headers.sh` | **815** berkas Go ber-header lengkap |
| `scrypts/gates/go-lint.sh` | lulus; satu band 220–250: `repository/postgres/quota.go` 222 — **milik pass kuota yang berjalan bersamaan**, bukan berkas pass ini |
| `scrypts/gates/contract-openapi.sh` | dokumen tersaji = YAML (tidak ada regenerasi yang perlu) |
| `scrypts/gates/contract-drift.sh` | 11 kode error sepakat |
| `scrypts/gates/secrets.sh` | PASS |

Ukuran berkas: semua berkas pass ini ≤233 baris; dua file test yang sempat 316 baris dipecah di
ambang §1.1 (`model_catalog_canonical_test.go` + `model_catalog_canonical_filters_test.go`), dan
`model_resolve.go` (233) dipecah menjadi 189 + `model_resolve_disabled.go` 70.

## 5. Yang sengaja TIDAK dikerjakan

- **Validasi tulis vs picker aktif (inti kalimat owner "filtering dari provider yang saat itu
  aktif").** Reference memvalidasi **tidak ada** dan menyaring di picker (waktu baca).
  `app-serv` memvalidasi di tulis (waktu tulis) atas keputusan §7.7 yang tertulis. Pass ini tidak
  membalik keputusan itu; yang diperbaiki adalah kesepakatan bentuk nama (F2) supaya validasi
  tulis tidak menolak nama yang router terima. "Filter by active provider" adalah lapisan picker
  panel (`app-ui`) — dicatat sebagai pekerjaan untuk pass `app-ui` berikutnya: picker combo panel
  sebaiknya hanya menawarkan provider yang punya endpoint aktif (atau no_auth), seperti
  `ModelSelectModal.js` reference.
- **`comboStrategies` per-combo di settings** (reference `settingsRepo` + `chat.js:99-101`):
  `app-serv` menyimpan strategi per combo di kolomnya sendiri — 1:1 kemampuan, bentuk beda atas
  keputusan §7.7. Tidak diubah.
- **Presets** (`/api/combos/presets`, sumber cursor/claude): fitur reference yang SPEC-API §7.7
  tidak spesifikasikan. Tidak dikerjakan (YAGNI; menunggu permintaan eksplisit).
- **Normalisasi kunci baris custom** (id kanonik di penyimpanan): lihat keputusan di §3.2 — filter
  dua arah dipilih daripada migration.

## 6. Keputusan yang diambil pass ini (ruling)

1. **F4 dikerjakan tanpa menunggu jawaban §6 versi DURING.** Menerima member yang runtime tolak
   adalah cacat terukur, bukan pilihan desain; menunggu akan meninggalkan picker yang menawarkan
   model gagal. Biaya bila salah: operator yang sengaja ingin combo di provider connector-only
   harus menunggu connector itu ada — itu jujur, bukan kehilangan.
2. **Presets dicatat, tidak dibangun** — di luar keluhan owner, tidak ada baris SPEC-API.
3. **Filter dua arah, bukan normalisasi penyimpanan** (§3.2).

## 7. Status per 2026-09-24 (penutup pass)

**CLOSED.** F1 (combo bersarang di runtime + guard siklus), F2 (kanonikalisasi ref di semua jalur
tulis + filter katalog/custom dua arah), F3 (vision adapter menerima ketiga bentuk), dan F4 (ref
harus servable chat, alasan disebut) selesai, semuanya diverifikasi hidup terhadap gateway yang
berjalan setelah build ulang. F5 tertutup bersama F2; F6, F7, F8 non-finding dengan alasan
tertulis. Delapan berkas sumber disentuh (5 diedit, 3 baru) plus empat berkas test baru dan satu
pin test dibalik; dua berkas dipecah di ambang §1.1. Seluruh gerbang §4 lulus. Baris probe hidup
dibersihkan (combos=0, aliases=0, vision adapter dinonaktifkan, gateway key probe dihapus);
satu-satunya data tersisa milik aktor lain.
