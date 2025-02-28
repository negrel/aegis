package pbutils

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// MustMarshalAny marshal given data or panic.
func MustMarshalAny(pb interface{}) *anypb.Any {
	msg, err := anypb.New(pb.(proto.Message))
	if err != nil {
		panic(err)
	}
	return msg
}

// MustMarshalValueMap marshal given data or panic.
func MustMarshalValueMap(pb map[string]any) map[string]*structpb.Value {
	s := make(map[string]*structpb.Value)

	var err error
	for k, v := range pb {
		s[k], err = structpb.NewValue(v)
		if err != nil {
			panic(err)
		}
	}

	return s
}
