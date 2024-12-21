package payments

import (
	"context"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"
)

type paymentUsecase struct {
	AppointmentFhirClient contracts.AppointmentFhirClient
	InternalConfig        *config.InternalConfig
}

func NewPaymentUsecase(
	appointmentFhirClient contracts.AppointmentFhirClient,
	internalConfig *config.InternalConfig,
) contracts.PaymentUsecase {
	return &paymentUsecase{
		AppointmentFhirClient: appointmentFhirClient,
		InternalConfig:        internalConfig,
	}
}

func (uc *paymentUsecase) PaymentRoutingCallback(ctx context.Context, request *requests.PaymentRoutingCallback) error {
	appointment, err := uc.AppointmentFhirClient.FindAppointmentByID(ctx, request.PartnerTrxID)
	if err != nil {
		return err
	}

	if request.PaymentStatus == constvars.OY_COMPLETE_STATUS {
		appointment.Status = constvars.FhirAppointmentStatusBooked
	} else if request.PaymentStatus == constvars.OY_EXPIRED_STATUS || request.PaymentStatus == constvars.OY_INCOMPLETE_STATUS || request.PaymentStatus == constvars.OY_PAYMENT_FAILED_STATUS {
		appointment.Status = "cancelled"
		appointment.ReasonCode = []fhir_dto.CodeableConcept{
			{
				Coding: []fhir_dto.Coding{
					{
						System:  "http://terminology.hl7.org/CodeSystem/appointment-cancellation-reason",
						Code:    "financial",
						Display: "Cancelled due to lack of payment",
					},
				},
				Text: "Cancelled because the payment was not made.",
			},
		}
	}

	_, err = uc.AppointmentFhirClient.UpdateAppointment(ctx, appointment)
	if err != nil {
		return err
	}

	return nil
}
