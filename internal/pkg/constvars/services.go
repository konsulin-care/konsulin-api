package constvars

// ServiceType represents known purchasable services in the payment by service domain.
type ServiceType string

const (
	ServiceAnalyze           ServiceType = "analyze"
	ServiceReport            ServiceType = "report"
	ServicePerformanceReport ServiceType = "performance-report"
	ServiceAccessDataset     ServiceType = "access-dataset"
)

// ServiceMinQuantity is a typed constant for minimum allowed quantity per service.
type ServiceMinQuantity int

const (
	DefaultMinQuantityAnalyze           ServiceMinQuantity = 10
	DefaultMinQuantityReport            ServiceMinQuantity = 1
	DefaultMinQuantityPerformanceReport ServiceMinQuantity = 1
	DefaultMinQuantityAccessDataset     ServiceMinQuantity = 1
)

// ServiceToMinQuantity maps each ServiceType to its minimum quantity requirement.
var ServiceToMinQuantity = map[ServiceType]ServiceMinQuantity{
	ServiceAnalyze:           DefaultMinQuantityAnalyze,
	ServiceReport:            DefaultMinQuantityReport,
	ServicePerformanceReport: DefaultMinQuantityPerformanceReport,
	ServiceAccessDataset:     DefaultMinQuantityAccessDataset,
}

// KnownServices lists all supported services. Useful for validation.
var KnownServices = []ServiceType{
	ServiceAnalyze,
	ServiceReport,
	ServicePerformanceReport,
	ServiceAccessDataset,
}

// PaymentServiceType represents the type of payment service for Xendit invoice callbacks.
// These variable will be used to indicate which service the payment
// is for.
type PaymentServiceType string

// list of known paid services
const (
	AppointmentPaymentService PaymentServiceType = "appointment"
	WebhookPaymentService     PaymentServiceType = "webhook"
)
