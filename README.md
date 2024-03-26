[msgpack](https://msgpack.org) encoding library for [gopher-lua](https://github.com/yuin/gopher-lua) in PeerDB

This follows many ideas from [gluajson](https://github.com/PeerDB-io/gluajson)

For now there's no unmark function, as most marker types do not have a clear & precise Lua value

We also don't have a ubiquitous MarshalJSON trait to rely on,
instead custom encodings must be done with a Packer interface:
```go
type Packer interface {
  PackMsg([]byte) []byte
}
```
Which is passed the entire msgpack buffer & should append accordingly, returning the result

----

`decode` is not currently implemented, only `encode`

Lua strings are checked, if valid utf8 they are encoded as str, otherwise as bin

`encode` checks UserData for a `__msgpack` metamethod,
the result of which is encoded

If no `__msgpack` metamethod exists, the Value is checked. If it implements the `Packer` interface, then it is invoked. Otherwise the following types have predictable implementations:

* `string` *(not checked for valid utf8)*
* `[]byte`
* `uint64`
* `int64`
* `time.Time`

There exists the following marker methods:

* `raw` takes a string
* `array`, `map` takes a table *(useful for encoding empty arrays)*
* `bin`, `str` takes a string
* `signed`, `unsigned` takes a number, or parses string
* `f32`, `f64` takes a number
* `time`, `time32`, `time64`, `time96` takes number representing time since unix epoch in seconds, or string with optional format (default RFC3339), or UserData with time.Time value
* `ext` takes a number for type & string for bytes