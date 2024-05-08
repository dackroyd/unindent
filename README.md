# Unindent

`unindent` is a linter for Go code to help avoid unnecessarily indented code blocks. These may appear in various cases,
commonly via `else` blocks which follow an `if` which always ends in a `return`, `break`, or `continue`. In these
situations, the `else` is not needed, and its contents be included at a lower level of indentation.

https://go.dev/wiki/CodeReviewComments#indent-error-flow

> Try to keep the normal code path at a minimal indentation, and indent the error handling, dealing with it first. This
> improves the readability of the code by permitting visually scanning the normal path quickly. For instance, don’t write:
>
> ```
> if err != nil {
>     // error handling
> } else {
>     // normal code
> }
> ```
>
> Instead, write:
>
> ```
> if err != nil {
>     // error handling
>     return // or continue, etc.
> }
> // normal code
> ```

While this calls out errors specifically, it does not only apply to error cases.
