# 045-APP-UI-POINTER-ROT.md: Komentar app-ui menyimpan koordinat, dan koordinat itu yang lebih dulu busuk

Dokumen kerja pass ini menutup audit antislop 007 atas `app-ui`. Auditnya ada di
`anti-slop/audit-007-2026-10-07.md` (direktori itu di-ignore git, jadi keputusan dan buktinya diulang di sini).
Yang disentuh pass ini: **komentar**. Nol baris executable, nol identifier, nol import, nol format, nol logika
di `app-ui/**`. Ini dibatasi oleh scope guardrail antislop-code, dan ditegakkan dengan `git diff` di bagian Hasil.

Kenapa pass ini perlu ada, dan kenapa ia bukan lanjutan otomatis dari 001-004: kelas slop generik di panel sudah
bersih (dash, separator, banner, end marker, emoji, TODO, narasi langkah: semuanya 0 terukur). Yang tersisa adalah
kelas yang tidak bisa dilihat dari bentuk kalimatnya. Komentar-komentar panel bagus dalam menjelaskan *kenapa*, dan
mereka menuliskan "kenapa" itu sebagai koordinat: nomor baris ke `app-serv`, nomor baris ke reference fork, nomor
draft, tanggal. Koordinat busuk tanpa suara. Yang busuk di sini sudah busuk, dan tiga di antaranya sekarang mengirim
pembaca ke deklarasi yang salah.

| | |
| --- | --- |
| **Status** | **KERJA. HIGH (F1, F2, F3) dikerjakan pada pass ini. MEDIUM dan LOW tercatat, belum dikerjakan** |
| **Mechanism** | AFTER (audit 007) lalu DURING untuk tulisan baru |
| **Scope** | Komentar di `app-ui/src/**` dan `app-ui/tests/**`. Perkiraan 47 file: 11 titik pointer `.go`, 41 file dengan pointer `.js`/`.css` (tumpang tindih), 3 file klaim perilaku |
| **Sumber temuan** | `anti-slop/audit-007-2026-10-07.md`, terukur atas 5.879 baris komentar di 320 file `src` + 3.141 di `tests` |
| **Struktural (AGENTS.md §1.9)** | N/A, no structural change: tidak ada domain boundary, data structure, service interaction, atau async topology yang bergerak |

## Aturan bentuk yang dipakai pass ini

Audit 007 menutup dengan alasan bahwa F1-F2-F5-F6 bukan cleanup, melainkan keputusan tentang *ke mana sebuah komentar
boleh menunjuk*, dan keputusan itu mengubah konvensi untuk setiap file panel berikutnya. Bentuknya sekarang, dan ini
yang dipakai pada semua edit di bawah:

1. **Pointer ke deklarasi menyebut simbolnya, bukan rentang barisnya.** `app-serv internal/schema/endpoint.go`,
   `EndpointKeyInput` bertahan terhadap suntingan apa pun pada file itu dan bisa di-grep; `:80-95` tidak bertahan
   terhadap apa pun. Ini perluasan dari pilihan yang sudah dibuat §4 untuk `SPEC-API §7.5`: 515 pointer section di
   panel tidak satu pun busuk, dan itu alasan kelas ini masih bisa diselamatkan.
2. **Pointer ke reference fork menyebut namanya dan tidak membawa nomor baris.** Identitas fork sudah tercatat sekali
   di `docs/SPEC-UI/001-SPEC-UI.md` §15 (`9router-app` v0.5.55, HEAD `9766494`, 2026-09-09, `/home/rusmanadodi/ai-gateway`).
   Satu penunjukan itu cukup; 66 rentang baris ke file yang tidak ada di repo ini dan tidak ada di path itu lagi tidak
   menambah apa pun. **Angka yang merupakan fakta tetap ditulis** (`nodeH = 30`, offset `-36` per siklus, `node = 16`
   kartu): yang busuk adalah rentang barisnya, bukan nilainya.
3. **Komentar tidak boleh mengklaim perilaku yang kode tidak lakukan.** Ini kelas yang sama yang menaikkan prioritas
   di penutupan audit 006, dan di sini ia punya satu kandidat cacat fungsional (F3a).

Yang tidak dilakukan: menghapus komentar panjang hanya karena panjang. §2.6 berlaku, "blok yang benar-benar memegang
lima constraint mengambil lima baris; itu pass, bukan near-miss". F7 (narasi sejarah) sengaja ditinggal untuk pass
berikut karena ia butuh pembacaan kalimat per kalimat, bukan substitusi pola.

**Catatan status aturan di atas:** ketiganya masih aturan kerja pass ini, bukan aturan proyek. Ia belum masuk
`docs/RULLES/ANTISLOP.md`, dan seharusnya masuk lewat keputusan F4, karena F4 adalah pertanyaan "gerbang mana yang
membaca panel" dan aturan tanpa tempat di dokumen adalah aturan yang akan hilang bersama dokumen ini. §2 membuka
dirinya dengan janji tidak mengklaim enforcement yang tidak ada, jadi menambah aturan ke §2 tanpa cek adalah
pelanggaran yang sama. Yang bisa dilakukan pass berikutnya tanpa menunggu: menulis aturan itu ke §3 (review-only,
jelas bukan machine-checked), yang tidak menuntut gerbang baru.

## Temuan

### F1 (HIGH) 16 pointer menunjuk `app-serv` dengan nomor baris, 3 di antaranya sekarang salah deklarasi

`endpoint-write.ts` menyebut `app-serv` lima kali. Diresolusi ke pohon saat ini:

| Di mana | Menyebut | Mengklaim | Yang sebenarnya |
| --- | --- | --- | --- |
| `:9` | `internal/schema/endpoint.go:80-95` | bentuk `{label?, value, priority?}` | `EndpointKeyInput` di **65-73**; 80-95 adalah `CreateEndpointRequest` dan `UpdateEndpointRequest` |
| `:10` | `internal/schema/validator.go:44-48` | `DisallowUnknownFields` | panggilan itu di **baris 94**, di dalam `func jsonDecoder`; 44-48 adalah `registerClearingURLRules` |
| `:33` | `internal/schema/endpoint.go:56-62` | `ParseAuthType` | di **baris 43**; 56-62 adalah `TestNodeRequest` |
| `:14` | `endpoint.go:122-136` | `BulkEndpointInput` | benar |
| `:174` | `endpoint.go:130-136` | `BulkAddKeysRequest` | benar |

Semuanya **benar saat ditulis**. Pada `57125b6` (2026-09-23), commit yang menambahkan komentar ini, file itu memegang
`ParseAuthType` di 56, `TestNodeRequest` di 74, `EndpointKeyInput` di 80, dan `validator.go` memegang
`DisallowUnknownFields` di 46. Pergeseran mendarat di `2ab0bb4` (2026-10-06), `docs(go): put every §1.2 field value
back on one line`, sweep ANTISLOP.md §2.7: `endpoint.go` turun dari 183 ke 170 baris. `validator.go` lebih dulu
menggeser panggilan yang sama dari 46 ke 99 pada `cb43461` (2026-10-05), lalu ke 94.

Jadi mekanismenya persis dan tidak nyaman: remediation yang menutup audit 006 item 2 diam-diam merusak komentar
frontend tentang backend-nya. Tidak ada yang bisa menangkapnya. `contract-drift.sh` membandingkan daftar kode
SPEC-API §8 dengan `app-ui/src/lib/schemas/error.ts`, itu cek nyata atas kontrak nyata; tidak ada cek mana pun yang
membaca nomor baris sebuah komentar, dan memang tidak bisa, karena nomor baris bukan kontrak.

Sisa target yang sudah diresolusi untuk perbaikan: `api_key` butuh satu kunci ada di `service/endpoint_create.go`
(`if in.AuthType == domain.UpstreamAuthAPIKey && len(in.Keys) == 0`, pesan "an api_key endpoint requires at least one
key"), cap batch 50 ada di `service/endpoint_bulk.go` (`maxBatchRows`, "a batch may hold at most 50 endpoints"),
`display_name` required ada di `schema/model.go` (`CreateCustomModelRequest.DisplayName`).

### F2 (HIGH) 66 pointer menunjuk file yang tidak ada di repo ini, dan tidak ada di mana pun

63 pointer ke `.js` dan 3 ke `globals.css`; `find` atas seluruh repo tidak mengembalikan satu pun:

```
ProviderTopology.js:137-245   AddCompatibleModal.js:7-30    ModelSelectModal.js:216-219
EndpointPageClient.js:647-668 AddApiKeyModal.js:148-182     globals.css:504-535
UsageStats.js:492-493         TopModelsChart.js:40          bulkAdd.js:76-96
```

18 nama file target, 41 file sumber yang menyimpannya. Ini reference fork tempat panel diport. Faktanya nyata dan
harus tinggal: offset animasi beam `-36` per siklus karena keyframe fork bilang begitu, node ditata pada `nodeH = 30`,
fork menyimpan satu tabel copy per varian. Yang salah adalah penunjuknya. Hanya 5 dari 66 yang menyebut ref (`on
origin/master`), dan ref itu milik repo yang sudah tidak ada di disk: path yang tercatat di SPEC-UI §15,
`/home/rusmanadodi/ai-gateway`, hari ini tidak ada.

Ini kelas §4 satu langkah lebih jauh: `draft NNN` menunjuk dokumen yang setidaknya masih ada di `docs/DRAFT/`, ini
menunjuk luar repositories dan luar mesin.

### F3 (HIGH) Tiga komentar menyatakan perilaku yang kodenya tidak lakukan

Ditemukan dengan membaca komentar terhadap body-nya, satu-satunya cara kelas ini ketemu.

**a. `schemas/token-saver.ts:72-73`** menulis "The URL is optional because the group is usually disabled" di atas
`url: z.string()`. `z.string()` menuntut key ada. Yang memakai `optionalAbsoluteUrl` adalah skema write di `:97`,
dan kalimat kedua komentar itu sendiri mengakuinya: "the write schema below is the one that checks its shape." Jadi
kalimatnya mendeskripsikan saudara, bukan baris di bawahnya, bentuk yang persis dilarang §3. **Satu-satunya temuan di
sini dengan kandidat cacat fungsional:** kalau gateway sungguh menghapus `url` untuk grup yang disabled, skema read ini
menolak responsnya. Komentar tidak boleh menegrimia fakta itu; yang bisa dikerjakan tanpa mengubah kode adalah
menuliskan apa yang skema lakukan, dan meninggalkan pertanyaannya sebagai open question di dokumen ini.

**b. `schemas/endpoint-write.ts:156-157`** menulis "Capped at the API's own batch limit so a paste of a hundred rows is
refused". Cap yang dipakai adalah `keyRows` = `.max(MAX_KEYS_PER_ENDPOINT)` di `:84`, dan konstanta itu 100. Batas
batch adalah konstanta lain, `MAX_BULK_CONNECTIONS = 50` di `:165`, dan tidak diterapkan di sini. Lalu `.max(100)`
inklusif: seratus baris lolos, seratus satu ditolak. Kodenya benar: `app-serv` memvalidasi
`required,min=1,max=100` pada route itu (`BulkAddKeysRequest.Keys`), dan `:25-26` sudah menamai konstantanya dengan
tepat sebagai per-endpoint key cap. Hanya komentar ini yang salah nama dan salah boundary. Satu kalimat, dua koreksi.

**c. `routes/+error.svelte:4-5`** menulis "The only error the panel raises on its own is a route that does not exist,
so 404 is the case with real copy: it names the requested path and offers the way back." `COPY` memegang entri 403
lengkap dengan judul dan deskripsi sendiri, status di luar daftar mendapat fallback yang juga lengkap, dan aksi "Back
to endpoint keys" dirender untuk **setiap** status. Yang khusus 404 hanya elemen requested-path di `:48`. Jadi
kalimatnya menyebut pembeda yang salah, dan melakukannya di paragraf yang justru dipercaya pembaca untuk menjelaskan
tabel status.

### F4 (MEDIUM) Gerbang tidak membaca panel untuk tiga dari delapan aturan keras

§1 menyatakan protokol MANDATORY untuk setiap file hand-authored di `app-*/**`. Yang terjadi: cek sitasi
`[ "${f##*.}" = "go" ] || continue`, cek suppression men-grep `//nolint:` di `'*.go'`, cek panjang hanya Go dan hanya
file berubah. Cek struktural memangcovering `*.ts`/`*.svelte` lewat `CODE_GLOBS` dan di sini memang 0. Delapan aturan,
empat yang sampai. §2 membuka dirinya dengan "the document never claims enforcement it does not have".

### F5 (MEDIUM) 138 sitasi `draft NNN` di komentar panel, tidak terlihat cek apa pun

62 di `src` (35 file), 76 di `tests` (43 file), 127 di antaranya di baris komentar. remedy §4 satu klausa tiap titik.
342 pointer SPEC-UI/SPEC-API/PORT tidak dihitung dan tidak boleh disentuh.

### F6 (MEDIUM) 61 baris komentar membawa tanggal, 32 di antaranya provenance bukan constraint

`(owner directive, 2026-09-26)`, `measured live 2026-09-24`, `added server-side on 2026-09-25`. §1.2 sudah menulis
aturan untuk sisi Go dan ia pindah apa adanya: "git blame owns that".

### F7 (MEDIUM) Narasi migrasi, bentuk paling murni dari §2.6 "argued rather than stated"

`+layout.svelte:19-20`, `CombosToolbar.svelte:5-6` dan `:10`, `ComboStrategyFields.svelte:4-5`, `model-picker.ts:11-14`,
`usage-beam.ts:63-65`, `gateway-key.ts:67-68`, `endpoint-write.ts:11`. Satu di antaranya ("220-line warning") menyebut
anggaran yang tidak ditegakkan gerbang mana pun untuk `.svelte`, jadi ia juga klaim tentang alat yang tidak ada.

### F8 (MEDIUM) `src/lib/primitives/**` output generator yang tidak menandai dirinya

57 file, 40 baris komentar, 7 di antaranya suara generator. Proyek sudah memperlakukan direktori ini sebagai generated
dua kali, keduanya di luar kode: `.prettierignore` (prosa) dan `eslint.config.js:35` (`ignores`). Tidak ada marker di
dalam file, daftar exemption ANTISLOP.md §1 menyebut `*_gen.go`, `*.pb.go`, `openapi.json`, `node_modules` dan tidak
menyebut direktori ini, dan `tree_scan` membaca `*.svelte` tanpa pengecualian. Dua akibat: 7 gema generator dihitung
sebagai hutang komentar yang tidak ditulis siapa pun dan akan dipulihkan oleh `bun x shadcn-svelte add` berikutnya;
dan kalau generator suatu hari memuntah dash atau banner, gerbang menggagalkan commit untuk teks yang tidak ditulis
tangan. Direktori ini juga tidak murni generated: `constants.ts:3-5` ditulis tangan dan mencatat divergensi yang
disengaja. Butuh keputusan status, bukan edit.

### F9 (LOW) Dua suppression tidak menyebut constraint

`utils.ts:21,23`, satu-satunya `any` di panel (2 titik), aturan `no-explicit-any` memang aktif di
`eslint.config.js:71` jadi disable-nya menanggung beban, dan alasannya tersedia (§2.5). Sembilan directive lain di
`src` sudah menulis alasannya.

### F10 (LOW) Sembilan baris `//` kosong di `lib/primitives/*/index.ts`

Kena tes empty label §2.2, tapi ia memisahkan grup import supaya formatter menahan nama type-only bersama nama value.
Menghapusnya mengubah bentuk file. Bergabung ke keputusan F8.

### F11 (LOW) Satu alasan ditempel di empat route, dan salinnya sudah bergeser

`api-docs:69`, `changelog:119`, `playground:148`, `skills:95`: kalimat sama, satu noun ditukar; salinan keempat sudah
berbeda ("claim about the **documents**"). Kelas §3 doc line diwariskan antar keluarga.

### F12 (LOW) Angka turunan yang benar hari ini dan tidak dipegang apa pun

`usage-topology-geometry.ts:37-38` menjabarkan `NODE_MAX_WIDTH = 130` sebagai `padding 8 + dot 8 + gap 8 + label 96 +
border 2`. Diverifikasi ke markup: 16+8+8+96+2 = 130, **akurat hari ini**. Risikonya lima dari angka itu hidup di file
lain dari konstantanya. Pola penutupnya sudah ada di repo ini: `constants.ts:3-5` menyebut test yang mengikatnya.

### F13 (LOW) `DEPRECATED` kapital di dua komentar

`token-saver.ts:4`, `TokenSaverForm.svelte:10`.

## Yang ditemukan bersih, disebut karena ini bukan hasil default

Dash, separator, banner, end marker, emoji, artefak ` , `, TODO, `@ts-ignore`, `@ts-expect-error`, narasi langkah,
empty logic label: semuanya 0 terukur di `src` + `tests`. `: any` hanya 2 titik, keduanya `utils.ts`. 187 blok header
lewat delapan baris dan mayoritas membayarnya: `CopyButton.svelte:11-18` (tiga constraint platform non-obvious),
`UsageTopologyDrawing.svelte:18-31` (model unit `cqw`/`--u` dan kenapa metric harus dideklarasikan di elemen yang
memakainya), `quota.ts:118-136` (enam constraint grouping), `navigation.ts:1-13` (R-24 sebagai jaminan struktural,
bukan aspirasi), dan `constants.ts:3-5` sebagai komentar terbaik di panel.

Tiga klaim lain ikut diperiksa dan **benar**: `model-picker-data.ts:10` tentang cap `per_page` 100, `utils.ts` tentang
asal registry helper-nya, `server/playground-relay.ts` tentang gateway yang tidak meng-echo credential. Panel ini
sebagian besar benar. F1-F3 adalah yang berhenti benar.

## Rencana verifikasi

1. `bun run format:check` (atau `prettier --check src tests`), karena edit komentar bisa menyentuh panjang baris.
2. `bun run lint` untuk memastikan tidak ada directive yang hilang atau berubah makna.
3. `bunx svelte-check` / `bun run check`, type check penuh: komentar tidak boleh mengubah tipe, dan ini jaring kalau
   salah hapus.
4. `bun run test` (vitest). Tidak ada perilaku yang boleh berubah, jadi suite harus hijau persis seperti sebelumnya.
5. `bun run build`, karena panel ini produknya.
6. `scrypts/gates/antislop.sh` tree-wide, dan sekali dengan `GATES_BASE_REF=main` atas diff pass ini.
7. Bukti negatif untuk scope guardrail: `git diff --stat` harus tidak menampilkan satu pun perubahan pada baris yang
   bisa dieksekusi; setiap hunk hanya baris komentar.
8. Setelah F1-F2 selesai: hitungan ulang pointer. Target yang dinyatakan di audit 007 (82 occurrence, 59 distinct)
   harus menjadi 0 rentang baris; nama file yang ditunjuk boleh tinggal.

## Hasil

Status: **F1, F2, F3 selesai.** MEDIUM (F4-F8) dan LOW (F9-F13) tidak disentuh, menunggu keputusan.

### F1 selesai: 16 pointer baris ke `app-serv` diganti nama deklarasi

Nol `.go:NN` yang tersisa di `src` dan `tests`. Setiap titik sekarang menyebut simbol yang bisa di-grep dan
bertahan terhadap suntingan apa pun di file Go itu:

| Fakta | Sekarang menunjuk |
| --- | --- |
| bentuk `{label?, value, priority?}` | `internal/schema/endpoint.go`, `EndpointKeyInput` |
| decoder menolak field_unknown | `internal/schema/validator.go`, `jsonDecoder` |
| satu elemen per account di bulk | `BulkEndpointInput` |
| `apikey` dipetakan ke `api_key` | `internal/schema/endpoint.go`, `ParseAuthType` |
| cap satu batch account | `internal/service/endpoint_bulk.go`, `maxBatchRows` |
| `api_key` butuh satu kunci | `service/endpoint_create.go`, `buildEndpoint` |
| `display_name` tidak boleh kosong | `schema/model.go`, `CreateCustomModelRequest.DisplayName` |
| kind mana yang melayani chat | `registry/types.go`, `IsChat` |
| node dapat entri synthesized | `registry/custom_node.go`, `Index.Synthesize` |
| label wajib di body bulk | `BulkEndpointInput.Label` |

Sepuluh nama simbol itu diverifikasi ada di pohon `app-serv` setelah edit, bukan setelah audit: masing-masing
mencocokkan definisinya sendiri, tidak ada yang hanya cocok di file test.

Koreksi atas audit 007, dan ini penting untuk kepercayaan angkanya: audit menandai `:14` dan `:174` sebagai
"benar". `:14` memang memuat deklarasi yang tepat, tapi `:174` (`endpoint.go:130-136`) mendarat di
`BulkAddKeysRequest` sementara kalimatnya mendeskripsikan `BulkEndpointInput` (126-130): range-nya kelebihan satu
deklarasi dan kurang satu. Rate yang sebenarnya untuk lima titik yang diuji isi adalah 3 salah + 1 longgar + 1
tepat, bukan 3 salah + 2 tepat. Kesimpulan kelasnya tidak berubah; angka audit-nya yang perlu diperketat.

### F2 selesai: 66 koordinat ke reference fork dibuang, faktanya ditahan

Nol `file.ext:NN` tersisa di seluruh `src` + `tests`, termasuk empat bentuk telanjang yang tidak tertangkap
pola pertama (`(`:161`, `:190-194`)`) dan dua bentuk tanpa backtick (`page.js:419-436`). Yang dikerjakan per
titik:

- Nomor baris dibuang, nama file ditahan, supaya pembaca masih tahu bagian fork mana yang dimaksud.
- Angka yang merupakan fakta tetap tinggal dan justru itu inti kalimatnya: `nodeH = 30`, offset animasi `-36`
  per siklus, `0.75s / 0.45s / 0.7s`, radius glow 16px seperempat alpha, enam orb lima spark, `1000.0K` untuk
  999.999, dan `16+8+8+96+2 = 130` tetap ditulis.
- Tiga fork yang punya simbol bernama dipertahankan sebagai simbol, persis seperti aturan di F1:
  `capFilter`, `resolveThinkingSuffix`, `providerThinkingLevels`, `saveThinkingConfig`. Ini yang membuktikan
  aturannya bukan "buang referensi", tapi "referensi ke nama, bukan ke koordinat".
- Empat rentang telanjang yang menggantung (merujuk file yang disebut kalimat sebelumnya) dihapus tanpa
  meninggalkan tanda baca yatim; kalimat tetap dibaca utuh.
- Identitas fork tidak hilang: `AddProviderKeysDialog.svelte:5` tetap menyebut `9router origin/master` dan
  `usage-beam.ts` tetap menyebut commit `a8c9d380`. Commit hash bertahan; itu penunjuk yang sah, dan
  SPEC-UI §15 memegang identitas repo.

Yang sengaja tidak disentuh selama F2, supaya pass ini tetap bisa dinilai scoped: 62 + 76 sitasi `draft NNN`
(F5), 515 pointer SPEC-UI/SPEC-API/PORT (memang legal), dan tanggal attribution (F6). Ketiga angka itu
dihitung ulang setelah edit dan **tidak berubah** dari angka audit, yang berarti sweep ini hanya menyentuh
koordinat dan tidak ikut menggerogoti kelas lain.

### F3 selesai: tiga komentar sekarang menyebut apa yang kodenya lakukan

- `token-saver.ts`: klaim "URL is optional" dibuang. Yang ditulis sekarang adalah apa yang bisa dibuktikan dari
  kontrak: §7.9 melayani ketiga key apa pun nilai `enabled`, `""` adalah nilai tersimpan (bukan key yang
  hilang), jadi `z.string()` adalah read yang jujur dan skema write yang memeriksa bentuk.
  **Open question untuk owner, dan pass ini tidak menjawabnya:** apakah gateway pernah merespons tanpa key
  `url` sama sekali. Kalau ya, yang salah bukan komentarnya saja tapi skemanya, dan itu perubahan perilaku,
  bukan perubahan komentar, jadi ia keluar dari scope antislop-code.
- `endpoint-write.ts`: cap yang benar dinamai (`MAX_KEYS_PER_ENDPOINT`, dan kata "inclusive" ditambahkan karena
  100 lolos), dan klaim "a hundred rows is refused" dibuang karena memang salah. `MAX_BULK_CONNECTIONS` bukan
  cap di titik ini dan kalimatnya sekarang mengatakan begitu.
- `+error.svelte`: pembeda yang salah dibetulkan. Yang khusus 404 adalah elemen requested path; 403 dan
  fallback juga punya copy lengkap, dan aksi "Back to endpoint keys" dirender untuk semua status. Klausa yang
  masih benar ("the only error the panel raises on its own is a route that does not exist") dipertahankan.

### Verifikasi

| Check | Hasil |
| --- | --- |
| `prettier --check .` | PASS, "All matched files use Prettier code style!" |
| `eslint .` | PASS, nol output |
| `svelte-check --tsgo` | PASS, 0 errors 0 warnings |
| `bun run build` | PASS, 5098 modules transformed, "built in 1m 19s", tanpa error atau warning |
| `scrypts/gates/antislop.sh` | PASS seluruhnya |
| `bun run test` (vitest) | **PASS dua kali.** Run pertama, atas 49 dari 50 file: 182 file, 2981 test, 0 gagal, 1431s. Run kedua, atas pohon final setelah satu edit komentar tersisa di `tests/support/model-stub.ts`: exit 0, 1296s, tanpa kegagalan. Angka 2981 berasal dari run pertama; run kedua hanya mencetak ekornya karena output disaring `tail`, jadi yang dijamin untuk pohon final adalah exit 0, bukan angka itu. Beda di antara keduanya satu baris komentar, dan bukti byte-per-byte di bawah menunjukkan tidak ada konten non-komentar yang berubah |
| Scope guardrail | **Terbukti, bukan dinyatakan.** Semua baris non-komentar di 50 file yang berubah dibandingkan byte-per-byte antara HEAD dan working tree: 0 file berbeda. Nol baris executable berubah |

Diff: 50 file, 127 insertions / 123 deletions, semuanya di dalam konstruk komentar.

Bukti pendukung F4 dari pass ini sendiri: gerbang dipanggil atas diff yang mengubah 50 file frontend dan
mencetak "PASS scratch-work citations (changed Go files)". Tidak satu pun dari 50 file itu dibaca untuk
sitasi, suppression, atau panjang, persis seperti yang dinyatakan F4. Sebuah pass komentar pada `.ts` dan
`.svelte` lolos gerbang tanpa mengatakan apa pun tentang komentar `.ts` dan `.svelte`.

