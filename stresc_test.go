package stresc_test

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"io"
	"reflect"
	"testing"

	"github.com/thejerf/stresc"
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

		{"x%RAW", []interface{}{"y"}, "", stresc.ErrIncompleteFormatString},
		{"x%RAW;", []interface{}{0}, "", stresc.ErrRawUnknownType},
		{"x%RAW;", []interface{}{}, "x%RAW error: No arg;", nil},
		{"x%blargh;", []interface{}{}, "", stresc.ErrUnknownFormatter("blargh")},
	}

	i := stresc.NewInterpolator()

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
	i := stresc.NewInterpolator()

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
	if !reflect.DeepEqual(err, stresc.ErrAlreadyExists("cdata")) {
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
	i := stresc.NewInterpolator()

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
	i := stresc.NewInterpolator()

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

	stresc.ErrAlreadyExists("x").Error()
	stresc.ErrUnknownFormatter("x").Error()
}

type WriterAlwaysEOF struct{}

func (wae WriterAlwaysEOF) Write(b []byte) (n int, err error) {
	if len(b) > 0 {
		return 0, io.EOF
	}
	return 0, nil
}
