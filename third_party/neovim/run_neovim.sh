#! /bin/bash
# Copyright (C) 2020 Google Inc.
#
# This file has been licensed under Apache 2.0 license.  Please see the LICENSE
# file at the root of the repository.

set -e

# This magic was copied from runfiles by consulting:
#   https://stackoverflow.com/questions/53472993/how-do-i-make-a-bazel-sh-binary-target-depend-on-other-binary-targets

# --- begin runfiles.bash initialization ---
# Copy-pasted from Bazel's Bash runfiles library (tools/bash/runfiles/runfiles.bash).
set -eo pipefail
if [[ ! -d "${RUNFILES_DIR:-/dev/null}" && ! -f "${RUNFILES_MANIFEST_FILE:-/dev/null}" ]]; then
  if [[ -f "$0.runfiles_manifest" ]]; then
    export RUNFILES_MANIFEST_FILE="$0.runfiles_manifest"
  elif [[ -f "$0.runfiles/MANIFEST" ]]; then
    export RUNFILES_MANIFEST_FILE="$0.runfiles/MANIFEST"
  elif [[ -f "$0.runfiles/bazel_tools/tools/bash/runfiles/runfiles.bash" ]]; then
    export RUNFILES_DIR="$0.runfiles"
  fi
fi
if [[ -f "${RUNFILES_DIR:-/dev/null}/bazel_tools/tools/bash/runfiles/runfiles.bash" ]]; then
  source "${RUNFILES_DIR}/bazel_tools/tools/bash/runfiles/runfiles.bash"
elif [[ -f "${RUNFILES_MANIFEST_FILE:-/dev/null}" ]]; then
  source "$(grep -m1 "^bazel_tools/tools/bash/runfiles/runfiles.bash " \
            "$RUNFILES_MANIFEST_FILE" | cut -d ' ' -f 2-)"
else
  echo >&2 "ERROR: cannot find @bazel_tools//tools/bash/runfiles:runfiles.bash"
  exit 1
fi
# --- end runfiles.bash initialization ---

readonly _gotopt_binary="$(rlocation \
  gotopt2/cmd/gotopt2/gotopt2_/gotopt2)"

# Exit quickly if the binary isn't found. This may happen if the binary location
# moves internally in bazel.
if [ -x "$(command -v ${_gotopt2_binary})" ]; then
  echo "gotopt2 binary not found"
  exit 240
fi

GOTOPT2_OUTPUT=$($_gotopt_binary "${@}" <<EOF
flags:
- name: "pcc-binary"
  type: string
  default: ""
  help: "the PCC binary to use"
- name: "tmp-dir"
  type: string
  default: ""
  help: "the temporary directory to use."
- name: "plugin-nvim-dir"
  type: string
  default: ""
  help: "the plugin nvim directory"
- name: "nvim-lua-dir"
  type: string
  default: ""
  help: ""
- name: "nvim-share-dir"
  type: string
  default: ""
  help: ""
- name: "nvim-lib-dir"
  type: string
  default: ""
  help: ""
- name: "nvim-binary"
  type: string
  default: ""
  help: ""
- name: "nvim-pidfile"
  type: string
  default: ""
  help: ""
- name: "timeout"
  type: string
  default: "1h"
  help: "The default timeout for a neovim run"
- name: "fifo-filename"
  type: string
  default: "neovim.fifo"
  help: ""
- name: "debug-keep-logs"
  type: bool
  help: "if set, logs are kept, not removed."
- name: "blocking"
  type: bool
  default: false
  help: "if set, neovim is started in blocking mode, this is for outside of tests"
EOF
)
if [[ "$?" == "11" ]]; then
  # When --help option is used, gotopt2 exits with code 11.
  exit 1
fi

# Evaluate the output of the call to gotopt2, shell vars assignment is here.
eval "${GOTOPT2_OUTPUT}"

function cleanup() {
  if [[ "${gotopt2_debug_keep_logs}" != "true" ]]; then
    rm -fr ${_tmpdir}
  fi
}

_tmpdir="${gotopt2_tmp_dir}"
if [[ ${_tmp_dir} == "" ]]; then
    _tmpdir="$(mktemp -d --tmpdir="${TMPDIR}" run_neovim.XXXXXXX)"
    trap 'cleanup()' EXIT
fi

if [[ "${gotopt2_plugin_nvim_dir}" == "" ]]; then
    echo "the flag --plugin-nvim-dir=... is required"
    exit 1
fi
readonly _plugin_nvim_dir="${gotopt2_plugin_nvim_dir}" # //plugin/nvim

if [[ "${gotopt2_pcc_binary}" == "" ]]; then
    echo "the flag --pcc-binary=... is required"
    exit 1
fi
readonly _pcc_binary="${gotopt2_pcc_binary}"

if [[ "${gotopt2_nvim_lua_dir}" == "" ]]; then
    echo "the flag --nvim-lua-dir=... is required"
    exit 1
fi
readonly _nvim_lua_dir="${gotopt2_nvim_lua_dir}" # //plugin

if [[ "${gotopt2_nvim_share_dir}" == "" ]]; then
    echo "the flag --nvim-share-dir=... is required"
    exit 1
fi
readonly _nvim_share_dir="${gotopt2_nvim_share_dir}" # neovim share dir

if [[ "${gotopt2_nvim_lib_dir}" == "" ]]; then
    echo "the flag --nvim-lib-dir=... is required"
    exit 1
fi
readonly _nvim_lib_dir="${gotopt2_nvim_lib_dir}" # path to shared libraries

_background="&"
if [[ "${gotopt2_blocking}" == "true" ]]; then
  _background=""
fi

chmod a+x -R "${_nvim_lib_dir}"

# XDG_CONFIG_HOME is where neovim looks for its init files and plugins.
# Not sure if any other directories actually makes a difference.

env \
    USERNAME="unknown" \
    LOGNAME="unknown" \
    PATH="" \
    HOME="${_tmpdir}" \
    PCC_LOG_DIR="${_tmpdir}" \
    PCC_BINARY="${_pcc_binary}" \
    XDG_CONFIG_HOME="${_nvim_lua_dir}" \
    XDG_CONFIG_DIRS="${_nvim_lua_dir}:${_plugin_nvim_dir}" \
    VIMRUNTIME="${_nvim_share_dir}" \
    LD_PRELOAD_PATH="${_nvim_lib_dir}" \
    /usr/bin/nohup \
    /usr/bin/timeout "${gotopt2_timeout}" \
    "${gotopt2_nvim_binary}" \
      ${gotopt2_args__[@]} \
      ${background}

readonly _nvim_pid="$!"

if [[ "${gotopt2_nvim_pidfile}" != "" ]]; then
    echo "${_nvim_pid}" > "${gotopt2_nvim_pidfile}"
else
    echo "nvim PID: ${_nvim_pid}"
fi
