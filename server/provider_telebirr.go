package main

// Telebirr adapter.
//
// STATUS: placeholder. It refuses to run, so nothing can pretend to take real money.
//
// TO MAKE IT REAL (after you sign the agreement and receive the integration guide):
//  1. Check the credential fields below match what the provider gave you; add or rename
//     fields here (they appear automatically on the admin Providers page).
//  2. Implement Check, Charge and ParseCallback by following the provider's guide.
//     ParseCallback MUST verify the provider's signature and reject anything unsigned.
//     Never log or return credential values.
//  3. Change Implemented() to return true.
//  4. Enter the credentials on the Providers page, press "Test connection", then switch
//     the provider to Live. Finally give approved merchants "Live access".
//
// The sandbox is not affected by any of this: it keeps working for every merchant.

type adapter_telebirr struct{ stubAdapter }

func init() {
	registerLive(adapter_telebirr{stubAdapter{
		code: "telebirr", name: "Telebirr", kind: "payment",
		fields: append([]CredField{
			{Key: "app_id", Label: "App ID", Hint: "Given by Ethio telecom after the merchant agreement", Secret: false, Required: true, Multiline: false},
			{Key: "app_secret", Label: "App secret / key", Hint: "Keep private", Secret: true, Required: true, Multiline: false},
			{Key: "short_code", Label: "Merchant short code", Hint: "", Secret: false, Required: true, Multiline: false},
			{Key: "public_key", Label: "Provider public key (PEM)", Hint: "Used to verify callbacks and encrypt requests", Secret: false, Required: true, Multiline: true},
			{Key: "base_url", Label: "API base address", Hint: "From the integration guide", Secret: false, Required: true, Multiline: false},
		}, commonFields()...),
	}})
}
