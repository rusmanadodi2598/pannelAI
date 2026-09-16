# pannelAI

<p>
  <img src="https://img.shields.io/badge/Go-0081A1?style=flat-square&logo=go&logoColor=white" alt="Go" title="Go" />
  <img src="https://img.shields.io/badge/TypeScript-3178C6?style=flat-square&logo=typescript&logoColor=white" alt="TypeScript" title="TypeScript" />
  <img src="https://img.shields.io/badge/Python-397BB2?style=flat-square&logo=python&logoColor=white" alt="Python" title="Python" />
  <img src="https://img.shields.io/badge/Svelte-DF3600?style=flat-square&logo=svelte&logoColor=white" alt="Svelte" title="Svelte" />
  <img src="https://img.shields.io/badge/PostgreSQL-4169E1?style=flat-square&logo=postgresql&logoColor=white" alt="PostgreSQL" title="PostgreSQL" />
  <img src="https://img.shields.io/badge/Redis-DB3A30?style=flat-square&logo=redis&logoColor=white" alt="Redis" title="Redis" />
  <img src="https://img.shields.io/badge/Zod-3674D7?style=flat-square&logo=zod&logoColor=white" alt="Zod" title="Zod" />
</p>

API Gateway AI berbasis Go untuk mengelola akses klien, provider, upstream endpoint, dan pemilihan model melalui satu layanan backend.

Proyek memisahkan tanggung jawab **Control Plane** (API manajemen) dan **Data Plane** (pemrosesan permintaan AI) secara logis di dalam `app-serv`, bukan sebagai dua server terpisah.

> **Status: dalam pengembangan.** Komponen data plane sudah tersedia di kode, tetapi keberadaan modul belum berarti seluruh alur HTTP, streaming, dan accounting telah terintegrasi atau teruji end-to-end. Dashboard `app-ui` masih direncanakan; spesifikasinya tersedia di `docs/SPEC-UI/`.

Pohon di bawah adalah **struktur target** yang menjadi acuan pengembangan. Saat ini repositori baru memuat dokumen spesifikasi dan aturan, ditambah `deployment/` dan `docs/SPEC-UI/` sebagai direktori penanda; `app-serv/`, `app-ui/`, dan `scrypts/` belum dibuat.

## Struktur proyek

```text
pannelAI/
├── app-serv/                   # Backend Go
│   ├── cmd/app-serv/           # Entry point dan perakitan dependensi
│   ├── internal/
│   │   ├── config/             # Konfigurasi environment
│   │   ├── dataplane/          # Resolve, select, translate, upstream
│   │   ├── domain/             # Entitas, aturan domain, interface storage
│   │   ├── handler/            # HTTP handlers dan middleware
│   │   ├── registry/           # Registry provider dan registry.yaml
│   │   ├── repository/         # Akses PostgreSQL dan Redis
│   │   ├── router/             # Pemetaan route HTTP
│   │   ├── schema/             # DTO, decoding, validasi request
│   │   └── service/            # Logika bisnis API manajemen
│   ├── migrations/             # Migrasi database
│   ├── .env.example            # Contoh konfigurasi
│   ├── go.mod                  # Modul dan dependensi Go
│   └── README.md               # Dokumentasi backend
├── app-ui/                     # Panel Svelte (Bun toolchain)
│   ├── src/                    # Komponen dan route panel
│   ├── static/                 # Aset statis
│   ├── package.json            # Dependensi dan skrip Bun
│   └── README.md               # Dokumentasi panel
├── docs/
│   ├── SPEC-API/               # Kontrak API dan OpenAPI
│   ├── SPEC-UI/                # Spesifikasi dashboard
│   └── RULLES/                 # Aturan kerja dan protokol pengujian
├── scrypts/                    # Nama direktori mengikuti repositori
│   ├── gates/                  # Quality gates
│   ├── hooks/                  # Git hooks
│   └── lib/                    # Helper skrip
├── deployment/                 # Berkas dan catatan deployment
├── .gitlab-ci.yml              # Konfigurasi CI
├── AGENTS.md                   # Pedoman kontribusi dan coding
├── SYSTEM_MAP.md               # Peta sistem
├── .gitignore
└── README.md
```

Panel `app-ui` hanya mengonsumsi API manajemen `app-serv` dan tidak mengakses PostgreSQL atau Redis secara langsung.

## Stack dan pembagian tanggung jawab

Fondasi backend menggunakan **Go**, HTTP standar **`net/http`**, **PostgreSQL** (`pgx/v5`), dan **Redis** (`go-redis/v9`). Validasi DTO backend menggunakan `go-playground/validator/v10`; validasi di sisi panel memakai **Zod**. Panel dibangun dengan **Svelte** dan toolchain **Bun**.

| Bagian | Tanggung jawab |
| --- | --- |
| Control Plane | Autentikasi sesi, pengelolaan gateway keys, katalog provider, upstream endpoints dan keys, combos, konfigurasi vision adapter |
| Data Plane | Komponen normalisasi request, resolusi model/combo, pemilihan endpoint/key, translasi format provider, HTTP upstream dengan retry/timeout |
| PostgreSQL | Data konfigurasi dan repository untuk usage, latency, serta request logs |
| Redis | Sesi, penghitung rate limit, dan state sticky round-robin |
| Dashboard | Panel Svelte (direncanakan) yang mengonsumsi API Control Plane |

Diagram awal disesuaikan dengan fondasi repo: tidak mengasumsikan Fiber/FastHTTP, Chi/Echo/Gin, SQLite, atau cache respons in-memory. Redis bukan hanya cache, dan kedua plane dapat membutuhkan PostgreSQL maupun Redis.

## Arsitektur API Gateway (Mermaid.js)

Diagram berikut menggambarkan **alur logis yang dituju** berdasarkan komponen yang tersedia. Garis putus-putus pada akses klien data plane dan dashboard menandai integrasi yang belum diverifikasi end-to-end, bukan jaminan endpoint sudah siap produksi.

```mermaid
flowchart LR
    Client["Client Request: OpenAI schema"]
    Admin["Admin / API client"]
    UI["UI Dashboard: direncanakan"]

    subgraph Gateway["Go AI Gateway Engine - app-serv"]
        direction TB
        subgraph DP["Data Plane - komponen dalam pengembangan"]
            Guard["Gateway authentication dan rate limit: integrasi ingress"]
            Normalize["Validasi dan normalisasi request"]
            Select["Resolusi model / combo dan pemilihan endpoint / key"]
            Translate["Translasi format dan panggilan upstream"]
            Record["Accounting: usage, latency, request logs"]
            Guard --> Normalize --> Select --> Translate
            Translate --> Record
        end
        subgraph CP["Control Plane - API manajemen"]
            Session["Autentikasi sesi"]
            Manage["Gateway keys, provider catalog, endpoints, combos, vision config"]
            Session --> Manage
        end
    end

    Redis[("Redis: sessions, rate limits, sticky rotation")]
    PG[("PostgreSQL: configuration, usage, logs")]
    Registry["Embedded provider registry"]
    Provider["Upstream AI Providers"]

    Client -.-> Guard
    Admin --> Session
    UI -.-> Session
    Session <--> Redis
    Manage <--> PG
    Manage --> Registry
    Guard <--> Redis
    Select <--> Redis
    Select --> PG
    Select --> Registry
    Translate -->|"Request sesuai format provider"| Provider
    Provider -->|"Response"| Translate
    Record --> PG
    Record -.->|"Respons ke klien setelah integrasi"| Client
```

## Contoh kasus data flow

**Skenario:** klien meminta model melalui sebuah combo dengan beberapa kandidat upstream.

1. Pada alur ingress yang dituju, gateway memeriksa akses, rate limit, dan payload klien.
2. Komponen data plane menormalisasi request ke bentuk chat internal, lalu meresolusikan combo, alias, atau referensi `provider/model`.
3. Kandidat diurutkan sesuai strategi combo. Endpoint dan key dipilih berdasarkan prioritas serta status yang dapat digunakan; sticky round-robin memakai Redis.
4. Request diterjemahkan ke format upstream. Implementasi renderer mencakup OpenAI, Claude, dan Gemini; hal ini bukan klaim seluruh fitur ketiganya sudah kompatibel.
5. Helper transport menyediakan retry untuk kegagalan jaringan, HTTP 429, dan 5xx. Pemindahan antar kandidat merupakan bagian dari integrasi alur fallback, berbeda dari retry ke target yang sama.
6. Repository menyediakan penulisan usage, latency, request logs, serta pembaruan penggunaan key. Integrasi pencatatan dan pengembalian respons perlu diuji sebagai satu alur lengkap.

Alur ini tidak menganggap dashboard analytics, streaming penuh, atau semua strategi combo sudah selesai. Strategi `fusion` belum didukung oleh komponen resolusi yang diperiksa.

## Dokumentasi lanjutan

- [Kontrak API](docs/SPEC-API/001-SPEC-API.md)
- [Aturan TDD](docs/RULLES/TDD.md)
- [Spesifikasi UI](docs/SPEC-UI/) (direncanakan)
- [Backend dan konfigurasi](app-serv/README.md) (direncanakan)
- [Contoh environment](app-serv/.env.example) (direncanakan)
- [Peta sistem](SYSTEM_MAP.md) (direncanakan)
- [Pedoman kontribusi](AGENTS.md) (direncanakan)
- [Quality gates](scrypts/gates/) (direncanakan)

Dokumentasi ini menjelaskan struktur dan rancangan alur; bukan pernyataan bahwa build, lint, atau seluruh pengujian telah lulus.

## Author

Dibangun oleh [rusmanadodi2598](https://github.com/rusmanadodi2598).