// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_constants.go
// @for       The client identity Qoder's COSY layer fingerprints, and the two
//
//	alphabets and the key its body and payload are built from.
//
// @uses      nothing beyond the constants themselves.
// @reason    Every value here is a fact of the vendor's wire rather than a
//
//	choice made in this file: the client statics are what qodercli sends,
//	the alphabets are its body obfuscation, and the RSA key is the one its
//	server unwraps. They sit together so a reviewer can check the whole set
//	against the reference in one pass instead of finding one header value in
//	each of four files. Endpoints and token prefixes arrive with the
//	connector that reads them.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-27
package provider

// The client identity a signed request carries. These are the values the reference
// sends, which are the values the vendor's own client sends: a device on Windows,
// IDE version 1.0.0, client type 5, data policy disagreed, no organization. None of
// them is configurable, because the vendor does not offer them as configuration.
const (
	qoderIDEVersion   = "1.0.0"
	qoderClientType   = "5"
	qoderMachineType  = "5"
	qoderMachineOS    = "x86_64_windows"
	qoderDataPolicy   = "disagree"
	qoderLoginVersion = "v2"
	qoderClientIP     = "127.0.0.1"
	qoderPayloadVer   = "v1"
)

// qoderSigPathPrefix is the path segment the CDN adds and the client strips before
// signing, so the signature describes the origin path rather than the routed one.
const qoderSigPathPrefix = "/algo"

// The two intl inference hosts, split by the kind of token they serve: a device
// token on the first, an exchanged job token on the second. Measured against the
// vendor (draft 036 §5) the device host answers a job token today, so the swap is
// the reference's rule kept rather than a proven requirement; the CN entry declares
// one gateway for every kind and is never rewritten.
const (
	qoderChatBaseIntlDevice = "https://api3.qoder.sh"
	qoderModelListPath      = "/api/v2/model/list"
	qoderChatBaseIntlJob    = "https://api2.qoder.sh"
)

// The credential prefixes the connector reads: a device token from a device login, a
// job token from an exchange, and a Personal Access Token, which cannot sign and is
// exchanged first.
const (
	qoderTokenDevice = "dt-"
	qoderTokenJob    = "jt-"
	qoderTokenPAT    = "pt-"
)

// The two 64-character alphabets of the body obfuscation: standard base64's
// characters mapped one-to-one onto the vendor's own. The padding character is
// mapped too, so an encoded body never carries "=".
const (
	qoderStdAlphabet    = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	qoderCustomAlphabet = "_doRTgHZBKcGVjlvpC,@aFSx#DPuNJme&i*MzLOEn)sUrthbf%Y^w.(kIQyXqWA!"
)

// qoderRSAPublicKeyPEM is the vendor's COSY encryption key, extracted from the Qoder
// IDE and shared by every client of the scheme. One request's AES key is wrapped in
// it, and nothing in this gateway can unwrap it again, which is the point.
const qoderRSAPublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDA8iMH5c02LilrsERw9t6Pv5Nc
4k6Pz1EaDicBMpdpxKduSZu5OANqUq8er4GM95omAGIOPOh+Nx0spthYA2BqGz+l
6HRkPJ7S236FZz73In/KVuLnwI8JJ2CbuJap8kvheCCZpmAWpb/cPx/3Vr/J6I17
XcW+ML9FoCI6AOvOzwIDAQAB
-----END PUBLIC KEY-----`
