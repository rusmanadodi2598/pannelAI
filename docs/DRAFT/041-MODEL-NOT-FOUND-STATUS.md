# 041-MODEL-NOT-FOUND-STATUS.md: §7.15 bilang 404, kode menjawab 400, dan tidak ada satu pun test yang notices

Register penutupan drift yang `docs/DRAFT/009-PLAYGROUND-CHAT-ENDPOINT-READINESS.md` §10.10 dan
`docs/DRAFT/040-PLAYGROUND-MODEL-LIST.md` §8 catat terbuka. Dokumen ini bukan kontrak; kontrak
tetap `docs/SPEC-API/001-SPEC-API.md` §7.15/§8 dan `docs/CONTRACT/001-CONTRACT-API-V1.yaml`.

|                      |                                                                                                                                                     |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**           | **CLOSED 2026-10-03.** Owner memutuskan arah: sinkron ke **404**, mengikuti §7.15 dan wire OpenAI                                                     |
| **Mechanism**        | DURING (test lebih dulu, lalu implementasi), AFTER sebagai register ini                                                                             |
| **Scope**            | `app-serv/internal/dataplane/errors.go`, kontrak YAML + artifact generated, `docs/SPEC-API` §8, `app-ui/src/lib/schemas/error.ts`                    |
| **Permintaan owner** | "Sinkron aja sesuai 404 MODEL_NOT_FOUND" (2026-10-03)                                                                                                 |
| **Kaitan**           | §7.15 resolusi model; §8 error envelope; `scrypts/gates/contract-drift.sh`; `scrypts/gates/contract-openapi.sh`; `internal/handler/datplane_errors.go` |
| **Tanggal**          | 2026-10-03                                                                                                                                            |

## 1. Tiga daftar, tiga jawaban

| Sumber | Menyatakan |
| --- | --- |
| SPEC-API §7.15:878 | `combo → alias → provider/model → 404 MODEL_NOT_FOUND` |
| SPEC-API §8 tabel | **tidak mencantumkan `MODEL_NOT_FOUND` sama sekali** |
| CONTRACT YAML `data_plane.codes` | `MODEL_NOT_FOUND: 400` |
| `errors.go` `statusFor` | 400 |
| Gateway hidup, diukur 2026-10-03 | `http=400 code=MODEL_NOT_FOUND` |
| OpenAI wire asli | 404 `invalid_request_error`, "The model `x` does not exist or you do not have access to it" |

Komentar `statusFor` bahkan menulis "§8's table is where both are published" untuk kode yang
tabel itu tidak pernah daftar. itulah sebabnya drift ini bertahan dua belas hari: gate
`contract-drift.sh` membandingkan **tabel §8** dengan enum tertutup panel, dan kode yang tidak
ada di kedua daftar tidak dibandingkan oleh apa pun. Tidak ada compile error, tidak ada test
gagal, di kedua sisi.

Yang membuat ini bukan sekadar angka: `statusFor` tidak punya unit test sebelum pass ini.

## 2. Keputusan, dan mengapa 404

Permukaan ini mengaku kompatibel OpenAI. Client membedakan **type** untuk klasifikasi dan
**status** untuk kebijakan retry. 400 berarti "perbaiki body, kirim lagi"; mengirim ulang nama
model yang sama adalah permintaan yang tidak akan pernah berhasil, dan retry policy yang benar
membaca itu sebagai izin mencoba. OpenAI menjawab 404 untuk model yang tidak dikenali, dan
§7.15 sudah menulis 404. Jadi sisi yang menyimpang adalah kode dan kontrak, bukan spesifikasinya.

`type` tetap `invalid_request_error`, mengikuti klasifikasi reference, sehingga client yang
cabang pada type tidak berubah. `PROVIDER_NOT_ROUTABLE` dan `UPSTREAM_REJECTED` tetap 400 untuk
alasan yang sudah tercatat di sebelahnya: keduanya cacat permintaan yang bisa diperbaiki client
dengan mengirim permintaan berbeda, bukan nama yang tidak ada.

## 3. Yang berubah

- `errors.go`: `CodeModelNotFound` pindah dari kasus 400 ke kasusnya sendiri, 404. Tidak ada
  call site yang memilih status sendiri; `dataPlaneError` yang menghitungnya.
- `dataplane_error_test.go`: satu baris tabel bergerak 400 → 404. Ini regresi yang memang
  dituntut keputusan owner, dan pernyataan itu adalah ejaan status, bukan perilaku.
- **Test baru** `errors_status_test.go`: table-driven untuk `statusFor` (10 kasus), `typeFor`
  (8 kasus), dan konstruktor `dataPlaneError` yang membawa status hasil pemetaan, supaya status
  yang dihitung lalu dibuang di jalan keluar tidak bisa lolos.
- **Test HTTP baru** di `chat_http_test.go`, dua kasus pada dua tabel yang sudah ada: satu di
  tabel request non-stream (`unknown model` → 404 `MODEL_NOT_FOUND`), satu di tabel
  `TestChatCompletionsHTTP_StreamLifecycle` (`unknown model before the first frame` → 404,
  `application/json`), karena 009 F3 menjanjikan error HTTP normal di titik itu dan sekarang
  harus 404, bukan 400 dan bukan 200 ber-SSE.
- **Kontrak YAML**: komponen baru `DataPlaneNotFoundError`; `MODEL_NOT_FOUND` keluar dari
  `DataPlaneBadRequestError` (bersama deskripsinya, yang tidak lagi mengklaim "named an unknown
  model"); `codes.data_plane` jadi 404. `openapi.json` diregenerasi lewat `tools/openapi-gen`,
  tidak disunting tangan.
- **§8** mendapat baris `MODEL_NOT_FOUND | 404`.
- **`app-ui/src/lib/schemas/error.ts`** mendapat kodenya plus satu kalimat fallback. Ini bukan
  hiasan: gate memaksa kedua daftar sama, dan enum panel sudah memuat kode data plane lain
  (`NO_PROVIDER_AVAILABLE`, `UPSTREAM_*`), jadi menambah satu kode mengikuti pola yang ada.
  Ditulis apa adanya soal pencapaiannya: `fallbackMessage` hanya dipanggil lewat
  `schemaApiErrorEnvelope` (`src/lib/api/errors.ts:60`), dan rute management tidak pernah
  membalut `MODEL_NOT_FOUND` di envelope itu, jadi entri ini hari ini defensif, belum tereksekusi.
  Kodenya memang sudah sampai ke panel sebagai **field per baris** pada hasil probe model
  (`internal/handler/provider_model_probe_test.go:52-53` menagih baris `200` yang membawa
  `CodeModelNotFound`), tapi jalur itu tidak melewati enum ini.

## 4. Sebelas rute, bukan dua belas

`GET /api/v1/models` selama ini ikut menunjuk komponen 400, tapi rute itu tidak resolve apa pun
(handler-nya hanya auth lalu membaca daftar). Memberinya 404 akan membuat kontrak mengklaim
respons yang tidak pernah bisa diproduksi. Jadi `DataPlaneNotFoundError` dipasang pada sebelas
rute yang model string-nya benar-benar masuk resolver; pengecualian `/api/v1/models` tidak
ditulis sebagai komentar YAML (kontrak ini tidak membawa komentar di blok path), melinkan
dibuktikan oleh daftar di bawah dan oleh hasil verifikasi artefak.

Verifikasi artefak generated: 11 rute menunjuk `DataPlaneNotFoundError`
(`chat/completions`, `messages`, `responses`, `messages/count_tokens`, `embeddings`,
`audio/speech`, `audio/transcriptions`, `audio/voices`, `images/generations`,
`videos/generations`, `search`), dan `/api/v1/models` tidak.

## 5. Bukti

Merah lebih dulu, karena perilaku: `statusFor("MODEL_NOT_FOUND") = 400, want 404` dan
`Status = 400, want 404`, sebelum implementasi.

| Gate | Hasil |
| --- | --- |
| `go build ./...`, `go vet ./...` | PASS |
| `go test -count=1 ./...` | PASS (17 paket) |
| `go test -race -count=1 ./...` | lihat §6 |
| `scrypts/gates/go-lint.sh` | PASS |
| `scrypts/gates/contract-drift.sh` | PASS, 13 kode sama di kedua sisi |
| `scrypts/gates/contract-openapi.sh` | PASS, dokumen served sama dengan YAML |
| `scrypts/gates/go-headers.sh` | PASS |
| `app-ui` `vitest run` (schema + error) | PASS, 1777 test / 67 berkas |

**Live, terhadap biner tambal di `:9091`** (gateway owner `:9090` tidak disentuh, tidak
di-restart):

| Permintaan | Status | Body |
| --- | --- | --- |
| `POST /chat/completions`, non-stream, `no-such-provider/no-such-model` | **404** | `{"message":"provider no-such-provider is not in the registry","type":"invalid_request_error","code":"MODEL_NOT_FOUND"}` |
| Permintaan yang sama dengan `"stream":true` | **404**, `content-type: application/json` | body yang sama, tanpa header SSE yang ikut tertulis |

Kasus stream adalah bukti ganda: ia mengukuhkan 009 F3 (kegagalan sebelum frame pertama adalah
error HTTP biasa) sekaligus menunjukkan statusnya kini benar di kedua bentuk wire.

## 6. Yang tidak ditutup di sini

- **Sisa `-race` yang flaky dan sudah ada sebelumnya**: `provider_model_probe_budget_test.go:33`
  dengan margin stub 20 ms di atas budget 20 s, terbukti gagal di HEAD bersih di bawah beban CPU
  (lihat 040 §8). Bukan perubahan ini.
- **`PROVIDER_NOT_ROUTABLE` juga tidak ada di tabel §8.** Kode, YAML, dan komentar `statusFor`
  sama-sama menyebutnya, gate tidak melihatnya, enum panel tidak memilikinya. Kelas cacat yang
  persis sama dengan yang baru ditutup di sini, tinggal satu baris, dan menunggu keputusan owner
  yang sama.
- **Body upstream yang bocor ke `message` client.** Diukur dua kali: tokenharbor menjawab
  `Model 'definitely-not-a-model' is not available. Browse models at https://tokenharbor.ai/...`
  dan tembus sebagai `502 UPSTREAM_ERROR` dengan kalimat vendor apa adanya; Qoder menjawab
  `{"pricingUrl":"https://qoder.com/pricing?client=qoder"}` dan message client-nya persis JSON itu.
  §6.7 draft 009 mengklaim raw upstream body tidak mencapai client; klaim itu tidak berlaku pada
  kedua kasus. Ada juga penyebab strukturalnya: `resolve.go:241` membiarkan node `Custom`
  menerima id model apa pun, jadi nama yang salah tidak pernah jadi `MODEL_NOT_FOUND` melainkan
  dial ke vendor, dan §8 memetakan vendor 404 ke `UPSTREAM_ERROR` 502 secara eksplisit
  (kecuali 401/402/403/404/429 baru jadi `UPSTREAM_REJECTED` 400). 502 untuk kondisi permanen
  adalah bahan bakar retry selamanya. Butuh keputusan owner terpisah, dan tidak dikerjakan diam-diam
  di batch ini.
- **Riwayat status code.** Perubahan ini mengubah respons yang terlihat client. Owner menerimanya
  sebagai sinkronisasi ke spesifikasi, bukan sebagai versi baru; kalau ada CLI yang hari ini
  bercabang pada `400 MODEL_NOT_FOUND`, itulah yang perlu dicari sebelum push berikutnya.
