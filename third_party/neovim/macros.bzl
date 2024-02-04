def run_neovim(name, args = [], data = []):
    """
    Runs the neovim binary with the supplied arguments.
    """
    native.sh_binary(
        name = name,
        srcs = [
            "run_neovim.sh",
        ],
        data = [
            "@neovim//:bin",
            "@neovim//:lib",
            "@neovim//:lib_dir",
            "@neovim//:share",
            "@neovim//:share_dir",
            "//:plugin_dir",
            "//plugin:nvim_dir",
            "//bin/pcc:pcc"
        ] + data,
        args = [
            # $x
            "$(location //plugin:nvim_dir)",
            # $1
            "$(location //bin/pcc:pcc)",
            # $2
            "$(location //:plugin_dir)",
            # $3
            # The location of the share dir.
            "$(location @neovim//:share_dir)",
            # $4
            # The location of the lib dir, with .so files.
            "$(location @neovim//:lib_dir)",
            # $5
            # The nvim binary itself.
            "$(location @neovim//:bin)",
        ] + args,
    )
