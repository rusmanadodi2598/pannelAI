# app-serv

Backend Go untuk pannelAI: API manajemen (control plane) dan proxy kompatibel OpenAI (data plane).

Kontrak perilaku ada di [`docs/SPEC-API/001-SPEC-API.md`](../docs/SPEC-API/001-SPEC-API.md); aturan koding mengikat ada di [`AGENTS.md`](../AGENTS.md).

## Status

**P0 sebagian.** Yang tersedia dan teruji:

- konfigurasi environment bertipe dengan validasi fail-fast
- koneksi PostgreSQL (pgx/v5) dan Redis (go-redis/v9) dengan limit pool eksplisit
- migrasi `gateway_keys`
- endpoint sistem: `/api/v1/health`, `/api/v1/version`
- CRUD gateway keys: list, create, get, patch, revoke
- middleware: pemulihan panic, envelope error §8, 404/405 dinormalkan

Belum ada dari lingkup P0 itu sendiri: autentikasi sesi (`§7.2`: login, logout, status, change-password) dan quality gates di `scrypts/`. Tanpa `§7.2`, seluruh route gateway keys masih terbuka.

Belum ada dari fase berikutnya: provider registry, upstream endpoint & multi-key, combos, vision adapter, data plane chat, usage/quota, logs, settings. Lihat SPEC-API-001 §10 untuk P1 sampai P3.

> **Rate limit belum ditegakkan.** `RATE_LIMIT_PER_MIN` divalidasi dan ditampilkan, tetapi belum ada middleware limiter yang memakainya; endpoint publik saat ini tidak terlindungi olehnya.
>
> Route manajemen **belum digerbangi sesi**, karena autentikasi dashboard (`§7.2`) belum dibangun. Jangan mengekspos server ini ke jaringan publik sampai itu selesai. P0 exit criteria di SPEC-API-001 §10 menyebut login dan key CRUD diuji bersamaan, jadi P0 belum tuntas.

## Prasyarat

- Go 1.26+
- PostgreSQL
- Redis

## Menjalankan

```bash
cd app-serv
cp .env.example .env        # isi nilai sebenarnya
set -a && . ./.env && set +a

go run ./cmd/app-serv
```

Migrasi diterapkan otomatis saat boot: berkas `migrations/*.up.sql` di-embed ke dalam binary, dijalankan berurutan menurut prefiks angkanya, dan dicatat di tabel `schema_migrations`. Sebuah migrasi dijalankan **paling banyak sekali** per database, di dalam satu transaksi bersama baris ledger-nya, sehingga kegagalan di tengah meninggalkan database pada migrasi terakhir yang utuh dan boot berikutnya melanjutkan dari sana. Database yang benar-benar kosong pun langsung siap dipakai.

Penerapan otomatis saat boot cocok untuk P0, tetapi tidak cocok untuk produksi multi-replika: setiap replika akan mencoba bermigrasi bersamaan. Sebelum deploy, pindahkan langkah ini ke job rilis terpisah.

**Rollback belum tertangani.** Runner hanya membaca `*.up.sql`; berkas `*.down.sql` disediakan untuk rollback manual dan belum dieksekusi oleh kode, dan ledger tidak menghapus baris saat rollback. Mengembalikan skema saat ini berarti menjalankan `down.sql` dengan `psql` lalu menghapus baris `schema_migrations` yang sesuai.


Server gagal start bila konfigurasi tidak valid, dan menyebut variabel yang bermasalah. Itu disengaja: konfigurasi yang salah harus terlihat saat boot, bukan saat melayani permintaan.

`SESSION_SECRET` minimal 32 byte dan `ENCRYPTION_KEY` tepat 32 byte (AES-256). Keduanya wajib.

## Endpoint P0

```bash
curl localhost:8080/api/v1/health
curl localhost:8080/api/v1/version

curl -X POST localhost:8080/api/v1/gateway-keys \
  -H 'Content-Type: application/json' -d '{"name":"cli"}'

curl localhost:8080/api/v1/gateway-keys
curl -X DELETE localhost:8080/api/v1/gateway-keys/{id}
```

Respons create memuat `plaintext_key` satu kali. Setelah itu hanya `key_hint` (`sk-…abcd`) yang dikembalikan; yang tersimpan hanyalah digest SHA-256.

## Struktur

```text
app-serv/
├── cmd/app-serv/          # composition root: wiring, tanpa logika
├── internal/
│   ├── config/            # konfigurasi env bertipe dan tervalidasi
│   ├── domain/            # entitas, value object, envelope error, ULID
│   ├── repository/        # kontrak penyimpanan + implementasi PostgreSQL
│   ├── service/           # use case; tidak mengimpor net/http
│   ├── schema/            # DTO, tag validasi, serialisasi respons
│   ├── handler/           # adaptor HTTP
│   └── router/            # tabel route dan middleware
├── migrations/            # migrasi up/down berpasangan
```

Alur layer satu arah: `schema → domain → repository → service → handler → router`.

## Pengujian

```bash
go test -race ./...
go vet ./...
staticcheck ./...
gofmt -l .
```

Cakupan saat ini:

| Paket | Yang diuji |
|---|---|
| `internal/domain` | generator ULID (keunikan, monotonisitas, alfabet), transisi state gateway key, hashing |
| `internal/config` | parsing env table-driven, tiap aturan penolakan, batas nilai, perbedaan "tidak diset" vs "diset kosong" |
| `internal/service` | transformasi `key_hint`, termasuk kasus kunci pendek yang tidak menyembunyikan apa pun |
| `internal/router` | setiap route melalui mux sungguhan: status, verbe salah, duplikat nama, `request_id`, siklus hidup create → get → delete |
| `internal/handler` | create/list/get/update/revoke termasuk paginasi dan validasi payload |
| `internal/repository/postgres` | pemetaan error driver → domain (unit); constraint, paginasi, dan round-trip terhadap PostgreSQL nyata (integrasi) |

### Test integrasi

Test yang butuh PostgreSQL nyata diberi build tag `integration`, sehingga `go test ./...` biasa tetap hermetik dan tidak mengompilasinya sama sekali (AGENTS.md §2.1 melarang `t.Skip` sebagai cara menghindari test):

```bash
PANNELAI_TEST_POSTGRES_DSN='postgres://user:pass@localhost:5432/pannelai_test?sslmode=disable' \
  go test -race -tags=integration ./internal/repository/postgres/
```

Test itu menjalankan migrasi ke database tersebut lalu `TRUNCATE gateway_keys`, jadi arahkan ke database khusus uji. Karena default run tidak mengompilasinya, laporan `ok` pada `go test ./...` **tidak** berarti constraint database sudah teruji; jalankan perintah bertag di atas saat menyentuh skema atau repository.

## Dependensi pihak ketiga

Sesuai AGENTS.md "Stack", pustaka standar dipakai lebih dulu. Dua pengecualian yang tidak terhindarkan, plus satu klien Redis:

| Modul | Alasan |
|---|---|
| `github.com/go-playground/validator/v10` | validasi struct-tag; pengecualian yang disebut AGENTS.md |
| `github.com/jackc/pgx/v5` | driver PostgreSQL; pengecualian yang disebut AGENTS.md |
| `github.com/redis/go-redis/v9` | klien Redis; Redis wajib untuk sesi, limiter, dan circuit state |

ULID dihasilkan sendiri (`internal/domain/ulid.go`) alih-alih menambah dependensi.