# 018-USAGE-DRAWING-SIZING-PARITY.md: Gambar `/usage` mengambil ukurannya dari kotaknya sendiri (draft 016 F8)

Dokumen kerja pass `app-ui` untuk endpoint `/usage` (Overview & Records). Pass ini mengerjakan satu
keputusan yang draft 016 serahkan ke owner: dua kotak node di dalam gambar live bertabrakan di viewport
390px. Bukan kontrak; kontrak tetap `docs/SPEC-API/001-SPEC-API.md` (wire) dan
`docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel).

|                  |                                                                                                                                                            |
| ---------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**       | CLOSED 2026-09-23. F1 sampai F4 selesai; commit lokal scope `app-ui` saja, belum di-push                                                                   |
| **Mechanism**    | DURING & AFTER (antislop)                                                                                                                                  |
| **Scope**        | `app-ui/.` saja. `app-serv/.` tidak disentuh pass ini                                                                                                      |
| **Permintaan owner** | Endpoint UI `/usage` (Overview & Records), node animation 1:1 terhadap REFERENCE, dikerjakan sampai CLOSED; commit dilakukan setelah CLOSED, lokal, hanya scope `app-ui` |
| **Reference**    | `decolua/9router`, checkout `/home/rusmanadodi/apps/9router`, dibaca di `origin/master` (`21583c03`, v0.5.85)                                              |
| **Kaitan**       | draft 012 F3, draft 013 F1/F2, draft 015 F1/F5, draft 016 F8 (ditutup pass ini); SPEC-UI §7.12, §8.4; DESIGN.md §11; AGENTS.md §1.1                          |
| **Tanggal**      | 2026-09-23                                                                                                                                                 |

## 1. Ringkasan

Draft 016 F8 mencatat satu cacat yang click-through-nya temukan dan tidak diperbaiki di sana: pada 390px
dua kotak node beririsan **47px mendatar dan 30px tegak**, sehingga label `MiMo Code Free` tertutup
separuh. Draft itu menyatakan perbaikannya bukan perbaikan melainkan keputusan, dan menyebut dua arah:
memberi gambar lebar minimum dengan scroll mendatar di dalam kartunya, atau menata ulang posisi node.

Pass ini mengambil arah ketiga yang justru perilaku reference-nya sendiri: **gambar mengambil ukurannya
dari lebar kotaknya**, seperti `fitView` reference menskalakan seluruh kanvasnya. Reference memang
menskalakan, bukan menggulir: `ProviderTopology.js:416-434` memasang `ResizeObserver` yang memanggil
`fitView` tiap kali kontainer berubah ukuran, dan `:452-455` memberi `fitView` dengan `minZoom={0.1}`
serta `maxZoom={2}`. Yang panel pakai bukan JavaScript melainkan satuan kontainer CSS (`cqw`), karena
gambar panel tidak punya pan dan zoom (`DESIGN.md` §11), sehingga tidak ada yang perlu diukur saat runtime.

Hasilnya, diukur di browser pada tree yang di-commit:

| Viewport | Kotak gambar | Lebar node     | Tinggi node | Font node   | Gateway        | Pasangan beririsan | Halaman meluber |
| -------- | ------------ | -------------- | ----------- | ----------- | -------------- | ------------------ | --------------- |
| 1360px   | 886 × 320    | 95 sampai 130  | 30          | 14px        | 108 × 38       | tidak ada          | tidak           |
| 390px    | 228 × 320    | 35 sampai 47   | 12          | 4,91077px   | 39 × 15 (49 × 15 saat routing) | tidak ada | tidak |

Angka 1360px **tidak berubah sama sekali** dari sebelum pass ini (node 95 sampai 130, gateway 108 × 38,
font 14px): yang berubah hanya kotak yang lebih sempit daripada lebar di mana node mencapai batasnya.

Empat temuan dicatat, semuanya selesai:

- **F1** cacat draft 016 F8 diukur ulang sebelum dan sesudah, dengan pasangan dan angkanya.
- **F2** satu unit gambar yang berasal dari lebar kotak, dan satu jebakan yang hanya terlihat di browser
  (font yang resolve `cqw` terhadap halaman, bukan terhadap gambar) diperbaiki dan dipaku test.
- **F3** file gambar melewati batas 220 baris karena komentar dan konstanta beam, jadi dipisah per
  lapisan: 231 baris menjadi 118 (kotak dan kartu) plus 150 (edge dan beam).
- **F4** tab Records diukur terhadap `RequestDetailsTab.js` reference: panel sudah memuat seluruh kolom
  reference, dan satu selisih yang tersisa dicatat dengan pilihannya, bukan dikerjakan diam-diam.

## 2. Keputusan owner yang dipakai

Owner tidak ditanya di pass ini: arahannya sudah lengkap (scope `/usage` Overview & Records, node
animation 1:1 dengan REFERENCE, selesai sampai CLOSED, commit lokal setelah CLOSED). Satu keputusan
diambil dari dua opsi yang draft 016 sebutkan, dan dicatat di sini supaya bisa dibalik kalau owner
memilih arah yang lain.

| #   | Pertanyaan                                                                                          | Arah yang dipakai pass ini                                                                                                    | Dipakai oleh |
| --- | --------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- | ------------ |
| D1  | Perbaikan F8: lebar minimum plus scroll mendatar di dalam kartu, atau gambar yang menskalakan diri? | **Menskalakan diri**, karena itu perilaku reference (`fitView`), dan karena scroll mendatar di dalam kartu menambah satu kontrol yang gambar ini tidak punya (`DESIGN.md` §11). Konsekuensinya label ikut mengecil di 390px (4,91px), sama seperti reference mengecilkan labelnya sendiri. | F1, F2       |
| D2  | Kolom `Cached` dan `Cache Creation` per baris di tab Records (reference memisahkannya, panel menaruhnya di drawer)? | **Tidak dikerjakan pass ini**, dicatat sebagai pilihan F4: wire panel sudah membawa `tokens_cache_read` dan `tokens_cache_write`, jadi menambah dua kolom adalah keputusan tampilan, bukan pekerjaan kontrak. | F4           |

## 3. F1 MEDIUM (FE): dua node bertabrakan di 390px, diukur sebelum dan sesudah

**Status: CLOSED 2026-09-23.**

**Fakta (sebelum).** Diukur dengan probe baca-saja terhadap panel yang sedang berjalan milik owner
(`http://127.0.0.1:3000`, port yang sama sekali tidak diganggu pass ini) sebelum satu baris pun diubah.
Pada 390px ada tepat satu pasangan yang beririsan: `MiMo Code Free` dan `Live pass 017`, `dx = 47`,
`dy = 30`. Pada 1360px tidak ada pasangan yang beririsan. Angka ini mereproduksi F8 draft 016 (`dx = 47`,
`dy = 30` dengan `Live pass 016`), karena posisi node adalah persentase dan hanya lebar kotaknya yang
berbeda.

Perhatikan bahwa node yang bertabrakan hanya bisa terjadi di antara dua node yang **berbagi baris**:
pasangan yang jarak tegaknya satu tinggi node atau lebih tidak bisa bersinggungan pada lebar berapa pun.
Itu yang membuat perbaikannya bisa dihitung, bukan dicari.

**Fakta (sesudah).** Click-through yang direkam (§8.1) mengukur setiap kotak di gambar pada dua viewport,
dengan pasangan yang beririsan dihitung dari rect masing-masing kotak:

- 390px: delapan kotak (tujuh node plus gateway), `overlaps = []`, `outsideTheBox = []`,
  `scrollWidth == clientWidth == 390`.
- 390px saat gateway routing (kasus terlebar: badge jumlah ikut tampil dan kartunya berdenyut ke 1,06):
  gateway 49 × 15, `overlaps = []` lagi.
- 1360px: `overlaps = []`, `outsideTheBox = []`, `scrollWidth == clientWidth == 1360`, dan angka tiap
  kotak sama dengan sebelum pass ini.

**Kriteria selesai.** Tidak ada pasangan kotak yang beririsan pada 390px, dan lebar halaman tetap tidak
meluber. Terpenuhi di ketiga pengukuran di atas.

## 4. F2 MEDIUM (FE): satu unit gambar, dari lebar kotaknya

**Status: CLOSED 2026-09-23.**

**Fakta.** Gambar ini sudah menyimpan semua posisinya sebagai persentase, jadi ia tidak pernah butuh
pengukuran untuk menempatkan node. Yang belum ia punya adalah ukuran: padding, font, ikon, lebar label,
dan lebar tiga stroke beam semuanya angka piksel tetap, dan itulah yang membuat dua node beririsan di
kotak 228px. Perbaikannya memberi gambar satu unit:

```
--u: min(1, calc(var(--share) * tan(atan2(100cqw, 130px))))
```

`100cqw` adalah lebar kotak gambar itu sendiri (`container-type: inline-size` di root-nya), `130px`
adalah lebar di mana sebuah node mencapai batasnya (`NODE_MAX_WIDTH` di `usage-topology-view.ts`, yang
memang jumlah dari padding, dot, gap, label, dan border), dan `tan(atan2(a, b))` mengubah perbandingan dua
panjang menjadi angka tanpa satuan, sehingga `--u` bisa dipakai mengalikan panjang. `--share` datang dari
`topologyNodes` dan adalah bagian lebar kotak yang boleh diambil satu node, dihitung dari jarak mendatar
terdekat di antara pasangan yang berbagi baris dikurangi kelonggaran 15 persen, dibatasi 20 persen. Karena
`--share` sendiri turun saat kotaknya sempit, node mengecil lebih cepat daripada kotaknya, dan pada 390px
jarak mendatar antara dua node sebaris yang tadinya beririsan menjadi 33px bersih.

Dua hal yang diukur, bukan diasumsikan:

1. **`tan(atan2())` bekerja di browser ini.** Diuji lebih dulu di Chrome sebelum dipakai
   (`/tmp/live017/probe-css.ts`): `--u` bernilai 1 pada kotak 650px ke atas dan turun proporsional di
   bawahnya.
2. **Sebuah elemen bukan kontainer kueri untuk dirinya sendiri.** Potongan pertama menaruh
   `font-size: calc(14px * var(--u))` di root gambar, dan click-through pertama mengukur hasilnya:
   **8,4px** di dalam node 47px pada 390px, karena font di root itu resolve `cqw` terhadap halaman,
   sementara padding di node resolve terhadap gambar. Ukuran font sekarang ada di kotak node dan di kartu
   gateway, dan keduanya resolve terhadap gambar. Test memaku jebakan itu: root tidak boleh memuat
   `font-size`, dan kotak node harus memuat `[font-size:calc(14px*var(--u))]`.

Satu batas yang dicatat apa adanya: border 1px tidak ikut menskalakan, karena di bawah satu piksel ia
tidak tergambar sama sekali. Model di `nodeShare` menghitungnya seolah ikut menskalakan, jadi lebar nyata
satu node bisa lebih besar sampai `2 - 2u` piksel daripada modelnya (1,3px pada 390px), dan kelonggaran
15 persen yang jauh lebih besar dari itu yang menutupinya. Yang membuktikan tidak ada tabrakan tetap
pengukuran browser di §8.1, bukan modelnya.

**Kriteria selesai.** Setiap metrik gambar berasal dari satu unit, unit itu berasal dari lebar kotaknya,
dan test memaku nilai yang dihitung maupun jebakan `cqw`-nya. Terpenuhi:
`tests/schemas/usage-topology-view.test.ts` menguji 7 lebar kotak dikali 39 jumlah node (2 sampai 40)
untuk membuktikan tidak ada pasangan node yang bersinggungan pada modelnya, dan tiga test komponen
memaku kelas `calc(Npx*var(--u))` di stroke, node, gateway, dan partikelnya.

## 5. F3 LOW (FE): gambar melewati 220 baris, dipisah per lapisan

**Status: CLOSED 2026-09-23.**

**Fakta.** `UsageTopologyDrawing.svelte` 211 baris di HEAD, dan pass ini menambah komentar yang mengikat
dua jebakan yang diukur di browser (satuan kontainer dan `fitView`) sehingga file itu menjadi 231 baris,
melewati ambang peringatan 220. Isinya memang dua teknik menggambar yang berbeda: lapisan SVG yang
direntangkan (`viewBox 0 0 100 100` plus `preserveAspectRatio="none"`, dengan `non-scaling-stroke`) dan
kartu HTML yang diposisikan absolut dengan ukuran dari satuan kontainer.

**Perbaikan.** Lapisan SVG dipisah ke `src/lib/components/UsageTopologyEdges.svelte` (150 baris), yang
memuat seluruh konstanta beam dan partikelnya, filter turbulensinya, dan satu `<line>` per node; kotak
gambar tinggal 118 baris dan memuat unitnya, kartu gateway, dan kotak tiap node. Batasnya bukan
aritmetika: `svg` ada di dalam file anak, sehingga konteks namespace-nya jelas, dan test yang sudah ada
membaca garis-garisnya lewat komponen frame, jadi pemisahan ini tidak mengubah satu perilaku pun.

**Yang diuji dan hasilnya negatif, dicatat apa adanya.** Rencana awalnya menambah satu test yang memaku
namespace SVG dari garis-garis itu, karena batas komponen dikhawatirkan kehilangan namespace. Mutasinya
dibuktikan lebih dulu: root file anak diganti dari `<svg>` menjadi `<div>`, dan test tetap lulus, karena
Svelte 5 memutuskan namespace dari **nama tag**, bukan dari leluhurnya (`RegularElement.js:133-152` di
paket `svelte`: `is_svg(node.name)` untuk setiap nama elemen SVG). Artinya test itu tidak bisa gagal
karena alasan yang ditulisnya sendiri, jadi ia dibuang, bukan dipertahankan dengan alasan yang salah.

**Kriteria selesai.** Tidak ada file pass ini yang melewati 220 baris, dan perilaku gambar tidak berubah.
Terpenuhi: 118 dan 150 baris, dan angka browser di §8.1 identik dengan yang diukur sebelum pemisahan.

## 6. F4 LOW (FE): tab Records diukur, tidak ada celah panel yang perlu dikerjakan

**Status: CLOSED 2026-09-23** (diukur, tidak ada perubahan kode).

**Fakta.** Permintaan owner menyebut `/usage` sebagai Overview & Records, jadi tab Records diukur ulang
terhadap reference, bukan hanya Overview-nya. Reference punya sembilan kolom di tabelnya
(`RequestDetailsTab.js:255-263`): Timestamp, Model, Provider, Input Tokens, Cached, Cache Creation,
Output Tokens, Latency, Action. Panel punya dua belas, diukur dari DOM pada click-through yang direkam:
Timestamp, Request ID, Model, Provider, Endpoint, Gateway key, Tokens in / out, Cost (USD), Latency,
Status, Error code, Detail.

Jadi panel memuat seluruh fakta yang baris reference tampilkan, dan enam fakta yang tidak ada di baris
reference (request id, endpoint, gateway key, biaya, status, kode error). Yang berbeda arah sebaliknya
hanya dua:

1. Reference memisahkan token cache menjadi dua kolom per baris (`Cached`, `Cache Creation`;
   `RequestDetailsTab.js:259-260`), panel menaruh keduanya di drawer sebagai `Cache read / write`. Wire
   panel sudah membawa `tokens_cache_read` dan `tokens_cache_write` (`src/lib/schemas/usage.ts:160-161`),
   jadi ini keputusan tampilan, bukan pekerjaan kontrak (D2).
2. Reference menampilkan TTFT dan Total per baris (`RequestDetailsTab.js:313-314`), panel satu angka
   `latency_ms` (`src/lib/schemas/usage.ts:165`). Memisahkannya butuh wire lebih dulu, jadi ini permintaan
   `app-serv`, dan pass ini mencatatnya di sini alih-alih menambahkannya ke daftar permintaan yang sudah
   ada (F5 sampai F7 draft 016) tanpa nomor.

**Kriteria selesai.** Selisih tab Records terhadap reference terukur dan terklasifikasi: tidak ada
pekerjaan panel yang tertinggal tanpa keputusan. Terpenuhi.

## 7. Non-findings

- **Tidak ada dependensi baru.** Satuan kontainer adalah CSS, bukan pustaka, dan tidak ada `ResizeObserver`
  yang ditambahkan: itu justru alasan memilih `cqw` daripada JavaScript.
- **Tidak ada warna baru.** Gambar tetap memakai token status panel (`--color-ok`, `--color-warn`,
  `--color-danger`, `--color-text`), bukan palet reference.
- **Dosis gerak tidak bertambah.** Jumlah partikel, durasi, dan fase tidak disentuh pass ini; yang berubah
  hanya ukurannya, dan itu pun lewat satu unit.
- **Kontrol tidak bertambah.** Tidak ada scroll mendatar, tombol zoom, atau kontrol pan yang dipasang di
  kartu gambar (D1).
- **`app-serv` tidak disentuh.** Tidak ada perubahan kontrak, route, atau data di sisi gateway.
- **Register 007 tidak dirapikan padding-nya.** Baris Usage yang pass ini sentuh sudah menyimpang dari
  lebar separatornya sebelum pass ini (tercatat juga di draft 016), dan pass ini tidak meretrofit tabelnya.

## 8. Bukti gate

Dijalankan dari `app-ui/` pada tree beku, sidik jari `md5sum` atas `src/`, `tests/`, `static/`, dan
`scripts/` (cakupan yang sama dengan draft 016 §12): **`e4305dd056ceafa416f1dd0e50c9fb64`** untuk run penuh,
dan **`90dcb873213725b8dd4aafdd2b0478f4`** untuk tree akhir setelah tiga kelompok edit di §8.2.

| Gate              | Hasil                                                                                                                                                                                                                                                                                                                                                    |
| ----------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 error, 0 warning                                                                                                                                                                                                                                                                                                                                        |
| `bun run test`    | **2554 test lulus di 147 file**, 943 s, dijalankan pada tree bersidik jari `e4305dd056ceafa416f1dd0e50c9fb64`. Pass sebelumnya mencatat 2548 di 147 file, jadi pass ini menambah **6**: tiga di `tests/schemas/usage-topology-view.test.ts` (bagian node per lebar kotak, penurunan share saat tetangga sebaris mulai berdesakan, dan perjalanan 7 lebar kotak dikali 39 jumlah node), satu di `tests/components/usage-topology-beam.test.ts` (lebar tiga stroke dari unit), satu di `tests/components/usage-topology.test.ts` (node mengambil ukurannya dari unit), dan satu di `tests/components/usage-topology-motion.test.ts` (gateway juga) |
| `bun run lint`    | Prettier: seluruh file sesuai gaya                                                                                                                                                                                                                                                                                                                        |
| `bun run lint:ts` | ESLint keluar 0. Run pertamanya menemukan satu error nyata dan memperbaikinya, bukan menekannya: `NODE_MAX_WIDTH` di `usage-topology-view.ts` menjadi tidak terpakai setelah potongan pertama perbaikan menulis `130px` langsung di dalam `calc()` komponen. Konstanta itu diekspor dan dipakai komponen, sehingga batas 130px kembali punya satu sumber dan bundel hasil build memuatnya (`atan2(100cqw, 130px)`) |
| `bun run build`   | keluar 0, keluaran di `build/`; bundel klien memuat `container-type:inline-size` dan rumus `--u` utuh                                                                                                                                                                                                                                                      |
| File size         | file terbesar yang pass ini sentuh adalah 211 baris (`tests/schemas/usage-topology-view.test.ts`, di bawah peringatan 220). Yang dipisah: `UsageTopologyDrawing.svelte` 231 menjadi 118, dengan lapisan SVG-nya menjadi `UsageTopologyEdges.svelte` 150. Yang baru: `usage-topology-view.ts` 181, `tests/components/usage-topology-beam.test.ts` 123. Tidak ada file pass ini yang melewati 220                                                                                        |
| Text hygiene      | 0 em dash di file baru dan file yang diubah                                                                                                                                                                                                                                                                                                              |

### 8.1 Live pass: click-through browser yang direkam, di atas route nyata

Dijalankan dari `/tmp/live017` (di luar repo): panel disalin ke `/tmp/live017/app-ui`, dibangun dari sumber
yang sama, dan dijalankan di **:3001** dengan `PANEL_API_TARGET` menunjuk gateway yang sedang berjalan
milik owner di **:9090**. Port owner (3000 dan 9090) tidak disentuh sama sekali: probe "sebelum" membaca
panel :3000 tanpa menulis apa pun, dan seluruh pembuktian "sesudah" berjalan di salinan :3001 terhadap
gateway yang sama. Fixture-nya satu node `Live pass 017` (prefix `live017`) yang ditambahkan lewat API
manajemen nyata, plus satu stub upstream yang menahan jawabannya enam detik supaya request bisa diamati
saat masih in-flight, dan menjawab 500 untuk model bernama `refuse`.

Driver-nya (`/tmp/live017/drive-browser.ts`, headless Chrome lewat CDP) menjalankan sepuluh aksi dan
menulis sepuluh tangkapan layar di `/tmp/live017/shots/` serta delapan belas kelompok nilai terukur di
`/tmp/live017/evidence-browser.json`:

| Aksi                        | Nilai terukur                                                                                                                                                                                                                                                                                                 |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1360px, dibuka              | chip `Live`; kotak gambar 886 × 320; `--share` 0.2; `--u` rumus utuh dengan `container-type: inline-size`; node 95 sampai 130 lebar, 30 tinggi, font 14px; gateway 108 × 38, font 14px; `overlaps = []`; `outsideTheBox = []`; `scrollWidth == clientWidth == 1360`; tiga bacaan usage nyata (`summary` per model, `timeseries`, `summary` per provider) |
| 1360px, satu call in-flight | node aktif: border, label, dan dot `rgb(20, 116, 58)` dengan glow `color(srgb 0.0784314 0.454902 0.227451 / 0.25) 0px 0px 16px 0px` dan `animate-ping`; beam: halo 10px, plasma 5px `14px, 11px`, core 2,2px `6px, 4px`, enam orb dan lima spark `0.01px, 99.99px`, enam edge biasa; kalimat `1 in flight: Live pass 017 (stub-model).`                                                                                             |
| Gerak                       | dua bacaan `stroke-dashoffset` atas garis core yang sama: `-4.55px` lalu `-18.1564px`                                                                                                                                                                                                                        |
| 1360px, selesai             | baris `just now \| stub-model \| 7 in \| 3 out \| Live pass 017 \| Success`, dot node `rgb(143, 92, 10)` (tone "terakhir")                                                                                                                                                                                     |
| 1360px, satu call ditolak   | jawaban 502 `UPSTREAM_ERROR`; dot node `rgb(179, 38, 30)`; kalimat `The gateway last reported an error on Live pass 017.`; baris `0 in \| 0 out \| Error`                                                                                                                                                     |
| 390px, diam                 | kotak 228 × 320; node 35 sampai 47 lebar, 12 tinggi, font 4,91077px; gateway 39 × 15, font 4,91077px; `overlaps = []`; `outsideTheBox = []`; `scrollWidth == clientWidth == 390`                                                                                                                              |
| 390px, saat routing         | gateway 49 × 15 (badge jumlah ikut tampil); `overlaps = []` lagi                                                                                                                                                                                                                                             |
| 1360px lagi                 | angka tiap kotak kembali sama dengan aksi pertama; `overlaps = []`                                                                                                                                                                                                                                           |
| Tema gelap                  | `dark = true`; halo `rgb(92, 201, 123)`, core `rgb(237, 234, 230)`, node hijau `rgb(92, 201, 123)` dengan ping, dan beam tetap berjalan (`dashoffset -34.8482px`)                                                                                                                                             |
| Tab Records dan drawer      | dua belas header kolom; baris pertama memuat `7 / 3`, `Success`, `None`; drawer terbuka dengan dua belas label, termasuk `Cache read / write`, dan tertutup dengan Escape                                                                                                                                      |

Dua cacat ditemukan oleh run ini dan diperbaiki sebelum run terakhir, keduanya hanya terlihat di browser:
font yang resolve terhadap halaman (F2), dan probe yang membaca pembungkus gateway alih-alih kartunya
(font terbaca 16px padahal kartunya 14px), yang diperbaiki di driver sebelum angka di atas dicatat.

Batas yang dicatat: gambar yang diukur di sini adalah gambar dengan **tujuh** node, karena itu jumlah
provider yang terkonfigurasi di gateway saat pass ini berjalan. Model di `nodeShare` diuji untuk 2 sampai
40 node, tetapi pengukuran browsernya tujuh, dan angka itu yang ditulis di sini apa adanya.

### 8.2 Run bertarget setelah tiga kelompok edit terakhir

Tiga kelompok edit terjadi setelah run penuh dimulai, dan ketiganya dibuktikan ulang di atas tree akhir
(`90dcb873213725b8dd4aafdd2b0478f4`):

1. Satu em dash di komentar `UsageTopologyDrawing.svelte` diganti titik dua, karena aturan teks pass ini
   tidak mengizinkan em dash. Satu karakter di komentar, tanpa perubahan perilaku, tetapi tetap dibuktikan
   ulang alih-alih diklaim tidak berpengaruh.
2. Bagian ke-25 `app-ui/README.md` ditambahkan (dokumen, tidak dibaca test mana pun).
3. Rujukan "draft 017" di komentar enam file diganti "draft 018", karena dokumen ini ternyata bernomor 018:
   `docs/DRAFT/017` sudah dipakai pass `app-serv` yang berjalan bersamaan. Enam baris komentar, dan test
   yang menyebut nomor draft itu ikut dibuktikan ulang di bawah.

Pembuktiannya: run bertarget atas empat file yang menyentuh gambar (`tests/components/usage-topology.test.ts`,
`usage-topology-beam.test.ts`, `usage-topology-motion.test.ts`, dan `tests/schemas/usage-topology-view.test.ts`)
lulus **54 test di 4 file** dalam 89,9 s, dan keempat gate cepat dijalankan ulang pada tree akhir:
`bun run check` 0 error 0 warning, `bun run lint` bersih, `bun run lint:ts` keluar 0, `bun run build`
keluar 0 dengan bundel klien memuat `container-type:inline-size`. Sidik jari tree akhir tercatat di atas,
dan `bun run lint` dijalankan sekali lagi setelah baris tabel README ini ditulis, karena Prettier
memformat tabel markdown di file itu.

## 9. Status per 2026-09-23

- F1 sampai F4 **CLOSED**; draft 016 F8 **CLOSED** oleh F1, dan barisnya di register 007 diperbarui.
- Commit lokal scope `app-ui` saja: gambar, schema, dan empat file test yang menyertainya. Belum di-push.
- Yang tidak dikerjakan dan tetap terbuka, tanpa berubah dari draft sebelumnya: F5 sampai F7 draft 016
  (permintaan `app-serv`: periode `All`, dimensi `Account`, drawer empat body), F13 register 007 (klaim
  versi Bun di README), dan F12 register 007 (click-through U0/U1 yang belum tercatat).
- Dua pilihan yang pass ini tidak ambil, tercatat di §2 supaya bisa dibalik: bentuk perbaikan F8 (D1) dan
  dua kolom cache per baris di tab Records (D2).
