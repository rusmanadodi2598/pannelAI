# 003-PORT-COMBO-VISION.md: Permukaan `/combos` (Combo & Vision Adapter) sejajar REFERENCE

Dokumen kerja pass `app-ui` untuk halaman **Combo & Vision Adapter**. Pass ini mengerjakan dua hal yang
owner sebut langsung: **revisi plain teks menjadi ikon** pada seluruh kontrol halaman Combo & Vision
(termasuk detail modal interaktif saat add combo dan add model vision), dan **tata letak compact +
simetris** pada layar itu. Bukan kontrak; kontrak tetap `docs/SPEC-API/001-SPEC-API.md` (wire) dan
`docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel). Pola mengikuti `001-PORT-ENDPOINT-KEYS.md` dan
`002-PORT-PROVIDER.md`: temuan bernomor F, bukti yang bisa diulang, keputusan di depan implementasi,
dan gerbang antislop di akhir.

|                      |                                                                                                                                                                                                                                                                                                                                                                                |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Status**           | **CLOSED 2026-09-25.** F1-F3 diimplementasi, D1-D6 dipakai, seluruh gerbang hijau, click-through hidup 29/29. Bukti gerbang dan click-through di §7; commit pekerjaan `d8a4d20`                                                                                                                                                                                                |
| **Mechanism**        | DURING & AFTER (antislop)                                                                                                                                                                                                                                                                                                                                                      |
| **Scope**            | `app-ui/.` (halaman `/combos`, kedua tab, dan setiap modalnya). `app-serv/.` tidak disentuh                                                                                                                                                                                                                                                                                    |
| **Permintaan owner** | (1) Revisi plain teks kontrol halaman Combo & Vision menjadi ikon, tanpa emoticon: Delete, Edit, Rename, Add Api Key, Test Endpoint, Save Endpoint, Save, Cancel, Prev, Next, Add Model, Copy, Remove, Refresh Now, dan yang relevan; termasuk detail modal interaktif saat add combo & vision. (2) Tata letak compact dan simetris. (3) Commit lokal setelah progres `CLOSED` |
| **Reference**        | `decolua/9router`, checkout `/home/rusmanadodi/apps/9router`, `39e36d3d` = tag `v0.5.86` (2026-09-23)                                                                                                                                                                                                                                                                          |
| **Kaitan**           | `docs/PORT/README.md` (keputusan desain); SPEC-UI §6.4, §8.4, §8.6, §8.11, §14 Q13; SPEC-API §7.6, §7.7, §7.8; DESIGN.md §2.1, §3.4, §10; `docs/RULLES/TDD.md`; draft 024 (combo/vision `app-serv`), draft 025 (picker + filter aktif)                                                                                                                                         |
| **Tanggal**          | 2026-09-25 (DURING)                                                                                                                                                                                                                                                                                                                                                            |

## 1. Ringkasan

Dua permintaan owner diukur dulu sebelum satu baris diubah, karena keduanya menunjuk sebab yang lebih
sempit dari kalimatnya.

| Yang owner sebut              | Yang diukur                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| ----------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Plain teks harus jadi ikon    | **Benar.** Tiga belas berkas permukaan Combo & Vision merender **23 tombol berteks tanpa glyph** (inventaris §2.2). Tujuh tombol sudah berglyph dan tidak diulang (`Up`/`Down` baris model, `Remove` baris model dan vision, `Move up`/`Move down`, tutup dialog bawaan `Modal`). Reference merender setiap aksi sebagai ikon + label (`Button icon="add"`, `combos/page.js:382`, `:421`, `:463`, `:474`, `:813`; `ComboFormModal.js:155`), dan permukaan provider sudah memakai bentuk itu sejak PORT 002 (SPEC-UI §8.11 butir 10)                                               |
| Tata letak belum compact      | **Benar.** Toolbar tab Combos menumpuk tiga blok: kalimat pengantar + tombol New combo, lalu `<RefreshControl>` di baris sendiri; di dalam editor, baris model memakai tiga tombol berlabel teks selebar 44 px masing-masing. Pada 390 px, satu baris model menghabiskan lebar penuh sebelum field `ref` sempat terbaca                                                                                                                                                                                                                                                           |
| Filter "hanya provider aktif" | **Bukan scope pass ini, dan sengaja tidak dikerjakan.** Server sudah punya `?active=true` (draft 025 CLOSED, terukur hidup menjawab 9 baris dari 4 provider), tetapi panel sudah punya aturan picker yang tertulis dan teruji: `endpoint_count > 0` atau `no_auth`, plus chip placeholder untuk provider ber-endpoint yang belum punya baris katalog (SPEC-UI §6.4, `pickerSections`). Memakai parameter itu mengganti perilaku yang terdokumentasi (placeholder terhapus, himpunan berbeda), yaitu keputusan desain yang bukan permintaan pass ini. Dicatat sebagai residu di §5 |

Jadi pass ini murni panel: memperluas keluarga ikon yang sudah ada ke permukaan Combo & Vision, dan
merapatkan dua tempat yang menumpuk. Tidak ada perubahan wire.

## 2. Bukti yang bisa diulang (diukur hidup, 2026-09-25)

Gateway owner berjalan di `127.0.0.1:9090`, sesi panel dibuka dengan `POST /api/v1/auth/login` (204 +
cookie) memakai `PANEL_BOOTSTRAP_PASSWORD` dari `app-serv/.env`. Panel owner di `:3000` tidak disentuh.
Angka di §2.1 hanya sebagai konteks keputusan §3 (F1/filter), bukan bahan implementasi.

### 2.1 Konteks picker (alasan filter aktif tidak dipakai)

```
GET /api/v1/models/catalog                  -> 937 baris, 91 provider
GET /api/v1/models/catalog?active=true      ->   9 baris,  4 provider
GET /api/v1/providers?per_page=100          -> 100 dari 112 baris
     endpoint_count > 0 atau no_auth        -> 6 provider
```

Kedua himpunan berbeda dan keduanya benar menurut aturannya masing-masing; selisih nyata hari ini
**0 chip yang salah tawar**, karena empat provider `no_auth` tanpa baris tidak punya baris katalog chat
sama sekali. Jadi tidak ada cacat yang ditutup dengan menggantinya, hanya perubahan bentuk.

### 2.2 Inventaris kontrol berteks tanpa glyph (sebelum)

Dihitung dari tiga belas berkas yang dirender `/combos`, sebelum satu baris diubah:

| Layar                                      | Kontrol berteks tanpa glyph                                  | Jumlah |
| ------------------------------------------ | ------------------------------------------------------------ | ------ |
| Tab Combos (`CombosTab.svelte`)            | New combo, Create the first combo, Try again, Previous, Next | 5      |
| Editor (`ComboEditor.svelte`)              | Add models, Create the combo / Save the combo, Cancel        | 3      |
| Baris model (`ComboModelRows.svelte`)      | Up, Down (per baris) + Add a model                           | 3      |
| Field judge (`ComboStrategyFields.svelte`) | Choose                                                       | 1      |
| Tabel combo (`ComboTable.svelte`)          | Edit, Test, Delete (per baris)                               | 3      |
| Dialog delete (`ComboDeleteDialog.svelte`) | Keep it, Delete the combo                                    | 2      |
| Dialog test (`ComboTestDialog.svelte`)     | Close, Test the chain / Test again / Testing                 | 2      |
| Tab Vision (`VisionAdapterForm.svelte`)    | Try again, Save the adapter                                  | 2      |
| Picker vision (`VisionModelPicker.svelte`) | Add models                                                   | 1      |
| Dialog picker (`ModelPickerDialog.svelte`) | Done                                                         | 1      |
| **Total**                                  |                                                              | **23** |

Tidak ada emoticon dan tidak ada karakter Unicode pengganti ikon di permukaan ini (diperiksa: 0 tanda
centang, 0 em dash di berkas komponen).

### 2.3 Bentuk kontrol reference

| Kontrol                                  | Reference                                     | Baris                                |
| ---------------------------------------- | --------------------------------------------- | ------------------------------------ |
| Create Combo                             | `Button icon="add"`                           | `combos/page.js:382`, `:421`         |
| Bulk Delete                              | `Button icon="delete"`                        | `combos/page.js:463`                 |
| Copy / Edit / Delete (per kartu)         | glyph + label kecil di bawahnya               | `combos/page.js:662`, `:672`, `:680` |
| Add Model (combo)                        | glyph `add` + label di tombol lebar kartu     | `ComboFormModal.js:155`              |
| Move up / Move down / Remove (per baris) | tiga tombol glyph dengan `title` sebagai nama | `ComboFormModal.js:38`, `:42`, `:46` |
| Add Model (adapter)                      | `Button icon="add"`                           | `combos/page.js:813`                 |
| Cancel / Save (modal)                    | teks saja, tombol penuh lebar                 | `ComboFormModal.js:161`, `:162`      |

Baris model reference memberi `title` (bukan label terlihat) pada ketiga aksi barisnya, yang mendukung
D2; modal combonya sendiri tidak berglyph, tetapi permukaan provider reference (`providers/[id]/page.js`)
memang berglyph, dan SPEC-UI §8.11 butir 10 sudah menetapkan satu bentuk untuk seluruh panel.

## 3. Keputusan pass ini

| #   | Pertanyaan                                                                      | Arah yang dipakai                                                                                                                                                                                                                                                                                             | Dipakai oleh |
| --- | ------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| D1  | Glyph + label atau icon-only?                                                   | **Glyph + label untuk kontrol berdiri sendiri, icon-only untuk aksi baris.** Sama dengan PORT 002 D4 dan SPEC-UI §8.11 butir 9/10; reference memakai kedua bentuk itu (`Button icon=...` untuk kontrol berdiri, tiga tombol `title` untuk aksi baris)                                                         | F1, F2       |
| D2  | Apakah `Up`/`Down` baris model masuk keluarga icon-only?                        | **Ya.** Ketiganya aksi baris, dan baris model adalah tempat paling padat di layar ini. Nama pindah ke `aria-label` + `title` (sudah ada `aria-label` untuk Move), glyph `ChevronUp`/`ChevronDown` menggantikan teks                                                                                           | F2           |
| D3  | Apakah Save/Cancel modal ikut berglyph walau modal combo reference tidak?       | **Ya, glyph + label.** SPEC-UI §8.11 butir 10 sudah menetapkan bentuk itu untuk seluruh panel (PORT 002), dan konsistensi satu panel lebih kuat daripada meniru satu berkas reference yang berbeda dari berkas reference lainnya                                                                              | F1           |
| D4  | Apakah picker pindah ke `?active=true`?                                         | **Tidak, sengaja tidak dikerjakan.** Aturan picker panel sudah tertulis dan teruji (SPEC-UI §6.4, `pickerSections`), termasuk placeholder dan klausa `no_auth`; menggantinya mengubah perilaku terdokumentasi dan bukan permintaan pass ini (draft 025 §5 sudah mencatatnya sebagai pekerjaan panel terpisah) | n/a          |
| D5  | Apakah baris tabel combo (`Edit`/`Test`/`Delete`) jadi icon-only?               | **Ya.** Baris aksi di permukaan provider sudah icon-only sejak PORT 001/002, dan tabel combo adalah satu-satunya tabel berteks yang tersisa di halaman ini                                                                                                                                                    | F1           |
| D6  | Apakah `Delete` dan aksi merusak lain memakai warna danger tanpa warna saingan? | **Ya.** Satu `text-*` per tombol, seperti kontrak `row-action-icons.test.ts`: dua utilitas warna bersaing diselesaikan urutan stylesheet, dan yang muted pernah menang diam-diam (terukur hidup 2026-09-24)                                                                                                   | F1           |

## 4. Temuan

### 4.1 F1 (MEDIUM): 23 kontrol berteks tanpa glyph

Inventaris §2.2. Sebagian besar aksi baris (`Edit`, `Test`, `Delete` di tabel combo) dan kontrol
berdiri sendiri (`New combo`, `Add models`, `Save`, `Cancel`, `Previous`, `Next`, `Try again`, `Choose`,
`Done`, `Keep it`, `Close`, `Add models` vision, `Save the adapter`).

### 4.2 F2 (LOW): toolbar dan editor belum compact

Tab Combos memakai dua blok vertikal untuk satu kalimat, satu tombol, dan satu kontrol refresh; baris
model memakai tiga tombol berlabel teks yang masing-masing 44 px, sehingga pada 390 px field `ref` tidak
punya ruang sampai baris membungkus. Tidak ada cacat fungsi, hanya kepadatan.

### 4.3 F3 (LOW): tombol `Try again` pada empty/error state berteks tanpa glyph

Tiga situs: `CombosTab.svelte` (error combos), `ComboDeleteDialog` tidak punya, dan `VisionAdapterForm`
(error adapter). Satu keluarga dengan F1; dicatat terpisah karena berbeda keadaan (bukan kontrol aksi
utama).

## 5. Rencana DURING

1. **`app-ui` peta ikon.** `src/lib/icons.ts`: `ROW_ACTION_ICONS` bertambah `test` (sudah ada), dan
   `CONTROL_ICONS` bertambah entri yang belum ada (`moveUp`/`moveDown` atau perluas `previous`/`next`,
   `choose`, `done`, `retry`, `addModel`, `close`). Setiap entri satu kalimat alasan (R-04, R-31), dan
   `tests/navigation/icons.test.ts` mengunci nama + alasan + larangan glyph terlarang.
2. **`app-ui` tab Combos (F1, F2).** Toolbar satu baris `flex flex-wrap items-stretch gap-2`: kalimat
   pengantar (flex-1) + New combo berglyph `Plus` + `<RefreshControl>`; empty/error state `Try again`
   berglyph `RefreshCw`; Previous/Next berglyph chevron seperti `/providers`.
3. **`app-ui` tabel combo (F1, D5, D6).** `Edit` (`Pencil`), `Test` (`Activity`), `Delete` (`Trash2`)
   jadi icon-only dengan `aria-label` + `title`, Delete warna danger tunggal.
4. **`app-ui` editor (F1, F2).** `Add models` berglyph `Plus`; baris model `Up`/`Down` icon-only
   (`ChevronUp`/`ChevronDown`), `Add a model` berglyph; field judge `Choose` berglyph; footer
   `Save`/`Create` berglyph `Check`, `Cancel` berglyph `X`.
5. **`app-ui` modal (F1).** `ComboDeleteDialog` (Keep it = X, Delete = Trash2), `ComboTestDialog`
   (Close = X, Test = Activity), `ModelPickerDialog` (Done = Check), `VisionModelPicker` (Add models =
   Plus), `VisionAdapterForm` (Save = Check, Try again berglyph).
6. **Test panel.** Setiap kontrol icon-only baru diuji dengan `expectIconOnly`; test lama yang
   mengalamatkan tombol berteks diperbarui ke `getByRole`/`getByLabelText`. Contract helper
   `tests/support/icon-only.ts` dipakai apa adanya.
7. **Gerbang.** `bun run test/check/lint/lint:ts/build` di `app-ui`; click-through hidup di panel port
   cadangan dengan driver CDP (di `/tmp`, tidak di repo).

## 6. Implementasi

Semua item §5 dikerjakan. Set perubahan: **13 berkas `app-ui`** (11 komponen, peta ikon, 2 test baru),
plus SPEC-UI. `app-serv` tidak disentuh, jadi gerbang Go tidak dijalankan.

### 6.1 `app-ui` peta ikon (D1, D2, D3)

`src/lib/icons.ts`: `ROW_ACTION_ICONS` bertambah `moveUp` (`ChevronUp`) dan `moveDown` (`ChevronDown`),
`CONTROL_ICONS` bertambah `choose` (`ChevronsUpDown`) dan `done` (`Check`). Setiap entri satu kalimat
alasan (R-04, R-31); `tests/navigation/icons.test.ts` mengunci nama, alasan, dan larangan glyph
terlarang, dan ia lulus tanpa perubahan karena memeriksa seluruh peta, bukan daftar tetap.

### 6.2 `app-ui` tab Combos (F1, F2, D5, D6)

`CombosTab.svelte` (259 → 243 baris) dan `CombosToolbar.svelte` (42 baris, baru): toolbar satu baris
berisi kalimat + New combo (glyph `Plus` + label) + `<RefreshControl>`, dengan `items-stretch` dan
`min-h-11` sehingga ketiganya satu baseline; Previous/Next berglyph chevron; `Try again` pada error
state dan `Create the first combo` pada empty state berglyph. Toolbar dipisah ke komponennya sendiri
saat pass ini mendorong berkas induk melewati ambang peringatan 220 baris, pada jahitan yang komentar
berkas itu sudah sebut.

`ComboTable.svelte`: `Edit` (`Pencil`), `Test` (`Activity`), `Delete` (`Trash2`) menjadi icon-only dengan
`aria-label` + `title` dan glyph `aria-hidden`; Delete memakai warna danger tunggal tanpa warna saingan
(D6). Ukuran target `min-h-11 min-w-11` (44 px).

### 6.3 `app-ui` editor (F1, F2, D2, D3)

`ComboEditor.svelte`: `Add models` berglyph `Plus`; `Save the combo`/`Create the combo` berglyph `Check`
dan `Cancel` berglyph `X`. `ComboModelRows.svelte`: `Up`/`Down` menjadi icon-only (`ChevronUp`/
`ChevronDown`) dengan nama `Move <ref> up` / `Move <ref> down` di `aria-label` dan `title`, tombol
`Remove` berglyph `Trash2` dengan warna danger, dan `Add a model` berglyph. `ComboStrategyFields.svelte`:
tombol `Choose` berglyph `ChevronsUpDown`.

### 6.4 `app-ui` modal (F1)

`ComboDeleteDialog` (Keep it = `X`, Delete the combo = `Trash2`), `ComboTestDialog` (Close = `X`, Test =
`Activity`), `ModelPickerDialog` (Done = `Check`), `VisionModelPicker` (Add models = `Plus`, Remove per
baris dengan target 44 px), dan `VisionAdapterForm` (Save the adapter = `Check`, Try again = `RefreshCw`).

**Satu bacaan click-through yang berhenti sebagai keputusan, bukan perbaikan.** Kontrol tutup bawaan
`Modal.svelte` terukur **36 px** di DOM (check H3). Bacaan pertama menyebutnya cacat terhadap target
sentuh 44 px R-03. Itu **salah**: `DESIGN.md` §7 menetapkan 36 px untuk kontrol di dalam dialog dan
44 px untuk sidebar, yang permukaan sentuh di setiap breakpoint, dan pass yang mengirim kontrol salin
icon-only sudah mencatat keputusan yang sama (`app-ui/README.md`, 2026-09-24: "The target is 36 px on
purpose... and the dialog close button's"). Mengubahnya diam-diam ke 44 px akan menimpa arah desain
owner (R-37), jadi kontrol itu **dibiarkan apa adanya** dan ukurannya dipin di
`tests/components/combo-vision-icons.test.ts` dengan alasan tertulis. Angka 36 px karena itu
dilaporkan sebagai temuan click-through yang ditolak, bukan ditutup.

### 6.5 `app-ui` uji

- `tests/components/combo-vision-icons.test.ts` (130 baris, baru): 9 kasus dari 5 blok `it`. Aksi baris
  tabel combo diuji dengan `it.each` atas tiga nama (TDD.md §2.5) dan tiga aksi baris model dengan
  `it.each`, plus warna danger Delete dan Remove.
- `tests/components/combo-dialog-icons.test.ts` (158 baris, baru): 6 kasus. Glyph+label pada footer tiga
  dialog, `Choose`, dua kontrol picker, plus asersi ukuran kontrol tutup `Modal` dengan alasan tertulis.
- `tests/components/combo-tab-toolbar.test.ts` (170 baris, baru): 6 test. Toolbar satu baris (satu
  parent, `flex-wrap`, `min-h-11`), glyph+label New combo dan Refresh now, chevron Previous/Next, dan
  paging yang benar-benar meminta `page=2` di kawat.
- Non-vacuity dibuktikan dengan mutasi: `aria-label="Edit"` dihapus dari `ComboTable` → tepat satu kasus
  merah (`renders Edit as a named icon`, pada asersi `title`); lalu dipulihkan dan hijau lagi.
- Berkas uji icon sempat 253 baris saat digabung; ia dipecah pada jahitan yang komentarnya sudah
  sebut (aksi baris vs footer dialog), mengikuti ambang §1.1 dan preseden `ed19008`.

## 7. Status pass ini

### 7.1 Gerbang

| Gerbang                                 | Hasil                                                                                                                                                                                                                    |
| --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `app-ui` prettier (`bun run lint`)      | PASS                                                                                                                                                                                                                     |
| `app-ui` eslint (`bun run lint:ts`)     | PASS, 0 error (satu unused import ditemukan dan dibuang)                                                                                                                                                                 |
| `app-ui` svelte-check (`bun run check`) | PASS, 0 error / 0 warning                                                                                                                                                                                                |
| `app-ui` build (`bun run build`)        | PASS, exit 0                                                                                                                                                                                                             |
| `app-ui` suite penuh (`bun run test`)   | lihat §7.3                                                                                                                                                                                                               |
| `scrypts/gates/panel-check.sh`          | prettier PASS, eslint PASS, svelte-check PASS; langkah `vitest` di gate itu bertabrakan dengan run suite terpisah (keduanya menjalankan vitest di pohon yang sama) dan keluar 143, jadi suite dijalankan sendiri di §7.3 |
| Batas baris                             | `CombosTab.svelte` 259 baris melewati batas keras 250 saat pass ini menambah glyph dan komentar; dipecah ke `CombosToolbar.svelte`, hasil 243 + 42. Semua berkas lain ≤ 247                                              |

### 7.2 Click-through hidup (R-35), 29/29

Perangkat: salinan panel dari pohon pass ini di `/tmp/pass34/panel` (`rsync` + symlink `node_modules` +
`bun run build`, port **:3002**), gateway dibangun dari pohon yang sama (`go build -o /tmp/pass34/gw
./cmd/app-serv`, `HTTP_ADDR=127.0.0.1:9092`). Panel owner di `:3000`, gateway `:9090`, dan perangkat
aktor lain di `:3001`/`:9091` tidak disentuh. Driver CDP di `/tmp/pass34/driver.ts` (Chrome headless port
9234), bukti di `/tmp/pass34/evidence.json` dan 7 tangkapan layar.

- **A1-A4**: toolbar: kalimat, New combo, dan Refresh now satu parent ber-`flex-wrap`, satu baseline
  (top `[221,221,221]`), dua kontrol `min-h-11` = 44 px, keduanya berglyph dan tetap berlabel.
- **B1-B4**: tabel combo: tiga aksi baris icon-only dengan `aria-label` + `title` + glyph
  `aria-hidden`, masing-masing 44 px, Delete memakai warna danger saja.
- **C1-C7**: editor `pi-agent`: tiga baris model, enam tombol move icon-only 44 px dengan nama
  `Move <ref> up|down` yang membawa ref aslinya, tiga tombol remove icon-only, dan Add models / Add a
  model / Save the combo / Cancel semuanya glyph + label pada 44 px.
- **D1-D3**: picker: Done glyph + label 44 px, 11 chip masing-masing 44 px, tujuh seksi terender
  (`Combos`, `Edge TTS`, `Google TTS`, `Local Device`, `SearXNG`, `OpenCode Free`, `TH HARBOR 1`).
- **E1-E2**: Escape menutup picker, Cancel menutup editor.
- **F1-F2**: tab Vision: Save the adapter dan Add models glyph + label 44 px.
- **G1**: picker vision: Done glyph + label.
- **H1-H3**: dialog delete: Keep it dan Delete the combo glyph + label 44 px, dan kontrol tutup dialog
  36 px. Angka 36 px itu **disengaja** (`DESIGN.md` §7, §6.4 pass ini): bacaan pertamanya menyebut cacat
  R-03, lalu dicabut setelah arah desain yang sudah tercatat dibaca; yang ditambahkan hanya pin test,
  bukan perubahan ukuran.
- **I1-I2**: 390 px: `scrollWidth` 390 = viewport 390, `scrollX` tetap 0, dan **0 elemen** di luar
  kontainer scroll (26 elemen memang lebih lebar dari viewport, seluruhnya di dalam satu pembungkus
  `overflow-x-auto` tabel combo, yaitu pola tabel panel sendiri).
- **J1**: 0 error konsol.

### 7.3 Suite penuh

`bun run test` dijalankan sendiri (tanpa vitest lain di pohon yang sama): **158 berkas, 2585 test,
seluruhnya lulus**, `EXIT=0`, 2185 dtk, pada pohon final (dengan ukuran kontrol tutup dialog
dikembalikan ke 36 px). Baseline sebelum pass ini 2564 test / 156 berkas (angka yang dicatat
`002-PORT-PROVIDER.md` §7.1), jadi pass ini menambah **21 kasus di 3 berkas uji**
(`combo-vision-icons.test.ts` 9, `combo-dialog-icons.test.ts` 6, `combo-tab-toolbar.test.ts` 6) dan tidak
menghapus satu pun: 2564 + 21 = 2585. Berkas uji icon sempat digabung (253 baris, 15 kasus) sebelum
dipecah pada ambang §1.1; pecahannya memindahkan 6 kasus tanpa mengubah jumlahnya, dan pasangan itu
dijalankan hijau (15 kasus) bersama toolbar (6).

Catatan kejujuran tentang satu kegagalan yang terlihat lalu hilang: run pertama saya bertabrakan dengan
run suite aktor lain di pohon yang sama, dan `tests/components/proxy-pools-save.test.ts` merah di bawah
kontensi itu. Berkas itu tidak disentuh pass ini; dijalankan sendiri ia lulus (9/9), dan pada run bersih
§7.3 ia lulus lagi. Kegagalannya milik kontensi, bukan pass ini, dan dicatat di sini supaya angkanya bisa
dibaca ulang.

### 7.4 Gerbang antislop (Delivery Gate)

Mode: DURING & AFTER. Setiap baris PASS dengan buktinya; baris yang tidak berlaku disebut alasannya.

**Block 1: Hard Gate**

| Aturan                        | Hasil | Bukti                                                                                                                                                                                                                                                                                                                             |
| ----------------------------- | ----- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| R-02 em dash                  | PASS  | `grep -cP "\x{2014}"` = 0 di ketiga belas berkas yang disentuh dan di dokumen ini (sembilan em dash yang terbawa kutipan kode reference dibuang sebelum commit)                                                                                                                                                                   |
| R-03 mobile                   | PASS  | Click-through I1-I2 pada 390 px: `scrollWidth` 390 = viewport 390, `scrollX` 0 setelah percobaan scroll 400 px, 0 elemen di luar kontainer scroll. Target sentuh: seluruh kontrol baru yang berdiri sendiri `min-h-11` (44 px), terukur di A3/B3/C2/C5/C6/C7/D1/D2/F1; kontrol tutup dialog 36 px mengikuti `DESIGN.md` §7 (§6.4) |
| R-17 angka tanpa sumber       | PASS  | Setiap angka di dokumen ini punya perintahnya (GET live, `wc -l`, `grep -c`). Tidak ada statistik produk                                                                                                                                                                                                                          |
| R-18 testimoni                | N/A   | Tidak ada permukaan testimoni di panel operator                                                                                                                                                                                                                                                                                   |
| R-23 aset tanpa instruksi     | N/A   | Pass ini tidak membuat logo, avatar, atau statistik; ikon diambil dari set yang sudah dipasang (`@lucide/svelte`) dan dicatat alasannya                                                                                                                                                                                           |
| R-24 nav tanpa tujuan         | PASS  | Pass ini tidak menyentuh `navigation.ts`; `/combos` sudah punya rute, dan `tests/navigation/navigation.test.ts` menguncinya                                                                                                                                                                                                       |
| R-25 kontras                  | PASS  | `tests/tokens/contrast.test.ts` 60 asersi lulus, termasuk pasangan teks yang dipakai kontrol baru (teks muted di surface, danger di surface)                                                                                                                                                                                      |
| R-26 kontrol mati             | PASS  | Setiap kontrol baru benar-benar bertindak: click-through mengklik Edit (C), Add models (D), Save the adapter (F), Delete (H), Cancel (E2), dan Escape (E1); tidak ada kontrol yang hanya terlihat                                                                                                                                 |
| R-27 empty/loading/error      | PASS  | Tab Combos merender ketiganya (`StateMessage`), tab Vision juga; test baru mengunci `Try again` berglyph pada error state dan `Create the first combo` berglyph pada empty state                                                                                                                                                  |
| R-28 FAQ                      | N/A   | Tidak ada FAQ                                                                                                                                                                                                                                                                                                                     |
| R-32 keyboard                 | PASS  | Probe keyboard 7/7: kontrol icon-only menerima fokus (K1), indikator fokus terlihat `outline 3px` dan bukan `outline: none` (K2), Enter mengaktifkan Edit (K3), Cancel terjangkau dan aktif lewat keyboard (K4-K5), Escape menutup picker (E1) dan dialog (H)                                                                     |
| R-33 patch via skrip          | PASS  | Tidak ada skrip yang menyunting sumber; seluruh perubahan ditulis di berkas sumbernya                                                                                                                                                                                                                                             |
| R-34 kedua tema               | PASS  | Probe T-light dan T-dark: kontrol yang sama terender, `scrollWidth` = viewport di keduanya, warna teks `rgb(26,22,20)` di `rgb(252,250,247)` dan `rgb(237,234,230)` di `rgb(26,25,23)`                                                                                                                                            |
| R-35 verifikasi sebelum kirim | PASS  | Build dijalankan, 29/29 click-through elemen-per-elemen + 7/7 probe keyboard/tema, 0 error konsol (J1). Daftar per elemen ada di §7.2                                                                                                                                                                                             |
| R-36 klaim fabrikasi          | PASS  | Tidak ada klaim keamanan, kepatuhan, atau performa di dokumen ini                                                                                                                                                                                                                                                                 |
| R-37 arah desain              | PASS  | `DESIGN.md` ada dan dibaca; satu bacaan click-through (kontrol tutup 36 px) dicabut setelah arah desain tercatat dibaca, dan tidak ada arah yang ditimpa diam-diam (§6.4)                                                                                                                                                         |
| R-38 konten nyata             | PASS  | Semua angka berasal dari pengukuran pass ini; tidak ada data yang dikarang                                                                                                                                                                                                                                                        |

**Block 2: Purpose-Gate**

| Aturan                                                           | Hasil | Bukti                                                                                                                                                                                                                                 |
| ---------------------------------------------------------------- | ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| R-04 ikon relevan                                                | PASS  | Setiap glyph baru satu kalimat alasan di `src/lib/icons.ts` (`moveUp`, `moveDown`, `choose`, `done`), dan `tests/navigation/icons.test.ts` menolak glyph terlarang serta alasan < 20 karakter. Tidak ada sparkle/star/magic/robot/orb |
| R-01, R-06, R-07, R-08, R-09, R-10, R-12, R-13, R-14, R-19, R-22 | N/A   | Pass ini tidak menambah gradien, glow, font, latar bermotif, badge, glassmorphism, bayangan, animasi, atau ilustrasi. Ikon di sini bukan hiasan: ia menggantikan teks aksi yang sudah ada                                             |

**Block 3: Liveliness**

| Pertanyaan                 | Hasil | Bukti                                                                                                                             |
| -------------------------- | ----- | --------------------------------------------------------------------------------------------------------------------------------- |
| Dial eksplisit             | PASS  | `DESIGN.md` §2.1: ENERGY 1 / RHYTHM 2 / MOTION 1                                                                                  |
| Konsisten dengan dial      | PASS  | RHYTHM 2: shell uniform, isi layar bervariasi; toolbar yang dirapatkan mengikuti bentuk `/providers` (PORT 002), bukan pola baru  |
| Satu focal point per layar | PASS  | Toolbar: New combo adalah aksi utama (aksen penuh); aksi baris deferred ke glyph muted, Delete danger                             |
| Whitespace struktural      | PASS  | Gap 2/4/5 tetap dari skala `DESIGN.md` §7; yang berubah hanya pengelompokan, bukan jarak                                          |
| Satu aksen sengaja         | PASS  | Aksen hanya pada aksi utama (New combo, Save), tidak disebar ke glyph aksi baris                                                  |
| Motif identitas            | PASS  | Bentuk glyph + label untuk kontrol berdiri dan icon-only + `aria-label`/`title` untuk aksi baris, dipakai ulang dari PORT 001/002 |
| Design Read dideklarasikan | PASS  | `DESIGN.md` §1: "instrument style dengan satu aksen coral"                                                                        |

**Block 4: Craftsmanship & Quality Locks**

| Aturan                                               | Hasil | Bukti                                                                                                                                                                                      |
| ---------------------------------------------------- | ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| C-1 intentionality                                   | PASS  | Setiap keputusan punya satu baris alasan (§3 D1-D6); yang tidak bisa ditulis alasan tidak dikerjakan (D4)                                                                                  |
| C-2 functional completeness                          | PASS  | Setiap kontrol baru diklik hidup (§7.2)                                                                                                                                                    |
| C-3 content-driven                                   | PASS  | Tidak ada seksi baru; yang dikerjakan adalah bentuk ulang kontrol yang sudah ada                                                                                                           |
| C-4 resilience                                       | PASS  | Tiga state, dua tema, 390 px, dan keyboard diuji (§7.2, ditambah probe keyboard 7/7 di §7.4)                                                                                               |
| C-5 evidence over claims                             | PASS  | Seluruh angka pass ini berasal dari pengukuran; kegagalan kontensi yang terlihat dicatat apa adanya (§7.3)                                                                                 |
| R-05, R-11, R-15, R-16, R-20, R-21, R-29, R-30, R-31 | PASS  | Tidak ada template layout, CTA marketing, buzzword, atau klon produk yang ditambahkan; radius, palet, dan tema tetap dari `DESIGN.md`; setiap entri ikon baru punya alasan tertulis (R-31) |

### 7.5 Residu

- **Picker belum memakai `?active=true`** (draft 025 §5 sudah mencatatnya sebagai pekerjaan panel
  terpisah, dan D4 memutuskan tidak mengerjakannya di pass ini): aturan panel tetap terdokumentasi dan
  teruji apa adanya. Kalau owner mau penyaringan pindah ke server, itu pass tersendiri dengan perubahan
  perilaku yang harus ditulis di SPEC-UI §6.4 lebih dulu.
- **Perangkat click-through masih hidup** setelah pass: gateway `:9092` dan panel salinan `:3002`
  (keduanya dari `/tmp/pass34`), menunggu keputusan owner: dipakai untuk klik-ulang sendiri, atau
  dimatikan.
- **`app-ui/README.md` diperbarui di pass yang sama**: entri ke-37 dan satu bagian
  ("The Combo & Vision icons and the compact toolbar, 2026-09-25") membawa angka suite, click-through,
  dan bacaan 36 px yang berhenti sebagai keputusan itu.

## 8. Status final

**CLOSED 2026-09-25.** F1 (23 kontrol berteks menjadi glyph; keluarga icon-only diperluas ke aksi baris
tabel combo dan baris model), F2 (toolbar satu baris dan baris model dirapatkan), dan F3 (`Try again`
berglyph) selesai. D1-D6 dipakai. Click-through hidup 29/29 menemukan satu bacaan yang berhenti sebagai
keputusan, bukan perbaikan: kontrol tutup dialog 36 px adalah arah desain `DESIGN.md` §7, jadi ia
dibiarkan dan dipin test dengan alasan tertulis (§6.4), bukan diubah ke 44 px secara diam-diam. D4
mencatat satu hal yang sengaja TIDAK dikerjakan: picker tetap memakai aturan dua-bacaan yang
terdokumentasi, bukan `?active=true`.

Seluruh gerbang §7.1 hijau, suite penuh 158 berkas / 2585 test (`EXIT=0`), click-through hidup 29/29
ditambah probe keyboard/tema 7/7, semuanya terekam di `/tmp/pass34/` (`evidence.json`,
`evidence-keyboard.json`, dan tangkapan layar). `CombosTab.svelte` dipecah pada ambang §1.1 (259 →
243 + `CombosToolbar.svelte` 42); semua berkas lain ≤ 247 baris. Commit pekerjaan dicatat pada commit
dokumen berikutnya, mengikuti pola `001` dan `002`.
