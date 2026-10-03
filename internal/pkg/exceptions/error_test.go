package exceptions

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"konsulin-service/internal/pkg/constvars"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
)

func TestCustomError(t *testing.T) {
	plain := BuildNewCustomError(nil, http.StatusBadRequest, "client", "developer")
	require.Equal(t, http.StatusBadRequest, plain.StatusCode)
	require.Equal(t, "client", plain.ClientMessage)
	require.Equal(t, "developer", plain.Error())
	require.False(t, plain.Success)
	require.NotEmpty(t, plain.Locations)
	require.NotEmpty(t, plain.Locations[0].File)
	require.Positive(t, plain.Locations[0].Line)
	require.NotEmpty(t, plain.Locations[0].FunctionName)
	withCause := BuildNewCustomError(errors.New("cause"), http.StatusBadRequest, "client", "developer")
	require.Equal(t, "developer: cause", withCause.Error())
	timeout := BuildNewCustomError(context.DeadlineExceeded, http.StatusBadRequest, "client", "developer")
	require.Equal(t, http.StatusGatewayTimeout, timeout.StatusCode)
	require.Equal(t, constvars.ErrClientServerLongRespond, timeout.ClientMessage)
	require.Contains(t, timeout.Error(), constvars.ErrDevServerDeadlineExceeded)
	require.Equal(t, []Location{{File: constvars.ResponseUnknown, FunctionName: constvars.ResponseUnknown}}, getLocations(100000))
}

func TestHTTPErrRetryable(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("connection refused"), false},
		{ErrSendHTTPRequest(errors.New("connection refused")), true},
		{fmt.Errorf("wrapped: %w", ErrSendHTTPRequest(errors.New("transport"))), true},
		{ErrCreateHTTPRequest(errors.New("bad URL")), false},
		{ErrDecodeResponse(errors.New("JSON"), "Patient"), false},
		{ErrRedisGet(errors.New("connection refused")), false},
	} {
		require.Equal(t, tc.want, IsHTTPErrRetryable(tc.err))
	}
}

func TestValidationErrorMessages(t *testing.T) {
	v := validator.New()
	for _, tag := range []string{"unknown_rule", "phone_number", "not_past_date", "not_past_time"} {
		require.NoError(t, v.RegisterValidation(tag, func(_ validator.FieldLevel) bool { return false }))
	}
	for _, tc := range []struct{ name, tag, wantFirst, wantAll string }{
		{"required", "required", "field is required", "field is required"},
		{"length parameter", "min=3", "field must be at least 3 characters long", "field must be at least 3 characters long"},
		{"options parameter", "oneof=patient practitioner", "field must be one of [patient, practitioner]", "field must be one of [patient, practitioner]"},
		{"unknown", "unknown_rule", "field is invalid", "field is invalid"},
		{"phone", "phone_number", "phone number given is not valid", "field phone number given is not valid"},
		{"date", "not_past_date", "the date must not be in the past", "field the date must not be in the past"},
		{"time", "not_past_time", "the time must not be in the past for today's date.", "field the time must not be in the past for today's date."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			typ := reflect.StructOf([]reflect.StructField{{Name: "Field", Type: reflect.TypeOf(""), Tag: reflect.StructTag(`validate:"` + tc.tag + `"`)}})
			err := v.Struct(reflect.New(typ).Interface())
			require.Error(t, err)
			require.Equal(t, tc.wantFirst, FormatFirstValidationError(err))
			require.Equal(t, tc.wantAll, FormatAllValidationErrors(err))
		})
	}
	require.Equal(t, constvars.ErrClientCannotProcessRequest, FormatAllValidationErrors(nil))
	require.Equal(t, constvars.ErrClientCannotProcessRequest, FormatFirstValidationError(nil))
	require.Equal(t, constvars.ErrDevInvalidInput, FormatFirstValidationError(errors.New("ordinary error")))
	err := v.Struct(struct {
		Name  string `validate:"required"`
		Email string `validate:"email"`
	}{})
	require.Equal(t, "name is required", FormatFirstValidationError(err))
	require.Equal(t, "name is required, email must be a valid email", FormatAllValidationErrors(err))
}
