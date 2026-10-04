# 043-USAGE-COMBO-NODE-FLOW.md: Animasi node menggambar jalur request penuh — Client, Combo, Gateway, Upstream, Response

Dokumen kerja pass ini untuk bagian **Live routing** di halaman `/usage`, dan seam outbound data plane
`app-serv`. Pass ini mengerjakan satu permintaan owner yang disebut langsung: animasi node hari ini hanya
tahu dua hal — gateway di tengah dan satu node per provider terkonfigurasi — sehingga request yang
mengalamat sebuah **model Combo** digambar persis seperti request biasa; nama Combo yang baru saja
divalidasi dan dituliskan ke baris usage dibuang sebelum sampai ke stream. Yang owner minta: nama Combo itu
muncul sebagai node, dan aliran gambar dibaca `Client >> Combo >> Gateway >> Upstream >> Response`. Bukan
kontrak; kontrak tetap `docs/SPEC-API/001-SPEC-API.md` (wire) dan `docs/SPEC-UI/001-SPEC-UI.md` (perilaku
panel). Pola mengikuti `035-USAGE-LIVE-ROW-TABS.md` dan `018-USAGE-DRAWING-SIZING-PARITY.md`: temuan
bernomor F, bukti yang bisa diulang, keputusan di depan implementasi, dan gerbang antislop di akhir.

| | |
| --- | --- |
| **Status** | **DURING selesai, AFTER menyusul (audit).** F1–F4 terimplementasi dan hijau di gerbang otomatis: `go test -race ./...` (18 paket, 0 FAIL), `go vet -tags=integration ./...`, `scrypts/gates/contract-openapi.sh`, `go-headers.sh`, `go-lint.sh` (0 issues), `bun run check` (0 error), suite panel penuh, `bun run lint`. Bukti hidup §8 sudah direkam pada 2026-10-04 terhadap PostgreSQL + Redis nyata dengan stub upstream loopback. Yang belum: audit AFTER |
| **Mechanism** | DURING & AFTER (antislop) |
| **Scope** | `app-serv/.` (domain marker, seam `dataplane.ActiveRequests`, frame `schema/usage_live.go`, store Redis penanda) dan `app-ui/.` (derivation gambar, komponen, baris fakta). Migrasi: tidak ada — `combos` dan kolom `usage_records.combo` sudah ada; yang berubah hanya payload di Redis |
| **Permintaan owner** | (1) Perbaikan dan improvement animasi node. (2) Penambahan NODE FLOW sehingga Combo ikut jadi node: nama Combo muncul ketika client request dengan model Combo. (3) Keputusan layout setelah klarifikasi: Combo di busur LUAR/ATAS gateway, upstream di busur bawah, dua terminal tetap di sumbu vertikal. (4) Semua Combo terkonfigurasi digambar (termasuk yang idle). (5) Jawaban kembali digambar sebagai beam balik selama stream berjalan. (6) Yang diperbaiki dari animasi yang ada: lompat tinggi kotak dan node yang muncul/hilang mendadak |
| **Reference** | `decolua/9router`, `ProviderTopology.js` (ellipse, beam, gateway card) — dipertahankan warnanya dan dosis geraknya; bentuknya diamandemen karena reference tidak punya tahap Combo sama sekali |
| **Kaitan** | SPEC-UI §6.5 (diamandemen pass ini), SPEC-API §7.12 (field `combo` di `active`), §7.7 (semantik Combo); AGENTS.md §1.1, §1.5, §1.9, §2.4 (field tambahan itu additive dan legal di v1); DESIGN.md §11 (lima baris log alasan pass ini); drafts 012 F3, 013 F2, 015 F1, 018 F1/F2, 023 F1/F2, 035 F1/F2 (aturannya dipertahankan); `app-ui/README.md` Recorded decisions; `docs/CONTRACT/001-CONTRACT-API-V1.yaml` `UsageLiveActive`; `SYSTEM_MAP.md` §5 dan §6 |
| **Tanggal** | 2026-10-03 (DURING) |

## 1. Ringkasan

Permintaan owner diukur dulu sebelum satu baris diubah.

| Yang owner sebut | Yang diukur |
| --- | --- |
| Nama Combo tidak muncul di animasi node | **Benar, dan penyebabnya di wire, bukan di panel.** `dataplane` sudah tahu namanya — `resolution.Combo.Name()` dipakai di `engine_relay.go:74` untuk `Outcome.Combo` dan berakhir di `usage_records.combo` — tetapi `dataplane.ActiveRequests.Begin(providerID, endpointID, model)` tidak pernah membawanya, jadi frame `active` hanya berisi provider + model anggota. Panel tidak punya apa pun untuk digambar |
| Aliran hari ini `Gateway >> Upstream` | **Benar.** `UsageTopologyEdges.svelte` menarik satu `<line>` per node, semuanya dari `(50,50)`; tidak ada simpul di sisi masuk request |
| Semua Combo boleh jadi node | **Benar secara data, tidak gratis secara geometri.** Panel tidak pernah memuat daftar Combo di `/usage`; `listCombos()` hanya dipakai tab Combos. Dan busur kedua mengurangi real estate angular setengahnya — lihat F1 |
| Animasi perlu diperbaiki | **Benar, dua hal.** Tinggi kotak di-set inline (`style={height: Npx}`) tanpa transisi sehingga gambar melompat saat jumlah node berubah; dan card muncul/menghilang seketika |

Reference tidak menolong di sini: fork itu menggambar router + provider saja, jadi tidak ada preseden visual
untuk tahap Combo. Yang dipertahankan darinya adalah warna, glow, dan takaran gerak beam.

## 2. Keputusan owner yang dipakai

| | |
| --- | --- |
| **D1** | Combo digambar di busur ATAS gateway, upstream di busur BAWAH, dua terminal (`Client`, `Response`) tetap di sumbu vertikal. Opsi "lingkaran konsentris" dan "tiga lajur horizontal" ditolak |
| **D2** | Semua Combo terkonfigurasi ikut digambar (idle seperti provider idle), dibaca `listCombos({per_page: 100})`. Combo yang sedang in-flight menyala dari field `combo` di frame |
| **D3** | Jawaban kembali digambar dengan beam pada hop `upstream → response` selama request masih in-flight; setelah selesai, jalur mengambil warna `last` yang sudah ada. Tidak ada timer satu-kejadian baru |
| **D4** | Terminal `Client` sederhana: label, dan menyala selama ada request in-flight. Nama gateway key TIDAK ditampilkan (marker tidak membawanya; itu perubahan kontrak kedua) |
| **D5** | `combo` ditambahkan hanya di separuh `active` frame. `recent` tidak ikut berubah: tidak ada yang memintanya, dan baris finished list tidak membacanya dari stream |

## 3. F1 HIGH (FE): busur kedua bisa meruntuhkan gambar kalau posisinya dipilih sembarang

**Fakta.** `nodeShare` (`usage-topology-view.ts`) membandingkan setiap pasang node yang jarak vertikalnya
kurang dari satu baris (`NODE_HEIGHT / height`) lalu mengambil jarak horizontal paling sempit; hasilnya
memakai `min()`. Dua node yang bercermin di sumbu horizontal (`−θ` dan `+θ` pada elips yang sama) punya
**x yang identik**, sehingga `tightest = 0`, `nodeShare = 0`, dan `--u = min(1, 0 × …) = 0` — semua kotak,
kartu gateway, dan stroke beam menyusut ke nol. Test lama "tidak ada dua node yang bersinggungan" lolos
**secara vacuous** di angka itu: node selebar 0px tidak pernah bersinggungan.

**Risiko dan aturan.** Dua busur pada satu elips (jari-jari sama, sudut sama) = gambar kosong. Karena itu
busurnya disjoint: Combo `θ ∈ [−168°, −12°]`, upstream `θ ∈ [12°, 168°]`. Jarak pasangan bercermin menjadi
`2 · RADIUS_Y · sin 12° ≈ 14,1%` dari tinggi kotak, sementara satu baris `30/360 ≈ 8,33%` — margin 1,7×,
dipilih agar tetap aman kalau orang memindahkan `MIN_HEIGHT` nanti. `RADIUS_X` tetap 40: node pada share
maksimum selebar 20% kotak dan berpusat, jadi pusatnya harus minimal 10% dari tiap tepi.

**Rencana DURING.** `usage-topology-geometry.ts` (baru) punya `bandPositions`, `nodeShare`, `boxHeight`,
dan posisi terminal; `usage-topology-view.ts` hanya menentukan state. `MIN_HEIGHT` naik 320 → 360 karena
terminal duduk di luar kedua busur. Tinggi kotak sekarang dihitung dari **total** node kedua busur.

**Kriteria selesai.** Sweep 0–24 upstream × 0–8 Combo assert `nodeShare > 0`, plus sweep lama "tidak ada dua
node bersinggungan pada setiap lebar kotak" (180–1086px) yang sekarang dijalankan dengan node di kedua busur.
Terpenuhi di `tests/schemas/usage-topology-view.test.ts`.

**Konsekuensi yang dinyatakan ke owner.** Membagi 360° menjadi dua busur 156° menggandakan kerapatan sudut.
Terukur (lebar kotak 900px): 4+4 node → 130px; 5+5 → 130px; 6+6 → 124px; 8+8 → 98px; 10+10 → 71px; 12+12 →
62px. Busur Combo karenanya dibatasi **8 node** (`COMBO_MAX_NODES`) dan baris panel menyatakan Combo mana
yang tidak ikut digambar. Busur upstream tidak dibatasi (sudah begitu sebelum pass ini, `per_page: 100`).

## 4. F2 MEDIUM (FE): arah beam diekspresikan oleh urutan titik, bukan oleh keyframe kedua

**Fakta.** `beam-dash` menganimasikan `stroke-dashoffset` ke `-100` pada path yang dinormalkan ke
`pathLength="100"`, sehingga setiap dash bergerak dari **awal** garis ke **akhir**-nya. Semua garis hari ini
berawal di `(50,50)`, jadi beam sudah bergerak gateway → upstream; gambar `Gateway >> Upstream` memang
sudah benar. Yang belum ada: sisi masuk dan sisi keluar.

**Risiko dan aturan.** Lima hop (`client-gateway`, `client-combo`, `combo-gateway`, `gateway-provider`,
`provider-response`) menuntut arah per hop, dan satu id filter per **garis** tidak cukup: upstream aktif
sekarang membawa beam di dua hop sekaligus, sehingga `id="beam-openai"` terduplikasi dan `url(#beam-openai)`
resolver ke yang pertama — beam kedua dapat filter milik yang pertama.

**Rencana DURING.** Hop dipecah ke `UsageTopologyHop.svelte` (satu hop: satu garis, atau beam penuh) dan
`UsageTopologyEdges.svelte` hanya mendaftar hop plus urutannya; `gateway-provider` ditulis dengan geometri
identik dengan hari ini supaya harness (yang mencocokkan garis lewat posisi ujungnya) tetap hijau. Id filter
dijadwalkan per hop: `beam-${hop}:${nodeKey}`. Nama node di-namespace: `provider:${id}` / `combo:${name}`
karena Combo bernama `openai` itu legal (`comboName` menerima `[A-Za-z0-9._-]+`).

**Kriteria selesai.** `tests/components/usage-topology-path.test.ts`: satu hop masuk dan satu hop keluar per
Combo; beam hanya pada Combo yang frame sebut; `provider-response` berangkat dari node (y2 = 96) sementara
`gateway-provider` berangkat dari pusat; tidak ada dua `<filter>` dengan id sama.

## 5. F3 MEDIUM (FE): lompat tinggi dan node yang muncul mendadak

**Fakta.** Kotak gambar menyetel `height` lewat style inline tanpa transisi, dan card di-`{#each}` tanpa
animasi masuk. Sebelum Combo ikut gambarnya hal ini jarang terlihat (jumlah provider praktis statis); dengan
satu busur yang node-nya menyala per request, perubahan tinggi dan kemunculan card jadi kejadian biasa.

**Rencana DURING.** `transition-[height] duration-300 motion-reduce:transition-none` pada kotak, dan
keyframe `node-enter` (opacity 0→1, scale 0.96→1, satu kali jalan) di `motion.css` untuk card node dan
terminal. Panel ini menjaga geraknya **hanya** lewat kelas `motion-reduce:` (tidak ada hook
`prefers-reduced-motion` di JS), jadi sebuah `transition:` Svelte bukan teknik yang kompatibel di sini.
Card yang pergi tetap hilang seketika: tidak ada state untuk dianimasikan keluar karena frame yang menghapus
juga menghapus element-nya. Ini dicatat, bukan diakali.

**Kriteria selesai.** `usage-topology-motion.test.ts` assert kelas `animate-node-enter` +
`motion-reduce:animate-none` pada card node dan kedua terminal, dan transisi tinggi pada kotak.

## 6. F4 HIGH (BE): nama Combo harus menyeberangi Redis, dan pembacanya tidak boleh menghapus apa yang tidak dikenali

**Fakta.** Marker in-flight ditulis sebagai JSON ke sorted set `pannelai:usage:active`; decoder-nya
`DisallowUnknownFields` (alasan OWASP A08: nilai dari Redis bersama = input tak terpercaya) dan
`ActiveRequestStore.Active` **meng-ZREM** anggota yang tidak bisa ia decode. Artinya build lama yang masih
berjalan akan **menghapus** penanda milik build baru yang membawa field `combo` — permintaan hidup proses
lain musnah oleh pembaca yang tidak tahu apa yang ia hapus.

**Rencana DURING.**
- `domain.ActiveRequest` dapat `Combo`; constructor sekarang `NewActiveRequest(ActiveRequestInput, now)`.
  Struct, bukan enam string positional: `Model` dan `Combo` sama-sama opsional dan bertetangga, sehingga
  transpose-nya kompilasi, lolos `Validate()`, tersimpan, dan menggambar label salah. Preseden persis:
  `NewUsageFilter(UsageFilterInput, now)`, `UsageRecordInput`, `RequestLogInput`.
- `json:"combo,omitempty"` — penanda tanpa combo (setiap panggilan media/embeddings/SystemOne, dan chat
  langsung) tersekuen **byte-identik** dengan bentuk yang build lama tulis, jadi jendela mixed-build aman
  untuk trafik mayoritas.
- `Active` sekarang **melewatkan** anggota yang tidak terbaca dan **membiarkannya di set**; prune skor tetap
  membereskannya dalam ≤60 dtk. `DisallowUnknownFields` dipertahankan: anggota asing tetap tidak pernah
  **dikembalikan**.
- Seam melebar ke `dataplane.ActiveMarker{ProviderID,EndpointID,Model,Combo}`; hanya `engine_relay.go` yang
  mengisi Combo (`resolution.Combo.Name()`). `markActiveRequest` di service tetap tiga string dan membangun
  marker dengan Combo kosong — tiga plane lain menolak Combo sebelum menandai apa pun
  (`embeddings_resolve.go:67`, `systemone.go:139`, `media_call.go:127`). Jalur fusion tidak butuh perubahan:
  `engine_fusion.go:148,191` men-stamp `member.Combo` sebelum tiap `relayOnce`, jadi semua marker satu Combo
  membawa satu nama.
- Frame: `schema.UsageLiveActive.Combo` (senantiasa ada, string kosong berarti "tanpa Combo"), kontrak YAML
  `UsageLiveActive` + regenerate `internal/handler/openapi.json`.
- Codec dipecah ke `domain/usage_active_codec.go` mengikuti pemisahan filenya di test, agar kedua file jauh
  dari plafon 250 baris AGENTS.md §1.1.

**Kriteria selesai.** `usage_active_codec_test.go` mempin payload Combo-kosong pada tepat 6 field dan
marker ber-Combo pada 7; `usage_active_store_test.go` punya baris "anggota asing dilewat dan tetap di set"
(tag `integration`); `TestActiveRequestTracker_BeginAndRelease` baris Combo; `engine_active_test.go` assert
marker relay membawa nama Combo; `schema/usage_live_test.go` mempin 5 member `active`; gerbang kontrak dan
parity property-name hijau.

## 7. Non-findings

- **Kolom `combo` di `usage_records`** sudah ada dan sudah ditulis jalur chat (`chat_record.go:91`); tidak ada
  migrasi baru untuk pass ini.
- **Media, embeddings, SystemOne menampilkan Combo** — tidak: ketiganya menolak Combo, jadi busur Combo untuk
  trafik itu kosong adalah jawaban yang benar, bukan lubang.
- **Mengganti elips dengan lajur horizontal** — ditolak owner pada klarifikasi layout; geometri elips dan
  aturan `--u` draft 018 dipertahankan.
- **Kunci payload versioned (`v` atau `:active:v2`)** — ditolak: build lama menolak `v` sama persis seperti ia
  menolak `combo` (tidak membeli apa pun), dan key baru membuat setengah gambar kosong selama jendela
  restart. Yang dipakai: `omitempty` + skip-not-delete.
- **Menampilkan gateway key pada terminal Client** — di luar permintaan; marker tidak membawa
  `gateway_key_id`, jadi itu perubahan kontrak kedua.

## 8. Bukti hidup (2026-10-04)

Pass ini dijalankan terhadap **instance buangan**, bukan data development owner: database
`pannelai_d043_test` (nama dengan `test` karena stack live memotong tabelnya — guard itu sendiri yang
menolak `pannelai_d043`), Redis buangan pada `127.0.0.1:7599`, dan upstream stub di loopback
(`127.0.0.1:8099`, menahan 4 detik) sehingga tidak ada panggilan vendor dan tidak ada kuota yang terpakai.
Data di-seed lewat API manajemen: provider node `openai-compatible` → endpoint + credential → Combo
`live-combo-043` (fallback atas `stub043/stub-model`) → gateway key.

| Yang dibuktikan | Yang terukur |
| --- | --- |
| Nama Combo menyeberangi stream | Frame `active` membawa `{"provider_id":"openai-compatible-03884HF4…","endpoint_id":"ep_03884HKCZT…","model":"stub-model","combo":"live-combo-043","started_at":"2026-10-04T01:35:03Z"}` — Combo dan anggota berdampingan, seperti §7.12 |
| Marker dilepas saat panggilan selesai | Frame berikutnya `active: []` dengan satu baris `recent` berstatus `success`, `tokens_in=7`, `tokens_out=3` |
| Node Combo menyala di busurnya | Screenshot `/tmp/drawing-inflight.png`: `Client` → `live-combo-…` → `Gateway` (badge 1) → `Stub 043` → `Response` semuanya berstatus ok; `OpenCode F…` tetap garis polos |
| Hop langsung tidak berbohong | Hitungan beam per hop saat satu request Combo in-flight: `client-combo` 1, `combo-gateway` 1, `gateway-provider` 1, `provider-response` 1, **`client-gateway` 0** — persis aturan `directCount` di F1 |
| Fakta yang sama tersedia dalam kata-kata | Baris fakta membaca `1 in flight: live-combo-043 → Stub 043 (stub-model) · Last finished: Stub 043`, dan jawaban data plane menggemakan `"model":"live-combo-043"` |
| Gerak berhenti saat stream jeda | Setelah Pause: `line[data-beam=core]` = 0; aturan "warna bertahan, gerak berhenti" tetap dijaga `usage-topology-motion.test.ts` |

Yang **tidak** dibuktikan pass hidup ini: label Combo yang panjang terpotong pada kartu (`live-combo-…`).
Itu aturan lama draft 018 (`max-w-[96px*--u] truncate`), bukan regresi, dan tidak ada tooltip untuknya.
Kalau owner ingin nama Combo penuh terbaca, itu keputusan terpisah tentang lebar label, bukan tentang jalur
request.

## 9. Gerbang

```bash
cd app-serv && go build ./... && go vet ./... && go test -race ./...
cd app-serv && go vet -tags=integration ./...
cd app-serv && go run ./tools/openapi-gen -check
./scrypts/gates/contract-openapi.sh
./scrypts/gates/go-headers.sh
cd app-ui && bun run check && bun run test && bun run lint
```

Non-vacuity (satu per pasangan test): buang stamp `Combo: resolution.Combo.Name()` di
`engine_relay.go` → baris Combo di `engine_active_test.go` harus merah; kembalikan `s.discard()` di
`usage_active_store.go` → baris "anggota asing tetap di set" harus merah; pindahkan satu busur ke sudut yang
sama dengan busur lawannya → assert `nodeShare > 0` harus merah.

**Klik hidup.** Sudah direkam di §8; tidak ada langkah hidup yang tersisa untuk pass ini.

## 10. Residu

- **Mixed-build window.** Penanda ber-Combo yang dibaca build lama ditolak dan (sebelum F4) dihapus. Setelah
  `omitempty` + skip-not-delete, paparan tinggal: Combo marker selama jendela restart. Yang sudah diverifikasi
  pada Redis nyata: anggota yang tidak bisa di-decode dilewat dan **tetap di set**
  (`usage_active_store_foreign_test.go`). Yang belum: dua build berdampingan sungguhan, karena repo ini tidak
  punya manifest deployment (`deployment/` hanya `.gitkeep`).
- **Plafon node.** Di atas ±12 node per busur label tidak terbaca lagi. Busur Combo dibatasi 8; busur
  upstream tidak dibatasi pass ini (kondisi yang sudah ada sebelumnya).
- **Label Combo terpotong.** Kartu membatasi label pada `96px * --u` (aturan draft 018), jadi nama Combo yang
  panjang tampil terpotong dan tidak ada tooltip yang mengungkapkannya. Itu keputusan lebar label yang
  terpisah dari jalur request.
- **Audit AFTER** belum dijalankan; status dokumen tidak boleh dibaca `CLOSED` sebelum itu masuk.
- **Lingkungan buangan pass ini ditinggal:** database `pannelai_d043_test` dan `pannelai_d043` di PostgreSQL
  lokal, berikut gateway key `live 043` di dalamnya. Keduanya tidak menyentuh `pannelai` milik development dan
  dihapus dengan `DROP DATABASE` atas permintaan owner.
