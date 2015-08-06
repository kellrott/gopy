// Copyright 2015 The go-python Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bind

import (
	"fmt"
	"strings"

	"go/types"
)

func (g *cpyGen) genStruct(cpy Struct) {
	pkgname := cpy.Package().Name()

	//fmt.Printf("obj: %#v\ntyp: %#v\n", obj, typ)
	g.decl.Printf("\n/* --- decls for struct %s.%v --- */\n", pkgname, cpy.GoName())
	g.decl.Printf("typedef void* %s;\n\n", cpy.sym.cgoname)
	g.decl.Printf("/* Python type for struct %s.%v\n", pkgname, cpy.GoName())
	g.decl.Printf(" */\ntypedef struct {\n")
	g.decl.Indent()
	g.decl.Printf("PyObject_HEAD\n")
	g.decl.Printf("%[1]s cgopy; /* unsafe.Pointer to %[2]s */\n",
		cpy.sym.cgoname,
		cpy.ID(),
	)
	g.decl.Outdent()
	g.decl.Printf("} %s;\n", cpy.sym.cpyname)
	g.decl.Printf("\n\n")

	g.impl.Printf("\n\n/* --- impl for %s.%v */\n\n", pkgname, cpy.GoName())

	g.genStructNew(cpy)
	g.genStructDealloc(cpy)
	g.genStructInit(cpy)
	g.genStructMembers(cpy)
	g.genStructMethods(cpy)

	g.genStructProtocols(cpy)

	g.impl.Printf("static PyTypeObject %sType = {\n", cpy.sym.cpyname)
	g.impl.Indent()
	g.impl.Printf("PyObject_HEAD_INIT(NULL)\n")
	g.impl.Printf("0,\t/*ob_size*/\n")
	g.impl.Printf("\"%s.%s\",\t/*tp_name*/\n", pkgname, cpy.GoName())
	g.impl.Printf("sizeof(%s),\t/*tp_basicsize*/\n", cpy.sym.cpyname)
	g.impl.Printf("0,\t/*tp_itemsize*/\n")
	g.impl.Printf("(destructor)%s_dealloc,\t/*tp_dealloc*/\n", cpy.sym.cpyname)
	g.impl.Printf("0,\t/*tp_print*/\n")
	g.impl.Printf("0,\t/*tp_getattr*/\n")
	g.impl.Printf("0,\t/*tp_setattr*/\n")
	g.impl.Printf("0,\t/*tp_compare*/\n")
	g.impl.Printf("0,\t/*tp_repr*/\n")
	g.impl.Printf("0,\t/*tp_as_number*/\n")
	g.impl.Printf("0,\t/*tp_as_sequence*/\n")
	g.impl.Printf("0,\t/*tp_as_mapping*/\n")
	g.impl.Printf("0,\t/*tp_hash */\n")
	g.impl.Printf("0,\t/*tp_call*/\n")
	g.impl.Printf("cpy_func_%s_tp_str,\t/*tp_str*/\n", cpy.sym.id)
	g.impl.Printf("0,\t/*tp_getattro*/\n")
	g.impl.Printf("0,\t/*tp_setattro*/\n")
	g.impl.Printf("0,\t/*tp_as_buffer*/\n")
	g.impl.Printf("Py_TPFLAGS_DEFAULT,\t/*tp_flags*/\n")
	g.impl.Printf("%q,\t/* tp_doc */\n", cpy.Doc())
	g.impl.Printf("0,\t/* tp_traverse */\n")
	g.impl.Printf("0,\t/* tp_clear */\n")
	g.impl.Printf("0,\t/* tp_richcompare */\n")
	g.impl.Printf("0,\t/* tp_weaklistoffset */\n")
	g.impl.Printf("0,\t/* tp_iter */\n")
	g.impl.Printf("0,\t/* tp_iternext */\n")
	g.impl.Printf("%s_methods,             /* tp_methods */\n", cpy.sym.cpyname)
	g.impl.Printf("0,\t/* tp_members */\n")
	g.impl.Printf("%s_getsets,\t/* tp_getset */\n", cpy.sym.cpyname)
	g.impl.Printf("0,\t/* tp_base */\n")
	g.impl.Printf("0,\t/* tp_dict */\n")
	g.impl.Printf("0,\t/* tp_descr_get */\n")
	g.impl.Printf("0,\t/* tp_descr_set */\n")
	g.impl.Printf("0,\t/* tp_dictoffset */\n")
	g.impl.Printf("(initproc)cpy_func_%s_init,      /* tp_init */\n", cpy.sym.id)
	g.impl.Printf("0,                         /* tp_alloc */\n")
	g.impl.Printf("cpy_func_%s_new,\t/* tp_new */\n", cpy.sym.id)
	g.impl.Outdent()
	g.impl.Printf("};\n\n")

	g.genStructConverters(cpy)

}

func (g *cpyGen) genStructNew(cpy Struct) {
	g.genTypeNew(cpy.sym)
}

func (g *cpyGen) genStructDealloc(cpy Struct) {
	g.genTypeDealloc(cpy.sym)
}

func (g *cpyGen) genStructInit(cpy Struct) {
	pkgname := cpy.Package().Name()

	g.decl.Printf("\n/* tp_init for %s.%v */\n", pkgname, cpy.GoName())
	g.decl.Printf(
		"static int\ncpy_func_%[1]s_init(%[2]s *self, PyObject *args, PyObject *kwds);\n",
		cpy.sym.id,
		cpy.sym.cpyname,
	)

	g.impl.Printf("\n/* tp_init */\n")
	g.impl.Printf(
		"static int\ncpy_func_%[1]s_init(%[2]s *self, PyObject *args, PyObject *kwds) {\n",
		cpy.sym.id,
		cpy.sym.cpyname,
	)
	g.impl.Indent()

	kwds := make(map[string]int)
	for _, ctor := range cpy.ctors {
		sig := ctor.Signature()
		for _, arg := range sig.Params() {
			n := arg.Name()
			if _, dup := kwds[n]; !dup {
				kwds[n] = len(kwds)
			}
		}
	}
	g.impl.Printf("static char *kwlist[] = {\n")
	g.impl.Indent()
	for k, v := range kwds {
		g.impl.Printf("%q, /* py_kwd_%d */\n", k, v)
	}
	g.impl.Printf("NULL\n")
	g.impl.Outdent()
	g.impl.Printf("};\n")

	for _, v := range kwds {
		g.impl.Printf("PyObject *py_kwd_%d = NULL;\n", v)
	}

	// FIXME(sbinet) remove when/if we manage to work out a proper dispatch
	// for ctors.
	g.impl.Printf("Py_ssize_t nkwds = (kwds != NULL) ? PyDict_Size(kwds) : 0;\n")
	g.impl.Printf("Py_ssize_t nargs = (args != NULL) ? PySequence_Size(args) : 0;\n")
	g.impl.Printf("if ((nkwds + nargs) > 0) {\n")
	g.impl.Indent()
	g.impl.Printf("PyErr_SetString(PyExc_TypeError, ")
	g.impl.Printf("\"%s.__init__ takes no argument\");\n", cpy.GoName())
	g.impl.Printf("return -1;\n")
	g.impl.Outdent()
	g.impl.Printf("}\n\n")

	g.impl.Printf("return 0;\n")
	g.impl.Outdent()
	g.impl.Printf("}\n\n")
}

func (g *cpyGen) genStructMembers(cpy Struct) {
	pkgname := cpy.Package().Name()
	typ := cpy.Struct()

	g.decl.Printf("\n/* tp_getset for %s.%v */\n", pkgname, cpy.GoName())
	for i := 0; i < typ.NumFields(); i++ {
		f := typ.Field(i)
		if !f.Exported() {
			continue
		}
		g.genStructMemberGetter(cpy, i, f)
		g.genStructMemberSetter(cpy, i, f)
	}

	g.impl.Printf("\n/* tp_getset for %s.%v */\n", pkgname, cpy.GoName())
	g.impl.Printf("static PyGetSetDef %s_getsets[] = {\n", cpy.sym.cpyname)
	g.impl.Indent()
	for i := 0; i < typ.NumFields(); i++ {
		f := typ.Field(i)
		if !f.Exported() {
			continue
		}
		doc := "doc for " + f.Name() // FIXME(sbinet) retrieve doc for fields
		g.impl.Printf("{%q, ", f.Name())
		g.impl.Printf("(getter)cpy_func_%[1]s_getter_%[2]d, ", cpy.sym.id, i+1)
		g.impl.Printf("(setter)cpy_func_%[1]s_setter_%[2]d, ", cpy.sym.id, i+1)
		g.impl.Printf("%q, NULL},\n", doc)
	}
	g.impl.Printf("{NULL} /* Sentinel */\n")
	g.impl.Outdent()
	g.impl.Printf("};\n\n")
}

func (g *cpyGen) genStructMemberGetter(cpy Struct, i int, f types.Object) {
	pkg := cpy.Package()
	ft := f.Type()
	var (
		cgo_fgetname = fmt.Sprintf("cgo_func_%[1]s_getter_%[2]d", cpy.sym.id, i+1)
		cpy_fgetname = fmt.Sprintf("cpy_func_%[1]s_getter_%[2]d", cpy.sym.id, i+1)
		ifield       = newVar(pkg, ft, f.Name(), "ret", "")
		results      = []*Var{ifield}
	)

	if needWrapType(ft) {
		g.decl.Printf("\n/* wrapper for field %s.%s.%s */\n",
			pkg.Name(),
			cpy.GoName(),
			f.Name(),
		)
		g.decl.Printf("typedef void* %[1]s_field_%d;\n", cpy.sym.cgoname, i+1)
	}

	g.decl.Printf("\n/* getter for %[1]s.%[2]s.%[3]s */\n",
		pkg.Name(), cpy.sym.goname, f.Name(),
	)
	g.decl.Printf("static PyObject*\n")
	g.decl.Printf(
		"%[2]s(%[1]s *self, void *closure); /* %[3]s */\n",
		cpy.sym.cpyname,
		cpy_fgetname,
		f.Name(),
	)

	g.impl.Printf("\n/* getter for %[1]s.%[2]s.%[3]s */\n",
		pkg.Name(), cpy.sym.goname, f.Name(),
	)
	g.impl.Printf("static PyObject*\n")
	g.impl.Printf(
		"%[2]s(%[1]s *self, void *closure) /* %[3]s */ {\n",
		cpy.sym.cpyname,
		cpy_fgetname,
		f.Name(),
	)
	g.impl.Indent()

	g.impl.Printf("PyObject *o = NULL;\n")
	ftname := g.pkg.syms.symtype(ft).cgoname
	if needWrapType(ft) {
		ftname = fmt.Sprintf("%[1]s_field_%d", cpy.sym.cgoname, i+1)
	}
	g.impl.Printf(
		"%[1]s c_ret = %[2]s(self->cgopy); /*wrap*/\n",
		ftname,
		cgo_fgetname,
	)

	{
		format := []string{}
		funcArgs := []string{}
		switch len(results) {
		case 1:
			ret := results[0]
			ret.name = "ret"
			pyfmt, pyaddrs := ret.getArgBuildValue()
			format = append(format, pyfmt)
			funcArgs = append(funcArgs, pyaddrs...)
		default:
			panic("bind: impossible")
		}
		g.impl.Printf("o = Py_BuildValue(%q, %s);\n",
			strings.Join(format, ""),
			strings.Join(funcArgs, ", "),
		)
	}

	g.impl.Printf("return o;\n")
	g.impl.Outdent()
	g.impl.Printf("}\n\n")

}

func (g *cpyGen) genStructMemberSetter(cpy Struct, i int, f types.Object) {
	var (
		pkg          = cpy.Package()
		ft           = f.Type()
		self         = newVar(pkg, cpy.GoType(), cpy.GoName(), "self", "")
		ifield       = newVar(pkg, ft, f.Name(), "ret", "")
		cgo_fsetname = fmt.Sprintf("cgo_func_%[1]s_setter_%[2]d", cpy.sym.id, i+1)
		cpy_fsetname = fmt.Sprintf("cpy_func_%[1]s_setter_%[2]d", cpy.sym.id, i+1)
	)

	g.decl.Printf("\n/* setter for %[1]s.%[2]s.%[3]s */\n",
		pkg.Name(), cpy.sym.goname, f.Name(),
	)

	g.decl.Printf("static int\n")
	g.decl.Printf(
		"%[2]s(%[1]s *self, PyObject *value, void *closure);\n",
		cpy.sym.cpyname,
		cpy_fsetname,
	)

	g.impl.Printf("\n/* setter for %[1]s.%[2]s.%[3]s */\n",
		pkg.Name(), cpy.sym.goname, f.Name(),
	)
	g.impl.Printf("static int\n")
	g.impl.Printf(
		"%[2]s(%[1]s *self, PyObject *value, void *closure) {\n",
		cpy.sym.cpyname,
		cpy_fsetname,
	)
	g.impl.Indent()

	ifield.genDecl(g.impl)
	g.impl.Printf("PyObject *tuple = NULL;\n\n")
	g.impl.Printf("if (value == NULL) {\n")
	g.impl.Indent()
	g.impl.Printf(
		"PyErr_SetString(PyExc_TypeError, \"Cannot delete '%[1]s' attribute\");\n",
		f.Name(),
	)
	g.impl.Printf("return -1;\n")
	g.impl.Outdent()
	g.impl.Printf("}\n")

	// TODO(sbinet) check 'value' type (PyString_Check, PyInt_Check, ...)

	g.impl.Printf("tuple = PyTuple_New(1);\n")
	g.impl.Printf("Py_INCREF(value);\n")
	g.impl.Printf("PyTuple_SET_ITEM(tuple, 0, value);\n\n")

	g.impl.Printf("\nif (!PyArg_ParseTuple(tuple, ")
	pyfmt, pyaddr := ifield.getArgParse()
	g.impl.Printf("%q, %s)) {\n", pyfmt, strings.Join(pyaddr, ", "))
	g.impl.Indent()
	g.impl.Printf("Py_DECREF(tuple);\n")
	g.impl.Printf("return -1;\n")
	g.impl.Outdent()
	g.impl.Printf("}\n")
	g.impl.Printf("Py_DECREF(tuple);\n\n")

	g.impl.Printf("%[1]s((%[2]s)(self->cgopy), c_%[3]s);\n",
		cgo_fsetname,
		self.CGoType(),
		ifield.Name(),
	)

	g.impl.Printf("return 0;\n")
	g.impl.Outdent()
	g.impl.Printf("}\n\n")
}

func (g *cpyGen) genStructMethods(cpy Struct) {

	pkgname := cpy.Package().Name()

	g.decl.Printf("\n/* methods for %s.%s */\n", pkgname, cpy.GoName())
	for _, m := range cpy.meths {
		g.genMethod(cpy, m)
	}

	g.impl.Printf("\n/* methods for %s.%s */\n", pkgname, cpy.GoName())
	g.impl.Printf("static PyMethodDef %s_methods[] = {\n", cpy.sym.cpyname)
	g.impl.Indent()
	for _, m := range cpy.meths {
		margs := "METH_VARARGS"
		if len(m.Signature().Params()) == 0 {
			margs = "METH_NOARGS"
		}
		g.impl.Printf(
			"{%[1]q, (PyCFunction)cpy_func_%[2]s, %[3]s, %[4]q},\n",
			m.GoName(),
			m.ID(),
			margs,
			m.Doc(),
		)
	}
	g.impl.Printf("{NULL} /* sentinel */\n")
	g.impl.Outdent()
	g.impl.Printf("};\n\n")
}

func (g *cpyGen) genMethod(cpy Struct, fct Func) {
	pkgname := g.pkg.pkg.Name()
	g.decl.Printf("\n/* wrapper of %[1]s.%[2]s */\n",
		pkgname,
		cpy.GoName()+"."+fct.GoName(),
	)
	g.decl.Printf("static PyObject*\n")
	g.decl.Printf("cpy_func_%s(PyObject *self, PyObject *args);\n", fct.ID())

	g.impl.Printf("/* wrapper of %[1]s.%[2]s */\n",
		pkgname,
		cpy.GoName()+"."+fct.GoName(),
	)
	g.impl.Printf("static PyObject*\n")
	g.impl.Printf("cpy_func_%s(PyObject *self, PyObject *args) {\n", fct.ID())
	g.impl.Indent()
	g.genMethodBody(fct)
	g.impl.Outdent()
	g.impl.Printf("}\n\n")
}

func (g *cpyGen) genMethodBody(fct Func) {
	g.genFuncBody(fct)
}

func (g *cpyGen) genStructProtocols(cpy Struct) {
	g.genStructTPStr(cpy)
}

func (g *cpyGen) genStructTPStr(cpy Struct) {
	g.genTypeTPStr(cpy.sym)
}

func (g *cpyGen) genStructConverters(cpy Struct) {
	g.genTypeConverter(cpy.sym)
}
