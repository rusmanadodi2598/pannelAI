// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/qoder_catalog_stampede_test.go
// @for       One catalogue read per host under concurrent lookups, and one per unknown model.
// @uses      sync, testing
// @reason    The model key arrives from the client, so a name the vendor does not list used to re-read the whole catalogue on every request, and concurrent requests each ran their own read of the same multi-megabyte document.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-04
package provider

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestQoderCatalog_CollapsesConcurrentReadsIntoOne(t *testing.T) {
	connector, lists, _ := newQoderStubVendor(t, qoderCatalogFixture)
	cred := qoderTestCredential()

	var wg sync.WaitGroup
	errs := make([]error, 8)
	for index := range errs {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			_, errs[slot] = connector.modelConfig(t.Context(), cred, "ultimate")
		}(index)
	}
	wg.Wait()

	for index, err := range errs {
		if err != nil {
			t.Fatalf("lookup %d error = %v, want every waiter served by the one read", index, err)
		}
	}
	if got := *lists; got != 1 {
		t.Fatalf("catalogue reads = %d, want 1 for eight concurrent lookups", got)
	}
}

func TestQoderCatalog_RefusesAnUnknownModelWithoutReReading(t *testing.T) {
	connector, lists, _ := newQoderStubVendor(t, qoderCatalogFixture)
	cred := qoderTestCredential()

	if _, err := connector.modelConfig(t.Context(), cred, "no-such-model"); err == nil {
		t.Fatal("modelConfig() accepted a model the vendor does not list")
	}
	readsAfterFirst := *lists
	for range 4 {
		if _, err := connector.modelConfig(t.Context(), cred, "no-such-model"); err == nil {
			t.Fatal("an unknown model was accepted on a later lookup")
		}
	}
	if got := *lists; got != readsAfterFirst {
		t.Fatalf("catalogue reads = %d, want the miss remembered for the window (%d after the first refusal)",
			got, readsAfterFirst)
	}
}

func TestQoderCatalog_AKnownModelIsServedFromCacheAfterTheFirstRead(t *testing.T) {
	connector, lists, _ := newQoderStubVendor(t, qoderCatalogFixture)
	cred := qoderTestCredential()

	if _, err := connector.modelConfig(t.Context(), cred, "auto"); err != nil {
		t.Fatalf("modelConfig() error = %v", err)
	}
	if got := *lists; got != 1 {
		t.Fatalf("catalogue reads = %d, want 1", got)
	}
	for range 3 {
		if _, err := connector.modelConfig(t.Context(), cred, "auto"); err != nil {
			t.Fatalf("cached lookup error = %v", err)
		}
	}
	if got := *lists; got != 1 {
		t.Fatalf("catalogue reads = %d, want the cached answer to be used", got)
	}
}

// TestQoderCatalog_ACancelledCallerStopsWaiting holds one fetch open the way a
// slow vendor would, and asks whether a caller whose client went away keeps
// paying for the document. It must not: the wait is the part a cancellation can
// end, and the read it was waiting for is shared, so it belongs to the leader.
func TestQoderCatalog_ACancelledCallerStopsWaiting(t *testing.T) {
	connector, lists, _ := newQoderStubVendor(t, qoderCatalogFixture)
	cred := qoderTestCredential()

	base, err := connector.inferenceBase(cred)
	if err != nil {
		t.Fatalf("inferenceBase() error = %v", err)
	}
	fetch, leader := connector.catalog.beginFetch(base)
	if !leader {
		t.Fatal("this test expected to hold the fetch slot itself")
	}

	ctx, cancel := context.WithCancel(context.Background())
	errs := make(chan error, 1)
	go func() {
		_, err := connector.modelConfig(ctx, cred, "auto")
		errs <- err
	}()

	cancel()
	select {
	case err := <-errs:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("modelConfig() error = %v, want the caller's cancellation", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a cancelled caller is still waiting on the catalogue read")
	}
	if got := *lists; got != 0 {
		t.Fatalf("catalogue reads = %d, want none spent on a caller that left", got)
	}
	close(fetch.ready)
}
