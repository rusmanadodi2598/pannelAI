// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/count_tokens.go
// @for       The §7.15 count_tokens use case: the token estimate and the rule
//
//	it is derived by.
//
// @uses      internal/schema.
// @reason    SPEC-API-001 §7.15 serves POST /api/v1/messages/count_tokens in P3.
//
//	The estimate is the route's whole answer and the handler must not
//	compute it (AGENTS.md §1.5), so the rule lives here where it can be
//	read and tested on its own.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"

// charsPerToken is the estimator's divisor. It is the reference's constant
// (~4 characters per token) and is deliberately the only tunable in the rule:
// the estimate is a rough one, and a second factor would imply a precision the
// route does not have.
const charsPerToken = 4

// TokenCountService answers §7.15's count_tokens route. It is stateless because
// the estimate reads nothing but the request (no upstream is dialed and no
// stored state is consulted), which is what makes the route cheap enough for a
// client to call before every request.
type TokenCountService struct{}

// NewTokenCountService returns the stateless estimator.
func NewTokenCountService() *TokenCountService { return &TokenCountService{} }

// Count returns the estimated input tokens for an Anthropic-wire request.
//
// The estimate is intentionally not a tokenizer: the reference route answers
// this same arithmetic, and the §10 exit criterion for P3 is a parity
// spot-check against it, so the gateway reports the number the client would
// have received from the reference rather than a second opinion.
func (s *TokenCountService) Count(req schema.CountTokensRequest) schema.CountTokensResponse {
	return schema.CountTokensResponse{InputTokens: estimateInputTokens(req.InputChars())}
}

// estimateInputTokens maps a character count to tokens: one token per four
// characters, rounded up. A count of zero maps to zero; rounding zero up to
// one would make every empty request claim a token it does not spend.
func estimateInputTokens(chars int) int {
	if chars <= 0 {
		return 0
	}
	return (chars + charsPerToken - 1) / charsPerToken
}
