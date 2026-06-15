---
title: "a consistent way to add content from the clipboard"
created_at: "2026-06-15T11:40:00Z"
slug: "a-consistent-way-to-add-content-from-the-clipboard"
copied_at: "2026-06-15T12:01:34Z"
---

# a consistent way to add content from the clipboard

I think we need to rethink how information goes from the cli into the sp system via the clip board -- this system lends itself very well to markdown structured in a  certain way: since most entries only request a title and some body text it makes sense that an idea is simply just some markdown text with a single # header followed by one or more paragraphs of plain text with no other markdown elements and the elements must appear in that order with this we can change the way the cli works, instead of running sp 'some title' --cb we can just run sp clipboard and that will check the clipboard and if the content on the clipboard is formatted correctly, it can be added to the system as an idea
