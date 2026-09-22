# 007-UI-ENDPOINT-READINESS.md: Audit Kesiapan Route Endpoint Panel (app-ui)

Dokumen kerja hasil pemeriksaan kesiapan route dan endpoint pada panel `app-ui` tanggal
2026-09-20. Bukan kontrak; kontrak tetap `docs/SPEC-UI/001-SPEC-UI.md` (perilaku, route,
scope) dan `docs/SPEC-API/001-SPEC-API.md` (wire). Dokumen ini melanjutkan pola
`003-ENDPOINT-READINESS.md` sampai `006-TOKEN-SAVER-READINESS.md`: temuan bernomor F,
owner memilih nomor yang dikerjakan.

|             |                                                                                                                                                                                                                                                                                                                                                                                             |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**  | F1 **CLOSED 2026-09-21** (layar `/api-docs` dibangun dari dokumen yang dilayani, §14 Q3 ditutup), F2 **CLOSED 2026-09-21** (layar `/skills` plus tujuh dokumen skill), F3 **CLOSED 2026-09-21** (layar `/playground` plus jalur injeksi kredensialnya), F4 **CLOSED 2026-09-21** (layar `/changelog` membaca rilis yang dilayani, §14 Q11 ditutup), F5 **CLOSED 2026-09-21** (filter tab Upstream endpoints pindah ke URL dan memfilter di server), F9 **CLOSED 2026-09-21** (`rate_limited_until` tampil sebagai countdown), F6 **CLOSED 2026-09-21** (mode baris berulang terpasang di drawer), F7 **CLOSED 2026-09-21** (empat layar tanpa test render sekarang punya, plus tiga cacat wire yang ikut tertutup), F8 **CLOSED 2026-09-21** (satu kontrol refresh bersama di sebelas layar daftar, D4 dijawab owner, §8.6.2 tetap), F10 **CLOSED 2026-09-21** (guard §8.4.4 di enam form berdraft, §8.4.5 diamandemen dengan alasan tertulis), F11 **CLOSED 2026-09-21** (angka dan klaim dokumen diukur ulang dengan perintahnya), F13 **OPEN 2026-09-22** (klaim versi Bun di sembilan belas baris README tidak dapat direproduksi di mesin ini; dua baris pass terbaru sudah memakai versi terukur). Sisa nomor tinggal F12 dan F13 |
| **Dibuat**  | 2026-09-20, dari `app-ui/src/routes/`, `app-ui/src/lib/`, `app-ui/tests/`, `app-ui/README.md`, `docs/SPEC-UI/001-SPEC-UI.md` §2.1/§5.1/§6/§8/§12/§14/§15, dan route P4 `app-serv`                                                                                                                                                                                                           |
| **Kaitan**  | SPEC-UI §2.1 (KEEP), §5.1 (route table), §6.1 sampai §6.16, §8.4/§8.6, §9.4, §10.2, §12, §14; SPEC-API §7.1 sampai §7.18, §10; `docs/RULLES/TDD.md`; `DESIGN.md`                                                                                                                                                                                                                            |
| **Lingkup** | hanya `app-ui/`. `app-serv/` dibaca sebagai sumber wire dan **tidak boleh diubah** dari draft ini: setiap kebutuhan yang jatuh di sana dicatat sebagai permintaan atau pertanyaan (F1, F2, F4), bukan dikerjakan                                                                                                                                                                            |

## 1. Metode

Empat langkah, semuanya bisa diulang:

1. Route table SPEC-UI §5.1 (19 baris) dibandingkan dengan `find app-ui/src/routes -type f`:
   setiap route yang ada dipetakan, setiap route yang tidak ada dicatat.
2. Kesiapan per layar diperiksa terhadap bullet §6.x-nya sendiri, bukan terhadap ringkasan
   README: tiap kontrol yang diminta spec dicari di komponen, dan tiap endpoint yang
   dipanggil dicocokkan dengan tabel §7 SPEC-API.
3. Sisi `app-serv` diperiksa hanya untuk menjawab "apakah endpoint-nya sudah ada": route P4
   (`/skills`, `/openapi.json`, `/changelog`) dibaca dari router, handler, dan dokumen yang
   dilayani.
4. Bukti mekanis dijalankan langsung: `wc -l` atas seluruh `src/` dan `tests/`, inventaris
   test per layar (`grep -rlF` atas import `+page.svelte`), dan pemetaan filter URL
   (`page.url.searchParams`).

## 2. Ringkasan kesiapan 14 item

| #   | Item (KEEP)            | Route                                    | Layar              | Endpoint SPEC-API                        | Status                                         |
| --- | ---------------------- | ---------------------------------------- | ------------------ | ---------------------------------------- | ---------------------------------------------- |
| 1   | Endpoint & Key         | `/endpoint-keys`                         | ada                | §7.3, §7.5 live                          | F5 **CLOSED**, F6 **CLOSED**, F7 **CLOSED**, F8 **CLOSED**, F9 **CLOSED** |
| 2   | Provider               | `/providers`, `/providers/[provider_id]` | ada                | §7.4, §7.6 live                          | F5 (list) **CLOSED**, F7 (list) **CLOSED**, F8 (list) **CLOSED**, Q12/Q13/Q19-Q23 tercatat |
| 3   | Combo & Vision Adapter | `/combos`                                | ada                | §7.7, §7.8 live                          | F8 **CLOSED**; F12 (bukti); test tingkat tab ada |
| 4   | Usage                  | `/usage`                                 | ada                | §7.12 live                               | F8 **CLOSED**; F12; tautan API Docs menunggu F1 |
| 5   | Quota Tracker          | `/quota`                                 | ada                | §7.12 live                               | bersih; live pass tercatat                     |
| 6   | Token Saver            | `/token-saver`                           | ada                | §7.9 live                                | bersih; live pass tercatat                     |
| 7   | Skill                  | `/skills`                                | ada                | §7.16 live sejak P4                      | F2 **CLOSED 2026-09-21**                       |
| 8   | Media Provider         | `/media-providers/[kind]`                | ada                | §7.10 live                               | F8 **CLOSED**; Q17/Q18 tercatat                |
| 9   | Playground Chat        | `/playground`                            | ada                | §7.15 live; §10 P4 menugaskan halamannya | F3 **CLOSED 2026-09-21**                       |
| 10  | Proxy Pools            | `/proxy-pools`                           | ada                | §7.11 live                               | F8 **CLOSED**; bersih; live pass tercatat      |
| 11  | API Docs               | `/api-docs`                              | ada                | §7.17 live sejak P4                      | F1 **CLOSED 2026-09-21**                       |
| 12  | Changelog              | `/changelog`                             | ada, membaca §7.18 | §7.18 live sejak P4                      | F4 **CLOSED 2026-09-21**, F8 **CLOSED**       |
| 13  | Console Log            | `/console-log`                           | ada                | §7.13 live                               | F8 **CLOSED 2026-09-21**                       |
| 14  | Setting                | `/settings`                              | ada                | §7.14 live                               | F10 **CLOSED 2026-09-21**                      |

Tidak ada lagi route yang tidak ada. Baris `playground` (`src/lib/navigation.ts:108`) ditutup F3 pada
2026-09-21 dan sekarang membawa `href: '/playground'`, dan baris `skills` (baris 100) ditutup F2 pada
tanggal yang sama. Satu-satunya flag `planned: true` yang tersisa ada di container Media Provider, dan
renderer memperlakukannya sebagai disclosure, bukan chip: jumlah baris yang dirender `Planned` adalah nol.

## 3. Bukti kesiapan yang sudah lolos

- 14 dari 14 layar terbangun dan routable; 19 baris sidebar routable (13 route statis plus 6
  kind media), 0 `Planned`, 1 container, 20 node dalam 5 grup (diukur ulang 2026-09-21 lewat
  `bun -e` atas `NAV_GROUPS` dan `allNodes`; angka draft 2026-09-20 adalah 11 dari 14, 16 routable,
  3 planned).
- Sembilan capability U2 (SPEC-UI §12) landed; dua dari empat exit criteria-nya sudah
  diverifikasi live (proxy tested from the panel; token saver config saved and reflected).
- Line limit terukur: **tidak ada satu pun file di `app-ui/src/` yang melewati 220 baris**
  (`find src -name '*.svelte' -o -name '*.ts' | xargs wc -l`). Yang di atas 250 hanya file
  test pra-eksisting (terbesar `tests/schemas/combo.test.ts` 582), konsisten dengan aturan
  "file test pra-eksisting dibiarkan".
- §8.4.2 (filter di URL) sudah dipatuhi Usage (dua tab), Logs, dan tab Upstream endpoints
  (`parseXSearch` + `nextXSearch` + `goto(resolve(...))` di `UsageRecordsTab`, `UsageOverviewTab`,
  `LogsRequestsTab`, `UpstreamEndpointsTab`).
- §8.6.2 (kontrol refresh eksplisit di setiap layar daftar) dipenuhi sejak F8: satu komponen
  bersama `RefreshControl` dipasang di sebelas layar daftar, `/quota` tetap memakai kontrolnya
  sendiri yang sudah teruji, dan test per layar membuktikan kontrolnya mengulang bacaan yang
  sedang tampil, bukan bacaan default.
- §6.6 (badge sumber, countdown, polling dengan pause) dan §6.7/§6.9/§6.8/§6.4 sudah sesuai
  bullet spec-nya, termasuk pengecualian yang dinyatakan (Q15 sampai Q18).
- Tiga endpoint P4 sudah live dan session-gated di `app-serv`: `GET /api/v1/skills` (katalog
  embedded, 7 baris), `GET /api/v1/openapi.json` (67 path, 23 tag, 2 security scheme),
  `GET /api/v1/changelog` (5 rilis, terbaru dulu).

## 4. F1 (HIGH): `/api-docs` belum dibangun padahal artefak mesin-bacanya sudah live

**Fakta.** SPEC-UI §5.1 menempatkan `/api-docs` di fase U1, dan §12 U1 menyebutnya di scope.
Tidak ada file route, dan `src/lib/navigation.ts:109` masih `planned: true`. Sejak P4,
`GET /api/v1/openapi.json` hidup (`app-serv/internal/router/router.go:205`, dokumen
`app-serv/internal/handler/openapi.json`): 67 path dengan `summary`, `tags`, dan `security`
per operasi, plus `components.securitySchemes` (`sessionCookie`, `gatewayKey`). Yang **tidak**
ada di dokumen: `components.schemas` (nol schema request/response), tabel error §8, dan
penanda fase per grup. §14 Q3 ("Machine-readable contract") karena itu sudah terjawab:
artefaknya ada, tinggal dikonsumsi.

**Risiko.** Satu-satunya item KEEP fase U1 yang belum punya layar, padahal endpoint-nya sudah
siap; §6.12 melarang salinan kontrak kedua yang ditulis tangan, jadi menunda berarti menunda
sampai ada yang menyalin tabel spec ke komponen. Empty state §6.5 ("plus a link to API Docs")
juga tidak bisa ditepati selama route-nya tidak ada.

**Kerja yang diusulkan.** Bangun `/api-docs` dari dokumen yang dilayani: satu modul
`src/lib/api/docs.ts` plus schema Zod untuk dokumennya, lalu render base URL, dua scheme
autentikasi dan operasi mana yang memakai yang mana, katalog path dikelompokkan per tag, satu
contoh curl per grup yang **disusun dari** `method` + `path` (bukan salinan), dan keadaan
kosong/loading/error. Dua celah isi diputuskan owner: (a) tabel error §8, dan (b) fase per
grup. Pilihan yang diusulkan: render hanya yang dokumen bawa, dan ganti tabel error dengan
satu kalimat yang menunjuk `docs/SPEC-API/001-SPEC-API.md` §8 (bukan salinan yang bisa
melenceng). Bila owner ingin tabel error dan fase tampil di layar, itu permintaan `app-serv`
(`x-error-codes`, `x-phase`), dicatat di sini dan tidak dikerjakan dari draft ini.

**Kriteria selesai.** Route file ada, baris nav routable (R-24), tiga state ada, tidak ada
salinan kontrak kedua, test schema + render hijau, click-through tercatat. §14 Q3 ditutup
dengan jawabannya di spec.

**Status: CLOSED 2026-09-21.** Layar dibangun dari dokumen yang dilayani, tanpa perubahan
`app-serv`:

- `src/routes/api-docs/+page.svelte` ada, dan `src/lib/navigation.ts:109` sekarang
  `href: '/api-docs'` (R-24). Sidebar terukur: 17 routable, 2 planned (`skills`, `playground`),
  1 container, 20 node.
- `src/lib/schemas/openapi.ts` mem-parse dokumen yang dilayani: field yang dibaca ketat, tambahan
  ditoleransi (§7.4.2). Empat modul murni menurunkan yang dirender: `openapi-catalog.ts` (urutan
  operasi dan grup per tag), `openapi-credentials.ts` (label scheme, penempatan, header, contoh
  curl yang disusun, hitungan pemakaian), dan `openapi-errors.ts` (satu tabel per plane, makna
  dari respons yang membawa envelope plane itu).
- Tidak ada salinan kontrak kedua (§6.12): layar tidak memuat satu pun path endpoint, kode error,
  atau nama scheme yang tidak datang dari dokumen. Blok yang tidak ada dinyatakan tidak ada.
- Tiga state ada (loading, error dengan Try again, empty untuk dokumen tanpa path), diuji di
  `tests/components/api-docs.test.ts`.
- Test: 6 file, 92 test, hijau di suite penuh (1868 test / 79 file). Rincian: `openapi.test.ts`
  15, `openapi-catalog.test.ts` 16, `openapi-credentials.test.ts` 23, `openapi-example.test.ts`
  16, `openapi-errors.test.ts` 8, `api-docs.test.ts` 14.
- Live pass 2026-09-21: 31 check, 0 gagal, di atas `app-serv` yang boot dari `.env`; dokumen live
  (67 path, 93 operasi, 23 tag, 161 schema, 18 respons) diparse lewat schema panel sendiri dan
  setiap derivasi dijalankan di atasnya. Dua bukti negatif ikut: schema menolak dokumen tanpa
  `paths` dan dokumen dengan `openapi` berupa angka. Baseline DB tidak berubah.
- §14 Q3 ditutup di SPEC-UI dengan jawabannya; §6.12 diamandemen (23 tag, bukan 15 grup; tabel
  error dari `x-contract.planes`; penanda fase digantikan pin router), §15 dan §16 R-24/R-38
  diperbarui, dan changelog serta README mencatat pass ini.
- Premis draft yang berubah: dokumen di working tree sudah membawa `components.schemas` (161),
  `components.responses` (18), `x-error-codes`, dan `x-contract.planes`, jadi usulan D3 "cukup
  yang ada di dokumen" terpenuhi tanpa permintaan `app-serv`. Penanda fase tetap tidak ada dan
  tidak diperlukan, karena `TestOpenAPICoversEveryRegisteredRoute` mem-pin dokumen ke router dua
  arah: grup di layar adalah route yang menjawab.
- Click-through: separuh wire tercatat di README (31 check). Click-through browser belum
  dilakukan dan README menyatakannya masih outstanding; panel client-rendered (`ssr = false`),
  jadi separuh render ditutup test jsdom per elemen (R-35).
- Permintaan `app-serv` (dicatat, tidak dikerjakan): blok `servers` dokumen dipatok generator ke
  `http://localhost:8080` dengan deskripsi "Local app-serv", jadi instance di alamat lain
  melayani base URL yang bukan alamatnya. Panel merender nilai dokumen apa adanya beserta
  deskripsinya; membuat blok itu mengikuti instance berjalan (atau menyatakan alamat default)
  adalah perubahan sisi layanan.

## 5. F2 (HIGH): `/skills` belum dibangun, dan dokumen skill-nya belum ada di repo

**Fakta.** `GET /api/v1/skills` hidup sejak P4
(`app-serv/internal/handler/skills.go`), mengembalikan `{data: [...]}` dengan 7 baris: satu
entry skill (`pannelai`) plus satu per endpoint data plane (`chat`, `image`, `tts`, `stt`,
`embeddings`, `web-search`). Tiap baris membawa `raw_url` dan `blob_url` yang dirakit dari
konstanta `rusmanadodi2598/pannelAI`, ref `main`, path `skills/<id>/SKILL.md`
(`skills.go:31-35`, `skills.go:118-119`). Dua kenyataan yang belum dijawab:

- **Direktori `skills/` tidak ada di repo ini.** `ls skills` gagal; jadi setiap `raw_url` yang
  dilayani hari ini menunjuk berkas 404, dan cabang yang ditunjuk (`origin/main`) juga
  tertinggal 104 commit dari HEAD lokal. §6.10 melarang mengirim kontrol copy yang menyalin
  tautan rusak, jadi layarnya tidak bisa dibangun tanpa ini.
- **§6.10 dan §7.16 menggambarkan katalog yang berbeda.** §6.10 meminta dua entri
  (`/antislop` AI, SuperPowers) dan menegaskan "no directory of extras", sedangkan §7.16
  melayani katalog per capability. §14 Q2 ("Skills source") sekarang terjawab sebagian:
  sumbernya sudah ditetapkan di kode, yang belum ada isinya.

**Risiko.** Layar yang dibangun dari §6.10 apa adanya akan menampilkan dua entri yang tidak
pernah dikirim API, dan layar yang dibangun dari API akan menampilkan tujuh entri yang tidak
diminta spec. Keduanya adalah pelanggaran R-38 (konten yang tidak nyata) atau R-26 (kontrol
mati).

**Kerja yang diusulkan.** (1) Owner memutuskan katalog mana yang benar, dan menulis berkas
`skills/<id>/SKILL.md` untuk tiap baris yang dilayani (7 berkas) atau mengubah katalog di
handler. (2) Amandemen §6.10 supaya cocok dengan katalog yang dilayani, termasuk aturan "entry
skill first" dan perbedaan `raw_url` (untuk agen) versus `blob_url` (untuk manusia). (3)
Bangun `/skills`: satu modul `src/lib/api/skills.ts` + schema, satu kartu per baris dengan
nama, deskripsi, endpoint, dua tautan, dan satu kontrol copy berisi baris instalasi. (4) Bila
sebuah `raw_url` tidak resolve, baris itu merender unavailable state yang menamai penyebabnya,
bukan kontrol copy.

**Kriteria selesai.** Tiap entri yang tampil memiliki `SKILL.md` yang benar-benar bisa dibaca
(dibuktikan dengan permintaan nyata ke `raw_url`, bukan diasumsikan), atau merender unavailable
state; tidak ada entri dengan tautan rusak; §6.10 dan §7.16 sepakat; §14 Q2 ditutup.

**Status: CLOSED 2026-09-21.** D1 dijawab owner 2026-09-21 (ikuti katalog yang dilayani, tulis tujuh
dokumen, probe saat load plus `Check again`, commit lokal saja tanpa push), lalu dikerjakan:

- Tujuh dokumen `skills/<id>/SKILL.md` ada, satu per baris yang dilayani: `pannelai` (entry) plus enam
  capability. Tiap berkas membawa frontmatter (`name`, `description`) dan heading; isinya di-groundkan
  pada §7.15 dengan membaca sumber Go-nya (`skills.go`, `media_call.go`, `media_shape.go`,
  `media_audio.go`, `media_search.go`, `embeddings_resolve.go`, `engine.go`, `engine_failure.go`,
  `catalog.go`, `chat_response.go`), bukan dari ingatan. Empat klaim draf pertama salah dan diperbaiki
  sebelum berkas ditulis: health menjawab `{"status":"ok",…}` bukan `{"ok":true}`; envelope error data
  plane membawa `message`/`type`/`code` tanpa `request_id`; combo yang gagal seluruhnya mengembalikan
  error anggota terakhir, bukan `NO_PROVIDER_AVAILABLE`; model media tanpa prefix provider menjawab
  `MODEL_NOT_FOUND`, dan provider media tanpa kind itu menjawab `PROVIDER_NOT_ROUTABLE`.
- Katalog yang dipakai adalah yang dilayani (§7.16). §6.10 diamandemen: satu baris per capability, entry
  skill first, `raw_url` untuk agen dan `blob_url` untuk manusia, availability ditanya bukan diasumsikan,
  plus catatan amandemen bahwa dua entri lama tidak pernah dilayani route mana pun. §5.1 baris `/skills`
  ikut diperbaiki supaya tidak bertentangan dengan §6.10, dan §14 Q2 ditutup.
- `src/routes/skills/+page.svelte` ada, dan `src/lib/navigation.ts:100` sekarang `href: '/skills'`
  (R-24). Sidebar terukur 2026-09-21, sebelum F3: 20 node dalam 5 grup, 18 leaf routable (12 route statis
  plus 6 kind media), 1 container, 1 baris `Planned` (`Playground Chat`); F3 memindahkan baris itu menjadi
  leaf, jadi angka setelahnya 19 leaf dan 0 `Planned`.
- Modul panel: `src/lib/schemas/skill.ts` (katalog; field yang dibaca ketat, tambahan ditoleransi §7.4),
  `src/lib/schemas/skill-source.ts` (empat sebab, `installLine`, `orderSkills`, dua classifier),
  `src/lib/api/skills.ts` (satu GET lewat `apiRequest`, satu HEAD publik tanpa kredensial dan tanpa
  prefix panel), `src/lib/strings/skills.ts`, `SkillCard.svelte`, `SkillEntryBlock.svelte`. Tiga state
  ada (loading, error dengan `Try again`, empty untuk katalog tanpa baris).
- Kontrol copy hanya muncul untuk baris yang benar-benar resolve, yang persis aturan §6.10.
  `CopyButton.svelte` sekarang menerima label sebagai prop, jadi kontrol yang sama dipakai header, API
  Docs, dan Skills tanpa modul copy kedua.
- Test: 3 file, 30 test, hijau di suite penuh (1898 test / 82 file, sebelumnya 1868 / 79). Rincian:
  `skill.test.ts` 6, `skill-source.test.ts` 11, `skills.test.ts` 13.
- Live pass 2026-09-21: 26 check, 0 gagal, di atas `app-serv` yang boot dari `.env` dengan run env yang
  hanya menambah `HTTP_ADDR`, `PUBLIC_BASE_URL`, `EGRESS_ALLOWED_TARGETS`, dan
  `PANEL_BOOTSTRAP_PASSWORD`. Katalog live (7 baris) dibaca lewat `fetchSkillCatalog` panel sendiri dan
  diparse lewat `schemaSkillCatalog`; `orderSkills`, `installLine`, `probeSkillSources`, `causeSentence`,
  dan `SKILLS_COPY.sources.summary` dijalankan di atas hasilnya; tujuh `raw_url` ditanya dengan permintaan
  HEAD nyata. Tiap baris yang dilayani juga dicocokkan ke berkas `skills/<id>/SKILL.md` di tree ini.
- Dua kontrol negatif membuat pass tidak hampa: berkas yang ada di ref yang sama (`README.md`) membaca
  `available` dan path yang tidak ada membaca `missing`, jadi jawabannya tentang alamat itu, bukan tentang
  pemeriksanya; dan `schemaSkillCatalog` menolak baris dengan `raw_url` relatif serta payload tanpa `data`.
- Kejujuran hasil live: ketujuh sumber menjawab 404 hari ini, karena `origin/main` tertinggal 106 commit
  dari HEAD lokal dan 0 di depan, dan `git ls-tree origin/main skills` kosong. Jadi layar merender tujuh
  unavailable state, nol kontrol copy, dan kalimat "0 of 7 sources are published at the ref the catalog
  names." Itu jawaban yang benar untuk keadaan sekarang, bukan cacat layar: begitu owner push, kontrol copy
  muncul tanpa perubahan panel. Dua batas dicatat di SPEC-UI: panel client-rendered (separuh render hanya
  dari test jsdom) dan dokumen belum ada di ref yang dinamai katalog.
- Satu cacat copy ditemukan oleh pass ini dan diperbaiki sebelum commit, dan polanya dicatat: kalimat
  ringkasannya dulu berbunyi "N of M sources answered" dengan N = jumlah baris `available`, sehingga keadaan
  hari ini terbaca "0 of 7 sources answered" sementara tujuh baris di bawahnya berkata host menjawab 404.
  Kalimat itu menyatakan pemeriksanya gagal padahal pemeriksanya berhasil, jadi bunyinya diubah menjadi
  "N of M sources are published at the ref the catalog names" dan variabelnya dinamai `published`. Angkanya
  sekarang sama dengan jumlah baris yang merender "Source available", yang bisa dihitung pembaca.
- README `app-ui` mencatat pass ini (26 check, 0 gagal) dan menyatakan click-through browser masih
  outstanding; DESIGN.md §11 mencatat empat keputusan layar ini (katalog yang dilayani, probe sebelum
  menawarkan copy, sebab bukan "unavailable" kosong, dua alamat untuk dua pembaca).
- Temuan di luar lingkup F2, dicatat bukan dikerjakan: §5.1 baris `/api-docs` masih menyebut "rendered
  from SPEC-API §7" padahal layarnya merender dokumen yang dilayani; itu sisa F1 dan masuk daftar F11.

## 6. F3 (HIGH): `/playground` belum dibangun, dan jalur injeksi kredensialnya belum ada

**Fakta.** SPEC-API §10 baris P4 menugaskan "panel playground (SPEC-UI page over §7.15, no new
route)", jadi fase yang ditunggu §14 Q7 sudah diberikan. Model auth-nya sudah diputuskan owner
(2026-09-17, §6.15): server panel yang melakukan panggilan data plane dan menyuntikkan gateway
key, browser tidak pernah memegang kredensial. Yang belum ada di panel:

- `src/lib/schemas/env.ts:9` hanya mendeklarasikan `PANEL_API_TARGET`; tidak ada
  `PANEL_PLAYGROUND_KEY`.
- `src/hooks.server.ts:11-15` meneruskan **semua** `/api/v1/*` apa adanya ke `app-serv` tanpa
  menyuntikkan header. Jadi panggilan browser ke `/api/v1/chat/completions` sampai ke gateway
  tanpa kredensial (401), dan sekaligus: membuat hook ini menyuntikkan key untuk semua path
  akan menyerahkan kredensial ke setiap permintaan yang bisa dibuat browser.
- Tidak ada file route dan baris nav masih `planned: true` (`navigation.ts:108`).

**Risiko.** Layar tidak bisa dibangun hanya dengan menambah halaman: jalur injeksinya adalah
pekerjaan panel, dan bentuk yang salah (injeksi di forwarder umum) justru memperluas
permukaan kredensial. Tanpa itu, satu-satunya cara memeriksa rute adalah CLI, dan halaman
yang §10 P4 tugaskan tetap kosong.

**Kerja yang diusulkan.** (1) Tambah `PANEL_PLAYGROUND_KEY` ke `envSchema` dengan aturan gagal
cepat yang sudah jadi kebiasaan panel (nama variabel disebut saat hilang atau salah bentuk).
(2) Tambah endpoint server khusus di luar prefix `/api/v1` (route `+server.ts`) yang melakukan
panggilan data plane dari server, hanya menambahkan `Authorization: Bearer` di sana, tidak
pernah mencatat, tidak pernah menggema, dan tidak pernah mengirim key ke browser. (3) Bangun
`/playground`: pilihan model, kotak pesan, tombol kirim yang menyatakan biayanya sebelum
kirim, lalu model yang ter-resolve dan status upstream pada hasilnya. (4) Bila key tidak
diset, layar merender unavailable state yang menyebut variabelnya, bukan meminta operator
menempel key.

**Kriteria selesai.** Tidak ada kredensial di bundle, storage, URL, atau field form (diuji);
key tidak muncul di respons mana pun (diuji); unavailable state saat key kosong; click-through
tercatat. Owner menyediakan satu gateway key untuk pass live.

**Status: CLOSED 2026-09-21.** Owner menjawab scope layarnya pada 2026-09-21 (streaming ikut, dan hasilnya
teks + fakta + JSON mentah), lalu dikerjakan:

- Jalur injeksinya ada, dan bentuknya bukan forwarder umum: `src/lib/schemas/env.ts` mendeklarasikan
  `PANEL_PLAYGROUND_KEY` (kosong atau tidak diset adalah keadaan yang didukung, bukan gagal boot), dan dua
  route panel di luar prefix `/api/v1` (`GET /playground/models`, `POST /playground/chat`) melakukan
  panggilan data plane sendiri. `src/lib/server/playground.ts:61` (`playgroundHeaders`) adalah satu-satunya
  tempat kredensial ditulis, jadi call site kedua tidak bisa menemukan penempatan kedua.
  `src/hooks.server.ts` tidak disentuh: `/api/v1/*` tetap diteruskan apa adanya, sehingga tidak ada
  permintaan browser mana pun yang ikut membawa kredensial.
- Layar `src/routes/playground/+page.svelte` ada dan `src/lib/navigation.ts` sekarang
  `href: '/playground'` (R-24). Sidebar terukur 2026-09-21: 19 leaf routable, 0 baris `Planned`.
- Modul panel: `src/lib/schemas/playground.ts` (kontrak panel, ketat) dan `playground-stream.ts` (frame
  reader), `src/lib/api/playground.ts` plus `playground-reader.ts` (klien; dipisah karena berkas pertama
  menembus ambang 200 baris), `src/lib/strings/playground.ts`, `PlaygroundComposer.svelte`, dan
  `PlaygroundAnswer.svelte`. Tiga state ada (loading, gagal dengan `Try again`, dan daftar model kosong),
  plus unavailable state yang memakai kalimat route panel sendiri dan tidak merender kontrol kirim.
- Empat aturan §6.15 dipegang test, bukan review: nama variabel hanya boleh muncul di schema yang
  mendeklarasikannya dan di `src/lib/server/`; kredensial ditulis tepat di satu fungsi; berkas browser
  dilarang menyebut store atau header kredensial; dan kalimat gateway yang dikutip ulang di-redact.
  Bundle produksi di-grep: `PANEL_PLAYGROUND_KEY` tidak ada di `build/client/`, hanya di `build/server/`.
- Envelope kegagalan milik panel sendiri (`PLAYGROUND_KEY_MISSING`, `UNAUTHORIZED`, `VALIDATION_ERROR`,
  `GATEWAY_UNREACHABLE`, `GATEWAY_KEY_REFUSED`, `GATEWAY_ERROR`, `INTERNAL_ERROR`), karena kedua plane
  memakai nama kode `UNAUTHORIZED` dan 401 yang diteruskan akan mengeluarkan operator dari sesi panel.
  Kode gateway tetap ikut di dalam envelope panel.
- Test: 133 test baru di 12 berkas (semuanya hijau), plus satu test tambahan di `tests/api/client.test.ts`
  untuk jalur envelope yang dipakai klien playground. Suite penuh di tree beku: 2032 test / 94 berkas,
  naik dari 1898 / 82 yang dicatat pass F2.
- Live pass 2026-09-21, empat fase, 37 check, 0 gagal, di atas `app-serv` yang dibangun dari **HEAD
  ter-commit** (working tree-nya sedang dipegang aktor lain, jadi `git archive HEAD` dipakai dan tidak ada
  berkas `app-serv` yang disentuh): fixture (5 check), tanpa key (5 check: 503 yang menyebut variabelnya,
  dan gateway maupun stub tidak didial sama sekali), dengan key (23 check: gerbang sesi, baca model,
  jawaban stream beserta faktanya, penolakan body kosong, tidak ada respons yang membawa salah satu
  kredensial, dan log stub yang membuktikan endpoint key plus `stream_options.include_usage`), dan key
  salah (4 check: `GATEWAY_KEY_REFUSED` dengan `UNAUTHORIZED` milik gateway di dalamnya, sesi panel tidak
  tersentuh).
- Fase kelima mengisolasi relay panel: panel kedua diarahkan ke stub yang menulis frame demi frame, karena
  buffering gateway akan menyembunyikan perilaku relay. Jawabannya datang dalam lima read terpisah 600 ms
  dan reader berakhir `done`, jadi klaim "panel men-streaming" punya bukti live yang tidak bergantung pada
  app-serv.
- Tiga cacat `app-serv` ditemukan pass ini dan **dicatat sebagai permintaan ke aktor itu, bukan dikerjakan
  dari sini**, semuanya diverifikasi terhadap `app-serv` di `7b9ac1d` (HEAD saat pass, dibangun lewat
  `git archive`): (1) `mustFrame` di `internal/dataplane/translate_stream_openai.go` mem-marshal payload
  tanpa membingkainya, sehingga stream membawa objek JSON telanjang dan `data: [DONE]` menempel padanya
  tanpa baris kosong; (2) `internal/handler/chat.go` menulis status 200 dan content type SSE sebelum model
  di-resolve, sehingga permintaan stream dengan model tak dikenal menjawab 200 berisi body error di bawah
  tipe SSE, sementara jalur non-stream menjawab 400 dengan benar (kontrol langsung ke gateway membuktikan
  itu); (3) gateway menulis seluruh jawaban sekaligus, jadi `sseSink.Flush()` tidak pernah sampai ke klien
  dan tidak ada yang benar-benar streaming. Panel sengaja tidak diajari mentoleransi tail yang rusak: ia
  melaporkan `truncated`, yang memang bacaan jujur atas byte yang diterimanya. Catatan status per
  2026-09-21: working tree `app-serv` sudah mengubah `internal/handler/datplane_errors.go` dan
  `internal/handler/chat.go`, dan diff-nya memindahkan commit status ke frame pertama, yaitu persis cacat
  (2); temuan (1) dan (3) belum terlihat tersentuh.
- Database live dikembalikan ke baseline persis (`usage=2 logs=1 keys=0 endpoints=0 upkeys=0 nodes=0
caps=0 settings=1 auth_null=true media_settings=0 proxies=0`); baris `usage_records`/`request_logs` pass
  dihapus, gateway key di-hard-delete (revoke saja menyisakan baris), hash `panel_auth` di-null setelah
  server berhenti, dan berkas temp pass dihapus.
- Batas yang dicatat, bukan disembunyikan: click-through browser masih outstanding karena panel
  client-rendered (separuh render hanya dari test jsdom), dan D6 tidak terpakai sebagai permintaan key ke
  owner: pass ini mencetak gateway key-nya sendiri lewat `POST /api/v1/gateway-keys` (plaintext hanya di
  respons create), jadi owner tidak perlu menyerahkan key. README `app-ui` mencatat pass ini dan DESIGN.md
  §11 mencatat dua belas keputusan layar ini.

## 7. F4 (HIGH): Changelog belum membaca sumber rilis yang sekarang sudah dilayani

**Fakta.** `GET /api/v1/changelog` hidup sejak P4
(`app-serv/internal/handler/changelog.go`), menjawab `{data: [{version, date, title, notes}]}`
terbaru dulu, 5 rilis (`v0.4.0` sampai `v0.0.1`). Layar `/changelog` masih membaca hanya
`GET /api/v1/version` dan menyimpan `entries` kosong
(`src/routes/changelog/+page.svelte:27,38`), dengan komentar yang menyatakan "SPEC-API §7
defines no changelog endpoint". Bentuk wire-nya juga belum cocok: `schemaChangelog` menuntut
`{entries: [{version, released_at, category, items[]}]}` (`src/lib/schemas/changelog.ts:22-47`)
sementara route mengirim `date`, `title`, `notes` datar, tanpa `category` dan tanpa `items`.
Dua cacat kecil ikut tercatat: komentar route menyebut §6.18 (section yang benar §6.16), dan
`app-ui/README.md` item 3 masih menyatakan tidak ada sumber rilis.

**Risiko.** Layar menyatakan "no release notes are available" sementara gateway menyajikannya;
itu klaim yang salah (R-38). Memetakan paksa `notes` menjadi `items[]` atau mengarang
`category` akan mengubah teks rilis menjadi sesuatu yang tidak ditulis siapa pun.

**Kerja yang diusulkan.** Owner memilih satu: (a) panel merender bentuk yang dilayani (version,
date, title, notes sebagai satu blok) dan §6.16 diamandemen dari `category` + `items[]` ke
bentuk itu, atau (b) `app-serv` memperluas entri dengan `category` dan `items[]`, yang dicatat
sebagai permintaan dan tidak dikerjakan dari draft ini. Setelah itu: `src/lib/api/changelog.ts`

- schema untuk bentuk yang dipilih, loader membaca dua route (changelog + version), marker
  `Running`/`Newer`/`Installed` tetap dihitung terhadap versi yang berjalan, empty state hanya
  muncul saat route menjawab nol baris, komentar §6.18 diperbaiki, dan Q11 ditutup.

**Kriteria selesai.** Layar merender rilis yang dilayani; perbandingan versi tetap benar; empty
state tidak lagi berbohong; test schema + render hijau; §14 Q11, §12, dan README diperbarui.

**Status: CLOSED 2026-09-21.** Owner memilih opsi (a) pada 2026-09-21 (render yang dilayani, §6.16
diamandemen), lalu seluruhnya dikerjakan di `app-ui` tanpa satu pun perubahan `app-serv`:

- Sumbernya dibaca: `src/lib/api/changelog.ts` (`CHANGELOG_PATH = '/changelog'`, `fetchChangelog` lewat
  `apiRequest`), dan `schemaChangelog` ditulis ulang ke bentuk yang dilayani,
  `{data: [{version, date, title, notes}]}`, ketat pada field wajib dan toleran pada tambahan (§7.4).
  Bentuk lama (`{entries: [{version, released_at, category, items[]}]}`) dihapus, bukan dipetakan:
  memetakan `notes` menjadi `items[]` dan mengarang `category` berarti menuliskan teks yang tidak ditulis
  siapa pun.
- `date` adalah tanggal kalender, bukan timestamp: kontraknya `format: date`, dan `Date.parse` saja bukan
  pemeriksaan itu (ia menerima `2026-9-2` dan menggulung `2026-02-30` ke Maret). Schema melakukan round trip
  UTC, dan kedua kasus itu punya test. Tanggal dirender apa adanya seperti gateway mengirimnya; formatter
  timestamp berzona akan mencetak hari yang tidak pernah dilaporkan gateway untuk pembaca di barat UTC.
- Layar `src/routes/changelog/+page.svelte` membaca dua route sekaligus (`/api/v1/changelog` dan
  `/api/v1/version`), merender tiap rilis sebagai version + date + title + notes dengan marker
  `Running`/`Newer`/`Installed` dihitung terhadap versi yang berjalan, plus baris fakta "Read from GET
  /changelog." dan jumlah rilis. Komentar route diperbaiki dari §6.18 ke §6.16.
- Dua kebohongan ikut ditutup, keduanya di luar daftar fakta awal. Versi yang tidak bisa dibandingkan
  (`0.1.0-dev`, nilai yang benar-benar dilaporkan `app-serv` hari ini) dulu membuat status line berbunyi
  "This build is the newest release listed."; sekarang `canCompare` memisahkan dua sebab (baca gagal vs
  nilainya bukan nomor rilis) dan tidak ada rilis yang ditandai. Dan empty state tidak lagi menyebut sumber
  yang tidak ada: ia menyebut route yang menjawab dengan daftar kosong, karena catatannya ikut di dalam
  biner.
- Test: `tests/schemas/changelog.test.ts` ditulis ulang untuk derivasi (compareVersions, canCompare,
  releaseMarker, sortChangelog, countNewer), `tests/schemas/changelog-contract.test.ts` baru untuk kontrak
  wire, dan `tests/components/changelog.test.ts` baru untuk render (tiga state, urutan, marker, dua
  kegagalan versi, dan drift). 78 test di 3 berkas, table-driven per TDD §2.5. Satu pasangan grafis baru
  (marker `warn` di atas baris rilis) ditambahkan ke `tests/tokens/contrast.test.ts`, yang sekarang
  menjalankan 52 assertion di kedua tema (sebelumnya 50), dan angka itu ikut diperbarui di baris R-25
  SPEC-UI. Suite penuh di tree beku: 2065 test / 96 berkas, naik dari 2032 / 94 yang dicatat pass F3.
- Live pass 2026-09-21, 18 check, 0 gagal, di atas `app-serv` yang dibangun dari **HEAD ter-commit**
  (`git archive`, working tree sedang dipegang aktor lain): login lewat origin panel, lalu route dibaca tiga
  cara (byte mentah lewat forwarder panel: 200, `application/json`, 1821 byte, 5 rilis; lewat
  `fetchChangelog` panel sendiri sehingga payload live di-parse `schemaChangelog`; dan lewat
  `fetchSystemInfo`). Setiap marker dan hitungan newer dihitung ulang dengan aritmetika sendiri di driver,
  jadi pass ini tidak menguji modul terhadap dirinya sendiri. Empat kontrol: envelope lama ditolak, baris
  live tanpa `date` ditolak, `{data: []}` lolos (jalur data empty state), dan route menjawab 401 tanpa sesi
  baik langsung maupun lewat origin panel.
- Satu fakta tentang data yang dilayani, dan itu milik `app-serv`: biner melaporkan `0.1.0-dev` sementara
  changelog-nya sendiri sampai `v0.4.0`, jadi layar dengan jujur menulis "4 releases newer than this build"
  untuk biner yang justru menyajikan catatan itu. Panel tidak bisa memperbaikinya dengan menebak; ia
  mencetak versi yang dilaporkan gateway. Dicatat sebagai permintaan: konstanta versi `app-serv` tidak ikut
  naik mengikuti fase.
- Database live dikembalikan ke baseline persis; pass ini tidak menulis satu baris pun, jadi satu-satunya
  perubahan adalah hash bootstrap yang di-seed saat boot, di-null setelah server berhenti, dan berkas temp
  dihapus.
- Dokumen: §6.16 diamandemen ke bentuk yang dilayani (status, header, daftar rilis, states, alasan tidak
  membundel berkas rilis), §14 Q11 ditutup dengan keputusan dan buktinya, ledger §2 baris 16 plus butir
  owner di atasnya diperbarui, §12 mendapat catatan U0, §16 mencatat changelog-nya, baris R-38 mencatat
  buktinya, DESIGN.md §11 mendapat empat baris alasan, dan README `app-ui` mencatat pass ini.
- README `app-ui` sekaligus diukur ulang karena daftarnya sedang disentuh: tiga butir "open items" yang
  sudah tertutup (app-serv tidak bisa boot, playground belum punya fase, tidak ada sumber changelog) keluar
  dari daftar dan sisanya dinomori ulang, sehingga tidak ada lagi item yang menyatakan blocker yang sudah
  selesai. Itu menutup separuh kriteria F11 (klaim blocker); angka-angka README/§15 yang tersisa masih
  milik F11.
- Batas yang dicatat, bukan disembunyikan: click-through browser masih outstanding karena panel
  client-rendered, jadi separuh render hanya dari test jsdom; dan empty state tidak bisa dibuktikan live
  karena route selalu menjawab lima baris, jadi buktinya test render plus kontrol schema.

## 8. F5 (MEDIUM): Filter tab Upstream endpoints tidak memfilter dan tidak ada di URL

**Fakta.** `UpstreamEndpointsTab.svelte` menyimpan `providerFilter` dan `statusFilter` di state
komponen (baris 29-30), menghitung `shown` dari keduanya (baris 45-51), tetapi merender tabel
dengan `{endpoints}` (baris 164), yaitu seluruh halaman yang dibaca. Akibatnya: memilih
provider atau status tidak pernah mempersempit tabel; yang berubah hanya apakah empty state
"No endpoint matches these filters" muncul. `load()` juga memanggil `listEndpoints({ page,
per_page })` (baris 69) tanpa mengirim filter, padahal `GET /api/v1/endpoints` menerima
`?provider_id=&status=` (SPEC-API §7.5) dan `src/lib/api/endpoints.ts:41-44` sudah menyediakan
tipe query-nya. Filter juga tidak ada di URL, yang melanggar §8.4.2; pola yang benar sudah ada
di `UsageRecordsTab`/`LogsRequestsTab` (parse/next + `goto(resolve(...))`).

**Risiko.** Operator yang memilih satu provider melihat daftar yang tidak cocok dengan
filternya, atau empty state "tidak ada yang cocok" sementara barisnya ada di layar; tampilan
tidak bisa dibagikan dan navigasi maju/mundur tidak mengembalikannya. `upstream_endpoints`
adalah layar U0/U1, jadi cacatnya ada di jalur yang paling awal dipakai.

**Kerja yang diusulkan.** Pindahkan kedua filter ke search params dengan pola
`parseEndpointSearch`/`nextEndpointSearch` seperti `log-search.ts`/`usage-search.ts`, kirim
sebagai `provider_id` dan `status` ke API (server-side, sesuai §8.4.1), dan render hasil
server. Halaman (`page_`) ikut ke URL agar tampilan yang dibagikan konsisten.

**Kriteria selesai.** Test menegaskan filter mempersempit baris yang dirender **dan** menulis
URL; tidak ada penyaringan client-side atas satu halaman; §8.4.2 terpenuhi di layar ini.

**Status: CLOSED 2026-09-21.** Filter tab ini sekarang URL-backed dan server-side, tanpa
perubahan `app-serv` (route §7.5 sudah membaca `provider_id`, `status`, dan `page`):

- `src/lib/schemas/endpoint-search.ts` (131 baris) mem-parse `provider_id`, `status`, dan `page`
  dari URL dengan pola yang sama seperti `log-search.ts`/`usage-search.ts`. Nilai yang tidak bisa
  dipakai dikoreksi dan koreksinya muncul sebagai notice, bukan hilang: `status` yang tidak ada di
  select, `page` yang bukan angka atau di bawah 1, dan nilai yang melewati batas panjang field.
  `nextEndpointSearch` menulis satu perubahan ke URL dan membuang `page` saat filter berubah,
  sehingga nomor halaman hasil lama tidak terbawa ke hasil baru.
- `src/lib/components/UpstreamEndpointsTab.svelte` (211 baris) membangun request dari URL dan
  merender hasil server, bukan `shown` yang dulu dihitung lalu tidak dipakai. Tidak ada
  penyaringan client-side atas satu halaman (§8.4.1): `listEndpoints` menerima `provider_id`,
  `status`, dan `page` yang sama dengan yang ada di URL.
- Daftar opsi provider dibaca terpisah dan **tanpa filter** (`per_page=100`, batas §4), karena opsi
  yang diturunkan dari hasil terfilter akan membuat provider lain hilang dari select begitu satu
  filter dipasang. Bila bacaan itu gagal, filter tetap menawarkan provider yang ada di hasil
  halaman dan sebuah notice menyebut alasannya; provider yang dinamai URL selalu ikut ditawarkan,
  supaya select tidak pernah tampil kosong untuk filter yang sedang aktif.
- Tiga keadaan kosong dibedakan karena aksinya berbeda: filter tidak cocok (Clear filters), halaman
  melewati ujung daftar (Go to the first page), dan registry kosong (Choose a provider).
- `src/lib/components/EndpointFilters.svelte` (85 baris) memisahkan bar filter dan notice-notice-nya
  dari tab, yang tanpa pemisahan itu menembus batas baris; `src/lib/schemas/endpoint-options.ts`
  (39 baris) memisahkan derivasi opsi dari komponennya.
- Test: 4 berkas baru, 43 test: `tests/schemas/endpoint-search.test.ts` (18),
  `tests/schemas/endpoint-options.test.ts` (8), `tests/components/upstream-endpoints.test.ts` (9),
  `tests/components/upstream-endpoints-states.test.ts` (8), plus `tests/support/endpoint-stub.ts`,
  stub fetch bersama yang menjawab route list dengan filter dan paginasi server-side serta mencatat
  setiap request (stub yang sama juga melayani route bulk, yang dipakai F6). Bukti yang diminta
  kriteria selesai: pemilihan provider/status mengubah query yang dikirim dan baris yang dirender
  adalah baris yang dikembalikan server, URL ikut berubah, dan tidak ada penyaringan di browser.
- Batas yang dicatat: click-through browser tidak dijalankan pada pass ini (panel client-rendered,
  jadi separuh render hanya dari test jsdom); bukti live-nya adalah test yang mengunci nama parameter
  ke `src/lib/api/endpoints.ts` plus route `app-serv` yang membaca parameter itu, dan click-through
  tetap outstanding bersama layar U0/U1 lain di F12.

## 9. F6 (MEDIUM): Mode baris berulang untuk menambah banyak key sudah ditulis di klien, belum ada UI-nya

**Fakta.** §6.2 meminta "one form, plus a repeatable row mode for adding several keys in one
submit loop". Klien dan schema-nya sudah ada: `addEndpointKeys` memanggil
`POST /endpoints/{id}/keys/bulk` (`src/lib/api/endpoints.ts:133-144`) dengan
`schemaBulkAddKeysForm` (maksimum 100 baris, `src/lib/schemas/endpoint.ts:213-217`). Tidak ada
satu pun komponen yang memanggilnya: `AddEndpointKeyForm.svelte:30` memakai jalur satu-key.
Fungsi itu karena itu kode tanpa pemanggil, dan aturan §6.2-nya belum terpenuhi.

**Risiko.** Multi-key per endpoint adalah inti layar ini (Locked Decision 2), dan onboarding
akun dengan beberapa key tetap berarti satu submit per key. Kode klien yang tidak dipakai juga
tidak teruji, jadi bentuk respons `{created, results[]}` yang all-or-nothing tidak pernah
dibuktikan di panel.

**Kerja yang diusulkan.** Tambah mode baris berulang di drawer: daftar baris label+nilai yang
bisa ditambah dan dihapus, satu tombol submit yang mengirim satu batch, dan hasil per indeks
yang ditampilkan per baris (sukses atau pesan penolakan), dengan aturan all-or-nothing
dinyatakan di UI. Perbarui tabel key setelah sukses.

**Kriteria selesai.** Test render: batch dua baris sukses, batch dengan satu baris duplikat
ditolak seluruhnya dan pesannya menempel pada barisnya; tidak ada request terkirim saat
form kosong.

**Status: CLOSED 2026-09-21.** Mode baris berulang terpasang di drawer, tanpa perubahan `app-serv`:

- `src/lib/components/AddEndpointKeysPanel.svelte` (28 baris) menaruh dua mode §6.2 bersebelahan di
  drawer: `One key` (form lama, satu jalur `POST /endpoints/{id}/keys`) dan `Several keys` (mode baris
  berulang). Panel-nya memakai `PanelTabs` yang sudah ada, jadi tidak ada kontrol tab kedua.
- `src/lib/components/BulkAddKeysForm.svelte` (198 baris) mengirim satu batch: baris label+nilai yang
  bisa ditambah dan dihapus, satu tombol submit, dan hasil per baris. Aturan all-or-nothing dinyatakan
  di UI sebelum submit, dan batas 100 baris §7.5 ditegakkan di panel dengan pesan, bukan dengan
  membiarkan server yang menolak setelah satu round trip.
- Penolakan seluruh batch dibaca, bukan dibuang: `schemaBulkKeyRefusal` (kini di
  `src/lib/schemas/endpoint-bulk.ts`) mem-parse `{error, results[]}`, dan pesan tiap baris dipetakan ke
  indeks yang dilaporkan server sehingga pesannya milik server, bukan rekonstruksi panel. Semua baris
  tetap di layar saat batch ditolak, karena operator harus memperbaiki salah satunya; baris dibersihkan
  hanya setelah sukses. Outcome dibersihkan pada setiap edit, karena pesan dari batch sebelumnya akan
  menunjuk baris yang sudah berubah.
- Jalur transportnya diperluas tanpa merusak pemanggil lama: `apiRequest` menerima `refusalSchema`
  opsional dan mengembalikan `ApiResult<T, R>` dengan `refusal?: R`, sehingga body penolakan dibaca
  sekali di klien dan diteruskan ke pemanggil sebagai tipe, bukan sebagai `any`. `errorFromPayload`
  menggantikan pembacaan body kedua di `errors.ts`.
- Split karena batas baris: tiga bentuk wire batch pindah ke `src/lib/schemas/endpoint-bulk.ts`
  (39 baris) sehingga `src/lib/schemas/endpoint.ts` kembali ke 198 baris, di bawah ambang 220 yang
  diklaim §3 dokumen ini.
- Test: `tests/components/bulk-add-keys.test.ts` (9 test): batch dua baris terkirim dalam satu request
  dan dilaporkan, batch yang ditolak seluruhnya menyimpan semua barisnya dan menyatakan tidak ada yang
  disimpan, pesan baris yang ditolak menempel pada input baris itu lewat `aria-describedby` (§8.8.5),
  tidak ada request terkirim saat satu baris kosong atau terlalu pendek, tombol baris menghormati batas
  baris terakhir, dan outcome hilang saat batch diedit. Dua test terakhir di berkas itu mengunci
  kontrak `schemaBulkKeyRefusal` (dibaca bila ada `results`, ditolak bila tidak ada).
- Dua cacat yang ikut tertutup di jalur ini, keduanya ditemukan saat menulis test: form itu memakai
  pola label-membungkus-input sehingga nama aksesibel field menyerap hint "(optional)"
  (`getByLabelText('Label for row 1')` gagal); sekarang `for`/`id` eksplisit dengan `aria-describedby`
  untuk hint-nya. Dan test tabelnya sempat memanggil `it()` di dalam `it()`; helper `forEachCase`
  memang menjadikan callback-nya badan test, jadi baris tabelnya sekarang bernama test penuh.
- Batas yang dicatat: drawer belum punya test render sendiri, jadi mode ini diuji sebagai komponen
  dengan stub route bulk; click-through browser tetap outstanding bersama U0/U1 lain (F12).
  Ditutup F7 2026-09-21: `tests/components/endpoint-detail-drawer.test.ts` merender drawer dan
  menekan kedua mode tambah key, jadi batas itu tidak lagi berlaku.

## 10. F7 (MEDIUM): Empat layar tidak punya test render sama sekali

**Fakta.** Diukur dengan mencari import `src/routes/<route>/+page.svelte` di `tests/`: yang
punya test render adalah `providers/[provider_id]` (4 file), `quota` (2), `token-saver` (2),
`media-providers/[kind]` (2), `proxy-pools` (3), `settings` (1), plus test tingkat tab untuk
combos (`CombosTab`), logs (`LogsRequestsTab`), console-log (`ConsoleLog`), usage
(`UsageOverviewTab`, `UsageRecordsTab`). Yang **tidak** punya test render di tingkat mana pun:
`/login`, `/endpoint-keys` (kedua tab), `/providers` (list; halaman detailnya punya), dan
`/changelog`. Seluruh permukaan tab 2 §6.2 (EndpointTable, EndpointDetailDrawer,
EndpointFieldsForm, EndpointKeysTable, AddEndpointKeyForm, CreateEndpointForm) tidak dirender
test mana pun; `tests/schemas/endpoint.test.ts` hanya menguji schema, termasuk
`schemaAddEndpointKeyForm`, bukan komponennya.

**Risiko.** Ini layar U0/U1, dan `docs/RULLES/TDD.md` §2.1 mensyaratkan satu test happy path
plus satu test validasi per route. Cacat seperti F5 tepatnya ada di area tanpa test; gerbang
`bun run test` hijau tidak mengatakan apa pun tentang layar-layar ini.

**Kerja yang diusulkan.** Test render table-driven per layar dengan fetch stub: gateway keys
(create lalu modal satu kali, rename, disable, revoke, empty state), tab endpoints (filter,
paginasi, drawer, blokir last active key, test), providers list (filter kategori, dua empty
state), login (password salah, rate limited, redirect), changelog (tiga state plus marker).
Tambahkan juga test untuk mode F6 begitu UI-nya ada.

**Kriteria selesai.** Tiap layar di atas punya test render yang mengasertif state-nya terhadap
stub; tidak ada penambahan test yang menembus 250 baris tanpa pemisahan per concern.

**Status: CLOSED 2026-09-21.** Empat layar yang tidak punya test render sekarang punya, dan menulis
test itu terhadap bentuk yang benar-benar dilayani `app-serv` membuka tiga cacat wire yang ikut
ditutup di commit yang sama (changelog sudah ditutup F4):

- Test baru: 29 test di 5 berkas: `tests/routes/login.test.ts` (7),
  `tests/routes/providers-list.test.ts` (5), `tests/components/gateway-keys-tab.test.ts` (5, list dan
  alur create), `tests/components/gateway-key-actions.test.ts` (5, rename/disable/revoke),
  `tests/components/endpoint-detail-drawer.test.ts` (7, tab 2 detail), plus
  `tests/support/gateway-key-stub.ts` (87 baris) sebagai stub bersama kedua berkas gateway keys.
  `tests/components/gateway-keys-tab.test.ts` dipecah dua ketika mencapai 270 baris, sesuai kriteria
  selesai di atas; keduanya 109 dan 112 baris.
- Cacat 1, §6.1 tidak diterapkan: layar login merender kalimat gateway apa adanya ("invalid
  password", "too many attempts; try again later") dan tidak pernah menyatakan aturannya maupun
  lama tunggunya, padahal §6.1 menetapkan dua kalimatnya dan meminta jendela retry dari respons.
  Sekarang `src/lib/strings/login.ts` (33 baris) memegang dua kalimat §6.1, `ApiError` membawa
  `retryAfterSeconds` dari header `Retry-After` (dibaca di `errors.ts`), dan layar memakai jendela
  dari respons dengan aturan 15 menit sebagai fallback. Menit dibulatkan ke atas supaya kalimatnya
  tidak pernah berbunyi "in 0 minutes".
- Cacat 2, gateway keys tidak cocok dengan wire yang dilayani: create menjawab `plaintext_key`
  (panel menuntut `key`), update menjawab baris key tanpa plaintext sama sekali (panel mem-parse
  bentuk create yang menuntutnya), dan `last_used_at`/`revoked_at` adalah `omitempty` sehingga key
  yang belum pernah dipakai tidak mengirim field itu (panel menuntut `nullable`). Akibatnya setiap
  create gagal parse sehingga kunci sekali-tampil tidak pernah sampai ke modal, dan setiap rename
  serta toggle gagal parse yang lalu dibuang barisnya. Ketiganya diperbaiki di skema dan
  `updateGatewayKey` sekarang mem-parse `schemaGatewayKey`.
- Cacat 3, aksi yang gagal tidak bersuara: `GatewayKeyRow` membuang hasil update dan
  `GatewayKeysTab` membuang hasil revoke, jadi rename yang ditolak gateway tampak seperti berhasil.
  Sekarang barisnya menyimpan draft dan pesan gateway (`role="alert"`), dan tab menampilkan
  penolakan revoke pada barisnya sendiri alih-alih mengganti tabel dengan state error.
- Dokumen: SPEC-API §7.3 barisnya sekarang menamai fieldnya (`plaintext_key`, absent di respons
  lain), karena tabel itu sebelumnya hanya berbunyi "returns full key once" dan nama field yang
  tidak disebut adalah nama yang ditebak salah.
- Batas yang dicatat: cacat wire di atas ditemukan dengan membaca DTO dan handler yang dilayani,
  bukan dari panggilan live; pass ini tidak menyalakan `app-serv`. Click-through browser tetap
  outstanding bersama U0/U1 lain (F12), dan layar gateway keys belum pernah diuji terhadap gateway
  yang hidup.

## 11. F8 (MEDIUM): Tidak ada kontrol refresh eksplisit di layar daftar mana pun kecuali `/quota`

**Fakta.** §8.6.2: "Every list view has an explicit refresh control, because operators distrust
auto-refresh they cannot trigger". Yang ada di panel: `Refresh now` hanya di
`src/routes/quota/+page.svelte:127`, dan `Auto refresh` di console log. Layar daftar lain
(endpoint-keys dua tab, providers, combos, usage dua tab, logs, proxy-pools,
media-providers, changelog) hanya menyediakan `Try again` di dalam state error, bukan kontrol
yang bisa ditekan kapan saja. Header panel juga tidak punya kontrol global (hanya Copy, tema,
sign out).

**Risiko.** Operator yang mencurigai data basi harus memuat ulang halaman; aturan §8.6.2
tertulis dan tidak dipenuhi di 9 layar, dan tidak ada test yang menutupnya.

**Kerja yang diusulkan.** Satu komponen refresh kecil yang menerima fungsi `load` dan
menyatakan hasilnya, dipasang di tiap layar daftar. Bila owner membaca §8.6.2 sebagai aturan
untuk layar yang polling saja (quota dan console log), alternatifnya adalah amandemen satu
kalimat di §8.6.2 dengan alasannya, dan tidak ada kode yang ditulis.

**Kriteria selesai.** Setiap layar daftar punya kontrol refresh yang teruji, atau §8.6.2
diamandemen dengan keputusan owner.

**Status: CLOSED 2026-09-21.** Owner memilih opsi (a) pada 2026-09-21 lewat D4: satu komponen
bersama dipasang di setiap layar daftar, dan §8.6.2 tetap seperti tertulis. Tidak ada perubahan
`app-serv`:

- `src/lib/components/RefreshControl.svelte` (42 baris) adalah satu-satunya tempat aturannya
  hidup: nama aksesibelnya "Refresh now", labelnya berubah menjadi "Refreshing" selama bacaan
  berjalan, dan klik kedua selama bacaan pertama masih jalan dibuang oleh guard, bukan oleh
  `disabled`. Itu keputusan yang sudah dicatat `/quota` untuk kontrolnya sendiri, dan alasannya
  sama: gerbang yang mati menelan klik dan terbaca sebagai tombol rusak, sedangkan label yang
  berubah mengakui kliknya dan state loading layar yang menunjukkan bacaannya. Baris pembungkus
  (`flex flex-wrap items-center gap-3`) ikut di dalam komponen, karena tombol telanjang di dalam
  kolom akan melebar selebar layar dan setiap pemanggil harus mengulang pembungkus yang sama.
- Sebelas layar memakainya, satu baris per layar: `GatewayKeysTab` (kedua tab lewat satu tab),
  `UpstreamEndpointsTab`, `/providers`, `CombosTab`, `UsageOverviewTab`, `UsageRecordsTab`,
  `LogsRequestsTab`, `/proxy-pools`, `/media-providers/[kind]`, `/changelog`, dan `/console-log`.
  Baris 13 tabel di §2 menandai layar console log untuk F8, dan layar itu memang kasus terkuat
  dari kalimat §8.6.2: satu-satunya kontrolnya adalah poll yang bisa di-*pause*, jadi poll yang
  dijeda meninggalkan muat ulang halaman sebagai satu-satunya cara membaca buffer lagi. `/quota`
  tetap memakai kontrolnya sendiri yang sudah ada dan sudah teruji, jadi tidak ada dua kontrol di
  satu layar.
- Kontrolnya mengulang bacaan yang sedang tampil, bukan bacaan default: layar berfilter memanggil
  `load(search)` dengan `search` yang sama dengan URL, dan `/providers` mengulang `load` yang
  membaca `category`/`page` dari state. Di `/media-providers/[kind]` kontrolnya **absen** saat
  alamat tidak menamai kind yang panel punya, karena di sana tidak ada bacaan untuk diulang dan
  tombol yang diam-diam tidak melakukan apa pun akan terbaca rusak; ketiadaan itu diuji, bukan
  dibiarkan.
- Test: satu test per layar, sebelas seluruhnya, yang menekan kontrolnya lalu membuktikan
  bacaan berikutnya terjadi. Yang lebih dari sekadar "ada tombol": layar berfilter membuktikan
  query yang diulang identik dengan yang sedang tampil (`/providers` tetap membawa `category`,
  usage dan logs tetap membawa filternya, upstream endpoints tetap membawa
  `provider_id`/`status`), `/changelog` membuktikan **kedua** route dibaca ulang, bukan hanya
  daftar rilisnya, dan test console log menjeda pollnya lebih dulu supaya bacaan baru hanya bisa
  datang dari kontrolnya.
  `tests/components/refresh-control.test.ts` (3 test) menutup perilaku komponennya sendiri: klik
  memanggil callback, label bertukar sambil tetap bisa ditekan, dan klik kedua dibuang lalu
  diizinkan lagi setelah bacaan mendarat.
- Dua stub bersama diperluas untuk bisa membuktikan itu: `tests/support/gateway-key-stub.ts` dan
  `tests/support/proxy-stub.ts` sekarang mencatat bacaan (`reads`), bukan hanya tulisan. Sebelum
  itu stub keduanya hanya mencatat tulis, jadi layar yang tidak pernah membaca ulang akan lulus.
- Batas yang dicatat: `CombosTab.svelte` duduk tepat di 220 baris, garis peringatan, naik dari 217
  karena pemasangan tiga baris (import, baris kosong, satu baris kontrol); memecah tab itu bukan
  bagian dari F8 dan tidak dilakukan di sini. Click-through browser tetap outstanding bersama
  layar U0/U1 lain (F12), dan pass ini tidak menyalakan `app-serv`: yang dibuktikan adalah bacaan
  yang diulang, bukan tampilan di browser.

## 12. F9 (LOW): `rate_limited_until` tampil mentah, bukan countdown

**Fakta.** §6.2 meminta kolom "`rate_limited_until` countdown", dan §4 menetapkan waktu
ditampilkan dalam zona waktu browser dengan label zona. `EndpointKeysTable.svelte:52-57`
mencetak nilai RFC3339 apa adanya ("rate limited until 2026-09-20T10:00:00Z"). Helper
`countdownText` dan `formatTimestamp` sudah ada di `src/lib/utils/time.ts` dan dipakai
`QuotaTable.svelte`.

**Risiko.** Kecil tetapi nyata: satu kolom di layar U0 membaca seperti dump, dan sisa waktu
tidak bisa dibaca sekilas padahal itu satu-satunya gunanya.

**Kerja yang diusulkan.** Ganti menjadi `formatTimestamp` plus `countdownText`, dengan `now`
yang di-tick bila countdown ingin hidup; atau tampilkan waktu absolut dengan zona plus sisa
waktu statis bila ticking dianggap berlebihan.

**Kriteria selesai.** Test menegaskan kolom memuat waktu terformat, bukan string RFC3339 mentah.

**Status: CLOSED 2026-09-21.** Kolomnya sekarang countdown, tanpa perubahan `app-serv`:

- `EndpointKeysTable.svelte` mencetak `rate limited until <waktu terformat> (<sisa waktu>)`.
  `formatTimestamp` memberi waktu absolut dengan label zona (§4) dan `countdownText` memberi sisanya;
  keduanya helper yang sudah dipakai `QuotaTable`, jadi tidak ada formatter kedua yang bisa melenceng.
- Countdown-nya hidup, bukan beku: drawer men-tick `now` satu detik sekali (`TICK_MS = 1000`, satuan
  terkecil yang dicetak, jadi tick yang lebih cepat hanya menggambar ulang tanpa mengubah bacaan) dan
  hanya selama drawer terbuka, karena tabel yang tidak di layar tidak punya apa pun untuk dihitung dan
  timer yang tertinggal adalah kebocoran. Tabelnya tetap murni renderer: ia menerima `now`, tidak
  memiliki jamnya sendiri, jadi test bisa memindahkan jamnya alih-alih menunggu.
- Test: `tests/components/endpoint-keys-table.test.ts` (7 test) menegaskan sel memuat waktu terformat
  dan bukan string RFC3339 mentah, empat offset (jam, menit, di bawah satu menit, dan yang sudah
  lewat), baris tanpa rate limit tidak menyebut apa pun, dan satu kasus memindahkan `now` setengah jam
  ke belakang untuk membuktikan countdown dihitung terhadap jam yang diberikan. Harapannya dibangun
  dengan helper panel sendiri (`formatTimestamp`/`countdownText`) supaya pass ini tidak mengunci locale
  mesin uji.
- Batas yang dicatat: yang diuji adalah tabel pada dua `now` berbeda, bukan `setInterval`-nya; yang
  ingin dibuktikan "kolom menghitung terhadap jam yang diberikan", dan itu terbukti tanpa fake timer.
  Drawer yang memegang tick belum punya test render sendiri (itu F7). Ditutup F7 2026-09-21:
  drawer-nya sekarang punya test render, tetapi tick satu detiknya sendiri tetap tidak diuji, dan
  itu tetap disengaja: yang dibuktikan adalah countdown dihitung terhadap jam yang diberikan.

## 13. F10 (LOW): §8.4.4 (dirty guard) dan §8.4.5 (validasi saat blur) belum diterapkan

**Fakta.** Tidak ada `beforeunload` maupun `onNavigate` di seluruh `app-ui/src/`; form
memvalidasi saat submit saja (`safeParse` di handler submit). §8.4.4 meminta peringatan sebelum
meninggalkan form dengan perubahan belum tersimpan, dan §8.4.5 meminta validasi saat blur dan
submit, bukan tiap ketikan. Beberapa tab settings sudah punya indikator "Unsaved changes" plus
Discard, yang memenuhi semangat §8.4.4 di dalam halaman tetapi tidak saat meninggalkannya.

**Risiko.** Operator bisa kehilangan draft konfigurasi (settings, token saver, proxy) tanpa
peringatan; pesan validasi baru muncul setelah submit, bukan setelah field ditinggalkan.

**Kerja yang diusulkan.** Owner memutuskan: (a) implementasi satu guard bersama untuk form yang
punya draft (settings tiga tab, token saver, proxy, combo editor) plus validasi blur pada form
multi-field, atau (b) amandemen §8.4.4 dan §8.4.5 dengan alasan yang ditulis, karena panel
memvalidasi di submit dan menampilkan indikator dirty.

**Kriteria selesai.** Salah satu jalur dipilih; bila (a), ada test untuk keduanya.

**Status: CLOSED 2026-09-21.** Owner menjawab D5 pada 2026-09-21 dengan jalur tengah: guard bersama
diimplementasikan untuk form yang punya draft, dan §8.4.5 diamandemen dengan alasan tertulis. Tidak ada
perubahan `app-serv`:

- `src/lib/dirty-guard.ts` (49 baris) adalah satu-satunya tempat aturannya hidup. Tiga fungsi:
  `registerDirtyForm(isDirty)` menyimpan getter dan mengembalikan pelepasnya, `hasDirtyForm()` membaca
  getter itu **saat navigasi terjadi** (bukan saat form mendaftar, jadi draft yang sudah disimpan atau
  dibuang berhenti memperingatkan tanpa pembukuan di form), dan `shouldCancelNavigation(navigation,
  confirmLeave)` memisahkan dua jalur pertanyaan.
- Dua jalur itu dipisah karena dialognya berbeda: navigasi yang melepas dokumen (`willUnload`) cukup
  dibatalkan, dan browser sendiri yang menanyakan; navigasi di dalam panel tidak punya dialog browser,
  jadi guard bertanya lewat `window.confirm`, yang sinkron dan karena itu bisa dipakai `beforeNavigate`.
  `confirm()` tidak bisa ditata seperti modal panel, dan itu keputusan yang dicatat: alternatifnya satu
  modal kustom yang harus menunda navigasi, yaitu bahasa dialog kedua untuk aturan yang browsernya sudah
  menjawab sendiri di jalur unload.
- Pemasangannya satu tempat: `src/routes/+layout.svelte:36-40` memanggil `beforeNavigate` dan
  membatalkan lewat `shouldCancelNavigation`, dengan alasan yang sama seperti gerbang sesi di berkas itu,
  yaitu satu aturan untuk semua layar yang tidak bisa dilupakan satu layar.
- Enam form berdraft mendaftar, satu efek per form: tiga tab Settings (`SettingsSecurityTab`,
  `SettingsRoutingTab`, `SettingsLoggingTab`), `TokenSaverForm` (dirty bila salah satu dari tiga section
  berubah, karena meninggalkan layar kehilangan ketiganya), `ProxyOutboundSettings`, dan `ComboEditor`.
  Form yang lain tidak mendaftar dan itu disengaja: hanya form yang punya baseline hasil baca server yang
  bisa mengatakan apa arti "belum tersimpan"; dialog yang membuang draftnya saat Cancel tidak punya draft
  untuk hilang.
- `ComboEditor` mendapat baseline baru: `comboFormDirty(baseline, form)` di `combo-form.ts` (142 baris)
  membandingkan field demi field, dan baseline-nya `$state.snapshot` dari salinan yang di-seed, karena
  proxy `$state` menulis menembus objek yang dibungkusnya sehingga satu objek bersama akan ikut bergerak
  dengan tiap ketikan. Pemasangan guard mendorong berkas itu dari 218 baris ke atas ambang 220, jadi dua
  field yang visibilitasnya mengikuti strategi dipindah ke `ComboStrategyFields.svelte` (60 baris),
  persis seam yang header editor itu sudah sebut; `ComboEditor` sekarang 201 baris.
- Test: 17 baru di 3 berkas, semuanya hijau. `tests/dirty-guard.test.ts` (9) menutup registry dan
  keputusan navigasi (tabel empat baris: bersih, unload, tolak, izinkan; plus kalimat yang ditanyakan dan
  pembacaan getter saat navigasi); `tests/components/dirty-forms.test.ts` (6, 178 baris) merender lima form
  asli bergaya settings dengan fixture test layarnya masing-masing dan membuktikan tiga hal per form: form
  yang baru dimuat belum kotor, satu suntingan membuatnya kotor, dan mengembalikan nilainya membuatnya
  bersih lagi, plus satu test yang membuktikan form yang di-unmount melepas pendaftarannya; dan
  `tests/components/dirty-combo-form.test.ts` (2, 95 baris) memegang dua kasus combo editor, combo
  tersimpan dan mode create. Form yang mendaftar sekali saat mount lalu tidak pernah membaca ulang akan
  gagal di asersi kedua. Berkas kedua dipisah dari yang pertama setelah Prettier mengalirkannya ke 225
  baris, di atas ambang 220, pada seam yang sudah ada: combo editor satu-satunya form yang dirty-nya
  perbandingan field demi field, bukan perbandingan dokumen settings.
- Batas yang dicatat, bukan disembunyikan: yang diuji adalah keputusan guard dan pendaftaran tiap form,
  bukan `beforeNavigate`-nya sendiri, karena harness test panel merender komponen tanpa router; menekan
  tautan navigasi dengan draft kotor adalah click-through browser yang tetap outstanding bersama F12.
  Tombol Cancel/Discard yang eksplisit tetap tidak bertanya, karena itu pembuangan yang disengaja, bukan
  meninggalkan form.

## 14. F11 (LOW): Angka dan klaim dokumen yang basi

**Fakta.** Beberapa klaim terukur tidak lagi cocok, dan dua di antaranya sudah selesai:

- `app-ui/README.md` "Open items that block later phases" butir 1 (app-serv tidak bisa boot),
  butir 2 (playground belum punya fase), dan butir 3 (tidak ada sumber changelog) semuanya
  sudah terjawab oleh P1/P4. **Ketiganya sudah keluar dari daftar pada 2026-09-21 lewat pass
  F4**, dan daftarnya dinomori ulang; yang tersisa adalah drift gate (kini butir 1), lingkup
  `AGENTS.md` (butir 2), dan enum status gateway key (butir 3). Butir drift gate menyebut
  berkas `docs/SPEC-API/002-SPEC-API-openapi.md` yang memang masih belum ada, tetapi premisnya
  berubah dan sudah ditulis ulang di README: sejak P4 kontraknya dilayani sebagai
  `GET /api/v1/openapi.json`, jadi gate itu punya bentuk baru yang bisa dipakai.
- README baris status menyebut "19 owner rows"; hasil ukur hari ini: 20 node, 14 baris
  top-level, 16 routable (10 route statis + 6 kind media), 3 planned.
- SPEC-UI §15 baris "Sidebar row count" masih "8 with a route, 11 planned leaves"; hasil ukur
  hari ini 16 routable, 3 planned, 1 container (total 20 tetap).
- SPEC-UI §12 tidak punya paragraf status untuk **U1**, padahal U0 dan U2 punya.
- Komentar `src/routes/changelog/+page.svelte:2` menyebut §6.18; section changelog adalah
  §6.16. **Diperbaiki pada 2026-09-21 lewat pass F4.**

**Risiko.** README dan §15 dibaca sebagai bukti (R-36); klaim "blocked" yang sudah selesai
mengarahkan pekerjaan berikutnya ke tempat yang salah, dan angka yang tidak diukur ulang
adalah klaim yang tidak berdasar.

**Kerja yang diusulkan.** Ukur ulang tiap angka dengan perintahnya dan tulis hasilnya; ganti
daftar "open items" README dengan yang benar-benar terbuka (F1 sampai F3, F12, dan permintaan
`app-serv`); tambahkan paragraf status U1 di §12; perbaiki rujukan §6.18.

**Kriteria selesai.** Tidak ada angka di README/§15 tanpa perintah ukur yang bisa diulang;
tidak ada item README yang menyatakan blocker yang sudah tertutup.

**Status: CLOSED 2026-09-21.** Setiap angka diukur ulang dengan perintahnya, dan setiap klaim yang basi
diperbaiki di dokumen pemiliknya. Tidak ada perubahan kode:

- Sidebar diukur ulang lewat `bun -e` atas `NAV_GROUPS` dan `allNodes` (`src/lib/navigation.ts`): 5 grup,
  20 node, 19 leaf routable (13 route statis plus 6 kind media), 1 container, 0 baris `Planned`. Sebuah leaf
  dihitung routable bila ia membawa `href` atau `link`, karena enam baris kind media membawa `link` ke route
  berparameter `/media-providers/[kind]`, bukan `href`; menghitung `href` saja akan melaporkan 13.
  `grep -c 'planned: true'` menemukan 1, yaitu container Media Provider yang dirender sebagai disclosure;
  baris "Sidebar row count" §15 karena itu sekarang menyebut perintah `bun -e` sebagai cara ukurnya,
  bukan `grep -c` saja.
- README: baris statusnya menyebut 18 routable dan 1 `Planned` dari keadaan sebelum `/playground`;
  sekarang 19 routable, 0 `Planned`, dan `/playground` masuk daftar "Screens built", tempat layar itu
  sebelumnya tidak pernah dicatat. Butir "Open items" nomor 1 menyatakan gate drift "belum wired",
  padahal `scrypts/gates/contract-drift.sh` sudah membandingkan SPEC-API §8 dengan enum panel dan
  berjalan di `scrypts/gates/all.sh`, dan `scrypts/gates/contract-openapi.sh` memaku dokumen yang
  dilayani ke kontrak YAML; butir itu sekarang menyatakan separuh yang benar-benar terbuka, yaitu schema
  respons panel tidak dibandingkan ke kontrak yang dilayani oleh gate mana pun. Kalimat penutup daftar
  itu menyebut "dua item keluar" padahal F4 mencatat tiga (boot `app-serv`, fase playground, sumber
  changelog); angkanya diperbaiki.
- SPEC-UI §15: baris "Panel test count" masih 1053 dari pass Logs; sekarang 2185 di 111 berkas (diukur pass
  ini) dan
  rantai riwayatnya diperpanjang, bukan diganti. Baris path `app-ui/` masih menyatakan sisa layar
  `Planned`; sekarang menyatakan 19 leaf routable dan 0 `Planned`, dengan click-through terekam sebagai
  satu-satunya langkah yang tersisa. Baris "Sidebar row count" sudah benar, hanya cara ukurnya diperjelas.
- SPEC-UI §12: paragraf status U1 ditambahkan; U0, U2, dan U3 sudah punya, U1 tidak. Isinya menyebut tiap
  layar di baris U1 sudah dibangun, tab Network adalah pointer ke Proxy Pools sesuai §6.9, dan exit
  criterion-nya adalah click-through terekam yang jadi milik F12.
- SPEC-UI §5.1: baris `/api-docs` masih menyebut "rendered from SPEC-API §7", padahal layarnya merender
  dokumen yang dilayani (`GET /api/v1/openapi.json`); barisnya sekarang menyebut itu. Itu sisa yang F2
  catat sebagai di luar lingkupnya.
- Dua butir F11 yang sudah selesai lebih dulu dicatat apa adanya: rujukan §6.18 di komentar route
  changelog (F4) dan tiga butir "open items" README yang blocker-nya sudah tertutup (F4).
- Batas yang dicatat: angka test count di §15 dan README diukur pada tree beku pass ini, jadi pass
  berikutnya menggesernya lagi; itu sebabnya tiap baris menyebut perintah pengukurnya.

## 15. F12 (MEDIUM): Bukti click-through U0, U1, dan dua kriteria U2 belum tercatat

**Fakta.** §9.4.5 dan R-35 mensyaratkan click-through terekam per elemen. SPEC-UI §12 U0
menyatakan "verification run outstanding"; §12 tidak punya status U1 sama sekali, dan exit
criterion U1 ("operator connects a provider endpoint with two keys, builds a `fallback` combo,
routes one request, reads usage row, quota window, and log detail that all match") belum
tercatat di mana pun. U2: dua dari empat criteria terpenuhi; OAuth round trip dan image
generation melalui media provider belum, dan keduanya butuh akun provider. Tujuh dari lima belas
bagian pass di `app-ui/README.md` menyebut click-through browser sebagai masih outstanding (diukur
ulang 2026-09-21 dengan `awk` per bagian atas README; angka draft "tujuh dari sembilan" tidak bisa
direproduksi, karena pengukuran yang sama pada commit draft `13ea269` menemukan satu dari sembilan
bagian). Pengukuran yang sama setelah pass "API base dialog dan copy key sekali-tampil"
(2026-09-21) menemukan delapan dari enam belas bagian: pass itu menambah bagian ke-16, dan bagian
itu menyebut click-through dialog barunya sendiri masih outstanding, jadi angka di kalimat
sebelumnya adalah keadaan sebelum commit pass tersebut, bukan angka yang salah saat ditulis.
Pengukuran yang sama setelah pass "The click-through's two findings" (2026-09-21) menemukan
sembilan dari tujuh belas bagian: bagian ke-17 mencatat click-through owner yang menemukan dua
cacat pada permukaan bagian ke-16 dan menyatakan perbaikannya belum diverifikasi ulang di browser,
jadi bagian itu terhitung juga, dan F12 tetap terbuka.

Pengukuran yang sama setelah pass "Custom provider surface" (2026-09-22) menemukan **sepuluh dari
delapan belas bagian**: bagian ke-18 menyebut permukaan barunya belum pernah dijalankan di browser
dan tidak dijalankan terhadap gateway yang hidup, jadi bagian itu terhitung juga. Perintah yang
dipakai pengukuran ini dicatat di sini, karena angka-angka sebelumnya tidak dapat direproduksi
(sebagaimana dicatat di paragraf di atas). Dijalankan dari root repo:

```bash
python3 - <<'PY'
import re
text = open('app-ui/README.md').read()
body = text[text.index('## Verification state'):]
body = body[: re.search(r'\n## ', body[5:]).start() + 5]
sections = [s for s in re.split(r'\n(?=### )', body) if s.startswith('### ')]
hits = [s for s in sections if re.search(r'(?s)click-through.*outstanding|outstanding.*click-through', s)]
print(len(hits), 'of', len(sections))
PY
```

Perintah itu membaca setiap bagian `###` di bawah `## Verification state` dan menghitung bagian yang
memuat "click-through" dan "outstanding" dalam urutan mana pun. Dijalankan pada commit ini ia
mencetak `10 of 18`; dijalankan pada tree sebelum pass ke-18 ia mencetak `9 of 17`, yang sama
dengan angka yang tercatat di paragraf sebelumnya.

Pengukuran yang sama setelah pass "Panel runtime config" (2026-09-22) menemukan **sepuluh dari
sembilan belas bagian**, dan perintah yang sama mencetak `10 of 19` pada commit pass itu. Penyebutnya
naik satu sementara pembilangnya tetap: pass ke-19 mengubah jalur boot (variabel runtime dan port
yang di-bind), tidak menambah layar maupun kontrol, dan bagiannya mencatat lima kasus boot yang
dijalankan langsung sebagai gantinya, jadi tidak ada click-through yang outstanding di bagian itu.

Pengukuran yang sama setelah pass "Usage live surface" (2026-09-22) menemukan **sebelas dari dua
puluh bagian**, dan perintah yang sama mencetak `11 of 20` pada commit pass itu (`10 of 19` pada tree
sebelumnya). Bagian ke-20 menambah permukaan live di `/usage` dan menyatakan click-through
browser-nya belum dijalankan: panel ini client-rendered, dan route `GET /api/v1/usage/live` yang
menjadi lawan bicaranya belum ada di tree `app-serv` (F4), jadi bagian itu terhitung juga.

**Risiko.** Panel bisa tampak selesai padahal setengah exit criteria-nya belum pernah
dijalankan orang. Ini persis bentuk klaim yang R-35/R-36 larang.

**Kerja yang diusulkan.** Satu pass live di atas stack berjalan yang menelusuri alur U0
(login, key create, modal satu kali, rename, disable, revoke) dan alur U1 (endpoint dua key,
combo fallback, satu request, lalu usage/quota/log yang saling cocok), plus dua kriteria U2
bila akun provider tersedia. Setiap elemen dicatat hasilnya di README dengan tanggal. Yang
tidak bisa dijalankan (OAuth start path dorman, media butuh akun) dicatat sebagai terblokir
dengan penyebabnya, bukan diklaim lulus.

**Kriteria selesai.** Click-through terekam per §9.4.5 dengan hasil per elemen; paragraf status
U0 dan U1 di §12 diperbarui; dua kriteria U2 dinyatakan terpenuhi atau terblokir dengan
penyebab yang disebut.

## 16. Keputusan yang diminta owner

| #   | Pertanyaan                                                                                                                                                    | Pilihan yang diusulkan                                            | Dipakai oleh                                                                                                                                   |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| D1  | Katalog Skills: §6.10 (dua entri: `/antislop` AI, SuperPowers) atau §7.16 (satu entri per capability, 7 baris)? Dan siapa menulis `skills/<id>/SKILL.md`?     | Ikuti katalog yang dilayani, amandemen §6.10, dan tulis 7 dokumen | F2, dijawab owner 2026-09-21: ikuti katalog yang dilayani, agen menulis 7 dokumen, probe saat load plus `Check again`, commit lokal saja       |
| D2  | Bentuk rilis changelog: render yang dilayani (`version, date, title, notes`) dengan amandemen §6.16, atau `app-serv` menambah `category` + `items[]`?         | Render yang dilayani, amandemen §6.16                             | F4 **CLOSED 2026-09-21** (dipakai; `app-serv` tidak disentuh)                                                                                  |
| D3  | API Docs: cukup yang ada di dokumen (path, tag, summary, auth) dengan tabel error ditunjuk lewat kalimat, atau minta `x-error-codes`/`x-phase` ke `app-serv`? | Cukup yang ada, tanpa salinan kedua                               | F1                                                                                                                                             |
| D4  | Refresh control: pasang di semua layar daftar, atau amandemen §8.6.2 menjadi khusus layar yang polling?                                                       | Pasang satu komponen bersama                                      | F8 **CLOSED 2026-09-21** (dipakai owner: satu komponen bersama di sebelas layar daftar, §8.6.2 tetap seperti tertulis)                         |
| D5  | §8.4.4 dirty guard dan §8.4.5 validasi blur: implementasi atau amandemen?                                                                                     | Implementasi untuk form berdraft, amandemen untuk blur            | F10 **CLOSED 2026-09-21** (dipakai owner: guard bersama di enam form berdraft, §8.4.5 diamandemen dengan alasan tertulis)                      |
| D6  | Playground: sediakan `PANEL_PLAYGROUND_KEY` (gateway key) untuk pass live?                                                                                    | Ya, satu key khusus playground                                    | F3, F12; F3 tidak memakainya: pass live mencetak key sendiri lewat `POST /api/v1/gateway-keys`, jadi tidak ada key owner yang perlu diserahkan |

## 17. Urutan pengerjaan yang disarankan

1. F11 (dokumen, paling murah, dan membuat sisa daftar ini jujur).
2. F5 lalu F9 (dua cacat kecil di satu layar U0/U1 yang sama, sekaligus menutup §8.4.2).
3. F6 (mode baris berulang; kliennya sudah ada).
4. F7 (test render empat layar; menutup area yang F5/F6 sentuh).
5. F8 dan F10 setelah D4 dan D5 dijawab (keduanya bisa jadi satu perubahan lintas layar).
   F8 **CLOSED 2026-09-21**; F10 **CLOSED 2026-09-21** (D5 dijawab owner: guard bersama plus
   amandemen §8.4.5).
6. F4, lalu F1, lalu F2, lalu F3 (empat layar; F4 paling kecil karena layarnya sudah ada,
   F3 terakhir karena butuh D6 dan menyentuh jalur kredensial). F1 **CLOSED 2026-09-21**, F2
   **CLOSED 2026-09-21**, F3 **CLOSED 2026-09-21**, F4 **CLOSED 2026-09-21**.
7. F12 sebagai penutup: satu pass live yang mencatat U0, U1, dan dua kriteria U2.
8. F13 (dokumen, paling murah): samakan klaim versi runtime di `app-ui/README.md` dengan versi
   yang terukur, atau buang nomornya dan sebut syarat `engines`.

Setiap nomor dikerjakan sebagai satu commit sendiri, dengan analysis plus compliance
self-check di badan commit (TDD §3), gate panel lengkap (`bun run test`, `bun run check`,
`bun run lint`, `bun run lint:ts`, `bun run build`), dan `wc -l` dijalankan atas setiap file
yang disentuh sebelum staging. Bila sebuah nomor ternyata membutuhkan perubahan `app-serv`,
pekerjaan itu berhenti dan dicatat sebagai permintaan, bukan dikerjakan dari draft ini.

## 18. Catatan lingkup

- Dua layar SPEC-UI §2.1 berada di luar daftar 14 item ini dan tidak diperiksa mendalam:
  `/login` (item 14 KEEP, fase U0) dan `/logs` (item 11 KEEP, Requests tab). Keduanya
  terbangun; `/login` ikut masuk F7 karena tidak punya test render, dan `/logs` tidak punya
  gap yang ditemukan. Bila owner ingin keduanya diaudit penuh, itu nomor terpisah.
- Seluruh temuan di atas diperiksa dengan membaca sumber, bukan dengan menjalankan panel di
  browser. Panel client-rendered (`ssr = false`), jadi bukti render hanya bisa datang dari
  test jsdom atau browser; F12 adalah nomor yang menutup celah itu.
- `app-serv/` tidak disentuh. Tiga route P4 dibaca sebagai fakta yang sudah ada, dan setiap
  kebutuhan yang jatuh di sana (D1, D2, D3) dicatat sebagai permintaan.

## 19. F13 (LOW): Klaim versi Bun di sembilan belas baris README tidak dapat direproduksi

**Fakta.** Sembilan belas baris di `app-ui/README.md` menyatakan `1.3.14` sebagai versi yang
dipakai: enam belas baris pembuka pass, satu baris tabel Requirements ("Bun | 1.3.14"), satu baris
tabel di bagian Node adapter yang menyatakan boot banner mencetak versi itu, dan satu baris tabel di
bagian verifikasi `bun run start`. Tiga baris lain menyebut angka itu justru untuk menunjuk temuan ini
(baris pembuka pass ke-18, ke-19, dan ke-20). Klaim itu sudah ada sejak commit U0 `abf9de5`
(`git log -S '1.3.14' -- app-ui/README.md`), jadi ia diwariskan dari pass ke pass, bukan diukur
ulang.

Diukur 2026-09-22 di mesin yang sama, apa adanya:

```bash
$ command -v bun
/home/rusmanadodi/.local/bin/bun        # mtime 2026-09-16 21:15
$ bun --version
1.3.0
$ ~/.bun/bin/bun --version
1.3.11
$ /usr/local/bin/bun --version
exit 132 (SIGILL; build AVX2 di CPU tanpa AVX2, persis kasus yang README ini catat di
bagian "Bun on a CPU without AVX2", jadi bukan binary rusak melainkan build yang salah)
```

Tidak ada binary 1.3.14 di mesin ini, dan banner itu sendiri membaca runtime alih-alih angka
tetap (`app-ui/scripts/boot-log.ts:15` mencetak `${Bun.version}`), jadi banner tidak mungkin
mencetak 1.3.14 di sini. Yang tetap benar: `app-ui/package.json` mensyaratkan `bun >= 1.3.0`,
dan binary yang dipakai memenuhinya, jadi syarat panelnya tidak salah; yang cacat adalah angka
yang tercatat, bukan jalannya panel.

Baris pass ke-18 ("Custom provider surface"), ke-19 ("Panel runtime config"), dan ke-20 ("Usage live
surface"), ketiganya 2026-09-22, sudah memakai versi terukur, `1.3.0`, dan menunjuk temuan ini. Enam
belas baris pass sebelumnya beserta baris Requirements dan Node adapter dibiarkan apa adanya: versi yang
benar-benar dipakai saat itu tidak dapat dibuktikan dari sini, jadi menggantinya dengan angka
hari ini akan menulis klaim baru yang sama tidak terbuktinya.

**Risiko.** Pembaca yang mengukur ulang di mesin ini tidak menemukan 1.3.14, lalu punya alasan
untuk tidak mempercayai seluruh catatan pass. Baris Node adapter lebih tajam lagi: ia klaim
verifikasi ("Verified by the boot banner printing ...") yang tidak bisa terjadi di mesin ini,
bentuk klaim yang R-38 larang.

**Kerja yang diusulkan.** Satu pass dokumen: ganti angka di sembilan belas baris itu dengan versi
terukur beserta perintahnya, atau buang nomor versinya dan sebut syarat `engines` yang memang
mengikat. Tidak ada perubahan kode.

**Kriteria selesai.** Tidak ada lagi baris `app-ui/README.md` yang menyebut versi runtime yang
tidak dapat direproduksi di mesin ini; tiap baris menyebut perintah pengukurnya.
