package stresc

import (
	"bytes"
	"errors"
	"io"
)

// A Formatter is a function that does formatting.
//
// The []byte is any additional parameters passed via the colon mechanism,
// containing only those extra parameters (i.e., no colon or
// semicolon). Interpreting them is entirely up to the function. This is
// nil if no colon was used. (Note this can be distinguished from blank,
// though that seems like a bad idea. Note also the len of a nil slice is
// 0, which makes that the easiest thing to check.)
//
// interface{} is the value. If the value was not given to the interpolator
// at all (i.e., more format strings given than values), the value will be
// == NotGiven.
//
// If the formatting could be completed successfully, the []byte should
// contain the results. If it could not be completed successfully, the
// error should be returned. Since this library is ultimately for a blog
// post and not for deployment, the error will be returned if any formatted
// yields one. (In practice we should probably give a choice between doing
// what fmt does and just tossing it in anyhow and carrying on, and
// something like a MustInterpolateSuccessfully; both behaviors have their
// place and really shouldn't be casually mixed the way the fmt library
// does.)
//
// If the Formatter can handle a []byte, it can also be used as a filter.
type Formatter func(io.Writer, interface{}, []byte) error

// ErrNotGiven is returned if a formatter is given without a value.
var ErrNotGiven = errors.New("value not given")

// ErrIncompleteFormatString is returned if the format string had a % but no
// matching ;.
var ErrIncompleteFormatString = errors.New("incomplete format string (no semi-colon found)")

// ErrRawUnknownType is returned if the RAW formatter is passed something
// it doesn't understand.
var ErrRawUnknownType = errors.New("type is unknown to the raw encoder")

// ErrAlreadyExists is the error that is returned when you attempt to register
// a given format string when it has already been registered.
type ErrAlreadyExists string

// Error implements the Error interface on the ErrAlreadyExists error.
func (ae ErrAlreadyExists) Error() string {
	// FIXME: need to use the interpolator
	return "The format string " + string(ae) + " is already declared"
}

// ErrUnknownFormatter is the error that will be returned by the interpolator
// when it encounters a format string it doesn't understand.
type ErrUnknownFormatter string

// Error implements the Error interface on the UnknownFormat error.
func (uf ErrUnknownFormatter) Error() string {
	// FIXME: Use the interpolator
	return "format string specified unknown formatter " + string(uf)
}

// An Interpolator represents an object that can perform string
// interpolation.
//
// Interpolators are created via NewInterpolator.
//
// Interpolators are designed to be used via being initialized with all
// desired format string handlers in a single thread. Once initialized, the
// interpolator can be freely used in any number of threads. Once that
// occurs no more format strings should be added.
type Interpolator struct {
	interpolators map[string]Formatter
}

// NewInterpolator returns a new Interpolator, with only the default load
// of interpolation primitives.
//
// These are:
//
//    "%": Magical, and yields a literal % without consuming an arg
//    "RAW": interpolates the given string, []byte, or io.Reader directly
//      (if an io.Reader, io.Copy is used)
//
// As a bonus observation, note how this interpolator is also potentially
// useful for streaming in a way your average string interpolator is
// not. Where you'd be stupid to fmt.Sprintf up a few megabytes, this
// approach can handle that via streaming just fine....
func NewInterpolator() *Interpolator {
	return &Interpolator{map[string]Formatter{
		"RAW": raw,
	}}
}

// AddFormat adds a interpolation format to the interpolator.
func (i *Interpolator) AddFormat(format string, handler Formatter) error {
	if i.interpolators[format] != nil {
		return ErrAlreadyExists(format)
	}

	i.interpolators[format] = handler

	return nil
}

// InterpStr is a convenience function that does interpolation on a format
// string and returns the resulting string.
func (i *Interpolator) InterpStr(format string, args ...interface{}) (string, error) {
	buf := new(bytes.Buffer)
	err := i.InterpWriter(buf, []byte(format), args...)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// InterpWriter interpolates the format []byte into the passed io.Writer.
func (i *Interpolator) InterpWriter(w io.Writer, formatBytes []byte, args ...interface{}) error {
	buf := bytes.NewBuffer(formatBytes)
	for {
		untilDelim, err := readBytesUntilUnescDelim(buf, 37) // 37 = %
		if err == io.EOF {
			// FIXME: Real code ought to do something with remaining unused
			// args
			_, err = w.Write(untilDelim)
			return err
		}

		_, err = w.Write(untilDelim)
		if err != nil {
			return err
		}

		rawFormat, err := readBytesUntilUnescDelim(buf, 59) // 59 = ;
		if err == io.EOF {
			return ErrIncompleteFormatString
		}

		formatChunks := bytes.SplitN(rawFormat, []byte(":"), 2)
		format := string(formatChunks[0])
		var formatArgs []byte
		if len(formatChunks) > 1 {
			formatArgs = formatChunks[1]
		}

		// implement the special % escaper
		if len(format) == 1 && format[0] == 37 {
			_, err = w.Write([]byte{37})
			if err != nil {
				return err
			}
		} else {
			formatter := i.interpolators[format]
			if formatter == nil {
				return ErrUnknownFormatter(string(format))
			}

			if len(args) > 0 {
				thisArg := args[0]
				args = args[1:]

				err = formatter(w, thisArg, formatArgs)
				if err != nil {
					return err
				}
			} else {
				_, err = w.Write([]byte("%" + string(format) + " error: No arg;"))
				if err != nil {
					return err
				}
			}
		}
	}
}

func raw(w io.Writer, x interface{}, args []byte) error {
	switch arg := x.(type) {
	case []byte:
		_, err := w.Write(arg)
		return err
	case string:
		_, err := w.Write([]byte(arg))
		return err
	case io.Reader:
		_, err := io.Copy(w, arg)
		return err
	default:
		return ErrRawUnknownType
	}
}

// like bytes.ReadBytes, except it goes until the target byte is found not
// preceded by a \, and eliminates the backslashes out of the result.
//
// This is basically the simplest possible correct form of backslash
// escaping. If it seems like overkill, bear in mind it is very simple and
// easy to understand, and a lot of the "corrections" that leap to people's
// minds are actually very complicated to implement *correctly*.
//
// In particular, we throw away a backslash if it is the last character,
// just so we don't end up with a corner case where a single backslash
// survives.
//
// This does not return the delimiter.
func readBytesUntilUnescDelim(buf *bytes.Buffer, delim byte) ([]byte, error) {
	result := []byte{}

	for {
		b, err := buf.ReadByte()
		if err != nil {
			return result, err
		}

		if b == 92 { // the backslash tells us to blindly read in the next byte
			b, err = buf.ReadByte()
			if err != nil {
				return result, err
			}
			result = append(result, b)
		} else if b == delim {
			return result, nil
		} else {
			result = append(result, b)
		}
	}
}
