# 002-G21-SEAM-DESIGN.md: Desain Seam Empat Kelas Multi-Step

Dokumen kerja untuk **G21** di `docs/DRAFT/001-P2-GAPS.md` §3. Bukan kontrak —
kontrak tetap `docs/SPEC-API/001-SPEC-API.md`. Tujuannya satu: owner menyetujui
bentuk seam per kelas **sebelum** kode ditulis, karena keempat kelas tidak bisa
masuk invariant yang dipakai seluruh adapter G5 — *satu URL, satu request, satu
jawaban*.

| | |
|---|---|
| **Status** | menunggu keputusan owner (§5). Tidak ada perubahan kode di dokumen ini |
| **Dibuat** | 2026-09-19, dari reference `/home/rusmanadodi/apps/9router/open-sse/handlers/` dan `registry.yaml` |
| **Kaitan** | G21 (register), SPEC-API §7.10 (batas format), §7.11 (rute egress) |

## 0. Invariant yang diminta berubah

| Kelas | Provider | Yang dibutuhkan | Aturan baru yang diminta |
|---|---|---|---|
| A | AssemblyAI (stt) | tiga request berurutan + polling | satu adapter boleh **N request upstream**, satu baris akuntansi |
| B | AWS Polly (tts) | penandatanganan SigV4 | kredensial materi >1 nilai + `{region}` yang harus diisi |
| C | Edge TTS, Google TTS (tts) | ambil token pihak ketiga + cache TTL | provider yang `base_url`-nya **penanda**, bukan URL |
| D | Local Device (tts) | eksekusi binary host | provider tanpa HTTP sama sekali |

**Yang tidak berubah** (dan desain ini tidak meminta): satu baris `usage_records` +
satu baris `request_logs` per panggilan klien; satu guard egress untuk semua dial;
amplop jawaban §7.10 (`{text}` untuk STT, bytes/base64 + `format` untuk TTS);
kredensial tidak pernah masuk log.

## 1. Kelas A — AssemblyAI (polling)

**Bukti reference** (`sttCore.js:57-84`). Upload `POST https://api.assemblyai.com/v2/upload`
(raw bytes, `Content-Type: application/octet-stream`) → `{upload_url}`; submit
`POST {base_url}` body `{audio_url, speech_models:[model], language_detection:true}`
→ `{id}`; poll `GET {base_url}/{id}` tiap 2 detik sampai 120 detik; `completed` →
`{text}`, `error` → 500 dengan pesan provider, lewat batas → 504. `base_url`
registry adalah URL submit (`https://api.assemblyai.com/v2/transcript`); URL upload
di reference **hardcoded**, bukan turunan.

**Bentuk seam.** Runner multi-langkah di `internal/service` (bukan di
`internal/dataplane`): adapter memanggil `dataplane.MediaCaller.Do` tiga kali, dan
tiap URL dibangun dari target yang sudah lolos guard. Yang dibutuhkan: (a) base URL
efektif sudah ada di `MediaCall`; (b) util path untuk endpoint upload; (c) anggaran
waktu total lewat `context.WithTimeout` yang **juga** menghormati context klien
(koneksi putus ⇒ rangkaian berhenti, bukan lanjut sampai 120 detik); (d) validasi
bentuk `id` dari provider (hanya `[A-Za-z0-9_-]`) sebelum dipakai di path poll,
supaya id tidak menjadi path tambahan ke host yang sama.

**Akuntansi & kesehatan.** Satu panggilan klien = satu baris usage + satu baris log
(latency = total rangkaian), dan endpoint dicatat sehat/gagal **sekali** setelah
rangkaian selesai — bukan per request upstream, karena yang dialami klien adalah
satu panggilan.

**Risiko.** `upload_url` dari provider **tidak pernah didial** gateway (hanya
dikirim balik ke provider yang sama), jadi tidak ada SSRF baru; poll tetap lewat
guard egress yang sama. Kegagalan sebagian (submit sukses, poll gagal) harus
menghasilkan satu kegagalan yang bisa didiagnosis, bukan baris akuntansi ganda.

**Test & bukti live.** Stub tiga langkah di loopback; kasus: selesai di poll
pertama, selesai di poll ketiga, `status:"error"` → `UPSTREAM_ERROR` dengan pesan
provider, poll 5xx → lanjut (bukan gagal), lewat anggaran → penolakan yang
menyebut "timed out" (anggaran di-inject kecil untuk test), dan kontrol benign
OpenAI multipart yang tidak berubah. Live: satu panggilan penuh ke stub tiga
langkah + satu kasus timeout.

**Pertanyaan.** A1 URL upload; A2 anggaran/interval; A3 kode saat lewat anggaran.

## 2. Kelas B — AWS Polly (SigV4)

**Bukti.** Reference **tidak mengimplementasikannya**: `FORMAT_HANDLERS`
(`genericFormats.js:157-169`) tidak memuat `aws-polly`. Registry:
`base_url: https://polly.{region}.amazonaws.com/v1/speech`, `auth_header: aws-sigv4`,
model = engine (`standard`, `neural`, `long-form`, `generative`), tanpa daftar voice.

**Bentuk seam.** Util `internal/dataplane/sigv4.go`: canonical request → string to
sign → `Authorization: AWS4-HMAC-SHA256 Credential=…/…/polly/aws4_request,
SignedHeaders=…, Signature=…`, plus `x-amz-date`. Body Polly
`{Text, VoiceId, OutputFormat:"mp3", Engine: <model>}`. Region tidak boleh ditebak:
kalau credential tidak memuatnya, tolak dengan nama (`VALIDATION_ERROR`), jangan
diam-diam memakai `us-east-1`.

**Risiko.** Secret tidak pernah masuk log (canonical request memuat hash, bukan
secret) — tapi util ini menyentuh kredensial, jadi ia harus diuji terhadap **vektor
resmi AWS**, bukan hanya "stub menerima 200": tanda tangan yang salah tetap terlihat
seperti 403 upstream. Jam host miring >15 menit juga menghasilkan 403 yang sama,
jadi pesan kegagalan harus menyebut kemungkinan itu. Karena tidak ada reference,
tidak ada perilaku terukur yang bisa di-diff — port ini menulis dari dokumentasi AWS.

**Test & bukti live.** Vektor SigV4 resmi AWS (unit), stub yang **menghitung ulang**
tanda tangan dari secret yang sama (server-side verify), region kosong →
`VALIDATION_ERROR`, engine tak dikenal → 400 sebelum dial, kontrol benign.

**Pertanyaan.** B1 letak region; B2 dikerjakan atau ditandai tidak didukung v1.

## 3. Kelas C — Edge TTS & Google TTS (token hasil scraping)

**Bukti reference.** Edge (`edgeTts.js`): token + cookie dari
`https://www.bing.com/translator` (regex `params_AbusePreventionHelper`), cache
**5 menit** (`:12-27`); SSML + `POST https://www.bing.com/tfettts?…` (`:29-50`);
429/403 → invalidate cache + retry **sekali** (`:73-79`); jawaban <1024 byte
dianggap kosong (`:86`); output MP3. Google (`googleTts.js`): token `FdrFJe`/`cfb2h`
dari `https://translate.google.com/`, cache **11 menit** (`:8-20`); teks
disanitasi (tanda baca → spasi, `", "` → `". "`) lalu RPC `batchexecute` `jQ1olc`
(`:27-52`); jawaban di-parse `split("\n")[3]`; <100 karakter dianggap kosong;
output MP3. Keduanya `no_auth` di registry dan `base_url`-nya **penanda**
(`base_url: edge-tts`, `google-tts`).

**Bentuk seam.** (a) `tokenSource` per provider dengan cache TTL di memori proses —
bukan Redis, karena token terikat pada cookie dan egress host, dan tiap replika
boleh punya token sendiri; (b) **aturan baru**: provider boleh mendeklarasikan
`base_url` sebagai penanda. Konsekuensinya harus eksplisit di tiga tempat:
`effectiveBaseURL` tidak menolaknya, `/media-providers` tidak menawarkan override
`base_url` untuk provider ber-penanda, dan SPEC-API §7.10 memuat daftarnya. Tanpa
aturan itu, penanda akan terbaca sebagai "base_url kosong" dan setiap panggilan
ditolak `VALIDATION_ERROR`.

**Risiko.** Scraping HTML vendor rapuh (berubah kapan saja) dan menyentuh ToS/abuse:
permintaan tanpa auth keluar dari egress operator ke `bing.com`/`translate.google.com`,
yang juga harus masuk allowlist egress (opt-in operator, bukan default). Token
adalah materi kredensial di memori — tidak boleh masuk log, DB, atau pesan error.
Kegagalan parse harus menyebut penyebabnya (halaman berubah vs jaringan), bukan 500
buta.

**Test & bukti live.** Stub dua langkah (halaman token + endpoint audio), TTL
di-inject; kasus: cache hit (dua panggilan hanya mengambil token sekali), 429 → retry
sekali lalu sukses, parse gagal → error yang menyebut penyebab, audio pendek →
penolakan, kontrol benign. Live: stub HTML + stub audio di loopback.

**Pertanyaan.** C1 dikerjakan atau tidak (ToS/kerapuhan); C2 aturan penanda; C3
tempat cache.

## 4. Kelas D — Local Device (proses host)

**Bukti reference** (`localDevice.js`). Voice dari `say -v ?` (`:12-25`) atau
PowerShell SAPI (`:27-53`, hanya enumerasi); sintesis `say -o out.aiff` lalu `ffmpeg`
ke MP3 (`:66-79`) — perhatikan sintesisnya **selalu** memanggil `say`, jadi jalur
Windows di reference pun tidak lengkap. Pemilihan platform hanya di enumerasi voice
(`:58`), dan **tidak ada jalur Linux** sama sekali.

**Temuan keamanan.** `voiceId` dan teks sama-sama dikendalikan klien dan sama-sama
menjadi argv (`say -v <voiceId> -o <file> <text>`, `:71-72`). `execFile` tanpa shell
menutup shell injection, tapi **argument injection** tetap terbuka: token yang
diawali `-` dibaca sebagai flag, dan reference tidak memakai terminator `--`. `-v` mengambil
token berikutnya sebagai nilai, jadi bentuk `voiceId` yang berbahaya terbatas; teks
adalah argumen terakhir tanpa pelindung, jadi teks yang diawali `-` berisiko dibaca
sebagai opsi, bukan diucapkan. Mitigasinya dua: pasang `--` sebelum teks dan
validasi `voiceId` terhadap daftar voice yang dienumerasi host (bukan meneruskan
string klien apa adanya). Teks panjang sebagai argumen juga menabrak batas argv.
Ditambah: binary yang dieksekusi adalah binary host — di container tidak ada, dan
ketersediaannya berbeda per mesin.

**Bentuk seam.** Tidak ada seam di arsitektur HTTP sekarang. Dua opsi: (i) tandai
**tidak didukung di v1** dengan alasan tertulis di SPEC-API §7.10; (ii) desain
**local agent** — proses kecil terpisah yang dijalankan operator di mesin yang punya
`say`/SAPI, gateway memanggilnya lewat HTTP (loopback, terautentikasi), sehingga
eksekusi binary tidak pernah terjadi di proses gateway dan invariant satu-URL tetap
berlaku.

**Pertanyaan.** D1 pilih (i) atau (ii); D2 kalau (ii): kontrak agent, otentikasi
loopback, dan aturan allowlist-nya.

## 5. Keputusan yang diminta

| # | Pertanyaan | Pilihan | Usulan |
|---|---|---|---|
| G21-A1 | URL upload AssemblyAI | (a) turunkan `origin + /v2/upload`, (b) field registry `upload_url` | (b) eksplisit — turunan implisit mudah salah saat host berubah |
| G21-A2 | Anggaran & interval poll | (a) 120 detik / 2 detik seperti reference, (b) lebih kecil | (a) paritas reference, diturunkan oleh `timeout_ms` provider bila diisi |
| G21-A3 | Kode saat lewat anggaran | (a) 504, (b) 502 `UPSTREAM_ERROR` | (b) satu kode untuk kegagalan upstream; pesan menyebut "timed out" |
| G21-B1 | Letak region Polly | (a) komposit `keyId:secret:region` (pola PlayHT), (b) field di endpoint record | (a) tanpa migrasi |
| G21-B2 | Polly dikerjakan? | (a) ya, tulis dari dokumentasi AWS, (b) tidak didukung v1 | (b) tidak ada reference untuk mengukur "benar" |
| G21-C1 | Edge/Google TTS dikerjakan? | (a) ya, opt-in + allowlist, (b) tidak didukung v1 | (b) untuk v1: scraping HTML + ToS |
| G21-C2 | `base_url` penanda | (a) penanda + adapter memiliki URL, (b) registry `endpoints:` eksplisit | (b) penanda mudah salah dibaca sebagai base_url kosong |
| G21-C3 | Tempat cache token | (a) memori proses, (b) Redis | (a) token terikat cookie + egress host; tiap replika boleh punya sendiri |
| G21-D1 | Local Device | (a) tidak didukung v1, (b) desain local agent | (a) untuk v1; (b) kalau operator memang butuh |

## 6. Urutan yang diusulkan (kalau disetujui)

1. **Kelas A (AssemblyAI)** — satu-satunya kelas yang punya perilaku reference
   terukur dan tidak menuntut aturan registry baru; sekaligus membuktikan seam
   "N request, satu baris akuntansi" yang akan dipakai kelas lain.
2. **Kelas C** — kalau owner menerima risiko ToS; butuh aturan penanda `base_url`.
3. **Kelas B** — kalau owner menerima bahwa port ini ditulis dari dokumentasi, bukan
   dari reference.
4. **Kelas D** — keputusan (i)/(ii) lebih dulu; implementasi hanya kalau (ii).

Definisi selesai tiap provider mengikuti register G21: seam + adapter, tabel test
termasuk kontrol benign, satu bukti live ke stub yang meniru alurnya — **atau**
keputusan tertulis "tidak didukung di v1" dengan SPEC-API §7.10 menyebut alasannya.
