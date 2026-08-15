package users

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestNewUserFHIRInitializerTakesDepsStruct locks the bundled-dependencies
// constructor shape so the 10-parameter signature cannot regress.
func TestNewUserFHIRInitializerTakesDepsStruct(t *testing.T) {
	instance := NewUserFHIRInitializer(UserFHIRInitializerDeps{
		PatientFhirClient:          &MockPatientFhirClient{},
		PractitionerFhirClient:     &MockPractitionerFhirClient{},
		PractitionerRoleFhirClient: &MockPractitionerRoleFhirClient{},
		Logger:                     zap.NewNop(),
	})
	require.NotNil(t, instance)

	uc, ok := instance.(*userUsecase)
	require.True(t, ok)
	require.NotNil(t, uc.PatientFhirClient)
	require.NotNil(t, uc.PractitionerFhirClient)
	require.NotNil(t, uc.PractitionerRoleFhirClient)
	require.NotNil(t, uc.Log)
}
