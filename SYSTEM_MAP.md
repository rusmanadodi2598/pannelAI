# SYSTEM_MAP.md

SSoT untuk topologi, batas domain, alur data, dan antrean asinkron **pannelAI**.
Diperbarui pada PR yang sama ketika topologi atau alur data berubah (AGENTS.md §1.9).

| | |
|---|---|
| **Status** | P0 selesai: config, migrasi, health/version, auth sesi, gateway keys, Redis lockout/rate limit, dan quality gates tercover. **P1 CLOSED**: registry provider di-embed (94 provider, decode ketat), seam plugin per provider, agregat `UpstreamEndpoint`/`UpstreamKey`/`ProviderNode` dengan circuit breaker per key, penyegel AES-256-GCM, dan migrasi P1 (000004-000008) terverifikasi terhadap PostgreSQL nyata. Seluruh endpoint manajemen P1 (§7.4-§7.8, §7.12-§7.14) plus data plane chat OpenAI+Anthropic dan embeddings terpasang dan teruji; multi-akun dan bulk onboarding (endpoint batch, key batch, OAuth import) lengkap dengan semantik all-or-nothing; adapter visi (§7.8) ikut menambah urutan model di jalur request lewat seam `dataplane.VisionAugmenter` dengan rotasi round-robin di Redis. Dua worker P1 berjalan: quota flush (Redis → PostgreSQL) dan log retention (purge per `retention_days`). Kriteria keluar P1 terpenuhi: `Engine.Relay` menuntaskan fallback combo end-to-end diuji di `internal/dataplane/engine_relay_test.go`, dan `go test -race ./...` bersih. Panel U0 selesai termasuk shell sidebar bertema; layar Usage dan Quota panel menyusul di atas P1 API |
| **Terakhir diperbarui** | 2026-09-19 |
| **Kontrak** | `docs/SPEC-API/001-SPEC-API.md` |

---

## 1. Topologi

Dua aplikasi, satu repositori. Keduanya berbagi PostgreSQL dan Redis secara logis; `app-ui` tidak pernah mengakses keduanya langsung.

```mermaid
flowchart LR
    CLI["CLI tools: Claude Code, Codex, Cursor"]
    UI["app-ui: Svelte panel U0 (login, gateway keys, settings, shell sidebar)"]

    subgraph serv["app-serv (Go 1.26)"]
        direction TB
        MW["Middleware: requestID, logging, recoverer, envelope, rate limit"]
        RT["router: /api/v1 + session guard"]
        H["handler"]
        S["service"]
        R["repository"]
        MW --> RT --> H --> S --> R
    end

    PG[("PostgreSQL: konfigurasi, usage, logs")]
    RD[("Redis: sesi, rate limit, sticky RR")]
    UP["Provider upstream"]

    CLI -->|"Bearer gateway key"| MW
    UI -->|"session cookie"| MW
    R --> PG
    S --> RD
    S -.->|"P1: translasi + upstream"| UP
```

**Batas domain saat ini:** `gateway_keys` dan `panel_auth` (P0); `provider_nodes`, `upstream_endpoints`, `upstream_keys`, `combos`, `model_aliases`, `models_custom`, `models_disabled`, `usage_records`, `quota_windows`, `quota_caps`, `request_logs`, dan `settings` (P1, migrasi 000004-000008; repository, service, handler, dan data plane-nya terpasang). Provider registry bukan tabel: ia dokumen YAML yang di-embed ke binary (§5).

**Catatan migrasi P1:** kolom `quota_windows."window"` adalah reserved word PostgreSQL dan wajib dikutip; nama kolomnya dipertahankan agar sama dengan field API (SPEC-API §7.12 mengembalikan `window`). Idempotensi runner diuji, bukan diasumsikan: `migrations/apply_test.go` (tag `integration`) menjalankan runner sungguhan dua kali dan memastikan ledger tidak bertambah.

---

## 2. Batas Layer

Alur satu arah, dipaksakan compiler lewat import antar-package (AGENTS.md §1.5):

```
schema → domain → repository → service → handler → router
```

| Layer | Paket | Tanggung jawab | Tidak boleh |
|---|---|---|---|
| schema | `internal/schema` | DTO, tag validasi, decoding, serialisasi respons | memanggil service atau repository |
| domain | `internal/domain` | Entitas, value object, transisi state, envelope error | mengimpor `net/http` |
| repository | `internal/repository`, `.../postgres` | Kontrak penyimpanan dan implementasinya | mengimpor service atau handler |
| service | `internal/service` | Orkestrasi use case, panggilan keluar dengan timeout | mengimpor `net/http` |
| handler | `internal/handler` | Decode, panggil service, encode | memuat SQL atau Redis langsung |
| router | `internal/router` | Tabel route dan middleware lintas-potong | memuat logika bisnis |
| config | `internal/config`, `internal/registry`, `internal/provider` | Konfigurasi env bertipe, registry provider yang di-embed, dan seam plugin konektivitas per provider | memuat logika use case atau SQL |

`cmd/app-serv/main.go` adalah composition root: hanya wiring, tanpa logika.

### 2.1 Seam plugin provider (wajib, permintaan owner)

Setiap provider berbeda dalam konektivitas: ada yang **api_key**, ada yang **OAuth**, ada yang **gratis tanpa kredensial**, dan ada yang menerima **keduanya**. Perbedaan itu tidak boleh masuk ke core, karena satu patch provider lalu menjadi perubahan kode bersama.

`internal/provider` memisahkannya:

| Bagian | Isi |
|---|---|
| `Plugin` | Antarmuka satu provider: `Endpoint` (URL), `ApplyAuth` (header dan skema), `DecodeUsage`, `ShouldRetry`, `IsQuotaError` |
| `Base` | Perilaku bawaan yang di-embed tiap connector, sehingga provider baru cukup mengisi `ProviderID` |
| `Default` | Connector fallback: URL dari registry, header dan skema dari `transport.auth`, jadi vendor OpenAI/Claude-compatible tidak perlu berkas baru |
| `Connectors` | Lookup `provider id -> Plugin`, menolak id ganda, dan `Unsupported()` melaporkan provider yang butuh connector tapi belum ada |

**Konsekuensi operasional:** menambah provider = menambah entri di `registry.yaml` (speak format standar) atau satu berkas connector (protokol khusus). Tidak ada `switch` pada provider id di core, sehingga provider A bisa di-patch tanpa menyentuh provider B.

**Batasan yang diketahui:** `Connectors.Unsupported()` sengaja melaporkan provider ber-format khusus (mis. `kiro`, `cursor`, `antigravity`) selama connector-nya belum ditulis; laporan itu adalah daftar kerja, bukan kegagalan diam.

---

## 3. Alur Data

### 3.1 Permintaan manajemen (P0: gateway keys)

```mermaid
sequenceDiagram
    participant C as Client
    participant R as router
    participant H as handler
    participant S as service
    participant P as PostgreSQL

    C->>R: POST /api/v1/gateway-keys {name}
    R->>H: Create
    H->>H: DecodeJSON + ValidateStruct
    H->>S: Create(name)
    S->>S: generate plaintext, SHA-256 digest, hitung hint
    S->>P: INSERT gateway_keys
    S-->>H: aggregate + plaintext
    H-->>C: 201 {plaintext_key, key_hint, ...}
```

Plaintext hanya muncul pada respons create. Setiap pembacaan setelahnya hanya membawa `key_hint` (`sk-…abcd`), dan digest SHA-256 adalah satu-satunya bentuk yang tersimpan (SPEC-API-001 §4, §6).

### 3.2 Permintaan panel (app-ui U0)

Shell panel dirender dari data navigasi di `src/lib/navigation.ts`: lima grup yang masing-masing menjawab satu
pertanyaan operator (Configure, Observe, Optimize, Developer, System) berisi total 19 baris. Baris yang
layarnya belum dibangun tidak membawa `href`, sehingga tidak bisa dirender sebagai link; R-24 jadi sifat tipe,
bukan disiplin review. Tiga bentuk responsif: drawer di bawah 768px, rail ikon 64px pada 768-1023px, dan
sidebar 264px pada 1024px ke atas, dengan preferensi collapse disimpan di cookie.

```mermaid
sequenceDiagram
    participant B as Browser
    participant U as app-ui (SvelteKit on Bun)
    participant S as app-serv
    participant R as Redis

    B->>U: POST /api/v1/auth/login {password}
    U->>S: POST /api/v1/auth/login (forwarded, PANEL_API_TARGET)
    S->>S: bcrypt compare + HMAC nonce
    S->>R: SET session digest with TTL
    S-->>U: 204 + HttpOnly pannel_session
    U-->>B: redirect to endpoint keys
    B->>U: GET /endpoint-keys
    U->>S: GET /api/v1/auth/status + session cookie
    S->>R: EXISTS session digest
    S-->>U: authenticated=true
```

The panel forwards `/api/v1` from its own server, so the browser talks to one
origin and no CORS rule is needed. The panel never touches PostgreSQL or Redis;
the API is its only surface (SPEC-API-001 §11.5).

### 3.3 Autentikasi dan revocation

`POST /api/v1/auth/login` memverifikasi bcrypt hash dari `panel_auth`, membuat
nonce 32-byte, menandatanganinya dengan HMAC-SHA256 `SESSION_SECRET`, lalu
menyimpan digest nonce di Redis selama `SESSION_TTL`. Cookie `pannel_session`
bersifat HttpOnly, SameSite=Lax, Path `/`, dan Secure pada production.

`POST /api/v1/auth/logout` menghapus digest dari Redis. Guard sesi memverifikasi
signature dan keberadaan digest sebelum logout, change-password, atau route
`gateway-keys` dipanggil. Login failure counter dan lockout disimpan di Redis;
lima kegagalan berturut-turut memicu `LOGIN_LOCKOUT` (default 15 menit).

### 3.4 Siklus hidup status

```mermaid
stateDiagram-v2
    [*] --> active
    active --> disabled
    disabled --> active
    active --> revoked
    disabled --> revoked
    revoked --> [*]
```

`revoked` bersifat terminal: transisi keluar ditolak dengan `CONFLICT`.

---

## 4. Endpoint Aktif (P0)

Setiap respons membawa `X-Request-Id` (dibuat bila tidak dikirim pemanggil) dan setiap request dicatat satu baris `slog` dengan id itu (SPEC-API-001 §4, §8).

| Method | Path | Auth | Sumber |
|---|---|---|---|
| GET | `/api/v1/health` | publik | liveness + PostgreSQL + Redis; `503` saat salah satu dependency tidak menjawab |
| GET | `/api/v1/version` | publik | build version, commit, registry revision |
| POST | `/api/v1/auth/login` | publik | bcrypt login; 204 + HttpOnly session cookie |
| GET | `/api/v1/auth/status` | publik | authenticated, require_login, password_configured |
| POST | `/api/v1/auth/logout` | S | revoke session; 204 + deletion cookie |
| POST | `/api/v1/auth/change-password` | S | verify current password; 204 |
| GET | `/api/v1/gateway-keys` | S | daftar (hint saja) + meta paginasi |
| POST | `/api/v1/gateway-keys` | S | buat; plaintext sekali |
| GET | `/api/v1/gateway-keys/{id}` | S | detail (hint saja) |
| PATCH | `/api/v1/gateway-keys/{id}` | S | ubah `name`, `status` |
| DELETE | `/api/v1/gateway-keys/{id}` | S | revoke (soft) |

Route metode-aware (Go 1.22 `ServeMux`), sehingga verbe yang salah dijawab mux dan dinormalkan ke envelope §8.

Route tidak dikenal dijawab `404 NOT_FOUND`, verbe salah dijawab `405 METHOD_NOT_ALLOWED`; keduanya lewat envelope §8 yang sama seperti error lain.

Semua endpoint manajemen selain health/version digerbangi sesi. `RATE_LIMIT_PER_MIN`
ditegakkan melalui Redis fixed-window middleware untuk traffic `/api/v1` selain
health/version (operational probes harus tetap dapat melaporkan Redis failure);
login memiliki counter dan lockout Redis terpisah.

## 4a. Migrasi

Berkas `app-serv/migrations/*.up.sql` di-embed ke binary dan diterapkan saat boot, dicatat di `schema_migrations`. Satu migrasi dijalankan paling banyak sekali per database, dalam satu transaksi bersama baris ledger-nya. Ini berlaku untuk P0 single-instance; produksi multi-replika harus memindahkannya ke job rilis terpisah.

Migrasi P1 (`000004`-`000008`) menambah `provider_nodes`, `upstream_endpoints` + `upstream_keys`, `combos` + `model_aliases` + `models_custom` + `models_disabled`, `usage_records` + `quota_windows` + `quota_caps`, `request_logs` + `settings`. Dua catatan yang lahir dari menjalankannya terhadap PostgreSQL nyata, bukan dari membaca kodenya:

- kolom `quota_windows."window"` **wajib dikutip**: `window` adalah reserved word di PostgreSQL, dan tanpa kutip migrasinya gagal parse. Nama kolomnya dipertahankan agar sama dengan field API (SPEC-API §7.12 mengembalikan `window`), bukan diganti demi parser lalu dipetakan balik di setiap query.
- idempotensi diuji, bukan diasumsikan: `migrations/apply_test.go` (tag `integration`) menjalankan runner sungguhan dua kali dan memastikan ledger tidak bertambah.

---

## 5. Penyimpanan

| Sumber | Isi | Catatan |
|---|---|---|
| PostgreSQL | `gateway_keys` + singleton `panel_auth` (P0), `provider_nodes`, `upstream_endpoints`, `upstream_keys`, `combos`, `model_aliases`, `models_custom`, `models_disabled`, `usage_records`, `quota_windows`, `quota_caps`, `request_logs`, `settings`, `schema_migrations` (P1) | pool limit eksplisit; setiap kolom lookup terindeks; `gateway_keys.name` UNIQUE dan `value_hash` terindeks untuk autentikasi data plane; `upstream_keys.value_encrypted` dan token OAuth disegel AES-256-GCM (`internal/domain/secret.go`), `key_hint` satu-satunya bentuk yang dibaca kembali |
| Redis | `pannelai:auth:session:*`, login failure/lockout keys, gateway rate limit, sticky round-robin, circuit state, console ring buffer | dibutuhkan untuk limiter dan state; session digest langsung dapat dicabut |

---

## 6. Asinkron

Tiga worker berjalan bersama server, semuanya dipulai lewat `cmd/app-serv/worker_wiring.go` dengan batas panic dan terminasi lewat context (AGENTS.md §1.6). Antrean berikut masuk pada P2 sesuai SPEC-API §3 dan §10:

| Worker | Pemicu | Kebijakan retry | Dead-letter |
|---|---|---|---|
| Quota flush (`internal/service/quota_flush.go`) | tick 30 detik, batch terbatas | fixed tick, `MaxAttempts: 5` | baris Redis tersisa dicoba lagi pada tick berikutnya |
| Log retention (`internal/service/log_retention.go`) | tick 1 jam, cutoff dari `settings.logging.retention_days` | fixed tick, `MaxAttempts: 3` | DELETE bersifat set-based dan atomik; baris tetap untuk percobaan berikutnya |
| OAuth token refresh (`internal/service/oauth_refresh_worker.go`) | tick 5 menit, hanya endpoint `oauth` yang `refresh_state=due` | eksponensial dari 30 detik, jitter maksimum seperempat delay, plafon 30 menit, 5 percobaan | percobaan kelima menandai endpoint `error` lewat `MarkRefreshDeadLetter` dan berhenti di-retry |
| Quota re-check | kadens per provider (5h/daily/weekly/monthly) | eksponensial (P2) | catat `resets_at` terakhir |

Alur OAuth §7.4 (`internal/service/oauth_flow*.go`) memakai dua state eksternal: `state` single-use 10 menit di Redis (`pannelai:oauth:state:*`, `SET NX` + `GETDEL`) sebagai replay guard, dan token yang disegel AES-GCM pada `upstream_endpoints.oauth`. `POST .../oauth/start`, `GET .../oauth/status`, dan `POST .../oauth/refresh` adalah rute sesi; `GET .../oauth/callback` publik karena browser provider tidak bisa membawa cookie sesi. Tujuan redirect browser tidak pernah diambil dari request: `PUBLIC_BASE_URL` menang, origin `redirect_uri` hanya fallback absolut http(s), dan bila keduanya tidak ada callback menjawab JSON.

Setiap goroutine baru wajib memulihkan panic dan punya kondisi terminasi eksplisit (AGENTS.md §1.6).

---

## 7. Perubahan yang Memicu Pembaruan Berkas Ini

- tabel atau domain baru
- endpoint baru atau perpindahan batas auth
- penambahan worker atau antrean asinkron
- perubahan pembagian tanggung jawab antar-layer