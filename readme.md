Set mock responses for grpc clients (connected to localhost:50000)

To start -

go run *.go

To set (http server runs at :51000) - 

format - 

{
    "path":"string",
    "request":{"key":{"pos":1,"val":42,"typ":string"}},
    "response":{"key":{"pos":1,"val":42,"typ":string"}}
}

eg. -

curl -XGET http://localhost:51000/set -d '{"path":"helloworld.Greeter/SayHello","request":{"name":{"pos":1,"val":"hohoho","typ":"string"}},"response":{"message":{"pos":1,"val":"merry xmas1233","typ":"string"}}}'