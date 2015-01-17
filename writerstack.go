package strinterp

import "io"

// A WriterStack allows us to wrap Encoders around a given io.Writer.
//
// WriterStack solves the problem of some of the Encoders potentially
// wanting to be .Close()d, even if the underlying io.Writer is not
// closable, or you do not wish to close the underlying writer so it
// can be reused later. This can, for instance, be seen in the base64 encoder
// shipped by this library. By calling .Finish() on this object
// you can safely use these Encoders. .Finish() should always be called
// to end a WriterStack's output.
//
// WriterStack can be used without any other strinterp functionality.
type WriterStack struct {
	io.Writer
	components []io.Writer
}

// NewWriterStack returns a new *WriterStack with the argument being used
// as the lowest-level writer.
func NewWriterStack(w io.Writer) *WriterStack {
	return &WriterStack{w, []io.Writer{}}
}

// Push wraps a writer on top of the stack, which will process any bytes
// and send them to any subsequent writers.
//
// If the Push returns an error, the WriterStack is no longer valid to use.
func (ws *WriterStack) Push(enc Encoder, args []byte) error {
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

// Finish will finish the WriterStack's work, which may flush intermediate
// encoders by calling .Close() on them. This will not close the base
// io.Writer.
func (ws *WriterStack) Finish() error {
	// need to close in reverse order for this to be correct, which
	// can be verified by reversing this and seeing the base64 test cases
	// fail
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
