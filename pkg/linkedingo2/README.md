The contents of `x-li-recipe-map.json` and `x-li-query-map.json` should be
populated with the (formatted) values of the corresponding headers in the
request to

```
https://www.linkedin.com/realtime/connect?rc=1
```

made by the Messaging page on the LinkedIn site.

This value can change occasionally, so it is stored in a JSON file so it can be
updated quickly. Note that the JSON files are prettified so that they can be
easily diffed. The `realtime.go` file has an `init` function which removes the
whitespace on startup.
