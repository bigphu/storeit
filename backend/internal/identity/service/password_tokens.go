package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"storeit/internal/identity/domain"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/errs"
)

// Quên mật khẩu: một account nhận tối đa một link mỗi khoảng này, để không ai
// dùng form công khai làm ngập hộp thư người khác
const forgotCooldown = 60 * time.Second

// ResendInvitation gửi lời mời mới (link cũ chết) cho account chưa đặt mật khẩu
func (s *Service) ResendInvitation(ctx context.Context, id uuid.UUID) error {
	if _, err := auth.Require(ctx, domain.PermAccountManage); err != nil {
		return err
	}
	a, err := s.accounts.Get(ctx, id)
	if err != nil {
		return err
	}
	switch {
	case !a.Active:
		return domain.ErrAccountInactive
	case a.HasPassword():
		return domain.ErrNotInvited
	}
	return s.issueLink(ctx, a.ID, domain.PurposeInvite, domain.TokenEventInvitationResent)
}

// SendPasswordReset: quản trị gửi link đặt lại mật khẩu. Không thu hồi phiên
// nào: người dùng đặt mật khẩu mới thì mới thu hồi. Account chưa nhận lời mời
// thì nhận lời mời mới.
func (s *Service) SendPasswordReset(ctx context.Context, id uuid.UUID) error {
	if _, err := auth.Require(ctx, domain.PermAccountManage); err != nil {
		return err
	}
	a, err := s.accounts.Get(ctx, id)
	if err != nil {
		return err
	}
	if !a.Active {
		return domain.ErrAccountInactive
	}
	if !a.HasPassword() {
		return s.issueLink(ctx, a.ID, domain.PurposeInvite, domain.TokenEventInvitationResent)
	}
	return s.issueLink(ctx, a.ID, domain.PurposeReset, domain.TokenEventResetSent)
}

// ForgotPassword là form công khai "quên mật khẩu". Luôn trả nil khi không có
// lỗi hệ thống: email lạ, account bị khoá hay đang trong cooldown đều im lặng,
// để form không trả lời được câu "email này có tài khoản không".
func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	e, err := domain.NormalizeEmail(email)
	if err != nil {
		return nil
	}
	a, err := s.accounts.GetByEmail(ctx, e)
	if errors.Is(err, domain.ErrAccountNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !a.Active {
		return nil
	}
	// Chưa nhận lời mời (có thể lời mời đã hết hạn): gửi lời mời mới
	purpose := domain.PurposeReset
	if !a.HasPassword() {
		purpose = domain.PurposeInvite
	}
	last, err := s.pwTokens.Latest(ctx, a.ID, purpose)
	if err != nil {
		return err
	}
	if last != nil && s.now().Sub(last.CreatedAt) < forgotCooldown {
		return nil
	}
	return s.issueLink(ctx, a.ID, purpose, domain.TokenEventNone)
}

// SetPassword đặt mật khẩu bằng link trong thư (lời mời hoặc đặt lại). Không
// đăng nhập luôn: người dùng đăng nhập bằng mật khẩu vừa đặt.
func (s *Service) SetPassword(ctx context.Context, token, password string) error {
	if err := domain.ValidatePassword(password); err != nil {
		return err
	}
	if token == "" {
		return domain.ErrInvalidPasswordToken
	}
	h, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	_, err = s.pwTokens.Use(ctx, hashSecret(token), s.now(), h)
	return err
}

// issueLink phát token mới cho account, kèm job gửi thư và event ev
func (s *Service) issueLink(ctx context.Context, accountID uuid.UUID, p domain.TokenPurpose, ev domain.TokenEvent) error {
	t, err := s.newToken(p)
	if err != nil {
		return err
	}
	return s.pwTokens.Issue(ctx, accountID, t, ev)
}

// newToken tạo token thô (vào thư) và hash của nó (vào DB) với hạn theo loại
func (s *Service) newToken(p domain.TokenPurpose) (domain.IssuedToken, error) {
	raw, h, err := newSecret()
	if err != nil {
		return domain.IssuedToken{}, errs.ErrInternal.With(errs.WithCause(err))
	}
	id, err := uuid.NewV7()
	if err != nil {
		return domain.IssuedToken{}, errs.ErrInternal.With(errs.WithCause(err))
	}
	return domain.IssuedToken{ID: id, Purpose: p, Raw: raw, Hash: h, ExpiresAt: s.now().Add(s.ttl(p))}, nil
}

func (s *Service) ttl(p domain.TokenPurpose) time.Duration {
	if p == domain.PurposeInvite {
		return s.settings.InviteTTL
	}
	return s.settings.ResetTTL
}
