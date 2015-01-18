strinterp - "String/Stream Interpolation"
=========================================

[![Build
Status](https://travis-ci.org/thejerf/strinterp.png?branch=master)](https://travis-ci.org/thejerf/strinterp)

Morally correct string/stream interpolation.

    go get github.com/thejerf/strinterp

This code is posted in support of a [blog post about why we continue to
write insecure software](http://www.jerf.org/iri/post/2942), which I
recommend reading in order to understand the design and the purpose of the
design. At the moment, I wouldn't particularly propose that you use it in
real code; I don't. After all this represents ~20 hours of screwing around
rather than something I'd ship directly. However, it does do what it does,
so if you are moved to use it, I won't object. As the LICENSE says, if it
breaks you get to keep both pieces. If enough pull requests come in to turn
this into a real library I won't complain.

But even as just some musings meant for support of a blog post, I could not
stand publishing this without the jerf-standard [full
godoc](http://godoc.org/github.com/thejerf/strinterp), including examples,
usage, and everything else you might otherwise expect this README.md to
cover on GitHub, plus full test coverage and golint-cleanliness.
