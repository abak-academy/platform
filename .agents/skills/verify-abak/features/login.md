# Login

Students sign in at `/login` and land on the student home. Admins sign in at `/admin/login` and land in `/admin`. A wrong password keeps the user on the form and shows an alert.

## Sub-features

- `login-student` signs a student in through `/login` and lands on `/`.
- `login-admin` signs a super_admin in through `/admin/login` and lands on `/admin`.
- `login-wrong-password` rejects a bad password with a visible alert and no navigation.
- `login-otp` sends an OTP-enabled account to `/otp` (not driven: needs Fazpass).
- `login-google` uses Google sign-in (not driven: the button is hidden without `NEXT_PUBLIC_GOOGLE_CLIENT_ID`).

## How to get to it (user POV)

- Open `/login`, fill `Email atau Username` and `Kata Sandi`, press `Masuk`.
- Open `/admin/login` with the same fields and button.
- Any gated page (`/`, `/admin`) redirects to `/login` when signed out.

## Driving it with abak-verify

Preconditions:

- `abak-verify doctor` prints `OK`.
- The seeded student and admin exist (`abak-verify seed` re-creates them if a journey changed their password).

- **Student sign-in.** Fill both fields and press `Masuk`. Run `abak-verify journey login -g login-student`. The URL becomes `/`, `abak-auth` in localStorage holds a token, and `GET /auth/me` with it returns `verify-student@example.test` with role `student` (`login-student.me.json`).
- **Admin sign-in.** The same steps on `/admin/login`. Run `abak-verify journey login -g login-admin`. The URL becomes `/admin` and `/auth/me` returns role `super_admin` (`login-admin.me.json`).
- **Wrong password.** Submit `definitely-wrong`. Run `abak-verify journey login -g login-wrong-password`. An element with role `alert` appears and the URL stays `/login`. The API log shows `POST /auth/login 401`.
- **Proof.** `login-*.before.png` (filled form, password masked), `login-*.after.png`, `login-*.aria.txt`, and `login-*.browser.log` in the evidence dir.

## Gotchas

- The first visit to a route compiles it in `next dev`. A cold `/` stranded the student shell on `Memuat…` for over 30s. `up` pre-compiles the main routes, and a new route a journey visits needs adding there.
- Loading `/` as a student fires `POST /orders` (201), which opens a cart. Cart state then carries over between journeys that share the student.
- `POST /auth/login` is rate limited (`LoginRateLimiter`). Tight retry loops on wrong passwords can start returning 429 instead of 401.
- A session pre-seeded into localStorage (like the `web/e2e` helpers do) skips the form, so it does not count as login proof.
