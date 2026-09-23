# 003-ENDPOINT-READINESS.md: Audit Kesiapan Endpoint app-serv

Dokumen kerja hasil audit endpoint pada 2026-09-20. Bukan kontrak; kontrak tetap
`docs/SPEC-API/001-SPEC-API.md`. Temuan yang diterima owner sudah diedit ke spec
di PR yang sama dengan dokumen ini (lihat §6).

| | |
|---|---|
| **Status** | audit selesai; F2 dan F3 diterima owner (2026-09-20) dan F3 sudah diimplementasikan sebagai P4 pada hari yang sama |
| **Dibuat** | 2026-09-20, dari `app-serv/internal/router/*.go` (main) dibandingkan dengan SPEC-API-001 §7.1 sampai §7.15 |
| **Kaitan** | SPEC-API-001 §2.2 (eksklusi), §7.6 (drift), §7.16 sampai §7.18 (section baru), §10 (fase P4) |

## 1. Metode

Tiga langkah:

1. Ekstraksi route terdaftar dari tiga file registrasi (`router.go`, `router_dataplane.go`,
   `router_media.go`), termasuk verifikasi bahwa tidak ada registrasi route di luar package
   `internal/router` (grep `HandleFunc|mux.Handle` di seluruh `cmd/` dan `internal/`: kosong).
2. Ekstraksi tabel route SPEC-API-001 §7.1 sampai §7.15 sebagai kontrak pembanding.
3. Pemetaan 14 area yang owner daftarkan ke section spec dan ke route yang ada.

Catatan referensi: checkout 9Router ternyata tersedia di `/home/rusmanadodi/apps/9router`
(spec mencatat `/home/rusmanadodi/ai-gateway`, path lama). Ini dipakai untuk membangun
desain F3 dari perilaku reference yang sesungguhnya, dan membuka parity spot-check
SPEC-API-002 §9 yang selama ini tertunda (lihat F5).

## 2. F1: Versioning, PASS

Semua route dibangun dari satu konstanta `APIVersion = "/api/v1"`
(`internal/router/router.go:28`), mengikuti §11.1 (satu prefix versi untuk management
dan data plane). Tidak ada path `/api/...` hardcoded di file registrasi, dan tidak ada
route yang mendaftar di luar package router. Versi `/api/v2` belum ada, sesuai §4:
breaking change baru memunculkannya.

## 3. Pemetaan 14 area owner

| Area owner | Section spec | Route di router | Status |
|---|---|---|---|
| Endpoint & Key | §7.3 + §7.5 | gateway-keys (5), endpoints CRUD + test (7), keys CRUD + bulk (5), endpoints/bulk, oauth/bulk | CLOSED (perjalanan dipin test, lihat §8) |
| Provider | §7.4 | providers (3), provider-nodes CRUD + test (6), OAuth start/callback/status/refresh (4) | lengkap |
| Combo & Vision Adapter | §7.7 + §7.8 | combos CRUD + test (6), vision-adapter GET/PUT | CLOSED (perjalanan dipin test, lihat §9) |
| Usage | §7.12 | summary, timeseries, records, records/{request_id} | lengkap |
| Quota Tracker | §7.12 | quotas, quotas/{endpoint_id} GET/PUT | lengkap |
| Token Saver | §7.9 | token-saver GET/PUT; engine native sudah Closed lewat 002 | lengkap |
| Skill | tidak ada (dulu §2.2) | tidak ada | F3, lihat §5 |
| Media Provider | §7.10 | media-providers (3) + speech, transcriptions, voices, images, videos, search | lengkap |
| Playground Chat | tidak ada (dulu §2.2) | tidak ada | F3, lihat §5 |
| Proxy Pools | §7.11 | proxies CRUD + test (6) | lengkap |
| API Docs | tidak ada (dulu §2.2) | tidak ada | F3, lihat §5 |
| Changelog | tidak ada (dulu §2.2) | tidak ada | F3, lihat §5 |
| Console Log | §7.13 | logs/console GET/DELETE, logs/requests (3) | lengkap |
| Setting | §7.14 | settings GET/PATCH | lengkap |

Data plane §7.15 lengkap juga: `chat/completions`, `messages`, `responses`, `models`,
`messages/count_tokens`, `embeddings` (`router_dataplane.go:33-47`). Semua fase P0
sampai P3 sudah ada route-nya di produksi.

## 4. F2: Drift `GET /models/custom`

Router mendaftarkan `GET /api/v1/models/custom` (`router.go:147`, daftar custom model,
teruji di `router_models_test.go:31`), tetapi tabel §7.6 hanya mencantumkan `POST` dan
`DELETE` untuk path yang sama. Implementasi di depan spec, melanggar aturan spec-first
("setiap deviasi wajib mengedit spec di PR yang sama").

**Keputusan owner (2026-09-20): route dipertahankan**, karena panel memang membutuhkan
daftar custom model yang sudah dibuat. Spec diedit: satu baris `GET /api/v1/models/custom`
masuk tabel §7.6, fase P2.

## 5. F3: Empat area yang dulu dieksklusikan, sekarang dimau

§2.2 mengecualikan "translator playground page, skills, basic-chat page" secara eksplisit,
dan menutup dengan "anything not listed in §2.1 is assumed not wanted; it may be added
later via a new spec only". Owner memutuskan (2026-09-20): empat area ini adalah fitur
yang harus ada dan akan diimplementasikan. Grounding dari reference sebelum menulis spec:

| Area | Bukti di reference | Keputusan desain |
|---|---|---|
| Skill | `src/shared/constants/skills.js`: skill = dokumen `SKILL.md` per kapabilitas di repo GitHub produk, URL raw-nya user tempel ke AI agent; halaman dashboard hanya merender katalog statis | Port bentuk yang sama: `GET /api/v1/skills` (S) melayani katalog embedded (id, name, description, endpoint, entry, raw_url, blob_url), satu entri per kapabilitas yang gateway ini benar-benar layani. Tidak ada DB, tidak ada CRUD, isi `SKILL.md` tetap konten repo owner |
| Playground Chat | `src/app/(dashboard)/dashboard/basic-chat/page.js`, terpisah dari halaman translator | Panel pannelAI adalah app terpisah, jadi playground memanggil route data plane §7.15 dengan gateway key dari §7.3, persis seperti CLI tool. Tanpa endpoint baru; halamannya milik SPEC-UI |
| API Docs | reference menyimpan docs di gitbook eksternal | Gateway self-hosted tidak boleh mengasumsikan internet, jadi kontrak dilayani sendiri: `GET /api/v1/openapi.json` (S), embedded saat build, memuat setiap route terdaftar |
| Changelog | `src/shared/components/ChangelogModal.js:21` fetch `GITHUB_CONFIG.changelogUrl` dari browser | pannelAI melayani riwayatnya sendiri: `GET /api/v1/changelog` (S), `{data: [{version, date, title, notes}]}`, embedded saat build, read-only, tanpa tabel DB |

Spec menerima section baru §7.16 (Skills), §7.17 (API Docs), §7.18 (Changelog), catatan
playground di §7.15, dan fase P4 di §10. Translator playground tetap dieksklusikan.

## 6. F4 dan F5: catatan penutup

- **F4 (bukan masalah):** `POST /embeddings` tercantum di §7.10 dan §7.15, tetapi
  diregistrasi tepat satu kali (`router_dataplane.go:41`). Tidak ada konflik mux.
- **F5 (tugas terpisah):** checkout reference tersedia di
  `/home/rusmanadodi/apps/9router`, jadi parity spot-check yang §9 SPEC-API-002 catat
  sebagai "tertunda sampai checkout tersedia" sekarang bisa dilakukan. Tidak dikerjakan
  di audit ini; spec 002 tidak diedit untuk itu di PR ini.

## 7. Jumlah route

Spec §7.1 sampai §7.15 mendefinisikan 89 route; router mendaftarkan 90. Satu-satunya
selisih adalah F2. Setelah edit spec ini, kedua sisi 90.

## 8. Penutupan area Endpoint & Key (2026-09-20)

Status **CLOSED**. Bukti perjalanan yang dipin test (commit `c3a2f56` dan `ca9e677`),
semuanya table-driven sesuai TDD.md §2.5 dan dipetakan ke OWASP:

| Perjalanan | Test | Bukti |
|---|---|---|
| Create mengembalikan plaintext tepat sekali, setelah itu hanya `key_hint` | `TestGatewayKey_Create_HappyPath`, `TestGatewayKey_Get_AfterCreate` | handler |
| Kredensial diterima dari salah satu header (Bearer / X-Api-Key, OWASP: klien OpenAI dan Anthropic) | `TestBearerTokenExtraction` (9 kasus, termasuk dua header sekaligus dan scheme asing) | handler |
| Settings gagal dibaca, auth tetap menolak (OWASP A01: deny by default) | `TestChatService_AuthenticateRecordsKeyUse`, 2 kasus baru | service |
| Kunci revoked berhenti melayani data plane, tanpa oracle: jawaban identik dengan kunci yang tak pernah ada | `TestRevokedKeyStopsServingTheDataPlane` | service |
| Revoke terminal, tidak bisa dihidupkan lewat PATCH maupun revoke ulang | `TestGatewayKey_StatusTransitions`, `TestGatewayKey_Revoke_IsTerminal` | domain, handler |
| Nama duplikat: create kedua = 409 CONFLICT, nama lain tetap berhasil | `TestGatewayKey_Create_DuplicateNameConflict`, `TestGatewayKey_Update_RenameCollision` | handler |
| Counter `request_count`/`last_used_at` maju per panggilan sah, gagal tulis tidak menggagalkan request | `TestChatService_AuthenticateRecordsKeyUse`, `TestChatService_AuthenticateSurvivesAFailedKeyUseWrite` | service |
| Route tanpa session = 401, verb salah = 405 (OWASP A01: enforcement per method) | `TestVersionedRoutesRequireSession`, `router_verb_test.go` | router |
| Rule keep-last-key §7.5 (endpoint `api_key` wajib menyisakan ≥1 kunci aktif) | aggregate `Endpoint.RemoveKey` → CONFLICT | domain, service |

Kontrak mesin-baca: dokumen `openapi.json` yang dilayani (§7.17) mengunci kelima verb
gateway-keys dan endpoint keys lewat coverage test dua arah di
`TestOpenAPICoversEveryRegisteredRoute`, jadi method DELETE tidak bisa hilang dari
kontrak tanpa gagal build.

Sisa gap pada audit ini: tidak ada. Area berikutnya yang bisa ditutup dengan pola yang
sama: Provider, Combo & Vision Adapter.

## 9. Penutupan area Combo & Vision Adapter (2026-09-20)

Status **CLOSED**. Audit menemukan cakupan perjalanan area ini sudah menyeluruh dari
domain sampai handler; satu-satunya gap adalah pengujian gerbang akses dan tabel verb di
level mux, ditutup pada commit `6cd9054`. Bukti per perjalanan, dipetakan ke TDD.md
(table-driven) dan OWASP:

| Perjalanan | Test | Bukti |
|---|---|---|
| Create combo: ref wajib deref satu tingkat (model, combo, atau alias); judge fusion wajib ada dan ikut tervalidasi; nama duplikat = CONFLICT | `TestComboService_Create` (tabel termasuk ref alias, combo, campuran, ref/judge tak terresolve, fusion tanpa judge) | service |
| Update combo: rename dicek terhadap nama tersimpan; daftar model atau strategi berubah mereset rotasi sticky | `TestComboService_Update`, `TestComboService_UpdateResetsRotationWhenTheListChanges` | service |
| DELETE combo ditolak CONFLICT selama alias masih menunjuknya, teruji sampai kabel (mux nyata, seeding alias lewat PUT /models/aliases) | `TestComboService_DeleteRefusesWhileAnAliasReferencesIt`, `TestComboDeleteThroughMux_ConflictWhileReferenced`, `TestManagementRoutes_DuplicateComboNameIsConflict` | service, router |
| Prioritas terurut, prioritas sama stabil (urutan input dipertahankan) | `TestNewCombo_OrdersModelsByPriority`, `TestNewCombo_KeepsEqualPrioritiesStable` | domain |
| Strategi: fallback, round_robin, fusion beserta rotasi dan batas sticky | `TestComboStrategy_NextOrder`, `TestRotationRequestIndex_Boundaries`, `TestRotateRefs`, `TestComboService_OrderRotatesAcrossRequests` | domain, service |
| Rotasi gagal/store absen: fallback ke urutan prioritas tersimpan, bukan gagal request | `TestComboService_OrderFallsBackWhenTheStoreFails`, `TestComboService_OrderWithoutAStore` | service |
| Probe combo: satu hasil per ref sesuai urutan tersimpan, anggota mati = hasil bukan error, judge fusion diprobe terakhir dengan role tersendiri | `TestComboTestService_Test` (tabel 4 kasus), `TestComboTestHandler_ReportsEveryReference`, `TestComboTestHandler_ADeadMemberIsAResult`, `TestComboTestHandler_UnknownCombo` | service, handler |
| Vision adapter: PUT = replace penuh, model wajib vision-capable (ditolak VALIDATION_ERROR per nama), round-trip GET=PUT, idempoten, model yang dilepas hilang | `TestVisionAdapterService_Replace` (tabel termasuk model non-vision), `TestVisionAdapterHandler_PutRoundTrip`, `TestVisionAdapterService_ReplaceIsIdempotent`, `TestVisionAdapterService_ReplaceRemovesDroppedModels` | service, handler |
| Augmenter: model yang sudah vision-capable atau adapter mati tidak diaugmentasi; rotasi advisory tidak menggagalkan serving | `TestVisionAugmenter_DeclinesForACapableModel`, `TestVisionAugmenter_DeclinesWhenTheAdapterIsDisabled`, `TestVisionAugmenter_AdvisoryRotationFailureDoesNotBlockServing` | service |
| Session gate dan verb table §7.7/§7.8: 401 tanpa session untuk kedelapan route, 200 dengan session (benign control), verb tak terdaftar = 405 METHOD_NOT_ALLOWED meski session valid (OWASP A01: function-level + per-method) | `TestComboAndVisionRoutes_SessionGated`, `TestComboAndVisionRoutes_VerbEnforcement` (commit `6cd9054`) | router |

Kontrak mesin-baca: kelima verb `/combos` dan keduanya `/vision-adapter` terkunci di
`openapi.json` yang dilayani lewat coverage test dua arah, sama seperti area Endpoint &
Key. Sisa gap: tidak ada. Area berikutnya dengan pola yang sama: Provider.
