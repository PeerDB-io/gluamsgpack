package gluamsgpack

import (
	"testing"
	"time"

	"github.com/yuin/gopher-lua"
)

const code = `
assert(mp.encode(mp.raw"asdf") == "asdf")
assert(mp.encode(nil) == string.char(0xc0))
assert(mp.encode(true) == string.char(0xc2))
assert(mp.encode(false) == string.char(0xc3))
assert(mp.encode(10) == string.char(10))
assert(mp.encode(mp.array{}) == string.char(0x90))
assert(#mp.encode(10.5) == 5)
assert(#mp.encode(10.1) == 9)
local timecode = mp.encode(time)
assert(#timecode == 10)
assert(string.byte(timecode, 1), 0xd6)
assert(string.byte(timecode, 2), 0xff)
assert(mp.encode(mp.array(nil)) == string.char(0xc0))
local t1 = setmetatable({}, {__msgpack = function() return 1 end})
assert(mp.encode(t1) == string.char(0x1))
local t2 = setmetatable({}, {__msgpack = function() return t1 end})
assert(mp.encode(t2) == string.char(0x1))
`

func Test(t *testing.T) {
	ls := lua.NewState(lua.Options{})
	Loader(ls)
	ls.Env.RawSetString("mp", ls.Get(1))
	ls.SetTop(0)

	ls.Env.RawSetString("time", &lua.LUserData{
		Value:     time.Date(1996, time.February, 29, 10, 20, 30, 40, time.UTC),
		Env:       ls.Env,
		Metatable: nil,
	})

	if err := ls.DoString(code); err != nil {
		t.Log(err)
		t.FailNow()
	}
}