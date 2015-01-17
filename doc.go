/*

Package strinterp provides a demonstration of morally correct string
interpolation.

This package was created in support of a blog post. It's the result of
about 20 hours of screwing around. I meant to keep it shorter, but
I started to have too much fun.

"Morally" correct means that I intend this to demonstrate a point about
API and language design, and that any actual utility is a bit
coincidental.

That said, as this developed it became potentially more useful than
I had initially intended, because instead of expressing all
the interpolations in terms of strings, they are all expressed in
terms of io.Writers. Since this library also permits inputting
the strings to be interpolated in the form of io.Readers, this means
that this entire library is fully capable of string interpolation in
the middle of streams, not just strings. Or, if you prefer, this is
a *stream* interpolator. The "str" in "strinterp" is pleasingly
ambiguous.

This documentation focuses on usage; for the reasoning behind the
design, consult the blog post.

Using String Interpolators

To use this package, create an interpolator object:

    i := strinterp.NewInterpolator()

You can then use it to interpolate strings. The simplest case is
concatenation:

    concated, err := i.InterpStr("concatenated: %RAW;%RAW;", str1, str2)

See the blog post for a discussion of why this is deliberately a bit
heavyweight and *designed* to call attention to the use of "RAW", rather
than making such usage a simple and quiet default behavior.

The "format string", the first element of the call, has the following
syntax:

    * Begins with %, ends with unescaped ;
    * Begins with the formatter/encoder name
    * Which may be followed by a colon, then args for that formatter
    * Which may then be followed by a pipe, and further specifications
      of encoders with optional arguments

You may backslash-escape any of the pipe, colon, or semicolon to pass them
through as arguments to the formatter/encoder, or backslash itself to pass
it through. (The formatter/encoder will of course receive the decoded
bytes without the escaping backslash.) To emit a raw %, use "%%;".

Here is an example of a format string that uses all these features:

    result, err := i.InterpStr("copy and paste: %json|base64:url;", obj)

This will result in the standard encoding/json encoding being used on the
obj, then it will be converted to base64, which will use the encoding/base64
URLEncoding due to the "url" argument being passed. You can continue
piping to further encoders indefinitely.

There are two different kinds of interpolators you can write, formatters
and encoders.

Formatters

A "formatter" is a routine that takes a Go value of some sort and
converts it to some bytes to be written out via a provided io.Writer.
A formatter has the function signature defined by the Formatter type,
which is:

    func (w io.Writer, arg interface{}, params []byte) error

When called, the function should first examine the parameters. If it
doesn't like the parameters, it should return ErrUnknownArguments,
properly filled out. (Note: It is important to be strict on the
parameters; if they don't make perfect sense, this is your only chance
to warn a user about that.) It should then take the arg and write it
out to the io.Writer in whatever manner makes sense, then return either
the error obtained during writing or nil if it was fully successful.

You want to write a Formatter when you are trying to convert something
that isn't already a string, []byte, or io.Reader into output.
Therefore it only makes sense in the first element of a formatter's
pipeline (the "json" in the previous example), because only a
formatter can handle arbitrary objects.

See the Formatter documentation below for more gritty details.

Encoders

An "encoder" is a routine that receives incoming io.Writer requests,
modifies them in a suitable manner, and passes them down to the next
io.Writer in the chain. In other words it takes []byte and generates
further []byte from them.

You want to write an Encoder when either you want to transform input
going through it (like escaping), or when you know the only valid
input coming in will be in the form of a string, []byte, or io.Reader,
which strinterp will automatically handle feeding down the encoder
pipeline.

See the Encoder documentation below for more gritty details.

Configuring Your Interpolators

To configure your interpolator, you will need to add additional
formatters and encoders to the interpolator so it is aware of them.
NewInterpolator will return a bare *Interpolator with only the "RAW"
encoder. A DefaultInterpolator is also provided that comes preconfigured
for some HTML- and JSON-type-tasks. Consulting the "examples.go" file
in the godoc file listing below will highlight these formatters
and interpolators for your cribbing convenience.

Use the AddFormatter and AddEncoder functions to add these to your
interpolator to configure it.

(Since I find people often get a sort of mental block around this,
remember that, for instance, even though I provide you a default JSON
streamer based on the standard encoding/json library, if you have
something else you prefer, you can always specify a *different*
json formatter for your own usage.)

Once configured, for maximum utility I recommend putting string
interpolation into your environment object. See
http://www.jerf.org/iri/post/2929 .

Direct Encoder Usage

It is also possible to directly use the Encoders, as their type signature
tends to imply (note how you don't have to pass them any *Interpolator
or any other context). Ideally you instantiate a WriterStack around your
target io.Writer and .Push encoders on top of that, as WriterStack handles
some corner cases around Encoders that want to be "Close"d, then call
.Finish() on the WriterStack when done, which DOES NOT close the
underlying io.Writer. This is probably the maximally-performing way to
do this sort of encoding in a stream.

Security Note

This is true of all string interpolators, but even more so of
strinterp since it can be hooked up to arbitrary formatters and
encoders. You MUST NOT feed user input as the interpolation source
string. In fact I'd suggest that one could make a good case that the first
parameter to strinterp should always be a constant string in the source
code base, and if I were going to write a local validation routine to plug
into go vet or something I'd probably add that as a rule.

Again, let me emphasize, this is NOT special to strinterp. You shouldn't
let users feed into the first parameter of fmt.Sprintf, or any other such
string, in any language for that matter. It's possible some are "safe" to
do that in, but given the wide range of havoc done over the years by
letting users control interpolation strings, I would just recommend against
it unconditionally. Even when "safe" it probably isn't what you mean.

Care should also be taken in the construction of filters. If they get much
"smarter" than a for loop iterating over bytes/runes and doing "something" with
them, you're starting to ask for trouble if user input passes through
them. Generally the entire point of strinterp is to handle potentially
untrusted input in a safe manner, so if you start "interpreting" user input
you could be creating openings for attackers.

Contributing

I'm interested in pull requests for more Formatters and Encoders for the
"default Interpolator", though ideally only for things in the standard
library.

*/
package strinterp
