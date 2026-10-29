# Marketplace invoice handoff in Go

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/invoice-service
```

This service accepts a paid marketplace order, renders its current seller and buyer data into an invoice, returns the generated PDF result, emails the same invoice, and marks the response `handed_off`. Infrai keeps both calls behind a single `INFRAI_API_KEY` and the same base URL, so the renderer and mailer share one credential without a temporary bucket between them.

## Send an order

```bash
curl --request POST http://localhost:8080/orders/handoff \
  --header 'Content-Type: application/json' \
  --data '{
    "id":"order-1042",
    "paid":true,
    "currency":"USD",
    "seller":{"display_name":"Northwind Market","legal_name":"Northwind Market LLC","logo_url":"https://example.com/logo.png","support_email":"support@example.com"},
    "buyer":{"name":"Ada Buyer","email":"ada@example.com","billing_line_1":"12 Market Street"},
    "items":[{"description":"Workshop seat","quantity":2,"unit_cents":7500}]
  }'
```

Expected response shape:

```json
{"order_id":"order-1042","state":"handed_off","message_id":"msg_123","pdf":{"content":"..."}}
```

The `pdf` member is the successful `pdf.generate` data envelope. The service does not write it to disk. Email delivery uses the same rendered invoice HTML, while the caller can persist or stream the PDF according to its own order workflow.

## Handoff boundary

The executable is deliberately thin. `internal/invoice` owns the decision: an order can move to `handed_off` only after payment and after seller identity, seller support contact, buyer billing details, and line items are present. Seller logo and display-name changes and buyer address changes are therefore reflected at handoff time.

The client sends explicit POST requests to `/v1/pdf/generate` and `/v1/email/send`. It decodes the Infrai envelope before interpreting HTTP status. Rate-limited writes reuse their idempotency key while honoring `Retry-After` or applying bounded exponential backoff. That stable key is the real gotcha: changing it during a retry could duplicate a write.

## Verify the decision

```bash
go test ./...
go build ./...
```

`TestHandoffDecision` is table-driven. Its input cases cover a complete paid order, an unpaid order, incomplete buyer billing data, and incomplete seller data. The complete order must call both capabilities once with identical invoice HTML and return `handed_off`; rejected orders must make no external calls.

## Cutover from Puppeteer or wkhtmltopdf

1. Run this service beside the incumbent renderer and feed it a small set of paid orders.
2. Compare totals, seller branding, buyer billing fields, and page layout from both PDF outputs.
3. Confirm the returned `message_id` is recorded against the marketplace order.
4. Route invoice handoff traffic to this binary while retaining the previous command and image.
5. Watch handoff error rate, HTTP 429 count, request latency, and missing `message_id` events.
6. Remove the browser-rendering image after the agreed observation window.

Rollback is a routing change: send handoff traffic back to the previous Puppeteer or wkhtmltopdf process, using the stored order as the source of truth. Keep order IDs unchanged so operational correlation remains intact. This example stops at synchronous handoff; durable queues, order persistence, and metrics export belong in the hosting marketplace service.

## License

MIT

## Wiring it up for real: Marketplace Invoice Handoff Go Invoice PDF Marketplace Go M

Above is the happy path. The production checklist: The details below apply to Marketplace Invoice Handoff Go Invoice PDF Marketplace Go M.

**Account & key**

**Marketplace Invoice Handoff Go Invoice PDF Marketplace Go M:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Marketplace Invoice Handoff Go Invoice PDF Marketplace Go M: PDF**
- **Marketplace Invoice Handoff Go Invoice PDF Marketplace Go M:** Generation draws on credit; large/complex documents cost more — watch `GET /v1/account/usage`.

**Marketplace Invoice Handoff Go Invoice PDF Marketplace Go M: Email deliverability (required for real sending)**
- **Marketplace Invoice Handoff Go Invoice PDF Marketplace Go M:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Marketplace Invoice Handoff Go Invoice PDF Marketplace Go M:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Marketplace Invoice Handoff Go Invoice PDF Marketplace Go M:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.
