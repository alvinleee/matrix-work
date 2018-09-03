package core

import "testing"

func testVerifierMsg(t *testing.T) {
	// 发送身份订阅消息
	got := update()
	want := "recv roleChange msg"
	assertCorrectMessage(t, got, want)
}

func TestLeaderCalculateUpdate(t *testing.T) {
	assertCorrectMessage := func(t *testing.T, got, want string) {
		t.Helper()
		if got != want {
			t.Errorf("got '%s' want '%s' ", got, want)
		}
	}

	//收到验证者消息
	t.Run("收到验证者消息", testVerifierMsg)

	//收到非验证者消息
	t.Run("收到非验证者消息", testOtherMsg)
}
