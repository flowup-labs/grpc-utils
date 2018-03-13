package runtime

import (
	"encoding/json"
	"io"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/golang/protobuf/proto"
	"bytes"
	"github.com/golang/protobuf/jsonpb"
)

// Marshaler is a configurable object for converting between
// protocol buffer objects and a JSON representation for them.
type JSONCustom struct {
	// Whether to render enum values as integers, as opposed to string values.
	EnumsAsInts bool

	// Whether to render fields with zero values.
	EmitDefaults bool

	// A string to indent each level by. The presence of this field will
	// also cause a space to appear between the field separator and
	// value, and for newlines to be appear between fields and array
	// elements.
	Indent string

	// Whether to use the original (.proto) name for fields.
	OrigName bool

	// A custom URL resolver to use when marshaling Any messages to JSON.
	// If unset, the default resolution strategy is to extract the
	// fully-qualified type name from the type URL and pass that to
	// proto.MessageType(string).
	AnyResolver AnyResolver

	// EmitDefaults can be override by removeEmpty
	// instead of omitEmpty is used removeEmpty
	// reason of this change is that some proto generators
	// generate omitEmpty everywhere
	RemoveEmpty bool
}

// ContentType always Returns "application/json".
func (*JSONCustom) ContentType() string {
	return "application/json"
}

// Marshal marshals "v" into JSON
func (j *JSONCustom) Marshal(v interface{}) ([]byte, error) {
	var w bytes.Buffer

	p, ok := v.(proto.Message)
	if !ok {
		return json.Marshal(v)
	}

	if err := (*Marshaler)(j).Marshal(&w, p); err != nil {
		return []byte{}, err
	}

	return w.Bytes(), nil
}

// Unmarshal unmarshals JSON data into "v".
func (j *JSONCustom) Unmarshal(data []byte, v interface{}) error {
	d := json.NewDecoder(bytes.NewReader(data))
	p, ok := v.(proto.Message)
	if !ok {
		return json.Unmarshal(data, v)
	}
	unmarshaler := &jsonpb.Unmarshaler{AllowUnknownFields: true}
	return unmarshaler.UnmarshalNext(d, p)
}

// NewDecoder returns a Decoder which reads JSON stream from "r".
func (j *JSONCustom) NewDecoder(r io.Reader) runtime.Decoder {
	return json.NewDecoder(r)
}

// NewEncoder returns an Encoder which writes JSON stream into "w".
func (j *JSONCustom) NewEncoder(w io.Writer) runtime.Encoder {
	return json.NewEncoder(w)
}

// Delimiter for newline encoded JSON streams.
func (j *JSONCustom) Delimiter() []byte {
	return []byte("\n")
}
