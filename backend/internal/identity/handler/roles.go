package handler

import (
	"context"

	"storeit/internal/identity/handler/api"
)

func (h *Handler) ListRoles(ctx context.Context, _ api.ListRolesRequestObject) (api.ListRolesResponseObject, error) {
	roles, err := h.svc.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	counts, err := h.svc.RoleMemberCounts(ctx)
	if err != nil {
		return nil, err
	}
	out := make(api.ListRoles200JSONResponse, len(roles))
	for i, r := range roles {
		out[i] = toAPIRole(r)
		n := counts[r.ID]
		out[i].MemberCount = &n
	}
	return out, nil
}

func (h *Handler) CreateRole(ctx context.Context, req api.CreateRoleRequestObject) (api.CreateRoleResponseObject, error) {
	r, err := h.svc.CreateRole(ctx, req.Body.Name, deref(req.Body.Description), deref(req.Body.Permissions))
	if err != nil {
		return nil, err
	}
	return api.CreateRole201JSONResponse(toAPIRole(r)), nil
}

func (h *Handler) GetRole(ctx context.Context, req api.GetRoleRequestObject) (api.GetRoleResponseObject, error) {
	r, err := h.svc.GetRole(ctx, req.RoleID)
	if err != nil {
		return nil, err
	}
	counts, err := h.svc.RoleMemberCounts(ctx)
	if err != nil {
		return nil, err
	}
	out := toAPIRole(r)
	n := counts[r.ID]
	out.MemberCount = &n
	return api.GetRole200JSONResponse(out), nil
}

func (h *Handler) RestoreRole(ctx context.Context, req api.RestoreRoleRequestObject) (api.RestoreRoleResponseObject, error) {
	r, err := h.svc.RestoreRole(ctx, req.RoleID)
	if err != nil {
		return nil, err
	}
	counts, err := h.svc.RoleMemberCounts(ctx)
	if err != nil {
		return nil, err
	}
	out := toAPIRole(r)
	n := counts[r.ID]
	out.MemberCount = &n
	return api.RestoreRole200JSONResponse(out), nil
}

func (h *Handler) UpdateRole(ctx context.Context, req api.UpdateRoleRequestObject) (api.UpdateRoleResponseObject, error) {
	r, err := h.svc.UpdateRole(ctx, req.RoleID, req.Body.Name, req.Body.Description)
	if err != nil {
		return nil, err
	}
	return api.UpdateRole200JSONResponse(toAPIRole(r)), nil
}

func (h *Handler) UpdateRolePermissions(ctx context.Context, req api.UpdateRolePermissionsRequestObject) (api.UpdateRolePermissionsResponseObject, error) {
	r, err := h.svc.UpdateRolePermissions(ctx, req.RoleID, req.Body.Permissions)
	if err != nil {
		return nil, err
	}
	return api.UpdateRolePermissions200JSONResponse(toAPIRole(r)), nil
}

func (h *Handler) DeleteRole(ctx context.Context, req api.DeleteRoleRequestObject) (api.DeleteRoleResponseObject, error) {
	if err := h.svc.DeleteRole(ctx, req.RoleID); err != nil {
		return nil, err
	}
	return api.DeleteRole204Response{}, nil
}

func (h *Handler) ListPermissions(ctx context.Context, _ api.ListPermissionsRequestObject) (api.ListPermissionsResponseObject, error) {
	perms, err := h.svc.ListPermissions(ctx)
	if err != nil {
		return nil, err
	}
	out := make(api.ListPermissions200JSONResponse, len(perms))
	for i, p := range perms {
		out[i] = api.Permission{Code: p.Code, Description: p.Description}
	}
	return out, nil
}
