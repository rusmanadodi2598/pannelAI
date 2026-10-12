# 012-PORT-VERTEX-VEO.md: Provider vertex (Veo), tipe auth service_account, dan adapter video dua arah

Pass `app-serv` **dan** `app-ui`. Permintaan owner satu kalimat: **"Perbaiki registrasi provider vertex Veo"**,
dengan empat jawaban terstruktur yang mengikat arah (adapter penuh, video saja, kolom/tipe auth baru,
`b64_json` opsional). Pass ini menutup residu butir 1 PORT 011: adapter `vertex` (Veo) yang saat itu
sengaja tidak diporting.

|                      |                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**           | **CLOSED 2026-10-12.** Provider `vertex` masuk registry sebagai provider ke-36 dengan empat model Veo (kind `video` saja) dan tipe auth keempat `service_account`; adapter video dua arah (`predictLongRunning` + `fetchPredictOperation`) hidup di `service`, dan panel mengenal tipe auth itu di kedua jalur create. Seluruh gerbang hijau di §5. Dua commit lokal, belum di-push.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| **Mechanism**        | DURING & AFTER (antislop)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| **Scope**            | `app-serv/tools/registry-gen.mjs`; `internal/registry/{registry.yaml,load.go,embedded_test.go,provider_vertex_test.go,testdata/*}`; `migrations/000016_service_account_endpoints.{up,down}.sql`; `internal/domain/{upstream_endpoint.go,upstream_endpoint_types.go,upstream_endpoint_keys.go,upstream_endpoint_service_account.go}` + tiga berkas uji; `internal/repository/postgres/{endpoint_row.go,endpoint_batch.go,endpoint_active_integration_test.go}`; `internal/schema/{endpoint.go,endpoint_response.go,media_video.go}`; `internal/service/{endpoint.go,endpoint_create.go,endpoint_bulk.go,media_call.go,media_shape.go,media_video.go,media_video_plan.go,media_service_account.go,media_video_vertex.go,media_video_vertex_id.go,media_video_vertex_read.go}` + enam berkas uji/stub; `internal/provider/{vertex_jwt.go,vertex_token.go}` + dua berkas uji; `internal/handler/{endpoint.go,endpoint_bulk.go,endpoint_response.go,endpoint_response_service_account_test.go,openapi.json}`; `internal/dataplane/{credential.go,selection_candidates.go}`; `cmd/app-serv/dataplane_media_wiring.go`; `docs/CONTRACT/001-CONTRACT-API-V1.{yaml,md}`; `docs/SPEC-API/001-SPEC-API.md`; `docs/PORT/011-PORT-VIDEO-ASYNC-JOB.md`; `SYSTEM_MAP.md`; `README.md`; `app-serv/README.md`; `app-ui/README.md`; `app-ui/src/lib/{schemas/endpoint.ts,schemas/endpoint-write.ts,schemas/endpoint-credential.ts,utils/routing.ts,api/endpoints.ts,api/provider-keys.ts,components/{CreateEndpointForm,AddProviderKeysDialog,ProviderKeyFields,BulkAddKeysForm,AddEndpointKeyForm,ProviderConnectionsSection}.svelte}` + enam berkas uji panel |
| **Permintaan owner** | "Perbaiki registrasi provider vertex Veo." Empat jawaban terstruktur yang mengikat: **adapter penuh** (Veo harus benar-benar bekerja, bukan hanya terdaftar), **video saja** (empat model Veo; empat model Gemini chat tidak diporting), **kolom/tipe auth baru** (`service_account` + `sa_project_id`/`sa_location`, JSON SA tersegel di `upstream_keys`), **`b64_json` opsional** di envelope video ternormalisasi                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| **Kaitan**           | PORT 011 §6 residu 1 dan §3.3 F3 (ditutup di sini); SPEC-API §6, §7.5, §7.10 (diamendemen) dan changelog; `docs/CONTRACT/001-CONTRACT-API-V1.{yaml,md}`; AGENTS.md §1.1/§1.2/§1.3/§1.4/§1.5/§1.6/§1.9/§2.1/§2.4; `docs/RULLES/TDD.md`; `docs/RULLES/SSRF.md` §2.1 (minter memakai klien egress-guard yang sama)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| **Tanggal**          | 2026-10-12                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| **Reference**        | `/home/rusmanadodi/backups/9router-39e36d3d` (pin, v0.5.86): `open-sse/providers/registry/vertex.js` (entri registry: 4 model chat + 4 model video, `serviceKinds: [llm, imageToText, video]`, `videoConfig`) dan `open-sse/handlers/videoProviders/vertex.js` (159 baris; adapter dua arah + kodek job id)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |

## 1. Ringkasan: yang terukur sebelum touch, dan yang berubah

| Yang owner minta            | Yang terukur di pohon sebelum pass ini                                                                                                                                                                                                                                                                                     |
| --------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Provider `vertex` terdaftar | **Tidak ada.** 35 provider di registry; reference mendeklarasikan `vertex` lengkap (4 model Gemini chat + 4 model Veo, `serviceKinds` tiga kind, `videoConfig`) tetapi entri itu bukan bagian KEEP-set owner 2026-09-26                                                                                                    |
| Adapter video `vertex`      | **Tidak ada.** PORT 011 §3.3 F3 mencatatnya sebagai pekerjaan bentuk kedua (dua request shape + kredensial service account) dan §6 residu 1 menahannya. Karena registry tidak mendeklarasikan `video` untuk `vertex`, provider itu ditolak dengan nama (`PROVIDER_NOT_ROUTABLE`), bukan didial dengan payload salah bentuk |
| Tempat kredensial SA        | **Tidak ada.** Tiga auth type (`api_key`, `oauth`, `no_auth`); JSON service account bukan plain key maupun token set yang bisa di-refresh, jadi tidak ada type yang benar untuknya, dan tidak ada kolom untuk project/location yang alamat panggilannya butuhkan                                                           |
| Envelope video              | **Tanpa anggota bytes.** `{task_id, status, url?, progress?, error?}` (PORT 011 D1); Veo tanpa `storageUri` menjawab `bytesBase64Encoded` inline, jadi tanpa anggota baru asset itu tidak punya tempat di envelope                                                                                                         |

Yang berubah di pass ini, dalam satu kalimat: `vertex` menjadi provider ke-36 (katalog empat model Veo,
kind `video` saja), gateway mendapat tipe auth keempat `service_account` dengan dua kolom non-secret dan
minter token (JWT RS256 → jwt-bearer) di seam kredensial media, adapter video `vertex` hidup dua arah
(create + poll + kodek job id + reader), envelope mendapat `b64_json?`, dan panel mengenal tipe auth baru
itu di kedua jalur create dengan label credential-nya sendiri.

### 1.1 Kenapa `videoConfig` reference saja tidak cukup untuk blok media

`videoConfig` reference hanya menamai `baseUrl`; proxy videonya selalu membangun `Authorization: Bearer`
dari access token akun (`videoCore.js:39-45`), dan token itu didapat dari mekanisme lain di luar blok.
app-serv menyimpan JSON service account sebagai kredensial (satu baris `upstream_keys` tersegel) dan
menukarnya menjadi bearer **sebelum** panggilan, jadi blok video vertex harus menamai `service_account`,
bukan bearer polos. Generator mendapat tabel baru `APP_SERV_MEDIA_OVERRIDES` untuk itu: ia menimpa satu
field blok yang reference **memang** deklarasikan (berbeda dari `APP_SERV_MEDIA_EXTENSIONS` yang mengisi
kind yang reference tinggalkan kosong), karena dua klaimnya berlawanan: extension mengalah pada blok
reference yang muncul belakangan, override menang atas nilai reference untuk field yang dinamainya.

### 1.2 Kenapa empat model chat tidak ikut (D2)

Reference mendeklarasikan empat model Gemini chat di entri yang sama, dan `serviceKinds`-nya menyebut
`llm`. Wire-nya `vertex`, yang tidak punya translator chat di gateway ini (`registry.ChatRoutability`
menandai entri ber-format `vertex` sebagai butuh connector), jadi mendaftarkan model chat itu berarti
menawarkan permintaan yang selalu gagal. Model yang sama sudah dilayani provider `gemini`
(`apikey`/`oauth`) di katalog yang sama. Generator karena itu mendapat `APP_SERV_MODEL_KINDS`: filter
katalog per provider yang menyempitkan `vertex` ke `kind: video` saja; model yang tidak mendeklarasikan
kind adalah model chat, jadi filter yang menamai `video` membuangnya. Ini penyimpangan sengaja dari
reference, dicatat di header registry dan di generator.

## 2. Keputusan pass ini

| #   | Pertanyaan                             | Arah                                                                                                                                                                                                       | Konsekuensi                                                                                                                                                                                                                           |
| --- | -------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| D1  | Seberapa jauh porting `vertex`?        | **Adapter penuh** (jawaban owner): Veo harus benar-benar bekerja, bukan hanya terdaftar                                                                                                                    | Dua request shape (`:predictLongRunning` + `:fetchPredictOperation`) plus kodek job id dan reader; bukan baris di tabel dialek                                                                                                        |
| D2  | Model yang didaftarkan                 | **Video saja** (jawaban owner): empat model Veo; empat model Gemini chat tidak diporting                                                                                                                   | `APP_SERV_MODEL_KINDS` menyempitkan katalog ke `kind: video`; alasan di §1.2                                                                                                                                                          |
| D3  | Bentuk kredensial                      | **Kolom/tipe auth baru** (jawaban owner): `service_account` + `sa_project_id`/`sa_location`; JSON SA tersegel di `upstream_keys`                                                                           | Migrasi 000016 melebarkan CHECK `auth_type` dan menambah dua kolom `NOT NULL DEFAULT ''`; pair divalidasi di aggregate (`SetServiceAccount`), bukan hanya di handler, karena project yang salah hanya terlihat sebagai URL yang salah |
| D4  | Byte video di envelope                 | **`b64_json` opsional** (jawaban owner), sejalan dengan reference yang juga mengirim `b64_json: s.bytesBase64Encoded \|\| s.video?.bytesBase64Encoded \|\| null` (`vertex.js:105`)                         | Satu anggota opsional di `VideoJob`; job yang menjawab URL meninggalkannya kosong                                                                                                                                                     |
| D5  | Kodek job id                           | Base64url dari operation name, dengan pertahanan tiga lapis: charset + panjang, re-encode byte-for-byte, bentuk operation ber-anchor + setiap segmen satu resource name (`decodeJobId`, `vertex.js:26-34`) | Id yang decoded-nya disisipkan ke URL tidak bisa menyelundupkan path walk; non-canonical spelling (spare bits di karakter terakhir) ditolak                                                                                           |
| D6  | Di mana JSON SA ditukar menjadi bearer | Di seam kredensial (`MediaCallService.bindCredential`, lewat `mintCredential`), bukan di route                                                                                                             | Setiap fase job (create dan poll) menukar dengan cara yang sama; deployment tanpa minter **menolak** panggilan service_account, bukan menyajikan JSON tersimpan sebagai bearer                                                        |

## 3. Temuan

### 3.1 F1 (residu PORT 011, ditutup): adapter `vertex` (Veo) tidak diporting

PORT 011 §3.3 F3 menahannya dengan alasan yang masih berlaku sampai pass ini: bukan variasi dialek,
melainkan penerjemah dua arah dengan kodek id. Pass ini memporting penuh: create, poll, kodek, reader,
dan blok media yang menamai kredensialnya. Butir 1 §6 PORT 011 ditandai tindak lanjut di berkas itu.

### 3.2 F2 (MEDIUM, ditutup): katalog chat reference tidak bisa dijawab gateway ini

Empat model Gemini di entri `vertex` berbicara wire `vertex` yang tidak punya translator chat; menjualnya
di katalog berarti setiap `POST /chat/completions` untuk model itu berakhir `PROVIDER_NOT_ROUTABLE`.
Ditutup di generator dengan `APP_SERV_MODEL_KINDS` (§1.2), dan `provider_vertex_test.go` mengunci katalog
video-only berikut jumlahnya.

### 3.3 F3 (MEDIUM, ditutup): blok `videoConfig` reference tidak menamai kredensial

`applyMediaCredential` berhenti ketika blok tidak mendeklarasikan `auth_type` (perilaku yang PORT 011 §1.1
temukan untuk xAI/OpenRouter), dan `videoConfig` vertex memang tidak menamainya. Generator sekarang
menimpanya dengan `APP_SERV_MEDIA_OVERRIDES` (§1.1), dan `provider_vertex_test.go` mengunci blok hasilnya:
`auth_type: service_account`, `auth_header: bearer`.

### 3.4 F4 (MEDIUM, ditutup): dua permukaan panel mengasumsikan `api_key`

Petunjuk "key terakhir tidak bisa dinonaktifkan" (`routing.ts`, `isLastActiveApiKey`) dan jalur key dialog
membaca `api_key` secara literal; endpoint `service_account` juga memegang kredensialnya di tabel keys
(`UsesStoredKey`), jadi keduanya salah untuk tipe baru itu. Ditutup dengan memperluas
`REQUIRES_KEY_AUTH_TYPES` dan `API_KEY_AUTH_TYPES`, dan satu aturan tambahan: kredensial service account
adalah **dokumen**, bukan string, jadi tab paste (satu kredensial per baris) tidak ditawarkan untuk tipe
itu; mode diturunkan dari tab yang dirender, sehingga mode basi tidak bisa submit lewat tab tersembunyi.

### 3.5 F5 (LOW, ditutup): empat berkas melewati ambang baris saat tumbuh

Gerbang Go **menggagalkan** berkas yang berubah di atas 250 baris (berkas uji termasuk); ambang panel
adalah aturan SPEC-UI §10. Yang dipecah di pass ini, semuanya di seam yang sudah ada:

| Berkas                                    | Sebelum | Sesudah | Berkas baru                                  |
| ----------------------------------------- | ------- | ------- | -------------------------------------------- |
| `schemas/endpoint-write.ts`               | 288     | 197     | `schemas/endpoint-credential.ts` (115)       |
| `components/AddProviderKeysDialog.svelte` | 258     | 229     | `components/ProviderKeyFields.svelte` (109)  |
| `service/media_video_vertex.go`           | 285     | 212     | `service/media_video_vertex_id.go` (97)      |
| `service/media_video_vertex_test.go`      | 264     | 215     | `service/media_video_vertex_id_test.go` (66) |

Pemecahan `endpoint-write.ts` mengikuti alasan yang sama dengan pemecahan `./endpoint` sebelumnya:
bentuk baca dan bentuk tulis adalah pekerjaan berbeda. `media_video_vertex_id.go` memisahkan kodek job id
(satu pekerjaan dengan rasionale keamanannya sendiri) dari dialeknya.

### 3.6 F6 (LOW, ditutup): lima temuan golangci-lint pada berkas baru

Gerbang `go-lint` menggagalkan lima temuan pada berkas yang pass ini tulis, dan semuanya diperbaiki pada
jalur yang sama:

| Temuan                                                               | Berkas                                   | Perbaikan                                                                                                                                                                                                              |
| -------------------------------------------------------------------- | ---------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `forbidigo` (`any` di luar boundary I/O yang dideklarasikan), 3 kali | `media_video_vertex.go`                  | Blok parameter menjadi struct bertipe `vertexPredictParameters` (pointer = knob absen), bukan `map[string]any`: nama parameternya tetap milik model sendiri, dan absence kini dinyatakan oleh tipe, bukan oleh `len()` |
| `gocritic offBy1` (Index bisa -1)                                    | `media_video_vertex_id.go`               | `vertexModelPath` memotong dengan `strings.Cut`: nama operasi sudah di-anchor dekodenya, jadi pemotong hadir tepat sekali dan tidak ada indeks -1 yang mungkin                                                         |
| `errchkjson` (nilai kembalian tidak dicek)                           | `internal/provider/vertex_token_test.go` | Helper `replacePrivateKey` menerima `*testing.T` dan menggagalkan uji bila kredensial yang ia tulis sendiri tidak bisa di-encode ulang                                                                                 |

Bentuk parameter yang bertipe juga lebih jujur dari peta: sebuah knob yang tidak ditulis klien kini
absen dari badan permintaan karena tipenya, bukan karena pemeriksaan jumlah kunci. Uji yang membaca badan
tersebut (`media_video_vertex_test.go:71`) tetap memakai `map[string]any` di sisi klien, karena di situ
ia memang sedang membaca JSON yang orang lain tulis.

## 4. Implementasi

**Registry dan generator.** `vertex` masuk KEEP-set (owner 2026-10-12) sebagai provider ke-36; generator
mendapat tiga tabel baru: `APP_SERV_PROVIDER_EXTENSIONS` (`auth_type`/`auth_modes`/`service_kinds`),
`APP_SERV_MEDIA_OVERRIDES`, dan `APP_SERV_MODEL_KINDS`. Entri hasilnya: priority 40, alias `vx`, kategori
`freeTier`, transport `format: vertex` + `base_url: https://aiplatform.googleapis.com`, empat model Veo
(3.1 preview, 3.1 fast preview, 3.0, 2.0) semuanya `kind: video`, blok `media.video` dengan
`auth_type: service_account`/`auth_header: bearer`, dan `service_kinds: [video]`. Korpus kapabilitas dan
pricing diregenerasi; `embedded_test.go` mengunci jumlah provider di **36**.

**Tipe auth keempat.** `domain.UpstreamAuthServiceAccount` + `UsesStoredKey()` (api_key **atau**
service_account) menggantikan pemeriksaan `== api_key` di tiga permukaan sekaligus (endpoint tanpa key
tidak bisa melayani, harus menjaga ≥1 key aktif, routing memilih key di dalamnya). `ServiceAccountRef`
adalah separuh non-secret; `SetServiceAccount` menolak endpoint bertipe lain, menolak field kosong, dan
menolak nilai yang bukan satu segmen path (`^[A-Za-z0-9._-]+$`, dot-name ditolak). Migrasi 000016
melebarkan CHECK menjadi `('api_key','oauth','no_auth','service_account')` (drop + add, idempoten untuk
boot yang mati separuh jalan) dan menambah `sa_project_id`/`sa_location` `NOT NULL DEFAULT ''` supaya
baris lama terbaca kosong, bukan gagal scan. Repository menulis/membaca kedua kolom di jalur satuan dan
batch; `schema` membawa keduanya di create, bulk, dan response (omitempty di response, sehingga hanya
endpoint SA yang membawanya).

**Minter token.** `provider.VertexToken` + `vertexTokenClient`: JSON SA diparse (`vertex_jwt.go`),
private key RSA dari dua bentuk PEM, assertion RS256 satu jam dengan scope `cloud-platform`, lalu
ditukar di `oauth2.googleapis.com/token` (grant `jwt-bearer`). Token di-cache per JSON dengan buffer 5
menit terhadap `expires_in` (default 1 jam), timeout 15 detik, batas body 1 MiB, dan klien egress-guard
yang sama dengan setiap upstream lain (yang SSRF.md §2.1 minta), bukan klien sendiri. Parse dan
signing hidup di satu berkas supaya material kunci punya tepat satu pembaca.

**Seam media.** `MediaCallServiceDeps.Tokens` (opsional) dan `mintCredential`: auth type bukan
`service_account` lewat tanpa sentuhan; tanpa minter = `InternalError`; JSON kosong = `ValidationError`;
mint gagal = `UpstreamRejected(502)` dengan alasan Google. Dipanggil dari `bindCredential` sehingga
setiap fase job menukar dengan cara yang sama. `mediaFormatSupported` mengenal `vertex` untuk kind
`video`.

**Adapter.** `media_video_plan.go` adalah tabel dialek per provider; baris `vertex` menamai tiga
pembangun/pembaca. Create: `POST {base}/v1/projects/{p}/locations/{l}/publishers/google/models/{m}:predictLongRunning`
dengan body diterjemahkan `{instances:[{prompt, image?, video?}], parameters:{...}}`; image/image_url
menjadi `bytesBase64Encoded`+`mimeType` (data URL), `gcsUri` (string lain), atau objek Vertex-native
(passthrough); delapan knob OpenAI-ish dipetakan ke nama parameter model
(`sampleCount`/`durationSeconds`/`aspectRatio`/`resolution`/`seed`/`negativePrompt`/`storageUri`/`generateAudio`).
Poll: `POST {base}/v1/{modelPath}:fetchPredictOperation` dengan `{operationName}`. Kodek:
`encodeVertexJobID`/`decodeVertexJobID` (D5). Reader: `name`+`done`+`error` menjadi keadaan kanonik;
asset dibaca dari `videos[]` atau `generateVideoResponse.generatedSamples[]`, URL dari
`gcsUri`/`video.uri`/`uri`, bytes dari `bytesBase64Encoded`/`video.bytesBase64Encoded` → `b64_json`.

**Envelope dan kontrak.** `VideoJob.B64JSON` (`b64_json,omitempty`); kontrak YAML mendapat tiga enum
`auth_type` + empat field SA + `b64_json` (semuanya di skema create, bulk, response, dan `VideoJob`),
`openapi.json` diregenerasi dari YAML, dan prosa CONTRACT MD + SPEC-API §7.5/§7.10 mencatatnya.

**Panel.** `AUTH_TYPES`/`AUTH_TYPE_LABELS` mendapat `service_account` ("Service account");
`endpoint-credential.ts` (hasil pecahan) memuat kosakata kredensial dengan
`REQUIRES_KEY_AUTH_TYPES`/`keyCredentialLabel` ("Service Account JSON")/`takesKeyCredential` yang diperluas;
`routing.ts` memperluas `API_KEY_AUTH_TYPES`; `CreateEndpointForm.svelte` menampilkan input Project ID +
Location hanya ketika auth type-nya SA dan mengirim pair hanya saat itu; `AddProviderKeysDialog.svelte`

- `ProviderKeyFields.svelte` melakukan hal yang sama di jalur dialog, dengan tab batch tidak ditawarkan;
  `provider-keys.ts` mengirim pair lewat `addConnection` dan mengekstrak `readStoredLabels`.

## 5. Gerbang

Diukur pada pohon beku. `app-serv`: sidik jari `506972cbbfd204d0e9d419e358136b56` (`find` atas
`*.go`/`*.yaml`/`*.json`/`*.mjs`, satu md5 dari seluruh md5sum). `app-ui`: sidik jari
`4eac1e21b28d46c80e28fbfbf9b423c6` (`find` atas `src`/`tests`). Tabel routing `scrypts/lib/gate-scope.sh`
menjalankan **kedua** paruh untuk change set ini: `docs/CONTRACT/*`, `internal/handler/openapi.json`,
`app-ui/src/lib/schemas/*`, dan dokumen akar semuanya masuk ember "both".

| Gerbang               | Hasil                                                                                                                                                                                                                       |
| --------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `gate-scope-test.sh`  | PASS, tabel routing utuh                                                                                                                                                                                                    |
| `go-lint.sh`          | PASS: `go vet` dan `go vet -tags=integration,live`, gofmt, staticcheck (dua tag), golangci-lint **0 isu**. Isu pertama jalannya adalah lima temuan pada berkas pass ini (F6), semuanya diperbaiki sebelum jalannya terakhir |
| `go-headers.sh`       | PASS: 1278 berkas Go membawa header lengkap                                                                                                                                                                                 |
| `antislop.sh`         | PASS seluruh pemeriksaan (R-02 menangkap satu em dash yang saya tulis sendiri di PORT 011 §6, diperbaiki)                                                                                                                   |
| `go-test.sh` (race)   | EXIT 0, 19 paket `ok` (`router` 192,84s adalah yang terlama). Suite integrasi ber-tag **dilewati** karena `PANNELAI_TEST_POSTGRES_DSN` tidak diekspor di shell ini: dicatat sebagai skip, bukan pass                        |
| `panel-check.sh`      | PASS seluruhnya: prettier, eslint, svelte-kit sync, svelte-check, vitest (**3029 test di 186 berkas, 1423,40s**), vite build 51,11s                                                                                         |
| `secrets.sh --all`    | PASS: 409 commit dipindai, nol kebocoran di commit dan di berkas yang akan dibawa push                                                                                                                                      |
| `contract-drift.sh`   | PASS: 13 kode error sepakat di kedua sisi                                                                                                                                                                                   |
| `contract-openapi.sh` | PASS: dokumen yang dilayani sama dengan kontrak YAML                                                                                                                                                                        |

Satu kegagalan nyata muncul di jalannya gerbang dan ditutup di pass ini juga, bukan diwariskan:
`go-lint` menggagalkan lima temuan pada berkas yang pass ini tulis (F6). Perbaikannya menyentuh berkas
Go, jadi pohon dibekukan ulang dan **seluruh** gerbang di atas dijalankan lagi di atas sidik jari yang
tercatat di atas, bukan diwarisi dari jalannya yang pertama.

Setelah jalannya yang hijau itu, yang berubah hanya dokumen: tabel §5 ini diisi dari hasilnya, satu
angka baris di §7 dikoreksi, dan baris §1.4 diperjelas. Tidak ada berkas `app-serv/` atau `app-ui/src`
yang berubah, dan kedua sidik jari di atas tetap sama; yang dijalankan ulang setelah penyuntingan itu adalah
`go-test.sh` (EXIT 0), vitest penuh (3029/186), `antislop.sh` (PASS), `go-headers.sh` (PASS 1278), dan
prettier atas dokumen yang disentuh. Angka 3029/1423,40s dan 19 paket berasal dari jalannya ulang itu.

## 6. Residu

1. **Belum dibuktikan live.** Tidak ada satu pun SA JSON nyata yang dipakai di pass ini: bentuk request
   create/poll dipin oleh test terhadap stub dan oleh adapter reference, tetapi panggilan sungguhan ke
   `aiplatform.googleapis.com` butuh project GCP + service account dengan Vertex AI API aktif. Sampai
   itu terjadi, klaim "Veo bekerja" adalah klaim bentuk, bukan klaim jawaban.
2. **Chat Vertex (empat model Gemini) tidak diporting** (D2/§1.2): butuh translator wire `vertex` untuk
   jalur chat, pekerjaan bentuk kedua yang tidak diminta owner.
3. **Tidak ada failover pada create** (mewarisi PORT 011 residu 3): `Perform` satu percobaan, jadi create
   video vertex tidak berputar ke akun lain.
4. **`edits`/`extensions` tetap tanpa route** (mewarisi PORT 011 residu 4).
5. **Panel: click-through live untuk dialog SA belum dilakukan.** Buktinya adalah render test terhadap
   stub yang menjawab persis body yang route baca; pass live butuh gateway yang sudah menjalankan
   migrasi 000016 + SA JSON nyata (butir 1).
6. **Panel belum merender kind `video`** (mewarisi PORT 011 residu 5): halaman media panel belum
   menampilkan model Veo atau job video.
7. **Batas `b64_json` adalah batas body media yang sudah ada:** 32 MiB (`mediaBodyBytes`,
   `dataplane/media.go:30`). Video inline di atas batas itu terpotong di pembacaan dan berakhir sebagai
   jawaban yang tidak terbaca, bukan sebagai envelope besar; reference juga tidak punya cap sendiri.

## 7. Checklist AGENTS.md per-PR

| Butir                     | Keadaan                                                                                                                                                                                                                                                                                                                                                                                                 |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| §1.1 ambang 250 baris     | Berkas baru, diukur `wc -l`: `media_video_vertex.go` 212, `media_video_vertex_id.go` 97, `media_video_vertex_read.go` 116, `media_service_account.go` 50, `media_video_plan.go` 96, `vertex_jwt.go` 145, `vertex_token.go` 195, `upstream_endpoint_service_account.go` 78, `endpoint-credential.ts` 115. Empat berkas yang tumbuh dipecah (F5); tidak ada berkas berubah yang melewati ambang keras 250 |
| §1.2 header per berkas    | Setiap berkas Go baru membawa header lengkap (delapan tag); `go-headers.sh` hijau di pass gerbang                                                                                                                                                                                                                                                                                                       |
| §1.3 bahasa error         | Semua kalimat client-facing berbahasa Inggris ("the vertex video job id is not usable", "sa_project_id and sa_location belong to a service_account endpoint", "the service-account endpoint has no stored credential")                                                                                                                                                                                  |
| §1.4 nol `any`            | `json.RawMessage` hanya di boundary body video (asset sumber yang diteruskan apa adanya); blok parameter adalah struct bertipe `vertexPredictParameters` (F6); tidak ada `interface{}` di luar boundary                                                                                                                                                                                                 |
| §1.5 batas layer          | Minter di `provider` (dial lewat klien egress-guard); penukaran di `service`; bentuk wire di `dataplane`; kontrak di `schema`; tidak ada import lintas-service                                                                                                                                                                                                                                          |
| §1.6 timeout + goroutine  | Exchange token: `context` 15 detik, body 1 MiB; tidak ada goroutine baru di pass ini                                                                                                                                                                                                                                                                                                                    |
| §2.1 test                 | TDD: test ditulis bersama implementasi (domain, service, provider, registry, handler, panel); tidak ada `t.Skip`, tidak ada filter `-run` yang di-commit                                                                                                                                                                                                                                                |
| §2.4 CDD                  | Kontrak YAML berubah lebih dulu (enum + field SA + `b64_json`), `openapi.json` diregenerasi dari YAML, lalu schema Go dan panel mengikutinya                                                                                                                                                                                                                                                            |
| §1.9 sinkronisasi dokumen | SPEC-API §6 (jumlah provider), §7.5 (prosa + baris tabel), §7.10 (paragraf job async + dialect vertex), changelog; CONTRACT MD; `SYSTEM_MAP.md` (jumlah provider + §3.6); `README.md`; `app-serv/README.md`; `app-ui/README.md`; PORT 011 tindak lanjut. Ada perubahan struktur data (migrasi 000016): peta diperbarui, jadi §1.9 terpenuhi bukan N/A                                                   |
