# Contract API v1

**Dokumen kontrak:** `001-CONTRACT-API-V1`  
**Format mesin:** `docs/CONTRACT/001-CONTRACT-API-V1.yaml`  
**Artefak served:** `app-serv/internal/handler/openapi.json`  
**Versi API:** `v1`  
**OpenAPI:** `3.1.0`

Dokumen YAML adalah sumber kontrak. `openapi.json` dihasilkan oleh `app-serv/tools/openapi-gen` dan di-embed ke binary `app-serv` pada `GET /api/v1/openapi.json`.

## 1. Aturan kontrak

### Base URL

```text
http://localhost:8080/api/v1
```

Host dan port dapat berubah melalui konfigurasi deployment. Prefix `/api/v1` tetap menjadi bagian dari path.

### Format JSON

- Field menggunakan `snake_case`.
- Timestamp menggunakan RFC3339 UTC.
- ID memakai string ber-prefix sesuai resource, misalnya `ep_`, `uky_`, `gky_`, `cmb_`, dan `prx_`.
- Nilai biaya memakai string desimal, misalnya `"0.0042"`.
- Nilai token memakai integer.
- Secret tidak dikembalikan setelah dibuat. Response hanya membawa `key_hint`, `token_hint`, atau flag seperti `has_password`.
- Field yang tidak tersedia dikirim sebagai `null` bila bentuk response mendefinisikan state tersebut; array kosong dikirim sebagai `[]`, bukan `null`.

### Security scheme

| Scheme | Header/cookie | Dipakai oleh |
|---|---|---|
| `sessionCookie` | Cookie `pannel_session` | Management API |
| `gatewayKey` | `Authorization: Bearer <gateway-key>` | Data plane |

Route publik tidak memerlukan credential. Data plane menolak request tanpa gateway key ketika `settings.security.require_api_key=true`.

## 2. Error mapping

### Management API

Bentuk response selalu:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "The request is invalid."
  }
}
```

`message` selalu berbahasa Inggris. Client memakai `code` untuk branching, bukan mencocokkan teks `message`.

| HTTP | Code | Arti |
|---:|---|---|
| 400 | `VALIDATION_ERROR` | Body, query, atau parameter tidak memenuhi kontrak. |
| 401 | `UNAUTHORIZED` | Session tidak ada, invalid, expired, atau revoked. |
| 403 | `FORBIDDEN` | Session valid tetapi tidak memiliki izin. |
| 404 | `NOT_FOUND` | Resource atau provider tidak ditemukan. |
| 405 | `METHOD_NOT_ALLOWED` | Method tidak terdaftar untuk path tersebut. |
| 409 | `CONFLICT` | Bentrok dengan uniqueness atau state saat ini. |
| 429 | `RATE_LIMITED` | Rate limit tercapai. Header `Retry-After` dapat disertakan. |
| 500 | `INTERNAL_ERROR` | Error internal yang tidak terduga. |
| 502 | `UPSTREAM_ERROR` | Provider upstream mengembalikan error yang tidak dapat dipulihkan. |
| 503 | `NO_PROVIDER_AVAILABLE` | Tidak ada endpoint/key yang dapat melayani request. |
| 504 | `UPSTREAM_TIMEOUT` | Panggilan ke upstream melewati batas waktu. |

### Data plane

Bentuk response mengikuti OpenAI-compatible error envelope:

```json
{
  "error": {
    "message": "The request is invalid.",
    "type": "invalid_request_error",
    "code": "VALIDATION_ERROR"
  }
}
```

| HTTP | Code | Type |
|---:|---|---|
| 400 | `VALIDATION_ERROR` | `invalid_request_error` |
| 400 | `MODEL_NOT_FOUND` | `invalid_request_error` |
| 400 | `PROVIDER_NOT_ROUTABLE` | `invalid_request_error` |
| 401 | `UNAUTHORIZED` | `authentication_error` |
| 429 | `RATE_LIMITED` | `rate_limit_error` |
| 500 | `INTERNAL_ERROR` | `server_error` |
| 502 | `UPSTREAM_ERROR` | `server_error` |
| 503 | `NO_PROVIDER_AVAILABLE` | `server_error` |
| 504 | `UPSTREAM_TIMEOUT` | `server_error` |

## 3. Endpoint catalog

Semua bentuk request, response, parameter, enum, dan reference schema yang normatif berada di YAML. Tabel berikut adalah indeks manual endpoint dan aturan operasional yang perlu dibaca oleh integrator.

### 3.1 System

| Method | Path | Auth | Success |
|---|---|---|---|
| GET | `/api/v1/health` | Public | `200 HealthResponse` |
| GET | `/api/v1/version` | Public | `200 SystemInfo` |

`health` melaporkan status service serta check database dan Redis. Status degraded memakai HTTP status yang sesuai.

### 3.2 Auth

| Method | Path | Auth | Success |
|---|---|---|---|
| POST | `/api/v1/auth/login` | Public | `204` dan session cookie |
| GET | `/api/v1/auth/status` | Public | `200 AuthStatusResponse` |
| POST | `/api/v1/auth/logout` | Session | `204` |
| POST | `/api/v1/auth/change-password` | Session | `204` |

`POST /auth/login` menerima `{ "password": "..." }`. Login dilindungi login limiter. Session cookie bersifat HttpOnly, ditandatangani, dan dapat dicabut melalui Redis.

### 3.3 Gateway keys

| Method | Path | Auth | Success |
|---|---|---|---|
| GET | `/api/v1/gateway-keys` | Session | `200 GatewayKeyList` |
| POST | `/api/v1/gateway-keys` | Session | `201 GatewayKeyResponse` |
| GET | `/api/v1/gateway-keys/{id}` | Session | `200 GatewayKeyResponse` |
| PATCH | `/api/v1/gateway-keys/{id}` | Session | `200 GatewayKeyResponse` |
| DELETE | `/api/v1/gateway-keys/{id}` | Session | `204` |

Plaintext key hanya muncul pada response create. Request berikutnya memakai `key_hint` untuk identifikasi.

### 3.4 Providers dan OAuth

| Method | Path | Auth | Success |
|---|---|---|---|
| GET | `/api/v1/providers` | Session | `200 ProviderListResponse` |
| GET | `/api/v1/providers/{provider_id}` | Session | `200 ProviderDetailResponse` |
| GET | `/api/v1/providers/{provider_id}/models` | Session | `200 ProviderModelList` |
| POST | `/api/v1/providers/{provider_id}/oauth/start` | Session | `200 OAuthStartResponse` |
| GET | `/api/v1/providers/{provider_id}/oauth/callback` | Public | `200 OAuthConnectResult` atau redirect browser |
| GET | `/api/v1/providers/{provider_id}/oauth/status` | Session | `200` |
| POST | `/api/v1/providers/{provider_id}/oauth/refresh` | Session | `200` |
| POST | `/api/v1/providers/{provider_id}/oauth/bulk` | Session | `201 BulkOAuthResponse` |

`GET /oauth/callback` memakai replay guard `state` single-use. Caller headless dapat mengirim `Accept: application/json`; browser menerima redirect ke panel.

`GET /providers` mendukung filter `category`, `routability`, `page`, dan `per_page`. `routability` bernilai `native` atau `connector`.

### 3.5 Provider nodes

| Method | Path | Auth | Success |
|---|---|---|---|
| GET | `/api/v1/provider-nodes` | Session | `200 ProviderNodeList` |
| POST | `/api/v1/provider-nodes` | Session | `201 ProviderNodeResponse` |
| GET | `/api/v1/provider-nodes/{id}` | Session | `200 ProviderNodeResponse` |
| PATCH | `/api/v1/provider-nodes/{id}` | Session | `200 ProviderNodeResponse` |
| DELETE | `/api/v1/provider-nodes/{id}` | Session | `204` |
| POST | `/api/v1/provider-nodes/{id}/test` | Session | `200 ProxyTestResponse` |

`type=openai-compatible` membutuhkan `api_type` `chat` atau `responses`. `base_url` hanya menerima URL absolute `http` atau `https`. Prefix yang bentrok dengan registry atau alias ditolak dengan `CONFLICT`.

### 3.6 Upstream endpoints dan keys

| Method | Path | Auth | Success |
|---|---|---|---|
| GET | `/api/v1/endpoints` | Session | `200 EndpointList` |
| POST | `/api/v1/endpoints` | Session | `201 EndpointResponse` |
| POST | `/api/v1/endpoints/bulk` | Session | `201 BulkEndpointResponse` |
| GET | `/api/v1/endpoints/{id}` | Session | `200 EndpointResponse` |
| PATCH | `/api/v1/endpoints/{id}` | Session | `200 EndpointResponse` |
| DELETE | `/api/v1/endpoints/{id}` | Session | `204` |
| POST | `/api/v1/endpoints/{id}/test` | Session | `200 EndpointTestResponse` |
| GET | `/api/v1/endpoints/{id}/keys` | Session | `200 EndpointKeyList` |
| POST | `/api/v1/endpoints/{id}/keys` | Session | `201 EndpointKeyResponse` |
| POST | `/api/v1/endpoints/{id}/keys/bulk` | Session | `201 BulkKeyResponse` |
| PATCH | `/api/v1/endpoints/{id}/keys/{key_id}` | Session | `200 EndpointKeyResponse` |
| DELETE | `/api/v1/endpoints/{id}/keys/{key_id}` | Session | `204` |

`POST /endpoints` menerima `auth_type` `api_key`, `oauth`, atau `no_auth`. Endpoint dengan `api_key` dapat memiliki beberapa key. Nilai key bersifat write-only.

Bulk endpoint dan bulk key bersifat all-or-nothing. Batch divalidasi penuh lalu diterapkan dalam satu transaksi.

### 3.7 Models

| Method | Path | Auth | Success |
|---|---|---|---|
| GET | `/api/v1/models` | Gateway key | `200 ModelListResponse` |
| GET | `/api/v1/models/catalog` | Session | `200 ModelList` |
| GET | `/api/v1/models/custom` | Session | `200 CustomModelList` |
| POST | `/api/v1/models/custom` | Session | `201` |
| DELETE | `/api/v1/models/custom/{id}` | Session | `204` |
| GET | `/api/v1/models/aliases` | Session | `200 AliasList` |
| PUT | `/api/v1/models/aliases` | Session | `200 AliasList` |
| GET | `/api/v1/models/disabled` | Session | `200 DisabledList` |
| PUT | `/api/v1/models/disabled` | Session | `200 DisabledList` |

Catalog mendukung filter `provider_id`, `capability`, dan `q`. Model disabled tidak muncul pada catalog routing.

### 3.8 Combos

| Method | Path | Auth | Success |
|---|---|---|---|
| GET | `/api/v1/combos` | Session | `200 ComboList` |
| POST | `/api/v1/combos` | Session | `201 ComboResponse` |
| GET | `/api/v1/combos/{id}` | Session | `200 ComboResponse` |
| PATCH | `/api/v1/combos/{id}` | Session | `200 ComboResponse` |
| DELETE | `/api/v1/combos/{id}` | Session | `204` |
| POST | `/api/v1/combos/{id}/test` | Session | `200 ComboTestResponse` |

`strategy` bernilai `fallback`, `round_robin`, atau `fusion`. `models` berisi reference model dan priority. Combo yang direferensikan alias tidak dapat dihapus sampai reference dibersihkan.

### 3.9 Vision adapter dan token saver

| Method | Path | Auth | Success |
|---|---|---|---|
| GET | `/api/v1/vision-adapter` | Session | `200 VisionAdapterResponse` |
| PUT | `/api/v1/vision-adapter` | Session | `200 VisionAdapterResponse` |
| GET | `/api/v1/token-saver` | Session | `200 TokenSaverResponse` |
| PUT | `/api/v1/token-saver` | Session | `200 TokenSaverResponse` |

Vision adapter hanya menerima model yang memiliki capability `vision`. Token saver memakai replacement body penuh. `rtk.filters=[]` berarti semua filter yang terdaftar dapat dipakai.

### 3.10 Proxy pools

| Method | Path | Auth | Success |
|---|---|---|---|
| GET | `/api/v1/proxies` | Session | `200 ProxyList` |
| POST | `/api/v1/proxies` | Session | `201 ProxyResponse` |
| PATCH | `/api/v1/proxies/{id}` | Session | `200 ProxyResponse` |
| DELETE | `/api/v1/proxies/{id}` | Session | `204` |
| POST | `/api/v1/proxies/test` | Session | `200 ProxyTestResponse` |
| POST | `/api/v1/proxies/{id}/test` | Session | `200 ProxyTestResponse` |

Password proxy tidak pernah dikembalikan. Response memakai `has_password`. Test yang gagal tetap `200` dengan `state:"fail"`; ini adalah hasil probe, bukan kegagalan transport API.

### 3.11 Media providers

| Method | Path | Auth | Success |
|---|---|---|---|
| GET | `/api/v1/media-providers` | Session | `200 MediaProviderList` |
| GET | `/api/v1/media-providers/{provider_id}` | Session | `200 MediaProviderDetail` |
| PATCH | `/api/v1/media-providers/{provider_id}` | Session | `200` |

Override disimpan per provider dan kind. `base_url` tidak boleh kosong setelah resolusi registry dan override selesai.

### 3.12 Usage, quota, logs, dan settings

| Method | Path | Auth | Success |
|---|---|---|---|
| GET | `/api/v1/usage/summary` | Session | `200 UsageSummaryResponse` |
| GET | `/api/v1/usage/timeseries` | Session | `200 UsageTimeseriesResponse` |
| GET | `/api/v1/usage/records` | Session | `200 UsageRecordList` |
| GET | `/api/v1/usage/records/{request_id}` | Session | `200 UsageRecordDetailResponse` |
| GET | `/api/v1/quotas` | Session | `200 QuotaWindowList` |
| GET | `/api/v1/quotas/{endpoint_id}` | Session | `200 QuotaEndpointDetail` |
| PUT | `/api/v1/quotas/{endpoint_id}` | Session | `200` |
| GET | `/api/v1/logs/requests` | Session | `200 LogList` |
| GET | `/api/v1/logs/requests/{request_id}` | Session | `200 LogDetailResponse` |
| DELETE | `/api/v1/logs/requests` | Session | `200 LogPurgeResponse` |
| GET | `/api/v1/logs/console` | Session | `200 ConsoleResponse` |
| DELETE | `/api/v1/logs/console` | Session | `204` |
| GET | `/api/v1/settings` | Session | `200 SettingsResponse` |
| PATCH | `/api/v1/settings` | Session | `200 SettingsResponse` |

List usage dan log memakai pagination. Filter waktu menggunakan RFC3339. Query tanpa range memakai bounded default window service.

### 3.13 Skills, OpenAPI, dan changelog

| Method | Path | Auth | Success |
|---|---|---|---|
| GET | `/api/v1/skills` | Session | `200 SkillsResponse` |
| GET | `/api/v1/openapi.json` | Session | OpenAPI JSON |
| GET | `/api/v1/changelog` | Session | `200 ChangelogResponse` |

`openapi.json` adalah artifact yang dihasilkan dari YAML, bukan file sumber manual.

### 3.14 Data plane

| Method | Path | Auth | Success |
|---|---|---|---|
| POST | `/api/v1/chat/completions` | Gateway key | `200` JSON atau SSE |
| POST | `/api/v1/messages` | Gateway key | `200` JSON atau SSE |
| POST | `/api/v1/responses` | Gateway key | `200` JSON atau SSE |
| GET | `/api/v1/models` | Gateway key | `200 ModelListResponse` |
| POST | `/api/v1/embeddings` | Gateway key | `200 EmbeddingsResponse` |
| POST | `/api/v1/messages/count_tokens` | Gateway key | `200 CountTokensResponse` |
| POST | `/api/v1/audio/speech` | Gateway key | `200` audio bytes atau JSON |
| POST | `/api/v1/audio/transcriptions` | Gateway key | `200` JSON atau text |
| GET | `/api/v1/audio/voices` | Gateway key | `200 VoiceList` |
| POST | `/api/v1/images/generations` | Gateway key | `200 MediaGenerationResponse` |
| POST | `/api/v1/videos/generations` | Gateway key | `200` |
| POST | `/api/v1/search` | Gateway key | `200 SearchResponse` |

Streaming menggunakan `text/event-stream`, `X-Accel-Buffering: no`, dan format SSE provider yang telah diterjemahkan ke wire format caller. Request data plane memakai `Authorization: Bearer <gateway-key>`.

`POST /messages/count_tokens` menghitung estimasi token tanpa memanggil upstream. `POST /audio/transcriptions` memakai multipart form. Bentuk rinci field dan response media berada di YAML.

## 4. Validasi kontrak

Dari module `app-serv`:

```bash
go run ./tools/openapi-gen
 go run ./tools/openapi-gen -check
 go test ./internal/handler/ -run 'TestOpenAPI' -count=1
```

Perintah pertama menghasilkan `app-serv/internal/handler/openapi.json` dari YAML. Mode `-check` gagal bila artifact committed berbeda dari hasil generate.

Test kontrak memeriksa:

- OpenAPI version `3.1.0`.
- Semua path memakai prefix `/api/v1`.
- Semua `$ref` lokal resolve ke schema atau response yang ada.
- Semua operation memiliki `operationId` unik.
- Semua operation memiliki response sukses dan error.
- Semua path parameter dideklarasikan sebagai required.
- Management error memakai `ManagementError`.
- Data-plane error memakai `DataPlaneError`.
- Route publik tidak membawa security requirement.

YAML lengkap, termasuk seluruh schema dan response reference, tersedia di [`001-CONTRACT-API-V1.yaml`](./001-CONTRACT-API-V1.yaml).
