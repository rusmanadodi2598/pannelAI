# 036-QODER-AUTH-READINESS.md: Provider Qoder bisa di-login — device flow OAuth, dan jalur PAT yang masih terbuka

Dokumen kerja pass `app-serv` (dan nanti `app-ui`) untuk menghubungkan akun **Qoder** ke panel. Pass ini
mengerjakan permintaan owner: "Provider Qoder, untuk OAuth & PAT, sesuai REFERENCE (9router)". Bukan
kontrak; kontrak tetap `docs/SPEC-API/001-SPEC-API.md` (wire) dan `docs/CONTRACT/001-CONTRACT-API-V1.yaml`
(skema). Pola mengikuti `002-PORT-PROVIDER.md`: temuan bernomor F, bukti yang bisa diulang, keputusan di
depan implementasi.

|                    |                                                                                                                                                                                    |
| ------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**         | **Slice A selesai 2026-09-27 (commit `0d519fa`). Slice B selesai lebih dulu (commit `c0e3034`). Slice C (modal device di panel) selesai 2026-09-27 (commit `0bd3b7c`). Slice D: langkah 1 (primitif COSY + encoder, terbukti hidup) `63a445a`, langkah 2 (konektor + exchange PAT + plumbing machine_id) `e4e4d30`, langkah 3a (decoder envelope + seam) `33d5fda`, langkah 3b (body agent + katalog) `7cdd7e8`, langkah 3c (baca kuota) menyusul commit ini.** |
| **Mechanism**      | PORT (reference → Go) + TDD                                                                                                                                                          |
| **Scope**          | `app-serv/.` (service, repository, handler, schema, router, kontrak) dan `app-ui/.` (schema, api, sections OAuth). Slice D menambah `internal/provider` + `internal/dataplane` + `internal/service/quotafetch`.                                     |
| **Permintaan owner** | (1) Qoder bisa di-OAuth. (2) Qoder bisa diisi PAT. (3) Paritas dengan reference. (4) Verifier device flow dipegang server, panel hanya mengirim `device_code`. (5) Qoder harus benar-benar terpakai — COSY, exchange PAT, dan baca kuota ikut pass ini (keputusan owner 2026-09-27). |
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

- **Slice D langkah 1 selesai (commit `63a445a`): primitifnya.** Encoder body, payload terenkripsi, dan
  komposisi tanda tangan COSY sudah ada di `internal/provider/qoder_{constants,encoding,cosy,crypto}.go` dan
  sudah diuji sampai vendor benar-benar menerima request bertanda tangan Go ini (§5). Doc reference menyebut
  "17 header"; yang klien kirim sebenarnya 19 (`Authorization`, 15 `Cosy-*`, `Login-Version`, `X-Request-Id`)
  dan itu yang diporting.
- **Slice D langkah 2 — konektor.** Belum ada yang memanggil primitif di atas. Yang dibutuhkan:
  `provider.Plugin` (`internal/provider/plugin.go:39`) per id provider lewat peta konektor
  (`internal/provider/connectors.go:35`, `cmd/app-serv/provider_wiring.go:58`, preseden penuh
  `provider.NewOpenCode`), dengan `Endpoint()` memilih host berdasar awalan token (`dt-` → `api3`,
  `jt-` → `api2` di intl; CN satu gateway) dan `ApplyAuth()` menandatangani byte yang benar-benar keluar.
  Dua hal yang harus diselesaikan lebih dulu: `Credential` saat ini tidak membawa machine_id
  (`internal/dataplane/credential.go:37` hanya mengisi `Account` dan `ProjectID`, padahal
  `provider.Credential.Metadata` tersedia), dan round-trip `account.machine_id` lewat JSONB
  (`internal/repository/postgres/endpoint_jsonb.go:111,126`) **belum punya test** — COSY akan memakai ulang
  nilai itu di setiap request, jadi hilangnya ia di jalur baca tidak boleh ketemu nanti.
- **Slice D langkah 3 sudah dikerjakan (3a envelope, 3b body+katalog, 3c kuota). Yang tersisa darinya, dan
  belum diport:** rewrite lampiran (`rewriteQoderMessageAttachments` + upload gambar ke
  `/api/v2/image/upload`, multipart field `file`) sehingga turn bergambar mengirim URL OSS alih-alih base64
  inline; pemilihan **context tier** (200K/400K/1M) dari `model_config.context_config`; dan satu
  completion hidup, yang butuh akun dengan kuota. Body ter-encode (`Encode=1`) juga belum dipakai — vendor
  menerima body polos (§5.1) dan reference memakai encoder untuk menghindari pola WAF, bukan karena
  wajib; ini kandidat perubahan berikutnya, bukan bug yang diketahui.
- **Catatan lama, sudah lewat.** Endpoint chat menolak body hasil translasi OpenAI
  (`400 None flow nodes found for router agent_router`) dan menjawab sebagai **envelope SSE**
  (`{headers, body, statusCodeValue, statusCode}` per frame, plus `event:finish` berisi timing), bukan chunk
  OpenAI. Jadi ini port nyata — builder body agent (chat record id stabil, `model_config` dari katalog hidup,
  konteks tier, penanganan gambar) dan decoder envelope — bukan perubahan konfigurasi reader yang ada.
  Keputusan `Encode=1` duduk di sisi builder, karena body ter-encode dan parameter itu satu pilihan.
- **Kuota Qoder — ikut slice D (keputusan owner 2026-09-27).** `internal/service/quotafetch/` belum punya
  keluarga `qoder`, URL-nya di sana masih hardcoded (tidak membaca `transport.usage.url`), **dan tidak ada
  satu pun pemanggil `Fetch`** — jadi yang ditagih keputusan ini bukan hanya Qoder, tapi wiring kuota
  keseluruhan. DTO `Quota`/`QuotaWindowResponse` juga tidak punya `unit`, padahal kuota Qoder dilaporkan
  dalam kredit (`userQuota` + `orgResourcePackage`) dan `remaining`-nya jumlah absolut, bukan persen (SPEC
  gap Q15). Baca kuota memakai bearer biasa, bukan COSY: `GET .../api/v2/quota/usage`, dan sebuah PAT harus
  bertukar ke job token lebih dulu karena endpoint itu menolak `pt-`.
- **`Features.UsageAPIKey`.** Sudah ada di registry dan sudah aktif untuk qoder/qoder-cn
  (`registry.yaml:745`), tetapi belum ada kode Go yang membacanya.

## 5. Bukti hidup slice D (2026-09-27, PAT owner, endpoint intl)

Dipanggil langsung dari mesin kerja, hanya bentuk jawabannya yang dicatat (token dipotong 4 karakter).

**`POST https://openapi.qoder.sh/api/v1/jobToken/exchange`** → `200`:

| field                        | bentuk nyata                                   |
| ---------------------------- | ---------------------------------------------- |
| `token`                      | `jt-…`, 27 karakter                             |
| `refresh_token`              | `jrt-…`, 28 karakter                            |
| `expires_at`                 | string RFC3339 UTC 20 karakter (`2026-09-…`)    |
| `expires_in`                 | **86400000** — milidetik, bukan detik           |
| `refresh_token_expires_at`   | string RFC3339 UTC                              |
| `refresh_token_expires_in`   | 172800000 (48 jam, dalam ms)                    |
| `created_at`                 | string RFC3339 UTC                              |

`GET https://openapi.qoder.sh/api/v1/userinfo` dengan `Authorization: Bearer jt-…` → `200` berisi `id`,
`name`, `username`, `email`, `organization_id` (string kosong untuk akun ini), `avatar`, `source`,
`current_sign_in_at` (objek `{seconds, nanos}`), `is_highest_tier`. Jadi `OAuthIdentity` yang membaca
`id`/`name`/`email` sudah cocok; tidak ada field identitas baru yang perlu.

**Dua hal yang ini ubah dari asumsi reference:**

1. `expires_in` di jalur PAT adalah **milidetik**. Reference (`open-sse/services/qoderModels.js:99`)
   menjumlahkannya ke `Date.now()` tanpa `*1000`, jadi cabang itu salah hitung — ia tidak ketahuan karena
   upstream selalu mengirim `expires_at`. Port Go memakai `expires_at` sebagai sumber benar dan memperlakukan
   `expires_in` sebagai ms, bukan mengikuti bug-nya.
2. Job token **bisa di-refresh**: ada `refresh_token` (`jrt-…`) berumur 48 jam dengan expiry sendiri, padahal
   reference tidak pernah menyentuhnya (ia cache 24 jam lalu bertukar ulang dari PAT). Slice D boleh memilih
   jalur yang lebih sederhana — tukar ulang dari PAT saat dekat expiry — selama PAT-nya tersimpan; itu yang
   reference lakukan, dan itu yang port ini ikuti. Yang tidak boleh: menyimpan `jt-` sebagai credential dan
   membuang `pt-`-nya, karena `jt-` mati dalam 24 jam.

**Yang sudah dibuktikan hidup (slice D langkah 1, `internal/provider/qoder_live_test.go`):**

```
$ PANNELAI_QODER_PAT='pt-…' go test -tags=integration ./internal/provider/ -run QoderLive -v
exchange    PASS  expires_in=86400000 ms, expires_at=2026-09-28T14:17:01Z, refresh token ada
userinfo    PASS  id + name="Dodi Rusmana" + email, organization_id kosong
model list  PASS  200, 61.542 byte katalog — request ditanda tangani port Go
```

Model list yang menjawab 200 itu adalah bukti tanda tangan COSY-nya benar: vendor memvalidasi
`Authorization: Bearer COSY.<payload>.<md5>`, `Cosy-Key` yang membungkus kunci AES, `Cosy-Bodyhash`, dan
casing header (`Cosy-Machineid`, bukan `Cosy-MachineID`). Satu byte meleset di komposisi signature, satu
padding PKCS#7 salah, atau satu header salah huruf, jawabannya 401/403 — bukan 200 dengan katalog.

**Temuan yang membatalkan satu aturan reference:** `api3.qoder.sh` (host token device) **melayani job token
`jt-`** untuk `/algo/api/v2/model/list` hari ini — reference mengklaim host itu menolak `jt-` dengan 403
"Login expired" sehingga trafik job token harus ke `api2`. Yang diuji adalah model list; jalur chat belum
diukur, jadi pemilihan host per jenis token tetap diporting (ia tidak berbahaya) tapi tidak boleh dianggap
aturan vendor yang sudah dipastikan. Test-nya mencatat dua-duanya supaya perubahan upstream ketahuan, bukan
dilupakan.

**Yang masih belum dibuktikan:** body chat `agent_chat_generation` (bukan bentuk OpenAI — record id stabil,
`model_config` dari katalog hidup, konteks tier, upload gambar) dan unwrap SSE-nya, tanda tangan pada request
ber-body, serta respons `/api/v2/quota/usage`.

### 5.1 Probe bentuk body chat (2026-09-27, sekali jalan, lalu dibuang)

Pertanyaan yang dijawab: apakah gateway bisa mengirim hasil translasi OpenAI mentah ke endpoint chat Qoder,
atau executor bespoke reference itu wajib? Jawabannya **wajib**, dan probe-nya juga menghasilkan dua fakta
lain yang tidak ada di catatan reference:

```
POST https://api2.qoder.sh/algo/.../agent_chat_generation?FetchKeys=llm_model_result&AgentId=agent_common
     body = {"model":"ultimate","stream":true,"messages":[{"role":"user","content":"Say OK."}]}
     COSY-signed oleh port Go, dikirim polos dan dikirim lewat encoder → dua-duanya sama

HTTP 200
data:{"headers":{"Content-Type":["application/json"]},
     "body":"{\"code\":\"400\",\"message\":\"[FAIL]node:agent_router msg:None flow nodes found for router agent_router\"}",
     "statusCodeValue":400,"statusCode":"BAD_REQUEST"}

event:finish
data:{"firstTokenDuration":…,"totalDuration":…,"serverDuration":…}
```

1. **Tanda tangan diterima pada POST ber-body.** Vendor menolak di lapisan *routing agent* (`flow nodes`),
   bukan di lapisan autentikasi — artinya `Cosy-Bodyhash` atas byte yang benar-benar keluar sudah cocok. Ini
   membuktikan jalur tanda tangan untuk request ber-body, yang model list (body kosong) tidak bisa buktikan.
2. **Encoder body juga diterima.** Versi `Encode=1` dijawab identik dengan versi polos, jadi obfuscation-nya
   benar menurut server, bukan hanya menurut vector reference.
3. **SSE-nya adalah envelope, bukan stream OpenAI.** Frame `data:` berisi `{headers, body, statusCodeValue,
   statusCode}` — body jawaban dibungkus JSON di dalam string — plus satu `event:finish` berisi timing.
   Reader stream gateway tidak bisa membaca bentuk ini apa adanya, jadi `wrapQoderSSE` reference memang
   diperlukan dan tidak bisa dipangkas jadi konfigurasi.

Konsekuensinya untuk slice D: executor Qoder butuh (a) builder body agent, (b) decoder envelope SSE, dan (c)
peta `model_config` dari katalog hidup. Itu unit berikutnya; primitif yang diuji di atas (exchange, identity,
encoder, signer) adalah fondasinya dan sudah terbukti sampai sini.

### 5.2 Slice D langkah 3a — decoder envelope sudah landed

**Coalescer reference sengaja tidak diport.** Decoder ditulis sebagai transform baris yang melestarikan JSON
jawaban apa adanya, tanpa menggabungkan frame finish dengan frame usage. Alasannya bukan menghemat kerja:
gateway ini sudah menegakkan "satu `finish_reason` di wire" dan sudah mengambil usage dari frame penutup
kedua (draft 034 F2, commit `6b68bb4` dan `50340a8`) — persis pola finish-then-usage Qoder. Coalescer kedua
di jalur ini berarti aturan yang sama ditulis dua kali, dan yang kedua selalu jadi yang melenceng.

**Yang memang harus ada: peek frame pertama.** Vendor menyembunyikan status sebenarnya di dalam body — `200`
di HTTP, `403`/`code 112` di frame (§5.1). Kalau frame itu di-pipe apa adanya, akun habis kuota tercatat
sebagai jawaban sukses dan ditagih. Jadi refusal sebelum frame pertama kembali sebagai `*UpstreamError`
dengan status vendor, dan jalur failover + klasifikasi kuota (`IsQuotaError`) memperlakukannya seperti
kegagalan HTTP biasa. Seam-nya opsional (`provider.StreamEnvelope` di `internal/provider/plugin.go`); test
di `internal/dataplane/transport_envelope_test.go` menjaga tiga hal: body ter-unwrap sampai ke klien,
refusal jadi kegagalan, dan provider yang tidak mendeklarasi seam menerima body-nya utuh tanpa perubahan.

### 5.3 Rantai penuh, terbukti hidup (langkah 3b + 3c)

```
$ PANNELAI_QODER_PAT='pt-…' go test -tags=integration ./internal/provider/ -run QoderLiveChat -v
the vendor refused the call: status=403 quota=true message={"pricingUrl":"https://qoder.com/pricing?client=qoder"}
--- PASS: TestQoderLiveChatBodyIsAcceptedByTheVendor
```

Yang dilewati rantai ini, satu per satu: katalog dibaca dengan tanda tangan akun → body agent dibangun
dari hasil translasi OpenAI → vendor **menerima** request (tidak ada lagi "flow nodes") → jawaban dibongkar
dari envelope → block penagihan dikenali sebagai kegagalan kuota, bukan jawaban sukses. Urutannya penting:
sebelum langkah 3b, request yang sama ditolak di lapisan routing; sebelum envelope decoder, penolakan itu
akan mengalir ke klien sebagai konten dan tercatat sebagai jawaban yang ditagih.

Yang **tidak** bisa dibuktikan akun ini: satu completion sukses. Kuotanya habis (`code 112`), dan itu
justru menguji jalur yang benar untuk pembukuan. Konsekuensinya tercatat di §7.

## 6. Gerbang

- Tidak ada `any`/`interface{}` baru; setiap tubuh request masuk struct tervalidasi
  (`OAuthDevicePollRequest` dengan `validate:"required,max=64"`).
- Tidak ada kredensial di log maupun di respons: hanya `token_hint` ter-mask (K4 §7.4).
- Setiap berkas baru di bawah 250 baris dan memakai header §1.2; `oauth_flow_device.go` dan test-nya
  masing-masing dipecah jadi start/poll karena melewati batas.
- Uang muka aturan lama dipertahankan: `router_session_sweep_test.go` tidak perlu daftar baru — kedua
  route device menjawab 401 untuk anonim, dan itu justru yang diuji sapuan itu.
- **SYSTEM_MAP untuk langkah 1 slice D: N/A, tidak ada perubahan struktural.** Belum ada route baru, tidak
  ada tipe data baru, tidak ada worker baru — primitif provider (`qoder_constants.go`, `qoder_encoding.go`,
  `qoder_cosy.go`, `qoder_crypto.go`) belum dipanggil siapa pun. Peta pemanggilan Qoder (konektor →
  `provider.Connectors` → `dataplane.Transport`) masuk ke §5 SYSTEM_MAP pada commit konektor, bukan di sini.
- Supresi linter hanya satu dan beralasan: `rsa.EncryptPKCS1v15` dan `DecryptPKCS1v15`deprecated di Go 1.26,
  dan skema vendor memang membaca padding itu — tiap suppressed call punya `reason:` sesuai AGENTS.md §1.4.
