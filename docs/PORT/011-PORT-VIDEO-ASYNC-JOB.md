# 011-PORT-VIDEO-ASYNC-JOB.md: Provider tokenrouter dan job video async, dua route, tiga dialek

Pass `app-serv` saja. Permintaan owner tiga butir: **(1) tambahkan provider `tokenrouter`**, **(2) support
endpoint video generation**, **(3) kerjakan di BE dulu** ("Kita kerjakan di BE dulu (app-serv)"), jadi panel
`app-ui` tidak disentuh di pass ini.

|                      |                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| -------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**           | **CLOSED 2026-10-12.** Provider `tokenrouter` masuk registry (35 provider), kind `video` kini punya tiga provider, dan job video async dilayani sepasang route dengan envelope ternormalisasi. Seluruh gerbang hijau di §5 (paruh Go dan paruh panel); dua commit lokal, belum di-push.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| **Mechanism**        | DURING & AFTER (antislop)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| **Scope**            | `app-serv/tools/registry-gen.mjs`; `internal/registry/{registry.yaml,testdata/*,provider_tokenrouter_test.go,embedded_test.go}`; `internal/reasoning/{applier.go,apply.go,apply_vendor_test.go,applier_test.go}`; `internal/schema/{media_video.go,media_video_test.go,media_image.go}`; `internal/dataplane/{selection_candidates.go,selection_pin_test.go,engine_relay.go}`; `internal/service/media_{video.go,video_read.go,call.go,perform.go,ports.go}` + empat fixture test; `internal/handler/{media_video.go,media_image.go,media_generation_test.go,media_guard_test.go,media_stub_test.go,skills.go,skills_test.go,openapi_error_test.go,openapi_dto_registry_test.go}`; `internal/router/{router_media.go,router_media_dataplane_test.go,router_media_stub_test.go,router_session_sweep_test.go}`; `docs/CONTRACT/001-CONTRACT-API-V1.{yaml,md}` + `internal/handler/openapi.json`; `docs/SPEC-API/001-SPEC-API.md`; `SYSTEM_MAP.md`; `README.md`; `app-serv/README.md`; `skills/pannelai-image/SKILL.md` |
| **Permintaan owner** | (1) Provider `tokenrouter` di registry. (2) Endpoint video generation disupport. (3) BE dulu. Tiga jawaban terstruktur yang mengikat arah: respons video **dinormalisasi** (bukan passthrough), cakupan route **generations + poll**, dan blok video tokenrouter **ya, ditambahkan** walau reference tidak mendeklarasikannya                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| **Kaitan**           | SPEC-API §7.10 (diamendemen di pass ini), `docs/DRAFT/001-P2-GAPS.md` G7, `docs/DRAFT/027-APP-SERV-CONCEPT-PARITY.md` (baris video), PORT 008/009 (mesin proxy + pengikatan per provider), AGENTS.md §1.1/§1.2/§1.5/§2.1/§2.4, `docs/RULLES/TDD.md`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| **Tanggal**          | 2026-10-12                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| **Reference**        | `/home/rusmanadodi/backups/9router-39e36d3d` (pin, v0.5.86). Live HEAD `a99cf572` (v0.5.95) diukur untuk satu hal saja: apakah tokenrouter mendapat `videoConfig` di revisi yang lebih baru, dan jawabannya tidak                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |

## 1. Ringkasan: yang terukur sebelum touch, dan yang berubah

| Yang owner minta          | Yang terukur di pohon sebelum pass ini                                                                                                                                                                                                                                                                                                             |
| ------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Provider `tokenrouter`    | **Tidak ada.** 34 provider di registry (KEEP-set owner 2026-09-26); `tokenrouter` bukan salah satunya, padahal reference mendeklarasikannya lengkap (`open-sse/providers/registry/tokenrouter.js`: 22 model, `embeddingConfig`, `imageConfig`, `thinkingFormat: "tokenrouter"`)                                                                    |
| Endpoint video generation | **Route-nya ada, provider-nya nol.** `POST /api/v1/videos/generations` terdaftar sejak pass media pertama dan menjawab `PROVIDER_NOT_ROUTABLE`, karena tidak ada satu pun provider yang mendeklarasikan kind `video`. Blok `video` milik `xai` dan `openrouter` sudah ada di registry hasil generator, **tetapi keduanya tidak punya `auth_type`** |
| "BE dulu"                 | Panel tidak disentuh; tidak ada berkas `app-ui/` yang berubah                                                                                                                                                                                                                                                                                      |

Yang berubah di pass ini, dalam satu kalimat: kind `video` kini punya tiga provider (`xai`, `openrouter`,
`tokenrouter`), rutenya menjadi sepasang (create + poll) dengan envelope ternormalisasi, dan dialect setiap
provider dibaca dari satu tabel.

### 1.1 Kenapa blok `video` yang sudah ada tetap tidak cukup

`applyMediaCredential` (`internal/dataplane/media_credential.go:31`) berhenti lebih awal ketika blok tidak
mendeklarasikan `auth_type`, dan tidak menulis kredensial apa pun. Jadi blok `xai`/`openrouter` sebelum pass
ini akan berdial **tanpa kredensial**. Ini cacat laten, bukan cacat hidup: route-nya memang belum bisa
mencapai provider mana pun (`PROVIDER_NOT_ROUTABLE`), jadi tidak ada panggilan nyata yang pernah salah.
Generator sekarang menuliskan `auth_type: apikey` dan `auth_header: bearer` ke **setiap** blok video, dan
generated diff-nya menyentuh tepat dua blok lama.

## 2. Keputusan pass ini

| #   | Pertanyaan                                 | Arah                                                                                                                                                                 | Konsekuensi                                                                                                                                                                                                                                                                                                          |
| --- | ------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| D1  | Bentuk respons route video                 | **Dinormalisasi** (jawaban owner). Reference justru proxy: `videoCore` mengembalikan body upstream apa adanya                                                        | Satu envelope `{task_id, status, url?, progress?, error?}` untuk tiga provider; dialect per provider dipetakan di `media_video_read.go`. Ini penyimpangan sengaja dari reference, bukan paritas                                                                                                                      |
| D2  | Cakupan route                              | **Generations + poll** (jawaban owner): `POST /api/v1/videos/generations` dan `GET /api/v1/videos/{task_id}`                                                         | Reference mengenal tiga action (`generations`/`edits`/`extensions`) dan satu route GET; dua action sisanya tidak diporting dan tidak didaftarkan, jadi tidak ada route yang menjawab 404 sendiri                                                                                                                     |
| D3  | Blok video `tokenrouter`                   | **Ditambahkan** (jawaban owner), walau reference tidak punya `videoConfig` untuknya di revisi pin **maupun** di HEAD `a99cf572`                                      | Bentuk wire-nya diambil dari **dokumentasi vendor** (`https://www.tokenrouter.com/docs/how-to-use-video-models/`, diukur 2026-10-12), bukan dari reference. Header registry mencatat penyimpangan ini: `# Media blocks app-serv declares beyond the reference: tokenrouter.`                                         |
| D4  | Dialect per provider                       | Satu tabel `videoPlans`, **default = bentuk xAI**, karena itu default reference (`videoProviders/index.js:3-5`)                                                      | `xai` memakai default (`POST {base}/generations`, poll `GET {base}/{id}`); `openrouter` membuat POST ke root koleksi tanpa sufiks (`openrouter.js:33-34`); `tokenrouter` memakai `base_url` yang sudah berupa alamat create, jadi `createPath` kosong                                                                |
| D5  | Apakah poll menulis health endpoint        | **Tidak** (`MediaCall.SkipHealth`). Reference justru memarkir akun pada poll yang gagal, karena aturan 404-nya masuk daftar status akun (`accountFallback.js:56-58`) | Task id basi yang menjawab 404 tidak memarkir kunci sehat. Alasannya sama dengan yang reference sendiri tulis untuk 400 request-scoped: kesalahan itu tentang request, bukan tentang kredensial. Penyimpangan sengaja, dan hanya separuh: baris usage + log tetap ditulis untuk poll                                 |
| D6  | Cara klien memaku poll ke akun pembuat job | Header `X-PannelAI-Endpoint-Id` pada create + `?endpoint=` pada poll; `provider` **wajib** di poll                                                                   | Reference memakai `x-connection-id` (header) dan `resolveGetProvider` dengan fallback default xAI (`videoGeneration.js:25-33`). Terjemahannya sama niatnya, beda ejaan; dan poll tanpa `?endpoint=` memakai seleksi provider yang biasa, jadi tidak ada pin yang mengikat lebih kuat daripada yang reference lakukan |

## 3. Temuan

### 3.1 F1 (HIGH, ditutup): blok video lama tidak memasang kredensial

Lihat §1.1. Ditutup di generator, bukan di registry hasil (registry adalah artefak: yang diedit generator).
`provider_tokenrouter_test.go` mengunci blok video tokenrouter berikut auth-nya, dan diff registry
menunjukkan dua blok lama ikut mendapat dua baris yang sama.

### 3.2 F2 (HIGH, ditutup): kind `video` kosong membuat route hanya bisa menolak

Draft 001 G7 mencatatnya sebagai keputusan sadar ("biarkan terdokumentasi, paritas reference"). Pass ini
mengubah premisnya karena owner memintanya: tiga provider mendeklarasikan `video`, dan `embedded_test.go`
mengunci jumlah provider di 35.

### 3.3 F3 (MEDIUM, residu): adapter `vertex` (Veo) tidak diporting

Reference punya tiga provider ber-`videoConfig`: `xai`, `openrouter`, `vertex`
(`open-sse/providers/registry/{xai,openrouter,vertex}.js`), dan dua adapter: `openrouter` dan `vertex`.
Adapter `vertex` bukan variasi dialek: ia menerjemahkan dua arah (`:predictLongRunning` dengan
`instances[]`/`parameters{}`, lalu `:fetchPredictOperation` dengan `operationName`), dan job id-nya adalah
resource path yang di-base64url-kan supaya `GET /videos/{id}` tetap satu segmen. Itu pekerjaan bentuk kedua,
bukan baris di tabel dialek. Registry **tidak** mendeklarasikan `video` untuk `vertex`, jadi provider itu
ditolak dengan nama (`PROVIDER_NOT_ROUTABLE`), bukan didial dengan payload yang salah bentuk.

### 3.4 F4 (MEDIUM, keputusan owner): daftar level picker dan enum vendor tokenrouter tidak sepakat

Diukur dari corpus kapabilitas: 22 model tokenrouter membawa format kapabilitas keluarga
(`openai` 6, `claude-adaptive` 3, `qwen` 3, `zai` 3, `deepseek` 2, `gemini-level` 2, `kimi` 2,
`claude-budget` 1). Tabel level menjawab dari format itu, sementara **wire**-nya sekarang `tokenrouter`
(entri provider menang, sesuai presedensi reference `thinkingUnified.js:112-113`). Akibatnya 11 dari 22
model menawarkan level yang enum vendor tidak terima: `minimal` untuk 6 model `openai` + 2 model
`gemini-level` (`openai/gpt-5.4`, `…-mini`, `…-pro`, `gpt-5.5`, `gpt-5.6-sol`, `x-ai/grok-4.5`,
`google/gemini-3.5-flash`, `google/gemini-3.6-flash`) dan `thinking` untuk 3 model `zai`
(`z-ai/glm-5.2`, `…-turbo`, `…-3-free`). `toLevel` meneruskan level apa adanya ke `reasoning_effort`, jadi
pilihan itu berakhir 400 di upstream.

Ini **paritas, bukan regresi**: reference melakukan hal yang sama (presedensi yang sama, tabel yang sama).
Yang tidak dilakukan pass ini: menambal tabel itu, karena ia cermin reference dan menambal satu barisnya
mengubah jawaban untuk provider lain juga. Keputusan owner yang terbuka: apakah tabel level untuk wire
`tokenrouter` dipersempit ke `low/medium/high/xhigh/max` (plus `none` yang berarti "tidak menulis field").

Satu hal yang **tidak** terjadi, dan ini juga diukur: tidak ada satu pun model tokenrouter yang format
kapabilitasnya `tokenrouter`, jadi fallback `L.base` yang akan muncul kalau tabel tidak punya baris
(`thinkingLevels.js:73`) tidak bisa dijangkau hari ini (0 dari 22).

### 3.5 F5 (LOW, ditutup): lima klaim hidup yang menjadi salah begitu pass ini mendarat

Dua di antaranya dilayani gateway, sisanya dokumen hidup yang dibaca orang berikutnya:

| Klaim                                                                  | Di mana                                                                              | Jadi apa                                                                                                                  |
| ---------------------------------------------------------------------- | ------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------- |
| "Video is registered but not served … answers `PROVIDER_NOT_ROUTABLE`" | `skills/pannelai-image/SKILL.md:50-54` (dokumen yang dilayani katalog §7.16)         | Diganti deskripsi job async: dua route, header, empat keadaan, dan aturan bahwa keadaan tak dikenal diteruskan apa adanya |
| Deskripsi katalog tanpa video                                          | `internal/handler/skills.go:69`                                                      | Menyebut job video async; tidak ada test yang mengunci kalimat itu, diperiksa dengan grep                                 |
| "34 provider" (tiga tempat)                                            | `SYSTEM_MAP.md:8`, `README.md:19`, `app-serv/README.md:25`, plus prosa `SPEC-API §6` | 35, dengan sebabnya disebut (permintaan owner 2026-10-12)                                                                 |
| "Enam rute data plane media" + daftar enam path                        | `SYSTEM_MAP.md §3.6`                                                                 | Tujuh rute, dan paragraf pertama §3.6 kini menyebut job async + `SkipHealth`                                              |
| Ringkasan changelog lama yang menyebut penolakan video                 | `SPEC-API` changelog 2026-09-19, draft 001 G7, draft 027                             | **Dibiarkan**: ketiganya catatan bertanggal, bukan keadaan hidup (F6)                                                     |

`SYSTEM_MAP.md` sebenarnya bukan N/A untuk pass ini, dan itu ketahuan dari membaca peta itu sendiri
sebelum menulis baris checklist-nya: peta menyebut jumlah provider dan mendaftar rute media satu per satu,
jadi perubahan ini menyentuhnya.

### 3.6 F6 (LOW, ditutup dengan sengaja tidak disentuh): dua register DRAFT masih menyebut premis lama

`docs/DRAFT/001-P2-GAPS.md` G7 (CLOSED 2026-09-19) dan `docs/DRAFT/027-APP-SERV-CONCEPT-PARITY.md` §baris
video keduanya mencatat "tidak ada provider yang mendeklarasikan `video`" sebagai keadaan saat itu.
Keduanya **dibiarkan apa adanya**: keduanya catatan temuan bertanggal, bukan daftar keadaan hidup, dan
konvensi register di pohon ini adalah menambah catatan baru (PORT) alih-alih menulis ulang temuan lama.
G7 bahkan sudah mengantisipasi perubahan ini di kalimat penutupnya ("kalau kelak ada provider ber-kind
`video`, adapter masuk G5 dan bukan gap ini").

## 4. Implementasi

**Route.** `POST /api/v1/videos/generations` membuat job dan menjawab envelope; `GET /api/v1/videos/{task_id}`
membacanya. Keduanya data-plane (kunci gateway, §4), keduanya terdaftar di `router_media.go` di sebelah
route media lain, dan keduanya masuk tabel pengecualian session-sweep dengan alasan tertulis.

**Request.** `schema.VideoRequest` hanya membaca dua anggota (`model`, `prompt`); sisanya milik provider dan
diteruskan apa adanya. `schema.RewriteVideoModel` mengganti satu anggota (`model` menjadi id upstream) dan
mengembalikan body byte-identik ketika modelnya sudah sama, yang merupakan aturan reference sendiri
(`videoGeneration.js:120-124`).

**Dialect.** `videoPlanFor` menjawab satu baris tabel per provider: `openrouter` (reader OpenRouter),
`tokenrouter` (reader TokenRouter), default xAI. Tiga reader memetakan tiga vocabulary status ke empat
keadaan kanonik (`queued`, `in_progress`, `succeeded`, `failed`); keadaan yang tidak dikenal diteruskan
lowercase, tidak ditebak menjadi terminal, supaya klien yang menunggu keadaan akhir tidak pernah diberi tahu
job selesai padahal tidak.

**Poll.** `preparePinned` menyelesaikan blok tanpa memanggil `Select` (cursor rotasi tidak bergerak karena
panggilan ini memaku akunnya sendiri), lalu `selectForPoll` memakai pin kalau ada dan seleksi provider kalau
tidak. Task id di-escape sebagai **satu** segmen path (`url.PathEscape` + `RawPath`), yang merupakan
`encodeURIComponent` reference (`videoCore.js:34-36`); id yang membawa `/` tidak boleh menjadi path yang
lebih dalam.

**Reasoning.** Provider yang mendeklarasikan `thinking_format` di entri transport-nya kini dipercaya
sebagai dialect wire: `Call.ProviderThinkingFormat` dibawa dari resolusi provider (`engine_relay.go`), dan
`resolveFormat` menaruhnya **sebelum** jawaban kapabilitas model, yang merupakan presedensi reference
(`thinkingUnified.js:112-113`). Perilaku tiga provider yang sudah mendeklarasikannya tidak berubah
(`openrouter`, `codebuddy-cn`, `codebuddy-intl`, ketiganya `openai`, yang juga jawaban lama mereka), dan
`tokenrouter` mendapat dialect-nya sendiri: satu case baru di `applyFormat` menulis `reasoning_effort` dan
menolak menulis `none` maupun `auto`, persis case reference (`thinkingUnified.js:333-341`). Wire
`openai-responses` tetap menang atas deklarasi mana pun, karena port ini tidak membawa executor yang
menulis ulang anggota tunggal wire itu.

**Envelope yang gagal.** Sebuah create yang menjawab 200 tanpa id job ditolak
(`the video upstream accepted the job but named no video job to poll`), karena klien tidak punya apa pun
untuk di-poll; poll yang menjawab tanpa keadaan ditolak dengan kalimat yang sama jelasnya. Keduanya lewat
`Perform`, jadi baris usage-nya tercatat sebagai gagal, bukan sebagai dilayani (aturan G17 media plane).

## 5. Gerbang

Diukur pada pohon beku `app-serv`: sidik jari `4311d591eb553451873f59593a533801` (`find` atas
`*.go`/`*.yaml`/`*.json`/`*.mjs`, satu md5 dari seluruh md5sum), dan nol berkas `app-ui/` berubah.
Tabel routing `scrypts/lib/gate-scope.sh` menjalankan **kedua** paruh untuk change set ini, karena
`docs/CONTRACT/*`, `internal/schema/*`, `internal/handler/openapi.json`, dan dokumen akar semuanya
masuk ember "both".

| Gerbang               | Hasil                                                                                                                                                                                                                                        |
| --------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `gate-scope-test.sh`  | PASS, tabel routing utuh                                                                                                                                                                                                                     |
| `go-lint.sh`          | PASS: `go vet` dan `go vet -tags=integration,live`, gofmt, staticcheck (dua tag), golangci-lint 0 isu                                                                                                                                        |
| `go-headers.sh`       | PASS: 1260 berkas Go membawa header lengkap                                                                                                                                                                                                  |
| `antislop.sh`         | PASS seluruh pemeriksaan                                                                                                                                                                                                                     |
| `go-test.sh` (race)   | EXIT 0, 20 paket `ok` dalam 3m38s. Suite integrasi ber-tag **dilewati** karena `PANNELAI_TEST_POSTGRES_DSN` tidak diekspor di shell ini: dicatat sebagai skip, bukan pass                                                                    |
| `panel-check.sh`      | PASS seluruhnya: prettier, eslint, svelte-kit sync, svelte-check, vitest (3014 test di 186 berkas, 1325,95s), dan vite build. Nol berkas `app-ui/` berubah; paruh ini tetap dijalankan karena kontrak dan dokumen akar termasuk ember "both" |
| `secrets.sh --all`    | PASS: nol kebocoran di commit dan di berkas yang akan dibawa push                                                                                                                                                                            |
| `contract-drift.sh`   | PASS: 13 kode error sepakat di kedua sisi                                                                                                                                                                                                    |
| `contract-openapi.sh` | PASS: dokumen yang dilayani sama dengan kontrak YAML                                                                                                                                                                                         |

Dua kegagalan nyata muncul di jalannya gerbang dan ditutup di pass ini juga, bukan diwariskan:
`liveKeylessRouter` (berkas uji ber-tag `integration,live`) tidak lagi mengimplementasikan
`MediaRouter` setelah port itu mendapat `SelectEndpoint`, dan satu saran gocritic (`stringXbytes`) di
`media_video_test.go`. Keduanya menyentuh berkas Go, jadi pohon dibekukan ulang dan **seluruh** gerbang
di atas dijalankan lagi di atas sidik jari yang tercatat di atas, bukan diwarisi dari jalannya yang
pertama.

Peringatan yang dibiarkan, dengan alasan: enam berkas yang disentuh pass ini berada di band 220-250
baris (226 sampai 239). Ambang keras §1.1 adalah 250 dan tidak ada yang melewatinya; memecah berkas
berjalan (empat di antaranya berkas lama yang tumbuh karena port reasoning) adalah pekerjaan bentuk
ketiga yang tidak diminta pass ini, jadi dicatat sebagai residu §6, bukan dikerjakan sekarang.

Sebelum commit, berkas yang disentuh diperiksa di level byte, dan pemeriksaan itu menangkap satu
kerusakan nyata: `SYSTEM_MAP.md` sempat ditulis ulang dengan 22 panah `→` menjadi byte tidak valid
(`E2 86 3F`). Berkas itu dipulihkan dari HEAD dan dua suntingan yang dimaksud diterapkan ulang secara
byte-preserving, jadi yang di-commit adalah UTF-8 valid dengan 22 panah utuh. Sapuan yang sama atas
seluruh berkas yang akan di-commit (50 berkas, terlacak dan baru) tidak menemukan tanda kerusakan lain,
dan tidak ada test yang membaca dokumen prosa, jadi hasil gerbang di atas tidak terpengaruh.

## 6. Residu

1. **Adapter `vertex` (Veo)** (F3): dua request shape + kredensial service account; provider-nya sengaja
   tidak didaftarkan sebagai video, jadi tidak ada route yang menebak.
2. **Tabel level vs enum vendor tokenrouter** (F4): keputusan owner; hari ini 11 dari 22 model menawarkan
   level yang upstream tolak.
3. **Tidak ada failover pada create.** `Perform` adalah satu percobaan (`media_perform.go:55`), jadi create
   video tidak berputar ke akun lain. Reference justru berputar pada 401/403/429 saja dan **tidak**
   mengulang pada 5xx, karena job mungkin sudah terbuat (`videoGeneration.js:35-41`). Pohon ini tidak
   mengulang sama sekali, jadi tidak ada risiko job ganda; yang hilang adalah failover-nya.
4. **`edits`/`extensions`**: tidak ada route dan tidak ada dialect; reference pun hanya mengizinkan
   `generations` untuk OpenRouter (`openrouter.js:9`).
5. **Panel belum tahu video.** `app-ui` tidak disentuh di pass ini sesuai permintaan owner; halaman media
   panel belum merender kind `video`, dan tidak ada test panel yang menuntutnya.
6. **Enam berkas di band peringatan 220-250 baris** (§5): dua berkas baru (`media_video_read.go` 233,
   `media_video_test.go` 239) dan empat berkas lama yang tumbuh karena port reasoning
   (`engine_relay.go` 237, `applier.go` 226, `apply.go` 236, `applier_test.go` 239). Semuanya di bawah
   ambang keras 250; pemecahannya belum dikerjakan dan tidak diminta pass ini.

## 7. Checklist AGENTS.md per-PR

| Butir                     | Keadaan                                                                                                                                                                                                                                                                                                                                                                                                            |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| §1.1 ambang 250 baris     | Berkas baru, diukur dengan `wc -l`: `media_video.go` (service) 186, `media_video_read.go` 233, `media_video_test.go` (service) 239, `media_video.go` (handler) 86, `media_video.go` (schema) 99, `media_video_test.go` (schema) 111, `selection_pin_test.go` 160, `provider_tokenrouter_test.go` 88. Tidak ada yang melewati ambang keras 250; enam berkas di band peringatan 220-250 dicatat di §5 dan §6 butir 6 |
| §1.2 header per berkas    | Setiap berkas baru membawa header lengkap; `go-headers.sh` hijau di pass gerbang                                                                                                                                                                                                                                                                                                                                   |
| §1.3 bahasa error         | Semua kalimat client-facing berbahasa Inggris ("query parameter provider is required", "the video upstream accepted the job but named no video job to poll")                                                                                                                                                                                                                                                       |
| §1.4 nol `any`            | Reader memakai struct bertipe per provider; `json.Unmarshal` hanya di boundary (schema, reader, rewrite) dengan target konkret                                                                                                                                                                                                                                                                                     |
| §1.5 batas layer          | `service` tidak mengimpor `net/http`; dialect dan reader hidup di `service`, bentuk wire di `dataplane`, kontrak di `schema`; tidak ada import lintas-service                                                                                                                                                                                                                                                      |
| §2.1 test                 | TDD: `selection_pin_test.go`, `media_video_test.go`, `media_video_test.go` (schema) dan `provider_tokenrouter_test.go` ditulis bersama implementasinya; tidak ada `t.Skip`, tidak ada filter `-run` yang di-commit                                                                                                                                                                                                 |
| §2.4 CDD                  | Kontrak YAML berubah lebih dulu (`VideoRequest` kini `model`+`prompt` wajib dengan `additionalProperties: true`, schema `VideoJob` baru, path poll baru), lalu `openapi.json` diregenerasi dari YAML, lalu `dtoRegistry` + daftar data-plane di test ikut menuntut keduanya                                                                                                                                        |
| §1.9 sinkronisasi dokumen | SPEC-API §7.10 (tabel route + paragraf job async + changelog) dan prosa §6 (jumlah provider), kontrak YAML + tabel rute MD + `openapi.json`, `SYSTEM_MAP.md` (jumlah provider + §3.6 tujuh rute + paragraf job async), `README.md` dan `app-serv/README.md` (jumlah provider), dokumen skill yang dilayani. Tidak ada perubahan boundary/struktur data/topology: tidak ada tabel, migrasi, worker, atau paket baru |
