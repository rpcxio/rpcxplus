package grpcx

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/smallnest/rpcx/client"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/examples/helloworld/helloworld"
)

func TestGrpcClientPlugin_GrpcClient(t *testing.T) {
	lis, err := net.Listen("tcp", ":0")
	assert.NoError(t, err)
	defer lis.Close()

	s := grpc.NewServer()
	helloworld.RegisterGreeterServer(s, &GreeterService{})

	go s.Serve(lis)
	time.Sleep(time.Second)

	//
	// grpc client visits
	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure(), grpc.WithTimeout(time.Second))
	assert.NoError(t, err)

	defer conn.Close()
	c := helloworld.NewGreeterClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.SayHello(ctx, &helloworld.HelloRequest{Name: "smallnest"})
	assert.NoError(t, err)
	assert.Equal(t, "hello smallnest", r.Message)
}

func TestGrpcClientPlugin_GrpcxClient(t *testing.T) {
	lis, err := net.Listen("tcp", ":0")
	assert.NoError(t, err)
	defer lis.Close()

	s := grpc.NewServer()
	helloworld.RegisterGreeterServer(s, &GreeterService{})

	go s.Serve(lis)
	time.Sleep(time.Second)

	// grpcx client
	gcp := NewGrpcClientPlugin([]grpc.DialOption{grpc.WithInsecure()}, nil)

	rpcxClient, err := gcp.GenerateClient(fmt.Sprintf("grpc@%s", lis.Addr().String()), "helloworld.Greeter", "SayHello")
	assert.NoError(t, err)
	assert.NotNil(t, rpcxClient)

	var argv = &helloworld.HelloRequest{
		Name: "smallnest",
	}
	var reply = &helloworld.HelloReply{}
	err = rpcxClient.Call(context.Background(), "helloworld.Greeter", "SayHello", argv, reply)
	assert.NoError(t, err)
	assert.Equal(t, "hello smallnest", reply.Message)
}

func TestGrpcClientPlugin_XClient(t *testing.T) {
	lis, err := net.Listen("tcp", ":0")
	assert.NoError(t, err)
	defer lis.Close()

	s := grpc.NewServer()
	helloworld.RegisterGreeterServer(s, &GreeterService{})

	go s.Serve(lis)
	time.Sleep(time.Second)

	// register CacheClientBuilder
	gcp := NewGrpcClientPlugin([]grpc.DialOption{grpc.WithInsecure()}, nil)
	client.RegisterCacheClientBuilder("grpc", gcp)

	// rpcx client
	d, _ := client.NewPeer2PeerDiscovery("grpc@"+lis.Addr().String(), "")
	opt := client.DefaultOption
	xclient := client.NewXClient("helloworld.Greeter", client.Failtry, client.RandomSelect, d, opt)
	defer xclient.Close()

	argv := &helloworld.HelloRequest{
		Name: "smallnest",
	}
	reply := &helloworld.HelloReply{}
	err = xclient.Call(context.Background(), "SayHello", argv, reply)
	assert.NoError(t, err)
	assert.Equal(t, "hello smallnest", reply.Message)
}
