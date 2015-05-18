package libmail

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/gabstv/libmail/smtpstream"
	"github.com/sloonz/go-mime-message"
	"github.com/sloonz/go-qprintable"
	"io"
	"log"
	"net/mail"
	"net/smtp"
)

var (
	Verbose = false
)

type Message struct {
	From          mail.Address
	To            []mail.Address
	Subject       string
	HTMLBody      string
	PlaintextBody string
	Files         *AttachmentList
	RawHeaders    map[string]string
}

func NewMessage() *Message {
	m := &Message{}
	m.Files = NewAttachmentList()
	m.RawHeaders = make(map[string]string)
	return m
}

func (m *Message) AddRecipient(to mail.Address) {
	if m.To == nil {
		m.To = make([]mail.Address, 0)
	}
	m.To = append(m.To, to)
}

type SMTP struct {
	Auth    smtp.Auth
	Address string
	TLS     *tls.Config
}

func NewSMTP(auth smtp.Auth, address string) *SMTP {
	return &SMTP{auth, address, nil}
}

func (s *SMTP) SetTLS(tlsc *tls.Config) {
	s.TLS = tlsc
}

func (s *SMTP) SubmitHTML(fromEmail, fromName, toEmail, toName, subject, htmlBody string, files *AttachmentList) (int, error) {
	m := NewMessage()
	m.From.Name = fromName
	m.From.Address = fromEmail
	m.AddRecipient(mail.Address{toName, toEmail})
	m.HTMLBody = htmlBody
	m.Files = files
	m.Subject = subject
	return s.submit(m)
}

func (s *SMTP) SubmitPlaintext(fromEmail, fromName, toEmail, toName, subject, plainBody string, files *AttachmentList) (int, error) {
	m := NewMessage()
	m.From.Name = fromName
	m.From.Address = fromEmail
	m.AddRecipient(mail.Address{toName, toEmail})
	m.PlaintextBody = plainBody
	m.Files = files
	m.Subject = subject
	return s.submit(m)
}

func (s *SMTP) SubmitMixed(fromEmail, fromName, toEmail, toName, subject, plainBody string, htmlBody string, files *AttachmentList) (int, error) {
	m := NewMessage()
	m.From.Name = fromName
	m.From.Address = fromEmail
	m.AddRecipient(mail.Address{toName, toEmail})
	m.HTMLBody = htmlBody
	m.PlaintextBody = plainBody
	m.Files = files
	m.Subject = subject
	return s.submit(m)
}

func (s *SMTP) SubmitMessage(msg *Message) error {
	_, err := s.submit(msg)
	return err
}

func (s *SMTP) submit(msg *Message) (int, error) {
	streams := make([]io.ReadCloser, 0, 512)

	// function to close all the streams on exit
	defer func() {
		for k := range streams {
			if streams[k] != nil {
				streams[k].Close()
			}
		}
	}()

	//var buffer bytes.Buffer
	bmarker := newBoundary()
	multipartmessage := message.NewMultipartMessage("mixed", bmarker)

	multipartmessage.SetHeader("From", fmt.Sprintf("%s <%s>", message.EncodeWord(msg.From.Name), msg.From.Address))
	multipartmessage.SetHeader("Return-Path", fmt.Sprintf("<%s>", msg.From.Address))
	// TO
	var tol bytes.Buffer
	for i := 0; i < len(msg.To); i++ {
		if i > 0 {
			tol.WriteString(", ")
		}
		tol.WriteString(message.EncodeWord(msg.To[i].Name))
		tol.WriteString(" <")
		tol.WriteString(msg.To[i].Address)
		tol.WriteString(">")
	}
	multipartmessage.SetHeader("To", tol.String())
	// SUBJECT
	multipartmessage.SetHeader("Subject", message.EncodeWord(msg.Subject))
	//// MIME-Version
	multipartmessage.SetHeader("MIME-Version", "1.0")
	//// [END] [HEADERS]
	//
	if Verbose {
		log.Println("msg.HTMLBody", msg.HTMLBody)
		log.Println("msg.PlaintextBody", msg.PlaintextBody)
	}

	if len(msg.HTMLBody) > 0 && len(msg.PlaintextBody) > 0 {
		alternatives := message.NewMultipartMessage("alternative", newBoundary())
		hmsg := message.NewTextMessage(qprintable.UnixTextEncoding, bytes.NewBufferString(msg.HTMLBody))
		hmsg.SetHeader("Content-Type", "text/html; charset=UTF-8")
		alternatives.AddPart(hmsg)
		tmsg := message.NewTextMessage(qprintable.UnixTextEncoding, bytes.NewBufferString(msg.PlaintextBody))
		tmsg.SetHeader("Content-Type", "text/plain; charset=UTF-8")
		alternatives.AddPart(tmsg)
		multipartmessage.AddPart(&alternatives.Message)
	} else if len(msg.HTMLBody) > 0 {
		hmsg := message.NewTextMessage(qprintable.UnixTextEncoding, bytes.NewBufferString(msg.HTMLBody))
		hmsg.SetHeader("Content-Type", "text/html; charset=UTF-8")
		multipartmessage.AddPart(hmsg)
	} else if len(msg.PlaintextBody) > 0 {
		tmsg := message.NewTextMessage(qprintable.UnixTextEncoding, bytes.NewBufferString(msg.PlaintextBody))
		tmsg.SetHeader("Content-Type", "text/plain; charset=UTF-8")
		multipartmessage.AddPart(tmsg)
	}

	tols := make([]string, len(msg.To))
	for k, v := range msg.To {
		tols[k] = v.Address
	}
	// CUSTOM HEADERS
	if msg.RawHeaders != nil {
		for k, v := range msg.RawHeaders {
			multipartmessage.SetHeader(k, v)
		}
	}
	//
	if msg.Files == nil {
		goto Submit
	}
	if msg.Files.Count() < 1 {
		goto Submit
	}

	for curItem := msg.Files.First(); curItem != nil; curItem = curItem.Next() {
		stream, err := curItem.Value.GetStream()
		if err != nil {
			return 0, err
		}
		streams = append(streams, stream)
		msg00 := message.NewBinaryMessage(stream)
		msg00.SetHeader("Content-Type", fmt.Sprintf("%v; name=\"%v\"", curItem.Value.MimeType, message.EncodeWord(curItem.Value.Name)))
		msg00.SetHeader("Content-Disposition", fmt.Sprintf("attachment; filename=\"%v\"", message.EncodeWord(curItem.Value.Name)))
		multipartmessage.AddPart(msg00)
	}

Submit:
	var rrdr io.Reader
	if Verbose {
		bf := new(bytes.Buffer)
		io.Copy(bf, multipartmessage)
		log.Println(bf.String())
		rrdr = bf
	} else {
		rrdr = multipartmessage
	}
	snd := msg.From.Address
	if len(msg.RawHeaders["Sender"]) > 0 {
		snd = msg.RawHeaders["Sender"]
	}
	return smtpstream.SendMail(s.Address, s.Auth, snd, tols, rrdr, s.TLS)
}
