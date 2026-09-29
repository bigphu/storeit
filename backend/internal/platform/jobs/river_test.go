package jobs

import (
	"reflect"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

func TestParseOpts(t *testing.T) {
	at := time.Date(2026, 9, 29, 8, 0, 0, 0, time.UTC)
	unique := river.UniqueOpts{
		ByArgs: true,
		ByState: []rivertype.JobState{
			rivertype.JobStateAvailable,
			rivertype.JobStatePending,
			rivertype.JobStateRunning,
			rivertype.JobStateRetryable,
			rivertype.JobStateScheduled,
		},
	}

	tests := []struct {
		name string
		opts []Option
		want river.InsertOpts
	}{
		// Không option => mọi field bằng 0, River tự lấy default
		{"none", nil, river.InsertOpts{}},
		{"nil option skipped", []Option{nil, WithQueue("mail")}, river.InsertOpts{Queue: "mail"}},
		{"all", []Option{WithQueue("mail"), WithPriority(2), WithSchedule(at), WithMaxAttempts(5), WithUniqueArgs()},
			river.InsertOpts{Queue: "mail", Priority: 2, ScheduledAt: at, MaxAttempts: 5, UniqueOpts: unique}},
		{"last wins", []Option{WithQueue("a"), WithQueue("b")}, river.InsertOpts{Queue: "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseOpts(tt.opts...); !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("got %+v, want %+v", *got, tt.want)
			}
		})
	}
}
