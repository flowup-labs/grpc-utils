runtime
------
### JSON marshal
JSON marshal  is a combination of  [golang jsonbp](https://github.com/golang/protobuf/blob/master/jsonpb/jsonpb.go) and [JSONBuiltin](https://github.com/grpc-ecosystem/grpc-gateway/blob/master/runtime/marshal_json.go) marshaler.
This marshaler **don't ignore** json tags as key name (`json:"myName"`) or ignored (`json:"-"`)

:warning: best for working is used import `gogoproto` in proto file
```proto
import "github.com/gogo/protobuf/gogoproto/gogo.proto";
```

#### RemoveEmpty
`RemoveEmpty` overrides `EmitDefaults` option.
It can be used to remove `json:"something,omitempty"` tags generated
by some generators
##### usage:

in main file
```go
mux := runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONCustom{RemoveEmpty:true}))
```

proto file
```proto
message User{
    name string = 1 [(gogoproto.jsontag) = "NAME,removeEmpty"];
}
```

#### Nested
This feature is used fully if you need nested output but in same time you want inside flat structure (for etc: save to database in one table)

:warning: is working only with structure generated from proto

##### Marshaler:
###### usage:
proto file
```proto
message Msg {
    Firstname string = 1 [(gogoproto.jsontag) = "name.firstname"];
    PseudoFirstname string = 2 [(gogoproto.jsontag) = "lastname"];
    EmbedMsg = 3;
    Lastname string = 4 [(gogoproto.jsontag) = "name.lastname"];
    Inside string  = 5 [(gogoproto.jsontag) = "name.inside.a.b.c"];
}

message EmbedMsg{
    Opt1 string = 1 [(gogoproto.jsontag) = "opt1"];
}
```

in your gocode
```go
msgInput := Msg{
    Firstname:       "One",
	Lastname:        "Two",
	PseudoFirstname: "Three",
	EmbedMsg: EmbedMsg{
		Opt1: "var",
	},
	Inside: "goo",
}
```

output
```json
{
	"lastname": "Three",
	"name": {
		"firstname": "One",
		"inside": {
			"a": {
				"b": {
					"c": "goo"
				}
			}
		},
		"lastname": "Two"
	},
	"opt1": "var"
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
