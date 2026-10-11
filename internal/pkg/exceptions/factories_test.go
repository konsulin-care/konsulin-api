package exceptions

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrorFactoriesHTTPContract(t *testing.T) {
	groups := []struct {
		status    int
		factories []func(error) *CustomError
	}{
		{http.StatusBadRequest, []func(error) *CustomError{
			ErrImageValidation, ErrInputValidation, ErrBuildRequest, ErrCannotParseMultipartForm, ErrCannotParseDate,
			ErrPasswordDoNotMatch, ErrEmailAlreadyExist, ErrPhoneNumberAlreadyRegistered, ErrUsernameAlreadyExist,
			ErrWhatsAppOTPInvalid, ErrInvalidRoleType, ErrUnknownRoleType, ErrReadBody, ErrCannotParseJSON, ErrCannotParseTime,
			func(err error) *CustomError { return ErrURLParamIDValidation(err, "patientID") },
			func(err error) *CustomError { return ErrInvalidFormat(err, "patientID") },
			func(err error) *CustomError { return ErrResultFetchedNotUniqueFhirResource(err, "Patient") },
			ErrClientCustomMessage,
		}},
		{http.StatusUnauthorized, []func(error) *CustomError{ErrInvalidUsernameOrPassword, ErrAccountDeactivationAgeExpired, ErrTokenMissing, ErrSessionTokenMissing, ErrAnonymousTokenMissing, ErrSupertokensSessionMissing, ErrAuthInvalidRole}},
		{http.StatusForbidden, []func(error) *CustomError{ErrMissingRequestID, ErrNotMatchRoleType}},
		{http.StatusNotFound, []func(error) *CustomError{ErrUserNotExist}},
		{http.StatusGone, []func(error) *CustomError{ErrTokenResetPasswordExpired, ErrWhatsAppOTPExpired}},
		{http.StatusGatewayTimeout, []func(error) *CustomError{ErrServerDeadlineExceeded}},
		{http.StatusInternalServerError, []func(error) *CustomError{
			ErrHashPassword, ErrCannotMarshalJSON, ErrTokenGenerate, ErrMongoDBFindDocument, ErrMongoDBDeleteDocument,
			ErrMongoDBIterateDocuments, ErrMongoDBNotObjectID, ErrMongoDBUpdateDocument, ErrMongoDBInsertDocument,
			ErrPostgresDBFindData, ErrPostgresDBDeleteData, ErrPostgresDBIterateDataset, ErrPostgresDBUpdateData, ErrPostgresDBInsertData,
			ErrRedisDelete, ErrRedisGet, ErrRedisSet, ErrRedisIncrement, ErrRedisPushToList, ErrRedisPopFromList, ErrRedisAddToSet, ErrRedisGetSetMembers, ErrRedisUnlock,
			ErrCreateHTTPRequest, ErrSendHTTPRequest, ErrSupertoken, ErrServerProcess,
			func(err error) *CustomError { return ErrRedisGetNoData(err, "key") },
			func(err error) *CustomError { return ErrSMTPSendEmail(err, "localhost") },
			func(err error) *CustomError { return ErrNoDataFHIRResource(err, "Patient") },
			func(err error) *CustomError { return ErrRabbitMQPublishMessage(err, "queue") },
			func(err error) *CustomError { return ErrCreateFHIRResource(err, "Patient") },
			func(err error) *CustomError { return ErrGetFHIRResource(err, "Patient") },
			func(err error) *CustomError { return ErrGetFHIRResourceDuplicate(err, "Patient") },
			func(err error) *CustomError { return ErrUpdateFHIRResource(err, "Patient") },
			func(err error) *CustomError { return ErrDecodeResponse(err, "Patient") },
		}},
	}
	for _, group := range groups {
		for _, factory := range group.factories {
			got := factory(errors.New("root cause"))
			require.Equal(t, group.status, got.StatusCode)
			require.False(t, got.Success)
			require.NotEmpty(t, got.ClientMessage)
			require.Contains(t, got.Error(), "root cause")
			require.NotEmpty(t, got.Locations)
		}
	}
	for _, tc := range []struct {
		factory func(error) error
		status  int
		message string
	}{
		{ErrInvalidAPIKey, http.StatusUnauthorized, "Invalid API key"},
		{ErrAPIKeyRequired, http.StatusUnauthorized, "API key is required"},
		{ErrRolesRequired, http.StatusBadRequest, "roles can't be empty"},
	} {
		var got *CustomError
		require.ErrorAs(t, tc.factory(nil), &got)
		require.Equal(t, tc.status, got.StatusCode)
		require.Equal(t, tc.message, got.ClientMessage)
	}
}
