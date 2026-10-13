# 050-KEY-PARKING-AND-ACCOUNT-HEALTH.md: kesehatan kunci menghukum penyebab yang bukan salah kunci, dan level akun dibangun tapi tidak pernah disambungkan

Laporan owner 2026-10-12: sepuluh temuan (T1-T10) atas jalur `key parking` / `account health` di data plane,
dengan daftar "(c) yang sudah benar" dan enam pertanyaan wajib. Pass pertama dokumen ini **tidak memperbaiki apa
pun**: ia memverifikasi kesepuluh klaim terhadap kode, mengoreksi dua di antaranya, menambah empat penajaman yang
tidak ada di laporan, dan menyusun sisanya sebagai pekerjaan berprioritas HIGH → MEDIUM → LOW untuk dilanjutkan.
Pass kedua mengerjakan **grup A (T3, T4)**; hasilnya di bagian terakhir dokumen ini.

Kesepuluh temuan terverifikasi. Tidak ada yang dibatalkan.

**Nomor baris.** Butir Temuan dan bagian (c) ditulis terhadap `main` @ `1d9da1a`, dan grup A lalu B mengubah berkas yang dirujuk di sana
(`upstream_key_health.go` dan `engine_failure.go` khususnya), jadi angka itu sekarang bergeser. Yang masih
boleh dipercaya apa adanya adalah rujukan ke berkas yang belum disentuh; rujukan ke berkas yang berubah
dikoreksi di bagian Hasil masing-masing grup, dan itu tempat grup C harus mulai.

## Status dan lingkup

| | |
| --- | --- |
| **Status** | **Grup A (T3, T4) landed 2026-10-12, commit `c3d7ff1`. Grup B (T7, T5) landed 2026-10-13, commit yang sama.** Grup C landed sebagian besar 2026-10-13: T6 (guard dan target probe), koreksi komentar T1/T2, dan jalur keluar kuburan T2 lewat refresh operator; T9 diputuskan tetap menulis kesehatan. Yang terbuka: **T1(b) kolom cooldown poller** (arah sudah dipilih, sisa pekerjaan) dan **T2(ii) retry ber-eskalasi** (diukur salah, diundur, lihat Hasil grup C). Grup D (T8, T10) ☐. Keputusan: D1 terjawab dan bentuknya berubah dari rekomendasi saya, D3 dijawab owner dengan opsi (a), D5 terjawab oleh pengerjaan (GREATEST, bukan transaksi). Yang masih menunggu putusan owner: **D2 (402)** dan **D4 (probe)** |
| **Cakupan** | `app-serv/internal/dataplane/` (`engine_failure.go`, `engine_relay.go`, `transport_call.go`, `selection.go`, `selection_health.go`, `ping.go`), `internal/domain/` (`upstream_key_health.go`, `upstream_endpoint.go`, `oauth_refresh.go`, `settings_routing.go`), `internal/repository/postgres/` (`endpoint_key_health.go`, `endpoint_parity.go`, `endpoint_oauth_batch.go`, `quota_published.go`), `internal/service/` (`endpoint_probe_use.go`, `media_perform.go`) |
| **Tidak termasuk** | `app-ui`. Kolom `rate_limited_until` dan `test_status` dirender panel (`EndpointKeysTable.svelte`, `EndpointDetailDrawer.svelte`) dan ikut berubah bentuknya hanya jika D3/D5 memutuskan begitu; itu pass tersendiri |
| **Struktural (AGENTS.md §1.9)** | **Bukan N/A.** T1 mengaktifkan kolom yang hari ini mati, T2/T6 mengubah siklus hidup `status`, T3/T4 mengubah arti `KeyFailureClass` yang merupakan kontrak §7.5, dan T8 menambahkan field ke `settings_routing`. `SYSTEM_MAP.md` harus diubah pada commit yang mengubahnya, dan SPEC-API §7.5 perlu dibaca ulang sebelum D3 dikerjakan |
| **Sumber bukti** | Grep + baca terhadap pohon kode pada `main` (`1d9da1a`). Tidak ada yang diverifikasi secara live; tidak ada gateway yang dijalankan, tidak ada `EXPLAIN`. Butir yang butuh pengukuran ditandai di butirnya sendiri |

## Diagnosa

Dua tema, dan keduanya adalah kegagalan yang sama dengan cara berbeda.

**Pertama: kunci dihukum untuk hal yang bukan keputusan kunci.** Kelas kegagalan diturunkan dari *status HTTP*, bukan
dari *siapa yang bersalah*. `FailureClass` (`engine_failure.go:27-39`) tidak bisa membedakan "kredensial ini mati"
dengan "saya menanyakan model yang tidak kamu punya": keduanya 401-shaped bagi fungsi itu, dan 404 masuk keranjang
yang sama (`:29-31`). `failureClass` (`:43-48`) lebih parah: apa pun yang bukan jawaban upstream adalah transient,
termasuk DNS dan TLS di host gateway sendiri, yang tidak pernah menyentuh kredensialnya. Konsekuensinya struktural,
dan ada di penajaman P2: satu request dengan nama model yang salah memparkir **setiap kunci** yang dilalui rantai
failover, bukan hanya satu.

**Kedua: level akun sudah dibangun, dan tidak pernah disambungkan.** `rate_limited_until` di `upstream_endpoints`
mempunyai kolom (`migrations/000005_upstream_endpoints.up.sql:17,49`), jalur baca (`endpoint_row.go:26`), jalur
bersih saat disable (`upstream_endpoint.go:179`), dan **tidak punya satu pun penulis deadline**. Komentar di
`quota_published.go:105-107` menyatakan kolom itu ditulis data plane saat 429; filter poller di `:113` menyaring
dengannya. Yang benar: tidak ada jalur Go yang menaruh nilai non-NULL di sana, dan satu-satunya nilai non-NULL yang
pernah terbukti ada di kolom itu dihasilkan oleh SQL mentah di test
(`quota_published_schedule_integration_test.go:183`). Jadi komentar itu mendeskripsikan perilaku yang hanya ada di
test. Filternya no-op, dan ini bukan salah pengujian: ini salah klaim.

Satu hal yang menghubungkan keduanya: **`error` di level endpoint tidak pernah diangkat kembali oleh apa pun
selain sukses.** MarkUnhealthy punya satu pemanggil (`oauth_flow_refresh.go:179`) dan `candidates()` menyaring
`Status: active` (`selection.go:180-183`). Itu kuburan, bukan cooldown, dan T6 menunjukkan varian diamnya: kolom
`status` bisa tinggal `error` selamanya meski jendelanya sudah lama lewat, hanya karena kuncinya tidak pernah
dipilih lagi untuk sukses.

## Keputusan yang dibutuhkan dari owner

Keputusan ini tidak bisa saya putuskan sendiri karena masing-masing memilih definisi "salah siapa", dan dua di
antaranya mengubah kontrak §7.5 yang sudah dipublikasikan. D2 satu-satunya yang belum punya rekomendasi.

**D1: 404 dari upstream, milik siapa?** **TERJAWAB 2026-10-12, dan rekomendasi saya salah.**
Rekomendasi semula: keluarkan 404 dari `KeyFailureAuth` dan masukkan ke `KeyFailureRequest`. Itu keliru karena
`KeyFailureRequest` menggabung dua akibat: parkir nol (`upstream_key_health.go:93-95`) **dan** hentikan rantai
failover (`engine_failure.go:81-86` lewat `failoverWorthy`). Untuk 404 yang pertama benar dan yang kedua salah:
satu akun yang tidak punya sebuah model tidak berarti akun berikutnya tidak punya, dan combo serta multi-akun ada
justru untuk berjalan ke akun berikutnya. Test `engine_failover_rules_test.go` sudah mem-pin "404 fails over" sejak
028, jadi memindahkan 404 ke Request akan mematahkan aturan yang sudah di-pin.

Yang dikerjakan: kelas baru `KeyFailureTarget` (parkir kunci nol, parity endpoint tetap ditulis, rantai tetap jalan),
dan kegagalan transport lokal yang tidak pernah sampai ke upstream masuk ke kelas yang sama. Aturan "kelas mana yang
mengimplikasikan kredensial" jadi satu predikat di domain (`ImplicatesCredential`) sehingga agregat dan selector tidak
bisa berbeda jawaban, dan predikat itu memakai daftar positif: kelas baru default-nya tidak memparkir.

Catatan yang tetap berlaku: 404 yang *dihasilkan gateway sendiri* (`CodeModelNotFound`, `resolve*.go`,
`errors.go:97-98`, keputusan 041) tidak melewati `FailureClass`, jadi keputusan ini hanya soal 404 dari vendor.
Dukungan tambahan: PORT 011 sudah memakai alasan yang persis sama untuk poll video, "a stale task id answering 404
would otherwise park a healthy key" (`docs/PORT/011`, kutipan di SPEC changelog 2026-10-12); jalur chat baru
sampai grup A ini mengikuti penalaran itu.

**D2: 402 di dalam keranjang Auth.** `engine_failure.go:29-30` menggabung 401/402/403/404. 401/403 kredensial,
402 tagihan habis, dan untuk gateway multi-akun, 402 adalah kondisi akun yang nyata dan berjam-jam, bukan 2 menit.
Bisa dikerjakan bersama D1, atau ditinggalkan. Tidak ada rekomendasi tanpa keputusan.

**D3: eskalasi kredensial mati. Permanen seperti apa, dan siapa yang membangunkan?** **TERJAWAB 2026-10-13: opsi (a).**
T5 menuntut ambang strike; T2 menunjukkan apa yang terjadi kalau status dipakai sebagai parkir (kuburan satu arah).
Jadi ini satu keputusan, bukan dua: keadaan apa yang boleh ditulis *sistem* ke `status`, dan bagaimana ia keluar.
Pilihan yang saya lihat: (a) tetap `error` + jendela panjang dengan eskalasi (mis. 2m→10m→30m, cap), tanpa status
baru; (b) status key baru di luar tiga yang ada (`ParseUpstreamKeyStatus`/`IsValid`/`Transition`,
`upstream_key.go:29-44,173-184`) plus re-probe malas; (c) tetap 2 menit selamanya dan terima konsekuensinya.
(a) paling kecil dan tidak menyentuh parse publik; (b) paling jujur secara semantik dan menyentuh kontrak.
Perlu keputusan juga apakah sistem boleh menulis `disabled`: komentar `upstream_key_health.go:74-75` menyatakan
`disabled` milik operator, dan klien sudah tidak bisa memalsukan state breaker (alasan di `oauth_refresh.go:54-57`).

**D4: probe operator boleh menulis kesehatan?**
T9 terkonfirmasi (`ping.go:41-47` → `Relay` → `relayOnce` → `recordFailure` `engine_relay.go:140,182`). Yang tidak
tertulis di laporan: bypass-nya selektif dan terbalik. `Ping` sengaja memanggil `Relay` langsung supaya usage row
tidak ditulis (`ping.go:38-41`), sehingga probe **lolos dari pembukuan spend tapi tetap membayar health write**,
yaitu diagnostik gratis pada sumbu yang salah. Rekomendasi: satu seam "jangan catat kesehatan" yang di-thread ke
relay,
bukan salinan loop relay; tapi letak seam-nya (opsi di `Request`, atau varian `Selection`, atau flag di `recordFailure`)
menentukan berapa banyak file yang bergerak.

**D5: keatomikan. CAS, transaksi, atau SQL yang menghitung sendiri?**
T7. `inTx` sudah dipakai lima kali di package ini (`endpoint.go:39`, `endpoint_batch.go:56,73`,
`endpoint_keys.go:154`, `endpoint_oauth_batch.go:48`), dan **`FOR UPDATE` juga sudah dipakai di jalur tulis endpoint
lain**: reorder mengunci barisnya dengan `SELECT id FROM upstream_endpoints ... FOR UPDATE` (`endpoint_keys.go:156`).
Jadi keduanya bukan primitif baru untuk D5, melainkan pola yang sudah ada di package yang sama dan belum diterapkan
di jalur kesehatan. Yang harus dipilih adalah mana dari keduanya yang cocok untuk semantik "jendela memperpanjang,
bukan menimpa". Rekomendasi: `GREATEST` untuk jendela (dua kegagalan konkuren harus memperpanjang, bukan
saling menimpa) plus satu tx untuk pasangan tulis key-health + endpoint-parity
(`dataplane/selection_health.go:50-57`), dan `status` diturunkan dari ada/tidaknya jendela alih-alih ditulis absolut. Yang harus diputuskan: apakah ladder tetap
di domain (DDD §2.2) dan hanya *deadline*-nya yang di-`GREATEST` di SQL, atau ambang strike ikut pindah ke SQL.
Butir terakhir ini belum terukur: kesimpulan "tidak ada lock/transaksi" berasal dari tidak adanya `Begin`/`FOR UPDATE`
di dua penulis jalur kesehatan (`endpoint_key_health.go:29-55`, `endpoint_parity.go:25-50`) maupun di pemanggilnya
(`dataplane/selection_health.go:44-57`), bukan dari `EXPLAIN` atau test konkurensi.

## Temuan

ID tetap memakai nomor laporan owner (T1-T10) supaya nyambung saat dilanjutkan. `Benar` = terverifikasi terhadap
kode. Prioritas menentukan kelompok pengerjaan, bukan urutan di dalam kelompok.

### Kelompok A (HIGH): klasifikasi (T3, T4)

Cuma menyentuh dua fungsi, tidak butuh migration, tidak butuh keputusan owner selain D1, dan ini yang paling
melukai klien hari ini.

#### T3 (HIGH) Benar, LANDED: 404 (model tidak ada) diperlakukan sebagai kredensial mati → parkir 2 menit

`FailureClass` memetakan 401/402/403/404 ke `KeyFailureAuth` (`engine_failure.go:29-31` sebelum perubahan),
`keyAuthCooldown` = 2 menit (`upstream_key_health.go:35,100`). Satu nama model yang salah cukup untuk memarkir
kunci yang sehat.

**P2: penajaman yang tidak ada di laporan.** Kerusakannya bukan satu kunci, melainkan semua kunci yang dilewati
satu request. Untuk 404, `translateCallError` tidak masuk cabang 429 maupun `KeyFailureRequest`
(`engine_failure.go:75-90`), jadi kodenya `CodeUpstreamError`, `failoverWorthy` benar (`:109-116`), dan loop relay
`continue` (`engine_relay.go:143-145`) ke kandidat berikutnya, dengan `recordFailure` di setiap lintasan
(`:140`). Rantainya berhenti di `NO_PROVIDER` (`selection_candidates.go:69-70`). Akibatnya: **satu request dengan
nama model yang salah memparkir seluruh kunci provider yang dilaluinya selama 2 menit**, termasuk untuk model lain
yang tidak bertanya apa pun. Nama model yang salah biasanya di-retry klien dengan nilai yang sama, jadi ini berulang.

Diperbaiki lewat D1, dengan kelas baru `KeyFailureTarget` dan bukan `KeyFailureRequest`, karena alasan di D1:
Request juga menghentikan rantai failover dan itu salah untuk 404. `FailureClass` sekarang menjawab Target untuk
404 (`engine_failure.go:33-36`) sementara 401/402/403 tetap Auth (`:29-30`).

Terbukti selesai, dengan pengukuran (lihat Hasil grup A): sebelum fix, satu request 404 atas endpoint berkunci dua
meninggalkan **dua** health write dengan `status:error` dan `consecutiveErrors:1`; sesudahnya nol health write untuk
kunci yang ditolak, parity endpoint tetap tercatat, dan rantai tetap berjalan ke kredensial kedua.

#### T4 (HIGH) Benar, LANDED: kegagalan jaringan lokal dihukum pada kunci

`failureClass` menjawab `KeyFailureTransient` untuk apa pun yang bukan `AsUpstreamError` (`engine_failure.go:47`
sebelum perubahan), dan transient memparkir 30 detik (`upstream_key_health.go:40,97`). DNS gagal, TLS gagal, dial
timeout, dan deadline lokal, semuanya ditulis sebagai kegagalan kredensial, padahal kredensialnya belum dievaluasi
sama sekali. Jalur media eksplisit melakukan hal yang sama: `media_perform.go:63,103` mengirim
`KeyFailureTransient` untuk "could not be reached". Saat dikerjakan, pola yang sama ditemukan di dua jalur lain yang
tidak disebut laporan: `embeddings_call.go:48` dan `systemone.go:172`, keduanya mengirim Transient untuk kegagalan
`caller.Do` yang bentuknya identik dengan media. Tiga-tiganya ikut diperbaiki.

**P3: penajaman.** Deadline *klien* ikut memarkir kunci. `transport_call.go:145-151` memetakan
`context.DeadlineExceeded`/`context.Canceled` ke `timeoutError`, yang bukan `AsUpstreamError`, jadi masuk transient.
Tab yang ditutup saat request masih di udara memparkir kunci sehat 30 detik. Asimetrinya membuktikan ini bukan
keputusan: disconnect *di tengah stream* sudah ditangani dengan benar dan sengaja tidak menghukum siapa pun
(`engine_stream.go:102-106`, "a quiet stop is the documented behaviour for a disconnected client"), hanya jalur dial
yang lupa pertanyaan yang sama.

Fix yang landed: `failureClass` menjawab `KeyFailureTarget` untuk setiap kegagalan yang tidak menghasilkan jawaban
upstream (`engine_failure.go:47-52`), dan tiga jalur media/embeddings/decision mengikuti. Keputusan "memarkir
*endpoint* atau tidak memarkir apa pun" untuk kegagalan transport **ditunda ke T1**, dan itu tidak memblokir fix ini:
yang dihentikan di grup A adalah hukuman pada kredensial, sementara jendela level endpoint memang belum bisa ditulis
sampai T1 hidup. Konsekuensi yang harus diketahui operator: 5xx yang dijawab upstream tetap memparkir kunci 30 detik
(kontrolnya di-pin `TestRelay_UpstreamFiveKeepsParkingTheCredential`), karena pemindahan parkir itu ke level target
butuh kolom T1 dan itu grup C.

Terbukti selesai, dengan pengukuran: sebelum fix, satu request dengan upstream yang tidak bisa dihubungi
meninggalkan dua health write `status:error` dengan jendela 30 detik; sesudahnya nol, dan test kontrol 5xx memastikan
perilaku yang sengaja dipertahankan tidak ikut berubah.

### Kelompok B (HIGH): counter, eskalasi, dan race (T5, T7)

Dikerjakan setelah A, dan T7 lebih dulu dari eskalasi T5: ambang strike yang dihitung dari nilai memori di atas
counter yang dimiliki SQL akan meleset persis pada kasus yang membuatnya dibutuhkan (dua kegagalan konkuren).

#### T7 (HIGH) Benar, LANDED: counter aman di SQL, tapi `rate_limited_until` / `status` / `last_error` ditulis absolut

Yang sudah benar dan jangan dirusak: `consecutive_errors = CASE WHEN $4::boolean THEN 0 ELSE consecutive_errors + 1
END` (`endpoint_key_health.go:37` hari ini), dengan alasannya tertulis di `:19-28`; `consecutive_use_count` sama
(`endpoint_parity.go:26`).

Yang belum (bentuk sebelum grup B): `status = $1` dan `rate_limited_until = $5` di statement yang sama adalah nilai absolut dari
agregat yang dimuat masing-masing request lewat `candidates()` → `List` (`selection.go:180`). Di jalur kesehatan ini
tidak ada `Begin`, tidak ada `FOR UPDATE`, tidak ada `GREATEST`, padahal ketiganya sudah dikenal package ini
(lihat D5: `endpoint_keys.go:156`).

Tiga konsekuensi, dan yang keempat tidak ada di laporan:

- sukses + gagal konkuren pada satu kunci → last-write-wins. Gagal terakhir = kunci yang baru saja melayani diparkir.
- level endpoint: `RecordSuccess` menulangkan `last_error` (`endpoint_parity.go:33-42`), `RecordFailure` mengisinya
  (`:50`). Keduanya saling menimpa.
- dua kegagalan konkuren: masing-masing menghitung `rateLimitBackoff(1)` = 2s dari nilai awal yang sama, counter DB
  jadi 2 sementara jendelanya tetap 2s, ladder tertinggal satu langkah. Flag reset (`key.ConsecutiveErrors() == 0`,
  `endpoint_key_health.go:46`) mewarisi pembacaan basi yang sama.
- **P4.** Kunci dan endpoint ditulis sebagai **dua statement terpisah** (`selection_health.go:50-57`:
  `RecordKeyHealth` lalu `RecordUpstreamOutcome`). Tidak ada satu transaksi, jadi kegagalan di antara keduanya
  meninggalkan kunci parked tanpa parity endpoint, state yang tidak akan ditulis siapa pun.

Butuh D5. Perlu pengukuran sebelum fix: test konkurensi (`-race`, §2.1) dengan dua goroutine yang memanggil
`RecordFailure` pada kunci yang sama, untuk mengubah "tidak saya temukan" menjadi terbukti.

#### T5 (HIGH) Benar, LANDED sebagian: tidak ada keadaan permanen untuk kredensial mati; 401 diulang tanpa eskalasi

`consecutiveErrors` hanya dipakai cabang ladder 429 (`upstream_key_health.go:101-102`). Untuk `KeyFailureAuth`: tidak
ada ladder, tidak ada ambang, tidak ada transisi ke `disabled`. Siklusnya permanen dan tidak berhenti sendiri:
401 → park 2 menit (`:100`) → jendela habis → `Available()` true (`:83`) → dipilih lagi → 401 → dst. Untuk endpoint
satu kunci, provider itu benar-benar kosong 2 menit penuh dan klien menerima `NO_PROVIDER`
(`selection_candidates.go:69-70`). Jawaban atas pertanyaan owner nomor 4: **ya, terjadi, dan tidak ada mekanisme
yang menghentikannya.**

Ini pertanyaan desain, bukan bug yang bisa langsung ditutup; butuh D3. Catatan yang harus dibaca bersama D3: T2
adalah apa yang terjadi kalau eskalasi dilakukan dengan menulis `status` tanpa jalur keluar, dan hari ini jalur
keluarnya hanya sukses (`upstream_key_health.go:116-118`).

Dikerjakan grup B: jendela kini monotonic (`GREATEST`), dan dua harm lain (urutan sukses-versus-gagal, dan P4) dinilai serta diputuskan di Hasil grup B.

### Kelompok C (MEDIUM): level akun (T1, T2, T6, T9)

#### T1 (MEDIUM) Benar, dengan koreksi: kolom ada dan ditulis, tapi tidak pernah diisi deadline

Koreksi terhadap laporan: **kolom itu bukan tidak pernah ditulis.** `updateEndpoint` meng-`UPDATE ... rate_limited_until
= $7` (`endpoint_oauth_batch.go:88`) dari `endpoint.RateLimitedUntil()` (`:102`). Yang benar adalah tidak ada satu pun
jalur yang pernah menaruh *deadline* non-NULL di agregat: satu-satunya mutasi field ini adalah konstruktor
(rehydrate dari row, `upstream_endpoint.go:99,110`) dan pembersihan saat disable (`:179`). Tidak ada setter
(`upstream_endpoint*.go` hanya menghasilkan `:32,99,110,134,143,179`). Jadi UPDATE tersebut menulis kembali nilai yang
barusan dibacanya, dan disable menulis NULL.

Dua jebakan yang membuat grep ini menipu, dan keduanya menjelaskan kenapa laporan semula bisa salah baca:

- **dua tabel, satu nama kolom.** `upstream_keys` dan `upstream_endpoints` sama-sama punya `rate_limited_until`.
  Yang ditulis data plane adalah milik *kunci* (`endpoint_key_health.go:38-39` hari ini, penulisnya
  `upstream_key_health.go:106`); milik *endpoint* (`endpoint_oauth_batch.go:88`) tidak pernah dapat deadline.
- **`upstream_endpoint_keys.go:95`** (`replaced.rateLimitedUntil = key.rateLimitedUntil`) terlihat seperti jalur
  tulis endpoint padahal `replaced` adalah `UpstreamKey`: pemeliharaan kesehatan saat kredensial di-rotasi
  ("a rotated credential is the same account", `:89-90`). Nama file-nya menaruhnya di glob `upstream_endpoint*`.

Efek yang dilaporkan tetap benar seluruhnya: `Available()` pada level endpoint (`:143`) tidak pernah menyala karena
jendela, dan filter poller `e.rate_limited_until > $1` (`quota_published.go:113`) selalu NULL → no-op, sementara
komentar `:105-107` mengklaim data plane sudah menulisnya. Test yang membuat filter itu tampak teruji mengeset kolom
dengan SQL mentah (`quota_published_schedule_integration_test.go:183`), itu membuktikan WHERE-nya benar, bukan
bahwa produksi menghasilkan nilainya.

Minimal dua hal yang harus dibereskan, dan hanya satu yang butuh keputusan: komentar harus dibuat akurat (tidak ada
debate di sini), dan jalur tulis harus ada atau filternya dihapus. Arah fix menunggu D1/T4, karena penulis alami
kolom ini adalah 429 pada level *endpoint*, yang hari ini hanya ada di level kunci.

#### T2 (MEDIUM) Benar, TIDAK dikerjakan: satu-satunya transisi akun → `error` adalah OAuth dead-letter, dan itu kuburan satu arah

`MarkUnhealthy` menulis `UpstreamEndpointError` (`oauth_refresh.go:58-61`), satu pemanggil
(`oauth_flow_refresh.go:179`), dan `grep UpstreamEndpointError` di luar deklarasi
(`upstream_endpoint_types.go:25`) hanya menghasilkan baris itu. `candidates()` menyaring `Status: active`
(`selection.go:180-183`), jadi endpoint hilang dari rotasi tanpa batas waktu sampai operator PATCH ke `active`
(`ParseUpstreamEndpointStatus` mengizinkan `active`, `upstream_endpoint_types.go:71-79`). Tidak ada job yang
mengangkatnya. Kegagalan upstream biasa tidak menyentuh status endpoint sama sekali
(`selection_health.go:50,52`).

Komentar di `oauth_refresh.go:54-57` menyatakan ini disengaja (dead-letter adalah aturan bisnis, bukan PATCH klien),
dan itu benar sebagai *sebab* masuknya keadaan. Yang tidak diputuskan di sana adalah *keluarnya*. Butuh D3, karena
jawaban yang sama dipakai T5 dan T6.

#### T6 (MEDIUM) Benar, LANDED untuk guard kunci; sisi probe masih keputusan: `status='error'` di DB tidak pernah dibalik kecuali oleh sukses

**Efek silang dari grup B, catat sebelum mengerjakan ini.** T5 menaikkan jendela kredensial dari 2 menit sampai
1 jam, dan kolom `status` hanya dibalik oleh sukses. Artinya rentang tempat T6 bekerja jadi lebih lebar: kunci yang
diparkir 40m sekarang tidak terpilih LRU selama 40 menit itu, jadi `selectProbeKey` melewatinya lebih lama dan
tombol Test lebih sering menemukan "no active key to test" pada endpoint yang sebenarnya sehat. Perbaikan T6
(derive, bukan re-read kolom) jadi lebih penting, bukan kurang penting, sejak 2026-10-13.

Satu-satunya penulis `active` dari `error` adalah `RecordSuccess` (`upstream_key_health.go:129-134` setelah grup B) via
`RecordKeyHealth` (`endpoint_key_health.go:38`). Kunci yang pernah gagal lalu tidak pernah terpilih lagi (LRU/
prioritas) tinggal `error` selamanya meski jendelanya sudah lama lewat.

Dampak yang terverifikasi di kode (belum diverifikasi live):
- `activeKeyCount()` tidak menghitungnya (`upstream_endpoint_keys.go:163-171`), jadi guard "harus menyisakan ≥1
  active key" (`RemoveKey:52`, `refusesDeactivation:64-72`) bekerja di atas hitungan yang sudah menyusut.
- `selectProbeKey` melewati kunci `Status() != active` (`endpoint_probe_use.go:125`), jadi tombol Test bisa menolak
  dengan "this endpoint has no active key to test" (`:133`) pada endpoint yang sebenarnya masih bisa melayani, karena `Available()` (`upstream_key_health.go:76-84`) menganggapnya pulih, sementara kolom `status` belum.

Ini juga alasan `Available()` dan `status` perlu dinyatakan sebagai dua hal yang berbeda, bukan dua nama untuk satu
state. `Available()` sudah benar: jendela adalah state-nya, tanpa sweeper.

#### T9 (MEDIUM) Benar, TIDAK dikerjakan: probe diagnostik memiliki efek samping pada kesehatan, dan bypass-nya separuh

`Engine.Ping` → `Relay` (`ping.go:41-47`) → `relayOnce` → `recordFailure` (`engine_relay.go:140,182`). Menekan
"Test" memparkir kunci persis seperti trafik nyata (2 menit untuk 401, 30 detik untuk 5xx, dan pasca-T4/D1 untuk
sisanya). Penajaman P1 di Diagnosa: komentar `ping.go:38-41` menjelaskan *kenapa* Relay dipanggil langsung supaya
usage row tidak ditulis, jadi probe sudah dikecualikan dari spend dan tidak dari kesehatan. Butuh D4.
`combo_probe.go` terhubung ke judgement, tapi sebagai konsumen yang menulis; "judge" di sana adalah model judge
fusion (`service/combo_probe.go:65-70`), bukan penilai kesehatan.

Dikerjakan grup B sebagai D3(a): ladder kredensial 2m berlipat sampai cap 1h, satu sukses mengembalikan ke langkah pertama. Yang TIDAK dikerjakan: keadaan permanen dan ambang strike, karena itu keputusan D3 untuk T2/T6 (grup C). Detail dan tabel jendelanya di Hasil grup B.

### Kelompok D (LOW): konfigurasi dan kolom tampilan (T8, T10)

#### T8 (LOW) Benar: tidak ada ambang kesehatan yang bisa dikonfigurasi per provider

`settings_routing.go` (216 baris) hanya membawa rotasi (`fallback_strategy`, `sticky_limit`) dan override per provider
(`:69-72,96`, resolusi `:131-150`). Semua jendela parkir adalah konstanta (`upstream_key_health.go:35,40,44-48`).
Retry per provider memang ada tapi itu retry dalam satu target (`retry.go:84-130`, `entry.Transport.Retry`), bukan
parking kredensial. Operator tidak bisa memperpendek/memanjangkan cooldown, menaikkan ambang strike, atau
menonaktifkan parking.

Sengaja di LOW: ini butuh bentuk data baru di `settings_routing` (§1.9, dan berkasnya sudah 216 dari 250), dan
semua nilainya akan jauh lebih berguna setelah D3 memutuskan berapa banyak ambang yang memang ada. Mendesain
konfigurasi untuk tiga konstanta hari ini adalah pekerjaan yang akan diulang.

#### T10 (LOW) Benar: `test_status` tidak pernah menyaring rotasi

`Available()` (`upstream_endpoint.go:139-154`) tidak membaca `testStatus`. `RecordTest` (`:205-208`) hanya mengisi
kolom tampilan, jadi akun yang gagal diuji tetap dipilih router. Ini bukan bug yang berdiri sendiri: memilih
berdasarkan hasil probe adalah keputusan desain (probe bisa basi, dan kolomnya sudah dipakai panel), dan kolom yang
sama sudah dipakai display. Rekomendasi biarkan display-only, dan sinyalnya datang dari trafik nyata (T1); itu
menutup pertanyaan yang sama tanpa membuat satu probe menentukan rotasi. Perlu dikonfirmasi ke owner sebelum
dijadikan "fixed".

## Penajaman terhadap laporan owner (P1-P4)

Empat hal baru yang tidak ada di daftar, semuanya terverifikasi dari kode dan sudah ditulis di butirnya:

- **P1** (T9) Bypass probe selektif dan terbalik: `Ping` sengaja melewati pencatatan usage (`ping.go:38-41`) tapi tetap membayar health write.
- **P2** (T3) Satu request model-tidak-dikenal memparkir **semua** kunci di sepanjang rantai failover (`engine_relay.go:140,143-145` + `failoverWorthy` `engine_failure.go:109-116`), bukan satu.
- **P3** (T4) Deadline *klien* ikut memarkir kunci sehat 30 detik (`transport_call.go:145-151`), padahal disconnect di tengah stream sudah benar tidak menghukum apa pun (`engine_stream.go:102-106`).
- **P4** (T7) Key-health dan endpoint-parity adalah dua statement tanpa transaksi (`selection_health.go:50-57`), jadi tulisannya bisa separuh jalan.

Koreksi terhadap laporan, dan ini soal **T1**: kolom `rate_limited_until` *ditulis* (`endpoint_oauth_batch.go:88`), hanya tidak pernah diisi deadline. Detail, karena fix-nya berbeda: "tambah jalur tulis" vs "hapus filter yang bohong".

## Yang sudah benar, dan tidak boleh ikut "diperbaiki"

Daftar owner bagian (c) terverifikasi seluruhnya. Ditulis ulang di sini sebagai guardrail pass ini, karena
sebagian dari fix di atas berada tepat di sebelahnya:

- **`KeyFailureRequest` tidak menghukum kunci** (`upstream_key_health.go:93-95`, `selection_health.go:45-47`) dan rantai failover berhenti untuk itu (`engine_failure.go:81-86,109-116`). Payload klien tidak pernah mencapai upstream. **Ini yang dipakai D1 sebagai tujuan 404.**
- **Pembebasan tanpa sweeper**: `Available()` menjadikan jendela itu sendiri state-nya (`upstream_key_health.go:67-84`, batas ujung dieksklusif dan di-pin di `upstream_key_health_test.go:131-134`). Parkir tidak bisa "lupa" dibebaskan. Fix mana pun tidak boleh menambah job untuk hal ini.
- **Ladder terlindung dari overflow** (`rateLimitBackoff`, `:53-64`, di-pin untuk 20 dan 100 kegagalan, `upstream_key_health_test.go:89-97`).
- **Counter digerakkan SQL, bukan memori** (`endpoint_key_health.go:35`, alasan tertulis `:23-28`; `endpoint_parity.go:26`). T7 memperbaiki kolom di sekitarnya, bukan ini.
- **Klien tidak bisa memalsukan state breaker** (`upstream_key.go:29-44,173-184`, `upstream_endpoint_types.go:71-79`, alasan di `oauth_refresh.go:54-57`). Relevan untuk D3: kalau sistem boleh menulis keadaan baru, gerbang parse-nya tidak boleh dilonggarkan untuk klien.
- **Scrub kredensial sebelum disimpan** (`upstream_endpoint_parity.go:134-147,157-165`), kolomnya dirender panel.
- **Kegagalan bookkeeping tidak menimpa jawaban klien** (`engine_relay.go:90-93`, `engine_failure.go:59-69`, `logx.Degraded`).
- **Disable membersihkan jendela** (`upstream_key.go:178-182`, `upstream_endpoint.go:175-180`, di-pin `upstream_key_test.go:107-110`).
- **Tidak ada kredensial dicoba dua kali dalam satu request** (`spent` set, `engine_relay.go:96,106` → `NextKeySkipping`/`FirstKeySkipping`).
- **`Clone()` mencegah dua salinan berbagi array kunci** (`upstream_endpoint_key_reads.go:27-36`), komentarnya menyebut race health-write eksplisit.
- **Fail-open terarah pada gate non-kritis** (`selection.go:154-169,204-215,220-231`).
- **Disconnect klien di tengah stream tidak menghukum siapa pun** (`engine_stream.go:102-106`), lihat P3: jalur dial adalah yang tertinggal, dan pola jawabannya sudah ada di berkas sebelahnya.

## Jawaban atas enam pertanyaan wajib

1. **Siapa menulis status buruk**: `Selector.RecordFailure` (`selection_health.go:44`), dipanggil dari
   `engine_relay.go:140,182`, `media_perform.go:93`, `embeddings_call.go:48,53`, `systemone.go:172,177`,
   `endpoint_probe_use.go:90`.
2. **Berapa lama dan siapa membebaskan**: 2 menit (auth), 30 detik (transient), ladder 2s→5m (429), 0 (request).
   Tidak ada job; pembebasan malas saat `Available()` dibaca (`upstream_key_health.go:76-84`). Jaminan kembalinya
   ada untuk *pemilihan*, tidak ada untuk kolom `status` di DB, hanya sukses yang membaliknya (T6).
3. **Ditulis walau bukan salah akun?** Payload klien: tidak. 400/422 upstream: tidak. 404 upstream: **ya** (T3,
   dan P2: untuk semua kunci di rantai). Jaringan/timeout lokal: **ya** (T4, dan P3: termasuk deadline klien).
4. **Sementara vs permanen?** Dibedakan antara 429 dan auth, tapi tidak ada keadaan permanen untuk 401 dan tidak
   ada ambang strike: akun dengan kunci mati dicoba lagi setiap ~2 menit selamanya (T5). Satu-satunya keadaan
   permanen yang ada adalah kuburan OAuth satu arah (T2).
5. **Race?** Ya. Counter aman di SQL; `rate_limited_until`, `status`, `last_error` absolut tanpa
   lock/transaksi, dan pasangan key/endpoint tertulis sebagai dua statement (T7 + P4). Belum dibuktikan dengan
   test konkurensi.
6. **Dipakai combo?** Tidak untuk pemilihan anggota (murni strategi rotasi, `settings_routing.go:131-150`).
   `combo_probe.go` bukan penilai. Efek sampingnya tetap ada: probe ikut memparkir kunci (T9).

## Urutan pengerjaan

Grup A berdiri sendiri dan tidak menunggu keputusan apa pun selain D1, yang rekomendasinya jelas. Grup B adalah
grup yang mengubah perilaku paling besar, dan tidak boleh dimulai sebelum D3 dijawab. Jangan mengerjakan C
sebelum B: T1 mengaktifkan kolom yang penulis alaminya adalah hasil dari D1/D3.

| Grup | Isi | Gerbang masuk | Perkiraan sentuhan |
| --- | --- | --- | --- |
| **A** | T3, T4 (klasifikasi) | **LANDED 2026-10-12.** D1 terjawab dan bentuknya berubah dari rekomendasi (lihat D1); D2 masih terbuka dan tidak ikut dikerjakan | `engine_failure.go` (120), `upstream_key_health.go` (134), `selection_health.go` (79), tiga site service, `engine_target_failure_test.go` baru (114). `engine_relay.go` (237) sengaja tidak disentuh |
| **B** | T7 lalu T5 (counter, race, eskalasi) | **LANDED 2026-10-13.** D3 dijawab (a); D5 selesai sebagai GREATEST satu kolom, transaksi sengaja tidak | `endpoint_key_health.go` (55), `upstream_key_health.go` (122), `selection_health.go` (77), test `-race` |
| **C** | T1, T2, T6, T9 (level akun, probe) | butuh D3 (lagi) + D4 | `quota_published.go` (135), `oauth_refresh.go` (61), `ping.go` (72), `endpoint_probe_use.go` (180), migration kalau bentuk data berubah |
| **D** | T8, T10 (konfigurasi, tampilan) | butuh bentuk hasil A-C | `settings_routing.go` (216 ⚠ 250), `upstream_endpoint.go` (208) |

Aturan repo yang mengikat setiap grup di atas (`AGENTS.md`):

- **§1.1 / 250 baris.** Yang berisiko saat pass ini: `engine_relay.go` **237**, `settings_routing.go` 216,
  `upstream_endpoint.go` 208, `selection.go` 231. Untuk A, `engine_relay.go` hampir tidak punya ruang: perubahan
  di situ harus jadi berkas baru, bukan tambahan.
- **§2.1 TDD.** Test untuk B dan C ditulis lebih dulu atau bersamaan; keduanya service/repository. Tidak ada
  `t.Skip()`, tidak ada filter `-run`. B menuntut `-race` dan test dua goroutine pada satu kunci secara khusus.
- **§1.5.** `failureClass` dan `rateLimitBackoff` tetap domain; repository hanya mengubah kolom. Jangan mendorong
  keputusan kelas ke SQL tanpa D5.
- **§1.9.** `SYSTEM_MAP.md` pada commit yang mengubah siklus hidup `status` atau mengaktifkan kolom (B, C), dan
  SPEC-API §7.5 dibaca ulang sebelum D3 landed: T3/T4/D1 mengubah makna kelas kegagalan yang dipublikasikan.
- Gerbang: `bash scrypts/gates/all.sh`, dan antislop untuk berkas teks yang disentuh.

## Rencana verifikasi

Tidak ada yang di bawah ini sudah dijalankan. Perintahnya ditulis supaya butir berikutnya bisa membuktikan
dirinya sendiri, mengikuti aturan 047.

```bash
# klasifikasi hari ini, dibaca dari kode (bukti statis T3/T4)
sed -n '27,48p' app-serv/internal/dataplane/engine_failure.go

# T1: satu-satunya penulis deadline adalah level kunci. Ini diskriminatornya:
grep -rn "rateLimitedUntil = " app-serv/internal/domain/
#   upstream_key_health.go:106  k.rateLimitedUntil = &until   <-- satu-satunya non-NULL
#   upstream_key_health.go:115 / upstream_key.go:181          = nil (reset, disable)
#   upstream_endpoint.go:179    e.rateLimitedUntil = nil      <-- level endpoint: hanya NULL
#   upstream_endpoint_keys.go:95 = key.rateLimitedUntil       <-- JEBAKAN: `replaced` itu UpstreamKey, bukan endpoint
grep -rn "rate_limited_until" app-serv/internal/repository/postgres/ | grep -v _test
#   JEBAKAN KEDUA: endpoint_key_health.go:36 adalah kolom upstream_keys, bukan upstream_endpoints

# T5: tidak ada cabang eskalasi untuk Auth
sed -n '90,107p' app-serv/internal/domain/upstream_key_health.go

# T7: kolom absolut vs counter SQL
sed -n '29,47p' app-serv/internal/repository/postgres/endpoint_key_health.go
grep -rn "FOR UPDATE\|r.pool.Begin\|inTx" app-serv/internal/repository/postgres/*.go | grep -v _test
#   precedent-nya ada: endpoint_keys.go:156 (FOR UPDATE pada reorder), inTx di 5 jalur
#   yang membuktikan ketiadaan: tidak ada satu pun di sekitar RecordKeyHealth / RecordUpstreamOutcome
grep -rn "func (r \*EndpointRepository) RecordKeyHealth" -A 20 app-serv/internal/repository/postgres/endpoint_key_health.go
grep -rn "func (s \*Selector) RecordFailure" -A 16 app-serv/internal/dataplane/selection_health.go

# T9: probe lewat jalur yang menghukum
sed -n '38,47p' app-serv/internal/dataplane/ping.go

# P2: rantai failover mengulang recordFailure per kunci
sed -n '138,147p' app-serv/internal/dataplane/engine_relay.go

# setelah fix per grup:
cd app-serv && go test -race ./...
bash scrypts/gates/all.sh
```

Dua hal yang butuh lingkungan nyata dan belum dikerjakan, dan tanpa keduanya T7 dan T6 tetap kesimpulan aliran kode,
bukan pengukuran:

- test konkurensi dua `RecordFailure` pada satu kunci (butuh DB nyata, pola scratch database seperti 046/047 F5).
- `EXPLAIN`/pemeriksaan apakah `e.rate_limited_until > $1` memang seq-scan tanpa index di `quota_published.go:113`.

## Hasil grup A (T3, T4), landed 2026-10-12

Satu kelas baru di domain, `KeyFailureTarget`, dan aturan "kelas mana yang mengimplikasikan kredensial" jadi satu
predikat yang dibaca dua pemakai (`ImplicatesCredential`, `upstream_key_health.go:39-49`): daftar positif, jadi kelas
yang belum dipikirkan orang berikutnya default-nya tidak memarkir kunci.

| Berkas | Perubahan | Baris |
| --- | --- | --- |
| `internal/domain/upstream_key_health.go` | `KeyFailureTarget` + `ImplicatesCredential`; `RecordFailure` memakai predikat, bukan `== KeyFailureRequest` | 134 |
| `internal/dataplane/engine_failure.go` | 404 keluar dari Auth ke Target (`:33-36`); kegagalan non-upstream ke Target (`:47-52`) | 120 |
| `internal/dataplane/selection_health.go` | Target menulis parity endpoint, tanpa tulis kesehatan kunci | 79 |
| `internal/service/media_perform.go`, `embeddings_call.go`, `systemone.go` | tiga site "could not be reached" pindah dari Transient ke Target | 112 / 68 / 192 |
| `internal/domain/upstream_key_health_test.go` | baris Target di tabel parkir, test predikat, test counter tidak naik | 190 |
| `internal/dataplane/engine_target_failure_test.go` | **baru**: 404 tidak memarkir sepanjang rantai, transport tidak memarkir, 5xx masih memarkir (kontrol) | 114 |
| `internal/dataplane/engine_failover_rules_test.go` | kolom `wantKeyPark`; 404 gagal-over tanpa parkir | 161 |
| `internal/dataplane/engine_failover_fixture_test.go` | double `alwaysRefusingUpstream` | 100 |

`engine_relay.go` (237) tidak disentuh, dan itu pilihan: ia satu-satunya berkas di jalur ini yang tidak punya
headroom §1.1, dan tidak ada perubahan yang menempel padanya.

### Bukti, dengan pengukuran

Tiga test baru dijalankan terhadap klasifikasi lama lebih dulu, dan merah dengan angka yang tepat:

```
--- FAIL: TestRelay_UnknownModelParksNoCredentialAcrossTheChain/404
    key health writes = map[uky-multi-a:{... status:error consecutiveErrors:1 rateLimitedUntil:0x...}
                            uky-multi-b:{... status:error consecutiveErrors:1 rateLimitedUntil:0x...}],
    want none for a refusal that is not about the credential
--- FAIL: TestRelay_TransportFailureParksNoCredential
    key health writes = map[uky-multi-a:{... status:error ...} uky-multi-b:{...}], want none
```

Itu sendiri yang membuktikan P2: **satu request 404 memarkir kedua kunci akun itu**, bukan hanya kunci yang
dipakai. Sesudah fix ketiganya hijau, dan test kontrol `TestRelay_UpstreamFiveKeepsParkingTheCredential` memastikan
5xx yang dijawab upstream tidak ikut berubah.

Satu koreksi atas asersi saya sendiri, tercatat karena ia hampir lolos sebagai test yang salah-bukti: assert awal
"`len(repo.health) == 0`" untuk kasus 404 gagal bukan karena kunci diparkir, tapi karena **kunci akun kedua yang
melayani** menulis suksesnya (`status:active`, tanpa jendela). Yang benar adalah "kunci yang ditolak tidak dapat health
write", dan assert itu yang sekarang di-pin.

### Gerbang

```
go test -race -count=1 ./...   bersih (13 paket app-serv, tanpa cache)
go-lint / go-headers / antislop / contract-drift / contract-openapi / secrets   PASS
panel-check                    PASS (exit 0, suite app-ui penuh; memang lama, >560 dtk)
```

### Prasyarat grup B: database scratch, dan jebakan yang sudah pernah terjadi

PostgreSQL host ini **hidup** (`/var/run/postgresql:5432 - accepting connections`), jadi T7 tidak harus berhenti
sebagai kesimpulan aliran kode: ia bisa diukur. Tapi ada dua hal yang harus dibaca dulu sebelum siapa pun menyetel
`PANNELAI_TEST_POSTGRES_DSN`:

- Harness menolak database yang namanya tidak mengandung `test`, dan alasannya tertulis di
  `integration_harness_test.go:44-47`: setiap harness di package ini **TRUNCATE** tabel yang dibacanya, jadi
  menunjuk variabel itu ke database nyata menghancurkan isinya. Guard itu ada justru karena kejadian yang
  terdokumentasi: `docs/DRAFT/025-CATALOG-ACTIVE-FILTER.md:197` mencatat DSN disetel ke database dev `pannelai`
  (menyalin `POSTGRES_DSN` dari `.env`), dan `docs/DRAFT/026-LIVE-REDIS-FLUSHED-BY-INTEGRATION-TESTS.md` mencatat
  Redis hidup yang ikut disapu.
- Host ini menyimpan database proyek lain (`APIGateway`, `kentangtech`, `appgateway`, `appbot_shared`, dan
  seterusnya). Sebuah scratch DB untuk pass ini harus dibuat baru dengan nama yang mengandung `test`, bukan
  dipinjam dari yang sudah ada, dan kredensialnya adalah keputusan owner mesin ini.

Karena itu **T7 belum dikerjakan di pass ini**: tidak ada kredensial yang saya punya, dan mencari-cari kredensial DB
di host yang berisi database produksi-like bukan tindakan yang bisa saya ambil sendiri. Yang dibutuhkan untuk
melanjutkan: satu DSN scratch, misalnya `postgres://<role>:<pass>@localhost:5432/pannelai_integration_test?sslmode=disable`.

### Kontrak dan dokumen

`docs/SPEC-API/001-SPEC-API.md` §7.5 ditulis ulang (401/402/403 ⇒ 2m; 429 ladder; 5xx dijawab ⇒ 30s; target-shaped
⇒ parity tercatat, kunci tidak diparkir, rantai jalan terus) plus entri changelog 2026-10-12, karena spesifikasi itu
menuntut "any deviation requires editing this spec in the same PR" dan kalimat lamenya memang menuliskan
`401/402/403/404 ⇒ 2m`. Tidak ada perubahan bentuk wire, jadi CONTRACT YAML dan OpenAPI tidak bergerak
(`contract-drift` dan `contract-openapi` PASS).

`SYSTEM_MAP.md`: **N/A untuk grup A.** Peta tidak punya tabel kelas-ke-jendela (itu milik §7.5) dan tidak ada struktur
data, batas domain, atau interaksi antar servis yang berubah. Satu barisnya ikut salah dan sengaja tidak saya
sentuh di sini: `SYSTEM_MAP.md:544` mengklaim poller kuota menyaring cooldown "`rate_limited_until` dari jalur data",
klaim yang sama bohongnya dengan komentar `quota_published.go:105-107` di T1. Itu ikut diperbaiki di grup C, di
commit yang sama dengan jalur tulis kolomnya.

### Yang terbuka dari grup A

- **D2 (402) belum diputuskan** dan 402 masih berada di keranjang Auth. Tidak saya ikut pindahkan, karena tidak ada
  keputusan: untuk gateway multi-akun 402 adalah kondisi akun yang berjam-jam, jadi jawabannya bisa "jendela lebih
  panjang", bukan "jangan parkir".
- **Parkir 5xx di level kunci tetap 30 detik.** Memindahkannya ke level target butuh kolom T1 (grup C), dan
  meninggalkan provider yang sakit tanpa breaker sama sekali adalah bertukar bahaya, bukan perbaikan.
- **Panel tidak berubah.** `test_status` dan kolom `rate_limited_until` di `app-ui` tetap seperti sebelumnya;
  `EndpointKeysTable.svelte` hanya akan melihat lebih sedikit kunci berstatus `error` untuk sebab yang bukan
  kredensial, dan itu konsekuensi yang benar.

## Hasil grup B (T7, T5), landed 2026-10-13

### Lingkungan yang dibutuhkan, dan cara membuatnya

T7 hanya bisa dibuktikan terhadap PostgreSQL nyata, dan host ini punya satu yang hidup. Yang dipakai pass ini:

```bash
createdb pannelai_health050_test          # nama WAJIB mengandung `test` (guard harness)
export PANNELAI_TEST_POSTGRES_DSN="user=rusmanadodi dbname=pannelai_health050_test host=/var/run/postgresql sslmode=disable"
cd app-serv && go test -race -tags=integration ./internal/repository/postgres/   # 7.6s, hijau
```

Database aplikasi `pannelai` dan database proyek lain di host ini tidak disentuh; scratch DB ini saya buat atas izin
eksplisit owner dan masih ada (di-drop dengan `dropdb pannelai_health050_test`).

### T7: jendela parkir tidak bisa lagi dipendekkan tulis basi

Sebelum fix, diukur di database nyata:

```
--- FAIL: TestIntegration_RecordKeyHealthNeverShortensAParkedWindow
    park window = 2s, want at least 16s: the 2s write shortened it
```

Itu kalimat ketiga T7 ("dua kegagalan konkuren ... ladder tertinggal satu langkah") dalam bentuk yang bisa
dijalankan: kunci yang diparkir 16 dtk oleh kegagalan keempat dilepaskan lebih dulu oleh tulis kegagalan pertama yang
mendarat belakangan. Perbaikannya satu ekspresi (`endpoint_key_health.go`), mengikuti pola counter yang sudah benar di
baris di atasnya:

```sql
rate_limited_until = CASE WHEN $4::boolean THEN $5::timestamptz
                          ELSE GREATEST(rate_limited_until, $5::timestamptz) END,
```

`$4` adalah flag reset yang sudah dipakai counter (`key.ConsecutiveErrors() == 0`), jadi satu diskriminator melayani
dua kolom: kegagalan **memperpanjang**, sukses **membersihkan**. Test keduanya di-pin, termasuk bahwa sukses masih
membuat kunci `active` dan menolkan counter, supaya monotonicity tidak berubah jadi ratchet.

Yang sengaja **tidak** diperbaiki di grup B, dan alasannya:

- **P4, dua statement tanpa transaksi.** Kerusakan nyata dari race ini (jendela memendek) sudah ditutup; yang tersisa
  adalah tulis parity endpoint yang bisa tertinggal satu langkah di belakang tulis kesehatan kunci, dan kolom itu
  display. Membungkusnya jadi satu transaksi berarti menambah satu metode ke interface repository, ke double `mem`
  di 30 test files, dan ke wiring `cmd`, dan itu refactor yang lebih besar dari manfaatnya sekarang, dan tidak ada
  keputusan owner yang menuntutnya. Kalau nanti grup C mengaktifkan jendela level endpoint, dua tulis itu jadi satu
  transaksi yang sama dan P4 ikut selesai di sana.
- **Urutan sukses-versus-gagal konkuren** masih last-write-wins untuk *status*, bukan untuk jendela. Jendela batasnya
  sendiri (`Available()` membaca jam), jadi terburuk kunci menunggu satu jendela, tidak pernah permanen.
- **`last_error` di level endpoint** tetap tulis absolut (`endpoint_parity.go:33-42,50`). Ia kolom tampilan dan sudah
  di-scrub; tidak ada keputusan routing yang membacanya.

### T5: kredensial mati tidak lagi dicoba setiap 2 menit selamanya

D3 dijawab owner: **(a) jendela Auth bereskalasi**, tanpa status baru. Ladder-nya sekarang dua: 429 (2s ke 5 menit,
tidak berubah) dan kredensial (2m ke 1 jam), dan keduanya memanggil satu helper `backoff(base, n, ceiling)` supaya
guard overflow (`n < 1`, `n > keyLadderMaxN`, `window <= 0 || window > ceiling`) hanya ada satu.

| Kegagalan kredensial berturut | Jendela sebelum | Jendela sesudah |
| --- | --- | --- |
| 1 | 2m | 2m |
| 2 | 2m | 4m |
| 3 | 2m | 8m |
| 5 | 2m | 32m |
| 6 ke atas | 2m | 1h (cap) |

Satu sukses mengembalikan ladder ke langkah pertama, dan itu di-pin terpisah (`TestUpstreamKey_AuthLadderResetsOnSuccess`)
karena perbedaan antara streak dan ratchet justru ada di sana. Transient tetap 30s datar (D3a memilih eskalasi hanya
untuk kredensial), dan test `TestUpstreamKey_TransientStaysFlat` menjaganya supaya "satu helper untuk dua ladder" tidak
diam-diam mengubah 5xx.

Yang T5 **tidak** lakukan, dan ini penting untuk dibaca bersama T2: tidak ada status permanen baru, tidak ada ambang
strike yang menulis `disabled` (kolom itu milik operator, `upstream_key_health.go:89-90`), dan kuburan satu arah T2
masih utuh. Yang berubah adalah interval percobaan ulang, jadi akun mati tetap dicoba, hanya jauh lebih jarang.
Keputusan "siapa yang membangunkan `error` di level endpoint" masih D3 untuk T2/T6, dan itu grup C.

### Berkas yang berubah di grup B

| Berkas | Baris | Catatan |
| --- | --- | --- |
| `internal/repository/postgres/endpoint_key_health.go` | 58 | GREATEST untuk jendela; doc block dipangkas (lihat di bawah) |
| `internal/repository/postgres/endpoint_key_health_integration_test.go` | 229 | dua test baru + helper `healthKeyAt`/`readHealthKey` |
| `internal/domain/upstream_key_health.go` | 139 | `backoff` bersama, `keyAuthBase`/`keyAuthMax`, `keyRateLimitMaxN` jadi `keyLadderMaxN` |
| `internal/domain/upstream_key_ladder_test.go` | 108 | **baru**, hasil pecahan: berkas test sebelumnya mencapai 247 dari 250 (§1.1) |
| `internal/domain/upstream_key_health_test.go` | 156 | setelah pecah |
| `docs/SPEC-API/001-SPEC-API.md` | | §7.5 ditulis ulang dua kali + dua entri changelog |

### Gerbang, dan satu tangkapan yang layak dicatat

`go test -race -count=1 ./...` bersih, suite integrasi hijau dengan `-race`, `go-lint` / `go-headers` /
`contract-drift` / `contract-openapi` / `secrets` PASS.

`antislop` sempat FAIL pada pass ini, dan terhadap saya sendiri:

```
FAIL app-serv/internal/repository/postgres/endpoint_key_health.go has a 19-line doc block
```

Saya menulis tiga paragraf alasan untuk satu ekspresi SQL, persis pola "over-explained comment" (fakta satu baris
ditanam di bawah rantai penalaran). Perbaikannya bukan memangkas sampai di bawah ambang 16, tapi memangkas ke satu
baris per fakta; fakta "GREATEST mengabaikan sisi NULL" saya pindah ke sebelah SQL tempat pembacaan terjadi, karena
di sana ia mencegah orang menambahkan COALESCE yang tidak perlu. Berkas test baru juga dibuat karena §1.1, bukan
karena selera: 247 dari 250.

## Hasil grup C (sebagian), 2026-10-13

Grup C berhenti di tengah dengan sengaja: empat butirnya ternyata adalah keputusan, bukan bug, dan tiga di antaranya
ternyata **sudah di-pin sebagai perilaku yang diinginkan**. Yang dikerjakan adalah yang tidak punya konflik seperti
itu.

### T6 (guard kunci): landed

Aturan "endpoint harus menyisakan ≥1 kunci" sekarang menghitung kunci yang **tidak di-disable operator**, bukan
kunci ber-status `active`. Alasannya diukur lebih dulu dengan test merah:

- `TestUpstreamEndpoint_RemovingOneOfTwoRecoveredKeysIsAllowed`: dua kunci parked, jendela lewat, menghapus satu
  ditolak hari ini (`CONFLICT: this endpoint must keep at least one active key`). Status `error` yang sudah pulih
  tidak terlihat oleh guard.
- Test kedua saya sempat salah, dan kesalahan itu menemukan harm yang laporan awal tidak menyebut: dengan guard
  berbasis status, **endpoint yang semua kuncinya sedang parked tidak bisa dikelola sama sekali** (hapus maupun
  disable ditolak), dan grup B memperpanjang jendela itu sampai 1 jam.

Kenapa tidak pakai `Available(now)` saja, walaupun itu jawaban yang lebih tampak benar: `Available()` untuk kunci
`error` dengan `rate_limited_until = NULL` adalah **false** (`upstream_key_health.go:96-99`: tanpa jendela, kunci
hanya tersedia bila statusnya active). Guard berbasis `Available` jadi tetap membekukan manajemen kunci selama park
berlangsung, yang justru harm kedua di atas. Park itu sementara, dan ia selesai sendiri; tidak ada alasan guard
memperlakukannya seperti kredensial yang dimatikan.

Batasan yang tetap: `ActiveKeyCount()` di respons publik (`handler/endpoint_response.go:70`, field
`active_key_count`) **tidak diubah**, karena ia menjawab pertanyaan tampilan ("berapa kunci yang hari ini aktif")
dan ia berdampingan dengan `available` pada respons yang sama. Tidak ada bentuk wire yang berubah
(`contract-drift` PASS). Pesan refusal ke klien juga tetap (§1.3), jadi §7.5 sekarang berkata "not disabled" sementara
pesan berkata "active"; itu tercatat di changelog spesifikasi sebagai gap kata, bukan sebagai perubahan copy.

### T1 dan T2 (klaim cooldown): komentar dan peta dibuat akurat

Komentar `quota_published.go:103-107` dan `SYSTEM_MAP.md:544` keduanya menyatakan data plane menulis deadline ke
`upstream_endpoints.rate_limited_until`. Tidak ada jalur Go yang menulisnya. Keduanya sekarang menyatakan yang
sebenarnya: filter poller **inert**, bandingannya selalu terhadap NULL, dan menyambungkan tulisannya (atau menghapus
filternya) adalah T1 yang masih menunggu keputusan. Tidak ada perilaku yang berubah di sini.

### Yang ternyata keputusan, bukan bug (butuh putusan owner)

- **T1 tulis deadline level endpoint.** `UpstreamEndpoint.Available()` (`upstream_endpoint.go:143`) sudah membaca
  kolom itu. Jadi menulis satu deadline non-NULL **langsung mengaktifkan skip level akun** di router: satu akun yang
  429 hilang dari rotasi untuk seluruh jendela, bukan hanya satu kunci. Itu breaker yang lebih benar secara model,
  dan sekaligus perubahan perilaku routing dengan blast radius satu akun penuh. Butuh keputusan, bukan asumsi.
- **T2 kuburan `error` level endpoint.** Temuan baru (**P5**): `oauth_refresh_worker.go:96` melewati endpoint dengan
  `status != active`, dan satu-satunya penulis `UpstreamEndpointActive` di domain adalah konstruktor
  (`upstream_endpoint.go:82`). Artinya akun yang di-dead-letter oleh OAuth **tidak ikut sweep yang bisa
  memulihkannya**, dan tidak ada sukses yang membalik statusnya: keluar dari kuburan hanya lewat PATCH operator.
  Pilihan yang saya lihat: (i) refresh sukses membalik `error → active`, (ii) sweep tetap menyertakan `error`,
  (iii) jendela + eskalasi seperti grup B untuk level endpoint, (iv) terima sebagai handal.
- **T6 sisi probe.** `TestEndpointService_TestRefusesWithoutAUsableCredential` (`endpoint_probe_selection_test.go:79-95`)
  sengaja mem-pin bahwa endpoint yang kunci satunya berstatus `error` **menolak** tombol Test, dengan komentar yang
  beralasan. Dok `selectProbeKey` (`endpoint_probe_use.go:107-113`) menyatakan hal sebaliknya: menguji kunci yang
  diparkir "exactly what an operator does after fixing a credential". Dua pernyataan itu bertabrakan, dan yang satu
  punya test. Saya tidak membalik keputusan yang di-pin tanpa izin.
- **T9 probe menulis kesehatan.** `TestEndpointService_TestParksOnFirstFailure` (`endpoint_probe_test.go:174-213`)
  mem-pin probe 401 memarkir kunci 2 menit **dan** mem-pin bahwa probe sukses berikutnya memulihkannya. Jadi parkir
  itu adalah cara operator mengumumkan "kredensial sudah saya betulkan". Efek sampingnya sejak grup B: kegagalan
  probe ikut menaikkan anak tangga ladder kredensial (2m → 4m → 8m …), karena ia menambah `consecutive_errors` sama
  seperti trafik nyata. Butuh D4 dengan fakta ini di atas meja.

### Dampak lingkungan yang perlu diketahui owner

`SYSTEM_MAP.md` **bukan UTF-8 valid**: 48 tempat di file itu menyimpan panah sebagai `e2 86 3f` (tiga byte `→` dengan
byte ketiga terganti `?`), yang menjelaskan kenapa `grep` treat file itu sebagai binary sejak pencarian pertama.
Ini hasil konversi lossy yang sudah ada sebelum pass ini (saya tidak menyentuh byte-byte itu; edit saya lakukan pada
level byte agar tidak mengarang ulang file). Cara memperbaikinya jelas (`\xe2\x86?` → `\xe2\x86\x92`) dan bisa
dibuktikan dengan `python3 -c "open('SYSTEM_MAP.md','rb').read().decode('utf-8')"` yang harusnya bersih, tapi ini
dokumen governance dengan 61 KB isi, jadi saya laporkan, bukan saya betulkan diam-diam.

## Grup C selesai, dengan satu keputusan owner yang harus diundur

Keputusan owner 2026-10-13: T2 = (ii) sweep sertakan `error` + (i) refresh sukses membalik status; T1 = (b) kolom
terpisah untuk poller; probe = (b) probe tetap menulis kesehatan tapi `selectProbeKey` dilonggarkan.

### Yang landed

- **T6 (guard ≥1 kunci)** dan **T6/T9 sisi probe.** `selectProbeKey` sekarang menolak hanya kunci `disabled`. Bukan
  perubahan arah: dok fungsi itu dan dok berkas test-nya sudah menyatakan "a key parked inside its circuit-breaker
  backoff is exactly what a manual test is for", sementara kodenya menyaring `Status() != active`. Sub-case yang
  mem-pin penolakan itu ditulis ulang ke bentuk yang sebenarnya ia maksud (kunci `disabled`), plus case baru bahwa
  kunci `error` tetap dites. Pesan refusal ikut ("no enabled key to test") dan tidak ada satu pun test, UI, atau
  kontrak yang memegang teks lama.
- **T2 sebagian: jalur keluarnya nyata sekarang.** `MarkHealthy` (domain, hanya `error → active`, tidak pernah
  membangunkan `disabled`) dipanggil di `refreshEndpoint` saat grant vendor diterima. Karena `sweepProvider` (jalur
  `POST /oauth/refresh`, menyaring hanya pada `RefreshDue`) sejak awal memang mencapai akun `error`, satu tombol
  refresh provider kini memulihkan akun yang di-dead-letter. Test: `TestOAuthFlowRefreshRecoversADeadLetteredAccount`
  dan tabel `TestUpstreamEndpoint_MarkHealthyReturnsOnlyFromError`.
- **T1/T2 komentar dan peta** sudah akurat (bagian sebelumnya). §7.5 amended dua kali (invariant ≥1 kunci, aturan
  target probe), `contract-drift` dan `contract-openapi` PASS.

### T2 (ii) diukur salah, dan saya undur

Sweep worker saya lebarkan ke `status != disabled` sesuai keputusan (ii). Hasilnya satu test lama gagal, dan test itu
bukan collateral: `TestOAuthRefreshWorkerRetriesThenDeadLetters` mem-pin bahwa worker berhenti mencoba setelah
dead-letter (`grant calls = 6, want exactly the 5 attempts`). Setelah dibaca ulang, pin itu benar dan (ii) salah
dengan cara yang lebih buruk dari kuburan yang ia gantikan: token hasil dead-letter selalu expired, jadi ia selalu
`RefreshDue`, jadi worker yang menyertakan `error` akan mengulang **seluruh budget 5 attempt setiap tick** terhadap
vendor yang sudah menjawab `invalid_grant`, lalu menuliskan dead-letter baru. Filter worker dikembalikan ke
`status == active`, dengan alasan itu tertulis di sana, dan keluar dari kuburan lewat jalur refresh operator.

Sisa dari (ii) yang tidak bisa dikerjakan tanpa keputusan lagi: "retry dead-letter pada schedule yang melebar"
(bentuknya sama dengan ladder grup B: jendela per akun, dibaca `Available()`), dan itu bukan satu baris. Ia butuh
kolom waktu-coba-berikutnya di level endpoint, yang tumpang tindih dengan keputusan T1(b) di bawah.

### T1(b) belum dikerjakan, dan ini alasan bentuknya

Kolom terpisah untuk poller (`quota_cooldown_until`, tulis oleh parity endpoint saat kelas 429) adalah migration +
satu kolom di agregat + jalur tulis di `endpoint_parity.go` + filter `quota_published.go` yang dibaca ulang, plus
`SYSTEM_MAP.md` dan §7.5. Tidak ada keputusan yang terbuka (arahnya sudah Anda pilih), hanya sisa pekerjaan: ia
sengaja tidak saya kerjakan di pass yang sama dengan empat item lain, karena ukurannya berbeda kelas dari yang lain
di grup ini.

### Gerbang dan ukuran berkas

`go test -race -count=1 ./...` bersih, suite integrasi `-race` hijau, `go-lint` / `go-headers` / `antislop` /
`contract-drift` / `contract-openapi` / `secrets` PASS.

Satu tangkapan lagi, dan kali ini terhadap prediksi saya sendiri: menambah dua test ke `oauth_refresh_worker_test.go`
(194 baris) membuatnya **264**, dan `go-lint` menolaknya sesuai §1.1. Test pemulihan pindah ke berkas baru
`oauth_refresh_recovery_test.go` (62), berkas asal kembali 222. Draft ini sudah memperingatkan pola ini ("berkas baru
lebih baik daripada menambah ke yang sudah mendekati 220") dan saya tetap melakukannya.

Yang mendekati batas setelah pass ini, untuk siapa pun yang melanjutkan: `upstream_endpoint_keys.go` 209,
`oauth_refresh_worker_test.go` 222, `endpoint_probe_use.go` 181, `upstream_key_health_test.go` 156 +
`upstream_key_ladder_test.go` 108.
