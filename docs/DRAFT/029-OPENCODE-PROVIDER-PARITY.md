# 029-OPENCODE-PROVIDER-PARITY.md: keluarga provider OpenCode `app-serv` terhadap reference 9Router

Analisis penuh dan perbandingan keluarga provider OpenCode (Free, Go, Zen, System One) pada `app-serv/.`
terhadap reference `https://github.com/decolua/9router`, dengan pemeriksaan porting yang presisi: setiap
temuan menyitir baris kedua sisi dan diukur dengan probe Go yang dijalankan lalu dihapus dari tree, pola
yang sama dengan draft 027 dan 028. Bukan kontrak; kontrak tetap `docs/SPEC-API/001-SPEC-API.md` (wire)
dan `docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel).

| | |
|---|---|
| **Status** | **OPEN, dokumen analisis + pass uji free tier.** Dua bagian: §4 F1-F7 adalah analisis (tidak ada berkas source diubah); §4.8 adalah pass uji yang owner minta, dan itu **mendaratkan test permanen** `internal/dataplane/engine_opencode_free*_test.go` (4 berkas, 626 baris) yang menguji ketiga model free tier tanpa auth. Test hijau, lolos mutation check, dan lolos live ke `opencode.ai` |
| **Mechanism** | DURING & AFTER (antislop) |
| **Scope** | `app-serv/.` keluarga provider OpenCode: registry, connector, transform body, capability, pricing, permukaan usage. `app-ui/.` tidak disentuh |
| **Permintaan owner** | Pass 1: "Full analisis dan perbandingan Provider (OpenCode): app-serv VS 9router (reference). Cek portingan secara presisi." Pass 2: "Yang harus di uji test dan harus lolos dari OpenCode hanya free tier aja ... tanpa auth, tanpa api key. MODELS: space-bunny-free, mimo-v2.6-flash-free, muse-spark-1.3-contributor-free" |
| **Reference** | `decolua/9router`, checkout `/home/rusmanadodi/apps/9router`, **`21583c03` = tag `v0.5.85` (2026-09-22)**, sasaran yang sama dengan draft 017/027/028. Checkout berada di `39e36d3d` = `v0.5.86` (2026-09-23); `git diff 21583c03..39e36d3d` atas seluruh berkas keluarga OpenCode keluar kosong kecuali satu baris di `thinkingLevels.js` (pola `*mimo*v2.6*`), jadi semua sitasi berlaku untuk keduanya |
| **Kaitan** | SPEC-API-001 §7.15, §7.12, §8; draft 011 §8; draft 017 (F2, §7.2 nomor 3); draft 027 §3.1-§3.3 (baris 9, 19, 27, 29, 39, 40), F4, F6; draft 028 (klasifikasi error); changelog SPEC 2026-09-20 (OpenCode Free discovery fix) |
| **Tanggal** | 2026-09-24 |

---

## 1. Ringkasan

Keluarga OpenCode di reference adalah **empat provider**, bukan satu: `opencode` (Free, tanpa auth),
`opencode-zen` (PAYG ber-kunci), `opencode-go` (langganan $5/mo ber-kunci), dan System One (Jev),
yang terakhir bukan provider terpisah melainkan satu `kind: "systemone"` yang dinyatakan pada model
`jev-1.13`/`jev-1.13-free` di tiga entri registry (`opencode`, `opencode-zen`, `openrouter`) dan
dilayani lewat satu route data plane `POST /api/v1/systemone`.

Di `app-serv`, keluarga ini hari ini adalah **dua entri registry** (`opencode`, `opencode-go`) dan
**satu connector** (`internal/provider/opencode.go`). Porting OpenCode Free sendiri sudah dalam
kondisi terukur yang baik: daftar modelnya dipulihkan commit `34f2c38` (2026-09-20), connector-nya
mendarat commit `440293d` (2026-09-22), dan sejak itu jejak perbaikan reference diikuti satu per satu
(session kanonik `0c6ab4f9`, reasoning item `eafac37d`, forced stream `93837af0`). Yang belum diukur
sampai dokumen ini adalah sisa keluarganya.

Hasilnya dalam angka:

| | Reference `21583c03` | `app-serv` |
|---|---|---|
| Entri registry keluarga OpenCode | 3 (`opencode`, `opencode-zen`, `opencode-go`) | 2 (`opencode`, `opencode-go`) |
| Model `opencode` (Free) | 4 (3 chat + 1 systemone) | 3 (chat saja) |
| Model `opencode-go` | 28 | 10 |
| Model `opencode-zen` | 74 | 0 (entri tidak ada) |
| Route System One | 1 (`POST /api/v1/systemone`) | 0 |
| Endpoint transport per provider | 1-3 (`transports[]` per format klien) | 1 (`base_url` tunggal) |
| Alat fingerprint free tier | 4 (`bash`, `glob`, `grep`, `read`) | 2 (`bash`, `read`) |
| Baris executor + util milik keluarga | 994 (487+315+192) | 562 (5 berkas connector `opencode*.go`, tanpa test) |
| Test unit keluarga | 1.616 baris (9 berkas) | 745 baris (5 berkas) |
| Provider-reported usage | 2 provider (`features.usage`) | 0 consumer di Go (struct ada, tidak dibaca siapa pun) |

Tujuh temuan tercatat; tiga di antaranya HIGH dan membuka jalur live, dan seluruhnya diukur dengan
probe Go yang keluarannya dikutip di §3.2:

| # | Tingkat | Isi singkat |
|---|---|---|
| **F1** | HIGH | `opencode-zen` tidak ada sama sekali: 74 model, jalur kredensial API key, dan permukaan usage PAYG-nya tidak dapat dinyatakan operator |
| **F2** | HIGH | Entri `opencode-go` tertinggal pada pin lama `db4499d`: 23 model reference tidak ada, 5 model yang dideklarasikan sudah tidak ada di reference, dan tabel `transports[]` tiga-endpoint (chat/messages/responses per format klien) tidak punya field padanan sehingga dijatuhkan generator |
| **F3** | MEDIUM | Fingerprint free tier belum dedup: kuartet dan kanonisasi **bukan** syarat upstream (terukur, §4.8), tetapi **duplikat nama persis** (`bash`+`bash`) menjawab 400, dan connector tidak punya dedup |
| **F4** | LOW | `union-alpha` di-POST ke `/zen/v1/messages` tanpa header `anthropic-version` yang reference kirim persis pada URL itu (`opencode.js:484`). Prioritas turun setelah pengukuran: model itu menjawab 401 `ModelError` di jalur tersebut dengan atau tanpa header, jadi ini kesetiaan porting, bukan cacat yang terukur merusak permintaan |
| **F5** | MEDIUM | Capability vision model OpenCode salah semua di korpus dan di resolver: reference mendeklarasikan `vision: true` untuk kedua `muse-spark-*-free` dan `union-alpha` (`capabilities.js:144-147`), port menjawab `false` untuk keduanya karena korpus digenerate dari pin `db4499d` saat entri itu belum ada di reference |
| **F6** | MEDIUM | System One (Jev) tidak diputuskan: route, `kind: systemone`, dan `service_kinds` tidak ada, dan `Model.IsChat()` akan menolak kind itu sebagai tidak routable bila ditambahkan tanpa keputusan; draft 027 F6 sudah mencatatnya sebagai surface gap tetapi tanpa pengukuran keluarga OpenCode ini |
| **F7** | LOW | Perbedaan presisi transform body pada wire Responses dan chat: `tool_choice` dipaksa `auto` juga pada model non-quirk (reference hanya menurunkan model ber-quirk; di kasus absent keduanya sudah sama), chat wire tanpa tools klien tidak menulis `tool_choice:none` sehingga decoy bisa dipilih model, `store:false` dan `reasoning:{effort,summary}` tidak pernah ditulis, input string kosong/array kosong tidak dinormalisasi, `call_id` lebih dari 64 karakter dan `arguments` berbentuk objek tidak dikoreksi pada jalur same-format, dan alat chat-shape tidak di-flatten ke bentuk flat Responses |

Satu temuan baru dari pass uji (§4.8), dengan tingkat tersendiri:

| # | Tingkat | Isi singkat |
|---|---|---|
| **F8** | MEDIUM | Operator tidak dapat memakai free tier tanpa membuat `upstream_endpoints` lebih dulu: reference menyuntik koneksi virtual untuk provider `noAuth` (`src/sse/services/auth.js:45-63`), `app-serv` menjawab `NO_PROVIDER_AVAILABLE` saat nol endpoint (`selection_candidates.go:52`). Test pass ini memakai endpoint `UpstreamAuthNone` eksplisit, jadi yang terbukti adalah "endpoint tanpa kunci", bukan "tanpa endpoint sama sekali" |

Dua catatan pembacaan sebelum temuan:

1. **Registry pin adalah keputusan terbuka yang sudah tercatat** (draft 011 §8, draft 017 §7.2 nomor 3),
   jadi fakta bahwa `registry.yaml` menyandang `revision: 9router@db4499d (2026-06-19)` bukan temuan
   baru. Yang menjadi temuan (F2) adalah akibat presisinya pada keluarga ini: entri `opencode-go`
   tidak pernah diperbarui sejak pin, sementara reference menerima 27 commit pada berkas keluarga
   ini sesudahnya, termasuk empat yang mengubah perilakunya: `8efacc11` (2026-06-20, selaraskan model
   Go dengan endpoint resmi), `e1115e28` (2026-08-14, `transports[]` + guard per-model), `702b57c3`
   (2026-09-17, model responses-only), `81f4f930` (2026-09-05, header session stabil). F1 dan F6
   akibatnya sama tetapi akar berbeda: `opencode-zen` dan System One tidak ada **field-nya** di
   `registry.yaml` sekalipun, jadi regenerasi pun tidak akan membawanya tanpa keputusan schema lebih
   dulu.
2. **Yang dimaksud "presisi" di sini sama dengan draft 027**: kesesuaian terhadap perilaku reference
   pada revisi sasaran dan terhadap klaim tertulis `app-serv` sendiri. Setiap temuan diukur dengan
   probe Go yang memanggil kode nyata (`registry.Load()`, `NewOpenCode`, `ResolveParts`) dan keluarannya
   dikutip apa adanya di §3.2; bentuk permanennya ada di §7.2 supaya bisa dipasang kembali sebagai test
   penerimaan.

## 2. Ledger keluarga OpenCode

Satu baris per konsep. Kosakata status sama dengan draft 027 §3.

| # | Konsep reference | Sumber reference (`21583c03`) | Padanan `app-serv` | Status |
|---|---|---|---|---|
| 1 | Entri registry `opencode` (Free, `noAuth`, `hasFree`) | `open-sse/providers/registry/opencode.js` | `registry.yaml:1144-1172` | 1:1 kecuali model `jev-1.13-free` (**F6**) |
| 2 | Entri registry `opencode-zen` (PAYG, `apikey`, alias `ocz`) | `registry/opencode-zen.js` (135 baris) | tidak ada | belum (**F1**) |
| 3 | Entri registry `opencode-go` (langganan, alias `ocg`) | `registry/opencode-go.js` | `registry.yaml:3125-3166` | pin lama: 10 dari 28 model (**F2**) |
| 4 | Model System One `jev-1.13`/`jev-1.13-free` (`kind: systemone`) | `registry/opencode.js:31`, `registry/opencode-zen.js:118-119`, `registry/openrouter.js:46` | tidak ada | belum (**F6**) |
| 5 | Route data plane `POST /api/v1/systemone` | `src/app/api/v1/systemone/route.js` + `src/sse/handlers/systemone.js` (152 baris, loop kredensial + fallback) | tidak ada | belum (**F6**, juga draft 027 F6) |
| 6 | Handler systemoneCore (pass-through JSON, header session) | `open-sse/handlers/systemoneCore.js` (95 baris) | tidak ada | belum (**F6**) |
| 7 | `serviceKinds: ["llm","systemone"]` pada 3 entri | `registry/opencode.js`, `opencode-zen.js`, `openrouter.js:48` | `service_kinds` di `opencode` hanya tersirat; `openrouter` di YAML kehilangan `systemone` | belum (**F6**) |
| 8 | Executor `opencode` (487 baris: session/request id, 3 leaf URL, header identitas, transform) | `open-sse/executors/opencode.js` | `internal/provider/opencode{,_body,_body_json,_decoys,_session}.go` (562 baris) | 1:1 pada jejak yang diikuti; sisa di **F3**, **F4**, **F7** |
| 9 | Executor `opencode-zen` (315 baris, fingerprint + session) | `open-sse/executors/opencode-zen.js` | tidak ada | belum (bagian **F1**) |
| 10 | Executor `opencode-go` (192 baris, responses-model, session) | `open-sse/executors/opencode-go.js` | fallback `provider.Default` | dipetakan sebagian (**F2**: URL claude-target salah) |
| 11 | `transports[]` multi-endpoint per format klien | `registry/opencode-go.js:31-35`, `opencode-zen.js:30-34`; `chatCore.js:89-101` | `Transport` struct tidak punya field `transports` | belum (**F2**) |
| 12 | `supportedFormats` per model (guard transport) | `registry/opencode-go.js` per model; `chatCore.js:91-96` | tidak ada field-nya | belum (**F2**) |
| 13 | Fingerprint kuartet + kanonisasi + restorasi respons | `open-sse/utils/opencodeFingerprint.js` (232 baris), commit `822aa958` | `opencode_decoys.go` (85 baris, 2 alat) | setengah port (**F3**) |
| 14 | Quirk `forceAutoToolChoiceModels` | `registry/opencode.js:22-24`; `executors/opencode.js:418-422` | `transformOpenCodeResponses` paksa `auto` untuk semua model | lebih luas dari reference (**F7**) |
| 15 | Session id kanonik `ses_` + translate + native forwarding | `executors/opencode.js:36-161` | `opencode_session.go` (derive dari endpoint id) | dipetakan: stateless, tanpa forwarding header klien (dicatat, bukan temuan; §5) |
| 16 | `x-opencode-request` id per turn | `executors/opencode.js:193-209, 471-475` | tidak ada | belum (dicatat di §5, tidak cacat terukur) |
| 17 | Header `anthropic-version` pada `/zen/v1/messages` | `executors/opencode.js:484` | `ApplyAuth` tidak menulisnya | belum (**F4**) |
| 18 | Capability vision muse-spark/union-alpha | `providers/capabilities.js:144-147` + pola `*muse*spark*` (:408) | `capability.go` + `testdata/capabilities_corpus.json:2341-2357` menjawab `false` | salah (**F5**) |
| 19 | Thinking levels muse-spark (`reasoning` hingga `xhigh`) | `capabilities.js:144-145` (`thinkingFormat: "openai"`) + `thinkingLevels.js` | tidak ada konsep level di registry Go | belum (bagian F4 metadata draft 027) |
| 20 | Pricing keluarga OpenCode | `providers/pricing.js`: tidak ada baris muse/union/jev (free, `getPricingForModel` mengembalikan `null`, biaya 0) | `pricing.json` (108 model, pin `21583c03`): juga tidak ada | 1:1 (non-finding, §5) |
| 21 | Usage provider-reported (`features.usage`, `usageApikey`) | `registry/opencode-go.js:69-72`, `opencode-zen.js:131-134`; `services/usage/{opencode-go,opencode-zen}.js` | `Features` struct ada, nol consumer; `UsageConfig` ada, nol consumer | belum (bagian **F1** untuk zen; dicatat untuk go) |
| 22 | Pengujian keluarga | `tests/unit/opencode-*.test.js` (9 berkas, 1.616 baris) | `internal/provider/opencode*_test.go` (5 berkas, 745 baris) + `registry/opencode_free_models_test.go` + `dataplane/resolve_opencode_test.go` | dipetakan: menutup subset |

## 3. Metode dan bukti

### 3.1 Revisi reference

```bash
cd /home/rusmanadodi/apps/9router
git log -1 --format='%H %ci %d'
# 39e36d3d0c849e0e01dfeacddf111edf892448fc 2026-09-23 10:05:33 +0700 (HEAD -> master, origin/master)
git describe --tags          # v0.5.86
git diff 21583c03..39e36d3d --stat -- open-sse/executors/opencode*.js \
  open-sse/providers/registry/opencode*.js open-sse/utils/opencodeFingerprint.js \
  open-sse/providers/capabilities.js open-sse/providers/pricing.js \
  open-sse/handlers/systemoneCore.js src/sse/handlers/systemone.js
# open-sse/providers/thinkingLevels.js | 1 +
# (seluruh berkas keluarga lainnya identik antara v0.5.85 dan v0.5.86)
```

Jejak perbaikan keluarga ini di reference, untuk kalimat "port mengikuti jejak perbaikan" di §1:

```bash
git log --oneline -- open-sse/executors/opencode.js
# 822aa958 fix(opencode): cloak Responses requests that already have tools      (fingerprint kuartet, 2026-09-19)
# 93837af0 fix(opencode): fix free tier 403 error and improve China region      (forced stream + decoy, 2026-09-18)
# aa14ef72 fix(opencode): normalize Muse Free tool choice                        (quirk auto-only, 2026-09-17)
# eafac37d fix(opencode): strip prior reasoning items on Muse Spark              (2026-09-17)
# 0c6ab4f9 fix(opencode): reuse one stable upstream session per identity         (2026-09-17)
# 6091ff59 fix(opencode): resolve 403 FreeTierError with canonical session       (2026-09-17)
# 2b65c49f fix(opencode): route Union Alpha through Messages API                 (2026-09-17)
# acb5c34c fix(opencode): route Muse Spark models to Responses API and declare vision (2026-09-03)
```

### 3.2 Cara mengukur

Probe ditulis sebagai berkas test sementara di paket yang tepat (`internal/dataplane` untuk resolusi
dan URL, `internal/registry` untuk capability, `internal/provider` untuk transform), dijalankan dengan
`go test -run 'TestProbe_' -v`, keluarannya dikutip, lalu berkasnya dihapus. Semua probe memanggil kode
produksi nyata: `registry.Load()` membaca YAML ter-embed yang sama yang di-boot server, bukan fixture.
Setelah pass ini: `go build ./...` bersih, `go vet ./...` bersih, `go test ./internal/provider/
./internal/registry/` lulus, dan `git status` tidak memuat satu pun berkas probe.

Bentuk permanen probe ada di §7.2; ringkasan keluaran terukur (semua dijalankan lewat
`go test -run 'TestProbe' -v`):

```
PROBE1 provider="opencode-go" models=10 features={Usage:false UsageAPIKey:false}
PROBE1 missing_vs_reference=23 ... (23 id reference tak ada, 5 id app tak ada di reference; daftar §4.2 F2)
PROBE2 opencode-zen present=false
PROBE2-resolve opencode-go/minimax-m2.7 target=claude        <- target benar, URL salah (F2)
PROBE3 url=https://opencode.ai/zen/go/v1/chat/completions    <- model claude-target dikirim ke chat URL
PROBE3 authorization="Bearer sk-go-test" x-api-key="" anthropic-version=""
PROBE12 url=https://opencode.ai/zen/v1/messages
PROBE12 anthropic-version="" ... Authorization="Bearer public"  (F4)
PROBE4 klien glob saja: tools=[glob, bash, read]             (F3: grep tidak pernah disuntik)
PROBE9 klien tanpa tools: tools=[bash, read], tanpa tool_choice  (F3 + F7: kuartet tak lengkap, decoy bisa dipilih)
PROBE8: klien "Bash" + decoy "bash" duplikat                   (F3)
PROBE5 muse-spark-1.2/1.3 + union-alpha vision=false           (F5)
PROBE14 model non-quirk dengan tool_choice eksplisit -> auto   (F7: reference membiarkannya)
PROBE13 opencode-go/deepseek-v4-pro -> ERR MODEL_NOT_FOUND     (F2)
PROBE13 opencode/jev-1.13-free -> target=openai                (F6: kind systemone tidak dikenal)
```

### 3.3 Set in-flight yang dikecualikan

Sama dengan draft 027 §2.3: working tree membawa pekerjaan pricing dan quota yang belum selesai
(`pricing*.go`, `quota_counter*.go`, `M` pada `chat_record.go` dan repository). Tidak ada temuan
F1-F7 yang berdiri di atas berkas in-flight itu; `pricing.json` dibaca apa adanya sebagai data
ter-commit dan bagian pricing keluarga OpenCode justru non-finding (§5).

Selama pass ini berjalan, set in-flight bertambah (terukur dari `git status --short`: 44 baris,
27 `M` dan 17 `??`, termasuk `internal/dataplane/selection.go` dan `selection_candidates.go` yang
belum ter-commit). Satu akibat yang diukur dan dicatat jujur: `go test ./internal/dataplane/`
**gagal hari ini** pada `TestRelay_KeysRotateWithinAnEndpoint` (`key 2 was never selected in 3
requests; keys do not rotate`), dan kegagalan itu **bukan** dari pass ini. Dibuktikan dengan
`git stash push -- internal/dataplane/selection.go internal/dataplane/selection_candidates.go`,
test yang sama lulus, lalu perubahan itu dikembalikan dengan `git stash pop`. Perbaikan F1-F7 tidak
menyentuh berkas selection; catatan ini ada supaya gate `go test ./internal/dataplane/` yang merah
tidak dikira berasal dari dokumen ini.

## 4. Findings

### 4.1 F1 (HIGH): `opencode-zen` tidak ada, dan tidak bisa "sekadar diregenerasi"

**Fakta.** Reference mendeklarasikan `opencode-zen` sebagai provider PAYG ber-kunci API
(`open-sse/providers/registry/opencode-zen.js`, 135 baris, commit `49185137` 2026-09-19): 74 model
(sekitar 64 model berbayar plus 10 model ber-akhiran `-free` di jalur berkunci), `transports[]`
tiga-endpoint, fitur usage
`features: {usage: true, usageApikey: true}`, dan pembaca kuota di
`open-sse/services/usage/opencode-zen.js` yang membaca `https://opencode.ai/zen/v1/usage`.

Di `app-serv`, probe menjawab:

```
PROBE2 opencode-zen present=false
```

`grep -rn "opencode-zen" app-serv` mengembalikan nol baris (di luar dokumen ini).

**Kenapa ini bukan sekadar regenerasi registry.** Dua hal di entri itu tidak punya tempat di
schema `app-serv` hari ini: (a) `transports[]` multi-endpoint, yang dijatuhkan generator karena
`Transport` struct tidak punya field-nya (`types_transport.go:33-60`; `mapTransport` di
`registry-gen.mjs:270-304` hanya membawa kunci yang bisa dibaca YAML target); (b) `features.usage`
dan `usageApikey`, yang memang punya struct (`types.go:88-91`) tetapi **nol consumer**: `grep
"Features.Usage\|\.Features\b"` atas `internal/ cmd/` tanpa test mengembalikan nol, dan `UsageConfig`
sama persis: dideklarasikan `types_transport.go:50,112`, dibaca 12 blok `usage:` di YAML
(14 entri ber-flag `features.usage`), tetapi tidak dibaca satu baris kode pun. Meregenerasi registry
tanpa keputusan dua field itu akan menghasilkan entri yang data usage-nya tersimpan lalu diabaikan,
yang bentuk janji-tanpa-pelaksana yang sudah pernah ditutup draft 005 F1 untuk kuota.

**Risiko.** Operator tidak dapat menyatakan kunci Zen PAYG sama sekali, padahal itu satu-satunya
jalur berbayar resmi opencode.ai; 74 modelnya juga tidak muncul di katalog. Sebaliknya, menambahkannya
setengah-setengah (entri tanpa usage surface) menciptakan jenis utang yang sama dengan yang audit
anti-slop 001 temukan pada kuota: tampilan yang menjanjikan, pelaksana yang tidak ada.

**Rencana DURING.** Dua langkah berurutan, keduanya perlu keputusan owner:

1. Keputusan schema: apakah `transports[]` dan `features.usage` masuk kontrak `app-serv`
   (SPEC-API-001 §7.4/§7.12) ataukah `opencode-zen` dipetakan ke tiga entri tunggal-endpoint
   (`opencode-zen` format openai, `opencode-zen-claude` format claude, dst.) dengan alasan tertulis.
   Mapping-entri-tunggal lebih murah tetapi memecah satu kredensial menjadi tiga, yang bertabrakan
   dengan model `upstream_endpoints` (satu akun, satu baris).
2. Setelah itu: entri registry + pemakaian `UsageConfig` di jalur kuota `reported` (method `Report`
   sudah ada di domain, `quota.go:206`, dengan nol pemanggil produksi hari ini), atau catat usage
   sebagai deferred dengan alasan.

**Kriteria selesai.** `registry.Load()` menjawab `opencode-zen` dengan jumlah model yang bisa
dibedakan dari YAML; bila usage ikut, satu test bernama membuktikan angka `reported` dari
`/zen/v1/usage` menulis `quota_windows` lewat `Report`; bila tidak, spec menyebut deferred-nya.

### 4.2 F2 (HIGH): `opencode-go` tertinggal di pin lama, dan wire claude-target-nya salah

**Fakta, tiga bagian:**

1. **Daftar model.** Entri `registry.yaml:3125-3166` mendeklarasikan 10 model. Reference pada
   `21583c03` mendeklarasikan 28. Terukur dengan probe yang membandingkan kedua daftar:

   ```
   MISSING IN APP (23): deepseek-flash, glm-5.3-flash, glm-5.3, glm-5.2, kimi-k2.7-code,
     kimi-k3, deepseek-v4-pro, deepseek-v4-flash, deepseek-v4-flash-vision-exp, longcat-2.0,
     mimo-v2.5, mimo-v2.5-pro, minimax-m3, qwen3.8-max, qwen3.8-flash, qwen3.7-max,
     qwen3.7-plus, hy4-preview, hy3, grok-4.6, gpt-5.6-luna,
     muse-spark-1.2-contributor, muse-spark-1.3-contributor
   STALE IN APP (5): kimi-k2.5, glm-5, qwen3.5-plus, mimo-v2-pro, mimo-v2-omni
   ```

   Entri YAML ini identik dengan `git show db4499d6:open-sse/providers/registry/opencode-go.js`
   (10 model, urutan sama), jadi entri ini tidak pernah disentuh sejak pin Juni. Resolver menjawab
   `MODEL_NOT_FOUND` untuk semua id baru (PROBE13) dan `passthrough_models` tidak dideklarasikan
   entri ini, jadi tidak ada jalur lain: 23 model reference tidak dapat diminta sama sekali.

2. **Tabel `transports[]` dan `supportedFormats`.** Reference `opencode-go.js:31-35` mendeklarasikan
   tiga endpoint (`/chat/completions` openai, `/messages` claude dengan `x-api-key` +
   `anthropicVersion`, `/responses`) dan setiap model mendeklarasikan `supportedFormats` yang
   menjawab endpoint mana yang boleh (`chatCore.js:89-101` membaca keduanya). `app-serv` tidak punya
   field `transports` maupun `supported_formats` di `Transport`/`Model` struct, dan generator
   menjatuhkannya. Akibat terukurnya di `app-serv` hari ini:

   ```
   PROBE2-resolve opencode-go/minimax-m2.7 target=claude
   PROBE3 url=https://opencode.ai/zen/go/v1/chat/completions
   PROBE3 authorization="Bearer sk-go-test" x-api-key="" anthropic-version=""
   ```

   Dua model minimax dideklarasikan `target_format: claude` (`registry.yaml:3161-3166`), resolver
   memilih target claude dengan benar, tetapi URL tetap `base_url` chat-completions dan kredensial
   dikirim sebagai `Authorization: Bearer`. Sebuah body Anthropic di-POST ke endpoint chat
   completions. Di pin lama ini bukan bug (executor pin lama `db4499d6` memang mengirim minimax ke
   `/messages` dengan `x-api-key` + `anthropic-version`, `git show
   db4499d6:open-sse/executors/opencode-go.js:17-41`); port-nya yang tidak membawa aturan per-model itu.

3. **Model responses-only.** Empat model reference (`grok-4.6`, `gpt-5.6-luna`,
   `muse-spark-{1.2,1.3}-contributor`) hanya dilayani `/zen/go/v1/responses` dan dideklarasikan
   `targetFormat: "openai-responses"`. Tidak ada di YAML, dan tanpa field untuk menyatakannya.

**Deviasi.** Ini bukan klaim spec yang bohong (spec tidak menjanjikan model opencode-go), melainkan
port yang berhenti di tengah: setengah entri dibawa, setengah aturan per-modelnya tidak.

**Rencana DURING.** Bergantung keputusan F1 nomor 1 (schema `transports`): kalau field-nya ditambah,
entri `opencode-go` diregenerasi utuh dari `21583c03` (jalur A draft 011 §5) dan connector
`OpenCode` diperluas membaca leaf dari transport terpilih; kalau tidak, aturan per-model
dimodelkan sebagai `target_format` + satu `base_url` per format seperti yang `opencode` Free sudah
lakukan (model dideklarasikan `target_format`, `Endpoint()` memilih leaf). Jalur kedua tidak
menuntut schema baru: `Endpoint()` di `opencode.go:104-128` sudah tahu menyusun
`{base}/zen/v1/{leaf}` dan bisa diberi prefiks `/zen/go/v1` untuk entri go, plus `ApplyAuth`
membaca `auth.api_key.header` yang entri minimax bisa deklarasikan (`x-api-key`) dan menambah
`anthropic-version` saat leaf `/messages` (menutup F4 sekaligus di jalur yang sama).

**Kriteria selesai.** Probe F2 ini dipasang permanen: resolusi `opencode-go/minimax-m2.7` menjawab
URL `/zen/go/v1/messages` dengan `x-api-key`; `opencode-go/grok-4.6` menjawab
`/zen/go/v1/responses`; jumlah model = jumlah model reference pada revisi pin.

### 4.3 F3 (MEDIUM): fingerprint free tier belum dedup, dan kuartetnya tidak dituntut

**Fakta.** Reference mengunci gerbang free tier lewat kuartet empat alat
(`OPENCODE_FINGERPRINT_TOOLS = ["bash", "glob", "grep", "read"]`, `opencodeFingerprint.js:12`,
commit `822aa958` 2026-09-19, tes `tests/unit/opencode-fingerprint.test.js` 191 baris). Alurnya:
(a) kanonisasi varian kapitalisasi (`Bash` menjadi `bash`) tanpa duplikat; (b) append hanya anggota
yang hilang; (c) `tool_choice` eksplisit diarahkan ulang ke nama kanonik; (d) di sisi respons, ejaan
asli klien dipulihkan lewat WeakMap (`restoreToolNames`).

Port menyuntik dua alat saja (`bash`, `read`, `opencode_decoys.go:44`), tanpa kanonisasi dan tanpa
restorasi. Terukur:

```
PROBE4 (klien glob saja):  tools = [glob, bash, read]        <- glob/grep tidak pernah disuntik
PROBE8 (klien "Bash"):     tools = [Bash, bash, read]
```

**Koreksi setelah pengukuran live (§4.8), tingkat turun HIGH ke MEDIUM.** Gate upstream hari ini
**tidak menuntut kuartet** dan **menerima varian kapitalisasi**: `[Bash, bash, read]` menjawab 200,
dan body dengan hanya `glob`+`grep` ditolak 403. Jadi dua dari tiga premis F3 versi pertama gugur.
Yang **tetap cacat terukur** adalah **duplikat nama persis**: `[bash, bash, read]` dan
`[bash, read, read]` sama-sama 400 `Provider returned error`. Connector tidak punya dedup, jadi
klien yang mendeklarasikan `bash` dua kali meneruskan duplikat itu apa adanya. Deteksi port membaca
kedua bentuk deklarasi (`name` dan `function.name`, `opencode_decoys.go:53-62`), sehingga jalur
"connector menyuntik `bash` kedua" tertutup untuk bentuk biasa; yang tidak tertutup adalah duplikat
yang klien sendiri bawa.

`opencodeFingerprint.js:63-64` menyebut alasan dedup dengan tepat: "`Bash` + `bash` is rejected
upstream as a duplicate." Komentar reference itu menua: pengukuran menunjukkan varian kapitalisasi
diterima, dan yang ditolak adalah nama yang sama persis. Komentar port (`opencode_body.go:8-11`)
menyebut "bash and read decoy tools", yang **cocok dengan syarat yang terukur hari ini**.

**Risiko.** Satu kelas kegagalan live: klien yang mendeklarasikan `bash` (atau `read`) dua kali
menerima 400, dan pesannya `Provider returned error` tanpa menyebut alat mana, sehingga sulit
dilacak dari sisi gateway. Restorasi ejaan (butir d) tetap sebuah delta terhadap reference, tetapi
ia kosmetik untuk klien (`Bash` vs `bash`), bukan penolakan.

**Rencana DURING.** Tambahkan dedup nama persis di `ensureOpenCodeDecoys` (dan pertahankan deteksi
dua bentuk yang sudah ada), lalu putuskan butir (d): restorasi ejaan klien di respons dibawa keluar
connector (field di `Request` plugin, yang sudah ada dan kosong), atau diterima bahwa klien melihat
nama kanonik. Kuartet dan kanonisasi **tidak** lagi diusulkan sebagai keharusan, karena pengukuran
menunjukkan keduanya bukan syarat; keduanya tetap boleh dikerjakan sebagai kesetiaan reference bila
owner mau, dengan biaya yang dinyatakan.

**Kriteria selesai.** Test tabel: klien `[bash, bash, read]` menghasilkan tepat satu `bash`; klien
`[bash, read]` tidak berubah; klien `[]` mendapat decoy `bash`+`read`; keputusan butir (d) tercatat
(dibawa keluar, atau known-delta dengan alasan).

### 4.4 F4 (LOW): `union-alpha` dikirim ke `/messages` tanpa `anthropic-version`

**Fakta.** Connector membangun URL `/zen/v1/messages` untuk model claude-target dengan benar
(`opencode.go:118-122`, PROBE12), dan `ApplyAuth` menulis lima header identitas, tetapi tidak
`anthropic-version`:

```
PROBE12 url=https://opencode.ai/zen/v1/messages
PROBE12 anthropic-version=""
```

Reference menambahkan header itu persis pada URL yang sama: `executors/opencode.js:484`
`if (url.endsWith("/messages")) headers["anthropic-version"] = ANTHROPIC_API_VERSION;`
(commit `2b65c49f`, "Route union-alpha through Messages API ... add anthropic-version header").

**Koreksi setelah pengukuran live (§4.8), prioritas turun.** Premis "kemungkinan besar 400 live"
**tidak terbukti**: diukur 2026-09-24, `/zen/v1/messages` menjawab **401 `ModelError: Model
union-alpha is not supported`**, dengan maupun tanpa header `anthropic-version`. Model itu sendiri
tidak dilayani di jalur tersebut hari ini, jadi status header tidak mengubah jawaban. F4 tetap
berlaku sebagai **presisi porting** (reference menulis header itu persis di URL itu, dan wire
Anthropic memang menuntut versi API), tetapi ia bukan cacat yang terukur merusak permintaan saat ini,
dan tidak ada bukti live bahwa memperbaikinya memperbaiki apa pun. Dua model minimax di
`opencode-go` yang juga `target_format: claude` tidak diuji live pass ini (jalur ber-kunci), jadi
status mereka tidak diklaim.

**Rencana DURING.** Satu baris di `ApplyAuth` (atau di `Endpoint`+`ApplyAuth` bersama): bila leaf
adalah `/messages`, set `anthropic-version: 2023-06-01`. Nilai itu sudah ada di YAML milik entri
anthropic (`registry.yaml:579` `Anthropic-Version: "2023-06-01"`) dan di reference
`providers/shared.js:24`, jadi tidak ada nilai baru yang perlu ditemukan.

**Kriteria selesai.** Probe F4 dipasang permanen di `opencode_auth_test.go`: request yang menuju leaf
`/messages` membawa `anthropic-version`. Klaim "ini memperbaiki 400 live" **tidak** dipakai lagi;
yang diklaim hanya kesetiaan ke reference.

### 4.5 F5 (MEDIUM): capability vision keluarga OpenCode salah semua

**Fakta.** Reference `capabilities.js:144-147` mendeklarasikan
`muse-spark-1.2-contributor-free`/`muse-spark-1.3-contributor-free` (`vision: true, reasoning: true`)
dan `union-alpha` (`vision: true`), plus pola `*muse*spark*` (:408). Port menjawab:

```
PROBE5 muse-spark-1.2-contributor-free vision=false
PROBE5 muse-spark-1.3-contributor-free vision=false
PROBE5 union-alpha vision=false
```

Sebabnya terukur: `testdata/capabilities_corpus.json` (entri `:2341-2357`) mengharapkan `false`
untuk ketiganya dan digenerate dari pin `db4499d6` (field `revision`), revisi ketika entri
capabilities itu **belum ada** di reference (commit `acb5c34c` yang menambahkannya tertanggal
2026-09-03). Test korpus lulus karena korpus dan tabel sepakat sama-sama ketinggalan. Ini persis
kelas cacat "port dan korpus sama-sama basi" yang draft 017 §9.2 klaim tertutup ("0 dari 507 baris
salah") untuk revisi pin lama; klaim itu tetap benar untuk pin-nya, tetapi pin-nya yang basi.

**Akibat.** `?capability=vision` tidak pernah menawarkan ketiga model itu (draft 017 F4 adalah
perbaikan pembaca tabelnya; datanya yang salah sekarang), dan vision adapter `§7.8` tidak akan
menilai request ber-gambar ke model itu.

**Rencana DURING.** Regenerate `capabilities_corpus.json` dari `21583c03` (jalur A draft 011, atau
jalur tambalan yang sama-sama sah dengan catatan), yang otomatis mengharapkan `true` ketiganya, lalu
tambahkan ketiganya ke `visionExactIDs` (`capability.go:42-47`) atau pola `*muse*spark*` +
`union-alpha` ke `visionRules`. Keputusan kecil: pola lebih setia ke reference (yang memakai pola),
exact-id lebih sempit efeknya.

**Kriteria selesai.** Probe F5 dibalik: ketiga id menjawab `vision=true` dari `registry.Load()`
tanpa fixture; korpus menuliskan revisi barunya.

### 4.6 F6 (MEDIUM): System One (Jev) tidak diputuskan

**Fakta.** Reference memasang konsep keputusan Jev di empat tempat: model `kind: "systemone"`
(`registry/opencode.js:31`), `serviceKinds` pada tiga entri, route `POST /api/v1/systemone`
(`src/app/api/v1/systemone/route.js`), dan handler dengan loop kredensial + fallback
(`src/sse/handlers/systemone.js`, 152 baris, pola yang sama dengan embeddings). Body JSON
(`state`, `questions`) di-forward utuh; `systemoneCore.js` menambah header session dan `model`.

`app-serv` tidak punya satu pun tempat itu: `grep -rn "systemone\|jev" app-serv` mengembalikan nol.
Draft 027 F6 sudah mencatat route-nya sebagai salah satu dari enam surface gap, tetapi tanpa
pengukuran keluarga OpenCode; pengukuran ini menambahkan tiga fakta: (a) `kind: "systemone"` akan
ditolak `Model.IsChat()` (`types.go:133-140` mengembalikan false untuk kind tak dikenal) sehingga
model jev di registry tanpa keputusan kind akan hilang dari katalog chat dengan diam; (b) probe
menunjukkan `opencode/jev-1.13-free` hari ini dijawab lewat passthrough (`target=openai`) karena id
tidak dideklarasikan, yang berarti request ke model keputusan ini akan dikirim sebagai chat body ke
`/chat/completions`; (c) `openrouter` di YAML juga kehilangan `typesafe/jev-1.13` dari daftar
modelnya (14 dari 18 model reference), bagian dari gap yang sama.

**Rencana DURING.** Ini keputusan owner, bukan pekerjaan langsung: port route + kind (dengan
`service_kinds` dan `IsChat` memperlakukan systemone sebagai non-chat sehingga hanya route systemone
yang menyajikannya), atau catat NOT PORTED dengan alasan di §2.2 SPEC-API. Yang tidak boleh
berlanjut adalah status sekarang: id jev dijawab sebagai model chat passthrough yang salah wire.

**Kriteria selesai.** Salah satu: `POST /api/v1/systemone` dengan test loop kredensialnya, atau
satu paragraf NOT PORTED yang menyebut jev di SPEC-API §7.15 dan `jev-1.13-free` dihapus dari
jawaban passthrough opencode (deklarasi model eksplisit tanpa kind akan menolak dengan jelas).

### 4.7 F7 (LOW): perbedaan presisi transform pada wire Responses

Enam selisih terukur, semuanya pada `opencode_body.go` versus `executors/opencode.js:412-449`:

1. **`tool_choice` dipaksa `auto` untuk semua model Responses** (`opencode_body.go:97`), termasuk
   model yang reference tidak menurunkan pilihannya. Reference menurunkan pilihan ke `auto` hanya
   pada model yang dideklarasikan quirk `forceAutoToolChoiceModels: ["muse-spark-1.3-contributor-free"]`
   (`registry/opencode.js:22-24`, `executors/opencode.js:418-422`), jadi `muse-spark-1.2-contributor-free`
   dengan `tool_choice` eksplisit tetap dikirim apa adanya di reference (PROBE14: port mengubahnya
   menjadi `auto`). Dua koreksi presisi yang ditemukan saat memverifikasi klaim ini:
   (a) kasus `tool_choice` **absent** bukan delta: reference menulis `auto` juga, lewat
   `applyFingerprintTools(body, true)` (`opencodeFingerprint.js:145-147`, dipanggil di
   `executors/opencode.js:444`), sehingga port 1:1 di kasus itu;
   (b) test reference `tests/unit/opencode-free-tool-choice.test.js:52-57` yang mengharapkan absent
   tetap absent **basi terhadap kode reference sendiri**: test itu terakhir disentuh `aa14ef72`
   (2026-09-17), sedangkan `applyFingerprintTools` yang menulis `auto` mendarat belakangan lewat
   `822aa958` (2026-09-19) dan tidak memperbarui ekspektasinya. Dokumen ini tidak memakai test itu
   sebagai bukti; yang dipakai adalah alur kode `transformRequest` sendiri. Catatan jujur: suite
   vitest reference **tidak dijalankan** pass ini (checkout tidak memasang `node_modules`), jadi
   pernyataan "test itu basi" adalah bacaan kode dua berkas, bukan hasil eksekusi.
2. **`store: false` tidak pernah ditulis.** Reference menulis `body.store = false` pada wire
   Responses (`executors/opencode.js:438`); port tidak. `schema.ResponsesRequest` punya fieldnya
   (`responses.go:60`) tetapi jalur same-format meneruskan `Raw` apa adanya sehingga klien yang
   tidak mengirim `store` akan tiba tanpa `store`.
3. **`reasoning_effort` tidak dinormalisasi ke `reasoning:{effort,summary:"auto"}`** (reference
   `normalizeOpencodeReasoning`, `executors/opencode.js:373-394`). PROBE5: body dengan
   `reasoning_effort:"high"` keluar dengan `reasoning_effort` utuh, tanpa `reasoning`, tanpa `store`.
4. **Input tidak dinormalisasi.** Reference mengubah string kosong/array kosong menjadi satu pesan
   placeholder (`normalizeResponsesInput`, `responsesApi.js:11-24`, alasan #389: "Empty input[]
   would produce messages:[] which all providers reject"). PROBE6: `"input":""`, `"input":[]`,
   dan `"input":"hello"` (string telanjang) keluar tidak tersentuh.
5. **`call_id` panjang dan `arguments` objek tidak dikoreksi pada jalur same-format.**
   `clampResponsesCallId`/`coerceResponsesArguments` ada di port (`translate_responses_items.go:204-210`)
   tetapi hanya berjalan di jalur penerjemahan; jalur same-format (`engine_translate.go:40-42`,
   `replaceModel`) meneruskan byte klien apa adanya, jadi `call_id` 85 karakter dan
   `arguments` objek tiba di upstream apa adanya (PROBE11), padahal reference meng-clamp 64 dan
   men-stringify (alasan #393 InputValidationError). Alat chat-shape (`{function:{name}}`) juga
   tidak di-flatten ke bentuk flat Responses oleh connector (PROBE10), yang reference lakukan di
   `normalizeResponsesTools`.
6. **Chat wire tanpa `tools` klien tidak menulis `tool_choice: "none"`.** Reference menulisnya
   (`opencodeFingerprint.js:146-147`: chat request tanpa tools klien memakai `none` "so the injected
   decoys cannot be selected"), sehingga decoy `bash`/`read` yang disuntik tidak pernah dipilih model.
   Port menyuntik decoy tanpa menulis `tool_choice` sama sekali (PROBE9 kasus 2), jadi model bebas
   memanggil `bash`/`read` yang deskripsinya sendiri katakan tidak tersedia.

**Kenapa LOW dan satu temuan.** Semuanya gagal-mode klien-tertentu (klien yang mengirim bentuk
tepat tidak terpengaruh), dan keenam byte yang salah datang dari jalur yang berbeda-beda tetapi
keputusannya satu: seberapa jauh connector boleh menulis ulang body klien same-format. Itu keputusan
arsitektural (jalur same-format sengaja lossless, alasan tertulis di `engine_translate.go:32-34`),
jadi memperbaikinya menuntut pengecualian yang dinyatakan untuk opencode, bukan tambalan per-field.

**Rencana DURING.** Satu keputusan: (A) connector opencode mendapat lisensi menulis ulang body
same-format untuk koreksi bentuk ini (kuartet kiri F3 sudah melakukannya untuk `tools`, jadi
presedennya ada), atau (B) daftar ini dicatat sebagai known-delta dengan alasan lossless. Kalau A:
quirk tool-choice dibaca dari registry (bukan hardcode), `store:false` + reasoning + input
normalization + clamp call_id masuk `TransformRequest` dengan test tabel per kasus.

**Kriteria selesai.** Bila A: probe F7 menjadi test tabel yang menjepit keenam kasus plus batas
quirk (1.2-free tidak diubah). Bila B: §7.15 menuliskan delta-nya.

### 4.8 Pass uji free tier (permintaan owner, DURING selesai)

Owner meminta: **hanya free tier**, **tanpa auth, tanpa api key**, dan tiga model
(`space-bunny-free`, `mimo-v2.6-flash-free`, `muse-spark-1.3-contributor-free`) harus **lolos test**.
Pass ini mendaratkannya, dan sekaligus mengoreksi dua asumsi draft ini yang ternyata salah saat
diukur terhadap upstream sungguhan. Keduanya dicatat sebagai koreksi, bukan dihapus.

**Yang mendarat.** Empat berkas test, 626 baris, semuanya di `internal/dataplane/`:

| Berkas | Baris | Isi |
|---|---|---|
| `engine_opencode_free_test.go` | 197 | Stand-in upstream yang menegakkan gerbang hasil ukur, tipe `openCodeFreeCall`, helper nama alat |
| `engine_opencode_free_engine_test.go` | 99 | Wiring pipeline nyata (resolver, selector, transport, connector) atas stand-in |
| `engine_opencode_free_streams_test.go` | 41 | Body SSE per wire (chat dan Responses) |
| `engine_opencode_free_acceptance_test.go` | 212 | Empat test penerimaan |
| `engine_opencode_free_live_test.go` | 77 | Test live opt-in (`-tags=integration`) ke `opencode.ai` |

Empat test penerimaan, semuanya hijau:

```bash
go test ./internal/dataplane/ -run 'TestOpenCodeFree' -count=1 -v
# --- PASS: TestOpenCodeFree_ThreeModelsAnswerWithoutACredential
# --- PASS: TestOpenCodeFree_NoCredentialMaterialLeavesTheGateway
# --- PASS: TestOpenCodeFree_DeclaredResponsesModelTakesTheResponsesWire
# --- PASS: TestOpenCodeFree_UnknownFreeIdStillPassesThrough

go test -tags=integration ./internal/dataplane/ -run 'TestOpenCodeFreeLive' -count=1 -timeout 300s
# --- PASS: TestOpenCodeFreeLive_ThreeModelsAnswerFromTheRealUpstream (9.17s)
```

Test live itu menjalankan **pipeline produksi yang sama** (resolver, selector, transport, connector
`OpenCode` asli) ke `https://opencode.ai`, dengan endpoint **tanpa kunci** (`UpstreamAuthNone`), dan
ketiga model menjawab stream penuh. Ini bukti R-35 yang diminta: bukan inferensi dari unit test,
melainkan pengukuran ke upstream.

**Mutation check (test tidak buta).** Tiga mutasi pada kode produksi, masing-masing tertangkap:

| Mutasi | Hasil |
|---|---|
| `ensureOpenCodeDecoys` dijadikan no-op | FAIL: `missing required tools: bash,read` |
| `x-opencode-session` tidak ditulis | FAIL: `no canonical session was presented` |
| `User-Agent` tidak ditulis | FAIL: `UPSTREAM_ERROR ... Forbidden` |

**Koreksi 1: gerbang free tier bukan kuartet, dan bukan kredensial.** Diukur langsung ke upstream
2026-09-24, satu variabel per percobaan:

| Percobaan | `space-bunny-free` | `mimo-v2.6-flash-free` | `muse-spark-1.3` (`/responses`) |
|---|---|---|---|
| Lengkap (session + UA + `bash`,`read` + stream) | 200 | 200 | 200 |
| Tanpa kredensial (`Authorization` dihapus) | 200 | 200 | 200 |
| Tanpa `x-opencode-session` | 200 | 403 | 403 |
| Tanpa `User-Agent: opencode/...` | 200 | 403 | 403 |
| Hanya session, tanpa UA | 403 | 403 | 403 |
| Tanpa `tools` sama sekali | 200 | 403 | 403 |
| Hanya `bash` (tanpa `read`) | n/a | 403 | 403 |
| Hanya `glob`+`grep` (tanpa `bash`,`read`) | n/a | 403 | 403 |
| `stream:false` | n/a | 403 | n/a |
| `tools` bentuk chat (nested `function`) di `/responses` | n/a | n/a | 400 `tools[0] missing required field name` |

Tiga kesimpulan yang mengoreksi draft ini:

1. **Kuartet bukan syarat.** Body dengan hanya `glob`+`grep` ditolak, dan body dengan hanya
   `bash`+`read` diterima. F3 draft ini menyebut "upstream menolak duplikat kuartet" berdasarkan
   komentar reference; pengukuran menunjukkan syarat hari ini lebih longgar. Port yang menyuntik
   `bash`+`read` karena itu **sudah cukup**, dan komentarnya (`opencode_body.go:8-11`) jujur.
2. **Kredensial bukan syarat.** Tanpa `Authorization` sama sekali tetap 200. Yang dijaga upstream
   adalah **bentuk identitas klien** (session + UA bersamaan) dan **bentuk body** (stream + alat),
   bukan kepemilikan kunci. Ini yang membuat jalur "tanpa auth, tanpa api key" bekerja, dan yang
   membuat endpoint `UpstreamAuthNone` sah untuk lane ini.
3. **`space-bunny-free` tidak bergerbang sama sekali.** Ia menjawab tanpa tools, tanpa session,
   tanpa UA. Karena itu test memisahkannya sebagai `openCodeFreeUngated` alih-alih memaksakan satu
gerbang untuk seluruh lane.

**Koreksi 2: duplikat yang ditolak adalah nama **persis**, bukan varian kapitalisasi.**

| Body | Hasil |
|---|---|
| `[Bash, bash, read]` (varian kapitalisasi) | 200 |
| `[bash, bash, read]` (duplikat persis) | 400 `Provider returned error` |
| `[bash, read, read]` (duplikat persis) | 400 |
| `[bash, read]` | 200 |

Jadi klaim F3 bahwa klien `Bash` menerima penolakan **tidak terbukti**: `Bash`+`bash` diterima.
Yang tetap cacat, dan tetap mungkin terjadi lewat dua jalur berbeda, adalah **duplikat persis**:
(1) klien sudah mendeklarasikan `bash` dan connector menyuntik `bash` kedua bila deteksinya meleset
pada bentuk deklarasi yang berbeda, dan (2) klien mendeklarasikan `bash` dua kali sendiri. Deteksi
port membaca kedua bentuk (`name` dan `function.name`) sehingga jalur (1) tertutup untuk kasus biasa,
tetapi tanpa dedup jalur (2) tetap diteruskan. F3 karena itu **turun dari HIGH ke MEDIUM**, dan
kriteria selesainya berubah: bukan "kuartet lengkap", melainkan **dedup nama persis**.

**Non-finding baru: `union-alpha` tidak dilayani `/zen/v1/messages`.** Diukur: 401 `ModelError: Model
union-alpha is not supported`, dengan atau tanpa header `anthropic-version`. Artinya F4 (header
`anthropic-version` hilang) **tidak dapat diverifikasi live** pada model itu hari ini: modelnya sendiri
tidak dilayani di jalur itu, jadi status header itu tidak mengubah jawaban. F4 tetap berlaku sebagai
presisi porting (reference menulis header itu persis di URL itu, `opencode.js:484`), tetapi
prioritasnya turun dan tidak ada bukti live bahwa ia memperbaiki apa pun saat ini. Ini juga
melemahkan premis F4 di §4.4 yang menyebut "kemungkinan besar 400 live": yang terukur adalah 401
`ModelError`, bukan 400 wire. Dua model minimax di `opencode-go` yang juga `target_format: claude`
tidak diuji live pass ini (jalur ber-kunci), jadi tidak diklaim.

**Satu gap yang tetap terbuka (F8, MEDIUM).** Test ini membuktikan lane free tier bekerja **lewat pipeline**,
tetapi belum membuktikan operator dapat memicunya lewat HTTP tanpa membuat endpoint lebih dulu:
reference menyuntik koneksi virtual untuk provider `noAuth` (`src/sse/services/auth.js:45-63`),
sedangkan `app-serv` menuntut baris `upstream_endpoints` (nol endpoint = `NO_PROVIDER_AVAILABLE`,
`selection_candidates.go:52`). Test memakai endpoint `UpstreamAuthNone` yang eksplisit, jadi yang
diuji adalah "endpoint tanpa kunci", bukan "tanpa endpoint sama sekali". Ini keputusan yang belum
diambil, bukan klaim yang sudah beres; bila owner ingin panel memakai free tier tanpa membuat
endpoint, itu pekerjaan tersendiri (seam koneksi virtual di `Selector`, dengan test yang menegaskan
nol-endpoint + `no_auth` tetap terpilih).

## 5. Non-findings dan catatan

Dicatat supaya tidak diperiksa ulang sebagai drift.

| Hal | Kenapa bukan temuan |
|---|---|
| Session id stateless (derive dari endpoint id) vs reference (map TTL + forwarding header klien) | Keputusan desain yang ditulis alasannya dua kali di kode (`opencode_session.go:37-44`: stateless, tanpa lock, kuota per-endpoint). Sifat yang dijaga reference (stabil per identitas) terjaga oleh derive; forwarding `x-opencode-session` klien native tidak mungkin tanpa meneruskan header klien ke connector, yang data plane `app-serv` memang tidak lakukan untuk provider mana pun (satu kebijakan, bukan celah opencode) |
| `x-opencode-request` per-turn tidak di-port | Bagian dari keputusan session di atas; tanpa header klien yang diteruskan, id per-turn tidak punya sumbernya. Dicatat di sini, bukan di F7, karena tidak menyentuh byte yang dikirim hari ini |
| Pricing keluarga OpenCode kosong di kedua sisi | `pricing.js` reference tidak memuat muse/union/jev (free tier, `getPricingForModel` mengembalikan `null`), dan `pricing.json` port (pin `21583c03`, 108 model) juga tidak. Biaya 0 adalah jawaban yang benar untuk jalur free; Zen PAYG berharga tetapi entri-nya adalah F1 |
| `mimo-v2.6-flash-free` di Zen (commit `41a1b800`, sebelum tag) | Bagian dari F1 (entri Zen tidak ada); bukan temuan terpisah |
| Satu baris `thinkingLevels.js` berubah antara v0.5.85 dan v0.5.86 (pola `*mimo*v2.6*`) | Di luar keluarga opencode (mimo), dan konsep thinking-levels belum di-port sama sekali (draft 027 F4 metadata); tidak ada klaim yang bertabrakan |
| Entri `opencode` Free dinyatakan `has_free: true` tanpa kolom `free` di model | Konsisten dengan reference (`hasFree: true`); tidak ada delta |
| Klaim changelog SPEC 2026-09-20 tentang discovery fix OpenCode Free | Sudah benar dan diukur draft 027; daftar model `opencode` hari ini benar (3 chat model dengan wire masing-masing, PROBE/`resolve_opencode_test.go` mengunci) |
| `space-bunny-free` dan `mimo-v2.6-flash-free` tidak ada di registry statis reference | Bukan drift: reference melayaninya lewat `passthroughModels: true` (daftar statisnya hanya 4 id, `registry/opencode.js:26-32`) plus `modelsFetcher` yang membaca `/zen/v1/models` live. Port memakai passthrough yang sama, dan `TestOpenCodeFree_UnknownFreeIdStillPassesThrough` menguncinya |
| `space-bunny-free` tidak bergerbang sama sekali (tanpa tools/session/UA tetap 200) | Sifat upstream, bukan delta port. Test memperlakukannya sebagai `openCodeFreeUngated` alih-alih memaksakan satu gerbang untuk seluruh lane |

### 5.1 Yang sudah 1:1 dan tidak diusulkan berubah

Jejak perbaikan free tier yang port ikuti satu per satu, dengan commit kedua sisi:

| Perilaku | Reference | Port |
|---|---|---|
| Session kanonik `ses_` bentuk regex, UA CLI >= 1.17 | `6091ff59` | `opencode_session.go:32` (regex + base62 sama), `opencode.go:36` (UA) |
| Satu session stabil per identitas | `0c6ab4f9` | derive endpoint id (stateless) |
| Reasoning item dibuang + `encrypted_content` dihapus | `eafac37d` | `dropOpenCodeReasoning` (`opencode_body.go:125-150`), test `opencode_wire_test.go:92-134` |
| Forced stream + decoy tools | `93837af0` | `TransformRequest` set `stream:true` + `ForcesStream()` (core ikut menyetel ulang, `transport_call.go:50-58`) |
| Leaf URL per wire (responses/messages/chat) | `acb5c34c`, `2b65c49f` | `Endpoint()` + `openCodeLeaf` (`opencode.go:104-128, 145-154`) |
| `max_output_tokens` rename | `executors/opencode.js:430-435` | `renameOpenCodeCeiling` (`opencode_body.go:105-119`) |

## 6. Keputusan owner

| # | Pertanyaan | Pilihan | Dipakai oleh |
|---|---|---|---|
| 1 | F1 schema: field `transports[]`+`supported_formats` masuk kontrak, atau opencode-zen dimapping entri tunggal? | **dijawab: `transports[]` + `supported_formats` masuk schema** (§7.3). Jalur entri-tunggal ditolak karena memecah satu akun menjadi tiga baris `upstream_endpoints` | F1, F2 |
| 2 | F1 usage: `UsageConfig` dihubungkan ke `Report` (jalur `reported` §7.12), atau usage dicatat deferred? | **deferred, dengan alasan terukur**: endpoint usage Zen menjawab 404 di upstream (diukur 2026-09-24), jadi jalur `reported` tidak punya sumber untuk provider itu hari ini. `opencode-go` menjawab 401 (endpoint ada, butuh kunci sah), jadi ia deferred sampai ada kunci. Dicatat di §7.4 | F1 |
| 3 | F2 urutan: regenerasi entri go menyusul keputusan 1, atau aturan per-model dimodelkan `target_format` saja dulu (tanpa schema baru)? | **dijawab: regenerasi mengikuti keputusan 1**, karena `transports[]` disetujui. Entri go diregenerasi dari `21583c03` (28 model, tiga endpoint) | F2 |
| 4 | F3 restorasi ejaan klien di respons: dibawa keluar connector (field di `Request` plugin), atau diterima bahwa klien melihat nama kanonik? | **diterima sebagai known-delta, dengan alasan**: pengukuran menunjukkan varian kapitalisasi diterima upstream, jadi restorasi hanya kosmetik untuk klien (`Bash` menjadi `bash` di sisi respons). Connector tidak mengganti nama yang klien deklarasikan, jadi yang terlihat klien adalah namanya sendiri kecuali ia menulisnya dengan kapitalisasi berbeda dari yang model panggil | F3 |
| 5 | F6 System One: port route + kind, atau NOT PORTED dengan alasan? | **dijawab: route diport** (`POST /api/v1/systemone`), dan model `kind: systemone` ditolak jalur chat dengan nama | F6 |
| 6 | F7: lisensi menulis ulang body same-format untuk opencode (A), atau known-delta (B)? | **dijawab: A, dengan lisensi sempit**: hanya anggota yang upstream tolak yang disentuh, dan setiap koreksi adalah rename atau clamp, bukan penghapusan isi klien | F7 |
| 7 | Prioritas: F3+F4 (free tier live rusak) duluan, atau F1+F2 (permukaan Zen/Go) duluan? | **dijawab: keduanya, berurutan** (§7.3): F3+F4+F5 lebih dulu (perbaikan kecil di jalur hidup), lalu F1+F2 (permukaan), lalu F6+F7+F8 | semua |
| 8 | F8: free tier harus bisa dipakai **tanpa membuat endpoint** (koneksi virtual seperti reference), atau tetap menuntut satu baris `upstream_endpoints`? | **dijawab: koneksi virtual diport**, dengan aturan sempit: hanya provider `no_auth` tanpa baris tersimpan | F8 |

## 7. Status pass ini

**CLOSED.** Tiga pass, ketiganya selesai:

| Pass | Keluaran | Status |
|---|---|---|
| Analisis (§4.1-§4.7) | dokumen ini | selesai |
| Uji free tier (§4.8) | 5 berkas test di `internal/dataplane/` (626 baris) | selesai |
| Implementasi gap F1-F8 (§7.4) | 5 commit kode + dokumen ini, 8 temuan tertutup | selesai |

- Semua angka diukur dari working tree 2026-09-24 dengan set in-flight §3.3; registry dibaca lewat
  `registry.Load()` yang memuat YAML ter-embed, bukan teks mentah, kecuali sitasi baris YAML yang
  menyebut nomor baris berkas.
- Gerbang akhir: `go build ./...` bersih, `go vet ./...` bersih (kedua tag set), `staticcheck`
  bersih (kedua tag set), `golangci-lint` 0 issues, `gofmt` bersih, `go test ./internal/...
  ./cmd/...` **seluruh 14 paket hijau**, dan kedua test live lulus terhadap `https://opencode.ai`
  sungguhan (free tier tiga model, dan route System One).
- **Mutation check dijalankan**: tiga mutasi kode produksi (decoy no-op, session tidak ditulis, UA
  tidak ditulis) masing-masing menggagalkan test, jadi test tidak buta.
- **Koreksi yang dicatat, bukan dihapus**: F3 turun HIGH ke MEDIUM (kuartet dan kanonisasi bukan
  syarat upstream, duplikat persis yang menjadi cacat), F4 turun MEDIUM ke LOW (`union-alpha`
  menjawab 401 `ModelError` di `/messages` dengan atau tanpa header). Keduanya di §4.8.
- **Temuan baru saat implementasi** (diukur, bukan diperkirakan): dua bug pada aturan
  multi-endpoint yang pass ini perkenalkan, keduanya ditangkap test sebelum commit. (a) `opencode-zen`
  dan `opencode-go` tidak terdaftar ke connector OpenCode, jadi URL-nya dibangun dari base chat
  entry: `gpt-5.6-luna` (responses-only) dikirim ke `/zen/v1/chat/completions`. (b) Guard
  `supported_formats` yang menolak lebih ketat dari reference menghapus **43 dari 74 model**
  `opencode-zen` dari routing (semua id claude-only, karena wire default provider adalah openai).
  Keduanya diperbaiki dan dipin dua test properti di `internal/dataplane/zen_routing_test.go`.
- **Yang tetap tidak diklaim**: jalur ber-kunci `opencode-zen`/`opencode-go` tidak diuji live (tidak
  ada kunci sah), dan usage provider-reported dicatat deferred dengan alasan terukur (§7.4).

### 7.4 Hasil implementasi (DURING, lima commit kode)

| Commit | Temuan yang ditutup | Isi |
|---|---|---|
| `7216a5a` | F1, F2, F3, F4, F5, F6 (guard) | Registry diregenerasi dari `21583c03` (121 provider, `opencode-zen` 74 model, `opencode-go` 28 model); generator reproducible dengan allowlist + laporan drift; fingerprint dedup; `anthropic-version`; capability 100 pola + tiga layer; model non-chat ditolak jalur chat |
| `d2ee432` | F7 | Enam aturan field Responses (quirk `tool_choice` dari registry, `store:false`, reasoning, input, `call_id`/`arguments`, flatten tools), plus `tool_choice:none` pada chat tanpa tools klien |
| `1eaa26c` | F8 | Endpoint virtual untuk provider `no_auth` tanpa baris tersimpan |
| `649d5bc` | F6 | Route `POST /api/v1/systemone` + aturan kind di satu tempat (`resolve_kind.go`), dipakai kedua plane |
| `11486a1` | F1, F2 | Tiga bug multi-endpoint: registrasi connector per lane, wire dari hasil translasi, fallback ke base entry (bukan tolak) |
| `-` (dokumen ini) | - | Status CLOSED, keputusan owner dijawab, spec §7.15 diperbarui |

**Yang diukur live dan hasilnya.** Tiga test live ber-tag `integration`:

```
go test -tags=integration ./internal/dataplane/ -run TestOpenCodeFreeLive
  --- PASS: space-bunny-free (1.69s)
  --- PASS: mimo-v2.6-flash-free (3.11s)
  --- PASS: muse-spark-1.3-contributor-free (48.58s, dua percobaan karena rate limit)

go test -tags=integration ./internal/service/ -run TestSystemOneLive
  --- PASS (1.38s): answers.probe terisi, model jev-1.13-free menjawab
```

Test free tier memakai retry berbatas (tiga percobaan, jeda 20 detik) untuk 429 dari lane bersama,
karena itu adalah sifat kapasitas upstream, bukan kegagalan pipeline. Assertion tetap utuh: test
menuntut jawaban, bukan melewati kegagalan.

**Satu gap yang dicatat deferred, bukan diklaim selesai.** Usage provider-reported (§7.12 jalur
`reported`): 24 provider mendeklarasikan `features.usage` di registry dan nol consumer di Go. Saat
mengukur untuk menutupnya, endpoint yang reference baca ternyata tidak tersedia:

```
GET https://opencode.ai/zen/v1/usage   -> 404 (endpoint tidak ada di sana)
GET https://opencode.ai/zen/go/v1/usage -> 401 (endpoint ada, butuh kunci sah)
GET https://opencode.ai/zen/v1/models   -> 200 (jalur yang sama, jadi host-nya benar)
```

Jadi jalur `reported` tidak punya sumber untuk provider ini hari ini, dan mengerjakannya tanpa kunci
sah berarti menulis kode yang tidak dapat dibuktikan. Ini keputusan owner nomor 2, dijawab deferred
dengan bukti di atas, bukan dikerjakan setengah.

### 7.1 Gerbang (antislop, DURING & AFTER)

Deliverable ini dokumen analisis tanpa UI; aturan yang berlaku dijawab, yang tidak berlaku dijawab
dengan alasannya, mengikuti format draft 027 §7.1 yang sudah diterima sebagai pola.

**Block 1: Hard Gate** (R-02 em dash): `grep -cP '\x{2014}' docs/DRAFT/029-OPENCODE-PROVIDER-PARITY.md`
= 0 (tanda hubung ganda pada tabel daftar id di §4.2 adalah koma dan spasi, bukan em dash). R-17/R-36:
setiap angka diikuti probe/perintah yang menghasilkannya; klaim 400 live pada F4 ditandai sebagai
bacaan aturan wire, bukan pengukuran. R-35: yang dijalankan adalah build, vet, test paket, dan probe
yang dikutip keluarannya; tidak ada UI untuk diklik dan itu dinyatakan. R-38: tidak ada konten
fabrikasi; sitasi kedua sisi dibaca dari checkout dan working tree.

**Block 2: Purpose-Gate**: tidak berlaku (tidak ada teknik visual); R-31 dijalankan pada tingkat isi:
setiap temuan menulis mengapa ia temuan dan setiap opsi menulis akibatnya.

**Block 3: Liveliness**: tidak berlaku (tidak ada layar/dial).

**Block 4: Craftsmanship**: C-1 lulus (setiap baris ledger bersitasi dua sisi), C-3 lulus (struktur
mengikuti pertanyaan owner: keluarga, perbandingan, presisi), C-5 lulus (bukti dikutip, bukan diklaim);
item UI lain tidak berlaku.

### 7.2 Kode probe (test penerimaan; dipasang kembali saat implementasi)

Probe pass analisis dijalankan sebagai berkas sementara lalu dihapus. Pass uji (§4.8) sudah
mendaratkan test permanen untuk free tier; sisanya tetap usulan bentuk, dipadatkan:

```go
// SUDAH MENDARAT (pass uji §4.8), internal/dataplane/engine_opencode_free*_test.go:
//   TestOpenCodeFree_ThreeModelsAnswerWithoutACredential  (3 model, tanpa kredensial, end to end)
//   TestOpenCodeFree_NoCredentialMaterialLeavesTheGateway (endpoint tanpa kunci)
//   TestOpenCodeFree_DeclaredResponsesModelTakesTheResponsesWire (muse -> /responses)
//   TestOpenCodeFree_UnknownFreeIdStillPassesThrough      (passthrough space-bunny/mimo)
//   TestOpenCodeFreeLive_ThreeModelsAnswerFromTheRealUpstream (build tag integration)

// F1: opencode-zen harus ada dengan jumlah model yang bisa dibaca.
func TestOpenCodeZen_RegistryEntry(t *testing.T) {
	index, _ := registry.Load()
	entry, ok := index.Provider("opencode-zen")
	if !ok {
		t.Fatal("opencode-zen is missing from the embedded registry")
	}
	if len(entry.Models) == 0 {
		t.Fatal("opencode-zen declares no models")
	}
}

// F2: model claude-target milik go harus tiba di /messages dengan x-api-key.
// Setelah perbaikan, connector yang dipakai adalah yang runtime pasang untuk entri ini
// (OpenCode, bila aturan per-model dimodelkan di sana); DefaultFactory adalah bentuk hari ini.
func TestOpenCodeGo_ClaudeTargetEndpoint(t *testing.T) {
	index, _ := registry.Load()
	entry, _ := index.Provider("opencode-go")
	model, _ := index.Model("opencode-go", "minimax-m2.7")
	url, err := provider.DefaultFactory(entry).Endpoint(
		provider.Request{Provider: entry, Model: model}, provider.Credential{})
	if err != nil || url != "https://opencode.ai/zen/go/v1/messages" {
		t.Fatalf("url = %s (err %v), want the messages endpoint", url, err)
	}
}

// F3 (kriteria diperbarui §4.3): dedup nama persis, bukan kuartet.
func TestOpenCode_DecoysAreDeduplicated(t *testing.T) {
	// klien [bash, bash, read] -> tepat satu bash + satu read
	// klien [bash, read]       -> tidak berubah
	// klien []                 -> bash + read disuntik
}

// F4: leaf /messages membawa anthropic-version.
func TestOpenCode_MessagesCarriesAnthropicVersion(t *testing.T) {
	request, _ := http.NewRequest(http.MethodPost, "https://opencode.ai/zen/v1/messages", nil)
	connector.ApplyAuth(request, provider.NoCredential("ep-1"))
	if request.Header.Get("anthropic-version") == "" {
		t.Fatal("the messages wire needs anthropic-version")
	}
}

// F5: capability vision ketiga model.
func TestRegistry_OpenCodeVision(t *testing.T) {
	for _, id := range []string{"muse-spark-1.2-contributor-free",
		"muse-spark-1.3-contributor-free", "union-alpha"} {
		if !registry.Capabilities("opencode", id).Vision {
			t.Fatalf("%s must be vision-capable", id)
		}
	}
}

// F8: nol endpoint + no_auth tetap terpilih (koneksi virtual).
func TestSelector_NoAuthProviderWithoutAnEndpoint(t *testing.T) {
	// provider no_auth tanpa baris upstream_endpoints -> Selection keyless,
	// bukan NO_PROVIDER_AVAILABLE seperti hari ini (selection_candidates.go:52).
}
```

### 7.3 Urutan pengerjaan yang disarankan

1. **F3** (dedup nama persis; test tabel) dan **F4** (satu baris `anthropic-version`, kesetiaan
   reference). Keduanya kecil dan mandiri.
2. **F5** (regenerasi korpus + tiga entri vision; keputusan pola vs exact-id).
3. **F8** (koneksi virtual `no_auth` tanpa endpoint) bila owner ingin free tier bisa dipakai tanpa
   membuat endpoint lebih dulu; ini yang membuat panel sepenuhnya setara reference.
4. **Keputusan 1 dan 3**, lalu **F2** (entri go: regenerasi atau target_format; menutup 23 model).
5. **Keputusan 2 dan 5**, lalu **F1** (zen + usage) dan **F6** (systemone) sebagai pass terpisah
   karena keduanya menambah permukaan wire baru (SPEC-API line lebih dulu, aturan spec-first §0).
6. **Keputusan 6**, lalu **F7** (quirk dari registry + enam koreksi bentuk, atau known-delta).
