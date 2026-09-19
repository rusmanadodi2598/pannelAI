# P2-GAPS.md: Sisa Gap Fase P2 (app-serv)

Dokumen kerja untuk menutup fase **P2**, bukan kontrak. Kontrak tetap
`docs/SPEC-API/001-SPEC-API.md`; peta topologi tetap `SYSTEM_MAP.md`. Setiap
keputusan yang diambil di sini harus berakhir sebagai changelog SPEC-API (dan
SYSTEM_MAP bila topologinya berubah), bukan hanya sebagai centang di tabel.

| | |
|---|---|
| **Status** | P2: seluruh permukaan rute terpasang dan terverifikasi live (§7.4, §7.6, §7.7, §7.9, §7.10, §7.11, §7.12, §7.15); 16 gap terdaftar, enam di antaranya temuan click-through, empat sudah CLOSED (G11, G13, G15, G16) |
| **Dibuat** | 2026-09-19, dari hasil click-through live §7.10 (PostgreSQL 14 + Redis lokal, stub upstream loopback) |
| **Bukti terakhir** | commit `1f00860` + `573979b`; click-through G1 2026-09-19 (baris §8); gate hijau |

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
| G2 | Dial upstream (chat + media + embeddings) tidak melewati `internal/netguard` | keamanan (A01) | **ya** | 2 |
| G3 | `provider_probe.go` mendial `base_url` dari user tanpa guard | keamanan (A01) | menyatu G2 | 2 |
| G4 | `settings.network.outbound_proxy_*` tidak dipakai di jalur dial | kontrak §7.11 | **ya** | 4 |
| G5 | Adapter media per-provider (format gate menolak 16 provider dengan nama) | fitur | tidak | 5 |
| G6 | Panggilan media tidak menulis usage/log; `gateway_keys.request_count` tidak pernah naik | akuntansi §7.12/§7.13 | **ya** | 3 |
| G7 | Kind `video` terdaftar tanpa provider (rute menolak) | cakupan | tidak | 6 |
| G8 | `.env` lokal drift dari `.env.example` sehingga boot polos gagal | lingkungan | tidak | 6 |
| G9 | Kegagalan request hanya tercatat sebagai `status`, tanpa kode/alasan | observability §1.6 | tidak | 6 |
| G10 | Status row `SYSTEM_MAP.md` masih menarasikan P1 sebagai fase terakhir | dokumen §1.9 | tidak | 6 |
| G11 | `POST /models/custom` menolak provider node kustom (katalog memegang index boot-time, bukan overlay). **CLOSED 2026-09-19** | bug wiring §7.6/§7.4 | tidak | 2 |
| G12 | Budget cap hanya bisa ditulis; `GET /quotas/{endpoint_id}` tidak pernah memuatnya sampai ada window usage | kontrak §7.12 | **ya** | 3 |
| G13 | Embeddings menolak node openai-compatible (reference punya adapter `openaiCompatNode`). **CLOSED 2026-09-19** | fitur/paritas | tidak | 5 |
| G14 | State store OAuth tanpa test; `GETDEL` butuh Redis ≥ 6.2 dan gagalnya 500 tanpa log | test + portabilitas | **ya** | 2 |
| G15 | Endpoint `no_auth` tanpa key tidak pernah terpilih (selector menuntut key sebelum cabang auth type). **CLOSED 2026-09-19** | bug routing §7.5 | tidak | 2 |
| G16 | Jalur media mengirim `Authorization: Bearer` kosong saat materi kredensial kosong; jalur chat mengirim tanpa header. **CLOSED 2026-09-19** | bug kredensial §8.1 | tidak | 2 |

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

### G2: Egress guard untuk dial upstream

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

**Definisi selesai.** Keputusan tercatat di SPEC-API; implementasi + tabel test
(host publik lolos, privat/loopback ditolak, loopback yang di-allowlist lolos).

### G3: `provider_probe.go` tanpa guard

**Bukti.** `cmd/app-serv/provider_probe.go` mendial `base_url` yang di-supply user
saat probe node kustom (§7.4/§7.5), tanpa guard. Ditemukan saat §7.11, dilaporkan,
belum diubah.

**Kenapa.** Kelas A01 yang sama dengan G2, tapi inputnya lebih langsung (satu
request panel, satu dial ke alamat pilihan penyerang).

**Pendekatan.** Menyatu dengan G2: keputusan yang sama, guard yang sama, allowlist
yang sama. Jangan diselesaikan terpisah supaya tidak ada dua semantik egress.

**Definisi selesai.** Probe memakai guard; tabel test mencakup host privat,
loopback, dan yang di-allowlist.

### G4: Proxy assignment belum dipakai di jalur dial

**Bukti.** `settings.network.outbound_proxy_*` disimpan, divalidasi, dan dirender
panel, tetapi tidak ada pembaca di jalur request. §7.11 berbunyi "assignment rides
on the settings", dan separuh itu belum benar.

**Keputusan owner.** (a) wire egress lewat proxy terkonfigurasi (binding
per-endpoint tetap deferred, sama seperti reference), atau (b) amend SPEC-API §7.11
agar menyatakan settings belum dihormati di v1.

**Pendekatan kalau (a).** Satu transport yang memilih dialer proxy berdasarkan
settings; bisa dibuktikan dengan proxy stub loopback + allowlist G2.

**Definisi selesai.** Pilihan tercatat; kalau (a), satu panggilan chat terbukti
keluar lewat proxy (log stub).

### G5: Adapter media per-provider

**Bukti.** 16 provider mendeklarasikan format media non-OpenAI
(`assemblyai, aws-polly, cartesia, coqui, deepgram, edge-tts, elevenlabs, gemini`
untuk tts+stt, lalu `google-tts, inworld, local-device, minimax` (+`minimax-cn`),
`nvidia-tts, playht, tortoise`), dan semuanya ditolak `PROVIDER_NOT_ROUTABLE`
dengan nama formatnya (terverifikasi live untuk elevenlabs, 2026-09-19).

**Kenapa.** Ini keputusan sadar (menolak dengan nama lebih baik daripada mengirim
bentuk yang salah), tapi selama belum di-port, provider itu tidak bisa dipakai.

**Pendekatan.** Port builder per provider dari reference, satu per satu, urut
berdasarkan kebutuhan operator. Tiap adapter: bentuk payload dari registry +
konversi jawaban ke envelope §7.10.

**Definisi selesai (per adapter).** Format ditranslate, tabel test 3 sampai 5 kasus
termasuk kontrol benign, dan satu panggilan live ke stub yang meniru provider itu.

### G6: Akuntansi panggilan media

**Bukti.** Setelah 9 panggilan media pada 2026-09-19: `usage_records` tidak
bertambah (2 baris lama), `request_logs` tidak bertambah (1 baris lama), dan
`gateway_keys.request_count` tidak pernah di-increment siapa pun. Grep tidak
menemukan UPDATE untuk kolom itu, untuk chat maupun media. `MediaCall.Outcome()`
sudah ada tetapi tidak dikonsumsi pemanggil mana pun.

**Kenapa.** Layar Usage (§7.12) dan Logs (§7.13) panel kosong untuk media; kolom
request_count di daftar gateway key selalu 0.

**Keputusan owner.** (a) media menulis usage row (tokens 0; untuk search ada
`cost_per_query` di registry) dan request log; (b) `request_count` di-increment
middleware untuk setiap panggilan data plane terautentikasi; (c) combo test §7.7
tetap tanpa usage row, keputusan itu sudah final dan tidak dibuka lagi di sini.

**Pendekatan.** `Outcome()` sudah membawa provider/endpoint/model; yang kurang
hanya pemanggil di handler/service dan (untuk cost) sumber harga per kind.

**Definisi selesai.** Keputusan tercatat; kalau (a)+(b): satu panggilan media
menghasilkan satu usage row + satu request log, dan request_count key naik 1.

### G7: Kind `video` tanpa provider

**Bukti.** Rute terdaftar dan menolak dengan nama; tidak ada provider di registry
yang mendeklarasikan `video` (reference juga tidak mendefinisikannya, paritas).

**Pendekatan.** Biarkan terdokumentasi (default), atau cari provider yang
mendeklarasikan kind + tulis adapter (masuk G5).

**Definisi selesai.** Keputusan tercatat di changelog SPEC-API §7.10.

### G8: `.env` lokal drift dari `.env.example`

**Bukti.** `.env.example` sudah benar dan lengkap; `.env` lokal tidak:
`ENCRYPTION_KEY` 41 byte (harus tepat 32), `REDIS_PASSWORD` basi (`AUTH failed`),
`PANEL_BOOTSTRAP_PASSWORD` kosong. Boot polos gagal di validasi config.

**Pendekatan.** Aksi pemilik: sinkronkan `.env` dengan `.env.example` + secret asli
(baca `requirepass` dari `/etc/redis/redis.conf` saat itu juga; jangan disimpan).
Resep override sementara ada di `reference_appserv_local_env.md`.

**Definisi selesai.** `./app-serv` boot bersih tanpa override environment.

### G9: Logging kegagalan request

**Bukti.** Kegagalan media tampil di log sebagai `request handled … "status":502`
plus request id, tanpa kode error atau alasan. Klien menerima envelope yang benar;
operator tidak menerima "kenapa". Click-through G1 menambah satu bukti yang lebih
tajam: `GET /oauth/callback` gagal 500 karena `GETDEL` tidak dikenal Redis lokal,
dan log hanya mencatat `"status":500` dalam 1 ms, tanpa satu baris pun yang
menyebut Redis (lihat G14).

**Pendekatan.** Log satu baris dengan `code` (dan `error`) di jalur
`writeDataPlaneError`/handler, memakai request id yang sudah ada.

**Definisi selesai.** Tiap kegagalan request menghasilkan satu baris log berisi
kode + request id; ada test yang mengunci bentuknya.

### G10: Narasi P2 di `SYSTEM_MAP.md`

**Bukti.** Status row masih menyebut P1 sebagai fase terakhir yang CLOSED; §3.6 dan
§5 sudah memuat media dan `proxies`/`media_provider_settings`.

**Pendekatan.** Update sekali saat P2 benar-benar ditutup, bersama narasi fase.
Jangan dipecah per slice agar tidak churn.

**Definisi selesai.** Status row menyebut P2 CLOSED dengan daftar buktinya.

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

### G12: Budget cap tidak terbaca kembali

**Bukti.** `PUT /api/v1/quotas/{endpoint_id}` menjawab 200
`{"monthly_cost_usd":"5.00000000","monthly_tokens":1000}` dan baris `quota_caps`
benar-benar ada di PostgreSQL, tetapi `GET /api/v1/quotas/{endpoint_id}` langsung
sesudahnya menjawab `{"data":[]}` karena rute baca hanya membaca `quota_windows`
yang terisi oleh flush counter usage. Tidak ada rute mana pun yang membaca
`quota_caps` kembali.

**Kenapa.** Panel tidak bisa menampilkan budget yang sudah disetel (form selalu
kosong sampai ada traffic), padahal §7.12 menjadikan cap sebagai kontrol P2.

**Keputusan owner.** (a) rute baca menggabungkan cap ke window (sintesis window
`limit` walaupun `used` 0), (b) tambah pembacaan cap di `GET /quotas/{endpoint_id}`
lewat field terpisah, atau (c) amend SPEC-API §7.12 agar cap dinyatakan tulis-saja
di v1.

**Definisi selesai.** Pilihan tercatat; kalau (a)/(b), satu PUT lalu GET
menampilkan cap yang sama; ada test yang mengunci bentuknya.

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

### G14: State store OAuth tanpa test dan tanpa lantai versi

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
`GET`+`DEL` yang tetap atomik dan jalan di 6.0 ke atas.

**Definisi selesai.** Test store (stage, take, replay ditolak, TTL) hijau terhadap
Redis nyata; pilihan lantai versi tercatat; kegagalan store menghasilkan satu
baris log berkode.

## 4. Keputusan yang menunggu owner

| # | Pertanyaan | Pilihan | Dampak kalau ditunda |
|---|---|---|---|
| D1 | Guard egress upstream? (G2/G3) | (a) guard + allowlist, (b) trusted + catat | dial internal tetap mungkin dari konfigurasi salah |
| D2 | Proxy settings dihormati di jalur dial? (G4) | (a) wire, (b) amend spec | §7.11 separuh benar |
| D3 | Apa yang dicatat panggilan media? (G6) | (a) usage+log, (b) +request_count, (c) combo test tetap tanpa row | layar Usage/Logs panel kosong untuk media |
| D4 | Lantai Redis untuk state OAuth? (G14) | (a) `GETDEL` + catat Redis ≥ 6.2, (b) `EVAL` Lua agar 6.0 ikut jalan | host Redis < 6.2 kehilangan rute callback |
| D5 | Bagaimana cap dibaca kembali? (G12) | (a) sintesis window, (b) field cap di rute baca, (c) amend spec | panel tidak bisa menampilkan budget tersimpan |

## 5. Urutan kerja usulan

1. **P2.1, G1**: verifikasi live sisa permukaan. Selesai 2026-09-19.
2. **P2.2, G2 + G3**: satu keputusan D1, satu implementasi guard.
3. **P2.3, G6**: keputusan D3 lalu sambungkan `Outcome()` ke recorder.
4. **P2.4, G4**: keputusan D2 lalu wire atau amend.
5. **P2.5, G14**: temuan G1 yang tersisa, butuh D4 untuk pilihan lantai versi
   (testnya bisa ditulis lebih dulu).
6. **P2.6, G12 + G13 + G5**: G12 butuh D5, G13 dan G5 adapter/paritas.
7. **P2.7, G15 + G16**: temuan click-through G13 (routing `no_auth` dan header
   media kosong), tanpa keputusan owner. Selesai 2026-09-19.
8. **P2.8, G7, G8, G9, G10**: penutup kecil + dokumen.

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
