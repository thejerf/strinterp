package strinterp

import "io"

// The "writerStack" allows us to track what writers are being added by
// Encoders, so we have the opportunity to .Close them at the end of use,
// if it turns out they can be so closed.

type writerStack struct {
	io.Writer
	components []io.Writer
}

func (ws *writerStack) push(enc Encoder, args []byte) error {
	var err error
	ws.Writer, err = enc(ws.Writer, args)
	if err != nil {
		return err
	}
	// note this DELIBERATELY does not end up including the "base writer"
	// that we're interpolating to... we do NOT want to end up closing
	// that, too!
	ws.components = append(ws.components, ws.Writer)
	return nil
}

// This implements io.Closer on the stack, allowing for compatibility with
// things like Base64.
func (ws *writerStack) Close() error {
	// need to close in reverse order for this to be correct
	for i := len(ws.components) - 1; i >= 0; i-- {
		closer, isCloser := ws.components[i].(io.Closer)
		if isCloser {
			err := closer.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}
