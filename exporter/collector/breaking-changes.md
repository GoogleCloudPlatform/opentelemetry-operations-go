# Breaking changes vs old googlecloud exporter

The new pdata based exporter has some breaking changes from the original OpenCensus stackdriver
based `googlecloud` exporter:

## Labels

Original label key mapping code is
[here](https://github.com/census-ecosystem/opencensus-go-exporter-stackdriver/blob/42e7e58efdb937e8477f827d3fba022212335dbc/sanitize.go#L26).
The new code does not:

- truncate label keys longer than 100 characters.
- prepend `key` when the first character is `_`.
