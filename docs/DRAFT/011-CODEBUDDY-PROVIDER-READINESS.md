# 011-CODEBUDDY-PROVIDER-READINESS.md: Kesiapan Provider CodeBuddy CN dan CodeBuddy Int di Registry app-serv

Dokumen kerja hasil pemeriksaan registry `app-serv/.` untuk dua provider yang diminta owner pada
2026-09-22. Bukan kontrak; kontrak tetap `docs/SPEC-API/001-SPEC-API.md` (wire) dan
`docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel). Dokumen ini mengikuti pola
`008-SKILL-ENDPOINT-READINESS.md` dan seterusnya: temuan bernomor F, bukti yang bisa diulang,
rencana DURING, dan keputusan owner di depan implementasi. Tidak ada kode produksi yang diubah
**pada tanggal audit ini**; pekerjaan yang dikerjakan sesudahnya dicatat di §10 tanpa menulis ulang
§1-§9, karena bagian itu adalah hasil pengukuran 2026-09-22.

| | |
|---|---|
| **Status** | F1, F2, F4 CLOSED per 2026-09-29 (§10), dan F3 tertutup lewat jalur keempat yang tidak ada di §8. F6 CLOSED: akun OAuth tanpa identitas kini jadi baris sendiri (§10.11). F7 CLOSED: vendor menolak list pesan OpenAI polos, connector sekarang membentuknya (§10.12). Yang masih terbuka satu: **F5**, gap connector untuk `force_stream` di empat entri lain (§10.7). Round state CodeBuddy sudah terbit lewat rute aslinya terhadap vendor dan panel tidak lagi menolaknya (§10.10). Sisi panel sudah CLOSED di scope-nya sendiri |
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

## 10. Status per 2026-09-29: F1, F2, F4 CLOSED; F3 tertutup lewat jalur keempat; F5 dibuka

Bagian ini menutup temuan yang dibuka §3-§6 dan mengoreksi klaim §2/§7 yang sudah tidak benar di
hari pengukuran ini. §1-§9 dibiarkan apa adanya sebagai hasil pengukuran 2026-09-22.

### 10.1 Yang berubah pada registry

Registry yang di-embed hari ini diukur dengan perintah yang sama seperti di §2:

```bash
grep -c '^  - id: ' app-serv/internal/registry/registry.yaml          # 34 entri
grep -c 'hidden: true' app-serv/internal/registry/registry.yaml       # 0
grep -c 'has_oauth: true' app-serv/internal/registry/registry.yaml    # 11
sed -n '1,8p' app-serv/internal/registry/registry.yaml                # revision + catatan generator
```

- Header: `revision: 9router@39e36d3d (2026-09-23)` dan satu baris tambahan `# Provider set: the
  owner's KEEP list (2026-09-26), enforced in the generator.`
- `codebuddy-cn` ada di `registry.yaml:1580`, `codebuddy-intl` di `:1653`. Keduanya `hidden: false`,
  `category: oauth`, `has_oauth: true`, `auth_modes: [oauth, apikey]`, alias `cbcn`/`cbai`,
  base URL `/v2/chat/completions`, `force_stream: true`, `thinking_format: openai`, enam header
  transport per wilayah (termasuk `x-codebuddy-request: "1"`), `usage.url` di domain wilayahnya
  masing-masing, dan blok `oauth` dengan `base_url`, `state_url`, `token_url`, `refresh_url`,
  `user_agent`, `platform`, `poll_interval`.
- Tidak ada entri `codebuddy` gabungan: `grep -n '^  - id: codebuddy$'` mengembalikan nol.

### 10.2 F1 CLOSED

Kriteria §3 terpenuhi, termasuk test yang sebelumnya belum ada. Yang membuktikannya sekarang:
`app-serv/internal/service/provider_codebuddy_listed_test.go`, yang memuat registry yang di-embed
(bukan index fixture) dan memaku:

- `List` tanpa filter melayani kedua entri, dan `Detail` menjawab untuk kedua id;
- field yang panel pakai untuk memutuskan tampilan: `has_oauth`, `auth_modes`, `category`, alias;
- bentuk yang port ini bergantung padanya: `base_url`, `force_stream`, `usage.url`, enam header,
  dan `OAuth.StateExchangeFlow()` benar untuk kedua wilayah;
- `List(q=codebuddy)` mengembalikan tepat dua entri, jadi tidak ada provider ketiga yang muncul
  dari nama yang sama.

Test itu gagal bila salah satu entri ditandai hidden atau salah satu field kehilangan nilainya, yang
persis perbedaan antara "YAML hari ini benar" dan "YAML hari ini tetap benar saat ada yang menyunting".

### 10.3 F2 CLOSED

Reference diikuti apa adanya: dua entri, dua id, dua alias, dua domain. Kriteria "tidak ada entri
`codebuddy` gabungan yang tersisa" terukur di §10.1, dan alias `cbcn`/`cbai` hanya muncul pada
`alias` dan `ui_alias` masing-masing wilayah (`grep -c -E 'cbcn|cbai'` = 4).

### 10.4 F3 tertutup lewat jalur yang tidak ada di §8

Owner tidak memilih A, B, atau C. Yang dikerjakan adalah regenerasi penuh dari revisi reference yang
sekarang dipin (`39e36d3d`) **plus** satu aturan penyaring di generator: `KEEP_PROVIDERS`
(`app-serv/tools/registry-gen.mjs:44`), yang berisi id yang port ini layani, termasuk kedua id
CodeBuddy. Hasilnya registry lebih kecil dari kedua angka yang §5 bandingkan (34, bukan 94 atau 119),
dan generator mencetak laporan entri reference yang berada di luar KEEP set supaya penyaringan itu
terlihat, bukan tersembunyi.

Aturan "do not edit by hand" di header YAML tetap pegang: kedua entri CodeBuddy masuk lewat
generator, bukan lewat tambalan pada YAML. Itu yang membuat pilihan C di §5 tidak perlu diambil.

### 10.5 F4 CLOSED

Angka 94 masih tertulis di tiga tempat hidup dan semuanya sudah dibetulkan di perubahan ini:

| Tempat | Sebelum | Sesudah |
|---|---|---|
| `app-serv/README.md:25` | "94 provider" | "34 provider (revisi `9router@39e36d3d`, disaring daftar KEEP owner 2026-09-26)" |
| `app-serv/internal/schema/provider.go:69` | "94 entries in the P1 document" | tidak menyebut angka lagi: komentar itu menjelaskan *sifat* daftar (seluruh set dijawab satu response), jumlah bukan bagian dari poinnya |
| `SYSTEM_MAP.md:8` | "(94 provider, decode ketat)" | "(34 provider, decode ketat)" |
| `docs/DRAFT/010-USAGE-ENDPOINT-READINESS.md:1084` | "94 provider" | tetap 94, dengan catatan bahwa angka itu diukur pada tanggal dokumen dan menunjuk ke §10 ini |

Satu kalimat yang bagian "Kriteria selesai" F4 tunggu juga sudah tidak benar bentuknya:
`docs/SPEC-UI/001-SPEC-UI.md`
tidak lagi mengklaim "one `has_oauth` provider", dan yang benar adalah 11 entri `has_oauth` (terukur
di §10.1), jadi angka itu tidak lagi ditulis sebagai klaim tunggal.

### 10.6 Koreksi untuk satu klaim §2 dan tiga klaim §7 yang sudah basi

Klaim-klaim ini ditulis 2026-09-22 dan tidak lagi benar; dicatat di sini supaya pembaca tidak
memakainya sebagai fakta hari ini.

- §2 bullet "ThinkingFormat dan Transport.Usage hanya dideklarasikan, tidak ada pembaca runtime":
  **tidak berlaku lagi.** `ThinkingFormat` dibaca `internal/reasoning/applier.go:197` dan
  `internal/registry/capability_levels.go:95`. `Transport.Usage.URL` dibaca
  `internal/service/quota_usage.go:132` dan `:154`, lalu dipakai `quotafetch.usageEndpoint()`, yang
  membuat registry sumber tunggal endpoint kuota dan `familyEndpoints` hanya fallback bagi keluarga
  yang tidak mendeklarasikannya (vercel-ai-gateway). Header transport ikut dibaca untuk hal yang
  sama lewat `Credentials.UsageHeaders`.
- §7 bullet "Section OAuth akan hidup sendiri, jalur start tetap dorman": **tidak berlaku lagi.**
  State round CodeBuddy sudah diport di `internal/service/oauth_client_state.go` dengan poll
  `?state=`, kode `11217` sebagai pending, dan refresh lewat header. `flowKind`
  (`internal/service/oauth_flow_refresh_policy.go:33-41`) menggolongkannya sebagai `device`, jadi
  panel menawarkan flow yang bisa dijalankan. Yang terbukti adalah round-trip terhadap vendor palsu
  di `oauth_flow_state_test.go` dan `oauth_client_state_test.go`; **connect live terhadap layanan
  Tencent belum pernah dijalankan.**
- §7 bullet "`force_stream: true` tidak akan berlaku dari registry; provider dilayani connector
  default": **sebagian tidak berlaku lagi.** Keputusan masih di connector (test
  `TestTransport_ForcesStreamIsDeclaredByTheConnector` tidak diubah), tapi CodeBuddy sekarang punya
  connector sendiri: `internal/provider/codebuddy.go` mendeklarasikan `ForcesStream() == true` dan
  didaftarkan untuk kedua wilayah di `cmd/app-serv/provider_wiring.go:86-92`. Boot log membuktikan
  `connectors:7`.

- §7 bullet "`thinking_format` dan `usage.url` bukan blocker untuk daftar; keduanya inert dan hanya
  relevan bila nanti ada pekerjaan penalaran dan kuota": **tidak berlaku lagi untuk `usage.url`.**
  Field itu sekarang punya pemanggil: `internal/service/quota_usage.go:132` dan `:154` menyalin
  `transport.usage.url` dan `transport.headers` entri ke `quotafetch.Credentials`, dan
  `usageEndpoint()` (`internal/service/quotafetch/dispatcher.go:48`) memakainya sebelum tabel bawaan
  per keluarga. Artinya alamat endpoint kuota CodeBuddy cukup dideklarasikan sekali, di entri;
  `familyEndpoints` tinggal fallback bagi keluarga yang tidak mendeklarasikannya
  (`vercel-ai-gateway`). Header yang sama ikut dipakai karena endpoint dan identitas harus datang
  dari satu deklarasi yang sama: test `TestCodeBuddy_ReadsItsEndpointAndHeadersFromTheEntry` menahan
  keduanya, termasuk arah sebaliknya (tanpa deklarasi, path keluarga yang jalan).

### 10.7 F5 (LOW, TERBUKA): empat entri mendeklarasikan `force_stream` tanpa connector yang memutuskannya

Fakta, terukur dengan:

```bash
awk '/^  - id: /{id=$3} /^      force_stream: true/{print id}' \
  app-serv/internal/registry/registry.yaml
# openai, opencode, codebuddy-cn, codebuddy-intl, commandcode, grok-cli, zed
```

Yang punya connector mendeklarasikan forced stream: `opencode` (`internal/provider/opencode.go`) dan
`codebuddy-cn`/`codebuddy-intl`. Yang tidak: **`openai`, `commandcode`, `grok-cli`, `zed`.** Untuk
empat provider itu, jawaban non-stream dibaca berbeda dari reference, yang persis alasan test
`TestTransport_ForcesStreamIsDeclaredByTheConnector` menahan field registry agar tidak dibaca
langsung.

Ini pekerjaan terpisah dari draft ini dan tidak ada satu pun provider CodeBuddy yang terpengaruh.
Yang dibutuhkan untuk menutupnya: satu connector per provider (atau satu connector bersama yang
membaca deklarasi itu), masing-masing dengan test yang membuktikan frame stream sampai ke client.
Belum ada keputusan owner tentang apakah keempatnya memang akan dilayani port ini; entri ini dibuka
supaya gap itu tercatat di register dan tidak hilang sebagai pengetahuan lisan.

### 10.8 Satu catatan bentuk: pembaca kuota CodeBuddy kini dua berkas

Setelah `transport.usage.url` dan `transport.headers` ikut dibaca (§10.6), `internal/service/quotafetch/codebuddy.go`
tepat menyentuh batas 250 baris yang AGENTS.md §1.1 pasang. Ia dibagi pada batas concern yang sudah ada,
bukan pada batas yang dicari-cari:

- `codebuddy.go` (132 baris): request, dua kegagalan lunak yang tetap lunak (`refusedCredential`,
  `billingRejected`), dan pembukaan envelope berlapis.
- `codebuddy_packs.go` (142 baris): bentuk paket kredit dan aturan yang memisahkan refill dari bonus,
  konstanta `refillGap`, dan urutan window berdasarkan expiry.

Tidak ada perilaku yang berubah; test `internal/service/quotafetch` hijau tanpa mengubah satu assertion
pun, karena pembagian ini hanya memindahkan kode ke berkas yang alasannya satu hal.

### 10.9 Apa yang jalur request baca dari kedua entri, plus satu field yang belum punya pembaca

Diperiksa setelah connector masuk, supaya klaim "terintegrasi" bisa disebut per field dan bukan per
perasaan. Untuk `codebuddy-intl` (`registry.yaml:1653`):

| Field entri | Pembacanya | Bentuk yang dikirim |
|---|---|---|
| `transport.base_url` | connector `Default` lewat seam `Endpoint()` | `https://www.codebuddy.ai/v2/chat/completions` |
| `transport.headers` | `Default` (dataplane menyalinnya ke request keluar) | enam header wilayah itu, termasuk `x-codebuddy-request: "1"` |
| `transport.auth` (`header`, `scheme`) | `internal/provider/default.go:160-173` | `Authorization: Bearer <token>`; skema kosong dinormalisasi ke `bearer` di sana, bukan dibiarkan telanjang |
| `transport.force_stream` | tidak dibaca sebagai field; keputusan di connector (§10.6) | jawaban selalu dialirkan sebagai stream |
| `transport.usage.url` + `transport.headers` | `quota_usage.go:132`/`:154` → `usageEndpoint()` | endpoint kuota dan identitasnya datang dari entri yang sama |
| `oauth.*` | `oauth_client_state.go` (state round, poll, refresh per header) | flow `device` yang bisa dijalankan |

Satu field belum punya pembaca di jalur request: `transport.auth.combined: true`, yang reference
pasang di kedua entri CodeBuddy. Di `internal/registry/types_transport.go:72` field itu dideklarasikan
dan dideskripsikan ("one header carries the credential and the scheme"), tapi `grep` atas
`internal/provider` dan `internal/dataplane` tidak menemukan pemakainya. Untuk CodeBuddy itu tidak
mengubah apa pun: `Bearer ` + token adalah juga hasil yang sudah dikirim untuk provider ini. Yang
perlu dicatat adalah batas klaimnya: kalau ada vendor yang butuh bentuk `combined` berbeda, field itu
hari ini tidak melakukan apa-apa, dan itu pekerjaan terpisah, bukan sesuatu yang sudah "terport".

### 10.10 Koreksi §10.6 dari kenyataan: panel menolak round CodeBuddy, dan itu bug kontrak

Bulet §10.6 yang mengklaim state round sudah bisa dijalankan panel ("`flowKind` ... jadi panel
menawarkan flow yang bisa dijalankan") **sebagian salah**, dan owner menemukannya sendiri pada
2026-09-29: menekan Start di section OAuth CodeBuddy menjawab

> Gagal OAuth: Unexpected response from the gateway at user_code: Too small: expected string to have >=1 characters

Yang benar adalah: panel **menawarkan** round itu, lalu **menolak** hasilnya. Penyebabnya bukan service
layer. Bentuk state round memang tidak punya code singkat (`oauth_client_state.go` `StateRound` hanya
mengembalikan `state` + `authUrl`; `oauth_flow_state_test.go:62-63` sudah memaku `UserCode` kosong sebagai
jawaban yang benar), tapi wire-nya masih menjanjikan field itu sebagai wajib: `internal/schema/oauth.go`
menulis `"user_code": ""` dan kontrak `docs/CONTRACT/001-CONTRACT-API-V1.yaml` mencantumkannya di daftar
`required:`. Schema panel (`src/lib/schemas/oauth.ts`, `user_code: z.string().min(1)`) menolak sebelum
render, dan `src/lib/api/client.ts:120` memformat issue Zod itu menjadi kalimat yang owner baca.

Dua jalan keluar yang **tidak** diambil, dengan alasannya: meniru code dari state (delapan karakter
pertama, misalnya) memberi operator sesuatu yang tidak pernah diminta vendor, dan code fiktif bukan data
nyata; kata flow baru (`state`) menuntut panel mengenal nilai baru padahal perilaku yang operator lihat
identik — buka link, setujui, panel bertanya sampai vendor memberi token.

Yang dikerjakan: `user_code` menjadi **kondisional di kontrak** (dihapus dari `required:`, `minLength: 1`
tetap menahan nilai yang hadir, `internal/handler/openapi.json` di-regenerate), `omitempty` pada tag Go,
`.optional()` pada schema Zod, dan blok "Your code" di `ProviderOAuthDevice.svelte` hanya dirender bila
round memang membawa code. Aturan "panel tidak memanggil `window.open`" dan bentuk round nonce tidak diubah.

Dibuktikan dari dua sisi. `TestDeviceStartAnswersAVendorMintedRoundCarriesNoShortCode`
(`internal/handler/oauth_device_state_round_test.go`) gagal sebelum fix dengan persis body `user_code:`
kosong dan kini menuntut key itu **tidak ada**, sementara
`TestDeviceStartAnswersTheVerificationRound` tetap memaku code nonce. Di panel: satu kasus schema menerima
round tanpa code, satu kasus menolak code berupa string kosong, dan satu kasus komponen memastikan link
plus loop polling tetap jalan tanpa blok code. SPEC-API §7.4 (bab "A round can also be minted by the
vendor"), SPEC-UI §6.3, dan `SYSTEM_MAP.md` ikut disesuaikan.

Sisa yang belum terbukti dari luar tinggal satu lapis: round live memang sudah dijalankan. Pada
2026-09-29 `POST /api/v1/providers/{id}/oauth/device/start` didorong lewat `http.ServeMux` nyata dengan
handler, service, index dan `OAuthHTTPClient` produksi (driver sekali-pakai di luar tree, tidak
di-commit), terhadap vendor aslinya untuk kedua wilayah:

```
codebuddy-intl -> status 200, keys [device_code expires_in interval_seconds verification_url]
                  body {"device_code":"3d09a2a0-…","verification_url":"https://www.codebuddy.ai/login?platform=ide&state=3d09a2a0-…","interval_seconds":5,"expires_in":300}
codebuddy-cn   -> status 200, keys [device_code expires_in interval_seconds verification_url]
                  body {"device_code":"041cca22-…","verification_url":"https://copilot.tencent.com/login?platform=CLI&state=041cca22-…","interval_seconds":5,"expires_in":300}
```

`user_code` **absent** di kedua jawaban, dan `verification_url` adalah halaman login vendor yang
membawa state itu — jadi body yang dulu ditolak schema panel sekarang terbit apa adanya dari jalur
handler yang sama. Yang masih menunggu akun owner hanyalah langkah sesudahnya: operator membuka link
itu dan menyetujui, sampai poll menjawab `connected`.

### 10.11 F6 CLOSED (2026-09-29): akun OAuth tanpa identitas kini jadi baris sendiri

Temuannya, sebagaimana dicatat saat dibuka: `StatePoll` tidak pernah punya identitas untuk
dikembalikan, dan kita **mengarang** satu email sintetis (`codebuddy-intl-user-`) yang nilainya sama
untuk setiap login di region itu. Karena `connectAccount` mendedup lewat
`FindOAuthEndpoint(providerID, account.Email, account.WorkspaceID)` — SQL-nya
`(($2 <> '' AND account->>'email' = $2) OR ($3 <> '' AND account->>'workspace_id' = $3)) ORDER BY
created_at, id LIMIT 1` (`internal/repository/postgres/endpoint_batch.go:130-146`) — login kedua selalu
cocok ke baris tertua dan **menimpa** kredensial akun pertama. `UNIQUE (provider_id, label)`
(`migrations/000005_upstream_endpoints.up.sql:29`) bahkan menahan baris kedua kalau pun dedupnya lewat.

Yang menentukan arah perbaikan adalah reading atas reference, bukan ide kita sendiri: di sana
`createProviderConnection` **hanya** mendedup ketika `data.email` ada
(`9router/src/lib/db/repos/connectionsRepo.js:133`), dan CodeBuddy `mapTokens` memang mengembalikan
`providerSpecificData: {}` tanpa email (`src/lib/oauth/providers/codebuddy-intl.js:65-70`) — jadi tiap
login di reference adalah baris UUID baru, dan label akun kedua `"Account ${all.length+1}"` (`:181`).
Artinya multi-akun reference bukan mekanisme identitas, melainkan **ketiadaan kunci dedup**. Fix yang
benar: berhenti mengarang kunci.

Yang dikerjakan:

- `deviceAccountEmail` (`internal/service/oauth_flow_device_poll.go:123-136`) mengembalikan string kosong
  untuk state round, dan kehilangan parameter `providerID` yang tidak lagi dipakai. Round PKCE tetap
  `qoder-user-<user_id>` apa adanya, karena baris yang sudah tersimpan di DB dicocokkan lewat nilai itu.
  Konsekuensinya enak: dengan email dan workspace kosong, kedua branch SQL di atas tidak aktif, jadi
  **tidak ada perubahan repository** untuk mendapat "selalu baris baru".
- `accountLabel` (`internal/service/oauth_flow_connect.go`) menamai baris baru: identitas dari vendor →
  label seperti sebelumnya; tidak ada identitas → `Account N`, N **nama pertama yang belum dipakai** oleh
  provider itu, dibaca lewat `Store.List` satu halaman (`internal/repository/postgres/endpoint.go:60-66`).
  Bukan `jumlah baris + 1`: hitungan itu adalah bug yang sama dengan bentuk rupangan — kalau `Account 1`
  dihapus dan `Account 2` masih ada, login berikutnya mendapat nama yang sudah dipegang baris tersisa,
  dan `UNIQUE (provider_id, label)` menolak akun yang vendor baru saja berikan. `connectLabel` yang lama
  dihapus karena cabang fallback-nya menjadi unreachable; dua fungsi bernama sama adalah cara lain untuk
  membuat aturan ini bercabang lagi.
- Label akun yang **tidak** diberi identitas oleh vendor kini tidak melewati `schema/endpoint.go:170-175`
  (`omitempty,email`), jadi email sintetis yang dulu juga tidak pernah bisa round-trip lewat API.

Dibuktikan oleh `internal/service/oauth_flow_multiaccount_test.go`: tiga login berurutan menghasilkan
tiga endpoint berbeda dengan credential masing-masing (`access-state-1/2/3`), label `Account 1/2/3`,
tanpa field identitas (`Email`/`Name`/`WorkspaceID` kosong) dan `machine_id` yang beda per ronde;
`TestStateRound_NumberedLabelTakesAFreeName` menutup lubang rupangan (baris yang tersisa hanya
`Account 2` → login baru memakai `Account 1`);
`TestDeviceAccountEmail_LeavesIdentitylessRoundsUnKeyed` menahan kedua arah (state: kosong, PKCE:
prefix historis). Satu case lama di `oauth_flow_callback_test.go` dipaksa berubah ekspektasi: provider
callback tanpa userinfo dulu memakai label konstan `pkce-provider oauth`, sekarang `Account 1` — dan itu
bukan regresi, karena label konstan itu membuat login identitas-kosong **kedua** pada provider yang sama
gagal di index UNIQUE.

Sweep refresh tidak perlu disentuh: ia sudah berjalan per `endpoint.EndpointID`
(`internal/service/oauth_refresh_worker.go:94-118`), jadi tiap akun renewed sendiri. Panel juga tidak:
tabel akun di-key `endpoint.endpoint_id` (`app-ui/src/lib/components/ProviderOAuthAccounts.svelte:99-160`).

### 10.12 F7 CLOSED (2026-09-29): vendor menolak list pesan OpenAI polos dengan `11101`

Login berhasil, `oauth/status` 200, `models/test` 200 — dan provider tetap tidak bisa dipakai. Penyebabnya
bukan kredensial: CodeBuddy menjawab body OpenAI polos dengan **`11101 invalid request`** dan reference
menghindarinya dengan menyusun ulang `messages` di
`open-sse/executors/codebuddy-intl.js:20-38` — satu turn `system` pembuka `"You are CodeBuddy Code."`,
`system`/`developer` milik klien dibuang, konten `user` berupa string diangkat jadi typed blocks
`[{type:"text",text:…}]`, `reasoning_effort` `none`/`off` dihapus dan effort lain dicerminkan sebagai
`reasoning_summary: "auto"`. Connector kita justru mendeskripsikan dirinya "tidak melakukan rewriting"
(`internal/provider/codebuddy.go`) dan komentar wiring mengulang klaim yang sama
(`cmd/app-serv/provider_wiring.go`) — dua pernyataan itu salah dan sudah diganti.

Keputusan owner (2026-09-29): **jangan buang instruksi klien.** System/developer text caller digabung ke
satu turn `system` leading (`"You are CodeBuddy Code.\n\n<punya klien>"`), jadi request tetap punya satu
system message seperti yang vendor minta tanpa kehilangan steering yang dipakai agent/combo. Konten yang
sudah berbentuk blok (jalur vision kita) disalin apa adanya; turn `assistant`/`tool` dipertahankan lengkap
dengan field-nya; body yang bukan object, `messages` yang bukan list, dan `reasoning_effort` non-string
**ditolak**, tidak diteruskan dalam keadaan separuh diubah.

Implementasinya `internal/provider/codebuddy_body.go`, menempel di seam opsional
`provider.Transformer` (`internal/provider/plugin.go:85-92`) yang dipanggil `applyShape`
(`internal/dataplane/transport_shape.go:39-49`) sebelum URL dibangun — jadi core tetap tidak bercabang pada
provider id, dan jalur `models/test` ikut terbaiki karena probe adalah panggilan data plane sungguhan
(`internal/service/provider_model_probe.go:15-16`).

Dibuktikan dua lapis: unit (`internal/provider/codebuddy_body_test.go`: merge system, typed blocks, array
utuh, turn lain tidak disentuh, mirror reasoning tiga kasus, body tak terbaca ditolak, byte-stable) dan
wire (`internal/dataplane/engine_codebuddy_wire_test.go`: body yang benar-benar keluar dari transport
membawa satu system leading gabungan, user sebagai blok bertipe, `stream: true`, dan jawaban stream
ter-lipat jadi satu completion untuk klien non-stream).

Dan live, 2026-09-29, dua panggilan ke `https://www.codebuddy.ai/v2/chat/completions` memakai akun yang
sudah tersimpan di DB lokal (`ep_0387DB1598…`, satu baris untuk region itu) lewat driver sekali-pakai yang
menjalani transform, endpoint, dan auth connector itu sendiri — bukan HTTP body yang ditulis tangan. Driver
itu sudah dihapus dan tidak pernah ada di tree:

1. `{"role":"user","content":"Reply with one word only: pong"}` → **200**, stream berisi delta `pong`,
   `finish_reason: stop`, usage 33/2/35. Body yang keluar: satu `system` leading berisi prompt vendor
   **plus** instruksi caller, dan `user` sebagai `[{type:"text",…}]`. Ini jawaban atas pertanyaan yang
   membuat §10.11 dibuka: vendor menerima system turn gabungan, jadi instruksi klien tidak perlu dibuang
   seperti reference melakukannya.
2. `reasoning_effort: "high"` → **200** dengan `reasoning_content` yang mengalir per delta, dan body yang
   keluar membawa `reasoning_summary: "auto"` di samping effort-nya — persis bentuk yang reference kirim.

Yang belum bisa diukur dari sini: login **kedua** pada region yang lain, karena ronde state butuh operator
membuka halaman vendor dan menyetujuinya di browser. Yang terbukti adalah sisi gateway-nya: akun kedua tidak
lagi cocok ke baris pertama (dedup tidak punya kunci), dan labelnya `Account 2`.
