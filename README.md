This repository contains an experimental rewrite of Sandstorm.

Currently, most of the sandbox setup code is built, and sandstorm-next
is capable of spawning sandstorm-http-bridge based apps and plumbing
http traffic to them from the outside.

# Building

To build sandstorm-next, you will need:

- Go 1.19 or later
- [tinygo](https://tinygo.org/)
- Standard C development tools (make, a C compiler, etc).
- capnp (command line tool) version 0.8 or later.
- capnpc-go code generator plugin

You will also need to separately check out the source for go-capnp and
go.sandstorm:

```
mkdir ../deps
cd ../deps
git clone https://github.com/capnproto/go-capnproto2
git clone https://github.com/zenhack/go.sandstorm
cd -
```

Then, run the configure script and then `make`. The configure script
accepts
most of the same options as typical gnu packages. Additionally you will
need to supply the paths to the repositories checked out above:

```
./configure \
    --with-go-capnp=../deps/go-capnproto2 \
    --with-go-sandstorm=../deps/go.sandstorm
make
```

Then run `make install` to install sandstorm-next system wide.

Tip: you can configure sandstorm-next to share a grain/app storage
directory with a legacy sandstorm system by passing
`--localstatedir=/opt/sandstorm/var` to `./configure`.  In addition to
the files used by legacy sandstorm, `sandstorm-next` will create a
couple extra things underneath that path, namely:

- an extra directory at `sandstorm/mnt`
- a sqlite3 database at `sandstorm/sandstorm.sqlite3`

# Importing data from legacy sandstorm

Sandstorm-next comes with a tool to import some data from a legacy
sandstorm installation's database; after running `make`, there will be
an executable at `_build/sandstorm-legacy-tool`. On a typical sandstorm
server you can export the contents of the database via:

```
./_build/sandstorm-legacy-tool --snapshot-dir /desired/path/to/snapshot export
```

If your sandstorm installation is in a non-standard path or mongoDB is
listening on a different port, you may have to supply additional
options; see `sandstorm-legacy-tool --help` to see the full list.

You can then import the snapshot into sandstorm-next via:

```
./_build/sandstorm-legacy-tool --snapshot-dir /path/to/snapshot import
```

# Running

At present, sandstorm-next has no user interface, and no way to install
apps or create grains; to experiment with it you must separately arrange
a suitable grain storage directory and populate the database. The easiest
way to do this is to point it at an existing sandstorm installation, and
import database info using `sandstsorm-legacy-tool`, per above.

You will need to be a member of the `sandstorm` group to run
`sandstorm-next` (or if you specified a different group name via the
`--group` configure flag, that group instead).

On startup, the `sandstorm-next` executable will attempt to launch the
grain specified in the environment variable `DUMMY_GRAIN_ID`.

`sandstorm-next` will start a web server on port 8000; to connect to the
UI, go to `http://local.sandstorm.io:8000`.

This will display the grain's UI within an iframe. Things like
offer iframes and anything that uses sandstorm specific APIs will not
work currently.