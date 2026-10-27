# pfind (embedded toolbox)

The search engine behind the find tool: ripgrep's crates, file names and contents, ranked. Prefer the find TOOL
(bounded, ranked output); call pfind in the shell only to pipe its output into something else.

    pfind "TODO" src                         # matching lines with context, under src
    pfind --names config                     # file and folder NAMES matching config
    pfind -t rust -w "fn main" --json        # Rust files only, whole word, machine-readable
