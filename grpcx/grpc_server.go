package grpcx

import (
	"net"
	"sync"
	"time"

	"github.com/soheilhy/cmux"
	"google.golang.org/grpc"
)

type GrpcServer struct {
	mu         sync.RWMutex
	l          net.Listener
	grpcServer *grpc.Server

	closed bool
}

// NewGrpcServer creates a new grpc server.
func NewGrpcServer() *GrpcServer {
	s := &GrpcServer{}
	s.grpcServer = grpc.NewServer()
	return s
}

// MuxMatch splits grpc Listener.
func (s *GrpcServer) MuxMatch(m cmux.CMux) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.l = m.MatchWithWriters(cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"))
}

// RegisterService registers grpc service by this method.
func (s *GrpcServer) RegisterService(registerFunc func(grpcServer *grpc.Server)) {
	registerFunc(s.grpcServer)
}

func (s *GrpcServer) Start() error {
	for {
		if s.closed {
			return nil
		}
		s.mu.RLock()
		l := s.l
		s.mu.RUnlock()
		if l != nil {
			break
		}
		time.Sleep(time.Second) // wait rpcx server starts
	}

	s.mu.RLock()
	l := s.l
	s.mu.RUnlock()

	if err := s.grpcServer.Serve(l); err != cmux.ErrListenerClosed {
		return err
	}

	return nil
}

// Close closes the grpc server.
func (s *GrpcServer) Close() error {
	s.grpcServer.Stop()
	s.closed = true
	return s.l.Close()
}
