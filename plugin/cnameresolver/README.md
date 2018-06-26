# cnames

## Name

*cnames* - performs `CNAME` target resolution by adding corresponding `A`/`AAAA` records for each `CNAME` record in a response.


## Description

For each CNAME in a response, the plugin will perform a self-lookup of that `CNAME` target and add the result to the response.
`CNAME` chains are supported, i.e. if a `CNAME` points to another `CNAME`, all members of the chain will be added to the result including the terminating `A`/`AAAA` record.
If a CNAME chain loop is encountered (no terminating `A` record) all members of the chain are added to the result (without duplicates).

This plugin can only be used once per Server Block.

## Syntax

~~~ txt
cnames [ZONES...]
~~~

* **ZONES** zones it should perform CNAME target lookups for. These zones refer to the question name zone, not CNAME target zones.
If empty, the zones from the configuration block are used.


## Examples

Enable CNAME target resolution for all zones.

~~~ corefile
. {
    cnames
}
~~~
