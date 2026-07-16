package document

import (
	"testing"

	"enterprise-rag/api/internal/model"
)

func TestCanWriteSubjectRequiresOwnership(t *testing.T) {
	tests := []struct {
		name    string
		subject *model.Subject
		userID  string
		want    bool
	}{
		{name: "owner can write private subject", subject: &model.Subject{OwnerID: "owner", Visibility: "private"}, userID: "owner", want: true},
		{name: "owner can write public subject", subject: &model.Subject{OwnerID: "owner", Visibility: "public"}, userID: "owner", want: true},
		{name: "non-owner cannot write public subject", subject: &model.Subject{OwnerID: "owner", Visibility: "public"}, userID: "reader", want: false},
		{name: "missing subject cannot be written", subject: nil, userID: "owner", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canWriteSubject(tt.subject, tt.userID); got != tt.want {
				t.Fatalf("canWriteSubject() = %t, want %t", got, tt.want)
			}
		})
	}
}
