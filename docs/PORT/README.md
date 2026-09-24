## PORTING REFERENCE

1. REFERENCE
- https://github.com/decolua/9router | home/rusmanadodi/apps/9router

2. Porting:
Yang di ammbil portingan ke Go (Golang, dan Svelte UI:

# CONFIGURE
- Endpoint & Key
- Provider
- Combo & Vision
- Media Provider

# OBSERVE
- Usage
- Quota Tracker
- Console Log

# OPTIMIZE
- Token Saver
- Skill

# Developer (None Porting)
- Playground Chat
- API Docs
- Changelog

# SYSTEM
- Proxy Pools
- Setting

3. Desicion keputusan design (sengaja tidak diporting, 2026-09-24):
- **Tunnel dan Tailscale** pada Endpoint & Key: paparan Cloudflare Tunnel, Tailscale Funnel, gerbang
  keamanannya, dan tujuh route `/api/tunnel/*` tidak diporting (F3 `001-PORT-ENDPOINT-KEYS.md`).
  Operator yang butuh paparan remote menjalankan tunnel sendiri di depan gateway; panel hanya
  menampilkan alamat dasar read-only.
- **Pengikatan mesin dan auto-provision kunci gateway**: kunci kita opaque dan sah di gateway mana pun
  yang berbagi database, dan panel tidak membuat kunci pertama sendiri (F4). Keduanya penyimpangan
  sengaja, dicatat di SPEC-API §7.3.
- **"Upstream endpoints" bukan isi halaman Endpoint & Key**: endpoint beserta kuncinya hidup di halaman
  Provider, tempat reference menyimpannya sebagai kartu Connections (keputusan owner 2026-09-24,
  SPEC-UI §6.2/§6.3). Generalisasi 1-akun-1-endpoint berisi 1..N kunci tetap berlaku (SPEC-API §7.5).
