package pbutils

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// MustMarshalAny marshal given data or panic.
func MustMarshalAny(pb interface{}) *anypb.Any {
	msg, err := anypb.New(pb.(proto.Message))
	if err != nil {
		panic(err)
	}
	return msg
}
