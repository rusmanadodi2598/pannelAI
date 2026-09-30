# 039-USAGE-CACHE-WRITE-ALWAYS-ZERO.md: kenapa tile "cache write" di dashboard tidak pernah bergerak, dan mana yang cacat versus mana yang memang nol

Register temuan `app-serv` dari permintaan owner: "cek cost (Perhitungan) cache write, karena di
(app-ui) dashboard khusus cache write masih dan selalu 0 nilai nya." Satu jawaban berupa patch (§5),
sisanya berupa pengukuran live yang menyatakan bahwa nol itu jujur.

|                      |                                                                                                                                                                                                                                                                       |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**           | **CLOSED 2026-09-30.** C1 **perilaku benar, bukan bug** (§3–§4). C2 patch: `translate_usage_read.go` membaca ejaan cache yang benar di OpenAI-wire (§5). C3 test: detail `CacheCreationTokens` pada jalur Claude belum ditegakkan (§5). Sisa: §6                                                           |
| **Mechanism**        | AFTER (diukur live pada deployment ini, lalu patch pada pembaca usage yang jadi corong enam titik panggil)                                                                                                                                                             |
| **Scope**            | `usage_records` 14 hari terakhir; 4 panggilan provider; `GET /api/v1/models`; `app-ui` tile Cache read/write; `internal/registry/pricing.json`                                                                                                                                                                                            |
| **Permintaan owner** | "Oke, next cek cost (Perhitungan) cache write, karena di (app-ui) dashboard khusus cache write masih dan selalu 0 nilai nya." (2026-09-30). Atas pertanyaan lanjutan, owner memilih: **baca nama yang benar saja** — `prompt_cache_miss_tokens` tidak dijadikan write (§6)                                                                    |
| **Reference**        | `docs/DRAFT/038-CODEBUDDY-OPENAI-WIRE-PARITY.md` (satu ronde pengukuran pada provider yang sama, termasuk blok usage codebuddy); `docs/DRAFT/034-DATAPLANE-STREAM-DUPLICATE-FRAMES.md` §12 (akuntansi stream); register `docs/DRAFT/021` §22.1                                                                                                          |
| **Kaitan**           | `internal/dataplane/translate_usage_read.go`, `internal/service/chat_record.go:100-103`, `internal/service/chat_record_cost.go:55-68`, `internal/registry/pricing_cost.go:88-97`, `internal/registry/pricing.json`, `internal/domain/usage.go:207`; `app-ui/src/lib/components/UsageTotalsTiles.svelte:22`, `app-ui/src/lib/schemas/usage.ts:103` |
| **AGENTS.md §1.9**   | `SYSTEM_MAP.md` **N/A**: tidak ada batas domain, struktur data lintas layanan, atau topologi antrean yang berubah. Yang berubah hanyalah dari mana satu pembaca field mengambil angka yang sama. Kontrak `docs/CONTRACT/001` tidak disentuh — `tokens_cache_write` sudah didefinisikan di sana (§3716 area `UsageTotals`)                    |
| **Tanggal**          | 2026-09-30                                                                                                                                                                                                                                                              |

---

## 0. Label temuan

| Label | Pernyataan                                                                       | Jawaban                                   |
| ----- | -------------------------------------------------------------------------------- | ---------------------------------------- |
| C1    | Tile cache write 0 — apakah rumus cost/accounting-nya rusak?                      | **TIDAK.** Perhitungan benar ( §3, §4)   |
| C2    | Pembaca usage OpenAI-wire tidak mengenal ejaan cache write yang dipakai vendor     | **CACAT. Ditambal** (§5)                 |
| C3    | Detail cache creation pada jalur Claude tidak ditegakkan test                     | **CACAT test. Ditambal** (§5)            |

## 1. Rantai yang diperiksa

`usage.usage` upstream → `openAIUsageFromObject` / `ClaudeUsageToOpenAI` → `schema.Usage.PromptTokensDetails`
→ `chat_record.go:100-103` (`TokensCacheRead`/`TokensCacheWrite`) → kolom `usage_records` →
`GET /api/v1/usage/summary` → `schemas/usage.ts:103` (`tokens_cache_write`, wajib, tanpa `.default()`) →
`UsageTotalsTiles.svelte:22`.

Fakta penting dari sisi `app-ui`: karena field-nya **wajib** dan tidak punya default, key yang hilang akan
menjatuhkan `safeParse` dan menampilkannya sebagai error drift — bukan sebagai 0. Jadi "selalu 0" pasti
berarti backend memang mengirim 0, dan penelusuran cukup satu arah.

## 2. Bukti bahwa rantai hidup

`usage_records` 14 hari, dipecah per provider:

| provider | panggilan | cache read | cache write |
| --- | --- | --- | --- |
| `opencode` | 933 | **37.513.030** | 0 |
| `openai-compatible-0386BKG9Q4DYZPYC01VJH4C51G` | 68 | 256 | 0 |
| `codebuddy-intl` | 34 | 128 | 0 |
| `qoder` | 53 | 0 | 0 |

Write nol di semua baris; read **tidak** nol. Read dan write ditulis dari struktur yang sama
(`PromptTokensDetails`), jadi plumbing-nya terbukti jalan dan yang kosong memang hanya angka creation.

## 3. Pengukuran live: vendor di sini tidak menagih penulisan cache

System prompt 7.015 karakter, dipanggil beruntun ke `codebuddy-intl/deepseek-v4.1-flash`:

* **Panggilan cold** — `prompt_cache_miss_tokens: 1501`, `prompt_cache_hit_tokens: 0`,
  `prompt_tokens_details.cached_tokens: 0`. Counter penulisan yang tersedia semuanya nol:
  `cache_creation_input_tokens: 0` **dan** `prompt_cache_write_tokens: 0`.
  Tercatat: in 1501 / read 0 / **write 0** / cost `$0.00021042`.
* **Panggilan warm** — `prompt_cache_hit_tokens: 1280`, `prompt_cache_miss_tokens: 221`,
  `prompt_tokens_details.cached_tokens: 1280`. Tercatat: in 1501 / **read 1280** / write 0 /
  cost `$0.00003480` (6× lebih murah — rate `cached` jelas terpakai).

Provider `opencode` (yang punya 37,5 juta read) hanya pernah mengirim `cached_tokens` dan
`cache_read_input_tokens`; panggilan cold-nya tidak mengirim blok details sama sekali.

Kesimpulan C1: pada wire OpenAI-style, penulisan cache tidak dipisahkan sebagai kuantitas tersendiri —
token yang "ditulis" sudah ada di dalam `prompt_tokens` dan ditagih dengan rate input biasa. Angka 0 di
tile itu jujur, bukan hilang. Satu-satunya bentuk upstream yang benar-benar menagih `cache_creation`
adalah Anthropic, dan tidak ada provider `format: claude` yang punya endpoint di deployment ini
(`upstream_endpoints` hanya berisi `qoder`, `codebuddy-intl`, dan satu node custom).

## 4. Rumus dan harga juga benar

`pricing_cost.go:88-97` memotong `cacheCreation` dari non-cached lalu menerapkan
`term(cacheCreation, fallbackRate(rates.CacheCreation, rates.Input))`. `pricing.json` memuat **271 baris
dengan harga `cache_creation`**, dan angka-angkanya adalah premium tulis Anthropic yang semestinya
(Sonnet 3,75 vs input 3; Opus 6,25 vs 5; Haiku 1,25 vs 1 — persis 1,25×). Arah Claude→OpenAI
(`translate_openai_claude.go:129-141`) juga sudah memfolding `cache_creation_input_tokens` ke
prompt **dan** memisahkannya ke `PromptTokensDetails`. Jadi begitu sebuah upstream mengirim angka, baik
token maupun biayanya sudah punya rumah.

## 5. Yang ditambal

**C2 — ejaan.** `openAIUsageFromObject` hanya membaca `prompt_tokens_details.cache_creation_tokens`.
Ejaan yang benar-benar dipakai vendor pada wire itu ada di **top level**: `cache_creation_input_tokens`
(yang codebuddy kirim, hari ini bernilai 0) dan `prompt_cache_write_tokens`. Artinya: begitu salah satu
vendor menaruh angka nyata di sana, panel dan cost tetap baca 0 — dan biaya write kehilangan premiumnya.

`openAICacheWriteCount` / `openAICacheReadCount` sekarang mencoba daftar ejaan dengan **urutan
prioritas**, bukan penjumlahan: dua cara menyatakan satu angka yang sama, dan menjumlahkannya akan
harga-kan token yang sama dua kali. Home yang didokumentasikan OpenAI menang atas alias top level.
Blok details juga tak lagi menjadi gerbang — vendor yang hanya bicara di top level tetap terbaca.

**C3 — test.** Yang ditegakkan sebelum ini hanya "prompt total ikut cache creation". Tidak ada yang
menegakkan member `PromptTokensDetails.CacheCreationTokens` — padahal itulah yang dibaca baris usage.
Ditambah, bersama empat kasus ejaan dan satu kasus anti-dobel (semua ejaan berisi 1280 → hasil tetap
1280, bukan 3840).

## 6. Yang sengaja tidak dilakukan

`prompt_cache_miss_tokens` adalah kedekatan DeepSeek-style untuk "baru saja ditulis". Memakainya sebagai
cache write akan membuat tile langsung non-nol, tapi dua hal melarangnya: miss **sudah** termasuk di
dalam `prompt_tokens`, dan `domain/usage.go:207` menjumlah `in + out + read + write` untuk total — jadi
token akan dobel; dan vendor itu tidak pernah menagih write, sehingga cost akan memuat premium yang
tidak pernah terjadi. Owner memilih opsi "baca nama yang benar saja", dan register ini mencatat alasannya
supaya keputusan itu tidak perlu dicari ulang.

Kondisi yang membuat tile ini akhirnya bergerak di deployment ini: pasang satu provider `format: claude`
dengan kredensial. Jalur tulisnya (pembaca → detail → kolom → harga 1,25×) sudah lengkap dan sudah dites.

## 7. Gate

`go build`, `go vet ./...`, `staticcheck ./internal/...`, `go test -race ./internal/...` bersih;
`gofmt -l` kosong. Test baru: 5 fungsi (1 di antaranya bertabel 4 kasus) pada
`translate_usage_read_test.go`, berkas itu sebelumnya belum punya test sama sekali.
`translate_usage_read.go` 116 baris, tes 151 baris — dua-duanya di bawah AGENTS.md §1.1.
Verifikasi ulang live setelah patch: tiga panggilan warm tetap mencatat read 1280 / write 0 /
cost `$0.00003480` — tidak ada regresi pada sisi read.
