package fhir_dto

import (
	"testing"
)

func TestPatient_FullName(t *testing.T) {
	tests := []struct {
		name    string
		patient Patient
		want    string
	}{
		{
			name:    "empty names, no email",
			patient: Patient{},
			want:    "",
		},
		{
			name: "empty names, with email",
			patient: Patient{
				Telecom: []ContactPoint{
					{System: ContactPointSystemEmail, Value: "john.doe@example.com"},
				},
			},
			want: "john.doe",
		},
		{
			name: "single name, family only",
			patient: Patient{
				Name: []HumanName{
					{Family: "Doe"},
				},
			},
			want: "Doe",
		},
		{
			name: "single name, given only",
			patient: Patient{
				Name: []HumanName{
					{Given: []string{"John"}},
				},
			},
			want: "John",
		},
		{
			name: "single name, prefix + given + family",
			patient: Patient{
				Name: []HumanName{
					{Prefix: []string{"Dr."}, Given: []string{"John"}, Family: "Doe"},
				},
			},
			want: "Dr. John Doe",
		},
		{
			name: "official preferred over first",
			patient: Patient{
				Name: []HumanName{
					{Use: "usual", Given: []string{"Johnny"}},
					{Use: "official", Given: []string{"John"}, Family: "Doe"},
				},
			},
			want: "John Doe",
		},
		{
			name: "usual preferred when no official",
			patient: Patient{
				Name: []HumanName{
					{Use: "nickname", Given: []string{"Johnny"}},
					{Use: "usual", Given: []string{"John"}, Family: "Doe"},
				},
			},
			want: "John Doe",
		},
		{
			name: "text takes precedence over structured fields",
			patient: Patient{
				Name: []HumanName{
					{Text: "Dr. John Doe", Given: []string{"John"}, Family: "Smith"},
				},
			},
			want: "Dr. John Doe",
		},
		{
			name: "first name used when no official or usual",
			patient: Patient{
				Name: []HumanName{
					{Use: "anonymous", Given: []string{"John"}, Family: "Doe"},
				},
			},
			want: "John Doe",
		},
		{
			name: "email fallback when no names",
			patient: Patient{
				Telecom: []ContactPoint{
					{System: ContactPointSystemEmail, Value: "user@konsulin.care"},
				},
			},
			want: "user",
		},
		{
			name: "email fallback: no @ in email",
			patient: Patient{
				Telecom: []ContactPoint{
					{System: ContactPointSystemEmail, Value: "noatsign"},
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.patient.FullName(); got != tt.want {
				t.Errorf("FullName() = %q, want %q", got, tt.want)
			}
		})
	}
}
