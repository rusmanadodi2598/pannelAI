# 009-PORT-PROVIDER-PROXY

**Status: CLOSED 2026-09-26.** Binding proxy pool per provider: provider tertentu bisa memakai
pool sendiri, `None`, atau `Global`, dan engine tetap memungut multi pool saat strategi
`fallback` / `round_robin`. Terbangun, teruji, dan ter-commit (detail gerbang di §6).

## 1. Direktif

> "Oke, next penyambungan engine proxy pools ke provider, by design dari REFRENCE sudah support itu,
> dan kita bisa set di provider teretntu. Jadi, proxy pool bekerja secara native pada provider yang
> kita set (None Global). Dan juga, pastikan engine bisa pickup multi proxy pools saat kita set
> fallback, ataupun round robin."

Pass lanjutan langsung PORT 008 (engine rotasi pool global). Pass ini menyambungkannya ke
provider: satu setelan per provider yang memilih pool (atau `None`, atau `Global`), plus override
strategi per provider.

## 2. Keputusan owner (sesi tanya jawab 2026-09-26)

1. **Level binding: per-provider (map settings).** Mengikuti bentuk referensi
   `settings.providerStrategies[providerId]`, satu setelan untuk semua endpoint provider itu.
   Kolom `upstream_endpoints.proxy_pool_id` yang sudah ada tidak dipakai (tetap ditunda).
2. **Semantik pin: pin dulu, lalu sisanya.** Pool terikat jadi kandidat pertama; pool usable lain
   menyusul sesuai strategi, jadi pin tidak pernah mematikan failover multi pool.
3. **Strategi: override per-provider.** Provider tertentu bisa `fallback` sementara yang lain
   `round_robin`; kosong berarti ikut global.

## 3. Temuan REFERENCE (diukur di `~/apps/9router` @ `39e36d3d`, v0.5.86)

- Binding per-provider hidup di `settings.providerStrategies[providerId]`
  (`src/lib/db/repos/settingsRepo.js:15`), dibaca jalur data di `src/sse/services/auth.js:47-48`
  dan `:138`.
- Provider tanpa auth (`FREE_PROVIDERS`): `auth.js:45-71` membangun koneksi virtual `"noauth"`.
  `rotateStrategy != "none"` berarti **semua pool aktif dipakai** (`getProxyPools({isActive:true})`
  lalu `pickProxyPoolId`), dan pilihan pool diabaikan; selain itu `override.proxyPoolId` yang
  dipakai. Kursor rotasi adalah `Map` in-memory berkunci providerId (`connectionProxy.js:18-34`),
  hilang saat restart.
- Provider ber-kredensial: binding per-koneksi `providerSpecificData.proxyPoolId`
  (`AddApiKeyModal.js:31,127,375-387`, select "Proxy Pool" dengan sentinel `__none__` =
  "None (direct)"), diresolusi `resolveConnectionProxyConfig`
  (`src/lib/network/connectionProxy.js:66-187`): pool > proxy legacy per-koneksi > none.
- UI `NoAuthProxyCard.js`: select "Proxy Pool" (`__none__` + pool aktif) dan "Rotation Strategy"
  (`none` / `round-robin` / `random`), butuh >= 2 pool aktif, dan teks hidupnya: "Pool selector is
  ignored when rotation is active: all active pools are used." Dipasang di halaman detail provider
  (`providers/[id]/page.js:1510`) dan media provider (`media-providers/[kind]/[id]/page.js:160`).
  Dua batas yang penting untuk paritas: kartu itu hanya dirender untuk provider **tanpa auth**
  (`providers/[id]/page.js:1508-1512`: `isFreeNoAuth ? <NoAuthProxyCard/> : <Card>Connections</Card>`),
  dan **select pool-nya di-disable saat rotasi aktif** (`NoAuthProxyCard.js:96`) karena di sana pin
  dan rotasi saling eksklusif. Keputusan owner kita berbeda di kedua titik (D1: satu kartu untuk
  semua provider; D4: pin memimpin, sisanya menyusul), jadi salinannya pun tidak mengulang kalimat
  "pool selector is ignored" itu.
- API provider menerima `proxyPoolId` di POST/PATCH dan memperlakukan `null`/`""`/`__none__`
  sebagai hapus (`src/app/api/providers/route.js:31-46,97-101,171-172`;
  `src/app/api/providers/[id]/route.js:36-55,114,150-153`).
- Referensi **tidak punya** setelan outbound global; semua proxy-nya per-provider/per-koneksi.

## 4. Kondisi kita sebelum pass ini

- Kolom `upstream_endpoints.proxy_pool_id` sudah ada (migrasi `000012_endpoint_connection_parity`,
  ter-index, tervalidasi ke tabel `proxies` di `internal/service/endpoint_parity.go:73-89`), sudah
  ada di wire PATCH (`internal/schema/endpoint.go:109`) dan di respons
  (`internal/schema/endpoint_response.go:91`), **tetapi nol pembaca di jalur request**: grep
  `ProxyPoolID()` hanya menemukan service parity, domain getter, dan repository.
- Preseden map per-provider sudah ada untuk rotasi akun: `routing.provider_strategies[providerId]`
  (`internal/domain/settings_routing.go:69-216`, panel `app-ui/src/lib/schemas/settings.ts:51-61`).
- Seam dial: `engine_relay.go:107` membangun `Call`, dan `selection.Endpoint` adalah
  `domain.UpstreamEndpoint` yang sudah punya `ProxyPoolID()`; `Plan(ctx, host)` tidak menerima
  identitas provider. `MediaCall.ProviderID` (`internal/service/media_call.go:93`) tersedia,
  `MediaRequest` belum membawanya.
- Panel: `proxy_pool_id` tidak disebut sama sekali; form edit endpoint
  (`app-ui/src/lib/components/EndpointFieldsForm.svelte`) hanya label/priority/status. Halaman
  detail provider (`app-ui/src/routes/providers/[provider_id]/+page.svelte`, 202 baris) adalah
  rumah kartu ini.
- SPEC-API §7.11 baris 468-470 mengklaim "per-endpoint proxy binding is deferred (not in the
  reference either: reference assigns pools globally with per-provider strategy overrides)".
  Klaim itu **salah di separuh koneksi**: referensi memang membind per-koneksi.

## 5. Desain

- **D1** Binding disimpan di `network.provider_proxies`: map `provider_id -> {pool_id, strategy}`.
  Kunci map adalah provider id yang sama dengan `upstream_endpoints.provider_id` (registry maupun
  node custom). Satu setelan berlaku untuk semua endpoint provider itu.
- **D2** `pool_id` adalah salah satu dari: `""` = **Global** (ikuti `network.outbound_proxy_*`
  persis seperti hari ini), `__none__` = **None** (dial langsung, mem-bypass setelan global), atau
  id baris `proxies` = **pin** ke pool itu. Sentinel `__none__` mengikuti referensi.
- **D3** `strategy` adalah `""` (ikut `network.outbound_proxy_strategy`), `fallback`, atau
  `round_robin`. Kosong berarti warisan, bukan default diam-diam: `ParseProxyStrategy("")` menjawab
  default, jadi resolusi memeriksa kosong lebih dulu.
- **D4** **Pin dulu, lalu sisanya.** Saat `pool_id` menamai satu baris, kandidat pertama adalah
  baris itu (bila usable); baris usable lain menyusul dalam urutan strategi yang berlaku
  (`fallback`: urutan simpan; `round_robin`: urutan kursor). URL statis tetap ekor last-resort.
- **D5** Pin bekerja tanpa switch `outbound_proxy_enabled` ON, karena pin adalah instruksi eksplisit
  per-provider; `__none__` mematikan proxy untuk provider itu walau switch ON. Untuk mode Global,
  switch tetap gerbangnya seperti hari ini. URL statis hanya menjadi kandidat ekor bila switch ON,
  karena ia milik konfigurasi global.
- **D6** Validasi tulis: `pool_id` harus `""` | `__none__` | id baris tersimpan; id menggantung
  ditolak sebelum disimpan (finder baru di `SettingsService`, nil finder menolak id non-kosong,
  pola yang sama dengan `endpoint_parity.go`). `strategy` divalidasi terhadap himpunan tertutup di
  `Settings.Validate()` sebagai backstop.
- **D7** Identitas provider dibawa ke dial: port menjadi `Plan(ctx, providerID, host)`, dialer
  `Do(ctx, request, providerID)`, `Call` membawa provider id dari `resolution.Provider.ID`, dan
  jalur media ikut (`MediaRequest.ProviderID` dari `MediaCall.ProviderID`).
- **D8** Kursor rotasi menjadi per-provider: `pannelai:proxy:rotation:<provider_id>`, "global" untuk
  pemanggil tanpa provider. Alasannya: strategi kini bisa berbeda per provider, dan kursor bersama
  membuat lalu lintas satu provider menggeser posisi provider lain. Referensi juga berkunci
  providerId.
- **D9** Panel: komponen baru `ProviderProxyCard.svelte` di halaman detail provider, dua select
  (pool: Global / None / baris; strategi: Ikut global / fallback / round_robin), menyimpan lewat
  PATCH `/settings` dengan map penuh `network.provider_proxies` (read-modify-write atas map yang
  baru dibaca, pola referensi). Entri dihapus saat kedua nilai kembali default. Dua hal yang
  ditambahkan saat implementasi dan diuji: (a) baris yang di-pin tapi sudah tidak ada di pool tetap
  dirender sebagai opsi berlabel `(missing)`, supaya select tidak diam-diam membaca sebagai Global
  dan men-drop pin pada perubahan strategi berikutnya; (b) satu baris keadaan menyebut perilaku
  efektif (pin memimpin lalu sisanya mengikuti strategi / None dial langsung / mengikuti setelan
  global), yang menggantikan kalimat referensi "pool selector is ignored".
- **D10** SPEC-API §7.11 dikoreksi (klaim reference) dan ditambah paragraf binding per-provider;
  §7.14 contoh dokumen + aturan patch; SPEC-UI §6.x baris kartu + §14.

## 6. Gerbang

Semua dijalankan pada pohon beku (tidak ada suntingan kode setelah gerbang terakhir).

- **TDD merah dulu per lapisan** (domain, service, dataplane, schema, panel): uji ditulis sebelum
  implementasi tiap lapisan. Hasil fokus sepanjang pass: 31/31 pada berkas baru; 59/59 pada suite
  komponen terdampak; 63 berkas / 1604 tes pada `tests/schemas/` + berkas route provider; 35/35
  pada re-run berkas yang copy-nya ikut berubah.
- **go lint** (`scrypts/gates/go-lint.sh`): PASS. `go vet` (dua tag), `gofmt`, staticcheck (dua
  tag), golangci-lint 0 issues. Dua FAIL batas 250 baris (AGENTS.md §1.1) muncul di pass ini dan
  diperbaiki dengan pemecahan berkas, bukan pengecualian: `internal/schema/settings_patch.go`
  282 → 239 baris (tipe + dua konversi pindah ke `internal/schema/settings_provider_proxy.go`,
  60 baris) dan `internal/service/proxy_route.go` 255 → 194 baris (urutan kandidat pindah ke
  `internal/service/proxy_route_order.go`, 104 baris: satu metode `candidateOrder` +
  `moveIDToFront`). Sisa SKIP: peringatan 220 baris pada berkas berubah, semuanya di bawah 250
  dan ditinjau.
- **go headers** (`scrypts/gates/go-headers.sh`): PASS, 918 berkas Go berheader lengkap (dua
  berkas baru ikut terperiksa).
- **go test -race** (`scrypts/gates/go-test.sh`): PASS, 5m06s, seluruh paket `ok`.
- **suite integrasi** (`-tags=integration`, Postgres + Redis): SKIP dengan alasan. Tidak ada
  perubahan migrasi maupun repository postgres/redis pada pass ini, dan suite bertag menyapu Redis
  DB 0 yang dipakai gateway live.
- **verifikasi state commit** (pohon terpisah dari indeks staged lewat `git checkout-index --all
  --prefix`): `gofmt -l` bersih, `go build ./...`, `go vet ./...`, dan `go test ./...` EXIT=0,
  termasuk `tools/openapi-gen -check` (drift nol pada pasangan kontrak YAML + `openapi.json` yang
  di-stage).
- **contract drift** (`scrypts/gates/contract-drift.sh`): PASS, 12 kode error sepakat di dua sisi.
- **secrets** (`scrypts/gates/secrets.sh`): PASS, 240 commit bersih dan tidak ada kebocoran di
  berkas yang akan dibawa push.
- **contract-openapi** (`scrypts/gates/contract-openapi.sh`): PASS, dokumen tersaji sama dengan
  kontrak YAML.
- **panel check** (`scrypts/gates/panel-check.sh`): PASS, EXIT=0. prettier, eslint, svelte-check
  (0 error / 0 warning), vitest 164 berkas / 2658 tes lulus (1452.04s), `vite build` PASS. Dua FAIL
  muncul di pass ini dan keduanya diperbaiki di sumbernya, lalu seluruh gerbang panel dijalankan
  ulang pada pohon beku: (a) eslint menandai dua variabel tak terpakai di berkas uji baru
  (`POOL_LABEL` dan `stub`; konstanta label kini dipakai di dua asersi, variabel stub dihapus);
  (b) audit R-02 menemukan tiga em dash di teks pass ini (satu kalimat copy kartu outbound, dua
  komentar berkas uji) dan semuanya diganti koma atau tanda kurung. Em dash yang masih ada di
  berkas commit ini sudah ada di `HEAD` sebelum pass ini dan diukur, bukan diklaim:
  `internal/dataplane/media.go` 1 baris, `internal/service/media_perform.go` 3 baris, dan
  `docs/SPEC-API` 4 baris (residu pass 008).
- **click-through** (Chrome headless + CDP, gateway kerja di :9096, panel di :3021, dua run:
  sebelum dan sesudah pemecahan berkas §1.1): PASS. 8 aksi, 9 body PATCH, 0 `pageErrors`, dan
  pembacaan server cocok dengan kartu di setiap aksi: kartu hadir di `/providers/openai` di antara
  Connections dan katalog model; pin satu baris
  (`{"network":{"provider_proxies":{"openai":{"pool_id":"prx_0386X78W4FBAJ5QTTWNEBDHZ7R"}}}}`),
  pin + strategi, sentinel `__none__`, kembali ke Global dengan strategi tetap
  (`{"openai":{"strategy":"round_robin"}}`), kedua select default menghapus entri (`{}`), pin
  bertahan setelah reload; pembersihan terbukti
  (`{"deleted":["prx_0386X78W4FBAJ5QTTWNEBDHZ7R"],"remainingWithLabel":0,"providerProxies":{}}`).
  Bukti: `/tmp/pass39/evidence.json` (+ salinan run pertama `evidence-prefix-split.json`) dan 2
  tangkapan layar di `/tmp/pass39/shots/`.
- **probe 390 px + keyboard** (R-03, R-32; driver `/tmp/pass39/probe390.ts`, data
  `/tmp/pass39/probe390.json`, tangkapan `probe390.png`): viewport 390x844 `mobile:false`;
  `documentElement.scrollWidth` 390 = `innerWidth` 390 dan `scrollTo(400, 0)` tidak menggeser
  dokumen (`scrollX` tetap 0), jadi tidak ada overflow level halaman; kartu dan kedua select
  berhenti di x=374; Tab memindahkan fokus dari select pool ke select strategi dan Shift+Tab
  kembali, keduanya dengan `outline: solid 2px` (cincin fokus `:focus-visible`; fokus programatik
  memang tidak memunculkannya).
- **probe tema + kontras** (R-25, R-34; driver `/tmp/pass39/probe-theme.ts`, data
  `/tmp/pass39/probe-theme.json`, tangkapan `probe-theme.png`): kartu tidak membawa satu pun warna
  literal, hanya token tema, dan itu diukur di dua mode. Terang: teks `#1a1614` di atas latar
  `#fcfaf7` = 17.25:1, teks muted `#5f574e` = 6.81:1. Gelap (setelah tombol `Switch to dark theme`):
  teks `#edeae6` di atas `#1a1917` = 14.65:1, muted `#a69e93` = 6.64:1. Semua di atas ambang AA
  4.5:1, dan latar kartu sendiri transparan (mewarisi latar halaman) di kedua mode.
- **Delivery Gate** (empat blok + checklist UI 15 item): laporan di respons penutup.

## 7. Residu

- Kolom `upstream_endpoints.proxy_pool_id` tetap tersimpan tanpa pembaca (binding per-endpoint
  ditunda atas keputusan owner); kandidat pass berikutnya, dan pertanyaan terbukanya tercatat
  sebagai SPEC-UI §14 Q25.
- Provider tanpa endpoint (mis. media provider) tetap memakai mode Global.
- F12/F13 register readiness app-ui belum diukur ulang untuk kartu ini.
- **Commit indeks di atas `HEAD`, aktor lain in-flight:** 47 berkas di-stage; enam berkas campuran
  di-stage sebagai potongan di atas `HEAD` (`6072f90`), sisanya ditambahkan per-path. Enam berkas
  itu: kontrak YAML (2 dari 4 hunk), `openapi.json` (2 dari 4 hunk), `docs/SPEC-API`,
  `docs/SPEC-UI`, `internal/schema/settings_patch.go`, dan
  `app-ui/tests/components/proxy-pools.test.ts`. Suntingan in-flight aktor lain (keluarga skema
  patch proxy baru, dua tag validator berawalan `clearing`, pesan batch paste berskema ftp, dan
  reflow prettier dua SPEC) tetap unstaged di pohon kerja; pindaian penanda pada
  `git diff --cached` menunjukkan 0 kemunculan. Pohon commit dimaterialisasi ulang dari indeks dan diuji: `gofmt -l` bersih,
  `go build`/`go vet`/`go test` EXIT=0, dan `tools/openapi-gen -check` PASS pada pasangan kontrak
  YAML + `openapi.json` yang di-stage.
- **`HEAD` membawa satu tes panel merah yang diperbaiki pass ini:**
  `app-ui/tests/components/proxy-pools.test.ts` di `HEAD` masih menanyakan label
  `'Outbound proxy URL'` sementara komponen di `HEAD` sudah memakai `'Last-resort proxy URL'`,
  jadi tes itu tidak bisa lulus di `HEAD`. Pass ini mengadopsi perbaikan satu barisnya (plus
  asersi strategi yang dijanjikan nama tesnya) dan menandainya dengan komentar, bukan mewariskan
  tes merah.
- **Suite integrasi** tidak dijalankan (alasan di §6); jalankan `PANNELAI_TEST_POSTGRES_DSN` +
  `PANNELAI_TEST_REDIS_ADDR` bila pass berikutnya menyentuh migrasi atau repository.
- **Em dash: diukur, bukan diklaim.** Dari 47 berkas commit ini, em dash hanya tersisa di baris
  yang sudah ada di `HEAD` sebelum pass ini (`dataplane/media.go` 1, `media_perform.go` 3,
  `docs/SPEC-API` 4). Tiga em dash milik teks pass ini (satu kalimat copy kartu outbound dan dua
  komentar berkas uji) ditemukan oleh audit R-02 setelah commit pertama dan diperbaiki di sumbernya
  sebelum push; commit pass ini di-amend agar satu pass tetap satu commit.
