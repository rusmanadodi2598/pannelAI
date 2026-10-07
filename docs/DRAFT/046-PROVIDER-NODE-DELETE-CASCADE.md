# 046-PROVIDER-NODE-DELETE-CASCADE.md: Delete provider ditolak bukan karena datanya salah, melainkan karena kontraknya menyuruh menolak

Laporan owner 2026-10-07: tombol Delete pada layar custom provider tidak bisa dipakai. Dialog membuka, tombol
"Delete the provider" ditekan, yang muncul adalah `Still referenced. an endpoint still references this provider
Remove or move its endpoints first.` Pemilik menyebut kelas ini "issues DELETE", dan minta delete benar-benar
menghapus provider.

## Status dan lingkup

| | |
| --- | --- |
| **Status** | **HIGH (F1, F2) ter-commit `b41f54b`. Sebagian F4 ikut landed pada commit itu juga: komentar yang menuliskan aturan lama akan menjadi klaim palsu dalam commit yang sama yang menghapus aturannya, jadi itu tidak bisa ditunda ke MEDIUM. Sisanya: F3 (dua spesifikasi + satu baris README), sisa F4 (komentar yang tidak ikut berubah oleh F1), F5 (catatan, bukan kode)** |
| **Mechanism** | DURING: tulisan baru mengikuti R-02 dan R-31, dan pesan yang dikirim ke operator harus bisa dibuktikan produk |
| **Scope** | **Bukan comment-only.** F1 mengubah `app-serv` service, repository, dan wiring; F2 mengubah komponen panel dan testnya; F3 mengubah dokumen kontrak; F4 menyunting komentar yang menuliskan aturan lama. Keluar dari guardrail antislop-code, atas izin eksplisit owner, seperti F4 dan F12 pada 045 |
| **Sumber temuan** | Laporan owner, log `app-serv` 2026-10-07 20:47 (`DELETE /api/v1/provider-nodes/anthropic-compatible-0388PGVSAVAW7MD0VTT2X9SAYA` → `409 CONFLICT`), lalu pengukuran terhadap PostgreSQL nyata dan pohon kode saat ini |
| **Struktural (AGENTS.md §1.9)** | `SYSTEM_MAP.md`: N/A, tidak ada domain boundary, tabel, kolom, atau async topology baru yang bergerak. Yang bergerak adalah semantik satu route, dan itu tinggal di `docs/SPEC-API` §7.4 plus `docs/SPEC-UI` §6.3, keduanya ikut diubah di F3 |

## Diagnosa

Penolakan itu **benar menurut kontrak hari ini**, dan kontrak itulah yang salah.

Fakta terukur, bukan dugaan:

1. Node `ApMix 2` (`ap-2`, `anthropic-compatible-0388PGVSAVAW7MD0VTT2X9SAYA`) masih punya tepat 1 baris
   `upstream_endpoints` dengan label `Key 1`. Diquery langsung ke PostgreSQL: `SELECT provider_id, count(*) FROM
   upstream_endpoints GROUP BY provider_id`.
2. `NodeService.Delete` memanggil `EndpointCounter.CountEndpoints`, dan selama hasil hitungannya lebih dari nol ia
   mengembalikan `domain.ErrNodeInUse`. Angka yang sudah ada di tangan itu dibuang, tidak pernah ikut ke pesan.
3. `ErrNodeInUse` adalah kalimat generik bentuk tunggal: `an endpoint still references this provider`. Guard
   kembarannya di file yang sama, `rejectComboReference`, sudah menyebut penyumbatnya oleh nama
   (`combo <nama> still references this provider`). Jadi ketidakkonsistenan ini ada di dalam satu fitur, bukan
   antara dua berkas.
4. Panel merangkai kalimatnya sendiri di depan kalimat server, tanpa pemisah: `Still referenced. {pesan server}
   Remove or move its endpoints first.` Itulah teks persis yang owner salin. Dua kalimat mengatakan hal yang sama,
   dan yang kedua mengirim operator ke tempat yang tidak ada.
5. "move" tidak ada di produk. `provider_id` tidak muncul di satu pun bentuk update, baik di sisi panel
   (`schemaUpdateEndpointForm`) maupun di sisi API (`UpdateEndpointRequest`, `UpdatePatch`). Yang bisa diubah pada
   sebuah endpoint hanyalah label, priority, status, default model, global priority, dan proxy pool. Menghapus
   koneksi memang bisa, satu per satu, dari bagian Connections pada layar yang sama. Memindahkannya tidak.

Karena itu teks yang sekarang adalah cacat, bukan gaya: ia memerintahkan aksi yang tidak bisa dikerjakan, dan ia
mengulang klaim server dengan kata-kata panel sendiri.

## Keputusan desain

**Delete node menghapus endpoint milik node itu sekaligus. Guard combo tetap menolak.**

Alasan cascade, dan ini alasan struktural bukan kemudahan:

- `upstream_endpoints.provider_id` adalah id node, dan basis URL serta format wire yang membuat endpoint itu bisa
  dijawab hidup di baris node. Node hilang, maka koneksi yang tersisa adalah koneksi yang tidak akan pernah bisa
  dirutekan oleh apa pun. Menolaknya berarti membiarkan baris mati menghidupkan layar.
- Id node tidak dipakai ulang (ia token yang dibuat saat create), jadi tidak ada jalur di mana penompaan ulang prefix
  menyambung kembali endpoint yatim ke node baru.
- Kunci ikut pergi lewat `ON DELETE CASCADE` yang sudah dideklarasikan skema: `upstream_keys.endpoint_id`
  (migrasi 000005) dan dua tabel cache kuota published (000013). Tidak ada DELETE manual yang perlu ditambah, dan
  tidak ada kunci yang tertinggal di tabel.
- Yang tetap tinggal adalah riwayat: `usage_records`, `quota_windows`, `request_logs` menyimpan `provider_id` sebagai
  teks tanpa FK, dan memang begitulah seharusnya. Menghapusnya berarti menghapus bukti pemakaian atas traffic yang
  sungguh-sungguh terjadi.

Kenapa combo tidak ikut:

- Combo adalah konfigurasi milik layar lain. Anggota combo yang kehilangan provider akan dicoba pada setiap request
  dan menjawab dengan penolakan tentang model yang tidak pernah disebut klien: itu persis alasan guard ini dibuat,
  dan itu tercatat di `provider_node_combo_guard.go`.
- Menghapus anggota combo diam-diam mengubah perilaku routing yang tidak diminta operator, sementara yang diminta
  operator hanyalah menghapus provider. Guard combo menyebut nama combo-nya, jadi penolakannya actionable: sunting
  combo, hapus anggotanya, lalu delete.

Urutan operasi penting, dan itu alasan guard combo dijalankan **sebelum** endpoint dihapus: kalau combo menolak,
tidak boleh ada satu baris koneksi pun yang sudah terlanjur hilang. Cascade adalah aksi menghancurkan, maka ia
harus menjadi langkah terakhir yang boleh gagal.

## Temuan

### F1 (HIGH) Delete harus menghapus, bukan menolak

`app-serv/internal/service/provider_node.go`:

- Port `EndpointCounter` (`CountEndpoints`) diganti `EndpointEraser` dengan `DeleteByProvider(ctx, providerID)
  error`. Rencana semula menulis port ini mengembalikan `(int64, error)` supaya jumlahnya tersedia untuk log
  terstruktur; itu dibatalkan sebelum kode ditulis, karena `NodeService` tidak punya logger dan DELETE menjawab
  `204` tanpa body, jadi angka itu tidak akan dibaca siapa pun. Port yang mengembalikan nilai yang tidak dipakai
  adalah port yang salah bentuk.
- `NodeService.Delete` menjadi: baca node → `rejectComboReference` → hapus endpoint milik node → hapus node →
  `invalidateOverlay`. Hitung-dan-tolak hilang dari fungsi ini.
- `deps.Counts` berganti nama menjadi `deps.Endpoints` beserta pesan validasinya ("endpoint eraser is required");
  ia bukan counter lagi.

`app-serv/internal/repository/postgres/endpoint.go`: tambah `DeleteByProvider` di sebelah `Delete`, dan hapus
`CountEndpoints` milik file itu. Yang terakhir ini hanya ada untuk memenuhi port `EndpointCounter` (interface
`repository.EndpointRepository` tidak mendeklarasikannya), jadi setelah F1 tidak ada pemakai tersisa.

Mesin penolakan yang dihapus seluruhnya, terhitung dari grep `CountEndpoints` di pohon:

| Lokasi | Bentuk | Nasib |
| --- | --- | --- |
| `service.EndpointCounter` | port sempit di package service | diganti `EndpointEraser` |
| `postgres.EndpointRepository.CountEndpoints` | implementasi, satu-satunya pemakai adalah port di atas | dihapus |
| `postgres.NodeRepository.CountEndpoints` | duplikat query yang sama, tidak pernah dipakai NodeService (wiring mengikat port ke endpoint repo) | dihapus |
| `repository.NodeRepository.CountEndpoints` | deklarasi di kontrak, alasan satu-satunya fake store harus punya method ini | dihapus |
| `readinessNodeStore.CountEndpoints` | fake, hanya ada karena kontrak di atas | dihapus |
| `readinessEndpointCounts.CountEndpoints` | fake port | diganti `readinessEndpointErasures`, yang mencatat id provider yang dihapus |

`repository.EndpointRepository` sengaja tidak ikut menambah `DeleteByProvider`: NodeService memakai port sempit
miliknya sendiri, dan menambah method ke kontrak besar berarti memaksa setiap fake endpoint repository ikut
mengimplementasikannya demi satu pemakai.

`app-serv/cmd/app-serv/management_wiring.go`: `NodeServiceDeps` sudah di-bound ke `endpointRepo` (`Counts:` hari ini),
jadi tidak ada tipe baru yang perlu dibuat, hanya nama field dan pesan validasinya yang berubah.

`app-serv/internal/domain/errors.go`: `ErrNodeInUse` tidak punya pemakai lagi setelah F1. Satu-satunya pemakai yang
tersisa adalah deklarasi itu sendiri. Mengirim pesan "refused while an endpoint references it" ke depan, lalu
menghapus alasan pengirimannya, berarti menyisakan error yang tidak bisa dihasilkan: lebih baik dihapus.

`app-serv/internal/service/provider_readiness_test.go`: `TestNodeService_DeleteReferencedNodeReturnsConflict` dan
stub `readinessEndpointCounts{}` adalah test yang mengunci perilaku lama. Test itu diubah menjadi test perilaku baru
(node dengan 2 koneksi terhapus, dan keduanya hilang dari store), bukan dibuang. Nama test ikut berubah, karena
nama yang masih berkata "ReturnsConflict" akan mengirim pembaca ke kontrak yang sudah tidak ada.

### F2 (HIGH) Dialog harus berkata benar tentang apa yang ikut terhapus

`app-ui/src/lib/components/CustomProviderCard.svelte`:

- Kalimat pengantar konfirmasi menyebut akibatnya sebelum tombol ditekan: koneksi milik provider ini dan kunci yang
  tersimpan di dalamnya ikut terhapus. Ini aksi yang tidak bisa dibatalkan dan yang menghapus kredensial, jadi
  ia tidak boleh menjadi kejutan setelah klik.
- Jumlah koneksi dibaca saat dialog dibuka, bukan diambil dari `provider.endpoint_count` pada halaman: bacaan
  halaman tidak disegarkan setiap kali koneksi bertambah, dan angka yang basi di depan aksi menghancurkan adalah
  angka yang salah. Satu query terbatas (`page_size: 1`, yang dipakai hanyalah `meta.total`) saat dialog dibuka.
  Kalau bacaan itu gagal, kalimatnya tidak menyebut angka: "its stored connections and the keys on them go with it".
  Tidak menyebut angka adalah bentuk yang jujur; menyebut angka yang mungkin basi bukan.
- Blok error `conflict` ditulis ulang: satu kalimat, kalimat server apa adanya, dan satu arah ke tempat yang bisa
  diperbaiki. Karena setelah F1 satu-satunya CONFLICT yang tersisa adalah combo, arah itu adalah daftar combo, bukan
  "remove or move its endpoints". Kata "move" hilang dari panel: produk tidak punya jalur memindah koneksi.
- `deleteError` dari server tidak boleh diapit kalimat panel yang mengatakan hal yang sama. `Still referenced.` dan
  `an endpoint still references this provider` dalam satu paragraf adalah dua kalimat untuk satu fakta.

Test panel yang mengunci teks lama ikut diubah, bukan disisakan: `app-ui/tests/components/custom-provider-card-actions.test.ts`
(test "renders the refusal that keeps a referenced node alive"), `app-ui/tests/support/provider-node-stub.ts`
(stub `referenced` yang mengembalikan CONFLICT endpoint), dan `app-ui/tests/api/provider-nodes.test.ts`. Stub
`referenced` diganti pembuat CONFLICT combo, karena itulah satu-satunya penolakan yang masih nyata.

### F3 (MEDIUM) Dua dokumen kontrak masih menuliskan aturan lama

- `docs/SPEC-API/001-SPEC-API.md` §7.4 baris route DELETE: "Remove the node; refused (`CONFLICT`) while an endpoint
  still references it". Kontrak baru: node dihapus bersama endpoint miliknya, kunci ikut lewat cascade, dan yang
  menolak adalah combo yang masih menyebut node itu. Tabel route saja tidak cukup; §7.4 punya prosanya sendiri dan
  perlu dibaca sampai selesai sebelum menulis.
- `docs/SPEC-UI/001-SPEC-UI.md` §6.3 blok "Custom provider node (landed)": "Delete asks first, and a `CONFLICT`
  answer (an endpoint still references the node) is rendered as the gateway's own sentence with the dialog still
  open". Kalimat itu berubah menjadi: konfirmasi menyebut yang ikut terhapus, dan CONFLICT yang tersisa adalah combo.
- Keduanya dapat satu entri changelog mengikuti bentuk entri yang sudah ada di tiap dokumen, tidak lebih.
- `app-serv/internal/handler/openapi.json` tidak berubah, dan itu diperiksa bukan diandaikan: summary delete adalah
  "Remove a node" (tetap benar) dan set statusnya sudah memuat `409` lewat `ManagementConflictError` karena combo juga
  sudah menolaknya sebelum perubahan ini. Yang menyebut alasan penolakan hanya prosa spesifikasi.
- `app-ui/README.md` punya satu baris yang menyatakan aturan lama, ditemukan oleh pencarian, bukan diandaikan: blok
  "The gateway's refusals are rendered as it stated them" menyebut "a node an endpoint still references (CONFLICT on
  delete)". Baris itu berubah menjadi combo, dan klaim utamanya (refusal dirender seperti gateway mengucapkannya,
  dialog tetap terbuka) tetap benar dan tidak perlu ditulis ulang.
- `README.md` akar: tidak ada baris yang menyebut penolakan ini, jadi tidak ada yang berubah di sana. Tidak
  menambahkan baris hanya supaya perubahan terasa tercatat.

### F4 (MEDIUM) Komentar yang menuliskan aturan lama menjadi komentar yang salah

Sekitar 045 F3, ini kelas "klaim perilaku yang kode tidak lakukan", hanya saja kali ini kodenya yang bergerak:

- `provider_node.go` header `@reason`: "...and a delete must refuse while an endpoint still references the node".
- `EndpointCounter` doc (port itu sendiri akan diganti, tapi kalimat "which is what makes a node delete refuse" tidak
  boleh berpindah utuh ke port baru).
- `handler/provider_node.go` `Delete` doc: "refused while an endpoint still references the node (CONFLICT, §7.4)".
- `repository/postgres/provider_node.go` dua-duanya bergerak: doc `CountEndpoints` hilang bersama methodnya, dan doc
  `Delete` berbunyi "The caller checks CountEndpoints first" sementara pemanggilnya tidak lagi memeriksa apa pun.
- `repository/endpoint.go` `Delete` doc ("removes the endpoint and, by cascade, its keys") benar dan tetap benar; ia
  menjadi alasan cascade F1 bisa diminta, jadi ia tidak disentuh.
- `CustomProviderCard.svelte` header: "the API refuses it outright while an endpoint still references the node".

Aturan 045 berlaku di sini juga: pointer menyebut deklarasi, bukan nomor baris.

### F5 (LOW) Sisa yang diketahui dan sengaja tidak dibersihkan

Baris `models_custom` dan `models_disabled` keyed by `provider_id` tetap ada setelah node hilang; tidak ada FK di
sana, jadi tidak ada cascade yang bisa diminta. Baris itu tidak terjangkau: kuncinya id node yang sudah mati dan id
node tidak dipakai ulang. Menuliskan ini di sini, bukan menghapusnya diam-diam, supaya audit berikutnya tahu bahwa
sisa ini dipilih, bukan terlewat. Kalau suatu saat prefix boleh dipakai ulang, kalimat di "Keputusan desain" di atas
salah, dan sisa ini menjadi bug sungguhan.

## Rencana verifikasi

Setiap klaim di bawah diukur, bukan dinyatakan:

1. `go build ./... && go test -race ./internal/service/ ./internal/handler/ ./internal/repository/postgres/` di
   `app-serv`. Test F1 baru berjalan di sini.
2. Test repository `DeleteByProvider` di belakang tag `integration` dengan `PANNELAI_TEST_POSTGRES_DSN`, karena yang
   dibuktikan adalah cascade sungguhan (kunci ikut hilang dari `upstream_keys`), dan itu hanya bisa dibuktikan
   terhadap PostgreSQL nyata, bukan stub.
3. Gerbang panel: `scrypts/gates/panel-check.sh` (format, lint, typecheck, unit, build lewat satu pintu). Teks F2 dan
   test F2 dibuktikan di sini.
4. `scrypts/gates/antislop.sh`, lalu `scrypts/gates/all.sh` sebagai gerbang penuh yang sama dengan yang dijalankan CI.
   Komentar F4 harus lolos, dan `all.sh` ikut membawa `contract-openapi.sh` serta `contract-drift.sh`, yang membaca
   kontrak route dari `docs/CONTRACT/openapi.json` dan dari pohon kode. Kalau dokumen kontrak menyebut alasan
   penolakan, ia ikut berubah di F3, bukan diabaikan.
5. Satu percobaan nyata terhadap `app-serv` yang berjalan di `:9090`: buat node sementara, tambahkan dua koneksi,
   `DELETE` node, lalu pastikan `provider_nodes`, `upstream_endpoints`, dan `upstream_keys` sama-sama bersih, dan
   `usage_records` tetap utuh. Nomor baris hasil query ditulis di catatan penutupan, karena klaim "cascade bekerja"
   tanpa angka adalah klaim.
6. Percobaan combo: node yang menjadi anggota combo harus tetap 409, dan pesan harus menyebut nama combo itu. Ini
   membuktikan guard yang disengaja bertahan, bukan ikut tersapu cascade.

## Hasil

### HIGH (F1, F2), ter-commit `b41f54b`

15 file, +315/−120. Yang diukur, bukan dinyatakan:

| Bukti | Hasil |
| --- | --- |
| `go build ./...`, `go vet`, `golangci-lint`, `staticcheck` dan `staticcheck -tags=integration,live` di `app-serv` | semuanya bersih, 0 issues |
| `scrypts/gates/go-headers.sh` | 1240 file Go header lengkap |
| `go test -race -count=1 ./...` di `app-serv` | exit 0, semua package ok |
| Cascade betulan, `go test -tags=integration -run TestIntegration_DeleteByProvider` | `--- PASS: TestIntegration_DeleteByProviderTakesTheEndpointsAndTheirKeys (0.88s)` |
| `svelte-check --tsgo`, `eslint`, `prettier --check .` di panel | 0 errors, 0 warnings, bersih |
| `vitest run custom-provider-card` | 13 test lolos (empat kasus delete: angka, nol, tanpa angka, dan penolakan combo) |
| `vitest run provider` | 29 file, 390 test lolos |
| Ronde nyata terhadap API | node sementara + 2 koneksi + 3 kunci → `DELETE` = `204`, ketiga tabel kembali 0 untuk id itu, dan 10 endpoint milik provider lain tidak tersentuh |

Cascade diuji pada database scratch `pannelai_schema_test` yang dibuat khusus untuk run ini dan di-drop
sesudahnya: harness integrasi menolak database yang namanya tidak memuat `test` justru karena setiap fixture
di package itu TRUNCATE, jadi menjalankannya terhadap `pannelai` milik owner akan menghancurkan datanya.

Ronde nyata memakai binary hasil build dari kode ini, dijalankan pada port 127.0.0.1:9097 agar instance yang
sudah jalan di :9090 tidak diganggu. Baris yang dibuatnya hanya node dan combo probe saja, dan keduanya
dihapus lagi oleh test itu sendiri.

**Yang tidak terbukti live.** Penolakan combo tidak sempat diukur lewat API: `POST /combos` untuk probe
menjawab 400 sebelum bagian delete dijalankan, dan bentuk body-nya tidak direproduksi pada ronde ini. Jalur
itu tetap terbukti di level service, dan test-nya sekarang ikut gagal kalau penolakan menghapus apa pun
(`assertComboBlocksDelete` memeriksa `erasures` kosong), jadi urutan guard-sebelum-erase terkunci oleh test,
bukan hanya oleh komentar. Kode guard sendiri tidak berubah pada pass ini, yang berubah hanya letaknya.

**Dua keputusan bentuk yang tercatat.** `CustomProviderCard.svelte` sekarang 263 baris, lewat dari anggaran
lunak 220 baris panel; dialognya memang bisa berdiri sendiri sebagai komponen mengikuti `ComboDeleteDialog`,
dan itu pekerjaan yang layak satu pass sendiri, bukan sisipan di commit HIGH. Dan delete ini dua statement,
bukan satu transaksi: urutannya dipilih supaya kegagalan meninggalkan node tanpa koneksi (terlihat, bisa
diulang) daripada meninggalkan kredensial yatim milik provider yang sudah mati.



- Tidak menambah route "move endpoint" atau membuat `provider_id` bisa di-patch. Yang diminta adalah menghapus
  provider, dan kata "move" dihapus dari panel karena ia menjanjikan jalur yang tidak ada, bukan karena jalurnya
  ditolak.
- Tidak menyentuh layar list provider. Delete tetap hidup di layar detail, tempat konteksnya ada.
- Tidak membersihkan `models_custom`/`models_disabled` (F5), dan tidak menyentuh riwayat pemakaian.
