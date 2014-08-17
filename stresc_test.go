package stresc_test

import (
	"fmt"
	"testing"
	"reflect"

    "github.com/thejerf/stresc"
)

type StrescTest struct {
	Format string
	Args []interface{}
	Result string
	Error error
}

func TestDefaultInterpolator(t *testing.T) {
	noargs := []interface{}{}

	tests := []StrescTest{
		{"x", noargs, "x", nil},
		{"x%RAW;", []interface{}{"y"}, "xy", nil},
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
