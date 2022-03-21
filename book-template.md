---
title: "{{ .Title }}, {{ .Author }}"
isbn: {{ .ISBN }}{{if .Rating }}
rating: {{ .Rating }}
{{ end }}
average: {{ .Average }}
pages: {{ .Pages }}
date: {{ .DateRead }}
tags: {{ .Tags }}
---

# {{ .Title }}

By **{{ .Author }}**

## Book data

[GoodReads ID/URL](https://www.goodreads.com/book/show/{{ .Id }})

- ISBN13: {{ .ISBN13 }}
- Rating: {{ if .Rating }}{{ .Rating }} {{ end }}(average: {{ .Average }})
- Published: {{ .Year }}
- Pages: {{ .Pages }}
- Date read: {{ .DateRead }}

## Review

{{ .Review }}
