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
