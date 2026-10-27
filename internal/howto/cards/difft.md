# difft (embedded toolbox: difftastic)

A diff that compares syntax, not lines: moved code and reformatting stop looking like changes.

    difft --color never old.rs new.rs                            # plain text, fit for tool output
    difft --color never --display inline a.json b.json
    $env:GIT_EXTERNAL_DIFF = 'difft --color never'; git diff; Remove-Item Env:GIT_EXTERNAL_DIFF
