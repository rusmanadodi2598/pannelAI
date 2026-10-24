# 055-DATAPLANE-CEILING-FOLLOWUPS.md: dua keputusan yang ditunda dari draft 054

Permintaan owner: "Masukan dulu ke DRAFT lanjutan aja." Ini draft lanjutan itu. Tidak ada kode yang
berubah di sini; isinya dua keputusan yang sengaja tidak diambil di draft 054 supaya perbaikan sinyal
ceiling mendarat tanpa membawa perubahan kontrak yang masih diperdebatkan.

| | |
|---|---|
| **Status** | **CLOSED 2026-10-16 lewat draft 058.** F1 mendarat sebagai aturan preseden: jawaban incomplete beralasan `max_output_tokens` dengan call yang belum tertutup dilaporkan `length`, dan call yang sudah tertutup tetap `tool_calls` (itu carve-out yang di bawah ini diminta diperiksa, sekarang di-pin dua arah, stream dan body). F2 dijawab untuk tiga model dengan angka terukur: `muse-spark-1.3-contributor-free` 512 dinaikkan ke 2048 karena 512 dan 1024 menjawab call kosong, `deepseek-v4.1-flash-free` 2048, `space-bunny-free` 1024. Keduanya dulunya pin keadaan; sekarang keduanya pin keputusan |
| **Mechanism** | BEFORE |
| **Scope** | `app-serv/internal/dataplane/translate_responses_answer.go` (F1), `app-serv/internal/registry/registry.yaml` (F2), test yang mem-pin-nya |
| **Kaitan** | draft 054 (sinyal ceiling di stream - P3 di sana adalah F1 di sini), draft 053 (katalog free tier, `min_output_tokens`), draft 051 (`clampOpenCodeCeiling`) |
| **Tanggal** | 2026-10-15 |

---

## F1 - tool call yang terpotong di tengah `arguments`

Keadaan hari ini, dan ia di-pin di `translate_stream_responses_ceiling_test.go`: sebuah call yang
arguments-nya belum tertutup (`{"path"`) dilaporkan sebagai `tool_calls`, bukan `length`. Alasannya ada di
`responsesFinishReason` (`translate_responses_answer.go:86-98`): `hasCalls` diuji **lebih dulu**, jadi status
incomplete tidak sempat bicara.

| Jalan | Isi | Konsekuensi |
|---|---|---|
| (a) laporkan `length` | Pemotongan ceiling diuji sebelum `hasCalls` | Klien tidak mendapat call yang tidak bisa dispatch; ia tahu jawabannya belum selesai dan melanjutkan turn. Yang hilang: call yang *sebetulnya* sudah lengkap tapi kebetulan datang bersama jawaban incomplete bisa ikut tersapu - perlu dilihat dulu apakah incomplete dengan `output` berisi call lengkap itu nyata di upstream |
| (b) laporkan `tool_calls` dan buang call-nya | Teks reasoning yang sudah tiba tetap berguna; call setengah tidak pernah sampai ke klien | Klien mengira turn selesai dengan rapi, lalu bingung tidak menemukan call yang ia tunggu |
| (c) biarkan apa adanya | Tidak ada perubahan | Klien coding agent memanggil tool dengan JSON rusak |

Untuk kasus owner (coding agent, combo berisi model free OpenCode) ini justru jalur yang paling sering
dilewati: turn model coding berakhir di tool call, jadi pemotongan setelah thinking hampir selalu memotong
arguments, bukan paragraf. F1 kemungkinan besar adalah gejala yang sebenarnya dia lihat, dan draft 054 tidak
menyentuhnya.

Ukuran yang perlu diambil sebelum memilih: satu live probe `opencode/deepseek-v4.1-flash-free` dengan tool
dan ceiling yang dipotong pas di tengah arguments, lalu catat apakah upstream menutup dengan
`response.incomplete` (wire Responses) atau `finish_reason:"length"` (wire chat) untuk bentuk itu. Wire chat
sudah terukur (draft 054 P3): ia mengirim `length`. Wire Responses belum.

## F2 - ceiling minimum untuk model yang berpikir

Terukur di draft 054: `deepseek-v4.1-flash-free` dengan `max_tokens: 32` → reasoning mengalir, content
kosong, `length`; tanpa ceiling ia menyelesaikan 3.4k karakter dan menutup dengan `stop`. Jadi pemotongan
hanya terjadi ketika klien mengirim ceiling kecil dibanding thinking-nya.

Satu-satunya tuas yang jujur adalah floor per model (`min_output_tokens` di `registry.yaml`, yang
`openCodeOutputFloor` naikkan dari 16), dan ia sudah dipakai muse-spark 1.3 = 512 atas dasar pengukuran.
Angka untuk model lain tidak boleh disalin dari angka itu: muse-spark 1.3 terukur menghabiskan ceiling kecil
sepenuhnya pada thinking, sementara deepseek-v4.1-flash-free terukur **selesai** pada 3.4k karakter tanpa
ceiling - dua kondisi yang berbeda.

Yang dibutuhkan untuk menutup F2: jawaban owner atas satu pertanyaan yang belum terjawab sampai sekarang -
**`pi coding cli` mengirim `max_tokens`/`max_completion_tokens` berapa, dan ke endpoint mana** (`/v1/chat/completions`,
`/v1/responses`, atau Messages). Dengan itu jelas apakah perbaikan yang benar ada di gateway (floor) atau di
sisi klien (ceiling yang terlalu kecil untuk model thinking). Tanpa itu, menambah angka ke registry berarti
mengarang - aturan yang sama menahan draft 053 D1.

## Yang tidak akan dilakukan di sini

Gateway tidak akan meneruskan turn atas nama klien. Menambah "auto-continue" berarti mengirim ulang turn ke
upstream yang kuota anonimnya milik bersama, dan menagih usage dua kali untuk satu permintaan user.
Reference juga tidak punya yang generik: repair retry-nya khusus Kiro dan hidup di dalam executor Kiro
(`open-sse/executors/kiro.js:124-131`). Tugas gateway adalah mengirim fakta bahwa jawaban belum selesai -
yang itu sudah selesai di draft 054 - dan keputusan melanjutkan tetap milik klien.
