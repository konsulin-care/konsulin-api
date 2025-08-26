package constvars

const (
	OyPaymentRoutingResource = "PaymentRouting"
)

const (
	OY_MOCK_ACCOUNT_NUMBER_SUCCESS              = "1234567890"
	OY_MOCK_ACCOUNT_NUMBER_FAILED               = "1234567891"
	OY_MOCK_ACCOUNT_NUMBER_FAILED_FORCE_CREDIT  = "1234567892"
	OY_MOCK_ACCOUNT_NUMBER_PENDING              = "1234567893"
	OY_MOCK_ACCOUNT_NUMBER_PENDING_FORCE_CREDIT = "1234567894"
)

// OYPaymentRoutingStatus is a typed payment status returned by OY
type OYPaymentRoutingStatus string

const (
	OYPaymentRoutingStatusCreated            OYPaymentRoutingStatus = "CREATED"
	OYPaymentRoutingStatusWaitingPayment     OYPaymentRoutingStatus = "WAITING_PAYMENT"
	OYPaymentRoutingStatusPaymentInProgress  OYPaymentRoutingStatus = "PAYMENT_IN_PROGRESS"
	OYPaymentRoutingStatusDisburseInProgress OYPaymentRoutingStatus = "DISBURSE_IN_PROGRESS"
	OYPaymentRoutingStatusComplete           OYPaymentRoutingStatus = "COMPLETE"
	OYPaymentRoutingStatusIncomplete         OYPaymentRoutingStatus = "INCOMPLETE"
	OYPaymentRoutingStatusExpired            OYPaymentRoutingStatus = "EXPIRED"
	OYPaymentRoutingStatusFailed             OYPaymentRoutingStatus = "PAYMENT_FAILED"
)
