// Package provider defines the plugin seam between the gateway core and one
// upstream provider.
//
// @file      internal/provider/qoder_catalog_isolation_test.go
// @for       One account's model catalogue, kept separate from the next account's.
// @uses      context, net/http, net/http/httptest, sync, testing, time
// @reason    The vendor answers the model list for the account that asked, so a cache keyed only by the gateway serves one account's entitlements to another, remembers one account's refusals against the rest, and delivers one account's failed read to a caller it never signed for. A single-account stub cannot show any of that, so this one answers per account.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability stable
// @since     2026-10-08
package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// qoderAccountCatalog is a vendor that answers the model list per account. An empty
// answer is a refusal rather than an empty catalogue, because a 401 has to stay
// distinguishable from a document that simply does not list the model.
type qoderAccountCatalog struct {
	answers map[string]string
	status  map[string]int
	reads   map[string]int
	// held, when non-nil, is closed once the blocked account's request has arrived
	// and is only released when the test closes it, which is what makes a joined
	// fetch observable instead of racy.
	held    chan struct{}
	release chan struct{}
}

func newQoderAccountCatalog() *qoderAccountCatalog {
	return &qoderAccountCatalog{
		answers: map[string]string{},
		status:  map[string]int{},
		reads:   map[string]int{},
	}
}

func (c *qoderAccountCatalog) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == qoderJobTokenExchangePath {
		// The connector exchanges the stored key for a job token before it signs, so
		// a vendor that never answers it cannot reach the catalogue at all.
		_, _ = w.Write([]byte(`{"token":"jt-issued","expires_at":"2099-01-01T00:00:00Z"}`))
		return
	}
	if r.URL.Path != "/algo"+qoderModelListPath {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	user := r.Header.Get("Cosy-User")
	c.reads[user]++
	if user == "user-a" && c.release != nil {
		close(c.held)
		<-c.release
	}
	if status := c.status[user]; status != 0 && status != http.StatusOK {
		w.WriteHeader(status)
		return
	}
	_, _ = w.Write([]byte(c.answers[user]))
}

// newQoderAccountVendor builds one connector over a vendor that answers each account
// differently. Both accounts reach the same host, which is the condition the
// catalogue cache is being tested under.
func newQoderAccountVendor(t *testing.T, catalog *qoderAccountCatalog) *Qoder {
	t.Helper()

	server := httptest.NewServer(catalog)
	t.Cleanup(server.Close)

	connector, err := NewQoder(qoderEntry("qoder", server.URL,
		server.URL+"/algo/api/v2/service/pro/sse/agent_chat_generation?FetchKeys=llm_model_result&AgentId=agent_common"),
		server.Client())
	if err != nil {
		t.Fatalf("NewQoder() error = %v", err)
	}
	return connector
}

// qoderAccountCredential is the credential of one named account. The user id is what
// the signed request carries, so it is how the vendor tells the accounts apart.
func qoderAccountCredential(endpointID, user string) Credential {
	return Credential{endpointID: endpointID, apiKey: "pt-secret-" + endpointID,
		family: FamilyStaticKey, projectID: user, account: user + "@example.com",
		metadata: map[string]string{MetadataMachineID: "machine-" + endpointID}}
}

// catalogOf builds a catalogue whose chat group lists exactly the given keys, so a
// test can name what one account is entitled to.
func catalogOf(keys ...string) string {
	body := `{"chat":[`
	for index, key := range keys {
		if index > 0 {
			body += ","
		}
		body += `{"key":"` + key + `","format":"openai","source":"system"}`
	}
	return body + `]}`
}

// TestQoderCatalog_TwoAccountsDoNotShareOneCachedAnswer is the leak in its plainest
// form: the vendor answers for the account that asked, so a document cached under one
// account must not be handed to the next. B is not entitled to `ultimate` at all, so
// the only way B can be served one is by reading A's cache.
func TestQoderCatalog_TwoAccountsDoNotShareOneCachedAnswer(t *testing.T) {
	catalog := newQoderAccountCatalog()
	catalog.answers["user-a"] = catalogOf("ultimate")
	catalog.answers["user-b"] = catalogOf("standard")
	connector := newQoderAccountVendor(t, catalog)
	ctx := context.Background()

	if _, err := connector.modelConfig(ctx, qoderAccountCredential("ep_a", "user-a"), "ultimate"); err != nil {
		t.Fatalf("account A modelConfig() error = %v", err)
	}
	if _, err := connector.modelConfig(ctx, qoderAccountCredential("ep_b", "user-b"), "ultimate"); err == nil {
		t.Fatal("account B was served a model only account A is listed for: the cached document travelled")
	}
	if got := catalog.reads["user-b"]; got != 1 {
		t.Fatalf("reads for B = %d, want 1: B was answered from A's cache without asking", got)
	}
}

// TestQoderCatalog_OneAccountsUnknownModelDoesNotRefuseAnother pins the negative
// cache to the account that was refused: a model listed for B stays routable for B
// after A asked for it and was told no.
func TestQoderCatalog_OneAccountsUnknownModelDoesNotRefuseAnother(t *testing.T) {
	catalog := newQoderAccountCatalog()
	catalog.answers["user-a"] = catalogOf("ultimate")
	catalog.answers["user-b"] = catalogOf("standard")
	connector := newQoderAccountVendor(t, catalog)
	ctx := context.Background()

	if _, err := connector.modelConfig(ctx, qoderAccountCredential("ep_a", "user-a"), "standard"); err == nil {
		t.Fatal("account A accepted a model its own catalogue does not list")
	}
	if _, err := connector.modelConfig(ctx, qoderAccountCredential("ep_b", "user-b"), "standard"); err != nil {
		t.Fatalf("account B modelConfig() error = %v, want the model it is listed for: A's refusal was remembered against B", err)
	}
}

// TestQoderCatalog_OneAccountsFailedReadDoesNotAnswerAnother holds one account's
// fetch open the way a slow vendor would, then asks for a second account while the
// first read is still running. The second must spend its own read rather than join a
// fetch it was not signed for, because what it would join is a 401 aimed at somebody
// else.
func TestQoderCatalog_OneAccountsFailedReadDoesNotAnswerAnother(t *testing.T) {
	catalog := newQoderAccountCatalog()
	catalog.answers["user-b"] = catalogOf("standard")
	catalog.status["user-a"] = http.StatusUnauthorized
	catalog.held = make(chan struct{})
	catalog.release = make(chan struct{})
	connector := newQoderAccountVendor(t, catalog)

	errs := make(chan error, 1)
	go func() {
		_, err := connector.modelConfig(context.Background(), qoderAccountCredential("ep_a", "user-a"), "standard")
		errs <- err
	}()
	// The request arriving is the proof that A holds the fetch slot for this host.
	select {
	case <-catalog.held:
	case <-time.After(2 * time.Second):
		t.Fatal("account A never reached the vendor")
	}
	// Released before the test waits on it: a deferred close would only happen after
	// that wait, and the wait would then be the fetch's own timeout rather than the
	// answer.
	release := sync.OnceFunc(func() { close(catalog.release) })
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := connector.modelConfig(ctx, qoderAccountCredential("ep_b", "user-b"), "standard"); err != nil {
		t.Fatalf("account B modelConfig() error = %v, want its own read: B joined A's in-flight fetch", err)
	}
	if got := catalog.reads["user-b"]; got != 1 {
		t.Fatalf("reads for B = %d, want 1 of B's own", got)
	}

	release()
	if err := <-errs; err == nil {
		t.Fatal("account A's 401 was answered as a success")
	}
}
