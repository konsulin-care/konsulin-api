package middlewares

import (
	"net/http"

	"github.com/konsulin-care/konsulin-api/internal/app/contracts"
	"github.com/konsulin-care/konsulin-api/internal/app/drivers/gateway"
	"github.com/konsulin-care/konsulin-api/internal/pkg/utils"

	"go.uber.org/zap"
)

// RequireGateway returns a middleware that guards federation-dependent routes
// behind the gateway integration status, acting as the standalone default
// response strategy for those routes.
//
// Semantics (matching the T07 gateway error mapping):
//   - a nil status counts as disabled, and a disabled gateway answers 503 with
//     the configure-the-gateway client message and dev_message
//     GATEWAY_NOT_CONFIGURED;
//   - an enabled gateway whose subject is not enrolled answers 503 with
//     dev_message GATEWAY_NOT_ENROLLED;
//   - an enabled and enrolled gateway delegates to the next handler.
//
// Responses are written through utils.BuildErrorResponse as the standard
// CustomError JSON envelope, and the next handler never runs on rejection.
//
// Parameters: status is the contracts.GatewayStatus to consult per request;
// a nil value means the gateway is not configured.
// Returns: a func(http.Handler) http.Handler middleware ready for chi routing.
func RequireGateway(status contracts.GatewayStatus) func(http.Handler) http.Handler {
	logger := zap.NewNop()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case status == nil || !status.Enabled():
				utils.BuildErrorResponse(logger, w,
					gateway.MapError(contracts.ErrGatewayNotConfigured))
			case !status.Enrolled():
				utils.BuildErrorResponse(logger, w,
					gateway.MapError(contracts.ErrGatewayNotEnrolled))
			default:
				next.ServeHTTP(w, r)
			}
		})
	}
}
