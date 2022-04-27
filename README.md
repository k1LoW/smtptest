# smtptest

`smtptest` provides SMTP server for testing.

## Usage

``` go
package main

import (
	"fmt"
	"net/smtp"
	"strings"
	"testing"

	"github.com/k1LoW/smtptest"
)

const testMsg = "To: recipient@example.net\r\n" +
	"Subject: discount Gophers!\r\n" +
	"\r\n" +
	"This is the email body.\r\n"

func TestServer(t *testing.T) {
	ts, err := smtptest.NewServer()
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
	msgs := ts.Messages()

	got := msgs[0].Header.Get("To")
	want := "recipient@example.net"
	if got != want {
		t.Errorf("got %v\nwant %v", got, want)
	}
}
```

## References

- https://github.com/influxdata/kapacitor/tree/master/services/smtp/smtptest
