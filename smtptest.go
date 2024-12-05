package smtptest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/mail"
	netsmtp "net/smtp"
	"strconv"
	"sync"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/google/uuid"
)

var _ smtp.AuthSession = (*Session)(nil)

type onReceiveFunc func(from, to string, recipients []string, msg *mail.Message) error

type backend struct {
	username       *string
	password       *string
	sessions       []*Session
	mu             sync.RWMutex
	onReceiveFuncs []onReceiveFunc
}

func (be *backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	ses := &Session{
		username: be.username,
		password: be.password,
		be:       be,
	}
	be.mu.Lock()
	be.sessions = append(be.sessions, ses)
	be.mu.Unlock()
	return ses, nil
}

type Session struct {
	from       string
	to         string
	recipients []string
	rawMsg     io.Reader
	msg        *mail.Message
	username   *string
	password   *string
	be         *backend
	mu         sync.RWMutex
}

func (s *Session) Reset() {}

func (s *Session) Logout() error {
	return nil
}

func (s *Session) AuthMechanisms() []string {
	if s.be.username == nil {
		return nil
	}
	return []string{sasl.Plain}
}

func (s *Session) Auth(mech string) (sasl.Server, error) {
	if s.be.username == nil {
		return nil, smtp.ErrAuthUnsupported
	}
	return sasl.NewPlainServer(func(identity, username, password string) error {
		if identity != "" && identity != username {
			return errors.New("Invalid identity")
		}
		if username != *s.be.username || password != *s.be.password {
			return errors.New("Invalid username or password")
		}
		return nil
	}), nil
}

func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	return nil
}

func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	if s.to == "" {
		s.to = to
	}
	s.recipients = append(s.recipients, to)
	return nil
}

func (s *Session) Data(r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	msg, err := mail.ReadMessage(bytes.NewReader(b))
	if err != nil {
		return err
	}
	s.msg = msg
	s.rawMsg = bytes.NewReader(b)
	for _, fn := range s.be.onReceiveFuncs {
		msg, err := mail.ReadMessage(bytes.NewReader(b))
		if err != nil {
			return err
		}
		if err := fn(s.From(), s.To(), s.Recipients(), msg); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) From() string {
	return s.from
}

// Deprecated: use Recipients instead to retrieve all recipients.
func (s *Session) To() string {
	return s.to
}

func (s *Session) Recipients() []string {
	recipients := make([]string, len(s.recipients))
	copy(recipients, s.recipients)
	return recipients
}

func (s *Session) Message() *mail.Message {
	return s.msg
}

func (s *Session) RawMessage() io.Reader {
	return s.rawMsg
}

type Server struct {
	Host string
	Port int
	Err  error

	server  *smtp.Server
	backend *backend
	wg      sync.WaitGroup
}

type Option func(*backend) error

func WithPlainAuth(username, password string) Option {
	return func(be *backend) error {
		be.username = &username
		be.password = &password
		return nil
	}
}

func WithOnReceiveFunc(fn onReceiveFunc) Option {
	return func(be *backend) error {
		be.onReceiveFuncs = append(be.onReceiveFuncs, fn)
		return nil
	}
}

func NewServer(opts ...Option) (*Server, error) {
	be := &backend{}
	for _, opt := range opts {
		if err := opt(be); err != nil {
			return nil, err
		}
	}
	return newServer(be)
}

func NewServerWithAuth(opts ...Option) (*Server, netsmtp.Auth, error) {
	be := &backend{}
	for _, opt := range opts {
		if err := opt(be); err != nil {
			return nil, nil, err
		}
	}
	var username, password string
	if be.username == nil || be.password == nil {
		username = fmt.Sprintf("%s@example.com", uuid.NewString())
		password = uuid.NewString()
		opt := WithPlainAuth(username, password)
		if err := opt(be); err != nil {
			return nil, nil, err
		}
	} else {
		username = *be.username
		password = *be.password
	}
	s, err := newServer(be)
	if err != nil {
		return nil, nil, err
	}
	auth := netsmtp.PlainAuth("", username, password, s.Host)
	return s, auth, nil
}

func newServer(be *backend) (*Server, error) {
	s := &Server{
		server:  smtp.NewServer(be),
		backend: be,
	}

	laddr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}

	s.server.Addr = laddr.String()
	s.server.Domain = "localhost"
	s.server.ReadTimeout = 60 * time.Second
	s.server.WriteTimeout = 60 * time.Second
	s.server.AllowInsecureAuth = true

	network := "tcp"
	if s.server.LMTP {
		network = "unix"
	}
	l, err := net.Listen(network, s.server.Addr)
	if err != nil {
		return nil, err
	}

	host, portStr, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return nil, err
	}
	port, err := strconv.ParseInt(portStr, 10, 64)
	if err != nil {
		return nil, err
	}
	s.Host = host
	s.Port = int(port)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.server.Serve(l); err != nil {
			s.Err = errors.Join(s.Err, err)
		}
	}()

	return s, nil
}

func (s *Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

func (s *Server) Sessions() []*Session {
	s.backend.mu.RLock()
	defer s.backend.mu.RUnlock()
	return s.backend.sessions
}

func (s *Server) Messages() []*mail.Message {
	msgs := []*mail.Message{}
	s.backend.mu.RLock()
	defer s.backend.mu.RUnlock()
	for _, ses := range s.backend.sessions {
		ses.mu.RLock()
		if ses.msg == nil {
			ses.mu.RUnlock()
			continue
		}
		msgs = append(msgs, ses.msg)
		ses.mu.RUnlock()
	}
	return msgs
}

func (s *Server) RawMessages() []io.Reader {
	raws := []io.Reader{}
	s.backend.mu.RLock()
	defer s.backend.mu.RUnlock()
	for _, ses := range s.backend.sessions {
		ses.mu.RLock()
		if ses.rawMsg == nil {
			ses.mu.RUnlock()
			continue
		}
		raws = append(raws, ses.rawMsg)
		ses.mu.RUnlock()
	}
	return raws
}

// Deprecated
func (s *Server) OnReceive(fn func(from, to string, recipients []string, msg *mail.Message) error) {
	s.backend.onReceiveFuncs = append(s.backend.onReceiveFuncs, fn)
}

func (s *Server) Close() {
	_ = s.server.Close()
	s.wg.Wait()
}
