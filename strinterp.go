package strinterp

import (
	"bytes"
	"io"
)

// An Interpolator represents an object that can perform string
// interpolation.
//
// Interpolators are created via NewInterpolator.
//
// Interpolators are designed to be used via being initialized with all
// desired format string handlers in a single goroutine. Once initialized,
// the interpolator can be freely used in any number of goroutines.
type Interpolator struct {
	formatters map[string]Formatter
	encoders   map[string]Encoder
}

/*

An Encoder is a function that takes an "inner" io.Writer and returns
an io.Writer that wraps that writer, such that calls to the returned
Writer will produce the desired encoding behavior. See examples.go.

In addition to conforming to the io.Writer interface, Encoders must
also never cut up Unicode characters between calls. This technically
means that existing io.Writer transformers *may* not conform to this
interface, though most if not all probably do by accident. Encoders
thus may also count on the fact that they will not receive partial Unicode
characters, which may permit stateless Encoders to be written. This
is facilitated with the provided WriteFunc type as well.

*/
type Encoder func(io.Writer, []byte) (io.Writer, error)

// NewInterpolator returns a new Interpolator, with only the default load
// of interpolation primitives.
//
// These are:
//
//    "%": Yields a literal % without consuming an arg
//    "RAW": interpolates the given string, []byte, or io.Reader directly
//      (if an io.Reader, io.Copy is used)
func NewInterpolator() *Interpolator {
	return &Interpolator{
		map[string]Formatter{},
		map[string]Encoder{
			"RAW": raw,
		},
	}
}

// NewDefaultInterpolator returns a new Interpolator set up with some more
// format strings available:
//
//  json: the JSON formatter
//  base64: the Base64 encoder
//  cdata: the HTML CDATA encoder
//
// More things may be added in future versions of this library. The safest
// long-term thing to do is to use NewInterpolator and configure it
// yourself. But this is convenient for demos and such.
func NewDefaultInterpolator() *Interpolator {
	return &Interpolator{
		map[string]Formatter{
			"json": JSON,
		},
		map[string]Encoder{
			"RAW":    raw,
			"cdata":  CDATA,
			"base64": Base64,
		},
	}
}

// AddFormatter adds a interpolation format to the interpolator.
//
// If the format string is already registered, an error will be returned.
func (i *Interpolator) AddFormatter(format string, handler Formatter) error {
	if i.formatters[format] != nil {
		return errAlreadyExists(format)
	}
	if i.encoders[format] != nil {
		return errAlreadyExists(format)
	}

	i.formatters[format] = handler

	return nil
}

// AddEncoder adds an encoder type to the interpolator.
//
// If the format string is already registered, an error will be returned.
func (i *Interpolator) AddEncoder(format string, handler Encoder) error {
	if i.formatters[format] != nil {
		return errAlreadyExists(format)
	}
	if i.encoders[format] != nil {
		return errAlreadyExists(format)
	}

	i.encoders[format] = handler

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
		untilDelim, err := readBytesUntilUnescDelim(buf, '%')
		if err == io.EOF {
			// FIXME: Real code ought to do something with remaining unused
			// args, like fmt does
			_, err = w.Write(untilDelim)
			return err
		}

		_, err = w.Write(untilDelim)
		if err != nil {
			return err
		}

		rawFormat, err := readBytesUntilUnescDelim(buf, ';')
		if err == io.EOF {
			return errIncompleteFormatString
		}

		// implement the special % escaper
		if len(rawFormat) == 1 && rawFormat[0] == '%' {
			_, err = w.Write([]byte("%"))
			if err != nil {
				return err
			}
			continue
		}

		formatSpecs := splitHonoringEscaping(bytes.NewBuffer(rawFormat), '|')

		writer := NewWriterStack(w)

		// if there are encoders in the specification, we construct them
		// backwards so as to properly modify the underlying writer.
		for j := len(formatSpecs) - 1; j >= 1; j-- {
			encoder, formatArgs, err := i.parseEncoder(formatSpecs[j])
			if err != nil {
				return err
			}

			err = writer.Push(encoder, formatArgs)
			if err != nil {
				return err
			}
		}

		thisFormatter := formatSpecs[0]
		formatChunks := bytes.SplitN(thisFormatter, []byte(":"), 2)
		format := string(formatChunks[0])
		var formatArgs []byte
		if len(formatChunks) > 1 {
			formatArgs = formatChunks[1]
		}

		// If the first string specifies a "formatter", then we go ahead
		// and just pass off the argument to the formatter and we're done
		// with it. If the first thing specifies an "encoder", then we
		// convert the argument to something that we can "Write" with
		// ourselves. If it's neither, well, that's a problem.
		formatter := i.formatters[format]
		encoder := i.encoders[format]

		if formatter == nil && encoder == nil {
			return errUnknownFormatter(format)
		}

		var thisArg interface{}
		if len(args) > 0 {
			thisArg = args[0]
			args = args[1:]
		} else {
			thisArg = NotGiven
		}

		if formatter != nil {
			err = formatter(writer, thisArg, formatArgs)
			err2 := writer.Finish()
			if err != nil {
				return err
			}
			if err2 != nil {
				return err2
			}
		}
		if encoder != nil {
			err = writer.Push(encoder, formatArgs)
			if err != nil {
				return err
			}
			err = i.writeArgument(thisArg, writer)
			err2 := writer.Finish()
			if err != nil {
				return err
			}
			if err2 != nil {
				return err2
			}
		}
	}
}

// this is the default specification of how to write "something" if an
// encoder is passed as the first argument to a format string. If you
// need something else sensible in here, please send a pull request and
// I'll be happy to incorporate anything that is Go-standard. If you need
// something super-custom let me know and I'll work in a way for you to
// hook into this.
func (i *Interpolator) writeArgument(a interface{}, w io.Writer) error {
	switch arg := a.(type) {
	case string:
		_, err := w.Write([]byte(arg))
		return err
	case []byte:
		_, err := w.Write(arg)
		return err
	case NotGivenType:
		return ErrNotGiven
	}

	reader, isReader := a.(io.Reader)
	if isReader {
		_, err := io.Copy(w, reader)
		return err
	}

	return errNoDefaultHandling
}
