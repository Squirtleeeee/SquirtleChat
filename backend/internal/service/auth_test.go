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

func TestUpdateProfileEmptyNickname(t *testing.T) {
	s := &AuthService{}
	_, err := s.UpdateProfile(context.Background(), 1, "", "")
	if err == nil {
		t.Fatal("expected error for empty nickname")
	}
}
