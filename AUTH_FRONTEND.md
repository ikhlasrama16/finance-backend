# Frontend auth integration

The web and API share `finance.mikra.my.id`, so browser session cookies work without storing a token in localStorage.

## Startup

Call `GET /api/v1/auth/me` when the app loads.

- `200`: keep the returned user in UI state and render the application.
- `401`: redirect/render the login screen.

## Login

```http
POST /api/v1/auth/login
Content-Type: application/json

{"email":"owner@example.com","password":"..."}
```

On `200`, the browser receives the HttpOnly `finance_session` cookie automatically. Do not try to read, save, or manually attach it. For cross-origin development requests, use `credentials: "include"`; same-origin production requests include the cookie automatically.

On `401`, show a generic “email atau password salah” message. Do not distinguish an unknown email from an incorrect password.

## Logout and expired sessions

Call `POST /api/v1/auth/logout`, clear user UI state, then navigate to login. For every API request, treat `401` as an expired/invalid session: stop polling or pending actions, clear UI session state, and return to login.

## Scope

This release is single-owner. Do not build registration, organization switching, or invite UI yet. The backend schema supports users and sessions, but existing financial records are not multi-tenant until the dedicated `user_id` migration is implemented.
