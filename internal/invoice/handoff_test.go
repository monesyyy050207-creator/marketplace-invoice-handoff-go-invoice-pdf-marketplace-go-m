package invoice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type recordingClient struct {
	pdfCalls   int
	emailCalls int
	html       string
}

func (c *recordingClient) GeneratePDF(_ context.Context, html, _ string) (json.RawMessage, error) {
	c.pdfCalls++
	c.html = html
	return json.RawMessage(`{"content":"pdf-data"}`), nil
}

func (c *recordingClient) SendInvoiceEmail(_ context.Context, _, _, html, _ string) (string, error) {
	c.emailCalls++
	if html != c.html {
		return "", &contentMismatch{}
	}
	return "msg_123", nil
}

type contentMismatch struct{}

func (*contentMismatch) Error() string { return "renderer and mailer received different invoices" }

func validOrder() Order {
	return Order{
		ID: "order-1042", Paid: true, Currency: "USD",
		Seller: Seller{DisplayName: "Northwind Market", LegalName: "Northwind Market LLC", SupportEmail: "support@example.com"},
		Buyer:  Buyer{Name: "Ada Buyer", Email: "ada@example.com", BillingLine1: "12 Market Street"},
		Items:  []LineItem{{Description: "Workshop seat", Quantity: 2, UnitCents: 7500}},
	}
}

func TestHandoffDecision(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Order)
		wantState string
		wantErr   string
		wantCalls int
	}{
		{name: "paid order with current parties", mutate: func(*Order) {}, wantState: "handed_off", wantCalls: 1},
		{name: "unpaid order", mutate: func(o *Order) { o.Paid = false }, wantErr: "order must be paid"},
		{name: "buyer update incomplete", mutate: func(o *Order) { o.Buyer.BillingLine1 = "" }, wantErr: "buyer billing details are required"},
		{name: "seller asset record incomplete", mutate: func(o *Order) { o.Seller.SupportEmail = "" }, wantErr: "seller legal name and support email are required"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			order := validOrder()
			tc.mutate(&order)
			client := &recordingClient{}
			result, err := NewHandoffService(client).Handoff(context.Background(), order)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want %q", err, tc.wantErr)
				}
			} else if err != nil {
				t.Fatalf("handoff: %v", err)
			}
			if result.State != tc.wantState || client.pdfCalls != tc.wantCalls || client.emailCalls != tc.wantCalls {
				t.Fatalf("state/calls = %q/%d/%d, want %q/%d/%d", result.State, client.pdfCalls, client.emailCalls, tc.wantState, tc.wantCalls, tc.wantCalls)
			}
		})
	}
}

func TestInfraiClientSendsIdempotencyKeyInPayload(t *testing.T) {
	wantKeys := map[string]string{
		"/v1/pdf/generate": "order-1042:pdf",
		"/v1/email/send":   "order-1042:email",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got := payload["idempotency_key"]; got != wantKeys[r.URL.Path] {
			t.Errorf("%s idempotency_key = %v, want %q", r.URL.Path, got, wantKeys[r.URL.Path])
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/email/send" {
			_, _ = w.Write([]byte(`{"ok":true,"data":{"message_id":"msg_123"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"content":"pdf-data"}}`))
	}))
	defer server.Close()

	client := NewInfraiClient("test-key")
	client.baseURL = server.URL
	if _, err := client.GeneratePDF(context.Background(), "<p>invoice</p>", "order-1042"); err != nil {
		t.Fatalf("generate PDF: %v", err)
	}
	if _, err := client.SendInvoiceEmail(context.Background(), "chenhua@changba.com", "Invoice", "<p>invoice</p>", "order-1042"); err != nil {
		t.Fatalf("send email: %v", err)
	}
}
