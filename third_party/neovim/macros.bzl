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
            "//:config_dir",
            "@neovim//:share",
            "@neovim//:share_dir",
            "//:lua_dir",
            "//config/nvim:scripts",
        ] + data,
        args = [
            "$(location //:config_dir)",
            "$(location //:lua_dir)",
            # The location of the share dir.
            "$(location @neovim//:share_dir)",
            # The location of the lib dir, with .so files.
            "$(location @neovim//:lib_dir)",
            # The nvim binary itself.
            "$(location @neovim//:bin)",
        ] + args,
    )
