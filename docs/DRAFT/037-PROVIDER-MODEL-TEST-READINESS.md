# 037 — Provider model test (test by model id)

| | |
|---|---|
| **Status** | CLOSED untuk scope pass ini (2026-09-27). |
| **Owner ask** | "Pada REFERENCE sudah support (Test) by models. Integrasikan supaya setiap provider bisa di-testing by model id, 1:1." (2026-09-27) |
| **Answers** | draft `017` §4.10 (F10), keputusan §7.2 nomor 2, dua route pertama |
| **Surfaces** | `app-serv` §7.4 management; `app-ui` provider detail → Model catalog (§6.3 SPEC-UI) |
| **Reference** | `decolua/9router` @ `39e36d3d` (v0.5.86): `src/app/api/models/test`, `src/app/api/providers/[id]/test-models`, `providers/[id]/ModelRow.js` |
| **Mode antislop** | DURING & AFTER (owner directive pada request yang sama) |

---

## 1. Yang ditanyakan operator, dan kenapa route yang ada tidak menjawabnya

`POST /endpoints/{id}/test` menjawab "kredensial ini bisa menghubungi provider".
`POST /provider-nodes/{id}/test` menjawab hal yang sama untuk node. Keduanya **GET ke daftar
model** (`provider_probe_call.go`), jadi keduanya buta terhadap pertanyaan berikutnya:
"gpt-4o jawab tidak?" — pertanyaan yang justru muncul setelah klien gagal satu panggilan.

Reference punya jalur itu: `POST /api/models/test` menendang satu model, dan UI-nya
(`providers/[id]/ModelRow.js:34-49`) menaruh tombol uji **per baris model**. Route
`/providers/[id]/test-models` ada di reference tetapi **tidak punya pemanggil UI** (dibuktikan
dengan grep repo: hanya satu berkas test yang menyentuhnya). Karena itu pass ini memperlakukan
sapuan sebagai tambahan yang berdiri sendiri, bukan sebagai permukaan utama; tombol per baris
yang menjadi permukaan 1:1.

## 2. Yang mendarat

### 2.1 app-serv

| Route | Bentuk |
|---|---|
| `POST /api/v1/providers/{provider_id}/models/test` | `{model_id}` → satu baris `{model_id, name, ok, latency_ms, endpoint_id?, status?, error_code?, error?}` |
| `POST /api/v1/providers/{provider_id}/test-models` | `{limit?}` → `{provider_id, source, warning?, tested, total, stopped?, results[]}` |

Satu port, satu prober, nol mekanisme kedua:

- `service.ModelProber` adalah `dataplane.Engine.Ping` yang sudah dipakai combo test (`§7.7`).
  Interface `ComboProber` dinamai ulang menjadi `ModelProber` karena kini dipakai dua layanan;
  tidak ada interface ketiga dan tidak ada jalur HTTP kedua.
- Probe **melalui pipeline nyata** — resolve → select key → translate → dial. Konsekuensinya
  adalah yang membuat jawaban bisa dipercaya: model yang tidak bisa di-route gateway tidak akan
  pernah melaporkan sehat dari route yang tidak akan gateway ambil.
- Daftar model dari `ProviderService.Models` (`§7.4`), yaitu registry untuk provider bawaan dan
  upstream-then-registry untuk node. `source` dan `warning` ikut dijawab, supaya operator tahu
  bahwa sapuan atas daftar fallback membuktikan lebih sedikit.
- Non-chat (embedding/image/stt) **ditolak dengan `VALIDATION_ERROR`** di satu model, dan
  **dilewati** di sapuan: probe ini panggilan chat, jadi kegagalan model embedding
  menggambarkan probe, bukan modelnya.
- Model yang tidak ada di katalog **tetap diuji**. Node passthrough menerima string yang registry
  belum kenal; menolak semuanya membuat node semacam itu tidak bisa diuji. Jawabannya
  `MODEL_NOT_FOUND` dari pipeline, bukan penolakan panel.

### 2.2 Budget, dan kenapa tidak sama dengan reference

Reference menembakkan model pertama sekuensial lalu sisanya `Promise.all` tanpa batas
(`test-models/route.js:51-57`), masing-masing `max_tokens: 1024`. Satu klik atas provider dengan
40 model = 40 panggilan inferensi nyata, dan timeout di salah satunya **menjatuhkan seluruh
route** (tidak ada `try/catch` per model) menjadi `500 Test failed`. Itu amplifikator kuota dan
bukan kegagalan yang bisa dibaca operator, jadi pass ini membawa bentuknya, bukan angkanya:

| Aturan | Nilai | Alasan terukur |
|---|---|---|
| Per probe | 20 s (`ProviderModelProbeTimeout`) | ceiling reference 15 s (`ping.js`); reasoning model dengan `max_tokens=1024` bisa lewat dari itu dan akan terbaca sebagai provider rusak |
| Per sapuan | 90 s | `WriteTimeout` server 120 s (`cmd/app-serv/main.go:134`); 90 s meninggalkan ruang untuk menulis jawaban daripada dipotong di tengah respons |
| Default | 6 model | satu klik punya biaya terpasang yang bisa disebut di tombol |
| Ceiling | 20 model | `limit` yang lebih besar di-clamp dan **dijawab** dengan `tested`, bukan diam-diam dipotong |
| Urutan | sekuensial, urutan registry | order jawaban tidak boleh bergantung pada model mana yang kebetulan jawab lebih dulu |
| Probe gagal | tetap baris, HTTP 200 | "model mana yang mati" adalah temuannya |
| Klien pergi | `ctx.Err()` diperiksa sebelum probe → error, bukan baris | tidak menghias pemutusan koneksi sebagai hasil probe |
| Budget habis | `stopped="deadline"`, `tested < total` | baris yang tidak sempat dijalankan adalah angka, bukan keheningan |

Probe yang kehabisan budget diberi kode sendiri, `MODEL_TEST_TIMEOUT`, bukan
`INTERNAL_ERROR`: yang pertama mengatakan "tunggu lebih lama atau coba lagi", yang kedua
menuduh gateway.

### 2.3 Persistensi

Tidak ada kolom baru. Preseden yang diikuti adalah `POST /provider-nodes/{id}/test`
(`provider_node_probe.go:38`), yang hasilnya memang tidak disimpan: verdict milik probe
terakhir, bukan milik baris. `upstream_endpoints.test_status` tetap hanya untuk uji konektivitas.

### 2.4 app-ui

- **Kolom `Test` per baris** di `ModelCatalogTable.svelte`: `Not tested` → `Testing` →
  jawaban (`Answered in 214 ms`, atau `Failed in 8 ms` di baris pertama dan
  `RATE_LIMITED: quota spent` di baris kedua, warna `--color-danger`).
- **Satu store, dua pintu masuk** (`stores/model-test.svelte.ts`): tombol baris dan sapuan
  menulis record yang sama, jadi sapuan mengisi baris dan satu baris bisa diuji ulang tanpa
  mengulang sapuan. Baca katalog baru menghapus jawaban sebelumnya (`clear()`), karena barisnya
  boleh jadi bukan baris yang sama.
- **Tiga keadaan yang dibedakan, bukan digabung**: baris gagal (`probed`, `ok:false`) adalah
  jawaban model; `unreachable` adalah keadaan di mana panel tidak pernah mendapat jawaban
  (rutenya menolak / jaringan putus). Keduanya teks berbeda di layar.
- **Tombol sapuan menyebut biayanya**: "Test the first 6 models", dan `limit` dikirim
  eksplisit supaya label dan permintaan tidak bisa berbeda.
- **Ikon, bukan emoji**: `ROW_ACTION_ICONS.test` (`Activity`, alasan sudah tertulis di
  `icons.ts:149`) untuk aksi baris; `LoaderCircle animate-spin` untuk keadaan berjalan,
  mengikuti preseden `StateMessage.svelte`. Diuji dengan `expectIconOnly`.
- Baris non-chat **tidak mendapat tombol** dan menulis `Not a chat model`. Kontrol yang hanya
  bisa menghasilkan jawaban menyesatkan adalah kontrol mati (R-26).

## 3. Keputusan design yang sengaja diambil (dan yang tidak)

| Keputusan | Kenapa |
|---|---|
| Hasil per baris, bukan banner bersama | reference memakai satu banner untuk semua baris (`page.js:1786`); di panel yang menampilkan 40 baris, jawaban tanpa nama modelnya tidak bisa ditindaklanjuti |
| `status` hanya ada saat gagal | `omitempty` di wire; mengisi 200 untuk probe sehat berarti menampilkan angka yang tidak pernah dikirim siapa pun (R-17, R-36) |
| `tested`/`total` dijawab, `truncated` tidak | duplikasi: `tested < total` sudah menyatakan pemotongan; satu angka tambahan hanya memberi kesempatan berbeda pendapat |
| Sapuan tanpa SSE | `usage-live.ts` tersedia, dan itu bentuk yang benar untuk *ratusan* baris; untuk budget 20 probe, satu jawaban dengan `stopped` lebih mudah dibaca dan tidak menambah jalur streaming |
| `kind` media tidak diuji lewat route ini | reference punya `kind` untuk embedding/image/stt/systemone; `app-serv` punya jalur media sendiri (`router_media.go`, §7.10) dan permintaannya bukan `messages`. Memakainya di sini berarti menulis jalur egress kedua |
| Tidak ada rate limit khusus route | dua route ini sudah dibatasi sesi management + budget di atas; menambah limiter kedua menumpuk tanpa alasan yang bisa ditulis satu baris (§1.6 minta alasan, bukan angka) |

## 4. Yang diwarisi dari reference, dan yang tidak

**Diwarisi:**
- `max_tokens=1024` untuk probe (`ping.js:174`). Sudah ada sebagai `PingMaxTokens`
  (`dataplane/ping.go:40`) karena pass combo membaca alasan yang sama; tidak diubah.
- Bentuk jawaban per model (`modelId`, `ok`, `latencyMs`, `error`, `status`) dan penempatannya di
  baris katalog (`ModelRow.js`), termasuk tombol per baris sebagai permukaan utama.

**Tidak diwarisi, dan alasannya tertulis:**
- Fan-out tanpa batas + timeout yang menjatuhkan seluruh route (lihat §2.2).
- Prompt. Reference mengirim `"hi"`, `app-serv` sudah punya `PingPrompt = "ping"`. Isinya tidak
  dibaca — yang dinilai adalah apakah pipeline selesai — jadi tidak ada yang diselaraskan, dan
  tidak ada dua prompt di dalam satu binary.
- Echo badan upstream mentah 500 karakter ke klien (`ping.js:193`). Baris kegagalan di sini memakai
  `dataplane.Error.Message`, yang sudah lewat pembatas dan pembersihan milik dataplane.
- kredensial di URL (`?key=` pada probe Gemini, `testUtils.js:575`) — tidak ada padanannya di sini;
  penempatan kredensial milik `provider/plugin_credential.go`.

## 5. Gerbang

Diukur di host yang sama, working tree ini. Angka ditulis dari jalan yang sebenarnya, bukan dari
harapan; yang belum selesai ditandai apa adanya.

| Gerbang | Hasil |
|---|---|
| `go build ./...` | bersih |
| `go vet ./...` | bersih |
| `gofmt -l .` | bersih |
| `go test -race -count=1 ./...` | **17/17 paket `ok`**, 0 kegagalan, 0 data race |
| `scrypts/gates/go-headers.sh` | **1038** berkas Go ber-header lengkap |
| `scrypts/gates/contract-openapi.sh` | dokumen tersaji = YAML |
| `staticcheck ./...` | bersih |
| `staticcheck -tags=integration ./...` | bersih |
| `golangci-lint run` | **0 issues** |
| limit baris (AGENTS.md §1.1) | 0 FAIL; **1 warning**: `internal/router/router.go` 231 (229 sebelum pass ini, +2 karena satu field Deps dan satu panggilan register) |

## 6. File yang berubah

**Baru, `app-serv`:** `internal/schema/provider_model_probe.go` (66), `internal/service/provider_model_probe.go` (187),
`internal/service/provider_model_budget.go` (120), `internal/handler/provider_model_probe.go` (132),
`internal/router/router_provider_model_probe.go` (32), tiga berkas test
(`provider_model_probe_test.go` 193, `provider_model_probe_sweep_test.go` 173,
`provider_model_probe_budget_test.go` 97).

**Diubah, `app-serv`:** `service/combo_probe.go` (port `ComboProber` → `ModelProber`, dipakai dua layanan),
`router/router.go` dan `router/router_models_test.go` (+2 masing-masing), `cmd/app-serv/{management_handlers,router_wiring}.go`,
`handler/model_repo_stub_test.go` (fixture menambah handler model-test di atas prober yang sudah ada),
`handler/openapi.json` (dihasilkan ulang), `docs/CONTRACT/001-CONTRACT-API-V1.yaml` (+2 path, +4 schema).

**Baru, `app-ui`:** `schemas/model-test.ts` (123), `stores/model-test.svelte.ts` (102),
`components/ModelCatalogSweep.svelte` (44), dua berkas test (`tests/schemas/model-test.test.ts` 28 tes,
`tests/components/model-catalog-probe.test.ts` 12 tes).

**Diubah, `app-ui`:** `api/providers.ts` (+2 fungsi), `components/ModelCatalogTable.svelte` (kolom Test),
`components/ModelCatalogList.svelte` (store + penempatan sapuan), `tests/support/model-stub.ts` (dua route terdaftar).

## 7. Yang sengaja TIDAK dikerjakan

- **`/models/availability`** (model mana yang sedang cooldown dan sampai kapan) — satu-satunya di
  daftar F10 yang menjawab pertanyaan yang belum bisa dijawab panel sama sekali. Datanya ada di
  `upstream_key_health.go`, bentuk jawabannya belum. Masuk pass berikutnya, bukan ditambal sekarang.
- **Empat route siklus hidup lain** (`DELETE /models/alias`, `POST/DELETE /models/disabled`,
  `POST /providers/test-batch`, dan bentuk batch-nya): `app-serv` memakai `PUT`-replace untuk alias
  dan disabled, itu keputusan §7.6 yang sadar dan lebih mudah diaudit.
- **Probe non-chat.** Reference membedakan `kind` (embedding/image/stt/systemone). `app-serv` punya
  jalur media sendiri (§7.10) dengan bentuk permintaan yang berbeda; menyatukannya ke probe ini
  berarti menulis jalur egress kedua, yang justru aturan §1.5/§1.6 larang.
- **Persistensi verdict per model.** Tidak ada kolom baru; lihat §2.3.
- **Streaming hasil sapuan.** Lihat §3.
- **`app-ui/src/routes/providers/[provider_id]/+page.svelte` tidak disentuh.** Store test dibuat di
  dalam `ModelCatalogList`, bukan di halaman, supaya satu pembaca katalog tambahan tidak perlu
  tahu-menahu tentang probe.

## 8. Cara menjalankan

```bash
# satu model
curl -s -X POST localhost:9090/api/v1/providers/openai/models/test \
  -H "Cookie: pannel_session=<sesi>" -d '{"model_id":"gpt-4o"}'

# sapuan terbatas
curl -s -X POST localhost:9090/api/v1/providers/openai/test-models \
  -H "Cookie: pannel_session=<sesi>" -d '{"limit":3}'
```

Panel: `/providers/<id>` → bagian Model catalog → kolom **Test** per baris, atau tombol
"Test the first 6 models" di di atas tabel.

Gate `app-ui`, dijalankan di `app-ui/` pada working tree ini:

| Gerbang | Hasil |
|---|---|
| `bun run test` | **2800 tes lulus dari 174 berkas**, 2829 s. Hijau di percobaan pertama. Dua berkas baru ikut: `tests/schemas/model-test.test.ts` (28 tes) dan `tests/components/model-catalog-probe.test.ts` (12 tes) |
| `bun run check` | `svelte-check found 0 errors and 0 warnings` |
| `bun run lint` (prettier `--check .`) | semua berkas lolos, termasuk dua berkas yang lebih dulu ditulis tanpa jalan format |
| `bun run lint:ts` (eslint) | bersih |
| `bun run build` | `BUILD_EXIT=0`, tanpa error; peringatan satu-satunya adalah biaya hook `sveltekit-guard` dari rolldown, tidak berkaitan pass ini |

Waktu 2829 s lebih lama dari 1945 s yang dicatat draft 019: `go test -race ./...` untuk pass ini
berjalan di atas host yang sama, dan load 5–6 itu yang menahan fork Vitest (CPU time runner hanya
3 m 20 d dari 45 m jalan). Tidak ada suntingan di antara keduanya, jadi angkanya dilaporkan apa
adanya, bukan dibandingkan seperti regresi.

## 9. Delivery Gate antislop (mode DURING)

Report untuk permukaan yang pass ini bangun. Butir penuh dan cara memeriksanya ada di
`anti-slop/audit-002-2026-09-27.md`.

**Block 1, Hard Gate.** R-02 PASS di berkas baru (0 karakter `—` pada copy yang dirender;
satu temuan di `EndpointDeleteDialog.svelte:34` adalah milik pass sebelum ini, dilaporkan, tidak
diubah tanpa persetujuan). R-03 PASS — tabel katalog menggulir horizontal dalam box-nya, pola yang
sama dipakai `EndpointKeysTable` (46rem) dan `ProviderTable`; `min-w` 56rem, tidak ada teks yang
keluar kontainer, target sentuh `min-h-11 min-w-11`. R-17/R-18/R-36/R-38 PASS — tidak ada angka
yang tidak dikirim API; `status` dihilangkan pada probe sehat daripada diisi 200. R-23 PASS — tidak
ada aset visual baru; ikon memakai registry yang sudah ada. R-24/R-28 PASS — tidak ada nav item atau FAQ.
R-25 PASS — hanya token yang sudah diukur `tests/tokens/contrast.test.ts`. R-26 PASS — baris
non-chat tidak mendapat tombol. R-27 PASS — empat keadaan ditulis dan diuji satu-satu. R-32 PASS —
`<button>` asli, indikator fokus bawaan tidak dibuang. R-33/R-34 PASS — ditulis di source, tidak
ada toggle tema. **R-35 PASS — klik-through tercatat.** Lihat §10.

**Block 2, Purpose-Gate.** R-04 PASS — `Activity` untuk probe alasannya sudah tertulis di
`icons.ts:149`; `LoaderCircle animate-spin` mengikuti `StateMessage.svelte:26`. R-01/R-09/R-10/
R-12/R-13/R-19/R-22 PASS — tidak ada gradien, glow, glass, badge dekoratif, bayangan baru, animasi
template, atau ilustrasi.

**Block 3, Liveliness.** DIAL dipertahankan dari `DESIGN.md`: ENERGY 1 / RHYTHM 2 / MOTION 1.
Focal point layar tetap tabel model; satu-satunya gerakan adalah spinner keadaan berjalan.
Aksen tetap satu (`--color-accent` tidak dipakai untuk hal baru); motif identitas panel (tabel
rapat, angka dari API) justru yang ditonjolkan kolom Test.

**Block 4, Craftsmanship & Quality Locks.** C-2 PASS (tiap kontrol punya efek nyata, terbukti di
tes). C-4 PASS untuk state dan breakpoint; temuan #4 di audit mencatat tabel menggulir lebih awal.
C-5 PASS (tak ada klaim). R-05/R-11/R-15/R-16/R-20/R-21/R-29/R-30/R-31 PASS — tidak ada seksi baru,
radius dan ritme warisan tabel, label tombol spesifik ("Test the first 6 models"), bukan buzzword.


## 10. Klik-through live (R-35)

Cara: binary hasil pass ini (`go build ./cmd/app-serv`) dijalankan di `127.0.0.1:9091` dengan `.env`
yang sama; instance panel dev kedua di `127.0.0.1:3101` dengan `PANEL_API_TARGET=http://127.0.0.1:9091`.
Proses owner (:9090, :3000) tidak disentuh; keduanya dimatikan lagi setelah selesai. Chrome tanpa
interface dikemudikan Playwright; login memakai kata sandi dev yang owner berikan di sesi ini.

Provider sasaran dipilih **yang tidak punya endpoint sama sekali** supaya tidak ada panggilan
inferensi nyata ke akun siapa pun: `tencent` (2 model chat, "No endpoint configured").

| Yang diklik | Yang muncul di layar |
|---|---|
| `Sign in` | sesi diterima, panel pindah dari `/login` |
| `/providers` | 35 baris registry, 33 tanpa endpoint |
| `/providers/tencent` | kolom **Test** ada di tabel katalog; 2 baris menawarkan probe; keduanya `Not tested`; 0 baris `Not a chat model` (provider ini hanya punya model chat) |
| tombol baris pertama | `Failed` + `NO_PROVIDER_AVAILABLE: no upstream endpoint is configured for provider tencent`, di baris model itu sendiri |
| `Test the first 6 models` | HTTP 200; badan jawaban `{provider_id:"tencent", source:"registry", tested:2, total:2, results:[…]}`; ringkasan `Tested 2 models: none answered.`; 2 baris terisi |
| Tab ke tombol Test | kontrol fokus (`aria-label="Test"`), outline `solid`, tanpa `box-shadow` |
| console | 0 error; 0 request gagal selain `POST /api/v1/auth/login` yang dibatalkan navigasi sesudah sign-in |

Screenshot: `/tmp/p037-01-idle.png`, `/tmp/p037-02-row.png`, `/tmp/p037-03-sweep.png`.

Dua hal **tidak** teramati di jalurnya sendiri dan tetap dibuktikan lewat tes, bukan klaim:
`Testing` (probe ke provider tanpa endpoint gagal dalam mikrodetik, jadi state berjalan tidak
sempat tertangkap layar; `tests/components/model-catalog-probe.test.ts` memegang state itu dengan
fetch yang ditahan), dan baris `Not a chat model` (`tencent` tidak punya model media; kasusnya ada
di tes yang sama dengan katalog dua baris).

## 11. Satu pengamatan di luar scope, dicatat bukan ditambal

`GET /providers` (juga `/usage`, `/combos`, `/api-docs`) dijawab panel dengan **500 `Internal
Error`** ketika permintaannya tidak punya cookie sesi, padahal `app-serv` membalas 401
`UNAUTHORIZED` dengan benar. `src/routes/+layout.ts` tidak punya cabang 401 → redirect, jadi
pembaca yang belum login melihat error, bukan halaman login. Ini bukan efek pass ini (tidak satu
pun berkas auth/layout/page disentuh; tiga rute yang sama persis perilakunya tidak tersentuh
pekerjaan ini) dan masuk kategori R-27 untuk pemiliknya putuskan.

## 12. Susulan (2026-09-29): node compatible belum ikut teruji

Owner melaporkan OpenAI-Compatible dan Anthropic-Compatible belum punya mekanisme test by models.
Ternyata gap-nya dua lapis, dan yang pertama tidak terlihat dari layar:

1. **Backend.** `ProviderService.Models()` menjawab daftar model node dari **upstream**-nya
   (`modelsFor` → `ListNodeModels`), dan itu kosong ketika upstream tidak menjawab — sementara baris
   yang operator deklarasikan di `models_custom` justru yang tampil di layar node dan justru yang
   bisa di-route. Sapuan atas node seperti itu menjawab `VALIDATION_ERROR: provider … offers no chat
   model to test` di samping tabel yang penuh model.
2. **Panel.** Cabang node di `/providers/[provider_id]` tidak memakai `ModelCatalogList` sama sekali;
   bagiannya adalah `ProviderCustomModels` → `CustomModelTable`, yang belum punya kolom Test.

**Perbaikan.** Aturan "model mana yang bisa diuji" ditaruh di pemilik daftar model, bukan diduplikasi
di service probe: `ProviderService.ProbeTargets(ctx, providerID)` menjawab `(list, targets)` dengan
`targets` = daftar ter-resolve ∪ baris deklarasi, dedupe by model id, urutan daftar dulu. Kalau kedua
sisi menyebut satu model, baris ter-resolve **dipertahankan** karena ia yang membawa `kind` (penentu
apakah probe chat bisa menjangkau model itu); baris deklarasi hanya mengisi nama yang diam. Preseden
urutan dan override diambil dari `ModelCatalogService.lookups` (§7.6), dengan satu perbedaan yang
ditulis di komentar: katalog meng-override baris registry dengan baris custom, sedangkan probe tidak
boleh kehilangan `kind`-nya.

**Panel.** `CustomModelTable` mendapat kolom Test; `ProviderCustomModels` membuat tombol sapuan
**hanya untuk node** (`prefix !== null`), karena layar registry provider sudah punya satu di atas.
Store probe dipindah ke halaman (`+page.svelte`) dan dioper ke kedua bagian: baris deklarasi sebuah
registry provider tampil dua kali di layar itu (sebagai custom row dan di dalam catalog gabungan),
dan dua verdict untuk satu model adalah dua pendapat tentang satu fakta. Sel keadaan diangkat ke
`ModelTestState.svelte` supaya kedua tabel tidak punya cara sendiri untuk menampilkan kegagalan.

Satu perubahan kecil yang perlu disebut: `store.running` kini getter, bukan `$derived`. Store dibuat
di luar component (halaman mengopernya ke test, dan test membuatnya sendiri) tidak boleh butuh effect
scope agar bisa ada.

**Nuansa yang sengaja dibiarkan terbaca.** Pada node yang upstream-nya menjawab 40 model, tabel ini
hanya menampilkan baris deklarasi operator, sementara sapuan berjalan atas gabungan (union) dan bisa
saja menguji 6 model yang tidak ada di tabel. Itu bukan bug yang ditutupi: ringkasan menjawab
`Tested 6 of 40 models`, jadi angkanya yang mengatakan bahwa yang diuji bukan isi tabel saja, dan baris
yang tidak tersentuh tetap terbaca `Not tested`. Meng-clip sapuan ke baris yang tampil saja berarti
menyembunyikan separuh dari kebenaran demi angka yang lebih enak dibaca.

**Bukti.** `internal/service/provider_model_probe_node_test.go` (6 fungsi, termasuk satu tabel): node
tanpa upstream tetap menyapu baris deklarasi; union tanpa duplikat dan urutan `gpt-4o, gpt-4o-mini,
deepseek-v3`; baris deklarasi registry ikut tersapu; gagal baca `models_custom` adalah error, bukan
daftar yang tampak kosong; nama model datang dari baris deklarasi; string passthrough tetap diuji.
`tests/components/provider-node-model-test.test.ts` (7 tes): aksi per baris di layar node, tombol
sapuan yang menyebut budget, permintaan `providers/<node>/models/test` atas id yang baris itu sebut,
`UNAUTHORIZED` + pesan di barisnya, satu sapuan mengisi dua baris, penolakan sapuan menampilkan
kalimat server, dan bagian supplement registry tidak menawarkan sapuan kedua.

## 13. Susulan (2026-09-29): round-robin tiga member terverifikasi, satu temuan ditambal

Uji live atas combo `round_robin` tiga member free (space-bunny-free → mimo-v2.6-flash-free →
muse-spark-1.3-contributor-free) mengonfirmasi distribusinya: urutan rotasi tepat dan
`sticky_limit: 1` berperilaku benar (satu request = satu member, cursor maju). Yang terbawa dari
uji itu tiga temuan, dan hanya satu yang menjadi milik gateway.

**1. muse-spark menghabiskan ceiling untuk reasoning — ditambal.** Dengan `max_tokens: 60` model
ini menjawab **200** dengan `content: ""` + `finish_reason: "length"`: 60 token habis untuk
thinking, nol untuk jawaban. Walk combo (`engine.go`) hanya pindah member ketika sebuah member
**gagal**, jadi kosong itu dihidangkan apa adanya; clamp 16 token yang ada di connector pun adalah
floor agar upstream tidak menjawab 400, bukan floor agar jawaban muat. Tambalnya dua lapis, dan
sengaja dua lapis karena keduanya menjawab sisi yang berbeda dari satu gejala:

- **Sisi request.** `min_output_tokens` menjadi field deklarasi per model di registry, dan
  connector OpenCode menaikkan ceiling yang sudah dikirim client ke nilai itu (512 untuk
  muse-spark 1.3 free). Ceiling yang tidak dikirim tidak di-inject; yang sudah di atas floor
  lewat apa adanya.
- **Sisi respons.** Jawaban non-stream yang kosong dan berhenti karena ceiling ditandai lewat
  `Outcome.Truncated`, dan combo walk mencoba member berikutnya sebelum body itu dihidangkan.
  Sengaja **bukan** error: `answer()` mencatat setiap error sebagai kegagalan kredensial
  (`engine_relay.go:179` → `failureClass`), padahal yang terjadi adalah model yang menjawab sesuai
  permintaannya — mem-park key karena itu salah sasaran. Kalau semua member sama kosongnya, body
  kosong terakhir yang dihidangkan (dengan `finish_reason: length` utuh), bukan 502: itu persis
  yang client dapat sebelumnya, dan itu yang memberi tahunya bahwa ceiling-nya terlalu kecil.
  Client streaming tidak di-walk ulang — frame-nya sudah sampai.

**2. Reasoning tidak muncul sebagai konten — dicatat.** Pada space-bunny dan mimo, thinking hanya
terlihat sebagai `usage.completion_tokens_details.reasoning_tokens`; tidak ada `reasoning_content`
seperti pada qoder/qfmodel, baik di mode non-stream maupun stream. Ini kemampuan upstream, bukan
perilaku gateway: gateway meneruskan yang ada. Konsekuensinya tercatat di §7.7: jawaban
reasoning-only tetap dihitung "menjawab", karena fold memang mempertahankan reasoning saat
content kosong.

**3. Format response id tidak seragam — dicatat.** `070a…` (space-bunny), `gen-…` (mimo),
`resp_…` (muse) karena tiga upstream berbeda. Tidak ada satu pun kode gateway yang mem-parse id
jawab, jadi tidak ada yang perlu ditambal; yang tidak boleh dilakukan pemakai adalah mengandalkan
formatnya. Yang justru sudah dinormalkan adalah **nama model** (§7.6, 2026-09-28), bukan id.

**Bukti.** `internal/registry/opencode_free_models_test.go` (floor ter-decode, dan id yang sama
di bawah dua entry OpenCode wajib menyebut angka yang sama);
`internal/provider/opencode_wire_test.go` (60 → 512, ceiling di atas floor tidak berubah, ceiling
yang absent tetap absent, tanpa floor tetap clamp 16);
`internal/dataplane/engine_answer_completeness_test.go` (22 kasus lintas tiga wire: kosong+length
vs ada content, tool call, reasoning-only, `finish: stop`);
`internal/dataplane/engine_truncated_failover_test.go` (lima kasus: combo skip ke member sehat
tanpa mem-park key, semua kosong → body kosong terhidang, member tunggal tidak berubah, jawaban
kosong mengalahkan error member berikutnya, client stream tidak diulang).
Gerbang: `go build ./...`, `go vet ./...`, `go test -race ./...` bersih (`go-headers.sh` dan
`go-lint.sh` PASS; staticcheck + golangci-lint 0 issue).

**Batas yang tidak diklaim.** Pengukuran live atas combo tiga member dengan `max_tokens: 60` **masih
milik operator** dan belum dijalankan untuk pass ini: gateway di `:9090` adalah proses `go run`
yang binary-nya lebih tua dari patch ini, sehingga panggilan apa pun ke sana hari ini menguji kode
lama, dan norma draft 034 §9 melarang proses itu dihentikan atau dimulai ulang oleh pass pengujian.
Kunci gateway juga tidak tersimpan di repo. Yang terbukti di dalam repo bukan pengganti pengukuran
itu, hanya rantainya: `engine_opencode_free_floor_test.go` menjalankan pipeline nyata (registry
ter-embed + connector OpenCode asli + engine) dan membaca body yang sampai ke upstream — 60 menjadi
512 untuk muse-spark 1.3, dan tetap 60 untuk member combo di sebelahnya;
`engine_truncated_failover_test.go` menjalankan walk combo nyata atas jawaban kosong. Yang tersisa untuk mata kepala: apakah upstream sungguhan menjawab 512 itu dengan konten, dan apakah
`reasoning_content` tetap tidak muncul di lane space-bunny/mimo (temuan #2).
