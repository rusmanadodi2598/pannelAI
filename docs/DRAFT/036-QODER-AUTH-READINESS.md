# 036-QODER-AUTH-READINESS.md: Provider Qoder bisa di-login — device flow OAuth, dan jalur PAT yang masih terbuka

Dokumen kerja pass `app-serv` (dan nanti `app-ui`) untuk menghubungkan akun **Qoder** ke panel. Pass ini
mengerjakan permintaan owner: "Provider Qoder, untuk OAuth & PAT, sesuai REFERENCE (9router)". Bukan
kontrak; kontrak tetap `docs/SPEC-API/001-SPEC-API.md` (wire) dan `docs/CONTRACT/001-CONTRACT-API-V1.yaml`
(skema). Pola mengikuti `002-PORT-PROVIDER.md`: temuan bernomor F, bukti yang bisa diulang, keputusan di
depan implementasi.

|                    |                                                                                                                                                                                    |
| ------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**         | **Slice A selesai 2026-09-27 (commit `0d519fa`). Slice B selesai lebih dulu (commit `c0e3034`). Slice C (modal device di panel) selesai 2026-09-27. Slice D (data plane Qoder: COSY + pertukaran PAT + kuota) TERBUKA.** |
| **Mechanism**      | PORT (reference → Go) + TDD                                                                                                                                                          |
| **Scope**          | `app-serv/.` (service, repository, handler, schema, router, kontrak). `app-ui/.` tidak disentuh pada slice A.                                                                       |
| **Permintaan owner** | (1) Qoder bisa di-OAuth. (2) Qoder bisa diisi PAT. (3) Paritas dengan reference. (4) Verifier device flow dipegang server, panel hanya mengirim `device_code` (keputusan owner 2026-09-27). |
| **Reference**      | `decolua/9router`, checkout `/home/rusmanadodi/apps/9router`; `src/lib/oauth/services/qoder.js`, `src/lib/oauth/providers/qoder.js`, `open-sse/services/qoderModels.js`, `open-sse/shared/qoder/constants.js` |
| **Kaitan**         | SPEC-API §7.4 (baris device + blok "The device flow has no callback"), §6 (credential tersegel), §4 (replay guard sekali pakai); draft 017/019 (permukaan Provider), draft 028 (fallback credential) |
| **Tanggal**        | 2026-09-27                                                                                                                                                                         |

## 1. Ringkasan

Sebelum pass ini, Qoder **tidak bisa dihubungkan sama sekali**, dan tidak ada satu pun jalur yang bisa
membuatnya terhubung.

| Yang ada                             | Yang diukur                                                                                                                                            |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Entri registry `qoder` / `qoder-cn`  | Ada (`registry.yaml:679`, `:748`), dengan blok `oauth` lengkap: `device_token_url`, `login_url`, `user_info_url`, `refresh_url`, `quota_usage_url`.     |
| Route OAuth yang melayani entri itu | **Nol.** `oauth_flow.go` hanya tahu code flow; `requireCodeFlow` menolak setiap provider tanpa `authorize_url` dengan pesan *"uses a device authorization flow; connect it through its device endpoint instead"* — dan endpoint yang dimaksud belum ada. |
| Sisa pekerjaan dari sesi sebelumnya  | `oauth_flow_device.go` + `oauth_client_device.go` tertulis tetapi **tidak compile** (`registry`, `errors`, `repository` belum di-import; `connectAccount` belum ada), dan test-nya menuntut API yang berbeda dari implementasinya. |

## 2. Bukti yang bisa diulang

### 2.1 Sebelum (state sesi sebelumnya)

```
$ go build ./...
internal/service/oauth_flow_device.go:93:66: undefined: registry
internal/service/oauth_flow_device.go:145:6: undefined: errors
internal/service/oauth_flow_device.go:218:20: s.connectAccount undefined
```

### 2.2 Sesudah

```
$ go test ./...                                     # semua paket app-serv hijau
$ go test -race ./internal/service/ -run Device     # 8 test device flow
$ bash scrypts/gates/go-lint.sh                     # vet, gofmt, staticcheck, golangci-lint: PASS
$ bash scrypts/gates/contract-openapi.sh            # PASS (openapi.json diregenerasi dari YAML)
```

Test yang memegang perilaku: `oauth_flow_device_test.go` (babak start + gerbang eligibilitas),
`oauth_flow_device_poll_test.go` (pending / sukses sekali / reconnect / fail-open / kegagalan yang bisa
diulang), `oauth_client_device_test.go` (bentuk wire poll di upstream httptest + tata urutan parse expiry +
floor satu hari), `handler/oauth_device_test.go` (kontrak HTTP kedua route), `router_oauth_test.go` (kedua
route sesi-gated, verb salah = 405).

### 2.3 Slice C — panel (setelah)

```
$ cd app-ui && npm run check          # svelte-check: 0 errors, 0 warnings
$ npx eslint <berkas yang disentuh>    # bersih
$ npx prettier --check src tests       # bersih
$ npx vitest run tests/schemas/oauth.test.ts   # 56 lulus (16 di antaranya kasus device baru)
$ npx vitest run tests/components/provider-oauth.test.ts  # 4 kasus device baru
```

Bukti slice C masih stub yang digerakkan (`tests/support/model-stub.ts` menambah antrean jawaban
`POST .../oauth/device/poll`), bukan login nyata ke `qoder.com`; yang terakhir itu butuh akun dan masuk
bagian dari K8/SPEC-UI Q23.

## 3. Keputusan

| #   | Keputusan                                                                                                                                     | Alasan                                                                                                                                                                                                                       |
| --- | --------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| K1  | **Verifier PKCE dan machine_id tidak pernah keluar dari gateway.** `POST .../oauth/device/start` menjawab `device_code`; poll hanya mengirim `device_code`. | Owner memutuskan ini (2026-09-27). Reference melempar `codeVerifier` + `_qoderMachineId` ke browser lalu memintanya kembali di tiap poll — itu berarti rahasia PKCE bolak-balik lewat klien tanpa menambah apa pun: `device_code` sudah capabilities yang tak tertebak dan sekali pakai. |
| K2  | **`Peek` ditambahkan ke state store**, `Take` tetap satu-satunya cara mengonsumsi.                                                             | Alur yang di-poll harus bisa membaca konteksnya berkali-kali. Di Redis, `Take` = `GETDEL` (atomik), jadi `Peek` = `GET` dan TTL-nya tidak disentuh.                                                                                       |
| K3  | **State dikonsumsi sebelum connect, bukan sesudah.**                                                                                              | Vendor bisa mengembalikan token yang sama dua kali; aturan §4 "sekali pakai" harus melindungi tulisan, bukan hanya pembacaan.                                                                                              |
| K4  | **Identitas device dibaca fail-open**, callback code flow tetap fail-closed.                                                                  | Paritas reference (`fetchUserInfo` menelan semua error). Menolak login yang sudah dikabulkan vendor karena userinfo mati justru membuat akun pecah dua di percobaan berikutnya. Fallback `qoder-user-<user_id>` tetap bisa dicocokkan. |
| K5  | **Kegagalan upstream dilaporkan, tidak ditelan.**                                                                                                | Reference memetakan semua kegagalan poll ke `poll_failed` lalu modal-nya terus polling sampai timeout 5 menit. Panel menjawab `UPSTREAM_ERROR` dengan alasan vendor, dan rondenya tetap bisa dicoba lagi — operator tahu, bukan menunggu buta. |
| K6  | **Token device tidak di-refresh.**                                                                                                              | Reference: `needsRefresh() === false`, upstream membalas 403. `oauth/refresh` sudah menolak provider tanpa `token_url`, dan entri registry Qoder tidak mendeklarasikannya. Perpanjangan = login ulang.                       |
| K7  | **`connectAccount` diekstrak** dan dipakai callback + device poll.                                                                            | Ekor connect (seal → cari akun → update/create) sebelumnya ada dua salinannya (callback dan import bulk). Aturan dedup yang sama dua kali akan melenceng.                                                                  |
| K8  | **Panel tidak memanggil `window.open`; loop poll satu timer, bukan interval.**                                                                  | Paritas reference berhenti di sini: modal reference membuka tab vendor sendiri dan memakai `setInterval`. Panel sudah punya aturan untuk URL pihak ketiga (tautan, bukan navigasi otomatis — lihat §6.3 dan `docs/PORT/README.md` §3), dan timer yang hanya di-re-arm setelah jawaban tiba tidak bisa menumpuk dua ask yang bersamaan. Reference: `src/shared/components/OAuthModal.js:135-196`. |

Konstanta yang dipindah dari reference (semuanya diuji, tidak ada yang dikira-kira): `expires_in: 300`,
`interval: 2` detik, TTL state 6 menit, `user_code` = 8 karakter pertama nonce (uppercase), PKCE verifier
32 byte base64url, `challenge_method=S256`, poll = `GET` dengan `nonce` + `verifier` + `challenge_method`
(tanpa `machine_id`), status pending `202` dan `404`, floor expiry 1 hari, default expiry 30 hari.

## 4. Yang sengaja TIDAK dikerjakan pass ini

- **Slice D — data plane Qoder.** Menyambungkan akun bukan bagian yang membuatnya dipakai. Belum ada:
  tanda tangan COSY (17 header + body ter-encode), pemilihan host berdasar jenis token
  (`dt-` → `api3.qoder.sh`, `jt-` → `api2.qoder.sh` di intl), pertukaran PAT `pt-` → `jt-` lewat
  `POST /api/v1/jobToken/exchange` dengan cache TTL (satu hari, buffer lima menit), daftar model, dan
  baca kuota. Titik masuknya sudah dipetakan: `provider.Plugin` (`internal/provider/plugin.go:39`),
  peta konektor per provider id (`internal/provider/connectors.go:35`, `cmd/app-serv/provider_wiring.go:58`),
  preseden penuh `provider.NewOpenCode`.
  Dua hal yang harus dibuktikan lebih dulu di slice D: `Credential` saat ini tidak membawa machine_id
  (`internal/dataplane/credential.go:37` hanya mengisi `Account` dan `ProjectID`, padahal
  `provider.Credential.Metadata` tersedia), dan round-trip `account.machine_id` lewat JSONB
  (`internal/repository/postgres/endpoint_jsonb.go:111,126`) **belum punya test** — COSY akan memakai ulang
  nilai itu di setiap request, jadi hilangnya ia di jalur baca tidak boleh ketemu nanti.
- **Kuota Qoder.** `internal/service/quotafetch/` belum punya keluarga `qoder`, URL-nya di sana masih
  hardcoded (tidak membaca `transport.usage.url`), **dan tidak ada satu pun pemanggil `Fetch`** — jadi
  kuota Qoder menunggu wiring itu, bukan menunggu Qoder. DTO `Quota`/`QuotaWindowResponse` juga tidak
  punya `unit`, padahal kuota Qoder dilaporkan dalam kredit (`userQuota` + `orgResourcePackage`) dan
  `remaining`-nya jumlah absolut, bukan persen (SPEC gap Q15).
- **`Features.UsageAPIKey`.** Sudah ada di registry dan sudah aktif untuk qoder/qoder-cn
  (`registry.yaml:745`), tetapi belum ada kode Go yang membacanya.

## 5. Gerbang

- Tidak ada `any`/`interface{}` baru; setiap tubuh request masuk struct tervalidasi
  (`OAuthDevicePollRequest` dengan `validate:"required,max=64"`).
- Tidak ada kredensial di log maupun di respons: hanya `token_hint` ter-mask (K4 §7.4).
- Setiap berkas baru di bawah 250 baris dan memakai header §1.2; `oauth_flow_device.go` dan test-nya
  masing-masing dipecah jadi start/poll karena melewati batas.
- Uang muka aturan lama dipertahankan: `router_session_sweep_test.go` tidak perlu daftar baru — kedua
  route device menjawab 401 untuk anonim, dan itu justru yang diuji sapuan itu.
