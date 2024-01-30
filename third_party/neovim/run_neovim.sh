#! /bin/bash
set -x

readonly _nvim_config_dir="${1}"
shift

readonly _nvim_lua_dir="${1}"
shift

readonly _nvim_share_dir="${1}"
shift

readonly _nvim_lib_dir="${1}"
shift

chmod a+x -R "${_nvim_lib_dir}"

env \
    XDG_CONFIG_DIRS="${_nvim_lua_dir}" \
	XDG_CONFIG_HOME="${_nvim_config_dir}" \
	VIMRUNTIME="${_nvim_share_dir}" \
	LD_PRELOAD_PATH="${_nvim_lib_dir}" \
	"$@"
