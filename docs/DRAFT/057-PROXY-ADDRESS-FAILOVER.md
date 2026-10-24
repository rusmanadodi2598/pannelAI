# 057-PROXY-ADDRESS-FAILOVER.md: kuota free tier dihitung per alamat, dan walk proxy belum memakainya

Permintaan owner: "Biar round robin bisa jalan di semua proxy sehingga mekanismenya benar-benar bekerja
merotasi proxy yang ada, supaya panggilan ke upstream lebih stabil dan rate limit OpenCode Free tertangani."

| | |
|---|---|
| **Status** | **MENDARAT 2026-10-15.** Keputusan owner: M2 (failover pada 429 per alamat, park mengikuti `Retry-After`) + M1 (catat kandidat yang melayani). M3 (satu sesi per kandidat) ditolak karena kuotanya bukan sesi. Draft 051 §4 (store sesi bermutex) tidak jadi dibangun; alasannya di §5 |
| **Mechanism** | BEFORE: walk proxy hanya berpindah kandidat bila connect stage mati; 429 dari upstream dikembalikan ke klien sementara sepuluh alamat lain masih punya sisa kuota. AFTER: 429 adalah keputusan walk, kandidat yang menolak di-skip selama wait yang ia sendiri nyatakan, dan kandidat yang melayani dicatat |
| **Scope** | `app-serv/internal/dataplane/proxy_route.go`, `proxy_route_walk.go` (baru), `transport.go`, `internal/service/proxy_route.go`, `internal/repository/proxy_route.go`, `cmd/app-serv/proxy_wiring.go`, `docs/SPEC-API/001-SPEC-API.md` §7.11 |
| **Permintaan owner** | rotasi proxy yang benar-benar bekerja, untuk meredam 429 OpenCode Free |
| **Kaitan** | draft 051 (identitas sesi + asumsi kuota per sesi), draft 056 (macetnya gerbang sesi), draft 050 (parking & cooldown), docs/PORT/008 D6-D7, docs/PORT/009 D4, SPEC-API §7.11 (kolam proxy), §7.5 (walk) |
| **Tanggal** | 2026-10-15 |

---

## 1. Ringkasan

Yang bikin 429 OpenCode Free tidak hilang bukan sesi, dan bukan juga jumlah proxy di kolam. Penyebabnya:
walk proxy hanya menganggap kegagalan connect sebagai alasan pindah alamat. Ketika satu alamat menjawab
429, gateway menyerahkan penolakan itu ke klien, padahal alamat lain di kolam yang sama masih punya
jatah. Bukurnya ada, pintunya dikunci dari dalam.

Pengukuran live (kolam berisi 11 kandidat Webshare, satu sesi, satu model):

| # | yang diubah | alamat | hasil |
|---|---|---|---|
| 1 | digempur sampai menolak | `84.247.60.125` | 429 |
| 2 | dibiarkan sama sekali tidak ikut burst | `23.26.71.145` | 200, 200, 200 |
| 3 | cek ulang alamat nomor 1, 11 detik kemudian | `84.247.60.125` | masih 429 |
| 4 | sesi BARU di alamat yang sedang ditolak | `84.247.60.125` | 429 |
| 5 | sesi LAMA di alamat yang baru dipakai | 3 alamat berbeda | 429 (saat kolam sedang penuh) |

Sel 1-3 adalah A/B yang bersih: satu detik yang sama, sesi sama, model sama, hanya alamat yang berbeda,
dan jawabannya berbeda. Sel 4 mematikan hipotesis sesi. Jadi unit kuota adalah **alamat egress**, dengan
jendela yang meluruh (semua kombinasi pulih ke 200 setelah ±90 detik tanpa traffic).

Konsekuensi yang harus diterima: `transport_sessions.go` menulis "the OpenCode free tier throttles per
session" dan `opencode_session.go` menulis "the free tier accounts quota per session". Keduanya salah
sebagai pernyataan kuota. Gerbang per-sesi tetap ada dan tetap benar untuk **konkurensi**, hanya bukan itu
yang menentukan 429. Klaimnya sudah dikoreksi di ketiga tempat (`transport_sessions.go:8`,
`provider/opencode_session.go:7`, `provider/plugin.go:101`).

## 2. Yang mendarat

**D1. 429 adalah milik walk, bukan milik klien.** `classifyAddressRefusal` (`proxy_route_walk.go`) membaca
satu status saja. Alasan pemilihannya: 429 menyebut alamat dan ditolak sebelum apa pun dibangkitkan, jadi
mengirim ulang aman; 403 (gate free tier), 404, dan 5xx menyebut request atau vendor, dan setiap kandidat
akan menjawab sama. `proxy_route.go` menutup body kandidat yang menolak, mem-park-nya, lalu lanjut ke
kandidat berikutnya.

**D2. Park selama wait yang dinyatakan sendiri.** `ReportFailure` kini menerima `ttl` (`ProxyRoutePlanner`,
`service.ProxyRouteService`, adapter `cmd/app-serv/proxy_wiring.go`). Kegagalan connect tetap memakai
`ProxyFailureCooldown` dua menit; penolakan 429 memakai `Retry-After` upstream bila ada, dipangkas ke
cooldown yang sama supaya header yang menyebut satu hari tidak menghapus satu alamat yang sehat selama
satu hari. Di lane yang kita ukur upstream tidak mengirim `Retry-After` sama sekali, jadi default itu yang
bekerja; jalurnya tetap dibaca karena vendor lain mengirimnya.

**D3. Yang tidak boleh diulang.** Walk berhenti pada kandidat terakhir dan mengembalikan response 429 apa
adanya (status, header, body), bukan mengarang error gateway. Dan kalau body tidak bisa dibangun ulang
(`request.GetBody == nil`, bentuknya body stream dari klien), request tidak pernah dikirim kedua kalinya:
penolakan tetap diserahkan, alamatnya tetap di-park. Ini menjaga aturan lama §7.11: permintaan yang mungkin
sudah sampai di wire bukan bahan failover.

**D4. Jejak (M1).** `noteServed` mencatat kandidat yang melayani, host tujuan, status, dan berapa kandidat
yang dilewati. Kasus normal masuk DEBUG; walk yang harus melompati alamat masuk WARN dengan id yang
dilewati. Sebelumnya tidak ada satu pun tempat di proses yang bisa menjawab "proxy mana yang membawa
request ini", sehingga perubahan rotasi tidak bisa diverifikasi dari data.

**D5. Pemecahan file.** `proxy_route.go` naik di atas ambang lunak 200 baris (§1.1) kalau semua keputusan
walk tinggal di situ, jadi `proxy_route_walk.go` memisahkan "membaca jawaban" dari "melakukan dial":
`addressRefusal`, `classifyAddressRefusal`, `addressParkFor`, `noteServed`, dan yang memang sudah seharusnya
di sana `proxyConnectRejected`, `rejectConnectResponse`, `isProxyConnectFailure`.

## 3. Rotasi: kenapa tadi tidak jalan, dan sekarang

`candidateOrder()` memutar daftar lebih dulu, lalu **setelahnya** menggeser baris pin ke depan
(`service/proxy_route_order.go:63-73`). Selama `provider_proxies.opencode.pool_id` terisi, pin itulah yang
selalu menjawab lebih dulu dan walk berhenti pada keberhasilan pertama, jadi rotasi tidak pernah terlihat.
Pin `opencode` dilepas (config, bukan kode) dan counter rotasi Redis terbukti bergerak 5 -> 6 -> 7 dengan
pemimpin berbeda per request. `MaxProxyRouteAttempts` tetap 3 (PORT 008 D7): dengan failover 429 ini yang
membatasi sebaran per request, dan angka itu belum saya ubah karena belum ada bukti yang menuntutnya.

## 4. Test

`internal/dataplane/proxy_address_failover_test.go`, httptest sebagai dua alamat: 429 lalu 200 (dan
sebaliknya), `Retry-After` 30 dipatuhi, 900 dipangkas ke cooldown, 500 TIDAK berpindah dan tidak mem-park,
dua-duanya 429 mengembalikan response terakhir utuh + dua park, body tanpa `GetBody` tidak diulang, dan
dua test jejak (WARN berisi skip, DEBUG bersih). Tabel `TestClassifyAddressRefusal` mengunci pemilahan
status itu.
`go test -race` hijau di `dataplane`, `service`, `cmd/app-serv`, `repository/redis`, `domain`.

## 5. Yang tidak jadi, dan mengapa

**Store sesi bermutex dari draft 051 §4** (kapasitas 1000, eviction `usedAt`, TTL 2 jam, sweep di dalam
mint). Sesi bukan unit kuota, jadi rotasi sesi tidak memangkas satu 429 pun. Yang tersisa dari ide itu
adalah concurrency (satu sesi = satu gate = satu call di seluruh lane), dan itu M3: butuh memindahkan
keputusan rute ke sebelum header sesi ditempel dan sebelum slot gate diambil. Owner memilih tidak membangun
M3 sekarang. Kalau nanti lane butuh paralelisme lebih dari satu call, M3 adalah pintunya, bukan store ini.

**Kolom durable per kandidat di `GET /proxies`** (`served_calls`, `last_served_at`). Butuh migration
AGENTS §1.7 plus jalur baca baru di service/handler/schema; jejak log menutup kebutuhan "apakah rotasi dan
failover bekerja" hari ini. Dicatat sebagai susulan, bukan disembunyikan.

**Sistem peta.** `SYSTEM_MAP.md` paragraf OpenCode masih menulis store + mutex + TTL sweep +
`x-opencode-request` + `msg_` ascending clock, yang tidak satu pun ada di tree. Itu defect dokumentasi yang
sudah ada sebelum perubahan ini dan saya biarkan: menyentuhnya berarti memutuskan apakah 051 C2 masih akan
dibangun, dan itu keputusan owner.

## 6. Yang masih terbuka

- Jendela pemulihan vendor diukur sebagai "±90 detik tanpa traffic", bukan sebagai angka yang dijamin. Kalau
  upstream mulai mengirim `Retry-After`, D2 sudah memakainya.
- `MaxProxyRouteAttempts = 3` membatasi sebaran; dengan 11 alamat, empat sisanya tidak tersentuh dalam satu
  request. Naikkan hanya kalau ada pengukuran yang menuntutnya.
- Failover 429 menambah satu round trip per alamat yang dilewati, di dalam deadline attempt yang sudah ada;
  belum ada jam walk yang menagihnya (walk budget ada di transport, tidak di dialer). Perlu diukur sebelum
  jadi alasan menambah batas baru.
- Lane berkunci (mis. `opencode-zen`) tidak menulis header identitas apa pun, jadi 429 di lane itu punya
  cerita kuota yang berbeda dan tidak diukur di sini.
