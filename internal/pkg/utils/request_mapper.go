package utils

import (
	"konsulin-service/internal/pkg/dto/requests"
	"strings"
)

func MapPaymentRequestToDTO(req *requests.PaymentRequest) *requests.PaymentRequestDTO {
	return &requests.PaymentRequestDTO{
		PartnerUserID:           req.PartnerUserID,
		UseLinkedAccount:        req.UseLinkedAccount,
		PartnerTransactionID:    req.PartnerTransactionID,
		PaymentExpirationTime:   req.PaymentExpirationTime,
		NeedFrontend:            req.NeedFrontend,
		SenderEmail:             req.SenderEmail,
		ReceiveAmount:           req.ReceiveAmount,
		ListEnablePaymentMethod: strings.Join(req.ListEnablePaymentMethod, ","),
		ListEnableSOF:           strings.Join(req.ListEnableSOF, ","),
		VADisplayName:           req.VADisplayName,
		PaymentRouting:          req.PaymentRouting,
	}
}
