# 062 - Audit UX & UI Panel (`app-ui`) di Atas Runtime Bun 1.3+

| | |
|---|---|
| **Status** | Active Audit & Remediation Blueprint |
| **Owner** | Dodi Rusmana <rusmanadodi@kentangtech.com> |
| **Tanggal** | 2026-10-17 |
| **Cakupan** | `app-ui/**` (Runtime Bun `>=1.3.0`, terukur `v1.4.2`), kontrak `DESIGN.md` (§1-§12, R-01 s.d. R-38), `SYSTEM_MAP.md` (§3.2, §3.5-§3.7, §6), dan `app-serv/internal/handler/openapi.json` |
| **Pendahulu** | `docs/DRAFT/009-PLAYGROUND-CHAT-ENDPOINT-READINESS.md`, `docs/DRAFT/010-USAGE-ENDPOINT-READINESS.md`, `draft 013`-`016`, `draft 023`, `draft 035`, `draft 043`, `draft 050`, `draft 052`, `draft 059`, `draft 060` |

---

## 1. Ringkasan Eksekutif & Konteks Runtime (Bun 1.3 Up)

Dokumen ini mencatat hasil audit menyeluruh atas pengalaman pengguna (UX), antarmuka operator (UI), kepatuhan terhadap **DESIGN.md** (aturan anti-slop `R-01` s.d. `R-38`), serta keselarasan kontrak antara frontend panel (`app-ui`) dan permukaan API `app-serv` (`/api/v1/*`) pada runtime **Bun 1.3+** (terukur di environment: `bun v1.4.2`).

Sesuai `DESIGN.md §1`, `pannelAI` adalah **instrumen operator teknis**, bukan halaman pemasaran:

1. **Dials (`DESIGN.md §2.1`)**: `ENERGY 1 / RHYTHM 2 / MOTION 1`. Kepadatan informasi (tabel 36px, sidebar touch target 44px) menjadi prioritas utama; dekorasi tanpa fungsi dilarang keras.
2. **Satu Aksen & Motif Identitas (`DESIGN.md §3.4, §6`)**: Aksen koral (`#E56A4A` pada dark, `#B8412A` pada light) dibatasi tepat di **lima tempat** (baris navigasi aktif dengan marker kiri 3px + ikon, satu aksi primer per layar, focus ring, link, dan garis bawah tab aktif). Status kesehatan selalu memakai tiga warna status (`--color-ok`, `--color-warn`, `--color-danger`) yang wajib berdampingan dengan label teks (`R-25`).
3. **Kejujuran Kontrak (`R-26`, `R-36`, `R-38`)**: Tidak boleh ada tombol mati (*dead control*), angka nol palsu untuk field yang tidak dikirim wire (`not stated`), atau klaim kapabilitas yang belum dilayani `app-serv`.

---

## 2. Verifikasi Toolchain & Runtime FE (`Bun >= 1.3.0`)

| Komponen | Spesifikasi Wajib (`AGENTS.md` / `gates-frontend.yml`) | Kondisi Terukur | Status |
|---|---|---|---|
| Runtime & Package Manager | `Bun 1.3 (Up)` (`bun install`, `bun run`) | `bun v1.4.2` | **PASS** |
| Type Checker | `TypeScript` (`bunx --bun tsc --noEmit`) | Zero error | **PASS** |
| Bundler & Dev Server | Vite (`0.0.0.0:3000`, single-origin `/api/v1` forwarding) | `bun run dev` / `bun run build` | **PASS** |
| Batas Baris Berkas (`AGENTS.md §1.1`) | `< 250` baris per berkas sumber non-generated | Modularisasi komponen per domain layar | **ENFORCED** |

---

## 3. Matriks Audit Kepatuhan `DESIGN.md` (R-01 s.d. R-38)

| Aturan | Topik | Temuan Audit pada `app-ui` | Status |
|---|---|---|---|
| **R-01** | Warna & Gradien | Nol gradien, nol background wash. Satu aksen koral (`#E56A4A` / `#B8412A`) di atas warm neutrals (`#1A1917`..`#302D29` dark; `#FCFAF7`..`#E9E4DC` light). | **PASS** |
| **R-03** | Touch Target & Kepadatan | Baris tabel padat `36px`; baris navigasi sidebar dan kontrol interaktif utama `44px` (`h-11`) di seluruh breakpoint. | **FIXED (F1)** |
| **R-04** | Set Ikon Tunggal | Satu set ikon garis (`lucide`), tanpa emoji di seluruh copy, heading, empty state, maupun tombol. | **PASS** |
| **R-06** | Tipografi | `Inter` (`14px` body, `12px-13px` label/secondary, `18px` screen title, `11px` uppercase `0.06em` hanya pada grup sidebar) + monospace sistem untuk ID, cost, token, dan log. | **PASS** |
| **R-09** | Disiplin Badge / Zero-Pill | Tidak ada pill dekoratif (`--radius-full` hanya untuk logo mark dan avatar). Status diwakili marker vertikal `3px` + teks status. | **PASS** |
| **R-10 / R-13** | Glass, Blur, Glow | Tidak ada `backdrop-blur` atau neon shadow berlapis. Satu-satunya glow adalah indikator node aktif pada gambar routing Usage saat request in-flight (`draft 013 F3`, `draft 015 F3`). | **PASS** |
| **R-11 / R-12** | Radius & Elevasi | `--radius-sm: 6px` (input/button/row), `--radius-md: 8px` (panel/card/table), `--radius-lg: 12px` (dialog/drawer). Shadow hanya pada dialog modal, mobile drawer, dan cincin logo. | **PASS** |
| **R-19** | Motion Discipline | `MOTION 1`. Animasi garis routing (`beam-dash`) hanya bergerak ketika ada frame aktif (`in-flight`) dan menghormati `prefers-reduced-motion`. | **FIXED (F9)** |
| **R-24** | Rute Belum Terbangun | Item navigasi tanpa layar aktif tidak boleh dirender sebagai link mati; seluruh 14 layar dalam 5 grup tugas kini memiliki permukaan fungsional penuh. | **PASS** |
| **R-25** | Kontras WCAG AA | Seluruh 13 pasangan warna `DESIGN.md §3.3` memenuhi floor `>= 4.5:1` (terendah `accent` di atas `surface-2` pada `4.87:1`, sehingga teks kecil di atas panel menggunakan `--color-text` dengan marker `--color-accent`). | **PASS** |
| **R-26** | Larangan Kontrol Mati | Tombol yang tidak dapat beraksi (mis. tutup modal one-time key sebelum copy berhasil, atau tulis ke `/models/disabled` sebelum bacaan pertama selesai) memberikan alasan eksplisit atau membuka kunci hanya sesudah aksi prasyarat sah. | **FIXED (F7, F8)** |
| **R-38** | Kejujuran Fakta Wire | Angka yang tidak dikirim upstream/gateway ditampilkan sebagai `not stated` (bukan `0`), dan `total: null` pada kuota ditampilkan sebagai `Unbounded (no vendor ceiling)` alih-alih `0%`. | **FIXED (F11, F12)** |

---

## 4. Temuan Audit UX & UI per Layar (`F1` - `F15`)

### 4.1 Shell, Navigasi, & Responsivitas (`DESIGN.md §8`, `SYSTEM_MAP.md §3.2`)

- **F1 - Perilaku Tiga Breakpoint Shell (`<768px` Drawer, `768-1023px` 64px Icon Rail, `>=1024px` 264px Sidebar) & Tinggi Baris 44px**:
  - *Gejala*: Implementasi awal hanya memiliki toggle manual `264px <-> 64px` tanpa off-canvas drawer ber-overlay pada viewport mobile (`<768px`), serta tinggi baris navigasi masih `40px` alih-alih `44px` (`DESIGN.md §7`).
  - *Keputusan & Perbaikan*:
    1. Baris navigasi sidebar ditetapkan `44px` (`h-[44px]`) dengan marker kiri `3px` (`--color-accent`) pada baris aktif.
    2. Pada `<768px`, sidebar berubah menjadi off-canvas drawer di atas dimmed overlay yang dibuka dari tombol header dan tertutup saat menekan `Escape`, klik overlay, atau pindah layar.
    3. Pada `768px-1023px`, shell menampilkan icon rail `64px` dengan ekspansi ke `264px`.
    4. Pada `>=1024px`, sidebar tampil penuh `264px` dan dapat di-collapse ke `64px`.

- **F2 - Guard Perubahan Belum Tersimpan (`dirty-guard`)**:
  - *Gejala*: Operator yang sedang mengubah form berbasis baseline server (Settings, Vision Adapter, Token Saver, Budget Cap, Model Aliases, Disabled Models) dapat berpindah menu sidebar tanpa peringatan kehilangan draft (`DESIGN.md §11`).
  - *Keputusan & Perbaikan*: Satu guard terpusat di shell memeriksa flag `isDirty` sebelum mengganti layar aktif dan menampilkan dialog konfirmasi jika terdapat draft yang belum di-commit.

- **F3 - Disiplin Lima Titik Aksen (`DESIGN.md §3.4`)**:
  - *Gejala*: Beberapa tombol sekunder atau badge berisiko memakai warna aksen.
  - *Keputusan & Perbaikan*: Warna `--color-accent` dikunci hanya pada (1) baris navigasi aktif (marker kiri 3px + ikon), (2) tepat satu CTA utama per layar, (3) `:focus-visible` ring, (4) tautan teks (`<a>`), dan (5) garis bawah tab aktif (`border-b-2 border-[var(--color-accent)]`).

---

### 4.2 Grup Configure (`Endpoint & Key`, `Provider`, `Combo & Vision Adapter`, `Gateway Keys`)

- **F4 - Operasi Massal (Bulk Status, Bulk Tags, Bulk Delete) & Jendela `disabled_until` (`SYSTEM_MAP.md` migrasi `000022-000023`, `draft 059`)**:
  - *Gejala*: `app-serv` telah menyediakan 6 rute mutasi massal (`POST /api/v1/endpoints/bulk-status`, `bulk-tags`, `bulk-delete`, serta `/endpoint-keys/bulk-*`) dengan selector tag (`tags @> '["x"]'`) dan kolom `disabled_until`, namun layar `Endpoint & Key` hanya mengizinkan toggle satu per satu.
  - *Keputusan & Perbaikan*: Ditambahkan bar seleksi & aksi massal pada layar `Endpoint & Key` (pilih baris / filter berdasarkan tag -> `Bulk Enable`, `Bulk Disable (5h window)`, `Bulk Tag`, `Bulk Delete`) serta tampilan `disabled_until` dan chip `tags` tanpa kotak pill berlebihan.

- **F5 - Transparansi Asal-Usul Parkir Kredensial (`reset_source`, `signature_id`, `last_selected_at`) (`SYSTEM_MAP.md §3.5b-§3.5c`, `draft 050`, `draft 052`)**:
  - *Gejala*: Tabel key di dalam endpoint hanya menampilkan `consecutive_auth / ratelimit / billing` dan `last_error_class`, tetapi belum menampilkan `reset_source` (`header` vs `body` vs `default`) dan `signature_id` (migrasi `000021`). Padahal operator membuka key yang diparkir untuk membedakan apakah waktu tunggu dijamin header/body vendor atau hasil taksiran ladder gateway.
  - *Keputusan & Perbaikan*: Sub-tabel key kini merender kolom **Rest Provenance** (`reset_source` + `signature_id` + `rate_limited_until`) berdampingan dengan `last_selected_at` (yang di-stamp saat seleksi `MarkSelected`, bukan saat sukses).

- **F6 - Permukaan Multi-Alur OAuth pada Detail Provider (`authorize_url`, PKCE Device Flow `qoder`, & Vendor State Round `codebuddy`) (`SYSTEM_MAP.md §6`, `DESIGN.md §11`)**:
  - *Gejala*: Modal/drawer detail provider hanya menampilkan daftar model tanpa menampilkan bagian OAuth bagi provider ber-flag `has_oauth`.
  - *Keputusan & Perbaikan*:
    1. Bagian OAuth hanya dirender bila `provider.has_oauth === true`.
    2. Tautan `authorize_url` dirender sebagai link eksplisit yang menampilkan host tujuan (panel tidak melakukan redirect otomatis).
    3. Mendukung tampilan status refresh akun (`refresh_state`, `oauth_retry_at`, `oauth_retry_rounds`) dan tombol manual refresh per akun (`POST /api/v1/providers/{id}/oauth/refresh`).

- **F7 - Pemisahan Daftar Model Aktif vs Disabled Set & Proteksi Tulis Sebelum Baca (`DESIGN.md §11`)**:
  - *Gejala*: Pada layar Provider/Katalog, model yang di-disable menghilang dari katalog gabungan, sehingga operator membutuhkan daftar kedua (`Disabled Models`) untuk mengaktifkannya kembali (`PUT /api/v1/models/disabled`).
  - *Keputusan & Perbaikan*: Ditambahkan panel **Disabled Models Set** berdampingan dengan **Global Model Aliases**, dengan aturan merge utuh di atas himpunan terakhir yang dibaca dari server.

---

### 4.3 Grup Observe (`Usage`, `Quota Tracker`, `Console Log`)

- **F9 - Gambar Jalur Request 5 Tahap (`Client >> Combo >> Gateway >> Upstream >> Response`) & Bar Kontrol Simetris (`draft 015`, `draft 035`, `draft 043`)**:
  - *Gejala*: Diagram topologi awal hanya menggambar 3 kotak (`Client -> Engine -> Provider`) dan animasi garis menyala terus-menerus meskipun stream sedang tidak menerima frame in-flight.
  - *Keputusan & Perbaikan*:
    1. Menggambar 5 tahap sesuai `draft 043 F1`: `Client` (kiri) -> busur atas `Combo` (`coding-flagship-ha`, maks 8 node dengan keterangan bila ada yang dipangkas) -> `pannelAI Gateway` (tengah, dengan penghitung in-flight) -> busur bawah `Upstream` (`claude`, `openrouter`, `opencode`) -> `Response` (kanan).
    2. Bar kontrol di atas kanvas menyatukan tab status koneksi (`Live Stream Connected` / `Paused`), tab fakta in-flight, dan tombol `Pause` / `Simulate In-Flight Frame` dalam tinggi kotak simetris (`draft 035 F1`).
    3. Animasi `beam-dash` hanya berjalan saat ada request in-flight atau saat simulasi/request Playground sedang berjalan (`DESIGN.md §11`: *"The drawing moves only while frames arrive"*).

- **F10 - Tabel Breakdown Multi-Kolom + Switch Grafik `Tokens / Cost` + Top 5 Models (`draft 014 F2/F6`, `draft 016 F1/F2`)**:
  - *Gejala*: Layar Usage belum menampilkan tabel breakdown per provider yang memuat seluruh kolom nilai sekaligus (`Requests`, `Tokens In`, `Tokens Out`, `Saved`, `Cost USD`) dan kartu peringkat 5 model teratas.
  - *Keputusan & Perbaikan*: Ditambahkan seksi **Provider Breakdown & Top 5 Models** dengan format angka kompak standar `Intl.NumberFormat('en-US', { notation: 'compact' })` dan kalimat eksplisit yang menyatakan jumlah model ekor yang tidak digambar.

- **F11 - Kejujuran Kuota Terbit (`total = NULL` vs `0`, `never_polled`, `truncated`) & Read-Only Input Sebelum Cap Terbaca (`SYSTEM_MAP.md §3.7`, `DESIGN.md §11`)**:
  - *Gejala*: Menyamakan `cap_cost_usd = null` atau `total = null` dengan `0` berisiko memberi kesan kuota habis, padahal `NULL` berarti tanpa batas (*unbounded*).
  - *Keputusan & Perbaikan*:
    1. Membedakan secara tegas tampilan **Gateway Counted Window** (`quota_windows`) dan **Vendor Published Quota** (`quota_published_window`: saldo kredit / `unlimited` / `never_polled`).
    2. Menampilkan kalimat eksplisit *"No cap stored (unbounded)"* ketika batas bernilai `null` atau `0` yang tidak disetel, serta peringatan dampak routing tepat di sebelah tombol `Commit Cap`.

---

### 4.4 Grup Optimize, Developer, & System (`Token Saver`, `Skill`, `Playground`, `API Docs`, `Changelog`, `Proxy Pools`, `Settings`)

- **F12 - Playground Chat: Tiga Kalimat Akhir Stream, Pengungkapan Frame Verbatim, & `not stated` (`DESIGN.md §11`)**:
  - *Gejala*: Layar Playground belum menyediakan blok *collapsible* untuk memeriksa frame mentah (*verbatim wire payload*) dan kalimat status terminasi stream (`Ended on [DONE] sentinel`, `Stopped by operator`, atau `Closed without [DONE] sentinel - response may be truncated`).
  - *Keputusan & Perbaikan*:
    1. Menambahkan baris kalimat terminasi eksplisit beserta marker kiri `3px`.
    2. Menampilkan `not stated` bila sebuah field usage tidak dilaporkan wire.
    3. Menambahkan blok `<details>` **Verbatim Wire Frames & Headers** untuk diagnosis deviasi kontrak secara langsung.
    4. Meletakkan kalimat peringatan biaya (*"Routes to a real upstream provider and writes a usage record"*) tepat di samping tombol kirim (`SPEC-UI §6.15 rule 4`).

- **F13 - Pemeriksaan Keterjangkauan Sumber pada Layar Skill (`DESIGN.md §11`)**:
  - *Gejala*: `DESIGN.md §11` menetapkan bahwa setiap alamat `raw_url` pada katalog Skill memiliki status keterjangkauan yang jelas dan ringkasan di bagian atas menghitung baris yang tersedia (`7 of 7 published skills available`).
  - *Keputusan & Perbaikan*: Menampilkan ringkasan ketersediaan sumber (`7 of 7 published skill documents verified`) serta memisahkan tombol salin alamat **Agent Raw (`SKILL.md`)** dari tautan bacaan manusia **GitHub Blob**.

- **F14 - Proxy Pools: Pratinjau Batch (`Bulk Preview`) & Pengikatan Strategi Proxy per Provider (`docs/PORT/009-PORT-PROVIDER-PROXY.md`)**:
  - *Gejala*: Operator sering menambahkan daftar proxy lewat tempelan teks multi-baris (`protocol://user:pass@host:port`) dan perlu melihat hasil parse sebelum satu pun request tulis dikirim (`DESIGN.md §11`: *"Batch add shows the parse before any write"*).
  - *Keputusan & Perbaikan*: Ditambahkan fitur **Batch Parse Preview** pada layar `Proxy Pools` yang mem-parsing daftar baris proxy secara lokal/server dan menampilkan tabel pratinjau validasi sebelum tombol `Commit Batch` ditekan.

- **F15 - Kepatuhan Batas 250 Baris per Berkas (`AGENTS.md §1.1`)**:
  - *Gejala*: Menggabungkan seluruh 14 layar ke dalam satu berkas `src/App.tsx` membuat berkas tersebut melampaui batas nyaman pemeliharaan.
  - *Keputusan & Perbaikan*: Memecah `src/` menjadi modul-modul layar bertipe ketat di bawah `src/components/` dan `src/screens/` sehingga setiap berkas sumber berada di bawah plafon `250` baris.

---

## 5. Rencana Verifikasi & Penutupan Gate

1. **Verifikasi Runtime Bun 1.3+**: Jalankan `bun install`, `bun run lint` (`tsc --noEmit`), dan `bun run build` (`vite build`).
2. **Verifikasi Kontras & Token (`DESIGN.md §3.2-§3.3`)**: Pastikan mode `dark` dan `light` menggunakan seluruh 11 token permukaan/teks/status tanpa inline style atau gradien.
3. **Verifikasi Batas Baris (`AGENTS.md §1.1`)**: Pastikan seluruh berkas `.ts` / `.tsx` di `src/` dan `server.ts` terstruktur rapi dan lulus kompilasi produksi.
