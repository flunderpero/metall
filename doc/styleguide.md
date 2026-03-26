# Metall - Design Language Style Guide

This is just a junkyard for all the rules that emerge or that we invent along the way.

## Naming

* `Foo.as_xxx()` if it is a cheap conversion without allocation or lots of computation.
* `Foo.to_xxx()` if it involves allocation or lots of computation.
