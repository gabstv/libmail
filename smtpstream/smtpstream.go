package smtpstream

import (
	"crypto/tls"
	"io"
	"net"
	"net/smtp"
	"time"
)

func SendMail(addr string, a smtp.Auth, from string, to []string, msg io.Reader, tlsc *tls.Config) error {
	return sendMail(addr, a, from, to, msg, time.Second*30, tlsc)
}

func sendMail(addr string, a smtp.Auth, from string, to []string, msg io.Reader, timeout time.Duration, tlsc *tls.Config) error {
	var c *smtp.Client
	var err error
	if timeout == 0 {
		c, err = smtp.Dial(addr)
	} else {
		c, err = DialWithTimeout(addr, timeout)
	}
	if err != nil {
		return err
	}

	defer c.Close()
	if err = c.Hello("localhost"); err != nil {
		return err
	}
	if tlsc != nil {
		err = c.StartTLS(tlsc)
	}
	if err != nil {
		return err
	}
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err = c.StartTLS(nil); err != nil {
			return err
		}
	}
	if a != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err = c.Auth(a); err != nil {
				return err
			}
		}
	}
	if err = c.Mail(from); err != nil {
		return err
	}
	for _, addr := range to {
		if err = c.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	_, err = io.Copy(w, msg)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	return c.Quit()
}

func DialWithTimeout(addr string, timeout time.Duration) (*smtp.Client, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	host, _, _ := net.SplitHostPort(addr)
	return smtp.NewClient(conn, host)
}
