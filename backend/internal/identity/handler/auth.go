package handler

import (
	"context"
	"errors"

	"storeit/internal/identity/domain"
	"storeit/internal/identity/handler/api"
	"storeit/internal/identity/service"
	"storeit/internal/platform/errs"
	"storeit/internal/platform/middleware"
)

func (h *Handler) Login(ctx context.Context, req api.LoginRequestObject) (api.LoginResponseObject, error) {
	sess, err := h.svc.Login(ctx, req.Body.Email, req.Body.Password, service.Device{
		UserAgent: deref(req.Params.UserAgent),
		IP:        middleware.ClientIPFrom(ctx),
	})
	if err != nil {
		return nil, err
	}
	return api.Login200JSONResponse{
		Body:    toSession(sess),
		Headers: api.Login200ResponseHeaders{SetCookie: h.cookie.set(sess.RefreshToken, sess.RefreshExpiresAt)},
	}, nil
}

// Refresh: mọi thất bại trả 401 kèm xoá cookie, để client không gửi lại một
// cookie đã chết. Thiếu cookie và cookie sai trả về đúng một thứ.
func (h *Handler) Refresh(ctx context.Context, req api.RefreshRequestObject) (api.RefreshResponseObject, error) {
	sess, err := h.svc.Refresh(ctx, deref(req.Params.StoreitRefresh))
	if errors.Is(err, domain.ErrInvalidRefreshToken) {
		p := errs.NewFrom(err)
		return api.Refresh401ApplicationProblemPlusJSONResponse{
			Body:    problem(p),
			Headers: api.Refresh401ResponseHeaders{SetCookie: h.cookie.clear()},
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.Refresh200JSONResponse{
		Body:    toSession(sess),
		Headers: api.Refresh200ResponseHeaders{SetCookie: h.cookie.set(sess.RefreshToken, sess.RefreshExpiresAt)},
	}, nil
}

func (h *Handler) Logout(ctx context.Context, req api.LogoutRequestObject) (api.LogoutResponseObject, error) {
	if err := h.svc.Logout(ctx, deref(req.Params.StoreitRefresh)); err != nil {
		return nil, err
	}
	return api.Logout204Response{Headers: api.Logout204ResponseHeaders{SetCookie: h.cookie.clear()}}, nil
}

func (h *Handler) ChangePassword(ctx context.Context, req api.ChangePasswordRequestObject) (api.ChangePasswordResponseObject, error) {
	err := h.svc.ChangePassword(ctx, req.Body.CurrentPassword, req.Body.NewPassword, deref(req.Params.StoreitRefresh))
	if err != nil {
		return nil, err
	}
	return api.ChangePassword204Response{}, nil
}

func (h *Handler) GetMe(ctx context.Context, _ api.GetMeRequestObject) (api.GetMeResponseObject, error) {
	me, err := h.svc.Me(ctx)
	if err != nil {
		return nil, err
	}
	perms := me.Permissions
	if perms == nil {
		perms = []string{}
	}
	return api.GetMe200JSONResponse{
		Account: toAPIAccount(me.Account), Roles: toSummaries(me.Roles), Permissions: perms,
	}, nil
}

// problem đổi *errs.Error sang kiểu Problem sinh từ common.yaml, cho các
// response lỗi khai báo riêng trong spec (vd 401 của refresh kèm Set-Cookie)
func problem(e *errs.Error) api.Problem {
	p := api.Problem{Type: e.Type(), Title: e.Title(), Status: e.Status()}
	if d := e.Detail(); d != "" {
		p.Detail = &d
	}
	return p
}
