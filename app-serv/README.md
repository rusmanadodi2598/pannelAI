# app-serv

Backend Go untuk pannelAI: API manajemen (control plane) dan proxy kompatibel OpenAI (data plane).

Kontrak perilaku ada di [`docs/SPEC-API/001-SPEC-API.md`](../docs/SPEC-API/001-SPEC-API.md); aturan koding mengikat ada di [`AGENTS.md`](../AGENTS.md).

## Status

**P0 selesai; P1 sedang berjalan.** Yang tersedia dan teruji:

P0 (selesai):

- konfigurasi environment bertipe dengan validasi fail-fast
- koneksi PostgreSQL (pgx/v5) dan Redis (go-redis/v9) dengan limit pool eksplisit
- migrasi `gateway_keys` dan singleton `panel_auth`
- endpoint sistem: `/api/v1/health`, `/api/v1/version`
- auth sesi: login, logout, status, change-password
- CRUD gateway keys: list, create, get, patch, revoke, seluruhnya session-gated
- HMAC-signed HttpOnly cookie, Redis session revocation, bcrypt password hash
- Redis login lockout (5 kegagalan → 15 menit default) dan gateway rate limit
- middleware: pemulihan panic, envelope error §8, 404/405 dinormalkan

P1 sejauh ini (kode ada dan teruji; endpoint HTTP-nya belum semua terpasang):

- **Registry provider di-embed**: `internal/registry/registry.yaml` dihasilkan `tools/registry-gen.mjs` dari referensi 9Router, 94 provider, di-decode ketat (`KnownFields`) sehingga field yang tidak dikenal menggagalkan boot, bukan hilang diam-diam
- **Seam plugin provider** (`internal/provider`): lookup `provider id → Plugin`, connector fallback untuk vendor OpenAI/Claude-compatible, dan `Unsupported()` yang melaporkan provider ber-protokol khusus yang belum punya connector
- **Agregat endpoint**: `UpstreamEndpoint` menahan 1..N `UpstreamKey`; endpoint `api_key` wajib menyisakan minimal satu key aktif; circuit breaker per key (3 kegagalan → backoff 2 menit, sukses mereset)
- **Migrasi P1** (000004-000008): provider nodes, upstream endpoints + keys, combos dan katalog model, usage dan quota, request logs dan settings

Belum terpasang: repository dan service endpoint, handler dan route CRUD-nya, bulk onboarding (endpoint batch, key batch, import kredensial OAuth), combos, vision adapter, data plane chat, usage/quota read, logs, settings. Lihat SPEC-API-001 §7 dan §10 untuk P1 sampai P3.

> `PANEL_BOOTSTRAP_PASSWORD` hanya dipakai saat row `panel_auth` belum memiliki hash. Setelah bootstrap, ubah password melalui endpoint change-password; env tidak menimpa hash yang sudah ada.

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

# Set PANEL_BOOTSTRAP_PASSWORD before the first boot, then use a cookie jar.
curl -i -c cookies.txt -X POST localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' -d '{"password":"your-panel-password"}'

curl -b cookies.txt localhost:8080/api/v1/auth/status
curl -b cookies.txt -X POST localhost:8080/api/v1/gateway-keys \
  -H 'Content-Type: application/json' -d '{"name":"cli"}'

curl -b cookies.txt localhost:8080/api/v1/gateway-keys
curl -b cookies.txt -X DELETE localhost:8080/api/v1/gateway-keys/{id}
curl -b cookies.txt -X POST localhost:8080/api/v1/auth/logout
```

Login, logout, dan change-password berhasil dengan `204 No Content`; kredensial sesi berada di cookie `pannel_session` (HttpOnly, SameSite=Lax). Respons create memuat `plaintext_key` satu kali. Setelah itu hanya `key_hint` (`sk-…abcd`) yang dikembalikan; yang tersimpan hanyalah digest SHA-256.

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
| `internal/router` | route `/api/v1` melalui mux sungguhan: auth login/status/logout, session gating, status, verbe salah, duplikat nama, `request_id`, limiter, siklus hidup create → get → delete |
| `internal/handler` | auth happy/validation/auth cases, cookie lifecycle, create/list/get/update/revoke, paginasi, dan validasi payload |
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
| `gopkg.in/yaml.v3` | membaca registry provider yang di-embed (SPEC-API-001 §6); hanya dipakai `internal/registry` |


## Registry provider dan seam plugin

### Registry di-embed

`internal/registry/registry.yaml` adalah sumber katalog provider (SPEC-API-001 §6). Berkas itu **dihasilkan**, bukan ditulis tangan:

```bash
node tools/registry-gen.mjs /path/ke/9router internal/registry/registry.yaml
```

Aturan yang membuatnya aman:

- decode **ketat** (`KnownFields(true)`): key yang tidak punya tag struct menggagalkan boot. Ini bukan kekakuan demi kekakuan; inilah yang menangkap drift saat porting, dan ia benar-benar menangkap 12 kelas field yang hilang pada percobaan pertama.
- `client_secret` **tidak** disalin dari referensi. Checkout referensi memuat kredensial pihak ketiga yang hidup, dan berkas ini di-commit; alur yang butuh secret membacanya dari config bertipe saat flow-nya diimplementasikan.
- identifier ganda ditolak saat load, bukan saat request.
- resolusi nama mengikuti referensi, bukan intuisi: **alias menang atas id milik provider lain**. Referensi membangun satu tabel alias datar (`uiAlias || alias`) dan resolve dengan `ALIAS_TO_ID[t] || t`, sehingga provider yang id-nya dipakai sebagai alias orang lain tidak terjangkau lewat id itu. Kasus itu nyata di data (`mimo-free` ber-`alias: mmf`, sementara entri tersembunyi ber-`id: mmf`), dan keduanya menuju base URL serta model yang sama; alias-vs-alias tetap fatal karena tidak ada dasar untuk memilih salah satu.
- **routability** dipublikasikan per provider: `native` untuk format yang diterjemahkan gateway sendiri (OpenAI, Claude, OpenAI Responses), `connector` untuk protokol khusus yang butuh connector. Tanpa itu, beberapa provider pada daftar owner (`commandcode`, `gemini-cli`, `kiro`, `gemini`, `cursor`, `antigravity`) bisa dikonfigurasi tapi tidak akan pernah menjawab.
- **blok media** (`media.embedding`, `media.image`, …) ikut di-port karena sebuah layanan media sering menaruh kredensialnya berbeda dari transport chat provider yang sama: Gemini memakai `auth_header: key` (query parameter) untuk embeddings, sementara chat-nya memakai header. `auth_header: key` berarti **query parameter**, bukan header.

### Seam plugin

Setiap provider berbeda dalam konektivitas (api_key, OAuth, gratis tanpa kredensial, atau keduanya). `internal/provider` memisahkan perbedaan itu dari core:

```go
type Plugin interface {
	ProviderID() string
	AuthType() string
	Endpoint(req Request, cred Credential) (string, error)
	ApplyAuth(req *http.Request, cred Credential) error
	DecodeUsage(status int, header http.Header) Usage
	ShouldRetry(status int, header http.Header) RetryDecision
	IsQuotaError(status int, body []byte) bool
}
```

Menambah provider:

1. **Format standar** (OpenAI atau Claude): cukup tambahkan entri di `registry.yaml`. `provider.Default` menangani URL dan auth dari entri itu. Tidak ada berkas Go baru.
2. **Protokol khusus**: buat satu berkas di `internal/provider/`, embed `Base`, daftarkan di `NewConnectors(...)` pada composition root. Tidak ada perubahan di core maupun provider lain.

`Connectors.Unsupported(idx)` melaporkan provider di registry yang formatnya belum didukung fallback dan belum punya connector. Laporan itu adalah daftar kerja yang terlihat, bukan kegagalan diam saat traffic datang.

ULID dihasilkan sendiri (`internal/domain/ulid.go`) alih-alih menambah dependensi. `gopkg.in/yaml.v3` adalah dependensi ketiga di luar daftar AGENTS.md Stack, ditambahkan untuk membaca registry yang di-embed; ia hanya dipakai di `internal/registry`.