# TechET Pay

A Chapa-style payment gateway **sandbox**: Go server + hosted checkout + merchant dashboard (web) + native Android merchant app.
Payment rails (Telebirr, CBE Birr, M-Pesa, Amole, cards, bank payouts) are **simulated**. No real money moves.

```
techet-pay/
  server/            Go 1.24 server, zero dependencies (API, checkout, dashboard, ledger, webhooks)
  android/           Native Android app (Kotlin + Jetpack Compose), opens in Android Studio
  render.yaml        Render blueprint (free plan, Docker)
  docker-compose.yml Run on any VPS
  deploy.ps1         One command: GitHub + Render + APK  (Windows PowerShell)
  .github/workflows  Builds the APK, runs server tests
```

## One-command deploy (Windows)

```powershell
cd path\to\techet-pay
powershell -ExecutionPolicy Bypass -File .\deploy.ps1
```

It will: install `git`/`gh` if missing, sign you in to GitHub, push the repo, create a **free Render web service**,
wait until it is live, build the APK on GitHub Actions, and download `TechET-Pay.apk` next to the script.

You need (both free, no card): a **Render API key** (Account Settings > API Keys) and an **Upstash Redis** database
(console.upstash.com > Create > copy REST URL + REST TOKEN). Upstash keeps accounts and balances alive while the free server sleeps.
If the automatic Render step fails, the script opens Render's one-click blueprint page instead.

Useful switches: `-Private`, `-SkipRender`, `-SkipApk`, `-ServerUrl https://my-server`, `-Region frankfurt|oregon|singapore|ohio|virginia`.

## Run locally

```bash
cd server
go run .                      # http://localhost:8080/dashboard
go test -race ./...
```

Environment: `PORT` / `TECHET_ADDR`, `TECHET_BASE_URL` (public https address for checkout links), `TECHET_DATA` (snapshot file),
`UPSTASH_REDIS_REST_URL` + `UPSTASH_REDIS_REST_TOKEN` (off-box snapshots), `TECHET_ADMIN_EMAIL` / `TECHET_ADMIN_PASSWORD` (sign-in for the owner panel at `/admin`; the panel stays off until a password is set), `TECHET_FEE_BPS` (250 = 2.5%), `TECHET_PAYOUT_FEE` (5.00).

## Move to a real server later

```bash
docker compose up -d --build        # data lives in the techet-data volume
```
Put HTTPS in front (Caddy example, 2 lines):
```
pay.yourdomain.com {
    reverse_proxy localhost:8080
}
```
Set `TECHET_BASE_URL=https://pay.yourdomain.com`, rebuild the APK with `-PserverUrl=https://pay.yourdomain.com` (or change the server address in the app's sign-in screen).
Remove the Upstash variables there: a real server keeps its own disk.

## Android app

Open the `android/` folder in Android Studio (Ladybug or newer, JDK 17) and press Run.
- Emulator: the app defaults to `http://10.0.2.2:8080` (your computer's `go run .` server).
- Real phone: set the server address on the sign-in screen (`Server settings`) to your Render address.
- Build from the command line: `cd android && ./gradlew assembleDebug -PserverUrl=https://your-server`.
- CI: every push touching `android/` builds `TechET-Pay.apk` and publishes it at `Releases > apk-latest`.

The APK is debug-signed (installable, not Play-Store ready). For Play Store: create a release keystore and a `signingConfigs` block.

## Sandbox test data

| Thing | Value |
|---|---|
| Wallet OTP / card code | `123456` (3 wrong tries fail the payment) |
| Phone ending `0000` | insufficient funds |
| Card `4242 4242 4242 4242` | success (any future expiry, any CVC) |
| Card `4000 0000 0000 0002` | always declined |
| Payout account ending `0000` | rejected by bank, refunded |

## API (Chapa-compatible shape)

Auth: `Authorization: Bearer <secret key>` (shown once at signup; rotate in the app or dashboard).

| Method | Path | Purpose |
|---|---|---|
| POST | `/v1/transaction/initialize` | create payment, returns `data.checkout_url` |
| GET | `/v1/transaction/verify/{tx_ref}` | payment status |
| GET | `/v1/transactions` | list payments |
| GET | `/v1/balance` | available balance |
| POST | `/v1/transfers` | payout to a bank/wallet |
| GET | `/v1/transfers/verify/{reference}` | payout status |
| GET | `/v1/banks`, `/v1/payment-methods` | public lists |

Webhooks: `charge.success`, `charge.failed`, `transfer.success`, `transfer.failed`. Headers `X-TechET-Timestamp`,
`X-TechET-Signature` = hex(HMAC-SHA256(signing secret, timestamp + "." + body)). Failed deliveries retry with backoff.

## Free-tier limits to know

- Render free servers sleep after 15 idle minutes (about 1 minute to wake) and are not meant for production.
- Upstash free: 500K commands/month. The server saves a snapshot at most every 30 seconds of activity and again on shutdown (SIGTERM), so a hard crash can lose the last few seconds.
- Everything in memory is snapshotted as one JSON document: fine for a sandbox, replace with PostgreSQL before real volume.

## Before real money

1. A National Bank of Ethiopia payment-operator licence. Ask a lawyer first.
2. Real rails: implement the `Provider` interface in `server/providers.go` (Telebirr, banks, card processor; use the processor's hosted card fields for PCI scope).
3. PostgreSQL with transactions instead of the JSON snapshot; keep the ledger append-only.
4. Merchant KYC/AML, settlement reconciliation, an admin back-office, audit log, monitoring, backups, a signed release APK.

## Install as a desktop app
Open `/dashboard` (merchants) or `/admin` (owner) in Chrome or Edge and click the install icon in the address bar
(or menu, "Install TechET Pay"). Each one gets its own window, Start-menu entry and icon. On Android Chrome use "Add to Home screen".

## Updating the Android app
The APK is signed with a fixed key (`android/app/techet.keystore`), so a new APK installs over the old one.
APKs built before that key was added must be uninstalled once first.
