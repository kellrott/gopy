# Copyright 2015 The go-python Authors.  All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

## py2/py3 compat
from __future__ import print_function

import simple as pkg

print("doc(pkg):\n%s" % repr(pkg.__doc__))
print("pkg.Func()...")
pkg.Func()
print("fct = pkg.Func...")
fct = pkg.Func
print("fct()...")
fct()

