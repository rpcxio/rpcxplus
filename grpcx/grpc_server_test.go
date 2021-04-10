package grpcx

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/smallnest/rpcx/server"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/examples/helloworld/helloworld"
)

type GreeterService struct {
	helloworld.UnimplementedGreeterServer
}

func (*GreeterService) SayHello(ctx context.Context, req *helloworld.HelloRequest) (*helloworld.HelloReply, error) {
	fmt.Println("server received:", req.Name)
	reply := &helloworld.HelloReply{Message: "hello " + req.GetName()}
	return reply, nil
}

func (*GreeterService) Greet(ctx context.Context, req *helloworld.HelloRequest, reply *helloworld.HelloReply) error {
	*reply = helloworld.HelloReply{Message: "hello " + req.Name}
	return nil
}

func TestGrpcServerPlugin(t *testing.T) {
	// create rpcx server and grpc server,
	// and add grpc server into rpcx's plugins.
	s := server.NewServer()
	gs := NewGrpcServerPlugin()
	s.Plugins.Add(gs)

	greetService := &GreeterService{}

	// register rpcx service
	err := s.Register(greetService, "")
	assert.NoError(t, err)
	// register grpc service
	gs.RegisterService(func(grpcServer *grpc.Server) {
		helloworld.RegisterGreeterServer(grpcServer, greetService)
	})

	// must start rpcx server
	go s.Serve("tcp", "127.0.0.1:0")
	defer s.Close()
	time.Sleep(100 * time.Millisecond)

	// and starts grpc server
	go func() {
		err := gs.Start()
		assert.NoError(t, err)
	}()

	// grpc client visits
	conn, err := grpc.Dial(s.Address().String(), grpc.WithInsecure(), grpc.WithTimeout(time.Second))
	assert.NoError(t, err)

	defer conn.Close()
	c := helloworld.NewGreeterClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.SayHello(ctx, &helloworld.HelloRequest{Name: "smallnest"})
	assert.NoError(t, err)
	assert.Equal(t, "hello smallnest", r.Message)
}
