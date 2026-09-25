# 031-OPENCODE-FREE-LANE-UNREACHABLE.md: kenapa `app-serv` menjawab 503/502 untuk tiga model free tier owner

Permintaan owner: "Provider: OpenCode | Pada free models: `oczen/space-bunny-free`,
`oczen/muse-spark-1.3-contributor-free`, `oczen/mimo-v2.6-flash-free`. Sebelumnya saya sudah apply patch
terbaru untuk 1:1 dengan reference 9router dalam penanganan model free tier tersebut, namun `app-serv`
masih 503 untuk upstream OpenCode tersebut. Cek dan analisis, lalu fix."

Dua cacat independen ditemukan, keduanya diukur live, dan keduanya diperbaiki. Bukan kontrak; kontrak
tetap `docs/SPEC-API/001-SPEC-API.md` (wire) dan `docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel).

| | |
|---|---|
| **Status** | **CLOSED 2026-09-24.** Dua perbaikan source + test regresi mendarat; belum di-push |
| **Mechanism** | DURING & AFTER (antislop) |
| **Scope** | `app-serv/internal/dataplane`, `app-serv/internal/service`, `app-serv/internal/registry`, `app-ui/src/lib/schemas/model-picker.ts`, amandemen SPEC-API §7.6 + kontrak OpenAPI |
| **Permintaan owner** | analisis + fix 503 pada keluarga OpenCode free tier |
| **Reference** | `decolua/9router`, checkout `/home/rusmanadodi/apps/9router`, `39e36d3d` = `v0.5.86` |
| **Kaitan** | draft 029 §4.8 (F8, endpoint virtual); draft 025 (filter `active`); draft 021 §15-§21 (riwayat pengukuran `oczen`); SPEC-API §7.6 |
| **Tanggal** | 2026-09-24 |

---

## 1. Ringkasan

Pesan 503 yang owner lihat menyebut provider **`openai-compatible-03863XJ2YM2KHF847XJP5YBSH8`**, yaitu
**custom node** `oczen` ("OpenCode Zen Free", `base_url https://opencode.ai/zen/v1`,
`api_type responses`), **bukan** entri registry `opencode` (Free, `no_auth`) yang justru 1:1 dengan
reference dan sudah bisa menjawab. Jadi 503 bukan karena patch free tier gagal: patch itu bekerja, tetapi
panel tidak pernah menawarkan lane yang bekerja, sehingga operator membuat node sendiri dan node itu
menabrak dua cacat:

| # | Tingkat | Isi |
|---|---|---|
| **F1** | HIGH | Custom node tidak bisa credential-free: `registry.Synthesize` menulis `AuthType: AuthAPIKey` secara hardcode (`custom_node.go:157`), sehingga aturan endpoint virtual F8 (`selection_virtual.go`) tidak pernah menyala untuk node mana pun. `oczen/*` selalu 503 `NO_PROVIDER_AVAILABLE` |
| **F2** | HIGH | `foldResponsesEvents` tidak mengenal `response.incomplete`, padahal upstream OpenCode mengirim event itu **setiap kali** jawaban berhenti di `max_output_tokens`. Klien non-streaming menerima 502 `UPSTREAM_ERROR` untuk stream yang lengkap dan benar |
| **F3** | HIGH (akar F1) | Premis filter `?active=true` (draft 025) sudah usang oleh F8: filter tidak pernah menambahkan provider `no_auth` tanpa baris endpoint, padahal router melayaninya lewat endpoint virtual. Terukur: `opencode/space-bunny-free` 200 di data plane, tetapi `GET /models/catalog?active=true` mengembalikan 0 baris `opencode`. Picker panel (`endpoint_count > 0`, tanpa klausa `no_auth`) melakukan hal yang sama |

## 2. Bukti terukur (sebelum perbaikan)

Gateway live, kunci `sandbox`, model persis seperti yang owner sebut:

| Ref | Sebelum | Sesudah |
| --- | --- | --- |
| `oczen/space-bunny-free` | 503 `NO_PROVIDER_AVAILABLE` (0,01 s) | (node dihapus; lane benar dipakai) |
| `oczen/mimo-v2.6-flash-free` | 503 `NO_PROVIDER_AVAILABLE` | idem |
| `oczen/muse-spark-1.3-contributor-free` | 503 `NO_PROVIDER_AVAILABLE` | idem |
| `opencode/muse-spark-1.3-contributor-free`, `max_tokens` 32 | **502** `UPSTREAM_ERROR` "the upstream stream ended without a final answer" | **200**, `finish_reason` `length`, usage 566/32 |
| `oc/muse-spark-1.3-contributor-free`, `max_tokens` 4000 | 200 (upstream sempat mencapai `response.completed`) | 200 |
| `oc/space-bunny-free` | 200 | 200 |
| `oc/mimo-v2.6-flash-free` | 200 | 200 |

Sesudah perbaikan, ketiga model owner menjawab 200 lewat lane bawaan (`oc/…`), non-streaming, dengan
`finish_reason` `stop`:

| Ref | Hasil |
| --- | --- |
| `oc/space-bunny-free` | 200, `'Hi there, friend!'` |
| `oc/mimo-v2.6-flash-free` | 200, `'Hello there friend!'` |
| `oc/muse-spark-1.3-contributor-free` | 200, `'Hi there, friend!'` |

Katalog: `GET /models/catalog?active=true&provider_id=opencode` menjawab **0 baris** sebelum, **4 baris**
sesudah (`opencode/jev-1.13-free`, `opencode/muse-spark-1.2-contributor-free`,
`opencode/muse-spark-1.3-contributor-free`, `opencode/union-alpha`).

### 2.1 Pengukuran upstream langsung (`opencode.ai/zen/v1`, tanpa kredensial)

| Model | `/chat/completions` | `/responses` | `/messages` |
| --- | --- | --- | --- |
| `space-bunny-free` | 200 | 401 `ModelError` | 200 |
| `mimo-v2.6-flash-free` | 200 | 500 | 500 |
| `muse-spark-1.3-contributor-free` | 500 | 200 | 500 |

Jadi setiap model punya satu wire yang benar, dan registry sudah mendeklarasikannya
(`target_format: openai-responses` untuk kedua muse-spark). Yang salah bukan wire, melainkan
penanganan event terminalnya.

`response.incomplete` terukur pada **setiap** panggilan `muse-spark-1.3-contributor-free` yang berhenti di
cekungan output (`max_output_tokens` 32/64/200/512/2000); `response.completed` muncul begitu budget cukup
besar untuk menyelesaikan jawaban (diukur pada `max_tokens` 4000). Jadi cacat F2 bukan kasus tepi: ia
adalah jalur normal untuk panggilan pendek.

## 3. Akar masalah

### 3.1 F1 — custom node tidak bisa credential-free

```go
// app-serv/internal/registry/custom_node.go:157 (sebelum)
provider := Provider{
    ...
    AuthType:  AuthAPIKey,   // hardcode
    ...
}
```

Aturan endpoint virtual F8 (`dataplane/selection_virtual.go`) hanya menyala bila
`entry.NoAuth || entry.Transport.NoAuth || entry.AuthType == registry.AuthNone`. Node hasil sintesis
selalu `AuthAPIKey`, jadi aturan itu menolak, `SelectNext` menjawab `NO_PROVIDER_AVAILABLE`, dan 503 itulah
yang owner lihat. Reference tidak punya masalah ini karena koneksi virtual-nya disuntik berdasarkan
`FREE_PROVIDERS[providerId]?.noAuth` (`src/sse/services/auth.js:46`), dan provider `noAuth` bawaan
(`opencode`) tidak pernah butuh baris endpoint sama sekali — tidak seperti node.

**Kesimpulan yang penting untuk owner:** node `oczen` tidak diperlukan. Provider `opencode` (alias `oc`)
sudah 1:1 dengan reference, sudah `no_auth`, dan sejak F8 menjawab tanpa satu baris `upstream_endpoints`
pun. Node `oczen` justru menabrak F1 dan (karena `api_type responses`) memaksa semua model lewat
`/responses`, padahal `space-bunny-free` hanya dilayani di `/chat/completions` dan `/messages` — inilah
sebab 502 "Model space-bunny-free is not supported for format openai" saat endpoint no_auth sementara
ditambahkan untuk node itu.

### 3.2 F2 — fold buta terhadap `response.incomplete`

`foldResponsesEvents` membaca event terminal dari belakang:

```go
case EventResponseCompleted, EventResponseDone, EventResponseFailed:   // sebelum
```

`EventResponseIncomplete` tidak ada di daftar **dan tidak ada** di blok konstanta. Komentar fungsi itu
sendiri sudah mengklaim ia ditangani:

```
// The Responses API states the whole answer in `response.completed` (and in
// `response.incomplete` when the model hit its ceiling), ...
```

Docstring dan kode berbeda — proxy fan-out untuk panggilan pendek sudah tersedia di
`translate_responses_answer.go:98` (`responsesFinishReason` memetakan `incomplete` +
`max_output_tokens` ke `length`), tetapi jalur forced-stream tidak pernah sampai ke sana karena fold
menolak lebih dulu. Test penerimaan draft 029 tidak menangkapnya karena stub upstream-nya hanya menulis
`response.completed` (`engine_opencode_free_streams_test.go:36`).

### 3.3 F3 — dua jawaban untuk satu pertanyaan ("provider ini bisa dilayani?")

F8 mendarat 2026-09-24 17:54. Filter `?active=true` dan picker panel mendarat lebih dulu (draft 025,
07:04) dengan premis yang ditulis eksplisit di komentarnya: "provider `no_auth` tanpa baris menjawab
`NO_PROVIDER_AVAILABLE` seperti provider lain". F8 membuat premis itu **salah**, tetapi tidak ada yang
memperbarui kedua tempat itu. Akibatnya panel menyembunyikan satu-satunya lane free tier yang hidup, dan
operator menyimpulkan ia harus mengonfigurasi provider OpenCode sendiri — dan membuat node `oczen`.

## 4. Perbaikan

Satu pertanyaan, satu jawaban; satu cacat, satu tempat.

| Berkas | Perubahan |
| --- | --- |
| `internal/registry/credential_free.go` (baru) | `Provider.NeedsNoCredential()` — satu-satunya jawaban "apakah provider ini menjawab tanpa kredensial", membaca ketiga ejaan dokumen (`no_auth` provider, `no_auth` transport, `auth_type: no_auth`) |
| `internal/dataplane/selection_virtual.go` | Aturan endpoint virtual memakai `entry.NeedsNoCredential()`; salinan predikat lokal dihapus |
| `internal/dataplane/engine_forced_stream.go` | `EventResponseIncomplete` masuk daftar event terminal |
| `internal/dataplane/translate_stream_responses.go` | Konstanta `EventResponseIncomplete = "response.incomplete"` ditambahkan (klaim docstring kini benar) |
| `internal/service/model_catalog_active.go` | `withCredentialFreeProviders` menambahkan provider `no_auth` tanpa baris ke himpunan aktif, setelah satu pembacaan tabel; provider ber-kunci tidak pernah ditemukan |
| `app-ui/src/lib/schemas/model-picker.ts` | `activeProviderIds` kembali ke aturan reference: `endpoint_count > 0 \|\| no_auth` |
| SPEC-API §7.6, kontrak OpenAPI, `handler/openapi.json` | Semantik `active=true` diperbarui + artifact di-regenerate |

**Batas yang sengaja tidak diperluas.** F1 (custom node tidak bisa `no_auth`) **tidak** ditambal dengan
menambah field ke `provider_nodes`, karena itu menambah permukaan kontrak untuk kasus yang sudah punya
jalan yang benar: provider `opencode` bawaan. Node tetap provider ber-kunci, persis seperti di reference
(`open-sse/executors/default.js` selalu memasang kredensial untuk node). Bila owner tetap ingin node
credential-free, itu keputusan schema tersendiri (`no_auth` pada `provider_nodes` + validasi + panel
form), bukan tambalan.

**Perbaikan yang ditolak.** Memetakan `response.incomplete` ke `length` di jalur **streaming**
(`responsesStreamState.completed`) sempat dicoba lalu dibatalkan: test `TestResponsesStreamState_Completed`
mengunci perilaku reference (`computeFinishReason`, `openai-responses.js:452`), yang selalu menjawab
`stop`/`tool_calls` pada jalur streaming. Klien non-streaming sudah mendapat `length` yang benar dari
`responsesFinishReason` setelah fold, jadi mengubah jalur streaming hanya akan menyimpang dari reference.

## 5. Test regresi

| Berkas | Mengunci apa |
| --- | --- |
| `internal/dataplane/engine_forced_stream_incomplete_test.go` (baru) | Fold menerima keempat event terminal dan menolak stream tanpa event terminal; `response.incomplete` + `max_output_tokens` menghasilkan `finish_reason` `length`; kontrol 502 |
| `internal/service/model_catalog_credential_free_test.go` (baru) | `active=true` menawarkan provider `no_auth` tanpa baris; tidak pernah menawarkan provider ber-kunci tanpa baris; membaca ejaan `no_auth` di transport |
| `internal/registry/credential_free_test.go` (baru) | Ketiga ejaan credential-free, dua kasus ber-kunci, dan predikat setelah `BuildAuthType` |
| `app-ui/tests/schemas/model-picker.test.ts` | Picker menawarkan provider `no_auth` tanpa endpoint (regresi premis draft 025) |

Setiap test dibuktikan menangkap cacatnya: dengan perbaikan dibalik, test fold gagal dengan
`UPSTREAM_ERROR: the upstream stream ended without a final answer`, dan kedua test katalog gagal dengan
`Catalog(active=true) = []`.

`go test -race ./...` bersih (15 paket + `cmd`), `bun run test` 2551 test hijau, dan ketujuh gate
`scrypts/gates/all.sh` PASS (go lint, go headers, go test, panel checks, secrets, contract drift,
contract artifact).

## 6. Yang harus dilakukan owner

1. Pakai lane bawaan, bukan node: **`oc/<model>`** atau **`opencode/<model>`** (alias `oc`).
2. Hapus node `oczen` bila tidak dipakai lagi (`DELETE /provider-nodes/{id}`), karena ia ber-kunci dan
   `api_type responses` tidak bisa melayani `space-bunny-free`.
3. Combo `pi-agent` boleh diperbarui: member `oczen/space-bunny-free` sebaiknya jadi
   `oc/space-bunny-free`. Sebelum perbaikan ini member itu selalu 503 dan combo diam-diam jatuh ke member
   `th-1` (terukur: `pi-agent` menjawab 200 dengan `model` `deepseek-v4.1-flash`).

## 7. Catatan lingkungan

Proses `app-serv` di `:9090` mati beberapa kali selama pengukuran karena ia adalah proses foreground milik
owner di terminal lain (log menutup rapi: `shutdown signal received` / `app-serv stopped cleanly`, bukan
panic). Pengukuran akhir dijalankan pada instance terisolasi di `:9091` dengan biner yang sama; kunci probe
sementara dan endpoint probe sudah dihapus setelah selesai. Tidak ada state owner yang tersisa berubah.