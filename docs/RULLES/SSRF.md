# Strict SSRF Rules — Outbound Destination Control

## 1. Role Definition

You are acting as a **Senior Security Engineer** writing or reviewing any code that makes app-serv dial a destination. Your objective is that every outbound connection the process makes passes through one generalized egress guard, so the entire Server-Side Request Forgery class is closed by construction rather than by a list of blocked examples.

This protocol is **MANDATORY** for every egress surface: a new HTTP client, a new connector, a new probe, or a new fetcher is in scope the moment it can dial an address the process did not hardcode.

---

## 2. Global Mandatory Constraints (MUST Comply)

### 2.1 One Guard, Every Dial

- Every outbound dial goes through the process egress guard (`internal/netguard`) via the guarded client built once in `cmd/app-serv/egress_wiring.go`.
- A second client built beside it, or a connector that constructs its own `http.Client` when handed a nil one, is a **PROTOCOL VIOLATION** (draft 042 R07, R08). The composition root passes the guarded client; structural assertions in `cmd/app-serv` fail the build when a constructor is handed or builds an unguarded one.

### 2.2 Check the Resolved Address, Not the String

- A hostname check on the request URL is the pattern this rule forbids: DNS can answer differently on a second lookup, so the check must run on the **resolved IP** and **again at connect time** on the address the dialer is about to reach (`netguard.Guard.CheckHost`, then `Control` on the dialer).
- The two checks are one rule. Removing either half is a violation: resolve-time alone loses to rebinding, connect-time alone spends an upstream call on a destination that should have been refused.

### 2.3 Fail Closed on Ambiguous Resolution

- A name that resolves to several addresses is refused when **any one** of them is denied. Picking the address that passes is the rebinding shape the rule exists to defeat.

### 2.4 Two-Tier Destination Policy

- Tier one, **never a host**: link-local, multicast, reserved, and CGNAT ranges are refused outright. An operator allowlist cannot reopen them; `0.0.0.0/0` in `EGRESS_ALLOWED_TARGETS` still cannot dial the metadata address.
- Tier two, **private by default**: loopback and private ranges are refused unless the operator names them in `EGRESS_ALLOWED_TARGETS`. Default deny, allowlist to permit, malformed entries fail the boot.

### 2.5 Address-Spelling Hygiene

- IPv4-mapped IPv6 (`::ffff:127.0.0.1`) is unwrapped before any range check.
- Transition addresses carrying an embedded IPv4 address (NAT64, 6to4, Teredo) have that embedded address checked; the wrapper spelling never grants passage.

---

## 3. Per-Surface Rules

| Surface                        | Destination source                      | Rule                                                                                    |
| ------------------------------ | --------------------------------------- | --------------------------------------------------------------------------------------- |
| Endpoint probe                 | Operator-supplied validate URL          | `CheckHost` before the call; guarded client only                                        |
| Provider node model fetch      | Operator-supplied base URL              | Same as the probe; the guard rides the wiring, not the caller's discipline              |
| Proxy connectivity test        | Operator-supplied candidate proxy       | The proxy address itself is a dial destination and is guarded                           |
| OAuth token and identity calls | Registry URLs, never a request field    | Registry-sourced URLs are not exempt: they still ride the guarded client                |
| Quota fetch families           | Registry and configured hosts           | The shared package client carries the guarded transport (`quotafetch.UseEgressClient`)  |
| Qoder exchange and catalog     | Registry URLs plus operator credentials | The connector receives the guarded client; a nil client must fail rather than fall back |
| Data plane upstream calls      | Registry endpoints                      | The guarded client is the one the engine uses; redirects are never followed             |

---

## 4. Redirects and Chaining

- The shared clients set `CheckRedirect` to refuse following a redirect (`http.ErrUseLastResponse`). A 30x from an upstream is surfaced to the failure policy instead of being chased cross-host with the credentials the original request presented.
- A proxy does not exempt the destination: when a route is proxied, the destination is still validated before the dial, because the guard sees the proxy's address, never the destination's, otherwise.
- A settings read or a proxy URL that fails refuses the request rather than dialing direct; quietly bypassing a proxy an operator enabled is the failure the proxy setting exists to prevent.

---

## 5. Review and Test Duties

1. A new egress surface ships with: the guard in its dial path, the guarded client from the composition root, and a structural or behavioral test that fails when the wiring is missing.
2. Tests must discriminate: a test that passes with and without the guard proves nothing. Stage a destination the guard refuses (loopback, link-local) and assert the call is denied with the guard and only the guard's removal changes the answer.
3. The refusal message names the tier and the remedy (the operator's own proxy belongs in `EGRESS_ALLOWED_TARGETS`), because the panel shows it to an operator who can act on it.
4. Any exception to this file requires a dated entry in a DRAFT register naming the surface, the reason, and the compensating control.
