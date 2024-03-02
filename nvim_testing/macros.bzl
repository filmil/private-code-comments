load("@io_bazel_rules_go//go:def.bzl", "go_test")

def nvim_go_test(name, srcs, embed, deps = [], data = [], args = [], size = "small"):
    go_test(
        name = name,
        embed = embed,
        srcs = srcs,
        deps = [
        ] + deps,
        args = [
            # These are all flags declared in neovim.go.
            "--plugin-nvim-dir",
            "$(location //plugin:nvim_dir)",
            "--pcc-binary",
            "$(location //bin/pcc:pcc)",
            "--nvim-lua-dir",
            "$(location //:plugin_dir)",
            "--nvim-share-dir",
            "$(location @neovim//:share_dir)",
            "--nvim-lib-dir",
            "$(location @neovim//:lib_dir)",
            "--nvim-binary",
            "$(location @neovim//:bin)",
        ] + args,
        data = [
            "@neovim//:bin",
            "@neovim//:lib",
            "@neovim//:lib_dir",
            "@neovim//:share",
            "@neovim//:share_dir",
            "//:plugin_dir",
            "//plugin:nvim_dir",
            "//bin/pcc:pcc",
        ] + data,
        # Run the test in the top level runfiles dir. This is *not* the
        # default go test behavior.
        rundir = ".",
        size = size,
    )
