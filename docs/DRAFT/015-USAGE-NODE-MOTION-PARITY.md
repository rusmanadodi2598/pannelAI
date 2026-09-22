# 015-USAGE-NODE-MOTION-PARITY.md: Parity Gerak Node `/usage` (app-ui)

Dokumen kerja lanjutan `013-USAGE-NODE-MOTION-READINESS.md`, dengan scope **hanya `app-ui/.`**. Kontrak
tetap `docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel) dan `DESIGN.md` (arah visual). Pola mengikuti `008`
sampai `014`: temuan bernomor F, bukti yang bisa diulang, rencana DURING, keputusan owner di depan
implementasi.

| | |
|---|---|
| **Status** | **CLOSED 2026-09-22.** F1 sampai F6 DURING selesai dan seluruh gate lulus; tidak ada temuan OPEN di scope panel |
| **Mechanism** | DURING & AFTER (antislop) |
| **Tanggal** | 2026-09-22 |
| **Scope** | `app-ui/.` saja. `app-serv/.` tidak disentuh oleh dokumen ini |
| **Layar** | `/usage` (tab Overview, panel Live routing) |
| **Reference fork** | `https://github.com/decolua/9router`, dibaca di `origin/master` commit `a8c9d380` (checkout `/home/rusmanadodi/apps/9router`), `ProviderTopology.js` dan `src/app/globals.css`; di akhir pass `origin/master` sudah di `21583c03` dan kedua file itu tidak berubah di antaranya |
| **Kaitan** | SPEC-UI §6.5; DESIGN.md §2.1, §11 (R-13, R-19, R-31); draft 013 F1 sampai F3; draft 007 F12 |

## 1. Ringkasan

Draft 013 menutup gerak node dengan satu gerak: dash berjalan di edge aktif. Owner membuka pass ini dengan
arahan yang sama seperti 013 tetapi lebih tegas pada bentuknya: **UI Node Animation 1:1 dengan REFERENCE**,
lalu memilih opsi "Gerak 1:1, warna panel" saat bentuk gerak dan warna disodorkan sebagai dua pilihan
terpisah. Jadi yang 1:1 adalah **geraknya**, sedangkan warnanya tetap token panel.

Reference, pada `origin/master`, menggambar edge aktif sebagai **beam tiga lapis** dengan partikel:
`ProviderTopology.js:137-245` memuat halo (lebar 10, opacity 0.35, `#22d3ee`), plasma (lebar 5, opacity
0.85, `#4ade80`), core putih (lebar 2.2), enam orb (`KAME_PARTICLE_COUNT = 6`, `:22`) yang bergerak dengan
`dur` `0.4 + i * 0.08` dan `begin` `i * 0.09`, serta lima spark (`SPARK_COUNT = 5`, `:23`) yang berkedip
dengan `values="0;1;0;0;1;0"`, `dur` `0.35 + (i % 3) * 0.1`, dan `animateMotion` `0.28 + i * 0.05`. Node
pusatnya (`:99-130`) berdenyut, ikonnya bergoyang, labelnya berkedip, dan badge-nya bercahaya selama ada
request in-flight (`globals.css:476-524`).

Pass ini memindahkan keenam bagian itu ke panel, dengan dua penyesuaian yang disebut di F1 dan F4 dan satu
override yang dicatat di F3.

## 2. Keputusan owner yang dipakai

| # | Pertanyaan | Jawaban | Konsekuensi |
|---|---|---|---|
| D1 | Mode antislop | DURING & AFTER | Aturan diterapkan sambil menulis, ditutup laporan Delivery Gate empat blok |
| D2 | Seberapa literal parity gerak | "Gerak 1:1, warna panel" | Gerak, timing, jumlah partikel, dan lapisan beam mengikuti reference; warnanya memakai `--color-ok`, `--color-text`, dan token panel lain, bukan neon reference |
| D3 | Register pass | DRAFT 015 baru | Dokumen ini, terpisah dari 013 |
| D4 | Commit | Setelah progres CLOSED, hanya scope `app-ui` | Satu commit lokal tanpa push |

## 3. F1 MEDIUM (FE): edge aktif hanya membawa satu dash, bukan beam reference

**Status: DURING selesai.**

**Fakta.** Pada `9668e3c` edge aktif adalah satu `<line>` dengan `stroke-dasharray:3 3` dan satu animasi
`flow` (draft 013 F1). Reference, pada layar yang sama, menggambar tiga lapis stroke dengan filter turbulensi
sendiri, enam orb, dan lima spark: `ProviderTopology.js:137-245`. Dash tunggal itu memang parity untuk
"edge bergerak", bukan parity untuk bentuk geraknya.

**Risiko dan aturan.** R-19 (gerak harus punya tujuan tertulis); R-29 (palet: satu accent plus warna status,
jadi warna per-provider reference tidak dipakai); R-26 (setiap bagian gerak harus berarti); SPEC-UI §6.5.

**Rencana DURING (FE).**

1. `usage-beam.ts`: kosakata beam dipisah dari `usage-topology-view.ts` sebagai angka murni, sehingga
   komponen tidak menghitung apa pun.
2. `UsageTopologyDrawing.svelte`: satu `<line>` per lapis (halo, plasma, core) dengan `pathLength="100"`,
   `vector-effect="non-scaling-stroke"`, plus enam orb dan lima spark sebagai dash ber-`stroke-linecap:round`
   yang bergerak dengan `animate-beam-travel`.
3. Gerak tetap digate dua kali seperti draft 013 F2: state provider aktif **dan** frame benar-benar tiba.
4. Angka partikel, lebar, kecepatan, dan fase diambil dari reference; **periode dash tidak**, karena jalur
   panel dinormalisasi ke 100 satuan sementara reference memindahkan offset -36 atas pola 14 dan 24
   (`globals.css:504-535`), jadi dash reference melompat di setiap siklus. Panel memilih periode 10 dan 25
   yang membagi 100 tepat, sehingga loop-nya mulus.
5. `prefers-reduced-motion` tetap mematikan semuanya, dan partikel yang tidak punya keadaan diam
   disembunyikan (`motion-reduce:hidden`).

**Kriteria selesai.** Tepat edge provider aktif yang membawa tiga lapis beam, enam orb, dan lima spark; angka
setiap partikel sama dengan reference; setiap gerak padam saat stream tidak live dan saat reduced motion.

## 4. F2 LOW (FE): dua cacat yang hanya terlihat di browser, keduanya diperbaiki

**Status: DURING selesai.**

**Fakta.** Dua rekaman browser berturut-turut menemukan dua cacat yang tidak bisa dilihat test jsdom, karena
test-nya memeriksa atribut dan bukan piksel:

1. **Warna titik.** Versi pertama memakai satu warna untuk semua partikel (`--color-ok`). Diukur di browser
   pada kedua tema, titik itu menyatu dengan stroke plasma dan seluruh beam terbaca sebagai satu batang
   padat. Reference menghindari masalah yang sama dengan mengalternasi warna orb-nya (`#fde047`, `#67e8f9`,
   `#fff`).
2. **Kepadatan titik.** Versi kedua memakai jumlah partikel sebagai periode dash (`100 / 6`), jadi tiap
   garis orb melukis enam titik: edge membawa 61 titik (36 dari orb, 25 dari spark) padahal reference
   membawa 11, dan tangkapan layar 08 dan 10 memperlihatkan rantai titik, bukan partikel yang berjalan.
   Reference memakai satu lingkaran per orb dengan `animateMotion`, bukan pola dash berulang.

**Risiko dan aturan.** R-19 (gerak yang tidak terbaca bukan indikator); R-26 (setiap bagian harus berbuat);
R-25 (setiap pasangan warna baru harus terukur); R-35 (klaim click-through harus direkam, bukan diasumsikan).

**Rencana DURING (FE).**

1. `BeamTone` menjadi `'ok' | 'text'`; setiap orb ketiga memakai `--color-text`, yang dekat hitam di tema
   terang dan dekat putih di tema gelap, dan itu juga tone yang reference berikan pada core-nya. Tidak ada
   warna baru, jadi tidak ada pasangan kontras baru.
2. `BEAM_PARTICLE_DASH = '0.01 99.99'` menggantikan pola per-partikel: satu titik dengan periode tepat 100,
   sehingga satu garis adalah satu pelancong dan siklusnya tetap mulus. Angka lebar, durasi, dan fase tetap
   milik reference, jadi yang berubah hanya berapa kali satu titik dicetak per garis.
3. Seluruh click-through direkam ulang setelah kedua perbaikan, dan dua test baru memaku keduanya: satu di
   lapisan schema (periode tepat 100, celah di atas 90) dan satu di lapisan komponen (setiap garis partikel
   membawa `stroke-dasharray: 0.01 99.99`).

**Kriteria selesai.** Setiap orb ketiga membawa kelas `stroke-[var(--color-text)]`; setiap garis partikel
melukis satu titik; tangkapan browser memperlihatkan beam berlapis dengan titik yang berjalan, bukan rantai.

## 5. F3 LOW (FE): glow gateway bertabrakan dengan dose cap R-13, dicatat sebagai override

**Status: DURING selesai, override tercatat.**

**Fakta.** Reference memberi router-nya tumpukan empat lapis glow: `0 0 8px #fde047`, `0 0 20px` primary,
`0 0 32px` cyan, dan satu inset (`globals.css:477-493`), plus bayangan badge dua lapis (`:523-525`). R-13
membatasi glow pada satu sampai dua elemen dengan dosis rendah. Owner memilih parity gerak, bukan parity
dosis.

**Risiko dan aturan.** R-13 (dose cap); R-37 (bila arah yang diminta bertabrakan dengan aturan, sebutkan
dan catat jawabannya); R-31.

**Rencana DURING (FE).** Siklus dan transformasi diambil apa adanya (0.75s pulse `scale(1.06)`, 0.45s shake
`rotate(±5deg) scale(1.08)`, 0.7s flicker), sedangkan tumpukan neon-nya menjadi **satu lapis** per elemen di
warna status: card `0 0 8px`/`0 0 14px`, badge `0 0 10px`, label `text-shadow` 6px/10px. Tidak ada `inset`,
tidak ada lapis kedua, dan ikon tidak diberi `drop-shadow`.

**Kriteria selesai.** Empat bagian gateway bergerak dengan siklus reference, masing-masing membawa satu
lapis glow, dan alasannya tertulis satu baris di DESIGN.md §11.

## 6. F4 LOW (FE): turbulensi reference dianimasikan dengan SMIL, panel memakai tekstur diam

**Status: DURING selesai, penyimpangan tercatat.**

**Fakta.** Filter reference menganimasikan `baseFrequency` `0.8;1.4;0.8` selama 0.25s dengan `<animate>`
(`ProviderTopology.js:167-172`) dan memakai `feDisplacementMap scale="3.5"` di koordinat piksel. SMIL tidak
membaca `prefers-reduced-motion`, jadi gerak itu akan tetap berjalan untuk pembaca yang memintanya berhenti.
Selain itu 3.5 satuan di jalur sepanjang ratusan piksel bukan 3.5 satuan di kotak 100 satuan panel.

**Risiko dan aturan.** R-19; SPEC-UI §11 (reduced motion); R-31.

**Rencana DURING (FE).** Tekstur `feTurbulence` reference dipertahankan diam (`baseFrequency 0.9`,
`numOctaves 2`, `seed 2`), dan displacement-nya 0.4 supaya proporsional terhadap kotak 100 satuan. Geraknya
datang dari dash, bukan dari filter, sehingga satu sumbu preferensi mematikan seluruh gerak.

**Kriteria selesai.** Filter tidak berisi elemen animasi, dan satu komentar menyebut skala serta alasannya.

## 7. F5 LOW (FE): blok gerak mendorong dua file melewati batasnya, keduanya dipisah

**Status: DURING selesai.**

**Fakta.** Setelah blok gerak masuk, `src/app.css` menjadi 269 baris (di atas ceiling 250) dan
`src/lib/schemas/usage-topology-view.ts` menjadi 236 (di atas ambang 220). File test komponen gerak juga
tumbuh ke 243 baris.

**Risiko dan aturan.** SPEC-UI §10 (batas 250, ambang peringatan); pola repo: file yang melewati ambang
dipisah menurut concern, bukan dipangkas.

**Rencana DURING (FE).**

1. Token dan keyframes gerak pindah ke `src/lib/motion.css`, diimpor oleh `app.css`, sehingga root layout
   tetap punya satu entry point (SPEC-UI §10.6). Hasil build diverifikasi memuat keempat keyframes dan
   kelas `animate-*`-nya, bukan hilang diam-diam.
2. Kosakata beam pindah ke `src/lib/schemas/usage-beam.ts`.
3. Test komponen gerak dipisah menurut concern menjadi `usage-topology-beam.test.ts` (bentuk dan angka
   beam) dan `usage-topology-motion.test.ts` (gate gerak dan gateway), dengan harness bersama di
   `tests/support/topology-harness.ts`.

**Kriteria selesai.** Setiap file yang pass ini sentuh di bawah ambang 220, dan tidak ada file baru yang
melewati 250.

## 8. F6 LOW (FE): dua sitasi reference salah baris, diverifikasi ulang ke `origin/master`

**Status: DURING selesai.**

**Fakta.** Komentar `usage-beam.ts` dan `UsageTopologyDrawing.svelte` menyebut `ProviderTopology.js:190-242`
dan `globals.css:526-534`. Diverifikasi ke `origin/master` `a8c9d380`: komponen beam-nya `:137-245`, kelas
edge-nya `globals.css:526-535`, glow node provider di `:41-42`, dan keyframes router di `:477-524`. Sitasi
lama menunjuk baris yang di checkout kerja malah berisi hal lain, karena checkout itu **447 commit di
belakang `origin/master`** dan di sana beam-nya belum ada sama sekali.

**Risiko dan aturan.** R-38 (komentar yang menyatakan fakta tidak boleh salah atau basi); R-35 (klaim harus
direkam, bukan diasumsikan).

**Rencana DURING (FE).** Semua sitasi diperbaiki ke baris yang benar di `origin/master`, commit-nya
disebut di header modul, dan setiap penyimpangan angka (periode dash di F1, displacement di F4) ditulis di
tempat angkanya dideklarasikan. Dua nama test yang menyebut "reference" untuk angka yang sebenarnya angka
panel diganti namanya. Satu `git fetch` di akhir pass memindahkan `origin/master` ke `21583c03` (v0.5.85),
dan `git diff a8c9d380 21583c03` atas kedua file reference mengembalikan nol baris, jadi angka dan baris
yang dikutip tidak berubah di tip baru.

**Kriteria selesai.** Setiap sitasi `file:line` di file yang pass ini sentuh menunjuk isi yang benar di
`origin/master`, dan angka yang bukan milik reference disebut sebagai angka panel.

## 9. Non-findings

- **Tidak ada dependensi baru.** Beam adalah SVG dan CSS, bukan pustaka graf.
- **Pan-and-zoom tetap tidak dipakai.** Reference menggambar di canvas React Flow yang bisa digeser dan
  di-zoom; panel tetap satu gambar responsif, seperti keputusan draft 012.
- **Layout, state precedence, dan kalimat teks tidak berubah.** Pass ini hanya menambah gerak dan perakuan
  pada state yang sudah ada.
- **`/quota`, `/logs`, dan `/playground` tidak disentuh.**
- **Angka agregat tetap milik REST.** Tidak ada field agregat yang berpindah tangan.
- **Frame live tetap dari contract double** selama `GET /api/v1/usage/live` belum ada di `app-serv`
  (draft 013 F4), dan itu disebut di setiap baris bukti live. Route itu diukur ulang pada gateway yang
  dibangun dari `f0435bc` dan tetap menjawab 404.
- **Padding tabel register 007 tidak dirapikan.** Himpunan baris yang menyimpang dari lebar separatornya
  sama di `HEAD` dan di tree ini: enam baris (11, 36, 37, 38, 39, 47). Baris ke-39 adalah baris Usage yang
  pass ini perluas, dan ia sudah menyimpang sebelum pass ini, jadi pass ini tidak meretrofitnya.
- **`SYSTEM_MAP.md` tidak berubah.** Pass ini tidak menyentuh batas domain, struktur data, atau topologi
  layanan.

## 10. Bukti gate

Dijalankan dari `app-ui/` pada tree beku (`md5sum` atas `src/` dan `tests/`:
`b949347ed1f2b04e75d4da77544791d6`).

| Gate | Hasil |
| --- | --- |
| `bun run check` | 0 error, 0 warning |
| `bun run test` | 2508 tes lulus di 144 file (pass terakhir yang tercatat: 2492 di 142 file; pass ini menambah 16, yaitu tujuh di `tests/components/usage-topology-beam.test.ts` dan sembilan di `tests/schemas/usage-beam.test.ts` yang keduanya baru, sementara `tests/components/usage-topology-motion.test.ts` tetap delapan baris karena baris beam-nya pindah ke file baru). Run penuh 1265 detik. Satu editan komentar yang mendarat setelah run mulai (header modul beam, yang menyebut dua angka panel padahal periode dash-nya tiga) dibuktikan ulang dengan run terarah atas enam file (`tests/components/usage-topology-beam.test.ts`, `tests/components/usage-topology-motion.test.ts`, `tests/schemas/usage-beam.test.ts`, `tests/schemas/usage-topology-view.test.ts`, `tests/components/usage-topology.test.ts`, `tests/components/usage-live-drawing.test.ts`) yang mencetak 64 tes lulus, dijalankan ulang saat penutupan di atas tree ber-fingerprint `13b9f55ead87caf26fcacea3651ad202`; angka run terarah yang sempat tercatat (lima file, 44 tes) tidak dapat direproduksi saat diukur ulang, jadi baris ini memakai angka terukur beserta daftar file yang diperiksa. Kedua fingerprint tree: `b949347ed1f2b04e75d4da77544791d6` sebelum editan komentar, `13b9f55ead87caf26fcacea3651ad202` sesudah |
| `bun run lint` | Prettier: seluruh file sesuai |
| `bun run lint:ts` | ESLint keluar 0 |
| `bun run build` | sukses, exit 0, keluaran di `build/` |
| Verifikasi CSS hasil build | keluaran build memuat keenam keyframes (`beam-dash`, `beam-flicker`, `beam-blink`, `router-pulse`, `router-shake`, `router-flicker`) dan kelas `animate-beam-*`/`animate-router-*`; token gerak pindah ke `src/lib/motion.css` dan tetap dihasilkan, jadi pemisahan file tidak menghilangkan satu animasi pun |
| Ukuran file | file terbesar yang pass ini sentuh 211 baris (`UsageTopologyDrawing.svelte`, di bawah ambang 220). Tiga file pindah isi, diukur tiga titik supaya tidak terbaca sebagai angka `HEAD`: `app.css` 189 di `HEAD`, 269 di tengah pass, 180 sesudah token gerak pindah ke `motion.css`; `usage-topology-view.ts` 139 di `HEAD`, 236 di tengah pass, 140 sesudah kosakata beam pindah ke `usage-beam.ts`; `UsageTopology.svelte` 201 di `HEAD`, 117 sesudah gambarnya pindah ke komponen sendiri. File baru semuanya di bawah ambang (`motion.css` 102, `usage-beam.ts` 112, `topology-harness.ts` 81, test 105/114/95). Tidak ada file over-cap lama yang disentuh |
| Kebersihan teks | 0 em dash di seluruh file baru dan yang diubah |

### 10.1 Live pass: click-through browser yang direkam

Chrome headless menggerakkan panel hasil `bun run build` di atas pasangan proses nyata. Gateway dibangun
dari **tree yang di-commit** (`git archive HEAD f0435bc`), bukan dari working tree, karena ada pekerjaan
`app-serv` yang belum di-commit di sana dan pass panel tidak boleh bergantung padanya. Route
`GET /api/v1/usage/live` diukur ulang pada gateway itu dan tetap **404**, jadi frame live tetap datang dari
**contract double** seperti draft 013 F4. Sebelas tangkapan layar di `/tmp/live015/shots/` dan sebelas
kelompok nilai terukur di `/tmp/live015/evidence.json`:

1. Login lewat form panel sendiri.
2. Frame tiba: dua edge routing (Edge TTS dan Google TTS), chip `Live`.
3. Beam: halo (lebar 10, opacity 0.35, `beam-flicker` 0.18s), plasma (lebar 5, opacity 0.85,
   `14px, 11px`, `beam-dash` 0.18s + `beam-flicker` 0.22s), core (lebar 2.2, `6px, 4px`, `beam-dash`
   0.22s, `rgb(26, 22, 20)` di tema terang), plus satu filter turbulensi per edge aktif.
4. Partikel: 12 orb dan 10 spark untuk dua edge, yaitu enam orb dan lima spark per edge; lebar 8/5/3.6,
   durasi `0.4` sampai `0.8` dan `0.28` sampai `0.48`, delay `0` sampai `-0.45` dan `0` sampai `-0.44`,
   `stroke-dasharray: 0.01px, 99.99px` (satu titik per garis), dan setiap orb ketiga `rgb(26, 22, 20)`.
5. Bukti gerak: `stroke-dashoffset` core berubah antara dua pembacaan (`-40.79px` lalu `-77.20px`) dan
   transform gateway terbaca di tengah denyut (`matrix(1.05258, ...)`, `router-pulse` 0.75s).
6. Node aktif: border `rgb(20, 116, 58)`, glow `0 0 16px` pada alpha 0.25, label status, `transition: all
   0.3s`, satu denyut `ping`. Node idle: border `rgb(223, 217, 208)`, tanpa glow, tanpa denyut.
7. `Pause live updates`: nol beam, animasi gateway `none`, chip hitungan tetap `2`, warna state tetap.
   Resume mengembalikan beam.
8. Reduced motion: partikel `display: none`, core tanpa animasi, gateway tanpa animasi.
9. Tema gelap: token bertukar (`rgb(92, 201, 123)` untuk halo, `rgb(237, 234, 230)` untuk core) dan node
   aktif membawa glow token gelap.
10. Double menutup stream: `Unavailable`, nol gerak, state terakhir tetap, dan kalimat ringkasannya utuh.
11. `Try again` setelah double menjawab lagi: beam kembali. Viewport 390px: `scrollWidth == clientWidth ==
    390` dan beam tetap ada.

**Cacat yang ditemukan pass ini dan diperbaiki.** Versi pertama memakai jumlah partikel sebagai periode
dash, jadi tiap garis orb melukis enam titik: edge membawa 61 titik padahal reference membawa 11, dan di
browser beam terbaca sebagai rantai, bukan sebagai partikel (tangkapan 08 dan 10 pada rekaman pertama).
Perbaikannya `BEAM_PARTICLE_DASH = '0.01 99.99'`, satu titik dengan periode tepat 100, dan seluruh
click-through direkam ulang setelah perbaikan. jsdom tidak bisa melihat cacat ini: test-nya memeriksa
atribut, bukan piksel.

**Jebakan yang tercatat.** Proses harness sesi sebelumnya masih memegang port 4100 dan 9090, sehingga
gateway yang boot pass ini mati di `EADDRINUSE` dan rekaman pertama berjalan di atas binary gateway yang
lebih lama. Port diperiksa, PID basi dihentikan lewat PID yang dibaca dari `ss`, dan pass dijalankan ulang
di atas build HEAD; itulah sebabnya bukti di atas menyebut `f0435bc`.

Baseline DB dihitung sebelum dan sesudah dan kembali persis: `usage=0 logs=0 keys=1 endpoints=1 upkeys=0
nodes=2 caps=0 settings=0 auth_null=0 media_settings=0 proxies=0`. Ketiga proses dihentikan lewat PID yang
dicatat dan ketiga port tertutup.

Yang **tidak** diklaim: frame live datang dari double, bukan dari gateway. Route nyatanya masih 404 di
`f0435bc`, jadi CLOSED F2/F3 di draft 012 tetap ditahan.

## 11. Status per 2026-09-22

- **CLOSED 2026-09-22.** F1 sampai F6 DURING selesai; tidak ada temuan OPEN di scope panel.
- Penutupan (closing sweep) mengukur ulang dua angka di baris gate §10 dan memperbaiki keduanya:
  run terarah yang membuktikan editan komentar ternyata enam file dan 64 tes saat dijalankan ulang,
  bukan lima file dan 44 tes; dan angka `usage-topology-view.ts` dipecah menjadi 139 di `HEAD`, 236 di
  tengah pass, 140 sesudahnya, karena bentuk lamanya terbaca seolah `HEAD`-nya 236.
- Register 007: F12 naik penyebutnya saja, dari dua belas dari dua puluh dua menjadi dua belas dari dua
  puluh tiga bagian, karena bagian ke-23 membawa click-through yang direkam dan menyebut batasnya tanpa
  memakai kata outstanding untuk click-through-nya; F13 naik baris penyebutnya dari lima ke enam (baris
  pembuka pass ini memakai versi terukur `1.3.0`), jumlah klaimnya tetap sembilan belas situs.
- Commit lokal: `feat(app-ui): give the live drawing the reference fork's beam, particles, and gateway
  motion (draft 015 F1 to F6)`, tanpa push.
