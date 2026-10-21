# 013-PORT-AGNES-AI.md: Provider agnes (Agnes AI), katalog campuran, dan dialek video kelima

Pass `app-serv`. Permintaan owner: **"penambahan provider agnes ai"**, dengan docs
`https://www.agnes-ai.com/en/docs/overview`, lalu **"Untuk BE (app-serv) commit dulu secara local supaya dapat
track histori"**.

|                      |                                                                                                                                                                                                                                                                                                                                                                       |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**           | **CLOSED 2026-10-21** untuk registrasi, katalog, dan dialek video. Live generation belum dijalankan: deployment ini belum punya API key Agnes. Dua commit lokal, `cf2d543` dan `c06caa4`, belum di-push.                                                                                                                                                                |
| **Mechanism**        | DURING (antislop)                                                                                                                                                                                                                                                                                                                                                     |
| **Permintaan owner** | Provider Agnes AI terdaftar dan bisa dipakai, bukan hanya masuk daftar. Fokus pass ini registrasi + jalur chat/image/video; pembuktian live menunggu key.                                                                                                                                                                                                              |
| **Kaitan**           | SPEC-API §7.4 dan §7.10 (diamendemen) + changelog 2026-10-21; `SYSTEM_MAP.md`; AGENTS.md §1.1, §1.2, §1.9, §2.4; PORT 011 (bentuk route video) dan PORT 012 (dialek video keempat)                                                                                                                                                                                      |
| **Tanggal**          | 2026-10-21                                                                                                                                                                                                                                                                                                                                                            |
| **Reference**        | `https://wiki.agnes-ai.com/en/docs/{overview,quickstart,agnes-25-flash,agnes-30-flash,agnes-image-25-flash,agnes-video-25,agnes-video-25-flash,pricing}.md`, diukur 2026-10-21; `GET https://apihub.agnes-ai.com/v1/models` (publik, tanpa auth). Reference 9Router `39e36d3d` tidak punya entri `agnes`.                                                              |

## 1. Ringkasan: yang terukur sebelum touch, dan yang berubah

| Yang diukur               | Sebelum pass                                                                 | Sesudah pass                                                                 |
| ------------------------- | ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| Provider di registry      | 36                                                                           | 37 (`agnes`, priority 340)                                                    |
| Katalog Agnes             | Tidak ada entri                                                              | 2 chat, 3 image, 1 video (persis `GET /v1/models`)                            |
| Dialek video              | 4: xAI default, openrouter, tokenrouter, vertex                              | 5: ditambah `agnes` (create `/v1/videos`, poll `/agnesapi`)                   |
| Blok media image          | Tidak ada                                                                    | `/v1/images/generations` dengan `apikey`/`bearer` eksplisit                   |
| Pasangan pricing          | 505                                                                          | 511 (enam pasangan Agnes, semuanya unpriced)                                  |
| Tes registry/service      | 36 provider, 4 dialek                                                        | 37 provider, 5 dialek, tes dialek Agnes                                       |

Berkas: `tools/registry-gen.mjs`; `internal/registry/{registry.yaml,embedded_test.go,provider_agnes_test.go,testdata/pricing_corpus.json}`;
`internal/service/{media_video_agnes.go,media_video_agnes_test.go,media_video_plan.go,media_video_test.go,media_call_entries_test.go}`;
`internal/service/{media_video_read.go,media_video_progress.go}` (perbaikan progress TokenRouter, commit `cf2d543`);
`docs/SPEC-API/001-SPEC-API.md`; `SYSTEM_MAP.md`; `README.md`; `app-serv/README.md`.

## 2. Keputusan pass ini

**D1. Katalog mengikuti endpoint, bukan halaman docs.** `GET /v1/models` menyajikan enam id:
`agnes-2.5-flash`, `agnes-3.0-flash`, `agnes-image-2.0-flash`, `agnes-image-2.1-flash`,
`agnes-image-2.5-flash`, `agnes-video-2.5-flash`. Docs juga menyebut `agnes-2.5-pro`, `agnes-2.5-pro-beta`,
`agnes-3.0-pro`, dan `agnes-video-2.5`, tetapi endpoint publik tidak menyajikannya. Model yang tidak bisa
dilayani tidak didaftarkan; menambahkannya berarti menawarkan request yang selalu ditolak.

**D2. Entri provider adalah deklarasi app-serv.** Reference tidak punya `agnes`, jadi seluruh entri hidup di
`APP_SERV_PROVIDER_ENTRIES` pada generator. Dokumen yang di-commit sudah menyimpang dari hasil generasi penuh
di provider lain (base URL opencode, retry qoder, beberapa model, komentar tangan), sehingga regenerasi utuh
akan mengembalikan perubahan itu. Blok Agnes disisipkan langsung, lalu **dibuktikan identik** dengan blok yang
dihasilkan generator; regenerasi berikutnya mereproduksi entri yang sama. Rekonsiliasi drift itu pass terpisah
(R2).

**D3. Satu task id membawa video id dan model.** Poll Agnes adalah
`GET /agnesapi?video_id=…&model_name=…` di host yang sama, dan task `keyframe`/`reference` ditolak tanpa
`model_name`, sedangkan route poll gateway tidak membawa model. Create karena itu menjawab
`base64url(video_id + "|" + model)` sebagai `task_id`; poll men-decode keduanya. Video id mentah tetap
diterima dan poll menghilangkan `model_name`, bentuk yang vendor baca untuk mode `text`. Model diambil dari
field `model` pada jawaban create (R1).

**D4. Blok image menamai kredensialnya sendiri.** Reference tidak menulis `auth_type`/`auth_header` pada blok
image mana pun, dan media plane app-serv tidak mengirim kredensial saat blok tidak menamainya. Blok Agnes
karena itu menulis `apikey`/`bearer` eksplisit. Provider image lain belum diperbaiki di pass ini (R3).

**D5. Pricing tidak ditebak.** Tabel pricing reference tidak mengenal Agnes, jadi keenam pasangan masuk corpus
sebagai unpriced dan usage mencatat nol. Halaman pricing vendor memang mencantumkan keenam model di `$0` selama
promosi, sehingga nol adalah angka yang benar hari ini, bukan angka yang hilang.

## 3. Verifikasi

| Gerbang               | Hasil                                                                                                  |
| --------------------- | ------------------------------------------------------------------------------------------------------ |
| `go-lint.sh`          | PASS: staticcheck (dua tag) dan golangci-lint **0 isu**                                                |
| `go-headers.sh`       | PASS: 1456 berkas Go membawa header lengkap                                                            |
| `antislop.sh`         | PASS seluruh pemeriksaan, termasuk R-02 dan panjang blok doc                                            |
| `secrets.sh`          | PASS: tidak ada kebocoran di perubahan yang di-commit                                                  |
| `contract-drift.sh`   | PASS: 14 kode error sepakat di SPEC dan panel                                                          |
| `go-test.sh`          | PASS: `go test -race -count=1 ./...` seluruh `app-serv`, 20 paket `ok`; suite integrasi di-skip karena `PANNELAI_TEST_POSTGRES_DSN` tidak diekspor |
| `go test -race`       | PASS terarah untuk `./internal/registry` dan `./internal/service` (23,4s dan 82,2s)                    |
| HEAD terisolasi       | `git worktree` di `c06caa4`: `go build ./...` sukses, registry dan service hijau                       |
| Live tanpa key        | `POST /v1/videos`, `GET /agnesapi`, `POST /v1/images/generations` menjawab **401 "Token not provided"** |

Probe 401 itu bukan bukti generasi bekerja, hanya bukti URL dan penempatan bearer yang ditulis blok registry
memang yang dibaca vendor. Yang belum diukur adalah satu putaran create + poll dengan key asli.

## 4. Residu

- **R1 (blocked, butuh key).** Live generation belum dijalankan. Yang harus dikonfirmasi: jawaban create
  menggemakan `model`, karena task id keyframe/reference bergantung padanya; tanpa echo itu poll jatuh ke mode
  `text`.
- **R2 (MEDIUM, pass terpisah).** Dokumen registry yang di-commit menyimpang dari generasi penuh di provider
  lain. Selama drift itu ada, regenerasi utuh bukan operasi yang aman.
- **R3 (MEDIUM, pass terpisah).** Blok image provider lain (`openai`, `xai`, `openrouter`, dan seterusnya)
  masih tidak menamai kredensial, sehingga media plane mengirim request tanpa bearer. Agnes adalah satu-satunya
  blok image yang sudah benar.
- **R4 (LOW).** `agnes-video-2.5` (non-flash, berbayar) dan keluarga pro belum didaftarkan karena tidak ada di
  `/v1/models`; `agnes-3.0-pro` masih "coming soon" di docs.
- **R5 (LOW).** Thinking vendor ditulis sebagai `chat_template_kwargs.enable_thinking`, bukan dialek reasoning
  yang diporting. Model tidak ditandai reasoning, jadi gateway tidak menyuntik field itu sendiri; klien masih
  bisa mengirimnya lewat body.
