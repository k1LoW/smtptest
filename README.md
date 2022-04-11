# smtptest

`smtptest` provides SMTP server for testing.

## Usage

``` go
package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/k1LoW/smtptest"
)

func TestServer(t *testing.T) {
	ts, err := smtptest.NewServer()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ts.Close()
	})

	addr := fmt.Sprintf("%s:%d", ts.Host, ts.Port)

	// ref: https://github.com/emersion/go-smtp#client
	auth := sasl.NewPlainClient("", "user@example.com", "password")
	to := []string{"recipient@example.net"}
	msg := strings.NewReader("To: recipient@example.net\r\n" +
		"Subject: discount Gophers!\r\n" +
		"\r\n" +
		"This is the email body.\r\n")
	if err := smtp.SendMail(addr, auth, "sender@example.org", to, msg); err != nil {
		t.Fatal(err)
	}

	if len(ts.Messages()) != 1 {
		t.Errorf("got %v\nwant %v", len(ts.Messages()), 1)
	}
	msgs := ts.Messages()

	got := msgs[0].Header.Get("To")
	want := "recipient@example.net"
	if got != want {
		t.Errorf("got %v\nwant %v", got, want)
	}
}
```
