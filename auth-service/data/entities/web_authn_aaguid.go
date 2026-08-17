package entities

import (
	uuid "github.com/satori/go.uuid"
)

// Fallback nicknames, used when the AAGUID is unknown or absent.
const (
	webAuthnNamePhone       = "Phone or tablet"
	webAuthnNameSecurityKey = "Security key"
	webAuthnNameSynced      = "Synced passkey"
	webAuthnNameThisDevice  = "This device"
)

// webAuthnAAGUIDNames maps authenticator models to the names their
// vendors use.
//
// These are deliberately the same labels the browser's own picker
// shows — "iCloud Keychain", "Google Password Manager", "Windows
// Hello". Someone who picked an option in that prompt should find an
// entry bearing the same words in their passkey list; anything else
// makes them wonder whether the right thing got saved.
//
// VERIFY BEFORE SHIPPING. These are transcribed from the community
// registry and I cannot check them from here:
//
//	https://github.com/passkeydeveloper/passkey-authenticator-aaguids
//
// A wrong entry produces a confidently wrong label, which is worse
// than none. A missing entry is harmless — it falls through to the
// transport heuristic, which covers the common shapes on its own.
//
// This map is display-only. Nothing security-relevant may depend on
// it: AAGUID is self-reported by the authenticator and unverified
// without attestation, so treating it as proof of anything would be a
// mistake. It decides a string in a list, nothing more.
var webAuthnAAGUIDNames = map[string]string{
	// Platform authenticators.
	"fbfc3007-154e-4ecc-8c0b-6e020557d7bd": "iCloud Keychain",
	"adce0002-35bc-c60a-648b-0b25f1f05503": "Chrome on Mac",
	"ea9b8d66-4d01-1d21-3ce4-b6b48cb575d4": "Google Password Manager",
	"08987058-cadc-4b81-b6e1-30de50dcbe96": "Windows Hello",
	"9ddd1817-af5a-4672-a2b9-3e3dd95000a9": "Windows Hello",
	"6028b017-b1d4-4c02-b4b3-afcdafc96bb2": "Windows Hello",
	"dd4ec289-e01d-41c9-bb89-70fa845d4bf2": "iCloud Keychain (Managed)",

	// Password managers.
	"bada5566-a7aa-401f-bd96-45619a55120d": "1Password",
	"d548826e-79b4-db40-a3d8-11116f7e8349": "Bitwarden",
	"531126d6-e717-415c-9320-3d9aa6981239": "Dashlane",
	"b84e4048-15dc-4dd0-8640-f4f60813c8af": "NordPass",
	"0ea242b4-43c4-4a1b-8b17-dd6d0b6baec6": "Keeper",

	// Hardware keys.
	"cb69481e-8ff7-4039-93ec-0a2729a154a8": "YubiKey 5 Series",
	"ee882879-721c-4913-9775-3dfcce97072a": "YubiKey 5 Series",
	"fa2b99dc-9e39-4257-8f92-4a30d23c4118": "YubiKey 5 NFC",
	"2fc0579f-8113-47ea-b116-bb5a8db9202a": "YubiKey 5 Series",
	"73bb0cd4-e502-49b8-9c6f-b59445bf720b": "YubiKey 5 FIPS",
	"85203421-48f9-4355-9bc8-8a53846e5083": "YubiKey 5Ci FIPS",
	"d8522d9f-575b-4866-88a9-ba99fa02f35b": "YubiKey Bio",
	"f8a011f3-8c0a-4d15-8006-17111f9edc7d": "Security Key by Yubico",
	"b92c3f9a-c014-4056-887f-140a2501163b": "Security Key by Yubico",
	"6d44ba9b-f6ec-2e49-b930-0c8fe920cb73": "Security Key NFC by Yubico",
	"149a2021-8ef6-4133-96b8-81f8d5b7f1f5": "Security Key NFC by Yubico",
	"a4e9fc6d-4cbe-4758-b8ba-37598bb5bbaa": "Security Key NFC by Yubico",
	"9c835346-796b-4c27-8898-d6032f515cc5": "Cryptnox FIDO2",
	"e1a96183-5016-4f24-b55b-e3ae23614cc6": "Titan Security Key",
	"39a5647e-1853-446c-a1f6-a79bae9f5bc7": "IDmelon",
	"3789da91-f943-46bc-95c3-50ea2012f03a": "NEOWAVE Winkeo",
	"ab32f0c6-2239-afbb-c470-d2ef4e254db7": "TOKEN2 FIDO2",
}

// DeriveNickname produces the label for a freshly enrolled credential.
//
// Called once, at registration. The person is never asked to name a
// passkey up front, because before the browser prompt resolves they
// have no idea what they are naming — the options range from "Touch
// ID" to "Your Chrome Profile", and no one guesses that in advance.
// Deriving it afterwards means the default is usually better than
// anything they would have typed, and the rename action is right
// there for the cases where it isn't.
//
// MUST NOT run on any path other than registration. A person who
// renames a passkey has said what they want it called, and a later
// call here would silently discard that.
func (c *WebAuthnCredential) DeriveNickname() string {
	if name := c.aaguidName(); name != "" {
		return name
	}

	// No AAGUID match. Transports still describe the shape of the
	// thing, which is most of what makes an entry recognisable in a
	// list — "Security key" is a perfectly good label even when we
	// cannot say whose.
	for _, t := range c.Transports {
		switch t {
		case "hybrid":
			return webAuthnNamePhone
		case "usb", "nfc", "ble":
			return webAuthnNameSecurityKey
		}
	}

	// Nothing but the backup flags left. Synced vs device-bound is
	// still the distinction that matters most to the person, so lead
	// with it rather than falling back to a bare "Passkey".
	if c.IsSynced() {
		return webAuthnNameSynced
	}

	return webAuthnNameThisDevice
}

// aaguidName resolves the authenticator model to a vendor name, or ""
// when unknown.
//
// An all-zero AAGUID is common and deliberate: many authenticators
// report it to avoid being fingerprinted by model. uuid.Nil therefore
// gets no special handling — it simply misses the map like any other
// unrecognised value and falls through to the transport heuristic.
func (c *WebAuthnCredential) aaguidName() string {
	if len(c.AAGUID) != 16 {
		return ""
	}

	id, err := uuid.FromBytes(c.AAGUID)
	if err != nil {
		return ""
	}

	return webAuthnAAGUIDNames[id.String()]
}
