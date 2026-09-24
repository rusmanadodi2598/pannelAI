# 025-COMBO-PICKER-PARITY.md: Picker Combo & Vision hanya menawarkan yang bisa menjawab (app-ui)

Dokumen kerja pass panel `app-ui` atas permukaan picker **Combo** dan **Vision Adapter** terhadap
reference `decolua/9router`. Bukan kontrak; kontrak tetap `docs/SPEC-UI/001-SPEC-UI.md` §6.4 (perilaku
panel) dan `docs/SPEC-API/001-SPEC-API.md` §7.6 (katalog). Pola mengikuti draft 016 sampai 024: temuan
bernomor F, bukti yang bisa diulang, gerbang terukur, dan keputusan yang diambil di depan.

| | |
|---|---|
| **Status** | **CLOSED 2026-09-24**: F1 dan F2 **CLOSED** di panel; F3 (filter aktif sisi server) **OPEN** sebagai permintaan `app-serv`, yang sudah membuka draftnya sendiri (nomor sama, §3.3); F4 non-finding. Gerbang dan click-through terekam: §4 |
| **Mechanism** | DURING & AFTER (antislop) |
| **Scope** | `app-ui/.` saja. `app-serv/.` tidak disentuh; satu permintaan difilekan (F3) |
| **Permintaan owner** | "Combo & Vision Adapter, pada REFERENCE bagian pembuatan combo models ini adalah bisa memfilter provider+models hanya dari yang active pada saat itu. Pada aplikasi kita belum menampilkan by filtering tersebut, dan malah menampilkan semua placeholder yang bahkan belum sama sekali di tambah model beserta provider nya." Keputusan owner: bentuk **modal ala reference**, panel dikerjakan sekarang, dan filter server-side **diajukan** ke `app-serv` |
| **Reference** | `decolua/9router`, checkout `/home/rusmanadodi/apps/9router`, `21583c03` = tag `v0.5.85` (2026-09-22) |
| **Kaitan** | SPEC-UI §6.4; draft 024 §5 (penyerahan pekerjaan picker ke panel), §3.7 F7 (no_auth tetap butuh endpoint), §3.4 F4 (servability di jalur tulis); draft 019 (permukaan provider) |
| **Tanggal** | 2026-09-24 |

## 1. Ringkasan

Keluhan owner diukur lebih dulu terhadap gateway yang berjalan (login sesi panel, `:9090`): yang
ditawarkan picker sebelum pass ini adalah **seluruh katalog**, sementara yang bisa menjawab adalah
**satu baris**.

| Sumber | Sebelum (terukur hidup) | Sesudah |
|---|---|---|
| Katalog chat | **587 ref / 67 provider** | dibaca sama; yang ditawarkan **1** |
| Provider aktif (`endpoint_count > 0`) | **1** dari 86 baris (`openai-compatible-0386BKG9Q4DYZPYC01VJH4C51G`, nama `TH HARBOR 1`, 3 endpoint) | sama |
| Ref katalog milik provider aktif | **1** | **1** (ditawarkan) |
| Combo tersimpan | **0** | 0 (bagian Combos tetap ada saat ada isinya) |
| Katalog `capability=vision` | **248 ref / 42 provider** | dibaca sama; yang ditawarkan **0** |
| Ref vision milik provider aktif | **0** | **0**, dan dialog menyebut sebabnya |
| Provider `no_auth` | **6** (tidak satu pun punya endpoint) | tidak ditawarkan (F1 §3.1) |
| Baris media di katalog | **135** dari 587 (image 56, embedding 30, tts 26, stt 21, video 2) | tidak ditawarkan (F2 §3.2) |
| Provider `routability: connector` | **11** dari 86 | tidak ditawarkan (F2 §3.2) |

Jadi dari 587 chip yang dulu bisa diklik, **586 di antaranya tidak akan pernah menjawab** (tidak ada
endpoint untuk providernya, providernya butuh connector, atau modelnya media), dan 248 chip vision
seluruhnya tidak akan pernah menjawab. Pass ini membuat picker menawarkan yang bisa menjawab saja,
dengan bentuk yang sama seperti reference, dan menyebut sebabnya ketika tidak ada yang bisa
ditawarkan.

## 2. Aturan reference dan aturan panel (baris terverifikasi)

Empat aturan reference yang dipakai pass ini, semuanya dibaca dari `origin/master` `21583c03`:

1. **Hanya provider yang punya koneksi.** `ModelSelectModal.js:216-219` membangun
   `providerIdsToShow = new Set([...activeConnectionIds, ...noAuthIds])` dari `activeProviders` yang
   halaman combos baca sekali (`combos/page.js:156-170`, `connections` di `:170`) lalu teruskan ke
   setiap modal: anggota combo `:1126-1141`, adapter `:826-835`.
2. **Placeholder untuk provider terhubung yang belum punya model.**
   `ModelSelectModal.js:335-339`: node compatible yang terhubung tanpa alias mendapat satu chip
   putus-putus bernilai `prefix/model-id` (`isPlaceholder: true`).
3. **Filter kapabilitas membuang provider yang tidak punya modelnya, dan membuang combos.**
   `ModelSelectModal.js:448-451` (`capFilter`) dan `:427` (`if (kindFilter || capFilter) return [];`).
   Adapter vision memakai jalur ini: `combos/page.js:826-835` mengirim `capFilter={cap.key}` dan
   `closeOnSelect={false}`. Filter kapabilitas juga **membuang placeholder** sebagai efek samping
   (ref placeholder tidak punya caps yang dilaporkan).
4. **Judul seksi yang cocok mencari seluruh isinya.** `ModelSelectModal.js:453-459`: query yang
   mengenai nama provider mengembalikan semua opsinya. Judge memakai modal yang sama dalam mode satu
   pilihan (`combos/page.js:688-698`).

Tiga aturan panel yang menyertainya, semuanya dari berkas yang bisa dibuka:

- **"Aktif" = punya endpoint.** `selection.go:141-143` menjawab `NO_PROVIDER_AVAILABLE` begitu
  `len(endpoints) == 0`, termasuk untuk provider `no_auth` (draft 024 §3.7 F7). Karena itu picker
  panel memakai `endpoint_count > 0` **tanpa** klausa "atau no_auth" milik reference: menawarkan
  provider `no_auth` tanpa baris akan menawarkan chip yang pasti gagal.
- **Provider ber-connector tidak dilayani chat.** `registry/index.go:126-140`
  (`ChatRoutability`/`IsChatRoutable`, nilai `native`/`connector`) dan jalur tulis menolaknya
  (`PROVIDER_NOT_ROUTABLE`, draft 024 F4).
- **Model media bukan model chat.** `registry/types.go:131-140`: `IsChat()` benar untuk kind kosong,
  `llm`, atau `chat`; jalur tulis combo/vision menolak sisanya dengan alasan yang disebut
  (draft 024 F4). `routability` adalah himpunan tertutup di schema (`schema/provider.go:27`).

## 3. Findings

### 3.1 F1 (HIGH, FE): picker menawarkan seluruh katalog, bukan yang aktif

**Status: CLOSED.**

**Fakta (sebelum).** `CombosTab.svelte` membangun saran dari `listModelCatalog({})`, 587 ref,
dan `VisionAdapterForm.svelte` dari `listModelCatalog({ capability: 'vision' })`, 248 ref, lalu
keduanya merendernya sebagai `<datalist>`/daftar checkbox. Tidak ada satu pun pembacaan
`GET /providers` di kedua layar, jadi tidak ada cara tahu provider mana yang punya endpoint.

**Yang dikerjakan.**

1. `src/lib/schemas/model-picker.ts`, derivasi bersama: `activeProviderIds` (`endpoint_count > 0`),
   `pickerSections` (combos lebih dulu, lalu satu seksi per provider aktif dengan opsi diurut label),
   `filterPickerSections` (pencarian), `pickerOptions`.
2. `src/lib/model-picker-data.ts`, pembacaan sumber: katalog + daftar provider, halaman provider
   dibaca berulang sampai `meta.total` terpenuhi (`per_page=100`, SPEC-API §4), dan satu kegagalan
   dilaporkan sebagai `failed` alih-alih daftar aktif yang diam-diam lebih pendek.
3. `src/lib/components/ModelPickerDialog.svelte`, satu modal untuk kedua tab: pencarian, chip
   `aria-pressed`, hitungan per seksi, chip placeholder putus-putus, kalimat gagal-baca, dan kalimat
   kosong dari pemanggilnya.
4. `ComboEditor.svelte` + `ComboStrategyFields.svelte` + `ComboModelRows.svelte`, editor memakai
   modal untuk anggota dan untuk judge (mode satu pilihan), dan input teks per baris tetap ada:
   ref yang picker belum bisa tawarkan tetap bisa diketik.
5. `VisionAdapterForm.svelte` + `VisionModelPicker.svelte`, tab vision memakai modal yang sama
   (`placeholders: false`, `combos: []`), baris terpilih menampilkan ref + tombol hapus, dan ref
   yang tidak lagi ditawarkan picker tetap tampil dengan alasannya.

**Bukti.** 15 test derivasi (`tests/schemas/model-picker.test.ts` 11 +
`tests/schemas/model-picker-filter.test.ts` 4), 10 test modal
(`tests/components/model-picker-dialog.test.ts`), 4 test loader
(`tests/api/model-picker-data.test.ts`), 4 test tab vision
(`tests/components/vision-adapter-picker.test.ts`), 9 test editor (`combo-editor.test.ts`) + 3 test
daftar model (`combo-editor-models.test.ts`), 2 test penjaga draf, 5 test tab delete, dan 10 test tab
probe. Yang menegakkan aturannya lewat render penuh ada di `combo-delete.test.ts`: `GPT-4o` dan `daily`
tampil, `MiMo v2.6` (provider `oczen`, endpoint 0) tidak.

### 3.2 F2 (LOW, FE): baris media dan provider ber-connector ikut ditawarkan

**Status: CLOSED** (dikerjakan dalam pass yang sama).

**Fakta.** Katalog memuat 135 baris media dari 587, dan daftar provider memuat 11 provider
`routability: connector` dari 86. Keduanya ditolak jalur tulis dengan alasan konkret (draft 024 F4:
`PROVIDER_NOT_ROUTABLE`, "is a media model (image), not a chat model"), jadi chip untuk keduanya
adalah penyimpanan yang tidak mungkin berhasil. Terukur hidup: dari 587 baris, **0** baris media
milik provider aktif dan **0** provider aktif ber-connector, jadi cacat ini belum bisa dijangkau
hari ini, tetapi aturannya satu keluarga dengan F1 dan sudah diserahkan draft 024 F4 ke panel
("`kind` sudah tersaji di baris katalog sehingga panel bisa menyaring sendiri").

**Yang dikerjakan.** Dua baris aturan di `pickerSections`, keduanya membaca field yang sudah ada di
baris yang panel punya: provider dengan `routability !== 'native'` dilewati, dan baris katalog dengan
`kind` di luar `'' | llm | chat` tidak dijadikan opsi. Test: satu test per aturan di
`tests/schemas/model-picker.test.ts`, plus satu baris `kind: 'image'` dengan `capabilities: ['vision']`
di test tab vision, model yang bisa membaca gambar tetap bukan model chat, jadi tidak ditawarkan.

### 3.3 F3 (LOW, BE): filter aktif sisi server

**Status: OPEN, permintaan `app-serv`.**

**Fakta.** Panel membaca dua rute lalu menggabungkannya di klien: `GET /models/catalog` (587 baris,
tanpa parameter keaktifan) dan `GET /providers` (86 baris, berpaginasi). Gabungan itu benar, tetapi
ia berarti panel adalah satu-satunya tempat aturan "aktif" hidup, dan dua pembacaan penuh dikirim
setiap kali picker dibuka.

**Permintaan.** Satu parameter `active=true` di `GET /api/v1/models/catalog` (SPEC-API §7.6) yang
menyaring baris ke provider yang **punya minimal satu endpoint ber-status `active`**, yaitu kandidat
yang benar-benar dipilih router (`selection.go:201-203`, `EndpointFilter{Status: active}`;
`selection.go:141-143` menolak ketika daftar kandidat itu kosong). Kalau itu ada, panel bisa membaca
satu rute untuk picker dan aturan "aktif" hidup di satu tempat. Alternatif yang sama-sama menutup
temuan: menerbitkan `endpoint_count` per baris katalog, sehingga panel tetap menggabungkan tetapi
tidak lagi menebak.

**Sudah diambil `app-serv`.** Draft `025-CATALOG-ACTIVE-FILTER.md` (nomor sama, kolisi penomoran
dengan draft ini; dilaporkan ke owner) sudah DURING untuk parameter yang sama dengan definisi yang
lebih tajam: **≥1 endpoint ber-status `active`** lewat satu query
`EndpointStatusCountsByProvider`. Catatan presisi yang perlu dibaca saat panel memakainya:
`EndpointStatusCounts.Add` menghitung endpoint `active` yang sedang rate-limited ke `RateLimited`,
**bukan** `Active` (`domain/endpoint_counts.go:37-53`), sedangkan router tetap menganggapnya kandidat;
jadi himpunan yang persis sama dengan router adalah `active > 0` **atau** `rate_limited > 0`.

**Sampai itu ada,** panel memakai jalur dua-bacaan di atas dengan `endpoint_count > 0`, yang adalah
aturan reference sendiri (route `/api/providers`-nya mengembalikan **semua** baris koneksi tanpa
filter status, `src/app/api/providers/route.js:52`). Deviasinya hanya bentuk, dan selisih himpunannya
hanya provider yang **seluruh** endpoint-nya `disabled`/`error`; terukur hidup 2026-09-24: **0 dari 86
baris** (satu-satunya provider ber-endpoint, `TH HARBOR 1`, 3/3 `active`), jadi tidak ada chip yang
salah tawar hari ini.

### 3.4 F4 (non-finding): picker vision kosong pada data hari ini

**Status: non-finding, dicatat.**

Provider aktif hari ini satu (`TH HARBOR 1`) dan katalog `capability=vision` tidak memuat satu baris
pun untuknya, jadi picker vision **kosong** setelah pass ini. Itu bukan cacat pass ini melainkan
konsekuensi aturan yang owner minta: reference pun membuang placeholder di bawah filter kapabilitas
(`ModelSelectModal.js:448-451`), sehingga picker vision-nya juga kosong pada keadaan yang sama.
Dialog menyebut sebab dan jalan keluarnya, dan baris adapter yang sudah tersimpan tetap tampil
dengan label "not reported as vision-capable by the catalog" supaya konfigurasi lama tidak hilang
diam-diam.

## 4. Gerbang

Semuanya dijalankan pada working tree akhir (fingerprint `1bebaefd3ae1d65d79c5996499238996`, 500
berkas di bawah `src/`, `tests/`, `static/`, `scripts/`):

| Gerbang | Hasil |
|---|---|
| `bun run check` | **0 errors, 0 warnings** |
| `bun run lint` | Prettier: seluruh berkas sesuai, setelah satu `prettier --write` atas berkas pass ini |
| `bun run lint:ts` | ESLint keluar 0 |
| `bun run build` | sukses, keluaran di `build/` |
| Test terarah | **10 berkas, 62 test lulus** dalam 2m34s |
| `vitest run` penuh | **157 berkas, 2569 test lulus** dalam 1658.89 s pada tree beku `1bebaefd3ae1d65d79c5996499238996` (500 berkas; fingerprint diukur ulang setelah run, sama) |
| Click-through browser | **lulus**: satu sesi terekam, panel melayani build pass ini di `:3002` menunjuk gateway hidup `:9090`. Picker combo merender tepat satu seksi `TH HARBOR 1 (1)` dengan satu chip `th-1` dan layar meminta `/providers?page=1&per_page=100` + `/models/catalog`; picker vision tidak merender seksi dan menampilkan kalimat dua jalan keluarnya, dengan `/vision-adapter`, `/providers?page=1&per_page=100`, `/models/catalog?capability=vision`. Tiga screenshot, bukti di `/tmp/picker31/evidence.json` |

## 5. Yang sengaja TIDAK dikerjakan

- **Filter aktif sisi server**, difilekan sebagai F3, bukan dibangun di panel; `app-serv` bukan
  scope pass ini.
- **Tiga adapter lain (pdf, audio-input, video-input)**, tetap tidak ada, sesuai SPEC-UI §6.4.
- **`disabled={!enabled}` pada tombol Add Model adapter** dan pemotongan chip menjadi tiga di kartu
  adapter (reference `combos/page.js:730-838`), perilaku tampilan yang tidak menyentuh aturan
  penawaran; tidak disalin.
- **Klausa "atau no_auth" reference**, lihat §2; pada gateway ini provider `no_auth` tetap butuh
  baris endpoint, jadi klausa itu akan menawarkan chip yang pasti gagal (draft 024 §3.7 F7).
- **Mengubah `ref` yang tersimpan menjadi kanonik**, ref disimpan apa adanya, karena router
  menerima ketiga bentuk nama (draft 024 §2 dan F2).

## 6. Keputusan yang diambil pass ini (ruling)

1. **Penyaring "aktif" = `endpoint_count > 0`, tanpa klausa no_auth.** Alasannya terukur: gateway
   ini memilih baris endpoint sebelum merutekan, jadi provider `no_auth` tanpa baris menjawab
   `NO_PROVIDER_AVAILABLE` (draft 024 §3.7 F7). Menyalin klausa reference akan menawarkan chip yang
   pasti gagal, persis cacat yang pass ini hapus.
2. **Dua aturan servability (F2) dikerjakan sekarang, bukan difilekan.** Keduanya satu keluarga
   dengan F1 ("jangan tawarkan yang tidak bisa menjawab"), datanya sudah ada di baris yang panel
   baca, dan draft 024 F4 sudah menyerahkannya ke panel.
3. **Placeholder hanya di picker combo.** Di bawah filter kapabilitas ia dibuang, karena chip
   kosong di daftar "model vision" menawarkan ref yang kapabilitasnya belum dikonfirmasi apa pun.
4. **Kedua tab berbagi satu modal dan satu derivasi.** Dua layar yang menjawab pertanyaan yang sama
   tidak boleh punya dua jawaban; `loadPickerSources` mengembalikan sumbernya, pemanggil menurunkan
   seksinya, sehingga bagian Combos tidak bisa melenceng dari daftar milik tab itu.

## 7. Status final

**CLOSED 2026-09-24.** F1 dan F2 ditutup di panel dengan test masing-masing; F3 tetap OPEN sebagai
permintaan `app-serv`, dan draft mereka untuk parameter itu sudah DURING dengan definisi yang lebih
tajam (§3.3); F4 non-finding. Seluruh gerbang dan click-through terekam di §4, semuanya pada tree beku
`1bebaefd3ae1d65d79c5996499238996` (500 berkas; fingerprint diukur ulang setelah suite dan tetap sama).
