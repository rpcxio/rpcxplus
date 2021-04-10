package grpcx

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"sync"

	"github.com/smallnest/rpcx/log"
)

type PbMessage interface {
	ProtoMessage()
}

// Precompute the reflect type for error. Can't use error directly
// because Typeof takes an empty interface value. This is annoying.
var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

// Precompute the reflect type for context.
var typeOfContext = reflect.TypeOf((*context.Context)(nil)).Elem()

// Precompute the reflect type for PB.
var typeOfPB = reflect.TypeOf((*PbMessage)(nil)).Elem()

type methodType struct {
	sync.Mutex // protects counters
	method     reflect.Method
	ArgType    reflect.Type
	ReplyType  reflect.Type
	// numCalls   uint
}

type grpcClient struct {
	name   string                 // name of service
	rcvr   reflect.Value          // receiver of methods for the service
	typ    reflect.Type           // type of the receiver
	method map[string]*methodType // registered methods
}

func (c *grpcClient) call(ctx context.Context, mtype *methodType, argv reflect.Value) (replyv reflect.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]

			err = fmt.Errorf("[service internal error]: %v, method: %s, argv: %+v",
				r, mtype.method.Name, argv.Interface())

			err2 := fmt.Errorf("[service internal error]: %v, method: %s, argv: %+v, stack: %s",
				r, mtype.method.Name, argv.Interface(), buf)
			log.Handle(err2)
		}
	}()

	function := mtype.method.Func
	// Invoke the method, providing a new value for the reply.
	returnValues := function.Call([]reflect.Value{c.rcvr, reflect.ValueOf(ctx), argv})

	errInter := returnValues[1].Interface()
	if errInter != nil {
		return returnValues[0], errInter.(error)
	}

	return returnValues[0], nil
}
