# 048-ANTISLOP-APP-UI-AUDIT.md: Target sentuh modal, diagnostik error, dan layout tabel mobile

Audit AFTER atas `app-ui` pada 2026-10-10, HEAD `e024f515eccbe90c328a067b3e74a244351a11fb`. Pada snapshot audit awal, dokumen ini mencatat temuan dan arah tindak lanjut saja; saat itu tidak ada kode, aset, atau konfigurasi produk yang diubah.

## Status dan lingkup

|                                    |                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| ---------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**                         | **F1 RESOLVED; F2/F3 RESOLVED; F4 tetap PARTIALLY VERIFIED/[UNVERIFIED] dan belum sign-off; F5 terintegrasi dengan residual LOW yang diterima.** Bukti visual F4 hanya 1/16 tabel (Endpoint & Key) pada 375/768 dengan satu baris sintetis dan mock-auth; 15/16 lainnya hanya diperiksa statis; screen reader/AT serta sentuh perangkat nyata belum diverifikasi. Tidak ada klaim F4 selesai, zero findings, atau GHAS lulus. |
| **Mechanism**                      | AFTER (audit)                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| **Scope**                          | `app-ui/src/**`, `app-ui/tests/**`, `DESIGN.md`, serta `docs/SPEC-UI/001-SPEC-UI.md` dan `docs/SPEC-API/001-SPEC-API.md` sebagai kontrak UI/API                                                                                                                                                                                                                                                                                                                         |
| **Sumber temuan**                  | Pembacaan statis dan pencocokan kode, test, desain, dan spesifikasi; Anti Slop R-03/R-31 serta kriteria fungsional C-2/C-4                                                                                                                                                                                                                                                                                                                                              |
| **Verifikasi pada snapshot audit** | Statis pada HEAD di atas. `bun` dan `app-ui/node_modules` tidak tersedia, jadi test, type-check, build, dan click-through browser belum dijalankan pada snapshot tersebut. Ini bukan klaim lulus R-35.                                                                                                                                                                                                                                                                  |
| **Perubahan saat audit awal**      | Hanya dokumen DRAFT ini; tidak ada perubahan pada kode produk pada snapshot audit awal. Tindak lanjut F1 dicatat di bagian bawah.                                                                                                                                                                                                                                                                                                                                       |
| **Rangkaian PR / Issue**           | PR [#10](https://github.com/rusmanadodi2598/pannelAI/pull/10) merged ke `feat/app-ui-fase-2` pada `1ee6f22c926d4fb38d69e9b0bdb1b832378a7453`; PR [#8](https://github.com/rusmanadodi2598/pannelAI/pull/8) merged ke `development` pada `78607f40194629de0e57861a7126a33b02f2f6bd`; PR [#11](https://github.com/rusmanadodi2598/pannelAI/pull/11) (test-only) merged ke `development` pada `056b4f8e5f7f20a545820350381a3c37bc3b3aca`; PR [#7](https://github.com/rusmanadodi2598/pannelAI/pull/7) merged dari `development` ke `main` pada `1f7427629eef05dcea46e6dbc3331e4506bc69fe`. Issue [#9](https://github.com/rusmanadodi2598/pannelAI/issues/9) CLOSED dengan komentar resolusi owner. |
| **GHAS pada PR #7**                | Check-run `114392414165` dalam run `38113051717`, pada head `056b4f8e5f7f20a545820350381a3c37bc3b3aca`, berstatus **FAILURE**: log menunjukkan HTTP 402 karena kuota bulanan layanan AI review terlampaui. Owner menerima kegagalan kuota sebagai unavailable untuk merge, bukan sebagai PASS; enam check-run lain pada head itu sukses. Penerimaan status GHAS ini tidak menyatakan sign-off visual F4. Tidak ada kesimpulan security-clean. |

## Temuan

### F1 (HIGH) Tombol penutup dialog hanya 36px pada layout sentuh

**Aturan:** R-03 (Hard Gate); `docs/SPEC-UI/001-SPEC-UI.md` §8.7 butir 3 menetapkan target sentuh sekurangnya 44px.

`app-ui/src/lib/components/Modal.svelte:63-70` memberi tombol `Close dialog` kelas `size-9`, yaitu area 36 × 36px. Komponen modal ini digunakan bersama oleh berbagai layar, sehingga ukuran itu tetap berlaku saat dipakai pada layar sentuh. `app-ui/tests/components/combo-dialog-icons.test.ts:114-134` bahkan mengunci `size-9` dan menyebutnya ukuran khusus tombol dialog di `DESIGN.md` §7. Namun §7 yang diperiksa menetapkan tinggi baris tabel padat 36px dan tinggi navigasi 44px, bukan ukuran tombol dialog. Test tersebut tidak membuktikan pengecualian dari minimum target sentuh pada §8.7.

**Dampak:** Tombol dapat dioperasikan, tetapi area tap lebih kecil dari ukuran minimum yang diwajibkan pada perangkat sentuh.

**Arah tindak lanjut pada saat audit:** Pastikan area interaksi minimal 44 × 44px pada layout sentuh dan selaraskan test dengan kontrak itu. Bila 36px tetap dipilih untuk pointer desktop, gunakan ukuran responsif serta uji kedua layout.

### F2 (MEDIUM) INTERNAL_ERROR kehilangan request ID yang diperlukan operator

**Aturan:** `docs/SPEC-UI/001-SPEC-UI.md` §8.2 mewajibkan banner `INTERNAL_ERROR` menampilkan `request_id` dan kontrol salin; berkaitan dengan C-2 dan C-4.

`app-ui/src/lib/api/errors.ts:49-63` membaca header `x-request-id` dan menyimpan nilainya pada `ApiError.requestId`. Namun `app-ui/src/lib/components/StateMessage.svelte:9-14` hanya menerima `kind`, `title`, `description`, dan `action`; contoh pemakaian di `app-ui/src/routes/providers/+page.svelte:57-70` hanya meneruskan `result.error.message`. Tidak ditemukan rendering atau kontrol salin untuk `requestId` di UI.

**Dampak:** Saat gateway mengembalikan `INTERNAL_ERROR`, operator tidak memperoleh ID korelasi yang menurut spesifikasi perlu diberikan untuk pencarian log.

**Arah tindak lanjut, belum diterapkan (historis: snapshot audit awal):** Tambahkan tampilan dan kontrol salin request ID pada penanganan `INTERNAL_ERROR`, lalu uji respons yang memuat dan tidak memuat header tersebut.

### F3 (LOW) Provider search menerima istilah yang server pasti tolak

**Aturan:** `docs/SPEC-API/001-SPEC-API.md` §7.4 membatasi `q` sampai 120 karakter; `docs/SPEC-UI/001-SPEC-UI.md` §7 mensyaratkan validasi/sanitasi input di batas panel.

`app-ui/src/routes/providers/+page.svelte:127-147` menyediakan kolom pencarian tanpa batas panjang dan `applySearch()` (`:97-103`) hanya melakukan `trim()`. `app-ui/src/lib/schemas/provider.ts:142-146` memang mendefinisikan `schemaProviderQuery.q` dengan `.max(120)`, tetapi `app-ui/src/lib/api/providers.ts:52-58` meneruskan query langsung ke `apiRequest` tanpa memvalidasi dengan skema itu. Test server `app-serv/internal/handler/provider_search_test.go:77-107` membedakan 120 karakter yang diterima dari 121 karakter yang ditolak.

**Dampak:** Istilah lebih dari 120 karakter menghasilkan penolakan API yang tampil sebagai error halaman, bukan umpan balik validasi pada kolom pencarian.

**Arah tindak lanjut, belum diterapkan (historis: snapshot audit awal):** Terapkan skema query sebelum request dan tampilkan pesan dekat kolom pencarian; uji batas 120 dan 121 karakter.

### F4 (LOW) Scroll horizontal tabel mobile menjadi pola umum tanpa alasan per layar

**Aturan:** R-31 (Quality Lock); `DESIGN.md` §8 dan `docs/SPEC-UI/001-SPEC-UI.md` §8.7 butir 2 meminta tabel ditata menjadi baris key-value di bawah 768px, kecuali perbandingan berdampingan memang esensial.

Pencarian statis menemukan 16 komponen di `app-ui/src` yang menggabungkan `overflow-x-auto` dengan lebar minimum tetap, tanpa cabang representasi mobile bertumpuk yang tampak pada komponen tersebut. Contoh: `UsageRecordTable.svelte:23-24` memakai `min-w-[64rem]`, `ProxyTable.svelte:57-58` memakai `min-w-[62rem]`, `EndpointTable.svelte:33-34` memakai `min-w-[52rem]`, dan `ProviderTable.svelte:23-24` memakai `min-w-[48rem]`. Spesifikasi mengizinkan scroll saat perbandingan penting, tetapi tidak ditemukan alasan per tabel yang menetapkan pengecualian itu.

**Dampak:** Pada layar sempit, pengguna perlu menggeser secara horizontal untuk membaca sebagian tabel; kebutuhan perbandingan dan kesesuaiannya dengan pengecualian belum terdokumentasi per layar.

**Arah tindak lanjut, belum diterapkan (historis: snapshot audit awal):** Catat tabel yang memang memerlukan perbandingan kolom berdampingan beserta alasannya. Untuk tabel lain, evaluasi representasi mobile bertumpuk atau kartu berlabel.

## Batas audit

- Temuan adalah daftar audit, bukan persetujuan perubahan. Tidak ada perbaikan kode yang dilakukan.
- Pada snapshot audit awal, audit visual di browser, perilaku pada perangkat nyata, serta test/build belum diverifikasi karena toolchain dan dependency lokal tidak tersedia.
- Setelah owner memilih nomor yang disetujui, perubahan harus dibatasi pada temuan tersebut lalu diverifikasi pada viewport yang relevan.

## Tindak lanjut AFTER

### F1 (HIGH): RESOLVED

- **Perubahan:** `Modal.svelte` memakai kelas `size-11`, yang menetapkan tombol penutup dialog ke 44 × 44px. Perubahan berlaku pada seluruh dialog yang berbagi komponen tersebut.
- **Regression test:** `combo-dialog-icons.test.ts` kini mengunci `size-11` dan merujuk SPEC-UI §8.7; asumsi sebelumnya bahwa DESIGN.md §7 menetapkan 36px untuk kontrol dialog telah dihapus.
- **Verifikasi:** `pnpm test`: 183 file dan 2.988 test lulus. `pnpm check`: 0 error dan 0 warning. `pnpm build`: berhasil; hasil CSS memuat `.size-11` untuk lebar dan tinggi target. Click-through browser dan uji perangkat sentuh nyata tidak dilakukan, jadi R-35 tidak diklaim lulus.
- **Batas lingkup:** F2, F3, dan F4 tidak diubah; tidak ada perubahan pada `app-ui/pnpm-lock.yaml`.

### F2 (MEDIUM): RESOLVED; merged melalui PR #8

- **Cakupan API:** `StateMessage` menerima `ApiError` utuh; `ApiErrorText` dipakai pada banner inline dan hasil aksi. Route, store, dialog, serta komponen yang sebelumnya hanya menyimpan `.message` kini mempertahankan objek error. Request ID ditampilkan dan dapat disalin hanya untuk `INTERNAL_ERROR`; kode lain tetap menampilkan pesan tanpa ID.
- **Login:** kegagalan login generik mempertahankan `ApiError`, bukan meratakannya menjadi string; regression test membuktikan ID dari header ditampilkan dan dapat disalin.
- **Batas kontrak Playground:** SPEC-UI §8.2 merujuk ke envelope error API dari SPEC-API §8. Playground memakai envelope panel tersendiri sesuai §6.15. Relay sekarang meneruskan `x-request-id` upstream yang benar-benar ada, dan UI menampilkannya untuk `INTERNAL_ERROR` di envelope panel atau kode gateway yang bersarang. Error konfigurasi lokal Playground tidak menyediakan ID; tidak ada ID rekaan maupun klaim bahwa SPEC menetapkan field ID untuk envelope tersebut.
- **Regression test:** komponen StateMessage/ApiErrorText, login, custom-provider, Playground model-read/chat, serta relay upstream menguji ID dan tombol salin. Respons tanpa header tetap tidak memunculkan ID.
- **Pemulihan sesi pada chat POST (koreksi QA):** respons panel-session 401 kini diteruskan sekali ke `reportUnauthorized`; regression test memastikan hanya satu request dan satu callback, tanpa loop retry. Status 403 tidak menghapus sesi. Penolakan gateway key tetap menjadi kegagalan normal 502 `GATEWAY_KEY_REFUSED` dan tidak menghapus sesi. Ini adalah cacat pemulihan sesi, bukan bukti auth bypass.

### F3 (LOW): RESOLVED; merged melalui PR #8

- **Perubahan:** query provider divalidasi dengan `schemaProviderQuery` di route dan helper API sebelum request. Panjang maksimum 120 karakter mendapat umpan balik inline dan request invalid tidak dikirim.
- **Regression test:** nilai 120 karakter diterima; 121 karakter menghasilkan error inline dan tidak memanggil transport API.
- **Follow-up cakupan test F3 pada PR #11:** mencakup astral Unicode dan mixed code-point boundaries; ini perubahan test-only dan tidak mengubah runtime.

### F4 (LOW): PARTIALLY VERIFIED/[UNVERIFIED]; belum ditutup, residual diterima hanya untuk push/PR bersyarat

- **Perubahan:** 16 tabel kandidat memakai kelas bersama `mobile-key-value-frame` / `mobile-key-value-table` yang menyusun tiap baris menjadi kartu berlabel di bawah 768px. Label sel mempertahankan konteks kolom; baris gateway key mengambil label dari `GatewayKeyRow`. Sel `Actions` tetap berupa `<td data-label="Actions">`; kelas flex berada pada wrapper di dalam sel.
- **Pengecualian:** tidak ada tabel yang dipertahankan sebagai scroll horizontal untuk pembacaan mobile; semua kandidat mendapat representasi key-value, jadi tidak ada pengecualian kolom berdampingan yang perlu dibenarkan.
- **Alasan desain (R-31):** tabel padat tetap menjadi tabel di desktop; pada viewport mobile kolom tidak lagi harus dibandingkan berdampingan, sehingga setiap nilai ditampilkan sebagai pasangan label-nilai agar terbaca tanpa scroll horizontal sesuai DESIGN.md §8 dan SPEC-UI §8.7 butir 2.
- **Pemeriksaan statis:** 16 kandidat masih memiliki tabel desktop dan kelas responsive; tidak ada kombinasi `overflow-x-auto` + `min-width` yang memaksa scroll, dan sel data memiliki label (GatewayKeyRow menjadi sumber label terpisah untuk tabel gateway keys).
- **Bukti viewport parsial dari Security:** halaman Endpoint & Key diperiksa pada viewport 375px dan 768px dengan scope satu baris sintetis dan mock-auth. Bukti ini hanya mencakup halaman tersebut, bukan semua tabel.
- **Temuan dan cakupan visual:** pada 768px, kelompok aksi tersusun vertikal. Endpoint & Key adalah satu-satunya tabel dengan bukti viewport; 15 tabel lainnya hanya diperiksa secara statis.
- **Keputusan owner (2026-10-11):** owner menerima stack aksi vertikal 768px sebagai residual produk dan mengizinkan push/PR bersyarat ke `development` setelah gate, scope, dan metadata lulus. Penerimaan ini hanya untuk residual tersebut, bukan sign-off F4. PR #7 kemudian memang menggabungkan `development` ke `main` pada `1f7427629eef05dcea46e6dbc3331e4506bc69fe`; otorisasi merge PR #7 dan fakta merge itu tidak mengubah status F4 menjadi terverifikasi atau tertutup. Tidak ada sign-off visual F4 terpisah untuk `main` atau release yang tercatat.
- **Accessibility tree:** pada halaman Endpoint & Key, tabel, header, cell, serta tombol bernama `Rename`, `Disable`, dan `Delete` muncul pada accessibility tree; browser console tidak melaporkan error. Ini bukan uji screen reader. Screen reader/AT dan sentuh di perangkat nyata tetap **[UNVERIFIED]**.

### F5: RESOLVED WITH ACCEPTED LOW RESIDUAL

- **Integrasi:** perubahan F5 masuk lewat PR #10 (`feat/app-ui-fase-5` → `feat/app-ui-fase-2`, merge `1ee6f22c926d4fb38d69e9b0bdb1b832378a7453`), lalu PR #8 ke `development` (`78607f40194629de0e57861a7126a33b02f2f6bd`). Issue #9 ditutup dengan komentar resolusi owner.
- **Perubahan:** relay memfilter upstream `x-request-id` yang mengandung key terkonfigurasi, meredaksi key dari `code`, `type`, dan `message` upstream, membersihkan pesan fallback dan kegagalan jaringan yang dibuat panel, serta mempertahankan nilai aman. Autentikasi, skema kode error, dan validasi `PANEL_API_TARGET` tidak diubah.
- **Residual yang diterima:** key non-native yang valid menurut skema dan dikonfigurasi manual dapat kebetulan sama dengan `error.code` luar yang dibuat panel (misalnya `GATEWAY_ERROR`). Tidak ada collision manual seperti itu yang diketahui; residual LOW ini tidak mengubah skema atau kontrak error-code. Status ini bukan klaim zero findings.

### Verifikasi dan gates

- **Type-check:** `npm run sync && npm run check`: `svelte-check` 0 error, 0 warning.
- **Regression suites terarah (historis; snapshot/run tidak teridentifikasi):** 14 file test, 109 test lulus (login, StateMessage/ApiErrorText, custom provider, Playground, relay, provider search, disabled models, provider thinking, bulk keys, serta model test).
- **Full quality gate historis sebelum PR #11 (2026-10-11; kode aplikasi sesuai commit `7eb51c1`):** `panel-check.sh` lulus: Prettier, ESLint, SvelteKit sync, `svelte-check` 0 error/0 warning, Vitest 186 file dan 3.006 test, serta production build berhasil. Ini snapshot terdahulu, bukan jumlah test final PR #11.
- **Cakupan TDD PR #11 (test-only):** menutup TDD §2.5 untuk 18 fungsi non-F5 pada 11 file; satu `it()` mencakup variasi sequential/await. Tidak ada perubahan runtime.
- **Full gate final PR #11 pada source tree SHA `001141b9fe4ad2aa569b5f4d2ef523c08cdd2b38`:** 3.014 Vitest tests lulus. Integrasi PostgreSQL di-skip karena `PANNELAI_TEST_POSTGRES_DSN` tidak disetel; skip tersebut bukan bukti bahwa integrasi PostgreSQL lulus.
- **Gitleaks historis sebelum PR #11 (snapshot SHA/tanggal tidak teridentifikasi):** `secrets.sh --all` mencatat 392 commit dan working tree tanpa kebocoran. Ini hasil snapshot lama, bukan pemindaian final PR #11. Gate panel pada snapshot itu menggunakan fallback npm/Node; jalur startup Bun `--preload` tetap belum terverifikasi.
- **Gitleaks final pada source tree SHA `001141b9fe4ad2aa569b5f4d2ef523c08cdd2b38`:** memindai 397 commit plus working tree; tidak menemukan kebocoran.
- **Hosted GHAS PR #7:** check-run `114392414165` pada run `38113051717` berakhir **FAILURE**; log menyatakan HTTP 402 karena layanan AI review melewati kuota bulanan. Owner menerima status unavailable untuk merge, tetapi check tetap **FAILURE/not PASS**. Enam check-run lain pada head `056b4f8` sukses; hasil ini bukan security-clean verdict.
- **Status F4 dan batas verifikasi:** F4 tetap parsial dan belum ditutup. Bukti 375/768 hanya mencakup Endpoint & Key dengan satu baris sintetis dan mock-auth; viewport 15 tabel lainnya belum diverifikasi secara visual. Screen reader/AT dan uji sentuh di perangkat nyata tetap **[UNVERIFIED]**. R-35 tidak diklaim lulus. Owner menerima residual stack vertikal 768px hanya untuk push/PR bersyarat; acceptance ini bukan verifikasi atau penutupan F4. QA/PM telah memeriksa bukti.
- **`app-ui/.prettierignore`:** `pnpm-lock.yaml` tetap tidak dilacak dan tidak masuk perubahan. Baris ignore diperlukan karena pemeriksaan Prettier package-wide memasukkan YAML lokal yang sudah ada; merapikannya akan mengubah artefak package manager yang tidak terkait dan tidak diminta. Alasan ini dicatat di sini dan sebagai komentar tepat di file ignore.
- **Keputusan dan keadaan merge:** penerimaan residual F4 sebelumnya hanya mendukung push/PR bersyarat ke `development`, bukan sign-off F4. PR #11 kemudian merged ke `development` pada `056b4f8`; PR #7 menggabungkan `development` ke `main` pada `1f74276`. Otorisasi merge PR #7 dan penerimaan HTTP 402 GHAS sebagai unavailable bukan sign-off visual F4; tidak ada catatan sign-off F4 terpisah untuk `main`/release. Catatan merge ini tidak memperluas bukti visual atau menutup F4; tidak ada klaim release.
