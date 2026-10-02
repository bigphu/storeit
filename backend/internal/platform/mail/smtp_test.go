package mail

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Cùng bản với compose.yml
const mailpitImage = "axllent/mailpit:v1.31"

// startMailpit dựng Mailpit làm máy chủ SMTP thật. Không có Docker thì skip,
// trừ khi CI=true (giống dbtest).
func startMailpit(t *testing.T) (host string, smtpPort int, apiURL string) {
	t.Helper()
	if os.Getenv("CI") == "" {
		testcontainers.SkipIfProviderIsNotHealthy(t)
	}
	ctx := context.Background()
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        mailpitImage,
			ExposedPorts: []string{"1025/tcp", "8025/tcp"},
			// Chaos bật sẵn nhưng không có trigger nào: test nào cần lỗi thì tự đặt
			Env: map[string]string{"MP_ENABLE_CHAOS": "true"},
			WaitingFor: wait.ForAll(
				wait.ForListeningPort("1025/tcp"),
				wait.ForHTTP("/api/v1/info").WithPort("8025/tcp"),
			),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start mailpit: %v", err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(context.Background()) })
	host, err = ctr.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sp, err := ctr.MappedPort(ctx, "1025/tcp")
	if err != nil {
		t.Fatal(err)
	}
	ap, err := ctr.MappedPort(ctx, "8025/tcp")
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(sp.Port())
	return host, port, fmt.Sprintf("http://%s:%s", host, ap.Port())
}

func TestSMTPDeliversToServer(t *testing.T) {
	host, port, api := startMailpit(t)
	s, err := New(Config{
		Transport: TransportSMTP, From: "StoreIt <no-reply@storeit.test>", ReplyTo: "support@storeit.test",
		SMTPHost: host, SMTPPort: port, SMTPTLS: TLSNone,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Send(context.Background(), testMsg); err != nil {
		t.Fatalf("Send() = %v", err)
	}

	var list struct {
		Messages []struct {
			Subject string
			To      []struct{ Name, Address string }
			ReplyTo []struct{ Address string }
		} `json:"messages"`
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := http.Get(api + "/api/v1/messages")
		if err != nil {
			t.Fatal(err)
		}
		err = json.NewDecoder(resp.Body).Decode(&list)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(list.Messages) > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if len(list.Messages) != 1 {
		t.Fatalf("mailpit has %d messages, want 1", len(list.Messages))
	}
	got := list.Messages[0]
	if got.Subject != testMsg.Subject || len(got.To) != 1 || got.To[0].Address != "lan@storeit.test" || got.To[0].Name != "Nguyễn Lan" {
		t.Errorf("message = %+v", got)
	}
	if len(got.ReplyTo) != 1 || got.ReplyTo[0].Address != "support@storeit.test" {
		t.Errorf("reply-to = %+v", got.ReplyTo)
	}
}

// Máy chủ không nghe: lỗi tạm thời, job được thử lại
func TestSMTPConnectionRefusedIsTemporary(t *testing.T) {
	s, err := New(Config{
		Transport: TransportSMTP, From: "no-reply@storeit.test",
		SMTPHost: "127.0.0.1", SMTPPort: 1, SMTPTLS: TLSNone,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Send(context.Background(), testMsg)
	if err == nil {
		t.Fatal("Send() to a closed port = nil")
	}
	if IsPermanent(err) {
		t.Errorf("connection refused classified as permanent: %v", err)
	}
}

// Máy chủ từ chối người nhận: 5xx là vĩnh viễn (huỷ job), 4xx là tạm thời (thử lại).
// Dùng Chaos của Mailpit để có mã lỗi SMTP thật.
func TestSMTPRejectionClassification(t *testing.T) {
	host, port, api := startMailpit(t)
	s, err := New(Config{
		Transport: TransportSMTP, From: "no-reply@storeit.test",
		SMTPHost: host, SMTPPort: port, SMTPTLS: TLSNone,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		code      int
		permanent bool
	}{{550, true}, {451, false}} {
		t.Run(strconv.Itoa(tc.code), func(t *testing.T) {
			setChaos(t, api, tc.code)
			err := s.Send(context.Background(), testMsg)
			if err == nil {
				t.Fatal("Send() = nil, want a rejection")
			}
			if IsPermanent(err) != tc.permanent {
				t.Errorf("IsPermanent(%v) = %v, want %v", err, IsPermanent(err), tc.permanent)
			}
		})
	}
}

// setChaos làm Mailpit trả mã lỗi code cho mọi RCPT TO
func setChaos(t *testing.T, api string, code int) {
	t.Helper()
	body := fmt.Sprintf(`{"Recipient":{"ErrorCode":%d,"Probability":100}}`, code)
	req, _ := http.NewRequest(http.MethodPut, api+"/api/v1/chaos", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set chaos: status %d", resp.StatusCode)
	}
}
