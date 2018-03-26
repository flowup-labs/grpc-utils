package runtime

// Go support for Protocol Buffers - Google's data interchange format
//
// Copyright 2015 The Go Authors.  All rights reserved.
// https://github.com/golang/protobuf
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//     * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//     * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//     * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

/*
Package jsonpb provides marshaling and unmarshaling between protocol buffers and JSON.
It follows the specification at https://developers.google.com/protocol-buffers/docs/proto3#json.

This package produces a different output than the standard "encoding/json" package,
which does not operate correctly on protocol buffers.
*/

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"

	stpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
)

type Properties struct {
	origProp *proto.Properties
	Embedded bool
	StdTime  bool
}

// Marshaler is a configurable object for converting between
// protocol buffer objects and a JSON representation for them.
type Marshaler struct {
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

	// RemoveEmpty overrides EmitDefaults option.
	// It can be used to remove `json:"something,omitempty"` tags generated
	// by some generators
	RemoveEmpty bool
}

// AnyResolver takes a type URL, present in an Any message, and resolves it into
// an instance of the associated message.
type AnyResolver interface {
	Resolve(typeUrl string) (proto.Message, error)
}

func defaultResolveAny(typeUrl string) (proto.Message, error) {
	// Only the part of typeUrl after the last slash is relevant.
	mname := typeUrl
	if slash := strings.LastIndex(mname, "/"); slash >= 0 {
		mname = mname[slash+1:]
	}
	mt := proto.MessageType(mname)
	if mt == nil {
		return nil, fmt.Errorf("unknown message type %q", mname)
	}
	return reflect.New(mt.Elem()).Interface().(proto.Message), nil
}

// JSONPBMarshaler is implemented by protobuf messages that customize the
// way they are marshaled to JSON. Messages that implement this should
// also implement JSONPBUnmarshaler so that the custom format can be
// parsed.
type JSONPBMarshaler interface {
	MarshalJSONPB(*Marshaler) ([]byte, error)
}

// JSONPBUnmarshaler is implemented by protobuf messages that customize
// the way they are unmarshaled from JSON. Messages that implement this
// should also implement JSONPBMarshaler so that the custom format can be
// produced.
type JSONPBUnmarshaler interface {
	UnmarshalJSONPB(*Unmarshaler, []byte) error
}

// Marshal marshals a protocol buffer into JSON.
func (m *Marshaler) Marshal(out io.Writer, pb proto.Message) error {
	writer := &errWriter{writer: out}
	builder := &Builder{}
	builder.Output = make(map[string]interface{})

	if err := m.marshalObject(pb, "", "", builder); err != nil {
		return err
	}

	b, err := json.Marshal(builder.Output)
	if err != nil {
		return err
	}

	writer.write(string(b))
	return nil
}

// MarshalToString converts a protocol buffer object to JSON string.
func (m *Marshaler) MarshalToString(pb proto.Message) (string, error) {
	var buf bytes.Buffer
	if err := m.Marshal(&buf, pb); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type int32Slice []int32

var nonFinite = map[string]float64{
	`"NaN"`:       math.NaN(),
	`"Infinity"`:  math.Inf(1),
	`"-Infinity"`: math.Inf(-1),
}

// For sorting extensions ids to ensure stable output.
func (s int32Slice) Len() int           { return len(s) }
func (s int32Slice) Less(i, j int) bool { return s[i] < s[j] }
func (s int32Slice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type wkt interface {
	XXX_WellKnownType() string
}

// marshalObject writes a struct to the Writer.
func (m *Marshaler) marshalObject(v proto.Message, nested, typeURL string, builder *Builder) error {
	if jsm, ok := v.(JSONPBMarshaler); ok {
		b, err := jsm.MarshalJSONPB(m)
		if err != nil {
			return err
		}
		if typeURL != "" {
			// we are marshaling this object to an Any type
			var js map[string]*json.RawMessage
			if err = json.Unmarshal(b, &js); err != nil {
				return fmt.Errorf("type %T produced invalid JSON: %v", v, err)
			}
			turl, err := json.Marshal(typeURL)
			if err != nil {
				return fmt.Errorf("failed to marshal type URL %q to JSON: %v", typeURL, err)
			}
			js["@type"] = (*json.RawMessage)(&turl)
			if b, err = json.Marshal(js); err != nil {
				return err
			}
		}
		// out.write(string(b))
		return nil
	}

	s := reflect.ValueOf(v).Elem()

	// Handle well-known types.
	if wkt, ok := v.(wkt); ok {

		switch wkt.XXX_WellKnownType() {
		case "DoubleValue", "FloatValue", "Int64Value", "UInt64Value",
			"Int32Value", "UInt32Value", "BoolValue", "StringValue", "BytesValue":
			// "Wrappers use the same representation in JSON
			//  as the wrapped primitive type, ..."
			sprop := proto.GetProperties(s.Type())
			_, err := m.marshalValue(&Properties{origProp: sprop.Prop[0]}, s.Field(0), builder)
			return err
		case "Any":
			// Any is a bit more involved.
			return m.marshalAny(v, nested, builder)
		case "Duration":
			// "Generated output always contains 3, 6, or 9 fractional digits,
			//  depending on required precision."
			s, ns := s.Field(0).Int(), s.Field(1).Int()
			d := time.Duration(s)*time.Second + time.Duration(ns)*time.Nanosecond
			x := fmt.Sprintf("%.9f", d.Seconds())
			x = strings.TrimSuffix(x, "000")
			x = strings.TrimSuffix(x, "000")
			// out.write(`"`)
			// out.write(x)
			// out.write(`s"`)
			return nil
		case "Struct", "ListValue":
			// Let marshalValue handle the `Struct.fields` map or the `ListValue.values` slice.
			// TODO: pass the correct Properties if needed.
			_, err := m.marshalValue(&Properties{}, s.Field(0), builder)
			return err
		case "Timestamp":
			// "RFC 3339, where generated output will always be Z-normalized
			//  and uses 3, 6 or 9 fractional digits."
			s, ns := s.Field(0).Int(), s.Field(1).Int()
			t := time.Unix(s, ns).UTC()
			// time.RFC3339Nano isn't exactly right (we need to get 3/6/9 fractional digits).
			x := t.Format("2006-01-02T15:04:05.000000000")
			x = strings.TrimSuffix(x, "000")
			x = strings.TrimSuffix(x, "000")
			// out.write(`"`)
			// out.write(x)
			// out.write(`Z"`)
			return nil
		case "Value":
			// Value has a single oneof.
			kind := s.Field(0)
			if kind.IsNil() {
				// "absence of any variant indicates an error"
				return errors.New("nil Value")
			}
			// oneof -> *T -> T -> T.F
			x := kind.Elem().Elem().Field(0)
			// TODO: pass the correct Properties if needed.
			_, err := m.marshalValue(&Properties{}, x, builder)
			return err
		}
	}

	// out.write("{")
	if m.Indent != "" {
		// out.write("\n")
	}

	firstField := true

	if typeURL != "" {

		if err := m.marshalTypeURL(nested, typeURL); err != nil {
			return err
		}
		firstField = false
	}

fieldLoop:
	for i := 0; i < s.NumField(); i++ {
		value := s.Field(i)
		valueField := s.Type().Field(i)
		if strings.HasPrefix(valueField.Name, "XXX_") {
			continue
		}

		// IsNil will panic on most value kinds.
		switch value.Kind() {
		case reflect.Chan, reflect.Func, reflect.Interface:
			if value.IsNil() {
				continue
			}
		}

		if !m.EmitDefaults {
			switch value.Kind() {
			case reflect.Bool:
				if !value.Bool() {
					continue
				}
			case reflect.Int32, reflect.Int64:
				if value.Int() == 0 {
					continue
				}
			case reflect.Uint32, reflect.Uint64:
				if value.Uint() == 0 {
					continue
				}
			case reflect.Float32, reflect.Float64:
				if value.Float() == 0 {
					continue
				}
			case reflect.String:
				if value.Len() == 0 {
					continue
				}
			case reflect.Map, reflect.Ptr, reflect.Slice:
				if value.IsNil() {
					continue
				}
			}
		}

		// Oneof fields need special handling.
		if valueField.Tag.Get("protobuf_oneof") != "" {
			// value is an interface containing &T{real_value}.
			sv := value.Elem().Elem() // interface -> *T -> T
			value = sv.Field(0)
			valueField = sv.Type().Field(0)
		}
		prop := jsonProperties(valueField, m.OrigName)

		// Get json tag if is exist
		if tag, ok := valueField.Tag.Lookup("json"); ok {
			switch tag {
			// ignore field
			case "-":
				continue
			default:
				tags := strings.Split(tag, ",")
				// use json tag as json name
				if len(tags) > 0 {
					prop.origProp.JSONName = tags[0]
				}

				for i := 1; i < len(tags); i++ {
					if m.RemoveEmpty && tags[i] == "removeEmpty" && value.String() == "" {
						continue fieldLoop
					}
				}
			}
		}

		if !firstField {
			// m.writeSep(out)
		}

		if err := m.marshalField(prop, value, nested, builder, valueField.Anonymous); err != nil {
			return err
		}
		firstField = false
	}

	// Handle proto2 extensions.
	if ep, ok := v.(proto.Message); ok {
		extensions := proto.RegisteredExtensions(v)
		// Sort extensions for stable output.
		ids := make([]int32, 0, len(extensions))
		for id, desc := range extensions {
			if !proto.HasExtension(ep, desc) {
				continue
			}
			ids = append(ids, id)
		}
		sort.Sort(int32Slice(ids))
		for _, id := range ids {
			desc := extensions[id]
			if desc == nil {
				// unknown extension
				continue
			}
			ext, extErr := proto.GetExtension(ep, desc)
			if extErr != nil {
				return extErr
			}
			value := reflect.ValueOf(ext)
			prop := Properties{origProp: &proto.Properties{}}
			prop.origProp.Parse(desc.Tag)
			prop.origProp.JSONName = fmt.Sprintf("[%s]", desc.Name)
			if !firstField {
				// m.writeSep(out)
			}
			if err := m.marshalField(&prop, value, nested, builder, false); err != nil {
				return err
			}
			firstField = false
		}

	}

	if m.Indent != "" {
		// out.write("\n")
		// out.write(indent)
	}
	// out.write("}")

	return nil
}

func (m *Marshaler) writeSep(out *errWriter) {
	if m.Indent != "" {
		// out.write(",\n")
	} else {
		// out.write(",")
	}
}

func (m *Marshaler) marshalAny(any proto.Message, indent string, builder *Builder) error {
	// "If the Any contains a value that has a special JSON mapping,
	//  it will be converted as follows: {"@type": xxx, "value": yyy}.
	//  Otherwise, the value will be converted into a JSON object,
	//  and the "@type" field will be inserted to indicate the actual data type."
	v := reflect.ValueOf(any).Elem()
	turl := v.Field(0).String()
	val := v.Field(1).Bytes()

	var msg proto.Message
	var err error
	if m.AnyResolver != nil {
		msg, err = m.AnyResolver.Resolve(turl)
	} else {
		msg, err = defaultResolveAny(turl)
	}
	if err != nil {
		return err
	}

	if err := proto.Unmarshal(val, msg); err != nil {
		return err
	}

	if _, ok := msg.(wkt); ok {
		// out.write("{")
		if m.Indent != "" {
			// out.write("\n")
		}
		if err := m.marshalTypeURL(indent, turl); err != nil {
			return err
		}
		// m.writeSep(out)
		if m.Indent != "" {
			// out.write(indent)
			// out.write(m.Indent)
			// out.write(`"value": `)
		} else {
			// out.write(`"value":`)
		}
		if err := m.marshalObject(msg, indent+m.Indent, "", builder); err != nil {
			return err
		}
		if m.Indent != "" {
			// out.write("\n")
			// out.write(indent)
		}
		// out.write("}")
		return nil
	}
	return m.marshalObject(msg, indent, turl, builder)
}

func (m *Marshaler) marshalTypeURL(indent, typeURL string) error {
	if m.Indent != "" {
		// out.write(indent)
		// out.write(m.Indent)
	}
	// out.write(`"@type":`)
	if m.Indent != "" {
		// out.write(" ")
	}
	_, err := json.Marshal(typeURL)
	if err != nil {
		return err
	}
	// out.write(string(b))
	return nil
}

type Builder struct {
	Output map[string]interface{}
}

func (m *Marshaler) mapBuilder(y, len int, include []string, x map[string]interface{}, value interface{}) {
	if y > len {
		return
	}

	tag := include[y]

	if x[tag] == nil {
		x[tag] = make(map[string]interface{})
	}

	if _, ok := x[tag].(map[string]interface{}); ok {
		m.mapBuilder(y+1, len, include, x[tag].(map[string]interface{}), value)
	}

	if y == len {
		x[tag] = value
	}

}

// marshalField writes field description and value to the Writer.
func (m *Marshaler) marshalField(prop *Properties, v reflect.Value, nested string, builder *Builder, anonymous bool) error {

	value, err := m.marshalValue(prop, v, builder)
	if err != nil {
		return err
	}

	include := strings.Split(prop.origProp.JSONName, ".")
	if nested != "" {
		include = append([]string{nested}, include...)
	}

	if len(include) > 0 && !anonymous && value != nil {
		m.mapBuilder(0, len(include)-1, include, builder.Output, value)
	}

	return nil
}

// marshalValue writes the value to the Writer.
func (m *Marshaler) marshalValue(prop *Properties, v reflect.Value, builder *Builder) (interface{}, error) {
	v = reflect.Indirect(v)

	// Handle nil pointer
	if v.Kind() == reflect.Invalid {
		return nil, nil
	}

	// Handle nested messages.
	if v.Kind() == reflect.Struct && !prop.StdTime {
		nested := prop.origProp.JSONName
		if prop.Embedded {
			nested = ""
		}

		if prop.Embedded {
			return nil, m.marshalObject(v.Addr().Interface().(proto.Message), nested, "", builder)
		}

		newBuilder := &Builder{}
		newBuilder.Output = make(map[string]interface{})

		err := m.marshalObject(v.Addr().Interface().(proto.Message), nested, "", newBuilder)
		if err != nil {
			return nil, err
		}

		return newBuilder.Output[prop.origProp.OrigName], nil
	}

	// Handle repeated elements.
	if v.Kind() == reflect.Slice && v.Type().Elem().Kind() != reflect.Uint8 {
		// out.write("[")
		slice := []interface{}{}

		for i := 0; i < v.Len(); i++ {
			value, err := m.marshalValue(prop, v.Index(i), builder)
			if err != nil {
				return nil, err
			}

			slice = append(slice, value)
		}

		return slice, nil
	}

	// Handle enumerations.
	if !m.EnumsAsInts && prop.origProp.Enum != "" {
		// Unknown enum values will are stringified by the proto library as their
		// value. Such values should _not_ be quoted or they will be interpreted
		// as an enum string instead of their value.
		enumStr := v.Interface().(fmt.Stringer).String()

		return enumStr, nil
	}

	return v.Interface(), nil
}

// Unmarshaler is a configurable object for converting from a JSON
// representation to a protocol buffer object.
type Unmarshaler struct {
	// Whether to allow messages to contain unknown fields, as opposed to
	// failing to unmarshal.
	AllowUnknownFields bool

	// A custom URL resolver to use when unmarshaling Any messages from JSON.
	// If unset, the default resolution strategy is to extract the
	// fully-qualified type name from the type URL and pass that to
	// proto.MessageType(string).
	AnyResolver AnyResolver
}

// UnmarshalNext unmarshals the next protocol buffer from a JSON object stream.
// This function is lenient and will decode any options permutations of the
// related Marshaler.
func (u *Unmarshaler) UnmarshalNext(dec runtime.Decoder, pb proto.Message) error {
	inputValue := json.RawMessage{}
	if err := dec.Decode(&inputValue); err != nil {
		return err
	}
	return u.unmarshalValue(reflect.ValueOf(pb).Elem(), inputValue, nil)
}

// Unmarshal unmarshals a JSON object stream into a protocol
// buffer. This function is lenient and will decode any options
// permutations of the related Marshaler.
func (u *Unmarshaler) Unmarshal(r io.Reader, pb proto.Message) error {
	dec := json.NewDecoder(r)
	return u.UnmarshalNext(dec, pb)
}

// UnmarshalNext unmarshals the next protocol buffer from a JSON object stream.
// This function is lenient and will decode any options permutations of the
// related Marshaler.
func UnmarshalNext(dec *json.Decoder, pb proto.Message) error {
	return new(Unmarshaler).UnmarshalNext(dec, pb)
}

// Unmarshal unmarshals a JSON object stream into a protocol
// buffer. This function is lenient and will decode any options
// permutations of the related Marshaler.
func Unmarshal(r io.Reader, pb proto.Message) error {
	return new(Unmarshaler).Unmarshal(r, pb)
}

// UnmarshalString will populate the fields of a protocol buffer based
// on a JSON string. This function is lenient and will decode any options
// permutations of the related Marshaler.
func UnmarshalString(str string, pb proto.Message) error {
	return new(Unmarshaler).Unmarshal(strings.NewReader(str), pb)
}

// unmarshalValue converts/copies a value into the target.
// prop may be nil.
func (u *Unmarshaler) unmarshalValue(target reflect.Value, inputValue json.RawMessage, prop *Properties) error {
	targetType := target.Type()

	// Allocate memory for pointer fields.
	if targetType.Kind() == reflect.Ptr {
		// If input value is "null" and target is a pointer type, then the field should be treated as not set
		// UNLESS the target is structpb.Value, in which case it should be set to structpb.NullValue.
		_, isJSONPBUnmarshaler := target.Interface().(JSONPBUnmarshaler)
		if string(inputValue) == "null" && targetType != reflect.TypeOf(&stpb.Value{}) && !isJSONPBUnmarshaler {
			return nil
		}
		target.Set(reflect.New(targetType.Elem()))

		return u.unmarshalValue(target.Elem(), inputValue, prop)
	}

	if jsu, ok := target.Addr().Interface().(JSONPBUnmarshaler); ok {
		return jsu.UnmarshalJSONPB(u, []byte(inputValue))
	}

	// Handle well-known types that are not pointers.
	if w, ok := target.Addr().Interface().(wkt); ok {
		switch w.XXX_WellKnownType() {
		case "DoubleValue", "FloatValue", "Int64Value", "UInt64Value",
			"Int32Value", "UInt32Value", "BoolValue", "StringValue", "BytesValue":
			return u.unmarshalValue(target.Field(0), inputValue, prop)
		case "Any":
			// Use json.RawMessage pointer type instead of value to support pre-1.8 version.
			// 1.8 changed RawMessage.MarshalJSON from pointer type to value type, see
			// https://github.com/golang/go/issues/14493
			var jsonFields map[string]*json.RawMessage
			if err := json.Unmarshal(inputValue, &jsonFields); err != nil {
				return err
			}

			val, ok := jsonFields["@type"]
			if !ok || val == nil {
				return errors.New("Any JSON doesn't have '@type'")
			}

			var turl string
			if err := json.Unmarshal([]byte(*val), &turl); err != nil {
				return fmt.Errorf("can't unmarshal Any's '@type': %q", *val)
			}
			target.Field(0).SetString(turl)

			var m proto.Message
			var err error
			if u.AnyResolver != nil {
				m, err = u.AnyResolver.Resolve(turl)
			} else {
				m, err = defaultResolveAny(turl)
			}
			if err != nil {
				return err
			}

			if _, ok := m.(wkt); ok {
				val, ok := jsonFields["value"]
				if !ok {
					return errors.New("Any JSON doesn't have 'value'")
				}

				if err := u.unmarshalValue(reflect.ValueOf(m).Elem(), *val, nil); err != nil {
					return fmt.Errorf("can't unmarshal Any nested proto %T: %v", m, err)
				}
			} else {
				delete(jsonFields, "@type")
				nestedProto, err := json.Marshal(jsonFields)
				if err != nil {
					return fmt.Errorf("can't generate JSON for Any's nested proto to be unmarshaled: %v", err)
				}

				if err = u.unmarshalValue(reflect.ValueOf(m).Elem(), nestedProto, nil); err != nil {
					return fmt.Errorf("can't unmarshal Any nested proto %T: %v", m, err)
				}
			}

			b, err := proto.Marshal(m)
			if err != nil {
				return fmt.Errorf("can't marshal proto %T into Any.Value: %v", m, err)
			}
			target.Field(1).SetBytes(b)
			return nil
		case "Duration":
			unq, err := strconv.Unquote(string(inputValue))
			if err != nil {
				return err
			}

			d, err := time.ParseDuration(unq)
			if err != nil {
				return fmt.Errorf("bad Duration: %v", err)
			}

			ns := d.Nanoseconds()
			s := ns / 1e9
			ns %= 1e9
			target.Field(0).SetInt(s)
			target.Field(1).SetInt(ns)
			return nil
		case "Timestamp":
			unq, err := strconv.Unquote(string(inputValue))
			if err != nil {
				return err
			}

			t, err := time.Parse(time.RFC3339Nano, unq)
			if err != nil {
				return fmt.Errorf("bad Timestamp: %v", err)
			}

			target.Field(0).SetInt(t.Unix())
			target.Field(1).SetInt(int64(t.Nanosecond()))
			return nil
		case "Struct":
			var m map[string]json.RawMessage
			if err := json.Unmarshal(inputValue, &m); err != nil {
				return fmt.Errorf("bad StructValue: %v", err)
			}

			target.Field(0).Set(reflect.ValueOf(map[string]*stpb.Value{}))
			for k, jv := range m {
				pv := &stpb.Value{}
				if err := u.unmarshalValue(reflect.ValueOf(pv).Elem(), jv, prop); err != nil {
					return fmt.Errorf("bad value in StructValue for key %q: %v", k, err)
				}
				target.Field(0).SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(pv))
			}
			return nil
		case "ListValue":
			var s []json.RawMessage
			if err := json.Unmarshal(inputValue, &s); err != nil {
				return fmt.Errorf("bad ListValue: %v", err)
			}

			target.Field(0).Set(reflect.ValueOf(make([]*stpb.Value, len(s), len(s))))
			for i, sv := range s {
				if err := u.unmarshalValue(target.Field(0).Index(i), sv, prop); err != nil {
					return err
				}
			}
			return nil
		case "Value":
			ivStr := string(inputValue)
			if ivStr == "null" {
				target.Field(0).Set(reflect.ValueOf(&stpb.Value_NullValue{}))
			} else if v, err := strconv.ParseFloat(ivStr, 0); err == nil {
				target.Field(0).Set(reflect.ValueOf(&stpb.Value_NumberValue{v}))
			} else if v, err := strconv.Unquote(ivStr); err == nil {
				target.Field(0).Set(reflect.ValueOf(&stpb.Value_StringValue{v}))
			} else if v, err := strconv.ParseBool(ivStr); err == nil {
				target.Field(0).Set(reflect.ValueOf(&stpb.Value_BoolValue{v}))
			} else if err := json.Unmarshal(inputValue, &[]json.RawMessage{}); err == nil {
				lv := &stpb.ListValue{}
				target.Field(0).Set(reflect.ValueOf(&stpb.Value_ListValue{lv}))
				return u.unmarshalValue(reflect.ValueOf(lv).Elem(), inputValue, prop)
			} else if err := json.Unmarshal(inputValue, &map[string]json.RawMessage{}); err == nil {
				sv := &stpb.Struct{}
				target.Field(0).Set(reflect.ValueOf(&stpb.Value_StructValue{sv}))
				return u.unmarshalValue(reflect.ValueOf(sv).Elem(), inputValue, prop)
			} else {
				return fmt.Errorf("unrecognized type for Value %q", ivStr)
			}
			return nil
		}
	}

	// Handle enums, which have an underlying type of int32,
	// and may appear as strings.
	// The case of an enum appearing as a number is handled
	// at the bottom of this function.
	if inputValue[0] == '"' && prop != nil && prop.origProp != nil && prop.origProp.Enum != "" {

		vmap := proto.EnumValueMap(prop.origProp.Enum)
		// Don't need to do unquoting; valid enum names
		// are from a limited character set.
		s := inputValue[1 : len(inputValue)-1]
		n, ok := vmap[string(s)]
		if !ok {
			return fmt.Errorf("unknown value %q for enum %s", s, prop.origProp.Enum)
		}
		if target.Kind() == reflect.Ptr { // proto2
			target.Set(reflect.New(targetType.Elem()))
			target = target.Elem()
		}
		target.SetInt(int64(n))
		return nil
	}

	// Handle nested messages.
	if targetType.Kind() == reflect.Struct {
		var jsonFields map[string]json.RawMessage
		if err := json.Unmarshal(inputValue, &jsonFields); err != nil {
			return err
		}

		consumeField := func(prop *Properties) (json.RawMessage, bool) {
			// Be liberal in what names we accept; both orig_name and camelName are okay.
			fieldNames := acceptedJSONFieldNames(prop)

			vOrig, okOrig := jsonFields[fieldNames.orig]
			vCamel, okCamel := jsonFields[fieldNames.camel]
			if !okOrig && !okCamel {
				return nil, false
			}
			// If, for some reason, both are present in the data, favour the camelName.
			var raw json.RawMessage
			if okOrig {
				raw = vOrig
				delete(jsonFields, fieldNames.orig)
			}
			if okCamel {
				raw = vCamel
				delete(jsonFields, fieldNames.camel)
			}
			return raw, true
		}

		sprops := proto.GetProperties(targetType)
		for i := 0; i < target.NumField(); i++ {
			var valueForField json.RawMessage
			var override bool

			ft := target.Type().Field(i)
			if strings.HasPrefix(ft.Name, "XXX_") {
				continue
			}

			if tag, ok := ft.Tag.Lookup("protobuf"); ok {
				if target.Field(i).Kind() == reflect.Struct && strings.Contains(tag, "embedded") {
					if err := json.Unmarshal(inputValue, &valueForField); err != nil {
						return err
					}

					spropsR := proto.GetProperties(target.Field(i).Type())

					for y := 0; y < target.Field(i).NumField(); y++ {
						if err := u.unmarshalValue(target.Field(i), valueForField, &Properties{origProp: spropsR.Prop[y]}); err != nil {
							return err
						}
					}
				}
			}

			if tag, ok := ft.Tag.Lookup("json"); ok {
				switch tag {
				// ignore field
				case "-":
					continue
				default:
					tags := strings.Split(tag, ",")
					if len(tags) < 0 {
						break
					}

					include := strings.Split(tags[0], ".")
					if len(include) > 0 {

						fields := map[string]interface{}{}

						if err := json.Unmarshal(inputValue, &fields); err != nil {
							return err
						}

						value := u.mapBuilder(fields, 0, len(include)-1, include)

						if _, ok := value.(string); ok {
							valueForField = []byte(`"` + value.(string) + `"`)
							override = true
						} else if _, ok := value.(float64); ok {
							valueForField = []byte(strconv.FormatFloat(value.(float64), 'f', 0, 64))
							override = true
						}

					}
				}
			}

			if !override {
				var ok bool
				valueForField, ok = consumeField(&Properties{origProp: sprops.Prop[i]})
				if !ok {
					continue
				}
			}

			if err := u.unmarshalValue(target.Field(i), valueForField, &Properties{origProp: sprops.Prop[i]}); err != nil {
				return err
			}
		}
		// Check for any oneof fields.
		if len(jsonFields) > 0 {
			for _, oop := range sprops.OneofTypes {
				raw, ok := consumeField(&Properties{origProp: oop.Prop})
				if !ok {
					continue
				}
				nv := reflect.New(oop.Type.Elem())
				target.Field(oop.Field).Set(nv)
				if err := u.unmarshalValue(nv.Elem().Field(0), raw, &Properties{origProp: oop.Prop}); err != nil {
					return err
				}
			}
		}
		// Handle proto2 extensions.
		if len(jsonFields) > 0 {
			if ep, ok := target.Addr().Interface().(proto.Message); ok {
				for _, ext := range proto.RegisteredExtensions(ep) {
					name := fmt.Sprintf("[%s]", ext.Name)
					raw, ok := jsonFields[name]
					if !ok {
						continue
					}
					delete(jsonFields, name)
					nv := reflect.New(reflect.TypeOf(ext.ExtensionType).Elem())
					if err := u.unmarshalValue(nv.Elem(), raw, nil); err != nil {
						return err
					}
					if err := proto.SetExtension(ep, ext, nv.Interface()); err != nil {
						return err
					}
				}
			}
		}
		if !u.AllowUnknownFields && len(jsonFields) > 0 {
			// Pick any field to be the scapegoat.
			var f string
			for fname := range jsonFields {
				f = fname
				break
			}
			return fmt.Errorf("unknown field %q in %v", f, targetType)
		}
		return nil
	}

	// Handle arrays (which aren't encoded bytes)
	if targetType.Kind() == reflect.Slice && targetType.Elem().Kind() != reflect.Uint8 {

		var slc []json.RawMessage
		if err := json.Unmarshal(inputValue, &slc); err != nil {
			return err
		}
		if slc != nil {
			l := len(slc)
			target.Set(reflect.MakeSlice(targetType, l, l))
			for i := 0; i < l; i++ {
				if err := u.unmarshalValue(target.Index(i), slc[i], prop); err != nil {
					return err
				}
			}
		}
		return nil
	}

	// Handle maps (whose keys are always strings)
	if targetType.Kind() == reflect.Map {
		var mp map[string]json.RawMessage
		if err := json.Unmarshal(inputValue, &mp); err != nil {
			return err
		}
		if mp != nil {
			target.Set(reflect.MakeMap(targetType))
			var keyprop, valprop *Properties
			if prop != nil {
				// These could still be nil if the protobuf metadata is broken somehow.
				// TODO: This won't work because the fields are unexported.
				// We should probably just reparse them.
				// keyprop, valprop = prop.mkeyprop, prop.mvalprop
			}
			for ks, raw := range mp {
				// Unmarshal map key. The core json library already decoded the key into a
				// string, so we handle that specially. Other types were quoted post-serialization.
				var k reflect.Value
				if targetType.Key().Kind() == reflect.String {
					k = reflect.ValueOf(ks)
				} else {
					k = reflect.New(targetType.Key()).Elem()
					if err := u.unmarshalValue(k, json.RawMessage(ks), keyprop); err != nil {
						return err
					}
				}

				// Unmarshal map value.
				v := reflect.New(targetType.Elem()).Elem()
				if err := u.unmarshalValue(v, raw, valprop); err != nil {
					return err
				}
				target.SetMapIndex(k, v)
			}
		}
		return nil
	}

	// 64-bit integers can be encoded as strings. In this case we drop
	// the quotes and proceed as normal.
	isNum := targetType.Kind() == reflect.Int64 || targetType.Kind() == reflect.Uint64
	if isNum && strings.HasPrefix(string(inputValue), `"`) {
		inputValue = inputValue[1 : len(inputValue)-1]
	}

	// Non-finite numbers can be encoded as strings.
	isFloat := targetType.Kind() == reflect.Float32 || targetType.Kind() == reflect.Float64
	if isFloat {
		if num, ok := nonFinite[string(inputValue)]; ok {
			target.SetFloat(num)
			return nil
		}
	}

	// Use the encoding/json for parsing other value types.
	return json.Unmarshal(inputValue, target.Addr().Interface())
}

func (u *Unmarshaler) mapBuilder(x map[string]interface{}, y, len int, include []string) interface{} {
	if y > len {
		return nil
	}

	if _, ok := x[include[y]].(map[string]interface{}); ok {
		if value := u.mapBuilder(x[include[y]].(map[string]interface{}), y+1, len, include); value != nil {
			return value
		}
	}

	if y == len {
		return x[include[y]]
	}

	return nil
}

// jsonProperties returns parsed Properties for the field and corrects JSONName attribute.
func jsonProperties(f reflect.StructField, origName bool) *Properties {
	origProp := proto.Properties{}
	origProp.Init(f.Type, f.Name, f.Tag.Get("protobuf"), &f)
	prop := Properties{origProp: &origProp}

	if strings.Contains(f.Tag.Get("protobuf"), "embedded=") {
		prop.Embedded = true
	}

	if strings.Contains(f.Tag.Get("protobuf"), "stdtime") {
		prop.StdTime = true
	}

	if origName || prop.origProp.JSONName == "" {
		prop.origProp.JSONName = prop.origProp.OrigName
	}
	return &prop
}

type fieldNames struct {
	orig, camel string
}

func acceptedJSONFieldNames(prop *Properties) fieldNames {
	opts := fieldNames{orig: prop.origProp.OrigName, camel: prop.origProp.OrigName}
	if prop.origProp.JSONName != "" {
		opts.camel = prop.origProp.JSONName
	}
	return opts
}

// Writer wrapper inspired by https://blog.golang.org/errors-are-values
type errWriter struct {
	writer io.Writer
	err    error
}

func (w *errWriter) write(str string) {
	if w.err != nil {
		return
	}
	_, w.err = w.writer.Write([]byte(str))
}

// Map fields may have key types of non-float scalars, strings and enums.
// The easiest way to sort them in some deterministic order is to use fmt.
// If this turns out to be inefficient we can always consider other options,
// such as doing a Schwartzian transform.
//
// Numeric keys are sorted in numeric order per
// https://developers.google.com/protocol-buffers/docs/proto#maps.
type mapKeys []reflect.Value

func (s mapKeys) Len() int      { return len(s) }
func (s mapKeys) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s mapKeys) Less(i, j int) bool {
	if k := s[i].Kind(); k == s[j].Kind() {
		switch k {
		case reflect.Int32, reflect.Int64:
			return s[i].Int() < s[j].Int()
		case reflect.Uint32, reflect.Uint64:
			return s[i].Uint() < s[j].Uint()
		}
	}
	return fmt.Sprint(s[i].Interface()) < fmt.Sprint(s[j].Interface())
}
