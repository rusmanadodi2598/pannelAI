# 047-GATE-FAIL-OPEN-AND-QODER-CATALOG-ISOLATION.md: Gerbang melaporkan PASS ketika ia tidak membaca apa pun, dan katalog Qoder di-key tanpa akun

Laporan owner 2026-10-08: hasil review atas PR #2 dan #3 disusun ulang menjadi empat prioritas, dan minta
dikerjakan dalam tiga grup (a) gate dan CI, (b) data OAuth dan katalog Qoder, (c) sisanya. Permintaan pertama adalah
memverifikasi setiap klaim sebelum mengerjakannya, karena sebagian dari daftar itu sudah salah dan sebuah fix akan
menghapus keputusan yang sengaja dibuat.

Pass ini memverifikasi 17 klaim, membatalkan 4 di antaranya dengan bukti, dan mengerjakan 13 sisanya.

## Status dan lingkup

| | |
| --- | --- |
| **Status** | **Ketiga grup landed. Grup A (F1, F4, F7, F8, F11) `07b2497`. Grup B (F2, F3, F5, F6, F9, F10) `d5b8de7`. Grup C (F12, F13, F14, F15).** Semua temuan di bawah terverifikasi terhadap kode, dan untuk F1, F2, F3, F4, F7 dan F9 dibuktikan dengan menjalankan kodenya: merah sebelum perbaikan, hijau sesudah. Dua butir review dibatalkan dan alasannya ada di bagiannya sendiri |
| **Mechanism** | DURING: tulisan baru mengikuti R-02 dan R-31, dan setiap klaim harus bisa dibuktikan oleh perintah yang tertulis di dokumennya sendiri |
| **Scope** | **Bukan comment-only.** F1 sampai F3 menyentuh `scrypts/`, `app-serv` repository, service dan provider. Keluar dari guardrail antislop-code, atas izin eksplisit owner, seperti F1 dan F2 pada 046 |
| **Sumber temuan** | Daftar review owner, lalu pengukuran terhadap pohon kode, gerbang yang dijalankan dengan `GATES_BASE_REF` rusak, `go run` untuk semantik `net/url` dan `fmt`, dan `git show ab1a4c4^` untuk test yang hilang |
| **Struktural (AGENTS.md §1.9)** | `SYSTEM_MAP.md`: **tidak N/A.** F2 mengubah kolom yang ditulis satu jalur tulis, F3 mengubah cakupan cache per akun, dan F6 menambahkan satu field ke respons publik. Ketiganya dicatat ke `SYSTEM_MAP.md` pada commit yang mengubahnya. F10 bergerak di semantik satu route tanpa mengubah bentuknya |

## Diagnosa

Satu tema menghubungkan temuan yang paling serius di daftar ini: **sebuah pemeriksaan yang tidak membaca apa pun
melaporkan hasil yang sama dengan pemeriksaan yang membaca segalanya dan menemukan tidak ada.**

Gerbang `changed_files` memang mengembalikan galat ketika `GATES_BASE_REF` tidak resolve (di `scrypts/lib/common.sh`),
dan menulis sebabnya ke stderr. Dua konsumennya memanggilnya di dalam process substitution:

```bash
# bentuk sebelum perbaikan, di dalam check_changed_line_limits (scrypts/gates/go-lint.sh)
done < <(changed_files)
# dan di dalam changed_text_paths (scrypts/gates/antislop.sh)
comm -12 <(text_paths | sort -u) <(changed_files | sort -u)
```

Bash tidak mewarisi exit code dari process substitution, dan `set -euo pipefail` tidak menjangkaunya karena perintah
itu bukan bagian dari pipeline maupun list. Ini bukan pembacaan yang bisa diperdebatkan; ini semantik shell yang bisa
dijalankan:

```
$ cat /tmp/failopen-test.sh          # pola yang sama, perintah stand-in yang mengembalikan 1
--- A: process substitution (go-lint.sh:63) ---   loop saw 0 file(s); rc after loop = 0
--- B: comm over process substitution (:206) ---  comm produced [] rc = 0
--- C: direct call for contrast ---               direct call DOES report rc=1
```

Lalu terhadap gerbang sungguhan, dengan `GATES_BASE_REF` yang tidak mungkin resolve:

```
$ GATES_BASE_REF=definitely-not-a-real-ref-123 bash scrypts/gates/go-lint.sh
gates: GATES_BASE_REF 'definitely-not-a-real-ref-123' does not resolve in this checkout
PASS go vet / PASS gofmt / PASS staticcheck / PASS golangci-lint
exit 0                                  <-- line-limit check tidak membaca satu file pun

$ GATES_BASE_REF=bogus-ref-xyz bash scrypts/gates/antislop.sh
gates: GATES_BASE_REF 'bogus-ref-xyz' does not resolve in this checkout
==> scratch-work citations (changed files)
PASS antislop gate
```

Yang membuat yang kedua lebih buruk dari sekadar lolos: `scope` menjadi kosong, dan cabang kosong itu mencetak
`no changed text files; set GATES_BASE_REF to check a diff range` (cabang kosong `report()` di
`scrypts/gates/antislop.sh`). Padahal `GATES_BASE_REF` sudah di-set; yang benar adalah bahwa nilainya tidak bisa
dibaca. Pesan itu mengarahkan pembaca ke tempat yang salah,
persis seperti "move or move its endpoints first" pada 046 F1.

CI sudah menutup satu kasus serupa, dan itu menunjukkan bahwa bentuk kegagalannya sudah pernah dipikirkan sekali:
langkah "Resolve the revision the gates diff against" di `gates.yml` menolak push yang tidak punya revisi di
belakangnya dengan `exit 1`. Yang
tersisa adalah sisi yang sama, di tempat lain.

## Keputusan desain

**Sebab yang tidak terbaca adalah kegagalan, bukan hasil kosong.** Untuk gerbang yang bekerja pada satu set berkas,
set yang tidak bisa dihitung berarti gerbang itu tidak menjalankan aturannya sama sekali, jadi jawabannya fail, bukan
skip. Ini mengikuti aturan yang sudah ditulis `scrypts/lib/gate-scope.sh:9-19` untuk routing: "A gate that reports a
pass after inspecting nothing is the failure this file exists to avoid." Routing sudah menerapkannya; pemeriksaan
berbasis berkas belum.

**Perbaikan F2 tidak boleh menyentuh statement bersama.** `updateEndpoint` dipakai tiga jalur (`Update` dan
`UpdateIfUnchanged` di `endpoint.go`, serta `ImportOAuthBatch` di `endpoint_oauth_batch.go`), dan untuk
`ImportOAuthBatch` serta `Update` menulis baris penuh memang benar: import memiliki seluruh bentuk baris itu. Yang
salah hanya jalur yang memuat agregat lalu menulisnya kembali. Karena itu kolom token mendapat statement sendiri, dan
`loaded != nil` memilih statement, bukan statement tunggal yang ikut berubah untuk semua pemanggil.

**Key cache per akun memakai identitas, bukan secret.** `Credential.EndpointID()` sudah ada dan merupakan pengenal
stabil. Memasukkan materi credential sebagai key map
menyimpan secret di struktur data yang hidup satu jam demi sebuah nilai yang sudah tersedia lewat kolomnya sendiri.
Repo ini sudah memutuskan hal yang sama di tempat lain: `qoder_identity.go:44` men-key cache identity dengan digest,
bukan dengan bearer mentah.

## Temuan

Klaim owner diberi verdict di judulnya. **Benar** berarti dikerjakan. **Dibatalkan** berarti kodenya sudah benar dan
perubahan yang diusulkan akan merusak sesuatu yang sengaja.

### F1 (HIGH) Benar: `changed_files` gagal, gerbang melaporkan PASS

`changed_files` di `scrypts/lib/common.sh` mengembalikan 1 untuk base ref yang tidak resolve.
`check_changed_line_limits` di `go-lint.sh` dan `changed_text_paths` di `antislop.sh` menelannya lewat process
substitution. Akibat terukur ada di Diagnosa: `go-lint.sh` exit 0,
`antislop.sh` mencetak `PASS antislop gate`.

Diperbaiki dengan capture output ke variabel, periksa exit code, lalu fail. Satu helper di `common.sh` lebih baik dari
dua pemeriksaan ad hoc, karena konsumennya akan bertambah. Pesan skip yang menuduh `GATES_BASE_REF` belum di-set ikut
dikoreksi: bedakan "memang tidak ada diff" dari "diff-nya tidak bisa dibaca".

Bukti selesai: `GATES_BASE_REF=bogus-ref-xyz` harus menghasilkan exit non-nol pada kedua gerbang, dan `gate-scope-test.sh`
tetap hijau.

### F2 (HIGH) Benar: refresh OAuth menulis 16 kolom, CAS hanya menjaga dua ciphertext

`refreshEndpoint` memanggil `SetOAuth` lalu `store.UpdateIfUnchanged`. Jalur itu jatuh ke `updateEndpoint`
(`endpoint_oauth_batch.go`) yang SET-nya mencakup `label`, `priority`, `status`,
`test_status`, `rate_limited_until`, `global_priority`, `default_model`, `consecutive_use_count`, `last_error`,
`last_error_at`, `error_code`, `proxy_pool_id`. Guard di `updateEndpoint` hanya membandingkan
`oauth->>'access_token_encrypted'` dan `oauth->>'refresh_token_encrypted'`.

Jadi selang antara `listOAuthEndpoints` dan tulisannya: operator mengubah `status` atau
`priority`, oauth tidak berubah, CAS lolos, dan nilai basi dari agregat yang dimuat sebelum perubahan itu tertimpa.
Lost update nyata, dan persis pada kolom yang keputusan review sebut.

Perilaku yang dipertahankan: `updated_at` tetap ditulis, dan dua ciphertext pembanding tetap menjaga rotasi yang
berlomba, karena alasan itu ada di komentar `UpdateIfUnchanged`.

### F3 (HIGH) Benar: katalog Qoder tidak terisolasi per akun, dan error satu akun di-delivery ke akun lain

Satu connector dibuat per registry entry (`qoder.go:60`, `catalog: newQoderCatalog()`), dan key lookup adalah
`base + "|" + modelKey`. `base` dari `inferenceBase` hanya bisa dua nilai: origin device atau origin job. Tidak ada
identitas akun di key mana pun.

Tiga konsekuensi, semuanya nyata di kode hari ini:

1. `entries` menyajikan katalog yang dipelajari atas nama akun A ke akun B. Ini bertentangan dengan baris `@for`
   file itu sendiri: "the model configuration Qoder publishes to an authenticated account".
2. `inflight` membuat satu fetch per host. Waiter yang masuk lewat cabang non-leader `beginFetch` mengembalikan
   `fetch.err` milik leader apa adanya, jadi 401 akun A menjadi galat akun B.
3. `misses` menolak nama model selama `qoderCatalogMissTTL`, 60 detik, berdasarkan jawaban vendor kepada akun lain,
   jadi model yang sebenarnya ada di akun B tetap ditolak tanpa pernah ditanyakan.

Komentar di atas `qoderCatalogEntry` menyatakan "the widest scope that is still correct" untuk key host-plus-model.
Klaim itulah yang salah, dan inkonsistensi internalnya yang menjadi bukti terkuatnya: cache identity di file
sebelahnya sudah di-key per credential.

Perbaikan mencakup ketiga map dengan key yang sama, plus galat yang tidak boleh berpindah antar akun.

### F4 (MEDIUM) Benar: `cite_warn` adalah counter, dibandingkan dengan `= "1"`

`report()` di `antislop.sh` menaikkan `cite_warn` per berkas dan men-set `cite_fail` ke 1 (boolean, bukan counter),
lalu membandingkan `cite_warn` dengan `= "1"`. Logikanya dijalankan sendiri:

```
cite_warn=1 -> gate_skip: warnings require review
cite_warn=3 -> gate_pass "scratch-work citations (changed files)"     <-- salah
```

Tiga berkas berubah yang masing-masing membawa sitasi terbaca sebagai bersih. `cite_fail` pada cabang sebelumnya benar
adanya dan tidak disentuh, karena nilainya hanya 0 dan 1.

### F5 (MEDIUM) Benar: tepat dua test integrasi `PageAccountsByProvider` hilang

Keduanya dihapus `ab1a4c4` saat file ditulis ulang:
`TestQuotaRepository_PageAccountsIncludesAnAccountWithNoWindows` dan
`TestQuotaRepository_PageAccountsIsCountPlusPageOnly`. Versi hari ini hanya menyisakan
`TestQuotaRepository_PageWindowsByProvider_GroupsAndCounts` di `quota_paging_integration_test.go`. Yang tersisa untuk
fungsi itu hanyalah tiga stub `PageAccountsByProvider`: di `handler/quota_account_stub_test.go`,
`service/quota_test.go`, dan `quota_flush_fixture_test.go`. SQL-nya tidak pernah dijalankan ke server nyata.

Ini port, bukan revert. Header versi lama melanggar §2.7 (nilai `@reason` lanjut ke baris ber-indent) dan §2.1
(mengandung satu em dash), jadi badan testnya saja yang pindah ke header yang sudah patuh.

Harness yang dibutuhkan masih hidup, dan itu membuat F5 murah: `newPublishedRepo(t)` mengembalikan
`(*PublishedQuotaRepository, *statementCounter)` (`quota_published_harness_integration_test.go:88`),
`seedPublishedEndpoint` ada di `:119`, dan `statementCounter` masih punya `reset`/`load`/`trace` (`:60`).

### F6 (MEDIUM) Benar: `LIMIT 1000` memotong tanpa sinyal ke klien

`quotaMaxRowsPerPage = 1000` di `quota_paging.go` dipakai sebagai `LIMIT $3::int`. `total` yang dikembalikan adalah
jumlah grup provider, jadi tidak menggambarkan pemotongan baris sama sekali. Kode itu sendiri mengakuinya: "A group
beyond the ceiling is served in key order and truncated".

Tidak ada tempat untuk membaca pemotongan itu: `schema.Page` hanya `page`, `per_page`, `total` (`dto.go:37-41`), dan
`QuotaHandler.List` membuang total akun (`accounts, _, err :=`). Nuansa yang harus ikut tercatat: ini ceiling baris
per halaman, bukan page size, karena paging-nya per grup.

### F7 (MEDIUM) Benar sebagian: alasan sebuah suppression tidak pernah benar-benar dituntut

Klaim owner berbunyi "mewajibkan nama rule dan alasan setelah `--`". Nama rule sebenarnya sudah dituntut, tetapi
hanya dalam bentuk plugin-qualified (`[^[:space:]]*/`), dan itu bukan hiasan: ia yang menjaga baris prosa
`eslint.config.js:47`, `// eslint-disable comment is an error rather than a warning.`, agar tidak dibaca sebagai
direktif. Yang benar-benar hilang adalah separuh alasan. Pemeriksaan lama mengecualikan apa pun yang memuat ` -- `
di mana pun:

```bash
unexplained_panel="$(tree_scan "$PANEL_SUPPRESS_RE" | grep -v ' -- ' || true)"
```

Dua lubang yang diukur, bukan dugaan. `git grep` menerapkan pola ke isi berkas lalu memberi prefix `file:line:` pada
keluarannya, sehingga `^` anchor ke konten dan pemeriksaan ini memang hidup (11 direktif tertangkap, dan itu sudah
dikonfirmasi sebelum mengubah apa pun). Terhadap keluaran yang hidup itu:

1. `// eslint-disable-next-line rule/name -- ` dengan alasan kosong lolos, karena ` -- ` hadir meski tidak mengikuti
   apa pun.
2. Yang lebih buruk dan tidak disebut review: pada `.svelte`, penutup komentar `-->` menyuplai `--` sendiri, sehingga
   `<!-- eslint-disable-next-line svelte/no-x -- -->` terbaca seolah beralasan. Ini false negative nyata pada bentuk
   yang paling mungkin ditulis orang saat lupa mengisi alasan.

Keduanya diuji sebagai fixture sebelum dan sesudah:

```
<!-- eslint-disable-next-line svelte/no-x -- -->      sebelum: lolos   sesudah: dilaporkan
11 direktif nyata di app-ui                            sebelum: lolos   sesudah: tetap lolos (tidak ada false positive)
```

Bagian "nama rule" diperlakukan sebagai pemeriksaan terpisah, dan hanya pada dua bentuk yang tidak bisa tertukar dengan
prosa: keyword yang mengakhiri baris, dan keyword yang langsung menuju separator. Direktif `// eslint-disable -- reason`
yang menonaktifkan semua rule, yaitu bentuk paling berbahaya dan selama ini tidak terlihat oleh pola mana pun, sekarang
gagal. `// eslint-disable comment is an error...` tetap tidak tersentuh, karena ada token setelah keyword.

### F8 (MEDIUM) Benar: supply chain CI memakai `@latest` dan tag, tanpa verifikasi checksum

Terkonfirmasi persis seperti klaim, dengan nilai yang sudah diambil dari upstream (bukan angka karangan):

| Sekarang | Di-pin ke |
| --- | --- |
| `actions/checkout@v4` (langkah checkout di `gates.yml` dan `gates-frontend.yml`) | `11d5960a326750d5838078e36cf38b85af677262` (v4.4.0) |
| `actions/setup-go@v5` (langkah setup Go di `gates.yml`) | `40f1582b2485089dde7abd97c1529aa768e1baff` (v5.6.0) |
| `staticcheck@latest` (langkah `Install the linters AGENTS.md §1.4 names`) | `v0.8.1` |
| `golangci-lint/v2@latest` (langkah yang sama) | `v2.9.0` |
| gitleaks 8.30.1 tar.gz (langkah `Install gitleaks`) | `551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb` |
| bun 1.3.0 baseline zip (langkah `Install the pinned Bun`) | `77336611905b9e876e52924f8b1a57e72669cf10541dc1e11269d2c9371f9e45` |

Kedua SHA action diverifikasi sebagai commit nyata lewat `GET /repos/{repo}/git/commits/{sha}`, dan sha teratas pada
respons cocok dengan yang diminta. Kedua file checksum upstream tersedia (gitleaks `gitleaks_8.30.1_checksums.txt`,
bun `SHASUMS256.txt` pada release `bun-v1.3.0`), jadi verifikasinya bisa dilakukan, tidak perlu di-skip.

Dua keputusan yang perlu disebut karena mereka bukan tanpa konsekuensi. Pin versi linter diambil pada versi terbaru
hari ini, supaya `@latest` diganti oleh nilai yang sama dan hasil gerbang tidak berubah di commit yang sama; bila
`v2.9.0` atau `v0.8.1` ternyata menemukan sesuatu yang baru di pohon ini, itu tercatat sebagai hasil, bukan
dibungkam. Dan `.golangci.yml` hari ini memakai `version: "2"` sebagai schema, bukan sebagai pin biner; konfigurasi
itu tidak berubah oleh F8.

Langkah `Install gitleaks` di `gates.yml` sudah menuliskan alasan yang bagus untuk mem-pin versi gitleaks (8.24.3 vs
8.30.1 menandai fixture yang berbeda). F8 menambah integritas di atas pin yang sudah bermakna itu, tidak menggantinya.

### F9 (LOW) Benar, dan naiknya kategori: `originOf` panic pada URL yang gagal di-parse

Klaim owner menempatkannya di prioritas "bersih-bersih". Ia bukan kosmetik. `originOf` menulis:

```go
parsed, err := url.Parse(rawURL)
if err != nil || parsed.Host == "" {
    return strings.TrimSuffix(rawURL, parsed.Path)   // :183
}
```

`net/url` mengembalikan `nil` bersama galat untuk sejumlah bentuk, dan cabang `err != nil` di atas masuk ke baris yang
membaca `parsed.Path`. Dijalankan terhadap `originOf` yang sama persis:

```
in="%zz"                  parsedNil=true   -> PANIC: invalid memory address or nil pointer dereference
in=":::"                  parsedNil=true   -> PANIC
in="http://[::1]:9x/"     parsedNil=true   -> PANIC
in="https://exa mple.com/p" parsedNil=true -> PANIC
```

Trigger-nya `entry.Transport.BaseURL` registry yang salah tulis, bukan input attacker, jadi ini bug robustness dan
bukan jalur eksploitasi. Yang membuatnya layak naik: `inferenceBase` dipanggil di `modelConfig`, sebelum
`beginFetch`, sehingga ia berada di jalur shaping request dan `runFetch` tidak memulihkannya. AGENTS.md
§1.6 melarang panic yang tidak dipulihkan bertahan, dan di sini ia bisa menghentikan proses untuk alasan satu karakter
salah ketik.

### F10 (LOW) Benar: satu akun yang konflik membatalkan seluruh batch refresh

`sweepProvider`, waktu itu masih bagian dari `Refresh`, mengembalikan galat langsung dari dalam loop, jadi
`outcome.Refreshed` dan `outcome.EndpointIDs` yang sudah terkumpul dibuang. Akun yang konflik adalah akun yang
refresh-nya kalah balapan, bukan akun yang membuat batch lain tidak boleh berhasil.

Perilaku yang harus dipertahankan: conflict tetap terlihat oleh operator sebagai hitungan yang bisa dibedakan, bukan
diam-diam hilang, karena `MarkRefreshDeadLetter` sudah menjadi jalur terminal untuk kegagalan yang lain.

### F11 (LOW) Benar: `<!--` dibaca untuk berkas non-svelte, dan locale tidak pernah di-set

`fe_body_comments` di `antislop.sh` memproses `<!--` dan `-->` untuk semua ekstensi yang memanggilnya, yaitu
`ts | tsx | js | mjs | svelte`. Di `.ts` dan `.js`, `<!--` bukan konstruk komentar. Arah kesalahannya
false positive, bukan lolos-check, jadi ini kebersihan, bukan lubang.

`LC_ALL` dan `LANG` tidak muncul di satu pun berkas `scrypts/` (digrep, nol hasil). Pemeriksaan emoji di `report()` dan
regex byte-oriented lainnya bergantung pada bagaimana locale menafsirkan byte, sehingga hasilnya bisa berbeda antara
mesin developer dan runner. Satu `export LC_ALL=C.UTF-8` membuat keduanya deterministik.

### F12 (LOW) Premis klaimnya salah, bentuk fix-nya benar, dan lubang sebenarnya adalah `%#v`

Klaim owner: "`Credential` tidak punya `GoString()` dan `LogValue()` sehingga secret bisa bocor ke log."

Yang benar: `String()` **sudah ada** di `Credential` dan me-redact lewat `redactedWhenSet`. Yang absen hanya
`GoString()` dan `LogValue()`. Pencarian site kebocoran di kode produksi tidak menemukan apa pun: nol `%+v` dan
nol `%#v` ke `Credential`, nol `slog.Any` di produksi, dan enam pemanggil `Secret()`
(`provider/default.go:117`, `qoder.go:143`, `opencode_auth.go:37`, `dataplane/media.go:179`,
`service/systemone_target.go:59`) semuanya hanya mengisi header.

Yang membuat butir ini tetap masuk daftar, dan yang membuat bentuk fix review justru benar: `%#v`.

Probe pertama saya, terhadap tipe yang saya karang sendiri, menyimpulkan `String()` dilewati untuk field nested
bahkan pada `%v`. Kesimpulan itu salah, dan salah karena probe-nya: field pada struct pembawa saya pakai
unexported, dan `fmt` tidak bisa memanggil method pada `reflect.Value` read-only. Bentuk produksi berbeda.
Diukur ulang terhadap `dataplane.Call` dan `dataplane.Selection` yang sebenarnya:

```
%v   on Call      : redacted   (String() dihormati, field-nya exported)
%+v  on Call      : redacted
%#v  on Call      : bocor      <-- satu-satunya verb yang menembus String()
%v   on unexported: bocor      <-- tidak ada struct produksi yang bentuknya begini
```

Verifikasi terakhir itu yang menentukan: `grep` atas semua field bertipe `provider.Credential` menemukan dua di kode
produksi, field `Credential` pada `dataplane.Call` dan `dataplane.Selection`, keduanya exported, dan dua sisanya hanya
ada di test double. Jadi `%v` dan `%+v` sudah aman hari ini; yang terbuka cuma `%#v`, dan `%#v` dibentuk
oleh refleksi atas field, bukan oleh method milik fieldnya, sehingga `GoString()` pada `Credential` yang
menutupnya, bukan `String()` pada `Call`.

Bukti dua arah, tanpa menebak: `GoString()` dilepas dari kode yang sudah ada testnya, dan kasus `%#v` pada kedua
tipe nyata langsung menulis plaintext; dipasang lagi, keduanya hijau.

```
--- FAIL: TestCarriedCredentialStaysRedacted/%#v_on_a_Call
        %#v on a Call wrote the plaintext: dataplane.Call{... apiKey:"pt-SUPERSECRET-material" ...}
--- FAIL: TestCarriedCredentialStaysRedacted/%#v_on_a_Selection
```

Komentar di atas field `Credential` pada `Selection` tetap ikut dikoreksi, tapi bukan karena ia salah klaim. Ia benar
bahwa selection tidak membocorkan apa pun; yang salah adalah alasannya, "assembled for exactly one request and never stored".
Seumur hidup pendek sebuah nilai tidak ada hubungannya dengan apa yang dicetaknya, dan pembaca berikutnya bisa
saja menghapus method redaksi dengan percaya alasan itu. Alasannya diganti:
`Credential` me-redact dirinya sendiri di setiap verb fmt dan di slog.

`LogValue()` bukan penutup kebocoran dan tidak dijual begitu. Tanpa method itu slog merefleksi struct, hanya
melihat field unexported, dan menulis objek kosong: tidak bocor, dan juga tidak menyebut akun mana pun, sehingga
baris log yang seharusnya menelusuri satu panggilan kehilangan keduanya. Bentuknya sekarang grup berisi
`endpoint_id`, `key_id`, `account` dan dua penanda redaksi.

Metodenya ditaruh di `plugin_credential_redact.go` baru, bukan di `plugin_credential.go` yang sudah 210 baris,
dan `redactedWhenSet` ikut pindah karena ia bagian dari concern yang sama.

### F13 (LOW) Sebagian benar, dan sebagian berbeda dari yang dinyatakan

Klaim owner: "hentikan HTTP dan drain stream dulu, baru hentikan worker."

HTTP memang di-drain: `serve` di `shutdown_drain.go` memanggil `srv.Shutdown`, dan `run` di `main.go` tidak memasang
`BaseContext`, sehingga context request tidak dibatalkan oleh sinyal dan `Shutdown` menunggu connection yang masih
hidup. Yang salah adalah dua hal lain:

1. Worker tidak pernah di-join. `runWorkers` di `worker_wiring.go` men-spawn `go runSupervised(...)` tanpa WaitGroup,
   jadi tidak ada satu pun titik yang menunggu mereka selesai.
2. Jalur publish keluar lebih dulu dari handler yang masih berjalan. `usage_event_publish.go:123-135` melakukan
   `case <-ctx.Done(): p.drain(ctx); return`, sehingga publisher drain lalu keluar, sementara handler in-flight masih
   merekam event selama jendela `shutdownTimeout` 15 detik. Event itu masuk ke antrian tanpa konsumen
   dan hilang di `:109-111`.

Ada satu kasus yang melebihi jendela secara eksplisit: `/api/v1/usage/live` punya `usageLiveMaxLifetime = 30 *
time.Minute` (`usage_live.go:53`). Stream sepanjang itu tidak akan pernah selesai secara wajar pada shutdown dan akan
dipotong.

Untuk quota flush, jalur normal sudah benar dan sudah didokumentasikan: `(*QuotaFlusher).Run` adalah ticker loop 30
detik, dan `quota_flush_policy.go:26-28` menyatakan BatchSize memang membatasi satu flush supaya backlog habis lewat
beberapa tick. Yang satu kali jalan adalah **drain shutdown**: `quotaDrain` memanggil `FlushOnce` sekali, satu batch
500, sehingga backlog lebih dari itu tertinggal di Redis sampai boot berikutnya. Loop yang benar ada di
drain shutdown, bukan di dalam `FlushOnce`.

### F14 (LOW) Benar: regex geometry longgar, dan path dihitung dari cwd

Assert `border` di `usage-topology-geometry.test.ts` membaca `expect(nodeBlock()).toMatch(/\bborder\b/)`. `-` bukan word
character, sehingga pola itu juga cocok untuk `border-[var(--color-border)]` dan varian `border-b`, padahal yang
dipin adalah utility width polos (`BORDER_ACROSS = 2`; kelas polos itu memang ada di
`UsageTopologyDrawing.svelte:102`). Menghapus utility width sambil menahan satu kelas `border-*` membuat test tetap
hijau, yaitu test yang tidak lagi menguji apa yang ia klaim.

Path: `resolve(process.cwd(), 'src/lib/components/UsageTopologyDrawing.svelte')`. Pola yang sama dipakai
`tests/tokens/contrast.test.ts:146`, jadi ini konvensi repo, tapi ia salah asumsi bila vitest dijalankan dari luar
`app-ui/`.

### F15 (LOW) Benar untuk tiga dari empat dokumen

| Dokumen | Yang basi |
| --- | --- |
| header `antislop.sh` | Masih menulis "Citations are a WARNING, not a failure", padahal cabang sitasi di `report()` sudah `gate_fail` untuk panel dan hanya Go yang warn. Banner `# ---- citations, warning only ----` masih berdiri tepat di atas penggantinya |
| header `panel-check.sh` | "format, type check, tests, build" tidak menyebut langkah eslint dan langkah svelte-kit sync yang keduanya dijalankan |
| `pre-push` | Daftar bernomor 1 sampai 4 tidak mencantumkan gate antislop yang dijalankan tanpa syarat (`run_gate "antislop comment rules"`), ditambahkan `f8067e0` tanpa ikut mengubah header |
| `docs/DRAFT/045` | Baris Status menulis "F12 butuh test" padahal `c31ae1c` menutup F12 pada hari yang sama (paragraf F12 di bagian Hasil dokumen itu sendiri sudah mencatatnya), dan baris Status penutup menulis "MEDIUM (F4-F8) dan LOW (F9-F13) tidak disentuh" padahal `6860901`, `aa95471` dan `b012205` sudah mengerjakannya |
| `ANTISLOP.md §headroom.url` | **Tidak basi, klaim review salah.** `schema/settings.go:101` memang `json:"url"` tanpa `omitempty`, dan `app-ui/src/lib/schemas/token-saver.ts:77` memang `z.string()`; yang optional hanyalah section tulis di `:98`, persis seperti yang didokumentasikan |

## Yang dibatalkan, dan alasannya

Dua butir pada daftar owner adalah permintaan untuk mengembalikan perilaku yang pernah ada, pernah didiagnosis, dan
dihapus dengan sengaja. Melakukannya akan membuka kembali lubang yang tercatat di komentar yang sama.

**Urutan consume pada device poll.** `oauth_flow_device_poll.go:78-87`consume state **sebelum** connect, dengan
alasan yang tertulis di sana: "a token the upstream hands out twice is stored once". `Take` mengembalikan `taken`, dan
poll kedua berhenti di `:85-86`. Mengubahnya menjadi consume-setelah-tersimpan berarti satu login vendor bisa
menulis dua baris endpoint, dan untuk vendor yang tidak menyatakan identitas akun (`deviceAccountEmail` mengembalikan
string kosong untuk state round, `:133-135`) tidak ada kunci dedup yang menahan yang kedua. `oauth_flow_device_race_test.go`
ada untuk perilaku ini. Tidak disentuh.

**`/health` keluar dari rate limiter.** `router/limiter.go:33-39` mencatat bahwa exempt itu **pernah ada dan dihapus
sengaja**, karena `/health` menjawab tiap panggilan dengan probe Postgres dan Redis yang hidup, sehingga exempt berarti
membiarkan siapa pun mengarahkan sejumlah tak terbatas dependency check ke database panel sendiri. `router_test`-nya
ada dan mengunci keputusan itu: `limiter_test.go:50-68` `TestRequestRateLimitMetersThePublicSystemRoutes` mengassert
429 pada kedua path. Limit hari ini `RATE_LIMIT_PER_MIN` default 120 (`config.go:105`). Kalau yang mengganggu adalah
probe internal, jawabannya menaikkan limit atau jalur terpisah, bukan exempt. Catatan: jalurnya `/api/v1/health`
(`router.go:90`), bukan `/health`. Tidak disentuh.

**`ANTISLOP.md` bagian headroom.url.** Sudah akurat, rinciannya di F15. Tidak ada yang diperbaiki.

**Premis `Credential` bocor ke log.** `String()` sudah ada dan site produksinya nol. Yang dikerjakan justru temuan
sebelahnya, yaitu F12, bukan apa yang tertulis pada daftar owner.

## Rencana verifikasi

F1 punya bukti yang sudah dijalankan dan harus dijalankan lagi setelah perubahan, dengan `GATES_BASE_REF` rusak yang
sama:

```bash
GATES_BASE_REF=bogus-ref-xyz bash scrypts/gates/go-lint.sh;    echo "exit=$?"     # harus non-nol
GATES_BASE_REF=bogus-ref-xyz bash scrypts/gates/antislop.sh;   echo "exit=$?"     # harus non-nol
bash scrypts/gates/gate-scope-test.sh                                            # harus tetap hijau
```

F4 diuji dengan tiga berkas berubah yang membawa sitasi, bukan satu, karena itu nilai yang hari ini dilaporkan PASS.

Sisanya mengikuti aturan yang sudah ada di repo ini:

- `go test -race ./...` pada `app-serv`, tanpa `t.Skip()` dan tanpa filter `-run` (§2.1).
- Test baru untuk F2, F3, F5, F6, F9, F10, F12 dan F13 ditulis lebih dulu atau bersamaan implementasinya, karena
  semuanya service atau repository.
- F5 hanya jalan dengan `-tags=integration` dan DSN nyata; `all.sh` hari ini melewatinya tanpa DSN, jadi ia dibuktikan
  terhadap scratch database seperti pada 046.
- F14 dijalankan dengan `bun run test`, dan bentuk `border` yang dimaksud dikonfirmasi ulang terhadap kelas di
  `UsageTopologyDrawing.svelte:102`.
- Setiap berkas `.go` yang disentuh dicek terhadap 250 baris §1.1, dengan peringatan pada 220. Yang berisiko saat itu:
  `oauth_flow_refresh.go` 216, `main.go` 210, `endpoint.go` 211, `dto.go` 197, `qoder_catalog.go` 207. Berkas baru
  lebih baik daripada menambah ke yang sudah mendekati 220.
- `SYSTEM_MAP.md` diubah pada commit yang menggerakkan bentuk data, bukan belakangan (§1.9).

## Hasil

### Grup A (F1, F4, F7, F8, F11), landed

F1 dibuktikan pada kedua gerbang, dengan `GATES_BASE_REF` yang sama seperti pada Diagnosa:

```
$ GATES_BASE_REF=bogus-ref-xyz bash scrypts/gates/go-lint.sh
FAIL the change set could not be read; a gate that inspects nothing is not a pass   exit=1
$ GATES_BASE_REF=bogus-ref-xyz bash scrypts/gates/antislop.sh
FAIL the change set could not be read; a gate that inspects nothing is not a pass   exit=1
```

Sebelumnya keduanya `exit 0` dan `PASS antislop gate`. Helper `change_set` ditaruh di `scrypts/lib/common.sh` dan
memanggil `gate_fail` sendiri, supaya konsumen ketiga yang datang nanti tidak perlu menemukan lagi bahwa exit code
`changed_files` harus diperiksa.

F4 diukur dengan fixture tiga berkas `.go` yang masing-masing membawa `draft 017 §4.6`, nilai yang persis dilaporkan
PASS sebelum perbaikan:

```
== 3 files ==  SKIP 3 changed Go file(s) still cite a closed draft
== 1 file ==   SKIP 1 changed Go file(s) still cite a closed draft     (tidak berubah)
```

Fixture dibatalkan setelah diukur; `git status` pada `app-serv/` bersih.

F7 dan F11 diukur sebagai fixture terpisah sebelum pola dipasang, dan hasilnya ada di bagian F7. Pemasangan `LC_ALL`
dipilih di `common.sh` alih-alih di `antislop.sh` saja, karena semua gate membaca byte dengan cara yang sama dan dua
mesin yang berbeda locale adalah kegagalan yang tidak akan terlihat sampai CI hijau dan developer merah. Konsekuensinya
diperiksa, bukan diasumsikan: `gate-scope-test`, `go-headers`, `contract-drift`, `contract-openapi`, `secrets`,
`go-lint` dan `antislop` dijalankan seluruhnya setelah perubahan dan semuanya PASS, jadi tidak ada pola di gate lain
yang bergeser maknanya karena locale.

F8 memakai nilai yang diambil dari upstream pada pass ini, bukan angka yang dihitung ulang dari tebakan. Kedua SHA
action diverifikasi sebagai commit nyata lewat endpoint `git/commits`, dan kedua hash dibaca dari berkas checksum
resmi release masing-masing (`gitleaks_8.30.1_checksums.txt`, `SHASUMS256.txt` pada `bun-v1.3.0`). Bentuk
verifikasinya diuji lebih dulu: `printf '%s  %s\n' "$sha" "$file" | sha256sum --check --quiet` mengembalikan 0 pada
hash yang cocok dan 1 pada yang tidak cocok, sehingga unduhan yang berubah isi menghentikan job sebelum di-unpack.

Pin versi linter sengaja diambil pada versi terbaru hari ini, supaya mengganti `@latest` tidak sekaligus mengubah
verdict gerbang pada commit yang sama. Itu berarti `v2.9.0` dan `v0.8.1` belum pernah berjalan pada pohon ini: bila CI
menemukan sesuatu yang baru, itu hasil yang harus dibaca, bukan alasan untuk menurunkan angka pin.

Satu hal yang tidak ikut berubah dan perlu disebut karena ia adalah batasan yang masih ada: `go-lint.sh` memperlakukan
golangci-lint yang hilang sebagai `gate_skip`, sementara staticcheck yang hilang sebagai `gate_fail` (§1.4). Asimetri itu
keputusan yang sudah ditulis di header file itu, dan F8 tidak menyentuhnya.

### Grup C (F12, F13, F14, F15), landed

**F12.** Premis review salah, bentuk fix-nya benar, dan lubang yang sebenarnya hanya `%#v`; rinciannya di bagian F12. Yang dikerjakan: `GoString()` dan `LogValue()` di `provider.Credential`, di
`plugin_credential_redact.go` baru (210 baris pada `plugin_credential.go` tidak tempat yang baik untuk menambah
dua method, dan `redactedWhenSet` ikut pindah karena ia bagian concern yang sama). Buktinya dua arah: `GoString()`
dilepas dari kode yang sudah ada testnya, dan `%#v` pada `dataplane.Call` serta `dataplane.Selection` yang
sebenarnya langsung menulis plaintext; dipasang lagi, ketiganya hijau.

Satu hal yang tidak ikut dikerjakan dan harus disebut karena review memintanya: `String()` pada `Call` dan
`Selection`. Tidak perlu, dan ukurannya yang membuktikan: field `Credential` pada kedua struct itu **exported**,
dan fmt memanggil method sebuah field exported. Yang membuat probe pertama saya salah adalah field unexported
pada stub karangan saya sendiri, dan `grep` atas seluruh field bertipe `provider.Credential` memastikan tidak ada
satu pun bentuk unexported di kode produksi.

`LogValue()` bukan penutup kebocoran. Tanpanya slog menulis `{}` untuk sebuah Credential: tidak bocor, tidak
berguna. Ia masuk karena baris log yang tidak menyebut akun tidak bisa ditelusuri, dan itu disebut apa adanya.

Komentar di atas field `Credential` pada `Selection` ikut dikoreksi, bukan karena klaimnya salah (selection memang
tidak membocorkan apa pun) tapi karena alasan yang tertulis di sana salah: "assembled for exactly one request and
never stored" tidak ada hubungannya dengan apa yang dicetak sebuah nilai, dan pembaca berikutnya bisa menghapus method redaksi
dengan percaya alasan itu.

**F13.** Klaim review separuh benar, dan separuh yang salah justru penting untuk dicatat karena ia adalah
sejarah: jalur HTTP **sudah** di-drain hari ini (`srv.Shutdown` di `serve`, dan `BaseContext` tidak
pernah di-install sehingga context request tidak dibatalkan sinyal). Yang benar dan belum ada adalah dua hal
lain, dan keduanya sekarang punya test.

Penghenti worker tidak lagi menempel ke context sinyal. Worker berjalan pada context sendiri, dan jalur shutdown
yang memadamkannya sesudah server selesai. Yang bikin urutan ini bukan kerapian: `usage_event_publish.go` menguras
antreannya saat context-nya batal, sehingga berhenti pada sinyal berarti record yang dihasilkan handler selama
jendela shutdown masuk ke antrean yang konsumennya sudah pergi. Test-nya mengukur urutan dua event, bukan
kehadiran fungsi: sinyal datang ketika handler masih berjalan, dan stop harus terjadi **setelah** handler itu
selesai. Diukur dua arah: dengan panggilan stop dihapus dari `serve`, test gagal dengan "the workers were never
stopped".

Join juga nyata sekarang. `runWorkers` dulu men-spawn `go runSupervised(...)` dan tidak pernah mengembalikan apa
pun, jadi `serve` kembali sementara worker masih berjalan. `workerGroup` membawa WaitGroup dan `stopAndJoin`
menunggunya dengan jendela sendiri, bukan tanpa batas: satu worker yang mengabaikan context-nya tidak boleh
menahan proses melewati kill timeout orkestrator, jadi keterlambatan dilaporkan, bukan disembunyikan.

Flush shutdown berhenti sebagai satu batch. `quotaDrain` memanggil `FlushUntilQuiet` yang mengulang cycle sampai
batch datang lebih pendek dari ceiling. Kenapa ini bukan perubahan semantik worker: `quota_flush_policy.go`
menyatakan tick sengaja satu batch supaya backlog tidak jadi satu statement tak berbatas (§1.7), dan keputusan
itu dibiarkan utuh; loop-nya hidup di shutdown karena itu satu-satunya moment tanpa tick berikutnya. Jalur tick
tetap diuji untuk membuktikan ia **tidak** ikut berubah. Test-nya memberi tiga batch penuh dan menuntut empat
bacaan Pending (tiga penuh, satu pendek sebagai tanda keyspace habis); dengan body lama yang hanya sekali flush,
ia merah pada `Pending calls = 1, want 4`.

Satu konsekuensi yang sengaja tidak diubah, karena ia keputusan dan bukan slip: `/api/v1/usage/live` punya
`usageLiveMaxLifetime` 30 menit (`usage_live.go:53`) sementara jendela `shutdownTimeout` 15 detik. Stream
sepanjang itu akan selalu dipotong, dan memperpanjang jendela untuk alasan itu akan menunda setiap restart
proses. Ini dicatat sebagai sisa, dengan flag `truncated` F6 sebagai teman yang membuat layar tidak membacanya
sebagai data yang hilang.

**F14.** Regex-nya nyata longgar dan dibuktikan dengan eksperimen, bukan dibaca: kelas `border` pada markup
node diganti `border-0` (utility width hilang, yang lain tetap), lalu assert lama dijalankan pada markup itu.
assert lama **lulus**, assert baru gagal. Itu false positive yang review sebut, terukur. Perbaikan: utility dibaca
sebagai token kelas utuh, bukan substring.

Bagian "path relatif ke file test, bukan cwd" **tidak dikerjakan, dan klaimnya salah sebagai cacat.** Pola itu
bukan slip satu berkas: `grep` menemukan enam berkas `app-ui/tests/**` yang memakainya, termasuk dua modul
support bersama (`tests/support/contrast.ts`, `tests/support/routes.ts`), dan `panel-check.sh` menjalankan vitest
dari dalam direktori panel pada **kedua** jalurnya (`run_script()` = `(cd "$panel" && bun run …)` baris 57, dan
fallback npm baris 73). Jadi asumsi cwd dipegang oleh cara gerbang memanggilnya, bukan oleh kebetulan. Mengubah
satu berkas akan meninggalkan lima dengan konvensi yang berbeda, yang justru bentuk cacat yang dokumen ini
laporkan di tempat lain.

**F15.** Empat dokumen, dan hitungannya lebih dari yang review sebut.

| Berkas | Yang berubah |
| --- | --- |
| `antislop.sh` header | Bullet "Citations are a WARNING, not a failure" diganti karena ia salah sejak `b012205`: panel gagal, Go hanya warn. Satu bullet baru ditambahkan untuk hal yang grup A ubah, yaitu perubahan set yang tidak terbaca kini gagal, bukan lolos. Banner basi `# ---- citations, warning only ----` dihapus; ia berdiri tepat di atas penggantinya yang benar |
| `panel-check.sh` header | "format, type check, tests, build" menjadi daftar yang sama dengan tubuhnya: prettier, eslint, svelte-kit sync, svelte-check, vitest, build. Dua langkah itu ditambahkan `9751af0` tanpa ikut mengubah baris ini |
| `pre-push` header | Daftar bernomor tidak pernah menyebut gate antislop, padahal ia jalan tanpa syarat. Urutannya sekarang benar dan kalimat routing-nya membedakan mana yang bisa di-skip (1 dan 2) dan mana yang tidak (3, 4, 5) |
| `ANTISLOP.md` §2.5 | Dua tempat: baris tabel reach dan bullet-nya. Yang ditulis sekarang adalah apa yang gerbang lakukan setelah F7, termasuk dua hal yang review tidak sebut: penutup `-->` sebagai alasan palsu, dan direktif tanpa nama rule sebagai kegagalan tersendiri. Alasan kenapa plugin-qualified masih menjadi pemisah juga ditulis, karena itu penyebab false positive nyata di `eslint.config.js` |
| `docs/DRAFT/045` | Baris Status dan baris Status penutup. Penutup masih menulis "MEDIUM (F4-F8) dan LOW (F9-F13) tidak disentuh" sementara paragraf F12 di bagian Hasil dokumen itu sendiri mencatat F12 tertutup dengan test. Baris Status ternyata basi pada **dua** klaim, bukan satu: F12 memang sudah tertutup (`c31ae1c`), dan F10 juga, dengan premisnya dibatalkan sendiri di paragraf F10 |

### Ukuran penutup

`go test -race ./...` exit 0, suite ber-tag `integration` dengan `-p 1` terhadap scratch PostgreSQL dan scratch
Redis exit 0, `go-lint.sh` rc=0 (`go vet` dua tag, `gofmt`, `staticcheck` dua tag, `golangci-lint`), dan
`gate-scope-test`, `go-headers`, `antislop`, `contract-drift`, `contract-openapi`, `secrets` semuanya PASS.
Sisi panel: suite `app-ui` penuh **183 berkas, 2988 test, semuanya lolos (rc=0)** dan `bun run lint:ts` rc=0.
Ketiga berkas yang berubah (`schemas/oauth.ts`, `schemas/quota.ts`, `tests/schemas/usage-topology-geometry.test.ts`)
juga dijalankan sebagai kelompok tersendiri lebih dulu, supaya kegagalan lokal tidak perlu menunggu suite penuh.

`go-lint.sh` menangkap satu temuan dari tulisannya sendiri, dan bentuk jawabannya layak dicatat karena ia
bukan pengecualian: `staticcheck` menandai `fmt.Sprintf("%s", cred)` dengan S1025 "use String() instead".
Kasus `%s` itu dibuang, bukan di-suppress, karena `%s`, `%v` dan `%+v` menempuh satu jalur yang sama
(`String()`), jadi ia memang redundan di sisi `%v`; `%#v` tetap karena jalurnya berbeda (`GoString`). Set
verb yang diuji sekarang adalah yang berbeda secara mekanisme, bukan yang banyak.

`selection.go` tetap 231 baris dan gerbang memberi peringatan di angka itu, sebagaimana sebelum pass ini:
yang berubah di sana hanya komentar dua baris, dan menjadikannya file yang benar-benar dipecah bukan
tuntutan butir ini. Peringatan ini dibiarkan terlihat, tidak dinaikkan diam-diam menjadi lolos.

F15 juga mengoreksi satu hal di dalam draft ini sendiri. Versi pertama F12 menyatakan `String()` dilewati
untuk field nested bahkan pada `%v`. Itu salah, dan sumber kesalahannya adalah probe terhadap tipe karangan
saya sendiri yang field-nya unexported, bentuk yang tidak ada di produksi. Bentuk yang benar sudah ditulis
ulang di bagian F12 beserta pengukurannya, dan klaim bahwa komentar `selection.go` "factually salah" diganti:
klaimnya benar, alasannya yang salah.

### Grup B (F2, F3, F5, F6, F10), landed

Setiap butir dimulai dari test yang gagal, dan pesan Kegagalannya dicatat di sini apa adanya, karena itu bukti
bahwa test-nya menguji hal yang ia klaim.

**F2.** `TestIntegration_UpdateIfUnchangedLeavesAnOperatorsEditAlone` ditulis lebih dulu terhadap `ab1a4c4` yang
masih ada, dan merah:

```
stored status = "active", want the operator's "disabled": the rotation wrote the status it loaded
```

Perbaikannya bukan mengubah `updateEndpoint` jadi token-only, karena statement itu juga dipakai
`Update` dan `ImportOAuthBatch` dan bagi kedua jalur itu menulis baris penuh memang benar. Jalur CAS
mendapat statement sendiri (`updateEndpointTokens`) yang hanya menulis `oauth` dan `updated_at`, dan
`updateEndpoint` kehilangan parameter `loaded` yang sekarang tidak dipakai siapa pun. Dua test CAS yang
sudah ada tetap hijau, termasuk `..._SurvivesAStoredWrite`, karena guard credential-nya tidak berubah.

**F3.** Tiga test isolasi, masing-masing untuk satu dari tiga kebocoran, semuanya merah sebelum perubahan:

```
B was served a model only account A is listed for: the cached document travelled
A's refusal was remembered against B
B joined A's in-flight fetch            (context deadline exceeded, 2s)
```

Cakupan key diambil dari `Credential.EndpointID()`, bukan dari materi credential, dan berlaku untuk ketiga map
sekaligus (`entries`, `inflight`, `misses`) lewat satu `catalogTarget{base, scope}`: `base` untuk menghubungkan,
`scope` untuk menyimpan. Test pertama dan kedua hitam-box penuh; test ketiga menahan slot fetch akun A lewat
kanal agar deterministik, bukan balapan. Bentuknya diverifikasi dua arah: dengan akun di luar scope ketiga test
itu merah, dengan akun di dalam scope ketiganya hijau, dan lima test stampede yang lama tetap hijau, jadi
satu-fetch-per-akun masih terbukti menyatukan pembaca konkuren.

Satu perbaikan ikutan: test ketiga semula menunggu 15 detik, karena `defer close(release)` baru berjalan
setelah test menunggu goroutine A, dan yang menunggu itu adalah timeout fetch-nya sendiri.Perbaikannya: melepas A sebelum menunggu, dan paket `provider` turun dari 18,4 detik ke 1,1 detik.

**F5.** Kedua test kembali sebagai port, bukan revert: harness-nya (`newPublishedRepo`, `seedPublishedEndpoint`,
`statementCounter`) masih hidup, tetapi header berkas lama melanggar §2.7 dan §2.1 sehingga tidak bisa
dikembalikan apa adanya. Yang counted-statement masih menjawab **2 statements** setelah F6 ikut masuk, dan itu
bukan kebetulan: plafon dibaca lewat satu baris probe pada statement yang sama, sehingga mengenal pemotongan
tidak menambah query.

**F6.** `truncated` masuk ke `QuotaWindowList`, dan ke kedua bacaan. Alasannya disebut di sini karena ini
keputusan, bukan routine: kalau hanya akun yang diberi flag, halaman bisa menjawab `truncated: false` sementara
baris `data` yang menggambar kartu justru yang terpotong. Flag itu hasil OR dari dua bacaan. Nilainya diambil
dari `LIMIT ceiling+1` lalu satu baris dibuang, jadi halaman yang persis memenuhi plafon tidak salah lapor.
Kontrak YAML, `openapi.json` (regenerasi, gate byte-compare PASS), SPEC-API §7.12, schema panel, dan
`SYSTEM_MAP.md` ikut; `skipped` untuk §7.4 dengan jalan yang sama.

**F10.** Penyapu due tidak lagi membatalkan batch. Yang berubah hanya jalur tanpa `endpoint_id`: satu akun yang
galat masuk `skipped[]` dengan alasan dari `domain.AsAppError(err).Message`, yang merupakan sanitiser yang sama
dengan yang dipakai `WriteError`, sehingga tidak ada rantai galat internal yang ikut ke klien. Jalur satu akun
yang disebut namanya tetap fail-fast, dan itu diuji eksplisit
(`TestOAuthRefresh_OneNamedAccountStillReportsItsConflict`) supaya keputusan review yang lama tidak ikut
tergerus. `oauth_flow_refresh.go` dipecah ke `oauth_flow_refresh_sweep.go` karena ia sudah 216 baris dan
§1.1 memperingatkan pada 220.

Refill: `Refresh` doc comment yang menyatakan "fails fast on the first refusal" memang pernyataan sengaja,
bukan slip; yang dipertahankan adalah sebabnya (alasan konkret untuk aksi yang diminta), dan bentuknya yang
berubah untuk jalur batch, yaitu alasan itu sekarang sampai per akun dan bukan hanya yang pertama.

Ukuran selesai: `go test -race ./...` exit 0 (20 paket, nol DATA RACE), suite ber-tag `integration` terhadap
scratch PostgreSQL dan scratch Redis berjalan bersih (exit 0, 21 paket, nol kegagalan, nol race),
`contract-openapi` PASS, 101 test skema panel hijau, prettier dan eslint pada berkas yang berubah bersih.

Dan gerbang grup A menangkap satu pelanggaran dari grup B sendiri, yang layak dicatat karena itulah fungsi
gerbangnya: `quota_paging_integration_test.go` hasil port mencapai 284 baris dan `go-lint.sh` menjawab

```
FAIL app-serv/internal/repository/postgres/quota_paging_integration_test.go has 284 lines (AGENTS.md §1.1 max 250)
```

Ia dipecah menjadi `quota_paging_integration_test.go` (138) dan
`quota_paging_accounts_integration_test.go` (169), dan pemecahannya bukan demi angka: seeder hanya dipakai sisi
akun, jadi ia ikut ke sana, dan `seedPagingAccounts` bersama ketiga test akun kini berada di satu berkas yang
`@for`-nya menyebut sisi akun. Keempat test tetap hijau setelahnya.

**F9 ikut landed di commit ini**, bukan di grup C, karena berkas yang sama bergerak oleh F3 dan pemindahan dua
fungsi justru menyelesaikan §1.1 di tempat yang sama: `qoder_catalog.go` mencapai 224 baris setelah key per akun
masuk, melewati ambang peringatan 220, sementara `inferenceBase` dan `originOf` ternyata hanya dipakai jalur
lookup. Keduanya pindah ke `qoder_catalog_lookup.go` (187 dan 203 sesudahnya, dua-duanya di bawah ambang), dan
itu pemisahan yang berdiri sendiri benar, bukan akrobat angka.

Branch nil-nya diuji dua arah. Test `TestOriginOfKeepsAURLItCannotParse` hijau pada bentuk yang benar, dan
terhadap bentuk lama ia panic persis seperti yang diukur di Diagnosa:

```
panic: runtime error: invalid memory address or nil pointer dereference
--- FAIL: TestOriginOfKeepsAURLItCannotParse/an_escape_it_cannot_read
```

Nilai yang diharapkan untuk setiap bentuk yang gagal di-parse adalah URL itu sendiri, apa adanya: membiarkannya
lewati membuat `http.NewRequest` yang menolak, dan itu satu panggilan yang gagal dengan pesan yang bisa dibaca,
bukan proses yang berhenti.

Catatan operasional yang perlu berdiri sendiri: percobaan pertama suite ber-tag dijalankan dengan
`PANNELAI_TEST_REDIS_ADDR=127.0.0.1:6379` tanpa kredensial, dan 37 test gagal pada `NOAUTH`. Itu bukan regresi,
dan keberuntungan itu sendiri yang harus dicatat: port 6379 di mesin ini adalah Redis yang dipakai gateway
pengembang, dan draft `026` mendokumentasikan suite ini memanggil `FlushDB` pada keyspace itu, yang pernah
mengusir operator dari sesinya. Tidak ada satu pun test yang sempat menulis, karena semuanya gagal pada
langkah flush pertama. Run berikutnya memakai Redis buang sendiri di port 6399 dan `go-test.sh` menuntut `-p 1`
untuk alasan yang sama. Menjalankan suite ber-tag di mesin pengembang tanpa Redis scratch adalah aksi yang
tidak boleh dianggap enteng, dan dokumen ini tidak akan merekomendasikannya.
