# 012-USAGE-LIVE-UI-READINESS.md: Kesiapan Live Usage di Panel (app-ui)

Dokumen kerja lanjutan `010-USAGE-ENDPOINT-READINESS.md`, dengan scope **hanya `app-ui/.`**. Kontrak
tetap `docs/SPEC-API/001-SPEC-API.md` (wire), `docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel), dan
`docs/CONTRACT/001-CONTRACT-API-V1.yaml`. Pola mengikuti `008` sampai `011`: temuan bernomor F, bukti
yang bisa diulang, rencana DURING, keputusan owner di depan implementasi.

| | |
|---|---|
| **Status** | DURING selesai 2026-09-22. F1 CLOSED; F2 dan F3 diimplementasikan penuh dengan test dan live pass, tetapi CLOSED ditahan oleh F4; F6 dicatat sebagai follow-up LOW |
| **Mechanism** | DURING & AFTER (antislop) |
| **Tanggal** | 2026-09-22 |
| **Scope** | `app-ui/.` saja. `app-serv/.` tidak disentuh oleh dokumen ini |
| **Layar** | `/usage` (Overview tab, Records tab) |
| **Reference fork** | `https://github.com/decolua/9router` (checkout `origin/master` di `/home/rusmanadodi/apps/9router`) |
| **Kaitan** | SPEC-UI §6.5, §8.6, §9.5; SPEC-API §7.12; DESIGN.md §2.1; draft 010 F10 sampai F14 |

## 1. Ringkasan

Draft 010 menutup sembilan temuan `app-serv` dan satu temuan FE, lalu menyisakan dua hal di sisi
panel: `per_page` URL yang tidak pernah direset (010 F12) dan keputusan live/SSE (010 F10, F13).
Owner memutuskan di sesi ini:

1. **SSE parity dipilih** (010 D5): panel membangun pembaca stream, bukan polling dan bukan status quo.
2. **Node animation seperti reference** (010 D6 direvisi): indikator aktivitas provider berdenyut
   seperti `ProviderTopology.js`, dengan sentuhan panel sendiri.
3. **Scope sesi ini hanya `app-ui`**: route dan state in-flight di `app-serv` dikerjakan tim lain.

Satu kenyataan yang menentukan status: **tree `app-serv` hari ini belum memuat route
`GET /api/v1/usage/live`**, sehingga F2 dan F3 dibangun terhadap kontrak yang direncanakan draft 010
§10.3 dan belum bisa dinyatakan CLOSED. Panel tidak boleh mengklaim live tanpa stream (R-36), jadi
keadaan "stream tidak tersedia" adalah keadaan yang benar-benar dirender, bukan cacat tersembunyi.

## 2. Keputusan owner yang dipakai

| # | Pertanyaan | Jawaban | Konsekuensi |
|---|---|---|---|
| D5 | Live Usage: status quo, polling, atau SSE? | SSE penuh (BE + FE) | FE membangun reader stream; BE menyusul di scope tim `app-serv` |
| D6 | Indikator aktif: transisi state atau denyut? | Denyut terbatas ala reference | DESIGN.md MOTION 1 diamandemen satu baris: denyut berjalan hanya selama provider aktif dan berhenti saat idle |
| D7 | Sumber node topologi | Provider yang terkonfigurasi | Node dibaca dari `GET /api/v1/providers`, bukan 94 entri registry (parity reference, menghindari node yang tidak pernah tersentuh) |
| D8 | Guard state aktif | Guard FE 60 detik | BE yang berhenti mengirim frame tidak meninggalkan node aktif bohong; bila BE nanti memakai TTL Redis, guard FE tetap tinggal sebagai jaring terakhir |

## 3. F1 LOW (FE): `per_page` URL diabaikan diam-diam, tanpa reset

**Status: CLOSED 2026-09-22.** Pindahan 010 F12, dan F6 `app-serv` sudah CLOSED (nilai di atas 100
ditolak, bukan diklem).

**Fakta.** `parseUsageSearch` (`app-ui/src/lib/schemas/usage-search.ts:72-77`) hanya menulis notice
"per_page was ignored" dan membiarkan nilai lama di URL. Karena tab Records membaca 25 baris, URL
dengan `per_page=100` berarti tiga jawaban berbeda untuk satu parameter: nilai di URL, notice di layar,
dan jumlah baris yang benar-benar dibaca.

**Risiko dan aturan.** SPEC-UI §7.1.1 dan §8.10 (koreksi harus terlihat, bukan senyap); R-26 (kontrol
yang tidak melakukan apa labelnya); AGENTS.md §2.4 (kontrak dan perilaku harus sepakat).

**Rencana DURING (FE).**

1. `UsageSearch` membawa `perPage: number` dan `perPageNotice: string | null`. Nilai divalidasi lewat
   primitif panel `perPage` (`src/lib/schemas/primitives.ts:170`), lalu dikoreksi ke
   `USAGE_RECORDS_PAGE_SIZE` bila bukan 25. Notice menyebut reset ke 25, bukan "diabaikan".
2. `cleanedUsageSearch(current)` mengembalikan URL tanpa `per_page`, atau `null` bila tidak ada yang
   perlu dibersihkan.
3. `UsageRecordsTab.svelte` menjalankan koreksi itu sekali: notice disimpan untuk ditampilkan,
   `goto` URL bersih dengan `replaceState`, dan pembacaan hanya terjadi setelah URL bersih. Notice
   bertahan sampai operator mengubah filter, jadi koreksinya terlihat, bukan sekilas.
4. Test: tabel nilai (`25`, `100`, `0`, `-1`, `1.5`, `lots`, `100000`, tanpa parameter, parameter
   asing) di `tests/schemas/usage-view.test.ts`, dan bukti URL bersih plus satu pembacaan di
   `tests/components/usage-records.test.ts`.

**Kriteria selesai.** URL hasil koreksi tidak membawa `per_page`, notice menyebut angka 25, hanya satu
pembacaan terjadi, dan test memaku ketiganya.

## 4. F2 MEDIUM (FE): pembaca stream Usage belum ada

**Status: DURING selesai, CLOSED ditahan oleh F4.**

**Fakta.** Tidak ada `EventSource`, schema frame, maupun modul stream di `app-ui` (grep `EventSource`
atas `src` dan `tests` mengembalikan nol baris). Reference membuka `EventSource("/api/usage/stream")`
dan **hanya** menimpa empat field real-time di atas stats REST (`UsageStats.js:268-295`), bukan
mengganti agregat.

**Risiko dan aturan.** SPEC-UI §6.5 ("panel computes nothing it cannot cite"); §8.6.1 (interval
terlihat, bisa dijeda, berhenti saat tab hidden) digeneralisasi ke medium stream; R-17 dan R-36 (tidak
ada angka atau klaim live tanpa sumber); R-26 (tidak ada kontrol yang tidak berbuat apa-apa).

**Rencana DURING (FE).**

1. `src/lib/schemas/usage-live.ts`: schema Zod untuk frame `active`, `recent`, `error_provider`.
   Koleksi memakai `nullableList` (Go mengirim `null` untuk slice kosong), waktu memakai
   `optionalTimestamp`, dan field yang tidak dikenal ditoleransi supaya penambahan field di gateway
   tidak menjatuhkan layar.
2. `src/lib/schemas/usage-live-view.ts`: derivasi murni tanpa DOM dan tanpa `EventSource`:
   `liveMerge` (hanya field real-time yang menimpa, totals dan chart tetap milik REST),
   `activeProviders` (guard kedaluwarsa 60 detik), `streamState`, dan `topologyNodes` untuk layout
   elips.
3. `src/lib/usage-live.ts`: pemilik koneksi, reconnect dengan backoff terbatas, berhenti saat tab
   hidden dan saat dijeda operator, dan menyatakan `unavailable` ketika koneksi gagal. Label "Live"
   hanya muncul setelah frame pertama benar-benar tiba.
4. Test: tabel parse frame, tabel merge dan kedaluwarsa (termasuk jam mundur), dan test komponen yang
   membuktikan totals REST tidak tertimpa frame.

**Kriteria selesai.** Frame pertama terukur sampai layar pada live pass, merge tidak pernah menimpa
agregat REST, dan tidak ada label live tanpa stream. Dua syarat terakhir sudah dipaku test; syarat
pertama menunggu F4.

## 5. F3 MEDIUM (FE): topologi node belum ada, dan keputusan motion belum tertulis

**Status: DURING selesai, CLOSED ditahan oleh F4.**

**Fakta.** `app-ui` tidak punya komponen topologi. Reference menggambar router di tengah plus satu
node per provider terkonfigurasi pada elips, menandai node aktif dengan `animate-ping`, menganimasikan
edge saat aktif, dan memakai guard 60 detik (`ProviderTopology.js:14-16,107-196`). DESIGN.md menetapkan
MOTION 1 ("hover, focus, state transition only"), sehingga denyut abadi bertabrakan dengan dial.

**Risiko dan aturan.** R-19 (motion harus punya tujuan tertulis dan sesuai dial); R-03 (tiga bentuk
layar: mobile, tablet, desktop); R-04 (ikon hanya kalau relevan); R-17 (tidak ada node yang mengklaim
aktivitas tanpa frame); §9.4.5 SPEC-UI (setiap elemen interaktif diverifikasi).

**Rencana DURING (FE).**

1. `UsageTopology.svelte` menggambar layout elips tanpa dependensi graph: router di pusat, satu node
   per provider dari `GET /api/v1/providers`, edge berubah state (aktif, terakhir, error).
2. Denyut hanya berjalan selama provider benar-benar aktif di frame terakhir, dan berhenti saat frame
   menyatakan tidak ada yang berjalan. Itu yang membuat amandemen MOTION bisa ditulis satu baris:
   indikator aktivitas yang berhenti sendiri saat idle bukan loop abadi.
3. Setiap topologi punya padanan teks (daftar provider aktif beserta model dan durasi, atau kalimat
   bahwa tidak ada request berjalan), mengikuti pola tabel fallback `UsageChart`.
4. Nol provider terkonfigurasi adalah keadaan kosong yang menyebut sebabnya dan menautkan `/providers`.
5. Test: node per provider, state aktif/error/terakhir, padanan teks, keadaan kosong, dan bukti denyut
   berhenti saat frame bersih.

**Kriteria selesai.** Node hanya muncul untuk provider yang benar-benar terkonfigurasi, tidak ada
aktivitas yang diklaim tanpa frame, dan click-through mencatat keadaan idle serta unavailable.

## 6. F4 BLOCKER (BE): route `GET /api/v1/usage/live` belum ada di tree

**Status: OPEN. Ini yang menahan CLOSED F2 dan F3.**

**Fakta.** Grep atas `app-serv/` untuk `usage/live`, `UsageLive`, dan `activeRequests` tidak
menemukan route, handler, maupun schema apa pun; yang ada hanya SSE pada jalur chat dan playground.
Draft 010 §10.3 menulis route, frame, dan sumber state sebagai **usulan**, dan `SPEC-API` §7.12 belum
menjanjikan stream.

**Risiko dan aturan.** AGENTS.md §2.4 (kontrak dulu, kode kemudian); R-26 (panel tidak boleh memasang
kontrol yang tidak punya lawan bicara); R-36 (tidak ada klaim live tanpa stream).

**Rencana DURING (tim `app-serv`, dicatat di sini supaya tidak hilang).**

1. Sumber state in-flight (Redis TTL 60 detik atau Pub/Sub) dengan satu choke point sebelum relay.
2. Route session-gated `text/event-stream`, `X-Accel-Buffering: no`, keepalive `: ping` maksimal 25
   detik, frame `{"active": [...], "recent": [...], "error_provider": "..."}` dari sumber bounded.
3. Amandemen SPEC-API §7.12, entri OpenAPI, dan pembaruan SYSTEM_MAP.md pada perubahan yang sama.

**Kriteria selesai.** Route nyata menjawab dengan session, dan live pass merekam frame pertama sampai
layar. Sampai itu terjadi, panel merender keadaan `unavailable` dengan sebabnya.

**Catatan 2026-09-22, setelah pass di §10.** Langkah 1 baru sebagian, dan sebagiannya sudah mendarat:
commit `62d21e1` (sisi `app-serv` draft 010 F4) memberi `usage.recorded` sebuah publisher bounded dan
subscriber nyata lewat Redis Pub/Sub di channel `pannelai:events:usage.recorded`. Bus itu membawa
request yang **sudah selesai**, jadi ia mengisi separuh `recent` pada frame yang direncanakan;
himpunan in-flight yang mengisi `active` masih butuh sumbernya sendiri (TTL Redis atau hitungan di
choke point relay). Route-nya sendiri belum ada: `grep -rn "usage/live" app-serv/` mengembalikan nol
baris pada tree ini, sehingga keadaan yang panel render hari ini tetap `unavailable` dengan sebab 404,
dan F4 tetap OPEN. Langkah 2 dan 3 belum tersentuh.

## 7. F5 INFO: catatan kontrak dan dokumen

- Panel tidak menambah route sendiri: `EventSource` menembak `/api/v1/usage/live` melalui origin panel,
  yang sudah mem-forward `/api/v1` (SPEC-UI §3.1), jadi tidak ada CORS baru.
- Amandemen dokumen pada perubahan ini: SPEC-UI §6.5 (paragraf Live), §8.6.1 (stream sebagai medium
  ketiga), §9.5 (baris "Polling instead of a live stream" ditandai digantikan), dan DESIGN.md §2.1
  (satu baris pengecualian MOTION untuk indikator aktivitas).
- `app-ui/README.md` mencatat slice ini pada tabel keputusan dan tabel gate.

## 8. F6 LOW (FE): pola notice-only `per_page` yang sama masih ada di dua layar lain

**Status: OPEN, dicatat 2026-09-22 sebagai follow-up F1.**

**Fakta.** F1 mengoreksi `per_page` di layar Usage: nilainya dibaca, dikoreksi ke ukuran yang layar
pakai, notice menyebut angkanya, dan parameternya dibuang dari URL. Dua layar lain masih memakai pola
lama yang hanya melaporkan: `log-search.ts:67-69` menulis "This screen reads N requests per page, so
per_page was ignored." dan `endpoint-search.ts:78-81` menulis kalimat yang sama untuk endpoints, tanpa
mengoreksi URL maupun membaca ulang. Jadi satu URL yang dibagikan tetap berarti tiga hal berbeda:
nilai di URL, notice di layar, dan jumlah baris yang benar-benar dibaca.

**Risiko dan aturan.** SPEC-UI §7.1.1 dan §8.10 (koreksi harus terlihat, dan URL adalah sumber
kebenaran); R-26 (kontrol yang tidak melakukan apa labelnya); AGENTS.md §2.4.

**Kerja yang diusulkan.** Angkat pola F1 ke satu tempat (satu jalur koreksi `per_page` yang dipakai
tiga layar), lalu pakai di `log-search.ts` dan `endpoint-search.ts` dengan test tabel per layar dan
bukti satu pembacaan di test komponennya. Tidak dikerjakan di pass ini karena scope-nya tiga layar di
luar Usage.

**Kriteria selesai.** Ketiga layar memakai satu jalur koreksi yang sama, dan URL hasil koreksi tidak
membawa `per_page` di layar mana pun.

## 9. Non-findings

- **010 F11 tetap CLOSED.** Placeholder "Request id, error code" sudah benar sejak F8 `app-serv`
  memperluas scope `q`; tidak ada pekerjaan FE yang menyertainya.
- **`/logs` dan `/quota` tidak disentuh.** Keduanya memakai polling dengan keputusan `pollDue` yang
  sudah teruji; modul stream ini tidak menggantikannya.
- **`app-serv` tidak disentuh oleh dokumen ini.** Sembilan temuan draft 010 sudah CLOSED di scope itu,
  dan sisa pekerjaannya (F4, F14) tercatat sebagai blocker, bukan dikerjakan dari sini.
- **Tidak ada dependensi graph baru.** Layout elips digambar sendiri; menambah pustaka graf untuk
  belasan node akan menjadi bobot yang tidak sebanding dengan gunanya.

## 10. Bukti gate

Dijalankan dari `app-ui/` pada tree beku. File `app-serv` yang belum di-commit di tree adalah pekerjaan
tim lain; tidak ada satu pun yang disentuh oleh pass ini.

| Gate              | Hasil                                                                                                                     |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 error, 0 warning                                                                                                        |
| `bun run test`    | 2435 test lulus di 137 file (pass terakhir 2303 di 125 file; pass ini menambah 132 test di dua belas file baru)           |
| `bun run lint`    | Prettier melaporkan semua file sesuai                                                                                     |
| `bun run lint:ts` | ESLint keluar 0                                                                                                           |
| `bun run build`   | sukses, keluar 0, output di `build/`                                                                                      |
| Live pass         | 17 cek, 0 gagal, di atas sepasang proses nyata                                                                            |
| Ukuran file       | file terbesar yang ditulis pass ini 222 baris (`src/lib/usage-live.ts`, dua di atas ambang 220 dan 28 di bawah batas 250) |
| Kebersihan teks   | 0 em dash di seluruh file baru dan yang diubah                                                                            |

Catatan delta: sembilan file yang ditulis pass ini dirapikan Prettier setelah `bun run test` dijalankan,
jadi tiga belas file test yang menutupinya dijalankan ulang dan hijau (141 test) alih-alih mengulang
seluruh suite; reflow hanya mengubah spasi, bukan logika.

Live pass dijalankan di atas sepasang proses nyata: `app-serv` dibangun dari HEAD lewat
`git archive HEAD app-serv` (working tree memuat pekerjaan tim lain yang belum di-commit, jadi build
dari sana akan mengompilasi kode yang belum jadi), lalu panel hasil `bun run build` dijalankan
detached di port 3000 dengan `PANEL_API_TARGET` diekspor. Yang digerakkan adalah modul panel sendiri,
karena panel ini client-rendered (`ssr = false`) sehingga tidak ada markup yang bisa dibaca `curl`:

1. `openUsageLive` melaporkan `unavailable` dengan kalimat gateway sendiri ("The gateway has no live
   stream route yet (it answered 404)."), nol frame, dan label `Unavailable`. Retry yang dijadwalkan
   sebelum `stop` dilepas oleh `stop` (0 laporan dalam 2,5 detik, sementara satu delay retry 1 detik).
2. F1: `parseUsageSearch` membaca `per_page=100` dan mengoreksinya dengan notice yang menyebut 25;
   `cleanedUsageSearch` membuang parameter itu dan mempertahankan sisanya; URL yang sudah memakai 25
   tidak dikoreksi dan tidak menghasilkan notice.
3. Kontrol wire untuk F1: gateway menghormati `per_page=100` (200, `meta.per_page` 100) dan menolak
   `per_page=101` dengan `VALIDATION_ERROR "per_page must be at most 100"` (bukan diklem), jadi
   parameter itu memang dihormati dan koreksi panel-lah yang mencegah tiga jawaban berbeda.
4. F3: `configuredProviders` atas 86 baris registry hidup menghasilkan 6 node
   (`edge-tts`, `google-tts`, `local-device`, `mimo-free`, `opencode`, `searxng`), dan `topologyNodes`
   menempatkannya di posisi berbeda dengan state `idle` saat tidak ada frame. State dari frame juga
   diuji atas id hidup: `edge-tts` menjadi `active` dan `google-tts` menjadi `last`, dengan
   perbandingan id tanpa peduli huruf besar-kecil.
5. F2: `liveMerge` atas frame berbentuk agregat hanya menghasilkan empat field real-time
   (`active`, `errorProvider`, `receivedAt`, `recent`), sehingga stream tidak punya tempat menulis
   agregat.

Baseline DB dihitung sebelum dan sesudah dan kembali persis: `usage=0 logs=0 keys=0 endpoints=0
upkeys=0 nodes=2 caps=0 settings=0 auth_null=false media_settings=0 proxies=0`. Login memakai password
dari `app-serv/.env`, jadi tidak ada hash yang dirotasi; kedua server dihentikan lewat PID yang
dicatat, bukan `pkill -f`.

Click-through browser tetap **blocked**, dan itu dicatat sebagai blocked: panel ini client-rendered
sehingga R-35 tidak bisa dipenuhi dengan `curl`, dan satu-satunya keadaan yang belum bisa direkam
adalah frame yang benar-benar tiba, karena route F4 belum ada di gateway.

## 11. Status per 2026-09-22

- F1 CLOSED dengan bukti test dan live pass: gateway menghormati `per_page=100` sementara panel
  membacanya sebagai 25, dan koreksi URL itu sekarang terlihat serta hanya satu pembacaan yang terjadi.
- F2 dan F3 diimplementasikan penuh dengan test; status CLOSED ditahan sampai F4 selesai dan live pass
  merekam frame nyata. Yang sudah terekam pada live pass sesi ini adalah keadaan sebaliknya, dan itu
  memang keadaan hari ini: route menjawab 404, panel menyatakan `unavailable` beserta sebabnya, dan
  tidak ada label live yang dipasang tanpa frame. Click-through browser dicatat sebagai blocked
  beserta sebabnya (panel client-rendered, route lawan bicara belum ada), bukan diklaim lulus.
- F4 OPEN, milik scope `app-serv`. Ini satu-satunya nomor yang menahan CLOSED F2 dan F3.
- F5 tercatat; amandemen dokumen (SPEC-UI §6.5, §8.6.1, §9.5 dan DESIGN.md §2.1) ikut di perubahan ini.
- F6 OPEN dan LOW: pola notice-only `per_page` yang sama masih ada di `log-search.ts` dan
  `endpoint-search.ts`, dicatat sebagai follow-up F1 untuk pass berikutnya, bukan dikerjakan di sini.
