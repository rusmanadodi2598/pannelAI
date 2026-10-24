# 051-OPENCODE-FREE-403-IDENTITY-FIX.md: id sesi OpenCode harus membawa timestamp, bukan bytes acak

Permintaan owner: "Cek PR decolua/9router #4786, cek bagian patch Provider OpenCode, lalu kita
terapkan ke pannelAI (app-serv)." Yang mendarat adalah seluruh tiga item PR, karena satu di antaranya
(`x-opencode-request`) belum pernah ada di app-serv dan dua sisanya menyentuh gerbang yang sama.

| | |
|---|---|
| **Status** | **CLOSED 2026-10-14.** Tujuh perbaikan source + test mendarat; 403 free tier terukur hilang (M1 → M5); belum di-push |
| **Mechanism** | DURING & AFTER (antislop) |
| **Scope** | `app-serv/internal/provider` (`opencode*.go`), `app-serv/internal/dataplane` (`translate_claude*`, `translate_response*`, `translate_stream_claude*`), `internal/registry/registry.yaml`, `SYSTEM_MAP.md` |
| **Permintaan owner** | port patch provider OpenCode dari PR #4786 |
| **Reference** | `decolua/9router` PR #4786 (`letinhminh`, commit `31feb2c6`, state CLOSED, diff diambil lewat `gh pr diff`); checkout lokal `/home/rusmanadodi/apps/9router` = `a99cf572` (v0.5.95) yang **pra-patch**, jadi ia acuan bentuk encoding (`generateSessionId`/`generateRequestId`), bukan acuan hasil patch |
| **Kaitan** | draft 029 (tabel paritas provider OpenCode, §5-nya diamandemen di sini), draft 031 (lane free tier), SPEC-API §7.6 |
| **Tanggal** | 2026-10-14 |

---

## 1. Ringkasan

Sejak OpenCode 1.19.0, free tier menjawab **setiap** request dengan

```
403 FreeTierError: OpenCode's free tier can only be used from within OpenCode.
```

Akar penyebabnya bukan bentuk id, melainkan **isinya**. `x-opencode-session` dibaca upstream sebagai
`Identifier.descending("session")`: dua belas karakter hex pertama adalah 48 bit terbawah dari
`~(BigInt(ms) * 0x1000n + counter)`. app-serv menaruh `sha256(...)[0:6]` di sana, sah secara regex,
dan selama upstream hanya mengecek regex, itu lolos. Begitu field-nya didekode sebagai waktu, nilai
acak berarti "timestamp dari era yang tidak masuk akal", dan jawabannya 403.

| # | Tingkat | Isi |
|---|---|---|
| **F1** | CRITICAL | Prefix sesi berupa bytes digest, bukan clock → `403 FreeTierError` untuk seluruh lane `oc/*` (`opencode_session.go`) |
| **F2** | HIGH | `x-opencode-request` tidak pernah dikirim; upstream yang sama menegakkan id turn pada 1.19.0 (`opencode_auth.go`) |
| **F3** | HIGH | Nama tool tidak disanitasi ke `^[a-zA-Z0-9_-]+$`; `mcp:fs:read` dijawab `400 invalid_request_error` (`opencode_responses_items.go`) |
| **F4** | MEDIUM | `tool_choice` yang menunjuk nama lama tidak ikut dibuang, konsekuensi langsung dari F3 (`opencode_responses.go`) |
| **F5** | HIGH | `function_call` yatim pasca `/compact` atau turn yang diinterupsi dijawab `400 No tool output found` (`opencode_responses_items.go`) |
| **F6** | MEDIUM | `max_output_tokens` hanya di-floor, tidak di-plafon; combo 1M konteks ditolak (`opencode_body.go`) |
| **F7** | HIGH | Model non-Anthropic mengarang nama parameter (`cmd`, `path`, `query`) dan Claude Code menolak sebelum menjalankan: `InputValidationError: The required parameter 'command' is missing` (`translate_response.go`, jalur stream) |
| **F8** | MEDIUM | `toolCallFrames` membuka blok lagi setiap kali upstream mengulang `id` yang sama, blok terbuka tak tertutup, dan buffer argumen pindah ke index yang tak pernah di-stop |

## 2. Bukti terukur

Tes live yang sudah ada di repo, tidak diubah, dijalankan utuh sebelum dan sesudah. Host yang sama,
byte yang sama, hanya kode yang berbeda.

**M1 sebelum perbaikan (baseline).** `go test -tags=integration,live -run TestOpenCodeFreeLive ./internal/dataplane/`

| Ref | Hasil |
|---|---|
| `oc/space-bunny-free` | **403** `FreeTierError` setelah 3 percobaan (1,78 s) |
| `oc/mimo-v2.6-flash-free` | **403** `FreeTierError` (0,36 s) |
| `oc/muse-spark-1.3-contributor-free` | **403** `FreeTierError` (0,45 s) |

Pesan yang terbaca di gateway: `UPSTREAM_ERROR: OpenCode's free tier can only be used from within
OpenCode. Download it at https://opencode.ai and keep building.: upstream returned Forbidden: …`

**M5 sesudah perbaikan.** Perintah yang sama.

| Ref | Hasil |
|---|---|
| `oc/space-bunny-free` | **200**, jawaban teks (2,26 s) |
| `oc/mimo-v2.6-flash-free` | **200**, jawaban teks (2,33 s) |
| `oc/muse-spark-1.3-contributor-free` | **200**, jawaban teks (1,50 s) |

`PASS 3/3`, 6,14 s. Yang dicatat jujur: patch dipasang sekaligus (C1-C6 pada satu request), jadi bukti
ini menunjukkan **patch** mengangkat 403, bukan salah satu header secara terpisah. Bisection per-header
tidak dilakukan karena setiap siklus memakan kuota anonim upstream.

**M6 gerbang hermetik.** `engine_opencode_free_test.go` sekarang menolak tiga hal yang dulu lolos:
sesi tanpa clock yang bisa didekode, sesi tanpa clock dalam jendela, dan request tanpa id turn.
Stand-in-nya menulis aturan decoding sendiri (bukan mengimpor helper connector) supaya ia tidak bisa
meloloskan bug pada kode yang ia jagakan, itulah cara bentuk SHA-256 lama lolos selama ini.

## 3. Tujuh perubahan

| # | File | Isi |
|---|---|---|
| C1 | `opencode_id_time.go` (baru), `opencode_session_cache.go` (baru), `opencode_session.go` | Prefix 12-hex = 48 bit terbawah dari komplemen `ms*0x1000 + counter` (descending, sesi) dan `ms*0x1000 + 1` (ascending, pesan). Suffix base62 dari digest tetap membawa identitas |
| C2 | `opencode_request.go` (baru), `opencode_auth.go` | `x-opencode-request` diturunkan dari sesi + pesan user terakhir pada body yang **akan berangkat** (`req.GetBody()`), karena `ApplyAuth` berjalan setelah `TransformRequest` |
| C3 | `opencode_responses_tools.go` (baru) | Nama tool disanitasi rune-per-rune ke alfabet wire, **lalu** dipotong ke 128, memotong dulu akan menghitung byte nama multi-byte terhadap batas yang dihitung upstream dalam karakter |
| C4 | `opencode_responses_pairing.go` (baru), `opencode_responses_items.go` | Dua fase: koreksi selesai dulu, pairing membaca `call_id` **pasca-clamp**, nilai yang upstream lihat, bukan yang klien tulis |
| C5 | `opencode_body.go` | `clampOpenCodeCeiling` menaiki floor lalu memasang plafon 65536 terakhir, sama seperti `Math.min(Math.max(x,16),65536)` |
| C6 | `opencode.go`, `registry.yaml:993,2635` | `opencode/1.18.31` → `opencode/1.19.0` |
| C7 | `translate_claude_tool_input.go` (baru), `translate_stream_claude_tools.go` (baru), `translate_stream_claude_passthrough.go` (baru), 4 file diubah | Tabel penggantian nama parameter; dipasang di dua jalur non-streaming dan di jalur stream lewat buffer argumen per index blok |

## 4. Keputusan desain

**Session id tidak lagi pure function.** Ini membatalkan keputusan yang ditulis dua kali di kode lama
dan di draft 029 §5. Konsekuensinya connector menyimpan state bersama, jadi state itu dijaga satu mutex
yang menutup map **dan** pasangan `(lastMs, counter)`, dua mint dalam milidetik yang sama tidak mungkin
berbagi counter.

**Eviction: buang `usedAt` terkecil saat penuh, bukan slice urutan.** Reference meniru `Map` JS dengan
`delete` + `set` ulang setiap hit. Menumuskan urutan itu di Go berarti memindahkan 1000 entri pada setiap
request; `usedAt` memberi hasil yang sama untuk kasus ini (store hanya ditulis saat miss) tanpa biaya per
request.

**TTL 2 jam + sweep 30 menit, tanpa goroutine.** Reference menjalankan `setInterval` untuk sweeper-nya.
Gateway tidak menyerahkan lifecycle ke connector dan §1.6 melarang goroutine tanpa kondisi berhenti, jadi
sweep berjalan di dalam mint yang sama, paling sering satu kali per 30 menit. Angka TTL dan intervalnya
persis `MEMORY_CONFIG` reference.

**Tanpa TTL, satu sesi terkunci selamanya.** Jendela kuota per sesi adalah milik upstream; store tanpa
batas waktu tidak pernah merotasinya. Itu alasan TTL dibawa, bukan kesibukan menambah fitur.

**`deriveRequestId` di-cache; reference tidak.** Reference menulis id pesan dari clock saat itu juga,
sehingga retry pada milidetik yang berbeda menghasilkan id yang berbeda, sementara komentar mereka
sendiri menuntut id yang sama untuk satu turn. app-serv memakai store yang sama, jadi sifat yang
didokumentasikan itu benar-benar berlaku di sini. Ini deviasi dari PR, ke arah niatnya.

## 5. Yang tidak di-port, dan mengapa

| | |
|---|---|
| `pages` / `isValidPdfPagesArg` | app-serv tidak punya konsep `pages`; menambah validator untuk field yang tidak ada adalah dead code |
| Menghapus alias untuk semua tool | Reference hanya meretire empat alias Bash; `path` tetap tinggal di samping `file_path`. Dibiarkan sama rata, goldens test mem-pin fakta ini supaya "pembersihan" berikutnya terbaca sebagai perubahan kontrak, bukan perbaikan bug |
| Dedup nama setelah sanitasi | `a.b` dan `a_b` bisa bertabrakan pada nama yang sama. Reference tidak mendedup; kita juga tidak. Dicatat sebagai sisa |
| `output:"{}"` sintetis | Literal reference. Sebagian backend ingin dokumen JSON sungguhan; tidak diubah tanpa pengukuran |
| Normalisasi pada jalur passthrough Claude | `forwardClaude` tidak menyentuh argumen, sama seperti reference, yang hanya memasang penerjemah pada arah OpenAI→Claude |
| Header identitas pada lane berkunci | `ApplyAuth` keluar lebih awal ketika ada key (`opencode_auth.go:36-46`), sehingga `opencode-zen`/`opencode-go` berkunci tidak mengirim sesi, UA, maupun id turn. Jika lane itu kena 403 yang sama, patch ini tidak menolongnya, keputusan terpisah, bukan bagian dari PR |

Konsekuensi yang diterima sadar-sadar: pada stream klien, input tool kini muncul sekali di akhir blok,
bukan sedikit demi sedikit. Reference berperilaku sama, dan tidak ada cara menamai ulang sebuah objek dari
potongannya.

## 6. Wrap 48 bit

`Date.now()*0x1000` melewati `2^48` sejak 2021, jadi field hanya memuat 36 bit terbawah dari
millisecond. Konsekuensinya, dan ini sifat skema upstream, bukan bug port: decode prefix mengembalikan
`ms mod 2^36`, dan itu yang assert clock di test bandingkan. Komentar `opencode_id_time.go` menuliskannya
supaya tidak ada yang "memperbaiki" field ini jadi lebih lebar.

## 7. Test

Baru (10 file): `opencode_id_time_test.go` (oracle `math/big` yang menghitung ulang aritmetika BigInt
reference, ekuivalensi `%012x`, urutan descending/ascending, kasus wrap), `opencode_session_test.go`
(prefix didekode ke waktu mint, asersi yang tidak ada sebelumnya), `opencode_session_cache_test.go`
(clock yang diinjeksi: reuse, counter per milidetik, TTL, bound, sweep; plus adu 8 goroutine × 200
panggilan melewati bound), `opencode_request_test.go`, `opencode_responses_names_test.go`,
`opencode_responses_pairing_test.go`, `opencode_ceiling_test.go`, `translate_claude_tool_input_test.go`,
`translate_stream_claude_tools_test.go`, `translate_response_openai_claude_test.go` (menutup lubang:
`OpenAIToClaudeResponse` tidak punya satu test pun sebelum ini).

Berubah: `opencode_auth_test.go` (id turn: bentuk, stabil untuk body sama, beda untuk turn berbeda),
`engine_opencode_free_test.go` (gerbang M6 di §2).

`go test -race ./internal/provider/ ./internal/dataplane/` bersih; `go vet ./...` bersih; suite hermetik
penuh hijau; tidak ada `t.Skip`.

## 8. Batas §1.1 yang ditemui

`opencode.go` duduk tepat di 220 (ambang warning, bukan failure) sehingga C6 harus in-place satu baris,
dan `translate_stream_claude.go` sudah warning di 236, karena itu `Usage`/`mergeUsage`/`forwardClaude`
dipindah ke file sendiri lebih dulu, baru satu field `tools` ditambahkan: file itu **susut** ke 204.
Tiga file lain dipecah untuk hal yang sama (`opencode_responses_items.go` 192 → 118, plus tools dan
pairing). Semua file yang diubah berhenti di bawah 220.
