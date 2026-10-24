# 052-FAILURE-CLASSIFICATION-AND-REST-PROVENANCE.md: satu status 429 menyimpan dua kegagalan, dan park tidak bisa membedakan keduanya

Dokumen ini mencatat patch yang sudah berjalan di working tree (klasifikasi kegagalan upstream + asal-usul
"rest" sebuah park), koreksi yang ditemukan saat patch itu diselesaikan, dan keputusan yang dibuat di
dalamnya. Tidak seperti 050, dokumen ini bukan register temuan: ia ditulis SETELAH kodenya ada, supaya
reviewer bisa menguji bentuk yang terpilih terhadap alasan yang menahannya, dan supaya sisanya yang belum
terbukti dinyatakan sebagai belum terbukti, bukan sebagai selesai.

## Status dan lingkup

| | |
| --- | --- |
| **Status** | **Landed di working tree, belum di-commit.** Build hijau, suite hermetik hijau, `go-headers.sh` hijau (1371 berkas). Yang MASIH BELUM dijalankan: suite `integration` (butuh `PANNELAI_TEST_POSTGRES_DSN`), jadi migrasi 000021 dan proyeksi kolom baru belum dibuktikan terhadap PostgreSQL nyata pada pass ini; dan tidak ada bukti live dari vendor: parser reset TH Harbor dibangun dari kalimat yang terlihat di produksi, bukan dari satu panggilan yang diulang hari ini. |
| **Cakupan** | `internal/domain/` (`failure_class*.go`, `failure_rest.go`, `key_failure_class.go`, `settings_routing*.go`, `upstream_key*.go`), `internal/provider/` (`failure_signature.go`, `failure_reset.go`, `plugin.go`), `internal/dataplane/` (`transport_call.go`, `transport_attempt.go`, `upstream.go`, `engine_failure.go`, `engine_relay.go`, `selection_health.go`, `outcome.go`), `internal/schema/`, `internal/repository/postgres/`, `internal/handler/`, `migrations/000021_*` |
| **Tidak termasuk** | Editor panel untuk `failure_classes` dan kolom provenance di tabel kunci (dikerjakan terpisah, bentuk wire-nya sudah final di sini); SPEC-API §7.5 belum dibaca ulang untuk menamai kelas kegagalan di dokumen kontrak |
| **Struktural (AGENTS.md §1.9)** | **Bukan N/A.** `SYSTEM_MAP.md` §3.5c menambah lapisan baru di walk kredensial dan §4a mencatat `000021`. Kontrak OpenAPI ikut berubah (`routing.failure_classes`, tiga field pada respons kunci), jadi `internal/handler/openapi.json` diregenerasi pada pass yang sama |
| **Sumber bukti** | Baca + grep terhadap kode, `go test -race ./internal/...`, dan gate repo. Satu angka di dokumen ini berasal dari produksi: empat dari enam kunci TH Harbor dipark lima menit melawan reset yang pernyataan vendor sendiri letakkan empat hari ke depan |

## Masalahnya

`FailureClass(status)` hanya tahu status HTTP. 429 selalu berarti "throttle", dan throttle dipark dengan ceiling
lima menit. Tapi 429 di TH Harbor adalah dua kalimat berbeda: `"under heavy load"` (lift sendiri dalam
menit) dan `"You've used this period's free allowance. Your next period starts on 19 Oct 2026 at 10:48
UTC"` (tidak lift sampai tanggal itu). Yang kedua dipark lima menit, masuk kembali ke rotasi, ditolak lagi,
dan menghabiskan satu leg walk untuk setiap siklus; empat hari penuh penolangan yang dibayar dengan request
klien nyata. Kelas yang sama juga tidak bisa dibedakan dari status: 401 "credential mati" dan 502 "free tier
hanya boleh dari client OpenCode" bukan kegagalan kredensial, tapi keduanya memarkir kunci.

## Bentuk yang dipilih

**Sembilan kelas kegagalan** (`domain/failure_class.go`) hidup di atas kelas park, bukan menggantikannya:
`KeyFailureClass` tetap menentukan anak tangga ladder, `FailureClass` menentukan apa yang sebenarnya terjadi.
Mapping satu arah (`KeyClass()`), karena ladder sudah punya bentuk yang benar dan yang salah adalah jawabannya
atas pertanyaan "siapa yang bersalah".

**Recognition tinggal di `internal/provider`.** Domain memiliki vocabulary; connector memiliki pola. Satu
`Classify(Failure) → FailureVerdict` menjawab kelas, reset, sumber, signature, dan aksi. Empat signature
di-seed (`tokenharbor.heavy-load`, `tokenharbor.free-allowance`, `qoder.envelope.quota`,
`opencode.free-tier-gate`) dan dua `genericResets` berjalan di atas body apa pun, supaya vendor yang belum
di-seed tetap menyerahkan tanggalnya sendiri. Registry bisa di-tambah saat boot (`RegisterSignature`) dan
signature terakhir yang daftar menang, itu jalur untuk vendor baru tanpa menyentuh core.

**Satu aturan preseden park** (`KeyHealthPolicy.RestUntil`):

```
COALESCE(reset yang dinyatakan vendor,
         now + clamp(Retry-After, ceiling kelas),
         now + cooldown kelas)
```

Reset vendor menang atas apa pun yang bisa dihitung gateway; ceiling hanya menjaga dari tanggal yang tidak
mungkin masuk (kapan pun sebuah tanggal bisa mengubur kunci, ia salah baca). Retry-After tetap memakai anak
tangga pertama ladder sebagai floor, seperti sebelumnya. Ladder itu sendiri TIDAK di-clamp ulang: jendela yang
operator konfigurasi adalah keputusan, dan ceiling kelas tidak boleh membatalkannya.

**Quota punya ladder sendiri, bukan doubling.** Empat langkah: 15 menit → 1 jam → 6 jam → 24 jam. Doubling dari
15 menit butuh delapan strike untuk mencapai satu hari, dan setiap strike adalah satu leg walk yang sudah
diketahui tidak bisa dilayani. Run quota menaikkan `consecutive_billing` (kolom yang sama), karena bagi akun
keduanya kondisi yang sama; kolom ketiga untuk kondisi yang tidak berbeda adalah biaya tanpa keuntungan.

**Aksi kelas sekarang menggerakkan walk, bukan cuma panjang park.** `park`/`rotate`/`stop` dibaca
`keepsWalking()` di `engine_relay.go`: `stop` mengakhiri rantai dan mengembalikan error klien, `rotate` berjalan
terus tanpa memarkir, `park` mengikuti aturan lama (`failoverWorthy`). Walk level combo (`engine.go`) ikut
menghormati `stop` lewat `Outcome.ChainStopped`, karena alasan "credential lain tidak akan menjawab ini berbeda"
berlaku juga untuk anggota combo berikutnya. Default per kelas park dipertahankan persis seperti perilaku lama
(`request` → stop, `target`/`gate` → rotate, sisanya → park), jadi tabel konfigurasi tidak mengubah satu pun
request yang hari ini sudah benar.

**Provenance ikut tersimpan dan dibaca.** `reset_source` (`header`/`body`/`default`) dan `signature_id` di
`upstream_keys` adalah jawaban atas "kenapa selama ini": keduanya ditulis oleh health write, tidak dibaca oleh
router, dan sekarang dibaca kembali lewat proyeksi kunci dan muncul di API kunci bersama `last_error_class`
yang selama ini tidak pernah sampai ke klien sama sekali.

## Koreksi yang ditemukan saat patch ini diselesaikan

Semua ini adalah kode yang sudah ada di working tree dan sudah diubah, bukan hipotesis:

1. **`Signature.Retryable` dan `Signature.Policy` tidak pernah dipakai.** verdict menghasilkan keduanya,
   `UpstreamError.Rest()` tidak mengisinya, dan transport membaca `failure.Class.Retryable()`. Satu signature
   yang menyatakan "ini bisa dicoba lagi" tidak mengubah apa pun. Keduanya sekarang menempel di error dan
   menjadi sumber keputusan retry maupun park; transport membaca jawaban verdict, bukan tebakan ulang atas kelas.
2. **`RetryAfter` dihitung dua kali di dua lapis.** Classifier sudah membacanya dari header; `Rest()` membaca
   header yang sekali lagi. Angka yang sama sekarang di-stamp sekali.
3. **`reset_source` berbohong.** Untuk signature yang cocok tanpa reset di body dan tanpa Retry-After, sumber
   tertinggal di `body`, jadi baris di database menyatakan vendor menjamin sebuah wait yang sebenarnya gateway
   pilih sendiri. Sumber sekarang hanya dinamai di tempat sebuah pernyataan benar-benar ditemukan.
4. **Satu blok `if` yang tidak melakukan apa pun** (`failure.RetryAfter > 0 && ResetAt.IsZero()` menugaskan ulang
   nilai yang sudah ditugaskan) dihapus.
5. **Group-level `failure_classes` bisa ditulis tapi tidak pernah dibaca.** PATCH menerimanya, GET tidak
   merendernya, jadi panel tidak dapat membaca kembali apa yang baru ia simpan. Sekarang dirender, dan hanya
   entry yang benar-benar tersimpan, bukan sembilan default.
6. **Dua kolom write-only.** `reset_source`/`signature_id` tidak ada di proyeksi baca mana pun dan tidak muncul
   di API, padahal seluruh alasan migrasinya adalah operator yang membacanya.
7. **Kontrak OpenAPI bertentangan dengan kode.** `ProviderStrategyPatch`/`Response` memakai
   `additionalProperties: false` tanpa `failure_classes`; field baru ditambahkan dan `openapi.json` diregenerasi.
8. **`IsQuotaError` menjadi dead code** begitu transport memutuskan retry dari kelas. Method interface,
   implementasi `Base`, testnya, dan baris di README dihapus; sinyal quota yang sebenarnya (`StreamFailure.Quota`
   dari envelope Qoder) sudah lewat classifier.

## Keputusan yang perlu ditantang reviewer

- **Tiga kelas ditolak di settings.** `TARGET_UNREACHABLE`, `GATE_BLOCKED`, `REQUEST_INVALID` tidak boleh punya
  entry `failure_classes`: router tidak membaca budget untuk kelas yang tidak memarkir kunci, jadi entry itu
  konfigurasi mati. Untuk kelas-kelas ini aksi bisa datang dari signature (code), bukan dari operator. Kalau
  owner ingin operator bisa menghentikan rantai pada kegagalan jaringan, yang perlu diubah adalah *moment baca
  settings*, bukan validasinya, dan itu menambah satu `Load()` settings per kegagalan jaringan, yang hari ini
  sengaja tidak dibayar (test `a network failure parks nothing and reads no budget` menjepitnya).
- **`request` tetap stop, bukan rotate.** `defaultActionFor` awalnya mengembalikan rotate untuk tiga kelas
  non-park; itu akan mengubah perilaku 400 menjadi "coba semua kredensial". Stop dipertahankan dan rotate tinggal
  untuk target/gate.
- **Aksi dibaca sekali, di health write.** `RecordFailure` mengembalikan `(action, error)` supaya park dan walk
  menjawab pertanyaan yang sama dari satu pembacaan policy. `Engine.RecordFailure` yang exported membuang
  aksi itu; hanya walk yang membacanya.
- **Satu pembacaan settings per kegagalan yang memarkir.** Tidak di-hoist ke atas: kegagalan target dan seleksi
  virtual harus tetap gratis, seperti sebelumnya.

## Yang tersisa

| | |
| --- | --- |
| **Bukti nyata** | Jalankan suite `integration` dengan DSN test (migrasi 000021, proyeksi kolom baru, round-trip provenance), lalu satu panggilan live ke TH Harbor yang menghasilkan `free allowance` untuk membuktikan `ResetParser` membaca tanggal vendor hari ini, bukan tanggal yang ditulis dari salinan body |
| **Panel** | Editor `failure_classes` dan kolom provenance di `EndpointKeysTable` (bentuk wire sudah final di dokumen ini) |
| **Spesifikasi** | SPEC-API §7.5 belum menamai sembilan kelas ini; kontrak API v1 sudah, SPEC belum |
| **Gate** | `staticcheck` sudah gagal di HEAD sebelum patch ini (`provider/opencode.go:186` S1017); itu utang terpisah dan tidak diperbaiki di sini |

## Log pengerjaan

| Tanggal | apa | hasil |
| --- | --- | --- |
| 2026-10-15 | Taksonomi + registry signature + `RestDirective` + kolom provenance (bagian pertama patch, sebelum dokumen ini) | klasifikasi menghasilkan kelas/reset/sumber; park quota membaca reset vendor |
| 2026-10-15 | Delapan koreksi di daftar "Koreksi"; aksi kelas wired ke walk (`keepsWalking`, `Outcome.ChainStopped`); retry transport membaca jawaban signature | `go test ./internal/...` hijau; test baru `engine_failover_actions_test.go` menjepit stop/rotate terhadap akun kedua, `failure_classification_test.go` menjepit "tidak ada retry pada spent allowance" dan park sampai reset vendor |
| 2026-10-15 | Read-back provenance ke proyeksi kunci + API + kontrak OpenAPI; GET `failure_classes` level group; validasi menolak kelas yang tidak bisa di-tune | `TestOpenAPIContract_PropertyNamesMatchDTOs` hijau setelah regenerasi; test hermetik untuk lowering/render/validation (`settings_failure_classes_test.go`, `failure_classes_routing_test.go`, `endpoint_key_provenance_test.go`) |
| 2026-10-15 | Pemecahan berkas sesuai AGENTS.md §1.1: sembilan berkas melewati 250 baris dipecah per concern (`failure_class_policy.go`, `failure_rest.go`, `settings_routing_rest.go`, `key_failure_class.go`, `transport_attempt.go`, `failure_reset.go`, `settings_failure_classes.go` + tiga berkas test) | `go build ./...`, `go-headers.sh`, dan `go-lint.sh` line-limit; suite hermetik dijalankan ulang setelah tiap pemecahan |
