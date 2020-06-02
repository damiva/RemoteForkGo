package main

import (
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/ailncode/gluaxmlpath"
	"github.com/cjoudrey/gluahttp"
	"github.com/cjoudrey/gluaurl"
	lua "github.com/yuin/gopher-lua"
	luajson "layeh.com/gopher-json"
)

// ServerLua module for lua
type ServerLua struct {
	W      http.ResponseWriter
	R      *http.Request
	Plug   string
	Script string
	After  string
	//Vars   map[string]string
	//p bool
}

// Loader is ServerLua module loader
func (s ServerLua) Loader(L *lua.LState) int {
	mod := L.SetFuncs(L.NewTable(), map[string]lua.LGFunction{
		"body":    s.body,
		"form":    s.form,
		"header":  s.header,
		"file":    s.file,
		"log_inf": s.logInf,
		"log_wrn": s.logWrn,
		"log_err": s.logErr,
		"log_dbg": s.logDbg,
		"enc64":   s.enc64,
		"dec64":   s.dec64,
		"decuri":  s.decuri,
		"encuri":  s.encuri,
	})
	L.SetField(mod, "version", lua.LString(myVers))
	L.SetField(mod, "method", lua.LString(s.R.Method))
	L.SetField(mod, "remote_addr", lua.LString(s.R.RemoteAddr))
	L.SetField(mod, "url", lua.LString("http://"+s.R.Host+s.R.RequestURI))
	L.SetField(mod, "host", lua.LString(s.R.Host))
	L.SetField(mod, "url_root", lua.LString("http://"+s.R.Host+pthTree+s.Plug))
	L.SetField(mod, "path_root", lua.LString(pthTree+s.Plug))
	L.SetField(mod, "path_script", lua.LString(s.Script))
	L.SetField(mod, "path_info", lua.LString(s.After))
	L.SetField(mod, "query_string", lua.LString(s.R.URL.RawQuery))
	L.Push(mod)
	return 1
}
func (s ServerLua) body(L *lua.LState) int {
	if np := L.GetTop(); np == 0 {
		if b, e := ioutil.ReadAll(s.R.Body); e != nil {
			panic(e)
		} else {
			s.R.Body.Close()
			L.Push(lua.LString(string(b)))
		}
	} else {
		var bs int
		for i := 1; i <= np; i++ {
			if b, e := s.W.Write([]byte(L.ToString(i))); e != nil {
				panic(e)
			} else {
				bs += b
			}
		}
		L.Push(lua.LNumber(bs))
	}
	return 1
}
func (s ServerLua) form(L *lua.LState) int {
	np := L.GetTop()
	if np == 0 {
		fs := L.NewTable()
		for k, vs := range s.R.Form {
			f := L.NewTable()
			for i, v := range vs {
				f.RawSetInt(i+1, lua.LString(v))
			}
			fs.RawSetString(k, f)
		}
		L.Push(fs)
		return 1
	}
	for i := 1; i <= np; i++ {
		L.Push(lua.LString(s.R.FormValue(L.ToString(i))))
	}
	return np
}
func (s ServerLua) header(L *lua.LState) int {
	c := false
	switch L.GetTop() {
	case 3:
		c = L.ToBool(3)
		fallthrough
	case 2:
		if c {
			s.W.Header().Add(L.ToString(1), L.ToString(2))
		} else {
			s.W.Header().Set(L.ToString(1), L.ToString(2))
		}
	case 1:
		s.W.WriteHeader(L.ToInt(1))
	case 0:
		hs := L.NewTable()
		for k, vs := range s.R.Header {
			h := L.NewTable()
			for i, v := range vs {
				h.RawSetInt(i+1, lua.LString(v))
			}
			hs.RawSetString(k, h)
		}
		L.Push(hs)
		return 1
	}
	return 0
}
func (s ServerLua) file(L *lua.LState) int {
	var wrt = os.O_CREATE | os.O_WRONLY
	switch L.GetTop() {
	case 3:
		if L.ToBool(3) {
			wrt = os.O_APPEND | wrt
		}
		fallthrough
	case 2:
		var e error
		var f *os.File
		var n int
		if L.CheckAny(2).Type() == lua.LTNil {
			e = os.Remove(filepath.Join(s.Plug, filepath.Clean(L.ToString(1))))
		} else if f, e = os.OpenFile(filepath.Join(s.Plug, filepath.Clean(L.ToString(1))), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666); e == nil {
			n, e = f.WriteString(L.ToString(2))
			f.Close()
		}
		L.Push(lua.LNumber(n))
		if e == nil {
			L.Push(lua.LNil)
		} else {
			L.Push(lua.LString(e.Error()))
		}
	case 1:
		if b, e := ioutil.ReadFile(filepath.Join(s.Plug, filepath.Clean(L.ToString(1)))); e == nil {
			L.Push(lua.LString(b))
			L.Push(lua.LNil)
		} else if os.IsNotExist(e) {
			L.Push(lua.LNil)
			L.Push(lua.LNil)
		} else {
			L.Push(lua.LNil)
			L.Push(lua.LString(e.Error()))
		}
	default:
		L.Push(lua.LNil)
		L.Push(lua.LNil)
	}
	return 2
}
func (s ServerLua) enc64(L *lua.LState) int {
	L.Push(lua.LString(base64.RawURLEncoding.EncodeToString([]byte(L.ToString(1)))))
	return 1
}
func (s ServerLua) dec64(L *lua.LState) int {
	if d, e := base64.RawURLEncoding.DecodeString(L.ToString(1)); e == nil {
		L.Push(lua.LString(string(d)))
		L.Push(lua.LNil)
	} else {
		L.Push(lua.LNil)
		L.Push(lua.LString(e.Error()))
	}
	return 2
}
func (s ServerLua) encuri(L *lua.LState) int {
	L.Push(lua.LString(url.QueryEscape(L.ToString(1))))
	return 1
}
func (s ServerLua) decuri(L *lua.LState) int {
	if s, e := url.QueryUnescape(L.ToString(1)); e == nil {
		L.Push(lua.LString(s))
		L.Push(lua.LNil)
	} else {
		L.Push(lua.LNil)
		L.Push(lua.LString(e.Error()))
	}
	return 2
}
func (s ServerLua) logInf(L *lua.LState) int {
	log.Info(L.ToString(1))
	return 0
}
func (s ServerLua) logWrn(L *lua.LState) int {
	log.Warning(L.ToString(1))
	return 0
}
func (s ServerLua) logErr(L *lua.LState) int {
	log.Error(L.ToString(1))
	return 0
}
func (s ServerLua) logDbg(L *lua.LState) int {
	if mySelf.Debug {
		log.Info(L.ToString(1))
	}
	return 0
}

// run lua:
func serveLua(w http.ResponseWriter, r *http.Request, file, plug, script, after string) {
	doFile := func(L *lua.LState) int {
		check(L.DoFile(filepath.Join(plug, filepath.FromSlash(L.ToString(1)))), true)
		return 0
	}
	//L := lua.NewState()
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer L.Close()
	for _, pair := range []struct {
		n string
		f lua.LGFunction
	}{
		{lua.LoadLibName, lua.OpenPackage}, // Must be first
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	} {
		check(L.CallByParam(lua.P{
			Fn:      L.NewFunction(pair.f),
			NRet:    0,
			Protect: true,
		}, lua.LString(pair.n)), true)
	}
	L.SetGlobal("dofile", L.NewFunction(doFile))
	L.PreloadModule("server", ServerLua{W: w, R: r, Plug: plug, Script: script, After: after}.Loader)
	L.PreloadModule("http", gluahttp.NewHttpModule(&http.Client{}).Loader)
	L.PreloadModule("url", gluaurl.Loader)
	//L.PreloadModule("scrape", gluahttpscrape.NewHttpScrapeModule().Loader)
	luajson.Preload(L)
	gluaxmlpath.Preload(L)
	check(L.DoFile(file), true)
}
