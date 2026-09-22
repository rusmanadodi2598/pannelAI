# 011-CODEBUDDY-PROVIDER-READINESS.md: Kesiapan Provider CodeBuddy CN dan CodeBuddy Int di Registry app-serv

Dokumen kerja hasil pemeriksaan registry `app-serv/.` untuk dua provider yang diminta owner pada
2026-09-22. Bukan kontrak; kontrak tetap `docs/SPEC-API/001-SPEC-API.md` (wire) dan
`docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel). Dokumen ini mengikuti pola
`008-SKILL-ENDPOINT-READINESS.md` dan seterusnya: temuan bernomor F, bukti yang bisa diulang,
rencana DURING, dan keputusan owner di depan implementasi. Tidak ada kode produksi yang diubah.

| | |
|---|---|
| **Status** | Terbuka; menunggu keputusan owner (§8). Sisi panel sudah CLOSED di scope-nya sendiri |
| **Scope** | hanya `app-serv/.` (registry dan generatornya). Panel `app-ui/` sudah selesai di scope-nya sendiri dan tidak disentuh oleh draft ini |
| **Kebutuhan** | Dua provider bawaan registry yang terlihat di halaman Provider panel: CodeBuddy CN dan CodeBuddy Int |
| **Kaitan** | SPEC-API §6, §7.4, §7.12; SPEC-UI §6.3; AGENTS.md §1.9; `app-serv/tools/registry-gen.mjs`; `app-serv/internal/registry/registry.yaml` |
| **Tanggal audit** | 2026-09-22 |

## 1. Ringkasan

Owner meminta empat provider yang belum tampil di halaman Provider panel: OpenAI Compatible,
Anthropic Compatible, CodeBuddy Int, dan CodeBuddy CN (permintaan 2026-09-22).

Dua yang pertama sudah ditutup di `app-ui`. Keduanya bukan entri registry melainkan node buatan
operator lewat `POST /api/v1/provider-nodes` (SPEC-API §7.4): gateway mensintesis entri provider dari
node, dan panel kini punya section Custom provider di `/providers` (daftar, dua tombol tambah, form
tambah dan ubah) plus halaman detail node dengan Edit, Test, dan Delete.

Dua yang terakhir tidak bisa ditutup dari panel. Keduanya provider bawaan registry, dan registry
yang di-embed `app-serv` tidak memuatnya: satu-satunya entri `codebuddy` yang ada `hidden: true`,
dan daftar provider menyaring entri hidden. Jadi pekerjaannya ada di `app-serv`, dan dokumen ini yang
mencatatnya supaya scope panel tetap bisa dinyatakan CLOSED tanpa mengklaim sesuatu yang belum ada.

## 2. Bukti

Tiga lapis, semuanya bisa diulang dari working tree ini.

**Registry yang di-embed hari ini**

- `app-serv/internal/registry/registry.yaml:5`: `revision: 9router@db4499d (2026-06-19)`, yaitu
  revisi reference yang jadi sumber generator saat file itu ditulis.
- `app-serv/internal/registry/registry.yaml:2274-2298`: satu entri `codebuddy` dengan
  `hidden: true`, `category: oauth`, base URL `https://copilot.tencent.com/v1/chat/completions`,
  blok `oauth` lengkap, dan tanpa `models`, tanpa `headers`, tanpa `force_stream`.
- `registry.yaml` memuat 94 entri (`grep -c '^  - id: '`), 10 di antaranya `hidden: true`
  (`grep -c 'hidden: true'`), jadi daftar panel hari ini berisi 84 provider dan tidak satu pun
  CodeBuddy.

**Yang benar-benar dibaca service dan data plane**

- `internal/service/provider.go:79` memanggil `entries(filter, true)`, dan `:150` melewati entri
  `Hidden` saat flag itu true. Entri `codebuddy` karena itu tidak pernah muncul di
  `GET /api/v1/providers`.
- `internal/service/provider.go:107` (`Detail`) tidak menyaring hidden, jadi `/providers/codebuddy`
  sebenarnya masih dapat dibuka hari ini lewat id-nya. Yang hilang adalah barisnya di daftar.
- `internal/dataplane/transport.go:142` menyalin `Transport.Headers` ke request keluar, jadi header
  yang dideklarasikan sebuah entri benar-benar dikirim. Ini penting untuk CodeBuddy: reference
  mendeklarasikan enam header di entri CN dan enam di entri Int.
- `internal/dataplane/transport_shape.go:77` dan test `engine_forced_scope_test.go:87` menetapkan
  bahwa forced stream diputuskan oleh **connector**, bukan oleh field `force_stream` registry.
  Test itu menuliskan alasannya: membaca field registry di situ akan diam-diam mengubah cara
  jawaban tiga provider (`openai`, `codex`, `commandcode`) dibaca. Satu-satunya connector yang
  mendeklarasikan forced stream adalah OpenCode (`internal/provider/opencode.go:188`).
- `ThinkingFormat` (`internal/registry/types_transport.go:54`) dan `Transport.Usage`
  (`internal/registry/types_transport.go:50`) hanya dideklarasikan. Tidak ada pembaca runtime untuk
  keduanya di `app-serv` (grep `ForceStream|ThinkingFormat` dan `UsageConfig|Transport\.Usage`),
  jadi keduanya akan inert kalau ditambahkan sebagai data.
- Panel tidak membaca blok `display` provider: `app-ui/src/lib/schemas/provider.ts` tidak memuat
  field ikon atau `display`, dan tabel daftar merender nama, kategori, auth, jumlah endpoint, dan
  ringkasan status. Jadi tidak ada pekerjaan ikon yang menyertai penambahan entri.

**Reference**

- Checkout `/home/rusmanadodi/apps/9router`, `origin/master` = `a8c9d380` (tag v0.5.81).
- `open-sse/providers/registry/codebuddy.js` sudah **tidak ada** di revisi ini. Yang ada dua berkas
  baru, keduanya `hidden: false`:
  - `open-sse/providers/registry/codebuddy-cn.js`: `id: codebuddy-cn`, `alias: cbcn`,
    `uiAlias: cbcn`, base URL `https://copilot.tencent.com/v2/chat/completions`,
    `forceStream: true`, `thinkingFormat: "openai"`, enam header (User-Agent CLI, X-Product,
    X-IDE-Type, X-IDE-Name, x-requested-with, x-codebuddy-request), `usage.url`
    `https://copilot.tencent.com/v2/billing/meter/get-user-resource`, 13 model, dan blok `oauth`
    dengan `platform: CLI`.
  - `open-sse/providers/registry/codebuddy-intl.js`: `id: codebuddy-intl`, `alias: cbai`,
    `uiAlias: cbai`, base URL `https://www.codebuddy.ai/v2/chat/completions`, `forceStream: true`,
    `thinkingFormat: "openai"`, enam header dengan User-Agent IDE, `usage.url` di domain `.ai`,
    15 model, dan blok `oauth` dengan `platform: ide`.
- Di revisi yang dipin registry, `git show db4499d:open-sse/providers/registry/codebuddy.js` masih
  ada dengan `hidden: true` dan base URL `/v1/chat/completions`, jadi YAML hari ini setia pada
  revisinya. Yang berubah adalah reference-nya, bukan generatornya.
- `app-serv/tools/registry-gen.mjs:359` menyalin `entry.hidden` apa adanya, dan `:33-35` mengimpor
  `open-sse/providers/registry/index.js` dari checkout reference. Jadi penambahan entri bukan
  keputusan generator: generator hanya memindahkan apa yang ada di reference.

**Perintah yang menghasilkan angka di atas**

Dijalankan dari root repo dan dari checkout reference (`/home/rusmanadodi/apps/9router`):

```bash
grep -c '^  - id: ' app-serv/internal/registry/registry.yaml          # 94 entri
grep -c 'hidden: true' app-serv/internal/registry/registry.yaml       # 10 tersembunyi
grep -c 'has_oauth: true' app-serv/internal/registry/registry.yaml    # 1, yaitu xai (registry.yaml:3315)
git show a8c9d38:open-sse/providers/registry/index.js | grep -c '^import p'   # 119 entri di HEAD
git show db4499d:open-sse/providers/registry/index.js | grep -c '^import p'   # 94 entri di revisi yang dipin
git ls-tree --name-only a8c9d38:open-sse/providers/registry/ | grep codebuddy # codebuddy-cn.js dan codebuddy-intl.js
git show a8c9d38:open-sse/providers/registry/codebuddy-cn.js | sed -n '/headers: {/,/},/p'  # enam header
```

## 3. F1 (HIGH): registry tidak memuat entri CodeBuddy yang terlihat

### Fakta

`GET /api/v1/providers` tidak dapat memuat CodeBuddy selama entri yang ada `hidden: true` dan tidak
ada entri lain. Panel membaca daftar itu apa adanya, jadi permintaan owner tidak dapat dipenuhi dari
sisi panel tanpa berbohong: membuat section atau baris khusus CodeBuddy di `app-ui` akan menampilkan
provider yang tidak ada di registry, yang persis dilarang oleh aturan kejujuran konten.

### Risiko dan aturan

- SPEC-UI §6.3: daftar Provider adalah registry yang dilayani API, bukan daftar yang diarang panel.
- R-38: UI tidak boleh menampilkan entri yang tidak berasal dari data nyata.
- SPEC-API §6: registry adalah config statis yang di-embed, dan satu-satunya cara sah menambah
  provider adalah lewat file itu.

### Rencana DURING

Pilihan ada di §8, karena keduanya menyentuh lebih dari satu berkas. Apa pun pilihannya, perubahan
dimulai dengan test yang gagal lebih dulu:

1. Test registry yang membaca entri `codebuddy-cn` dan `codebuddy-intl` dari index, dengan expected
   id dan alias yang eksplisit (bukan assert jumlah entri).
2. Test service yang membuktikan keduanya muncul di `GET /api/v1/providers` tanpa filter, dan
   membuktikan `Detail` tetap menjawab untuk keduanya.
3. Test handler yang membuktikan `has_oauth: true` dan `auth_modes: ["oauth", "apikey"]` ikut
   tersaji, karena panel memakai `has_oauth` untuk memutuskan apakah section OAuth dirender.

### Kriteria selesai

- Dua entri ada di registry yang di-embed, `hidden: false`, dan terbaca lewat `GET /api/v1/providers`.
- Revisi reference yang jadi sumber tercatat di header `registry.yaml` dan tidak lebih tua dari
  revisi yang benar-benar memuat kedua entri.
- Tidak ada test yang hanya mengasertif jumlah entri; test memuat expected id.
- Panel tidak diubah untuk kasus ini: entri registry cukup.

## 4. F2 (HIGH): "unhide entri codebuddy" bukan perbaikan yang setara

### Fakta

Mengubah `hidden: true` menjadi `false` pada entri lama akan menampilkan satu provider bernama
CodeBuddy dengan base URL `https://copilot.tencent.com/v1/chat/completions`, tanpa daftar model,
tanpa header CodeBuddy, dan tanpa `force_stream`. Reference justru menghapus entri itu dan
menggantinya dengan dua entri terpisah, dan perbedaannya bukan kosmetik:

| Aspek | Entri lama (`codebuddy`, db4499d) | Reference sekarang |
|---|---|---|
| Jumlah entri | 1 | 2 (`codebuddy-cn`, `codebuddy-intl`) |
| Jalur chat | `/v1/chat/completions` | `/v2/chat/completions` |
| Domain Int | tidak ada | `https://www.codebuddy.ai` |
| Model | tidak ada | 13 di CN, 15 di Int |
| Header wajib | tidak ada | enam per entri, termasuk `x-codebuddy-request: 1` |
| `force_stream` | tidak ada | `true` di keduanya |
| `thinking_format` | tidak ada | `openai` di keduanya |
| Platform OAuth | `CLI` | `CLI` (CN) dan `ide` (Int) |
| Alias model | tidak ada | `cbcn` dan `cbai` |

Satu provider gabungan juga tidak dapat mewakili dua domain dan dua platform OAuth yang berbeda.
Alias `cbcn` dan `cbai` adalah nama yang dipakai operator di `provider/model`, jadi keduanya
menentukan alamat model dan tidak boleh disatukan. Keduanya masih bebas di registry hari ini:
`grep -c -E "cbcn|cbai" app-serv/internal/registry/registry.yaml` mengembalikan nol.

### Risiko dan aturan

- SPEC-API §6: id dan alias entri adalah kontrak yang dipakai jalur request, bukan label tampilan.
- R-38: menampilkan satu provider yang mewakili dua backend berbeda akan membuat operator salah
  menebak endpoint mana yang dipanggil.
- `hidden: true` di entri lama bukan kelalaian yang bisa dibalik: reference menandainya begitu
  karena entri itu memang bukan provider yang dapat dikonfigurasi sendiri pada revisinya.

### Rencana DURING

Ikuti reference apa adanya: dua entri, dua id, dua alias, dua domain. Jangan menggabungkan keduanya
menjadi satu entri baru, dan jangan menyalin nama `codebuddy` untuk salah satunya.

### Kriteria selesai

- Dua entri terpisah, dengan `alias` dan `ui_alias` sesuai reference.
- Tidak ada entri `codebuddy` gabungan yang tersisa di registry.
- Alias `cbcn` dan `cbai` unik terhadap seluruh registry dan tidak menabrak alias yang sudah ada.

## 5. F3 (MEDIUM): regenerasi penuh menambah 25 provider, bukan dua

### Fakta

`app-serv/tools/registry-gen.mjs` mengimpor `open-sse/providers/registry/index.js` dari checkout
reference dan menulis ulang seluruh YAML. Jumlah entri di `index.js`:

- revisi yang dipin sekarang (`db4499d`): 94 entri, sama dengan 94 entri YAML hari ini;
- reference sekarang (`a8c9d38`): 119 entri.

Selisihnya 25 entri bersih, bukan dua. Di direktori registry, tiga berkas entri hilang
(`codebuddy.js`, `kimi-coding.js`, `qwen.js`) dan 31 berkas muncul, dua di antaranya entri CodeBuddy.
Artinya jalur "regenerate dari HEAD" membawa 23 provider lain yang tidak diminta siapa pun pada
perubahan ini, masing-masing dengan model, header, dan blok transportnya sendiri.

### Risiko dan aturan

- AGENTS.md §1.7 dan disiplin perubahan kecil: satu permintaan provider tidak boleh menyelundupkan
  23 provider lain ke dalam diff yang sama.
- Regenerasi penuh juga menaikkan `revision:` di header YAML, yang berarti klaim "registry ini
  setia pada revisi X" berubah untuk seluruh berkas, bukan hanya untuk dua entri.
- Sebaliknya, menambal YAML dengan tangan melanggar aturan di header berkas itu sendiri
  ("Do not edit by hand: edit the generator, or the reference, and regenerate").

### Rencana DURING

Owner memilih salah satu, dan pilihannya dicatat di commit yang mengerjakannya:

A. **Regenerasi penuh dari `a8c9d38`.** Paling setia pada aturan generator, dan sekaligus menutup
   drift tiga bulan. Harganya: diff besar, 23 provider ikut masuk, dan setiap test yang membaca
   entri lain harus ikut diperiksa. Bila ini yang dipilih, kerjakan sebagai perubahan tersendiri
   dengan test paritas registry yang membaca jumlah entri dari dokumen, bukan angka yang ditulis di
   test.

B. **Pin generator ke satu revisi antara.** Tidak disarankan: reference tidak menyediakan revisi
   yang hanya berisi kedua entri CodeBuddy, jadi ini berarti memilih revisi yang tetap membawa
   provider lain.

C. **Tambal dua entri ke YAML, lalu catat utangnya.** Diff kecil dan mudah ditinjau, tetapi
   melanggar aturan "do not edit by hand". Bila owner memilih ini, generator harus diberi jalur
   tambalan yang eksplisit (misalnya daftar entri yang disalin dari revisi lebih baru) supaya
   berkas hasilnya tetap dapat direproduksi, dan komentar di header YAML harus menyebut jalur itu.

### Kriteria selesai

- Berkas hasil dapat direproduksi dengan satu perintah yang tertulis di dokumen ini atau di
  komentar generator.
- Tidak ada entri yang hilang tanpa disebutkan: bila regenerasi penuh, tiga entri yang hilang di
  reference dibahas di commit body.
- Test paritas membaca daftar id dari YAML dan membandingkannya dengan index reference, sehingga
  drift berikutnya gagal di test, bukan di produksi.

## 6. F4 (LOW): dua angka dokumen menjadi basi bila entri bertambah

### Fakta

Jumlah 94 provider tertulis di empat tempat:

- `SYSTEM_MAP.md:8` ("registry provider di-embed (94 provider, decode ketat)");
- `app-serv/README.md:25` ("94 provider");
- `app-serv/internal/schema/provider.go:61` (komentar: "94 entries in the P1 document");
- `docs/DRAFT/010-USAGE-ENDPOINT-READINESS.md:860` (menyebut registry 94 provider).

Tidak ada test yang memaku angka itu (`grep -rn '\b94\b' app-serv/internal/registry app-serv/internal/handler
app-serv/internal/service` mengembalikan nol baris di luar YAML itu sendiri), jadi menambah entri tidak akan
menggagalkan gate; yang basi hanya dokumennya.

Satu kalimat kontrak ikut menjadi basi, dan ini yang lebih penting daripada angkanya:
`docs/SPEC-UI/001-SPEC-UI.md:339-340` menyebut "The registry's one `has_oauth` provider declares no
`authorize_url`". Hari ini kalimat itu benar: hanya satu entri yang `has_oauth: true`
(`registry.yaml:3315`, entri `xai`). Kedua entri CodeBuddy juga `hasOAuth: true` tanpa
`authorize_url`, jadi setelah ditambahkan kalimat itu harus berubah dari "one" menjadi tiga provider
yang berbagi sifat itu.

### Risiko dan aturan

- R-17 dan R-36: klaim angka harus terukur dan tidak boleh ketinggalan dari kenyataan.
- Sebagian besar tempat itu dibaca orang yang mencari status, bukan kode, sehingga angka yang basi di
  sana menyesatkan pembaca yang tidak akan membuka YAML-nya.

### Rencana DURING

Hitung ulang dengan perintah yang sama yang dipakai dokumen ini (`grep -c '^  - id: '`) dan tulis
angka barunya di commit yang sama dengan penambahan entri. Bila nomor di
`docs/DRAFT/010-USAGE-ENDPOINT-READINESS.md` yang berubah, tambahkan catatan bahwa angka itu
diukur pada tanggalnya, karena dokumen itu register bertanggal dan bukan kontrak hidup.

### Kriteria selesai

- Setiap angka provider di dokumen sama dengan `grep -c '^  - id: '` atas YAML yang di-embed.
- Tidak ada dokumen yang menyebut jumlah provider lama tanpa keterangan tanggal.
- Kalimat SPEC-UI §6.3 tentang provider `has_oauth` tanpa `authorize_url` diperbarui pada perubahan
  yang sama, karena jumlahnya berubah dari satu menjadi tiga.

## 7. Non-findings

- **Panel tidak perlu diubah untuk dua provider ini.** `app-ui` membaca daftar provider dari API dan
  tidak menyimpan daftar nama provider sendiri; `app-ui/src/lib/schemas/provider.ts` tidak memuat
  `display` maupun ikon. Setelah entri ada, halaman Provider, halaman detail, form endpoint, dan
  section OAuth bekerja tanpa perubahan panel.
- **Section OAuth akan hidup sendiri.** Kedua entri reference punya `hasOAuth: true` tanpa
  `authorize_url` di bentuk yang dipakai registry, jadi panel akan melaporkan flow `device` dan
  jalur start tetap dorman. Itu perilaku yang sudah tercatat di SPEC-UI §6.3 dan Q23, bukan temuan
  baru di sini.
- **Tidak ada pekerjaan ikon.** `public/providers/codebuddy-cn.png` dan `codebuddy-intl.png` ada di
  reference, tetapi panel tidak merender ikon provider dari registry.
- **`thinking_format` dan `usage.url` bukan blocker untuk daftar.** Keduanya inert hari ini, dan
  keduanya hanya relevan bila nanti ada pekerjaan penalaran dan kuota untuk provider ini. Mencatat
  keduanya di sini supaya tidak ada yang mengira menambah entri sudah menutup seluruh kontrak
  transport CodeBuddy.
- **`force_stream: true` tidak akan berlaku dari registry.** Itu keputusan yang sudah dipaku test
  `TestTransport_ForcesStreamIsDeclaredByTheConnector`. Provider ini akan dilayani connector default
  sampai ada connector yang mendeklarasikannya, dan itu pekerjaan terpisah.
- **Tidak ada perubahan yang dikerjakan di sini.** Dokumen ini hanya mencatat; nol berkas produksi
  `app-serv` disentuh.

## 8. Keputusan yang dibutuhkan owner

1. **Jalur penambahan entri**: regenerasi penuh dari `a8c9d38` (F3 pilihan A) atau tambalan dua entri
   dengan jalur generator yang eksplisit (F3 pilihan C).
2. **Cakupan**: hanya CodeBuddy, atau sekalian menutup drift 25 entri itu sebagai satu perubahan
   tersendiri setelah CodeBuddy masuk.
3. **Urutan**: apakah CodeBuddy dikerjakan sebelum atau sesudah `docs/DRAFT/010` selesai. Draft 010
   masih memegang `app-serv/README.md` dan `SYSTEM_MAP.md`, jadi mengerjakan keduanya sekaligus akan
   membuat dua register menyunting baris yang sama.

Tidak ada source `app-serv` yang boleh diubah sebelum owner memilih nomor dan jalur di atas.

## 9. Status per 2026-09-22

- **Sisi panel CLOSED untuk scope ini.** Permintaan owner memuat empat provider. Dua di antaranya
  (OpenAI Compatible, Anthropic Compatible) selesai di `app-ui` lewat section Custom provider di
  `/providers` dan halaman detail node dengan Edit, Test, dan Delete. Dua sisanya (CodeBuddy CN,
  CodeBuddy Int) adalah pekerjaan registry `app-serv` dan tercatat di dokumen ini sebagai F1 sampai
  F4, belum dikerjakan.
- **F1, F2, F3, F4 terbuka.** Tidak ada yang CLOSED per tanggal dokumen ini.
