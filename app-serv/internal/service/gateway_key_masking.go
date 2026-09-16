// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/gateway_key_masking.go
// @for       The key_hint transform: "<prefix>…<last four>" (SPEC-API-001 §4).
// @uses      standard library only.
// @reason    SPEC-API-001 §4 fixes the hint as sk-…abcd and §4 makes the key
//
//	prefix configurable, so the transform needs the configured prefix
//	and belongs with the service that issues the credential. It must
//	never reveal more than the family plus the trailing characters.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtechstore.com>
// @layer     service
// @stability experimental
// @since     2026-09-16
package service

// keyHintTail is the number of trailing characters a key_hint reveals.
const keyHintTail = 4

// MaskGatewayKey renders the hint shown in list and detail responses. It keeps
// the configured prefix (so the user recognises the credential family) and the
// trailing characters, eliding everything between.
//
// A value no longer than prefix+tail is returned unchanged: the masked form
// would reconstruct it entirely, hiding nothing while looking concealed. The
// mint path never reaches that branch, because a generated key is far longer.
func MaskGatewayKey(prefix, plaintext string) string {
	if len(plaintext) <= len(prefix)+keyHintTail {
		return plaintext
	}
	return prefix + "\u2026" + plaintext[len(plaintext)-keyHintTail:]
}
