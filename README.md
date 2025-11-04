# Private Code Comments LSP

[![Test status](https://github.com/filmil/private-code-comments/workflows/Test/badge.svg)](https://github.com/filmil/private-code-comments/workflows/Test/badge.svg) [![Tag and Release status](https://github.com/filmil/private-code-comments/workflows/Tag%20and%20Release/badge.svg)](https://github.com/filmil/private-code-comments/workflows/Tag%20and%20Release/badge.svg)

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

## Installation

Download the [release archive][rel], and unpack it into the directory
`$HOME/.config`. This should create a directory `$HOME/.config/pcc` on your
disk.

[rel]: https://github.com/filmil/private-code-comments/releases

Add the following configuration in your `init.lua`.

```lua
-- This setup places the private comments plugin into the directory $HOME/.config/pcc.
require('lspconfig')
local pcc_dir = os.getenv("HOME") .. "/.config/pcc"
vim.opt.rtp:prepend(pcc_dir)
local pcc_plugin = require('pcc')

pcc_plugin.setup_server_with_lsp_config(
 {}, -- lspconfig settings
 {
     pcc_binary = pcc_dir .. "/bin/pcc",
     log_dir = os.getenv("HOME") .. '/.local/state/pcc/logs',
     db = os.getenv("HOME") .. '/.local/state/pcc/db/db.sqlite',
     log_verbosity = 3,
     filetypes = {
         "bzl",
         "c",
         "cpp",
         "gn",
         "go",
         "lua",
         "markdown",
         "python",
         "rust",
         "text",
     },
     autostart = true,
 }
)
require('lspconfig').pcc.setup {}

vim.keymap.set({'n'}, '<leader>cr', pcc_plugin.edit, { desc = "[C]omment [R]eview" })
vim.keymap.set({'n'}, '<leader>cd', pcc_plugin.delete, { desc = "[C]omment [D]elete" })
```

## Usage

### Adding or editing a comment

Press the key combination for "Comment Review". A small annotation window
appears, where you can write your comment.

Once you are done writing the comment, pressing `q` in normal mode closes
the annotation window, and shows the comment as a hint on the line where the
cursor was.

If you do this on a line that already has a comment, the annotation window will
load the current comment.

### Deleting a comment

Press the key combination for "Comment Delete". The comment will be deleted on
the line if it exists.  Nothing changes if there is no comment to be deleted.

### Naming a workspace

Putting a file named `pcc.config.json` in the desired workspace root directory
allows you to give the name to the workspace. This makes all comments for that
workspace rooted at the location of the file, and makes all files be saved
in that workspace:

```
{
  "workspace_name": "some_name"
}
```

You can add the `pcc.config.json` in your global `.gitignore` file so that you can
place it in your project directories. This allows sharing comments if you so
choose.

## References

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

## Troubleshooting

If all else fails, [file a bug][bug].

[bug]: https://github.com/filmil/private-code-comments/issues

