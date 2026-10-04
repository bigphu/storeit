package handler

import (
	"github.com/google/uuid"

	"storeit/internal/identity/domain"
	"storeit/internal/identity/handler/api"
	"storeit/internal/identity/service"
)

func toAPIAccount(a domain.Account) api.Account {
	return api.Account{
		Id: a.ID, Email: a.Email, Name: a.Name, MemberId: a.MemberID,
		Active: a.Active, Status: api.AccountStatus(a.Status()), Version: a.Version,
		LastSignInAt: a.LastSignInAt, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

func toAPIDetail(v service.AccountView) api.AccountDetail {
	a := v.Account
	return api.AccountDetail{
		Id: a.ID, Email: a.Email, Name: a.Name, MemberId: a.MemberID,
		Active: a.Active, Status: api.AccountStatus(a.Status()), Version: a.Version,
		LastSignInAt: a.LastSignInAt, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
		Roles: toSummaries(v.Roles), ActiveSessions: v.Sessions, InviteExpiresAt: v.InviteExpiresAt,
	}
}

func toSummaries(roles []domain.Role) []api.RoleSummary {
	out := make([]api.RoleSummary, len(roles))
	for i, r := range roles {
		out[i] = api.RoleSummary{Id: r.ID, Name: r.Name}
	}
	return out
}

func toAPIRole(r domain.Role) api.Role {
	perms := r.Permissions
	if perms == nil {
		perms = []string{}
	}
	return api.Role{Id: r.ID, Name: r.Name, Description: r.Description, IsSystem: r.IsSystem, Permissions: perms}
}

func toSession(s service.Session) api.SessionResponse {
	return api.SessionResponse{
		AccessToken: s.AccessToken,
		TokenType:   api.Bearer,
		ExpiresAt:   s.AccessExpiresAt,
		Account:     toAPIAccount(s.Account),
	}
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

func ids(p *[]uuid.UUID) []uuid.UUID {
	if p == nil {
		return nil
	}
	return *p
}
