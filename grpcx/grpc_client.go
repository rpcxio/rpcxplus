package grpcx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/smallnest/rpcx/client"
	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
	"google.golang.org/grpc"
)

// Errors for grpc client.
var (
	ErrNotSupported              = errors.New("feature not supported")
	ErrClientNotRegistered       = errors.New("grpc client not registered")
	ErrGrpcMethodNotFound        = errors.New("grpc method not found")
	ErrReplyMustBePointer        = errors.New("reply must be pointer type")
	ErrGrpcClientBuilderNotFound = errors.New("grpc client builder not found")
	ErrGrpcReplyCannotSet        = errors.New("grpc reply can not be set")
)

// GrpcClientPlugin is used for managing rpcx clients for grpc protocol.
type GrpcClientPlugin struct {
	clientMapMu sync.RWMutex
	clientMap   map[string]*GrpcClient
	dialOpts    []grpc.DialOption
	callOpts    []grpc.CallOption
}

// NewGrpcClientPlugin creates a new GrpcClientPlugin.
func NewGrpcClientPlugin(dialOpts []grpc.DialOption, callOpts []grpc.CallOption) *GrpcClientPlugin {
	return &GrpcClientPlugin{
		clientMap: make(map[string]*GrpcClient),
		dialOpts:  dialOpts,
		callOpts:  callOpts,
	}
}

// SetCachedClient sets the cache client.
func (c *GrpcClientPlugin) SetCachedClient(client client.RPCClient, k, servicePath, serviceMethod string) {

}

// FindCachedClient gets a cached client if exist.
func (c *GrpcClientPlugin) FindCachedClient(k, servicePath, serviceMethod string) client.RPCClient {
	c.clientMapMu.RLock()
	defer c.clientMapMu.RUnlock()
	client, ok := c.clientMap[servicePath]
	if !ok {
		return nil
	}

	return client
}

// DeleteCachedClient deletes an exited client.
func (c *GrpcClientPlugin) DeleteCachedClient(client client.RPCClient, k, servicePath, serviceMethod string) {
	c.clientMapMu.Lock()
	defer c.clientMapMu.Unlock()
	cc := c.clientMap[servicePath]
	if cc != nil {
		cc.Close()
		delete(c.clientMap, servicePath)
	}
}

// GenerateClient generates an new grpc client.
func (c *GrpcClientPlugin) GenerateClient(k, servicePath, serviceMethod string) (client client.RPCClient, err error) {
	_, addr := splitNetworkAndAddress(k)

	conn, err := grpc.Dial(addr, c.dialOpts...)
	if err != nil {
		return nil, err
	}

	c.clientMap[servicePath] = &GrpcClient{
		client:     conn,
		callOpts:   c.callOpts,
		remoteAddr: addr,
	}
	return c.clientMap[servicePath], err
}

func splitNetworkAndAddress(server string) (string, string) {
	ss := strings.SplitN(server, "@", 2)
	if len(ss) == 1 {
		return "tcp", server
	}

	return ss[0], ss[1]
}

// GrpcClient is a grpc client wrapper and implements RPCClient interface.
type GrpcClient struct {
	client     *grpc.ClientConn // client wrapper
	callOpts   []grpc.CallOption
	remoteAddr string
	closed     bool
}

// Connect connects the server.
func (c *GrpcClient) Connect(network, address string) error {
	return ErrNotSupported
}

// Go calls the grpc servics asynchronizously.
func (c *GrpcClient) Go(ctx context.Context, servicePath, serviceMethod string, args interface{}, reply interface{}, done chan *client.Call) *client.Call {
	if done == nil {
		done = make(chan *client.Call, 10)
	}

	call := new(client.Call)
	call.ServicePath = servicePath
	call.ServiceMethod = serviceMethod
	meta := ctx.Value(share.ReqMetaDataKey)
	if meta != nil { // copy meta in context to meta in requests
		call.Metadata = meta.(map[string]string)
	}

	if _, ok := ctx.(*share.Context); !ok {
		ctx = share.NewContext(ctx)
	}

	call.Args = args
	call.Reply = reply
	call.Done = done

	go func() {
		err := c.Call(ctx, servicePath, serviceMethod, args, reply)
		call.Error = err
		close(call.Done)
	}()

	return call
}

// Call invoke the grpc sevice.
func (c *GrpcClient) Call(ctx context.Context, servicePath, serviceMethod string, argv interface{}, reply interface{}) error {
	cc := c.client
	if cc == nil {
		return ErrClientNotRegistered
	}

	err := grpc.Invoke(ctx, fmt.Sprintf("/%s/%s", servicePath, serviceMethod), argv, reply, cc, c.callOpts...)
	return err
}

// SendRaw sends raw data.
func (c *GrpcClient) SendRaw(ctx context.Context, r *protocol.Message) (map[string]string, []byte, error) {
	return nil, nil, ErrNotSupported
}

// Close record this client closed.
func (c *GrpcClient) Close() error {
	c.closed = true
	return nil
	//return c.clientConn.Close()
}

// RemoteAddr returns the remote address.
func (c *GrpcClient) RemoteAddr() string {
	return c.remoteAddr
}

// RegisterServerMessageChan register stream chan.
func (c *GrpcClient) RegisterServerMessageChan(ch chan<- *protocol.Message) {
	// not supported
}

// UnregisterServerMessageChan unregister stream chan.
func (c *GrpcClient) UnregisterServerMessageChan() {
	// not supported
}

// IsClosing return closed or not.
func (c *GrpcClient) IsClosing() bool {
	return c.closed
}

// IsShutdown return closed or not.
func (c *GrpcClient) IsShutdown() bool {
	return c.closed
}

// GetConn returns underlying net.Conn.
// Always returns nil.
func (c *GrpcClient) GetConn() net.Conn {
	return nil
}
