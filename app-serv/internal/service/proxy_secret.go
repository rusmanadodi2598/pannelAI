// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/proxy_secret.go
// @for       The password sealing helpers the proxy CRUD and probe paths share.
// @uses      internal/domain.
// @reason    §7.11 keeps the password write-only, so both the create/update
// path and the probe path seal and open through the same helpers; they live
// apart from the CRUD use cases because AGENTS.md §1.1 caps a file at 250 lines.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// seal seals a plaintext password, or returns "" for an absent one.
func (s *ProxyService) seal(password string) (string, error) {
	if password == "" {
		return "", nil
	}
	sealed, err := s.sealer.Seal(password)
	if err != nil {
		return "", domain.NewInternalError("the proxy password could not be stored")
	}
	return sealed, nil
}

// sameSecret reports whether the typed password is the stored one.
//
// A secret that cannot be opened compares as different: it is already broken,
// and refusing the save would leave the operator no way to replace it.
func (s *ProxyService) sameSecret(proxy domain.Proxy, plaintext string) bool {
	if !proxy.HasPassword() {
		return false
	}
	current, err := s.sealer.Open(proxy.PasswordEncrypted())
	return err == nil && current == plaintext
}

// open unseals a stored password for the duration of one probe.
func (s *ProxyService) open(proxy domain.Proxy) (string, error) {
	if !proxy.HasPassword() {
		return "", nil
	}
	password, err := s.sealer.Open(proxy.PasswordEncrypted())
	if err != nil {
		return "", domain.NewInternalError("the stored proxy password could not be read")
	}
	return password, nil
}
