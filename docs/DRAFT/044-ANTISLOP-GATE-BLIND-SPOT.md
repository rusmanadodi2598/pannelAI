# 044-ANTISLOP-GATE-BLIND-SPOT.md: Header §1.2 adalah satu-satunya tempat yang tidak dibaca gerbang panjang komentar

Dokumen kerja pass ini menutup audit antislop 006 atas pull request #2. Audit-nya sendiri ada di
`anti-slop/audit-006-2026-10-06.md` (directori itu di-ignore git, jadi keputusan dan buktinya diulang di sini).
Yang berubah di pass ini: `scrypts/gates/antislop.sh` (ukuran blok dokumen), komentar di dalamnya,
`docs/RULLES/ANTISLOP.md`, dan header komentar file Go hasil sweep. Tidak satu pun baris executable di
`app-*/**` disentuh.

Kenapa pass ini perlu ada: PR #2 menulis aturan komentar ke sebuah gerbang, lalu gerbang itu membaca dirinya sendiri
hijau. Waktu dipanggil, gerbang itu cek satu hal dan mengklaim hal lain.

| | |
| --- | --- |
| **Status** | **CLOSED. HIGH (F1-F3), MEDIUM (F4-F6), LOW (F7-F8) selesai.** F8 ikut tertutup bareng F4. Bukti di §Hasil |
| **Mechanism** | AFTER (audit) lalu DURING untuk tulisan baru |
| **Scope** | `scrypts/gates/antislop.sh` (aturan baru + angka yang dikoreksi), `scrypts/gates/all.sh` (label), `docs/RULLES/ANTISLOP.md` §2.1, §2.6, §2.7 baru, §4, §6, dan header komentar 1194 file Go. Nol baris executable berubah |
| **Sumber temuan** | PR #2 `fix/anti-slop-audit-003-remediation`, head `f8067e0`, 23 commit, 1325 file berubah |
| **Struktural (AGENTS.md §1.9)** | N/A, no structural change: tidak ada domain boundary, data structure, service interaction, atau async topology yang bergerak |

## Yang sudah bersih, dan itu bukan pekerjaan kecil

Gerbang jalan sendiri atas diff PR #2 (`GATES_BASE_REF=main`) dan hijau semua. Angka yang diukur, bukan sampling:

| Cek | Cakupan | Hasil |
| --- | --- | --- |
| R-02 dash, tree-wide | semua text file tracked | 1 baris, dan itu marker `antislop:ignore` di ANTISLOP.md yang menyebut karakter terlarang |
| Separator, banner ALL CAPS, end marker, emoji, artefak substitusi | tree-wide | 0 |
| Suppressing tanpa alasan (AGENTS.md §1.4) | semua file Go | 0 |
| 4900 baris komentar yang ditambahkan PR | scan pola antislop-code | 0 empty label, 0 narasi langkah, 0 TODO kabur, 0 signature echo |
| 461 baris komentar non-Go (ts, svelte, sql, sh, md) | scan yang sama | bersih |

Sweep-nya kerja. Sisanya kelas panjang dan kelas akurasi, dan keduanya tinggal tepat di tempat yang tidak dibaca
gerbang.

## Temuan

### F1 (HIGH) Aturan panjang didokumentasikan sebagai enforced, padahal header tidak pernah diukur

ANTISLOP.md §2 menulis kontraknya: aturan tanpa cek tidak boleh diklaim punya cek. §2.6 lalu mengklaim "The gate
fails a Go doc block over 16 lines and warns past 10, on changed files."

Klaim itu benar untuk blok di atas `func`/`type` dan salah untuk header file, yaitu tempat PR ini menaruh prosa paling
banyak. `scrypts/gates/antislop.sh:201-210` menghitung baris komentar dan baru membaca hitungannya ketika menemukan
`^(func|type)`. Header berhenti di `package`, yang jatuh ke cabang reset, jadi hitungan dibuang tanpa pernah dibaca.

Terukur di 1224 file Go diff PR: 1207 header lewat 8 baris, terpanjang 41 baris
(`app-serv/internal/service/usage_event_consume.go`, `package` di baris 42).

Keputusan awal: gerbang ikut mengukur blok header dengan ambang §2.6. Setelah F2 jalan, ambang itu terbukti metrik
yang salah, dan yang ditegakkan sebagai gantinya adalah bentuk field: satu nilai §1.2 satu baris, tree-wide, FAIL.
Alasannya dan bentuk aturannya ada di §Hasil F1 dan ANTISLOP.md §2.7. Tidak ada pengecualian baru.

### F2 (HIGH) Nilai field §1.2 yang di-wrap ditulis ulang gofmt jadi bentuk yang merusak konvensi header

AGENTS.md §1.2 ada supaya authorship, purpose, dan layer "explicit and grep-able". Field-nya fixed-width satu baris.
Isinya prosa yang cukup panjang untuk di-wrap, lalu gofmt menulis ulang wrap itu:

```go
// @for       The quota flush worker's lifecycle: one bounded goroutine, a
//
//	single-flight guard, and the composition-root entry point.
```

Itu `app-serv/internal/service/quota_flush.go:4-6`, dan `gofmt -d` atas bentuk satu baris menghasilkan persis
pola itu: gofmt membaca sambungan ber-indent sebagai code block lalu memberi baris kosong di sekitarnya.
`log_retention.go:4-6` dan `:10-14`, `usage_event_consume.go:4-6` dan `:8-10` peristiwa yang sama.

Dua sifat membuat ini murah: tiga file itu `gofmt -l` bersih, jadi bentuk rusak yang canonical dan tidak ada alat yang
protes; dan satu baris panjang stabil, karena gofmt tidak mem-wrap prosa komentar (dibuktikan `quota_published_policy.go:4`
dengan `@for` 150 karakter). Jadi: satu field satu baris.

Cakupan tree-wide, bukan hanya diff: **1165 dari 1245** file Go punya sambungan ber-indent di header, 2206 blok, 5671
baris `//` kosong di dalam header. Dua bentuk lain (sambungan biasa setelah baris kosong, dan sambungan biasa tepat di
bawah tag) menambah perhitungannya, jadi total yang dibawa ke satu baris per field: **1194 file, 5788 sambungan**.

Batas sweep: hanya sambungan yang menempel pada baris `// @<tag>` dari delapan field §1.2. Prosa bebas di antara tag
(tabel retry/dead-letter/termination milik F4) tidak ikut digabung, karena isinya bukan nilai field dan F4 belum
disetujui.

### F3 (HIGH) Komentar dan dokumen aturan menyebut angka yang dibatalkan PR ini sendiri

Komentar yang menyatakan fakta salah lebih buruk dari komentar yang tidak menyatakan apa pun.

| Tempat | Yang ditulis | Yang ada di tree |
| --- | --- | --- |
| `scrypts/gates/antislop.sh:19-21` (ditambahkan PR ini) | "170 of them are legitimate references inside the §1.2 @reason headers" | 148 saat audit dihitung, 146 setelah sapuan field |
| ANTISLOP.md §4 dan §6 | "the tree carries 142 of them" di body comment | 6, dan semuanya string `t.Fatalf`/`t.Errorf`, bukan komentar |
| ANTISLOP.md §6 | 821 baris dash di 27 file markdown, 50 di AGENTS.md, dan "The gate prints the current count on every run" | 0 baris dash, dan gerbang tidak mencetak hitungan apa pun |

Angkanya basi karena branch ini terus bergerak setelah dokumen ditulis: `cb43461` menghapus 136 sitasi body dari 96
file dan sapuan markdown menghabiskan sisa dash, tapi kalimat yang menyebut hitungan lama tidak ikut ditulis ulang.
§6 yang paling berbahaya: ia menyuruh pembaca mengira `docs/DRAFT/**` dan AGENTS.md masih membawa karakter terlarang
dan dikecualikan, padahal R-02 sekarang tree-wide dan lolos.

Keputusan: angka dinyatakan ulang dari tree saat ini, dan bagian §6 yang sudah lunas dicabut, bukan dibiarkan berdiri
sebagai utang.

### F4 (MEDIUM) Fakta worker §1.6 ditulis sebagai prosa bebas di dalam header §1.2, dalam dua bentuk

`quota_published_policy.go:8-26` (19 baris), `log_retention.go:16-26`, `quota_flush.go:20-23` menaruh retry policy,
dead-letter policy, dan termination setelah `@reason` dan sebelum `@author`. `go-headers.sh` hanya mengecek delapan tag
ada tepat satu kali, jadi struktur bebas di antaranya lolos. Isinya sah dan harus tinggal; yang belum diputuskan
adalah bentuknya, dan tiga file sudah memakai dua bentuk berbeda. Tidak disentuh di pass ini.

### F5 (MEDIUM) Tujuh blok inline lewat batas yang ditulis proyek sendiri

Dari 656 run komentar 3 baris atau lebih di body, tujuh lewat "about eight lines" §2.6:
`published_quota.go:55-67` (13), `combo_order.go:83-95` (13), `outcome.go:54-64` (11), `custom_node.go:44-53` (10),
`usage.go:74-83` (10), `transport_call.go:49-57` (9), `model_catalog_writes.go:45-53` (9). PR ini menambah 1 sampai 5
baris komentar di masing-masing, jadi sebagian besar warisan. Yang dipotong nanti rantainya ("because", kalimat
insiden), tidak faktanya. Tidak disentuh di pass ini.

### F6 (MEDIUM) Cek sitasi membaca string test sebagai komentar

`go_body` mem-grep semua di bawah baris `package` tanpa memisahkan komentar, jadi `CITATION_RE` ikut menangkap literal
string. Lima file yang diperingatkan gerbang atas range PR ini semuanya `t.Fatalf`/`t.Errorf`:
`egress_guard_assert_test.go:96` dan `:126`, `provider_wiring_guard_test.go:90`, `quotafetch_wiring_guard_test.go:71`,
`chat_refusal_record_test.go:149`, `provider_validate_plan_test.go:146`. Utang yang seharusnya dilacak peringatan ini
berarti 0. Tidak disentuh di pass ini; F1 punya alasan supaya peringatan ini bisa dinaikkan jadi sinyal nyata.

### F7 (LOW) Doc comment yang menggema setengah

`app-serv/internal/handler/chat.go` `NewChatHandler validates deps and returns the handler.` (paruh kedua mengulang
signature), `model_catalog_writes.go` `Aliases returns the whole alias set.`, `vision_rotation.go` `visionRotationKey
derives the Redis key the adapter's rotation occupies.` Tidak disentuh di pass ini.

### F8 (LOW) Koma dipakai untuk pekerjaan yang diatur §2.1 ke titik dua

`quota_published_policy.go:8`, `:17`, `:25`: `Retry policy, ...`, `Dead-letter policy, ...`, `Termination, ...`. Sisa
sapuan dash yang bertemu heading. Titik dalam blok F4, tidak disentuh di pass ini.

## Rencana verifikasi

Setiap angka di atas dihitung ulang setelah perubahan, bukan dipercaya dari rencana:

1. `gofmt -l` atas seluruh file yang disapu: harus kosong, artinya hasil sweep adalah bentuk kanonik gofmt.
2. `scrypts/gates/go-headers.sh`: delapan tag utuh di 1245 file, 0 pelanggaran.
3. `scrypts/gates/antislop.sh` dengan `GATES_BASE_REF=main`: harus hijau dengan cek header yang baru.
4. `go build ./...` dan `go vet ./...` di `app-serv`: pembuktian bahwa yang berubah hanya komentar.
5. Diff terverifikasi lewat `go/parser` dengan komentar dibuang: tidak ada satu pun baris executable yang berubah.
6. Kode yang sama dijalankan sebelum dan sesudah untuk membuktikan angka F3 dan ambang F1.

## Hasil

### F2 selesai: 1194 file header disapu, satu algoritma

Sweep jalan sebagai satu transformasi deterministik, bukan beberapa pass yang ditumpuk. Nilai field §1.2 diserap sampai
tag berikutnya, tiga bentuk sekaligus: blok ber-indent yang dihasilkan gofmt, baris biasa setelah baris `//` kosong, dan
baris biasa tepat di bawah tag.

**1194 file, 5788 sambungan digabung.** Hasilnya idempotent: dijalankan ulang menghasilkan 0 perubahan.

Sebuah fragmen hanya melanjutkan field kalau field itu belum menutup kalimatnya, atau fragmennya dibuka dengan huruf
kecil. Tiga hal yang muncul karena implementasinya diuji, bukan dipercaya:

- Dua pass pertama saling melewatkan satu bentuk. `quota_published_test.go` punya blok tab yang rantainya melewati
  sebuah sambungan biasa, jadi blok itu tinggal. Aturan baru di F1 menemukannya, lalu seluruhnya dihitung ulang dari
  HEAD dengan satu algoritma.
- Bentuk ketiga (sambungan tanpa baris pemisah) baru terlihat setelah aturan itu jalan: `playground_live_doubles_test.go`
  punya `@uses` yang lanjut ke dua baris biasa tanpa `//` kosong di antaranya. 55 baris di 42 file.
- Versi yang terlalu rakus menghancurkan konten. Menyerap blok ber-indent tanpa syarat menelan contoh `go test` yang
  memang di-indent di `playground_live_flush_test.go` dan `systemone_live_test.go`, dan mengubah teks yang bisa
  di-copy-paste jadi satu baris prose. Sekarang blok ber-indent setelah kalimat yang selesai adalah struktur, bukan
  sambungan field: 158 header menyimpan contoh yang runnable persis karena batas itu.

Sapuan akhir juga harus menjalankan `gofmt -w` di 20 file: begitu sambungan field diserap, blok contoh di bawahnya jadi
blok sendiri dan gofmt menormalkan indentasi dasarnya. Bukan efek samping yang mengejutkan, itu bentuk kanoniknya.

Artefak koma: menggabungkan baris yang putus tepat sebelum koma menulis `credential , which`, yang dilarang §2.1. Ada
5 tempat. Fragmen yang dibuka dengan koma, titik-koma, atau kurung kini menempel ke kata sebelumnya tanpa spasi. Tiga
di antaranya sebelumnya tidak terlihat gerbang justru karena terbelah baris: penyapuan ini menyingkapnya, bukan
menyembunyikannya.

### F1 selesai: yang diukur adalah bentuk field, bukan panjang header

Panjang header total ternyata metrik yang salah. Setelah F2, satu header yang benar adalah 8 baris field ditambah
kalimat `package`; 744 file jatuh ke 9-10 baris dan 493 ke 11-16 tanpa satu pun berisi prosa yang mengarang. Yang
tersisa di atas 16 baris 65 file, dan bukan karena prose yang mengarang: 15 di antaranya blok prosa F4, sisanya contoh
perintah ber-indent yang memang sengaja disimpan.

Jadi yang ditegakkan adalah bentuk yang §1.2 janjikan: **satu nilai field satu baris**, tree-wide, FAIL.
`scrypts/gates/antislop.sh` dapat fungsi `wrapped_field_hits` (mesin state awk di wilayah atas `package`) dan aturan itu
tersedia sebagai ANTISLOP.md §2.7. §2.6 ditulis ulang supaya tidak lagi mengklaim cakupan yang tidak dimiliki.

Bukti bahwa aturan itu menggigit, bukan hanya lewat: tiga bentuk pelanggaran disuntikkan ke
`internal/domain/quota.go` dan ketiganya muncul sebagai FAIL (`:7` blok tab, `:10` sambungan biasa setelah baris kosong,
`:12` sambungan biasa tepat di bawah tag), lalu di-restore. Restore itu sendiri jadi bukti kedua: file kembali ke versi
belum-disapu dan aturan baru langsung menandainya, termasuk bentuk ketiga yang sapuan pertama belum lihat.

Satu masalah desain muncul setelah itu: dua implementasi dari aturan yang sama, Python untuk sapuan dan awk untuk
penegakan, berbeda pendapat soal cara memotong fragmen. awk membuang spasi setelah tab, Python tidak, jadi contoh
perintah `  go test ...` terlihat seperti sambungan bagi gate dan bukan bagi sapuan. Yang diperbaiki bukan salah satu
pihak secara diam-diam: potong fragmen disamakan (tab saja, tanpa spasi), dan tree dinyatakan konvergen ke aturan gate
itu sendiri, bukan ke keluaran skrip. Setelah itu `wrapped_field_hits` melaporkan 0 dan contoh perintah di 158 header
tetak utuh.

### F3 selesai: angka dinyatakan ulang dari tree

| Tempat | Sebelum | Sesudah |
| --- | --- | --- |
| `antislop.sh` komentar cakupan | "170 of them" | 146 sitasi di region header, dihitung ulang dari tree setelah sapuan selesai |
| `antislop.sh` komentar cakupan | Sitasi body dianggap utang | Nol di komentar; yang tersisa string `t.Fatalf`, dan itu F6 |
| ANTISLOP.md §2.1 | R-02 "enforced on changed files", docs dan AGENTS.md dikecualikan | Tree-wide, gagal di semua penggunaan; 0 baris dash di seluruh tree |
| ANTISLOP.md §4 | "142 of them", "701 citations" | 146 header, 0 body-comment |
| ANTISLOP.md §6 | 3 butir utang yang sudah lunas, dan klaim "gerbang mencetak hitungan setiap run" | Utang lunas dipindah ke daftar closed dengan caranya ditutup; yang open dinamai satu per satu; klaim mencetak hitungan dihapus karena memang tidak ada |

AGENTS.md ditinjau dan tidak diubah: §1.2 sudah menuliskan field sebagai satu baris, jadi yang kurang adalah penegakan,
bukan aturannya.

### Bukti

| Gerbang / perintah | Hasil |
| --- | --- |
| `go/parser`, komentar dibuang, 1194 file vs HEAD | identical code: 1194, code changed: 0, unreadable: 0 |
| `gofmt -l` atas 1245 file Go | kosong: hasil sweep plus `gofmt -w` adalah bentuk kanonik |
| `scrypts/gates/antislop.sh` (`GATES_BASE_REF=main`) | PASS semua, termasuk aturan field tiga bentuk; 5 peringatan sitasi tidak berubah (F6) |
| `scrypts/gates/all.sh` | `all gates passed`, 8 gerbang: go lint, go headers, antislop, go test race, panel checks, secrets, contract drift, contract artifact |
| `scrypts/gates/go-headers.sh` | PASS, 1234 file header lengkap |
| `scrypts/gates/go-lint.sh` | gofmt, go vet, staticcheck (termasuk `-tags=integration,live`), golangci-lint 0 issues |
| `go build ./...`, `go vet ./...` di app-serv | rc=0 keduanya |
| `go test -race ./...` di app-serv | exit 0: 20 paket ok, 0 FAIL, 0 panic, 4 paket tanpa file test |
| Distribusi panjang header sesudah | <=8: 0, 9-10: 696, 11-16: 484, >16: 65 (15 prose F4 + 158 contoh ber-indent, semuanya struktur yang disengaja) |
| Site yang dianggap sambungan oleh gate | 0 di seluruh tree |

### F4 selesai: prosa keluar dari header, dan F8 ikut tertutup

Aturannya dulu, baru isinya. `wrapped_field_hits` jadi `header_shape_hits`: selain nilai field yang menyambung, kini
**paragraf prosa di antara dua field** juga FAIL tree-wide. Header adalah delapan field itu dan bukan yang lain.

Empat file worker membawa pernyataan §1.6 (retry, dead-letter, termination) di antara tag:

- `quota_published_policy.go`: tiga blok pindah ke body file, tepat di samping angka yang mereka namai, dengan satu
  kalimat rationale yang ternyata sudah ada di doc `publishedPollIntervals` dipangkas agar tidak dobel.
- `quota_flush.go`, `quota_flush_policy.go`, `log_retention.go`: bloknya **dihapus**, bukan dipindah. Doc `Run`,
  doc tipe `QuotaFlushPolicy`, dan doc `recordFailure` sudah menyatakan fakta yang sama; blok header adalah duplikasi.
  Satu fakta memang belum ada di mana pun (MaxAttempts membatasi kegagalan beruntun lalu mereset pencacah) dan itu
  masuk ke doc `recordFailure`, tempat `go doc` menampilkannya.
- Keempat header sekarang tepat 10 baris, 8 field.

Lima file lain punya kasus berbeda: paragraf kedua `@reason` yang menggantung di tengah header. Empat di antaranya
terbawa oleh perbaikan definisi, yaitu bahwa **titik dua tidak menutup kalimat**; ia memperkenalkan baris sesudahnya,
jadi nilai field yang putus setelah `stop_reason:` kini ikut terserap. Satu file digabung tangan, dan penggabungan itu
menyingkap artefak nyata: `anything already recorded ,` di `published_quota_rollback_test.go`, koma yang selama ini
tersembunyi karena gerbang membaca pola `kata , kata` per baris dan barisnya terbelah tepat di situ.

F8 ikut selesai karena pemindahan yang sama: `Retry policy,` / `Dead-letter policy,` / `Termination,` jadi titik dua.
Sisa tiga tembakan grep pola itu (`RetryAfter, when positive,` dan dua lainnya) adalah appositive biasa, bukan heading.

### F5 selesai, dengan hasil yang tidak seragam dan itu benar

Tujuh blok inline lewat batas delapan baris. Yang dipotong: rantai argumen, kalimat yang menyatakan ulang fakta
sebelahnya, dan satu cerita insiden. Tidak ada fakta yang dipadatkan jadi lebih sedikit kata.

| Lokasi | Sebelum | Sesudah |
| --- | --- | --- |
| `service/combo_order.go` | 13 | 9 |
| `repository/published_quota.go` | 13 | 12 |
| `dataplane/outcome.go` | 11 | 9 |
| `registry/custom_node.go` | 10 | 6 |
| `repository/usage.go` | 10 | 9 |
| `service/model_catalog_writes.go` | 9 | 7 |
| `dataplane/transport_call.go` | 9 | 9 |

Dua blok terakhir tetap lewat batas dan itu bukan kegagalan pass ini: `published_quota.go` menyatakan sembilan fakta
berbeda (apa yang distempel, baris dibuat kalau belum ada, delta nol, delta negatif ditolak, kalimat provider, ke baris
mana kalimat itu disimpan, window row tidak disentuh, nomor basi mempertahankan stamp-nya) dan `transport_call.go`
empat. §2.6 bilang blok yang memang memegang lima batasan mengambil lima baris; memaksa keduanya ke delapan berarti
membuang fakta, yaitu kegagalan yang paling dokumen ini peringatkan. Yang hilang dari `outcome.go` justru yang seharusnya:
cerita "model adapter buta melayani gambar merah sebagai abu-abu selama satu combo penuh", riwayat insiden, bukan
batasan bagi pembaca field itu.

Belum ada cek yang mengukur panjang blok di dalam body; itu §3 material dan pass ini tidak melebarkannya diam-diam.

### F6 selesai: cek sitasi hanya membaca komentar

`go_body_comments` mengambil bagian komentar dari sebuah baris dan membuang `//` yang terletak di dalam string (paritas
tanda kutip). `go_body` ikut dihapus karena tidak ada lagi yang memanggilnya. Dibuktikan dua arah: komentar berisi
`draft 099 F1` dilaporkan, `t.Errorf` berisi `draft 017 §4.6` tidak. Lima peringatan yang masih tercetak sepanjang pass
HIGH sekarang hilang, dan itu syarat supaya peringatan ini layak dinaikkan jadi FAIL suatu hari nanti.

### Satu kesalahan alat, bukan pohon

Jalankan pertama MEDIUM melaporkan `code changed: 4`. Yang salah verifier saya, bukan tree: `cmcheck` membersihkan Doc
pada file, func, type, value dan import, tapi tidak pada `*ast.Field`, jadi komentar anggota struct dan method interface
ikut tercetak dan setiap edit komentar field terlihat seperti perubahan kode. Setelah diperbaiki, sapuan yang sudah
ter-commit diverifikasi ulang dengan alat yang benar: 1194 file, 0 kode berubah. Bug itu cuma bisa laporan palsu
"berubah", tidak sebaliknya, jadi klaim HIGH tetap berdiri; sekarang ia terbukti, bukan disimpulkan.

### F7 selesai, dan temuan aslinya bukan gema

Audit 006 mencatat F7 sebagai lima doc comment yang menggema setengah signature, prioritas LOW. Waktu digarap, yang
ketemu kelas lain: `NewXHandler validates deps and returns the handler` beredar di 56 file, dan untuk 26 di antaranya
kalimat itu **salah**. Konstructor-konstructor itu cuma `return &T{dep: dep}` dan tidak mengembalikan error; yang
memeriksa deps adalah saudara mereka di layer service (`NewEngine`, `NewSelector`, `NewTransport`, semuanya `error)`
dan nil-check). Doc line diwariskan antar keluarga file, dan yang dibaca reader adalah klaim perilaku yang tidak ada.

Komentar ini lebih buruk dari gema: gema hanya boros satu baris, klaim palsu mengirim orang ke tempat yang salah waktu
debug. Tidak ada linter yang menangkapnya; statik check tidak membandingkan kalimat dengan body.

Yang dilakukan, comment-only semuanya:

- 22 file: baris doc dihapus. Tidak ada fakta tersisa untuk disimpan; doc route di bawahnya sudah membawa faktualnya.
- 4 handler (`embeddings.go`, `media.go`, `systemone.go`, `token_count.go`) dan `NewProxyRouteService`: kalimat pertamanya
  dibuang, kalimat keduanya justru yang berharga (authenticator §4 dipakai bareng supaya dua route tidak mulai
  berbeda pendapat soal key mana yang valid; clock default ke `time.Now`). Doc sekarang mulai dari fakta itu.
- 5 gema diganti constraint yang tidak terlihat di signature: `Hash` jalan di bcrypt default cost yang tidak bisa
  diubah lewat method ini; `Aliases` tanpa filter dan tanpa page; `visionRotationKey` cuma fungsi dari konstanta
  prefix, jadi semua proses dan semua request menghormati satu slot rotasi yang sama; `NewHealthService` "returns a
  ready service" dihapus (ready itu padding).

Dibuktikan, bukan dinyatakan: skrip klasifikasi membandingkan klaim di doc dengan body-nya (signature punya `error)`?
ada nil-check?) dan melaporkan 29 klaim benar / 26 salah / 1 mixed sebelum, dan 0 salah sesudah. `staticcheck`,
`golangci-lint`, dan `go-headers` tetap hijau tanpa doc line itu, jadi tidak ada aturan yang menuntut baris kosong
tersebut.

§3 ANTISLOP.md sekarang menuliskan kelasnya sebagai aturan review: komentar tidak boleh mengklaim perilaku yang kode
tidak lakukan, dan ini sengaja tidak ditulis sebagai aturan yang di-enforce, karena tidak ada cek untuk itu.

