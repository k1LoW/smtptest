# smtptest [![build](https://github.com/k1LoW/smtptest/actions/workflows/ci.yml/badge.svg)](https://github.com/k1LoW/smtptest/actions/workflows/ci.yml) ![Coverage](https://raw.githubusercontent.com/k1LoW/octocovs/main/badges/k1LoW/smtptest/coverage.svg) ![Code to Test Ratio](https://raw.githubusercontent.com/k1LoW/octocovs/main/badges/k1LoW/smtptest/ratio.svg) ![Test Execution Time](https://raw.githubusercontent.com/k1LoW/octocovs/main/badges/k1LoW/smtptest/time.svg)

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

const testMsg = "To: alice@example.net\r\n" +
	"Subject: Hello Gophers!\r\n" +
	"\r\n" +
	"This is the email body.\r\n"

func TestSendMail(t *testing.T) {
	ts, auth, err := smtptest.NewServerWithAuth()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ts.Close()
	})

	addr := ts.Addr()
	if err := smtp.SendMail(addr, auth, "sender@example.org", []string{"alice@example.net"}, []byte(testMsg)); err != nil {
		t.Fatal(err)
	}

	if len(ts.Messages()) != 1 {
		t.Errorf("got %v\nwant %v", len(ts.Messages()), 1)
	}
	msgs := ts.Messages()

	got := msgs[0].Header.Get("To")
	want := "alice@example.net"
	if got != want {
		t.Errorf("got %v\nwant %v", got, want)
	}
}
```

## References

- https://github.com/influxdata/kapacitor/tree/master/services/smtp/smtptest
