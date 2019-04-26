package honeycomb

import (
	"encoding/hex"
	"reflect"
	"strings"
	"testing"
)

func TestJSONInterpreter_Interpret(t *testing.T) {
	r := &JSONInterpreter{}

	tests := []struct {
		name    string
		input   string
		fields  map[string]interface{}
		wantlen int
		wantf   map[string]interface{}
	}{
		{"not json at all", "hi", map[string]interface{}{"a": "hi"}, 2, map[string]interface{}{"a": "hi"}},
		{"not a json object", `"hi"`, map[string]interface{}{"c": "abc"}, 4, map[string]interface{}{"c": "abc"}},
		{"empty json", "{}", map[string]interface{}{"a": "abc"}, 0, map[string]interface{}{"a": "abc"}},
		{"simple", `{"b":"hi"}`, map[string]interface{}{"a": "abc"}, 0, map[string]interface{}{"a": "abc", "b": "hi"}},
		{"several", `{"b":"hi", "msg":"lots of things"}`, map[string]interface{}{"a": "abc"}, 0, map[string]interface{}{
			"a": "abc", "b": "hi", "msg": "lots of things",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotbytes, gotfields := r.Interpret([]byte(tt.input), tt.fields)
			if len(gotbytes) != tt.wantlen {
				t.Errorf("RedisInterpreter.Interpret() return %d bytes, expected %d", len(gotbytes), tt.wantlen)
			}
			if !reflect.DeepEqual(gotfields, tt.wantf) {
				t.Errorf("JSONInterpreter.Interpret() got1 = %v, want %v", gotfields, tt.wantf)
			}
		})
	}
}

func TestLastChanceInterpreter_Interpret(t *testing.T) {
	r := &LastChanceInterpreter{
		Escaper: func(data []byte) string { return hex.EncodeToString(data) },
	}

	tests := []struct {
		name    string
		input   string
		fields  map[string]interface{}
		wantlen int
		wantf   map[string]interface{}
	}{
		{"basic", "hi", map[string]interface{}{}, 0, map[string]interface{}{"_other": "6869"}},
		{"additive", "hi", map[string]interface{}{"c": "abc"}, 0, map[string]interface{}{"_other": "6869", "c": "abc"}},
		{"override", "ddd", map[string]interface{}{"a": "abc"}, 0, map[string]interface{}{"a": "abc", "_other": "646464"}},
		{"empty", "", map[string]interface{}{"a": "abc"}, 0, map[string]interface{}{"a": "abc"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotbytes, gotfields := r.Interpret([]byte(tt.input), tt.fields)
			if len(gotbytes) != tt.wantlen {
				t.Errorf("RedisInterpreter.Interpret() return %d bytes, expected %d", len(gotbytes), tt.wantlen)
			}
			if !reflect.DeepEqual(gotfields, tt.wantf) {
				t.Errorf("JSONInterpreter.Interpret() got1 = %v, want %v", gotfields, tt.wantf)
			}
		})
	}
}

func TestRequiredFieldsInterpreterBasic(t *testing.T) {
	r := &RequiredFieldsInterpreter{
		Defaults: map[string]interface{}{
			"a": 1,
			"b": "buzz",
		},
	}

	tests := []struct {
		name    string
		input   string
		fields  map[string]interface{}
		wantlen int
		wantf   map[string]interface{}
	}{
		{"basic", "hi", map[string]interface{}{}, 2, map[string]interface{}{"a": 1, "b": "buzz"}},
		{"additive", "hi", map[string]interface{}{"c": "hello"}, 2, map[string]interface{}{"a": 1, "b": "buzz", "c": "hello"}},
		{"override", "whee", map[string]interface{}{"a": "hello"}, 4, map[string]interface{}{"a": 1, "b": "buzz"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotbytes, gotfields := r.Interpret([]byte(tt.input), tt.fields)
			if len(gotbytes) != tt.wantlen {
				t.Errorf("RedisInterpreter.Interpret() return %d bytes, expected %d", len(gotbytes), tt.wantlen)
			}
			if !reflect.DeepEqual(gotfields, tt.wantf) {
				t.Errorf("JSONInterpreter.Interpret() got1 = %v, want %v", gotfields, tt.wantf)
			}
		})
	}
}

func TestTendermintInterpreter_Interpret(t *testing.T) {
	type fields struct {
		Keys []string
	}
	type args struct {
		data   []byte
		fields map[string]interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []byte
		want1  map[string]interface{}
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &TendermintInterpreter{
				Keys: tt.fields.Keys,
			}
			got, got1 := i.Interpret(tt.args.data, tt.args.fields)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TendermintInterpreter.Interpret() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("TendermintInterpreter.Interpret() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestRedisInterpreterBasic(t *testing.T) {
	r := &RedisInterpreter{}
	emptyf := make(map[string]interface{})
	expected := []string{
		"pid", "role", "timestamp", "level", "msg",
	}

	tests := []struct {
		name    string
		input   string
		fields  map[string]interface{}
		wantlen int
		wantf   []string
	}{
		{"basic", "66940:C 18 Apr 2019 15:18:28.565 # Configuration loaded", emptyf, 0, expected},
		{"bad date", "23434:M 18-Apr-2019 14:12:28.565 . bad date", emptyf, 0, expected},
		{"short pid", "5:C 23 Jul 2020 15:18:28.032 - 342asdfuj2", emptyf, 0, expected},
		{"fail", "this is a failure", emptyf, 0, []string{"_txt"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotbytes, gotfields := r.Interpret([]byte(tt.input), tt.fields)
			if len(gotbytes) != tt.wantlen {
				t.Errorf("RedisInterpreter.Interpret() return %d bytes, expected %d", len(gotbytes), tt.wantlen)
			}
			for _, e := range tt.wantf {
				if _, ok := gotfields[e]; !ok {
					t.Errorf("RedisInterpreter.Interpret() got = %#v, expected it to have %s", gotfields, e)
				}
			}
		})
	}
}

var sampleRedisLogs = `
66940:C 18 Apr 2019 15:18:28.565 # oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0Oo
66940:C 18 Apr 2019 15:18:28.565 # Redis version=5.0.4, bits=64, commit=00000000, modified=0, pid=66940, just started
66940:C 18 Apr 2019 15:18:28.565 # Configuration loaded
66940:M 18 Apr 2019 15:18:28.566 # You requested maxclients of 10000 requiring at least 10032 max file descriptors.
66940:M 18 Apr 2019 15:18:28.566 # Server can't set maximum open files to 10032 because of OS error: Operation not permitted.
66940:M 18 Apr 2019 15:18:28.566 # Current maximum open files is 1024. maxclients has been reduced to 992 to compensate for low ulimit. If you need higher maxclients increase 'ulimit -n'.
66940:M 18 Apr 2019 15:18:28.567 * Running mode=standalone, port=6380.
66940:M 18 Apr 2019 15:18:28.567 # Server initialized
66940:M 18 Apr 2019 15:18:28.569 * DB loaded from disk: 0.001 seconds
66940:M 18 Apr 2019 15:18:28.569 * Ready to accept connections
66940:M 18 Apr 2019 15:19:29.084 * 1 changes in 60 seconds. Saving...
66940:M 18 Apr 2019 15:19:29.085 * Background saving started by pid 67252
67252:C 18 Apr 2019 15:19:29.087 * DB saved on disk
66940:M 18 Apr 2019 15:19:29.190 * Background saving terminated with success
66940:M 18 Apr 2019 15:20:30.003 * 1 changes in 60 seconds. Saving...
66940:M 18 Apr 2019 15:20:30.011 * Background saving started by pid 67489
67489:C 18 Apr 2019 15:20:30.015 * DB saved on disk
66940:M 18 Apr 2019 15:20:30.111 * Background saving terminated with success
`

func TestRedisInterpreterReal(t *testing.T) {
	r := &RedisInterpreter{}
	f := make(map[string]interface{})
	expected := []string{
		"pid", "role", "timestamp", "level", "msg",
	}

	for _, line := range strings.Split(sampleRedisLogs, "\n") {
		gotbytes, gotfields := r.Interpret([]byte(line), f)
		if len(gotbytes) == 0 && len(gotfields) == 0 {
			continue
		}
		if len(gotbytes) != 0 {
			t.Errorf("RedisInterpreter.Interpret() got = %v, expected nothing", gotbytes)
		}
		for _, e := range expected {
			if _, ok := gotfields[e]; !ok {
				t.Errorf("RedisInterpreter.Interpret() got = %#v, expected it to have %s\n(parsing %s)", gotfields, e, line)
			}
		}
	}
}
