# 001-PORT-ENDPOINT-KEYS.md: porting permukaan Endpoint & Key (`app-serv` + `app-ui`) terhadap reference 9Router

Analisis porting pertama seri `docs/PORT/` (taksonomi di `docs/PORT/README.md`): item **CONFIGURE:
Endpoint & Key**. Dua pohon dibandingkan sekaligus, `app-serv` (gateway) dan `app-ui` (panel), karena
permukaan ini hidup di keduanya. Bukan kontrak; kontrak tetap `docs/SPEC-API/001-SPEC-API.md` (wire),
`docs/CONTRACT/001-CONTRACT-API-V1.yaml` (dokumen mesin), dan `docs/SPEC-UI/001-SPEC-UI.md` (perilaku
panel). Pola mengikuti `027` dan `028`: temuan bernomor F, bukti yang bisa diulang, dan keputusan owner
di depan implementasi.

| | |
|---|---|
| **Status** | **OPEN, dokumen analisis; implementasi sebagian 2026-09-24.** Temuan F1-F7 tetap berlaku. D3/D4 dicatat sebagai keputusan desain dan D5 dilaksanakan; D1/D2/D6/D7 terbuka. Pekerjaan permukaan (ikon aksi + pindah upstream endpoints ke Provider) commit `0c8fa07` (§7.2) |
| **Mechanism** | DURING & AFTER (antislop) |
| **Scope** | `app-serv/.` + `app-ui/.`, permukaan Endpoint & Key. Layar reference: `/dashboard/endpoint`, label sidebar yang sama (`Sidebar.js:21`) |
| **Permintaan owner** | "Full analisis dan perbandingan app-serv VS 9router (reference) pada Endpoint & UI: Endpoint & Key. Cek portingan secara presisi." |
| **Reference** | `decolua/9router`, checkout `/home/rusmanadodi/apps/9router`, **`39e36d3d` = `v0.5.85-4-g39e36d3d` (2026-09-23)**, tip yang sama dengan pengukuran draft 027/028/030 |
| **Kaitan** | `docs/PORT/README.md`; SPEC-API-001 §4, §7.3, §7.5; SPEC-UI-001 §6.2, §14 item 9; CONTRACT-API-V1; draft 017 §4.1b; draft 027; draft 028; draft 030 |
| **Tanggal** | 2026-09-24 |

---

## 1. Ringkasan

### 1.1 Cara membaca pemetaannya

Nama layarnya sama ("Endpoint & Key", baris pertama sidebar reference, `Sidebar.js:21`), tetapi isinya
bukan satu blok yang sama. Halaman reference memuat dua kartu: **API Endpoint** (alamat gateway dalam
tiga bentuk paparan: Local, Cloudflare Tunnel, Tailscale Funnel) dan **API Keys** (kredensial yang
CLI tool persembahkan ke gateway). Layar panel memuat dua tab: **Gateway keys** dan **Upstream
endpoints**. Pemetaannya:

1. **Tab "Gateway keys" adalah port dari kartu "API Keys"**, dengan hardening yang tertulis di
   SPEC-API §4 (plaintext sekali, sisanya hint) dan §7.3 (revoke terminal, nama UNIQUE).
2. **Tab "Upstream endpoints" tidak punya padanan halaman di reference.** Ia generalisasi yang
   SPEC-API §7.5 nyatakan sendiri (`001-SPEC-API.md:242`): "This generalizes the reference
   1-connection-1-key model". Padanan terdekatnya di reference adalah kartu Connections di halaman
   Provider (`providers/[id]/`), yang masuk item **CONFIGURE: Provider**, bukan item ini.
3. **Separuh kartu "API Endpoint" tidak diporting sama sekali**: paparan Tunnel dan Tailscale, gerbang
   keamanannya, dan tujuh route `/api/tunnel/*` tidak punya jejak di `app-serv` maupun `app-ui`
   (F3). Yang ada hanya alamat dasar read-only (`ApiBaseDialog`).

### 1.2 Angka

| | Reference `39e36d3d` | Kita |
|---|---|---|
| Berkas permukaan | 12 berkas, 1.723 baris (`endpoint/` 8 berkas 1.459 + `api/keys` 2 berkas 100 + `apiKey.js` 98 + `machineId.js` 66) | `app-serv` 38 berkas non-test, 5.578 baris; `app-ui` 25 berkas, 2.776 baris |
| Handler route | 5 handler kunci (2 berkas `route.js`) + 7 handler tunnel | 17 route terdaftar: 5 gateway-keys (`router.go:107-111`) + 12 endpoint family (`router.go:139-150`) |
| Konsep kunci | 1 kunci = 1 baris `apiKeys`, plaintext disimpan, `isActive` boolean, hard delete | 1 kunci = agregat `GatewayKey` (hash + hint, status tiga nilai, revoke terminal); 1 endpoint = 1 akun berisi 1..N kunci |

Perintah pengukurannya ada di §2.2.

### 1.3 Temuan

| # | Tingkat | Isi singkat |
|---|---|---|
| **F1** | MEDIUM | Kalimat SPEC-UI §6.2 "the row leaves the active list" tidak benar di pohon: baris revoked tetap terdaftar, dan tombolnya (Enable, Revoke) menjawab 409 |
| **F2** | MEDIUM | Tiga field parity koneksi (`default_model`, `global_priority`, `proxy_pool_id`) diterima API dan sedang ditambahkan ke kontrak, tetapi tidak punya permukaan panel |
| **F3** | MEDIUM | Separuh kartu "API Endpoint" (Tunnel, Tailscale, gerbang keamanannya, 7 route) tidak diporting dan belum ada keputusan tercatat |
| **F4** | LOW | Dua perilaku kunci reference tanpa keputusan tercatat: kunci terikat mesin (`machineId` + CRC) dan auto-provision "Default Key" |
| **F5** | LOW | Pertanyaan enum status gateway key (SPEC-UI §14 item 9) sudah terjawab di kode dan kontrak, tetapi tiga permukaan dokumen masih membacanya terbuka |
| **F6** | LOW | Kolom `created` yang §6.2 minta tidak ada di tabel gateway keys; `revoked_at` diparse tapi tidak pernah dirender |
| **F7** | LOW | Kontrak tidak mendeklarasikan `page`/`per_page` untuk `GET /api/v1/gateway-keys`, padahal handler memparse dan panel mengirimnya |

---

## 2. Metode dan bukti

### 2.1 Revisi reference

```
cd /home/rusmanadodi/apps/9router
git log -1 --format='%H %ci'   # 39e36d3d0c849e0e01dfeacddf111edf892448fc 2026-09-23 10:05:33 +0700
git describe --tags            # v0.5.85-4-g39e36d3d
```

Semua sitasi reference dibaca lewat `git show origin/master:<path>` pada tip ini, kebiasaan yang
menangkap drift sebelumnya. Setiap nomor baris reference di bawah diverifikasi ulang dengan
`grep -n` pada tip hari ini; dua sitasi warisan memang bergeser (Default Model kini `:297`, Proxy
Pool kini `:375-389`) dan yang dikutip di sini adalah angka yang terverifikasi.

### 2.2 Cara mengukur

```
# Skala reference
cd /home/rusmanadodi/apps/9router
wc -l "src/app/(dashboard)/dashboard/endpoint/"*.js \
      "src/app/(dashboard)/dashboard/endpoint/components/"*.js \
      src/app/api/keys/route.js "src/app/api/keys/[id]/route.js"          # 1559
git show origin/master:src/shared/utils/apiKey.js | wc -l                   # 98
git show origin/master:src/shared/utils/machineId.js | wc -l                # 66

# Skala kita (working tree 2026-09-24)
cd /home/rusmanadodi/pannelAI
wc -l app-serv/internal/{handler/gateway_key.go,handler/endpoint*.go, \
     service/gateway_key.go,service/gateway_key_masking.go,service/key_use.go, \
     service/endpoint*.go,schema/endpoint*.go,domain/gateway_key*.go, \
     domain/upstream_endpoint*.go,domain/upstream_key*.go, \
     repository/postgres/endpoint*.go,repository/postgres/gateway_key*.go}   # 5578
wc -l app-ui/src/routes/endpoint-keys/+page.svelte app-ui/src/lib/components/{GatewayKeysTab, \
     UpstreamEndpointsTab,EndpointTable,EndpointDetailDrawer,EndpointKeysTable,EndpointFilters, \
     EndpointFieldsForm,AddEndpointKeyForm,AddEndpointKeysPanel,BulkAddKeysForm,CreateEndpointForm, \
     CreateGatewayKeyForm,GatewayKeyRow,OneTimeKeyModal,AddProviderKeysDialog}.svelte \
     app-ui/src/lib/api/{endpoints,gateway-keys}.ts \
     app-ui/src/lib/schemas/{endpoint,endpoint-bulk,endpoint-options,endpoint-search,endpoint-write,gateway-key}.ts \
     app-ui/src/lib/utils/routing.ts                                        # 2776

# Jejak yang dicari dan tidak ada
grep -rn "tunnel\|tailscale" app-serv/internal app-serv/cmd app-ui/src       # 0 baris
```

### 2.3 Set in-flight yang dikecualikan

Working tree 2026-09-24 membawa pekerjaan aktor lain. Semua di bawah ini **dikecualikan** dari temuan
ini, dan sitasi yang menyentuh berkasnya dibaca pada keadaan hari ini:

| Berkas | Status | Isi |
|---|---|---|
| `docs/CONTRACT/001-CONTRACT-API-V1.yaml` | `M` (190 sisipan) | Aktor lain menambahkan `default_model`, `global_priority`, `proxy_pool_id`, `consecutive_use_count`, `EndpointError` ke skema endpoint; tidak ada hunk yang menyentuh blok yang dikutip dokumen ini (`:180-236`, `:3979-4100`) |
| `app-serv/internal/{registry/pricing*,service/chat_record_cost*,repository/*quota*}` dan `service/quota_flush_*`, `schema/systemone.go` | `??`/`M` | Pekerjaan pricing/quota/systemone; di luar permukaan ini |
| `docs/DRAFT/017…`, `027…`, `029…` | `??` | Draft pass lain, belum ter-commit |

### 2.4 Batas yang sudah dinyatakan (tidak dihitung temuan)

- "Upstream endpoints" sebagai generalisasi 1-connection-1-key adalah keputusan spesifikasi, bukan
  celah (`001-SPEC-API.md:242`).
- Kartu Connections reference (Test One-by-One, Apply Proxy, Round Robin toggle, prioritas panah)
  milik item **CONFIGURE: Provider** (`docs/PORT/README.md`), bukan item ini; yang dibandingkan di
  sini hanya bagian yang dipakai layar `/endpoint-keys`.
- Hardening sekali-lihat kunci gateway adalah §4 (`001-SPEC-API.md:97`), jadi bedanya dengan
  reference yang menyimpan plaintext bukan temuan.

---

## 3. Ledger porting

Status: **1:1** (perilaku setara atau lebih ketat dengan alasan tertulis), **dipetakan** (ada
padanannya di permukaan lain, dengan keputusan tertulis), **kurang** (ada di reference, tidak ada di
kita, tanpa keputusan), **beda** (perilaku berbeda; sengaja bila tertulis, terbuka bila belum).

### 3.1 Peta besar

| Blok reference | Padanan kita | Status |
|---|---|---|
| Sidebar "Endpoint & Key" (`Sidebar.js:21`) | `/endpoint-keys`, judul dan kalimat pengantar (`+page.svelte:20-23`) | 1:1 |
| Kartu "API Keys" (`EndpointPageClient.js:966-1146`) | Tab "Gateway keys" (`GatewayKeysTab.svelte`) | 1:1 dengan hardening (§3.2) |
| Kartu "API Endpoint", baris Local (`:730-737`) | `ApiBaseDialog` di header panel (`PanelHeader.svelte:38,68`) | dipetakan, kita lebih benar (§3.3 baris 1) |
| Kartu "API Endpoint", baris Tunnel + Tailscale (`:738-913`) | tidak ada | **kurang, F3** |
| Gerbang keamanan paparan (`:916-963`) | tidak ada | **kurang, F3** |
| Toggle "Require API key" di layar (`:978-989`) | Settings, Security (`SettingsSecurityTab.svelte:97-105`) | dipetakan |
| Padanan upstream endpoints (kartu Connections Provider) | Tab "Upstream endpoints" + §7.5 | dipetakan ke item Provider (§3.4) |

### 3.2 Kartu "API Keys" vs tab "Gateway keys"

| # | Item | Reference (`git show origin/master`, `39e36d3d`) | Kita | Status |
|---|---|---|---|---|
| 1 | List | `GET /api/keys` mengembalikan semua baris, tanpa paginasi (`api/keys/route.js:8-14`; `apiKeysRepo.js:14-17`, `ORDER BY createdAt ASC`) | `GET /api/v1/gateway-keys`, paginasi `page`/`per_page`, terbaru dulu (`handler/gateway_key.go:37-53`; `postgres/gateway_key.go:67-76`) | dipetakan (aturan paginasi §4, `001-SPEC-API.md:100`) |
| 2 | Create | `POST /api/keys` `{name}`, nama wajib, jawab 201 `{key, name, id, machineId}` (`api/keys/route.js:16-40`) | `POST /api/v1/gateway-keys` `{name}` (1..120), jawab 201 dengan `plaintext_key` sekali (`handler/gateway_key.go:56-72`; `dto.go:83-86,98-108`) | 1:1 |
| 3 | Read satu | `GET /api/keys/[id]` (`[id]/route.js:5-22`) | `GET /api/v1/gateway-keys/{id}` (`handler/gateway_key.go:74-87`) | 1:1 |
| 4 | Update | `PUT /api/keys/[id]` `{isActive}` saja, tanpa rename (`[id]/route.js:24-46`) | `PATCH /api/v1/gateway-keys/{id}` `{name?, status?}`, status enum `active\|disabled` (`dto.go:88-93`) | dipetakan: boolean menjadi status tiga nilai; rename adalah tambahan kita |
| 5 | Delete | hard `DELETE` (`[id]/route.js:48-57`) | soft revoke, terminal, mencatat `revoked_at` (`service/gateway_key.go:122-132`; `postgres/gateway_key.go:159-166`) | beda (sengaja): §7.3 `001-SPEC-API.md:182` "Revoke (soft)" |
| 6 | Nilai kunci | plaintext disimpan dan bisa ditampilkan ulang (show/hide + copy, `EndpointPageClient.js:1018-1037`; `apiKeysRepo.js:68-73` membandingkan nilai) | hash SHA-256 + `key_hint`; plaintext tidak bisa dipulihkan (`domain/gateway_key.go:41-63`; `postgres/gateway_key.go:121-134` mencari berdasar hash) | beda (sengaja): §4 `001-SPEC-API.md:97` |
| 7 | Format kunci | `sk-{machineId}-{keyId}-{crc8}`, CRC8 = HMAC-SHA256 atas machineId+keyId dengan `API_KEY_SECRET` (`apiKey.js:22,30,37`); machineId = `sha256(raw + salt)[0:16]` (`machineId.js:36-40`) | `GATEWAY_KEY_PREFIX` (default `sk-`) + 48 karakter acak dari alfabet tanpa ambigu, rejection sampling (`config/config.go:74`; `service/gateway_key.go:27-31,134-170`) | beda; pengikatan mesin tidak diporting, **F4** |
| 8 | Nama unik | tidak ada pengecekan (`createApiKey` menyisip begitu saja) | index UNIQUE + `CONFLICT`, terbukti integrasi (`migrations/000002_gateway_keys_name_unique.up.sql:10`; `gateway_key_constraint_test.go:33-40`) | kita lebih ketat: §4 `001-SPEC-API.md:130` |
| 9 | Pemakaian per kunci | tidak ada di baris kunci; pemakaian per API key dihitung dari riwayat di layar /usage (`usageRepo.js:91-93`) | `request_count` + `last_used_at` naik pada setiap panggilan data plane ter-autentikasi (`service/key_use.go:46-54`; `service/chat.go:118-144`) | dipetakan |
| 10 | Enforce data plane | `settings.requireApiKey` default `true` (`settingsRepo.js:27`), dicek per handler, 401 "Missing/Invalid API key" (`chat.js:69-80`) | `security.require_api_key` default `true` (`domain/settings.go:193`), satu titik `Authenticate`, 401 bahasa Inggris, alasan tidak bocor (`service/chat.go:118-144`) | 1:1, kita lebih ketat (read settings gagal dianggap wajib) |
| 11 | Auto-provision | saat daftar kosong, halaman minta `POST /api/keys {name: "Default Key"}` sendiri (`EndpointPageClient.js:266-276`) | tidak ada; panel menampilkan empty state "Create one to let a CLI tool reach the gateway" (`GatewayKeysTab.svelte:91-96`) | beda tanpa keputusan, **F4** |
| 12 | Modal kunci baru | "Save this key now!" + "This is the only time you will see this key", copy + Done (`:1113-1146`) | `OneTimeKeyModal`: kalimat §6.2, copy yang melaporkan penolakan, dismissal terkunci sampai ack atau copy sukses (`OneTimeKeyModal.svelte:36-58`) | 1:1, kita lebih ketat |
| 13 | Masking baris | `maskKey` 6 depan + titik + 4 belakang, bisa dibuka ulang (`:685-697`) | `key_hint` saja (`GatewayKeyRow.svelte:88`), tidak ada show/hide karena tidak mungkin | dipetakan (akibat #6) |
| 14 | Toggle akses | "Require API key" di layar ini, PATCH `/api/settings` (`:978-989,243-254`), plus peringatan host remote (`:991-995`) | `require_api_key` di Settings Security (`SettingsSecurityTab.svelte:97-105`), ditulis lewat `PATCH /api/v1/settings` | dipetakan |

### 3.3 Kartu "API Endpoint" vs panel

| # | Item | Reference | Kita | Status |
|---|---|---|---|---|
| 1 | Baris Local | `{window.location.origin}/v1`, read-only + copy (`:699-706,730-737`) | `ApiBaseDialog` tiga tab (Base URL / cURL / OpenAI), alamat dibaca dari server panel, bukan dari `location.origin` (`ApiBaseDialog.svelte:59-63`; `api-base.ts:3-11`) | dipetakan; kita sengaja berbeda karena alamat bind-all tidak bisa dipanggil klien |
| 2 | Baris Tunnel | enable/disable Cloudflare Tunnel, empat keadaan render, ping kesehatan, progress unduhan (`:738-829,287-378`) | tidak ada | **kurang, F3** |
| 3 | Baris Tailscale | install + connect + funnel enable, log progres, popup auth (`:830-913,401-623,1209-1292`) | tidak ada | **kurang, F3** |
| 4 | Gerbang keamanan | tunnel menolak aktif selama `requireApiKey` mati atau login tidak aman (`:814-824,901-907`); banner peringatan (`:916-949`) | tidak ada padanannya di layar mana pun | **kurang, F3** |
| 5 | `tunnelDashboardAccess` | toggle PATCH `/api/settings` (`:951-963,230-241`) | tidak ada | **kurang, F3** |
| 6 | Route tunnel | 7 berkas `src/app/api/tunnel/*`; `tunnel` di `PROTECTED_API_PATHS` (`dashboardGuard.js:68`) dan di `LOCAL_ONLY_PATHS` (`:76-81`) | tidak ada | **kurang, F3** |
| 7 | Ping kesehatan klien | `endpointPing.js` menyentuh `/api/health` dengan timeout dan miss-debounce (`endpointPing.js:6-29`; `endpointConstants.js:8-18`) | tidak ada | **kurang, F3** |

### 3.4 Upstream endpoints: §7.5 vs reference

| # | Item | Reference | Kita | Status |
|---|---|---|---|---|
| 1 | Konsep | 1 koneksi = 1 kunci; N kunci berarti N koneksi (`AddApiKeyModal.js:148-182`, satu POST per baris) | 1 endpoint = 1 akun berisi 1..N kunci (`service/endpoint_keys.go:46-62`), batas 100 (`service/endpoint.go:35`) | dipetakan: generalisasi tertulis §7.5 (`001-SPEC-API.md:242`) |
| 2 | Bulk | paste N kunci, N POST terpisah, ringkasan `✓ N added, ✗ M failed` (`:148-182,216-217`) | `POST /endpoints/bulk` dan `POST /endpoints/{id}/keys/bulk`, all-or-nothing, verdict per baris (`schema/endpoint.go:131-148`; `service/endpoint_keys.go:164-188`; panel `BulkAddKeysForm.svelte:58-107`) | dipetakan, kita lebih ketat (§8.1) |
| 3 | Prioritas | panah naik/turun per koneksi | angka prioritas + reorder transactional (`service/endpoint.go:151-196`) | dipetakan |
| 4 | Health kunci | tidak ada parking per koneksi; failover ditangani loop akun (draft 028) | parking per kunci dengan jendela per kelas (`001-SPEC-API.md:274-279`; `domain/upstream_key_health.go`) | kita lebih ketat |
| 5 | Test | `POST /api/provider-nodes/validate` untuk kredensial yang belum tersimpan (`AddCompatibleModal.js`, draft 017 F6) | `POST /endpoints/{id}/test` untuk endpoint tersimpan, `{key_id?}` opsional, 15 detik (`service/endpoint_probe.go:38`) | beda: probe butuh endpoint tersimpan; celah Check tercatat di draft 017 §4.6 F6 |
| 6 | Default Model + Proxy Pool | dialog koneksi merender keduanya (`AddApiKeyModal.js:297`, `:375-389`) | API menerima ketiga field parity (`schema/endpoint.go:107-109`; `service/endpoint_parity.go:46-89`); panel tidak menulis satupun | **kurang, F2** |
| 7 | Urutan kandidat | `fill-first` / `round-robin` per provider, override per provider di settings (draft 030) | sama, override per provider (`001-SPEC-API.md:232-240`) | 1:1 (draft 030) |

### 3.5 Wire: tabel route ke route

| Reference | Kita | Status |
|---|---|---|
| `GET /api/keys` | `GET /api/v1/gateway-keys` | dipetakan (paginasi) |
| `POST /api/keys` | `POST /api/v1/gateway-keys` | 1:1 + hardening |
| `GET /api/keys/[id]` | `GET /api/v1/gateway-keys/{id}` | 1:1 |
| `PUT /api/keys/[id]` | `PATCH /api/v1/gateway-keys/{id}` | dipetakan |
| `DELETE /api/keys/[id]` | `DELETE /api/v1/gateway-keys/{id}` | beda (soft revoke) |
| (tidak ada) | `POST /api/v1/endpoints/bulk` | tambahan kita (§8.1) |
| (tidak ada) | `POST /api/v1/endpoints/{id}/keys/bulk` | tambahan kita (§8.1) |
| (tidak ada) | `POST /api/v1/providers/{id}/oauth/bulk` | tambahan kita (§7.5) |
| `POST /api/provider-nodes/validate` | `POST /api/v1/endpoints/{id}/test` | beda (endpoint tersimpan) |
| 7 route `/api/tunnel/*` | tidak ada | **F3** |
| `PATCH /api/settings {requireApiKey}` | `PATCH /api/v1/settings {security.require_api_key}` | dipetakan |

---

## 4. Temuan

### 4.1 F1 (MEDIUM): baris revoked tetap terdaftar dan kontrolnya menjawab 409

Kalimat spesifikasi: "revoking is a soft delete and **the row leaves the active list**"
(`001-SPEC-UI.md:255`). Pohonnya tidak memenuhinya:

- `GET /api/v1/gateway-keys` tidak menyaring apa pun: `postgres/gateway_key.go:71-76` memilih semua
  baris tanpa `WHERE` status, dan handler hanya memparse `page`/`per_page`
  (`handler/gateway_key.go:37-42`).
- Panel merender semua baris yang dibaca (`GatewayKeysTab.svelte:112-119`).
- Untuk baris yang statusnya bukan `active`, `GatewayKeyRow.svelte:100-107` tetap menawarkan
  **Enable** dan **Revoke** (turunan `active` di `:27`). Keduanya menjawab `409 CONFLICT`:
  `Transition` menolak status apa pun dari `revoked` (`domain/gateway_key.go:124-126`) dan `Revoke`
  juga (`:136-138`), keduanya lewat `ErrGatewayKeyRevoked = NewConflictError("gateway key is already
  revoked")` (`domain/errors.go:114`). **Rename** pada baris revoked tetap berhasil
  (`domain/gateway_key.go:108-116` tidak memeriksa status), jadi barisnya setengah hidup.
- Kontrak mendeklarasikan `status` respons sebagai string polos (`CONTRACT:191-223`), jadi `revoked`
  memang nilai yang sah untuk dibaca; yang tidak ada adalah keputusan tentang **di mana** baris itu
  ditampilkan.

Opsi:

- **(a) Route menyaring `revoked` secara default**, plus parameter `?status=` untuk audit, kontrak dan
  panel mengikutinya. Kalimat §6.2 tetap benar, dan panel tidak perlu mengubah render. Biaya: satu
  perubahan route + kontrak + test.
- **(b) Panel merender baris terminal tanpa aksi** (Disable/Revoke hilang, Rename boleh), dan §6.2
  diamandemen menjadi "the row stays listed as revoked, without actions". Biaya: satu komponen panel +
  satu kalimat spec.
- **(c) Amend kalimat saja** tanpa mengubah apa pun. Termurah, tetapi membiarkan dua tombol yang
  selalu gagal 409 tetap di layar.

Rekomendasi: (a), karena mengikuti kalimat yang sudah tertulis dan reference (hard delete) juga
menghilangkan barisnya.

### 4.2 F2 (MEDIUM): tiga field parity koneksi punya API, tidak punya permukaan panel

`PATCH /api/v1/endpoints/{id}` menerima `default_model`, `global_priority`, `proxy_pool_id`
(`schema/endpoint.go:107-109`, komentarnya menunjuk draft 017 §4.1b), tervalidasi lewat
`service/endpoint_parity.go:46-89` (termasuk penolakan `proxy_pool_id` yang tidak menunjuk pool
tersimpan). Kontrak sedang ditambah skemanya oleh aktor lain (§2.3). Panelnya kosong:

- Satu-satunya penulis PATCH endpoint di panel adalah `EndpointFieldsForm.svelte:35-57`
  (label, priority) dan `:59-70` (status). Tidak ada `default_model`, `global_priority`, atau
  `proxy_pool_id` di mana pun: `grep -rn "default_model\|global_priority\|proxy_pool_id" app-ui/src`
  hanya menyentuh media-provider dan proxy-pool, konsep lain.
- Reference merender dua di antaranya di dialog koneksi: **Default Model** (`AddApiKeyModal.js:297`)
  dan **Proxy Pool** (`:375-389`, dengan empty state "No active proxy pools available").

Opsi: (a) tambahkan ketiga field ke drawer endpoint (`EndpointFieldsForm` + `endpoint-write.ts`), atau
(b) ke dialog koneksi di layar Provider (`AddProviderKeysDialog`), atau (c) nyatakan API-only di
SPEC-UI §6.2. Perhatikan: permukaan reference adalah dialog koneksi, jadi (b) yang paling setara;
tapi drawer endpoint adalah tempat satu endpoint diedit penuh di panel, jadi (a) yang paling
dapat ditemukan. Rekomendasi: (a), dengan catatan `proxy_pool_id` menyalin empty state reference
bila pool kosong.

### 4.3 F3 (MEDIUM): separuh kartu "API Endpoint" tidak diporting dan belum ada keputusan

`grep -rn "tunnel\|tailscale" app-serv/internal app-serv/cmd app-ui/src` mengembalikan **0 baris**.
Reference memuat: dua baris paparan dengan lima keadaan render masing-masing
(`EndpointPageClient.js:738-913`), gerbang keamanan sebelum enable (`:814-824,901-907`) dan tiga
banner sesudahnya (`:916-949`), toggle `tunnelDashboardAccess` (`:951-963`), tujuh route
`src/app/api/tunnel/*` (status, enable, disable, tailscale-check/-install/-enable/-disable),
perlindungan `LOCAL_ONLY_PATHS` untuk route yang menyentuh host (`dashboardGuard.js:72-81`), dan
ping kesehatan dari browser (`endpointPing.js`). Panel hanya membawa alamat dasar read-only.

Ini bukan celah kecil: di reference, separuh nilai halaman ini adalah "gateway saya bisa dipanggil
dari mana saja". Yang dibutuhkan bukan cuma UI, tapi mekanisme paparan di `app-serv` (proses
`cloudflared`/`tailscale`, status, keamanan).

Opsi: (a) port penuh (pekerjaan besar, melewati batas domain `app-serv`), (b) nyatakan **luar
cakupan** di SPEC-API §7 dan beri catatan di `docs/PORT/README.md`, (c) port sebagian: dokumentasikan
bahwa operator memasang tunnel sendiri, dan panel hanya menampilkan alamat paparan tambahan.
Rekomendasi: (b) atau (c); (a) hanya bila owner ingin paritas penuh di item ini, dan kalau begitu
itemnya layak jadi pass tersendiri.

### 4.4 F4 (LOW): dua perilaku kunci reference tanpa keputusan tercatat

1. **Pengikatan mesin.** Kunci reference menyandikan `machineId` dan CRC yang bisa diverifikasi
   (`apiKey.js:30,37` untuk formatnya, `:49-64` verifikasi CRC-nya; machineId dari
   `machineId.js:36-40`). Kunci kita opaque dan sah di gateway
   mana pun yang berbagi DB. Untuk gateway multi-host ini justru benar, tetapi keputusannya tidak
   tertulis di SPEC-API §7.3 maupun di dokumen mana pun.
2. **Auto-provision "Default Key".** Reference menciptakan kunci pertama sendiri saat daftar kosong
   (`EndpointPageClient.js:266-276`), sehingga endpoint langsung terpakai. Panel menolak melakukannya
   dan meminta operator membuat kunci (`GatewayKeysTab.svelte:91-96`). Membuat kredensial tanpa
   niat operator adalah perilaku yang wajar ditolak, tetapi tolakannya juga belum tercatat.

Opsi: (a) tulis dua keputusan ini sebagai penyimpangan sengaja di SPEC-API §7.3 (satu paragraf),
(b) port auto-provision, (c) port pengikatan mesin. Rekomendasi: (a); (b) dan (c) melawan arah
desain yang sudah diambil (§4 key masking, `app-serv` stateless).

### 4.5 F5 (LOW): pertanyaan enum status sudah terjawab, tiga permukaan dokumen masih membacanya terbuka

- Kode: `UpdateGatewayKeyRequest.Status` enum `active|disabled` (`dto.go:92`); pola sama untuk kunci
  endpoint (`schema/endpoint.go:126`).
- Kontrak: `GatewayKeyUpdateRequest.status` enum `active|disabled` (`CONTRACT:180-190`); sama di
  `EndpointKeyUpdateRequest` (`:2205-2209`).
- SPEC-API §7.3 baris PATCH tetap menulis `{name?, status?}` tanpa nilai (`001-SPEC-API.md:181`).
- SPEC-UI §14 item 9 masih terbuka dengan premis "SPEC-API §7.3 accepts `status` on PATCH but never
  lists the allowed values" (`001-SPEC-UI.md:1658-1662`).
- Panel mengulang premis itu sebagai alasan skema longgar: "SPEC-API §7.3 accepts `status` on PATCH
  but does not enumerate its values, and inventing an enum here would make the panel reject a value
  the API accepts" (`schemas/gateway-key.ts:3-6`). Arah kalimatnya sudah tidak akurat: API menolak
  nilai di luar `active|disabled` dengan `VALIDATION_ERROR`, jadi panel yang menyempitkan tidak akan
  menolak apa pun yang API terima.

Fix: satu baris di tabel §7.3 (`status` ∈ `active|disabled`; `revoked` hanya lewat DELETE), tutup
item 9, perbarui komentar `gateway-key.ts`. Skema longgar panel boleh tetap (merender nilai asing
verbatim adalah pilihan yang aman), hanya alasannya yang harus jujur.

### 4.6 F6 (LOW): kolom `created` yang §6.2 minta tidak ada; `revoked_at` diparse tapi tak dirender

SPEC-UI §6.2 tab 1: "Table: name, `key_hint`, status, last used, request count, **created**"
(`001-SPEC-UI.md:248-249`). Tabel panel merender Name, Key, Status, Requests, Last used, Actions
(`GatewayKeysTab.svelte:103-108`); `created_at` dan `revoked_at` diparse
(`schemas/gateway-key.ts:26-35`) dan tidak pernah muncul. Reference menampilkan "Created {date}" per
baris (`EndpointPageClient.js:1039-1041`).

Fix: tambahkan kolom Created (dan tampilkan `revoked_at` pada baris revoked, yang berguna sekalian
untuk F1), atau amend §6.2. Biaya terukur: satu kolom + satu test.

### 4.7 F7 (LOW): kontrak tidak mendeklarasikan `page`/`per_page` untuk `GET /api/v1/gateway-keys`

- Handler memparse keduanya (`handler/gateway_key.go:38`, `schema/dto.go:40-81`).
- Panel mengirim keduanya (`GatewayKeysTab.svelte:38`).
- Kontrak untuk route ini tidak punya blok `parameters:` sama sekali (`CONTRACT:3980-3993`), sementara
  `GET /api/v1/endpoints` mendeklarasikan `provider_id`, `status`, `page`, `per_page`
  (`CONTRACT:4945-4969`).
- Catatan perubahan §4 menyebut empat route yang parameternya dideklarasikan: `providers`,
  `endpoints`, `usage/records`, `logs/requests` (`001-SPEC-API.md:803`). `gateway-keys` adalah list
  paginasi kelima dan tidak disebut.

Fix: tambahkan dua parameter ke kontrak dan regenerasi `openapi.json` (pinned oleh gate generator,
jadi drift ini terdeteksi otomatis begitu ada deklarasi yang melenceng, tapi ketiadaan deklarasi
tidak terdeteksi oleh siapa pun).

---

## 5. Non-findings

Diperiksa dan sengaja berbeda, dengan alasan tertulis:

| Item | Kenapa bukan temuan |
|---|---|
| Reference menyimpan plaintext kunci dan bisa menampilkannya ulang; kita hash + hint | §4 `001-SPEC-API.md:97` memutuskan sekali-lihat |
| Hard delete (reference) vs soft revoke (kita) | §7.3 `001-SPEC-API.md:182` |
| Disable kunci tanpa konfirmasi di panel, sementara reference mengonfirmasi pause | Disable reversible; preamble §6 hanya mewajibkan konfirmasi untuk aksi destruktif, dan revoke tetap dikonfirmasi (`GatewayKeysTab.svelte:150-174`) |
| Membuat endpoint hanya lewat `?provider=` dari layar Providers | §6.3 yang memutuskan; komentar `CreateEndpointForm.svelte:5-8` menuliskan alasannya |
| "Which key would route now" tidak ada di reference | Tambahan yang §6.2 minta (`001-SPEC-UI.md:267-269`), diturunkan dari field API, bukan simulasi (`utils/routing.ts:1-31`) |
| Urutan list kunci: reference terlama dulu, kita terbaru dulu | Tidak ada klaim urutan di spec kedua sisi |
| Batas 100 kunci per endpoint, 50 baris per bulk | Batas kita, tertulis §7.5/§8.1 dan diulang di panel (`endpoint-write.ts:27,130`) |
| `machineId` tidak pernah dirender di halaman reference | Hanya `propTypes` (`EndpointPageClient.js:1308-1310`); tidak ada perilaku yang perlu diporting |
| Reference menampilkan tanggal dengan `toLocaleDateString`, panel menampilkan `last_used_at` mentah RFC3339 di kedua tabel | Inkonsistensi gaya render di dalam layar (bandingkan `GatewayKeyRow.svelte:91` dengan `EndpointKeysTable.svelte:60-66` yang memformat `rate_limited_until`); dicatat sebagai rabaan perbaikan, bukan temuan, karena tidak ada aturan format yang dilanggar |

---

## 6. Keputusan owner

| # | Pertanyaan | Opsi | Rekomendasi |
|---|---|---|---|
| D1 (F1) | Ke mana baris revoked pergi? | (a) route menyaring default + `?status=`; (b) baris terminal tanpa aksi + amend; (c) amend saja | (a) |
| D2 (F2) | Di mana ketiga field parity diedit? | (a) drawer endpoint; (b) dialog koneksi Provider; (c) API-only, dicatat di §6.2 | (a) |
| D3 (F3) | Tunnel/Tailscale diporting atau tidak? | (a) port penuh; (b) luar cakupan, dicatat; (c) parsial (alamat manual) | (b) atau (c) |
| D4 (F4) | Machine binding dan auto-provision? | (a) catat sebagai penyimpangan sengaja; (b) port auto-provision; (c) port machine binding | (a) |
| D5 (F5) | Enum status di spec? | (a) satu baris di §7.3 + tutup item 9 + perbaiki komentar schema; (b) biarkan | (a) |
| D6 (F6) | Kolom Created? | (a) tambah kolom + `revoked_at` pada baris revoked; (b) amend §6.2 | (a) |
| D7 (F7) | Parameter paginasi gateway-keys di kontrak? | (a) tambah + regenerasi `openapi.json`; (b) biarkan | (a) |

Tidak ada kode yang boleh ditulis untuk F1-F7 sebelum keputusannya dijawab.

Keputusan yang sudah diambil (arahan owner 2026-09-24): **D3 (b)** dan **D4 (a)**, karena owner menulis
"sisa port: Tailscale, dan lainnya tidak kita port, tandai saja sebagai keputusan design", dicatat di
SPEC-API §7.3 dan `docs/PORT/README.md` §3. **D5 (a)** dilaksanakan di pass yang sama karena API dan
kontrak sudah menegakkannya; SPEC-API §7.3 kini menulis `status` ∈ `active|disabled`, SPEC-UI §14 Q9
ditutup, dan komentar `gateway-key.ts` diperbarui. **D1, D2, D6, dan D7 tetap terbuka**, dan tidak ada
kode untuk F1/F2/F6/F7 yang ditulis di pass ini.

---

## 7. Status pass ini

- **Analisis selesai; tidak ada berkas source yang diubah.** Keluaran pass ini satu berkas:
  `docs/PORT/001-PORT-ENDPOINT-KEYS.md` (berkas kosong yang owner siapkan diisi, tanpa menimpa
  apa pun). `docs/PORT/` masih belum ter-commit; keputusan commit milik owner, sesuai pola pass
  analisis sebelumnya.
- Reference diukur pada `39e36d3d` = `v0.5.85-4-g39e36d3d` (tip, 2026-09-23); checkout tidak
  disentuh (hanya `git show`/`git grep`, tanpa pull).
- Setiap nomor baris reference diverifikasi `grep -n` pada tip hari ini; dua sitasi warisan dari
  catatan 2026-09-23 ternyata bergeser (Default Model `:295-302` kini `:297`; Proxy Pool `:374-389`
  kini `:375-389`) dan yang dikutip adalah angka yang terverifikasi.
- Set in-flight aktor lain dinyatakan di §2.3 dan tidak masuk temuan; hunk kontrak yang sedang
  berjalan tidak menyentuh blok yang dikutip.
- Tidak ada test dan tidak ada gate build yang dijalankan, karena pass ini tidak mengubah kode. Yang
  dijalankan adalah perintah pengukuran yang bisa diulang (§2.2), dan keluarannya dikutip apa adanya.

### 7.1 Gerbang (antislop, DURING & AFTER)

Pass analisis ini tidak menyentuh UI. Empat blok dilaporkan seluruhnya; item yang tidak berlaku dijawab
dengan alasannya, dan yang berlaku dijawab dengan bukti.

**Block 1: Hard Gate (17 item, semua harus "tidak")**

| Item | Jawaban | Bukti / alasan |
|---|---|---|
| R-02 em dash | **tidak** | `grep -cP '\x{2014}|\x{2013}' docs/PORT/001-PORT-ENDPOINT-KEYS.md` = 0 (dicek setelah penulisan) |
| R-03 overflow mobile | tidak berlaku | Tidak ada UI; keluaran berkas markdown |
| R-17 angka tanpa sumber | **tidak** | Setiap angka diikuti perintah yang menghasilkannya (§2.2) atau `file:line` yang bisa dibuka |
| R-18 testimoni fiktif | tidak berlaku | Tidak ada testimoni |
| R-23 aset tanpa instruksi | tidak berlaku | Tidak ada aset visual yang dibuat |
| R-24 navigasi mati | tidak berlaku | Tidak ada navigasi |
| R-25 kontras | tidak berlaku | Tidak ada teks ter-render |
| R-26 kontrol mati | tidak berlaku | Tidak ada kontrol; temuan tentang kontrol yang mati ada di pohon, bukan di dokumen ini |
| R-27 state UI | tidak berlaku | Tidak ada UI berdata |
| R-28 FAQ | tidak berlaku | Tidak ada FAQ |
| R-32 keyboard | tidak berlaku | Tidak ada UI |
| R-33 patch via script | **tidak** | Pass ini menulis satu berkas markdown dengan alat tulis; tidak ada skrip yang menyunting source |
| R-34 tema | tidak berlaku | Tidak ada tema |
| R-35 dijalankan sebelum diserahkan | tidak berlaku untuk UI | Tidak ada UI yang berubah; yang dijalankan adalah perintah pengukuran dan verifikasi sitasi (`grep -n`), keduanya dikutip |
| R-36 klaim fabrikasi | **tidak** | Setiap temuan menyitir berkas yang bisa dibuka di kedua pohon; tidak ada klaim keamanan/compliance |
| R-37 arah desain | tidak berlaku | Tidak ada UI yang dibangun |
| R-38 konten fabrikasi | **tidak** | Sitasi reference dibaca dari `39e36d3d` lewat `git show origin/master`; sitasi kita dari working tree yang dinyatakan di §2.3 |

**Block 2: Purpose-Gate (12 item)**

Seluruhnya **tidak berlaku** dengan satu alasan yang sama: deliverable ini dokumen analisis, tidak
memuat gradien, ikon, tipografi, latar bermotif, panah, badge, glassmorphism, shadow, glow, kartu,
animasi, atau ilustrasi. Tidak ada teknik visual yang dipakai, jadi tidak ada alasan yang perlu
ditulis. R-31 (alasan satu baris per keputusan) justru **dijalankan** pada tingkat isi: setiap
temuan menuliskan mengapa ia temuan, dan §6 menuliskan akibat tiap opsi.

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
| C-3 content-driven composition | **tidak** | Struktur mengikuti pemetaan tiga bagian di §1.1, bukan template |
| C-4 resilience | tidak berlaku | Tidak ada UI |
| C-5 evidence over claims | **tidak** | Tidak ada statistik tanpa sumber; dua sitasi warisan yang bergeser dilaporkan, bukan disembunyikan (§7) |
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

### 7.2 Implementasi 2026-09-24 (arahan owner)

- **Ikon aksi baris.** Kedua tabel (gateway keys dan endpoint keys) memakai tombol ikon-saja dengan
  `aria-label` dan `title` kata kerja aksi, glyph `aria-hidden`, target 44 px; peta ikon dan alasan
  satu baris per aksi ada di `app-ui/src/lib/icons.ts` (`ROW_ACTION_ICONS`), sesuai SPEC-UI §8.11.
- **"Upstream endpoints" pindah ke Provider.** Tab di `/endpoint-keys` dihapus bersama bilah filter,
  dua skema, dan empat berkas test (8 berkas, 1113 baris, 47 kasus). Tabel endpoint, drawer detail,
  tabel kunci, dan form tambah kini dirender di seksi Connections halaman Provider (SPEC-UI §6.3),
  dengan aksi tambah dipisah menurut auth type provider.
- **Keputusan desain dicatat.** SPEC-API §7.3 memuat paragraf keputusan untuk F3 (tunnel/Tailscale
  luar cakupan) dan F4 (tanpa pengikatan mesin, tanpa auto-provision); `docs/PORT/README.md` §3
  merangkumnya.
- **F5 dilaksanakan.** SPEC-API §7.3 menyebut `status` ∈ `active|disabled`; skema tulis panel
  menyempit ke dua anggota itu; SPEC-UI §14 Q9 ditutup.
- **Verifikasi.** Lima gate `app-ui` hijau dan click-through terekam (sembilan tangkapan, tanpa
  panggilan gagal, tanpa tulisan) terhadap build pass sendiri dan gateway dari HEAD; rincian di
  `app-ui/README.md`. Commit `app-ui` `0c8fa07`.
- D1, D2, D6, D7 tetap terbuka; tidak ada kode untuk F1/F2/F6/F7 di pass ini.
