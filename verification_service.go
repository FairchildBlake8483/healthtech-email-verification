package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"net/url"
)

type SignupRequest struct {
	Email             string `json:"email"`
	PatientID         string `json:"patient_id"`
	AppointmentID     string `json:"appointment_id"`
	VerificationState string `json:"verification_state"`
	VerificationToken string `json:"verification_token"`
}

type SignupResult struct {
	State     string `json:"state"`
	MessageID string `json:"message_id,omitempty"`
}

type EmailSender interface {
	Send(ctx context.Context, email Email, idempotencyKey string) (string, error)
}

type VerificationService struct {
	sender  EmailSender
	baseURL string
}

func NewVerificationService(sender EmailSender, verificationBaseURL string) (*VerificationService, error) {
	parsed, err := url.Parse(verificationBaseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, errors.New("VERIFICATION_BASE_URL must be an absolute HTTPS URL")
	}
	return &VerificationService{sender: sender, baseURL: verificationBaseURL}, nil
}

func (s *VerificationService) Start(ctx context.Context, request SignupRequest) (SignupResult, error) {
	if request.Email == "" || request.PatientID == "" || request.AppointmentID == "" {
		return SignupResult{}, errors.New("email, patient_id, and appointment_id are required")
	}
	if request.VerificationState != "pending_email_verification" {
		return SignupResult{State: request.VerificationState}, nil
	}
	if request.VerificationToken == "" {
		return SignupResult{}, errors.New("verification_token is required while verification is pending")
	}

	key := stableKey(request.PatientID, request.Email)
	link := s.baseURL + "?token=" + url.QueryEscape(request.VerificationToken)
	messageID, err := s.sender.Send(ctx, Email{
		To:      request.Email,
		Subject: "Verify your patient portal email",
		HTML: fmt.Sprintf(
			`<p>Confirm this email address to continue setting up your patient portal.</p><p><a href="%s">Verify email</a></p><p>If you did not request this, you can ignore this message.</p>`,
			html.EscapeString(link),
		),
	}, "signup-verification-"+key)
	if err != nil {
		return SignupResult{}, err
	}
	return SignupResult{State: "verification_sent", MessageID: messageID}, nil
}

func stableKey(patientID, email string) string {
	sum := sha256.Sum256([]byte(patientID + "\x00" + email))
	return hex.EncodeToString(sum[:])
}
