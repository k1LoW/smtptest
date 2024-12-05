package smtptest

import (
	"net/mail"
	"net/smtp"
	"sort"
	"strings"
	"testing"

	"github.com/jhillyerd/enmime"
	"golang.org/x/sync/errgroup"
)

const testMsg = "To: recipient@example.net\r\n" +
	"Subject: discount Gophers!\r\n" +
	"\r\n" +
	"This is the email body.\r\n"

func TestServer(t *testing.T) {
	ts, err := NewServer(WithOnReceiveFunc(func(from, to string, recipients []string, msg *mail.Message) error {
		{
			got := from
			want := "sender@example.org"
			if got != want {
				t.Errorf("got %v\nwant %v", got, want)
			}
		}
		{
			got := to
			want := "recipient@example.net"
			if got != want {
				t.Errorf("got %v\nwant %v", got, want)
			}
		}
		{
			got := msg.Header.Get("To")
			want := "recipient@example.net"
			if got != want {
				t.Errorf("got %v\nwant %v", got, want)
			}
		}
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ts.Close()
	})

	addr := ts.Addr()
	if err := smtp.SendMail(addr, nil, "sender@example.org", []string{"recipient@example.net"}, []byte(testMsg)); err != nil {
		t.Fatal(err)
	}

	if len(ts.Messages()) != 1 {
		t.Errorf("got %v\nwant %v", len(ts.Messages()), 1)
	}
	sessions := ts.Sessions()
	msgs := ts.Messages()
	raws := ts.RawMessages()

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

	{
		e, err := enmime.ReadEnvelope(raws[0])
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(e.Text, "This is the email body.") {
			t.Errorf("got %v\n", e.Text)
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

func TestServerMultipleRecipients(t *testing.T) {
	ts, err := NewServer(WithOnReceiveFunc(func(from, to string, recipients []string, msg *mail.Message) error {
		{
			got := from
			want := "sender@example.org"
			if got != want {
				t.Errorf("got %v\nwant %v", got, want)
			}
		}
		{
			got := to
			want := "recipient@example.net"
			if got != want {
				t.Errorf("got %v\nwant %v", got, want)
			}
		}
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ts.Close()
	})

	addr := ts.Addr()
	if err := smtp.SendMail(addr, nil, "sender@example.org", []string{"recipient@example.net", "another_recipient@example.net"}, []byte(testMsg)); err != nil {
		t.Fatal(err)
	}

	sessions := ts.Sessions()
	if len(sessions) != 1 {
		t.Errorf("got %v\nwant %v", len(sessions), 1)
	}

	recipients := sessions[0].Recipients()
	if len(recipients) != 2 {
		t.Errorf("got %v\nwant %v", len(recipients), 2)
	}
	sort.Strings(recipients)
	{
		got := recipients[0]
		want := "another_recipient@example.net"
		if got != want {
			t.Errorf("got %v\nwant %v", got, want)
		}
	}
	{
		got := recipients[1]
		want := "recipient@example.net"
		if got != want {
			t.Errorf("got %v\nwant %v", got, want)
		}
	}
}

func TestServerParallel(t *testing.T) {
	ts, err := NewServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ts.Close()
	})

	addr := ts.Addr()

	eg := new(errgroup.Group)
	for i := 0; i < 100; i++ {
		eg.Go(func() error {
			return smtp.SendMail(addr, nil, "sender@example.org", []string{"recipient@example.net", "another_recipient@example.net"}, []byte(testMsg))
		})
	}
	if err := eg.Wait(); err != nil {
		t.Fatal(err)
	}

	if len(ts.Messages()) != 100 {
		t.Errorf("got %v\nwant %v", len(ts.Messages()), 100)
	}
}
