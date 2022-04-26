package smtptest

import (
	"net/smtp"
	"testing"
)

const testMsg = "To: recipient@example.net\r\n" +
	"Subject: discount Gophers!\r\n" +
	"\r\n" +
	"This is the email body.\r\n"

func TestServer(t *testing.T) {
	ts, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ts.Close()
	})

	addr := ts.Addr()
	auth := smtp.PlainAuth("", "user@example.com", "password", ts.Host)
	if err := smtp.SendMail(addr, auth, "sender@example.org", []string{"recipient@example.net"}, []byte(testMsg)); err != nil {
		t.Fatal(err)
	}

	if len(ts.Messages()) != 1 {
		t.Errorf("got %v\nwant %v", len(ts.Messages()), 1)
	}
	sessions := ts.Sessions()
	msgs := ts.Messages()

	{
		got := sessions[0].From()
		want := "sender@example.org"
		if got != want {
			t.Errorf("got %v\nwant %v", got, want)
		}
	}

	{
		got := msgs[0].Header.Get("To")
		want := "recipient@example.net"
		if got != want {
			t.Errorf("got %v\nwant %v", got, want)
		}
	}
}

func TestServerWithAuth(t *testing.T) {
	ts, auth, err := NewServerWithAuth()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ts.Close()
	})
	addr := ts.Addr()

	invalidAuth := smtp.PlainAuth("", "user@example.com", "password", ts.Host)
	if err := smtp.SendMail(addr, invalidAuth, "sender@example.org", []string{"recipient@example.net"}, []byte(testMsg)); err == nil {
		t.Fatal("want err")
	}

	if err := smtp.SendMail(addr, auth, "sender@example.org", []string{"recipient@example.net"}, []byte(testMsg)); err != nil {
		t.Fatal(err)
	}

	sessions := ts.Sessions()
	msgs := ts.Messages()

	{
		if len(sessions) != 2 {
			t.Errorf("got %v\nwant %v", len(sessions), 2)
		}
		got := sessions[1].From()
		want := "sender@example.org"
		if got != want {
			t.Errorf("got %v\nwant %v", got, want)
		}
	}

	{
		if len(msgs) != 1 {
			t.Errorf("got %v\nwant %v", len(msgs), 1)
		}
		got := msgs[0].Header.Get("To")
		want := "recipient@example.net"
		if got != want {
			t.Errorf("got %v\nwant %v", got, want)
		}
	}
}
