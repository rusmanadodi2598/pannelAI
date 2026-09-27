// Package provider implements the per-provider connectors the gateway calls.
//
// @file      internal/provider/qoder_encoding.go
// @for       The body obfuscation Qoder's chat endpoint reads as Encode=1.
//
// @uses      encoding/base64, strings.
// @reason    The vendor puts a WAF in front of the chat endpoint and the
//
//	official client does not send plaintext JSON to it, so a gateway
//	that sent a readable body would be pattern-matched. The scheme is
//	base64, then thirds reordered, then a character substitution: an
//	obfuscation, not encryption, and the whole reason it can live in
//	one pure function.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     config
// @stability experimental
// @since     2026-09-27
package provider

import "encoding/base64"

// qoderEncodeTable maps one standard base64 character to the vendor's own, plus
// the padding character to "$". It is built from the two alphabets rather than
// typed as a table so the pair can be checked against the reference as the two
// 64-character strings the reference declares.
var qoderEncodeTable = func() [256]byte {
	var table [256]byte
	for i := range table {
		table[i] = byte(i)
	}
	for i := 0; i < len(qoderStdAlphabet); i++ {
		table[qoderStdAlphabet[i]] = qoderCustomAlphabet[i]
	}
	table['='] = '$'
	return table
}()

// qoderEncodeBody renders a request body in the vendor's alphabet: base64 with
// standard padding, split into thirds and reordered tail-mid-head, then
// substituted character by character. The decode reverses the same three steps,
// so an empty body stays empty and a body of any length keeps its length.
func qoderEncodeBody(plain []byte) string {
	std := base64.StdEncoding.EncodeToString(plain)
	n := len(std)
	if n == 0 {
		return ""
	}
	third := n / 3
	rearranged := std[n-third:] + std[third:n-third] + std[:third]

	encoded := make([]byte, n)
	for i := 0; i < n; i++ {
		encoded[i] = qoderEncodeTable[rearranged[i]]
	}
	return string(encoded)
}
