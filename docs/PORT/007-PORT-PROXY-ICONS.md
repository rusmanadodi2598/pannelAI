# 007-PORT-PROXY-ICONS

**Status: CLOSED 2026-09-26.** Glyph kontrol Proxy Pools terbangun dan teruji, refresh ganda dihapus.

## 1. Direktif

Dua butir dari pemilik (2026-09-26), angka adalah label bukan urutan:

1. Perbaikan penambahan Icon (None Emoticon, None Plain teks) untuk: Refresh Now, Add a Proxy,
   Add Several at once, Refresh, Edit, Test, Delete, Save Outbound settings.
2. Antara "Refresh Now" dan "Refresh" pakai satu saja, jangan double refresh.

Lingkup: app-ui `/proxy-pools` saja (halaman, tabel, kartu outbound). Gateway tidak tersentuh.

## 2. Keputusan desain

- **D1, satu refresh:** kontrol yang dipertahankan adalah `RefreshControl` ("Refresh now") bersama,
  karena ia dirender di setiap state halaman (memuat, galat, tabel), punya aturan klik-saat-baca
  (§8.6.2), dan glyph-nya sudah benar. Tombol polos "Refresh" di toolbar dihapus; kontrol yang hilang
  tidak menghilangkan kemampuan apa pun karena keduanya memanggil `load()` yang sama.
- **D2, glyph baris aksi:** Edit, Test, Delete menjadi icon-only memakai `ROW_ACTION_ICONS`
  (`edit`/`test`/`delete`) dengan kontrak yang sama dengan tabel Endpoint & Key: nama aksesibel dan
  `title` membawa nama barisnya ("Edit Frankfurt egress"), glyph dekoratif, Delete berwarna danger
  dengan tepat satu utilitas `text-*` (pelajaran 2026-09-24). Saat probe berjalan, tombol Test baris
  itu `disabled` (pengganti label "Testing" yang hilang bersama teksnya).
- **D3, glyph kontrol:** "Add a proxy" memakai `CONTROL_ICONS.add` (Plus), "Add several at once"
  memakai entri baru `addMany` (ListPlus), "Save outbound settings" memakai entri baru `save` (Check).
  Semua glyph + label, tidak ada emoticon, tidak ada teks telanjang.
- **D4, state toggle batch:** glyph ListPlus dipertahankan di kedua state ("Add several at once" dan
  "Hide the paste box"); identitas kontrol adalah kemampuan batch-add-nya dan label yang melapor
  state, berbeda dari pause/resume yang dua arahnya dua aksi berlawanan.
- **D5, state kosong:** tombol "Add a proxy" di state kosong mendapat glyph yang sama dengan
  toolbar, karena keduanya satu kontrol yang sama pada state berbeda.
- **D6, "Refresh Now" dan "Test this candidate" (modal form):** sudah sesuai atau di luar daftar;
  tidak diubah.

## 3. Bukti merah-dulu

Berkas uji baru `tests/components/proxy-pools-icons.test.ts` (murni milik pass ini, menyenggol
`proxy-pools.test.ts` milik aktor lain): 7 kasus, seluruhnya MERAH sebelum implementasi
(7 failed / 7, termasuk dua kegagalan yang membuktikan tombol "Refresh" lama masih ada dan kelas
danger belum dipakai). Hijau 57/57 pada 5 berkas uji terkait setelah implementasi.

## 4. Implementasi

- `src/lib/icons.ts`: import `ListPlus`; `CONTROL_ICONS` bertambah `addMany` dan `save`, masing-masing
  dengan alasan tertulis satu kalimat (R-04, R-31).
- `src/routes/proxy-pools/+page.svelte`: glyph + label pada "Add a proxy" (toolbar dan state kosong)
  dan "Add several at once"; tombol "Refresh" toolbar dihapus dengan komentar yang merekam alasannya.
- `src/lib/components/ProxyTable.svelte`: tiga aksi baris icon-only, kelas aksi rumah
  (`actionBase`/`actionClass`/`dangerClass`), `title` = nama aksesibel.
- `src/lib/components/ProxyOutboundSettings.svelte`: glyph save pada tombol simpan.

## 5. Gerbang

- prettier: PASS setelah satu kali `--write` (halaman + berkas uji baru).
- eslint (5 berkas sentuh): PASS tanpa temuan.
- svelte-check: 0 error, 0 warning.
- Suite penuh terakhir di atas pohon beku, sidik jari `6316dde9cc184f008d21ff22a2f9628e`:
  hasil dicatat pada §6.

## 6. Hasil gerbang terukur

- Suite penuh terakhir atas pohon beku `6316dde9cc184f008d21ff22a2f9628e`, dicatat utuh dari berkas
  log: lari pertama (30,8 menit, bersamaan dengan click-through Chrome + panel harness) 2621/2622
  dengan satu timeout 5s yang berpindah berkas (`proxy-pools-save`, kelas kontensi 2 core; berkas
  yang sama hijau pada lari terarah); lari ulang tanpa kontensi 2622/2622 pada 161 berkas dalam
  1433,52 s, EXIT 0, nol FAIL (/tmp/pass37/suite-007.log; lari kontensi disimpan sebagai
  suite-007-contention.log).
- Click-through pada panel harness :3015/:3016 (target gateway :9091, sesi operator nyata):
  - Toolbar: `refreshNowCount: 1`, `plainRefreshCount: 0` (bukti hidup D1), `refreshNowSvg: true`,
    `addOneSvg: true`, `addManySvg: true`.
  - Toggle batch: "Hide the paste box" hadir dengan svg, kotak tempel muncul dan tertutup.
  - Aksi baris (pool hidup, baris "proxy 1"): teks `""`, `title` = "Edit/Test/Delete proxy 1", svg
    dekoratif (`aria-hidden="true"`), Delete membawa tepat satu utilitas warna
    `text-[var(--color-danger)]` tanpa utilitas muted yang bersaing.
  - "Save outbound settings": svg dekoratif + label utuh.
  - Refresh now membaca ulang pool: bacaan /api/v1/proxies 1 menjadi 2.
  - 390 px: `docSw: 390`, `scrollX` 0 paksa; 32 elemen yang melewati viewport adalah tabel di dalam
    wrapper scroll-nya sendiri (min-w 62rem), halaman sendiri tak meluas.
  - `windowErrors: none`. Tangkapan layar: /tmp/pass37/shots/after-desktop.png, after-390.png.

## 7. Residu

- "Test this candidate" dan "Save proxy" di dalam modal form tidak memakai glyph (di luar daftar
  direktif); konsistensinya bisa jadi pass berikutnya bila diminta.
- Pekerjaan proxy app-serv milik aktor lain tetap tidak tersentuh di pohon.
