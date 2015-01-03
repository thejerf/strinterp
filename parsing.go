package strinterp

import (
	"bytes"
	"io"
)

// This file contains misc. details related to parsing the formatting
// parameters, etc.

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

// This function works like "split" but honors escaping.
func splitHonoringEscaping(buf *bytes.Buffer, delim byte) [][]byte {
	result := [][]byte{}

	var err error

	for err == nil {
		var b []byte
		b, err = readBytesUntilUnescDelim(buf, delim)
		if err == nil || err == io.EOF {
			result = append(result, b)
		}
	}

	return result
}

func (i *Interpolator) parseEncoder(formatSpec []byte) (Encoder, []byte, error) {
	formatChunks := bytes.SplitN(formatSpec, []byte(":"), 2)
	format := string(formatChunks[0])
	var formatArgs []byte
	if len(formatChunks) > 1 {
		formatArgs = formatChunks[1]
	}

	encoder := i.encoders[format]
	if encoder == nil {
		return nil, nil, ErrUnknownEncoder(format)
	}
	return encoder, formatArgs, nil
}
