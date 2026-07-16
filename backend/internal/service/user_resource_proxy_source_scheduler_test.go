package service

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestClaimDueProxySourceIsOwnerScopedAndAtomic(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()
	mock.ExpectExec(`UPDATE proxy_sources`).
		WithArgs(int64(44), int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	svc := NewUserResourceService(db, nil, nil, nil)
	claimed, err := svc.claimDueProxySource(context.Background(), dueUserProxySource{ID: 44, OwnerID: 9})
	if err != nil {
		t.Fatalf("claim due source: %v", err)
	}
	if !claimed {
		t.Fatal("expected due source to be claimed")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestProxySourceNodeIdentityIsStableAndNamesFitSchema(t *testing.T) {
	node := parsedProxyNode{Name: "US West", Kind: "xray", Protocol: "vless", Host: "node.example.com", Port: 443, Network: "grpc"}
	base := proxySourceNodeBaseKey(node)
	first := proxySourceNodeKey(base, 1)
	if first != proxySourceNodeKey(base, 1) {
		t.Fatal("source node key is not stable")
	}
	if first == proxySourceNodeKey(base, 2) {
		t.Fatal("duplicate source nodes received the same key")
	}
	name := proxySourceNodeName(44, strings.Repeat("source", 30), strings.Repeat("node", 30), 7)
	if len([]rune(name)) > 100 || !strings.HasSuffix(name, " [44:7]") {
		t.Fatalf("source proxy name is not schema-safe: %q", name)
	}
}

func TestStripProxySourceMetadataPreventsUserSpoofing(t *testing.T) {
	payload := map[string]any{"extra": map[string]any{
		"source_id": int64(9), "source_node_key": "spoofed", "raw": "vless://example",
	}}
	stripProxySourceMetadata(payload)
	extra := payload["extra"].(map[string]any)
	if _, ok := extra["source_id"]; ok {
		t.Fatal("user-provided source_id was retained")
	}
	if _, ok := extra["source_node_key"]; ok {
		t.Fatal("user-provided source_node_key was retained")
	}
	if extra["raw"] != "vless://example" {
		t.Fatal("non-reserved proxy metadata was removed")
	}
}

func TestUserResourceServiceCloseWithoutSchedulerIsSafe(t *testing.T) {
	svc := NewUserResourceService(nil, nil, nil, nil)
	if err := svc.Close(); err != nil {
		t.Fatalf("close service: %v", err)
	}
}
