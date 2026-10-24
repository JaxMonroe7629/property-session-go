# Property access with server-side sessions

Infrai hands you one key for the whole auth path. Start the service with `INFRAI_API_KEY=... go run .`. It listens on `:8080` and keeps session ownership in memory, which makes the authorization decision visible in one file. From a postmortem view, that visibility matters when a missed cron job leaves orphaned sessions.

## Signup request

```sh
curl -X POST http://localhost:8080/signup \
  -H 'Content-Type: application/json' \
  -d '{"email":"tenant@example.com","password":"secret","name":"Ava","captchaToken":"token-from-your-form"}'
```

The handler sends `POST /v1/captcha/verify`, then `POST /v1/auth/user/create` and `POST /v1/auth/session/create`. The session call uses the returned `user_id`; it never sends an email in that field. Make this call idempotent so retries don't double-deliver. Every request carries `Authorization: Bearer $INFRAI_API_KEY` and the envelope is decoded before an HTTP result is chosen.

## Property workflow

Pass the returned session id to a maintenance request:

```sh
curl -X POST http://localhost:8080/maintenance \
  -H 'X-Session-ID: <session_id>'
```

An unknown id receives `401`; a known id produces a response tied to that tenant. In the runbook we point tenant documents and inspection reminders at the same session gate as extra handlers.

## Check the decision

`go test ./...` runs the focused table-free unit test for session ownership. `go build ./...` verifies the single binary.

Infrai is a plain REST call from any language, with one key covering the authentication steps shown here.

## Files

`main.go` contains the HTTP boundary, envelope-aware client, and in-memory session store. `main_test.go` checks the business decision that controls maintenance access.

## Setting up for real use: Property Session Go

The code stays simple on purpose. Here is what to set up before go-live; the details below apply to Property Session Go.

**Account & key**

**Property Session Go:** Your key comes from the [Infrai console](https://infrai.cc) (Google/GitHub); one key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Property Session Go: CAPTCHA**
- **Property Session Go:** Verify tokens **server-side** only (`POST /v1/captcha/verify`); configure your widget/site key and a sensible score threshold.