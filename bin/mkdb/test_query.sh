#! /bin/bash

# Tests that it's possible to open a new database. This should verify
# that cgo compilation works.

set -x
set -euo pipefail

readonly _bin="${1}"
"${_bin}" --query="select * from Annotations;"

