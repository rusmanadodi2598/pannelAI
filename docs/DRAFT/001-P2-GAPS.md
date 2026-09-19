# P2-GAPS.md: Sisa Gap Fase P2 (app-serv)

Dokumen kerja untuk menutup fase **P2**, bukan kontrak. Kontrak tetap
`docs/SPEC-API/001-SPEC-API.md`; peta topologi tetap `SYSTEM_MAP.md`. Setiap
keputusan yang diambil di sini harus berakhir sebagai changelog SPEC-API (dan
SYSTEM_MAP bila topologinya berubah), bukan hanya sebagai centang di tabel.

| | |
|---|---|
| **Status** | P2: seluruh permukaan rute terpasang dan terverifikasi live (§7.4, §7.6, §7.7, §7.9, §7.10, §7.11, §7.12, §7.15); 21 gap terdaftar, sebelas di antaranya temuan click-through/pass live, dua puluh sudah CLOSED (G1, G2, G3, G4, G5, G6, G7, G8, G9, G10, G11, G12, G13, G14, G15, G16, G17, G18, G19, G20); G21 terdaftar sebagai seam baru dan menjadi satu-satunya item terbuka — desain seam-nya sudah ada, keputusannya (D6) menunggu owner |
| **Dibuat** | 2026-09-19, dari hasil click-through live §7.10 (PostgreSQL 14 + Redis lokal, stub upstream loopback) |
| **Bukti terakhir** | pass G5 slice 11 2026-09-19 (Gemini STT ke stub loopback 8093: URL `:generateContent` + `key` query, audio inline base64, prompt bawaan/eksplisit, `{text}`; kontrol assemblyai tetap ditolak dengan nama); sebelum itu pass G4 (proxy loopback 8092), G5 slice 4–10 (tujuh adapter TTS ke stub), G19 (migration `000011`), dan G8 (boot polos tanpa override) |

## 1. Cara pakai

- Satu gap = satu ID. ID tidak didaur ulang setelah ditutup.
- Setiap item punya: **Bukti** (keadaan hari ini), **Kenapa** (dampak), **Keputusan**
  (kalau menunggu owner), **Pendekatan**, **Definisi selesai**.
- Item yang menunggu keputusan owner tidak dikerjakan sebagian. Jawab dulu,
  baru implementasi (OWASP §2.3: analisis sebelum mitigasi).
- Setiap penutupan item menambah satu baris di §8.

## 2. Register

| ID | Gap | Kelas | Keputusan? | Prioritas |
|---|---|---|---|---|
| G1 | Verifikasi live sisa permukaan P2 (OAuth, combo test, proxies, token-saver, budget caps, katalog model, chat/embeddings) | verifikasi | tidak | 1 |
| G2 | Dial upstream (chat + media + embeddings) tidak melewati `internal/netguard`. **CLOSED 2026-09-19** | keamanan (A01) | **ya** | 2 |
| G3 | `provider_probe.go` mendial `base_url` dari user tanpa guard. **CLOSED 2026-09-19** | keamanan (A01) | menyatu G2 | 2 |
| G4 | `settings.network.outbound_proxy_*` tidak dipakai di jalur dial. **CLOSED 2026-09-19** | kontrak §7.11 | **ya** | 4 |
| G5 | Adapter media per-provider (format gate menolak 16 provider dengan nama); sebelas format selesai, lima sisanya pindah ke G21. **CLOSED 2026-09-19** | fitur | tidak | 5 |
| G6 | Panggilan media tidak menulis usage/log; `gateway_keys.request_count` tidak pernah naik. **CLOSED 2026-09-19** | akuntansi §7.12/§7.13 | **ya** | 3 |
| G7 | Kind `video` terdaftar tanpa provider (rute menolak). **CLOSED 2026-09-19** | cakupan | tidak | 6 |
| G8 | `.env` lokal drift dari `.env.example` sehingga boot polos gagal. **CLOSED 2026-09-19** | lingkungan | tidak | 6 |
| G9 | Kegagalan request hanya tercatat sebagai `status`, tanpa kode/alasan. **CLOSED 2026-09-19** | observability §1.6 | tidak | 6 |
| G10 | Status row `SYSTEM_MAP.md` masih menarasikan P1 sebagai fase terakhir. **CLOSED 2026-09-19** | dokumen §1.9 | tidak | 6 |
| G11 | `POST /models/custom` menolak provider node kustom (katalog memegang index boot-time, bukan overlay). **CLOSED 2026-09-19** | bug wiring §7.6/§7.4 | tidak | 2 |
| G12 | Budget cap hanya bisa ditulis; `GET /quotas/{endpoint_id}` tidak pernah memuatnya sampai ada window usage. **CLOSED 2026-09-19** | kontrak §7.12 | **ya** | 3 |
| G13 | Embeddings menolak node openai-compatible (reference punya adapter `openaiCompatNode`). **CLOSED 2026-09-19** | fitur/paritas | tidak | 5 |
| G14 | State store OAuth tanpa test; `GETDEL` butuh Redis ≥ 6.2 dan gagalnya 500 tanpa log. **CLOSED 2026-09-19** | test + portabilitas | **ya** | 2 |
| G15 | Endpoint `no_auth` tanpa key tidak pernah terpilih (selector menuntut key sebelum cabang auth type). **CLOSED 2026-09-19** | bug routing §7.5 | tidak | 2 |
| G16 | Jalur media mengirim `Authorization: Bearer` kosong saat materi kredensial kosong; jalur chat mengirim tanpa header. **CLOSED 2026-09-19** | bug kredensial §8.1 | tidak | 2 |
| G17 | Kegagalan panggilan chat tidak menulis usage row (outcome kosong di jalur error). **CLOSED 2026-09-19** | akuntansi §7.12 | tidak | 3 |
| G18 | Jalur chat tidak menulis `request_logs` sama sekali; hanya media/embeddings yang menulis. **CLOSED 2026-09-19** | akuntansi §7.13 | tidak | 3 |
| G19 | `media_provider_settings` dan `proxies` dimiliki role superuser; role aplikasi `pannelai` tanpa privilege, jadi §7.10/§7.11 gagal di host ini. **CLOSED 2026-09-19** | lingkungan (aksi pemilik) | tidak | 6 |
| G20 | Penolakan media/embeddings sebelum panggilan (Prepare: provider tak dikenal, `base_url` kosong, format gate, tanpa kredensial, tanpa akun) tidak meninggalkan baris log, sedangkan jalur chat kini mencatat setiap panggilan. **CLOSED 2026-09-19** | akuntansi §7.13 | tidak | 6 |
| G21 | Lima format media butuh lebih dari satu request (AssemblyAI, AWS Polly, Edge TTS, Google TTS, Local Device) sehingga tidak bisa masuk pipeline satu-URL | fitur/seam baru | tidak | 7 |

## 3. Detail per gap

### G1: Verifikasi live sisa permukaan P2

**Bukti.** 2026-09-19 seluruh permukaan digiring terhadap stack hidup (PostgreSQL
14 + Redis lokal, stub upstream dan stub proxy loopback). Hasil per rute ada di
§8. Empat gap baru lahir dari pass ini: G11, G12, G13, G14.

**Kenapa.** Aturan spec sendiri: fase tidak boleh disebut CLOSED tanpa click-through
yang tercatat. Unit test tidak membuktikan migrasi, wiring, atau bentuk wire.

**Definisi selesai.** Setiap rute di tabel §7 dijalankan minimal sekali dengan hasil
tercatat (status + bentuk body), plus satu jalur gagal per rute. Gap baru yang
muncul menjadi ID baru di register ini. **Selesai 2026-09-19** untuk rute yang
tidak menunggu keputusan owner.

### G2: Egress guard untuk dial upstream (CLOSED 2026-09-19)

**Bukti.** `dataplane.NewHTTPClient()`, dipakai chat, embeddings, dan media, tidak
melewati `internal/netguard`; guard itu baru dipakai kandidat proxy (§7.11). Header
`internal/domain/media_provider.go` sudah dikoreksi 2026-09-19 agar tidak lagi
mengklaim dua lapis SSRF.

**Kenapa.** Base URL upstream berasal dari operator (override §7.10, endpoint §7.5,
node §7.4). Satu akun panel yang salah konfigurasi bisa membuat gateway mendial
alamat internal; kelas A01 yang sama dengan yang sudah dijaga untuk proxy.

**Keputusan owner.** (a) guard semua dial upstream dengan `EGRESS_ALLOWED_TARGETS`
(self-hosted loopback wajib di-allowlist, itu use case resmi §7.10, jadi
allowlist harus nyaman), atau (b) nyatakan upstream yang dikonfigurasi operator
sebagai trusted dan catat itu eksplisit di SPEC-API §6.

**Pendekatan kalau (a).** Guard harus injectable di level transport: test hermetic
memakai double loopback (`httptest`) yang akan ikut ditolak kalau guard dipasang
global tanpa seam. Ini alasan guard tidak disapu diam-diam saat §7.10.

**Perbaikan (2026-09-19, D1 = a).** Satu guard per proses: `egress_wiring.go`
membangun `netguard.Guard` dari `EGRESS_ALLOWED_TARGETS` plus satu `*http.Client`
di atas dialer guard itu, lalu client yang sama di-inject ke transport chat
(`TransportDeps.Client`), media/embeddings (`NewMediaTransport`), OAuth
(`NewOAuthHTTPClient`), dan probe; `buildProxies` tidak lagi membangun guard
sendiri, jadi hanya ada satu allowlist. `dataplane.NewHTTPClient` menerima
`HTTPClientDeps{Dialer}`: default-nya dialer polos, dan seam itulah yang membuat
test hermetic (`httptest` loopback) tetap jalan tanpa melemahkan produksi.
Allowlist yang salah gagal saat boot, bukan memperlebar policy diam-diam.
Keputusan dan aturannya tercatat di SPEC-API §9 butir 9 + changelog.

**Bukti penutupan (2026-09-19).** `egress_wiring_test.go` (tabel: loopback ditolak
default, `127.0.0.1/32` dan `127.0.0.0/8` lolos, kontrol benign publik lolos dan
privat ditolak, allowlist `10.0.0.0/33` menolak boot). Tabel itu merah kalau
dialer guard dilepas (`Get() reached a loopback address that is not allowlisted`).
Live (PostgreSQL 14 + Redis nyata, stub `0.0.0.0:8091`): node A
`http://127.0.0.1:8091/v1` (allowlist) menjawab chat 200 dan embeddings 200
dengan baris stub bertambah; node B `http://127.0.0.2:8091/denied` (loopback yang
alamatnya terbukti hidup lewat curl langsung, tapi tidak ada di allowlist)
menjawab 502 `UPSTREAM_ERROR` tanpa satu pun baris `/denied` di log stub. Rute
proxy test tetap benar lewat guard yang sama: privat dan loopback non-allowlist
`state=fail` dengan alasan + petunjuk allowlist, loopback allowlist
`the proxy could not be reached` (guard meloloskan, tidak ada yang mendengarkan).
Boot dengan `EGRESS_ALLOWED_TARGETS=10.0.0.0/33` berhenti dengan
`management wiring: egress wiring: netguard: "10.0.0.0/33" is neither a CIDR prefix
nor an IP address`.

**Definisi selesai.** Terpenuhi 2026-09-19: keputusan tercatat di SPEC-API §9;
implementasi + tabel test (host publik lolos, privat/loopback ditolak, loopback
yang di-allowlist lolos).

### G3: `provider_probe.go` tanpa guard (CLOSED 2026-09-19)

**Bukti.** `cmd/app-serv/provider_probe.go` mendial `base_url` yang di-supply user
saat probe node kustom (§7.4/§7.5), tanpa guard. Ditemukan saat §7.11, dilaporkan,
belum diubah.

**Kenapa.** Kelas A01 yang sama dengan G2, tapi inputnya lebih langsung (satu
request panel, satu dial ke alamat pilihan penyerang).

**Pendekatan.** Menyatu dengan G2: keputusan yang sama, guard yang sama, allowlist
yang sama. Jangan diselesaikan terpisah supaya tidak ada dua semantik egress.

**Perbaikan (2026-09-19, menyatu G2).** `newHTTPEndpointProber` menerima guard
proses dan membangun client-nya dari `dataplane.NewHTTPClient` dengan dialer
guard; `probe()` memanggil `guard.CheckHost` sebelum mengirim apa pun, sehingga
alamat yang ditolak dilaporkan sebagai penolakan beralasan alih-alih "host tidak
terjangkau", dan hook `Control` di dialer mengulang cek pada alamat yang
benar-benar didial. `provider_probe.go` dipecah: klasifikasi pindah ke
`provider_probe_call.go` supaya file pertama tetap di bawah ambang §1.1.

**Bukti penutupan (2026-09-19).** Tabel `TestProbeNode_RefusesADeniedAddress`:
node loopback ditolak default (0 request sampai ke server), loopback yang
di-allowlist diprobe (1 request), alamat privat ditolak dengan alasan. Live:
probe node A `state=ok` (baris stub 3→4), node B `the upstream address was refused:
a loopback address; add it to EGRESS_ALLOWED_TARGETS if it is your own proxy`,
node C `a private address; ...`, node D `a link-local address` (tanpa petunjuk
allowlist, benar untuk tier never-a-host), ketiganya 0 baris stub baru.

**Definisi selesai.** Terpenuhi 2026-09-19: probe memakai guard; tabel test
mencakup host privat, loopback, dan yang di-allowlist.

### G4: Proxy assignment belum dipakai di jalur dial (CLOSED 2026-09-19)

**Bukti.** `settings.network.outbound_proxy_*` disimpan, divalidasi, dan dirender
panel, tetapi tidak ada pembaca di jalur request. §7.11 berbunyi "assignment rides
on the settings", dan separuh itu belum benar.

**Keputusan owner (D2).** (a) wire egress lewat proxy terkonfigurasi (binding
per-endpoint tetap deferred, sama seperti reference), atau (b) amend SPEC-API §7.11
agar menyatakan settings belum dihormati di v1. **Diputuskan (a) 2026-09-19.**

**Perbaikan (2026-09-19).** `dataplane.HTTPClientDeps` mendapat seam `Proxy
func(*http.Request) (*url.URL, error)` yang diteruskan ke `http.Transport`, dan
`cmd/app-serv/egress_wiring.go` membangunnya: `egressProxy` membaca
`settings.network` **per request** (aturan yang sama dengan override §7.10) lalu
mengembalikan rute — `nil` berarti dial langsung. `outbound_no_proxy` dihormati
sebagai daftar koma berisi host atau sufiks domain (`*` untuk semua). Karena client
yang sama dipakai chat, media, embeddings, OAuth, dan probe, satu hook berlaku
untuk semua jalur dial.

**Bagian keamanannya (kenapa cek tujuan ada di hook, bukan di dialer).** Request
yang lewat proxy tidak pernah mendial tujuannya, jadi guard di dialer hanya melihat
alamat proxy — tanpa cek eksplisit, mengaktifkan proxy akan mematikan policy A01
untuk setiap panggilan. Karena itu hook memvalidasi tujuan lewat
`guard.CheckHost` **sebelum** mengembalikan rute. Dua aturan lain: settings yang
gagal dibaca atau URL proxy yang tidak bisa dipakai menolak request, bukan dial
langsung — melewati proxy yang operator aktifkan diam-diam adalah kegagalan yang
justru dicegah setelan ini.

**Bukti penutupan (2026-09-19, live).** PostgreSQL 14 + Redis nyata, stub chat
`0.0.0.0:8091`, proxy forwarding `0.0.0.0:8092`, `EGRESS_ALLOWED_TARGETS=127.0.0.1/32`,
DSN `.env` polos. Empat langkah berurutan dengan node allowlist yang sama: proxy
mati → chat 200, log proxy **0** baris, log stub 1; `PATCH /settings` proxy nyala →
chat 200, log proxy 1 baris (`POST http://127.0.0.1:8091/v1/chat/completions ->
127.0.0.1:8091`) dan stub tetap menerima; `outbound_no_proxy=127.0.0.1` → chat 200
langsung lagi, log proxy 0 baris; `outbound_no_proxy` dikosongkan lalu chat ke node
kedua `http://127.0.0.2:8091/v1` (hidup, di luar allowlist) → 502 `UPSTREAM_ERROR`
dengan **0** baris di log proxy dan **0** baris baru di log stub. Test
`egress_proxy_test.go` mengunci keempatnya plus tabel `proxyRoute` (sembilan baris:
mati, tanpa URL, sufiks bertitik, batas label `notexample.com`, `*`, daftar
berkomentar, URL tanpa host) dan dua mutasi diukur merah: hook dilepas → baris
"arrives at it" gagal, cek tujuan dilepas → baris A01 gagal. Artefak dibersihkan
(dua node, dua endpoint, gateway key, baris usage/log, baris settings `network`,
hash panel NULL; baseline `usage=2 logs=1 settings=1` pulih).

**Definisi selesai.** Terpenuhi 2026-09-19: pilihan (a) tercatat di SPEC-API §7.11,
dan satu panggilan chat terbukti keluar lewat proxy lewat log stub proxy.

### G5: Adapter media per-provider (CLOSED 2026-09-19)

**Bukti.** 16 provider mendeklarasikan format media non-OpenAI
(`assemblyai, aws-polly, cartesia, coqui, deepgram, edge-tts, elevenlabs, gemini`
untuk tts+stt, lalu `google-tts, inworld, local-device, minimax` (+`minimax-cn`),
`nvidia-tts, playht, tortoise`); sebelum adapter ini semuanya ditolak
`PROVIDER_NOT_ROUTABLE` dengan nama formatnya (terverifikasi live untuk elevenlabs,
2026-09-19). Sebelas format sudah di-port, lima masih berada di gate tersebut
(kini terdaftar sebagai G21).

**Kenapa.** Ini keputusan sadar (menolak dengan nama lebih baik daripada mengirim
bentuk yang salah), tapi selama belum di-port, provider itu tidak bisa dipakai.

**Pendekatan.** Port builder per provider dari reference, satu per satu, urut
berdasarkan kebutuhan operator. Tiap adapter: bentuk payload dari registry +
konversi jawaban ke envelope §7.10.

**Deepgram STT (adapter selesai 2026-09-19).** Deepgram adalah adapter pertama
karena reference mendefinisikan satu POST tanpa polling: bytes audio mentah,
`model` + `smart_format=true` + `punctuate=true` (atau `detect_language=true`),
`Authorization: Token`, lalu nested `results.channels[0].alternatives[0].transcript`
dinormalisasi ke `{text}`. `TranscriptionForm` membawa MIME part; MIME audio yang
valid dipakai setelah `mime.ParseMediaType`, MIME non-audio/malformed tidak disalin
ke outbound header dan jatuh ke pemetaan extension atau `application/octet-stream`.
Alias provider ikut resolve (`dg/nova-2`).

**NVIDIA NIM TTS (adapter selesai 2026-09-19).** Reference mendefinisikan satu
POST JSON tanpa polling: `{input:{text}, voice, model}`, `Authorization: Bearer`,
dan bytes WAV. Voice default provider adalah `default` (bukan default OpenAI
`alloy`), dan response route selalu dilabeli `wav` — termasuk bentuk JSON base64 —
agar bytes WAV tidak salah dilabeli mp3. Model harus dinamai (`nvidia/fastpitch` atau
`nvidia/tacotron2`); registry sudah mendeklarasikan keduanya.

**Cartesia TTS (adapter selesai 2026-09-19).** Reference mendefinisikan satu POST
JSON tanpa polling: `{model_id, transcript, voice?, output_format}` dengan
`Cartesia-Version: 2024-06-10` dan `X-API-Key`; voice dihilangkan jika kosong,
`output_format` tetap MP3 (`bit_rate=128000`, `sample_rate=44100`), dan response
bytes dilabeli `mp3` termasuk bentuk JSON base64. Registry mendeklarasikan
`sonic-2`/`sonic-3`; model tetap harus dinamai (`cartesia/sonic-2`).
AWS Polly belum diambil karena reference tidak punya builder SigV4.

**Tujuh adapter TTS (slice 4–10, selesai 2026-09-19).** Satu pass mengerjakan
seluruh sisa provider TTS yang bisa diselesaikan dalam satu request, sesuai
keputusan owner "TTS saja (7)":

- **ElevenLabs** — `{text, model_id, voice_settings}`, kredensial `xi-api-key`,
  voice sebagai segmen path terakhir; voice wajib (tanpa voice ditolak
  `VALIDATION_ERROR`).
- **MiniMax dan MiniMax CN** — `{model, text, stream:false, language_boost,
  output_format:"hex", voice_setting, audio_setting}`, `Authorization: Bearer`,
  jawaban hex di-decode; `base_resp.status_code` bukan nol menjadi penolakan
  upstream meski HTTP 200.
- **Inworld** — `{text, voiceId, modelId, audioConfig}`, `Authorization: Basic`
  (payload base64 mentah seperti registry), jawaban `audioContent` base64.
- **PlayHT** — `{text, voice, voice_engine, output_format, speed}`; satu materi
  kredensial `userId:apiKey` dipecah menjadi `X-USER-ID` + `Authorization: Bearer`;
  `Accept: audio/mpeg` ikut payload.
- **Coqui dan Tortoise** — self-hosted tanpa kredensial, `{text, voice?}`
  (`speaker_id` hanya bila diisi), jawaban bytes WAV; tortoise default voice
  `random`.
- **Gemini TTS** — model sebagai segmen path sebelum `:generateContent`,
  kredensial sebagai query `key`, prompt dibangun dari input (+ bahasa), jawaban
  base64 PCM dibungkus header RIFF/WAVE 44 byte; 200 tanpa audio (mis. finish
  reason `SAFETY`) menjadi penolakan upstream.

Seam yang ditambahkan agar tujuh itu masuk: `MediaPath` (sufiks path untuk provider
yang menaruh model/voice di URL), cabang kredensial `basic`/`playht` di
`media_credential.go`, dan `mediaAnswerReader` — pembaca jawaban per provider yang
dipanggil `MediaCallService.Perform`, sehingga satu adapter bisa mengubah bytes
jawaban sebelum amplop §7.10 dibangun.

**Gemini STT (slice 11, adapter selesai 2026-09-19).** Sisa G5 yang bukan G21: satu
`generateContent` dengan audio inline, di permukaan yang sama dengan adapter TTS
Gemini. Model jadi segmen path sebelum `:generateContent`, kredensial tetap di
query `key` (ditempatkan `MediaTarget`, §8.1), audio dikirim inline sebagai
`inline_data` base64 dengan `mime_type` hasil resolver yang sama dengan Deepgram
(sehingga content type non-audio tidak pernah ikut), dan prompt diambil dari field
`prompt` pemanggil atau instruksi bawaan reference, ditambah ` Language: <lang>.`
bila bahasa diisi. Jawaban menggabungkan part teks kandidat pertama menjadi
`{text}`; jawaban tanpa kandidat adalah transkrip kosong, bukan penolakan — diam
memang bertranskrip kosong — sedangkan jawaban 200 yang tidak terbaca tetap
`INTERNAL_ERROR`. Field JSON mengikuti ejaan reference untuk permukaan STT
(`inline_data`/`mime_type`; keduanya diterima pemetaan JSON Gemini, dan port
mempertahankan yang diukur). Lima format sisa pindah ke G21, sehingga tidak ada
lagi pekerjaan adapter biasa di G5.

**Definisi selesai (per adapter).** Format ditranslate, tabel test 3 sampai 5 kasus
termasuk kontrol benign, dan satu panggilan live ke stub yang meniru provider itu.
Untuk Deepgram terpenuhi 2026-09-19: `media_deepgram_test.go` mengunci dua bentuk
request (language eksplisit dan auto-detection), auth/query/raw bytes/MIME/alias,
empat variasi MIME, empat bentuk respons (nested, kosong, malformed, passthrough),
dan bukti live ke stub dicatat pada §8. Untuk NVIDIA terpenuhi 2026-09-19:
`media_nvidia_test.go` mengunci default/explicit voice, nested body, bearer, WAV
output, dan kontrol OpenAI; live bytes + `?response_format=json` ke stub membuktikan
body, auth, `audio/wav`, base64 `format:wav`. Untuk Cartesia terpenuhi 2026-09-19:
`media_cartesia_test.go` mengunci voice omitted/explicit, nested body, API key,
version header, fixed MP3 output, and OpenAI boundary; live bytes +
`?response_format=json` membuktikan `X-API-Key`, `Cartesia-Version`, body, `audio/mp3`,
and base64 `format:mp3`. Untuk tujuh adapter slice 4–10 terpenuhi 2026-09-19: satu
file test per provider (`media_elevenlabs_test.go`, `media_minimax_test.go`,
`media_inworld_test.go`, `media_playht_test.go`, `media_local_tts_test.go`,
`media_gemini_tts_test.go`) plus tabel entri registry di
`media_call_entries_test.go`, dan bukti live ke stub dicatat pada §8. Untuk Gemini
STT terpenuhi 2026-09-19: `media_gemini_stt_test.go` mengunci tiga bentuk prompt/MIME
(bawaan + MIME dari ekstensi, prompt pemanggil + bahasa + MIME audio ter-parse, MIME
non-audio + suffix tak dikenal), penggabungan part jawaban, transkrip kosong yang
tetap served, jawaban rusak yang tetap `INTERNAL_ERROR`, tabel gate per kind, dan
kontrol benign jalur OpenAI multipart yang tidak berubah; bukti live ke stub dicatat
pada §8.

### G6: Akuntansi panggilan media (CLOSED 2026-09-19)

**Bukti.** Setelah 9 panggilan media pada 2026-09-19: `usage_records` tidak
bertambah (2 baris lama), `request_logs` tidak bertambah (1 baris lama), dan
`gateway_keys.request_count` tidak pernah di-increment siapa pun. Grep tidak
menemukan UPDATE untuk kolom itu, untuk chat maupun media. `MediaCall.Outcome()`
sudah ada tetapi tidak dikonsumsi pemanggil mana pun.

**Kenapa.** Layar Usage (§7.12) dan Logs (§7.13) panel kosong untuk media; kolom
request_count di daftar gateway key selalu 0.

**Keputusan owner (D3).** (a) media menulis usage row (tokens 0; untuk search ada
`cost_per_query` di registry) dan request log; (b) `request_count` di-increment
middleware untuk setiap panggilan data plane terautentikasi; (c) combo test §7.7
tetap tanpa usage row, keputusan itu sudah final dan tidak dibuka lagi di sini.
**Diputuskan (b) 2026-09-19:** usage + log + request_count.

**Perbaikan (2026-09-19).** Counter: `RecordUse` ditambahkan ke kontrak
`repository.GatewayKeyRepository` (satu UPDATE `request_count = request_count + 1,
last_used_at = $1`, baris hilang → `ErrGatewayKeyNotFound`), dan seam
`KeyUseRecorder` dipanggil dari **satu** titik — `ChatService.Authenticate`, choke
point yang sama untuk chat, models, media, dan embeddings — jadi satu panggilan
terautentikasi menambah tepat satu, aritmetikanya di database (bukan
read-modify-write), dan kegagalan tulis tidak menolak request yang sudah lolos
autentikasi. Akuntansi media: `dataPlaneRecorder` menulis satu usage row + satu
log row di bawah request id router (`requestIDOrNew` memakai ULID baru bila
context tidak membawa id), dipakai `MediaCallService.Perform` dan
`EmbeddingsService.perform`; klasifikasi satu-exit (transport error, upstream
non-2xx via `UpstreamRejected`, atau sukses) membangun outcome **sebelum**
panggilan, sehingga baris error tetap membawa provider/endpoint/model. Search
mencatat `cost_per_query` registry dan model = provider id lewat
`accountingModel()` (rute search tidak menyebut model, kolomnya wajib). Teks error
log `CODE: message` tanpa cause terbungkus — URL media bisa membawa kredensial
sebagai query param (§8.1) — dan body log media sengaja kosong (reference mencatat
detail hanya untuk chat). `MediaCall.Model` (string klien, hanya di-echo) dihapus.
Test: `key_use_test.go`, `media_record_test.go` + stub-nya, dan
`TestIntegration_RecordUse`.

**Bukti penutupan (2026-09-19, live).** PostgreSQL 14 + Redis 6.0.16 host, stub
upstream loopback 8091, gateway 127.0.0.1:8099 (`EGRESS_ALLOWED_TARGETS=127.0.0.1/32`).
Baseline `usage=2 logs=1 reqcount=0`; satu endpoint `openai` (override §7.10 ke
stub) dan satu endpoint `brave-search`, satu gateway key. Empat panggilan media:
speech 200 (usage+1, log+1, reqcount+1), speech ke jalur 500 (usage+1 `status:error`
`UPSTREAM_ERROR`, log+1 `UPSTREAM_ERROR: stub refused`, reqcount+1), embeddings 200
(+1/+1/+1), search 200 (+1/+1/+1, `cost_usd` 0.005 = registry `cost_per_query`,
model `brave-search` = provider id). Setiap pasangan usage+log memakai request id
yang sama, tokens 0, latency sama, key id terisi; kontrol chat 502 menaikkan
reqcount (+1) tanpa baris media. Artefak dibersihkan: baseline pulih (`usage=2
logs=1`, `gateway_keys=0`, `upstream_endpoints=0`, `upstream_keys=0`,
`provider_nodes=0`, `media_provider_settings=0`, `panel_auth.password_hash` NULL).
Pass ini melahirkan G17 dan G18 (jalur chat).

**Definisi selesai.** Terpenuhi 2026-09-19: satu panggilan media menghasilkan satu
usage row + satu request log dengan request id yang sama, dan `request_count` key
naik 1 per panggilan terautentikasi (terbukti live untuk tts, embeddings, dan
search, termasuk jalur gagal).

### G7: Kind `video` tanpa provider (CLOSED 2026-09-19)

**Bukti.** Rute terdaftar dan menolak dengan nama; tidak ada provider di registry
yang mendeklarasikan `video` (reference juga tidak mendefinisikannya, paritas).

**Pendekatan.** Biarkan terdokumentasi (default), atau cari provider yang
mendeklarasikan kind + tulis adapter (masuk G5).

**Definisi selesai.** Keputusan tercatat di changelog SPEC-API §7.10.
**Terpenuhi 2026-09-19:** changelog §7.10 (commit `1f00860`) sudah mencatat
`POST /videos/generations` terdaftar tetapi menolak `PROVIDER_NOT_ROUTABLE` karena
tidak ada provider yang mendeklarasikan `video`, dengan alasan paritas reference
(`videoConfig` dikenali, tidak ada yang mendefinisikannya). Default "biarkan
terdokumentasi" yang dipilih; kalau kelak ada provider ber-kind `video`, adapter
masuk G5 dan bukan gap ini.

### G8: `.env` lokal drift dari `.env.example` (CLOSED 2026-09-19)

**Bukti.** `.env.example` sudah benar dan lengkap; `.env` lokal tidak:
`ENCRYPTION_KEY` 41 byte (harus tepat 32), `REDIS_PASSWORD` basi (`AUTH failed`),
`PANEL_BOOTSTRAP_PASSWORD` kosong, dan blok `PUBLIC_BASE_URL` tidak ada. Boot polos
gagal di validasi config.

**Pendekatan.** Aksi pemilik: sinkronkan `.env` dengan `.env.example` + secret asli
(baca `requirepass` dari `/etc/redis/redis.conf` saat itu juga; jangan disimpan).
Resep override sementara ada di `reference_appserv_local_env.md`.

**Perbaikan (2026-09-19, disetujui owner).** `.env` dibangun ulang **dari**
`.env.example` sehingga hanya empat baris nilai yang berbeda — `POSTGRES_DSN`,
`ENCRYPTION_KEY` (32 byte acak baru; aman karena DB tidak memegang baris sealed
sama sekali saat itu: `upstream_keys=0`, `proxies=0`), `REDIS_PASSWORD` (dibaca
dari `/etc/redis/redis.conf` pada saat penulisan), dan `PANEL_BOOTSTRAP_PASSWORD`
tetap kosong (bukan penghalang boot; owner yang mengisi). Sisanya, termasuk blok
`PUBLIC_BASE_URL`, kini identik dengan template.

**Bukti penutupan (2026-09-19).** Boot polos (hanya `.env`, tanpa override
environment apa pun) menjawab `GET /api/v1/health` dengan
`{"status":"ok","checks":{"postgres":"ok","redis":"ok"}}`; `redis-cli PING` dengan
`REDISCLI_AUTH` dari `.env` menjawab `PONG`; diff `.env` vs `.env.example` hanya
delapan baris (empat pasang nilai). Pass G19 dan G5 slice 4–10 sesudahnya memakai
DSN `.env` yang sama tanpa satu pun override.

**Definisi selesai.** Terpenuhi 2026-09-19: `./app-serv` boot bersih tanpa override
environment.

### G9: Logging kegagalan request (CLOSED 2026-09-19)

**Bukti.** Kegagalan media tampil di log sebagai `request handled … "status":502`
plus request id, tanpa kode error atau alasan. Klien menerima envelope yang benar;
operator tidak menerima "kenapa". Click-through G1 menambah satu bukti yang lebih
tajam: `GET /oauth/callback` gagal 500 karena `GETDEL` tidak dikenal Redis lokal,
dan log hanya mencatat `"status":500` dalam 1 ms, tanpa satu baris pun yang
menyebut Redis (lihat G14).

**Pendekatan.** Log satu baris dengan `code` (dan `error`) di jalur
`writeDataPlaneError`/handler, memakai request id yang sudah ada.

**Perbaikan.** Baris akses yang sudah ada (`request handled`, dengan request id
dan status) yang diperkaya — bukan baris kedua, supaya satu baris menjawab "apa"
dan "kenapa". Seam-nya `schema.ErrorCodeRecorder` (satu metode), di-set oleh
`schema.WriteError` (envelope manajemen) dan `handler.writeDataPlaneError`
(envelope data plane), diimplementasikan `router.responseRecorder`, dan
diteruskan `router.statusRecorder`. Penerusan itu eksplisit karena Go hanya
mempromosikan metode milik interface yang di-embed (`http.ResponseWriter`), bukan
metode tambahan milik nilai konkretnya — tanpa itu assertion handler gagal
diam-diam dan kode berhenti tercatat. `writeEnvelope` (404/405 milik mux) ikut
mencatat kodenya. Field `code` hanya muncul saat request gagal, jadi
kehadirannya berarti "request ini gagal". **Pesan tidak pernah dicatat** — hanya
kode — karena pesan kegagalan data plane bisa mengutip teks upstream yang membawa
kredensial (aturan yang sama dengan G18); jalur panic tetap mencatat id + nilai
panic lewat `recoverer`.

**Bukti live (2026-09-19).** Binary dari tree yang sama, `.env` + override run
(tanpa stub, tanpa artefak DB): `GET /api/v1/nope` → 404 baris
`code=NOT_FOUND` (envelope mux), `POST /api/v1/chat/completions` tanpa key → 401
`code=UNAUTHORIZED`, `POST /api/v1/images/generations` tanpa key → 401
`code=UNAUTHORIZED` (keduanya lewat `writeDataPlaneError`), dan dua permintaan
sukses (`GET /api/v1/health`, `GET /api/v1/version`) tidak membawa field `code`.
Baseline tidak berubah (`usage=2 logs=1 keys=0 endpoints=0 nodes=0 settings=1
auth_null=true`); `panel_auth.password_hash` di-null kembali setelah server
berhenti.

**Definisi selesai.** Tiap kegagalan request menghasilkan satu baris log berisi
kode + request id; ada test yang mengunci bentuknya. **Terpenuhi 2026-09-19**:
`router_errorlog_test.go` mengunci empat jalur (error handler, rute tak dikenal,
verb salah, penolakan data plane) plus kasus negatif (request sukses tanpa field
`code`), changelog SPEC-API dicatat, dan bukti live di atas.

### G10: Narasi P2 di `SYSTEM_MAP.md` (CLOSED 2026-09-19)

**Bukti.** Status row masih menyebut P1 sebagai fase terakhir yang CLOSED; §3.6 dan
§5 sudah memuat media dan `proxies`/`media_provider_settings`.

**Pendekatan.** Update sekali saat P2 benar-benar ditutup, bersama narasi fase.
Jangan dipecah per slice agar tidak churn.

**Perbaikan (2026-09-19).** Status row kini menyebut **P2 CLOSED** dengan daftar
bukti live-nya (OAuth round-trip, combo test, proxy pools + guard egress,
token-saver, budget caps, katalog model, media §7.10, embeddings lewat node,
akuntansi usage/log), plus apa yang **tidak** ikut tertutup supaya "CLOSED" tidak
dibaca sebagai "tidak ada sisa": lima format media sisa (G21, setelah Gemini STT
ditutup di slice 11) dan baris ini sendiri. Tiga bagian lain ikut diperbarui karena berubah sejak
P1: §3.6 menambah paragraf seam adapter (empat seam + `MediaPath` +
`media_credential.go`, sebelas format teradaptasi, batas G21), §4a menambah narasi
migrasi P2 `000009`-`000011` termasuk aturan kepemilikannya, dan batas domain §1
menyebut `000011` untuk kedua tabel P2.

**Definisi selesai.** Terpenuhi 2026-09-19: status row menyebut P2 CLOSED dengan
daftar buktinya, dan sisa yang terbuka disebut eksplisit di baris yang sama.

### G11: Model kustom untuk provider node ditolak (CLOSED 2026-09-19)

**Bukti.** `POST /api/v1/models/custom` dengan `provider_id` sebuah node
(`openai-compatible-0385R7Z3Z8FJM23V16F9EBGFGC`, dibuat lewat §7.4 dan terbukti
routable) menjawab
`400 VALIDATION_ERROR "unknown provider_id: openai-compatible-…"`. Penyebabnya di
composition root: `management_wiring.go` membangun `ModelCatalogService` dengan
`Index: index` (registry embedded), sementara provider, endpoint, node, dan prober
mendapat `runtimeIndex` (overlay node). `ModelCatalogServiceDeps.Index` bertipe
`*registry.Index`, jadi overlay tidak bisa diserahkan tanpa seam. Efek lanjutan
yang terukur: combo dengan ref `stub/stub-model` ditolak
`"model reference does not resolve to a model, combo, or alias"`, dan
`GET /api/v1/models` (262 model) tidak memuat satu pun model node.

**Kenapa.** Komentar fungsi `AddCustom` sendiri menjanjikan "including a synthesized
provider node", dan panel butuh baris katalog untuk menampilkan model sebuah node.
Tanpa itu, node hanya bisa dipakai kalau klien menebak string `prefix/model`.

**Perbaikan (2026-09-19).** `ModelCatalogServiceDeps.Index` menjadi interface
sempit `CatalogIndex` (`Provider`, `All`) dan wiring menyerahkan `runtimeIndex`.
Dua test baru mengunci keduanya:
`TestModelCatalogService_RegistersAModelUnderACustomNode` dan
`TestComboService_AcceptsARefUnderACustomNode`. Diverifikasi live pada binary yang
sama: `POST /models/custom` untuk node 201, katalog memuat barisnya, dan
`POST /combos` dengan ref node 201.

**Definisi selesai.** Terpenuhi: 201 untuk model di bawah node, baris katalog dan
ref combo terlihat, dan test mengunci keduanya.

### G12: Budget cap tidak terbaca kembali (CLOSED 2026-09-19)

**Bukti.** `PUT /api/v1/quotas/{endpoint_id}` menjawab 200
`{"monthly_cost_usd":"5.00000000","monthly_tokens":1000}` dan baris `quota_caps`
benar-benar ada di PostgreSQL, tetapi `GET /api/v1/quotas/{endpoint_id}` langsung
sesudahnya menjawab `{"data":[]}` karena rute baca hanya membaca `quota_windows`
yang terisi oleh flush counter usage. Tidak ada rute mana pun yang membaca
`quota_caps` kembali.

**Kenapa.** Panel tidak bisa menampilkan budget yang sudah disetel (form selalu
kosong sampai ada traffic), padahal §7.12 menjadikan cap sebagai kontrol P2.

**Keputusan owner (D5).** (a) rute baca menggabungkan cap ke window (sintesis
window `limit` walaupun `used` 0), (b) tambah pembacaan cap di
`GET /quotas/{endpoint_id}` lewat field terpisah, atau (c) amend SPEC-API §7.12
agar cap dinyatakan tulis-saja di v1. **Diputuskan (b) 2026-09-19:** field cap
terpisah.

**Perbaikan (2026-09-19).** `GET /api/v1/quotas/{endpoint_id}` sekarang menjawab
`{endpoint_id, cap, data}`: `cap` berisi cap tersimpan (`QuotaCapResponse` yang
sama dengan jawaban PUT) atau `null` eksplisit saat belum ada cap — `null` dipilih
daripada field hilang supaya "belum ada cap" bisa dibedakan dari jawaban yang
lupa memuatnya. `QuotaService.GetCap` yang tadinya tanpa pemanggil produksi kini
dipakai handler; rute koleksi `GET /api/v1/quotas` tetap windows-only (cap milik
satu endpoint) dan bentuknya tidak berubah. Test `quota_test.go` mengunci PUT→GET
(cap yang sama terbaca walau `data` kosong), kasus `"cap":null` dengan window
tetap tampil, rute koleksi tanpa field cap, dan jalur validasi (cost negatif 400
tanpa menulis apa pun).

**Bukti penutupan (2026-09-19, live).** Boot dengan DSN `.env` (role aplikasi,
tanpa workaround G19 — tabel quota milik `pannelai`), login, satu endpoint
`openai`: `PUT /quotas/{id}` `{"monthly_cost_usd":"5","monthly_tokens":1000}` → 200
`{"monthly_cost_usd":"5.00000000","monthly_tokens":1000,…}`;
`GET /quotas/{id}` → 200 `{"endpoint_id":…,"cap":{"monthly_cost_usd":"5.00000000",
"monthly_tokens":1000,…},"data":[]}` — gejala G12 (`{"data":[]}` tanpa cap)
hilang; `GET /quotas` → 200 `{"data":[]}` tetap tanpa field cap. Artefak
dibersihkan (endpoint, baris `quota_caps`, hash panel NULL; baseline `usage=2
logs=1` pulih).

**Definisi selesai.** Terpenuhi 2026-09-19: satu PUT lalu GET menampilkan cap yang
sama, dengan test yang mengunci bentuknya.

### G13: Embeddings menolak node openai-compatible (CLOSED 2026-09-19)

**Bukti.** `POST /api/v1/embeddings` dengan model `stub/stub-embed` (node §7.4 yang
sama, endpoint + key aktif) menjawab
`400 PROVIDER_NOT_ROUTABLE "provider openai-compatible-… does not offer embeddings"`.
`EmbeddingsService.mediaConfig` menuntut blok `media.embedding` di registry,
sedangkan provider hasil sintesis node tidak punya blok media. Reference justru
punya adapter khusus untuk kasus ini:
`open-sse/handlers/embeddingProviders/openaiCompatNode.js`, yang membangun
`<baseUrl>/embeddings` dari `baseUrl` node (dan menerima `custom-embedding-*`).

**Kenapa.** Operator yang menambahkan node OpenAI-compatible mengharapkan
embeddings ikut jalan; sekarang satu-satunya rute embeddings yang bisa dipakai
adalah provider registry yang mendeklarasikan blok media.

**Pendekatan.** Cabang sintesis di `mediaConfig`: provider `Custom` memakai
`Transport.BaseURL` + `/embeddings` sebagai target, dengan payload OpenAI yang
sudah dipakai adapter registry.

**Perbaikan (2026-09-19).** `mediaConfig` memanggil `nodeEmbeddingMedia` baru:
node `Custom` berformat OpenAI (chat atau Responses) mendapat blok sintesis
(`BaseURL` = base node dengan aturan trim reference, `AuthType` api_key,
`Format` openai), node Anthropic-compatible tetap ditolak. Blok sintesis
mendeklarasikan formatnya secara eksplisit, dan `IsGeminiEmbedding` kini
mendahulukan format yang dideklarasikan, supaya node yang menunjuk host
OpenAI-compatible Gemini tidak dibaca sebagai protokol native. Dua test baru:
tabel lima bentuk node plus satu kontrol benign.

**Definisi selesai.** Terpenuhi: satu panggilan embeddings lewat node menjawab
200 dengan bentuk OpenAI; tabel test lima kasus. Kasus node tanpa kredensial
diukur live (endpoint `api_key` menolak dibuat tanpa key; endpoint `no_auth`
tanpa key ditolak selector tanpa dial) dan melahirkan G15/G16.

### G15: Endpoint `no_auth` tanpa key tidak pernah terpilih (CLOSED 2026-09-19)

**Bukti.** `POST /api/v1/endpoints` dengan `auth_type: no_auth` dan tanpa key
menjawab 201 `"key_count":0,"available":true`, tetapi
`POST /api/v1/embeddings` lewat provider itu (node openai-compatible) menjawab
`503 NO_PROVIDER_AVAILABLE "every upstream endpoint for provider … is unavailable
or has no usable key"`, tanpa satu pun dial ke stub. `Selector.Select` memanggil
`endpoint.NextKey(now)` dan `continue` bila tidak ada key sehat, sedangkan cabang
`no_auth` di `credential()` (yang memang mengabaikan key) baru dicapai setelah
itu. Efeknya endpoint `no_auth` hanya bisa dipakai kalau operator menyimpan key
placeholder, dan key itu tidak pernah dikirim.

**Kenapa.** Auth type `no_auth` adalah jalur resmi (§7.5) untuk provider tanpa
kredensial; sekarang endpoint seperti itu mati sampai diisi rahasia palsu.

**Pendekatan.** Di `Select`, cabang `no_auth` dipilih sebelum pengecekan key
(kredensial kosong, `Selection.Key` zero), dengan jalur health write yang tidak
menuntut key id. Alternatifnya: `no_auth` tetap butuh key, dan itu dinyatakan di
SPEC-API.

**Perbaikan (2026-09-19).** `Select` hanya menuntut key untuk endpoint ber-auth
key (nilai `Available` sudah menolak endpoint berkey yang tidak punya key sehat),
dan `RecordSuccess`/`RecordFailure` menjadi no-op untuk selection tanpa key
(`selection_health.go`, file baru hasil pemisahan agar `selection.go` tetap di
bawah ambang baris). Dua test: pemilihan tanpa key + health no-op, dan kontrol
benign bahwa endpoint `no_auth` yang disabled tetap ditolak.

**Definisi selesai.** Terpenuhi: live pada binary yang sama, endpoint `no_auth`
tanpa key menjawab 200 untuk chat maupun embeddings; health write tidak menulis
baris key.

### G16: Jalur media mengirim bearer kosong (CLOSED 2026-09-19)

**Bukti.** Pada endpoint `no_auth` yang diberi key placeholder (satu-satunya cara
G15 membiarkannya terpilih), `POST /api/v1/chat/completions` sampai ke stub
**tanpa** header `Authorization` (`ApplyAuth` berhenti saat materi kredensial
kosong), sedangkan `POST /api/v1/embeddings` dengan endpoint yang sama sampai
dengan `Authorization: Bearer` kosong. `MediaTarget` menulis header itu di cabang
default tanpa memeriksa secret; reference pun mengirim
`Bearer ${creds.apiKey || creds.accessToken}` sehingga paritas bukan pembenaran.

**Kenapa.** Satu akun menyajikan dua aturan kredensial yang berbeda di dua jalur,
dan header kosong adalah kredensial salah bentuk yang dikirim ke upstream yang
justru tidak butuh kredensial.

**Pendekatan.** Samakan dengan aturan `ApplyAuth`: materi kredensial kosong
berarti tidak ada header yang dikirim (cabang default dan bearer), sementara
cabang query-param tetap menolak keras. Test tabel di level `MediaTarget`.

**Perbaikan (2026-09-19).** Cabang `bearer` dan default di `MediaTarget` hanya
menulis header saat secret tidak kosong; cabang `key`/`query` tetap menolak.
Tabel tujuh baris mengunci ketiga cabang dalam dua keadaan kredensial, dan dua
barisnya gagal sebelum perbaikan (`Bearer ` kosong).

**Definisi selesai.** Terpenuhi: live pada binary yang sama, chat dan embeddings
lewat endpoint `no_auth` tanpa key sama-sama 200 dan stub tidak melihat header
`Authorization` sama sekali; kontrol berkey tetap melihat `Bearer` berisi key.

### G14: State store OAuth tanpa test dan tanpa lantai versi (CLOSED 2026-09-19)

**Bukti.** `internal/repository/redis/oauth_state.go` memakai `GETDEL` dan tidak
punya file test sama sekali (tidak ada `oauth_state_test.go`, tidak ada test
bertag `integration` untuk store ini). Redis lokal host adalah 6.0.16, sedangkan
`GETDEL` baru ada di 6.2: `GET /oauth/callback` menjawab
`500 INTERNAL_ERROR "an unexpected error occurred"` dalam 1 ms, sebelum satu pun
dial ke provider, dan tanpa baris log yang menyebut Redis (lihat G9). Setelah
`GETDEL` diemulasi oleh shim RESP loopback, alur callback penuh (token exchange,
userinfo, sealing, single-use state) lulus.

**Kenapa.** Satu perintah Redis yang tidak dikenal mematikan seluruh rute §7.4
callback, tanpa test yang menangkapnya dan tanpa log yang menjelaskannya. Stack
memang mem-pin Redis latest, jadi kode tidak salah terhadap kontrak, tapi
lantainya tidak dinyatakan dan tidak dijaga.

**Keputusan owner (D4).** (a) pertahankan `GETDEL`, tambah test store dan catat
lantai Redis ≥ 6.2 di SPEC-API §6, atau (b) ganti `Take` ke `EVAL` skrip Lua
`GET`+`DEL` yang tetap atomik dan jalan di 6.0 ke atas. **Diputuskan (b)
2026-09-19:** perbaiki agar jalan di 6.0 ke atas, tanpa amend lantai versi.

**Definisi selesai.** Test store (stage, take, replay ditolak, TTL) hijau terhadap
Redis nyata; pilihan lantai versi tercatat; kegagalan store menghasilkan satu
baris log berkode.

**Perbaikan (2026-09-19).** `Take` memakai `redis.NewScript` (EVALSHA dengan
fallback EVAL) berisi `GET` + `DEL` dalam satu langkah atomik, jadi tidak ada
lantai versi di atas 6.0. Empat test store baru bertag `integration` mengunci
stage + TTL, take yang menghapus key, replay sebagai jawaban `ok=false` tanpa
error, stage ulang yang ditolak `ErrStateAlreadyStaged` tanpa menimpa payload
pertama, dan kedaluwarsa TTL. Diuji terhadap Redis 6.0.16 host tanpa shim: merah
`unknown command getdel` sebelum perbaikan, hijau sesudahnya; `GET /oauth/callback`
dengan state asing menjawab 302 ke panel dengan `oauth_error` (sebelumnya 500).
Baris log berkode untuk kegagalan store tetap bagian G9 (P2.8).

### G17: Kegagalan panggilan chat tidak menulis usage row (CLOSED 2026-09-19)

**Bukti.** Pass G6 (2026-09-19): satu `POST /chat/completions` dengan gateway key
nyata menjawab 502 `UPSTREAM_ERROR` (upstream asli menolak kredensial endpoint
stub), `gateway_keys.request_count` naik 1, tetapi `usage_records` tidak
bertambah satu baris pun. Penyebabnya di kode: `Engine.Relay` mengembalikan
`Outcome{}` di setiap jalur error (identity yang sudah dibangun `relayOnce`
dibuang), sedangkan `domain.NewUsageRecord` menolak input tanpa
`provider_id`/`model` — jadi `ChatService.record` gagal validasi dan errornya
diabaikan (`_, _ =`). Panggilan yang dilayani menulis baris normal.

**Kenapa.** Kegagalan chat tidak terlihat di layar Usage (§7.12) padahal kolom
`status`/`error_code` ada untuk itu, dan jalur media sudah mencatat kegagalan
(baris `tts-1` `status:error` `UPSTREAM_ERROR` di pass yang sama) — dua jalur data
plane dengan aturan berbeda. Reference mencatat kegagalan chat sebagai request
detail `status: "error"`.

**Pendekatan.** Pakai pola satu-exit yang sama dengan media: bangun outcome
(provider/endpoint/model/combo) sebelum dial dan kembalikan bersama error, agar
`ChatService.record` menerima identitas yang cukup untuk menulis baris error.

**Perbaikan.** `relayOnce` — dipindah ke `internal/dataplane/engine_relay.go`
karena `engine.go` menembus batas 250 baris (§1.1) — membangun `Outcome` sebelum
langkah pertama yang bisa gagal dan mengembalikannya di setiap jalan keluar, dengan
`latency_ms` diisi lewat `elapsedMS` (termasuk jalur gagal). `Engine.Relay`
menyimpan `lastOutcome` dari anggota terakhir yang benar-benar dicoba, sehingga combo
yang habis melaporkan identity anggota yang errornya diterima klien. Jalur fusion
mendapat aturan yang sama: `fanOut` mengembalikan identity anggota pertama yang
gagal (error yang dilaporkan), `relayFusion` meneruskan outcome `relayOnce`/`judge`
alih-alih `Outcome{}`. `ChatService.record` menulis baris usage hanya ketika
identity ada (provider + model): panggilan yang ditolak sebelum ada percobaan
(provider tak dikenal) tidak menulis baris usage, dan pelewatan itu eksplisit —
bukan validasi agregat yang gagal lalu ditelan, pola yang menyembunyikan gap ini.

**Bukti live (2026-09-19).** Stub chat loopback (200 + usage 5/2 untuk model apa
pun kecuali `broken`, yang menjawab 500 dengan pesan `Incorrect API key provided:
sk-g18-should-not-be-stored`), node `openai-compatible` + endpoint + key, tiga
gateway key, `EGRESS_ALLOWED_TARGETS=127.0.0.1/32`. Satu `POST /chat/completions`
model `g18node/broken` menjawab 502 `UPSTREAM_ERROR` dan meninggalkan **satu baris
`usage_records`** `status:error`, `error_code:UPSTREAM_ERROR`,
provider/endpoint/model terisi (`openai-compatible-…`, `ep_…`, `broken`), token 0,
latency 7 ms — request id `0385RT4FG14Y44H76TZW2NB1YJ` yang sama dengan baris
log-nya. Panggilan sukses menulis baris `success` token 5/2, dan `POST` model
`nosuchprovider/model` menjawab 400 `MODEL_NOT_FOUND` dengan baris log **tanpa**
baris usage (batas sebelum percobaan). Baseline dipulihkan (`usage=2 logs=1`).

**Definisi selesai.** Satu panggilan chat yang gagal di upstream menghasilkan satu
usage row `status: error` dengan `error_code` dan identity terisi; test mengunci
bentuknya; changelog SPEC-API §7.12. **Terpenuhi 2026-09-19**: tiga test baru
(`TestRelay_FailureKeepsTheAttemptedIdentity`,
`TestRelay_ExhaustedComboReportsTheLastAttemptedMember`,
`TestRelay_FusionPanelFailureKeepsTheFirstAttemptedIdentity` +
`TestRelay_FusionJudgeFailureKeepsTheJudgesIdentity`), plus changelog §7.12 di
`docs/SPEC-API/001-SPEC-API.md`.

### G18: Jalur chat tidak menulis `request_logs` (CLOSED 2026-09-19)

**Bukti.** Pass G6 (2026-09-19): satu-satunya penulis `domain.RequestLogInput` di
tree non-test adalah `dataPlaneRecorder` (media + embeddings). Panggilan chat —
sukses maupun gagal — tidak pernah menghasilkan baris `request_logs`;
`LogService.Record` sudah lengkap (selalu menulis baris, hanya body yang
dipotong/dibuang sesuai `settings.logging.request_capture_enabled`), tetapi tidak
ada pemanggil dari jalur chat.

**Kenapa.** Layar Logs (§7.13) tidak pernah menampilkan lalu lintas chat — justru
lalu lintas utama gateway — dan `GET /usage/records/{request_id}` tidak bisa
menjoin log untuk request chat. Reference mencatat request detail chat (dengan
body saat capture aktif).

**Pendekatan.** Panggil `LogService.Record` dari jalur chat dengan request id
router yang sama seperti usage row-nya; keputusan body tetap satu tempat di
`LogService.Record`, jadi capture tetap satu aturan.

**Perbaikan.** `ChatService` menerima seam `Logs RequestLogRecorder`, dipasang di
`cmd/app-serv/dataplane_wiring.go` dengan `LogService` yang sama seperti jalur
media/embeddings. `record` — dipindah ke `internal/service/chat_record.go` agar
`chat.go` tetap di bawah batas baris — menulis satu baris `request_logs` per
panggilan chat: request id router yang sama dengan baris usage, identity, status,
latency, `request_body` dari `in.Raw`, `response_body` dari `outcome.Body`, dan
`error` = **kode saja**. Itu berbeda dari baris media (`CODE: message`) dengan
alasan keamanan: pesan upstream chat mengutip kembali kredensial yang dikirim
(terukur: `Incorrect API key provided: sk-…`), sedangkan pesan jalur media adalah
pesan tetap milik gateway. Body diserahkan apa adanya supaya `LogService.Record`
yang memutuskan capture/truncation, dan panggilan yang ditolak sebelum pipeline
berjalan tetap meninggalkan baris log.

**Bukti live (2026-09-19).** Lima panggilan chat dari pass yang sama: 200 sukses,
502 `UPSTREAM_ERROR`, 200 passthrough, 400 `MODEL_NOT_FOUND`, dan satu panggilan
dengan capture aktif. Setiap request id di log server punya baris `request_logs`
sendiri, dan baris sukses/gagal berbagi request id dengan baris usage-nya
(`0385RT4F97CD2B8S4JMXSKGEWW`, `0385RT4FG14Y44H76TZW2NB1YJ`,
`0385RT4FKMS66HGQ841KG2A78M`). Baris gagal menyimpan `error` = `UPSTREAM_ERROR`
saja; pencarian sentinel `sk-g18` di `request_logs` (error + kedua body) dan
`usage_records` menjawab 0 baris, padahal respons 502 ke klien memuat teks
upstream itu. Capture default mati: `request_body`/`response_body` kosong; setelah
`PATCH /settings {"logging":{"request_capture_enabled":true}}`, satu panggilan
menyimpan body permintaan (`{"model": "g18node/stub-ok", …}`) dan body jawaban
utuh di bawah request id yang sama. Baris setting capture dihapus saat cleanup
(baseline `settings` 1 baris), dan `gateway_keys.request_count` naik 1 per
panggilan terautentikasi (3/1/1 untuk tiga key).

**Definisi selesai.** Satu panggilan chat menghasilkan satu baris `request_logs`
dengan request id yang sama dengan usage row-nya; body hanya tersimpan saat
capture aktif; test mengunci bentuknya; changelog SPEC-API §7.13. **Terpenuhi
2026-09-19**: tiga test baru (`TestChatService_RecordWritesTheAccountingPairOnFailure`,
`TestChatService_RecordRefusedBeforeAnAttemptLogsWithoutUsage`,
`TestChatService_RecordWritesTheServedAnswer`), plus changelog §7.13 di
`docs/SPEC-API/001-SPEC-API.md`.

### G19: Tabel P2 dimiliki role superuser, role aplikasi tanpa privilege (CLOSED 2026-09-19)

**Bukti.** Pass G6 (2026-09-19): menjalankan gateway dengan DSN `.env` (role
`pannelai`) gagal `ERROR: permission denied for table media_provider_settings`
saat `PATCH /media-providers/{provider_id}`. `has_table_privilege` untuk role itu:
`media_provider_settings SELECT=false INSERT=false UPDATE=false`, `proxies
SELECT=false INSERT=false`, sementara tabel lain dimiliki `pannelai`. Kedua tabel
dibuat migration P2 tetapi dimiliki `rusmanadodi` (superuser), jadi override §7.10
dan rute §7.11 tidak bisa dipakai dengan kredensial aplikasi. Pass diselesaikan
dengan DSN superuser — workaround, bukan keadaan yang boleh dibiarkan.

**Akar masalah.** Migration berjalan sebagai role yang mem-boot gateway. Ledger
menunjukkannya: `000001`–`000008` diterapkan 2026-09-18 (role aplikasi, tabel
`pannelai`-owned), sedangkan `000009`/`000010` diterapkan 2026-09-19 12:18 oleh
boot ber-DSN superuser (pass G1) — dan tabelnya ikut menjadi milik superuser.
Jadi ini bukan cacat satu host, melainkan aturan yang belum ada: **setiap tabel
milik role aplikasi**, apa pun role yang menjalankan migration.

**Perbaikan (2026-09-19, disetujui owner).** `migrations/000011_p2_table_ownership.up.sql`
mengembalikan kepemilikan kedua tabel ke role aplikasi, dengan `gateway_keys`
(tabel migration pertama) sebagai jangkar — itu menyatakan satu aturan, bukan
menyebut nama role yang berbeda per deployment. Karena role yang tidak memiliki
tabel tidak bisa memindahkan kepemilikannya, penolakan ditangkap dan dilaporkan
sebagai **warning** berisi statement yang harus dijalankan, bukan kegagalan boot:
kegagalan di titik ini akan mengubah dua rute yang rusak menjadi gateway yang tidak
pernah hidup. Agar warning itu terlihat, `migrations.openDB` menyetel `OnNotice`
pgx ke `slog` (severity NOTICE/INFO/DEBUG turun ke Debug, sisanya Warn) — pgx
membuang notice saat `OnNotice` nil, dan warning yang tidak dibaca sama saja dengan
tidak ada. Down migration-nya sengaja kosong: keadaan yang digantikannya adalah
defect itu sendiri.

**Bukti penutupan (2026-09-19).** Test `migrations/ownership_test.go` (bertag
`integration`) mengunci invariannya untuk **seluruh** schema: tidak ada tabel
`public` yang pemiliknya berbeda dari pemilik `gateway_keys`. Diskriminatif dan
diukur: dengan `proxies` dipindah ke `rusmanadodi` di `pannelai_test`, test merah
menyebut `table proxies is owned by rusmanadodi`; setelah baris ledger `000011`
dihapus dan `Apply` dijalankan role aplikasi, warning muncul lengkap dengan resep
`ALTER TABLE proxies OWNER TO pannelai` dan boot tetap jalan; dijalankan role
ber-privilege, migration memindahkan tabel **dan indeksnya** tanpa warning lalu test
hijau. Live: boot ber-DSN superuser menerapkan `000011`
(`migration applied file=000011_p2_table_ownership.up.sql`, tanpa warning), kedua
tabel + pkey menjadi `pannelai`, lalu boot berikutnya memakai DSN `.env` polos:
`PATCH /media-providers/openai {"kind":"tts","base_url":…}` → **200**
`base_url_source:"override"` dan terbaca kembali lewat `GET`, `POST /proxies` →
**201**, `GET /proxies` → 200 memuat barisnya, `DELETE /proxies/{id}` → **204**
(sebelumnya 500 `permission denied`). Cleanup memakai role aplikasi — termasuk
`delete from media_provider_settings` langsung, yang tadinya mustahil.

**Definisi selesai.** Terpenuhi 2026-09-19: `PATCH /media-providers/{provider_id}`
dan rute §7.11 berhasil dengan DSN `.env` tanpa perubahan environment, dan
aturan kepemilikannya kini dijaga test + migration, bukan ALTER manual di satu host.

### G20: Penolakan media/embeddings sebelum panggilan tidak meninggalkan baris log (CLOSED 2026-09-19)

**Bukti.** Temuan pass G17/G18 (2026-09-19): jalur chat kini mencatat **setiap**
panggilan — termasuk `MODEL_NOT_FOUND` yang ditolak sebelum pipeline berjalan
(terbukti live: baris log tanpa baris usage) — sedangkan `MediaCallService.Prepare`
dan `EmbeddingsService` menulis baris hanya setelah `Perform` dijalankan. Jadi
penolakan provider tak dikenal, `base_url` kosong, format gate, kredensial kosong,
atau `NO_PROVIDER_AVAILABLE` di §7.10/§7.9 tidak pernah muncul di layar Logs,
padahal klien menerima error dan request id-nya tercatat di log server.

**Kenapa.** Layar Logs (§7.13) adalah tempat operator melihat lalu lintas yang
ditolak; jalur media/embeddings yang ditolak sebelum dial tidak terlihat di sana,
dan aturannya berbeda dari jalur chat untuk kelas kegagalan yang sama.

**Pendekatan.** Panggil recorder di pembungkus handler (atau di jalur `Prepare`)
sehingga satu panggilan yang ditolak pun menulis baris log — dengan identity
sebisanya (provider/model) dan kode error saja. Baris usage tetap hanya untuk
panggilan yang mencapai percobaan, sama seperti chat.

**Perbaikan.** Penulis terpisah dipilih: `dataPlaneRecorder.refuse` menulis satu
baris `request_logs` (tanpa baris usage — aggregate menuntut provider dan model,
aturan yang sama dengan penolakan pra-pipeline chat) dengan `error` = kode saja,
latensi 0, dan body kosong. Di jalur media, `MediaCallService.prepareForCall`
membungkus `Prepare` dan mencatat penolakannya; identity disusun
`refusalOutcome` dari string `provider/model` yang dikirim klien (string tanpa
provider tidak diberi nama — sama seperti chat yang tidak mengarang identity),
dan dua penolakan awal `Search` (`searchProvider`, `Block`) mencatat lewat
`mediaRefusalOutcome`/outcome kosong. Di jalur embeddings, fase resolusi dipindah
ke `resolveCall` (`embeddings_resolve.go`) yang **membangun identity sebelum
langkah pertama yang bisa gagal** — aturan yang sama dengan `relayOnce` G17 —
sehingga `Embed` punya satu titik penolakan; `EmbeddingsServiceDeps.Engine`
diganti dua port sempit (`ModelResolver`, `MediaRouter`) agar use case tidak
membawa engine ke setiap test. Penolakan di handler (body invalid, 401) tidak
mencatat — batas yang sama dengan jalur chat. `Voices` juga tidak mencatat: rute
itu membaca katalog registry dan tidak pernah dial (keputusan §7.10 yang sudah
ada).

**Bukti live (2026-09-19).** Binary dari tree yang sama, `.env` + override run,
tanpa stub: login + satu gateway key, lalu empat penolakan — `POST
/images/generations` model `nope/whatever` → 400 `MODEL_NOT_FOUND`, `POST
/embeddings` model `nope/embed` → 400 `MODEL_NOT_FOUND`, `POST /audio/speech`
model `elevenlabs/voice` → 400 `PROVIDER_NOT_ROUTABLE` (format gate), `POST
/search` provider `nope` → 400 `MODEL_NOT_FOUND`. Keempatnya meninggalkan
**satu** baris `request_logs` dengan request id yang sama dengan baris akses
server, `status:error`, `error` = kode saja, latensi 0, body kosong; identity
terisi sejauh resolusi sampai (`nope/whatever`, `elevenlabs/voice`,
`nope/nope`), dan penolakan embeddings provider tak dikenal tidak beridentity
karena resolver gagal sebelum provider diketahui. Nol baris usage untuk keempatnya
(total usage tetap 2). Dua kontrol negatif: panggilan media tanpa key (401) dan
penolakan validasi body di handler (400) **tidak** meninggalkan baris
(`control_rows=0`); `gateway_keys.request_count` = 5 untuk empat penolakan +
satu panggilan yang terautentikasi lalu ditolak handler. Baseline dipulihkan
(`usage=2 logs=1 keys=0 endpoints=0 upkeys=0 nodes=0 caps=0 settings=1
auth_null=true`), artefak `/tmp/g20_*` dihapus.

**Definisi selesai.** Satu `POST /images/generations` (atau rute §7.10 lain) dengan
provider tak dikenal meninggalkan satu baris `request_logs` dengan kode error dan
request id yang sama dengan log server; test mengunci bentuknya; changelog §7.13.
**Terpenuhi 2026-09-19**: `media_refusal_test.go` (lima kelas penolakan + kontrol
benign + dua penolakan `Search`) dan `embeddings_refusal_test.go` (empat kelas +
kontrol served) mengunci bentuknya, changelog SPEC-API dicatat, dan bukti live di
atas.

### G21: Lima format media butuh lebih dari satu request

**Bukti.** Sisa G5 yang tidak bisa masuk pipeline media sekarang — satu URL, satu
request, satu jawaban — dan sejak slice 11 (Gemini STT, 2026-09-19) satu-satunya
format media yang masih ditolak gate. Kelimanya berasal dari daftar format yang sama
(`registry.yaml`), dan alasan masing-masing terlihat di reference
(`open-sse/handlers/`):

- **AssemblyAI** (`stt`) — upload → submit → poll sampai 120 detik
  (`sttCore.js:57`): tiga request dengan loop dan batas waktu.
- **AWS Polly** (`tts`) — `base_url` memuat placeholder `{region}` dan
  `auth_header: aws-sigv4`; reference **tidak punya implementasinya sama sekali**
  (`FORMAT_HANDLERS` di `genericFormats.js` tidak memuatnya), jadi port berarti
  menulis penandatanganan SigV4 dari dokumentasi AWS.
- **Edge TTS** (`tts`) — mengambil `bing.com/translator`, mem-parse token dari HTML
  + cookie (cache ~5 menit), lalu POST SSML ke endpoint Bing
  (`edgeTts.js:getToken`).
- **Google TTS** (`tts`) — mengambil `translate.google.com`, mem-parse `f.sid`/`bl`
  dari HTML (cache ~11 menit), lalu memanggil RPC `batchexecute` (`googleTts.js`).
- **Local Device** (`tts`) — menjalankan binary host: `say` di macOS, SAPI lewat
  PowerShell di Windows, `ffmpeg` untuk konversi (`localDevice.js`) — bukan HTTP
  sama sekali.

Yang lebih tajam dari kelimanya: `base_url` tiga provider (edge-tts, google-tts,
local-device) di registry berisi **penanda**, bukan URL (`base_url: edge-tts`),
jadi model "satu target URL" tidak berlaku untuk mereka tanpa aturan baru.

**Kenapa jadi gap terpisah.** Keputusan owner 2026-09-19 memisahkan pekerjaan ini
dari G5 ("TTS saja (7)" untuk slice berikutnya, sisanya "gap baru + seam
multi-step"): mencampurnya akan membuat satu adapter memaksa seam baru
(polling/scraping/proses host) di tengah slice yang lain, dan format gate tetap
menolak kelimanya **dengan nama** selama itu — perilaku yang diinginkan §6.

**Pendekatan (belum dikerjakan).** Seam yang dibutuhkan berbeda per kelas, jadi
jangan dipaksakan satu bentuk: (a) adapter berurutan dengan anggaran waktu untuk
AssemblyAI (upload → submit → poll), (b) penandatanganan SigV4 sebagai util untuk
AWS Polly, (c) pengambil token pihak ketiga dengan cache TTL untuk Edge/Google TTS
— yang juga menuntut `base_url` berhenti menjadi URL, dan (d) eksekusi proses host
untuk Local Device, yang perlu keputusan tersendiri karena menjalankan binary dari
proses gateway memperbesar permukaan ancaman (OWASP) dan tidak berlaku di container.

**Desain (2026-09-19).** Keempat kelas sudah punya desain seam yang bisa direview:
`docs/DRAFT/002-G21-SEAM-DESIGN.md` — per kelas berisi bukti reference (file:line),
bentuk seam, kontrak/registry yang berubah, risiko + temuan keamanan (mis. argument
injection di Local Device, ToS/kerapuhan scraping di Edge/Google), test yang akan
mengunci, dan pertanyaan keputusan. Ringkasan keputusannya ada di dokumen itu §5
(A1–A3, B1–B2, C1–C3, D1) dan tercatat sebagai D6 di §4 berkas ini. Tidak ada kode
yang ditulis sebelum keputusan itu dijawab.

**Definisi selesai.** Tiap provider punya seam + adapter-nya, tabel test termasuk
kontrol benign, dan satu bukti live ke stub yang meniru alurnya; atau keputusan
tertulis bahwa provider itu tidak didukung di v1, dengan SPEC-API §7.10 menyebut
alasannya.

## 4. Keputusan yang menunggu owner

| # | Pertanyaan | Pilihan | Jawaban owner | Dampak kalau ditunda |
|---|---|---|---|---|
| D1 | Guard egress upstream? (G2/G3) | (a) guard + allowlist, (b) trusted + catat | **(a) guard + allowlist** (2026-09-19) | dial internal tetap mungkin dari konfigurasi salah |
| D2 | Proxy settings dihormati di jalur dial? (G4) | (a) wire, (b) amend spec | **(a) wire** (2026-09-19) | §7.11 separuh benar |
| D3 | Apa yang dicatat panggilan media? (G6) | (a) usage+log, (b) +request_count, (c) combo test tetap tanpa row | **(b) usage + log + request_count** (2026-09-19) | layar Usage/Logs panel kosong untuk media |
| D4 | Lantai Redis untuk state OAuth? (G14) | (a) `GETDEL` + catat Redis ≥ 6.2, (b) `EVAL` Lua agar 6.0 ikut jalan | **(b) perbaiki agar jalan di 6.0 ke atas** (2026-09-19) | host Redis < 6.2 kehilangan rute callback |
| D5 | Bagaimana cap dibaca kembali? (G12) | (a) sintesis window, (b) field cap di rute baca, (c) amend spec | **(b) field cap terpisah** (2026-09-19) | panel tidak bisa menampilkan budget tersimpan |
| D6 | Bentuk seam empat kelas G21 | lihat `docs/DRAFT/002-G21-SEAM-DESIGN.md` §5: A1–A3 (AssemblyAI), B1–B2 (Polly), C1–C3 (Edge/Google TTS), D1 (Local Device) | **(belum dijawab)** | tidak ada adapter multi-request yang boleh ditulis; kelimanya tetap ditolak dengan nama |

## 5. Urutan kerja usulan

1. **P2.1, G1**: verifikasi live sisa permukaan. Selesai 2026-09-19.
2. **P2.2, G2 + G3**: satu keputusan D1, satu implementasi guard. Selesai
   2026-09-19 dengan D1 = (a).
3. **P2.3, G6**: keputusan D3 lalu sambungkan `Outcome()` ke recorder. Selesai
   2026-09-19 dengan D3 = (b).
4. **P2.4, G4**: keputusan D2 lalu wire atau amend. Selesai 2026-09-19 dengan
   D2 = (a): hook rute per request + cek tujuan tetap jalan, bukti live proxy
   loopback.
5. **P2.5, G14**: temuan G1 yang tersisa, butuh D4 untuk pilihan lantai versi
   (testnya bisa ditulis lebih dulu). Selesai 2026-09-19 dengan D4 = (b).
6. **P2.6, G12 + G13 + G5**: G12 butuh D5, G13 dan G5 adapter/paritas. G13 selesai
   2026-09-19; G12 selesai 2026-09-19 dengan D5 = (b); G5 incremental:
   Deepgram STT slice 1, NVIDIA NIM TTS slice 2, Cartesia TTS slice 3, tujuh
   adapter TTS slice 4–10 (ElevenLabs, MiniMax + MiniMax CN, Inworld, PlayHT,
   Coqui, Tortoise, Gemini TTS), dan Gemini STT slice 11 selesai 2026-09-19 —
   G5 CLOSED, lima format multi-request tersisa di G21.
7. **P2.7, G15 + G16**: temuan click-through G13 (routing `no_auth` dan header
   media kosong), tanpa keputusan owner. Selesai 2026-09-19.
8. **P2.8, G8, G9, G10, G19**: penutup kecil + dokumen + aksi lingkungan
   pemilik. G7 selesai 2026-09-19 (keputusan default tercatat di changelog §7.10);
   G9 selesai 2026-09-19 (baris akses kegagalan membawa kode); G8 dan G19 selesai
   2026-09-19 (`.env` disinkronkan, migration `000011` mengembalikan kepemilikan
   tabel P2 ke role aplikasi); G10 selesai 2026-09-19 (status row
   `SYSTEM_MAP.md` menyebut P2 CLOSED beserta sisanya).
9. **P2.9, G17 + G18**: temuan pass G6 di jalur chat (baris usage error dan baris
   log chat), tanpa keputusan owner. Selesai 2026-09-19.
10. **P2.10, G20**: temuan pass G17/G18 (penolakan media/embeddings pra-panggilan
    tanpa baris log), tanpa keputusan owner. Selesai 2026-09-19.
11. **P2.11, G21**: lima format yang butuh lebih dari satu request, dipisahkan dari
    G5 oleh keputusan owner 2026-09-19. Terdaftar, belum dikerjakan — desain seam
    keempat kelas tersedia 2026-09-19 di `docs/DRAFT/002-G21-SEAM-DESIGN.md`, dan
    keputusannya menunggu owner (D6 di §4).

D1, D2, D3, D4, dan D5 sudah dijawab owner 2026-09-19 (§4); P2.2, P2.3, P2.4,
P2.5, P2.6 (G12/G13/G5 slice 1–11), P2.7, P2.8 (G7/G8/G9/G10/G19), P2.9, dan P2.10
sudah selesai. Yang terdaftar dan belum dikerjakan: G21, dan satu-satunya keputusan
yang masih menunggu owner adalah bentuk seam-nya (D6, desain di
`docs/DRAFT/002-G21-SEAM-DESIGN.md`).

## 6. Bukan gap (keputusan final, jangan dibuka lagi)

- Dua wire klien: OpenAI-compatible + Anthropic-compatible. Tidak ada bentuk ketiga.
- Format gate menolak provider yang belum di-port **dengan nama formatnya**; itu
  perilaku yang diinginkan, bukan bug (G5 yang mengubahnya, bukan keluhan).
- Shorthand `provider/model/voice` tidak di-port (id model boleh mengandung slash).
- Override §7.10 dibaca per panggilan; registry adalah default, override menang,
  tanpa fallback cloud diam-diam.
- Media tidak punya alias dan combo: rute media menyapa layanan provider langsung.
- Combo test §7.7 tidak menulis usage row (tanpa gateway key, murni diagnostik).
- Node kustom adalah passthrough: model apa pun di bawah prefix-nya diteruskan apa
  adanya ke upstream (terverifikasi live: `stub/whatever-model` sampai ke stub),
  jadi routing tidak menuntut baris katalog. G11 soal tampilan dan validasi, bukan
  soal routing.
- Probe endpoint pada node kustom menjawab
  `"this provider does not declare a validation endpoint"`; itu jawaban yang
  benar (node memang tidak punya validate URL, dan probe node §7.4 yang dipakai
  untuk kasus ini), bukan bug.
- `has_oauth` di `GET /providers` berasal dari field YAML registry, bukan turunan
  dari blok `oauth`; `claude` mendeklarasikan blok oauth tetapi `has_oauth: false`,
  persis seperti data reference.
- `GET /api/v1/models` (data plane) sengaja hanya memuat model yang dideklarasikan
  registry plus nama combo; baris `models_custom` tidak masuk ke situ, dan itu
  tertulis di `internal/dataplane/catalog.go`: provider passthrough tidak
  menyebutkan katalognya, jadi satu id karangan akan menjadi tebakan. G11 soal
  katalog manajemen dan ref combo, bukan daftar data plane.

## 7. Definisi selesai & cara verifikasi

- **Kode:** `go test -race -count=1 ./...` hijau; tagged integration hijau;
  `go-lint.sh` dan `go-headers.sh` PASS; commit per item dengan pesan yang menyebut
  buktinya.
- **Live:** boot terhadap PostgreSQL + Redis nyata, rute digiring dengan curl,
  hasil dicatat di §8 (bukan hanya "berhasil").
- **Dokumen:** changelog SPEC-API untuk perubahan kontrak; SYSTEM_MAP untuk
  perubahan topologi; file ini diperbarui statusnya.

## 8. Log verifikasi

Harness click-through G1 (2026-09-19): dua binary dari tree yang sama, satu polos
dan satu dibangun dengan patch registry sementara (`claude.oauth.token_url` dan
`user_info_url` diarahkan ke loopback; `registry.yaml` dipulihkan sebelum dan
sesudah build, `git diff` bersih); stub upstream OpenAI/Anthropic + token endpoint
di 127.0.0.1:8091; stub proxy forwarding di 127.0.0.1:8092; shim RESP `GETDEL`
untuk Redis 6.0.16 di 127.0.0.1:6390. `EGRESS_ALLOWED_TARGETS=127.0.0.1/32`,
`PROXY_TEST_URL` menunjuk stub. Semua artefak DB/Redis dibersihkan setelah pass
(usage 4 baris, gateway key, quota cap, endpoint, node, combo, proxy, model kustom,
alias, disabled, state OAuth; `panel_auth.password_hash` kembali NULL).

| Tanggal | Item | Hasil |
|---|---|---|
| 2026-09-19 | §7.10 media (G1 sebagian) | PASS: 9 rute live, override per-call, health 0→1→0, tanpa usage row; artefak uji dibersihkan |
| 2026-09-19 | G1 manajemen: provider-nodes, endpoints + keys, models catalog/custom/aliases/disabled, combos, combo test, token-saver, proxies (dua rute test), quotas | PASS dengan temuan: `POST /models/custom` untuk node 400 (G11), ref combo node ditolak (G11), cap PUT 200 tetapi GET `{"data":[]}` (G12); prefix collision 409, alias target tak dikenal 400, level token-saver tak dikenal 400, cap negatif 400; proxy test loopback allowlist `ok` 3 ms, `10.0.0.1` ditolak "a private address" + petunjuk allowlist, `169.254.169.254` ditolak "a link-local address"; endpoint test node menjawab "does not declare a validation endpoint" (by design); combo test `NO_PROVIDER_AVAILABLE` untuk ref registry tanpa endpoint |
| 2026-09-19 | G1 data plane: chat non-stream, chat stream, messages (wire Anthropic), models | PASS: 200 dengan jawaban stub apa adanya; stream SSE utuh; `/messages` ditranslate ke bentuk Anthropic (usage 11/7); 401 tanpa key dan salah key (envelope OpenAI), provider tak dikenal 400 `MODEL_NOT_FOUND`; key endpoint terpasang ke upstream sebagai header Bearer (dibuktikan dari log stub) |
| 2026-09-19 | G1 embeddings lewat node kustom | FAIL by design: 400 `PROVIDER_NOT_ROUTABLE` (G13) |
| 2026-09-19 | G1 OAuth: start, callback (JSON + redirect), status, refresh (per endpoint + due sweep) | PASS lewat shim `GETDEL`: authorize URL berisi client_id, state, PKCE S256, scopes; callback 200 `created:true` label `stub@example.com` (identitas dari userinfo stub), replay dan state asing 400, callback browser 302 ke `/providers/claude?oauth=connected&endpoint_id=…`; status menampilkan `refresh_state:"due"`; refresh per endpoint dan due sweep `refreshed:1`; endpoint asing 404, provider non-oauth 400; stub melihat grant JSON dengan `code_verifier` dan refresh grant dengan `refresh_token`. Tanpa shim (Redis 6.0.16) callback 500 tanpa log: G14 |
| 2026-09-19 | **G11 CLOSED**: `CatalogIndex` + `runtimeIndex` di wiring, dua test baru | PASS: suite `-race` 13 paket hijau, `go-lint.sh` dan `go-headers.sh` (442 file) PASS; live pada binary yang sama: `POST /models/custom` node 201, katalog memuat `openai-compatible-…/stub-model`, `POST /combos` ref node 201; artefak dibersihkan (node, model, combo, key, hash panel) |
| 2026-09-19 | **G13 CLOSED**: `nodeEmbeddingMedia` + format eksplisit di `IsGeminiEmbedding`, dua test baru (tabel lima bentuk node + kontrol benign) | PASS live (PostgreSQL 14 + Redis nyata, stub loopback 8091, tanpa shim): node `openai-compatible-…` (base `http://127.0.0.1:8091/v1`) + endpoint + key, `POST /embeddings` model `g13node/stub-embed` menjawab 200 dalam 58 ms bentuk OpenAI (`data[0].embedding [0.1,0.2,0.3]`, usage 4); log stub membuktikan `POST /v1/embeddings` dengan payload OpenAI dan kredensial endpoint sebagai bearer. Negatif: endpoint `api_key` tanpa key ditolak saat create (VALIDATION_ERROR), endpoint `no_auth` tanpa key ditolak selector 503 tanpa dial (G15), node anthropic-compatible tetap 400 `PROVIDER_NOT_ROUTABLE`. Artefak dibersihkan (tiga node, tiga endpoint, key, gateway key, hash panel) |
| 2026-09-19 | G13 temuan: `no_auth` tanpa key vs bearer kosong | G15: endpoint `no_auth` tanpa key 201 tetapi 503 `NO_PROVIDER_AVAILABLE`, nol dial. G16: dengan key placeholder, chat sampai ke stub tanpa header `Authorization`, embeddings sampai dengan `Authorization: Bearer` kosong (keduanya dari log stub) |
| 2026-09-19 | **G15 + G16 CLOSED**: `Select` memilih endpoint `no_auth` tanpa key, health write no-op untuk selection tanpa key, `MediaTarget` tidak menulis header saat secret kosong; tiga test baru | PASS live (PostgreSQL 14 + Redis nyata, stub loopback 8091): node A dengan endpoint `no_auth` **tanpa key** menjawab 200 untuk chat dan embeddings (sebelumnya 503), dan log stub menunjukkan **tidak ada** header `Authorization` di kedua panggilan; kontrol node B berkey tetap 200 dengan `Bearer` berisi key di kedua jalur. Artefak dibersihkan (dua node, dua endpoint, gateway key, usage baris pass, hash panel) |
| 2026-09-19 | **G14 CLOSED**: `Take` jadi skrip atomik `GET`+`DEL` via `redis.NewScript`, empat test store bertag `integration` | PASS: terhadap Redis 6.0.16 host **tanpa shim**, keempat test merah `unknown command getdel` sebelum perbaikan dan hijau sesudahnya (stage+TTL, take menghapus key, replay `ok=false` tanpa error, stage ulang `ErrStateAlreadyStaged` tanpa menimpa payload, TTL kedaluwarsa); `GET /oauth/callback` state asing menjawab 302 `oauth_error` dalam 4 ms (sebelumnya 500 dalam 1 ms); tanpa key tersisa |
| 2026-09-19 | **G2 + G3 CLOSED**: `egress_wiring.go` (satu guard + satu client ber-guard), `HTTPClientDeps{Dialer}` di `NewHTTPClient`, guard di probe, empat jalur wiring berbagi guard; `egress_wiring_test.go` + tabel probe | PASS: suite `-race` 13 paket hijau, tagged integration hijau, `go-lint.sh` PASS (golangci-lint 0 issues), `go-headers.sh` 452 file PASS; live (PostgreSQL 14 + Redis nyata, stub `0.0.0.0:8091`): chat 200 dan embeddings 200 lewat node allowlist (baris stub bertambah), node `127.0.0.2` (terbukti hidup lewat curl langsung) 502 `UPSTREAM_ERROR` tanpa baris `/denied` di stub, probe empat node (allowlist `ok`; loopback/privat/link-local `fail` beralasan, 0 dial), proxy test lewat guard yang sama tetap benar, boot dengan allowlist salah berhenti dengan pesan netguard. Artefak dibersihkan (4 node, 2 endpoint, 2 gateway key, usage baris pass, hash panel kembali NULL) |
| 2026-09-19 | **G6 CLOSED**: `RecordUse` di kontrak repo gateway key + seam `KeyUseRecorder` di `ChatService.Authenticate`; `dataPlaneRecorder` (pasangan usage+log) di media dan embeddings; klasifikasi satu-exit di `Perform`/`perform`; `accountingModel()` untuk search; `MediaCall.Model` dihapus; test baru `key_use_test.go`, `media_record_test.go` (+stub), `TestIntegration_RecordUse` | PASS: suite `-race` 13 paket hijau, tagged integration hijau, `go-lint.sh` PASS (golangci-lint 0 issues), `go-headers.sh` 460 file PASS; live (PostgreSQL 14 + Redis nyata, stub loopback 8091, gateway 127.0.0.1:8099): baseline `usage=2 logs=1 reqcount=0` → speech 200 (+1/+1/+1), speech ke jalur 500 (+1/+1/+1, usage `status:error` `UPSTREAM_ERROR`, log `UPSTREAM_ERROR: stub refused`), embeddings 200 (+1/+1/+1), search 200 (+1/+1/+1, `cost_usd` 0.005 = registry `cost_per_query`, model = provider id `brave-search`); tiap pasangan usage+log satu request id, tokens 0, latency sama, key id terisi; kontrol chat 502 menaikkan reqcount tanpa baris media. Artefak dibersihkan: baseline pulih (`usage=2 logs=1`, keys/endpoints/upkeys/nodes/media rows 0, `panel_auth.password_hash` NULL). Dua temuan baru dicatat: G17 (kegagalan chat tanpa usage row), G18 (chat tanpa `request_logs`) |
| 2026-09-19 | G19 temuan pass G6: kepemilikan tabel P2 | FAIL dengan DSN aplikasi (role `pannelai`): `PATCH /media-providers/openai` → `permission denied for table media_provider_settings`; `has_table_privilege` media `SELECT=false INSERT=false UPDATE=false`, `proxies SELECT=false INSERT=false`, sementara tabel lain dimiliki `pannelai`. Pass diselesaikan dengan DSN superuser; aksi pemilik dicatat sebagai G19 |
| 2026-09-19 | **G12 CLOSED**: `cap` di body `GET /quotas/{endpoint_id}` (D5 = b), `QuotaService.GetCap` dipakai handler, `quota_test.go` (PUT→GET, `"cap":null`, rute koleksi tanpa cap, cost negatif 400) | PASS: suite `-race` 13 paket hijau, tagged integration hijau, `go-lint.sh` PASS, `go-headers.sh` PASS; live (PostgreSQL 14 + Redis nyata, DSN `.env` role aplikasi, tanpa workaround G19): `PUT /quotas/{id}` → 200 `{"monthly_cost_usd":"5.00000000","monthly_tokens":1000}`, `GET /quotas/{id}` → 200 `{"endpoint_id":…,"cap":{…5.00000000, 1000…},"data":[]}` (sebelumnya `{"data":[]}`), `GET /quotas` → 200 `{"data":[]}` tanpa field cap. Artefak dibersihkan (endpoint, baris `quota_caps`, hash panel NULL; baseline `usage=2 logs=1` pulih) |
| 2026-09-19 | **G7 CLOSED**: keputusan default kind `video` (biarkan terdokumentasi, tanpa provider) | Terpenuhi oleh changelog §7.10 yang sudah ada (commit `1f00860`): rute terdaftar menolak `PROVIDER_NOT_ROUTABLE` dengan alasan paritas reference; tidak ada perubahan kode. Kalau kelak ada provider ber-kind `video`, adapternya masuk G5 |
| 2026-09-19 | **G17 CLOSED**: identity dibangun sebelum dial di `relayOnce` (dipindah ke `engine_relay.go` karena `engine.go` menembus batas 250 baris), `lastOutcome` di `Relay`, identity fusion dari `fanOut`/`judge`, guard identity eksplisit di `ChatService.record`; empat test baru | PASS live: `POST /chat/completions` model `g18node/broken` menjawab 502 `UPSTREAM_ERROR` dan menulis satu baris `usage_records` `status:error` `error_code:UPSTREAM_ERROR` dengan provider/endpoint/model terisi (`openai-compatible-…`/`ep_…`/`broken`), token 0, latency 7 ms, request id `0385RT4FG14Y44H76TZW2NB1YJ` sama dengan baris log-nya; sukses 200 token 5/2; `nosuchprovider/model` 400 `MODEL_NOT_FOUND` hanya baris log; baseline `usage=2 logs=1 keys=0 endpoints=0 nodes=0` dipulihkan; suite `-race` 13 paket, tagged integration, `go-lint.sh` (0 issues), `go-headers.sh` (464 file) hijau |
| 2026-09-19 | **G18 CLOSED**: seam `Logs RequestLogRecorder` di `ChatService` + `chat_record.go` (satu baris `request_logs` per panggilan chat, request id sama dengan usage row, `error` = kode saja, body dari `in.Raw`/`outcome.Body` diserahkan ke aturan capture `LogService.Record`); tiga test baru | PASS live: lima panggilan chat masing-masing meninggalkan satu baris `request_logs` dengan request id yang sama seperti log server (`0385RT4F97CD2B8S4JMXSKGEWW` 200, `0385RT4FG14Y44H76TZW2NB1YJ` 502, `0385RT4FKMS66HGQ841KG2A78M` 200, `0385RT5213H7JB69WFC19FJHS4` 400, `0385RT7NDEKVYRZKWR56AC8BM0` 200 capture); baris gagal menyimpan `error` = `UPSTREAM_ERROR` saja sementara respons klien memuat pesan upstream berisi `sk-g18-should-not-be-stored`; pencarian sentinel `sk-g18` di `request_logs` (error + dua body) dan `usage_records` menjawab 0 baris; capture default mati → body kosong, `PATCH /settings {"logging":{"request_capture_enabled":true}}` → body permintaan + jawaban tersimpan utuh; baris setting capture dihapus saat cleanup; `gateway_keys.request_count` 3/1/1 untuk tiga key |
| 2026-09-19 | G20 temuan pass G17/G18: penolakan media/embeddings sebelum panggilan tidak menulis baris log | FAIL by design saat ini: `MediaCallService.Prepare` dan `EmbeddingsService.Embed` menulis baris hanya setelah `perform`, jadi provider tak dikenal, `base_url` kosong, format gate, kredensial kosong, atau `NO_PROVIDER_AVAILABLE` tidak meninggalkan `request_logs` — sedangkan jalur chat kini mencatat setiap panggilan (terbukti live untuk 400 `MODEL_NOT_FOUND`). Dicatat sebagai G20 |
| 2026-09-19 | **G9 CLOSED**: seam `schema.ErrorCodeRecorder` di-set `WriteError`/`writeDataPlaneError`/`writeEnvelope`, diimplementasikan `responseRecorder`, diteruskan eksplisit `statusRecorder` (Go hanya mempromosikan metode interface yang di-embed); `logging` menambah attr `code` hanya saat gagal; `router_errorlog_test.go` (empat jalur + kasus negatif) | PASS: suite `-race` 13 paket hijau, tagged integration hijau, `go-lint.sh` PASS (golangci-lint 0 issues), `go-headers.sh` 466 file PASS; live pada binary dari tree yang sama (tanpa stub): `GET /api/v1/nope` → 404 baris `code=NOT_FOUND`, `POST /api/v1/chat/completions` tanpa key → 401 `code=UNAUTHORIZED`, `POST /api/v1/images/generations` tanpa key → 401 `code=UNAUTHORIZED`, `GET /api/v1/health` dan `/version` 200 tanpa field `code`; baseline tidak berubah (`usage=2 logs=1 keys=0 endpoints=0 nodes=0 settings=1 auth_null=true`), `panel_auth.password_hash` di-null kembali; changelog SPEC-API dicatat |
| 2026-09-19 | **G20 CLOSED**: `dataPlaneRecorder.refuse` (satu baris `request_logs`, kode saja, tanpa usage), `MediaCallService.prepareForCall` + `refusalOutcome`, dua penolakan awal `Search`, `EmbeddingsService` dipecah ke `resolveCall` (identity dibangun sebelum langkah gagal pertama) + port `ModelResolver`/`MediaRouter`; dua test baru | PASS: suite `-race` 13 paket hijau, tagged integration hijau, `go-lint.sh` PASS (golangci-lint 0 issues), `go-headers.sh` 470 file PASS; live pada binary dari tree yang sama: empat penolakan (images `nope/whatever` 400 `MODEL_NOT_FOUND`, embeddings `nope/embed` 400 `MODEL_NOT_FOUND`, speech `elevenlabs/voice` 400 `PROVIDER_NOT_ROUTABLE`, search `nope` 400 `MODEL_NOT_FOUND`) masing-masing meninggalkan **satu** baris `request_logs` dengan request id yang sama dengan baris akses server, `status:error`, `error` = kode saja, latensi 0, body kosong, identity sejauh resolusi (`nope/whatever`, `elevenlabs/voice`, `nope/nope`, embeddings tanpa identity); nol baris usage (total tetap 2); kontrol negatif 401 tanpa key dan 400 validasi handler tidak menulis baris (`control_rows=0`); baseline dipulihkan (`usage=2 logs=1 keys=0 endpoints=0 upkeys=0 nodes=0 caps=0 settings=1 auth_null=true`), `/tmp/g20_*` dihapus |
| 2026-09-19 | **G5 adapter Deepgram STT (slice 1)**: format gate `deepgram` untuk `stt`; raw audio request + safe MIME, `model`/`smart_format`/`punctuate`/`language|detect_language`, `Authorization: Token`, nested transcript → `{text}`; alias resolution; `media_deepgram_test.go` + MIME/auth tests | PASS: focused + affected package `-race` tests, `go-lint.sh` (golangci-lint 0 issues), `go-headers.sh` (472 files); full hermetic/tagged suite after the code freeze; live stub proof: Deepgram registry base URL overridden to loopback, multipart upload through `/audio/transcriptions` returned normalized `{"text":"deepgram live"}`, stub saw raw bytes, `Content-Type: audio/mpeg`, query model + flags, `Authorization: Token`; 15 format/provider adapters remain open |
| 2026-09-19 | **G5 adapter NVIDIA NIM TTS (slice 2)**: format gate `nvidia-tts` untuk `tts`; typed `{input:{text}, voice, model}` body, voice default `default`, `Authorization: Bearer`, output format WAV; `media_nvidia_test.go` + OpenAI control | PASS: focused + affected package `-race` tests, `go-lint.sh` (golangci-lint 0 issues), `go-headers.sh` PASS; full hermetic/tagged suite after code freeze; live stub proof: `nvidia/fastpitch` returned raw bytes with `Content-Type: audio/wav`, `?response_format=json` + explicit voice returned base64 `format:wav`, stub saw nested body and bearer; baseline dipulihkan (`usage=2 logs=1 keys=0 endpoints=0 upkeys=0 nodes=0 caps=0 settings=1 auth_null=true`); 14 format/provider adapters remain open |
| 2026-09-19 | **G5 adapter Cartesia TTS (slice 3)**: format gate `cartesia` untuk `tts`; typed `{model_id, transcript, voice?, output_format}` body, voice omitted/default behavior, `X-API-Key`, `Cartesia-Version: 2024-06-10`, fixed MP3 output; `media_cartesia_test.go` + OpenAI control | PASS: focused + affected package `-race` tests, `go-lint.sh` (golangci-lint 0 issues), `go-headers.sh` PASS; full hermetic/tagged suite after code freeze; live stub proof: `cartesia/sonic-2` returned raw bytes `Content-Type: audio/mp3`, `?response_format=json` + `sonic-3` explicit voice returned base64 `format:mp3`, stub saw `X-API-Key`, version header, omitted/explicit voice bodies; baseline dipulihkan (`usage=2 logs=1 keys=0 endpoints=0 upkeys=0 nodes=0 caps=0 settings=1 auth_null=true`); 13 format/provider adapters remain open |
| 2026-09-19 | **G8 CLOSED**: `.env` dibangun ulang dari `.env.example` (hanya empat baris nilai berbeda), `REDIS_PASSWORD` dibaca dari `/etc/redis/redis.conf` saat penulisan, `ENCRYPTION_KEY` 32 byte baru (DB memegang 0 baris sealed), blok `PUBLIC_BASE_URL` ikut template | PASS: boot polos (hanya `.env`, tanpa override) menjawab `{"status":"ok","checks":{"postgres":"ok","redis":"ok"}}`; `redis-cli PING` dengan `REDISCLI_AUTH` dari `.env` menjawab `PONG`; diff `.env` vs `.env.example` = 8 baris; pass G19/G5/G4 sesudahnya memakai DSN yang sama tanpa override |
| 2026-09-19 | **G19 CLOSED**: `migrations/000011_p2_table_ownership.up.sql` (re-own ke role pemilik `gateway_keys`, penolakan jadi warning berisi resep, bukan boot gagal), `migrations.openDB` menyetel `OnNotice` pgx ke slog, `migrations/ownership_test.go` mengunci invarian seluruh schema | PASS: test merah saat `proxies` dipindah ke `rusmanadodi` (`table proxies is owned by rusmanadodi`), warning muncul dengan resep `ALTER TABLE proxies OWNER TO pannelai` saat role aplikasi menjalankan `000011`, role ber-privilege memindahkan tabel **dan indeks** lalu hijau; live: boot ber-DSN superuser menerapkan `000011`, kedua tabel jadi `pannelai`, lalu boot DSN `.env` polos: `PATCH /media-providers/openai` 200 `base_url_source:"override"`, `POST /proxies` 201, `GET /proxies` 200, `DELETE /proxies/{id}` 204 (sebelumnya 500 `permission denied`); cleanup `delete from media_provider_settings` dijalankan role aplikasi sendiri; baseline `usage=2 logs=1 settings=1 auth_null=true` pulih |
| 2026-09-19 | **G5 slice 4–10 CLOSED**: tujuh adapter TTS (ElevenLabs, MiniMax + MiniMax CN, Inworld, PlayHT, Coqui, Tortoise, Gemini TTS), seam `MediaPath` + cabang kredensial `basic`/`playht` + `mediaAnswerReader`; satu file test per provider | PASS: suite `-race` 13 paket hijau, tagged integration hijau, `go-lint.sh` PASS, `go-headers.sh` PASS; live (registry sementara diarahkan ke stub loopback 8091 lalu dipulihkan, `git diff` bersih, DSN `.env` polos): ElevenLabs `xi-api-key` + voice di path + `{text,model_id,voice_settings}` → bytes MP3 + `format:mp3`; MiniMax & MiniMax CN bearer + `output_format:"hex"` + nested `voice_setting`/`audio_setting` → hex di-decode; Inworld `Authorization: Basic` + `{text,voiceId,modelId,audioConfig}` → base64; PlayHT `X-USER-ID` + bearer dari satu materi `userId:apiKey` (dibuktikan setelah key diperbarui) + `Accept: audio/mpeg`; Coqui `{text}` tanpa kredensial → WAV; Tortoise `{text,voice:"random"}` → WAV; Gemini `key` query + model di path sebelum `:generateContent` + prompt "Say: …" → PCM dibungkus RIFF/WAVE 44 byte; kontrol negatif lima provider multi-step tetap 400 `PROVIDER_NOT_ROUTABLE` dengan nama format; 22 panggilan terautentikasi = 22 `request_count`, tiap panggilan satu usage + satu log; artefak dibersihkan (8 endpoint, 6 key, 3 gateway key, 17 usage, 22 log; baseline `usage=2 logs=1 keys=0 endpoints=0 upkeys=0 settings=1 auth_null=true` pulih) |
| 2026-09-19 | **G21 terdaftar**: lima format yang butuh lebih dari satu request (AssemblyAI upload→submit→poll, AWS Polly SigV4 tanpa implementasi reference, Edge TTS & Google TTS token hasil scraping HTML, Local Device proses host) — keputusan owner memisahkannya dari G5 | Terdaftar di §2/§3 dengan alasan per provider dan bentuk seam yang dibutuhkan; SPEC-API §7.10 menyebut kelimanya sebagai batas yang dipilih, bukan kelalaian; format gate tetap menolak dengan nama (terbukti live di pass G5 slice 4–10) |
| 2026-09-19 | **G4 CLOSED**: seam `Proxy` di `dataplane.HTTPClientDeps`, `egressProxy`/`proxyRoute`/`exemptFromProxy` di `egress_wiring.go` (settings dibaca per request, tujuan divalidasi sebelum rute dikembalikan, gagal baca/URL ⇒ tolak bukan bypass); `egress_proxy_test.go` (tabel rute + A01) | PASS: dua mutasi diukur merah (hook dilepas ⇒ baris "arrives at it" gagal; cek tujuan dilepas ⇒ baris A01 gagal), tree hijau setelah dipulihkan; live (stub chat 8091, proxy forwarding 8092, `EGRESS_ALLOWED_TARGETS=127.0.0.1/32`, DSN `.env`): proxy mati → log proxy 0 baris; proxy nyala → log proxy 1 baris berisi absolute-form ke 8091 dan stub tetap menerima; `outbound_no_proxy=127.0.0.1` → langsung lagi, log proxy 0; tujuan `127.0.0.2` (di luar allowlist) → 502 `UPSTREAM_ERROR` dengan 0 baris di proxy dan 0 baris baru di stub; artefak dibersihkan (2 node, 2 endpoint, gateway key, 4 usage, 4 log, baris settings `network`; baseline pulih) |
| 2026-09-19 | **G10 CLOSED**: status row `SYSTEM_MAP.md` menyebut **P2 CLOSED** dengan daftar bukti live-nya dan sisa yang eksplisit tidak ikut tertutup (lima format media G21); §3.6 menambah paragraf seam adapter, §4a menambah narasi migrasi P2 `000009`-`000011` + aturan kepemilikan, batas domain §1 menyebut `000011` | PASS: narasi diverifikasi terhadap kode dan bukti pass yang tercatat di berkas ini (bukan diklaim ulang); tidak ada perubahan kode |
| 2026-09-19 | **G5 slice 11 CLOSED** (G5 penuh): adapter Gemini STT (`media_gemini_stt.go`), seam `transcriptionRequest` (dispatch request per format di `media_transcription.go`, sehingga `media_audio.go` tinggal memanggil), gate `gemini-stt` untuk `stt`, `transcriptionText` sebagai pembentuk jawaban bersama Deepgram; `media_gemini_stt_test.go` + tabel gate per kind | PASS: suite `-race` 13 paket hijau, tagged integration hijau, `go-lint.sh` PASS (golangci-lint 0 issues), `go-headers.sh` 496 file PASS; live (registry sementara diarahkan ke stub loopback 8093 lalu dipulihkan, `git diff` bersih, DSN `.env` polos tanpa grant superuser): `POST /audio/transcriptions` `gemini/gemini-2.5-flash` filename `clip.mp3` dengan `prompt=names` + `language=Indonesian` → 200 `application/json` `{"text":"gemini stt live transcript"}`; stub melihat `POST /gmstt/v1beta/models/gemini-2.5-flash:generateContent?key=gm-secret` dengan body `{"contents":[{"parts":[{"text":"names Language: Indonesian."},{"inline_data":{"mime_type":"audio/mpeg","data":"<base64 audio>"}}]}]}` (prompt, MIME, bytes, dan key-query keempatnya cocok); panggilan kedua `gemini/gemini-2.5-pro` filename `clip.wav` → prompt bawaan + `mime_type":"audio/wav"`; kontrol benign `assemblyai/best` (G21) tetap 400 `PROVIDER_NOT_ROUTABLE` "does not translate yet" tanpa baris stub baru; akuntansi: 2 panggilan → +2 usage (`success`, provider `gemini`, model benar) + 2 log + `request_count=2`, penolakan kontrol menulis satu log `error=PROVIDER_NOT_ROUTABLE` tanpa usage; artefak dibersihkan (2 gateway key, 1 endpoint + 1 upstream key, 2 usage, 3 log, `panel_auth.password_hash` di-null) → baseline `usage=2 logs=1 keys=0 endpoints=0 upkeys=0 nodes=0 caps=0 settings=1 auth_null=true media_settings=0 proxies=0` pulih; `/tmp/g5d-app-serv` dan cookie dihapus |
| 2026-09-19 | **Desain seam G21** (empat kelas multi-request) ditulis di `docs/DRAFT/002-G21-SEAM-DESIGN.md`; §3 G21 menunjuk dokumen itu, §4 menambah D6, §5 P2.11 mencatat desainnya | Belum ada kode — menunggu keputusan owner. Isi per kelas: bukti reference (file:line), bentuk seam, kontrak/registry yang berubah, risiko + temuan keamanan (argument injection `say -v <voiceId>` di Local Device; ToS/kerapuhan scraping Edge/Google; region Polly tidak boleh ditebak), test yang akan mengunci, dan tabel keputusan A1–A3/B1–B2/C1–C3/D1. Format gate tidak berubah: kelimanya tetap ditolak dengan nama sampai keputusan dijawab |
