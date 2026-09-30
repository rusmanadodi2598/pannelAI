# 038-CODEBUDDY-OPENAI-WIRE-PARITY.md: kenapa `stop`, `tool_choice` paksa, dan nama model di jawaban gagal pada codebuddy-intl, serta apa yang sudah ditutup

Register temuan `app-serv` dari pengujian data plane yang diminta owner atas satu model
`codebuddy-intl/deepseek-v4.1-flash`, empat mode uji (chat completion, tool, streaming, reasoning).
Register ini **mengubah** kode: ketujuh temuan ditutup di ronde ini, satu temuan sejenis dicatat
sebagai sisa (§9).

|                      |                                                                                                                                                                                                                                                                                                          |
| -------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**           | **CLOSED 2026-09-30.** F1 `stop_sequence.go` + `translate_stream_openai_sanitize.go`; F2 `normalizeStopMember`; F3 `codebuddy_choice.go`; F4 lepasnya strip pada `foldedChat.response()`; F5 `Resolution.Requested` + `ClientModel()`; F6 prune delta; F7 `jsonErrorTail`. Sisa: wire Anthropic untuk `stop_sequences` (§9) |
| **Mechanism**        | AFTER (temuan diukur live, patch mendarat di working tree, diverifikasi ulang live)                                                                                                                                                                                                                       |
| **Scope**            | Pengujian gateway di `127.0.0.1:9090` atas `codebuddy-intl/deepseek-v4.1-flash` pada `POST /api/v1/chat/completions` (stream + non-stream, tools, reasoning). Daftar model `/api/v1/models` menjawab 314 entri; model target ada di dalamnya                                                                 |
| **Permintaan owner** | "Testing request response: Chat Completion, Tool, Streaming, Reasoning" atas endpoint + kunci + satu model itu (2026-09-30), lalu "Fix temuannya"                                                                                                                                                        |
| **Reference**        | `docs/DRAFT/034-DATAPLANE-STREAM-DUPLICATE-FRAMES.md` (framing, `finish_reason` ganda, usage chunk); `docs/DRAFT/021-DATAPLANE-SSE-FRAMING.md` §2–§4; `sseToJsonHandler.js` aturan strip `reasoning_content` yang baru saja ditinggalkan (§5)                                                              |
| **Kaitan**           | SPEC-API §4 (SSE + usage chunk), §7.6 (nama model pada jawaban), §7.15 (matriks translasi); `internal/dataplane/{stop_sequence,translate_stream_openai_sanitize,engine_forced_chat,engine_forced_stream,engine_translate,engine_answer,engine_stream,request,resolve,answer_identity}.go`; `internal/provider/codebuddy_{body,choice}.go`; `internal/schema/chat.go` |
| **Kontrak**          | `docs/CONTRACT/001-CONTRACT-API-V1.yaml` tidak berubah: tidak ada field baru, tidak ada tipe yang diganti. Yang berubah adalah perilaku terhadap field yang sudah ada                                                                                                                                    |
| **Tanggal**          | 2026-09-30                                                                                                                                                                                                                                                                                                |

---

## 0. Label temuan

| Label | Gejala                                                                   | Lapisan yang salah                     | Jawaban              |
| ----- | ------------------------------------------------------------------------ | -------------------------------------- | -------------------- |
| F1    | `stop` tidak berpengaruh sama sekali — output identik byte per byte       | dataplane: janji OpenAI tidak ditepati | §2, **CLOSED**       |
| F2    | `stop` bentuk string ditolak vendor (`UPSTREAM_REJECTED` 400)             | dataplane: body diteruskan apa adanya  | §3, **CLOSED**       |
| F3    | `tool_choice` obyek bernama ditolak vendor (400)                          | provider: bentuk tak didukung vendor   | §4, **CLOSED**       |
| F4    | `reasoning_content` hilang di jawaban non-stream, token-nya tetap ditagih | dataplane: fold menghapus               | §5, **CLOSED**       |
| F5    | `model` jawaban tidak bisa dikirim balik (`MODEL_NOT_FOUND`)               | dataplane: nama vendor yang dipakai     | §6, **CLOSED**       |
| F6    | Delta stream membawa noise vendor dan `finish_reason:""`                   | dataplane: passthrough verbatim         | §7, **CLOSED**       |
| F7    | Pesan validasi membocorkan nama tipe Go                                     | schema: `jsonErrorTail`                  | §8, **CLOSED**       |

## 1. Yang diuji sebelum patch

Ronde pengukuran: 13 panggilan pada model target. Empat mode dasar **lulus** — chat completion
(`PONG`, 2,52 s), tool non-stream (`finish_reason: tool_calls`, argumen JSON valid), tool round-trip
dengan hasil tool, streaming (11 chunk, `data:` + `[DONE]`, usage di ekor, delta tool terangkai per
`index`), dan reasoning pada stream (799 karakter `reasoning_content`, 136 `reasoning_tokens`).
Empat error path menjawab terstruktur dan berbahasa Inggris (401 kunci salah, 400 model tak dikenal,
400 body cacat). Tujuh sisanya menjadi F1–F7.

## 2. F1 — `stop` diabaikan

**Gejala.** `stop:["STOPHERE"]` dengan kalimat yang memang memuat `STOPHERE` menghasilkan output yang
**identik byte** dengan tanpa `stop`: `A STOPHERE B STOPHERE C`, `finish_reason: stop`, 9 token. Wire
OpenAI menjadikan `stop` janji gateway, bukan saran vendor.

**Akar.** `codebuddy_body.go` hanya menyentuh `messages` dan `reasoning_effort`; `stop` diteruskan apa
adanya dan vendor mengabaikannya. Tidak ada satu pun jalur yang memotong teks untuk klien. Satu-satunya
pembaca `StopSequences()` adalah translasi OpenAI→Claude (`translate_openai_claude.go:96`), jadi klien
OpenAI tidak pernah sampai ke sana.

**Jawaban.** Potong dilakukan di sisi gateway, pada dua bentuk jawaban:

* **Stream** — `stopGuard` menahan ekor yang masih mungkin menjadi marker, jadi marker yang dipecah
  vendor melintasi dua frame (`"AB STOP"` lalu `"HERE"`) tidak pernah bocor. Frame yang sudah terpotong
  dibuang; alasan `stop` diumumkan **sekali** (`cutAnnounced`) supaya frame penutup vendor tidak
  melahirkan `finish_reason` ganda (034 F2). Tahan yang terbukti bukan marker dilepas di `Finish()`.
* **Fold** — `foldedChat.stop` memotong `content` saat jawaban tunggal disusun, dan memaksa
  `finish_reason: stop`: vendor yang melanjutkan sampai langit-langitnya tidak boleh melaporkan
  `length` untuk teks yang tidak dikirim ke pemanggil.

**Verifikasi live.** Stream dengan `stop:["STOPHERE"]` → teks terangkai `'A '`, satu
`finish_reason: stop`, `[DONE]` ada. Non-stream `stop:"green"` → `'red\n'`.

## 3. F2 — `stop` bentuk string ditolak vendor

`ChatRequest.StopSequences()` membaca dua bentuk, dan `schema/chat.go` sengaja menerima keduanya karena
OpenAI mendefinisikan keduanya. Yang dikirim ke vendor tetap apa adanya, sehingga `"stop":"green"`
menarik `Bad Request` sedangkan `["green"]` diterima.

`normalizeStopMember()` dijalankan pada jalur same-format untuk target OpenAI: string menjadi array
satu unsur, array dan `null` dibiarkan byte-per-byte, dan member lain (`temperature`, `tools`, field tak
bermodel apa pun) tetap hidup karena rewrite dilakukan pada `map[string]json.RawMessage`, bukan pada DTO.
Ini aman untuk semua vendor OpenAI-wire: array adalah bentuk yang sama artinya.

## 4. F3 — `tool_choice` paksa bernama

Vendor menjawab `auto`, `none`, dan `required`; obyek bernama khas OpenAI ditolak. Uji bentuk lain
menegaskan tidak ada padanannya: `"get_weather"` dan `{"type":"function","name":...}` ditolak validasi
gateway, `named_tool` bukan bentuk yang dikenali.

`mirrorCodeBuddyToolChoice()` membawa separuh "yang mana" lewat satu-satunya jalan yang dibaca vendor:
daftar `tools` dipersempit ke fungsi yang dituju, lalu `tool_choice` menjadi `"required"`. Kalau hanya
satu tool yang bisa dipanggil, "panggil sebuah tool" berarti "panggil tool itu". Nama fungsi yang tidak
didaftarkan pemanggil dibiarkan apa adanya — itu kesalahan klien, danconnector tidak menjawabnya dengan
mengarang tool atau memilihkan tool lain.

**Verifikasi live.** Dua tool didaftarkan, `tool_choice` memaksa yang kedua (`lookup`) → 200,
`finish_reason: tool_calls`, dan satu-satunya call adalah `lookup` dengan argumen valid.

## 5. F4 — reasoning ditagih tapi disembunyikan

`foldedChat.response()` menghapus `reasoning_content` selama `content` tidak kosong — aturan
`sseToJsonHandler.js`. Akibatnya klien non-stream ditagih `reasoning_tokens: 120` untuk teks yang tidak
pernah dilihatnya, sementara klien stream dari model yang sama melihat teks itu sepotong-sepotong.

Strip dilepas. Satu jawaban dan satu stream sekarang mengatakan hal yang sama. Case "reasoning yields to
content" pada `engine_forced_chat_test.go` diubah menjadi "reasoning stays beside content" karena kontrak
yang lama memang yang sedang diperbaiki.

## 6. F5 — nama model pada jawaban tidak bisa dikirim balik

`ClientModel()` hanya mengembalikan nama combo dan `""` untuk model langsung, sehingga fallback
(`ModelID` / `UpstreamID`) yang menang: pemanggil `codebuddy-intl/deepseek-v4.1-flash` dilayani
`deepseek-v4.1-flash`, dan mengirim nama itu balik berujung `MODEL_NOT_FOUND` ("provider nope is not in
the registry" untuk varian ber-alias). Komentar `translate_stream_openai.go:159` sudah lama menyatakan
niat sebaliknya; yang belum ada kendaraannya.

`Resolution.Requested` diisi sekali di `Relay()` — dari string yang dikirim klien, sufiks `(level)` sudah
dibelah — dan menemani anggota combo (`member.Requested = resolution.Requested`). `ClientModel()`
mengembalikannya untuk model langsung, combo tetap menang, dan `answerModel()` tidak berubah bentuknya:
fallback tetap melayani `Resolution` yang dibangun di luar relay (test, seam yang resolve satu anggota).

Aturan lama pada `TestRelay_PlainModelKeepsTheUpstreamsLabel` dibalik menjadi
`TestRelay_PlainModelNamesTheAddressedModel`; `Outcome.Model` tetap nama anggota yang benar-benar dipanggil,
jadi penagihan tidak ikut berubah.

**Verifikasi live.** Non-stream dan stream sama-sama mengembalikan `codebuddy-intl/deepseek-v4.1-flash`;
combo `pi-agent` tetap mengembalikan `pi-agent`.

## 7. F6 — noise vendor pada delta

Passthrough sengaja meneruskan byte upstream supaya field yang tidak dimodelkan skema tidak hilang. Yang
ikut lolos: `function_call:null`, `refusal:""`, `tool_calls:[]`, `extra_fields:null`,
`reasoning_content:""` pada **setiap** frame, dan `finish_reason:""` — nilai yang bukan `null` dan bukan
alasan. Klien yang menguji `if delta.tool_calls` membaca list benar-benar pada sebelas frame jawaban lima
token; klien yang menolak deprecated field membaca `function_call` sebagai objek kosong.

`sanitizeChunk()` membuang member yang tidak membawa nilai, dengan satu aturan tambahan: objek yang
seluruh membernya kosong (`{"name":"","arguments":""}`) juga dianggap tidak berkata apa-apa — bentuk
`function_call` yang vendor tulis pada frame penutup. `content` sengaja tidak disentuh, karena frame
pembuka `role` memang wajib membawa string kosong, dan tool call sungguhan tetap lewat.

Satu penjagaan datang dari test yang sudah ada: frame penutup ganda milik upstream **tetap dikirim**
dengan alasannya dinolkan, karena ia membawa usage dan delta `role` di ujung (021 F2, 034 F1/F2). Frame
hanya dibuang kalau potongannya sendiri yang mengosongkan delta itu.

**Verifikasi live.** Stream lima token: `tool_calls:[]` 0, `function_call` 0, `refusal` 0,
`extra_fields` 0, `finish_reason:""` 0, usage satu, `[DONE]` ada.

## 8. F7 — pesan validasi membocorkan tipe Go

`jsonErrorTail()` memotong 128 karakter terakhir dari pesan codec, dan bagian itu justru
`... of type []schema.ChatMessage`. Nama paket internal bukan sesuatu yang bisa dikirim balik klien, dan
tidak memberi mereka apa pun untuk diperbaiki.

Yang diambil adalah member yang salah bentuk (`json.UnmarshalTypeError.Field`):
`invalid request body: field messages holds a value of the wrong type`. Pesan syntax yang pendek tetap
diteruskan apa adanya; yang panjang diringkas jadi `the body is not valid JSON`.

## 9. Yang tetap terbuka — `stop_sequences` pada wire Anthropic

`GET /api/v1/messages` dengan `stop_sequences:["STOPHERE"]` masih mengembalikan
`A STOPHERE B` (`stop_reason: end_turn`). Ini cacat kelas sama — vendor yang mengabaikan `stop` — tapi
sengaja tidak ditutup di sini bersama F1, karena:

* `Request.stopSequences()` hanya membaca `in.Chat`, dan fold memang memotong lewat jalur itu;
* setengah menutup (memotong teks tapi melaporkan `end_turn`) akan **salah dalam cara baru**: wire
  Anthropic menuntut `stop_reason: "stop_sequence"` plus `stop_sequence` yang cocok, dan itu pekerjaan
  `claudeAnswer`/`ClaudeStreamState`, bukan tempelan pada guard;
* titik potongnya berbeda lagi: `translate_stream_claude_delta.go:48` untuk stream, dan pita terjemah
  `raw` OpenAI di `engine_translate.go:124` untuk jawaban tunggal.

Pekerjaannya: umpan `Messages.StopSequences` ke guard yang sama, dan pemetaan alasan berhenti yang benar.

## 10. Test, batas, dan peta

* **TDD.** 26 test function baru pada lima berkas: `stop_sequence_test.go` (7),
  `translate_stream_openai_sanitize_test.go` (7), `engine_translate_stop_test.go` (4),
  `codebuddy_choice_test.go` (5), `chat_decode_test.go` (3); yang bertabel menambah kasus per member
  noise dan per bentuk `stop`. Dua test kontrak lama dibalik karena memang
  mengatur perilaku yang dilaporkan salah (F4, F5). Tidak ada `t.Skip()`, tidak ada filter `-run` yang
  dibiarkan di diff.
* **Gate.** `go build ./...`, `go vet ./...`, `staticcheck` pada tiga paket terdampak, dan
  `go test -race ./internal/...` — semuanya bersih. `gofmt -l internal/` kosong.
* **AGENTS.md §1.1.** Berkas baru terbesar 201 baris (`translate_stream_openai_sanitize.go`);
  `codebuddy_body.go` sengaja tidak ditambahi logika tool_choice supaya tidak melewati 250 — ia pindah ke
  `codebuddy_choice.go`. `resolve.go` sekarang 249 baris: satu field plus dokumentasinya, dan itu sinyal
  untuk dipecah pada pekerjaan berikutnya, bukan hari ini.
* **AGENTS.md §1.9.** `SYSTEM_MAP.md` **N/A untuk topologi**: tidak ada domain boundary, service
  interaction, atau async queue yang berubah. Yang berubah adalah perilaku translasi di dalam `app-serv`
  dan satu field baru pada `dataplane.Resolution` — state intra-layanan, bukan kontrak lintas layanan.
  Kontrak `docs/CONTRACT/001-CONTRACT-API-V1.yaml` tidak disentuh.
* **Catatan operasi.** Ronde verifikasi sempat mematikan proses `app-serv` milik owner di `:9090`
  (pola `pkill` yang terlalu lebar, dan pola itu juga cocok dengan path binary hasil `go run`). Server
  sudah dinaikkan kembali di `:9090` dengan build yang sama dan menjawab 200 pada `/api/v1/models`.
