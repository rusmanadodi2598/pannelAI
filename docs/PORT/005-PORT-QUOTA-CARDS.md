# 005-PORT-QUOTA-CARDS.md: Kartu per provider, lipat, lipat massal, paginasi (app-ui)

Dokumen kerja pass panel `app-ui` atas permukaan Quota Tracker, lanjutan langsung pass 004 pada
halaman yang sama. Pola mengikuti draft 016 sampai 025: temuan bernomor F, bukti yang bisa diulang,
keputusan yang diambil di depan, gerbang terukur, dan status yang jujur.

|                      |                                                                                                                                                                                                                                                                                                                                                                |
| -------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**           | **CLOSED 2026-09-26.** D1-D8 dipakai; kartu per provider dengan isi menggulir, lipat, lipat massal, dan paginasi 5 per halaman terbangun dan teruji; seluruh gerbang hijau dan click-through A-G di §7; SPEC-UI §6.6 diamendemen                                                                                                                               |
| **Mechanism**        | DURING & AFTER (antislop)                                                                                                                                                                                                                                                                                                                                      |
| **Scope**            | `app-ui/.` saja. Gateway tidak disentuh; wire `/api/v1/quotas` tidak berubah                                                                                                                                                                                                                                                                                   |
| **Permintaan owner** | (1) Halaman compact dan simetris: penanganan pagination, tiap kartu bisa dilipat, dan lipat massal dengan ceklis supaya cepat. (2) Pagination dibatasi maksimal 5 kartu per halaman. (3) Satu kartu per Provider dengan isi kartu yang bisa discrolling, karena akun atau key provider bisa ratusan bahkan ribuan; bila 1 key = 1 card, monitoring menyulitkan |
| **Reference**        | `decolua/9router` (`39e36d3d`, v0.5.86); bentuk kartu per provider dengan daftar yang menggulir adalah generalisasi langsung dari ProviderLimits yang sudah diport di pass 004                                                                                                                                                                                 |
| **Kaitan**           | SPEC-UI §6.6 (amendemen di pass ini; batas D7 pass 004 hangus karena pohon kini bersih), §8.6; draft 005 (vertical kuota app-serv); PORT 004 `docs/PORT/004-PORT-QUOTA-TRACKER.md`                                                                                                                                                                             |
| **Tanggal**          | 2026-09-26 (DURING)                                                                                                                                                                                                                                                                                                                                            |

## 1. Ringkasan

Pass 004 membangun kartu per endpoint di bawah judul provider. Skala yang owner sebut (ratusan
sampai ribuan key per provider) membalik bentuknya: satu kartu per provider, isi berupa daftar
endpoint dan window yang menggulir, kartu bisa dilipat, lipat massal lewat ceklis, dan paginasi
maksimal 5 kartu per halaman menjaga halaman tetap compact pada skala mana pun.

## 2. Bukti terukur

### 2.1 Skala yang menyandera bentuk lama

- `GET /providers`: **112 baris** (2026-09-26, dua halaman; 86 pada 2026-09-24), katalog
  `GET /models/catalog`: **937 baris** (587 pada 2026-09-24). Pertumbuhan dua hari membenarkan
  kekhawatiran skala owner.
- `GET /api/v1/quotas` hari ini: 16 window, terkelompok menjadi 2 kelompok provider
  (`openai-compatible-0386BKG9...` dengan 3 endpoint, "No provider" dengan 1 endpoint virtual dan
  4 window). Dengan 1 kartu per endpoint, halaman membesar linear terhadap jumlah key; dengan
  1 kartu per provider dan paginasi 5 per halaman, tinggi halaman terikat.

### 2.2 Bentuk sebelum pass ini ( QuotaCards.svelte, 142 baris)

- Judul provider `h3`, lalu grid dua kolom berisi kartu `h4` per endpoint; tiap kartu memuat baris
  window (kind, badge sumber, persen, palang, penghitung, baris reset).
- Tidak ada lipat, tidak ada paginasi, tidak ada seleksi; satu-satunya pengikat tinggi halaman
  adalah jumlah data.
- Bukti click-through pass 004: toolbar satu basaat (157 = 157), caps satu baris, 390 px tanpa
  overflow, `windowErrors: "none"`; keempatnya harus tetap hijau setelah rework.

## 3. Keputusan (D)

| #   | Pertanyaan                | Keputusan                                                                                                                                                                                                                                                                 |
| --- | ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| D1  | Kartu untuk apa?          | **Satu kartu per provider** (dan satu kartu "No provider"). Header kartu membawa nama, hitungan ringkas (N endpoint, M window), ceklis seleksi, dan tombol lipat. Isi kartu: daftar endpoint dan window-nya, menggulir saat panjang. Owner butir 3                        |
| D2  | Lipat default?            | **Terbuka secara default**, lipat adalah kemampuan, bukan keadaan awal: paginasi 5 per halaman yang mengikat tinggi halaman, dan isi kartu memang untuk dibaca. Kartu terlipat tetap informatif lewat hitungan di headernya (R-27)                                        |
| D3  | Mekanisme lipat?          | Tombol chevron di header dengan `aria-expanded` + `aria-controls`; isi disembunyikan (`hidden`) saat terlipat, jadi pembaca layar dan test melihat keadaan yang sama. Ceklis adalah kontrol terpisah di header, tidak menglipat                                           |
| D4  | Lipat massal?             | Ceklis per kartu; saat seleksi lebih dari nol, satu baris aksi muncul dengan "Fold selected (n)" dan "Unfold selected (n)". Seleksi dibersihkan saat pindah halaman (dikunci test), supaya aksi massal selalu terlihat sasarannya                                         |
| D5  | Paginasi apa dan di mana? | Paginasi klien atas kartu provider hasil pengelompokan, **5 kartu per halaman** (`PAGE_SIZE = 5`), baris "Page X of Y" dengan Previous/Next di bawah kartu, nonaktif di batas. Wire tidak berubah; paginasi server-side window adalah filing terpisah bila skala menuntut |
| D6  | Menggulir di mana?        | Isi kartu `max-h-80` (320 px) + `overflow-y-auto`; halaman tetap tidak menggulir mendatar (R-03), tabel lebar tetap menggulir dalam wadahnya sendiri sesuai konvensi panel                                                                                                |
| D7  | SPEC-UI §6.6?             | **Diamendemen di pass ini** (pohon bersih, batas D7 pass 004 hangus): bentuk kartu per provider, lipat, lipat massal, paginasi 5 per halaman, dan wilayah menggulir tertulis; sekaligus menutup residu §7.4 pass 004                                                      |
| D8  | Salinan                   | Bahasa Inggris jujur tanpa emoji; baris aksi massal menyebut angka seleksi yang nyata, bukan "all" yang tidak menjelaskan cakupannya                                                                                                                                      |

## 4. Findings

- **F1 (HIGH, arsitektur tampilan):** 1 kartu per endpoint melanggar skala yang owner sebut;
  diukur 112 provider / 937 baris katalog dan naik dua hari ini saja. Ditutup oleh D1.
- **F2 (MEDIUM):** tidak ada paginasi; jumlah kartu tak terikat. Ditutup oleh D5.
- **F3 (MEDIUM):** tidak ada lipat dan tidak ada seleksi; memantau banyak provider memaksa
  menggulir panjang. Ditutup oleh D2-D4.

## 5. Rencana (TDD, merah dulu)

1. `tests/components/quota.test.ts` dirombak: kartu per provider (satu `h3` kartu per kelompok,
   bukan kartu `h4` per endpoint), hitungan header, lipat (klik chevron, `aria-expanded` false +
   isi `hidden`, kembali terbuka), kartu "No provider" ikut berbentuk kartu.
2. Test baru paginasi: dengan 7 provider di-stub, halaman 1 tepat 5 kartu, Next membawa 2 kartu di
   halaman 2, Previous kembali, tombol nonaktif di batas, label "Page 1 of 2".
3. Test baru lipat massal: ceklis dua kartu, baris aksi tampil dengan angka 2, "Fold selected"
   melipat keduanya, "Unfold selected" membuka lagi, seleksi bersih saat pindah halaman.
4. Test layout: wilayah isi memakai `overflow-y: auto` dan tinggi maksimum terukur (getComputedStyle),
   baris paginasi dan baris aksi massal dalam satu tata letak yang tidak merusak toolbar rumah
   (157 = 157 tetap), 390 px tetap tanpa overflow.
5. Implementasi: `QuotaCards.svelte` dirombak (kartu, lipat, ceklis, gulir; paginasi dan baris aksi
   massal tinggal di komponen yang sama, karena keduanya presentasi murni atas pengelompokan milik
   komponen, dan `+page.svelte` tidak berubah); skema dan caps tidak tersentuh; SPEC-UI §6.6
   diamendemen.
6. Gerbang: prettier, eslint, svelte-check, build, test terarah, suite penuh tanpa beban bersama,
   click-through (desktop + 390 px) dengan driver CDP; Delivery Gate; commit saat CLOSED.

## 6. Implementasi

- `QuotaCards.svelte` (dirombak): kartu per provider dari potongan `visible` atas kelompok
  first-seen; header memuat ceklis yang dibungkus label 44 px (target sentuh, kotaknya tetap
  native), nama `h3`, hitungan "N endpoints, M windows", dan tombol lipat icon-only
  (`aria-label` + `title` "Fold <provider>", `aria-expanded`, `aria-controls`); isi kartu
  `max-h-80 overflow-y-auto` dengan judul baris `h4` per endpoint dan baris window yang tidak
  berubah dari pass 004. Baris aksi massal (`role="status"`) tampil hanya saat ada seleksi, dengan
  "N selected", "Fold selected", dan "Unfold selected" (`min-h-11`). Paginasi: `nav`
  "Quota card pages", Previous/Next icon-only dari `CONTROL_ICONS.previous/next`, label
  "Page X of Y", nonaktif di batas. Nomor halaman di-clamp lewat `safePage` ($derived) ketika
  data menyusut; seleksi dibersihkan pada tiap perpindahan halaman (D4). Nol `Set`/`Map` dalam
  state (pelajaran pass 004): lipat dan seleksi adalah array string yang di-reassign.
- `src/lib/icons.ts`: satu glyph baru `fold` (ChevronDown) dengan alasan tertulis (R-04);
  `previous` dan `next` sudah ada dan dipakai ulang.
- `QuotaCardBody.svelte` (baru, 100 baris): isi kartu (kalimat no-provider, baris `h4` per
  endpoint, baris window) dipindah utuh dari QuotaCards setelah rework membuat berkas itu 301
  baris (di atas batas 250); QuotaCards kini 221 baris. Markup dipindah apa adanya (nol perubahan
  DOM), wrapper scroll tetap di QuotaCards karena `aria-controls` menunjuk ke sana, dan kalimat
  "Source: ..." tetap di QuotaCards sebagai salinan tingkat layar.
- `+page.svelte`: tidak berubah. `QuotaCaps.svelte`: tidak berubah. Skema: tidak berubah.
- Test: `quota.test.ts` kini 20 kasus (14 lama dijawikan ke bentuk kartu + 6 baru: kartu per
  provider, lipat satu kartu, lipat massal dua arah, lipat tanpa seleksi, paginasi 7 provider,
  seleksi bersih saat pindah halaman); `quota-layout.test.ts` kini 5 kasus (+ wilayah scroll
  terstruktur, + paginasi parkir di satu halaman). Merah terbukti sebelum implementasi: 8 merah /
  17 lulus pada pohon pra-implementasi, dan daftar merahnya persis enam test perilaku baru plus
  dua test layout.

## 7. Status pass ini

### 7.1 Gerbang

| Gerbang                               | Hasil                                                                                                                                                              |
| ------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| app-ui prettier (`bun run lint`)      | PASS, setelah `--write` atas lima berkas baru/ubah (selalu gerbang penuh, bukan per berkas)                                                                        |
| app-ui eslint (`bun run lint:ts`)     | PASS; nol `Set`/`Map` baru karena aturan itu diterapkan sejak penulisan (pelajaran pass 004)                                                                       |
| app-ui svelte-check (`bun run check`) | PASS 0 error / 0 warning                                                                                                                                           |
| app-ui build (`bun run build`)        | PASS, exit 0 (jalan pertama gagal karena /tmp tmpfs penuh 100 persen, bukan kode; lihat catatan lingkungan di §7.3)                                                |
| app-ui suite penuh (`bun run test`)   | PASS **2600 test / 160 berkas, 1479.05 dtk, 0 gagal**, sendirian pada pohon beku (sidik jari `7b9c084abefb690a82407c6c93f9cd56`, diverifikasi ulang sebelum jalan) |

**Pembekuan ulang (split berkas):** setelah tabel di atas terekam, QuotaCards.svelte terukur 301
baris, di atas batas 250, dan dipecah (lihat §6). Pohon membeku ulang pada sidik jari
`f0552dcdf94a522ee0c63bcfcf6007f6` (perintah: `find src tests -type f \( -name "*.ts" -o -name
"*.tsx" -o -name "*.svelte" \) | sort | xargs md5sum | md5sum` dari `app-ui/`), dan seluruh gerbang
dijalankan ulang padanya: prettier PASS (satu `--write` pada QuotaCardBody), eslint PASS (menangkap
satu import tak terpakai di child (dibersihkan, itulah fungsi gerbang), svelte-check 0/0, build
PASS, test terarah quota 25/25. Suite penuh dan click-through diulang pada pohon ini; hasilnya di
§7.2 dan §7.3.

### 7.2 Click-through hidup (R-35)

Panel salinan pohon AFTER di `:3001` (dibangun ulang dari pohon beku §7.1) ke gateway owner `:9090`;
driver CDP `/tmp/pass36/driver.ts`, hasil `after.json`, tangkapan `shots/after-desktop.png` dan
`shots/after-390.png` (pemeriksaan visual dua PNG itu milik owner; pengukuran di bawah numerik dari
DOM).

- **A, toolbar rumah tetap satu basaat**: `oneRow: true`.
- **B, kartu per provider**: dua kartu `section[aria-label]`; kartu provider membawa hitungan
  **"3 endpoints, 12 windows"** dengan baris Key 1/2/3, kartu "No provider" membawa
  **"1 endpoint, 4 windows"** dengan baris `virtual:opencode`; keduanya membawa ceklis, tombol
  lipat `aria-expanded="true"`, dan isi `max-h-80 overflow-y-auto` (kelas terbaca di DOM). Dua belas
  window dalam satu kartu berarti wilayah menggulir benar-benar bekerja pada data hidup, bukan
  hanya pada data uji.
- **C, lipat satu kartu**: klik tombol "Fold <provider>" → `aria-expanded` false dan baris kartu itu
  hilang dari DOM (4 baris menjadi 1); klik lagi → true dan 4 baris kembali.
- **D, lipat massal**: dua ceklis dicentang → baris aksi tampil dengan "2 selected", "Fold
  selected", "Unfold selected"; "Fold selected" melipat semuanya (`aria-expanded` false, 0 baris
  terlihat); "Unfold selected" membuka kedua kartu (4 baris kembali); seleksi dibersihkan lewat
  ceklis dan baris aksi hilang.
- **E, paginasi parkir pada data hidup**: "Page 1 of 1" dengan Previous dan Next nonaktif; dua
  kelompok provider hari ini belum mencapai 5 kartu, jadi perpindahan halaman terkunci oleh test
  jsdom (7 provider), bukan oleh data hidup.
- **F, 390 px**: `scrollWidth` 390 = viewport 390, paksaan `scrollTo(400,0)` meninggalkan `scrollX`
  0, 0 elemen keluar viewport.
- **G, konsol**: `windowErrors: "none"`.

**Re-run pasca-split:** seluruh A-G diulang pada panel yang dibangun ulang dari pohon pasca-split
(`QuotaCards.svelte` 221 + `QuotaCardBody.svelte` 100, log rebuild `/tmp/pass36/panel-rebuild.log`,
boot `:3001` dengan env sama) dan `after.json` hasilnya **byte-identik** dengan jalan pra-split
(`diff` kosong; bukti pra-split diarsipkan sebagai `after-presplit.json`). Bukti di atas berlaku
untuk pohon final.

### 7.3 Suite penuh

**2600 test / 160 berkas, seluruhnya lulus**, exit 0, 1435.84 dtk, tanpa beban bersama, pada pohon
final pasca-split `f0552dcdf94a522ee0c63bcfcf6007f6` (perintah sidik jari di §7.1; log penuh
tersimpan di `/tmp/pass36/suite-postsplit.log`). Aritmetika terhadap baseline pass 004 (2592/160):
quota.test.ts 14 menjadi 20 (+6), quota-layout.test.ts 3 menjadi 5 (+2), total +8 = 2600; jumlah
berkas tetap 160 karena split QuotaCardBody tidak menambah berkas uji. Jalan pertama pasca-split
(1704.12 dtk) sebenarnya lulus juga, tetapi ringkasannya terpotong pipa `tail` sehingga jumlahnya
tak bisa dikutip; diulang dengan log penuh dan jalan inilah rekam otoritatifnya (pelajaran yang
sama dengan sidik jari: gerbang direkam utuh, bukan lewat jendela terakhir.

**Catatan lingkungan (jujur):** /tmp tmpfs 3,9G mencapai 100 persen di tengah pass dan menghasilkan
tiga gejala yang menyesatkan: log kosong dengan exit 1, crash pekerja vitest yang BERPINDAH antar
berkas (level-berkas gagal sementara semua test di berkasnya lulus, tetap terjadi meski
`--maxWorkers=1` dan `--pool=forks`), dan build exit 1 dengan log 0 byte. Setelah pembersihan
berbasis umur atas perintah owner (795 entri pra-hari-ini, /tmp turun ke 12 persen) dan pemasangan
cron harian 07:30 yang melindungi `go-build*` milik proses hidup, seluruh gerbang diulang dan hijau
pada pohon yang sama; suite ini adalah satu-satunya rekam otoritatif.

### 7.4 Residu

- Paginasi server-side atas `/api/v1/quotas` bila skala window menuntut (filing terpisah, D5).
- D6 pass 004 (endpoint→keluarga fetcher) tetap menunggu keputusan owner; pass ini tidak menyentuhnya.
