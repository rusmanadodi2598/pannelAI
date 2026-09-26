# 033-PROXY-CONNECT-FAILOVER.md: kenapa walk proxy tidak failover saat kandidat menolak CONNECT, dan perbaikannya

Permintaan owner: "Oke, saya sudah add 1 proxy mati, dan kita bisa lanjut testing failover nya,
untuk membuktikan apakah bekerja secara presisi. Sebelum kita memutuskan pekerjaan." Lalu:
"Boleh, lanjut fix aja." Penutupnya: "Ya, sinkronkan docs".

Uji failover live pada baris mati owner menemukan satu kelas kegagalan yang lolos dari classifier
walk: proxy yang **menjawab CONNECT dengan status non-200** (tunnel tidak pernah terbuka). Baris itu
tidak diparkir dan kandidat berikutnya tidak dicoba; request hanya selamat karena retry provider
merencanakan ulang. Kelas ini ditutup di `d1c2a81` dan dibuktikan ulang live pada gateway owner.

|                      |                                                                                                                 |
| -------------------- | --------------------------------------------------------------------------------------------------------------- |
| **Status**           | **CLOSED 2026-09-26** sebagai `d1c2a81` (2 berkas, +190/−1); belum di-push                                      |
| **Mechanism**        | DURING & AFTER (antislop)                                                                                       |
| **Scope**            | `app-serv/internal/dataplane` (satu berkas source diubah, satu berkas uji baru)                                 |
| **Permintaan owner** | buktikan failover bekerja presisi pada baris mati yang ia tambahkan, lalu perbaiki                              |
| **Reference**        | tidak ada; ini cacat internal engine, bukan paritas reference                                                   |
| **Kaitan**           | PORT 008 D6/D7 (parkir + failover terbatas), PORT 009 §7 (tindak lanjut), draft 021 (jalur transport yang sama) |
| **Tanggal**          | 2026-09-26                                                                                                      |

## 1. Ringkasan

PORT 008 D7 menjanjikan "hanya kegagalan connect-stage (dial ke/lewat proxy) yang memicu failover",
dan D6 memarkir kandidat yang gagal connect. Implementasinya (`isProxyConnectFailure`) hanya
mengenali `*net.OpError` atau `net.Error.Timeout()`, dan itu cukup untuk proxy mati di level TCP
(`127.0.0.1:1` di suite) tetapi **tidak** untuk proxy yang TCP-nya hidup lalu menolak tunnel:

1. Go mengembalikan penolakan CONNECT non-200 sebagai error polos `errors.New(text)`
   (`net/http/transport.go:1931-1938`), bukan `*net.OpError`.
2. `isProxyConnectFailure` menjawab `false`, sehingga `ProxyDialer.Do` langsung `return`
   (`proxy_route.go:98-102`): tidak ada `ReportFailure`, tidak ada kandidat berikutnya.
3. Request selamat hanya karena `Transport.Do` meretry (`dataplane/retry.go:36`,
   `DefaultAttempts = 3`) dan retry itu merencanakan ulang, sehingga rotasi memindahkan kandidat
   pertama ke baris sehat. Kursor rotasi karena itu maju **+2**, bukan +1.

Kelas ini tidak punya satu pun tes: grep `407`, `OnProxyConnectResponse`, dan `proxyconnect` di
`app-serv` menghasilkan nol kemunculan sebelum perbaikan.

## 2. Bukti terukur (sebelum perbaikan)

Baris mati owner: `prx_0386XYJ9J4110ZXGN1N59K9SPC` ("proxy 3", `216.26.249.201:3128`, enabled,
belum pernah di-Test, username sama dengan baris hidup, port berbeda dari baris `:3129` yang hidup).
Uji live memakai request `pi-agent` ke gateway `:9090` dengan sampler `ss` per 50 ms dan pembacaan
kursor serta kunci park di Redis. Pola yang terukur pada run pertama:

| #   | kursor sebelum | memimpin    | socket ke baris mati | park          | hasil                            |
| --- | -------------- | ----------- | -------------------- | ------------- | -------------------------------- |
| 1   | 16             | proxy 2     | tidak                | tidak ada     | 200, satu attempt                |
| 2   | 17             | **proxy 3** | **ada**              | **tidak ada** | 200 lewat proxy 1, **kursor +2** |
| 3   | 19             | proxy 2     | tidak                | tidak ada     | 200, satu attempt                |
| 4   | 20             | **proxy 3** | **ada**              | **tidak ada** | 200 lewat proxy 1, **kursor +2** |

Socket baris mati terlihat pada setiap request yang memimpin dengan proxy 3: `SYN-SENT → ESTAB
(~350 ms) → LAST-ACK`, hidup ~0.8 s, dengan counter **`sent=160 B`, `recv=146 B`** pada run kedua
yang mengukur byte: satu request CONNECT dan satu penolakan, dan `sent` tidak pernah bertambah,
jadi **tidak ada TLS ClientHello** dan tunnel memang tidak pernah terbuka. Penolakan itu juga bukan
timeout (attempt selesai 0.8 s, batas CONNECT 10 s).

Repro lokal stdlib (`/tmp/pass39/connectclass/main.go`, proxy stub yang menjawab 407 ke CONNECT)
menunjukkan bentuk error yang sama: `As(net.OpError)=false`; `As(net.Error)=true` (karena
`*url.Error` memenuhi interface itu) tetapi `Timeout()=false`, sehingga classifier menjawab `false`.

## 3. Akar masalah

| #      | Tingkat | Isi                                                                                                                                                                                                                                                                                               |
| ------ | ------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **F1** | HIGH    | Penolakan CONNECT non-200 tidak terklasifikasi sebagai kegagalan connect-stage: baris rusak tidak diparkir, kandidat berikutnya tidak dicoba, dan keselamatan request bergantung pada retry provider. Provider dengan retry habis/dinonaktifkan akan menjawab 502 padahal ada baris sehat di pool |
| **F2** | MEDIUM  | Efek samping retry: tiap request yang kena baris rusak memajukan kursor rotasi dua langkah, sehingga pola rotasi bergeser dan baris rusak tetap membebani trafik selamanya (tidak pernah masuk cooldown)                                                                                          |

## 4. Perbaikan

- `clientFor` (`dataplane/proxy_route.go`) memasang `clone.OnProxyConnectResponse = rejectConnectResponse`
  pada tiap clone kandidat: CONNECT non-200 menjadi error bertipe `proxyConnectRejected{status}`.
- `isProxyConnectFailure` memeriksa tipe itu lebih dulu, lalu kasus lama (`*net.OpError`,
  `net.Error.Timeout()`) tetap utuh.
- Alasan memakai hook: `OnProxyConnectResponse` adalah API publik Go yang dipanggil **sebelum**
  cabang non-200 milik Go, dan error yang dikembalikannya sampai ke pemanggil (dibuktikan lokal:
  `As(proxyConnectRejected)=true`), jadi klasifikasinya bertipe, bukan pencocokan string.
- Efek yang dituju: baris di-park 2 menit dan walk lanjut ke kandidat berikutnya **dalam request
  yang sama**; kursor kembali +1.

## 5. Verifikasi

**TDD merah-dulu.** Berkas uji baru `dataplane/proxy_connect_rejection_test.go` (156 baris; berkas
terpisah karena `proxy_route_test.go` sudah 240 baris dan gate batas baris tidak mengecualikan
`*_test.go` meski komentarnya menyebut sebaliknya). Dijalankan sebelum perbaikan, merah dengan teks
error persis seperti live: `Post "https://127.0.0.1:34839/x": Proxy Authentication Required`.
Double-nya dua: `refusingProxy` (menjawab 407 ke CONNECT) dan `tunnelingProxy` (hijack lalu tunnel),
diuji end-to-end lewat destinasi TLS asli; ditambah tabel classifier (error bertipe vs kalimat yang
sama tanpa tipe, yang kedua harus `false`).

**Gerbang** pada pohon yang di-commit: go-lint PASS (vet dua tag, gofmt, staticcheck dua tag,
golangci-lint 0 issues; empat peringatan batas baris semuanya berkas aktor lain), go-headers 919
berkas, go-test `-race` 4m43s seluruh paket `ok`, contract-drift 12 kode, contract-openapi PASS,
secrets PASS (241 commit + pohon kerja), hook pre-commit hijau.

**Bukti ulang live** pada gateway owner yang sudah di-build ulang dan restart (proses `21:12:55 WIB`
lebih baru dari commit `21:08:00 WIB`, jadi binary memuat perbaikan). Karena P3 sudah `state: fail`
dari Test manual owner dan karenanya keluar dari rencana, dipakai baris sementara atas izin owner:
`proxy 4 (temp)` = `example.com:80` tanpa kredensial (endpoint publik yang menjawab
`HTTP/1.1 400 Bad Request` ke CONNECT, kelas non-200 yang sama). Lima request `pi-agent`:

| #   | kursor      | HTTP | pembawa | socket baris mati   | kunci park         |
| --- | ----------- | ---- | ------- | ------------------- | ------------------ |
| 1   | 30 → 31     | 200  | proxy 1 | tidak               | tidak ada          |
| 2   | 31 → 32     | 200  | proxy 2 | tidak               | tidak ada          |
| 3   | **32 → 33** | 200  | proxy 1 | **ada** (`sent=91`) | **ada, TTL 118 s** |
| 4   | 33 → 34     | 200  | proxy 2 | tidak               | hidup (112 s)      |
| 5   | 34 → 35     | 200  | proxy 1 | tidak               | hidup (110 s)      |

Request 3 adalah inti pembuktian: baris mati memimpin, socket-nya terlihat
(`[2606:4700:10::6814:179a]:80`), kunci park `pannelai:proxy:park:<sha256(id)>` muncul dengan TTL
~120 s, kursor maju **+1** (bukan +2, artinya failover terjadi di dalam walk dan retry provider
tidak dipakai), dan 200 dilayani proxy 1 sehingga klien tidak pernah melihat kegagalan. Request 4
dan 5 tidak menyentuh baris mati sama sekali selama cooldown, jadi filter parkir juga terbukti live.

**Bersih-bersih:** baris sementara di-`DELETE` (204) dan kunci park-nya dihapus dari Redis; pool
kembali ke tiga baris (P1 `ok`, P2 `ok`, P3 `fail`), kursor 35, parks 0. Log:
`/tmp/pass39/rt-failover.log` + `rt-f*.log`.

## 6. Residu

- **Baris mati owner (P3) tetap `state: fail`** dari Test manual dan karena itu tetap di luar
  rencana. Itu perilaku by design (`RouteUsable()` mengecualikan `fail` sampai ada re-test sukses),
  bukan sisa cacat; lever manualnya adalah tombol Test di panel.
- **Kelas tetangga yang tidak ditutup:** kegagalan TLS ke destinasi setelah CONNECT 200. Itu milik
  destinasi, bukan pool, sesuai D7; tidak ada perubahan di sana.
- **Suite integrasi** tidak dijalankan (tidak ada perubahan migrasi/repository).
- **Em dash: diukur.** Teks register ini 0 karakter U+2014; teks sumber yang diperbaiki juga 0.
- **Belum di-push:** `d1c2a81` lokal di atas `c1796d2`; push menunggu keputusan owner.
