# Making a release #

Compile and test

Then run

  make test_release

To test the build

When happy, tag the release

  git tag -a 0.0XX -m "Release 0.0XX"

Then do a release build (set GITHUB token first)

  make release
