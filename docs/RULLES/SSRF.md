# Strict Security Coding Rules — Vulnerability Prevention Playbook

This document defines **mandatory, non-negotiable rules** an AI coding assistant MUST follow when writing, reviewing, or fixing code that handles user-influenced network requests, input, or output. Each vulnerability category has: a real-world **Case**, its **Root Cause**, **Mandatory Rules**, an explicit **Blacklist** of prohibited patterns, and **Mandatory Test Design** requirements. New categories are appended below as new `##` sections.

---

## SSRF (Server-Side Request Forgery)

### Case

A seemingly harmless endpoint accepts a URL and fetches it server-side:

```
POST /fetch-image
{"url": "https://example.com/logo.png"}
```

Nothing looks dangerous — the server just fetches an image from a given URL. But if an attacker substitutes the URL:

```
POST /fetch-image
{"url": "http://169.254.169.254/latest/meta-data/"}
```

A server that does not validate its request destination can end up hitting the cloud provider's **metadata endpoint** (AWS/GCP/Azure/Alibaba), potentially leaking internal credentials, IAM tokens, or other secrets — turning an "innocent" fetch feature into a pivot into the internal network.

### Root Cause

The server blindly trusts a user-supplied destination (URL, host, or IP) and forwards it to an outbound HTTP client without validating **where the request actually resolves to and lands**.

### Mandatory Rules (MUST Comply)

1. **MANDATORY — Allowlist First.** Outbound destinations MUST be validated against an explicit allowlist of permitted hosts/domains. A blocklist alone is **NOT** sufficient as the primary control — it is defense-in-depth only, never the main defense.
2. **MANDATORY — Block Reserved/Internal IP Ranges.** Requests MUST be rejected when the resolved IP falls in: loopback (`127.0.0.0/8`, `::1`), private ranges (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`), link-local (`169.254.0.0/16`, `fe80::/10`), and known cloud metadata addresses (`169.254.169.254`, `metadata.google.internal`, `100.100.100.200`, `fd00:ec2::254`, etc.).
3. **MANDATORY — Resolve, Validate, THEN Connect (Anti DNS-Rebinding).** DNS MUST be resolved once, the resolved IP validated against the rules above, and the connection made **to that validated IP** — not re-resolved at connection time. This prevents TOCTOU DNS-rebinding attacks where a domain resolves to a safe IP during validation and an internal IP during the actual request.
4. **MANDATORY — Protocol Restriction.** Only `http://` and `https://` are permitted. Schemes such as `file://`, `ftp://`, `gopher://`, `dict://`, `sftp://`, and `jar://` MUST be rejected outright.
5. **MANDATORY — Revalidate Every Redirect Hop.** Redirects MUST NOT be followed blindly. Each redirect target MUST pass through the full validation pipeline (rules 1–4) again, and the total redirect count MUST be capped.
6. **MANDATORY — Reject IP Obfuscation Tricks.** Validation MUST normalize and catch encoded/obfuscated IP forms before checking, including: decimal (`2130706433`), octal (`017700000001`), hexadecimal (`0x7f000001`), IPv6-mapped IPv4 (`::ffff:127.0.0.1`), and userinfo tricks (`http://expected-host@evil.com`).
7. **MANDATORY — Timeouts & Response Size Limits.** Outbound requests MUST have a strict connect/read timeout and a maximum response size, to limit abuse against slow or oversized internal responses.
8. **MANDATORY — No Raw Response Passthrough.** The raw body/headers of the fetched resource MUST NOT be reflected back to the caller unfiltered, to avoid leaking internal response details (banners, headers, error traces).

### Explicit Prohibited Patterns (Blacklist)

- Passing user-supplied input directly into an HTTP client (`fetch`, `axios`, `requests`, `curl`, `http.get`, etc.) with no validation step.
- Validating the **string** the user typed (e.g. checking it "looks like" a public domain) instead of validating the **resolved IP** actually connected to.
- Using blocklist-only validation with no allowlist.
- Following redirects without re-running validation on the new target.
- Trusting a `Host` header from the client as the request destination.

### Mandatory Test Design (Parameterized, Anti-Gaming)

Per the project's TDD rules, SSRF-prevention logic MUST be verified with a **parameterized test table** covering, at minimum, these variations in a single test run:

| Input | Expected |
|---|---|
| Valid public URL (e.g. `https://example.com/logo.png`) | ALLOWED |
| Loopback (`http://127.0.0.1`, `http://localhost`, `http://[::1]`) | BLOCKED |
| Cloud metadata (`http://169.254.169.254/latest/meta-data/`) | BLOCKED |
| Private range (`http://10.0.0.5`, `http://192.168.1.1`) | BLOCKED |
| Obfuscated IP (`http://2130706433`, `http://0x7f000001`) | BLOCKED |
| Non-HTTP scheme (`file:///etc/passwd`, `gopher://...`) | BLOCKED |
| External URL redirecting to an internal IP | BLOCKED (on redirect revalidation) |

### Mandatory Output Order

When implementing or fixing SSRF-related code, the response MUST follow:
1. **Threat Analysis** — attack surface, which internal targets are reachable, which rule(s) above apply.
2. **Implementation** — the actual validation/allowlist logic (never a hardcoded check for the one malicious URL given in an example).
3. **Compliance Self-Check** — confirms allowlist is enforced, resolved IP (not raw string) is validated, redirects are revalidated, and the parameterized test table above is covered.

---

*(Next sections — e.g. XSS, SQL Injection, IDOR — will be appended below as new `##` headings.)*
