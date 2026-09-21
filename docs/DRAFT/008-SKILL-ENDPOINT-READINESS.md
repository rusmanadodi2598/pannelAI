# 008-SKILL-ENDPOINT-READINESS.md: Audit Kesiapan Endpoint Skill app-serv

Dokumen kerja hasil audit endpoint Skill pada `app-serv/.` tanggal 2026-09-20.
Bukan kontrak; kontrak tetap `docs/SPEC-API/001-SPEC-API.md` dan perilaku panel tetap
`docs/SPEC-UI/001-SPEC-UI.md`. Dokumen ini mengikuti mekanisme antislop AFTER: temuan
bernomor, prioritas, bukti, dan rencana perbaikan. Tidak ada kode produksi yang diubah.

| | |
|---|---|
| **Status** | Sebagian CLOSED per 2026-09-21; F1 menunggu push, F3 dan F4 masih terbuka (§10) |
| **Scope** | hanya `app-serv/.` dan kontrak yang diperlukan untuk endpoint Skill |
| **Endpoint** | `GET /api/v1/skills` |
| **Kaitan** | AGENTS.md §1.1, §1.2, §1.3, §1.4, §1.5, §1.9, §2.1; `docs/RULLES/TDD.md`; `docs/RULLES/OWASP.md`; SPEC-API §7.16; SPEC-UI §5.1, §6.10, §12, §14 |
| **Tanggal audit** | 2026-09-20 |

## 1. DURING & AFTER record

User memilih mekanisme `DURING & AFTER`. Untuk pekerjaan ini, mode AFTER dipakai untuk
memeriksa implementasi yang sudah ada tanpa mengubah source. Mode DURING diterapkan pada
rencana berikutnya: setiap perubahan harus dimulai dengan test, menjaga header AGENTS.md,
validasi typed, dan gate penuh sebelum dinyatakan selesai.

Antislop UI dan copy tidak dijalankan pada source backend ini karena endpoint tidak merender
interface. Namun aturan kejujuran konten tetap relevan pada katalog dan URL skill: tautan
harus menunjuk dokumen yang benar-benar tersedia, dan metadata tidak boleh dikarang.

## 2. Metode dan bukti yang dijalankan

1. Membaca `AGENTS.md`, `SYSTEM_MAP.md`, `docs/RULLES/TDD.md`, `docs/RULLES/OWASP.md`,
   SPEC-API §7.16, dan SPEC-UI §6.10/§14.
2. Menelusuri route, handler, response DTO, test, wiring, dan OpenAPI yang disajikan oleh
   `app-serv`.
3. Membandingkan endpoint Skill dengan data plane route yang benar-benar terdaftar.
4. Memeriksa URL raw terhadap branch remote yang dipakai handler.
5. Menjalankan gate backend yang relevan.

Bukti yang sudah diperoleh:

- `internal/router/router.go:204` mendaftarkan `GET /api/v1/skills` dengan session guard.
- `internal/handler/skills.go:102` mengembalikan envelope `{data: [...]}` tanpa DB, Redis,
  atau outbound call.
- Wiring produksi menyiapkan `handler.NewSkillsHandler()` di
  `cmd/app-serv/router_wiring.go:100`.
- `internal/handler/openapi.json:436` mencantumkan operasi Skill, dan
  `TestOpenAPICoversEveryRegisteredRoute` menguji paritas route dan dokumen dua arah.
- `TestP4Routes_SessionGated` menguji 401 tanpa session. `TestP4Routes_ThroughMux`
  menguji 200 dengan session dan 405 untuk POST. Test handler menguji shape, URL derivation,
  ID unik, entry pertama, dan endpoint yang termasuk daftar data plane.
- `go test -race -count=1 ./...` PASS.
- `go vet ./...` PASS.
- `gofmt -l .` PASS.
- `go-headers.sh` PASS untuk 631 file Go.
- `contract-drift.sh` PASS, 11 error codes cocok.
- Semua source file Skill yang diperiksa berada di bawah 220 baris.
- `staticcheck` tidak tersedia di environment audit, sehingga belum ada evidence staticcheck.

## 3. Ringkasan kesiapan

| Area | Status | Kesimpulan |
|---|---|---|
| Route dan session gate | PASS | Route terdaftar dan terlindungi session middleware |
| Method enforcement | PASS | GET dilayani dan POST ditolak 405 melalui mux |
| Handler response | PASS sebagian | Shape typed dan JSON ada, tetapi sumber dokumen eksternal tidak diverifikasi saat serving |
| Validation | N/A untuk GET | Endpoint tidak menerima body, query, path, atau header bisnis yang dipakai handler |
| Auth failure | PASS | 401 route-level teruji |
| Test | PASS sebagian | Happy path, shape, invariant, auth, dan wrong verb ada; remote document existence belum teruji |
| Contract parity | GAP | SPEC-UI §6.10 meminta 2 skill, API §7.16 melayani 7 capability rows |
| Content integrity | GAP | URL `raw_url` aktif di branch `main` mengembalikan 404 |
| Documentation sync | GAP | `SYSTEM_MAP.md` belum menyebut P4/Skill, dan `app-serv/README.md` masih menyatakan P1 lama |
| Lint evidence | GAP | `staticcheck` belum tersedia; line-limit gate hanya melihat file changed |

**Keputusan audit:** endpoint belum siap ditutup sebagai READY karena URL dokumen yang
menjadi keluaran utama katalog saat ini rusak dan dua dokumen kontrak memberikan katalog yang
berbeda. Temuan di bawah harus dipilih owner sebelum implementasi.

## 4. F1 (HIGH): `raw_url` dan `blob_url` menunjuk sumber skill yang tidak tersedia

### Fakta

`internal/handler/skills.go:31-35` menetapkan:

- repository: `rusmanadodi2598/pannelAI`
- branch: `main`
- path: `skills`
- setiap row: `<id>/SKILL.md`

Repository GitHub ada, tetapi `skills/pannelai/SKILL.md` pada branch `main` diuji dengan
HTTP dan menjawab **404**. `find . -name SKILL.md` di working tree juga menemukan nol
file. Karena katalog tetap menyajikan `raw_url` dan `blob_url` seolah dapat dipakai, klien
akan menerima tautan yang tidak mengarah ke dokumen.

### Risiko dan aturan

- R-38 dan C-5: metadata yang tampak final tidak boleh menunjuk konten fiktif atau rusak.
- SPEC-UI §6.10: copy control tidak boleh dikirim untuk link rusak; unavailable state harus
  menyebut source yang tidak tersedia.
- SPEC-API §7.16: route menjanjikan katalog dokumen skill yang dapat dibaca oleh agent.
- OWASP A08: integritas data yang ditampilkan harus diverifikasi sebelum dipercaya.

### Rencana DURING

Owner memilih salah satu jalur, lalu perubahan dimulai dengan test table-driven:

A. Tambahkan `skills/<id>/SKILL.md` yang benar-benar dimiliki project untuk setiap row yang
   tetap dilayani, verifikasi raw URL dan blob URL dengan request nyata, lalu pertahankan
   katalog capability.

B. Ubah sumber dan metadata katalog ke repository/ref/path yang benar-benar tersedia, dengan
   validasi format URL dan test yang menolak row tanpa sumber.

C. Bila owner belum menyediakan sumber, ubah kontrak dan handler agar row tidak menyajikan
   copyable URL sebagai available. Handler harus mengembalikan status metadata yang jelas,
   atau panel menampilkan unavailable state. Jangan melakukan HTTP fetch dari handler GET
   hanya untuk menyembunyikan masalah source; endpoint sekarang sengaja static dan tidak
   memiliki timeout, cache, atau failure policy untuk outbound dependency.

### Kriteria selesai

- Setiap row available memiliki `SKILL.md` yang dapat diambil dari `raw_url` dan dibuka dari
  `blob_url`, dibuktikan dengan test/integration check yang tidak dijalankan sebagai default
  unit test tanpa jaringan.
- Tidak ada copyable link 404.
- Source, ref, path, dan ID katalog sama di handler, test, SPEC-API, SPEC-UI, dan README.
- Error source tidak bocor sebagai stack trace atau error client yang tidak terstruktur.

## 5. F2 (HIGH): SPEC-UI §6.10 tidak sesuai dengan katalog API §7.16

### Fakta

SPEC-UI §6.10 meminta tepat dua entry: `/antislop` AI dan SuperPowers, tanpa directory
extras. SPEC-API §7.16 dan handler melayani tujuh row: satu entry `pannelai` dan enam
capability rows. `app-ui/src/lib/navigation.ts` masih menandai Skill sebagai planned, dan
`docs/SPEC-UI/001-SPEC-UI.md` §14 Q2 masih menyebut source belum diputuskan. Walau scope
implementasi ini hanya `app-serv`, kontrak yang tidak sejalan membuat endpoint tidak dapat
dinyatakan siap untuk konsumen panel.

### Risiko dan aturan

- CDD/spec-first: satu endpoint tidak boleh mempunyai dua definisi isi.
- R-38: UI tidak boleh menampilkan entry yang tidak berasal dari data nyata.
- SPEC-UI §6.10 dan §7.16 saling memberi acceptance criteria yang berbeda.
- Test saat ini hanya membuktikan katalog internal konsisten dengan dirinya sendiri, bukan
  konsisten dengan SPEC-UI.

### Rencana DURING

Owner harus memilih source of truth:

A. Capability catalog menjadi source of truth. Amandemen SPEC-UI §6.10 dari dua skill
   menjadi katalog capability yang dilayani API, termasuk entry-first, endpoint, raw URL,
   blob URL, unavailable state, dan aturan copy. Lalu update navigation dari planned setelah
   panel benar-benar siap. Ini paling konsisten dengan implementasi `app-serv` dan §7.16.

B. Dua owner-selected skills menjadi source of truth. Ubah `skillCatalog` di handler menjadi
   dua row yang benar-benar bersumber, revisi SPEC-API §7.16 dan OpenAPI bila perlu, lalu
   update test route coverage. Capability skill rows tidak boleh tetap tersembunyi di API.

C. Pisahkan dua konsep dengan dua endpoint atau field contract yang eksplisit. Pilihan ini
   lebih mahal dan tidak disarankan karena menambah surface tanpa kebutuhan yang terbukti.

### Kriteria selesai

- Satu katalog normatif dipilih dan disebut sama di SPEC-API, SPEC-UI, handler, dan test.
- Tidak ada test yang hanya mengasertif jumlah row; test memuat expected IDs dan memverifikasi
  seluruh row sesuai kontrak.
- Jika kontrak berubah, `SYSTEM_MAP.md` diperbarui pada perubahan yang sama bila topology,
  data source, atau service interaction berubah.

## 6. F3 (MEDIUM): test belum memenuhi seluruh strict-test acceptance criteria

### Fakta

Test yang ada cukup untuk endpoint static dasar, tetapi belum memenuhi seluruh klaim P4 yang
tertulis di SPEC-API §10: "happy/validation/auth per route". GET Skill tidak menerima input
payload sehingga validation body tidak berlaku secara langsung, tetapi test saat ini belum
mendokumentasikan pengecualian itu. Selain itu:

- `TestSkillCatalog_Invariants` adalah satu loop atas global catalog, bukan table-driven test
  dengan expected variations seperti diwajibkan TDD.md §2.5.
- `TestSkillsList_Shape` hanya satu response case.
- Tidak ada test untuk empty catalog behavior, karena `skillCatalog` selalu non-empty.
- Tidak ada test yang memastikan response tidak berisi `SKILL.md` body.
- Tidak ada test untuk content type exactness, cache policy, atau source health policy.
- Test route session gate dan wrong verb berada di P4 test bersama, bukan test bernama Skill
  untuk auth and method contract.

### Risiko dan aturan

- TDD.md §2.1 dan §2.5 meminta coverage route dan table-driven variation.
- AGENTS.md §2.1 meminta happy path, validation-failure jika input ada, serta auth failure.
- OWASP A01 requires server-side function-level access control, already present but should be
  pinned against Skill explicitly if the endpoint is retained.
- OWASP A08 requires schema/integrity validation for untrusted data, relevant if remote source
  verification is added.

### Rencana DURING

Tambahkan test sebelum production implementation:

1. Table-driven handler tests for catalog rows, including typical capability row, entry row,
   nullable endpoint, URL path derivation, and malformed metadata fixture.
2. Explicit mux test for `GET /api/v1/skills` without session = 401, with session = 200, and
   unsupported method = 405.
3. Validation applicability test/documentation: GET has no request body/query/path schema, so
   no `VALIDATION_ERROR` case is applicable. If future query parameters are introduced, add a
   typed schema using `go-playground/validator/v10` before handler use.
4. Response safety test asserting only metadata fields are served and no document body or
   secret appears in the response.
5. Source availability verification as a separately marked integration check, with explicit
   timeout and no network dependency in deterministic unit tests.

### Kriteria selesai

- Test names map to endpoint behavior and use parameterized cases where inputs vary.
- Auth failure and wrong verb are directly attributable to Skill route.
- Validation applicability is documented, and any future input is schema-validated.
- `go test -race -count=1 ./...` remains green with no skipped tests.

## 7. F4 (MEDIUM): documentation and readiness metadata are stale

### Fakta

`SYSTEM_MAP.md` says it was last updated 2026-09-19 and has no P3/P4/Skill/openapi/changelog
entry. `app-serv/README.md` still says P0/P1 is in progress and lists major P1 surfaces as
not installed, despite the current tree serving those routes and the P4 static routes. The
Skill endpoint itself is a new static catalog data source and should be reflected in topology
and service status. The current `docs/SPEC-API/001-SPEC-API.md` has an uncommitted unrelated
blank line at EOF in the working tree; that change must not be mixed into the Skill plan unless
owner explicitly intends it.

### Risiko dan aturan

- AGENTS.md §1.9 requires SYSTEM_MAP sync when data source, service interactions, or topology
  changes.
- R-17/R-36 prohibit stale or unsupported status and numeric claims.
- A maintainer reading README can incorrectly conclude the endpoint is not implemented.

### Rencana DURING

Update only after F1/F2 contract decisions are settled:

- `SYSTEM_MAP.md`: record P4 static catalog, session-gated route, embedded/static source,
  no DB/Redis/outbound interaction, and current date.
- `app-serv/README.md`: replace stale P0/P1 status with measured current status, list P4
  Skill/OpenAPI/changelog routes, and retain only verifiable claims with commands.
- `docs/SPEC-API/001-SPEC-API.md` and/or `docs/SPEC-UI/001-SPEC-UI.md`: update the selected
  contract in the same change. Do not hand-edit generated OpenAPI independently; regenerate
  or update its source artifact in the same change and rerun route coverage.
- Add a concise verification record with exact commands and date.

### Kriteria selesai

- No contradictory statement remains across source, contract, SYSTEM_MAP, and README.
- Route and source-of-truth changes are reflected in the same change set.
- No unsupported number or release claim is introduced.

## 8. Non-findings and accepted implementation details

- The endpoint has no external input payload, so `go-playground/validator/v10` is not needed
  for the current GET contract. Adding a validator only to satisfy a checklist would be slop.
- No DB or Redis use is a gap. The spec explicitly defines the catalog as embedded static data.
- No SSRF is present in the current route because it does not fetch a caller-supplied URL.
  Adding runtime URL verification would create an outbound dependency and a new SSRF/timeout
  surface, not fix the current contract mismatch.
- No `any` appears in Skill handler application code. `schema.WriteJSON` uses the repository's
  existing serialization boundary, which is typed at all Skill call sites.
- No line-limit violation exists in Skill source or tests.
- No new `SYSTEM_MAP.md` topology change was made during this audit; this document only records
  the existing mismatch. A contract/source decision still requires a sync update.
- The current local backend gates pass except for `staticcheck` evidence because that tool is
  not installed in the environment.

## 9. Recommended order

1. **F1**: make every served source document real, or explicitly make unavailable status part
   of the API.
2. **F2**: choose one catalog contract and update SPEC-API/SPEC-UI together.
3. **F3**: strengthen table-driven endpoint tests and document GET validation applicability.
4. **F4**: synchronize SYSTEM_MAP and app-serv README after the contract decision.

No source code should be modified until the owner selects the finding numbers and, for F1/F2,
the catalog/source option.

## 10. Status per 2026-09-21

Perubahan `app-ui` pada `7b9ac1d` ("build /skills over the served catalog and publish the seven
skill documents") menyentuh temuan dokumen ini dari sisi panel dan dokumen sumber. Kondisi
yang terukur hari ini:

- **F1 sebagian CLOSED.** Tujuh dokumen `skills/<id>/SKILL.md` sudah ada dan ter-commit di tree
  (`skills/pannelai/SKILL.md` dan enam capability row lainnya), jadi sumbernya nyata, bukan
  fiktif. Namun branch lokal berada di depan `origin/main`, dan `raw_url` menunjuk ref `main`,
  sehingga permintaan HTTP ke raw URL masih menjawab **404** sampai branch di-push. Selama itu
  katalog tetap menyajikan tautan yang belum dapat diambil.
- **F2 CLOSED.** SPEC-UI §6.10 diamandemen 2026-09-21: katalog capability di `GET /api/v1/skills`
  (SPEC-API §7.16) ditetapkan sebagai source of truth, dan §14 Q2 ditutup dengan keputusan itu.
  `app-ui/src/lib/navigation.ts` memuat baris Skill dengan `href: '/skills'` dan tanpa flag
  `planned`, sehingga panel menautkannya sebagai layar nyata.
- **F3 masih terbuka.** `internal/handler/skills_test.go` masih berisi dua fungsi test:
  `TestSkillCatalog_Invariants` (satu loop atas katalog) dan `TestSkillsList_Shape` (satu
  response). Belum ada variasi table-driven, kasus empty catalog, atau test yang memastikan body
  `SKILL.md` tidak ikut disajikan.
- **F4 sebagian CLOSED.** `app-serv/README.md` sudah diperbarui: statusnya kini P0 sampai P4
  selesai dan permukaan §7 yang benar-benar terpasang, sehingga klaim lama "P1 sedang berjalan"
  hilang. `SYSTEM_MAP.md` masih belum punya entri khusus untuk rute statis P4
  (`/skills`, `/openapi.json`, `/changelog`), sehingga bagian topology dari F4 tetap terbuka.

Verifikasi yang mendasari catatan ini: `ls skills/` (tujuh direktori), `git ls-files skills`
(tujuh berkas ter-track), permintaan HTTP ke
`https://raw.githubusercontent.com/rusmanadodi2598/pannelAI/refs/heads/main/skills/pannelai/SKILL.md`
(**404**), `grep -n "Amended 2026-09-21" docs/SPEC-UI/001-SPEC-UI.md` (amendemen §6.10), dan
`wc -l app-serv/internal/handler/skills_test.go` (88 baris, dua fungsi test).
