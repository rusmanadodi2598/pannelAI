# 028-FALLBACK-CREDENTIAL-PARITY.md: urutan failover `app-serv` terhadap reference 9Router

Laporan owner dari testing live: ketika sebuah model gagal dipanggil, `app-serv` merouting ke
model lain yang tidak diminta, termasuk model yang tidak ada dan provider yang tidak
dikonfigurasi. Dokumen ini mengukur laporan itu, membandingkan mekanismenya dengan reference, dan
menuliskan keputusan yang harus diambil supaya semantiknya 1:1. Bukan kontrak; kontrak tetap
`docs/SPEC-API/001-SPEC-API.md` (wire) dan `docs/SPEC-UI/001-SPEC-UI.md` (perilaku panel).

| | |
|---|---|
| **Status** | **IMPLEMENTED (2026-09-24).** F1-F4 dikerjakan 1:1 dengan reference (owner memilih (a) untuk keempatnya, §6); spec diamandemen lebih dulu, kode dan test permanen mendarat di `04e422e`, dokumen ini menyusul sebagai commit kedua |
| **Mechanism** | DURING & AFTER (antislop) |
| **Scope** | `app-serv/.` data plane: urutan failover, klasifikasi kegagalan, kesehatan kredensial. `app-ui/.` tidak disentuh |
| **Permintaan owner** | "Ya, langsung DRAFT kan supaya 1:1 dengan REFERENCE" |
| **Laporan yang diukur** | "ketika model gagal di panggil ... dia merouting ke model lain yang sebenarnya itu salah, model tersebut tidak ada, konfigurasi provider tidak ada" |
| **Reference** | `decolua/9router`, checkout `/home/rusmanadodi/apps/9router`, **`21583c03` = tag `v0.5.85` (2026-09-22)** |
| **Kaitan** | SPEC-API-001 §7.5, §7.7, §7.15; draft 017 F1b; draft 024 §3.2/§3.4; draft 027 §3.1 baris 3-10 dan §4 |
| **Tanggal** | 2026-09-24 |

---

## 1. Ringkasan

Lima probe penerimaan dijalankan di dalam paket `internal/dataplane`, memakai fixture relay yang
sudah ada (`engine_relay_fixture_test.go`), lalu berkas probe dihapus. Setiap probe menegaskan
perilaku yang DIINGINKAN (semantik reference), sehingga **kelimanya gagal hari ini** dan menjadi
test penerimaan yang lulus setelah perbaikan. Semuanya mereproduksi satu kelas masalah:
**failover `app-serv` hanya mengenal satu arah, yaitu pindah model, dan pindahnya tidak sadar
kelas error maupun sadar kredensial.**

| # | Tingkat | Isi singkat |
|---|---|---|
| **F1** | HIGH | Tidak ada failover kredensial dalam satu request. Satu `Select` per member, dan satu-satunya failover adalah pindah member combo. Akun/key sehat berikutnya di provider yang sama tidak pernah dicoba di request yang sama |
| **F2** | HIGH | Klasifikasi kegagalan tidak sadar kelas: semua status upstream non-429 menjadi `UPSTREAM_ERROR`, dan `UPSTREAM_ERROR` layak failover, sehingga 400 request-scoped pun melewati rantai ke model lain |
| **F3** | MEDIUM | Member yang tidak pernah dipanggil (provider tanpa endpoint, model tidak resolve) tetap mengisi error klien dan baris usage; reference tidak menulis akuntansi apa pun sebelum `handleChatCore` |
| **F4** | MEDIUM | Satu endpoint dengan N key tidak diperlakukan sebagai kumpulan kredensial: `NextKey` selalu memilih key prioritas tertinggi (`MarkUsed` nol pemanggil, tidak ada rotasi), dan sirkuit key baru parkir setelah 3 kegagalan dengan backoff tetap 2 menit |

Dua catatan pembacaan sebelum temuan:

1. **Rotasi combo sendiri sudah 1:1.** Kedua sistem memajukan rotasi pada setiap request, termasuk
   yang gagal (`combo_order.go:46`; reference `combo.js:222-234`). Yang berbeda bukan rotasinya,
   melainkan apa yang dilakukan setelah satu member gagal.
2. **Perilaku sekarang terkunci oleh spec, bukan drift kode.** SPEC-API-001 §7.5 baris 232-233
   ("Routing picks endpoint by priority, then a healthy key inside it") dan §7.7 baris 291
   ("`fallback`: try models in priority order until success or all exhausted") menuliskan konsep
   yang berlaku sekarang, dan `engine_relay_test.go:34-108` menguncinya dengan test yang lulus.
   Target "1:1 dengan reference" karena itu berarti **amandemen spec lebih dulu**, bukan tambalan
   di kode.

## 2. Metode dan bukti

### 2.1 Revisi reference

```bash
cd /home/rusmanadodi/apps/9router
git log -1 --format='%H %ci %d'
# 21583c03e5c5d5276924efad82328ebe6e215854 2026-09-22 15:43:30 +0700 (tag: v0.5.85)
```

Checkout itu kemudian maju ke `39e36d3d` (2026-09-23) saat implementasi, dan semantiknya
diukur tidak berubah: `git diff 21583c03..39e36d3d` atas `chat.js`, `combo.js`, `auth.js`,
`accountFallback.js`, dan `errorConfig.js` keluar kosong, jadi F1-F4 dikerjakan terhadap
revisi baru tanpa menyalin perilaku yang berbeda.

### 2.2 Cara mengukur

Probe ditulis sebagai satu berkas test sementara di `internal/dataplane/` (fixture relay yang sama
dipakai test yang sudah ada), dijalankan, lalu dihapus. Tree bersih setelahnya: `git status`
tidak memuat berkas probe. Kodenya ada di §2.4 supaya bisa dipasang kembali sebagai test
penerimaan; setiap probe mengasersikan perilaku yang DIINGINKAN, jadi kelimanya FAIL terhadap
kode hari ini dan PASS setelah perbaikan.

```bash
cd /home/rusmanadodi/pannelAI/app-serv
go test ./internal/dataplane/ -run 'TestProbe_' -v
```

### 2.3 Output terukur (lima FAIL, perilaku yang diinginkan belum ada)

```
=== RUN   TestProbe_CredentialFailoverWithinRequest
    zz_probe_acceptance_temp_test.go:51: request failed although a second healthy account exists: calls=1 err=UPSTREAM_ERROR: upstream refused m: upstream returned Internal Server Error: upstream refused m
--- FAIL: TestProbe_CredentialFailoverWithinRequest (0.00s)

=== RUN   TestProbe_UnroutableMemberDoesNotOwnTheError
    zz_probe_acceptance_temp_test.go:74: the never-called member owns the outcome: provider="ghost" code=NO_PROVIDER_AVAILABLE
--- FAIL: TestProbe_UnroutableMemberDoesNotOwnTheError (0.00s)

=== RUN   TestProbe_UnresolvableMemberDoesNotOwnTheError
    zz_probe_acceptance_temp_test.go:93: the stale member owns the client error: code=MODEL_NOT_FOUND message=model strict/missing is not available
--- FAIL: TestProbe_UnresolvableMemberDoesNotOwnTheError (0.00s)

=== RUN   TestProbe_RequestScoped400IsReturnedToTheClient
    zz_probe_acceptance_temp_test.go:108: the 400 was not returned: the engine failed over to the next model (calls=2)
--- FAIL: TestProbe_RequestScoped400IsReturnedToTheClient (0.00s)

=== RUN   TestProbe_KeysRotateWithinAnEndpoint
    zz_probe_acceptance_temp_test.go:135: key 2 was never selected in 3 requests; keys do not rotate
--- FAIL: TestProbe_KeysRotateWithinAnEndpoint (0.00s)
FAIL
FAIL	github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane	0.020s
FAIL
```

### 2.4 Kode probe (test penerimaan; dipasang kembali saat implementasi)

> **Status (2026-09-24): sudah dipasang kembali sebagai test permanen dan kelima lulus.**
> `engine_failover_acceptance_test.go` memuat probe 2-5 apa adanya; probe 1 dipasang dalam
> bentuk yang lebih kuat, karena bentuk di bawah **tidak bisa lulus bahkan setelah
> perbaikan**: kegagalannya per-model, dan status per-model menjawab akun kedua persis
> seperti akun pertama, sehingga tidak ada failover kredensial yang bisa membalikkannya.
> Test permanennya menolak kredensial akun pertama lewat header `Authorization`
> (`plain-sealed-ep-1`) dan menegaskan request dilayani `ep-2` dengan `calls=2`. Double
> upstream kedua bentuk itu hidup di `engine_failover_fixture_test.go`. Blok kode di bawah
> tetap sebagai catatan apa adanya dari pass analisis.

```go
// Setiap probe menegaskan perilaku yang DIINGINKAN, jadi semuanya FAIL terhadap
// kode hari ini dan PASS setelah perbaikan F1-F4.
func probeUpstream(t *testing.T, calls *int, statusByModel map[string]int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		if status, ok := statusByModel[body.Model]; ok && status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"message":"upstream refused ` + body.Model + `"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","created":1,"model":"` + body.Model + `",` +
			`"choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],` +
			`"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	t.Cleanup(server.Close)
	return server
}

// F1 diinginkan: satu request mencoba akun sehat berikutnya sebelum menyerah.
// Terukur sekarang: calls=1, request gagal.
func TestProbe_CredentialFailoverWithinRequest(t *testing.T) {
	var calls int
	server := probeUpstream(t, &calls, map[string]int{"m": http.StatusInternalServerError})
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{
		relayEndpoint(t, "ep-1", "alpha"),
		relayEndpoint(t, "ep-2", "alpha"),
	}
	engine := newRelayEngine(t, server.URL, repo, nil)
	_, err := engine.Relay(context.Background(), relayRequest("alpha/m"), nil)
	if err != nil {
		t.Fatalf("request failed although a second healthy account exists: calls=%d err=%v", calls, err)
	}
	if calls != 2 {
		t.Fatalf("upstream calls = %d, want 2 (one per account)", calls)
	}
}

// F3 diinginkan: member tanpa endpoint tidak memiliki error klien maupun
// identitas yang tercatat; error milik member yang benar-benar dipanggil.
// Terukur sekarang: provider="ghost" code=NO_PROVIDER_AVAILABLE.
func TestProbe_UnroutableMemberDoesNotOwnTheError(t *testing.T) {
	var calls int
	server := probeUpstream(t, &calls, map[string]int{"broken": http.StatusInternalServerError})
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	providers := []registry.Provider{relayProvider("alpha", server.URL), relayProvider("ghost", server.URL)}
	engine := newEngineWith(t, providers, repo, map[string]domain.Combo{
		"daily": comboRow("daily", "alpha/broken", "ghost/anything"),
	}, nil)
	outcome, err := engine.Relay(context.Background(), relayRequest("daily"), nil)
	if err == nil {
		t.Fatal("want the called member's failure")
	}
	if outcome.ProviderID == "ghost" || AsError(err).Code == CodeNoProvider {
		t.Fatalf("the never-called member owns the outcome: provider=%q code=%s", outcome.ProviderID, AsError(err).Code)
	}
}

// F3 diinginkan: member yang tidak resolve tidak menjadi error klien.
// Terukur sekarang: code=MODEL_NOT_FOUND untuk "strict/missing".
func TestProbe_UnresolvableMemberDoesNotOwnTheError(t *testing.T) {
	var calls int
	server := probeUpstream(t, &calls, map[string]int{"broken": http.StatusInternalServerError})
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	strict := relayProvider("strict", server.URL)
	strict.PassthroughModels = false
	engine := newEngineWith(t, []registry.Provider{relayProvider("alpha", server.URL), strict}, repo,
		map[string]domain.Combo{"daily": comboRow("daily", "alpha/broken", "strict/missing")}, nil)
	_, err := engine.Relay(context.Background(), relayRequest("daily"), nil)
	if err == nil {
		t.Fatal("want the called member's failure")
	}
	if code := AsError(err).Code; code != CodeUpstreamError {
		t.Fatalf("the stale member owns the client error: code=%s message=%s", code, AsError(err).Message)
	}
}

// F2 diinginkan: 400 request-scoped adalah jawaban klien, bukan pemicu
// failover. Terukur sekarang: engine pindah model (calls=2).
func TestProbe_RequestScoped400IsReturnedToTheClient(t *testing.T) {
	var calls int
	server := probeUpstream(t, &calls, map[string]int{"bad": http.StatusBadRequest})
	repo := newMemEndpointRepo()
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{relayEndpoint(t, "ep-alpha", "alpha")}
	engine := newRelayEngine(t, server.URL, repo, map[string]domain.Combo{
		"daily": comboRow("daily", "alpha/bad", "alpha/works"),
	})
	_, err := engine.Relay(context.Background(), relayRequest("daily"), nil)
	if err == nil {
		t.Fatalf("the 400 was not returned: the engine failed over to the next model (calls=%d)", calls)
	}
	if status := AsError(err).Status; status != http.StatusBadRequest {
		t.Fatalf("client status = %d, want 400", status)
	}
}

// F4 diinginkan: key sehat berotasi di dalam satu endpoint.
// Terukur sekarang: key 2 tidak pernah terpilih dalam 3 request.
func TestProbe_KeysRotateWithinAnEndpoint(t *testing.T) {
	var calls int
	server := probeUpstream(t, &calls, nil)
	repo := newMemEndpointRepo()
	ep := relayEndpoint(t, "ep-1", "alpha")
	second := domain.RehydrateUpstreamKey(
		"uky-k2", "ep-1", "key2", "sealed-k2", "…hint2",
		1, domain.UpstreamKeyActive, nil, "", 0, nil, now, now,
	)
	ep.AttachKey(second)
	repo.byProvider["alpha"] = []domain.UpstreamEndpoint{ep}
	engine := newRelayEngine(t, server.URL, repo, nil)
	for i := 0; i < 3; i++ {
		if _, err := engine.Relay(context.Background(), relayRequest("alpha/m"), nil); err != nil {
			t.Fatalf("request %d error = %v", i, err)
		}
	}
	_, key2Used := repo.health["uky-k2"]
	if !key2Used {
		t.Fatalf("key 2 was never selected in %d requests; keys do not rotate", calls)
	}
}
```

### 2.5 Yang diperiksa dan bukan temuan

| Hal | Kenapa bukan temuan |
|---|---|
| Rotasi combo per request | Sama di kedua sisi: app-serv `combo_order.go:46` (state Redis), reference `combo.js:222-234` (state in-process). Keduanya maju walau request gagal |
| Rotasi endpoint lintas request | Ada dan berjalan di app-serv (`selection.go:146,222-231`, cursor Redis + sticky limit). Yang tidak ada rotasi adalah key di dalam endpoint (F4) |
| Retry POST chat maksimum 1 | Sesuai SPEC-API-001 §4 dan alasan billing (`retry.go:37-40`); bukan drift |
| Reference juga failover pada 500 dan 404 | Benar (`errorConfig.js:70-75`). Yang berbeda adalah urutan (kredensial dulu) dan perlakuan 4xx request-scoped (F1/F2) |
| `TestRelay_RequestFailureStopsTheCombo` (`engine_relay_test.go:113-140`) | Mengunci kegagalan translasi **sebelum** panggilan upstream (`calls=0`, `CodeValidation`). Kelasnya berbeda dari 400 yang dikembalikan upstream, jadi tidak bertentangan dengan F2; reference pun tidak menghabiskan akun untuk request yang tidak bisa ditranslasi |
| Klaim draft 027 §3.1 baris 3 ("Seleksi akun ... 1:1") | Benar untuk permukaan `Select` yang ada (kandidat tidak tersedia dilewati, semua habis menjadi 503 `NO_PROVIDER_AVAILABLE`), tetapi tidak untuk loop akun di dalam request: padanan `excludeConnectionIds.add` + `continue` tidak ada, dan itulah yang diukur F1 dokumen ini. Baris 027 itu perlu dikoreksi satu baris (belum disentuh supaya pass ini tetap satu berkas) |
| `statusFor(CodeModelNotFound) = 400` sementara komentarnya menyebut 404 | Ditemukan saat membaca (`errors.go:90-101`); tidak berdampak pada temuan di dokumen ini, dicatat sebagai perbaikan terpisah |

## 3. Temuan

### 3.1 F1 (HIGH): failover kredensial tidak ada dalam satu request

**Fakta.** Seleksi dijalankan tepat satu kali per member, dan hanya member combo yang diiterasi:

```bash
cd /home/rusmanadodi/pannelAI/app-serv
grep -rn 'selector.Select' internal/dataplane/*.go | grep -v _test
# internal/dataplane/engine_relay.go:48:	selection, err := e.selector.Select(ctx, resolution.Provider.ID)
```

- Loop member: `engine.go:180-204` (`for index, ref := range refs`), dengan `refs` dari
  `resolution.Combo.Refs()` atau satu referensi untuk model tunggal (`engine.go:163-166`).
- Retry di transport mengulang **target yang sama**: `transport_call.go:69-99` memanggil
  `t.attempt(ctx, plugin, call, url)` dengan `url` dan `call` (termasuk kredensial) yang sama, dan
  POST non-idempoten dibatasi 1 retry (`retry.go:117-119`).
- Akun lain hanya dipakai pada **request berikutnya**: cursor endpoint (`selection.go:146`) atau
  setelah sirkuit key trip (`upstream_key_health.go:45-55`).

Probe 1 menutup kemungkinan "sudah dicoba tapi gagal": dengan dua endpoint (dua akun) di satu
provider dan akun kedua sehat, hasilnya `calls=1` dan request gagal. Tidak ada percobaan kedua.

**Reference.** `handleSingleModelChat` membungkus pemanggilan dalam loop akun: `chat.js:233`
(`while (true)`), `:234` (`getProviderCredentials` dengan `excludeConnectionIds`), `:329-335`
(setelah gagal: `excludeConnectionIds.add(...)` lalu `continue`), `:337` (berhenti hanya saat
`shouldFallback` bernilai false). Jadi satu request klien menghabiskan akun yang tersedia dulu,
baru (bila member-nya bagian combo) pindah model.

**Akibat.** Satu 429/5xx dari akun pertama membuat request klien gagal meskipun akun lain sehat di
provider yang sama. Operator melihat kegagalan yang seolah soal model, padahal soal kredensial.
Karena satu-satunya failover yang tersedia adalah pindah model, setiap kegagalan kredensial juga
menjadi kesempatan membakar akun model lain.

**Rencana DURING bila owner memilih 1:1.** Tambah jalur "kandidat berikutnya" pada `Selector`
(mis. `SelectNext(ctx, providerID, exclude []string)`), lalu ubah `relayOnce` menjadi loop: pilih
kandidat, panggil, dan bila kegagalannya layak failover (lihat F2), tandai kandidat itu dan coba
kandidat sehat berikutnya dari provider yang sama sebelum menyerah ke member combo berikutnya.
Kriteria selesai: probe 1 berbalik menjadi sukses dengan `calls=2`, dan satu request dengan dua
akun sehat tidak pernah gagal karena akun pertama mati.

### 3.2 F2 (HIGH): klasifikasi kegagalan tidak sadar kelas

**Fakta.** Semua status upstream selain 429 dipetakan ke satu kode, dan kode itu layak failover:

```bash
cd /home/rusmanadodi/pannelAI/app-serv
sed -n '46,73p' internal/dataplane/engine_failure.go
# 429 → CodeRateLimited; 401/403 → CodeUpstreamError; sisanya → CodeUpstreamError
# failoverWorthy: CodeUpstreamError, CodeUpstreamTimeout, CodeRateLimited, CodeNoProvider
```

Retry transport pun hanya mengenal 429/502/503/504 (`provider/plugin_base.go:55-63`), dan 402/403
diparkir tanpa retry (`plugin_base.go:67-69`, `transport_call.go:84-86`). Sebuah 400 yang
disebabkan request (context overflow, parameter tak didukung, model tidak ada di upstream) karena
itu menjadi `UPSTREAM_ERROR`, yaitu kode yang memicu failover.

Probe 4 mengukurnya: member pertama menjawab 400, dan engine failover ke member kedua (`calls=2`),
sehingga 400 itu tidak pernah sampai ke klien.

**Reference.** `accountFallback.js:48-60` sengaja mengembalikan `shouldFallback: false` untuk 4xx
yang bukan 401/402/403/429, dengan alasan tertulis di komentar: error yang disebabkan request
tidak mengatakan apa pun tentang kredensial, dan memarkir akun hanya menghapus koneksi sehat dari
rotasi. Aturan ini dipakai di dua tempat: loop akun (`chat.js:327-337`) dan loop combo
(`combo.js:335-340`). Status yang tetap failover: 401/402/403/404 dan 429 (`errorConfig.js:70-75`),
ditambah aturan teks (`"no credentials"`, `"rate limit"`, `"overloaded"`, dan seterusnya,
`errorConfig.js:59-68`).

**Akibat.** Persis laporan owner: request yang salah bentuk dibayar di beberapa model, dan klien
dapat menerima jawaban dari model yang tidak ia minta. Efek samping: akun sehat di member
berikutnya ikut terkunci oleh error yang bukan salahnya.

**Rencana DURING bila owner memilih 1:1.** Pisahkan kelas "upstream menolak request" dari
`UPSTREAM_ERROR` (kode baru, mis. `UPSTREAM_REJECTED`, status 400), dan keluarkan kode itu dari
`failoverWorthy`. Aturan kelasnya mengikuti reference: 400/406/409/422 dan sejenisnya berhenti;
401/402/403/404/429 tetap failover. Kriteria selesai: probe 4 mengembalikan 400 itu ke klien tanpa
memanggil member kedua.

### 3.3 F3 (MEDIUM): member yang tidak pernah dipanggil mengisi error dan akuntansi

**Fakta.** Dua jalur, keduanya terukur di probe 2 dan 3:

- **Tidak resolve** (`engine.go:181-185`): `lastErr = resolveErr; continue`. Member dilewati
  diam-diam, tetapi `lastErr`-nya tetap menjadi error akhir bila tidak ada member lain yang
  menghasilkan error setelahnya (`engine.go:205`).
- **Resolve tetapi tidak punya endpoint**: `relayOnce` membangun `Outcome` sebelum langkah pertama
  yang bisa gagal (`engine_relay.go:40-46`), selection gagal
  (`NO_PROVIDER_AVAILABLE`, `selection.go:141-144`), dan outcome itu menjadi `lastOutcome`
  (`engine.go:195-197`). Outcome membawa `ProviderID` dan `Model` yang terisi, sehingga
  `ChatService.record` menulis baris usage dan request log untuk identitas itu
  (`service/chat_record.go:60-90`, `:100-115`).

Probe 2: klien menerima `NO_PROVIDER_AVAILABLE ... provider ghost` dan outcome berisi
`ghost/anything`, padahal `ghost` tidak pernah menerima panggilan (`calls=1`, hanya member alpha
yang dipanggil). Probe 3: klien menerima `MODEL_NOT_FOUND` untuk `strict/missing` yang tidak
pernah ia sebut.

**Reference.** Tidak ada akuntansi sebelum panggilan keluar: member yang tidak punya kredensial
dijawab 404 "No active credentials for provider: X" di `chat.js:244-247`, sebelum
`handleChatCore` (satu-satunya tempat `saveRequestDetail`/`appendRequestLog` dipanggil,
`chatCore.js:320-321,471-481`). Saat semua member gagal, combo mengembalikan 503 dengan pesan
member terakhir dan status kegagalan pertama (`combo.js:352-353,363-381`).

**Asal member basi.** Validasi referensi combo ada di write time (`combo_order.go:61-130`), tetapi
tidak bertahan terhadap penghapusan sesudahnya: `NodeService.Delete` hanya menolak bila masih ada
**endpoint** yang merujuk node itu (`provider_node.go:184-200`), dan tidak ada pemeriksaan combo
(`grep` `combo` di `internal/service/provider_node*.go` = 0 hasil). Menghapus node/provider yang
masih dirujuk combo meninggalkan member basi yang persis berperilaku seperti probe 2/3.

**Akibat.** Panel menampilkan request terhadap provider yang tidak dikonfigurasi dan model yang
tidak pernah diminta, dan error klien menunjuk keduanya. Ini bagian yang paling mungkin dibaca
operator sebagai "gateway merouting ke model yang salah".

**Rencana DURING bila owner memilih 1:1.** (a) Hanya member yang benar-benar mencapai panggilan
upstream yang boleh mengisi `lastOutcome` (dan karena itu baris usage); member yang gagal sebelum
panggilan tidak boleh menjadi identitas akuntansi. (b) Error akhir mengikuti reference: status
kegagalan pertama, pesan kegagalan terakhir, dan member yang tidak bisa dirutekan tidak menyetel
keduanya. (c) `NodeService.Delete` mempertimbangkan combo (menolak, atau memberi peringatan yang
menyebut combo yang terdampak). Kriteria selesai: probe 2 dan 3 menghasilkan error tentang member
yang benar-benar dicoba, dan tidak ada baris usage untuk member yang tidak pernah dipanggil.

### 3.4 F4 (MEDIUM): satu endpoint multi key tidak diperlakukan sebagai kumpulan kredensial

**Fakta.** Tiga hal, semuanya terukur:

```bash
cd /home/rusmanadodi/pannelAI/app-serv
grep -rn 'MarkUsed' --include='*.go' internal/ | grep -v _test
# internal/domain/upstream_key_health.go:72-77   (hanya definisi, nol pemanggil)
sed -n '82,91p' internal/domain/upstream_endpoint_key_reads.go
# NextKey = ValidKeys[0] = key prioritas tertinggi yang sehat
sed -n '28,36p' internal/domain/upstream_key.go
# keyCircuitThreshold = 3; keyCircuitBackoff = 2 * time.Minute
```

- **Tanpa rotasi key**: `NextKey` selalu mengembalikan key prioritas tertinggi yang sehat, dan
  tidak ada kode yang memanggil `MarkUsed`, jadi `lastUsedAt` tidak pernah dipakai untuk memilih.
  Dua key sehat di satu endpoint: key pertama melayani semuanya (probe 5: setelah 3 request,
  hanya key 1 yang pernah dipakai).
- **Parkir terlambat**: `RecordFailure` baru menandai key setelah 3 kegagalan beruntun
  (`upstream_key_health.go:45-55`), dan window-nya tetap 2 menit untuk semua kelas error
  (`upstream_key.go:30,36`).
- **Satu key per request**: konsekuensi F1; key kedua tidak pernah dicoba dalam request yang sama.

**Reference.** Satu koneksi adalah satu key (`auth.js:197-223`), kegagalan pertama langsung
mengunci pasangan akun+model dengan cooldown per kelas (`auth.js:239-287`): 401/402/403/404 = 2
menit, 429 = backoff eksponensial 2 detik sampai 5 menit, transient = 30 detik
(`errorConfig.js:45-77`), dan rotasi antar akun mengikuti strategi fill-first / round-robin +
sticky (`auth.js:136-193`).

**Akibat.** Key mati masih melayani dua request gagal berikutnya; 429 tidak langsung diparkir;
dua key sehat tidak pernah bergantian sehingga kuota keduanya tidak terpakai merata. Digabung F1,
satu request tidak pernah mencoba key kedua di endpoint yang sama.

**Rencana DURING bila owner memilih 1:1.** (a) Parkir key pada kegagalan pertama dengan cooldown
per kelas (menggantikan ambang 3), dan (b) rotasi key sehat dalam endpoint (memakai `MarkUsed`
atau cursor key), dengan (c) failover key dalam request dari F1. Kriteria selesai: probe 5
menunjukkan key 2 dipakai, dan key yang gagal 429 langsung tidak dipilih pada request berikutnya.

## 4. VS reference

Hierarki reference untuk satu request klien: **kredensial dulu, model kemudian, URL terakhir**.

```
reference:  member (model) → akun/koneksi → executor retry → base URL fallback
app-serv:   member (model) → [retry target yang sama] → menyerah
```

| Aspek | `app-serv` | 9Router `v0.5.85` |
|---|---|---|
| Unit failover dalam satu request | member combo (model) saja | akun dari model yang sama, baru member combo |
| Kapan kredensial gagal diparkir | setelah 3 kegagalan beruntun, window tetap 2 menit | kegagalan pertama, cooldown per kelas (2 menit / eksponensial / 30 detik) |
| 4xx request-scoped (400, 406, 409, 422) | failover lanjut (`UPSTREAM_ERROR`) | berhenti, error dikembalikan |
| 401/402/403/404 | failover lanjut; parkir setelah 3 | failover; akun dikunci 2 menit untuk model itu |
| 429 | failover lanjut; parkir setelah 3 | failover; parkir langsung, backoff eksponensial |
| Rotasi key dalam satu endpoint | tidak ada (`NextKey` = prioritas tertinggi) | rotasi antar koneksi (fill-first / round-robin + sticky) |
| Rotasi combo per request | ada (Redis) | ada (in-process) |
| Akuntansi untuk member yang tidak pernah dipanggil | baris usage + log ditulis | tidak ada tulisan sebelum `handleChatCore` |
| Error akhir saat semua member gagal | error dan identitas member terakhir | pesan member terakhir + status kegagalan pertama |

## 5. Di mana konsepnya berbeda

Reference memperlakukan **kredensial sebagai unit yang bisa dicoba ulang di dalam satu request**;
kegagalan adalah fakta tentang pasangan kredensial+model, dicatat per pasangan
(`modelLock_${model}`, `auth.js:267-276`). `app-serv` memperlakukan **endpoint sebagai unit
pemilihan** dan **member combo sebagai satu-satunya failover**; kegagalan kredensial baru menjadi
fakta lintas request lewat sirkuit ambang 3. Dua akibat langsung:

1. Ketika satu key/akun gagal, satu-satunya langkah yang tersedia adalah **model lain**, dan
   model lain itu bisa member combo yang basi (F3).
2. Error yang seharusnya menghentikan rantai (request-scoped 4xx) malah menjadi pemicu perpindahan
   model (F2), sehingga klien bisa dilayani model yang tidak ia minta.

Yang mengunci konsep sekarang: SPEC-API-001 §7.5 baris 232-233 dan 265-266 (endpoint, key
priority, sirkuit ambang 3), §7.7 baris 291 (fallback antar model), §7.15 baris 570-579 (diagram
pipeline tanpa langkah failover kredensial), dan test `engine_relay_test.go:34-108`. Karena itu
urutan kerjanya: **amandemen spec dulu, lalu kode, lalu probe di §2.4 menjadi test penerimaan.**

## 6. Keputusan owner

Empat pertanyaan, satu per temuan. Tidak ada source yang diubah sebelum jawabannya ada.

| # | Pertanyaan | Pilihan | Rekomendasi penulis |
|---|---|---|---|
| 1 | **F1 urutan failover** | (a) Port loop kredensial: kandidat key/endpoint sehat berikutnya dari provider yang sama dicoba dulu, model kemudian (1:1 reference); (b) pertahankan model-first, catat sebagai deviasi | **(a)**, karena ini inti keluhan dan menutup probe 1 |
| 2 | **F2 klasifikasi kegagalan** | (a) Kelas sadar: 4xx request-scoped berhenti, 401/402/403/404/429 tetap failover (1:1 reference); (b) pertahankan | **(a)**; berdiri sendiri, bisa dikerjakan tanpa F1 |
| 3 | **F3 identitas error dan akuntansi** | (a) Hanya member yang benar-benar dipanggil yang mengisi outcome/usage; error akhir = status pertama + pesan terakhir; guard delete mempertimbangkan combo; (b) pertahankan | **(a)** |
| 4 | **F4 kesehatan dan rotasi key** | (a) Parkir key pada kegagalan pertama dengan cooldown per kelas + rotasi key sehat dalam endpoint (1:1 reference); (b) pertahankan ambang 3 | **(a)**, dikerjakan bersama F1 karena keduanya menyentuh jalur seleksi yang sama |

**Keputusan (2026-09-24): owner memilih (a) untuk keempatnya**, kutipan langsung: "Keputusan Anda
(§6, rekomendasi (a) semua)". Pass implementasi karena itu menjalankan F1-F4 1:1 dengan reference,
dengan catatan pelaksanaan di bawah sebagai batasnya.

Catatan pelaksanaan:

- **Amandemen spec lebih dulu**, dalam satu commit yang sama dengan kode atau sebelumnya: §7.5
  (model key health + aturan failover kredensial), §7.7 (urutan fallback: kredensial lalu model),
  §7.15 (diagram pipeline menambah langkah failover kredensial).
- **Satu slice, satu seri test**: probe §2.4 dipasang kembali sebagai test penerimaan, plus test
  regresi untuk `engine_relay_test.go` yang sekarang mengunci perilaku lama (`:34-108`).
- **Deviasi yang sengaja dipertahankan** (bukan 1:1): rotasi combo tetap di Redis (reference
  in-process, hilang saat proses restart), dan endpoint multi-key tetap model app-serv (reference
  1 koneksi = 1 key). Keduanya dicatat supaya "1:1" tidak dibaca sebagai "menyalin semua".

## 7. Status pass ini

- **Analisis selesai; tidak ada berkas source yang diubah.** Keluaran pass ini satu berkas:
  `docs/DRAFT/028-FALLBACK-CREDENTIAL-PARITY.md`.
- Lima probe penerimaan dijalankan (lima FAIL terhadap kode hari ini, §2.3) lalu berkas probe
  dihapus; tree bersih setelahnya (`git status` tanpa berkas probe).
- Semua angka diukur dari working tree 2026-09-24; reference pada `21583c03` = `v0.5.85`.
- Tidak ada test suite penuh yang dijalankan karena tidak ada kode yang berubah. Yang dijalankan
  adalah probe bertarget (§2.2) dan `go test -run 'TestRelay_(Combo|RequestFailure)' -v` (tiga
  test lulus) untuk membuktikan perilaku lama masih terkunci.

### 7.1 Gerbang pass analisis (antislop, DURING & AFTER)

Pass ini tidak menyentuh UI. Empat blok tetap dilaporkan seluruhnya; item yang tidak berlaku
dijawab dengan alasannya.

**Block 1: Hard Gate (17 item, semua harus "tidak")**

| Item | Jawaban | Bukti / alasan |
|---|---|---|
| R-02 em dash | **tidak** | `grep -cP '\x{2014}\|\x{2013}' docs/DRAFT/028-FALLBACK-CREDENTIAL-PARITY.md` = 0 |
| R-03 overflow mobile | tidak berlaku | Tidak ada UI; keluaran berkas markdown |
| R-17 angka tanpa sumber | **tidak** | Setiap angka diikuti perintah/berkas sumbernya (§2.3, §3) |
| R-18 testimoni fiktif | tidak berlaku | Tidak ada testimoni |
| R-23 aset tanpa instruksi | tidak berlaku | Tidak ada aset visual yang dibuat |
| R-24 navigasi mati | tidak berlaku | Tidak ada navigasi |
| R-25 kontras | tidak berlaku | Tidak ada teks ter-render |
| R-26 kontrol mati | tidak berlaku | Tidak ada kontrol |
| R-27 state UI | tidak berlaku | Tidak ada UI berdata |
| R-28 FAQ | tidak berlaku | Tidak ada FAQ |
| R-32 keyboard | tidak berlaku | Tidak ada UI |
| R-33 patch via script | **tidak** | Tidak ada skrip yang menyunting source; probe ditulis dan dihapus manual |
| R-34 tema | tidak berlaku | Tidak ada tema |
| R-35 dijalankan sebelum diserahkan | tidak berlaku untuk UI | Tidak ada UI. Yang dijalankan: lima probe penerimaan (`go test -run 'TestProbe_' -v`, lima FAIL, output §2.3) dan satu run test yang ada (`go test -run 'TestRelay_(Combo\|RequestFailure)' -v`, tiga test lulus) |
| R-36 klaim fabrikasi | **tidak** | Setiap klaim menyitir berkas yang bisa dibuka; tidak ada klaim keamanan/compliance |
| R-37 arah desain | tidak berlaku | Tidak ada UI yang dibangun |
| R-38 konten fabrikasi | **tidak** | Sitasi reference dari checkout `21583c03`; sitasi `app-serv` dari working tree; output probe dikutip apa adanya |

**Block 2: Purpose-Gate (12 item)**

Seluruhnya **tidak berlaku** dengan satu alasan yang sama: deliverable ini dokumen keputusan,
tidak memuat gradien, ikon, tipografi, latar bermotif, panah, badge, glassmorphism, shadow, glow,
kartu, animasi, atau ilustrasi. R-31 (alasan satu baris per keputusan) dijalankan pada tingkat
isi: setiap temuan menuliskan mengapa ia temuan, setiap opsi menuliskan akibatnya (§3, §6).

**Block 3: Liveliness (7 item)**

Tidak berlaku seluruhnya: tidak ada layar, jadi tidak ada dial ENERGY/RHYTHM/MOTION, focal point,
whitespace, accent, atau motif identitas yang bisa dinilai. Yang dibuat adalah dokumen kerja, dan
statusnya dinyatakan di tabel header (OPEN, keputusan).

**Block 4: Craftsmanship & Quality Locks (14 item)**

| Item | Jawaban | Bukti / alasan |
|---|---|---|
| C-1 intentionality | **tidak** | Setiap temuan punya bukti terukur; tidak ada keputusan yang alasannya "default AI" |
| C-2 functional completeness | tidak berlaku | Tidak ada elemen interaktif |
| C-3 content-driven composition | **tidak** | Struktur mengikuti laporan owner dan hasil probe, bukan template |
| C-4 resilience | tidak berlaku | Tidak ada UI |
| C-5 evidence over claims | **tidak** | Tidak ada testimoni/statistik tanpa sumber |
| R-05 template layout | tidak berlaku | Tidak ada layout |
| R-11 radius | tidak berlaku | Tidak ada UI |
| R-15 CTA | tidak berlaku | Tidak ada CTA |
| R-16 buzzwords | **tidak** | Tidak ada "AI Powered", "Seamless", "Revolutionary" di teks |
| R-20 identitas visual | tidak berlaku | Tidak ada visual |
| R-21 dark mode | tidak berlaku | Tidak ada tema |
| R-29 palet | tidak berlaku | Tidak ada warna |
| R-30 klon produk | tidak berlaku | Tidak ada visual |
| R-31 alasan tertulis | **tidak** (lulus) | Setiap temuan menulis alasan dan setiap opsi menulis akibat (§3, §6) |

Tidak ada item yang FAIL. Catatan jujur yang sama dengan draft 027: R-35 dan seluruh Block 2/3
dijawab "tidak berlaku", bukan "PASS", karena tidak ada UI yang bisa diklik; satu-satunya bukti
yang bisa diberikan pass ini adalah perintah pengukuran dan test yang bisa diulang, dan itu yang
dikutip.

## 8. Pass implementasi (2026-09-24)

Owner memilih (a) untuk keempat temuan (§6). Urutan kerjanya seperti yang dijanjikan dokumen ini:
amandemen spec lebih dulu, lalu kode, lalu probe §2.4 dipasang kembali sebagai test penerimaan.

| Temuan | Kode | Test |
|---|---|---|
| F1 | `selection_candidates.go` (`SelectNext`, `CandidateID`), `engine_relay.go` (loop kandidat), `engine.go` (identitas per panggilan) | `TestRelay_CredentialFailoverTriesTheNextAccount`, `TestRelay_CredentialsAreTriedBeforeTheNextModel` |
| F2 | `engine_failure.go` (`FailureClass`, `translateCallError`), `errors.go` (`UPSTREAM_REJECTED`), `failoverWorthy` | `TestRelay_RequestScopedRefusalIsReturnedToTheClient`, `TestRelay_FailureClassDecidesWhetherTheChainContinues` |
| F3 | `engine.go` (`firstErr`/`lastErr`/`preCallErr`), `engine_failure.go` (`finalError`), `provider_node.go` + `provider_node_combo_guard.go`, `error.ts`/`playground.ts` (kode di enum panel) | probe 2/3, `TestRelay_ExhaustedChainReportsFirstStatusAndLastMessage`, `TestNodeService_DeleteRefusesWhileAComboReferencesTheNode` |
| F4 | `upstream_key_health.go` (kelas + tangga backoff), `upstream_endpoint_key_reads.go` (`NextKeySkipping` LRU), `selection_health.go` (`RecordFailure` per kelas) | probe 5, baris parkir di `TestRelay_FailureClassDecidesWhetherTheChainContinues`, test domain `upstream_key_health_test.go` |

Dua hal yang baru terukur saat implementasi, keduanya memperketat dokumen ini:

1. **Probe 1 §2.4 tidak bisa lulus bahkan setelah perbaikan.** Kegagalannya per-model, dan
   status per-model menjawab akun kedua persis seperti akun pertama, jadi tidak ada failover
   kredensial yang bisa membalikkannya. Test permanennya menolak kredensial akun pertama lewat
   header `Authorization` dan menegaskan `ep-2` yang melayani (`calls=2`).
2. **Guard delete F3 harus memeriksa prefix, bukan hanya id.** `combo_order.go` menerima member
   yang ditulis dengan prefix node (`oczen/...`), jadi guard yang hanya mengenal id akan
   meloloskan member ber-prefix; `provider_node_combo_guard_test.go` mengunci kedua ejaan itu.

Deviasi yang sengaja dipertahankan (sama dengan changelog SPEC-API): parkir per key, bukan per
pasangan key+model (`modelLock_${model}` di reference); aturan teks reference ("no credentials",
"improperly formed request", ...) tidak diport, kelas ditentukan dari status; media/embeddings
tetap satu panggilan tanpa rantai, dengan kelas yang sama diturunkan dari statusnya; cursor
endpoint Redis maju per panggilan `SelectNext`, sehingga satu request yang mencoba dua kredensial
memajukan rotasi dua langkah.

Angka yang diukur: `go test -race -count=1 ./...` PASS, 5m45s untuk 17 entri modul (15 paket
ber-test, 2 tanpa test);
himpunan test failover baru 17 PASS (`-run 'TestRelay_(CredentialFailover|UnroutableMember|UnresolvableMember|RequestScopedRefusal|KeysRotate|CredentialsAreTried|ExhaustedChainReports|FailureClassDecides)'`);
himpunan test probe/guard service 17 PASS (`-run 'TestNodeService_Delete|TestEndpointService_Test'`);
`go-lint.sh` PASS (vet, vet `-tags=integration`, gofmt, staticcheck keduanya, golangci-lint 0
issue), dengan satu peringatan baris pada `internal/repository/postgres/quota.go` (222 baris,
pekerjaan aktor lain yang sedang berjalan, bukan pass ini); `go-headers.sh` PASS (837 berkas);
`contract-drift.sh` PASS (12 kode sepakat); `panel-check.sh` PASS (prettier, eslint,
svelte-check, vitest, vite build).

Commit: `04e422e` (spec + kode + test, 38 berkas, 1541 insertions / 598 deletions), dokumen ini
di commit kedua.

### 8.1 Gerbang pass implementasi (antislop, DURING & AFTER)

Pass ini menyentuh dua berkas `app-ui` (`error.ts`, `playground.ts`) tetapi tidak ada layar yang
dibangun atau diubah, jadi checklist 15 item antislop-ui (tap target, kontras, motion) tidak
berlaku; bukti untuk kedua berkas itu adalah gate panel. Empat blok tetap dilaporkan seluruhnya.

**Block 1: Hard Gate (17 item, semua harus "tidak")**

| Item | Jawaban | Bukti / alasan |
|---|---|---|
| R-02 em dash | **tidak** | `grep -cP '\x{2014}\|\x{2013}' docs/DRAFT/028-FALLBACK-CREDENTIAL-PARITY.md` = 0; baris yang ditambahkan pass ini di `docs/SPEC-API/001-SPEC-API.md` (§7.5) dinormalisasi ke titik dua. Em dash yang tersisa di berkas lain milik pass sebelumnya: dilaporkan, tidak diubah |
| R-03 overflow mobile | tidak berlaku | Tidak ada UI yang dibuat; dua berkas app-ui yang disentuh menambah satu anggota enum dan satu kalimat fallback |
| R-17 angka tanpa sumber | **tidak** | Setiap angka di §8 berasal dari perintah yang dikutip (test dan gate di baris di atas) |
| R-18 testimoni fiktif | tidak berlaku | Tidak ada testimoni |
| R-23 aset tanpa instruksi | tidak berlaku | Tidak ada aset visual yang dibuat |
| R-24 navigasi mati | tidak berlaku | Tidak ada navigasi |
| R-25 kontras | tidak berlaku | Tidak ada teks ter-render |
| R-26 kontrol mati | tidak berlaku | Tidak ada kontrol |
| R-27 state UI | tidak berlaku | Tidak ada UI berdata |
| R-28 FAQ | tidak berlaku | Tidak ada FAQ |
| R-32 keyboard | tidak berlaku | Tidak ada UI |
| R-33 patch via script | **ya, didiskalikan** | Dua suntingan batch dijalankan dengan skrip: satu `python3` heredoc memindahkan dua helper upstream dari `engine_failover_acceptance_test.go` ke `engine_failover_fixture_test.go`, dan beberapa penggantian kata/import dengan `sed -i`. Setiap hasil dibaca ulang dan `gofmt`/`go vet`/`go test` membuktikan hasilnya; tidak ada tabel dokumen yang disunting lewat skrip |
| R-34 tema | tidak berlaku | Tidak ada tema |
| R-35 dijalankan sebelum diserahkan | tidak berlaku untuk UI | Tidak ada UI. Yang dijalankan: `go-lint.sh`, `go-test.sh`, `go-headers.sh`, `contract-drift.sh`, `panel-check.sh` (semua PASS; angka di §8) |
| R-36 klaim fabrikasi | **tidak** | Setiap klaim menyitir berkas yang bisa dibuka; tidak ada klaim keamanan/compliance |
| R-37 arah desain | tidak berlaku | Tidak ada UI yang dibangun |
| R-38 konten fabrikasi | **tidak** | Sitasi reference dari checkout `39e36d3d` (dengan `21583c03..39e36d3d` diukur kosong atas lima berkas failover), sitasi app-serv dari working tree, output test dan gate dikutip apa adanya |

**Block 2: Purpose-Gate (12 item)**

Seluruhnya **tidak berlaku** dengan satu alasan yang sama: keluaran pass ini kode Go, test, dua
baris skema TypeScript, dan dokumen; tidak ada gradien, ikon, tipografi, latar bermotif, panah,
badge, glassmorphism, shadow, glow, kartu, animasi, atau ilustrasi. R-31 dijalankan pada tingkat
isi: setiap temuan §8 menuliskan sebabnya, dan setiap deviasi menuliskan alasannya.

**Block 3: Liveliness (7 item)**

Tidak berlaku seluruhnya: tidak ada layar, jadi tidak ada dial ENERGY/RHYTHM/MOTION, focal point,
whitespace, accent, atau motif identitas yang bisa dinilai.

**Block 4: Craftsmanship & Quality Locks (14 item)**

| Item | Jawaban | Bukti / alasan |
|---|---|---|
| C-1 intentionality | **tidak** | Setiap keputusan kode berasal dari temuan yang terukur (§3) atau aturan reference yang disitir; tidak ada "default AI" |
| C-2 functional completeness | tidak berlaku | Tidak ada elemen interaktif |
| C-3 content-driven composition | **tidak** | Struktur mengikuti F1-F4 dan hasil test, bukan template |
| C-4 resilience | **tidak** | Failover, parkir, dan guard delete adalah aturan resilience; test menutup jalur gagal (probe 2/3, kelas 4xx, guard kombo) |
| C-5 evidence over claims | **tidak** | Tidak ada testimoni/statistik tanpa sumber |
| R-05 template layout | tidak berlaku | Tidak ada layout |
| R-11 radius | tidak berlaku | Tidak ada UI |
| R-15 CTA | tidak berlaku | Tidak ada CTA |
| R-16 buzzwords | **tidak** | Tidak ada "AI Powered", "Seamless", "Revolutionary" di teks |
| R-20 identitas visual | tidak berlaku | Tidak ada visual |
| R-21 dark mode | tidak berlaku | Tidak ada tema |
| R-29 palet | tidak berlaku | Tidak ada warna |
| R-30 klon produk | tidak berlaku | Tidak ada visual |
| R-31 alasan tertulis | **tidak** (lulus) | Setiap temuan §8 dan setiap deviasi menulis alasan |

Tidak ada item yang FAIL. Sama seperti §7.1: item UI dijawab "tidak berlaku" dengan alasannya,
bukan "PASS".
