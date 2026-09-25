// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/accounting_context.go
// @for       The context one accounting write runs under, so a client that
//
//	disconnected cannot erase the row its call left.
//
// @uses      context, time.
// @reason    A call that died mid-flight wrote no row at all while its gateway
//
//	key counter still counted it (draft 021 F6): the write ran under the
//	client's request context, which the server cancels the moment the
//	client goes away. Accounting describes work already done, so it
//	outlives the client, and it keeps its own bound (AGENTS.md §1.6).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-25
package service

import (
	"context"
	"time"
)

// accountingTimeout bounds one accounting write. It is deliberately short: the
// write is one row and one counter, and a database that cannot take it in this
// window is already reported by the panel's totals, which is where an operator
// can act on it.
const accountingTimeout = 5 * time.Second

// accountingContext detaches one accounting write from the client's cancellation
// while keeping the request's values, so both rows keep the request id the router
// put there. The returned context carries its own deadline, and the caller owns
// the cancel.
func accountingContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), accountingTimeout)
}
