package invoice

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"strings"
)

type Seller struct {
	DisplayName  string `json:"display_name"`
	LegalName    string `json:"legal_name"`
	LogoURL      string `json:"logo_url"`
	SupportEmail string `json:"support_email"`
}

type Buyer struct {
	Name         string `json:"name"`
	Email        string `json:"email"`
	BillingLine1 string `json:"billing_line_1"`
}

type LineItem struct {
	Description string `json:"description"`
	Quantity    int    `json:"quantity"`
	UnitCents   int64  `json:"unit_cents"`
}

type Order struct {
	ID       string     `json:"id"`
	Paid     bool       `json:"paid"`
	Currency string     `json:"currency"`
	Seller   Seller     `json:"seller"`
	Buyer    Buyer      `json:"buyer"`
	Items    []LineItem `json:"items"`
}

func (o Order) ReadyForHandoff() error {
	if strings.TrimSpace(o.ID) == "" {
		return errors.New("order id is required")
	}
	if !o.Paid {
		return errors.New("order must be paid")
	}
	if strings.TrimSpace(o.Seller.LegalName) == "" || strings.TrimSpace(o.Seller.SupportEmail) == "" {
		return errors.New("seller legal name and support email are required")
	}
	if strings.TrimSpace(o.Buyer.Name) == "" || strings.TrimSpace(o.Buyer.Email) == "" || strings.TrimSpace(o.Buyer.BillingLine1) == "" {
		return errors.New("buyer billing details are required")
	}
	if len(o.Items) == 0 {
		return errors.New("at least one line item is required")
	}
	for _, item := range o.Items {
		if strings.TrimSpace(item.Description) == "" || item.Quantity <= 0 || item.UnitCents < 0 {
			return errors.New("line items need a description, positive quantity, and non-negative unit price")
		}
	}
	return nil
}

func (o Order) TotalCents() int64 {
	var total int64
	for _, item := range o.Items {
		total += int64(item.Quantity) * item.UnitCents
	}
	return total
}

var invoiceTemplate = template.Must(template.New("invoice").Funcs(template.FuncMap{
	"money":     func(cents int64) string { return fmt.Sprintf("%.2f", float64(cents)/100) },
	"lineTotal": func(item LineItem) int64 { return int64(item.Quantity) * item.UnitCents },
}).Parse(`<!doctype html>
<html><head><meta charset="utf-8"><style>
body{font-family:Arial,sans-serif;color:#202124;margin:40px} header{display:flex;justify-content:space-between;align-items:start}
.logo{max-height:56px;max-width:180px} table{width:100%;border-collapse:collapse;margin-top:28px} th,td{padding:10px;border-bottom:1px solid #ddd;text-align:left}.amount{text-align:right}
</style></head><body>
<header><div>{{if .Seller.LogoURL}}<img class="logo" src="{{.Seller.LogoURL}}" alt="">{{end}}<h1>Invoice</h1><strong>{{.Seller.DisplayName}}</strong><br>{{.Seller.LegalName}}<br>{{.Seller.SupportEmail}}</div><div>Order {{.ID}}</div></header>
<h2>Bill to</h2><p>{{.Buyer.Name}}<br>{{.Buyer.BillingLine1}}<br>{{.Buyer.Email}}</p>
<table><thead><tr><th>Item</th><th>Qty</th><th class="amount">Amount</th></tr></thead><tbody>{{range .Items}}<tr><td>{{.Description}}</td><td>{{.Quantity}}</td><td class="amount">{{$.Currency}} {{money (lineTotal .)}}</td></tr>{{end}}</tbody><tfoot><tr><th colspan="2">Total</th><th class="amount">{{.Currency}} {{money .TotalCents}}</th></tr></tfoot></table>
</body></html>`))

type invoiceView struct {
	Order
	TotalCents int64
}

func RenderHTML(w io.Writer, order Order) error {
	return invoiceTemplate.Execute(w, invoiceView{Order: order, TotalCents: order.TotalCents()})
}
