# 021-DATAPLANE-SSE-FRAMING.md: Frame buatan gateway tidak di-SSE-frame, jadi `[DONE]` tidak terbaca klien

Register temuan `app-serv` dari pengujian data plane yang diminta owner. Bukan kontrak; kontrak tetap
`docs/SPEC-API/001-SPEC-API.md` (baris Streaming di §4). Pass pengujian ini **tidak mengubah** berkas
`app-serv/` maupun `app-ui/` mana pun.

|                      |                                                                                                                                                                                          |
| -------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Status**           | OPEN 2026-09-23. F1 sampai F5 terukur di gateway yang berjalan; belum ada perbaikan, dan belum ada keputusan siapa yang memperbaiki. Uji ulang 17:50 menemukan tabel endpoint kosong (§9); setelah endpoint dibuat, uji ulang 17:55 menjawab dan F1 sampai F3 tetap (§11), plus F5 (§12); uji ulang 22:41 mengulang F1 sampai F3 dan F5 pada byte baru, plus F6 (§13); uji ulang 23:07 pada gateway yang sudah di-restart mengulang F1, F2, F3, dan F5 dengan bentuk byte yang sama (§14); cek dua model `oczen` 23:14 menjawab 503 karena node itu tanpa endpoint (§15); uji empat model 2026-09-24 08:43 menjawab `th-1` dan menolak tiga model `oczen` (400 untuk bentuk telanjang, 503 untuk bentuk terkuantifikasi, §16); uji ulang empat model yang sama 09:12 hasilnya identik dengan §16 dan F1 sampai F3 serta F5 tetap (§17); uji combo `pi-agent` 2026-09-24 15:45 (empat model yang sama plus combo, pada gateway yang di-restart 15:36 dengan override rotasi per provider) mengulang F1 sampai F3 dan F5, memverifikasi rotasi `round-robin` `sticky_limit` 2 pada ketiga kredensial `th-1`, dan menemukan F7: kredensial `Key 2` menggantung setiap panggilan tanpa timeout, tanpa failover, dan tanpa baris akuntansi (§18); uji request bersih 2026-09-24 16:33 (setelah `usage_records` dikosongkan atas permintaan owner, satu request per ref persis seperti yang disebut, tanpa probe) menjawab `th-1` dan `pi-agent` dengan 200 `pong` sementara tiga nama telanjang tetap 400, seluruhnya tercatat di akuntansi yang mulai dari nol (§19); cek filtering combo dan provider aktif 2026-09-24 membandingkan empat permukaan di kedua pohon, mencatat yang sudah setara (combo tanpa syarat aktivitas di kedua pohon, predikat picker panel sama dengan picker REFERENCE, `?active=true` katalog bekerja) dan menemukan F8 (daftar klien tanpa filter provider aktif), F9 (picker combo hanya halaman tabel), dan F10 (model custom provider aktif tidak masuk daftar klien) (§20); uji combo `pi-agent` 2026-09-25 08:50 (tiga mode, sembilan panggilan, hanya `pi-agent` yang dipanggil) mengulang F1 sampai F3 dan F5 pada binary baru, memverifikasi rotasi tiga anggota `opencode` lewat endpoint virtual tanpa kredensial, dan memverifikasi mekanisme F5 di `engine_relay.go:168` (§22); patch F1, F2, F3, F5, timeout fold, dan F6 mendarat 2026-09-25 sebagai commit lokal `7078835` dengan suite hermetik dan lint gate hijau serta live pass di port 9091, sisa kerja di §23.2 (§23) |
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

## 15. Cek dua model `oczen` atas permintaan owner: node itu tidak punya endpoint

Owner meminta pengecekan `oczen/mimo-v2.6-flash-free` dan `oczen/muse-spark-1.3-contributor-free`
(2026-09-23 23:14). Keduanya menjawab **503 `NO_PROVIDER_AVAILABLE`** dalam ~20 ms dengan pesan "no upstream
endpoint is configured for provider openai-compatible-03863XJ2YM2KHF847XJP5YBSH8": prefiks `oczen/` resolve
ke node yang benar (bukan `MODEL_NOT_FOUND`), tetapi node `oczen` ("OpenCode Zen Free",
`https://opencode.ai/zen/v1`, `api_type responses`) tidak punya satu baris pun di `upstream_endpoints`;
tiga endpoint yang ada semuanya milik `th-1`. Bentuk id node menjawab sama, dan provider bawaan `opencode`
untuk model yang sama juga 503 ("no upstream endpoint is configured for provider opencode").

| Uji (23:14) | Hasil terukur |
| --- | --- |
| `oczen/mimo-v2.6-flash-free` | 503 dalam 18 ms; baris `usg_0386F1FT3BHY5Y6RV7HSAG98P0` dan `0386F1FT2QQ28VJRF1PMB1CXMJ` |
| `oczen/muse-spark-1.3-contributor-free` | 503 dalam 25 ms; baris `usg_0386F1FT769PTA15D9B7EYQ2VM` dan `0386F1FT640GXSKX7JFH54XDW6` |
| Bentuk id node, `mimo-v2.6-flash-free` | 503 dalam 34 ms; baris `usg_0386F1G54D0RT2D9MS1HYX6CTG` dan `0386F1G530K9H5RPHEDCM7KER4` |
| Provider bawaan `opencode`, `muse-spark-1.3-contributor-free` | 503 dalam 32 ms; baris `usg_0386F1GK6M5HG7F5Z4M38NMFHA` dan `0386F1GK54663HQ87HTHD2QYM0` |
| Katalog saat diukur | 367 id: 80 di bawah id node `oczen`, 3 di bawah `opencode`, dan 0 di bawah prefiks `oczen` |

Keempat penolakan tercatat jujur (`status` `error`, `latency_ms` 0, token 0), jadi yang hilang bukan
pencatatan melainkan kredensial: tanpa endpoint dan tanpa kunci opencode.ai Zen tidak ada yang bisa
dirutekan, sama seperti §9. Itu pekerjaan owner di panel, bukan cacat gateway. Catatan keadaan:
`provider_nodes` sekarang 2 baris (`oczen`, `th-1`); node `occap` ("Zen Capture") yang masih ada pada §13
sudah tidak ada lagi. Tidak ada kode yang disentuh pass ini.

## 16. Uji empat model 2026-09-24 08:43: satu model menjawab, tiga menunggu koneksi `oczen`

Owner meminta pengujian endpoint dengan empat model sekaligus (kunci `sandbox` yang sama, hint `sk-…rmEu`):
`th-1/deepseek-v4.1-flash:free`, `space-bunny-free`, `mimo-v2.6-flash-free`, dan
`muse-spark-1.3-contributor-free`. Gateway yang diukur adalah proses yang baru di-restart owner: PID
`2716056` mulai 08:38:23 (biner dari cache build Go), menggantikan PID `2214789` di §14. Baseline saat
mulai (08:42): `usage_records` 36, `request_logs` 45, `gateway_keys` 16 dengan `sandbox` `request_count`
23, `upstream_endpoints` 3 (semuanya milik `th-1`, label `Key 1`/`Key 2`/`Key 3`, `priority` 1,
`status` `active`), `upstream_keys` 3, `provider_nodes` 2 (`oczen`, `th-1`), `settings` 1, `quota_caps`
dan `combos` 0; katalog `GET /api/v1/models` menjawab 363 id.

| Model dipaste apa adanya | Hasil terukur |
| --- | --- |
| `th-1/deepseek-v4.1-flash:free` | HTTP 200 dalam 37,4 s; `content` `pong`, `reasoning_content` ada, `finish_reason` `stop`, `usage` 36/37/73; `model` di wire `deepseek-v4.1-flash` |
| `space-bunny-free` | HTTP 400 dalam <0,1 s: `MODEL_NOT_FOUND`, "model space-bunny-free is not a known model, alias, or combo" |
| `mimo-v2.6-flash-free` | HTTP 400, `MODEL_NOT_FOUND` |
| `muse-spark-1.3-contributor-free` | HTTP 400, `MODEL_NOT_FOUND` |

Ketiga nama telanjang itu ada di katalog, tetapi hanya dalam bentuk terkuantifikasi. Tujuh panggilan
lanjutan memakai bentuk yang resolve, dan semuanya menjawab 503 `NO_PROVIDER_AVAILABLE` dalam ~0,0 sampai
0,3 s karena node `oczen` tidak punya satu baris pun di `upstream_endpoints`:

| Bentuk yang resolve | Provider yang disebut pesan | Hasil |
| --- | --- | --- |
| `openai-compatible-03863XJ2YM2KHF847XJP5YBSH8/{space-bunny-free, mimo-v2.6-flash-free, muse-spark-1.3-contributor-free}` | `openai-compatible-03863XJ2YM2KHF847XJP5YBSH8` | 503, tiga panggilan |
| `oczen/{space-bunny-free, mimo-v2.6-flash-free, muse-spark-1.3-contributor-free}` | id node yang sama | 503, tiga panggilan |
| `opencode/muse-spark-1.3-contributor-free` | `opencode` | 503, satu panggilan |

Jadi prefiks `oczen/` dan id node sama-sama resolve ke node yang benar (sama seperti §15), dan
satu-satunya yang hilang adalah koneksi OpenCode Zen untuk node itu. Itu pekerjaan owner di panel; sampai
koneksi itu ada, ketiga model tidak bisa dirutekan, dan setelah ada pun nama telanjang tetap 400 karena
ref yang sah adalah `<prefiks>/<model>`, id katalog, alias, atau combo.

### 16.1 Stream pada `th-1`: F1 sampai F3 dan F5 tetap, bentuk byte sama

| Uji stream (08:44) | Hasil terukur |
| --- | --- |
| Tanpa `include_usage` | HTTP 200, TTFB 0,72 s, total 0,73 s, 1074 byte: 4 frame berframe (role+content, content, `finish`, `usage` dari upstream), lalu satu frame `finish` **telanjang** dengan `data: [DONE]` menempel; `"finish_reason":"stop"` dua kali; 0 kecocokan `^data: \[DONE\]$` |
| Dengan `include_usage` | HTTP 200, TTFB 30,3 s, total 30,3 s, 1316 byte: 4 frame berframe, lalu satu baris telanjang berisi **dua** objek `usage` kembar (36/37/73) dengan `data: [DONE]` menempel |
| Pembaca SSE panel atas kedua byte | `frames=4` di keduanya, sentinel tidak pernah menjadi frame sendiri, jawaban terlipat benar (`pong`, `deepseek-v4.1-flash`, `stop`, 36/37/73), alasan akhir `truncated` |

F5 juga tetap: kedua stream menulis `usage_records` 0/0 dengan `status` `success` (`latency_ms` 708 dan
30299) padahal wire membawa 36/37/73, sementara panggilan non-streaming 25 detik sebelumnya mencatat 36/37
dengan benar (`latency_ms` 37295). Yang bergerak dari §14 hanya latensi upstream (non-streaming 72,4 s
menjadi 37,4 s; TTFB stream 51,8 s dan 101,4 s menjadi 0,72 s dan 30,3 s).

### 16.2 Pencatatan dan keadaan akhir

Tiga belas panggilan pass ini semuanya terhitung di `gateway_keys.request_count` kunci `sandbox` (23
menjadi 36): empat panggilan pertama, tujuh penolakan 503, dan dua stream. `request_logs` bertambah 13
baris (tiga `MODEL_NOT_FOUND` dengan kolom provider dan model kosong, tujuh `NO_PROVIDER_AVAILABLE`, satu
non-streaming `success`, dua streaming `success`), dan `usage_records` bertambah 10 baris (tiga
`MODEL_NOT_FOUND` tidak menulis baris usage, sesuai batas yang sudah tercatat di §13). Baris-baris itu
**sengaja ditinggalkan** sebagai rekaman, seperti putaran sebelumnya. Satu catatan pengukuran: katalog
manajemen yang dipakai pemilih panel (`GET /api/v1/models/catalog?provider_id=<id node oczen>`) menjawab
82 baris dengan id berbentuk `<id node>/<model>`, jadi panel menawarkan ref yang terkuantifikasi; nama
telanjang yang dipaste owner adalah id milik node itu sendiri (daftar `/models`-nya), dan satu di
antaranya bahkan muncul sudah berprefiks (`oczen/muse-spark-1.3-contributor-free`), bersama
saudara-saudaranya `muse-spark-1.2`, `muse-spark-1.2-contributor-free`, dan `muse-spark-1.3`.

Berkas bukti: `/tmp/smoke6/*.json` (tiga belas badan jawaban), `/tmp/smoke6/stream-plain.txt` dan
`stream-usage.txt`, `/tmp/smoke6_run.py`, dan `/tmp/smoke6_panel_reader.ts`.

## 17. Uji ulang empat model 2026-09-24 09:12: identik dengan §16, F1-F3 dan F5 tetap

Owner mengirim ulang endpoint, kunci, dan empat model yang sama (kunci `sandbox`, hint `sk-…rmEu`):
`th-1/deepseek-v4.1-flash:free`, `space-bunny-free`, `mimo-v2.6-flash-free`, dan
`muse-spark-1.3-contributor-free`. Gateway tidak berubah sejak §16: PID `2716056` (mulai 08:38:23,
biner cache build Go), dan keadaan tabel saat mulai (09:12) identik dengan baseline §16:
`upstream_endpoints` 3 (semuanya milik `th-1`), `upstream_keys` 3, `provider_nodes` 2 (`oczen`, `th-1`),
`usage_records` 46, `request_logs` 58, `request_count` kunci `sandbox` 36. Tidak ada baris baru dari
aktor lain antara 08:47 dan 09:12. Katalog menjawab 364 id: 0 berawalan `th-1/`, 80 milik node `oczen`
di bawah id node, 3 di bawah `opencode`; satu id baru sejak §16, `pi-agent` (owned_by `combo`).

Empat model dipaste apa adanya (non-streaming, `max_tokens` 64, instruksi "Balas satu kata saja: pong"):

| Model dipaste | Hasil terukur |
| --- | --- |
| `th-1/deepseek-v4.1-flash:free` | HTTP 200 dalam 55,5 s; `content` `pong`, `reasoning_content` ada, `finish_reason` `stop`, `usage` 38/90/128; `model` di wire `deepseek-v4.1-flash` |
| `space-bunny-free` | HTTP 400 dalam 0,02 s: `MODEL_NOT_FOUND`, "model space-bunny-free is not a known model, alias, or combo" |
| `mimo-v2.6-flash-free` | HTTP 400, `MODEL_NOT_FOUND` |
| `muse-spark-1.3-contributor-free` | HTTP 400, `MODEL_NOT_FOUND` |

Tujuh panggilan lanjutan memakai bentuk yang resolve; semuanya menjawab 503 `NO_PROVIDER_AVAILABLE`
dalam 0,01-0,67 s, tetap karena node `oczen` tidak punya satu baris pun di `upstream_endpoints`:

| Bentuk yang resolve | Hasil |
| --- | --- |
| `oczen/{space-bunny-free, mimo-v2.6-flash-free, muse-spark-1.3-contributor-free}` | 503, tiga panggilan |
| `openai-compatible-03863XJ2YM2KHF847XJP5YBSH8/{ketiganya}` | 503, tiga panggilan |
| `opencode/muse-spark-1.3-contributor-free` | 503, satu panggilan |

### 17.1 Stream pada `th-1`: F1 sampai F3 dan F5 tetap, bentuk byte sama

Dua stream diukur; tangkapan pertama rusak karena skrip (satu byte hilang di awal), lalu ditangkap ulang
bersih dan diulang analisisnya atas byte yang baru.

| Uji stream (09:24) | Hasil terukur |
| --- | --- |
| Tanpa `include_usage` | HTTP 200, total 3,1 s, 4337 byte: 18 frame berframe (role, 15 delta `content`, `finish`, `usage` dari upstream), lalu satu frame `finish` **telanjang** dengan `data: [DONE]` menempel; `"finish_reason":"stop"` dua kali; 0 kecocokan `^data: \[DONE\]$` |
| Dengan `include_usage` | HTTP 200, total 35,5 s, 1316 byte: 4 frame berframe, lalu satu baris telanjang berisi **dua** objek `usage` kembar (38/48/86) dengan `data: [DONE]` menempel; 0 kecocokan `^data: \[DONE\]$` |
| Pembaca SSE panel atas kedua byte | `frames=18` dan `frames=4`, sentinel tidak pernah menjadi frame sendiri, jawaban terlipat benar (`pong`, `deepseek-v4.1-flash`, `stop`, 38/48/86), alasan akhir `truncated` |

F5 juga tetap: kedua stream menulis `usage_records` 0/0 dengan `status` `success` (`latency_ms` 2776 dan
34648) padahal wire membawa 38/48/86, sementara panggilan non-streaming 09:17 mencatat 38/90 dengan benar
(`latency_ms` 54822, `cost_usd` 0,00003052). Selisih kecil §13.2 juga terulang: kolom `model` di
`usage_records` menyimpan `deepseek-v4.1-flash:free`, sedangkan `model` di wire `deepseek-v4.1-flash`.

### 17.2 Pencatatan dan keadaan akhir

Pass ini mengirim 24 panggilan chat dan 6 `GET /api/v1/models` (5 pengukuran katalog, 1 verifikasi
langsung bahwa `GET /models` menaikkan counter +1 per panggilan). Semua 30 terhitung di
`gateway_keys.request_count` kunci `sandbox` (36 menjadi 66). Rincian 24 chat itu: 4 bentuk id node dari
loop curl pertama (503 semua), 3 nama telanjang loop kedua (400) plus satu `th-1/…` yang diputus klien
karena batas `-m 60`, satu probe `curl -w` pada `th-1/…` yang juga diputus klien (`-m 90`), 4 nama
telanjang dari `run.py`, 7 bentuk resolve dari `run2.py`, dan 2+2 stream dari `run3.py`/`run4.py`. Kedua
panggilan yang diputus klien itu tidak meninggalkan baris akuntansi, bentuk yang sama dengan F6 (§13.1),
tetapi seperti §14, karena pemutusnya klien, kejadian itu tidak dihitung sebagai bukti F6. Kelima panggilan `th-1/…` yang selesai (satu non-streaming dari
`run.py`, dua stream `run3.py`, dua stream `run4.py`) semuanya menjawab 200; keempat stream menulis baris
`usage_records` `success` 0/0.

`request_logs` bertambah 22 baris (24 chat − 2 yang diputus): 11 `NO_PROVIDER_AVAILABLE` (4 loop pertama
+ 7 `run2.py`), 6 `MODEL_NOT_FOUND` (3 loop kedua + 3 `run.py`), 1 `success` non-streaming, dan 4
`success` streaming. `usage_records` bertambah 16 baris: 11 `NO_PROVIDER_AVAILABLE` 0/0, 1 `success`
non-streaming 38/90 (`latency_ms` 54822, `cost_usd` 0,00003052), dan 4 `success` streaming 0/0 (F5,
`latency_ms` 70338/50443/2776/34648); keenam `MODEL_NOT_FOUND` tidak menulis baris usage, sesuai §13.
Catatan pembacaan log: baris 09:12:37 mencatat `deepseek-v4.1-flash` **telanjang** padahal panggilannya
memakai bentuk id node (router mencatat nama model tanpa prefiks), dan 503-nya mengarah ke node `oczen`;
itu menegaskan temuan §16 bahwa hanya bentuk `<prefiks>/<model>` (di sini `th-1/…`) yang mencapai
tokenharbor. Baris-baris itu **sengaja ditinggalkan** sebagai rekaman, seperti putaran sebelumnya.

Berkas bukti: `/tmp/smoke7/` (`run.py`, `run2.py`, `run3.py`, `run4.py`, `models.json`,
`th-1_deepseek-v4.1-flash_free.json`, `stream-plain.txt`, `stream-usage.txt`, `panel_reader.ts`).

## 18. Uji combo `pi-agent` pada gateway dengan rotasi per provider (2026-09-24 15:45-16:09)

Paste owner kali ini sama dengan §16/§17 (endpoint, kunci `sandbox` hint `sk-…rmEu`, empat model)
ditambah satu elemen baru: sebuah combo. Gateway sudah di-restart owner pukul 15:36 (PID `3053853`,
`go run ./cmd/app-serv`, biner `/tmp/go-build3328289290/b001/exe/app-serv`), dan dua state berubah
sejak §17: baris `settings` `routing` ditulis pukul 15:39:28 dengan override per provider untuk node
`th-1` (`fallback_strategy: round-robin`, `sticky_limit: 2`) di atas default global `fill-first`, dan
combo `pi-agent` (`cmb_0386H365H7JMRMQ7Q3D1TQGD7W`, `round_robin`, `combo_sticky_limit` 1) diperbarui
15:38:21 dengan dua anggota: `openai-compatible-…51G/deepseek-v4.1-flash:free` (prioritas 0) dan
`oczen/space-bunny-free` (prioritas 1). Ketiga endpoint `th-1` tetap `Key 1`/`Key 2`/`Key 3`
(prioritas 1, `active`, dibuat 2026-09-23 22:36), dan node `oczen` tetap tanpa endpoint.

| Uji (19 panggilan) | Hasil terukur |
| --- | --- |
| `th-1/deepseek-v4.1-flash:free` (5) | 3 menjawab 200 (857 ms; 52,5 s; stream 48,0 s) dan **2 menggantung tanpa satu byte pun** (dipotong klien di 300 s dan 90 s) |
| tiga nama telanjang (3) | 400 `MODEL_NOT_FOUND` dalam < 0,1 s, sama seperti §16 |
| `oczen/<ketiganya>` dan bentuk id node (6) | 503 `NO_PROVIDER_AVAILABLE`, node `oczen` masih tanpa endpoint |
| `pi-agent` (5) | 3 menjawab 200 (2,8 s dan 0,9 s non-streaming, usage 38/55/93, `combo=pi-agent` tercatat; 1 stream 71,0 s) dan **2 menggantung** (dipotong 300 s dan 120 s) |

### 18.1 Rotasi per provider bekerja, dan setiap panggilan teratribusi

Kesepuluh panggilan yang mencapai `th-1` menggerakkan kursor round-robin persis seperti
`sticky_limit` 2 mengharuskan, dan setiap hasil cocok dengan offset yang dipakai. State awal
`(offset 0, used 2)` adalah sisa §17 yang berjalan pada default global `sticky_limit` 3 (override
baru ditulis 15:39, setelah §17), dan urutannya:

| # | Panggilan | Kursor sebelum | Offset | Endpoint | Hasil |
| --- | --- | --- | --- | --- | --- |
| 1 | `th-1/…` non-stream | (0, 2) | 1 | Key 2 | menggantung, klien memotong di 300 s |
| 2 | `pi-agent` c1 | (1, 1) | 1 | Key 2 | menggantung, klien memotong di 300 s |
| 3 | `pi-agent` c2 | (1, 2) | 2 | Key 3 | 200, 1,5 s, 38/55 |
| 4 | `pi-agent` c3 | (2, 1) | 2 | Key 3 | 200, 0,9 s, 38/55 |
| 5 | `th-1/…` non-stream | (2, 2) | 0 | Key 1 | 200, 857 ms, 38/55 |
| 6 | `th-1/…` non-stream | (0, 1) | 0 | Key 1 | 200, 52,5 s, 38/55 |
| 7 | `th-1/…` non-stream | (0, 2) | 1 | Key 2 | menggantung, klien memotong di 90 s; marker `usage:active` menyebut `ep_0386EX3Z1D8JYP3XAHJAG8KP0A` |
| 8 | `pi-agent` stream | (1, 1) | 1 | Key 2 | menggantung, klien memotong di 120 s |
| 9 | `pi-agent` stream+usage | (1, 2) | 2 | Key 3 | 200, TTFB 61,6 s, total 71,0 s |
| 10 | `th-1/…` stream+usage | (2, 1) | 2 | Key 3 | 200, TTFB 48,0 s, total 48,0 s |

Kolom endpoint untuk baris 3, 4, 5, 6, 9, dan 10 datang dari `usage_records.endpoint_id`; baris 7
dari marker `pannelai:usage:active` yang ditulis gateway saat request dimulai (MONITOR); baris 1, 2,
dan 8 dari state kursor (offset 1) plus tidak adanya baris akuntansi apa pun. Satu koreksi metode
yang dicatat: pembacaan kunci Redis harus memakai `${VAR}:suffix`, karena `$VAR:suffix` di zsh
menerapkan modifier `:u` (uppercase) dan menghasilkan kunci lain (`ABCsed`), yang sempat membuat
pembacaan `:used` terlihat kosong padahal sehat.

### 18.2 F7 (HIGH): kredensial yang menggantung membekukan request, tanpa timeout, tanpa failover, tanpa baris

`Key 2` (`ep_0386EX3Z1D8JYP3XAHJAG8KP0A`, hint kunci upstream `0UQv`) **menggantung setiap panggilan
yang memakainya**: empat panggilan (tiga non-streaming, satu stream) menerima **nol byte** dan
dipotong klien pada 300 s, 300 s, 90 s, dan 120 s. Baris 7 di atas membuktikan atribusinya lewat
marker gateway sendiri. Kredensial ini belum pernah menyelesaikan satu panggilan pun sejak endpoint
dibuat (2026-09-23 22:36): `usage_records` 0 baris dan `request_logs` 0 baris untuk endpoint itu,
sementara `Key 1` dan `Key 3` masing-masing 11 dan 12 baris.

Yang membuat ini kelasnya berbeda dari F6: tidak ada satu pun jalur yang memberi sinyal. Klien tidak
menerima timeout (tidak ada 504 `UPSTREAM_TIMEOUT`); baris endpoint tetap `status active` dengan
`last_error` kosong dan `consecutive_use_count` 0; dan tidak ada baris akuntansi. Padahal kode
menyediakan ketiganya: `ResponseHeaderTimeout` `TotalTimeout` 120 s
(`internal/dataplane/transport.go:42,131`), retry POST non-idempoten dibatasi satu (`retry.go:40`),
dan timeout memang layak failover (`engine_failure.go:116-122`), sehingga rantai seharusnya berakhir
pada kredensial berikutnya dalam sekitar 2,5 menit. Yang terukur: klien menggantung melewati 300 s.
Mekanisme (sisi upstream yang tidak menjawab, atau jalur gateway yang tidak menerapkan deadline pada
kasus ini) belum diverifikasi; yang terukur hanya gejalanya, dan setiap panggilan yang menggantung
sengaja dipotong klien sehingga tidak ada bukti bahwa panggilan itu akan berakhir sendiri.

### 18.3 Stream: F1 sampai F3 dan F5 tetap, pembaca panel tetap `truncated`

Dua stream `include_usage` yang selesai membawa bentuk byte yang sama dengan §17: 29 frame berframe
(stream combo) dan 4 frame berframe (stream `th-1`), lalu satu baris **telanjang** 448 byte berisi
dua chunk gabungan (finish sintetis dan chunk usage) dengan `data: [DONE]` menempel di ujungnya, dan
0 kecocokan `^data: \[DONE\]$`. Wire membawa usage 38/79/117. Pembaca SSE panel (`readSseData` plus
`readStream`) melipat jawaban dengan benar (`pong`, `deepseek-v4.1-flash`, `stop`, 38/79/117) dan
tetap melaporkan alasan akhir `truncated`. F5 juga tetap: kedua stream menulis `usage_records` 0/0
dengan `status` `success` (`latency_ms` 69484 dan 47658), sementara empat panggilan non-streaming
mencatat 38/55 dengan benar.

### 18.4 Pencatatan dan keadaan akhir

Pass ini mengirim 19 panggilan chat: 13 dari driver non-streaming, 3 probe, dan 3 stream. Semuanya
terhitung di `gateway_keys.request_count` kunci `sandbox` (66 menjadi 85), termasuk empat panggilan
yang menggantung dan tidak menulis baris apa pun. `request_logs` bertambah 15 baris (9 `error`: 3
`MODEL_NOT_FOUND` dan 6 `NO_PROVIDER_AVAILABLE`; 6 `success`), `usage_records` bertambah 6 baris (4
non-streaming 38/55 dan 2 stream 0/0). Baris-baris itu **sengaja ditinggalkan** sebagai rekaman,
seperti putaran sebelumnya. Satu pengamatan tambahan: setelah enam panggilan sukses, ketiga baris
endpoint tetap `consecutive_use_count` 0 dan `last_used_at` NULL, karena tidak ada pemanggil produksi
untuk `RecordUse`/`RecordUpstreamSuccess` (grep; state rotasi hidup di Redis,
`pannelai:cursor:endpoint:<sha256(provider)>` dengan pasangan offset dan `:used`, TTL 24 jam), jadi
panel tidak bisa menampilkan rotasi per kredensial dari kolom itu.

Berkas bukti: `/tmp/smoke8/` (`run.env` mode 600, `run.py`, `stream.sh`, `panel_reader.ts`,
`analyze.py`, `run.log`, `resp/` 11 tangkapan, `probe1.json`, `probe2.json`, `probe3.json` (kosong),
`monitor.log`, `monitor2.log`, `used_watch.log`, `stream-combo-plain.txt` (kosong),
`stream-combo-usage.txt`, `stream-th1-usage.txt`).

## 19. Uji request bersih setelah tabel usage dikosongkan (2026-09-24 16:33)

Atas permintaan owner, `usage_records` dikosongkan lebih dulu (68 baris; backup
`/tmp/usage_records-backup-20260924-163022.sql`), lalu lima request dikirim persis seperti bentuk
panggilan klien: satu request non-streaming per ref, tanpa probe dan tanpa instrumentasi tambahan,
hanya empat model yang disebut plus combo `pi-agent`. Gateway yang melayani tetap PID `3053853`
(start 15:36), dan baris `settings` `routing` tidak berubah (override `th-1` `round-robin`
`sticky_limit` 2).

| # | Ref (persis seperti diminta) | Respons app-serv |
| --- | --- | --- |
| 1 | `th-1/deepseek-v4.1-flash:free` | 200, 16,56 s; `model` `deepseek-v4.1-flash`, `finish` `stop`, `content` `pong`, usage 38/69/107 |
| 2 | `space-bunny-free` | 400 `MODEL_NOT_FOUND` ("is not a known model, alias, or combo") dalam 0,01 s |
| 3 | `mimo-v2.6-flash-free` | 400 `MODEL_NOT_FOUND` dalam 0,01 s |
| 4 | `muse-spark-1.3-contributor-free` | 400 `MODEL_NOT_FOUND` dalam 0,01 s |
| 5 | `pi-agent` | 200, 1,27 s; `model` `deepseek-v4.1-flash`, `finish` `stop`, `content` `pong`, usage 38/69/107 |

### 19.1 Jejak akuntansi yang mulai dari nol

`usage_records` (0 baris sebelum uji) menerima tepat 2 baris, keduanya endpoint
`ep_0386EX3Z19198FTDJK4TKE97RM` (Key 1): 38/69 dengan `latency_ms` 16503 dan 910, dan baris kedua
membawa `combo=pi-agent`. `request_logs` bertambah 5 baris (2 `success`, 3 `MODEL_NOT_FOUND`;
baris terakhir sebelum uji 16:09:15), dan `gateway_keys.request_count` kunci `sandbox` 85 menjadi
90. Kursor rotasi berakhir `(offset 0, used 2)` dan counter combo `pi-agent` 5 menjadi 6, konsisten
dengan dua seleksi kredensial yang keduanya jatuh ke offset 0. Walk ronde ini tidak pernah mencapai
`Key 2`, jadi F7 tidak terpicu ulang; itu bukan bukti F7 hilang.

Berkas bukti: `/tmp/smoke9/req.sh`, `/tmp/smoke9/req.log`, `/tmp/smoke9/req/01.json` sampai
`05.json`.

## 20. Cek filtering combo dan provider aktif di penanganan models (2026-09-24)

Permintaan owner: "Cek pada filtering `Combo` dalam penangan models. dan seharusnya filtering bekerja pada
provider yang memamng aktif seperti REFERENCE". Pass ini mengukur keempat permukaan yang menjawab "model
apa yang bisa dirutekan" di kedua pohon dan tidak menyentuh kode mana pun.

### 20.1 Dua aturan REFERENCE yang berbeda, dan padanannya di pohon ini

REFERENCE memakai dua predikat yang tidak sama di dua permukaan: daftar klien hanya menyebut provider yang
punya koneksi `isActive !== false` (`src/app/api/v1/models/route.js:269`, `:303-306`, `:369`), sementara
picker-nya menawarkan provider yang punya baris koneksi apa pun statusnya, karena `/api/providers`
mengembalikan seluruh koneksi (`combos/page.js:170`; `connectionsRepo.js:92-103` tanpa filter `isActive`).

| Permukaan | REFERENCE | Pohon ini |
| --- | --- | --- |
| Daftar model klien (`GET /api/v1/models`) | hanya provider dengan ≥1 koneksi aktif (`route.js:269`, `:369`); model = live/enabled/statis digabung model custom provider itu (`:447`, `:487`); combo tanpa syarat (`:316`) | seluruh provider chat-routable plus seluruh node (fetch live, tanpa syarat endpoint) plus combo tanpa syarat (`internal/dataplane/catalog.go:49`, `:68`) |
| Picker combo (`/combos` ke editor) | combo di-fetch sendiri saat dibuka, seluruhnya (`ModelSelectModal.js:120-133`); filter combo hanya search (`:426-429`) | combo = halaman tabel saat ini saja (`CombosTab.svelte:55`); filter combo hanya search (`model-picker.ts:129-146`) |
| Picker vision (`capability=vision`) | combo disembunyikan saat `capFilter` (`ModelSelectModal.js:427`) | `combos: []` (`VisionAdapterForm.svelte:57`) |
| Katalog manajemen (`GET /models/catalog`) | tidak ada padanannya | `?active=true` menyempit ke provider dengan endpoint `status='active'` (`model_catalog.go:120`, `model_catalog_active.go:74-83`) |

### 20.2 Yang sudah setara, jadi tidak ada yang perlu diperbaiki

- Combo tidak pernah difilter berdasarkan aktivitas provider, di kedua pohon: daftar klien menyebut combo
  tanpa syarat (`route.js:316` berpasangan dengan `catalog.go:68`), dan picker combo juga tidak
  (`ModelSelectModal.js:426-429` berpasangan dengan `model-picker.ts:81-88`). Kalau harapannya "combo ikut
  difilter ke provider aktif", REFERENCE justru tidak begitu.
- Predikat picker panel (`endpoint_count > 0`, `model-picker.ts:46`) sama dengan aturan picker REFERENCE
  (punya baris koneksi), bukan aturan daftar kliennya. `endpoint_count` sendiri adalah `summary.Total`
  (`schema/provider.go:136`), jumlah semua status.
- Katalog manajemen: `?active=true` bekerja; diukur 587 baris (67 provider) menjadi 1 baris, hanya
  `openai-compatible-0386BKG9Q4DYZPYC01VJH4C51G/deepseek-v4.1-flash:free`. Panel tidak pernah mengirim
  parameter ini (`schemaCatalogQuery` tanpa field `active`, `model.ts:46-50`).

### 20.3 F8 (MEDIUM): daftar model klien tidak punya filter provider aktif

Ini bentuk konkret dari F4 (§10), dan pengukuran hari ini menunjukkan kedua sisinya sekaligus:

| Pengukuran 2026-09-24 | Hasil |
| --- | --- |
| `GET /api/v1/models` dengan kunci gateway | 200, 364 baris, 44 owner |
| Baris dari node `OpenCode Zen Free` (`openai-compatible-03863XJ2YM2KHF847XJP5YBSH8`, `endpoint_count` 0) | 80 baris (fetch live upstream berhasil, 80 model) |
| Baris dari `TH HARBOR 1` (3 endpoint, semuanya `status='active'`) | 0 baris (route model node menjawab `{"data":[],"source":"registry","warning":"the upstream answered 401"}`) |
| Combo `pi-agent` | ada, satu baris, tanpa syarat |

Jadi daftar yang dibaca klien, dan layar `/playground` yang membaca rute ini apa adanya lewat
`playground/models/+server.ts`, menyebut 80 id yang tidak bisa dirutekan, sementara model yang terbukti
bekerja di §19 (`th-1/deepseek-v4.1-flash:free`) tidak ada di daftar itu.

### 20.4 F9 (LOW): picker combo editor hanya menawarkan combo halaman tabel yang sedang dibuka

`CombosTab.svelte:55` membangun seksi combo dari `combos` halaman itu (`listCombos({page, per_page: 25})`),
jadi combo di halaman lain tidak bisa dipilih sebagai anggota. REFERENCE tidak begitu: `ModelSelectModal`
mengambil `/api/combos` sendiri setiap kali dibuka dan menampilkan seluruhnya (`ModelSelectModal.js:120-133`).
Dengan satu combo hidup, selisih ini belum terlihat di layar.

### 20.5 F10 (MEDIUM): model custom provider aktif tidak pernah masuk daftar klien

`ModelList` hanya membaca `Provider.Models` dari indeks (`catalog.go:53`), dan indeks node diisi dari fetch
live (`cmd/app-serv/provider_index.go:143`); `models_custom` tidak pernah dibaca rute ini. REFERENCE
menggabungkannya: `mergedModelIds = new Set([...modelIds, ...customModelIds, ...aliasModelIds])`
(`route.js:487`) lalu mendorong setiap id (`:489`). Terukur: 5 baris `models_custom` ada (4 di node oczen, 1
di `th-1` dengan `model_id` `deepseek-v4.1-flash:free`), dan yang `th-1` itu tidak muncul di 364 baris daftar
klien.

F8 dan F10 saling mengunci: menerapkan filter aktif saja akan menyisakan 1 baris (combo `pi-agent`) dan tetap
menyembunyikan model yang bekerja; menerapkan penggabungan model custom saja akan menyembunyikan 80 id mati
tapi tetap menyebut ratusan id tanpa endpoint. Bentuk yang setara REFERENCE hari ini adalah 2 baris:
`deepseek-v4.1-flash:free` dan `pi-agent`.

### 20.6 Bukti

Berkas: `/tmp/combochk/dp-models.json` (daftar klien), `cat-all.json` dan `cat-active.json` (katalog),
`providers.json` (roll-up endpoint), `node-models-th1.json` dan `node-models-oczen.json` (route model node).
Tidak ada kode yang disentuh pass ini.

## 21. Uji chat, streaming, dan tool atas daftar model owner (2026-09-24 20:28-20:38)

Owner menempel endpoint, kunci, dan daftar model, meminta tiga mode uji (chat, streaming, tool) dan
melarang menyentuh model lain. Gateway yang melayani baru di-restart 20:20:26 (`go run ./cmd/app-serv`,
PID 3477809). Kunci yang ditempel adalah satu-satunya baris `gateway_keys` (`sandbox`, hint `sk-…rmEu`,
`request_count` 90 menjadi 117 sepanjang ronde).Ralat owner di tengah ronde: nama ketiga yang benar adalah
`oczen/muse-spark-1.3-contributor-free`; versi prefiks-ganda yang tertulis lebih dulu tetap diuji dan
hasilnya identik.

| Model | Non-stream | Stream | Tool (`get_weather`, Jakarta) |
| --- | --- | --- | --- |
| `th-1/deepseek-v4.1-flash:free` | percobaan-1 hang 60 s (0 byte); retry 200, 1,09 s, `pong`, 36/38/74 | 200, 1,29 s, 1105 byte, usage frame 36/38/74, bingkai akhir tanpa `data: ` | percobaan-1 hang 90 s (0 byte); retry 200, 3,43 s, `tool_calls` `get_weather({"city":"Jakarta"})`, `finish_reason` `tool_calls`, 305/54/359 |
| `oczen/space-bunny-free` | 503 `NO_PROVIDER_AVAILABLE` 0,48 s | 503 0,03 s | 503 0,03 s |
| `oczen/muse-spark-1.3-contributor-free` (ralat) | 503 0,02 s (prefiks-ganda juga 503) | 503 0,01 s | 503 0,01 s |
| `oczen/mimo-v2.6-flash-free` | 503 0,02 s | 503 0,01 s | 503 0,02 s |
| `pi-agent` (combo) | 200, 1,59 s, `pong` + `reasoning_content`, 36/38/74 | 200, 0,84 s, usage frame 36/38/74, bingkai akhir tanpa `data: ` | percobaan-1 hang 90 s (0 byte); retry 200, 2,93 s, `tool_calls` `get_weather({"city": "Jakarta"})`, 305/52/357 |

Pesan 503 persis sama untuk keempat nama `oczen`: "no upstream endpoint is configured for provider
`openai-compatible-03863XJ2YM2KHF847XJP5YBSH8`". Kontrol tambahan (tools terpasang, prompt tidak
memicu): 200, 1,37 s, `pong`, 295/4/299.

### 21.1 Temuan

- **F1-F3 tetap hidup.** Kedua stream yang sukses mengakhiri dengan chunk JSON mentah tanpa prefiks
  `data: ` yang menempel langsung ke `data: [DONE]`; artefak `/tmp/smoke10/stream-th1.sse` (2231 byte)
  menyimpannya.
- **F5 tetap hidup, dan kini lebih tajam.** Baris akuntansi kedua stream adalah 0/0, padahal gateway
  sendiri yang mengirim usage frame 36/38/74 di kawat - data yang dibutuhkan sudah lewat tangannya.
- **Pola hang-tanpa-baris tidak lagi bisa disematkan ke kunci tertentu (perbaikan atas F7).** Lima dari
  23 panggilan hidup menggantung tanpa timeout gateway (60-90 s, 0 byte): `th-1` non-stream percobaan-1,
  `th-1` tool percobaan-1, `pi-agent` tool percobaan-1, dan `pi-agent` stream dua kali berturut-turut.
  Semua yang di-retry menjawab dalam 3,4 s. Dengan kunci `sandbox` sebagai satu-satunya kunci, hang itu
  milik jalur gateway→upstream yang tidak punya timeout (berlawanan dengan AGENTS.md §1.6: setiap
  panggilan keluar wajib membawa `context` timeout).
- **Panggilan yang menggantung tak berjejak.** `request_logs` ronde ini berisi 18 baris (10 `error`
  `NO_PROVIDER_AVAILABLE`, 8 `success`) - persis panggilan yang terjawab; lima hang tidak menulis baris
  di `request_logs` maupun `usage_records`.
- Node `oczen` masih tanpa endpoint (satu keluarga temuan F4/F8 yang sama): 503 pada ketiga mode,
  termasuk dengan nama yang sudah diralat.

### 21.2 Akuntansi

`usage_records` 2 menjadi 9 baris, tujuh baris baru semuanya `success`: 36/38 (non-stream `pi-agent`),
36/38 (non-stream `th-1`), 0/0 (stream `th-1`), 0/0 (stream `pi-agent`), 305/54 (tool `th-1`), 295/4
(kontrol), 305/52 (tool `pi-agent`). `request_logs` +18. Rincian lengkap dari kueri DB pada transkrip
ronde ini.

Bukti: `/tmp/smoke10/stream-th1.sse`; `/tmp/smoke10/stream-piagent.sse` tidak pernah tercipta karena
kedua percobaannya menggantung 0 byte. Tidak ada kode yang disentuh ronde ini.

## 22. Uji chat, streaming, dan tool atas combo `pi-agent` (2026-09-25 08:50-08:53)

Owner menempel endpoint dan satu model, combo `pi-agent`, meminta tiga mode uji (chat, streaming, tool),
dan melarang menyentuh model lain. Ronde ini hanya `pi-agent` yang dipanggil: tidak ada `GET /models`,
tidak ada model lain, tidak ada probe. Gateway yang melayani adalah build baru
(`/tmp/go-build1157667703/b001/exe/app-serv`, PID 3695333, mulai 08:43:38) dan kuncinya tetap
satu-satunya baris `gateway_keys` yang aktif (`sandbox`, hint `sk-…rmEu`, `request_count` 117 menjadi 126).

Combo berubah sejak §21: `combos.updated_at` 2026-09-25 08:48:34, `strategy` `round_robin`,
`sticky_limit` 1, tiga anggota `opencode/space-bunny-free`, `opencode/mimo-v2.6-flash-free`, dan
`opencode/muse-spark-1.3-contributor-free`. Ketiganya dilayani tanpa baris `upstream_endpoints`:
`usage_records.endpoint_id` menulis `virtual:opencode` (`VirtualEndpointIDPrefix`,
`internal/dataplane/selection_virtual.go:42`), jadi keluarga model OpenCode Free berjalan tanpa kredensial
dan tanpa endpoint tersimpan. Tabel endpoint tetap 3 baris, semuanya `th-1`, dan tidak satu pun tersentuh
ronde ini.

Sembilan panggilan, dan rotasi combo terverifikasi berpola 0,1,2 berulang (`space-bunny-free`,
`mimo-v2.6-flash-free`, `muse-spark-1.3-contributor-free`); setiap baris `usage_records` mencocokkan
anggota yang menjawab, dan counter Redis `pannelai:combo:rotation:c6c85ab8…` berakhir di 9.

| Uji | Anggota yang menjawab | Hasil |
| --- | --- | --- |
| Chat non-stream ("Balas satu kata saja: pong") | `space-bunny-free` | 200, 2,55 s, `pong` + `reasoning_content`, 414/43/457 |
| Streaming (prompt sama) | `mimo-v2.6-flash-free` | 200, TTFB 22,2 s, 4480 byte, 13 frame ber-`data: `, teks `pong`, usage kawat 130/36/166 |
| Tool `get_weather` (Jakarta), `max_tokens` 256 | `muse-spark-1.3-contributor-free` | 200, 5,84 s, `finish_reason` `length`, konten kosong, 639/256 (253 token reasoning) |
| Tool yang sama, `max_tokens` 2048 (ulang) | `space-bunny-free` | 200, 1,14 s, `tool_calls` `get_weather({"city": "Jakarta"})`, `finish_reason` `tool_calls`, 488/47/535 |
| Tool + `stream` | `mimo-v2.6-flash-free` | 200, 20,0 s, 4222 byte, `tool_calls` terakit dari delta, `finish_reason` `tool_calls` |
| `stream` + `stream_options.include_usage`, `max_tokens` 64 | `muse-spark-1.3-contributor-free` | 200, 2,56 s, 194 byte, tanpa frame konten sama sekali |
| `include_usage`, `max_tokens` 1024, tiga kali | ketiganya | 200, `pong` normal (1898, 4952, 1328 byte) |

Tidak ada panggilan yang menggantung ronde ini (0 dari 9); lima hang §21 tidak terulang.

### 22.1 Temuan: F1, F2, F3, dan F5 tetap hidup, dan mekanisme F5 kini terverifikasi

- **F1.** Frame yang dibangun gateway sendiri keluar tanpa prefiks `data: ` dan tanpa baris kosong
  penutup, lalu menempel ke byte berikutnya. Tanpa `include_usage` baris terakhir adalah
  `{...finish_reason":"stop"}data: [DONE]`; dengan `include_usage`, baris 6 `stream-03a.sse` memuat
  `{0/0/0}data: {usage asli}` (chunk gateway menempel ke frame upstream). Tidak ada satu pun baris yang
  cocok `^data: \[DONE\]$`. Jalur kodenya tetap `mustFrame` yang hanya `json.Marshal`
  (`internal/dataplane/translate_stream_openai.go:208`), sementara `Frame` yang menambahkan `data: ` dan
  `\n\n` dipakai hanya untuk payload upstream (`internal/dataplane/stream.go:34`).
- **F2.** Dengan `include_usage` usage chunk terkirim dua kali. `stream-03c.sse` memuat satu baris dengan
  dua objek usage identik (571/219/790) menempel ke `data: [DONE]`; `stream-03a.sse` memuat dua chunk
  berbeda: 0/0/0 lebih dulu, lalu 414/34/448. Mekanisme chunk nol: frame finish upstream membawa
  `"usage":null`, `objectField` mengembalikan `(nil, true)` untuk JSON `null`
  (`internal/dataplane/translate_wire.go:94`), dan `openAIUsageFromObject(nil)` mengembalikan Usage nol
  yang bukan nil (`internal/dataplane/translate_usage_read.go:36`), sehingga cabang `includeUsage` di
  `translate_stream_openai.go:179` ikut mengirim chunk sebelum angka asli tiba.
- **F3.** `finish_reason` muncul dua kali di setiap stream: sekali dari frame finish upstream, sekali dari
  chunk penutup sintetis gateway.
- **F5, mekanisme terverifikasi.** Keenam baris stream adalah 0/0 dengan `status` `success`, padahal kawat
  membawa 414/34/448, 130/34/164, dan 571/219/790. Sebabnya dua baris: `relayStream` menulis
  `outcome.Usage = state.Usage()` (`internal/dataplane/engine_stream.go:77`), lalu `readAnswer`
  mengembalikan `nil` untuk cabang stream (`internal/dataplane/engine_relay.go:184`) dan `answer` menimpa
  `outcome.Usage` dengan nil itu (`internal/dataplane/engine_relay.go:168-169`). `record` membaca
  `outcome.Usage` (`internal/service/chat_record.go:73`), jadi angka yang sudah dipegang gateway dibuang
  sebelum dicatat. Baris non-stream pada jam yang sama mencatat token dengan benar, jadi ini khusus jalur
  stream.

### 22.2 Akuntansi

`usage_records` 40 menjadi 49 (+9, semuanya `success`), `request_logs` 160 menjadi 169 (+9),
`request_count` kunci `sandbox` 117 menjadi 126 (+9). Sembilan baris baru semuanya `provider_id` `opencode`,
`endpoint_id` `virtual:opencode`, `combo` `pi-agent`: tiga non-stream mencatat token nyata (414/43,
639/256, 488/47) dan enam stream mencatat 0/0. Satu di antaranya, stream `muse-spark` dengan `max_tokens`
64, mengembalikan stream kosong (hanya chunk penutup sintetis dan `[DONE]`) tetapi tetap tercatat
`success`; anggota yang sama menjawab `pong` normal pada `max_tokens` 1024, jadi bentuk kosong itu batas
budget model reasoning, bukan kegagalan gateway.

Bukti: `/tmp/smoke11/` (`chat-01.json`, `stream-01.sse`, `tool-01.json`, `tool-02.json`,
`toolstream-01.sse`, `stream-02-usage.sse`, `stream-03a.sse`, `stream-03b.sse`, `stream-03c.sse`, plus
berkas `.headers` masing-masing). Tidak ada kode yang disentuh ronde ini.

## 23. Patch F1, F2, F3, F5, timeout fold, dan F6 (2026-09-25)

Owner memutuskan `Termasuk timeout + F6` atas pertanyaan "apakah semuanya bekerja atau perlu kita patch
kembali", jadi pass ini adalah yang pertama menyentuh kode `app-serv/` untuk register ini. Patch mendarat
sebagai commit lokal `7078835` (12 berkas, +618/−73), dan gateway owner di :9090 tidak disentuh. Enam
perubahan:

| # | Perubahan | Berkas |
| --- | --- | --- |
| F1 | Frame buatan gateway di-SSE-frame, sehingga `data: [DONE]` menjadi baris sendiri | `translate_stream_openai.go` (`Finish`), `translate_stream_openai_frames.go` (`chunk`, `usageChunk`) |
| F2 | `usage` bernilai `null` tidak lagi menjadi Usage nol, dan usage chunk dikirim paling banyak sekali (`usageSent`) | `translate_usage_read.go`, `translate_stream_openai.go` |
| F3 | `finishSent` di-set saat frame finish upstream diteruskan, jadi tidak ada frame finish kedua | `translate_stream_openai.go` |
| F5 | `answer` tidak lagi menimpa `outcome.Usage` dengan `nil` dari cabang stream | `engine_relay.go` |
| timeout | Deadline satu attempt ditentukan bentuk CLIENT, bukan bentuk upstream setelah rewrite `force_stream`, sehingga fold tidak lagi tanpa batas total | `transport_call.go`, `transport_timeouts.go` |
| F6 | Pasangan akuntansi ditulis di bawah context yang tidak ikut dibatalkan klien, berbatas 5 detik | `accounting_context.go`, `chat.go` |

Catatan desain yang terukur:

- **Cabang usage di `openAIFrames` dihapus, bukan dijaga.** Setelah F3, `finishSent` sudah true saat frame
  finish upstream diproses, jadi cabang itu tidak pernah bisa menyala lagi; `Finish()` selalu berjalan
  sesudah `pump`, sehingga satu-satunya tempat usage chunk dibangun adalah `Finish` lewat `usageChunk()`.
- **`chunk()` sengaja tetap mentah.** Jalur klien Anthropic (`claudeFrames`) dan Responses
  (`chunkFrames`) membangun eventnya sendiri dari chunk yang sama dan membingkainya di call site
  masing-masing; menambahkan `Frame` di dalam `chunk()` sempat menghasilkan `data: data: ...` dan
  tertangkap oleh `TestClaudeUpstreamToOpenAIClientStaysSingleFramed`.
- **Sisa F2 yang disengaja:** klien `include_usage` masih melihat dua objek usage, yaitu frame usage milik
  upstream yang diteruskan apa adanya dan satu usage chunk buatan gateway. Yang hilang adalah chunk kedua
  dan chunk 0/0/0.
- **Fold sekarang berbatas** 120 s (atau override `timeout_ms` provider) karena klien meminta satu body;
  klien yang meminta stream tetap tanpa batas total sesuai SPEC-API §4, dengan idle read 300 s sebagai
  batasnya.
- **Panggilan yang klien putus di tengah tercatat `UPSTREAM_TIMEOUT`** (klasifikasi lama: context yang
  dibatalkan dipetakan sebagai timeout). Barisnya ada, yang memang inti F6; labelnya tidak diubah pass ini.
- **Nomor baris di §22.1 menunjuk pohon SEBELUM patch ini.** Lokasi sesudahnya: `mustFrame` pindah ke
  `translate_stream_openai_frames.go:64`, `usageChunk` di `:48`, `chunk` di `:33`, `Finish` di
  `translate_stream_openai.go:119`, `openAIFrames` di `:147` dengan cabang usage-nya dihapus, dan
  `openAIUsageFromObject` di `translate_usage_read.go:39` yang kini mengembalikan `nil` untuk `null`.

### 23.1 Bukti

- Suite hermetik `go test -race -count=1 ./...` hijau di pohon beku (3m38s, 17 paket).
- Lint gate hijau: vet (plain dan tagged), gofmt, staticcheck (plain dan tagged), `golangci-lint` 0 issues.
- Live pass pada gateway kedua di `127.0.0.1:9091` (build dari working tree; gateway owner di :9090 tidak
  disentuh), empat panggilan `pi-agent`: chat 200, stream `include_usage` 200, tool 200 (`tool_calls`
  `get_weather({"city":"Jakarta"})`), lalu satu stream yang klien putus di 1 detik.
- Byte stream setelah patch: 9 baris, 0 baris tanpa prefiks `data: `, tepat 1 baris `data: [DONE]`,
  `finish_reason` sekali (milik upstream), dan tepat satu usage chunk gateway.
- Pembaca panel sendiri (`app-ui/src/lib/api/playground-reader.ts`) atas byte sebelum dan sesudah: capture
  ronde 11 `sentinel-as-own-frame=false end=truncated`, byte setelah patch
  `sentinel-as-own-frame=true end=done`.
- Akuntansi live: panggilan stream mencatat 130/27 dan 571/200 (bukan 0/0), dan panggilan yang klien putus
  di tengah menulis satu baris `usage_records` dan satu `request_logs` (`error`, `UPSTREAM_TIMEOUT`,
  987 ms) padahal sebelumnya tidak menulis apa pun.
- Berkas bukti: `/tmp/smoke11/live/` (`chat.json`, `stream.sse`, `tool.json`, `killed.sse`), skrip
  `/tmp/smoke11/live.sh`, driver panel `/tmp/smoke11/panel_reader.ts`.

### 23.2 Yang belum dikerjakan

- Suite ber-tag `integration` belum dijalankan: satu run (atau pre-push, yang menjalankannya) menghapus
  Redis DB 0 lewat `FlushDB` dan mengeluarkan operator dari panel, jadi menunggu saat dia tidak memakainya.
- Context akuntansi yang sama belum diterapkan ke tiga jalur yang masih memakai context klien:
  `media_perform.go:81`, `embeddings_call.go:66`, `systemone.go:197`. F6 yang terukur hanya di jalur chat.
- F4 dan F8 sampai F10 (permukaan daftar model) tidak disentuh pass ini.
