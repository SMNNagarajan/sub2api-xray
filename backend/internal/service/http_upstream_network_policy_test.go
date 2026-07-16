package service

import (
	"net/http"
	"testing"
)

func TestProtectUserOwnedUpstreamRequest(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	ownerID := int64(7)
	protected := ProtectUserOwnedUpstreamRequest(req, &Account{OwnerUserID: &ownerID}, "")
	if policy := HTTPUpstreamNetworkPolicyFromContext(protected.Context()); !policy.PublicOnly {
		t.Fatal("user-owned account request was not protected")
	}

	system := ProtectUserOwnedUpstreamRequest(req, &Account{}, "")
	if policy := HTTPUpstreamNetworkPolicyFromContext(system.Context()); policy.PublicOnly {
		t.Fatal("system account request unexpectedly received user network policy")
	}
}
