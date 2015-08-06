// Copyright 2015 The go-python Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bind

import (
	"fmt"
	"hash/fnv"
	"reflect"
	"sort"
	"strings"

	"go/types"
)

var (
	universe *symtab
)

func hash(s string) string {
	h := fnv.New32a()
	h.Write([]byte(s))
	return fmt.Sprintf("0x%d", h.Sum32())
}

// symkind describes the kinds of symbol
type symkind int

const (
	skConst symkind = 1 << iota
	skVar
	skFunc
	skType
	skArray
	skBasic
	skInterface
	skMap
	skNamed
	skPointer
	skSignature
	skSlice
	skStruct
	skString
)

// symbol is an exported symbol in a go package
type symbol struct {
	kind    symkind
	gopkg   *types.Package
	goobj   types.Object
	gotyp   types.Type
	doc     string
	id      string // mangled name of entity (eg: <pkg>_<name>)
	goname  string // name of go entity
	cgoname string // name of entity for cgo
	cpyname string // name of entity for cpython

	// for types only

	pyfmt string // format string for PyArg_ParseTuple
	pybuf string // format string for 'struct'/'buffer'
	pysig string // type string for doc-signatures
	c2py  string // name of c->py converter function
	py2c  string // name of py->c converter function
}

func (s symbol) isType() bool {
	return (s.kind & skType) != 0
}

func (s symbol) isNamed() bool {
	return (s.kind & skNamed) != 0
}

func (s symbol) isBasic() bool {
	return (s.kind & skBasic) != 0
}

func (s symbol) isArray() bool {
	return (s.kind & skArray) != 0
}

func (s symbol) isSlice() bool {
	return (s.kind & skSlice) != 0
}

func (s symbol) isStruct() bool {
	return (s.kind & skStruct) != 0
}

func (s symbol) hasConverter() bool {
	return s.pyfmt == "O&" && (s.c2py != "" || s.py2c != "")
}

func (s symbol) pkgname() string {
	if s.gopkg == nil {
		return ""
	}
	return s.gopkg.Name()
}

func (s symbol) GoType() types.Type {
	if s.goobj != nil {
		return s.goobj.Type()
	}
	return s.gotyp
}

func (s symbol) cgotypename() string {
	typ := s.gotyp
	switch typ := typ.(type) {
	case *types.Basic:
		n := typ.Name()
		if strings.HasPrefix(n, "untyped ") {
			n = string(n[len("untyped "):])
		}
		return n
	case *types.Named:
		obj := s.goobj
		switch typ.Underlying().(type) {
		case *types.Struct:
			return s.cgoname
		case *types.Interface:
			if obj.Name() == "error" {
				return "error"
			}
		}
	}
	return s.cgoname
}

// symtab is a table of symbols in a go package
type symtab struct {
	pkg    *types.Package
	syms   map[string]*symbol
	parent *symtab
}

func newSymtab(pkg *types.Package, parent *symtab) *symtab {
	if parent == nil {
		parent = universe
	}
	s := &symtab{
		pkg:    pkg,
		syms:   make(map[string]*symbol),
		parent: parent,
	}
	return s
}

func (sym *symtab) names() []string {
	names := make([]string, 0, len(sym.syms))
	for n := range sym.syms {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (sym *symtab) sym(n string) *symbol {
	s, ok := sym.syms[n]
	if ok {
		return s
	}
	if sym.parent != nil {
		return sym.parent.sym(n)
	}
	return nil
}

func (sym *symtab) typeof(n string) *symbol {
	s := sym.sym(n)
	switch s.kind {
	case skVar, skConst:
		tname := sym.typename(s.goobj.Type())
		return sym.sym(tname)
	case skFunc:
		//FIXME(sbinet): really?
		return s
	case skType:
		return s
	default:
		panic(fmt.Errorf("unhandled symbol kind (%v)", s.kind))
	}
	panic("unreachable")
}

func (sym *symtab) typename(t types.Type) string {
	return types.TypeString(t, types.RelativeTo(sym.pkg))
}

func (sym *symtab) symtype(t types.Type) *symbol {
	tname := sym.typename(t)
	s := sym.sym(tname)
	if s != nil {
		return s
	}
	obj := sym.pkg.Scope().Lookup(tname)
	switch {
	case obj == nil:
		// means we are actually creating a new type.
		// fmt.Printf(">>> adding new type [%s]...\n", tname)
		switch typ := t.(type) {
		case *types.Pointer:
			s = sym.symtype(typ.Elem())
			if s == nil {
				return nil
			}
			sym.addType(s.goobj, typ)
			return sym.sym(tname)

		case *types.Slice:
			s = sym.symtype(typ.Elem())
			if s == nil {
				return nil
			}
			sym.addType(s.goobj, typ)
			return sym.sym(tname)

		case *types.Signature:
			sym.addType(obj, typ)
			return sym.sym(tname)

		case *types.Named:
			switch tn := typ.Underlying().(type) {
			default:
				panic(fmt.Errorf("gopy: %#v -- %#v", typ, tn))
			}
		}
		panic(fmt.Errorf(
			"gopy: could not lookup type [%s] (%#v)",
			tname,
			t,
		))
	default:
		sym.addSymbol(obj)
		s = sym.symtype(obj.Type())
		if s == nil {
			panic(fmt.Errorf("gopy: could not lookup type [%s]", tname))
		}
		return s
	}
	return sym.sym(tname)
}

func (sym *symtab) addSymbol(obj types.Object) {
	n := obj.Name()
	pkg := obj.Pkg()
	id := n
	if pkg != nil {
		id = pkg.Name() + "_" + n
	}
	switch obj.(type) {
	case *types.Const:
		sym.syms[n] = &symbol{
			gopkg:   pkg,
			goobj:   obj,
			kind:    skConst,
			id:      id,
			goname:  n,
			cgoname: "cgo_const_" + id,
			cpyname: "cpy_const_" + id,
		}
		sym.addType(obj, obj.Type())

	case *types.Var:
		sym.syms[n] = &symbol{
			gopkg:   pkg,
			goobj:   obj,
			kind:    skVar,
			id:      id,
			goname:  n,
			cgoname: "cgo_var_" + id,
			cpyname: "cpy_var_" + id,
		}
		sym.addType(obj, obj.Type())

	case *types.Func:
		sym.syms[n] = &symbol{
			gopkg:   pkg,
			goobj:   obj,
			kind:    skFunc,
			id:      id,
			goname:  n,
			cgoname: "cgo_func_" + id,
			cpyname: "cpy_func_" + id,
		}

	case *types.TypeName:
		sym.addType(obj, obj.Type())

	default:
		panic(fmt.Errorf("gopy: handled object [%#v]", obj))
	}
}

func (sym *symtab) addType(obj types.Object, t types.Type) {
	n := sym.typename(t)
	var pkg *types.Package
	if obj != nil {
		pkg = obj.Pkg()
	}
	id := n
	if pkg != nil {
		id = pkg.Name() + "_" + n
	}
	kind := skType
	switch typ := t.(type) {
	case *types.Basic:
		kind |= skBasic
		styp := sym.symtype(typ)
		if styp == nil {
			panic(fmt.Errorf("builtin type not already known [%s]!", n))
		}

	case *types.Array:
		sym.addArrayType(pkg, obj, t, kind, id, n)

	case *types.Slice:
		sym.addSliceType(pkg, obj, t, kind, id, n)

	case *types.Signature:
		sym.addSignatureType(pkg, obj, typ, kind, id, n)

	case *types.Named:
		kind |= skNamed
		switch typ := typ.Underlying().(type) {
		case *types.Struct:
			sym.addStructType(pkg, obj, t, kind, id, n)

		case *types.Basic:
			bsym := sym.symtype(typ)
			sym.syms[n] = &symbol{
				gopkg:   pkg,
				goobj:   obj,
				gotyp:   typ,
				kind:    kind | skBasic,
				id:      id,
				goname:  n,
				cgoname: "cgo_type_" + id,
				cpyname: "cpy_type_" + id,
				pyfmt:   bsym.pyfmt,
				pybuf:   bsym.pybuf,
				pysig:   "object",
				c2py:    "cgopy_cnv_c2py_" + id,
				py2c:    "cgopy_cnv_py2c_" + id,
			}

		case *types.Array:
			sym.addArrayType(pkg, obj, typ, kind, id, n)

		case *types.Slice:
			sym.addSliceType(pkg, obj, typ, kind, id, n)

		case *types.Signature:
			sym.addSignatureType(pkg, obj, typ, kind, id, n)

		default:
			panic(fmt.Errorf("unhandled named-type: [%T]\n%#v\n", obj, t))
		}

	case *types.Pointer:
		// FIXME(sbinet): better handling?
		elm := *sym.symtype(typ.Elem())
		elm.kind |= skPointer
		sym.syms[n] = &elm

	default:
		panic(fmt.Errorf("unhandled obj [%T]\ntype [%#v]", obj, t))
	}
}

func (sym *symtab) addArrayType(pkg *types.Package, obj types.Object, t types.Type, kind symkind, id, n string) {
	typ := t.(*types.Array)
	kind |= skArray
	enam := sym.typename(typ.Elem())
	elt := sym.sym(enam)
	if elt == nil || elt.goname == "" {
		eobj := sym.pkg.Scope().Lookup(enam)
		if eobj == nil {
			panic(fmt.Errorf("could not look-up %q!\n", enam))
		}
		sym.addSymbol(eobj)
		elt = sym.typeof(enam)
	}
	id = hash(id)
	sym.syms[n] = &symbol{
		gopkg:   pkg,
		goobj:   obj,
		gotyp:   typ,
		kind:    kind,
		id:      id,
		goname:  n,
		cgoname: "cgo_type_" + id,
		cpyname: "cpy_type_" + id,
		pyfmt:   "O&",
		pybuf:   fmt.Sprintf("%d%s", typ.Len(), elt.pybuf),
		pysig:   "[]" + elt.pysig,
		c2py:    "cgopy_cnv_c2py_" + id,
		py2c:    "cgopy_cnv_py2c_" + id,
	}
}

func (sym *symtab) addSliceType(pkg *types.Package, obj types.Object, t types.Type, kind symkind, id, n string) {
	typ := t.(*types.Slice)
	kind |= skSlice
	enam := sym.typename(typ.Elem())
	elt := sym.sym(enam)
	if elt == nil || elt.goname == "" {
		eobj := sym.pkg.Scope().Lookup(enam)
		if eobj == nil {
			panic(fmt.Errorf("could not look-up %q!\n", enam))
		}
		sym.addSymbol(eobj)
		elt = sym.typeof(enam)
	}
	id = hash(id)
	sym.syms[n] = &symbol{
		gopkg:   pkg,
		goobj:   obj,
		gotyp:   typ,
		kind:    kind,
		id:      id,
		goname:  n,
		cgoname: "cgo_type_" + id,
		cpyname: "cpy_type_" + id,
		pyfmt:   "O&",
		pybuf:   elt.pybuf,
		pysig:   "[]" + elt.pysig,
		c2py:    "cgopy_cnv_c2py_" + id,
		py2c:    "cgopy_cnv_py2c_" + id,
	}
}

func (sym *symtab) addStructType(pkg *types.Package, obj types.Object, t types.Type, kind symkind, id, n string) {
	typ := t.Underlying().(*types.Struct)
	kind |= skStruct
	pybuf := make([]string, 0, typ.NumFields())
	for i := 0; i < typ.NumFields(); i++ {
		ftyp := typ.Field(i).Type()
		fsym := sym.symtype(ftyp)
		if fsym == nil {
			sym.addType(typ.Field(i), ftyp)
			fsym = sym.symtype(ftyp)
			if fsym == nil {
				panic(fmt.Errorf(
					"gopy: could not add type [%s]",
					ftyp.String(),
				))
			}
		}
		pybuf = append(pybuf, fsym.pybuf)
	}
	sym.syms[n] = &symbol{
		gopkg:   pkg,
		goobj:   obj,
		gotyp:   typ,
		kind:    kind,
		id:      id,
		goname:  n,
		cgoname: "cgo_type_" + id,
		cpyname: "cpy_type_" + id,
		pyfmt:   "O&",
		pybuf:   strings.Join(pybuf, ""),
		pysig:   "object",
		c2py:    "cgopy_cnv_c2py_" + id,
		py2c:    "cgopy_cnv_py2c_" + id,
	}
}

func (sym *symtab) addSignatureType(pkg *types.Package, obj types.Object, t types.Type, kind symkind, id, n string) {
	typ := t.(*types.Signature)
	kind |= skSignature
	id = hash(id)
	sym.syms[n] = &symbol{
		gopkg:   pkg,
		goobj:   obj,
		gotyp:   typ,
		kind:    kind,
		id:      id,
		goname:  n,
		cgoname: "cgo_type_" + id,
		cpyname: "cpy_type_" + id,
		pyfmt:   "O&",
		pybuf:   "P",
		pysig:   "callable",
		c2py:    "cgopy_cnv_c2py_" + id,
		py2c:    "cgopy_cnv_py2c_" + id,
	}
}

func init() {

	look := types.Universe.Lookup
	syms := map[string]*symbol{
		"bool": {
			gopkg:   look("bool").Pkg(),
			goobj:   look("bool"),
			gotyp:   look("bool").Type(),
			kind:    skType | skBasic,
			goname:  "bool",
			cgoname: "GoUint8",
			cpyname: "GoUint8",
			pyfmt:   "O&",
			pybuf:   "?",
			pysig:   "bool",
			c2py:    "cgopy_cnv_c2py_bool",
			py2c:    "cgopy_cnv_py2c_bool",
		},
		"byte": {
			gopkg:   look("byte").Pkg(),
			goobj:   look("byte"),
			gotyp:   look("byte").Type(),
			kind:    skType | skBasic,
			goname:  "byte",
			cpyname: "uint8_t",
			cgoname: "GoUint8",
			pyfmt:   "b",
			pybuf:   "B",
			pysig:   "int", // FIXME(sbinet) py2/py3
		},
		"int": {
			gopkg:   look("int").Pkg(),
			goobj:   look("int"),
			gotyp:   look("int").Type(),
			kind:    skType | skBasic,
			goname:  "int",
			cpyname: "int",
			cgoname: "GoInt",
			pyfmt:   "i",
			pybuf:   "i",
			pysig:   "int",
			c2py:    "cgopy_cnv_c2py_int",
			py2c:    "cgopy_cnv_py2c_int",
		},

		"int8": {
			gopkg:   look("int8").Pkg(),
			goobj:   look("int8"),
			gotyp:   look("int8").Type(),
			kind:    skType | skBasic,
			goname:  "int8",
			cpyname: "int8_t",
			cgoname: "GoInt8",
			pyfmt:   "b",
			pybuf:   "b",
			pysig:   "int",
			c2py:    "cgopy_cnv_c2py_int8",
			py2c:    "cgopy_cnv_py2c_int8",
		},

		"int16": {
			gopkg:   look("int16").Pkg(),
			goobj:   look("int16"),
			gotyp:   look("int16").Type(),
			kind:    skType | skBasic,
			goname:  "int16",
			cpyname: "int16_t",
			cgoname: "GoInt16",
			pyfmt:   "h",
			pybuf:   "h",
			pysig:   "int",
			c2py:    "cgopy_cnv_c2py_int16",
			py2c:    "cgopy_cnv_py2c_int16",
		},

		"int32": {
			gopkg:   look("int32").Pkg(),
			goobj:   look("int32"),
			gotyp:   look("int32").Type(),
			kind:    skType | skBasic,
			goname:  "int32",
			cpyname: "int32_t",
			cgoname: "GoInt32",
			pyfmt:   "i",
			pybuf:   "i",
			pysig:   "int",
			c2py:    "cgopy_cnv_c2py_int32",
			py2c:    "cgopy_cnv_py2c_int32",
		},

		"int64": {
			gopkg:   look("int64").Pkg(),
			goobj:   look("int64"),
			gotyp:   look("int64").Type(),
			kind:    skType | skBasic,
			goname:  "int64",
			cpyname: "int64_t",
			cgoname: "GoInt64",
			pyfmt:   "k",
			pybuf:   "q",
			pysig:   "long",
			c2py:    "cgopy_cnv_c2py_int64",
			py2c:    "cgopy_cnv_py2c_int64",
		},

		"uint": {
			gopkg:   look("uint").Pkg(),
			goobj:   look("uint"),
			gotyp:   look("uint").Type(),
			kind:    skType | skBasic,
			goname:  "uint",
			cpyname: "unsigned int",
			cgoname: "GoUint",
			pyfmt:   "I",
			pybuf:   "I",
			pysig:   "int",
			c2py:    "cgopy_cnv_c2py_uint",
			py2c:    "cgopy_cnv_py2c_uint",
		},

		"uint8": {
			gopkg:   look("uint8").Pkg(),
			goobj:   look("uint8"),
			gotyp:   look("uint8").Type(),
			kind:    skType | skBasic,
			goname:  "uint8",
			cpyname: "uint8_t",
			cgoname: "GoUint8",
			pyfmt:   "B",
			pybuf:   "B",
			pysig:   "int",
			c2py:    "cgopy_cnv_c2py_uint8",
			py2c:    "cgopy_cnv_py2c_uint8",
		},
		"uint16": {
			gopkg:   look("uint16").Pkg(),
			goobj:   look("uint16"),
			gotyp:   look("uint16").Type(),
			kind:    skType | skBasic,
			goname:  "uint16",
			cpyname: "uint16_t",
			cgoname: "GoUint16",
			pyfmt:   "H",
			pybuf:   "H",
			pysig:   "int",
			c2py:    "cgopy_cnv_c2py_uint16",
			py2c:    "cgopy_cnv_py2c_uint16",
		},
		"uint32": {
			gopkg:   look("uint32").Pkg(),
			goobj:   look("uint32"),
			gotyp:   look("uint32").Type(),
			kind:    skType | skBasic,
			goname:  "uint32",
			cpyname: "uint32_t",
			cgoname: "GoUint32",
			pyfmt:   "I",
			pybuf:   "I",
			pysig:   "long",
			c2py:    "cgopy_cnv_c2py_uint32",
			py2c:    "cgopy_cnv_py2c_uint32",
		},

		"uint64": {
			gopkg:   look("uint64").Pkg(),
			goobj:   look("uint64"),
			gotyp:   look("uint64").Type(),
			kind:    skType | skBasic,
			goname:  "uint64",
			cpyname: "uint64_t",
			cgoname: "GoUint64",
			pyfmt:   "K",
			pybuf:   "Q",
			pysig:   "long",
			c2py:    "cgopy_cnv_c2py_uint64",
			py2c:    "cgopy_cnv_py2c_uint64",
		},

		"float32": {
			gopkg:   look("float32").Pkg(),
			goobj:   look("float32"),
			gotyp:   look("float32").Type(),
			kind:    skType | skBasic,
			goname:  "float32",
			cpyname: "float",
			cgoname: "GoFloat32",
			pyfmt:   "f",
			pybuf:   "f",
			pysig:   "float",
			c2py:    "cgopy_cnv_c2py_f32",
			py2c:    "cgopy_cnv_py2c_f32",
		},
		"float64": {
			gopkg:   look("float64").Pkg(),
			goobj:   look("float64"),
			gotyp:   look("float64").Type(),
			kind:    skType | skBasic,
			goname:  "float64",
			cpyname: "double",
			cgoname: "GoFloat64",
			pyfmt:   "d",
			pybuf:   "d",
			pysig:   "float",
			c2py:    "cgopy_cnv_c2py_float64",
			py2c:    "cgopy_cnv_py2c_float64",
		},
		"complex64": {
			gopkg:   look("complex64").Pkg(),
			goobj:   look("complex64"),
			gotyp:   look("complex64").Type(),
			kind:    skType | skBasic,
			goname:  "complex64",
			cpyname: "float complex",
			cgoname: "GoComplex64",
			pyfmt:   "D",
			pybuf:   "ff",
			pysig:   "complex",
			c2py:    "cgopy_cnv_c2py_complex64",
			py2c:    "cgopy_cnv_py2c_complex64",
		},
		"complex128": {
			gopkg:   look("complex128").Pkg(),
			goobj:   look("complex128"),
			gotyp:   look("complex128").Type(),
			kind:    skType | skBasic,
			goname:  "complex128",
			cpyname: "double complex",
			cgoname: "GoComplex128",
			pyfmt:   "D",
			pybuf:   "dd",
			pysig:   "complex",
			c2py:    "cgopy_cnv_c2py_complex128",
			py2c:    "cgopy_cnv_py2c_complex128",
		},

		"string": {
			gopkg:   look("string").Pkg(),
			goobj:   look("string"),
			gotyp:   look("string").Type(),
			kind:    skType | skBasic,
			goname:  "string",
			cpyname: "GoString",
			cgoname: "GoString",
			pyfmt:   "O&",
			pybuf:   "s",
			pysig:   "str",
			c2py:    "cgopy_cnv_c2py_string",
			py2c:    "cgopy_cnv_py2c_string",
		},

		"rune": { // FIXME(sbinet) py2/py3
			gopkg:   look("rune").Pkg(),
			goobj:   look("rune"),
			gotyp:   look("rune").Type(),
			kind:    skType | skBasic,
			goname:  "rune",
			cpyname: "GoRune",
			cgoname: "GoRune",
			pyfmt:   "O&",
			pybuf:   "p",
			pysig:   "str",
			c2py:    "cgopy_cnv_c2py_rune",
			py2c:    "cgopy_cnv_py2c_rune",
		},

		"error": &symbol{
			gopkg:   look("error").Pkg(),
			goobj:   look("error"),
			gotyp:   look("error").Type(),
			kind:    skType | skInterface,
			goname:  "error",
			cgoname: "GoInterface",
			cpyname: "GoInterface",
			pyfmt:   "O&",
			pybuf:   "PP",
			pysig:   "object",
			c2py:    "cgopy_cnv_c2py_error",
			py2c:    "cgopy_cnv_py2c_error",
		},
	}

	if reflect.TypeOf(int(0)).Size() == 8 {
		syms["int"] = &symbol{
			gopkg:   look("int").Pkg(),
			goobj:   look("int"),
			gotyp:   look("int").Type(),
			kind:    skType | skBasic,
			goname:  "int",
			cpyname: "int64_t",
			cgoname: "GoInt",
			pyfmt:   "k",
			pybuf:   "q",
			pysig:   "int",
			c2py:    "cgopy_cnv_c2py_int",
			py2c:    "cgopy_cnv_py2c_int",
		}
		syms["uint"] = &symbol{
			gopkg:   look("uint").Pkg(),
			goobj:   look("uint"),
			gotyp:   look("uint").Type(),
			kind:    skType | skBasic,
			goname:  "uint",
			cpyname: "uint64_t",
			cgoname: "GoUint",
			pyfmt:   "K",
			pybuf:   "Q",
			pysig:   "int",
			c2py:    "cgopy_cnv_c2py_uint",
			py2c:    "cgopy_cnv_py2c_uint",
		}
	}

	for _, o := range []struct {
		kind  types.BasicKind
		tname string
		uname string
	}{
		{types.UntypedBool, "bool", "bool"},
		{types.UntypedInt, "int", "int"},
		{types.UntypedRune, "rune", "rune"},
		{types.UntypedFloat, "float64", "float"},
		{types.UntypedComplex, "complex128", "complex"},
		{types.UntypedString, "string", "string"},
		//FIXME(sbinet): what should be the python equivalent?
		//{types.UntypedNil, "nil", "nil"},
	} {
		sym := *syms[o.tname]
		n := "untyped " + o.uname
		syms[n] = &sym
	}

	universe = &symtab{
		pkg:    nil,
		syms:   syms,
		parent: nil,
	}
}
