# 019-PROVIDER-SURFACE-PARITY.md: Permukaan `/provider` (dialog Add Compatible + detail page) 1:1 dengan REFERENCE

Dokumen kerja pass `app-ui` untuk halaman **Provider**. Pass ini mengerjakan dua hal yang owner sebut
langsung: dialog **Add Anthropic Compatible / Add OpenAI Compatible** belum 1:1 dengan reference, dan
**detail page** belum punya mekanisme add API key, bulk API key, serta penambahan model id yang bekerja.
Bukan kontrak; kontrak tetap `docs/SPEC-API/001-SPEC-API.md` (wire) dan `docs/SPEC-UI/001-SPEC-UI.md`
(perilaku panel).

|                        |                                                                                                                                                                                                   |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**             | DURING berjalan. F1, F2, dan F4 (kecuali baris Check) diperbaiki; F3 dan F5 dibangun ulang setelah owner menolak konsepnya; F6 (label kolom tersembunyi yang melebarkan dokumen di 390 px) ditemukan dan ditutup oleh pengukuran gerbang; baris Check bergantung `app-serv` |
| **Mechanism**          | DURING & AFTER (antislop)                                                                                                                                                                         |
| **Scope**              | `app-ui/.` saja. `app-serv/.` tidak disentuh pass ini                                                                                                                                              |
| **Permintaan owner**   | `Lanjut ke page: Provider`: (1) Add Anthropic Compatible & Add OpenAI Compatible belum 1:1 dengan REFERENCE; (2) pada detail page belum ada mekanisme add api key, bulk api key, serta penambahan models id, belum berjalan semestinya. Langsung FIX |
| **Koreksi owner**      | Halaman detail node custom diukur lalu dibangun ulang ke bentuk reference; kalimat owner: "Custom provider >> Detail Provider = SALAH KONSEP, sangat jauh dari REFERENCE yang seharusnya." |
| **Reference**          | `decolua/9router`, checkout `/home/rusmanadodi/apps/9router`, dibaca di `origin/master` (`21583c03` = tag `v0.5.85`)                                                                                |
| **Kaitan**             | draft 017 (pass `app-serv` yang berjalan bersamaan: F2 daftar model node, F6 validate); SPEC-API §7.4, §7.5, §7.6; SPEC-UI §6.2, §6.3; DESIGN.md §11; AGENTS.md §1.1                              |
| **Tanggal**            | 2026-09-23                                                                                                                                                                                        |

## 1. Ringkasan

Diukur dulu sebelum satu baris diubah, karena dua dari tiga keluhan owner menunjuk ke sebab yang berbeda
dari yang tersirat di kalimatnya. Hasil pengukuran hidup terhadap gateway yang sedang berjalan
(`:9090`, panel owner di `:3000` tidak disentuh):

| Yang owner sebut                        | Yang diukur                                                                                                                                          |
| --------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| Add Compatible belum 1:1                | **Benar sebagian.** Empat fieldnya sudah ada dan aturan prefix/base URL panel malah lebih ketat daripada reference. Yang hilang: default base URL yang terisi, placeholder/hint per varian, reset saat API Type berubah, dan tombol **Check**, yang terakhir butuh route yang `app-serv` belum punya. Satu lagi ditemukan saat memperbaikinya: hint base URL per varian **tidak pernah tampil**, karena preview URL gabungan menggantikannya dan URL itu selalu terisi (F4) |
| belum ada mekanisme add api key         | **Benar**, dan sebabnya lebih dalam: form "Add an endpoint" yang ada **selalu ditolak** wire. Body panel membawa `key_value`, DTO `app-serv` hanya mengenal `keys`, dan decoder-nya `DisallowUnknownFields`. Diukur: `400 unknown field "key_value"` |
| bulk api key                            | **Mekanismenya ada, tempatnya salah.** Panel sudah punya bulk key (`POST /endpoints/{id}/keys/bulk`, `BulkAddKeysForm`) tetapi hanya di drawer endpoint pada `/endpoint-keys`, bukan di detail page provider seperti reference |
| penambahan models id belum berjalan     | **Benar.** Wire menuntut `display_name` (`validate:"required,min=1,max=120"`), panel menuntutnya lebih dulu lagi lewat `label` ("A name is required."), sedangkan reference menambah model hanya dengan model id. Diukur: `400 field DisplayName failed validation: is required` |
| Custom provider detail = konsep salah   | **Benar.** Halaman detail node custom merender blok milik provider registry (`Custom provider`, `Model catalog`, `Models this provider cannot route`, `Custom models`, `Aliases`, `Connections` di posisi terakhir, `Endpoint`), sedangkan reference memberi node tiga blok saja: kartu detail, `Connections` tepat di bawahnya, lalu `Available Models` (`/tmp/ref019/page.js:1447-1506`, `:1508-1741`, `:1745-1819`). Diukur hidup sebelum diubah (`/tmp/provider019/evidence-node-page.json`: 13 heading halaman, 2 tangkapan layar) |

Jadi F1 (form endpoint yang selalu ditolak) adalah cacat terukur yang menjelaskan keluhan "add api key
belum berjalan semestinya" lebih baik daripada dugaan "belum ada mekanismenya": mekanismenya ada, tetapi
tidak pernah berhasil.

Koreksi owner di tengah pass, dan konsekuensinya:

> "Kenapa di dalam detail provider yang telah di tambahkan saat: add api key / bulk api key, harus add
> endpoint tambahan? KONSEP SALAH. Anda baca gak itu REFERENCE, bagaimana konsep detail provider."

Koreksi itu benar dan terukur. Reference tidak mengenal "endpoint" sebagai benda yang operator harus buat
lebih dulu: halaman detail provider memegang daftar **Connections** (`providers/[id]/page.js:1508-1514`,
satu baris per kredensial), dan **setiap key yang ditambahkan menjadi satu baris di daftar itu**
(`AddApiKeyModal.js:148-182`: mode bulk mengirim satu request per key). Bentuk pertama F3 di pass ini (satu
endpoint berisi N key) memaksa operator memberi nama sebuah wadah lebih dulu, dan itu memang konsep yang
salah. F3 dibangun ulang: satu key sama dengan satu connection (§5).

Koreksi kedua datang setelah F1 sampai F4 selesai, dan menunjuk halaman yang belum tersentuh pass ini:

> "Custom provider >> Detail Provider = SALAH KONSEP, sangat jauh dari REFERENCE yang seharusnya."

Itu juga benar dan terukur: halaman detail node custom memakai bentuk halaman provider registry, sedangkan
reference memberi node bentuknya sendiri. F5 dibangun ulang mengikuti bentuk itu (§7).

## 2. Keputusan yang dipakai pass ini

| #   | Pertanyaan                                                                                              | Arah yang dipakai                                                                                                                                                            | Dipakai oleh |
| --- | ------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------ |
| D1  | Bulk key di detail page: satu endpoint berisi N key, atau satu connection (endpoint) per key?             | **Satu connection per key.** Owner menolak bentuk pertama sebagai konsep yang salah, dan reference memang begitu: halaman detailnya memegang daftar Connections (`page.js:1508-1514`) dan mode bulk `AddApiKeyModal` mengirim satu request per key (`:148-182`). Wire panel sudah punya route untuk bentuk itu, `POST /api/v1/endpoints/bulk` (`router.go:138`, `endpoint.go:122-136`), yang sebelumnya tidak dipakai panel mana pun | F3           |
| D2  | Menambah model id: apakah field display name dihapus atau dibuat opsional?                                | **Opsional, dengan default model id.** Wire menuntut `display_name` non-kosong, jadi menghapus fieldnya akan membuat setiap penambahan gagal. Model id adalah default yang jujur: `customModelLabel` panel sudah memakai model id sebagai tampilan saat display name kosong | F2           |
| D3  | Tombol **Check** pada dialog Add Compatible (API key + model id opsional → validate)                     | **Tidak dibangun pass ini.** Route `POST /api/v1/provider-nodes/validate` diukur `405` (tidak ada), dan draft 017 F6 mencatatnya sebagai pekerjaan `app-serv` yang belum mendarat. Kontrol yang memanggil route tidak ada adalah kontrol mati (R-26); panel mencatatnya, bukan memalsukannya | F4           |
| D4  | Halaman detail node custom: bentuk provider registry yang sudah ada, atau bentuk node milik reference?     | **Bentuk node milik reference.** Owner menolak bentuk pertama sebagai konsep yang salah. Node bukan provider registry: katalog registry-nya kosong untuk node, `routability` tidak berarti apa-apa di sana, dan `Connections` adalah blok utamanya, bukan blok terakhir. Provider registry tetap memakai urutan bloknya sendiri, tidak diubah pass ini | F5           |
| D5  | **Import from /models** untuk node: dipasang, padahal F2 mencatat datanya belum ada?                      | **Dipasang, dengan hasil yang jujur.** Route `GET /api/v1/providers/{provider_id}/models` sudah ada dan menjawab `200 {"data":[]}` untuk node; yang belum ada adalah isinya (draft 017 F2). Tombol yang memanggil route hidup bukan kontrol mati (R-26), dan jawaban kosongnya diterjemahkan apa adanya, "No models returned from /models.", persis kalimat reference untuk hasil yang sama (`CompatibleModelsSection.js:140`). Keputusan F2 §4 untuk **tidak** memasangnya di provider registry tetap, karena di sana daftar itu sudah tampil sebagai katalog | F5           |

## 3. F1 HIGH (FE): form "Add an endpoint" selalu ditolak wire

**Status: DURING selesai.**

**Fakta.** Sebelum pass ini `CreateEndpointForm.svelte` menyusun body `{provider_id, label, auth_type,
priority, key_value}` dari `schemaCreateEndpointForm`, dan `createEndpoint` mengirim objek itu apa adanya
sebagai body. `key_value` selalu ikut, termasuk saat kosong, karena `optionalSecretValue` adalah union yang
menerima `''`. Wire-nya tidak mengenal field itu:

```bash
# app-serv/internal/schema/endpoint.go:89-95
type CreateEndpointRequest struct {
	ProviderID string             `json:"provider_id" ...`
	Label      string             `json:"label" ...`
	AuthType   string             `json:"auth_type" ...`
	Priority   int                `json:"priority,omitempty" ...`
	Keys       []EndpointKeyInput `json:"keys,omitempty" ...`
}
# app-serv/internal/schema/validator.go:44-48
d.DisallowUnknownFields()
```

**Diukur hidup** (`/tmp/provider019/probe-wire.ts`, body persis seperti yang form kirim):

```
POST /api/v1/endpoints  -> 400
{"error":{"code":"VALIDATION_ERROR","message":"invalid request body:  unknown field \"key_value\""}}
```

Artinya form itu tidak pernah berhasil, dengan atau tanpa key yang diketik. Diukur juga bahwa bentuk
yang benar bekerja, dan bahwa `auth_type=api_key` **mewajibkan** minimal satu key:

```
POST /api/v1/endpoints  {"keys": []}                -> 400 "an api_key endpoint requires at least one key"
# app-serv/internal/service/endpoint_create.go:53-55
```

Konsekuensinya kedua: body harus memakai `keys`, dan field "First key (optional)" panel adalah klaim
yang salah untuk `api_key`: key wajib untuk auth type itu (`apikey` juga, karena `registryAuthAliases`
memetakan keduanya ke `api_key`).

**Perbaikan.** Body dipisah dari form: `createEndpointBody(form)` di lapisan schema
(`endpoint-write.ts:89-98`) menyusun bentuk wire (`keys` bila ada key), `createEndpoint`
(`api/endpoints.ts:86-95`) memakainya, dan schema form (`endpoint-write.ts:47-66`) menolak `api_key` tanpa
key dengan pesan yang sama artinya dengan pesan gateway. Komentar "The first credential is optional"
diganti, karena itu yang membuat cacat ini tidak terlihat selama ini.

**Kriteria selesai.** Form membuat endpoint sungguhan lewat wire, key yang diketik tersimpan, dan
`api_key` tanpa key ditolak panel sebelum round trip dengan pesan yang menyebut sebabnya. Terpenuhi:
dibuktikan di click-through §9.1 (aksi 3 dan 4).

## 4. F2 MEDIUM (FE): penambahan model id menuntut display name yang reference tidak minta

**Status: DURING selesai.**

**Fakta.** Reference menambah model hanya dengan model id (`CompatibleModelsSection.js:106-118`:
`handleAdd` memanggil `onAddCustomModel(modelId)`; satu-satunya field di layar itu bernama `Model ID`,
`:171`). Panel menuntut dua field: `schemaCustomModelForm` memakai `label` untuk `display_name`, yang
menolak kosong dengan "A name is required." (`primitives.ts:23-32`). Wire menuntut hal yang sama:

```bash
# app-serv/internal/schema/model.go:47
DisplayName  string   `json:"display_name" validate:"required,min=1,max=120"`
```

**Diukur hidup:**

```
POST /api/v1/models/custom  {"provider_id":"openai","model_id":"probe-019-model"}
  -> 400 {"error":{"code":"VALIDATION_ERROR","message":"field DisplayName failed validation: is required"}}

POST /api/v1/models/custom  {..., "display_name":"probe-019-model"}
  -> 201 {"id":"mdl_0386C2XTFB2E4S6SY7WWXBK1KE", ...}
```

Baris probe itu dihapus lagi setelah pengukuran (`DELETE /models/custom/mdl_…` → `204`, daftar custom
kembali `0` baris), jadi tidak ada sisa data pass ini di gateway owner.

**Perbaikan (D2).** Display name menjadi opsional di form (`custom-model.ts:83-95`); `customModelBody`
(`:115-123`) mengirim model id sebagai display name saat field itu kosong. Aturan panjang dan angle
bracket tetap berlaku saat diisi, karena keduanya aturan wire juga.

**Kriteria selesai.** Menambah model dengan hanya model id berhasil, dan barisnya tampil dengan nama
model id. Terpenuhi: dibuktikan di click-through §9.1 (aksi 9).

**Yang tidak dikerjakan dan alasannya.** Reference juga punya **Import from /models** di section yang
sama (`CompatibleModelsSection.js:125-159`, tombolnya `:185-186`): baca `GET /api/providers/{id}/models`
milik connection aktif, lalu tambahkan setiap id yang belum ada. Untuk provider registry panel tetap tidak
memasangnya: daftar itu sudah tampil di section "Model catalog" di atasnya, jadi mengimpornya sebagai
custom model akan menduplikasi model yang sudah routable, dan permintaannya sudah tercatat di draft 017.
Untuk **node** keputusan itu dibalik oleh F5: di halaman node tidak ada katalog registry, route itu
satu-satunya sumber model node, dan hasil kosongnya (`{"data":[]}` dari gateway yang berjalan, draft 017 F2
bagian A) diterjemahkan apa adanya menjadi "No models returned from /models." (§7, D5).

## 5. F3 MEDIUM (FE): detail page provider tidak punya kontrol add key, reference punya

**Status: DURING selesai setelah konsepnya dibangun ulang.**

**Fakta.** Reference menaruh kontrolnya di halaman detail provider, dan konsepnya adalah daftar
**Connections**: satu baris per kredensial (`providers/[id]/page.js:1508-1514`; barisnya dirender
`ConnectionRow.js`, yang memakai `connection.name` sebagai nama baris, `:77-80`). Tombolnya `Add API Key`
untuk provider compatible (`:1631`), dan dialognya (`AddApiKeyModal.js`) punya dua mode (`:195-196`):

- **Single** (`:229-403`): Name (wajib, `:96-99`), API Key, Priority (`:367-372`), lalu Save.
- **Bulk Add** (`:199-227`): satu key per baris, format `name|apiKey` atau `apiKey` saja yang diberi nama
  otomatis (`:206`), tombol `Add All Keys` (`:220-225`), hasil `✓ N added, ✗ M failed` (`:215-219`).

Yang menentukan dari mode bulk: ia **mengirim satu request per key** (`:148-182`, satu `POST /api/providers`
per baris rencana), jadi N key menjadi N connection, bukan satu wadah berisi N key. Nama tiap baris
direncanakan `planBulkAdd` (`shared/utils/bulkAdd.js:76-105`) supaya tidak menabrak nama yang sudah ada,
karena backend reference melakukan upsert berdasarkan nama (`:1-18`).

Panel sebelum pass ini hanya punya **tautan** ke layar lain
(`providers/[provider_id]/+page.svelte:165-168` menunjuk `/endpoint-keys?provider=…`), dan mekanisme
key-nya hidup di drawer endpoint.

**Bentuk pertama F3, dan kenapa dibuang.** Versi pertama pass ini menambahkan kontrol `Add an API key` yang
menyusun **satu endpoint berisi N key** (mode "One key" dan "Several keys"), dengan alasan bahwa SPEC-API §7.5
memang memberi satu endpoint 1..N key. Owner menolak: konsep itu membuat operator harus membuat "endpoint
tambahan" lebih dulu untuk menyimpan key, sesuatu yang reference tidak pernah minta. D1 dibalik.

**Perbaikan (D1 baru).** Section itu sekarang memakai konsep reference:

1. Judul section menjadi **Connections**, kalimat kosongnya "No connections yet" (`page.js:1591`), dan
   kontrolnya **Add API Key** (`:1631`) untuk provider yang auth type-nya memakai key. Provider yang tidak
   memakai key tetap memakai tautan ke form endpoint, karena form itulah yang membawa pilihan auth type.
   Tabel di bawahnya tetap tabel SPEC-UI §6.2 yang sama, jadi kolomnya tetap memakai kata API untuk barisnya.
2. Satu key menjadi satu connection: `POST /api/v1/endpoints` dengan `keys: [{value}]` dan label dari
   operator (`AddProviderKeysDialog.svelte`, mode Single: Name, API Key, Priority).
3. Beberapa key menjadi beberapa connection dalam satu batch: `POST /api/v1/endpoints/bulk`
   (`router.go:138`; body `endpoint.go:122-136` menyatakan provider dan auth type sekali, lalu satu elemen
   per akun, maksimum 50). Route ini sudah ada dan **belum pernah dipakai panel**; bentuknya memang bentuk
   bulk reference, satu elemen per key.
4. Baris tanpa nama direncanakan `planConnectionLines` (`connection-plan.ts`): gap-fill `Key <n>` terhadap
   nama yang sudah tersimpan dan nama yang sudah dipakai paste itu sendiri. Alasannya sama dengan alasan
   reference: `(provider_id, label)` unik
   (`app-serv/migrations/000005_upstream_endpoints.up.sql:29`), jadi nama yang menabrak menolak seluruh
   batch. Nama yang diketik operator dipertahankan apa adanya bila masih bebas, berbeda dari reference yang
   menambahkan indeks ke setiap baris (`bulkAdd.js:76-96`); alasan perbedaan itu ada di header
   `connection-plan.ts`.
5. Batch yang ditolak tidak menyimpan apa pun (SPEC-API §8.1, `endpoint_bulk.go:43-91`), dan pesannya dikembalikan
   ke **nomor baris yang ditempel** operator, bukan ke indeks batch: `endpoint_bulk.go` melaporkan tiap
   baris lewat `results[].index` (`handler/endpoint_bulk.go:139-166`), dan dialog memetakan indeks itu ke
   baris asalnya.

**Selisih yang tersisa, dengan sebabnya.**

| Hal                     | Reference                                                | Panel                                          | Sebab                                                                                                                        |
| ----------------------- | -------------------------------------------------------- | ---------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| Bentuk batch            | satu request per key, sukses sebagian dihitung (`:148-182`) | satu `POST /endpoints/bulk`, semua-atau-tidak | Route batch panel memang satu transaksi (SPEC-API §8.1, `endpoint_bulk.go:87-89`); penolakan tetap menyebut baris yang salah           |
| Field **Default Model** | ada untuk provider compatible (`:295-302`)               | tidak ada                                      | `EndpointResponse` tidak punya field model default (`endpoint_response.go:64-83`), jadi tidak ada tempat menyimpannya        |
| Select **Proxy Pool**   | ada (`:374-389`)                                         | tidak ada                                      | Endpoint panel tidak membawa field proxy                                                                                     |
| Tombol **Check**        | API Key lalu `POST /providers/validate` (`:263-266`)     | tidak ada                                      | Belum ada route untuk memvalidasi kredensial sebelum barisnya ada; tercatat di draft 017 F6                                   |

**Catatan implementasi.** Pilihan tab hidup di dalam `PanelTabs`, bukan di dialog, sehingga versi pertama
pass ini punya mode bulk yang fieldnya tampil tetapi tidak pernah terkirim: `collect()` dan tombol submit
membaca `mode` yang tidak pernah berubah. Yang menemukannya adalah test route ini, bukan pembacaan ulang
kode. `PanelTabs` sekarang memberi tahu pilihan itu lewat prop opsional `onchange`, dan dialog memakainya
sebagai satu-satunya sumber `mode`.

**Kriteria selesai.** Dari detail page provider, operator bisa menyimpan satu key dan beberapa key tanpa
berpindah layar, tanpa pernah membuat "endpoint" lebih dulu, dan tiap key tampil sebagai satu baris
connection di section itu. Terpenuhi: dibuktikan di click-through §9.1 (aksi 5 sampai 8).

## 6. F4 LOW (FE): sisa selisih dialog Add Compatible

**Status: DURING selesai untuk yang tidak butuh wire; satu item bergantung `app-serv`.**

**Fakta.** Reference (`AddCompatibleModal.js`) memberi tiap varian konfigurasi sendiri:

| Hal               | Reference                                                        | Panel sebelum pass ini                                  |
| ----------------- | ---------------------------------------------------------------- | ------------------------------------------------------- |
| Default base URL  | terisi nilai (`defaultBaseUrl`)                                  | hanya placeholder                                       |
| Placeholder nama  | `OpenAI Compatible (Prod)` / `Anthropic Compatible (Prod)`        | `OpenAI Compatible (prod)`                              |
| Placeholder prefix| `oc-prod` / `ac-prod`                                            | `mycorp`                                                |
| Hint base URL     | per varian, Anthropic menyebut `/messages` yang ditambahkan       | satu kalimat generik                                    |
| Hint vs preview   | hint selalu tampil di bawah field (`Input hint`, `:166`)          | hint **tergantikan** preview URL gabungan, dan karena URL itu selalu terisi, hint per varian tidak pernah terbaca |
| Ganti API Type    | me-reset base URL ke default varian                              | tidak ada perilaku                                      |
| **Check**         | API Key + Model ID opsional → `POST /provider-nodes/validate`     | tidak ada (route `app-serv` tidak ada; D3)              |

**Perbaikan.** Lima baris pertama dikerjakan: default base URL menjadi nilai draft baru
(`NODE_BASE_URL_DEFAULTS`), placeholder dan hint per varian (`NODE_COPY`), dan select API Type
me-reset base URL ke default varian saat ditambah. Baris **hint vs preview** ikut diperbaiki setelah
diukur: hint sekarang selalu tampil di bawah field seperti reference, dan URL gabungan menjadi baris
tambahan di bawahnya, bukan pengganti. Satu kata sengaja tidak disalin: reference menulis "The system
will append /messages", panel menulis "The gateway appends /messages", karena seluruh panel menyebut
`app-serv` sebagai gateway dan satu kata asing di tengah dialog adalah inkonsistensi yang lebih besar
daripada selisih kata itu sendiri. Baris **Check** tidak dibangun (D3) dan tercatat sebagai satu-satunya
selisih yang tersisa, dengan route yang ditunggu sudah punya nomor di draft 017 F6.

**Kriteria selesai.** Dialog menampilkan nilai dan kalimat per varian, dan ganti API Type
mengembalikan base URL ke default. Terpenuhi: dibuktikan di click-through §9.1 (aksi 1 dan 2).

## 7. F5 MEDIUM (FE): halaman detail node custom memakai bentuk halaman provider registry

**Status: DURING selesai.**

**Fakta.** Kalimat owner: "Custom provider >> Detail Provider = SALAH KONSEP, sangat jauh dari REFERENCE
yang seharusnya." Diukur di browser sebelum satu baris diubah (`/tmp/provider019/probe-node-page.ts`, node
`openai-compatible-0386CEPG…`, 2 tangkapan layar, 7 kelompok nilai): halaman itu merender blok milik
provider registry dalam urutan `Custom provider` (kartu, judulnya jenis node), `Model catalog`,
`Models this provider cannot route`, `Custom models`, `Aliases`, `Connections` di posisi terakhir, dan
`Endpoint`. Untuk sebuah node, tiga di antaranya tidak punya isi: katalog registry node diukur 0 baris,
section model-disabled selalu kosong karena tidak ada baris registry yang bisa dimatikan, dan jawaban
provider untuk node sendiri memperlihatkan sebabnya (`routability: "native"`, `category: "apikey"`,
`model_count: 0`). Yang paling jauh dari reference adalah urutannya: `Connections`, blok pertama setelah
kartu di reference, justru berada di bawah semua blok registry.

Reference (`/tmp/ref019/page.js`) memberi node tiga blok saja:

- `:1447-1506` kartu detail, judul `OpenAI Compatible Details` / `Anthropic Compatible Details` (`:1451`),
  baris `Chat Completions · <base>/chat/completions`, `Responses API · .../responses`, atau
  `Messages API · .../messages` (`:1452-1454`), aksi Add API Key / Edit / Delete.
- `:1508-1741` **Connections**: keadaan kosong `No connections yet` (`:1591`), tombol `Add API Key`
  (`:1631`), satu baris per kredensial.
- `:1745-1819` **Available Models**: Model ID + Add, tombol `Import from /models` (`CompatibleModelsSection.js:185-186`),
  baris `alias/modelId`.

Tidak ada tabel registry, tidak ada blok model-disabled, dan tidak ada blok alias di halaman ini.

**Perbaikan (D4, D5).** Halaman detail sekarang bercabang dua bentuk, dan hanya bentuk node yang berubah:

1. **Provider registry**: urutannya tidak berubah sama sekali (Facts → Model catalog → Models this provider
   cannot route → Custom models → Aliases → Connections → OAuth), dan pembacaan store `disabled` dilewati
   untuk node, karena daftar itu milik registry.
2. **Node custom**: kartu detail → **Connections** → **Available Models** → **Aliases**, dengan perubahan
   berikut:
   - Kartu detail memakai judul `{jenis} Details`, baris label + URL dari `nodeEndpointLabel` /
     `nodeEndpointUrl` (keduanya sudah menangani `responses` dan `/messages`), aksi Add API Key / Edit /
     Delete, dan baris fakta `Endpoint` dihapus karena URL-nya kini ada di bawah judul, seperti reference.
     Kartu juga menyerahkan `prefix` node ke halaman lewat prop `onprefix`, karena baris model harus tampil
     sebagai `prefix/model` dengan tombol Copy.
   - **Connections** naik ke posisi kedua dengan judul, kalimat kosong, dan tombol yang sama seperti F3
     (`ProviderConnectionsSection`).
   - **Available Models** memakai kalimat pengantar "Add {vendor}-compatible models manually or import them
     from the node's /models endpoint." dan keadaan kosong "No models for this node yet" dengan penjelasan
     `A model declared here is addressed as {prefix}/model-id.`.
   - **Import from /models** (D5): gate-nya sama dengan reference, `canImport = connections.some(conn => conn.isActive !== false)`
     (`CompatibleModelsSection.js:161`) dengan kalimat `Add a connection to enable importing models.`
     (`:190-194`) saat tidak ada connection aktif. Panel membaca daftar connection-nya sendiri alih-alih
     memakai baris yang sudah dirender, dan baca yang gagal meninggalkan gerbangnya terbuka, karena
     jawaban import itu sendiri yang jadi kebenaran. Dua kalimat hasil reference disalin apa adanya:
     `No models returned from /models.` (`:140`) dan `No new models were added.` (`:152`), dan penolakan
     gateway diteruskan sebagai `Nothing was imported. The gateway refused …`. Tiap baris baru menjadi satu
     `POST /models/custom` dengan `display_name` = nama baris kalau ada, kalau tidak model id (aturan F2),
     dan baris yang sudah ada tidak dideklarasikan ulang.

**Selisih yang tersisa, dengan sebabnya.**

| Hal                  | Reference                                                            | Panel                                             | Sebab                                                                                                                                                                        |
| -------------------- | -------------------------------------------------------------------- | ------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Blok **Aliases**     | tidak ada blok terpisah; alias menempel di baris model (`page.js:1216-1223`) | ada, sebagai blok keempat                         | Tabel alias adalah bagian layar detail provider di SPEC-UI §6.3 (`001-SPEC-UI.md:326-330`) dan setnya global, jadi ia tetap tampil di halaman node; bentuk reference tidak bisa disalin tanpa memindahkan set global itu ke dalam tabel model |
| Isi `/models`        | daftar model upstream                                                | `{"data":[]}`                                     | Route-nya ada, isinya pekerjaan `app-serv` (draft 017 F2); panel menerjemahkan jawaban apa adanya, bukan menyembunyikannya                                                     |
| Tombol **Check**     | hanya di dialog Add, bukan di kartu                                  | tidak ada                                         | Sama dengan D3                                                                                                                                                                |

**Kriteria selesai.** Halaman detail node menampilkan tiga blok reference dalam urutannya, tidak lagi
menampilkan blok registry, dan import model membaca route node lalu melaporkan hasilnya apa adanya.
Terpenuhi: dibuktikan di click-through §9.2.

## 8. F6 LOW (FE): label kolom tersembunyi melebarkan dokumen di 390 px

**Status: DURING selesai.**

**Fakta.** Pengukuran gerbang di 390 px (R-03) menemukan halaman ini bisa digeser mendatar:
`documentElement.scrollWidth` 652 px terhadap viewport 390, dan `window.scrollTo(400, 0)` benar-benar
memindahkan halaman 262 px, di kedua tema. Pelakunya bukan tabelnya, melainkan label kolom tersembunyi di
dalamnya. Tabel lebar permukaan provider memakai `<span class="sr-only">Actions</span>` sebagai label kolom
aksi, dan `sr-only` Tailwind adalah `position: absolute`; karena tidak ada leluhur ber-`position: relative`,
containing block label itu adalah initial containing block, sehingga label 1 px itu keluar dari pembungkus
`overflow-x: auto` dan mendarat di ujung kanan tabel. Terukur: satu-satunya elemen non-`static` di luar
viewport adalah `span.sr-only` ber-`right: 652`, sama di light dan dark
(`/tmp/provider019/evidence-mobile-before.json`).

**Perbaikan.** Pembungkus gulir di lima tabel permukaan provider dijadikan containing block dengan
`relative`, jadi label itu tinggal di dalam kotak gulirnya dan tidak lagi menyentuh dokumen:
`CustomModelTable.svelte`, `ModelCatalogTable.svelte`, `ProviderAliases.svelte`,
`ProviderDisabledModels.svelte`, dan `ProviderOAuthAccounts.svelte`. Satu baris komentar di tiap berkas
menyebut alasan `relative`-nya, karena kelas itu terlihat seperti hiasan. Pembungkus `EndpointTable.svelte`
sengaja tidak diubah: label tersembunyinya sebuah `caption`, posisi statisnya di tepi kiri tabel, dan
terukur tidak pernah keluar viewport (`wrapperPosition: static` di §9.3).

**Kriteria selesai.** Di 390 px halaman tidak bisa digeser mendatar, di kedua tema dan di kedua bentuk
halaman. Terpenuhi: diukur di §9.3 (`pageOverflow` 262 menjadi 0, `scrolledX` 262 menjadi 0, dan 0 elemen
lolos dari scroller-nya).

## 9. Bukti gate

Dijalankan di `app-ui/` pada tree beku `a1d057db6e7084ff487831ebb61bc3cd` (`md5sum` atas `src/`, `tests/`,
`static/`, dan `scripts/`, 497 berkas), sesudah F5, dua pemecahan berkas di gerbang ukuran, dan perbaikan
F6 (lima pembungkus `relative`, lalu satu jalan Prettier atas berkas yang sama). Gerbang ini semula
dijalankan pada tree `cfd19f1ce7e72437ae526c8c46382407`; pengukuran 390 px milik F6 menemukan cacatnya
sesudah commit pass, jadi seluruh gerbang dijalankan ulang pada tree di atas dan baris-baris di bawah
adalah hasil jalan terakhir. Kalimat ini sebelumnya menulis 490 berkas untuk definisi empat direktori yang
sama; 490 adalah jumlah `src/` + `tests/` saja, jadi angkanya dikoreksi di sini alih-alih diwariskan.

| Gate              | Hasil                                          |
| ----------------- | ---------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                           |
| `bun run lint`    | Prettier: seluruh berkas patuh                 |
| `bun run lint:ts` | ESLint keluar 0                                |
| `bun run build`   | sukses, `build/index.js` tertulis              |
| `bun run test`    | 2604 tes lulus dari 154 berkas, 1958 s di host ini, pada tree beku di atas, hijau di percobaan pertama. Jalan pada tree sebelumnya (`cfd19f1ce7e72437ae526c8c46382407`) tidak hijau di percobaan pertama: dua tes kena timeout 5000 ms Vitest (satu di antaranya tes sinkron, yang tidak bisa menggantung pada promise) selagi `golangci-lint` dan proses tes Go aktor lain berjalan di host yang sama; tidak ada suntingan di antara kedua jalan, dan jalan kedua yang hijau itu yang dicatat |
| Uji terarah (F6)  | tujuh berkas yang memuat lima komponen itu lulus, exit 0, 310,77 s: `model-catalog.test.ts`, `provider-aliases.test.ts`, `provider-custom-models.test.ts`, `provider-custom-models-node.test.ts`, `provider-oauth.test.ts`, `provider-models.test.ts`, dan `provider-detail-node-page.test.ts` |
| Click-through     | dua puluh tangkapan layar, lima puluh kelompok nilai (§9.1, §9.2, §9.3) |
| Ukuran berkas     | dua berkas yang pass ini dorong melewati ambang 220 dipecah, bukan dicatat: `CustomProviderCard.svelte` 232 menjadi 209 + `ProviderNodeFacts.svelte` 37, dan `AddProviderKeysDialog.svelte` 227 menjadi 177 + `api/provider-keys.ts` 126. Dua berkas lain sudah melewati ambang itu di HEAD dan tetap dilewati, dicatat apa adanya: `tests/support/model-stub.ts` 483 menjadi 652, dan `tests/components/provider-custom-models.test.ts` 273 menjadi 293. Lima tabel yang diperbaiki F6 semuanya jauh di bawah ambang: `ProviderAliases.svelte` 149, `ProviderOAuthAccounts.svelte` 122, `ProviderDisabledModels.svelte` 113, `ModelCatalogTable.svelte` 108, dan `CustomModelTable.svelte` 85 (82 sebelum F6; dua baris tambahannya komentar dan atribut kelas yang dibungkus Prettier) |

### 9.1 Click-through terekam: jalur provider registry

Panel dijalankan dari salinan hasil build di `/tmp/provider019/panel` pada port 3001 dengan
`PANEL_API_TARGET=http://127.0.0.1:9090`, di atas gateway owner yang sedang berjalan. Port owner 3000 dan
9090 tidak disentuh. Driver: `/tmp/provider019/drive-browser.ts` (Chrome headless + CDP), bukti:
`/tmp/provider019/evidence-browser.json` (17 kelompok nilai), tangkapan layar:
`/tmp/provider019/shots/01..08*.png`. Baris ini dijalankan ulang pada build terakhir (tree beku di atas,
sesudah F5 dan dua pemecahan berkas di gerbang ukuran) dan hasilnya sama dengan jalannya sebelum F5, jadi
jalur registry tidak tersentuh pass ini.

| Aksi                          | Nilai terukur                                                                                                                                                    |
| ----------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Login                         | mendarat di `/endpoint-keys`                                                                                                                                      |
| Dialog Add OpenAI Compatible  | judul `Add OpenAI Compatible`, Base URL bernilai `https://api.openai.com/v1`, placeholder nama `OpenAI Compatible (Prod)`, prefix `oc-prod`, kalimat penutupnya menyebut `https://api.openai.com/v1/chat/completions` |
| Ganti API type ke Responses   | Base URL kembali ke default varian dan kalimatnya berganti ke `.../v1/responses`                                                                                  |
| Dialog Add Anthropic Compatible | tanpa select API type, Base URL `https://api.anthropic.com/v1`, hint menyebut "The gateway appends /messages."                                                   |
| Judul section detail provider | `Connections`, keadaan kosong `No connections yet`                                                                                                                |
| Dialog add key                | judul `Add OpenAI API Key`, field Name/API Key/Priority, tombol `Save` dan `Add All Keys`                                                                         |
| Satu key                      | `✓ Probe 019 single added.`, tabel menampilkan satu baris `Probe 019 single`, `API key`, priority 1, `1 of 1 healthy`                                              |
| Buka ulang dialog             | field Name kosong, jadi regresi label basi tetap tertutup                                                                                                         |
| Tiga baris tempel             | `✓ 3 added.`, tabel menampilkan tiga baris baru: `Probe 019 bulk a`, `Probe 019 bulk b`, `Probe 019 bulk d`, masing-masing `1 of 1 healthy`                        |
| Model id saja                 | `openai/probe-019-model was added to the catalog.`, tabel custom memuat `probe-019-model`                                                                         |
| Pembersihan                   | 4 endpoint dan 1 model dihapus lewat API dengan cookie sesi; sisa `endpoints=1` (baris `ep-live` milik aktor lain), `models=0`; muat ulang menampilkan `No connections yet` |

### 9.2 Click-through terekam: halaman node custom, sebelum dan sesudah

Dua driver, satu untuk mengukur bentuk lama dan satu untuk membuktikan bentuk baru, keduanya di atas panel
dan gateway yang sama: `/tmp/provider019/probe-node-page.ts` (sebelum, bukti
`/tmp/provider019/evidence-node-page.json` 7 kelompok nilai, tangkapan layar `shots-node/01..02*.png`) dan
`/tmp/provider019/probe-node-after.ts` (sesudah, bukti `/tmp/provider019/evidence-node-after.json` 5
kelompok nilai, tangkapan layar `shots-node-after/01..02*.png`). Keduanya membuat node sementara sendiri
(`openai-compatible`, base URL `https://llm.example.com/v1`), lalu menghapusnya lagi. Driver "sebelum"
mengukur halaman lama, jadi ia memang berjalan sebelum perbaikan; driver "sesudah" dijalankan ulang pada
build terakhir, sesudah dua pemecahan berkas di gerbang ukuran, dan hasilnya sama dengan jalannya sebelum
pemecahan.

| Aksi                       | Nilai terukur                                                                                                                                                        |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Sebelum: blok halaman      | urutan `Custom provider`, `Model catalog`, `Models this provider cannot route`, `Custom models`, `Aliases`, `Connections`, `Endpoint`; 13 heading termasuk dialog      |
| Sesudah: heading halaman   | `["OpenAI Compatible Details", "Connections", "Available Models", "Aliases"]`, dan tidak ada heading registry                                                          |
| Kartu detail               | `Chat Completions · https://llm.example.com/v1/chat/completions`, baris `Model prefix p019node/model`, fakta Base URL / Wire format / Created / Last changed, aksi Add API Key, Edit, Delete |
| Connections                | tabel menampilkan satu baris `Probe 019 node key`, `API key`, priority 1, `Active`, `1 of 1 healthy`, `Not tested`                                                     |
| Available Models           | baris `probe-019-node-model` dengan alamat `p019node/probe-019-node-model` dan tombol Copy                                                                             |
| Import from /models        | tombol aktif karena ada satu connection aktif; setelah diklik, `No models returned from /models.` (jawaban gateway `{"data":[]}`, bukan kegagalan baca)                 |
| Aliases                    | `No aliases yet`                                                                                                                                                      |
| Pembersihan                | 1 endpoint dan 1 model dihapus, `DELETE` node `204`; sisa `endpoints=1` (`ep-live` milik aktor lain) dan 3 node milik aktor lain, tidak ada yang tersentuh             |

### 9.3 Pengukuran 390 px dan dua tema (R-03, R-34)

Driver `/tmp/provider019/probe-mobile.ts` (Chrome headless + CDP, `Emulation.setDeviceMetricsOverride`
390x844 dengan `mobile: false`, karena `mobile: true` memberi viewport 652 px dan mengukur lebar yang
salah). Ia membuat satu node sementara berikut satu connection dan satu model, mengukur halaman node di
kedua tema, membuka dialog key dan mengukurnya, lalu mengukur halaman provider registry
(`/providers/openai`) di kedua tema, lalu menghapus semua yang dibuatnya. Bukti:
`/tmp/provider019/evidence-mobile-before.json` (sebelum F6 diperbaiki, hanya halaman node) dan
`/tmp/provider019/evidence-mobile.json` (sesudah, empat pengukuran), tangkapan layar
`/tmp/provider019/shots-mobile/01..05*.png`.

| Ukuran                                        | Sebelum (hanya halaman node) | Sesudah (node dan registry) |
| --------------------------------------------- | ---------------------------- | --------------------------- |
| `documentElement.scrollWidth` / viewport      | `652 / 390` | `390 / 390` |
| `pageOverflow`                                | 262 | 0 |
| `scrolledX` sesudah `window.scrollTo(400, 0)` | 262 | 0 |
| Elemen lolos scroller (non-`static`)          | 1 (`span.sr-only`, `right: 652`) | 0 |
| `bodyOverflow`                                | 0 | 0 |
| Dialog key                                    | 352 px di dalam 390, `dialogOverflow` 0 | sama |
| Tema terukur                                  | light, dark | light, dark (node), dark, light (registry) |
| Tabel lebar                                   | connections 832 px, models 704 px, `wrapperOverflowX: auto`, `wrapperScrollable: true` | sama, ditambah `wrapperPosition: relative` pada pembungkusnya |
| Halaman registry                              | tidak diukur | `/providers/openai`, `pageOverflow` 0, `scrolledX` 0, 40 elemen di luar viewport semuanya di dalam scroller |
| Pembersihan                                   | 1 model dan 1 endpoint dihapus, `DELETE` node 204, sisa `endpoints=1` | sama |

Halaman registry diukur hanya sesudah perbaikan; sebelum perbaikan hanya halaman node yang diukur, dan
satu elemen pelakunya sudah cukup untuk menamai sebabnya. Dua tabel lebar tetap lebih lebar dari viewport
dan tetap bisa digulir di dalam kotaknya masing-masing, bukan melebarkan dokumen.

## 10. Status per 2026-09-23

**CLOSED.** Enam temuan ditutup: F1 (form endpoint yang selalu ditolak wire), F2 (display name yang
reference tidak minta), F3 (kontrol add key di detail page, dibangun ulang mengikuti konsep reference
setelah owner menolaknya), F4 (sisa selisih dialog Add Compatible), F5 (halaman detail node custom yang
memakai bentuk provider registry, dibangun ulang ke bentuk node reference setelah owner menolaknya), dan F6
(label kolom tersembunyi yang melebarkan dokumen di 390 px, ditemukan oleh pengukuran gerbang pass ini
sendiri dan ditutup di pass yang sama). Satu baris tetap tidak dibangun di F4: tombol **Check**, yang
route-nya belum ada di `app-serv` dan sudah tercatat sebagai draft 017 F6. Angka pass: `app-ui/README.md`
bagian ke-26, register `docs/DRAFT/007-UI-ENDPOINT-READINESS.md` F12 naik ke dua belas dari dua puluh enam
bagian dan F13 ke sembilan baris penunjuk (jumlah klaimnya tetap sembilan belas). Commit lokal `app-ui`
saja, tanpa push, sesuai aturan owner. Pass ini mendarat sebagai dua commit lokal: `57125b6` (F1 sampai
F5) dan `9d7e4f2` (F6, lima pembungkus `relative` plus catatan ini), keduanya scope `app-ui` saja dan
belum di-push.

## 11. Koreksi lanjutan 2026-09-23: blok Aliases dihapus dari kedua bentuk halaman provider

Owner menunjuk dua teks di halaman detail provider: judul **Add an alias** ("kenapa di semua provider ada
ini?") dan kalimat target ("A target is a provider/model reference or a combo name. An alias that is already
in the table is changed here rather than added twice."), dengan putusan "seharusnya tidak ada alias seperti
ini, REFERENCE tidak ada seperti ini". Reference diukur sebelum apa pun dihapus, dan pengukuran pertamanya
salah dengan cara yang dicatat di sini: `grep -rn -i alias` yang dibatasi ke berkas `.svelte`, `.ts`, dan
`.tsx` mengembalikan **nol**, tetapi reference adalah aplikasi Next.js ber-ekstensi `.js`, jadi nol itu tidak
pernah bisa mengenai berkas yang seharusnya cocok. Diukur ulang atas seluruh pohon `origin/master`
(`21583c03`, tag `v0.5.85`), kata itu muncul di 144 berkas `open-sse/` dan 72 berkas `src/`, dan keduanya dua
hal yang berbeda. Di bawah `providers/**` kata itu adalah prefix provider sendiri
(`AddCustomModelModal.js:10` `providerAlias`, `:22-29` `stripAlias`), yang setara `prefix` panel dan bukan
sebuah tabel. Fitur model-alias reference yang sebenarnya ada di layar **CLI Tools**-nya
(`src/app/api/models/alias/route.js`, `src/lib/db/repos/aliasRepo.js`, pemetaan per-tool di
`cli-tools/components/AntigravityToolCard.js`), yang memetakan nama model sebuah CLI tool ke target; permukaan
itu tidak punya padanan di panel ini dan bukan blok yang owner tunjuk. Yang tidak dimiliki halaman detail
provider reference adalah section alias sama sekali: tiga kartunya adalah details card, Connections, dan
Available Models, sesuai pembacaan pass 26 atas `[id]/page.js:1447-1819`.

| Berkas | Sebelum | Sesudah |
| --- | --- | --- |
| `src/lib/components/ProviderAliases.svelte` | 149 baris | dihapus |
| `src/lib/components/ModelAliasForm.svelte` | 97 baris | dihapus |
| `src/lib/schemas/model-alias.ts` | 124 baris | dihapus |
| `src/lib/stores/model-alias.svelte.ts` | 130 baris | dihapus |
| `src/routes/providers/[provider_id]/+page.svelte` | dua section `Aliases`, satu di cabang node dan satu di cabang registry | keduanya dihapus, plus impornya |
| `src/lib/api/models.ts` | `listModelAliases` dan `replaceModelAliases` | keduanya dihapus, plus impor schema-nya |
| `tests/components/provider-aliases.test.ts`, `tests/schemas/model-alias.test.ts`, `tests/stores/model-alias.test.ts` | 41 blok `it()` dan `forEachCase`, yang menjalankan **71 kasus** (selisih terukur pada suite: 2605/154 menjadi 2534/151) | ketiganya dihapus |

Yang menyesuaikan supaya tidak ada sisa:

- `CombosTab.svelte` tidak lagi membaca set alias, jadi picker referensi hanya menawarkan id katalog dan nama
  combo di halaman itu.
- `ComboDeleteDialog.svelte` tidak lagi menunjuk layar yang sudah tidak ada; salinannya kini hanya pesan
  gateway, dan tesnya mengunci bahwa salinan itu tidak menyebut `Aliases` maupun `detail screen`.
- `tests/support/model-stub.ts` kehilangan rute `/models/aliases` beserta knob-nya (652 menjadi 600 baris).
- `ComboEditor.svelte`, `ComboModelRows.svelte`, `schemas/combo.ts`, dan `schemas/combo-test.ts` tidak lagi
  menyebut alias sebagai bentuk ref yang ditawarkan panel.
- SPEC-UI: bullet §6.3 diganti catatan penghapusan, §6.4 disesuaikan, baris tabel ref model menjadi "Model
  refs", baris berkas `model_alias.ts` dihapus, dan Q20 serta Q21 ditutup oleh penghapusan ini.

Yang tidak disentuh: route `GET`/`PUT /api/v1/models/aliases` di `app-serv` (milik tim lain). Alias yang
ditulis lewat API tetap resolve di gateway; panel hanya tidak lagi membaca atau menulisnya, sehingga route
itu kini tanpa konsumen di panel. Itu dicatat di sini untuk tim `app-serv`, bukan sebagai temuan baru.

### 11.1 Bukti gate

| Gerbang | Hasil |
| --- | --- |
| `bun run check` | 0 error, 0 warning; cache `.svelte-kit/.svelte-check` dibersihkan dulu karena masih memuat salinan berkas yang dihapus |
| Uji terarah | 10 berkas, 111 tes lulus, 4m44s: tiga berkas yang disunting, enam konsumen lain `tests/support/model-stub.ts`, dan `usage-topology-view.test.ts` |
| `bun run lint` | Prettier: semua berkas sesuai |
| `bun run lint:ts` | ESLint keluar 0 |
| `bun run build` | sukses, keluaran di `build/` |
| `bun run test` | **2534 tes lulus dari 151 berkas**, 1945 s, pada tree beku `fb731f74e3335bf77cedbfcd935588ca` (`md5sum` atas `src/`, `tests/`, `static/`, dan `scripts/`, 490 berkas, tujuh lebih sedikit dari 497 pass sebelumnya karena tujuh berkas dihapus). Jalan pertama pada tree yang sama merah di satu baris: timeout 5000 ms di `tests/schemas/usage-topology-view.test.ts`, berkas yang tidak disentuh pass ini, selagi build panel pass ini berjalan bersamaan; uji terarah di atas menjalankan berkas itu lagi dan hijau, jadi baris merah itu starvation host, bukan cacat. `README.md`, di luar sidik jari itu, disunting sesudah jalan hijau dan `lint` dijalankan ulang di tree akhir |
| Click-through browser | Satu jalan terekam terhadap panel yang menyajikan build pass ini di port 3001, diarahkan ke gateway tim yang hidup di `:9090`, dua bentuk halaman lewat alamatnya masing-masing: node `/providers/openai-compatible-0386BKG9Q4DYZPYC01VJH4C51G` (TH HARBOR 1, node milik owner) dan registry `/providers/openai`. Judul `h2` yang terbaca: node `OpenAI Compatible Details`, `Connections`, `Available Models`; registry `Model catalog`, `Models this provider cannot route`, `Custom models`, `Connections`. Tidak ada judul `Aliases` di keduanya, tidak ada teks `Add an alias` di keduanya, dan `performance.getEntriesByType('resource')` mencatat **0 permintaan** ke `/models/aliases` di kedua halaman. Kalimat `The set is global` juga tidak ada lagi di body registry. Dua tangkapan layar di `/tmp/alias-removal/shots/` |

Tidak ada baris temuan baru: ini penghapusan permukaan, bukan perbaikan cacat. Angka pass masuk ke
`app-ui/README.md` sebagai pass ketiga puluh.
