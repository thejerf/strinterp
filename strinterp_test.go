package strinterp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"
)

type StrinterpTest struct {
	Format string
	Args   []interface{}
	Result string
	Error  error
}

var ErrCustom = errors.New("custom error")

func badEncoder(inner io.Writer, params []byte) (io.Writer, error) {
	return nil, ErrCustom
}

func badFormatter(inner io.Writer, val interface{}, params []byte) error {
	return ErrCustom
}

// This is the Big Daddy of the tests; does the interpolator actually interpolate?
func TestInterpolator(t *testing.T) {
	noargs := []interface{}{}

	tests := []StrinterpTest{
		// really basic tests
		{"x", noargs, "x", nil},
		{"x%%;y", []interface{}{}, "x%y", nil},
		{"x%RAW;", []interface{}{"y"}, "xy", nil},
		{"x%RAW;", []interface{}{[]byte("y")}, "xy", nil},
		{"x%RAW;", []interface{}{bytes.NewBuffer([]byte("y"))}, "xy", nil},

		// error tests
		{"x%RAW", []interface{}{"y"}, "", ErrIncompleteFormatString},
		{"x%RAW;", []interface{}{0}, "", ErrNoDefaultHandling},
		{"x%RAW;", []interface{}{}, "", ErrNotGiven},
		{"x%blargh;", []interface{}{}, "", ErrUnknownFormatter("blargh")},
		{"x%RAW|blargh;", []interface{}{0}, "", ErrUnknownEncoder("blargh")},
		{"x%badform;", []interface{}{0}, "", ErrCustom},
		{"x%RAW|badenc;", []interface{}{0}, "", ErrCustom},
		{"x%RAW|badclose;", []interface{}{"a"}, "", ErrBadClose},
		{"x%json|badclose;", []interface{}{"a"}, "", ErrBadClose},

		// functionality tests
		// for base64, taken from the package docs, this ensures that we are
		// closing the base64 encoder properly
		{"%base64;", []interface{}{"foo\x00bar"}, "Zm9vAGJhcg==", nil},
		{"%RAW|base64:std;", []interface{}{"foo\x00bar"}, "Zm9vAGJhcg==", nil},
		{"%RAW|RAW|base64:std;", []interface{}{"foo\x00bar"}, "Zm9vAGJhcg==", nil},
		{"%RAW|base64:url;", []interface{}{"foo\x00bar"}, "Zm9vAGJhcg==", nil},
		// this turns out to be a good test to ensure that the writerStack
		// is indeed closing everything in the correct order; if
		// writerStack.Close() is reversed, the result gets cut off
		{"%base64|base64;", []interface{}{"a"}, "WVE9PQ==", nil},
		{"%base64:bad;", []interface{}{"a"}, "", ErrUnknownArguments{[]byte("bad"), "can only be std or url, to indicate the standard or URL base64 encoding"}},

		// JSON gets a lot of cases here because we have to cover all the
		// stuff in htmlSafeJSON
		{"%json;", []interface{}{"a"}, "\"a\"\n", nil},
		// verify the JSON HTML escaping is functioning correctly
		{"%json;", []interface{}{"\u2028"}, "\"\\u2028\"\n", nil},

		{"%cdata;", []interface{}{""}, "", nil},
		{"%cdata;", []interface{}{"a"}, "a", nil},
		{"%cdata;", []interface{}{"a<b>c"}, "a&lt;b&gt;c", nil},
		{"%cdata;", []interface{}{"<b>c"}, "&lt;b&gt;c", nil},
		{"%cdata;", []interface{}{"a<b>"}, "a&lt;b&gt;", nil},
		{"%cdata;", []interface{}{"a<>c"}, "a&lt;&gt;c", nil},
		{"%cdata;", []interface{}{"<>"}, "&lt;&gt;", nil},
		{"%cdata;", []interface{}{"aa<bb>cc"}, "aa&lt;bb&gt;cc", nil},
		{"%cdata;", []interface{}{"\r\n"}, "&#13;&#10;", nil},
		{"%cdata:nocrlf;", []interface{}{"\r\n"}, "\r\n", nil},
		{"%cdata:blargh;", []interface{}{"a"}, "", ErrUnknownArguments{[]byte("blargh"), "only nocrlf allowed for CDATA"}},
	}

	i := NewInterpolator()
	i.AddFormatter("json", JSON)
	i.AddEncoder("cdata", CDATA)
	i.AddEncoder("base64", Base64)
	i.AddFormatter("badform", badFormatter)
	i.AddEncoder("badenc", badEncoder)
	i.AddEncoder("badclose", badCloseEncoder)

	for _, test := range tests {
		res, err := i.InterpStr(test.Format, test.Args...)

		if test.Error != nil && !reflect.DeepEqual(test.Error, err) {
			// note in this case we aren't being hypocritcal... having just
			// established this package's interpolation is actually broken,
			// don't try to use it to output an error message!
			t.Fatal(fmt.Sprintf("for %s, expected error '%v', got '%v'", test.Format, test.Error, err))
		}
		if test.Result != "" && test.Result != res {
			t.Fatal(fmt.Sprintf("for %s, expected result '%s', got '%s'", test.Format, test.Result, res))
		}
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
	if err != ErrNotGiven {
		t.Fatal("Didn't handle error on missing params correctly:")
	}

	err = i.InterpWriter(WriterAlwaysEOF{}, []byte("%%;"))
	if err != io.EOF {
		t.Fatal("Didn't handle error on percent correctly", err)
	}
}

func TestParameters(t *testing.T) {
	i := NewInterpolator()

	i.AddFormatter("p", func(w io.Writer, arg interface{}, params []byte) error {
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
	// just assert these don't crash
	ErrAlreadyExists("x").Error()
	ErrUnknownFormatter("x").Error()
	ErrUnknownArguments{[]byte("a"), "hello"}.Error()
	ErrUnknownEncoder("x").Error()

	interp := NewDefaultInterpolator()
	if !reflect.DeepEqual(interp.AddFormatter("json", JSON), ErrAlreadyExists("json")) ||
		!reflect.DeepEqual(interp.AddEncoder("json", Base64), ErrAlreadyExists("json")) ||
		!reflect.DeepEqual(interp.AddFormatter("base64", JSON), ErrAlreadyExists("base64")) ||
		!reflect.DeepEqual(interp.AddEncoder("base64", Base64), ErrAlreadyExists("base64")) {
		t.Fatal("Can add existing formats or encoders")
	}

	err := interp.InterpWriter(WriterAlwaysEOF{}, []byte("%cdata;"), "<")
	if err != io.EOF {
		t.Fatal("Got the wrong error from CDATA when stream closed")
	}
}

type readBytesTest struct {
	input    string
	delim    string
	expected string
	error    error
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

var ErrBadClose = errors.New("weird, man, like, the close failed")

// This allows me to test that the writerStack properly propagates errors
// that .Close may throw. Which ought to be one heck of a corner case,
// but it could theoretically happen....
type WriterFailClose struct{}

func (wfc WriterFailClose) Write(b []byte) (n int, err error) {
	return len(b), nil
}

func (wfc WriterFailClose) Close() error {
	return ErrBadClose
}

func badCloseEncoder(w io.Writer, args []byte) (io.Writer, error) {
	return WriterFailClose{}, nil
}

// The purpose of this code is to test whether it is faster to try to
// "memoize" the resolution of the interface method, or if that doesn't
// save any time.
//
// Surprisingly to me, as of Go 1.4, calling straight through the interface
// doesn't just *tie* the "memoized" version, it is *faster* than it. I
// could easily see this changing in future versions, though.
type Blank interface {
	Nothing()
}

type B struct{}

func (b B) Nothing() {}

func BenchmarkCallThroughInterface(b *testing.B) {
	var instance Blank
	instance = B{}

	for i := 0; i < b.N; i++ {
		instance.Nothing()
	}
}

func BenchmarkMemoizedInterfaceCall(b *testing.B) {
	var instance Blank
	instance = B{}

	nothing := instance.Nothing

	for i := 0; i < b.N; i++ {
		nothing()
	}
}
