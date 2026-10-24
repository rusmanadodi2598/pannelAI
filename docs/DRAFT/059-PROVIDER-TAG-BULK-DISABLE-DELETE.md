# 059-PROVIDER-TAG-BULK-DISABLE-DELETE.md: tag hidup di server, disable punya jendela adaptif, dan Select-All harus di-resolve di server

Permintaan owner, pass 1: **"Pada provider untuk key, atau akun: Sudah support tag (1 akun), dan bulk tag (semua
akun) untuk di disable sementara, atau dihapus secara permanent."** Lalu: *"Output hasilnya, jadikan dulu sebuah
DRAFT lanjutan supaya bisa dikerjakan secara `continue`."* Dan: *"Kita pakai Icon saja (None plain teks, dan None
Emoticon)."*

Jawaban owner, pass 2 (2026-10-16), yang mengubah bentuk rancangan:

1. **Q1 dijawab:** persistence di server side (`app-serv`). Dashboard UI cukup menjalankan fungsinya.
2. **Q2 dijawab:** adaptive. Default auto-aktif lagi dalam 24 jam, karena rata-rata key/akun - khususnya Free atau
   yang sedang event - reset tiap 24 jam. Beberapa case mengikuti window 5 jam: Gemini, Claude, OpenAI (Codex).
3. **Scope bertambah:** Proxy ikut Select-All.
4. **Syarat:** Select-All harus bekerja di semua provider - native, custom endpoint OpenAI-compatible, dan
   Anthropic-compatible.

Keputusan owner, pass 3 (2026-10-16): **ketiga pertanyaan §5 diterima persis seperti usulan** - plafon `all` 2000
baris, P0 memakai jendela policy yang deterministik sementara sambungan `resets_at` vendor tetap di P1, dan
`ProviderNode` serta `gateway_keys` keluar dari draft ini (bukan ditunda: kalau nanti diminta, itu draft tersendiri).
Owner juga mengotorisasi penghapusan `proxy/proxyscrape_premium_http_proxies.txt` ("proxy sudah mati, hapus juga tidak
masalah") - dilakukan, lihat §8.

Dokumen kerja atas permukaan provider (`upstream_endpoints`, `upstream_keys`) dan proxy (`proxies`) terhadap
reference `https://github.com/decolua/9router`. Bukan kontrak; kontrak tetap `docs/SPEC-API/001-SPEC-API.md` (wire)
dan `docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel). Pola mengikuti draft 027/028/029 (banding reference, sitasi dua
sisi) dan 046/050 (keputusan yang sudah diambil tidak diulang): temuan bernomor F, bukti yang bisa diulang,
keputusan bernomor D, dan urutan kerja yang bisa diambil alih pass berikutnya.

| | |
|---|---|
| **Status** | **OPEN, dokumen rencana - semua keputusan owner tertutup, kodenya belum.** Pass 1 mengoreksi premis (reference tidak punya tag). Pass 2 menutup Q1/Q2 (D1 tag persisten di server, D3 jendela adaptif) plus scope proxy (D8) dan cakupan lintas provider (D10). Pass 3 menutup tiga pertanyaan terakhir (§5) dan membersihkan satu catatan keamanan (§8). **Tidak ada pertanyaan yang tersisa untuk pass implementasi** - §6 adalah urutan kerja yang bisa langsung dikerjakan. Yang masih mengikat: dua cacat ownership (F13, F14) adalah **prasyarat P0-0**, sebelum satu pun rute bulk dibuka |
| **Mechanism** | BEFORE: disable/enable satu baris lewat `PATCH`, tidak ada seleksi, tidak ada jalur massal, tidak ada glyph tag, `disabled` tidak pernah menyala sendiri. AFTER: tag dan seleksi di-resolve server lewat satu selector; disable operator punya `disabled_until` yang dinilai jam-dinding saat dibaca tanpa job; Select-All + bulk disable/delete bekerja di endpoint, key, dan proxy, di semua bentuk auth |
| **Scope** | `app-serv/internal/{domain,repository/postgres,service,handler,schema,router}` permukaan endpoint + key + proxy, `app-serv/migrations` (2 kolom tag + 2 kolom jendela + index, settings policy), `app-ui/src/lib/{icons.ts,components/*}` + tabel provider dan proxy, `docs/SYSTEM_MAP.md` |
| **Permintaan owner** | tag per akun + bulk ke semua akun sebagai pemegang disable sementara dan hapus permanen; UI ikon saja, tanpa teks dan tanpa emotikon; proxy ikut; semua jenis provider harus kena |
| **Reference** | `decolua/9router`, checkout `/home/rusmanadodi/apps/9router`, **`87a74e1b` = `v0.5.85-150` (2026-10-15)**. Draft 027/029 mengunci `21583c03` = `v0.5.85`; semua sitasi di bawah berlaku untuk checkout hari ini |
| **Kaitan** | draft 046 (empat aturan cascade, diwarisi utuh), draft 050:337 (invarian tanpa sweeper) dan T1 (deadline jangan dikacaukan dengan gerbang serving), draft 052 (provenance park read-only), draft 057 (walk alamat proxy), SPEC-UI §8.7.2 §8.7.3 §8.10 §8.11.2 §8.11.4 §8.11.8 §8.11.9, SPEC-API §7.5 (CONFLICT key terakhir) dan baris 731 serta 1088, AGENTS.md §1.1 §1.2 §1.3 §1.5 §1.6 §1.7 §1.9 §2.1, `scrypts/gates/antislop.sh`, skill `antislop-code` |
| **Tanggal** | 2026-10-16 (pass 3) |

---

## 1. Ringkasan

Tiga hal berubah setelah pass 2, dan satu di antaranya adalah koreksi atas premis awal.

**Pertama, premisnya sudah dibenahi dan jawaban owner mengubah arah.** Reference tidak mendukung tag: grep
`tag|label|group|category` atas `src/lib/db/schema.js` keluar nol, dan `/api/tags` adalah endpoint daftar model
Ollama (`src/app/api/tags/route.js:1,13-17`). Yang reference dukung adalah seleksi checkbox atas banyak akun plus
`isActive` untuk disable reversibel dan `DELETE` keras untuk hapus permanen. Owner memutuskan tag **memang**
persisten dan place-nya di `app-serv` - jadi ini bukan port, ini konsep baru kita, dan panel hanya jadi pembaca.
Konsekuensi yang saya terima bersama keputusan itu: selector `all` dan selector `tag` dua-duanya **di-resolve
server**, bukan klien mengirim daftar id hasil iterasi DOM. Itu sekaligus menutup lubang paling mahal di reference
(§2 F6) secara struktural.

**Kedua, disable sementara kini punya jendela, dan repo ini sudah mengenal kedua angkanya.** `24h` adalah puncak
tangga kuota yang sudah ada (`domain/upstream_key_health.go:40-45`), dan **`5h` sudah menjadi jenis window kuota**
(`domain/quota.go:22` `QuotaWindowFiveHour`, `Duration() -> 5 * time.Hour` di `:56-57`, sudah di-CHECK di
`migrations/000007:49`). quotafetch bahkan sudah **melabeli** keluarga yang sama dengan 5 jam: "Gemini (5h)",
"Claude & GPT (5h)" (`internal/service/quotafetch/antigravity_weekly.go:183-184`). Yang tidak ada: 5 jam pernah
dipakai sebagai panjang *park*, dan jendela disable milik operator tidak ada sama sekali (grep
`disable_until|paused_until|maintenance|suspend` atas `internal/` + `migrations/` keluar nol, F15). Jadi yang perlu
dibangun adalah jembatan, bukan penemuan.

**Ketiga, scope proxy dan syarat "semua provider" memunculkan dua cacat ownership yang sudah ada di production dan
akan diperbesar oleh bulk**, bukan diciptakan olehnya. Sweep OAuth manual per provider memfilter hanya pada provider
dan `AuthType == oauth` tanpa filter status (`internal/service/oauth_flow_refresh_sweep.go:63`, dibaca langsung):
akun OAuth yang baru di-bulk-disable **terus membakar refresh grant-nya** di vendor. Dan `MarkUnhealthy` menulis
`status = UpstreamEndpointError` tanpa syarat (`internal/domain/oauth_refresh.go:58-60`, dibaca langsung) sementara
saudaranya `MarkHealthy` justru menjaga - "`disabled` is an operator decision, and no worker may vote it back into
rotation" (`:66-69`). Asimetri itu adalah bug yang komentarnya sendiri menjadi bukti aturannya: satu arah menghormati
kepunyaan operator, arah lain tidak. Bulk disable atas puluhan akun menabrak keduanya sekaligus, karena tabrakan yang
hari ini membutuhkan satu akun mati kebetulan tepat setelah di-disable menjadi kejadian yang diantisipasi, bukan
kecelakaan. Keduanya masuk P0 (F13, F14, D4).

Satu catatan keamanan di luar rencana ada di §8.

---

## 2. Temuan di reference (F1-F6)

**F1. Disable adalah kolom sendiri, bukan turunan status.** `providerConnections.isActive INTEGER DEFAULT 1`
(`src/lib/db/schema.js:41`) dengan index majemuk `(provider, isActive)` (`:49`); `apiKeys.isActive` (`:84`) tanpa
index. Tidak ada kolom `status`, `disabled`, atau `archived` di kedua tabel. Koersi int <-> boolean ditempatkan di
satu titik tiap arah (`connectionsRepo.js:46` baca `=== 1 || === true`, `:61` tulis `=== false ? 0 : 1`;
`apiKeysRepo.js:13,71,88`).

**F2. Yang bisa dinyalakan lagi, harus dibersihkan state penaltinya.** Saat akun diaktifkan lagi, health dan backoff
ikut dihapus (`connectionsRepo.js:15-33`). Keputusan bagus, diambil alih di D3.

**F3. Semua delete keras, reorder di dalam transaksi yang sama.** `connectionsRepo.js:279-290` menjalankan `DELETE`
lalu `reorderInTx` di dalam `db.transaction()`; `:292-297` bentuk tunggal; `apiKeysRepo.js:78-82` mengembalikan
`changes > 0`. Atomicity ada, tapi hanya per baris.

**F4. "Bulk" di reference adalah loop klien, bukan operasi server.** `providers/[id]/page.js:818-827` menghapus satu
per satu dengan `for (const id of idsToDelete) fetch DELETE` dan menghitung `failed`; `:982-1005`
`applyProxyAssignments` melooping `PUT`; `proxy-pools/page.js:219-240` `bulkSetActive` mengulang hal yang sama untuk
enable/disable massal. Tidak ada satu pun endpoint yang menerima daftar id: `PUT /api/keys/[id]` menerima
`{isActive, access}`, dan `PUT /api/providers/[id]` memakai whitelist field eksplisit
(`src/app/api/providers/[id]/route.js:113-122`) - bentuk whitelist-nya sehat, satu-barisnya bukan.

**F5. Bentuk seleksi massal yang diminta owner.** Checkbox per baris di luar kartu
(`providers/[id]/page.js:1031-1040`), "Select All" dengan label teks (`:1670-1680`), tombol massal ikon + teks +
hitungan (`:1529-1537` "Apply Proxy", `:1541-1550` `variant="danger"` "Delete Selected (N)"), muncul hanya saat
`> 0`. Untuk key tidak ada checkbox sama sekali (`endpoint/EndpointPageClient.js:1022-1101`): pause/resume berupa
toggle switch tanpa ikon dan tanpa label, maknanya hanya di `title` (`:1090`); delete ikon `delete` polos
(`:1093-1097`).

**F6. Empat cacat yang tidak boleh ikut pindah.**

- Judul modal memakai `connections.length`, bukan jumlah yang dipilih (`providers/[id]/page.js:1096`) - operator
  membaca angka yang salah sebelum mengonfirmasi penghancuran.
- `handleApplySinglePool` memetakan atas seluruh `connections`, bukan `selectedConnections` (`:1007-1010`).
- `bulkSetActive` menentukan target dengan `selectedIds.length > 0 ? selectedIds : proxyPools.map(...)`
  (`proxy-pools/page.js:220`, dibaca langsung): **seleksi kosong diam-diam mengenai seluruh tabel.**
- Delete di baris key disembunyikan sampai hover lewat `sm:opacity-0 sm:group-hover:opacity-100`
  (`EndpointPageClient.js:1093-1097`); disable ditandai teks "Paused" (`:1057`) dan `opacity-60` (`:1025`) -
  warna/opacity saja, yang SPEC-UI §8.11.8 larang.

---

## 3. Temuan di kita (F7-F21)

### Yang sudah ada

**F7. Disable reversibel sudah ada, dan `disabled` sudah menang atas jendela health.** Kosakata
`active|disabled|error` (`domain/upstream_key.go:22-26`, `upstream_endpoint_types.go:22-26`); `error` ditolak dari
wire (`ParseUpstreamEndpointStatus` `:71`, `IsValid()` `:42`). `Available()` (`domain/upstream_key_health.go:91-96`,
dibaca langsung) menutup pintu `disabled` sebelum sempat melihat jendela: *"A disabled key stays unavailable
regardless of the window, because that state comes from an operator rather than from health tracking."* Jendela
sendiri adalah state-nya - tidak ada job.

**F8. Tidak ada tag di lapisan mana pun.** Nol kolom, field, method, DTO, migrasi. `upstream_endpoints` punya tiga
kolom `jsonb` (`oauth`, `account`, `test_status`, `migrations/000005:8-27` + 000012/000016/000017/000018/000020),
tapi `account` adalah codec bentuk tetap dengan index ekspresi pada `account->>'email'` dan `->>'workspace_id'`
(`repository/postgres/endpoint_jsonb.go:36-118`, `migrations/000014`): menumpang tag di sana menabrak index itu.
`upstream_keys` tidak punya kolom jsonb sama sekali (`000005:33-52` + 000019/000021). `label` yang sudah ada di
kedua agregat adalah nama manusia dan unik per provider (`UNIQUE (provider_id, label)`), bukan tag: jangan dipakai
ulang untuk dua makna.

**F9. Tidak ada satu pun endpoint penerima daftar id, tapi pola kerjanya sudah matang.** Tiga rute bulk yang ada
(`POST /endpoints/bulk`, `POST /endpoints/{id}/keys/bulk`, `POST /providers/{provider_id}/oauth/bulk` di
`schema/endpoint.go:128-144,146,153-175`, terdaftar di `router/router.go:110-114,144-155`) semuanya **buat baru**.
Tidak ada bulk delete maupun bulk patch. Yang bisa diambil utuh: skema membatasi dengan
`validate:"required,min=1,max=50,dive"`, service memeriksa ulang plafon (`service/endpoint_bulk.go:23 maxBatchRows=50`,
`:43-47`), seluruh agregat dibangun di memori sebelum statement apa pun jalan, kegagalan dikaitkan ke indeks baris
nol-based (`BulkRowError`, `repository/postgres/endpoint_batch.go:26-44`, `service/endpoint_bulk.go:180-195`), semua
ditulis dalam **satu transaksi** via `inTx` (`repository/postgres/endpoint_write.go:42-56`), handler merender
`{error:{code,message}, results:[{index,id,error}]}` (`handler/endpoint_bulk.go:134-153`). SQL berbentuk himpunan
sudah ada: `endpoint_id = ANY($1)` (`repository/postgres/endpoint.go:185-194`) dan `IDsByProvider`
(`endpoint_batch.go:88`).

**F10. Cascade hapus node belum transaksional.** `NodeService.Delete` (`service/provider_node.go:202-223`, dibaca
langsung) menjalankan lima statement terpisah - dua guard, `DeleteByProvider`, `models.DeleteForProvider`,
`store.Delete` - tanpa transaksi, dan `invalidateOverlay` di luarnya. Guard combo/alias sudah benar mendahului
(`046`), key naik `ON DELETE CASCADE` skema (`000005:33-52`), riwayat (`usage_records`, `quota_windows`,
`request_logs`) sengaja tidak ikut. Bulk akan melipatgandakan gagal-separuh ini.

**F11. Aturan "key aktif terakhir" harus diuji atas himpunan, bukan per baris.** Guard-nya `UsesStoredKey()` =
`api_key || service_account` (`domain/upstream_endpoint_keys.go:52-53`, `upstream_endpoint_types.go:64-66`) - jadi
**tidak** berlaku untuk `oauth` dan `no_auth` yang menggendong kredensialnya di baris
(`upstream_endpoint_types.go:56-66`). Create memakai lantai yang sama (`service/endpoint_create.go:43-45`), dan
`refusesDeactivation` menerapkannya ke **disable** juga (`:65-73`). Jebakannya: dua key dari satu endpoint
ber-two-key lolos tes per-baris dan menghancurkan endpoint-nya. Server menghitung *enabled* sebagai
`status != disabled` (`:169-177`) sementara cermin UI menyaring `status === active`
(`app-ui/src/lib/utils/routing.ts:39,56`) - dua predikat berbeda; bulk wajib percaya server.

**F16. Per-provider duration SUDAH configurable, tapi satu jalur mengabaikannya.**
`settings_routing_health.go:208-218` `RoutingSettings.HealthFor(providerID)` menggabungkan `routing.key_park` global
(`settings_routing.go:157`) dengan `routing.provider_strategies["<id>"].key_park` / `.failure_classes`
(`:75-81,152-160`), disimpan sebagai satu baris jsonb per key di Postgres `settings` (`migrations/000008:29-33`,
`repository/postgres/settings.go:58-68`) - bukan `.env`, bukan `registry.yaml`. Bounds sudah ada
(`settings_routing_health.go:22-31`, `failure_class_policy.go:19-28`, plafon 30 hari di `failure_class.go:86`) dan
reader-nya sudah di-wire ke dataplane (`selection_ports.go:72-78`). Tapi `quotaStep`
(`upstream_key_health.go:49-57`) membaca package var dan **mengabaikan policy**, padahal `RestUntil`
(`settings_routing_rest.go:31-63`) menghormati vendor reset dan `Retry-After`. Jadi: tidak perlu penyimpanan atau
validasi baru untuk jendela disable (D3), dan jangan mengulangi bentuk setengah-kabel itu.

**F19. Proxy sudah punya disable reversibel - tanpa bulk dan tanpa kontrolnya.** Satu tabel `proxies`
(`migrations/000009:6-20`), **tidak ada tabel `proxy_pools`**: "pool" adalah himpunan `proxies`
(`schema/proxy.go:80-84`) dan **satu baris = satu alamat** (`host`, `port`, `protocol`, satu sealed secret, `:9-14`);
`DialURL` menyusun tepat satu endpoint per baris (`domain/proxy_rotation.go:133-139`). `enabled boolean NOT NULL
DEFAULT true` (`000009:14`, dibaca langsung) dengan `Proxy.SetEnabled` (`domain/proxy.go:165-168`) lewat
`PATCH /api/v1/proxies/{id}` (`schema/proxy.go:43`, `service/proxy.go:176-178`); kepakaiannya
`enabled && status.State != "fail"` (`proxy_rotation.go:113-115`). Park alamat hidup di **Redis TTL 2 menit**
(`repository/redis/proxy_rotation.go:31,80`, `ProxyFailureCooldown = 2m` di `proxy_rotation.go:38`), bukan kolom.
Rutenya semua tunggal (`router/router.go:181-186`); tidak ada satu pun metode himpunan di
`repository/proxy.go:21-41`. Di panel, `Enabled` adalah **sel teks**, bukan kontrol (`ProxyTable.svelte:86`).

### Yang tidak ada dan harus dibangun

**F13. Sweep OAuth manual per provider tidak menyaring status.** `listOAuthEndpoints`
(`service/oauth_flow_refresh_sweep.go:61-79`, dibaca langsung) membangun
`filter := repository.EndpointFilter{ProviderID: providerID}` lalu hanya menyaring
`endpoint.AuthType() == domain.UpstreamAuthOAuth` - **tanpa filter status**, di-loop sampai 50 halaman x 100. Dari
sana `refreshEndpoint` menerbitkan grant (`service/oauth_flow_refresh.go:135-186`) lewat `OAuthFlowService.Refresh`
yang tanpa endpoint id jatuh ke `sweepProvider` (`:129`). Kontras: worker latar aman, `Sweep` hanya pindah pada
`active` dan `error` sehingga `disabled` tidak pernah di-refresh (`service/oauth_refresh_worker.go:100-116`, dipatok
`oauth_refresh_worker_test.go:196-220`). Jalurnya tidak konsisten, dan bulk disable akun OAuth akan tetap
menghabiskan refresh grant vendor.

**F14. Dead-letter refresh dapat menimpa `disabled` milik operator.** `MarkUnhealthy`
(`domain/oauth_refresh.go:58-60`, dibaca langsung) menulis `e.status = UpstreamEndpointError` tanpa memeriksa status
berjalan, dan dapat dicapai dari `MarkRefreshDeadLetter` (`service/oauth_flow_refresh.go:195-204`). `MarkHealthy`
(`:68-74`) menjaga dengan `if e.status != UpstreamEndpointError { return }` dan komentar yang menyatakan aturannya:
`disabled` adalah keputusan operator dan tidak ada worker yang boleh mengembalikan akun itu ke rotasi. Aturan
ownership itu hanya ditegakkan satu arah.

**F15. Tidak ada jendela disable operator, dan 5 jam belum pernah menjadi panjang park.** Kolom "until" yang ada
saat ini semuanya punyanya health atau poller: `upstream_keys.rate_limited_until` (`000005:49`, ditulis `RecordFailure`
`upstream_key_health.go:130`, di-persist extend-only dengan `GREATEST`
`repository/postgres/endpoint_key_health.go:57-58`), `upstream_endpoints.rate_limited_until` (`000005:17`, hanya untuk
akun keyless, `domain/upstream_endpoint_account_health.go:40` dengan gerbang `ServesItsOwnCredential()` `:20-22`),
`upstream_endpoints.quota_cooldown_until` (`000017:22`, `MarkQuotaThrottled` dengan 15 menit tetap di
`domain/upstream_endpoint_quota_cooldown.go:22`, dan **ditegaskan bukan input `Available()`**
`upstream_endpoint.go:47-50` - itu persis pemisahan draft 050 T1), `oauth_retry_at` (`000018:28`, dasar 1 jam plafon
24 jam `domain/upstream_endpoint_oauth_retry.go:20-21`). Tangga park: 5m/30s/1h/6h/30d untuk kelas
(`failure_class.go:72-81`) dan 15m/1h/6h/24h untuk kuota (`upstream_key_health.go:40-45`). **Tidak ada 5h di
keduanya**, dan grep `disable_until|paused_until|maintenance|suspend` keluar nol.

**F17. Select-All akan jatuh ke daftar yang terpaginasi, dan "semua" tidak bisa berarti "yang termuat".**
`ProviderEndpoints.svelte:33` `PAGE_SIZE = 10` dengan muat `{provider_id, page, per_page}` (`:59-80`) dan
previous/next (`:132-158`) - **server-paginated**. `EndpointKeysTable.svelte:55` memakai `pageOf(keys, requestedPage)`
(`lib/schemas/local-page.ts:17-26`) dengan Previous/Next (`:169-182`) - **client-paginated atas key satu endpoint**,
karena barisnya dimuat utuh lewat `getEndpoint` (`EndpointDetailDrawer.svelte:65`) dan `EndpointService.ListKeys`
memaginasi di memori (`service/endpoint_keys.go:61-73`). `ProviderOAuthAccounts.svelte:116` menampilkan hasil
`GET /providers/{id}/oauth/status` yang dibatasi `oauthListPerPage = 100`, 50 halaman
(`service/oauth_flow_refresh_sweep.go:23,65-78`). Semua list di panel kini di-scope satu provider
(`ProviderEndpoints.svelte:61-64`); backend justru sudah bisa lintas provider
(`WHERE ($1 = '' OR provider_id = $1)`, `repository/postgres/endpoint.go:56-57`).

**F18. `EndpointFilter` tidak bisa mengungkapkan bentuk auth atau himpunan id.** Isinya hanya `ProviderID` + `Status`
(`repository/endpoint.go:22-25`). Padahal syarat owner poin 4 menuntut seleksi menjangkau key-backed, OAuth, dan node
custom sekaligus. Pola `= ANY($1::text[])` sudah dipakai di tempat lain (`service/endpoint_summary.go:35-38`), jadi
ekstensinya punya preseden.

**F20. Hapus proxy tidak dijaga apa pun, dan teks dialognya salah.** `Delete` adalah `DELETE` satu baris tanpa syarat
(`repository/postgres/proxy.go:133-142`), diangkat 1:1 oleh `service/proxy.go:207-209` dan `handler/proxy.go:77-87`:
tidak ada cek referensi, tidak ada nulling, tidak ada penolakan. Efeknya senyap:
`upstream_endpoints.proxy_pool_id` (`000012:35`, `text` polos tanpa FK, di-index `:42-43`, divalidasi saat tulis
`service/endpoint_parity.go:62-78`) tetap menggantung selamanya, dan pin `network.provider_proxies[p].pool_id`
bertahan sementara walk hanya melewatinya (SPEC-API baris 1088, `service/proxy_route_order.go:68-73`); panel
menampilkan id mati itu sebagai `${poolChoice} (missing)` (`ProviderProxyCard.svelte:60-70,125`).
`ProxyDeleteDialog.svelte:34` (dibaca langsung) menegaskan *"Deleting here does not change the outbound path"* - benar
untuk switch global, **salah untuk provider yang di-pin**. Bulk delete akan menggandakan yatim senyap ini dengan nol
umpan balik server.

**F21. Ejaan auth registry bukan ejaan wire, dan "Codex" bukan provider.** `UpstreamAuthType` di wire persis empat
nilai: `api_key | oauth | no_auth | service_account` (`domain/upstream_endpoint_types.go:34-43`, validasi `:46-54`).
Registry menulisnya `apikey` / `none`, dan banyak entri tidak menuliskan sama sekali karena **diturunkan** di
`registry/load.go:162-178` (`no_auth` -> `oauth` bila blok `oauth:` ada -> selain itu `api_key`). Normalisasi ke wire
hanya terjadi di batas HTTP (`schema/endpoint.go:33-50`, `registryAuthAliases`). **Sebuah selector tidak boleh
membandingkan string `auth_type` mentah antar lapisan.** "Codex" bukan id provider: ia hanya id model di dalam
`openai` (`registry.yaml:901, 2428-2473`). Keluarga yang owner maksud untuk window 5 jam adalah id provider `openai`
(`:500-513`), `claude` (`:9-64`), `gemini` (`:1086-1112`), dan sepupunya yang ber-cadence sama `gemini-cli`
(`:385-420`) serta `antigravity` (`:228`). Registry memuat 36 provider (`internal/registry/embedded_test.go:25`), 14
di antaranya `category: oauth`.

Node custom tinggal di **tabel yang sama** tanpa diskriminator: `upstream_endpoints.provider_id` hanya `text`
(`000005:9`) yang berisi `openai-compatible-<...>` atau `anthropic-compatible-<...>`
(`domain/provider_node.go:23-26,32-35`; id prefix dan sintetisnya di `registry/custom_node.go:23-26,141-171` dengan
`Category: "apikey"`, `AuthType: AuthAPIKey`, `Custom: true`). `buildEndpoint` menerima id apa pun yang di-resolve
indeks runtime (`service/endpoint_create.go:29-31`; indeks = registry + overlay node
`cmd/app-serv/provider_index.go:85-120`), dan menghapus node menghapus barisnya
(`endpoint_delete_by_provider_integration_test.go:6-8`). Jadi **syarat owner poin 4 sudah benar di level
penyimpanan**; yang rawan ada di paginasi (F17), filter (F18), dan ejaan auth (F21), bukan di tabel.

---

## 4. Keputusan desain (D1-D10)

**D1 - Tag persisten di `app-serv`; panel tidak memegang state seleksi sebagai kebenaran.** Jawaban owner poin 1.
Bentuknya mengikuti idiom rumah (jsonb ber-codec, bukan array, bukan tabel join polimorfik yang melanggar bahasa
universal §2.2 DDD):

```sql
-- migrations/000022_endpoint_tags.up.sql
ALTER TABLE upstream_endpoints ADD COLUMN IF NOT EXISTS tags jsonb NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE upstream_keys      ADD COLUMN IF NOT EXISTS tags jsonb NOT NULL DEFAULT '[]'::jsonb;
CREATE INDEX IF NOT EXISTS idx_upstream_endpoints_tags ON upstream_endpoints USING gin (tags);
CREATE INDEX IF NOT EXISTS idx_upstream_keys_tags      ON upstream_keys      USING gin (tags);
```

- **Mengapa GIN, bukan index ekspresi:** pertanyaannya "semua baris yang memuat tag X", bukan "satu nilai di posisi
  tetap". `account` memakai index ekspresi (`000014`) dan tidak bisa dipakai ulang tanpa menabraknya (F8).
- **Aturan §1.7:** kolom pencarian baru lahir bersama index-nya, tidak menyusul.
- **Value object** `Tag` dengan normalisasi lowercase-trim dan pola `^[a-z0-9][a-z0-9_-]{0,47}$`; plafon 16 tag per
  baris, 48 karakter per tag, **tolak jangan potong** - mengikuti cara reference membatasi `accessAllow`
  (`src/shared/constants/keyAccess.js:7-8,12`), bukan memotong diam-diam.
- **Tulis hanya lewat method agregat** (`AddTag`/`RemoveTag`/`ClearTags`) di root; tidak ada patch kolom mentah dari
  repository maupun handler (aggregate root sebagai mutation boundary, §2.2 DDD).
- **Resolve di server.** `all` dan `tag` dievaluasi di repository (containment `@>` ke GIN), bukan klien mengirim
  daftar id hasil iterasi DOM. UI tidak pernah lagi punya kesempatan "salah hitung" - ini penutup struktural untuk
  F6 cacat ketiga.
- **Daftar tag untuk picker** = distinct atas satu provider scope saja, tidak pernah seluruh tabel tanpa batas:
  `SELECT DISTINCT jsonb_array_elements_text(tags) ... WHERE provider_id = $1` dengan `LIMIT`.

**D2 - Aksi massal dialamatkan satu `BulkSelector` bertype; seleksi kosong adalah error, bukan wildcard.**

```go
// schema/endpoint_bulk_mutation.go
type BulkSelector struct {
    IDs        []string `json:"ids,omitempty"         validate:"omitempty,max=200,dive,uuid4"`
    Tag        string   `json:"tag,omitempty"         validate:"omitempty,max=48,tagname"`
    AuthType   string   `json:"auth_type,omitempty"   validate:"omitempty,oneof=api_key oauth no_auth service_account"`
    ProviderID string   `json:"provider_id,omitempty" validate:"omitempty,max=120"`
    All        bool     `json:"all,omitempty"`
}
```

Tepat satu dari `ids` / `tag` / `all` terisi; nol terisi -> `400 BULK_SELECTOR_REQUIRED`; lebih dari satu ->
`400 BULK_SELECTOR_AMBIGUOUS`. Tidak ada jalur di seluruh kode di mana seleksi kosong berarti "semua" - yang berarti
"semua" hanya `all=true` yang ditulis terang. `provider_id` kosong pada mode `all` berarti seluruh provider, native
maupun node (F17: SQL-nya sudah mendukung hari ini). `auth_type` memakai enum wire, bukan ejaan registry (F21), dan
membutuhkan perluasan `EndpointFilter` (F18). Plafon keras untuk himpunan yang ter-resolve:
`MaxBulkTargets = 2000` (keputusan owner pass 3), dan `all` yang mengenai lebih dari itu ditolak
`413 BULK_TARGET_TOO_LARGE` dengan pesan yang menyuruh operator menyempitkan ke provider atau tag. Ini aturan
no-unbounded §1.7, bukan kenakalan UI - plafonnya ada di service, bukan hanya di skema, sama seperti `maxBatchRows`
hari ini (`service/endpoint_bulk.go:23`).

**D3 - Disable operator mendapat jendela adaptif yang dinilai jam-dinding saat dibaca; tanpa job.** Jawaban owner
poin 2. Dua kolom baru, dan panjangnya diambil dari policy yang sudah ada (F16), bukan dari tabel baru:

```sql
-- migrations/000023_operator_disable_window.up.sql
ALTER TABLE upstream_endpoints ADD COLUMN IF NOT EXISTS disabled_until timestamptz;
ALTER TABLE upstream_keys      ADD COLUMN IF NOT EXISTS disabled_until timestamptz;
```

- **Default 24 jam** (`DisabledWindowDefault = 24 * time.Hour`), karena itulah ritme reset mayoritas free tier dan
  akun event - alasan owner, dan ia layak menjadi satu baris komentar konstrain di kode, bukan komentar hiasan.
- **5 jam untuk keluarga ber-cadence 5 jam** - angkanya bukan penemuan: `QuotaWindowFiveHour` sudah ada
  (`domain/quota.go:22` + `Duration()` `:56-57`) dan quotafetch sudah melabeli Gemini/Claude/GPT pada 5 jam
  (`quotafetch/antigravity_weekly.go:183-184`). Yang berubah hanya: angka itu kini boleh menjadi panjang park, bukan
  hanya jenis window kuota.
- **Sumbernya policy, bukan daftar nama di kode.** Satu field jendela disable ditambahkan pada
  `routing.provider_strategies["<id>"].key_park` yang sudah ada (`settings_routing.go:75-81,152-160`, di-resolve
  `HealthFor` `settings_routing_health.go:208-218`); seed 5h untuk `openai`, `claude`, `gemini`, `gemini-cli`,
  `antigravity` lewat baris `settings` (`000008:29-33`), **bukan** lewat `switch providerID`. Menyimpan di Postgres
  membuatnya bisa diubah panel tanpa deploy, dan menghindari jebakan setengah-kabel yang sudah dilakukan `quotaStep`
  (F16): **`quotaStep` harus ikut membaca policy dalam pass ini**, kalau tidak kita menambah satu lagi jalur yang
  pura-pura policy itu ada.
- **Tanpa sweeper, mengikuti invarian 050:337.** `Available()` menambah satu gerbang di atas yang sudah ada:
  `disabled` dengan `disabled_until` masih di depan masa -> tidak tersedia; `disabled` dengan jendela sudah lewat ->
  jendela itu sendiri yang menyatakan dirinya selesai, persis bagaimana park health diperlakukan hari ini
  (`upstream_key_health.go:82-90`). Baris tetap menyimpan `disabled` untuk audit sampai request sukses berikutnya
  membersihnya lewat jalan `RecordSuccess()` yang sudah ada (`:184`). Tidak ada jendela yang bisa "lupa dibebaskan",
  tidak ada worker.
- **Tetap satu makna per kolom.** Draft 050 T1 memisahkan deadline dari gerbang serving, dan `quota_cooldown_until`
  sudah ditegaskan **bukan** input `Available()` (`upstream_endpoint.go:47-50`). `disabled_until` adalah kebalikannya:
  ia **memang** gerbang serving, dan itu harus tertulis di header `@reason` berkas domain-nya supaya tidak dikira
  duplikat cooldown.
- **Re-enable membersihkan penalti (F2) dan tidak menyentuh jendela health.** Kembali `active` menghapus streak dan
  park milik health; `disabled_until` dihapus eksplisit saat enable, tidak dibiarkan basi.

**D4 - Dua cacat ownership diperbaiki lebih dulu; bulk tidak landed di atasnya.** F13 dan F14 adalah prasyarat, bukan
pekerjaan lanjutan, karena keduanya hanya berbahaya dalam jumlah - dan jumlahlah yang sedang kita tambah.

- `listOAuthEndpoints` wajib menyaring status sehingga `disabled` tidak lagi menerbitkan grant
  (`oauth_flow_refresh_sweep.go:63`), disamakan dengan `Sweep` worker yang sudah benar
  (`oauth_refresh_worker.go:100-116`).
- `MarkUnhealthy` wajib menghormati kepemilikan operator: `disabled` tidak boleh ditimpa menjadi `error`, menegakkan
  aturan yang sudah ditulis `MarkHealthy` di komentarnya sendiri (`oauth_refresh.go:66-69`).

Keduanya punya tes sendiri sebelum satu pun rute bulk dibuka.

**D5 - Hapus permanen tetap keras; yang baru hanya jangkauannya.** Tidak `deleted_at`, tidak arsip - reference juga
tidak punya (F3) dan tidak ada alasan yang membayarnya (nol `deleted_at` di seluruh tree). Empat aturan draft 046
diwarisi utuh (guard combo/alias mendahului erasure, key naik cascade skema tanpa DELETE manual, riwayat tidak pernah
ikut, urutan "yang boleh gagal paling akhir" tidak ditukar). Dua pekerjaan ikut wajib: cascade node satu transaksi
(F10) dan **hapus proxy memberi laporan referensi**, bukan yatim senyap (F20) - sebelum menghapus N proxy, server
mengembalikan jumlah endpoint yang menggantung dan pin provider yang ikut mati, dan operator mengonfirmasi angka itu.

**D6 - Bentuk wire: POST bermassa selector, mengikuti pola bulk-create, bukan `DELETE` ber-body.**

| Method + path | Body | Catatan |
|---|---|---|
| `POST /api/v1/endpoints/bulk-tags` | `{selector, add[], remove[]}` | add/remove sekaligus; tulis lewat method agregat (D1) |
| `POST /api/v1/endpoints/bulk-status` | `{selector, status, window, confirmCount}` | `status` `oneof=active disabled`; `active` memicu pembersihan D3 |
| `POST /api/v1/endpoints/bulk-delete` | `{selector, confirmCount}` | satu transaksi; hasil per baris |
| `POST /api/v1/endpoint-keys/bulk-tags` | `{selector, add[], remove[]}` | path datar, beralasan: selector menyeberang endpoint |
| `POST /api/v1/endpoint-keys/bulk-status` | `{selector, status, window, confirmCount}` | jendela dihitung per key dari policy provider induknya |
| `POST /api/v1/endpoint-keys/bulk-delete` | `{selector, confirmCount}` | invarian himpunan F11; CONFLICT sebagai hasil per baris |
| `POST /api/v1/proxies/bulk-status` | `{selector, enabled, confirmCount}` | menempel `SetEnabled` (`domain/proxy.go:165-168`); **tidak** menyentuh park Redis maupun `status` |
| `POST /api/v1/proxies/bulk-delete` | `{selector, confirmCount}` | membawa laporan referensi D5 |

Respons persis precedent (`handler/endpoint_bulk.go:134-153`): `{error:{code,message}, results:[{index,id,error}]}`;
plafon diperiksa di skema **dan** service (`endpoint_bulk.go:23,43-47`); seluruh rute baru di bawah gerbang sesi yang
sama (`router/router.go:110-114,181-186`) dan tiap rute membawa happy + validasi-gagal + auth-gagal (§2.1).

**D7 - `confirmCount` adalah penjaga jangkauan dan wajib untuk kedua aksi.** Klien mengirim ulang angka yang persis
dia tampilkan; server menghitung ulang target yang benar-benar di-resolve selector dan menolak bila berbeda ->
`409 BULK_TARGET_COUNT_MISMATCH`. Menutup dua kelas kecelakaan: seleksi basi setelah daftar disegarkan (reference
membuang id yang hilang secara diam-diam, `providers/[id]/page.js:953-955`), dan `all` yang tertukar dengan seleksi
sebagian. Disable memakai angka yang sama - reversibel bukan alasan untuk menebak.

**D8 - Proxy ikut, dan Select-All acted on proxy rows.** Jawaban owner poin 3. Satu baris = satu alamat (F19), jadi
tidak ada sub-alamat untuk dipilih; "pool" hanya tampilan agregat. Aturannya:

- Kontrolnya adalah `enabled` - kolomnya sudah ada, **kontrolnya belum** (hari ini sel teks
  `ProxyTable.svelte:86`), dipromosikan menjadi ikon toggle per baris.
- Disable operator masuk lewat `RouteUsable()` (`service/proxy_route.go:113-118`), **di luar** himpunan park Redis,
  supaya tidak bertabrakan dengan failover alamat draft 057 yang memutuskan skip otomatis pada granularitas alamat
  (`dataplane/proxy_route_walk.go:45-50`).
- Enable-ulang tidak menghapus `status` probe terakhir dan tidak membersihkan park: tidak ada sweeper di jalur itu,
  dan hasil probe bukan milik operator (kontras dengan D3 yang memang membersihkan penalti health).
- Bulk delete hanya boleh landed setelah laporan referensi D5 ada, dan teks `ProxyDeleteDialog.svelte:34` yang salah
  untuk provider ter-pin ikut dibetulkan pada pass yang sama.

**D9 - Select-All berarti "semua yang cocok di server", dan kalimatnya tidak boleh menyiratkan "yang termuat".**
Jawaban langsung untuk F17, dan ini keputusan yang paling sering salah dikerjakan di UI berpaginasi:

- Bar massal menampilkan angka yang **dikembalikan server** untuk scope itu, bukan `rows.length` dari halaman
  sekarang. Judul modal memakai angka yang sama - `providers/[id]/page.js:1096` di reference adalah contoh
  kegagalannya.
- Checkbox per baris = seleksi eksplisit (`ids`); "Select All" = permintaan `all` dengan scope filter yang sedang
  aktif, dan **mengganti** seleksi eksplisit, bukan menambahkannya. State tidak pernah memegang keduanya (D2 menolak
  `AMBIGUOUS`).
- Select-All menyebut scope di teks bar (bar bukan kontrol, jadi teks boleh lewat sana): `Select all 47 in this
  provider` / `Select all 312 across all providers`, sementara kontrolnya tetap ikon-semata.
- Daftar key yang client-paginated (`EndpointKeysTable.svelte:55`) tidak boleh mengira dirinya punya Select-All
  lintas-endpoint; di sana `all` = seluruh key milik satu endpoint yang sedang dibuka. Perluasan lintas-endpoint
  masuk lewat permukaan endpoint, bukan lewat drawer key.

**D10 - UI ikon-semata mengikuti kontrak yang sudah diberlakukan mesin, dan tidak mengimpor kebiasaan reference.**
Kontrak kita (`app-ui/tests/support/icon-only.ts:13-20`, dibaca langsung; SPEC-UI §8.11.9 `:1454-1460`):
`textContent` kosong, `title` **sama persis** dengan nama aksesibel, glyph `aria-hidden="true"`.

- **Tiga glyph baru, masing-masing dengan satu baris alasan** (SPEC-UI §8.11.2 `:1412-1414`) di
  `app-ui/src/lib/icons.ts`: `ROW_ACTION_ICONS` (`:122-169`, isinya rename/edit/disable/enable/test/delete/remove/
  save/cancel) tidak punya **tag**; `CONTROL_ICONS` (`:182-255`) tidak punya **select** maupun **select-none**.
  `select-none` wajib ada, kalau tidak "kosongkan seleksi" tidak punya ikon dan godaannya pakai teks. Satu set saja
  dari `@lucide/svelte` 1.47.0 (`app-ui/package.json`, `components.json` `"iconLibrary": "lucide"`), tanpa sprite
  sendiri dan tanpa sparkle/star/lightning/robot/orb (§8.11.5).
- **Bar massal:** pola `QuotaCards.svelte:195-218` - hitungan di teks statis `<span>N selected</span>`, kontrolnya
  ikon-semata dengan nama tetap `Tag selected` / `Disable selected` / `Enable selected` / `Delete selected`. Label
  tombol massal tidak ikut berubah saat busy, karena `expectIconOnly` menuntut `title === name` (lihat butir
  pelanggaran di bawah).
- **Kolom checkbox:** `QuotaCards.svelte:150-158` adalah satu-satunya precedent; label `min-h-11 min-w-11` dengan
  `aria-label` yang menyebut objeknya (`Select ${name} for bulk folding`) diangkat ke baris pertama tabel. Target
  sentuh 44px (§8.7.3 `:1371`).
- **Penamaan ikut model row-scoped satu-satunya yang ada:** `ProxyTable.svelte:109-129` (`Edit ${proxy.label}`,
  `Test ${proxy.label}`, `Delete ${proxy.label}` dengan `title` identik, glyph `aria-hidden` di `:113,123,132`) -
  bukan verb telanjang seperti `EndpointTable.svelte:86`, `EndpointKeysTable.svelte:123,131,144`,
  `ComboTable.svelte:82,91,100`, `ModelCatalogTable.svelte:147,158`, `CustomModelTable.svelte:122,132`. SPEC-UI §8.10
  `:1401` menuntut label menamai aksi **dan** objeknya.
- **Status tidak boleh warna-saja** (§8.11.8 `:1448`). Baris disabled butuh penanda yang terbaca tanpa warna, di luar
  `--color-danger` (`app.css:56,84`, alias `--color-destructive` `:107`). Ini juga menjawab `opacity-60` reference
  (F6).
- **Tag tidak menjadi kolom baru di tabel.** SPEC-UI §8.7.2 (`:1369-1370`) plus `app.css:182-248` mengubah setiap
  `<td>` menjadi baris berlabel di bawah 768px lewat `content: attr(data-label)`, dan `EndpointKeysTable.svelte:21-23`
  sudah menyatakan biaya itu secara eksplisit. Tag menjadi chip di sel identitas, di belakang kontrol edit/rename.
- **Dua kebiasaan reference yang jangan diimpor:** `title` telanjang tanpa `aria-label` (F5 toggle pause) dan
  hover-reveal kontrol destruktif (`sm:opacity-0 sm:group-hover:opacity-100`, F6) yang membuat delete tak terjangkau
  di layar sentuh.
- **Konfirmasi destruktif memakai `Modal.svelte`** (`:27-42`, `<dialog>` + `showModal`, fokus terjaga) dengan prosa
  kepermanentinan seperti `GatewayKeysTab.svelte:193` dan tombol `bg-[var(--color-danger)]` (`:205`). **Tidak ada toast
  dan tidak ada undo di repo ini**, jadi UI tidak boleh menjanjikan pembalikan yang tidak bisa ia tampilkan; hasil
  dilaporkan inline lewat `role="status"` / `role="alert"` seperti `ProviderOAuthAccounts.svelte:167-175`.
- **Nol emoji, dan gerbang kita tidak akan menangkapnya.** Cek emoji `scrypts/gates/antislop.sh:361-363` adalah kelas
  tujuh glif hardcode, bukan pemindaian Unicode - emoji baru di label, komentar, atau `aria-label` **lulus gerbang**
  dan hanya dilarang review (`docs/RULLES/ANTISLOP.md` §2.4 `:73-78`, SPEC-UI §8.11.4). Karena owner menyebut "None
  Emoticon" sebagai perintah, penjaga berbasis kategori Unicode masuk P2 (§6) dengan sengaja.
- **Yang sudah melanggar kontrak hari ini, dan ikut diperbaiki:** `ModelCatalogTable.svelte:147,158` dan
  `ProviderDisabledModels.svelte:92` mengganti `aria-label` menjadi partisipel berjalan sementara `title` tetap verb -
  `expectIconOnly` akan menolaknya. Membiarkannya berarti tes baru tampak hijau di atas baris yang memang salah.

---

## 5. Keputusan pass 3: ketiga pertanyaan tertutup

**Q1 - Plafon `all` = 2000 baris. DITERIMA owner.** Terikat sebagai `MaxBulkTargets` di service (D2), ditolak dengan
`413 BULK_TARGET_TOO_LARGE`. Konsekuensi yang harus ikut dibangun: respons penolakan itu **menyertakan jumlah yang
sebenarnya ter-resolve**, supaya operator tahu seberapa jauh dia menyempitkan, bukan sekadar "terlalu banyak".

**Q2 - Apakah jendela disable boleh dipotong reset vendor yang sesungguhnya?** Jalur sinyalnya sudah ada dan hidup:
`ResetParser`/`genericResets` (`provider/failure_reset.go:79-128`) -> `RestDirective`
(`domain/failure_rest.go:33-51`) -> `upstream_key_health.go:112-114`, dengan `reset_source` header|body|default sejak
`000021:10-12` yang **ditegaskan diagnostik, tidak dibaca router**. `quota_published_window.resets_at`
(`domain/quota_published.go:66`) juga sudah ada tapi hanya dibaca cache/respons
(`handler/quota_published_response.go:68-69`, `service/quota_published_cache.go:150-152,173-179`) dan **tidak pernah
masuk ke health**. Menyambungkan salah satunya ke `disabled_until` membuat panjang park ikut kebenaran vendor - dan
ikut kegagalan vendor. **DITERIMA owner: P0 memakai policy window yang deterministik dan bisa dites; sambungan ke `resets_at` vendor
ditahan sebagai P1.** Itu pilihan yang benar bukan karena lebih sederhana, tapi karena menyambungkannya sekarang akan
menjadikan keberhasilan disable dependen pada keberhasilan fetch kuota pihak ketiga - sebuah tombol yang menolak
bekerja karena quotafetch gagal adalah tombol yang tidak bisa dipercaya operator. Syarat sebelum P1 boleh dibuka:
tentukan lebih dulu apa yang terjadi ketika `resets_at` tidak ada atau basi (jatuh ke policy, atau tolak disable),
dan siapa yang berhak memendekkan jendela milik operator.

**Q3 - Permukaan mana yang belum ikut?** `ProviderNode` masih tidak punya status sama sekali
(`domain/provider_node.go:40-49`, `migrations/000004:8-18`): node hanya bisa ada atau dihapus, jadi Select-All pada
daftar node tidak punya aksi disable untuk dikerjakan. `gateway_keys` punya `revoked_at` satu arah
(`domain/gateway_key.go:37-47`, `service/gateway_key.go:119`) tanpa enable-ulang. **DITERIMA owner: keduanya keluar
dari draft ini** - bukan ditunda ke P2, dan bukan pula jadi pekerjaan tambahan yang menyusup. Kalau suatu saat daftar
node butuh bulk-only-delete atau `gateway_keys` butuh enable-ulang, itu draft tersendiri dengan aturannya sendiri.

---

## 6. Urutan kerja, siap diambil `continue`

**P0-0 - Prasyarat correctness, sebelum satu pun rute bulk dibuka.**

1. `listOAuthEndpoints` menyaring status: akun `disabled` tidak lagi menerbitkan refresh grant (F13, D4). Tes:
   bulk-disable lalu sweep, jumlah grant terbit = 0.
2. `MarkUnhealthy` tidak menimpa `disabled` (F14, D4). Tes: dead-letter di atas akun yang baru di-disable, status
   tetap `disabled`.
3. `NodeService.Delete` satu transaksi (F10), dengan tes rollback.

**P0-1 - Kolom dan jendela.**

4. `migrations/000022` tags + GIN dan `migrations/000023` `disabled_until` (D1, D3), keduanya additive dengan
   pasangan `down`.
5. Value object `Tag` + method agregat; field `disabledUntil` pada `UpstreamKey` dan `UpstreamEndpoint` lewat
   constructor/Rehydrate yang sudah ada (`domain/upstream_key.go:153`, `domain/upstream_endpoint.go:32`);
   `Available()` menambah gerbang jendela tanpa job (D3).
6. Satu field jendela pada `routing.provider_strategies[<id>].key_park` + seed 5h untuk keluarga 5 jam, **dan
   `quotaStep` ikut membaca policy** (F16) - tidak menambah jalur kedua yang pura-pura policy ada.

**P0-2 - Aksi massal.**

7. `BulkSelector` + validasi tepat-satu-terisi (D2) di lapisan schema, dengan tes penolakan tiap kombinasi.
8. `POST /endpoints/bulk-status` dan `/endpoints/bulk-delete` - service + repository satu transaksi via `inTx`,
   atribusi `BulkRowError`, `confirmCount` (D7).
9. `POST /endpoint-keys/bulk-status` dan `/endpoint-keys/bulk-delete` dengan invarian himpunan F11 (`UsesStoredKey()`
   = `api_key` + `service_account`), CONFLICT sebagai hasil per baris.
10. `POST /endpoints/bulk-tags` dan `/endpoint-keys/bulk-tags` (D1), plus `EndpointFilter` yang diperluas untuk
    `auth_type` dan himpunan id (F18; `= ANY($1::text[])` sudah punya preseden di `endpoint_summary.go:35-38`).

**P0-3 - Proxy (owner poin 3).**

11. `POST /proxies/bulk-status` menempel `SetEnabled`, masuk lewat `RouteUsable()`, **tanpa** menyentuh park Redis
    atau `status` probe (D8).
12. `POST /proxies/bulk-delete` dengan laporan referensi, dan teks `ProxyDeleteDialog.svelte:34` dibetulkan untuk
    provider ter-pin (F20, D5).

**P0-4 - Panel (ikon-semata, dan Select-All yang jujur).**

13. Kolom checkbox + Select-All di tabel endpoint, key, dan proxy; bar massal; angka dari server, bukan dari halaman
    yang termuat (D9).
14. `select` / `select-none` / `tag` di `icons.ts` dengan alasan satu baris; chip tag di sel identitas, bukan kolom
    baru; penanda disabled non-warna; `expectIconOnly` atas tiap kontrol baru.
15. `Enabled` proxy dipromosikan dari sel teks (`ProxyTable.svelte:86`) menjadi kontrol ikon toggle per baris.

**P1 - Setelah dua syarat §5 Q2 dijawab.** Q2 (`resets_at` -> `disabled_until`), autocomplete tag lintas provider
dengan batas yang benar, dan Select-All key lintas-endpoint bila memang diminta.

**P2 - Hardening.** Penjaga emoji berbasis kategori Unicode di `antislop.sh` (D10),
`expectIconOnly` dinaikkan ke seluruh tree termasuk tiga pelanggar yang disebut D10, dan row-scoped naming untuk tabel
yang masih memakai verb telanjang. `ProviderNode` dan `gateway_keys` **tidak** dititipkan ke sini - owner menutupnya
di §5 Q3 sebagai keluar dari draft ini.

**Catatan gerbang untuk pass implementasi.** `scrypts/gates/antislop.sh:126-129` memasukkan `*.svelte` dan `*.ts` ke
`CODE_GLOBS`, jadi aturan ini berlaku untuk kode panel: `CITATION_RE` (`:85`, dijalankan `:335-341`) menolak sitasi
`draft NNN §` dan `docs/DRAFT/` di berkas panel yang berubah - **alasan draft ini tidak boleh disalin ke komentar
source**; yang pindah ke komentar hanyalah konstrainnya sendiri. Dilarang em dash dan en dash (`DASH_RE` `:67`, `:301-307`),
artifact ` , ` (`:72`, `:356`), separator dekoratif (`:75`, `:357`), `} // end` (`:79`, `:359`), dan `eslint-disable`
tanpa `// reason:` (`:91-101`, `:379-384`).

Aturan komentar dari skill `antislop-code` berlaku penuh di pass implementasi, dan scoped-nya hanya komentar: tidak ada
satu baris pun komentar yang menarasikan langkah (`// Step 1: validate input`), melabeli kosong (`// Business logic`,
`// Main logic`), menggaungkan signature, atau memasang marker penutup blok. Konstrain panjang: **satu baris, dua bila
fakta keduanya baru** - dan nilai tidak membeli panjang. Yang wajib dipertahankan justru komentar pembawa konstrain
non-obvious, dan pass ini punya beberapa yang nyata layak ditulis: kenapa jendela disable dinilai saat dibaca dan bukan
dibebaskan job, kenapa `disabled_until` adalah gerbang serving sementara `quota_cooldown_until` dilarang menjadi satu
(D3), kenapa invarian key-terakhir diuji atas himpunan dan bukan per baris (F11), kenapa `MarkUnhealthy` tidak boleh
menyentuh `disabled` (F14), dan kenapa bulk-status proxy tidak membersihkan park Redis (D8). AGENTS.md §1.2 menuntut
header `@file/@for/@uses/@reason/@layer/@stability/@since` di setiap berkas baru, dan §1.1 (250 baris) akan memaksa
`endpoint_bulk.go` dipecah sebelum `bulk-status` + `bulk-delete` + `bulk-tags` masuk bersamanya. Seluruh string error ke
klien berbahasa Inggris (§1.3). `SYSTEM_MAP.md` **wajib** ikut di PR yang sama karena ini menambah kolom, rute, dan
interaksi worker->status baru (§1.9; F13/F14 justru mengubah interaksi terakhir itu).

---

## 7. Rencana verifikasi

Mengikuti `docs/RULLES/TDD.md` dan AGENTS.md §2.1: tes ditulis bersama atau di depan implementasi, tanpa `t.Skip()`,
`go test -race ./...` hijau, dan tiap rute baru membawa happy + validasi-gagal + auth-gagal.

**Matrix "Select-All harus membuktikan dirinya" (owner poin 4).** Bukan satu tes generik; tiap bentuk punya barisnya
sendiri di table-driven test, dengan id registry/DTO yang mengalaminya:

| Bentuk | Sasaran yang dites | Bukti di kode |
|---|---|---|
| `api_key` native, kunci-only | last-key protection hidup; disable per key ikut lantai yang sama | `openai` (`registry.yaml:500`), `deepseek` (`:2054`), `tokenrouter` (`:3116`); `endpoint_create.go:43-45,65-73` |
| `api_key` dengan header query-param | selector menormalkan ejaan registry `apikey` -> wire `api_key` | `gemini` (`:1086-1090`); `schema/endpoint.go:33-50`, `registry/load.go:162-178` |
| OAuth account (diturunkan, tanpa `auth_type` tertulis) | disable menghentikan grant sweep; tidak ada tabel key yang tersentuh | `claude` (`:9-13,55-58`), `gemini-cli` (`:385-420`), `antigravity` (`:228`), `github` (`:850`), `xai` (`:2900`) |
| OAuth dengan ladder `refreshRetryAt` | dead-letter tidak menimpa `disabled` (F14) | `kimi` (`:2191`), `grok-cli` (`:2831`), `zed` (`:3029-3034`) |
| Dual-mode oauth + apikey dalam satu agregat | seleksi memilih satu bentuk tanpa ikut yang lain | `qoder` (`:682-691`), `qoder-cn` (`:768-777`), `clinepass` (`:1613-1621`), `codebuddy-cn`/`-intl` (`:1675,1748`) |
| `service_account` | key terlindungi `UsesStoredKey()`; kolom sa ikut utuh | `vertex` (`:1014-1022`), `migrations/000016` |
| `no_auth` | tidak ada lantai key; disable tetap bekerja | `opencode` (`:949-953`), `registry/load.go:163-172` |
| Custom **OpenAI-compatible** `api_type: chat` | terjangkau `all` lewat id `openai-compatible-<...>` di tabel yang sama | `provider_node.go:33`, `custom_node.go:80-89` |
| Custom **OpenAI-compatible** `api_type: responses` | format `openai-responses`, tetap satu tabel | `custom_node.go:69-71`, `types.go:79` |
| Custom **Anthropic-compatible** | format `claude`, `api_type` wajib kosong, terjangkau `all` | `custom_node.go:66-68,112-115`, `000004:16-17` |
| `all` lintas provider (`provider_id` kosong) | native dan node terpilih bersamaan | `repository/postgres/endpoint.go:56-57` |
| Provider yang id-nya dibayangi alias | scope di-resolve sama seperti `Index.Provider` | `registry/load.go:96-140` |
| Proxy rows | `enabled` lewat `RouteUsable()`; park Redis dan `status` tidak tersentuh | `domain/proxy.go:165-168`, `proxy_route.go:113-118`, `redis/proxy_rotation.go:31,80` |

| Lapisan | Yang diuji |
|---|---|
| `domain` | `disabled` menang atas jendela; `disabled` dengan jendela lewat diperlakukan selesai tanpa job; jendela belum lewat tetap terkunci; enable-ulang bersih dari streak dan park health (D3, F2); `MarkUnhealthy` tidak menyentuh `disabled`; tag dinormalisasi, plafon 16/48, duplikat ditolak; invarian key-terakhir diuji **atas himpunan** - dua key dari satu endpoint ber-two-key harus CONFLICT (F11) |
| `schema` | nol target -> `BULK_SELECTOR_REQUIRED`; dua pengalih -> `BULK_SELECTOR_AMBIGUOUS`; `auth_type` di luar enum wire ditolak; plafon id 200; `status` di luar `active\|disabled` ditolak sebelum service; tag bertype ditolak sebelum repository |
| `repository/postgres` | containment tag memakai GIN dan mengembalikan baris lintas provider; batch gagal di tengah -> **nol** baris tersimpan; `= ANY($1)` satu statement, bukan per id (§1.7 N+1); `confirmCount` tidak cocok menolak tanpa efek; `tags` dan `disabled_until` tidak menyentuh `account`/`oauth`/`test_status` |
| `service` | guard combo/alias mendahului erasure (046); hasil per baris tidak membatalkan baris sah; jendela dihitung per provider lewat `HealthFor`, dan `quotaStep` terbukti membacanya (F16); hapus proxy mengembalikan laporan referensi, bukan diam |
| `handler` | bentuk `{error, results}`; **bulk delete yang diulang** memberi CONFLICT per baris di replay, bukan batch failure (§1.6 idempoten); auth-gagal menutup seluruh delapan rute |
| `app-ui` | `expectIconOnly` atas tiap kontrol baru termasuk bar massal dan toggle `Enabled` proxy; Select-All meminta **hitungan server** dan `confirmCount` yang dikirim sama; jumlah baris termuat **tidak pernah** jadi angka konfirmasi; kontrol destruktif terlihat tanpa hover; label Select-All menyebut scope (`@testing-library/svelte` 5.4.2, pola `tests/components/row-action-icons.test.ts:14-59`, `tests/support/gateway-key-stub.ts`) |

Verifikasi hidup, sebelum status draft ini pindah ke CLOSED:

1. Disable satu akun dan buktikan ia keluar dari rotasi seleksi tanpa menyentuh `rate_limited_until`.
2. Disable, lewati `disabled_until`, buktikan ia kembali **tanpa satu pun job** berjalan.
3. Enable-ulang membuktikan streak dan park ikut bersih (F2, D3).
4. Satu akun OAuth di-disable, jalankan `sweepProvider`, **hitung grant vendor yang diterbitkan = 0** (F13).
5. Pancing dead-letter di atas akun ber-`disabled`, buktikan status tetap `disabled` (F14).
6. Select-All di provider server-paginated dengan 10 baris termuat dari 47 total harus mengonfirmasi **47**, dan
   menolak `confirmCount` bila satu baris dihapus di antara render dan klik (D9, D7).
7. `POST /proxies/bulk-delete` atas proxy yang di-pin satu provider harus menampilkan jumlah endpoint menggantung dan
   pin yang ikut mati (F20, D5).
8. Satu `POST /endpoints/bulk-delete` yang baris keduanya CONFLICT meninggalkan **nol** perubahan di
   `upstream_endpoints`, `upstream_keys`, dan riwayat yang harusnya utuh (F10, D5).

---

## 8. Catatan di luar rencana - closed pass 3

`proxy/proxyscrape_premium_http_proxies.txt` adalah **100 baris kredensial proxy plaintext** berbentuk
`user:pass@ip:port` yang duduk di worktree (satu pasangan rotation-user Proxycrape dipakai bersama seluruh 100
alamat). Ia tidak di-track git (`git ls-files proxy/` kosong) dan di-ignore `.gitignore:60`, jadi tidak pernah keluar
lewat commit. Owner menutupnya: *"Ini proxy sudah mati, hapus juga tidak masalah."* **Dihapus 2026-10-16.** Direktori
`proxy/` dibiarkan ada (kosong) karena ia lokasi tempel kotak batch-add.

Dua hal yang ikut tercatat supaya tidak diangkat ulang sebagai temuan:

1. **Yang terhapus hanya sumber tempelnya, bukan proxy yang tersimpan.** Baris di tabel `proxies`
   (`migrations/000009:6-20`) tidak tersentuh oleh penghapusan ini. Kalau alamat-alamat mati itu masih ada di panel,
   membersihkannya justru pelanggan pertama yang tepat untuk `POST /proxies/bulk-delete` §6 P0-3 - dengan catatan
   laporan referensi D5 wajib lebih dulu landed, karena menghapus pool yang di-pin provider adalah satu-satunya
   jalur di permukaan proxy yang hari ini senyap (F20).
2. **`PANNELAI_TEST_POSTGRES_DSN` di `.github/workflows/gates.yml:66` bukan kebocoran.** Ia mencocokkan pola
   `user:pass@ip:port` dan memang saya periksa: postgres service-container dengan kredensial bawaan image, diikat ke
   `127.0.0.1` di network namespace job itu sendiri, database `pannelai_gate_test`. Tidak ada kredensial produksi di
   dalamnya. Dicatat di sini supaya pemindaian berikutnya tidak menghabiskan waktu pada baris yang sama.

---

## 9. Pass 4: apa yang landed, dan dua tempat saya menyimpang

P0 (§6 P0-0 sampai P0-4) dikerjakan dan hijau: `scrypts/gates/antislop.sh`, `go-headers.sh`, `go-lint.sh`
(gofmt + `go vet` + staticcheck bertag + golangci-lint, 0 issues) lolos, `go test -race` dan suite app-ui lolos.

**P0-0.** `sweepProvider` melewati akun `disabled` dan menamainya di `skipped[]` (F13); `MarkUnhealthy` tidak
lagi menimpa `disabled` tapi tetap mencatat alasannya (F14); `NodeService.Delete` jadi satu
`repository.NodeRepository.DeleteCascade` di dalam satu transaksi, dengan dua statement dihapus dari service dan
satu integration test rollback (node row hilang di tengah cascade -> nol perubahan di tiga tabel).

**P0-1.** `000022_endpoint_tags` (jsonb + GIN) dan `000023_operator_disable_window` (`disabled_until`, tanpa index
karena tidak ada lookup lewat kolom itu), masing-masing berpasangan `down` dan terverifikasi apply/up/down/up
terhadap PostgreSQL nyata. Value object `Tag`, `TagSet`, method agregat di kedua root, dan `Available()` yang
menilai jendela pada saat dibaca.

**P0-2 sampai P0-4.** Sembilan rute POST (`/endpoints/bulk-{status,tags,delete}`,
`/endpoint-keys/bulk-{status,tags,delete}`, `/proxies/bulk-{status,preview,delete}`), `BulkSelector` tepat-satu,
`MaxBulkTargets = 2000` dengan jumlah ter-resolve di pesan 413, `confirmCount` -> 409, satu transaksi per aksi,
attribusi per baris lewat `BulkRowError`, dan panel dengan kolom checkbox + Select-All di tiga tabel plus bar
massal ikon-semata yang dijaga `expectIconOnly`.

**Dua penyimpangan dari rencana, keduanya disengaja:**

1. **Seed 5 jam tidak jadi baris `settings`.** §6 butir 6 menuliskan seed lewat baris settings (`000008:29-33`).
   Yang landed adalah default di `defaultRoutingSettings()`: `quota_seconds = 18000` untuk `openai`, `claude`,
   `gemini`, `gemini-cli`, `antigravity`. Alasannya bukan malas SQL: tabel `settings` menyimpan dokumen routing
   utuh per key, jadi men-seed lewat migrasi berarti men-*merge* jsonb ke dokumen yang mungkin sudah ditulis
   operator, dan `INSERT ... ON CONFLICT` yang aman untuk itu lebih mudah salah daripada nilainya. Yang tetap
   benar sama persisnya: nilainya bisa diubah panel tanpa deploy, karena pembacaannya lewat
   `RoutingSettings.HealthFor` yang meng-merge override operator di atas default - dan override memang menang,
   itu justru salah satu test yang saya tulis (`TestOperatorOverrideBeatsTheSeed`). Kalau owner tetap mau angkanya
   ada di baris database, itu satu migrasi tambahan di atas ini, bukan perubahan bentuk.
2. **Satu rute kesembilan: `POST /proxies/bulk-preview`.** §6 menulis delapan rute. Enam saja tidak cukup untuk
   D5: panel tidak bisa mengonfirmasi "berapa endpoint yang ikut mati" sebelum server memberitahunya, karena
   `proxy_pool_id` adalah id string, bukan foreign key. Mengirim `confirm_endpoints: 0` dari panel akan berarti
   setiap bulk-delete dibayar 409, yaitu tombol yang tidak bisa dipakai. Preview menjawab jumlah dan provider yang
   kehilangan pin tanpa menulis apa pun; `bulk-delete` tetap memeriksaan angka itu sendiri, jadi preview bukan
   pengganti guard, hanya sumber angkanya.

**Yang belum dan tidak saya sentuh:** sambungan `resets_at` vendor ke `disabled_until` (P1, dua prasyarat §5 Q2
masih terbuka), autocomplete tag lintas provider (P1), penjaga emoji berbasis kategori Unicode di `antislop.sh`
dan kenaikan `expectIconOnly` ke seluruh tree termasuk tiga pelanggar yang disebut D10 (P2), dan **baris changelog**:
saya tidak menambah entri karena test `TestChangelogEntries_NewestFirst` memotong versi head ke `v0.4.0` dan
`openapi.json` membawa versi dokumen yang sama - itu keputusan rilis owner, bukan keputusan implementasi.
