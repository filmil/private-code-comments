#! /bin/bash
set -x

readonly _tmpdir="$(mktemp -d --tmpdir="${TMPDIR}" run_neovim.XXXXXXX)"
trap 'rm -fr ${_tmpdir}' EXIT

readonly _plugin_nvim_dir="${1}" # //plugin/nvim
shift

readonly _pcc_binary="${1}"
shift

readonly _nvim_lua_dir="${1}" # //plugin
shift

readonly _nvim_share_dir="${1}" # neovim share dir
shift

readonly _nvim_lib_dir="${1}" # path to shared libraries
shift

chmod a+x -R "${_nvim_lib_dir}"

# XDG_CONFIG_HOME is where neovim looks for its init files and plugins.
# Not sure if any other directories actually makes a difference.

env \
    USERNAME="unknown" \
    LOGNAME="unknown" \
    PATH="" \
    HOME="${_tmpdir}" \
    PCC_BINARY="${_pcc_binary}" \
    XDG_CONFIG_HOME="${_nvim_lua_dir}" \
    XDG_CONFIG_DIRS="${_nvim_lua_dir}:${_plugin_nvim_dir}" \
    VIMRUNTIME="${_nvim_share_dir}" \
    LD_PRELOAD_PATH="${_nvim_lib_dir}" \
    "$@"
