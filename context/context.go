package context

import (
	"golang.org/x/net/context"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"google.golang.org/grpc/metadata"
)

// The most used headers keys
const (
	Authorization = "authorization"
)

// IncomingToOutgoingHeaderKey transforms incoming gRPC context into outgoing.
// It can be used when proxying gRPC requests.
//
// usage:
// someClient.someMethod(IncomingToOutgoingContext(ctx), &someMsg{})
//
func IncomingToOutgoingHeaderKey(ctx context.Context, key string) context.Context {
	return metautils.
		NiceMD(metadata.Pairs(key, metautils.ExtractIncoming(ctx).
		Get(key))).
		ToOutgoing(context.Background())
}
