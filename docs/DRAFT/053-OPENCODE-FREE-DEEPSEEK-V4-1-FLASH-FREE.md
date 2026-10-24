# 053-OPENCODE-FREE-DEEPSEEK-V4-1-FLASH-FREE.md: katalog free tier tertinggal satu generasi, dan `deepseek-v4.1-flash-free` sudah hidup di upstream

Permintaan owner: "Cek Repo REFERENCE lain (`mhiqrambg/9router-mibp-version`), cek hanya pada Provider
OpenCode Free, cek bagian Upstream untuk models deepseek 4.1 flash free, lalu buatkan DRAFT."

| | |
|---|---|
| **Status** | **CLOSED 2026-10-15.** Keputusan owner: D3 jalan (b) `union-alpha` keluar dari katalog, luas C1+C2+C3. Yang mendarat C1-C6; `go test -race` hijau di registry/dataplane/provider, suite penuh + `go vet` bersih; belum di-push |
| **Mechanism** | BEFORE (draft ini) → DURING saat patch |
| **Scope** | `app-serv/internal/registry/registry.yaml` (blok `opencode.models`), `app-serv/internal/registry/opencode_free_models_test.go`, `app-serv/internal/dataplane/resolve_opencode_test.go` |
| **Permintaan owner** | audit upstream lane OpenCode Free untuk keluarga `deepseek-v4.1-flash`, lalu draft patch |
| **Reference** | `github.com/mhiqrambg/9router-mibp-version` (clone `--depth 1`, 2026-10-15): `open-sse/providers/registry/opencode.js`, `open-sse/executors/opencode.js`, `open-sse/utils/reasoningContentInjector.js`, `open-sse/utils/toolDeduper.js`, `src/app/api/providers/suggested-models/filters.js` |
| **Kaitan** | draft 029 (tabel paritas provider OpenCode), draft 051 (id sesi + gerbang identitas v2), draft 052 (klasifikasi kegagalan & parking), SPEC-API §7.6 (katalog) |
| **Tanggal** | 2026-10-15 |

---

## 1. Ringkasan

`deepseek-v4.1-flash-free` **sudah menjawab hari ini** lewat pipeline kita, tanpa satu pun perubahan kode.
Yang tidak benar adalah **katalog**: upstream free tier menampilkan 11 id, `registry.yaml` kita hanya
mendeklarasikan 4, dan salah satu yang kita deklarasikan (`union-alpha`) sudah dipensiunkan upstream.

Daftar free tier yang live, `GET https://opencode.ai/inference/v1/models` (2026-10-15, 86 id total, 11 di
antaranya free):

| id live | ada di `registry.yaml` kita | ada di katalog statis reference |
|---|---|---|
| `big-pickle` | - | ya |
| `cat-jam-free` | - | ya |
| `mimo-v2.6-flash-free` | - | ya |
| `muse-spark-1.2-contributor-free` | ya | ya |
| `muse-spark-1.3-contributor-free` | ya | ya |
| `nemotron-3.5-lightning-free` | - | ya |
| `jev-1.13-free` (systemone) | ya | ya |
| `step-5-preview-free` | - | ya |
| `longcat-2.5-preview-free` | - | ya |
| **`deepseek-v4.1-flash-free`** | - | **tidak** |
| `space-bunny-free` | - | ya |

Dua arah sekaligus: kita kehilangan 8 id, dan reference kehilangan satu yang baru terbit dua hari
setelah snapshot-nya (`registry/opencode.js:28` menulis "synced 2026-10-13"). Reference juga masih
mencantumkan `ling-3.1-flash-free` yang hari ini **tidak** ada di feed live.

## 2. Bukti terukur

Semua probe lewat pipeline nyata (`Engine.Relay`, harness `engine_opencode_free_live_test.go`,
`newOpenCodeFreeEngine(t, "https://opencode.ai", …)`), host yang sama, tidak ada kredensial. File scratch
dijalankan lalu dihapus; tabel ini hasil run 2026-10-15.

| # | Probe (model `opencode/deepseek-v4.1-flash-free`) | Hasil |
|---|---|---|
| P1 | teks biasa, `stream:true` | **200**, `"pong"` |
| P2 | dua tool dengan **nama sama** (`Bash` × 2) | **200**, menjawab sebagai tool call - tidak ada `400 Tool names must be unique` |
| P3 | riwayat 3 giliran, pesan assistant **tanpa** `reasoning_content` | **200**, `"6"` |
| P4 | PNG 4×4 merah murni, "warna apa ini?" | **200**, `"Red"` → model ini benar-benar bermata |
| P5 | `reasoning_effort:"high"` | **200**, jawaban benar |

| # | Model lain di lane free | Jawaban upstream | Kelas kita |
|---|---|---|---|
| P6 | `union-alpha` (kita deklarasikan) | **400** `Upstream request failed: Model is unavailable.` | `UPSTREAM_REJECTED` |
| P7 | `ling-3.1-flash-free` (reference deklarasikan) | **400** sama | `UPSTREAM_REJECTED` |
| P8 | `deepseek-v4-flash-free` (yang lama) | **400** sama | `UPSTREAM_REJECTED` |
| P9 | `deepseek-v4.1-flash` (id berbayar, dikirim ke lane free) | **401** `Missing API key.` | `UPSTREAM_ERROR` |

| # | Kontrol | Hasil |
|---|---|---|
| C-a | `curl` langsung ke `/inference/v1/chat/completions` dengan identitas v2 yang sah **tanpa** `tools` | **403** `FreeTierError` - konfirmasi bahwa deklarasi `read`/`shell` kanonik memang gerbangnya, bukan header saja |
| C-b | `GET /zen/v1/models` | **200**, daftar id yang sama - yang pindah ke `/inference/v1` adalah jalur *chat*, bukan daftar model |

## 3. Temuan

| # | Tingkat | Isi |
|---|---|---|
| **F1** | HIGH | Delapan id free yang hidup tidak ada di `opencode.models`. `passthrough_models: true` membuatnya tetap bisa dirutekan kalau klien menebak id-nya, tetapi tidak pernah muncul di `GET /api/v1/models`, di playground, atau di daftar combo. Efeknya persis seperti yang sudah ditulis `opencode_free_models_test.go:6` untuk generasi sebelumnya: *"answerable through passthrough yet absent from every models list"* |
| **F2** | MEDIUM | `union-alpha` kita deklarasikan sebagai model chat dan bermata; upstream menjawab 400 untuk setiap penelfon (P6). Katalog menjanjikan model yang tidak menjawab. Reference menahan id yang sama dengan alasan test (`registry/opencode.js:33-35`), bukan alasan produk |
| **F3** | LOW | Kode connector untuk keluarga ini sudah lengkap, dan **bukan** karena kebetulan: dedup nama tool sudah terjadi di `opencode_fingerprint.go:164` (`if seen[name] { continue }`) sebelum body berangkat, jadi aturan `toolDeduper.js` reference sudah tercakup dan P2 tidak bisa menghasilkan 400; visi dijawab pola `*deepseek-v4.*` → `true` (`capability_patterns.go:104`) dan P4 mengonfirmasi; injeksi `reasoning_content` reference **tidak** dibutuhkan lane ini (P3) |
| **F4** | LOW | 401 pada lane no-auth (P9) **tidak** mem-park apa pun: endpoint free adalah virtual (`selection_virtual.go`) dan `RecordFailure` keluar lebih awal untuk endpoint virtual (`selection_health.go:122`), jadi tidak ada baris key yang bisa disiram. Diperiksa, tidak ada bug, tidak ada perubahan |
| **F5** | MEDIUM | Akar drift: tidak ada `modelsFetcher` untuk lane ini di app-serv. Reference punya (`opencode.js:54`) dan daftar statis mereka tetap salah dalam dua hari. Salin-manual akan mengulang F1 setiap minggu |

## 4. Keputusan desain

**D1 - tambah delapan id yang live, bukan salin katalog reference.** Dasar masuknya satu hal: id itu ada
di feed live hari ini. Karena itu `ling-3.1-flash-free` (P7) dan `deepseek-v4-flash-free` (P8) **tidak**
ikut, walaupun reference dan riwayat masing-masing menyebutnya. `target_format` untuk kedelapan id dibiarkan
kosong (= OpenAI, jalur `chat/completions`) karena itulah yang terukur menjawab; pasangan muse tetap
`openai-responses`, `jev-1.13-free` tetap `kind: systemone`. Tidak ada `min_output_tokens` baru: P1-P5
menjawab dengan ceiling default, dan angka yang tidak diukur tidak masuk registry.

**D2 - tidak menyentuh connector.** F3 adalah hasilnya, dan itu keputusan, bukan kelalaian: tiga perilaku
yang reference pasang untuk `deepseek` (dedup, injeksi reasoning, floor) sudah ada di tempat lain atau
terukur tidak diperlukan. Menambahkannya "biar sama dengan reference" adalah kode yang tidak dipakai.

**D3 - `union-alpha`: PUTUSAN OWNER, jalan (b), keluar dari katalog.** Dua jalan yang disajikan:
- *(a) Tetap dideklarasikan, diberi catatan.* Alasannya konkret: ia satu-satunya jangkar wire Claude
  terhadap dokumen registry yang sebenarnya - `resolve_opencode_test.go:48` membaca `registry.Load()` dan
  mem-pin `union-alpha → TargetClaude`, dan `opencode_test.go:93` mem-pin URL `/messages`. Menghapus baris
  itu tanpa pengganti memutus pin itu, dan `/inference/v1/messages` tidak lagi punya satu pun model free
  yang hidup (tidak ada di feed live hari ini).
- *(b) Keluarkan dari katalog, pindahkan pin ke index sintetis.* Katalog berhenti berbohong; tetapi gerbang
  wire Claude berpindah dari "dokumen yang disajikan router" ke "index buatan test".

  **Yang dipilih: (b).** Konsekuensi yang kini benar: katalog tidak lagi mengiklankan model yang upstream
  tolak, `resolve_opencode_test.go` mem-pin wire per-model pada id chat yang hidup
  (`deepseek-v4.1-flash-free → TargetOpenAI`), dan aturan Messages wire tetap dijaga
  `provider/opencode_test.go:93` yang membangun `registry.Model{ID: "union-alpha", TargetFormat: "claude"}`
  sendiri. Yang hilang: tidak ada satu pun model free hidup yang menempuh `/inference/v1/messages` -
  jalur itu hari ini hanya teruji, belum terukur.

**D4 - `modelsFetcher` di luar patch ini.** Menyamakan daftar dengan realita secara permanen berarti
menambah jalur fetch-live + merge + cache + kegagalan jaringan untuk satu lane; itu fitur, dan punya
draft-nya sendiri (nomor 054, belum ditulis). Patch ini menutup gap yang terukur hari ini; F5 tetap tercatat
supaya tidak dianggap selesai.

## 5. Perubahan yang diusulkan

| # | File | Isi |
|---|---|---|
| C1 | `registry.yaml`, blok `opencode.models` (972-1001) | Delapan id live masuk: `big-pickle`, `cat-jam-free`, `deepseek-v4.1-flash-free`, `longcat-2.5-preview-free`, `mimo-v2.6-flash-free`, `nemotron-3.5-lightning-free`, `space-bunny-free`, `step-5-preview-free`; `union-alpha` keluar. Tinggal 11 = persis jumlah free di feed live. `target_format` kedelapan id kosong (= OpenAI, `chat/completions`) karena itulah yang terukur menjawab |
| C2 | `opencode_free_models_test.go` | `cases` 4 → 11 pada assertion yang sama (id, name, target_format, `IsChat()`, `min_output_tokens`). `len(provider.Models) != len(cases)` membuat menambah baris yaml tanpa test ikut berubah **gagal**, bukan lolos diam-diam |
| C3 | `resolve_opencode_test.go` | Pin wire per-model berpindah dari `union-alpha → TargetClaude` ke `deepseek-v4.1-flash-free → TargetOpenAI` (konstanta chat di repo ini bernama `TargetOpenAI`, lihat `target.go:22`); `ModelsAreListed` menambah `deepseek-v4.1-flash-free` dan `space-bunny-free` |
| C4 | `registry.yaml` komentar katalog | Tanggal pengukuran + aturan mainnya: satu id keluar karena upstream menjawab 400, bukan karena ada test yang mau ia tinggal |
| C5 | `testdata/pricing_corpus.json` | `TestPricing_MatchesTheReferenceCorpus` menuntut satu pasangan korpus untuk setiap model yang registry deklarasikan. Diisi ulang dengan `node tools/pricing-gen.mjs <checkout 39e36d3d>` **pada revisi yang sudah tertanam di `pricing.json`**, bukan HEAD reference: `pricing.json` tidak berubah sama sekali (0 baris), korpus bertambah tepat 8 pasangan opencode dan mengurangi `union-alpha` |
| C6 | `engine_opencode_free_acceptance_test.go` | Premis `TestOpenCodeFree_UnknownFreeIdStillPassesThrough` ternyata sudah salah sejak awal: ia memakai `space-bunny-free`/`mimo-v2.6-flash-free` sebagai contoh "tidak dideklarasikan" dengan alasan reference hanya menampilkan mereka lewat fetcher live, padahal `registry/opencode.js:40,44` mendaftarkan keduanya secara statis. Id pin dipindah ke `union-alpha` dan `ling-3.1-flash-free`, yang memang di luar katalog hari ini |

Tidak ada migrasi, tidak ada perubahan API, tidak ada perubahan `SYSTEM_MAP.md` (topologi dan batas domain
tidak bergerak; yang berubah hanya isi satu tabel katalog).

## 6. Test (hasil nyata)

- `go test -race ./internal/dataplane/ ./internal/registry/ ./internal/provider/` → ketiganya `ok`.
- `go test ./...` → hijau; `go vet ./...` → bersih. Tidak ada `t.Skip`, tidak ada filter `-run` yang ditinggalkan.
- `scrypts/gates/go-headers.sh` → PASS (1376 file). `scrypts/gates/antislop.sh` → PASS; dua hal yang
  needing review ditinggalkan sadar: satu sitasi `draft 053` di badan test Go (gate-nya SKIP untuk Go, dan
  konvensi tree sudah memakainya, mis. `selection_health.go:93`), dan blok doc `opencode_free_models_test.go`
  yang dipangkas ke 11 baris.
- Yang gagal lalu dibetulkan dalam perjalanan: C6. Tanpa C6, `TestOpenCodeFree_UnknownFreeIdStillPassesThrough`
  mati justru karena ia benar-benar membaca registry.

Live (tidak dijalankan ulang setelah patch): probe P1-P9 di §2 sudah menjawab pertanyaan yang sama dengan
biaya kuota anonim upstream. Menjalankan `TestOpenCodeFreeLive` dengan `openCodeFreeModels` ditambah
`deepseek-v4.1-flash-free` tetap tersedia sebagai bukti M-end kalau owner ingin satu angka sesudah patch.

## 7. Yang tidak dikerjakan, dan mengapa

| | |
|---|---|
| Injeksi `reasoning_content` ala `reasoningContentInjector.js` | P3: upstream lane ini tidak menuntutnya. Aturan reference itu lahir untuk `api.deepseek.com` dan bentuk vendor lain |
| Dedup nama tool khusus-deepseek | Sudah terjadi untuk semua model di lane ini (`opencode_fingerprint.go:164`); P2 mengonfirmasi dari sisi upstream |
| Floor `min_output_tokens` untuk `deepseek-v4.1-flash-free` | Tidak diukur. Menyalin 512 milik muse-spark akan mengarang angka untuk model yang berbeda |
| `vision:false` untuk id baru | Pola `*deepseek-v4.*` sudah menjawab `true` dan P4 membuktikannya; tidak ada baris yang perlu ditulis |
| `modelsFetcher` live | D4 |
| Lane `opencode-zen` / `opencode-go` | Owner meminta lane free saja. Dicatat sekalian: registry kita menaruh zen di `/inference/v1/*` sedangkan reference menaruhnya di `/zen/v1/*` (`registry/opencode-zen.js:31-33`) - itu pertanyaan terpisah, belum diukur, tidak disentuh di sini |
