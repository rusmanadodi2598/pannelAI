# 049-MOBILE-LAYOUT-PROVIDER-COMBO-USAGE: audit layout mobile halaman Detail Provider, Combo dan Vision, serta Usage topology

Dokumen kerja atas permintaan owner: **"layout mobile masih berantakan, baik teks, component, dan layout yang kurang menarik"** pada tiga area `app-ui`: Detail Provider (bawaan dan custom endpoint OpenAI/Anthropic), Combo dan Vision, serta Usage (node motion). Dokumen ini bukan kontrak; kontrak tetap `docs/SPEC-UI/001-SPEC-UI.md`. Pola mengikuti draft 021 sampai 026: temuan bernomor F, bukti file:line yang bisa diulang, dan keputusan yang diambil di depan.

| | |
|---|---|
| **Status** | **OPEN 2026-10-11**: F1 sampai F29 diajukan sebagai backlog mobile `app-ui`. Kode **tidak diubah** (audit read-only). Perbaikan menunggu persetujuan nomor temuan oleh owner |
| **Mechanism** | Audit statis kode Svelte: dibaca 14+ file provider, 10+ file combo/vision, 8 file usage topology. Standar: antislop core (R-01 sampai R-38) + `antislop-ui` + `antislop-layoutmobile` + `antislop-human` (kontras AA 4.5:1, tap target min 44px) |
| **Scope** | `app-ui/` saja: `src/routes/providers/[provider_id]/+page.svelte`, `src/routes/combos/+page.svelte`, `src/routes/usage/+page.svelte`, dan komponen di `src/lib/components/`. `app-serv/` tidak disentuh. Desktop dinyatakan OK oleh owner dan tidak diaudit ulang |
| **Permintaan owner** | Perbaiki layout mobile Detail Provider, tombol Combo dan Vision yang tidak simetris, dan teks/node Usage yang terlalu kecil |
| **Kaitan** | SPEC-UI §6 (layar providers, combos, usage), draft 012/013/015/018/022/023 (usage drawing), draft 017/019 (provider parity), draft 024/025 (combo/vision readiness), `docs/RULLES/TDD.md` (protokol implementasi), `docs/RULLES/ANTISLOP.md` (aturan teks, tanpa em dash) |
| **Tanggal** | 2026-10-11 |

## 1. Ringkasan

Tiga audit paralel menemukan **29 temuan** (10 provider, 12 combo/vision, 7 usage), semuanya di viewport sempit (320 sampai 390px). Kabar baiknya: fondasi mobile sudah benar di banyak tempat (grid fakta collapse 1 kolom, pola tabel `mobile-key-value`, drawer `min(32rem,100vw-2rem)`, kontras light theme lolos AA, tombol utama `min-h-11`). Masalahnya terkonsentrasi pada tiga pola: **tap target di bawah 44px** (R-03), **tombol CTA yang tidak satu bahasa ukuran** (R-31), dan **drawing topology yang diskalakan linear tanpa floor** sehingga seluruh isi menyusut bersama (R-03).

Prioritas yang diusulkan: P0 = F11, F16, F17, F18, F23, F24, F28 (keluhan owner yang terbukti + tap target + overflow nyata). P1 = sisanya, dikerjakan sekaligus per file agar satu bahasa ukuran.

## 2. Temuan area Provider (F1 sampai F10)

Sumber dibaca: `src/routes/providers/[provider_id]/+page.svelte`, `ProviderFacts`, `ProviderNodeFacts`, `ProviderEndpoints`, `EndpointTable`, `EndpointKeysTable`, `EndpointDetailDrawer`, `ModelCatalogTable`, `ModelCatalogList`, `CustomModelTable`, `CustomProviderCard`, `ProviderCustomModels`, `CustomModelForm`, `Modal`, `CopyButton`. Catatan lingkup: `EndpointFieldsForm`, `CreateEndpointForm`, `NodeModelsImport`, `ProviderRotationSwitch`, `ProviderOAuth`, `ProviderProxyCard` belum dibaca, butuh audit lanjutan jika disentuh.

### F1 (P0): target sentuh Copy di bawah 44px di tabel model (R-03)

`ModelCatalogTable.svelte:112` dan `CustomModelTable.svelte:88` memakai `<CopyButton value={address} />` tanpa `touchTarget`. Default `CopyButton.svelte:126-128` adalah `size-9` (36px) dan `:138-140` `min-h-9`. Di kartu mobile, tombol 36px sulit disentuh jempol.

Fix: teruskan varian `touchTarget` (`size-11`/`min-h-11`) untuk kedua pemakaian di tabel.

### F2 (P1): input lebar fix tidak menyusut di 320-360px (R-03)

`ModelCatalogList.svelte:128` (`w-64`), `CustomModelForm.svelte:74,82,90` (`w-64`/`w-56`/`w-64`). Baris form `CustomModelForm.svelte:69` memakai `flex flex-wrap` tetapi label/input tanpa `min-w-0`/`flex-1`, sehingga input 256px memaksa baris di viewport 320px.

Fix: `w-full max-w-64 min-w-0` (dan `max-w-56` untuk display name), label `min-w-0 flex-1 basis-48`.

### F3 (P0): retry/empty-state links tanpa tinggi sentuh (R-03)

`[provider_id]/+page.svelte:174` (`Try again`), `:175` (`Back to registry`), `ProviderEndpoints.svelte:114`, `ModelCatalogList.svelte:165,177`, `ProviderCustomModels.svelte:136`, `CustomProviderCard.svelte:151`: `class="underline"` murni (tinggi aktual sekitar 20px). Tidak konsisten: `Clear filters` di `ModelCatalogList.svelte:150` sudah `min-h-11`.

Fix: `inline-flex min-h-11 items-center underline` untuk semua text-button retry/clear, agar konsisten dengan yang sudah benar.

### F4 (P1): tombol "Open" kolom Detail hanya tinggi, lebar di bawah 44px (R-03)

`EndpointTable.svelte:77`: `class="min-h-11 underline"` pada teks "Open" (lebar sekitar 40px). Tinggi lolos, lebar tidak.

Fix: `inline-flex min-h-11 min-w-11 items-center justify-center px-2 underline`.

### F5 (P1): grup aksi ikon berdempet `gap-1` di kartu mobile (R-03, Targets Too Close)

`EndpointKeysTable.svelte:90` (3 tombol 44px), `ModelCatalogTable.svelte:134` + `CustomModelTable.svelte:110` (2 tombol, `justify-end`). Di `app.css:220-231` sel mobile menjadi grid sempit label/nilai, sehingga 3x44px plus gap hanya 4px berdesakan di kolom nilai, dan `justify-end` menjauh dari jempol.

Fix: `gap-2`, dan di mobile `justify-start` (pertahankan `sm:justify-end`).

### F6 (P1): aksi ikon bernama generik tanpa konteks baris (R-32)

`EndpointTable.svelte:86` dan `EndpointKeysTable.svelte:115` memakai label "Delete" generik. Di daftar kartu mobile, pengguna sentuh dan pembaca layar tidak tahu baris mana yang dihapus.

Fix: `aria-label` berkonteks (contoh: nama entry/kunci bersangkutan), dan `title` mengikuti.

### F7 (P1): kolom Test `max-w-72` melebihi kolom nilai kartu mobile (R-03)

`ModelCatalogTable.svelte:130` dan `CustomModelTable.svelte:106`. Kolom nilai kartu mobile sekitar 65% dari 360px (kira-kira 200px) setelah label `minmax(6.5rem,35%)` (`app.css:222`), sehingga 288px meluap.

Fix: lingkup ke desktop (`sm:max-w-72`), biarkan mobile `min-w-0`.

### F8 (P1): teks hasil test drawer tanpa pemutus kata (R-03)

`EndpointDetailDrawer.svelte:179-184`: `{lastTest.state} {latency}ms{message}` tanpa `break-words`/`min-w-0` di dalam `flex flex-wrap` (`:169`). Pesan error panjang berisiko meluap di 360px.

Fix: `min-w-0 break-words` pada span hasil.

### F9 (P1): footer modal tidak wrap (R-03)

`Modal.svelte:79`: `flex justify-end gap-2` tanpa `flex-wrap`. Tombol drawer "Delete connection" (`EndpointDetailDrawer.svelte:224`, `px-4`) dan "Close" pas-pasan di 320px atau zoom 200%.

Fix: tambah `flex-wrap`.

### F10 (P1): back link header kecil (R-03)

`[provider_id]/+page.svelte:160`: `w-fit text-sm underline` tanpa tinggi sentuh.

Fix: `inline-flex min-h-11 items-center`.

### Lolos provider (terverifikasi, tanpa aksi)

Grid fakta collapse 1 kolom (`ProviderFacts.svelte:25`, `ProviderNodeFacts.svelte:14`) plus `break-all` di URL; pola tabel `mobile-key-value` (`app.css:184-247`) mengubah baris jadi kartu berlabel (nol `overflow-x` di mobile); lebar drawer `w-[min(32rem,calc(100vw-2rem))]` (`Modal.svelte:58`); tombol utama/Add/pagination/filter capability semua `min-h-11`; tombol tutup dialog `size-11`. Kontras light theme dihitung manual dan lolos AA (muted 6.81:1, warn 5.44:1, danger 6.27:1, ok 5.62:1). Dark theme tidak terverifikasi (token dark tidak dibaca), dicatat sebagai tindak lanjut verifikasi.

## 3. Temuan area Combo dan Vision (F11 sampai F22)

Sumber dibaca: `src/routes/combos/+page.svelte`, `CombosTab`, `CombosToolbar`, `ComboEditor`, `ComboTable`, `ComboIdentityFields`, `ComboStrategyFields`, `ComboModelRows`, `VisionAdapterForm`, `VisionModelPicker`, `RefreshControl`. Keluhan owner tentang simetri tombol **terbukti**.

### F11 (P0): Save vs Cancel tidak simetris (R-31, R-11)

`ComboEditor.svelte:203-220`. Save `min-h-11 px-4 rounded bg-accent` vs Cancel `underline` tanpa `px/border/rounded`. Di mobile keduanya berdampingan (`flex-wrap items-center gap-3`), Cancel terlihat jauh lebih kecil.

Fix: wadah jadi `flex-col sm:flex-row`, kedua tombol `w-full sm:w-auto justify-center min-h-12 px-4 rounded`. Cancel diberi varian outline (`border px-4`), bukan underline.

### F12 (P1): New combo vs Refresh beda size (R-31)

`CombosToolbar.svelte:32-43` + `RefreshControl.svelte:44`: `px-4 bg-accent` vs `px-3 border`. Wrapper `gap-2` bersarang dengan `RefreshControl:41 gap-3`. Di mobile dua tombol setengah-lebar berdampingan, tidak full-width.

Fix: inner row `max-sm:flex-col max-sm:items-stretch`; kedua tombol `max-sm:w-full max-sm:justify-center max-sm:min-h-12`, samakan `px-4 gap-3`.

### F13 (P1): Add models outline kecil vs Save filled (R-31)

`ComboEditor.svelte:180-187`: `px-3 border` vs Save `px-4`.

Fix: samakan ke `px-4 min-h-12 w-full sm:w-fit justify-center`.

### F14 (P1): Add models Vision dan Add a model menyusut selebar konten (R-31)

`VisionModelPicker.svelte:72-79` dan `ComboModelRows.svelte:145-152`: `w-fit px-3`. Di mobile menyusut selebar konten, tidak sejajar Save `px-4 w-auto`.

Fix: `w-full sm:w-fit justify-center min-h-12 px-4`.

### F15 (P1): baris Save Vision hanya satu CTA rata kiri (R-31)

`VisionAdapterForm.svelte:214-222`: `flex-wrap gap-3` satu CTA, tidak simetris dengan Add models di atasnya.

Fix: `flex-col sm:flex-row`, Save `w-full sm:w-auto justify-center min-h-12 px-4`.

### F16 (P0): tap target di bawah 44px di empty state (R-03)

`CombosTab.svelte:162,176-186`: "Try again" dan "Create the first combo" hanya `inline-flex gap-2 underline` tanpa `min-h-*` (tinggi aktual sekitar 20-24px).

Fix: `min-h-11 px-2` pada kedua tombol (visual tetap underline, hit-area 44px).

### F17 (P0): baris Judge tidak collapse, risiko overflow nyata (R-03)

`ComboStrategyFields.svelte:53-60`: `flex items-center gap-2` tanpa `flex-wrap`. Input `min-w-48` (192px) `flex-1` + tombol Choose sekitar 110px + padding form 32px = sekitar 342px, pecah di 320px.

Fix: `flex flex-wrap` + `max-sm:flex-col max-sm:items-stretch`; input `min-w-0 w-full`; Choose `w-full justify-center min-h-12 px-4`.

### F18 (P0): pagination tidak wrap (R-03)

`CombosTab.svelte:204-230`: `flex items-center gap-3` tanpa `flex-wrap`. Prev + "Page X of Y" + Next berdesakan di 360px. Tombol Prev/Next `min-h-11` OK tetapi tanpa `px` (lebar = konten).

Fix: tambah `flex-wrap`; tombol `px-3`.

### F19 (P1): target 44x44 terlalu rapat (R-03, Targets Too Close)

`ComboTable.svelte:80`: `gap-1` (4px) antar tombol Edit/Test/Delete `min-h-11 min-w-11`.

Fix: `gap-2` (8px).

### F20 (P1): row wrap tak terkontrol di ComboModelRows (R-03)

`ComboModelRows.svelte:63-140`: `flex flex-wrap items-center` berisi ref `min-w-48 flex-1` + priority `w-24` fixed + 3 tombol 44px. Di mobile priority 96px menggantung setengah baris, aksi menempel.

Fix: `max-sm:flex-col max-sm:items-stretch`; grup priority+aksi jadi baris kedua `w-full justify-between`; atau minimal aksi `w-full justify-start`.

### F21 (catatan, bukan bug): ritme spacing campur

`ComboIdentityFields.svelte:20` stack otomatis (OK karena `min-w-56 flex-1`), tetapi form `gap-4` vs inner `gap-3/gap-2/gap-1`. Samakan ritme saat menyentuh file lain, jangan buka PR khusus.

### F22 (catatan, anti-fix keliru): checkbox Vision sudah OK

`VisionAdapterForm.svelte:174,184`: label `min-h-11` = hit-area 44px walau box `size-5`. Dicatat agar tidak "diperbesar" keliru saat implementasi.

## 4. Temuan area Usage topology (F23 sampai F29)

Sumber dibaca: `UsageTopologyDrawing.svelte`, `UsageTopology.svelte`, `UsageTopologyHop.svelte`, `UsageTopologyEdges.svelte`, `UsageLivePanel.svelte`, `UsageTotalsTiles.svelte`, `usage-topology-geometry.ts`, `usage-topology-view.ts`. Keluhan owner tentang teks/node kecil **terbukti dan terukur**.

Akar masalah satu kalimat: semua metrik node = `px * --u` tanpa floor, dan `--u` menyusut linear dengan lebar box, sehingga di HP seluruh drawing (font, dot, badge, beam) mengecil bersama, bukan reflow.

### F23 (P0): font node/gateway di bawah batas baca mobile (R-25, R-03)

`UsageTopologyDrawing.svelte:140` (`font-size:calc(14px*var(--u))`), `:102` (gateway, sama), `:131` (terminal `13px*--u`). Komentar file sendiri `:26-27` mengukur hasil browser: teks 8.4px di dalam node 47px pada 390px. Estimasi dengan `--u` di `:90` (`--share * width/130`, share max 0.2): box sekitar 230px menjadi `--u` sekitar 0.35, font sekitar 4.9px. Bahkan 8.4px pun gagal WCAG dan aturan zoom (min body mobile sekitar 12px, label 11-12px).

Fix: floor font (`max(12px, calc(14px*var(--u)))` untuk node/gateway, `max(11px, calc(13px*--u))` untuk terminal); atau di bawah 640px ganti drawing ke daftar vertikal, bukan skala.

### F24 (P0): dot status, logo, badge ikut menyusut sampai hilang (R-25 non-teks)

`UsageTopologyDrawing.svelte:145` (`size-[calc(8px*var(--u))]`), `:109` (logo `16px*--u`), `:116` (badge `min-w-[calc(20px*--u))]`, teks `0.857em` dari font yang sudah kecil). Pada `--u` sekitar 0.35: dot sekitar 2.8px, logo 5.6px, badge 7px dengan teks 4px. Status color-only sekecil ini gagal kontras non-teks 3:1.

Fix: floor terpisah (dot `max(8px, ...)`, logo `max(14px, ...)`, badge `min-w:max(20px, ...)` + teks `max(11px, 0.857em)`).

### F25 (P1): label terpotong parah, truncate tanpa akses nama di mobile (R-03, R-32)

`UsageTopologyDrawing.svelte:153` (`max-w-[calc(96px*var(--u))] truncate`). Pada `--u` sekitar 0.35, label max 34px: hampir semua nama provider terpotong jadi 3-5 karakter. Drawing `aria-hidden="true"` (`:91`) sehingga tidak ada cara tap/hover melihat nama penuh (hover-only tidak ada di touch).

Fix: floor `max-width:max(64px, calc(96px*--u))` + fallback list/nama penuh di bawah drawing pada mobile (atau tap-to-expand). Jangan andalkan truncate visual saja.

### F26 (P1): margin `mx-12` mobile mencuri 96px dan memperkecil `--u` (R-03)

`UsageTopology.svelte:68` (`mx-12 sm:mx-16`) di dalam `figure p-4` (`:55`). Di viewport 390px, 96px margin + 32px padding = sepertiga lebar hilang sebelum drawing diukur. Nilai `cqw` di Drawing `:90` mengecil, sehingga F23-F25 memburuk. Ini pola desktop-sized spacing di mobile.

Fix: `mx-2 sm:mx-16` (atau `mx-0`), padding figure `p-2 sm:p-4`.

### F27 (P1): beam jadi sub-pixel dan beban GPU di HP (R-19)

`UsageTopologyHop.svelte:73-78` (`10px*--u`, `5px*--u`, `2.2px*--u`), `:138,:147` (orb/spark `width px*--u` + `animate-beam-travel/spark` infinite). Pada `--u` sekitar 0.35: halo 3.5px, plasma 1.75px, core 0.77px + 6 orbs + 5 sparks per hop berjalan selamanya. Dengan N provider menjadi puluhan animasi infinite di atas node 5px (endless loops, motion tanpa purpose pada skala itu).

Fix: floor stroke (core `max(1.5px, ...)`, plasma `max(2px, ...)`); di bawah 640px matikan orbs/sparks (sisakan halo+core) via media query atau prop `compact`; pertahankan `prefers-reduced-motion` yang sudah ada.

### F28 (P0): tidak ada reflow mobile, satu layout dikecilkan (R-03)

`usage-topology-geometry.ts:30-33` (`MIN_HEIGHT 360`, `HEIGHT_BASE 120 + 22/node`, `MAX 640`); `UsageTopologyDrawing.svelte:89-90` (satu box absolut, persen + `--u`); `UsageTopologyEdges.svelte:35` (`viewBox 0 0 100 100 preserveAspectRatio="none"`). Tidak ada breakpoint/`sm:`/alternatif layout di keempat file topology. HP menggambar layout elips desktop yang sama lalu `preserveAspectRatio="none"` meregangkan garis (stroke diselamatkan `non-scaling-stroke`, tetapi posisi node menumpuk vertikal di box 360px). Mobile = desktop diperkecil, gagal R-03.

Fix, pilih satu di bawah 560-640px: (a) container `overflow-x:auto` + `min-width:520px` pada inner drawing sehingga node tetap 1:1 dan bisa digeser; atau (b) matikan drawing absolut, tampilkan path Client ke Gateway ke Upstream ke Response sebagai list vertikal dengan state dot + label 13px. Keduanya lebih baik dari skala 0.35. Opsi (b) direkomendasikan karena menyelesaikan F23-F25 sekaligus.

### F29 (P1): risiko tumpang tindih di tepi pada box sempit (R-03)

`usage-topology-geometry.ts:16-17` (`RADIUS_X 40`, node center x di 10-90) + `:53` (`NODE_MAX_SHARE 0.2`, center-anchored, setengah node 10%). Node di ujung: center 10% + setengah lebar 10% = tepi tepat 0%, tanpa clearance. Dengan pembulatan/`truncate` dan badge gateway di tengah (`Drawing:100`), tabrakan gateway vs node di 360px sangat mungkin. Tidak ada `overflow` guard/scroll container.

Fix: kecilkan `RADIUS_X` di mobile (misal 32 via prop/query), atau bungkus drawing di `overflow-x:clip` + fallback scroll seperti F28. Verifikasi drag viewport 360 ke 768px (R-35), bukan hanya 390px.

### Lolos usage (tidak perlu diubah)

`UsageLivePanel.svelte:41-42` `TAB_BOX min-h-11` (44px touch target, wrap) dan `UsageTotalsTiles` `grid-cols-2` mobile, keduanya reflow dengan benar.

## 5. Rencana implementasi yang diusulkan (setelah owner menyetujui nomor)

Fase A (P0, satu PR per area atau satu PR gabungan kecil): F11, F16, F17, F18 + F1, F3 + F23, F24, F28. Fase B (P1): F2, F4-F10 + F12-F15, F19, F20 + F25-F27, F29. Estimasi kasar: Fase A 7 file disentuh, Fase B 15+ file. Tindak lanjut verifikasi: kontras dark theme (token dark belum dibaca saat audit) dan audit lanjutan 6 file provider yang belum dibaca (§2).

Aturan implementasi yang mengikat: `docs/RULLES/TDD.md` (test-first, table-driven, RED-GREEN-REFACTOR) untuk setiap perubahan perilaku; `docs/RULLES/ANTISLOP.md` (tanpa em dash, tanpa separator dekoratif, tanpa komentar narasi) untuk setiap file yang disentuh; SPEC-UI §7.1.5/§8.10 untuk bahasa dan typing panel. Setiap PR yang menyentuh layar wajib menjalankan gate `scrypts/gates/all.sh` dan verifikasi R-35: build, click-through tiap kontrol, dan cek viewport 320/360/390/768 (bukan hanya 390).

## 6. Keputusan yang perlu owner ambil

1. Setujui nomor temuan yang dikerjakan (usulan: seluruh P0 dulu, P1 menyusul).
2. Pilih opsi F28: (a) scroll horizontal dengan min-width 520px, atau (b) list vertikal di bawah 640px (audit merekomendasikan b).
3. Konfirmasi tidak ada `DESIGN.md` baru: perbaikan mengikuti bahasa komponen panel yang ada (dark themed), bukan direction baru, agar tidak melanggar R-37.
