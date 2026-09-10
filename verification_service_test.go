package main

import (
	"context"
	"strings"
	"testing"
)

type recordingSender struct {
	calls int
	email Email
	key   string
}

func (s *recordingSender) Send(_ context.Context, email Email, key string) (string, error) {
	s.calls++
	s.email = email
	s.key = key
	return "msg_123", nil
}

func TestVerificationDecision(t *testing.T) {
	tests := []struct {
		name      string
		state     string
		wantCalls int
		wantState string
	}{
		{name: "pending verification sends", state: "pending_email_verification", wantCalls: 1, wantState: "verification_sent"},
		{name: "already verified is quiet", state: "email_verified", wantCalls: 0, wantState: "email_verified"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &recordingSender{}
			service, err := NewVerificationService(sender, "https://patient.example/verify-email")
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.Start(context.Background(), SignupRequest{
				Email: "patient@example.com", PatientID: "pat-17", AppointmentID: "apt-42",
				VerificationState: tt.state, VerificationToken: "one-time-token",
			})
			if err != nil {
				t.Fatal(err)
			}
			if sender.calls != tt.wantCalls || result.State != tt.wantState {
				t.Fatalf("calls/state = %d/%q, want %d/%q", sender.calls, result.State, tt.wantCalls, tt.wantState)
			}
			if sender.calls == 1 {
				if strings.Contains(sender.email.HTML, "apt-42") || strings.Contains(sender.email.Subject, "apt-42") {
					t.Fatal("patient notification exposed appointment identifier")
				}
				if !strings.HasPrefix(sender.key, "signup-verification-") {
					t.Fatalf("unexpected idempotency key %q", sender.key)
				}
			}
		})
	}
}
