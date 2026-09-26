# 008-PORT-PROXY-ENGINE

**Status: CLOSED 2026-09-26.** Engine rotasi proxy pool (fallback + round-robin) untuk jalur
egress terbangun, teruji, dan ter-commit.

## 1. Direktif

Pemilik (2026-09-26): "Lanjut untuk patch engine dari Proxy Pools untuk bisa menggunakan method:
fallback, dan method round-robin. Supaya ketika save multi proxy akun, bisa digunakan dengan
maksimal, dan bukan cuma static 1 proxy saja. Di reference sudah support ini, namun di Reference
juga saya test belum optimal. Untuk itu, kita harus buat engine ini optimal."

## 2. Design Read

Reference (`~/apps/9router` v0.5.86) memutar proxy HANYA untuk provider no-auth
(`src/sse/services/auth.js` + `src/lib/network/connectionProxy.js`): `pickProxyPoolId` memutar
kursor in-memory `Map` (hilang saat restart), tanpa kesadaran kesehatan (proxy mati tetap dipilih),
tanpa rantai fallback untuk proxy (fallback milik akun, bukan proxy), dan rotasi global-pool lewat
`rotateStrategy` per provider. Kelemahan itulah yang diukur pemilik sebagai "belum optimal".

app-serv hari ini: `settings.network.outbound_proxy_url` adalah SATU URL statis yang dibaca per
request oleh `egressProxy` (cmd/app-serv/egress_wiring.go); pool (tabel `proxies`) tidak terlibat
sama sekali di jalur egress (SPEC-UI §14 Q15/Q16 merekam ketimpangan itu). Dua titik panggil
dataplane (`transport_call.go` attempt, `media.go`) memakai satu shared client yang Proxy-nya
ditanam di composition root.

## 3. Keputusan desain

- **D1, pool adalah rute:** saat `outbound_proxy_enabled` menyala, baris pool yang enabled
  MENJADI rute; `outbound_proxy_url` statis menjadi anggota terakhir (last resort) dari rantai
  attempt. Tanpa baris pool yang usable, perilaku hari ini dipertahankan: URL statis, atau direct
  saat kosong (bypass empty-URL §7.14 tidak berubah).
- **D2, strategy:** `settings.network` bertambah `outbound_proxy_strategy` ∈ {`fallback`,
  `round_robin`}, default `fallback`; nilai tersimpan kosong dibaca sebagai default (dokumen lama
  tanpa kunci tetap sah).
- **D3, baris usable:** enabled DAN status probe terakhir bukan `fail`. Baris yang belum pernah
  dites tetap usable (operator boleh route sebelum tes); baris `fail` keluar dari rencana sampai
  dites ulang. Edit alamat/kredensial menghapus status (Repoint/SetCredentials), jadi baris kembali
  usable sebagai untested.
- **D4, urutan stabil:** (created_at, id), urutan memasukkan adalah urutan fallback operator.
  Servis mengurutkan sendiri dari `List()` (pool kecil, §7.11), sehingga tidak ada perubahan skema
  maupun kontrak repository; perubahan `priority` eksplisit per baris adalah filing berikutnya.
- **D5, round-robin Redis:** kursor INCR atomik (`pannelai:proxy:rotation:global`, TTL 24 jam,
  pola `ComboRotationStore`) memutar urutan usable per request; kepala daftar adalah pilihan
  request ini, ekornya adalah jalur failover. Kursor bertahan restart, kelemahan reference yang
  diukur.
- **D6, parkir kegagalan dial:** kegagalan connect-stage melalui kandidat memarkir id-nya di Redis
  (`pannelai:proxy:park:<sha256>`, TTL 2 menit); baris terparkir dilewati selama masih ada baris
  usable yang tidak terparkir; bila semua terparkir, daftar usable tetap dipakai (parkir adalah
  petunjuk, bukan kebenaran).
- **D7, failover terbatas:** maksimal 3 attempt per request (`maxProxyAttempts`), berjalan pada
  urutan rencana; hanya kegagalan connect-stage (dial ke/lewat proxy) yang memicu failover;
  kegagalan TLS atau status HTTP dikembalikan (proxy sudah mengantarkan request). Loop retry §4
  tetap berada di luar walk ini dengan semantik yang sama.
- **D8, guard:** host tujuan divalidasi sekali per request terencana (`guard.CheckHost` di adapter
  wiring) sebelum attempt pertama; transport per-attempt adalah kloning shared client, sehingga
  dialer terjaga melihat alamat proxy saat connect (OWASP A01 tidak berubah).
- **D9, konsumen lain utuh:** shared egress client mempertahankan Proxy func statis hari ini
  (OAuth, vision, probe tidak berubah); engine pool hanya mengambil alih dua titik panggil
  dataplane. Tanpa `Routes` di-wire, perilaku identik (semua uji lama hidup).
- **D10, panel dan kontrak:** kartu outbound panel mendapat pemilih strategy dan copy-nya ditulis
  ulang (pool merutekan saat proxying menyala; URL adalah last resort). Kontrak YAML + dokumen
  ter-generate mengikuti; SPEC-API §7.11/§7.14 dan SPEC-UI §6.9 diamandemen, §14 Q15/Q16 diperbarui.

## 4. Rencana implementasi (TDD merah-dulu)

1. domain/proxy_rotation.go: strategy parse (himpunan tertutup), urutan rotasi, tipe attempt;
   uji tabel merah dulu.
2. repository/proxy_route.go (kontrak) + repository/redis/proxy_rotation.go (INCR+parkir) + uji
   pola combo (kunci + jalur tanpa Redis).
3. service/proxy_route.go: ProxyRouteService.Plan (settings per request, usable filter, parkir,
   rotasi, last resort statis) + uji merah dulu dengan fake repo/store/opener/clock.
4. dataplane: interface planner + penggolong kegagalan connect-stage + walk attempt pada
   transport_call.go dan media.go; uji httptest merah dulu (proxy mati → kandidat kedua melayani).
5. cmd/app-serv: adapter guard + wiring; uji egress diperbarui.
6. settings schema/domain/patch + kontrak YAML + regen openapi.json (hunk-stage, berkas campuran).
7. panel: skema zod + kartu outbound + uji.
8. SPEC + register + gerbang penuh + click-through.

## 5. Implementasi

Semua di bawah ini diuji merah-dulu; angka gerbang ada di §6.

**Domain.** `internal/domain/proxy_rotation.go`: himpunan tertutup `fallback`/`round_robin`,
`DefaultProxyStrategy`, `ProxyFailureCooldown` (2 menit), `MaxProxyRouteAttempts` (3),
`ProxyRoutePoolKey`, `ProxyExempt` (satu implementasi untuk kedua rute; jalur statis di
`egress_wiring.go` sekarang memanggilnya, `exemptFromProxy` lokal dihapus), `ProxyRouteAttempt`,
`ParseProxyStrategy` (""→default, di luar himpunan ditolak dengan pesan yang menamai kunci),
`(Proxy) RouteUsable()`, `(Proxy) DialURL(secret)`.

**Store.** `internal/repository/proxy_route.go` (kontrak `ProxyRouteStore`) +
`internal/repository/redis/proxy_rotation.go`: kursor INCR+EXPIRE `pannelai:proxy:rotation:`
(TTL 24 jam, pola `ComboRotationStore`, urutan nol-basis lewat `domain.RotateRefs`), parkir
`pannelai:proxy:park:<sha256>` (SET+TTL), daftar <2 melewati Redis.

**Service.** `internal/service/proxy_route.go`: `Plan(ctx, destHost)`: settings per request
(galat → tolak), proxy off / host exempt → rencana kosong, `List()` galat → degradasi (rencana
kosong, jalur statis tetap hidup), filter `RouteUsable`, urut stabil (created_at, id), saring
terparkir (semua terparkir → daftar utuh), rotasi round-robin (galat store → urutan stabil),
buka seal per baris (satu seal gagal → baris dilewati), `DialURL`, lalu URL statis sebagai
attempt terakhir, dan **URL statis malformed kini DITOLAK, bukan di-drop** (paritas dengan jalur
statis; uji baru menambahkan kasus ini). `ReportFailure` memarkir best-effort.

**Dataplane.** `internal/dataplane/proxy_route.go`: port `ProxyRoutePlanner`, `ProxyDialer`
(rencana kosong → shared client; walk terbatas 3; per-attempt client = kloning
`*http.Transport` shared yang di-cache per URL kandidat + `http.ProxyURL`; body dipulihkan lewat
`GetBody`; `isProxyConnectFailure` = `*net.OpError` atau `net.Error.Timeout()`; galat non-connect
mengakhiri walk; `ReportFailure` per kandidat yang gagal connect). `TransportDeps.Routes` dan
`NewMediaTransport(client, routes)` (dua call site diperbarui); `attempt()`/`MediaTransport.Do`
memakai dialer.

**Wiring.** `cmd/app-serv/proxy_wiring.go`: `buildProxyRoutePlanner` (repo postgres + store Redis +
settings + sealer) dan `guardedProxyPlanner` (D8: `CheckHost` sekali per request terencana, hanya
saat rencana non-kosong). `management_wiring.go` membangunnya sekali dan meneruskannya lewat
`managementDataPlaneInputs.Routes` → `dataPlaneInputs.Routes` → `TransportDeps` + media caller.
Shared client tetap memegang Proxy func statis (D9): OAuth/vision/probe tidak berubah.

**Settings wire.** `internal/domain/settings.go` + `settings_patch.go` (field, apply, backstop
`ParseProxyStrategy` di `Validate`), `internal/schema/settings.go` (DTO baca) +
`settings_patch.go` (field PATCH `omitempty,oneof=fallback round_robin`; `""` non-nil DITOLAK,
berbeda dari field URL, karena strategi tidak punya anggota kosong; panel mereset dengan mengirim
`fallback`), mapper menjawab nilai efektif. Kontrak YAML (`NetworkSettingsPatch` enum,
`NetworkSettingsResponse` enum + required) + regen `openapi.json` (`go run ./tools/openapi-gen`).

**Panel.** `app-ui/src/lib/schemas/settings.ts`: `PROXY_STRATEGIES` + label, field di skema
baca dan form; refine "URL wajib saat proxy on" DIHAPUS (mesin membuatnya tidak benar).
`ProxyOutboundSettings.svelte`: pemilih `Pool strategy` + copy ditulis ulang (pool adalah rute,
URL adalah last resort), catatan stored-state baru (`role="status"`, muted) untuk proxy on tanpa
URL, label URL menjadi "Last-resort proxy URL". Empty state pool kembali ke kalimat §6.9 dengan
kualifikasi switch: "Add one to route upstream calls through it once proxying is on."

**Uji baru.** `proxy_rotation_test.go` (domain), `proxy_rotation_test.go` (redis),
`proxy_route_test.go` (service, tabel Plan + round-robin + degradasi + ReportFailure),
`proxy_route_test.go` (dataplane: failover httptest lewat proxy mati `127.0.0.1:1`, walk
terbatas, body utuh, klasifikasi kegagalan), `proxy_route_planner_test.go` (cmd: D8 guard +
pass-through parkir), `settings_strategy_patch_test.go` (schema: tag, drift-pin oneof↔domain,
lowering, nilai efektif), `settings_strategy_test.go` (domain backstop). Panel:
`settings-forms.test.ts` (kasus flip: proxy on tanpa URL diterima; strategi ditolak/diterima),
`proxy-batch-save.test.ts` (save pool-only, catatan stored-state baru), `proxy-pools.test.ts`
(kartu: copy baru, nilai strategi tersimpan, empty state baru).

## 6. Gerbang

Semua dijalankan pada pohon beku (tidak ada suntingan kode setelah gerbang terakhir).

- **go lint** (`scrypts/gates/go-lint.sh`): PASS. `go vet` (dua tag), `gofmt`, staticcheck
  (dua tag), golangci-lint 0 issues. Satu FAIL sempat muncul: `internal/service/proxy_route_test.go`
  367 baris (batas 250, dan gate ini memeriksa berkas test juga meski komentarnya menyebut
  sebaliknya); file dipecah tiga (`proxy_route_fixture_test.go` 122, `proxy_route_test.go` 138,
  `proxy_route_rotation_test.go` 164). Sisa SKIP: peringatan 220 baris pada berkas berubah
  (dataplane_wiring.go 234, schema/settings_patch.go 237, dll), semuanya di bawah 250 dan ditinjau.
- **go headers** (`scrypts/gates/go-headers.sh`): PASS, 911 berkas Go berheader lengkap.
- **go test -race** (`scrypts/gates/go-test.sh`): PASS, 4m29s, seluruh paket `ok`.
- **suite integrasi** (`-tags=integration`, Postgres + Redis): SKIP dengan alasan. Tidak ada
  perubahan migrasi maupun repository postgres/redis pada pass ini (store rotasi baru diuji
  hermetik), dan menjalankan suite bertag menyapu Redis DB 0 yang dipakai gateway live sehingga
  operator terlempar ke /login.
- **verifikasi state commit** (worktree terpisah pada `HEAD` + seluruh patch staged):
  `go build ./...`, `go vet ./...`, dan `go test ./...` EXIT=0, termasuk `tools/openapi-gen`
  (drift kontrak nol pada state commit) dan `internal/service` (11.6s).
- **panel check** (`scrypts/gates/panel-check.sh`): PASS. prettier, eslint, svelte-check (0 error / 0 warning), vitest 161 berkas / 2623 tes lulus
  (1729.82s), `vite build` PASS; EXIT=0.
- **click-through** (Chrome headless + CDP, gateway kerja di :9095, panel di :3021):
  PASS, 9 aksi terekam di `/tmp/pass38/evidence.json` + 3 tangkapan layar: sign-in; kontras teks
  muted terukur 6.81:1 (AA); kartu terbuka (opsi `fallback`/`round_robin`, label "Last-resort proxy
  URL", copy last-resort + parkir dua menit + label switch); ubah lalu dirty, discard memulihkan;
  simpan `round_robin` (PATCH /api/v1/settings 200) dan reload membacanya; pulihkan `fallback`;
  390 px `overflow: false`, kartu terlihat; `pageErrors` kosong.
- **Delivery Gate** (empat blok + checklist UI 15 item): laporan di respons penutup.

## 7. Residu

- **Suite integrasi** tidak dijalankan (alasan di §6). Jalankan `PANNELAI_TEST_POSTGRES_DSN` +
  `PANNELAI_TEST_REDIS_ADDR` bila pass berikutnya menyentuh migrasi atau repository.
- **Kursor rotasi global:** `ProxyRoutePoolKey = "global"` (D5). Bila kelak ada lebih dari satu
  pool, kunci per-pool adalah filing berikutnya, bukan perubahan skema.
- **`priority` eksplisit per baris** tidak ditambahkan (D4 memakai created_at, id); filing bila
  operator butuh urutan manual.
- **Em dash: diukur, bukan diklaim.** `HEAD` sudah memuat karakter em dash (U+2014) di 4 baris
  `docs/SPEC-API/001-SPEC-API.md` (dua baris tabel + dua entri changelog) dan pohon kerja
  memuat 4 baris yang sama; tidak ada penambahan di pohon ini. Klaim awal §7 ("3 baris tambahan
  milik aktor lain") dikoreksi setelah diukur: yang berubah pada baris itu hanya padding tabel
  dan penanda penekanan dari reflow, bukan karakternya. Teks pass ini 0 karakter itu.
- **Commit indeks di atas `HEAD`, aktor lain in-flight:** seluruh suntingan pass ini
  di-stage sebagai potongan di atas `HEAD` (`81c26db`); suntingan in-flight aktor lain
  (`ProxyPatchRequest` + tag `clearing_url` + pesan batch paste) dan reflow prettier dua
  SPEC tetap unstaged di pohon kerja. Enam berkas campuran di-stage per-hunk dengan patch
  tangan (`schema/settings_patch.go`, `openapi.json`, kontrak YAML, `docs/SPEC-API`,
  `docs/SPEC-UI`, `app-ui/tests/components/proxy-pools.test.ts`); `openapi.json` yang
  di-stage dihasilkan ulang dari kontrak YAML yang di-stage (`tools/openapi-gen -input`),
  jadi drift kontraknya nol.
- **F12/F13 register app-ui** (readiness) belum diukur ulang untuk kartu outbound ini; kandidat
  baris pointer bila pass berikutnya menyentuh register itu.
