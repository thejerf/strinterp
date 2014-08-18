package strinterp

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"io"
	"reflect"
	"testing"
)

type StrescTest struct {
	Format string
	Args   []interface{}
	Result string
	Error  error
}

func TestDefaultInterpolator(t *testing.T) {
	noargs := []interface{}{}

	tests := []StrescTest{
		{"x", noargs, "x", nil},
		{"x%%;y", []interface{}{}, "x%y", nil},
		{"x%RAW;", []interface{}{"y"}, "xy", nil},
		{"x%RAW;", []interface{}{[]byte("y")}, "xy", nil},
		{"x%RAW;", []interface{}{bytes.NewBuffer([]byte("y"))}, "xy", nil},

		{"x%RAW", []interface{}{"y"}, "", ErrIncompleteFormatString},
		{"x%RAW;", []interface{}{0}, "", ErrRawUnknownType},
		{"x%RAW;", []interface{}{}, "x%RAW error: No arg;", nil},
		{"x%blargh;", []interface{}{}, "", ErrUnknownFormatter("blargh")},
	}

	i := NewInterpolator()

	for _, test := range tests {
		res, err := i.InterpStr(test.Format, test.Args...)

		if test.Error != nil && !reflect.DeepEqual(test.Error, err) {
			// note in this case we aren't being hypocritcal... having just
			// established this package's interpolation is actually broken,
			// don't try to use it to output an error message!
			t.Fatal(fmt.Sprintf("for %s, expected error %v, got %v", test.Format, test.Error, err))
		}
		if test.Result != "" && test.Result != res {
			t.Fatal(fmt.Sprintf("for %s, expected result '%s', got '%s'", test.Format, test.Result, res))
		}
	}
}

func TestOtherInterpolators(t *testing.T) {
	i := NewInterpolator()

	err := i.AddFormat("cdata", func(w io.Writer, arg interface{}, params []byte) error {
		switch a := arg.(type) {
		case []byte:
			newBytes := []byte(html.EscapeString(string(a)))
			_, err := w.Write(newBytes)
			return err
		case string:
			newBytes := []byte(html.EscapeString(a))
			_, err := w.Write(newBytes)
			return err
		default:
			return errors.New("unknown type for cdata")
		}
	})

	if err != nil {
		t.Fatal("Can't add new format?")
	}

	err = i.AddFormat("cdata", func(io.Writer, interface{}, []byte) error { return nil })
	if !reflect.DeepEqual(err, ErrAlreadyExists("cdata")) {
		t.Fatal("fails to catch double-type add")
	}

	res, err := i.InterpStr("hello %cdata;", "& stuff")
	if err != nil {
		t.Fatal("Error: " + err.Error())
	}

	if res != "hello &amp; stuff" {
		t.Fatal("Didn't get correct cdata string: " + res)
	}
}

func TestWriterErrors(t *testing.T) {
	i := NewInterpolator()

	err := i.InterpWriter(WriterAlwaysEOF{}, []byte("x"))
	if err != io.EOF {
		t.Fatal("Don't handle EOF in writer correctly")
	}

	err = i.InterpWriter(WriterAlwaysEOF{}, []byte("x%RAW;"), "x")
	if err != io.EOF {
		t.Fatal("Doesn't handle EOF in plain template writing correctly")
	}

	err = i.InterpWriter(WriterAlwaysEOF{}, []byte("%RAW;"))
	if err != io.EOF {
		t.Fatal("Didn't handle error on missing params correctly")
	}

	err = i.InterpWriter(WriterAlwaysEOF{}, []byte("%%;"))
	if err != io.EOF {
		t.Fatal("Didn't handle error on %%; correctly")
	}
}

func TestParameters(t *testing.T) {
	i := NewInterpolator()

	i.AddFormat("p", func(w io.Writer, arg interface{}, params []byte) error {
		w.Write(params)
		w.Write([]byte(arg.(string)))
		return nil
	})

	res, err := i.InterpStr("%p:hi!;", "moo")
	if err != nil || res != "hi!moo" {
		t.Fatal("parameters not handled correctly")
	}
}

func TestCover(t *testing.T) {
	// assert these don't crash

	ErrAlreadyExists("x").Error()
	ErrUnknownFormatter("x").Error()
}

type readBytesTest struct {
	input string
	delim string
	expected string
	error error
}

func TestReadBytesUntilUnesc(t *testing.T) {
	tests := []readBytesTest{
		{"abc", "c", "ab", nil},
		{"abc", "d", "abc", io.EOF},
		{`a\bc`, "c", "ab", nil},
		{`a\\bc`, "c", `a\b`, nil},
		{`abc\`, "d", `abc`, nil},
	}

	for _, test := range tests {
		res, err := readBytesUntilUnescDelim(
			bytes.NewBuffer([]byte(test.input)),
			[]byte(test.delim)[0],
		)
		if test.error != nil && !reflect.DeepEqual(test.error, err) {
			t.Fatal("Failed: wrong error on " + test.input)
		}
		if res != nil && string(res) != test.expected {
			t.Fatal("Failed: wrong result on " + test.input)
		}
	}
}

type WriterAlwaysEOF struct{}

func (wae WriterAlwaysEOF) Write(b []byte) (n int, err error) {
	if len(b) > 0 {
		return 0, io.EOF
	}
	return 0, nil
}
