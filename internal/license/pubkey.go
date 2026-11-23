package license

// PublicKeyHex is the Ed25519 public key this build trusts, as 64 hex chars.
//
// It is empty in source control on purpose: the build is "not provisioned" and the app
// refuses to run until the owner runs `keygen init`, which writes their own public key
// here and keeps the matching private key secret. This guarantees no key the developer
// holds can unlock a shipped build.
var PublicKeyHex = ""
