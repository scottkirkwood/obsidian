---
{{.YamlTags}}
---

# {{ .Title }}

By **{{ .Author }}**

## Book data

[GoodReads ID/URL](https://www.goodreads.com/book/show/{{ .Id }})

- Published: {{ .Year }}
- pages: {{ .Pages }}
- Date read: {{ .DateRead }}{{if .Tags}}
{{end}}

## Review

{{ .Review }}
