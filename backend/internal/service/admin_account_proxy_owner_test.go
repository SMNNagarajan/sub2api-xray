package service

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type accountProxyOwnerRepoStub struct {
	ProxyRepository
	proxy *Proxy
}

func (s *accountProxyOwnerRepoStub) GetByID(context.Context, int64) (*Proxy, error) {
	return s.proxy, nil
}

func proxyRepoReturning(proxy *Proxy) ProxyRepository {
	return &accountProxyOwnerRepoStub{proxy: proxy}
}

func TestAccountProxyOwnerCompatible(t *testing.T) {
	ownerOne := int64(1)
	ownerTwo := int64(2)

	tests := []struct {
		name         string
		accountOwner *int64
		proxy        *Proxy
		want         bool
	}{
		{name: "system account with system proxy", proxy: &Proxy{}, want: true},
		{name: "system account with user proxy", proxy: &Proxy{OwnerUserID: &ownerOne}, want: false},
		{name: "user account with owned proxy", accountOwner: &ownerOne, proxy: &Proxy{OwnerUserID: &ownerOne}, want: true},
		{name: "user account with another private proxy", accountOwner: &ownerOne, proxy: &Proxy{OwnerUserID: &ownerTwo}, want: false},
		{name: "user account with private system proxy", accountOwner: &ownerOne, proxy: &Proxy{}, want: false},
		{name: "user account with public system proxy", accountOwner: &ownerOne, proxy: &Proxy{IsPublic: true}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, accountProxyOwnerCompatible(tt.accountOwner, tt.proxy))
		})
	}
}

func TestCreateAccountRejectsUserOwnedProxyForSystemAccount(t *testing.T) {
	proxyOwner := int64(9)
	proxyID := int64(41)
	repo := &upstreamBillingProbeAccountRepo{}
	svc := &adminServiceImpl{
		accountRepo: &upstreamBillingProbeAdminRepo{repo},
		proxyRepo:   proxyRepoReturning(&Proxy{ID: proxyID, OwnerUserID: &proxyOwner}),
	}

	_, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "system-account",
		Platform:             PlatformOpenAI,
		Type:                 AccountTypeAPIKey,
		Credentials:          map[string]any{"api_key": "sk-test"},
		ProxyID:              &proxyID,
		SkipDefaultGroupBind: true,
	})

	require.Error(t, err)
	require.Equal(t, "ACCOUNT_PROXY_OWNER_MISMATCH", string(infraerrors.Reason(err)))
	require.Empty(t, repo.accounts)
}

func TestUpdateAccountRejectsProxyFromAnotherOwnerBeforeWrite(t *testing.T) {
	accountOwner := int64(7)
	proxyOwner := int64(8)
	accountID := int64(22)
	proxyID := int64(42)
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{
		accountID: {ID: accountID, OwnerUserID: &accountOwner, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive},
	}}
	svc := &adminServiceImpl{
		accountRepo: &upstreamBillingProbeAdminRepo{repo},
		proxyRepo:   proxyRepoReturning(&Proxy{ID: proxyID, OwnerUserID: &proxyOwner}),
	}

	_, err := svc.UpdateAccount(context.Background(), accountID, &UpdateAccountInput{ProxyID: &proxyID})

	require.Error(t, err)
	require.Equal(t, "ACCOUNT_PROXY_OWNER_MISMATCH", string(infraerrors.Reason(err)))
	require.Nil(t, repo.accounts[accountID].ProxyID)
}

func TestBulkUpdateAccountsAcceptsPublicProxyForUserAccounts(t *testing.T) {
	accountOwner := int64(7)
	accountID := int64(23)
	proxyID := int64(43)
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{
		accountID: {ID: accountID, OwnerUserID: &accountOwner, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive},
	}}
	svc := &adminServiceImpl{
		accountRepo: &upstreamBillingProbeAdminRepo{repo},
		proxyRepo:   proxyRepoReturning(&Proxy{ID: proxyID, IsPublic: true}),
	}

	result, err := svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
		AccountIDs: []int64{accountID},
		ProxyID:    &proxyID,
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Success)
	require.Len(t, repo.bulkUpdates, 1)
	require.Equal(t, &proxyID, repo.bulkUpdates[0].ProxyID)
}
