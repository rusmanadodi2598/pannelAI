# 002-PORT-PROVIDER.md: Permukaan `/provider` (daftar + detail) sejajar REFERENCE

Dokumen kerja pass `app-ui` untuk halaman **Provider**, dengan satu kiriman kecil di `app-serv`. Pass ini
mengerjakan tiga hal yang owner sebut langsung: **revisi plain teks menjadi ikon** pada kontrol halaman
Provider (termasuk detail tiap provider), **fitur Search** yang reference sudah punya, dan **tata letak
compact + simetris** untuk Filter Categori, Refresh Now, dan Search. Bukan kontrak; kontrak tetap
`docs/SPEC-API/001-SPEC-API.md` (wire) dan `docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel). Pola mengikuti
`001-PORT-ENDPOINT-KEYS.md` dan draft 017-031: temuan bernomor F, bukti yang bisa diulang, keputusan di
depan implementasi, dan gerbang antislop di akhir.

|                        |                                                                                                                                                                                                                                                                                                                                                                              |
| ---------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**             | **CLOSED 2026-09-25.** F1-F4 diimplementasi, D1-D7 dipakai, seluruh gerbang hijau, click-through hidup 22/22. Bukti gerbang dan click-through di §7; commit pekerjaan `faa57a3`                                                                                                                                                                                            |
| **Mechanism**          | DURING & AFTER (antislop)                                                                                                                                                                                                                                                                                                                                                    |
| **Scope**              | `app-ui/.` (halaman Provider: `/providers` dan `/providers/[provider_id]`, kedua bentuk layar) + `app-serv/.` (satu parameter `?q` di `GET /api/v1/providers`, §7.4)                                                                                                                                                                                                          |
| **Permintaan owner**   | (1) Revisi plain teks kontrol halaman Provider menjadi ikon, tanpa emoticon: Delete, Edit, Rename, Add Api Key, Test Endpoint, Save Endpoint, Save, Cancel, Prev, Next, Add Model, Copy, Remove, Refresh Now, dan yang relevan; termasuk detail provider spesifik. (2) Penambahan fitur Search seperti REFERENCE. (3) Tata letak Filter Categori, Refresh Now, Search dibuat compact dan simetris. (4) Commit lokal setelah progres `CLOSED`. |
| **Reference**          | `decolua/9router`, checkout `/home/rusmanadodi/apps/9router`, `39e36d3d` = tag `v0.5.86` (2026-09-23)                                                                                                                                                                                                                                                                         |
| **Kaitan**             | `docs/PORT/README.md` (keputusan desain); SPEC-UI §6.3, §8.4, §8.6, §8.11, §14 Q13; SPEC-API §7.4; DESIGN.md §2.1, §3.4, §11; `docs/RULLES/TDD.md`, `docs/RULLES/OWASP.md`; draft 017 §4.2/§4.5, draft 019, draft 025 §1                                                                                                                                                     |
| **Tanggal**            | 2026-09-25 (DURING)                                                                                                                                                                                                                                                                                                                                                          |

## 1. Ringkasan

Tiga permintaan owner diukur dulu sebelum satu baris diubah, karena dua di antaranya menunjuk sebab yang
lebih sempit dari yang tersirat di kalimatnya.

| Yang owner sebut            | Yang diukur                                                                                                                                                                                                                                                                                                                                |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Search belum ada            | **Benar, dan sebabnya di server.** `GET /api/v1/providers` hanya membaca `category` dan `routability` (`app-serv/internal/handler/provider.go:42-52`). Diukur hidup pada gateway `:9090`: `?q=openai` dan `?q=zzzz-nothing` sama-sama menjawab **total 112** dengan baris pertama yang sama (`api-airforce`), yaitu parameter diabaikan senyap; `?category=zzz` justru ditolak `VALIDATION_ERROR`. §6.3 sudah mencatat kontrol ini absen daripada palsu (Q13), dan disiplin paginasi §6.3 melarang penyaringan registry di browser |
| Plain teks harus jadi ikon  | **Benar.** Halaman Provider masih merender 35 entri kontrol berteks tanpa glyph (inventaris §2.3, dihitung per layar), sementara reference merender setiap `Button` sebagai ikon + label (`providers/[id]/page.js:1460,1472,1481`, `:1532,1541`; `Button` menerima prop `icon`). Sebagian aksi baris panel sudah icon-only (kunci endpoint dan gateway, keputusan owner 2026-09-24), jadi yang tersisa adalah ketidakcocokan, bukan kekurangan mekanisme                                                                                                                     |
| Tata letak belum compact    | **Benar.** Filter kategori adalah label bertumpuk di atas `<select>` (`+page.svelte:93-105`), Refresh Now duduk di baris sendiri di bawahnya (`:107`), dan tidak ada Search sama sekali, sehingga tiga kontrol yang sejenis tersebar di tiga baris                                                                                                                                                    |

Jadi F1 (parameter `?q` yang tidak ada) adalah sebab Search tidak bisa dibangun di panel, bukan sekadar
kontrol yang belum ditambahkan: tanpa kiriman `app-serv`, Search hanya bisa menyaring 25 baris halaman yang
sedang tampil, dan itu kontrol yang berbohong (R-26).

## 2. Bukti yang bisa diulang (diukur hidup, 2026-09-25)

Gateway owner berjalan di `127.0.0.1:9090` (build `go run`), panel di `:3000` tidak disentuh pass ini.
Sesi dibuka dengan `POST /api/v1/auth/login` (204 + cookie) memakai `PANEL_BOOTSTRAP_PASSWORD` dari
`app-serv/.env`.

### 2.1 Registry dan `?q`

```
GET /api/v1/providers?per_page=1                    -> 200, meta.total = 112, first = api-airforce
GET /api/v1/providers?per_page=1&q=openai           -> 200, meta.total = 112, first = api-airforce
GET /api/v1/providers?per_page=1&q=zzzz-nothing     -> 200, meta.total = 112, first = api-airforce
GET /api/v1/providers?per_page=1&category=zzz       -> 400, {"error":{"code":"VALIDATION_ERROR",...}}
GET /api/v1/providers?per_page=200                  -> 400, per_page must be at most 100
```

`?q` tidak ditolak dan tidak dipakai: dua nilai yang mustahil berbeda hasilnya sama dengan tanpa parameter.
Kategori yang tidak dikenal justru ditolak, jadi route ini memvalidasi yang ia kenal dan mengabaikan yang
tidak ia kenal, dan itu bentuk kegagalan senyap yang paling sulit terlihat dari panel.

Jumlah baris yang akan cocok setelah F1 diperbaiki, dihitung dari 112 baris yang dibaca penuh
(`per_page=100`, dua halaman), pencocokan substring case-insensitive pada `id` atau `name`:

| Query   | Baris | Contoh id yang cocok                                          |
| ------- | ----- | ------------------------------------------------------------- |
| `openai`| 4     | `codex`, `openai`, `azure`, `openai-compatible-0386BKG9Q4…`   |
| `code`  | 11    | `alicode-intl`, `claude`, `codex`, `opencode`, `kilocode`, …  |
| `anthro`| 1     | `anthropic`                                                   |

Dua baris terakhir penting untuk click-through: `openai-compatible-0386BKG9Q4DYZPYC01VJH4C51G` adalah
node custom yang sudah ada, jadi layar detail node bisa dikunjungi tanpa menulis baris baru.

### 2.2 Bentuk kontrol reference

Reference menaruh pencarian di Header, bukan di badan halaman: `registerSearch("Search providers...")`
(`page.js:116`) menampilkan `HeaderSearch` (`Header.js:336-370`) yang berisi glyph `search` di dalam field,
tombol clear ber-`aria-label="Clear search"`, dan penyaringan **di sisi klien** atas seluruh daftar
(`page.js:120-123`, `matchSearch`). Filter statusnya satu `<select>` rata kanan di atas daftar
(`page.js:386-397`). Panel tidak bisa meniru sisi kliennya: §6.3 mewajibkan paginasi server, dan draft 025
sudah memutuskan penyaringan pindah ke server untuk kasus yang sama di katalog model.

Setiap tombol aksi reference memakai ikon + label: `icon="add"` (Add Compatible), `icon="edit"`,
`icon="delete"`, `icon="sync"`, `icon="key"` (`providers/[id]/page.js:1460-1730`). Jadi bentuk parity
untuk kontrol berlabel adalah **glyph + label**, bukan glyph menggantikan label.

### 2.3 Inventaris kontrol berteks pada halaman Provider (sebelum)

Dihitung dari pohon pada berkas yang dirender halaman Provider, sebelum satu baris diubah:

| Layar                                    | Kontrol berteks tanpa glyph                                                                                       | Jumlah |
| ---------------------------------------- | ----------------------------------------------------------------------------------------------------------------- | ------ |
| `/providers`                             | Previous, Next, Refresh now, Open (per baris), Add endpoint (per baris), Add Anthropic Compatible, Add OpenAI Compatible | 7      |
| Detail registry (`/providers/[id]`)      | Search, Clear filters, Disable (per baris), Enable (per baris), Remove (per baris), Copy, Add the model, Import from /models, Try again ×4, Add API Key, Add a connection, Previous, Next, Test endpoint, Save endpoint, Close | 19     |
| Detail node custom                       | Add API Key, Edit, Delete, Test the endpoint, Cancel, Save the provider, Refresh (per baris), Keep it, Remove the model | 9      |

Tiga string notifikasi memakai tanda centang Unicode `✓` sebagai pengganti ikon
(`+page.svelte:194-195`, `NodeModelsImport.svelte:104,108`), dan itu teks polos yang menyamar jadi glyph.

### 2.4 Kontrol ikon yang sudah ada (tidak diulang)

Aksi baris tabel kunci endpoint dan gateway sudah icon-only sejak keputusan owner 2026-09-24
(`EndpointKeysTable.svelte:39-42`, `GatewayKeyRow.svelte:30-35`), kontraknya di `tests/support/icon-only.ts`,
dan salinannya (`CopyButton`) punya bentuk `iconOnly`. Pass ini memperluas keluarga itu, bukan membangunnya.

## 3. Keputusan pass ini

| #   | Pertanyaan                                                        | Arah yang dipakai                                                                                                                                                                                                                                                       | Dipakai oleh |
| --- | ----------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| D1  | Di mana Search disaring?                                          | **Server.** `GET /api/v1/providers?q=` mencocokkan substring case-insensitive pada `id` atau nama tampilan. Reference menyaring di klien karena memuat seluruh daftar; §6.3 melarang panel melakukan itu, dan draft 025 sudah memilih arah server untuk kasus setara       | F1, F2       |
| D2  | Apa arti `q` kosong?                                              | **Tanpa filter.** Handler melakukan `TrimSpace` lebih dulu, lalu `q` kosong sama dengan tidak dikirim. Kotak kosong berarti "semua", bukan "cocok dengan string kosong"                                                                                                  | F1           |
| D3  | Berapa batas panjang `q`?                                         | **120 karakter**, divalidasi `validate:"omitempty,max=120"` seperti batas label lain di repo. Input eksternal wajib terbatas (AGENTS.md §1.4, OWASP A03)                                                                                                                  | F1           |
| D4  | Glyph + label atau icon-only?                                     | **Glyph + label untuk kontrol berlabel, icon-only untuk aksi baris.** Reference merender ikon + label; panel sudah mencatat penyimpangan icon-only khusus aksi baris (§8.11 butir 9). Kontrol yang berdiri sendiri mempertahankan labelnya agar nama aksinya terbaca         | F2, F3       |
| D5  | Apakah label "Category" di atas select dipertahankan?             | **Dipindah ke nama aksesibel.** Label bertumpuk itulah yang memecah simetri baris; `aria-label="Category"` menjaga nama kontrolnya. Nilai terpilih tetap terbaca dari select-nya sendiri                                                                                  | F3           |
| D6  | Apakah `✓` diganti?                                               | **Ya, jadi glyph `Check`.** Tanda centang Unicode adalah teks polos yang berdiri sebagai ikon; §8.11 butir 4 melarang simbol di string UI, dan R-04 meminta glyph nyata dengan alasan tertulis                                                                             | F4           |
| D7  | Apakah Enable/Disable model masuk keluarga icon-only?             | **Ya.** Keduanya aksi baris di dua tabel model, dan sebelum pass ini mereka satu-satunya aksi baris berteks di layar provider. Kontraknya sama dengan aksi kunci: nama di `aria-label` + `title`, glyph dekoratif                                                    | F3           |

## 4. Temuan

### 4.1 F1 (HIGH): `?q` tidak ada di route daftar provider

`ProviderListQuery` hanya mengenal `category` dan `routability`, dan handler membaca keduanya
(`app-serv/internal/schema/provider.go:25-28`, `internal/handler/provider.go:42-52`). Akibatnya §6.3
"search over name and ID" tidak bisa dipenuhi panel: parameter apa pun di luar dua itu diabaikan senyap
(§2.1), sehingga Search yang mengirim `?q` akan tampak bekerja sambil menampilkan seluruh registry. Ini
temuan tertinggi pass ini karena ia satu-satunya yang menyentuh wire.

### 4.2 F2 (MEDIUM): Search tidak ada di panel, dan §14 Q13 masih terbuka

§6.3 mencatat kontrolnya absen daripada palsu dan menggantung keputusan di §14 Q13 ("SPEC-API adds `?q` or
§6.3 drops the requirement"). Owner sudah memutuskan: Search dibangun. Pass ini menutup Q13 dengan arah
"SPEC-API menambah `?q`", dan mencatatnya di ketiga permukaan (route, kontrak, panel).

### 4.3 F3 (MEDIUM): 35 entri kontrol berteks tanpa glyph, dan tata letak filter tersebar

Inventaris §2.3 (7 di daftar, 19 di detail registry, 9 di detail node; satu kontrol yang muncul di dua
bentuk layar terhitung di keduanya). Dua di antaranya cacat tata letak yang owner sebut: label kategori bertumpuk dan Refresh
Now di baris terpisah, tanpa Search sama sekali. Sisanya ketidakcocokan dengan reference yang merender
ikon + label di setiap tombol aksi.

### 4.4 F4 (LOW): `✓` sebagai pengganti ikon di tiga string notifikasi

`+page.svelte:194-195` dan `NodeModelsImport.svelte:104,108`. Bukan emoji, tetapi tetap karakter teks yang
berdiri sebagai glyph; diganti glyph `Check` dari peta ikon yang sama.

## 5. Rencana DURING

1. **`app-serv` (F1).** `ProviderFilter` bertambah `Q`; pencocokan di `entries` pada `ID` dan
   `Display.Name`; handler membaca `q`, `TrimSpace`, validasi `max=120`; test table-driven di service dan
   handler (termasuk negative control: `q` yang tidak cocok mengembalikan 0 baris, bukan seluruh daftar);
   baris §7.4 diperbarui.
2. **`app-ui` peta ikon.** `src/lib/icons.ts` bertambah entri `CONTROL_ICONS` (`search`, `refresh`,
   `previous`, `next`, `add`, `addKey`, `import`, `open`) dan `ROW_ACTION_ICONS` (`edit`, `remove`), masing
   masing beralasan satu baris; alasan `save`, `cancel`, `delete`, `enable`, `disable`, `test` diperluas
   agar mencakup permukaan provider, bukan hanya kunci.
3. **`app-ui` daftar (F2, F3).** Toolbar satu baris: field Search (flex-1) + tombol Search, select
   Category ber-`aria-label`, dan Refresh Now, semuanya `min-h-11` dan `rounded-[var(--radius-sm)]`;
   Search mengirim `q` dan mengembalikan `page` ke 1; empty state menyebut kedua filter dan menyediakan
   "Clear the filters"; Previous/Next memakai chevron; tabel provider memberi glyph pada Open dan Add
   endpoint.
4. **`app-ui` detail (F3, F4).** Node card: Add API Key (glyph + label), Edit dan Delete (icon-only,
   Delete warna danger). Dialog: Save/Cancel berglyph. Model: Disable/Enable/Remove icon-only, Add the
   model dan Import from /models berglyph, Copy berglyph di bentuk berlabel. Connections: Add API Key dan
   Add a connection berglyph. Endpoint: Test endpoint, Save endpoint, Previous/Next berglyph. OAuth:
   Refresh berglyph. Notifikasi `✓` menjadi glyph `Check`.
5. **Test panel.** `providers-list.test.ts` bertambah kasus Search (mengirim `q`, reset page, empty state
   baru); setiap kontrol icon-only baru diuji dengan `expectIconOnly`; test yang lama diperbarui ke
   `getByRole`/`getByLabelText`.
6. **Gerbang.** `bun run test/check/lint/lint:ts/build` di `app-ui`; `go build/vet/test -race/gofmt` plus
   scrypts di `app-serv`; click-through hidup di panel port cadangan.

## 6. Implementasi

Semua item §5 dikerjakan. Set perubahan: **39 berkas, +666/-133** (termasuk kontrak dan SPEC).

### 6.1 `app-serv`: `?q` pada `GET /api/v1/providers` (F1, D1-D3)

- `internal/service/provider.go`: `ProviderFilter` bertambah `Q`; `entries` memangkas `needle :=
  strings.ToLower(strings.TrimSpace(filter.Q))` dan melompati baris yang gagal `matchesQuery`; helper
  `matchesQuery` mencocokkan substring case-insensitive pada `ID` atau `Display.Name` (D1), `needle`
  kosong berarti tanpa filter (D2).
- `internal/schema/provider.go`: `ProviderListQuery` bertambah `q` dengan
  `validate:"omitempty,max=120"` (D3): DTO ini sebelumnya terdefinisi tapi tidak pernah dibaca handler;
  kini ia kendaraan validasi yang sesungguhnya.
- `internal/handler/provider.go`: `List` membaca `q` (`TrimSpace`), memvalidasi lewat
  `schema.ValidateStruct(schema.ProviderListQuery{...})`, dan meneruskan `Q` ke filter.
- Kontrak: `docs/CONTRACT/001-CONTRACT-API-V1.yaml` `/api/v1/providers` bertambah parameter query `q`
  (`maxLength: 120`); `internal/handler/openapi.json` diregenerasi (`go run ./tools/openapi-gen`);
  `docs/SPEC-API/001-SPEC-API.md` §7.4 barisnya memuat semantik `q`.

**TDD.** Dua berkas uji baru, table-driven: `internal/service/provider_search_test.go`
(`TestProviderService_QueryMatchesIDOrName`, 8 kasus; `TestProviderService_QueryComposesWithTheOtherFilters`,
4 kasus termasuk jendela halaman-2) dan `internal/handler/provider_search_test.go`
(`TestProviderHandler_ListQueryParameter`, 11 kasus termasuk batas 120 diterima / 121 ditolak
`VALIDATION_ERROR`; `TestProviderHandler_ListQueryNeverLeaksTheUnfilteredSet` sebagai negative control).
Ketidak-hampaan (non-vacuity) dibuktikan dengan mutasi: filter selalu-true, pencocokan id-saja, dan
pelepasan `max=120` masing-masing merahkan kasus yang tepat, lalu dipulihkan.

### 6.2 `app-ui`: peta ikon (D4, D6, D7)

`src/lib/icons.ts`: `CONTROL_ICONS` bertambah `search`, `refresh`, `previous`, `next`, `add`, `addKey`,
`import`, `open`, `clear`; `ROW_ACTION_ICONS` bertambah `edit` (Pencil) dan `remove` (Trash2); alasan
`save`, `cancel`, `delete`, `enable`, `disable`, `test` diperluas agar mengatas nama permukaan provider.
Semua entri satu kalimat alasan (R-04); tidak ada glyph terlarang (zinciran `zap`/`bot`/`orb`/`cube`).

### 6.3 `app-ui`: halaman daftar (F2, F3)

`src/routes/providers/+page.svelte`: toolbar satu baris `flex flex-wrap items-stretch gap-2`: field
Search (`aria-label="Search providers by name or id"`) + tombol Search berglyph, select Category dengan
nama aksesibel `aria-label="Category"` (D5), `<RefreshControl>`; pola draft/applied seperti
ModelCatalogList; submit memotong spasi, mengirim `q`, dan mengembalikan `page` ke 1; `q` kosong tidak
dikirim; empty state menyebut kedua filter ("The category filter and the search…") dan menyediakan
"Clear the filters"; Previous/Next memakai chevron. `ProviderTable`: Open dan Add endpoint berglyph.
`RefreshControl`: glyph + label busy, `min-h-11`.

### 6.4 `app-ui`: layar detail, kedua bentuk (F3, F4)

- Detail registry: `ProviderConnectionsSection` (Add API Key = KeyRound, Add a connection = Plus,
  Hide the form = X; notifikasi merender glyph `Check` di samping teks), `ModelCatalogTable`
  (Disable icon-only, Pause), `ProviderDisabledModels` (Enable icon-only, Play), `ModelCatalogList`
  (Search/Clear filters), `NodeModelsImport` (Import = Download; `✓` di notifikasi menjadi glyph
  `Check` dengan flag `noticeOk`), `EndpointDetailDrawer` (Test endpoint = Activity, Close = X),
  `EndpointFieldsForm` (Save endpoint = Check), `CreateEndpointForm` (Add endpoint = Plus),
  `ProviderOAuthAccounts` (Refresh = RefreshCw).
- Detail node: `CustomProviderCard` (Add API Key glyph + label; Edit/Delete icon-only, Delete warna
  danger), `CustomProviderTest` (Activity), `CustomProviderDialog` dan `AddProviderKeysDialog`
  (Cancel = X, Save = Check), `CustomModelForm` (Plus), `CustomModelTable` (Remove icon-only;
  CopyButton baris), `CustomModelDeleteDialog` (Keep it = X, Remove the model = Trash2).
- `CopyButton`: bentuk berlabel mendapat glyph Copy di samping teks (melayani semua konsumennya,
  `/api-docs` hingga modal kunci sekali pakai). Notifikasi `providers/[provider_id]/+page.svelte`
  kehilangan `✓`-nya.

### 6.5 `app-ui`: uji

- `tests/support/providers-route-stub.ts`: stub kini menyaring `q` seperti API (substring
  case-insensitive pada id/nama), jadi panel dan stub berbohong dengan cara yang sama tidak mungkin.
- `tests/routes/providers-list.test.ts`: ditulis ulang (empty state) + 5 kasus Search baru (q terpotong
  + reset halaman, cocok nama-saja, submit kosong membuang parameter, q + kategori menyusun, Refresh
  mengulang q).
- Kontrak icon-only diperluas dengan `expectIconOnly` di `custom-provider-card-actions.test.ts`
  (Edit/Delete), `provider-models.test.ts` (Disable/Enable), `provider-custom-models.test.ts`
  (Remove); asersi `✓` diganti kalimat datar di `provider-detail-key-dialog.test.ts` dan
  `provider-custom-models-node.test.ts`.

## 7. Status pass ini

### 7.1 Gerbang

| Gerbang                                   | Hasil                                                                                  |
| ----------------------------------------- | -------------------------------------------------------------------------------------- |
| `app-ui` prettier (`bun run lint`)        | PASS                                                                                   |
| `app-ui` eslint (`bun run lint:ts`)       | PASS                                                                                   |
| `app-ui` svelte-check (`bun run check`)   | PASS, 0 error / 0 warning                                                              |
| `app-ui` build (`bun run build`)          | PASS, exit 0                                                                           |
| `app-ui` suite penuh (`bun run test`)     | PASS, **156 berkas / 2564 uji dalam 1511.67 s**                                        |
| `app-serv` gofmt / go vet / go build      | PASS / PASS / PASS                                                                     |
| `app-serv` go test                        | PASS, 15 paket (uji integrasi tidak dijalankan: DSN ter-tag tidak diekspor)             |
| scrypts `contract-openapi -check`         | PASS (artefak byte-identik dengan kontrak)                                             |
| scrypts `contract-drift`                  | PASS                                                                                   |
| scrypts `go-headers`                      | PASS                                                                                   |
| scrypts `go-lint` (golangci + staticcheck, juga varian ter-tag) | PASS                                                              |
| scrypts `secrets`                         | PASS                                                                                   |

Audit batas baris manual (tidak ada gerbang yang menegakkannya): semua berkas yang disentuh di bawah
250; **satu peringatan**: `internal/service/provider.go` kini 242 baris (peringatan di 220); pass ini
menambah 27 baris bersih ke berkas yang sebelumnya 215. Review: di bawah batas keras 250 dengan 8 baris
ruang; memindahkan predikat pencarian saja meninggalkan ~229 (masih berperingatan), jadi pemecahan
perhatian pencarian utuh difile sebagai residu, bukan digarap di detik terakhir.

Catatan kejujuran fingerprint: nilai yang dicatat sebelum peluncuran (`eda754cf…`) ternyata dihitung
sebelum suntingan terakhir pass (12:43-12:46); hash yang terverifikasi untuk pohon yang diuji adalah
`2b9c4cb8651ecaeef56655da233188f3` (497 berkas), dan sweep mtime membuktikan tidak ada berkas di bawah
`src tests static scripts` yang berubah setelah peluncuran (terbaru 12:46:55, peluncuran 12:51:48).

### 7.2 Click-through hidup (R-35), 22/22

Perangkat: salinan panel dari pohon pass ini di `/tmp/pass33/panel` (`rsync` + symlink `node_modules` +
`bun run build`, port **:3001**, `env -i` + `PANEL_API_TARGET=http://127.0.0.1:9091`), gateway kedua
dibangun dari pohon kerja yang sama (`go build -o /tmp/pass33/gw ./cmd/app-serv`, `HTTP_ADDR=
127.0.0.1:9091`, `.env` di-sumber); `git archive HEAD` sengaja TIDAK dipakai karena perubahan `?q`
belum ter-commit. Panel owner di `:3000` dan gateway `:9090` tidak disentuh. Bukti kawat sebelum
browser: login 204; `?q=openai` → 4 baris; `?q=zzzz-nothing` → 0; `q` 121 karakter → 400.

Driver CDP (`/tmp/pass33/driver.ts`, Chrome headless port 9231; 9229 dipegang `workerd` milik aktor
lain), **22/22 cek**:

- **A1-A3**: toolbar: field Search bernama aksesibel benar; Search, Category, dan Refresh now duduk
  pada satu baseline (tinggi `[44,44,44,44]`, top `[157,157,157,157]`); Search dan Refresh berglyph.
- **B1-B2**: submit Search mengirim `?q=openai` di kawat (`/api/v1/providers?page=1&per_page=25&q=openai`)
  dan merender 4 baris, angka yang sama dengan ukuran kawat §2.1.
- **C1-C2**: pencarian tanpa hasil menyebut kedua filter dan menyediakan "Clear the filters"; membersihkan
  mengembalikan 25 baris.
- **D1-D2**: Previous/Next berglyph chevron; Next pindah ke "Page 2 of 5" (112 baris, 25/halaman).
- **E1**: Refresh now mengulang pencarian yang tampil (`q=openai` terbawa).
- **F1-F2**: detail registry `openai`: Add API Key tetap berlabel dengan glyph; Disable katalog
  icon-only dengan nama di `aria-label`+`title`.
- **G1-G3**: detail node custom: Edit/Delete icon-only; Add API Key, Test the endpoint, Import
  berglyph.
- **G4-G7**: drawer endpoint dari baris tabel §6.2 (node memegang 3 endpoint): Test endpoint, Save
  endpoint, dan Close berglyph; Close menutup drawer.
- **G8**: `/api-docs`: 24 kontrol Copy, 24 berglyph (CopyButton bentuk berlabel).
- **H1**: 390 px (`mobile:false`): tidak ada luapan horizontal (`scrollWidth` 390 = viewport 390)
  setelah uji scroll.

Dua asersi pertama diarahkan ulang sebelum lari final, dengan sebab: openai adalah entri registry
dengan `endpoint_count` 0 dan node memegang 0 model, jadi drawer dan CopyButton **mustahil ada** di dua
fixture itu (jebakan fixture; mengukur kontrol yang tidak dirender bukan bukti). Keduanya digerakkan di
permukaan yang benar: drawer di node (G4-G7), Copy di `/api-docs` (G8). Delapan tangkapan layar di
`/tmp/pass33/shots/` (A…G) tersimpan sebagai artefak; angka geometris di atas adalah ukuran DOM, bukan
 silhouette.

### 7.3 Yang sengaja tidak dikerjakan

- **Rename**: tidak ada kontrol Rename di permukaan provider (daftar maupun detail); verb itu sudah
  icon-only di halaman kunci gateway sejak 2026-09-24. Tidak dikarang kontrol baru hanya demi daftar.
- **"Start the authorization" (OAuth)**: tetap teks penuh: aksi sekali-jalan berkalimat, bukan aksi
  baris; D4 menyimpulkan glyph + label untuk kontrol berlabel dan ia tidak termasuk keluarga icon-only.
- **Chip kapabilitas** (native/connector): teks informatif, bukan kontrol; dibiarkan berteks.

### 7.4 Residu

- Pemecahan `provider.go` di bawah garis peringatan 220 (§7.1).
- Perangkat click-through masih hidup setelah pass: gateway `:9091` dan panel salinan `:3001`
  (keduanya dari `/tmp/pass33`, PIDs dipegang task latar sesi ini), menunggu keputusan owner: dipakai
  untuk klik-ulang sendiri, atau dimatikan.
