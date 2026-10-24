# 063-APP-SERV-STABILITY-AUDIT.md: koreksi SSRF, worker OAuth yang memblokir, dan debt 250 baris

Dokumen ini adalah **rencana kerja**, bukan catatan yang sudah selesai. Temuan di bawah sudah diverifikasi di kode dan suite `-race`; status tiap butir diperbarui saat perbaikannya mendarat.

| | |
| --- | --- |
| **Status** | Audit selesai. Seluruh HIGH, MEDIUM terpilih (M1-M12), dan LOW (L1-L7) selesai. M2 memakai pengecualian owner untuk OpenCode, Qoder, dan CodeBuddy; M12 menandai stream terpotong tanpa memarkir endpoint. HSTS diputuskan milik terminator TLS, bukan app-serv. Lima file test di atas 250 baris sudah dipecah, dan gate pre-push kini meneruskan daftar file yang di-push. `SYSTEM_MAP.md` §5 dan §6 diperbarui. |
| **Sumber** | `app-serv/internal/**`, `app-serv/cmd/app-serv/**`, `docs/RULLES/SSRF.md`, suite `go test -race`, tiga pass audit paralel. |
| **Lingkup** | Backend `app-serv`: validasi, keamanan, sanitasi, race, lifecycle worker, dan debt ukuran file. |
| **Tidak termasuk** | Panel `app-ui`, deployment multi-replika, dan perubahan bentuk wire; batas 100 grup pada ringkasan usage hanya ditambahkan sebagai deskripsi kontrak, skemanya tidak berubah. |

## 1. Koreksi penting: SSRF lewat proxy pool

Klaim awal: `dataplane/proxy_route.go:151` mengganti `clone.Proxy` sehingga `egress_wiring.go:88` tidak pernah memvalidasi target.

Hasil verifikasi: **tidak berlaku di wiring produksi.** `cmd/app-serv/proxy_wiring.go:76-87` mengembalikan `guardedProxyPlanner`; `Plan` memanggil `guard.CheckHost(ctx, host)` sebelum mengembalikan plan yang tidak kosong. Planner itulah yang dipakai chat dan media (`management_wiring.go:173`, `dataplane_media_wiring.go:28`).

Yang tersisa adalah fragility dan satu bug kecil:

- `ProxyDialer` sendiri tidak memegang guard. Planner lain yang lupa memanggil `CheckHost`, termasuk fake di test, akan membuka jalur yang sama. Pertahanan sebaiknya dipasang di dialer atau dijaga test kontrak planner.
- `guardedProxyPlanner.Plan` menerima `request.URL.Host` dari `ProxyDialer.Do`, yang bisa berisi `host:port`. `netguard.CheckHost` tidak memisahkan port, jadi target dengan port eksplisit ditolak. Perbaikan: pakai `Hostname()` atau `net.SplitHostPort` sebelum `CheckHost`.

Status: MEDIUM, bukan HIGH. Tidak ada perubahan produksi mendesak.

## 2. Temuan HIGH

| ID | Temuan | Bukti | Perbaikan | Status |
| --- | --- | --- | --- | --- |
| H1 | Worker OAuth tidur inline dan memblokir seluruh sweep | `service/oauth_refresh_worker.go:175`, loop serial `:96-117`, backoff sampai 30m `:31-33`; test mengganti `sleep` dengan no-op `oauth_refresh_worker_test.go:67` | Jadwalkan percobaan berikutnya di map `next`, jangan tidur di dalam loop; timer `Run` bangun pada waktu terdekat | Selesai: `oauth_refresh_schedule.go`, timer `Run`, test backoff dan recovery hijau |
| H2 | Tulis penuh endpoint bisa mengembalikan token OAuth yang sudah dirotasi | `service/endpoint.go:157,182`, `endpoint_probe_use.go:32,101`, `endpoint_keys.go:142`, `oauth_flow_connect.go:60`, `oauth_flow_refresh.go:196-203`; SQL `repository/postgres/endpoint_oauth_batch.go:84-106` | Keluarkan `oauth`, `account`, dan `consecutive_use_count` dari update penuh; connect memakai `UpdateIfUnchanged` yang diperluas ke `account` | Selesai: update penuh tanpa kolom kredensial/counter, connect CAS, test integrasi ditambah |
| H3 | Purge retensi log satu `DELETE` tak terbatas dengan timeout 10 detik | `repository/postgres/log.go:137`, `service/log_retention.go:44` | Hapus bertahap `ctid IN (SELECT ... LIMIT n)` dan lanjutkan sampai habis atau waktu mati | Selesai: batch 5000, progres parsial, test integrasi ditambah; kebijakan owner 2026-10-22: tick 2 hari, retensi default 2 hari, cap `REQUEST_LOG_MAX_ROWS` 20000 lewat `TrimTo` |
| H4 | Fold forced-stream bisa menahan sampai sekitar 64 GiB | `dataplane/engine_forced_stream.go:28,80`, `maxEventBytes` 1 MiB di `engine_stream.go:29` | Tambah batas total byte, bukan hanya jumlah event | Selesai: `maxFoldBytes`, test cap |
| H5 | Peek envelope Qoder berjalan sebelum guard first-byte | `provider/qoder_envelope.go:39`; guard baru di `dataplane/transport_attempt.go:106-109`; context stream tanpa deadline `:40-42` | Jalankan peek di bawah timer yang membatalkan attempt, atau pasang guard yang mencakup peek | Selesai: `openEnvelopeWithin`, test keepalive |

Catatan retensi: default 2 hari hanya berlaku pada dokumen settings yang belum menyimpan `logging.retention_days`; deployment yang sudah menyimpan 7 tetap memakai 7 sampai nilainya di-PATCH. Cap `REQUEST_LOG_MAX_ROWS` dan `USAGE_RETENTION_DAYS` adalah env, bukan field kontrak.

## 3. Temuan MEDIUM

| ID | Temuan | Bukti | Perbaikan | Status |
| --- | --- | --- | --- | --- |
| M1 | Guard SSRF bergantung pada planner; port eksplisit ditolak | `cmd/app-serv/proxy_wiring.go:76-87`, `dataplane/proxy_route.go:62,151` | Pasang guard di `ProxyDialer` atau uji kontrak planner; pisahkan port sebelum `CheckHost` | Selesai: port dipisah, test planner ditambah |
| M2 | Kegagalan provider quota dibaca sebagai jawaban lunak | `quotafetch/codebuddy.go:78-87`, `claude.go:56-59`, `quota_published_worker.go:185-187` | Tandai 5xx/transport sebagai `Failed`, biarkan backoff bekerja | Selesai dengan pengecualian owner: transport dan 5xx menjadi `Failed` untuk semua family; 429 juga menjadi `Failed` kecuali Claude (punya cooldown sendiri) dan tiga family exempt. OpenCode (`opencode-zen/go`), Qoder (`qoder/qoder-cn`), dan CodeBuddy (`codebuddy-cn/intl`) mempertahankan jawaban lunak karena patch reference-nya sudah bekerja. 401/403/404 tetap soft. Test `failure_policy_test.go` |
| M3 | `X-Forwarded-For` multi-baris melewati rate limit dan login lockout | `router/limiter.go:41`, `handler/auth.go:119`, `clientip/clientip.go:83` | Gabungkan `Header.Values("X-Forwarded-For")` | Selesai: limiter dan login memakai seluruh baris, test ditambah |
| M4 | Request ID klien bisa menekan baris audit | `router/middleware.go:51-56`, `repository/postgres/log.go:71` | ID baris audit dari server, atau `ON CONFLICT DO UPDATE`; trace ID klien dipisah | Selesai: migrasi `000024`, `id` ULID sebagai PK, `request_id` terindeks non-unik, detail menjawab baris terbaru |
| M5 | Akun OAuth tanpa expiry tidak pernah dikunjungi ulang | `domain/oauth_refresh.go:39-41`, `oauth_refresh_worker.go:97`, `oauth_flow_connect.go:147-149` | Revisit akun `error` tanpa menunggu expiry | Selesai: akun `error` memakai jendela ladder, bukan `RefreshDue`; test tanpa expiry ditambah |
| M6 | Worker multi-replika tanpa lease; upsert kuota bisa mundur sementara | `quota_published_worker.go:113-127`, `oauth_refresh_worker.go:42`, `repository/postgres/quota.go:99` | Lease/claim atau nyatakan single-replica; `GREATEST` pada `used_units` bila perlu | Selesai: lease Redis per siklus untuk OAuth, quota flush, published quota; upsert menjaga high-water `used_units` untuk source computed dalam window yang sama; retensi log/usage tidak di-lease karena idempotent |
| M7 | `usage_records` tidak dipurge dan tidak dipartisi | `migrations/000007_usage_quota.up.sql:6`; SPEC-API §6 `docs/SPEC-API/001-SPEC-API.md:135` | Partisi bulanan dan job drop/retensi, atau catat deviasi | Selesai sebagian: worker retensi 24 jam, default 90 hari, `USAGE_RETENTION_DAYS`, DELETE bertahap; partisi bulanan belum, deviasi dicatat di SYSTEM_MAP |
| M8 | Klien Redis memakai default library | `cmd/app-serv/main.go:66-69` | Tambah config pool/timeout Redis dan validasi | Selesai: `REDIS_POOL_SIZE` dan tiga timeout, default serta batas, `.env.example` |
| M9 | Body request tidak sepenuhnya ketat | `schema/dto.go:116-120`, `handler/endpoint_body.go:91` | `MaxBytesReader`, decode kedua `io.EOF`, `DisallowUnknownFields`, 415 untuk non-JSON | Selesai: trailing data, field tak dikenal, content-type; body besar menjawab 400 |
| M10 | URL proxy global dikembalikan apa adanya tanpa `no-store` | `schema/settings_response.go:40`, `router/middleware.go:37-39` | Mask password atau `has_password`; `Cache-Control: no-store` | Selesai: password dimask `****`, PATCH yang mengirim mask mempertahankan password tersimpan, `no-store` global |
| M11 | Identitas Qoder dibaca tanpa timeout | `provider/qoder_identity.go:143-152`, dipanggil `provider/qoder.go:105` | `context.WithTimeout` seperti `JobToken`/vertex | Selesai: timeout 15 detik |
| M12 | EOF bersih di tengah jawaban dicatat sukses | `dataplane/engine_stream.go:138`, `translate_stream_openai.go:144-153`, `engine_outcome.go:27-35` | Tandai `Truncated` bila tidak ada finish/terminal event | Selesai: `Outcome.StreamTruncated` diisi dari terminal marker per format (`[DONE]`, finish frame, `message_stop`, finish reason); frame finish dan `[DONE]` sintetis tetap dikirim agar client lepas; health tidak mencatat sukses dan hanya menulis warning, tidak memarkir endpoint; test stream |

## 4. Temuan LOW

| ID | Temuan | Bukti | Perbaikan | Status |
| --- | --- | --- | --- | --- |
| L1 | OAuth callback merefleksikan teks provider sebelum state divalidasi | `service/oauth_flow_callback.go:103-108`, `handler/oauth_callback.go:110` | Validasi state lebih dulu atau kalimat tetap; batasi panjang | Selesai: state dikonsumsi dulu, teks provider dibatasi 200 rune |
| L2 | Filter/path belum divalidasi ketat | `handler/endpoint.go:38`, `handler/media_provider.go:140`, `handler/media_video.go:69-80` | Query struct bertipe dan validator | Selesai: `schema.EndpointListQuery` menutup status ke `active/disabled/error` dan membatasi provider id 64; `schema.MediaVideoPollQuery` mewajibkan provider serta membatasi endpoint 64 dan task id 256 sebelum URL dibangun; kind media sudah ditolak `ParseMediaKind`. Test handler ditambah |
| L3 | Tidak ada header keamanan respons | `router/middleware.go:37-39` | `X-Content-Type-Options`, HSTS, `no-store` untuk rahasia | Selesai: nosniff dan no-store terpasang; HSTS diputuskan milik terminator TLS karena app-serv melayani HTTP polos, dicatat di `securityHeaders` |
| L4 | Ringkasan usage tanpa LIMIT | `repository/postgres/usage.go:139-143` | Top-N atau LIMIT | Selesai: `domain.MaxUsageGroupRows = 100`, urut jumlah request lalu kunci, `LIMIT` di statement; test integrasi 105 grup; batas didokumentasikan di SPEC-API §7.12 dan deskripsi operasi kontrak, artifact OpenAPI diregenerasi |
| L5 | Cache transport proxy tanpa eviction | `dataplane/proxy_route.go:47` | Batasi entri atau buang saat kandidat berubah | Selesai: map + urutan FIFO, plafon 32, koneksi idle ditutup saat evict, test reuse/eviction dan race |
| L6 | `ReadTimeout` 30 detik vs unggahan 25 MB | `cmd/app-serv/main.go:132`, `schema/media_transcription.go:27` | Pisahkan timeout header dan body unggahan | Selesai: `http.ResponseController` memperpanjang baca 5 menit dan tulis 2 menit di atasnya khusus rute transkripsi; test server `ReadTimeout` 200ms dengan unggahan bertahap |
| L7 | Media memotong body 32 MiB tanpa error | `dataplane/media.go:108` | Pakai `readBounded` yang memeriksa `limit+1` | Selesai: body berlebih menjadi error, bukan 200 terpotong |

## 5. Debt ukuran file

| File lama | Baris | Hasil split |
| --- | ---: | --- |
| `internal/service/media_video_test.go` | 295 | 139 `media_video_test.go` (create) + 175 `media_video_poll_test.go` |
| `internal/provider/failure_signature_test.go` | 284 | 176 `failure_signature_test.go` (recognition) + 127 `failure_signature_reset_test.go` |
| `internal/provider/opencode_fingerprint_test.go` | 257 | 160 `opencode_fingerprint_test.go` (conceal) + 117 `opencode_fingerprint_definitions_test.go` |
| `internal/dataplane/transport_clamp_test.go` | 253 | 161 `transport_clamp_test.go` (clamp) + 116 `transport_walk_deadline_test.go` |
| `internal/provider/opencode_auth_test.go` | 251 | 134 `opencode_auth_test.go` (headers) + 134 `opencode_session_test.go` |

Tidak ada file Go yang melewati 250 setelah split. Yang tersisa di band peringatan antara lain: `quotafetch/zed.go` 249, `schema/settings_routing_patch.go` 249, `quotafetch/minimax.go` 248, `service/quota_published_worker_test.go` 248, `quotafetch/glm_test.go` 248, `router/router.go` 248, `quotafetch/opencode_test.go` 246, `quotafetch/github_test.go` 246, `quotafetch/claude_org.go` 242, `dataplane/translate_stream_claude.go` 229, dan `quotafetch/commandcode.go` 226. Split berikutnya harus terjadi sebelum menambah logika ke file-file itu.

Gate `check_changed_line_limits` hanya melihat `change_set`; hook pre-push dulu tidak mengekspor base, jadi setelah commit diff kosong dan gate memeriksa nol file. Perbaikan: hook sekarang mengekspor `GATES_CHANGE_FILES` berisi path yang benar-benar di-push, dan `scrypts/lib/common.sh` memakainya lebih dulu daripada `GATES_BASE_REF`. Yang tersisa adalah tindakan operasional di GitHub: aktifkan required check pada `main` supaya gate CI tidak bisa dilewati merge.

## 6. Urutan kerja

- [x] H1: worker OAuth tanpa sleep inline
- [x] H2: pisahkan tulis kredensial dari tulis penuh endpoint
- [x] H3: purge retensi log bertahap
- [x] H4: batas byte pada fold forced-stream
- [x] H5: batas waktu pada peek envelope Qoder
- [x] M1, M3, M4, M5, M6, M7, M8, M9, M10, M11
- [x] M2: 5xx/429/transport menjadi `Failed`, dengan pengecualian OpenCode, Qoder, dan CodeBuddy
- [x] M12: stream tanpa terminal event ditandai `StreamTruncated` dan tidak mencatat sukses
- [x] L1-L7 (L3: nosniff dan no-store terpasang; HSTS diputuskan milik terminator TLS)
- [x] Debt: lima file test dipecah, gate pre-push meneruskan `GATES_CHANGE_FILES`; lima file produksi 242-249 tetap di bawah batas dan perlu split sebelum logika baru ditambahkan; required check di GitHub tetap tindakan operasional

## 7. Gerbang verifikasi

Setiap butir diverifikasi dengan test yang gagal sebelum perbaikan dan hijau sesudahnya, lalu:

```bash
cd app-serv && go test -race -count=1 ./...
scrypts/gates/go-lint.sh
scrypts/gates/go-headers.sh
scrypts/gates/antislop.sh
scrypts/gates/all.sh
```

Butir yang mengubah perilaku HTTP juga menambah test di paket `handler` atau `router` sesuai AGENTS.md §2.1.

Hasil di mesin ini: `go test -race ./...` hijau setelah M2/M12 (test kebijakan quota, stream tanpa terminal event, terminal per format, dan health truncation); go-lint (vet, tagged vet, gofmt, staticcheck, golangci-lint 0 issue), go-headers 1491 file, antislop, contract-drift, contract-openapi, gate-scope, dan secrets PASS. `all.sh` dihentikan saat vitest panel karena 270 file test berjalan lambat di mesin ini; prettier, eslint, svelte-kit sync, svelte-check, dan `tests/contract/panel-schemas.test.ts` sudah PASS sebelum dihentikan. Integration test repository hanya dikompilasi (`go vet -tags=integration,live`) karena `PANNELAI_TEST_POSTGRES_DSN` tidak di-set lokal; CI yang menjalankannya.
