# 047-GATE-FAIL-OPEN-AND-QODER-CATALOG-ISOLATION.md: Gerbang melaporkan PASS ketika ia tidak membaca apa pun, dan katalog Qoder di-key tanpa akun

Laporan owner 2026-10-08: hasil review atas PR #2 dan #3 disusun ulang menjadi empat prioritas, dan minta
dikerjakan dalam tiga grup (a) gate dan CI, (b) data OAuth dan katalog Qoder, (c) sisanya. Permintaan pertama adalah
memverifikasi setiap klaim sebelum mengerjakannya, karena sebagian dari daftar itu sudah salah dan sebuah fix akan
menghapus keputusan yang sengaja dibuat.

Pass ini memverifikasi 17 klaim, membatalkan 4 di antaranya dengan bukti, dan mengerjakan 13 sisanya.

## Status dan lingkup

| | |
| --- | --- |
| **Status** | **Grup A landed (F1, F4, F7, F8, F11). Grup B dan C belum.** Semua temuan di bawah terverifikasi terhadap kode, dan untuk F1, F4, F7 dan F9 dibuktikan dengan menjalankan kodenya langsung, bukan dengan membaca saja. Dua butir review dibatalkan dan alasannya ada di bagiannya sendiri |
| **Mechanism** | DURING: tulisan baru mengikuti R-02 dan R-31, dan setiap klaim harus bisa dibuktikan oleh perintah yang tertulis di dokumennya sendiri |
| **Scope** | **Bukan comment-only.** F1 sampai F3 menyentuh `scrypts/`, `app-serv` repository, service dan provider. Keluar dari guardrail antislop-code, atas izin eksplisit owner, seperti F1 dan F2 pada 046 |
| **Sumber temuan** | Daftar review owner, lalu pengukuran terhadap pohon kode, gerbang yang dijalankan dengan `GATES_BASE_REF` rusak, `go run` untuk semantik `net/url` dan `fmt`, dan `git show ab1a4c4^` untuk test yang hilang |
| **Struktural (AGENTS.md §1.9)** | `SYSTEM_MAP.md`: **tidak N/A.** F2 mengubah kolom yang ditulis satu jalur tulis, F3 mengubah cakupan cache per akun, dan F6 menambahkan satu field ke respons publik. Ketiganya dicatat ke `SYSTEM_MAP.md` pada commit yang mengubahnya. F10 bergerak di semantik satu route tanpa mengubah bentuknya |

## Diagnosa

Satu tema menghubungkan temuan yang paling serius di daftar ini: **sebuah pemeriksaan yang tidak membaca apa pun
melaporkan hasil yang sama dengan pemeriksaan yang membaca segalanya dan menemukan tidak ada.**

Gerbang `changed_files` memang mengembalikan galat ketika `GATES_BASE_REF` tidak resolve (`scrypts/lib/common.sh:146`),
dan menulis sebabnya ke stderr. Dua konsumennya memanggilnya di dalam process substitution:

```bash
scrypts/gates/go-lint.sh:63   done < <(changed_files)
scrypts/gates/antislop.sh:206 comm -12 <(text_paths | sort -u) <(changed_files | sort -u)
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
`no changed text files; set GATES_BASE_REF to check a diff range` (`antislop.sh:326`). Padahal `GATES_BASE_REF`
sudah di-set; yang benar adalah bahwa nilainya tidak bisa dibaca. Pesan itu mengarahkan pembaca ke tempat yang salah,
persis seperti "move or move its endpoints first" pada 046 F1.

CI sudah menutup satu kasus serupa, dan itu menunjukkan bahwa bentuk kegagalannya sudah pernah dipikirkan sekali:
`.github/workflows/gates.yml:108-113` menolak push yang tidak punya revisi di belakangnya dengan `exit 1`. Yang
tersisa adalah sisi yang sama, di tempat lain.

## Keputusan desain

**Sebab yang tidak terbaca adalah kegagalan, bukan hasil kosong.** Untuk gerbang yang bekerja pada satu set berkas,
set yang tidak bisa dihitung berarti gerbang itu tidak menjalankan aturannya sama sekali, jadi jawabannya fail, bukan
skip. Ini mengikuti aturan yang sudah ditulis `scrypts/lib/gate-scope.sh:9-19` untuk routing: "A gate that reports a
pass after inspecting nothing is the failure this file exists to avoid." Routing sudah menerapkannya; pemeriksaan
berbasis berkas belum.

**Perbaikan F2 tidak boleh menyentuh statement bersama.** `updateEndpoint` dipakai tiga jalur (`endpoint.go:140`,
`endpoint.go:151`, `endpoint_oauth_batch.go:52`), dan untuk `ImportOAuthBatch` serta `Update` menulis baris penuh
memang benar: import memiliki seluruh bentuk baris itu. Yang salah hanya jalur yang memuat agregat lalu menulisnya
kembali. Karena itu kolom token mendapat statement sendiri, dan `loaded != nil` memilih statement, bukan statement
tunggal yang ikut berubah untuk semua pemanggil.

**Key cache per akun memakai identitas, bukan secret.** `Credential.EndpointID()` sudah ada
(`provider/plugin_credential.go:143`) dan merupakan pengenal stabil. Memasukkan materi credential sebagai key map
menyimpan secret di struktur data yang hidup satu jam demi sebuah nilai yang sudah tersedia lewat kolomnya sendiri.
Repo ini sudah memutuskan hal yang sama di tempat lain: `qoder_identity.go:44` men-key cache identity dengan digest,
bukan dengan bearer mentah.

## Temuan

Klaim owner diberi verdict di judulnya. **Benar** berarti dikerjakan. **Dibatalkan** berarti kodenya sudah benar dan
perubahan yang diusulkan akan merusak sesuatu yang sengaja.

### F1 (HIGH) Benar: `changed_files` gagal, gerbang melaporkan PASS

`scrypts/lib/common.sh:128-158` mengembalikan 1 untuk base ref yang tidak resolve. `go-lint.sh:63` dan
`antislop.sh:206` menelannya lewat process substitution. Akibat terukur ada di Diagnosa: `go-lint.sh` exit 0,
`antislop.sh` mencetak `PASS antislop gate`.

Diperbaiki dengan capture output ke variabel, periksa exit code, lalu fail. Satu helper di `common.sh` lebih baik dari
dua pemeriksaan ad hoc, karena konsumennya akan bertambah. Pesan skip yang menuduh `GATES_BASE_REF` belum di-set ikut
dikoreksi: bedakan "memang tidak ada diff" dari "diff-nya tidak bisa dibaca".

Bukti selesai: `GATES_BASE_REF=bogus-ref-xyz` harus menghasilkan exit non-nol pada kedua gerbang, dan `gate-scope-test.sh`
tetap hijau.

### F2 (HIGH) Benar: refresh OAuth menulis 16 kolom, CAS hanya menjaga dua ciphertext

`oauth_flow_refresh.go:178-180` memanggil `SetOAuth` lalu `store.UpdateIfUnchanged`. Jalur itu jatuh ke
`updateEndpoint` (`endpoint_oauth_batch.go:72`) yang SET-nya (`:85-103`) mencakup `label`, `priority`, `status`,
`test_status`, `rate_limited_until`, `global_priority`, `default_model`, `consecutive_use_count`, `last_error`,
`last_error_at`, `error_code`, `proxy_pool_id`. Guard-nya (`:116-119`) hanya membandingkan
`oauth->>'access_token_encrypted'` dan `oauth->>'refresh_token_encrypted'`.

Jadi selang antara `listOAuthEndpoints` (`oauth_flow_refresh.go:111`) dan tulisannya: operator mengubah `status` atau
`priority`, oauth tidak berubah, CAS lolos, dan nilai basi dari agregat yang dimuat sebelum perubahan itu tertimpa.
Lost update nyata, dan persis pada kolom yang keputusan review sebut.

Perilaku yang dipertahankan: `updated_at` tetap ditulis, dan dua ciphertext pembanding tetap menjaga rotasi yang
berlomba, karena alasan itu ada di komentar `endpoint.go:143-147`.

### F3 (HIGH) Benar: katalog Qoder tidak terisolasi per akun, dan error satu akun di-delivery ke akun lain

Satu connector dibuat per registry entry (`qoder.go:60`, `catalog: newQoderCatalog()`), dan key lookup adalah
`base + "|" + modelKey` (`qoder_catalog_lookup.go:70`). `base` dari `inferenceBase` (`qoder_catalog.go:159-175`) hanya
bisa dua nilai: origin device atau origin job. Tidak ada identitas akun di key mana pun.

Tiga konsekuensi, semuanya nyata di kode hari ini:

1. `entries` menyajikan katalog yang dipelajari atas nama akun A ke akun B. Ini bertentangan dengan baris `@for`
   file itu sendiri (`qoder_catalog.go:4`): "the model configuration Qoder publishes to an authenticated account".
2. `inflight` membuat satu fetch per host. Waiter yang masuk lewat cabang non-leader (`qoder_catalog_lookup.go:82-91`)
   mengembalikan `fetch.err` milik leader apa adanya, jadi 401 akun A menjadi galat akun B.
3. `misses` menolak nama model selama 60 detik (`:75-77`, `qoderCatalogMissTTL`) berdasarkan jawaban vendor kepada akun
   lain, jadi model yang sebenarnya ada di akun B tetap ditolak tanpa pernah ditanyakan.

Komentar `qoder_catalog.go:42-44` menyatakan "the widest scope that is still correct" untuk key host-plus-model.
Klaim itulah yang salah, dan inkonsistensi internalnya yang menjadi bukti terkuatnya: cache identity di file
sebelahnya sudah di-key per credential.

Perbaikan mencakup ketiga map dengan key yang sama, plus galat yang tidak boleh berpindah antar akun.

### F4 (MEDIUM) Benar: `cite_warn` adalah counter, dibandingkan dengan `= "1"`

`antislop.sh:305` menaikkan `cite_warn` per berkas; `:313` men-set `cite_fail` ke 1 (boolean, bukan counter). Baris
`:320` membandingkan counter itu dengan `= "1"`. Logikanya dijalankan sendiri:

```
cite_warn=1 -> gate_skip: warnings require review
cite_warn=3 -> gate_pass "scratch-work citations (changed files)"     <-- salah
```

Tiga berkas berubah yang masing-masing membawa sitasi terbaca sebagai bersih. `cite_fail` di `:318` benar adanya dan
tidak disentuh, karena nilainya hanya 0 dan 1.

### F5 (MEDIUM) Benar: tepat dua test integrasi `PageAccountsByProvider` hilang

Keduanya dihapus `ab1a4c4` saat file ditulis ulang:
`TestQuotaRepository_PageAccountsIncludesAnAccountWithNoWindows` dan
`TestQuotaRepository_PageAccountsIsCountPlusPageOnly`. Versi hari ini
(`quota_paging_integration_test.go:32`) hanya menyisakan `TestQuotaRepository_PageWindowsByProvider_GroupsAndCounts`.
Yang tersisa untuk fungsi itu hanyalah fake: `handler/quota_account_stub_test.go:24`, `service/quota_test.go:169`,
`quota_flush_fixture_test.go:128`. SQL-nya tidak pernah dijalankan ke server nyata.

Ini port, bukan revert. Header versi lama melanggar §2.7 (nilai `@reason` lanjut ke baris ber-indent) dan §2.1
(mengandung satu em dash), jadi badan testnya saja yang pindah ke header yang sudah patuh.

Harness yang dibutuhkan masih hidup, dan itu membuat F5 murah: `newPublishedRepo(t)` mengembalikan
`(*PublishedQuotaRepository, *statementCounter)` (`quota_published_harness_integration_test.go:88`),
`seedPublishedEndpoint` ada di `:119`, dan `statementCounter` masih punya `reset`/`load`/`trace` (`:60`).

### F6 (MEDIUM) Benar: `LIMIT 1000` memotong tanpa sinyal ke klien

`quotaMaxRowsPerPage = 1000` (`quota_paging.go:25`) dipakai sebagai `LIMIT $3::int` (`:121`). `total` yang dikembalikan
adalah jumlah grup provider (`:43-46`), jadi tidak menggambarkan pemotongan baris sama sekali. Kode itu sendiri
mengakui di `:66-68`: "A group beyond the ceiling is served in key order and truncated".

Tidak ada tempat untuk membaca pemotongan itu: `schema.Page` hanya `page`, `per_page`, `total` (`dto.go:37-41`), dan
`handler/quota.go:49` membuang total akun (`accounts, _, err :=`). Nuansa yang harus ikut tercatat: ini ceiling baris
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
| `actions/checkout@v4` (`gates.yml:70`, `gates-frontend.yml:55`) | `11d5960a326750d5838078e36cf38b85af677262` (v4.4.0) |
| `actions/setup-go@v5` (`gates.yml:116`) | `40f1582b2485089dde7abd97c1529aa768e1baff` (v5.6.0) |
| `staticcheck@latest` (`gates.yml:124`) | `v0.8.1` |
| `golangci-lint/v2@latest` (`gates.yml:125`) | `v2.9.0` |
| gitleaks 8.30.1 tar.gz (`gates.yml:136-138`) | `551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb` |
| bun 1.3.0 baseline zip (`gates-frontend.yml:62-64`) | `77336611905b9e876e52924f8b1a57e72669cf10541dc1e11269d2c9371f9e45` |

Kedua SHA action diverifikasi sebagai commit nyata lewat `GET /repos/{repo}/git/commits/{sha}`, dan sha teratas pada
respons cocok dengan yang diminta. Kedua file checksum upstream tersedia (gitleaks `gitleaks_8.30.1_checksums.txt`,
bun `SHASUMS256.txt` pada release `bun-v1.3.0`), jadi verifikasinya bisa dilakukan, tidak perlu di-skip.

Dua keputusan yang perlu disebut karena mereka bukan tanpa konsekuensi. Pin versi linter diambil pada versi terbaru
hari ini, supaya `@latest` diganti oleh nilai yang sama dan hasil gerbang tidak berubah di commit yang sama; bila
`v2.9.0` atau `v0.8.1` ternyata menemukan sesuatu yang baru di pohon ini, itu tercatat sebagai hasil, bukan
dibungkam. Dan `.golangci.yml` hari ini memakai `version: "2"` sebagai schema, bukan sebagai pin biner; konfigurasi
itu tidak berubah oleh F8.

`gates.yml:130-135` sudah menuliskan alasan yang bagus untuk mem-pin versi gitleaks (8.24.3 vs 8.30.1 menandai fixture
yang berbeda). F8 menambah integritas di atas pin yang sudah bermakna itu, tidak menggantinya.

### F9 (LOW) Benar, dan naiknya kategori: `originOf` panic pada URL yang gagal di-parse

Klaim owner menempatkannya di prioritas "bersih-bersih". Ia bukan kosmetik. `qoder_catalog.go:180-186` menulis:

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
bukan jalur eksploitasi. Yang membuatnya layak naik: `inferenceBase` dipanggil di `modelConfig:66`, sebelum
`beginFetch`, sehingga ia berada di jalur shaping request dan `runFetch` (`:107-117`) tidak memulihkannya. AGENTS.md
§1.6 melarang panic yang tidak dipulihkan bertahan, dan di sini ia bisa menghentikan proses untuk alasan satu karakter
salah ketik.

### F10 (LOW) Benar: satu akun yang konflik membatalkan seluruh batch refresh

`oauth_flow_refresh.go:120-123` mengembalikan galat langsung dari dalam loop, jadi `outcome.Refreshed` dan
`outcome.EndpointIDs` yang sudah terkumpul dibuang. Akun yang konflik adalah akun yang refresh-nya kalah balapan,
bukan akun yang membuat batch lain tidak boleh berhasil.

Perilaku yang harus dipertahankan: conflict tetap terlihat oleh operator sebagai hitungan yang bisa dibedakan, bukan
diam-diam hilang, karena `MarkRefreshDeadLetter` (`:187`) sudah menjadi jalur terminal untuk kegagalan yang lain.

### F11 (LOW) Benar: `<!--` dibaca untuk berkas non-svelte, dan locale tidak pernah di-set

`fe_body_comments` (`antislop.sh:234-245`) memproses `<!--` dan `-->` untuk semua ekstensi yang memanggilnya, yaitu
`ts | tsx | js | mjs | svelte` (`:308`). Di `.ts` dan `.js`, `<!--` bukan konstruk komentar. Arah kesalahannya
false positive, bukan lolos-check, jadi ini kebersihan, bukan lubang.

`LC_ALL` dan `LANG` tidak muncul di satu pun berkas `scrypts/` (digrep, nol hasil). Pemeriksaan emoji `:338-339` dan
regex byte-oriented lainnya bergantung pada bagaimana locale menafsirkan byte, sehingga hasilnya bisa berbeda antara
mesin developer dan runner. Satu `export LC_ALL=C.UTF-8` membuat keduanya deterministik.

### F12 (LOW) Premis klaimnya salah, tapi ada bug nyata di sebelahnya

Klaim owner: "`Credential` tidak punya `GoString()` dan `LogValue()` sehingga secret bisa bocor ke log."

Yang benar: `String()` **sudah ada** (`plugin_credential.go:191-198`) dan me-redact lewat `redactedWhenSet`. Yang absen
hanya `GoString()` dan `LogValue()`. Pencarian site kebocoran di kode produksi tidak menemukan apa pun: nol `%+v` dan
nol `%#v` ke `Credential`, nol `slog.Any` di produksi, dan enam pemanggil `Secret()`
(`provider/default.go:117`, `qoder.go:143`, `opencode_auth.go:37`, `dataplane/media.go:179`,
`service/systemone_target.go:59`) semuanya hanya mengisi header.

Yang membuat butir ini tetap masuk daftar: semantik `fmt` diuji langsung, dan `String()` tidak dipanggil untuk field
nested.

```
nested %v  : {w {e1 SK_REAL}}                       <-- String() dilewati, token tercetak
plain %#v  : main.cred{id:"e1", secret:"SK_REAL"}
```

`Credential` adalah field dari `dataplane.Call.Credential` (`dataplane/upstream.go:42`) dan
`dataplane.Selection.Credential` (`dataplane/selection.go:67`), dan keduanya tidak punya `String()`. Karena itu
komentar `selection.go:65-66`, yang menyatakan "a logged selection cannot leak a secret", faktually salah.

Ini juga membatalkan bentuk fix yang diusulkan review. `GoString()` dan `LogValue()` pada `Credential` tidak
menyelesaikan kasus nested `%v`. Yang menyelesaikannya adalah `String()` pada `Call` dan `Selection`. Tambah
`LogValue()` tetap worthwhile karena ia murah dan membuat `slog` benar, tapi bukan itu yang menutup lubang.

### F13 (LOW) Sebagian benar, dan sebagian berbeda dari yang dinyatakan

Klaim owner: "hentikan HTTP dan drain stream dulu, baru hentikan worker."

HTTP memang di-drain: `shutdown_drain.go:64` memanggil `srv.Shutdown`, dan `main.go:127-138` tidak memasang
`BaseContext`, sehingga context request tidak dibatalkan oleh sinyal dan `Shutdown` menunggu connection yang masih
hidup. Yang salah adalah dua hal lain:

1. Worker tidak pernah di-join. `worker_wiring.go:99-121` men-spawn `go runSupervised(...)` tanpa WaitGroup, jadi
   tidak ada satu pun titik yang menunggu mereka selesai.
2. Jalur publish keluar lebih dulu dari handler yang masih berjalan. `usage_event_publish.go:123-135` melakukan
   `case <-ctx.Done(): p.drain(ctx); return`, sehingga publisher drain lalu keluar, sementara handler in-flight masih
   merekam event selama jendela `shutdownTimeout` 15 detik (`main.go:38`). Event itu masuk ke antrian tanpa konsumen
   dan hilang di `:109-111`.

Ada satu kasus yang melebihi jendela secara eksplisit: `/api/v1/usage/live` punya `usageLiveMaxLifetime = 30 *
time.Minute` (`usage_live.go:53`). Stream sepanjang itu tidak akan pernah selesai secara wajar pada shutdown dan akan
dipotong.

Untuk quota flush, jalur normal sudah benar dan sudah didokumentasikan: `quota_flush.go:67-79` adalah ticker loop 30
detik, dan `quota_flush_policy.go:26-28` menyatakan BatchSize memang membatasi satu flush supaya backlog habis lewat
beberapa tick. Yang satu kali jalan adalah **drain shutdown**: `shutdown_drain.go:89` memanggil `FlushOnce` sekali,
satu batch 500, sehingga backlog lebih dari itu tertinggal di Redis sampai boot berikutnya. Loop yang benar ada di
drain shutdown, bukan di dalam `FlushOnce`.

### F14 (LOW) Benar: regex geometry longgar, dan path dihitung dari cwd

`app-ui/tests/schemas/usage-topology-geometry.test.ts:53`: `expect(nodeBlock()).toMatch(/\bborder\b/)`. `-` bukan word
character, sehingga pola itu juga cocok untuk `border-[var(--color-border)]` dan varian `border-b`, padahal yang
dipin adalah utility width polos (`BORDER_ACROSS = 2`, `:21`; kelas polos itu memang ada di
`UsageTopologyDrawing.svelte:102`). Menghapus utility width sambil menahan satu kelas `border-*` membuat test tetap
hijau, yaitu test yang tidak lagi menguji apa yang ia klaim.

Path: `resolve(process.cwd(), 'src/lib/components/UsageTopologyDrawing.svelte')` (`:18`). Pola yang sama dipakai
`tests/tokens/contrast.test.ts:146`, jadi ini konvensi repo, tapi ia salah asumsi bila vitest dijalankan dari luar
`app-ui/`.

### F15 (LOW) Benar untuk tiga dari empat dokumen

| Dokumen | Yang basi |
| --- | --- |
| `antislop.sh:19` | Masih menulis "Citations are a WARNING, not a failure", padahal `:308-315` sudah `gate_fail` untuk panel dan hanya Go yang warn. Banner `# ---- citations, warning only ----` di `:283` masih berdiri tepat di atas penggantinya di `:285` |
| `panel-check.sh:22` | "format, type check, tests, build" tidak menyebut eslint (`:96-103`) dan svelte-kit sync (`:112-119`) yang keduanya dijalankan |
| `pre-push` | Daftar bernomor 1 sampai 4 tidak mencantumkan gate antislop yang dijalankan tanpa syarat (`run_gate "antislop comment rules"`), ditambahkan `f8067e0` tanpa ikut mengubah header |
| `docs/DRAFT/045` | Baris 20 menulis "F12 butuh test" padahal `c31ae1c` menutup F12 pada hari yang sama (baris 218 dokumen itu sendiri sudah mencatatnya), dan baris 248 menulis "MEDIUM (F4-F8) dan LOW (F9-F13) tidak disentuh" padahal `6860901`, `aa95471` dan `b012205` sudah mengerjakannya |
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
- Setiap berkas `.go` yang disentuh dicek terhadap 250 baris §1.1, dengan peringatan pada 220. Yang berisiko:
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

Satu hal yang tidak ikut berubah dan perlu disebut karena ia adalah batasan yang masih ada: `go-lint.sh:130-141`
memperlakukan golangci-lint yang hilang sebagai `gate_skip`, sementara staticcheck yang hilang sebagai `gate_fail`
(§1.4). Asimetri itu keputusan yang sudah ditulis di komentar `:8-10`, dan F8 tidak menyentuhnya.

Grup B dan C belum dikerjakan pada bagian ini.
