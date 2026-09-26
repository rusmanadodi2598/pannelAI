# 006-PORT-QUOTA-PAGING.md: Paginasi server-side atas /api/v1/quotas + glyph kontrol refresh (app-serv, app-ui)

Pass lanjutan langsung pass 005 pada layar Quota Tracker. Dua permintaan owner 2026-09-26:
paginasi server-side yang di filing 005 §7.4, dan glyph peta ikon untuk kontrol "Refresh now" serta
"Pause refresh"/"Resume refresh" (tanpa emoticon). Pola mengikuti register 004 dan 005.

|                      |                                                                                                                                                                                                                                                                                                                       |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**           | **CLOSED 2026-09-26.** Wire terpaginasi (D1-D7) dan glyph kontrol refresh (D8) terbangun dan teruji; suite 2615/160 pada pohon beku final, integration `-race` hijau dengan DSN, click-through A-G pada gateway HEAD `:9091` membuktikan baca terpaginasi di jalur hidup; SPEC-API §7.12 dan SPEC-UI §6.6 diamendemen |
| **Mechanism**        | DURING & AFTER (antislop)                                                                                                                                                                                                                                                                                             |
| **Scope**            | `app-serv/` (route baca /quotas + kontrak) dan `app-ui/` (halaman /quota). Route detail dan PUT caps tidak berubah                                                                                                                                                                                                    |
| **Permintaan owner** | (1) "Kerjakan pagination server side juga." (2) "Pergantian Icon (None Emoticon): Refresh Now & Paush Refresh"                                                                                                                                                                                                        |
| **Reference**        | Konvensi paging rumah: `DecodePage` (schema/dto.go), meta `Page{page,per_page,total}` (SPEC-API §4), deklarasi params route /endpoints                                                                                                                                                                                |
| **Kaitan**           | SPEC-API §4, §7.12 (diamendemen di pass ini); SPEC-UI §6.6; PORT 005 `docs/PORT/005-PORT-QUOTA-CARDS.md` (§7.4 D5 filing); PORT 004                                                                                                                                                                                   |
| **Tanggal**          | 2026-09-26 (DURING)                                                                                                                                                                                                                                                                                                   |

## 1. Ringkasan

Pass 005 memaginasi kartu di sisi klien atas seluruh window yang wire kirim; skala yang owner
sebut (ratusan sampai ribuan key per provider) membuat wire tak terikat itulah sisa masalahnya.
Pass ini memindah halaman ke server, dan memberi dua kontrol refresh di toolbar /quota glyph dari
peta ikon panel.

## 2. Bukti terukur

### 2.1 Keadaan wire saat ini

- `GET /api/v1/quotas` (handler/quota.go `List` → `writeWindows`): seluruh window, tanpa params,
  tanpa meta; `QuotaWindowList{Data}` (schema/usage_record.go:73). Repo: `ORDER BY endpoint_id,
window` (repository/postgres/quota.go:61); provider_id di-join, bukan disimpan.
- Konvensi paging rumah sudah ada dan teruji: `DecodePage` (page >= 1; 1 <= per_page <= 100;
  default 25; refusal bukan clamp, draft 010 F6), meta `Page{page,per_page,total}` (§4), dan
  `TestOpenAPIContract_PaginationParamsAreBounded` memeriksa tiap route yang mendeklarasikan
  params (page: integer min 1; per_page: min 1 max 100 default 25).
- Panel: `listQuotaWindows()` tanpa query (api/usage.ts:69); paginasi 5 kartu di klien
  (QuotaCards.svelte `FOLD_PAGE_SIZE`).

### 2.2 Kontrol refresh saat ini

- Toolbar /quota (routes/quota/+page.svelte:140-151): "Refresh now" dan "Pause refresh"/"Resume
  refresh" adalah tombol teks polos tanpa glyph; komentar pass lama mencatat glyph bukan scope itu.
- `RefreshControl.svelte` (dipakai layar lain) sudah membawa `CONTROL_ICONS.refresh` (RefreshCw);
  peta sudah punya `refresh`; `pause`/`resume` belum ada di CONTROL_ICONS (Pause/Play hanya di
  ROW_ACTION_ICONS untuk disable/enable baris).

## 3. Keputusan (D)

| #   | Pertanyaan                | Keputusan                                                                                                                                                                                                                                                                        |
| --- | ------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| D1  | Satuan halaman?           | **Grup provider** (kartu), bukan baris window: satu halaman wire memuat seluruh window milik N grup provider, jadi kartu tidak terpotong antar halaman. `per_page` di route INI menghitung grup provider; hal ini disebut eksplisit di §7.12, karena route lain menghitung baris |
| D2  | Params dan meta?          | Konvensi rumah apa adanya: `DecodePage` (page >= 1, 1 <= per_page <= 100, default 1/25, refusal) dan meta `Page{page,per_page,total}`; `total` = jumlah seluruh grup provider (grup "No provider" ikut dihitung satu)                                                            |
| D3  | Urutan?                   | Grup diurutkan `min(endpoint_id)` ASC (urutan first-seen yang klien lihat hari ini); window dalam halaman `endpoint_id ASC, window ASC` seperti sekarang, sehingga pengelompokan first-seen klien mereproduksi urutan grup                                                       |
| D4  | Perilaku tanpa params?    | Halaman 1, per_page 25 (default rumah), meta tetap ada. Konsumen wire hanya panel dan panel kini selalu mengirim page + per_page eksplisit                                                                                                                                       |
| D5  | Pager di klien bagaimana? | State halaman naik ke `+page.svelte` (fetch saat pindah halaman); QuotaCards kehilangan pemotongan klien, menerima `page`/`pageCount`/`onpagechange`; seleksi tetap dibersihkan saat pindah halaman (D4 pass 005); lipat bertahan per kunci                                      |
| D6  | Cakupan picker caps?      | `windowEndpointIds` kini menamai endpoint halaman berjalan saja (bukan seluruh wire). Batas ini ditulis jujur; daftar label endpoint (halaman 1, 100) tidak berubah                                                                                                              |
| D7  | Kontrak?                  | CONTRACT YAML `/api/v1/quotas` get mendapat params page/per_page dan `QuotaWindowList` mendapat `meta` ($ref Page); openapi.json diregenerasi; SPEC-API §7.12 diamendemen (per_page = grup provider di route ini)                                                                |
| D8  | Glyph kontrol refresh?    | Dua entri baru di CONTROL_ICONS: `pause` (Pause) dan `resume` (Play) dengan alasan tertulis; "Refresh now" membawa glyph `refresh` yang sudah ada; semuanya glyph peta (R-04, R-31), nol emoticon                                                                                |

## 4. Findings

- **F1 (MEDIUM):** wire `/quotas` tak terikat (semua window, tanpa meta) pada skala yang owner
  sebut; diukur di §2.1. Ditutup D1-D4.
- **F2 (LOW):** dua kontrol toolbar /quota tanpa glyph, padahal peta dan komponen
  `RefreshControl` membuktikan bahasa visualnya. Ditutup D8.

## 5. Rencana (TDD, merah dulu)

1. app-serv: test handler baru untuk List terpaginasi (meta benar, halaman grup, grup tanpa
   provider, refusal params) merah lebih dulu dengan stashing perubahan sumber; lalu interface
   repo `PageWindowsByProvider` + implementasi postgres (dua query: hitung grup, halaman window),
   service, handler, schema `Meta Page`.
2. Kontrak: YAML (params + meta), regenerasi openapi.json, amendemen SPEC-API §7.12.
3. app-ui: schema zod `meta`, `listQuotaWindows(query)`, state halaman di `+page.svelte`,
   QuotaCards server-driven, glyph toolbar; tes komponen dirombak merah dulu.
4. Gerbang: gofmt/headers/vet/lint/tests + tagged suite (DSN), prettier/eslint/check/build/suite
   penuh panel tanpa beban bersama, click-through, Delivery Gate, commit saat CLOSED.

## 6. Implementasi

- app-serv `repository/postgres/quota.go`: metode baru `PageWindowsByProvider(ctx, page, perPage)`,
  dua pernyataan (hitung grup, halaman window) dengan CTE `page_groups` yang memilih grup
  `LIMIT $1 OFFSET $2` atas `min(endpoint_id)`, lalu seluruh window grup terpilih dalam urutan
  `endpoint_id, window`. Grup tanpa provider (`coalesce(e.provider_id,'')`) ikut sebagai satu grup.
- `repository/usage.go`: kontrak `QuotaRepository` bertambah satu metode (dengan komentar alasan);
  stub `stubQuotaRepo` (handler), `stubCapRepo` dan `recordingQuotaRepo` (service) menyesuaikan.
- `service/quota.go`: `ListWindowsPaged` meneruskan ke repo; `handler/quota.go`: `List` membaca
  params lewat `schema.DecodePage` (refusal, bukan clamp), menjawab `QuotaWindowList{Data, Meta}`;
  `writeWindows` yang kini tak terpakai dihapus.
- `schema/usage_record.go`: `QuotaWindowList` membawa `Meta Page` (§4).
- Kontrak: `docs/CONTRACT/001-CONTRACT-API-V1.yaml`, params page/per_page pada `get /api/v1/quotas`
  dan `meta` ($ref Page) pada `QuotaWindowList`; `openapi.json` diregenerasi lewat
  `go run ./tools/openapi-gen`; `docs/SPEC-API/001-SPEC-API.md` §7.12 diamendemen (per_page =
  grup provider di route ini) dengan paragraf changelog.
- app-ui `src/lib/polling.ts`: `QUOTA_PAGE_SIZE = 5` dan keputusan `clampPage` (pohon yang menyusut
  memarkir pager di halaman terakhir) di samping `pollDue`, karena keduanya keputusan dengan test.
- `src/lib/schemas/quota.ts`: `schemaQuotaWindowList` menerima `meta: pageMeta` (primitive rumah);
  `src/lib/api/usage.ts`: `listQuotaWindows({page, per_page})`.
- `src/routes/quota/+page.svelte`: state `page`/`totalGroups`, `goToPage` (bounded), clamp memicu
  satu baca ulang lewat mekanisme `rerunRequested` yang sudah ada; toolbar "Refresh now" membawa
  glyph `refresh`, toggle pause membawa glyph `pause`/`resume` sesuai keadaan (ikon peta, bukan
  emoticon).
- `src/lib/components/QuotaCards.svelte`: pemotongan klien dihapus; pager menerima
  `page`/`pageCount`/`onpagechange`; seleksi dibersihkan lewat `$effect` yang hanya melacak halaman
  (poll di halaman yang sama tidak menghapus seleksi, D4 pass 005 tetap).
- `src/lib/icons.ts`: dua entri baru CONTROL_ICONS `pause` (Pause) dan `resume` (Play) dengan
  alasan tertulis; test peta ikon menutupnya otomatis (alasan, glyph terlarang).
- Test: handler `quota_paging_test.go` (halaman grup, meta jujur, grup tanpa provider, refusal
  params) merah terbukti (`schema.QuotaWindowList has no field Meta`); integration
  `quota_integration_test.go` menambah `TestQuotaRepository_PageWindowsByProvider_GroupsAndCounts`
  (hijau vs server nyata, -race); panel: `quota.test.ts` dirombak ke shared stub yang kini
  menjawab read terpaginasi (page/per_page, meta, `quotaReads`, `shrinkTo`), 5 kasus baru
  (baca terpaginasi, jalan Next/Previous, seleksi bersih, parkir saat data menyusut, glyph
  toolbar) merah terbukti sebelum implementasi; `polling.test.ts` menambah 4 kasus `clampPage`.

## 7. Status pass ini

### 7.1 Gerbang

| Gerbang                           | Hasil                                                                                                                                                                              |
| --------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| go lint (vet, gofmt, staticcheck) | PASS setelah satu `gofmt -w` atas berkas test baru; golangci-lint 0 issues                                                                                                         |
| go headers (§1.2)                 | PASS 897 berkas Go membawa header lengkap                                                                                                                                          |
| go test (race)                    | PASS seluruh paket; integration di-skip hook (DSN tidak diekspor) lalu dijalankan terpisah: `migrations` + `repository/postgres` `-race -tags=integration` PASS dengan DSN         |
| kontrak (openapi-gen --check)     | PASS dokumen tersaji cocok YAML kontrak; drift kode error panel PASS 12                                                                                                            |
| app-ui prettier / eslint / check  | PASS (svelte-check 0/0 setelah satu guard null di stub); satu berkas milik aktor lain (`tests/schemas/proxy-batch.test.ts`) diformat agar gerbang lolos dan sengaja tidak di-stage |
| app-ui build                      | PASS exit 0                                                                                                                                                                        |

### 7.2 Suite penuh panel

Dua jalan, keduanya jujur tercatat:

1. Jalan pertama pada sidik jari `690f76a514f52fba6423ce70909ebe57`: **3 gagal / 2611 lulus** dalam
   3016 dtk (pohon yang sama tetapi mesin sedang lambat). Dua di antaranya adalah fallout kontrak
   yang sah, dua kasus `tests/schemas/quota.test.ts` masih menganggap body tanpa `meta` sah,
   padahal wire terpaginasi selalu membawa meta; test diperbarui ke kontrak baru plus satu kasus
   refusal baru. Satu lagi adalah timeout 5 dtk `usage-topology-view` yang pindah-pindah berkas,
   kelas kontensi yang sudah dikenal, lulus sendirian segera sesudahnya.
2. Jalan kedua pada pohon beku ulang `27d602856a898fd4682e40c202eca05d`: **2615 test / 160 berkas,
   seluruhnya lulus, exit 0, 1275.73 dtk**, sendirian, log utuh `/tmp/pass36/suite-006b.log`. Itulah
   rekam otoritatif.

Aritmetika atas baseline pass 005 (2600/160): kasus pass ini +8 (quota.test.ts 20 menjadi 23,
polling.test.ts +4 kasus clampPage, schemas/quota.test.ts +1 kasus refusal meta); +7 sisanya adalah
kasus proxy milik aktor lain yang ikut terukur karena suite berjalan atas pohon kerja bersama,
commit pass ini tidak membawa berkas mereka.

### 7.3 Click-through hidup (R-35)

Panel salinan pohon AFTER di `:3001` → **gateway HEAD di `:9091`** (bukan :9090 milik owner yang
masih biner lama tanpa wire terpaginasi; dibangun dari pohon kerja, bukan `git archive HEAD`, karena
perubahan wire belum di-commit, kesalahan pertama yang tertangkap sendiri). Driver
`/tmp/pass36/driver.ts`, hasil `after.json`, tangkapan `shots/after-{desktop,390}.png`; bukti pass
005 diarsipkan sebagai `after-pass005.json`.

- **A, toolbar satu basaat + glyph peta**: `oneRow: true`; `refreshSvg: true`, `pauseSvg: true`
  dengan label "Pause refresh", glyph adalah svg peta ikon, nol emoticon.
- **B, kartu per provider**: dua kartu (3 endpoints, 12 windows + No provider 1 endpoint, 4
  windows), jawaban wire dua grup dirender dua kartu, isi menggulir utuh (`max-h-80
overflow-y-auto`).
- **C, lipat satu kartu**: `aria-expanded` false → baris hilang → kembali.
- **D, lipat massal**: "2 selected" → Fold selected (0 baris) → Unfold selected (4 baris) → bar
  hilang setelah ceklis dilepas.
- **E, paginasi server-driven pada data hidup**: "Page 1 of 1" dengan dua tombol nonaktif (2 grup <
  5), dan bukti kunci, `quotaReads: ["/api/v1/quotas?page=1&per_page=5"]`: yang diminta layar
  adalah halaman server, bukan pemotongan klien. Wire diperiksa langsung juga: meta
  `{"page":1,"per_page":5,"total":2}` dan refusal `?per_page=101` → VALIDATION_ERROR.
- **F, 390 px**: `scrollWidth` 390 = viewport, `scrollX` 0 setelah paksaan, 0 elemen keluar.
- **G, konsol**: `windowErrors: "none"`.

### 7.4 Residu

- Halaman quota kini terikat halaman server; langkah lanjut bila skala menuntut: paged read untuk
  daftar endpoint di picker caps (picker hari ini membaca halaman pertama, 100) dan penomoran
  halaman URL yang bisa dibagikan.
- D6 pass 004 (endpoint→keluarga fetcher) tetap menunggu keputusan owner; pass ini tidak
  menyentuhnya.
