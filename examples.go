package strinterp

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"strconv"
)

// This file contains the examples of how to write Formatters and Encoders
// that ship with the library.

// WriterFunc is a type that wraps a function that implements the io.Writer
// interface with an implementation of calling it for .Write. This allows
// Encoders to easily return stateless functions as their implementation.
// See several examples in examples.go.
type WriterFunc func([]byte) (int, error)

// This implements the Write method of io.Writer.
func (wf WriterFunc) Write(b []byte) (int, error) {
	return wf(b)
}

// This is the simplest possible encoder, the "raw" encoder. It simply
// passes all bytes through. It ignores all parameters, and thus has
// no way to fail.
//
// This is private to this module and automatically included by all
// interpolators, but if you were going to register this yourself,
// it would be:
//
//  i.AddEncoder("RAW", raw)
func raw(inner io.Writer, params []byte) (io.Writer, error) {
	return WriterFunc(func(src []byte) (int, error) {
		return inner.Write(src)
	}), nil
}

// This next one is slightly more complicated, as it actually handles
// parameters. This actually returns an io.WriteCloser, but strinterp
// handles this correctly.

// Base64 defines an Encoder that implements base64 encoding.
//
// It takes as a parameter either "std" or "url", to select between
// Standard or URL base64 encoding. If no parameter is given, Standard is
// chosen. Any other parameter results in ErrUnknownArguments.
func Base64(w io.Writer, args []byte) (io.Writer, error) {
	encoding := base64.StdEncoding
	if args != nil {
		switch string(args) {
		case "std":
			// still uses StdEncoding, but does not yield
			// ErrUnknownArguments
		case "url":
			encoding = base64.URLEncoding
		default:
			return nil, ErrUnknownArguments{args, "can only be std or url, to indicate the standard or URL base64 encoding"}
		}
	}

	wc := base64.NewEncoder(encoding, w)
	return wc, nil
}

// JSON defineds a formatter that uses the standard encoding/json module to
// output JSON.
func JSON(w io.Writer, val interface{}, params []byte) error {
	e := json.NewEncoder(w)
	return e.Encode(val)
}

var hex = "0123456789abcdef"

var lt = []byte("&lt;")
var gt = []byte("&gt;")
var cr = []byte("&#13;")
var lf = []byte("&#10;")
var quot = []byte("&quot;")
var apos = []byte("&apos;")

// CDATA defines an HTML CDATA escaper, which is to say, the type of data
// that appears as "text" within HTML.
//
// There's a lot of history and browser variations here. By default this
// is a very aggressive encoding function suitable for use in all the
// parts of HTML that permit "CDATA" that I know of, including attribute
// values. (Some browsers do not like literal newlines in attributes,
// considering it to terminate the tag.) However, this aggression may
// result in difficult-to-read HTML. If you are outputting HTML text as
// text (as opposed to attribute values), you can pass the argument
// "nocrlf" to avoid encoding CR and LF as entities.
func CDATA(inner io.Writer, args []byte) (io.Writer, error) {
	var encodeCRLF = true
	if args != nil {
		if string(args) == "nocrlf" {
			encodeCRLF = false
		} else {
			return nil, ErrUnknownArguments{args, "only nocrlf allowed for CDATA"}
		}
	}

	return WriterFunc(func(by []byte) (n int, err error) {
		goodfrom := 0

		for idx, b := range by {
			// this if clause is "if this is a character we need to encode"
			if b == '<' || b == '>' ||
				(b < ' ' && (encodeCRLF || (b != '\n' && b != '\r'))) {
				if goodfrom != idx {
					_, err = inner.Write(by[goodfrom:idx])
				}
				goodfrom = idx + 1

				// emit the properly-encoded value
				switch b {
				case '<':
					_, err = inner.Write(lt)
				case '>':
					_, err = inner.Write(gt)
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
					num := "&#" + strconv.FormatInt(int64(b), 10) + ";"
					_, err = inner.Write([]byte(num))
				}

				if err != nil {
					return
				}
			}
		}

		n = len(by)
		if goodfrom <= n-1 {
			_, err = inner.Write(by[goodfrom:n])
		}

		return
	}), nil
}
