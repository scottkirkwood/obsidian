---
title: "{{ .Title }}, {{ .Author }}"
isbn: {{ .ISBN }}
rating: {{ .Rating }}
average: {{ .Average }}
pages: {{ .Pages }}
date: {{ .DateRead }}
---

# {{ .Title }}

By **{{ .Author }}**

## Book data

[GoodReads ID/URL](https://www.goodreads.com/book/show/{{ .Id }})

- ISBN13: {{ .ISBN13 }}
- Rating: {{ .Rating }} (average: {{ .Average }})
- Published: {{ .Year }}
- Pages: {{ .Pages }}
- Date read: {{ .DateRead }}

## Review

{{ .Review }}
