// Package provider holds the per-provider plugin seam: the connector that decides
// how one provider is authenticated and addressed.
//
// @file      internal/provider/gemini_cloudcode_requestid_test.go
// @for       Which credential identity becomes the Cloud Code Assist session id.
// @uses      strings, testing.
// @reason    The requestId is what the vendor attributes a call to, so a session that changes on every retry splits one conversation across buckets, and a session keyed on the account instead of the project blends two projects of one owner into one bucket. This is the half of the wire that only shows up when the same turn is sent twice, so it is pinned apart from the envelope shape.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     domain
// @stability stable
// @since     2026-10-18
package provider

import (
	"strings"
	"testing"
)

// TestCloudCodeRequestID_ScopesConversationToTheProject pins which identity acts as
// the session: the project, so a retry of one turn keeps its conversation while two
// projects of one account do not blend into one attribution bucket.
func TestCloudCodeRequestID_ScopesConversationToTheProject(t *testing.T) {
	entry := cloudcodeEntry("antigravity")
	connector := NewCloudCode(entry, CloudCodeAntigravity)

	conversationOf := func(project string) string {
		request := cloudcodeRequest(t, entry, 1, false, project)
		if err := connector.TransformRequest(&request); err != nil {
			t.Fatalf("TransformRequest() error = %v", err)
		}
		members := envelopeMembers(t, request.Body)
		parts := strings.Split(strings.Trim(string(members["requestId"]), `"`), "/")
		if len(parts) != 5 {
			t.Fatalf("requestId = %s, want five parts", members["requestId"])
		}
		return parts[1]
	}

	first := conversationOf("proj-9")
	retry := conversationOf("proj-9")
	if first != retry {
		t.Fatal("one project produced two conversation ids, so a retry would read as a new conversation")
	}
	if conversationOf("proj-9") == conversationOf("proj-10") {
		t.Fatal("two projects share one conversation id, so the session is not scoped to the project")
	}
}
