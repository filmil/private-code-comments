# Private Code Comments LSP [![Test status](https://github.com/filmil/private-code-comments/workflows/Test/badge.svg)](https://github.com/filmil/private-code-comments/workflows/Test/badge.svg)

## Introduction

### What is this?

1. A LSP server for keeping private comments and annotations.
2. A Neovim plugin for the said LSP server.

### What does it do?

It allows you to leave notes on specific lines in the files you edit. The notes
are private for you only, and will not appear in the file itself.

The LSP server is the backend that saves your notes. Neovim is the frontend,
which uses the LSP client side facilities to show and manage your notes.

While I used Neovim as a vehicle for the private code comments, it should be
possible to use a setup for *any* editor you choose. However, for the time being,
I only have the Neovim plugin.

### What can you use it for?

For example, you can use it to leave breadcrumbs for yourself when you are
reading and analyzing a large code base.  If you figure a piece of code out,
you can write yourself a short note at the point where you did so. The note
remains available to you for as long as you want.

### References

This was not built in a vacuum.  Here are some similar projects that I used
as inspiration.

* https://github.com/masukomi/private_comments
* https://github.com/winter-again/annotate.nvim

There is at least one other project that I remember poring over, but I forgot
what that was.

The above projects however are not using the LSP infrastructure, so they end up
needing to reinvent the presentation of the annotations.

I also owe gratitude to my colleague Chris, for inspiring me to write this
little program.

## How to configure neovim to run this LSP.

> I would like to see a more automatic installation in the future. For the time
> being, this is not the case. 

Download and install the `pcc` binary somewhere in your `$PATH`.

Add the following configuration somewhere in your `init.lua`.

```lua
local pccpath = vim.fn.stdpath("data") .. "/lazy/pcc"
vim.opt.rtp:prepend(pccpath)
require('pcc').setup({
  -- Set to wherever you installed `pcc`.
  pcc_binary = os.getenv("HOME") .. "/local/bin/pcc", 
  -- Set to where you want your annotation database to be.
  db = os.getenv("HOME) .. "/tmp/pcc/db/db.sqlite",
  -- Set to where you want your logs to be written.
  log_dir = os.getenv("HOME) .. "/tmp/pcc/logs",
  -- Set the file patters you wish to install this server to.
  file_patterns = { "text" }

  -- Possibly set other options, see the below sections what they are.
})
```

### Defaults

Below are the default settings.

```
local default_opts = {
    -- The default width of the annotation window.
    annot_win_width = 25,

    -- The default padding of the annotation window.
    annot_win_padding = 2,

    -- The directory where logs will be saved.
    log_dir = os.getenv("PCC_LOG_DIR"),

    -- The path to the `pcc` binary.
    -- For some reason, using `pcc` will turn off workspacing.
    pcc_binary = os.getenv("PCC_BINARY") or "pcc",

    -- The path to the SQLITE database where annotations will be stored.
    db = os.getenv("PCC_DB"),

    -- The file patterns used to determine workspace root.
    root_patterns = {
        ".git",
        -- From //nvim_testing/content:workspace.marker
        "workspace.marker",
    },

    -- The file types that the server will apply to.
    file_patterns = { "text" },

    -- The log verbosity, perhaps keep at a low level.  The higher, the more
    -- verbose the logs are.
    log_verbosity = 4,

    -- The default "add or edit" key binding.
    annotate_command = "<leader>cr",

    -- The default "delete" key binding.
    delete_command = "<leader>cd",
}
```

## Maintenance

### Prerequisites

Install `bazelisk`, under the name `bazel` and place it in your `$PATH`.
Yes, Bazel gets flak for being complex, but it's really the only good way to
build software. If you disagree, we'll need to continue disagreeing.

Check out the code:

```sh
git clone https://github.com/filmil/private-code-comments
```

### Building

```bash
bazel build //...
```

Bazel will download all the needed dependencies, and build the project.

### Testing

Run tests:

```bash
bazel test //...
```

View the log results of the tests, after the test run.

```
bazel run //scripts:view_logs
```

The following tests are implemented:
* unit tests: for most functionality, aiming for full coverage.
* integration tests: the tests covering the interaction with neovim, using
  a hermetic instance of neovim that is brought up with the test fixture.

## Known issues

These are the issues I'd like to see resolved.

* The current plugin is *extremely* janky. I don't yet know enough to
  understand why. I hope to learn a bit more about Neovim internals so I can
  fix that.

* Ideally we'd have a `lsp-config` entry for this LSP server too.  There currently
  isn't one.

* The installation is manual. This should be automated.

* For some reason, the current setup above will kick out any other LSP servers
  you may want to use. Even though in *theory* it should coexist with whatever
  servers you may want to use.

* The code could be nicer and better modularized. At least, all the functionality
  is covered by hermetic tests, so if you find a bug I can fix it and add a
  regression test**.

