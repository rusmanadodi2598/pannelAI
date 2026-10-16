# 054-DATAPLANE-CEILING-SIGNAL-IN-STREAMS.md: jawaban yang terpotong di ceiling dilaporkan sebagai jawaban selesai ke klien stream

Permintaan owner: "untuk models baru deepseek sudah bekerja dengan baik dengan patch ini... hanya saja case
untuk penguatan dalam streaming, karena di beberapa case response di codingan setelah thinking models
dilakukan, response berhenti, dan kita di codingan CLI, IDE, sejenisnya, harus continue manual lagi. Cek
bagian engine tersebut."

| | |
|---|---|
| **Status** | **SEBAGIAN CLOSED 2026-10-15.** P1, P2, P4 mendarat (dua arah terjemahan); **P3 = D2 masih terbuka** dan sengaja tidak diputuskan. Suite penuh + `-race` hijau, `go vet` bersih, gate headers & antislop PASS; belum di-push |
| **Mechanism** | BEFORE (draft ini) → DURING saat patch |
| **Scope** | `app-serv/internal/dataplane` (`translate_stream_responses.go`, `translate_stream_responses_client_lifecycle.go`, `translate_responses_answer.go`), `app-serv/internal/schema/responses_answer.go` |
| **Permintaan owner** | perkuat streaming supaya pemotongan tidak lagi terasa seperti "selesai sendiri" |
| **Reference** | `mhiqrambg/9router-mibp-version` - tidak punya continuation generik; mekanisme repair-nya khusus Kiro dan hidup di executor sendiri (`open-sse/executors/kiro.js:35-39`, `REPAIR_INSTRUCTIONS`) |
| **Kaitan** | draft 051 (gerbang identitas free tier), draft 053 (katalog free + `deepseek-v4.1-flash-free`), SPEC-API §7.6 |
| **Tanggal** | 2026-10-15 |

---

## 1. Ringkasan

Yang bermasalah bukan thinking-nya dan bukan upstream-nya. **Sinyal pemotongan tidak selamat lewat
terjemahan wire.** Satu peristiwa yang sama, dua jawaban berbeda tergantung bentuk klien:

| Klien | Upstream berhenti di ceiling | Yang diterima klien |
|---|---|---|
| non-stream (chat) | fold membaca `response.incomplete` | `finish_reason: "length"` - benar, sudah di-pin `engine_forced_stream_incomplete_test.go` |
| **stream, wire chat** | upstream mengirim `finish_reason: "length"` | `length` - benar (terukur live, P1 di bawah) |
| **stream, sumber Responses** | `response.incomplete` | **`finish_reason: "stop"`** - salah (terukur replay R-1) |
| **stream, klien Responses** | upstream chat memotong (`length`) | **`response.completed` / `status:"completed"`** - salah (terlacak kode C-1) |

Artinya bagi coding CLI/IDE: sebuah turn yang habis budget-nya setelah thinking **dilaporkan sebagai turn
yang selesai normal**. Klien yang wajar akan melanjutkan sendiri kalau ia melihat `length`/`incomplete`;
kalau yang ia baca `stop`/`completed`, satu-satunya jalan adalah user mengetik "continue" - persis gejala
yang owner sebut.

## 2. Bukti

**P - live, upstream nyata, lane OpenCode Free** (`Engine.Relay`, tanpa kredensial, 2026-10-15):

| # | Body | Yang sampai ke klien |
|---|---|---|
| P1 | `deepseek-v4.1-flash-free`, `max_tokens: 32` | reasoning mengalir, content kosong, **`finish_reason:"length"`** ✅ |
| P2 | idem, tanpa ceiling | 457 frames, 3.4k karakter, `stop` - jawaban memang selesai |
| P3 | idem + 1 tool, `max_tokens: 120` | tool call tertutup rapi, `finish_reason:"tool_calls"` |

Jadi keluarga deepseek (wire `chat/completions`) sudah benar; gejala owner tidak datang dari sana.

**R - replay hermetik, body SSE yang sudah ada di repo** (`responsesIncompleteStreamBody`,
`engine_forced_stream_incomplete_test.go:29-38`) dijalankan melalui jalur stream klien
(`NewStreamState` → `Frames(TargetResponses, …)` → `Finish()`), event demi event persis seperti `pump`:

| # | Isi stream upstream | Frame penutup yang dibaca klien |
|---|---|---|
| R1 | text delta + `response.incomplete{reason:"max_output_tokens"}` | `{"delta":{},"finish_reason":"stop"}` ❌ |
| R2 | tool call terbuka (`arguments: "{\"path\""`) + incomplete yang sama | `{"delta":{},"finish_reason":"stop"}` ❌ |

**C - lintasan kode yang menghasilkan R1/R2:**

| # | Tempat | Fakta |
|---|---|---|
| C1 | `translate_stream_responses.go:66-90` | Switch peristiwa sumber Responses menyebut `completed`, `done`, `failed`, `error` - **`response.incomplete` tidak ada**. Jatuh ke `return nil`: tidak ada frame, `finishReason` tetap kosong |
| C2 | `translate_stream_openai.go:135-139` | `Finish()` menutup stream yang tidak pernah melaporkan alasan dengan **`FinishStop` yang disintesis** - perilaku yang benar untuk koneksi yang putus, dan yang salah untuk pemotongan yang upstream-nya justru bilang "incomplete" |
| C3 | `translate_stream_responses_client_lifecycle.go:44-59` | `completedFrame()` selalu menulis `status: ResponsesStatusCompleted`. Konstanta `ResponsesStatusIncomplete` (`schema/responses_answer.go:20`) **tidak dipakai satu kali pun** di kode non-test: vocabulary-nya ada, jalur stream-nya tidak pernah mengisinya |
| C4 | `engine_forced_stream.go:108` | Jalur fold **memang** membaca `EventResponseIncomplete`. Asimetri inilah intinya: dua jalur dari peristiwa yang sama, hanya satu yang jujur |

## 3. Temuan

| # | Tingkat | Isi |
|---|---|---|
| **F1** | HIGH | Pemotongan ceiling dari sumber Responses ditutup dengan `finish_reason:"stop"` ke klien stream (C1+C2, terukur R1). Klien coding agent menganggap turn selesai dan berhenti meminta lanjutan |
| **F2** | HIGH | Arah sebaliknya: sumber chat yang melaporkan `length` ditulis ke klien Responses sebagai `response.completed` (C3). Klien Codex-style tidak pernah melihat `incomplete_details` |
| **F3** | MEDIUM | `responsesFinishReason` (`translate_responses_answer.go:86-96`) menguji `hasCalls` **sebelum** status incomplete. Jawaban yang terpotong saat arguments belum tertutup dilaporkan sebagai `tool_calls` - panggilan dispatch dengan JSON setengah (`{"path"`), yang justru mengundang klien mengeksekusi sampah |
| **F4** | - | Bukan temuan, hasil pemeriksaan: wire chat stream untuk lane free sudah benar (P1), dan connector tidak mengarang ceiling (`opencode_body.go:141-144`, "a ceiling the client never sent stays absent"). Tidak ada yang perlu diubah di sana |

## 4. Keputusan

**D1 - satu fungsi keputusan, dua arah.** `length` vs `stop` untuk satu peristiwa pemotongan sudah punya
satu jawaban di repo: `responsesFinishReason`. Yang hilang adalah pemanggilnya di jalur stream. Jadi
perbaikannya bukan membuat aturan kedua, melainkan memasang yang sudah ada di dua titik yang belum memakai:
sumber Responses → klien (F1) dan sumber mana pun → klien Responses (F2). Aturan yang ditulis dua kali
adalah cara fold dan stream bercerai lagi.

**D2 - tool call terpotong: PUTUSAN OWNER.** Dua pilihan, keduanya masuk akal:
- *(a) laporkan `length`.* Jujur: JSON-nya belum tertutup dan tidak bisa dispatch. Klien yang melanjutkan
  turn akan mendapat arguments utuh; klien yang nekat dispatch tetap gagal, tapi ia gagal pada angka yang
  benar.
- *(b) laporkan `tool_calls` dan buang panggilannya.* Tidak ada call setengah yang sampai ke klien; teks
  reasoning yang sudah tiba tetap berguna.

  Rekomendasi draft: **(a)**. Alasannya konkret: `answerShowable` (`engine_answer_completeness.go:52-57`)
  sudah menganggap sebuah tool call sebagai "jawaban yang layak disajikan"; kalau call itu ikut dilaporkan
  sementara arguments-nya terpotong, aturan yang sama justru melindungi data rusak.

**D3 - tidak ada auto-continue di gateway.** Menambah "lanjutkan otomatis" berarti mengirim ulang turn atas
nama klien, memakan kuota anonim upstream, dan menagih usage dua kali untuk satu permintaan user. Reference
sendiri tidak punya yang generik - repair retry-nya khusus Kiro dan terjadi di dalam executor Kiro
(`kiro.js:124-131`), bukan di jalur umum. Yang kita kirim adalah **fakta** bahwa jawabannya belum selesai;
keputusan melanjutkan tetap milik klien.

**D4 - apakah ceiling perlu dinaikkan? PUTUSAN OWNER, dan butuh angka.** Kalau coding CLI owner mengirim
`max_tokens` kecil (mis. 1024) ke model yang berpikir panjang, upstream memotong dan P1 adalah perilaku yang
benar - tidak ada bug di sini, hanya budget. Satu-satunya tuas yang jujur adalah floor per model yang sudah
ada mekanismenya (`min_output_tokens`, dipakai muse-spark 1.3 = 512). Memberi angka untuk
`deepseek-v4.1-flash-free` tanpa pengukuran adalah mengarang (aturan yang sama menahan draft 053 D1), jadi
kalau owner mau jalur ini, ukur dulu: prompt coding nyata, ceiling berapa yang menyelesaikan turn.

## 5. Perubahan

| # | File | Isi | Status |
|---|---|---|---|
| P1 | `translate_stream_responses.go` | `EventResponseIncomplete` masuk ke case terminal (sebelumnya jatuh ke `return nil`), dan `completed()` tidak lagi menulis aturannya sendiri: ia memanggil `responsesFinishReason(response, r.sawToolCall)` - jawaban yang sama yang dipakai fold. 191 baris | **MENDARAT** |
| P2 | `translate_stream_responses_client.go`, `translate_stream_responses_client_lifecycle.go` | finish reason yang datang dari upstream kini disimpan (`finishReason`), dan `completedFrame()` menulis `response.incomplete` + `status:"incomplete"` + `incomplete_details.reason` untuk `length`; `ResponsesStatusIncomplete` yang selama ini tidak terpakai akhirnya terpakai | **MENDARAT** |
| P3 | `translate_responses_answer.go` | Precedence `hasCalls` di atas status incomplete (= keputusan D2) | **DIPINDAH ke draft 055 F1, owner memutuskan di sana** |
| P4 | test baru + satu test lama dikoreksi | lihat §6 | **MENDARAT** |
| - | `translate_responses_answer.go` | `ResponsesCeilingReason` menjadi satu ejaan untuk `max_output_tokens`, dipakai arah baca dan arah tulis; `schema.ResponsesStatusIncomplete` dipakai di tempat string literalnya | MENDARAT |

Tidak ada perubahan connector, tidak ada migrasi, `SYSTEM_MAP.md` N/A (tidak ada batas domain atau topology
yang bergerak; ini koreksi penerjemahan di dalam satu service).

## 6. Test (hasil nyata)

`translate_stream_responses_ceiling_test.go` (baru, 149 baris) mem-pin dua arah:

- sumber Responses → klien chat: `response.incomplete{max_output_tokens}` pada jawaban teks → `length`;
  incomplete dengan alasan lain (`content_filter`) → tetap `stop`; dan **kasus tool call terpotang dipin
  apa adanya sebagai `tool_calls`**, supaya P3/D2 terbaca sebagai keputusan yang belum diambil, bukan
  sebagai efek samping perbaikan ini.
- sumber chat → klien Responses: chunk `finish_reason:"length"` → satu `response.incomplete` dengan
  `status` dan `incomplete_details.reason` di dalamnya, dan **nol** `response.completed`; `stop` biasa tetap
  `response.completed`.

Satu test lama ternyata mem-pin perilaku cacatnya: `translate_stream_responses_tools_test.go` menulis
"an answer cut by the ceiling still finishes as a stop" danexpects `stop` untuk event yang payload-nya
sendiri berkata `status:"incomplete"`. Yang diubah adalah **ekspektasinya** (`length`), dengan alasan
tertulis di test: objek response-lah otoritas atas bagaimana jawaban berakhir, sama seperti yang fold baca -
bukan nama event-nya.

Yang tetap hijau dan memang harus tetap hijau: `engine_forced_stream_incomplete_test.go` (fold, `length`),
`TestResponsesClientStream_ClosersAreIdempotent` dan `SecondFinishRepeatsNothing` (satu event penutup per
stream), sanitiser same-format chat→chat, dan `StreamState.Finish()` yang masih mensintesis `stop` untuk
stream yang benar-benar putus tanpa peristiwa terminal - aturan itu tetap benar untuk koneksi yang mati.

`go test ./...` hijau, `go test -race ./internal/dataplane/` hijau, `go vet ./...` bersih, gate §1.2 headers
PASS, antislop PASS. File terpanjang yang tersentuh 191 baris.

**Yang belum terukur dan tidak boleh diklaim:** R1/R2 adalah replay dari bentuk peristiwa upstream yang
sudah di-pin test fold (pengukuran upstream 2026-09-24), bukan pengukuran live baru pada muse-spark. Kalau
owner mau bukti ujung-ke-ujung dari upstream nyata, itu satu request live pada
`opencode/muse-spark-1.3-contributor-free` dengan `max_tokens` kecil - memakan kuota anonim, jadi tidak saya
jalankan tanpa persetujuan.

## 7. Pertanyaan untuk owner (menentukan prioritas P1 vs P2)

Gejalanya cocok dengan F1 **atau** F2, dan keduanya punya fix sendiri. Yang membedakan hanya bentuk request
klien:

1. CLI/IDE apa yang dipakai, dan endpoint mana yang ia panggil - `/v1/chat/completions`, `/v1/responses`,
   atau Messages? (opencode/Cline → chat, F1 hanya kena model muse-spark; Codex → Responses, F2 yang kena.)
2. Model mana yang sedang dipakai waktu berhenti - `deepseek-v4.1-flash-free` (chat) atau `muse-spark-*`
   (Responses)?
3. Request itu mengirim `max_tokens` berapa?

Dengan tiga jawaban itu, satu dari dua jalur ini terbukti jadi penyebabnya dan patch bisa dijalankan tanpa
meraba. Tanpa itu, P1-P3 tetap layak dikerjakan karena keduanya adalah bug penerjemahan yang berdiri sendiri.
