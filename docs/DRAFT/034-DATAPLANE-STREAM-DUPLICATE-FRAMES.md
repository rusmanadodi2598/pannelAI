# 034-DATAPLANE-STREAM-DUPLICATE-FRAMES.md: kenapa gateway masih mengirim chunk usage dua kali, dan di mana `finish_reason` ganda itu sebenarnya berasal

Register temuan `app-serv` dari pengujian data plane yang diminta owner atas empat model plus combo
`pi-agent`, empat mode uji (chat completion, tool, streaming, reasoning). Pass ini **tidak mengubah**
berkas `app-serv/` maupun `app-ui/` mana pun; tidak ada satu pun patch yang mendarat di sini.

|                      |                                                                                                                                                                                                                                                                                                       |
| -------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**           | **F1 CLOSED 2026-09-27** — patch `50340a8` (TDD merah-hijau, gerbang hijau); F2, F4, F5 terukur dan menunggu keputusan owner; F3 CLOSED dari 021                                                                                                                                                      |
| **Mechanism**        | AFTER (register temuan; F1 sudah diperbaiki, sisanya keputusan)                                                                                                                                                                                                                                       |
| **Scope**            | Pengujian gateway yang berjalan di `127.0.0.1:9090` atas `th-1/deepseek-v4.1-flash:free`, `opencode/mimo-v2.6-flash-free(high)`, `opencode/muse-spark-1.3-contributor-free(high)`, `opencode/space-bunny-free`, combo `pi-agent`                                                                      |
| **Permintaan owner** | "Testing request response: Chat Completion, Tool, Streaming, Reasoning" atas endpoint + kunci + daftar model itu, lalu "Cek lebih details: F2. F3, dan F5" (2026-09-27)                                                                                                                               |
| **Reference**        | register lama `docs/DRAFT/021-DATAPLANE-SSE-FRAMING.md`: §2 (F1 framing), §3 (F2 usage dobel), §4 (F3 finish dobel), §12 + §22.1 (F5 akuntansi stream); patch ketiganya 021 §23 (commit `7078835`)                                                                                                    |
| **Kaitan**           | SPEC-API §4 baris Streaming; `internal/dataplane/translate_stream_openai.go`, `internal/dataplane/translate_stream_openai_frames.go`, `internal/schema/chat_validation_parts.go`, `internal/reasoning/levels.go`, `internal/reasoning/suffix.go`; panel `app-ui/src/lib/schemas/playground-stream.ts` |
| **Tanggal**          | 2026-09-27                                                                                                                                                                                                                                                                                            |

---

## 0. Label temuan

Register ini menomori temuanya sendiri (F1–F5 di bawah), sedangkan permintaan owner menyebut **label
021**. Peta jawabannya:

| Owner menyebut                  | Ini jawaban di      | Isi singkat                                                              |
| ------------------------------- | ------------------- | ------------------------------------------------------------------------ |
| 021 F2 — usage chunk dobel      | §3, jadi F1 di sini | Masih hidup di 5/5 stream; guard `usageSent` ada di sisi yang salah      |
| 021 F3 — finish chunk dobel     | §4, jadi F2 di sini | Mekanisme gateway sudah beres; sisa duplikasi milik upstream `mimo` saja |
| 021 F5 — stream dicatat 0/0     | §5, jadi F3 di sini | **CLOSED**; 20/20 baris ronde 1 mencatat token nyata                     |
| (021 F1 — chunk tanpa `data: `) | §1 penutup          | Tidak terulang; `unframed=0` di 10/10 stream                             |

## 1. Yang diuji, dan yang bekerja

Gateway yang melayani adalah `go run ./cmd/app-serv` PID 1567836, mulai 11:30:53, build dari working
tree pada commit `d90d5a1` (`GET /version`: `0.1.0-dev`, registry `9router@39e36d3d`). Kunci yang ditempel
owner adalah satu-satunya kunci gateway yang aktif. Dua ronde: **ronde 1** 20 panggilan (4 mode × 5 ref,
satu panggilan per sel, tanpa probe), **ronde 2** 15 panggilan penelusuran akar.

**32 dari 35 panggilan menjawab 200; tiga sisanya 400 `VALIDATION_ERROR` yang memang disengaja (§6).**
Tidak ada satu pun panggilan yang menggantung: 32 panggilan yang menjawab berada di rentang 2,59 s–
20,60 s, tertinggi `th-1` tool-stream 20,6 s dan non-stream `th-1` 17,3 s. Pola lima hang pada 021 §21
tidak terulang di kedua ronde ini.

| Ref                                              | Chat                                              | Reasoning                                          | Tool                                                                    | Streaming                            |
| ------------------------------------------------ | ------------------------------------------------- | -------------------------------------------------- | ----------------------------------------------------------------------- | ------------------------------------ |
| `th-1/deepseek-v4.1-flash:free`                  | 200, `pong` + `reasoning_content` 73 ch, 38/22/60 | 200, reasoning 737 ch, 76/399/475                  | 200, `get_weather({"city":"Jakarta"})`, `finish=tool_calls`, 303/60/363 | 200, 3654 B, 17 frame, DONE benar    |
| `opencode/mimo-v2.6-flash-free(high)`            | 200, `pong`, 130/14/144                           | 200, 175/471/646                                   | 200, `tool_calls`, 197/32/229                                           | 200, 2500 B, 9 frame, **`finish=2`** |
| `opencode/muse-spark-1.3-contributor-free(high)` | 200, `pong`, 571/163/734                          | 200, reasoning 167 ch, 610/892/1502                | 200, `tool_calls`, 630/385/1015                                         | 200, 987 B, 4 frame                  |
| `opencode/space-bunny-free`                      | 200, `Pong`, 427/3/430                            | 200, 466/490/956                                   | 200, `tool_calls`, 492/38/530                                           | 200, 1630 B, 7 frame                 |
| `pi-agent` (combo)                               | 200 via muse-spark, 571/66/637                    | 200 via muse-spark, `finish=length`, 610/1059/1669 | 200 via mimo, `tool_calls`, 197/28/225                                  | 200 via space-bunny, 427/24/451      |

Yang terbukti bekerja: auth kunci gateway, resolusi prefiks node (`th-1`) **dan** alias provider tanpa
baris endpoint (`opencode/*` lewat `virtual:opencode`), pelepasan sufiks `(high)`/`:free` sebelum
resolusi (kolom `model` di kawat dan di DB menyebut id telanjang), tool calling pada kelima ref dengan
argumen yang benar, reasoning non-stream (`reasoning_content`) maupun stream (delta), dan rotasi combo
yang terdistribusi ke tiga anggotanya sepanjang empat panggilan.

**F1 register 021 (chunk tanpa `data: `) dinyatakan tidak terulang.** Pada kesepuluh stream ronde ini
setiap baris non-kosong diawali `data: ` dan stream ditutup `data: [DONE]` sebagai baris sendiri; tidak
ada satu pun byte tanpa framing (`unframed=0` di semua stream). Perbaikan `7078835` bekerja.

## 2. Sidik byte: cara membedakan frame buatan gateway dari frame upstream

Dua temuan di bawah bergantung pada satu penentuan atribusi, jadi aturannya dinyatakan lebih dulu.
`openAIFrames` meneruskan payload upstream dengan `json.Marshal` atas `object` yang
alias-nya `map[string]json.RawMessage` (`translate_wire.go:61`, dipakai `translate_stream_openai.go:148`):

- **frame yang diteruskan**: kunci tingkat atas tersusun ulang **alfabetis** (`choices, created, id,
model, object[, usage]`), dan isi di dalam `choices` tetap byte aslinya karena ia `RawMessage` —
  urutan kunci dalam bisa apa pun, mis. `index, finish_reason, delta`.
- **frame yang dibangun gateway**: `mustFrame` mem-_marshal struct_ Go (`translate_stream_openai_frames.go:34`,
  `:50`), jadi urutannya adalah urutan deklarasi field: `id, object, created, model, choices[, usage]`,
  dan delta-nya `schema.Delta{}` kosong — **tidak mungkin** memuat `"role"`.

Sidik inilah yang dipakai §3 dan §4 untuk menunjuk pembuat tiap frame.

## 3. F1 (HIGH): chunk usage dikirim dua kali di setiap stream yang meminta `include_usage`

Ini kelanjutan langsung dari 021 F2, dan **masih hidup utuh**. Ronde 1 memanggil kelima ref dengan
`stream_options.include_usage: true`; semuanya membawa **dua** frame usage dengan angka identik:

| Stream                            | Dua frame usage di kawat      |
| --------------------------------- | ----------------------------- |
| `th-1`                            | 38/38/76, lalu 38/38/76       |
| `mimo-v2.6-flash-free`            | 130/14/144, lalu 130/14/144   |
| `muse-spark-1.3-contributor-free` | 571/234/805, lalu 571/234/805 |
| `space-bunny-free`                | 427/17/444, lalu 427/17/444   |
| `pi-agent`                        | 427/24/451, lalu 427/24/451   |

Yang pertama bersidik _diteruskan_ (`choices` di depan, dan pada lane opencode memuat anggota yang tidak
dikenal schema: `"cost":"0"`, `prompt_tokens_details.audio_tokens`). Yang terakhir bersidik _dibangun
gateway_: `{"id":"gen-…","object":"chat.completion.chunk","created":…,"model":"…","choices":[],"usage":{…}}`
— persis urutan deklarasi `schema.UsageChunk` (`internal/schema/chat_response.go:122`).

**Akar.** Guard-nya ada di sisi yang salah. `usageSent` hanya diset di dalam `usageChunk()`
(`translate_stream_openai_frames.go:49`), sementara frame upstream yang **sudah** membawa usage ke klien
dibaca di `translate_stream_openai.go:158-160` hanya untuk mengisi `s.usage` — tanpa menandai bahwa
angka itu sudah lewat di kawat. `Finish()` lalu memeriksa `!s.usageSent`
(`translate_stream_openai.go:133`) dan menambah chunk keduanya. Komentar di
`translate_stream_openai_frames.go:46-47` menyatakan "Emitting it marks usageSent, so one stream carries
at most one (draft 021 F2)" — klaim itu hanya benar untuk jalur sintetis, bukan untuk jalur penerusan
yang justru dipakai keempat lane ini.

Catatan: 021 §22.1 juga menduga chunk pertama adalah nol (0/0/0) karena `"usage":null`. **Tidak terulang
ronde ini** — tidak ada satu pun chunk 0/0/0; keduanya memuat angka yang sama. Mekanisme lamanya
(`openAIUsageFromObject(nil)`) sudah tertutup, sisa cacatnya murni duplikasi.

**Dampak ke klien.** Klien yang menimpa totalnya (`usage = chunk.usage`) mendapat angka yang benar, jadi
ini tidak merusak akuntansi; yang rusak adalah klien yang **menjumlahkan** — OpenAI SDK dan pelacak
biaya yang meng-akumulasi tiap frame usage akan melipatduakan tagihan. Perbaikan yang paling murah:
tandai `usageSent = true` saat `openAIFrames` meneruskan frame yang `usage` -nya bukan nil dan bukan
objek kosong, sehingga `Finish()` tidak menambah apa pun.

**Penutupan F1 (2026-09-27, `50340a8`).** Resepi di atas diimplementasi apa adanya: `openAIFrames`
menandai `usageSent` di titik baca `usage` ketika objek yang diteruskan tidak kosong, sehingga
`Finish()` tidak menambah chunk kedua; anggota `null` maupun `{}` bukan angka di kawat dan tetap
menyerah pada guard `Finish` sendiri. TDD merah-hijau: tabel baru `TestOpenAIStream_UsageIsDeliveredOnce`
(tujuh sel — usage di frame finish, restatement upstream, usage di frame awal, klien tanpa
`include_usage`, tanpa usage sama sekali, `null`, `{}`) dan dua test framing lama yang sebelumnya
mem-pinkan kawat dobel dibalik ke aturan satu-penyampaian. Merah terbukti pada source lama (2 objek
usage pada bentuk register, 3 pada restatement — yang ekstra dibangun gateway), hijau pada seluruh
paket `dataplane` dan `go test -race ./...` lengkap; akuntansi `state.Usage()` dipinkan tidak berubah.

## 4. F2 (MEDIUM): `finish_reason` ganda pada `mimo-v2.6-flash-free` berasal dari upstream, bukan dari gateway

Hanya satu lane yang menggandakan finish, dan ia konsisten di kedua ronde: `mimo` menghasilkan
`finish=2` (`t1/m2-stream` dan `t2/m2-plainstream`), sementara `th-1`, `muse-spark`, `space-bunny` dan
combo menghasilkan `finish=1`. Dua frame itu:

```
data: {"choices":[{"index":0,"finish_reason":"stop","delta":{"role":"assistant","content":"","reasoning":null}}], …
data: {"choices":[{"index":0,"finish_reason":"stop","delta":{"role":"assistant","content":""}}, …, "usage":{…}}
```

Keduanya **bersidik frame yang diteruskan** (§2): `choices` di tingkat atas muncul lebih dulu, urutan
dalam `index, finish_reason, delta` bukan urutan struct, dan delta-nya memuat `"role":"assistant"` —
sesuatu yang tidak bisa dihasilkan `s.chunk(schema.Delta{}, …)`. Jadi upstream opencode untuk model ini
mengirim dua chunk penutup: satu yang menulangkan reasoning (`"reasoning":null`, bekas jalur fold
`b51a214`), satu yang menutup konten sambil membawa usage.

**Mekanisme 021 F3 — chunk sintetis gateway — sudah tertutup dan terverifikasi.** `openAIFrames` menandai
`finishSent` pada saat frame finish upstream diteruskan (`translate_stream_openai.go:161-170`), sehingga
`Finish()` tidak menambah yang kedua (`:121`). Tidak ada frame ketiga di mana pun.

Yang tersisa adalah pilihan desain, dan karena itu butuh keputusan, bukan otomatis diperbaiki: gateway
bisa tetap meneruskan apa adanya (aturan "forward verbatim" di `:173-175`, yang menjaga field apa pun
yang tidak dimodelkan schema), atau mengenal pola "chunk penutup tanpa konten" dan memangkasnya. Klien
yang berhenti membaca pada `finish_reason` pertama tidak terpengaruh; klien yang menghitung jumlah
jawaban atau menunggu usage **setelah** finish akan salah baca. Kontrol langsung ke
`opencode.ai/zen/v1` untuk memastikan bentuk upstream-nya tidak tercapai dari mesin ini — ia menjawab
403 `FreeTierError: OpenCode's free tier can only be used from within OpenCode`, dan percobaan lewat
klien lain tidak dilakukan. Atribusi di atas karena itu bertumpu pada sidik byte §2, bukan pada
perbandingan dua ujung.

## 5. F3 (CLOSED): 021 F5 — stream tidak lagi dicatat 0/0

Permintaan owner adalah memeriksa F5, dan F5 **beres**. Kolom `usage_records` bukan `prompt_tokens`
melainkan `tokens_in`/`tokens_out`, dan tidak punya penanda stream, jadi tiap baris dicocokkan ke angka
kawat yang sudah direkam di `/tmp/t1`. Dua puluh baris pada jam 11:35:37–11:35:52 persis sepadan dengan
dua puluh panggilan ronde 1; tak satu pun bernilai nol. Contoh empat baris stream:

| `ts`     | `model`                           | `tokens_in`/`tokens_out` | Angka di kawat | `endpoint_id`                   | `combo`    |
| -------- | --------------------------------- | ------------------------ | -------------- | ------------------------------- | ---------- |
| 11:35:37 | `deepseek-v4.1-flash:free`        | 38/38                    | 38/38/76       | `ep_0386EX3Z1D8JYP3XAHJAG8KP0A` | —          |
| 11:35:38 | `space-bunny-free`                | 427/24                   | 427/24/451     | `virtual:opencode`              | `pi-agent` |
| 11:35:39 | `mimo-v2.6-flash-free`            | 130/14                   | 130/14/144     | `virtual:opencode`              | —          |
| 11:35:41 | `muse-spark-1.3-contributor-free` | 571/234                  | 571/234/805    | `virtual:opencode`              | —          |

Baris stream `th-1` mencatat 38/38 — sama dengan chunk usage upstream di kawat, meski panggilan
non-stream model yang sama mencatat 38/22. Jadi gateway mencatat apa yang dia lihat, bukan angka yang
dia karang; selisihnya milik upstream, bukan jalur akuntansi. Jalur yang 021 §22.1 tunjuk
(`engine_stream.go:77` lalu `engine_relay.go:168-169` menimpa dengan nil) sudah tidak menimpa.

Rotasi combo ikut terverifikasi dari DB: baris `combo = pi-agent` tersebar ke tiga anggota berbeda
(muse-spark 571/66 non-stream, mimo 197/28 tool, space-bunny 427/24 stream, muse-spark 610/1059
reasoning), semuanya `endpoint_id = virtual:opencode` — keluarga OpenCode Free tetap melayani tanpa
kredensial dan tanpa baris endpoint.

## 6. F4 (LOW): panggilan yang ditolak validasi tidak menulis baris akuntansi sama sekali

Tiga probe `reasoning_effort: "none"` dijawab `400 {"code":"VALIDATION_ERROR","message":"reasoning_effort
has an unexpected value"}` dalam 0,16 s — pesan yang benar dan cepat. Tidak ada satu pun dari ketiganya
yang meninggalkan baris di `usage_records` **atau** `request_logs`: jendela 28 menit berisi 34 baris
(2 pra-tes + 32 panggilan yang melewati gerbang), persis sejumlah yang bukan 400.

Sebagai pembanding, 021 §21 mencatat 503 `NO_PROVIDER_AVAILABLE` sebagai baris `request_logs` berstatus
`error`. Jadi tolak-ukurnya tidak seragam: kegagalan routing tercatat, kegagalan skema tidak. Untuk
panel, ini berarti kartu "error rate" dan daftar request tidak akan pernah melihat request yang ditolak
validasi, termasuk percobaan klien yang salah kirim `reasoning_effort` berulang-ulang. Belum ada
keputusan apakah ini memang disengaja.

## 7. F5 (MEDIUM): `reasoning_effort` menolak kosakata yang diterima sufiks model

Satu-satunya kegagalan non-probe di kedua ronde adalah probe itu sendiri, dan ia membuka ketidaksamaan
dua pintu masuk yang melayani hal yang sama:

| Pintu                         | Kosakata yang diterima                                                                                                                         | Sumber                                       |
| ----------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------- |
| `reasoning_effort` (body)     | `minimal`, `low`, `medium`, `high`                                                                                                             | `chat_validation_parts.go:103-110`           |
| Sufiks `(level)` pada `model` | `none`, `off`, `auto`, `ultra`, angka budget, plus semua kunci `LevelToBudget` (`minimal…xhigh…max`)                                           | `reasoning/suffix.go:69-85`                  |
| Kosakata internal             | `EffortLevels` = `minimal, low, medium, high, xhigh, max`; `LevelToBudget` mengenal `none` (= budget 0) dan memetakan `none`/`off` → `minimal` | `reasoning/levels.go:25`, `:31-37`, `:49-56` |

Akibatnya `xhigh` dan `max` — dua level yang punya budget dan punya tes di paket `reasoning` — **tidak
bisa** diminta lewat field standar OpenAI, dan `none` tidak bisa diminta sama sekali lewat body, padahal
mesin mengenalnya dan `intent_test.go:40` menguji bentuk itu. Yang belum diukur: apakah sufiks
`(xhigh)`, `(max)`, `(none)` benar-benar mencapai upstream untuk keempat model ini, dan apakah
`(level)` punya pengaruh terukur pada lane opencode. Ronde 2 sempat membanding `(high)` vs tanpa sufiks
vs `(low)` pada `mimo`: `completion_tokens_details.reasoning_tokens` = 35 / 37 / 37 — perbedaan yang
tidak bisa dibaca sebagai efek level. Kesimpulan ditahan; itu pertanyaan lain, dan bukan bagian dari
empat mode yang diminta.

## 8. Cara mengulang

```bash
BASE=http://127.0.0.1:9090/api/v1
curl -sS -N -m 90 -H "Authorization: Bearer <kunci gateway>" -H "Content-Type: application/json" \
  -X POST "$BASE/chat/completions" -d '{"model":"opencode/mimo-v2.6-flash-free(high)",
  "max_tokens":512,"stream":true,"stream_options":{"include_usage":true},
  "messages":[{"role":"user","content":"Reply with one word only: pong"}]}' | tail -4
# F1: sebelum 50340a8 — dua frame usage identik, satu bersidik struct (id lebih dulu) dan satu bersidik map (choices lebih dulu)
# F1: setelah 50340a8 — satu frame usage saja (yang diteruskan); chunk sintetis tidak lagi ditambah
# F2: grep -c '"finish_reason":"stop"'  → 2 pada mimo, 1 pada tiga lane lain
```

Penanda frame buatan gateway vs milik upstream: lihat §2. Akuntansi: cocokkan `tokens_in`/`tokens_out`
`usage_records` pada jam panggilan dengan angka chunk usage di ujung kawat.

Bukti mentah: `/tmp/t1/` (ronde 1, 20 pasangan `.code`/`.t`/`.out`, termasuk `m2-stream.out` untuk
`finish=2` dan `m1…m5-stream.out` untuk chunk usage ganda) dan `/tmp/t2/` (ronde 2: `m2-plainstream.out`
memotong duplikasi finish tanpa `include_usage`; `*-toolstream.out` memverifikasi `tool_calls` yang
terakit dari delta; `m1-effort-none.out` menyimpan 400 §6). Skrip penganalisisnya ada di
`/tmp/t1/summarize.py` dan `/tmp/t2/sum.py`. Tidak ada berkas source yang disentuh pass ini.

## 9. Batas yang tidak diklaim

- F1 diukur pada empat lane berbeda bentuknya (`th-1`, opencode chat, opencode responses, combo); pola
  "dua angka identik" terbukti di kelimanya, tetapi mekanisme pendorong chunk kedua diverifikasi lewat
  kode, bukan lewat 20 stream terpisah.
- F2 tidak punya kontrol upstream langsung (§4): 403 `FreeTierError`. Atribusinya bertumpu pada sidik
  byte §2, yang merupakan argumen struktur — bukan observasi dua ujung.
- Angka `reasoning_tokens` untuk soal `(level)` datang dari tiga panggilan, tanpa pengulangan; itu
  cukup untuk menahan kesimpulan, tidak cukup untuk menyatakan sufiks diabaikan.
- Pass ini tidak menguji: endpoint `responses`, mode non-chat (gambar/audio/embedding), `tool_choice`,
  `n>1`, `parallel_tool_calls`, batas laju, dan perilaku failover kredensial.
- Gateway di `:9090` adalah proses `go run` milik owner yang berjalan sejak 11:30:53; tidak ada proses
  yang dihentikan atau dimulai ulang selama pengujian. Baris `usage_records` dan `request_logs` yang
  ditambahkan kedua ronde dibiarkan apa adanya — tidak dibersihkan.
