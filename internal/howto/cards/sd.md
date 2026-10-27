# sd (embedded toolbox)

Find and replace in files with regular expressions (Rust regex syntax; `$1` for groups). It changes the files IN
PLACE: preview first with `-p`, and use the edit tool for a change in one place.

    sd -p 'oldName\(' 'newName(' src/main.rs        # preview, changes nothing
    sd -F '1.2.3' '1.2.4' Cargo.toml README.md       # fixed string, no regex
    'a-b-c' | sd '-' '_'                             # in a pipeline: prints a_b_c
