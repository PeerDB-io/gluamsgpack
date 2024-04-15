package gluamsgpack

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/yuin/gopher-lua"
)

var be = binary.BigEndian

type Packer interface {
	PackMsg([]byte) []byte
}

type Raw string
type Array *lua.LTable
type Map *lua.LTable
type Str string
type Bin string
type Signed int64
type Unsigned uint64
type F32 float32
type F64 float64
type Time32 time.Time
type Time64 time.Time
type Time96 time.Time
type Time time.Time
type Ext struct {
	x    int8
	data string
}

func newRaw(ls *lua.LState) Raw {
	return Raw(ls.CheckString(1))
}

func newArray(ls *lua.LState) Array {
	return Array(ls.OptTable(1, nil))
}

func newMap(ls *lua.LState) Map {
	return Map(ls.OptTable(1, nil))
}

func newStr(ls *lua.LState) Str {
	return Str(ls.CheckString(1))
}

func newBin(ls *lua.LState) Bin {
	return Bin(ls.CheckString(1))
}

func newSigned(ls *lua.LState) Signed {
	return Signed(ls.CheckNumber(1))
}

func newUnsigned(ls *lua.LState) Unsigned {
	return Unsigned(ls.CheckNumber(1))
}

func newF32(ls *lua.LState) F32 {
	return F32(ls.CheckNumber(1))
}

func newF64(ls *lua.LState) F64 {
	return F64(ls.CheckNumber(1))
}

func newTime(ls *lua.LState) Time {
	switch v := ls.Get(1).(type) {
	case lua.LNumber:
		s, subs := math.Modf(float64(v))
		return Time(time.Unix(int64(s), int64(subs*1e9)))
	case lua.LString:
		format := ls.OptString(2, time.RFC3339)
		t, err := time.Parse(format, string(v))
		if err == nil {
			return Time(t)
		}
		ls.RaiseError(err.Error())
	case *lua.LUserData:
		if t, ok := v.Value.(time.Time); ok {
			return Time(t)
		}
		ls.RaiseError("Cannot convert non-time userdata to time")
	default:
		ls.RaiseError("Cannot convert to time: " + v.Type().String())
	}
	return Time(time.Time{})
}

func newTime32(ls *lua.LState) Time32 {
	return Time32(newTime(ls))
}

func newTime64(ls *lua.LState) Time64 {
	return Time64(newTime(ls))
}

func newTime96(ls *lua.LState) Time96 {
	return Time96(newTime(ls))
}

func newExt(ls *lua.LState) Ext {
	return Ext{
		x:    int8(ls.CheckNumber(1)),
		data: string(ls.CheckString(2)),
	}
}

func marker[T any](f func(ls *lua.LState) T) func(ls *lua.LState) int {
	return func(ls *lua.LState) int {
		ls.Push(&lua.LUserData{
			Value:     f(ls),
			Env:       ls.Env,
			Metatable: nil,
		})
		return 1
	}
}

func (s Raw) PackMsg(buf []byte) []byte {
	return append(buf, s...)
}

func (s Str) PackMsg(buf []byte) []byte {
	l := len(s)
	switch {
	case l < 32:
		buf = append(buf, 0xa0|byte(l))
	case l < 0x100:
		buf = append(buf, 0xd9, byte(l))
	case l < 0x10000:
		buf = append(buf, 0xda)
		buf = be.AppendUint16(buf, uint16(l))
	default:
		buf = append(buf, 0xdb)
		buf = be.AppendUint32(buf, uint32(l))
	}
	return append(buf, s...)
}

func (s Bin) PackMsg(buf []byte) []byte {
	l := len(s)
	switch {
	case l < 0x100:
		buf = append(buf, 0xc4)
		buf = append(buf, byte(l))
	case l < 0x10000:
		buf = append(buf, 0xc5)
		buf = be.AppendUint16(buf, uint16(l))
	default:
		buf = append(buf, 0xc6)
		buf = be.AppendUint32(buf, uint32(l))
	}
	return append(buf, s...)
}

func (i Signed) PackMsg(buf []byte) []byte {
	switch {
	case i > -32 && i < 0x80:
		return append(buf, byte(i))
	case i >= math.MinInt8 && i <= math.MaxInt8:
		return append(buf, 0xd0, byte(int8(i)))
	case i >= math.MinInt16 && i <= math.MaxInt16:
		buf = append(buf, 0xd1)
		return be.AppendUint16(buf, uint16(i))
	case i >= math.MinInt32 && i <= math.MaxInt32:
		buf = append(buf, 0xd2)
		return be.AppendUint32(buf, uint32(i))
	default:
		buf = append(buf, 0xd3)
		return be.AppendUint64(buf, uint64(i))
	}
}

func (u Unsigned) PackMsg(buf []byte) []byte {
	switch {
	case u < 0x80:
		return append(buf, byte(u))
	case u <= math.MaxUint8:
		return append(buf, 0xcc, uint8(u))
	case u <= math.MaxUint16:
		buf = append(buf, 0xcd)
		return be.AppendUint16(buf, uint16(u))
	case u <= math.MaxUint32:
		buf = append(buf, 0xce)
		return be.AppendUint32(buf, uint32(u))
	default:
		buf = append(buf, 0xcf)
		return be.AppendUint64(buf, uint64(u))
	}
}

func (f F32) PackMsg(buf []byte) []byte {
	buf = append(buf, 0xca)
	return be.AppendUint32(buf, math.Float32bits(float32(f)))
}

func (f F64) PackMsg(buf []byte) []byte {
	buf = append(buf, 0xcb)
	return be.AppendUint64(buf, math.Float64bits(float64(f)))
}

func (t Time32) PackMsg(buf []byte) []byte {
	buf = append(buf, 0xd6, 0xff)
	return be.AppendUint32(buf, uint32(time.Time(t).Unix()))
}

func (t Time64) PackMsg(buf []byte) []byte {
	s := time.Time(t).Unix()
	ns := time.Time(t).Nanosecond()
	buf = append(buf, 0xd7, 0xff)
	return be.AppendUint64(buf, uint64(ns)<<34|uint64(s))
}

func (t Time96) PackMsg(buf []byte) []byte {
	s := time.Time(t).Unix()
	ns := time.Time(t).Nanosecond()
	buf = append(buf, 0xc7, 12, 0xff)
	buf = be.AppendUint32(buf, uint32(ns))
	return be.AppendUint64(buf, uint64(s))
}

func (t Time) PackMsg(buf []byte) []byte {
	s := time.Time(t).Unix()
	ns := time.Time(t).Nanosecond()
	if s&(-1<<34) != 0 {
		return Time96(t).PackMsg(buf)
	} else if ns != 0 {
		return Time64(t).PackMsg(buf)
	} else {
		return Time32(t).PackMsg(buf)
	}
}

func (x Ext) PackMsg(buf []byte) []byte {
	l := len(x.data)
	switch {
	case l == 1:
		buf = append(buf, 0xd4)
	case l == 2:
		buf = append(buf, 0xd5)
	case l == 4:
		buf = append(buf, 0xd6)
	case l == 8:
		buf = append(buf, 0xd7)
	case l == 16:
		buf = append(buf, 0xd8)
	case l <= math.MaxUint8:
		buf = append(buf, 0xc7)
		buf = Unsigned(l).PackMsg(buf)
	case l <= math.MaxUint16:
		buf = append(buf, 0xc8)
		buf = Unsigned(l).PackMsg(buf)
	default:
		buf = append(buf, 0xc9)
		buf = Unsigned(l).PackMsg(buf)
	}
	return append(append(buf, byte(x.x)), x.data...)
}

func MsgEncode(ls *lua.LState) int {
	dupe := make(map[*lua.LTable]struct{})
	buf := lmEncode(ls, ls.Get(1), nil, dupe)
	ls.Push(lua.LString(unsafe.String(unsafe.SliceData(buf), len(buf))))
	return 1
}

func markDupe(ls *lua.LState, dupe map[*lua.LTable]struct{}, v *lua.LTable) {
	_, has := dupe[v]
	if has {
		ls.RaiseError("object contained cycle")
	}
	dupe[v] = struct{}{}
}

func lmEncode(
	ls *lua.LState,
	value lua.LValue,
	buf []byte,
	dupe map[*lua.LTable]struct{},
) []byte {
	for {
		if fn, ok := ls.GetMetaField(value, "__msgpack").(*lua.LFunction); ok {
			top := ls.GetTop()
			ls.Push(fn)
			ls.Push(value)
			ls.Call(1, 1)
			value = ls.Get(-1)
			ls.SetTop(top)
		} else {
			break
		}
	}
	switch v := value.(type) {
	case *lua.LNilType:
		return append(buf, 0xc0)
	case lua.LBool:
		if v {
			return append(buf, 0xc2)
		} else {
			return append(buf, 0xc3)
		}
	case lua.LNumber:
		if v == lua.LNumber(int64(v)) {
			return Signed(v).PackMsg(buf)
		} else if v == lua.LNumber(float32(v)) {
			return F32(v).PackMsg(buf)
		} else {
			return F64(v).PackMsg(buf)
		}
	case lua.LString:
		if utf8.ValidString(string(v)) {
			return Str(v).PackMsg(buf)
		} else {
			return Bin(v).PackMsg(buf)
		}
	case *lua.LTable:
		vlen := ls.ObjLen(v)
		if vlen == 0 {
			return lmEncodeMap(ls, v, buf, dupe)
		} else {
			return lmEncodeArray(ls, v, vlen, buf, dupe)
		}
	case *lua.LUserData:
		switch ud := v.Value.(type) {
		case Packer:
			return ud.PackMsg(buf)
		case Array:
			if ud == nil {
				return append(buf, 0xc0)
			} else {
				vlen := ls.ObjLen(v)
				return lmEncodeArray(ls, (*lua.LTable)(ud), vlen, buf, dupe)
			}
		case Map:
			if ud == nil {
				return append(buf, 0xc0)
			} else {
				return lmEncodeMap(ls, (*lua.LTable)(ud), buf, dupe)
			}
		case string:
			return Str(ud).PackMsg(buf)
		case []byte:
			return Bin(unsafe.String(unsafe.SliceData(ud), len(ud))).PackMsg(buf)
		case uint64:
			return Unsigned(ud).PackMsg(buf)
		case int64:
			return Signed(ud).PackMsg(buf)
		case time.Time:
			return Time(ud).PackMsg(buf)
		default:
			ls.RaiseError(fmt.Sprintf("UserData(%T) cannot encode to msgpack", ud))
			return buf
		}
	default:
		ls.RaiseError("Cannot encode " + v.Type().String())
		return buf
	}
}

func lmEncodeArray(
	ls *lua.LState,
	v *lua.LTable,
	vlen int,
	buf []byte,
	dupe map[*lua.LTable]struct{},
) []byte {
	markDupe(ls, dupe, v)
	switch {
	case vlen < 16:
		buf = append(buf, 0x90|byte(vlen))
	case vlen < 0x10000:
		buf = append(buf, 0xdc)
		buf = be.AppendUint16(buf, uint16(vlen))
	default:
		buf = append(buf, 0xdd)
		buf = be.AppendUint32(buf, uint32(vlen))
	}
	for i := range vlen {
		buf = lmEncode(ls, v.RawGetInt(i+1), buf, dupe)
	}
	return buf
}

func lmEncodeMap(
	ls *lua.LState,
	v *lua.LTable,
	buf []byte,
	dupe map[*lua.LTable]struct{},
) []byte {
	markDupe(ls, dupe, v)
	vlen := 0
	mapbuf := make([]byte, 0, 64)
	v.ForEach(func(k lua.LValue, v lua.LValue) {
		vlen += 1
		mapbuf = lmEncode(ls, k, mapbuf, dupe)
		mapbuf = lmEncode(ls, v, mapbuf, dupe)
	})
	switch {
	case vlen < 16:
		buf = append(buf, 0x80|byte(vlen))
	case vlen < 0x10000:
		buf = append(buf, 0xde)
		buf = be.AppendUint16(buf, uint16(vlen))
	default:
		buf = append(buf, 0xdf)
		buf = be.AppendUint32(buf, uint32(vlen))
	}
	return append(buf, mapbuf...)
}

func Loader(ls *lua.LState) int {
	m := ls.NewTable()
	m.RawSetString("encode", ls.NewFunction(MsgEncode))
	m.RawSetString("raw", ls.NewFunction(marker(newRaw)))
	m.RawSetString("array", ls.NewFunction(marker(newArray)))
	m.RawSetString("map", ls.NewFunction(marker(newMap)))
	m.RawSetString("bin", ls.NewFunction(marker(newBin)))
	m.RawSetString("str", ls.NewFunction(marker(newStr)))
	m.RawSetString("signed", ls.NewFunction(marker(newSigned)))
	m.RawSetString("unsigned", ls.NewFunction(marker(newUnsigned)))
	m.RawSetString("f32", ls.NewFunction(marker(newF32)))
	m.RawSetString("f64", ls.NewFunction(marker(newF64)))
	m.RawSetString("time32", ls.NewFunction(marker(newTime32)))
	m.RawSetString("time64", ls.NewFunction(marker(newTime64)))
	m.RawSetString("time96", ls.NewFunction(marker(newTime96)))
	m.RawSetString("time", ls.NewFunction(marker(newTime)))
	m.RawSetString("ext", ls.NewFunction(marker(newExt)))
	ls.Push(m)
	return 1
}