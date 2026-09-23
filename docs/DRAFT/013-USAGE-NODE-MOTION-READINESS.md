# 013-USAGE-NODE-MOTION-READINESS.md: Kesiapan Node Motion di Panel (app-ui)

Dokumen kerja lanjutan `012-USAGE-LIVE-UI-READINESS.md`, dengan scope **hanya `app-ui/.`**. Kontrak
tetap `docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel) dan `DESIGN.md` (arah visual). Pola mengikuti
`008` sampai `012`: temuan bernomor F, bukti yang bisa diulang, rencana DURING, keputusan owner di
depan implementasi.

| | |
|---|---|
| **Status** | CLOSED 2026-09-22. F1 sampai F3 diimplementasikan dan direkam; F4 (route gateway) ditutup di scope `app-serv` dengan bukti live pass di §6.1 |
| **Mechanism** | DURING & AFTER (antislop) |
| **Tanggal** | 2026-09-22 |
| **Scope** | `app-ui/.` untuk F1 sampai F3, lalu `app-serv/.` untuk F4 (§6.1). Keduanya dicatat di dokumen ini |
| **Layar** | `/usage` (Overview tab, panel Live routing) |
| **Reference fork** | `https://github.com/decolua/9router` (checkout `origin/master` di `/home/rusmanadodi/apps/9router`), `ProviderTopology.js` |
| **Kaitan** | SPEC-UI §6.5; DESIGN.md §2.1, §11 (R-13, R-19); draft 012 F3; draft 010 §10.3 |

## 1. Ringkasan

Draft 012 membangun pembaca stream, topologi node, dan koreksi `per_page`, lalu menahan CLOSED F2
dan F3 karena route gateway belum ada. Owner membuka pass ini dengan satu arahan: **bangun tuntas
NODE MOTION dari fitur `/usage`**, dengan parity ke reference dan sentuhan panel sendiri. Panel
sekarang hanya memberi satu gerak pada node: denyut titik saat provider aktif. Reference memberi
tiga, dan ketiganya adalah informasi, bukan hiasan:

1. **Edge aktif dianimasikan** (`ProviderTopology.js:190`, `animated: active`): garis menuju provider
   yang sedang dilayani bergerak, jadi arah trafik terbaca tanpa membaca label.
2. **Node aktif berganti perlakuan** (`:33-38`): border ikut warna state, label ikut warna state, dan
   ada glow lembut; `:66-71` menambah denyut yang sudah panel punya.
3. **Gateway menghitung yang sedang jalan** (`:91-95`): jumlah request in-flight tampil di node pusat.

Pass ini menyelesaikan ketiganya dalam bahasa visual panel (token status, radius, tanpa warna per
provider), dan menambahkan satu aturan yang reference tidak punya: **gerak hanya hidup selama frame
benar-benar tiba**, sehingga stream yang dijeda atau jatuh tidak meninggalkan gambar yang terus
berdenyut seolah ada trafik.

## 2. Keputusan owner yang dipakai

| # | Pertanyaan | Jawaban | Konsekuensi |
|---|---|---|---|
| D1 | Mode antislop | DURING & AFTER | Aturan diterapkan sambil menulis, ditutup laporan Delivery Gate empat blok |
| D2 | Bentuk NODE MOTION | Parity reference | Garis putus berjalan di edge aktif, glow lembut di node aktif, hitungan in-flight di gateway |
| D3 | Register pass | DRAFT 013 baru | Dokumen ini, terpisah dari 012 |
| D4 | Status route live | Full implementasi; F2-F3 dianggap selesai di commit `app-serv` terbaru | Diverifikasi ke tree: route belum ada (F4). Panel dibangun penuh; live pass memakai contract double, dan CLOSED F2/F3 di 012 tetap ditahan sampai route nyata menjawab |

## 3. F1 MEDIUM (FE): gerak node berhenti di satu denyut, bukan parity reference

**Status: DURING selesai.**

**Fakta.** `UsageTopology.svelte` hanya punya `animate-ping` pada titik node aktif
(`UsageTopology.svelte:139-143` pada `e1819cc`, keadaan sebelum pass ini). Edge hanya berganti warna
dan lebar (`EDGE`, `:51-56` di commit yang sama), node tidak
berganti border maupun label, gateway tidak menghitung apa pun. Reference, pada layar yang sama:
edge aktif `animated` (`ProviderTopology.js:190`), node aktif memakai border warna state, label warna
state, dan `boxShadow: 0 0 16px colour40` dengan transisi 300ms (`:33-38`), plus badge `activeCount`
di node pusat (`:91-95`). Arahan owner di sesi 012 sudah menyebut "NODE ANIMATION seperti REFERENCE";
yang dibangun saat itu baru denyutnya.

**Risiko dan aturan.** R-19 (motion harus punya tujuan tertulis dan sesuai dial MOTION 1); R-13
(dose cap glow); R-29 (palet: satu accent plus warna status, jadi warna per provider reference tidak
dipakai); R-25 (setiap pasangan teks baru harus terukur); R-31 (alasan satu baris per keputusan).

**Rencana DURING (FE).**

1. `app.css`: satu animasi `--animate-flow` (`flow 0.9s linear infinite`) dengan keyframes
   `stroke-dashoffset: -6`, yaitu tepat satu periode dash sehingga loop mulus.
2. `UsageTopology.svelte`: `<line>` memakai `pathLength="100"`, dan edge state `active` menambah
   `[stroke-dasharray:3_3]` + `animate-flow` + `motion-reduce:animate-none`. `pathLength` dipakai
   karena kotak gambar memakai `preserveAspectRatio="none"`; tanpa normalisasi itu dash akan lebih
   panjang di satu sumbu daripada di sumbu lain.
3. Node state `active`: border dan label memakai `--color-ok`, plus glow `0 0 16px` pada alpha rendah;
   `transition-colors` supaya pergantian state halus (state transition, bukan loop).
4. Gateway menampilkan jumlah in-flight sebagai chip kecil, hanya ketika lebih dari nol.
5. Kalimat `figcaption` dan padanan teks di bawah gambar ikut menyebut gerak dan hitungan, karena
   gambarnya `aria-hidden`.

**Kriteria selesai.** Hanya edge provider aktif yang membawa dash dan animasi; hanya node aktif yang
membawa border/label status dan glow; chip hitungan sama dengan jumlah entri aktif; ketiganya padam
untuk pembaca yang meminta reduced motion; pasangan teks baru terukur di `contrast.test.ts`.

## 4. F2 MEDIUM (FE): gerak yang berjalan tanpa frame adalah klaim aktivitas

**Status: DURING selesai.**

**Fakta.** Denyut sekarang bergantung pada `active` saja, bukan pada status stream. Stream yang
dijeda operator, atau yang koneksinya jatuh, tetap menyimpan entri terakhir (guard 60 detik), jadi
node akan terus berdenyut sementara chip status berkata `Paused` atau `Unavailable`. Kriteria
selesai draft 012 F3 berbunyi "tidak ada aktivitas yang diklaim tanpa frame", dan R-36 melarang
klaim live tanpa sumber yang hidup.

**Risiko dan aturan.** R-36 (tidak ada klaim tanpa stream); R-19 (gerak harus berarti sesuatu);
SPEC-UI §8.6.1 (interval terlihat, bisa dijeda, berhenti saat tab hidden).

**Rencana DURING (FE).**

1. `UsageTopology` menerima prop `live` (frame tiba di koneksi yang terbuka sekarang).
2. Denyut dan aliran hanya hidup bila `live` benar. Perlakuan state (warna titik, border, glow, chip
   hitungan) tetap tampil sebagai keadaan terakhir yang diketahui, karena itu state, bukan gerak.
3. `UsageLivePanel` menyalurkan `report.status === 'live'`.
4. Test: dengan frame aktif dan `live` benar, denyut dan aliran ada; setelah `pause`, keduanya hilang
   sementara node tetap membawa state terakhirnya.

**Kriteria selesai.** `Paused` dan `Unavailable` tidak menampilkan satu pun gerak, dan test memaku
perbedaan antara "state terakhir" dan "sedang terjadi".

## 5. F3 LOW (FE): glow parity bertabrakan dengan dose cap R-13, dicatat sebagai override

**Status: DURING selesai, override tercatat.**

**Fakta.** Reference memakai glow per node aktif. R-13 membatasi glow pada "maksimum 1-2 elemen".
Bila tiga provider aktif bersamaan, tiga node akan membawa glow, jadi batas huruf R-13 terlampaui.
Owner memilih parity (D2) setelah opsi rekomendasi tanpa glow disodorkan, jadi ini keputusan owner
yang dicatat, bukan pelanggaran senyap (R-37).

**Risiko dan aturan.** R-13 (dose cap), R-37 (bila arah yang diminta bertabrakan dengan aturan,
sebutkan dan catat jawabannya), R-31.

**Rencana DURING (FE).**

1. Glow hanya ada pada state `active`, yaitu state yang berakhir saat frame berhenti menyebutnya.
2. Alpha rendah tanpa spread (`0 0 16px` pada 25%), dan tidak ada elemen lain di layar yang memakai
   glow, jadi tidak ada kombinasi card + button + badge + icon + background + border.
3. Satu baris override dicatat di `DESIGN.md` §11 (reason log) dan satu baris di §2.1 yang menyebut
   gerak indikator live.

**Kriteria selesai.** Glow hanya pada node aktif, alasannya tertulis satu baris di DESIGN.md, dan
tidak ada glow di elemen lain.

## 6. F4 BLOCKER (BE): route `GET /api/v1/usage/live` masih belum ada di tree

**Status: CLOSED 2026-09-22 (scope `app-serv`; bukti live pass di bawah).**

**Fakta.** Klaim sesi ini adalah F2-F3 sudah selesai di commit `app-serv` terbaru. Verifikasi ke
tree, tiga lapis: `grep -rn "usage/live" app-serv/` mengembalikan nol baris source; daftar
`mux.Handle` di `app-serv/internal/router/router.go:185-188` memuat empat read Usage tanpa route
stream; dan `git log --all -S"usage/live"` hanya menemukan commit FE serta dokumen. Yang mendarat di
`app-serv` adalah separuh state (`1ad828d`, bus `usage.recorded`) dan dukungan transport
(`e286cb8`, `http.Flusher` diteruskan middleware), bukan route-nya. Commit `app-serv` terbaru di
branch ini tetap `1ad828d`.

**Risiko dan aturan.** AGENTS.md §2.4 (kontrak dulu, kode kemudian); R-36 (tidak ada klaim live
tanpa stream); R-35 (klaim click-through harus direkam, bukan diasumsikan).

**Rencana DURING (pass ini).**

1. Live pass tetap dijalankan penuh, tetapi dengan **contract double**: satu proses kecil yang
   mem-proxy seluruh `/api/v1/*` ke `app-serv` dan menjawab `/api/v1/usage/live` sendiri dengan frame
   sesuai kontrak draft 010 §10.3 (`active`, `recent`, `error_provider`).
2. Click-through headless Chrome direkam di atas pasangan itu: layar idle, frame tiba (gerak hidup),
   pause (gerak padam), resume, stream ditutup (unavailable), dan `Try again`.
3. Kejujuran laporan: frame berasal dari double, bukan dari gateway, dan itu disebut di setiap baris
   bukti. F2/F3 di 012 tetap ditahan CLOSED sampai route nyata menjawab.

**Kriteria selesai.** Route nyata menjawab dengan session, dan live pass merekam frame dari gateway
sendiri. Sampai itu terjadi, panel tetap merender `unavailable` dengan sebabnya ketika gateway tidak
menyediakan route.

### 6.1 Bukti penutupan (2026-09-22, scope `app-serv`)

Route-nya ada di tree sekarang: `mux.Handle("GET "+APIVersion+"/usage/live", ...)` di
`internal/router/router.go`, session-gated bersama empat read Usage di sebelahnya, dengan handler
`internal/handler/usage_live.go` dan kontrak di `docs/CONTRACT/001-CONTRACT-API-V1.yaml` (media type
`text/event-stream`, schema `UsageLiveFrame`).

**Tiga keputusan desain yang berbeda dari rencana draft 010 §10.3, masing-masing dengan alasan.**

1. **Sorted set, bukan hash.** Draft 010 §10.3 mengusulkan hash `pannelai:active:<request_id>` dengan
   TTL 60 detik. Sebuah hash memaksa `HGETALL` (tak terbatas) atau `SCAN` (dilarang di request path,
   AGENTS.md §1.7). Yang dipakai: satu sorted set `pannelai:usage:active`, skor = `started_at`, dibaca
   dengan `ZRANGEBYSCORE` yang menerapkan cutoff dan limit di server. Member-nya adalah bentuk
   terenkode dari marker itu sendiri, jadi pelepasan adalah satu `ZREM` eksak dan pemangkasan adalah
   satu `ZREMRANGEBYSCORE` pada key yang sama: tidak ada key kedua yang harus dijaga sinkron, jadi
   tidak ada entri yatim yang bisa bocor.
2. **Tiga seam, bukan satu choke point.** Draft 010 §10.3 meminta "satu choke point sebelum relay".
   Gateway punya tiga bidang outbound (chat lewat `relayOnce`, media lewat `Perform`, embeddings lewat
   `perform`) dan tidak ada satu fungsi yang dilalui semuanya. Yang dipakai: satu tipe
   `ActiveRequestTracker` yang dipakai di tiap seam, mengikuti preseden `QuotaCounter` dan
   `dataPlaneRecorder` yang sudah ada. Alternatifnya menaruh field akuntansi di struct `Call` dan
   `MediaRequest`, tempat yang bukan miliknya (AGENTS.md §1.5).
3. **`error_provider` diturunkan, bukan key sendiri.** Nilainya diambil dari pembacaan `recent` yang
   sama dalam jendela 10 detik, bukan dari key Redis kedua. Dua sumber untuk satu fakta bisa berbeda
   pendapat tentang request mana yang mereka maksud; satu sumber tidak bisa.

**Bukti live pass (R-35), dijalankan pada tree beku dengan `-race -tags=integration`.**

| # | Yang direkam | Hasil terukur |
|---|---|---|
| 1 | Panggilan anonim ke route | `status=401 content_type=application/json`, yaitu envelope §8, bukan stream |
| 2 | Satu panggilan data-plane nyata, upstream menahan jawabannya | `frames=2 active=1 provider="live" model="live-model" started_at="2026-09-22T14:12:12Z" recent=0` |
| 3 | Upstream melepas jawaban, panggilan selesai | `status=200`, lalu `active=0 recent=1 status="success" tokens_in=7 tokens_out=3` (angka upstream, bukan angka yang dikarang) |
| 4 | Panggilan yang ditolak upstream | `status=502`, lalu `error_provider="live" recent=1 recent_status="error"` |
| 5 | Koneksi idle | frame pertama `{"active":[],"recent":[],"error_provider":""}`, baris berikutnya `": ping\n"` (komentar SSE, bukan frame) |

Panggilan data-plane di baris 2 dan 3 berjalan di atas upstream HTTP nyata lewat leg relay nyata, dan
marker-nya dibaca dari Redis nyata melalui route nyata di atas socket nyata. Frame-nya **berasal dari
gateway**, bukan dari double: itu perbedaan yang draft 013 catat sebagai satu-satunya yang menahan
CLOSED. Yang tidak diklaim: klik-through browser pada route nyata tetap belum direkam, karena pass ini
scope `app-serv`; panel sudah merekam klik-through-nya di draft 013 §8.1 terhadap contract double, dan
F2/F3 draft 012 sekarang bisa ditutup karena route yang mereka tunggu sudah menjawab.

**Ditutup 2026-09-22 oleh pass panel (draft 016 §12.1).** Klik-through browser pada route nyata yang
paragraf di atas sebut belum direkam sudah direkam: lima tangkapan layar dan tujuh kelompok nilai
terukur di `/tmp/live016/`, plus 21 pemeriksaan klien yang menggerakkan modul panel sendiri, semuanya
di atas gateway yang dibangun dari tree yang di-commit (`eeef715`) dan trafik nyata lewat data plane.
Tidak ada contract double di jalur bukti itu.

**Gate `app-serv` pada tree beku.** `go build ./...` bersih; `go vet ./...` dan
`go vet -tags=integration ./...` bersih; `go test -race -count=1 ./...` hijau di tujuh belas paket;
`gofmt -l .` kosong; `scrypts/gates/go-headers.sh` PASS 747 file; `staticcheck` (dua build) bersih;
`golangci-lint` 0 issue; `contract-openapi.sh` dan `contract-drift.sh` PASS. Satu peringatan
§1.1 yang tersisa adalah `cmd/app-serv/management_wiring.go` (239 baris, di bawah cap 250; file ini
241 baris di HEAD, jadi pass ini menurunkannya).

## 7. Non-findings

- **Layout elips tidak disentuh.** Posisi, tinggi kotak, dan state precedence sudah dipaku test di
  draft 012; pass ini hanya menambah gerak dan perlakuan pada state yang sudah ada.
- **Tidak ada dependensi baru.** Animasi dash adalah satu token CSS di `app.css`, bukan pustaka graf.
- **`/logs` dan `/quota` tidak disentuh.** Keduanya memakai polling dengan keputusan `pollDue`.
- **Totals dan chart tetap milik REST.** Tidak ada field agregat yang berpindah tangan di pass ini.

## 8. Bukti gate

Dijalankan dari `app-ui/` pada tree beku.

| Gate              | Hasil                                                                                                                       |
| ----------------- | --------------------------------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 error, 0 warning                                                                                                          |
| `bun run test`    | 2448 test lulus di 138 file (pass terakhir 2435 di 137; pass ini menambah 13: delapan di `usage-topology-motion.test.ts`, satu di `usage-live-drawing.test.ts`, dan empat dari dua pasangan teks baru di `contrast.test.ts` yang diukur sekali per tema) |
| `bun run lint`    | Prettier melaporkan semua file sesuai                                                                                       |
| `bun run lint:ts` | ESLint keluar 0                                                                                                             |
| `bun run build`   | sukses, keluar 0, output di `build/`                                                                                        |
| Verifikasi CSS    | hasil build memuat `@keyframes flow`, `.animate-flow`, `[stroke-dasharray:3_3]`, dan glow `0 0 16px` dari token status, jadi kelasnya benar-benar dihasilkan, bukan hilang diam-diam |
| Ukuran file       | file terbesar yang disentuh 207 baris (`tests/tokens/contrast.test.ts`, di bawah ambang 220); `UsageTopology.svelte` 201, `app.css` 189, `UsageLivePanel.svelte` 186, tiga file test 149. Tidak ada file over-cap lama yang disentuh |
| Kebersihan teks   | 0 em dash di seluruh file baru dan yang diubah                                                                              |

### 8.1 Live pass: click-through browser yang pass lalu catat blocked

Pass ini merekam yang pass sebelumnya tidak bisa: **click-through di browser nyata**. Chrome headless
150.0.7871.114 menggerakkan panel hasil `bun run build` di atas pasangan proses nyata, dengan
**contract double** di depan gateway karena route `GET /api/v1/usage/live` masih tidak ada (F4).
Sepuluh babak, tiap babak merekam screenshot dan nilai terukur:

1. Panel menolak boot saat `.env` `app-serv` ikut terwarisi (`PANEL_BOOTSTRAP_PASSWORD: Unknown panel
   variable`); itu justru kebenaran skema env pass ke-19, dan panel dijalankan ulang dengan env bersih.
2. Login lewat form panel sendiri (satu field, tombol `Sign in`).
3. `/usage` keadaan `Connecting`: 6 node dari registry, nol gerak, nol frame.
4. Frame tiba: **tepat satu edge mengalir** (`animation-name: flow`, `stroke-dasharray: 3px, 3px`,
   `stroke-dashoffset` berubah antar-capture), node aktif border dan label `rgb(20, 116, 58)`, glow
   `0 0 16px` pada 25%, chip gateway `1`, satu denyut.
5. `Pause live updates`: aliran dan denyut hilang, warna state dan chip hitungan tetap. Resume
   mengembalikan keduanya.
6. Double menutup stream: `Unavailable` dengan kalimat "The gateway closed the live stream. Another
   attempt is scheduled."
7. Anggaran retry habis: "The live stream answered 503. Retrying has stopped. Use Try again to
   reconnect."
8. `Try again` setelah double menjawab lagi: kembali `Live` dengan aliran hidup.
9. Tema gelap: seluruh perlakuan ikut bertukar (`rgb(92, 201, 123)`, glow memakai token gelap).
10. Viewport 390px: `scrollWidth == clientWidth == 390`, tidak ada overflow horizontal.

Baseline DB dihitung sebelum dan sesudah dan kembali persis: `usage=0 logs=0 keys=0 endpoints=0
upkeys=0 nodes=2 caps=0 settings=0 auth_null=0 media_settings=0 proxies=0`. Kedua server gateway dan
double dihentikan lewat PID yang dicatat.

Yang **tidak** diklaim: frame itu datang dari double, bukan dari gateway. Route nyata masih nol baris
di `app-serv` pada `e1819cc`, jadi CLOSED F2/F3 di draft 012 tetap ditahan dan click-through
route-nyata tetap outstanding.

## 9. Status per 2026-09-22

- F1, F2, F3 DURING selesai dengan test, verifikasi CSS hasil build, dan click-through browser di §8.1.
- F4 **CLOSED** di scope `app-serv`: route `GET /api/v1/usage/live` menjawab dengan session, dan live
  pass merekam frame dari gateway sendiri (§6.1). Ini yang menahan CLOSED F2 dan F3 di draft 012, jadi
  keduanya tidak lagi tertahan oleh route yang tidak ada. Yang belum direkam: klik-through browser pada
  route nyata, karena pass ini scope `app-serv`; panel sudah merekam klik-through-nya terhadap contract
  double di §8.1.
- Register 007: F12 bergerak ke dua belas dari dua puluh satu bagian pass, dan F13 kini menyebut empat
  baris pembuka yang mengutip `1.3.14` untuk menunjuk temuannya (klaimnya tetap sembilan belas situs).
