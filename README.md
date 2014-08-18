# strinterp - "String Interpolation"

Done "right", for suitably pedantic definitions of "right".

    go get github.com/thejerf/strinterp

A prototype library for interpolating strings in the manner I propose to be
more correct than the usual manners.

This code is posted in support of a blog post, which I recommend reading in
order to understand the design and the purpose of the design. At the
moment, I wouldn't particularly propose that you use it in real code; I
don't. After all this represents ~4 hours of screwing around rather than
something I'd ship directly. However, it does do what it does, so if you
are moved to use it, I won't object. As the LICENSE says, if it breaks you
get to keep both pieces. If enough pull requests come in to turn this into
a real library I won't complain.

The obvious next step to take, given my purpose in writing this, is to
allow chains of formats to be used. My tentative plan is to separate the
chains by forward slashes in the format specification, and tweak everything
to rewrite that into a series of io.Writer and io.Readers such that
everything can be chained in a stream. It exceeds my desires to write for a
blog post, but if I'm ever moved to use this directly it'll be added.