// Copyright 2015 The go-python Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bind

import (
	"fmt"
	"strings"

	"go/types"
)

func (g *cpyGen) genType(sym *symbol) {
	if !sym.isType() {
		return
	}
	if sym.isStruct() {
		return
	}
	if sym.isBasic() && !sym.isNamed() {
		return
	}

	pkgname := sym.pkgname()

	g.decl.Printf("\n/* --- decls for type %s.%v --- */\n", pkgname, sym.goname)
	if sym.isBasic() {
		// reach at the underlying type
		btyp := g.pkg.syms.symtype(sym.GoType().Underlying())
		g.decl.Printf("typedef %s %s;\n\n", btyp.cgoname, sym.cgoname)
	} else {
		g.decl.Printf("typedef void* %s;\n\n", sym.cgoname)
	}
	g.decl.Printf("/* Python type for type %s.%v\n", pkgname, sym.goname)
	g.decl.Printf(" */\ntypedef struct {\n")
	g.decl.Indent()
	g.decl.Printf("PyObject_HEAD\n")
	if sym.isBasic() {
		g.decl.Printf("%[1]s cgopy; /* value of %[2]s */\n",
			sym.cgoname,
			sym.id,
		)
	} else {
		g.decl.Printf("%[1]s cgopy; /* unsafe.Pointer to %[2]s */\n",
			sym.cgoname,
			sym.id,
		)
	}
	g.decl.Outdent()
	g.decl.Printf("} %s;\n", sym.cpyname)
	g.decl.Printf("\n\n")

	g.impl.Printf("\n\n/* --- impl for %s.%v */\n\n", pkgname, sym.goname)

	g.genTypeNew(sym)
	g.genTypeDealloc(sym)
	g.genTypeInit(sym)
	g.genTypeMembers(sym)
	g.genTypeMethods(sym)

	g.genTypeProtocols(sym)

	asBuffer := "0"
	asSequence := "0"
	tpFlags := "Py_TPFLAGS_DEFAULT"
	if sym.isArray() || sym.isSlice() {
		asBuffer = fmt.Sprintf("&%[1]s_tp_as_buffer", sym.cpyname)
		asSequence = fmt.Sprintf("&%[1]s_tp_as_sequence", sym.cpyname)
		switch g.lang {
		case 2:
			tpFlags = fmt.Sprintf(
				"(%s)",
				strings.Join([]string{
					"Py_TPFLAGS_DEFAULT",
					"Py_TPFLAGS_HAVE_NEWBUFFER",
				},
					" |\n ",
				))
		case 3:
		}
	}

	g.impl.Printf("static PyTypeObject %sType = {\n", sym.cpyname)
	g.impl.Indent()
	g.impl.Printf("PyObject_HEAD_INIT(NULL)\n")
	g.impl.Printf("0,\t/*ob_size*/\n")
	g.impl.Printf("\"%s.%s\",\t/*tp_name*/\n", pkgname, sym.goname)
	g.impl.Printf("sizeof(%s),\t/*tp_basicsize*/\n", sym.cpyname)
	g.impl.Printf("0,\t/*tp_itemsize*/\n")
	g.impl.Printf("(destructor)%s_dealloc,\t/*tp_dealloc*/\n", sym.cpyname)
	g.impl.Printf("0,\t/*tp_print*/\n")
	g.impl.Printf("0,\t/*tp_getattr*/\n")
	g.impl.Printf("0,\t/*tp_setattr*/\n")
	g.impl.Printf("0,\t/*tp_compare*/\n")
	g.impl.Printf("0,\t/*tp_repr*/\n")
	g.impl.Printf("0,\t/*tp_as_number*/\n")
	g.impl.Printf("%s,\t/*tp_as_sequence*/\n", asSequence)
	g.impl.Printf("0,\t/*tp_as_mapping*/\n")
	g.impl.Printf("0,\t/*tp_hash */\n")
	g.impl.Printf("0,\t/*tp_call*/\n")
	g.impl.Printf("cpy_func_%s_tp_str,\t/*tp_str*/\n", sym.id)
	g.impl.Printf("0,\t/*tp_getattro*/\n")
	g.impl.Printf("0,\t/*tp_setattro*/\n")
	g.impl.Printf("%s,\t/*tp_as_buffer*/\n", asBuffer)
	g.impl.Printf("%s,\t/*tp_flags*/\n", tpFlags)
	g.impl.Printf("%q,\t/* tp_doc */\n", sym.doc)
	g.impl.Printf("0,\t/* tp_traverse */\n")
	g.impl.Printf("0,\t/* tp_clear */\n")
	g.impl.Printf("0,\t/* tp_richcompare */\n")
	g.impl.Printf("0,\t/* tp_weaklistoffset */\n")
	g.impl.Printf("0,\t/* tp_iter */\n")
	g.impl.Printf("0,\t/* tp_iternext */\n")
	g.impl.Printf("%s_methods,             /* tp_methods */\n", sym.cpyname)
	g.impl.Printf("0,\t/* tp_members */\n")
	g.impl.Printf("%s_getsets,\t/* tp_getset */\n", sym.cpyname)
	g.impl.Printf("0,\t/* tp_base */\n")
	g.impl.Printf("0,\t/* tp_dict */\n")
	g.impl.Printf("0,\t/* tp_descr_get */\n")
	g.impl.Printf("0,\t/* tp_descr_set */\n")
	g.impl.Printf("0,\t/* tp_dictoffset */\n")
	g.impl.Printf("(initproc)%s_init,      /* tp_init */\n", sym.cpyname)
	g.impl.Printf("0,                         /* tp_alloc */\n")
	g.impl.Printf("cpy_func_%s_new,\t/* tp_new */\n", sym.id)
	g.impl.Outdent()
	g.impl.Printf("};\n\n")

	g.genTypeConverter(sym)
}

func (g *cpyGen) genTypeNew(sym *symbol) {
	pkgname := sym.pkgname()

	g.decl.Printf("\n/* tp_new for %s.%v */\n", pkgname, sym.goname)
	g.decl.Printf(
		"static PyObject*\ncpy_func_%s_new(PyTypeObject *type, PyObject *args, PyObject *kwds);\n",
		sym.id,
	)

	g.impl.Printf("\n/* tp_new */\n")
	g.impl.Printf(
		"static PyObject*\ncpy_func_%s_new(PyTypeObject *type, PyObject *args, PyObject *kwds) {\n",
		sym.id,
	)
	g.impl.Indent()
	g.impl.Printf("%s *self;\n", sym.cpyname)
	g.impl.Printf("self = (%s *)type->tp_alloc(type, 0);\n", sym.cpyname)
	g.impl.Printf("self->cgopy = cgo_func_%s_new();\n", sym.id)
	g.impl.Printf("return (PyObject*)self;\n")
	g.impl.Outdent()
	g.impl.Printf("}\n\n")
}

func (g *cpyGen) genTypeDealloc(sym *symbol) {
	pkgname := sym.pkgname()

	g.decl.Printf("\n/* tp_dealloc for %s.%v */\n", pkgname, sym.goname)
	g.decl.Printf("static void\n%[1]s_dealloc(%[1]s *self);\n",
		sym.cpyname,
	)

	g.impl.Printf("\n/* tp_dealloc for %s.%v */\n", pkgname, sym.goname)
	g.impl.Printf("static void\n%[1]s_dealloc(%[1]s *self) {\n",
		sym.cpyname,
	)
	g.impl.Indent()
	if !sym.isBasic() {
		g.impl.Printf("cgopy_decref((%[1]s)(self->cgopy));\n", sym.cgoname)
	}
	g.impl.Printf("self->ob_type->tp_free((PyObject*)self);\n")
	g.impl.Outdent()
	g.impl.Printf("}\n\n")
}

func (g *cpyGen) genTypeInit(sym *symbol) {
	pkgname := sym.pkgname()

	g.decl.Printf("\n/* tp_init for %s.%v */\n", pkgname, sym.goname)
	g.decl.Printf(
		"static int\n%[1]s_init(%[1]s *self, PyObject *args, PyObject *kwds);\n",
		sym.cpyname,
	)

	g.impl.Printf("\n/* tp_init */\n")
	g.impl.Printf(
		"static int\n%[1]s_init(%[1]s *self, PyObject *args, PyObject *kwds) {\n",
		sym.cpyname,
	)
	g.impl.Indent()

	g.impl.Printf("return 0;\n")
	g.impl.Outdent()
	g.impl.Printf("}\n\n")
}

func (g *cpyGen) genTypeMembers(sym *symbol) {
	pkgname := sym.pkgname()
	g.decl.Printf("\n/* tp_getset for %s.%v */\n", pkgname, sym.goname)
	g.impl.Printf("\n/* tp_getset for %s.%v */\n", pkgname, sym.goname)
	g.impl.Printf("static PyGetSetDef %s_getsets[] = {\n", sym.cpyname)
	g.impl.Indent()
	g.impl.Printf("{NULL} /* Sentinel */\n")
	g.impl.Outdent()
	g.impl.Printf("};\n\n")
}

func (g *cpyGen) genTypeMethods(sym *symbol) {

	pkgname := sym.pkgname()
	g.decl.Printf("\n/* methods for %s.%s */\n", pkgname, sym.goname)
	g.impl.Printf("\n/* methods for %s.%s */\n", pkgname, sym.goname)
	g.impl.Printf("static PyMethodDef %s_methods[] = {\n", sym.cpyname)
	g.impl.Indent()
	g.impl.Printf("{NULL} /* sentinel */\n")
	g.impl.Outdent()
	g.impl.Printf("};\n\n")
}

func (g *cpyGen) genTypeProtocols(sym *symbol) {
	g.genTypeTPStr(sym)
	if sym.isSlice() || sym.isArray() {
		g.genTypeTPAsSequence(sym)
		g.genTypeTPAsBuffer(sym)
	}
}

func (g *cpyGen) genTypeTPStr(sym *symbol) {
	g.decl.Printf("\n/* __str__ support for %[1]s.%[2]s */\n",
		g.pkg.pkg.Name(),
		sym.goname,
	)
	g.decl.Printf(
		"static PyObject*\ncpy_func_%s_tp_str(PyObject *self);\n",
		sym.id,
	)

	g.impl.Printf(
		"static PyObject*\ncpy_func_%s_tp_str(PyObject *self) {\n",
		sym.id,
	)

	g.impl.Indent()
	g.impl.Printf("%[1]s c_self = ((%[2]s*)self)->cgopy;\n",
		sym.cgoname,
		sym.cpyname,
	)
	g.impl.Printf("GoString str = cgo_func_%[1]s_str(c_self);\n",
		sym.id,
	)
	g.impl.Printf("return cgopy_cnv_c2py_string(&str);\n")
	g.impl.Outdent()
	g.impl.Printf("}\n\n")
}

func (g *cpyGen) genTypeTPAsSequence(sym *symbol) {
	pkgname := g.pkg.pkg.Name()
	g.decl.Printf("\n/* sequence support for %[1]s.%[2]s */\n",
		pkgname, sym.goname,
	)

	var arrlen int64
	var etyp types.Type
	switch typ := sym.GoType().(type) {
	case *types.Array:
		etyp = typ.Elem()
		arrlen = typ.Len()
	case *types.Slice:
		etyp = typ.Elem()
	case *types.Named:
		switch typ := typ.Underlying().(type) {
		case *types.Array:
			etyp = typ.Elem()
			arrlen = typ.Len()
		case *types.Slice:
			etyp = typ.Elem()
		default:
			panic(fmt.Errorf("gopy: unhandled type [%#v]", typ))
		}
	default:
		panic(fmt.Errorf("gopy: unhandled type [%#v]", typ))
	}
	esym := g.pkg.syms.symtype(etyp)
	if esym == nil {
		panic(fmt.Errorf("gopy: could not retrieve element type of %#v",
			sym,
		))
	}

	switch g.lang {
	case 2:

		g.decl.Printf("\n/* len */\n")
		g.decl.Printf("static Py_ssize_t\ncpy_func_%[1]s_len(%[2]s *self);\n",
			sym.id,
			sym.cpyname,
		)

		g.impl.Printf("\n/* len */\n")
		g.impl.Printf("static Py_ssize_t\ncpy_func_%[1]s_len(%[2]s *self) {\n",
			sym.id,
			sym.cpyname,
		)
		g.impl.Indent()
		if sym.isArray() {
			g.impl.Printf("return %d;\n", arrlen)
		} else {
			g.impl.Printf("GoSlice *slice = (GoSlice*)(self->cgopy);\n")
			g.impl.Printf("return slice->len;\n")
		}
		g.impl.Outdent()
		g.impl.Printf("}\n\n")

		g.decl.Printf("\n/* item */\n")
		g.decl.Printf("static PyObject*\n")
		g.decl.Printf("cpy_func_%[1]s_item(%[2]s *self, Py_ssize_t i);\n",
			sym.id,
			sym.cpyname,
		)

		g.impl.Printf("\n/* item */\n")
		g.impl.Printf("static PyObject*\n")
		g.impl.Printf("cpy_func_%[1]s_item(%[2]s *self, Py_ssize_t i) {\n",
			sym.id,
			sym.cpyname,
		)
		g.impl.Indent()
		g.impl.Printf("PyObject *pyitem = NULL;\n")
		if sym.isArray() {
			g.impl.Printf("if (i < 0 || i >= %d) {\n", arrlen)
		} else {
			g.impl.Printf("GoSlice *slice = (GoSlice*)(self->cgopy);\n")
			g.impl.Printf("if (i < 0 || i >= slice->len) {\n")
		}
		g.impl.Indent()
		g.impl.Printf("PyErr_SetString(PyExc_IndexError, ")
		g.impl.Printf("\"array index out of range\");\n")
		g.impl.Printf("return NULL;\n")
		g.impl.Outdent()
		g.impl.Printf("}\n\n")
		g.impl.Printf("%[1]s item = cgo_func_%[2]s_item(self->cgopy, i);\n",
			esym.cgoname,
			sym.id,
		)
		g.impl.Printf("pyitem = %[1]s(&item);\n", esym.c2py)
		g.impl.Printf("return pyitem;\n")
		g.impl.Outdent()
		g.impl.Printf("}\n\n")

		g.decl.Printf("\n/* ass_item */\n")
		g.decl.Printf("static int\n")
		g.decl.Printf("cpy_func_%[1]s_ass_item(%[2]s *self, Py_ssize_t i, PyObject *v);\n",
			sym.id,
			sym.cpyname,
		)

		g.impl.Printf("\n/* ass_item */\n")
		g.impl.Printf("static int\n")
		g.impl.Printf("cpy_func_%[1]s_ass_item(%[2]s *self, Py_ssize_t i, PyObject *v) {\n",
			sym.id,
			sym.cpyname,
		)
		g.impl.Indent()
		g.impl.Printf("%[1]s c_v;\n", esym.cgoname)
		if sym.isArray() {
			g.impl.Printf("if (i < 0 || i >= %d) {\n", arrlen)
		} else {
			g.impl.Printf("GoSlice *slice = (GoSlice*)(self->cgopy);\n")
			g.impl.Printf("if (i < 0 || i >= slice->len) {\n")
		}
		g.impl.Indent()
		g.impl.Printf("PyErr_SetString(PyExc_IndexError, ")
		g.impl.Printf("\"array assignment index out of range\");\n")
		g.impl.Printf("return -1;\n")
		g.impl.Outdent()
		g.impl.Printf("}\n\n")
		g.impl.Printf("if (v == NULL) { return 0; }\n") // FIXME(sbinet): semantics?
		g.impl.Printf("if (!%[1]s(v, &c_v)) { return -1; }\n", esym.py2c)
		g.impl.Printf("cgo_func_%[1]s_ass_item(self->cgopy, i, c_v);\n", sym.id)
		g.impl.Printf("return 0;\n")
		g.impl.Outdent()
		g.impl.Printf("}\n\n")

		g.impl.Printf("\n/* tp_as_sequence */\n")
		g.impl.Printf("static PySequenceMethods %[1]s_tp_as_sequence = {\n", sym.cpyname)
		g.impl.Indent()
		g.impl.Printf("(lenfunc)cpy_func_%[1]s_len,\n", sym.id)
		g.impl.Printf("(binaryfunc)0,\n")   // array_concat,               sq_concat
		g.impl.Printf("(ssizeargfunc)0,\n") //array_repeat,                 /*sq_repeat
		g.impl.Printf("(ssizeargfunc)cpy_func_%[1]s_item,\n", sym.id)
		g.impl.Printf("(ssizessizeargfunc)0,\n") // array_slice,             /*sq_slice
		g.impl.Printf("(ssizeobjargproc)cpy_func_%[1]s_ass_item,\n", sym.id)
		g.impl.Printf("(ssizessizeobjargproc)0,\n") //array_ass_slice,      /*sq_ass_slice
		g.impl.Printf("(objobjproc)0,\n")           //array_contains,                 /*sq_contains
		g.impl.Printf("(binaryfunc)0,\n")           //array_inplace_concat,           /*sq_inplace_concat
		g.impl.Printf("(ssizeargfunc)0\n")          //array_inplace_repeat          /*sq_inplace_repeat
		g.impl.Outdent()
		g.impl.Printf("};\n\n")

	case 3:
	}
}

func (g *cpyGen) genTypeTPAsBuffer(sym *symbol) {
	pkgname := g.pkg.pkg.Name()
	g.decl.Printf("\n/* buffer support for %[1]s.%[2]s */\n",
		pkgname, sym.goname,
	)

	g.decl.Printf("\n/* __get_buffer__ impl for %[1]s.%[2]s */\n",
		pkgname, sym.goname,
	)
	g.decl.Printf("static int\n")
	g.decl.Printf(
		"cpy_func_%[1]s_getbuffer(PyObject *self, Py_buffer *view, int flags);\n",
		sym.id,
	)

	var esize int64
	var arrlen int64
	esym := g.pkg.syms.symtype(sym.goobj.Type())
	switch o := sym.goobj.Type().(type) {
	case *types.Array:
		esize = g.pkg.sz.Sizeof(o.Elem())
		arrlen = o.Len()
	case *types.Slice:
		esize = g.pkg.sz.Sizeof(o.Elem())
	}

	g.impl.Printf("\n/* __get_buffer__ impl for %[1]s.%[2]s */\n",
		pkgname, sym.goname,
	)
	g.impl.Printf("static int\n")
	g.impl.Printf(
		"cpy_func_%[1]s_getbuffer(PyObject *self, Py_buffer *view, int flags) {\n",
		sym.id,
	)
	g.impl.Indent()
	g.impl.Printf("if (view == NULL) {\n")
	g.impl.Indent()
	g.impl.Printf("PyErr_SetString(PyExc_ValueError, ")
	g.impl.Printf("\"NULL view in getbuffer\");\n")
	g.impl.Printf("return -1;\n")
	g.impl.Outdent()
	g.impl.Printf("}\n\n")
	g.impl.Printf("%[1]s *py = (%[1]s*)self;\n", sym.cpyname)
	if sym.isArray() {
		g.impl.Printf("void *array = (void*)(py->cgopy);\n")
		g.impl.Printf("view->obj = (PyObject*)py;\n")
		g.impl.Printf("view->buf = (void*)array;\n")
		g.impl.Printf("view->len = %d;\n", arrlen)
		g.impl.Printf("view->readonly = 0;\n")
		g.impl.Printf("view->itemsize = %d;\n", esize)
		g.impl.Printf("view->format = %q;\n", esym.pybuf)
		g.impl.Printf("view->ndim = 1;\n")
		g.impl.Printf("view->shape = (Py_ssize_t*)&view->len;\n")
	} else {
		g.impl.Printf("GoSlice *slice = (GoSlice*)(py->cgopy);\n")
		g.impl.Printf("view->obj = (PyObject*)py;\n")
		g.impl.Printf("view->buf = (void*)slice->data;\n")
		g.impl.Printf("view->len = slice->len;\n")
		g.impl.Printf("view->readonly = 0;\n")
		g.impl.Printf("view->itemsize = %d;\n", esize)
		g.impl.Printf("view->format = %q;\n", esym.pybuf)
		g.impl.Printf("view->ndim = 1;\n")
		g.impl.Printf("view->shape = (Py_ssize_t*)&slice->len;\n")
	}
	g.impl.Printf("view->strides = &view->itemsize;\n")
	g.impl.Printf("view->suboffsets = NULL;\n")
	g.impl.Printf("view->internal = NULL;\n")

	g.impl.Printf("\nPy_INCREF(py);\n")
	g.impl.Printf("return 0;\n")
	g.impl.Outdent()
	g.impl.Printf("}\n\n")

	switch g.lang {
	case 2:
		g.decl.Printf("\n/* readbuffer */\n")
		g.decl.Printf("static Py_ssize_t\n")
		g.decl.Printf(
			"cpy_func_%[1]s_readbuffer(%[2]s *self, Py_ssize_t index, const void **ptr);\n",
			sym.id,
			sym.cpyname,
		)

		g.impl.Printf("\n/* readbuffer */\n")
		g.impl.Printf("static Py_ssize_t\n")
		g.impl.Printf(
			"cpy_func_%[1]s_readbuffer(%[2]s *self, Py_ssize_t index, const void **ptr) {\n",
			sym.id,
			sym.cpyname,
		)
		g.impl.Indent()
		g.impl.Printf("if (index != 0) {\n")
		g.impl.Indent()
		g.impl.Printf("PyErr_SetString(PyExc_SystemError, ")
		g.impl.Printf("\"Accessing non-existent array segment\");\n")
		g.impl.Printf("return -1;\n")
		g.impl.Outdent()
		g.impl.Printf("}\n\n")
		if sym.isArray() {
			g.impl.Printf("*ptr = (void*)self->cgopy;\n")
			g.impl.Printf("return %d;\n", arrlen)
		} else {
			g.impl.Printf("GoSlice *slice = (GoSlice*)self->cgopy;\n")
			g.impl.Printf("*ptr = (void*)slice->data;\n")
			g.impl.Printf("return slice->len;\n")
		}
		g.impl.Outdent()
		g.impl.Printf("}\n\n")

		g.decl.Printf("\n/* writebuffer */\n")
		g.decl.Printf("static Py_ssize_t\n")
		g.decl.Printf(
			"cpy_func_%[1]s_writebuffer(%[2]s *self, Py_ssize_t segment, void **ptr);\n",
			sym.id,
			sym.cpyname,
		)

		g.impl.Printf("\n/* writebuffer */\n")
		g.impl.Printf("static Py_ssize_t\n")
		g.impl.Printf(
			"cpy_func_%[1]s_writebuffer(%[2]s *self, Py_ssize_t segment, void **ptr) {\n",
			sym.id,
			sym.cpyname,
		)
		g.impl.Indent()
		g.impl.Printf("return cpy_func_%[1]s_readbuffer(self, segment, (const void**)ptr);\n",
			sym.id,
		)
		g.impl.Outdent()
		g.impl.Printf("}\n\n")

		g.decl.Printf("\n/* segcount */\n")
		g.decl.Printf("static Py_ssize_t\n")
		g.decl.Printf("cpy_func_%[1]s_segcount(%[2]s *self, Py_ssize_t *lenp);\n",
			sym.id,
			sym.cpyname,
		)

		g.impl.Printf("\n/* segcount */\n")
		g.impl.Printf("static Py_ssize_t\n")
		g.impl.Printf("cpy_func_%[1]s_segcount(%[2]s *self, Py_ssize_t *lenp) {\n",
			sym.id,
			sym.cpyname,
		)
		g.impl.Indent()
		if sym.isArray() {
			g.impl.Printf("if (lenp) { *lenp = %d; }\n", arrlen)
		} else {
			g.impl.Printf("GoSlice *slice = (GoSlice*)(self->cgopy);\n")
			g.impl.Printf("if (lenp) { *lenp = slice->len; }\n")
		}
		g.impl.Printf("return 1;\n")
		g.impl.Outdent()
		g.impl.Printf("}\n\n")

		g.decl.Printf("\n/* charbuffer */\n")
		g.decl.Printf("static Py_ssize_t\n")
		g.decl.Printf("cpy_func_%[1]s_charbuffer(%[2]s *self, Py_ssize_t segment, const char **ptr);\n",
			sym.id,
			sym.cpyname,
		)

		g.impl.Printf("\n/* charbuffer */\n")
		g.impl.Printf("static Py_ssize_t\n")
		g.impl.Printf("cpy_func_%[1]s_charbuffer(%[2]s *self, Py_ssize_t segment, const char **ptr) {\n",
			sym.id,
			sym.cpyname,
		)
		g.impl.Indent()
		g.impl.Printf("return cpy_func_%[1]s_readbuffer(self, segment, (const void**)ptr);\n", sym.id)
		g.impl.Outdent()
		g.impl.Printf("}\n\n")

	case 3:
		// no-op
	}

	switch g.lang {
	case 2:
		g.impl.Printf("\n/* tp_as_buffer */\n")
		g.impl.Printf("static PyBufferProcs %[1]s_tp_as_buffer = {\n", sym.cpyname)
		g.impl.Indent()
		g.impl.Printf("(readbufferproc)cpy_func_%[1]s_readbuffer,\n", sym.id)
		g.impl.Printf("(writebufferproc)cpy_func_%[1]s_writebuffer,\n", sym.id)
		g.impl.Printf("(segcountproc)cpy_func_%[1]s_segcount,\n", sym.id)
		g.impl.Printf("(charbufferproc)cpy_func_%[1]s_charbuffer,\n", sym.id)
		g.impl.Printf("(getbufferproc)cpy_func_%[1]s_getbuffer,\n", sym.id)
		g.impl.Printf("(releasebufferproc)0,\n")
		g.impl.Outdent()
		g.impl.Printf("};\n\n")
	case 3:

		g.impl.Printf("\n/* tp_as_buffer */\n")
		g.impl.Printf("static PyBufferProcs %[1]s_tp_as_buffer = {\n", sym.cpyname)
		g.impl.Indent()
		g.impl.Printf("(getbufferproc)cpy_func_%[1]s_getbuffer,\n", sym.id)
		g.impl.Printf("(releasebufferproc)0,\n")
		g.impl.Outdent()
		g.impl.Printf("};\n\n")
	}
}

func (g *cpyGen) genTypeConverter(sym *symbol) {
	g.decl.Printf("\n/* converters for %s - %s */\n",
		sym.id,
		sym.goname,
	)
	g.decl.Printf("static int\n")
	g.decl.Printf("cgopy_cnv_py2c_%[1]s(PyObject *o, %[2]s *addr);\n",
		sym.id,
		sym.cgoname,
	)
	g.decl.Printf("static PyObject*\n")
	g.decl.Printf("cgopy_cnv_c2py_%[1]s(%[2]s *addr);\n\n",
		sym.id,
		sym.cgoname,
	)

	g.impl.Printf("static int\n")
	g.impl.Printf("cgopy_cnv_py2c_%[1]s(PyObject *o, %[2]s *addr) {\n",
		sym.id,
		sym.cgoname,
	)
	g.impl.Indent()
	g.impl.Printf("%s *self = NULL;\n", sym.cpyname)
	g.impl.Printf("self = (%s *)o;\n", sym.cpyname)
	g.impl.Printf("*addr = self->cgopy;\n")
	g.impl.Printf("return 1;\n")
	g.impl.Outdent()
	g.impl.Printf("}\n\n")

	g.impl.Printf("static PyObject*\n")
	g.impl.Printf("cgopy_cnv_c2py_%[1]s(%[2]s *addr) {\n", sym.id, sym.cgoname)
	g.impl.Indent()
	g.impl.Printf("PyObject *o = cpy_func_%[1]s_new(&%[2]sType, 0, 0);\n",
		sym.id,
		sym.cpyname,
	)
	g.impl.Printf("if (o == NULL) {\n")
	g.impl.Indent()
	g.impl.Printf("return NULL;\n")
	g.impl.Outdent()
	g.impl.Printf("}\n")
	g.impl.Printf("((%[1]s*)o)->cgopy = *addr;\n", sym.cpyname)
	g.impl.Printf("return o;\n")
	g.impl.Outdent()
	g.impl.Printf("}\n\n")

}
