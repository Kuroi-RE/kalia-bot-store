package payment

import "net/http"

// ProviderConfig carries the settings needed to build a gateway.
type ProviderConfig struct {
	Provider        string // "midtrans" (default) or "temanqris"
	MidtransKey     string
	MidtransBaseURL string
	Acquirer        string
	QRISStatic      string
}

// NewGateway builds the configured payment gateway. Unknown providers fall back
// to Midtrans.
func NewGateway(cfg ProviderConfig, httpClient *http.Client) Gateway {
	switch cfg.Provider {
	case "temanqris":
		return NewTemanQRIS(cfg.QRISStatic)
	default:
		return NewMidtrans(cfg.MidtransKey, cfg.MidtransBaseURL, cfg.Acquirer, httpClient)
	}
}

// ManualConfirmer is implemented by gateways whose payments are confirmed by an
// admin rather than an automatic settlement signal.
type ManualConfirmer interface {
	RequiresManualConfirmation() bool
}

// RequiresManualConfirmation reports whether g needs admin confirmation.
func RequiresManualConfirmation(g Gateway) bool {
	mc, ok := g.(ManualConfirmer)
	return ok && mc.RequiresManualConfirmation()
}
