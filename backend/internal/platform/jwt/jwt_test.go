package jwt

import (
	"encoding/base64"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func newTestProvider(t *testing.T) *Provider {
	t.Helper()
	secret := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", minSecretLen)))
	p, err := New(Config{
		Keys:           "v1:" + secret,
		ActiveKID:      "v1",
		Issuer:         "storeit",
		Audience:       "storeit-api",
		AccessTokenTTL: 15 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestIssueVerify_CarriesPermissions(t *testing.T) {
	p := newTestProvider(t)
	accountID := uuid.New()
	perms := []string{"inventory.asset.read", "identity.role.manage"}

	tok, err := p.Issue(accountID, perms)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := p.Verify(tok.Value)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(claims.Permissions, perms) {
		t.Errorf("permissions = %v, want %v", claims.Permissions, perms)
	}
	if id, err := claims.UserID(); err != nil || id != accountID {
		t.Errorf("UserID() = %v, %v, want %v", id, err, accountID)
	}
}

// Account không có role nào vẫn được đăng nhập, chỉ là không có quyền gì
func TestIssueVerify_NoPermissions(t *testing.T) {
	p := newTestProvider(t)

	tok, err := p.Issue(uuid.New(), nil)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := p.Verify(tok.Value)
	if err != nil {
		t.Fatal(err)
	}

	if len(claims.Permissions) != 0 {
		t.Errorf("permissions = %v, want none", claims.Permissions)
	}
}
