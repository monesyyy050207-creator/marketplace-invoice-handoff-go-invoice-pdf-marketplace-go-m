package invoice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

type DeliveryClient interface {
	GeneratePDF(ctx context.Context, html, requestID string) (json.RawMessage, error)
	SendInvoiceEmail(ctx context.Context, to, subject, html, requestID string) (string, error)
}

type HandoffResult struct {
	OrderID   string          `json:"order_id"`
	State     string          `json:"state"`
	MessageID string          `json:"message_id"`
	PDF       json.RawMessage `json:"pdf"`
}

type HandoffService struct{ client DeliveryClient }

func NewHandoffService(client DeliveryClient) *HandoffService {
	return &HandoffService{client: client}
}

func (s *HandoffService) Handoff(ctx context.Context, order Order) (HandoffResult, error) {
	if err := order.ReadyForHandoff(); err != nil {
		return HandoffResult{}, err
	}
	var rendered bytes.Buffer
	if err := RenderHTML(&rendered, order); err != nil {
		return HandoffResult{}, fmt.Errorf("render invoice: %w", err)
	}
	html := rendered.String()
	pdf, err := s.client.GeneratePDF(ctx, html, order.ID)
	if err != nil {
		return HandoffResult{}, fmt.Errorf("generate invoice PDF: %w", err)
	}
	messageID, err := s.client.SendInvoiceEmail(ctx, order.Buyer.Email, "Invoice for order "+order.ID, html, order.ID)
	if err != nil {
		return HandoffResult{}, fmt.Errorf("email invoice: %w", err)
	}
	return HandoffResult{OrderID: order.ID, State: "handed_off", MessageID: messageID, PDF: pdf}, nil
}
