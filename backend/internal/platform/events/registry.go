package events

import (
	"context"
	"fmt"
	"slices"
)

// Handler là một phản ứng với event. Phải idempotent: River có thể giao một
// event nhiều lần (retry, crash giữa chừng).
type Handler func(ctx context.Context, e Event) error

// Registry nối loại event với các subscriber có tên. cmd/api và cmd/worker
// dựng cùng một registry: API cần biết enqueue job nào, worker cần biết chạy
// code nào. Đăng ký hết lúc khởi động, trước khi dùng; không an toàn khi
// đăng ký song song.
type Registry struct {
	byType   map[string][]string // loại event -> tên subscriber
	all      []string            // subscriber nhận mọi event
	handlers map[string]Handler  // tên subscriber -> handler
}

func NewRegistry() *Registry {
	return &Registry{
		byType:   map[string][]string{},
		handlers: map[string]Handler{},
	}
}

// On đăng ký subscriber cho một hay nhiều loại event. Tên dạng
// "<module>.<phản ứng>", là khoá của job nên không được đổi khi còn job đang
// chờ, và mỗi tên chỉ đăng ký một lần (gom mọi loại event vào một lần gọi):
//
//	r.On("notifications.checkout_email", m.subscriber.CheckoutEmail,
//		invcontract.EventAssetCheckedOut, invcontract.EventAssetReturned)
func (r *Registry) On(subscriber string, h Handler, eventTypes ...string) {
	if len(eventTypes) == 0 {
		panic(fmt.Sprintf("events: subscriber %q needs at least one event type", subscriber))
	}
	r.add(subscriber, h)
	for _, t := range slices.Compact(slices.Sorted(slices.Values(eventTypes))) {
		r.byType[t] = append(r.byType[t], subscriber)
	}
}

// OnAll đăng ký subscriber nhận mọi event, vd activity ghi audit trail
func (r *Registry) OnAll(subscriber string, h Handler) {
	r.add(subscriber, h)
	r.all = append(r.all, subscriber)
}

func (r *Registry) add(subscriber string, h Handler) {
	if subscriber == "" || h == nil {
		panic("events: subscriber needs a name and a handler")
	}
	if _, dup := r.handlers[subscriber]; dup {
		panic(fmt.Sprintf("events: subscriber %q registered twice", subscriber))
	}
	r.handlers[subscriber] = h
}

// For trả tên các subscriber của eventType, đã sắp xếp
func (r *Registry) For(eventType string) []string {
	subs := slices.Concat(r.all, r.byType[eventType])
	slices.Sort(subs)
	return subs
}

func (r *Registry) handler(subscriber string) (Handler, bool) {
	h, ok := r.handlers[subscriber]
	return h, ok
}
