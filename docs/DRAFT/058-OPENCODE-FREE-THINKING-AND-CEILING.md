# 058-OPENCODE-FREE-THINKING-AND-CEILING.md: combo berhenti setelah thinking, dan yang menyalakan thinking adalah kita sendiri

Permintaan owner: "Combo `pi-agent` dipakai di pi coding CLI, isinya model OpenCode Free. Di beberapa case
sering terjadi response berhenti." Lalu: "Terpenting, engine kita sudah kuat dan bekerja dengan benar... tinggal
check bagian max content, token, dan lainnya. Pastikan BE sudah bisa mengaturnya dengan baik." Dan: "Combo harus
ikut ke setting provider di engine-nya masing-masing."

| | |
|---|---|
| **Status** | **MENDARAT 2026-10-16.** Tiga keputusan: setting provider-thinking yang memicu starve dikosongkan (config, live hari ini), floor output kini ditegakkan di wire chat juga dengan angka terukur per model, dan tool call yang terpotong di ceiling dilaporkan sebagai `length`, bukan `tool_calls`. Walk, commitment stream, dan failover order TIDAK diubah - itu perintah owner. `-race` hijau satu modul penuh; gate headers, antislop, go-lint (vet, gofmt, staticcheck, golangci-lint) PASS |
| **Mechanism** | BEFORE: `settings.reasoning.provider_thinking.opencode = high` -> upstream diminta berpikir di level tertinggi untuk semua modelnya; `max_tokens` klien di wire chat lolos tanpa floor; call yang terpotong dilaporkan dispatchable. AFTER: provider yang memutuskan thinking, floor per model berlaku di kedua wire, dan pemotongan disebut dengan namanya |
| **Scope** | `app-serv/internal/provider/opencode_ceiling.go` (baru), `opencode_body.go`, `internal/dataplane/translate_responses_answer.go`, `translate_responses_response.go`, `translate_stream_responses.go`, `internal/registry/registry.yaml`, test-nya, `docs/SPEC-API/001-SPEC-API.md` §7.15, `app-serv/.env` tidak berubah (yang berubah adalah baris settings di Postgres) |
| **Permintaan owner** | respons tidak berhenti lagi di combo free, dan request app-serv ke upstream memenuhi standar provider |
| **Kaitan** | draft 054 (sinyal ceiling di stream), draft 055 F1/F2 (yang ditutup di sini), draft 053 (katalog free + floor 512), draft 057 (kuota per alamat, jadi rotasi proxy bukan jalan keluar untuk starve), SPEC-API §7.14 (mode thinking), §7.15 (pipeline, ceiling, suffix), §7.7 (combo walk) |
| **Tanggal** | 2026-10-16 |

---

## 1. Ringkasan

Yang berhenti bukan streamnya dan bukan engine-nya. Yang berhenti adalah jawaban: model menghabiskan seluruh
ceiling untuk thinking, lalu menjawab tanpa isi, dan upstream menghitung itu sebagai jawaban 200.

Dua hal menyebabkan itu, dan keduanya milik kita:

1. **Kita yang meminta thinking tertinggi.** `GET /api/v1/settings` menjawab
   `reasoning.provider_thinking.opencode.mode = "high"`. Preseden §7.15 (dan `reasoning/applier.go:105-131`)
   adalah suffix -> intent klien -> **mode tersimpan per provider**. pi tidak mengirim apa pun, jadi mode
   tersimpan yang menang, dan `reasoning_effort: high` ditulis ke badan request untuk **setiap** model di
   provider itu, termasuk yang tidak pernah diminta berpikir.
2. **Floor output hanya ditegakkan di satu wire.** `openCodeOutputFloor` sudah ada sejak September dan sudah
   menaikkan ceiling klien ke angka yang model umumkan, tapi hanya dipanggil dari jalur Responses
   (`opencode_responses.go:46`). Anggota combo yang menjawab di wire chat (`deepseek-v4.1-flash-free`,
   `space-bunny-free`) tidak punya floor sama sekali, dan `max_tokens` klien lewat apa adanya.

Yang ketiga tidak menstarve sesi ini tapi punya akar yang sama, dan ikut dibereskan: satu mode tersimpan berlaku
ke semua model di provider tanpa ditanyakan ke daftar level yang masing-masing model terima (D4).

Ini bukan temuan spekulatif: mekanismenya terbaca di kode, angkanya terbaca di log upstream, dan perbaikannya
terukur sebelum-sesudah di lane yang sama.

## 2. Pengukuran

Combo `pi-agent` (round_robin, tiga anggota free), request bentuk coding CLI: chat, stream, `tools` dengan
`write_file`, dan `max_tokens` kecil seperti yang dikirim klien coding:

| kondisi | reasoning char | content | tool args | finish_reason |
|---|---|---|---|---|
| sebelum, `pi-agent` `max_tokens=2500` | 9585 | 0 | 0 | `length` |
| sesudah S0, call 1 | 0 (706 token thinking) | 65 | 5667 | `tool_calls` |
| sesudah S0, call 2 | 160 (35 token) | 0 | 6453 | `tool_calls` |

Setting yang sama di-clear, angka yang sama keluar: jawaban jadi ada, dan thinking turun dari 2499 token ke 706/35.
Sesuai testing owner: `pi` dengan thinking off = stream sehat.

Lantai per model, diukur sebagai ceiling terkecil yang masih menuntaskan satu `write_file` call (argumennya ter-parse
sebagai JSON):

| model | 256 | 512 | 1024 | 2048 | tanpa `max_tokens` | floor dinyatakan |
|---|---|---|---|---|---|---|
| `muse-spark-1.3-contributor-free` | args **0**, `tool_calls`, completion **512** | `tool_calls` dengan args **0** | `tool_calls` dengan args **0** | selesai (2776 char) | selesai (5173 char, 1731 token) | **2048** |
| `deepseek-v4.1-flash-free` | args 0, content 800, `length` | args 31 char, `length` | args 0, thinking 1024/1024, `length` | selesai (5520 char) | selesai (3940 char, 1508 token) | **2048** |
| `space-bunny-free` | args 0, reasoning 1050, `length` | args 0, `length` | selesai (2835 char) | selesai | content 7275 char, **tanpa call**, `stop` | **1024** |

Tiga hal dari tabel itu. Pertama, floor 512 yang dinyatakan September untuk muse-spark cukup untuk jawaban teks dan
tidak cukup untuk tool call, jadi angkanya dinaikkan dengan pengukuran baru, bukan diturunkan dari model lain.
Kedua, dan ini yang menutup draft 055 F1: muse-spark pada 512 dan 1024 mengirim **`finish_reason: tool_calls` dengan
`arguments` kosong**. Klien coding menerima call yang tidak bisa di-dispatch dan tidak ada sinyal ada yang hilang.
Itu bentuk paling buruk dari "response berhenti", karena tidak ada error yang bisa ditanggapinya. Ketiga, sel 256
muse-spark bukan angka klien: cell itu diukur pada binary sebelum D2, dan floor 512 yang sudah dinyatakan September
tetap jalan di jalur Responses, jadi completion-nya 512, bukan 256. Sel "tanpa ceiling" space-bunny adalah kelas
lain yang tidak bisa dibeli dengan angka: model menjawab 7275 karakter prosa dan `stop` tanpa memanggil tool sama
sekali, jadi floor berapapun tidak menjamin call; yang dijamin floor hanyalah bahwa kalau call itu ada, ia muat.

## 3. Yang mendarat

**D1. Setting dikosongkan, bukan kode yang ditambah.** `PATCH /api/v1/settings` dengan
`reasoning.provider_thinking: {}`. Setelah itu `resolve()` mengembalikan `nil` dan connector tidak menulis field
apa pun: lane free berpikir sebanyak yang upstream-nya sendiri putuskan. Untuk dua anggota combo yang starve,
memang tidak ada yang salah di mesinnya: `high` level sah untuk pola `*deepseek-v4*` (`capability_levels.go:60`)
dan muse-spark menerima `[none, minimal, low, medium, high, xhigh]`. Yang salah adalah nilainya. Kelas yang lebih
besar tinggal, dan itu D4.

**D2. Floor di kedua wire.** `provider/opencode_ceiling.go` (baru) memuat seluruh aritmatika ceiling: `renameOpenCodeCeiling`
dan `clampOpenCodeCeiling` yang sudah ada, ditambah `clampOpenCodeChatCeiling` yang dipasang di cabang non-Responses
`TransformRequest`. Preseden key mengikuti referensi lokal (`max_completion_tokens` lalu `max_tokens`,
`apps/9router/open-sse/executors/opencode.js:430-432`). Raise dibatasi: setengah `context_length` yang dinyatakan,
`max_output_tokens` model, dan 4096 bila window tidak dinyatakan. Guard gagal -> angka klien diteruskan, karena
jawaban kurus masih jawaban sementara 400 adalah permintaan yang mati. Registry sekarang menyatakan tiga floor di
atas, dan satu test end-to-end (`engine_opencode_free_floor_test.go`) membuktikan ceiling yang sampai ke upstream
adalah angka floor untuk kedua wire, dan tetap 60 untuk model tanpa floor.

**D3. Pemotongan disebut dengan namanya.** `responsesFinishReason` sekarang menerima dua fakta
(`hasCalls`, `openCall`) dan membaca ceiling lebih dulu: call yang belum tertutup di jawaban incomplete karena
`max_output_tokens` -> `length`; call yang sudah tertutup -> tetap `tool_calls` (carve-out yang draft 055 minta
diperiksa, sekarang di-pin). Stream mengambil statusnya dari lifecycle item (`output_item.added` tanpa
`output_item.done`), fold mengambilnya dari argumennya sendiri (absen, kosong, atau bukan JSON object), lewat satu
fungsi sehingga kedua arah tidak bisa berbeda tentang satu jawaban. Wire chat tidak punya masalah preseden: ia
meneruskan `finish_reason` upstream apa adanya.

**D4. Mode tersimpan ditanyakan ke daftar level model.** Yang D1 biarkan utuh adalah kelasnya, bukan case ini: satu mode per
provider berlaku ke semua model di lane itu, dan sebelum ini hanya `max`/`ultra` (`NormalizeOpenAILevel`) dan `none`
(`canDisable`) yang diperiksa terhadap daftar model. Buktinya masih di lane yang sama: `nemotron-3.5-lightning-free`
menerima `[none, low, medium, high]` dan `mimo-v2.6-flash-free` `[low, medium, high, xhigh]`, jadi mode tersimpan `minimal`
dulu tertulis apa adanya untuk keduanya, sebuah angka yang upstream tidak enumerasi. `reasoning.ReasoningLevelFor` sekarang
menanyakan daftar itu (`registry.ThinkingLevels`, sudah tersedia di `caps.levels`) sebelum kedua bentuk OpenAI menulis: turun
ke level terdekat yang diterima, naik kalau tidak ada yang di bawah, tidak pernah jatuh ke `none` karena mematikan thinking
adalah keputusan yang berbeda dari memikir lebih sedikit, dan model tanpa daftar tetap menerima angka yang pemanggil tulis. Format
lain tidak disentuh: `deepseek`, `kimi`, dan `zai` sudah memetakan level ke kosakatanya sendiri, `claude` dan `gemini` menulis
budget, dan `tokenrouter` mendokumentasikan pass-through sebagai aturan vendor-nya sendiri.

Satu penyimpangan dari rencana, sengaja: rencana menulis hanya jalur tersimpan yang divalidasi. Titik yang sama sudah
men-normalisasi `max`/`ultra` untuk semua sumber sejak awal, jadi validasi ini ikut berlaku untuk intent klien dan
suffix; memisahkannya berarti satu body bisa membawa level yang model tidak dokumentasikan tergantung siapa yang
menulisnya. Yang terukur di §5: pada lane free upstream tidak menolak level di luar daftar, ia menjawab apa adanya,
jadi nilai validasi ini adalah body yang sesuai kontrak vendor, bukan request yang diselamatkan dari 400.

## 4. Yang tidak jadi

**Perubahan semantik stream/walk.** Opsi yang semula dirancang (commit pada frame jawaban, bukan byte pertama,
sehingga combo boleh berpindah anggota di tengah stream) ditolak owner: engine sudah kuat dan berperilaku benar.
Yang benar memang begitu - dengan D1 dan D2 kasus starve tidak lagi meminta mekanisme itu, dan aturan
`FramesWritten` hari ini justru melindungi klien dari tagihan ganda. Kalau nanti masih ada sisa, ia dibuka lagi
sebagai keputusan tersendiri, bukan diselipkan.

**Cap `min_output_tokens` sebagai obat.** Tabel 2 membuktinya bukan obat: satu request menghabiskan 2499 dari 2500.
Floor membuat kelas "ceiling mungil" mustahil; kelas "thinking panjang" tetap ada dan sekarang dijawab dengan
`length` yang benar, bukan dengan angka yang lebih besar.

**Eksklusi call tak-dispatchable dari `response.output` terminal klien Responses.** Sengaja tidak dilakukan: klien
sudah menerima delta call-nya (tidak bisa ditarik), dan `response.incomplete` adalah instruksi untuk tidak
men-dispatch-nya. Ditunda sampai ada kasus yang menuntut, bukan karena lupa.

## 5. Yang masih terbuka

- Lane berkunci (`opencode-zen`, `opencode-go`) tidak menulis header identitas sama sekali, jadi 429 dan starve di
  lane itu punya cerita kuota yang sendiri dan tidak diukur di sini.
- `space-bunny-free` floor 1024 diukur dari satu bentuk prompt tool-bearing; model lain di lane yang sama
  (`cat-jam-free`, `big-pickle`, `nemotron-*`, `step-5-preview-free`, `longcat-*`) belum, dan tidak dinyatakan
  karena tidak diukur.
- `context_length` tidak dinyatakan untuk satupun model free: upstream tidak memublikasikannya di
  `GET /inference/v1/models` (86 id, hanya `id/object/created/owned_by`, diukur 2026-10-16), jadi raise berjalan di
  batas blind 4096 sampai ada angka dari dokumen provider.
- Perilaku klien pi sendiri ketika menerima `length` tanpa isi (retry? minta lanjut?) ada di sisi klien; gateway
  sekarang memberi sinyal yang benar, itu batasnya.
- D4 dibangun dari daftar level yang tabel kurasi umumkan, dan biaya versi sebelumnya sudah diukur: free tier
  TIDAK menolak level di luar daftar. `opencode/nemotron-3.5-lightning-free` (menerima `[none, low, medium, high]`)
  dipanggil tiga kali per level pada kedua binary, `minimal` dan `xhigh` yang tidak ada di daftarnya plus `high`
  sebagai kontrol: semuanya menjawab 200, tanpa satu pun 400. Yang berubah karena D4 adalah body yang sampai ke
  upstream sesuai kosakata model, itu klaimnya; bukan angka kegagalan, karena pada n=3 selisihnya tenggelam di
  noise lane ini.
- Model yang sama menjawab `tool_calls` dengan content kosong pada request tanpa tool, di kedua binary dan pada
  level yang daftarnya terima. Itu perilaku upstream, bukan terjemahan kita, dan tidak diubah di sini.

## 6. Verifikasi hidup

A/B dijalankan 2026-10-16 pada dua instance gateway yang sama DB, sama pool proxy, sama upstream: `:9090` proses
yang sudah hidup sejak 07:15 (tree sebelum D2/D3) dan `:9091` binary dari tree sekarang. Bentuk request sama persis
dengan yang dikirim pi coding CLI: wire chat, `stream: true`, satu tool `write_file`, prompt suruh menulis
`fib.py`. Yang dibandingkan hanya angka yang sampai ke upstream, jadi `completion_tokens` di kolom bawah adalah
bukti bukan tafsir.

| model | `max_tokens` klien | :9090 lama | :9091 baru |
| --- | --- | --- | --- |
| `muse-spark-1.3-contributor-free` | 512 | `completion 512`, args 0, `tool_calls` | `completion 1382`, args 3227 tertutup, `tool_calls` |
| `deepseek-v4.1-flash-free` | 512 | `completion 512`, content 1679, args 0, `length` | `completion 1176`, args 4131 tertutup, `tool_calls` |
| `space-bunny-free` | 256 | `completion 256`, reasoning 1042, content 0, args 0, `length` | `completion 1024`, args 1972 tertutup, `length` |

Tiga hal dari tabel itu. Pada binary lama `completion_tokens` berhenti **tepat** di angka klien (512/512/256), jadi
ceiling klien memang lolos apa adanya ke upstream; pada binary baru angkanya naik sendiri dan tidak pernah melewati
floor yang dinyatakan, dengan `space-bunny` mendarat persis di 1024. Tool call yang di lama tidak pernah keluar
(arguments kosong atau badan teks tanpa call) sekarang keluar tertutup pada kedua model floor 2048. Dan
`space-bunny` tetap `length`: floor 1024 cukup untuk menuntaskan argumennya tapi tidak cukup untuk sisa jawabannya,
persis pernyataan bahwa floor adalah mitigasi bukan obat.

Sisa kelas yang sama masih hidup dan tidak diselesaikan D2 maupun D3: `cat-jam-free` tidak punya floor, jadi ceiling
kecil dari klien tetap lolos.

Burst sepuluh request berturut-turut pada combo `pi-agent` (round_robin, jadi ketiganya bergilir), `max_tokens: 2500`,
dijalankan dua kali: sekali pada binary D2/D3 dan sekali pada binary setelah D4. Dua puluh call, dan tidak satu pun
berakhir tanpa sesuatu yang bisa ditampilkan: sembilan dan sepuluh call menghasilkan tool call yang argumennya
ter-parse, sisanya berhenti di `length` dengan teks penuh (7455 karakter) atau dengan call yang sudah tertutup plus
teks. Yang berhenti di 2500 menghabiskan ceiling sambil menulis prosa atau melanjutkan jawaban setelah call, bukan
membelanjakannya seluruhnya untuk thinking seperti tabel §2.

Satu koreksi metode: burst pertama membaca `arguments` dengan menggabungkan fragmen semua call dalam satu string,
jadi tiga call terbaca "rusak" padahal probe-nya yang menyambung dua slot. Versi kedua mem-parse per index dan
hasilnya nol call rusak. Angka yang dikutip di atas dari versi per-index; yang dari versi gabungan hanya dipakai
sebagai bukti bahwa floor menaikkan ceiling, karena itu yang tidak berubah oleh cara bacanya.

D4 diuji hidup dan hasilnya juga negatif untuk klaim kegagalan: `opencode/nemotron-3.5-lightning-free` dipanggil tiga
kali per level pada kedua binary, `minimal` dan `xhigh` yang tidak ada di daftar model itu plus `high` sebagai
kontrol, dan dua belas call semuanya menjawab 200 tanpa satu pun penolakan. Yang berubah adalah body: binary lama
menulis level apa yang pemanggil nama, binary baru menulis level yang model itu deklarasikan. Sisanya adalah noise
lane free, dan satu call 504 pada probe pertama tidak terulang ketika diukur ulang.

S3/D3 diuji hidup dan hasilnya **negatif untuk lane ini**: pada kedua binary, call yang terpotong oleh ceiling
menutup klien dengan `length` (stream `response.incomplete` maupun jawaban non-stream), karena lane opencode free
menjawab lewat stream dan jalur stream sudah memublikasikan sinyal ceiling sejak pengamatan 054. Perubahan preseden
hari ini menyatukan pembacaan ceiling antara stream dan fold ke satu fungsi, dan dibuktikan oleh pin unit, bukan
oleh A/B ini. Bentuk yang justru muncul di kolom lama, `tool_calls` dengan arguments kosong sementara
`completion_tokens` pas di ceiling, adalah upstream yang menjawab `completed` bukan `incomplete`: tidak ada preseden
apa pun yang bisa memperbaikinya, dan yang menghapusnya dari lane combo adalah floor.
