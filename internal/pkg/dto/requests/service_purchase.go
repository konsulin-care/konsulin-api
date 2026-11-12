package requests

// ServicePurchaseRequest represents the payload accepted by the service
// purchase endpoint: the provider alias (or raw party id) providing the
// listing and the listing name to buy.
type ServicePurchaseRequest struct {
	Provider string `json:"provider" validate:"required"`
	Name     string `json:"name" validate:"required"`
}
