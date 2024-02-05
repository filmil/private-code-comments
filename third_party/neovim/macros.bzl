def run_neovim(name, args = [], data = [], deps = []):
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
            "//bin/pcc:pcc",
            "@gotopt2//cmd/gotopt2",
        ] + data,
        args = [
            "--plugin-nvim-dir", "$(location //plugin:nvim_dir)",
            "--pcc-binary", "$(location //bin/pcc:pcc)",
            "--nvim-lua-dir", "$(location //:plugin_dir)",
            "--nvim-share-dir", "$(location @neovim//:share_dir)",
            "--nvim-lib-dir", "$(location @neovim//:lib_dir)",
            "--nvim-binary", "$(location @neovim//:bin)",
        ] + args,
        deps = [
            "@bazel_tools//tools/bash/runfiles",
        ] + deps,
    )
