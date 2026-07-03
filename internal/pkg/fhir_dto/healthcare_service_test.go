package fhir_dto

import (
	"testing"
)

func TestHealthcareService_ServiceDurationMinutes(t *testing.T) {
	tests := []struct {
		name    string
		hs      *HealthcareService
		want    int
		wantErr bool
	}{
		{
			name: "valid duration returns minutes",
			hs: &HealthcareService{
				Extension: []Extension{
					{Url: "https://konsulin.id/fhir/StructureDefinition/serviceDuration", ValueDuration: &Duration{Value: ptrFloat(60)}},
				},
			},
			want:    60,
			wantErr: false,
		},
		{
			name: "nil extensions returns error",
			hs: &HealthcareService{
				Extension: nil,
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "empty extensions returns error",
			hs: &HealthcareService{
				Extension: []Extension{},
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "missing serviceDuration extension returns error",
			hs: &HealthcareService{
				Extension: []Extension{
					{Url: "https://konsulin.id/fhir/StructureDefinition/serviceBuffer", ValueDuration: &Duration{Value: ptrFloat(5)}},
				},
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "nil valueDuration.Value returns error",
			hs: &HealthcareService{
				Extension: []Extension{
					{Url: "https://konsulin.id/fhir/StructureDefinition/serviceDuration", ValueDuration: &Duration{Value: nil}},
				},
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "zero duration returns error",
			hs: &HealthcareService{
				Extension: []Extension{
					{Url: "https://konsulin.id/fhir/StructureDefinition/serviceDuration", ValueDuration: &Duration{Value: ptrFloat(0)}},
				},
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "negative duration returns error",
			hs: &HealthcareService{
				Extension: []Extension{
					{Url: "https://konsulin.id/fhir/StructureDefinition/serviceDuration", ValueDuration: &Duration{Value: ptrFloat(-10)}},
				},
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "matches by suffix not full URL domain",
			hs: &HealthcareService{
				Extension: []Extension{
					{Url: "http://dev.konsulin.id/fhir/StructureDefinition/serviceDuration", ValueDuration: &Duration{Value: ptrFloat(30)}},
				},
			},
			want:    30,
			wantErr: false,
		},
		{
			name: "multiple extensions finds correct one",
			hs: &HealthcareService{
				Extension: []Extension{
					{Url: "https://konsulin.id/fhir/StructureDefinition/fee", ValueMoney: &Money{Value: 250000, Currency: "IDR"}},
					{Url: "https://konsulin.id/fhir/StructureDefinition/serviceDuration", ValueDuration: &Duration{Value: ptrFloat(45)}},
				},
			},
			want:    45,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.hs.ServiceDurationMinutes()
			if (err != nil) != tt.wantErr {
				t.Errorf("ServiceDurationMinutes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ServiceDurationMinutes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHealthcareService_ServiceBufferMinutes(t *testing.T) {
	tests := []struct {
		name string
		hs   *HealthcareService
		want int
		ok   bool
	}{
		{
			name: "valid buffer returns minutes and true",
			hs: &HealthcareService{
				Extension: []Extension{
					{Url: "https://konsulin.id/fhir/StructureDefinition/serviceBuffer", ValueDuration: &Duration{Value: ptrFloat(10)}},
				},
			},
			want: 10,
			ok:   true,
		},
		{
			name: "nil extensions returns 0, false",
			hs: &HealthcareService{
				Extension: nil,
			},
			want: 0,
			ok:   false,
		},
		{
			name: "empty extensions returns 0, false",
			hs: &HealthcareService{
				Extension: []Extension{},
			},
			want: 0,
			ok:   false,
		},
		{
			name: "missing serviceBuffer extension returns 0, false",
			hs: &HealthcareService{
				Extension: []Extension{
					{Url: "https://konsulin.id/fhir/StructureDefinition/serviceDuration", ValueDuration: &Duration{Value: ptrFloat(30)}},
				},
			},
			want: 0,
			ok:   false,
		},
		{
			name: "nil valueDuration.Value returns 0, false",
			hs: &HealthcareService{
				Extension: []Extension{
					{Url: "https://konsulin.id/fhir/StructureDefinition/serviceBuffer", ValueDuration: &Duration{Value: nil}},
				},
			},
			want: 0,
			ok:   false,
		},
		{
			name: "zero buffer returns 0, false",
			hs: &HealthcareService{
				Extension: []Extension{
					{Url: "https://konsulin.id/fhir/StructureDefinition/serviceBuffer", ValueDuration: &Duration{Value: ptrFloat(0)}},
				},
			},
			want: 0,
			ok:   false,
		},
		{
			name: "negative buffer returns 0, false",
			hs: &HealthcareService{
				Extension: []Extension{
					{Url: "https://konsulin.id/fhir/StructureDefinition/serviceBuffer", ValueDuration: &Duration{Value: ptrFloat(-5)}},
				},
			},
			want: 0,
			ok:   false,
		},
		{
			name: "matches by suffix not full URL domain",
			hs: &HealthcareService{
				Extension: []Extension{
					{Url: "http://dev.konsulin.id/fhir/StructureDefinition/serviceBuffer", ValueDuration: &Duration{Value: ptrFloat(15)}},
				},
			},
			want: 15,
			ok:   true,
		},
		{
			name: "multiple extensions finds correct one",
			hs: &HealthcareService{
				Extension: []Extension{
					{Url: "https://konsulin.id/fhir/StructureDefinition/fee", ValueMoney: &Money{Value: 250000, Currency: "IDR"}},
					{Url: "https://konsulin.id/fhir/StructureDefinition/serviceDuration", ValueDuration: &Duration{Value: ptrFloat(30)}},
					{Url: "https://konsulin.id/fhir/StructureDefinition/serviceBuffer", ValueDuration: &Duration{Value: ptrFloat(5)}},
				},
			},
			want: 5,
			ok:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.hs.ServiceBufferMinutes()
			if ok != tt.ok {
				t.Errorf("ServiceBufferMinutes() ok = %v, want %v", ok, tt.ok)
				return
			}
			if got != tt.want {
				t.Errorf("ServiceBufferMinutes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func ptrFloat(f float64) *float64 {
	return &f
}
