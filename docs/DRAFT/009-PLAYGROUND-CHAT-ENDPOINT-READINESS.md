# 009-PLAYGROUND-CHAT-ENDPOINT-READINESS.md: DRAFT PLAN Kesiapan Endpoint Playground Chat app-serv

Dokumen ini adalah **DRAFT PLAN**, bukan kontrak dan bukan implementasi. Audit dibatasi pada
`app-serv/.` sesuai permintaan. Playground Chat sendiri adalah layar di `app-ui`; pada sisi
`app-serv` ia memakai data plane `POST /api/v1/chat/completions` dan, untuk pemilihan model,
`GET /api/v1/models`. Tidak boleh dibuat route baru `/playground` di app-serv.

| | |
|---|---|
| **Status** | F1-F8 **CLOSED** dan F9 **CLOSED dengan live evidence**, 2026-09-21 (§10). Owner memilih seluruh nomor temuan |
| **Mechanism** | DURING & AFTER |
| **Tanggal audit** | 2026-09-21 |
| **Scope** | hanya `app-serv/.` |
| **Endpoint utama** | `POST /api/v1/chat/completions` |
| **Endpoint pendukung** | `GET /api/v1/models` |
| **Feature owner** | Playground Chat, panel client atas data plane |
| **Sumber aturan** | `AGENTS.md`, `SYSTEM_MAP.md`, `docs/RULLES/TDD.md`, `docs/RULLES/OWASP.md`, `docs/SPEC-API/001-SPEC-API.md`, `docs/CONTRACT/001-CONTRACT-API-V1.yaml` |

## 1. DURING & AFTER record

User menetapkan mekanisme `DURING & AFTER`.

- **AFTER:** memeriksa implementasi yang sudah ada tanpa mengubah source produksi.
- **DURING:** setiap perbaikan yang dipilih harus dimulai dari test table-driven, mempertahankan
  header `AGENTS.md`, memakai `go-playground/validator/v10`, menjaga pemisahan layer, dan
  menjalankan seluruh gate sebelum ditutup.
- Tidak ada perubahan source produksi yang dibuat oleh audit ini.
- Tidak ada kontrol UI, credential browser, atau route `/playground` yang diusulkan untuk
  ditambahkan ke `app-serv`.

Owner memilih seluruh nomor F1 sampai F9 pada 2026-09-21, sehingga mekanisme AFTER selesai dan
mode DURING dijalankan untuk setiap temuan. Perubahan produksi yang lahir dari batch ini:
`internal/schema/chat_validation.go`, `internal/schema/chat_validation_parts.go`,
`internal/handler/chat.go`, `internal/handler/datplane_errors.go`, `internal/schema/dto.go`,
`tools/openapi-gen/`, kontrak `docs/CONTRACT/001-CONTRACT-API-V1.yaml` beserta artifact
generated-nya, dan gate `scrypts/gates/contract-openapi.sh`. Bukti per temuan ada di §10.

Antislop UI tidak diterapkan pada source Go karena app-serv tidak merender interface. Aturan
kejujuran, error terstruktur, akses kontrol, dan bukti verification tetap diterapkan karena
endpoint ini menjadi backend yang dipakai interface.

## 2. Ringkasan keputusan kesiapan

**Endpoint utama sudah ada dan jalur produksi utamanya sudah terhubung**, tetapi belum dapat
dinyatakan READY terhadap seluruh mandatory karena:

1. kontrak OpenAPI chat memakai schema `MessagesRequest` untuk route OpenAI, bukan kontrak
   typed `ChatRequest` yang benar;
2. validasi request Chat masih menerima beberapa bentuk semantik yang tidak valid;
3. urutan aktual adalah decode dan validate lalu authenticate, sedangkan SPEC-API menetapkan
   auth lalu schema validation;
4. error stream sebelum frame pertama sudah lebih dulu mengunci status HTTP 200;
5. tidak ada test HTTP lengkap untuk `POST /api/v1/chat/completions` yang mencakup happy path,
   validation failure, auth failure, dan stream;
6. gate lint penuh belum hijau karena `golangci-lint` menemukan tiga masalah `gocritic` pada
   `tools/openapi-gen`; dan
7. belum ada evidence live baru untuk satu request Playground end to end dengan PostgreSQL,
   Redis, dan upstream yang benar-benar hidup.

**Keputusan audit:** `NOT READY`, lanjut sebagai DRAFT PLAN. Temuan harus dipilih owner sebelum
implementasi, mengikuti aturan AFTER bahwa item yang tidak dipilih tidak disentuh.

## 3. Jalur endpoint yang diperiksa

### 3.1 Rute dan handler

- `app-serv/internal/router/router_dataplane.go:31-37` mendaftarkan:
  `POST /api/v1/chat/completions`, `POST /api/v1/messages`, `POST /api/v1/responses`, dan
  `GET /api/v1/models` ketika `deps.Chat != nil`.
- `app-serv/internal/handler/chat.go:42-55` menyediakan handler untuk tiga wire chat.
- `app-serv/internal/handler/chat.go:57-129` menjalankan pembacaan body, decode, validation,
  authentication, relay non-stream, dan relay stream.
- `app-serv/internal/handler/chat.go:149-164` mengautentikasi lalu membaca model list dari
  resolver.
- Route data plane sengaja **tidak** session-gated. `Authorization: Bearer <gateway key>` atau
  `X-Api-Key` adalah credential yang benar untuk CLI dan server-side panel. Hal ini diperiksa
  oleh `internal/router/router_session_sweep_test.go:50-64`.

### 3.2 Schema dan validasi

- `app-serv/internal/schema/chat.go:52-66` membatasi body sampai `MaxBodyBytes` 8 MiB dan
  menolak body kosong.
- `app-serv/internal/schema/chat.go:72-85` mendefinisikan `ChatRequest` dengan struct tags
  validator untuk `model`, `messages`, temperature, top-p, dan token limit.
- `app-serv/internal/schema/chat_parts.go:24-69` menangani union message content string atau
  array parts pada boundary JSON.
- `app-serv/internal/schema/validator.go:26-39` membuat singleton
  `go-playground/validator/v10`.
- `app-serv/internal/schema/dto.go:155-159` memanggil `engine().Struct(v)` dan mengubah error
  menjadi error bahasa Inggris.
- Decode Chat sengaja memakai `json.Unmarshal`, bukan `DisallowUnknownFields`, karena komentar
  schema menyatakan wire third-party harus toleran terhadap field yang belum dimodelkan. Ini
  dicatat sebagai keputusan compatibility, bukan dianggap sebagai validasi semantic penuh.

### 3.3 Authentication dan rate limiting

- `app-serv/internal/service/chat.go:118-143` meng-hash gateway key dan mengambil key aktif dari
  repository. Jika pembacaan settings gagal, ia mengembalikan `INTERNAL_ERROR`, bukan bypass.
- `app-serv/internal/router/limiter.go:23-44` memasang Redis fixed-window rate limit pada semua
  route API selain health dan version.
- `cmd/app-serv/dataplane_wiring.go:170-191` memasang key lookup, settings reader, usage, logs,
  quota counter, dan request-id reader ke `ChatService`.
- `cmd/app-serv/dataplane_wiring.go:104-140` memasang quota gate ke selector, sehingga endpoint
  yang exhausted dilewati.

### 3.4 Routing, upstream, timeout, dan accounting

- `app-serv/internal/dataplane/engine.go:118-174` menjalankan resolve model, combo, vision,
  relay member, dan failover.
- `app-serv/internal/dataplane/engine_relay.go:37-104` memilih endpoint, menerapkan token saver,
  memanggil transport, menerjemahkan jawaban, dan mencatat health.
- `app-serv/internal/dataplane/transport_call.go:82-147` memakai context timeout untuk non-stream,
  context cancellation dan idle guard untuk stream, batas response body, serta retry policy.
- `app-serv/internal/service/chat_record.go:45-135` menulis usage, quota, dan request log. Body
  capture tetap diputuskan oleh `LogService`, bukan handler.
- `app-serv/internal/dataplane/engine_stream.go:29-136` membaca SSE bounded per event dan
  menutup stream dengan frame akhir.

## 4. Bukti verification yang sudah dijalankan

Perintah dijalankan dari `app-serv/`, kecuali gate repository yang dijalankan dari root.

| Pemeriksaan | Hasil | Catatan |
|---|---|---|
| `go build ./...` | PASS | binary dan package berhasil dibangun |
| `go vet ./...` | PASS | tidak ada temuan vet |
| `go test -count=1 ./internal/schema/... ./internal/handler/...` | PASS | package terkait schema dan handler hijau |
| `go test -count=1 ./internal/dataplane/... ./internal/service/... ./internal/router/...` | PASS | package pipeline hijau |
| `go test -race -count=1 ./...` | PASS | seluruh module test hijau |
| `gofmt -l .` | PASS | tidak ada file yang dilaporkan |
| `go-headers.sh` | PASS | 634 file Go membawa header lengkap |
| `staticcheck ./...` melalui `go-lint.sh` | PASS | staticcheck default dan integration pass |
| `golangci-lint` melalui `go-lint.sh` | **FAIL** | tiga `gocritic/stringXbytes` pada `tools/openapi-gen` |
| line-limit gate changed files | WARNING | `openapi_contract_test.go` 241, `openapi_dto_test.go` 242, `openapi_error_test.go` 243 |
| HTTP happy path Chat | GAP | belum ada test handler yang menjalankan request valid sampai upstream |
| HTTP validation Chat | GAP | belum ada test khusus `POST /api/v1/chat/completions` |
| HTTP auth Chat | GAP | extraction test ada, auth route test lengkap belum ada |
| HTTP stream Chat | GAP | tidak ada test handler untuk status/error sebelum frame dan setelah frame |
| live PostgreSQL + Redis + upstream | BELUM DIBUKTIKAN | audit ini tidak mengklaim live pass |

Temuan lint berasal dari working tree yang sudah memiliki `tools/openapi-gen` sebagai perubahan
lain. Audit ini tidak mengubah file tersebut. Karena mandatory menyebut gate lint, kegagalan tetap
harus diselesaikan atau secara eksplisit dikeluarkan dari release gate sebelum endpoint diberi
status READY.

## 5. Temuan yang memerlukan keputusan owner

### F1 HIGH: Strict validation Chat menerima bentuk semantik yang tidak valid

**Status: CLOSED 2026-09-21.**

**Fakta.** Struct tags pada Chat hanya mewajibkan `ChatMessage.Role` tidak kosong
(`internal/schema/chat.go:148-160`). `Content` tidak memiliki tag semantic pada field
`ChatMessage`, dan `ContentPart.Type` hanya `required` (`internal/schema/chat_parts.go:108-120`).
Probe table-driven terhadap `DecodeChatRequest` dan `ValidateStruct` menunjukkan beberapa bentuk
mencurigakan diterima:

- role arbitrary seperti `banana` diterima;
- content part tanpa type diterima;
- content part dengan type `banana` diterima;
- image part tanpa URL diterima;
- `reasoning_effort` arbitrary diterima;
- `response_format.type=json_schema` tanpa schema block diterima;
- stop object yang bentuknya tidak didukung tidak ditolak, lalu `StopSequences()` mengembalikan
  kosong.

Batas token, empty messages, tool type, dan tool name sudah punya sebagian aturan dan menolak
beberapa kasus.

**Risiko dan aturan.**

- `AGENTS.md` §1.4 dan §2.4 CDD: input eksternal harus typed dan divalidasi sebelum digunakan.
- `docs/RULLES/TDD.md` §2.3 dan §2.4: implementasi harus menjelaskan edge case dan mitigation
  yang generalized.
- OWASP A05/A06/A10: input tidak boleh dibiarkan mencapai translator atau provider dengan
  bentuk yang tidak diputuskan, dan failure path harus eksplisit.
- Request yang diterima tetapi field-nya diam-diam dibuang oleh translator dapat berbeda dari
  yang operator kirim, sehingga kontrak menjadi tidak jujur.

**Rencana DURING.**

1. Tulis test gagal table-driven lebih dulu untuk role, content union, content part, image URL,
   stop, response format, reasoning effort, tools, dan batas ukuran.
2. Tetapkan closed set dan cross-field rule di kontrak sebelum mengisi tag. Jangan mengarang enum
   provider; nilai yang memang diteruskan harus disebut sebagai compatibility exception.
3. Pisahkan validasi union dan cross-field ke file schema baru bila perlu agar file tetap di
   bawah 220 baris warning dan di bawah 250 baris hard limit.
4. Pertahankan unknown-field compatibility hanya bila keputusan itu tetap ditulis di kontrak
   dan ditest sebagai compatibility behavior.
5. Pastikan semua pesan error tetap bahasa Inggris dan terstruktur sebagai data-plane envelope.

**Kriteria selesai.**

- Valid request text, multimodal, tool, stop string, stop array, dan response format yang sah
  tetap diterima.
- Role unknown, content kosong, part unknown, image tanpa URL, stop shape invalid, dan
  cross-field response format ditolak dengan `VALIDATION_ERROR`.
- Test memiliki minimal 3 sampai 5 variasi per fungsi yang relevan, termasuk benign control,
  boundary, empty, negative atau malformed, dan large input.
- Tidak ada raw input yang lolos ke translator hanya karena validator tidak melihat nested union.

### F2 HIGH: Urutan authentication aktual berbeda dari kontrak pipeline

**Status: CLOSED 2026-09-21.**

**Fakta.** SPEC-API §7.15 menyatakan pipeline:

```text
auth → schema validation → bypass detection → model resolve → ...
```

Namun `internal/handler/chat.go:57-113` menjalankan `ReadBody`, decode, `ValidateStruct`, baru
memanggil `h.chat.Authenticate`. Dengan body malformed dan tanpa gateway key, caller menerima
`VALIDATION_ERROR`, bukan penolakan credential. Ini berbeda dengan urutan kontrak dan memperluas
permukaan schema oracle kepada caller yang belum terautentikasi.

**Risiko dan aturan.**

- `docs/SPEC-API/001-SPEC-API.md` §7.15: urutan auth adalah bagian dari pipeline.
- OWASP A07: authentication harus menjadi gate sebelum operasi protected dijalankan.
- OWASP A06/A10: failure behavior perlu diputuskan konsisten, bukan bergantung pada bentuk body.
- AGENTS.md §2.1 meminta auth-failure test untuk route protected oleh credential.

**Rencana DURING.**

1. Tulis table test dengan kombinasi credential valid, credential salah, credential kosong, body
   valid, body malformed, dan body besar.
2. Putuskan dan dokumentasikan urutan final. Rekomendasi: authenticate segera setelah boundary
   request minimum, sebelum decode semantic; body tidak perlu dibaca untuk caller yang ditolak.
3. Jika body harus dibaca lebih dahulu karena alasan transport, dokumentasikan pengecualian di
   SPEC-API dan test response precedence secara eksplisit.
4. Jangan mengubah auth menjadi session auth. Playground tetap memakai gateway key server-side.

**Kriteria selesai.**

- Caller tanpa key atau key invalid mendapat `401 UNAUTHORIZED` secara konsisten, termasuk saat
  body malformed.
- Caller dengan key valid dan body invalid mendapat `400 VALIDATION_ERROR`.
- Tidak ada credential atau body mentah yang ditulis ke log error.
- Kontrak dan implementation order sama.

### F3 HIGH: Stream error sebelum frame pertama terlanjur berstatus 200

**Status: CLOSED 2026-09-21.**

**Fakta.** `internal/handler/chat.go:133-143` membuat sink, mengatur SSE header, lalu memanggil
`w.WriteHeader(http.StatusOK)` sebelum `h.chat.Relay`. Jika relay gagal sebelum frame pertama,
handler memanggil `writeDataPlaneError`, tetapi status sudah committed. Probe HTTP menghasilkan:

```text
status=200
content-type=text/event-stream
body={"error":{"message":"the model field is required","type":"invalid_request_error","code":"VALIDATION_ERROR"}}
```

Dengan demikian caller menerima error JSON dalam response 200 dan content type SSE. Setelah frame
pertama memang tidak mungkin mengganti status, tetapi sebelum frame pertama masih mungkin.

**Risiko dan aturan.**

- OWASP A10: exceptional condition harus ditangani secara eksplisit dan tidak menghasilkan
  success status untuk operasi yang gagal.
- AGENTS.md §1.3: response error harus structured dan status harus merepresentasikan error.
- Antislop R-26/C-2 pada boundary API: output harus benar-benar dapat dipakai caller, bukan
  tampak berhasil tetapi berisi error.

**Rencana DURING.**

1. Tulis test table untuk: gagal sebelum frame, write failure pada frame pertama, gagal setelah
   frame, stream sukses dengan `[DONE]`, dan stream upstream malformed.
2. Ubah sink atau lifecycle agar header 200 dan SSE baru di-commit ketika frame pertama benar-benar
   ditulis. Bila relay gagal sebelum itu, gunakan data-plane error envelope normal.
3. Setelah header committed, kirim failure sesuai protokol SSE yang sudah ditetapkan, atau tutup
   stream dengan kebijakan yang didokumentasikan. Jangan mencoba menulis JSON HTTP kedua setelah
   status committed.
4. Pertahankan idle guard, context cancellation, bounded event, dan panic recovery yang sudah ada.

**Kriteria selesai.**

- Pre-frame failure tidak lagi berstatus 200.
- Stream success tetap `text/event-stream`, flush per frame, dan berakhir dengan terminal frame.
- Post-frame failure tidak menghasilkan body JSON ganda.
- Test dijalankan dengan `go test -race` dan tidak memiliki timing race.

### F4 HIGH: Test HTTP endpoint utama belum memenuhi AGENTS.md dan TDD

**Status: CLOSED 2026-09-21.**

**Fakta.** Test yang ditemukan untuk Chat hanya:

- `internal/handler/chat_auth_test.go`, yaitu table extraction header;
- `internal/router/router_dataplane_routes_test.go`, yang menguji validation untuk Responses,
  bukan Chat Completions;
- test engine relay langsung di package `internal/dataplane`, tanpa handler HTTP;
- test accounting langsung memanggil `record`, bukan request penuh.

Tidak ditemukan handler test untuk `ChatHandler.Completions`, tidak ditemukan happy-path HTTP test
untuk Chat, dan tidak ditemukan auth-failure test untuk route Chat.

**Risiko dan aturan.**

- AGENTS.md §2.1: setiap HTTP route wajib memiliki happy path, validation failure, dan auth
  failure bila protected.
- TDD.md §2.5: test harus table-driven dengan variasi nyata, bukan satu fixture statis.
- OWASP A01/A07: credential gate harus dibuktikan pada route nyata, bukan hanya helper.

**Rencana DURING.**

Tambahkan file terpisah agar line limit terjaga:

- `app-serv/internal/handler/chat_test.go`: non-stream handler happy, validation, auth, body
  limit, error mapping, usage request.
- `app-serv/internal/handler/chat_stream_test.go`: status lifecycle dan SSE behavior.
- `app-serv/internal/router/router_chat_routes_test.go`: route registration, wrong verb, no
  session requirement, request-id, rate-limit boundary.
- `app-serv/internal/schema/chat_test.go`: schema table-driven validation.

Gunakan upstream `httptest.Server` dan fixture engine yang sudah ada, tetapi wiring handler harus
nyata: key lookup, settings, resolver, selector, transport, usage/log/quota doubles. Jangan
menggunakan `&service.ChatService{}` sebagai pengganti service pada happy path.

**Kriteria selesai.**

- Chat Completions memiliki minimal happy, validation, dan auth test melalui HTTP.
- Stream memiliki pre-frame, first-frame, post-frame, success, and malformed cases.
- Wrong verb 405 dan data-plane bukan session-gated tetap dibuktikan.
- Semua test table-driven sesuai TDD, tanpa `t.Skip()`, isolated `-run`, atau ignored error.

### F5 HIGH: OpenAPI request schema Chat salah dan tidak terhubung ke DTO request

**Status: CLOSED 2026-09-21.**

**Fakta.**

- `docs/CONTRACT/001-CONTRACT-API-V1.yaml:7159-7172` memakai
  `#/components/schemas/MessagesRequest` untuk `POST /api/v1/chat/completions`.
- `app-serv/internal/handler/openapi.json:10868-10885` membawa drift yang sama.
- Typed implementation memakai `schema.ChatRequest` di `internal/schema/chat.go:72-86`.
- `MessagesRequest` di contract hanya mendeskripsikan sebagian field, `messages` sebagai
  `JsonObject`, dan `additionalProperties: true`, sehingga contract tidak memvalidasi wire
  OpenAI yang sebenarnya.
- `internal/handler/openapi_dto_test.go:30-150` mendaftarkan response DTO Chat, tetapi tidak
  mendaftarkan `ChatRequest` sebagai request schema. Test karena itu tidak menangkap mismatch ini.

**Risiko dan aturan.**

- AGENTS.md §2.4 CDD: schema-first dan contract harus selaras dengan typed struct.
- AGENTS.md §1.9: contract artifact harus disinkronkan ketika bentuk endpoint berubah.
- OWASP A08: data yang dideserialisasi dan dipublikasikan harus memiliki schema integrity.
- R-38/C-5: dokumentasi tidak boleh mengklaim shape yang berbeda dari runtime.

**Rencana DURING.**

1. Tulis contract test yang mengasertif request schema per wire, bukan hanya response schema.
2. Tambah `ChatRequest` dan nested schema yang memang dapat dipublikasikan, atau nyatakan union
   sebagai `oneOf` dengan batas yang tepat.
3. Biarkan `MessagesRequest` hanya dipakai oleh `/api/v1/messages`.
4. Update YAML sebagai source, jalankan `go run ./tools/openapi-gen`, dan jangan mengedit JSON
   generated secara manual.
5. Update `docs/CONTRACT/001-CONTRACT-API-V1.md` dan SPEC bila schema behavior berubah.
6. Jalankan route parity dan property parity test setelah generation.

**Kriteria selesai.**

- OpenAPI `/chat/completions` menunjuk schema Chat OpenAI yang benar.
- OpenAPI `/messages` tetap menunjuk schema Anthropic.
- Request properties, required fields, limits, and response content type dapat diaudit dari
  contract.
- `go run ./tools/openapi-gen -check` pass.

### F6 MEDIUM: Endpoint pendukung `GET /api/v1/models` belum memiliki readiness evidence

**Status: CLOSED 2026-09-21.**

**Fakta.** Playground membutuhkan model yang dapat dipilih atau dikirim. Route sudah terdaftar
pada `router_dataplane.go:36`, handler melakukan auth dan `h.chat.Models` pada `chat.go:149-164`,
dan resolver memiliki model list. Namun tidak ditemukan HTTP happy-path atau auth-failure test
khusus untuk `/api/v1/models`; test model yang ada dominan untuk management catalog, bukan data
plane model list.

**Risiko dan aturan.**

- Playground dapat terlihat siap tetapi tidak dapat mengisi model selector.
- AGENTS.md §2.1 tetap meminta test untuk route yang dipakai feature.
- R-27/C-4 membutuhkan empty, loading, dan error handling pada consumer. Sisi app-ui berada di
  luar scope, jadi app-serv hanya dapat membuktikan response/error contract.

**Rencana DURING.**

1. Tambahkan handler test table untuk gateway key valid, key invalid, settings require key false,
   resolver error, catalog empty, dan catalog populated.
2. Tambahkan router test untuk GET, wrong POST, dan request tanpa session.
3. Pastikan model list hanya berisi model/combo yang routable, sesuai resolver yang dipakai relay.
4. Catat bahwa loading/error/empty visual state harus dikerjakan di app-ui, bukan ditambahkan ke
   app-serv.

**Kriteria selesai.**

- Model list dengan key valid memiliki shape OpenAI `{object, data}`.
- Key invalid menghasilkan 401 structured error.
- Empty catalog menghasilkan response valid, bukan panic atau fabricated model.
- Playground dapat memakai model id yang sama dengan resolver request path.

### F7 HIGH: Mandatory lint gate belum hijau dan working tree memiliki warning line limit

**Status: CLOSED 2026-09-21.**

**Fakta.** `scrypts/gates/go-lint.sh` menjalankan vet, staticcheck, gofmt, integration build,
dan golangci-lint. Hasil audit:

- vet, staticcheck, gofmt, dan header gate pass;
- golangci-lint gagal pada:
  - `app-serv/tools/openapi-gen/main.go:99`, `stringXbytes`;
  - `app-serv/tools/openapi-gen/main_test.go:61`, `stringXbytes`;
  - `app-serv/tools/openapi-gen/main_test.go:77`, `stringXbytes`;
- changed-file warning gate melaporkan:
  - `app-serv/internal/handler/openapi_contract_test.go`: 241 baris;
  - `app-serv/internal/handler/openapi_dto_test.go`: 242 baris;
  - `app-serv/internal/handler/openapi_error_test.go`: 243 baris.

File pipeline utama yang diperiksa masih di bawah warning 220, termasuk `chat.go` 182 baris,
`schema/chat.go` 185 baris, `service/chat.go` 160 baris, `engine.go` 197 baris, dan
`engine_relay.go` 116 baris. Namun `internal/schema/messages.go` berjumlah 260 baris dan
melampaui hard limit AGENTS.md, walaupun file itu bukan perubahan yang dibuat oleh audit ini.

**Risiko dan aturan.**

- User menetapkan lint max <250 dan warning 220.
- AGENTS.md §1.1 dan §1.2 adalah mandatory.
- Release dengan gate lint merah tidak boleh disebut READY.

**Rencana DURING.**

1. Selesaikan `stringXbytes` dengan perbandingan byte yang sesuai atau keputusan lint yang
   terdokumentasi. Jangan menambahkan suppression tanpa alasan dan ticket.
2. Pecah test file yang mendekati 250 sebelum menambah test Chat.
3. Buat inventory line count seluruh file hand-authored app-serv dan pisahkan
   `internal/schema/messages.go` sebelum file itu dimodifikasi kembali.
4. Rerun `go-lint.sh`, `go-headers.sh`, dan line-limit check setelah semua test ditambah.

**Kriteria selesai.**

- `golangci-lint` pass tanpa suppression yang tidak beralasan.
- Tidak ada file yang dimodifikasi/new di atas 250 baris.
- File baru berhenti sebelum 220 bila masih memiliki lebih dari satu concern.
- Warning yang tersisa dicatat sebagai pre-existing dan tidak disentuh bila di luar scope owner.

### F8 MEDIUM: Dokumentasi status app-serv dan topology masih drift dari source

**Status: CLOSED 2026-09-21.**

**Fakta.**

- `app-serv/README.md:9` masih menyatakan P1 sedang berjalan.
- `app-serv/README.md:30` masih menyatakan repository/service endpoint, combos, data plane chat,
  usage/quota read, logs, dan settings belum terpasang, padahal route dan wiring tersebut ada.
- `SYSTEM_MAP.md` menyatakan P2 selesai dan mencatat chat, tetapi belum memberikan entry yang
  jelas untuk dependency Playground Chat sebagai panel client atas route data plane, serta
  metadata tanggalnya lebih lama daripada audit ini.
- SPEC-UI telah memutuskan auth Playground secara server-side, tetapi pekerjaan panel tersebut
  berada di luar scope app-serv. Source app-serv tidak boleh menambahkan `PANEL_PLAYGROUND_KEY`
  atau menyuntikkan key ke browser.

**Risiko dan aturan.**

- AGENTS.md §1.9 mewajibkan sync bila topology atau service interaction berubah.
- R-17, R-36, dan C-5 melarang status yang tidak didukung bukti.
- R-24 melarang navigation/control yang menunjuk route yang belum ada. Di app-serv, route data
  plane sudah ada; `/playground` tetap bukan route app-serv.

**Rencana DURING.**

1. Setelah F1 sampai F6 diputuskan, perbarui `SYSTEM_MAP.md` dengan alur panel server-side ke
   gateway key data plane, tanpa menyimpan credential di browser.
2. Perbarui `app-serv/README.md` memakai status yang dapat dibuktikan dan commands verification.
3. Jangan menduplikasi kontrak UI di app-serv. Detail route `/playground`, env panel, dan server
   forwarding tetap owner `app-ui`.
4. Jika perubahan dokumentasi menyebut angka coverage, ambil dari command yang benar-benar
   dijalankan.

**Kriteria selesai.**

- README, SYSTEM_MAP, SPEC-API, CONTRACT, dan route table tidak bertentangan.
- Boundary app-serv versus app-ui tertulis eksplisit.
- Tidak ada claim live atau numeric yang tidak punya evidence.

### F9 HIGH: Verification live Playground belum dibuktikan

**Status: CLOSED 2026-09-21 dengan live evidence yang dijalankan terhadap PostgreSQL, Redis,
dan upstream loopback nyata (§10.9).**

**Fakta.** Audit ini membuktikan compile, static tests, race tests, vet, staticcheck, headers,
dan sebagian gate. Audit ini belum menjalankan satu request nyata melalui:

```text
panel server-side caller
  → POST /api/v1/chat/completions
  → gateway key auth
  → Redis rate limit and quota counter
  → PostgreSQL usage/log row
  → guarded upstream
  → response or SSE back to caller
```

Existing repository documentation menyebut live checks pada pekerjaan sebelumnya, tetapi audit ini
membutuhkan evidence command, environment, request id, status, dan hasil row yang dapat diulang.
Tidak ada claim bahwa Playground UI sudah click-through karena `app-ui` tidak masuk scope.

**Risiko dan aturan.**

- Antislop R-35: tidak boleh menyatakan verified tanpa run/click-through evidence.
- AGENTS.md §1.6, §1.7, dan §2.1: timeout, pool, race, and route behavior harus diverifikasi.
- OWASP A01/A07/A09: auth, access control, and security logging harus terlihat pada request nyata.

**Rencana DURING & AFTER.**

1. Siapkan database integration khusus dan Redis khusus, tanpa credential masuk ke source atau log.
2. Jalankan migration, buat gateway key dengan lifecycle yang benar, dan siapkan upstream
   `httptest` atau test provider yang diizinkan egress guard.
3. Kirim request non-stream valid dengan gateway key dan catat request id, status, response shape,
   provider/model, usage row, quota increment, dan request log.
4. Kirim table request invalid, no key, wrong key, oversized body, unknown model, upstream 5xx,
   upstream timeout, dan stream failure.
5. Verifikasi client response tidak berisi provider raw body, credential, stack trace, atau path
   internal.
6. Hanya owner `app-ui` yang mencatat click-through `/playground`, unavailable key state, loading,
   empty model list, dan UI error state.

**Kriteria selesai.**

- Ada evidence live yang bisa diulang untuk non-stream dan stream.
- Usage, quota, request log, rate limiter, and key request count sesuai satu request id dan aturan
  accounting.
- Error path memiliki status dan code yang benar.
- Status READY hanya diberikan setelah F1 sampai F7 mandatory dan F9 evidence pass.

## 6. Non-findings dan batasan yang dipertahankan

Hal berikut sudah sesuai atau sengaja bukan gap app-serv:

1. **No `/playground` route in app-serv:** SPEC-API menyatakan Playground adalah client atas
   §7.15, bukan route group. Menambah route baru akan membuat contract surface yang tidak diminta.
2. **Gateway key, bukan dashboard session:** data plane route tidak session-gated karena CLI dan
   panel server-side memakai gateway key.
3. **Rate limiting:** middleware Redis berlaku pada API traffic selain health/version.
4. **Timeout dan response bounds:** transport memiliki connect/header/total timeout, stream idle
   guard, body limit, and retry policy.
5. **SSRF surface:** endpoint Chat tidak menerima URL tujuan server-side untuk fetching. Provider
   URL berasal dari registry/config dan outbound dial memakai egress guard. SSRF caller input
   tidak muncul pada endpoint ini.
6. **No idempotency claim:** chat POST sengaja diperlakukan non-idempotent; transport membatasi
   retry non-idempotent dan SPEC-API menyatakan data plane bukan idempotency surface.
7. **Structured client errors:** `writeDataPlaneError` memakai OpenAI envelope dan tidak
   meneruskan upstream raw body.
8. **Usage and quota wiring:** quota gate, Redis counter, usage, and log seams sudah dipasang di
   composition root. Gap yang tersisa adalah evidence dan test coverage, bukan bukti bahwa seam
   sama sekali tidak ada.
9. **Go stack:** `app-serv/go.mod` menggunakan Go 1.26/toolchain 1.26.5,
   `go-playground/validator/v10` v10.30.4, pgx/v5, dan go-redis/v9. Svelte, TypeScript, Zod,
dan Bun berada di app-ui dan tidak dinilai dalam scope ini.
10. **Unknown fields:** penerimaan unknown fields pada Chat adalah compatibility exception yang
    sudah disebut komentar schema. Jika owner menginginkan strict rejection, itu harus menjadi
    keputusan contract versioning, bukan perubahan diam-diam.

## 7. Urutan implementasi yang disarankan

Owner memilih nomor terlebih dahulu. Jika semua dipilih, urutan aman:

1. **F1:** schema and validation tests first, lalu implementation.
2. **F2:** settle auth precedence against SPEC-API and add auth table tests.
3. **F3:** fix stream response commit lifecycle and add failure tests.
4. **F4:** add complete Chat HTTP and router tests using real typed fixtures.
5. **F5:** correct OpenAPI YAML, generate JSON, and add request DTO contract parity.
6. **F6:** cover models dependency used by Playground.
7. **F7:** make repository mandatory gates green and split warning files before more additions.
8. **F8:** synchronize README and SYSTEM_MAP after behavior and contract are settled.
9. **F9:** run live app-serv evidence. UI click-through remains an app-ui deliverable.

Setiap task harus memiliki:

- structured analysis sebelum implementation sesuai TDD §2.3 dan OWASP §2.3 bila menyentuh
  security behavior;
- failing table-driven tests sebelum production implementation;
- file header sesuai AGENTS.md §1.2;
- line count check sebelum dan sesudah perubahan;
- no ignored errors, no `t.Skip()`, no static test-fixture hardcoding;
- compliance self-check yang menyatakan logic generalized, edge cases covered, dan gate result.

## 8. Gate dan evidence setelah perbaikan

Jalankan dari root:

```bash
cd app-serv
go test -race -count=1 ./...
go vet ./...
gofmt -l .
go run ./tools/openapi-gen -check
cd ..
bash scrypts/gates/go-headers.sh
bash scrypts/gates/go-lint.sh
bash scrypts/gates/contract-drift.sh
bash scrypts/gates/contract-openapi.sh
git diff --check
find app-serv/internal app-serv/cmd -name '*.go' \
  -not -name '*_gen.go' -not -name '*.pb.go' \
  -not -path '*/vendor/*' -print0 | xargs -0 wc -l | sort -nr | head -50
```

Untuk live integration, gunakan database dan Redis khusus test. Jangan memasukkan key, DSN,
provider credential, atau response body sensitif ke dokumen evidence. Record hanya request id,
status, machine code, bounded metrics, dan row counts yang diperlukan.

## 9. Owner selection

Owner memilih seluruh nomor temuan, `F1` sampai `F9`, pada 2026-09-21. Batch ditutup pada tanggal
yang sama; bukti per temuan ada di §10.

## 10. Catatan penutupan batch (2026-09-21)

### 10.1 F1 - validasi semantik Chat

`internal/schema/chat_validation.go` dan `internal/schema/chat_validation_parts.go` memegang
aturan yang tidak dapat diekspresikan tag struct: union content, kosa kata tertutup untuk role,
part, dan detail, bentuk `stop`, pasangan `max_tokens`/`max_completion_tokens`, serta
`response_format` yang menyatakan `json_schema` tanpa schema block. `ValidateStruct` memanggilnya
lewat type switch pada `ChatRequest`, sehingga satu jalur validasi tetap dipakai semua pemanggil.

Bukti: `internal/schema/chat_validation_test.go`, `chat_members_test.go`, `chat_body_test.go`, dan
`chat_request_rules_test.go` (table-driven, 26 kasus pada file pertama). Kontrol benign
(`text conversation`, `developer role`, `image content`, `tool declaration`, `stop array`) tetap
lolos; `unknown field compatibility` tetap diterima sesuai keputusan kompatibilitas di §6.

### 10.2 F2 - auth sebelum decode

`ChatHandler.serve` mengautentikasi lebih dahulu, lalu membaca dan memvalidasi body, sehingga
urutan kontrak `auth -> schema validation` berlaku. Body malformed dari pemanggil tanpa key
dijawab `401 UNAUTHORIZED`, bukan `VALIDATION_ERROR`.

Bukti: `internal/handler/chat_auth_precedence_test.go`, termasuk kasus body 9 MiB tanpa key
(`401`) dan dengan key valid (`400`), dan `internal/router/router_chat_boundary_test.go`.

### 10.3 F3 - SSE commit pada frame pertama

`sseSink.WriteFrame` meng-commit header dan status pada frame pertama; kegagalan sebelum frame
pertama tetap error HTTP biasa. Setelah frame pertama, stream berakhir tanpa frame terminal,
dan keputusan itu didokumentasikan di komentar `ChatHandler.stream` beserta alasannya.

Bukti: `internal/handler/chat_stream_lifecycle_test.go` (pre-frame failure, successful stream,
post-frame failure, malformed upstream, plus nol/satu/beberapa frame) dan
`internal/handler/chat_upstream_test.go` sebagai upstream double.

### 10.4 F4 - test HTTP endpoint utama

Fixture `internal/handler/chat_http_fixture_test.go` membangun `service.ChatService` nyata dengan
engine, resolver, selector, dan transport sungguhan; double hanya pada upstream dan seam
penyimpanan. Test yang ditambahkan: `chat_http_test.go` (happy, malformed JSON, role unknown,
missing key, invalid key, pre-frame failure), `chat_accounting_test.go`,
`internal/router/router_chat_routes_test.go` (wrong verb 405), dan
`internal/router/router_chat_boundary_test.go` (request id, batas rate limit, body limit di route).
Tidak ada `t.Skip()` dan tidak ada filter `-run` yang terisolasi.

### 10.5 F5 - request schema OpenAPI

`docs/CONTRACT/001-CONTRACT-API-V1.yaml` kini memakai `ChatRequest` untuk
`POST /api/v1/chat/completions` dan tetap `MessagesRequest` untuk `POST /api/v1/messages`.
Artifact `app-serv/internal/handler/openapi.json` diregenerasi oleh `app-serv/tools/openapi-gen`
(bukan diedit tangan) dan gate `scrypts/gates/contract-openapi.sh` menolak artifact yang stale;
gate itu terdaftar di `scrypts/gates/all.sh`.

Bukti: `internal/handler/openapi_chat_request_test.go` (parity request schema per wire),
`openapi_contract_test.go`, `openapi_dto_test.go`, `openapi_dto_registry_test.go`,
`openapi_error_test.go`, `openapi_helpers_test.go`, dan `tools/openapi-gen/{main,encode}_test.go`.

### 10.6 F6 - endpoint pendukung models

`internal/handler/chat_models_test.go` memaku kontrak yang dibaca Playground: key valid menerima
shape OpenAI `{object, data}`, key tidak dikenal dan tanpa key ditolak `401`, dan katalog kosong
menjawab list kosong yang valid, bukan panic atau model yang dikarang.

### 10.7 F7 - gate lint dan batas baris

`golangci-lint` bersih (0 issues) setelah `tools/openapi-gen` memakai perbandingan byte; gate
header melaporkan PASS untuk 657 file Go. Ketiga file test yang sebelumnya berada di pita
peringatan sudah dipecah: `openapi_contract_test.go` 178, `openapi_dto_test.go` 99,
`openapi_error_test.go` 132 baris. Seluruh file baru dan yang diubah pada batch ini berada di
bawah 250 baris. `internal/schema/messages.go` (260 baris) tidak disentuh batch ini dan tetap
tercatat sebagai pre-existing di luar scope owner.

### 10.8 F8 - dokumentasi dan topology

`SYSTEM_MAP.md` mencatat jalur Playground Chat sebagai panel client di atas data plane gateway
key, kontrak wire, dan artifact generated-nya; `docs/SPEC-API/001-SPEC-API.md` mencatat urutan
auth lebih dahulu serta aturan sinkronisasi kontrak; dan `app-serv/README.md` tidak lagi
menyatakan P1 sedang berjalan: statusnya kini P0 sampai P4 selesai, dengan permukaan §7 yang
benar-benar terpasang, contoh pemanggilan data plane, dan tabel cakupan test yang sudah
diverifikasi ulang terhadap tree ini.

### 10.9 F9 - live evidence

Dijalankan terhadap database throwaway dan Redis terpisah, dengan upstream `httptest` loopback,
lewat `go test -race -tags=integration -run TestPlaygroundLive ./cmd/app-serv/`. Hasil: 3 test
PASS, 0 FAIL.

| Bukti | Nilai terukur |
|---|---|
| non-stream | status 200, `usage_rows=1`, `log_rows=1` |
| token upstream | `tokens_in=7`, `tokens_out=3` |
| kuota Redis | `redis_keys=2` (counter window tersentuh) |
| `request_count` key | 1 untuk satu panggilan terautentikasi |
| stream | status 200, `bytes=346`, frame terminal `[DONE]`, `usage_rows=1` |
| tanpa key | 401 `UNAUTHORIZED` |
| key salah | 401 `UNAUTHORIZED` |
| body malformed | 400 `VALIDATION_ERROR` |
| role invalid | 400 `VALIDATION_ERROR` |
| model tidak dikenal | 400 `MODEL_NOT_FOUND` |

Satu drift kontrak tercatat, bukan diselesaikan diam-diam di sini: SPEC-API §7.15 menulis
`404 MODEL_NOT_FOUND` sementara kontrak YAML memetakan `MODEL_NOT_FOUND` ke `400`, dan §7.15
berada di luar scope batch ini.

### 10.10 Sisa yang tidak ditutup batch ini

- **SPEC-API §7.15 `404 MODEL_NOT_FOUND`** berbeda dari kontrak YAML (`400`).
- **`internal/schema/messages.go`** 260 baris, di atas batas 250 AGENTS.md §1.1, tidak disentuh
  batch ini dan tetap pre-existing.
- **UI click-through `/playground`** tetap milik `app-ui` dan di luar scope dokumen ini.
- **`skills/pannelai*/SKILL.md`** sudah ada di tree dan di-commit, tetapi belum ada di
  `origin/main` (branch lokal berada 112 commit di depan), sehingga `raw_url` masih menjawab 404
  sampai branch di-push. Ini milik draft `008-SKILL-ENDPOINT-READINESS.md` F1, bukan F1-F9 di sini.

Nomor yang tidak dipilih tidak ada; seluruh F1 sampai F9 dipilih owner dan ditutup.
