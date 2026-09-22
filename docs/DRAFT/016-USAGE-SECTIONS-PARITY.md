# 016-USAGE-SECTIONS-PARITY.md: Parity Bagian `/usage` dan Sambungan Route Live (app-ui)

Dokumen kerja lanjutan `014-USAGE-PARITY-READINESS.md` dan `015-USAGE-NODE-MOTION-PARITY.md`, dengan scope
**hanya `app-ui/.`**. Kontrak tetap `docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel) dan `DESIGN.md` (arah
visual). Pola mengikuti `008` sampai `015`: temuan bernomor F, bukti yang bisa diulang, keputusan owner di
depan implementasi.

| | |
|---|---|
| **Status** | **CLOSED 2026-09-22** untuk scope pass ini: F1 sampai F4 DURING selesai dan seluruh gate lulus. **F8 OPEN** (panel, cacat gambar di viewport 390px, ditemukan click-through pass ini dan bukan buatan pass ini); **F5 sampai F7 OPEN sebagai permintaan `app-serv`** |
| **Mechanism** | DURING & AFTER (antislop) |
| **Tanggal** | 2026-09-22 |
| **Scope** | `app-ui/.` saja. `app-serv/.` tidak disentuh: F5 sampai F7 adalah permintaan bernomor, bukan pekerjaan pass ini |
| **Layar** | `/usage`, tab Overview (dua bar chart baru, daftar Finished requests) dan panel Live routing |
| **Reference fork** | `https://github.com/decolua/9router`, dibaca di `origin/master` commit `21583c03` (checkout `/home/rusmanadodi/apps/9router`) |
| **Kaitan** | SPEC-UI §6.5; SPEC-API §7.12; DESIGN.md §11 (R-02, R-13, R-29, R-35); draft 012 F2/F3/F4; draft 013 F4; draft 014 F5; register 007 F12/F13 |

## 1. Ringkasan

Owner membuka pass ini dengan pengukuran yang tegas: pada REFERENCE, `/usage` "fiturnya begitu lengkap: Node
animation, Kuota, Model yang dipakai, detail, cost, dan lainnya", sedangkan panel "belum ada tab-tab
tersebut". Pengukuran dua tree sebelum implementasi menemukan tiga hal, dan ketiganya dipakai sebagai
premis:

1. **Jumlah tab sudah sama.** Reference punya dua kontrol tab, `Overview` dan `Details`
   (`usage/page.js:47-54`), plus cabang `logs` yang hanya bisa dicapai lewat URL (`:31-34`, `:72`). Panel
   punya dua juga, `Overview` dan `Records` (`src/routes/usage/+page.svelte:13-14`). Yang berbeda bukan
   jumlah tab, melainkan isi tab kedua (F7).
2. **Kuota bukan milik `/usage` di reference.** Layar itu berdiri sendiri di `dashboard/quota/page.js`, dan
   `/usage` tidak memuat kartu kuota. Jadi item "Kuota" di daftar owner tidak menjadi temuan di sini; kuota
   panel sudah punya layarnya sendiri dan draft kuota sudah CLOSED.
3. **Yang benar-benar hilang dari panel ada tiga:** bar chart per provider, bar chart model teratas, dan
   pemisahan token In/Out di daftar request live. Ketiganya dibangun di pass ini (F1 sampai F3).

Tiga selisih lain butuh `app-serv` dan dicatat sebagai permintaan bernomor, bukan dikerjakan di sini:
periode `All` (F5), dimensi `Account` (F6), dan drawer empat body (F7).

Di tengah pass, owner memberi kabar bahwa `GET /api/v1/usage/live` sudah mendarat di `app-serv` (`ede03d2`)
dan meminta panel disambungkan supaya progresnya benar-benar CLOSED. Pass ini karena itu menaikkan bukti
live dari contract double ke **route nyata**: 21 pemeriksaan klien dan satu click-through browser yang
direkam, keduanya di atas gateway yang dibangun dari tree yang di-commit. Itu menutup separuh gateway yang
ditahan draft 012 F2/F3 dan draft 013 F4.

## 2. Keputusan owner yang dipakai

| # | Pertanyaan | Jawaban | Konsekuensi |
|---|---|---|---|
| D1 | Mode antislop | DURING & AFTER | Aturan diterapkan sambil menulis, ditutup laporan Delivery Gate empat blok |
| D2 | Seberapa banyak dibangun | Ketiga blok: bar chart provider, bar chart model teratas, pemisahan In/Out di daftar live | F1 sampai F3 DURING; selisih yang butuh backend dipisah ke F5 sampai F7 |
| D3 | Selisih yang butuh `app-serv` | Dicatat sebagai permintaan bernomor | F5 sampai F7 berstatus permintaan, dengan sitasi reference dan bentuk kontrak yang diminta |
| D4 | Route live yang sudah mendarat | Sambungkan dan buktikan di route nyata | §12.1 menjadi click-through route nyata; draft 012 F2/F3 dan 013 F4 ditutup |

## 3. F1 MEDIUM (FE): bar chart per provider belum ada

**Status: DURING selesai.**

**Fakta.** Reference menggambar satu bar chart horizontal per provider di dalam layar yang sama
(`UsageStats.js:492`), dengan delapan warna tetap (`ProviderBarChart.js:17`), nama provider dipotong sepuluh
karakter di tick formatter (`:75`), dan satu saklar Tokens/Requests (`:40`). Panel hanya punya angka provider
lewat tabel breakdown (draft 014 F1) dan lewat kartu ringkasan, jadi pertanyaan "provider mana yang
menghabiskan window ini" harus dibaca dari tabel.

**Risiko dan aturan.** R-29 (palet: satu accent plus warna status, jadi delapan warna reference tidak
dipakai); R-13 (dosis gerak dan warna, jadi bar tidak beranimasi); SPEC-UI §6.5 (angka harus terbaca tanpa
grafiknya).

**Rencana DURING (FE).**

1. `src/lib/schemas/usage-bars.ts`: seluruh aritmetika bar sebagai angka murni, tanpa DOM. `usageBars`
   membuang grup bernilai nol pada measure yang aktif, mengurutkan turun dengan urutan API sebagai
   pemecah seri, dan memberi lebar `max(1, round(value / longest * 100))` supaya bar terkecil tidak
   terbaca sebagai ketiadaan.
2. `src/lib/components/UsageBarChart.svelte`: satu gambar bar plus saklar Tokens/Requests, ringkasan
   kalimat, dan tabel `<details>` yang memuat angka persisnya. Bar sendiri `aria-hidden` karena kalimat
   dan tabelnya sudah menyatakan angka yang sama.
3. `formatCompact` memakai `Intl.NumberFormat('en-US', { notation: 'compact', maximumFractionDigits: 1 })`
   dengan locale yang dipin, bukan ambang tulis tangan reference (`ProviderBarChart.js:19-23`) yang
   mencetak `1000.0K` untuk 999.999. Itu deviasi yang disengaja dan diuji (`999_999` menjadi `1M`).

**Kriteria selesai.** Bar diurutkan, grup nol tidak digambar, lebar punya lantai satu persen, dan angka
persisnya satu disclosure dari grafiknya.

**Bukti.** `tests/schemas/usage-bars.test.ts` (24 tes) dan `tests/components/usage-bar-chart.test.ts` (6
tes): urutan, pembuangan nol, lebar `['100%', '25%']` dan lantai satu persen, seri yang mempertahankan
urutan API, batas lima, kalimat ringkasan, format compact, tabel disclosure yang mencetak `4.000` dan `40`,
label yang dipotong dengan `title`, kalimat kosong, dan penolakan yang menggantikan bar.

## 4. F2 MEDIUM (FE): chart model teratas belum ada

**Status: DURING selesai.**

**Fakta.** Reference menggambar lima model teratas (`TopModelsChart.js:40` memotong `slice(0, 5)`), dengan
nama dipotong 22 karakter (`:25`) dan `rawModel` sebagai kunci (`:34`). Panel tidak punya peringkat model
sama sekali: angka per model hanya muncul di tabel breakdown.

**Risiko dan aturan.** Sama seperti F1, plus satu batasan kontrak: `GET /api/v1/usage/summary` menjawab
**satu** `group_by` per panggilan (draft 014, diukur ulang di F4), sedangkan reference membaca semua dimensi
dari satu respons (`UsageStats.js:492-493`).

**Rencana DURING (FE).**

1. `src/lib/components/UsageOverviewBarCharts.svelte`: memasang pasangan chart dan membaca dimensi yang
   belum dipegang tab. Kalau breakdown operator adalah `provider` atau `model`, respons tab dipakai ulang
   dan tidak ada bacaan tambahan untuk dimensi itu.
2. `USAGE_MODEL_BAR_LIMIT = 5` dengan komentar yang menyitir `TopModelsChart.js:40`, dan `total` dihitung
   sebelum limit supaya kalimatnya bisa mengatakan berapa yang tidak digambar.
3. `src/lib/components/UsageOverviewEmpty.svelte`: cabang kosong tab dipindah ke komponennya sendiri saat
   `UsageOverviewTab.svelte` menyentuh ambang peringatan 220 baris.

**Kriteria selesai.** Chart provider dan chart model berdiri di antara chart seri dan tabel breakdown, tiap
chart punya saklar sendiri, dan tidak ada dimensi yang dibaca dua kali.

**Bukti.** `tests/components/usage-overview-bar-charts.test.ts` (6 tes): bacaan provider tepat satu kali
dengan window tab, nol bacaan untuk dimensi yang sedang tampil, isolasi penolakan per chart, measure kosong,
saklar measure per chart, dan kalimat `model-0 leads with 700 tokens; the top 5 of 7 models with usage are
drawn.` Satu baris di `tests/components/usage-overview.test.ts` memastikan urutan mount-nya di antara chart
seri dan tabel (`compareDocumentPosition`).

## 5. F3 MEDIUM (FE): daftar request live belum memisahkan In dan Out

**Status: DURING selesai.**

**Fakta.** Frame live sudah membawa `tokens_in` dan `tokens_out` sejak draft 012, dan reference mencetak
pasangan itu di tabel request-nya: header `In / Out` (`UsageStats.js:61`), nilainya `{fmt(r.promptTokens)}↑`
dan `{fmt(r.completionTokens)}↓` (`:78-80`). Panel mencetak model, provider, dan hasilnya, tanpa angka token,
jadi satu-satunya tempat token per request terlihat adalah tab Records.

**Risiko dan aturan.** SPEC-UI §6.5 (angka harus terbaca tanpa warna); R-29 (warna bukan satu-satunya
pembeda, jadi arah token ditulis sebagai kata).

**Rencana DURING (FE).**

1. `UsageRecentList.svelte` mencetak `7 in` dan `3 out` dengan `formatCount`, memakai kata yang sama dengan
   kartu "Tokens in" dan "Tokens out" di atasnya, bukan panah reference: baris ini dibaca tepat di sebelah
   kartu itu, dan panah tanpa legenda adalah teka-teki.
2. Nilai yang tidak dilaporkan frame tidak diklaim. Reference mencetak `0` untuk nilai yang tidak pernah
   diterima (`UsageTable.js:8` memakai `n || 0`); daftar ini mencetak `0 in` hanya kalau frame benar-benar
   mengirim nol, mencetak satu sisi kalau hanya satu sisi ada, dan mencetak `No token counts reported`
   kalau keduanya tidak ada.
3. Spasi antara angka dan arahnya ditulis sebagai ekspresi `{' '}`, karena Svelte memangkas spasi depan
   elemen yang memegang kata itu dan hasilnya `12,345in`.

**Kriteria selesai.** Baris yang sudah selesai membawa kedua angka dalam kata yang sama dengan kartunya, dan
ketiadaan angka tidak pernah dilaporkan sebagai nol.

**Bukti.** `tests/components/usage-recent-list.test.ts` (7 tes, dua baris baru): pemisahan `12,345 in` dan
`3,000 out`, nol sebagai angka versus ketiadaan, dan frame yang hanya melaporkan satu sisi. Cacat
`12,345in` ditemukan oleh run pertama test ini, bukan oleh mata.

## 6. F4 LOW (FE): satu bacaan ringkasan menjawab satu dimensi, dan biayanya diukur

**Status: DURING selesai, dicatat sebagai deviasi.**

**Fakta.** `GET /api/v1/usage/summary` menjawab untuk satu `group_by` per panggilan. Reference tidak punya
batasan itu: satu respons `stats` membawa `byProvider` dan `byModel` sekaligus, dan kedua chart membaca dari
objek yang sama (`UsageStats.js:492-493`). Jadi pasangan chart di panel berbiaya satu bacaan tambahan saat
breakdown operator bukan `provider`, dan dua saat breakdown-nya dimensi ketiga.

**Risiko dan aturan.** SPEC-API §7.12 (bentuk respons) dan aturan panel bahwa angka agregat tetap milik REST.

**Keputusan yang dipakai.** Biaya itu diterima dan diukur, bukan disembunyikan: test menghitung bacaan per
dimensi, dan kalimat loading menyebut apa yang sedang dibaca (`Reading the provider and model breakdowns.`).
Kalau owner ingin biayanya turun, permintaannya adalah summary multi-dimensi di `app-serv`; itu belum
diajukan karena panel masih nyaman dengan satu bacaan tambahan.

**Kriteria selesai.** Tidak ada dimensi yang dibaca dua kali, dan biayanya tertulis.

## 7. F5 LOW (BE): periode `All` belum ada di API

**Status: OPEN, permintaan `app-serv`.**

**Fakta.** Reference menawarkan enam periode, dan yang keenam adalah `{ value: "all", label: "All" }`
(`UsageStats.js:196-203`, baris `all` di `:202`). Panel menawarkan lima: `USAGE_PERIODS` di
`src/lib/schemas/usage.ts:29` berisi `today`, `24h`, `7d`, `30d`, `60d`, dan aritmetika window-nya
(`src/lib/schemas/usage-view.ts:35-39`) tidak punya entri untuk periode tanpa batas.

**Permintaan.** Satu window tanpa batas bawah (atau batas bawah yang diturunkan dari data tertua) di
`GET /api/v1/usage/summary`, `timeseries`, dan `records`, sehingga panel bisa menambah satu opsi periode
tanpa mengarang window di sisi klien. Sampai itu ada, `60d` adalah periode terpanjang panel, dan itu
tercatat sebagai deviasi di draft 014 F5.

## 8. F6 LOW (BE): dimensi `Account` belum ada di API

**Status: OPEN, permintaan `app-serv`.**

**Fakta.** Reference menawarkan empat tabel: `Usage by Model`, `Usage by Account`, `Usage by API Key`,
`Usage by Endpoint` (`UsageStats.js:189-194`). Panel menawarkan empat juga, tetapi dengan anggota yang
berbeda: `USAGE_GROUP_BYS` di `src/lib/schemas/usage.ts:44` berisi `provider`, `model`, `endpoint`,
`gateway_key`. `gateway_key` menutupi "API Key" dan `provider` adalah dimensi yang reference tidak punya di
tabelnya, tetapi **`account` tidak punya padanan di kontrak panel**.

**Permintaan.** Nilai `account` untuk parameter `group_by` di `GET /api/v1/usage/summary` dan `records`,
beserta label yang harus dipakai panel. Dimensi ini masuk akal hanya kalau `app-serv` menyimpan identitas
akun per request; kalau tidak, jawabannya "tidak berlaku" juga memadai dan panel tidak menambah opsi.

## 9. F7 LOW (BE): drawer empat body tidak bisa dibangun dari satu pasang body

**Status: OPEN, permintaan `app-serv`.**

**Fakta.** Tab kedua reference adalah drawer detail request dengan **empat** bagian yang bisa dibuka:
`1. Client Request (Input)`, `2. Provider Request (Translated)`, `3. Provider Response (Raw)`,
`4. Client Response (Final)` (`RequestDetailsTab.js:459`, `:466`, `:474`, `:484`, komponennya `:54`). Panel
membawa satu pasang: `schemaLogDetail` di `src/lib/schemas/log.ts:59-60` memuat `request_body` dan
`response_body` saja, jadi yang bisa ditampilkan panel adalah body klien dan body final, bukan pasangan
terjemahannya.

**Permintaan.** Kalau `app-serv` menyimpan body hasil terjemahan dan body mentah provider, keduanya perlu
tampil di detail log (dua field tambahan dengan batas byte yang sama), supaya drawer panel bisa punya empat
bagian yang sama. Kalau tidak disimpan, jawabannya "tidak berlaku" menutup temuan ini, dan tab Records panel
tetap seperti sekarang: tabel yang bisa difilter dan diurut, dengan satu pasang body di drawer-nya.

## 10. F8 LOW (FE): dua node bertabrakan di viewport 390px

**Status: OPEN, temuan panel, bukan buatan pass ini.**

**Fakta.** Click-through pass ini mengukur setiap kotak node di dua viewport. Pada 390px ada satu pasangan
yang bertabrakan: `MiMo Code Free` (kiri 170, kanan 300) dan `Live pass 016` (kiri 94, kanan 217) beririsan
**47px** mendatar dan **30px** tegak, yaitu seluruh tinggi kotaknya, sehingga label MiMo tertutup separuh
(tangkapan `05-phone.png`). Pada 1360px tidak ada pasangan yang beririsan. Di kedua viewport
`scrollWidth == clientWidth` (390 dan 1360), jadi halaman tidak meluber; yang bertabrakan hanya gambar di
dalam kartunya.

**Kenapa tidak diperbaiki di sini.** Gambar itu milik draft 012 dan 015, dan pass ini tidak menyentuh satu
baris pun di `UsageTopologyDrawing.svelte` maupun `usage-topology-view.ts`. Perbaikannya bukan perbaikan
melainkan keputusan: memberi gambar lebar minimum dengan scroll mendatar di dalam kartunya, atau menata
posisi node dengan sadar tabrakan. Keduanya mengubah perilaku gambar yang sudah CLOSED, jadi temuan ini
dicatat dengan angkanya dan diserahkan ke owner.

**Kriteria selesai.** Tidak ada pasangan node yang beririsan pada 390px, dan lebar halaman tetap tidak
meluber.

## 11. Non-findings

- **Tidak ada dependensi baru.** Kedua chart adalah div dan persentase, bukan pustaka graf.
- **Jumlah tab tidak berubah.** Reference dua kontrol tab plus satu cabang URL; panel dua tab. Yang
  berbeda isi tab kedua, dan itu F7.
- **Kuota tidak dipindahkan.** Layar kuota tetap miliknya sendiri, sesuai reference yang juga memisahkannya.
- **Angka agregat tetap milik REST.** Tidak ada field agregat yang berpindah tangan, dan tidak ada
  perhitungan ulang di klien.
- **Palet tidak bertambah.** Kedua chart memakai `--color-accent` dan token panel, bukan delapan warna
  reference.
- **`SYSTEM_MAP.md` tidak disentuh pass ini.** Peta itu memang berubah di pass ini, tetapi oleh pass
  `app-serv` `ede03d2` (baris 342 mendokumentasikan route live-nya), bukan oleh pass panel: tidak ada batas
  domain, struktur data, atau topologi layanan yang pass ini ubah.
- **Padding tabel register 007 tidak dirapikan.** Baris Usage yang pass ini perluas sudah menyimpang dari
  lebar separatornya sebelum pass ini, dan pass ini tidak meretrofit tabelnya.

## 12. Bukti gate

Dijalankan dari `app-ui/` pada tree beku (`md5sum` atas `src/`, `tests/`, `static/`, dan `scripts/`). Run penuh
`test` berjalan di atas `9b7274fab034bd1c0202e22000b41021`; gate lain (`check`, `lint`, `lint:ts`, `build`) dan run
terarah penutup berjalan di atas `5f331af088e1f63a41ce3bf08eebd11f`, yaitu tree yang sama setelah empat editan
yang disebut di baris `bun run test`.

| Gate | Hasil |
| --- | --- |
| `bun run check` | 0 error, 0 warning |
| `bun run test` | 2548 tes lulus di 147 file (pass terakhir yang tercatat: 2508 di 144 file; pass ini menambah 40, yaitu 24 di `tests/schemas/usage-bars.test.ts`, 6 di `tests/components/usage-bar-chart.test.ts`, dan 6 di `tests/components/usage-overview-bar-charts.test.ts` yang ketiganya baru, sisanya empat baris di lima file test yang pass ini ubah). Run penuh 1078 detik. Empat editan yang mendarat setelah run mulai (tiga sitasi reference di `usage-bars.ts`, `UsageOverviewBarCharts.svelte`, dan `UsageRecentList.svelte`, satu di antaranya menunjuk baris yang salah, plus perbaikan lint di `UsageRecentList.svelte` dan satu import di test bar chart) dibuktikan ulang dengan run terarah atas lima file (`tests/schemas/usage-bars.test.ts`, `tests/components/usage-bar-chart.test.ts`, `tests/components/usage-overview-bar-charts.test.ts`, `tests/components/usage-recent-list.test.ts`, `tests/components/usage-overview.test.ts`) yang mencetak 53 tes lulus di atas tree ber-fingerprint `5f331af088e1f63a41ce3bf08eebd11f`. Tiga fingerprint: `9b7274fab034bd1c0202e22000b41021` sebelum editan komentar, `775b5bf9aa504ca344e1f20e205ad3ae` sesudah sitasi, `5f331af088e1f63a41ce3bf08eebd11f` sesudah perbaikan lint |
| `bun run lint` | Prettier: seluruh file sesuai |
| `bun run lint:ts` | ESLint keluar 0. Run pertama pass ini menemukan tiga error nyata dan ketiganya diperbaiki: dua `svelte/no-useless-mustaches` pada ekspresi spasi di `UsageRecentList.svelte` (spasi itu memang load-bearing, jadi file-nya membawa satu disable beralasan, pola yang sudah dipakai empat kali di panel) dan satu import `totals` yang tidak terpakai di test bar chart |
| `bun run build` | sukses, exit 0, keluaran di `build/` |
| Verifikasi keluaran build | bundel klien hasil build memuat ketiga teks baru di `build/client/_app/immutable/nodes/19.DXQ1_dSm.js`: `Reading the provider and model breakdowns`, `Usage per provider in this window`, dan `No token counts reported`, jadi kedua chart dan pemisahan In/Out benar-benar ikut ter-bundle, bukan hanya ada di sumber |
| Ukuran file | file terbesar yang pass ini sentuh 211 baris (`tests/components/usage-breakdown.test.ts`, di bawah ambang 220). File baru semuanya di bawah ambang (`usage-bars.ts` 136, `UsageBarChart.svelte` 124, `UsageOverviewBarCharts.svelte` 146, `UsageOverviewEmpty.svelte` 37, test 200/190/159, support 106/39). Satu file tiga titik: `tests/support/usage-overview-stub.ts` 159 di `HEAD`, 255 di tengah pass (di atas ambang 250), 127 sesudah pembaca dan fixture-nya pindah ke `usage-stub-queries.ts` (39) dan `usage-fixtures.ts` (106). `UsageOverviewTab.svelte` 205 di `HEAD`, 206 sesudahnya; `UsageRecentList.svelte` 67 di `HEAD`, 99 sesudahnya (dua di antaranya disable lint beralasan). Tidak ada file over-cap lama yang disentuh |
| Kebersihan teks | 0 em dash di seluruh file baru dan yang diubah |
| Sitasi reference | tujuh sitasi `file:line` di file pass ini diverifikasi ulang ke `origin/master` dengan `grep -n`; dua di antaranya salah saat ditulis dan diperbaiki di pass ini: `UsageStats.js:490-495` menjadi `:492-493` (dua tempat) dan `UsageStats.js:75-77` menjadi `:61` plus `:78-80` |

### 12.1 Live pass: route nyata, bukan contract double

Pass-pass sebelumnya merekam click-through di atas **contract double** karena `app-serv` belum melayani
`GET /api/v1/usage/live`. Route itu sekarang ada (`app-serv` `ede03d2`, terdaftar di `router.go:192`, kontrak
di `docs/CONTRACT/001-CONTRACT-API-V1.yaml:7025`), jadi seluruh bukti di bawah berjalan di atas gateway nyata,
registry nyata, dan trafik nyata lewat data plane.

**Harness.** Gateway dibangun dari **tree yang di-commit** (`git archive HEAD app-serv` pada `ede03d2`), bukan
dari working tree, karena ada pekerjaan `app-serv` lain yang belum di-commit di sana. Gateway di `:9090`,
panel hasil `bun run build` di `:3000` dengan `PANEL_API_TARGET` menunjuk gateway, dan satu stub upstream
berbentuk OpenAI di `:8097` yang menahan jawabannya enam detik (itu yang membuat jendela in-flight
terukur), menjawab 500 untuk model berakhiran `refuse`, dan melaporkan `usage` 7 prompt dan 3 completion.
Fixture: login, satu node provider "Live pass 016" (prefix `live016`), satu endpoint, satu gateway key.

**Klien panel, 21 pemeriksaan lulus.** `/tmp/live016/drive-client.ts` menggerakkan modul panel sendiri
(`openUsageLive`, `liveMerge`, `freshActive`, `topologyNodes`, `configuredProviders`, `providerNameMap`,
`listProviders`) dengan cookie sesi disuntik dan URL relatif ditulis ulang ke panel:

1. Relay panel menjawab route live **200** dengan `content-type: text/event-stream`, `cache-control:
   no-cache`, `x-accel-buffering: no`.
2. Frame pertama tiba **10ms** setelah permintaan (16ms pada run pertama) dan berbunyi
   `data: {"active":[],"recent":[],"error_provider":""}`: state penuh, bukan delta.
3. Klien panel melaporkan `{"status":"live","reason":null,"retrying":false}`.
4. Satu panggilan chat nyata muncul sebagai `active[{provider_id, endpoint_id, model: "stub-model",
   started_at}]` selama request berjalan, lalu hilang.
5. Request yang selesai muncul sebagai baris `recent` dengan `tokens_in: 7`, `tokens_out: 3`,
   `status: "success"`, dan `request_id` ULID 26 karakter milik gateway (tanpa prefix fixture).
6. Panggilan yang ditolak menjawab **502** dan providernya muncul di `error_provider`.
7. Derivasi panel dari frame nyata: node menyala `active`, lalu `error`, dan nama provider resolve
   (`Live pass 016`).
8. Setiap frame adalah state penuh (5 frame), tidak ada yang berbentuk delta.

**Click-through browser yang direkam.** `/tmp/live016/drive-browser.ts` menggerakkan panel hasil build di
Chrome headless lewat CDP: lima tangkapan layar di `/tmp/live016/shots/` dan tujuh kelompok nilai terukur di
`/tmp/live016/evidence-browser.json`, semuanya nilai milik browser:

1. `opened`: chip `Live`; node pass ini sudah dalam state `last` karena window `recent` gateway masih memuat
   baris run sebelumnya (titik `rgb(143, 92, 10)`), dan node tetangga yang benar-benar idle bertitik
   `rgb(223, 217, 208)`.
2. `inFlight`: border, label, dan titik node `rgb(20, 116, 58)`, denyut `ping`, dan tally garis `<line>` di
   gambar menurut warnanya: sebelas `rgb(20, 116, 58)`, enam `rgb(223, 217, 208)`, lima `rgb(26, 22, 20)`
   (edge aktif beserta lapisan beam-nya), plus kalimat `1 in flight: Live pass 016 (stub-model). The last
   request to finish went to Live pass 016.`
3. `finished`: titik node kembali `rgb(143, 92, 10)`, dan baris pertama daftar berbunyi
   `just now | stub-model | 7 in | 3 out | Live pass 016 | Success`.
4. `refused`: titik node `rgb(179, 38, 30)`, tally garisnya bergeser menjadi satu garis `rgb(179, 38, 30)`,
   baris `just now | refuse | 0 in | 0 out | Live pass 016 | Error`, dan kalimat `The gateway last reported an
   error on Live pass 016.` Panggilan yang sama menjawab 502 dengan body error stub yang diteruskan.
5. `phone`: `scrollWidth == clientWidth == 390`, chip tetap `Live`, dan satu tabrakan node yang jadi F8.

**Cacat yang ditemukan pass ini dan diperbaiki (dua-duanya di driver, bukan di produk).**

1. Fixture pertama memasang endpoint ke **prefix** node (`live016`) alih-alih ke **id** registry-nya
   (`openai-compatible-…`). Data plane menjawab `503 NO_PROVIDER_AVAILABLE`, dan frame-nya menunjukkan
   `endpoint_count: 0` dengan `tokens_in/out: 0`. Dua belas dari 21 pemeriksaan gagal karena satu alasan
   ini; setelah endpoint dipasang ke id, 21/21 lulus. Perbedaan id registry versus prefix nama model itu
   sendiri tercatat sebagai jebakan di referensi environment.
2. Aktor `refused` pertama direkam dengan `sleep` tetap 2,5 detik dan menangkap node **masih routing**,
   karena stub menahan jawaban penolakan sekitar enam detik. Aktor itu sekarang menunggu kalimat yang
   ditulis layar dari frame error, bukan menunggu durasi.
3. Satu pembacaan node hanya membaca kotaknya dan menyimpulkan state error "tidak terlihat": di gambar ini
   `last` dan `error` memang sama-sama memakai border netral (`UsageTopologyDrawing.svelte:55-56`), dan
   state-nya terbaca dari titik dan edge. Probe `/tmp/live016/probe-error.ts` merekam empat instan
   (`evidence-probe-error.json`) dan menunjukkan frame error tiba 0,8 detik setelah 502, dengan
   `error_provider` terisi lalu kedaluwarsa sesuai jendela 10 detik gateway.

**Jebakan harness yang tercatat.** `pkill -f 'remote-debugging-port=9223'` ikut mencocokkan baris perintah
shell yang menjalankannya dan mematikan shell itu sendiri; PID basi dihentikan lewat PID yang dibaca dari
`ss`. Satu run yang gagal sebelum `chrome.kill()` meninggalkan Chrome yang memegang port debug, jadi run
berikutnya menyambung ke instance lama; profil Chrome karena itu dihapus per run dan setiap skrip sekarang
mengakhiri browsernya sendiri.

**Teardown.** Baseline DB dihitung sebelum dan sesudah, dan hasilnya kembali persis:
`0 0 1 1 0 2 0 0 0 0 0` (usage, logs, keys, endpoints, upkeys, nodes, caps, settings, auth_null,
media_settings, proxies). Dua koreksi diperlukan sebelum itu tercapai, dan keduanya dicatat:

1. Skrip teardown pertama menghapus baris dengan mencocokkan prefix model (`live016`), padahal baris membawa
   **id registry** provider. Sebelas baris usage dan sebelas baris log tertinggal. Aturannya sekarang
   menghapus lewat id node fixture dan lewat himpunan id node ber-prefix `live016`; percobaan fixture pertama
   (yang endpoint-nya dipasang ke prefix sehingga tidak merutekan apa pun) meninggalkan dua baris lagi di
   bawah id node pertama, dan aturan per-node yang sama menghapusnya juga.
2. Dua berkas PID sudah basi: `stub.pid` menunjuk proses yang sudah mati (fixture me-restart stub dengan PID
   baru) dan `app-serv.pid` menunjuk PID yang bukan proses gateway. Dua port karena itu masih menjawab
   setelah skrip selesai. Keduanya dihentikan lewat PID yang dibaca dari `ss -ltnp`, bukan lewat berkas PID,
   dan keempat port (3000, 8097, 9090, 9222) sekarang tertutup.

## 13. Status per 2026-09-22

- **CLOSED 2026-09-22** untuk scope pass ini: F1 sampai F4 DURING selesai dan seluruh gate lulus.
- **F8 OPEN** di scope panel: cacat gambar di viewport 390px, terukur 47px, ditemukan click-through pass ini
  dan bukan buatan pass ini (gambar tidak disentuh pass ini).
- **F5 sampai F7 OPEN sebagai permintaan `app-serv`**: periode `All`, dimensi `Account`, drawer empat body.
- **Draft 012 F2/F3 dan draft 013 F4 ditutup**: route `GET /api/v1/usage/live` sudah ada, panel tersambung
  ke route nyata, dan buktinya 21 pemeriksaan klien plus satu click-through browser yang direkam.
- Register 007: F12 naik penyebutnya saja, dari dua belas dari dua puluh tiga menjadi dua belas dari dua
  puluh empat bagian, karena bagian ke-24 membawa click-through route nyata dan tidak menyebut
  click-through mana pun sebagai outstanding; F13 naik baris penyebutnya dari enam ke tujuh (bagian ini
  memakai versi runtime terukur), jumlah klaimnya tetap sembilan belas situs.
- Commit lokal: `feat(app-ui): give the usage overview the reference's two bar charts and the live In/Out
  split, and prove the live stream against the real gateway (draft 016 F1 to F4)`, tanpa push, disusul satu
  commit dokumen kecil yang mencatat dua koreksi teardown di §12.1.
