package grpcx

import (
	"context"
	"fmt"
	"log"
	"net"
	"testing"
	"time"

	"github.com/smallnest/rpcx/client"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/examples/helloworld/helloworld"
)

func TestGrpcClientPlugin(t *testing.T) {
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

	//
	// grpcx client
	gcp := NewGrpcClientPlugin()
	gcp.Register("GreeterService", func(addr string) interface{} {
		conn, err := grpc.Dial(addr, grpc.WithInsecure())
		if err != nil {
			log.Fatalf("faild to connect: %v", err)
		}

		return helloworld.NewGreeterClient(conn)
	})

	rpcxClient, err := gcp.GenerateClient(fmt.Sprintf("grpc@%s", lis.Addr().String()), "GreeterService", "SayHello")
	assert.NoError(t, err)
	assert.NotNil(t, rpcxClient)

	var argv = &helloworld.HelloRequest{
		Name: "smallnest",
	}
	var reply = &helloworld.HelloReply{}
	err = rpcxClient.Call(context.Background(), "GreeterService", "SayHello", argv, reply)
	assert.NoError(t, err)
	assert.Equal(t, "hello smallnest", reply.Message)

	//
	// rpcx client
	client.RegisterCacheClientBuilder("grpc", gcp)

	d, _ := client.NewPeer2PeerDiscovery("grpc@"+lis.Addr().String(), "")
	opt := client.DefaultOption
	xclient := client.NewXClient("GreeterService", client.Failtry, client.RandomSelect, d, opt)
	defer xclient.Close()

	argv = &helloworld.HelloRequest{
		Name: "smallnest",
	}
	reply = &helloworld.HelloReply{}
	err = xclient.Call(context.Background(), "SayHello", argv, reply)
	assert.NoError(t, err)
	assert.Equal(t, "hello smallnest", reply.Message)
}
