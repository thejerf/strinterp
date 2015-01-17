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
// objects, such as JSON-encoding them. If your argument is anything
// other than a string, []byte, or io.Reader, you'll need a Formatter.
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
// == NotGiven, a singleton value used for this case.
//
// If the formatting could be completed successfully, the bytes should all
// be written to the io.Writer by the time the formatter returns. If the
// formatting could not be completed successfully, an error should be
// returned. In that case there are no guarantees about how much of the
// stream may have been written, which is fundamental to a stream-style
// library.
type Formatter func(io.Writer, interface{}, []byte) error

// ErrNotGiven will be passed to a Formatter as the value it is encoding,
// if the caller did not give enough arguments to the InterpStr or
// InterpWriter calls.
//
// This is public so your formatter can check for it.
var ErrNotGiven = errors.New("value not given")

var errIncompleteFormatString = errors.New("incomplete format string, no semi-colon found")

var errNoDefaultHandling = errors.New("no default encoder handling for type")

// ErrAlreadyExists is the error that is returned when you attempt to register
// a given format string when it has already been registered.
type errAlreadyExists string

// Error implements the Error interface on the ErrAlreadyExists error.
func (ae errAlreadyExists) Error() string {
	// FIXME: need to use the interpolator
	return "The format string " + string(ae) + " is already declared"
}

// ErrUnknownArguments is the error that is returned when you pass
// arguments to a formatter/encoder that it doesn't understand. This is
// public so your formatters and encoders can reuse it.
type ErrUnknownArguments struct {
	Arguments []byte
	ErrorStr  string
}

func (ua ErrUnknownArguments) Error() string {
	return "The arguments \"" + string(ua.Arguments) + "\" were invalid: " + ua.ErrorStr
}

// ErrUnknownFormatter is the error that will be returned by the interpolator
// when it encounters a format string it doesn't understand.
type errUnknownFormatter string

// Error implements the Error interface on the UnknownFormat error.
func (uf errUnknownFormatter) Error() string {
	// FIXME: Use the interpolator
	return "format string specified unknown formatter " + string(uf)
}

// ErrUnknownEncoder is the error that will be returned by the interpolator
// when it encouters an encoder string it doesn't understand.
type errUnknownEncoder string

// Error implements the Error interface on the UnknownEncoder error.
func (ue errUnknownEncoder) Error() string {
	return "format string specified unknown encoder " + string(ue)
}
