package runtime

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/golang/protobuf/ptypes/struct"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/grpc-ecosystem/grpc-gateway/examples/examplepb"
	"github.com/golang/protobuf/ptypes/duration"
	"time"
)

func TestJSONBuiltinMarshal(t *testing.T) {
	var m JSONCustom
	msg := examplepb.SimpleMessage{
		Id: "foo",
	}

	buf, err := m.Marshal(&msg)
	if err != nil {
		t.Errorf("m.Marshal(%v) failed with %v; want success", &msg, err)
	}

	var got examplepb.SimpleMessage
	if err := json.Unmarshal(buf, &got); err != nil {
		t.Errorf("json.Unmarshal(%q, &got) failed with %v; want success", buf, err)
	}
	if want := msg; !reflect.DeepEqual(got, want) {
		t.Errorf("got = %v; want %v", &got, &want)
	}
}

func TestJSONBuiltinMarshalField(t *testing.T) {
	var m JSONCustom
	for _, fixt := range builtinFieldFixtures {
		buf, err := m.Marshal(fixt.data)
		if err != nil {
			t.Errorf("m.Marshal(%v) failed with %v; want success", fixt.data, err)
		}
		if got, want := string(buf), fixt.json; got != want {
			t.Errorf("got = %q; want %q; data = %#v", got, want, fixt.data)
		}
	}
}

// todo need be fix
/*func TestJSONBuiltinMarshalFieldKnownErrors(t *testing.T) {
	var m JSONCustom
	for _, fixt := range builtinKnownErrors {
		buf, err := m.Marshal(fixt.data)
		if err != nil {
			t.Errorf("m.Marshal(%v) failed with %v; want success", fixt.data, err)
		}
		if got, want := string(buf), fixt.json; got == want {
			t.Errorf("surprisingly got = %q; as want %q; data = %#v", got, want, fixt.data)
		}
	}
}*/

func TestJSONBuiltinsnmarshal(t *testing.T) {
	var (
		m   JSONCustom
		got examplepb.SimpleMessage

		data = []byte(`{"id": "foo"}`)
	)
	if err := m.Unmarshal(data, &got); err != nil {
		t.Errorf("m.Unmarshal(%q, &got) failed with %v; want success", data, err)
	}

	want := examplepb.SimpleMessage{
		Id: "foo",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got = %v; want = %v", &got, &want)
	}
}

func TestJSONBuiltinUnmarshalField(t *testing.T) {
	var m JSONCustom
	for _, fixt := range builtinFieldFixtures {
		dest := reflect.New(reflect.TypeOf(fixt.data))
		if err := m.Unmarshal([]byte(fixt.json), dest.Interface()); err != nil {
			t.Errorf("m.Unmarshal(%q, dest) failed with %v; want success", fixt.json, err)
		}

		if got, want := dest.Elem().Interface(), fixt.data; !reflect.DeepEqual(got, want) {
			t.Errorf("got = %#v; want = %#v; input = %q", got, want, fixt.json)
		}
	}
}

func TestJSONBuiltinUnmarshalFieldKnownErrors(t *testing.T) {
	var m JSONCustom
	for _, fixt := range builtinKnownErrors {
		dest := reflect.New(reflect.TypeOf(fixt.data))
		if err := m.Unmarshal([]byte(fixt.json), dest.Interface()); err == nil {
			t.Errorf("m.Unmarshal(%q, dest) succeeded; want ane error", fixt.json)
		}
	}
}

func TestJSONBuiltinEncoder(t *testing.T) {
	var m JSONCustom
	msg := examplepb.SimpleMessage{
		Id: "foo",
	}

	var buf bytes.Buffer
	enc := m.NewEncoder(&buf)
	if err := enc.Encode(&msg); err != nil {
		t.Errorf("enc.Encode(%v) failed with %v; want success", &msg, err)
	}

	var got examplepb.SimpleMessage
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Errorf("json.Unmarshal(%q, &got) failed with %v; want success", buf.String(), err)
	}
	if want := msg; !reflect.DeepEqual(got, want) {
		t.Errorf("got = %v; want %v", &got, &want)
	}
}

func TestJSONBuiltinEncoderFields(t *testing.T) {
	var m JSONCustom
	for _, fixt := range builtinFieldFixtures {
		var buf bytes.Buffer
		enc := m.NewEncoder(&buf)
		if err := enc.Encode(fixt.data); err != nil {
			t.Errorf("enc.Encode(%#v) failed with %v; want success", fixt.data, err)
		}

		if got, want := buf.String(), fixt.json+"\n"; got != want {
			t.Errorf("got = %q; want %q; data = %#v", got, want, fixt.data)
		}
	}
}

func TestJSONBuiltinDecoder(t *testing.T) {
	var (
		m   JSONCustom
		got examplepb.SimpleMessage

		data = `{"id": "foo"}`
	)
	r := strings.NewReader(data)
	dec := m.NewDecoder(r)
	if err := dec.Decode(&got); err != nil {
		t.Errorf("m.Unmarshal(&got) failed with %v; want success", err)
	}

	want := examplepb.SimpleMessage{
		Id: "foo",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got = %v; want = %v", &got, &want)
	}
}

func TestJSONBuiltinDecoderFields(t *testing.T) {
	var m JSONCustom
	for _, fixt := range builtinFieldFixtures {
		r := strings.NewReader(fixt.json)
		dec := m.NewDecoder(r)
		dest := reflect.New(reflect.TypeOf(fixt.data))
		if err := dec.Decode(dest.Interface()); err != nil {
			t.Errorf("dec.Decode(dest) failed with %v; want success; data = %q", err, fixt.json)
		}

		if got, want := dest.Elem().Interface(), fixt.data; !reflect.DeepEqual(got, want) {
			t.Errorf("got = %v; want = %v; input = %q", got, want, fixt.json)
		}
	}
}

var (
	builtinFieldFixtures = []struct {
		data interface{}
		json string
	}{
		{data: "", json: `""`},
		{data: proto.String(""), json: `""`},
		{data: "foo", json: `"foo"`},
		{data: proto.String("foo"), json: `"foo"`},
		{data: int32(-1), json: "-1"},
		{data: proto.Int32(-1), json: "-1"},
		{data: int64(-1), json: "-1"},
		{data: proto.Int64(-1), json: "-1"},
		{data: uint32(123), json: "123"},
		{data: proto.Uint32(123), json: "123"},
		{data: uint64(123), json: "123"},
		{data: proto.Uint64(123), json: "123"},
		{data: float32(-1.5), json: "-1.5"},
		{data: proto.Float32(-1.5), json: "-1.5"},
		{data: float64(-1.5), json: "-1.5"},
		{data: proto.Float64(-1.5), json: "-1.5"},
		{data: true, json: "true"},
		{data: proto.Bool(true), json: "true"},
		{data: (*string)(nil), json: "null"},
		{data: new(empty.Empty), json: "{}"},
		{data: examplepb.NumericEnum_ONE, json: "1"},
		{
			data: (*examplepb.NumericEnum)(proto.Int32(int32(examplepb.NumericEnum_ONE))),
			json: "1",
		},
	}
	builtinKnownErrors = []struct {
		data interface{}
		json string
	}{
		{data: examplepb.NumericEnum_ONE, json: "ONE"},
		{
			data: (*examplepb.NumericEnum)(proto.Int32(int32(examplepb.NumericEnum_ONE))),
			json: "ONE",
		},
		{
			data: &examplepb.ABitOfEverything_OneofString{OneofString: "abc"},
			json: `"abc"`,
		},
		{
			data: &timestamp.Timestamp{
				Seconds: 1462875553,
				Nanos:   123000000,
			},
			json: `"2016-05-10T10:19:13.123Z"`,
		},
		{
			data: &wrappers.Int32Value{Value: 123},
			json: "123",
		},
		{
			data: &structpb.Value{
				Kind: &structpb.Value_StringValue{
					StringValue: "abc",
				},
			},
			json: `"abc"`,
		},
	}
)

// todo need be fix
/*func TestJSONPbMarshal(t *testing.T) {
	msg := examplepb.ABitOfEverything{
		SingleNested:        &examplepb.ABitOfEverything_Nested{},
		RepeatedStringValue: []string{},
		MappedStringValue:   map[string]string{},
		MappedNestedValue:   map[string]*examplepb.ABitOfEverything_Nested{},
		RepeatedEnumValue:   []examplepb.NumericEnum{},
		TimestampValue:      &timestamp.Timestamp{},
		Uuid:                "6EC2446F-7E89-4127-B3E6-5C05E6BECBA7",
		Nested: []*examplepb.ABitOfEverything_Nested{
			{
				Name:   "foo",
				Amount: 12345,
			},
		},
		Uint64Value: 0xFFFFFFFFFFFFFFFF,
		EnumValue:   examplepb.NumericEnum_ONE,
		OneofValue: &examplepb.ABitOfEverything_OneofString{
			OneofString: "bar",
		},
		MapValue: map[string]examplepb.NumericEnum{
			"a": examplepb.NumericEnum_ONE,
			"b": examplepb.NumericEnum_ZERO,
		},
	}

	for i, spec := range []struct {
		enumsAsInts, emitDefaults bool
		indent                    string
		origName                  bool
		verifier                  func(json string)
	}{
		{
			verifier: func(json string) {
				if strings.ContainsAny(json, " \t\r\n") {
					t.Errorf("strings.ContainsAny(%q, %q) = true; want false", json, " \t\r\n")
				}
				if !strings.Contains(json, "ONE") {
					t.Errorf(`strings.Contains(%q, "ONE") = false; want true`, json)
				}
				if want := "uint64Value"; !strings.Contains(json, want) {
					t.Errorf(`strings.Contains(%q, %q) = false; want true`, json, want)
				}
			},
		},
		{
			enumsAsInts: true,
			verifier: func(json string) {
				if strings.Contains(json, "ONE") {
					t.Errorf(`strings.Contains(%q, "ONE") = true; want false`, json)
				}
			},
		},
		{
			emitDefaults: true,
			verifier: func(json string) {
				if want := `"sfixed32Value"`; !strings.Contains(json, want) {
					t.Errorf(`strings.Contains(%q, %q) = false; want true`, json, want)
				}
			},
		},
		{
			indent: "\t\t",
			verifier: func(json string) {
				if want := "\t\t\"amount\":"; !strings.Contains(json, want) {
					t.Errorf(`strings.Contains(%q, %q) = false; want true`, json, want)
				}
			},
		},
		{
			origName: true,
			verifier: func(json string) {
				if want := "uint64_value"; !strings.Contains(json, want) {
					t.Errorf(`strings.Contains(%q, %q) = false; want true`, json, want)
				}
			},
		},
	} {
		m := JSONCustom{
			EnumsAsInts:  spec.enumsAsInts,
			EmitDefaults: spec.emitDefaults,
			Indent:       spec.indent,
			OrigName:     spec.origName,
		}
		buf, err := m.Marshal(&msg)
		if err != nil {
			t.Errorf("m.Marshal(%v) failed with %v; want success; spec=%v", &msg, err, spec)
		}

		var got examplepb.ABitOfEverything
		if err := UnmarshalString(string(buf), &got); err != nil {
			t.Errorf("jsonpb.UnmarshalString(%q, &got) failed with %v; want success; spec=%v", string(buf), err, spec)
		}
		if want := msg; !reflect.DeepEqual(got, want) {
			t.Errorf("case %d: got = %v; want %v; spec=%v", i, &got, &want, spec)
		}
		if spec.verifier != nil {
			spec.verifier(string(buf))
		}
	}
}*/

func TestJSONPbMarshalFields(t *testing.T) {
	var m JSONCustom
	for _, spec := range []struct {
		val  interface{}
		want string
	}{} {
		buf, err := m.Marshal(spec.val)
		if err != nil {
			t.Errorf("m.Marshal(%#v) failed with %v; want success", spec.val, err)
		}
		if got, want := string(buf), spec.want; got != want {
			t.Errorf("m.Marshal(%#v) = %q; want %q", spec.val, got, want)
		}
	}

	m.EnumsAsInts = true
	buf, err := m.Marshal(examplepb.NumericEnum_ONE)
	if err != nil {
		t.Errorf("m.Marshal(%#v) failed with %v; want success", examplepb.NumericEnum_ONE, err)
	}
	if got, want := string(buf), "1"; got != want {
		t.Errorf("m.Marshal(%#v) = %q; want %q", examplepb.NumericEnum_ONE, got, want)
	}
}

func TestJSONPbUnmarshal(t *testing.T) {
	var (
		m   JSONCustom
		got examplepb.ABitOfEverything
	)
	for i, data := range []string{
		`{
			"uuid": "6EC2446F-7E89-4127-B3E6-5C05E6BECBA7",
			"nested": [
				{"name": "foo", "amount": 12345}
			],
			"uint64Value": 18446744073709551615,
			"enumValue": "ONE",
			"oneofString": "bar",
			"mapValue": {
				"a": 1,
				"b": 0
			}
		}`,
		`{
			"uuid": "6EC2446F-7E89-4127-B3E6-5C05E6BECBA7",
			"nested": [
				{"name": "foo", "amount": 12345}
			],
			"uint64Value": "18446744073709551615",
			"enumValue": "ONE",
			"oneofString": "bar",
			"mapValue": {
				"a": 1,
				"b": 0
			}
		}`,
		`{
			"uuid": "6EC2446F-7E89-4127-B3E6-5C05E6BECBA7",
			"nested": [
				{"name": "foo", "amount": 12345}
			],
			"uint64Value": 18446744073709551615,
			"enumValue": 1,
			"oneofString": "bar",
			"mapValue": {
				"a": 1,
				"b": 0
			}
		}`,
	} {
		if err := m.Unmarshal([]byte(data), &got); err != nil {
			t.Errorf("case %d: m.Unmarshal(%q, &got) failed with %v; want success", i, data, err)
		}

		want := examplepb.ABitOfEverything{
			Uuid: "6EC2446F-7E89-4127-B3E6-5C05E6BECBA7",
			Nested: []*examplepb.ABitOfEverything_Nested{
				{
					Name:   "foo",
					Amount: 12345,
				},
			},
			Uint64Value: 0xFFFFFFFFFFFFFFFF,
			EnumValue:   examplepb.NumericEnum_ONE,
			OneofValue: &examplepb.ABitOfEverything_OneofString{
				OneofString: "bar",
			},
			MapValue: map[string]examplepb.NumericEnum{
				"a": examplepb.NumericEnum_ONE,
				"b": examplepb.NumericEnum_ZERO,
			},
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("case %d: got = %v; want = %v", i, &got, &want)
		}
	}
}

// todo need be fix
/*func TestJSONPbUnmarshalFields(t *testing.T) {
	var m JSONCustom
	for _, fixt := range fieldFixtures {
		if fixt.skipUnmarshal {
			continue
		}

		dest := reflect.New(reflect.TypeOf(fixt.data))
		if err := m.Unmarshal([]byte(fixt.json), dest.Interface()); err != nil {
			t.Errorf("m.Unmarshal(%q, %T) failed with %v; want success", fixt.json, dest.Interface(), err)
		}
		if got, want := dest.Elem().Interface(), fixt.data; !reflect.DeepEqual(got, want) {
			t.Errorf("dest = %#v; want %#v; input = %v", got, want, fixt.json)
		}
	}
}*/

// todo need be fix
/*func TestJSONPbEncoder(t *testing.T) {
	msg := examplepb.ABitOfEverything{
		SingleNested:        &examplepb.ABitOfEverything_Nested{},
		RepeatedStringValue: []string{},
		MappedStringValue:   map[string]string{},
		MappedNestedValue:   map[string]*examplepb.ABitOfEverything_Nested{},
		RepeatedEnumValue:   []examplepb.NumericEnum{},
		TimestampValue:      &timestamp.Timestamp{},
		Uuid:                "6EC2446F-7E89-4127-B3E6-5C05E6BECBA7",
		Nested: []*examplepb.ABitOfEverything_Nested{
			{
				Name:   "foo",
				Amount: 12345,
			},
		},
		Uint64Value: 0xFFFFFFFFFFFFFFFF,
		OneofValue: &examplepb.ABitOfEverything_OneofString{
			OneofString: "bar",
		},
		MapValue: map[string]examplepb.NumericEnum{
			"a": examplepb.NumericEnum_ONE,
			"b": examplepb.NumericEnum_ZERO,
		},
	}

	for i, spec := range []struct {
		enumsAsInts, emitDefaults bool
		indent                    string
		origName                  bool
		verifier                  func(json string)
	}{
		{
			verifier: func(json string) {
				if strings.ContainsAny(json, " \t\r\n") {
					t.Errorf("strings.ContainsAny(%q, %q) = true; want false", json, " \t\r\n")
				}
				if strings.Contains(json, "ONE") {
					t.Errorf(`strings.Contains(%q, "ONE") = true; want false`, json)
				}
				if want := "uint64Value"; !strings.Contains(json, want) {
					t.Errorf(`strings.Contains(%q, %q) = false; want true`, json, want)
				}
			},
		},
		{
			enumsAsInts: true,
			verifier: func(json string) {
				if strings.Contains(json, "ONE") {
					t.Errorf(`strings.Contains(%q, "ONE") = true; want false`, json)
				}
			},
		},
		{
			emitDefaults: true,
			verifier: func(json string) {
				if want := `"sfixed32Value"`; !strings.Contains(json, want) {
					t.Errorf(`strings.Contains(%q, %q) = false; want true`, json, want)
				}
			},
		},
		{
			indent: "\t\t",
			verifier: func(json string) {
				if want := "\t\t\"amount\":"; !strings.Contains(json, want) {
					t.Errorf(`strings.Contains(%q, %q) = false; want true`, json, want)
				}
			},
		},
		{
			origName: true,
			verifier: func(json string) {
				if want := "uint64_value"; !strings.Contains(json, want) {
					t.Errorf(`strings.Contains(%q, %q) = false; want true`, json, want)
				}
			},
		},
	} {
		m := JSONCustom{
			EnumsAsInts:  spec.enumsAsInts,
			EmitDefaults: spec.emitDefaults,
			Indent:       spec.indent,
			OrigName:     spec.origName,
		}

		var buf bytes.Buffer
		enc := m.NewEncoder(&buf)
		if err := enc.Encode(&msg); err != nil {
			t.Errorf("enc.Encode(%v) failed with %v; want success; spec=%v", &msg, err, spec)
		}

		var got examplepb.ABitOfEverything
		if err := UnmarshalString(buf.String(), &got); err != nil {
			t.Errorf("jsonpb.UnmarshalString(%q, &got) failed with %v; want success; spec=%v", buf.String(), err, spec)
		}
		if want := msg; !reflect.DeepEqual(got, want) {
			t.Errorf("case %d: got = %v; want %v; spec=%v", i, &got, &want, spec)
		}
		if spec.verifier != nil {
			spec.verifier(buf.String())
		}
	}
}*/

// todo need be fix
/*func TestJSONPbEncoderFields(t *testing.T) {
	var m JSONCustom
	for _, fixt := range fieldFixtures {
		var buf bytes.Buffer
		enc := m.NewEncoder(&buf)
		if err := enc.Encode(fixt.data); err != nil {
			t.Errorf("enc.Encode(%#v) failed with %v; want success", fixt.data, err)
		}
		if got, want := buf.String(), fixt.json; got != want {
			t.Errorf("enc.Encode(%#v) = %q; want %q", fixt.data, got, want)
		}
	}

	m.EnumsAsInts = true
	buf, err := m.Marshal(examplepb.NumericEnum_ONE)
	if err != nil {
		t.Errorf("m.Marshal(%#v) failed with %v; want success", examplepb.NumericEnum_ONE, err)
	}
	if got, want := string(buf), "1"; got != want {
		t.Errorf("m.Marshal(%#v) = %q; want %q", examplepb.NumericEnum_ONE, got, want)
	}
}*/

// todo need be fix
/*func TestJSONPbDecoder(t *testing.T) {
	var (
		m   JSONCustom
		got examplepb.ABitOfEverything
	)
	for _, data := range []string{
		`{
			"uuid": "6EC2446F-7E89-4127-B3E6-5C05E6BECBA7",
			"nested": [
				{"name": "foo", "amount": 12345}
			],
			"uint64Value": 18446744073709551615,
			"enumValue": "ONE",
			"oneofString": "bar",
			"mapValue": {
				"a": 1,
				"b": 0
			}
		}`,
		`{
			"uuid": "6EC2446F-7E89-4127-B3E6-5C05E6BECBA7",
			"nested": [
				{"name": "foo", "amount": 12345}
			],
			"uint64Value": "18446744073709551615",
			"enumValue": "ONE",
			"oneofString": "bar",
			"mapValue": {
				"a": 1,
				"b": 0
			}
		}`,
		`{
			"uuid": "6EC2446F-7E89-4127-B3E6-5C05E6BECBA7",
			"nested": [
				{"name": "foo", "amount": 12345}
			],
			"uint64Value": 18446744073709551615,
			"enumValue": 1,
			"oneofString": "bar",
			"mapValue": {
				"a": 1,
				"b": 0
			}
		}`,
	} {
		r := strings.NewReader(data)
		dec := m.NewDecoder(r)
		if err := dec.Decode(&got); err != nil {
			t.Errorf("m.Unmarshal(&got) failed with %v; want success; data=%q", err, data)
		}

		want := examplepb.ABitOfEverything{
			Uuid: "6EC2446F-7E89-4127-B3E6-5C05E6BECBA7",
			Nested: []*examplepb.ABitOfEverything_Nested{
				{
					Name:   "foo",
					Amount: 12345,
				},
			},
			Uint64Value: 0xFFFFFFFFFFFFFFFF,
			EnumValue:   examplepb.NumericEnum_ONE,
			OneofValue: &examplepb.ABitOfEverything_OneofString{
				OneofString: "bar",
			},
			MapValue: map[string]examplepb.NumericEnum{
				"a": examplepb.NumericEnum_ONE,
				"b": examplepb.NumericEnum_ZERO,
			},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got = %v; want = %v; data = %v", &got, &want, data)
		}
	}
}*/

// todo need be fix
/*func TestJSONPbDecoderFields(t *testing.T) {
	var m JSONCustom
	for _, fixt := range fieldFixtures {
		if fixt.skipUnmarshal {
			continue
		}

		dest := reflect.New(reflect.TypeOf(fixt.data))
		dec := m.NewDecoder(strings.NewReader(fixt.json))
		if err := dec.Decode(dest.Interface()); err != nil {
			t.Errorf("dec.Decode(%T) failed with %v; want success; input = %q", dest.Interface(), err, fixt.json)
		}
		if got, want := dest.Elem().Interface(), fixt.data; !reflect.DeepEqual(got, want) {
			t.Errorf("dest = %#v; want %#v; input = %v", got, want, fixt.json)
		}
	}
}*/

var (
	fieldFixtures = []struct {
		data          interface{}
		json          string
		skipUnmarshal bool
	}{
		{data: int32(1), json: "1"},
		{data: proto.Int32(1), json: "1"},
		{data: int64(1), json: "1"},
		{data: proto.Int64(1), json: "1"},
		{data: uint32(1), json: "1"},
		{data: proto.Uint32(1), json: "1"},
		{data: uint64(1), json: "1"},
		{data: proto.Uint64(1), json: "1"},
		{data: "abc", json: `"abc"`},
		{data: proto.String("abc"), json: `"abc"`},
		{data: float32(1.5), json: "1.5"},
		{data: proto.Float32(1.5), json: "1.5"},
		{data: float64(1.5), json: "1.5"},
		{data: proto.Float64(1.5), json: "1.5"},
		{data: true, json: "true"},
		{data: false, json: "false"},
		{data: (*string)(nil), json: "null"},
		{
			data: examplepb.NumericEnum_ONE,
			json: `"ONE"`,
			// TODO(yugui) support unmarshaling of symbolic enum
			skipUnmarshal: true,
		},
		{
			data: (*examplepb.NumericEnum)(proto.Int32(int32(examplepb.NumericEnum_ONE))),
			json: `"ONE"`,
			// TODO(yugui) support unmarshaling of symbolic enum
			skipUnmarshal: true,
		},

		{
			data: map[string]int32{
				"foo": 1,
			},
			json: `{"foo":1}`,
		},
		{
			data: map[string]*examplepb.SimpleMessage{
				"foo": {Id: "bar"},
			},
			json: `{"foo":{"id":"bar"}}`,
		},
		{
			data: map[int32]*examplepb.SimpleMessage{
				1: {Id: "foo"},
			},
			json: `{"1":{"id":"foo"}}`,
		},
		{
			data: map[bool]*examplepb.SimpleMessage{
				true: {Id: "foo"},
			},
			json: `{"true":{"id":"foo"}}`,
		},
		{
			data: &duration.Duration{
				Seconds: 123,
				Nanos:   456000000,
			},
			json: `"123.456s"`,
		},
		{
			data: &timestamp.Timestamp{
				Seconds: 1462875553,
				Nanos:   123000000,
			},
			json: `"2016-05-10T10:19:13.123Z"`,
		},
		{
			data: new(empty.Empty),
			json: "{}",
		},

		// TODO(yugui) Enable unmarshaling of the following examples
		// once jsonpb supports them.
		{
			data: &structpb.Value{
				Kind: new(structpb.Value_NullValue),
			},
			json:          "null",
			skipUnmarshal: true,
		},
		{
			data: &structpb.Value{
				Kind: &structpb.Value_NumberValue{
					NumberValue: 123.4,
				},
			},
			json:          "123.4",
			skipUnmarshal: true,
		},
		{
			data: &structpb.Value{
				Kind: &structpb.Value_StringValue{
					StringValue: "abc",
				},
			},
			json:          `"abc"`,
			skipUnmarshal: true,
		},
		{
			data: &structpb.Value{
				Kind: &structpb.Value_BoolValue{
					BoolValue: true,
				},
			},
			json:          "true",
			skipUnmarshal: true,
		},
		{
			data: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"foo_bar": {
						Kind: &structpb.Value_BoolValue{
							BoolValue: true,
						},
					},
				},
			},
			json:          `{"foo_bar":true}`,
			skipUnmarshal: true,
		},

		{
			data: &wrappers.BoolValue{Value: true},
			json: "true",
		},
		{
			data: &wrappers.DoubleValue{Value: 123.456},
			json: "123.456",
		},
		{
			data: &wrappers.FloatValue{Value: 123.456},
			json: "123.456",
		},
		{
			data: &wrappers.Int32Value{Value: -123},
			json: "-123",
		},
		{
			data: &wrappers.Int64Value{Value: -123},
			json: `"-123"`,
		},
		{
			data: &wrappers.UInt32Value{Value: 123},
			json: "123",
		},
		{
			data: &wrappers.UInt64Value{Value: 123},
			json: `"123"`,
		},
		// TODO(yugui) Add other well-known types once jsonpb supports them
	}
)

type Msg struct {
	Firstname       string `protobuf:"bytes,1,opt,name=firstname,proto3" json:"name.firstname,omitempty"`
	PseudoFirstname string `protobuf:"bytes,3,opt,name=pseudoLastname,proto3" json:"firstname,omitempty"`
	EmbedMsg               `protobuf:"bytes,4,opt,name=embedMsg,embedded=embedMsg" json:"embedMsg,omitempty"`
	Lastname        string `protobuf:"bytes,5,opt,name=lastname,proto3" json:"name.lastname,omitempty"`
	Inside          string `protobuf:"bytes,6,opt,name=inside,proto3" json:"name.inside.a.b.c,omitempty"`
	Normal          int32  `protobuf:"varint,6,opt,name=normal,proto3" json:"notToNormal,omitempty"`
}

func (m *Msg) Reset()         { *m = Msg{} }
func (m *Msg) String() string { return proto.CompactTextString(m) }
func (*Msg) ProtoMessage()    {}

type EmbedMsg struct {
	Opt1 string `protobuf:"bytes,1,opt,name=opt1,proto3" json:"opt1,omitempty"`
}

func (m *EmbedMsg) Reset()         { *m = EmbedMsg{} }
func (m *EmbedMsg) String() string { return proto.CompactTextString(m) }
func (*EmbedMsg) ProtoMessage()    {}

func TestJSONCustomNestedMarshal(t *testing.T) {
	var m JSONCustom

	msgInput := Msg{
		Firstname:       "One",
		Lastname:        "Two",
		PseudoFirstname: "Three",
		EmbedMsg: EmbedMsg{
			Opt1: "var",
		},
		Inside: "goo",
		Normal: 444,
	}

	data := []byte(`{"firstname":"Three","name":{"firstname":"One","inside":{"a":{"b":{"c":"goo"}}},"lastname":"Two"},"notToNormal":444,"opt1":"var"}`)

	buf, err := m.Marshal(&msgInput)
	if err != nil {
		t.Errorf("m.Marshal(%v) failed with %v; want success", &msgInput, err)
	}

	if !reflect.DeepEqual(buf, data) {
		t.Errorf("got = %v; want %v", string(buf), string(data))
	}
}

func TestJSONCustomNestedUnmarshal(t *testing.T) {
	var m JSONCustom

	msgInput := Msg{
		Firstname:       "One",
		Lastname:        "Two",
		PseudoFirstname: "Three",
		EmbedMsg: EmbedMsg{
			Opt1: "var",
		},
		Inside: "goo",
		Normal: 44,
	}

	data := []byte(`{"firstname":"Three","name":{"firstname":"One","inside":{"a":{"b":{"c":"goo"}}},"lastname":"Two"},"opt1":"var", "notToNormal":44}`)

	msgOutput := Msg{}

	if err := m.Unmarshal(data, &msgOutput); err != nil {
		t.Errorf("json.Unmarshal(%q, &data) failed with %v; want success", data, err)
	}

	if want := msgInput; !reflect.DeepEqual(msgOutput, want) {
		t.Errorf("got = %v; want %v", &msgOutput, &want)
	}
}

type Enum int32

const (
	Enum_ZERO Enum = 0
	Enum_ONE  Enum = 1
)

var Enum_name = map[int32]string{
	0: "ZERO",
	1: "ONE",
}
var Enum_value = map[string]int32{
	"ZERO": 0,
	"ONE":  1,
}

func (x Enum) String() string {
	return proto.EnumName(Enum_name, int32(x))
}

type Msg2 struct {
	Name            string         `protobuf:"bytes,1,opt,name=name" json:"name,omitempty"`
	NotEmbedMsgName *NotEmbedMsg   `protobuf:"bytes,2,opt,name=notEmbedMsgName" json:"notEmbedMsgName,omitempty"`
	Own             *Msg2          `protobuf:"bytes,3,opt,name=own" json:"own,omitempty"`
	Enum            Enum           `protobuf:"varint,4,opt,name=enum,proto3,enum=runtime.Enum" json:"enum,omitempty"`
	ThisIsSlice     []*NotEmbedMsg `protobuf:"bytes,5,rep,name=thisIsSlice" json:"thisIsSlice,omitempty"`
}

func (m *Msg2) Reset()         { *m = Msg2{} }
func (m *Msg2) String() string { return proto.CompactTextString(m) }
func (*Msg2) ProtoMessage()    {}

type NotEmbedMsg struct {
	Opt1 string `protobuf:"bytes,1,opt,name=opt1,proto3" json:"opt1,omitempty"`
	Enum Enum   `protobuf:"varint,2,opt,name=enum,proto3,enum=runtime.Enum" json:"enum,omitempty"`
}

func (m *NotEmbedMsg) Reset()         { *m = NotEmbedMsg{} }
func (m *NotEmbedMsg) String() string { return proto.CompactTextString(m) }
func (*NotEmbedMsg) ProtoMessage()    {}

func TestJSONCustomNested2Marshal(t *testing.T) {
	var m JSONCustom
	m.EmitDefaults = true
	m.EnumsAsInts = false

	msgInput := Msg2{
		Name:            "first of name",
		NotEmbedMsgName: &NotEmbedMsg{Opt1: "var", Enum: Enum_ZERO},
		Own:             &Msg2{},
		Enum:            Enum_ONE,
		ThisIsSlice: []*NotEmbedMsg{
			&NotEmbedMsg{Opt1: "slicevar0", Enum: Enum_ONE},
			&NotEmbedMsg{Opt1: "slicevar1", Enum: Enum_ZERO},
		},
	}

	data := []byte(`{"enum":"ONE","name":"first of name","notEmbedMsgName":{"enum":"ZERO","opt1":"var"},"own":{"enum":"ZERO","name":"","thisIsSlice":[]},"thisIsSlice":[{"enum":"ONE","opt1":"slicevar0"},{"enum":"ZERO","opt1":"slicevar1"}]}`)

	buf, err := m.Marshal(&msgInput)
	if err != nil {
		t.Errorf("m.Marshal(%v) failed with %v; want success", &msgInput, err)
	}

	if !reflect.DeepEqual(buf, data) {
		t.Errorf("got = %v; want %v", string(buf), string(data))
	}
}

type Model struct {
	ID        uint64     `protobuf:"varint,1,opt,name=ID,proto3" json:"ID,omitempty"`
	CreatedAt time.Time  `protobuf:"bytes,2,opt,name=created_at,json=createdAt,stdtime" json:"created_at"`
	UpdatedAt time.Time  `protobuf:"bytes,3,opt,name=updated_at,json=updatedAt,stdtime" json:"updated_at"`
	DeletedAt *time.Time `protobuf:"bytes,4,opt,name=deleted_at,json=deletedAt,stdtime" json:"deleted_at,omitempty"`
}

func (m *Model) Reset()         { *m = Model{} }
func (m *Model) String() string { return proto.CompactTextString(m) }
func (*Model) ProtoMessage()    {}

type T struct {
	Model       `protobuf:"bytes,1,opt,name=model,embedded=model" json:""`
	Name string `protobuf:"bytes,2,opt,name=name,proto3" json:"name"`
}

func (m *T) Reset()         { *m = T{} }
func (m *T) String() string { return proto.CompactTextString(m) }
func (*T) ProtoMessage()    {}

func TestJSONCustomNestedModelMarshal(t *testing.T) {
	var m JSONCustom
	m.EmitDefaults = true
	m.EnumsAsInts = false

	msgInput := T{
		Name: "first of name",
		Model: Model{
			ID:        1,
			CreatedAt: time.Date(1, 0, 0, 0, 0, 0, 0, time.Local),
			UpdatedAt: time.Date(2, 0, 0, 0, 0, 0, 0, time.Local),
			DeletedAt: nil,
		},
	}

	data := []byte(`{"ID":1,"created_at":"0000-11-30T00:00:00+00:57","name":"first of name","updated_at":"0001-11-30T00:00:00+00:57"}`)

	buf, err := m.Marshal(&msgInput)
	if err != nil {
		t.Errorf("m.Marshal(%v) failed with %v; want success", &msgInput, err)
	}

	if !reflect.DeepEqual(buf, data) {
		t.Errorf("got = %v; want %v", string(buf), string(data))
	}
}
