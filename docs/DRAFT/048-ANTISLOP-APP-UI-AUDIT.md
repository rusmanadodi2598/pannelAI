# 048-ANTISLOP-APP-UI-AUDIT.md: Target sentuh modal, diagnostik error, dan layout tabel mobile

Audit AFTER atas `app-ui` pada 2026-10-10, HEAD `e024f515eccbe90c328a067b3e74a244351a11fb`. Pada snapshot audit awal, dokumen ini mencatat temuan dan arah tindak lanjut saja; saat itu tidak ada kode, aset, atau konfigurasi produk yang diubah.

## Status dan lingkup

| | |
| --- | --- |
| **Status** | **PARTIALLY RESOLVED. F1 HIGH selesai ditindaklanjuti; F2 MEDIUM, F3 LOW, dan F4 LOW tetap OPEN.** |
| **Mechanism** | AFTER (audit) |
| **Scope** | `app-ui/src/**`, `app-ui/tests/**`, `DESIGN.md`, serta `docs/SPEC-UI/001-SPEC-UI.md` dan `docs/SPEC-API/001-SPEC-API.md` sebagai kontrak UI/API |
| **Sumber temuan** | Pembacaan statis dan pencocokan kode, test, desain, dan spesifikasi; Anti Slop R-03/R-31 serta kriteria fungsional C-2/C-4 |
| **Verifikasi pada snapshot audit** | Statis pada HEAD di atas. `bun` dan `app-ui/node_modules` tidak tersedia, jadi test, type-check, build, dan click-through browser belum dijalankan pada snapshot tersebut. Ini bukan klaim lulus R-35. |
| **Perubahan saat audit awal** | Hanya dokumen DRAFT ini; tidak ada perubahan pada kode produk pada snapshot audit awal. Tindak lanjut F1 dicatat di bagian bawah. |

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

**Arah tindak lanjut, belum diterapkan:** Tambahkan tampilan dan kontrol salin request ID pada penanganan `INTERNAL_ERROR`, lalu uji respons yang memuat dan tidak memuat header tersebut.

### F3 (LOW) Provider search menerima istilah yang server pasti tolak

**Aturan:** `docs/SPEC-API/001-SPEC-API.md` §7.4 membatasi `q` sampai 120 karakter; `docs/SPEC-UI/001-SPEC-UI.md` §7 mensyaratkan validasi/sanitasi input di batas panel.

`app-ui/src/routes/providers/+page.svelte:127-147` menyediakan kolom pencarian tanpa batas panjang dan `applySearch()` (`:97-103`) hanya melakukan `trim()`. `app-ui/src/lib/schemas/provider.ts:142-146` memang mendefinisikan `schemaProviderQuery.q` dengan `.max(120)`, tetapi `app-ui/src/lib/api/providers.ts:52-58` meneruskan query langsung ke `apiRequest` tanpa memvalidasi dengan skema itu. Test server `app-serv/internal/handler/provider_search_test.go:77-107` membedakan 120 karakter yang diterima dari 121 karakter yang ditolak.

**Dampak:** Istilah lebih dari 120 karakter menghasilkan penolakan API yang tampil sebagai error halaman, bukan umpan balik validasi pada kolom pencarian.

**Arah tindak lanjut, belum diterapkan:** Terapkan skema query sebelum request dan tampilkan pesan dekat kolom pencarian; uji batas 120 dan 121 karakter.

### F4 (LOW) Scroll horizontal tabel mobile menjadi pola umum tanpa alasan per layar

**Aturan:** R-31 (Quality Lock); `DESIGN.md` §8 dan `docs/SPEC-UI/001-SPEC-UI.md` §8.7 butir 2 meminta tabel ditata menjadi baris key-value di bawah 768px, kecuali perbandingan berdampingan memang esensial.

Pencarian statis menemukan 16 komponen di `app-ui/src` yang menggabungkan `overflow-x-auto` dengan lebar minimum tetap, tanpa cabang representasi mobile bertumpuk yang tampak pada komponen tersebut. Contoh: `UsageRecordTable.svelte:23-24` memakai `min-w-[64rem]`, `ProxyTable.svelte:57-58` memakai `min-w-[62rem]`, `EndpointTable.svelte:33-34` memakai `min-w-[52rem]`, dan `ProviderTable.svelte:23-24` memakai `min-w-[48rem]`. Spesifikasi mengizinkan scroll saat perbandingan penting, tetapi tidak ditemukan alasan per tabel yang menetapkan pengecualian itu.

**Dampak:** Pada layar sempit, pengguna perlu menggeser secara horizontal untuk membaca sebagian tabel; kebutuhan perbandingan dan kesesuaiannya dengan pengecualian belum terdokumentasi per layar.

**Arah tindak lanjut, belum diterapkan:** Catat tabel yang memang memerlukan perbandingan kolom berdampingan beserta alasannya. Untuk tabel lain, evaluasi representasi mobile bertumpuk atau kartu berlabel.

## Batas audit

- Temuan adalah daftar audit, bukan persetujuan perubahan. Tidak ada perbaikan kode yang dilakukan.
- Pada snapshot audit awal, audit visual di browser, perilaku pada perangkat nyata, serta test/build belum diverifikasi karena toolchain dan dependency lokal tidak tersedia.
- Setelah owner memilih nomor yang disetujui, perubahan harus dibatasi pada temuan tersebut lalu diverifikasi pada viewport yang relevan.

## Tindak lanjut AFTER

### F1 (HIGH) — RESOLVED

- **Perubahan:** `Modal.svelte` memakai kelas `size-11`, yang menetapkan tombol penutup dialog ke 44 × 44px. Perubahan berlaku pada seluruh dialog yang berbagi komponen tersebut.
- **Regression test:** `combo-dialog-icons.test.ts` kini mengunci `size-11` dan merujuk SPEC-UI §8.7; asumsi sebelumnya bahwa DESIGN.md §7 menetapkan 36px untuk kontrol dialog telah dihapus.
- **Verifikasi:** `pnpm test` — 183 file dan 2.988 test lulus. `pnpm check` — 0 error dan 0 warning. `pnpm build` — berhasil; hasil CSS memuat `.size-11` untuk lebar dan tinggi target. Click-through browser dan uji perangkat sentuh nyata tidak dilakukan, jadi R-35 tidak diklaim lulus.
- **Batas lingkup:** F2, F3, dan F4 tidak diubah; tidak ada perubahan pada `app-ui/pnpm-lock.yaml`.
