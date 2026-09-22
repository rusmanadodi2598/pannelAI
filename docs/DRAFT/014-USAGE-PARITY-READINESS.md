# 014-USAGE-PARITY-READINESS.md: Kesiapan Parity `/usage` di Panel (app-ui)

Dokumen kerja lanjutan `013-USAGE-NODE-MOTION-READINESS.md`, dengan scope **hanya `app-ui/.`**. Kontrak
tetap `docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel) dan `DESIGN.md` (arah visual). Pola mengikuti `008`
sampai `013`: temuan bernomor F, bukti yang bisa diulang, rencana DURING, keputusan owner di depan
implementasi.

| | |
|---|---|
| **Status** | DURING. F1 sampai F6 diimplementasikan; F7 (separuh `app-serv`) tetap OPEN |
| **Mechanism** | DURING & AFTER (antislop) |
| **Tanggal** | 2026-09-22 |
| **Scope** | `app-ui/.` saja. `app-serv/.` tidak disentuh oleh dokumen ini |
| **Layar** | `/usage` (tab Overview dan Records, panel Live routing) |
| **Reference fork** | `https://github.com/decolua/9router` (checkout `origin/master` di `/home/rusmanadodi/apps/9router`), `src/shared/components/UsageStats.js`, `UsageTable.js`, `UsageChart.js`, `(dashboard)/dashboard/usage/components/RequestDetailsTab.js` |
| **Kaitan** | SPEC-UI §6.5; DESIGN.md §11 (R-31, R-26, R-36); draft 012 F2; draft 013 F2; draft 007 F12 |

## 1. Ringkasan

Pass 013 menutup gerak node. Owner lalu membuka pass ini dengan satu kalimat: **"bagian `app-ui` nya juga
masih jauh dari REFERENCE. dan masih banyak yang seharusnya di tampilkan untuk page /usage"**. Audit
reference-vs-panel menemukan delapan celah, empat pertanyaan disodorkan, dan keempatnya dijawab owner
dengan arah yang sama: kerjakan `app-ui` sekarang, tanpa menunggu `app-serv`.

Yang ditemukan, semuanya pada layar yang sudah ada:

1. Blok breakdown hanya muncul bila URL menyebut `group_by`, dan tabelnya tidak punya urutan, tidak
   meresolusi nama provider, dan tidak punya default. Reference membuka tabelnya pada view model
   (`UsageStats.js:177-182`), dengan kolom yang bisa diurut dan `Last Used` di setiap set kolom
   (`:146-176`, `:87-102`).
2. Chart hanya menggambar satu seri nilai (token). Reference punya sakelar Tokens/Cost dengan dua gradien
   (`UsageChart.js:23`, `:28`, `:56-64`), dan panel belum pernah menggambar biaya sama sekali.
3. Filter tab Records tidak pernah mengirim `provider_id` maupun `gateway_key_id`, padahal kontrak
   menerimanya (`docs/CONTRACT/001-CONTRACT-API-V1.yaml:6875`, `:6890`) dan parser URL panel sudah
   membacanya.
4. Daftar request selesai menampilkan instan absolut. Reference menampilkan jarak waktu
   (`UsageTable.js:11-17`).
5. Empty state tab Records belum membawa link API Docs yang §6.5 minta, dengan komentar lama yang
   mengklaim route-nya belum ada; route itu sudah ada sejak slice API Docs.
6. Tiga penyimpangan sadar dari reference (ukuran halaman, bentuk rentang tanggal, id mentah di dua
   dimensi) belum tercatat di satu tempat.

Separuh reference yang **tidak bisa** dikerjakan di panel dicatat sebagai F7 dan tetap OPEN: `UsageGroup`
di kontrak hanya `{key, totals}` (`:2768-2778`), jadi baris grup tidak membawa `last_used` maupun anak
baris, dan detail request panel adalah `{usage, capture_enabled, log}`, bukan empat bagian collapsible
reference (`RequestDetailsTab.js`). Keduanya butuh `app-serv` lebih dulu (AGENTS.md §2.4).

## 2. Keputusan owner yang dipakai

| # | Pertanyaan | Jawaban | Konsekuensi |
|---|---|---|---|
| D1 | Bentuk pekerjaan | "LANGSUNG IMPLEMENTASI, DRAFT PLAN SUDAH ADA. Tinggal EKSEKUSI" | Tidak ada putaran register-dulu; pass ini langsung menulis kode dan mencatat temuan di dokumen ini |
| D2 | Celah `app-serv` | "LANGSUNG IMPLEMENTASI `app-ui`" | Hanya separuh panel yang dikerjakan; F7 difilekan, tidak dikejar |
| D3 | Penyimpangan ukuran halaman dan rentang tanggal | "LANGAUSUNG KERJAKAN IMPLEMENTASI app-ui" | Penyimpangan dipertahankan dan dicatat sebagai alasan di F6, bukan diubah |
| D4 | Chart biaya | "Langsung kerjakan app-ui" | Sakelar Tokens/Cost dibangun di panel; seri biaya dihitung panel dari string desimal yang API kirim |

## 3. F1 MEDIUM (FE): breakdown tidak tampil sebagai tabel, dan tidak bisa diurut

**Status: DURING selesai.**

**Fakta.** Pada `9668e3c`, tab Overview hanya merender blok grup bila URL menyebut `group_by`, dan blok
itu berupa daftar angka tanpa header yang bisa diklik. Reference, pada layar yang sama:
`TABLE_OPTIONS` (`UsageStats.js:177-182`) adalah pemilih dimensi; `sortData` (`:87-102`) mengurutkan
seluruh peta data di klien; `sortBy`/`sortOrder` dibaca dari URL dengan default `rawModel`/`asc`
(`:196-197`); klik header membalik arah kolom yang sama dan memulai kolom baru dengan `asc`
(`:299-303`); dan setiap set kolom diakhiri `Last Used` (`:146-176`). Kontrak tidak punya parameter
urut sama sekali: seluruh parameter `/api/v1/usage/summary` adalah
`from,to,group_by,granularity,status,provider_id,endpoint_id,model,gateway_key_id,q`
(`docs/CONTRACT/001-CONTRACT-API-V1.yaml:6650-6723`), dan `UsageGroup` hanya `key` plus `totals`
(`:2768-2778`).

**Risiko dan aturan.** R-25 (pasangan teks baru harus terukur); R-26 (tidak ada kontrol yang tidak bisa
berbuat); §8.4.2 (view yang bisa dibagikan adalah URL state); R-31 (alasan satu baris per keputusan).

**Rencana DURING (FE).**

1. `usage.ts`: `USAGE_BREAKDOWN_NONE = 'none'` sebagai kata layar untuk ketiadaan parameter, plus
   `USAGE_SORTS`/`USAGE_ORDERS` dan `DEFAULT_USAGE_BREAKDOWN = 'model'`.
2. `usage-search.ts`: `groupBy`, `sort`, `order` menjadi URL state; nilai yang tidak dikenal dikoreksi
   dengan notice yang menyebut nama parameternya.
3. `usage-view.ts`: `sortGroups` (string dibandingkan sebagai code point, angka sebagai selisih, ties
   stabil, tanpa memutasi array masukan) dan `formatCost` (empat desimal, sama dengan presisi API).
4. `UsageGroupTable.svelte`: header yang bisa diklik dengan `aria-sort`, penanda arah, tombol
   "Clear sort", dan kolom dimensi yang dinamai menurut dimensi yang dipilih.
5. `providerNameMap` di `usage-topology-view.ts` dan satu pembacaan registry yang hanya berjalan saat
   dimensinya provider.
6. Tabel menampilkan seluruh kolom nilai sekaligus (keputusan, lihat F6 dan DESIGN.md §11).

**Kriteria selesai.** Tabel ada di layar sejak pembacaan pertama tanpa menyentuh kontrol; klik header
menulis `sort`/`order` ke URL dan mengurutkan seluruh respons; request tidak pernah membawa parameter
urut; kunci provider tampil sebagai namanya saat registry menjawab, dan sebagai id plus satu baris
sebab saat registry gagal.

## 4. F2 MEDIUM (FE): chart tidak pernah menggambar biaya

**Status: DURING selesai.**

**Fakta.** Panel hanya punya satu seri nilai: token in plus token out. Reference punya dua mode pada satu
kartu: `viewMode` default `tokens` (`UsageChart.js:28`), dua tombol Tokens dan Cost (`:56-64`), dan
formatter biaya empat desimal (`:23`). Biaya adalah angka yang sudah dikirim API sebagai string desimal
(`cost_usd`, SPEC-API §4), jadi mode biaya bisa digambar tanpa request tambahan.

**Risiko dan aturan.** R-26 (mode harus benar-benar mengubah gambar); R-38 (angka biaya harus disebut
estimasi di tempat ia muncul); R-31.

**Rencana DURING (FE).**

1. `UsageOverviewCharts.svelte`: satu kartu nilai dengan `chartMode` lokal, seri token dan seri biaya
   dihitung dari bucket yang sama.
2. Mode biaya memakai `formatCost` (empat desimal) pada baris ringkasan, tabel disclosure, dan label
   sumbu; captionnya menyebut estimasi, sama seperti tile biaya.
3. Sakelar adalah state lokal, bukan URL state, karena ia tidak mengubah satu pun request.
4. `UsageChart.svelte` menerima `format` dan satu snippet `controls`, sehingga kartu yang menggambar satu
   seri bisa membawa kontrolnya sendiri tanpa memindahkan layout ke induk.

**Kriteria selesai.** Mode biaya menggambar nilai biaya, menuliskannya dalam mata uang pada presisi API,
menyebut estimasi, dan tidak menambah satu pun request; kembali ke mode token memulihkan seri token.

## 5. F3 MEDIUM (FE): filter provider dan gateway key tidak pernah dikirim

**Status: DURING selesai.**

**Fakta.** Parser URL panel sudah membaca `provider_id` dan `gateway_key_id` sejak pass `per_page`, dan
`usageFiltersApplied` sudah menghitungnya, tetapi `UsageRecordsTab.load` tidak pernah menyalin keduanya
ke query. Kontrak menerima keduanya di route records (`:6875`, `:6890`) dan di summary. Reference
menyaring dengan provider pada tab detail (`RequestDetailsTab.js`), panel belum.

**Risiko dan aturan.** R-26 (pilihan kosong harus benar-benar berarti "tanpa filter"); R-38 (layar tidak
boleh menyatakan sesuatu yang tidak benar tentang tabel di bawahnya); §8.4.1 (tidak ada penyaringan di
browser).

**Rencana DURING (FE).**

1. `UsageRecordsFilters.svelte` membaca dua kosakata miliknya sendiri: registry provider dan daftar
   gateway key, masing-masing `per_page=100`.
2. Setiap select menawarkan nilai kosong ("Any provider", "Any gateway key"), dan bila id di URL tidak
   ada di daftar yang terbaca, id itu tetap ditampilkan sebagai pilihannya sendiri, supaya select tidak
   berbohong tentang tabel yang sedang tersaring.
3. Satu baris notice per daftar yang gagal dibaca, menyebut sebabnya.
4. `UsageRecordsTab.load` mengirim `providerId` dan `gatewayKeyId`; `clearFilters` menghapus keduanya.

**Kriteria selesai.** Perubahan select menulis URL dan request; id yang tersaring tetap terlihat walau
daftarnya tidak memuatnya; daftar yang gagal dibaca disebut namanya; "Clear filters" mengosongkan
keduanya tanpa menyentuh period.

## 6. F4 LOW (FE): daftar request selesai memakai instan, bukan jarak waktu

**Status: DURING selesai.**

**Fakta.** `UsageLivePanel.svelte` menampilkan `formatTimestamp(record.ts)`. Reference menampilkan jarak
waktu lewat `fmtTime` (`UsageTable.js:11-17`): "Just now" di bawah satu menit, `Xm ago`, `Xh ago`, lalu
tanggal. Pembaca daftar live bertanya "baru saja atau lama", bukan "jam berapa persisnya".

**Risiko dan aturan.** R-19 (gerak harus punya tujuan tertulis); R-36 (tidak ada klaim live tanpa
sumber hidup); draft 013 F2 (gerak gambar hanya hidup saat frame tiba).

**Rencana DURING (FE).**

1. `elapsedText(value, now)` di `utils/time.ts`: batasnya sama dengan reference (`just now`, `Xm ago`,
   `Xh ago`, lalu tanggal), instan di masa depan dibaca "just now" (jam panel tertinggal, bukan request
   yang selesai di masa depan), dan nilai yang tidak bisa diparse disebut apa adanya.
2. `UsageRecentList.svelte` memisahkan daftar dari panel, dengan ticker satu detik selama daftarnya tidak
   kosong dan `clearInterval` saat unmount.
3. Ticker ini **bukan** gerak yang di-gate draft 013 F2: teks "just now" yang beku akan menjadi angka
   yang salah satu menit kemudian, sedangkan gerak gambar adalah klaim "sedang terjadi". Alasannya
   dicatat satu baris di DESIGN.md §11.
4. Instan persisnya tidak hilang: `request_id` yang sama membuka drawer-nya di tab Records.

**Kriteria selesai.** Baris menampilkan jarak waktu; batas menit/jam/hari teruji; teks berubah saat
batas terlewati sementara barisnya masih di layar; `ts` yang tidak ada tetap berbunyi "No time reported".

## 7. F5 LOW (FE): empty state tab Records belum membawa link API Docs

**Status: DURING selesai.**

**Fakta.** §6.5 tab 2 meminta empty state "No requests in this window" disertai link ke API Docs, dan
komentar di `UsageRecordsTab.svelte` menyatakan link itu ditahan "sampai route-nya ada" karena R-24
melarang link ke layar yang panel tidak punya. Route `/api-docs` sudah ada sejak slice API Docs
(draft 007 F1), jadi alasan penahanannya sudah tidak berlaku dan komentarnya menjadi klaim yang basi.

**Risiko dan aturan.** R-24 (link harus menuju layar yang ada); R-38 (komentar yang menyatakan fakta
tidak boleh dibiarkan basi); R-26.

**Rencana DURING (FE).** Tautan "Open API Docs" masuk ke snippet action empty state, di samping "Look
back 60 days" yang sudah ada, dan komentar basi di header dihapus.

**Kriteria selesai.** Empty state merender link yang resolve ke `/api-docs`, dan satu test memakunya.

## 8. F6 LOW (FE): penyimpangan sadar dari reference, dicatat satu per satu

**Status: DURING selesai (tercatat).**

Empat penyimpangan dipertahankan atas keputusan owner (D3) atau karena kontrak panel, masing-masing
dengan alasan yang juga masuk DESIGN.md §11:

| Penyimpangan | Reference | Alasan panel |
|---|---|---|
| Tabel breakdown menampilkan seluruh kolom nilai sekaligus | Sakelar Costs/Tokens di atas tabel (`UsageStats.js:488-500`) | Sakelar itu menyembunyikan angka yang tabel panel sudah tampilkan sekaligus; sakelar tetap ada di chart, yang memang menggambar satu seri pada satu waktu (F2) |
| Ukuran halaman tetap 25 | Pemilih ukuran halaman, default 20 (`RequestDetailsTab.js:93-95`, `:164`) | `per_page` adalah bagian kontrak layar panel dan sudah punya koreksi bernotice (draft 012); menambah pemilih berarti satu kontrol yang mengubah request tanpa menambah fakta baru |
| Rentang tanggal berupa lima period bernama | Dua input Start Date dan End Date (`RequestDetailsTab.js:198`, `:212`) | Pasangan tanggal native mengambil tanggal kalender lokal terhadap API yang membaca instan UTC; kelima period sudah didefinisikan di layar dan dipakai kedua tab |
| Dimensi endpoint dan gateway key menampilkan id mentah | Reference meresolusi keduanya | Nama-nama itu dimiliki layar Endpoint & Key dan Gateway Keys; membaca dua registry lagi di tab ini adalah dua request untuk dimensi yang tidak sedang dilihat |

## 9. F7 BLOCKER (BE): tiga fakta reference yang butuh `app-serv` lebih dulu

**Status: OPEN, scope `app-serv`.**

**Fakta.** Tiga hal yang reference tampilkan tidak bisa dibangun dari kontrak sekarang:

1. **`last_used` per grup.** `groupDataByKey` menghitung `lastUsed` sebagai maksimum anak-anaknya
   (`UsageStats.js:117-144`) dan setiap set kolom menampilkannya (`:146-176`). Kontrak `UsageGroup`
   hanya `{key, totals}` dengan `additionalProperties: false` (`:2768-2778`), jadi panel tidak boleh
   menampilkannya tanpa mengubah kontrak.
2. **Baris grup yang bisa dibuka.** `UsageTable.js:103-122` menyimpan keadaan buka di localStorage dan
   `:202` merender anak tiap grup; kontrak tidak mengirim anak.
3. **Detail request yang lebih kaya.** Reference punya empat bagian collapsible termasuk request yang
   diterjemahkan, respons mentah, thinking, dan statistik kompresi; panel hanya menerima
   `{usage, capture_enabled, log}`.

**Risiko dan aturan.** AGENTS.md §2.4 (kontrak dulu, kode kemudian); R-38 (jangan menampilkan angka yang
tidak dikirim wire); R-35 (klaim harus direkam).

**Rencana.** Tidak ada pekerjaan panel di pass ini. Bila `app-serv` menambah `last_used` dan anak grup
pada `UsageGroup`, tabel panel mendapat kolom dan chevron-nya; bila route detail menambah bagian, drawer
panel mendapat bagiannya. Keduanya dimulai dari kontrak, bukan dari panel.

**Kriteria selesai.** Kontrak memuat field-nya dan panel merendernya dengan test yang memaku nilainya.

## 10. Non-findings

- **Tidak ada dependensi baru.** Chart tetap CSS, tanpa pustaka graf.
- **Tidak ada request tambahan untuk dimensi yang tidak dilihat.** Pembacaan registry hanya hidup saat
  dimensi provider dipilih.
- **`/quota`, `/logs`, dan `/playground` tidak disentuh.**
- **Angka agregat tetap milik REST.** Tidak ada field agregat yang berpindah tangan di pass ini.
- **Pemilih period adalah `<select>`**, bukan lima tombol bersegmen seperti reference
  (`UsageStats.js:438`). Kelima periodnya sama; bentuk kontrolnya mengikuti pola panel. Bukan celah
  informasi, jadi tidak dihitung sebagai temuan.
- **Padding tabel DESIGN.md §11 sudah tidak konsisten di HEAD** untuk 17 baris: 16 baris memad kolom
  kedua ke 285 karakter sementara baris separator 284, dan satu baris memad kolom pertama ke 112
  sementara separator 111. Diukur dengan `python3` yang menghitung `len(line.split('|')[i])` atas
  seluruh baris tabel di `HEAD` dan di tree ini; himpunan yang menyimpang sama di kedua tree (119 baris
  sekarang, 113 di HEAD, 17 menyimpang di keduanya). Pass ini tidak memperbaikinya (itu 17 baris yang
  tidak berhubungan) dan enam baris baru dipad ke lebar separator, jadi tidak ada baris baru yang ikut
  menyimpang.

## 11. Bukti gate

Dijalankan dari `app-ui/` pada tree beku (`md5sum` atas `src/` dan `tests/` dicatat sebelum suite penuh:
`accc61752981e2da437627b3ba88e60e`).

| Gate | Hasil |
| --- | --- |
| `bun run check` | 0 error, 0 warning |
| `bun run test` | 2492 tes lulus di 142 file (pass terakhir yang tercatat: 2448 di 138 file; pass ini menambah 44 tes di empat file baru, dan jumlah file membaca 142 karena describe sort dan cost milik pass ini dipindah ke `tests/schemas/usage-sort-cost.test.ts` supaya file yang sudah lewat batas tidak bertambah panjang). Run penuh 1077 detik; tiga editan komentar (dua koreksi sitasi) yang mendarat setelah run mulai dibuktikan ulang dengan run terarah lima file (28 tes) plus Prettier, dan fingerprint tree sebelum/sesudah dicatat (`accc61752981e2da437627b3ba88e60e` lalu `1ee22f37d020dc3f2fd34520e34af888`) |
| `bun run lint` | Prettier: seluruh file sesuai |
| `bun run lint:ts` | ESLint keluar 0 |
| `bun run build` | sukses, exit 0, keluaran di `build/` |
| Pass live | click-through browser terekam: 12 tangkapan layar di `/tmp/live014/shots/` dan 20 nilai terukur di `/tmp/live014/evidence.json`; 6 baris `usage_records` plus 1 gateway key disemai lalu dihapus, baseline DB sebelas tabel identik sebelum dan sesudah, ketiga proses dihentikan lewat PID yang dicatat. Setengah live dijalankan di atas contract double (gateway belum melayani `GET /api/v1/usage/live`), dan itu tercatat di bagian Limit README |
| Ukuran file | file sumber terbesar yang disentuh: `usage-view.ts` 264 (sudah di atas ambang 220 di HEAD, pass ini menambah 4 baris), `usage-search.ts` 229 (sudah di atas ambang), komponen terbesar `UsageRecordsTab.svelte` 206; file test terbesar yang disentuh: `usage-view.test.ts` 461 (di HEAD 438; describe milik pass ini dipindah keluar, sisanya editan di dalam test yang sudah ada), `usage-records.test.ts` 418 (di HEAD 336, dilaporkan), `usage.test.ts` 364 (di HEAD 355); file test baru 205, 122, 114, dan 102 baris |
| Kebersihan teks | 0 em dash di seluruh file baru dan yang diubah |

## 12. Status per 2026-09-22

- F1 sampai F6 DURING selesai dengan test; F7 OPEN, milik scope `app-serv`.
- Register 007: F12 naik penyebutnya saja, dari dua belas dari dua puluh satu menjadi dua belas dari dua
  puluh dua bagian, karena bagian ke-22 membawa click-through yang direkam; F13 naik baris penyebutnya
  dari empat ke lima (baris pembuka pass ini memakai versi terukur `1.3.0`), jumlah klaimnya tetap
  sembilan belas.
- Commit lokal: `feat(app-ui): give the usage screen the reference's breakdown table, cost series, and
  record filters (draft 014 F1 to F6)`, tanpa push.
