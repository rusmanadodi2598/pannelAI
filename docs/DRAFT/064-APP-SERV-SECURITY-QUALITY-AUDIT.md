# 064-APP-SERV-SECURITY-QUALITY-AUDIT.md: 16 CVE stdlib dan rotasi sesi yang bisa balapan

Dokumen ini adalah **rencana kerja**, bukan catatan yang sudah selesai. Temuan sudah diverifikasi di kode,
`govulncheck`, dan suite `-race`; status tiap butir diperbarui saat perbaikannya mendarat.

| | |
| --- | --- |
| **Status** | Fase 0 (baseline), Fase 1 (H1), dan Fase 2 (M1/M2) selesai. `govulncheck` melaporkan 0 CVE reachable; suite `-race` pada paket yang berubah dan suite integrasi Redis hijau. Sisa 3 temuan tingkat module ada di `golang.org/x/crypto/ssh` dan `openpgp` yang tidak diimpor. Fase 3-4 belum dikerjakan. |
| **Sumber** | Audit read-only `app-serv`: `go build`, `go vet`, `gofmt`, `staticcheck v0.8.1`, `go test ./...`, `go test -race` pada paket kritis, dan `govulncheck` (16 CVE reachable). |
| **Lingkup** | `app-serv` (`cmd/`, `internal/`, `migrations/`, `go.mod`) plus gate `scrypts/gates/` dan `.github/workflows/gates.yml`. |
| **Tidak termasuk** | `app-ui`, deployment multi-replika, dan perubahan bentuk wire. |
| **Struktural (AGENTS.md §1.9)** | Fase 1 N/A. Fase 2 mengubah bentuk state sesi: `SYSTEM_MAP.md` diperbarui pada alur login/guard dan daftar key Redis; tidak ada perubahan domain, wire, atau batas service. |

## 1. Baseline yang sudah dijalankan

| Pemeriksaan | Hasil |
| --- | --- |
| `go build ./...`, `go vet ./...` | PASS |
| `gofmt -l .` | bersih |
| `staticcheck v0.8.1 ./...` | tanpa temuan |
| `go test ./...` | exit 0 |
| `go test -race` pada `service`, `dataplane`, `provider`, `config`, `handler` | PASS |
| `govulncheck` | 16 CVE stdlib reachable; 3 di paket import dan 6 di module require tidak terpanggil |
| `t.Skip` / `//nolint` tanpa alasan | tidak ada / tidak ada |

## 2. Temuan

### 2.1 HIGH

| ID | Temuan | Bukti | Perbaikan | Status |
| --- | --- | --- | --- | --- |
| H1 | Toolchain Go 1.26.5 memuat 16 CVE stdlib yang ter-reach | `app-serv/go.mod:5`; govulncheck: GO-2026-6809, 6617, 6613, 6612, 6611, 6610, 6608, 6607, 6605, 6603, 6218, 6090, 6089, 6088, 5972, 5026 | Naikkan `toolchain` ke `go1.26.10`, tambahkan gate `govulncheck` yang di-pin ke CI dan `all.sh` | Selesai: toolchain 1.26.10; `scrypts/gates/go-vuln.sh` dipanggil `all.sh`, pre-push, dan CI; hasil 0 CVE reachable |

Trace dengan bobot tertinggi: GO-2026-6608 melewati batas memori parser MIME di jalur unggahan
(`internal/schema/media_transcription.go:43`); GO-2026-6617/6603 menyerang `http.Server` publik;
GO-2026-6613/6605 mendesinkronkan koneksi HTTP/1 CONNECT; GO-2026-6218 membebani parsing URL tiap request.

### 2.2 MEDIUM

| ID | Temuan | Bukti | Perbaikan | Status |
| --- | --- | --- | --- | --- |
| M1 | Login dengan password lama bisa membuat sesi setelah `RevokeAll` rotasi password, dan kegagalan `RevokeAll` setelah CAS meninggalkan sesi hidup | `internal/service/session.go:71-76`, `internal/service/auth.go:121-145` | Tambahkan *session epoch* yang diikat ke kredensial dan dicek di `Authenticate`; pisahkan error revoke parsial | Selesai: epoch Redis (`pannelai:auth:epoch`) dibaca sebelum hash dan disimpan per digest; `Authenticate` membandingkan lewat satu `MGET`; `RevokeAll` menjadi satu `INCR`; kegagalan revoke menjadi `ErrSessionRevocationIncomplete` |
| M2 | Cookie sesi hanya `Secure` bila `APP_ENV=production`; default `development`; tanpa prefix `__Host-` | `cmd/app-serv/auth_wiring.go:46`, `internal/config/config.go:88,166`, `internal/handler/auth_cookie.go:37,45` | Default `Secure: true` dengan opt-out lokal eksplisit; pertimbangkan `__Host-` dan HSTS di terminator TLS | Selesai: `Secure` default `true`, opt-out `SESSION_COOKIE_INSECURE=true` ditolak saat `APP_ENV=production`; nama cookie tidak diubah (lihat §5) |

### 2.3 LOW

| ID | Temuan | Bukti | Perbaikan | Status |
| --- | --- | --- | --- | --- |
| L1 | Redis tanpa TLS; `POSTGRES_DSN` tidak menegakkan `sslmode` | `cmd/app-serv/main.go:66-73`, `internal/config/config_validate.go:26-27` | Dukung TLS Redis; wajibkan `sslmode` verifikasi di luar local | Belum |
| L2 | Planner proxy fail-open ke egress langsung saat pool gagal dibaca | `internal/service/proxy_route.go:105-111` | Fail-closed bila proxy diwajibkan; fail-open hanya untuk kondisi "tidak ada proxy" | Belum |
| L3 | Query list proxy/provider node/combo tanpa `LIMIT`, dibaca ulang tiap request | `internal/repository/postgres/proxy.go:79`, `provider_node.go:65`, `combo.go:125`; `internal/service/proxy_route.go:86,105,127-147` | `LIMIT`/pagination dan cache per request atau TTL singkat | Belum |
| L4 | Komentar duplikat dan `@reason` basi | `internal/domain/gateway_key.go:49-53`, `internal/domain/media_provider.go:7` | Hapus duplikasi; samakan `@reason` dengan egress yang sudah dijaga | Belum |

### 2.4 INFO

| ID | Temuan | Bukti | Perbaikan | Status |
| --- | --- | --- | --- | --- |
| I1 | CSRF hanya bersandar pada `SameSite=Lax` + JSON | `internal/handler/auth_cookie.go:37` | Pemeriksaan `Origin`/`Referer` sebagai lapisan kedua | Belum |
| I2 | CI tidak menjalankan pemindaian kerentanan | `.github/workflows/gates.yml` | Ditutup oleh H1 | Selesai bersama H1 |

## 3. Fase kerja

### Fase 0: baseline (selesai)

Semua pemeriksaan di §1 dijalankan pada working tree bersih sebelum perubahan apa pun.

### Fase 1: H1, toolchain dan gate kerentanan (selesai)

1. `app-serv/go.mod`: `toolchain go1.26.5` menjadi `go1.26.10`.
2. `AGENTS.md` baris versi Go disamakan dengan toolchain yang di-pin.
3. `scrypts/gates/go-vuln.sh`: menjalankan `govulncheck` per modul `app-*`, gagal bila tool tidak ada,
   memakai `tool_path` seperti gate lain.
4. `scrypts/gates/all.sh`, `scrypts/hooks/pre-push`, dan `.github/workflows/gates.yml` memanggil gate
   baru; CI meng-install `golang.org/x/vuln/cmd/govulncheck@v1.9.0` (pin, bukan `@latest`).
5. Kriteria keluar: `govulncheck ./...` melaporkan 0 CVE reachable, build/vet/test/gate lain tetap hijau.
   Hasil pada pass ini: `govulncheck` 0 reachable (3 temuan module-only), `go test -race -count=1 ./...`
   PASS, `go-lint` PASS (vet, tagged vet, gofmt, staticcheck, golangci-lint 0 issue), `go-headers`
   1491 file PASS, `antislop` PASS, dan `gate-scope` PASS.

### Fase 2: MEDIUM (selesai)

- M1: epoch kredensial disimpan di Redis, dibaca `Login` sebelum hash, dan disimpan bersama digest sesi.
  `Authenticate` membandingkan nilai digest dengan epoch saat ini dalam satu `MGET`; `RevokeAll` cukup
  satu `INCR` sehingga tidak ada keadaan revoke sebagian. Kegagalan `INCR` setelah CAS dilaporkan sebagai
  `ErrSessionRevocationIncomplete` (kode wire `INTERNAL_ERROR`), bukan sebagai password yang gagal diubah.
- M2: `Secure` default `true`; `SESSION_COOKIE_INSECURE` adalah opt-out eksplisit untuk development dan
  ditolak saat `APP_ENV=production`. Wiring memakai `!cfg.SessionCookieInsecure`.
- Test: `TestAuthLoginFromBeforeRotationIsRejected` menempatkan rotasi tepat setelah hash dibaca;
  `TestAuthChangePasswordReportsRevokeFailure` memisahkan kegagalan revoke; suite Redis menguji epoch
  lama dan epoch baru; test config dan atribut cookie mencakup kedua profil deployment.
- Kriteria keluar: test baru gagal sebelum patch dan hijau sesudahnya; `go test -race` pada paket yang
  berubah hijau; `SYSTEM_MAP.md` diperbarui karena bentuk state sesi berubah.
  Hasil pada pass ini: `go test -race -count=1` pada `service`, `handler`, `config`, `domain`, dan
  `router` PASS; suite integrasi Redis PASS; `go-lint` PASS (vet, tagged vet, gofmt, staticcheck,
  golangci-lint 0 issue, tanpa peringatan batas baris), `go-headers` 1494 file PASS, `antislop` PASS,
  `gate-scope` PASS, `go-vuln` 0 reachable (3 module-only), dan `git diff --check` bersih.

### Fase 3: LOW

- L1: konfigurasi TLS Redis + validasi `sslmode` Postgres.
- L2: fail-closed untuk proxy yang diwajibkan; test planner untuk error pembacaan pool.
- L3: `LIMIT` + cache; test query dan test planner.
- L4: higiene komentar; `antislop.sh` dan `go-headers.sh` tetap hijau.

Kriteria keluar: setiap butir punya test atau gate yang membuktikan perilakunya, bukan hanya komentar.

### Fase 4: INFO

- I1: pemeriksaan `Origin`/`Referer` untuk endpoint state-changing, dengan test yang menolak origin asing
  dan tetap menerima request tanpa header dari klien non-browser.

### Fase 5: penutupan

- `go test -race -count=1 ./...`, `scrypts/gates/go-lint.sh`, `go-headers.sh`, `antislop.sh`, `go-vuln.sh`,
  dan `all.sh` dijalankan; status tabel diperbarui; `SYSTEM_MAP.md` ditandai N/A atau diperbarui.

## 4. Gerbang verifikasi

```bash
cd app-serv && go test -race -count=1 ./...
scrypts/gates/go-lint.sh
scrypts/gates/go-headers.sh
scrypts/gates/antislop.sh
scrypts/gates/go-vuln.sh
scrypts/gates/all.sh
```

## 5. Catatan keputusan

- Bump toolchain dipilih daripada menambal per-paket: seluruh 16 temuan berasal dari standard library yang
  ikut binary, jadi hanya patch toolchain yang benar-benar menghapusnya.
- Gate `govulncheck` di-pin karena `@latest` membuat hasil gate berubah tanpa commit, alasan yang sama yang
  sudah dipakai untuk `staticcheck` dan `gitleaks` di workflow ini.
- Temuan yang tidak melibatkan jalur kode (3 paket import, 6 module require) tidak dianggap blocker, tetapi
  tetap terlihat di output gate sehingga bisa ditangani saat dependensi itu disentuh.
- Epoch Redis dipilih daripada kolom kredensial di PostgreSQL: state sesi sudah hidup di Redis, sehingga
  pemeriksaan `Authenticate` tidak menambah round trip database. Kolom versi di PostgreSQL akan menuntut
  baca per request atau cache yang justru memunculkan masalah invalidasi yang sama. `INCR` juga membuat
  pencabutan massal atomik, berbeda dari `SCAN` + `DEL` yang bisa berhenti di tengah.
- Urutan `Login` adalah bagian dari perbaikan, bukan detail: epoch dibaca sebelum hash. Membacanya sesudah
  hash akan memberi sesi baru epoch terbaru walaupun password yang dibandingkan sudah basi.
- Deploy yang naik ke bentuk epoch ini memaksa login ulang sekali. Digest lama tidak membawa nilai epoch,
  dan memperlakukannya sebagai epoch nol akan mempertahankan sesi yang dibuat sebelum rotasi terakhir,
  persis yang ingin dicabut.
- `__Host-` tidak dipakai. Nama `pannel_session` adalah bagian dari permukaan yang sudah didokumentasikan
  (contoh OpenAPI dan test panel menyebutnya), dan mode opt-out lokal tidak bisa memakai cookie `__Host-`
  tanpa `Secure`. HSTS tetap tanggung jawab terminator TLS; aplikasi hanya memastikan atribut cookie benar.
- Kegagalan `RevokeAll` setelah CAS tidak diulang otomatis: password sudah berubah, jadi password lama tidak
  bisa dipakai lagi untuk mengulang request. Caller menerima kondisi terpisah yang menyebut password sudah
  berubah tetapi sesi lama belum tercabut.
