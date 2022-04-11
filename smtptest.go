package smtptest

import (
	"io"
	"net"
	"net/mail"
	"strconv"
	"sync"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/hashicorp/go-multierror"
)

type Backend struct {
	sessions []*Session
}

func (be *Backend) Login(state *smtp.ConnectionState, username, password string) (smtp.Session, error) {
	return be.newSession(state)
}

func (be *Backend) AnonymousLogin(state *smtp.ConnectionState) (smtp.Session, error) {
	return be.newSession(state)
}

func (be *Backend) newSession(state *smtp.ConnectionState) (smtp.Session, error) {
	ses := &Session{state: state}
	be.sessions = append(be.sessions, ses)
	return ses, nil
}

type Session struct {
	from  string
	to    string
	msg   *mail.Message
	state *smtp.ConnectionState
}

func (s *Session) Mail(from string, opts smtp.MailOptions) error {
	s.from = from
	return nil
}

func (s *Session) Rcpt(to string) error {
	s.to = to
	return nil
}

func (s *Session) Data(r io.Reader) error {
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return err
	}
	s.msg = msg
	return nil
}

func (s *Session) Reset() {}

func (s *Session) Logout() error {
	return nil
}

func (s *Session) From() string {
	return s.from
}

func (s *Session) To() string {
	return s.to
}

func (s *Session) Message() *mail.Message {
	return s.msg
}

type Server struct {
	Host string
	Port int
	Err  error

	server  *smtp.Server
	backend *Backend
	wg      sync.WaitGroup
}

func NewServer() (*Server, error) {
	be := &Backend{}
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
			s.Err = multierror.Append(s.Err, err)
		}
	}()

	return s, nil
}

func (s *Server) Sessions() []*Session {
	return s.backend.sessions
}

func (s *Server) Messages() []*mail.Message {
	msgs := []*mail.Message{}
	for _, ses := range s.backend.sessions {
		if ses.msg == nil {
			continue
		}
		msgs = append(msgs, ses.msg)
	}
	return msgs
}

func (s *Server) Close() error {
	if err := s.server.Close(); err != nil {
		return err
	}
	s.wg.Wait()
	return nil
}
