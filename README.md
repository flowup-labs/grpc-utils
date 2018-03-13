runtime
------
### JSON marshal
JSON marshal  is a combination of  [golang jsonbp](https://github.com/golang/protobuf/blob/master/jsonpb/jsonpb.go) and [JSONBuiltin](https://github.com/grpc-ecosystem/grpc-gateway/blob/master/runtime/marshal_json.go) marshaler.
This marshaler **don't ignore** json tags as key name (`json:"myName"`) or ignored (`json:"-"`)

#### RemoveEmpty
`RemoveEmpty` overrides `EmitDefaults` option.
It can be used to remove `json:"something,omitempty"` tags generated
by some generators
##### usage:

in main file
```go
mux := runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONCustom{RemoveEmpty:true}))
```

and in struct
```go
type user struct{
    name string `json:"NAME,removeEmpty"`
}
```

context
------
IncomingToOutgoingHeaderKey transforms incoming gRPC context into outgoing.
 It can be used when proxying gRPC requests.

 ##### usage:
 ```go
someClient.someMethod(IncomingToOutgoingContext(ctx), &someMsg{})
```
