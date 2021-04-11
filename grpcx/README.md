## Server

服务端Grpc服务的实现，支持各种语言的grpc client访问。

### 1、 创建一个grpc server插件

创建一个grpc server插件，并把它加入到rpcx server的插件容器中。

```go
	s := server.NewServer() // rpcx server
	gs := NewGrpcServer() // grpc server plugin
	s.Plugins.Add(gs) // add  grpc server plugin into plugins of rpcx
```

### 2、注册 rpcx 服务和 grpc 服务

在这个例子中,`GreeterService`既实现了rpcx的服务方法`Greet`,也实现了grpc服务`helloworld.GreeterServer`。

* 2.1 注册rpcx的代码如下:

```go
err := s.Register(greetService, "")
```

* 2.2 注册grpc服务如下:

```go
gs.RegisterService(func(grpcServer *grpc.Server) {
		helloworld.RegisterGreeterServer(grpcServer, greetService)
})
```

### 3. 启动rpcx服务

```go
go s.Serve("tcp", "127.0.0.1:0")
```


### 4. 启动grpc server 插件

```go
err := gs.Start()
```

至此，服务端启动完成


### grpc client

rpcx的客户端不变， grpc客户端可以使用任何你喜欢的语言去实现都可以。

比如使用Go语言访问这个服务:

```go
    conn, err := grpc.Dial(s.Address().String(), grpc.WithInsecure(), grpc.WithTimeout(time.Second))
	
	defer conn.Close()
	c := helloworld.NewGreeterClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.SayHello(ctx, &helloworld.HelloRequest{Name: "smallnest"})
```

## 客户端

rpcx client支持访问各种编程语言实现的grpc服务，并能提供FailMode、熔断服务治理能力。

### 客户端注册Grpc支持能力

```go
	// register CacheClientBuilder
	gcp := NewGrpcClientPlugin([]grpc.DialOption{grpc.WithInsecure()}, nil)
	client.RegisterCacheClientBuilder("grpc", gcp)
```


### 访问grpc服务

和访问rpcx服务一模一样，没有特殊的配置。

```go
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
```