# ssot

SSOT: Single Source Of Truth command line utility

## Overview
SSOT stores a single source of truth for constant values in an `./ssot.yaml` file that contains a list of constants and a list of files to processes.

SSOT directives are comments in the native language and replacements are done _"inline."_

Directives are end-of-line comments that contain a named constant to replace, and a regexp to allow direct matching of content within the line. Matching identifies the content in the line that `ssot` will replace with the constant's value from the `Constants` map in  `./ssot/yaml`.

The regex can — but is not required to — have begin (`^`) and end (`$`) of line anchors but only one capture group (`(...)`) which should identify the value to replace.

## Benefits 
Why use `ssot`?  Here are the benefits I was after when I chose to develop and start using it:

1. **Use directives** to identify locations in source where constants are shared across language files.
2. **Enable error checking** for all directives to ensure correct syntax.
3. **Change values in one place** when there is a need to change values.
4. Or, **Easily rename constants** accurately via editor search.

## Usage 
Store a `ssot.yaml` file in whatever directory you want to maintain your constants with:

1. A `comments` map with file extensions and their related comment start characters, 
2. An `files` array with filenames to scan for constants, and 
3. A `constants` map with constant keys and values 

For example:
```yaml
---
comments:
    .go: //
    .sql: --

files:
    - ./shared/link_group.go
    - ./query.sql

constants:
    from_group_missing:      "from_group_missing"
    link_missing:            "link_missing"
    to_group_found:          "to_group_found"
```

Then in your source files use an end-of-line comment in the form of:

```
ssot[<constant>]: <regex>
```

Finally, run `ssot` in the directory where your `ssot.yaml` file exists.

![Running ssot](./assets/running-ssot.png)

### Example .GO file:

```go
package shared

const (
	FromGroupMissing     = "from_group_missing"       //ssot[from_group_missing]: "([^"]+)"
	LinkMissing          = "link_missing"             //ssot[link_missing]: "([^"]+)"
	ToGroupFound         = "to_group_found"           //ssot[to_group_found]: "([^"]+)"
)
```

### Example .SQL file:

```sql
SELECT
   'link_missing' AS exception, --ssot[link_missing]: '([^']+)'
   from_group_id,
   from_group_name,
   link_id,
   link_url,
   to_group_id,
   to_group_name
FROM
   link_group_move_from_to
WHERE
   link_found = 0
UNION
SELECT
   'from_group_missing' AS exception, --ssot[from_group_missing]: '([^']+)'
   from_group_id,
   from_group_name,
   link_id,
   link_url,
   to_group_id,
   to_group_name
FROM
   link_group_move_from_to
WHERE
   from_group_found = 0
UNION
SELECT
   'to_group_found' AS exception, --ssot[to_group_found]: '([^']+)'
   from_group_id,
   from_group_name,
   link_id,
   link_url,
   to_group_id,
   to_group_name
FROM
   link_group_move_from_to
WHERE
   to_group_found = 0;
```

## Goals
The current goals for `ssot` are:

1. **In-place updates** — no `/src` _(e.g. `.ts`)_ vs. `/dst` _(e.g. `.js`)_ files.
2. **Performance first** — Don't scan all files, require developer to provide a list to scan. 
3. **Keep it simple** — only adding complexity when really required.
4. **YAGNI** — Do not implement more than I _(or maybe someone else)_ currently need(s).

## Rationale
Written to scratch my own itch. I wanted to have a single-source of truth across different source files from different programming languages, but I did not want to have a `source -> dist` build step given the nature of the `truth` being small compared to the size of the code the truth is typically embedded in. 

## Known Limitations
1. Currently only one named constant can be replaced per line, but a constant can be referenced multiple times in the same line, if needed _(this is assumed but untested.)_
2. Wildcards for files are not _(yet?)_ supported.
3. No command line options so no execution options _(yet?)_.
4. No versioning _(yet?)_; use latest and caveat emptor. 
5. No tests _(yet?)_ as I did not need for the simple use-case that inspired me to write `ssot`.

## Bug Reports and Pull Requests
...are **Welcome!**  

If you find this and it is useful for you but you discover bugs or have improvements to add, please feel free to create a bug report and/or a pull request.   

## Copyright
Copyright 2024 Mike Schinkel

## License 
MIT
