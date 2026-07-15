package service

import (
	"context"
	"testing"
)

func TestSearchUsersEmptyQuery(t *testing.T) {
	s := &AuthService{}
	_, err := s.SearchUsers(context.Background(), "", 10)
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestChangePasswordValidation(t *testing.T) {
	s := &AuthService{}
	if err := s.ChangePassword(context.Background(), 1, "", "abcdef"); err == nil {
		t.Fatal("expected error for empty old password")
	}
	if err := s.ChangePassword(context.Background(), 1, "oldpass", "123"); err == nil {
		t.Fatal("expected error for short new password")
	}
	if err := s.ChangePassword(context.Background(), 1, "same", "same"); err == nil {
		t.Fatal("expected error when passwords match")
	}
}
