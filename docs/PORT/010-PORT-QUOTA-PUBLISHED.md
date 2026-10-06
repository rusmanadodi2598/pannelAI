# 010-PORT-QUOTA-PUBLISHED.md: Kuota terbit tiap provider, sampai ke layar, tanpa fan-out

Pass `app-serv` + `app-ui` untuk layar **Quota Tracker**. Permintaan owner satu kalimat:
**quota tracker belum bekerja semestinya; di REFERENCE tiap provider tampil dengan baik dan UI-nya
jelas.** Reference: `/home/rusmanadodi/apps/9router` (`open-sse/services/usage/`, 3.872 baris JS di 21
berkas; kartu per koneksi di `dashboard/usage/components/ProviderLimits/`).

| | |
| --- | --- |
| **Status** | **SELESAI dan terukur.** Semua gerbang hijau di tree terakhir, diukur solo dan berurutan: `go-lint` rc=0 (golangci 0 issues), `-race` seluruh modul (16 paket `ok` + `tools/openapi-gen` + `migrations` di tag integration), `router` sendirian `ok 307,547s`, suite panel penuh **179 berkas / 2.933 test hijau**, `check`/`lint`/`lint:ts`/`build` rc=0, kontrak + drift + secrets rc=0. Empat cacat awal tertutup dan diukur di layar hidup; 16 temuan (§3) tercatat, enam di antaranya muncul karena diukur, bukan karena direncanakan (F8 stempel tahun 1, F10 blok provider palsu, F11 kunci alamat tanpa test pemindah, F13 budget tak bisa diatur, F14 cooldown diabaikan, F16 split kebijakan gagal hilang). Semua gerbang Go + kontrak + secrets hijau di tree final; §10 adalah audit penutupnya |
| **Mechanism** | DURING & AFTER (antislop) |
| **Scope** | `app-serv/internal/service/quotafetch/**` (14 keluarga baru), `internal/service/quota_usage*.go` + `quota_published_cache.go` + worker, `internal/repository{,/postgres}/published_quota*`, `internal/domain/quota_published.go`, `migrations/000013_*`, `internal/{schema,handler}` kuota, `docs/CONTRACT`, `app-ui` halaman `/quota` |
| **Permintaan owner** | (1) Tiap provider menampilkan kuotanya. (2) UI sejelas reference. (3) Ralat saat planning: fetch model = **worker + tabel cache**, bukan fan-out per baca; aturan no-N+1 **tetap mengikat**. (4) Kartu dipimpin angka provider, hitungan gateway jadi satu baris ringkas. |
| **Kaitan** | SPEC-API §7.12 (diamendemen di pass ini), SPEC-UI §6.6 (diamendemen), PORT 004 §7.4 (residunya ditutup di sini), PORT 005 (bentuk kartu), PORT 006 (paging grup provider), AGENTS.md §1.1/§1.2/§1.4/§1.6/§1.7/§1.9/§2.1, `docs/RULLES/TDD.md` |
| **Tanggal** | 2026-10-02 |

## 1. Ringkasan: keluhan satu kalimat, terukur jadi empat cacat

| Yang owner bilang | Yang terukur di pohon ini |
| --- | --- |
| "belum bekerja semestinya" | **Benar, dan bukan satu sebab.** 14 dari 18 provider yang registry-nya menandai `usage: true` tidak punya fetcher sama sekali; jawabannya kalimat `"Usage API not implemented for X"`. Hanya qoder, qoder-cn, codebuddy-cn, codebuddy-intl yang membawa angka |
| "tiap provider bisa ditampilkan" | **Dua cacat wiring, bukan cuma jumlah fetcher.** (a) Service hanya meneruskan `transport.usage.url`, padahal registry menulis alamat kuota di `quota_url`, `quota_api_url`, `urls[]`, `oauth_url`, `org_url`, `user_url`, jadi keluarga yang membaca key lain dipanggil dengan alamat kosong, tanpa suara. (b) `fetchVercel` terdaftar di bawah id `vercel-ai-gateway` yang tidak ada di registry, sehingga tak bisa dijangkau dari produksi |
| "UI yang jelas" | **Angka provider tersembunyi di balik tombol per kartu, dan baris yang tampil tidak punya langit-langit.** PORT 004 §7.2 mengukur layar hidup: **0 bar progres terender**, karena `quota_windows` gateway ini tidak memuat `limit`. Jadi halaman terbaca sebagai counter, bukan kuota |

### 1.2 Verifikasi hidup pada deployment ini (2026-10-02, build baru di port cadangan `:9091`)

`GET /api/v1/quotas?page=1&per_page=5` setelah worker menyelesaikan satu sweep:

| provider | akun di halaman | baris provider | keadaan |
| --- | --- | --- | --- |
| `codebuddy-intl` | 1 | **9** (`Bonus Pack 1..4` dipakai/total 250/250, 0/100, 30/30, 30/30 …) | angka provider nyata |
| `qoder` | 3 | 0 | kalimat provider: *"Qoder reports this account's quota as exceeded."* |
| `opencode-zen` | 1 | 0 | kalimat provider: *"OpenCode Zen quota API error (404)"* |
| `openai-compatible-0386…` (custom node) | 3 | - | **tidak** muncul, provider ini tidak menerbitkan kuota |
| lane virtual (`''`, `virtual:opencode`) | 2 | - | **tidak** muncul, bukan provider |

Tiga hal yang hanya bisa dilihat dengan menjalankan ini, semuanya mengubah kode:

1. **`opencode-zen` kini muncul padahal tidak punya satu baris hitungan pun.** Sebelum bagian ini,
   kartu dibangun dari `quota_windows`, jadi akun yang belum pernah dilewati traffic tidak punya kartu
   sama sekali. Provider-nya sudah di-poll dan sudah menjawab, layar tetap kosong. Halaman kini dipilih
   oleh **akun** (`PageAccountsByProvider`), bukan oleh counter, dan kedua bacaan (window dan akun)
   memakai sumber grup yang sama sehingga satu nomor halaman tidak pernah berarti dua set provider.
2. **Kalimat `404` tidak lagi membawa HTML.** Provider yang memindahkan endpoint menjawab dengan
   `<!DOCTYPE html>…`; `softFailure` dulu mengutip 200 byte body apa adanya, dan worker **menyimpan**
   kalimat itu. Kini markup dibuang dan alasan plain-text tetap lewat. Test mengunci kedua arah.
   `opencode-zen` memang mati di upstream: `…/zen/v1/usage` menjawab 404 seperti rute fiktif, sementara
   `…/zen/go/v1/usage` menjawab 401. Port ini setia ke reference; URL reference-nya yang sudah basi.
3. **"belum di-poll" tidak boleh ditujukan ke provider yang tidak menerbitkan kuota.** Entri pertama
   untuk akun custom-node muncul dengan `never_polled: true`, yang menjanjikan poll yang tidak akan
   pernah datang. Akun kini disaring pada `Features.Usage`; kartu untuk provider seperti itu cukup
   baris hitungannya. Perbaikan ini baru separuh jalan di bacaan payload: backend sudah benar, kartu
   tetap merender blok dengan kalimat "Not polled yet" untuk akun yang tak seorang pun akan tanyai.
   Sisanya ditutup kemudian, saat layar benar-benar dibuka: lihat F10 di §3.10 dan angka DOM di §7.

## 2. Keputusan pass ini

| # | Pertanyaan | Arah | Dipakai oleh |
| --- | --- | --- | --- |
| D1 | Nama keluarga dari mana? | **`endpoint.ProviderID()` adalah kunci keluarga.** Tidak ada kolom `quota_family`: id registry (`claude`, `qoder-cn`, `opencode-zen`, …) sudah sama dengan kunci peta fetcher, sehingga keputusan PORT 004 F6/D6 "tunggu pemilik" ternyata tidak diperlukan | seluruh backend |
| D2 | Blok `usage` registry sampai ke fetcher bagaimana? | **Satu struktur cermin, satu mapper.** `quotafetch.UsageEndpoints` dideklarasikan di dalam package (package service tidak boleh di-import config ke atas, dan fetcher tidak perlu mengenal registry); `usageEndpoints(registry.UsageConfig)` memetakan seluruh blok sekali. Test mengunci tiap key terisi + test lebar-membaca (`reflect.NumField`) menangkap key baru yang tidak dipetakan | F2 di bawah |
| D3 | Fetcher yang tidak bisa dijangkau? | **`vercel.go` dihapus**, tidak ditambah entri registry. Menambah provider ke KEEP-set milik owner adalah keputusan katalog (dan akan menabrak `embedded_test.go` yang mengunci jumlah 34), sementara fetcher tanpa id adalah kode mati yang hanya test yang jalankan. Port-nya hidup kembali di riwayat git begitu entri registry dibuat dengan sengaja | F3 |
| D4 | Bagaimana layar mendapat angka provider tanpa fan-out? | **Worker menulis, layar membaca satu query.** `quota_published_state` + `quota_published_window` (migrasi 000013) diisi `QuotaPublishedWorker`; `GET /api/v1/quotas` membawa `published[]` untuk endpoint pada halaman itu, dari **satu** `WHERE endpoint_id = ANY($1)`. Aturan no-N+1 owner tetap berlaku; yang berubah adalah siapa yang menghubungi provider, bukan berapa banyak panggilan per baca | F4 |
| D5 | Kenapa tabel baru, bukan `quota_windows`? | `quota_windows."window"` ter-CHECK pada empat jenis milik gateway dan satuannya integer tanpa nama; label provider adalah string penulis bebas ("Claude & GPT (Weekly)") dan saldonya uang. Dipaksa masuk ke sana = saldo kredit ditampilkan sebagai persentase dari sesuatu yang bukan kredit | 000013 |
| D6 | Persentase disimpan? | **Tidak.** Ia aturan tampilan dan layar yang memilikinya; menyimpannya membuat perubahan pembulatan menjadi backfill | 000013 |
| D7 | Kartu dipimpin siapa? | **Angka provider duluan**, hitungan gateway jadi satu baris ringkas per koneksi. `unit`, `unlimited`, `is_credit_balance` ikut lewat ke wire supaya tiga keadaan "bukan persen dari total" tidak diseragamkan | layar |
| D8 | `?force=1`? | Satu kartu, satu akun, satu panggilan. Seam yang sama yang reference pakai untuk refresh per kartu. Nilai apa pun selain `"1"` tetap baca cache, supaya bookmark basi atau proxy yang menulis ulang tidak mengubah baca layar menjadi traffic provider | handler + kontrak |

### 1.1 Bukti hidup yang diukur sebelum touch (PostgreSQL `pannelai`, 2026-10-02)

`SELECT provider_id, auth_type, status, count(*) FROM upstream_endpoints GROUP BY 1,2,3`:

| provider_id | auth_type | status | akun | Sebelum pass ini | Sesudah |
| --- | --- | --- | --- | --- | --- |
| `qoder` | api_key | active | 2 | angka (fetcher ada) | angka |
| `qoder` | oauth | active | 1 | angka | angka |
| `codebuddy-intl` | oauth | active | 1 | angka | angka |
| `opencode-zen` | api_key | active | 1 | **`"Usage API not implemented for opencode-zen"`** | **angka provider** |
| `openai-compatible-0386BKG9…` | api_key | active | 3 | tanpa kuota (custom node, `usage` tidak dideklarasikan) | tetap tanpa kuota, dan itu jawaban yang benar |

Jadi keluhan owner punya satu kasus yang bisa ditunjuk di deployment ini sendiri: satu akun
`opencode-zen` yang kartunya hanya bisa membaca "not implemented", persis bentuk yang membuat layar
terasa tidak bekerja. Lima dari delapan koneksi kini punya jawaban provider; tiga sisanya adalah node
kustom yang memang tidak menerbitkan kuota.

## 3. Temuan

### 3.1 F1 (HIGH): 14 keluarga ber-`usage: true` tanpa fetcher
**Ditutup.** Peta keluarga kini 18/18: `claude`, `commandcode`, `deepseek`, `gemini-cli`, `antigravity`,
`github`, `glm`, `grok-cli`, `groq`, `kimi`, `minimax`, `opencode-zen`, `opencode-go`, `zed`, plus
qoder pair dan codebuddy pair. Keluarga yang berbagi bentuk wire berbagi berkas
(`opencode.go` dua wilayah, `google.go` + `google_quota.go` + `google_account.go` untuk dua keluarga
google, `codebuddy.go` dua wilayah).

### 3.2 F2 (HIGH): alamat kuota declared tidak pernah sampai ke fetcher
**Ditutup lewat D2.** Sebelum ini, `quota_usage.go:132,154` hanya meneruskan `entry.Transport.Usage.URL`.
Test yang mengikat: `quota_usage_endpoints_test.go` men-setting tiap `UsageConfig` ke nilai yang saling
berbeda lalu menuntut nilainya kembali utuh, plus test lebar yang gagal jika registry menambah key tanpa
mirror. Tiap keluarga juga punya test stub yang mengunci PATH yang benar-benar diminta. Itulah regresi
yang menangkap "keluarga memanggil alamat kosong" untuk tiap keluarga baru, bukan hanya yang sudah ada.

### 3.3 F3 (MEDIUM): fetcher mati yang tak bisa dijangkau produksi
**Ditutup dengan penghapusan (D3).** Konsekuensinya test dispatcher lama yang mengharapkan rute
`vercel-ai-gateway` ikut dibuang.

### 3.4 F4 (HIGH): angka provider tidak pernah tampil tanpa klik
**Ditutup lewat D4 + D7.** `QuotaPublished` berhenti memanggil route per kartu saat render; ia menerima
entry dari halaman. Tombol "Ask provider" digantikan kontrol refresh per koneksi (`?force=1`).

### 3.5 F5 (HIGH): provider yang menolak kredensial tidak bisa dibedakan dari provider tanpa kuota
Keadaan ini kini nyata di layar: kalimat lunak (`message`) dirender muted, refusal keras (provider tak
dikenal, `features.usage` false, kunci di bawah endpoint token-only, akun tanpa kredensial) tetap non-2xx.
Perilaku ini dipertahankan per keluarga, tidak diseragamkan. Reference memang membedakan keluarga yang
`throw` dan yang mengembalikan pesan.

### 3.6 F6 (DITUTUP di pass ini): `ProviderSpecificData` kini terisi untuk akun OAuth

**Catatan kejujuran:** klaim penutup ini sempat salah. Fungsi pemetanya ada, punya test, dan **tidak
punya pemanggil produksi**. `staticcheck -tags=integration` menemukannya sebagai `U1000 func
publishedAccountData is unused` di gerbang final. Jadi selama itu keluarga cloudcode tetap membayar
satu `loadCodeAssist` per poll dan grok-cli/zed tetap tanpa user id, persis biaya yang F6 bilang
dihindari. Yang salah bukan pemetaannya, melainkan bahwa "sudah wired" disimpulkan dari ada fungsi +
ada test, bukan dari ada panggilan.

Perbaikannya sekarang di jalur, dan dijaga oleh test yang salahnya terbukti:
`TestQuotaService_PublishedUsageCredential` meminta `PublishedUsage(force)` lewat fixture nyata dan
mengasserta `fetcher.creds.ProviderSpecificData`: `projectId`/`email`/`userId` untuk akun OAuth,
`email` baris account untuk akun kunci. Menghapus satu baris wiring membuatnya FAIL kedua kasus
(dibuktikan dengan melepas field-nya, lalu mengembalikannya); test pemeta yang lama tidak bisa
menangkapnya karena ia memanggil helper itu langsung, bukan lewat pembangun kredensial.

Sekalian dua file yang menyentuhnya dipisah agar tetap di bawah ambang §1.1: `quota_usage.go` 243 →
239 lewat helper `publishedCredentials` (dua cabang auth dulu menulis field yang sama dua kali), dan
penjadwalan seed pindah ke `quota_published_seed.go`.

Aslinya F6 menutup utang PORT 004: `domain.OAuthCredential` ternyata **sudah** menyimpan `ProjectID`,
`AccountID`, dan `AccountEmail`, jadi tidak perlu kolom baru seperti yang draft 004 perkirakan.

Yang membuktikan wiring-nya nyata: `grep` atas `QuotaPublished` menemukan hanya satu pemanggil
(`QuotaCardBody`), dan `grep` atas `logger.*` di seluruh berkas baru tidak menemukan satu pun field
bertema kredensial. Dua-duanya murah dan tidak bisa diganti oleh "sudah ada test-nya".

Dua aturan yang membuat pemetaan ini aman: `email` jatuh ke baris account ketika kredensial tidak
menyimpannya, dan akun yang tidak punya apa pun menghasilkan map **kosong**, bukan `"projectId": ""`
yang bisa dibaca provider sebagai nilai nyata yang blank.

Yang tetap tidak ada sumbernya: akun berauth-type kunci (memang tidak menyimpan project) dan
`principalId` Grok.

### 3.7 F7 (MEDIUM, residu): `github` registry berauth-type apikey, endpoint kuotanya membaca token akun
`registry.yaml` menulis `github` dengan `auth_type: apikey` dan tanpa `usageApikey: true`, sementara
reference menghubungi `copilot_internal/user` dengan token OAuth. Karena keputusan kredensial mengikuti
`auth_type`, akun kunci github ditolak sebelum panggilan, perilaku yang benar dan terbaca, tetapi
berarti kuota Copilot tidak akan pernah tampil untuk endpoint tipe kunci hari ini. Perlu keputusan owner:
apakah entri github berubah ke oauth, atau `usageApikey` benar-benar dibacakan. Tidak diubah di pass ini
karena entri registry adalah cermin reference, bukan tempat menebak.

### 3.8 F8 (DITUTUP di pass ini): jawaban lunak menampilkan "Asked 1 Januari tahun 1"

Terlihat saat payload hidup diukur, bukan saat kartu dirancang. Tiga akun `qoder` menjawab dengan kalimat
("Qoder reports this account's quota as exceeded.") dan `data` kosong; `fetched_at` baris itu tetap
`0001-01-01T00:00:00Z` karena `RecordAttempt` menyimpan kalimat tanpa menyentuh angka, dan kartu mencetak
stempel apa adanya. Fixture test memakai stempel 2026 sehingga cacat ini tidak terlihat di panel.

Aturannya bukan "sembunyikan untuk soft answer": `fetched_at` menanggali **angka**, dan angka bisa
bertahan melintasi kegagalan berikutnya (worker menyimpan last-good), jadi menanggalinya dengan waktu
kegagalan justru salah. Yang tidak boleh dicetak adalah placeholder. `publishedAskedStamp` kini
mengembalikan `null` untuk instant di atau sebelum epoch unix, dan kartu tidak mencetak stempel tanpa
nilai. Kontrak tidak berubah: kolomnya tetap wajib dan tetap RFC3339.

### 3.9 F9 (LOW, residu milik pass lain): satu test probe provider flaky di bawah beban

`go test -race ./...` pada tree final gagal di
`internal/service/provider_model_probe_budget_test.go:43` (`TestProviderModelTestService_AProbeThatRunsOutItsBudgetIsATimeout`).
Anggaran probe 20s dan stub ditunda `ProviderModelProbeTimeout + 20ms`; pada mesin dua core dengan tiga
suite race berjalan bersamaan, `select` bisa memilih hasil stub yang baru tiba pada 20,02s sebelum timer
context sempat melayani, sehingga probe melapor `ok`. Test yang sama lolos sendirian (21,087s). Bukan
regresi kuota. Test ini milik pass probe provider dan sudah di main. Yang perlu dilakukan padanya:
lebarkan margin (mis. `+2s`) agar tidak lagi mengukur jitter scheduler; tidak diubah di pass ini karena
di luar lingkup.

### 3.10 F10 (DITUTUP di pass ini): kartu menawarkan blok provider kepada provider yang tidak menerbitkan apa pun

Terlihat saat layar dibuka di browser, bukan saat fixture ditulis. Tiga koneksi `openai-compatible-*`
menampilkan blok provider lengkap dengan tombol "Ask … now" dan kalimat **"Not polled yet. X has not been
asked for this connection."**, padahal id itu tidak ada di registry, jadi tidak ada yang bisa ditanya dan
tidak akan pernah ada. SPEC-UI §6.6 yang ditulis di pass ini sendiri sudah menyatakan sebaliknya: akun di
balik provider tanpa kuota tidak diberi blok provider. Jadi implementasi menyangkal spesifikasinya sendiri,
dan kalimat "belum di-poll" itu adalah kebohongan yang saya tulis di pass ini (sebelumnya blok hanya
menampilkan tombol, tanpa klaim apa pun).

Aturannya sudah benar di backend dan tidak diubah: `PagePublished` mengirim satu entri untuk **setiap**
akun yang bisa ditanya (termasuk yang ditandai `never_polled`) sehingga *kehadiran* entri kini adalah
cara kartu tahu providernya memang menerbitkan kuota. `QuotaCardBody` hanya merender blok ketika entri itu
ada; `QuotaPublished.usage` menjadi wajib (tidak ada lagi `null` dari pemanggil), dan cabang `null` yang
tersisa hanyalah press milik kartu itu sendiri yang masih di jalan.

Satu kebohongan kecil ikut tertutup di jalur yang sama: selama press berjalan `override.usage` bernilai
null, jadi kartu sempat menampilkan "Not polled yet" di bawah status "Asking the provider…". Baris itu kini
tidak muncul selama press: status di atasnya sudah mengatakan apa yang sedang terjadi.

### 3.11 F11 (DITUTUP di pass ini): kunci alamat yang dibaca produksi tapi tidak pernah dipindah oleh test

Cacat #2 adalah soal **kunci alamat**, jadi ukuran penutupnya bukan "fetcher-nya jalan" melainkan
"kalau registry menulis kunci lain, panggilan yang keluar ikut pindah". Diukur ke pohon:
`UsageEndpoints` punya 17 field, 11 dibaca kode produksi, dan hanya 6 yang pernah **dinyatakan bukan
kosong** oleh test keluarga: `oauth_url` milik claude (kunci yang justru disebut lebih dulu di
diagnosis pass ini), `settings_url`, `user_url` milik grok-cli, dan `load_code_assist_url` milik
gemini-cli semuanya dibaca tapi tidak pernah dipindah. Test yang ada mengunci alamat *bawaan*, dan
alamat bawaan tetap benar persis saat plumbing-nya rusak, jadi test-test itu tidak bisa menangkap
cacat yang mereka klaim tutup.

Ditutup dengan tiga berkas: `antigravity_paths_test.go` (lima kasus: tidak ada deklarasi, tiap kunci
dipindah sendiri, dan ketiganya sekaligus), `claude_paths_test.go` (deklarasi `oauth_url` memindah
bacaan utama; deklarasi `settings_url` memindah pasangan legacy yang hanya dipakai saat bacaan utama
menolak), `google_paths_test.go` (deklarasi `load_code_assist_url` memindah lookup project), plus
`TestGrok_AsksTheDeclaredUserURL` dan satu route kedua di stub grok.

**Buktinya bukan "testnya hijau", tapi testnya merah saat plumbing-nya dirusak.** `declaredOr`
dibuat selalu mengembalikan bawaan: 7 subtest gagal dengan pesan yang menyebut alamat yang salah
(`calls = [POST /v1internal:loadCodeAssist …], want [POST /moved:loadCodeAssist …]`). Cabang
deklarasi `user_url` grok dibuat mati: `user read = 1 at "/v1/user", want 1 at the declared
/v1/declared-user`. Kedua mutasi lalu dilepas dan berkas diverifikasi identik byte-per-byte dengan
sebelumnya.

Yang sengaja tidak diubah: enam field `UsageEndpoints` yang tidak dibaca siapa pun (`token_url`,
`limits_path`, `reset_credits*`, `cw_host`, `q_host`). Mereka ada supaya cermin itu lengkap terhadap
`registry.UsageConfig` dan supaya test lebar `NumField()` menangkap kunci baru yang masuk registry
tanpa ikut dipetakan. Menghapusnya membuat cermin itu parsial, dan cermin parsial adalah cara cacat
#2 lahir.

### 3.12 F12 (DITUTUP di pass ini): split kebijakan gagal reference hilang saat pindah ke cache

Rencana pass ini menyebut satu hal yang harus dipertahankan: reference membedakan keluarga yang
**melempar error** (layar menampilkan baris error) dan keluarga yang **mengembalikan pesan** (lunak,
angka terakhir dipertahankan). Diukur ke pohon: `quotafetch.Fetch` mengembalikan `Result` dan tidak ada
satu pun keluarga yang punya saluran error. Split itu diseragamkan jadi "semua lunak", padahal
instruksinya eksplisit: "port that per family rather than unifying it".

Dibaca ulang, split literal tidak bisa dipertahankan di bawah arsitektur yang owner pilih. Reference
melempar ke pemanggil yang sedang menunggu; worker tidak punya pemanggil saat ia gagal, dan ia
**sengaja** menyimpan angka baik terakhir alih-alih menghapusnya. Baris error yang menggantikan angka
itu akan berbohong tentang data yang memang masih benar. Jadi yang dipertahankan adalah informasinya,
bukan mekanismenya:

- `failures` (panjang run kegagalan) dan `last_attempt_at` (kapan terakhir ditanya, berhasil atau
  tidak) kini ikut ke wire: keduanya sudah ada di baris state dan tidak pernah dibaca siapa pun
  (`FailuresRun` dan `LastAttemptKnown` ditulis lalu dibiarkan; `LastAttemptKnown` diganti menjadi
  `LastAttemptAt *time.Time` supaya instannya nyata, bukan hanya boolean).
- Kartu menampilkan "Last poll failed (N in a row), asked <instant>. The figures above are the last the
  provider gave." dengan `--color-warn`, bukan danger, karena bacaan yang menghasilkan angka itu memang
  selesai.
- `last_attempt_at` sengaja beda jenis dengan `fetched_at`: poll yang gagal menggeser yang pertama dan
  membiarkan stempel yang kedua di tempat angka itu diucapkan.

Dijaga di tiga lapis: test handler membuktikan kedua field sampai ke body (`"failures":3`,
`"last_attempt_at":"2025-09-27T19:16:40Z"`, `fetched_at` tetap lebih tua) dan jawaban sehat justru
**tidak** membawanya; test helper mengunci empat keadaan run kegagalan; test kartu memastikan angka
yang bertahan tidak ditarik dan kalimatnya muncul. Test kartu itu sendiri menangkap satu jebakan
kecil: regex `/asked .*2026/` gagal karena `.` tidak melewati baris baru di template, diperbaiki jadi
`[\s\S]`, dan itu terbukti dari teks yang benar-benar dirender, bukan dari tebakan.

### 3.13 F13 (DITUTUP di pass ini): budget dan concurrency sweep akhirnya bisa diatur operator

Rencana langkah D menulis satu kalimat yang lewat tanpa jejak di pohon: budget sweep "config-injected
per §1.4's typed Config, no raw `os.Getenv`". Yang terpasang sampai audit terakhir adalah
`DefaultPublishedPollPolicy()` dipanggil langsung dari wiring, tidak ada `os.Getenv` liar (jadi §1.4
tidak dilanggar), tetapi juga tidak ada apa pun yang bisa digerakkan operator tanpa mengubah kode.
Angka 40 dan 4 itu keputusan yang benar sebagai *default*, bukan sebagai *satu-satunya nilai*:
berapa banyak akun yang boleh ditanya per tick bergantung pada berapa banyak akun deployment dan
seberapa toleran provider-nya, dan itu tahu operator, bukan biner.

Ditutup mengikuti pola yang sudah ada di file itu, bukan pola baru: `QUOTA_POLL_BUDGET` (default 40,
plafon 500) dan `QUOTA_POLL_CONCURRENCY` (default 4, plafon 16) masuk ke tabel `intFields` bertipe plus
validasi "between" yang sama seperti `DB_POOL_MAX`, lalu mengalir ke policy lewat
`WithSweepLimits(budget, concurrency)`, yang sengaja hanya mengganti dua angka itu dan menolak nilai
≤ 0, supaya timeout dan seed page tidak ikut hilang dan deployment yang tidak men-set apa pun
berperilaku persis seperti sebelumnya.

Bukti jalur lengkap, bukan hanya unit:

- boot dengan `QUOTA_POLL_BUDGET=0` → `app-serv: config: invalid: QUOTA_POLL_BUDGET must be between 1 and 500`,
  proses keluar **sebelum** bind port (gagal cepat, bukan worker diam-diam).
- boot dengan `QUOTA_POLL_BUDGET=7 QUOTA_POLL_CONCURRENCY=2` → "is listening", tanpa error atau panic,
  lalu dihentikan berdasarkan PID dan portnya dilepas.
- `TestNewQuotaPublishedWorkerRejectsAnUnsetPolicy` menjaga hal yang membuat wiring yang hilang tidak
  bisa lolos diam-diam: policy tanpa budget ditolak konstruktor.

### 3.14 F14 (DITUTUP di pass ini): cooldown `RateLimitedUntil` belum dihormati sweep

Langkah D rencana menyebut satu syarat kelayakan yang belum ada di pohon sama sekali: "Skip when
`RateLimitedUntil()` is in the future or `next_attempt_at > now`". Yang kedua ada (antrian indeed
difilter oleh `next_attempt_at`); yang pertama tidak. `grep` atas seluruh berkas worker dan repository
published tidak menemukan satu pun referensi ke deadline cooldown yang sudah disimpan agregat dan sudah
dipakai jalur data (`UpstreamEndpoint.Available`). Artinya: endpoint yang baru saja menjawab 429 ke
traffic nyata tetap ditanya provider-nya pada tick berikutnya, tepat pada saat yang paling tidak tepat
untuk bertanya.

Ditutup di SQL antrian, bukan di loop: `DueForRefresh` kini mengecualikan endpoint yang
`rate_limited_until` masih berlaku pada instan yang sedang ditanyakan sweep. Tempatnya di `WHERE`
sebelum `LIMIT` karena alasan yang sama dengan budget: kalau disaring setelah baris kembali, endpoint
yang sedang cooldown tetap memakan slot oldest-due dan menahan akun di belakangnya kelaparan, justru
kebalikan dari yang cooldown itu coba capai.

`TestPublishedQuotaRepository_DueForRefreshSkipsAThrottledEndpoint` menjaganya di PostgreSQL nyata:
dua akun jatuh tempo bersamaan, satu di-throttle → hanya satu yang kembali; lalu deadline digeser ke
masa lalu → keduanya kembali, tanpa ada yang perlu mengedit baris. Test ini juga mengoreksi satu
jebakan yang saya buat sendiri saat menulisnya: membandingkan cooldown dengan instan sweep, bukan dengan
jam proses, karena deadline adalah momen dan bukan bendera.

### 3.15 F15 (RESIDU, gerbang bersama): `go-test.sh` mewarisi timeout 10 menit untuk paket yang butuh 9,3

Bukan temuan kuota, tetapi temuan tentang **kepercayaan pada sinyal hijau**, jadi dicatat di sini.
`scrypts/gates/go-test.sh` menjalankan `go test -race -count=1 ./...` tanpa `-timeout`, jadi setiap
paket mewarisi default Go 10 menit. Paket `internal/router` (yang memegang test pin rute dan
sengaja tidak disentuh pass ini) membutuhkan **434s dan 560s** saat dijalankan sendiri di mesin dua
core ini. Artinya gerbang resmi berada satu derajat beban dari gagal bukan karena ada assertion yang
salah, dan itulah persis yang terjadi pada rantai verifikasi pass ini: `FAIL internal/router 600,151s`
(600,151 detik = timeout default, bukan kegagalan test).

Yang TIDAK dilakukan: mengubah `scrypts/gates/`. Gerbang itu dipakai bersama dan di luar rencana
yang disetujui. Yang dilakukan: verdict router pass ini dinyatakan **belum diketahui** sampai paket itu
dijalankan ulang sendirian dengan `-timeout 20m`, dan hasil akhirnya dicatat apa adanya di §10.

**Dikonfirmasi kemudian, bukan diasumsikan:** router dijalankan sendirian dengan
`-race -count=1 -timeout 25m` dan hasilnya **`ok internal/router 307,547s`** (5,1 menit tanpa beban,
jauh dari 600,151 detik tempat ia mati sebelumnya). Jadi diagnosis "timeout karena beban, bukan assertion
yang salah" benar, dan F15 tetap berlaku sebagai kerentanan gerbang (paket itu butuh ~half dari budget
default bahkan saat sendirian, dan apa pun yang berjalan berbarengan akan menolaknya melewati batas).

Yang disarankan ke owner (satu baris, risiko kecil): `go test -race -count=1 -timeout 20m ./...` di
gerbang, atau `-timeout` per-paket untuk `internal/router`. Tanpa itu, CI di mesin dua core akan
menghasilkan false negative yang menggoda orang untuk "memperbaiki" kode yang tidak rusak, cara
kegagalan yang sama persis dengan yang baru saja dihindari di sini.

### 3.16 F16 (DITUTUP di pass ini): split "throw vs pesan" reference ternyata hilang, dan itu membuat backoff tidak pernah aktif

F12 menutup separuh pertama split itu (fakta "poll terakhir gagal" sampai ke operator). Audit
terhadap reference menutup separuh yang lain, yang ternyata adalah bug perilaku, bukan gaya:

`open-sse/services/usage.js` tidak menangkap apa pun: handler yang `throw` membuat panggilan
gagal di pemanggil, sementara handler yang mengembalikan `{message}` menjawab dengan kalimat. Dua
file yang `throw` karena provider error adalah `github.js:39` dan `google.js:155` (jalur antigravity).
Port ini menyatukan keduanya: `Result` tidak punya penanda, jadi 502 github tersimpan sebagai
**jawaban lunak**. `consecutive_failures` tidak naik, backoff tidak pernah menyala, dan endpoint
yang rusak terus ditanya pada floor keluarganya selamanya sementara cache terlihat sehat. Itu
persis kegagalan senyap yang §1.6 minta dicegah.

Daftar keluarnya diverifikasi, bukan ditebak: hanya `codex.js` (6 `throw`; tidak ada entri registry,
sudah masuk residu §6.4), `github.js` (3) dan `google.js` (1) yang melempar; tidak ada `Promise.reject`
atau `reject(` di seluruh direktori, dan `misc.js` menangani `!response.ok` dengan mengembalikan pesan.
Lemparan di `google.js:155` berada di dalam `getAntigravityUsage` (mulai baris 120), bukan
`getGeminiUsage` (baris 22), jadi yang ditandai keras adalah antigravity, dan gemini-cli memang tetap
lunak.

Perbaikannya kecil dan berada di tempat yang benar: `quotafetch.Result.Failed` (ditandai lewat
`hardFailure`, yang hanya memanggil `softFailure` lalu men-set satu bit, kalimatnya tetap sama,
yang berbeda hanya siapa yang mengangkatnya), dipakai di `fetchGitHub` dan `fetchAntigravity`;
`pollOne` mengirim jawaban berpenanda ke jalur gagal (delta 1, backoff, kalimat tetap disimpan,
bucket baik terakhir tetap utuh) dan mengembalikannya sebagai "tidak terjawab".

Yang sengaja TIDAK dibuat: field `failed` di wire. Setelah satu tick (≤ 1 menit) keadaan ini sudah
terbaca sebagai `failures: 1`, dan menambahkan sumbu styling kedua untuk jendela sekecil itu berarti
mengubah kontrak lagi demi perbedaan yang operator tidak akan pernah lihat berdiri sendiri.
Keputusan ini ditulis supaya bisa ditantang, bukan disembunyikan.

Bukti, termasuk non-vakuitas: `failure_policy_test.go` (3 test: `softFailure` vs `hardFailure`
berbeda hanya di satu bit dan 2xx tidak pernah men-setnya; github 502 → `Failed`; opencode 500 →
bukan `Failed`), dan `quota_published_failure_test.go` (refusal → `polled = 0`, delta 1, kalimat
tersimpan, bucket bertahan; kalimat tanpa penanda → `polled = 1`, delta 0). Cabangnya dibuat mati
sengaja: test refusal gagal (`sweep polled = 1, want 0`), lalu worker dikembalikan dan diverifikasi
identik dengan cadangan.

## 4. Implementasi backend

**Migrasi `000013_published_quota`.** Dua tabel, PK `endpoint_id` dan `(endpoint_id, label)`, index
`next_attempt_at` (ordered key sweep) dan `fetched_at` (prune TTL), FK `ON DELETE CASCADE` ke
`upstream_endpoints`, `consecutive_failures >= 0`. `total` NULLable dengan komentar eksplisit bahwa NULL
= tanpa batas dan `0` = batas habis. Dua fakta berbeda, dan menyamakannya adalah cara paling menyesatkan
untuk salah gambar. `is_credit_balance` adalah keadaan ketiga: saldo uang dengan mata uang bernama, bukan
sinonim `unlimited`.

**Repository** `internal/repository{,/postgres}/quota_published*.go` + port `PublishedQuotaRepository`.
Empat metode, semuanya berbatas; baca layar satu statement untuk seluruh halaman (diuji dengan
`pgx.QueryTracer`: 3 id = 1 statement, 0 id = 0 statement). `StorePublished` satu transaksi: upsert
state, upsert bucket, lalu **hapus label yang tidak ada di jawaban baru**. Prune ini yang mencegah
provider yang mengganti nama bucket meninggalkan total lama di samping penggantinya, dan biayanya per
tabel, bukan per bucket (diuji: 1 bucket dan 8 bucket = 5 statement).

**Fetch layer** `quotafetch/`: `request.go` (satu helper keluar: `requestUsage`, `bearer`, `decodeBody`,
`softFailure`), `endpoints.go` (resolusi declared→built-in, `endpointFor` sebagai seam test), 14 keluarga
baru, tiap berkas di bawah 250 baris dan tiap keluarga menyimpan kebijakan kegagalannya sendiri. Groq
khusus: tidak punya endpoint usage, kuotanya ada di header `x-ratelimit-*` dari `GET /v1/models`, dan
reset-nya durasi Go (`"2m59.56s"`). Grok khusus: frame gRPC-web biner, diuraikan di `grok_frame.go`
dengan validasi panjang sebelum slicing dan batas payload 1 MiB, karena yang di-parse adalah byte
pihak ketiga.

**Paritas dijaga test, bukan daftar:** `quotafetch/parity_test.go` dua arah. Tiap kunci fetcher harus
jadi id registry yang nyata (menangkap F3), dan tiap provider `usage: true` harus punya fetcher
(menangkap F1, dan akan menangkap provider berikutnya yang di-add tanpa handler).

### 4a. Kebijakan worker yang sebenarnya terpasang

Angka ini dibaca dari `quota_published_policy.go`, bukan dari rencana:

| Kebijkan | Nilai | Catatan |
| --- | --- | --- |
| Interval keluarga | default **2 menit**, claude **10 menit**, gemini-cli/antigravity/grok-cli **5 menit** | claude karena endpoint-nya menjawab 429 pada cadence lebih tinggi; keluarga cloudcode menghitung kuota per project, bukan per request |
| Backoff | dasar **30 detik**, plafon **30 menit**, di-double per kegagalan, diambil `max(family floor, backoff)` | supaya claude yang gagal dua kali tidak kembali lebih cepat dari floor-nya sendiri |
| Budget sweep | **40** endpoint per tick sebagai default, kini lewat config `QUOTA_POLL_BUDGET` (F13; rencana menyebut 50) | yang terbaca operator: satu halaman layar tidak pernah menunggu lebih dari satu tick |
| Concurrency | **4** semaphore berbatas, lewat config `QUOTA_POLL_CONCURRENCY` (F13) | bukan `engine_fusion.fanOut` yang tak berbatas |
| Timeout | fetch **25s**, store **10s** | fetch di atas requestTimeout `quotafetch` 20s |
| Seed | **200** endpoint per tick, satu halaman per tick | `SeedScheduling` batched tercatat sebagai follow-up di kode |
| Tick worker | **1 menit** (`publishedPollTick`) | interval keluarga adalah floor di atas tick, bukan pengganti tick |

Verifikasi hidup: satu sweep menanyakan 5 akun dan hanya 5. Akun custom-node dan lane virtual
tidak pernah ditanya, dan tidak ada akun dengan kegagalan beruntun (`consecutive_failures = 0`).

**Jitter dan floor terbaca di data produksi, bukan hanya di test clock palsu.** `gap = next_attempt_at
- last_attempt_at` per akun: `122, 139, 147, 134, 146` detik, semuanya di atas floor 120 detik keluarga
default, dan ketiganya akun `qoder` jatuh di detik yang berbeda-beda. Artinya risiko #2 (herd: seribu
satu keluarga ditanya pada detik yang sama) tidak sedang terjadi bahkan pada skala lima akun, dan yang
mencegahnya adalah `publishedJitter`, bukan kebetulan.

**Biaya baca satu halaman collection, diukur dari handler:** 5 statement. Dua `count(*)` grup (satu dari
baca window, satu dari baca account), dua page-read, dan **tepat satu** baca batch provider-cache untuk
seluruh halaman (itulah yang dijaga `TestQuotaHandler_ListProviderReadIsOneQueryPerRoute`). Dua count itu
redundan: handler memakai total dari baca window dan mengabaikan total dari baca account, padahal keduanya
sudah dipastikan sama karena berbagi `quotaGroupCount`. Yang tidak dilakukan di pass ini: menambah method
repo tanpa-count hanya untuk menghapus satu statement. Cost-nya sebuah index scan kecil per halaman,
bukan per akun, dan menghapusnya berarti API surface baru.

| Gerbang | Hasil |
| --- | --- |
| `go build ./...`, `go vet ./internal/...` | PASS |
| `go test -race -count=1 ./internal/service/quotafetch/` | PASS (18 keluarga, tiap keluarga: happy path, kunci PATH+method+header, 401, non-2xx, nol panggilan tanpa kredensial, tepat satu panggilan per baca) |
| `quota_usage_credential_test.go` (F6, jalur produksi) | PASS, dan **dibuktikan gagal** kalau wiring dilepas: `account facts = map[], want map[email:… projectId:proj-pub userId:user-pub]` untuk kedua cabang auth. Worker polling memakai `QuotaService.livePublishedUsage` yang sama (`worker_wiring.go` → `NewQuotaPublishedWorker` → `fetch = deps.Quotas.livePublishedUsage`), jadi satu test ini menutup jalur layar DAN jalur worker |
| `go test -count=1 -run TestUsageEndpoints… ./internal/service/` | PASS: 16 key `UsageConfig` + test lebar |
| `go test -tags=integration ./migrations/` terhadap PostgreSQL nyata | **PASS**: 000013 ter-apply, bentuk kolom + index terverifikasi, invariant role-app `ownership_test.go` hijau untuk kedua tabel baru; diulang di tree final (lihat baris di bawah) |
| `000013_…down.sql` (kini dijaga test) | **PASS**. `TestPublishedQuotaDownDropsBothTablesAndReapplies` (integration): naik → 2 tabel, down → 0 tabel, re-apply **tanpa** membersihkan ledger → tetap 0 (inilah jebakan yang operator tabrak saat rollback manual), ledger dibersihkan → re-apply → 2 tabel. Sebelumnya klaim ini hanya dibuktikan sekali dengan tangan. Catatan operasi yang muncul dari menjalankannya, bukan dari membacanya: runner melewati migrasi yang sudah tercatat di `schema_migrations`, dan berkas down **tidak** menghapus baris ledger. Jadi rollback = jalankan down **plus** hapus baris `000013%` dari ledger; tanpa itu re-apply diam-diam tidak menciptakan kembali tabelnya dan test bentuk schema-lah yang menjerit, bukan boot |
| `go test -tags=integration ./migrations/ ./internal/repository/postgres/` terhadap PostgreSQL nyata | **PASS di tree final** (`migrations 2,188s`, `postgres 11,705s`): satu statement per batch, prune label, NULL-total-vs-0 bertahan round-trip, dan invariant role-app `ownership_test.go` hijau untuk kedua tabel baru setelah F11-F14 |
| Panel (`bun run check`) | PASS: 0 errors, 0 warnings |
| Panel (`bun run lint`, `bun run lint:ts`) | PASS: prettier bersih, eslint bersih |
| Panel (`bun run build`) | PASS: exit 0 |
| Panel suite penuh, solo, tree F16 | **PASS: 179 berkas / 2.933 test hijau, rc=0** (exit code dari `bun run test` sendiri). Durasi 1.713,39s saat sepi; dua percobaan sebelumnya mati di ceiling 50/55 menit karena saya menumpuk pengukuran lain di atasnya |
| Panel suite quota (`bun run test -- quota`) | PASS: 242 test / 11 berkas (keadaan unlimited, credit balance, unit, expires-vs-resets, "belum di-poll", `published_note`, read `?force=1` per koneksi, refusal yang membuang angka lama, jalur tanpa provider) |
| Panel suite penuh, **solo** di tree final | **PASS: 179 berkas / 2.933 test, semua hijau, 1.713,39s (28,6 menit)**. Exit code diambil dari `bun run test` itu sendiri, bukan dari pipe. Dua percobaan sebelumnya mati di ceiling 50 dan 55 menit BUKAN karena testnya lambat. Suite ini selesai dalam 28,6 menit saat tidak ada apa pun yang berjalan berbarengan, dan lebih dari 55 menit saat saya menumpuk pengukuran lain di atasnya |
| `scrypts/gates/go-lint.sh` | **PASS di tree final** (setelah F11/F12): vet (+tagged), gofmt, staticcheck (+tagged) **0 issues**, golangci-lint **0 issues** pada rerun sendirian (lihat §9 soal tabrakan gerbang), ambang 250 baris §1.1: tidak ada berkas ≥250 |
| `scrypts/gates/go-headers.sh` | **PASS**: 1153 berkas Go membawa header §1.2 lengkap |
| Test kunci alamat deklaratif (F11) | `antigravity_paths_test.go` (5 kasus), `claude_paths_test.go` (3), `google_paths_test.go` (2), `TestGrok_AsksTheDeclaredUserURL`, **dibuktikan merah** saat `declaredOr` dibuat mengabaikan deklarasi (7 subtest gagal) dan saat cabang `user_url` grok dimatikan; berkas hasil mutasi diverifikasi identik byte-per-byte setelah dikembalikan |
| Field percobaan gagal (F12) | Test handler mengunci `"failures":3` + `last_attempt_at` nyata dengan `fetched_at` yang tetap lebih tua, dan mengunci keduanya **tidak muncul** untuk jawaban sehat; payload hidup mengonfirmasi `last_attempt_at` ada di kelima entri dan `failures` tidak ada karena semua akun sehat |
| Config sweep (F13) | `QUOTA_POLL_BUDGET` / `QUOTA_POLL_CONCURRENCY` diuji table-driven (nol, negatif, di atas plafon, absent) plus dua assert default; boot nyata dengan budget 0 keluar **sebelum** bind port, dengan 7/2 mencapai "is listening" |
| Cooldown endpoint (F14) | `TestPublishedQuotaRepository_DueForRefreshSkipsAThrottledEndpoint` di PostgreSQL nyata: satu di-throttle → hanya satu kembali; deadline digeser ke masa lalu → keduanya kembali |
| `scrypts/gates/secrets.sh --all` | **PASS kedua mode**: sempat FAIL di working tree karena fixture test saya sendiri memakai awalan `sk-` (26 karakter) yang terbaca gitleaks sebagai `generic-api-key`; diganti nilai non-kredensial yang tetap panjang dan tetap membuktikan scrub. Commits: bersih |
| `scrypts/gates/contract-openapi.sh`, `contract-drift.sh` | **PASS** |
| `go test -race -timeout 25m ./internal/router/` sendirian | **PASS: `ok 307,547s`** di tree F16. Ini yang mengubah verdict router dari "belum diketahui" menjadi hijau, dan sekaligus membuktikan F15: paket yang sama mati di 600,151s (timeout default) saat ada beban lain berjalan |
| `go test -race ./internal/... ./cmd/...` + `./tools/...`, tree F16, **solo** | **PASS: 16 paket `ok`, nol FAIL, nol panic**; `tools/openapi-gen` ok 3,466s; `migrations` hijau di bawah tag integration (ok 2,188s). F9 tidak muncul lagi pada tree yang sama saat mesin sepi (`internal/service 67,95s`), yang mengonfirmasi diagnosisnya |
| `go test -race ./...` (`scrypts/gates/go-test.sh`) | **pernah 1 paket merah, bukan karena kuota**: 17 paket hijau; `internal/service` gagal di `TestProviderModelTestService_AProbeThatRunsOutItsBudgetIsATimeout` (F9, test pass-017 dengan margin 20 ms di atas budget 20 s). Test yang sama **PASS** sendirian (21,087s), dan **PASS lagi** di rerun terakhir (`go test -race ./internal/service/... ./internal/handler/... ./cmd/...` → `ok internal/service 111,141s`). Dua hasil bertolak belakang dari tree yang sama adalah diagnosis flake, bukan regresi. `go test -race -tags=integration ./...` merah hanya pada test opt-in yang menuntut `PANNELAI_TEST_REDIS_ADDR` / `PANNELAI_QODER_PAT` (44 baris, semuanya "must be set"; nol kegagalan test kuota) |
| Click-through hidup (Chrome headless via CDP, panel dev `:3001` → gateway baru `:9091`, Postgres + Redis milik operator) | **PASS**: lihat §7 |

## 6a. Yang berubah di luar rencana, dan mengapa

Empat hal muncul karena diukur, bukan karena direncanakan:

1. **`RecordAttempt` bisa membawa kalimat provider.** Rencana menuliskan bahwa jawaban lunak (tanpa
   bucket) hanya boleh di-log, karena `StorePublished` menghapus label yang tidak ada di jawaban, jadi
   menyimpan jawaban kosong berarti menyapu bersih angka baik di kartu. Itunya benar, tetapi akibatnya
   salah: akun dengan kredensial mati akan dibaca layar sebagai "belum di-poll", padahal provider sudah
   menjawab. Jadi `domain.PublishedAttempt` lahir, satu penulis state yang men-stamp jadwal DAN
   `plan`/`message` dengan `COALESCE`, tanpa menyentuh baris window dan tanpa menyentuh `fetched_at`
   (percobaan bukan jawaban; stamp yang menua dirinya adalah kebohongan).
2. **Delta kegagalan adalah 1, bukan panjang run.** Saat memindahkan `schedule` ke bentuk struct, delta
   sempat diisi `ConsecutiveFailures+1`. Repository **menambahkan** delta ke run yang tersimpan, jadi
   backoff berlipat ganda diam-diam. Test worker milik agent yang menangkapnya
   (`deltas = 0 and 5, want 0 and 1`), bukan review.
3. **`published[]` tidak boleh menjadi `null`.** Test bentuk schema menemukan jalur handler mengirim
   `"published":null` untuk halaman tanpa jawaban, sementara `data` selalu `[]`. Diperbaiki di handler
   (di tempat jaminan itu memang ada), dan assertion dipindah ke test route.
4. **Kredensial yang dikembalikan provider kini disanitasi.** Ini temuan audit, bukan rencana.
   `softFailure` mengutip sampai 200 byte body provider ke kalimat kartu, dan error transport Go
   mengutip URL. Karena worker kini **menyimpan** kalimat itu ke `quota_published_state.message`,
   sebuah token bearer yang di-echo vendor bisa berakhir di disk dan di layar. `requestUsage` sekarang
   mengetahui kredensial yang ia pasang sendiri (header kredensial + `?api_key=`-style query) dan
   menghapusnya dari body maupun error sebelum keluar. Nilainya pendek (<9 char) tidak disanitasi:
   kalau ikut, kalimat provider yang menyebut versi editor jadi rusak, dan itu justru yang operator
   butuhkan. Test mengunci kedua arah: token yang di-echo hilang, `Editor-Version` yang di-echo utuh.

Perubahan #4 juga mengubah bentuk helper: `decodeBody[T any]` dihapus karena linter repo menandai
`any` secara literal (AGENTS.md §1.4) dan tidak ada preseden `nolint:forbidigo` di pohon ini; 21
titik panggil kini memakai `json.Unmarshal` langsung dengan target konkret.

5. **F6 nyatanya belum terpasang, dan yang menemukannya adalah gerbang, bukan review.** Lihat §3.6:
   pemetanya ada + punya test + nol pemanggil produksi. Pelajaran prosesnya konkret: "ada fungsi dan
   ada test untuk fungsi itu" bukan bukti bahwa jalur produksi memakainya. Yang menangkap adalah
   `staticcheck -tags=integration` (`U1000 func publishedAccountData is unused`), jadi gerbang
   ber-tag itu bukan formalitas dan tidak boleh dihapus saat tree terasa berisik. Test penutupnya
   (`quota_usage_credential_test.go`) dibaca lewat seam fetcher yang sama, dan sudah dibuktikan
   gagal kalau wiring-nya dilepas.
6. **SPEC-UI §6.6 mendeskripsikan layar yang sudah tidak ada.** Saat §6.6 dibaca ulang terhadap kode
   (bukan sebaliknya), dua bullet masih menjelaskan bentuk pra-reshape: body kartu "daftar window dengan
   used/limit/persen/badge sumber", dan "Source badge: computed atau reported" per baris. Yang pertama
   sudah digantikan satu baris ringkasan; yang kedua tidak ada lagi sebagai badge: `grep` atas
   `Quota*.svelte` menunjukkan kata itu hanya hidup di **legend** bawah kartu (`QuotaCards.svelte:244`).
   Keduanya diperbaiki di pass ini dan ditandai superseded, karena spec yang tidak cocok dengan layar
   lebih berbahaya daripada spec yang belum ditulis: pembaca berikutnya akan "memperbaiki" kode yang
   justru benar.

## 6b. Residu yang diketahui sebelum layar dibuka

- **`github` (F7)**: endpoint kuotanya membaca token akun, entri registry-nya berauth-type kunci →
  kartu akan menolak dengan kalimat yang benar, bukan menampilkan angka. Butuh keputusan registry.
- **`ProviderSpecificData`**: kini terisi untuk akun OAuth (project, email, user id); akun kunci tetap
  tidak punya project karena memang tidak ada sumbernya.
- **Cooldown Claude** hidup di jadwal worker (10 menit), bukan di fetcher yang stateless; belum ada
  penahan 180 detik per-token seperti reference.

## 6. Residuan untuk pass berikutnya

1. **F6 `ProviderSpecificData`**: keputusan pemodelan owner; tanpa ini keluarga google membayar satu
   request bootstrap per poll.
2. **F7 `github` auth-type vs endpoint kuotanya**: perlu keputusan registry, bukan tebakan kode.
   Ditulis sebagai SPEC-UI §14 item 28 supaya keputusan itu tercatat di spec, bukan hilang di pass ini.
3. **Cooldown Claude di level worker.** Fetcher sengaja stateless; kalimat 429 sudah mengembalikan
   `Retry-After`, dan penjadwalan tahan-timeout adalah milik sweep, bukan fetcher.
4. **Keluarga reference yang tidak diport karena tidak ada entri registry-nya:** `codex`, `kiro`, `iflow`,
   `ollama`, `glm-cn`, `minimax-cn`, `xiaomi-mimo`, `vercel-ai-gateway`. Port-nya tersedia di reference;
   yang belum ada adalah provider-nya di KEEP-set owner.
5. **Utang dokumen PORT 004 ditutup di pass ini**: SPEC-UI §14 kini punya item 26 (`provider_id`
   longgar = keadaan nyata, bukan payload rusak) dan 27 (persen yang dicetak = persen TERPAKAI, warna bar
   dari SISA; asimetri ini disengaja). §6.6 tidak lagi menyebut "Table:".
6. **`docs/DRAFT` register**: G22 (quota re-check) kini punya pembacaan worker untuk sisi terbit;
   sisi hitungan `quota_windows` tidak berubah.

## 7. Click-through hidup: yang diukur di layar, bukan di fixture

Persetujuan owner: build baru di port cadangan, Postgres + Redis tetap milik operator. Panel dev di
`:3001` (`PANEL_API_TARGET=http://127.0.0.1:9091`) → `app-serv` final di `:9091`; Chrome headless
dikendalikan lewat CDP, sesi operator dipakai satu kali login (204, cookie `pannel_session`).

Angka di bawah adalah hasil `Runtime.evaluate` atas DOM yang sudah ter-render, bukan pembacaan kode:

| Yang diukur | Nilai | Artinya |
| --- | --- | --- |
| Blok provider terender | **5** | tepat 5 akun yang kuotanya bisa ditanya; bukan 8 seperti sebelum F10 ditutup |
| **Bar progres terender** | **9** | PORT 004 §7.2 mengukur layar yang sama: **0**. Inilah angka yang menjawab keluhan owner |
| Kartu `codebuddy-intl` | `Bonus Pack 1 100% 250 / 250`, `Bonus Pack 2 0% 0 / 100`, `Bonus Pack 3..8 100% 30 / 30`, `Bonus Pack 9 0% 0 / 30` | angka provider nyata, bukan hitungan gateway, tanpa satu pun tombol ditekan |
| Stempel "Asked" | **1** (hanya kartu codebuddy) | jawaban lunak qoder/opencode tidak lagi mencetak `0001-01-01` (F8); `yearOnes: none` |
| Kalimat "Not polled yet" | **0** | tidak ada lagi klaim polling untuk provider yang tidak menerbitkan apa pun (F10) |
| Baris `Counted by this gateway` | 10 | hitungan gateway tetap ada, satu baris di bawah angka provider |
| Lebar 390 px | `scrollWidth 390 == viewport 390`, elemen meluber: **0**, tombol tinggi < 44 px: **0** | tidak ada overflow ponsel; target sentuh utuh |

Invarian tabel cache ikut dibaca langsung dari PostgreSQL produksi (bukan dari test): 5 baris state, 9
baris window (semuanya milik codebuddy-intl), `orphan_window_rows = 0` (tidak ada angka yang bertahan di
bawah jawaban yang tidak punya angka), `rows_without_state = 0`, dan `consecutive_failures = 0` di semua
baris, jadi risiko #1 (label yang berganti nama meninggalkan total basi) tidak sedang terjadi, dan
prune-lah yang menjaganya.

Dua hal yang hanya terlihat di layar dan tidak ada di rencana: F8 (stempel tahun 1 pada jawaban lunak) dan
F10 (blok provider untuk provider yang tidak menerbitkan kuota). Keduanya ditutup di pass ini; §6b tidak
lagi menyebutnya residu.

Setelah F12 ditutup, layar yang sama diukur ulang dengan build final: payload tetap 5 entri dan 9 baris
codebuddy, `failures` **tidak** muncul (semua akun sehat, field-nya omitempty), dan `last_attempt_at`
muncul nyata di kelima entri, jadi jalur repo → service → handler → kontrak → panel terbukti hidup
sampai ke produksi, bukan hanya ke fixture. Yang **tidak** bisa dilihat di layar ini adalah kalimat
"Last poll failed" itu sendiri: tidak ada satu pun akun operator yang sedang punya run kegagalan
(404 opencode-zen dihitung jawaban lunak, bukan kegagalan), jadi baris warn itu dibuktikan test
komponen dan test handler, bukan oleh mata. Ditulis apa adanya supaya pembaca berikutnya tidak
mengira baris itu sudah terlihat hidup.

Daftar verifikasi rencana menyebut tiga hal yang harus dilihat di layar: (a) kartu provider
menampilkan bar dari angkanya sendiri tanpa menekan apa pun, (b) keluarga rawan 429 menampilkan cooldown-nya
alih-alih error, (c) baris hitungan gateway tetap ada di bawahnya. (a) dan (c) terukur di atas. **(b) tidak
bisa diukur di deployment ini**: tidak ada satu pun akun claude/antigravity/gemini-cli di dalamnya, jadi
jalur cooldown hanya terbukti di test: `claude_test.go` mengunci kalimat 429 beserta `Retry-After`-nya,
`quota_published_worker_test.go` mengunci floor 10 menit claude, dan F16 memastikan refusal tetap
terhitung sebagai kegagalan tanpa menghapus angka baik terakhir. Dinyatakan di sini supaya tidak ada yang
mengira baris itu sudah dilihat dengan mata.

Yang masih belum bisa dibuktikan layar ini: keluarga OAuth yang tidak punya akun di deployment operator
(claude, antigravity, gemini-cli, github, grok-cli, zed). Test stub membuktikan bentuk request-nya, bukan
jawaban providernya. Klaim "hijau" untuk keluarga-keluarga itu butuh akun nyata.

## 8. Checklist AGENTS.md per-PR: dengan buktinya, bukan klaim

| Butir | Keadaan |
| --- | --- |
| §1.1 ambang 250 baris | Tidak ada berkas ≥ 250 di tree final (diperiksa dari keluaran gerbang, bukan dari ingatan): 26 berkas berada di pita peringatan 220-249 yang hanya minta direview. Empat berkas menyentuh atau melewati ambang dan dipecah karena alasan berubah yang nyata, bukan demi angka: `quota_published_worker.go` 250 → 190 (`quota_published_store.go` = apa yang satu jawaban tulis), `quota_published_policy.go` 250 → 177 (`quota_published_seed.go` = menyiapkan akun yang belum punya baris jadwal), `quotafetch/grok.go` 250 → 187 (`grok_wire.go` = bentuk payload), `handler/quota.go` 252 → 195 (`quota_published_response.go` = peta jawaban provider ke body). `quota_usage_test.go` sempat 264 setelah test wiring ditambahkan → 202 + `quota_usage_credential_test.go` 88 |
| §1.2 header per berkas | `go-headers.sh` PASS; berkas baru membawa header lengkap (`quota_published_store.go`, `quota_published_seed.go`, `grok_wire.go`, `quota_usage_credential_test.go`) |
| §1.3 bahasa error | Kalimat client-facing tetap Inggris; kalimat provider lewat apa adanya (F4), dan yang dibuang hanyalah markup + kredensial yang dia-echo |
| §1.4 nol `any` | `decodeBody[T any]` dihapus; target decode konkret di 21 titik panggil. `go vet` + staticcheck + golangci bersih |
| §1.5 batas layer | `service` tidak mengimpor `net/http`; `repository` tidak mengimpor `service`; tidak ada import lintas-service baru |
| §1.6 ketahanan | Setiap panggilan keluar di bawah context dengan deadline (fetch 25s di sweep, 20s di `quotafetch`); worker dijalankan lewat `runSupervised` (pulih dari panic); retry policy stated di §4a; tidak ada goroutine tanpa kondisi berhenti |
| §1.7 disiplin query | Satu baca batch untuk seluruh halaman (dijaga test route), index baru ikut migrasi, `LIMIT` di sweep dan seed, tidak ada `SELECT *` di jalur request |
| §2.1 test | Tanpa `t.Skip`, tanpa filter `-run` yang di-commit; `-race` untuk paket yang menyentuh channel/shared state; test wiring F6 dibuktikan gagal kalau wiring dilepas |
| §1.9 sinkronisasi dokumen | `SYSTEM_MAP.md` §3.7 + tabel worker diperbarui di pass ini; SPEC-API §7.12 dan SPEC-UI §6.6/§14 diamendemen; kontrak YAML + `openapi.json` + tabel rute MD berubah bersama kode. **Graphify: N/A dengan bukti**. `app-serv/go.mod`, `app-serv/go.sum`, `app-ui/package.json` tidak berubah di pass ini, jadi tidak ada tepi dependensi modul baru; yang berubah adalah berkas di dalam paket yang sudah ada |

## 9. Catatan operasi: apa yang dijalankan, apa yang disentuh, apa yang menunggu keputusan owner

**Harness verifikasi.** `app-serv` final di `:9091` (Postgres + Redis milik operator, `HTTP_ADDR`
di-override), panel dev `vite` di `:3001` dengan `PANEL_API_TARGET=http://127.0.0.1:9091`, Chrome
headless dikendalikan lewat CDP di `:9333` (satu login, cookie `pannel_session`, tanpa nilai kredensial
yang dicetak). Semua proses ini sudah dihentikan setelah pengukuran; `:9090` dan `:3000` milik operator
tidak pernah disentuh selama pengukuran ulang.

**Dua kali `pkill -f` menjadi insiden, dan keduanya pelajaran yang sama.** Yang pertama mematikan
gateway operator di `:9090` (dipulihkan <1 menit dari `app-serv/.env` yang sama); yang kedua, di pass
ini, pola `remote-debugging-port=9333` ternyata cocok dengan baris perintah shell-nya sendiri dan
membunuh perintah yang sedang berjalan. Sejak itu semua penghentian dilakukan **berdasarkan PID**, dan
proses Chrome berumur tujuh hari milik operator (pid 4003232) sengaja dibiarkan karena bukan milik
sesi ini. Aturan yang berlaku: `pkill -f` tidak boleh dipakai di pohon ini saat layanan operator hidup.

**Cara membaca hasil gerbang itu sendiri bisa memalsukannya.** Rantai terakhir menjalankan
`go test -race ./internal/... ./cmd/... | grep -aE "^ok|^FAIL"` lalu `echo "raceall rc=$?"`, dan
`$?` di situ adalah status **grep**, bukan test. Rapor itu berbunyi `raceall rc=0` tepat di bawah
`FAIL internal/router 600,151s`. Kalau angka itu yang dipercaya, pass ini akan mencatat hijau yang
tidak pernah ada. Aturan yang berlaku sekarang: exit code diambil dari perintah test itu sendiri
(tanpa pipe), dan keluaran yang sudah disaring hanya dipakai untuk membaca detail.

Keturunan dari species yang sama terjadi saat membuktikan non-vakuitas F16: mutasi pertama saya
(`Failed:  result.Failed,` dengan dua spasi) tidak cocok karena gofmt sudah menyelaraskannya ke satu
spasi, jadi test "tetap hijau" padahal tidak ada yang diubah, persis green-check yang tidak memeriksa
apa pun. Mutasi yang benar membuat test gagal dengan pesan yang tepat. Aturan yang dipakai sejak itu:
setelah menulis mutasi, verifikasi ia benar-benar mengganti sesuatu (assert pada teks sumber), sebelum
mempercayai hasil testnya.

Angka 600,151s juga bukan sekadar gagal: itu persis timeout default `go test` (10 menit), sementara
paket router sendiri butuh 434s dan 560s saat dijalankan tanpa beban. Jadi yang terjadi adalah
timeout yang saya sebabkan sendiri (saya menjalankan set integration berbarengan dengan rantai).
Verdict router dinyatakan **belum diketahui**, bukan "gagal assertion", sampai paket itu
dijalankan ulang sendirian dengan `-timeout` yang lebih longgar.

**Satu pelajaran operasi lagi: gerbang berat tidak boleh ditumpuk, dan hasilnya bisa menipu.**
Dua sapuan `go-lint.sh` sempat berjalan bersamaan; `golangci-lint` keluar dengan
`Error: parallel golangci-lint is running` dan gerbangnya FAIL. Itu bukan temuan kode, dan kalau
dibaca sebagai temuan, pass ini akan "memperbaiki" sesuatu yang tidak rusak. Karena itu `golangci-lint`
dijalankan ulang sendirian di tree yang sama: **0 issues, PASS**. Aturan yang dipakai sekarang: satu
gerbang berat pada satu waktu, dan hasil FAIL dari alat yang tahu dirinya sedang bertabrakan diperlakukan
sebagai hasil yang belum ada sampai diulang dalam keadaan sepi.

**Yang menunggu keputusan owner, bukan keputusan kode.**
1. **`:9090` masih menjalankan build jam 14:01 dari working tree ini**, bukan build final. Yang
   berbeda sejak itu: wiring `ProviderSpecificData` (F6), tiga berkas test baru (F11/F12), field
   `failures`/`last_attempt_at` (F12), dan empat berkas yang dipecah. Restart adalah aksi pada service
   hidup milik operator, jadi tidak dilakukan tanpa diminta.
2. **`github` auth-type vs endpoint kuotanya (F7)**: perlu keputusan registry, bukan kode.
3. **Margin test probe provider (F9)**: satu baris (`+20ms` → mis. `+2s`) di pass lain; sudah
   menangkap satu false positive nyata di mesin dua core.
4. **Keluarga OAuth tanpa akun di deployment ini** (claude, antigravity, gemini-cli, github, grok-cli,
   zed): bentuk request-nya terbukti di stub, jawaban providernya belum, dan sekarang ada test yang
   membuktikan alamat deklarasinya benar (F11), yang sebelumnya tidak ada.

## 10. Audit penutup: setiap butir rencana terhadap bukti di pohon

Rencana ini punya empat cacat, empat keputusan owner, enam langkah (A-F), satu daftar verifikasi, dan
tiga risiko. Tabel ini memetakan masing-masing ke bukti yang bisa diperiksa orang lain, bukan ke
klaim saya.

| Butir rencana | Bukti di pohon / terukur |
| --- | --- |
| Cacat 1: 14 keluarga tanpa fetcher | `familyFetchers` = 18 kunci; `parity_test.go` dua arah (fetcher→registry dan `usage: true`→fetcher) hijau; 14 berkas keluarga baru + testnya masing-masing |
| Cacat 2: alamat usage tidak pernah delivered | `UsageEndpoints` (17 member, sama lebar dengan `registry.UsageConfig`, dijaga `TestUsageBlockAndMirrorHaveEqualWidth` lewat `reflect.NumField`); `grep Transport.Usage.URL` di luar mapper = nol; kunci deklarat yang dibaca produksi dipindah-oleh-test untuk antigravity, claude, gemini-cli, grok (F11) |
| Cacat 3: `fetchVercel` tak terjangkau | Berkas dihapus (`git rm`), alasan + opsi yang ditolak dicatat di §3 F3; `vercel-ai-gateway` masuk daftar residu §6.4 karena KEEP-set adalah keputusan owner |
| Cacat 4: angka provider di balik tombol, baris tanpa langit-langit | PORT 004 §7.2 mengukur **0 bar**; layar yang sama hari ini mengukur **9 bar** dan **5 blok provider** tanpa satu pun tombol ditekan (§7) |
| Keputusan: port semua 14 termasuk OAuth | Terpenuhi; yang TIDAK terbukti adalah jawaban live provider OAuth (deployment ini tidak punya akunnya), dinyatakan apa adanya di §7, bukan diklaim hijau |
| Keputusan: worker + tabel cache | Migrasi 000013 (2 tabel, index pada `next_attempt_at` dan `fetched_at`), `quota_published_worker.go` + policy + store + seed, started lewat `runSupervised("published quota poll", …)` |
| Keputusan: angka provider memimpin kartu | `QuotaCardBody` merender blok provider sebelum baris hitungan; screenshot 1440 dan 390 membuktikan urutannya |
| Keputusan: no-N+1 tetap mengikat | `TestQuotaHandler_ListProviderReadIsOneQueryPerRoute` (tepat satu baca cache per halaman, tiga akun), test repository "satu statement per batch", dan `DueForRefresh` tetap satu statement setelah F14 |
| Langkah A plumbing + tripwire | §3 F1/F2 + baris F11 di tabel gerbang |
| Langkah B vercel | §3 F3, §6.4 |
| Langkah C 7 langkah port | Berkas per keluarga di `quotafetch/`, semua <250 baris dengan header §1.2; kebijakan gagal per keluarga dipertahankan lewat F12 (bukan disatukan) |
| Langkah D migrasi/repo/worker/baca/contract | Baris gerbang §5; budget & concurrency kini config-injected (F13); cooldown dihormati (F14) |
| Langkah E kartu UI | §7 (diukur dari DOM nyata), plus `quota-geometry.ts` sebagai satu sumber aturan persen/warna |
| Langkah F dokumen | `SYSTEM_MAP.md` §3.7 + baris worker; SPEC-API §7.12; SPEC-UI §6.6/§14 (26,27,28); CONTRACT yaml + `openapi.json` + tabel rute MD; pass record ini |
| Verifikasi: `gofmt`/`vet`/`build` | `go-lint.sh` rc=0 di tree F16 (vet ±tagged, gofmt, staticcheck ±tagged, golangci **0 issues**, tanpa berkas ≥250 baris) |
| Verifikasi: `-race` paket kuota lalu seluruh modul | **Hijau di tree F16, diukur solo.** `./internal/... ./cmd/...` → 16 paket `ok`, nol FAIL, nol panic (`router 230,19s`, `service 67,95s`, `handler`, `quotafetch 4,85s`, dst). Dua paket tidak tercakup pola itu dan dijalankan terpisah: `tools/openapi-gen` **ok 3,466s** (yang ini penting karena F12 mengubah YAML yang dibacanya) dan `migrations` tanpa test di build non-tagged. Testnya hidup di bawah tag integration dan hijau di sana (`ok 2,188s`). Catatan kejujuran: rantai saya menulis `./internal/... ./cmd/...`, bukan `./...`, jadi klaim "modul penuh" baru benar setelah dua paket di atas ditutup terpisah |
| Verifikasi: test per keluarga, satu panggilan, path, tanpa kredensial di log | Tiap `*_test.go` keluarga; `grep` atas `logger.*` di berkas baru = nol field berkredensial |
| Verifikasi: test yang **harus ada** (usageEndpoints, parity dua arah, sweep jam palsu, prune label, satu query published per halaman) | Kelimanya ada dan hijau; prune label juga dibuktikan oleh invarian DB produksi (0 yatim) |
| Verifikasi: migrasi `down` bersih + invariant role | `TestPublishedQuotaDownDropsBothTablesAndReapplies` dan `TestApply_EveryTableBelongsToTheAppRole`, keduanya integration dan hijau di tree final |
| Verifikasi: gerbang kontrak/lint/headers/secrets | rc=0 keempatnya (§5) |
| Verifikasi: panel `test`/`check`/`lint`/`lint:ts`/`build` | **Semua hijau di tree F16, diukur solo dan berurutan**: `check`/`lint`/`lint:ts`/`build` rc=0, lalu suite penuh **179 berkas / 2.933 test lulus semua** (rc=0, exit code dari perintah test itu sendiri) |
| Verifikasi: click-through hidup sebagai satu-satunya bukti keluhan owner tertutup | §7: baris nyata, tanpa tombol, 390 px bersih, dan dua cacat (F8, F10) yang hanya bisa terlihat dengan membuka layarnya |
| Risiko 1 label churn | Prune di repository + test prune + 0 baris yatim di DB produksi |
| Risiko 2 herd sweep | Budget + floor per keluarga + `RateLimitedUntil` (F14) + jitter; jitter terlihat di data nyata (`122,139,147,134,146` detik) |
| Risiko 3 OAuth butuh akun live | Dinyatakan sebagai belum terbukti di §7 dan §9.4, bukan ditutupi test stub |

Semua baris di atas terisi dari keluaran yang diukur, bukan dari ingatan. Yang tersisa sebagai
keputusan owner (bukan sebagai pekerjaan yang belum selesai) ada di §9: restart `:9090` ke build
final, `github` auth-type (F7), margin test probe provider (F9), dan `-timeout` pada gerbang
`go-test.sh` (F15).
