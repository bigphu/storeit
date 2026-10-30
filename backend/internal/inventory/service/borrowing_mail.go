package service

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"log/slog"
	texttemplate "text/template"
	"time"

	"github.com/google/uuid"

	idcontract "storeit/internal/identity/contract"
	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/job"
	"storeit/internal/platform/logger"
	"storeit/internal/platform/mail"
)

// Thư của việc mượn (US-10): job do repository xếp trong cùng transaction với thay đổi; ở
// đây đọc lại phiếu/khoản lúc chạy, đã đổi thì bỏ qua (vd phiếu duyệt rồi bị huỷ trước khi
// thư đi), rồi dựng thư và gửi.

//go:embed templates
var borrowTemplateFS embed.FS

var (
	borrowText = texttemplate.Must(texttemplate.ParseFS(borrowTemplateFS, "templates/*.txt"))
	borrowHTML = htmltemplate.Must(htmltemplate.ParseFS(borrowTemplateFS, "templates/*.html"))
)

// Quét quá hạn chỉ từ giờ này (giờ ứng dụng), để thư nhắc tới vào buổi sáng
const overdueScanHour = 8

type borrowEmail struct {
	Name      string
	Number    int64
	Tag       string
	AssetName string
	Reason    string
	Due       string // "25 Oct 2026"
	Link      string
	LogoURL   string
}

// SendBorrowingEmail là việc của job inventory.send_borrowing_email
func (s *Service) SendBorrowingEmail(ctx context.Context, args job.BorrowingEmailArgs) error {
	if s.mail == nil {
		return errors.New("inventory: no mail sender configured")
	}
	log := logger.FromContext(ctx)
	var (
		to      uuid.UUID
		data    = borrowEmail{Link: s.appURL + "/borrowing/mine", LogoURL: s.appURL + "/email-logo.png"}
		key     string
		assetID *uuid.UUID
		due     time.Time
	)
	switch args.EmailKind {
	case job.EmailTicketApproved, job.EmailTicketRejected, job.EmailTicketCancelled:
		if args.TicketID == nil {
			return fmt.Errorf("inventory: %s email without ticket", args.EmailKind)
		}
		tk, err := s.borrowing.GetTicket(ctx, *args.TicketID)
		if errors.Is(err, domain.ErrTicketNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		want := map[string]domain.TicketState{
			job.EmailTicketApproved: domain.TicketApproved, job.EmailTicketRejected: domain.TicketRejected,
			job.EmailTicketCancelled: domain.TicketCancelled,
		}[args.EmailKind]
		if tk.State != want {
			log.InfoContext(ctx, "borrowing email skipped, ticket moved on", slog.String("ticket_id", tk.ID.String()), slog.String("kind", args.EmailKind))
			return nil
		}
		to, assetID, key = tk.RequesterID, tk.AssetID, tk.ID.String()
		data.Number, data.Reason = tk.Number, tk.Reason
	case job.EmailLoanOverdue:
		if args.LoanID == nil {
			return errors.New("inventory: overdue email without loan")
		}
		l, err := s.borrowing.GetLoan(ctx, *args.LoanID)
		if errors.Is(err, domain.ErrLoanNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if l.ReturnedOn != nil || l.ReturnBy == nil {
			log.InfoContext(ctx, "borrowing email skipped, loan returned", slog.String("loan_id", l.ID.String()))
			return nil
		}
		to, key, due = l.BorrowerID, l.ID.String(), *l.ReturnBy
		assetID = &l.AssetID
		data.Due = due.Format("2 Jan 2006")
	default:
		return fmt.Errorf("inventory: unknown borrowing email %q", args.EmailKind)
	}

	acc, err := s.accounts.GetAccount(ctx, to)
	if errors.Is(err, idcontract.ErrAccountNotFound) || (err == nil && !acc.Active) {
		log.InfoContext(ctx, "borrowing email skipped, account gone or disabled", slog.String("account_id", to.String()))
		return nil
	}
	if err != nil {
		return err
	}
	data.Name = acc.Name
	if assetID != nil {
		a, err := s.assets.Get(ctx, *assetID)
		if err != nil && !errors.Is(err, domain.ErrAssetNotFound) {
			return err
		}
		data.Tag, data.AssetName = a.Tag, a.Name
	}
	subject := map[string]string{
		job.EmailTicketApproved:  fmt.Sprintf("Ticket #%d approved: %s is ready for pickup", data.Number, data.Tag),
		job.EmailTicketRejected:  fmt.Sprintf("Ticket #%d was not approved", data.Number),
		job.EmailTicketCancelled: fmt.Sprintf("Ticket #%d was cancelled", data.Number),
		job.EmailLoanOverdue:     fmt.Sprintf("%s was due back on %s", data.Tag, due.Format("2 Jan")),
	}[args.EmailKind]

	var text, html bytes.Buffer
	if err := borrowText.ExecuteTemplate(&text, args.EmailKind+".txt", data); err != nil {
		return fmt.Errorf("inventory: render %s email: %w", args.EmailKind, err)
	}
	if err := borrowHTML.ExecuteTemplate(&html, args.EmailKind+".html", data); err != nil {
		return fmt.Errorf("inventory: render %s email: %w", args.EmailKind, err)
	}
	err = s.mail.Send(ctx, mail.Message{
		To: mail.Address{Name: acc.Name, Email: acc.Email}, Subject: subject, Text: text.String(), HTML: html.String(),
		// Mỗi phiếu/khoản một key cho mỗi loại thư: job chạy lại sau khi đã gửi thì Resend bỏ qua
		IdempotencyKey: "inventory/borrowing/" + args.EmailKind + "/" + key,
	})
	if err != nil {
		return fmt.Errorf("inventory: send %s email: %w", args.EmailKind, err)
	}
	log.InfoContext(ctx, "sent borrowing email", slog.String("kind", args.EmailKind), slog.String("account_id", to.String()))
	return nil
}

// ScanOverdueLoans là việc của job định kỳ inventory.scan_overdue_loans: từ 08:00 giờ ứng dụng,
// xếp thư nhắc cho mọi khoản quá hạn chưa báo (mỗi khoản một lần)
func (s *Service) ScanOverdueLoans(ctx context.Context) error {
	if s.now().In(s.loc).Hour() < overdueScanHour {
		return nil
	}
	const batch = 100
	for {
		n, err := s.borrowing.QueueOverdueEmails(ctx, s.today(), batch)
		if err != nil {
			return err
		}
		if n < batch {
			return nil
		}
	}
}
