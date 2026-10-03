# 040-PLAYGROUND-MODEL-LIST.md: daftar model klien menyebut yang tidak bisa dirutekan, dan menyembunyikan model yang operator tambahkan

Register penutupan `docs/DRAFT/021-DATAPLANE-SSE-FRAMING.md` §20 (F8 dan F10) dan
`docs/DRAFT/009-PLAYGROUND-CHAT-ENDPOINT-READINESS.md` §10.10, dari permintaan owner untuk
menuntaskan Playground Chat di kedua sisi. Dokumen ini bukan kontrak; kontrak tetap
`docs/SPEC-API/001-SPEC-API.md` §7.15 dan `docs/SPEC-UI/001-SPEC-UI.md` §6.15.

|                      |                                                                                                                                                                                  |
| -------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**           | **CLOSED 2026-10-02.** F1 dan F2 mendarat sebagai `app-serv`; F3 sebagai `app-ui`. Bukti §5                                                                                       |
| **Mechanism**        | DURING (test table-driven lebih dulu, lalu implementasi), AFTER sebagai register ini                                                                                              |
| **Scope**            | `app-serv/internal/dataplane/**` (daftar model), `app-serv/cmd/app-serv/**` (wiring), `app-ui/.env.example`, dokumentasi §7.15/§6.15/SYSTEM_MAP                                    |
| **Permintaan owner** | "Endpoint: Playground Chat, bantu untuk menuntaskan playground chat. Baik di sisi BE (app-serv), maupun disisi FE (app-ui)" lalu "Coba pakai ini: sk-…" (2026-10-02)               |
| **Keputusan owner**  | Ejaan model custom: **prefiks node**; filter aktivitas: **"hanya models yang aktif, dan hanya provider yang aktif saat itu... mirip dengan saat pickup pengisian Combo & Vision"** |
| **Kaitan**           | `internal/dataplane/catalog.go`, `resolve.go`, `model_lookup.go`, `registry_surface.go`; `cmd/app-serv/dataplane_inputs.go`, `dataplane_wiring.go`; `app-ui/src/lib/components/PlaygroundComposer.svelte`, `src/lib/schemas/env.ts` |
| **Tanggal**          | 2026-10-02                                                                                                                                                                          |

## 1. Yang terukur sebelum perubahan

Layar `/playground` hanya punya satu control model: `<select>` yang isinya persis
`GET /api/v1/models` (`PlaygroundComposer.svelte:71-78`, tidak ada jalur teks bebas).
Karena itu daftar itu adalah batas layar: model yang tidak muncul tidak bisa dikirim,
dan model yang muncul tapi tidak bisa dirutekan adalah control yang tampak hidup.

Diukur pada gateway owner yang berjalan di `:9090` dengan kunci gateway aktif:

| Fakta | Nilai |
| --- | --- |
| Baris yang diterbitkan `GET /api/v1/models` | 315, dari 26 provider |
| Baris `models_custom` di PostgreSQL | 7 |
| Baris custom yang muncul di daftar | **0** |
| Provider dengan endpoint `status='active'` | 4 (`qoder`, `codebuddy-intl`, `opencode-zen`, node `th-1`) |
| `th-1/deepseek-v4.1-flash:free` | Tidak terdaftar, tetapi **menjawab 200** lewat `POST /playground/chat` |

Dua arah kesalahan yang sama, yaitu F8 dan F10 di 021 §20, dan keduanya punya satu penyebab:
`Resolver.ModelList` hanya membaca indeks registry (`catalog.go:49`, `:53`) dan tidak pernah
membaca `upstream_endpoints` maupun `models_custom`.

## 2. F1 (HIGH): daftar tidak menggate provider dengan koneksi hidup

**Gejala.** Provider tanpa satu pun endpoint yang akan dipilih selector tetap menerbitkan
seluruh modelnya. Enam model `byteplus` diuji terhadap gateway tambal dan seluruhnya
menjawab `NO_PROVIDER_AVAILABLE`: daftar menyebut model yang router tidak punya tempat untuk
mengirimnya.

**Perbaikan.** `ModelList` kini meminta satu bacaan `ActiveProviders` atas seluruh id
provider, lalu hanya menerbitkan provider yang jawabannya benar **atau** yang tidak butuh
kredensial. Predikatnya bukan kesehatan sesaat dan bukan `endpoint_count` (yang menghitung
semua status): ia pinjaman pertanyaan kandidat selector, supaya jendela backoff tetap
mencantumkan provider sementara endpoint yang error atau disabled tidak. Setengah
tanpa-kredensial dibaca dari registry lewat `Provider.NeedsNoCredential()`, yaitu sifat yang
sama yang membuat selector mensintesis endpoint virtual; tanpa bagian ini lane gratis
hilang dari daftar.

**Yang dipertahankan.** Combo tetap tanpa syarat aktivitas, sesuai 021 §20.2: sebuah combo
dialamat dengan nama dan selector berjalan anggota per request.

## 3. F2 (HIGH): model yang operator tambahkan tidak pernah masuk daftar

**Gejala.** `models_custom` tidak dibaca rute ini sama sekali. Untuk node custom, dampaknya
paling keras: daftar model sebuah node berasal dari fetch live ke upstream node itu
(`cmd/app-serv/provider_index.go:143`), dan fetch yang menjawab 401 meninggalkan node dengan
nol model, padahal operator sudah menulis modelnya sendiri di tabel.

**Perbaikan.** Baris custom di-merge ke himpunan yang sama. Provider id pada baris
dikanonkan lewat indeks (`Provider(name)` menerima id, alias, maupun prefiks), sehingga baris
yang disimpan dengan id node dan baris yang disimpan dengan prefiks node menghasilkan entri
registry yang sama. Baris yang provider-nya tidak dikenal indeks di-skip, tidak dikarang.
Baris yang dobel dengan model deklaratif menjadi satu baris karena kunci himpunan adalah id
akhir.

**Ejaan, dan mengapa keputusan ini perlu.** `catalog.go:61` lama memakai `entry.ID`, sedangkan
yang diketik operator dan yang terbukti menjawab adalah `th-1/deepseek-v4.1-flash:free`. Owner
memilih **ejaan prefiks**. Aturan yang dipakai: `entry.Alias` untuk entri `Custom` (node),
`entry.ID` untuk provider registry. Akibatnya 315 baris registry tidak berubah ejaan, dan
hanya baris node yang kini memakai prefiks. Keduanya resolve; yang prefiks adalah satu-satunya
yang pernah ditulis manusia. `OwnedBy` ikut ejaan yang sama supaya panel tidak menampilkan id
internal di samping id yang bisa dikirim.

**Regresi yang disengaja dan diakui.** `node_models_surfaces_test.go` sebelumnya menagih
model node terbit di bawah `node.ID()`. Tuntutan itu adalah pernyataan ejaan, bukan perilaku,
jadi diubah untuk menagih prefiks `corp/` dengan komentar yang menunjuk keputusan owner. Tanpa
perubahan itu test akan menolak keputusan yang baru saja diterima.

## 4. Bentuk seam, dan mengapa bukan impor lintas layer

`internal/dataplane` tidak bisa mengimpor `internal/service`: service sudah mengimpor dataplane
(`internal/service/media_inworld.go:26`), jadi arah sebaliknya adalah cycle yang ditolak
compiler (AGENTS.md §1.5). Karena itu predikat "provider aktif" masuk sebagai interface sendiri:

- `ModelLookup` (di `resolve.go`) bertambah dua metode: `CustomModels` dan `ActiveProviders`.
- `CatalogLookup` (`model_lookup.go`) mengimplementasikannya; `CustomModels` tidak menambah
  dependensi karena seam ini sudah memegang `ModelCatalogRepository`, `ActiveProviders`
  membaca satu reader baru `ActiveEndpointReader` yang dipenuhi `*postgres.EndpointRepository`
  secara struktural.
- Reader itu **wajib**: `NewCatalogLookup` menolak nil, supaya deployment yang lupa memasang
  tidak menjawab "tidak ada provider yang aktif" padahal yang terjadi adalah wiring kurang.
  Test `catalog_list_read_test.go` menagih tiap bacaan yang gagal menjadi error, bukan daftar pendek.
- Interface `ProviderRegistry` pindah ke `registry_surface.go`. `resolve.go` ada di 249 baris
  sebelum perubahan dan additions ini akan melewatkan batas 250 AGENTS.md §1.1; permukaan
  registry dan aturan resolusi memang dua concern berbeda, jadi dipisah alih-alih komentar
  dipadatkan.

## 5. Bukti

**Test lebih dulu, dan merah karena perilaku.** Setelah seam diperlebar (mekanis, tanpa
perilaku), sebelas kasus `catalog_list_test.go` gagal dengan pesan `listed = [dark/dark-chat
free/free-chat lit/lit-chat], want [...]`, bukan gagal compile: persis dua aturan yang hilang.

| Gate | Hasil |
| --- | --- |
| `go build ./...`, `go vet ./...`, `go vet -tags=integration ./cmd/...` | PASS |
| `go test -count=1 ./...` | PASS, dua kali |
| `go test -race -count=1 ./...` | 1 kegagalan, dan itu flaky yang sudah ada lebih dulu (`provider_model_probe_budget_test.go:33`, lihat §8); bukan data race, dan tidak menyentuh jalur daftar model |
| `scrypts/gates/go-lint.sh` | PASS (vet plain+tagged, gofmt, staticcheck plain+tagged, `golangci-lint` 0 issues) |
| `scrypts/gates/go-headers.sh` | PASS |
| `scrypts/gates/contract-drift.sh`, `contract-openapi.sh` | PASS |
| `git diff --check` | PASS |

`go vet ./...` tidak menangkap satu pun double ber-tag integration; `staticcheck -tags=integration`
menangkap `internal/service/systemone_live_test.go` yang kurang dua metode. Gate lint wajib itu
yang menemukan, bukan review.

**A/B live.** Biner tambal jalan di `:9091` dengan Postgres dan Redis yang sama; gateway owner
`:9090` tidak disentuh dan tidak di-restart. Panel kedua di `:3001` menunjuk `:9091` supaya
jalur layar yang benar-benar dipakai yang diukur, bukan hanya rute gateway.

| Ukur | `:9090` (pra) | `:9091` (pasca) |
| --- | --- | --- |
| Baris `GET /api/v1/models` | 315 | 113 |
| Provider yang disebut | 26 | 6 (`opencode-zen`, `qoder`, `codebuddy-intl`, `opencode`, `th-1`, combo) |
| Baris `th-1/` | 0 | 1 |
| Baris baru | - | `th-1/deepseek-v4.1-flash:free`, `opencode/mimo-v2.6-flash-free`, `opencode/space-bunny-free` |
| 4 model `byteplus` yang di-drop, dikirim sungguhan | - | semuanya `NO_PROVIDER_AVAILABLE`, jadi pengecualian benar |

**Kirim nyata lewat rute panel** (`POST http://127.0.0.1:3001/playground/chat`, model dari isi
picker): `http=200`, ttfb 1,72 s, total 2,92 s, 2131 byte, 11 event, `data: [DONE]` tiba sebagai
event `data:` sendiri yang lengkap, usage chunk `37/19/56`, teks jawaban `pong`.

## 6. F3 (MEDIUM, app-ui): variabel yang layar butuhkan tidak ada di template

`PANEL_PLAYGROUND_KEY` dideklarasikan `src/lib/schemas/env.ts:85` dan dibaca
`src/lib/server/playground-context.ts:16`, tetapi tidak ada di `app-ui/.env.example` satu
baris pun. Operator yang membuka template tidak diberi tahu layar itu butuh kredensial, dan
`/playground` menampilkan "The panel has no gateway key" padahal tidak ada yang salah pada
kodonya. Ditambahkan sebagai `[P1][TAG:PLAYGROUND]` dengan nilai kosong, catatan bahwa ini
gateway key bukan provider key, bahwa kosong adalah state yang didukung, dan bahwa panel harus
di-restart karena `config.ts:13` men-cache env per proses.

`.env` nyata owner juga diisi (nilai tidak dicatat di dokumen ini; lihat §7).
`git check-ignore` mengonfirmasi `app-ui/.env` di-ignore sebelum penulisannya.

## 7. Yang disentuh, dan state setelahnya

Source: `catalog.go`, `resolve.go`, `registry_surface.go` (baru), `model_lookup.go`,
`catalog_list_test.go` (baru), `catalog_list_read_test.go` (baru), `resolve_fixtures_test.go`,
`engine_relay_fixture_test.go`, `resolve_combo_budget_test.go`,
`chat_http_doubles_test.go`, `systemone_live_test.go`, 4 file wiring `cmd/app-serv`,
`node_models_surfaces_test.go`, `provider_index_test.go`, `playground_live_doubles_test.go`,
`app-ui/.env.example`.

Dokumentasi: SPEC-API §7.15 (definisi *routable*), SPEC-UI §6.15 (klaim stream basi dikoreksi;
dua cacat `app-serv` yang masih tercatat "open" ditutup dengan bukti terukur) dan §16,
SYSTEM_MAP §3.2 (sumber bacaan daftar).

Gateway `:9090` dan panel `:3000` owner masih berjalan seperti semula. Proses kedua
(`:9091`, `:3001`) dihentikan dan biner sementara dihapus. Satu request nyata tercatat:
usage dan log baris bertambah untuk panggilan `th-1` yang menjawab.

## 8. Yang tidak ditutup di sini

- **Browser click-through `/playground` masih terbuka.** Yang dikerjakan pass ini adalah rute
  panel lewat HTTP dengan cookie sesi nyata, bukan render di browser. SPEC-UI §6.15 dan README
  panel tetap menagihnya; Playwright tidak terpasang di host ini.
- **Perbedaan status `MODEL_NOT_FOUND`.** **CLOSED 2026-10-03** oleh
  `docs/DRAFT/041-MODEL-NOT-FOUND-STATUS.md`: SPEC-API §7.15 menulis `404`, kontrak YAML dan
  `statusFor` memetakan `400`, dan §8 tidak daftar kodenya sama sekali. Owner memilih sinkron
  ke 404.
- **`internal/schema/messages.go` 260 baris**, di atas AGENTS.md §1.1, pre-existing.
- **Tiga peringatan batas baris** hasil perubahan ini, semua di bawah 250 dan dilaporkan gate
  apa adanya: `resolve.go` 241 (sebelumnya 249, jadi turun), `management_wiring.go` 227,
  `engine_relay_fixture_test.go` 227 (sebelumnya 215, naik karena dua metode double).
- **Satu test `internal/service` yang flaky, sudah ada sebelum perubahan ini, dan terbukti
  begitu di HEAD yang bersih.**
  `TestProviderModelTestService_AProbeThatRunsOutItsBudgetIsATimeout`
  (`provider_model_probe_budget_test.go:33`) gagal pada assertion baris 43
  (`a probe that hit its budget must not report ok`). Mekanismenya di baris 35: budget
  `ProviderModelProbeTimeout = 20 * time.Second` (`provider_model_budget.go:33`) sementara stub
  menjawab pada `timeout + 20 ms` — margin 0,1 %, jadi di bawah perebutan scheduler goroutine
  deadline bisa kalah dan jawaban "yang telat" itu menang sebagai `OK`.
  Host ini 2 CPU, dan `go test -race ./...` menjalankan puluhan biner paket bersamaan.
  Buktinya, diukur pada `git worktree` HEAD `dcbc96d` tanpa satu pun perubahan batch ini:
  full `go test -race ./...` **hijau 2x**, lalu test yang sama **gagal 1 dari 8** jalan dengan
  beban CPU buatan (4 busy-loop pada 2 core). Jadi "pre-existing" bukan argumen, tapi hasil run.
  Tidak ada hubungan dengan daftar model: satu-satunya berkas `internal/service` yang disentuh
  batch ini adalah `systemone_live_test.go`, dan itu ber-tag `integration` sehingga tidak ikut
  dibangun pada run normal. Perbaikan yang murah: lebarkan margin stub (mis. `+ 2 s`); yang
  benar: jam palsu diinjeksikan ke layanan probe. Tidak dikerjakan di batch ini karena ini
  temuan register 037/020, bukan playground.
- **Kegagalan laten pertama `internal/service`** pada jalan gabungan paling awal (sebelum
  perubahan ini) tidak bisa direproduksi pada lima jalan berikutnya; kemungkinan kelas yang
  sama dengan poin di atas, karena paket test jalan paralel dan berbagi Redis host.
- **`GET /api/v1/models` masih fetch daftar model node ke upstream** lewat cache
  (`cmd/app-serv/node_models.go`). F1/F2 membuat hasil fetch tidak lagi menentukan kebenaran
  daftar, jadi fetch itu bukan blocker; owner meminta "jangan fetch" untuk *menyaring*, dan
  penyaringan kini baca `upstream_endpoints`, bukan jaringan. Menghapus fetch sepenuhnya
  adalah pertanyaan terpisah karena permukaan lain (detail provider) juga membacanya.
