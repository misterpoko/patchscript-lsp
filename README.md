# This is the unofficial patchscript lsp

Patchscript repo: 
`https://github.com/CapnKraken/patchscript`

Features include

- Syntax Highlighting
- Get Reference
- Rename
- Get Definition
- Autocomplete Suggestions



build using command

```
go build
```

make sure the binary is accessible (./patchscript-lsp)


For Neovim insert into

*init.lua*

```
# autocompletion suggestions automatically
vim.opt.autocomplete = true
vim.opt.complete:append("o")
vim.opt.completeopt = { "menuone", "noselect", "popup" }

#lsp configurations
vim.filetype.add({ extension = { patch = "patchscript" } })
vim.lsp.config("patchscript", {
    cmd = { "/path/to/patchscript-lsp" },
    filetypes = { "patchscript" },
    root_markers = { "scripts" },
})
vim.lsp.enable("patchscript")

```
