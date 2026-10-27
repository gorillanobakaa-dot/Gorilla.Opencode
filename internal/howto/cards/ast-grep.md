# ast-grep (embedded toolbox)

Search and rewrite code by its syntax tree, not by text: `$A` matches one node, `$$$ARGS` any number. Knows the
language from the file extension or `--lang` (rust, go, ts, tsx, js, python, c, cpp, java, ...).

    ast-grep run --pattern 'fmt.Println($$$A)' --lang go internal       # every call, with location
    ast-grep run -p 'unwrap()' -l rust --json src                       # machine-readable matches
    ast-grep run -p 'log.Printf($$$A)' -r 'slog.Info($$$A)' -l go .      # shows the rewrite as a diff
Adding `-U` (--update-all) WRITES the rewrite into every file: show the diff first and ask.
