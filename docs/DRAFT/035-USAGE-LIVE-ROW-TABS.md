# 035-USAGE-LIVE-ROW-TABS.md: Baris kontrol `/usage` berukur sama, teks dinamis keluar dari animasi node

Dokumen kerja pass `app-ui` untuk bagian **Live routing** di halaman `/usage`. Pass ini mengerjakan dua
permintaan owner yang disebut langsung: tab-tab pada baris kontrol Live Routing (chip status, Pause,
Try again) diperbaiki supaya ukurannya sama dan simetris, dan teks dinamis "in-flight" serta "finish"
dipindahkan dari frame animasi node ke satu tab terpisah yang sejajar dengan tab-tab baris itu. Bukan
kontrak; kontrak tetap `docs/SPEC-API/001-SPEC-API.md` (wire) dan `docs/SPEC-UI/001-SPEC-UI.md` (perilaku
panel). Pola mengikuti `003-PORT-COMBO-VISION.md`: temuan bernomor F, bukti yang bisa diulang, keputusan
di depan implementasi, dan gerbang antislop di akhir.

|                      |                                                                                                                                                                                                                                                                                                                                                       |
| -------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**           | **DURING selesai, AFTER selesai 2026-09-27; gerbang final pada build akhir + commit lokal menyusul.** F1 dan F2 terimplementasi; audit AFTER mengembalikan 32 temuan, 30 diterima dan sudah dikerjakan, 2 berhenti sebagai keputusan (§6, §7.5). §7.1 direkam pada markup final (87 test lulus, dua mutasi merah-hijau); §7.2 direkam pada build sebelum koreksi AFTER sehingga wajib diulang pada build final. Penyebab tertundanya: shell sesi kerja mati di tengah pass (efek `pkill` pass ini terhadap runtime CLI-nya sendiri), bukan cacat kode |
| **Mechanism**        | DURING & AFTER (antislop)                                                                                                                                                                                                                                                                                                                             |
| **Scope**            | `app-ui/.` (bagian Live routing pada tab Overview `/usage`). `app-serv/.` tidak disentuh sumbernya; hanya dipakai read-only untuk bukti hidup §7.2, lewat instance kedua di port cadangan dengan database khusus `pannelai_d035` (§7.6)                                                                                                                 |
| **Permintaan owner** | (1) Baris Live Routing: chip Refresh/Unavailable, Pause, Try again diperbaiki supaya tab-nya sama ukuran dan simetris. (2) Teks dinamis in-flight dan finish yang berada di animasi node mengganggu UI node, pindah ke satu tab terpisah sejajar dengan tab-baris itu, disamakan ukurannya. (3) Commit lokal setelah progres `CLOSED` |
| **Reference**        | `decolua/9router`, checkout `/home/rusmanadodi/apps/9router`; bacaan §2.3                                                                                                                                                                                                                                                                             |
| **Kaitan**           | SPEC-UI §6.5 (diamandemen pass ini) dan §8.6 rule 3; DESIGN.md §2.1 dan §11 (empat baris log alasan pass ini); drafts 012 F2/F3, 013 F2, 022 F2, 023 F1/F2 (semua CLOSED, aturannya dipertahankan kecuali tempat kalimat fakta duduk); register draft 007 baris 4; `app-ui/README.md` entri ke-38                                                     |
| **Tanggal**          | 2026-09-27 (DURING & AFTER)                                                                                                                                                                                                                                                                                                                           |

## 1. Ringkasan

Dua permintaan owner diukur dulu sebelum satu baris diubah.

| Yang owner sebut                                 | Yang diukur                                                                                                                                                                                                                                                                                                                                                                                                              |
| ------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Tab baris belum sama dan simetris                | **Benar.** Chip status memakai `px-2 py-1 text-sm` (kotak teks biasa setinggi satu baris `text-sm`), sedangkan kedua tombolnya `min-h-11` (44px): satu baris dengan dua tinggi kotak, dan `items-center` membuat chip menggantung di tengah. `Try again` tetap kondisional terhadap state `unavailable`; yang tidak seragam adalah kotaknya, bukan jumlahnya                                                            |
| Teks in-flight/finish di animasi node mengganggu | **Benar.** Kalimat fakta (`2 in flight: ...`, `Last finished: ...`, `Last error: ...`) dirender `UsageTopology.svelte` **di dalam** `<figure>` gambar, tepat di atas node. Aturan slot draft 023 F1 hanya menjaganya setinggi satu baris selama muat satu baris; pada 390px kalimat terukur 60px melawan slot 20px, jadi setiap request mulai atau selesai menggeser gambar di bawah mata pembaca. Teks itu sendiri bukan kontrol: tidak ada yang bisa diklik di sana, hanya kalimat yang menempati ruang gambar |

Reference mengukuhkan arah pemisahan ini: `UsageStats.js` melancarkan field live hanya ke stats dan tabel
(`activeRequests`/`recentRequests`, :282-294), sementara `ProviderTopology` menerima prop itu sekadar untuk
mewarnai node dan memuat badge hitungan di kartu gateway (:478). Tidak ada satu pun kalimat "in flight"
di dalam kanvas reference; kanvas reference memang tidak pernah punya teks status.

## 2. Bukti yang bisa diulang (sebelum pass ini)

### 2.1 Baris kontrol (`UsageLivePanel.svelte`, sebelum)

- baris: `flex flex-wrap items-center gap-3` dengan tiga anak: `span[role=status]` (chip), `button` Pause (44px), `button` Try again (44px, kondisional)
- chip: `rounded-[var(--radius-sm)] border border-[var(--color-border)] px-2 py-1 text-sm`, tidak ikut `min-h-11`
- `grep -n "px-2 py-1" src/lib/components/UsageLivePanel.svelte` -> 1 hit (sebelum), 0 hit (sesudah; diverifikasi di berkas final)

### 2.2 Kalimat fakta (`UsageTopology.svelte`, sebelum)

- paragraf `<p class="min-h-5 text-sm">` dirender sebelum `{#if providers.length === 0}`, di dalam `<figure>` yang sama dengan gambar
- SPEC-UI §6.5 lama mencatat pengukurannya: pada 1360px slot dan kalimat sama-sama 20px, pada 390px kalimat 60px melawan slot 20px

### 2.3 Bentuk reference

| Kontrol                          | Reference                                                            | Baris                        |
| -------------------------------- | --------------------------------------------------------------------- | ---------------------------- |
| Teks live (in flight, finished)  | masuk sebagai baris tabel stats, bukan teks di kanvas                 | `UsageStats.js:282-294,:525` |
| Topologi                         | hanya menerima `activeRequests` untuk warna node dan badge gateway    | `UsageStats.js:478`          |
| Tidak ada legenda/kalimat canvas | `ProviderTopology.js` tidak merender kalimat status                   | (ditarik di draft 023 F1)    |

## 3. Keputusan pass ini

| #   | Pertanyaan                                                            | Arah yang dipakai                                                                                                                                                                                                                                       | Dipakai oleh |
| --- | ----------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| D1  | Kotak bersama tab baris                                                 | **Satu kotak untuk semua tab yang ter-render**: `inline-flex min-h-11 items-center rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm`. Dua tombolnya memang sudah begitu; chip dan tab fakta naik ke kotak itu, tidak sebaliknya. Alasannya: 44px adalah lantai target sentuh kontrol panel (SPEC-UI §8.6 rule 3) sekaligus kotak yang sudah dipakai kontrol baris ini sendiri. Bukan `DESIGN.md` §7: baris 36px/44px-nya scoped ke tabel dan sidebar, jadi tidak dikutip sebagai alasan (koreksi dari audit AFTER) | F1           |
| D2  | Apakah `Try again` dibuat selalu tampil supaya baris tidak bergeser?    | **Tidak.** R-26 melarang kontrol mati; render kondisional dipertahankan. Simetri yang diminta adalah ukuran dan bentuk. Pembeda kontrol dari pernyataan: hanya dua tombol memakai `hover:bg-[var(--color-surface-2)]` (kesepakatan audit AFTER baris 7), chip dan tab fakta tidak                                                                                          | F1           |
| D3  | Tab fakta: kontrol atau pernyataan?                                     | **Pernyataan.** `span` biasa tanpa `button`/`role=tab`/`tabindex`, idiom yang sama yang ditetapkan draft 023 F2 untuk label separator; letaknya sejajar chip dan Pause, kotaknya kotak D1                                                                                                                                | F2           |
| D4  | Bagaimana aturan "idle tidak menyatakan ketiadaan" (2026-09-23) bertahan? | **Tab fakta tidak dirender sama sekali saat tidak ada fakta**, dan test idle mengunci jumlah anak baris (2) sebagai premis positif kesunyiannya. Slot `min-h-5` dipensiunkan bersama paragrafnya                                                                                                                            | F2, D2       |
| D5  | Derivasi fakta pindah ke mana?                                          | **Ke `usage-live-view.ts` sebagai `liveFacts` + `providerDisplayName`** (murni, tanpa DOM/jam), mengikuti aturan berkas itu; `nameOf`/`inFlight` lokal di `UsageTopology` dihapus; pasangan panel+gambar berhenti menduplikasi lookup nama (provider-name map milik `usage-topology-view` tetap untuk node, dan panel mendelegasi)                                                            | F2           |
| D6  | Badge angka pada node gateway ikut dipindah?                            | **Tidak.** Itu bagian desain kartu gateway reference (badge menghitung request, `ProviderTopology.js:99-130`); teks dinamis yang owner sebut adalah kalimat "in flight"/"finished", bukan angka pada kartu                                                                                                                  | n/a          |
| D7  | Saat paused/unavailable, fakta masih berwarna status?                    | **Ya, dipertahankan; ini keputusan bukan cacat.** Bacaan audit AFTER (baris 3, R-27) menuntut warna `ok` turun saat stream berhenti; bacaan itu berhenti sebagai keputusan: draft 013 F2 sudah menetapkan "colour is state, motion is now" dan SPEC-UI §6.5 menuliskannya untuk gambar ("keeps the last known state in colour") sementara paragraf lama berperilaku sama; tab hanya memindahkan kalimat yang sama. Yang menjaga klaim "now" adalah chip dan baris status, dan keduanya teruji                    | §7.4         |

## 4. Temuan

### 4.1 F1 (MEDIUM): baris kontrol memakai dua ukuran kotak

Chip `px-2 py-1` duduk di antara dua tombol `min-h-11` dengan `items-center`; tiga tab, dua tinggi.

### 4.2 F2 (MEDIUM): kalimat fakta hidup di dalam frame animasi node

Paragraf fakta mendorong gambar ke bawah setiap kali berubah; pada 390px pergeserannya 60px lawan 20px
(slot hanya menahan sampai satu baris). Pemindahan yang owner minta: satu tab terpisah, sejajar,
disamakan kotaknya.

## 5. Rencana DURING

1. `usage-live-view.ts`: `liveFacts(providers, active, last, error)` + `providerDisplayName`, tipe `LiveFact` berton `status|plain`; model kosong dari wire tidak boleh mencetak kurung kosong (audit AFTER baris 27).
2. `UsageLivePanel.svelte`: konstanta kotak tab bersama (D1), baris `items-stretch`, tab fakta antara chip dan Pause hanya saat ada fakta, `min-w-0 flex-wrap` dengan `break-words` pada nilai agar nilai panjang tidak pernah melebarkan halaman (audit baris 1/31), spasi kalimat via `{' '}` agar terbaca utuh oleh AT (audit baris 30), `hover` khusus kontrol (D2); `providerName` mendelegasi ke `providerDisplayName`.
3. `UsageTopology.svelte`: paragraf fakta dan helper-nya dihapus; frame menyisakan gambar dan empty state; komentar `UsageTopologyDrawing.svelte` diperbarui (audit baris 11).
4. Test: `tests/schemas/usage-live-facts.test.ts` (baru, 10 kasus, table-driven per §2.5) mengunci derivasi; `tests/components/usage-live-row.test.ts` (baru, 10 kasus) mengunci kotak baris dengan tab fakta ikut di-loop, tab-not-a-control, idle tanpa tab plus premis jumlah, hover khusus kontrol, dan kalimat ber-spasi; `usage-topology.test.ts` menukar lima baris fakta dengan baris "frame tidak membawa kalimat" (berpremis positif) plus baris warna label node; **tiga await** yang menyinkron pada kalimat lama dipindah ke nilai tab (dua di `usage-live-drawing.test.ts`, satu di `usage-overview-live.test.ts`), masing-masing ditambah await `Gateway` sebagai premis gambar (audit baris 24); pointer split di header `usage-live-view.test.ts`.
5. SPEC-UI §6.5, DESIGN.md §11 (empat baris), register 007, README entri ke-38, diamandemen di commit yang sama.
6. Gerbang §7.

## 6. Implementasi

Semua item §5 dikerjakan. Set perubahan: 3 berkas `src/lib` (2 komponen + 1 schema), 6 berkas `tests`
(2 baru + 3 rereset + 1 pointer header), SPEC-UI §6.5, DESIGN.md §11 (4 baris), register draft 007,
`app-ui/README.md`. `app-serv` tidak disentuh sumbernya.

Audit AFTER dijalankan sebagai Workflow empat reviewer read-only terhadap diff (`usage-row-pass-review`,
32 temuan): 30 diterima dan seluruhnya sudah masuk ke set perubahan di atas (kotak aman 390px, spasi AT,
hover-only-control, table-driven §2.5, blank model, premis positif di tiga berkas test, koreksi kutipan
`DESIGN.md` §7 yang salah di komentar dan D1, serta angka-angka dokumen yang tidak bisa ditelusuri);
2 berhenti sebagai keputusan dan dicatat apa adanya: D7 (warna fakta saat paused) dan satu em dash di
`app-serv/internal/schema/chat_validation_test.go:41` yang berasal dari pass draft 034 yang sudah
ter-commit, di luar diff ini (residu §7.5).

Bentuk final: kalimat `Label: value.` kehilangan titik akhirnya karena tab dibaca label, bukan paragraf;
pemisah antar fakta `·` dengan `aria-hidden` di sekelilingnya spasi teks nyata (R-02 bersih: 0 em dash di
seluruh set, diverifikasi `grep -P \x{2014}`); warna mengikuti aturan draft 023 F1 apa adanya; glyph tidak
ditambahkan (bukan permukaan ikon).

## 7. Gerbang dan bukti

### 7.1 Suite `app-ui` (pada markup final)

| Gerbang | Hasil |
| ------- | ----- |
| Targeted 9 berkas usage/topology (`vitest run`) | **PASS: 9 berkas, 87 test lulus, exit 0**, pukul 14:29, pada markup yang sekarang terpasang (source diverifikasi ulang pasca §7.3: `Read` + `Grep` atas `UsageLivePanel.svelte:137-175` dan `usage-live-view.ts:169-189`) |
| Non-vacuity mutasi | dua mutasi, masing-masing tepat satu baris merah (§7.3) |
| `check` / `lint` / `lint:ts` / `build` / `test` penuh | Hijau sebelum koreksi AFTER (svelte-check 0 error 0 warning, prettier PASS, eslint 0 error, build exit 0). Run penuh pertama terkontaminasi `pkill` pass ini dan sengaja tidak dihitung sebagai angka. **Kelimanya wajib diulang pada build final** dan menimpa baris ini sebelum status CLOSED |

### 7.2 Click-through hidup (R-35), rig §7.6, rekaman 14:06-14:08 pada build 14:05

Panel build sendiri di `:3003` (`PANEL_API_TARGET` ke gateway instance `:9093`, DB khusus), frame SSE
nyata dari request yang benar-benar dirutekan gateway ke upstream mock lokal. Bukti:
`/tmp/d035/evidence.json` dan lima tangkapan layar `row1-idle..row5-unavailable`.

- **A1-A4** idle: baris ber-kelas `flex flex-wrap items-stretch gap-3`; hanya dua tab (chip, Pause), keduanya 44px satu baseline (top 1604); **nol tab fakta saat tidak ada yang terjadi** (A4 true)
- **B0-B4** error nyata: `b35/boom` dijawab gateway `502 UPSTREAM_ERROR`; tab fakta muncul satu parent dengan chip, 44px, selisih top < 2px; `figure` tidak membawa kalimat; teks tab `Last finished: d035 err· Last error: d035 err`
- **C1-C3** in-flight nyata: `s35/slow` tertahan 9 detik di upstream; tab terbaca `1 in flight: d035 slow (slow)` dengan nilai `--color-ok` (C2 true), finished/error tetap polos (C2b true); badge kartu gateway di dalam gambar menghitung request yang sedang berjalan (=1); screenshot saat frame masih hidup
- **D1-D3** keyboard: fokus Pause terlihat, Enter men-toggle ke `Paused`, Resume mengembalikan ke `Live`
- **E1** paused: tiga tab ter-render (chip, fakta, `Resume`) semuanya 44px
- **F0-F0b** unavailable nyata: gateway instance dimatikan; chip `Unavailable` dan `Try again` ter-render 44px segaris (empat tab, top 458); setelah restart, reconnect terjadwal mengembalikan ke `Live` (jalur "scheduled"; jalur manual `Try again` terukur pada versi run sebelumnya dan diuji di §7.1 test retry)
- **F1** 390px: `scrollWidth` 390 = viewport 390; **0 elemen berada di luar scroller**; 294 elemen yang lebih lebar dari viewport seluruhnya di dalam pembungkus scroll halaman (pola tabel panel, sama seperti bacaan I1 PORT 003); tiga tab baris tetap 44px
- **G1** dua tema: toggle ada; light `rgb(252,250,247)` menjadi dark `rgb(26,25,23)` dengan teks chip `rgb(237,234,230)`, pasangan yang sama dengan probe PORT 003
- **Z** konsol: 0 error pada seluruh run, kecuali dua milik jendela F0 saat gateway sengaja dimatikan (`ERR_INVALID_CHUNKED_ENCODING` + 500 dari proxy panel karena connection refused); dicatat apa adanya, bukan kegagalan panel

**Catatan kejujuran:** rekaman ini memakai build sebelum koreksi AFTER; yang berubah sesudahnya hanya
`min-w-0 flex-wrap` pada tab, `break-words` pada nilai, spasi AT, dan `hover` khusus kontrol, jadi angka
kotaknya tetap sah, namun §7.2 **wajib diulang pada build final** dan menimpa bagian ini di commit
penutup.

### 7.3 Non-vacuity (mutasi merah-hijau)

| Mutasi | Hasil |
| ------ | ----- |
| Chip dikembalikan ke `px-2 py-1` (bukan `TAB_BOX`) | tepat satu baris merah: `gives every tab it renders, facts tab included, the same box`; 9 lainnya hijau; dipulihkan |
| `liveFacts` kembali ke `entry.model === undefined` (blank mencetak `name ()`) | tepat satu baris merah: `draws no empty parenthesis for a frame whose model is blank`; 9 lainnya hijau; dipulihkan |

### 7.4 antislop Delivery Gate

Mode DURING & AFTER; baris N/A beralasan; semua PASS di bawah punya bukti di §7.1-§7.2 kecuali yang
disebut bersyarat.

**Block 1 Hard Gate**: R-02 PASS (0 em dash di 7 berkas src/test/docs pass ini). R-03 PASS bersyarat (F1 390px nol overflow; angka final build akhir menyusul §7.2). R-17 PASS (setiap angka punya perintah atau file bukti). R-18 N/A (panel operator). R-23 N/A (tidak ada aset/logo/statistik baru; tidak ada ikon baru). R-24 PASS (navigasi tidak disentuh). R-25 PASS (pasangan warna tab dikunci `tests/tokens/contrast.test.ts`; probe G1 §7.2). R-26 PASS (pass ini tidak menambah satu pun kontrol: tab fakta pernyataan, dua kontrol nyata tetap bertingkah dan terukur diklik hidup). R-27 PASS (idle/connecting/live/paused/unavailable terekam B..F0). R-28 N/A. R-32 PASS (D1-D3; pernyataan memang bukan target fokus). R-33 PASS (semua ditulis di sumber). R-34 PASS (G1). R-35 PASS bersyarat (§7.2 diulang pada build final sebelum CLOSED). R-36 PASS (aturan "Live hanya saat frame tiba" tidak diubah; D7 mencatat warna = state sesuai preseden). R-37 PASS (`DESIGN.md` dibaca; kutipan §7 yang salah dikoreksi, bukan ditutupi; tidak ada arah owner yang ditimpa diam-diam). R-38 PASS bersyarat (status dokumen ini masih DURING justru karena gerbang final belum diulang).

**Block 2 Purpose-Gate**: N/A kecuali alasan tertulis D1-D7 dan aturan warna warisan draft 023 F1 yang
dipertahankan apa adanya; tidak ada gradien, glow, ikon, font, atau motion baru.

**Block 3 Liveliness**: dial `DESIGN.md` §2.1 (ENERGY 1 / RHYTHM 2 / MOTION 1) tidak dilanggar; satu
fokus layar tidak berubah; motif baris (satu kotak; aksen hanya pada kontrol nyata) justru dikuatkan D2.

**Block 4 Craftsmanship & Quality Locks**: C-1 s/d C-5 PASS (alasan satu baris per keputusan; kontrol
nyata; tidak ada seksi baru; state/tema/breakpoint/keyboard terukur; angka dari pengukuran). R-05, R-11,
R-15, R-16, R-20, R-21, R-29, R-30, R-31 PASS (tanpa template baru; radius/palet/tema dari `DESIGN.md`;
setiap keputusan punya baris §3 dan empat barisnya di log alasan §11).

### 7.5 Residu

- **Gerbang final pada build akhir**: `bun run check/lint/lint:ts/build/test` penuh + ulang §7.2 setelah
  rebuild; setelah hijau semua, Status berbalik CLOSED dan commit lokal dibuat (permintaan owner butir 3).
  Penyebab tertunda: shell sesi kerja mati oleh `pkill` pass ini sendiri saat run penuh pertama berjalan;
  run itu tidak dihitung.
- **Em dash `app-serv/internal/schema/chat_validation_test.go:41`** warisan pass draft 034 yang sudah
  ter-commit; di luar diff ini; dilaporkan untuk keputusan owner, tidak disentuh diam-diam.
- **Perangkat §7.6 masih hidup** menunggu keputusan: gateway `:9093` (pid `/tmp/d035/gw.pid`, DB
  `pannelai_d035`), mock `:9094` (`mock.pid`), panel `:3003` (`panel.pid`, melayani build pass ini).
  Cleanup terjadual: matikan bertiga, `DROP DATABASE pannelai_d035`, hapus
  `app-ui/panel.log` (untracked) bila tidak dipakai lagi.

### 7.6 Resep perangkat bukti hidup (reproducible)

`/tmp/d035/`: `start_gw.sh` (binary build pohon ini, DB khusus `pannelai_d035`,
`EGRESS_ALLOWED_TARGETS=127.0.0.1/32,172.20.10.5/32`, port 9093), `mock.ts` (upstream lokal:
`*slow*` telat 9s, `*boom*` 500), node provider prefix `b35`/`s35` di DB khusus itu, gateway key
`sk.txt`, password bootstrap `password.txt`, driver `drive.py` (Playwright sinkron + Chrome sistem),
output `evidence.json`. Dua penemuan lingkungan selama pass, dicatat supaya tidak diulang sebagai bug
panel: instance gateway kedua mewarisi `settings.network` dari DB pemilik
(`outbound_proxy_enabled` true dengan URL kosong menolak seluruh upstream), maka perangkat pindah ke DB
khusus alih-alih mengubah pengaturan pemilik; dan guard egress default-menolak loopback bila tidak
di-allowlist. Postgres dan Redis pemilik hanya dipakai read-only; dua node, empat endpoint, dua key yang
sempat dibuat di DB pemilik dihapus kembali (enam DELETE 204) sebelum perangkat pindah ke DB khusus.

## 8. Status final

Belum CLOSED. Yang sudah: F1 dan F2 terimplementasi penuh termasuk seluruh koreksi audit AFTER; §7.1
targeted hijau pada markup final dengan dua mutasi merah-hijau; §7.2 click-through hidup terekam dengan
frame SSE nyata (idle tanpa tab, lima state berkotak 44px, in-flight berwarna, 390px nol overflow, dua
tema, keyboard lolos). Yang tersisa sebelum CLOSED dan commit: rebuild `app-ui`, ulang §7.2 pada build
itu, jalankan lima gerbang penuh `app-ui`, timpa angka §7.1/§7.2, balikan Status, lalu satu commit lokal
tanpa push berisi seluruh set ini (termasuk SPEC-UI §6.5, DESIGN.md §11, register 007, README, dokumen
ini).
