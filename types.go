package strinterp

import (
	"errors"
	"io"
)

// This file bundles together all the various type definitions and such
// that otherwise clutter up the understanding of the strinterp.go file
// itself.

// NotGivenType uniquely identifies the token passed to formatters when
// an argument is not given for the formatter.
type NotGivenType struct{}

// NotGiven is the token passed to the formatters to indicate the value
// was not given. This distinguishes the value from "nil", which may well
// be perfectly legitimate.
var NotGiven = NotGivenType{}

// A Formatter is a function that takes the argument interface{} and writes
// the corresponding bytes to the io.Writer, based on the arguments. This
// is generally useful for doing non-trivial transforms on arbitrary
// objects, such as JSON-encoding them. Generally if you *can* express your
// target transform as an Encoder rather than a Formatter, you
// *should*. When you can't, create a Formatter.
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
// If the formatting could be completed successfully, the bytes should all
// be written to the io.Writer by the time the formatter returns. If the
// formatting could not be completed successfully, an error should be
// returned. Since this is still ultimately a blog post rather than
// proved production-quality library, if formatting fails, the
// interpolation will cease, but if it was interpolating to a live stream
// like a socket or something, could result in a half-a-stream being generated.
type Formatter func(io.Writer, interface{}, []byte) error

// ErrNotGiven is returned if a formatter is given without a value.
var ErrNotGiven = errors.New("value not given")

// ErrIncompleteFormatString is returned if the format string had a % but no
// matching ;.
var ErrIncompleteFormatString = errors.New("incomplete format string, no semi-colon found")

// ErrNoDefaultHandling is returned when you pass something to an encoder
// being used as a formatter (first element of a format string), but
// strinterp has no default handling for that type.
var ErrNoDefaultHandling = errors.New("no default encoder handling for type")

// ErrAlreadyExists is the error that is returned when you attempt to register
// a given format string when it has already been registered.
type ErrAlreadyExists string

// Error implements the Error interface on the ErrAlreadyExists error.
func (ae ErrAlreadyExists) Error() string {
	// FIXME: need to use the interpolator
	return "The format string " + string(ae) + " is already declared"
}

// ErrUnknownArguments is the error that is returned when you pass
// arguments to a formatter/encoder that it doesn't understand.
type ErrUnknownArguments struct {
	Arguments []byte
	ErrorStr  string
}

func (ua ErrUnknownArguments) Error() string {
	return "The arguments \"" + string(ua.Arguments) + "\" were invalid: " + ua.ErrorStr
}

// ErrUnknownFormatter is the error that will be returned by the interpolator
// when it encounters a format string it doesn't understand.
type ErrUnknownFormatter string

// Error implements the Error interface on the UnknownFormat error.
func (uf ErrUnknownFormatter) Error() string {
	// FIXME: Use the interpolator
	return "format string specified unknown formatter " + string(uf)
}

// ErrUnknownEncoder is the error that will be returned by the interpolator
// when it encouters an encoder string it doesn't understand.
type ErrUnknownEncoder string

// Error implements the Error interface on the UnknownEncoder error.
func (ue ErrUnknownEncoder) Error() string {
	return "format string specified unknown encoder " + string(ue)
}
