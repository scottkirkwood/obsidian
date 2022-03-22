---
title: "{{ .Title }}, {{ .Author }}"{{if .ISBN}}
isbn: {{ .ISBN }}
{{ end }}{{if .Rating }}rating: {{ .Rating }}
{{ end }}
average: {{ .Average }}
date: {{ .DateRead }}
{{if .Tags}}tags: [{{ .Tags }}]{{end}}
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
