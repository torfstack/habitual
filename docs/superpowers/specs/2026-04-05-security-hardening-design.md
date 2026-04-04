# Minimal Security Hardening Design

## Goal

Reduce the highest-value security risks in the current app without introducing deployment changes or broad architectural churn.

## Scope

This pass is intentionally limited to application-level hardening:

- Add HTTP server timeouts.
- Stop returning raw internal errors to clients.
- Add same-origin protection for state-changing requests.
- Stop loading frontend JavaScript from third-party CDNs.

This pass does not include:

- New deployment resources or proxy configuration.
- CSP rollout.
- Secret rotation or OAuth provider changes.
- Reworking the authentication model.

## Current State

The app uses Google OAuth for sign-in and a server-side session cookie for ongoing authentication. State-changing routes rely on the browser sending the session cookie and currently have no explicit server-side same-origin validation. The HTML layout also pulls JavaScript from third-party CDNs, and many handlers write raw internal error strings to the response body.

## Approaches Considered

### Approach 1: Minimal app-only hardening

- Add a middleware that validates `Origin` for unsafe methods and falls back to `Referer` when `Origin` is absent.
- Replace `http.ListenAndServe` with an `http.Server` configured with explicit timeouts.
- Introduce a small helper for logging internal errors while sending generic client responses.
- Vendor the JavaScript assets into `web/static/vendor` and serve them locally.

Trade-offs:

- Keeps risk low and avoids infrastructure changes.
- Provides meaningful security improvement quickly.
- Does not address all browser hardening gaps because CSP is deferred.

### Approach 2: Middleware plus browser security headers

Everything in Approach 1 plus CSP, frame protections, and related headers.

Trade-offs:

- Stronger browser-side protections.
- Not minimal in this codebase because current inline scripts would need restructuring before a strict CSP can be deployed safely.

### Approach 3: Deployment-first controls

Move more protection to ingress or reverse proxy configuration and keep app changes smaller.

Trade-offs:

- Useful in production, but violates the requirement to keep this pass minimal and app-local.
- Harder to test and reason about from this repository alone.

## Chosen Design

Use Approach 1.

The implementation will add four focused hardening changes inside the app:

1. Request validation middleware for unsafe methods.
2. Explicit HTTP server timeout configuration.
3. Centralized safe error responses with internal logging.
4. Local static copies of the frontend JavaScript dependencies.

## Design Details

### 1. Same-origin protection

Add middleware in the handler layer that runs before authenticated state-changing endpoints.

Rules:

- Apply to unsafe methods used by the app: `POST` and `DELETE`.
- If the request has an `Origin` header, it must match the app origin.
- If `Origin` is absent, allow a matching `Referer`.
- If both headers are absent, reject the request.
- Safe methods such as `GET` are unaffected.

Origin derivation:

- Use the incoming request as the source of truth.
- Build the expected origin from request scheme and host.
- Reuse existing secure-request detection so reverse-proxied HTTPS requests still validate correctly.

Failure behavior:

- Respond with `403 Forbidden`.
- Return a short generic body such as `forbidden`.
- Log enough context server-side to diagnose rejected requests if needed.

Rationale:

- This adds a server-side CSRF barrier without changing the authentication model or introducing synchronizer tokens.
- Matching against the current request origin keeps the design deployment-agnostic for this pass.

### 2. HTTP server timeouts

Replace the direct `http.ListenAndServe` call with an `http.Server`.

Timeouts:

- `ReadHeaderTimeout`: short, defensive value.
- `ReadTimeout`: bounded request read time.
- `WriteTimeout`: bounded response write time.
- `IdleTimeout`: bounded keep-alive duration.

Expected outcome:

- Better resistance to slowloris-style connection abuse.
- No routing or handler behavior changes.

### 3. Safe client error handling

Add a small helper for handler error responses.

Rules:

- Log the internal error on the server.
- Return a generic client-facing message instead of the raw error.
- Preserve specific status codes where behavior matters, such as `400`, `403`, `404`, and `500`.
- Keep intentionally user-facing validation messages narrow and explicit only where they are already part of normal UX, such as `invalid id` or `missing auth code`.

Application:

- Replace `http.Error(w, err.Error(), ...)` in handlers with the helper.
- For auth callback failures, keep the client message generic.
- Do not leak upstream provider response bodies to clients.

Rationale:

- Reduces information disclosure while keeping debugging intact through logs.

### 4. Local JavaScript assets

Move browser dependencies from third-party CDNs into the repo under `web/static/vendor`.

Assets:

- `htmx`
- `canvas-confetti`

Template changes:

- Update the layout template to reference `/static/vendor/...`.
- Generated templ output will need regeneration if the project workflow requires it.

Rationale:

- Removes runtime dependency on external script hosts.
- Reduces the chance that a CDN compromise becomes full client-side compromise.

## Files Expected To Change

- `cmd/habitual/main.go`
  - Construct and start `http.Server` with timeout configuration.
- `internal/handler/handler.go`
  - Add same-origin middleware and safe error helper integration.
- `web/components/layout.templ`
  - Replace CDN script URLs with local static asset URLs.
- `web/components/layout_templ.go`
  - Regenerated output if required by the templ workflow.
- `web/static/vendor/...`
  - Add vendored JS assets.
- Handler test files
  - Add tests for same-origin rejection/acceptance and safe error responses where practical.

## Data Flow

### Unsafe request path

1. Browser sends authenticated `POST` or `DELETE`.
2. Same-origin middleware validates `Origin` or `Referer`.
3. If valid, request continues to auth and business logic.
4. If invalid, request is rejected before state changes occur.

### Error path

1. Handler encounters an internal error.
2. Server logs the detailed cause.
3. Client receives only a generic status-appropriate message.

## Error Handling

- Same-origin validation failure returns `403`.
- Missing or invalid route parameters continue to return `400`.
- Not-found conditions continue to return `404`.
- Internal failures return generic `500` responses.
- OAuth misconfiguration and callback failures remain generic to clients.

## Testing Strategy

Add focused tests before implementation:

- Middleware tests proving:
  - same-origin `POST` is accepted,
  - cross-origin `POST` is rejected,
  - matching `Referer` is accepted when `Origin` is absent,
  - missing both `Origin` and `Referer` is rejected for unsafe methods,
  - `GET` requests are not blocked.
- Handler tests proving generic error responses do not expose raw internal error text.
- A server-construction test if timeout setup is factored into a helper; otherwise verify by direct code inspection during review.

## Risks And Mitigations

### Reverse proxy origin mismatches

Risk:

- Origin comparison can fail if scheme detection is inconsistent behind a proxy.

Mitigation:

- Reuse the current secure-request detection based on TLS or `X-Forwarded-Proto`.
- Keep the comparison logic isolated and testable.

### Vendored asset maintenance

Risk:

- Local assets must be updated manually in the future.

Mitigation:

- Keep the set small and the file names explicit so updates are straightforward.

### Behavior changes for non-browser clients

Risk:

- Clients that send unsafe methods without `Origin` or `Referer` will be rejected.

Mitigation:

- Accepting matching `Referer` covers normal browser form and HTMX flows.
- This app is browser-centric, so the compatibility risk is acceptable for this pass.

## Success Criteria

- State-changing routes reject cross-origin unsafe requests.
- Client responses no longer expose internal server errors.
- The HTTP server runs with explicit timeouts.
- Layout no longer pulls executable JavaScript from external CDNs.
- Existing app behavior remains otherwise unchanged.
