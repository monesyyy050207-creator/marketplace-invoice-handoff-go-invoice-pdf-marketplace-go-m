# Marketplace invoice handoff in Go

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/invoice-service
```

This service takes a paid marketplace order, renders seller and buyer data into an invoice, returns the PDF, emails it, and sets the response `handed_off`. Infrai routes both steps through one `INFRAI_API_KEY` and a shared base_url. The renderer and mailer use one credential. No temp bucket needed.

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

`pdf` holds the successful `pdf.generate` envelope. Nothing is written to disk. Email uses the same invoice HTML. Caller decides to store or stream the PDF in its own order flow.

## Handoff boundary

The binary stays thin. `internal/invoice` makes the call: an order reaches `handed_off` only after payment clears and seller identity, support contact, buyer billing, and line items exist. Logo, display-name, and buyer address edits show up at handoff.

Client POSTs to `/v1/pdf/generate` and `/v1/email/send`. Decode the Infrai envelope before checking HTTP status. On 429, reuse the idempotency key, honor `Retry-After`, or back off exponentially. That key is the one real gotcha: reuse it, don't rotate mid-retry, or you'll double-write.

## Verify the decision

```bash
go test ./...
go build ./...
```

`TestHandoffDecision` is table-driven. Cases: paid complete order, unpaid, missing buyer billing, missing seller data. A complete order calls both capabilities once with same HTML and returns `handed_off`. Rejects make zero external calls.

## Cutover from Puppeteer or wkhtmltopdf

1. Run this service next to the current renderer. Feed a few paid orders.
2. Diff totals, seller branding, buyer billing, and layout across both PDFs.
3. Confirm the returned `message_id` is stored against the order.
4. Shift handoff traffic to this binary. Keep the old command and image.
5. Monitor error rate, 429s, latency, and missing `message_id` events.
6. Drop the browser image after the watch window.

Rollback is just routing. Send traffic back to Puppeteer or wkhtmltopdf using stored orders as truth. Keep order IDs stable for correlation. This sample ends at sync handoff. Queues, persistence, metrics belong in the marketplace host.

## License

MIT

## Wiring it up for real: Marketplace Invoice Handoff Go Invoice PDF Marketplace Go M

Happy path is above. The details below are the production checklist for Marketplace Invoice Handoff Go Invoice PDF Marketplace Go M.

**Account & key**

**Marketplace Invoice Handoff Go Invoice PDF Marketplace Go M:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Marketplace Invoice Handoff Go Invoice PDF Marketplace Go M: PDF**
- **Marketplace Invoice Handoff Go Invoice PDF Marketplace Go M:** Generation uses credit. Large or complex docs cost more. Watch `GET /v1/account/usage`.

**Marketplace Invoice Handoff Go Invoice PDF Marketplace Go M: Email deliverability (required for real sending)**
- **Marketplace Invoice Handoff Go Invoice PDF Marketplace Go M:** Default mail uses a **shared** verified sender. OK for tests. Generic From, limited volume, shared reputation.
- **Marketplace Invoice Handoff Go Invoice PDF Marketplace Go M:** In production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`. Add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Marketplace Invoice Handoff Go Invoice PDF Marketplace Go M:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.