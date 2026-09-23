# 020-ENDPOINT-KEY-COPY.md: Kontrol copy di dalam dialog melaporkan sukses sementara clipboard kosong

Dokumen kerja pass `app-ui` untuk halaman `/endpoint-keys`. Pass ini mengerjakan satu cacat yang
dilaporkan owner dari click-through-nya sendiri. Bukan kontrak; kontrak tetap
`docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel) dan `docs/SPEC-API/001-SPEC-API.md` (wire).

|                      |                                                                                                                                                     |
| -------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**           | CLOSED 2026-09-23. F1 sampai F3 selesai; commit lokal `app-ui` + catatan draft 020 ini dan baris register draft 007, belum di-push                              |
| **Permintaan owner** | "app-ui, Pada Page: Endpoint & Keys, disini tombol `copy` tidak bekerja sama sekali. Fix dan perbaiki" (2026-09-23)                                 |
| **Scope**            | `app-ui/.` saja. `app-serv/.` tidak disentuh pass ini                                                                                               |
| **Kaitan**           | SPEC-UI §6.2 (modal kunci sekali tampil), §5.2 (dialog API base), §8.10; draft 007 F7 (test render empat layar), F12; `CopyButton.svelte`, `Modal.svelte` |
| **Tanggal**          | 2026-09-23                                                                                                                                          |

## 1. Ringkasan

Owner melaporkan tombol `copy` di halaman Endpoint & Key tidak bekerja sama sekali. Halaman itu punya
satu kontrol copy: modal `Copy this key now` yang muncul setelah sebuah gateway key dibuat (§6.2).
Kontrol yang sama juga dipakai tiga kali di dialog `API base` pada header shell (§5.2), jadi keduanya
diperiksa dan keduanya memang cacat yang sama.

Yang diukur lebih dulu, bukan yang diasumsikan: kontrol itu **melaporkan sukses sementara clipboard
tetap kosong**. Dijalankan di browser sungguhan pada origin yang tidak aman (`http://172.20.10.5:3000`,
`isSecureContext: false`, `navigator.clipboard` tidak ada), sebuah gateway key dibuat lewat UI, tombol
`Copy` ditekan dengan event mouse sungguhan, dan isi clipboard dibaca kembali dengan menempelkannya ke
sebuah field:

| Langkah                     | Sebelum perbaikan                        | Sesudah perbaikan                        |
| --------------------------- | ---------------------------------------- | ---------------------------------------- |
| Status yang dilaporkan      | `Copied.`                                | `Copied.`                                |
| `doneDisabledAfterCopy`     | `true`                                   | `true`                                   |
| Isi yang menempel kembali   | **kosong**                               | `sk-8EFgsX4PG2Zep6peaZze7tCHhDAHrce8WHSBt9Z6ELLzxpBT` |

Baris `doneDisabledAfterCopy` mengukur atribut `disabled` tombol `Done`, yang di `OneTimeKeyModal.svelte`
memang bergantung pada checkbox acknowledgement (`disabled={!acknowledged}`), bukan pada hasil copy; nilainya
sama di kedua kolom dan bukan sinyal cacat. Yang jadi sinyal adalah baris terakhir: isi yang menempel
kembali. Gerbang yang dibuka oleh copy yang gagal adalah `dismissible={acknowledged || copied}`, yaitu
Escape dan klik backdrop, dan itulah yang membuat sukses palsu berbahaya di layar ini.

Jadi cacatnya bukan tombol yang mati, melainkan kontrol yang berbohong: pada modal kunci sekali tampil
itu juga membuka gerbang penutupan, sehingga kunci yang hanya ada satu kali bisa hilang tanpa pernah
sampai ke clipboard. Itu tepat aturan yang §6.2 larang.

## 2. F1: mekanismenya, dan mengapa hanya jalur seleksi yang kena

Jalur pertama (`navigator.clipboard.writeText`) tidak menyentuh DOM, jadi ia tidak terpengaruh. Yang
rusak adalah jalur kedua, dan penyebabnya satu sifat dialog modal: **`<dialog>` yang dibuka dengan
`showModal()` membuat seluruh dokumen di luarnya inert**, dan simpul inert tidak bisa difokuskan maupun
diseleksi. Scratch field jalur itu (`document.createElement('textarea')`) di-append ke
`document.body` (`CopyButton.svelte`, `writeLegacy`), yaitu tepat di luar dialog, sehingga:

1. `area.select()` tidak menghasilkan seleksi apa pun,
2. `document.execCommand('copy')` **tetap mengembalikan `true`** walau tidak ada yang disalin,
3. kontrol membaca `true` itu sebagai sukses dan menulis `Copied.`.

Diukur di Chrome 150 dengan tiga kasus yang nilainya berbeda satu sama lain, supaya clipboard yang
tertinggal dari kasus sebelumnya tidak bisa memalsukan hasil. Setiap kasus menempelkan kembali ke field
yang sudah dikosongkan:

| Kasus                                              | `execCommand` | `document.activeElement === area` | `getSelection()` | Clipboard menerima |
| -------------------------------------------------- | ------------- | --------------------------------- | ---------------- | ------------------ |
| Dialog terbuka, field di `document.body` (kode lama) | `true`        | `false`                           | `''`             | **kosong**         |
| Dialog terbuka, field di dalam dialog (perbaikan)    | `true`        | `true`                            | nilai            | nilainya           |
| Tanpa dialog, field di `document.body`               | `true`        | `true`                            | nilainya         | nilainya           |

Baris pertama adalah cacatnya, baris ketiga menjelaskan kenapa cacat ini tidak pernah muncul di layar
yang kontrol copy-nya tidak berada di dalam dialog (API Docs, Skills, Available Models): di sana field
yang sama berfungsi.

Dua pengukuran lain ikut menentukan bentuk perbaikan, dan keduanya juga dari browser:

- `selectionStart`/`selectionEnd` **tidak** bisa dipakai sebagai bukti: pada kasus inert keduanya tetap
  terbaca `0` dan `17`, seolah seleksinya benar.
- Setelah field dihapus, fokus jatuh ke `BODY` (di luar dialog), jadi perbaikan harus mengembalikannya;
  tanpa itu operator kehilangan fokus keyboard-nya di dalam modal.

## 3. F2: perbaikannya

Tiga hal, semuanya di `src/lib/components/CopyButton.svelte`:

1. **Scratch field masuk ke dialog terbuka** kalau ada (`document.querySelectorAll('dialog[open]')`, yang
   terakhir di urutan dokumen, dan di setiap alur panel ini itulah yang di atas), dan ke `document.body`
   kalau tidak ada. Itu satu-satunya subtree yang tidak inert saat dialog modal tampil, dan itu yang
   membuat seleksinya mendarat.
2. **Jawaban `execCommand` tidak lagi dipercaya sendirian.** Setelah `focus()` dan `select()`, kontrol
   memeriksa `document.activeElement === area`; kalau tidak, tidak ada seleksi untuk disalin dan kontrol
   melaporkan gagal. Ini yang menjaga §6.2: copy yang gagal tidak boleh membuka gerbang penutupan modal.
3. **Fokus dikembalikan** ke elemen yang tadi fokus sebelum scratch field dihapus, supaya operator tidak
   terlempar ke luar dialog.

## 4. F3: bukti

**Test.** Dua kasus baru di `tests/components/copy-button.test.ts` (file itu dari 4 menjadi 6 kasus):

- kontrol menaruh scratch field-nya di dalam dialog yang terbuka (host diambil dari dalam stub
  `execCommand`, karena jsdom tidak bisa mereproduksi inert),
- kontrol melaporkan gagal saat scratch field-nya tidak bisa mengambil fokus (persis yang inert lakukan
  di browser: `focus()` ditolak), dan `execCommand` tidak pernah dipanggil.

Keduanya dibuktikan bisa gagal: mengembalikan host ke `document.body` memunculkan
`expected [ 'BODY' ] to deeply equal [ 'DIALOG' ]`, dan menghapus guard fokus memunculkan kegagalan pada
kasus kedua. Pohon dikembalikan byte-identik sesudah kedua mutasi itu (`md5sum -c`).

**Click-through di build yang dikirim.** Panel dijalankan dari `build/` pada `http://172.20.10.5:3000`
menunjuk gateway asli `127.0.0.1:9090`, login sungguhan, halaman `Endpoint & Key`, kunci dibuat lewat UI,
tombol `Copy` ditekan dengan mouse sungguhan, modal ditutup lewat acknowledgement, lalu clipboard dibaca
dengan menempelkan kembali. Pengukuran **sebelum** perbaikan (kolom kiri tabel §1) berjalan di dev server
yang sedang dipakai melihat panel (`/tmp/live-panel.log`, banner `(development)`), bukan build produksi;
pengukuran sesudahnya yang berjalan di `build/`. Dua permukaan diukur, dua screenshot disimpan. Driver:
`/tmp/copyrepro/panel-drive.ts` (Chrome headless + CDP, klik dan ketik lewat `Input.dispatchMouseEvent`/
`Input.dispatchKeyEvent`, karena `.click()` dari JS tidak membawa user activation); bukti:
`/tmp/copyrepro/panel-drive.out` (sebelum perbaikan), `/tmp/copyrepro/panel-after.out` dan
`/tmp/copyrepro/panel-shot.out` (sesudah, yang terakhir sekaligus mengambil gambar); tangkapan layar:
`/tmp/copyrepro/shots/endpoint-key-copied.png` dan `api-base-copied.png`. Probe mekanisme dan fokus ada di
`/tmp/copyrepro/mechanism.ts` dan `focus.ts` (halaman `page.html` dilayani `serve.ts` di port 8899),
keluarannya `mechanism.out` dan `focus.out`.

| Permukaan                       | Nilai yang ditampilkan                     | Status      | Yang menempel kembali                       |
| ------------------------------- | ------------------------------------------ | ----------- | ------------------------------------------- |
| Modal `Copy this key now` (§6.2) | `sk-8EFgsX4PG2Zep6peaZze7tCHhDAHrce8WHSBt9Z6ELLzxpBT` | `Copied.`   | kunci yang sama                             |
| Dialog `API base`, tab Base URL  | `http://127.0.0.1:9090/api/v1`             | `Copied.`   | alamat yang sama                            |

**Basis data.** Dihitung sebelum dan sesudah, dan dikembalikan persis:
`usage=0 logs=0 keys=3 endpoints=2 upkeys=1 nodes=3 auth_null=false`. Dua baris `gateway_keys` milik pass
ini (berprefix `probe-copy-`) dihapus di akhir; baris milik aktor lain tidak disentuh, dan diukur ulang
sesudah pembersihan: key `playground-live`, `sandbox`, `testing`; endpoint `live endpoint` dan `Key 1`;
satu upstream key; node `oczen`, `occap`, `th-1`. Angka ini berlaku pada jendela pass ini saja: sekitar
pukul 16.33 hari yang sama, tabel operasional DB yang sama dikosongkan oleh jalan lain (lihat
`docs/DRAFT/021-DATAPLANE-SSE-FRAMING.md` §6), jadi pembaca yang mengukur sekarang akan melihat angka yang
berbeda dan itu bukan koreksi atas baris ini.

## 5. Bukti gate

Dijalankan di `app-ui/` pada tree beku `644b0295da11fbf9447b3618d14976d2` (`md5sum` atas `src/`, `tests/`,
`static/`, dan `scripts/`, 497 berkas), sesudah komentar dokumen terakhir di `CopyButton.svelte` dan
sesudah Prettier. Satu jalan suite penuh sebelumnya sempat dimulai pada tree sebelum komentar itu; jalan
itu dihentikan dan diganti jalan di bawah ini, supaya angka yang dicatat berasal dari tree yang sama
dengan yang di-commit.

| Gate              | Hasil                                                                                                                                                                                                                                                                                                                                                            |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bun run check`   | 0 errors, 0 warnings                                                                                                                                                                                                                                                                                                                                             |
| `bun run lint`    | Prettier: seluruh berkas patuh                                                                                                                                                                                                                                                                                                                                   |
| `bun run lint:ts` | ESLint keluar 0                                                                                                                                                                                                                                                                                                                                                  |
| `bun run build`   | sukses, exit 0, keluaran di `build/`                                                                                                                                                                                                                                                                                                                             |
| `bun run test`    | 2606 tes lulus dari 154 berkas, 2187 s di host ini, pada tree beku di atas, hijau di percobaan pertama                                                                                                                                                                                                                                                                                                                            |
| Click-through     | dua tangkapan layar dan dua jalan driver pada build yang dikirim (§4): modal kunci sekali tampil di `/endpoint-keys` dan dialog `API base` di header, keduanya menempelkan kembali nilai yang ditampilkan layar                                                                                                                                                    |
| Ukuran berkas     | dua berkas yang pass ini sentuh keduanya di bawah ambang 220: `src/lib/components/CopyButton.svelte` 114 dan `tests/components/copy-button.test.ts` 122                                                                                                                                                                                                           |
| Higiene teks      | 0 em dash di berkas baru dan tersunting                                                                                                                                                                                                                                                                                                                          |

## 6. Batas yang tidak diklaim

- Yang diperbaiki hanya jalur seleksi. Jalur async (`navigator.clipboard`) tidak berubah, dan di origin
  aman (mis. `localhost`) ia yang dipakai lebih dulu.
- jsdom tidak mereproduksi inertness, jadi test-nya memaku dua fakta yang browser sudah buktikan (host
  field, dan jawaban saat fokus ditolak), bukan menirukan perilakunya.
- Klaim "clipboard menerima" hanya berlaku pada kombinasi yang diukur: Chrome 150 di Linux, origin
  `http://<lan-ip>:3000`. Browser lain tidak diuji pass ini.
- Dialog `API base` ikut sembuh karena memakai kontrol dan `Modal` yang sama. Test render-nya
  (`tests/components/api-base-dialog.test.ts`) sudah menekan jalur seleksi, tetapi hanya pada cabang
  gagalnya (`document.execCommand` tidak ada di jsdom, jadi kontrol melaporkan gagal); tidak ada kasus yang
  menuntut jalur itu melaporkan sukses, jadi cacat "sukses palsu" tidak akan tertangkap di sana. Kasus baru
  pass ini ada di test kontrolnya sendiri, bukan di test layar.
