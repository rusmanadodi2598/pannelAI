# 017-PROVIDER-PARITY-READINESS.md: Parity `/provider` app-serv terhadap REFERENCE

Dokumen kerja hasil pemeriksaan permukaan provider `app-serv/.` terhadap reference
`https://github.com/decolua/9router`. Bukan kontrak; kontrak tetap `docs/SPEC-API/001-SPEC-API.md`
(wire) dan `docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel). Pola mengikuti `008` sampai `016`:
temuan bernomor F, bukti yang bisa diulang, rencana DURING, dan keputusan owner di depan implementasi.

| | |
|---|---|
| **Status** | **CLOSED 2026-09-23** untuk scope pass ini: F2 (A+B), F3, F4, F5, F6, dan F1b selesai, seluruh gate lulus. **F7, F9, F10 dan regenerasi registry tetap OPEN (§7.2).** Satu deferral dicatat: `proxy_pool_id` tersimpan, tervalidasi, dan tersaji; **pengikatan ke egress per-endpoint belum** (§4.1b) |
| **Mechanism** | DURING & AFTER (antislop) |
| **Scope** | `app-serv/.` saja. `app-ui/.` tidak disentuh pass ini |
| **Permintaan owner** | `app-serv` endpoint `/provider` disamakan 1:1 dengan REFERENCE: cara penanganan, cara penambahan provider (custom OpenAI Compatible, Anthropic, dan provider tertentu), cara mengambil models id, dan penanganan models id |
| **Reference** | `decolua/9router`, checkout `/home/rusmanadodi/apps/9router`, **`21583c03` = tag `v0.5.85` (2026-09-22)**, `git pull` 2026-09-23 |
| **Kaitan** | SPEC-API §6, §7.4, §7.5, §7.6; SPEC-UI §6.3; AGENTS.md §1.1–§1.9, §2.4; `docs/RULLES/OWASP.md` (A01 SSRF); draft 003, 004, 011 |
| **Tanggal** | 2026-09-23 |

### 1.1 Keputusan owner (2026-09-23)

| # | Pertanyaan (§7) | Jawaban | Konsekuensi |
|---|---|---|---|
| 1 | Arti `/providers` | **A — pertahankan registry**, tetapi tetap harus 1:1 dengan REFERENCE | Registry tetap sumber `GET /providers`; kesejajaran 1:1 di titik itu dipenuhi lewat pemetaan yang dinyatakan (§3.1), bukan dengan mengubah artinya |
| 2 | Revisi reference sasaran | **Terbaru** — `git pull` dulu | Checkout sudah dinaikkan dari `db4499d6` (2026-06-19) ke `21583c03`/v0.5.85 (2026-09-22). Semua angka di dokumen ini diukur ulang terhadap revisi itu (§2.1) |
| 3 | Cakupan | **Semua: F2–F6** | F2 (bagian A+B), F3, F4, F5, F6 masuk pass ini. F1, F7, F9, F10 mengikuti §6 |

## 1. Ringkasan

Permintaan owner diterjemahkan menjadi empat pertanyaan yang bisa diukur, dan setiap jawabannya
dibandingkan dengan reference pada baris kode yang bisa dibuka ulang:

1. **Cara penanganan** — apa yang sebenarnya dilakukan route itu.
2. **Cara penambahan provider** — jalur mana yang sah untuk menambah provider, dan apa yang
   divalidasi sebelum barisnya disimpan.
3. **Cara mengambil models id** — dari mana daftar model datang.
4. **Penanganan models id** — bagaimana id itu dipakai setelah ada.

Hasilnya: **empat hal sudah 1:1 secara sengaja**, dan **sepuluh temuan tercatat** (F1–F10), tiga di
antaranya cacat yang terukur hari ini:

| Sudah 1:1 | Bukti |
|---|---|
| Provider registry membaca katalog statis, sama seperti reference | `page.js:136` vs `internal/service/provider.go:79` |
| CRUD node kompatibel (`GET`/`POST`/`GET{id}`/`PATCH`/`DELETE`/`test`) | §2.2 |
| CRUD custom model | §2.2 |
| Bentuk id node (prefix + ULID) | §4.9 |

Yang paling menentukan bukan jumlah temuan, melainkan satu perbedaan definisi yang harus diputuskan
owner lebih dulu (§3), karena pilihan itu menentukan apakah temuan yang menyentuh permukaan route
dikerjakan apa adanya atau berubah bentuk.

Tiga temuan di bawah ini bukan selisih gaya melainkan cacat yang terukur hari ini:

- **F2** — node OpenAI-compatible/Anthropic-compatible yang baru dibuat mengembalikan daftar model
  **kosong**: entry yang disintesis tidak pernah diberi model, sehingga `ModelCount` = 0 dan katalog
  tidak memuat satu model pun untuk node itu. Diukur dengan menjalankan `WithCustom` atas registry
  nyata (§4.2).
- **F3** — `base_url` tidak pernah disanitasi, sehingga node Anthropic-compatible yang operator isi
  dengan `https://host/v1/messages` mengirim ke `https://host/v1/messages/messages`. Dibuktikan
  dengan menjalankan `joinPath` yang asli (§4.3).
- **F4** — dua filter capability yang panel tawarkan (`vision`, `tools`) mengembalikan **nol baris**
  terhadap registry nyata, sementara 507 model terdaftar. Diukur dengan membaca registry yang di-embed
  (§4.4).

Satu koreksi pembacaan dicatat di §4.2: versi pertama audit ini menyimpulkan "reference selalu
mengambil daftar model dari upstream", dan itu keliru — untuk provider registry, reference membaca
katalog statis, sama seperti `app-serv`. Yang live hanya jalur node kompatibel. Koreksinya dibiarkan
tertulis, bukan dihapus, karena kesimpulan yang salah itu yang menentukan bentuk temuan.

## 2. Bukti

Tiga lapis, semuanya bisa diulang dari working tree ini.

### 2.1 Revisi reference — sudah dinaikkan ke terbaru (keputusan owner 2)

Owner memutuskan sasaran adalah **reference terbaru**. Checkout karena itu di-`git pull` pada
2026-09-23, dan seluruh angka di dokumen ini diukur ulang terhadap revisi baru itu.

```bash
cd /home/rusmanadodi/apps/9router
git fetch --all --tags                       # * [new tag] v0.5.85 -> v0.5.85
git pull --ff-only                           # db4499d6 -> 21583c03 (484 commit)
git log -1 --format='%H%n%ci%n%d'            # 21583c03e5c5d5276924efad82328ebe6e215854
                                              # 2026-09-22 15:43:30 +0700
                                              # (HEAD -> master, tag: v0.5.85, origin/master)
git describe --tags                          # v0.5.85
git rev-list --count db4499d6..HEAD          # 484
```

| Revisi | Tanggal | Status |
|---|---|---|
| `db4499d6` | 2026-06-19 | checkout lama; **masih** yang dipin `registry.yaml` yang di-embed |
| `a8c9d380` = v0.5.81 | 2026-09-18 | dipakai draft 011 §2 |
| **`21583c03` = v0.5.85** | **2026-09-22** | **sasaran sekarang** (draft 016 memakai ini; pass ini menariknya ke checkout) |

Dua akibat yang harus diingat saat mengerjakan:

1. `registry.yaml` yang di-embed masih setia pada `db4499d` — **484 commit** dan **27 entri** di
   belakang sasaran. Ini pertanyaan draft 011 §8 yang belum dijawab, dan pass ini tidak menjawabnya:
   F2–F6 dikerjakan terhadap **mekanisme** v0.5.85, sedangkan isi registry tetap menunggu keputusan
   regenerasi.
2. Mekanisme di dalam route `/models` tumbuh hampir dua kali lipat antara kedua revisi, **tanpa satu
   pun route baru** (§2.3).

Jumlah entri registry per revisi (diukur ulang di v0.5.85):

```bash
cd /home/rusmanadodi/apps/9router
ls open-sse/providers/registry/*.js | wc -l                        # 125 berkas
grep -c '^import p' open-sse/providers/registry/index.js           # 121 entri
git ls-tree --name-only db4499d6:open-sse/providers/registry/ | grep -c '\.js$'   # 95

cd /home/rusmanadodi/pannelAI/app-serv
grep -c '^  - id: ' internal/registry/registry.yaml                # 94
grep -c 'hidden: true' internal/registry/registry.yaml             # 10
grep -c 'validate_url' internal/registry/registry.yaml             # 18
grep -c '^      - id: ' internal/registry/registry.yaml            # 507 model
```

Selisih sasaran: **121 − 94 = 27 entri** (draft 011 menyebut 25 terhadap v0.5.81; angka bergerak
karena sasarannya kini v0.5.85).

### 2.1b Angka v0.5.85 yang jadi sasaran F2–F6

Diukur dari working tree yang sudah di-pull. **Catatan metodologis:** dua baris terakhir di bawah ini
menggantikan angka `grep` di versi pertama dokumen, yang **salah** — pola `grep -cE '^  "?[a-z0-9-]+"?: \{'`
hanya cocok untuk entri yang ditulis sebagai objek literal, sementara mayoritas entri dibangun lewat
helper `createOpenAIModelsConfig(url)`. Hitungan yang benar membaca blok `PROVIDER_MODELS_CONFIG`
sampai penutupnya dan mengklasifikasi tiap kunci:

```bash
cd /home/rusmanadodi/apps/9router
# 18 entri objek + 23 entri helper + 2 entri factory = 43
grep -c 'createOpenAIModelsConfig(' src/app/api/providers/\[id\]/models/route.js   # 23
grep -c '^    customResolver:' src/app/api/providers/\[id\]/models/route.js        # 10
grep -c 'customResolver:' src/app/api/providers/\[id\]/models/route.js              # 11
```

| Berkas | `db4499d6` | **v0.5.85** |
|---|---|---|
| `providers/route.js` | 206 | 197 |
| `providers/[id]/models/route.js` | 526 | **678** |
| `providers/validate/route.js` | 622 | 651 |
| `provider-nodes/route.js` | 104 | 104 |
| `provider-nodes/validate/route.js` | 201 | 212 |
| `providers/[id]/test/testUtils.js` | 685 | **914** |
| `PROVIDER_MODELS_CONFIG` entri | 35 | **43** |
| entri yang punya `customResolver` | 5 | **12** (10 inline + 2 lewat factory) |

Rincian v0.5.85: 43 entri = 18 objek literal + 23 lewat `createOpenAIModelsConfig(url)` + 2 lewat
`buildQoderModelsResolver(providerId)`. Dari 43 entri, **12** punya `customResolver` (10 inline +
`qoder`/`qoder-cn` lewat satu factory yang sama, jadi 11 definisi melayani 12 entri). Sisanya
config-driven: satu URL, satu metode, satu penempatan kredensial, satu parser.

### 2.2 Permukaan route reference yang diperiksa

```bash
cd /home/rusmanadodi/apps/9router
find src/app/api/providers src/app/api/provider-nodes -type f -name '*.js' | sort
```

| Route reference | Method | Padanan `app-serv` |
|---|---|---|
| `/api/providers` | GET, POST | `GET /api/v1/providers` (F1: **arti berbeda**) |
| `/api/providers/[id]` | GET, PUT, DELETE | `GET /api/v1/providers/{provider_id}` (baca saja) |
| `/api/providers/[id]/models` | GET | `GET /api/v1/providers/{provider_id}/models` (F2: **kosong untuk node**) |
| `/api/providers/[id]/test` | POST | `POST /api/v1/endpoints/{id}/test` (per-endpoint, bukan per-provider) |
| `/api/providers/[id]/test-models` | POST | **tidak ada** (F10) |
| `/api/providers/test-batch` | POST | **tidak ada** (F10) |
| `/api/providers/validate` | POST | **tidak ada** (F6) |
| `/api/providers/client` | GET | **tidak ada** (F10) |
| `/api/providers/suggested-models` | GET | **tidak ada** (F5) |
| `/api/providers/kilo/free-models` | GET | **tidak ada** (F5) |
| `/api/provider-nodes` | GET, POST | `GET`/`POST /api/v1/provider-nodes` — **1:1** |
| `/api/provider-nodes/[id]` | PUT, DELETE | `PATCH`/`DELETE /api/v1/provider-nodes/{id}` — **1:1** |
| `/api/provider-nodes/validate` | POST | **tidak ada** (F6) |
| `/api/models` | GET, PUT | `GET /api/v1/models/catalog` + `models/aliases` |
| `/api/models/alias` | GET, PUT, **DELETE** | `GET`/`PUT /api/v1/models/aliases` (tanpa DELETE) (F10) |
| `/api/models/custom` | GET, POST, DELETE | **1:1** |
| `/api/models/disabled` | GET, **POST**, **DELETE** | `GET`/`PUT /api/v1/models/disabled` (F10) |
| `/api/models/availability` | GET, POST | **tidak ada** (F10) |
| `/api/models/test` | POST | **tidak ada** (F10) |

### 2.3 Mekanisme `/models` tumbuh tanpa route baru

Route-nya sendiri **tidak berubah** antara kedua revisi — yang tumbuh adalah isi route `/models`:

```bash
git diff --stat db4499d6 21583c03 -- src/app/api/providers src/app/api/provider-nodes
```

```
 src/app/api/provider-nodes/validate/route.js      |  11 +
 src/app/api/providers/[id]/models/route.js        | 282 +++++++++++++++-----
 src/app/api/providers/[id]/test/testUtils.js      | 299 +++++++++++++++++++---
 src/app/api/providers/route.js                    |  21 +-
 src/app/api/providers/suggested-models/filters.js |  11 +-
 src/app/api/providers/validate/route.js           |  43 +++-
 6 files changed, 544 insertions(+), 123 deletions(-)
```

| Berkas | pinned `db4499d6` | **v0.5.85** |
|---|---|---|
| `providers/route.js` | 206 | 197 |
| `providers/[id]/models/route.js` | 526 | 678 |
| `providers/validate/route.js` | 622 | 651 |
| `provider-nodes/validate/route.js` | 201 | 212 |
| `providers/[id]/test/testUtils.js` | 685 | 914 |
| `PROVIDER_MODELS_CONFIG` entri | 35 | **43** |
| entri ber-`customResolver` | 5 | **12** |

(Angka entri dan resolver diukur dengan cara di §2.1b; versi `grep` pertama di dokumen ini salah dan
sudah dikoreksi di sana.)

Artinya: mekanisme "cara mengambil models id" adalah bagian yang paling banyak berubah, dan ia
berubah **tanpa** route baru. Ini menaikkan biaya F2 **bagian B** di atas apa yang draft 011
perkirakan.

## 3. Keputusan yang dibutuhkan owner lebih dulu: arti `/providers`

Satu perbedaan definisi menentukan bentuk temuan yang menyentuh permukaan route (F1, F5, F10), jadi
ia diputuskan sebelum yang lain.

**Fakta.** Reference `GET /api/providers` mengembalikan **koneksi** (akun/credential yang sudah
dikonfigurasi), bukan katalog registry:

```bash
git show 21583c03:src/app/api/providers/route.js | sed -n '51,53p;78,79p'
```

```js
// GET /api/providers - List all connections
export async function GET() {
    const connections = await getProviderConnections();
    // Hide sensitive fields, enrich name for compatible providers
    const safeConnections = connections.map(c => { ... });
    return NextResponse.json({ connections: safeConnections });
```

Sedangkan `app-serv` mengembalikan **registry** (katalog provider bawaan):

```bash
sed -n '78,80p' app-serv/internal/service/provider.go
```

```go
func (s *ProviderService) List(ctx context.Context, filter ProviderFilter, page, perPage int) ([]ProviderRow, int64, error) {
	providers, err := s.entries(filter, true)
```

**Risiko dan aturan.**

- SPEC-API §7.4 sudah menetapkan arti ini secara eksplisit: "Registry list:
  `{id, name, category, auth_type, routability, endpoint_count, status_summary}`". Jadi `app-serv`
  bukan menyimpang dari kontraknya; ia menyimpang dari **reference**.
- SPEC-UI §6.3 dan `app-ui/src/routes/providers/+page.svelte` membaca daftar itu sebagai registry
  (kolom "category, auth type, endpoint count, status summary"). Mengubahnya menjadi daftar koneksi
  akan mematahkan layar Provider dan menduplikasi `/endpoints`, yang di `app-serv` adalah akun.
- Di reference, "Provider" dan "Endpoint" adalah satu benda; di `app-serv` keduanya sengaja dipisah
  (§7.5: satu endpoint memegang 1..N key). Karena itu 1:1 literal di titik ini tidak mungkin tanpa
  membongkar §7.5.

**Tiga pilihan.**

| # | Pilihan | Akibat |
|---|---|---|
| **A** | **Pertahankan arti sekarang** (registry), dan catat sebagai deviasi terkontrak | F2–F10 dikerjakan apa adanya. Tidak ada breaking change. Rekomendasi penulis |
| **B** | Tambah route baru `GET /api/v1/provider-accounts` yang meniru bentuk reference | 1:1 tersedia tanpa mematahkan §7.4; menambah permukaan baru yang harus dirawat |
| **C** | Ubah arti `GET /api/v1/providers` menjadi daftar koneksi | 1:1 literal, tetapi breaking change, mematahkan panel, dan menduplikasi `/endpoints` |

### 3.1 Keputusan owner: A, tetapi tetap 1:1 (2026-09-23)

Owner memilih **A** (pertahankan arti registry) dengan syarat tambahan: **tetap harus 1:1 dengan
REFERENCE**. Dua hal itu bisa dipenuhi bersama, dan bedanya harus dinyatakan supaya tidak jadi klaim
kosong:

- **Yang tetap berbeda dan disengaja:** endpoint-nya. Reference memakai satu route untuk dua benda;
  `app-serv` memisahkannya (§7.4 registry, §7.5 akun). Pemetaan 1:1-nya bukan lewat route yang sama,
  melainkan lewat **kesetaraan kemampuan**: apa pun yang bisa dilakukan reference terhadap sebuah
  "connection", `app-serv` bisa lakukan terhadap sebuah **endpoint**.
- **Yang jadi pekerjaan pass ini:** menutup selisih kemampuan itu, bukan memindahkan route.

**Pemetaan 1:1 yang diukur.** "Connection" di reference = satu baris `providerConnections` yang
menyimpan satu credential. "Endpoint" di `app-serv` = satu baris `upstream_endpoints` yang menyimpan
1..N key. Jadi endpoint adalah **superset**: satu endpoint bisa mewakili satu connection, dan
kemampuan multi-key adalah tambahan, bukan pengurangan.

Daftar field kanonik reference diambil dari `OPTIONAL_FIELDS` di repo-nya
(`src/lib/db/repos/connectionsRepo.js:5-11`) dan dari `SAFE_FIELDS` yang route klien izinkan keluar
(`src/app/api/providers/client/route.js:6-12`), bukan dari tebakan:

```
displayName, email, globalPriority, defaultModel, accessToken, refreshToken, expiresAt,
tokenType, scope, projectId, apiKey, testStatus, lastTested, lastError, lastErrorAt,
rateLimitedUntil, expiresIn, errorCode, consecutiveUseCount, idToken, lastRefreshAt
```

| Kemampuan reference (`/api/providers/[id]`) | `app-serv` | Status |
|---|---|---|
| `GET` satu connection | `GET /endpoints/{id}` | **1:1** |
| `POST` buat connection | `POST /endpoints` (+ `/endpoints/bulk`) | **1:1**, bulk adalah tambahan |
| `PUT` ubah | `PATCH /endpoints/{id}` | **1:1** untuk `label`/`priority`/`status` |
| `DELETE` hapus | `DELETE /endpoints/{id}` | **1:1** |
| `GET` daftar connection | `GET /endpoints` | **1:1** (plus paging + filter yang reference tidak punya) |
| `POST /providers/[id]/test` | `POST /endpoints/{id}/test` | **1:1** |
| `apiKey` | `upstream_keys.value` (AES-GCM, hanya `key_hint` dibaca balik) | **1:1** |
| `accessToken` / `refreshToken` / `expiresAt` / `tokenType` / `scope` / `idToken` / `lastRefreshAt` / `expiresIn` | `oauth` jsonb (redacted saat dibaca) + `oauth_refresh_worker.go` | **1:1** |
| `rateLimitedUntil` | `rate_limited_until` | **1:1** |
| `testStatus` / `lastTested` | `test_status` jsonb | **1:1** |
| `isActive` | `status` ∈ `active\|disabled\|error`; PATCH menerima `active\|disabled`, `error` di-set sistem | **1:1** (tiga-nilai, bukan boolean) |
| `projectId` | `account.project_id` | **1:1** |
| `email` / `displayName` | `account.email` / `account.name` | **1:1** |
| `lastError` / `lastErrorAt` / `errorCode` | `test_status` saja | **selisih**: reference menyimpan error terakhir yang tidak berasal dari test |
| `globalPriority` | — | **selisih**: prioritas lintas-provider; `app-serv` hanya per-provider |
| `defaultModel` | — | **selisih**: model bawaan per akun |
| `consecutiveUseCount` | — | **selisih**: dasar rotasi round-robin di reference |
| `proxyPoolId` (+ `connectionProxyEnabled/Url/NoProxy`) | — | **selisih**: proxy per akun; `app-serv` punya proxy pools di `/proxies` tapi belum terikat ke endpoint |
| Satu connection = satu credential | Endpoint = 1..N key | **superset** (bukan selisih) |

Lima baris "selisih" itu menjadi **F1b** di §4.1b, dan itulah bentuk konkret "tetap 1:1" di bawah
keputusan A: bukan memindahkan `GET /providers`, melainkan membuat endpoint mampu melakukan apa yang
connection reference mampu lakukan.

## 4. Findings

### 4.1 F1 (HIGH) — arti `/providers` berbeda dari reference

**Status: DIJAWAB owner 2026-09-23 — pilihan A, pertahankan registry.**

Bukan cacat, melainkan deviasi yang dispesifikasikan dan sekarang disetujui. Yang hilang bukan
datanya (akun tersedia di `GET /api/v1/endpoints`), melainkan **bentuk** yang reference pakai.
Kesejajaran 1:1 dipenuhi lewat pemetaan kemampuan di §3.1, bukan dengan memindahkan route.

### 4.1b F1b (MEDIUM) — lima field connection reference belum punya padanan di endpoint

**Status: masuk cakupan pass ini** (owner memilih "tetap harus 1:1", §1.1).

**Fakta.** Setelah keputusan A, satu-satunya cara "tetap 1:1" adalah memastikan sebuah endpoint
mampu melakukan apa yang sebuah connection reference mampu lakukan. Lima field belum punya padanan,
dan masing-masingnya bukan kosmetik:

| Field reference | Kenapa berguna | Padanan `app-serv` |
|---|---|---|
| `globalPriority` | Urutan lintas-provider: provider mana yang dicoba lebih dulu saat beberapa punya akun aktif. Tanpa ini, urutan antar-provider tidak bisa dinyatakan. | — |
| `defaultModel` | Model yang dipakai bila request tidak menyebut model. Dipakai juga oleh validate anthropic-compatible reference (`validate/route.js:160`). | — |
| `consecutiveUseCount` | Penghitung pemakaian berurutan; dasar rotasi round-robin dan deteksi akun yang selalu kena rate limit. | — |
| `lastError` / `lastErrorAt` / `errorCode` | Error terakhir yang **bukan** hasil test, mis. penolakan saat request sungguhan. `test_status` hanya menyimpan hasil probe, jadi kegagalan saat routing tidak terlihat di layar akun. | hanya `test_status` |
| `proxyPoolId` (+ `connectionProxyEnabled/Url/NoProxy`) | Proxy per akun: akun yang perlu keluar lewat IP berbeda tidak bisa dinyatakan. `app-serv` sudah punya tabel proxy pools (§7.11) tetapi belum ada kolom yang mengikatnya ke endpoint. | — |

**Risiko dan aturan.**

- AGENTS.md §1.7: kolom baru yang dipakai untuk lookup butuh migration + index, dan migration butuh
  `down` yang bekerja.
- `lastError` menyimpan teks error upstream. SPEC-API §8 dan `docs/RULLES/OWASP.md` A09 melarang
  materi kredensial masuk log/pesan; karena itu kolom ini harus menyimpan **kode + pesan yang sudah
dibersihkan**, bukan body upstream mentah.
- `proxyPoolId` menyentuh jalur egress. Bila dikerjakan, ia wajib lewat `netguard` yang sama (F8),
  karena proxy adalah tujuan yang juga berasal dari operator.

**Rencana DURING.**

1. Test RED lebih dulu untuk setiap field, tabel-driven (TDD.md §2.5).
2. Migration `000012_endpoint_connection_parity` (up+down) menambah kolom ke `upstream_endpoints`:
   `global_priority integer`, `default_model text`, `consecutive_use_count integer NOT NULL DEFAULT 0`,
   `last_error text`, `last_error_at timestamptz`, `error_code text`, `proxy_pool_id text`.
   Index: `(global_priority)` untuk urutan lintas-provider, `(proxy_pool_id)` untuk lookup.
3. `domain.UpstreamEndpoint` — field unexported + method, mengikuti pola aggregate yang ada
   (AGENTS.md §2.2). `consecutive_use_count` naik di choke point yang sama yang menaikkan
   `request_count` (§4), supaya satu request tidak menghitung dua kali.
4. `schema` — `UpdateEndpointRequest` menerima `default_model`, `global_priority`, `proxy_pool_id`;
   `EndpointResponse` menyajikannya. `last_error`/`error_code` dibaca-saja.
5. **Urutan pengerjaan: setelah F8**, karena `proxy_pool_id` menambah jalur egress baru.

**Kriteria selesai.** Lima field punya padanan yang tersaji di `GET /endpoints/{id}` dan bisa diubah
lewat `PATCH` (kecuali yang dibaca-saja); migration punya `down`; `last_error` tidak pernah memuat
kredensial; proxy yang diikat ke endpoint tetap lewat guard.

### 4.2 F2 (HIGH) — node kompatibel tidak punya daftar model sama sekali

**Koreksi atas pembacaan pertama.** Versi awal temuan ini menulis "reference mengambil daftar model
langsung dari upstream" seolah itu berlaku untuk semua provider. Itu **salah**, dan pemeriksaan
ulang menemukan tiga jalur berbeda di reference:

| Kasus | Sumber daftar di reference | Sitasi |
|---|---|---|
| Provider registry, layar detail | **Statis** — `getModelsByProviderId()` → `PROVIDER_MODELS` | `dashboard/providers/[id]/page.js:136`, `open-sse/config/providerModels.js:74-77` |
| Node kompatibel, layar detail | **Live** — `CompatibleModelsSection` → `GET /api/providers/{id}/models` | `page.js:923-925`, `CompatibleModelsSection.js:151` |
| Data plane `GET /api/v1/models` | Statis, **plus** `fetchCompatibleModelIds()` bila node kompatibel dan daftarnya kosong | `api/v1/models/route.js:262`, `:285` |

Jadi untuk provider registry, `app-serv` **sudah 1:1** — keduanya statis dari katalog. Yang menjadi
temuan adalah kasus kedua.

**Ini deferral yang sudah tercatat, bukan penemuan baru.** SPEC-API sudah menyebutnya secara eksplisit,
dan kalimatnya menyebut persis kasus node kompatibel:

> the noAuth virtual connection exists there because the list is connection-gated, which this port's
> registry-driven list never was, and **modelsFetcher (a live upstream /models query) stays
> unported, since dynamic discovery is the compatible-provider behavior PR #4073's third part fixes,
> not a property this list has**.
>
> — SPEC-API §7.15 changelog 2026-09-20

Jadi temuan ini bukan drift yang tak terdeteksi; ia keputusan yang tercatat dan belum ditinjau ulang.
Yang ditambahkan pass ini adalah **akibatnya yang terukur**: selama deferral itu berlaku, node yang
baru dibuat tampil tanpa model di empat permukaan sekaligus (tabel di bawah), dan operator tidak punya
cara melihat model apa yang upstream-nya sebenarnya tawarkan. Keputusan owner di §7 nomor 3 adalah
apakah deferral itu dicabut sekarang.

**Fakta.** Node kompatibel di reference mendapat daftar modelnya dari upstream, dan `app-serv`
mengembalikan **daftar kosong**. Entry yang disintesis tidak pernah diberi model:

```bash
grep -n "Models:" app-serv/internal/registry/custom_node.go     # kosong
sed -n '145,150p' app-serv/internal/registry/custom_node.go     # Provider{...} tanpa Models
```

Diukur dengan menjalankan `WithCustom` yang asli atas registry nyata:

```
node openai-compatible-01TEST resolves, but len(entry.Models) = 0
```

**Bentuk cacatnya: bisa dijawab, tetapi tidak terdaftar.** Node tetap routable — `resolve.go:184`
memperlakukan `entry.Custom` sebagai passthrough, jadi `prefix/model` yang diketik operator tetap
sampai ke upstream. Yang hilang adalah **daftar**:

| Permukaan | Isi untuk node | Sitasi |
|---|---|---|
| `GET /providers/{id}/models` | `{"data":[]}` | `internal/schema/provider.go:173` |
| `GET /providers/{id}` → `model_count` | `0`, `chat_model_count` `0` | `:162-163` |
| `GET /models/catalog?provider_id=<node>` | tidak memuat model node | `internal/service/model_catalog.go:144-159` |
| `GET /api/v1/models` (data plane) | tidak memuat model node | `internal/dataplane/catalog.go:51-60` |

Properti "listed and answerable are one property" adalah yang `internal/dataplane/catalog.go:12-14`
nyatakan sebagai alasan berkas itu ada, dan untuk node properti itu pecah. Ini juga pola yang sama
yang SPEC-API catat untuk OpenCode: "the provider's free models answered any request through
passthrough yet appeared in no models list".

Akibat praktisnya: operator yang baru membuat node OpenAI-compatible melihat node-nya berdiri tanpa
satu pun model yang bisa dipilih di panel, dan katalog tidak menawarkannya. Satu-satunya jalan hari
ini adalah mengetik `prefix/model` dari ingatan, atau menambah barisnya sendiri lewat
`POST /models/custom` — yang berarti menyalin daftar yang upstream sudah punya.

**Tabel konfigurasi itu nyata, dan ia menunjukkan biaya sesungguhnya.** Reference menempatkan
kredensial secara berbeda per provider — bukan satu jalur seragam:

| Provider | Penempatan | Sitasi |
|---|---|---|
| `claude`/`anthropic` | `x-api-key`, `Anthropic-Version: 2023-06-01` | `models/route.js:182`, `:179` |
| `gemini` | **query** `?key=` | `:189` |
| OpenAI-compatible | `Authorization: Bearer` | `:79-80` |
| `github` | 5 header termasuk `Copilot-Integration-Id` | `:217-229` |
| `antigravity` | `POST` dengan body `{}` | `:210`, `:215` |
| `kiro`, `qoder`, `qoder-cn`, `gemini-cli`, `ollama-local`, `codex`, `cursor`, `zed`, `cline`, `clinepass`, `kimchi`, `grok-cli` | `customResolver` (12 entri, 11 definisi) | `:135`, `:193`, `:306-530` |

`app-serv` tidak bisa menyatakan satu pun dari itu: `AuthConfig` hanya punya `Header` dan `Scheme`
(`internal/registry/types_transport.go:64-71`), jadi provider yang menaruh kredensial di query
tidak bisa dinyatakan sama sekali.

**Akibat kedua yang terukur.** `Transport.ValidateURL` hanya dideklarasikan 18 dari 94 entri:

```bash
grep -c 'validate_url' app-serv/internal/registry/registry.yaml   # 18
grep -c '^  - id: ' app-serv/internal/registry/registry.yaml      # 94
```

Reference menulis URL validasi itu di dalam route, bukan di registry
(`providers/validate/route.js` berisi switch panjang dengan URL per provider). Jadi 76 provider yang
probe-nya hari ini menjawab "this provider does not declare a validation endpoint"
(`cmd/app-serv/provider_probe.go:93-95`) kehilangan kemampuan itu karena dokumennya tidak memuat
URL-nya — bukan karena providernya tidak bisa.

**Rencana DURING.** Dua bagian, dan yang pertama berdiri sendiri:

*Bagian A — node kompatibel punya daftar model (menutup cacat).*

1. `schema` — DTO hasil fetch: `{provider, models: [{id, name}], source: "upstream"|"registry", warning?}`. `source` wajib ada supaya panel bisa membedakan jawaban live dari jawaban cadangan.
2. `service` — port `ModelLister` + implementasi HTTP di `cmd/app-serv/`, mengikuti pola `EndpointProber`/`NodeProber`: port di `service`, adapter di composition root, sehingga `internal/service` tetap bebas `net/http` (AGENTS.md §1.5).
3. **Fallback wajib**: upstream gagal → jawab daftar registry dengan `source: "registry"` + `warning`, jangan 5xx. Reference melakukan ini (`models/route.js:118-121`). Tanpa ini, node yang upstream-nya sedang mati kehilangan seluruh daftar modelnya.
4. **Egress guard wajib** (F8): dial lewat `netguard.Guard`, sama seperti probe.

*Bagian B — tabel penempatan kredensial per provider (menutup 76 provider tanpa validate URL).*

5. `registry` — tambah `auth_query` ke `Transport` (untuk gemini) dan tabel penempatan kredensial per **format**, bukan per provider id, supaya satu aturan melayani satu keluarga.
6. Isi `validate_url` untuk entri yang reference sudah punya URL-nya, lewat generator — bukan tambalan tangan, karena header `registry.yaml` melarangnya.

**Kriteria selesai.** Node OpenAI-compatible yang dibuat lewat `POST /provider-nodes` muncul di
`GET /providers/{id}/models`, di `GET /models/catalog?provider_id=<node>`, dan di data plane
`GET /api/v1/models`; `source` menyatakan asal jawaban; upstream yang mati menghasilkan daftar
cadangan, bukan 5xx; dan tidak ada `net/http` yang diimpor `internal/service`.

### 4.3 F3 (HIGH) — `base_url` tidak disanitasi, dan ini cacat yang terukur

**Fakta.** Reference menyaring `base_url` node pada **dua** jalur (create dan update): buang `/` di
akhir, lalu buang `/messages` (Anthropic) atau `/embeddings` (custom-embedding) di akhir:

```bash
git show 21583c03:src/app/api/provider-nodes/route.js | sed -n '81,95p'
git show 21583c03:src/app/api/provider-nodes/\[id\]/route.js | sed -n '36,48p'
```

```js
let sanitizedBaseUrl = (baseUrl || ANTHROPIC_COMPATIBLE_DEFAULTS.baseUrl).trim().replace(/\/$/, "");
if (sanitizedBaseUrl.endsWith("/messages")) {
  sanitizedBaseUrl = sanitizedBaseUrl.slice(0, -9); // remove /messages
}
```

`app-serv` tidak menyaring apa pun. `validateNodeBaseURL` hanya memeriksa skema dan host, lalu
menyimpan apa adanya (`internal/domain/provider_node.go:231-247`), dan `chatPath()` **menambahkan**
`/messages` (`internal/registry/custom_node.go:70-79`).

**Dibuktikan** dengan menjalankan `joinPath` yang asli (`internal/provider/default.go:117`):

```
base=https://api.anthropic.com/v1/messages      path=/messages   -> https://api.anthropic.com/v1/messages/messages
base=https://api.anthropic.com/v1/              path=/messages   -> https://api.anthropic.com/v1/messages
base=https://api.anthropic.com/v1               path=/messages   -> https://api.anthropic.com/v1/messages
```

Baris pertama adalah cacatnya: operator yang menempel URL lengkap dari dokumentasi vendor — hal yang
paling wajar dilakukan — mendapat 404 dari upstream, dan pesan yang muncul menunjuk ke kredensial
atau model, bukan ke URL.

`joinPath` sudah benar untuk garis miring ganda; yang hilang adalah sanitasi **sufiks**, bukan
pemisah.

**Rencana DURING.** Sanitasi di domain, bukan di handler, supaya `POST` dan `PATCH` tidak bisa
berbeda (reference menyaring di dua tempat dan itulah yang membuatnya mudah lepas):

1. `domain.ProviderNode` — tambah penyaring sufiks di `validateNodeBaseURL`/`Rebase`: buang `/`
   berlebih di akhir, lalu buang sufiks per tipe (`/messages` untuk Anthropic-compatible,
   `/embeddings` untuk custom-embedding bila F7 dikerjakan).
2. Simpan nilai yang sudah bersih, sehingga baris lama dan baru punya bentuk yang sama.
3. Test tabel (TDD.md §2.5) minimal 5 variasi: dengan `/`, tanpa `/`, dengan `/messages`, dengan
   `/messages/`, dan URL polos yang **tidak boleh** berubah.
4. Karena `ProbeNode` menyusun targetnya sendiri (`node.BaseURL() + "/models"`,
   `cmd/app-serv/provider_probe.go:144`), ia ikut memakai perbaikan ini — dan garis miring gandanya
   hilang sekalian.

**Kriteria selesai.** Node yang dibuat dengan `https://host/v1/messages` mengirim ke
`https://host/v1/messages`, bukan `.../messages/messages`; node dengan URL polos tidak berubah.

**Status: DURING selesai 2026-09-23.**

Yang dikerjakan:

| Berkas | Isi |
|---|---|
| `internal/domain/provider_node_url.go` (baru, 116) | `normalizeNodeBaseURL` + `nodeAppendedSuffix`; `validateNodeBaseURL` pindah ke sini karena keduanya aturan URL |
| `internal/domain/provider_node_validate.go` (baru, 88) | `validateNodePrefix`, `nodeID`, `validateNodeAPIType` — dipecah dari aggregate karena file itu menembus 220 |
| `internal/domain/provider_node.go` (165, tadinya 227) | constructor memanggil `normalizeNodeBaseURL`; `Rebase` juga |
| `internal/domain/provider_node_suffix_test.go` (baru, 129) | tabel create-path, 10 variasi |
| `internal/domain/provider_node_compose_test.go` (baru, 124) | tabel mutation-path + property komposisi, 6+5 variasi |
| `cmd/app-serv/provider_probe_node_test.go` (baru, 167) | `TestProbeNode_ComposesOneModelsPath`, 5 variasi |
| `cmd/app-serv/provider_probe_fixture_test.go` (baru, 63) | fixture bersama |
| `cmd/app-serv/provider_probe_test.go` (217, tadinya 389) | dipecah; **file ini sudah melewati batas 250 sebelum pass ini** |

**Dua keputusan implementasi yang menyimpang dari rencana, dan alasannya:**

1. **Sufiks di-strip dari `parsed.Path`, bukan dari string mentah.** Rencana awal menulis "buang `/`
   berlebih di akhir, lalu buang sufiks". Kalau itu dilakukan pada string, host yang kebetulan
   mengeja sufiksnya — `https://messages` — akan kehilangan host-nya. Bekerja pada `parsed.Path`
   membuat kasus itu tidak mungkin, dan query string ikut selamat tanpa penanganan khusus.
2. **Strip diulang sampai stabil, bukan sekali.** Operator yang sudah menempel URL yang terlanjur
   ganda (`…/v1/messages/messages`) diperbaiki ke `…/v1`, bukan ke URL lain yang masih salah.

**Yang sengaja TIDAK dilakukan.** `/embeddings` tidak ikut di-strip, karena `custom-embedding`
belum ada di `app-serv` (F7). `nodeAppendedSuffix` mengembalikan `""` untuk tipe yang tidak dikenal,
sehingga menambahkan tipe ketiga nanti hanya perlu satu baris di sana — dan sampai itu terjadi, tidak
ada sufiks yang di-strip atas nama node yang tidak ada.

**Bukti gate.** `go build`, `go vet`, `gofmt`, `staticcheck`, `golangci-lint` (0 issues),
`go-headers.sh` (753 berkas) semuanya lulus. `scrypts/gates/go-lint.sh` melaporkan **nol** peringatan
batas baris — dua berkas yang sempat masuk band 220–250 sudah dipecah.

**Cacat pre-existing yang ikut ditutup.** `cmd/app-serv/provider_probe_test.go` sudah 389 baris di
`HEAD` (melewati batas 250) sebelum pass ini menyentuhnya. Karena pass ini menambah test di file itu,
file tersebut dipecah menjadi tiga: suite endpoint, suite node, dan fixture bersama.

### 4.4 F4 (HIGH) — dua filter capability yang panel tawarkan mengembalikan nol baris

**Fakta.** SPEC-API §7.6 menjanjikan `?capability=vision|tools`. Panel menawarkan tepat dua nilai itu:

```bash
grep -n "CATALOG_CAPABILITY_FILTERS" app-ui/src/lib/schemas/model.ts
# 57: export const CATALOG_CAPABILITY_FILTERS = ['vision', 'tools'] as const;
```

Filter itu mencocokkan `registry.yaml` apa adanya (`internal/service/model_catalog.go:170-172`), dan
`registry.yaml` hanya memuat capability media:
```bash
python3 -c "..." # unique capabilities in registry.yaml
# ['edit', 'mask', 'text2img']
```

**Diukur** terhadap registry yang di-embed, bukan terhadap fixture:

```
capability=vision    -> 0 models in the REAL registry
capability=tools     -> 0 models in the REAL registry
capability=edit      -> 7 models in the REAL registry
capability=text2img  -> 3 models in the REAL registry
capability=mask      -> 1 models in the REAL registry
```

Jadi kedua tombol filter yang panel render tidak pernah bisa mengembalikan satu baris pun, dari 507
model yang terdaftar (`grep -c "^      - id: " registry.yaml` → 507). Test yang ada lulus karena
memakai fixture yang menuliskan `vision` langsung di datanya
(`internal/service/model_catalog_test.go:86`, `internal/handler/model_test.go:50`) — inilah persis
pola yang `docs/RULLES/TDD.md` §2.1 larang: fixture yang membuktikan dirinya sendiri.

**Akar masalahnya: dua sumber kebenaran untuk satu kata.** `app-serv` sudah punya tabel capability,
tetapi hanya separuh dan hanya untuk satu pemakai:

| Pemakai | Sumber | Cakupan |
|---|---|---|
| `GET /models/catalog?capability=` | `registry.yaml` `capabilities:` | 3 nilai media, 507 model |
| Vision adapter | `registry.VisionCapable()` (`capability.go`, 160 baris) | pola model id |

Reference punya **satu** sumber untuk keduanya: `open-sse/providers/capabilities.js` (605 baris)
dengan empat lapis resolusi — `PROVIDER_CAPABILITIES`, `MODEL_CAPABILITIES` (exact id),
`PATTERN_CAPABILITIES` (glob, ordered specific→generic), lalu `DEFAULT_CAPABILITIES` sebagai lantai
aman, ditambah katalog models.dev dan `visionPatterns.js`.

**Rencana DURING.**

1. **RED lebih dulu**: test yang membaca registry nyata dan menuntut `capability=vision` mengembalikan
   baris, bukan fixture. Test ini gagal hari ini.
2. Port `capabilities.js` menjadi tabel Go di `internal/registry/`, dengan urutan resolusi yang sama
   (exact → pattern → default). `capability.go` yang ada menjadi **pemakai** tabel itu, bukan tabel
   kedua.
3. Catalog mengisi `Capabilities` dari resolver itu untuk setiap model registry, bukan hanya dari
   `registry.yaml`. Capability media (`edit`, `mask`, `text2img`) tetap dibaca dari YAML karena itu
   memang data dokumen.
4. Jangan tulis ulang 605 baris apa adanya: port **keputusannya**, bukan bentuknya — mengikuti
   preseden yang sudah dipakai `capability.go:19-27`, dan pin hasilnya dengan test terhadap korpus
   yang dihasilkan dari `capabilities.js` sendiri.

**Kriteria selesai.** `?capability=vision` dan `?capability=tools` mengembalikan baris dari registry
nyata; `capability.go` tidak lagi punya tabel pola sendiri; ada satu test yang membaca registry
di-embed, bukan fixture.

### 4.5 F5 (MEDIUM) — `?suggested=true` bukan parameter reference, dan implementasinya kosong

**Fakta.** SPEC-API §7.4 menulis "`?suggested=true` returns suggested set", dan handler `app-serv`
menerima parameter itu sebagai no-op:

```bash
sed -n '83,88p' app-serv/internal/handler/provider.go
sed -n '181,183p' app-serv/internal/schema/provider.go   # Suggested: true
```

Tetapi reference **tidak punya** parameter itu di route mana pun:

```bash
grep -rn "suggested" src/app/api/ --include=*.js | grep -v suggested-models   # kosong
grep -n "searchParams" src/app/api/providers/\[id\]/models/route.js          # kosong
```

Yang reference punya adalah mekanisme lain dengan nama yang mirip: `modelsFetcher` pada 4 entri
registry, disajikan lewat `GET /api/providers/suggested-models?url=&type=` + `filters.js`
(`openrouter-free`, `opencode-free`, `mimo-free`), dan dikonsumsi panel di
`dashboard/providers/[id]/page.js:401-403` sebagai daftar saran terpisah dari daftar model.

Panel `app-ui` sudah mencatat keadaan ini dengan jujur
(`app-ui/src/lib/schemas/model.ts:7-8`, `ModelCatalogList.svelte:6`): toggle "suggested" tidak
dibangun karena registry melaporkan semuanya `true`.

**Rencana DURING.** Perbaiki **kontraknya**, bukan kodenya: `Suggested: true` yang konstan adalah
klaim yang tidak diukur siapa pun. Dua langkah, dan langkah pertama tidak butuh F2:

1. Hapus `?suggested` dari SPEC-API §7.4 bila owner memilih A di §3, atau ganti dengan bentuk
   reference (`modelsFetcher` + route `suggested-models`) bila memilih B. `Suggested: true` konstan
   dihapus dari wire — field yang selalu sama tidak membawa informasi dan menyesatkan pembaca.
2. `modelsFetcher` untuk 4 entri itu ditambahkan ke `registry.yaml` lewat generator (saat ini
   generator sengaja tidak menulisnya, dan menyatakan alasannya di `tools/registry-gen.mjs:20-22`
   dan `:155-157`), dan route penyajinya mengikuti pola F2.

**Kriteria selesai.** Tidak ada field wire yang nilainya konstan; SPEC-API §7.4 tidak menyebut
parameter yang tidak ada di reference tanpa alasan tertulis.

### 4.6 F6 (MEDIUM) — tidak ada validasi kredensial sebelum baris disimpan

**Fakta.** Reference memvalidasi kredensial **sebelum** menyimpan, di dua route terpisah:

- `POST /api/providers/validate` (651 baris di master) — per provider, dengan cabang khusus per
  keluarga: web provider, media provider, `cloudflare-ai`, `azure`, anthropic-compatible, dll.
- `POST /api/provider-nodes/validate` (212 baris di master) — per base URL, dengan fallback chat.

`app-serv` hanya bisa menguji **sesudah** baris ada (`POST /api/v1/endpoints/{id}/test`), dan tidak
punya route validate sama sekali:

```bash
grep -n "validate" app-serv/internal/router/router.go    # kosong
```

**Dua perilaku reference yang mudah terlewat, dan keduanya bukan detail:**

1. **Fallback chat ketika `/models` bukan 2xx.** Kalau `/models` menjawab 404, reference mencoba
   `/chat/completions` dengan `max_tokens: 1` — karena banyak server kompatibel tidak menyediakan
   `/models` tetapi tetap bisa melayani chat. `app-serv` hanya tahu `/models`
   (`provider_probe.go:144`), jadi server seperti itu selalu dilaporkan gagal.
2. **400/529 berarti kredensial diterima.** Untuk anthropic-compatible, reference menganggap
   `status !== 401 && status !== 403` sebagai valid (`providers/validate/route.js:178`) — body yang
   ditolak karena `max_tokens` atau model tidak dikenal tetap membuktikan kuncinya diterima. Ini
   pembedaan yang benar antara "kunci salah" dan "permintaan salah", dan `app-serv` belum
   memisahkannya.

**Rencana DURING.**

1. Route baru `POST /api/v1/provider-nodes/validate` (tanpa id — node belum ada) dan
   `POST /api/v1/providers/validate`, keduanya **stateless**: body membawa `base_url`/`provider_id` +
   kredensial, tidak ada baris yang ditulis.
2. Perluas `ProbeOutcome` (`internal/service/endpoint_probe.go:40-58`) dengan `Method` supaya jawaban
   bisa menyebut jalur mana yang membuktikan (`models` atau `chat`) — reference sudah melakukan ini.
3. Fallback `/models` → `/chat/completions` untuk node kompatibel, dan aturan
   `401/403 = kunci salah, lainnya = kunci benar` untuk Anthropic-compatible.
4. Kredensial di body tidak boleh masuk log maupun pesan error (AGENTS.md §1.3, OWASP A09).

**Kriteria selesai.** Operator bisa membuktikan sebuah kredensial sebelum menyimpannya; jawabannya
menyebut jalur yang dipakai; tidak ada baris DB yang ditulis oleh route validate.

### 4.7 F7 (MEDIUM) — tipe node `custom-embedding` tidak ada

**Fakta.** Reference punya **tiga** tipe node:

```bash
git show 21583c03:src/app/api/provider-nodes/route.js | grep -n "nodeType ===\|_PREFIX" | head
```

| Tipe | Id prefix | Default base URL |
|---|---|---|
| `openai-compatible` | `openai-compatible-` | `https://api.openai.com/v1` |
| `anthropic-compatible` | `anthropic-compatible-` | `https://api.anthropic.com/v1` |
| `custom-embedding` | `custom-embedding-` | `https://api.openai.com/v1` |

`app-serv` punya dua. `custom-embedding` muncul **hanya sebagai kasus penolakan** di test:

```bash
grep -n "custom-embedding" app-serv/internal/domain/provider_node_test.go
# 95:  nodeType: NodeType("custom-embedding"), apiType: NodeAPIChat, baseURL: "https://p.test/v1", wantErr: true,
```

Artinya perilaku hari ini terkunci oleh test sebagai "ditolak", dan itu konsisten dengan SPEC-API
§7.4 yang hanya menyebut dua tipe. Jadi ini **bukan cacat**, melainkan fitur yang belum
dispesifikasikan.

**Biaya kalau dikerjakan.** Bukan satu `const`. Menyentuh: `domain.NodeType` + `validateNodeAPIType`
(embedding tidak punya `api_type`), `registry.CustomNode.format()`/`chatPath()` (tidak ada
`chat_path` — embedding memakai `/embeddings`), **CHECK constraint di migration** (lihat di bawah),
`NodeService.List` filter, dan `ProbeNode` yang hari ini selalu menempel `/models`.

```sql
-- migrations/000004_provider_nodes.up.sql:16-17
CONSTRAINT provider_nodes_type_check
    CHECK (type IN ('openai-compatible', 'anthropic-compatible'))
```

Constraint itu adalah perubahan migration `00000N`, dan AGENTS.md §1.7 mewajibkan migration itu
punya `down` yang bekerja.

**Rencana DURING.** Kerjakan hanya bila owner memutuskan ya di §7. Urutan: test RED → domain →
migration (up+down) → registry synthesis → probe (POST `/embeddings` dengan `input: "ping"`, sesuai
`provider-nodes/validate/route.js:80-96`) → handler.

**Kriteria selesai.** Tipe ketiga bisa dibuat, disintesis menjadi provider yang routable untuk
embeddings, dan probe-nya memakai `/embeddings` bukan `/models`.

### 4.8 F8 (MEDIUM) — guard SSRF hanya terpasang di jalur probe

**Fakta.** `app-serv` sudah punya guard yang kuat, dan lebih ketat daripada reference:

| Lapis | `app-serv` (`internal/netguard/`) | Reference |
|---|---|---|
| Tier 1 — tidak pernah jadi host | `hardDeniedPrefixes`: `0.0.0.0/8`, CGNAT `100.64.0.0/10` (tempat metadata Alibaba), `192.0.0.0/24`, broadcast, `240.0.0.0/4` + link-local/multicast/unspecified | tidak ada tier tetap |
| Tier 2 — butuh allowlist | `needsAllowlist()` + `EGRESS_ALLOWED_TARGETS` | tidak ada |
| Anti-rebinding | `Control` hook memvalidasi IP **yang benar-benar didial**, setelah resolusi | tidak ada |
| Semua jawaban DNS diperiksa | `CheckHost` gagal-tertutup bila satu alamat ditolak | tidak ada |

Reference baru menambahkan penjaganya di `origin/master`, dan hanya satu baris:

```bash
git diff db4499d6 21583c03 -- src/app/api/provider-nodes/validate/route.js
# + import { assertPublicUrl } from "@/shared/utils/ssrfGuard.js";
# + import { isLocalRequest } from "@/dashboardGuard";
# + if (!isLocalRequest(request)) { try { assertPublicUrl(baseUrl); } catch { return 400 } }
```

**Yang menjadi temuan.** Guard `app-serv` hari ini hanya dipasang di satu jalur — prober
(`cmd/app-serv/provider_probe.go:76`: `guard.NewDialer(...)`). F2 (daftar model node dari upstream)
dan F6 (validate) **sama-sama mendial URL yang berasal dari operator**, jadi keduanya wajib lewat
guard yang sama. Kalau tidak, F2 menjadi satu-satunya jalur keluar yang tidak diperiksa, dan itu
persis pola yang `docs/RULLES/OWASP.md` A01 larang: mitigasi yang menutup satu jalur sambil
meninggalkan kelas serangannya terbuka di jalur lain.

**Rencana DURING.**

1. Setiap adapter baru di `cmd/app-serv/` yang mendial URL operator **wajib** menerima
   `*netguard.Guard` dan memakai `guard.NewDialer(...)`, seperti `newHTTPEndpointProber`.
2. Tambahkan assertion struktural yang gagal bila sebuah adapter egress dibangun tanpa guard —
   supaya aturannya tidak bergantung pada review.
3. Test tabel (OWASP §2.5) minimal 5 variasi yang mencakup **satu kasus sah yang harus tetap
   diizinkan**, supaya perbaikannya tidak berubah menjadi over-blocking: URL publik lolos,
   `169.254.169.254` ditolak, `100.100.100.200` ditolak, `localhost`/`127.0.0.1` ditolak,
   `::ffff:127.0.0.1` ditolak.

**Kriteria selesai.** Tidak ada jalur egress baru yang bisa dibangun tanpa guard; test-nya
tabel-driven dan memuat kasus yang harus lolos.

### 4.9 F9 (LOW) — bentuk amplop dan bentuk id node berbeda

**Fakta.** Reference menjawab daftar model dengan amplop yang menyebut asalnya:

```js
// providers/[id]/models/route.js:558-561
return NextResponse.json({ provider: connection.provider, connectionId: connection.id, models });
```

`app-serv` menjawab `{"data": [...]}` (`internal/schema/provider.go:112-115`). Tidak ada yang salah
dengan `{"data":...}` — itu konsisten dengan sisa API `app-serv` — tetapi `provider`/`connectionId`
di reference berguna justru saat F2 dikerjakan: jawaban bisa datang dari upstream **atau** registry,
dan amplop yang menyebut asalnya membuat panel tidak perlu menebak.

Bentuk id node juga berbeda: reference menyisipkan `apiType` ke dalam id
(`openai-compatible-chat-<id>`), `app-serv` tidak (`openai-compatible-<ULID>`,
`internal/domain/provider_node.go:201`). Karena `format()` sudah membaca `api_type` dari kolomnya
sendiri (`internal/registry/custom_node.go:55-63`), bentuk `app-serv` konsisten dan tidak perlu
ubah — dicatat supaya tidak dikira drift.

**Rencana DURING.** Tambahkan `source` pada amplop model (bagian dari F2 bagian A langkah 1). Jangan
ubah bentuk id node.

### 4.10 F10 (LOW) — route siklus hidup model yang belum ada

**Fakta.** Reference punya enam route yang `app-serv` belum punya:

| Route | Kegunaan | Padanan terdekat |
|---|---|---|
| `POST /api/models/test` | ping satu model | tidak ada |
| `GET/POST /api/models/availability` | model yang sedang cooldown karena error | tidak ada |
| `DELETE /api/models/alias` | hapus satu alias | hanya `PUT` (replace seluruh himpunan) |
| `POST/DELETE /api/models/disabled` | tambah/hapus satu | hanya `PUT` (replace seluruh himpunan) |
| `POST /api/providers/[id]/test-models` | uji semua model satu provider | tidak ada |
| `POST /api/providers/test-batch` | uji banyak koneksi per grup | tidak ada |

Pola `PUT`-replace di `app-serv` adalah pilihan §7.6 yang sadar dan lebih mudah diaudit; yang hilang
adalah operasi satu-item yang lebih murah untuk panel. `/models/availability` tidak punya padanan
sama sekali, dan itulah satu-satunya di daftar ini yang menutup pertanyaan yang belum bisa dijawab
panel: "model mana yang sedang di-cooldown, dan sampai kapan".

**Rencana DURING.** Kerjakan `/models/availability` lebih dulu (nilainya paling jelas dan datanya
sudah ada di `upstream_key_health.go`), lalu `/models/test`. Empat sisanya menunggu keputusan owner.
## 5. Non-findings

Dicatat supaya tidak dikira belum diperiksa.

| Hal | Kenapa bukan temuan |
|---|---|
| `DELETE /provider-nodes/{id}` menolak `CONFLICT` di `app-serv`, sedangkan reference meng-cascade koneksinya | `app-serv` lebih ketat dan itu pilihan §7.4 yang tertulis. Menghapus node yang masih dipakai akan meninggalkan endpoint yang menunjuk provider tidak ada |
| `PATCH` vs `PUT` pada node | §7.4 memilih `PATCH`. Bentuk partial lebih benar untuk patch sebagian, dan itu yang handler pakai (`UpdateProviderNodeRequest` semua pointer) |
| Satu koneksi per node di revisi yang dipin | Reference `origin/master` sudah **mengizinkannya banyak** untuk node LLM ("Compatible LLM nodes support multiple API-key connections (key pool)", `providers/route.js:129`). Model endpoint `app-serv` (1..N key per endpoint) sudah merupakan perilaku master, jadi tidak ada yang tertinggal |
| `auth_modes` tidak ditegakkan | Reference master menambahkan `supportsApiKeyMode` untuk provider dual-auth (`providers/route.js:107`). `app-serv` membawa `auth_modes` sebagai data (`xai` satu-satunya dual-auth) tetapi belum memakainya untuk memvalidasi `auth_type` saat membuat endpoint. Ini longgar, bukan salah — dicatat di sini, dinaikkan bila owner mau |
| `transport.force_stream` dan `thinking_format` inert | Sudah dicatat draft 011 §2 sebagai deklarasi tanpa pembaca runtime. Bukan temuan baru pass ini |
| `/api/providers/client` | Itu route untuk klien eksternal dengan penyaring field aman (`SAFE_FIELDS`/`SAFE_PSD_FIELDS`) dan paging. Berguna, tetapi bukan bagian permukaan `/provider` yang owner minta |

## 6. Urutan pengerjaan

Keputusan owner (§7.1) menjadikan cakupan pass ini: **F2 (A+B), F3, F4, F5, F6, dan F1b**. Urutannya
dipilih supaya cacat ditutup lebih dulu, dan supaya guard mendarat **sebelum** jalur egress baru:

| # | Temuan | Alasan urutan | Perkiraan |
|---|---|---|---|
| 1 | **F3** sanitasi `base_url` | Cacat terukur, perbaikannya di domain, tidak bergantung apa pun | Kecil |
| 2 | **F4** capability | Cacat terukur, test RED-nya jelas, menyatukan dua sumber kebenaran | Sedang |
| 3 | **F8** guard di jalur baru | Wajib mendarat **sebelum** F2/F6/F1b, karena ketiganya menambah jalur egress | Kecil |
| 4 | **F2 bagian A** daftar model node | Menutup cacat "node tanpa model"; satu port baru + fallback, tanpa tabel kredensial | Sedang |
| 5 | **F6** validate | Stateless, memakai ulang `ProbeOutcome` + prober yang ada | Sedang |
| 6 | **F1b** lima field connection | Migration + domain + schema; `proxy_pool_id` butuh F8 selesai | Sedang |
| 7 | **F2 bagian B** tabel kredensial + `validate_url` | Bagian terbesar: menyentuh generator, 76 entri, dan `AuthConfig` | Besar |
| 8 | **F5** `?suggested` | Setelah F2, karena bentuknya bergantung pada hasilnya | Kecil |

F2 dipecah karena kedua bagiannya punya nilai dan risiko yang berbeda: bagian A menutup cacat yang
terlihat operator setiap hari, sedangkan bagian B menyentuh berkas yang di-generate dan 76 entri
sekaligus. Bagian A bisa mendarat tanpa bagian B.

Di luar cakupan pass ini: **F1** (sudah dijawab), **F7** dan **F9/F10** (menunggu §7.2).

## 7. Keputusan owner

### 7.1 Sudah dijawab (2026-09-23)

| # | Pertanyaan | Jawaban | Akibat |
|---|---|---|---|
| 1 | Arti `/providers` | **A — pertahankan registry**, tetap 1:1 lewat pemetaan kemampuan | §3.1; melahirkan F1b |
| 2 | Revisi reference sasaran | **Terbaru** — `git pull` | Checkout naik ke `21583c03`/v0.5.85; semua angka diukur ulang (§2.1) |
| 3 | Cakupan | **Semua: F2–F6** | F2 (A+B), F3, F4, F5, F6 masuk pass ini; F1b ikut karena keputusan 1 |

### 7.2 Masih terbuka

1. **F7 `custom-embedding`**: dikerjakan (butuh migration `00000N` + `down`) atau tetap ditolak
   seperti sekarang? Ini satu-satunya tipe node reference yang belum ada di `app-serv`.
2. **F10**: `/models/availability` dan `/models/test` masuk pass ini, atau dicatat saja? Empat route
   siklus hidup lainnya (`DELETE /models/alias`, `POST/DELETE /models/disabled`,
   `POST /providers/test-batch`, `POST /providers/[id]/test-models`) juga menunggu.
3. **Regenerasi registry**: `registry.yaml` masih di `db4499d` (94 entri) sementara sasaran v0.5.85
   punya 121. F2–F6 dikerjakan terhadap **mekanisme** v0.5.85, jadi keputusan ini tidak memblokir
   pass ini — tetapi ia tetap terbuka, dan draft 011 §8 sudah menanyakannya lebih dulu.

Tidak ada source `app-serv` yang diubah sebelum §7.1 dijawab; §7.1 sudah dijawab, jadi F2–F6 dan F1b
boleh dikerjakan.

## 8. Status per 2026-09-23

- **CLOSED untuk scope pass ini** — lihat §9 untuk hasil per temuan, dua koreksi pembacaan, satu
  deferral yang dinyatakan, dan angka gerbang. §7.2 tetap terbuka dan tidak memblokir.
- **Keputusan owner §7.1 sudah dijawab** (A + reference terbaru + cakupan F2–F6); implementasi boleh
  mulai. §7.2 masih terbuka dan tidak memblokir.
- Reference sudah di-`git pull` ke **`21583c03` = v0.5.85 (2026-09-22)**, dan seluruh angka di dokumen
  ini diukur ulang terhadap revisi itu (§2.1, §2.1b).
- Baseline terukur sebelum audit: `go build ./...` bersih, `go vet ./...` bersih,
  907 fungsi test di `app-serv`.
- Tiga cacat dibuktikan dengan menjalankan kode aslinya, bukan dengan membaca:
  F2 lewat `WithCustom` atas registry nyata, F3 lewat `joinPath`, F4 lewat pembacaan registry yang
  di-embed. Ketiganya bisa diulang dari §4.2, §4.3, dan §4.4.
- **Dua koreksi pembacaan dicatat, bukan dihapus.**
  1. Versi pertama F2 menyimpulkan "reference selalu mengambil daftar model dari upstream". Itu
     keliru: untuk provider registry, reference membaca katalog statis (`page.js:136`), sama seperti
     `app-serv`. Setelah dikoreksi, cakupan temuan menyempit ke node kompatibel — dan justru di situ
     cacatnya nyata (`len(entry.Models) = 0`).
  2. Versi pertama §2.1b mencatat `PROVIDER_MODELS_CONFIG` 19 entri dan `customResolver` 11, hasil
     `grep` yang hanya cocok untuk entri berbentuk objek literal. Hitungan yang benar adalah
     **43 entri** (18 objek + 23 helper + 2 factory) dengan **12 entri** ber-`customResolver`.
     Kesalahan ini penting karena ia menyembunyikan bahwa mayoritas entri dibangun lewat helper —
     yang justru membuat tabel penempatan kredensial di F2 bagian B lebih masuk akal daripada 43
     blok terpisah.
- Draft 011 masih memegang pertanyaan regenerasi registry yang sama; draft ini tidak menggantikannya,
  melainkan menambahkan akibat kedua dari keputusan yang sama (mekanisme di dalam route, bukan hanya
  isi registry).

## 9. Status per 2026-09-23 (penutup pass)

Bagian ini mengikuti preseden §4.3: teks rencana di atas **tidak dihapus**, dan status tiap temuan
ditulis di sini. Semua angka di bawah diukur dari working tree, bukan dari ingatan.

### 9.1 Ringkasan hasil

| Temuan | Status | Angka sebelum → sesudah |
|---|---|---|
| **F3** sanitasi `base_url` | **CLOSED** (sudah di working tree saat pass dimulai; diverifikasi ulang) | `…/v1/messages` + `/messages` → `…/v1/messages/messages` menjadi `…/v1/messages` |
| **F4** filter capability | **CLOSED** | `?capability=vision` **0 → 205** baris; `tools` **0 → 502**; `edit`/`mask`/`text2img` tetap 7/1/3 |
| **F2 bagian A** daftar model node | **CLOSED** | `len(entry.Models)` untuk node **0 → 2** (dari upstream); tiga permukaan terisi |
| **F2 bagian B** tabel kredensial + validate URL | **CLOSED** | permukaan validasi **18 → 45** dari 60 entri ber-transport; **7/7** provider claude-format kini tercakup |
| **F6** validate sebelum simpan | **CLOSED** | dua route stateless; `POST /provider-nodes/validate` dan `POST /providers/validate` |
| **F1b** lima field connection | **CLOSED dengan satu deferral** | lima field tersimpan + tersaji; pengikatan `proxy_pool_id` ke egress **ditunda** (§9.3) |
| **F5** `?suggested` | **CLOSED** | field konstan dihapus dari wire, parameter dihapus dari kontrak dan spec |
| **F1, F7, F9, F10** | tetap seperti §6 | F1 sudah dijawab; F7/F9/F10 di luar scope |
| Regenerasi registry (draft 011 §8) | **tetap OPEN** | tidak diputuskan pass ini |

### 9.2 Koreksi pembacaan, dicatat bukan dihapus

Tiga hal yang versi pertama dokumen ini (atau rencana pass) perkirakan salah, dan apa yang sebenarnya
terukur:

1. **§4.4 langkah 2 ("port `capabilities.js` menjadi tabel Go") tidak diperlukan.** Port-nya sudah
   ada: `internal/registry/capability.go` adalah port keputusan vision dari `PATTERN_CAPABILITIES`,
   dan diuji terhadap korpus yang dihasilkan dari `capabilities.js` sendiri: **0 dari 507 baris
   salah**. Tabel penuh 79 aturan yang ditranskripsi mekanis juga 0 salah, jadi port ulang hanya
   menambah risiko tanpa mengubah jawaban. Yang benar-benar hilang bukan tabelnya, melainkan
   **pembacanya**: katalog membaca `registry.yaml` dan tidak pernah bertanya ke tabel itu. Itu
   sebabnya perbaikan pass ini menambah resolver + `tools`, bukan tabel baru.
2. **F2 bagian B bukan "76 provider kehilangan validate URL".** Diukur ulang: **18** entri
   mendeklarasikan `validate_url`, **19** menurunkan URL dari `base_url`, **25** tidak punya
   keduanya. Ke-18 deklarasi reference **sudah ada semua** di `registry.yaml` — tidak ada yang perlu
   disalin. Celah sesungguhnya adalah **7 provider claude-format yang `base_url`-nya ADALAH endpoint
   messages**, sehingga menempelkan path tidak bisa menolong; reference mem-POST satu pesan
   satu-token ke URL itu dan membaca status. Itu yang ditambahkan `PlanFor`.
3. **`auth_query` tidak dipakai entri mana pun.** Field-nya ditambahkan dan diuji (kapabilitas yang
   draft §4.2 minta: "provider yang menaruh kredensial di query tidak bisa dinyatakan sama sekali"),
   tetapi setelah diperiksa di reference pada revisi yang dipin, **tidak ada provider yang
   mendeklarasikannya**: gemini memakai `x-goog-api-key` untuk chat, dan bentuk query (`key`) hanya
   muncul di blok *media*-nya, yang sudah ditangani §7.10 lewat `media.auth_header`. Jadi
   kapabilitasnya kini bisa dinyatakan; menyalakannya menunggu entri yang membutuhkannya.

### 9.3 Deferral yang dinyatakan

`proxy_pool_id` **tidak** diikatkan ke egress per-endpoint di pass ini, dan itu keputusan yang
dicatat, bukan yang terlewat. Alasannya: data plane membangun **satu** `*http.Client` untuk semua
endpoint (`dataplane.NewTransport(TransportDeps{Client})`), sehingga egress per-endpoint menuntut
pool client per-proxy plus pemeriksaan guard per-request — mekanisme baru, bukan field baru. Aturan
urutan draft §6 ("F1b setelah F8") dipenuhi dengan mendaratkan assertion guard F8 **lebih dulu**
(Task 4). Yang operator dapat sekarang: field-nya tersimpan, tervalidasi terhadap tabel `proxies`
(id menggantung ditolak), dan tersaji. Yang belum: lalulintasnya benar-benar keluar lewat proxy itu.

### 9.4 Gerbang

Semuanya dijalankan pada working tree akhir:

| Gerbang | Hasil |
|---|---|
| `go build ./...` | bersih |
| `go vet ./...` | bersih |
| `gofmt -l .` | bersih |
| `go test -race -count=1 ./...` | **15/15 paket `ok`** |
| `staticcheck ./...` | bersih |
| `golangci-lint run` | **0 issues** |
| `scrypts/gates/go-headers.sh` | **800** berkas Go ber-header lengkap |
| `scrypts/gates/go-lint.sh` | lulus; **0** berkas di band 220–250 |
| `scrypts/gates/contract-openapi.sh` | dokumen tersaji = YAML |
| `scrypts/gates/contract-drift.sh` | 11 kode error sepakat |
| `scrypts/gates/go-test.sh` | lulus |
| `scrypts/gates/panel-check.sh` | lulus |
| `scrypts/gates/secrets.sh --all` | commit bersih; working tree **122** temuan, **semuanya pra-ada** (120 `graphify-out/cache`, 2 `app-serv/.env` yang untracked dan ter-`.gitignore`) — pass ini menambah **0** |

### 9.5 Yang sengaja TIDAK dikerjakan

- **F7 `custom-embedding`**, **F9** (bentuk amplop/id), **F10** (enam route siklus hidup model):
  tetap menunggu keputusan §7.2, seperti sebelumnya.
- **Regenerasi `registry.yaml`**: `tools/registry-gen.mjs` diukur dan **tidak** mereproduksi berkas
  yang ter-commit — regenerasi dari pin `db4499d` menghasilkan `revision: 9router@db4499d6` dan
  **kehilangan** tiga model OpenCode Free serta sepuluh voice TTS yang ditambahkan tangan di
  commit `34f2c38`. Karena itu `auth_query` **tidak** ditulis lewat regenerasi; generator hanya
  diperbaiki agar tidak menjatuhkannya bila kelak dijalankan. Ketidakreproduksian ini cacat
  tersendiri dan dicatat di sini sebagai temuan baru untuk draft 011 §8, bukan ditambal diam-diam.
- **`app-ui`**: pass ini tidak menyentuh satu berkas pun di sana. Draft
  `019-PROVIDER-SURFACE-PARITY.md` yang berjalan bersamaan menunggu dua kiriman pass ini (D3 tombol
  Check → `POST /provider-nodes/validate`, dan F5 isi `/models` untuk node); keduanya kini menjawab,
  dan penyambungannya milik pass `app-ui`.
