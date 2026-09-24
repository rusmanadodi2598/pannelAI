# 030-CREDENTIAL-ROTATION-POLICY.md: saklar rotasi kredensial per provider, 1:1 dengan reference 9Router

Permintaan owner: mekanisme failover kredensial sudah 1:1, tetapi saklarnya baru ada di combo.
Reference punya saklar per provider juga, jadi `fill-first` atau `round-robin` harus mengatur SETIAP
request lewat sebuah provider, bukan hanya member combo. Dokumen ini mencatat kedua setengah
(`app-serv` dan `app-ui`), kontraknya, dan batas yang masih terbuka. Bukan kontrak; kontrak tetap
`docs/SPEC-API/001-SPEC-API.md` (wire) dan `docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel).

| | |
|---|---|
| **Status** | **CLOSED 2026-09-24.** Kedua setengah mendarat di commit lokal `c33b468` (app-serv, panel, spec, kontrak; 36 berkas); draft ini di commit berikutnya; belum di-push |
| **Mechanism** | DURING & AFTER (antislop) |
| **Scope** | `app-serv/` (domain, schema, service, data plane, config, wiring) dan `app-ui/` (skema, klien API, dua komponen, test), plus amandemen SPEC-API §7.5/§7.14 dan SPEC-UI §6.3/§6.13 |
| **Permintaan owner** | "Nah, berarti tinggal satu fix lagi, benar faileover untuk round robin per provider, supaya mekanisme nya bisa 1:1, saklar ini bukan hanya aktif di mode `combo` aja. Karena di REFERENCE ada saklar per provider juga." |
| **Reference** | `decolua/9router`, checkout `/home/rusmanadodi/apps/9router`, `39e36d3d` = `v0.5.86` |
| **Kaitan** | SPEC-API §7.5, §7.14; SPEC-UI §6.3, §6.13; draft 028 (semantik failover kredensial, deviasi yang dipertahankan); draft 019 §11 |
| **Tanggal** | 2026-09-24 |

---

## 1. Ringkasan

Sebelum pass ini, satu-satunya saklar rotasi yang benar-benar sampai ke router adalah milik combo
(`routing.combo_strategy`). Saklar kredensial tidak ada: `routing.sticky_limit` disimpan dan
divalidasi, tetapi tidak ada pembaca; limit yang dipakai selector datang dari knob lingkungan
`DATA_PLANE_STICKY_LIMIT`, yang bahkan tidak pernah dideklarasikan di `.env.example`. Mode
`fill-first` tidak ada sama sekali, sehingga setiap walk kredensial berotasi.

Yang mendarat adalah satu kebijakan yang dibaca per seleksi:

- `routing.fallback_strategy` (`fill-first` | `round-robin`, default `fill-first`) adalah default
  global, dan `routing.provider_strategies` memetakan id provider ke strateginya sendiri plus
  `sticky_limit` opsional (SPEC-API §7.14). Peta itu ditulis UTUH setiap kali, karena ia satu nilai
  settings; entri yang tidak ada berarti provider itu mengikuti default global.
- `fill-first` menyajikan urutan prioritas dan memulai setiap request dari endpoint pertama yang
  bisa dipakai, memilih key sehat pertama menurut prioritas. `round-robin` memajukan kursor endpoint
  bersama, menahan satu endpoint selama `sticky_limit` request, dan merotasi key dengan aturan
  least-recently-used.
- Kebijakan yang tidak bisa dibaca turun ke `fill-first`, degradasi yang sama dengan kursor Redis
  yang tidak menjawab: rotasi adalah optimasi, bukan input kebenaran.

Panel mendapat blok **Connections** milik reference di kedua bentuk halaman provider (registry dan
node custom) plus pilihan yang sama di tab Routing untuk default global.

## 2. Reference 1:1

Tiga sitasi diukur ulang dengan `grep -n` pada `39e36d3d` sebelum ditulis:

- `src/sse/services/auth.js:139`: `const strategy = providerOverride.fallbackStrategy ||
  settings.fallbackStrategy || "fill-first";` dan baris berikutnya
  `const stickyLimit = providerOverride.stickyRoundRobinLimit || settings.stickyRoundRobinLimit || 3;`
  Override provider menang atas default global; default terakhir `fill-first` dan sticky `3`.
- `src/app/(dashboard)/dashboard/providers/components/ConnectionsCard.js:405-427`: blok kendalinya.
  Toggle `Round Robin` (`:407-414`) mengirim `strategy = enabled ? "round-robin" : null`; input
  `Sticky:` (`:417-423`) hanya render saat strateginya `round-robin`; `saveStrategy` (`:330-342`)
  membaca dokumen settings lebih dulu, menyusun entri override, dan menulis PETA UTUH lewat
  `PATCH /api/settings`, menghapus entri saat saklar mati. Nilai sticky yang kosong/tidak bisa
  diurai dipaksa `3` oleh `Number(stickyLimit) || 3`.
- `src/app/(dashboard)/dashboard/profile/page.js:1449-1485`: pasangan kontrol yang sama untuk
  default global.

Semantik `round-robin` reference (`auth.js:146-175`): pertahankan koneksi saat ini selama
`consecutiveUseCount < stickyLimit`, lalu pilih yang paling lama tidak dipakai. Di `app-serv` satu
endpoint adalah satu koneksi (deviasi yang sudah dicatat draft 028: reference 1 koneksi = 1 key,
`app-serv` 1 endpoint boleh N key), jadi "satu akun melayani N request" setara: kursor endpoint
ditahan selama `sticky_limit` panggilan, key di dalamnya dipilih LRU.

## 3. Yang diukur sebelum perubahan

Diukur pada tree yang belum membawa kedua setengah:

- Tidak ada mode `fill-first`; `Selector` selalu memajukan kursor (`selection.go` HEAD).
- `routing.sticky_limit` disimpan, divalidasi `validateRouting`, dan dirender panel, tetapi nol
  pembaca di jalur data.
- Limit yang benar-benar dipakai datang dari `DATA_PLANE_STICKY_LIMIT` (`config.go` HEAD), knob yang
  tidak ada di `.env.example`; satu-satunya saklar yang sampai ke router adalah `combo_strategy`.
- Panel hanya punya `combo_strategy`/`combo_sticky_limit`/`sticky_limit` di tab Routing dan tidak
  punya kontrol rotasi di halaman provider.

## 4. Implementasi `app-serv`

| Berkas | Isi |
|---|---|
| `internal/domain/settings_routing.go` (baru, 220 baris) | `CredentialRotation` (`fill-first`/`round-robin`), `ProviderStrategy`, `RotationPolicy`, `RoutingSettings.RotationFor(providerID)`, `Normalized()` (mengisi key yang tidak ada di baris lama), `validateRouting` (entri kosong ditolak dengan nama) |
| `internal/schema/settings_routing_patch.go` (baru, 108) | Bentuk wire patch/response untuk kedua key baru; tag `dive` pada peta override |
| `internal/schema/settings_patch.go` (220) | `validateProviderStrategies`, `SettingsResponseFrom` memetakan key baru |
| `internal/service/settings.go` (181) | `RotationPolicy(ctx, providerID)` (`:90`) sebagai seam; read menerapkan `Normalized()` |
| `internal/dataplane/selection.go` (215) | Interface `CredentialStrategy` + `rotationPolicy()` yang menurunkan nil/error ke `fill-first`; `SelectorDeps.Strategies` |
| `internal/dataplane/selection_candidates.go` (113) | `SelectNext` membaca kebijakan per seleksi: kursor hanya di bawah `UsesRotation()`, `pickKey` memilih `FirstKeySkipping` (fill-first) atau `NextKeySkipping` (LRU) |
| `internal/config/config.go` (176) | `DATA_PLANE_STICKY_LIMIT` dihapus |
| `cmd/app-serv/dataplane_wiring.go` (217) | Settings service dirakit sebelum selector dan dipasang sebagai `Strategies` |

Test permanen: `settings_routing_test.go` (217), `settings_routing_patch_test.go` (157),
`settings_rotation_test.go` (119), `selection_policy_test.go` (193) + `selection_key_policy_test.go`
(84, hasil split dari satu berkas 249 baris).

Mutasi untuk membuktikan test-nya diskriminatif (semua mutasi dikembalikan; suite hijau sesudahnya):
domain 4/4 tertangkap (override diabaikan; limit tersimpan < 1 berhenti mewarisi; `Normalized`
berhenti mengisi; entri kosong diterima), schema 3/3 (tag `dive` dilepas; entri kosong diterima;
mapper mengembalikan nil), service 3/3 (provider salah; read berhenti menormalkan; error ditelan).

## 5. Implementasi `app-ui`

| Berkas | Isi |
|---|---|
| `src/lib/schemas/settings.ts` (197) | `CREDENTIAL_ROTATIONS` + labelnya, `schemaProviderStrategy`, `fallback_strategy` pada form dan dokumen, `provider_strategies` sebagai record, `schemaProviderStrategiesPatch` |
| `src/lib/api/settings.ts` (114) | `patchProviderStrategy(providerID, next)`: baca ulang peta, gabungkan satu entri, tulis utuh; `next = null` menghapus entri |
| `src/lib/components/SettingsRoutingTab.svelte` (183) | Select `Credential rotation` untuk default global; form tab memproyeksikan empat key miliknya sendiri sebelum parse |
| `src/lib/components/ProviderConnectionsSection.svelte` (212) | Saklar `Round Robin` + kotak `Sticky` di blok Connections, di kedua bentuk halaman provider; laporan menyebut override dan default yang diwarisi |

Test: `tests/components/provider-rotation-switch.test.ts` (190 baris, 8 kasus) plus pembaruan
`dirty-forms.test.ts`, `settings.test.ts`, `settings-forms.test.ts`, `settings-document.ts`, dan
`model-stub.ts` (rute settings dengan semantik apply-on-write).

Dua cacat panel ditemukan dan diperbaiki di pass ini:

1. Form ketat tab Routing menolak dokumen settings bersama begitu ia membawa `provider_strategies`
   (key asing), ditemukan test dirty-form yang sudah ada; tab sekarang memproyeksikan empat key
   miliknya.
2. Checkbox berubah sebelum jawaban PATCH tiba, sehingga penolakan gateway meninggalkan saklar yang
   mengklaim perubahan yang tidak tersimpan; saklar sekarang menyimpan state sendiri dan
   mengembalikannya saat jawabannya penolakan.

Satu selisih dari reference ditemukan saat menulis catatan ini dan diperbaiki sebelum freeze:
`writeRotation` menggabungkan perubahan ke peta yang dibaca saat halaman dimuat, sehingga entri
provider lain yang berubah di antara baca dan tulis akan terhapus. Reference membaca ulang dokumen
sebelum menulis (`ConnectionsCard.js:330-342`); panel sekarang melakukan hal yang sama lewat
`patchProviderStrategy`, dan satu kasus test baru menahan aturan itu (peta berubah setelah load,
entri baru harus selamat).

## 6. Kontrak dan spec

- SPEC-API §7.5/§7.14 diamandemen lebih dulu; baris changelog-nya menunjuk dokumen ini.
- `docs/CONTRACT/001-CONTRACT-API-V1.yaml` bertambah 54 baris (`fallback_strategy` dan
  `provider_strategies` pada patch/response, plus `ProviderStrategyPatch`/`ProviderStrategyResponse`)
  dan `internal/handler/openapi.json` diregenerasi (70 baris); `contract-openapi.sh` PASS.
- SPEC-UI §6.3 mendapat bullet **Connections (landed)** dan §6.13 baris Routing menyebut
  `routing.fallback_strategy` beserta override per provider; tabelnya di-pad ulang ke lebar yang sama.

## 7. Deviasi dan batas yang dicatat

1. **Saklar melaporkan override, bukan kebijakan efektif.** Ini bacaan reference sendiri
   (`checked={providerStrategy === "round-robin"}`, `ConnectionsCard.js:407-408`): provider tanpa
   entri tampil mati meski default global `round-robin`. Kalimat di bawah saklar menyebut default
   yang diwarisi supaya keadaan itu bukan misteri.
2. **Lantai sticky.** Panel menolak nilai di bawah 1 dengan pesan; reference memaksa nilai kosong
   menjadi 3 secara diam (`Number(stickyLimit) || 3`). Di dokumen, limit tersimpan di bawah 1
   mewarisi limit global (rantai `||` reference), dan jalur tulis menolak menyimpannya.
3. **Limit global default 3**, sama dengan reference (`DefaultStickyLimit`, `settings_routing.go:39`).
4. **Rotasi combo tetap di Redis** dan tidak disentuh pass ini (deviasi draft 028).
5. **`DATA_PLANE_STICKY_LIMIT` dihapus.** Deployment yang menyetel knob itu kehilangan efeknya;
   knob itu tidak pernah dideklarasikan di `.env.example`, dan dokumen settings sekarang satu-satunya
   sumber.
6. **`engine_relay_tokensaver_test.go` tetap 229 baris** (sudah di atas ambang 220 sejak HEAD,
   disentuh pass ini, bukan ditumbuhkan) dan dilaporkan, bukan dipisah di sini.

## 8. Temuan

| # | Tingkat | Status | Isi |
|---|---|---|---|
| **F1** | MEDIUM | **OPEN** | Click-through browser untuk saklar ini belum dijalankan: gateway yang melayani `routing.provider_strategies` adalah setengah `app-serv` pass ini dan belum ter-deploy, jadi probe ke gateway yang hidup hanya akan menguji jalur penolakan. Interaksi ditutup 8 kasus jsdom; setengah wire ditutup test service/selector. Dijalankan saat gateway membawa setengah ini |
| **F2** | MEDIUM | **CLOSED di pass ini** | Peta override ditulis utuh dari salinan yang dibaca saat load, sehingga entri provider lain yang berubah di antara baca dan tulis terhapus. Diperbaiki dengan baca-ulang sebelum gabung (1:1 `ConnectionsCard.js:330-342`) plus satu kasus test |

## 9. Status pass ini

Keluaran pass ini kode Go + test, kode panel + test, amandemen dua spec, kontrak + artefaknya, dan
dokumen ini. Angka yang diukur:

- Panel, run bertarget: 17 berkas, 197 test PASS (8 di antaranya suite saklar baru).
- Panel, suite penuh: 2580 test PASS di 158 berkas (1053.84 s, hijau pada percobaan pertama; pass sebelumnya
  2570 di 157).
- Go: `go test -race ./...` PASS (15 paket ber-test, 2 tanpa test; suite integrasi SKIP karena
  `PANNELAI_TEST_POSTGRES_DSN` tidak diset). `go-lint.sh`, `go-headers.sh`, `contract-drift.sh`,
  `contract-openapi.sh`, `secrets.sh`, dan `panel-check.sh` PASS pada tree beku
  `9a5a611acbe432f97e5aad9debf21c2b` (1449 berkas); `scrypts/gates/all.sh` keluar 0.

### 9.1 Gerbang pass ini (antislop, DURING & AFTER)

Pass ini menyentuh UI (dua kontrol baru di layar yang sudah ada), jadi checklist 15 item antislop-ui
dijalankan bersama empat blok di bawah.

**Block 1: Hard Gate (17 item, semua harus "tidak")**

| Item | Jawaban | Bukti / alasan |
|---|---|---|
| R-02 em dash | **tidak** | `grep -cP '\x{2014}\|\x{2013}'` = 0 pada baris yang ditambahkan pass ini di kedua spec, README, dan dokumen ini |
| R-03 overflow mobile | **tidak** | Kedua kontrol memakai `min-h-11` dan membungkus (`flex-wrap`); tidak ada ukuran tetap yang baru. Tidak ada probe browser di pass ini (F1) |
| R-17 angka tanpa sumber | **tidak** | Setiap angka diikuti perintah/berkas sumbernya (§2-§4, §9) |
| R-18 testimoni fiktif | tidak berlaku | Tidak ada testimoni |
| R-23 aset tanpa instruksi | tidak berlaku | Tidak ada aset visual yang dibuat |
| R-24 navigasi mati | tidak berlaku | Tidak ada navigasi baru |
| R-25 kontras | **tidak** | Kedua kontrol memakai token teks/border yang sudah lulus kontras; tidak ada pasangan warna baru |
| R-26 kontrol mati | **tidak** | Saklar menulis lewat `patchProviderStrategy`; kotak Sticky menulis; select tab menulis. Ketiganya punya test |
| R-27 state UI | **tidak** | Error baca rotasi dinamai penyebabnya (`role="status"`), loading adalah kontrol disabled, dan pesan hasil menamai keadaannya |
| R-28 FAQ | tidak berlaku | Tidak ada FAQ |
| R-32 keyboard | **tidak** | Ketiga kontrol native (checkbox, number input, select) dengan label `for`/`id`; tidak ada handler keyboard kustom |
| R-33 patch via script | **ya, didiskalikan** | Lihat §9.2 |
| R-34 tema | **tidak** | Tidak ada warna literal baru; semuanya token yang sudah ada di kedua tema |
| R-35 dijalankan sebelum diserahkan | **sebagian, dicatat** | Yang dijalankan: svelte-check, run bertarget 17 berkas, suite penuh, seluruh gerbang repo. Click-through browser TIDAK dijalankan dan difile sebagai F1 dengan sebabnya |
| R-36 klaim fabrikasi | **tidak** | Setiap klaim menyitir berkas yang bisa dibuka; tidak ada klaim keamanan/compliance |
| R-37 arah desain | **tidak** | Kedua kontrol mengikuti blok reference dan token `DESIGN.md`; tidak ada arah baru |
| R-38 konten fabrikasi | **tidak** | Sitasi reference dari checkout `39e36d3d` yang di-`grep` ulang; sitasi `app-serv`/panel dari working tree; output test dan gerbang dikutip apa adanya |

**Block 2: Purpose-Gate (12 item)**

Seluruhnya **tidak berlaku** dengan satu alasan: pass ini tidak menambah gradien, ikon, tipografi,
latar bermotif, panah, badge, glassmorphism, shadow, glow, kartu, animasi, atau ilustrasi. Dua
kontrol baru memakai kontrol native dan token yang sudah ada. R-31 dijalankan pada tingkat isi:
setiap deviasi §7 menuliskan alasannya, dan setiap temuan §8 menuliskan sebabnya.

**Block 3: Liveliness (7 item)**

Tidak berlaku: tidak ada dial ENERGY/RHYTHM/MOTION, focal point, whitespace, accent, atau motif
identitas yang berubah. Saklar dan kotak sticky mewarisi ritme blok Connections yang sudah ada.

**Block 4: Craftsmanship & Quality Locks (14 item)**

| Item | Jawaban | Bukti / alasan |
|---|---|---|
| C-1 intentionality | **tidak** | Setiap keputusan berasal dari aturan reference yang disitir atau temuan terukur (§2-§5) |
| C-2 functional completeness | **tidak** | Ketiga kontrol punya jalur sukses, penolakan, dan (untuk baca) error; test menutup ketiganya |
| C-3 content-driven composition | **tidak** | Blok Connections adalah bentuk reference, bukan template; tidak ada section yang ditambah untuk mengisi ruang |
| C-4 resilience | **tidak** | Kebijakan tak terbaca turun ke `fill-first`; penolakan tulis mengembalikan saklar; baca-ulang sebelum gabung menutup clobber peta |
| C-5 evidence over claims | **tidak** | Tidak ada testimoni/statistik tanpa sumber |
| R-05 template layout | tidak berlaku | Tidak ada layout baru |
| R-11 radius | **tidak** | Memakai `--radius-sm` yang sudah ada |
| R-15 CTA | tidak berlaku | Tidak ada CTA |
| R-16 buzzwords | **tidak** | Copy baru: "Round Robin", "Sticky:", "Credentials follow the global default (…)", "This provider overrides the global default (…)", pesan hasil dan penolakan |
| R-20 identitas visual | tidak berlaku | Tidak ada visual baru |
| R-21 dark mode | **tidak** | Tidak ada warna literal; token mengikuti kedua tema |
| R-29 palet | **tidak** | Tidak ada warna baru |
| R-30 klon produk | tidak berlaku | Tidak ada visual baru |
| R-31 alasan tertulis | **tidak** (lulus) | Setiap deviasi §7 dan temuan §8 menulis alasan |

**Checklist 15 item antislop-ui**

| # | Item | Jawaban | Bukti / alasan |
|---|---|---|---|
| 1 | Palet dari `DESIGN.md`, bukan gradien default | ya | Tidak ada warna baru; kontrol memakai token border/surface/teks yang sudah ada |
| 2 | Accent hanya di momen kunci | ya | Tidak ada accent yang ditambah |
| 3 | Copy bebas emoji dekoratif | ya | Copy baru murni teks |
| 4 | Komposisi mengikuti dial RHYTHM | ya | Tidak ada komposisi baru; blok Connections sudah ada dan tidak berubah ritmenya |
| 5 | Bebas bentuk AI default (bento, terminal palsu, tiga kolom harga, stripe kiri) | ya | Tidak ada satupun |
| 6 | Tidak ada pill badge di atas H1 | ya | Tidak ada heading baru |
| 7 | Setiap elemen interaktif punya perilaku nyata | ya | Saklar, kotak sticky, dan select menulis; tiga kasus penolakan diuji |
| 8 | Motion mengikuti dial, tanpa loop | ya | Tidak ada motion baru |
| 9 | Glass/glow/shadow/radius pada dosisnya | ya | Tidak ada glass/glow/shadow baru; radius memakai token |
| 10 | Setiap dot/status light menandai keadaan nyata | ya | Tidak ada dot baru; pesan memakai `role="status"` untuk keadaan yang benar-benar terjadi |
| 11 | Layar app dibangun di sekitar keputusan user | ya | Saklar duduk di blok tempat koneksi provider dikelola, seperti reference |
| 12 | Setiap angka/baris nyata, tanpa metrik karangan | ya | Tidak ada angka baru di UI; kotak sticky menampilkan nilai efektif yang tersimpan |
| 13 | Field kosong tetap kosong atau placeholder jujur | ya | Kotak sticky menampilkan nilai efektif (milik provider atau global), bukan contoh |
| 14 | State kosong/loading/error menamai sebab dan aksi berikutnya | ya | Error baca menamai penyebabnya; loading adalah kontrol disabled; hasil tulis menamai keadaannya |
| 15 | Tahan di semua breakpoint/tema/state + keyboard | **belum diukur di browser** | Kontrol native berlabel dan `min-h-11`, tetapi probe 390 px/tema/keyboard adalah click-through yang difile sebagai F1, bukan diklaim lulus |

Item 15 adalah satu-satunya jawaban yang bukan "ya", dan ia menunjuk F1 yang sama. Tidak ada item
yang FAIL.

### 9.2 R-33: suntingan berskrip yang didiskalikan

- Satu `python3` heredoc mengganti rujukan draft `029-CREDENTIAL-ROTATION-POLICY.md` menjadi `030-`
  di SPEC-API (dengan assert jumlah penggantian), karena nomor 029 dipakai aktor lain.
- Satu `python3` heredoc mem-pad ulang tabel §6.13 SPEC-UI ke lebar karakter yang sama setelah baris
  Routing berubah.
- Satu `python3` heredoc menulis ulang bagian fixture `dirty-forms.test.ts` untuk dokumen form baru.
- Satu `cat >> … << 'EOF'` menambahkan baris changelog SPEC-API (dijalankan di paruh awal sesi).
- `git apply --cached` dipakai untuk men-stage hanya hunk kontrak milik pass ini dari patch yang
  dibangun atas HEAD, karena berkas yang sama sedang disunting aktor lain.

Setiap hasil dibaca ulang; `gofmt`/`go vet`/`golangci-lint`/test membuktikan hasilnya, dan tidak ada
tabel dokumen yang disunting lewat skrip tanpa dibaca ulang.
