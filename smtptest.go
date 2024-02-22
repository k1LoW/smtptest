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

	"github.com/emersion/go-smtp"
	"github.com/google/uuid"
)

type backend struct {
	username *string
	password *string
	sessions []*Session
	mu       sync.RWMutex
}

func (be *backend) NewSession(state smtp.ConnectionState, _ string) (smtp.Session, error) {
	ses := &Session{
		state:    &state,
		username: be.username,
		password: be.password,
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
	state      *smtp.ConnectionState
	username   *string
	password   *string
	mu         sync.RWMutex
}

func (s *Session) Reset() {}

func (s *Session) Logout() error {
	return nil
}

func (s *Session) AuthPlain(username, password string) error {
	if s.username != nil && s.password != nil && (*s.username != username || *s.password != password) {
		return errors.New("invalid username or password")
	}
	return nil
}

func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	return nil
}

func (s *Session) Rcpt(to string) error {
	s.to = to
	s.recipients = append(s.recipients, to)
	return nil
}

func (s *Session) Data(r io.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := new(bytes.Buffer)
	a := io.TeeReader(r, b)
	msg, err := mail.ReadMessage(a)
	if err != nil {
		return err
	}
	s.msg = msg
	s.rawMsg = b
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

func NewServer() (*Server, error) {
	return newServer(&backend{})
}

func NewServerWithAuth() (*Server, netsmtp.Auth, error) {
	username := fmt.Sprintf("%s@example.com", uuid.NewString())
	password := uuid.NewString()
	s, err := newServer(&backend{
		username: &username,
		password: &password,
	})
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
	s.server.ReadTimeout = 10 * time.Second
	s.server.WriteTimeout = 10 * time.Second
	s.server.MaxMessageBytes = 1024 * 1024
	s.server.MaxRecipients = 50
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

func (s *Server) Close() {
	_ = s.server.Close()
	s.wg.Wait()
}
