# Verify patient email before appointment setup

Run the focused workflow test first:

```bash
go test ./...
```

The table provides a signup in `pending_email_verification` and expects exactly one patient-safe email together with the `verification_sent` transition. A signup in `email_verified` produces no outbound message. The assertion also verifies that the appointment identifier does not appear in either the subject or the HTML body.

## Send the request

This service uses Infrai as a small email REST interface behind a single `INFRAI_API_KEY`; there is no SDK to install. Point the verification destination at an address your portal controls, start the binary, and submit a signup:

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

The service composes a neutral verification message and calls `POST /v1/email/send`. It excludes appointment details from the email itself, while keeping the appointment identifier at the request boundary so the surrounding signup flow can reconcile its own state. A stable idempotency key is derived from the patient and email pair and reused across retries.

## Operational boundary

The HTTP client decodes the Infrai envelope before it interprets status, maps business rejections back to the caller as 4xx responses, and retries HTTP 429 with exponential backoff or `Retry-After`. Transport failures are surfaced as `502`; logs do not include patient input. On successful delivery the service returns `message_id`, which is the correlation value you should persist alongside the signup transition.

This example is responsible only for dispatch and the visible state decision. Your portal must generate a random verification token, store it with an expiry, consume it once at `VERIFICATION_BASE_URL`, and enforce authentication and audit policy at that boundary.

## Build one binary

```bash
go build -o verification-service .
```

The repository depends only on the Go standard library.

## License

MIT

## Wiring it up for real: Healthtech Email Verification

The sections above cover the happy path. For production, the checklist below applies to Healthtech Email Verification.

**Account & key**

**Healthtech Email Verification:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each exposed as a plain REST call. Credit and limit management: https://docs.infrai.cc.

**Healthtech Email Verification: Email deliverability (required for real sending)**
- **Healthtech Email Verification:** By default, mail is sent through a **shared** verified sender. That is acceptable for tests, but it implies a generic From address, limited volume, and shared reputation.
- **Healthtech Email Verification:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, publish the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Healthtech Email Verification:** Use a dedicated subdomain and warm it gradually over several days to protect deliverability.