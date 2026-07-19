# 🧵 Loom

**A multi-provider payment orchestration layer for Go — one interface, several African payment rails, automatic failover.**

Loom sits between your application and payment providers like Paystack and Flutterwave, so your code calls one consistent API instead of hand-rolling retries, idempotency, and webhook parsing per provider. If one provider is down or erroring, Loom fails over to the next — automatically, with circuit breakers so a struggling provider gets skipped rather than repeatedly hammered.

```go
result, err := client.Charges.Initialize(ctx, &types.ChargeRequest{
    Amount:   types.Money{Value: 500000, Currency: types.NGN}, // ₦5,000.00
    Email:    "customer@example.com",
    Reference: "order_1234",
})
// Loom picks a provider, tries it, and fails over automatically if needed —
// your code doesn't know or care whether Paystack or Flutterwave handled it.
```

---

## Why Loom exists

Integrating one African payment provider is manageable. Integrating three — each with its own SDK, its own webhook signature scheme, its own error shapes, its own uptime quirks — turns into duplicated glue code scattered across your app. Loom centralizes that:

- **One request type in, one response type out** — regardless of which provider actually processed it
- **Automatic failover** — a failed charge attempt on Provider A can retry on Provider B without your application code knowing
- **Circuit breakers per provider** — a provider that's actively failing gets temporarily skipped instead of retried into the ground
- **Idempotency built in** — duplicate requests (client retries, double form-submits) are locked and deduplicated before they ever reach a provider
- **Normalized webhooks** — verify signatures and parse events from any supported provider into one `WebhookEvent` shape

---

## Supported providers

| Provider | Charges | Refunds | Transfers | Virtual Accounts | Webhooks |
|---|:---:|:---:|:---:|:---:|:---:|
| Paystack | ✅ | ✅ | ✅ | ✅ | ✅ |
| Flutterwave | ✅ | ✅ | ✅ | ✅ | ✅ |

The provider interface is built to extend — `types.ProviderName` already reserves names for Monnify and Stripe, ready for whenever those get implemented.

---

## Architecture, in one picture

```
Your application
      │
      ▼
┌─────────────────────────────────────────────┐
│  pkg/loom.Client   (public entry point)      │
└─────────────────────────────────────────────┘
      │
      ▼
┌─────────────────────────────────────────────┐
│  pkg/orchestrator                            │
│    • selects candidate providers             │
│    • checks each one's circuit breaker        │
│    • tries them in order, fails over on error │
└─────────────────────────────────────────────┘
      │
      ▼
┌─────────────────────────────────────────────┐
│  pkg/provider.Registry                       │
│    Paystack  │  Flutterwave  │  (future...)  │
└─────────────────────────────────────────────┘
```

Requests that mutate state (charges, refunds, transfers, virtual accounts) pass through `pkg/idempotency` first — a request carrying a repeated idempotency key is either replayed from its cached result or rejected as in-flight, never processed twice.

---

## Quick start

### As a library

```bash
go get github.com/loom-payments/loom
```

```go
client, err := loom.NewClient(
    loom.WithPaystack(loom.PaystackConfig{
        SecretKey: os.Getenv("PAYSTACK_SECRET_KEY"),
    }),
    loom.WithFlutterwave(loom.FlutterwaveConfig{
        SecretKey: os.Getenv("FLUTTERWAVE_SECRET_KEY"),
    }),
    loom.WithProviderPriority("paystack", "flutterwave"),
    loom.WithFailover(true),
)
if err != nil {
    log.Fatal(err)
}

resp, err := client.Charges.Initialize(ctx, &types.ChargeRequest{
    Amount:    types.Money{Value: 500000, Currency: types.NGN},
    Email:     "customer@example.com",
    Reference: "order_1234",
})
```

### As a standalone service

```bash
cp .env.example .env    # fill in at least one provider's keys
go run main.go
```

```bash
curl -X POST http://localhost:8080/api/v1/charges \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: order_1234" \
  -d '{"amount": {"value": 500000, "currency": "NGN"}, "email": "customer@example.com", "reference": "order_1234"}'
```

---

## Configuration

Loom is configured entirely through environment variables (see `.env.example` for the full list). The essentials:

| Variable | Purpose |
|---|---|
| `PAYSTACK_SECRET_KEY` / `FLUTTERWAVE_SECRET_KEY` | At least one is required — Loom won't boot with zero working providers |
| `PROVIDER_PRIORITY` | Comma-separated order to try providers in, e.g. `paystack,flutterwave` |
| `FAILOVER_ENABLED` | Whether to try the next provider on failure, or fail fast on the first |
| `DATABASE_URL` | Postgres connection string — powers idempotency and webhook storage. Omit it and Loom falls back to an in-memory store (fine for local dev, not for production) |
| `CIRCUIT_BREAKER_MAX_FAILURES` / `CIRCUIT_BREAKER_TIMEOUT` | How many failures trip a provider's circuit, and how long before it's tested again |
| `IDEMPOTENCY_TTL` / `IDEMPOTENCY_LOCK_DURATION` | How long a completed request is cached, and how long an in-flight lock is held |

Prefer code over env vars? Every setting has a functional-option equivalent in `pkg/loom/options.go` — `WithPaystack`, `WithFlutterwave`, `WithProviderPriority`, `WithFailover`, `WithCircuitBreaker`, `WithIdempotencyStore`, and more.

---

## API endpoints

When run as a service (`main.go`), Loom exposes:

```
POST   /api/v1/charges              Initialize a charge
GET    /api/v1/charges/:reference   Verify a charge
POST   /api/v1/refunds              Create a refund
GET    /api/v1/refunds/:id          Check refund status
POST   /api/v1/transfers            Initiate a transfer
GET    /api/v1/transfers/:reference Verify a transfer
GET    /api/v1/banks                List supported banks
POST   /api/v1/virtual-accounts     Create a virtual account
POST   /api/v1/webhooks/:provider   Receive provider webhooks
GET    /health                      Service + per-provider health
```

All mutating endpoints (`POST /charges`, `/refunds`, `/transfers`, `/virtual-accounts`) honor an `Idempotency-Key` header.

---

## Migrating the database

```bash
psql $DATABASE_URL -f migrations/001_initial_schema.sql
```

This creates tables for transactions, idempotency records, webhook events, virtual accounts, reconciliation, and provider metrics.

---

## Project status

Loom's core is solid: the provider abstraction, circuit breakers, sequential failover, and idempotency locking all work and are consistent with each other. A few things are mid-flight, worth knowing if you're extending it:

- Only priority-based provider selection is currently wired in — round-robin, random, weighted, and cost-based selectors exist in `pkg/orchestrator/selector.go` but aren't yet reachable through config
- The `loom_webhook_events` table (dedupe + replay-ready schema) isn't yet written to by the webhook handler
- Reconciliation and provider-metrics tables exist in the migration ahead of the code that would populate them

Contributions closing any of these gaps are very welcome.

---

## License

*(Add your preferred license here — MIT is a common default for a project like this.)*
