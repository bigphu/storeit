package logger

import (
	"context"
	"log/slog"
	"slices"
	"sync"
)

type ctxKey struct{}

// With gắn thêm attr vào ctx. Log nào ghi bằng ctx đó (InfoContext,
// ErrorContext...) cũng có các attr này, nên không phải truyền logger đi đâu:
//
//	ctx = logger.With(ctx, slog.String("request_id", chimw.GetReqID(ctx)))
func With(ctx context.Context, attrs ...slog.Attr) context.Context {
	if len(attrs) == 0 {
		return ctx
	}
	// Clip để append luôn tạo mảng mới, không ghi đè lên ctx cha hay ctx anh em
	all := append(slices.Clip(attrsFrom(ctx)), attrs...)
	return context.WithValue(ctx, ctxKey{}, all)
}

func attrsFrom(ctx context.Context) []slog.Attr {
	attrs, _ := ctx.Value(ctxKey{}).([]slog.Attr)
	return attrs
}

type scopeKey struct{}

// scope giữ attr thêm vào sau khi ctx đã được tạo, dùng chung cho cả cây ctx
// bên dưới WithScope
type scope struct {
	mu    sync.Mutex
	attrs []slog.Attr
}

// WithScope mở một scope, thường cho một request (RequestLogger gọi). Khác với
// With, attr thêm bằng AddToScope từ ctx con cũng hiện trong log ghi bằng ctx
// cha. Nhờ vậy actor do auth đọc từ token, ở sâu bên trong, vẫn có trong dòng
// access log và log panic ở bên ngoài.
//
// Trên nhánh đã có scope thì dùng lại scope đó.
func WithScope(ctx context.Context) context.Context {
	if _, ok := ctx.Value(scopeKey{}).(*scope); ok {
		return ctx
	}
	return context.WithValue(ctx, scopeKey{}, &scope{})
}

// AddToScope thêm attr vào scope của ctx, mọi log ghi bằng ctx nào trong
// scope đó (kể cả ctx cha) từ giờ đều có. Không có scope thì bỏ qua:
//
//	logger.AddToScope(r.Context(), slog.String("actor_id", actor.AccountID.String()))
func AddToScope(ctx context.Context, attrs ...slog.Attr) {
	s, ok := ctx.Value(scopeKey{}).(*scope)
	if !ok || len(attrs) == 0 {
		return
	}
	s.mu.Lock()
	s.attrs = append(s.attrs, attrs...)
	s.mu.Unlock()
}

func scopeAttrs(ctx context.Context) []slog.Attr {
	s, ok := ctx.Value(scopeKey{}).(*scope)
	if !ok {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.attrs)
}

// contextHandler thêm các attr gắn bằng With và AddToScope vào từng bản ghi
// rồi chuyển cho handler thật.
//
// Attr của ctx luôn ở cấp ngoài cùng, kể cả khi logger đang mở group
// (log.WithGroup("job")). Attr thêm vào record sẽ rơi vào group đang mở, nên
// khi đã có group thì dựng lại handler từ root: thêm attr của ctx trước, rồi
// áp lại các WithAttrs/WithGroup theo đúng thứ tự.
type contextHandler struct {
	slog.Handler // đã áp mọi WithAttrs/WithGroup

	root    slog.Handler                      // handler thật, chưa áp gì
	ops     []func(slog.Handler) slog.Handler // các WithAttrs/WithGroup đã áp
	grouped bool                              // đã mở group nào chưa
}

func newContextHandler(h slog.Handler) contextHandler {
	return contextHandler{Handler: h, root: h}
}

func (h contextHandler) Handle(ctx context.Context, r slog.Record) error {
	attrs := slices.Concat(attrsFrom(ctx), scopeAttrs(ctx))
	if len(attrs) == 0 {
		return h.Handler.Handle(ctx, r)
	}
	if !h.grouped {
		r.AddAttrs(attrs...)
		return h.Handler.Handle(ctx, r)
	}
	inner := h.root.WithAttrs(attrs)
	for _, op := range h.ops {
		inner = op(inner)
	}
	return inner.Handle(ctx, r)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	return h.with(func(inner slog.Handler) slog.Handler { return inner.WithAttrs(attrs) }, false)
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	if name == "" { // slog: group rỗng là không có group
		return h
	}
	return h.with(func(inner slog.Handler) slog.Handler { return inner.WithGroup(name) }, true)
}

func (h contextHandler) with(op func(slog.Handler) slog.Handler, group bool) contextHandler {
	return contextHandler{
		Handler: op(h.Handler),
		root:    h.root,
		// Clip để handler anh em (cùng cha) không ghi đè ops của nhau
		ops:     append(slices.Clip(h.ops), op),
		grouped: h.grouped || group,
	}
}

type loggerKey struct{}

// NewContext gắn log vào ctx. RequestLogger gọi cho mỗi request, để code
// platform cần log (vd web.WriteProblem) dùng đúng logger của server mà không
// phải đọc biến toàn cục.
func NewContext(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, log)
}

// FromContext trả logger gắn bằng NewContext, không có thì slog.Default()
// (vd code chạy ngoài HTTP request)
func FromContext(ctx context.Context) *slog.Logger {
	if log, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && log != nil {
		return log
	}
	return slog.Default()
}
