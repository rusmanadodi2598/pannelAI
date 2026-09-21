# 006-TOKEN-SAVER-READINESS.md: Audit Kesiapan Vertical Token Saver app-serv

Dokumen kerja hasil audit ulang vertical Token Saver pada 2026-09-20. Dokumen ini
melanjutkan pola `003-ENDPOINT-READINESS.md`, `004-ENDPOINT-READINESS-FOLLOW-UP.md`,
dan `005-QUOTA-TRACKER-CLOSURE.md`. Bukan kontrak; kontrak tetap
`docs/SPEC-API/001-SPEC-API.md` dan `docs/SPEC-API/002-TOKEN-SAVER.md`.

| | |
|---|---|
| **Status** | F1 **CLOSED 2026-09-20** (direct orchestrator and translator tests; no production behavior change) |
| **Dibuat** | 2026-09-20, dari vertical `app-serv/internal/tokensaver/`, `internal/dataplane/`, `internal/handler/`, `internal/schema/`, `internal/service/`, dan router Token Saver |
| **Kaitan** | SPEC-API-001 §4, §7.9, §7.15, §8, §9; SPEC-API-002 §4, §5, §6, §7, §8, §9; AGENTS.md §1.1, §1.2, §1.4, §2.1, §2.5; `docs/RULLES/TDD.md`; `docs/RULLES/OWASP.md` |
| **Lingkup** | hanya `app-serv/`. `app-ui/` di luar lingkup dokumen ini dan tidak boleh disentuh |

## 1. Keputusan scope

**CAVEMAN = DEPRECATED.**

Yang sudah ada dibiarkan. Caveman bukan work item dalam draft ini.

## 2. Metode

Audit dilakukan per lapisan dan per perjalanan request:

1. Membandingkan kontrak Token Saver dengan schema, domain, service, handler, router,
   applier, translator, dan seam relay.
2. Menelusuri bypass `X-Token-Saver: off` dari HTTP header sampai `Engine.Relay`.
3. Memeriksa batas keamanan Headroom: timeout, fail-open, batas respons, pembentukan
   endpoint, dan penggunaan HTTP client yang sudah dijaga egress.
4. Memetakan test yang sudah ada ke aturan TDD/AGENTS, lalu menutup gap dengan
   test table-driven tanpa mengubah kode produksi.
5. Menjaga scope hanya pada `app-serv/`; tidak ada perubahan pada `app-ui/**`.

## 3. Bukti yang sudah lolos

Hal-hal berikut sudah ada dan dibiarkan sebagaimana adanya:

- `GET` dan `PUT /api/v1/token-saver` terdaftar di router dan session-gated.
- Schema memakai `go-playground/validator/v10`, `DisallowUnknownFields()`, dan
  validasi URL `http`/`https` serta aturan lintas-field Headroom.
- Default wire document mengikuti SPEC-API-002 §6.1: `rtk`, `headroom` dengan URL
  kosong, dan `ponytail` level `full`; seluruh saver default off.
- Cap RTK dan detection window sesuai SPEC-API-002 §5.1.
- Headroom memakai timeout `5s`, fail-open, dan batas respons `16 MiB` melalui
  `io.LimitReader` serta pemeriksaan panjang setelah pembacaan.
- Endpoint Headroom menambahkan `/v1/compress`, mempertahankan base path/query,
  membuang fragment, dan label error menghapus userinfo/query/fragment.
- `NewApplierWithClient` menerima HTTP client yang sama dengan client yang sudah
  dikomposisikan melalui egress guard.
- `X-Token-Saver: off` diparsing secara trim + case-insensitive dan diteruskan ke
  `dataplane.Request`.
- Relay menerapkan saver setelah target/wire/model ter-resolve dan sebelum upstream,
  untuk jalur streamed maupun non-streamed. Test seam juga sudah mencakup bypass,
  pass-through, dan setiap member combo.
- Settings write path memvalidasi dokumen gabungan sebelum persist; error validasi
  menggunakan envelope `VALIDATION_ERROR`.
- Test handler, service, schema, router, Headroom, RTK, Ponytail, dan relay yang
  sudah ada tidak perlu ditulis ulang.

## 4. F1 (MEDIUM): Orchestrator Applier dan translator Headroom belum punya test langsung

**Status: CLOSED 2026-09-20.** Test langsung sudah ditambahkan tanpa perubahan
pada kode produksi:

- `internal/tokensaver/applier_test.go` menguji bypass, body kosong, settings
  fail-open, group disabled, URL Headroom kosong, translator refusal, proxy error,
  restore error, dan urutan RTK → Headroom → Ponytail.
- `internal/tokensaver/applier_fixtures_test.go` memisahkan fixture settings,
  translator, dan proxy agar batas 250 baris AGENTS.md tetap dipatuhi.
- `internal/dataplane/token_saver_translate_prepare_test.go` menguji Prepare
  untuk OpenAI, Claude, Responses, refusal reasoning/function output, malformed
  body, dan wire tidak dikenal.
- `internal/dataplane/token_saver_translate_restore_test.go` menguji Restore
  untuk ketiga wire, preservasi sibling member, malformed output, dan unknown wire.
- `internal/dataplane/token_saver_translate_test.go` berisi helper typed untuk
  membaca hasil JSON test.

Kriteria F1 terpenuhi. Test tetap table-driven, seluruh file baru memiliki header,
dan tidak ada perubahan urutan pipeline atau perilaku endpoint.

**Catatan scope:** Caveman tidak disentuh. CAVEMAN = DEPRECATED.

**Fakta audit awal.** `internal/tokensaver/applier.go` menjalankan keputusan utama pipeline:
settings dibaca per request, RTK diterapkan lebih dahulu, Headroom berikutnya,
Ponytail terakhir, dan seluruh kegagalan optional dikembalikan sebagai pass-through.

`internal/dataplane/token_saver_translate.go` dan
`internal/dataplane/token_saver_translate_json.go` menangani perjalanan `Prepare`/`Restore`
pada wire OpenAI, Claude, dan Responses. Test relay yang sudah ada membuktikan seam
placement dan bypass; test baru mengunci transformasi pivot Headroom di tiap wire.

## 5. Validasi setelah implementasi

Dari `app-serv/`, jalankan seluruh gate, bukan hanya test yang baru:

```bash
go test -race -count=1 ./...
go vet ./...
gofmt -l .
scrypts/gates/go-lint.sh
```

Jika tersedia di environment, jalankan juga `staticcheck ./...` dan tagged gate yang
berlaku. Verifikasi tambahan:

- `git diff --check` bersih;
- tidak ada file `app-ui/**` yang ikut berubah atau ter-stage;
- `go test` khusus paket `internal/tokensaver`, `internal/dataplane`, `internal/handler`,
  `internal/schema`, dan `internal/service` hijau;
- tidak ada test yang memakai `t.Skip()`, isolated `-run`, atau mengabaikan error;
- perubahan tidak memerlukan update `SYSTEM_MAP.md`: tidak ada perubahan domain
  boundary, struktur data, interaksi service, atau topology async. Jika implementasi
  ternyata mengubah salah satunya, update wajib dibuat dalam perubahan yang sama.

## 6. Status akhir dan verification evidence

F1 sudah CLOSED. Tidak ada gap implementasi produksi yang tersisa dari draft ini.
CAVEMAN = DEPRECATED, dan yang sudah ada dibiarkan.

Verification yang dijalankan dari `app-serv/`:

- `go test -race -count=1 ./...` PASS.
- `go vet ./...` PASS.
- `gofmt -l .` PASS.
- `scrypts/gates/go-lint.sh` PASS, termasuk untagged/tagged vet, gofmt,
  staticcheck, dan golangci-lint.
- `scrypts/gates/go-headers.sh` PASS: 631 file Go memiliki header lengkap.
- Seluruh file test baru berada di bawah 250 baris dan tidak memakai `t.Skip()`.

Perubahan hanya menambah test di `app-serv/` dan memperbarui draft ini. Tidak ada
perubahan pada `app-ui/**`, `SYSTEM_MAP.md`, spec, atau kode produksi.

## 7. Urutan pengerjaan

F1 sudah selesai; tidak ada urutan implementasi lanjutan dari draft ini.
