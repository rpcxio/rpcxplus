package grpcx

import (
	"context"
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/smallnest/rpcx/client"
	"github.com/smallnest/rpcx/log"
	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
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
	clientMapMu    sync.RWMutex
	clientMap      map[string]*GrpcClient
	clientConnMap  map[string]interface{}
	clientBuilders map[string]func(string) interface{}
}

// NewGrpcClientPlugin creates a new GrpcClientPlugin.
func NewGrpcClientPlugin() *GrpcClientPlugin {
	return &GrpcClientPlugin{
		clientMap:      make(map[string]*GrpcClient),
		clientConnMap:  make(map[string]interface{}),
		clientBuilders: make(map[string]func(string) interface{}),
	}
}

// SetCachedClient sets the cache client.
func (c *GrpcClientPlugin) SetCachedClient(client client.RPCClient, k, servicePath, serviceMethod string) {

}

// FindCachedClient gets a cached client if exist.
func (c *GrpcClientPlugin) FindCachedClient(k, servicePath, serviceMethod string) client.RPCClient {
	c.clientMapMu.RLock()
	defer c.clientMapMu.RUnlock()
	return c.clientMap[servicePath]
}

// DeleteCachedClient deletes an exited client.
func (c *GrpcClientPlugin) DeleteCachedClient(client client.RPCClient, k, servicePath, serviceMethod string) {
	c.clientMapMu.Lock()
	defer c.clientMapMu.Unlock()
	gc := c.clientMap[servicePath]
	if gc != nil {
		gc.Close()
		delete(c.clientMap, servicePath)
	}

	ccm := c.clientConnMap[servicePath]
	if ccm != nil {
		delete(c.clientConnMap, servicePath)
		if gconn, ok := ccm.(io.Closer); ok {
			gconn.Close()
		}
	}
}

// GenerateClient generates an new grpc client.
func (c *GrpcClientPlugin) GenerateClient(k, servicePath, serviceMethod string) (client client.RPCClient, err error) {
	_, addr := splitNetworkAndAddress(k)

	builder := c.clientBuilders[servicePath]
	if builder == nil {
		return nil, ErrGrpcClientBuilderNotFound
	}

	rcvr := builder(addr)
	c.clientConnMap[servicePath] = rcvr
	_, err = c.register(rcvr, addr, servicePath)
	return c.clientMap[servicePath], err
}

func splitNetworkAndAddress(server string) (string, string) {
	ss := strings.SplitN(server, "@", 2)
	if len(ss) == 1 {
		return "tcp", server
	}

	return ss[0], ss[1]
}

func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return isExported(t.Name()) || t.PkgPath() == ""
}

// Register adds grpc clients into rpcx client.
func (c *GrpcClientPlugin) Register(servicePath string, builder func(string) interface{}) {
	c.clientMapMu.Lock()
	defer c.clientMapMu.Unlock()

	c.clientBuilders[servicePath] = builder
}

func (c *GrpcClientPlugin) register(rcvr interface{}, addr, name string) (string, error) {
	c.clientMapMu.Lock()
	defer c.clientMapMu.Unlock()

	gc := new(grpcClient)
	gc.typ = reflect.TypeOf(rcvr)
	gc.rcvr = reflect.ValueOf(rcvr)
	sname := reflect.Indirect(gc.rcvr).Type().Name() // Type
	if name != "" {
		sname = name
	}
	if sname == "" {
		errorStr := "grpcx.Register: no client name for type " + gc.typ.String()
		log.Error(errorStr)
		return sname, errors.New(errorStr)
	}
	gc.name = sname

	// Install the methods
	gc.method = suitableMethods(gc.typ, true)
	if len(gc.method) == 0 {
		var errorStr string

		// To help the user, see if a pointer receiver would work.
		method := suitableMethods(reflect.PtrTo(gc.typ), true)
		if len(method) != 0 {
			errorStr = "grpcx.Register: type " + sname + " has no exported methods of suitable type (hint: pass a pointer to value of that type)"
		} else {
			errorStr = "grpcx.Register: type " + sname + " has no exported methods of suitable type"
		}
		log.Error(errorStr)
		return sname, errors.New(errorStr)
	}

	c.clientMap[gc.name] = &GrpcClient{client: gc, remoteAddr: addr}
	return sname, nil
}

// suitableMethods returns suitable Rpc methods of typ, it will report
// error using log if reportErr is true.
func suitableMethods(typ reflect.Type, reportErr bool) map[string]*methodType {
	methods := make(map[string]*methodType)
	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)
		mtype := method.Type
		mname := method.Name
		// Method must be exported.
		if method.PkgPath != "" {
			continue
		}
		// Method needs four ins: receiver, context.Context, *args, *reply.
		if mtype.NumIn() < 3 {
			if reportErr {
				log.Debug("method ", mname, " has wrong number of ins:", mtype.NumIn())
			}
			continue
		}
		// First arg must be context.Context
		ctxType := mtype.In(1)
		if !ctxType.Implements(typeOfContext) {
			if reportErr {
				log.Debug("method ", mname, " must use context.Context as the first parameter")
			}
			continue
		}

		// Second arg need not be a pointer.
		argType := mtype.In(2)

		if !argType.Implements(typeOfPB) {
			if reportErr {
				log.Debug("method ", mname, " must implement pb.Message as the request parameter")
			}
			continue
		}

		// Method needs two out.
		if mtype.NumOut() != 2 {
			if reportErr {
				log.Info("method", mname, " has wrong number of outs:", mtype.NumOut())
			}
			continue
		}

		replyType := mtype.Out(0)
		if !argType.Implements(typeOfPB) {
			if reportErr {
				log.Debug("method ", mname, "must implement pb.Message as the response parameter")
			}
			continue
		}

		// The return type of the method must be error.
		if returnType := mtype.Out(1); returnType != typeOfError {
			if reportErr {
				log.Info("method", mname, " returns ", returnType.String(), " not error")
			}
			continue
		}
		methods[mname] = &methodType{method: method, ArgType: argType, ReplyType: replyType}
	}
	return methods
}

// GrpcClient is a grpc client wrapper and implements RPCClient interface.
type GrpcClient struct {
	client     *grpcClient // client wrapper
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
	gc := c.client
	if gc == nil {
		return ErrClientNotRegistered
	}

	mtype := gc.method[serviceMethod]
	if mtype == nil {
		return ErrGrpcMethodNotFound
	}

	var argvValue reflect.Value
	if mtype.ArgType.Kind() != reflect.Ptr {
		argvValue = reflect.ValueOf(argv).Elem()
	} else {
		argvValue = reflect.ValueOf(argv)
	}

	if mtype.ReplyType.Kind() != reflect.Ptr {
		return ErrReplyMustBePointer
	}

	replyType := reflect.TypeOf(reply)
	if replyType.Kind() != reflect.Ptr {
		return ErrReplyMustBePointer
	}

	replyValue := reflect.ValueOf(reply)
	if replyValue.CanSet() {
		return ErrReplyMustBePointer
	}

	replyv, err := gc.call(ctx, mtype, argvValue)
	if err != nil {
		return err
	}
	replyValue.Elem().Set(replyv.Elem())
	return nil
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
