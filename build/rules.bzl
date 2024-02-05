# Bazel build rules for neovim compilation and testing.

def _nvim_integration_test_impl(ctx):
    name = ctx.attr.name
    test_binary = ctx.attr.binary.files.to_list()[0]
    _mkfifo = ctx.attr._mkfifo.files.to_list()[0]
    _mkdb = ctx.attr._mkdb.files.to_list()[0]
    _run_neovim = ctx.attr._run_neovim.files.to_list()[0]
    query = ctx.attr.query.files.to_list()[0]

    inputs = [ test_binary, query]
    tools = [ _mkfifo, _mkdb]

    runfiles = ctx.runfiles(files = [])
    transitive_runfiles = []

    for target in (ctx.attr.binary, ctx.attr._run_neovim):
        transitive_runfiles.append(target[DefaultInfo].default_runfiles)
        transitive_runfiles.append(target[DefaultInfo].data_runfiles)

    transitive_files = []
    for data in ctx.attr._data:
        transitive_runfiles.append(
            ctx.runfiles(files = data.files.to_list()))
        transitive_files += data.files.to_list()
    print("transitive: ", transitive_files)
    runfiles.merge_all(transitive_runfiles)

    # outputs
    outputs = []

    # create database with query
    tmp_dir = ctx.actions.declare_directory("{}.tmp.dir".format(name))
    outputs += [tmp_dir]
    db = ctx.actions.declare_file("{}.sqlite".format(name))
    outputs += [db]

    mkdb_args = ctx.actions.args()
    mkdb_args.add("--db", db.path)
    mkdb_args.add("--query-file", query.path)
    mkdb_args.add("--v", 3)
    mkdb_args.add("--log_dir", tmp_dir.path)
    ctx.actions.run(
        inputs = [query, tmp_dir],
        outputs = [db],
        executable = _mkdb,
        arguments = [mkdb_args],
        mnemonic = "mkdb",
    )

    # run a process, return PID file.
    pidfile = ctx.actions.declare_file("{}.nvim.pid".format(name))
    outputs += [pidfile]
    run_neovim_args = ctx.actions.args()

    ctx.actions.run(
        inputs = transitive_files + [tmp_dir],
        outputs = [pidfile],
        executable = _run_neovim,
        arguments = [run_neovim_args],
        mnemonic = "runNeovim",
        tools = [_run_neovim] + transitive_files,
    )

    # Create a test binary.
    test_file = ctx.actions.declare_file("{}.nvim.test.sh".format(name))
    outputs += [test_file]

    _test_binary_content = """#!/bin/bash
# GENERATED FILE, DO NOT EDIT!
# Runs a neovim integration test in a very specific environment.
# The test binary is assumed to have the --pid-file and --db-file flags.
{test_binary} --pid-file={pidfile} --db-file={dbfile}
""".format(
        test_binary = test_binary.path,
        pidfile = pidfile.path,
        dbfile = db.path,
    )

    test_args = ctx.actions.args()
    ctx.actions.write(
        output = test_file,
        content = _test_binary_content,
        is_executable = True,
    )

    # Collect all the runfiles from the dependencies and such.
    return [
        DefaultInfo(
            # Consider not returning all the outputs, but only the ones that
            # are actually useful.
            files = depset(outputs), 
            # This is the binary to be run when `bazel test` is run.
            executable = test_file,
            runfiles = runfiles,
        )
    ]

nvim_integration_test = rule(
    implementation = _nvim_integration_test_impl,
    test = True,
    attrs = {
        "binary": attr.label(
            mandatory = True,
            executable = True,
            cfg = "host",
        ),
        "query": attr.label(
            mandatory = True,
            allow_files = True,
        ),
        "_mkfifo": attr.label(
            default = Label("//bin/mkfifo"),
            executable = True,
            cfg = "exec",
        ),
        "_mkdb": attr.label(
            default = Label("//bin/mkdb"),
            executable = True,
            cfg = "exec",
        ),
        "_run_neovim": attr.label(
            default = Label("//third_party/neovim:neovim"),
            executable = True,
            cfg = "exec",
        ),
        "_data": attr.label_list(
            default = [
                Label("@bazel_tools//tools/bash/runfiles"),
                #Label(""),
            ],
            allow_files = True,
        ),
    },
)
