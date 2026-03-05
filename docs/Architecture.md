# ROCKS Protocol Architecture

**ROCKS** — Relay-Oriented Cryptographic Key-addressed Service
A Decentralized Identity-Routed Web Protocol

---

## 1. Overview

ROCKS is a decentralized web protocol where a **relay mesh network** replaces the traditional internet infrastructure. There are no domain names, no DNS, no exposed server ports. Instead:

- Services and clients connect **outbound** to a relay mesh
- The mesh resolves **cryptographic identities** and routes traffic
- All connections pass through a **minimum of 3 relay hops**
- End-to-end **TLS 1.3** ensures relays are blind to content
- A **blockchain** provides identity registry, relay staking, and economic security

```
┌──────────┐                                              ┌──────────┐
│  Client  │──outbound──→  Relay Mesh  ←──outbound──      │ Service  │
│ (no open │              (3+ relays                      │ (no open │
│  ports)  │               ALWAYS)                        │  ports)  │
└──────────┘                                              └──────────┘
```

**Nothing is directly reachable.** Only relay nodes have public ports. Every other node — client or service — connects outbound to the mesh and is hidden behind it.

---

## 2. Design Principles

1. **Identity-first networking** — Addresses are cryptographic keys, not IP addresses or domains
2. **Relay-mediated always** — No direct connections, ever. Minimum 3 relay hops, always
3. **Blind relays** — Relays pass bytes, never see content. TLS 1.3 is end-to-end
4. **Identity on every request** — All requests carry a cryptographic identity. Anonymous requests are rejected
5. **Read is free, write is paid** — GET/HEAD are free by default; POST/PUT/PATCH/DELETE cost tokens (configurable per service)
6. **Economic security** — Validators are relays. Staking and slashing enforce honest behavior

---

## 3. Identity System

### 3.1 Single Key, Every Layer

Every entity in the ROCKS network — client, service, relay, validator — is identified by a single **ed25519 keypair**.

```
ed25519 private key (master identity)
    │
    ├──→ ROCKS address (bech32)         — URL addressing, on-chain identity
    ├──→ CometBFT validator key         — consensus voting (ed25519 native)
    ├──→ Cosmos SDK account key         — sign transactions (ed25519 native)
    ├──→ libp2p peer ID                 — mesh networking (ed25519 native)
    ├──→ TLS certificate pubkey         — self-signed, identity-verified
    └──→ X25519 (derived)              — TLS 1.3 ECDH key exchange
```

ed25519 is the native key type for CometBFT, Cosmos SDK, and libp2p. No key conversion or bridging is needed except the standard ed25519 → X25519 derivation for TLS key exchange.

### 3.2 Address Format

ROCKS addresses use **bech32 encoding** with a `rocks` human-readable prefix:

```
rocks1a3b5c7d9e1f2a4b6c8d0e2f4a6b8c0d2e4f6a8b0c2d4e6f8a0b2c4d6e8f
```

The address is derived directly from the ed25519 public key:

```
address = bech32("rocks", sha256(ed25519_pubkey)[:20])
```

### 3.3 URL Scheme

```
rocks://<rocks_address>/<path>[?query][#fragment]
```

Examples:

```
rocks://rocks1a3b5c7d9e.../profile
rocks://rocks1a3b5c7d9e.../api/users?limit=10
rocks://rocks1x9f8f72aa9.../blog/posts/42
```

### 3.4 Identity on Every Request

**Every request in the ROCKS network MUST carry a valid identity.** Requests without identity are rejected at the protocol level.

```
GET /profile ROCKS/1.0.0
identity: rocks1a3b5c7d9e...
signature: <ed25519 signature of request>
timestamp: 1704067200
content-length: 0
```

The `identity` header contains the sender's ROCKS address.
The `signature` header contains an ed25519 signature of the request line + headers + timestamp.
This prevents replay attacks and ensures non-repudiation.

**Why identity on every request:**
- Services know exactly who is calling them
- Rate limiting is per-identity, not per-IP
- Billing for write operations is per-identity
- Services can ban specific identities
- Audit trail for all network activity

---

## 4. Network Architecture

### 4.1 Node Roles

The ROCKS network has four logical roles. A single node can serve multiple roles.

#### Validator-Relay Node (public backbone)

**Validators ARE relays.** This is a dual role — the same staked nodes that run consensus also relay traffic.

Responsibilities:
- Run CometBFT consensus (validate blocks, vote on proposals)
- Relay traffic through the mesh (forward encrypted bytes)
- Maintain ARP table (identity → relay mapping)
- Accept inbound connections from clients and services
- **Only nodes with public ports** (port 4878 for relay, port 26656 for CometBFT p2p)
- Staked on-chain with slashable deposit

```
┌─────────────────────────────────┐
│     Validator-Relay Node        │
│                                 │
│  ┌───────────┐  ┌────────────┐  │
│  │ CometBFT  │  │  Relay     │  │
│  │ Consensus │  │  Service   │  │
│  │ (p:26656) │  │  (p:4878)  │  │
│  └───────────┘  └────────────┘  │
│  ┌───────────────────────────┐  │
│  │      libp2p Host          │  │
│  │   ed25519 identity        │  │
│  │   ARP table               │  │
│  │   DHT peer discovery      │  │
│  └───────────────────────────┘  │
└─────────────────────────────────┘
```

#### Service Node (hidden)

Hosts applications accessible via ROCKS protocol.

- **Exposes NO ports** — connects outbound to multiple relay nodes
- Registers on blockchain: identity, endpoint metadata, pricing
- Announces to relays: "Route traffic for my address to me"
- Maintains persistent connections with heartbeat
- Serves ROCKS requests over end-to-end TLS 1.3

#### Client Node (hidden)

End-user node. Runs on user devices.

- **Exposes NO ports** — connects outbound to relay nodes
- Resolves identities via blockchain
- Requests routing through relay mesh
- Establishes TLS 1.3 with services (through relay pipes)
- Optionally runs a local HTTP gateway for browser integration

---

### 4.2 Network Topology

```
                    ┌─────────────────────┐
                    │  ROCKS Blockchain   │
                    │  (CometBFT + SDK)   │
                    │                     │
                    │  - Identity registry│
                    │  - Relay stakes     │
                    │  - Service metadata │
                    │  - Slashing records │
                    └──────────┬──────────┘
                               │
          ┌────────────────────┼────────────────────┐
          │                    │                    │
   ┌──────▼──────┐     ┌──────▼──────┐     ┌──────▼──────┐
   │ Validator/  │◄───►│ Validator/  │◄───►│ Validator/  │
   │ Relay A     │     │ Relay B     │     │ Relay C     │
   │ (public)    │     │ (public)    │     │ (public)    │
   └──┬───────┬──┘     └──────┬──────┘     └──┬───────┬──┘
      │       │               │                │       │
      │       │          (mesh links)          │       │
      │       │               │                │       │
   ┌──▼──┐ ┌──▼──┐           │           ┌───▼──┐ ┌──▼───┐
   │Svc 1│ │Svc 2│           │           │Cli 1 │ │Cli 2 │
   │(out)│ │(out)│           │           │(out) │ │(out) │
   └─────┘ └─────┘           │           └──────┘ └──────┘
                              │
                         ┌────▼────┐
                         │ Svc 3   │
                         │ (out)   │
                         └─────────┘

   (out) = outbound connection only, no exposed ports
   ◄──► = relay mesh links (libp2p)
```

---

## 5. Connection Flow

### 5.1 Service Registration

When a service starts up:

```
1. Generate or load ed25519 identity
2. Register on blockchain:
   - ROCKS address
   - Service metadata (endpoints, schemas, pricing)
   - On-chain transaction signed with identity key
3. Connect outbound to N relay nodes
4. Announce identity to each relay:
   - "I am rocks1abc..., route traffic for me here"
   - Relay updates its ARP table
5. Maintain connections with periodic heartbeat
```

### 5.2 Client Request (Always 3+ Relay Hops)

```
Client          Relay A       Relay B       Relay C       Service
(entry)        (middle)      (exit)
  │               │             │             │             │
  │──query blockchain: "does rocks1abc... exist?"           │
  │←─yes, registered with metadata                         │
  │               │             │             │             │
  │──connect to Relay A (outbound)            │             │
  │──"route me to rocks1abc..."──→│           │             │
  │               │               │           │             │
  │         A selects B (middle)  │           │             │
  │               │──forward────→│            │             │
  │               │              │            │             │
  │         B selects C (exit)   │            │             │
  │               │              │──forward──→│             │
  │               │              │            │             │
  │         C looks up ARP: rocks1abc → connected here      │
  │               │              │            │──pipe──→    │
  │               │              │            │             │
  │   3-relay pipe established:                             │
  │   Client ↔ A ↔ B ↔ C ↔ Service                        │
  │               │              │            │             │
  │←══════ TLS 1.3 HANDSHAKE (through pipe) ═══════════════→│
  │  Client verifies: cert pubkey == rocks1abc...?          │
  │               │              │            │             │
  │←══════ ROCKS REQUEST (encrypted, through pipe) ════════→│
  │  identity: rocks1xyz...                                 │
  │  signature: <signed>                                    │
  │  GET /profile ROCKS/1.0.0                              │
  │               │              │            │             │
  │←══════ ROCKS RESPONSE (encrypted, through pipe) ═══════→│
  │  ROCKS/1.0.0 200 OK                                    │
  │  {json response}                                       │
  │               │              │            │             │
  │  ALL traffic through A → B → C for entire session       │
  │  NEVER upgrades to direct connection                    │
```

### 5.3 Relay Knowledge Isolation

No single relay has the complete picture:

| Relay | Knows | Cannot See |
|-------|-------|------------|
| **A** (entry) | Client's IP/connection, next hop = B | Service identity, content |
| **B** (middle) | Previous hop = A, next hop = C | Client IP, service identity, content |
| **C** (exit) | Previous hop = B, service identity | Client IP, content |

TLS 1.3 is end-to-end between client and service. All 3 relays see only ciphertext.

### 5.4 Why Self-Signed TLS Works

The ROCKS address IS the public key. No certificate authority is needed:

```
1. Service presents self-signed X.509 cert with its ed25519 pubkey
2. Client derives: bech32("rocks", sha256(cert_pubkey)[:20])
3. Client compares: derived address == target address?
4. Match → authenticated. Mismatch → reject.
```

Relays cannot MITM because they don't possess the service's private key. The blockchain serves as the certificate authority — if a ROCKS address is registered on-chain, the corresponding public key is the only valid cert.

---

## 6. ARP-Like Discovery Protocol

The relay mesh maintains a distributed identity resolution table, similar to ARP on a LAN.

### 6.1 ARP Table Structure

Each relay maintains:

```
┌──────────────────────────────────────────────────────┐
│                    ARP Table                         │
├──────────────────┬───────────┬────────┬──────────────┤
│ ROCKS Address    │ Relay ID  │ Hops   │ Last Seen    │
├──────────────────┼───────────┼────────┼──────────────┤
│ rocks1abc...     │ self      │ 0      │ 2s ago       │
│ rocks1def...     │ relay-B   │ 1      │ 5s ago       │
│ rocks1ghi...     │ relay-C   │ 2      │ 12s ago      │
└──────────────────┴───────────┴────────┴──────────────┘
```

- **Hops = 0**: Service is directly connected to this relay
- **Hops = 1+**: Service is connected to a neighboring relay (learned via gossip)

### 6.2 ARP Operations

**Announce**: Service connects to relay, declares its identity → relay adds entry with hops=0

**Gossip**: Relays periodically share their ARP tables with neighbors → entries propagate with incrementing hop count

**Expire**: Entries expire if not refreshed by heartbeat → stale services are removed

**Lookup**: When routing a request, relay checks its ARP table → finds which relay holds the target service → forwards accordingly

### 6.3 ARP + Blockchain

The blockchain provides the **authoritative** identity registry:
- "Does rocks1abc... exist?" → Query blockchain
- "Where is rocks1abc... right now?" → Query relay ARP tables

Blockchain = identity exists.
ARP table = identity is online and reachable.

---

## 7. Request Identity & Authentication

### 7.1 Mandatory Identity

Every ROCKS request MUST include:

```
GET /profile ROCKS/1.0.0
identity: rocks1xyz...
signature: <base64(ed25519_sign(request_hash))>
timestamp: 1704067200
nonce: a1b2c3d4
```

| Header | Purpose |
|--------|---------|
| `identity` | Sender's ROCKS address |
| `signature` | ed25519 signature of `method + path + timestamp + nonce + body_hash` |
| `timestamp` | Unix timestamp (prevents replay after TTL) |
| `nonce` | Random value (prevents replay within TTL window) |

Requests without valid identity + signature are rejected with `401 UNAUTHORIZED`.

### 7.2 Signature Verification

```
request_hash = sha256(METHOD + " " + PATH + " " + TIMESTAMP + " " + NONCE + " " + sha256(BODY))
valid = ed25519_verify(identity_pubkey, request_hash, signature)
```

The service:
1. Extracts the `identity` header → derives public key from ROCKS address
2. Recomputes `request_hash` from the request components
3. Verifies the ed25519 signature against the public key
4. Checks timestamp is within acceptable window (e.g., ±60 seconds)
5. Checks nonce hasn't been seen before (replay protection)

### 7.3 Response Signing (optional)

Services MAY sign responses to prove authenticity:

```
ROCKS/1.0.0 200 OK
identity: rocks1abc...
signature: <base64(ed25519_sign(response_hash))>
content-type: application/json
content-length: 42

{"name": "Alice"}
```

---

## 8. Economics: Read Free, Write Paid

### 8.1 Default Pricing Model

| Method | Default | Configurable |
|--------|---------|-------------|
| `GET` | Free | Can be made paid |
| `HEAD` | Free | Can be made paid |
| `OPTIONS` | Free | Always free |
| `POST` | Paid | Can be made free |
| `PUT` | Paid | Can be made free |
| `PATCH` | Paid | Can be made free |
| `DELETE` | Paid | Can be made free |

**Rationale**: Reading data doesn't change state. Writing data requires storage, computation, and creates state changes — this should cost tokens to prevent spam.

### 8.2 Payment Flow

```
1. Client queries blockchain for service's pricing:
   - rocks1abc.../api/submit → costs 0.001 ROCKS per request
   - rocks1abc.../api/data   → free (GET)

2. For paid requests, client includes a payment proof:
   POST /api/submit ROCKS/1.0.0
   identity: rocks1xyz...
   signature: <signed>
   payment: <on-chain payment tx hash or payment channel proof>
   content-type: application/json

   {"data": "hello"}

3. Service verifies payment before processing
4. Service responds with receipt
```

### 8.3 Payment Mechanisms

Two payment models:

**On-chain**: Client sends a transaction to the service's address with a memo referencing the request. Service verifies transaction on-chain before responding. Higher latency but simple.

**Payment channels** (future): Client and service open a payment channel. Micropayments flow off-chain. Channel settles on-chain periodically. Low latency, high throughput.

### 8.4 Service Pricing Configuration

Services declare pricing in their on-chain registration:

```json
{
  "address": "rocks1abc...",
  "endpoints": [
    {
      "path": "/api/data",
      "methods": ["GET"],
      "pricing": "free",
      "rate_limit": "100/min"
    },
    {
      "path": "/api/submit",
      "methods": ["POST"],
      "pricing": {
        "amount": "1000",
        "denom": "urocks"
      },
      "rate_limit": "10/min"
    }
  ]
}
```

---

## 9. Service Registry (On-Chain)

### 9.1 Registration

Services register on the blockchain with comprehensive metadata:

```json
{
  "address": "rocks1abc...",
  "name": "My API Service",
  "description": "A decentralized API for...",
  "version": "1.0.0",
  "endpoints": [
    {
      "path": "/users",
      "methods": ["GET"],
      "description": "List users",
      "request_schema": null,
      "response_schema": {
        "type": "array",
        "items": {"type": "object", "properties": {"id": "string", "name": "string"}}
      },
      "pricing": "free",
      "rate_limit": "100/min"
    },
    {
      "path": "/users",
      "methods": ["POST"],
      "description": "Create a user",
      "request_schema": {
        "type": "object",
        "required": ["name"],
        "properties": {"name": "string", "email": "string"}
      },
      "response_schema": {
        "type": "object",
        "properties": {"id": "string", "name": "string"}
      },
      "pricing": {"amount": "1000", "denom": "urocks"},
      "rate_limit": "10/min"
    }
  ],
  "banned_addresses": [],
  "metadata": {
    "website": "rocks://rocks1abc.../",
    "docs": "rocks://rocks1abc.../docs"
  }
}
```

### 9.2 Registry as Discovery + Testing Ground

The on-chain registry acts as:

- **Service directory** — anyone can browse all registered services
- **API documentation** — endpoint schemas are on-chain and queryable
- **Testing ground** — clients can inspect request/response schemas before calling
- **Pricing transparency** — all costs are visible on-chain

### 9.3 Registry Transactions

| Transaction | Purpose |
|-------------|---------|
| `MsgRegisterService` | Register a new service with metadata |
| `MsgUpdateService` | Update service metadata, endpoints, pricing |
| `MsgDeregisterService` | Remove service from registry |
| `MsgBanAddress` | Ban a specific ROCKS address from the service |
| `MsgUnbanAddress` | Remove a ban |
| `MsgUpdateRateLimit` | Update rate limits for endpoints |

### 9.4 Registry Queries

| Query | Purpose |
|-------|---------|
| `QueryService(address)` | Full service info by ROCKS address |
| `QueryAllServices()` | List all registered services (paginated) |
| `QueryServiceEndpoints(address)` | List endpoints for a service |
| `QueryServicePricing(address, path)` | Get pricing for specific endpoint |
| `QueryBannedAddresses(address)` | List banned addresses for a service |
| `QueryServicesByTag(tag)` | Search services by tag/category |

---

## 10. Security & Enforcement

### 10.1 Slashing Conditions

Validator-relay nodes are staked and subject to slashing for misbehavior:

| Offense | Severity | Slash % | Detection |
|---------|----------|---------|-----------|
| **Downtime** (missed blocks) | Low | 0.1% | CometBFT consensus |
| **Double signing** (equivocation) | Critical | 5% | CometBFT evidence |
| **Packet dropping** (refusing to relay) | Medium | 1% | Client reports + proofs |
| **Censorship** (selectively dropping) | High | 3% | Statistical analysis |
| **Data tampering** (modifying relay bytes) | Critical | 5% | TLS verification failure reports |
| **ARP poisoning** (false routing) | Critical | 5% | Service reports + proofs |

### 10.2 Service-Level Bans

Services can ban specific ROCKS addresses:

```
MsgBanAddress {
  service: "rocks1abc..."
  banned:  "rocks1evil..."
  reason:  "spam"
  duration: 86400  // seconds, 0 = permanent
}
```

When a banned address tries to connect:
1. Service receives the request through the relay pipe
2. Checks `identity` header against ban list
3. Responds with `403 FORBIDDEN` and closes pipe

Bans are stored on-chain and can be queried by anyone for transparency.

### 10.3 Rate Limiting

Rate limits operate at multiple levels:

| Level | Enforced By | Scope |
|-------|-------------|-------|
| **Protocol-level** | Relay nodes | Per-identity, per-relay (prevents relay abuse) |
| **Service-level** | Service node | Per-identity, per-endpoint (declared on-chain) |
| **Network-level** | Consensus | Per-identity, global (prevents network spam) |

Rate limit headers in response:

```
ROCKS/1.0.0 200 OK
x-ratelimit-limit: 100
x-ratelimit-remaining: 42
x-ratelimit-reset: 1704067260
```

When rate limited:

```
ROCKS/1.0.0 429 TOO_MANY_REQUESTS
retry-after: 30
x-ratelimit-limit: 100
x-ratelimit-remaining: 0
x-ratelimit-reset: 1704067260
```

### 10.4 Request Validation Pipeline

Every request passes through this validation chain:

```
1. Identity present?          → No  → 401 UNAUTHORIZED
2. Signature valid?           → No  → 401 UNAUTHORIZED
3. Timestamp within window?   → No  → 408 TIMEOUT
4. Nonce seen before?         → Yes → 409 CONFLICT (replay)
5. Identity banned?           → Yes → 403 FORBIDDEN
6. Rate limit exceeded?       → Yes → 429 TOO_MANY_REQUESTS
7. Payment required?          → Yes → Verify payment
8. Payment valid?             → No  → 402 PAYMENT_REQUIRED
9. Process request            → Execute handler
```

---

## 11. Protocol Message Format

### 11.1 Request

```
METHOD /path ROCKS/1.0.0\r\n
identity: rocks1xyz...\r\n
signature: <base64>\r\n
timestamp: 1704067200\r\n
nonce: a1b2c3d4\r\n
content-type: application/json\r\n
content-length: 42\r\n
\r\n
[body]
```

### 11.2 Response

```
ROCKS/1.0.0 200 OK\r\n
identity: rocks1abc...\r\n
content-type: application/json\r\n
content-length: 42\r\n
server: ROCKS/1.0.0\r\n
date: Wed, 01 Jan 2025 00:00:00 GMT\r\n
\r\n
[body]
```

### 11.3 HANDSHAKE (session initialization)

```
→ HANDSHAKE / ROCKS/1.0.0\r\n
  identity: rocks1xyz...\r\n
  signature: <base64>\r\n
  timestamp: 1704067200\r\n
  user-agent: ROCKS-Client/1.0.0\r\n
  \r\n

← ROCKS/1.0.0 101 SWITCHING_PROTOCOLS\r\n
  identity: rocks1abc...\r\n
  rocks-version: 1.0.0\r\n
  encryption: TLS/1.3\r\n
  server: ROCKS/1.0.0\r\n
  date: Wed, 01 Jan 2025 00:00:00 GMT\r\n
  \r\n
```

### 11.4 Methods

| Method | Purpose | Body | Default Cost |
|--------|---------|------|-------------|
| `GET` | Retrieve resource | No | Free |
| `POST` | Submit data | Yes | Paid |
| `PUT` | Replace resource | Yes | Paid |
| `DELETE` | Remove resource | No | Paid |
| `HEAD` | Headers only | No | Free |
| `OPTIONS` | Capabilities | No | Free |
| `PATCH` | Partial update | Yes | Paid |
| `HANDSHAKE` | Session init | No | Free |

### 11.5 Status Codes

ROCKS uses HTTP-compatible status codes with additions:

**Success**: 200 OK, 201 CREATED, 202 ACCEPTED, 204 NO_CONTENT
**Protocol**: 101 SWITCHING_PROTOCOLS
**Client Errors**: 400 BAD_REQUEST, 401 UNAUTHORIZED, 402 PAYMENT_REQUIRED, 403 FORBIDDEN, 404 NOT_FOUND, 405 METHOD_NOT_ALLOWED, 408 TIMEOUT, 409 CONFLICT, 413 TOO_LARGE, 415 UNSUPPORTED_MEDIA_TYPE, 429 TOO_MANY_REQUESTS
**Server Errors**: 500 INTERNAL_SERVER_ERROR, 501 NOT_IMPLEMENTED, 502 BAD_GATEWAY, 503 SERVICE_UNAVAILABLE, 504 GATEWAY_TIMEOUT

---

## 12. Blockchain Layer

### 12.1 Consensus

ROCKS uses **CometBFT** (formerly Tendermint) for Byzantine Fault Tolerant consensus.

- Block time: ~5 seconds
- Finality: Instant (no probabilistic finality)
- Validator set: Staked validator-relay nodes
- Key type: ed25519 (same key as ROCKS identity)

### 12.2 Application Framework

Built on **Cosmos SDK** with custom modules:

| Module | Purpose |
|--------|---------|
| `x/registry` | Service identity registry (addresses, metadata, endpoints, schemas) |
| `x/relay` | Relay staking, slashing, reward distribution |
| `x/evm` | EVM compatibility (deploy Solidity contracts) |
| `x/auth` | Account management (Cosmos SDK standard) |
| `x/bank` | Token transfers (Cosmos SDK standard) |
| `x/staking` | Validator staking (Cosmos SDK standard) |
| `x/gov` | Governance proposals (Cosmos SDK standard) |

### 12.3 EVM Compatibility

ROCKS includes an EVM module for deploying Solidity smart contracts:

- Full EVM execution via `go-ethereum`
- Ethereum JSON-RPC compatibility (`eth_`, `web3_`, `net_`)
- Deploy and call Solidity contracts
- Interact with relay staking and service registry from EVM

Smart contracts:
- `RelayStaking.sol` — stake/unstake/slash via EVM interface
- `ServiceRegistry.sol` — EVM-accessible service directory
- `ROCKSToken.sol` — native token ERC-20 wrapper

### 12.4 Native Token

The ROCKS network has a native token (`ROCKS`, smallest unit `urocks`) used for:

- Relay staking (economic security)
- Paid write requests (POST/PUT/PATCH/DELETE)
- Service registration fees
- Governance voting
- Relay reward distribution

---

## 13. Relay Mesh Implementation

### 13.1 Transport: libp2p

The relay mesh uses **libp2p** for peer-to-peer networking:

- **Identity**: ed25519 (native, same key as ROCKS identity)
- **Transport**: TCP with Noise Protocol encryption
- **Multiplexing**: yamux (multiple streams over one connection)
- **Discovery**: Kademlia DHT + bootstrap nodes + blockchain registry
- **NAT Traversal**: AutoRelay + hole punching

### 13.2 libp2p Protocol IDs

```
/rocks/announce/1.0.0     — Service announces identity to relay
/rocks/route/1.0.0        — Client requests routing to identity
/rocks/forward/1.0.0      — Relay-to-relay connection forwarding
/rocks/pipe/1.0.0         — Bidirectional data pipe (carries TLS)
/rocks/heartbeat/1.0.0    — Keep-alive for service connections
/rocks/arp/1.0.0          — ARP table gossip between relays
```

### 13.3 Connection Architecture

```
Layer 4: ROCKS protocol       (GET /profile — application data)
Layer 3: TLS 1.3              (end-to-end encryption, client ↔ service)
Layer 2: libp2p pipe           (byte stream, chained through 3+ relays)
Layer 1: libp2p Noise + TCP   (hop-by-hop transport between relays)
```

Relays operate at Layer 1-2. They forward Layer 3-4 bytes blindly.

### 13.4 Three-Relay Minimum (Hard Protocol Rule)

Every connection MUST traverse at least 3 relay hops:

```
Client ──→ Relay A (entry) ──→ Relay B (middle) ──→ Relay C (exit) ──→ Service
```

This is enforced at the protocol level:
- Entry relay (A) selects a middle relay (B)
- Middle relay (B) selects an exit relay (C)
- Exit relay (C) resolves the destination via ARP table
- If the service is on a different relay (D), the chain extends (4+ hops)
- The pipe is maintained for the entire session — never collapses to direct

### 13.5 Two Separate P2P Networks

```
Network 1: CometBFT p2p (port 26656)
  → Validators gossip blocks, votes, mempool transactions
  → CometBFT's built-in p2p layer
  → Same ed25519 key as ROCKS identity

Network 2: ROCKS libp2p mesh (port 4878)
  → Relays forward encrypted traffic
  → Clients and services connect here
  → Same ed25519 key as ROCKS identity
```

Same identity key on both networks. Different ports, different protocols, independent operation.

---

## 14. Browser Integration

### 14.1 Local Gateway Daemon

A local gateway daemon allows standard browsers to access `rocks://` URLs:

```
Browser ──HTTP──→ Local Gateway (localhost:8080)
                      │
                      ├── Parse rocks:// URL
                      ├── Resolve identity via blockchain
                      ├── Connect to relay mesh
                      ├── Build 3-relay pipe
                      ├── TLS 1.3 handshake
                      ├── Send ROCKS request
                      ├── Receive response
                      └── Return as HTTP response to browser
```

### 14.2 Firefox Extension (Future)

- Native `rocks://` URL scheme handler
- Identity wallet management
- Circuit visualization
- Service directory browser

---

## 15. Development Phases

### Phase 1: Identity + Protocol Core
- ed25519 keypair generation and bech32 address encoding
- Self-signed TLS certificate from identity
- `rocks://` URL parser
- ROCKS message format (request/response parser)
- HANDSHAKE flow
- Request signing and verification

### Phase 2: Relay Mesh Network
- libp2p host with ed25519 identity
- Custom protocol IDs and stream handlers
- ARP table (identity → relay mapping)
- Relay node (accept connections, route traffic)
- Service node (outbound connection, announce identity)
- Client node (outbound connection, request routing)
- Bidirectional pipe (3+ relay chain)
- Multi-hop router (enforces 3-relay minimum)
- Peer discovery (bootstrap + DHT + blockchain)

### Phase 3: Blockchain Layer
- Cosmos SDK app scaffold with CometBFT
- Service Registry module (`x/registry`)
- Relay Staking module (`x/relay`)
- EVM module (`x/evm`)
- Solidity smart contracts
- Blockchain integration in mesh (query chain for relays, services)

### Phase 4: Integration
- Node orchestration (single binary, multiple modes)
- Full end-to-end pipeline
- Relay incentive system
- Slashing enforcement
- Rate limiting
- Ban system

### Phase 5: Browser Gateway
- Local HTTP-to-ROCKS proxy daemon
- Browser configuration (PAC file)
- Firefox extension (future)

### Phase 6: Testing & Hardening
- Unit tests, integration tests, fuzzing
- DevNet tooling (Docker Compose, genesis generator)
- CLI tools (key generation, service registration, client)
- Protocol specification document

---

## 16. Key Dependencies

```
crypto/ed25519                        — identity keys (stdlib)
crypto/tls                            — TLS 1.3 (stdlib)
crypto/x509                           — self-signed certs (stdlib)
github.com/cometbft/cometbft          — consensus (ed25519 native)
github.com/cosmos/cosmos-sdk          — app framework (ed25519 native)
github.com/ethereum/go-ethereum       — EVM execution engine
github.com/libp2p/go-libp2p          — relay mesh (ed25519 native)
github.com/libp2p/go-libp2p-kad-dht  — DHT peer discovery
google.golang.org/protobuf            — serialization
google.golang.org/grpc                — RPC
```
