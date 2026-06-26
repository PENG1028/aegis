# Egress Trace Real Smoke — v1.8A-3

> Real egress trace smoke tests against Aegis v1.8A-2.
> 4 scenarios: managed route, unknown domain, public DNS, and (simulated) self resolution.

---

## Scenario 1: Managed Domain with Route

**Domain:** `lb-public-gw.smoke.test` (registered route with GatewayLink)

```bash
aegis safety trace-egress lb-public-gw.smoke.test
```

```
Egress Trace: lb-public-gw.smoke.test
  Resolved IPs:       (none)
  IP Classification:  public
  Managed Domain:     false
  Matched Route:      rt_faee40f4ced69486
  Current Node:       node_PENGSPC
  Target:             <SERVER_B_IP>:80
  Has Gateway Link:   true
  Gateway Link ID:    gwlink_smoke
```

**Verification:**
- Output shows the matched route, correct target, and gateway link
- No risks (public target with GatewayLink is safe)
- ✅ Trace correctly resolves route from domain

---

## Scenario 2: Unknown Domain (No DNS, No Route)

**Domain:** `completely-unknown-domain-12345.test`

```bash
aegis safety trace-egress completely-unknown-domain-12345.test
```

```
Egress Trace: completely-unknown-domain-12345.test
  Resolved IPs:       (none)
  IP Classification:  
  Managed Domain:     false
  Current Node:       node_PENGSPC
  Has Gateway Link:   false

  Risks:
    ℹ [UNKNOWN_DOMAIN] domain completely-unknown-domain-12345.test does not resolve

  Recommendation: check if the domain is correct or register it as a managed domain
```

**Verification:**
- No route matched, no managed domain, DNS fails
- `UNKNOWN_DOMAIN` risk with info severity
- Recommendation guides the user
- ✅ Trace handles unresolvable domains gracefully

---

## Scenario 3: Domain Resolving to Public IP (No Route)

**Domain:** `example.com`

```bash
aegis safety trace-egress example.com
```

```
Egress Trace: example.com
  Resolved IPs:       2606:4700:10::6814:179a, 2606:4700:10::ac42:93f3, 172.66.147.243, 104.20.23.154
  IP Classification:  public
  Managed Domain:     false
  Current Node:       node_PENGSPC
  Has Gateway Link:   false

  Risks:
    ⚠ [PUBLIC_DOMAIN_BOUNCE] example.com resolves to public IP 2606:4700:10::6814:179a with no Aegis route

  Recommendation: bind example.com using bind-http-domain to control egress
```

**Verification:**
- DNS resolves to multiple IPs (including IPv6)
- `PUBLIC_DOMAIN_BOUNCE` warning fires correctly
- Recommendation suggests binding the domain
- ✅ Trace correctly identifies public DNS resolution without Aegis route

---

## Scenario 4: Self Resolution (Simulated)

**Setup:** A route targeting the node's own IP was created as `lb-self.smoke.test`

```bash
aegis safety trace-egress lb-self.smoke.test
```

```
Egress Trace: lb-self.smoke.test
  Matched Route:      rt_06d68dcaa77ffffe
  ...
  Risks:
    ✗ [SELF_LOOP] route target 127.0.0.1 is the gateway itself — would cause loop
```

**Verification:**
- Route matched, target host is `127.0.0.1` (node's own IP)
- `SELF_LOOP` error fires correctly
- ✅ Trace detects self-targeting routes

---

## Coverage Summary

| Scenario | Data Source | Verdict |
|----------|------------|---------|
| 1. Known route with GatewayLink | Real Aegis route | ✅ |
| 2. Unknown domain (no DNS) | Random domain | ✅ |
| 3. Public domain no route | example.com | ✅ |
| 4. Self-targeting route | Real Aegis route | ✅ |
