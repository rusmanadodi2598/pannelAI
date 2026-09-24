# 026-LIVE-REDIS-FLUSHED-BY-INTEGRATION-TESTS.md: Suite integrasi app-serv mengosongkan Redis yang dipakai gateway, sesi panel mati, dan operator terlempar ke /login

Dokumen kerja atas laporan bug owner: **"Saat add model pada provider selalu terlempak ke login page"**,
dengan log akses gateway 08:24:47 sampai 08:25:10 yang memperlihatkan sesi valid lalu `POST
/api/v1/models/custom` menjawab `401 UNAUTHORIZED` dan setiap permintaan sesudahnya juga 401. Dokumen ini
bukan kontrak; kontrak tetap `docs/SPEC-UI/001-SPEC-UI.md` §8.1 (perilaku panel atas 401) dan
`docs/SPEC-API/001-SPEC-API.md` §7.2 (sesi). Pola mengikuti draft 021 sampai 025: temuan bernomor F,
bukti yang bisa diulang, dan keputusan yang diambil di depan.

| | |
|---|---|
| **Status** | **OPEN 2026-09-24**: F1 dan F2 diajukan ke `app-serv` (pemilik harness test). Panel **tidak punya** yang perlu diperbaiki (§4). Penjaga flush dipasang dan siap menangkap kejadian berikutnya (§6) |
| **Mechanism** | N/A: diagnosis, tidak ada perubahan UI |
| **Scope** | Diagnosis dijalankan terhadap gateway hidup `:9090` dan panel owner `:3000`; perbaikannya milik `app-serv/`. `app-ui/` tidak disentuh |
| **Permintaan owner** | "BUG: Saat add model pada provider selalu terlempak ke login page." beserta log akses gateway 08:24:47-08:25:10 |
| **Kaitan** | SPEC-UI §8.1 (401 → `/login`), `scrypts/gates/go-test.sh:43-50`, `scrypts/hooks/pre-push:39`, draft 021 §6 (tabel operasional DB dikosongkan aktor lain), draft 025 (pass panel yang baru ditutup, bukan penyebab) |
| **Tanggal** | 2026-09-24 |

## 1. Ringkasan

Yang dilaporkan bukan cacat panel dan bukan cacat rute. Rute add-model menjawab normal pada sesi yang
hidup, dan panel bereaksi atas 401 persis seperti SPEC-UI §8.1 menetapkan (kembali ke `/login` sambil
mengingat tujuan). Yang terjadi adalah **sesi Redis-nya dihapus dari bawah panel**, dan pelakunya adalah
**suite integrasi `app-serv` sendiri**, yang memanggil `FlushDB` pada database Redis yang sama dengan
yang dipakai gateway hidup.

Rangkaian waktunya, seluruhnya terukur:

| Waktu (WIB) | Bukti | Arti |
|---|---|---|
| 08:02, 08:16 | log PostgreSQL, 18+9 dan 27 baris `pannelai_test` | dua run suite bertag menyentuh database uji |
| 08:23 sampai 08:25 | log PostgreSQL, 57+26+27 baris `pannelai_test` | run suite bertag ketiga berjalan |
| 08:24:47.926 | gateway: `POST /api/v1/auth/login` **204** | operator login (artinya ia sudah terlempar sebelum ini) |
| 08:24:48 sampai 08:24:54 | gateway: `auth/status`, providers, provider-nodes, endpoints, models/custom, providers, provider-nodes, semua **200** | sesi baru hidup, panel bekerja |
| 08:25:07.535 | gateway: `POST /api/v1/models/custom` **401 UNAUTHORIZED**, `duration_ms 0` | guard sesi menolak sebelum handler: kunci Redis sesi sudah tidak ada |
| 08:25:07.572, 08:25:10.708 | gateway: `GET /api/v1/providers/...` **401** | sesi tetap mati, panel mengalihkan ke `/login` |
| 08:25:24, 08:25:25, 08:25:27 | log PostgreSQL, constraint dan duplicate key di `pannelai_test` | run suite masih berjalan melewati menit 401 itu |
| 08:26:42 | `fa37ed5` (`?active=true` pada katalog) | aktor app-serv commit 95 detik setelah 401 |

Counter Redis menguatkan: **`cmdstat_flushdb:calls=143`** sejak Redis hidup dua hari, sementara gateway
sendiri tidak pernah memanggil `FlushDB` (tidak ada satu pun di kode non-test).

## 2. F1 (HIGH): tiga test integrasi mengosongkan database Redis yang dipakai gateway

Tiga situs, semuanya test bertag `integration`:

| Situs | Baris |
|---|---|
| `app-serv/internal/repository/redis/quota_counter_test.go` | `:61` `client.FlushDB(ctx)` |
| `app-serv/internal/repository/redis/usage_active_store_test.go` | `:63` `client.FlushDB(ctx)` |
| `app-serv/cmd/app-serv/playground_live_stack_test.go` | `:107` `client.FlushDB(ctx)` |

Klien uji itu tidak memilih database. Kedua helper-nya hanya memindahkan `host:port` dan kredensial:

- `internal/repository/redis/console_buffer_test.go:72-89` (`redisOptions`) membangun `redis.Options{Addr: raw}` (`:74`) lalu `options.Addr = raw[at+1:]` (`:79`).
- `cmd/app-serv/playground_live_doubles_test.go:76-90` (`liveRedisOptions`) bentuk yang sama (`:77`, `:82`).

`redis.Options.DB` tidak pernah di-set, jadi keduanya memakai **DB 0**. Gateway hidup memakai klien yang
sama polosnya: `cmd/app-serv/main.go:83-86` (`redis.NewClient(&redis.Options{Addr: ..., Password: ...})`,
tanpa `DB`), juga **DB 0**.

Akibatnya satu `FlushDB` menghapus seluruh state hidup gateway yang disimpan di Redis, bukan hanya sesi.
Prefiks yang terdaftar di DB 0 (semuanya diverifikasi `grep -n`):

| Kunci | Sumber | Yang hilang saat flush |
|---|---|---|
| `pannelai:auth:session:` | `session.go:25` | **setiap sesi panel**, termasuk sesi yang sedang dipakai operator |
| `pannelai:rate:` | `limiter.go:27` | jendela rate limit 120/menit, hitungannya mulai dari nol |
| `pannelai:auth:login:fail:` / `lock:` | `limiter.go:25-26` | hitungan gagal login dan lockout |
| `pannelai:quota:` | `quota_counter.go:41` | counter kuota yang belum di-flush ke PostgreSQL |
| `pannelai:console:lines` | `console_buffer.go:33` | ring konsol |
| `pannelai:usage:active` | `usage_active_store.go:44` | himpunan permintaan aktif untuk `/usage/live` |
| `pannelai:oauth:state:` | `oauth_state.go:32` | state OAuth sekali pakai yang sedang berjalan |
| `pannelai:combo:rotation:` / `pannelai:vision:rotation:` | `combo_rotation.go:40`, `vision_rotation.go:39` | posisi rotasi combo dan vision |
| `pannelai:cursor:endpoint:` | `endpoint_cursor.go:34` | kursor paginasi endpoint |

Sisi PostgreSQL sudah punya penjaga untuk masalah yang sama, sisi Redis tidak. Harness PostgreSQL
menolak DSN yang namanya tidak menyatakan dirinya database uji, dengan alasan tertulis "the harness
truncates its tables" (`internal/repository/postgres/integration_harness_test.go:118-134`, `:129` mencari
substring `test`). Tidak ada padanan apa pun di harness Redis, padahal yang dilakukannya lebih luas:
`TRUNCATE` beberapa tabel versus `FLUSHDB` satu database penuh.

## 3. F2 (MEDIUM): jangkauannya bukan cuma run manual, gerbang pre-push ikut menjalankannya

`scrypts/gates/go-test.sh:43-50` menjalankan `go test -race -tags=integration -count=1 ./...` untuk setiap
modul Go **setiap kali `PANNELAI_TEST_POSTGRES_DSN` di-set**, dan `scrypts/hooks/pre-push:39` memanggil
gerbang itu. Jadi di checkout ini, sebuah `git push` (dan setiap run manual yang disiplin) mengosongkan
Redis hidup. Itu menjelaskan kenapa kejadiannya berulang, bukan sekali: tiga run dalam 25 menit pagi ini.

## 4. Bukan temuan: alur add-model dan hop proxy panel benar

Diukur pada sesi yang hidup, supaya klaimnya bukan asumsi:

| Jalur | Hasil terukur |
|---|---|
| `POST /api/v1/auth/login` lewat origin panel `:3000` | **204**, cookie `pannel_session` terpasang |
| `GET /api/v1/providers/{id}` lewat panel | **200** |
| `GET /api/v1/models/custom` lewat panel | **200** |
| `POST /api/v1/models/custom` lewat panel dengan body kosong | **400 VALIDATION_ERROR** (guard sesi lolos, validasi yang menolak) |
| `GET /api/v1/providers/{id}` lagi | **200** (sesi selamat melewati POST) |
| Click-through browser atas panel owner `:3000` (`/tmp/bugfix/drive.ts`) | `POST /api/v1/models/custom` **201**, baris `bugprobe-20260924-a` muncul, **0 respons 401** di seluruh run, baris probe dihapus lagi (`DELETE` 204, `stillThere: false`) |

Jadi tidak ada yang perlu diubah di panel: 401 berarti sesi tidak valid, dan SPEC-UI §8.1 memang
memerintahkan kembali ke `/login`. Satu-satunya cacat yang tersisa di jalur panel adalah cacat yang sudah
tercatat sebagai draft 021 F6 (panggilan yang mati di tengah tidak menulis baris akuntansi), tidak
berkaitan.

## 5. Bukti dan cara mengulang

```bash
# 1. Counter flush dan prefiks yang hidup di DB 0 (tidak menulis apa pun)
PW=$(sudo -n grep -oP '^requirepass \K.*' /etc/redis/redis.conf)
redis-cli -a "$PW" --no-auth-warning info commandstats | grep flushdb
redis-cli -a "$PW" --no-auth-warning -n 0 --scan

# 2. Jendela run suite bertag, dari log PostgreSQL (prefix log membawa user@db)
sudo -n grep -h 'pannelai_test' /var/log/postgresql/postgresql-14-main.log | awk '{print $1" "substr($2,1,2)}' | uniq -c

# 3. Rute dan proxy panel pada sesi hidup (204/200/200/400/200)
#    lihat /tmp/bugfix/  : run-probe.sh + drive.ts + evidence.json + shots/
```

Penjaga yang dipasang dan masih berjalan saat dokumen ini ditulis: `/tmp/bugfix/watch-flush.sh`
menyampel `cmdstat_flushdb:calls` tiap 2 detik, menulis ke `/tmp/bugfix/flush-watch.log` saat angkanya
naik, dan menyertakan `ps` proses `go test` pada detik itu. Ia menangkap kejadian berikutnya dengan
stempel waktu dan pelakunya.

## 6. Usulan perbaikan (milik `app-serv`)

**R1 (disarankan): beri harness Redis penjaga yang setara dengan penjaga PostgreSQL, dan pindahkan test
ke database sendiri.** Pilih satu indeks database khusus uji (mis. `DB: 15`) di kedua helper test, lalu
tolak alamat yang menunjuk database gateway. Ini mengikuti pola yang sudah ada di repo, bukan pola baru.
Catatan pelaksanaan: test yang menyalakan gateway sendiri harus membuat server itu memakai database yang
sama, supaya kedua sisi sepakat.

**R2 (tanpa perubahan kode): jalankan suite bertag terhadap instance Redis kedua** (mis. `redis-server
--port 6380 --requirepass ...`), lalu arahkan `PANNELAI_TEST_REDIS_ADDR` ke sana. Resepnya hari ini
hanya hidup di header berkas test (`PANNELAI_TEST_REDIS_ADDR='[user:password@]host:port'`) dan di
komentar `scrypts/gates/go-test.sh:11`, jadi keduanya perlu diperbarui; `app-serv/README.md` hanya
mendokumentasikan run PostgreSQL bertag (`:148-155`), tanpa peringatan setara untuk Redis.

**R3 (pertahanan berlapis, tidak menggantikan R1/R2):** pindahkan state hidup gateway ke indeks database
Redis tersendiri lewat config (mis. `REDIS_DB`), sehingga flush tak sengaja atas DB 0 tidak bisa lagi
mengeluarkan operator dari panel. Ini mengurangi kerusakan, bukan menghilangkan penyebabnya, karena
counter kuota dan jendela rate limit tetap hidup di database yang sama dengan sesi.

## 7. Batas yang tidak diklaim

- Flush pada 08:25:07 **tidak terlihat langsung**: penjaga baru dipasang 08:33. Korelasinya dibangun dari
  jendela run suite di log PostgreSQL, counter `flushdb` yang kumulatif, dan waktu commit aktor
  (08:26:42). Yang belum ada adalah satu baris yang menyebut detik flush itu sendiri.
- Angka 143 adalah jumlah sejak Redis hidup dua hari, bukan sejak pagi ini, dan tidak dibagi per pelaku.
- Gateway yang diperiksa adalah proses `go run` milik owner (`:9090`, hidup sejak 07:49:35); tidak ada
  restart di antara 08:24 dan 08:25, jadi sesi tidak bisa hilang karena restart.
- Perbaikan R1 sampai R3 tidak dikerjakan di dokumen ini: `app-serv/` milik aktor lain, dan salah satu
  berkas yang perlu diubah (`quota_counter_test.go`) masih untracked di working tree-nya.
