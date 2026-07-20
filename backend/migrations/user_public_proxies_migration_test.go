package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserPublicProxiesMigrationWidensOnlyPublicAccess(t *testing.T) {
	content, err := FS.ReadFile("185_user_public_proxies.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS is_public BOOLEAN NOT NULL DEFAULT FALSE")
	require.Contains(t, sql, "proxy_owner IS DISTINCT FROM NEW.owner_user_id AND NOT COALESCE(proxy_public, FALSE)")
	require.Contains(t, sql, "owner_user_id IS DISTINCT FROM NEW.owner_user_id")
	require.Contains(t, sql, "backup_owner IS DISTINCT FROM NEW.owner_user_id AND NOT COALESCE(backup_public, FALSE)")
	require.Contains(t, sql, "system accounts cannot use user-owned proxies")
	require.Contains(t, sql, "system proxies cannot fall back to user-owned proxies")
}
