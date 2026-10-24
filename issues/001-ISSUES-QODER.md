# 9Router - Qoder PAT Stability Patches

> Patches ini ditujukan untuk developer yang menjalankan 9Router sendiri dan mengalami masalah koneksi Qoder PAT (Personal Access Token) yang tidak stabil - seperti PAT selalu error, sering 10605, lock terlalu lama, atau test model timeout.
>
> Semua patch dilakukan di file bundle JavaScript milik 9Router. Nama file (chunk) bisa berbeda antar versi - selalu gunakan **pencarian pola teks** untuk menemukan lokasi yang tepat, jangan pakai nama file.

---

## ⚠️ Backup Dulu

```bash
# Windows
xcopy /E /I ".next-cli-build\server\chunks" ".next-cli-build\server\chunks_backup"

# Linux / macOS
cp -r .next-cli-build/server/chunks .next-cli-build/server/chunks_backup
```

Lokasi folder chunks:
- **Windows:** `%APPDATA%\npm\node_modules\9router\app\.next-cli-build\server\chunks\`
- **macOS/Linux:** `~/.npm-global/lib/node_modules/9router/app/.next-cli-build/server/chunks/`

Cara cari file berdasarkan isi:
```bash
# Windows (PowerShell)
Get-ChildItem *.js | Select-String -Pattern "POLA"

# Linux / macOS
grep -rl "POLA" *.js
```

---

## Patch 1 - Test Connection PAT Selalu Gagal

**Masalah:** Klik test connection PAT di UI → selalu error merah, padahal token valid.

**Sebab:** Ada bug di kode test connection Qoder - variabel `provider` dipakai langsung tapi tidak didefinisikan di scope fungsi itu. Seharusnya `a.provider` (dari parameter fungsi).

**Akibat:** JavaScript throw `ReferenceError: provider is not defined`, test selalu gagal, PAT tidak pernah jadi "active".

**Solusi:**
```bash
grep -rl '"qoder-cn"===provider?' *.js
```
```
CARI:    "qoder-cn"===provider?
GANTI:   "qoder-cn"===a.provider?
```

---

## Patch 2 - PAT Dikirim ke Server yang Salah

**Masalah:** PAT jauh lebih sering kena error 10605 (queued) dibanding OAuth, padahal akun sama-sama valid.

**Sebab:** Setelah PAT di-exchange ke `jt-` token, 9Router cek apakah token `jt-` → kalau iya, routing ke variabel `i` (server cadangan `api2.qoder.sh`). OAuth langsung ke variabel `h` (server utama `api3.qoder.sh`). Server cadangan lebih sering penuh.

**Akibat:** PAT selalu masuk server yang lebih sibuk → lebih sering 10605 → di-lock → makin jarang dipakai → lingkaran setan.

**Solusi:**
```bash
grep -rl 'startsWith("jt-"))?i:h}' *.js
```
```
CARI:    startsWith("jt-"))?i:h}
GANTI:   startsWith("jt-"))?h:h}
```
> Verifikasi dulu bahwa `h` memang `api3` dan `i` memang `api2` di file Anda.

---

## Patch 3 - Cooldown 403 Terlalu Lama

**Masalah:** Setelah PAT kena error 403 (termasuk 10605 yang dibungkus HTTP 403), PAT hilang dari rotasi selama 2 menit.

**Sebab:** Konfigurasi cooldown HTTP 403 di-set 120.000ms (2 menit). Padahal Qoder sendiri bilang retry setelah ~30 detik.

**Akibat:** Satu PAT error → lock 2 menit → request pindah ke PAT lain → PAT lain juga error → lock 2 menit → akhirnya semua PAT terkunci bersamaan.

**Solusi:**
```bash
grep -rl '{status:403,cooldownMs:12e4}' *.js
```
> Biasanya ditemukan di **11 file** - patch semua.
```
CARI:    {status:403,cooldownMs:12e4}
GANTI:   {status:403,cooldownMs:3e4}
```

---

## Patch 4 - Cooldown 404 Terlalu Lama

**Masalah:** Request ke model yang tidak ada → koneksi di-lock 2 menit, padahal ini bukan masalah koneksi.

**Sebab:** Cooldown HTTP 404 juga di-set 120.000ms (2 menit), sama seperti 403.

**Akibat:** Salah kirim nama model → koneksi itu terkunci 2 menit untuk semua model, bukan cuma model yang 404.

**Solusi:**
```bash
grep -rl '{status:404,cooldownMs:12e4}' *.js
```
> Patch semua file yang ditemukan.
```
CARI:    {status:404,cooldownMs:12e4}
GANTI:   {status:404,cooldownMs:1e4}
```

---

## Patch 5 - Error 10605 Bikin PAT Unavailable Permanen

**Masalah:** Setelah Qoder server busy sesaat (10605), PAT di database berubah jadi `testStatus: "unavailable"` dan tidak pernah auto-recover.

**Sebab:** Saat koneksi error, 9Router menulis `testStatus: "unavailable"` ke database tanpa cek apakah error-nya temporary atau permanent. Error 10605 (queue/throttle, hilang dalam 30 detik) diperlakukan sama seperti error fatal.

**Akibat:** PAT yang sehat jadi "unavailable" permanen → 9Router skip PAT itu untuk semua request berikutnya → makin sedikit PAT yang aktif → sisanya makin terbebani → cascade failure.

**Solusi:**
```bash
grep -rl 'testStatus:"unavailable",lastError:s,errorCode:b,lastErrorAt:new Date' *.js
```
```
CARI:
  updateProviderConnection(a,{...t,testStatus:"unavailable",lastError:s,errorCode:b,lastErrorAt:new Date().toISOString(),backoffLevel:o??q})

GANTI:
  updateProviderConnection(a,{...t,...(String(c||"").includes("10605")?{}:{testStatus:"unavailable"}),lastError:s,errorCode:b,lastErrorAt:new Date().toISOString(),backoffLevel:o??q})
```
> ⚠️ File ini minified - hati-hati kutip. Setelah edit, cek syntax: `node -c namafile.js`

---

## Patch 6 - Error 10605 Dianggap Billing Error

**Masalah:** Saat Qoder sedang busy, request langsung distop dengan "billing block" - tidak ada retry atau fallback ke koneksi lain.

**Sebab:** Ada fungsi yang mendeteksi apakah error adalah billing problem. Fungsi ini memasukkan kode `10605` ke dalam daftar billing error codes - padahal 10605 itu queue/throttle temporary, bukan masalah billing.

**Akibat:** Request yang seharusnya bisa di-retry ke PAT lain langsung dibatalkan. User lihat error, padahal PAT lain mungkin masih available.

**Solusi:**
```bash
grep -rl '"10605"===c)return!0' *.js
```
Ubah **2 tempat** di file yang sama:
```
CARI:    if("110"===c||"112"===c||"10605"===c)return!0
GANTI:   if("110"===c||"112"===c)return!0

CARI:    return/"code"\s*:\s*"(112|10605)"/.test(a)
GANTI:   return/"code"\s*:\s*"(112)"/.test(a)
```

---

## Patch 7 - Backoff Escalation Terlalu Agresif

**Masalah:** Setelah beberapa error berturut-turut, koneksi di-lock semakin lama - bisa sampai 5 menit per error, dan cap absolutnya 30 menit.

**Sebab:** 9Router pakai exponential backoff dengan 3 variabel global:
- `EQ.base = 2000ms` - interval awal
- `EQ.max = 300.000ms` (5 menit) - batas atas exponential backoff
- `fh = 1.800.000ms` (30 menit) - cap absolut untuk semua cooldown

Jadi kalau PAT kena 5x error berturut: 2s → 4s → 8s → 16s → 32s → ... max 5 menit per lock. Dan cap absolutnya 30 menit - artinya satu kali lock bisa tahan sampai 30 menit.

**Akibat:** Sekali kena error berturut, PAT bisa terkunci sangat lama. Restart 9Router pun tidak cukup kalau backoff level sudah tersimpan di DB.

**Solusi:**
```bash
grep -rl 'f={base:2e3,max:3e5,maxLevel:15},g=3e4,h=18e5' *.js
```
> Biasanya ditemukan di **11 file** - patch semua.
```
CARI:    f={base:2e3,max:3e5,maxLevel:15},g=3e4,h=18e5
GANTI:   f={base:1e3,max:6e4,maxLevel:15},g=3e4,h=6e4
```

| Nilai | Sebelum | Sesudah |
|---|---|---|
| Base backoff | 2 detik | 1 detik |
| Max backoff | 5 menit | 60 detik |
| Cap absolut | 30 menit | 60 detik |

---

## Patch 8 - PAT Baru Tidak Otomatis Dapat userId

**Masalah:** Setiap add PAT baru di UI 9Router, field `userId` di database selalu kosong. Harus diisi manual pakai script.

**Sebab:** Fungsi test connection PAT cuma exchange token (`pt-` → `jt-`) dan cek apakah berhasil (`ok`). Tidak ada langkah tambahan untuk mengambil `userId` dari Qoder API.

**Akibat:** Tanpa `userId`, 9Router tidak bisa membuat COSY signature yang valid → Qoder tidak mengenali request sebagai IDE client → request masuk antrian prioritas rendah (`p3`) → error 10605 terus-menerus, meskipun server tidak sibuk.

**Solusi:**
```bash
grep -rl 'return{valid:f.ok,error:f.ok?null:"Invalid Personal Access Token"}' *.js
```
```
CARI:
  f=await v(c,{method:"POST",headers:{"Content-Type":"application/json",Accept:"application/json","Cosy-Version":"1.0.1","Cosy-ClientType":"5"},body:JSON.stringify({personal_token:e})},b);return{valid:f.ok,error:f.ok?null:"Invalid Personal Access Token"}

GANTI:
  f=await v(c,{method:"POST",headers:{"Content-Type":"application/json",Accept:"application/json","Cosy-Version":"1.0.1","Cosy-ClientType":"5"},body:JSON.stringify({personal_token:e})},b);if(f.ok){try{let g=await f.clone().json(),h=g.token||g.data?.token;if(h){let i=await v("https://openapi.qoder.sh/api/v1/userinfo",{headers:{Authorization:"Bearer "+h,Accept:"application/json"}},b);if(i.ok){let j=await i.json();if(j.id||j.userId)return{valid:true,newTokens:{providerSpecificData:{userId:j.id||j.userId,authMethod:"pat"}}}}}}catch(g){}}return{valid:f.ok,error:f.ok?null:"Invalid Personal Access Token"}
```

Setelah patch ini: add PAT baru → test connection → `userId` otomatis terisi.

---

## Fix Database - userId PAT Lama yang Kosong

**Masalah:** PAT yang sudah ada sebelum Patch 8 tetap tidak punya `userId`.

**Sebab & Akibat:** Sama seperti Patch 8 - tanpa userId, COSY signature gagal, Qoder treat sebagai CLI client, masuk queue p3, error 10605 terus.

**Diagnosa:**
```javascript
const db = require('better-sqlite3')('/path/to/9router/db/data.sqlite');
const conns = db.prepare('SELECT data FROM providerConnections').all();
conns.filter(c => JSON.parse(c.data).apiKey?.startsWith('pt-')).forEach(c => {
    const d = JSON.parse(c.data);
    console.log(d.apiKey.substring(0,15), '| userId:', d.providerSpecificData?.userId || 'KOSONG!');
});
```

**Fix:** Untuk setiap PAT yang userId-nya kosong:
1. Exchange PAT: `POST https://openapi.qoder.sh/api/v1/jobToken/exchange` body `{"personal_token":"pt-xxx"}`
2. Ambil userId: `GET https://openapi.qoder.sh/api/v1/userinfo` header `Authorization: Bearer jt-xxx`
3. Simpan `response.id` ke `providerSpecificData.userId` di database

---

## Auto-Warmup - Inject ke cli.js

**Masalah:** Setelah restart 9Router, test model timeout atau gagal karena cache internal kosong.

**Sebab:** 9Router menyimpan model list dan `jt-` token cache di memory. Setelah restart, semua hilang. Request pertama harus fetch dari nol - dan kalau banyak request parallel masuk sebelum cache ready, semua gagal.

**Akibat:** Setelah setiap restart, harus manual jalankan `node warmup.js` - kalau lupa, koneksi error.

**Solusi:** Inject fungsi `runAutoWarmup()` ke `cli.js` (file utama 9Router) supaya warmup otomatis jalan setiap kali 9Router start.

**File:** `cli.js` di root folder 9Router
- Windows: `%APPDATA%\npm\node_modules\9router\cli.js`
- macOS/Linux: `~/.npm-global/lib/node_modules/9router/cli.js`

**Backup dulu:** `copy cli.js cli.js.backup`

**Step 1 - Tambahkan fungsi ini SEBELUM baris `// Start server immediately`:**

```javascript
function runAutoWarmup(warmupPort) {
  const http = require("http");
  const warmupApiKey = (() => {
    try {
      const dbPath = path.join(getAppDataDir(), "db", "data.sqlite");
      const sqliteDir = path.join(getAppDataDir(), "runtime", "node_modules", "better-sqlite3");
      if (!fs.existsSync(dbPath) || !fs.existsSync(sqliteDir)) return null;
      const sqliteDb = require(sqliteDir)(dbPath);
      const row = sqliteDb.prepare("SELECT key FROM apiKeys WHERE isActive = 1 LIMIT 1").get();
      return row?.key || null;
    } catch { return null; }
  })();
  if (!warmupApiKey) return;
  const warmupReq = (opts, body) => new Promise(resolve => {
    const req = http.request({ hostname: "127.0.0.1", port: warmupPort, ...opts,
      headers: { Authorization: `Bearer ${warmupApiKey}`, "Content-Type": "application/json", ...opts.headers }
    }, r => { let d = ""; r.on("data", c => d += c); r.on("end", () => resolve({ status: r.statusCode })); });
    req.on("error", () => resolve({ status: 0 }));
    req.setTimeout(15000, () => { req.destroy(); resolve({ status: 0 }); });
    if (body) req.write(body); req.end();
  });
  setImmediate(async () => {
    try {
      const dbPath = path.join(getAppDataDir(), "db", "data.sqlite");
      const sqliteDir = path.join(getAppDataDir(), "runtime", "node_modules", "better-sqlite3");
      const sqliteDb = require(sqliteDir)(dbPath);
      const conns = sqliteDb.prepare("SELECT id, data FROM providerConnections").all();
      const qc = conns.filter(c => {
        try { const d = JSON.parse(c.data); return d.apiKey?.startsWith("pt-") || d.accessToken?.startsWith("dt-"); }
        catch { return false; }
      });
      if (qc.length === 0) return;
      // Step 1: Fetch model list → populate in-memory cache
      await Promise.all(qc.map(c => warmupReq({ path: `/api/providers/${c.id}/models`, method: "GET" })));
      // Step 2: Reset PAT yang stuck unavailable akibat 10605
      for (const c of conns.filter(c => {
        try { return JSON.parse(c.data).apiKey?.startsWith("pt-"); } catch { return false; }
      })) {
        try {
          const d = JSON.parse(c.data);
          if (d.testStatus === "unavailable" && d.lastError?.includes("10605")) {
            sqliteDb.prepare("UPDATE providerConnections SET data = ? WHERE id = ?").run(
              JSON.stringify({ ...d, testStatus: "active", lastError: null, errorCode: null,
                lastErrorAt: null, backoffLevel: 0 }), c.id);
          }
        } catch {}
      }
      // Step 3: Pre-cache jt- tokens via lightweight chat requests
      const models = ["qd/efficient","qd/gfmodel","qd/dfmodel","qd/gmodel","qd/auto","qd/performance"];
      await Promise.all(models.map(m => {
        const b = JSON.stringify({model:m, stream:false, messages:[{role:"user",content:"hi"}], max_tokens:3});
        return warmupReq({path:"/v1/chat/completions", method:"POST",
          headers:{"Content-Length":Buffer.byteLength(b)}}, b);
      }));
    } catch {}
  });
}
```

**Step 2 - Cari `waitServerReady(port).then` (ada 2 tempat di cli.js), tambahkan `runAutoWarmup(port);` setelah `initTrayIcon();`:**

```javascript
waitServerReady(port).then(() => {
  initTrayIcon();
  runAutoWarmup(port);  // ← tambahkan
  ...
});
```

> ⚠️ API key dibaca dari tabel `apiKeys` (bukan `settings`). Query: `SELECT key FROM apiKeys WHERE isActive = 1 LIMIT 1`

---

## All-in-One Setup

Semua di atas (Patch 1-8 + Fix DB + Auto-warmup) bisa dilakukan otomatis dengan satu command. Setelah setiap **update 9Router**:

```bash
node 9r-setup.js
9router restart
```

| Part | Yang dilakukan |
|---|---|
| Part 1 | Apply Patch 1-8 ke semua chunk files |
| Part 2 | Fix userId PAT di database |
| Part 3 | Inject auto-warmup ke cli.js |

---

## Verifikasi

Setelah patch + restart:
- Test connection PAT → ✅ active
- Test model parallel → tidak timeout
- Qoder busy (10605) → PAT tetap active, lock max 30-60 detik
- PAT baru → otomatis dapat userId saat test connection
- Warmup otomatis saat start → tanpa manual script

---

## Ringkasan

| # | Masalah | Sebab | Akibat | Solusi |
|---|---|---|---|---|
| 1 | Test PAT crash | `provider` undefined | PAT tidak pernah active | Fix jadi `a.provider` |
| 2 | PAT sering 10605 | Routing ke server cadangan | Server sibuk, sering queue | Routing ke server utama |
| 3 | Lock 2 menit (403) | Cooldown 403 = 120s | PAT terkunci terlalu lama | Turunkan ke 30s |
| 4 | Lock 2 menit (404) | Cooldown 404 = 120s | Koneksi terkunci sia-sia | Turunkan ke 10s |
| 5 | PAT unavailable permanen | 10605 set DB unavailable | PAT di-skip selamanya | Skip unavailable untuk 10605 |
| 6 | Request distop saat busy | 10605 = billing error | Tidak ada fallback | Hapus 10605 dari billing list |
| 7 | Lock 5-30 menit | Backoff max terlalu tinggi | Error berulang = lock makin lama | Turunkan max ke 60s |
| 8 | PAT baru tanpa userId | Test conn tidak fetch userId | COSY gagal → queue p3 | Auto-fetch userId saat test |
| DB | PAT lama tanpa userId | userId belum pernah diisi | Sama seperti #8 | Exchange + userinfo + simpan |
| WU | Timeout setelah restart | Cache kosong | Model test gagal | Auto-warmup saat start |
