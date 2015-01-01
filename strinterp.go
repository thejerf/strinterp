/*

Package strinterp provides a demonstration of morally correct string
interpolation.

"Morally" correct means that it may not be any other kind of correct,
including "free of bugs", "fast", or "useful". But this package does
offer you the opportunity to feel warm, fuzzy feelings for using it.
And how many packages offer you that?

This package is created in support of a blog post. It's the result of about
8 hours of screwing around. But it does what it does, and I could not bear
the thought of putting anything up on GitHub without full test coverage, so
if you feel you are interested in using it, it's not necessarily a bad
idea... I'd just suggest going into it knowing that you're probably going
to want to create a few pull requests.

It may well be the only "string interpolator" that can use io.Writers
for streaming... though this ought to be propogated even more deeply than
it has. Mail me if you're interested.

Using String Interpolators

To use this package, create an interpolator object:

    i := strinterp.NewInterpolator()

Add any additional interpolators you may wish. For instance, assuming you
import the html package, here's how to add a "HTML CDATA" interpolator:

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

And interpolate strings:

    s, err := i.InterpStr("hello %cdata;", "<world>")
    // yields "hello &lt;world&gt;"

The format is percent sign, the identifier of the interpolator, an optional
colon followed by any additional parameters for the interpolator, and
a semicolon. Colon and semicolon can be backslash-escaped to avoid their
active meanings (in which case backtick strings may be more useful). The
optional parameters are passed to the interpolation function as the
"params"; it is entirely up to the function to determine what they mean.

Features:

 * use of streams should be more deeply propagated
 * InterpStr probably can lose its "error" return
 * profile and performance review
 * it would be useful to be able to chain formatters, as mentioned in blog postxs

Security Note

This is true of all string interpolators, but even more so of
strinterp. You MUST NOT feed user input as the interpolation source
string. In fact I'd suggest that one could make a good case that the first
parameter to strinterp should always be a constant string in the source
code base, and if I were going to write a local validation routine to plug
into go vet or something I'd probably add that as a rule.

Again, let me emphasize, this is NOT special to strinterp. You shouldn't
let users feed into the first parameter of fmt.Sprintf, or any other such
string, in any language for that matter. It's possible some are "safe" to
do that in, but given the wide range of havoc done over the years by
letting users control interpolation strings, I would just recommend against
it unconditionally.

Care should also be taken in the construction of filters. If they get much
"smarter" than a for loop iterating over runes and doing "something" with
them, you're starting to ask for trouble if user input passes through
them. Generally the entire point of strinterp is to handle potentially
untrusted input in a safe manner, so if you start "interpreting" user input
you could be creating openings for attackers.

*/
package strinterp

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strconv"
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
//
// The only error that can result is a ErrAlreadyExists object.
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

// A RuneWriter is very much like a Writer, except with the guarantee that
// the written []byte will only break along legal boundaries for Runes,
// that is, it is guaranteed not to write a *partial* UTF-8
// character. RuneWriters are permitted to return errors if a partial UTF-8
// character is encountered, or may replace it with the "Unicode
// replacement character", as it sees fit.
//
// This allows the easy implementation of stateless transformers, and
// simplifies implementations of stateful transformers.
//
// RuneWriters must also be guaranteed to process the full []byte string
// passed to them, so we skip the return bit that reports the length of the
// written bytes.
//
// FIXME: Benchmark this to see if there is a significant difference
// between for loops that WriteRunes one at a time vs. ones that collect as
// much as they can that is legal before they WriteRunes.
type RuneWriter interface {
	WriteRunes([]rune) (err error)
}

var ErrIncompleteRune = errors.New("incomplete rune passed to RuneWriter")

type RuneWriterFunc func([]rune) (err error)

func (rwf RuneWriterFunc) WriteRunes(r []rune) (err error) {
	return rwf(r)
}

var lt = []rune{'&', 'l', 't', ';'}
var gt = []rune{'&', 'g', 't', ';'}
var cr = []rune{'&', '#', '1', '0', ';'}
var lf = []rune{'&', '#', '1', '3', ';'}
var quot = []rune{'&', 'q', 'u', 'o', 't', ';'}
var apos = []rune{'&', 'a', 'p', 'o', 's', ';'}

// bufioRuneWriters converts from the RuneWriter stream of runes into an
// io.Writer, via a bufio.Writer for buffering so we don't hammer the
// Writer on the other end.
type bufioRuneWriter struct {
	*bufio.Writer
}

func (brw bufioRuneWriter) WriteRunes(runes []rune) (err error) {
	for _, r := range runes {
		_, err = brw.WriteRune(r)
		if err != nil {
			return err
		}
	}
	return err
}

// byteBufferRuneWriter converts from the RuneWriter stream of runes into a
// bytes.Buffer, suitable for conversion to a string once collected.
type byteBufferRuneWriter struct {
	*bytes.Buffer
}

func (bbrw byteBufferRuneWriter) WriteRunes(runes []rune) (err error) {
	for _, r := range runes {
		// per documentation on bytes.WriteRune, this will never return an
		// err; it just matches bufio.Writer's WriteRune signature.
		_, _ = bbrw.WriteRune(r)
	}
	return nil
}

// In theory, CDATA is just "character data", with bits occasionally carved
// out for things like attributes. In reality, there has historically been
// a lot of fuzzy edge cases around newlines in attributes and such. In
// principle, a single encoding function could be defined that would handle
// all of these cases, but as that encoding function would then be forced
// to turn newlines into character entities, the resulting HTML would be
// quite ugly. Therefore, despite the fact it is theoretically possible to
// produce one encoding function for all HTML CDATA that simply
// conservatively encodes all controversial characters, in practice you
// want at least two, one for HTML text that leaves newlines alone, and one
// for attributes.
//
// Also the official rules have changed over the years, plus the browsers
// have chewed these up, so this is quite conservative. It converts both
// angle brackets and all control characters except CR and LF.
func encodeHTMLAttribute(inner RuneWriter) RuneWriter {
	return RuneWriterFunc(func(runes []rune) (err error) {
		goodfrom := 0

		for idx, r := range runes {
			// "below 32" is a control char
			if r <= '>' &&
				((r == '<' || r == '>') || (r < ' ' && r != '\n' && r != '\r')) {
				// this character is bad, needs encoding

				// emit anything good that we have
				if goodfrom != idx {
					err = inner.WriteRunes(runes[goodfrom:idx])
					goodfrom = idx + 1
				}

				// emit the properly-encoded value
				switch r {
				case '<':
					err = inner.WriteRunes(lt)
				case '>':
					err = inner.WriteRunes(gt)
				case '\n':
					err = inner.WriteRunes(lf)
				case '\r':
					err = inner.WriteRunes(cr)
				default:
					// this could be made more efficient with even nastier
					// code, probably
					// if this seems hypocritical, because I am here
					// concatenating strings, bear in mind that it's not a
					// great idea to implement strinterp in terms of
					// itself, both for performance reasons and for code
					// sanity reasons, and this is, after all, the language
					// primitive that is supported by the core
					// environment. So it's legal to wrap this
					// functionality safely (note how easy it is to
					// characterize the nature of the output of FormatInt
					// here, for instance, as opposed to uncontrolled
					// string concatenation). This is like how in any
					// "safe" environment you can never really get away
					// from "unsafe" code; it is a matter of confining it
					// and minimizing it, rather than trying to write
					// entirely "safely". In the string interpolators
					// themselves, you have license to *carefully* write
					// the unsafe code if you need to, vet it once, and
					// forget about it after that.
					num := "&#" + strconv.FormatInt(int64(r), 10) + ";"
					err = inner.WriteRunes(bytes.Runes([]byte(num)))
				}

				if err != nil {
					return
				}
			}
		}

		runecount := len(runes)
		if goodfrom < runecount-1 {
			err = inner.WriteRunes(runes[goodfrom:runecount])
		}

		return
	})
}

func encodeHTMLCDATA(inner RuneWriter) RuneWriter {
	return RuneWriterFunc(func(runes []rune) (err error) {
		return err
	})
}
