package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"storeit/internal/identity/domain"
	"storeit/internal/identity/job"
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
	_, err = s.issueLink(ctx, a.ID, domain.PurposeInvite, domain.TokenEventInvitationResent, 0)
	return err
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
		_, err = s.issueLink(ctx, a.ID, domain.PurposeInvite, domain.TokenEventInvitationResent, 0)
		return err
	}
	_, err = s.issueLink(ctx, a.ID, domain.PurposeReset, domain.TokenEventResetSent, 0)
	return err
}

// ForgotPassword là form công khai "quên mật khẩu": chỉ kiểm dạng email rồi
// xếp job. Email có tài khoản hay không thì request cũng làm đúng một việc như
// nhau, nên cả câu trả lời (202) lẫn thời gian phản hồi đều không lộ gì.
func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	e, err := domain.NormalizeEmail(email)
	if err != nil {
		return nil
	}
	return s.jobs.Enqueue(ctx, job.ForgotPasswordArgs{Email: e})
}

// ProcessForgotPassword là việc của job identity.forgot_password. Email lạ,
// account bị khoá hay đang trong cooldown đều kết thúc job mà không phát gì.
func (s *Service) ProcessForgotPassword(ctx context.Context, args job.ForgotPasswordArgs) error {
	a, err := s.accounts.GetByEmail(ctx, args.Email)
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
	_, err = s.issueLink(ctx, a.ID, purpose, domain.TokenEventNone, forgotCooldown)
	if errors.Is(err, domain.ErrAccountInactive) {
		return nil // bị khoá sau lúc tra ở trên
	}
	return err
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

// issueLink phát token mới cho account, kèm job gửi thư và event ev. minAge > 0
// là cooldown: token cùng loại còn mới hơn thế thì không phát (trả false).
func (s *Service) issueLink(ctx context.Context, accountID uuid.UUID, p domain.TokenPurpose, ev domain.TokenEvent, minAge time.Duration) (bool, error) {
	t, err := s.newToken(p)
	if err != nil {
		return false, err
	}
	return s.pwTokens.Issue(ctx, accountID, t, ev, minAge)
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
