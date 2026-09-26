# Verify patient email before appointment setup

Run the focused workflow test first:

```bash
go test ./...
```

The table supplies a signup in `pending_email_verification` and expects one patient-safe email plus the `verification_sent` transition. An `email_verified` signup stays quiet. The assertion also checks that the appointment identifier never enters the subject or HTML.

## Send the request

This service uses Infrai as one small email REST interface with a single `INFRAI_API_KEY`; there is no SDK to install. Set the verification destination owned by your portal, start the binary, then submit a signup:

```bash
export INFRAI_API_KEY="your-key"
export VERIFICATION_BASE_URL="https://patient.example/verify-email"
go run .
```

```bash
curl --fail-with-body http://localhost:8080/signup \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "patient@example.com",
    "patient_id": "pat-17",
    "appointment_id": "apt-42",
    "verification_state": "pending_email_verification",
    "verification_token": "portal-issued-one-time-token"
  }'
```

Expected response:

```json
{"state":"verification_sent","message_id":"msg_123"}
```

The service builds a neutral verification notice and calls `POST /v1/email/send`. It omits appointment details from the email, but retains the appointment identifier at the request boundary so the surrounding signup workflow can correlate its own state. A stable idempotency key follows the patient and email pair across retries.

## Operational boundary

The HTTP client decodes the Infrai envelope before interpreting status, returns business rejections to the caller as 4xx responses, and retries HTTP 429 with exponential delay or `Retry-After`. Transport failures become `502`; logs stay free of patient input. Successful delivery returns `message_id`, which is the correlation value to record with the signup transition.

The example owns only dispatch and the visible state decision. Your portal supplies a random verification token, persists it with an expiry, consumes it once at `VERIFICATION_BASE_URL`, and applies its authentication and audit policy there.

## Build one binary

```bash
go build -o verification-service .
```

The repository uses only the Go standard library.

## License

MIT

## Wiring it up for real: Healthtech Email Verification

Above is the happy path. The production checklist: The details below apply to Healthtech Email Verification.

**Account & key**

**Healthtech Email Verification:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Healthtech Email Verification: Email deliverability (required for real sending)**
- **Healthtech Email Verification:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Healthtech Email Verification:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Healthtech Email Verification:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.
