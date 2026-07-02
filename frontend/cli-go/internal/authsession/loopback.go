package authsession

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// LoopbackServerProvider starts a one-shot listener bound to 127.0.0.1 for the
// Google OAuth redirect used by desktop CLI login.
type LoopbackServerProvider struct{}

func (LoopbackServerProvider) Start(ctx context.Context) (LoopbackSession, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	session := &loopbackServerSession{
		listener: listener,
		values:   make(chan url.Values, 1),
		errs:     make(chan error, 1),
	}
	server := &http.Server{Handler: session}
	session.server = server
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			select {
			case session.errs <- err:
			default:
			}
		}
	}()
	go func() {
		<-ctx.Done()
		_ = session.Close()
	}()
	return session, nil
}

type loopbackServerSession struct {
	listener net.Listener
	server   *http.Server
	values   chan url.Values
	errs     chan error
	once     sync.Once
}

func (s *loopbackServerSession) RedirectURI() string {
	return "http://" + s.listener.Addr().String()
}

func (s *loopbackServerSession) Wait(ctx context.Context) (url.Values, error) {
	select {
	case values := <-s.values:
		return values, nil
	case err := <-s.errs:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *loopbackServerSession) Close() error {
	var err error
	s.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err = s.server.Shutdown(ctx)
	})
	return err
}

func (s *loopbackServerSession) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if host, _, err := net.SplitHostPort(r.Host); err == nil && host != "127.0.0.1" {
		http.Error(w, "invalid loopback host", http.StatusBadRequest)
		return
	}
	values := r.URL.Query()
	select {
	case s.values <- values:
		fmt.Fprintln(w, "sqlrs login complete. You can close this browser tab.")
		go func() { _ = s.Close() }()
	default:
		http.Error(w, "callback already received", http.StatusConflict)
	}
}
