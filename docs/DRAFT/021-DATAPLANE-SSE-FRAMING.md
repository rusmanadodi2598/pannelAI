# 021-DATAPLANE-SSE-FRAMING.md: Frame buatan gateway tidak di-SSE-frame, jadi `[DONE]` tidak terbaca klien

Register temuan `app-serv` dari pengujian data plane yang diminta owner. Bukan kontrak; kontrak tetap
`docs/SPEC-API/001-SPEC-API.md` (baris Streaming di §4). Pass pengujian ini **tidak mengubah** berkas
`app-serv/` maupun `app-ui/` mana pun.

|                      |                                                                                                                                                                                          |
| -------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**           | OPEN 2026-09-23. F1 sampai F5 terukur di gateway yang berjalan; belum ada perbaikan, dan belum ada keputusan siapa yang memperbaiki. Uji ulang 17:50 menemukan tabel endpoint kosong (§9); setelah endpoint dibuat, uji ulang 17:55 menjawab dan F1 sampai F3 tetap (§11), plus F5 (§12); uji ulang 22:41 mengulang F1 sampai F3 dan F5 pada byte baru, plus F6 (§13); uji ulang 23:07 pada gateway yang sudah di-restart mengulang F1, F2, F3, dan F5 dengan bentuk byte yang sama (§14) |
| **Permintaan owner** | "Lanjut testing server dan response AI nya: Endpoint: http://127.0.0.1:9090 \| Api Key: sk-…Ddj6 \| Models: th-1/deepseek-v4.1-flash:free" lalu "Update endpoitnya: http://127.0.0.1:9090/api/v1" (2026-09-23) |
| **Scope**            | Pengujian gateway yang sedang berjalan di `127.0.0.1:9090`; tidak ada kode yang disunting pass ini                                                                                         |
| **Kaitan**           | SPEC-API §4 baris Streaming; `internal/dataplane/translate_stream_openai.go`, `internal/dataplane/stream.go`, `internal/handler/datplane_errors.go`; dampak panel di `app-ui/src/lib/schemas/playground-stream.ts` dan `app-ui/src/lib/api/playground-reader.ts` |
| **Tanggal**          | 2026-09-23                                                                                                                                                                              |

## 1. Yang diuji, dan yang bekerja

Dua rute diuji dengan kunci gateway `kts-1` (hint `sk-…Ddj6`, dibuat owner hari ini) dan model
`th-1/deepseek-v4.1-flash:free` pada node `th-1` ("TH HARBOR 1", `https://tokenharbor.ai/v1`).

| Uji | Hasil |
| --- | --- |
| Non-streaming `POST /api/v1/chat/completions` | HTTP 200 dalam 75,0 s; `content` = `pong` (instruksi "Balas satu kata saja: pong"), `finish_reason` `stop`, `usage` 38/80/118, `model` dilaporkan sebagai `deepseek-v4.1-flash` (prefiks `th-1/` dan sufiks `:free` dilepas gateway) |
| Streaming (`stream: true`) | HTTP 200, TTFB 30,2 s, total 31,2 s, 26 frame `data:` berisi delta `reasoning_content` lalu `content` |
| Streaming + `stream_options.include_usage` | HTTP 200, TTFB 62,3 s, total 76,9 s, 96 frame `data:` |
| Pencatatan | Saat diukur (16:30), gateway menulis baris `usage_records` (model `deepseek-v4.1-flash:free`, 38/80 token, 74.930 ms, `success`) dan `request_logs` yang cocok, serta menaikkan `request_count` kunci menjadi 1 |

Jadi jalur data plane-nya hidup: auth kunci gateway, resolusi prefiks node, kredensial upstream,
penerusan jawaban, dan pencatatan usage semuanya bekerja. Yang cacat adalah **framing SSE pada frame yang
gateway bangun sendiri**, dan itu membuat stream tidak pernah berakhir dengan cara yang bisa dibaca klien
(§2 sampai §5).

## 2. F1 (HIGH): `mustFrame` tidak membungkus frame dengan `data: ` dan terminator

`Frame()` (stream.go:34) menulis `data: ` + payload + `\n\n`, dan jalur penerusan memakainya
(translate_stream_openai.go:178). Tetapi `mustFrame()` (translate_stream_openai.go:208) hanya
`json.Marshal` dan mengembalikan JSON mentah, tanpa prefiks dan tanpa baris kosong penutup, dan
`sseSink.WriteFrame` (handler/datplane_errors.go:122) menulis byte apa adanya. Semua frame yang dibangun
lewat `mustFrame` karena itu keluar tanpa framing:

- frame `finish` sintetis (translate_stream_openai.go:119 lewat `chunk()`),
- chunk `usage` (translate_stream_openai.go:126 dan :180).

Diukur pada byte mentah (`od -c`) di ujung dua stream:

```
... "usage":{...,"total_tokens":124}}\n\n{"id":"chatcmpl-…","choices":[{"index":0,"delta":{},
"finish_reason":"stop"}]}data: [DONE]\n\n
```

Chunk `usage` di baris itu datang berframe benar (dari upstream, diteruskan lewat `Frame()`), lalu frame
`finish` sintetis menempel tanpa prefiks, dan `data: [DONE]` menempel langsung di belakangnya tanpa baris
kosong pemisah. Konsekuensinya bukan kosmetik: `[DONE]` tidak pernah menjadi baris `data:` sendiri,
padahal SPEC-API §4 menyebut penanda itu "the client's only signal that the answer is complete".

## 3. F2 (MEDIUM): chunk `usage` dikirim dua kali saat klien meminta usage

Dengan `stream_options.include_usage: true`, `openAIFrames` menambahkan chunk usage
(translate_stream_openai.go:180) **dan** `Finish()` menambahkannya lagi (translate_stream_openai.go:126)
tanpa syarat. Diukur pada stream yang sama: tiga objek `"usage":{"prompt_tokens"`: satu dari upstream
(berframe benar) dan dua dari gateway (keduanya tanpa framing, menempel ke `data: [DONE]`). Klien yang
meminta usage karena itu menerima dua frame usage yang tidak sah, dan nol frame usage yang bisa dibaca.

## 4. F3 (MEDIUM): frame `finish` dikirim dua kali saat upstream sudah mengirimnya

`s.finishSent` hanya diset di cabang `includeUsage` (translate_stream_openai.go:181), tidak di jalur
penerusan (translate_stream_openai.go:178). Jadi ketika upstream mengirim frame `finish` sendiri dan klien
**tidak** meminta usage, `Finish()` melihat `finishSent == false` dan mengirim frame `finish` kedua
(translate_stream_openai.go:114-121), yang menurut F1 juga tanpa framing. Diukur: `"finish_reason":"stop"`
muncul dua kali pada stream pertama, satu berframe dari upstream dan satu telanjang dari gateway.

## 5. Dampak terukur: `[DONE]` tidak terbaca, panel berhenti sebagai `truncated`

Pembaca SSE milik panel (`app-ui/src/lib/schemas/playground-stream.ts:29`) memecah buffer pada baris
kosong lalu hanya mengambil baris yang diawali `data:`. Karena `data: [DONE]` menempel pada frame
telanjang, penanda itu tidak pernah menjadi frame tersendiri. Diukur dengan menjalankan pemecah itu
langsung atas byte yang direkam (`/tmp/reader-probe.ts`):

```
frames parsed: 26
sees the DONE sentinel: false
sees a finish frame: true
```

`playground-reader.ts` karena itu keluar dari loop baca saat body habis dan memanggil
`handlers.onEnd('truncated')` (playground-reader.ts:90), bukan `done`. Artinya jawaban yang selesai normal
dilaporkan sebagai jawaban yang terpotong. Ini bukan cacat panel: panel membaca sesuai SPEC-API §4, dan
byte yang salah datang dari gateway.

## 6. Catatan lingkungan: tabel operasional DB dikosongkan di tengah pengujian

Diukur 16:30 dan 16:36 pada DSN yang sama, dan urutannya dicatat apa adanya karena memengaruhi reproduksi:

- 16:30: `gateway_keys` 4 baris (termasuk `kts-1` milik owner), `upstream_endpoints` 2, `upstream_keys` 1.
- Sekitar 16:33 ada yang mengosongkan tabel operasional: tinggal `gateway_keys` = 1 (`record-use`, hint
  `sk-…alue`, `request_count` 3), `upstream_endpoints` = 0, `upstream_keys` = 0, plus tiga baris fixture
  `req_alpha`/`req_beta`/`req_gamma` (model `gpt-4o`, `claude-3`, `o3-mini`, latensi 42 ms) di
  `request_logs`. `provider_nodes` (termasuk `th-1`) dan `panel_auth` tidak ikut dikosongkan.
- Permintaan uji ketiga (16:33:15 sampai 16:34:32) tercatat `status=error`, `error=MODEL_NOT_FOUND`,
  walaupun klien menerima HTTP 200 dan stream lengkap. Mekanismenya tidak diverifikasi di sini.

Dua akibat yang perlu diketahui owner: kunci `kts-1` tidak lagi ada (permintaan berikutnya dengan kunci
itu akan ditolak), dan karena `upstream_endpoints` kosong, model `th-1/deepseek-v4.1-flash:free` tidak
punya endpoint untuk dirutekan sampai endpoint-nya dibuat ulang. Siapa yang mengosongkan tabel itu tidak
diketahui dari sini.

## 7. Batas yang tidak diklaim

- Yang diukur hanya satu upstream (`tokenharbor.ai`) dan satu model; upstream lain tidak diuji, dan
  apakah mereka mengirim frame `finish`/`usage` sendiri tidak diperiksa.
- Dampak panel diukur dengan menjalankan pemecah SSE panel atas byte yang direkam, bukan dengan
  menjalankan ulang Playground di browser; yang tidak dijalankan tidak diklaim.
- Latensi (TTFB 30-62 s) adalah milik upstream pada jalur ini; tidak ada perbandingan upstream lain, dan
  tidak ada klaim bahwa gateway menambah atau mengurangi latensi.
- Jalur non-streaming tidak menunjukkan cacat framing apa pun pada pengukuran ini.
- Perbaikan tidak dikerjakan: `app-serv` milik tim lain, jadi temuan ini dicatat dan tidak disentuh.

## 8. Bukti dan cara mengulang

| Berkas | Isi |
| --- | --- |
| `/tmp/chat1.json` | jawaban non-streaming lengkap (HTTP 200, `content` `pong`) |
| `/tmp/chat-stream.txt` | stream tanpa `include_usage` (26 frame, ujung telanjang + `[DONE]` menempel) |
| `/tmp/chat-usage.txt` | stream dengan `include_usage` (96 frame, dua chunk usage telanjang) |
| `/tmp/reader-probe.ts` | pemecah SSE panel dijalankan atas byte di atas; mencetak `sees the DONE sentinel: false` |

Perintah pengukuran, dijalankan pada gateway yang sedang berjalan:

```bash
curl -sS -N -o /tmp/chat-stream.txt -w 'HTTP %{http_code} ttfb=%{time_starttransfer}s total=%{time_total}s\n' \
  -X POST http://127.0.0.1:9090/api/v1/chat/completions \
  -H "Authorization: Bearer <kunci gateway>" -H "Content-Type: application/json" \
  -d '{"model":"th-1/deepseek-v4.1-flash:free","messages":[{"role":"user","content":"Sebutkan tiga warna, satu kata per baris."}],"stream":true,"max_tokens":64}'

tail -c 420 /tmp/chat-stream.txt | od -c        # ujung telanjang, `data: [DONE]` menempel
bun run /tmp/reader-probe.ts                    # pemecah panel: DONE tidak terlihat
```

Untuk mengulang setelah §6, endpoint dan kredensial upstream untuk node `th-1` harus dibuat ulang lebih
dulu, karena tabel `upstream_endpoints` dan `upstream_keys` sekarang kosong.

## 9. Uji ulang 2026-09-23 17:50 dengan kunci baru: tidak ada satu endpoint pun

Owner meminta uji ulang ke `http://127.0.0.1:9090/api/v1` dengan kunci baru (hint `sk-…rmEu`; baris
`gateway_keys` `gky_0386DNQ1Z30M7R9KC1N1WH3K1R` bernama `sandbox`, `status` `active`). Kunci `kts-1` dari
§6 sudah tidak ada, jadi kunci inilah yang dipakai. Tiga panggilan `POST /api/v1/chat/completions`
dilakukan, semuanya dengan `max_tokens` 16.

| Uji | Hasil terukur |
| --- | --- |
| `GET /api/v1/models` dengan kunci baru | HTTP 200, 362 id; tidak ada satu pun berawalan `th-1/` |
| Model yang diminta owner, `th-1/deepseek-v4.1-flash:free` | HTTP 503 dalam 54 ms, `code` `NO_PROVIDER_AVAILABLE`, pesan "no upstream endpoint is configured for provider openai-compatible-0386BKG9Q4DYZPYC01VJH4C51G" |
| Model registry lain, `deepseek/deepseek-chat` | HTTP 503, `NO_PROVIDER_AVAILABLE` untuk provider `deepseek` |
| Id katalog dipakai apa adanya, `openai-compatible-03863XJ2YM2KHF847XJP5YBSH8/deepseek-v4.1-flash` | HTTP 503 untuk node itu; id katalog memang bisa dirutekan, dan prefiks `th-1` maupun id node sama-sama resolve ke node yang benar |
| Pencatatan ketiga penolakan | `request_logs` `0386DWD3X639MWWN7JB85SHREA` (17:50:19), `0386DWFVAYBY4W4RZQ08D3J7RT` (17:51:04), `0386DWM9M3827EA167SGT5CCQY` (17:52:17), semuanya `status` `error`, `error` `NO_PROVIDER_AVAILABLE`, `latency_ms` 0; `usage_records` mencatat tiga baris kembar (`usg_0386DWD3YXEQ2F33CFSVJBBYYA`, `usg_0386DWFVZ1F1R3VJZAGNTH3MKG`, `usg_0386DWMA8A1SNTE3Z2FXZZ3FV6`) dengan token 0 dan biaya 0 |
| Keadaan tabel saat diukur | `upstream_endpoints` 0, `upstream_keys` 0, `provider_nodes` 3 (`oczen`, `occap`, `th-1`), `gateway_keys` 1 (`sandbox`), `models_custom` 1 (model `th-1`), `panel_auth` 1; `combos`, `models_disabled`, `quota_windows`, `quota_caps`, `model_aliases`, `proxies`, dan `settings` semuanya 0 |

Kesimpulannya: kunci gateway, resolusi prefiks node, dan pencatatan penolakan semuanya bekerja dan jujur
(503 berkode mesin, bukan 200 palsu), tetapi **tidak ada koneksi upstream untuk node mana pun**, jadi tidak
ada model yang bisa dirutekan. Hosti `tokenharbor.ai` sendiri hidup: resolve dan menjawab HTTP 401 di
`https://tokenharbor.ai/v1/models`, jadi yang hilang adalah kredensial tokenharbor.ai, bukan kode. Uji ini
tidak bisa lulus sampai endpoint untuk node `th-1` dibuat ulang (§8, kalimat terakhir).

## 10. F4 (LOW, pertanyaan desain): katalog menyebut 362 id saat tidak ada satu pun yang bisa dirutekan

`ModelList` (`internal/dataplane/catalog.go:37`) menyusun daftar dari indeks registry dan himpunan disabled
saja; berkas itu tidak pernah membaca `upstream_endpoints` (satu-satunya kemunculan kata "endpoint" di
berkas itu ada di komentar paket). Komentarnya sendiri menyebut tujuannya menjaga "listed" dan "answerable"
sebagai satu sifat, dan pada keadaan §9 keduanya berbeda: 362 id terdaftar, 0 endpoint aktif, dan setiap
panggilan berakhir 503. Ini bukan cacat yang terbukti salah satu pihak, melainkan keputusan tim: apakah
katalog sebaiknya menyembunyikan model milik node yang tidak punya endpoint aktif. Tidak ada usulan
perbaikan dari pass ini, dan tidak ada kode yang disentuh.

Catatan pengukuran yang menyertainya: katalog menyebut model node custom dengan **id node**
(`openai-compatible-…`), bukan prefiksnya (`oczen`), dan id itu memang bisa dirutekan apa adanya (§9 baris
keempat). Model passthrough seperti `th-1/deepseek-v4.1-flash:free` tidak muncul di katalog, sesuai
komentar `catalog.go:34` bahwa provider passthrough tidak menghitung katalog. Uji ulang §11 mengukuhkannya:
setelah satu endpoint aktif dibuat untuk `th-1`, modelnya tetap tidak muncul, sementara 362 id yang
terdaftar tetap tanpa endpoint.

## 11. Uji ulang 2026-09-23 17:55 setelah endpoint dibuat: data plane menjawab, F1 sampai F3 tetap

Setelah §9, sebuah endpoint dibuat untuk node `th-1` (`ep_0386DWPTPD77W6S3QNFH1RZAE4`, label `Key 1`,
`auth_type` `api_key`, `priority` 1, `status` `active`, plus satu baris `upstream_keys`). Owner meminta uji
ulang dengan kunci yang sama. Tiga panggilan dilakukan dan semuanya dijawab upstream.

| Uji | Hasil terukur |
| --- | --- |
| Non-streaming, model `th-1/deepseek-v4.1-flash:free` | HTTP 200 dalam 72,6 s; `content` `pong`, `reasoning_content` "The user wants exactly \"pong\". Simple.", `finish_reason` `stop`, `usage` 36/13/49; `model` di wire `deepseek-v4.1-flash` |
| Streaming tanpa `include_usage` | HTTP 200, TTFB 42,2 s, total 43,6 s, 5729 byte: 24 frame berframe + 1 frame **telanjang** (frame `finish` sintetis), `data: [DONE]` menempel di ujung frame telanjang itu, dan `"finish_reason":"stop"` muncul dua kali |
| Streaming dengan `stream_options.include_usage: true` | HTTP 200, TTFB 79,6 s, total 79,6 s, 1333 byte: 4 frame berframe + 1 baris telanjang berisi **dua** objek `usage` kembar (42/76/118) lalu `data: [DONE]` menempel |
| Katalog setelah endpoint ada | tetap 362 id dan tetap tanpa `th-1/`; 79 id di antaranya milik node `oczen` yang tidak punya endpoint |
| Pencatatan | tiga baris `usage_records` `success` (`usg_0386DWYAYVV6MAY50GN940D5QE` 17:55:01, `usg_0386DX19WQSBD4EPHD4FX8C11T` 17:55:50, `usg_0386DX6Y5RG4D1ZTGH4Q6XTTRE` 17:57:22) dan tiga baris `request_logs` yang cocok (`0386DWSX449J73SXFXW4509EWE`, `0386DWYMQ7P2HH85P43F4QAHZ6`, `0386DX22PMEQP66993MPXS3KSM`), semuanya `status` `success`, `latency_ms` 72257/42996/79250 |

Jadi jalur data plane hidup kembali begitu endpoint ada: auth kunci gateway, resolusi prefiks, kredensial
upstream, dan penerusan jawaban bekerja. F1, F2, dan F3 dari §2 sampai §4 **tetap tereproduksi** pada byte
baru, dengan bentuk yang sama seperti pengukuran pertama: frame `finish` dan chunk `usage` buatan gateway
keluar tanpa `data: ` dan tanpa baris kosong, sehingga `data: [DONE]` tidak pernah menjadi frame sendiri
(0 kecocokan `^data: \[DONE\]$` di kedua stream).

## 12. F5 (MEDIUM): permintaan streaming mencatat 0 token di `usage_records`

Diukur pada dua permintaan streaming §11: byte stream membawa `usage` 42/76/118 (di stream tanpa flag, satu
chunk berframe dari upstream; di stream dengan flag, satu chunk berframe dari upstream plus dua chunk
telanjang dari gateway), tetapi baris `usage_records` keduanya mencatat `tokens_in` 0 dan `tokens_out` 0
dengan `status` `success` dan `cost_usd` 0. Permintaan non-streaming pada jam yang sama mencatat 36/13
dengan benar, jadi ini khusus jalur streaming, bukan kegagalan pencatatan umum. Dampaknya:
`GET /api/v1/usage/summary|records` dan layar `/usage` melaporkan permintaan streaming sebagai nol token,
dan cap kuota yang menghitung token tidak pernah melihat panggilan streaming. Mekanismenya (usage dibaca
sebelum chunk terakhir tiba, atau `Finish()` tidak menyalin usage ke akuntansi) tidak diverifikasi di sini;
yang diukur hanya selisihnya.

## 13. Uji ulang 2026-09-23 22:41: F1 sampai F3 dan F5 tetap, dan dua panggilan yang tidak selesai tidak tercatat sama sekali

Owner mengirim ulang endpoint, kunci, dan model yang sama (kunci `sandbox`, hint `sk-…rmEu`; model
`th-1/deepseek-v4.1-flash:free`). Lima panggilan dilakukan pada gateway yang berjalan. Keadaan tabel saat
mulai: `provider_nodes` 3, `upstream_endpoints` 3 (semuanya untuk `th-1`, label `Key 1`, `Key 2`, `Key 3`,
`priority` 1, `status` `active`), `upstream_keys` 3, `gateway_keys` 2, `usage_records` 5, `request_logs` 3,
`models_custom` 1, `panel_auth` 1.

| Uji | Hasil terukur |
| --- | --- |
| `GET /api/v1/models` | HTTP 200, 363 id; 0 berawalan `th-1/`, dan 80 id milik node `oczen` muncul di bawah **id node** `openai-compatible-03863XJ2YM2KHF847XJP5YBSH8`, bukan di bawah prefiksnya |
| Non-streaming | HTTP 200 dalam 18,1 s; `content` `pong`, `reasoning_content` ada, `finish_reason` `stop`, `usage` 38/29/67; `model` di wire `deepseek-v4.1-flash` |
| Streaming tanpa `include_usage` (ulangan, bersih) | HTTP 200, TTFB 41,1 s, total 41,1 s, 1097 byte: 4 frame berframe, lalu satu frame `finish` **telanjang** dengan `data: [DONE]` menempel; `"finish_reason":"stop"` muncul dua kali; 0 kecocokan `^data: \[DONE\]$` |
| Streaming + `include_usage` (ulangan, bersih) | HTTP 200, TTFB 32,3 s, total 32,3 s, 1316 byte: 4 frame berframe, lalu satu baris telanjang berisi **dua** objek `usage` kembar (37/26/63) dengan `data: [DONE]` menempel |
| Pembaca SSE panel atas kedua byte itu | `frames=4`, `seesDone=false`; teks, model, finish, dan usage terlipat benar, dan layar akan melaporkan alasan akhir `truncated` |

Jadi F1, F2, F3, dan F5 tetap seperti §11; yang bergerak hanya angka usage dari upstream. Dua temuan baru
menyertainya.

### 13.1 F6 (MEDIUM): panggilan yang tidak selesai tidak menulis baris akuntansi sama sekali

Dua dari lima panggilan tidak selesai, dan keduanya tidak meninggalkan satu baris pun di `usage_records`
maupun `request_logs`, sementara `gateway_keys.request_count` kunci `sandbox` naik dari 3 menjadi 8 (lima
panggilan terhitung semuanya):

| Panggilan | Yang diterima klien | Baris akuntansi |
| --- | --- | --- |
| Streaming tanpa flag, percobaan pertama | HTTP 200, TTFB 119,4 s, total 125,8 s, 3905 byte, `curl` keluar 18 ("transfer closed with outstanding read data remaining"): 17 frame berframe, tanpa frame telanjang, tanpa `data: [DONE]` | tidak ada |
| Streaming + `include_usage`, percobaan pertama | Tidak ada byte sama sekali; koneksi ditutup setelah 178,9 s, `curl` keluar 52 ("Empty reply from server") | tidak ada |

Kedua percobaan itu berhasil saat diulang sendirian (tabel di atas), jadi kejadian ini
**tidak tereproduksi** dan sisi mana yang memutus koneksi tidak bisa dipastikan dari sini. Hosti
`tokenharbor.ai` sendiri sehat saat diukur: `GET https://tokenharbor.ai/v1/models` menjawab HTTP 401 dalam
0,52 s. Yang bisa diklaim: pada keadaan ini panggilan yang gagal di tengah stream tidak terlihat di
`/usage` dan tidak terlihat di log permintaan, walaupun kuncinya tetap menghitung panggilan itu.

### 13.2 F5 tetap, dan satu selisih kecil

Kedua stream yang selesai menulis `usage_records` dengan `tokens_in` 0 dan `tokens_out` 0 (`status`
`success`, `latency_ms` 32281 dan 40716) padahal wire membawa 41/56/97 dan 37/26/63; panggilan
non-streaming pada jam yang sama mencatat 38/29 dengan benar. Selisih kecil yang ikut terukur: kolom
`model` di `usage_records` menyimpan `deepseek-v4.1-flash:free` (dengan sufiks), sedangkan `model` di wire
sudah dilepas menjadi `deepseek-v4.1-flash`.

### 13.3 Batas dan keadaan akhir

Yang diukur tetap satu upstream dan satu model. Tiga baris `usage_records` dan tiga baris `request_logs`
yang ditulis pass ini **sengaja ditinggalkan** sebagai rekaman jujur pengujian, seperti §11; menghapusnya
berarti menghapus bukti F5 dan F6. `request_count` kunci `sandbox` saat itu 8.

## 14. Uji ulang stream 2026-09-23 23:07 sampai 23:11, setelah gateway di-restart: bentuk yang sama

Owner meminta pengecekan ulang permintaan streaming pada model yang sama. Gateway tempat pengukuran
berjalan sudah diganti sejak §13: PID `2212055` mati dan digantikan PID `2214789` pada 22:54:23 (biner dari
cache build Go), sedangkan keadaan endpoint tidak berubah (`upstream_endpoints` 3 untuk `th-1`,
`upstream_keys` 3). Dua percobaan pertama setelah restart tidak menerima satu byte pun selama 280 s
(batas `curl`), dan keduanya tidak menulis baris akuntansi (tidak ada baris di antara 22:46:28 dan
23:03:56); karena yang memutus koneksi di situ adalah klien, kejadian itu **tidak** dihitung sebagai bukti
F6.

| Uji | Hasil terukur |
| --- | --- |
| Non-streaming | HTTP 200 dalam 72,4 s; `content` `pong`, `finish_reason` `stop`, `usage` 38/18/56, `model` di wire `deepseek-v4.1-flash`; baris `usage_records` 38/18, `latency_ms` 72350 (23:07:53) |
| Streaming tanpa `include_usage` | HTTP 200, TTFB 51,8 s, total 58,6 s, 4572 byte: 19 frame berframe, lalu satu frame `finish` **telanjang** dengan `data: [DONE]` menempel; `"finish_reason":"stop"` dua kali; 0 kecocokan `^data: \[DONE\]$`; baris `usage_records` 0/0, `latency_ms` 58520 (23:08:57) |
| Streaming + `include_usage` | HTTP 200, TTFB 101,4 s, total 113,8 s, 3169 byte: 12 frame berframe, lalu satu baris telanjang berisi **dua** objek `usage` kembar (37/26/63) dengan `data: [DONE]` menempel; 0 kecocokan `^data: \[DONE\]$`; baris `usage_records` 0/0, `latency_ms` 113385 (23:10:56) |
| Pembaca SSE panel atas kedua byte itu | `frames=19` dan `frames=12`, `seesDone=false` di keduanya; teks, model, finish, dan usage terlipat benar, dan layar tetap melaporkan alasan akhir `truncated` |

Jadi pada biner yang baru di-restart, F1, F2, F3, dan F5 **tetap** dengan bentuk byte yang identik; yang
bergerak hanya latensi upstream (non-streaming 18,1 s menjadi 72,4 s; TTFB stream 41,1 s dan 32,3 s menjadi
51,8 s dan 101,4 s). Tidak ada kode yang disentuh pass ini. Pada penutup pengukuran, `request_count` kunci
`sandbox` terbaca 17, `usage_records` 18 baris, dan `request_logs` 19 baris; angka tabel terakhir itu juga
memuat baris milik aktor lain pada 23:03 sampai 23:04 (model `big-pickle`, `gpt-4o`,
`claude-sonnet-4-5-20250929`, semuanya `NO_PROVIDER_AVAILABLE`) dan kunci baru `probe-combo-key`, jadi bukan
seluruhnya berasal dari pengujian ini.
