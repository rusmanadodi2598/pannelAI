# 023-USAGE-DRAWING-TEXT-AND-FINISHED-SEPARATOR.md: Caption gambar node dihapus, fakta live pindah ke atas, dan daftar finished dipisah dengan idiom tab

Dokumen kerja pass `app-ui` untuk halaman `/usage`. Pass ini mengerjakan revisi tiga item yang owner
laporkan sendiri. Bukan kontrak; kontrak tetap `docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel) dan
`docs/SPEC-API/001-SPEC-API.md` (wire).

|                      |                                                                                                                                                                                                                                                                                    |
| -------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**           | CLOSED 2026-09-23. F1 dan F2 selesai; commit lokal `7790ed2` (kode `app-ui`, test, README, amandemen §6.5, tiga baris log alasan `DESIGN.md` §11, baris register draft 007, dan draft 023 ini) plus satu commit catatan status sesudahnya, keduanya belum di-push |
| **Permintaan owner** | "REVISI A. `app-ui/.` 1. Teks: `1 in flight: TH HARBOR 1 (deepseek-v4.1-flash:free).` - Ini tidak perlu dan clean up saja. 2. Teks: `Finished requests` - Seharusnya berada di TAB baru sesuai REFERENCE UI dari: decula (9router). 3. Teks: `Every configured provider around the gateway. Green is routing now, amber finished last, red is where the gateway last reported an error. A routing provider's line carries a beam with dots running along it, its node a soft glow, and the gateway pulses while it counts the requests in flight.`" (2026-09-23; instruksi item 3 tidak ikut terkirim, ditanyakan dan dijawab di §2) |
| **Scope**            | `app-ui/.` saja. `app-serv/.` tidak disentuh pass ini                                                                                                                                                                                                                               |
| **Kaitan**           | SPEC-UI §6.5 (Overview `/usage`, kalimat caption dan "states the same facts in words" diamandemen di pass ini); DESIGN.md §6 (motif) dan §11 (tiga baris log alasan baru); draft 012 F3, draft 013 F1/F2, draft 016 F3, draft 022 F2; draft 007 F12/F13; `UsageTopology.svelte`, `UsageRecentList.svelte` |
| **Tanggal**          | 2026-09-23                                                                                                                                                                                                                                                                         |

## 1. Ringkasan

Tiga item, satu blok layar. Ketiganya tentang teks di sekitar gambar node: satu kalimat fakta live yang
tidak perlu, satu daftar yang harusnya dipisah, dan satu paragraf penjelasan yang harusnya bersih.

Yang diukur lebih dulu adalah kedua tree, bukan hanya milik panel ini, karena item 2 menyebut reference:

| Yang diukur (9router `origin/master` @ `21583c03`)                                        | Hasil                                                                                        |
| ----------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| Tab yang terlihat di `/usage` reference                                                    | Dua: Overview dan Details (`page.js:30-37`)                                                   |
| Cabang tab ketiga                                                                          | `?tab=logs`, URL-only, tidak ada di kontrol (`page.js:34-36,74`)                              |
| Tempat kartu "Recent Requests"                                                             | Di Overview, bersebelahan dengan topology dalam grid `2fr/1fr` (`UsageStats.js:474-483`)       |
| Isi cabang `logs`                                                                          | Tabel log dari DB dengan auto-refresh 3 detik, judul "Request Logs" (`RequestLogger.js:16-46`) |
| Teks legend di sekitar topology reference                                                  | Tidak ada: hanya label node dan label "Gateway"                                               |

Jadi premis "reference menaruh daftar request di tab" hanya berlaku untuk cabang URL `logs`; kartu
"Recent Requests"-nya sendiri duduk di Overview. Owner menjawab pengukuran itu dengan bentuk yang
berbeda lagi: daftarnya tetap di Overview, dipisah dengan idiom tab **tanpa** kontrol yang bisa diklik
(§2), dan seluruh teks penjelasan di gambar dibersihkan (§3).

## 2. Ronde pertanyaan (satu, tiga pertanyaan, dijawab penuh)

Instruksi item 3 tidak ikut terkirim bersama teksnya, jadi ronde ini juga menanyakannya, plus dua bentuk
yang jawabannya menentukan bentuk permukaan:

| Pertanyaan                                            | Jawaban owner                                                                                                                        |
| ----------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| Mode antislop                                         | DURING & AFTER                                                                                                                        |
| Item 1: fakta mana yang tetap diucapkan?              | "Pindahkan aja ke paling atas, menggantikan teks yang panjang (Every configured..), namun teks nya di buat lebih rapih untuk supaya UI nya mengikuti animation yang ada. Intinya match dengan Node Animation." |
| Item 2: bentuk tab di bawah Node Animation?           | "Tetap di page Usage >> Overview \| Namun, ada 1 tab terpisah di bawah Node Animation"                                                 |
| Item 3: caption diapakan?                             | "Semua teks yang di Node (Every configured...) ini clean up semua. Hanya `Node Animation dan teks tiap providernya` saja di tab tersebut." |

Satu pertanyaan lanjutan dipakai untuk mengunci dua hal yang jawaban itu buka: fakta mana yang tinggal
di baris atas (dijawab **ketiga fakta, satu baris**) dan bentuk pemisahnya (dijawab **tidak perlu tab
untuk diklik, hanya pemisahannya pakai tab, isinya info statis seperti reference**).

## 3. F1: fakta live pindah ke atas, caption dihapus

**Fakta.** `UsageTopology.svelte` merender tiga kalimat di bawah gambar (in-flight, finished terakhir,
error terakhir) dan satu `figcaption` di atasnya yang menjelaskan warna dan gerak. Owner meminta caption
hilang seluruhnya dan kalimat in-flight naik ke atas.

**Perbaikan.** `figcaption` dihapus. Tiga fakta menjadi satu array `Fact` (`label`, `value`, `tone`) yang
dirender sebagai satu baris **di atas** gambar, dengan label yang lebih pendek: `N in flight:`,
`Last finished:`, `Last error:`. Baris itu memakai aturan warna gambar itu sendiri, yang sekaligus
menggantikan definisi warna yang hilang bersama caption: label node yang routing memakai warna status,
jadi nama provider di fakta in-flight memakai `--color-ok`, sementara fakta finished dan error tidak
mewarnai namanya karena node `last`/`error` di gambar juga tidak. Slot baris itu tetap ada saat tidak ada
fakta (`min-h-5`, setinggi satu baris `text-sm`), supaya gambar tidak bergerak di depan mata operator saat
sebuah request mulai.

Tiga baris test menyesuaikan: `usage-topology.test.ts` menguji ketiadaan fakta saat diam pada label
barunya, satu baris baru menguji urutan (baris fakta mendahului gambar), dan baris "explains the colours
in words" diganti baris yang menguji aturan warna itu; `usage-topology-motion.test.ts` kehilangan baris
yang menguji kalimat gerak di caption; `usage-live-drawing.test.ts` dan `usage-overview-live.test.ts`
menyinkron pada nilai fakta, bukan pada kalimat lama.

## 4. F2: daftar finished dipisah dengan idiom tab, tanpa kontrol

**Fakta.** `UsageRecentList.svelte` membuka dengan `<h3>Finished requests</h3>`, jadi daftar itu hanya
judul di bawah gambar. Owner meminta pemisahnya memakai tab, dan menjawab pertanyaan lanjutan bahwa tab
itu **tidak** untuk diklik.

**Perbaikan.** Judulnya diganti pemisah bergaya tab: pembungkus `border-b` setinggi label dan label yang
membawa rule aksen `border-b-2` seperti tab aktif di `PanelTabs.svelte`, minus semua kelas interaktifnya
(tidak ada `button`, tidak ada `role="tab"`, tidak ada `min-h-11`, tidak ada fokus). Daftarnya sendiri
tidak berubah: barisnya masih membawa waktu relatif, model, token in/out, provider, dan status.

## 5. Verifikasi

Lima run browser pada instance pass ini (login lewat form sungguhan), dengan potret layar dan berkas bukti
JSON di `/tmp/usage023/`:

| Run | Instance | Yang dibaca |
| --- | -------- | ----------- |
| 1360 px terang | panel port 3001, `PANEL_API_TARGET=http://127.0.0.1:4100` (contract double di depan gateway tim di `:9090`) | `figcaption` count 0; baris fakta di atas gambar; nama provider yang routing memakai `rgb(20, 116, 58)` sementara fakta finished dan error memakai warna teks bawaan `rgb(26, 22, 20)`; pemisah `Finished requests` bertag `span`, tanpa `role`, 0 elemen bisa difokus, 0 `role=tab` di section; scroll dokumen 0 |
| 1360 px gelap | sama, tema diganti lewat tombol panel | `classList.contains('dark')` benar; nama yang routing `rgb(92, 201, 123)`, fakta lain `rgb(237, 234, 230)`; angka lain sama |
| 390 px | sama, `Emulation.setDeviceMetricsOverride {mobile: false}` | `innerWidth` mencetak 390; `scrollWidth - clientWidth` 0 dan `scrollTo(400, 0)` membaca `scrollX` 0; baris fakta membaca 60 px (tiga baris) terhadap slot diam 20 px |
| stream diam | sama, double dialihkan ke mode diam lewat `/__control/mode?idle` | pada satu halaman yang sama: slot diam 20 px dengan `drawingTop` 1737, lalu satu fakta masuk dan barisnya tetap 20 px dengan `drawingTop` 1737, jadi gambar tidak bergerak di 1360 px |
| route nyata | panel port 3003, `PANEL_API_TARGET=http://127.0.0.1:9090` (tanpa double) | route `usage/live` milik gateway sendiri: chip `Live`, tidak ada fakta untuk dinyatakan (`factsText` kosong, slot 20 px, `drawingTop` 1737), dan daftar finished tidak dirender karena stream tidak membawa baris `recent` |

Batas yang dicatat: warna `ok` pada nama provider yang sedang routing dibuktikan pada frame **contract
double**, bukan pada gateway, karena route gateway yang asli tidak melaporkan permintaan yang sedang berjalan
selama run; yang dibuktikan pada route nyata adalah slot kosong dan gambar yang tidak bergerak. Pada 390 px
baris fakta membaca 60 px sementara slot diamnya 20 px, jadi klaim "slotnya tetap" hanya berlaku di lebar
tempat fakta muat satu baris, dan §6.5 menuliskan angka itu apa adanya.

Catatan harness, bukan kode panel: double sempat meneruskan header `transfer-encoding` dari panel ke gateway,
sehingga setiap penulisan lewat panel menjawab `400 invalid request body: i/o timeout` sementara request yang
sama langsung ke double menjawab 204. Double sekarang membuang header hop-by-hop itu; tidak ada file panel
yang diubah karenanya.

## 6. Gerbang

| Gerbang            | Hasil                                                                                                    |
| ------------------ | -------------------------------------------------------------------------------------------------------- |
| `bun run check`    | 0 errors, 0 warnings                                                                                     |
| `bun run test`     | 2605 tes lulus di 154 file (pass terakhir 2604 di 154, jadi +1: satu baris baru di `usage-topology.test.ts` dan satu di `usage-live-drawing.test.ts`, dikurangi satu di `usage-topology-motion.test.ts` yang kehilangan baris caption, sementara `usage-overview-live.test.ts` tetap 3). Durasi 1202 s pada tree berfingerprint `f752bd0293c3d1d880d664c2c0bbdb44` (497 file), hijau pada percobaan pertama |
| `bun run lint`     | Prettier melaporkan semua file sesuai (dijalankan terakhir, setelah baris suite di atas diisi)           |
| `bun run lint:ts`  | ESLint keluar 0                                                                                          |
| `bun run build`    | sukses, exit 0, keluaran di `build/`                                                                     |
| Test terarah       | 4 file (`usage-topology`, `usage-topology-motion`, `usage-live-drawing`, `usage-overview-live`) lulus 32 tes dalam 96.33 s, dijalankan pada tree beku yang sama dengan suite di atas |
| Ukuran file        | Keenam file yang disentuh pass ini di bawah ambang 220 (diukur sesudah Prettier): `UsageTopology.svelte` 123, `UsageRecentList.svelte` 106, `usage-topology.test.ts` 193, `usage-topology-motion.test.ts` 119, `usage-live-drawing.test.ts` 182, `usage-overview-live.test.ts` 177 |
| Kebersihan teks    | 0 em dash di file yang disentuh pass ini                                                                 |

## 7. Yang tidak dikerjakan

- `app-serv/` tidak disentuh. Tidak ada route baru yang dibutuhkan ketiga item ini.
- Daftar finished tidak pindah ke tab tingkat halaman: owner memutuskan tetap di Overview, dipisah di
  bawah gambar.
- Tidak ada perubahan pada gambar itu sendiri (node, beam, gateway, ukurannya): yang berubah hanya teks
  di sekitarnya.
