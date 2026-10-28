package output

import (
	"bytes"
	"testing"
)

func TestResolveEncoding(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "utf-8", false},
		{"UTF-8", "utf-8", false},
		{"utf8", "utf-8", false},
		{"gbk", "gbk", false},
		{"GB18030", "gb18030", false},
		{"gb2312", "gb2312", false},
		{"latin1", "", true},
	}
	for _, tc := range cases {
		enc, name, err := ResolveEncoding(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("%q: want error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if name != tc.want {
			t.Fatalf("%q: name=%s want %s", tc.in, name, tc.want)
		}
		if tc.want == "utf-8" && enc != nil {
			t.Fatalf("%q: utf-8 should use nil encoding", tc.in)
		}
		if tc.want != "utf-8" && enc == nil {
			t.Fatalf("%q: expected non-nil encoding", tc.in)
		}
	}
}

func TestMarshalUTF8Chinese(t *testing.T) {
	data, err := marshalResult(OK(map[string]string{"text": "你好"}), true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("你好")) {
		t.Fatalf("expected raw UTF-8 Chinese, got %s", data)
	}
	if bytes.Contains(data, []byte(`\u`)) {
		t.Fatalf("should not contain \\u escapes: %s", data)
	}
}

func TestEncodedWriterGBK(t *testing.T) {
	enc, _, err := ResolveEncoding("gbk")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	w := NewEncodedWriter(&buf, enc)
	if _, err := w.Write([]byte("张")); err != nil {
		t.Fatal(err)
	}
	if buf.String() == "张" {
		t.Fatal("expected GBK bytes, got UTF-8")
	}
	if len(buf.Bytes()) == 0 {
		t.Fatal("empty write")
	}
}
