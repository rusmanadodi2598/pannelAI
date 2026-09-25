# 004-PORT-QUOTA-TRACKER.md: Permukaan `/quota` sejajar REFERENCE, plus fondasi fetcher app-serv

Dokumen kerja pass `app-ui`+`app-serv` untuk halaman **Quota Tracker**. Permintaan owner awal satu
kalimat: **tata letak compact dan simetris**. Ralat owner di tengah pass (2026-09-25) menaikkan
lingkupnya: **fitur Quota Tracker mengikuti REFERENCE** �? kuota hanya untuk provider yang memang
mendukung (codebuddy int/cn, antigravity, gemini, openai, claude, vercel, dan yang sejenis), dan
owner memilih opsi terlengkap: panel dibentuk ulang ala reference **dan** fondasi fetcher
app-serv dimulai. Bukan kontrak; kontrak tetap `docs/SPEC-API/001-SPEC-API.md` (wire) dan
`docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel). Pola mengikuti `001` sampai `003-PORT`.

|                      |                                                                                                                                                                                                                                                                                                                                                                                                       |
| -------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**           | **CLOSED 2026-09-25.** Ralat owner diterima; lingkup panel + fondasi backend; F1-F4 diukur; D1-D8 dipakai; panel kartu ala reference dan paket `quotafetch` (dispatcher + vercel + codebuddy cn/intl) terbangun dan teruji; seluruh gerbang hijau dan click-through 7/7 di §7. Residu daftar di §7.4                                                                                                  |
| **Mechanism**        | DURING & AFTER (antislop)                                                                                                                                                                                                                                                                                                                                                                             |
| **Scope**            | `app-ui/.` (halaman `/quota` dan komponennya, skema quota, uji) dan `app-serv/.` (paket fetcher kuota baru + uji). Tabel window §6.6 dan bagian caps tetap bagian layar                                                                                                                                                                                                                               |
| **Permintaan owner** | (1) Tata letak compact dan simetris. (2) **Ralat 2026-09-25:** fitur Quota Tracker mengikuti REFERENCE; hanya provider pendukung kuota. (3) Opsi terlengkap dipilih: panel ala reference + backend fetcher dimulai. (4) Commit lokal setelah `CLOSED`                                                                                                                                                 |
| **Reference**        | `decolua/9router`, checkout `/home/rusmanadodi/apps/9router`, `39e36d3d` = tag `v0.5.86`. Halaman `/quota` = `src/app/(dashboard)/dashboard/quota/page.js` merender `ProviderLimits` (kartu per koneksi berkelompok provider, bar progres, hitung mundur reset, pesan untuk provider tanpa API). Fetcher: `open-sse/services/usage.js` (dispatcher 26 keluarga) + `services/usage/*.js` (3.779 baris) |
| **Kaitan**           | SPEC-UI §6.6, §8.6, §8.7.5, §14; SPEC-API §7.12; draft 005 (vertical kuota app-serv, CLOSED); draft 031 (free lane, sumber F1); DESIGN.md §2.1, §3.4, §7; `docs/RULLES/TDD.md`, `docs/RULLES/OWASP.md`                                                                                                                                                                                                |
| **Tanggal**          | 2026-09-25 (CLOSED)                                                                                                                                                                                                                                                                                                                                                                                   |

## 1. Ringkasan

| Yang owner sebut         | Yang diukur                                                                                                                                                                                                                                                                                                                                                                                          |
| ------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Tata letak belum compact | **Benar.** Puncak layar menumpuk tiga blok (header top 85, kontrol top 157, kalimat status top 221); editor caps menumpuk picker (499) dan form (579). Angka di §2                                                                                                                                                                                                                                   |
| Ralat: ikuti reference   | **Benar, dan lebih besar dari tata letak.** Reference menampilkan kuota sebagai kartu per koneksi (dikelompokkan provider) dengan bar progres per window; datanya dari **fetcher per keluarga provider** di backend reference (`open-sse/services/usage/`, ~20 berkas). Panel punya 0 fetcher: semua window di kawat `computed`, metode domain `Report` (window `reported`) tanpa pemanggil produksi |
| (temuan pass)            | **Tabel mati di gateway hidup.** `GET /api/v1/quotas` menjawab 16 baris; baris 12-15 (endpoint virtual `virtual:openco...` free lane draft 031) membawa `provider_id: ""`, skema panel menolak seluruh daftar, layar jatuh ke error. F1 (HIGH)                                                                                                                                                       |

## 2. Bukti yang bisa diulang (diukur hidup, 2026-09-25)

Perangkat: salinan panel dari pohon kerja di `/tmp/pass35/panel` (port **:3001**) mengarah ke
gateway owner `127.0.0.1:9090`. Panel owner `:3000` dan perangkat pass 003 `:3002`/`:9092` tidak
disentuh. Driver CDP `/tmp/pass35/measure.ts` (port 9241), bukti `/tmp/pass35/before.json`,
tangkapan layar `/tmp/pass35/shots/`.

### 2.1 Kawat

```
POST /api/v1/auth/login                      -> 204
GET  /api/v1/quotas                          -> 200, 16 baris; baris 12-15 endpoint
                                                `virtual:openco...` dengan provider_id ""
GET  /api/v1/endpoints?per_page=100          -> 3 baris (Key 1, Key 2, Key 3)
GET  /api/v1/quotas/ep_...Key 1              -> cap: null
```

Pesan galat yang layar tampilkan (`before.json` `tableAlert`): "Quota windows could not be
loaded. Unexpected response from the gateway at data.12.provider_id: Too small: expected string
to have >=1 characters".

### 2.2 Geometri sebelum (desktop 1360 px, `before.json`)

| Blok                       | Top (px) | Catatan                                      |
| -------------------------- | -------- | -------------------------------------------- |
| Header (h1 + kalimat)      | 85       | tinggi 52                                    |
| Baris kontrol (2 tombol)   | 157      | Refresh now 108.8 px, Pause refresh 117.8 px |
| Kalimat status             | 221      | satu baris sendiri, tinggi 20                |
| H2 Budget caps             | 407      |                                              |
| Picker endpoint            | 499      | satu baris sendiri                           |
| Form caps (2 field + Save) | 579      | 44 px semuanya                               |

Enam kontrol terukur 44 px satu ukuran; yang hilang adalah susunan blok, bukan ukuran kontrol.
Tabel window **tidak terender** (`tableRendered: false`) karena F1.

### 2.3 Geometri sebelum (390 px)

`docSw` 390 = viewport 390, `scrollX` 0, **0 elemen** di luar kontainer scroll: tidak ada cacat
R-03 hari ini. Tata letak baru diukur ulang di §7.

### 2.4 Mekanisme reference (dibaca dari checkout, bisa diulang)

- `services/usage.js`: dispatcher `USAGE_HANDLERS` memetakan nama keluarga ke handler
  (`claude`, `codex`, `codebuddy-cn`, `codebuddy-intl`, `antigravity`, `gemini-cli`,
  `vercel-ai-gateway`, dan ~19 lainnya); keluarga tanpa handler menjawab
  `{ message: "Usage API not implemented for <provider>" }` �? pesan itu yang layar render di
  kartu (prop `message`, `ProviderLimitCard.js:135-147`).
- Kartu (`ProviderLimitCard.js`): ikon + nama + badge plan + tombol refresh per kartu; quotas
  sebagai bar progres (`QuotaProgressBar.js`: persen sisa, warna hijau >70 / kuning 30-70 /
  merah <30 dari SISA, `used / total requests`, `Reset in <countdown>`, `Reset at <instant>`,
  `Expires` untuk paket sekali pakai `recurring:false`); state loading/error/message/empty.
- Halaman (`index.js`): koneksi dikelompokkan per provider (`groupByProviderStable`), grid
  `md:grid-cols-2 gap-3`, refresh semua berkala (60 detik; claude 10 menit karena 429).

## 3. Keputusan pass ini

| #   | Pertanyaan                                              | Arah yang dipakai                                                                                                                                                                                                                                                                                                                                                                                                                               | Dipakai oleh |
| --- | ------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| D1  | Bentuk penyajian window?                                | **Kartu ala reference** (ralat owner): satu kartu per endpoint, dikelompokkan per provider, tiap window satu bar progres (label jenis window, persen terpakai, used/total, hitung mundur reset). Warna bar dari token tema panel, bukan palet tailwind reference                                                                                                                                                                                | F2, F4       |
| D2  | Provider tanpa kuota (lane virtual, `provider_id: ""`)? | **Kartu pesan ala reference**, bukan baris tabel: window lane virtual dikelompokkan di kartu info "counted locally, the provider publishes no quota". Skema baca tetap dilonggarkan ke string bebas (F1) supaya satu baris buruk tidak mematikan daftar                                                                                                                                                                                         | F1, F2       |
| D3  | Apakah editor Budget caps ikut berubah?                 | **Bertahan apa adanya** (sudah CLOSED sebagai slice U2), hanya dirapatkan ke satu baris sesuai directive compact awal. Caps berkunci endpoint, bukan provider; reference tidak punya padanannya                                                                                                                                                                                                                                                 | F3           |
| D4  | Baris kontrol puncak?                                   | Satu baris pola rumah 003: kalimat status `flex-1` + dua tombol, satu baseline. Tombol tetap teks (ikon bukan lingkup pass ini; `icons.ts` membawa suntingan 003 owner)                                                                                                                                                                                                                                                                         | F2           |
| D5  | Fondasi fetcher app-serv: mulai dari mana?              | Paket `internal/service/quotafetch`: antarmuka `Fetcher` + dispatcher peta keluarga (mirip `USAGE_HANDLERS`) + jenis hasil bersama. Dua keluarga terdiri diport utuh dengan uji `httptest`: `vercel-ai-gateway` dan `codebuddy-cn`/`codebuddy-intl` (satu pola, 146 baris reference). Keluarga OAuth (claude, codex, gemini-cli, antigravity) difile: masing-masing membawa cooldown 429, cache, dan fallback yang harus diport, bukan dilempar | F5           |
| D6  | Bagaimana endpoint mengetahui keluarganya?              | **Belum ditautkan.** Model provider app-serv tidak punya konsep keluarga (hanya `nodeType` + `apiType`); pilihan pemodelan (kolom baru di `provider_nodes`, turunan prefix id, atau peta konfigurasi) adalah keputusan owner. Karena itu worker re-check §7.12 dan route refresh per kartu juga difile, bukan dibangun setengah                                                                                                                 | F6           |
| D7  | Apakah skema/§6.6 diedit di berkas spec?                | **Tidak di pass ini.** `docs/SPEC-UI/001-SPEC-UI.md` membawa suntingan 003 owner yang belum ter-commit; catatan §6.6/§14 yang terutang dicatat sebagai residu §7.4                                                                                                                                                                                                                                                                              | semua        |
| D8  | Copy kartu pesan dan empty state?                       | Bahasa Inggris panel, kalimat jujur: kartu info menyebut lane dihitung lokal dan provider menerbitkan kuota tidak (bukan klaim "tidak didukung")                                                                                                                                                                                                                                                                                                | F2           |

## 4. Temuan

### 4.1 F1 (HIGH): satu window tanpa provider mematikan seluruh layar

`schemaQuotaWindow.provider_id` adalah `z.string().min(1)`; window lane virtual membawa
`provider_id: ""` (live: 4 dari 16 baris), jadi satu baris buruk menolak seluruh daftar. Diperbaiki
sisi panel (D2); kartu pesan adalah tempat baris itu tampil.

### 4.2 F2 (MEDIUM): layar belum bentuk reference

Tiga blok bertumpuk di puncak (85/157/221) dan tabel window per endpoint×window-kind bukan
bentuk reference (kartu per endpoint berkelompok provider dengan bar progres). Ralat owner
menetapkan arah kartu; compact+simetris berlaku pada bentuk baru.

### 4.3 F3 (MEDIUM): editor caps menumpuk picker di atas form

Picker (top 499) dan form (579) adalah dua baris untuk empat kontrol 44 px. Dirapatkan satu baris (D3).

### 4.4 F4 (HIGH, sebagian dikerjakan): panel tidak punya penyajian per provider

Bentuk lama merender baris mentah per endpoint×window tanpa kelompok provider dan tanpa proporsi
(bar progres). Ini inti ralat; dikerjakan di panel (D1, D2).

### 4.5 F5 (HIGH, fondasi dikerjakan): tidak ada fetcher kuota provider di app-serv

Semua window `computed`; `QuotaWindow.Report` tanpa pemanggil. Fondasi dibangun (D5); keluarga
OAuth dan penautan endpoint difile (D6).

### 4.6 F6 (MEDIUM, difile untuk owner): model provider tidak mengenal keluarga

`provider_nodes` hanya punya `node_type` + `api_type`. Agar worker §7.12 bisa men-fetch kuota
per endpoint, app-serv perlu peta endpoint→keluarga. Tiga opsi untuk keputusan owner:
(a) kolom `quota_family` baru di `provider_nodes` + migrasi; (b) turunan dari prefix id;
(c) peta konfigurasi di luar DB. Pass fetcher berikutnya butuh keputusan ini sebelum kode.

## 5. Rencana DURING

1. **TDD skema (F1).** Kasus `provider_id: ""` lolos parse (merah dulu), `provider_id` menjadi
   string bebas dengan alasan di komentar.
2. **Panel kartu (F2, D1, D2, D8).** Komponen `QuotaCards`: kartu per endpoint (label endpoint
   dari bacaan label, fallback id), dikelompokkan per provider; bar progres per window dengan
   persen terpakai, used/limit, hitung mundur reset, badge source; kartu pesan untuk kelompok
   tanpa provider; empty/loading/error tetap. `QuotaTable` pensiun dari layar.
3. **Tata letak (F3, D4).** Baris puncak rumah 003; editor caps satu baris.
4. **TDD tata letak.** `tests/components/quota-layout.test.ts`: baris puncak satu parent; baris
   caps satu parent; kartu: kelompok per provider, kartu pesan tanpa provider.
5. **Backend fondasi (F5, D5).** Paket `quotafetch`: `Fetcher`, dispatcher, jenis hasil bersama
   (quotas + plan + message), dua keluarga diport (vercel-ai-gateway; codebuddy-cn dan
   codebuddy-intl), uji `httptest` table-driven per keluarga (satu panggilan keluar per bacaan,
   context timeout, tanpa kredensial yang dicatat).
6. **Gerbang.** Panel: `bun run test/check/lint/lint:ts/build` + click-through hidup AFTER.
   app-serv: `go build/vet/test -race`, gofmt, header, lint, kontrak tidak berubah (tanpa
   route baru), gerbang scrypts yang relevan.
7. **File untuk pass berikutnya.** Keluarga OAuth (claude/codex/gemini-cli/antigravity) dengan
   berkas reference masing-masing; keputusan D6; worker re-check §7.12; route refresh per kartu;
   live pass dengan kredensial nyata.

## 6. Implementasi

Semua butir §5 dikerjakan. Set perubahan: **app-ui** 8 berkas (halaman, `QuotaCards` baru,
`QuotaCaps`, skema, 3 berkas uji, `QuotaTable` dihapus), **app-serv** 7 berkas (paket
`quotafetch`: tipe+dispatcher+dua keluarga+tiga berkas uji). Tidak ada route baru; kontrak wire
tidak berubah.

### 6.1 Panel: kartu ala reference (F2, D1, D2, D8)

`QuotaCards.svelte` (139 baris, baru) menggantikan `QuotaTable`: kartu per endpoint, kelompok
provider urut kemunculan pertama (aturan `groupByProviderStable` reference), satu bar progres per
window. Konvensi panel dipertahankan: persen yang dicetak adalah persen TERPAKAI (§6.6,
`quotaPercentLabel`), angka tanpa unit (kawat tidak membawanya); yang diambil dari reference adalah
warna bar pada SISA (di atas 70% `--color-ok`, 30-70 `--color-warn`, di bawah `--color-danger`) dan
kalimat hitung mundur `Resets <countdown> (at <instant>)`. Kelompok tanpa provider merender heading
"No provider" + kalimat "Counted locally by this gateway; the provider behind this lane publishes
no quota." (bentuk kartu pesan reference). Kalimat penjelas source tetap di bawah kartu (§8.7.5).

### 6.2 Panel: tata letak (F2, F3, D3, D4)

Baris puncak rumah 003 (`flex flex-wrap items-stretch justify-between gap-2`, kalimat status
`flex-1`, dua tombol satu baseline); editor caps satu baris `flex flex-wrap items-end gap-3`
(picker, dua field, Save) dengan kalimat peringatan, kalimat tersimpan, dan FormIssues tetap di
bawahnya.

### 6.3 Skema (F1, D2)

`schemaQuotaWindow.provider_id` menjadi string bebas dengan alasan di komentar; sel kelompok
"No provider" adalah tempat baris itu tampil. Satu baris buruk tidak lagi menolak seluruh daftar.

### 6.4 app-serv: fondasi fetcher (F5, D5, D6)

Paket `internal/service/quotafetch` (5 sumber + 3 uji, semua di bawah ambang): `Quota`/`Result`/
`Credentials` (mengikuti bentuk `Result` reference: plan, quotas, message lembut), dispatcher
peta keluarga dengan kalimat "Usage API not implemented for <family>" untuk keluarga tanpa
handler, dan dua keluarga diport utuh: `vercel-ai-gateway` (GET /v1/credits, dua baris
used/remaining, alokasi bulanan $5 sebagai penyebut) serta `codebuddy-cn`+`codebuddy-intl`
(POST endpoint billing Tencent/CodeBuddy, header transport registry, pisah paket refill vs
bonus dengan aturan gap 2 hari, label kadens Monthly/Weekly/Daily, `Recurring:false` untuk paket
sekali-pakai). Timeout keluar 20 detik di satu client; pesan kesalahanIntl memakai kata wilayahnya
sendiri (deviasi tertulis dari reference yang menulis "CN" untuk kedua wilayah). Penautan
endpoint→keluarga, worker re-check §7.12, dan route refresh per kartu DIFILE (D6/F6): model
provider belum punya konsep keluarga, dan tiga opsi pemodelan menunggu keputusan owner.

### 6.5 Uji (TDD)

- Merah alami terbukti sebelum implementasi: kasus `provider_id: ""` di skema, uji baris
  puncak, dan uji baris caps (4 merah, 41 hijau pada pohon pra-implementasi).
- Mutasi: pemetaan keluarga dimatikan (semua window masuk satu kelompok) membuat tepat uji
  kelompok merah (3 merah), dipulihkan, hijau lagi.
- Panel: 4 berkas quota (57 uji). app-serv: 3 berkas uji `quotafetch` (httptest, satu panggilan
  keluar per bacaan, header transport dan body "{}" diverifikasi, kutipan pesan dikunci).

## 7. Status pass ini

### 7.1 Gerbang

| Gerbang                                 | Hasil                                                                                                                                                                                                                                                                                               |
| --------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| app-ui prettier (`bun run lint`)        | PASS, seluruh pohon rapi                                                                                                                                                                                                                                                                            |
| app-ui eslint (`bun run lint:ts`)       | PASS (satu temuan diperbaiki: `svelte/prefer-svelte-reactivity` pada `Map` lokal `QuotaCards.svelte`, dikelompokkan ulang lewat array)                                                                                                                                                              |
| app-ui svelte-check (`bun run check`)   | PASS 0 error / 0 warning (diulang pasca perbaikan eslint)                                                                                                                                                                                                                                           |
| app-ui build (`bun run build`)          | PASS, exit 0 (diulang pasca perbaikan eslint)                                                                                                                                                                                                                                                       |
| app-ui suite penuh (`bun run test`)     | PASS 2592 test / 160 berkas, 1406.60 dtk, exit 0, sendirian pada pohon final                                                                                                                                                                                                                        |
| app-serv gofmt / vet / build            | PASS                                                                                                                                                                                                                                                                                                |
| app-serv `go test -race -count=1 ./...` | PASS 16 paket ok. Jalan ulang pasca perbaikan S1039 bersamaan dengan suite panel mengisi kedua inti CPU membuat paket `internal/router` menghantam batas 10 menit per paket (`TestSettingsRoutes_ThroughMux` baru 11 detik saat alarm); diulang sendirian pada pohon final: PASS 16 paket ok, 4m14s |
| app-serv go-headers.sh                  | PASS (setelah ketiga berkas uji `quotafetch` diberi header lengkap)                                                                                                                                                                                                                                 |
| app-serv go-lint.sh                     | PASS (satu temuan staticcheck S1039, `fmt.Sprintf` tanpa argumen di `codebuddy_test.go`, diperbaiki)                                                                                                                                                                                                |
| app-serv secrets gate                   | PASS (gate mendaftar berkas dari indeks, jadi penghapusan `QuotaTable.svelte` yang belum di-stage mematahkan materialisasi tar; penghapusan di-stage)                                                                                                                                               |
| Kontrak                                 | Tanpa perubahan; `openapi.json` tidak tersentuh; `contract-openapi` dan `contract-drift` PASS                                                                                                                                                                                                       |

### 7.2 Click-through hidup (R-35)

Panel salinan pohon AFTER di `:3001` (dibangun ulang dari pohon final, sidik jari di atas) ke
gateway owner di `:9090`; driver CDP `/tmp/pass35/driver.ts`, hasil `after.json`, tangkapan
`shots/after-desktop.png` dan `shots/after-390.png` (pemeriksaan visual dua PNG itu diserahkan ke
owner; pengukuran di bawah numerik dari DOM, bukan simpulan dari kelas CSS).

- **A, baris toolbar satu basaat**: `oneRow: true`, Refresh now dan kalimat interval di `top` yang
  sama (157 = 157), tombol memuat "Pause refresh".
- **B, kartu**: 2 kelompok `h3` (provider `openai-compatible-0386BKG9...` lalu "No provider"),
  4 kartu endpoint (`Key 1`, `Key 2`, `Key 3`, `virtual:opencode`), 8 label persen; `bars: 0` karena
  gateway hidup ini tidak menerbitkan satu pun `limit`, jadi tiap baris mencetak penghitung
  ("No limit" atau jumlah terpakai: 1, 1, 831, 831) tanpa palang, sesuai aturan null-ceiling.
- **C, lorong tanpa kredensial**: grup "No provider" ada (`noProvider: true`) beserta kalimat
  penjelas sumber (`sourceSentence: true`).
- **D, editor caps satu baris**: `sharedRow`, `wrap`, `baseline`, `oneHeight` seluruhnya `true`
  (pemilih endpoint, harga, token, dan Save berbagi satu baris).
- **E, jeda dan muat ulang**: sebelum klik `pauseBefore: false`, setelah klik "Pause refresh" tampil
  (`pausedShown: true`), "Resume refresh" tampil setelahnya (`resumedShown: true`), dan "Refresh
  now" memicu pembacaan ulang (`refreshOK: true`).
- **F, 390 px**: `scrollWidth` dokumen 390 = viewport 390, paksaan `scrollTo(400,0)` meninggalkan
  `scrollX` 0, dan 0 elemen di luar wadah gulir (`overflowCount: 0`).
- **G, konsol**: `windowErrors: "none"`, nol error jendela pada halaman quota.

Perbaikan perangkat saat sesi: kolektor `window.onerror` versi pertama terpasang sebelum navigasi
penuh ke `/quota`, sehingga realm-nya terhapus dan pengukuran G mengembalikan `undefined` yang
diam-diam dibuang `JSON.stringify`; kolektor dipindah ke setelah navigasi dan G terukur.

### 7.3 Suite penuh

Jalan pertama (bersamaan dengan pekerjaan berat lain di mesin dua inti): 4 test menghantam batas
`Test timed out in 5000ms` (`combo-editor-models`, `custom-provider-card-actions`,
`sidebar-landmark`, `providers-custom-section`) dan keluar kode 1. Tidak ada asersi yang merah;
keempat berkas itu tidak tersentuh pass ini, lulus 21/21 saat dijalankan sendiri pada pohon yang
sama, dan polanya persis kelas kontensi CPU yang ditemui gerbang Go (`internal/router`, batas 10
menit per paket). Suite diulang sendirian pada pohon final (sidik jari `md5sum` berkas
`src`+`tests` bertipe `.ts/.tsx/.svelte`, `94f3daa6045e977884347f32597713f0`, dibuktikan dengan
`find -newermt` nol berkas berubah sejak jalan pertama dimulai): **2592 test / 160 berkas,
seluruhnya lulus**, exit 0, 1406.60 dtk. Aritmetika terhadap baseline 2585/158 (record 003 yang
sudah membawa 21 kasus 3 berkas icon): quota.test.ts 12 menjadi 14 kasus (+2), quota-layout.test.ts
baru (+3), dua baris tabel skema (+2), total +7 = 2592; berkas 158 + 1 hasil split icon + 1
quota-layout = 160.

### 7.4 Residu

- **Keluarga OAuth** (claude, codex, gemini-cli, antigravity, dan ~19 keluarga reference lainnya)
  belum diport; berkas reference masing-masing tercantum di `open-sse/services/usage/` (claude.js
  199 baris dengan cooldown 429 dan cache per token adalah polanya yang paling rumit).
- **Penautan endpoint→keluarga** (F6/D6): tiga opsi menunggu keputusan owner (kolom
  `quota_family` di `provider_nodes`; turunan prefix id; peta konfigurasi). Worker re-check
  §7.12 dan route refresh per kartu menunggu keputusan ini.
- **Live pass dengan kredensial nyata** untuk keluarga yang diport: butuh kredensial
  vercel/codebuddy milik owner; uji terhadap stub httptest sudah tertutup.
- **SPEC-UI §6.6** menyebut "Table:"; amendemen ke bentuk kartu tertunda karena berkas itu
  membawa suntingan 003 owner yang belum ter-commit (D7). Catatan §14 untuk `provider_id`
  longgar dan konvensi persen terpakai ikut amendemen itu.
- Perangkat click-through (panel salinan :3001 dari `/tmp/pass35`, gateway owner :9090) tetap
  hidup setelah pass menunggu keputusan owner: dipakai klik-ulang, atau dimatikan.
