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
