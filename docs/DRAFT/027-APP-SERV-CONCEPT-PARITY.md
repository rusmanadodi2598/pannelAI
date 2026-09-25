# 027-APP-SERV-CONCEPT-PARITY.md: perbandingan konsep `app-serv` terhadap reference 9Router

Analisis konsep `app-serv/.` terhadap reference `https://github.com/decolua/9router`, dengan
pemeriksaan porting yang bisa diulang. Bukan kontrak; kontrak tetap `docs/SPEC-API/001-SPEC-API.md`
(wire) dan `docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel). Pola mengikuti `017` dan `019`: temuan
bernomor F, bukti yang bisa diulang, dan keputusan owner di depan implementasi.

| | |
|---|---|
| **Status** | **OPEN, dokumen analisis.** Tidak ada berkas source yang diubah pass ini; dokumen ini satu-satunya keluaran |
| **Mechanism** | DURING & AFTER (antislop) |
| **Scope** | `app-serv/.` dibandingkan reference. `app-ui/.` tidak disentuh |
| **Permintaan owner** | "Full analisis dan perbandingan konsep app-serv VS 9router (reference). Cek portingan secara presisi." |
| **Reference** | `decolua/9router`, checkout `/home/rusmanadodi/apps/9router`, **`21583c03` = tag `v0.5.85` (2026-09-22)**, sasaran yang sama dengan draft 017 |
| **Kaitan** | SPEC-API-001 §1, §2.1, §2.2, §7.8, §7.10, §7.12, §7.15, §8, §10; SPEC-API-002; draft 011 §8; draft 017 (F1-F10); draft 021 F5; draft 025; AGENTS.md §1.1-§1.9 |
| **Tanggal** | 2026-09-24 |

---

## 1. Ringkasan

Permintaan owner diterjemahkan menjadi tiga pertanyaan yang bisa diukur:

1. **Konsep apa saja yang dimiliki reference**, diukur dari pohon `v0.5.85`.
2. **Apa padanannya di `app-serv`**, diukur dari working tree ini, bukan dari ingatan.
3. **Seberapa presisi portingnya**: mana yang 1:1, mana yang dipetakan ke permukaan lain dengan
   alasan tertulis, mana yang stub, dan mana yang tidak ada tanpa keputusan tercatat.

Hasilnya, dalam angka:

| | Reference `v0.5.85` | `app-serv` |
|---|---|---|
| Berkas source (tanpa test) | 918 `.js` (src + open-sse) | 415 `.go` |
| Baris source (tanpa test) | 138.206 | 54.491 |
| Sisi server | 90.294 (api 16.934 + open-sse 54.947 + lib 15.435 + sse 2.978) | 54.491 (seluruh `app-serv`) |
| Sisi UI | 43.579 (src/app 32.092 tanpa api + src/shared 11.487) | `app-ui`: 26.928 dari 303 berkas (`find src -type f \( -name '*.svelte' -o -name '*.ts' \)`; test-nya di `app-ui/tests/`, 157 berkas) |
| Route terdaftar | 164 berkas `route.js` (140 manajemen + 24 data plane) | 96 route (84 manajemen + 12 data plane) |
| Entri registry | 121 impor di `registry/index.js` | 94 entri di `registry.yaml` |
| Format terjemahan | 16 pasangan request + 14 pasangan response | 3 target upstream (openai, claude, openai-responses) |

Empat hal sudah 1:1 secara sengaja dan tidak diusulkan berubah: pipeline routing beserta
retry/seleksi/circuit-nya, agregat endpoint multi-key (superset dari `connectionsRepo`), token saver
native (RTK 12 filter + Headroom + Ponytail), dan guard egress (lebih ketat daripada reference).

Enam temuan tercatat, dua di antaranya cacat yang terukur di berkas yang sudah ter-commit:

| # | Tingkat | Isi singkat |
|---|---|---|
| **F1** | HIGH | Permukaan wire klien Gemini dan Ollama tidak ada, sementara judul SPEC-API §7.15 mengklaim "Gemini-compatible" dan berkas penerjemah Gemini menyebut klaim itu sebagai alasannya |
| **F2** | HIGH | Konsep video adalah stub, dan komentar yang membenarkannya menyatakan hal yang **salah** tentang reference sasaran |
| **F3** | MEDIUM | "bypass detection (naming/warmup)" ada di diagram pipeline SPEC-API §7.15 dan **tidak ada kodenya** di `app-serv` |
| **F4** | MEDIUM | Daftar model data plane tanpa metadata (`capabilities`, `context_length`), dan satu field yang selalu bernilai nol |
| **F5** | MEDIUM | Katalog models.dev (sync harian) belum ada; ini lapisan keempat resolusi capability yang draft 017 §4.4 sebut tetapi perbaikannya tidak menyentuh |
| **F6** | LOW | Enam permukaan reference yang belum diporting dan belum punya keputusan tercatat di §2.1/§2.2 |

Tiga catatan pembacaan yang penting sebelum membaca temuan:

1. **Draft 017 ada di working tree, belum ter-commit** (`git status` menandainya `??`). Dokumen ini
   merujuknya sebagai sumber keputusan owner, dan status itu dinyatakan supaya tidak dikira sudah
   masuk sejarah.
2. **Working tree ini membawa pekerjaan pihak lain yang belum selesai** (pricing dan quota, berkas
   `??`/`M` sejak 2026-09-23 20:23 sampai 22:25). Semua angka di dokumen ini diukur dari working
   tree dan set in-flight itu disebut di §2.3; tidak satu pun temuan berdiri di atas berkas itu.
3. **Yang dimaksud "presisi" di sini adalah kesesuaian terhadap dua hal sekaligus**: perilaku
   reference pada revisi sasaran, dan klaim tertulis `app-serv` sendiri (SPEC-API). Sebuah temuan
   bisa berupa selisih dengan reference, atau klaim spec yang tidak punya kode di belakangnya. F1
   dan F3 adalah jenis kedua, dan keduanya sama-sama cacat karena pembaca spec akan mengira
   kemampuan itu ada.

## 2. Metode dan bukti

### 2.1 Revisi reference

Sasaran mengikuti keputusan owner di draft 017 §1.1 nomor 2 (**terbaru**), yaitu checkout yang sudah
di-`git pull`:

```bash
cd /home/rusmanadodi/apps/9router
git log -1 --format='%H%n%ci%n%d'   # 21583c03e5c5d5276924efad82328ebe6e215854
                                    # 2026-09-22 15:43:30 +0700
                                    # (HEAD -> master, tag: v0.5.85, origin/master, origin/HEAD)
git describe --tags                 # v0.5.85
```

Angka registry yang di-embed `app-serv` masih dari pin lama (`registry.yaml:5`:
`revision: 9router@db4499d (2026-06-19)`), jadi setiap baris di dokumen ini menyebut revisi mana
yang diukur. Perbedaan pin itu sendiri bukan temuan baru; ia keputusan terbuka draft 011 §8 dan
draft 017 §7.2 nomor 3.

### 2.2 Cara mengukur

Setiap angka di dokumen ini berasal dari perintah yang bisa dijalankan ulang. Empat kelompok
perintah yang dipakai berulang:

```bash
# Skala dan inventaris
cd /home/rusmanadodi/apps/9router
find src open-sse -name '*.js' -not -path '*/node_modules/*' | wc -l          # 918
find src open-sse -name '*.js' -not -path '*/node_modules/*' -exec cat {} + | wc -l   # 138206
find src/app/api -name 'route.js' | wc -l                                    # 164
find src/app/api -name 'route.js' -not -path '*/v1/*' -not -path '*/v1beta/*' | wc -l   # 140

cd /home/rusmanadodi/pannelAI/app-serv
find . -name '*.go' -not -name '*_test.go' | wc -l                            # 415
grep -c 'mux.Handle' internal/router/router.go internal/router/router_dataplane.go \
  internal/router/router_media.go internal/router/router_provider_nodes.go    # 73+6+9+8 = 96

# Registry
python3 -c "import yaml,collections; d=yaml.safe_load(open('internal/registry/registry.yaml')); \
  print(len(d['providers']))"                                                 # 94

# Capability media per kind
python3 -c "import yaml,collections; d=yaml.safe_load(open('internal/registry/registry.yaml')); \
  c=collections.Counter(k for p in d['providers'] for k in (p.get('media') or {})); print(dict(c))"
# {'embedding': 12, 'image': 16, 'tts': 16, 'stt': 5, 'webSearch': 9, 'webFetch': 4}
```

Pemeriksaan porting dilakukan dengan membaca **kedua sisi** pada baris yang sama dan menuliskan
sitasi `file:line` untuk keduanya, bukan hanya untuk reference. Sebuah baris ledger tanpa padanan
di `app-serv` diverifikasi dengan `grep` yang hasilnya kosong, dan hasil kosong itu yang dikutip.

### 2.3 Set in-flight yang dikecualikan

Working tree membawa pekerjaan yang belum selesai dan **bukan** bagian pass ini. Ia diukur supaya
tidak tertukar dengan temuan, dan supaya pembaca berikutnya tahu mengapa beberapa area terlihat
"setengah jalan":

```bash
cd /home/rusmanadodi/pannelAI && git status --short
```

| Berkas | Status | Isi |
|---|---|---|
| `app-serv/internal/registry/pricing{,_cost}.go`, `pricing.json`, `pricing_test.go`, `testdata/pricing_corpus.json`, `tools/pricing-gen.mjs` | `??` baru | Tabel tarif dan resolver biaya (konsep pricing reference) |
| `app-serv/internal/service/chat_record_cost{,_test}.go` | `??` baru | Pemakaian resolver di jalur pencatatan |
| `app-serv/internal/repository/redis/quota_counter*.go`, `postgres/quota*.go`, `service/quota_flush_*.go`, `service/chat_record.go`, `repository/usage.go` | `??`/`M` | Pekerjaan quota (settle/flush) |
| `docs/DRAFT/017-PROVIDER-PARITY-READINESS.md` | `??` | Draft pass provider, sumber keputusan owner yang dikutip dokumen ini |

Konsekuensinya untuk dokumen ini: baris ledger "Pricing" dan "Quota" menyebut keadaan **committed**
plus pekerjaan in-flight itu, dan **tidak** ada temuan yang berdiri di atas berkas in-flight.
Semua temuan F1-F6 menyentuh berkas yang sudah ter-commit atau spec.

### 2.4 Batas yang sudah dinyatakan (tidak dihitung sebagai temuan)

Tiga batas sudah tertulis di spec dan diulang di sini supaya tidak diperiksa ulang sebagai drift:

- **§2.2 NOT PORTED**: cloud sync, tunnel, MITM, pxpipe, CLI-tools writer (25 route), translator
  playground (6 route), headroom process manager (6 route), proxy-pool cloud deployer (3 route),
  SAML/OIDC (7 route), MCP (2 route), shutdown/update (4 route), i18n. Semuanya **tidak ada** di
  `app-serv` dan memang tidak diminta.
- **§8 `routability: connector`**: 9 provider chat ber-protokol khusus tidak diterjemahkan. Diukur:
  `kiro`, `antigravity`, `gemini-cli`, `ollama`, `ollama-local`, `vertex`, `cursor`, `gemini`,
  `commandcode`. Test `internal/registry/routability_test.go:89` mengunci enam di antaranya secara
  eksplisit; sisanya masuk lewat pemeriksaan "setiap provider harus mendarat di salah satu bucket".
- **§7.10 G21**: lima format media tanpa adapter (AssemblyAI STT, AWS Polly, Edge TTS, Google TTS,
  Local Device TTS) tetap `PROVIDER_NOT_ROUTABLE` yang menyebut formatnya. Diukur: `grep` kelima
  nama itu di `internal/service/*.go` mengembalikan nol kecuali di komentar spec.

## 3. Ledger konsep

Satu baris per konsep reference. Kolom **Status** memakai kosakata tertutup:

| Status | Arti |
|---|---|
| **1:1** | Ada padanannya dan perilakunya setara, atau lebih ketat dengan alasan tertulis |
| **dipetakan** | Ada padanannya tetapi di permukaan/ bentuk lain, dengan keputusan tertulis |
| **stub** | Route ada, eksekusinya belum ada |
| **belum** | Tidak ada padanannya; kolom keterangan menyebut apakah ada keputusan tercatat |
| **luar cakupan** | Tidak ada dan memang tidak diminta (§2.2) |

### 3.1 Mesin routing

| # | Konsep reference | Sumber reference | Padanan `app-serv` | Status |
|---|---|---|---|---|
| 1 | Pipeline permintaan (parse, resolve, capability, combo/adapter, saver, translate, select, call, record) | `src/sse/handlers/chat.js`, `open-sse/handlers/chatCore.js` | `internal/dataplane/engine.go` + `engine_*.go` | dipetakan; satu langkah di diagram §7.15 tidak punya kode (**F3**) |
| 2 | Resolusi model (combo, alias, `provider/model`, 404) | `src/sse/services/model.js` | `internal/dataplane/resolve.go`, `resolve_combo.go` | 1:1 |
| 3 | Seleksi akun (loop `excludeConnectionIds`, `allRateLimited` menjadi 503 + `retryAfter`) | `src/sse/handlers/chat.js:233` | `internal/dataplane/selection.go` | 1:1; selisih field di draft 017 F1b |
| 4 | Strategi per provider (`providerStrategies.fallbackStrategy`) | `src/sse/services/auth.js:138` | hanya `routing.sticky_limit` global | belum (tanpa keputusan) |
| 5 | Retry/backoff (3 percobaan, exponential + jitter, override per provider, cap POST non-idempoten) | `open-sse/handlers/chatCore.js` | `internal/dataplane/retry.go` | 1:1 |
| 6 | Circuit/health per key (`rateLimitedUntil`, `consecutiveErrors`) | `connectionsRepo.js`, `src/sse/services/auth.js` | `internal/dataplane/selection_health.go`, `upstream_key_health` | 1:1 |
| 7 | Rotasi sticky (`stickyRoundRobinLimit`) | `settingsRepo.js:14` | `internal/dataplane/rotation.go` | 1:1 |
| 8 | Terjemahan format (16 pasangan request, 14 response, pivot OpenAI, direct route) | `open-sse/translator/*` | `internal/dataplane/translate_*.go` | dipetakan: 3 target upstream dari 10 format; sisanya `connector` |
| 9 | Executor per protokol (31 berkas) | `open-sse/executors/*` | `internal/provider` (connector fallback + `Unsupported()`) | dipetakan: 3 keluarga native, sisanya ditolak bernama |
| 10 | Bypass permintaan CLI (warmup, count, title, naming) | `open-sse/utils/bypassHandler.js` | tidak ada | belum; diklaim ada di §7.15 (**F3**) |
| 11 | Token saver RTK (12 filter kanonik) | `open-sse/rtk/*` | `internal/tokensaver/*` (RTK + 12 filter) | 1:1 |
| 12 | Ponytail, Headroom, Caveman | `open-sse/rtk/{ponytail,headroom,caveman}.js` | `tokensaver/{ponytail,headroom}.go`; caveman deprecated | 1:1 dengan satu deprecation yang diputuskan (§7.9) |
| 13 | pxpipe (transform in-process) | `open-sse/rtk/pxpipe.js` | tidak ada | luar cakupan §2.2 |

### 3.2 Permukaan data plane

| # | Konsep reference | Sumber reference | Padanan `app-serv` | Status |
|---|---|---|---|---|
| 14 | Chat OpenAI | `src/app/api/v1/chat/completions/route.js` | `POST /api/v1/chat/completions` | 1:1 |
| 15 | Messages Anthropic | `src/app/api/v1/messages/route.js` | `POST /api/v1/messages` | 1:1 |
| 16 | Responses API | `src/app/api/v1/responses/route.js` (+ `compact`) | `POST /api/v1/responses` | 1:1 untuk jalur utama; `compact` belum |
| 17 | **Permukaan klien Gemini** | `src/app/api/v1beta/models/route.js` (62) + `[...path]/route.js` (585) | tidak ada | belum; judul §7.15 mengklaimnya (**F1**) |
| 18 | **Permukaan klien Ollama** | `src/app/api/v1/api/chat/route.js` + `transformToOllama` | tidak ada | belum (tanpa keputusan) |
| 19 | Daftar model + metadata | `src/app/api/v1/models/route.js` | `internal/dataplane/catalog.go` | dipetakan; metadata hilang (**F4**) |
| 20 | Model per kind, info, lookup satu model | `v1/models/[...model]/route.js`, `v1/models/info/route.js` | tidak ada | belum (**F6**) |
| 21 | Embeddings | `src/app/api/v1/embeddings/route.js` | `POST /api/v1/embeddings` | 1:1 |
| 22 | Audio (speech, transcriptions, voices) | `src/app/api/v1/audio/*` | `POST /api/v1/audio/speech`, `POST /api/v1/audio/transcriptions`, `GET /api/v1/audio/voices` | 1:1 |
| 23 | Images | `src/app/api/v1/images/generations/route.js` | `POST /api/v1/images/generations` | 1:1 |
| 24 | **Video (4 route, job async)** | `v1/videos/{generations,edits,extensions,[id]}` | satu route, `PROVIDER_NOT_ROUTABLE` | stub (**F2**) |
| 25 | Search | `src/app/api/v1/search/route.js` | `POST /api/v1/search` | 1:1 |
| 26 | Web fetch | `src/app/api/v1/web/fetch/route.js` + `handlers/fetch.js` | tidak ada (kind `webFetch` ada di registry) | belum (**F6**) |
| 27 | Systemone (keputusan Jev) | `src/app/api/v1/systemone/route.js` | tidak ada | belum (**F6**) |
| 28 | Hitung token | `src/app/api/v1/messages/count_tokens/route.js` | `POST /api/v1/messages/count_tokens` | 1:1 |

### 3.3 Permukaan manajemen

| # | Konsep reference | Sumber reference | Padanan `app-serv` | Status |
|---|---|---|---|---|
| 29 | Registry provider | `open-sse/providers/registry/*` (121 entri) | `internal/registry/registry.yaml` (94 entri, pin `db4499d6`) | belum; keputusan terbuka draft 011 §8 / draft 017 §7.2 |
| 30 | Koneksi/akun | `connectionsRepo.js` (1 koneksi = 1 key) | `upstream_endpoints` + `upstream_keys` (1..N) | 1:1, superset |
| 31 | Provider node kompatibel | `provider-nodes/*` (3 route) | 6 route + 2 route validate | 1:1 (draft 017 F3/F6 CLOSED) |
| 32 | Siklus hidup model (alias, custom, disabled) | `models/{alias,custom,disabled}` | 8 route | 1:1; `availability`, `test`, `catalog-sync` belum (draft 017 F10 + **F5/F6**) |
| 33 | Combo + preset | `combos`, `combos/[id]`, `combos/presets` | 6 route (tanpa preset) | 1:1; preset belum (**F6**) |
| 34 | Vision adapter | `capacityAdapter` (4 capability) | `vision-adapter` (1 capability) | 1:1 untuk vision; tiga capability lain diputuskan tidak (§7.8) |
| 35 | OAuth + refresh | 16 route, `src/lib/oauth/*` (50 berkas) | 4 route generik + `oauth/bulk` + refresh worker | dipetakan: route per-provider diganti satu jalur generik |
| 36 | Proxy pool | `proxy-pools/*` (6 route) + `proxyPoolsRepo.js` | 6 route + `settings.network.outbound_proxy_*` | 1:1 untuk global; pengikatan per-endpoint deferred (draft 017 F1b) |
| 37 | Gateway key | `apiKeysRepo.js`, `keys/*` | `gateway_keys` + 5 route | 1:1 |
| 38 | Usage | `src/app/api/usage/*` (10 route) | 4 route + `/usage/live` (5 total) + 3 route quota | dipetakan: dekomposisi berbeda, alasan di §7.12 |
| 39 | Quota tracker | `quotaAutoPing.js`, `sse/services/antigravityQuota.js` | `quota_windows` + `quota_caps` + flush worker | 1:1 (sebagian masih in-flight, §2.3) |
| 40 | Pricing | `src/app/api/pricing/route.js`, `pricingRepo.js`, `open-sse/providers/pricing.js` | `internal/registry/pricing*.go` (in-flight) | sedang dikerjakan, bukan temuan |
| 41 | Logs | `requestDetailsRepo.js`, `consoleLogBuffer.js` | `logs/requests` + `logs/console` | 1:1 |
| 42 | Settings | `settingsRepo.js` (52 key) | subset §7.14 (security, routing, network, token_saver, logging) | dipetakan; `providerStrategies`/`comboStrategies`/`quotaVisibility` belum |
| 43 | Skills, API docs, changelog | gitbook eksternal + `ChangelogModal` | §7.16-§7.18 (3 route) | 1:1 dengan bentuk berbeda (disengaja) |
| 44 | Guard SSRF | `ssrfGuard.js` (satu panggilan di `provider-nodes/validate`) | `internal/netguard` (4 lapis, anti-rebinding) | 1:1, lebih ketat |
| 45 | CLI tools (25 route) | `src/app/api/cli-tools/*` | tidak ada | luar cakupan §2.2 |
| 46 | Tunnel, Headroom manager, translator playground, MCP, SAML/OIDC, version/shutdown, locale | 33 route (7 + 6 + 6 + 2 + 7 + 4 + 1) | tidak ada | luar cakupan §2.2 |

## 4. Findings

### 4.1 F1 (HIGH): permukaan klien Gemini tidak ada, sementara judul §7.15 mengklaimnya

**Fakta.** Reference menyajikan permukaan **Gemini API** penuh untuk klien:

```bash
cd /home/rusmanadodi/apps/9router
find src/app/api/v1beta -name 'route.js' -exec wc -l {} +
#  585 src/app/api/v1beta/models/[...path]/route.js
#   62 src/app/api/v1beta/models/route.js
#  647 total
grep -n 'generateContent\|streamGenerateContent\|GEMINI_NATIVE_BASE_URL' \
  'src/app/api/v1beta/models/[...path]/route.js' | head
# 14:const GEMINI_NATIVE_BASE_URL = "https://generativelanguage.googleapis.com/v1beta/models";
# 170:  const upstreamUrl = new URL(`${GEMINI_NATIVE_BASE_URL}/...`);
```

Route itu menerima `POST /v1beta/models/{model}:generateContent` dan
`:streamGenerateContent`, menerjemahkan SSE OpenAI ke format Gemini, dan untuk model `gemini`
meneruskan ke `generativelanguage.googleapis.com/v1beta/models` (baris 14). Jadi ini permukaan
klien, bukan sekadar jalur upstream. Reference juga punya permukaan Ollama di
`src/app/api/v1/api/chat/route.js` (`transformToOllama`).

`app-serv` tidak punya satu pun dari keduanya:

```bash
cd /home/rusmanadodi/pannelAI/app-serv
grep -rn 'v1beta' internal/router/ | wc -l                          # 0
grep -rn 'OpenAIToGemini' --include='*.go' . | grep -v '_test.go' | wc -l   # 2
grep -rln 'OpenAIToGemini' --include='*.go' .
# ./internal/dataplane/translate_openai_gemini.go   (hanya berkas definisinya)
```

Yang ada adalah **penerjemah tanpa pemanggil**: `internal/dataplane/translate_openai_gemini.go`
membangun payload Gemini lewat `OpenAIToGemini` (`:146`), dan `grep` di atas membuktikan seluruh
kemunculan nama itu berada di berkas definisinya sendiri, nol di jalur permintaan. Berkas itu
menyatakan alasannya di header:

```bash
sed -n '10,19p' internal/dataplane/translate_openai_gemini.go
# @reason    SPEC-API-001 §7.15 lists Gemini among the translated formats and the
#            reference implements this direction as a pure function ...
```

Dan `targetFormat` memang mengecualikannya dengan sadar:

```bash
sed -n '41,45p' internal/dataplane/target.go
# Gemini is deliberately absent: the registry reports every provider declaring it
# as `routability: connector` ...
```

**Kenapa ini temuan, bukan sekadar deferral.** Dua klaim tertulis bertabrakan dengan kode:

1. Judul SPEC-API §7.15 berbunyi "Data Plane (OpenAI/Anthropic/Gemini-compatible, gateway-key
   auth)", sementara tabel di bawahnya (baris 560-568) hanya mendaftar OpenAI, Anthropic, dan
   OpenAI Responses. Pembaca spec, dan berkas `translate_openai_gemini.go` sendiri, memakai judul
   itu sebagai rujukan.
2. Deferral yang tercatat di SPEC-API §7.15 (kutipan di draft 017 §4.2) berbicara tentang
   `modelsFetcher`, bukan tentang permukaan klien Gemini. Tidak ada satu pun keputusan tertulis
   yang mengatakan "klien Gemini tidak dilayani"; yang ada adalah ketiadaan route.

**Akibat praktis.** Gemini CLI (dan klien apa pun yang berbicara wire Gemini) tidak bisa diarahkan
ke gateway ini, meskipun provider `gemini` ada di registry dan penerjemah payload-nya sudah
ditulis. Untuk Ollama, hal yang sama terjadi pada klien Ollama.

**Rencana DURING bila owner memilih membangun.** Dua bagian, dan yang pertama kecil:

1. `POST /api/v1beta/models/{model}:generateContent` dan `:streamGenerateContent`, memakai ulang
   `translate_openai_gemini.go` sebagai **arah upstream** dan menambah arah **klien** (Gemini
   request menjadi permintaan internal, jawaban OpenAI menjadi Gemini SSE). Pola `v1beta` reference
   bisa disalin perilakunya, termasuk penamaan `models/{provider}/{model}`.
2. Bila hanya ingin jujur: ubah judul §7.15 menjadi "OpenAI/Anthropic-compatible", dan ganti alasan
   di header `translate_openai_gemini.go` supaya menyebut bahwa berkas itu **belum punya rute**,
   bukan bahwa §7.15 sudah menjanjikannya.

**Kriteria selesai (bila dibangun).** Gemini CLI menyelesaikan satu percakapan streamed melalui
gateway, dan ada test tabel untuk kedua aksi URL (`:generateContent`, `:streamGenerateContent`).

### 4.2 F2 (HIGH): konsep video adalah stub, dan alasannya salah tentang reference

**Fakta.** `internal/service/media_image.go:53-56` (ter-commit di `da9d632`) membenarkan stub itu
dengan klaim bahwa reference berada dalam keadaan yang sama: belum ada provider registry yang
mendeklarasikan blok video, dan `videoConfig` disebut sebagai "a recognized key nothing defines".

```bash
cd /home/rusmanadodi/pannelAI/app-serv
sed -n '53,56p' internal/service/media_image.go
```

Kalimat "the same state the reference is in" **salah terhadap revisi sasaran**:

```bash
cd /home/rusmanadodi/apps/9router
grep -rn 'videoConfig' open-sse/providers/registry/*.js
# open-sse/providers/registry/xai.js:43:  videoConfig: { baseUrl: "https://api.x.ai/v1/videos" },
# open-sse/providers/registry/openrouter.js:71:  videoConfig: { ... }
# open-sse/providers/registry/vertex.js:38:  videoConfig: { baseUrl: "https://aiplatform.googleapis.com" },
find src/app/api/v1/videos -name 'route.js' | wc -l          # 4
wc -l open-sse/handlers/videoCore.js open-sse/handlers/videoProviders/*.js
#  209 videoCore.js, 13+39+159 videoProviders
```

Reference punya job video **async** (POST membuat job, GET mem-poll), tiga provider
(`xai` Grok Imagine, `openrouter`, `vertex` Veo lewat `predictLongRunning`), dan empat route
klien: `generations`, `edits`, `extensions`, `[id]`.

Kapan konsep itu mendarat, dan mengapa komentar di atas bisa terasa benar saat ditulis:

```bash
cd /home/rusmanadodi/apps/9router
git log -1 --format='%h %ci %s' d6761c6f      # 2026-07-16 feat(xai): add Grok Imagine video generation
git log -1 --format='%h %ci %s' 3288bbc4      # feat(video): add OpenRouter and Vertex AI (Veo)
git show db4499d6:open-sse/providers/registry/xai.js | grep -c videoConfig   # 0
git merge-base --is-ancestor d6761c6f 21583c03 && echo "ancestor of v0.5.85"
```

Pin `db4499d6` (2026-06-19) memang belum punya video; konsepnya mendarat 2026-07-16, **sesudah**
pin dan **sebelum** `v0.5.85`. Jadi kalimat itu benar tentang pin dan salah tentang sasaran, dan
itu jenis kesalahan yang draft 017 §9.2 sudah catat satu kali untuk capability: kesimpulan yang
benar pada satu revisi dipakai untuk membenarkan keadaan di revisi lain.

Sisi `app-serv`, diukur dari registry yang di-embed:

```bash
cd /home/rusmanadodi/pannelAI/app-serv
python3 -c "import yaml,collections; d=yaml.safe_load(open('internal/registry/registry.yaml')); \
  print(dict(collections.Counter(k for p in d['providers'] for k in (p.get('media') or {}))))"
# {'embedding': 12, 'image': 16, 'tts': 16, 'stt': 5, 'webSearch': 9, 'webFetch': 4}
# tidak ada 'video' sama sekali
python3 -c "import yaml; d=yaml.safe_load(open('internal/registry/registry.yaml')); \
  print([(p['id'], [m['id'] for m in (p.get('models') or []) if m.get('kind')=='video']) \
  for p in d['providers'] if any(m.get('kind')=='video' for m in (p.get('models') or []))])"
# [('runwayml', ['gen4_turbo', 'gen3a_turbo'])]
```

Jadi registry menyebut dua model video di `runwayml`, sementara blok media `video` nol. Route
`POST /api/v1/videos/generations` ada dan menjawab `PROVIDER_NOT_ROUTABLE`, dan tiga route video
lainnya tidak ada.

**Dua lapis biaya yang harus dipisahkan supaya keputusannya jujur:**

1. **Lapis pin**: regenerate registry dari `v0.5.85` akan membawa `videoConfig` untuk tiga provider
   (generator sudah memetakannya: `tools/registry-gen.mjs:174` berisi `["videoConfig", "video"]`).
   Ini keputusan draft 011 §8 yang sama.
2. **Lapis mesin**: meskipun blok media-nya ada, `app-serv` belum punya job async (submit lalu
   poll), belum punya adapter Veo (`predictLongRunning`), dan hanya punya 1 dari 4 route. Ini
   pekerjaan baru, bukan field baru.

**Rencana DURING.** Tiga pilihan di §6; yang termurah adalah memperbaiki komentar supaya
menyebutkan pin sebagai alasannya (bukan reference), dan mencatat konsep video sebagai pekerjaan
terpisah. Yang termahal adalah memporting konsepnya utuh.

### 4.3 F3 (MEDIUM): "bypass detection (naming/warmup)" ada di spec dan tidak ada di kode

**Fakta.** SPEC-API §7.15 menuliskan pipeline permintaan sebagai berikut:

```bash
cd /home/rusmanadodi/pannelAI
sed -n '570,579p' docs/SPEC-API/001-SPEC-API.md
# Request pipeline (port of `sse/handlers/chat.js` + `open-sse`):
# auth → schema validation → bypass detection (naming/warmup) → model resolve
#   → required-capability detection → ...
```

Reference memang punya langkah itu, dan bentuknya bukan satu baris:

```bash
cd /home/rusmanadodi/apps/9router
grep -n 'handleBypassRequest\|SKIP_PATTERNS' open-sse/utils/bypassHandler.js | head -3
# 11:export function handleBypassRequest(body, model, userAgent = "", ccFilterNaming = false)
# 50:  if (!shouldBypass && SKIP_PATTERNS?.length) {
grep -n 'Pattern [0-9]\|isNewTopic' open-sse/utils/bypassHandler.js
# 27, 33, 41, 49, 58 (lima pola), 67 dan 83 (nama topik)
wc -l open-sse/utils/bypassHandler.js                        # 313
```

Lima pola, semuanya digerbangi `user-agent` yang mengandung `claude-cli`:

| Pola | Deteksi | Jawaban |
|---|---|---|
| Title extraction | pesan terakhir `assistant` dengan konten pertama `"{"` | teks tetap `DEFAULT_BYPASS_TEXT` |
| Warmup | pesan pertama berisi `"Warmup"` | idem |
| Count | satu pesan `user` berisi `"count"` | idem |
| Skip patterns | `SKIP_PATTERNS` cocok di pesan user | idem |
| Naming (`isNewTopic`) | `ccFilterNaming` dan `system` memuat `isNewTopic` | JSON `{isNewTopic:true,title}` dengan tiga kata pertama |

`app-serv` tidak punya satu pun:

```bash
cd /home/rusmanadodi/pannelAI/app-serv
grep -rn 'Warmup\|isNewTopic\|SKIP_PATTERNS\|claude-cli' --include='*.go' . | wc -l   # 0
```

**Akibat praktis.** Klien utama gateway ini adalah Claude Code, dan Claude Code memang mengirim
permintaan warmup, permintaan `count`, dan permintaan penamaan topik. Tanpa langkah ini, ketiganya
diteruskan ke upstream dan dibayar sebagai permintaan penuh, sementara permintaan penamaan
mendapat jawaban model sungguhan alih-alih judul sintetis yang CLI harapkan.

**Rencana DURING.** Port sebagai satu fungsi murni di `internal/dataplane` (atau `internal/service`)
plus pembangun jawaban per wire (OpenAI, Claude), dengan test tabel untuk kelima pola, dan satu
kasus negatif yang membuktikan permintaan biasa tidak di-bypass. Ukurannya kecil: perilakunya
fungsi dari `body`, `user-agent`, dan satu flag setting.

**Kriteria selesai.** Permintaan warmup/count/title tidak menghasilkan panggilan upstream, penamaan
menjawab JSON `isNewTopic`, dan permintaan biasa tidak tersentuh. Atau, bila owner memilih tidak
membangunnya: kalimat "bypass detection (naming/warmup)" dihapus dari §7.15 karena ia klaim yang
tidak diukur siapa pun.

### 4.4 F4 (MEDIUM): daftar model data plane tanpa metadata, dan satu field yang selalu nol

**Fakta.** Reference menerbitkan metadata per model di `/v1/models`, dan komentarnya menjelaskan
konsekuensi klien:

```bash
cd /home/rusmanadodi/apps/9router
sed -n '506,531p' src/app/api/v1/models/route.js
```

Yang diterbitkan: `capabilities` (baris 511, bila resolver mengembalikan sesuatu),
`context_length` dan `max_completion_tokens` (baris 530-531, hanya bila nilainya finite), dan untuk
combo `capabilities` agregat (baris 327). Alasan yang ditulis reference di situ: klien yang menebak
jendela konteks dari nama model "guess high", dan model 372k yang dibaca 1.05M "never reaches its
compaction threshold and hard-fails upstream".

`app-serv` menerbitkan empat field, dan satu di antaranya konstan:

```bash
cd /home/rusmanadodi/pannelAI/app-serv
sed -n '60,64p;73p' internal/dataplane/catalog.go
sed -n '104,109p' internal/schema/chat_response.go
grep -rn 'ModelObject{' --include='*.go' . | grep -v _test
# ./internal/dataplane/catalog.go:60 ... satu-satunya dua tempat pembentukan
```

`Created int64` tidak pernah diisi di kedua tempat itu, jadi setiap baris di wire berbunyi
`"created":0`. Ini cacat yang sama dengan `Suggested: true` yang draft 017 F5 hapus: field yang
selalu sama tidak membawa informasi dan menyesatkan pembaca.

Yang penting: **capability-nya sudah ada di permukaan manajemen**. `schema.ModelResponse`
(`internal/schema/model.go:32`) membawa `capabilities`, dan resolver yang draft 017 F4 bangun
(`internal/registry/capability_resolve.go`) sudah menjawab `vision`/`tools`. Yang hilang hanya
pemakaiannya di daftar data plane, plus jendela konteks yang tidak dimiliki registry sama sekali
(lihat F5).

**Rencana DURING.**

1. Isi `capabilities` di `ModelList` dari resolver yang sudah ada, untuk setiap model registry.
2. Putuskan `created`: diisi dari waktu boot (konstan per proses) atau dihapus dari wire. Rekomendasi
   penulis: hapus, karena reference pun tidak menerbitkannya dan nilainya tidak dipakai siapa pun.
3. Jendela konteks (`context_length`, `max_completion_tokens`) menunggu F5, karena datanya tidak ada
   di registry.

**Kriteria selesai.** `GET /api/v1/models` memuat `capabilities` yang cocok dengan
`GET /api/v1/models/catalog`, dan tidak ada field yang nilainya konstan di seluruh baris.

### 4.5 F5 (MEDIUM): katalog models.dev belum ada, dan itu lapisan yang draft 017 §4.4 sebut

**Fakta.** Reference menyegarkan katalog capability dari models.dev setiap 24 jam dan membacanya
di jalur panas:

```bash
cd /home/rusmanadodi/apps/9router
wc -l src/lib/modelCatalog/sync.js open-sse/providers/catalogOverride.js   # 268, 82
grep -n 'CATALOG_URL\|SYNC_INTERVAL_MS\|RETRY_DELAY_MS' src/lib/modelCatalog/sync.js | head -4
# const CATALOG_URL = "https://models.dev/api.json";
# export const SYNC_INTERVAL_MS = 24 * 60 * 60 * 1000;
find src/app/api/models/catalog-sync -name 'route.js' -exec wc -l {} +    # 31
```

`open-sse/providers/catalogOverride.js:1-2` menyatakan perannya: "Read side of the model catalog
synced from models.dev", dengan cache yang dibuang saat mtime berubah, dibaca
`getCapabilitiesForModel` per permintaan.

Draft 017 §4.4 menyebut lapisan ini sebagai bagian dari empat lapis resolusi reference
("`PROVIDER_CAPABILITIES`, `MODEL_CAPABILITIES`, `PATTERN_CAPABILITIES`, lalu `DEFAULT_CAPABILITIES`
... ditambah katalog models.dev dan `visionPatterns.js`"), dan §9.2 menutup F4 dengan resolver pola
saja. Jadi lapisan katalog ini **tidak** ikut dikerjakan, dan tidak ada keputusan tertulis yang
mengeluarkannya.

```bash
cd /home/rusmanadodi/pannelAI/app-serv
grep -rn 'models.dev\|catalog-sync\|catalogOverride' --include='*.go' . | wc -l   # 0
```

**Akibat.** Model yang tidak ada di tabel pola tangan mendapat `DEFAULT_CAPABILITIES` dan tanpa
batas token. Untuk model baru (yang justru paling sering dipakai lewat node kompatibel), reference
tahu capability dan limit-nya dalam 24 jam; `app-serv` tidak pernah tahu. Ini juga alasan F4
langkah 3 tidak bisa dikerjakan sekarang.

**Rencana DURING.** Dua bentuk, dan yang pertama lebih kecil:

1. **Port penuh**: worker sync (interval, retry, ETag) + penyimpanan (tabel atau berkas) + pembacaan
   di resolver capability. Menyentuh AGENTS.md §1.6 (worker dengan retry policy dan panic recovery).
2. **Nyatakan batasnya**: catat di §7.6/§7.9 bahwa lapisan katalog models.dev tidak diporting, dan
   bahwa konsekuensinya adalah `DEFAULT_CAPABILITIES` untuk model di luar tabel. Ini membuat F4
   langkah 3 menjadi keputusan yang sadar, bukan gap yang tak terlihat.

### 4.6 F6 (LOW): enam permukaan reference tanpa keputusan tercatat

**Fakta.** Enam permukaan berikut ada di reference, tidak ada di `app-serv`, dan **tidak** disebut
di §2.1 KEEP maupun §2.2 NOT PORTED. §2.2 menutupnya dengan kalimat "Anything not listed in §2.1 is
assumed not wanted", jadi secara kontrak tidak ada pelanggaran; yang hilang adalah jejak
keputusannya.

| # | Permukaan reference | Ukuran | Catatan |
|---|---|---|---|
| 1 | `GET /api/combos/presets` | 110 baris | Combo siap pakai dari provider yang terhubung (`PRESET_SOURCES`) |
| 2 | `GET /api/models/catalog-sync`, `POST` | 31 baris | Sisi route dari F5 |
| 3 | `GET /api/tags` | 18 baris | Daftar model Ollama (`open-sse/config/ollamaModels.js`) |
| 4 | `GET /api/media-providers/tts/voices` dan 4 route per-provider | 5 route | Daftar suara di permukaan **manajemen**; `app-serv` menyajikannya di data plane (`GET /audio/voices`) |
| 5 | `GET /v1/models/{kind}` (image, tts, stt, embedding, image-to-text, web) | 78 baris | Discovery model media oleh klien data plane |
| 6 | `GET /v1/models/info` | 104 baris | Metadata model (`kind`, `endpoint`, `params`, `voicesUrl`, `searchTypes`) |

**Rencana DURING.** Tidak ada kode yang diusulkan. Yang diusulkan adalah satu paragraf di §2.2 yang
menyebut keenamnya, supaya pemeriksaan berikutnya tidak mengulang pertanyaan yang sama. Bila owner
mau salah satunya dibangun, nomor 5 dan 6 adalah yang paling dekat dengan F4 (metadata model) dan
datanya sebagian sudah ada di permukaan manajemen.

## 5. Non-findings

Dicatat supaya tidak diperiksa ulang sebagai drift.

| Hal | Kenapa bukan temuan |
|---|---|
| Daftar model reference digerbangi koneksi aktif, `app-serv` tidak | Sudah dinyatakan di SPEC-API §7.15 changelog 2026-09-20 ("the list is connection-gated, which this port's registry-driven list never was"), dan dikutip draft 017 §4.2 |
| 9 provider chat ber-`routability: connector` | Keputusan §10 P1 yang dipublikasikan lewat `GET /providers` (§8), dengan test yang mengunci bucket-nya (`routability_test.go:89`) |
| Registry 94 entri vs 121, pin `db4499d6` | Keputusan terbuka draft 011 §8 dan draft 017 §7.2 nomor 3, bukan temuan baru |
| Lima format media tanpa adapter (G21) | §7.10 menyatakannya, termasuk alasan per format |
| `capacityAdapter` hanya vision | §7.8: "v1 ships vision only; the adapter framework is capability-generic internally (pdf/audio/video adapters from the reference are not ported)" |
| `supportedFormats` per model (multi-transport) | Konsep pasca-pin; bagian dari lag registry yang sama, bukan kegagalan porting terpisah |
| `providerStrategies`/`comboStrategies`/`quotaVisibility` tidak ada | Bagian dari subset settings §7.14 yang tertulis; yang belum tercatat hanya strategi per provider (baris 4 di §3.1), dan itu dicatat sebagai belum tanpa keputusan |
| `PATCH` vs `PUT`, `CONFLICT` pada delete node, `auth_modes` inert | Sudah dibahas draft 017 §5 Non-findings |
| Jalur usage streamed yang mencatat 0/0 token | Sudah difilekan sebagai draft 021 F5; masih terbuka dan tidak diulang di sini |
| Em dash di dokumen DRAFT lain | Pra-ada dan bukan milik pass ini (mis. draft 017); dokumen ini nol em dash, dan sweep dokumen lain bukan scope-nya |

## 6. Keputusan owner

Enam pertanyaan, satu per temuan. Tidak ada source yang diubah sebelum jawabannya ada.

| # | Pertanyaan | Pilihan | Rekomendasi penulis |
|---|---|---|---|
| 1 | **F1 permukaan klien Gemini/Ollama** | (a) Bangun permukaan `v1beta` + Ollama; (b) ubah judul §7.15 dan alasan di `translate_openai_gemini.go` supaya jujur; (c) biarkan seperti sekarang | **(b) sekarang**, (a) sebagai slice terpisah bila Gemini CLI memang mau dilayani |
| 2 | **F2 video** | (a) Perbaiki komentar saja, konsep video dicatat sebagai pekerjaan terpisah; (b) porting utuh (4 route + job async + adapter Veo/Grok); (c) hapus route stub dan dua model video `runwayml` sampai diporting | **(a)**; (b) hanya bila video xAI/Veo memang dipakai |
| 3 | **F3 bypass** | (a) Porting (kecil, nilainya langsung untuk Claude Code); (b) hapus klaim dari §7.15 | **(a)** |
| 4 | **F4 metadata model** | (a) Isi `capabilities` dari resolver + hapus `created`; (b) isi `capabilities` dan isi `created` dari waktu boot; (c) biarkan | **(a)**, `context_length` menyusul bersama F5 |
| 5 | **F5 katalog models.dev** | (a) Porting worker sync + pembacaan; (b) nyatakan sebagai batas tertulis di §7.6/§7.9 | **(b) dulu**, (a) bila batas token per model memang dibutuhkan klien |
| 6 | **F6 enam permukaan** | (a) Sebut keenamnya di §2.2 sebagai tidak diminta; (b) pilih sebagian untuk dibangun | **(a)** |

## 7. Status pass ini

- **Analisis selesai; tidak ada berkas source yang diubah.** Keluaran pass ini satu berkas:
  `docs/DRAFT/027-APP-SERV-CONCEPT-PARITY.md`.
- Seluruh angka diukur dari working tree 2026-09-24, dengan set in-flight yang dinyatakan di §2.3
  dan dikecualikan dari temuan.
- Reference diukur pada `21583c03` = `v0.5.85`, revisi sasaran yang sama dengan draft 017.
- Empat klaim yang saling bertabrakan ditemukan dengan `grep`/`sed` yang bisa diulang: F1 (judul
  §7.15 dan header penerjemah Gemini), F2 (komentar `media_image.go:53-56`), F3 (§7.15 baris 573),
  F4 (`created` konstan). Dua di antaranya menyentuh berkas yang sudah ter-commit; dua menyentuh
  spec.
- Tidak ada test yang dijalankan dan tidak ada gate build yang dijalankan, karena pass ini tidak
  mengubah kode. Yang dijalankan adalah perintah pengukuran, dan keluarannya dikutip apa adanya.

### 7.1 Gerbang (antislop, DURING & AFTER)

Pass ini tidak menyentuh UI. Empat blok tetap dilaporkan seluruhnya; item yang tidak berlaku
dijawab dengan alasannya, dan yang berlaku dijawab dengan bukti.

**Block 1: Hard Gate (17 item, semua harus "tidak")**

| Item | Jawaban | Bukti / alasan |
|---|---|---|
| R-02 em dash | **tidak** | `grep -cP '\x{2014}' docs/DRAFT/027-APP-SERV-CONCEPT-PARITY.md` = 0 |
| R-03 overflow mobile | tidak berlaku | Tidak ada UI; keluaran berkas markdown |
| R-17 angka tanpa sumber | **tidak** | Setiap angka diikuti perintah yang menghasilkannya (§2.2 dan setiap temuan) |
| R-18 testimoni fiktif | tidak berlaku | Tidak ada testimoni |
| R-23 aset tanpa instruksi | tidak berlaku | Tidak ada aset visual yang dibuat |
| R-24 navigasi mati | tidak berlaku | Tidak ada navigasi |
| R-25 kontras | tidak berlaku | Tidak ada teks ter-render |
| R-26 kontrol mati | tidak berlaku | Tidak ada kontrol |
| R-27 state UI | tidak berlaku | Tidak ada UI berdata |
| R-28 FAQ | tidak berlaku | Tidak ada FAQ |
| R-32 keyboard | tidak berlaku | Tidak ada UI |
| R-33 patch via script | **tidak** | Pass ini menulis satu berkas markdown; tidak ada skrip yang menyunting source |
| R-34 tema | tidak berlaku | Tidak ada tema |
| R-35 dijalankan sebelum diserahkan | tidak berlaku untuk UI | Tidak ada UI yang berubah; yang dijalankan adalah perintah pengukuran, dan keluarannya dikutip (§2.2). Tidak ada build yang dijalankan karena tidak ada kode yang berubah, dan itu dinyatakan apa adanya |
| R-36 klaim fabrikasi | **tidak** | Setiap temuan menyitir berkas yang bisa dibuka; tidak ada klaim keamanan/compliance |
| R-37 arah desain | tidak berlaku | Tidak ada UI yang dibangun |
| R-38 konten fabrikasi | **tidak** | Sitasi reference dibaca dari checkout `21583c03`; sitasi `app-serv` dari working tree |

**Block 2: Purpose-Gate (12 item)**

Seluruhnya **tidak berlaku** dengan satu alasan yang sama: deliverable ini dokumen analisis, tidak
memuat gradien, ikon, tipografi, latar bermotif, panah, badge, glassmorphism, shadow, glow, kartu,
animasi, atau ilustrasi. Tidak ada teknik visual yang dipakai, jadi tidak ada alasan yang perlu
ditulis. R-31 (alasan satu baris per keputusan) justru **dijalankan** pada tingkat isi: setiap
temuan menuliskan mengapa ia temuan dan setiap opsi menuliskan akibatnya (§6).

**Block 3: Liveliness (7 item)**

Tidak berlaku seluruhnya: tidak ada layar, jadi tidak ada dial ENERGY/RHYTHM/MOTION yang bisa
dinyatakan dan tidak ada focal point, whitespace, accent, atau motif identitas yang bisa dinilai.
Draft tanpa arah tidak dibuat; yang dibuat adalah dokumen kerja, dan statusnya dinyatakan di tabel
header (OPEN, analisis).

**Block 4: Craftsmanship & Quality Locks (14 item)**

| Item | Jawaban | Bukti / alasan |
|---|---|---|
| C-1 intentionality | **tidak** | Setiap baris ledger punya sitasi kedua sisi; tidak ada keputusan yang alasannya "default AI" |
| C-2 functional completeness | tidak berlaku | Tidak ada elemen interaktif |
| C-3 content-driven composition | **tidak** | Struktur mengikuti pertanyaan owner (§1), bukan template |
| C-4 resilience | tidak berlaku | Tidak ada UI |
| C-5 evidence over claims | **tidak** | Tidak ada testimoni/statistik tanpa sumber |
| R-05 template layout | tidak berlaku | Tidak ada layout |
| R-11 radius | tidak berlaku | Tidak ada UI |
| R-15 CTA | tidak berlaku | Tidak ada CTA |
| R-16 buzzwords | **tidak** | Tidak ada "AI Powered", "Seamless", "Revolutionary" di teks |
| R-20 identitas visual | tidak berlaku | Tidak ada visual |
| R-21 dark mode | tidak berlaku | Tidak ada tema |
| R-29 palet | tidak berlaku | Tidak ada warna |
| R-30 klon produk | tidak berlaku | Tidak ada visual |
| R-31 alasan tertulis | **tidak** (lulus) | Setiap temuan menulis alasan dan setiap opsi menulis akibat (§4, §6) |

Tidak ada item yang FAIL. Satu catatan jujur: R-35 dan seluruh Block 2/3 dijawab "tidak berlaku",
bukan "PASS", karena tidak ada UI yang bisa diklik atau dijalankan; satu-satunya bukti yang bisa
diberikan pass ini adalah perintah pengukuran yang bisa diulang, dan itu yang dikutip.
