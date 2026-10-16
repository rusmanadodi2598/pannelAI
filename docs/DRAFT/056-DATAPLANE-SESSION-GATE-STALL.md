# 056-DATAPLANE-SESSION-GATE-STALL.md: lane free bisa macet total tanpa jejak, dan hanya restart yang melepaskannya

Permintaan owner: "Bantu check engine untuk status 502, 429, di case combo model (atau yang lain juga),
terjadi macet dan tidak ada health (pemulihan) padahal seharusnya ada mekanisme tersebut. Pada akhirnya BE
harus restart terlebih dahulu. Bukti live: combo `pi-agent` (isinya model free OpenCode) kena 502 TIMEOUT
(tidak pulih) dan 429 (tidak pulih); setelah app-serv restart, semua model bekerja lagi."

| | |
|---|---|
| **Status** | **O3 CLOSED 2026-10-15; O1 dibuka dengan bentuk yang direvisi.** O3 mendarat: waktu tunggu antrian jadi peristiwa yang tercatat, dan permintaan yang mati di antrian tidak lagi dilaporkan sebagai upstream yang tidak menjawab. O1 tidak jadi memotong pemegang (lihat §5) |
| **Mechanism** | BEFORE (diagnosis) - belum ada perubahan kode |
| **Scope** | `app-serv/internal/dataplane/transport_sessions.go`, `transport_call.go`, `stream.go`, `selection_health.go`, `app-serv/internal/provider/opencode_session.go` |
| **Kaitan** | draft 051 (sesi free tier & kuota per sesi), draft 050 (parking & account health), draft 052 (klasifikasi kegagalan), SPEC-API §4 (deadline), §7.5 (walk) |
| **Tanggal** | 2026-10-15 |

---

## 1. Ringkasan

Yang dicatat owner - macet, tidak pulih, tidak ada health, restart menyembuhkan - bukan tiga gejala terpisah.
Satu mekanisme menghasilkan keempatnya sekaligus, dan ia tidak ada di database.

`app-serv` memaksa **satu panggilan in-flight per sesi upstream** untuk lane OpenCode free, karena upstream
menghitung kuota per sesi. Id sesi itu adalah fungsi murni dari `(client, endpoint)` - bukan dari percakapan.
Akibatnya satu gate mengatur **seluruh lane**: semua model di combo `pi-agent` yang mengarah ke OpenCode free
antri di satu channel berkapasitas 1.

Gate itu dilepas hanya ketika **body jawaban ditutup**. Stream yang sudah terlanjur menjawab tidak dipotong
clock walk; satu-satunya remnya adalah idle guard, dan idle guard di-reset oleh **setiap read minimal satu
byte**. Jadi satu upstream yang menetes pelan - atau satu klien yang berhenti membaca - menahan gate tanpa
batas waktu. Semua permintaan lain tidak pernah sampai ke upstream: mereka tunggu di antrian lalu mati sesuai
deadline-nya sendiri, sebagai timeout.

Kenapa restart adalah satu-satunya obat: antriannya hidup di memori proses, dan gate tidak punya TTL maupun
sweeper. Kenapa tidak ada health: jalur no-auth memakai endpoint virtual, dan `RecordFailure` keluar lebih awal
untuk endpoint virtual, jadi **tidak ada baris kesehatan apa pun yang bisa pulih** - tidak ada yang di-park,
tidak ada yang di-rotate, tidak ada yang muncul di panel.

## 2. Bukti

**Eksperimen hermetik** (scratch, dijalankan lalu dihapus; server peniru menjawab 200 lalu menetes satu frame
per 50 ms selama 10 detik; ctx tiap panggilan 400 ms):

| Ref | Hasil |
|---|---|
| Id sesi untuk dua percakapan berbeda pada satu lane | **sama** - `ses_e10eb8f9dffeCYvgHjzSU84ZO6`, karena id diturunkan dari `(client, endpoint)` saja |
| Pemegang pertama | slot diambil setelah **1 ms** |
| Permintaan kedua, sesi sama, pemegang masih terbuka | `UPSTREAM_TIMEOUT: the upstream did not answer in time: context deadline exceeded` setelah **400 ms** - tanpa satu bayt pun meninggalkan gateway |
| Permintaan ketiga, segera setelah pemegang melepas | **sukses, 0 ms** |

Baris pertama dan terakhir adalah yang penting: yang macet bukan upstream-nya, tapi antrian lokal; dan ia
lepas seketika ketika gate dibuka, bukan ketika sesuatu "pulih".

**Lacak kode:**

| # | Tempat | Fakta |
|---|---|---|
| K1 | `provider/opencode_session.go:54-66` | `OpenCodeSessionFor(clientTool, endpointID)` → SHA-256 dari pasangan itu saja. Tidak ada percakapan, tidak ada pemanggil, tidak ada permintaan |
| K2 | `transport_sessions.go:25-31` | `gates map[string]chan struct{}`; komentar dokumennya sendiri menulis "kept for the life of the process". Tidak ada TTL, tidak ada sweep, tidak ada batas kedalaman antrian |
| K3 | `transport_sessions.go:43-65` | `acquire` mengirim ke channel kap-1; yang membebaskan penunggu hanyalah `<-gate` milik pemegang, atau `ctx.Done()` penunggu itu sendiri |
| K4 | `transport_call.go:64-73,155-163` | slot diambil **sebelum** loop retry dan hands-off ke `sessionBody`; `release()` berjalan saat body ditutup |
| K5 | `transport_call.go:98-101` | "A streamed client keeps no context deadline: the idle guard owns the stream once it has spoken" - setelah jawaban mulai, clock walk sengaja tidak memotong |
| K6 | `stream.go:70-121` | idle guard reset di setiap read ≥1 byte, dan `fired` latch per body |
| K7 | `selection_health.go:119-124` | endpoint virtual: `RecordFailure` return lebih awal. Tidak ada park, tidak ada parity state, tidak ada angka pemulihan |
| K8 | `transport_sessions.go` | `grep logx.\|metrics` pada file ini: **nol**. Waktu tunggu hanya dikembalikan ke budget (`call.Budget.pause`, :93) dan tidak pernah dicatat |

## 3. Temuan

| # | Tingkat | Isi |
|---|---|---|
| **G1** | CRITICAL | Satu gate untuk seluruh lane (K1+K2): kegagalan satu stream menjadi kegagalan semua model free di combo, tanpa batas waktu selama body pemegang belum ditutup (K4+K5+K6) |
| **G2** | HIGH | Tidak ada health untuk dipulihkan pada lane ini (K7). Panel tidak akan pernah menampilkan apa pun, dan tidak ada aksi rotate/park yang bisa mengakhiri kebuntuan - inilah "tidak ada pemulihan" yang owner lihat |
| **G3** | HIGH | Kebuntuan tidak terobservasi (K8). Yang tercatat hanya hasil akhir tiap permintaan (`UPSTREAM_TIMEOUT`), bukan fakta bahwa ia tidak pernah memanggil upstream. Dari luar, gateway tampak timeout karena upstream lambat |
| **G4** | MEDIUM | 429 memperparah, bukan memulihkan. Tidur retry terjadi **di dalam** masa pegangan gate (K4), selama `Retry-After` masih muat di budget walk (`boundedWait`, `transport_call.go:148-154`). Satu `Retry-After: 60` berarti lane berhenti 60 detik untuk satu permintaan, dan setiap penunggu di belakangnya membayar jatah yang sama bergantian |
| **G5** | MEDIUM | Kombinasi yang mematikan justru ada di bentuk pemakaian nyata: combo berisi model free OpenCode, dipanggil agen coding yang membuka beberapa stream sekaligus. Serialization ini dibuat untuk mencegah request menumpuk di upstream (alasan di `transport_sessions_test.go` mencatat "5 parallel requests, ~7s TTFB, observed"); ia berhasil mencegah tumpukan, dan menggantinya dengan satu antrian global tanpa batas |

## 4. Yang tidak saya klaim

- Kejadian produksi owner belum direproduksi. E1 adalah mekanisme, bukan rekaman insidennya. Yang cocok
  adalah bentuknya: timeout untuk semua, tanpa jejak kesehatan, dan sembuh oleh restart.
- Tidak ada bukti satu pemegang menahan gate sampai sekarang di proses owner. Untuk itu perlu pengukuran di
  proses yang hidup (lihat §6).
- `idleGuard.fired` latch (K6) per body, bukan per proses. Ia bukan penjelasan restart-menyembuhkan, dan saya
  tidak membawanya sebagai temuan.

## 5. Arah perbaikan (owner yang memilih)

| Opsi | Isi | Harga | Status |
|---|---|---|---|
| **O3 - buat kebuntuan terlihat** | Catat: waktu tunggu gate, panjang antrian, dan "permintaan mati sebelum pernah memanggil upstream" sebagai peristiwa sendiri | Tidak memperbaiki apa pun; membuatnya bisa didiagnosis | **MENDARAT** (§5a) |
| **O1 - batas antrian, bukan batas pemegang** | Revisi bentuk, lihat §5b | Butuh angka batas antrian | **DIBUKA** |
| O2 - sesi per percakapan | Turunkan id dari percakapan/pemanggil | Mengembalikan request menumpuk ke upstream yang menghitung kuota per sesi (G5) | Ditolak draft ini |
| O4 - health in-memory untuk lane virtual | Park per endpoint ber-TTL khusus lane no-auth | Restart masih menguapkannya | Tertunda di belakang O1 |

### 5a. O3 yang mendarat

| # | File | Isi |
|---|---|---|
| 1 | `transport_sessions.go` | `sessionQueue` mencatatberapa penunggu saat ia tiba (`queued`), dan `noteWait` menulis satu baris Warn dengan `provider_id`, `endpoint_id`, `waited_ms`, `queued_calls`. Di bawah ambang dan tanpa error: tidak menulis apa pun, supaya lane sehat tidak menghasilkan satu baris per permintaan |
| 2 | `transport_sessions.go` | `sessionWaitTimeout` - error antrian dengan pesan yang menyebut antrean gateway. Code-nya tetap `UPSTREAM_TIMEOUT` karena klien bercabang di situ (§2.4 Contract Immutability); yang berubah hanya pesannya, dan itu bukan breaking change |
| 3 | `transport_call.go` | Jalur kegagalan `acquireSession` tidak lagi memakai `timeoutError`, yang berbunyi "the upstream did not answer in time" untuk permintaan yang tidak pernah keluar |
| 4 | `transport.go`, `cmd/app-serv/dataplane_wiring.go` | `TransportDeps.Logger`, disambung ke logger proses. Nil jatuh ke logger default, jadi test lama tidak perlu ikut berubah |
| 5 | test | `transport_sessions_stall_test.go` (baru). `transport_sessions_test.go` dipecah jadi tiga file karena sudah 407 baris dan §1.1 menghitung file yang disentuh |

Ambang `sessionWaitWarnBounds = 2s`: antrian sehat menyerahkan slot dalam orde mikrodetik, jadi 2 detik sudah
merupakan tanda satu panggilan tertahan di belakang jawaban yang belum selesai.

### 5b. Kenapa O1 tidak lagi memotong pemegang

Rencana awal O1 adalah jam lepas-paksa yang menutup body pemegang setelah `T`. Setelah §2 dibaca ulang,
rencana itu salah sasaran: satu-satunya pemegang yang bisa bertahan tanpa batas adalah stream yang **masih
mengalir** (idle guard direset oleh byte apa pun, K6), dan SPEC-API §4 menyatakan stream tidak punya total
cap justru karena jawaban panjang bukan kegagalan. Memotongnya berarti merusak kontrak demi kasus yang jarang.

Yang bisa macet tanpa melanggar kontrak adalah **antrian di belakangnya**. Maka O1 berubah bentuk: batas
jumlah penunggu per sesi, dan penolakan cepat dengan `Retry-After` ketika batas itu penuh - bukan menjawab
lambat lalu mati sendiri. Permintaan yang ditolak cepat tetap dapat diulang klien, lane yang macet berhenti
menular ke permintaan lain, dan tidak ada jawaban sah yang pernah dipotong.

## 6. Test yang sudah mendarat untuk O3

- `TestTransport_QueueTimeoutNamesTheGatewayNotTheUpstream`: satu panggilan memegang sesi, satu lagi mati di
  antrian → code tetap `UPSTREAM_TIMEOUT`, pesan menyebut antrean, dan secara khusus diuji bahwa pesan **tidak**
  menyalahkan upstream; baris log-nya harus ada dan membawa `provider_id`, `waited_ms`, `queued_calls`.
- `TestSessionQueue_NoteWaitWritesOnlyWhatMatters`: slot tanpa antrian tidak menulis apa pun; antrian di atas
  ambang menulis satu baris; penunggu yang menyerah menulis meski cepat. `queued_calls` dibaca dari kedalaman
  yang dilihat penunggu saat ia tiba - setelah antrian ia selalu nol, dan itu justru angka yang test ini jaga.

`go test ./...` hijau, `go test -race ./internal/dataplane/` hijau, `go-lint.sh` PASS (vet, gofmt,
staticcheck, golangci-lint 0 issues, batas baris §1.1), antislop PASS.

## 7. Yang belum diukur

- Insiden produksi owner belum direproduksi. §3 adalah mekanisme, bukan rekaman kejadiannya.
- Berapa panjang antrian yang benar-benar terjadi saat lane macet. O3 baru dipasang hari ini, jadi angkanya
  belum ada. Batas O1 harus dibaca dari log itu, bukan dari perkiraan.

## 6. Pengukuran yang menutup diagnosis ini di proses nyata

1. `GET /api/v1/models` terhadap combo `pi-agent` sambil satu stream dibiarkan menggantung - kalau semua
   permintaan lain timeout tanpa memanggil upstream, G1 terbukti di produksi.
2. Matikan/mute klien di tengah stream, lalu lihat apakah lane lain ikut mati. Kalau ya: pegangan tidak
   dibatasi oleh kecepatan klien (K4+K5).
3. Baca `Retry-After` yang upstream kirim saat 429 dan bandingkan dengan lama lane macet. Kalau sama: G4.
4. Sebar O3 (waktu tunggu + panjang antrian) dan ulangi satu sesi normal: kalau angkanya nol saat sehat dan
   besar saat macet, diagnosis selesai.

## 7. Catatan pembanding dari reference

Reference memperlakukan limit free tier sebagai **milik pool egress, bukan milik akun**: 429/403 dengan body
berbau limit menandai pool tidak layak, mencoba pool lain, dan menampilkannya sebagai sesuatu yang bisa
dibersihkan di halaman Proxy Fitness (`open-sse/executors/opencode.js:322-334`, `poolScoped: {reason: "ip-limit"}`).
app-serv tidak punya padanannya untuk lane virtual (K7), jadi kegagalan yang di reference terlihat dan bisa
dibersihkan, di sini tidak tercatat sama sekali.
