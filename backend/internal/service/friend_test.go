package service

import "testing"

func TestErrBadRequest(t *testing.T) {
	var e errBadRequest = "测试错误"
	if e.Error() != "测试错误" {
		t.Fatal("unexpected message")
	}
}
