# 022-USAGE-SCROLL-AND-DRAWING-TEXT.md: Halaman `/usage` bisa digeser ke halaman kosong, dan teks statis di gambar node

Dokumen kerja pass `app-ui` untuk halaman `/usage`. Pass ini mengerjakan dua item yang owner
laporkan sendiri. Bukan kontrak; kontrak tetap `docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel) dan
`docs/SPEC-API/001-SPEC-API.md` (wire).

|                      |                                                                                                                                                                        |
| -------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**           | CLOSED 2026-09-23. F1 dan F2 selesai; commit lokal `app-ui` + catatan draft 022 ini, baris register draft 007, dan amandemen §6.5, belum di-push                       |
| **Permintaan owner** | "1. Page `/usage`, perbaiki scrolling page karena bagian terbawah memiliki `blank page` yang merusak UI usage itu sendiri. 2. Pada Node Animation, tidak perlu lagi ada teks statis ini: 6 providers are configured: Edge TTS, Google TTS, Local Device, MiMo Code Free, OpenCode Free, SearXNG. No request is in flight. No request has finished since this screen opened." (2026-09-23) |
| **Scope**            | `app-ui/.` saja. `app-serv/.` tidak disentuh pass ini                                                                                                                  |
| **Kaitan**           | SPEC-UI §6.5 (Overview `/usage`, kalimat "states the same facts in words" diamandemen di pass ini), §8.6.1 (label koneksi live); draft 012 F3 (gambar live), draft 019 F6 (pembungkus tabel yang sama), draft 007 F12/F13; `AppShell.svelte`, `UsageTopology.svelte` |
| **Tanggal**          | 2026-09-23                                                                                                                                                             |

## 1. Ringkasan

Dua item, satu halaman. Yang pertama adalah cacat yang tidak terlihat dari kode mana pun: halaman
`/usage` bisa digeser ke area kosong di bawah isinya. Yang kedua adalah koreksi owner atas teks yang
tidak perlu ada di layar.

Item pertama diukur lebih dulu, bukan diasumsikan. Halaman yang sama dibuka di browser sungguhan
pada dua keadaan data, dan yang diukur adalah tinggi area geser dokumen
(`document.documentElement.scrollHeight - clientHeight`), bukan tinggi isi:

| Keadaan `/usage` (1360x1000)          | Sebelum perbaikan | Sesudah perbaikan |
| ------------------------------------- | ----------------- | ----------------- |
| Overview dengan 30 baris usage tersemai | **591 px** area kosong | **0**      |
| Overview tanpa baris usage            | 0                 | 0                 |
| `/api-docs`                           | **8638 px** area kosong | **0**       |
| `/token-saver`                        | **822 px** area kosong | **0**        |
| `/providers`                          | 0                 | 0                 |
| `/endpoint-keys`                      | 0                 | 0                 |

Halaman yang panjang itu tetap punya satu penggeser, `main`, dan `main` tetap bisa digeser
(1349 px pada `/usage` dengan data, 9048 px pada `/api-docs`). Yang ditambahkan 591 px dan 8638 px itu
adalah area kedua di luar penggeser itu: seluruh dokumen, sehingga roda mouse atau tombol Page Down
menggeser seluruh shell ke ruang kosong di bawah kolom isi, sementara sidebar tetap di tempatnya.
Itulah "blank page" yang owner lihat.

## 2. F1: penyebabnya satu sifat CSS, bukan satu tabel

**Fakta.** Penggeser `main` di `AppShell.svelte` tidak punya `position`, jadi ia bukan containing
block. Setiap label `sr-only` di dalamnya (Tailwind: `position: absolute`, `width/height: 1px`)
berposisi terhadap halaman, bukan terhadap tabelnya, dan containing block-nya adalah leluhur dari
penggeser itu. Akibatnya label itu **tidak terpotong oleh `main`**, melainkan menambah posisi
statisnya sendiri ke area geser dokumen. Diukur pada `/usage` dengan data, dua label keluar:

| Label (teksnya)                | Pemiliknya                                                     | `bottomAbs` |
| ------------------------------ | -------------------------------------------------------------- | ----------- |
| `Usage broken down by model`   | tabel breakdown `UsageGroupTable.svelte`                       | 1504        |
| `By provider by provider`      | tabel angka `UsageBarChart.svelte`, di dalam `<details>` tertutup | 1474     |

Baris kedua itu yang membuat mekanismenya perlu dicatat: `<details>` yang tertutup tetap
me-layout isinya di Chrome 150 (`content-visibility`, bukan `display: none`), jadi caption-nya ikut
keluar walaupun tabelnya tidak terlihat. `documentElement.scrollHeight` yang terukur, 1504, sama dengan
dasar label terjauh itu (1503 ditambah tinggi satu pikselnya).

Survei lima belas halaman panel (setiap route halaman kecuali `/login`, yang tidak punya tabel) diukur
dengan satu driver yang sama, dan yang dihitung adalah elemen `position: absolute/fixed` yang berposisi
terhadap halaman **dan** bukan `sr-only`: nol di setiap halaman. Jadi perbaikan di lapisan shell tidak
memindahkan UI apa pun; yang berubah hanya label yang memang tidak terlihat. Sidebar shell tetap
`position: fixed` (terhadap viewport, bukan terhadap penggeser), dan dialog yang tertutup tetap di luar
hitungan. `/api-base` bukan halaman melainkan route `+server.ts`, jadi ia tidak dihitung sebagai halaman.

**Perbaikan.** Satu kata di `AppShell.svelte`: penggeser itu menjadi containing block
(`relative`), sehingga setiap label `sr-only` di dalamnya diposisikan terhadap penggeser dan
terpotong oleh benda yang memang menggulir. Lima pembungkus `overflow-x: auto` yang sudah `relative`
dari draft 019 F6 dibiarkan apa adanya: keduanya benar, dan yang lama tetap yang paling ketat
(label tertahan di tabelnya sendiri). Satu pengecualian di draft 019 (pembungkus `EndpointTable`
sengaja dibiarkan statis) jadi tidak lagi relevan, bukan karena diukur salah, tetapi karena
lapisan shell sekarang menahannya.

**Batasan yang dicatat.** Angka 591 px itu keadaan dengan data; keadaan kosong halaman ini sudah 0
sebelum perbaikan, karena tidak ada tabel yang jatuh di bawah lipatan. Jadi cacat ini bergantung
data, dan halaman yang pendek tidak pernah menunjukkannya.

## 3. F2: teks statis di gambar node

**Fakta.** Gambar node (`UsageTopology.svelte`) menyatakan tiga kalimat di bawah gambarnya:
daftar provider yang terkonfigurasi, "No request is in flight.", dan "No request has finished since
this screen opened.". Owner meminta ketiganya tidak ada lagi. Ketiganya memang kalimat yang tidak
pernah berubah selama layar diam: daftar provider adalah label node yang sudah tampil di gambar,
dan dua kalimat lain hanya menyatakan ketiadaan.

**Perbaikan.** Blok kalimat itu sekarang hanya memuat fakta yang sedang terjadi, dalam satu baris,
dan tidak dirender sama sekali saat tidak ada yang bisa dinyatakan: `N in flight: ...` dari frame
yang membawa permintaan berjalan, `The last request to finish went to ...`, dan
`The gateway last reported an error on ...`. Kalimat "states the same facts in words" di SPEC-UI
§6.5 diamandemen sejalan: yang dinyatakan dalam kata adalah fakta yang sedang terjadi, karena label
node adalah daftar providernya sendiri dan caption gambar sudah mendefinisikan tampilan node diam.

Diukur di browser, halaman Overview tanpa data turun tinggi isinya dari 1044 px menjadi 988 px
(blok dua baris itu beserta jaraknya). Dua test file ikut berubah: `usage-topology.test.ts` dari 14
baris menjadi 12 (tiga baris kalimat statis diganti satu baris yang menguji ketiadaannya), dan
`usage-live-drawing.test.ts` dua baris berpindah ke label node. Dijalankan:
`bun run test tests/components/usage-topology.test.ts tests/components/usage-live-drawing.test.ts`
mencetak 19 test lulus (12 + 7) dalam 62,02 s.

## 4. Verifikasi di browser

Driver yang sama dipakai untuk semua angka di §1 dan §3: login lewat form sungguhan, buka `/usage`,
ukur geometri, gulir penggeser ke dasar, ukur jarak di bawah isi, lalu potret layar. Panel yang
diukur adalah instance pass ini sendiri, bukan panel owner:

- **Sesudah**: salinan `app-ui/` di `/tmp/usageprobe/panel` dibangun dengan `bun run build` (exit 0)
  dan dijalankan di port 3001 dengan `PANEL_API_TARGET=http://127.0.0.1:9091`.
- **Sebelum**: `app-ui/build/` apa adanya di port 3002, dibangun dari tree sebelum pass ini.
- Gateway-nya build terpisah dari HEAD di `127.0.0.1:9091`, karena proses `app-serv` milik tim di
  9090 datang dan pergi sepanjang sesi.

Yang terekam: angka di §1 datang dari dua survei halaman (lima belas halaman sebelum perbaikan, lima
halaman sesudah) dan dari pengukuran per halaman pada dua build panel, lalu diukur ulang pada build yang
dikirim di 1360 px dan 390 px. Tiga potret layar tersimpan: keadaan kosong sebelum perbaikan
(`/tmp/usageprobe/overview-1360x1000-bottom.png`) dan keadaan dengan data sesudah perbaikan pada 1360 px
dan 390 px (`/tmp/usageprobe/overview-1360x1000-live-bottom.png` dan
`/tmp/usageprobe/overview-390x844-live-bottom.png`). Jarak di bawah isi sesudah perbaikan 16 px di kedua
keadaan, yaitu padding `p-4` milik shell dan bukan ruang kosong.

## 5. Batas yang tidak diklaim

- Kalimat fakta live (`in flight`, `last`, `error`) tidak dipotret di browser pada pass ini, karena
  keadaan diam yang dipotret. Cabang itu ditutup test, bukan click-through.
- jsdom tidak bisa mengukur cacat ini (tidak ada layout, tidak ada `position` yang mengalir), jadi
  tidak ada test regresi untuk F1; buktinya adalah pengukuran browser di §1, dan komentar di
  `AppShell.svelte` menyebut angkanya supaya perbaikan itu tidak dicabut tanpa alasan.
- Angka 8638 px di `/api-docs` diukur pada halaman yang dimuat penuh, bukan pada setiap tab atau
  filter di dalamnya.
- Dua percobaan pengukuran pertama pass ini menggantung tanpa keluaran, dan penyebabnya `/tmp` penuh
  oleh profil Chrome milik probe itu sendiri; dicatat supaya pembaca berikutnya tidak menyalahkan
  driver-nya.

## 6. Gerbang

Suite penuh pertama pada tree `9dc440f3f781460ac0a553c82ca55a6f` menemukan satu asersi basi:
`tests/components/usage-overview-live.test.ts` adalah konsumen ketiga dari gambar itu dan masih menuntut
kalimat `One provider is configured: OpenAI.` yang dihapus F2. Run terarah pass ini hanya mencakup dua
berkas yang ditulis perubahannya, jadi berkas ketiga itu baru terlihat di suite penuh. Asersinya sekarang
menyinkron pada label node yang masih ada (`OpenAI`), dan seluruh gerbang diulang pada tree setelah
perbaikan itu (`616c6b0002592623f8c1fe7431dffc2c`). Editnya satu baris di berkas test: tidak ada berkas
runtime yang berubah, jadi pengukuran browser di §4 tetap menggambarkan kode yang sama.

| Gerbang            | Hasil                                                                                                    |
| ------------------ | -------------------------------------------------------------------------------------------------------- |
| `bun run check`    | 0 errors, 0 warnings                                                                                     |
| `bun run test`     | 2604 test lulus di 154 berkas dalam 952,73 s pada tree `616c6b0002592623f8c1fe7431dffc2c` (`md5sum` atas `src/`, `tests/`, `static/`, `scripts/`; 497 berkas), hijau pada percobaan pertama di tree ini. Run sebelumnya di tree pra-perbaikan `9dc440f3f781460ac0a553c82ca55a6f` tidak hijau: ia gagal pada satu asersi basi di `usage-overview-live.test.ts` yang lalu diperbaiki (catatan di atas tabel) |
| `bun run lint`     | Prettier melaporkan semua file sesuai                                                                            |
| `bun run lint:ts`  | ESLint keluar 0                                                                                          |
| `bun run build`    | sukses, exit 0, keluaran di `build/`                                                                     |
| Test terarah       | tiga berkas test yang disentuh, 22 test lulus dalam 68,76 s                                              |
| Ukuran file        | `AppShell.svelte` 55, `UsageTopology.svelte` 109, `UsageLivePanel.svelte` 163, `usage-topology.test.ts` 168, `usage-live-drawing.test.ts` 152, `usage-overview-live.test.ts` 177; keenamnya di bawah ambang 220 |
| Kebersihan teks    | 0 em dash di file yang disentuh pass ini                                                                 |

## 7. Yang tidak dikerjakan

- `app-serv/` tidak disentuh. Tidak ada route baru yang dibutuhkan kedua item ini.
- Tidak ada test regresi jsdom untuk F1 (lihat §5), dan tidak ada perubahan pada lima pembungkus
  `relative` dari draft 019 F6.
